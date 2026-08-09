package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

type completeTurnPersistenceDiagnostic struct {
	Operation string `json:"operation"`
	Cause     string `json:"cause"`
}

func (s *Server) completeTurnPersistenceDiagnostics(details []string) []completeTurnPersistenceDiagnostic {
	out := make([]completeTurnPersistenceDiagnostic, 0, len(details))
	seen := map[string]bool{}
	runtime := s.runtimeConfigSnapshot()
	secrets := []string{
		runtime.MainAPIKey,
		runtime.CriticAPIKey,
		runtime.SupervisorAPIKey,
		runtime.EmbeddingAPIKey,
		runtime.SourceSearchPlannerAPIKey,
		s.Cfg.MariaDBDSN,
	}
	for _, raw := range details {
		text := strings.TrimSpace(scrubCriticFailureText(raw, ""))
		for _, secret := range secrets {
			if secret = strings.TrimSpace(secret); secret != "" {
				text = strings.ReplaceAll(text, secret, "[redacted]")
			}
		}
		if text == "" || seen[text] {
			continue
		}
		seen[text] = true
		parts := strings.SplitN(text, ":", 2)
		operation := strings.TrimSpace(parts[0])
		cause := text
		if len(parts) == 2 {
			cause = strings.TrimSpace(parts[1])
		}
		if operation == "" {
			operation = "persistence"
		}
		out = append(out, completeTurnPersistenceDiagnostic{
			Operation: truncateRunes(operation, 120),
			Cause:     truncateRunes(cause, 600),
		})
	}
	return out
}

func completeTurnPersistenceRollbackState(diagnostics []completeTurnPersistenceDiagnostic, committed, errors int) string {
	for _, diagnostic := range diagnostics {
		if diagnostic.Operation == "CommitMemoryAdmission" {
			return "atomic_rollback"
		}
	}
	if errors <= 0 {
		return "not_applicable"
	}
	if committed > 0 {
		return "partial_commit"
	}
	return "no_commit"
}

func completeTurnPersistenceFailureSummary(diagnostics []completeTurnPersistenceDiagnostic) string {
	if len(diagnostics) == 0 {
		return "derived_persist_failed"
	}
	return "derived_persist_failed: operation=" + diagnostics[0].Operation + "; cause=" + diagnostics[0].Cause
}

func completeTurnPersistenceDiagnosticMessages(diagnostics []completeTurnPersistenceDiagnostic) []string {
	out := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		out = append(out, diagnostic.Operation+": "+diagnostic.Cause)
	}
	return out
}

func completeTurnPersistenceHUDDetails(
	diagnostics []completeTurnPersistenceDiagnostic,
	attempted, committed int,
	rollbackState string,
	reprocessingDurable bool,
) []turnWorkflowHUDDetail {
	details := []turnWorkflowHUDDetail{
		{Key: "derived_attempted", Value: strconv.Itoa(maxInt(0, attempted))},
		{Key: "derived_committed", Value: strconv.Itoa(maxInt(0, committed))},
		{Key: "transaction", Value: strings.TrimSpace(rollbackState)},
	}
	if reprocessingDurable {
		details = append(details, turnWorkflowHUDDetail{Key: "reprocessing", Value: "queued"})
	}
	for _, diagnostic := range diagnostics {
		details = append(details,
			turnWorkflowHUDDetail{Key: "operation", Value: diagnostic.Operation},
			turnWorkflowHUDDetail{Key: "cause", Value: diagnostic.Cause},
		)
	}
	return details
}

func (s *Server) handleCompleteTurn(w http.ResponseWriter, r *http.Request) {
	var req dto.M4CompleteTurnRequest
	if err := dto.DecodeWithDefaults(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	workflowRequestID := completeTurnWorkflowRequestID(req)

	s.executeCompleteTurnIdempotent(
		r.Context(),
		w,
		completeTurnIdempotencyKey(req.ClientMeta),
		completeTurnRequestFingerprint(req),
		func(target http.ResponseWriter) {
			acceptance := s.beginCompleteTurnSourceAcceptance(r.Context(), req)
			if acceptance.Enabled && !acceptance.Accepted {
				if s.TurnWorkflows != nil && workflowRequestID != "" {
					s.TurnWorkflows.invalidate(workflowRequestID, acceptance.Reason)
				}
				writeCompleteTurnSourceAcceptanceRejection(target, req, acceptance)
				return
			}
			if acceptance.Enabled && acceptance.BoundTurn > 0 {
				req.TurnIndex = acceptance.BoundTurn
			}
			s.handleCompleteTurnDecoded(target, r, req, acceptance)
		},
	)
}

func (s *Server) handleCompleteTurnDecoded(w http.ResponseWriter, r *http.Request, req dto.M4CompleteTurnRequest, sourceAcceptance completeTurnSourceAcceptanceDecision) {
	timing := newBackendTimingTrace("complete_turn.backend_timing.v1")
	preflightStartedAt := time.Now()
	workflowRequestID := completeTurnWorkflowRequestID(req)
	defer func() {
		if s.TurnWorkflows == nil || workflowRequestID == "" {
			return
		}
		view, ok := s.TurnWorkflows.snapshot(workflowRequestID)
		if !ok || turnWorkflowHUDTerminal(view.Status) {
			return
		}
		stageKey := turnWorkflowStageFinalAccepted
		if view.CurrentStage != nil && strings.TrimSpace(view.CurrentStage.Key) != "" {
			stageKey = view.CurrentStage.Key
		}
		s.TurnWorkflows.fail(workflowRequestID, "COMPLETE_TURN_ABORTED", "turn_hud.error.complete_turn_aborted", stageKey, true)
	}()
	sid := strings.TrimSpace(req.ChatSessionID)
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}

	ctx := r.Context()
	if lock, err := s.sessionMigrationSourceLock(ctx, sid); err != nil {
		writeInternalError(w, err.Error())
		return
	} else if lock != nil {
		s.writeCompleteTurnMigrationSourceLockBlocked(w, req, sid, workflowRequestID, lock)
		return
	}
	if s.TurnWorkflows != nil && workflowRequestID != "" {
		s.TurnWorkflows.setLogicalTurn(workflowRequestID, req.TurnIndex)
		s.TurnWorkflows.startStage(workflowRequestID, turnWorkflowStageFinalAccepted)
		payloadObservation := mapFromAny(req.ClientMeta["source_to_final_lineage_observation"])
		payloadStatus := strings.TrimSpace(extractionStringFromAny(payloadObservation["payload_application_status"]))
		payloadStage := strings.TrimSpace(extractionStringFromAny(payloadObservation["payload_observation_stage"]))
		payloadFact := turnWorkflowHUDFact{
			Key:         "payload_delivery",
			Owner:       "risu_host",
			Scope:       "current_request",
			Status:      firstNonEmpty(payloadStatus, "unobserved"),
			Disposition: "deferred",
			ReasonCode:  "source_to_final_payload_application_unobserved",
			Severity:    turnWorkflowHUDSeverityNotice,
		}
		if (payloadStatus == "applied" || payloadStatus == "empty") && payloadStage == "archive_center_before_request_return" {
			payloadFact.Disposition = "delivered"
			payloadFact.ReasonCode = "risu_host_payload_application_observed"
			payloadFact.Severity = turnWorkflowHUDSeverityNormal
		} else if payloadStatus != "" && payloadStatus != "applied" && payloadStatus != "empty" {
			payloadFact.Disposition = "dropped"
			payloadFact.ReasonCode = "risu_host_payload_application_rejected"
			payloadFact.Severity = turnWorkflowHUDSeverityWarning
		}
		s.TurnWorkflows.setFact(workflowRequestID, payloadFact)
		s.TurnWorkflows.setFact(workflowRequestID, turnWorkflowHUDFact{
			Key:         "finality",
			Owner:       "go_backend",
			Scope:       "current_request",
			Status:      "active_final",
			Disposition: "delivered",
			ReasonCode:  "active_final_source_accepted",
			Severity:    turnWorkflowHUDSeverityNormal,
		})
	}
	userText := sanitizeCriticStorageText(*req.UserInput)
	assistantText := sanitizeCriticStorageText(*req.AssistantContent)
	actualEmptyUserInput := completeTurnActualEmptyUserInput(req.ClientMeta)
	if strings.TrimSpace(userText) == "" && actualEmptyUserInput {
		userText = completeTurnAutoContinueUserInputMarker
	}
	content := strings.TrimSpace(strings.Join([]string{userText, assistantText}, "\n"))
	effectiveInputObservation := mapFromAny(req.ClientMeta["effective_input_observation"])
	verifiedEffectiveInput := ""
	if extractionStringFromAny(effectiveInputObservation["contract_version"]) == "effective_input_observation.v1" &&
		extractionStringFromAny(effectiveInputObservation["status"]) == "verified" &&
		extractionStringFromAny(effectiveInputObservation["capture_stage"]) == "before_request_return" &&
		extractionStringFromAny(effectiveInputObservation["hash_algorithm"]) == "or1c_utf16_djb2.v1" &&
		completeTurnBoolFromAny(effectiveInputObservation["payload_content_match"]) {
		candidate := strings.TrimSpace(extractionStringFromAny(effectiveInputObservation["effective_input"]))
		if candidate != "" && prepareOR1CHash(candidate) == extractionStringFromAny(effectiveInputObservation["effective_input_hash"]) {
			verifiedEffectiveInput = candidate
		}
	}
	extractionCfg := s.completeTurnExtractionConfig(req.ClientMeta)
	languageContext := completeTurnLanguageContextFromClientMeta(req.ClientMeta)
	llmConfigTrace := completeTurnLLMConfigTrace(extractionCfg)
	requestedTurnIndex := req.TurnIndex
	if requestedTurnIndex <= 0 {
		requestedTurnIndex = 1
	}
	preserveRequestedTurnIndex := completeTurnPreserveRequestedTurnIndex(req.ClientMeta)
	rawTurnAlreadyPersisted := false
	rawUserAlreadyPersisted := false
	rawAssistantAlreadyPersisted := false
	requestedTurnHasAnyRaw := false
	rawTurnContentConflict := false
	if sourceAcceptance.Enabled && sourceAcceptance.Accepted && sourceAcceptance.ReplaceExisting {
		now := time.Now().UTC()
		if !s.completeTurnSourceAcceptanceStillCurrent(sourceAcceptance, sid, req.TurnIndex) {
			if s.TurnWorkflows != nil && workflowRequestID != "" {
				s.TurnWorkflows.invalidate(workflowRequestID, "source_acceptance_revision_superseded_before_replacement")
			}
			writeCompleteTurnSourceAcceptanceRejection(w, req, rejectedCompleteTurnSourceAcceptance("source_acceptance_revision_superseded_before_replacement", false, sourceAcceptance.Observation))
			return
		}
		if replacementErr := s.replaceCompleteTurnLogicalTail(ctx, sid, req.TurnIndex, userText, assistantText, sourceAcceptance, now); replacementErr != nil {
			queueAction := "retry"
			status := "error"
			saveOK := false
			reconciliationRequired := replacementErr.CommitState == "unknown"
			if !replacementErr.Retryable {
				queueAction = "discard"
			}
			if replacementErr.RawCommitted {
				status = "partial"
				saveOK = true
				queueAction = "discard"
				reconciliationRequired = true
				s.completeTurnSourceReplacementCompleted(ctx, sourceAcceptance, sid, req.TurnIndex)
			}
			if s.TurnWorkflows != nil && workflowRequestID != "" {
				if replacementErr.RawCommitted {
					s.TurnWorkflows.finishStage(workflowRequestID, turnWorkflowStageFinalAccepted, "succeeded", "")
					s.TurnWorkflows.finishStage(workflowRequestID, turnWorkflowStageRawPersist, "succeeded", "")
					s.TurnWorkflows.addWarning(workflowRequestID, replacementErr.Code, "turn_hud.error.logical_turn_replace_failed", turnWorkflowStageRawPersist)
					s.TurnWorkflows.complete(workflowRequestID)
				} else {
					s.TurnWorkflows.fail(
						workflowRequestID,
						replacementErr.Code,
						"turn_hud.error.logical_turn_replace_failed",
						turnWorkflowStageFinalAccepted,
						replacementErr.Retryable,
					)
				}
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status":                  status,
				"code":                    replacementErr.Code,
				"stage":                   replacementErr.Stage,
				"retryable":               replacementErr.Retryable && !replacementErr.RawCommitted,
				"raw_committed":           replacementErr.RawCommitted,
				"commit_state":            replacementErr.CommitState,
				"save_ok":                 saveOK,
				"reconciliation_required": reconciliationRequired,
				"derived_retry_required":  false,
				"queue_action":            queueAction,
				"turn_index":              req.TurnIndex,
				"fail_reasons":            []string{replacementErr.Code},
				"turn_workflow_hud":       s.turnWorkflowHUDSnapshot(workflowRequestID),
			})
			return
		}
		// ReplaceLogicalTurn committed both canonical raw roles atomically. Shadow
		// mode reads from its no-op primary, so a follow-up ListChatLogs cannot be
		// used to rediscover those rows and must not trigger duplicate raw writes.
		rawTurnAlreadyPersisted = true
		rawUserAlreadyPersisted = true
		rawAssistantAlreadyPersisted = true
		requestedTurnHasAnyRaw = true
		s.completeTurnSourceReplacementCompleted(ctx, sourceAcceptance, sid, req.TurnIndex)
	}
	if s.usesShadowWriteStore() && req.TurnIndex > 0 && !rawTurnAlreadyPersisted {
		if existingLogs, err := s.Store.ListChatLogs(ctx, sid, req.TurnIndex, req.TurnIndex); err == nil {
			rawUserAlreadyPersisted, rawAssistantAlreadyPersisted = completeTurnRawRolePresence(existingLogs, sid, req.TurnIndex)
			requestedTurnHasAnyRaw = rawUserAlreadyPersisted || rawAssistantAlreadyPersisted
			rawUserContentMatches, rawAssistantContentMatches := completeTurnRawRoleContentMatches(existingLogs, sid, req.TurnIndex, userText, assistantText)
			rawExactPairAlreadyPersisted := completeTurnAlreadyPersistedWithContent(existingLogs, sid, req.TurnIndex, userText, assistantText)
			rawTurnAlreadyPersisted = rawExactPairAlreadyPersisted || (rawUserAlreadyPersisted && rawAssistantAlreadyPersisted)
			rawTurnContentConflict = (strings.TrimSpace(userText) != "" && rawUserAlreadyPersisted && !rawUserContentMatches) ||
				(strings.TrimSpace(assistantText) != "" && rawAssistantAlreadyPersisted && !rawAssistantContentMatches) ||
				(rawUserAlreadyPersisted && rawAssistantAlreadyPersisted && !rawExactPairAlreadyPersisted)
			if rawTurnContentConflict {
				now := time.Now().UTC()
				duplicateHUD := s.completeTurnWorkflowHUDDuplicate(
					workflowRequestID,
					sid,
					req.TurnIndex,
					"duplicate_turn_conflict",
					"DUPLICATE_TURN_CONFLICT",
					"turn_hud.warning.duplicate_turn_conflict",
					"turn_hud.notice.duplicate_conflict_preserved",
				)
				writeJSON(w, http.StatusOK, map[string]any{
					"status":                  "partial",
					"source":                  s.storeWriteSource(),
					"chat_session_id":         sid,
					"turn_index":              req.TurnIndex,
					"generated_at":            now.Format(time.RFC3339),
					"save_ok":                 true,
					"save_error":              "",
					"chat_logs_saved":         0,
					"memories_saved":          0,
					"evidence_saved":          0,
					"kg_triples_saved":        0,
					"vectors_upserted":        0,
					"derived_artifacts_saved": 0,
					"critic_triggered":        false,
					"critic_result":           nil,
					"llm_config_trace":        llmConfigTrace,
					"episode_result":          map[string]any{"checked": false, "triggered": false, "range": nil, "reason": "raw_turn_content_conflict"},
					"chapter_result":          map[string]any{"checked": false, "triggered": false, "range": nil, "reason": "raw_turn_content_conflict"},
					"maintenance_enqueued":    false,
					"fail_reasons":            []string{"raw_turn_content_conflict"},
					"trace_handoff": map[string]any{
						"skeleton":           false,
						"turn_index":         req.TurnIndex,
						"save_ok":            true,
						"critic_triggered":   false,
						"store_mode":         string(s.Cfg.StoreMode),
						"store_write_source": s.storeWriteSource(),
						"existing_chat_logs": len(existingLogs),
						"duplicate_guard":    "same_turn_role_pair_exists_with_different_content",
						"note":               "complete-turn refused duplicate raw/derived writes for an existing turn with conflicting raw text",
					},
					"warnings":          []string{"complete_turn_raw_content_conflict: existing user+assistant logs for this turn differ; duplicate writes skipped"},
					"turn_workflow_hud": duplicateHUD,
					"note":              "complete-turn duplicate guard kept existing raw turn; use explicit rollback/delete+rebuild to replace it",
				})
				return
			}
			if rawExactPairAlreadyPersisted && completeTurnHasDerivedArtifacts(ctx, s.Store, sid, req.TurnIndex) {
				now := time.Now().UTC()
				duplicateHUD := s.completeTurnWorkflowHUDDuplicate(
					workflowRequestID,
					sid,
					req.TurnIndex,
					"duplicate_turn_replay",
					"DUPLICATE_TURN_REPLAY",
					"turn_hud.warning.duplicate_turn_replay",
					"turn_hud.notice.duplicate_existing_preserved",
				)
				writeJSON(w, http.StatusOK, map[string]any{
					"status":                  "ok",
					"source":                  s.storeWriteSource(),
					"chat_session_id":         sid,
					"turn_index":              req.TurnIndex,
					"generated_at":            now.Format(time.RFC3339),
					"save_ok":                 true,
					"save_error":              "",
					"chat_logs_saved":         0,
					"memories_saved":          0,
					"evidence_saved":          0,
					"kg_triples_saved":        0,
					"vectors_upserted":        0,
					"derived_artifacts_saved": 0,
					"critic_triggered":        false,
					"critic_result":           nil,
					"llm_config_trace":        llmConfigTrace,
					"episode_result":          map[string]any{"checked": false, "triggered": false, "range": nil, "reason": "idempotent_replay"},
					"chapter_result":          map[string]any{"checked": false, "triggered": false, "range": nil, "reason": "idempotent_replay"},
					"maintenance_enqueued":    false,
					"fail_reasons":            []string{},
					"trace_handoff": map[string]any{
						"skeleton":             false,
						"turn_index":           req.TurnIndex,
						"save_ok":              true,
						"critic_triggered":     false,
						"idempotent_replay":    true,
						"llm_config_trace":     llmConfigTrace,
						"store_mode":           string(s.Cfg.StoreMode),
						"store_write_source":   s.storeWriteSource(),
						"existing_chat_logs":   len(existingLogs),
						"derived_write_policy": "skip_when_raw_and_derived_artifacts_exist",
						"note":                 "complete-turn retry detected existing raw and derived turn artifacts and skipped duplicate writes",
					},
					"warnings":          []string{"complete_turn_idempotent_replay: existing raw and derived turn artifacts found; duplicate writes skipped"},
					"turn_workflow_hud": duplicateHUD,
					"note":              "complete-turn idempotent replay; existing turn artifacts kept",
				})
				return
			}
		}
	}
	if s.usesShadowWriteStore() && strings.TrimSpace(userText) != "" && strings.TrimSpace(assistantText) != "" {
		if existingLogs, err := s.Store.ListChatLogs(ctx, sid, 0, 0); err == nil {
			if existingTurn, ok := completeTurnFindPersistedTurnWithContent(existingLogs, sid, userText, assistantText); ok && existingTurn > 0 && existingTurn != req.TurnIndex {
				if completeTurnHasDerivedArtifacts(ctx, s.Store, sid, existingTurn) {
					now := time.Now().UTC()
					duplicateHUD := s.completeTurnWorkflowHUDDuplicate(
						workflowRequestID,
						sid,
						req.TurnIndex,
						"duplicate_pair_replay",
						"DUPLICATE_PAIR_REPLAY",
						"turn_hud.warning.duplicate_pair_replay",
						"turn_hud.notice.duplicate_existing_preserved",
					)
					writeJSON(w, http.StatusOK, map[string]any{
						"status":                  "ok",
						"source":                  s.storeWriteSource(),
						"chat_session_id":         sid,
						"turn_index":              existingTurn,
						"generated_at":            now.Format(time.RFC3339),
						"save_ok":                 true,
						"save_error":              "",
						"chat_logs_saved":         0,
						"memories_saved":          0,
						"evidence_saved":          0,
						"kg_triples_saved":        0,
						"vectors_upserted":        0,
						"derived_artifacts_saved": 0,
						"critic_triggered":        false,
						"critic_result":           nil,
						"llm_config_trace":        llmConfigTrace,
						"episode_result":          map[string]any{"checked": false, "triggered": false, "range": nil, "reason": "idempotent_pair_replay"},
						"chapter_result":          map[string]any{"checked": false, "triggered": false, "range": nil, "reason": "idempotent_pair_replay"},
						"maintenance_enqueued":    false,
						"fail_reasons":            []string{},
						"trace_handoff": map[string]any{
							"skeleton":               false,
							"turn_index":             existingTurn,
							"save_ok":                true,
							"critic_triggered":       false,
							"idempotent_replay":      true,
							"idempotent_pair_replay": true,
							"requested_turn_index":   requestedTurnIndex,
							"llm_config_trace":       llmConfigTrace,
							"store_mode":             string(s.Cfg.StoreMode),
							"store_write_source":     s.storeWriteSource(),
							"existing_chat_logs":     len(existingLogs),
							"duplicate_guard":        "same_session_exact_pair_exists_on_another_turn",
							"note":                   "complete-turn detected the same raw user+assistant pair on another turn and skipped duplicate writes",
						},
						"warnings":          []string{"complete_turn_idempotent_pair_replay: same raw user+assistant pair already exists on turn " + strconv.Itoa(existingTurn) + "; duplicate writes skipped"},
						"turn_workflow_hud": duplicateHUD,
						"note":              "complete-turn idempotent pair replay; existing turn artifacts kept",
					})
					return
				}
				req.TurnIndex = existingTurn
				requestedTurnIndex = existingTurn
				rawUserAlreadyPersisted = true
				rawAssistantAlreadyPersisted = true
				rawTurnAlreadyPersisted = true
				requestedTurnHasAnyRaw = true
			}
		}
	}
	turnIndex := requestedTurnIndex
	if s.usesShadowWriteStore() && !rawTurnAlreadyPersisted && !preserveRequestedTurnIndex && !requestedTurnHasAnyRaw {
		turnIndex = canonicalCompleteTurnIndex(ctx, s.Store, sid, requestedTurnIndex)
	}
	if turnIndex != req.TurnIndex {
		rawUserAlreadyPersisted = false
		rawAssistantAlreadyPersisted = false
		rawTurnAlreadyPersisted = false
		requestedTurnHasAnyRaw = false
	}
	sourceAcceptance = s.rebindCompleteTurnSourceAcceptance(ctx, sourceAcceptance, sid, sourceAcceptance.BoundTurn, turnIndex)
	if sourceAcceptance.Enabled && !sourceAcceptance.Accepted {
		if s.TurnWorkflows != nil && workflowRequestID != "" {
			s.TurnWorkflows.invalidate(workflowRequestID, sourceAcceptance.Reason)
		}
		writeCompleteTurnSourceAcceptanceRejection(w, req, sourceAcceptance)
		return
	}
	if shouldApplyCompleteTurnOOCGuard(req.ClientMeta) {
		if s.TurnWorkflows != nil && workflowRequestID != "" {
			s.TurnWorkflows.setLogicalTurn(workflowRequestID, turnIndex)
			s.TurnWorkflows.finishStage(workflowRequestID, turnWorkflowStageFinalAccepted, "succeeded", "")
			s.TurnWorkflows.finishStage(workflowRequestID, turnWorkflowStageRawPersist, "skipped", "ooc_turn_guard")
			s.TurnWorkflows.finishStage(workflowRequestID, turnWorkflowStageCriticLLM, "skipped", "ooc_turn_guard")
			s.TurnWorkflows.finishStage(workflowRequestID, turnWorkflowStageDerivedPersist, "skipped", "ooc_turn_guard")
			s.TurnWorkflows.finishStage(workflowRequestID, turnWorkflowStageCheckpoints, "skipped", "ooc_turn_guard")
			s.TurnWorkflows.addNotice(workflowRequestID, "OOC_TURN_SKIPPED", "turn_hud.notice.ooc_turn_skipped", turnWorkflowStageFinalAccepted)
			s.TurnWorkflows.setCounts(workflowRequestID, turnWorkflowHUDCountsFromComplete(
				false, false, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
			))
			s.TurnWorkflows.setPersistenceFacts(workflowRequestID, "skipped", 0, "skipped", 0, "not_requested", 0)
			s.TurnWorkflows.completeWithNotice(
				workflowRequestID,
				"turn_hud.notice.ooc_recognized",
				"turn_hud.notice.ooc_recognized_detail",
				"OOC_INPUT_CANCELLED",
			)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":               "ok",
			"source":               s.storeWriteSource(),
			"chat_session_id":      sid,
			"turn_index":           turnIndex,
			"generated_at":         time.Now().UTC().Format(time.RFC3339),
			"save_ok":              true,
			"save_error":           "skipped_by_ooc_guard",
			"critic_triggered":     false,
			"critic_result":        nil,
			"llm_config_trace":     llmConfigTrace,
			"episode_result":       map[string]any{"checked": false, "triggered": false, "range": nil, "reason": "ooc_turn_guard"},
			"chapter_result":       map[string]any{"checked": false, "triggered": false, "range": nil, "reason": "ooc_turn_guard"},
			"maintenance_enqueued": false,
			"fail_reasons":         []string{},
			"trace_handoff": map[string]any{
				"skeleton":               false,
				"turn_index":             turnIndex,
				"save_ok":                true,
				"critic_triggered":       false,
				"llm_config_trace":       llmConfigTrace,
				"ooc_turn_guard_applied": true,
				"store_mode":             string(s.Cfg.StoreMode),
				"note":                   "OOC turn skipped before chat log, critic, memory, evidence, KG, and vector writes",
			},
			"warnings":          []string{"ooc_turn_guard_applied"},
			"turn_workflow_hud": s.turnWorkflowHUDSnapshot(workflowRequestID),
			"note":              "complete-turn skipped by OOC guard",
		})
		return
	}
	if strings.TrimSpace(userText) == "" && !rawUserAlreadyPersisted {
		if s.TurnWorkflows != nil && workflowRequestID != "" {
			s.TurnWorkflows.fail(workflowRequestID, "USER_INPUT_MISSING", "turn_hud.error.user_input_missing", turnWorkflowStageFinalAccepted, false)
			s.TurnWorkflows.setCounts(workflowRequestID, turnWorkflowHUDCountsFromComplete(
				false, false, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
			))
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":                           "error",
			"source":                           s.storeWriteSource(),
			"chat_session_id":                  sid,
			"turn_index":                       turnIndex,
			"generated_at":                     time.Now().UTC().Format(time.RFC3339),
			"save_ok":                          false,
			"save_error":                       "user_input_missing",
			"chat_logs_saved":                  0,
			"memories_saved":                   0,
			"evidence_saved":                   0,
			"kg_triples_saved":                 0,
			"persona_capsule_candidates":       0,
			"subjective_entity_memories_saved": 0,
			"derived_artifacts_saved":          0,
			"critic_triggered":                 false,
			"critic_result":                    nil,
			"llm_config_trace":                 llmConfigTrace,
			"episode_result":                   map[string]any{"checked": false, "triggered": false, "range": nil, "reason": "user_input_missing"},
			"chapter_result":                   map[string]any{"checked": false, "triggered": false, "range": nil, "reason": "user_input_missing"},
			"maintenance_enqueued":             false,
			"fail_reasons":                     []string{"user_input_missing"},
			"trace_handoff": map[string]any{
				"skeleton":           false,
				"turn_index":         turnIndex,
				"save_ok":            false,
				"critic_triggered":   false,
				"llm_config_trace":   llmConfigTrace,
				"store_mode":         string(s.Cfg.StoreMode),
				"store_write_source": s.storeWriteSource(),
				"note":               "complete-turn refused to persist an assistant-only turn without a user input row",
			},
			"warnings":          []string{"user_input_missing: assistant-only complete-turn request skipped to prevent phantom turns"},
			"turn_workflow_hud": s.turnWorkflowHUDSnapshot(workflowRequestID),
			"note":              "complete-turn skipped because user_input is required for a persisted turn",
		})
		return
	}
	timing.addElapsed("preflight", preflightStartedAt)

	// Once a validated turn reaches the persistence boundary, an HTTP client or
	// reverse-proxy disconnect must not cancel canonical writes. Provider and
	// embedding calls still apply their configured child deadlines.
	ctx = context.WithoutCancel(ctx)
	ctx, releaseSourceAcceptanceWorker := s.completeTurnSourceAcceptanceProcessingContext(ctx, sourceAcceptance, sid, turnIndex)
	defer releaseSourceAcceptanceWorker()
	if lock, err := s.sessionMigrationSourceLock(context.WithoutCancel(ctx), sid); err != nil {
		writeInternalError(w, err.Error())
		return
	} else if lock != nil {
		s.writeCompleteTurnMigrationSourceLockBlocked(w, req, sid, workflowRequestID, lock)
		return
	}
	if !s.completeTurnSourceAcceptanceStillCurrent(sourceAcceptance, sid, turnIndex) {
		if s.TurnWorkflows != nil && workflowRequestID != "" {
			s.TurnWorkflows.invalidate(workflowRequestID, "source_acceptance_revision_superseded_before_persistence")
		}
		rejectedDecision := completeTurnSourceAcceptanceDecision{
			Enabled: true, Accepted: false, Status: "rejected", Reason: "source_acceptance_revision_superseded_before_persistence",
			QueueAction: "discard", Revision: sourceAcceptance.Revision, Previous: sourceAcceptance.Previous,
			Observation: sourceAcceptance.Observation,
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "rejected", "code": "source_acceptance_revision_superseded_before_persistence",
			"chat_session_id": sid, "turn_index": turnIndex, "save_ok": false,
			"chat_logs_saved": 0, "derived_artifacts_saved": 0, "vectors_upserted": 0,
			"critic_triggered": false, "derived_retry_required": false, "queue_action": "discard",
			"fail_reasons":            []string{"source_acceptance_revision_superseded_before_persistence"},
			"source_acceptance":       completeTurnSourceAcceptancePayload(rejectedDecision),
			"source_to_final_lineage": buildSourceToFinalLineage(req, rejectedDecision),
			"turn_workflow_hud":       s.turnWorkflowHUDSnapshot(workflowRequestID),
		})
		return
	}
	now := time.Now().UTC()
	if s.TurnWorkflows != nil && workflowRequestID != "" {
		s.TurnWorkflows.setLogicalTurn(workflowRequestID, turnIndex)
		s.TurnWorkflows.finishStage(workflowRequestID, turnWorkflowStageFinalAccepted, "succeeded", "")
		s.TurnWorkflows.startStage(workflowRequestID, turnWorkflowStageRawPersist)
	}
	rawStoreStartedAt := time.Now()
	rawSave := s.persistCompleteTurnRaw(ctx, sid, turnIndex, userText, assistantText, now, rawTurnAlreadyPersisted, rawUserAlreadyPersisted, rawAssistantAlreadyPersisted)
	timing.addElapsed("raw_and_audit_store", rawStoreStartedAt)
	rawTurnDurable := rawSave.UserDurable && rawSave.AssistantDurable
	if rawTurnDurable {
		if err := s.registerCompleteTurnSourceRevision(ctx, sourceAcceptance, sid, turnIndex, userText, assistantText, now); err != nil {
			rawSave.Errors++
			rawSave.ErrorDetails = append(rawSave.ErrorDetails, "RegisterAcceptedSourceRevision: "+err.Error())
			rawTurnDurable = false
		}
	}
	if rawTurnDurable {
		s.wakeMemoryWorkers()
	}
	if s.TurnWorkflows != nil && workflowRequestID != "" {
		if !s.usesShadowWriteStore() {
			s.TurnWorkflows.finishStage(workflowRequestID, turnWorkflowStageRawPersist, "skipped", "store_writes_disabled")
			s.TurnWorkflows.addWarning(workflowRequestID, "STORE_WRITES_DISABLED", "turn_hud.warning.store_writes_disabled", turnWorkflowStageRawPersist)
		} else if rawTurnDurable {
			s.TurnWorkflows.finishStage(workflowRequestID, turnWorkflowStageRawPersist, "succeeded", "")
		} else {
			s.TurnWorkflows.fail(workflowRequestID, "RAW_TURN_PERSIST_FAILED", "turn_hud.error.raw_turn_persist_failed", turnWorkflowStageRawPersist, true)
		}
	}

	var criticResult map[string]any
	criticTrace := map[string]any{}
	criticTriggered := false
	criticFailureReason := ""
	reprocessingReason := ""
	reprocessingDurable := false
	var criticFailureTrace map[string]any
	criticFailure := map[string]any{}
	failReasons := []string{}
	criticWorkflowStageHandled := false
	if s.usesShadowWriteStore() && content != "" {
		if !rawTurnDurable {
			failReasons = append(failReasons, "critic_skipped: raw_chat_logs_not_durable")
			criticWorkflowStageHandled = true
			if s.TurnWorkflows != nil && workflowRequestID != "" {
				s.TurnWorkflows.finishStage(workflowRequestID, turnWorkflowStageCriticLLM, "skipped", "raw_chat_logs_not_durable")
			}
		} else if extractionCfg.Critic.hasConfig() {
			if assistantText == "" {
				failReasons = append(failReasons, "critic_skipped: assistant_content_missing")
				criticWorkflowStageHandled = true
				if s.TurnWorkflows != nil && workflowRequestID != "" {
					s.TurnWorkflows.finishStage(workflowRequestID, turnWorkflowStageCriticLLM, "skipped", "assistant_content_missing")
				}
			} else {
				criticWorkflowStageHandled = true
				if s.TurnWorkflows != nil && workflowRequestID != "" {
					s.TurnWorkflows.startStage(workflowRequestID, turnWorkflowStageCriticLLM)
				}
				criticStartedAt := time.Now()
				result, trace, err := s.runCompleteTurnCriticWithInputPolicy(ctx, sid, turnIndex, userText, assistantText, req.ContextMessages, req.OutputLanguageOverride, extractionCfg.Critic, true, s.completeTurnCriticInputPolicy(req.ClientMeta), completeTurnCriticInputReplay{SourceRevision: sourceAcceptance.Revision}, languageContext)
				timing.addElapsed("critic_llm", criticStartedAt)
				if err != nil {
					criticFailure = criticPipelineErrorDetails(err)
					criticCode := strings.TrimSpace(stringFromMap(criticFailure, "code"))
					if criticCode == "" {
						criticCode = "CRITIC_UNKNOWN_FAILED"
					}
					criticFailureReason = strings.TrimSpace(err.Error())
					if !strings.HasPrefix(criticFailureReason, criticCode) {
						criticFailureReason = criticCode + ": " + criticFailureReason
					}
					reprocessingReason = criticCode
					failReasons = append(failReasons, criticFailureReason)
					if trace != nil {
						criticTrace = trace
						criticFailureTrace = trace
					} else {
						criticTrace = criticFailure
						criticFailureTrace = criticFailure
					}
				} else {
					criticTriggered = true
					if s.TurnWorkflows != nil && workflowRequestID != "" {
						s.TurnWorkflows.finishStage(workflowRequestID, turnWorkflowStageCriticLLM, "succeeded", "")
					}
					var personaRoleTrace map[string]any
					criticResult, personaRoleTrace = applyRisuPersonaSubjectiveMemoryRoles(result, req.ClientMeta)
					criticTrace = trace
					criticTrace["risu_persona_role_resolution"] = personaRoleTrace
				}
			}
		} else {
			failReasons = append(failReasons, "critic_config_missing")
			reprocessingReason = "critic_config_missing"
			criticWorkflowStageHandled = true
			if s.TurnWorkflows != nil && workflowRequestID != "" {
				s.TurnWorkflows.finishStage(workflowRequestID, turnWorkflowStageCriticLLM, "skipped", "critic_config_missing")
				s.TurnWorkflows.addWarning(workflowRequestID, "CRITIC_LLM_NOT_CONFIGURED", "turn_hud.warning.critic_llm_not_configured", turnWorkflowStageCriticLLM)
			}
		}
	}
	if !criticWorkflowStageHandled && s.TurnWorkflows != nil && workflowRequestID != "" {
		s.TurnWorkflows.finishStage(workflowRequestID, turnWorkflowStageCriticLLM, "skipped", "critic_not_applicable")
	}
	if s.TurnWorkflows != nil && workflowRequestID != "" {
		s.TurnWorkflows.startStage(workflowRequestID, turnWorkflowStageDerivedPersist)
	}
	if !s.completeTurnSourceAcceptanceStillCurrent(sourceAcceptance, sid, turnIndex) {
		if s.TurnWorkflows != nil && workflowRequestID != "" {
			s.TurnWorkflows.invalidate(workflowRequestID, "source_acceptance_revision_superseded_during_processing")
		}
		rejectedDecision := completeTurnSourceAcceptanceDecision{
			Enabled: true, Accepted: false, Status: "rejected", Reason: "source_acceptance_revision_superseded_during_processing",
			QueueAction: "discard", Revision: sourceAcceptance.Revision, Previous: sourceAcceptance.Previous,
			Observation: sourceAcceptance.Observation,
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "rejected", "code": "source_acceptance_revision_superseded_during_processing",
			"chat_session_id": sid, "turn_index": turnIndex, "save_ok": rawTurnDurable,
			"chat_logs_saved": rawSave.ChatLogsSaved, "derived_artifacts_saved": 0, "vectors_upserted": 0,
			"critic_triggered": criticTriggered, "derived_retry_required": false, "queue_action": "discard",
			"fail_reasons":            []string{"source_acceptance_revision_superseded_during_processing"},
			"source_acceptance":       completeTurnSourceAcceptancePayload(rejectedDecision),
			"source_to_final_lineage": buildSourceToFinalLineage(req, rejectedDecision),
			"turn_workflow_hud":       s.turnWorkflowHUDSnapshot(workflowRequestID),
		})
		return
	}

	// save_ok is the canonical raw-turn durability boundary. Derived/audit
	// failures are reported separately and must never cause a full raw replay.
	saveOK := rawTurnDurable
	saveErr := "shadow_mode: save disabled in R0/R1"
	if rawTurnDurable {
		saveErr = ""
	} else if s.usesShadowWriteStore() {
		saveErr = "raw turn persistence failed"
	}
	chatLogsSaved := rawSave.ChatLogsSaved
	effectiveInputSaved := 0
	auditSaved := 0
	criticFeedbackSaved := 0
	memoriesSaved := 0
	preciseMemoryUnitsSaved := 0
	evidenceSaved := 0
	kgTriplesSaved := 0
	personaCapsuleCandidates := 0
	subjectiveEntityMemoriesSaved := 0
	characterEventsSaved := 0
	storylinesSaved := 0
	worldRulesSaved := 0
	characterStatesSaved := 0
	physicalConditionsSaved := 0
	entityConditionsSaved := 0
	statusSchemaDefinitionsSaved := 0
	statusEffectsSaved := 0
	narrativeCurrentStatesSaved := 0
	narrativeStateEventsSaved := 0
	relationshipCurrentStatesSaved := 0
	relationshipStateEventsSaved := 0
	habitEvidenceCurrentSaved := 0
	habitEvidenceEventsSaved := 0
	characterProfilesSaved := 0
	voiceBehaviorProjectionsSaved := 0
	pendingThreadsSaved := 0
	activeStatesSaved := 0
	canonicalStateLayersSaved := 0
	entitiesSaved := 0
	entityIdentitiesSaved := 0
	identitySurfacesSaved := 0
	identityLinksSaved := 0
	identityBindingsSaved := 0
	speakerAttributionsSaved := 0
	trustStatesSaved := 0
	vectorsUpserted := 0
	vectorsMemoryUpserted := 0
	vectorsEvidenceUpserted := 0
	vectorsWorldRuleUpserted := 0
	derivedWriteAttempted := 0
	derivedWriteErrors := 0
	derivedWriteErrorDetails := []string{}
	derivedArtifactsSaved := 0
	derivedCommitted := 0
	derivedDiagnostics := []completeTurnPersistenceDiagnostic{}
	derivedRollbackState := "not_applicable"
	storeWriteAttempted := rawSave.Attempted
	storeWriteErrors := rawSave.Errors
	storeWriteErrorDetails := append([]string(nil), rawSave.ErrorDetails...)
	artifactWarnings := append([]string(nil), rawSave.Warnings...)
	conflictResolutions := []map[string]any{}
	retentionDecisions := []map[string]any{}
	var canonicalStateWriteCost any
	embeddingStatus := "not_requested"
	vectorStatus := "not_requested"

	auditStoreStartedAt := time.Now()
	writeSource := s.storeWriteSource()
	if s.usesShadowWriteStore() {
		if verifiedEffectiveInput != "" {
			storeWriteAttempted++
			if err := s.Store.SaveEffectiveInput(ctx, &store.EffectiveInput{
				ChatSessionID:  sid,
				TurnIndex:      turnIndex,
				EffectiveInput: verifiedEffectiveInput,
				CreatedAt:      now,
			}); err != nil {
				storeWriteErrors++
				storeWriteErrorDetails = append(storeWriteErrorDetails, "SaveEffectiveInput: "+err.Error())
			} else {
				effectiveInputSaved++
			}
		}

		if hasStructuredFeedback(req.ContextMessages) || hasImprovementTrace(req.ImprovementTrace) {
			storeWriteAttempted++
			if err := s.Store.SaveCriticFeedback(ctx, &store.CriticFeedback{
				ChatSessionID: sid,
				TargetType:    "turn",
				TargetID:      int64(turnIndex),
				FeedbackValue: "structured_feedback",
				FeedbackNote:  fmt.Sprintf(`{"turn_index":%d,"context_count":%d,"has_improvement_trace":%t}`, turnIndex, len(req.ContextMessages), req.ImprovementTrace != nil),
				Source:        writeSource,
				CreatedAt:     now,
			}); err != nil {
				storeWriteErrors++
				storeWriteErrorDetails = append(storeWriteErrorDetails, "SaveCriticFeedback: "+err.Error())
			} else {
				criticFeedbackSaved++
			}
		}

		if effectiveInputSaved > 0 {
			storeWriteAttempted++
			if err := s.Store.SaveAuditLog(ctx, &store.AuditLog{
				ChatSessionID: sid,
				EventType:     "effective_input_saved",
				TargetType:    "turn",
				TargetID:      int64(turnIndex),
				Summary:       fmt.Sprintf("effective input saved turn %d", turnIndex),
				DetailsJSON:   fmt.Sprintf(`{"turn_index":%d,"length":%d}`, turnIndex, len(verifiedEffectiveInput)),
				Source:        writeSource,
				CreatedAt:     now,
			}); err != nil {
				storeWriteErrors++
				storeWriteErrorDetails = append(storeWriteErrorDetails, "SaveAuditLog: "+err.Error())
			} else {
				auditSaved++
			}
		}

		if criticFailureReason != "" {
			storeWriteAttempted++
			if err := s.Store.SaveAuditLog(ctx, &store.AuditLog{
				ChatSessionID: sid,
				EventType:     "critic_extract_failed",
				TargetType:    "turn",
				TargetID:      int64(turnIndex),
				Summary:       fmt.Sprintf("critic extraction failed turn %d", turnIndex),
				DetailsJSON: mustCompactJSON(map[string]any{
					"turn_index":       turnIndex,
					"reason":           criticFailureReason,
					"failure":          criticFailure,
					"trace":            criticFailureTrace,
					"llm_config_trace": llmConfigTrace,
				}),
				Source:    writeSource,
				CreatedAt: now,
			}); err != nil {
				storeWriteErrors++
				storeWriteErrorDetails = append(storeWriteErrorDetails, "SaveAuditLog(critic_extract_failed): "+err.Error())
			} else {
				auditSaved++
			}
		}
		var existingEvidence []store.DirectEvidence
		if s.Store != nil {
			existingEvidence, _ = s.Store.ListEvidence(ctx, sid)
		}
		timing.addElapsed("raw_and_audit_store", auditStoreStartedAt)
		if criticResult != nil {
			artifactStartedAt := time.Now()
			artifactContext := contextWithEntityIdentitySource(ctx, sourceAcceptance)
			artifactResult := s.saveCriticExtractionArtifacts(artifactContext, sid, turnIndex, criticResult, content, extractionCfg.Embedder, now, existingEvidence)
			artifactTotalMS := durationMilliseconds(time.Since(artifactStartedAt))
			embeddingMS := artifactResult.TimingMS["embedding"]
			vectorUpsertMS := artifactResult.TimingMS["vector_upsert"]
			derivedStoreMS := artifactTotalMS - embeddingMS - vectorUpsertMS
			if derivedStoreMS < 0 {
				derivedStoreMS = 0
			}
			timing.addMilliseconds("derived_store", derivedStoreMS)
			timing.addMilliseconds("embedding", embeddingMS)
			timing.addMilliseconds("vector_upsert", vectorUpsertMS)
			memoriesSaved += artifactResult.Memories
			preciseMemoryUnitsSaved += artifactResult.PreciseMemoryUnits
			evidenceSaved += artifactResult.Evidence
			kgTriplesSaved += artifactResult.KGTriples
			personaCapsuleCandidates += artifactResult.PersonaCapsuleCandidates
			subjectiveEntityMemoriesSaved += artifactResult.SubjectiveEntityMemories
			characterEventsSaved += artifactResult.CharacterEvents
			storylinesSaved += artifactResult.Storylines
			worldRulesSaved += artifactResult.WorldRules
			characterStatesSaved += artifactResult.CharacterStates
			physicalConditionsSaved += artifactResult.PhysicalConditions
			entityConditionsSaved += artifactResult.EntityConditions
			statusSchemaDefinitionsSaved += artifactResult.StatusSchemaDefinitions
			statusEffectsSaved += artifactResult.StatusEffects
			narrativeCurrentStatesSaved += artifactResult.NarrativeCurrentStates
			narrativeStateEventsSaved += artifactResult.NarrativeStateEvents
			relationshipCurrentStatesSaved += artifactResult.RelationCurrentStates
			relationshipStateEventsSaved += artifactResult.RelationStateEvents
			habitEvidenceCurrentSaved += artifactResult.HabitEvidenceCurrent
			habitEvidenceEventsSaved += artifactResult.HabitEvidenceEvents
			characterProfilesSaved += artifactResult.CharacterProfiles
			voiceBehaviorProjectionsSaved += artifactResult.VoiceBehaviorProjections
			pendingThreadsSaved += artifactResult.PendingThreads
			activeStatesSaved += artifactResult.ActiveStates
			canonicalStateLayersSaved += artifactResult.CanonicalStateLayers
			entitiesSaved += artifactResult.Entities
			entityIdentitiesSaved += artifactResult.EntityIdentities
			identitySurfacesSaved += artifactResult.IdentitySurfaces
			identityLinksSaved += artifactResult.EntityIdentityLinks
			identityBindingsSaved += artifactResult.IdentityBindings
			speakerAttributionsSaved += artifactResult.SpeakerAttributions
			trustStatesSaved += artifactResult.TrustStates
			vectorsUpserted += artifactResult.VectorsUpserted
			vectorsMemoryUpserted += artifactResult.VectorsMemoryUpserted
			vectorsEvidenceUpserted += artifactResult.VectorsEvidenceUpserted
			vectorsWorldRuleUpserted += artifactResult.VectorsWorldRuleUpserted
			derivedWriteAttempted += artifactResult.Attempted
			derivedWriteErrors += artifactResult.Errors
			derivedWriteErrorDetails = append(derivedWriteErrorDetails, artifactResult.ErrorDetails...)
			storeWriteAttempted += artifactResult.Attempted
			storeWriteErrors += artifactResult.Errors
			storeWriteErrorDetails = append(storeWriteErrorDetails, artifactResult.ErrorDetails...)
			artifactWarnings = append(artifactWarnings, artifactResult.Warnings...)
			conflictResolutions = append(conflictResolutions, artifactResult.ConflictResolutions...)
			retentionDecisions = append(retentionDecisions, artifactResult.RetentionDecisions...)
			if artifactResult.CanonicalStateWriteCost != nil {
				canonicalStateWriteCost = artifactResult.CanonicalStateWriteCost
			}
			embeddingStatus = artifactResult.EmbeddingStatus
			vectorStatus = artifactResult.VectorStatus
		}
		derivedArtifactsSaved = memoriesSaved + preciseMemoryUnitsSaved + evidenceSaved + kgTriplesSaved + subjectiveEntityMemoriesSaved + characterEventsSaved + storylinesSaved + worldRulesSaved + characterStatesSaved + physicalConditionsSaved + entityConditionsSaved + statusSchemaDefinitionsSaved + statusEffectsSaved + narrativeCurrentStatesSaved + narrativeStateEventsSaved + relationshipCurrentStatesSaved + relationshipStateEventsSaved + habitEvidenceCurrentSaved + habitEvidenceEventsSaved + characterProfilesSaved + voiceBehaviorProjectionsSaved + pendingThreadsSaved + activeStatesSaved + canonicalStateLayersSaved + entitiesSaved + entityIdentitiesSaved + identitySurfacesSaved + identityLinksSaved + identityBindingsSaved + speakerAttributionsSaved + trustStatesSaved
		derivedDiagnostics = s.completeTurnPersistenceDiagnostics(derivedWriteErrorDetails)
		derivedCommitted = derivedArtifactsSaved
		derivedRollbackState = completeTurnPersistenceRollbackState(derivedDiagnostics, derivedCommitted, derivedWriteErrors)
		if derivedRollbackState == "atomic_rollback" {
			derivedCommitted = 0
		}
		if reprocessingReason == "" && derivedWriteErrors > 0 {
			reprocessingReason = "derived_persist_failed"
		}
		if reprocessingReason != "" && rawTurnDurable &&
			sourceAcceptance.Enabled && sourceAcceptance.Accepted &&
			strings.TrimSpace(sourceAcceptance.Revision) != "" {
			_, supported := s.Store.(store.MemoryReprocessingJobStore)
			if availability, ok := s.Store.(store.MemoryDerivationLifecycleAvailability); ok &&
				!availability.MemoryDerivationLifecycleEnabled() {
				supported = false
			}
			if supported {
				storeWriteAttempted++
				reprocessingJobReason := reprocessingReason
				if reprocessingReason == "derived_persist_failed" {
					reprocessingJobReason = completeTurnPersistenceFailureSummary(derivedDiagnostics)
				}
				if _, err := s.enqueueCompleteTurnReprocessingJob(
					ctx, sourceAcceptance, sid, reprocessingJobReason, now,
				); err != nil {
					storeWriteErrors++
					storeWriteErrorDetails = append(
						storeWriteErrorDetails,
						"EnqueueMemoryReprocessingJob: "+err.Error(),
					)
				} else {
					reprocessingDurable = true
				}
			}
		}
		if s.TurnWorkflows != nil && workflowRequestID != "" {
			switch {
			case criticResult == nil:
				s.TurnWorkflows.finishStage(workflowRequestID, turnWorkflowStageDerivedPersist, "skipped", "critic_result_unavailable")
				criticCode := strings.TrimSpace(stringFromMap(criticFailure, "code"))
				if criticCode != "" {
					details := []turnWorkflowHUDDetail{}
					for _, item := range []struct {
						key   string
						value string
					}{
						{key: "pipeline_stage", value: stringFromMap(criticFailure, "stage")},
						{key: "provider", value: stringFromMap(criticFailureTrace, "provider")},
						{key: "model", value: stringFromMap(criticFailureTrace, "model")},
						{key: "http_status", value: extractionStringFromAny(criticFailure["http_status"])},
						{key: "cause", value: scrubCriticFailureText(criticFailureReason, extractionCfg.Critic.APIKey)},
						{key: "raw_preview", value: stringFromMap(criticFailureTrace, "raw_preview")},
					} {
						if value := strings.TrimSpace(item.value); value != "" {
							details = append(details, turnWorkflowHUDDetail{Key: item.key, Value: truncateRunes(value, 1000)})
						}
					}
					if reprocessingDurable {
						details = append(details, turnWorkflowHUDDetail{Key: "reprocessing", Value: "queued"})
					} else {
						details = append(details, turnWorkflowHUDDetail{Key: "reprocessing", Value: "unavailable"})
					}
					s.TurnWorkflows.failWithDetails(
						workflowRequestID,
						criticCode,
						"turn_hud.error.critic_llm_failed",
						turnWorkflowStageCriticLLM,
						boolFromAny(criticFailure["retryable"]),
						details,
					)
				}
			case derivedWriteErrors > 0:
				s.TurnWorkflows.failWithDetails(
					workflowRequestID,
					"DERIVED_PERSIST_FAILED",
					"turn_hud.error.derived_persist_failed",
					turnWorkflowStageDerivedPersist,
					true,
					completeTurnPersistenceHUDDetails(
						derivedDiagnostics,
						derivedWriteAttempted,
						derivedCommitted,
						derivedRollbackState,
						reprocessingDurable,
					),
				)
			case strings.HasPrefix(embeddingStatus, "error:"):
				s.TurnWorkflows.fail(workflowRequestID, "EMBEDDING_FAILED", "turn_hud.error.embedding_failed", turnWorkflowStageDerivedPersist, true)
			case strings.HasPrefix(vectorStatus, "error:"):
				s.TurnWorkflows.fail(workflowRequestID, "VECTOR_INDEX_FAILED", "turn_hud.error.vector_index_failed", turnWorkflowStageDerivedPersist, true)
			default:
				s.TurnWorkflows.finishStage(workflowRequestID, turnWorkflowStageDerivedPersist, "succeeded", "")
				if embeddingStatus == "missing_config" || vectorStatus == "missing_embedding_config" || vectorStatus == "vector_not_configured" {
					s.TurnWorkflows.addWarning(workflowRequestID, "VECTOR_INDEX_SKIPPED", "turn_hud.warning.vector_index_skipped", turnWorkflowStageDerivedPersist)
				}
			}
			s.TurnWorkflows.startStage(workflowRequestID, turnWorkflowStageCheckpoints)
		}

	}
	if !s.usesShadowWriteStore() {
		timing.addElapsed("raw_and_audit_store", auditStoreStartedAt)
		if s.TurnWorkflows != nil && workflowRequestID != "" {
			s.TurnWorkflows.finishStage(workflowRequestID, turnWorkflowStageDerivedPersist, "skipped", "store_writes_disabled")
			s.TurnWorkflows.addWarning(workflowRequestID, "STORE_WRITES_DISABLED", "turn_hud.warning.store_writes_disabled", turnWorkflowStageDerivedPersist)
			s.TurnWorkflows.startStage(workflowRequestID, turnWorkflowStageCheckpoints)
		}
	}

	note := "complete-turn is a shadow skeleton; no mutations performed"
	if s.usesShadowWriteStore() {
		if saveOK {
			note = "complete-turn saved in " + writeSource + " mode"
		} else {
			note = "complete-turn write attempted in " + writeSource + " mode but failed"
		}
	}

	writebackPlan := buildWritebackPlan(sid, turnIndex, s.usesShadowWriteStore(), writeSource, req)
	warnings := []string{"complete-turn did not write because store writes are disabled"}
	if s.usesShadowWriteStore() {
		warnings = []string{"complete-turn writes use critic extraction when LLM settings are present; no fake memory/evidence/KG placeholders are written"}
	}
	warnings = append(warnings, artifactWarnings...)
	if !extractionCfg.Critic.hasConfig() {
		warnings = append(warnings, "critic_config_missing: derived memory/evidence/KG/entities/trust/world extraction skipped")
	} else if !extractionCfg.Embedder.hasConfig() {
		warnings = append(warnings, "embedding_config_missing: memory text can be saved but vector upsert is skipped")
	}
	if len(failReasons) == 0 {
		failReasons = []string{}
	}

	maintenanceStartedAt := time.Now()
	maintenanceHandoff := s.buildCompleteTurnMaintenanceHandoff(ctx, sid, turnIndex, saveOK, now, writeSource, req)
	timing.addElapsed("maintenance_handoff", maintenanceStartedAt)
	auditSaved += maintenanceHandoff.AuditSaved
	storeWriteAttempted += maintenanceHandoff.Attempted
	storeWriteErrors += maintenanceHandoff.Errors
	storeWriteErrorDetails = append(storeWriteErrorDetails, maintenanceHandoff.ErrorDetails...)
	if maintenanceHandoff.Errors > 0 {
		failReasons = append(failReasons, "maintenance_audit")
	}
	if len(failReasons) == 0 {
		failReasons = []string{}
	}
	episodeResult := map[string]any{"checked": false, "triggered": false, "range": nil, "reason": "store_write_not_ok"}
	if saveOK {
		episodeStartedAt := time.Now()
		episodeResult = s.completeTurnEpisodeCheckpoint(ctx, sid, turnIndex, req.ClientMeta)
		timing.addElapsed("episode_checkpoint", episodeStartedAt)
		if errText := strings.TrimSpace(stringFromMap(episodeResult, "error")); errText != "" {
			warnings = append(warnings, "episode_checkpoint_failed: "+errText)
		}
	}
	hierarchyPromotionResult := map[string]any{"checked": false, "triggered": false, "policy": completeTurnHierarchyPromotionVersion, "reason": "store_write_not_ok"}
	if saveOK {
		hierarchyStartedAt := time.Now()
		hierarchyPromotionResult = s.completeTurnHierarchyPromotionCheckpoint(ctx, sid, turnIndex, req.ClientMeta)
		timing.addElapsed("hierarchy_checkpoint", hierarchyStartedAt)
		if errText := strings.TrimSpace(stringFromMap(hierarchyPromotionResult, "error")); errText != "" {
			warnings = append(warnings, "hierarchy_promotion_failed: "+errText)
		}
	}
	episodeSummariesSaved := intFromAny(episodeResult["generated"], 0)
	rawStatus := "skipped"
	if rawTurnDurable {
		rawStatus = "ok"
	} else if s.usesShadowWriteStore() {
		rawStatus = "error"
	}
	derivedPersistenceFailed := rawTurnDurable && derivedWriteErrors > 0
	derivedStatus := "skipped"
	if derivedPersistenceFailed {
		derivedStatus = "error"
	} else if derivedCommitted > 0 {
		derivedStatus = "ok"
	} else if criticTriggered && derivedCommitted == 0 {
		derivedStatus = "empty"
	} else if rawStatus == "ok" && !criticTriggered {
		derivedStatus = "delayed"
	} else if rawStatus == "error" {
		derivedStatus = "not_checked_no_raw"
	}
	vectorPipelineStatus := vectorStatus
	if vectorPipelineStatus == "" {
		vectorPipelineStatus = "not_requested"
	}
	if s.TurnWorkflows != nil && workflowRequestID != "" {
		s.TurnWorkflows.setPersistenceFacts(
			workflowRequestID,
			rawStatus,
			rawSave.ChatLogsSaved,
			derivedStatus,
			derivedCommitted,
			vectorPipelineStatus,
			vectorsUpserted,
		)
		if derivedPersistenceFailed {
			s.TurnWorkflows.setPersistenceFailureDetail(
				workflowRequestID,
				"derived_memory",
				"DERIVED_PERSIST_FAILED",
				fmt.Sprintf(
					"attempted=%d / committed=%d / transaction=%s / %s",
					derivedWriteAttempted,
					derivedCommitted,
					derivedRollbackState,
					completeTurnPersistenceFailureSummary(derivedDiagnostics),
				),
				derivedCommitted,
			)
		}
		s.TurnWorkflows.setCounts(workflowRequestID, turnWorkflowHUDCountsFromComplete(
			rawSave.UserDurable,
			rawSave.AssistantDurable,
			effectiveInputSaved,
			memoriesSaved,
			preciseMemoryUnitsSaved,
			evidenceSaved,
			kgTriplesSaved,
			subjectiveEntityMemoriesSaved,
			worldRulesSaved,
			characterStatesSaved,
			physicalConditionsSaved,
			entityConditionsSaved,
			statusSchemaDefinitionsSaved,
			statusEffectsSaved,
			characterEventsSaved,
			storylinesSaved,
			narrativeCurrentStatesSaved,
			narrativeStateEventsSaved,
			relationshipCurrentStatesSaved,
			relationshipStateEventsSaved,
			pendingThreadsSaved,
			activeStatesSaved,
			canonicalStateLayersSaved,
			entitiesSaved,
			trustStatesSaved,
			entityIdentitiesSaved,
			identitySurfacesSaved,
			identityBindingsSaved,
			speakerAttributionsSaved,
			episodeSummariesSaved,
			vectorsUpserted,
		))
		switch {
		case strings.TrimSpace(stringFromMap(episodeResult, "error")) != "":
			s.TurnWorkflows.fail(workflowRequestID, "EPISODE_CHECKPOINT_FAILED", "turn_hud.error.episode_checkpoint_failed", turnWorkflowStageCheckpoints, true)
		case strings.TrimSpace(stringFromMap(hierarchyPromotionResult, "error")) != "":
			s.TurnWorkflows.fail(workflowRequestID, "HIERARCHY_CHECKPOINT_FAILED", "turn_hud.error.hierarchy_checkpoint_failed", turnWorkflowStageCheckpoints, true)
		default:
			s.TurnWorkflows.finishStage(workflowRequestID, turnWorkflowStageCheckpoints, "succeeded", "")
			if maintenanceHandoff.Errors > 0 {
				s.TurnWorkflows.addWarning(workflowRequestID, "MAINTENANCE_HANDOFF_FAILED", "turn_hud.warning.maintenance_handoff_failed", turnWorkflowStageCheckpoints)
			}
			if sourceAcceptance.Enabled && sourceAcceptance.Accepted && sourceAcceptance.ReplaceExisting {
				s.TurnWorkflows.completeWithNotice(
					workflowRequestID,
					"turn_hud.notice.reroll_confirmed",
					"turn_hud.notice.reroll_confirmed_detail",
					"LOGICAL_TURN_REPLACED",
				)
			} else {
				s.TurnWorkflows.complete(workflowRequestID)
			}
		}
	}
	persistencePipeline := map[string]any{
		"contract_version": "complete_turn.persistence_pipeline.v1",
		"raw": map[string]any{
			"status":                rawStatus,
			"chat_logs_saved":       chatLogsSaved,
			"effective_input_saved": effectiveInputSaved,
		},
		"derived": map[string]any{
			"status":                           derivedStatus,
			"attempted":                        derivedWriteAttempted,
			"committed":                        derivedCommitted,
			"rollback_state":                   derivedRollbackState,
			"error_count":                      derivedWriteErrors,
			"error_diagnostics":                derivedDiagnostics,
			"artifacts_saved":                  derivedCommitted,
			"memories_saved":                   memoriesSaved,
			"precise_memory_units_saved":       preciseMemoryUnitsSaved,
			"direct_evidence_saved":            evidenceSaved,
			"kg_triples_saved":                 kgTriplesSaved,
			"world_rules_saved":                worldRulesSaved,
			"subjective_entity_memories_saved": subjectiveEntityMemoriesSaved,
			"character_states_saved":           characterStatesSaved,
			"physical_conditions_saved":        physicalConditionsSaved,
			"entity_conditions_saved":          entityConditionsSaved,
			"status_schema_definitions_saved":  statusSchemaDefinitionsSaved,
			"status_effects_saved":             statusEffectsSaved,
			"narrative_current_states_saved":   narrativeCurrentStatesSaved,
			"narrative_state_events_saved":     narrativeStateEventsSaved,
			"canonical_state_layers_saved":     canonicalStateLayersSaved,
			"entity_identities_saved":          entityIdentitiesSaved,
			"identity_surfaces_saved":          identitySurfacesSaved,
			"identity_links_saved":             identityLinksSaved,
			"identity_bindings_saved":          identityBindingsSaved,
			"speaker_attributions_saved":       speakerAttributionsSaved,

			"relationship_current_states_saved": relationshipCurrentStatesSaved,
			"relationship_state_events_saved":   relationshipStateEventsSaved,
			"habit_evidence_current_saved":      habitEvidenceCurrentSaved,
			"habit_evidence_events_saved":       habitEvidenceEventsSaved,
			"character_profiles_saved":          characterProfilesSaved,
			"voice_behavior_projections_saved":  voiceBehaviorProjectionsSaved,
		},
		"vector": map[string]any{
			"status":                   vectorPipelineStatus,
			"embedding_status":         embeddingStatus,
			"upserted_total":           vectorsUpserted,
			"memory_upserted":          vectorsMemoryUpserted,
			"direct_evidence_upserted": vectorsEvidenceUpserted,
			"world_rule_upserted":      vectorsWorldRuleUpserted,
		},
	}
	backendTiming := timing.snapshot()
	derivedRetryRequired := rawTurnDurable && reprocessingReason != "" && reprocessingDurable
	reconciliationRequired := rawTurnDurable && derivedPersistenceFailed
	nonDurableDerivedRetryRequired := rawTurnDurable &&
		(criticFailureReason != "" || derivedPersistenceFailed) &&
		!derivedRetryRequired
	responseStatus := "ok"
	queueAction := ""
	retryable := false
	commitState := "not_committed"
	if rawTurnDurable {
		commitState = "committed"
	}
	if rawTurnDurable && derivedRetryRequired {
		responseStatus = "partial"
		queueAction = "discard"
	} else if nonDurableDerivedRetryRequired {
		responseStatus = "partial"
		queueAction = "retry"
		retryable = true
	} else if !rawTurnDurable && s.usesShadowWriteStore() {
		responseStatus = "error"
		queueAction = "retry"
		retryable = true
	}
	reconciliationRetryIdempotencyKey := ""
	if nonDurableDerivedRetryRequired {
		reconciliationRetryIdempotencyKey = completeTurnReconciliationRetryIdempotencyKey(req.ClientMeta)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":                               responseStatus,
		"source":                               writeSource,
		"chat_session_id":                      sid,
		"turn_index":                           turnIndex,
		"generated_at":                         time.Now().UTC().Format(time.RFC3339),
		"save_ok":                              saveOK,
		"save_error":                           saveErr,
		"raw_committed":                        rawTurnDurable,
		"commit_state":                         commitState,
		"reconciliation_required":              reconciliationRequired,
		"reconciliation_retry_idempotency_key": nilIfEmpty(reconciliationRetryIdempotencyKey),
		"retryable":                            retryable,
		"queue_action":                         queueAction,
		"memories_saved":                       memoriesSaved,
		"precise_memory_units_saved":           preciseMemoryUnitsSaved,
		"evidence_saved":                       evidenceSaved,
		"kg_triples_saved":                     kgTriplesSaved,
		"persona_capsule_candidates":           personaCapsuleCandidates,
		"subjective_entity_memories_saved":     subjectiveEntityMemoriesSaved,
		"character_events_saved":               characterEventsSaved,
		"storylines_saved":                     storylinesSaved,
		"world_rules_saved":                    worldRulesSaved,
		"character_states_saved":               characterStatesSaved,
		"physical_conditions_saved":            physicalConditionsSaved,
		"entity_conditions_saved":              entityConditionsSaved,
		"status_schema_definitions_saved":      statusSchemaDefinitionsSaved,
		"status_effects_saved":                 statusEffectsSaved,
		"narrative_current_states_saved":       narrativeCurrentStatesSaved,
		"narrative_state_events_saved":         narrativeStateEventsSaved,
		"relationship_current_states_saved":    relationshipCurrentStatesSaved,
		"relationship_state_events_saved":      relationshipStateEventsSaved,
		"habit_evidence_current_saved":         habitEvidenceCurrentSaved,
		"habit_evidence_events_saved":          habitEvidenceEventsSaved,
		"character_profiles_saved":             characterProfilesSaved,
		"voice_behavior_projections_saved":     voiceBehaviorProjectionsSaved,
		"pending_threads_saved":                pendingThreadsSaved,
		"active_states_saved":                  activeStatesSaved,
		"canonical_state_layers_saved":         canonicalStateLayersSaved,
		"entities_saved":                       entitiesSaved,
		"entity_identities_saved":              entityIdentitiesSaved,
		"identity_surfaces_saved":              identitySurfacesSaved,
		"identity_links_saved":                 identityLinksSaved,
		"identity_bindings_saved":              identityBindingsSaved,
		"speaker_attributions_saved":           speakerAttributionsSaved,
		"trust_states_saved":                   trustStatesSaved,
		"vectors_upserted":                     vectorsUpserted,
		"vectors_memory_upserted":              vectorsMemoryUpserted,
		"vectors_evidence_upserted":            vectorsEvidenceUpserted,
		"vectors_world_rule_upserted":          vectorsWorldRuleUpserted,
		"chat_logs_saved":                      chatLogsSaved,
		"effective_input_saved":                effectiveInputSaved,
		"audit_saved":                          auditSaved,
		"critic_feedback_saved":                criticFeedbackSaved,
		"store_write_attempted":                storeWriteAttempted,
		"store_write_errors":                   storeWriteErrors,
		"store_write_error_details":            completeTurnPersistenceDiagnosticMessages(s.completeTurnPersistenceDiagnostics(storeWriteErrorDetails)),
		"derived_write_attempted":              derivedWriteAttempted,
		"derived_write_committed":              derivedCommitted,
		"derived_write_errors":                 derivedWriteErrors,
		"derived_write_error_diagnostics":      derivedDiagnostics,
		"derived_write_rollback_state":         derivedRollbackState,
		"critic_triggered":                     criticTriggered,
		"critic_result":                        criticResult,
		"critic_failure":                       criticFailure,
		"language_context":                     languageContext,
		"llm_config_trace":                     llmConfigTrace,
		"derived_artifacts_saved":              derivedCommitted,
		"derived_retry_required":               derivedRetryRequired,
		"source_acceptance":                    completeTurnSourceAcceptancePayload(sourceAcceptance),
		"source_to_final_lineage":              buildSourceToFinalLineage(req, sourceAcceptance),
		"episode_result":                       episodeResult,
		"chapter_result":                       nil,
		"hierarchy_promotion_result":           hierarchyPromotionResult,
		"persistence_pipeline":                 persistencePipeline,
		"backend_timing":                       backendTiming,
		"turn_workflow_hud":                    s.turnWorkflowHUDSnapshot(workflowRequestID),
		"maintenance_enqueued":                 maintenanceHandoff.Enqueued,
		"maintenance_audit_recorded":           maintenanceHandoff.AuditRecorded,
		"memory_reprocessing_queue": map[string]any{
			"required":            reprocessingReason != "",
			"durable_or_existing": reprocessingDurable,
			"source_revision":     nilIfEmpty(sourceAcceptance.Revision),
			"reason_code":         nilIfEmpty(reprocessingReason),
		},
		"fail_reasons": failReasons,
		"trace_handoff": map[string]any{
			"shadow_mode":                              s.Cfg.StoreMode != config.StoreModeMariaDBAuthority,
			"store_mode":                               string(s.Cfg.StoreMode),
			"save_ok":                                  saveOK,
			"critic_attempted":                         extractionCfg.Critic.hasConfig(),
			"critic_triggered":                         criticTriggered,
			"llm_config_trace":                         llmConfigTrace,
			"derived_artifacts_saved":                  derivedCommitted,
			"derived_retry_required":                   derivedRetryRequired,
			"critic_trace":                             criticTrace,
			"critic_pipeline_version":                  completeTurnCriticPipelineVersion,
			"language_context":                         languageContext,
			"critic_pipeline_split_enabled":            true,
			"critic_pipeline_all_in_single_call":       false,
			"critic_pipeline_extractor_stage":          "complete_turn.configured_critic_extract",
			"critic_pipeline_reducer_stage":            "complete_turn.saveCriticExtractionArtifacts",
			"critic_pipeline_compactor_stage":          "maintenance_handoff_shadow",
			"critic_pipeline_compactor_owner":          "complete_turn.maintenance_handoff",
			"persona_capsule_candidate_policy":         "proposal_only_auto_create_disabled",
			"persona_capsule_candidates":               personaCapsuleCandidates,
			"subjective_entity_memories_saved":         subjectiveEntityMemoriesSaved,
			"subjective_entity_memory_policy":          "support_only_entity_subjective_memory_bank",
			"critic_preview_pass_version":              completeTurnCriticPreviewPassVersion,
			"critic_preview_pass_enabled":              true,
			"critic_preview_pass_scope":                "recent_raw_and_direct_evidence",
			"critic_preview_compaction_mode":           "hint_only",
			"canonical_state_promotion_policy_version": "hs1.verified_only.v1",
			"canonical_state_layers_saved":             canonicalStateLayersSaved,
			"canonical_state_hard_floor_enabled":       true,
			"physical_conditions_saved":                physicalConditionsSaved,
			"entity_conditions_saved":                  entityConditionsSaved,
			"status_schema_definitions_saved":          statusSchemaDefinitionsSaved,
			"status_effects_saved":                     statusEffectsSaved,
			"relationship_current_states_saved":        relationshipCurrentStatesSaved,
			"relationship_state_events_saved":          relationshipStateEventsSaved,
			"habit_evidence_current_saved":             habitEvidenceCurrentSaved,
			"habit_evidence_events_saved":              habitEvidenceEventsSaved,
			"character_profiles_saved":                 characterProfilesSaved,
			"voice_behavior_projections_saved":         voiceBehaviorProjectionsSaved,
			"physical_condition_policy":                "evidence_bound_status_effect_no_default_duration",
			"entity_condition_policy":                  "evidence_bound_entity_status_effect_no_default_duration",
			"canonical_state_upsert": map[string]any{
				"cost_measurement_policy_version": "lc1b.v1",
				"cost_measurement":                canonicalStateWriteCost,
			},
			"conflict_resolution_version":              "ea1h.v1",
			"conflict_confidence_policy_version":       "ea1i.v1",
			"conflict_resolutions":                     conflictResolutions,
			"direct_evidence_retention_policy_version": completeTurnDirectEvidenceRetentionVersion,
			"direct_evidence_retention_enabled":        true,
			"direct_evidence_retention_mode":           "importance_lineage_ttl",
			"retention_decisions":                      retentionDecisions,
			"maintenance_enqueued":                     maintenanceHandoff.Enqueued,
			"maintenance_audit_recorded":               maintenanceHandoff.AuditRecorded,
			"maintenance_queue_status":                 maintenanceHandoff.QueueStatus,
			"maintenance_queue_depth":                  maintenanceHandoff.QueueDepth,
			"maintenance_refresh_enabled":              maintenanceHandoff.RefreshEnabled,
			"maintenance_refresh_plan":                 maintenanceHandoff.RefreshPlan,
			"maintenance_handoff":                      maintenanceHandoff.Trace,
			"hierarchy_promotion_policy":               completeTurnHierarchyPromotionVersion,
			"hierarchy_promotion":                      hierarchyPromotionResult,
			"embedding_status":                         embeddingStatus,
			"vector_status":                            vectorStatus,
			"persistence_pipeline":                     persistencePipeline,
			"note":                                     "complete-turn owns save, critic extraction, maintenance handoff, and JS adapter handoff; no fake memory/evidence/KG placeholders are written",
		},
		"writeback_plan": writebackPlan,
		"warnings":       warnings,
		"note":           note,
	})
}

func (s *Server) writeCompleteTurnMigrationSourceLockBlocked(
	w http.ResponseWriter,
	req dto.M4CompleteTurnRequest,
	sid string,
	workflowRequestID string,
	lock *store.SessionMigrationLock,
) {
	if s.TurnWorkflows != nil && workflowRequestID != "" {
		s.TurnWorkflows.invalidate(workflowRequestID, "source_session_migrated_away")
	}
	now := time.Now().UTC()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":                           "blocked",
		"source":                           s.storeWriteSource(),
		"chat_session_id":                  sid,
		"turn_index":                       req.TurnIndex,
		"generated_at":                     now.Format(time.RFC3339),
		"save_ok":                          false,
		"save_error":                       "source_session_migrated_away",
		"chat_logs_saved":                  0,
		"memories_saved":                   0,
		"evidence_saved":                   0,
		"kg_triples_saved":                 0,
		"persona_capsule_candidates":       0,
		"subjective_entity_memories_saved": 0,
		"derived_artifacts_saved":          0,
		"critic_triggered":                 false,
		"critic_result":                    nil,
		"maintenance_enqueued":             false,
		"fail_reasons":                     []string{"source_session_migrated_away"},
		"migration_source_lock":            sessionMigrationLockPayload(lock),
		"trace_handoff": map[string]any{
			"skeleton":              false,
			"turn_index":            req.TurnIndex,
			"save_ok":               false,
			"critic_triggered":      false,
			"store_mode":            string(s.Cfg.StoreMode),
			"store_write_source":    s.storeWriteSource(),
			"migration_source_lock": sessionMigrationLockPayload(lock),
			"note":                  "complete-turn refused writes for a migrated-away source session",
		},
		"warnings": []string{"source_session_migrated_away: continue in target_session_id " + lock.TargetSessionID},
		"note":     "complete-turn blocked because this source session has been migrated away",
	})
}

func (s *Server) completeTurnEpisodeCheckpoint(ctx context.Context, sid string, turnIndex int, meta map[string]any) map[string]any {
	interval := normalizedEpisodeInterval(intFromAny(meta["episode_interval_turns"], 0))
	result := map[string]any{
		"checked":   true,
		"triggered": false,
		"interval":  interval,
		"range":     nil,
	}
	if s == nil || s.Store == nil {
		result["reason"] = "store_unavailable"
		return result
	}
	if turnIndex <= 0 {
		result["reason"] = "turn_index_missing"
		return result
	}
	fromTurn := ((turnIndex - 1) / interval * interval) + 1
	toTurn := fromTurn + interval - 1
	result["range"] = []int{fromTurn, toTurn}
	if turnIndex < toTurn {
		result["reason"] = "episode_interval_not_closed"
		return result
	}
	logs, err := s.Store.ListChatLogs(ctx, sid, 0, 0)
	if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
		result["status"] = "partial_error"
		result["error"] = "list_chat_logs: " + err.Error()
		return result
	}
	memories, err := s.Store.ListMemories(ctx, sid, 0, 0)
	if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
		result["status"] = "partial_error"
		result["error"] = "list_memories: " + err.Error()
		return result
	}
	evidence, err := s.Store.ListEvidence(ctx, sid)
	if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
		result["status"] = "partial_error"
		result["error"] = "list_evidence: " + err.Error()
		return result
	}
	checkpoint := s.backfillEpisodeSummariesFromChatLogs(ctx, sid, logs, memories, evidence, interval, false, map[int]bool{turnIndex: true}, false)
	checkpoint["checked"] = true
	checkpoint["triggered"] = true
	checkpoint["range"] = []int{fromTurn, toTurn}
	checkpoint["policy"] = "complete_turn_interval_checkpoint"
	return checkpoint
}

func (s *Server) completeTurnHierarchyPromotionCheckpoint(ctx context.Context, sid string, turnIndex int, meta map[string]any) map[string]any {
	result := map[string]any{
		"checked":   true,
		"triggered": false,
		"policy":    completeTurnHierarchyPromotionVersion,
		"mode":      "guarded_closed_ranges",
	}
	if s == nil || s.Store == nil {
		result["reason"] = "store_unavailable"
		return result
	}
	if turnIndex <= 0 {
		result["reason"] = "turn_index_missing"
		return result
	}
	if !completeTurnBoolFromAny(meta["long_session_refresh_enabled"]) {
		result["reason"] = "long_session_refresh_disabled"
		return result
	}
	chapterEnabled := completeTurnBoolFromAny(meta["chapter_auto_enabled"])
	arcEnabled := true
	if _, ok := meta["arc_auto_enabled"]; ok {
		arcEnabled = completeTurnBoolFromAny(meta["arc_auto_enabled"])
	}
	sagaEnabled := true
	if _, ok := meta["saga_auto_enabled"]; ok {
		sagaEnabled = completeTurnBoolFromAny(meta["saga_auto_enabled"])
	}
	if !chapterEnabled && !arcEnabled && !sagaEnabled {
		result["reason"] = "hierarchy_layers_disabled"
		return result
	}
	logs, err := s.Store.ListChatLogs(ctx, sid, 0, 0)
	if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
		result["status"] = "partial_error"
		result["error"] = "list_chat_logs: " + err.Error()
		return result
	}
	promotionMeta := map[string]any{}
	for k, v := range meta {
		promotionMeta[k] = v
	}
	backfill := s.backfillHierarchySummaries(ctx, sid, logs, map[int]bool{turnIndex: true}, promotionMeta, false)
	result["triggered"] = true
	result["reason"] = nil
	result["backfill"] = backfill
	result["status"] = backfill["status"]
	result["chapter"] = backfill["chapter"]
	result["arc"] = backfill["arc"]
	result["saga"] = backfill["saga"]
	if errText := strings.TrimSpace(stringFromMap(backfill, "error")); errText != "" {
		result["error"] = errText
	}
	return result
}

func completeTurnAlreadyPersistedWithContent(logs []store.ChatLog, sid string, turnIndex int, userText, assistantText string) bool {
	hasUser := false
	hasAssistant := false
	normalizedUser := completeTurnComparableContentForRole("user", userText)
	normalizedAssistant := completeTurnComparableContentForRole("assistant", assistantText)
	if normalizedUser == "" || normalizedAssistant == "" {
		return false
	}
	for _, item := range logs {
		if item.ChatSessionID != sid || item.TurnIndex != turnIndex {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(item.Role))
		content := completeTurnComparableContentForRole(role, item.Content)
		switch role {
		case "user":
			if content == normalizedUser {
				hasUser = true
			}
		case "assistant":
			if content == normalizedAssistant {
				hasAssistant = true
			}
		}
	}
	return hasUser && hasAssistant
}

func completeTurnFindPersistedTurnWithContent(logs []store.ChatLog, sid string, userText, assistantText string) (int, bool) {
	normalizedUser := completeTurnComparableContentForRole("user", userText)
	normalizedAssistant := completeTurnComparableContentForRole("assistant", assistantText)
	if normalizedUser == "" || normalizedAssistant == "" {
		return 0, false
	}
	type pairPresence struct {
		user      bool
		assistant bool
	}
	byTurn := map[int]pairPresence{}
	for _, item := range logs {
		if item.ChatSessionID != sid || item.TurnIndex <= 0 {
			continue
		}
		presence := byTurn[item.TurnIndex]
		role := strings.ToLower(strings.TrimSpace(item.Role))
		content := completeTurnComparableContentForRole(role, item.Content)
		switch role {
		case "user":
			if content == normalizedUser {
				presence.user = true
			}
		case "assistant":
			if content == normalizedAssistant {
				presence.assistant = true
			}
		}
		byTurn[item.TurnIndex] = presence
	}
	bestTurn := 0
	for turn, presence := range byTurn {
		if presence.user && presence.assistant && (bestTurn == 0 || turn < bestTurn) {
			bestTurn = turn
		}
	}
	if bestTurn <= 0 {
		return 0, false
	}
	return bestTurn, true
}

func completeTurnComparableContentForRole(role, text string) string {
	normalizedRole := strings.ToLower(strings.TrimSpace(role))
	content := text
	if normalizedRole == "assistant" {
		content = completeTurnCanonicalAssistantPersistenceText(content)
	}
	return completeTurnLooseCompareText(content)
}

func completeTurnLooseCompareText(text string) string {
	clean := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n"))
	if clean == "" {
		return ""
	}
	return strings.Join(strings.Fields(clean), " ")
}

func completeTurnCanonicalAssistantPersistenceText(text string) string {
	raw := strings.TrimSpace(text)
	if raw == "" {
		return ""
	}
	for _, tag := range []string{
		"ArchiveCenterFinalOutput",
		"ArchiveCenterFinal",
		"ACFinalOutput",
		"ACFinal",
		"PostprocessorFinalOutput",
		"PostprocessorFinal",
		"PostProcessFinal",
		"FinalAssistantOutput",
		"CanonicalAssistantOutput",
		"QualityLayerFinalOutput",
		"QualityLayerFinal",
	} {
		if blocks := completeTurnExtractTaggedBlocks(raw, tag); len(blocks) > 0 {
			return strings.TrimSpace(strings.Join(blocks, "\n\n"))
		}
	}
	if strings.Contains(strings.ToLower(raw), "<rekocompare") || strings.Contains(strings.ToLower(raw), "<rekoresult") {
		visible := completeTurnRemoveTaggedBlocks(raw, "ReKoCompare")
		if strings.TrimSpace(visible) != "" {
			return strings.TrimSpace(visible)
		}
		if blocks := completeTurnExtractTaggedBlocks(raw, "ReKoAfter"); len(blocks) > 0 {
			return strings.TrimSpace(strings.Join(blocks, "\n\n"))
		}
		if blocks := completeTurnExtractTaggedBlocks(raw, "ReKoResult"); len(blocks) > 0 {
			return strings.TrimSpace(strings.Join(blocks, "\n\n"))
		}
	}
	if strings.Contains(strings.ToLower(raw), "<gigatrans") {
		if blocks := completeTurnExtractTaggedBlocks(raw, "GigaTrans"); len(blocks) > 0 {
			return strings.TrimSpace(strings.Join(blocks, "\n\n"))
		}
	}
	return raw
}

func completeTurnExtractTaggedBlocks(text, tagName string) []string {
	tag := strings.TrimSpace(tagName)
	if tag == "" || strings.TrimSpace(text) == "" {
		return nil
	}
	re := regexp.MustCompile(`(?is)<\s*` + regexp.QuoteMeta(tag) + `\b[^>]*>(.*?)<\s*/\s*` + regexp.QuoteMeta(tag) + `\s*>`)
	matches := re.FindAllStringSubmatch(text, -1)
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		if block := strings.TrimSpace(match[1]); block != "" {
			out = append(out, block)
		}
	}
	return out
}

func completeTurnRemoveTaggedBlocks(text, tagName string) string {
	tag := strings.TrimSpace(tagName)
	if tag == "" || strings.TrimSpace(text) == "" {
		return text
	}
	re := regexp.MustCompile(`(?is)<\s*` + regexp.QuoteMeta(tag) + `\b[^>]*>.*?<\s*/\s*` + regexp.QuoteMeta(tag) + `\s*>`)
	return strings.TrimSpace(re.ReplaceAllString(text, ""))
}

type completeTurnRawSaveResult struct {
	ChatLogsSaved    int
	Attempted        int
	Errors           int
	ErrorDetails     []string
	Warnings         []string
	UserDurable      bool
	AssistantDurable bool
}

func (s *Server) persistCompleteTurnRaw(ctx context.Context, sid string, turnIndex int, userText, assistantText string, now time.Time, pairAlreadyPersisted, userAlreadyPersisted, assistantAlreadyPersisted bool) completeTurnRawSaveResult {
	result := completeTurnRawSaveResult{
		UserDurable:      userAlreadyPersisted,
		AssistantDurable: assistantAlreadyPersisted,
	}
	if !s.usesShadowWriteStore() {
		return result
	}
	if pairAlreadyPersisted {
		result.UserDurable = true
		result.AssistantDurable = true
		result.Warnings = append(result.Warnings, "raw_chat_logs_already_persisted: duplicate raw save skipped")
		return result
	}
	if userAlreadyPersisted {
		result.Warnings = append(result.Warnings, "raw_user_chat_log_already_persisted: duplicate user raw save skipped")
	} else {
		result.Attempted++
		if err := s.Store.SaveChatLog(ctx, &store.ChatLog{
			ChatSessionID: sid,
			TurnIndex:     turnIndex,
			Role:          "user",
			Content:       userText,
			CreatedAt:     now,
		}); err != nil {
			result.Errors++
			result.ErrorDetails = append(result.ErrorDetails, "SaveChatLog(user): "+err.Error())
		} else {
			result.ChatLogsSaved++
			result.UserDurable = true
		}
	}
	if assistantAlreadyPersisted {
		result.Warnings = append(result.Warnings, "raw_assistant_chat_log_already_persisted: duplicate assistant raw save skipped")
	} else {
		result.Attempted++
		if err := s.Store.SaveChatLog(ctx, &store.ChatLog{
			ChatSessionID: sid,
			TurnIndex:     turnIndex,
			Role:          "assistant",
			Content:       assistantText,
			CreatedAt:     now,
		}); err != nil {
			result.Errors++
			result.ErrorDetails = append(result.ErrorDetails, "SaveChatLog(assistant): "+err.Error())
		} else {
			result.ChatLogsSaved++
			result.AssistantDurable = true
		}
	}
	return result
}

func completeTurnRawRolePresence(logs []store.ChatLog, sid string, turnIndex int) (bool, bool) {
	hasUser := false
	hasAssistant := false
	for _, item := range logs {
		if item.ChatSessionID != sid || item.TurnIndex != turnIndex {
			continue
		}
		if strings.TrimSpace(item.Content) == "" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(item.Role)) {
		case "user":
			hasUser = true
		case "assistant":
			hasAssistant = true
		}
	}
	return hasUser, hasAssistant
}

func completeTurnRawRoleContentMatches(logs []store.ChatLog, sid string, turnIndex int, userText, assistantText string) (bool, bool) {
	userMatches := false
	assistantMatches := false
	normalizedUser := completeTurnComparableContentForRole("user", userText)
	normalizedAssistant := completeTurnComparableContentForRole("assistant", assistantText)
	for _, item := range logs {
		if item.ChatSessionID != sid || item.TurnIndex != turnIndex {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(item.Role))
		content := completeTurnComparableContentForRole(role, item.Content)
		switch role {
		case "user":
			if normalizedUser != "" && content == normalizedUser {
				userMatches = true
			}
		case "assistant":
			if normalizedAssistant != "" && content == normalizedAssistant {
				assistantMatches = true
			}
		}
	}
	return userMatches, assistantMatches
}

func completeTurnHasDerivedArtifacts(ctx context.Context, st store.Store, sid string, turnIndex int) bool {
	if st == nil || strings.TrimSpace(sid) == "" || turnIndex <= 0 {
		return false
	}
	if memories, err := st.ListMemories(ctx, sid, turnIndex, turnIndex); err == nil {
		for _, item := range memories {
			if item.ChatSessionID == sid && item.TurnIndex == turnIndex {
				return true
			}
		}
	}
	if evidence, err := st.ListEvidence(ctx, sid); err == nil {
		for _, item := range evidence {
			if item.ChatSessionID != sid || item.Tombstoned {
				continue
			}
			if item.TurnAnchor == turnIndex || item.SourceTurnStart == turnIndex || item.SourceTurnEnd == turnIndex {
				return true
			}
		}
	}
	if triples, err := st.ListKGTriples(ctx, sid); err == nil {
		for _, item := range triples {
			if item.ChatSessionID == sid && (item.SourceTurn == turnIndex || item.ValidFrom == turnIndex) {
				return true
			}
		}
	}
	return false
}

func (s *Server) buildCompleteTurnMaintenanceHandoff(ctx context.Context, sid string, turnIndex int, saveOK bool, now time.Time, writeSource string, req dto.M4CompleteTurnRequest) completeTurnMaintenanceHandoff {
	handoff := completeTurnMaintenanceHandoff{
		QueueStatus: "audit_not_recorded",
		QueueDepth:  0,
		RefreshPlan: map[string]any{},
		Trace: map[string]any{
			"owner":          "complete_turn",
			"version":        completeTurnMaintenancePlanVersion,
			"worker_enabled": false,
			"queue_mode":     "none",
			"audit_mode":     "plan_only",
			"status":         "audit_not_recorded",
		},
	}
	if !s.usesShadowWriteStore() {
		handoff.QueueStatus = "skipped_store_write_disabled"
		handoff.Trace["status"] = handoff.QueueStatus
		return handoff
	}
	if !saveOK {
		handoff.QueueStatus = "skipped_save_failed"
		handoff.Trace["status"] = handoff.QueueStatus
		return handoff
	}
	if s.Store == nil {
		handoff.QueueStatus = "skipped_store_missing"
		handoff.Trace["status"] = handoff.QueueStatus
		return handoff
	}

	meta := req.ClientMeta
	refreshEnabled := completeTurnBoolFromAny(meta["long_session_refresh_enabled"])
	chapterAutoEnabled := completeTurnBoolFromAny(meta["chapter_auto_enabled"])
	arcAutoEnabled := true
	if _, ok := meta["arc_auto_enabled"]; ok {
		arcAutoEnabled = completeTurnBoolFromAny(meta["arc_auto_enabled"])
	}
	sagaAutoEnabled := true
	if _, ok := meta["saga_auto_enabled"]; ok {
		sagaAutoEnabled = completeTurnBoolFromAny(meta["saga_auto_enabled"])
	}
	plan := map[string]any{
		"enabled": refreshEnabled,
		"version": completeTurnMaintenancePlanVersion,
		"mode":    "complete_turn_maintenance_audit",
		"layers": map[string]any{
			"chapter": map[string]any{
				"enabled":           refreshEnabled && chapterAutoEnabled,
				"interval_episodes": intFromAny(meta["chapter_interval_episodes"], 0),
			},
			"arc": map[string]any{
				"enabled":           refreshEnabled && arcAutoEnabled,
				"interval_chapters": intFromAny(meta["arc_interval_chapters"], 0),
			},
			"saga": map[string]any{
				"enabled":       refreshEnabled && sagaAutoEnabled,
				"interval_arcs": intFromAny(meta["saga_interval_arcs"], 0),
			},
		},
		"worker_enabled": false,
		"queue_mode":     "none",
		"audit_only":     true,
	}

	handoff.RefreshEnabled = refreshEnabled
	handoff.RefreshPlan = plan
	handoff.QueueStatus = "audit_recorded"
	handoff.QueueDepth = 0
	handoff.Enqueued = false
	handoff.Trace = map[string]any{
		"owner":                    "complete_turn",
		"version":                  completeTurnMaintenancePlanVersion,
		"status":                   handoff.QueueStatus,
		"queue_depth":              handoff.QueueDepth,
		"worker_enabled":           false,
		"queue_mode":               "none",
		"audit_mode":               "plan_only",
		"maintenance_pass_enabled": false,
		"refresh_enabled":          refreshEnabled,
		"refresh_plan":             plan,
	}

	handoff.Attempted = 1
	err := s.Store.SaveAuditLog(ctx, &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "maintenance_audit_recorded",
		TargetType:    "turn",
		TargetID:      int64(turnIndex),
		Summary:       fmt.Sprintf("complete-turn maintenance plan recorded turn %d", turnIndex),
		DetailsJSON:   mustCompactJSON(plan),
		Source:        writeSource,
		CreatedAt:     now,
	})
	if err != nil {
		handoff.AuditRecorded = false
		handoff.QueueStatus = "audit_record_failed"
		handoff.QueueDepth = 0
		handoff.Errors = 1
		handoff.ErrorDetails = append(handoff.ErrorDetails, "SaveAuditLog(maintenance_audit_recorded): "+err.Error())
		handoff.Trace["status"] = handoff.QueueStatus
		handoff.Trace["queue_depth"] = 0
		handoff.Trace["error"] = err.Error()
		return handoff
	}
	handoff.AuditSaved = 1
	handoff.AuditRecorded = true
	return handoff
}

func completeTurnBoolFromAny(v any) bool {
	switch typed := v.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "1", "true", "yes", "on", "enabled":
			return true
		default:
			return false
		}
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case float64:
		return typed != 0
	case json.Number:
		i, err := typed.Int64()
		return err == nil && i != 0
	default:
		return false
	}
}

func completeTurnActualEmptyUserInput(meta map[string]any) bool {
	if meta == nil {
		return false
	}
	if completeTurnBoolFromAny(meta["actual_empty_user_input"]) {
		return true
	}
	if kind, ok := meta["user_input_kind"].(string); ok && strings.EqualFold(strings.TrimSpace(kind), "auto_continue") {
		return true
	}
	if key, ok := meta["logical_user_turn_key"].(string); ok && strings.TrimSpace(key) == completeTurnAutoContinueUserInputMarker {
		return true
	}
	return false
}

func mustCompactJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// handlePrepareTurn replaces the degraded/off placeholder with a Store-backed
// read assembly where possible. No LLM calls and no writes are performed.
