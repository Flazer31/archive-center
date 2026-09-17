package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

type contextualizedEmbeddingItem struct {
	Key              string
	Text             string
	TurnIndex        int
	ContextTurnKnown bool
	NeedsEmbedding   bool
}

func callContextualizedEmbeddingItems(ctx context.Context, cfg completeTurnEmbeddingConfig, logs []store.ChatLog, items []contextualizedEmbeddingItem) (map[string]string, string, error) {
	if !usesVoyageContextualizedEmbedding(cfg) || len(items) == 0 {
		return nil, "", nil
	}
	turnLogs := map[int]map[string]string{}
	for _, log := range logs {
		role := strings.ToLower(strings.TrimSpace(log.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		if turnLogs[log.TurnIndex] == nil {
			turnLogs[log.TurnIndex] = map[string]string{}
		}
		turnLogs[log.TurnIndex][role] = appendUniqueTurnRoleText(turnLogs[log.TurnIndex][role], log.Content)
	}
	itemsByTurn := map[int][]contextualizedEmbeddingItem{}
	standalone := []contextualizedEmbeddingItem{}
	seen := map[string]bool{}
	for _, item := range items {
		if strings.TrimSpace(item.Key) == "" || seen[item.Key] {
			return nil, "", fmt.Errorf("contextualized embedding item key is empty or duplicated")
		}
		seen[item.Key] = true
		if !item.ContextTurnKnown {
			standalone = append(standalone, item)
			continue
		}
		itemsByTurn[item.TurnIndex] = append(itemsByTurn[item.TurnIndex], item)
	}
	turns := make([]int, 0, len(itemsByTurn))
	for turn := range itemsByTurn {
		turns = append(turns, turn)
	}
	sort.Ints(turns)
	out := make(map[string]string, len(items))
	model := ""
	for _, turn := range turns {
		turnItems := itemsByTurn[turn]
		needsEmbedding := false
		for _, item := range turnItems {
			if item.NeedsEmbedding {
				needsEmbedding = true
				break
			}
		}
		if !needsEmbedding {
			continue
		}
		inputs := make([]string, 0, len(turnItems)+2)
		for _, role := range []string{"user", "assistant"} {
			if text := strings.TrimSpace(turnLogs[turn][role]); text != "" {
				inputs = append(inputs, text)
			}
		}
		itemOffset := len(inputs)
		for _, item := range turnItems {
			inputs = append(inputs, item.Text)
		}
		grouped, resolvedModel, err := callDocumentEmbeddings(ctx, cfg, inputs)
		if err != nil {
			return nil, "", err
		}
		if strings.TrimSpace(resolvedModel) != "" {
			model = resolvedModel
		}
		for i, item := range turnItems {
			out[item.Key] = grouped[itemOffset+i]
		}
	}
	for _, item := range standalone {
		if !item.NeedsEmbedding {
			continue
		}
		grouped, resolvedModel, err := callDocumentEmbeddings(ctx, cfg, []string{item.Text})
		if err != nil {
			return nil, "", err
		}
		if strings.TrimSpace(resolvedModel) != "" {
			model = resolvedModel
		}
		out[item.Key] = grouped[0]
	}
	return out, model, nil
}

func adminReindexContextualizedEmbeddingItems(memories []store.Memory, evidence []store.DirectEvidence, worldRules []store.WorldRule, maxItems int, cfg completeTurnEmbeddingConfig, force, derivedNeedsEmbedding bool) []contextualizedEmbeddingItem {
	items := make([]contextualizedEmbeddingItem, 0, len(memories)+len(evidence)+len(worldRules))
	for _, mem := range memories {
		if text := reindexMemoryDocumentText(mem); text != "" {
			items = append(items, contextualizedEmbeddingItem{
				Key:              "memory:" + strconv.FormatInt(mem.ID, 10),
				Text:             text,
				TurnIndex:        mem.TurnIndex,
				ContextTurnKnown: true,
				NeedsEmbedding:   memoryNeedsEmbeddingForModel(mem, cfg, force),
			})
		}
	}
	evidenceCount, worldRuleCount := adminReindexDerivedArtifactCandidateCounts(maxItems, evidence, worldRules)
	for _, item := range evidence {
		if !adminEvidenceVectorEligible(item) || evidenceCount == 0 {
			continue
		}
		turnIndex := maxInt(item.TurnAnchor, item.SourceTurnEnd)
		items = append(items, contextualizedEmbeddingItem{
			Key:              "evidence:" + strconv.FormatInt(item.ID, 10),
			Text:             directEvidenceVectorDocumentText(item),
			TurnIndex:        turnIndex,
			ContextTurnKnown: turnIndex > 0,
			NeedsEmbedding:   derivedNeedsEmbedding,
		})
		evidenceCount--
	}
	for _, item := range worldRules {
		if !adminWorldRuleVectorEligible(item) || worldRuleCount == 0 {
			continue
		}
		items = append(items, contextualizedEmbeddingItem{
			Key:              "world_rule:" + strconv.FormatInt(item.ID, 10),
			Text:             worldRuleVectorDocumentText(item),
			TurnIndex:        item.SourceTurn,
			ContextTurnKnown: item.SourceTurn > 0,
			NeedsEmbedding:   derivedNeedsEmbedding,
		})
		worldRuleCount--
	}
	return items
}

func contextualizedEmbeddingItemsNeedEmbedding(items []contextualizedEmbeddingItem) bool {
	for _, item := range items {
		if item.NeedsEmbedding {
			return true
		}
	}
	return false
}

func memoryNeedsEmbeddingForModel(mem store.Memory, cfg completeTurnEmbeddingConfig, force bool) bool {
	embedding := strings.TrimSpace(mem.Embedding)
	return force || embedding == "" || embedding == "[]" ||
		(usesVoyageContextualizedEmbedding(cfg) &&
			!strings.EqualFold(strings.TrimSpace(mem.EmbeddingModel), strings.TrimSpace(cfg.Model)))
}

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
	result, err := s.runAdminReindexJob(r.Context(), sid, req, nil)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, "POST /admin/reindex")
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	result["background"] = false
	writeJSON(w, http.StatusOK, result)
}

type adminCanonicalMemoryReplayCandidate struct {
	Source            *store.MemorySourceRevision
	RequiresEmbedding bool
}

func (s *Server) adminCanonicalMemoryReplayCandidates(
	ctx context.Context,
	sid string,
	maxItems int,
) ([]adminCanonicalMemoryReplayCandidate, []int64, []string, error) {
	lister, ok := s.Store.(store.ActiveSourceRevisionLister)
	if !ok {
		return nil, nil, nil, store.ErrNotEnabled
	}
	sources, ok := s.Store.(store.SourceRevisionStore)
	if !ok {
		return nil, nil, nil, store.ErrNotEnabled
	}
	if _, ok := s.Store.(store.MemoryAdmissionWriter); !ok {
		return nil, nil, nil, store.ErrNotEnabled
	}
	lifecycle, ok := s.Store.(store.MemoryDerivationLifecycleAvailability)
	if !ok || !lifecycle.MemoryDerivationLifecycleEnabled() {
		return nil, nil, nil, store.ErrNotEnabled
	}
	if availability, ok := s.Store.(store.MemoryAdmissionWriteAvailability); ok &&
		!availability.MemoryAdmissionWritesEnabled() {
		return nil, nil, nil, store.ErrNotEnabled
	}
	active, err := lister.ListActiveSourceRevisions(ctx, sid, 0, 0)
	if err != nil {
		return nil, nil, nil, err
	}
	if maxItems > 0 && len(active) > maxItems {
		active = active[:maxItems]
	}
	candidates := make([]adminCanonicalMemoryReplayCandidate, 0, len(active))
	failedTurns := []int64{}
	errorsOut := []string{}
	for _, listed := range active {
		source, err := sources.GetSourceRevision(ctx, sid, listed.SourceRevision)
		if err != nil || source == nil {
			failedTurns = append(failedTurns, int64(listed.TurnIndex))
			if err == nil {
				err = store.ErrNotFound
			}
			errorsOut = append(errorsOut, fmt.Sprintf(
				"source_revision:%s read: %v", listed.SourceRevision, err,
			))
			continue
		}
		if source.LifecycleState != "active" || source.DerivedAdmissionState != "committed" {
			failedTurns = append(failedTurns, int64(source.TurnIndex))
			errorsOut = append(errorsOut, fmt.Sprintf(
				"source_revision:%s is not an active committed admission", source.SourceRevision,
			))
			continue
		}
		extraction, present, failure := storedMemoryAdmissionExtraction(source)
		if !present || failure != "" {
			if failure == "" {
				failure = "committed_derived_result_version_unsupported"
			}
			failedTurns = append(failedTurns, int64(source.TurnIndex))
			errorsOut = append(errorsOut, fmt.Sprintf(
				"source_revision:%s %s", source.SourceRevision, failure,
			))
			continue
		}
		candidates = append(candidates, adminCanonicalMemoryReplayCandidate{
			Source:            source,
			RequiresEmbedding: buildPublicMemoryProjection(extraction, "").Eligible,
		})
	}
	return candidates, failedTurns, errorsOut, nil
}

func (s *Server) replayAdminCanonicalMemories(
	ctx context.Context,
	cfg completeTurnExtractionConfig,
	candidates []adminCanonicalMemoryReplayCandidate,
	reconcileEligibility bool,
	progress adminJobProgressFunc,
	totalCandidates int,
) (processed, completed, vectorQueued, skipped int, failedTurns []int64, errorsOut []string) {
	for _, candidate := range candidates {
		if ctx.Err() != nil {
			errorsOut = append(errorsOut, "canonical memory replay canceled: "+ctx.Err().Error())
			break
		}
		processed++
		replayCtx := store.WithMemoryAdmissionVectorReplay(ctx, true, reconcileEligibility)
		result := s.processAcceptedSourceRevisionWithOptions(
			replayCtx,
			candidate.Source,
			cfg,
			acceptedSourceDerivationOptions{CommittedReplayOnly: true},
		)
		if result.State == "completed" {
			completed++
			// A later world-rule projection can replace the shared VectorStatus
			// after admission queued durable operations. Projection counters remain
			// admission-owned evidence that a configured replay has outbox work.
			admissionProjected := result.SaveResult.Memories > 0 ||
				result.SaveResult.Evidence > 0 || result.SaveResult.PreciseMemoryUnits > 0
			if result.SaveResult.VectorStatus == "queued" ||
				(reconcileEligibility && admissionProjected) {
				vectorQueued++
			}
		} else {
			skipped++
			failedTurns = append(failedTurns, int64(candidate.Source.TurnIndex))
			errorsOut = append(errorsOut, fmt.Sprintf(
				"source_revision:%s %s: %s",
				candidate.Source.SourceRevision, result.State, result.Failure,
			))
		}
		if progress != nil {
			p := adminReindexProgress(
				processed, totalCandidates, 0, skipped,
				failedTurns, nil, int64(candidate.Source.TurnIndex), errorsOut,
			)
			p["stage"] = "canonical_memory_replay"
			p["tier"] = "memory_evidence_precise"
			p["canonical_replays_completed"] = completed
			p["vector_replays_queued"] = vectorQueued
			progress(p)
		}
	}
	return processed, completed, vectorQueued, skipped, failedTurns, errorsOut
}

func (s *Server) runAdminReindexJob(ctx context.Context, sid string, req map[string]any, progress adminJobProgressFunc) (map[string]any, error) {
	maxItems := intFromAny(req["max_items"], 0)
	if maxItems < 0 {
		maxItems = 0
	}
	batchSize := intFromAny(req["batch_size"], 0)
	if batchSize < 0 {
		batchSize = 0
	}
	force := completeTurnBoolFromAny(req["force"])
	dryRun := completeTurnBoolFromAny(req["dry_run"])
	background := completeTurnBoolFromAny(req["background"])
	meta := mapFromAny(req["client_meta"])
	cfg := s.completeTurnExtractionConfig(meta)
	staleDeleteOperationsRejected := int64(0)
	if !dryRun {
		if maintenance, ok := s.Store.(store.MemoryVectorOutboxMaintenanceStore); ok {
			rejected, cleanupErr := maintenance.CoalesceInactiveMemoryVectorDeleteOperations(ctx, sid, time.Now().UTC())
			if cleanupErr != nil && !errors.Is(cleanupErr, store.ErrNotEnabled) {
				return nil, cleanupErr
			}
			staleDeleteOperationsRejected = rejected
		}
	}

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
	if !dryRun && !force && boolFromAny(preIntegrity["vector_index_current"]) {
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
			"background":             background,
			"note":                   "reindex skipped because the canonical vector candidate count and stored ChromaDB documents are already current",
		}
		result["duplicate_deletes_rejected"] = staleDeleteOperationsRejected
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
	publicMemories := make([]store.Memory, 0, len(memories))
	for _, mem := range memories {
		if strings.TrimSpace(reindexMemoryDocumentText(mem)) != "" {
			publicMemories = append(publicMemories, mem)
		}
	}
	derivedEvidenceCandidates, derivedWorldRuleCandidates := adminReindexDerivedArtifactCandidateCounts(maxItems, allEvidence, allWorldRules)
	canonicalCandidates := []adminCanonicalMemoryReplayCandidate{}
	invalidSourceTurns := []int64{}
	sourceErrors := []string{}
	canonicalOwnerUnavailable := false
	if !dryRun {
		var err error
		canonicalCandidates, invalidSourceTurns, sourceErrors, err =
			s.adminCanonicalMemoryReplayCandidates(ctx, sid, maxItems)
		if errors.Is(err, store.ErrNotEnabled) {
			canonicalOwnerUnavailable = true
			if len(allMemories) > 0 || len(allEvidence) > 0 {
				invalidSourceTurns = append(invalidSourceTurns, 0)
				sourceErrors = append(sourceErrors, "canonical source revision/admission owner is unavailable")
			}
		} else if err != nil {
			return nil, err
		}
		// When the unbounded inventory contains both canonical sources and
		// legacy/manual artifacts, do not let the valid sources hide rows that
		// have no source revision owner. Those rows are never sent directly to
		// Chroma; report their turn explicitly for repair instead.
		if len(canonicalCandidates) > 0 {
			coveredTurns := make(map[int64]struct{}, len(canonicalCandidates))
			for _, candidate := range canonicalCandidates {
				coveredTurns[int64(candidate.Source.TurnIndex)] = struct{}{}
			}
			reportedTurns := make(map[int64]struct{}, len(invalidSourceTurns))
			for _, turn := range invalidSourceTurns {
				reportedTurns[turn] = struct{}{}
			}
			markUnmatched := func(turn int64, artifact string, id int64) {
				if _, ok := coveredTurns[turn]; ok {
					return
				}
				sourceErrors = append(sourceErrors, fmt.Sprintf(
					"%s:%d has no active committed source revision", artifact, id,
				))
				if _, reported := reportedTurns[turn]; !reported {
					reportedTurns[turn] = struct{}{}
					invalidSourceTurns = append(invalidSourceTurns, turn)
				}
			}
			for _, memory := range allMemories {
				if _, covered := coveredTurns[int64(memory.TurnIndex)]; maxItems > 0 && !covered {
					continue
				}
				markUnmatched(int64(memory.TurnIndex), "memory", memory.ID)
			}
			for _, item := range allEvidence {
				turn := item.TurnAnchor
				if turn == 0 {
					turn = item.SourceTurnEnd
				}
				if turn == 0 {
					turn = item.SourceTurnStart
				}
				if _, covered := coveredTurns[int64(turn)]; maxItems > 0 && !covered {
					continue
				}
				captureStage := strings.TrimSpace(item.CaptureStage)
				if captureStage != "" && captureStage != "critic_extract" {
					sourceErrors = append(sourceErrors, fmt.Sprintf(
						"evidence:%d has unsupported capture_stage %q for canonical admission replay",
						item.ID, captureStage,
					))
					if _, reported := reportedTurns[int64(turn)]; !reported {
						reportedTurns[int64(turn)] = struct{}{}
						invalidSourceTurns = append(invalidSourceTurns, int64(turn))
					}
					continue
				}
				markUnmatched(int64(turn), "evidence", item.ID)
			}
		}
	}
	canonicalReplayNeedsEmbedding := false
	for _, candidate := range canonicalCandidates {
		if candidate.RequiresEmbedding {
			canonicalReplayNeedsEmbedding = true
			break
		}
	}
	preflightEvidenceCandidates := derivedEvidenceCandidates
	preflightMemories := publicMemories
	if !dryRun {
		// Evidence mutation is part of canonical admission replay. It is never
		// embedded again through the legacy derived-artifact loop. Once canonical
		// replay owns memory/evidence, its public projections also own the embedding
		// decision: legacy rows must not hide an invalid source or block a private
		// delete replay.
		preflightEvidenceCandidates = 0
		if !canonicalOwnerUnavailable {
			preflightMemories = nil
		}
	}
	if block := adminReindexEmbeddingPreflightBlock(
		sid, cfg, meta, force, dryRun, maxItems, batchSize, preflightMemories,
		len(canonicalCandidates), canonicalReplayNeedsEmbedding,
		preflightEvidenceCandidates, derivedWorldRuleCandidates, preIntegrity,
	); block != nil {
		if progress != nil {
			progress(block)
		}
		return block, nil
	}
	totalCandidates := len(canonicalCandidates) + len(invalidSourceTurns) + derivedWorldRuleCandidates
	if dryRun {
		totalCandidates = len(publicMemories) + derivedEvidenceCandidates + derivedWorldRuleCandidates
	} else if len(canonicalCandidates) == 0 && len(invalidSourceTurns) == 0 &&
		(len(allMemories) > 0 || len(allEvidence) > 0) {
		invalidSourceTurns = append(invalidSourceTurns, 0)
		sourceErrors = append(sourceErrors, "canonical source revision is missing for memory/evidence reindex")
		totalCandidates++
	}
	if progress != nil {
		progress(map[string]any{
			"status":                 "running",
			"stage":                  "canonical_memory_replay",
			"tier":                   "memory_evidence_precise",
			"candidate_count":        totalCandidates,
			"memory_candidates":      len(canonicalCandidates),
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

	processed := len(invalidSourceTurns)
	upserted := 0
	canonicalReplaysCompleted := 0
	vectorReplaysQueued := 0
	skipped := len(invalidSourceTurns)
	errorsOut := append([]string{}, sourceErrors...)
	failedTurns := append([]int64{}, invalidSourceTurns...)
	failedIDs := []int64{}
	skippedIDs := []int64{}
	contextMemoryEmbeddings := map[string]string(nil)
	contextMemoryErr := error(nil)
	if !dryRun && cfg.Embedder.hasConfig() && usesVoyageContextualizedEmbedding(cfg.Embedder) {
		derivedNeedsEmbedding := s.Vector != nil && strings.TrimSpace(s.Cfg.ChromaEndpoint) != ""
		// Canonical admission constructs its own public-only contextual chunks.
		// World rules remain an independent tier and are embedded without replaying
		// raw user/assistant chat logs into the administrator's Voyage request.
		items := adminReindexContextualizedEmbeddingItems(nil, nil, allWorldRules, maxItems, cfg.Embedder, force, derivedNeedsEmbedding)
		if contextualizedEmbeddingItemsNeedEmbedding(items) {
			contextMemoryEmbeddings, _, contextMemoryErr = callContextualizedEmbeddingItems(ctx, cfg.Embedder, nil, items)
		}
	}
	if !dryRun {
		replayProcessed, replayCompleted, replayVectorQueued, replaySkipped, replayFailed, replayErrors :=
			s.replayAdminCanonicalMemories(
				ctx, cfg, canonicalCandidates,
				s.Vector != nil && strings.TrimSpace(s.Cfg.ChromaEndpoint) != "",
				progress, totalCandidates,
			)
		processed += replayProcessed
		canonicalReplaysCompleted += replayCompleted
		vectorReplaysQueued += replayVectorQueued
		skipped += replaySkipped
		failedTurns = append(failedTurns, replayFailed...)
		errorsOut = append(errorsOut, replayErrors...)
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
	derivedEvidence := []store.DirectEvidence(nil)
	if dryRun {
		derivedEvidence = allEvidence
	}
	artifactResult := s.adminReindexDerivedArtifacts(ctx, sid, cfg, dryRun, maxItems, derivedEvidence, allWorldRules, contextMemoryEmbeddings, contextMemoryErr, artifactProgress)
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
		processedBatches = 1
		if batchSize > 0 {
			processedBatches = (processed + batchSize - 1) / batchSize
		}
	}
	qualityStatus := "not_run"
	if dryRun {
		qualityStatus = "dry_run"
	} else if vectorReplaysQueued > 0 {
		qualityStatus = "pending_outbox_readback"
	} else if upserted > 0 {
		qualityStatus = "requires_before_after_report"
	}
	integrityReport := preIntegrity
	var postIntegrity map[string]any
	if !dryRun {
		if refreshedMemories, err := s.Store.ListMemories(ctx, sid, 0, 0); err == nil {
			allMemories = refreshedMemories
		}
		if refreshedEvidence, err := s.Store.ListEvidence(ctx, sid); err == nil {
			allEvidence = refreshedEvidence
		}
		if refreshedWorldRules, err := s.Store.ListWorldRules(ctx, sid); err == nil {
			allWorldRules = refreshedWorldRules
		}
		postIntegrity = s.adminReindexIntegrityReport(ctx, sid, allMemories, allEvidence, allWorldRules, strings.TrimSpace(cfg.Embedder.Model))
		integrityReport = postIntegrity
	}
	if vectorReplaysQueued > 0 {
		postIntegrity = nil
		integrityReport = preIntegrity
	}
	postIntegrityStatus := "observed"
	if dryRun {
		postIntegrityStatus = "dry_run"
	} else if vectorReplaysQueued > 0 {
		postIntegrityStatus = "pending_outbox_readback"
	}
	now := time.Now().UTC()
	s.saveAuditLogBestEffort(ctx, &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "admin_reindex",
		TargetType:    adminAuditTargetType(sid),
		TargetID:      0,
		Summary:       "Admin reindex requested",
		DetailsJSON: mustCompactJSON(map[string]any{
			"background":                  background,
			"request_keys":                adminAuditRequestKeys(req),
			"dry_run":                     dryRun,
			"force":                       force,
			"batch_size":                  batchSize,
			"max_items":                   maxItems,
			"candidates":                  totalCandidates,
			"processed":                   processed,
			"processed_batches":           processedBatches,
			"upserted":                    upserted,
			"canonical_replays_completed": canonicalReplaysCompleted,
			"vector_replays_queued":       vectorReplaysQueued,
			"duplicate_deletes_rejected":  staleDeleteOperationsRejected,
			"skipped":                     skipped,
			"embedding_model":             strings.TrimSpace(cfg.Embedder.Model),
			"embedding_provider":          strings.TrimSpace(cfg.Embedder.Provider),
			"embedding_configured":        cfg.Embedder.hasConfig(),
			"embedding_missing_fields":    cfg.Embedder.missingFields(),
			"embedding_config_trace":      adminEmbeddingConfigTrace(meta, cfg),
			"failed_ids":                  failedIDs,
			"failed_turns":                failedTurns,
			"skipped_ids":                 skippedIDs,
			"derived_artifact_reindex":    artifactResult.Summary(),
			"errors":                      errorsOut,
			"integrity_report":            integrityReport,
			"pre_reindex_integrity":       preIntegrity,
			"post_reindex_integrity":      postIntegrity,
			"quality_verification": map[string]any{
				"status":               qualityStatus,
				"required_for_cutover": true,
			},
		}),
		Source:    s.storeWriteSource(),
		CreatedAt: now,
	})
	result := map[string]any{
		"status":                      "ok",
		"source":                      s.storeWriteSource(),
		"chat_session_id":             sid,
		"mutation_enabled":            true,
		"reindex_executed":            !dryRun && (canonicalReplaysCompleted > 0 || upserted > 0),
		"dry_run":                     dryRun,
		"force":                       force,
		"batch_size":                  batchSize,
		"max_items":                   maxItems,
		"candidates":                  totalCandidates,
		"processed":                   processed,
		"processed_batches":           processedBatches,
		"upserted":                    upserted,
		"canonical_replays_completed": canonicalReplaysCompleted,
		"vector_replays_queued":       vectorReplaysQueued,
		"duplicate_deletes_rejected":  staleDeleteOperationsRejected,
		"vector_delivery_pending":     vectorReplaysQueued > 0,
		"post_integrity_status":       postIntegrityStatus,
		"skipped":                     skipped,
		"embedding_model":             strings.TrimSpace(cfg.Embedder.Model),
		"embedding_provider":          strings.TrimSpace(cfg.Embedder.Provider),
		"embedding_configured":        cfg.Embedder.hasConfig(),
		"embedding_missing_fields":    cfg.Embedder.missingFields(),
		"embedding_config_trace":      adminEmbeddingConfigTrace(meta, cfg),
		"failed_ids":                  failedIDs,
		"failed_turns":                failedTurns,
		"skipped_ids":                 skippedIDs,
		"derived_artifact_reindex":    artifactResult.Summary(),
		"errors":                      errorsOut,
		"integrity_report":            integrityReport,
		"pre_reindex_integrity":       preIntegrity,
		"post_reindex_integrity":      postIntegrity,
		"quality_verification": map[string]any{
			"status":                qualityStatus,
			"required_for_cutover":  true,
			"before_after_required": true,
			"report_scope":          "search quality before/after reindex",
		},
		"audit_written": true,
		"changed_at":    now,
		"background":    background,
		"note":          "reindex replayed canonical committed memory admissions through the durable vector outbox and retained world-rule maintenance",
	}
	if progress != nil {
		progress(map[string]any{
			"status":           "completed",
			"candidate_count":  totalCandidates,
			"processed":        processed,
			"upserted":         upserted,
			"skipped_count":    skipped,
			"failed_count":     len(failedIDs) + len(failedTurns),
			"failed_ids":       failedIDs,
			"failed_turns":     failedTurns,
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
	eligibleMemoryCount := 0
	for _, mem := range memories {
		if strings.TrimSpace(reindexMemoryDocumentText(mem)) == "" {
			continue
		}
		eligibleMemoryCount++
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
	canonicalMemoryCount := eligibleMemoryCount
	eligiblePreciseMemoryCount := 0
	canonicalVectorCandidateCount := canonicalMemoryCount + eligibleEvidenceCount + eligibleWorldRuleCount
	canonicalInventoryAvailable := false
	canonicalInventoryError := ""
	canonicalIDs := map[string]bool{}
	canonicalCounts := map[string]int{}
	if s != nil && s.Store != nil {
		var err error
		canonicalIDs, canonicalCounts, err = s.adminCanonicalVectorReferences(ctx, sid)
		if err != nil {
			canonicalInventoryError = err.Error()
		} else {
			canonicalInventoryAvailable = true
			canonicalMemoryCount = canonicalCounts["memories"]
			eligibleEvidenceCount = canonicalCounts["direct_evidence_records"]
			eligiblePreciseMemoryCount = canonicalCounts["precise_memory_units"]
			eligibleWorldRuleCount = canonicalCounts["world_rules"]
			canonicalVectorCandidateCount = 0
			for _, count := range canonicalCounts {
				canonicalVectorCandidateCount += count
			}
		}
	}

	vectorConfigured := s != nil && s.Vector != nil && strings.TrimSpace(s.Cfg.ChromaEndpoint) != ""
	vectorStatus := "not_configured"
	vectorCount := 0
	vectorCountKnown := false
	vectorCountErr := ""
	managedVectorCount := 0
	managedVectorCountKnown := false
	managedVectorListingError := ""
	matchedCanonicalVectorCount := 0
	managedOrphanCount := 0
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
		if canonicalInventoryAvailable {
			if lister, ok := s.Vector.(vector.DocumentLister); ok {
				docs, err := lister.ListDocuments(ctx, sid)
				if err != nil {
					managedVectorListingError = err.Error()
				} else {
					managedVectorCountKnown = true
					for _, doc := range docs {
						if adminManagedVectorTier(doc) == "" {
							continue
						}
						managedVectorCount++
						id := strings.TrimSpace(doc.ID)
						if canonicalIDs[id] {
							matchedCanonicalVectorCount++
						} else {
							managedOrphanCount++
						}
					}
				}
			}
		}
	}

	missingVectorEstimate := 0
	extraVectorEstimate := 0
	comparisonVectorCount := vectorCount
	comparisonVectorCountKnown := vectorCountKnown
	if managedVectorCountKnown {
		comparisonVectorCount = matchedCanonicalVectorCount
		comparisonVectorCountKnown = true
		extraVectorEstimate = managedOrphanCount
		if duplicateManagedCount := matchedCanonicalVectorCount - canonicalVectorCandidateCount; duplicateManagedCount > 0 {
			extraVectorEstimate += duplicateManagedCount
		}
	}
	if comparisonVectorCountKnown {
		if canonicalVectorCandidateCount > comparisonVectorCount {
			missingVectorEstimate = canonicalVectorCandidateCount - comparisonVectorCount
		} else if !managedVectorCountKnown && comparisonVectorCount > canonicalVectorCandidateCount {
			extraVectorEstimate = comparisonVectorCount - canonicalVectorCandidateCount
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
	if canonicalInventoryError != "" {
		reasons = append(reasons, "canonical_vector_inventory_error")
	}
	if managedVectorListingError != "" {
		reasons = append(reasons, "managed_vector_listing_error")
	} else if vectorConfigured && canonicalInventoryAvailable && !managedVectorCountKnown {
		reasons = append(reasons, "managed_vector_listing_unavailable")
	}
	if comparisonVectorCountKnown && comparisonVectorCount < canonicalMemoryCount {
		reasons = append(reasons, "vector_count_below_canonical_memory_count")
	}
	if comparisonVectorCountKnown && comparisonVectorCount < canonicalVectorCandidateCount {
		reasons = append(reasons, "vector_count_below_canonical_vector_candidate_count")
	}
	if managedVectorCountKnown && managedOrphanCount > 0 {
		reasons = append(reasons, "managed_vector_orphans_present")
	} else if comparisonVectorCountKnown && comparisonVectorCount > canonicalVectorCandidateCount {
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
	vectorIndexCurrent := vectorConfigured && managedVectorCountKnown &&
		comparisonVectorCount == canonicalVectorCandidateCount &&
		extraVectorEstimate == 0 && len(reasons) == 0
	return map[string]any{
		"policy_version":                        adminReindexIntegrityPolicyVersion,
		"status":                                status,
		"chat_session_id":                       sid,
		"canonical_memory_count":                canonicalMemoryCount,
		"canonical_evidence_vector_count":       eligibleEvidenceCount,
		"canonical_precise_memory_vector_count": eligiblePreciseMemoryCount,
		"canonical_world_rule_vector_count":     eligibleWorldRuleCount,
		"canonical_vector_candidate_count":      canonicalVectorCandidateCount,
		"vector_configured":                     vectorConfigured,
		"vector_status":                         vectorStatus,
		"vector_health":                         vectorHealth,
		"vector_count":                          vectorCount,
		"vector_count_known":                    vectorCountKnown,
		"vector_count_error":                    nilIfEmpty(vectorCountErr),
		"canonical_inventory_available":         canonicalInventoryAvailable,
		"canonical_inventory_error":             nilIfEmpty(canonicalInventoryError),
		"canonical_counts":                      canonicalCounts,
		"managed_vector_count":                  managedVectorCount,
		"managed_vector_count_known":            managedVectorCountKnown,
		"managed_vector_listing_error":          nilIfEmpty(managedVectorListingError),
		"matched_canonical_vector_count":        matchedCanonicalVectorCount,
		"managed_orphan_count":                  managedOrphanCount,
		"vector_count_matches_canonical":        vectorIndexCurrent,
		"vector_index_current":                  vectorIndexCurrent,
		"missing_vector_count_estimate":         missingVectorEstimate,
		"extra_vector_count_estimate":           extraVectorEstimate,
		"missing_embedding_count":               len(missingEmbeddingIDs),
		"missing_embedding_ids":                 missingEmbeddingIDs,
		"expected_embedding_model":              expectedEmbeddingModel,
		"observed_embedding_models":             observedModels,
		"embedding_model_mismatch_count":        len(modelMismatchIDs),
		"embedding_model_mismatch_ids":          modelMismatchIDs,
		"reindex_recommended":                   len(reasons) > 0,
		"reindex_reasons":                       reasons,
		"reembed_recommended":                   len(reembedReasons) > 0,
		"reembed_reasons":                       reembedReasons,
		"index_usable_for_vector_first_read":    vectorIndexCurrent && canonicalVectorCandidateCount > 0,
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

func adminReindexEmbeddingPreflightBlock(sid string, cfg completeTurnExtractionConfig, meta map[string]any, force, dryRun bool, maxItems, batchSize int, memories []store.Memory, canonicalCandidates int, canonicalReplayNeedsEmbedding bool, evidenceCandidates, worldRuleCandidates int, integrity map[string]any) map[string]any {
	if dryRun {
		return nil
	}
	if !adminReindexNeedsEmbedding(force, memories, canonicalReplayNeedsEmbedding, evidenceCandidates, worldRuleCandidates) {
		return nil
	}
	if cfg.Embedder.hasConfig() {
		return nil
	}
	reason := "missing_embedding_config"
	if strings.Contains(strings.TrimSpace(cfg.Embedder.Source), "partial") {
		reason = "embedding_config_incomplete"
	}
	memoryCandidates := len(memories)
	if canonicalCandidates > 0 {
		memoryCandidates = canonicalCandidates
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
		"candidate_count":          memoryCandidates + evidenceCandidates + worldRuleCandidates,
		"memory_candidates":        memoryCandidates,
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

func adminReindexNeedsEmbedding(force bool, memories []store.Memory, canonicalReplayNeedsEmbedding bool, evidenceCandidates, worldRuleCandidates int) bool {
	if canonicalReplayNeedsEmbedding || evidenceCandidates > 0 || worldRuleCandidates > 0 {
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
	return strings.TrimSpace(memorySearchTextFromMemory(mem).Text)
}
