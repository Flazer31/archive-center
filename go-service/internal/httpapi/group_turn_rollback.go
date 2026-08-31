package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

func (s *Server) handleRollback(w http.ResponseWriter, r *http.Request) {
	turnIndexStr := r.PathValue("turn_index")
	turnIndex, err := strconv.Atoi(turnIndexStr)
	if err != nil || turnIndex < 0 {
		writeError(w, http.StatusBadRequest, "bad_request", "invalid turn_index")
		return
	}

	sid := strings.TrimSpace(r.URL.Query().Get("chat_session_id"))
	if sid == "" {
		sid = "default"
	}
	reqSource := strings.TrimSpace(r.URL.Query().Get("req_source"))
	if reqSource == "" {
		reqSource = "unknown"
	}
	hostObservedAtMS, _ := strconv.ParseInt(strings.TrimSpace(r.URL.Query().Get("host_observed_at_ms")), 10, 64)
	decisionToken := strings.TrimSpace(r.URL.Query().Get("decision_token"))
	assistantObservationDigest := strings.TrimSpace(r.URL.Query().Get("assistant_observation_digest"))
	decisionVerified := false
	lifecycleAction := store.LogicalTurnLifecycleDeleted
	if decisionToken != "" {
		record, ok := s.rollbackDecisionLedger().consume(decisionToken, sid, turnIndex, assistantObservationDigest)
		if !ok {
			writeJSON(w, http.StatusConflict, map[string]any{
				"status": "blocked", "code": "rollback_decision_token_invalid",
				"chat_session_id": sid, "turn_index": turnIndex,
			})
			return
		}
		decisionVerified = true
		lifecycleAction = record.LifecycleAction
		// A one-use decision owns the operation identity from detection through
		// terminal persistence. Do not let a second query value split one delete
		// into unrelated HUD notices or audit sources.
		if decisionSource := strings.TrimSpace(record.RequestSource); decisionSource != "" {
			reqSource = decisionSource
		}
		if strings.TrimSpace(record.StableCharacterID) != "" || strings.TrimSpace(record.HostChatID) != "" {
			binding, resolveErr := resolveExistingRollbackRoute(
				r.Context(), s.Store, record.StableCharacterID, record.HostChatID,
			)
			if resolveErr != nil ||
				strings.TrimSpace(binding.CanonicalSessionID) != strings.TrimSpace(record.CanonicalSessionID) ||
				strings.TrimSpace(binding.CanonicalSessionID) != sid ||
				binding.Revision != record.RouteBindingRevision {
				writeJSON(w, http.StatusConflict, map[string]any{
					"status": "blocked", "code": "rollback_session_route_changed",
					"chat_session_id": sid, "turn_index": turnIndex,
				})
				return
			}
		}
	}
	requestedTurnIndex := turnIndex
	protectedBeforeTurn := intFromAny(r.URL.Query().Get("protected_before_turn"), 0)
	minFromTurn := intFromAny(r.URL.Query().Get("min_from_turn"), 0)
	if minFromTurn <= 0 && protectedBeforeTurn > 0 {
		minFromTurn = protectedBeforeTurn + 1
	}
	baselineClamped := false
	if minFromTurn > 0 && turnIndex < minFromTurn {
		turnIndex = minFromTurn
		baselineClamped = true
	}

	rollbackStore, hasRollback := s.Store.(store.RollbackStore)
	canonicalTailStore, hasCanonicalTail := s.Store.(store.LogicalTurnReplacementStore)
	if (!hasRollback && !hasCanonicalTail) || !s.usesShadowWriteStore() {
		rollbackPlan := buildRollbackPlan(sid, turnIndex, reqSource)
		rollbackPlan["requested_turn_index"] = requestedTurnIndex
		rollbackPlan["protected_before_turn"] = protectedBeforeTurn
		rollbackPlan["min_from_turn"] = minFromTurn
		rollbackPlan["session_routing_baseline_clamped"] = baselineClamped
		rollbackPlan["decision_verified"] = decisionVerified
		writeJSON(w, http.StatusOK, map[string]any{
			"status":          "ok",
			"source":          "shadow",
			"chat_session_id": sid,
			"turn_index":      turnIndex,
			"rollback_plan":   rollbackPlan,
			"note":            "rollback is a shadow plan; no mutations performed",
		})
		return
	}

	ctx := r.Context()
	deletions := map[string]any{}
	var delErrs []string
	// Stop and drain source-bound foreground/worker writes before invalidating
	// or deleting any derived rows. This prevents a canceled late Critic result
	// from recreating secondary artifacts after rollback cleanup has passed.
	if err := s.cancelCompleteTurnSourceWorkers(sid, turnIndex); err != nil {
		requestID := fmt.Sprintf("rollback:%s:%d:%s", sid, turnIndex, reqSource)
		view := s.turnWorkflowHUDOperationNotice(
			requestID,
			sid,
			turnIndex,
			"failed",
			"warning",
			"turn_hud.notice.delete_sync_failed",
			"turn_hud.error.delete_sync_interrupted",
			"COMPLETE_TURN_SOURCE_WORKER_STOP_TIMEOUT",
		)
		writeJSON(w, http.StatusConflict, map[string]any{
			"status":            "interrupted",
			"code":              "complete_turn_source_worker_stop_timeout",
			"retryable":         true,
			"chat_session_id":   sid,
			"turn_index":        turnIndex,
			"detail":            err.Error(),
			"turn_workflow_hud": view,
		})
		return
	}
	lifecycleOutbox := false
	canonicalRollback := false
	if hasCanonicalTail {
		err := canonicalTailStore.RollbackCanonicalTail(ctx, store.LogicalTurnRollback{
			ChatSessionID:   sid,
			TurnIndex:       turnIndex,
			LifecycleAction: lifecycleAction,
			Reason:          "turn_rollback",
			CreatedAt:       time.Now().UTC(),
		})
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			deletions["canonical_tail_transaction"] = map[string]any{"ok": false, "error": err.Error()}
			requestID := fmt.Sprintf("rollback:%s:%d:%s", sid, turnIndex, reqSource)
			rollbackHUDView := s.turnWorkflowHUDOperationNotice(
				requestID,
				sid, turnIndex, "failed", "error",
				"turn_hud.notice.delete_sync_failed",
				"turn_hud.error.delete_sync_partial",
				"ASSISTANT_OUTPUT_DELETE_SYNC_PARTIAL",
			)
			for _, fact := range rollbackHUDBlockedDeletionFacts() {
				setTurnWorkflowHUDFactValue(&rollbackHUDView, fact)
				if s.TurnWorkflows != nil {
					s.TurnWorkflows.setFact(requestID, fact)
				}
			}
			syncTurnWorkflowHUDPresentation(&rollbackHUDView)
			var rollbackHUD any = rollbackHUDView
			if s.TurnWorkflows != nil {
				rollbackHUD = s.turnWorkflowHUDSnapshot(requestID)
			}
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"status": "error", "code": "canonical_tail_rollback_failed",
				"chat_session_id": sid, "turn_index": turnIndex,
				"deletions":         deletions,
				"note":              "rollback transaction failed; MariaDB canonical state was not partially deleted",
				"turn_workflow_hud": rollbackHUD,
			})
			return
		}
		if err == nil {
			canonicalRollback = true
			lifecycleOutbox = true
			deletions["canonical_tail_transaction"] = map[string]any{
				"ok": true, "canonical_committed": true, "vector_cleanup": "durable_outbox",
			}
			deletions["source_revisions"] = map[string]any{"ok": true, "vector_cleanup": "durable_outbox"}
		}
	}
	if !canonicalRollback {
		if lifecycle, ok := s.Store.(store.SourceRevisionStore); ok {
			if availability, hasAvailability := s.Store.(store.MemoryDerivationLifecycleAvailability); !hasAvailability || availability.MemoryDerivationLifecycleEnabled() {
				if err := lifecycle.InvalidateSourceRevisions(ctx, sid, turnIndex, lifecycleAction, "turn_rollback", time.Now().UTC()); err != nil {
					deletions["source_revisions"] = map[string]any{"ok": false, "error": err.Error()}
					requestID := fmt.Sprintf("rollback:%s:%d:%s", sid, turnIndex, reqSource)
					rollbackHUDView := s.turnWorkflowHUDOperationNotice(
						requestID,
						sid, turnIndex, "failed", "error",
						"turn_hud.notice.delete_sync_failed",
						"turn_hud.error.delete_sync_partial",
						"ASSISTANT_OUTPUT_DELETE_SYNC_PARTIAL",
					)
					for _, fact := range rollbackHUDBlockedDeletionFacts() {
						setTurnWorkflowHUDFactValue(&rollbackHUDView, fact)
						if s.TurnWorkflows != nil {
							s.TurnWorkflows.setFact(requestID, fact)
						}
					}
					syncTurnWorkflowHUDPresentation(&rollbackHUDView)
					var rollbackHUD any = rollbackHUDView
					if s.TurnWorkflows != nil {
						rollbackHUD = s.turnWorkflowHUDSnapshot(requestID)
					}
					writeJSON(w, http.StatusInternalServerError, map[string]any{
						"status":            "error",
						"code":              "source_invalidation_failed",
						"chat_session_id":   sid,
						"turn_index":        turnIndex,
						"deletions":         deletions,
						"note":              "rollback stopped before canonical deletion",
						"turn_workflow_hud": rollbackHUD,
					})
					return
				} else {
					lifecycleOutbox = true
					deletions["source_revisions"] = map[string]any{"ok": true, "vector_cleanup": "durable_outbox"}
				}
			}
		}
	}
	var vectorIDs []string
	var vectorCollectErr error
	if !lifecycleOutbox {
		vectorIDs, vectorCollectErr = rollbackVectorDocumentIDs(ctx, s.Store, sid, turnIndex)
	}
	vectorCountBefore := -1
	vectorCountAfter := -1
	if s.Vector != nil {
		if count, err := s.Vector.Count(ctx, sid); err == nil {
			vectorCountBefore = count
		}
	}

	tables := []struct {
		name string
		fn   func() error
	}{
		{"chat_logs", func() error { return rollbackStore.DeleteChatLogs(ctx, sid, turnIndex) }},
		{"effective_inputs", func() error { return rollbackStore.DeleteEffectiveInputs(ctx, sid, turnIndex) }},
		{"memories", func() error { return rollbackStore.DeleteMemories(ctx, sid, turnIndex) }},
		{"direct_evidence", func() error { return rollbackStore.DeleteEvidence(ctx, sid, turnIndex) }},
		{"kg_triples", func() error { return rollbackStore.DeleteKGTriples(ctx, sid, turnIndex) }},
		{"critic_feedback", func() error { return rollbackStore.DeleteCriticFeedback(ctx, sid, turnIndex) }},
		{"character_events", func() error { return rollbackStore.DeleteCharacterEvents(ctx, sid, turnIndex) }},
		{"entities", func() error { return rollbackStore.DeleteEntities(ctx, sid, turnIndex) }},
		{"trust_states", func() error { return rollbackStore.DeleteTrustStates(ctx, sid, turnIndex) }},
		{"storylines", func() error { return rollbackStore.DeleteStorylines(ctx, sid, turnIndex) }},
		{"world_rules", func() error { return rollbackStore.DeleteWorldRules(ctx, sid, turnIndex) }},
		{"character_states", func() error { return rollbackStore.DeleteCharacterStates(ctx, sid, turnIndex) }},
		{"pending_threads", func() error { return rollbackStore.DeletePendingThreads(ctx, sid, turnIndex) }},
		{"active_states", func() error { return rollbackStore.DeleteActiveStates(ctx, sid, turnIndex) }},
		{"canonical_state_layers", func() error { return rollbackStore.DeleteCanonicalStateLayers(ctx, sid, turnIndex) }},
		{"episode_summaries", func() error { return rollbackStore.DeleteEpisodeSummaries(ctx, sid, turnIndex) }},
		{"guidance_plan_states", func() error { return rollbackStore.DeleteGuidancePlanState(ctx, sid, turnIndex) }},
		{"chapter_summaries", func() error { return rollbackStore.DeleteChapterSummaries(ctx, sid, turnIndex) }},
		{"arc_summaries", func() error { return rollbackStore.DeleteArcSummaries(ctx, sid, turnIndex) }},
		{"saga_digests", func() error { return rollbackStore.DeleteSagaDigests(ctx, sid, turnIndex) }},
		{"session_active_scopes", func() error { return rollbackStore.DeleteSessionActiveScopes(ctx, sid, turnIndex) }},
		{"subjective_entity_memories", func() error { return rollbackStore.DeleteProtagonistEntityMemories(ctx, sid, turnIndex) }},
		{"consequence_records", func() error { return rollbackStore.DeleteConsequenceRecords(ctx, sid, turnIndex) }},
		{"psychology_branches", func() error { return rollbackStore.DeletePsychologyBranches(ctx, sid, turnIndex) }},
		{"theme_offscreen_carries", func() error { return rollbackStore.DeleteThemeOffscreenCarries(ctx, sid, turnIndex) }},
		{"capture_verification_records", func() error { return rollbackStore.DeleteCaptureVerificationRecords(ctx, sid, turnIndex) }},
		{"status_current_values", func() error { return rollbackStore.DeleteStatusCurrentValues(ctx, sid, turnIndex) }},
		{"status_change_events", func() error { return rollbackStore.DeleteStatusChangeEvents(ctx, sid, turnIndex) }},
		{"status_effects", func() error { return rollbackStore.DeleteStatusEffects(ctx, sid, turnIndex) }},
	}

	for _, t := range tables {
		if canonicalRollback {
			deletions[t.name] = map[string]any{"ok": true, "mode": "canonical_tail_transaction"}
			continue
		}
		if err := t.fn(); err != nil {
			deletions[t.name] = map[string]any{"ok": false, "error": err.Error()}
			delErrs = append(delErrs, fmt.Sprintf("%s: %v", t.name, err))
		} else {
			deletions[t.name] = map[string]any{"ok": true}
		}
	}
	if cleared, err := clearReferenceRuntimeCandidatesAfterRollback(ctx, s.Store, sid, turnIndex); err != nil {
		deletions["reference_runtime"] = map[string]any{"ok": false, "error": err.Error()}
		delErrs = append(delErrs, fmt.Sprintf("reference runtime: %v", err))
	} else {
		deletions["reference_runtime"] = map[string]any{"ok": true, "cleared_candidates": cleared, "bindings_preserved": true}
	}
	if restored, err := restoreNarrativeCurrentStatesAfterRollback(ctx, s.Store, sid, turnIndex-1); err != nil {
		deletions["narrative_current_state_restore"] = map[string]any{"ok": false, "error": err.Error()}
		delErrs = append(delErrs, fmt.Sprintf("narrative current state restore: %v", err))
	} else {
		deletions["narrative_current_state_restore"] = map[string]any{"ok": true, "restored": restored}
	}
	if canonicalRollback {
		deletions["story_clock_current_restore"] = map[string]any{"ok": true, "mode": "canonical_tail_transaction"}
	} else if restored, err := restoreStoryClockCurrentAfterRollback(ctx, s.Store, sid); err != nil {
		deletions["story_clock_current_restore"] = map[string]any{"ok": false, "error": err.Error()}
		delErrs = append(delErrs, fmt.Sprintf("story clock current restore: %v", err))
	} else {
		deletions["story_clock_current_restore"] = map[string]any{"ok": true, "restored": restored}
	}
	if canonicalRollback {
		deletions["reversible_state_current_restore"] = map[string]any{"ok": true, "mode": "canonical_tail_transaction"}
	} else if restored, err := restoreReversibleStateCurrentAfterRollback(ctx, s.Store, sid); err != nil {
		deletions["reversible_state_current_restore"] = map[string]any{"ok": false, "error": err.Error()}
		delErrs = append(delErrs, fmt.Sprintf("reversible state current restore: %v", err))
	} else {
		deletions["reversible_state_current_restore"] = map[string]any{"ok": true, "restored": restored}
	}
	if lifecycleOutbox {
		s.wakeMemoryWorkers()
		deletions["vectors"] = map[string]any{
			"ok":                  true,
			"mode":                "durable_outbox",
			"canonical_committed": true,
			"vector_cleanup":      "queued",
			"queued":              true,
			"drain_attempted":     false,
			"processed":           0,
			"canonical_note":      "MariaDB invalidation is committed; vector cleanup is queued for the bounded worker",
		}
	} else if vectorCollectErr != nil {
		deletions["vectors"] = map[string]any{"ok": false, "attempted": false, "error": vectorCollectErr.Error()}
		delErrs = append(delErrs, fmt.Sprintf("vectors: collect rollback ids: %v", vectorCollectErr))
	} else if len(vectorIDs) == 0 {
		deletions["vectors"] = map[string]any{"ok": true, "attempted": false, "deleted_ids": 0}
	} else if s.Vector == nil {
		deletions["vectors"] = map[string]any{"ok": true, "attempted": false, "deleted_ids": 0, "warning": "vector store is not configured"}
	} else if deleter, ok := s.Vector.(vector.DocumentDeleter); ok {
		if err := deleter.DeleteDocuments(ctx, vectorIDs); err != nil {
			if errors.Is(err, vector.ErrNotEnabled) {
				deletions["vectors"] = map[string]any{"ok": true, "attempted": true, "deleted_ids": 0, "warning": "vector store is not enabled"}
			} else {
				deletions["vectors"] = map[string]any{"ok": false, "attempted": true, "deleted_ids": 0, "error": err.Error()}
				delErrs = append(delErrs, fmt.Sprintf("vectors: %v", err))
			}
		} else {
			deletions["vectors"] = map[string]any{"ok": true, "attempted": true, "deleted_ids": len(vectorIDs)}
		}
	} else {
		deletions["vectors"] = map[string]any{"ok": true, "attempted": false, "deleted_ids": 0, "warning": "vector store does not support document delete"}
	}
	if s.Vector != nil && !lifecycleOutbox {
		if count, err := s.Vector.Count(ctx, sid); err == nil {
			vectorCountAfter = count
		}
	}
	vectorOrphanCheck := map[string]any{
		"status":                 "bounded",
		"policy":                 "known_doc_ids_deleted_then_session_vector_count_checked",
		"known_delete_id_count":  len(vectorIDs),
		"session_count_before":   nilIfNegative(vectorCountBefore),
		"session_count_after":    nilIfNegative(vectorCountAfter),
		"full_listing_available": false,
	}
	if lifecycleOutbox {
		vectorOrphanCheck["status"] = "queued"
		vectorOrphanCheck["policy"] = "durable_outbox_cleanup_queued"
		vectorOrphanCheck["known_delete_id_count"] = nil
	}
	if s.Vector != nil && !lifecycleOutbox {
		fullAudit := s.adminVectorOrphanAudit(ctx, sid, false)
		if available, _ := fullAudit["full_listing_available"].(bool); available {
			fullAudit["status"] = "full"
			fullAudit["policy"] = "post_rollback_full_chromadb_listing_compared_with_mariadb_canonical_rows"
			if lifecycleOutbox {
				fullAudit["known_delete_id_count"] = nil
			} else {
				fullAudit["known_delete_id_count"] = len(vectorIDs)
			}
			fullAudit["session_count_before"] = nilIfNegative(vectorCountBefore)
			fullAudit["session_count_after"] = nilIfNegative(vectorCountAfter)
			vectorOrphanCheck = fullAudit
		} else {
			vectorOrphanCheck["full_audit"] = fullAudit
		}
	}
	deletions["vector_orphan_check"] = vectorOrphanCheck
	if err := s.Store.SaveAuditLog(ctx, &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "rollback",
		TargetType:    "turn",
		TargetID:      int64(turnIndex),
		Summary:       fmt.Sprintf("rollback from turn %d", turnIndex),
		DetailsJSON:   fmt.Sprintf(`{"turn_index":%d,"req_source":%q,"vector_ids":%d}`, turnIndex, reqSource, len(vectorIDs)),
		Source:        reqSource,
		CreatedAt:     time.Now().UTC(),
	}); err != nil {
		deletions["rollback_audit"] = map[string]any{"ok": false, "error": err.Error()}
		delErrs = append(delErrs, fmt.Sprintf("rollback_audit: %v", err))
	} else {
		deletions["rollback_audit"] = map[string]any{"ok": true, "source": reqSource}
	}
	s.invalidateCompleteTurnSourceAcceptances(ctx, sid, turnIndex, reqSource, hostObservedAtMS)

	status := "ok"
	note := "rollback executed"
	hudStatus := "completed"
	hudSeverity := "info"
	hudTitleKey := "turn_hud.notice.delete_confirmed"
	hudMessageKey := "turn_hud.notice.delete_confirmed_detail"
	hudNoticeCode := "ASSISTANT_OUTPUT_DELETE_CONFIRMED"
	if len(delErrs) > 0 {
		status = "partial_error"
		note = "rollback executed with partial errors"
		hudStatus = "failed"
		hudSeverity = "error"
		hudTitleKey = "turn_hud.notice.delete_sync_failed"
		hudMessageKey = "turn_hud.error.delete_sync_partial"
		hudNoticeCode = "ASSISTANT_OUTPUT_DELETE_SYNC_PARTIAL"
	}
	var rollbackHUD any
	if decisionVerified || reqSource == "auto" || reqSource == "auto_rollback" || reqSource == "manual" {
		requestID := fmt.Sprintf("rollback:%s:%d:%s", sid, turnIndex, reqSource)
		view := s.turnWorkflowHUDOperationNotice(
			requestID,
			sid,
			turnIndex,
			hudStatus,
			hudSeverity,
			hudTitleKey,
			hudMessageKey,
			hudNoticeCode,
		)
		derivedKeys := make([]string, 0, len(tables))
		for _, table := range tables {
			if table.name != "chat_logs" && table.name != "effective_inputs" {
				derivedKeys = append(derivedKeys, table.name)
			}
		}
		derivedKeys = append(derivedKeys, "reference_runtime", "narrative_current_state_restore")
		facts := []turnWorkflowHUDFact{
			rollbackHUDDeletionFact("raw_persistence", "canonical_store", deletions, []string{"chat_logs", "effective_inputs"}),
			rollbackHUDDeletionFact("derived_memory", "canonical_store", deletions, derivedKeys),
			rollbackHUDVectorDeletionFact(deletions),
		}
		for _, fact := range facts {
			setTurnWorkflowHUDFactValue(&view, fact)
			if s.TurnWorkflows != nil {
				s.TurnWorkflows.setFact(requestID, fact)
			}
		}
		syncTurnWorkflowHUDPresentation(&view)
		rollbackHUD = view
		if s.TurnWorkflows != nil {
			rollbackHUD = s.turnWorkflowHUDSnapshot(requestID)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":          status,
		"source":          s.storeWriteSource(),
		"chat_session_id": sid,
		"turn_index":      turnIndex,
		"rollback_plan": map[string]any{
			"status":                           "executed",
			"source":                           s.storeWriteSource(),
			"chat_session_id":                  sid,
			"turn_index":                       turnIndex,
			"requested_turn_index":             requestedTurnIndex,
			"req_source":                       reqSource,
			"protected_before_turn":            protectedBeforeTurn,
			"min_from_turn":                    minFromTurn,
			"session_routing_baseline_clamped": baselineClamped,
			"decision_verified":                decisionVerified,
			"would_delete":                     true,
			"would_write":                      true,
			"mutation_enabled":                 true,
			"sync_replay_gate":                 true,
			"save_update_delete_gate":          true,
			"stale_vector_replay_gate":         true,
			"rollback_vector_delete_gate":      true,
			"rebuild_replay_gate":              false,
			"vector_doc_delete_policy":         "canonical_row_first_then_vector",
			"stale_summary_policy":             "tombstone_before_rebuild",
			"turn_delete_policy":               "tail_from_earliest_deleted_turn",
			"hierarchy_invalidation":           "delete_overlapping_episode_chapter_arc_saga_ranges",
			"step23_invalidation":              "delete_turn_scoped_support_records_from_from_turn",
			"rebuild_owner":                    "chroma_shadow_orchestrator",
		},
		"deletions":         deletions,
		"errors":            delErrs,
		"turn_workflow_hud": rollbackHUD,
		"note":              note,
	})
}

func rollbackHUDBlockedDeletionFacts() []turnWorkflowHUDFact {
	facts := []turnWorkflowHUDFact{{
		Key: "finality", Owner: "go_backend", Scope: "current_request",
		Status: "blocked", Disposition: "dropped",
		ReasonCode: "source_invalidation_failed", Severity: turnWorkflowHUDSeverityError,
	}}
	for _, item := range []struct {
		key   string
		owner string
	}{
		{key: "raw_persistence", owner: "canonical_store"},
		{key: "derived_memory", owner: "canonical_store"},
		{key: "vector_index", owner: "vector_store"},
	} {
		facts = append(facts, turnWorkflowHUDFact{
			Key: item.key, Owner: item.owner, Scope: "current_turn",
			Status: "blocked", Disposition: "dropped",
			ReasonCode: "source_invalidation_failed", Severity: turnWorkflowHUDSeverityError,
		})
	}
	return facts
}

func rollbackHUDDeletionFact(
	key string,
	owner string,
	deletions map[string]any,
	deletionKeys []string,
) turnWorkflowHUDFact {
	fact := turnWorkflowHUDFact{
		Key: key, Owner: owner, Scope: "current_turn",
		Status: "unobserved", Disposition: "deferred",
		ReasonCode: "rollback_lane_unobserved", Severity: turnWorkflowHUDSeverityNotice,
	}
	observed := 0
	failed := 0
	for _, deletionKey := range deletionKeys {
		result := mapFromAny(deletions[deletionKey])
		if len(result) == 0 {
			continue
		}
		observed++
		ok, typed := result["ok"].(bool)
		if !typed || !ok {
			failed++
		}
	}
	switch {
	case observed == 0:
		return fact
	case failed > 0:
		fact.Status = "partial_error"
		fact.Disposition = "dropped"
		fact.ReasonCode = "rollback_lane_partial_error"
		fact.Severity = turnWorkflowHUDSeverityError
	default:
		fact.Status = "deleted"
		fact.Disposition = "dropped"
		fact.ReasonCode = "rollback_lane_deleted"
		fact.Severity = turnWorkflowHUDSeverityNotice
	}
	return fact
}

func rollbackHUDVectorDeletionFact(deletions map[string]any) turnWorkflowHUDFact {
	fact := turnWorkflowHUDFact{
		Key: "vector_index", Owner: "vector_store", Scope: "current_turn",
		Status: "unobserved", Disposition: "deferred",
		ReasonCode: "rollback_vector_unobserved", Severity: turnWorkflowHUDSeverityNotice,
	}
	result := mapFromAny(deletions["vectors"])
	if len(result) == 0 {
		return fact
	}
	if queued, _ := result["queued"].(bool); queued || strings.EqualFold(strings.TrimSpace(extractionStringFromAny(result["vector_cleanup"])), "queued") {
		fact.Status = "queued"
		fact.Disposition = "deferred"
		fact.ReasonCode = "rollback_vector_cleanup_queued"
		fact.Severity = turnWorkflowHUDSeverityNotice
		fact.Count = intValuePtr(0)
		return fact
	}
	if permanent := intFromAny(result["permanent"], 0); permanent > 0 {
		fact.Status = "partial_error"
		fact.Disposition = "dropped"
		fact.ReasonCode = "rollback_vector_permanent_failure"
		fact.Severity = turnWorkflowHUDSeverityError
		fact.Count = intValuePtr(permanent)
		return fact
	}
	if ok, typed := result["ok"].(bool); !typed || !ok {
		fact.Status = "failed"
		fact.Disposition = "dropped"
		fact.ReasonCode = "rollback_vector_delete_failed"
		fact.Severity = turnWorkflowHUDSeverityError
		return fact
	}
	if retryable := intFromAny(result["retryable_queued"], 0); retryable > 0 {
		fact.Status = "queued"
		fact.Disposition = "deferred"
		fact.ReasonCode = "rollback_vector_retry_queued"
		fact.Severity = turnWorkflowHUDSeverityNotice
		fact.Count = intValuePtr(retryable)
		return fact
	}
	if warning := strings.TrimSpace(extractionStringFromAny(result["warning"])); warning != "" {
		fact.Status = "skipped"
		fact.Disposition = "dropped"
		fact.ReasonCode = "rollback_vector_unavailable"
		fact.Severity = turnWorkflowHUDSeverityWarning
		fact.Count = intValuePtr(0)
		return fact
	}
	count := intFromAny(result["deleted_ids"], intFromAny(result["completed"], 0))
	fact.Count = intValuePtr(maxInt(0, count))
	if count == 0 {
		fact.Status = "empty"
		fact.Disposition = "dropped"
		fact.ReasonCode = "rollback_vector_empty"
		fact.Severity = turnWorkflowHUDSeverityNormal
		return fact
	}
	fact.Status = "deleted"
	fact.Disposition = "dropped"
	fact.ReasonCode = "rollback_vector_deleted"
	fact.Severity = turnWorkflowHUDSeverityNotice
	return fact
}

func clearReferenceRuntimeCandidatesAfterRollback(ctx context.Context, base store.Store, sid string, fromTurn int) (int, error) {
	ref, ok := base.(store.ReferenceLibraryStore)
	if !ok {
		return 0, nil
	}
	bindings, err := ref.ListSessionReferenceBindings(ctx, sid, false)
	if err != nil {
		return 0, err
	}
	cleared := 0
	for _, binding := range bindings {
		runtimeState, err := ref.GetSessionReferenceRuntime(ctx, binding.BindingID)
		if errors.Is(err, store.ErrNotFound) {
			continue
		}
		if err != nil {
			return cleared, err
		}
		if runtimeState.CandidateSourceTurn <= 0 || runtimeState.CandidateSourceTurn < fromTurn {
			continue
		}
		runtimeState.CandidateNodeID = ""
		runtimeState.CandidateSourceTurn = 0
		runtimeState.CandidateEvidenceJSON = ""
		runtimeState.CandidateConfirmed = false
		runtimeState.LastClaimIDsJSON = ""
		runtimeState.DiagnosticsJSON = `{"reason":"candidate_source_turn_rolled_back"}`
		if err := ref.UpsertSessionReferenceRuntime(ctx, runtimeState, runtimeState.Revision); err != nil {
			return cleared, err
		}
		cleared++
	}
	return cleared, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func rollbackVectorDocumentIDs(ctx context.Context, st store.Store, sid string, fromTurn int) ([]string, error) {
	if st == nil {
		return nil, nil
	}
	ids := []string{}
	seen := map[string]bool{}
	add := func(candidates ...string) {
		for _, id := range candidates {
			id = strings.TrimSpace(id)
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			ids = append(ids, id)
		}
	}

	memories, err := st.ListMemories(ctx, sid, fromTurn, 0)
	if err != nil {
		if !errors.Is(err, store.ErrNotEnabled) {
			return nil, err
		}
	} else {
		for _, mem := range memories {
			if mem.ChatSessionID != "" && mem.ChatSessionID != sid {
				continue
			}
			if mem.ID > 0 {
				add(memoryVectorDocumentID(sid, mem), rollbackVectorDocumentAlias("memory", sid, mem.ID), rollbackVectorDocumentLegacyAlias("memory", mem.ID))
			}
		}
	}

	evidence, err := st.ListEvidence(ctx, sid)
	if err != nil {
		if !errors.Is(err, store.ErrNotEnabled) {
			return nil, err
		}
	} else {
		for _, item := range evidence {
			if item.ID > 0 && item.SourceTurnEnd >= fromTurn {
				add(rollbackVectorDocumentAlias("evidence", sid, item.ID), rollbackVectorDocumentLegacyAlias("evidence", item.ID))
			}
		}
	}

	worldRules, err := st.ListWorldRules(ctx, sid)
	if err != nil {
		if !errors.Is(err, store.ErrNotEnabled) {
			return nil, err
		}
	} else {
		for _, item := range worldRules {
			if item.ID > 0 && item.SourceTurn >= fromTurn {
				add(rollbackVectorDocumentAlias("world_rule", sid, item.ID), rollbackVectorDocumentLegacyAlias("world_rule", item.ID))
			}
		}
	}

	triples, err := st.ListKGTriples(ctx, sid)
	if err != nil {
		if !errors.Is(err, store.ErrNotEnabled) {
			return nil, err
		}
	} else {
		for _, item := range triples {
			if item.ID > 0 && (item.SourceTurn >= fromTurn || item.ValidFrom >= fromTurn) {
				add(rollbackVectorDocumentAlias("kg_triple", sid, item.ID), rollbackVectorDocumentLegacyAlias("kg_triple", item.ID))
			}
		}
	}

	episodes, err := st.ListEpisodeSummaries(ctx, sid, 0, 0, 0)
	if err != nil {
		if !errors.Is(err, store.ErrNotEnabled) {
			return nil, err
		}
	} else {
		for _, item := range episodes {
			if item.ID > 0 && (item.ToTurn >= fromTurn || item.FromTurn >= fromTurn) {
				add(rollbackVectorDocumentAlias("episode", sid, item.ID), rollbackVectorDocumentLegacyAlias("episode", item.ID))
			}
		}
	}

	if chapterStore, ok := st.(store.ChapterSummaryStore); ok {
		chapters, err := chapterStore.SearchChapterSummaries(ctx, sid, "", 0, 0, 0)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			return nil, err
		}
		for _, item := range chapters {
			if item.ID > 0 && (item.ToTurn >= fromTurn || item.FromTurn >= fromTurn) {
				add(rollbackVectorDocumentAlias("chapter", sid, item.ID), rollbackVectorDocumentLegacyAlias("chapter", item.ID))
			}
		}
	}

	if arcStore, ok := st.(store.ArcSummaryStore); ok {
		arcs, err := arcStore.ListArcSummaries(ctx, sid, "", 0)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			return nil, err
		}
		for _, item := range arcs {
			if item.ID > 0 && (item.ToTurn >= fromTurn || item.FromTurn >= fromTurn) {
				add(rollbackVectorDocumentAlias("arc", sid, item.ID), rollbackVectorDocumentLegacyAlias("arc", item.ID))
			}
		}
	}

	if sagaStore, ok := st.(store.SagaDigestStore); ok {
		sagas, err := sagaStore.ListSagaDigests(ctx, sid, 0)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			return nil, err
		}
		for _, item := range sagas {
			if item.ID > 0 && (item.ToTurn >= fromTurn || item.FromTurn >= fromTurn) {
				add(rollbackVectorDocumentAlias("saga", sid, item.ID), rollbackVectorDocumentLegacyAlias("saga", item.ID))
			}
		}
	}
	return ids, nil
}

func rollbackVectorDocumentAlias(tier, sid string, rowID int64) string {
	tier = strings.TrimSpace(tier)
	sid = strings.TrimSpace(sid)
	if tier == "" || sid == "" || rowID <= 0 {
		return ""
	}
	return fmt.Sprintf("%s:%s:%d", tier, sid, rowID)
}

func rollbackVectorDocumentLegacyAlias(tier string, rowID int64) string {
	tier = strings.TrimSpace(tier)
	if tier == "" || rowID <= 0 {
		return ""
	}
	return fmt.Sprintf("%s:%d", tier, rowID)
}

func memoryVectorDocumentID(sid string, mem store.Memory) string {
	sid = strings.TrimSpace(sid)
	if sid == "" {
		return ""
	}
	sourceRowID := ""
	if mem.ID > 0 {
		sourceRowID = strconv.FormatInt(mem.ID, 10)
	} else if mem.TurnIndex > 0 {
		sourceRowID = fmt.Sprintf("turn_%d_memory", mem.TurnIndex)
	}
	if sourceRowID == "" {
		return ""
	}
	return fmt.Sprintf("memory:%s:%s", sid, sourceRowID)
}

func nilIfNegative(value int) any {
	if value < 0 {
		return nil
	}
	return value
}

func vectorDocumentSearchPreview(docs []vector.VectorDocument) []map[string]any {
	out := make([]map[string]any, 0, len(docs))
	for _, doc := range docs {
		item := map[string]any{
			"id":              doc.ID,
			"tier":            doc.Tier,
			"chat_session_id": doc.ChatSessionID,
			"source_table":    doc.SourceTable,
			"source_row_id":   doc.SourceRowID,
			"schema_version":  doc.SchemaVersion,
			"preview":         truncateTextForShadow(doc.DocumentText, 240),
		}
		if doc.SimilarityAvailable {
			item["similarity"] = doc.Similarity
			item["distance"] = doc.Distance
			item["similarity_source"] = doc.SimilaritySource
		}
		if strings.TrimSpace(doc.SearchTextPolicy) != "" {
			item["search_text_policy"] = strings.TrimSpace(doc.SearchTextPolicy)
		}
		if strings.TrimSpace(doc.RawLanguage) != "" {
			item["raw_language"] = strings.TrimSpace(doc.RawLanguage)
		}
		if strings.TrimSpace(doc.SummaryLanguage) != "" {
			item["summary_language"] = strings.TrimSpace(doc.SummaryLanguage)
		}
		if strings.TrimSpace(doc.SessionOutputLanguage) != "" {
			item["session_output_language"] = strings.TrimSpace(doc.SessionOutputLanguage)
		}
		if doc.AliasCount > 0 {
			item["alias_count"] = doc.AliasCount
		}
		if doc.MigrationID > 0 {
			item["migration_id"] = doc.MigrationID
		}
		if strings.TrimSpace(doc.MigratedFromSessionID) != "" {
			item["migrated_from_session_id"] = doc.MigratedFromSessionID
		}
		out = append(out, item)
	}
	return out
}

func hasStructuredFeedback(msgs []map[string]any) bool {
	for _, m := range msgs {
		keys := []string{"score", "rating", "feedback_type", "category", "suggestion", "correction", "issues", "improvements", "critique", "review"}
		for _, k := range keys {
			if _, ok := m[k]; ok {
				return true
			}
		}
	}
	return false
}

func completeTurnPreserveRequestedTurnIndex(meta map[string]any) bool {
	if completeTurnBoolFromAny(meta["preserve_requested_turn_index"]) {
		return true
	}
	activeBackfill := mapFromAny(meta["active_chat_backfill"])
	if completeTurnBoolFromAny(activeBackfill["preserve_requested_turn_index"]) {
		return true
	}
	source := strings.TrimSpace(fmt.Sprint(activeBackfill["source"]))
	switch source {
	case "active_chat_recent_rebuild", "risu_active_chat_complete_turn_backfill":
		return true
	default:
		return false
	}
}

func hasImprovementTrace(trace *map[string]any) bool {
	if trace == nil || len(*trace) == 0 {
		return false
	}
	keys := []string{"score", "rating", "feedback_type", "category", "suggestion", "correction", "issues", "improvements", "critique", "review"}
	for _, k := range keys {
		if _, ok := (*trace)[k]; ok {
			return true
		}
	}
	return false
}

func stringPtrValue(v *string, fallback string) string {
	if v == nil {
		return fallback
	}
	return strings.TrimSpace(*v)
}

func intPtrValue(v *int, fallback int) int {
	if v == nil {
		return fallback
	}
	return *v
}
