package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

type adminRescanRequest struct {
	ChatSessionID      string         `json:"chat_session_id"`
	MaxItems           int            `json:"max_items"`
	TurnIndices        []int          `json:"turn_indices"`
	ClientMeta         map[string]any `json:"client_meta"`
	DryRun             bool           `json:"dry_run"`
	Background         bool           `json:"background"`
	CanonicalRawReplay bool           `json:"-"`
}

func (s *Server) runAdminRescan(ctx context.Context, sid string, req adminRescanRequest) (map[string]any, error) {
	return s.runAdminRescanWithProgress(ctx, sid, req, nil)
}

func (s *Server) runAdminRescanWithProgress(ctx context.Context, sid string, req adminRescanRequest, progress adminJobProgressFunc) (map[string]any, error) {
	maxItems := req.MaxItems

	logs, err := s.Store.ListChatLogs(ctx, sid, 0, 0)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	memories, err := s.Store.ListMemories(ctx, sid, 0, 0)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}

	targetTurns := map[int]bool{}
	for _, turn := range req.TurnIndices {
		if turn >= 0 {
			targetTurns[turn] = true
		}
	}
	forceWorldRuleBackfill := boolFromAny(req.ClientMeta["force_world_rule_backfill"]) ||
		boolFromAny(req.ClientMeta["force_focused_world_rule_audit"])
	fullSessionBackfill := boolFromAny(req.ClientMeta["full_session_backfill"]) ||
		boolFromAny(req.ClientMeta["session_normalize_full_session_backfill"])
	forceRawWorldRuleAudit := boolFromAny(req.ClientMeta["force_raw_world_rule_audit"]) ||
		boolFromAny(req.ClientMeta["force_focused_world_rule_audit"]) ||
		fullSessionBackfill
	forceDerivedRebuild := boolFromAny(req.ClientMeta["force_derived_rebuild"]) ||
		boolFromAny(req.ClientMeta["derived_backfill_only"])
	memoryTurns := map[int]bool{}
	for _, mem := range memories {
		if mem.ChatSessionID == sid && mem.TurnIndex >= 0 {
			memoryTurns[mem.TurnIndex] = true
		}
	}
	turnLogs := map[int]map[string]string{}
	for _, log := range logs {
		if log.ChatSessionID != sid || log.TurnIndex < 0 {
			continue
		}
		if len(targetTurns) > 0 && !targetTurns[log.TurnIndex] {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(log.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		if turnLogs[log.TurnIndex] == nil {
			turnLogs[log.TurnIndex] = map[string]string{}
		}
		turnLogs[log.TurnIndex][role] = appendUniqueTurnRoleText(turnLogs[log.TurnIndex][role], log.Content)
	}

	turns := []int{}
	for turn, roleMap := range turnLogs {
		if memoryTurns[turn] && !forceDerivedRebuild && !req.CanonicalRawReplay {
			continue
		}
		if strings.TrimSpace(roleMap["user"]) == "" && strings.TrimSpace(roleMap["assistant"]) == "" {
			continue
		}
		turns = append(turns, turn)
	}
	turns = uniqueSortedNonNegativeInts(turns)
	if maxItems > 0 && len(turns) > maxItems {
		turns = turns[:maxItems]
	}
	if progress != nil {
		progress(map[string]any{
			"status":             "running",
			"stage":              "candidate_scan",
			"candidate_count":    len(turns),
			"processed":          0,
			"succeeded":          0,
			"failed_count":       0,
			"skipped_count":      0,
			"processed_turns":    []int{},
			"failed_turns":       []map[string]any{},
			"skipped_turns":      []map[string]any{},
			"progress_percent":   0,
			"foreground_timeout": false,
			"timeout_policy":     "background_job_detached_from_http_request",
		})
	}

	extractionCfg := s.completeTurnExtractionConfig(req.ClientMeta)
	llmTrace := completeTurnLLMConfigTrace(extractionCfg)
	failedTurns := []map[string]any{}
	skippedTurns := []map[string]any{}
	deferredTurns := []map[string]any{}
	processedTurns := []int{}
	succeeded := 0
	failed := 0
	skipped := 0
	deferred := 0
	artifactCounts := map[string]int{
		"memories":                   0,
		"evidence":                   0,
		"kg_triples":                 0,
		"subjective_entity_memories": 0,
		"character_events":           0,
		"storylines":                 0,
		"world_rules":                0,
		"character_states":           0,
		"pending_threads":            0,
		"active_states":              0,
		"entities":                   0,
		"trust_states":               0,
		"episode_summaries":          0,
		"chapter_summaries":          0,
		"arc_summaries":              0,
		"saga_digests":               0,
		"vectors_upserted":           0,
	}
	warnings := []string{}
	episodeInterval := normalizedEpisodeInterval(intFromAny(req.ClientMeta["episode_interval_turns"], 0))
	forceEpisodeBackfill := boolFromAny(req.ClientMeta["force_episode_backfill"])
	episodeBackfill := skippedEpisodeBackfillResult(req.DryRun, episodeInterval, forceEpisodeBackfill, "not_run")
	worldRuleBackfill := skippedWorldRuleBackfillResult(req.DryRun, "not_run")
	hierarchyBackfill := skippedHierarchyBackfillResult(req.DryRun, "not_run")
	runBackfills := func(runLogs []store.ChatLog, runMemories []store.Memory, runEvidence []store.DirectEvidence, runTargets map[int]bool) {
		if progress != nil {
			progress(map[string]any{"stage": "episode_backfill", "candidate_count": len(turns)})
		}
		episodeBackfill = s.backfillEpisodeSummariesFromChatLogs(ctx, sid, runLogs, runMemories, runEvidence, episodeInterval, req.DryRun, runTargets, forceEpisodeBackfill)
		artifactCounts["episode_summaries"] += intFromAny(episodeBackfill["generated"], 0)
		if errText := strings.TrimSpace(stringFromMap(episodeBackfill, "error")); errText != "" {
			warnings = append(warnings, "episode_backfill_failed: "+errText)
		}
		if progress != nil {
			progress(map[string]any{"stage": "world_rule_backfill", "episode_backfill": episodeBackfill})
		}
		worldRuleBackfill = s.backfillWorldRulesFromMemories(ctx, sid, runMemories, runTargets, req.DryRun)
		artifactCounts["world_rules"] += intFromAny(worldRuleBackfill["generated"], 0)
		if errText := strings.TrimSpace(stringFromMap(worldRuleBackfill, "error")); errText != "" {
			warnings = append(warnings, "world_rule_backfill_failed: "+errText)
		}
		shouldRunRawWorldAudit := forceWorldRuleBackfill &&
			(forceRawWorldRuleAudit || (artifactCounts["world_rules"] == 0 && intFromAny(worldRuleBackfill["generated"], 0) == 0))
		if shouldRunRawWorldAudit {
			rawWorldRuleBackfill := s.backfillWorldRulesFromChatLogs(ctx, sid, runLogs, runTargets, req.DryRun, extractionCfg.Critic, progress)
			worldRuleBackfill = mergeWorldRuleBackfillResults(worldRuleBackfill, rawWorldRuleBackfill)
			artifactCounts["world_rules"] += intFromAny(rawWorldRuleBackfill["generated"], 0)
			if errText := strings.TrimSpace(stringFromMap(rawWorldRuleBackfill, "error")); errText != "" {
				warnings = append(warnings, "raw_world_rule_backfill_failed: "+errText)
			}
		}
		if progress != nil {
			progress(map[string]any{"stage": "hierarchy_backfill", "episode_backfill": episodeBackfill, "world_rule_backfill": worldRuleBackfill})
		}
		hierarchyBackfill = s.backfillHierarchySummaries(ctx, sid, runLogs, runTargets, req.ClientMeta, req.DryRun)
		artifactCounts["chapter_summaries"] += intFromAny(mapFromAny(hierarchyBackfill["chapter"])["generated"], 0)
		artifactCounts["arc_summaries"] += intFromAny(mapFromAny(hierarchyBackfill["arc"])["generated"], 0)
		artifactCounts["saga_digests"] += intFromAny(mapFromAny(hierarchyBackfill["saga"])["generated"], 0)
		if errText := strings.TrimSpace(stringFromMap(hierarchyBackfill, "error")); errText != "" {
			warnings = append(warnings, "hierarchy_backfill_failed: "+errText)
		}
		if progress != nil {
			progress(map[string]any{"stage": "backfill_done", "episode_backfill": episodeBackfill, "world_rule_backfill": worldRuleBackfill, "hierarchy_backfill": hierarchyBackfill})
		}
	}
	episodeBackfillOnly := boolFromAny(req.ClientMeta["episode_backfill_only"])
	if episodeBackfillOnly {
		runBackfills(logs, memories, nil, targetTurns)
		return map[string]any{
			"status":                "ok",
			"source":                s.storeWriteSource(),
			"chat_session_id":       sid,
			"dry_run":               req.DryRun,
			"episode_backfill_only": true,
			"candidate_count":       0,
			"succeeded":             0,
			"failed":                0,
			"skipped":               0,
			"processed_turns":       []int{},
			"failed_turns":          []map[string]any{},
			"skipped_turns":         []map[string]any{},
			"artifact_counts":       artifactCounts,
			"episode_backfill":      episodeBackfill,
			"world_rule_backfill":   worldRuleBackfill,
			"hierarchy_backfill":    hierarchyBackfill,
			"warnings":              warnings,
			"llm_config_trace":      llmTrace,
			"note":                  "rescan ran episode/world-rule backfill only and did not reprocess Critic-derived artifacts",
		}, nil
	}

	if len(turns) == 0 {
		runBackfills(logs, memories, nil, targetTurns)
		return map[string]any{
			"status":              "ok",
			"source":              s.storeWriteSource(),
			"chat_session_id":     sid,
			"dry_run":             req.DryRun,
			"candidate_count":     0,
			"succeeded":           0,
			"failed":              0,
			"skipped":             0,
			"processed_turns":     []int{},
			"failed_turns":        []map[string]any{},
			"skipped_turns":       []map[string]any{},
			"artifact_counts":     artifactCounts,
			"episode_backfill":    episodeBackfill,
			"world_rule_backfill": worldRuleBackfill,
			"hierarchy_backfill":  hierarchyBackfill,
			"llm_config_trace":    llmTrace,
			"note":                "rescan found no raw chat_log turns missing memory for this session/target set",
		}, nil
	}

	if availability, ok := s.Store.(store.MemoryDerivationLifecycleAvailability); ok &&
		availability.MemoryDerivationLifecycleEnabled() {
		lister, listOK := s.Store.(store.ActiveSourceRevisionLister)
		sourceWriter, writerOK := s.Store.(store.SourceRevisionStore)
		queue, queueOK := s.Store.(store.MemoryReprocessingJobStore)
		if !listOK || !writerOK || !queueOK {
			return nil, fmt.Errorf("durable rescan queue is unavailable")
		}
		sources, err := lister.ListActiveSourceRevisions(ctx, sid, 0, 0)
		if err != nil {
			return nil, err
		}
		sourcesByTurn := map[int][]store.MemorySourceRevision{}
		for _, source := range sources {
			sourcesByTurn[source.TurnIndex] = append(sourcesByTurn[source.TurnIndex], source)
		}
		queued := 0
		reopened := 0
		now := time.Now().UTC()
		for _, turn := range turns {
			roleMap := turnLogs[turn]
			candidates := sourcesByTurn[turn]
			if len(candidates) == 0 && req.CanonicalRawReplay {
				if req.DryRun {
					skipped++
					skippedTurns = append(skippedTurns, map[string]any{
						"turn_index": turn,
						"reason":     "canonical_raw_source_registration_required",
					})
					if progress != nil {
						progressValue := adminRescanProgress(
							succeeded+failed+skipped+deferred, len(turns), succeeded, failed,
							skipped, processedTurns, failedTurns, skippedTurns,
							artifactCounts, turn, "canonical_raw_source_registration_required",
						)
						progressValue["deferred_count"] = deferred
						progressValue["deferred_turns"] = append([]map[string]any{}, deferredTurns...)
						progress(progressValue)
					}
					continue
				}
				source := adminRescanCanonicalRawSourceRevision(
					sid,
					turn,
					roleMap["user"],
					roleMap["assistant"],
					now,
				)
				if source != nil {
					registration, registerErr := sourceWriter.RegisterAcceptedSourceRevision(ctx, source)
					if registerErr == nil && (registration.Inserted || registration.Idempotent) {
						candidates = []store.MemorySourceRevision{*source}
						sourcesByTurn[turn] = candidates
					} else {
						failed++
						reason := "canonical_source_registration_failed"
						if errors.Is(registerErr, store.ErrSourceRevisionConflict) {
							reason = "canonical_source_revision_conflict"
						} else if registerErr == nil {
							reason = "canonical_source_registration_unconfirmed"
						}
						failedTurns = append(failedTurns, map[string]any{
							"turn_index": turn,
							"reason":     reason,
						})
						if progress != nil {
							progressValue := adminRescanProgress(
								succeeded+failed+skipped+deferred, len(turns), succeeded, failed,
								skipped, processedTurns, failedTurns, skippedTurns,
								artifactCounts, turn, reason,
							)
							progressValue["deferred_count"] = deferred
							progressValue["deferred_turns"] = append([]map[string]any{}, deferredTurns...)
							progress(progressValue)
						}
						continue
					}
				} else {
					failed++
					failedTurns = append(failedTurns, map[string]any{
						"turn_index": turn,
						"reason":     "canonical_raw_pair_incomplete",
					})
					if progress != nil {
						progressValue := adminRescanProgress(
							succeeded+failed+skipped+deferred, len(turns), succeeded, failed,
							skipped, processedTurns, failedTurns, skippedTurns,
							artifactCounts, turn, "canonical_raw_pair_incomplete",
						)
						progressValue["deferred_count"] = deferred
						progressValue["deferred_turns"] = append([]map[string]any{}, deferredTurns...)
						progress(progressValue)
					}
					continue
				}
			}
			if len(candidates) == 1 &&
				(sanitizeCriticStorageText(candidates[0].UserContent) != sanitizeCriticStorageText(roleMap["user"]) ||
					sanitizeCriticStorageText(candidates[0].AssistantContent) != sanitizeCriticStorageText(roleMap["assistant"])) {
				failed++
				failedTurns = append(failedTurns, map[string]any{
					"turn_index": turn,
					"reason":     "active_source_raw_mismatch",
				})
				if progress != nil {
					progressValue := adminRescanProgress(
						succeeded+failed+skipped+deferred, len(turns), succeeded, failed,
						skipped, processedTurns, failedTurns, skippedTurns,
						artifactCounts, turn, "active_source_raw_mismatch",
					)
					progressValue["deferred_count"] = deferred
					progressValue["deferred_turns"] = append([]map[string]any{}, deferredTurns...)
					progress(progressValue)
				}
				continue
			}
			if req.CanonicalRawReplay && len(candidates) == 1 {
				inspectedSource, inspectErr := sourceWriter.GetSourceRevision(
					ctx,
					sid,
					candidates[0].SourceRevision,
				)
				if inspectErr != nil || inspectedSource == nil {
					failed++
					failedTurns = append(failedTurns, map[string]any{
						"turn_index": turn,
						"reason":     "canonical_source_inspection_failed",
					})
					if progress != nil {
						progressValue := adminRescanProgress(
							succeeded+failed+skipped+deferred, len(turns), succeeded, failed,
							skipped, processedTurns, failedTurns, skippedTurns,
							artifactCounts, turn, "canonical_source_inspection_failed",
						)
						progressValue["deferred_count"] = deferred
						progressValue["deferred_turns"] = append([]map[string]any{}, deferredTurns...)
						progress(progressValue)
					}
					continue
				}
				candidates[0] = *inspectedSource
				if !forceDerivedRebuild {
					projectionComplete, projectionErr := s.adminRescanSourceProjectionComplete(
						ctx,
						inspectedSource,
					)
					if projectionErr != nil {
						failed++
						failedTurns = append(failedTurns, map[string]any{
							"turn_index": turn,
							"reason":     "derived_projection_inspection_failed",
						})
						if progress != nil {
							progressValue := adminRescanProgress(
								succeeded+failed+skipped+deferred, len(turns), succeeded, failed,
								skipped, processedTurns, failedTurns, skippedTurns,
								artifactCounts, turn, "derived_projection_inspection_failed",
							)
							progressValue["deferred_count"] = deferred
							progressValue["deferred_turns"] = append([]map[string]any{}, deferredTurns...)
							progress(progressValue)
						}
						continue
					}
					if projectionComplete {
						skipped++
						skippedTurns = append(skippedTurns, map[string]any{
							"turn_index": turn,
							"reason":     "derived_projection_complete",
						})
						if progress != nil {
							progressValue := adminRescanProgress(
								succeeded+failed+skipped+deferred, len(turns), succeeded, failed,
								skipped, processedTurns, failedTurns, skippedTurns,
								artifactCounts, turn, "derived_projection_complete",
							)
							progressValue["deferred_count"] = deferred
							progressValue["deferred_turns"] = append([]map[string]any{}, deferredTurns...)
							progress(progressValue)
						}
						continue
					}
				}
			}
			switch {
			case len(candidates) == 0:
				failed++
				failedTurns = append(failedTurns, map[string]any{
					"turn_index": turn,
					"reason":     "accepted_source_revision_missing",
				})
			case len(candidates) > 1:
				failed++
				failedTurns = append(failedTurns, map[string]any{
					"turn_index": turn,
					"reason":     "active_source_revision_ambiguous",
				})
			case req.DryRun:
				skipped++
				skippedTurns = append(skippedTurns, map[string]any{
					"turn_index": turn,
					"reason":     "dry_run",
				})
			case req.CanonicalRawReplay:
				derivation := s.processAcceptedSourceRevision(
					ctx,
					&candidates[0],
					extractionCfg,
					true,
				)
				switch derivation.State {
				case "completed":
					succeeded++
					processedTurns = append(processedTurns, turn)
					addAdminRescanArtifactCounts(artifactCounts, derivation.SaveResult)
					warnings = append(warnings, derivation.SaveResult.Warnings...)
				case "skipped_ooc":
					skipped++
					skippedTurns = append(skippedTurns, map[string]any{
						"turn_index": turn,
						"reason":     derivation.Failure,
					})
				default:
					failed++
					failedItem := map[string]any{
						"turn_index": turn,
						"reason":     derivation.Failure,
						"state":      derivation.State,
					}
					if len(derivation.CriticTrace) > 0 {
						failedItem["trace"] = derivation.CriticTrace
					}
					failedTurns = append(failedTurns, failedItem)
					if derivation.State == "retryable" {
						inserted, enqueueErr := s.enqueueSourceRevisionReprocessingJob(
							ctx,
							queue,
							&candidates[0],
							derivation.Failure,
							now,
						)
						if enqueueErr != nil {
							warnings = append(
								warnings,
								"reprocessing_enqueue_failed: "+enqueueErr.Error(),
							)
						} else if inserted {
							queued++
						}
					}
				}
			default:
				inserted, enqueueErr := s.enqueueSourceRevisionReprocessingJob(
					ctx, queue, &candidates[0], "admin_rescan_requested", now,
				)
				if enqueueErr != nil {
					failed++
					failedTurns = append(failedTurns, map[string]any{
						"turn_index": turn,
						"reason":     "reprocessing_enqueue_failed",
					})
					break
				}
				if inserted {
					deferred++
					deferredTurns = append(deferredTurns, map[string]any{
						"turn_index": turn,
						"reason":     "reprocessing_queued",
					})
					queued++
					break
				}
				if !forceDerivedRebuild {
					deferred++
					deferredTurns = append(deferredTurns, map[string]any{
						"turn_index": turn,
						"reason":     "reprocessing_job_already_pending",
					})
					break
				}
				reopener, reopenOK := s.Store.(store.MemoryReprocessingJobReopener)
				if !reopenOK {
					skipped++
					skippedTurns = append(skippedTurns, map[string]any{
						"turn_index": turn,
						"reason":     "reprocessing_reopen_unavailable",
					})
					break
				}
				source := &candidates[0]
				idempotencyKey := completeTurnReprocessingIdempotencyKey(
					source.ChatSessionID,
					source.SourceRevision,
					store.MemoryAdmissionContract,
					completeTurnCriticPipelineVersion,
					memoryAdmissionIndexVersion,
				)
				wasReopened, reopenErr := reopener.ReopenMemoryReprocessingJob(
					ctx,
					idempotencyKey,
					source.ChatSessionID,
					source.SourceRevision,
					now,
				)
				if reopenErr != nil {
					failed++
					failedTurns = append(failedTurns, map[string]any{
						"turn_index": turn,
						"reason":     "reprocessing_reopen_failed",
					})
					break
				}
				if !wasReopened {
					skipped++
					skippedTurns = append(skippedTurns, map[string]any{
						"turn_index": turn,
						"reason":     "reprocessing_job_not_reopened",
					})
					break
				}
				deferred++
				deferredTurns = append(deferredTurns, map[string]any{
					"turn_index": turn,
					"reason":     "reprocessing_reopened",
				})
				queued++
				reopened++
				s.wakeMemoryWorkers()
				if auditErr := s.Store.SaveAuditLog(ctx, &store.AuditLog{
					ChatSessionID: sid,
					EventType:     "memory_reprocessing_reopened",
					TargetType:    "source_revision",
					TargetID:      0,
					Summary:       fmt.Sprintf("Reopened derived-memory rebuild for turn %d", turn),
					DetailsJSON: mustCompactJSON(map[string]any{
						"turn_index":      turn,
						"source_revision": source.SourceRevision,
						"idempotency_key": idempotencyKey,
						"raw_preserved":   true,
						"secondary_rows":  "preserved_for_operator_review",
					}),
					Source:    s.storeWriteSource(),
					CreatedAt: now,
				}); auditErr != nil {
					warnings = append(warnings, "memory_reprocessing_reopen_audit_failed")
				}
			}
			if progress != nil {
				phase := "durable_reprocessing_queue"
				if req.CanonicalRawReplay {
					phase = "canonical_raw_source_replay"
				}
				progressValue := adminRescanProgress(
					succeeded+failed+skipped+deferred, len(turns), succeeded, failed,
					skipped, processedTurns, failedTurns, skippedTurns,
					artifactCounts, turn, phase,
				)
				progressValue["deferred_count"] = deferred
				progressValue["deferred_turns"] = append([]map[string]any{}, deferredTurns...)
				progress(progressValue)
			}
		}
		if req.CanonicalRawReplay {
			backfillTargets := targetTurns
			if fullSessionBackfill {
				backfillTargets = map[int]bool{}
			} else if len(processedTurns) > 0 {
				backfillTargets = intsToSet(processedTurns)
			}
			postLogs, postMemories, postEvidence := logs, memories, []store.DirectEvidence(nil)
			if listed, listErr := s.Store.ListChatLogs(ctx, sid, 0, 0); listErr == nil {
				postLogs = listed
			}
			if listed, listErr := s.Store.ListMemories(ctx, sid, 0, 0); listErr == nil {
				postMemories = listed
			}
			if listed, listErr := s.Store.ListEvidence(ctx, sid); listErr == nil {
				postEvidence = listed
			}
			runBackfills(postLogs, postMemories, postEvidence, backfillTargets)
		}
		status := "ok"
		switch {
		case failed > 0:
			status = "partial_error"
		case deferred > 0:
			status = "deferred"
		}
		return map[string]any{
			"status":              status,
			"source":              s.storeWriteSource(),
			"chat_session_id":     sid,
			"dry_run":             req.DryRun,
			"candidate_count":     len(turns),
			"succeeded":           succeeded,
			"failed":              failed,
			"skipped":             skipped,
			"deferred":            deferred,
			"queued":              queued,
			"reopened":            reopened,
			"processed_turns":     processedTurns,
			"failed_turns":        failedTurns,
			"skipped_turns":       skippedTurns,
			"deferred_turns":      deferredTurns,
			"artifact_counts":     artifactCounts,
			"episode_backfill":    episodeBackfill,
			"world_rule_backfill": worldRuleBackfill,
			"hierarchy_backfill":  hierarchyBackfill,
			"warnings":            warnings,
			"llm_config_trace":    llmTrace,
			"note": func() string {
				if req.CanonicalRawReplay {
					return "canonical raw pairs were rebuilt synchronously through the shared source-fenced derivation owner"
				}
				return "rescan candidates were handed to the durable source-fenced reprocessing worker"
			}(),
		}, nil
	}

	if !extractionCfg.Critic.hasConfig() {
		for _, turn := range turns {
			failedTurns = append(failedTurns, map[string]any{"turn_index": turn, "reason": "critic_config_missing"})
		}
		runBackfills(logs, memories, nil, targetTurns)
		if progress != nil {
			progress(adminRescanProgress(len(turns), len(turns), 0, len(turns), 0, []int{}, failedTurns, []map[string]any{}, artifactCounts, 0, "critic_config_missing"))
		}
		return map[string]any{
			"status":              "partial_error",
			"source":              s.storeWriteSource(),
			"chat_session_id":     sid,
			"dry_run":             req.DryRun,
			"candidate_count":     len(turns),
			"succeeded":           0,
			"failed":              len(turns),
			"skipped":             0,
			"processed_turns":     []int{},
			"failed_turns":        failedTurns,
			"skipped_turns":       []map[string]any{},
			"artifact_counts":     artifactCounts,
			"episode_backfill":    episodeBackfill,
			"world_rule_backfill": worldRuleBackfill,
			"hierarchy_backfill":  hierarchyBackfill,
			"llm_config_trace":    llmTrace,
			"note":                "rescan needs configured Critic LLM settings before derived Memory/Direct Evidence/KG/state can be regenerated",
		}, nil
	}

	now := time.Now().UTC()
	for _, turn := range turns {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		roleMap := turnLogs[turn]
		userText := sanitizeCriticStorageText(roleMap["user"])
		assistantText := sanitizeCriticStorageText(roleMap["assistant"])
		if strings.TrimSpace(assistantText) == "" {
			failed++
			failedTurns = append(failedTurns, map[string]any{"turn_index": turn, "reason": "assistant_content_missing"})
			if progress != nil {
				progress(adminRescanProgress(len(processedTurns)+failed+skipped, len(turns), succeeded, failed, skipped, processedTurns, failedTurns, skippedTurns, artifactCounts, turn, "assistant_content_missing"))
			}
			continue
		}
		if req.DryRun {
			skipped++
			skippedTurns = append(skippedTurns, map[string]any{"turn_index": turn, "reason": "dry_run"})
			if progress != nil {
				progress(adminRescanProgress(len(processedTurns)+failed+skipped, len(turns), succeeded, failed, skipped, processedTurns, failedTurns, skippedTurns, artifactCounts, turn, "dry_run"))
			}
			continue
		}
		extraction, trace, err := s.runCompleteTurnCriticFromCanonicalLogs(ctx, sid, turn, userText, assistantText, extractionCfg.Critic)
		if err != nil {
			failed++
			failedTurns = append(failedTurns, map[string]any{"turn_index": turn, "reason": "critic_extract_failed: " + err.Error(), "trace": trace})
			if progress != nil {
				progress(adminRescanProgress(len(processedTurns)+failed+skipped, len(turns), succeeded, failed, skipped, processedTurns, failedTurns, skippedTurns, artifactCounts, turn, "critic_extract_failed"))
			}
			continue
		}
		content := strings.TrimSpace(strings.Join([]string{userText, assistantText}, "\n"))
		saveResult := s.saveCriticExtractionArtifacts(ctx, sid, turn, extraction, content, extractionCfg.Embedder, now)
		if saveResult.Errors > 0 {
			failed++
			failedTurns = append(failedTurns, map[string]any{"turn_index": turn, "reason": "artifact_save_failed", "errors": saveResult.ErrorDetails})
			warnings = append(warnings, saveResult.Warnings...)
			if progress != nil {
				progress(adminRescanProgress(len(processedTurns)+failed+skipped, len(turns), succeeded, failed, skipped, processedTurns, failedTurns, skippedTurns, artifactCounts, turn, "artifact_save_failed"))
			}
			continue
		}
		succeeded++
		processedTurns = append(processedTurns, turn)
		artifactCounts["memories"] += saveResult.Memories
		artifactCounts["evidence"] += saveResult.Evidence
		artifactCounts["kg_triples"] += saveResult.KGTriples
		artifactCounts["character_events"] += saveResult.CharacterEvents
		artifactCounts["storylines"] += saveResult.Storylines
		artifactCounts["world_rules"] += saveResult.WorldRules
		artifactCounts["character_states"] += saveResult.CharacterStates
		artifactCounts["pending_threads"] += saveResult.PendingThreads
		artifactCounts["active_states"] += saveResult.ActiveStates
		artifactCounts["entities"] += saveResult.Entities
		artifactCounts["trust_states"] += saveResult.TrustStates
		artifactCounts["vectors_upserted"] += saveResult.VectorsUpserted
		warnings = append(warnings, saveResult.Warnings...)
		if progress != nil {
			progress(adminRescanProgress(len(processedTurns)+failed+skipped, len(turns), succeeded, failed, skipped, processedTurns, failedTurns, skippedTurns, artifactCounts, turn, "saved"))
		}
	}

	backfillTargets := targetTurns
	if fullSessionBackfill {
		backfillTargets = map[int]bool{}
	} else if len(processedTurns) > 0 {
		backfillTargets = intsToSet(processedTurns)
	}
	postLogs := logs
	postMemories := memories
	postEvidence := []store.DirectEvidence(nil)
	if s.Store != nil {
		if listed, err := s.Store.ListChatLogs(ctx, sid, 0, 0); err == nil {
			postLogs = listed
		}
		if listed, err := s.Store.ListMemories(ctx, sid, 0, 0); err == nil {
			postMemories = listed
		}
		if listed, err := s.Store.ListEvidence(ctx, sid); err == nil {
			postEvidence = listed
		}
	}
	runBackfills(postLogs, postMemories, postEvidence, backfillTargets)

	if succeeded > 0 {
		_ = s.Store.SaveAuditLog(ctx, &store.AuditLog{
			ChatSessionID: sid,
			EventType:     "rescan_rebuild",
			TargetType:    "session",
			TargetID:      0,
			Summary:       fmt.Sprintf("Rescan rebuilt derived artifacts for %d turns", succeeded),
			DetailsJSON: mustCompactJSON(map[string]any{
				"processed_turns":     processedTurns,
				"artifact_counts":     artifactCounts,
				"episode_backfill":    episodeBackfill,
				"world_rule_backfill": worldRuleBackfill,
				"hierarchy_backfill":  hierarchyBackfill,
				"failed":              failed,
				"skipped":             skipped,
			}),
			Source:    s.storeWriteSource(),
			CreatedAt: now,
		})
	}

	result := map[string]any{
		"status":              "ok",
		"source":              s.storeWriteSource(),
		"chat_session_id":     sid,
		"dry_run":             req.DryRun,
		"candidate_count":     len(turns),
		"succeeded":           succeeded,
		"failed":              failed,
		"skipped":             skipped,
		"processed_turns":     uniqueSortedNonNegativeInts(processedTurns),
		"failed_turns":        failedTurns,
		"skipped_turns":       skippedTurns,
		"artifact_counts":     artifactCounts,
		"episode_backfill":    episodeBackfill,
		"world_rule_backfill": worldRuleBackfill,
		"hierarchy_backfill":  hierarchyBackfill,
		"warnings":            warnings,
		"llm_config_trace":    llmTrace,
		"note":                "rescan reprocessed raw chat_logs that were missing memory and rebuilt derived artifacts through the configured Critic pipeline",
	}
	if progress != nil {
		progress(map[string]any{
			"status":              "completed",
			"stage":               "completed",
			"candidate_count":     len(turns),
			"processed":           len(processedTurns) + failed + skipped,
			"succeeded":           succeeded,
			"failed_count":        failed,
			"skipped_count":       skipped,
			"processed_turns":     uniqueSortedNonNegativeInts(processedTurns),
			"failed_turns":        failedTurns,
			"skipped_turns":       skippedTurns,
			"artifact_counts":     cloneIntMapAny(artifactCounts),
			"episode_backfill":    episodeBackfill,
			"world_rule_backfill": worldRuleBackfill,
			"hierarchy_backfill":  hierarchyBackfill,
			"progress_percent":    100,
		})
	}
	return result, nil
}

func adminRescanCanonicalRawSourceRevision(
	sid string,
	turn int,
	userText string,
	assistantText string,
	observedAt time.Time,
) *store.MemorySourceRevision {
	sid = strings.TrimSpace(sid)
	if sid == "" || turn <= 0 || userText == "" || assistantText == "" {
		return nil
	}
	if observedAt.IsZero() {
		observedAt = time.Now().UTC()
	}
	content := strings.TrimSpace(strings.Join([]string{userText, assistantText}, "\n"))
	contentHash := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
	userHash := fmt.Sprintf("%x", sha256.Sum256([]byte(userText)))
	assistantHash := fmt.Sprintf("%x", sha256.Sum256([]byte(assistantText)))
	revisionSeed := strings.Join([]string{
		"canonical_raw_reprocessing.v1",
		sid,
		fmt.Sprint(turn),
		contentHash,
		observedAt.UTC().Format(time.RFC3339Nano),
	}, "\x1f")
	revisionHash := fmt.Sprintf("%x", sha256.Sum256([]byte(revisionSeed)))
	return &store.MemorySourceRevision{
		ContractVersion:              store.MemorySourceRevisionContract,
		SourceRevision:               "sar_" + revisionHash,
		ChatSessionID:                sid,
		LogicalTurnID:                "canonical_turn_" + revisionHash,
		TurnIndex:                    turn,
		BranchState:                  "not_exposed",
		UserContent:                  userText,
		AssistantContent:             assistantText,
		CombinedContentHash:          contentHash,
		UserObservedContentHash:      userHash,
		AssistantObservedContentHash: assistantHash,
		HashAlgorithm:                "sha256",
		HostObservedAtMS:             observedAt.UnixMilli(),
		LifecycleState:               "active",
		CreatedAt:                    observedAt,
		UpdatedAt:                    observedAt,
	}
}

func (s *Server) adminRescanSourceProjectionComplete(
	ctx context.Context,
	source *store.MemorySourceRevision,
) (bool, error) {
	if s == nil || s.Store == nil || source == nil ||
		source.LifecycleState != "active" ||
		source.DerivedAdmissionState != "committed" ||
		source.DerivedAdmissionVersion != store.MemoryAdmissionContract ||
		source.DerivedExtractorVersion != completeTurnCriticPipelineVersion ||
		source.DerivedIndexVersion != memoryAdmissionIndexVersion ||
		strings.TrimSpace(source.DerivedResultHash) == "" ||
		strings.TrimSpace(source.DerivedResultJSON) == "" {
		return false, nil
	}
	logs, err := s.Store.ListAuditLogs(
		ctx,
		source.ChatSessionID,
		"critic_ingest_trace",
		0,
	)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return false, err
	}
	for _, item := range logs {
		if item.ChatSessionID != source.ChatSessionID ||
			item.TargetType != "turn" ||
			item.TargetID != int64(source.TurnIndex) {
			continue
		}
		details := map[string]any{}
		if json.Unmarshal([]byte(strings.TrimSpace(item.DetailsJSON)), &details) != nil {
			continue
		}
		if !boolFromAny(details["pipeline_complete"]) ||
			strings.TrimSpace(stringFromMap(details, "source_revision")) != source.SourceRevision ||
			strings.TrimSpace(stringFromMap(details, "derivation_version")) != store.MemoryAdmissionContract ||
			strings.TrimSpace(stringFromMap(details, "extractor_version")) != completeTurnCriticPipelineVersion ||
			strings.TrimSpace(stringFromMap(details, "index_version")) != memoryAdmissionIndexVersion {
			continue
		}
		return true, nil
	}
	return false, nil
}

func addAdminRescanArtifactCounts(counts map[string]int, result artifactSaveResult) {
	counts["memories"] += result.Memories
	counts["evidence"] += result.Evidence
	counts["kg_triples"] += result.KGTriples
	counts["subjective_entity_memories"] += result.SubjectiveEntityMemories
	counts["character_events"] += result.CharacterEvents
	counts["storylines"] += result.Storylines
	counts["world_rules"] += result.WorldRules
	counts["character_states"] += result.CharacterStates
	counts["pending_threads"] += result.PendingThreads
	counts["active_states"] += result.ActiveStates
	counts["entities"] += result.Entities
	counts["trust_states"] += result.TrustStates
	counts["vectors_upserted"] += result.VectorsUpserted
}

func adminRescanProgress(processed, total, succeeded, failed, skipped int, processedTurns []int, failedTurns, skippedTurns []map[string]any, artifactCounts map[string]int, lastTurn int, lastReason string) map[string]any {
	return map[string]any{
		"status":           "running",
		"stage":            "critic_artifact_rebuild",
		"candidate_count":  total,
		"processed":        processed,
		"succeeded":        succeeded,
		"failed_count":     failed,
		"skipped_count":    skipped,
		"processed_turns":  uniqueSortedNonNegativeInts(processedTurns),
		"failed_turns":     append([]map[string]any{}, failedTurns...),
		"skipped_turns":    append([]map[string]any{}, skippedTurns...),
		"artifact_counts":  cloneIntMapAny(artifactCounts),
		"last_processed":   lastTurn,
		"last_reason":      nilIfEmpty(lastReason),
		"progress_percent": adminJobProgressPercent(processed, total),
	}
}

func cloneIntMapAny(in map[string]int) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func skippedEpisodeBackfillResult(dryRun bool, interval int, force bool, reason string) map[string]any {
	return map[string]any{
		"status":    "skipped",
		"dry_run":   dryRun,
		"interval":  interval,
		"candidate": 0,
		"generated": 0,
		"existing":  0,
		"skipped":   0,
		"force":     force,
		"reason":    reason,
	}
}

func skippedWorldRuleBackfillResult(dryRun bool, reason string) map[string]any {
	return map[string]any{
		"status":    "skipped",
		"dry_run":   dryRun,
		"candidate": 0,
		"generated": 0,
		"existing":  0,
		"skipped":   0,
		"reason":    reason,
	}
}

func skippedHierarchyBackfillResult(dryRun bool, reason string) map[string]any {
	return map[string]any{
		"status":  "skipped",
		"dry_run": dryRun,
		"reason":  reason,
		"chapter": hierarchyLayerBackfillResult(dryRun, reason),
		"arc":     hierarchyLayerBackfillResult(dryRun, reason),
		"saga":    hierarchyLayerBackfillResult(dryRun, reason),
	}
}

func hierarchyLayerBackfillResult(dryRun bool, reason string) map[string]any {
	return map[string]any{
		"status":    "skipped",
		"dry_run":   dryRun,
		"candidate": 0,
		"generated": 0,
		"existing":  0,
		"skipped":   0,
		"blocked":   []map[string]any{},
		"reason":    reason,
	}
}

func (s *Server) backfillHierarchySummaries(ctx context.Context, sid string, logs []store.ChatLog, targetTurns map[int]bool, meta map[string]any, dryRun bool) map[string]any {
	result := skippedHierarchyBackfillResult(dryRun, "not_run")
	result["status"] = "ok"
	result["reason"] = nil
	if s == nil || s.Store == nil {
		result["status"] = "skipped"
		result["reason"] = "store_unavailable"
		return result
	}
	minTurn, maxTurn := chatLogTurnBounds(sid, logs)
	if minTurn <= 0 || maxTurn <= 0 {
		result["status"] = "skipped"
		result["reason"] = "no_chat_logs"
		return result
	}
	chapterInterval := normalizedHierarchyChildCount(intFromAny(meta["chapter_interval_episodes"], 0))
	arcInterval := normalizedHierarchyChildCount(intFromAny(meta["arc_interval_chapters"], 0))
	sagaInterval := normalizedHierarchyChildCount(intFromAny(meta["saga_interval_arcs"], 0))
	force := boolFromAny(meta["force_hierarchy_backfill"]) || boolFromAny(meta["force_chapter_backfill"]) || boolFromAny(meta["force_arc_backfill"]) || boolFromAny(meta["force_saga_backfill"])

	var chapterResult map[string]any
	if hierarchyBackfillLayerEnabled(meta, "chapter_auto_enabled", true) {
		var err error
		chapterResult, err = s.backfillChapterSummaries(ctx, sid, minTurn, maxTurn, chapterInterval, targetTurns, dryRun, force)
		if err != nil {
			result["status"] = "partial_error"
			result["error"] = err.Error()
		}
	} else {
		chapterResult = hierarchyLayerBackfillResult(dryRun, "chapter_auto_disabled")
		chapterResult["interval_episodes"] = chapterInterval
	}
	result["chapter"] = chapterResult

	var arcResult map[string]any
	if hierarchyBackfillLayerEnabled(meta, "arc_auto_enabled", true) {
		var err error
		arcResult, err = s.backfillArcSummaries(ctx, sid, minTurn, maxTurn, arcInterval, targetTurns, dryRun, force)
		if err != nil && result["status"] != "partial_error" {
			result["status"] = "partial_error"
			result["error"] = err.Error()
		}
	} else {
		arcResult = hierarchyLayerBackfillResult(dryRun, "arc_auto_disabled")
		arcResult["interval_chapters"] = arcInterval
	}
	result["arc"] = arcResult

	var sagaResult map[string]any
	if hierarchyBackfillLayerEnabled(meta, "saga_auto_enabled", true) {
		var err error
		sagaResult, err = s.backfillSagaDigests(ctx, sid, minTurn, maxTurn, sagaInterval, targetTurns, dryRun, force)
		if err != nil && result["status"] != "partial_error" {
			result["status"] = "partial_error"
			result["error"] = err.Error()
		}
	} else {
		sagaResult = hierarchyLayerBackfillResult(dryRun, "saga_auto_disabled")
		sagaResult["interval_arcs"] = sagaInterval
	}
	result["saga"] = sagaResult
	result["chapter_interval_episodes"] = chapterInterval
	result["arc_interval_chapters"] = arcInterval
	result["saga_interval_arcs"] = sagaInterval
	result["range"] = map[string]any{"from_turn": minTurn, "to_turn": maxTurn}
	result["policy"] = "step23_closed_range_hierarchy_backfill"
	return result
}

func hierarchyBackfillLayerEnabled(meta map[string]any, key string, fallback bool) bool {
	if meta == nil {
		return fallback
	}
	if _, ok := meta[key]; !ok {
		return fallback
	}
	return boolFromAny(meta[key])
}

func (s *Server) backfillChapterSummaries(ctx context.Context, sid string, minTurn, maxTurn, interval int, targetTurns map[int]bool, dryRun, force bool) (map[string]any, error) {
	layer := hierarchyLayerBackfillResult(dryRun, "")
	layer["interval_episodes"] = interval
	if interval <= 0 {
		layer["status"] = "skipped"
		layer["reason"] = "chapter_interval_episodes_not_configured"
		return layer, nil
	}
	chapterStore, ok := s.Store.(store.ChapterSummaryStore)
	if !ok {
		layer["status"] = "skipped"
		layer["reason"] = "chapter_store_not_available"
		return layer, nil
	}
	episodes, err := s.Store.ListEpisodeSummaries(ctx, sid, 0, 0, 0)
	if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
		return layer, err
	}
	sort.SliceStable(episodes, func(i, j int) bool {
		if episodes[i].FromTurn == episodes[j].FromTurn {
			return episodes[i].ToTurn < episodes[j].ToTurn
		}
		return episodes[i].FromTurn < episodes[j].FromTurn
	})
	closed := make([]store.EpisodeSummary, 0, len(episodes))
	seen := map[string]bool{}
	for _, episode := range episodes {
		if episode.FromTurn < minTurn || episode.ToTurn > maxTurn || episode.FromTurn <= 0 || episode.ToTurn < episode.FromTurn {
			continue
		}
		key := fmt.Sprintf("%d:%d", episode.FromTurn, episode.ToTurn)
		if seen[key] {
			continue
		}
		seen[key] = true
		closed = append(closed, episode)
	}
	groupStart := 0
	for ; groupStart+interval <= len(closed); groupStart += interval {
		group := closed[groupStart : groupStart+interval]
		fromTurn := group[0].FromTurn
		toTurn := group[len(group)-1].ToTurn
		if len(targetTurns) > 0 && !turnRangeContainsTargetTurn(fromTurn, toTurn, targetTurns) {
			layer["skipped"] = intFromAny(layer["skipped"], 0) + 1
			continue
		}
		if !episodeCoverageComplete(group, fromTurn, toTurn) {
			addHierarchyBlocked(layer, fromTurn, toTurn, "blocked_missing_episode")
			continue
		}
		layer["candidate"] = intFromAny(layer["candidate"], 0) + 1
		exists, err := chapterSummaryExists(ctx, chapterStore, sid, fromTurn, toTurn)
		if err != nil {
			return layer, err
		}
		if exists {
			layer["existing"] = intFromAny(layer["existing"], 0) + 1
			continue
		}
		if dryRun {
			continue
		}
		chapter, _ := s.buildChapterSummaryForRange(ctx, sid, fromTurn, toTurn, groupStart/interval+1, group)
		if err := chapterStore.SaveChapterSummary(ctx, &chapter); err != nil {
			return layer, err
		}
		layer["generated"] = intFromAny(layer["generated"], 0) + 1
	}
	if groupStart < len(closed) {
		addHierarchyBlocked(layer, closed[groupStart].FromTurn, closed[len(closed)-1].ToTurn, "open_tail_episode_count")
	}
	layer["status"] = "ok"
	layer["reason"] = nil
	return layer, nil
}

func (s *Server) backfillArcSummaries(ctx context.Context, sid string, minTurn, maxTurn, interval int, targetTurns map[int]bool, dryRun, force bool) (map[string]any, error) {
	layer := hierarchyLayerBackfillResult(dryRun, "")
	layer["interval_chapters"] = interval
	if interval <= 0 {
		layer["status"] = "skipped"
		layer["reason"] = "arc_interval_chapters_not_configured"
		return layer, nil
	}
	arcStore, ok := s.Store.(store.ArcSummaryStore)
	if !ok {
		layer["status"] = "skipped"
		layer["reason"] = "arc_store_not_available"
		return layer, nil
	}
	chapterStore, ok := s.Store.(store.ChapterSummaryStore)
	if !ok {
		layer["status"] = "skipped"
		layer["reason"] = "chapter_store_not_available"
		return layer, nil
	}
	chapters, err := chapterStore.SearchChapterSummaries(ctx, sid, "", 0, 0, 0)
	if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
		return layer, err
	}
	sort.SliceStable(chapters, func(i, j int) bool {
		if chapters[i].FromTurn == chapters[j].FromTurn {
			return chapters[i].ToTurn < chapters[j].ToTurn
		}
		return chapters[i].FromTurn < chapters[j].FromTurn
	})
	closed := make([]store.ChapterSummary, 0, len(chapters))
	seen := map[string]bool{}
	for _, chapter := range chapters {
		if chapter.FromTurn < minTurn || chapter.ToTurn > maxTurn || chapter.FromTurn <= 0 || chapter.ToTurn < chapter.FromTurn {
			continue
		}
		key := fmt.Sprintf("%d:%d", chapter.FromTurn, chapter.ToTurn)
		if seen[key] {
			continue
		}
		seen[key] = true
		closed = append(closed, chapter)
	}
	groupStart := 0
	for ; groupStart+interval <= len(closed); groupStart += interval {
		group := closed[groupStart : groupStart+interval]
		fromTurn := group[0].FromTurn
		toTurn := group[len(group)-1].ToTurn
		if len(targetTurns) > 0 && !turnRangeContainsTargetTurn(fromTurn, toTurn, targetTurns) {
			layer["skipped"] = intFromAny(layer["skipped"], 0) + 1
			continue
		}
		if !chapterCoverageComplete(group, fromTurn, toTurn) {
			addHierarchyBlocked(layer, fromTurn, toTurn, "blocked_missing_chapter")
			continue
		}
		layer["candidate"] = intFromAny(layer["candidate"], 0) + 1
		exists, err := arcSummaryExists(ctx, arcStore, sid, fromTurn, toTurn)
		if err != nil {
			return layer, err
		}
		if exists {
			layer["existing"] = intFromAny(layer["existing"], 0) + 1
			continue
		}
		if dryRun {
			continue
		}
		arc, _ := s.buildArcSummaryForRange(ctx, sid, fromTurn, toTurn, groupStart/interval+1, group)
		if err := arcStore.SaveArcSummary(ctx, sid, &arc); err != nil {
			return layer, err
		}
		layer["generated"] = intFromAny(layer["generated"], 0) + 1
	}
	if groupStart < len(closed) {
		addHierarchyBlocked(layer, closed[groupStart].FromTurn, closed[len(closed)-1].ToTurn, "open_tail_chapter_count")
	}
	layer["status"] = "ok"
	layer["reason"] = nil
	return layer, nil
}

func (s *Server) backfillSagaDigests(ctx context.Context, sid string, minTurn, maxTurn, interval int, targetTurns map[int]bool, dryRun, force bool) (map[string]any, error) {
	layer := hierarchyLayerBackfillResult(dryRun, "")
	layer["interval_arcs"] = interval
	if interval <= 0 {
		layer["status"] = "skipped"
		layer["reason"] = "saga_interval_arcs_not_configured"
		return layer, nil
	}
	sagaStore, ok := s.Store.(store.SagaDigestStore)
	if !ok {
		layer["status"] = "skipped"
		layer["reason"] = "saga_store_not_available"
		return layer, nil
	}
	arcStore, ok := s.Store.(store.ArcSummaryStore)
	if !ok {
		layer["status"] = "skipped"
		layer["reason"] = "arc_store_not_available"
		return layer, nil
	}
	arcs, err := arcStore.SearchArcSummaries(ctx, sid, "", 0, 0, 0)
	if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
		return layer, err
	}
	sort.SliceStable(arcs, func(i, j int) bool {
		if arcs[i].FromTurn == arcs[j].FromTurn {
			return arcs[i].ToTurn < arcs[j].ToTurn
		}
		return arcs[i].FromTurn < arcs[j].FromTurn
	})
	closed := make([]store.ArcSummary, 0, len(arcs))
	seen := map[string]bool{}
	for _, arc := range arcs {
		if arc.FromTurn < minTurn || arc.ToTurn > maxTurn || arc.FromTurn <= 0 || arc.ToTurn < arc.FromTurn {
			continue
		}
		key := fmt.Sprintf("%d:%d", arc.FromTurn, arc.ToTurn)
		if seen[key] {
			continue
		}
		seen[key] = true
		closed = append(closed, arc)
	}
	groupStart := 0
	for ; groupStart+interval <= len(closed); groupStart += interval {
		group := closed[groupStart : groupStart+interval]
		fromTurn := group[0].FromTurn
		toTurn := group[len(group)-1].ToTurn
		if len(targetTurns) > 0 && !turnRangeContainsTargetTurn(fromTurn, toTurn, targetTurns) {
			layer["skipped"] = intFromAny(layer["skipped"], 0) + 1
			continue
		}
		if !arcCoverageComplete(group, fromTurn, toTurn) {
			addHierarchyBlocked(layer, fromTurn, toTurn, "blocked_missing_arc")
			continue
		}
		layer["candidate"] = intFromAny(layer["candidate"], 0) + 1
		exists, err := sagaDigestExists(ctx, sagaStore, sid, fromTurn, toTurn)
		if err != nil {
			return layer, err
		}
		if exists {
			layer["existing"] = intFromAny(layer["existing"], 0) + 1
			continue
		}
		if dryRun {
			continue
		}
		saga, _ := s.buildSagaDigestForRange(ctx, sid, fromTurn, toTurn, group)
		if err := sagaStore.SaveSagaDigest(ctx, sid, &saga); err != nil {
			return layer, err
		}
		layer["generated"] = intFromAny(layer["generated"], 0) + 1
	}
	if groupStart < len(closed) {
		addHierarchyBlocked(layer, closed[groupStart].FromTurn, closed[len(closed)-1].ToTurn, "open_tail_arc_count")
	}
	layer["status"] = "ok"
	layer["reason"] = nil
	return layer, nil
}

func normalizedHierarchyChildCount(value int) int {
	if value <= 0 {
		return 0
	}
	return value
}

func chatLogTurnBounds(sid string, logs []store.ChatLog) (int, int) {
	minTurn, maxTurn := 0, 0
	for _, log := range logs {
		if log.ChatSessionID != sid || log.TurnIndex <= 0 {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(log.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		if strings.TrimSpace(log.Content) == "" {
			continue
		}
		if minTurn == 0 || log.TurnIndex < minTurn {
			minTurn = log.TurnIndex
		}
		if log.TurnIndex > maxTurn {
			maxTurn = log.TurnIndex
		}
	}
	return minTurn, maxTurn
}

func addHierarchyBlocked(layer map[string]any, fromTurn, toTurn int, reason string) {
	layer["skipped"] = intFromAny(layer["skipped"], 0) + 1
	blocked := []map[string]any{}
	if raw, ok := layer["blocked"].([]map[string]any); ok {
		blocked = append(blocked, raw...)
	}
	blocked = append(blocked, map[string]any{"from_turn": fromTurn, "to_turn": toTurn, "reason": reason})
	layer["blocked"] = blocked
}

func chapterSummaryExists(ctx context.Context, chapterStore store.ChapterSummaryStore, sid string, fromTurn, toTurn int) (bool, error) {
	items, err := chapterStore.SearchChapterSummaries(ctx, sid, "", fromTurn, toTurn, 50)
	if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
		return false, err
	}
	for _, item := range items {
		if item.FromTurn == fromTurn && item.ToTurn == toTurn {
			return true, nil
		}
	}
	return false, nil
}

func arcSummaryExists(ctx context.Context, arcStore store.ArcSummaryStore, sid string, fromTurn, toTurn int) (bool, error) {
	items, err := arcStore.SearchArcSummaries(ctx, sid, "", fromTurn, toTurn, 50)
	if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
		return false, err
	}
	for _, item := range items {
		if item.FromTurn == fromTurn && item.ToTurn == toTurn {
			return true, nil
		}
	}
	return false, nil
}

func sagaDigestExists(ctx context.Context, sagaStore store.SagaDigestStore, sid string, fromTurn, toTurn int) (bool, error) {
	items, err := sagaStore.SearchSagaDigests(ctx, sid, "", fromTurn, toTurn, 50)
	if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
		return false, err
	}
	for _, item := range items {
		if item.FromTurn == fromTurn && item.ToTurn == toTurn {
			return true, nil
		}
	}
	return false, nil
}

func episodeCoverageComplete(items []store.EpisodeSummary, fromTurn, toTurn int) bool {
	ranges := make([]turnRange, 0, len(items))
	for _, item := range items {
		ranges = append(ranges, turnRange{fromTurn: item.FromTurn, toTurn: item.ToTurn})
	}
	return turnRangesCover(ranges, fromTurn, toTurn)
}

func chapterCoverageComplete(items []store.ChapterSummary, fromTurn, toTurn int) bool {
	ranges := make([]turnRange, 0, len(items))
	for _, item := range items {
		ranges = append(ranges, turnRange{fromTurn: item.FromTurn, toTurn: item.ToTurn})
	}
	return turnRangesCover(ranges, fromTurn, toTurn)
}

func arcCoverageComplete(items []store.ArcSummary, fromTurn, toTurn int) bool {
	ranges := make([]turnRange, 0, len(items))
	for _, item := range items {
		ranges = append(ranges, turnRange{fromTurn: item.FromTurn, toTurn: item.ToTurn})
	}
	return turnRangesCover(ranges, fromTurn, toTurn)
}

type turnRange struct {
	fromTurn int
	toTurn   int
}

func turnRangesCover(ranges []turnRange, fromTurn, toTurn int) bool {
	if fromTurn <= 0 || toTurn < fromTurn {
		return false
	}
	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].fromTurn == ranges[j].fromTurn {
			return ranges[i].toTurn < ranges[j].toTurn
		}
		return ranges[i].fromTurn < ranges[j].fromTurn
	})
	next := fromTurn
	for _, item := range ranges {
		if item.fromTurn <= 0 || item.toTurn < item.fromTurn {
			continue
		}
		if item.toTurn < next {
			continue
		}
		if item.fromTurn > next {
			return false
		}
		next = item.toTurn + 1
		if next > toTurn {
			return true
		}
	}
	return next > toTurn
}

func intsToSet(items []int) map[int]bool {
	out := map[int]bool{}
	for _, item := range items {
		if item >= 0 {
			out[item] = true
		}
	}
	return out
}

func uniqueSortedNonNegativeInts(values []int) []int {
	seen := map[int]bool{}
	out := []int{}
	for _, value := range values {
		if value < 0 || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Ints(out)
	return out
}

func (s *Server) backfillEpisodeSummariesFromChatLogs(ctx context.Context, sid string, logs []store.ChatLog, memories []store.Memory, evidence []store.DirectEvidence, interval int, dryRun bool, targetTurns map[int]bool, force bool) map[string]any {
	result := map[string]any{
		"status":    "skipped",
		"dry_run":   dryRun,
		"interval":  interval,
		"candidate": 0,
		"generated": 0,
		"existing":  0,
		"skipped":   0,
		"force":     force,
	}
	if s == nil || s.Store == nil {
		result["reason"] = "store_unavailable"
		return result
	}
	episodeStore, ok := s.Store.(store.EpisodeSummaryStore)
	if !ok {
		result["reason"] = "episode_store_not_available"
		return result
	}
	if interval <= 0 {
		interval = normalizedEpisodeInterval(0)
	}
	if memories == nil {
		if listed, err := s.Store.ListMemories(ctx, sid, 0, 0); err == nil {
			memories = listed
		}
	}
	if evidence == nil {
		if listed, err := s.Store.ListEvidence(ctx, sid); err == nil {
			evidence = listed
		}
	}
	minTurn, maxTurn := 0, 0
	for _, log := range logs {
		if log.ChatSessionID != sid || log.TurnIndex <= 0 {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(log.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		if minTurn == 0 || log.TurnIndex < minTurn {
			minTurn = log.TurnIndex
		}
		if log.TurnIndex > maxTurn {
			maxTurn = log.TurnIndex
		}
	}
	if minTurn <= 0 || maxTurn <= 0 {
		result["reason"] = "no_chat_logs"
		return result
	}
	if minTurn > 1 {
		minTurn = ((minTurn-1)/interval)*interval + 1
	}
	candidates := 0
	generated := 0
	existingCount := 0
	skipped := 0
	partialSkipped := 0
	for fromTurn := minTurn; fromTurn <= maxTurn; fromTurn += interval {
		fullToTurn := fromTurn + interval - 1
		if fullToTurn > maxTurn {
			partialSkipped++
			skipped++
			continue
		}
		toTurn := fullToTurn
		if len(targetTurns) > 0 && !turnRangeContainsTargetTurn(fromTurn, toTurn, targetTurns) {
			skipped++
			continue
		}
		chatLogs := filterChatLogsForTurnRange(logs, fromTurn, toTurn, 24)
		rangeMemories := filterMemoriesForTurnRange(memories, sid, fromTurn, toTurn)
		rangeEvidence := filterEvidenceForTurnRange(evidence, sid, fromTurn, toTurn)
		if len(chatLogs) == 0 && len(rangeMemories) == 0 && len(rangeEvidence) == 0 {
			skipped++
			continue
		}
		candidates++
		existing, err := s.Store.ListEpisodeSummaries(ctx, sid, 0, fromTurn, toTurn)
		if err == nil {
			foundExact := false
			for _, item := range existing {
				if item.FromTurn == fromTurn && item.ToTurn == toTurn {
					foundExact = true
					break
				}
			}
			if foundExact {
				if !force {
					existingCount++
					continue
				}
				if !dryRun {
					if deleter, ok := s.Store.(episodeSummaryRangeDeleter); ok {
						if _, err := deleter.DeleteEpisodeSummariesInRange(ctx, sid, fromTurn, toTurn); err != nil {
							result["status"] = "partial_error"
							result["error"] = err.Error()
							return result
						}
					} else {
						existingCount++
						continue
					}
				}
			}
		} else if !errors.Is(err, store.ErrNotEnabled) && !errors.Is(err, store.ErrNotFound) {
			result["status"] = "partial_error"
			result["error"] = err.Error()
			return result
		}
		if dryRun {
			continue
		}
		episode, _ := buildEpisodeSummaryForRangeWithArtifacts(sid, fromTurn, toTurn, chatLogs, rangeMemories, rangeEvidence)
		if err := episodeStore.SaveEpisodeSummary(ctx, &episode); err != nil {
			result["status"] = "partial_error"
			result["error"] = err.Error()
			return result
		}
		generated++
	}
	result["status"] = "ok"
	result["candidate"] = candidates
	result["generated"] = generated
	result["existing"] = existingCount
	result["skipped"] = skipped
	result["partial_skipped"] = partialSkipped
	return result
}

func turnRangeContainsTargetTurn(fromTurn, toTurn int, targetTurns map[int]bool) bool {
	if len(targetTurns) == 0 {
		return true
	}
	for turn := range targetTurns {
		if turn >= fromTurn && turn <= toTurn {
			return true
		}
	}
	return false
}

func filterMemoriesForTurnRange(items []store.Memory, sid string, fromTurn, toTurn int) []store.Memory {
	out := []store.Memory{}
	for _, item := range items {
		if item.ChatSessionID != sid || item.TurnIndex <= 0 {
			continue
		}
		if fromTurn > 0 && item.TurnIndex < fromTurn {
			continue
		}
		if toTurn > 0 && item.TurnIndex > toTurn {
			continue
		}
		out = append(out, item)
	}
	return out
}

func filterEvidenceForTurnRange(items []store.DirectEvidence, sid string, fromTurn, toTurn int) []store.DirectEvidence {
	out := []store.DirectEvidence{}
	for _, item := range items {
		if item.ChatSessionID != sid {
			continue
		}
		start := item.SourceTurnStart
		end := item.SourceTurnEnd
		if start <= 0 {
			start = item.TurnAnchor
		}
		if end <= 0 {
			end = start
		}
		if start <= 0 {
			continue
		}
		if toTurn > 0 && start > toTurn {
			continue
		}
		if fromTurn > 0 && end < fromTurn {
			continue
		}
		out = append(out, item)
	}
	return out
}
