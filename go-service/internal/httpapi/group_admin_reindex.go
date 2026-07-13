package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func (s *Server) handleAdminReindex(w http.ResponseWriter, r *http.Request) {
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, "POST /admin/reindex")
		return
	}
	req, ok := decodeAdminAuditBody(w, r)
	if !ok {
		return
	}
	sid := strings.TrimSpace(extractionStringFromAny(req["chat_session_id"]))
	if sid == "" {
		writeBadRequest(w, "chat_session_id is required")
		return
	}
	if completeTurnBoolFromAny(req["background"]) {
		if s.AdminJobs == nil {
			s.AdminJobs = newAdminJobManager()
		}
		job := s.AdminJobs.start("reindex", sid, req, func(ctx context.Context, progress adminJobProgressFunc) (map[string]any, error) {
			return s.runAdminReindexJob(ctx, sid, req, progress)
		})
		job["status"] = "accepted"
		job["job_status"] = "queued"
		job["poll_route"] = "/admin/jobs/" + fmt.Sprint(job["job_id"])
		job["note"] = "reindex is running in the background; poll the job route for progress"
		writeJSON(w, http.StatusAccepted, job)
		return
	}
	maxItems := intFromAny(req["max_items"], 200)
	if maxItems <= 0 {
		maxItems = 200
	}
	if maxItems > 5000 {
		maxItems = 5000
	}
	batchSize := intFromAny(req["batch_size"], 20)
	if batchSize <= 0 {
		batchSize = 20
	}
	if batchSize > 100 {
		batchSize = 100
	}
	force := completeTurnBoolFromAny(req["force"])
	dryRun := completeTurnBoolFromAny(req["dry_run"])
	meta := mapFromAny(req["client_meta"])
	cfg := s.completeTurnExtractionConfig(meta)

	memories, err := s.Store.ListMemories(r.Context(), sid, 0, 0)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, "POST /admin/reindex")
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	allMemories := append([]store.Memory(nil), memories...)
	evidence, err := s.Store.ListEvidence(r.Context(), sid)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			evidence = nil
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	worldRules, err := s.Store.ListWorldRules(r.Context(), sid)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			worldRules = nil
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	allEvidence := append([]store.DirectEvidence(nil), evidence...)
	allWorldRules := append([]store.WorldRule(nil), worldRules...)
	preIntegrity := s.adminReindexIntegrityReport(r.Context(), sid, allMemories, allEvidence, allWorldRules, strings.TrimSpace(cfg.Embedder.Model))
	if len(memories) > maxItems {
		memories = memories[:maxItems]
	}

	processed := 0
	upserted := 0
	skipped := 0
	errorsOut := []string{}
	failedIDs := []int64{}
	skippedIDs := []int64{}
	if !dryRun {
		for i := range memories {
			mem := memories[i]
			processed++
			summary := reindexMemoryDocumentText(mem)
			if summary == "" {
				skipped++
				skippedIDs = append(skippedIDs, mem.ID)
				continue
			}
			embeddingText := strings.TrimSpace(mem.Embedding)
			embeddingModel := strings.TrimSpace(mem.EmbeddingModel)
			if (force || embeddingText == "" || embeddingText == "[]") && cfg.Embedder.hasConfig() {
				emb, model, err := callEmbedding(r.Context(), cfg.Embedder, summary)
				if err != nil {
					errorsOut = append(errorsOut, fmt.Sprintf("memory:%d embedding: %s", mem.ID, err.Error()))
					failedIDs = append(failedIDs, mem.ID)
					skipped++
					continue
				}
				embeddingText = emb
				embeddingModel = model
			}
			embedding := parseFloat32JSONList(embeddingText)
			if len(embedding) == 0 {
				skipped++
				skippedIDs = append(skippedIDs, mem.ID)
				continue
			}
			mem.Embedding = embeddingText
			mem.EmbeddingModel = embeddingModel
			result := artifactSaveResult{VectorStatus: "not_requested"}
			s.upsertMemoryVector(r.Context(), sid, mem.TurnIndex, &mem, summary, embedding, &result)
			if result.VectorsUpserted > 0 {
				upserted += result.VectorsUpserted
			} else {
				skipped++
				if result.VectorStatus != "" && result.VectorStatus != "not_requested" && result.VectorStatus != "ok" {
					errorsOut = append(errorsOut, fmt.Sprintf("memory:%d vector: %s", mem.ID, result.VectorStatus))
					failedIDs = append(failedIDs, mem.ID)
				} else {
					skippedIDs = append(skippedIDs, mem.ID)
				}
			}
		}
	}
	artifactResult := s.adminReindexDerivedArtifacts(r.Context(), sid, cfg, dryRun, maxItems, allEvidence, allWorldRules, adminReindexDerivedArtifactProgress{})
	if !dryRun {
		processed += artifactResult.Processed
		upserted += artifactResult.Upserted
		skipped += artifactResult.Skipped
		errorsOut = append(errorsOut, artifactResult.Errors...)
	}
	processedBatches := 0
	if processed > 0 {
		processedBatches = (processed + batchSize - 1) / batchSize
	}
	qualityStatus := "not_run"
	if dryRun {
		qualityStatus = "dry_run"
	} else if upserted > 0 {
		qualityStatus = "requires_before_after_report"
	}
	integrityReport := preIntegrity
	var postIntegrity map[string]any
	if !dryRun {
		postIntegrity = s.adminReindexIntegrityReport(r.Context(), sid, allMemories, allEvidence, allWorldRules, strings.TrimSpace(cfg.Embedder.Model))
		integrityReport = postIntegrity
	}
	now := time.Now().UTC()
	s.saveAuditLogBestEffort(r.Context(), &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "admin_reindex",
		TargetType:    adminAuditTargetType(sid),
		TargetID:      0,
		Summary:       "Admin reindex requested",
		DetailsJSON: mustCompactJSON(map[string]any{
			"request_keys":             adminAuditRequestKeys(req),
			"dry_run":                  dryRun,
			"force":                    force,
			"batch_size":               batchSize,
			"max_items":                maxItems,
			"candidates":               len(memories),
			"processed":                processed,
			"processed_batches":        processedBatches,
			"upserted":                 upserted,
			"skipped":                  skipped,
			"embedding_model":          strings.TrimSpace(cfg.Embedder.Model),
			"embedding_provider":       strings.TrimSpace(cfg.Embedder.Provider),
			"embedding_configured":     cfg.Embedder.hasConfig(),
			"embedding_missing_fields": cfg.Embedder.missingFields(),
			"embedding_config_trace":   adminEmbeddingConfigTrace(meta, cfg),
			"failed_ids":               failedIDs,
			"skipped_ids":              skippedIDs,
			"derived_artifact_reindex": artifactResult.Summary(),
			"errors":                   errorsOut,
			"integrity_report":         integrityReport,
			"pre_reindex_integrity":    preIntegrity,
			"post_reindex_integrity":   postIntegrity,
			"quality_verification": map[string]any{
				"status":               qualityStatus,
				"required_for_cutover": true,
			},
		}),
		Source:    s.storeWriteSource(),
		CreatedAt: now,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"status":                   "ok",
		"source":                   s.storeWriteSource(),
		"chat_session_id":          sid,
		"mutation_enabled":         true,
		"reindex_executed":         !dryRun && upserted > 0,
		"dry_run":                  dryRun,
		"force":                    force,
		"batch_size":               batchSize,
		"max_items":                maxItems,
		"candidates":               len(memories),
		"processed":                processed,
		"processed_batches":        processedBatches,
		"upserted":                 upserted,
		"skipped":                  skipped,
		"embedding_model":          strings.TrimSpace(cfg.Embedder.Model),
		"embedding_provider":       strings.TrimSpace(cfg.Embedder.Provider),
		"embedding_configured":     cfg.Embedder.hasConfig(),
		"embedding_missing_fields": cfg.Embedder.missingFields(),
		"embedding_config_trace":   adminEmbeddingConfigTrace(meta, cfg),
		"failed_ids":               failedIDs,
		"skipped_ids":              skippedIDs,
		"derived_artifact_reindex": artifactResult.Summary(),
		"errors":                   errorsOut,
		"integrity_report":         integrityReport,
		"pre_reindex_integrity":    preIntegrity,
		"post_reindex_integrity":   postIntegrity,
		"quality_verification": map[string]any{
			"status":                qualityStatus,
			"required_for_cutover":  true,
			"before_after_required": true,
			"report_scope":          "search quality before/after reindex",
		},
		"audit_written": true,
		"changed_at":    now,
		"note":          "reindex rebuilt vector documents for memories and eligible derived artifacts when embedding settings were available",
	})
}

func (s *Server) runAdminReindexJob(ctx context.Context, sid string, req map[string]any, progress adminJobProgressFunc) (map[string]any, error) {
	maxItems := intFromAny(req["max_items"], 200)
	if maxItems <= 0 {
		maxItems = 200
	}
	if maxItems > 5000 {
		maxItems = 5000
	}
	batchSize := intFromAny(req["batch_size"], 20)
	if batchSize <= 0 {
		batchSize = 20
	}
	if batchSize > 100 {
		batchSize = 100
	}
	force := completeTurnBoolFromAny(req["force"])
	dryRun := completeTurnBoolFromAny(req["dry_run"])
	meta := mapFromAny(req["client_meta"])
	cfg := s.completeTurnExtractionConfig(meta)

	memories, err := s.Store.ListMemories(ctx, sid, 0, 0)
	if err != nil {
		return nil, err
	}
	allMemories := append([]store.Memory(nil), memories...)
	evidence, err := s.Store.ListEvidence(ctx, sid)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			evidence = nil
		} else {
			return nil, err
		}
	}
	worldRules, err := s.Store.ListWorldRules(ctx, sid)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			worldRules = nil
		} else {
			return nil, err
		}
	}
	allEvidence := append([]store.DirectEvidence(nil), evidence...)
	allWorldRules := append([]store.WorldRule(nil), worldRules...)
	preIntegrity := s.adminReindexIntegrityReport(ctx, sid, allMemories, allEvidence, allWorldRules, strings.TrimSpace(cfg.Embedder.Model))
	if !dryRun && !force && boolFromAny(preIntegrity["index_usable_for_vector_first_read"]) {
		result := map[string]any{
			"status":                 "ok",
			"source":                 s.storeWriteSource(),
			"chat_session_id":        sid,
			"mutation_enabled":       true,
			"reindex_executed":       false,
			"reason":                 "vector_index_already_current",
			"dry_run":                false,
			"force":                  false,
			"batch_size":             batchSize,
			"max_items":              maxItems,
			"candidates":             0,
			"processed":              0,
			"upserted":               0,
			"skipped":                0,
			"embedding_config_trace": adminEmbeddingConfigTrace(meta, cfg),
			"integrity_report":       preIntegrity,
			"pre_reindex_integrity":  preIntegrity,
			"post_reindex_integrity": preIntegrity,
			"errors":                 []string{},
			"background":             true,
			"note":                   "reindex skipped because the canonical vector candidate count and stored ChromaDB documents are already current",
		}
		if progress != nil {
			progress(map[string]any{
				"status":           "completed",
				"stage":            "already_current",
				"reason":           "vector_index_already_current",
				"candidate_count":  0,
				"processed":        0,
				"upserted":         0,
				"skipped_count":    0,
				"failed_count":     0,
				"integrity_report": preIntegrity,
				"progress_percent": 100,
			})
		}
		return result, nil
	}
	if len(memories) > maxItems {
		memories = memories[:maxItems]
	}
	derivedEvidenceCandidates, derivedWorldRuleCandidates := adminReindexDerivedArtifactCandidateCounts(maxItems, allEvidence, allWorldRules)
	totalCandidates := len(memories) + derivedEvidenceCandidates + derivedWorldRuleCandidates
	if block := adminReindexEmbeddingPreflightBlock(sid, cfg, meta, force, dryRun, maxItems, batchSize, memories, derivedEvidenceCandidates, derivedWorldRuleCandidates, preIntegrity); block != nil {
		if progress != nil {
			progress(block)
		}
		return block, nil
	}
	if progress != nil {
		progress(map[string]any{
			"status":                 "running",
			"stage":                  "memory_reindex",
			"tier":                   "memory",
			"candidate_count":        totalCandidates,
			"memory_candidates":      len(memories),
			"evidence_candidates":    derivedEvidenceCandidates,
			"world_rule_candidates":  derivedWorldRuleCandidates,
			"processed":              0,
			"upserted":               0,
			"skipped_count":          0,
			"failed_count":           0,
			"progress_percent":       0,
			"foreground_timeout":     false,
			"timeout_policy":         "background_job_detached_from_http_request",
			"integrity_report":       preIntegrity,
			"llm_config_trace":       completeTurnLLMConfigTrace(cfg),
			"embedding_config_trace": adminEmbeddingConfigTrace(meta, cfg),
		})
	}

	processed := 0
	upserted := 0
	skipped := 0
	errorsOut := []string{}
	failedIDs := []int64{}
	skippedIDs := []int64{}
	if !dryRun {
		for i := range memories {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
			mem := memories[i]
			processed++
			summary := reindexMemoryDocumentText(mem)
			if summary == "" {
				skipped++
				skippedIDs = append(skippedIDs, mem.ID)
			} else {
				embeddingText := strings.TrimSpace(mem.Embedding)
				embeddingModel := strings.TrimSpace(mem.EmbeddingModel)
				if (force || embeddingText == "" || embeddingText == "[]") && cfg.Embedder.hasConfig() {
					emb, model, err := callEmbedding(ctx, cfg.Embedder, summary)
					if err != nil {
						errorsOut = append(errorsOut, fmt.Sprintf("memory:%d embedding: %s", mem.ID, err.Error()))
						failedIDs = append(failedIDs, mem.ID)
						skipped++
						if progress != nil {
							p := adminReindexProgress(processed, totalCandidates, upserted, skipped, failedIDs, skippedIDs, mem.ID, errorsOut)
							p["stage"] = "memory_reindex"
							p["tier"] = "memory"
							p["memory_candidates"] = len(memories)
							p["evidence_candidates"] = derivedEvidenceCandidates
							p["world_rule_candidates"] = derivedWorldRuleCandidates
							progress(p)
						}
						continue
					}
					embeddingText = emb
					embeddingModel = model
				}
				embedding := parseFloat32JSONList(embeddingText)
				if len(embedding) == 0 {
					skipped++
					skippedIDs = append(skippedIDs, mem.ID)
				} else {
					mem.Embedding = embeddingText
					mem.EmbeddingModel = embeddingModel
					result := artifactSaveResult{VectorStatus: "not_requested"}
					s.upsertMemoryVector(ctx, sid, mem.TurnIndex, &mem, summary, embedding, &result)
					if result.VectorsUpserted > 0 {
						upserted += result.VectorsUpserted
					} else {
						skipped++
						if result.VectorStatus != "" && result.VectorStatus != "not_requested" && result.VectorStatus != "ok" {
							errorsOut = append(errorsOut, fmt.Sprintf("memory:%d vector: %s", mem.ID, result.VectorStatus))
							failedIDs = append(failedIDs, mem.ID)
							if isChromaDimensionMismatchStatus(result.VectorStatus) {
								blocked := adminReindexCollectionMismatchResult(sid, cfg, meta, dryRun, force, maxItems, batchSize, totalCandidates, processed, upserted, skipped, failedIDs, skippedIDs, errorsOut, preIntegrity, adminReindexDerivedArtifactResult{}, "memory", mem.ID)
								if progress != nil {
									progress(blocked)
								}
								return blocked, nil
							}
						} else {
							skippedIDs = append(skippedIDs, mem.ID)
						}
					}
				}
			}
			if progress != nil {
				p := adminReindexProgress(processed, totalCandidates, upserted, skipped, failedIDs, skippedIDs, mem.ID, errorsOut)
				p["stage"] = "memory_reindex"
				p["tier"] = "memory"
				p["memory_candidates"] = len(memories)
				p["evidence_candidates"] = derivedEvidenceCandidates
				p["world_rule_candidates"] = derivedWorldRuleCandidates
				progress(p)
			}
		}
	}
	artifactProgress := adminReindexDerivedArtifactProgress{
		Progress:      progress,
		BaseProcessed: processed,
		BaseUpserted:  upserted,
		BaseSkipped:   skipped,
		Total:         totalCandidates,
		FailedIDs:     append([]int64{}, failedIDs...),
		SkippedIDs:    append([]int64{}, skippedIDs...),
		Errors:        append([]string{}, errorsOut...),
	}
	artifactResult := s.adminReindexDerivedArtifacts(ctx, sid, cfg, dryRun, maxItems, allEvidence, allWorldRules, artifactProgress)
	if !dryRun {
		processed += artifactResult.Processed
		upserted += artifactResult.Upserted
		skipped += artifactResult.Skipped
		errorsOut = append(errorsOut, artifactResult.Errors...)
		if artifactResult.BlockedReason != "" {
			blocked := adminReindexCollectionMismatchResult(sid, cfg, meta, dryRun, force, maxItems, batchSize, totalCandidates, processed, upserted, skipped, failedIDs, skippedIDs, errorsOut, preIntegrity, artifactResult, artifactResult.BlockedTier, artifactResult.BlockedRowID)
			if progress != nil {
				progress(blocked)
			}
			return blocked, nil
		}
	}
	processedBatches := 0
	if processed > 0 {
		processedBatches = (processed + batchSize - 1) / batchSize
	}
	qualityStatus := "not_run"
	if dryRun {
		qualityStatus = "dry_run"
	} else if upserted > 0 {
		qualityStatus = "requires_before_after_report"
	}
	integrityReport := preIntegrity
	var postIntegrity map[string]any
	if !dryRun {
		postIntegrity = s.adminReindexIntegrityReport(ctx, sid, allMemories, allEvidence, allWorldRules, strings.TrimSpace(cfg.Embedder.Model))
		integrityReport = postIntegrity
	}
	now := time.Now().UTC()
	s.saveAuditLogBestEffort(ctx, &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "admin_reindex",
		TargetType:    adminAuditTargetType(sid),
		TargetID:      0,
		Summary:       "Admin reindex requested",
		DetailsJSON: mustCompactJSON(map[string]any{
			"background":               true,
			"request_keys":             adminAuditRequestKeys(req),
			"dry_run":                  dryRun,
			"force":                    force,
			"batch_size":               batchSize,
			"max_items":                maxItems,
			"candidates":               len(memories),
			"processed":                processed,
			"processed_batches":        processedBatches,
			"upserted":                 upserted,
			"skipped":                  skipped,
			"embedding_model":          strings.TrimSpace(cfg.Embedder.Model),
			"embedding_provider":       strings.TrimSpace(cfg.Embedder.Provider),
			"embedding_configured":     cfg.Embedder.hasConfig(),
			"embedding_missing_fields": cfg.Embedder.missingFields(),
			"embedding_config_trace":   adminEmbeddingConfigTrace(meta, cfg),
			"failed_ids":               failedIDs,
			"skipped_ids":              skippedIDs,
			"derived_artifact_reindex": artifactResult.Summary(),
			"errors":                   errorsOut,
			"integrity_report":         integrityReport,
			"pre_reindex_integrity":    preIntegrity,
			"post_reindex_integrity":   postIntegrity,
			"quality_verification": map[string]any{
				"status":               qualityStatus,
				"required_for_cutover": true,
			},
		}),
		Source:    s.storeWriteSource(),
		CreatedAt: now,
	})
	result := map[string]any{
		"status":                   "ok",
		"source":                   s.storeWriteSource(),
		"chat_session_id":          sid,
		"mutation_enabled":         true,
		"reindex_executed":         !dryRun && upserted > 0,
		"dry_run":                  dryRun,
		"force":                    force,
		"batch_size":               batchSize,
		"max_items":                maxItems,
		"candidates":               len(memories),
		"processed":                processed,
		"processed_batches":        processedBatches,
		"upserted":                 upserted,
		"skipped":                  skipped,
		"embedding_model":          strings.TrimSpace(cfg.Embedder.Model),
		"embedding_provider":       strings.TrimSpace(cfg.Embedder.Provider),
		"embedding_configured":     cfg.Embedder.hasConfig(),
		"embedding_missing_fields": cfg.Embedder.missingFields(),
		"embedding_config_trace":   adminEmbeddingConfigTrace(meta, cfg),
		"failed_ids":               failedIDs,
		"skipped_ids":              skippedIDs,
		"derived_artifact_reindex": artifactResult.Summary(),
		"errors":                   errorsOut,
		"integrity_report":         integrityReport,
		"pre_reindex_integrity":    preIntegrity,
		"post_reindex_integrity":   postIntegrity,
		"quality_verification": map[string]any{
			"status":                qualityStatus,
			"required_for_cutover":  true,
			"before_after_required": true,
			"report_scope":          "search quality before/after reindex",
		},
		"audit_written": true,
		"changed_at":    now,
		"background":    true,
		"note":          "reindex rebuilt vector documents for memories and eligible derived artifacts when embedding settings were available",
	}
	if progress != nil {
		progress(map[string]any{
			"status":           "completed",
			"candidate_count":  len(memories),
			"processed":        processed,
			"upserted":         upserted,
			"skipped_count":    skipped,
			"failed_count":     len(failedIDs),
			"failed_ids":       failedIDs,
			"skipped_ids":      skippedIDs,
			"errors":           errorsOut,
			"integrity_report": integrityReport,
			"progress_percent": 100,
		})
	}
	return result, nil
}

const adminReindexIntegrityPolicyVersion = "29-3.v1"

func (s *Server) adminReindexIntegrityReport(ctx context.Context, sid string, memories []store.Memory, evidence []store.DirectEvidence, worldRules []store.WorldRule, expectedEmbeddingModel string) map[string]any {
	expectedEmbeddingModel = strings.TrimSpace(expectedEmbeddingModel)
	missingEmbeddingIDs := []int64{}
	modelMismatchIDs := []int64{}
	observedModels := map[string]int{}
	for _, mem := range memories {
		model := strings.TrimSpace(mem.EmbeddingModel)
		if model != "" {
			observedModels[model]++
		}
		embedding := parseFloat32JSONList(strings.TrimSpace(mem.Embedding))
		if len(embedding) == 0 {
			if mem.ID > 0 {
				missingEmbeddingIDs = append(missingEmbeddingIDs, mem.ID)
			}
			continue
		}
		if expectedEmbeddingModel != "" && model != expectedEmbeddingModel {
			if mem.ID > 0 {
				modelMismatchIDs = append(modelMismatchIDs, mem.ID)
			}
		}
	}
	eligibleEvidenceCount := 0
	for _, item := range evidence {
		if adminEvidenceVectorEligible(item) {
			eligibleEvidenceCount++
		}
	}
	eligibleWorldRuleCount := 0
	for _, item := range worldRules {
		if adminWorldRuleVectorEligible(item) {
			eligibleWorldRuleCount++
		}
	}

	vectorConfigured := s != nil && s.Vector != nil && strings.TrimSpace(s.Cfg.ChromaEndpoint) != ""
	vectorStatus := "not_configured"
	vectorCount := 0
	vectorCountKnown := false
	vectorCountErr := ""
	vectorHealth := map[string]any{
		"status": "not_configured",
	}
	if vectorConfigured {
		vectorStatus = "configured"
		health, err := s.Vector.Health(ctx)
		if err != nil {
			vectorStatus = "health_error"
			vectorHealth = map[string]any{
				"status": "error",
				"error":  err.Error(),
			}
		} else {
			if strings.TrimSpace(health.Status) != "" {
				vectorStatus = strings.TrimSpace(health.Status)
			}
			vectorHealth = map[string]any{
				"status":           strings.TrimSpace(health.Status),
				"collection":       strings.TrimSpace(health.Collection),
				"persist_dir":      strings.TrimSpace(health.PersistDir),
				"total_count":      health.TotalCount,
				"project_model":    strings.TrimSpace(health.ProjectModel),
				"model_ready":      health.ModelReady,
				"preflight_issues": append([]string(nil), health.PreflightIssues...),
			}
		}
		count, err := s.Vector.Count(ctx, sid)
		if err != nil {
			vectorCountErr = err.Error()
		} else {
			vectorCount = count
			vectorCountKnown = true
		}
	}

	canonicalMemoryCount := len(memories)
	canonicalVectorCandidateCount := canonicalMemoryCount + eligibleEvidenceCount + eligibleWorldRuleCount
	missingVectorEstimate := 0
	extraVectorEstimate := 0
	if vectorCountKnown {
		if canonicalVectorCandidateCount > vectorCount {
			missingVectorEstimate = canonicalVectorCandidateCount - vectorCount
		} else if vectorCount > canonicalVectorCandidateCount {
			extraVectorEstimate = vectorCount - canonicalVectorCandidateCount
		}
	}

	reasons := []string{}
	reembedReasons := []string{}
	if !vectorConfigured {
		reasons = append(reasons, "vector_not_configured")
	}
	if vectorCountErr != "" {
		reasons = append(reasons, "vector_count_error")
	}
	if vectorCountKnown && vectorCount < canonicalMemoryCount {
		reasons = append(reasons, "vector_count_below_canonical_memory_count")
	}
	if vectorCountKnown && vectorCount < canonicalVectorCandidateCount {
		reasons = append(reasons, "vector_count_below_canonical_vector_candidate_count")
	}
	if vectorCountKnown && vectorCount > canonicalVectorCandidateCount {
		reasons = append(reasons, "vector_count_above_canonical_vector_candidate_count")
	}
	if len(missingEmbeddingIDs) > 0 {
		reasons = append(reasons, "memory_rows_missing_embedding")
		reembedReasons = append(reembedReasons, "memory_rows_missing_embedding")
	}
	if len(modelMismatchIDs) > 0 {
		reasons = append(reasons, "embedding_model_mismatch")
		reembedReasons = append(reembedReasons, "embedding_model_mismatch")
	}
	projectModel := strings.TrimSpace(stringFromAny(vectorHealth["project_model"]))
	if expectedEmbeddingModel != "" && projectModel != "" && projectModel != expectedEmbeddingModel {
		reasons = append(reasons, "vector_project_model_mismatch")
		reembedReasons = append(reembedReasons, "vector_project_model_mismatch")
	}

	status := "usable"
	if len(reasons) > 0 {
		status = "reindex_recommended"
	}
	if !vectorConfigured {
		status = "vector_not_configured"
	}
	return map[string]any{
		"policy_version":                     adminReindexIntegrityPolicyVersion,
		"status":                             status,
		"chat_session_id":                    sid,
		"canonical_memory_count":             canonicalMemoryCount,
		"canonical_evidence_vector_count":    eligibleEvidenceCount,
		"canonical_world_rule_vector_count":  eligibleWorldRuleCount,
		"canonical_vector_candidate_count":   canonicalVectorCandidateCount,
		"vector_configured":                  vectorConfigured,
		"vector_status":                      vectorStatus,
		"vector_health":                      vectorHealth,
		"vector_count":                       vectorCount,
		"vector_count_known":                 vectorCountKnown,
		"vector_count_error":                 nilIfEmpty(vectorCountErr),
		"vector_count_matches_canonical":     vectorCountKnown && vectorCount == canonicalVectorCandidateCount,
		"missing_vector_count_estimate":      missingVectorEstimate,
		"extra_vector_count_estimate":        extraVectorEstimate,
		"missing_embedding_count":            len(missingEmbeddingIDs),
		"missing_embedding_ids":              missingEmbeddingIDs,
		"expected_embedding_model":           expectedEmbeddingModel,
		"observed_embedding_models":          observedModels,
		"embedding_model_mismatch_count":     len(modelMismatchIDs),
		"embedding_model_mismatch_ids":       modelMismatchIDs,
		"reindex_recommended":                len(reasons) > 0,
		"reindex_reasons":                    reasons,
		"reembed_recommended":                len(reembedReasons) > 0,
		"reembed_reasons":                    reembedReasons,
		"index_usable_for_vector_first_read": vectorConfigured && vectorCountKnown && vectorCount > 0 && len(reasons) == 0,
	}
}

func adminReindexProgress(processed, total, upserted, skipped int, failedIDs, skippedIDs []int64, lastID int64, errorsOut []string) map[string]any {
	return map[string]any{
		"status":           "running",
		"candidate_count":  total,
		"processed":        processed,
		"upserted":         upserted,
		"skipped_count":    skipped,
		"failed_count":     len(failedIDs),
		"failed_ids":       append([]int64{}, failedIDs...),
		"skipped_ids":      append([]int64{}, skippedIDs...),
		"errors":           append([]string{}, errorsOut...),
		"last_processed":   lastID,
		"progress_percent": adminJobProgressPercent(processed, total),
	}
}

func adminReindexEmbeddingPreflightBlock(sid string, cfg completeTurnExtractionConfig, meta map[string]any, force, dryRun bool, maxItems, batchSize int, memories []store.Memory, evidenceCandidates, worldRuleCandidates int, integrity map[string]any) map[string]any {
	if dryRun {
		return nil
	}
	if !adminReindexNeedsEmbedding(force, memories, evidenceCandidates, worldRuleCandidates) {
		return nil
	}
	if cfg.Embedder.hasConfig() {
		return nil
	}
	reason := "missing_embedding_config"
	if strings.Contains(strings.TrimSpace(cfg.Embedder.Source), "partial") {
		reason = "embedding_config_incomplete"
	}
	return map[string]any{
		"status":                   "blocked",
		"stage":                    "embedding_config_preflight",
		"reason":                   reason,
		"ui_action":                "complete_embedding_settings_or_disable_reindex",
		"chat_session_id":          sid,
		"mutation_enabled":         true,
		"reindex_executed":         false,
		"dry_run":                  dryRun,
		"force":                    force,
		"batch_size":               batchSize,
		"max_items":                maxItems,
		"candidate_count":          len(memories) + evidenceCandidates + worldRuleCandidates,
		"memory_candidates":        len(memories),
		"evidence_candidates":      evidenceCandidates,
		"world_rule_candidates":    worldRuleCandidates,
		"processed":                0,
		"upserted":                 0,
		"skipped_count":            0,
		"failed_count":             0,
		"progress_percent":         100,
		"embedding_configured":     false,
		"embedding_missing_fields": cfg.Embedder.missingFields(),
		"embedding_config_trace":   adminEmbeddingConfigTrace(meta, cfg),
		"integrity_report":         integrity,
		"errors":                   []string{reason},
		"note":                     "vector reindex was blocked before mutation because embedding settings are incomplete; UI/env/runtime fields were not mixed",
	}
}

func adminReindexNeedsEmbedding(force bool, memories []store.Memory, evidenceCandidates, worldRuleCandidates int) bool {
	if evidenceCandidates > 0 || worldRuleCandidates > 0 {
		return true
	}
	for _, mem := range memories {
		if strings.TrimSpace(reindexMemoryDocumentText(mem)) == "" {
			continue
		}
		embeddingText := strings.TrimSpace(mem.Embedding)
		if force || embeddingText == "" || embeddingText == "[]" {
			return true
		}
	}
	return false
}

func isChromaDimensionMismatchStatus(status string) bool {
	text := strings.ToLower(strings.TrimSpace(status))
	return strings.Contains(text, "chroma collection dimension mismatch") ||
		(strings.Contains(text, "expecting embedding with dimension") && strings.Contains(text, "got"))
}

func adminReindexCollectionMismatchResult(sid string, cfg completeTurnExtractionConfig, meta map[string]any, dryRun, force bool, maxItems, batchSize, totalCandidates, processed, upserted, skipped int, failedIDs, skippedIDs []int64, errorsOut []string, integrity map[string]any, artifactResult adminReindexDerivedArtifactResult, blockedTier string, blockedRowID int64) map[string]any {
	return map[string]any{
		"status":                   "blocked",
		"stage":                    "collection_recreate_required",
		"reason":                   "chroma_collection_dimension_mismatch",
		"ui_action":                "recreate_chromadb_collection_then_reindex",
		"chat_session_id":          sid,
		"mutation_enabled":         true,
		"reindex_executed":         upserted > 0,
		"dry_run":                  dryRun,
		"force":                    force,
		"batch_size":               batchSize,
		"max_items":                maxItems,
		"candidate_count":          totalCandidates,
		"processed":                processed,
		"upserted":                 upserted,
		"skipped_count":            skipped,
		"failed_count":             len(failedIDs),
		"failed_ids":               append([]int64{}, failedIDs...),
		"skipped_ids":              append([]int64{}, skippedIDs...),
		"errors":                   append([]string{}, errorsOut...),
		"blocked_tier":             nilIfEmpty(blockedTier),
		"blocked_row_id":           blockedRowID,
		"embedding_model":          strings.TrimSpace(cfg.Embedder.Model),
		"embedding_provider":       strings.TrimSpace(cfg.Embedder.Provider),
		"embedding_configured":     cfg.Embedder.hasConfig(),
		"embedding_missing_fields": cfg.Embedder.missingFields(),
		"embedding_config_trace":   adminEmbeddingConfigTrace(meta, cfg),
		"derived_artifact_reindex": artifactResult.Summary(),
		"integrity_report":         integrity,
		"quality_verification": map[string]any{
			"status":               "blocked_collection_recreate_required",
			"required_for_cutover": true,
		},
		"note": "ChromaDB collection uses a different embedding dimension. The job stopped at the first mismatch to avoid repeated failures.",
	}
}

func adminEmbeddingConfigTrace(meta map[string]any, cfg completeTurnExtractionConfig) map[string]any {
	rawEmbedder := completeTurnExtractionConfigFromMeta(meta).Embedder
	metaEmbedding := mapFromAny(meta["embedding"])
	source := strings.TrimSpace(cfg.Embedder.Source)
	if source == "" {
		source = "missing"
		switch {
		case rawEmbedder.hasConfig():
			source = "client_meta"
		case len(metaEmbedding) > 0:
			source = "client_meta_partial"
		case cfg.Embedder.hasConfig():
			source = "runtime_or_env"
		}
	}
	return map[string]any{
		"configured":                 cfg.Embedder.hasConfig(),
		"source":                     source,
		"provider":                   strings.TrimSpace(cfg.Embedder.Provider),
		"endpoint_host":              endpointHost(cfg.Embedder.Endpoint),
		"model":                      strings.TrimSpace(cfg.Embedder.Model),
		"timeout_ms":                 cfg.Embedder.TimeoutMs,
		"missing_fields":             cfg.Embedder.missingFields(),
		"client_meta_present":        len(metaEmbedding) > 0,
		"client_meta_configured":     rawEmbedder.hasConfig(),
		"client_meta_missing_fields": rawEmbedder.missingFields(),
	}
}

func reindexMemoryDocumentText(mem store.Memory) string {
	if searchText := strings.TrimSpace(memorySearchTextFromMemory(mem).Text); searchText != "" {
		return searchText
	}
	return strings.TrimSpace(memorySummaryPreview(mem.SummaryJSON))
}
