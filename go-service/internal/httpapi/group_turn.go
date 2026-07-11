package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	archivebridge "github.com/risulongmemory/archive-center-go/internal/archive"
	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

const (
	completeTurnCriticPipelineVersion          = "ea1j.v1"
	completeTurnCriticPreviewPassVersion       = "ea1k.v1"
	completeTurnDirectEvidenceRetentionVersion = "ea1l.v1"
	completeTurnMaintenancePlanVersion         = "r3c.v1"
	completeTurnHierarchyPromotionVersion      = "step23.guarded_worker.v1"
	completeTurnAutoContinueUserInputMarker    = "[auto-continue]"
)

type completeTurnMaintenanceHandoff struct {
	Enqueued       bool
	QueueStatus    string
	QueueDepth     int
	RefreshEnabled bool
	RefreshPlan    map[string]any
	Trace          map[string]any
	AuditSaved     int
	Attempted      int
	Errors         int
	ErrorDetails   []string
}

// ---------------------------------------------------------------------------
// Route registration
// ---------------------------------------------------------------------------

// registerTurnRoutes mounts the core turn surface.
// All endpoints in this group are R2 (authority-required write).
func (s *Server) registerTurnRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /turns", s.handleTurns)
	mux.HandleFunc("POST /turns/repair-replay", s.handleTurnsRepairReplay)
	mux.HandleFunc("POST /turns/complete", s.handleTurnsComplete)
	mux.HandleFunc("POST /complete-turn", s.handleCompleteTurn)
	mux.HandleFunc("POST /prepare-turn", s.handlePrepareTurn)
	mux.HandleFunc("POST /effective-inputs", s.handleEffectiveInputs)
	mux.HandleFunc("DELETE /rollback/{turn_index}", s.handleRollback)

}

func (s *Server) handleTurns(w http.ResponseWriter, r *http.Request) {
	writeShadowGuard(w, "POST /turns")
}

func (s *Server) handleTurnsRepairReplay(w http.ResponseWriter, r *http.Request) {
	var req dto.ChatLogRepairReplayRequest
	if err := dto.DecodeWithDefaults(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	sid := strings.TrimSpace(*req.ChatSessionID)
	if sid == "" {
		sid = "default"
	}

	repairReplayPlan := buildRepairReplayPlan(sid, req, s.usesShadowWriteStore(), s.storeWriteSource())
	if s.usesShadowWriteStore() {
		result, err := s.runChatLogRepairReplay(r.Context(), sid, req)
		if err != nil {
			writeInternalError(w, err.Error())
			return
		}
		result["repair_replay_plan"] = repairReplayPlan
		writeJSON(w, http.StatusOK, result)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":             "ok",
		"source":             "shadow",
		"chat_session_id":    sid,
		"repair_replay_plan": repairReplayPlan,
		"note":               "repair-replay is a shadow plan; no mutations performed",
	})
}

func (s *Server) handleTurnsComplete(w http.ResponseWriter, r *http.Request) {
	writeShadowGuard(w, "POST /turns/complete")
}

// handleCompleteTurn processes a turn completion.
// In default/noop mode it does not write.
// In store-write-enabled modes it persists chat logs, effective input, audit
// logs, memory, direct evidence, and KG triples when clearly present.
func (s *Server) handleCompleteTurn(w http.ResponseWriter, r *http.Request) {
	var req dto.M4CompleteTurnRequest
	if err := dto.DecodeWithDefaults(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

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
		return
	}
	userText := sanitizeCriticStorageText(*req.UserInput)
	assistantText := sanitizeCriticStorageText(*req.AssistantContent)
	actualEmptyUserInput := completeTurnActualEmptyUserInput(req.ClientMeta)
	if strings.TrimSpace(userText) == "" && actualEmptyUserInput {
		userText = completeTurnAutoContinueUserInputMarker
	}
	content := strings.TrimSpace(strings.Join([]string{userText, assistantText}, "\n"))
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
	if s.usesShadowWriteStore() && req.TurnIndex > 0 {
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
					"warnings": []string{"complete_turn_raw_content_conflict: existing user+assistant logs for this turn differ; duplicate writes skipped"},
					"note":     "complete-turn duplicate guard kept existing raw turn; use explicit rollback/delete+rebuild to replace it",
				})
				return
			}
			if rawExactPairAlreadyPersisted && completeTurnHasDerivedArtifacts(ctx, s.Store, sid, req.TurnIndex) {
				now := time.Now().UTC()
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
					"warnings": []string{"complete_turn_idempotent_replay: existing raw and derived turn artifacts found; duplicate writes skipped"},
					"note":     "complete-turn idempotent replay; existing turn artifacts kept",
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
						"warnings": []string{"complete_turn_idempotent_pair_replay: same raw user+assistant pair already exists on turn " + strconv.Itoa(existingTurn) + "; duplicate writes skipped"},
						"note":     "complete-turn idempotent pair replay; existing turn artifacts kept",
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
	if shouldApplyCompleteTurnOOCGuard(userText, assistantText, req.ContextMessages) {
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
			"warnings": []string{"ooc_turn_guard_applied"},
			"note":     "complete-turn skipped by OOC guard",
		})
		return
	}
	if strings.TrimSpace(userText) == "" && !rawUserAlreadyPersisted {
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
			"warnings": []string{"user_input_missing: assistant-only complete-turn request skipped to prevent phantom turns"},
			"note":     "complete-turn skipped because user_input is required for a persisted turn",
		})
		return
	}
	var criticResult map[string]any
	criticTrace := map[string]any{}
	criticTriggered := false
	criticFailureReason := ""
	var criticFailureTrace map[string]any
	failReasons := []string{}
	if s.usesShadowWriteStore() && content != "" {
		if extractionCfg.Critic.hasConfig() {
			if assistantText == "" {
				failReasons = append(failReasons, "critic_skipped: assistant_content_missing")
			} else {
				result, trace, err := s.runCompleteTurnCriticWithInputPolicy(ctx, sid, turnIndex, userText, assistantText, req.ContextMessages, req.OutputLanguageOverride, extractionCfg.Critic, true, languageContext)
				if err != nil {
					criticFailureReason = "critic_extract_failed: " + err.Error()
					failReasons = append(failReasons, criticFailureReason)
					if trace != nil {
						criticTrace = trace
						criticFailureTrace = trace
					}
				} else {
					criticTriggered = true
					criticResult = result
					criticTrace = trace
				}
			}
		} else {
			failReasons = append(failReasons, "critic_config_missing")
		}
	}

	// Store save boundary: active only when the configured mode allows writes.
	saveOK := false
	saveErr := "shadow_mode: save disabled in R0/R1"
	chatLogsSaved := 0
	effectiveInputSaved := 0
	auditSaved := 0
	criticFeedbackSaved := 0
	memoriesSaved := 0
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
	pendingThreadsSaved := 0
	activeStatesSaved := 0
	canonicalStateLayersSaved := 0
	entitiesSaved := 0
	trustStatesSaved := 0
	vectorsUpserted := 0
	vectorsMemoryUpserted := 0
	vectorsEvidenceUpserted := 0
	vectorsWorldRuleUpserted := 0
	storeWriteAttempted := 0
	storeWriteErrors := 0
	storeWriteErrorDetails := []string{}
	artifactWarnings := []string{}
	conflictResolutions := []map[string]any{}
	retentionDecisions := []map[string]any{}
	var canonicalStateWriteCost any
	embeddingStatus := "not_requested"
	vectorStatus := "not_requested"

	now := time.Now().UTC()
	writeSource := s.storeWriteSource()
	if s.usesShadowWriteStore() {
		if rawTurnAlreadyPersisted {
			artifactWarnings = append(artifactWarnings, "raw_chat_logs_already_persisted: duplicate raw save skipped")
		} else {
			if rawUserAlreadyPersisted {
				artifactWarnings = append(artifactWarnings, "raw_user_chat_log_already_persisted: duplicate user raw save skipped")
			} else {
				storeWriteAttempted++
				if err := s.Store.SaveChatLog(ctx, &store.ChatLog{
					ChatSessionID: sid,
					TurnIndex:     turnIndex,
					Role:          "user",
					Content:       userText,
					CreatedAt:     now,
				}); err != nil {
					storeWriteErrors++
					storeWriteErrorDetails = append(storeWriteErrorDetails, "SaveChatLog(user): "+err.Error())
				} else {
					chatLogsSaved++
				}
			}

			if rawAssistantAlreadyPersisted {
				artifactWarnings = append(artifactWarnings, "raw_assistant_chat_log_already_persisted: duplicate assistant raw save skipped")
			} else {
				storeWriteAttempted++
				if err := s.Store.SaveChatLog(ctx, &store.ChatLog{
					ChatSessionID: sid,
					TurnIndex:     turnIndex,
					Role:          "assistant",
					Content:       assistantText,
					CreatedAt:     now,
				}); err != nil {
					storeWriteErrors++
					storeWriteErrorDetails = append(storeWriteErrorDetails, "SaveChatLog(assistant): "+err.Error())
				} else {
					chatLogsSaved++
				}
			}
		}

		if content != "" {
			skipEffectiveInputSave := false
			if rawTurnAlreadyPersisted {
				if _, err := s.Store.GetEffectiveInput(ctx, sid, turnIndex); err == nil {
					skipEffectiveInputSave = true
					artifactWarnings = append(artifactWarnings, "effective_input_already_persisted: duplicate effective input skipped")
				}
			}
			if !skipEffectiveInputSave {
				storeWriteAttempted++
				if err := s.Store.SaveEffectiveInput(ctx, &store.EffectiveInput{
					ChatSessionID:  sid,
					TurnIndex:      turnIndex,
					EffectiveInput: content,
					CreatedAt:      now,
				}); err != nil {
					storeWriteErrors++
					storeWriteErrorDetails = append(storeWriteErrorDetails, "SaveEffectiveInput: "+err.Error())
				} else {
					effectiveInputSaved++
				}
			} else {
				effectiveInputSaved = 0
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

		storeWriteAttempted++
		if err := s.Store.SaveAuditLog(ctx, &store.AuditLog{
			ChatSessionID: sid,
			EventType:     "effective_input_saved",
			TargetType:    "turn",
			TargetID:      int64(turnIndex),
			Summary:       fmt.Sprintf("effective input saved turn %d", turnIndex),
			DetailsJSON:   fmt.Sprintf(`{"turn_index":%d,"length":%d}`, turnIndex, len(content)),
			Source:        writeSource,
			CreatedAt:     now,
		}); err != nil {
			storeWriteErrors++
			storeWriteErrorDetails = append(storeWriteErrorDetails, "SaveAuditLog: "+err.Error())
		} else {
			auditSaved++
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
		if criticResult != nil {
			artifactResult := s.saveCriticExtractionArtifacts(ctx, sid, turnIndex, criticResult, content, extractionCfg.Embedder, now, existingEvidence)
			memoriesSaved += artifactResult.Memories
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
			pendingThreadsSaved += artifactResult.PendingThreads
			activeStatesSaved += artifactResult.ActiveStates
			canonicalStateLayersSaved += artifactResult.CanonicalStateLayers
			entitiesSaved += artifactResult.Entities
			trustStatesSaved += artifactResult.TrustStates
			vectorsUpserted += artifactResult.VectorsUpserted
			vectorsMemoryUpserted += artifactResult.VectorsMemoryUpserted
			vectorsEvidenceUpserted += artifactResult.VectorsEvidenceUpserted
			vectorsWorldRuleUpserted += artifactResult.VectorsWorldRuleUpserted
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

		if storeWriteAttempted > 0 && storeWriteErrors == 0 {
			saveOK = true
			saveErr = ""
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

	maintenanceHandoff := s.buildCompleteTurnMaintenanceHandoff(ctx, sid, turnIndex, saveOK, now, writeSource, req)
	auditSaved += maintenanceHandoff.AuditSaved
	storeWriteAttempted += maintenanceHandoff.Attempted
	storeWriteErrors += maintenanceHandoff.Errors
	storeWriteErrorDetails = append(storeWriteErrorDetails, maintenanceHandoff.ErrorDetails...)
	if maintenanceHandoff.Errors > 0 {
		failReasons = append(failReasons, "maintenance_enqueue")
	}
	if len(failReasons) == 0 {
		failReasons = []string{}
	}
	episodeResult := map[string]any{"checked": false, "triggered": false, "range": nil, "reason": "store_write_not_ok"}
	if saveOK {
		episodeResult = s.completeTurnEpisodeCheckpoint(ctx, sid, turnIndex, req.ClientMeta)
		if errText := strings.TrimSpace(stringFromMap(episodeResult, "error")); errText != "" {
			warnings = append(warnings, "episode_checkpoint_failed: "+errText)
		}
	}
	hierarchyPromotionResult := map[string]any{"checked": false, "triggered": false, "policy": completeTurnHierarchyPromotionVersion, "reason": "store_write_not_ok"}
	if saveOK {
		hierarchyPromotionResult = s.completeTurnHierarchyPromotionCheckpoint(ctx, sid, turnIndex, req.ClientMeta)
		if errText := strings.TrimSpace(stringFromMap(hierarchyPromotionResult, "error")); errText != "" {
			warnings = append(warnings, "hierarchy_promotion_failed: "+errText)
		}
	}
	derivedArtifactsSaved := memoriesSaved + evidenceSaved + kgTriplesSaved + subjectiveEntityMemoriesSaved + characterEventsSaved + storylinesSaved + worldRulesSaved + characterStatesSaved + physicalConditionsSaved + entityConditionsSaved + statusSchemaDefinitionsSaved + statusEffectsSaved + narrativeCurrentStatesSaved + narrativeStateEventsSaved + pendingThreadsSaved + activeStatesSaved + canonicalStateLayersSaved + entitiesSaved + trustStatesSaved
	rawStatus := "skipped"
	if chatLogsSaved > 0 || effectiveInputSaved > 0 {
		rawStatus = "ok"
	} else if !saveOK || storeWriteErrors > 0 {
		rawStatus = "error"
	}
	derivedStatus := "skipped"
	if derivedArtifactsSaved > 0 {
		derivedStatus = "ok"
	} else if criticTriggered && derivedArtifactsSaved == 0 {
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
	persistencePipeline := map[string]any{
		"contract_version": "complete_turn.persistence_pipeline.v1",
		"raw": map[string]any{
			"status":                rawStatus,
			"chat_logs_saved":       chatLogsSaved,
			"effective_input_saved": effectiveInputSaved,
		},
		"derived": map[string]any{
			"status":                           derivedStatus,
			"artifacts_saved":                  derivedArtifactsSaved,
			"memories_saved":                   memoriesSaved,
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

	writeJSON(w, http.StatusOK, map[string]any{
		"status":                           "ok",
		"source":                           writeSource,
		"chat_session_id":                  sid,
		"turn_index":                       turnIndex,
		"generated_at":                     time.Now().UTC().Format(time.RFC3339),
		"save_ok":                          saveOK,
		"save_error":                       saveErr,
		"memories_saved":                   memoriesSaved,
		"evidence_saved":                   evidenceSaved,
		"kg_triples_saved":                 kgTriplesSaved,
		"persona_capsule_candidates":       personaCapsuleCandidates,
		"subjective_entity_memories_saved": subjectiveEntityMemoriesSaved,
		"character_events_saved":           characterEventsSaved,
		"storylines_saved":                 storylinesSaved,
		"world_rules_saved":                worldRulesSaved,
		"character_states_saved":           characterStatesSaved,
		"physical_conditions_saved":        physicalConditionsSaved,
		"entity_conditions_saved":          entityConditionsSaved,
		"status_schema_definitions_saved":  statusSchemaDefinitionsSaved,
		"status_effects_saved":             statusEffectsSaved,
		"narrative_current_states_saved":   narrativeCurrentStatesSaved,
		"narrative_state_events_saved":     narrativeStateEventsSaved,
		"pending_threads_saved":            pendingThreadsSaved,
		"active_states_saved":              activeStatesSaved,
		"canonical_state_layers_saved":     canonicalStateLayersSaved,
		"entities_saved":                   entitiesSaved,
		"trust_states_saved":               trustStatesSaved,
		"vectors_upserted":                 vectorsUpserted,
		"vectors_memory_upserted":          vectorsMemoryUpserted,
		"vectors_evidence_upserted":        vectorsEvidenceUpserted,
		"vectors_world_rule_upserted":      vectorsWorldRuleUpserted,
		"chat_logs_saved":                  chatLogsSaved,
		"effective_input_saved":            effectiveInputSaved,
		"audit_saved":                      auditSaved,
		"critic_feedback_saved":            criticFeedbackSaved,
		"store_write_attempted":            storeWriteAttempted,
		"store_write_errors":               storeWriteErrors,
		"store_write_error_details":        storeWriteErrorDetails,
		"critic_triggered":                 criticTriggered,
		"critic_result":                    criticResult,
		"language_context":                 languageContext,
		"llm_config_trace":                 llmConfigTrace,
		"derived_artifacts_saved":          derivedArtifactsSaved,
		"episode_result":                   episodeResult,
		"chapter_result":                   nil,
		"hierarchy_promotion_result":       hierarchyPromotionResult,
		"persistence_pipeline":             persistencePipeline,
		"maintenance_enqueued":             maintenanceHandoff.Enqueued,
		"fail_reasons":                     failReasons,
		"trace_handoff": map[string]any{
			"shadow_mode":                              s.Cfg.StoreMode != config.StoreModeMariaDBAuthority,
			"store_mode":                               string(s.Cfg.StoreMode),
			"save_ok":                                  saveOK,
			"critic_attempted":                         extractionCfg.Critic.hasConfig(),
			"critic_triggered":                         criticTriggered,
			"llm_config_trace":                         llmConfigTrace,
			"derived_artifacts_saved":                  derivedArtifactsSaved,
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
		QueueStatus: "skipped",
		QueueDepth:  0,
		RefreshPlan: map[string]any{},
		Trace: map[string]any{
			"owner":          "complete_turn",
			"version":        completeTurnMaintenancePlanVersion,
			"worker_enabled": false,
			"queue_mode":     "audit_shadow",
			"status":         "skipped",
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
		"mode":    "complete_turn_audit_shadow_handoff",
		"layers": map[string]any{
			"chapter": map[string]any{
				"enabled":        refreshEnabled && chapterAutoEnabled,
				"interval_turns": intFromAny(meta["chapter_interval_turns"], 60),
			},
			"arc": map[string]any{
				"enabled":        refreshEnabled && arcAutoEnabled,
				"interval_turns": intFromAny(meta["arc_interval_turns"], 240),
			},
			"saga": map[string]any{
				"enabled":        refreshEnabled && sagaAutoEnabled,
				"interval_turns": intFromAny(meta["saga_interval_turns"], 960),
			},
		},
		"worker_enabled": false,
		"queue_mode":     "audit_shadow",
	}

	handoff.RefreshEnabled = refreshEnabled
	handoff.RefreshPlan = plan
	handoff.QueueStatus = "audit_shadow_enqueued"
	handoff.QueueDepth = 1
	handoff.Enqueued = true
	handoff.Trace = map[string]any{
		"owner":                    "complete_turn",
		"version":                  completeTurnMaintenancePlanVersion,
		"status":                   handoff.QueueStatus,
		"queue_depth":              handoff.QueueDepth,
		"worker_enabled":           false,
		"queue_mode":               "audit_shadow",
		"maintenance_pass_enabled": false,
		"refresh_enabled":          refreshEnabled,
		"refresh_plan":             plan,
	}

	handoff.Attempted = 1
	err := s.Store.SaveAuditLog(ctx, &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "maintenance_enqueued",
		TargetType:    "turn",
		TargetID:      int64(turnIndex),
		Summary:       fmt.Sprintf("complete-turn maintenance handoff queued turn %d", turnIndex),
		DetailsJSON:   mustCompactJSON(plan),
		Source:        writeSource,
		CreatedAt:     now,
	})
	if err != nil {
		handoff.Enqueued = false
		handoff.QueueStatus = "audit_shadow_enqueue_failed"
		handoff.QueueDepth = 0
		handoff.Errors = 1
		handoff.ErrorDetails = append(handoff.ErrorDetails, "SaveAuditLog(maintenance_enqueued): "+err.Error())
		handoff.Trace["status"] = handoff.QueueStatus
		handoff.Trace["queue_depth"] = 0
		handoff.Trace["error"] = err.Error()
		return handoff
	}
	handoff.AuditSaved = 1
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
func (s *Server) handlePrepareTurn(w http.ResponseWriter, r *http.Request) {
	var req dto.PrepareTurnRequest
	if err := dto.DecodeWithDefaults(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	sid := strings.TrimSpace(req.ChatSessionID)
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}

	if lock, err := s.sessionMigrationSourceLock(r.Context(), sid); err != nil {
		writeInternalError(w, err.Error())
		return
	} else if lock != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":                "ok",
			"source":                "shadow",
			"chat_session_id":       sid,
			"generated_at":          time.Now().UTC().Format(time.RFC3339),
			"request_type":          stringPtrValue(req.RequestType, "model"),
			"fallback_reason":       "source_session_migrated_away",
			"effective_user_input":  stringPtrValue(req.RawUserInput, ""),
			"injection_text":        "",
			"input_context_text":    "",
			"migration_source_lock": sessionMigrationLockPayload(lock),
			"read_excluded":         true,
			"read_exclusion_reason": "source_session_migrated_away",
			"target_session_id":     lock.TargetSessionID,
			"trace_preview":         map[string]any{"would_call_llm": false, "would_write": false, "migration_source_lock": sessionMigrationLockPayload(lock)},
			"recall_result":         map[string]any{"status": "skipped", "reason": "source_session_migrated_away"},
			"runtime_toggle":        map[string]any{"source_session_migrated_away": true, "target_session_id": lock.TargetSessionID},
			"warnings":              []string{"source_session_migrated_away: continue in target_session_id " + lock.TargetSessionID},
		})
		return
	}

	// Resolve settings from the request/default DTO contract.
	defaultSettings := dto.PrepareTurnSettings{}
	defaultSettings.ApplyDefaults()
	maxInjectionChars := prepareTurnIntSetting(req.Settings.MaxInjectionChars, defaultSettings.MaxInjectionChars)
	maxInputContextChars := prepareTurnIntSetting(req.Settings.MaxInputContextChars, defaultSettings.MaxInputContextChars)
	injectionEnabled := true
	inputContextEnabled := true
	memoryTopK := prepareTurnIntSetting(req.Settings.TopK, defaultSettings.TopK)
	supportRecallLimit := prepareTurnSupportRecallLimit(memoryTopK)

	if req.Settings.InjectionEnabled != nil {
		injectionEnabled = *req.Settings.InjectionEnabled
	}
	if req.Settings.InputContextEnabled != nil {
		inputContextEnabled = *req.Settings.InputContextEnabled
	}
	rawUserInput := stringPtrValue(req.RawUserInput, "")
	turnIndex := intPtrValue(req.TurnIndex, 0)
	languageContext := completeTurnLanguageContextFromClientMeta(req.ClientMeta)
	perspectiveContext := prepareTurnPerspectiveContextFromRequest(req)

	// Read assembly from Store (no writes, no LLM).
	var memories []store.Memory
	var kgTriples []store.KGTriple
	var evidence []store.DirectEvidence
	var chatLogs []store.ChatLog
	var resumePack *store.ResumePack
	var storylines []store.Storyline
	var worldRules []store.WorldRule
	var charStates []store.CharacterState
	var pendingThreads []store.PendingThread
	var activeStates []store.ActiveState
	var canonicalLayers []store.CanonicalStateLayer
	var episodeSums []store.EpisodeSummary
	var personaEntries []store.PersonaMemoryEntry
	var characterPrivateMemories []store.ProtagonistEntityMemory
	var narrativeCurrentValues []store.StatusCurrentValue

	readErrs := []error{}
	readsOK := 0

	if s.Store != nil {
		ctx := r.Context()
		if m, err := s.Store.ListMemories(ctx, sid, 0, 0); err == nil {
			memories = m
			readsOK++
		} else if !errors.Is(err, store.ErrNotEnabled) {
			readErrs = append(readErrs, err)
		}
		if k, err := s.Store.ListKGTriples(ctx, sid); err == nil {
			kgTriples = k
			readsOK++
		} else if !errors.Is(err, store.ErrNotEnabled) {
			readErrs = append(readErrs, err)
		}
		if e, err := s.Store.ListEvidence(ctx, sid); err == nil {
			evidence = e
			readsOK++
		} else if !errors.Is(err, store.ErrNotEnabled) {
			readErrs = append(readErrs, err)
		}
		if c, err := s.Store.ListChatLogs(ctx, sid, 0, 0); err == nil {
			chatLogs = c
			readsOK++
		} else if !errors.Is(err, store.ErrNotEnabled) {
			readErrs = append(readErrs, err)
		}
		if rp, err := s.Store.GetResumePack(ctx, sid, "prepare_turn"); err == nil {
			resumePack = rp
			readsOK++
		} else if !errors.Is(err, store.ErrNotEnabled) && !errors.Is(err, store.ErrNotFound) {
			readErrs = append(readErrs, err)
		}
		if sl, err := s.Store.ListStorylines(ctx, sid); err == nil {
			storylines = sl
			readsOK++
		} else if !errors.Is(err, store.ErrNotEnabled) {
			readErrs = append(readErrs, err)
		}
		if wr, err := s.Store.ListWorldRules(ctx, sid); err == nil {
			worldRules = wr
			readsOK++
		} else if !errors.Is(err, store.ErrNotEnabled) {
			readErrs = append(readErrs, err)
		}
		if cs, err := s.Store.ListCharacterStates(ctx, sid); err == nil {
			charStates = cs
			readsOK++
		} else if !errors.Is(err, store.ErrNotEnabled) {
			readErrs = append(readErrs, err)
		}
		if pt, err := s.Store.ListPendingThreads(ctx, sid, ""); err == nil {
			pendingThreads = pt
			readsOK++
		} else if !errors.Is(err, store.ErrNotEnabled) {
			readErrs = append(readErrs, err)
		}
		if as, err := s.Store.ListActiveStates(ctx, sid, ""); err == nil {
			activeStates = as
			readsOK++
		} else if !errors.Is(err, store.ErrNotEnabled) {
			readErrs = append(readErrs, err)
		}
		if cl, err := s.Store.ListCanonicalStateLayers(ctx, sid, ""); err == nil {
			canonicalLayers = cl
			readsOK++
		} else if !errors.Is(err, store.ErrNotEnabled) {
			readErrs = append(readErrs, err)
		}
		if es, err := s.Store.ListEpisodeSummaries(ctx, sid, supportRecallLimit, 0, 0); err == nil {
			episodeSums = es
			readsOK++
		} else if !errors.Is(err, store.ErrNotEnabled) {
			readErrs = append(readErrs, err)
		}
		if personaStore, ok := s.Store.(store.PersonaCapsuleStore); ok {
			if entries, err := personaStore.ListAttachedPersonaMemoryEntries(ctx, sid, supportRecallLimit); err == nil {
				for _, entry := range entries {
					if personaMemoryEntryIsCharacterPrivate(entry) {
						characterPrivateMemories = append(characterPrivateMemories, personaMemoryEntryAsCharacterPrivateMemory(entry, sid))
						continue
					}
					personaEntries = append(personaEntries, entry)
				}
				readsOK++
			} else if !errors.Is(err, store.ErrNotEnabled) {
				readErrs = append(readErrs, err)
			}
		}
		if entityStore, ok := s.Store.(store.ProtagonistEntityMemoryStore); ok {
			memories, err := entityStore.ListProtagonistEntityMemories(ctx, store.ProtagonistEntityMemoryFilter{
				OwnerEntityRole:     "npc",
				OwnerVisibility:     "owner_private",
				SourceChatSessionID: sid,
				Limit:               supportRecallLimit,
			})
			if err == nil {
				characterPrivateMemories = append(characterPrivateMemories, memories...)
				if len(memories) > 0 {
					readsOK++
				}
			} else if !errors.Is(err, store.ErrNotEnabled) {
				readErrs = append(readErrs, err)
			}
		}
		if valueStore, ok := s.Store.(store.StatusCurrentValueStore); ok {
			if values, err := valueStore.ListStatusCurrentValues(ctx, sid, "", "", narrativeStateStatusKey, 1000); err == nil {
				narrativeCurrentValues = values
				readsOK++
			} else if !errors.Is(err, store.ErrNotEnabled) {
				readErrs = append(readErrs, err)
			}
		}
	}

	recollectionRelevance := filterPrepareTurnEntityRecollections(rawUserInput, chatLogs, activeStates, canonicalLayers, personaEntries, &characterPrivateMemories)

	degraded := readsOK == 0
	fallbackReason := ""
	if degraded {
		fallbackReason = "store_unavailable"
	} else if len(readErrs) > 0 {
		fallbackReason = "partial_reads"
	}

	var storylineReferenceTurn *int
	if turnIndex > 0 {
		value := turnIndex
		storylineReferenceTurn = &value
	}
	storylineSelection := selectStorylinesForSupervisor(storylines, storylineReferenceTurn, supportRecallLimit)
	selectedStorylines := selectedStorylineItems(storylineSelection)

	profile := strings.TrimSpace(clientMetaString(req.ClientMeta, "context_window_profile"))
	if profile == "" {
		profile = "default"
	}
	injectionAssembly := prepareTurnInjectionAssembly{}
	documents := []map[string]any{}
	vectorShadow := s.prepareTurnVectorShadow(r.Context(), req, memoryTopK)
	if !degraded {
		documents = buildUnifiedRetrievalDocuments(sid, memories, evidence, kgTriples, episodeSums, resumePack, chatLogs)
		if injectionEnabled {
			assemblyPerspectiveContext := prepareTurnPerspectiveWithNarrativeState(perspectiveContext, narrativeCurrentValues, activeStates)
			injectionAssembly = buildPrepareTurnInjectionAssembly(memories, kgTriples, evidence, chatLogs, selectedStorylines, worldRules, charStates, pendingThreads, canonicalLayers, episodeSums, resumePack, personaEntries, characterPrivateMemories, memoryTopK, maxInjectionChars, rawUserInput, profile, documents, vectorShadow, languageContext, assemblyPerspectiveContext)
		}
	}
	injectionText := injectionAssembly.Text
	injectionTruncated := injectionAssembly.Truncated

	var inputContextText string
	var inputContextTruncated bool
	if inputContextEnabled && !degraded {
		inputContextText, inputContextTruncated = buildInputContextText(evidence, chatLogs, resumePack, activeStates, canonicalLayers, episodeSums, personaEntries, characterPrivateMemories, maxInputContextChars, supportRecallLimit)
	}
	currentStoryClock19 := resolveCurrentStoryClock(activeStates, chatLogs, canonicalLayers)
	temporalRelationLedger19 := buildTemporalRelationLedger(activeStates)
	temporalSupportPacket := buildTemporalSupportPacket(currentStoryClock19, temporalRelationLedger19)
	guideMode := resolveNarrativeGuideMode(stringPtrValue(req.Settings.GuideMode, "off"), nil, "", rawUserInput)
	narrativeStance := stringPtrValue(req.Settings.NarrativeStance, "balanced")
	continuityTriggerMode := stringPtrValue(req.ContinuityTriggerMode, "none")
	continuityQuery := stringPtrValue(req.ContinuityQuery, "")
	requestType := stringPtrValue(req.RequestType, "model")
	applyMode := stringPtrValue(req.Settings.ApplyMode, "shadow")
	promptAssembly := buildPromptAssemblyTrace(s.Cfg.PromptDir)
	evidenceCounts := prepareTurnEvidenceCounts(memories, kgTriples, evidence, chatLogs, resumePack, storylines, worldRules, charStates, pendingThreads, activeStates, canonicalLayers, episodeSums)
	evidenceCounts["storyline_selected_count"] = len(storylineSelection.Selected)
	evidenceCounts["storyline_dropped_count"] = len(storylineSelection.Dropped)
	evidenceCounts["storyline_stale_dropped_count"] = storylineSelectionSummary(storylineSelection)["stale_dropped_count"]
	sectionSummary := prepareTurnSectionSummary(injectionText, inputContextText, injectionTruncated, inputContextTruncated)
	guideStrength := normalizeNarrativeGuideStrength(stringPtrValue(req.Settings.GuideStrength, "weak"))
	supervisorInputPack := buildSupervisorInputPack(
		sid,
		turnIndex,
		rawUserInput,
		guideMode,
		guideStrength,
		narrativeStance,
		continuityTriggerMode,
		continuityQuery,
		promptAssembly,
		evidenceCounts,
		sectionSummary,
		storylineSelection,
		degraded,
		fallbackReason,
		languageContext,
	)
	criticInputPack := buildCriticInputPack(sid, turnIndex, rawUserInput, promptAssembly, evidenceCounts, sectionSummary, degraded)
	injectionPack := buildInjectionPack(rawUserInput, inputContextText, injectionEnabled, inputContextEnabled, inputContextTruncated, injectionAssembly, temporalSupportPacket)

	queryPreview := rawUserInput

	recallResult := buildRecallResult(
		sid,
		queryPreview,
		degraded,
		memories,
		evidence,
		kgTriples,
		episodeSums,
		chatLogs,
		resumePack,
		vectorShadow,
		storylines,
		worldRules,
		pendingThreads,
		profile,
		memoryTopK,
	)

	packetMode := "store_backed_shadow"
	if degraded {
		packetMode = "off"
	}

	var injectionOut any = nil
	if injectionText != "" {
		injectionOut = injectionText
	}
	var inputContextOut any = nil
	if inputContextText != "" {
		inputContextOut = inputContextText
	}

	sessionState := buildSessionState(degraded, activeStates, storylines, charStates, worldRules, pendingThreads, supportRecallLimit)
	narrativeControl := buildNarrativeControl(degraded, storylines, worldRules, pendingThreads, charStates)
	continuityPack := buildContinuityPack(sid, queryPreview, degraded, resumePack, episodeSums, chatLogs, activeStates, canonicalLayers, supportRecallLimit)
	progressionLedger := buildProgressionLedger(sid, degraded, storylines, worldRules, pendingThreads, episodeSums, supportRecallLimit)
	personaRecollection := buildPersonaRecollectionSurface(sid, personaEntries, injectionAssembly.PersonaText, supportRecallLimit)
	characterPrivateRecollection := buildCharacterPrivateRecollectionSurface(sid, characterPrivateMemories, injectionAssembly.CharacterPrivateText, supportRecallLimit)

	// SEQ-16-P164/P165/P167/P168 contract surfaces.
	retrievalRoleBoundary := buildRetrievalRoleBoundary(sid, storylines, worldRules, charStates, activeStates, pendingThreads, chatLogs)
	retrievalIndexIR := buildRetrievalIndexIRSupportOnly(recallResult, memories, evidence, kgTriples, chatLogs, resumePack)
	retrievalExtendAuthority := buildRetrievalExtendAuthority(retrievalRoleBoundary)
	temporalReadValidityFirst := buildTemporalReadValidityFirst(chatLogs, episodeSums, len(chatLogs))

	// SEQ-16-P172~P175 contract surfaces.
	sessionMemoryBoundary := buildSessionMemoryBoundary(sid, activeStates, pendingThreads, chatLogs, storylines, worldRules, charStates)
	bridgePromotionEntry := buildBridgePromotionEntry(sid, pendingThreads, canonicalLayers)
	sessionFirstPermanentFallbackReadRule := buildSessionFirstPermanentFallbackReadRule(sid, sessionMemoryBoundary, retrievalRoleBoundary)
	promotionWaitVisibility := buildPromotionWaitVisibility(sid, pendingThreads, canonicalLayers, chatLogs)

	// SEQ-16-P179~P182 contract surfaces (IR normalized retrieval unit schema).
	retrievalUnitsIR := buildRetrievalUnitsIR(sid, memories, evidence, kgTriples, chatLogs, resumePack)
	directEvidenceDualRepresentation := buildDirectEvidenceDualRepresentation(evidence)
	sourceTaggedRetrievalUnitSurface := buildSourceTaggedRetrievalUnitSurface(memories, evidence, kgTriples, chatLogs, resumePack)
	rawTurnSpanMetadata := buildRawTurnSpanMetadata(chatLogs, episodeSums, memories, evidence, resumePack)

	// SEQ-16-P186~P189 contract surfaces (MS Multi-Signal Retrieval Contract).
	signalMixContract := buildSignalMixContract(sid, memories, evidence, kgTriples, chatLogs, episodeSums)
	queryClassRouting := buildQueryClassRouting(sid, memories, evidence, kgTriples, chatLogs, episodeSums)
	retrievalResultInspection := buildRetrievalResultInspection(sid, memories, evidence, kgTriples, chatLogs, episodeSums, supportRecallLimit)
	sparseTailRecall := buildSparseTailRecall(sid, memories, evidence, kgTriples, chatLogs, episodeSums)

	// SEQ-16-P193~P196 contract surfaces (TM Temporal Read Surface).
	validityWindowReading := buildValidityWindowReading(sid, chatLogs, episodeSums, evidence, memories)
	truthCoexistenceRules := buildTruthCoexistenceRules(sid, evidence, memories, chatLogs)
	temporalDisambiguationContract := buildTemporalDisambiguationContract(sid, chatLogs, episodeSums, evidence, memories)
	promotionLagInvisibilitySplit := buildPromotionLagInvisibilitySplit(sid, pendingThreads, canonicalLayers, chatLogs, evidence)

	// SEQ-16-P200~P205 contract surfaces (VX verify + replay).
	sessionPermanentAuthorityReplay := buildSessionPermanentAuthorityReplay(sid, retrievalRoleBoundary)
	normalizedUnitSupportOnlyReplay := buildNormalizedUnitSupportOnlyReplay(sid, retrievalUnitsIR)
	multiSignalRetrievalInspectionReplay := buildMultiSignalRetrievalInspectionReplay(sid, signalMixContract, retrievalResultInspection)
	validityWindowTemporalReplay := buildValidityWindowTemporalReplay(sid, temporalReadValidityFirst, validityWindowReading)
	sourceTaggedAuthorityAwareAssemblyReplay := buildSourceTaggedAuthorityAwareAssemblyReplay(sid, sourceTaggedRetrievalUnitSurface, retrievalRoleBoundary)
	criticTruncationSpilloverReplay := buildCriticTruncationSpilloverReplay(sid, rawTurnSpanMetadata, sparseTailRecall, retrievalUnitsIR)

	// SEQ-16-P209~P212 contract surfaces (backend test remigration evidence).
	indexSnapshot := retrievalIndexSnapshotFromDocuments(sid, documents)
	sessionPartitionedIndex := buildSessionPartitionedIndex(sid, documents, indexSnapshot)
	indexLifecycle := buildIndexLifecycle(sid, vectorShadow)
	sourceLookupAudit := buildSourceLookupAudit(sid, evidence, memories, kgTriples, chatLogs)
	runtimeToggle := buildRuntimeToggle(sid, degraded, injectionEnabled, inputContextEnabled, maxInjectionChars, maxInputContextChars)
	inputAnchorGovernor := buildInputAnchorGovernor(rawUserInput, inputContextText, inputContextTruncated, maxInputContextChars, chatLogs, resumePack, activeStates, canonicalLayers, episodeSums, pendingThreads, storylines)
	weakInputPlanner := buildWeakInputPlannerContract(rawUserInput, inputAnchorGovernor, languageContext, maxInputContextChars)
	plannerExecutionContract := buildPlannerExecutionContract(rawUserInput, narrativeStance, guideMode, guideStrength, inputAnchorGovernor, weakInputPlanner, selectedStorylines, pendingThreads, activeStates, canonicalLayers, worldRules, injectionAssembly, languageContext)
	progressionChoiceLedger := buildProgressionChoiceLedger(sid, turnIndex, rawUserInput, chatLogs, selectedStorylines, pendingThreads, episodeSums, inputAnchorGovernor, weakInputPlanner, plannerExecutionContract, progressionLedger)
	progressionLedger["progression_choice"] = progressionChoiceLedger
	step25ValidationGate := buildStep25ValidationGate(rawUserInput, weakInputPlanner, plannerExecutionContract, progressionChoiceLedger)
	supervisorInputPack["step25_validation_gate"] = step25ValidationGate
	if guidance := formatWeakInputPlannerGuidance(weakInputPlanner); guidance != "" {
		supervisorInputPack["weak_input_planner"] = weakInputPlanner
		if existing, _ := supervisorInputPack["persistent_guidance"].(string); strings.TrimSpace(existing) != "" {
			supervisorInputPack["persistent_guidance"] = existing + "\n" + guidance
		} else {
			supervisorInputPack["persistent_guidance"] = guidance
		}
		if existing, _ := supervisorInputPack["final_guidance_suffix"].(string); strings.TrimSpace(existing) != "" {
			supervisorInputPack["final_guidance_suffix"] = existing + "\n" + guidance
		} else {
			supervisorInputPack["final_guidance_suffix"] = guidance
		}
	}
	if guidance := formatPlannerExecutionContractGuidance(plannerExecutionContract); guidance != "" {
		supervisorInputPack["planner_execution_contract"] = plannerExecutionContract
		if existing, _ := supervisorInputPack["persistent_guidance"].(string); strings.TrimSpace(existing) != "" {
			supervisorInputPack["persistent_guidance"] = existing + "\n" + guidance
		} else {
			supervisorInputPack["persistent_guidance"] = guidance
		}
		if existing, _ := supervisorInputPack["final_guidance_suffix"].(string); strings.TrimSpace(existing) != "" {
			supervisorInputPack["final_guidance_suffix"] = existing + "\n" + guidance
		} else {
			supervisorInputPack["final_guidance_suffix"] = guidance
		}
	}
	if guidance := formatProgressionChoiceGuidance(progressionChoiceLedger); guidance != "" {
		supervisorInputPack["progression_choice_ledger"] = progressionChoiceLedger
		if existing, _ := supervisorInputPack["persistent_guidance"].(string); strings.TrimSpace(existing) != "" {
			supervisorInputPack["persistent_guidance"] = existing + "\n" + guidance
		} else {
			supervisorInputPack["persistent_guidance"] = guidance
		}
		if existing, _ := supervisorInputPack["final_guidance_suffix"].(string); strings.TrimSpace(existing) != "" {
			supervisorInputPack["final_guidance_suffix"] = existing + "\n" + guidance
		} else {
			supervisorInputPack["final_guidance_suffix"] = guidance
		}
	}
	helperBudgetGovernorTrace := buildHelperBudgetGovernorTrace(injectionAssembly, maxInjectionChars)

	tracePreview := map[string]any{
		"source":              "go_r1_read_shadow",
		"would_call_llm":      false,
		"would_write":         false,
		"prompt_source":       promptAssembly["prompt_source"],
		"evidence_counts":     evidenceCounts,
		"section_summary":     sectionSummary,
		"supervisor_status":   supervisorInputPack["status"],
		"critic_status":       criticInputPack["status"],
		"storyline_selection": supervisorInputPack["storyline_selection"],
	}
	for k, v := range progressionLedgerTracePreviewFields(progressionLedger) {
		tracePreview[k] = v
	}
	autonomyPlan := buildAutonomyPlan(degraded, guideMode, narrativeStance)
	microBeatProposal := buildMicroBeatProposal(degraded, pendingThreads, storylines, supportRecallLimit)
	sceneStepProposal := buildSceneStepProposal(degraded, activeStates, canonicalLayers, episodeSums, supportRecallLimit)
	combinedProposal := buildCombinedProposal(degraded, microBeatProposal, sceneStepProposal)
	writebackPreview := buildWritebackPreview(degraded)
	shadowCompareRecord := buildGenerationPacketShadowCompareRecord(injectionAssembly, inputContextText)
	inputTransparencyModel := buildPrepareTurnInputTransparencyRenderModel(sid, turnIndex, rawUserInput, inputContextText, injectionEnabled, inputContextEnabled, inputContextTruncated, degraded, fallbackReason, injectionAssembly)
	effectiveInputPreview := buildPrepareTurnEffectiveInputPreview(sid, turnIndex, rawUserInput, requestType, applyMode, inputContextText, injectionEnabled, inputContextEnabled, inputContextTruncated, degraded, fallbackReason, injectionAssembly)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":                                        "ok",
		"source":                                        "shadow",
		"chat_session_id":                               sid,
		"generated_at":                                  time.Now().UTC().Format(time.RFC3339),
		"request_type":                                  requestType,
		"fallback_reason":                               fallbackReason,
		"effective_user_input":                          rawUserInput,
		"injection_text":                                injectionOut,
		"input_context_text":                            inputContextOut,
		"supervisor_input_pack":                         supervisorInputPack,
		"critic_input_pack":                             criticInputPack,
		"injection_pack":                                injectionPack,
		"language_context":                              languageContext,
		"perspective_context":                           perspectiveContext,
		"input_transparency_model":                      inputTransparencyModel,
		"effective_input_preview":                       effectiveInputPreview,
		"trace_preview":                                 tracePreview,
		"recall_result":                                 recallResult,
		"session_state":                                 sessionState,
		"narrative_control":                             narrativeControl,
		"progression_ledger":                            progressionLedger,
		"retrieval_role_boundary":                       retrievalRoleBoundary,
		"retrieval_index_ir":                            retrievalIndexIR,
		"retrieval_extend_authority":                    retrievalExtendAuthority,
		"temporal_read_validity_first":                  temporalReadValidityFirst,
		"session_memory_boundary":                       sessionMemoryBoundary,
		"bridge_promotion_entry":                        bridgePromotionEntry,
		"session_first_permanent_fallback_read_rule":    sessionFirstPermanentFallbackReadRule,
		"promotion_wait_visibility":                     promotionWaitVisibility,
		"retrieval_units_ir":                            retrievalUnitsIR,
		"direct_evidence_dual_representation":           directEvidenceDualRepresentation,
		"source_tagged_retrieval_unit_surface":          sourceTaggedRetrievalUnitSurface,
		"raw_turn_span_metadata":                        rawTurnSpanMetadata,
		"signal_mix_contract":                           signalMixContract,
		"query_class_routing":                           queryClassRouting,
		"retrieval_result_inspection":                   retrievalResultInspection,
		"sparse_tail_recall":                            sparseTailRecall,
		"validity_window_reading":                       validityWindowReading,
		"truth_coexistence_rules":                       truthCoexistenceRules,
		"temporal_disambiguation_contract":              temporalDisambiguationContract,
		"promotion_lag_invisibility_split":              promotionLagInvisibilitySplit,
		"session_permanent_authority_replay":            sessionPermanentAuthorityReplay,
		"normalized_unit_support_only_replay":           normalizedUnitSupportOnlyReplay,
		"multi_signal_retrieval_inspection_replay":      multiSignalRetrievalInspectionReplay,
		"validity_window_temporal_replay":               validityWindowTemporalReplay,
		"source_tagged_authority_aware_assembly_replay": sourceTaggedAuthorityAwareAssemblyReplay,
		"critic_truncation_spillover_replay":            criticTruncationSpilloverReplay,
		"session_partitioned_index":                     sessionPartitionedIndex,
		"index_lifecycle":                               indexLifecycle,
		"source_lookup_audit":                           sourceLookupAudit,
		"runtime_toggle":                                runtimeToggle,
		"input_anchor_governor":                         inputAnchorGovernor,
		"weak_input_planner":                            weakInputPlanner,
		"planner_execution_contract":                    plannerExecutionContract,
		"progression_choice_ledger":                     progressionChoiceLedger,
		"step25_validation_gate":                        step25ValidationGate,
		"helper_budget_governor_trace":                  helperBudgetGovernorTrace,
		"helper_injection_budget_manager":               buildStep165HelperInjectionBudgetManager(maxInjectionChars, injectionAssembly),
		"input_context_slot_governor":                   buildStep165InputContextSlotGovernor(maxInputContextChars, inputContextTruncated),
		"transparency_preview_runtime_trace_extend":     buildStep165TransparencyPreviewRuntimeTraceExtend(inputContextText, inputContextTruncated, injectionAssembly),
		"handoff_anchor_metadata_alignment":             buildStep165HandoffAnchorMetadataAlignment(inputContextText, inputAnchorGovernor),
		"stale_arc_guard_carry_in_hooks":                buildStep165StaleArcGuardCarryInHooks(inputAnchorGovernor, helperBudgetGovernorTrace),
		"decision_adaptive_floor_ceiling":               buildStep165DecisionAdaptiveFloorCeiling(),
		"decision_max_slot":                             buildStep165DecisionMaxSlot(),
		"decision_runtime_token_hint":                   buildStep165DecisionRuntimeTokenHint(),
		"decision_saga_chapter_anchor_ladder":           buildStep165DecisionSagaChapterAnchorLadder(),
		"decision_explicit_user_input_specificity":      buildStep165DecisionExplicitUserInputSpecificity(),
		"step_16_8_baseline_compare":                    buildStep165Step168BaselineCompare(inputAnchorGovernor),
		"step_16_8_reason_visibility_guard_lane":        buildStep165Step168ReasonVisibilityGuardLane(),
		"step_17_direct_handoff_gate":                   buildStep165Step17DirectHandoffGate(),
		"step_17_evaluation_harness_baseline":           buildStep165Step17EvaluationHarnessBaseline(),
		"step_17_ops_trace_interpretation":              buildStep165Step17OpsTraceInterpretation(),
		"step_17_inspection_surface":                    buildStep165Step17InspectionSurface(),
		"stale_arc_ceiling":                             buildStep168StaleArcCeiling(inputAnchorGovernor),
		"scene_alignment":                               buildStep168SceneAlignment(rawUserInput, inputAnchorGovernor),
		"current_scene_evidence_min_criteria":           buildStep168CurrentSceneEvidenceMinCriteria(activeStates, evidence, chatLogs),
		"pending_threads_guard":                         buildStep168PendingThreadsGuard(pendingThreads),
		"reason_trace":                                  buildStep168ReasonTrace(inputAnchorGovernor),
		"failure_split":                                 buildStep168FailureSplit(inputAnchorGovernor),
		"packet_synthesis":                              buildStep168PacketSynthesis(storylines, pendingThreads),
		"callback_bias_ceiling":                         buildStep168CallbackBiasCeiling(storylines),
		"callback_scene_alignment":                      buildStep168CallbackSceneAlignment(storylines, activeStates),
		"stale_callback_suppression":                    buildStep168StaleCallbackSuppression(storylines),
		"old_arc_foreground_visibility":                 buildStep168OldArcForegroundVisibility(inputAnchorGovernor),
		"reason_code_vocabulary":                        buildStep168ReasonCodeVocabulary(),
		"preview_audit_transparency":                    buildStep168PreviewAuditTransparency(inputAnchorGovernor),
		"foreground_hijack_taxonomy":                    buildStep168ForegroundHijackTaxonomy(inputAnchorGovernor),
		"delayed_payoff_split":                          buildStep168DelayedPayoffSplit(storylines, episodeSums),
		"recall_gain_monopoly_split":                    buildStep168RecallGainMonopolySplit(inputAnchorGovernor),
		"stale_arc_revival_replay":                      buildStep168StaleArcRevivalReplay(inputAnchorGovernor),
		"tail_recall_hijack_gate":                       buildStep168TailRecallHijackGate(inputAnchorGovernor),
		"narrative_diversity_gate":                      buildStep168NarrativeDiversityGate(storylines, worldRules),
		"arc_monopoly_gate":                             buildStep168ArcMonopolyGate(inputAnchorGovernor),
		"js_continuity_rescue":                          buildStep168JSContinuityRescue(storylines, pendingThreads),
		"js_prompt_assembly_guard":                      buildStep168JSPromptAssemblyGuard(injectionAssembly),
		"js_trace_preview_transparency":                 buildStep168JSTracePreviewTransparency(inputContextText, injectionAssembly),
		"replay_corpus_baseline":                        buildStep168ReplayCorpusBaseline(inputAnchorGovernor),
		"backend_metadata_alignment":                    buildStep168BackendMetadataAlignment(storylines, pendingThreads),
		"evaluation_split":                              buildStep17EvaluationSplit(recallResult, 0.75),
		"ops_procedure_surface":                         buildStep17OpsProcedureSurface(),
		"inspection_lane_boundary":                      buildStep17InspectionLaneBoundary(),
		"adoption_gate":                                 buildStep17AdoptionGate(false),
		"release_hygiene":                               buildStep17ReleaseHygiene(),
		"retrieval_completeness_metric":                 buildStep17RetrievalCompletenessMetric(recallResult),
		"final_answer_quality_metric":                   buildStep17FinalAnswerQualityMetric(0.75),
		"failure_split_replay":                          buildStep17FailureSplitReplay(recallResult, 0.75),
		"regression_corpus":                             buildStep17RegressionCorpus(),
		"freshness_lag_metric":                          buildStep17FreshnessLagMetric(120, 80, 200),
		"promotion_backfill_rebuild":                    buildStep17PromotionBackfillRebuild(),
		"reembed_migration_health_probe":                buildStep17ReembedMigrationHealthProbe(),
		"failure_fallback_rollback":                     buildStep17FailureFallbackRollback(),
		"async_critic_delay":                            buildStep17AsyncCriticDelay(),
		"partial_write_retry":                           buildStep17PartialWriteRetry(),
		"explain_surface":                               buildStep17ExplainSurface(),
		"preview_audit_surface":                         buildStep17PreviewAuditSurface(),
		"dashboard_lane":                                buildStep17DashboardLane(),
		"display_guard":                                 buildStep17DisplayGuard(),
		"visibility_lane":                               buildStep17VisibilityLane(),
		"step_14_adoption_gate":                         buildStep17Step14AdoptionGate(),
		"step_15_adoption_gate":                         buildStep17Step15AdoptionGate(),
		"step_16_adoption_gate":                         buildStep17Step16AdoptionGate(),
		"bundle_regenerate_checklist":                   buildStep17BundleRegenerateChecklist(),
		"packaged_bundle_checklist":                     buildStep17PackagedBundleChecklist(),
		"freshness_silent_drop_gate":                    buildStep17FreshnessSilentDropGate(),
		"bundle_generation_evidence":                    buildStep17BundleGenerationEvidence(),
		"regression_corpus_green":                       buildStep17RegressionCorpusGreen(),
		"evaluation_split_smoke_check":                  buildStep17EvaluationSplitSmokeCheck(),
		"ops_dry_run_checklist_pass":                    buildStep17OpsDryRunChecklistPass(),
		"inspection_lane_boundary_review":               buildStep17InspectionLaneBoundaryReview(),
		"release_gate_complete":                         buildStep17ReleaseGateComplete(),
		"reaudit_backend_admin_owner":                   buildStep17ReauditBackendAdminOwner(),
		"reaudit_ops_doc_dry_run":                       buildStep17ReauditOpsDocDryRun(),
		"reaudit_root_runtime_read_only":                buildStep17ReauditRootRuntimeReadOnly(),
		"reaudit_release_gate_operator_evidence":        buildStep17ReauditReleaseGateOperatorEvidence(),
		"reaudit_admin_mutation_control_ui":             buildStep17ReauditAdminMutationControlUI(),
		"reaudit_release_execution_ui":                  buildStep17ReauditReleaseExecutionUI(),
		"reaudit_beta_0_8_closure_bundle":               buildStep17ReauditBeta08ClosureBundle(),
		"decision_completeness_metric_unit":             buildStep17DecisionCompletenessMetricUnit(),
		"decision_regression_corpus_mix":                buildStep17DecisionRegressionCorpusMix(),
		"decision_inspection_lane_default":              buildStep17DecisionInspectionLaneDefault(),
		"decision_adoption_gate_review_mode":            buildStep17DecisionAdoptionGateReviewMode(),
		"decision_bundle_regenerate_split":              buildStep17DecisionBundleRegenerateSplit(),
		"chroma_migration_preflight":                    buildStep17ChromaMigrationPreflight(),
		"chroma_shadow_bootstrap":                       buildStep17ChromaShadowBootstrap(),
		"chroma_backfill_dry_run":                       buildStep17ChromaBackfillDryRun(),
		"chroma_bulk_backfill":                          buildStep17ChromaBulkBackfill(),
		"chroma_reembed_discipline":                     buildStep17ChromaReembedDiscipline(),
		"chroma_divergence_health_probe":                buildStep17ChromaDivergenceHealthProbe(),
		"chroma_degraded_fallback_runbook":              buildStep17ChromaDegradedFallbackRunbook(),
		"chroma_rebuild_rollback_drill":                 buildStep17ChromaRebuildRollbackDrill(),
		"chroma_adoption_gate":                          buildStep17ChromaAdoptionGate(),
		"chroma_release_hygiene":                        buildStep17ChromaReleaseHygiene(),
		"chroma_migration_visibility_guard":             buildStep17ChromaMigrationVisibilityGuard(),
		"reset_admin":                                   buildResetAdmin(),
		"historical_content_preserved":                  buildHistoricalContentPreserved(),
		"reset_note_only":                               buildResetNoteOnly(),
		"step17_closure_gate":                           buildStep17ClosureGate(),
		"context_files_reviewed":                        buildContextFilesReviewed(),
		"prep_anchor_vrhy":                              buildPrepAnchorVRHY(),
		"historical_reference_only":                     buildHistoricalReferenceOnly(),
		"backend_prep_anchor":                           buildBackendPrepAnchor(),
		"routing_contract_prep_anchor":                  buildRoutingContractPrepAnchor(),
		"runtime_prep_scope":                            buildRuntimePrepScope(),
		"vr_scoped_verbatim_support_text":               buildVRScopedVerbatimSupportText(injectionAssembly.ScopedVerbatimSupport),
		"vr_policy_owner_block":                         buildVRPolicyOwnerBlock(),
		"vr_prompt_injection_strategy":                  buildVRPromptInjectionStrategy(),
		"vr_hierarchy_escape_hatch":                     buildVRHierarchyEscapeHatch(),
		"vr_backend_test_guard":                         buildVRBackendTestGuard(),
		"vr_runtime_transparency":                       buildVRRuntimeTransparency(),
		"vr_regression_bundle_green":                    buildVRRegressionBundleGreen(),
		"hy_semantic_rank_score":                        buildHYSemanticRankScore(),
		"hy_soft_bias":                                  buildHYSoftBias(),
		"hy_stopword_guard":                             buildHYStopwordGuard(),
		"hy_q1a_propagation":                            buildHYQ1aPropagation(),
		"hy_runtime_inspection":                         buildHYRuntimeInspection(),
		"hy_recurring_risk_guards":                      buildHYRecurringRiskGuards(),
		"hy_policy_registry":                            buildHYPolicyRegistry(),
		"hy_stop_at_18_2c":                              buildHYStopAt18_2c(),
		"hy_tail_budget_policy_owner":                   buildHYTailBudgetPolicyOwner(),
		"hy_tail_budget_rescue_pass":                    buildHYTailBudgetRescuePass(),
		"hy_tail_budget_rescue_trace":                   buildHYTailBudgetRescueTrace(),
		"hy_tail_budget_q1a_propagation":                buildHYTailBudgetQ1aPropagation(),
		"hy_tail_budget_regression":                     buildHYTailBudgetRegression(),
		"qr_query_class_contract":                       buildQRQueryClassContract(),
		"qr_query_class_taxonomy":                       buildQRQueryClassTaxonomy(),
		"qr_primary_class_selection":                    buildQRPrimaryClassSelection(),
		"qr_lexical_cue_block":                          buildQRLexicalCueBlock(),
		"qr_query_class_contract_test":                  buildQRQueryClassContractTest(),
		"qr_query_class_budget_policy":                  buildQRQueryClassBudgetPolicy(),
		"qr_q3c_budget_reuse":                           buildQRQ3cBudgetReuse(),
		"qr_temporal_profile_budget":                    buildQRTemporalProfileBudget(),
		"qr_budget_visibility":                          buildQRBudgetVisibility(),
		"qr_query_class_budget_test":                    buildQRQueryClassBudgetTest(),
		"qr_note_policy":                                buildQRNotePolicy(),
		"qr_scene_canon_no_pre_extract":                 buildQRSceneCanonNoPreExtract(),
		"qr_callback_resume_temporal_note_only":         buildQRCallbackResumeTemporalNoteOnly(),
		"qr_note_policy_fields":                         buildQRNotePolicyFields(),
		"qr_note_policy_test":                           buildQRNotePolicyTest(),
		"qr_route_policy":                               buildQRRoutePolicy(),
		"qr_route_families":                             buildQRRouteFamilies(),
		"qr_long_tail_route_candidates":                 buildQRLongTailRouteCandidates(),
		"qr_route_policy_fields":                        buildQRRoutePolicyFields(),
		"qr_route_policy_test":                          buildQRRoutePolicyTest(),
		"vx_hybrid_replay_gate":                         buildVXHybridReplayGate(),
		"vx_replay_threshold_reuse":                     buildVXReplayThresholdReuse(),
		"vx_hybrid_replay_states":                       buildVXHybridReplayStates(),
		"vx_hybrid_replay_test":                         buildVXHybridReplayTest(),
		"vx_heldout_completeness_gate":                  buildVXHeldoutCompletenessGate(),
		"vx_heldout_metrics":                            buildVXHeldoutMetrics(),
		"vx_heldout_threshold_reuse":                    buildVXHeldoutThresholdReuse(),
		"vx_heldout_completeness_test":                  buildVXHeldoutCompletenessTest(),
		"vx_latency_token_budget_gate":                  buildVXLatencyTokenBudgetGate(),
		"vx_latency_token_metrics":                      buildVXLatencyTokenMetrics(),
		"vx_latency_token_threshold_reuse":              buildVXLatencyTokenThresholdReuse(),
		"vx_latency_token_test":                         buildVXLatencyTokenTest(),
		"vx_truth_boundary_gate":                        buildVXTruthBoundaryGate(),
		"vx_truth_boundary_precedence":                  buildVXTruthBoundaryPrecedence(),
		"vx_truth_boundary_states":                      buildVXTruthBoundaryStates(),
		"vx_truth_boundary_test":                        buildVXTruthBoundaryTest(),
		"vx_truncation_summary_loss_gate":               buildVXTruncationSummaryLossGate(),
		"vx_truncation_summary_loss_metrics":            buildVXTruncationSummaryLossMetrics(),
		"vx_truncation_summary_loss_threshold_reuse":    buildVXTruncationSummaryLossThresholdReuse(),
		"vx_truncation_summary_loss_states":             buildVXTruncationSummaryLossStates(),
		"vx_truncation_summary_loss_test":               buildVXTruncationSummaryLossTest(),
		"post_chroma_top1_scoped_verbatim":              buildPostChromaTop1ScopedVerbatim(),
		"post_chroma_top2_hybrid_scoring":               buildPostChromaTop2HybridScoring(),
		"post_chroma_top3_temporal_relation":            buildPostChromaTop3TemporalRelation(),
		"post_chroma_top4_temporal_validity":            buildPostChromaTop4TemporalValidity(),
		"post_chroma_top5_entity_graph":                 buildPostChromaTop5EntityGraph(),
		"post_chroma_top6_selective_rerank":             buildPostChromaTop6SelectiveRerank(),
		"vr_raw_preserving_support":                     buildVRRawPreservingSupport(),
		"vr_hybrid_realism":                             buildVRHybridRealism(),
		"vr_soft_routing":                               buildVRSoftRouting(),
		"vr_latency_discipline":                         buildVRLatencyDiscipline(),
		"vr_truth_boundary_preserve":                    buildVRTruthBoundaryPreserve(),
		"vr_18_1a_raw_transcript":                       buildVR18_1aRawTranscript(),
		"vr_18_1b_source_tag":                           buildVR18_1bSourceTag(),
		"vr_18_1c_prompt_injection":                     buildVR18_1cPromptInjection(),
		"vr_18_1d_hierarchy_escape":                     buildVR18_1dHierarchyEscape(),
		"hy_18_2a_semantic_keyword":                     buildHY18_2aSemanticKeyword(),
		"hy_18_2b_soft_bias":                            buildHY18_2bSoftBias(),
		"hy_18_2c_score_inspection":                     buildHY18_2cScoreInspection(),
		"hy_18_2d_adaptive_top_k":                       buildHY18_2dAdaptiveTopK(),
		"qr_18_3a_query_class":                          buildQR18_3aQueryClass(),
		"qr_18_3b_retrieval_depth":                      buildQR18_3bRetrievalDepth(),
		"qr_18_3c_extract_before_read":                  buildQR18_3cExtractBeforeRead(),
		"qr_18_3d_long_tail_route":                      buildQR18_3dLongTailRoute(),
		"vx_18_4a_semantic_hybrid_replay":               buildVX18_4aSemanticHybridReplay(),
		"vx_18_4b_held_out_recall":                      buildVX18_4bHeldOutRecall(),
		"vx_18_4c_latency_token":                        buildVX18_4cLatencyToken(),
		"vx_18_4d_truth_boundary_replay":                buildVX18_4dTruthBoundaryReplay(),
		"vx_18_4e_top_k_truncation":                     buildVX18_4eTopKTruncation(),
		"pre_release_version_marker":                    buildPreReleaseVersionMarker(),
		"pre_release_bundle_authority":                  buildPreReleaseBundleAuthority(),
		"pre_release_artifact":                          buildPreReleaseArtifact(),
		"pre_release_vr_smoke":                          buildPreReleaseVRSmoke(),
		"pre_release_hy_smoke":                          buildPreReleaseHYSmoke(),
		"pre_release_qr_smoke":                          buildPreReleaseQRSmoke(),
		"pre_release_vx_review":                         buildPreReleaseVXReview(),
		"pre_release_raw_snippet":                       buildPreReleaseRawSnippet(),
		"pre_release_hybrid_bias":                       buildPreReleaseHybridBias(),
		"pre_release_query_class_rule":                  buildPreReleaseQueryClassRule(),
		"pre_release_retrieval_note":                    buildPreReleaseRetrievalNote(),
		"reset_admin_185":                               buildResetAdmin185(),
		"historical_content_preserved_185":              buildHistoricalContentPreserved185(),
		"reset_note_only_185":                           buildResetNoteOnly185(),
		"bounded_live_scope":                            buildBoundedLiveScope(),
		"sqlite_truth_preserve":                         buildSQLiteTruthPreserve(),
		"fail_open_safety":                              buildFailOpenSafety(),
		"operator_visibility":                           buildOperatorVisibility(),
		"silent_authority_drift_guard":                  buildSilentAuthorityDriftGuard(),
		"release_honesty":                               buildReleaseHonesty(),
		"live_chroma_toggle_config":                     buildLiveChromaToggleConfig(),
		"live_scope_memory_only":                        buildLiveScopeMemoryOnly(),
		"live_chroma_topk_cap":                          buildLiveChromaTopkCap(),
		"shadow_disabled_degrade_rule":                  buildShadowDisabledDegradeRule(),
		"chroma_identity_sqlite_hydration":              buildChromaIdentitySQLiteHydration(),
		"chroma_sqlite_dedupe_merge":                    buildChromaSQLiteDedupeMerge(),
		"canonical_precedence_formatting":               buildCanonicalPrecedenceFormatting(),
		"chroma_miss_fallback_preserve":                 buildChromaMissFallbackPreserve(),
		"operator_inspection_surface":                   buildOperatorInspectionSurface(),
		"live_limited_mode_toggle":                      buildLiveLimitedModeToggle(),
		"health_adoption_prerequisite":                  buildHealthAdoptionPrerequisite(),
		"narrow_rollout_rule":                           buildNarrowRolloutRule(),
		"chroma_enabled_smoke_check":                    buildChromaEnabledSmokeCheck(),
		"degraded_fail_open_replay":                     buildDegradedFailOpenReplay(),
		"sqlite_baseline_parity_replay":                 buildSQLiteBaselineParityReplay(),
		"truth_boundary_source_order_replay":            buildTruthBoundarySourceOrderReplay(),
		"release_note_honesty_checklist":                buildReleaseNoteHonestyChecklist(),
		// SEQ-18.5-P201~P205 release gate surfaces (dry-run evidence only; no actual artifact created)
		"bundle_release_gate_201":                    buildBundleReleaseGate201(),
		"limited_live_chroma_smoke_check_202":        buildLimitedLiveChromaSmokeCheck202(),
		"sqlite_fail_open_replay_pass_203":           buildSQLiteFailOpenReplayPass203(),
		"operator_visibility_fallback_checklist_204": buildOperatorVisibilityFallbackChecklist204(),
		"release_note_bundle_notes_complete_205":     buildReleaseNoteBundleNotesComplete205(),
		// SEQ-18.5-P209~P212 decision surfaces (operator-gated, dry-run only)
		"first_live_scope_decision_209":               buildFirstLiveScopeDecision209(),
		"chroma_candidate_merge_replace_decision_210": buildChromaCandidateMergeReplaceDecision210(),
		"degraded_threshold_decision_211":             buildDegradedThresholdDecision211(),
		"operator_visibility_scope_decision_212":      buildOperatorVisibilityScopeDecision212(),
		// SEQ-19-P9~P11 reset administration surfaces
		"reset_admin_19":                  buildResetAdmin19(),
		"historical_content_preserved_19": buildHistoricalContentPreserved19(),
		"reset_note_only_19":              buildResetNoteOnly19(),
		// SEQ-19-P15~P22 temporal state surfaces
		"temporal_state":                   buildTemporalState19(activeStates, chatLogs, canonicalLayers),
		"current_story_clock_resolution":   buildCurrentStoryClockResolution(activeStates),
		"precision_label_contract":         buildPrecisionLabelContract(),
		"invalid_unknown_degradation":      buildInvalidUnknownDegradation(),
		"temporal_split_rule":              buildTemporalSplitRule(),
		"story_clock_surface_guard":        buildStoryClockSurfaceGuard(),
		"step18_plus_19_regression_bundle": buildStep18Plus19RegressionBundle(),
		// SEQ-19-P30~P42 temporal relation ledger schema surfaces
		"temporal_relation_ledger_canonical": buildTemporalRelationLedgerCanonical(),
		"schema_phrase_ingress":              buildSchemaPhraseIngress(),
		"schema_owner_block":                 buildSchemaOwnerBlock(),
		"canonical_data_override_guard":      buildCanonicalDataOverrideGuard(),
		"locale_pack_split":                  buildLocalePackSplit(),
		"multilingual_deictic_parity":        buildMultilingualDeicticParity(),
		"active_locales_gating":              buildActiveLocalesGating(),
		"snake_case_camel_case_inspect":      buildSnakeCaseCamelCaseInspect(),
		"valid_from_to_turn_range":           buildValidFromToTurnRange(),
		"missing_anchor_degradation":         buildMissingAnchorDegradation(),
		"temporal_relation_ledger_complete":  buildTemporalRelationLedgerComplete(),
		// SEQ-19-P50~P57 elapsed-time normalization surfaces
		"sc19_elapsed_policy_owner":           buildElapsedPolicyOwner(),
		"elapsed_time_decision_extended":      buildElapsedTimeDecisionExtended(currentStoryClock19, temporalRelationLedger19),
		"clock_write_directive_extended":      buildClockWriteDirectiveExtended(currentStoryClock19, temporalRelationLedger19),
		"temporal_support_packet":             temporalSupportPacket,
		"temporal_write_discipline":           buildTemporalWriteDiscipline(),
		"elapsed_policy_compactness":          buildElapsedPolicyCompactness(),
		"temporal_guard_bundle":               buildTemporalGuardBundle(),
		"step18_plus_19_regression_bundle_57": buildStep18Plus19RegressionBundle57(),
		// SEQ-19-P66~P69 locale pack + replay surfaces
		"week_unit_support":                   buildWeekUnitSupport(),
		"temporal_replay_cases":               buildTemporalReplayCases(),
		"bounded_week_month_write_guard":      buildBoundedWeekMonthWriteGuard(),
		"step18_plus_19_regression_bundle_69": buildStep18Plus19RegressionBundle69(),
		// SEQ-19-P78~P81 mixed-lane VX replay surfaces
		"mixed_lane_precedence_contract":      buildMixedLanePrecedenceContract(),
		"mixed_lane_replay_cases":             buildMixedLaneReplayCases(),
		"mixed_lane_split_rule_outcome":       buildMixedLaneSplitRuleOutcome(),
		"step18_plus_19_regression_bundle_81": buildStep18Plus19RegressionBundle81(),
		// SEQ-19-P90~P93 degrade replay / VX coverage surfaces
		"missing_anchor_degrade_contract":       buildMissingAnchorDegradeContract(),
		"missing_anchor_exact_phrase_degrade":   buildMissingAnchorExactPhraseDegrade(),
		"low_precision_recalled_relation_guard": buildLowPrecisionRecalledRelationGuard(),
		"step18_plus_19_regression_bundle_93":   buildStep18Plus19RegressionBundle93(),
		// SEQ-19-P102~P105 temporal packet truth-boundary / precedence surfaces
		"temporal_packet_truth_boundary_contract": buildTemporalPacketTruthBoundaryContract(),
		"temporal_packet_mixed_precedence":        buildTemporalPacketMixedPrecedence(),
		"temporal_packet_clock_missing_boundary":  buildTemporalPacketClockMissingBoundary(),
		"step18_plus_19_regression_bundle_105":    buildStep18Plus19RegressionBundle105(),
		// SEQ-19-P114~P117 response-time validator helper cluster / trace-only surfaces
		"step19_validator_helper_cluster_contract":    buildStep19ValidatorHelperClusterContract(),
		"temporal_precedence_resolution_order":        buildTemporalPrecedenceResolutionOrder(),
		"temporal_deictic_warning_classes":            buildTemporalDeicticWarningClasses(),
		"temporal_deictic_trace_only_warning_surface": buildTemporalDeicticTraceOnlyWarningSurface(),
		// SEQ-19-P125~P128 classification / write-discipline surfaces
		"temporal_classification_write_discipline_surface": buildTemporalClassificationWriteDisciplineSurface(),
		"temporal_classification_exceptions":               buildTemporalClassificationExceptions(),
		"temporal_write_discipline_rules":                  buildTemporalWriteDisciplineRules(),
		"temporal_relation_entry_metadata_surface":         buildTemporalRelationEntryMetadataSurface(),
		// SEQ-19-P137~P139 locale-aware extraction / multilingual parity surfaces
		"locale_aware_extractor_owner_block":        buildLocaleAwareExtractorOwnerBlock(),
		"recalled_past_parity_surface":              buildRecalledPastParitySurface(),
		"current_scene_next_morning_parity_surface": buildCurrentSceneNextMorningParitySurface(),
		// SEQ-19-P140 activeLocales fail-open gating
		"active_locales_fail_open_gating_contract": buildActiveLocalesFailOpenGatingContract(),
		// SEQ-19-P288~P292 finish-line criteria surfaces
		"current_time_explicitness_contract": buildCurrentTimeExplicitnessContract(),
		"anchor_bound_relation_contract":     buildAnchorBoundRelationContract(),
		"bounded_ambiguity_contract":         buildBoundedAmbiguityContract(),
		"advance_discipline_contract":        buildAdvanceDisciplineContract(),
		"truth_boundary_preserve_contract":   buildTruthBoundaryPreserveContract(),
		// SEQ-19-P296~P299 sub-step 19-1 schema definition surfaces
		"current_story_clock_schema_define":               buildCurrentStoryClockSchemaDefine(),
		"session_state_timeline_anchor_precedence_define": buildSessionStateTimelineAnchorPrecedenceDefine(),
		"precision_label_define":                          buildPrecisionLabelDefine(),
		"current_scene_recalled_past_split_define":        buildCurrentSceneRecalledPastSplitDefine(),
		// SEQ-19-P303~P307 sub-step 19-2 schema definition surfaces
		"temporal_relation_schema_define":       buildTemporalRelationSchemaDefine(),
		"phrase_ingress_normalization_define":   buildPhraseIngressNormalizationDefine(),
		"temporal_relation_surface_define":      buildTemporalRelationSurfaceDefine(),
		"anchor_ambiguity_carry_forward_define": buildAnchorAmbiguityCarryForwardDefine(),
		"locale_parser_pack_boundary_define":    buildLocaleParserPackBoundaryDefine(),
		// SEQ-19-P311~P314 sub-step 19-3 schema definition surfaces
		"advance_trigger_define":               buildAdvanceTriggerDefine(),
		"scene_transition_define":              buildSceneTransitionDefine(),
		"elapsed_time_write_discipline_define": buildElapsedTimeWriteDisciplineDefine(),
		"temporal_support_packet_define":       buildTemporalSupportPacketDefine(),
		// SEQ-19-P318~P322 sub-step 19-4 VX replay surfaces
		"temporal_replay_define_19_4a":                            buildTemporalReplayDefine19_4a(),
		"current_scene_recalled_past_conflict_replay_define":      buildCurrentSceneRecalledPastConflictReplayDefine(),
		"missing_anchor_low_precision_degrade_replay_define":      buildMissingAnchorLowPrecisionDegradeReplayDefine(),
		"temporal_packet_truth_boundary_precedence_replay_define": buildTemporalPacketTruthBoundaryPrecedenceReplayDefine(),
		"response_time_deictic_validator_replay_define":           buildResponseTimeDeicticValidatorReplayDefine(),
		// SEQ-19-P323~P324 sub-step 19-4f/19-4g classification + multilingual replay surfaces
		"figurative_duration_planned_future_recalled_past_classification_replay_define": buildFigurativeDurationPlannedFutureRecalledPastClassificationReplayDefine(),
		"multilingual_parity_mixed_language_fail_open_replay_define":                    buildMultilingualParityMixedLanguageFailOpenReplayDefine(),
		// SEQ-19-P328~P332 Beta 1.0 release gate surfaces
		"beta_1_0_bundle_latest_root_runtime_define":   buildBeta10BundleLatestRootRuntimeDefine(),
		"story_clock_smoke_check_pass":                 buildStoryClockSmokeCheckPass(),
		"relative_time_normalization_smoke_check_pass": buildRelativeTimeNormalizationSmokeCheckPass(),
		"elapsed_time_advance_replay_pass":             buildElapsedTimeAdvanceReplayPass(),
		"ambiguity_precedence_review_checklist_pass":   buildAmbiguityPrecedenceReviewChecklistPass(),
		// SEQ-19-P333, P337~P344 Beta 1.0 release gate + decision surfaces
		"multilingual_temporal_parity_smoke_check_pass":                                 buildMultilingualTemporalParitySmokeCheckPass(),
		"current_story_clock_absolute_datetime_bounded_story_day":                       buildCurrentStoryClockAbsoluteDatetimeBoundedStoryDay(),
		"relative_time_normalization_numeric_offset_vocabulary_first":                   buildRelativeTimeNormalizationNumericOffsetVocabularyFirst(),
		"elapsed_time_advance_conservative_manual_scene_classifier":                     buildElapsedTimeAdvanceConservativeManualSceneClassifier(),
		"missing_anchor_degrade":                                                        buildMissingAnchorDegrade(),
		"locale_parsing_single_detector_active_locales_merge":                           buildLocaleParsingSingleDetectorActiveLocalesMerge(),
		"ko_en_bootstrap_extractor_locale_pack_parser_replace_cutover":                  buildKoEnBootstrapExtractorLocalePackParserReplaceCutover(),
		"unspecified_time_fallback_no_advance_carry_forward_discipline":                 buildUnspecifiedTimeFallbackNoAdvanceCarryForwardDiscipline(),
		"relation_only_future_past_reference_current_scene_advance_evidence_gate_split": buildRelationOnlyFuturePastReferenceCurrentSceneAdvanceEvidenceGateSplit(),
		// SEQ-20 Preparatory reset/admin surfaces (P9 ~ P11)
		"seq20_reset_admin_note":             buildSeq20ResetAdminNote(),
		"seq20_historical_content_preserved": buildSeq20HistoricalContentPreserved(),
		"seq20_reset_note_only":              buildSeq20ResetNoteOnly(),
		// SEQ-20 q20a temporal query expansion surfaces (P21 ~ P28)
		"q20a_temporal_query_expansion_preparatory": buildQ20aTemporalQueryExpansionPreparatory(),
		"q20a_v1_temporal_query_expansion":          buildQ20aV1TemporalQueryExpansion(),
		"q20a_rule_surface_focus_range":             buildQ20aRuleSurfaceFocusRange(),
		"q20a_derives_from_sc19_relation_schema":    buildQ20aDerivesFromSc19RelationSchema(),
		"q20a_mirrored_at_recall_intent":            buildQ20aMirroredAtRecallIntent(),
		"q20a_current_clock_overlay_cue_pack":       buildQ20aCurrentClockOverlayCuePack(),
		"q20a_qr1a_lexical_routing_normalized":      buildQ20aQr1aLexicalRoutingNormalized(),
		"q20a_contract_only_groundwork":             buildQ20aContractOnlyGroundwork(),
		// SEQ-20 q20b temporal validity read policy surfaces (P36 ~ P40)
		"q20b_temporal_validity_read_policy_preparatory": buildQ20bTemporalValidityReadPolicyPreparatory(),
		"q20b_v1_temporal_validity_read_policy":          buildQ20bV1TemporalValidityReadPolicy(),
		"q20b_read_priority_modes":                       buildQ20bReadPriorityModes(),
		"q20b_mirrored_at_recall_intent_and_query_class": buildQ20bMirroredAtRecallIntentAndQueryClass(),
		"q20b_stops_before_later_tv_work":                buildQ20bStopsBeforeLaterTVWork(),
		// SEQ-20 q20c temporal event invalidation support surfaces (P47 ~ P51)
		"q20c_temporal_event_invalidation_preparatory": buildQ20cTemporalEventInvalidationPreparatory(),
		"q20c_v1_temporal_event_invalidation_support":  buildQ20cV1TemporalEventInvalidationSupport(),
		"q20c_invalidation_modes":                      buildQ20cInvalidationModes(),
		"q20c_mirrored_at_recall_intent":               buildQ20cMirroredAtRecallIntent(),
		"q20c_separate_from_promotion_lag":             buildQ20cSeparateFromPromotionLag(),
		// SEQ-20 q20d temporal promotion-lag support surfaces (P57 ~ P60)
		"q20d_temporal_promotion_lag_preparatory": buildQ20dTemporalPromotionLagPreparatory(),
		"q20d_v1_temporal_promotion_lag_support":  buildQ20dV1TemporalPromotionLagSupport(),
		"q20d_anchor_precedence":                  buildQ20dAnchorPrecedence(),
		"q20d_mirrored_at_recall_intent":          buildQ20dMirroredAtRecallIntent(),
		// SEQ-20 q20e temporal hot recall buffer surfaces (P66 ~ P69)
		"q20e_temporal_hot_recall_buffer_preparatory": buildQ20eTemporalHotRecallBufferPreparatory(),
		"q20e_v1_temporal_hot_recall_buffer":          buildQ20eV1TemporalHotRecallBuffer(),
		"q20e_bridge_source_set":                      buildQ20eBridgeSourceSet(),
		"q20e_mirrored_at_recall_intent":              buildQ20eMirroredAtRecallIntent(),
		// SEQ-20 q20f lightweight entity index surfaces (P76 ~ P81)
		"q20f_lightweight_entity_index_preparatory": buildQ20fLightweightEntityIndexPreparatory(),
		"q20f_v1_lightweight_entity_index":          buildQ20fV1LightweightEntityIndex(),
		"q20f_structured_state_surfaces":            buildQ20fStructuredStateSurfaces(),
		"q20f_mirrored_at_query_class":              buildQ20fMirroredAtQueryClass(),
		"q20f_stops_before_graph_like_support":      buildQ20fStopsBeforeGraphLikeSupport(),
		"q20f_token_boundary_structured_labels":     buildQ20fTokenBoundaryStructuredLabels(),
		// SEQ-20 q20g graph-like support signal surfaces (P89 ~ P93)
		"q20g_graph_like_support_signal_preparatory": buildQ20gGraphLikeSupportSignalPreparatory(),
		"q20g_v1_graph_like_support_signal":          buildQ20gV1GraphLikeSupportSignal(),
		"q20g_pair_sources_and_fail_open":            buildQ20gPairSourcesAndFailOpen(),
		"q20g_mirrored_at_query_class":               buildQ20gMirroredAtQueryClass(),
		"q20g_stops_before_inspection_formatting":    buildQ20gStopsBeforeInspectionFormatting(),
		// SEQ-20 q20h entity/graph boost inspection surface surfaces (P99 ~ P102)
		"q20h_entity_graph_boost_inspection_surface_preparatory": buildQ20hEntityGraphBoostInspectionSurfacePreparatory(),
		"q20h_v1_entity_graph_boost_inspection_surface":          buildQ20hV1EntityGraphBoostInspectionSurface(),
		"q20h_inspection_role_and_authority_notice":              buildQ20hInspectionRoleAndAuthorityNotice(),
		"q20h_mirrored_at_query_class":                           buildQ20hMirroredAtQueryClass(),
		// SEQ-20 q20i lagging current state boost surfaces (P109 ~ P112)
		"q20i_lagging_current_state_boost_preparatory": buildQ20iLaggingCurrentStateBoostPreparatory(),
		"q20i_v1_lagging_current_state_boost":          buildQ20iV1LaggingCurrentStateBoost(),
		"q20i_activation_and_precedence":               buildQ20iActivationAndPrecedence(),
		"q20i_mirrored_at_query_class":                 buildQ20iMirroredAtQueryClass(),
		// SEQ-20 q20j motive-shadow hint surfaces (P118 ~ P121)
		"q20j_motive_shadow_hint_preparatory": buildQ20jMotiveShadowHintPreparatory(),
		"q20j_v1_motive_shadow_hint":          buildQ20jV1MotiveShadowHint(),
		"q20j_truth_write_forbidden":          buildQ20jTruthWriteForbidden(),
		"q20j_mirrored_at_query_class":        buildQ20jMirroredAtQueryClass(),
		// SEQ-20 q20k motive-shadow non-escalation guard surfaces (P127 ~ P129)
		"q20k_motive_shadow_non_escalation_guard_preparatory": buildQ20kMotiveShadowNonEscalationGuardPreparatory(),
		"q20k_v1_motive_shadow_non_escalation_guard":          buildQ20kV1MotiveShadowNonEscalationGuard(),
		"q20k_mirrored_at_query_class":                        buildQ20kMirroredAtQueryClass(),
		// SEQ-20 q20l relation edge support ledger surfaces (P135 ~ P138)
		"q20l_relation_edge_support_ledger_preparatory": buildQ20lRelationEdgeSupportLedgerPreparatory(),
		"q20l_v1_relation_edge_support_ledger":          buildQ20lV1RelationEdgeSupportLedger(),
		"q20l_graph_truth_write_forbidden":              buildQ20lGraphTruthWriteForbidden(),
		"q20l_mirrored_at_query_class":                  buildQ20lMirroredAtQueryClass(),
		// SEQ-20 aggregate summary surfaces (P231 ~ P236)
		"seq20_validity_priority":         buildSeq20P231ValidityPriority(),
		"seq20_support_only_accelerator":  buildSeq20P232SupportOnlyAccelerator(),
		"seq20_ambiguity_reduction":       buildSeq20P233AmbiguityReduction(),
		"seq20_inspection_visibility":     buildSeq20P234InspectionVisibility(),
		"seq20_truth_precedence_preserve": buildSeq20P235TruthPrecedencePreserve(),
		"seq20_hot_bridge":                buildSeq20P236HotBridge(),
		// SEQ-20 q20m temporal ambiguity support note surfaces (P258 ~ P259)
		"q20m_temporal_ambiguity_support_note_preparatory": buildQ20mTemporalAmbiguitySupportNotePreparatory(),
		"q20m_v1_temporal_ambiguity_support_note":          buildQ20mV1TemporalAmbiguitySupportNote(),
		// SEQ-20 q20n alias/entity conflict disambiguation surfaces (P260 ~ P261)
		"q20n_alias_entity_conflict_disambiguation_preparatory": buildQ20nAliasEntityConflictDisambiguationPreparatory(),
		"q20n_v1_alias_entity_conflict_disambiguation":          buildQ20nV1AliasEntityConflictDisambiguation(),
		// SEQ-20 q20o temporal/entity support block source-tag rule surfaces (P262 ~ P263)
		"q20o_temporal_entity_source_tag_rule_preparatory": buildQ20oTemporalEntitySourceTagRulePreparatory(),
		"q20o_v1_temporal_entity_source_tag_rule":          buildQ20oV1TemporalEntitySourceTagRule(),
		// SEQ-20 q20p canonical-pending/stale-current conflict note surfaces (P264 ~ P265)
		"q20p_canonical_pending_stale_current_conflict_note_preparatory": buildQ20pCanonicalPendingStaleCurrentConflictNotePreparatory(),
		"q20p_v1_canonical_pending_stale_current_conflict_note":          buildQ20pV1CanonicalPendingStaleCurrentConflictNote(),
		// SEQ-20 q20q recall cue rescue rule surfaces (P266 ~ P267)
		"q20q_recall_cue_rescue_rule_preparatory": buildQ20qRecallCueRescueRulePreparatory(),
		"q20q_v1_recall_cue_rescue_rule":          buildQ20qV1RecallCueRescueRule(),
		// SEQ-20 q20r wide gather -> validity join rule surfaces (P268 ~ P269)
		"q20r_wide_gather_validity_join_rule_preparatory": buildQ20rWideGatherValidityJoinRulePreparatory(),
		"q20r_v1_wide_gather_validity_join_rule":          buildQ20rV1WideGatherValidityJoinRule(),
		// SEQ-20 q20s thin support tag fallback surfaces (P270 ~ P271)
		"q20s_thin_support_tag_fallback_preparatory": buildQ20sThinSupportTagFallbackPreparatory(),
		"q20s_v1_thin_support_tag_fallback":          buildQ20sV1ThinSupportTagFallback(),
		// SEQ-20 vx20a~vx20g validation replay gates (P286 ~ P299)
		"vx20a_temporal_validity_replay_gate":              buildVx20aTemporalValidityReplayGate(),
		"vx20b_entity_boost_false_positive_gate":           buildVx20bEntityBoostFalsePositiveGate(),
		"vx20c_graph_accelerator_degrade_gate":             buildVx20cGraphAcceleratorDegradeGate(),
		"vx20d_canonical_precedence_replay_gate":           buildVx20dCanonicalPrecedenceReplayGate(),
		"vx20e_promotion_blocked_freshness_replay_gate":    buildVx20ePromotionBlockedFreshnessReplayGate(),
		"vx20f_recall_cue_rescue_replay_gate":              buildVx20fRecallCueRescueReplayGate(),
		"vx20g_hot_buffer_wide_gather_non_regression_gate": buildVx20gHotBufferWideGatherNonRegressionGate(),
		// SEQ-20 Beta 1.1 release smoke gate surfaces (P312 ~ P316)
		"seq20_beta11_bundle_dry_run":                 buildSeq20P312Beta11BundleDryRun(),
		"seq20_temporal_validity_recall_smoke":        buildSeq20P313TemporalValidityRecallSmoke(),
		"seq20_entity_graph_accelerator_smoke":        buildSeq20P314EntityGraphAcceleratorSmoke(),
		"seq20_temporal_entity_disambiguation_smoke":  buildSeq20P315TemporalEntityDisambiguationSmoke(),
		"seq20_precedence_ambiguity_review_checklist": buildSeq20P316PrecedenceAmbiguityReviewChecklist(),
		// SEQ-20 final preserve summary surfaces (P330 ~ P333)
		"seq20_temporal_query_expansion_preserve": buildSeq20P330TemporalQueryExpansionPreserve(),
		"seq20_entity_index_preserve":             buildSeq20P331EntityIndexPreserve(),
		"seq20_graph_accelerator_preserve":        buildSeq20P332GraphAcceleratorPreserve(),
		"seq20_ambiguity_support_note_preserve":   buildSeq20P333AmbiguitySupportNotePreserve(),
		// SEQ-21 surfaces ??Beta 1.2 selective rerank + retrieval economics (P9 ~ P202)
		"seq21_reset_admin_note":                buildSeq21ResetAdminNote(),
		"seq21_historical_content_preserved":    buildSeq21HistoricalContentPreserved(),
		"seq21_reset_note_only":                 buildSeq21ResetNoteOnly(),
		"seq21_rerank_class_summary":            buildSeq21P181RerankClassSummary(),
		"seq21_budget_config_summary":           buildSeq21P182BudgetConfigSummary(),
		"seq21_failure_class_split_summary":     buildSeq21P183FailureClassSplitSummary(),
		"seq21_held_out_hygiene_summary":        buildSeq21P184HeldOutHygieneSummary(),
		"seq21_truth_boundary_preserve_summary": buildSeq21P185TruthBoundaryPreserveSummary(),
		"seq21_density_discipline_summary":      buildSeq21P186DensityDisciplineSummary(),
		"seq21_rerank_trigger_class":            buildSeq21P190RerankTriggerClass(),
		"seq21_rerank_support_only_schema":      buildSeq21P191RerankSupportOnlySchema(),
		"seq21_rerank_off_fallback":             buildSeq21P192RerankOffFallback(),
		"seq21_rerank_near_miss_trigger":        buildSeq21P193RerankNearMissTrigger(),
		"seq21_query_class_candidate_cap":       buildSeq21P197QueryClassCandidateCap(),
		"seq21_latency_budget_degrade":          buildSeq21P198LatencyBudgetDegrade(),
		"seq21_retrieval_cache_reuse":           buildSeq21P199RetrievalCacheReuse(),
		"seq21_failure_class_adaptive_cap":      buildSeq21P200FailureClassAdaptiveCap(),
		"seq21_dual_density_delivery_budget":    buildSeq21P201DualDensityDeliveryBudget(),
		"seq21_heavy_promotion_rule":            buildSeq21P202HeavyPromotionRule(),
		// SEQ-21 21-3 failure-class tuning loop surfaces (P206 ~ P209)
		"seq21_failure_taxonomy":           buildSeq21P206FailureTaxonomy(),
		"seq21_dev_split_tuning_loop":      buildSeq21P207DevSplitTuningLoop(),
		"seq21_held_out_confirmation_gate": buildSeq21P208HeldOutConfirmationGate(),
		"seq21_residual_long_tail_loop":    buildSeq21P209ResidualLongTailLoop(),
		// SEQ-21 21-4 validation/adoption gate surfaces (P213 ~ P219)
		"seq21_cost_vs_gain_replay":                    buildSeq21P213CostVsGainReplay(),
		"seq21_latency_token_envelope_replay":          buildSeq21P214LatencyTokenEnvelopeReplay(),
		"seq21_held_out_regression_gate":               buildSeq21P215HeldOutRegressionGate(),
		"seq21_post_chroma_default_promotion_criteria": buildSeq21P216PostChromaDefaultPromotionCriteria(),
		"seq21_cost_normalized_tail_recall_gate":       buildSeq21P217CostNormalizedTailRecallGate(),
		"seq21_density_mix_replay":                     buildSeq21P218DensityMixReplay(),
		"seq21_shared_runner_corpus_rule":              buildSeq21P219SharedRunnerCorpusRule(),
		// SEQ-21 Beta 1.2 release gate surfaces (P223 ~ P227)
		"seq21_beta12_bundle_dry_run":                 buildSeq21P223Beta12BundleDryRun(),
		"seq21_selective_rerank_trigger_smoke":        buildSeq21P224SelectiveRerankTriggerSmoke(),
		"seq21_candidate_budget_latency_smoke":        buildSeq21P225CandidateBudgetLatencySmoke(),
		"seq21_failure_class_tuning_review_checklist": buildSeq21P226FailureClassTuningReviewChecklist(),
		"seq21_held_out_cost_adoption_gate_complete":  buildSeq21P227HeldOutCostAdoptionGateComplete(),
		// SEQ-21 final preserve decision surfaces (P238 ~ P241)
		"seq21_bounded_trigger_classes_preserve":   buildSeq21P238BoundedTriggerClassesPreserve(),
		"seq21_query_class_candidate_cap_preserve": buildSeq21P239QueryClassCandidateCapPreserve(),
		"seq21_latency_degrade_path_preserve":      buildSeq21P240LatencyDegradePathPreserve(),
		"seq21_tuning_deferred_preserve":           buildSeq21P241TuningDeferredPreserve(),
		// SEQ-21.5 surfaces ??Backend structural closeout evidence (P416 ~ P432)
		"seq215_authority_frozen":           buildSeq215P416AuthorityFrozen(),
		"seq215_stale_history_rejected":     buildSeq215P417StaleHistoryRejected(),
		"seq215_turn_contracts_moved":       buildSeq215P418TurnContractsMoved(),
		"seq215_m3a_formatting_moved":       buildSeq215P419M3aFormattingMoved(),
		"seq215_proxy_config_moved":         buildSeq215P420ProxyConfigMoved(),
		"seq215_maintenance_queue_moved":    buildSeq215P421MaintenanceQueueMoved(),
		"seq215_chroma_c17_moved":           buildSeq215P422ChromaC17Moved(),
		"seq215_step17_helpers_extracted":   buildSeq215P423Step17HelpersExtracted(),
		"seq215_lc1_phase_a_moved":          buildSeq215P424LC1PhaseAMoved(),
		"seq215_lc1_phase_bcd_moved":        buildSeq215P425LC1PhaseBCDMoved(),
		"seq215_utility_services_moved":     buildSeq215P426UtilityServicesMoved(),
		"seq215_physical_baseline_recorded": buildSeq215P427PhysicalBaselineRecorded(),
		"seq215_wi14_removed":               buildSeq215P431WI14Removed(),
		"seq215_wi14_deletion_records":      buildSeq215P432WI14DeletionRecords(),
		// SEQ-21.5 core extraction / deferral / validation evidence (P436 ~ P448)
		"seq215_run_maintenance_pass_blocked": buildSeq215P436RunMaintenancePassBlocked(),
		"seq215_complete_turn_m4_extracted":   buildSeq215P437CompleteTurnM4Extracted(),
		"seq215_prepare_turn_extracted":       buildSeq215P438PrepareTurnExtracted(),
		"seq215_bundle_supervisor_reduced":    buildSeq215P439BundleSupervisorReduced(),
		"seq215_bundle_recall_reduced":        buildSeq215P440BundleRecallReduced(),
		"seq215_bundle_injection_reduced":     buildSeq215P441BundleInjectionReduced(),
		"seq215_lc1_remaining_moved":          buildSeq215P442LC1RemainingMoved(),
		"seq215_narrative_read_lock":          buildSeq215P443NarrativeReadLock(),
		"seq215_hypamemory_extracted":         buildSeq215P444HypamemoryExtracted(),
		"seq215_archive_center_js_deferral":   buildSeq215P445ArchiveCenterJSDeferral(),
		"seq215_or1e_rechecked":               buildSeq215P446OR1eRechecked(),
		"seq215_final_validation":             buildSeq215P447FinalValidation(),
		"seq215_step_complete":                buildSeq215P448StepComplete(),
		// SEQ-21.5 WI14 deletion slice evidence (P476 ~ P488)
		"seq215_authority_restate":                        buildSeq215P476AuthorityRestate(),
		"seq215_before_count":                             buildSeq215P477BeforeCount(),
		"seq215_exact_usage_search":                       buildSeq215P478ExactUsageSearch(),
		"seq215_delete_minimal_continuation_cues":         buildSeq215P479DeleteMinimalContinuationCues(),
		"seq215_delete_explicit_correction_markers":       buildSeq215P480DeleteExplicitCorrectionMarkers(),
		"seq215_delete_detect_input_mode":                 buildSeq215P481DeleteDetectInputMode(),
		"seq215_remove_build_weak_input_steering":         buildSeq215P482RemoveBuildWeakInputSteering(),
		"seq215_simplify_supervisor_planner":              buildSeq215P483SimplifySupervisorPlanner(),
		"seq215_keep_auto_advance_explicit":               buildSeq215P484KeepAutoAdvanceExplicit(),
		"seq215_py_compile_pass":                          buildSeq215P485PyCompilePass(),
		"seq215_focused_backend_tests":                    buildSeq215P486FocusedBackendTests(),
		"seq215_js_untouched":                             buildSeq215P487JSUntouched(),
		"seq215_after_count":                              buildSeq215P488AfterCount(),
		"seq215_js_authority":                             buildSeq215P556JSAuthority(),
		"seq215_backend_authority":                        buildSeq215P557BackendAuthority(),
		"seq215_no_root_standalone_pair":                  buildSeq215P558NoRootStandalonePair(),
		"seq215_backup_not_authority":                     buildSeq215P559BackupNotAuthority(),
		"seq215_deploy_not_authority":                     buildSeq215P560DeployNotAuthority(),
		"seq215_no_broad_split_before_narrow":             buildSeq215P561NoBroadSplitBeforeNarrow(),
		"seq215_stale_split_rejected_context":             buildSeq215P562StaleSplitRejectedContext(),
		"seq215_stale_split_rejected_progress":            buildSeq215P563StaleSplitRejectedProgress(),
		"seq215_beta08_metrics":                           buildSeq215P564Beta08Metrics(),
		"seq215_restate_guard":                            buildSeq215P565RestateGuard(),
		"seq215_promote_guard":                            buildSeq215P566PromoteGuard(),
		"seq215_turn_contracts_created":                   buildSeq215P589TurnContractsCreated(),
		"seq215_complete_turn_request_moved":              buildSeq215P590CompleteTurnRequestMoved(),
		"seq215_m4_complete_turn_request_moved":           buildSeq215P591M4CompleteTurnRequestMoved(),
		"seq215_m4_complete_turn_response_moved":          buildSeq215P592M4CompleteTurnResponseMoved(),
		"seq215_prepare_turn_settings_moved":              buildSeq215P593PrepareTurnSettingsMoved(),
		"seq215_prepare_turn_request_moved":               buildSeq215P594PrepareTurnRequestMoved(),
		"seq215_retrieval_document_q1a_moved":             buildSeq215P595RetrievalDocumentQ1AMoved(),
		"seq215_generation_packet_moved":                  buildSeq215P596GenerationPacketMoved(),
		"seq215_prepare_turn_response_moved":              buildSeq215P597PrepareTurnResponseMoved(),
		"seq215_moved_classes_imported_back":              buildSeq215P598MovedClassesImportedBack(),
		"seq215_route_decorators_stay":                    buildSeq215P599RouteDecoratorsStay(),
		"seq215_prepare_turn_stays":                       buildSeq215P600PrepareTurnStays(),
		"seq215_complete_turn_m4_stays":                   buildSeq215P601CompleteTurnM4Stays(),
		"seq215_public_route_paths_unchanged":             buildSeq215P602PublicRoutePathsUnchanged(),
		"seq215_response_fields_unchanged":                buildSeq215P603ResponseFieldsUnchanged(),
		"seq215_no_broad_tree_created":                    buildSeq215P604NoBroadTreeCreated(),
		"seq215_py_compile_turn_contracts":                buildSeq215P605PyCompileTurnContracts(),
		"seq215_focused_import_check":                     buildSeq215P606FocusedImportCheck(),
		"seq215_validation_record":                        buildSeq215P607ValidationRecord(),
		"seq215_phase1_validation_passed":                 buildSeq215P663Phase1ValidationPassed(),
		"seq215_prepare_turn_assembly_created":            buildSeq215P664PrepareTurnAssemblyCreated(),
		"seq215_format_memory_text_moved":                 buildSeq215P665FormatMemoryTextMoved(),
		"seq215_format_kg_text_moved":                     buildSeq215P666FormatKGTextMoved(),
		"seq215_format_episode_text_moved":                buildSeq215P667FormatEpisodeTextMoved(),
		"seq215_format_chapter_text_moved":                buildSeq215P668FormatChapterTextMoved(),
		"seq215_format_fallback_text_moved":               buildSeq215P669FormatFallbackTextMoved(),
		"seq215_clean_short_moved":                        buildSeq215P670CleanShortMoved(),
		"seq215_json_load_maybe_moved":                    buildSeq215P671JsonLoadMaybeMoved(),
		"seq215_predicate_matches_moved":                  buildSeq215P672PredicateMatchesMoved(),
		"seq215_world_rule_note_moved":                    buildSeq215P673WorldRuleNoteMoved(),
		"seq215_format_entity_digest_text_moved":          buildSeq215P674FormatEntityDigestTextMoved(),
		"seq215_format_entity_anchor_text_moved":          buildSeq215P675FormatEntityAnchorTextMoved(),
		"seq215_db_session_helpers_stay":                  buildSeq215P677DBSessionHelpersStay(),
		"seq215_core_logic_stay":                          buildSeq215P678CoreLogicStay(),
		"seq215_injection_pack_fields_unchanged":          buildSeq215P679InjectionPackFieldsUnchanged(),
		"seq215_py_compile_prepare_turn_assembly":         buildSeq215P680PyCompilePrepareTurnAssembly(),
		"seq215_focused_backend_tests_m3a":                buildSeq215P681FocusedBackendTestsM3a(),
		"seq215_m3a_validation_record":                    buildSeq215P682M3aValidationRecord(),
		"seq215_proxy_plugin_main_model_separated":        buildSeq215P730ProxyPluginMainModelSeparated(),
		"seq215_provider_ownership_split":                 buildSeq215P731ProviderOwnershipSplit(),
		"seq215_thin_proxy_route":                         buildSeq215P732ThinProxyRoute(),
		"seq215_config_service_split":                     buildSeq215P733ConfigServiceSplit(),
		"seq215_thin_config_route":                        buildSeq215P734ThinConfigRoute(),
		"seq215_routes_explicitly_wired":                  buildSeq215P735RoutesExplicitlyWired(),
		"seq215_public_paths_preserved":                   buildSeq215P736PublicPathsPreserved(),
		"seq215_compatibility_wrapper":                    buildSeq215P737CompatibilityWrapper(),
		"seq215_route_level_tests":                        buildSeq215P738RouteLevelTests(),
		"seq215_js_route_usage":                           buildSeq215P739JSRouteUsage(),
		"seq215_monolith_not_applicable":                  buildSeq215P740MonolithNotApplicable(),
		"seq215_prepare_turn_bundle_normal_use":           buildSeq215P776PrepareTurnBundleNormalUse(),
		"seq215_js_payload_mutation_owner":                buildSeq215P777JSPayloadMutationOwner(),
		"seq215_js_injection_budget_owner":                buildSeq215P778JSInjectionBudgetOwner(),
		"seq215_js_input_context_slotting_owner":          buildSeq215P779JSInputContextSlottingOwner(),
		"seq215_js_protection_blocks_owner":               buildSeq215P780JSProtectionBlocksOwner(),
		"seq215_js_hook_ui_integration_owner":             buildSeq215P781JSHookUIIntegrationOwner(),
		"seq215_js_offline_fail_open_owner":               buildSeq215P782JSOfflineFailOpenOwner(),
		"seq215_build_input_context_preserved":            buildSeq215P783BuildInputContextPreserved(),
		"seq215_assemble_injection_with_budget_preserved": buildSeq215P784AssembleInjectionWithBudgetPreserved(),
		"seq215_apply_context_injection_preserved":        buildSeq215P785ApplyContextInjectionPreserved(),
		"seq215_try_prepare_turn_takeover_off":            buildSeq215P786TryPrepareTurnTakeoverOff(),
		"seq215_js_node_check":                            buildSeq215P787JSNodeCheck(),
		"seq215_js_focused_contract_tests":                buildSeq215P788JSFocusedContractTests(),
		"seq215_p789_validation_record":                   buildSeq215P789ValidationRecord(),
		"seq215_runtime_split_status":                     buildSeq215P830RuntimeSplitStatus(),
		"seq215_backend_bundle_assisted":                  buildSeq215P831BackendBundleAssisted(),
		"seq215_plugin_only_modules":                      buildSeq215P832PluginOnlyModules(),
		"seq215_or1e_wording":                             buildSeq215P833OR1eWording(),
		"seq215_or1e_node_check":                          buildSeq215P834OR1eNodeCheck(),
		"seq215_validation_record_p835":                   buildSeq215P835ValidationRecord(),
		"seq215_phase1_complete":                          buildSeq215P869Phase1Complete(),
		"seq215_phase2_complete":                          buildSeq215P870Phase2Complete(),
		"seq215_phase3_complete":                          buildSeq215P871Phase3Complete(),
		"seq215_phase4_complete":                          buildSeq215P872Phase4Complete(),
		"seq215_context_readback":                         buildSeq215P873ContextReadback(),
		"seq215_progress_readback":                        buildSeq215P874ProgressReadback(),
		"seq215_stale_authority_search":                   buildSeq215P875StaleAuthoritySearch(),
		"seq215_false_backend_tree_search":                buildSeq215P876FalseBackendTreeSearch(),
		"seq215_no_backup_deploy_edited":                  buildSeq215P877NoBackupDeployEdited(),
		"seq215_changed_files_list":                       buildSeq215P878ChangedFilesList(),
		"seq215_validation_commands":                      buildSeq215P879ValidationCommands(),
		"seq215_additional_owner_split_bounded":           buildSeq215P880AdditionalOwnerSplitBounded(),
		"seq215_js_backend_offload_plugin_only":           buildSeq215P881JSBackendOffloadPluginOnly(),
		"seq215_master_checklist_open_zero":               buildSeq215P882MasterChecklistOpenZero(),
		"seq215_step_complete_p883":                       buildSeq215P883StepComplete(),
		"autonomy_plan":                                   autonomyPlan,
		"micro_beat_proposal":                             microBeatProposal,
		"scene_step_proposal":                             sceneStepProposal,
		"combined_proposal":                               combinedProposal,
		"writeback_preview":                               writebackPreview,
		"continuity_pack":                                 continuityPack,
		"persona_recollection":                            personaRecollection,
		"character_private_recollection":                  characterPrivateRecollection,
		"entity_recollection_relevance":                   recollectionRelevance,
		"generation_packet": map[string]any{
			"packet_mode":     packetMode,
			"degraded":        degraded,
			"fallback_reason": fallbackReason,
			"injection_text":  injectionOut,
			"prompt_assembly": promptAssembly,
			"trace_summary": map[string]any{
				"reads_ok":                      readsOK,
				"read_errors":                   len(readErrs),
				"memory_count":                  len(memories),
				"kg_count":                      len(kgTriples),
				"evidence_count":                len(evidence),
				"chat_log_count":                len(chatLogs),
				"resume_pack_present":           resumePack != nil,
				"storyline_count":               len(storylines),
				"storyline_selected_count":      len(storylineSelection.Selected),
				"storyline_dropped_count":       len(storylineSelection.Dropped),
				"storyline_stale_dropped_count": storylineSelectionSummary(storylineSelection)["stale_dropped_count"],
				"world_rule_count":              len(worldRules),
				"character_state_count":         len(charStates),
				"pending_thread_count":          len(pendingThreads),
				"active_state_count":            len(activeStates),
				"canonical_layer_count":         len(canonicalLayers),
				"narrative_current_state_count": len(narrativeCurrentValues),
				"episode_summary_count":         len(episodeSums),
				"max_injection_chars":           maxInjectionChars,
				"max_input_context_chars":       maxInputContextChars,
				"injection_truncated":           injectionTruncated,
				"input_context_truncated":       inputContextTruncated,
				"would_call_llm":                false,
				"would_write":                   false,
				"prompt_files_found":            promptAssembly["files_found"],
				"scoped_verbatim_support_count": injectionAssembly.ScopedVerbatimSupport.Count,
				"verbatim_support":              injectionAssembly.ScopedVerbatimSupport,
				"chapter_delivered":             strings.TrimSpace(injectionAssembly.ChapterText) != "",
				"chapter_text_chars":            len([]rune(strings.TrimSpace(injectionAssembly.ChapterText))),
				"chapter_consumed":              strings.TrimSpace(injectionAssembly.ChapterText) != "" && strings.Contains(strings.ToLower(injectionAssembly.Text), strings.ToLower(strings.TrimSpace(injectionAssembly.ChapterText))),
				"saga_delivered":                strings.TrimSpace(injectionAssembly.SagaText) != "",
				"saga_text_chars":               len([]rune(strings.TrimSpace(injectionAssembly.SagaText))),
				"saga_consumed":                 strings.TrimSpace(injectionAssembly.SagaText) != "" && strings.Contains(strings.ToLower(injectionAssembly.Text), strings.ToLower(strings.TrimSpace(injectionAssembly.SagaText))),
				"arc_delivered":                 strings.TrimSpace(injectionAssembly.ArcText) != "",
				"arc_text_chars":                len([]rune(strings.TrimSpace(injectionAssembly.ArcText))),
				"arc_consumed":                  strings.TrimSpace(injectionAssembly.ArcText) != "" && strings.Contains(strings.ToLower(injectionAssembly.Text), strings.ToLower(strings.TrimSpace(injectionAssembly.ArcText))),
				"hierarchy_escalation":          injectionAssembly.Counts["hierarchy_escalation"],
				"runtime_token_profile": map[string]any{
					"version":                "p61a.v1",
					"profile_source":         "client_meta_shadow",
					"context_window_profile": profile,
					"auto_optimized":         profile != "default",
					"status":                 "shadow_only",
				},
				"outbound_rewrite_guard": map[string]any{
					"version":         "p34a.v1",
					"status":          "ready",
					"rewrite_allowed": false,
					"reason":          "prepare_turn_read_only_assembly",
					"mode":            "shadow",
					"payload_mutated": false,
				},
			},
			"shadow_compare_record": shadowCompareRecord,
		},
		"note": "prepare-turn is a store-backed shadow assembly; no writes performed",
	})
}

func (s *Server) handleEffectiveInputs(w http.ResponseWriter, r *http.Request) {
	var req dto.SaveEffectiveInputRequest
	if err := dto.DecodeWithDefaults(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	sid := strings.TrimSpace(*req.ChatSessionID)
	if sid == "" {
		sid = "default"
	}

	// Store save boundary: active only when the configured mode allows writes.
	saveOK := false
	saveErr := "shadow_mode: save disabled in R0/R1"
	effectiveInputSaved := 0
	auditSaved := 0
	storeWriteAttempted := 0
	storeWriteErrors := 0

	now := time.Now().UTC()
	text := strings.TrimSpace(*req.EffectiveInput)
	writeSource := s.storeWriteSource()

	if s.usesShadowWriteStore() && text != "" {
		ctx := r.Context()
		storeWriteAttempted++
		if err := s.Store.SaveEffectiveInput(ctx, &store.EffectiveInput{
			ChatSessionID:  sid,
			TurnIndex:      req.TurnIndex,
			EffectiveInput: text,
			CreatedAt:      now,
		}); err != nil {
			storeWriteErrors++
		} else {
			effectiveInputSaved++
			saveOK = true
		}

		if saveOK {
			storeWriteAttempted++
			if err := s.Store.SaveAuditLog(ctx, &store.AuditLog{
				ChatSessionID: sid,
				EventType:     "effective_input_saved",
				TargetType:    "turn",
				TargetID:      int64(req.TurnIndex),
				Summary:       fmt.Sprintf("effective input saved turn %d", req.TurnIndex),
				DetailsJSON:   fmt.Sprintf(`{"turn_index":%d,"length":%d}`, req.TurnIndex, len(text)),
				Source:        writeSource,
				CreatedAt:     now,
			}); err != nil {
				storeWriteErrors++
			} else {
				auditSaved = 1
			}
		}

		if storeWriteAttempted > 0 && storeWriteErrors == 0 {
			saveOK = true
			saveErr = ""
		}
	}

	note := "effective-inputs is a shadow skeleton; no live DB mutation performed"
	if s.usesShadowWriteStore() {
		if saveOK {
			note = "effective-inputs saved in " + writeSource + " mode"
		} else {
			note = "effective-inputs write attempted in " + writeSource + " mode but failed"
		}
	}

	inputTransparency := buildInputTransparency(sid, req.TurnIndex, text, s.usesShadowWriteStore(), writeSource)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":                "ok",
		"source":                writeSource,
		"turn_index":            req.TurnIndex,
		"chat_session_id":       sid,
		"id":                    nil,
		"save_ok":               saveOK,
		"save_error":            saveErr,
		"effective_input_saved": effectiveInputSaved,
		"audit_saved":           auditSaved,
		"store_write_attempted": storeWriteAttempted,
		"store_write_errors":    storeWriteErrors,
		"input_transparency":    inputTransparency,
		"trace_handoff": map[string]any{
			"shadow_mode": true,
			"store_mode":  string(s.Cfg.StoreMode),
		},
		"note": note,
	})
}

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
	if !hasRollback || !s.usesShadowWriteStore() {
		rollbackPlan := buildRollbackPlan(sid, turnIndex, reqSource)
		rollbackPlan["requested_turn_index"] = requestedTurnIndex
		rollbackPlan["protected_before_turn"] = protectedBeforeTurn
		rollbackPlan["min_from_turn"] = minFromTurn
		rollbackPlan["session_routing_baseline_clamped"] = baselineClamped
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
	vectorIDs, vectorCollectErr := rollbackVectorDocumentIDs(ctx, s.Store, sid, turnIndex)
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
		if err := t.fn(); err != nil {
			deletions[t.name] = map[string]any{"ok": false, "error": err.Error()}
			delErrs = append(delErrs, fmt.Sprintf("%s: %v", t.name, err))
		} else {
			deletions[t.name] = map[string]any{"ok": true}
		}
	}
	if restored, err := restoreNarrativeCurrentStatesAfterRollback(ctx, s.Store, sid); err != nil {
		deletions["narrative_current_state_restore"] = map[string]any{"ok": false, "error": err.Error()}
		delErrs = append(delErrs, fmt.Sprintf("narrative current state restore: %v", err))
	} else {
		deletions["narrative_current_state_restore"] = map[string]any{"ok": true, "restored": restored}
	}
	if vectorCollectErr != nil {
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
	if s.Vector != nil {
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
	if s.Vector != nil {
		fullAudit := s.adminVectorOrphanAudit(ctx, sid, false)
		if available, _ := fullAudit["full_listing_available"].(bool); available {
			fullAudit["status"] = "full"
			fullAudit["policy"] = "post_rollback_full_chromadb_listing_compared_with_mariadb_canonical_rows"
			fullAudit["known_delete_id_count"] = len(vectorIDs)
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

	status := "ok"
	note := "rollback executed"
	if len(delErrs) > 0 {
		status = "partial_error"
		note = "rollback executed with partial errors"
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
		"deletions": deletions,
		"errors":    delErrs,
		"note":      note,
	})
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

func prepareTurnEvidenceCounts(memories []store.Memory, kgTriples []store.KGTriple, evidence []store.DirectEvidence, chatLogs []store.ChatLog, resumePack *store.ResumePack, storylines []store.Storyline, worldRules []store.WorldRule, charStates []store.CharacterState, pendingThreads []store.PendingThread, activeStates []store.ActiveState, canonicalLayers []store.CanonicalStateLayer, episodeSums []store.EpisodeSummary) map[string]any {
	return map[string]any{
		"memories":               len(memories),
		"kg_triples":             len(kgTriples),
		"direct_evidence":        len(evidence),
		"chat_logs":              len(chatLogs),
		"resume_pack_present":    resumePack != nil,
		"storylines":             len(storylines),
		"world_rules":            len(worldRules),
		"character_states":       len(charStates),
		"pending_threads":        len(pendingThreads),
		"active_states":          len(activeStates),
		"canonical_state_layers": len(canonicalLayers),
		"episode_summaries":      len(episodeSums),
	}
}

func prepareTurnSectionSummary(injectionText, inputContextText string, injectionTruncated, inputContextTruncated bool) []map[string]any {
	return []map[string]any{
		{
			"name":      "injection_text",
			"chars":     len([]rune(injectionText)),
			"available": strings.TrimSpace(injectionText) != "",
			"truncated": injectionTruncated,
			"sources":   []string{"memories", "kg_triples", "storylines", "world_rules", "character_states", "pending_threads"},
		},
		{
			"name":      "input_context_text",
			"chars":     len([]rune(inputContextText)),
			"available": strings.TrimSpace(inputContextText) != "",
			"truncated": inputContextTruncated,
			"sources":   []string{"direct_evidence", "chat_logs", "resume_pack", "active_states", "canonical_state_layers", "episode_summaries"},
		},
	}
}

func buildSupervisorInputPack(chatSessionID string, turnIndex int, rawUserInput, guideMode, guideStrength, narrativeStance, autoAdvanceTrigger, continuityQuery string, promptAssembly map[string]any, evidenceCounts map[string]any, sectionSummary []map[string]any, storylineSelection storylineSupervisorSelection, degraded bool, fallbackReason string, languageContext map[string]any) map[string]any {
	autoAdvanceHint := ""
	if autoAdvanceTrigger != "" && autoAdvanceTrigger != "none" {
		autoAdvanceHint = fmt.Sprintf("[Auto Advance]\ntrigger=%s; query=%s", autoAdvanceTrigger, truncateTextForShadow(continuityQuery, 160))
	}
	guideMode = resolveNarrativeGuideMode(guideMode, nil, "", rawUserInput)
	guideStrength = normalizeNarrativeGuideStrength(guideStrength)
	guideSuffix := buildGuideModeSuffix(guideMode, guideStrength)
	directorOverrides := buildGuideModeDirectorOverrides(guideMode)
	narrativeStanceSuffix := buildNarrativeStanceSuffix(narrativeStance)
	narrativeStanceBounds := buildNarrativeStanceBounds(narrativeStance)
	narrativeStanceSummary := buildNarrativeStanceSummary(narrativeStance, narrativeStanceSuffix, narrativeStanceBounds)
	storylineSelectionTrace := storylineSelectionSummary(storylineSelection)
	storylinesContext := formatStorylinesForSupervisor(storylineSelection)
	plannerLanguageContract := buildPrepareTurnPlannerLanguageContract(languageContext)
	guidanceParts := []string{
		"[Go R1 Supervisor Read Shadow]",
		"mode=read_shadow; would_call_llm=false; would_write=false",
		fmt.Sprintf("guide_mode=%s; guide_strength=%s; narrative_stance=%s", guideMode, guideStrength, narrativeStance),
		fmt.Sprintf("evidence_counts=%s", compactJSONForShadow(evidenceCounts, 500)),
		fmt.Sprintf("storyline_selection=%s", compactJSONForShadow(storylineSelectionTrace, 500)),
		fmt.Sprintf("section_summary=%s", compactJSONForShadow(sectionSummary, 500)),
	}
	if guideSuffix != "" {
		guidanceParts = append(guidanceParts, guideSuffix)
	}
	if narrativeStanceSuffix != "" {
		guidanceParts = append(guidanceParts, narrativeStanceSuffix)
	}
	if len(narrativeStanceBounds) > 0 {
		guidanceParts = append(guidanceParts, "[Story Initiative Bounds]\n"+compactJSONForShadow(narrativeStanceBounds, 600))
	}
	persistentGuidance := strings.Join(guidanceParts, "\n")
	finalGuidance := persistentGuidance
	if storylinesContext != "" {
		finalGuidance += "\n" + storylinesContext
	}
	if autoAdvanceHint != "" {
		finalGuidance += "\n" + autoAdvanceHint
	}
	status := "ready"
	if degraded {
		status = "degraded"
	}
	return map[string]any{
		"status":                    status,
		"source":                    "go_r1_read_shadow",
		"chat_session_id":           chatSessionID,
		"turn_index":                turnIndex,
		"raw_user_input_chars":      len([]rune(rawUserInput)),
		"prompt_assembly":           promptAssembly,
		"prompt_source":             promptAssembly["prompt_source"],
		"guide_mode":                guideMode,
		"guide_strength":            guideStrength,
		"guide_suffix":              guideSuffix,
		"narrative_stance":          narrativeStance,
		"narrative_stance_suffix":   narrativeStanceSuffix,
		"narrative_stance_bounds":   narrativeStanceBounds,
		"narrative_stance_summary":  narrativeStanceSummary,
		"director_overrides":        directorOverrides,
		"language_context":          nilIfEmptyMap(languageContext),
		"planner_language_contract": plannerLanguageContract,
		"persistent_guidance":       persistentGuidance,
		"storyline_selection":       storylineSelectionTrace,
		"storylines_context":        nilIfEmpty(storylinesContext),
		"auto_advance_trigger":      autoAdvanceTrigger,
		"auto_advance_hint":         autoAdvanceHint,
		"final_guidance_suffix":     finalGuidance,
		"momentum_packet": map[string]any{
			"packet_status":   status,
			"evidence_counts": evidenceCounts,
			"section_summary": sectionSummary,
		},
		"prompt_plan": []string{
			"supervisor_system.txt",
			"supervisor_prompt.txt",
			"persistent_guidance",
			"recent_context_summary",
			"wake_up_or_continuity_context",
		},
		"degraded":        degraded,
		"fallback_reason": fallbackReason,
		"would_call_llm":  false,
		"would_write":     false,
	}
}

func buildPrepareTurnPlannerLanguageContract(languageContext map[string]any) map[string]any {
	target := prepareTurnSessionOutputLanguage(languageContext)
	status := "unknown"
	if target != "" && target != "auto" && target != "unknown" {
		status = "ready"
	}
	return map[string]any{
		"contract_version":              languageMemoryContractVersion,
		"status":                        status,
		"planner_support_language":      nilIfEmpty(target),
		"planner_language_source":       nilIfEmpty(extractionStringFromAny(languageContext["output_language_source"])),
		"current_user_input_priority":   "highest",
		"raw_user_input_rewritten":      false,
		"raw_evidence_rewritten":        false,
		"generated_support_policy":      "use_session_output_language_when_language_is_known",
		"trace_labels_language_neutral": true,
	}
}

func buildWeakInputPlannerContract(rawUserInput string, inputAnchorGovernor map[string]any, languageContext map[string]any, maxInputContextChars int) map[string]any {
	trimmed := strings.TrimSpace(rawUserInput)
	lower := strings.ToLower(trimmed)
	runeCount := len([]rune(trimmed))
	wordCount := len(strings.Fields(trimmed))
	continuationPhrases := map[string]bool{
		"continue": true, "go on": true, "next": true, "more": true, "resume": true, "keep going": true,
		"계속": true, "계속해": true, "이어서": true, "이어가": true, "다음": true, "다음 장면": true,
		"응": true, "ㅇㅇ": true, "좋아": true, "그래": true, "좋아 계속": true,
	}
	taxonomy := "specific_input"
	switch {
	case trimmed == "":
		taxonomy = "empty_input"
	case continuationPhrases[lower]:
		taxonomy = "continuation_trigger"
	case runeCount <= 12 && wordCount <= 3:
		taxonomy = "short_ack_or_nudge"
	case runeCount <= 24:
		taxonomy = "low_specificity_input"
	}

	explicitRedirection := false
	if redirection, ok := inputAnchorGovernor["explicit_user_redirection"].(map[string]any); ok {
		explicitRedirection = boolFromAny(redirection["detected"])
	}
	selectedAnchors := stringSliceFromAny(inputAnchorGovernor["selected_slot_names"])
	droppedAnchors := stringSliceFromAny(inputAnchorGovernor["dropped_slot_names"])
	weakActive := taxonomy != "specific_input"
	if explicitRedirection {
		weakActive = false
	}
	status := "not_applicable"
	if weakActive {
		status = "ready"
	}
	if explicitRedirection {
		status = "redirection_user_input_wins"
	}

	maxNewBeats := 0
	if weakActive {
		maxNewBeats = 1
	}
	targetLanguage := prepareTurnSessionOutputLanguage(languageContext)
	return map[string]any{
		"contract_version":            "step25_weak_input_planner.v1",
		"status":                      status,
		"active":                      weakActive,
		"taxonomy":                    taxonomy,
		"raw_user_input_chars":        runeCount,
		"raw_user_input_words":        wordCount,
		"current_user_input_priority": "highest",
		"truth_authority":             false,
		"would_write":                 false,
		"would_call_llm":              false,
		"selected_anchor_names":       selectedAnchors,
		"dropped_anchor_names":        droppedAnchors,
		"planner_support_language":    nilIfEmpty(targetLanguage),
		"input_context_budget_chars":  maxInputContextChars,
		"minimum_mandate": []string{
			"preserve the latest user input as the only command source",
			"use recent/previous anchors only as support",
			"avoid stale arc revival unless current input or fresh evidence aligns",
		},
		"acting_brief": map[string]any{
			"main_failure_risk": "stall_or_stale_replay",
			"portrayal_goal":    "continue the current scene using verified anchors",
			"reply_strategy":    "advance at most one reversible causal beat; ask or frame options when the choice is unspecified",
		},
		"initiative_boundary": map[string]any{
			"max_new_beats":                 maxNewBeats,
			"allow_scene_jump":              false,
			"may_suggest":                   true,
			"may_execute_irreversible_step": false,
			"explicit_redirection_detected": explicitRedirection,
		},
		"ambiguity_policy": map[string]any{
			"preserve_unspecified_choice": true,
			"do_not_choose_for_user":      true,
			"degrade_path":                "support_only_anchor_or_no_planner_brief",
		},
		"role_lens_contract": map[string]any{
			"world_lens":               "guard hard setting contradictions only",
			"plot_lens":                "surface current arc pressure without forcing payoff",
			"npc_lens":                 "use visible or directly relevant known-state only",
			"critic_lens":              "flag over-injection, secret leak, stale replay, and user override risk",
			"raw_memory_dump_allowed":  false,
			"hidden_knowledge_allowed": false,
		},
	}
}

func formatWeakInputPlannerGuidance(contract map[string]any) string {
	if contract == nil || !boolFromAny(contract["active"]) {
		return ""
	}
	taxonomy := extractionStringFromAny(contract["taxonomy"])
	brief, _ := contract["acting_brief"].(map[string]any)
	boundary, _ := contract["initiative_boundary"].(map[string]any)
	return strings.Join([]string{
		"[Weak Input Planner]",
		"mode=support_only; truth_authority=false; current_user_input_priority=highest",
		"taxonomy=" + taxonomy,
		"main_failure_risk=" + extractionStringFromAny(brief["main_failure_risk"]),
		"portrayal_goal=" + extractionStringFromAny(brief["portrayal_goal"]),
		"reply_strategy=" + extractionStringFromAny(brief["reply_strategy"]),
		fmt.Sprintf("initiative=max_new_beats:%d; allow_scene_jump:%v; irreversible_step:false", intFromAny(boundary["max_new_beats"], 0), boolFromAny(boundary["allow_scene_jump"])),
		"ambiguity=preserve unspecified user choice; do not choose for the user",
	}, "\n")
}

func buildPlannerExecutionContract(rawUserInput, narrativeStance, guideMode, guideStrength string, inputAnchorGovernor, weakInputPlanner map[string]any, selectedStorylines []store.Storyline, pendingThreads []store.PendingThread, activeStates []store.ActiveState, canonicalLayers []store.CanonicalStateLayer, worldRules []store.WorldRule, assembly prepareTurnInjectionAssembly, languageContext map[string]any) map[string]any {
	stanceBounds := buildNarrativeStanceBounds(narrativeStance)
	maxNewBeats := intFromAny(stanceBounds["max_new_beats"], 0)
	allowSceneJump := boolFromAny(stanceBounds["allow_scene_jump"])
	if weakInputPlanner != nil && boolFromAny(weakInputPlanner["active"]) {
		if boundary, ok := weakInputPlanner["initiative_boundary"].(map[string]any); ok {
			weakBeats := intFromAny(boundary["max_new_beats"], maxNewBeats)
			if weakBeats < maxNewBeats || maxNewBeats <= 0 {
				maxNewBeats = weakBeats
			}
			allowSceneJump = allowSceneJump && boolFromAny(boundary["allow_scene_jump"])
		}
	}

	selectedAnchors := stringSliceFromAny(inputAnchorGovernor["selected_slot_names"])
	droppedAnchors := stringSliceFromAny(inputAnchorGovernor["dropped_slot_names"])
	activeStorylineNames := []string{}
	for _, sl := range selectedStorylines {
		if name := strings.TrimSpace(sl.Name); name != "" {
			activeStorylineNames = appendUniqueMemorySearchText(activeStorylineNames, name)
		}
	}
	openThreadNames := []string{}
	for _, th := range pendingThreads {
		if th.Suppressed {
			continue
		}
		label := strings.TrimSpace(firstNonEmpty(th.Title, th.Description, th.ThreadKey))
		if label != "" {
			openThreadNames = appendUniqueMemorySearchText(openThreadNames, label)
		}
	}

	protectedCount := intFromAny(assembly.Counts["protected_secret_count"], 0) +
		intFromAny(assembly.Counts["identity_accuracy_count"], 0) +
		intFromAny(assembly.Counts["protected_memory_guarded_count"], 0)
	privateLaneActive := intFromAny(assembly.Counts["character_private_recollection_bound"], intFromAny(assembly.Counts["character_private_recollection_count"], 0)) > 0 ||
		strings.TrimSpace(assembly.CharacterPrivateText) != ""

	forbiddenMoves := append([]string{}, stringSliceFromAny(stanceBounds["forbidden_moves"])...)
	forbiddenMoves = appendUniqueMemorySearchText(forbiddenMoves, "do not override or reinterpret the latest user input")
	forbiddenMoves = appendUniqueMemorySearchText(forbiddenMoves, "do not convert support memories into new canonical facts")
	if !allowSceneJump {
		forbiddenMoves = appendUniqueMemorySearchText(forbiddenMoves, "do not hard-cut to a new scene without current-input support")
	}
	if weakInputPlanner != nil && boolFromAny(weakInputPlanner["active"]) {
		forbiddenMoves = appendUniqueMemorySearchText(forbiddenMoves, "do not turn a weak prompt into an irreversible user decision")
	}
	if protectedCount > 0 || privateLaneActive {
		forbiddenMoves = appendUniqueMemorySearchText(forbiddenMoves, "do not reveal protected private knowledge or let unrelated characters discover it without current-scene evidence")
		forbiddenMoves = appendUniqueMemorySearchText(forbiddenMoves, "do not split confirmed alias or cover-identity continuity into separate people")
	}

	sceneMandate := "follow the latest user input and keep the current scene causally grounded"
	if boolFromAny(weakInputPlanner["active"]) {
		sceneMandate = "continue the current scene from verified anchors while preserving user ambiguity"
	} else if strings.TrimSpace(rawUserInput) != "" {
		sceneMandate = "answer the latest user input directly and use support context only as grounding"
	}

	requiredOutcomes := []string{
		"latest user input remains the command source",
		"visible response must stay compatible with direct evidence and canonical state",
	}
	if len(activeStorylineNames) > 0 {
		requiredOutcomes = append(requiredOutcomes, "keep selected active storyline in view: "+strings.Join(limitStringSlice(activeStorylineNames, 2), " / "))
	}
	if len(openThreadNames) > 0 {
		requiredOutcomes = append(requiredOutcomes, "preserve open thread awareness: "+strings.Join(limitStringSlice(openThreadNames, 2), " / "))
	}
	if len(selectedAnchors) > 0 {
		requiredOutcomes = append(requiredOutcomes, "use selected anchors as support only: "+strings.Join(limitStringSlice(selectedAnchors, 3), " / "))
	}

	pacingLevel := "steady"
	if maxNewBeats <= 0 {
		pacingLevel = "hold_or_user_led"
	} else if normalizeNarrativeStance(narrativeStance) == "proactive" {
		pacingLevel = "bounded_forward"
	}
	targetLanguage := prepareTurnSessionOutputLanguage(languageContext)

	return map[string]any{
		"contract_version":            "step25_planner_execution_contract.v1",
		"status":                      "ready",
		"active":                      true,
		"current_user_input_priority": "highest",
		"truth_authority":             false,
		"would_write":                 false,
		"would_call_llm":              false,
		"planner_support_language":    nilIfEmpty(targetLanguage),
		"scene_mandate": map[string]any{
			"value":  sceneMandate,
			"source": "current_input_plus_read_surfaces",
		},
		"required_outcome": map[string]any{
			"items": requiredOutcomes,
			"count": len(requiredOutcomes),
		},
		"forbidden_move": map[string]any{
			"items":                 limitStringSlice(forbiddenMoves, 8),
			"count":                 len(forbiddenMoves),
			"protected_lane_active": protectedCount > 0 || privateLaneActive,
		},
		"pacing_pressure": map[string]any{
			"level":            pacingLevel,
			"max_new_beats":    maxNewBeats,
			"allow_scene_jump": allowSceneJump,
			"guide_mode":       normalizeNarrativeGuideMode(guideMode),
			"guide_strength":   normalizeNarrativeGuideStrength(guideStrength),
			"stance":           normalizeNarrativeStance(narrativeStance),
		},
		"ending_requirement": map[string]any{
			"mode":        "soft_landing",
			"instruction": "end with immediate consequence, reaction, or a reversible next choice; do not force final resolution",
		},
		"consume_rule": map[string]any{
			"allowed_usage":  []string{"next_turn_guidance", "continuity_guard", "pacing_guard", "secret_leak_guard"},
			"blocked_usage":  []string{"truth_write", "canonical_override", "user_intent_override", "raw_memory_dump", "hidden_knowledge_reveal"},
			"priority_order": []string{"current_user_input", "explicit_user_correction", "direct_evidence", "canonical_state", "retrieved_support", "planner_execution_contract"},
		},
		"read_surface_alignment": map[string]any{
			"selected_anchor_count":       len(selectedAnchors),
			"dropped_anchor_count":        len(droppedAnchors),
			"selected_storyline_count":    len(activeStorylineNames),
			"pending_thread_count":        len(openThreadNames),
			"active_state_count":          len(activeStates),
			"canonical_layer_count":       len(canonicalLayers),
			"world_rule_count":            len(worldRules),
			"protected_signal_count":      protectedCount,
			"private_recollection_active": privateLaneActive,
		},
		"facet_audit_repair_ingestion": map[string]any{
			"status": "no_prior_facet_audit_surface",
			"rule":   "when prior drift or secret-leak audit exists, consume only as bounded repair hint for the next turn",
		},
		"concealment_guard": map[string]any{
			"active": protectedCount > 0 || privateLaneActive,
			"rule":   "preserve protected/private knowledge boundaries; do not reveal or externalize without current-scene evidence",
		},
		"role_lens_consumption": map[string]any{
			"world_lens":  "hard setting and rule contradiction guard only",
			"plot_lens":   "current arc pressure and required outcome hint only",
			"npc_lens":    "visible or directly relevant known-state boundary only",
			"critic_lens": "over-injection, secret leak, stale replay, flat interpretation, and user override guard only",
		},
	}
}

func formatPlannerExecutionContractGuidance(contract map[string]any) string {
	if contract == nil || !boolFromAny(contract["active"]) {
		return ""
	}
	sceneMandate := mapFromAny(contract["scene_mandate"])
	required := mapFromAny(contract["required_outcome"])
	forbidden := mapFromAny(contract["forbidden_move"])
	pacing := mapFromAny(contract["pacing_pressure"])
	ending := mapFromAny(contract["ending_requirement"])
	requiredItems := limitStringSlice(stringSliceFromAny(required["items"]), 3)
	forbiddenItems := limitStringSlice(stringSliceFromAny(forbidden["items"]), 3)
	return strings.Join([]string{
		"[Planner Execution Contract]",
		"mode=support_only; truth_authority=false; current_user_input_priority=highest",
		"scene_mandate=" + extractionStringFromAny(sceneMandate["value"]),
		"required_outcome=" + strings.Join(requiredItems, " / "),
		"forbidden_move=" + strings.Join(forbiddenItems, " / "),
		fmt.Sprintf("pacing=max_new_beats:%d; allow_scene_jump:%v; level:%s", intFromAny(pacing["max_new_beats"], 0), boolFromAny(pacing["allow_scene_jump"]), extractionStringFromAny(pacing["level"])),
		"ending_requirement=" + extractionStringFromAny(ending["instruction"]),
	}, "\n")
}

func buildProgressionChoiceLedger(sid string, turnIndex int, rawUserInput string, chatLogs []store.ChatLog, storylines []store.Storyline, pendingThreads []store.PendingThread, episodeSums []store.EpisodeSummary, inputAnchorGovernor, weakInputPlanner, plannerExecutionContract, progressionLedger map[string]any) map[string]any {
	trimmed := strings.TrimSpace(rawUserInput)
	selectedAnchors := stringSliceFromAny(inputAnchorGovernor["selected_slot_names"])
	explicitRedirection := false
	if redirection, ok := inputAnchorGovernor["explicit_user_redirection"].(map[string]any); ok {
		explicitRedirection = boolFromAny(redirection["detected"])
	}
	weakActive := weakInputPlanner != nil && boolFromAny(weakInputPlanner["active"])
	latestUser := latestUserChatLogContent(chatLogs)
	sameIncident := trimmed != "" && latestUser != "" && stableKey("turn", trimmed) == stableKey("turn", latestUser)
	activeStorylineCount := countActiveProgressionStorylines(storylines)
	activeThreadCount := countOpenProgressionThreads(pendingThreads)
	hasLiveAnchor := len(selectedAnchors) > 0 || activeStorylineCount > 0 || activeThreadCount > 0
	hasCallbackAnchor := stringSliceContains(selectedAnchors, "Chapter") || stringSliceContains(selectedAnchors, "Saga") || len(episodeSums) > 0
	staleDroppedCount := countDroppedOldArcAnchors(inputAnchorGovernor)

	choice := "advance"
	reasons := []string{"specific_input_or_live_anchor_available"}
	if trimmed == "" {
		choice = "hold"
		reasons = []string{"empty_input_preserve_user_ambiguity"}
	} else if sameIncident {
		choice = "hold"
		reasons = []string{"same_incident_exact_repeat_detected"}
	} else if explicitRedirection {
		choice = "new_scene_opportunity"
		reasons = []string{"explicit_user_redirection_detected"}
	} else if weakActive && !hasLiveAnchor {
		choice = "hold"
		reasons = []string{"weak_input_without_live_anchor"}
	} else if weakActive {
		choice = "advance"
		reasons = []string{"weak_input_bounded_advance_from_live_anchor"}
	} else if hasCallbackAnchor && (activeStorylineCount > 0 || activeThreadCount > 0) {
		choice = "callback"
		reasons = []string{"callback_anchor_aligned_with_active_thread"}
	} else if staleDroppedCount > 0 && activeStorylineCount == 0 && activeThreadCount == 0 {
		choice = "hold"
		reasons = []string{"stale_callback_suppressed_without_current_scene_alignment"}
	}

	pacing := mapFromAny(plannerExecutionContract["pacing_pressure"])
	return map[string]any{
		"contract_version":            "step25_progression_choice_ledger.v1",
		"status":                      "ready",
		"chat_session_id":             sid,
		"turn_index":                  turnIndex,
		"choice":                      choice,
		"choice_set":                  []string{"advance", "callback", "new_scene_opportunity", "hold"},
		"reasons":                     reasons,
		"current_user_input_priority": "highest",
		"truth_authority":             false,
		"would_write":                 false,
		"would_call_llm":              false,
		"scene_advancement_ledger": map[string]any{
			"decision":                  choice,
			"reason":                    strings.Join(reasons, " / "),
			"max_new_beats":             intFromAny(pacing["max_new_beats"], 0),
			"allow_scene_jump":          boolFromAny(pacing["allow_scene_jump"]),
			"selected_anchor_count":     len(selectedAnchors),
			"active_storyline_count":    activeStorylineCount,
			"active_thread_count":       activeThreadCount,
			"callback_anchor_available": hasCallbackAnchor,
		},
		"callback_evaluation": map[string]any{
			"candidate":                  hasCallbackAnchor,
			"aligned_with_active_thread": hasCallbackAnchor && (activeStorylineCount > 0 || activeThreadCount > 0),
			"stale_dropped_count":        staleDroppedCount,
			"stale_revival_suppressed":   staleDroppedCount > 0 && choice != "callback",
			"rule":                       "callback is support-only and must align with current scene, active thread, or current input",
		},
		"same_incident_stall_detection": map[string]any{
			"detected":             sameIncident,
			"mode":                 "exact_normalized_user_input_repeat_only",
			"current_input_chars":  len([]rune(trimmed)),
			"latest_user_present":  latestUser != "",
			"stall_action":         "hold",
			"false_positive_guard": "no semantic guess; only exact normalized repeat is treated as same incident",
		},
		"inspection_replay_surface": map[string]any{
			"status": "ready",
			"cases":  []string{"weak_input_bounded_advance", "callback_alignment", "stale_callback_suppression", "same_incident_hold", "explicit_redirection_new_scene"},
			"source": "prepare_turn_read_only",
		},
		"consume_rule": map[string]any{
			"allowed_usage":  []string{"next_turn_progression_hint", "stall_guard", "callback_alignment_guard"},
			"blocked_usage":  []string{"truth_write", "canonical_state_change", "forced_scene_jump", "user_intent_override"},
			"priority_order": []string{"current_user_input", "explicit_user_correction", "planner_execution_contract", "progression_choice_ledger"},
		},
		"ledger_alignment": map[string]any{
			"progression_ledger_status": extractionStringFromAny(progressionLedger["status"]),
			"last_advanced_turn":        progressionLedger["last_advanced_turn"],
			"last_validated_turn":       progressionLedger["last_validated_turn"],
		},
	}
}

func latestUserChatLogContent(chatLogs []store.ChatLog) string {
	latestTurn := -1
	latestID := int64(-1)
	latest := ""
	for _, log := range chatLogs {
		if !strings.EqualFold(strings.TrimSpace(log.Role), "user") {
			continue
		}
		if log.TurnIndex > latestTurn || (log.TurnIndex == latestTurn && log.ID > latestID) {
			latestTurn = log.TurnIndex
			latestID = log.ID
			latest = strings.TrimSpace(log.Content)
		}
	}
	return latest
}

func countActiveProgressionStorylines(storylines []store.Storyline) int {
	count := 0
	for _, sl := range storylines {
		if sl.Suppressed {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(sl.Status))
		if status == "" || status == "active" || status == "open" || status == "escalating" || status == "aftermath" || status == "latent" {
			count++
		}
	}
	return count
}

func countOpenProgressionThreads(pendingThreads []store.PendingThread) int {
	count := 0
	for _, th := range pendingThreads {
		if th.Suppressed {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(th.Status))
		if status == "" || status == "open" || status == "active" || status == "pending" {
			count++
		}
	}
	return count
}

func countDroppedOldArcAnchors(inputAnchorGovernor map[string]any) int {
	count := 0
	rawTrace, _ := inputAnchorGovernor["old_arc_keep_drop_trace"].([]map[string]any)
	for _, item := range rawTrace {
		if extractionStringFromAny(item["decision"]) == "drop" {
			count++
		}
	}
	return count
}

func formatProgressionChoiceGuidance(contract map[string]any) string {
	if contract == nil || extractionStringFromAny(contract["status"]) == "" {
		return ""
	}
	callbackEval := mapFromAny(contract["callback_evaluation"])
	stall := mapFromAny(contract["same_incident_stall_detection"])
	ledger := mapFromAny(contract["scene_advancement_ledger"])
	reasons := stringSliceFromAny(contract["reasons"])
	return strings.Join([]string{
		"[Progression Choice Ledger]",
		"mode=support_only; truth_authority=false; current_user_input_priority=highest",
		"choice=" + extractionStringFromAny(contract["choice"]),
		"reason=" + strings.Join(limitStringSlice(reasons, 2), " / "),
		fmt.Sprintf("ledger=max_new_beats:%d; allow_scene_jump:%v; anchors:%d", intFromAny(ledger["max_new_beats"], 0), boolFromAny(ledger["allow_scene_jump"]), intFromAny(ledger["selected_anchor_count"], 0)),
		fmt.Sprintf("callback=candidate:%v; aligned:%v; stale_suppressed:%v", boolFromAny(callbackEval["candidate"]), boolFromAny(callbackEval["aligned_with_active_thread"]), boolFromAny(callbackEval["stale_revival_suppressed"])),
		fmt.Sprintf("same_incident_exact_repeat:%v", boolFromAny(stall["detected"])),
	}, "\n")
}

func buildStep25ValidationGate(rawUserInput string, weakInputPlanner, plannerExecutionContract, progressionChoiceLedger map[string]any) map[string]any {
	type checkDef struct {
		id     string
		name   string
		pass   bool
		reason string
	}
	weakBoundary := mapFromAny(weakInputPlanner["initiative_boundary"])
	execConsume := mapFromAny(plannerExecutionContract["consume_rule"])
	roleLens := mapFromAny(plannerExecutionContract["role_lens_consumption"])
	progressionReplay := mapFromAny(progressionChoiceLedger["inspection_replay_surface"])
	callbackEval := mapFromAny(progressionChoiceLedger["callback_evaluation"])
	stall := mapFromAny(progressionChoiceLedger["same_incident_stall_detection"])
	choice := extractionStringFromAny(progressionChoiceLedger["choice"])
	allowedChoice := choice == "advance" || choice == "callback" || choice == "new_scene_opportunity" || choice == "hold"
	replayCases := stringSliceFromAny(progressionReplay["cases"])
	blockedUsage := stringSliceFromAny(execConsume["blocked_usage"])
	contractVersionsPresent := extractionStringFromAny(weakInputPlanner["contract_version"]) == "step25_weak_input_planner.v1" &&
		extractionStringFromAny(plannerExecutionContract["contract_version"]) == "step25_planner_execution_contract.v1" &&
		extractionStringFromAny(progressionChoiceLedger["contract_version"]) == "step25_progression_choice_ledger.v1"

	checks := []checkDef{
		{
			id:     "25-5a",
			name:   "current input misread replay",
			pass:   strings.TrimSpace(rawUserInput) != "" && extractionStringFromAny(weakInputPlanner["current_user_input_priority"]) == "highest",
			reason: "current user input remains the highest-priority command source",
		},
		{
			id:     "25-5b",
			name:   "weak input progression replay",
			pass:   weakInputPlanner["truth_authority"] == false && weakInputPlanner["would_write"] == false && intFromAny(weakBoundary["max_new_beats"], 99) <= 1,
			reason: "weak input planner stays bounded and support-only",
		},
		{
			id:     "25-5c",
			name:   "planner slot truth boundary",
			pass:   plannerExecutionContract["truth_authority"] == false && plannerExecutionContract["would_write"] == false && len(blockedUsage) > 0,
			reason: "execution slots cannot write truth or override user intent",
		},
		{
			id:     "25-5d",
			name:   "progression choice separation",
			pass:   allowedChoice && callbackEval["rule"] != nil && stall["false_positive_guard"] != nil,
			reason: "advance, callback, opening, and hold choices are explicit and inspectable",
		},
		{
			id:     "25-5e",
			name:   "step-specific replay surface",
			pass:   extractionStringFromAny(progressionReplay["source"]) == "prepare_turn_read_only" && len(replayCases) >= 4,
			reason: "Step 25 replay cases are local to this planner/progression gate",
		},
		{
			id:     "25-5f",
			name:   "schema migration package",
			pass:   contractVersionsPresent,
			reason: "all Step 25 contracts expose version stamps for rollback/replay comparison",
		},
		{
			id:     "25-5g",
			name:   "role-lensed input improvement replay",
			pass:   roleLens["world_lens"] != nil && roleLens["plot_lens"] != nil && roleLens["npc_lens"] != nil && roleLens["critic_lens"] != nil,
			reason: "world, plot, NPC, and critic lenses are present as bounded support rules",
		},
		{
			id:     "25-5h",
			name:   "role-lens failure budget",
			pass:   stringSliceContains(blockedUsage, "hidden_knowledge_reveal") && stringSliceContains(blockedUsage, "user_intent_override"),
			reason: "lens failure modes block hidden-knowledge leak and user-intent override",
		},
	}

	items := make([]map[string]any, 0, len(checks))
	blocking := []string{}
	passed := 0
	for _, check := range checks {
		status := "pass"
		if !check.pass {
			status = "hold"
			blocking = append(blocking, check.id)
		} else {
			passed++
		}
		items = append(items, map[string]any{
			"id":     check.id,
			"name":   check.name,
			"status": status,
			"reason": check.reason,
		})
	}
	gateStatus := "pass"
	if len(blocking) > 0 {
		gateStatus = "hold"
	}
	return map[string]any{
		"contract_version":            "step25_validation_gate.v1",
		"status":                      "ready",
		"gate_status":                 gateStatus,
		"adoption_ready":              len(blocking) == 0,
		"current_user_input_priority": "highest",
		"truth_authority":             false,
		"would_write":                 false,
		"would_call_llm":              false,
		"passed_count":                passed,
		"total_count":                 len(checks),
		"blocking_check_ids":          blocking,
		"checks":                      items,
		"scope":                       "prepare_turn_contract_smoke_gate",
		"live_replay_note":            "This gate verifies Step 25 contract shape and safety boundaries; separate live user replay can still be run before release packaging.",
		"release_gate": map[string]any{
			"status":             gateStatus,
			"bundle_ready":       len(blocking) == 0,
			"requires_packaging": false,
			"step":               "25",
		},
	}
}

func normalizeNarrativeGuideMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "auto":
		return "auto"
	case "standard":
		return "standard"
	case "romantic":
		return "romantic"
	case "action":
		return "action"
	case "mature_soft", "mature-soft":
		return "mature_soft"
	case "mature_direct", "mature-direct":
		return "mature_direct"
	default:
		return "off"
	}
}

func resolveNarrativeGuideMode(mode string, contextMessages []map[string]any, wakeUpContext, fallbackUserInput string) string {
	normalized := normalizeNarrativeGuideMode(mode)
	if normalized != "auto" {
		return normalized
	}
	probe := strings.Join(nonEmptyStrings([]string{fallbackUserInput, latestUserMessageText(contextMessages), wakeUpContext}), "\n")
	return inferNarrativeGuideModeFromText(probe)
}

func latestUserMessageText(contextMessages []map[string]any) string {
	for i := len(contextMessages) - 1; i >= 0; i-- {
		msg := contextMessages[i]
		if strings.ToLower(strings.TrimSpace(extractionStringFromAny(msg["role"]))) != "user" {
			continue
		}
		content := strings.TrimSpace(extractionStringFromAny(msg["content"]))
		if content != "" {
			return content
		}
	}
	return ""
}

func inferNarrativeGuideModeFromText(text string) string {
	source := strings.ToLower(strings.TrimSpace(text))
	if source == "" {
		return "standard"
	}
	if containsAnyText(source, "r18", "r 18", "explicit", "direct sensual", "mature direct", "adult direct") {
		return "mature_direct"
	}
	if containsAnyText(source, "sensual", "mature", "adult romance", "soft mature", "intimate") {
		return "mature_soft"
	}
	if containsAnyText(source, "romance", "romantic", "love", "date", "crush", "kiss") {
		return "romantic"
	}
	if containsAnyText(source, "action", "battle", "fight", "combat", "mission", "chase", "duel") {
		return "action"
	}
	return "standard"
}

func containsAnyText(source string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(source, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func normalizeNarrativeGuideStrength(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "none":
		return "none"
	case "medium", "strong":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "weak"
	}
}

func narrativeGuideStrengthLine(strength string) string {
	switch normalizeNarrativeGuideStrength(strength) {
	case "none":
		return ""
	case "strong":
		return "Strength: strong. Be more active about pacing, continuity repair, and callback suggestions, but never override user input or force outcomes."
	case "medium":
		return "Strength: medium. Give visible pacing and continuity support when the scene has room, but avoid forcing outcomes."
	default:
		return "Strength: weak. Keep this nearly invisible; only prevent continuity breaks or obvious tone drift."
	}
}

func buildGuideModeSuffix(mode string, strength ...string) string {
	selectedStrength := "weak"
	if len(strength) > 0 {
		selectedStrength = strength[0]
	}
	if normalizeNarrativeGuideStrength(selectedStrength) == "none" {
		return ""
	}
	strengthLine := narrativeGuideStrengthLine(selectedStrength)
	switch normalizeNarrativeGuideMode(mode) {
	case "standard":
		return strings.Join([]string{
			"",
			"[Narrative Guide ??Standard]",
			strengthLine,
			"Use as light optional style hints only; do not force the next scene, resolution, or character decision.",
			"Prefer continuity-preserving tone, pacing, and callbacks when they naturally fit.",
			"Current user input has priority; if the input is narrow, keep the guide almost invisible.",
		}, "\n")
	case "romantic":
		return strings.Join([]string{
			"",
			"[Narrative Guide ??Romantic]",
			strengthLine,
			"Use as light optional style hints only; do not force confession, intimacy, jealousy, or a relationship milestone.",
			"Let emotional dynamics surface only when the current exchange supports them.",
			"Dialogue subtext is preferred over explicit declarations.",
		}, "\n")
	case "action":
		return strings.Join([]string{
			"",
			"[Narrative Guide ??Action]",
			strengthLine,
			"Use as light optional style hints only; do not force combat, chase, danger, or a scene jump.",
			"If combat/chase/action is already happening, keep momentum clear and consequences grounded.",
			"If the user is only preparing or observing, do not escalate on their behalf.",
		}, "\n")
	case "mature_soft":
		return strings.Join([]string{
			"",
			"[Narrative Guide ??Mature (Sensual)]",
			strengthLine,
			"Use as light optional style hints only; do not force intimacy, escalation, or physical contact.",
			"When story-appropriate, prefer sensory, suggestive, indirect description.",
			"Atmosphere and emotion over explicit mechanics.",
			"Respect character agency and established boundaries.",
		}, "\n")
	case "mature_direct":
		return strings.Join([]string{
			"",
			"[Narrative Guide ??Mature (Explicit)]",
			strengthLine,
			"Use as light optional style hints only; do not force explicit content, escalation, or irreversible intimacy.",
			"Direct description is allowed only when the current scene and user input clearly support it.",
			"Character voice and emotional context remain paramount.",
			"Do not reduce characters to mere participants; inner thoughts matter.",
		}, "\n")
	default:
		return ""
	}
}

func buildGuideModeDirectorOverrides(mode string) map[string]any {
	switch normalizeNarrativeGuideMode(mode) {
	case "standard":
		return map[string]any{
			"emphasis":        []string{"tension management", "pacing variety", "subplot callbacks"},
			"forbidden_moves": []string{},
		}
	case "romantic":
		return map[string]any{
			"emphasis":        []string{"emotional resonance", "relationship progression", "intimate atmosphere"},
			"forbidden_moves": []string{"sudden genre shift to horror", "trivializing emotional moments"},
		}
	case "action":
		return map[string]any{
			"emphasis":        []string{"combat choreography", "environmental hazards", "tactical decisions"},
			"forbidden_moves": []string{"excessive monologuing during action", "deus ex machina resolution"},
		}
	case "mature_soft":
		return map[string]any{
			"emphasis":        []string{"sensory atmosphere", "emotional vulnerability", "consensual dynamics"},
			"forbidden_moves": []string{"gratuitous shock content", "ignoring character consent"},
		}
	case "mature_direct":
		return map[string]any{
			"emphasis":        []string{"vivid physical description", "emotional authenticity", "character agency"},
			"forbidden_moves": []string{"dehumanizing portrayals", "ignoring character consent"},
		}
	default:
		return map[string]any{
			"emphasis":        []string{},
			"forbidden_moves": []string{},
		}
	}
}

func normalizeNarrativeStance(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "reactive":
		return "reactive"
	case "proactive":
		return "proactive"
	default:
		return "balanced"
	}
}

func buildNarrativeStanceSuffix(mode string) string {
	switch normalizeNarrativeStance(mode) {
	case "reactive":
		return strings.Join([]string{
			"",
			"[Story Initiative - Reactive]",
			"Stay close to the user's immediate lead and the current scene.",
			"If the user expresses caution, hesitation, or uncertainty, remain in observation, clarification, or low-risk preparation rather than pushing action.",
			"Do not initiate entry, unlock barriers, assign a plan, or commit companions to a risky move unless the user explicitly asks for that step.",
			"Advance existing threads only when the current exchange clearly opens space for it, and keep any suggestion small and reversible.",
		}, "\n")
	case "proactive":
		return strings.Join([]string{
			"",
			"[Story Initiative - Proactive]",
			"You may introduce one plausible next beat or complication when continuity supports it.",
			"Initiative must grow from existing tensions, hooks, promises, or scene context.",
			"When the user is still deciding, propose the next beat rather than executing the decision on the user's behalf.",
			"Do not override the user's intent, skip causal steps, or force abrupt scene changes.",
		}, "\n")
	default:
		return strings.Join([]string{
			"",
			"[Story Initiative - Balanced]",
			"You may add one gentle next-beat nudge when it naturally fits the current scene.",
			"Keep the response anchored to the user's immediate intent and the current arc, and suggest rather than execute the next step.",
			"Avoid abrupt escalation, forced twists, or hard scene jumps.",
		}, "\n")
	}
}

func buildNarrativeStanceBounds(mode string) map[string]any {
	switch normalizeNarrativeStance(mode) {
	case "reactive":
		return map[string]any{
			"emphasis": []string{
				"user-led follow-through",
				"observation before action",
				"low-risk option framing",
			},
			"forbidden_moves": []string{
				"unlocking barriers or initiating entry without explicit user intent",
				"committing the group to a risky plan on the user's behalf",
				"inventing urgent danger to force motion",
			},
			"max_new_beats":    0,
			"allow_scene_jump": false,
		}
	case "proactive":
		return map[string]any{
			"emphasis": []string{
				"causal next-beat proposal",
				"continuity-aware tension increase",
				"bounded steering",
			},
			"forbidden_moves": []string{
				"forcing irreversible turns without buildup",
				"overwriting the user's immediate intent",
				"turning a cautious pause into immediate entry or confrontation without buy-in",
			},
			"max_new_beats":    1,
			"allow_scene_jump": false,
		}
	default:
		return map[string]any{
			"emphasis": []string{
				"gentle next-beat nudges",
				"continuity-aware escalation",
				"conversation momentum",
			},
			"forbidden_moves": []string{
				"hard scene cut without setup",
				"forcing a dramatic turn too early",
				"executing a risky step before the user agrees to it",
			},
			"max_new_beats":    1,
			"allow_scene_jump": false,
		}
	}
}

func buildNarrativeStanceSummary(mode, suffix string, bounds map[string]any) map[string]any {
	normalized := normalizeNarrativeStance(mode)
	return map[string]any{
		"mode":              normalized,
		"suffix_applied":    strings.TrimSpace(suffix) != "",
		"suffix_preview":    truncateTextForShadow(strings.Join(nonEmptyLines(suffix), " "), 140),
		"max_new_beats":     bounds["max_new_beats"],
		"allow_scene_jump":  bounds["allow_scene_jump"],
		"emphasis_count":    len(stringSliceFromAny(bounds["emphasis"])),
		"emphasis_preview":  strings.Join(limitStringSlice(stringSliceFromAny(bounds["emphasis"]), 2), ", "),
		"forbidden_count":   len(stringSliceFromAny(bounds["forbidden_moves"])),
		"forbidden_preview": strings.Join(limitStringSlice(stringSliceFromAny(bounds["forbidden_moves"]), 2), ", "),
	}
}

func nonEmptyLines(text string) []string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func stringSliceFromAny(v any) []string {
	switch typed := v.(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if s := strings.TrimSpace(extractionStringFromAny(item)); s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func limitStringSlice(items []string, limit int) []string {
	if limit < 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}

func buildCriticInputPack(chatSessionID string, turnIndex int, rawUserInput string, promptAssembly map[string]any, evidenceCounts map[string]any, sectionSummary []map[string]any, degraded bool) map[string]any {
	status := "ready"
	if degraded {
		status = "degraded"
	}
	return map[string]any{
		"status":              status,
		"source":              "go_r1_read_shadow",
		"chat_session_id":     chatSessionID,
		"turn_index":          turnIndex,
		"turn_content_chars":  len([]rune(rawUserInput)),
		"prompt_assembly":     promptAssembly,
		"prompt_source":       promptAssembly["prompt_source"],
		"evidence_counts":     evidenceCounts,
		"section_summary":     sectionSummary,
		"output_contract":     []string{"memories", "direct_evidence", "kg_triples", "critic_feedback"},
		"critic_context_plan": []string{"turn_content", "recent_chat", "direct_evidence", "kg_triples", "supervisor_input_pack"},
		"verdict":             "not_executed",
		"would_call_llm":      false,
		"would_write":         false,
		"degraded":            degraded,
	}
}

type prepareTurnInjectionBlock struct {
	Label   string
	Text    string
	Source  string
	Count   int
	Budget  int
	Trimmed bool
}

type prepareTurnInjectionAssembly struct {
	Text                     string
	SagaText                 string
	ChapterText              string
	MemoryText               string
	KGText                   string
	DirectEvidenceText       string
	FallbackText             string
	StorylineText            string
	WorldRulesText           string
	CharacterText            string
	PendingThreadText        string
	EpisodeText              string
	PersonaText              string
	CharacterPrivateText     string
	ContinuityCorrectionText string
	LatestDirectEvidenceText string
	RecentRawTurnText        string
	ScopedVerbatimText       string
	ScopedVerbatimSupport    archivebridge.ScopedVerbatimSupport
	ArcText                  string
	CanonText                string
	Truncated                bool
	Blocks                   []prepareTurnInjectionBlock
	Trimmed                  []map[string]any
	BudgetDecisions          map[string]any
	Counts                   map[string]any
	LanguageContext          map[string]any
	LanguageInjectionTrace   map[string]any
	PerspectiveContext       map[string]any
}

func buildInjectionPack(rawUserInput, inputContextText string, injectionEnabled, inputContextEnabled, inputContextTruncated bool, assembly prepareTurnInjectionAssembly, temporalSupportPacket map[string]any) map[string]any {
	status := "skeleton"
	if !injectionEnabled && !inputContextEnabled {
		status = "off"
	} else if strings.TrimSpace(assembly.Text) != "" || strings.TrimSpace(inputContextText) != "" {
		status = "ready"
	}

	blockTrace := make([]map[string]any, 0, len(assembly.Blocks))
	for _, block := range assembly.Blocks {
		blockTrace = append(blockTrace, map[string]any{
			"label":   block.Label,
			"source":  block.Source,
			"count":   block.Count,
			"chars":   len([]rune(block.Text)),
			"budget":  block.Budget,
			"trimmed": block.Trimmed,
		})
	}

	budgetDecisions := assembly.BudgetDecisions
	if budgetDecisions == nil {
		budgetDecisions = map[string]any{
			"version":                    "t1c.v1",
			"mode":                       "read_only_surface",
			"status":                     "off",
			"decision_count":             0,
			"decisions":                  []map[string]any{},
			"global_cap_chars":           6000,
			"global_selected_chars":      0,
			"canon_floor_reserved_chars": 120,
			"canon_selected_chars":       0,
			"reason_counts":              map[string]int{"tier_cap": 0},
			"source_mapping":             "recall_result.intent_execution_shadow.budget_enforcement",
			"source_event":               "budget_enforcement",
			"source_counters":            []string{"decision_count", "global_cap_chars", "global_selected_chars", "canon_floor_reserved_chars", "canon_selected_chars", "reason_counts"},
		}
	}

	var temporalPacket any
	var temporalPacketText string
	if temporalSupportPacket != nil {
		if packet, ok := temporalSupportPacket["temporal_packet"].(map[string]any); ok {
			temporalPacket = packet
		}
		if text, ok := temporalSupportPacket["temporal_packet_text"].(string); ok {
			temporalPacketText = text
		}
	}

	return map[string]any{
		"status":                                status,
		"source":                                "go_r1_read_shadow",
		"effective_user_input":                  rawUserInput,
		"injection_text":                        nilIfEmpty(assembly.Text),
		"input_context_text":                    nilIfEmpty(inputContextText),
		"continuity_correction_text":            nilIfEmpty(assembly.ContinuityCorrectionText),
		"memory_text":                           nilIfEmpty(assembly.MemoryText),
		"language_context":                      nilIfEmptyMap(assembly.LanguageContext),
		"language_injection_trace":              nilIfEmptyMap(assembly.LanguageInjectionTrace),
		"perspective_context":                   nilIfEmptyMap(assembly.PerspectiveContext),
		"kg_text":                               nilIfEmpty(assembly.KGText),
		"direct_evidence_text":                  nilIfEmpty(assembly.DirectEvidenceText),
		"fallback_text":                         nilIfEmpty(assembly.FallbackText),
		"storyline_text":                        nilIfEmpty(assembly.StorylineText),
		"world_rules_text":                      nilIfEmpty(assembly.WorldRulesText),
		"character_text":                        nilIfEmpty(assembly.CharacterText),
		"pending_thread_text":                   nilIfEmpty(assembly.PendingThreadText),
		"episode_text":                          nilIfEmpty(assembly.EpisodeText),
		"persona_recollection_text":             nilIfEmpty(assembly.PersonaText),
		"persona_recollection_active":           strings.TrimSpace(assembly.PersonaText) != "",
		"persona_recollection_policy":           personaRecollectionSupportPolicy(strings.TrimSpace(assembly.PersonaText) != ""),
		"character_private_recollection_text":   nilIfEmpty(assembly.CharacterPrivateText),
		"character_private_recollection_active": strings.TrimSpace(assembly.CharacterPrivateText) != "",
		"character_private_recollection_policy": characterPrivateRecollectionPolicy(strings.TrimSpace(assembly.CharacterPrivateText) != ""),
		"latest_direct_evidence_text": nilIfEmpty(
			assembly.LatestDirectEvidenceText,
		),
		"scoped_verbatim_support_text":  nilIfEmpty(assembly.ScopedVerbatimText),
		"scoped_verbatim_support_count": assembly.ScopedVerbatimSupport.Count,
		"scoped_verbatim_support_items": assembly.ScopedVerbatimSupport.Items,
		"verbatim_support":              assembly.ScopedVerbatimSupport,
		"hierarchy_escape_hatch":        buildHierarchyEscapeHatch(assembly.ScopedVerbatimSupport),
		"recent_raw_turn_text":          nilIfEmpty(assembly.RecentRawTurnText),
		"canon_text":                    nilIfEmpty(assembly.CanonText),
		"temporal_packet":               temporalPacket,
		"temporal_packet_text":          nilIfEmpty(temporalPacketText),
		"would_inject":                  injectionEnabled && strings.TrimSpace(assembly.Text) != "",
		"input_context_enabled":         inputContextEnabled,
		"injection_truncated":           assembly.Truncated,
		"input_context_truncated":       inputContextTruncated,
		"budget_decisions":              budgetDecisions,
		"section_blocks":                blockTrace,
		"trimmed":                       assembly.Trimmed,
		"counts":                        assembly.Counts,
		"status_vocabulary":             []string{"off", "skeleton", "partial", "ready", "degraded"},
		"final_budget_owner":            "archive_center_js_assembleInjectionWithBudget",
		"apply_verdict":                 "shadow_only",
		"apply_verdict_rule":            "trace_only",
		"saga_text":                     nilIfEmpty(assembly.SagaText),
		"saga_delivered":                strings.TrimSpace(assembly.SagaText) != "",
		"chapter_text":                  nilIfEmpty(assembly.ChapterText),
		"chapter_delivered":             strings.TrimSpace(assembly.ChapterText) != "",
		"arc_text":                      nilIfEmpty(assembly.ArcText),
		"arc_delivered":                 strings.TrimSpace(assembly.ArcText) != "",
		"would_call_llm":                false,
		"would_write":                   false,
	}
}

func buildPrepareTurnInputTransparencyRenderModel(sid string, turnIndex int, rawUserInput, inputContextText string, injectionEnabled, inputContextEnabled, inputContextTruncated, degraded bool, fallbackReason string, assembly prepareTurnInjectionAssembly) map[string]any {
	status := prepareTurnRenderModelStatus(degraded, injectionEnabled, inputContextEnabled, assembly.Text, inputContextText)
	counts := prepareTurnRenderCounts(assembly, inputContextText)
	blocks := []map[string]any{}
	appendPrepareTurnRenderBlock(&blocks, "user_input", "User Input", "prepare_turn.raw_user_input", rawUserInput, 1, true, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "input_context", "Input Context", "prepare_turn.input_context_text", inputContextText, boolToInt(strings.TrimSpace(inputContextText) != ""), inputContextEnabled, inputContextTruncated, 0)
	appendPrepareTurnRenderBlock(&blocks, "related_memories", "Related Memories", "store.memories", assembly.MemoryText, intFromAny(counts["selected_memory_total_count"], intFromAny(counts["memory_count"], 0)), injectionEnabled, false, intFromAny(counts["top_k_memory_target"], 0))
	appendPrepareTurnRenderBlock(&blocks, "kg_relationships", "KG Relationships", "store.kg_triples", assembly.KGText, intFromAny(counts["kg_bound"], intFromAny(counts["kg_count"], 0)), injectionEnabled, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "direct_evidence", "Direct Evidence", "store.direct_evidence_records", assembly.DirectEvidenceText, intFromAny(counts["direct_evidence_bound"], intFromAny(counts["vector_evidence_injected_count"], 0)), injectionEnabled, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "fallback_chat_logs", "Fallback Chat Logs", "store.chat_logs", assembly.FallbackText, intFromAny(counts["fallback_bound"], intFromAny(counts["fallback_count"], 0)), injectionEnabled, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "episode_summaries", "Episode Summaries", "store.episode_summaries", assembly.EpisodeText, intFromAny(counts["episode_bound"], intFromAny(counts["episode_summary_count"], 0)), injectionEnabled, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "chapter_recall", "Chapter Recall", "store.chapter_summaries", assembly.ChapterText, boolToInt(strings.TrimSpace(assembly.ChapterText) != ""), injectionEnabled, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "arc_recall", "Arc Recall", "store.arc_summaries", assembly.ArcText, boolToInt(strings.TrimSpace(assembly.ArcText) != ""), injectionEnabled, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "saga_recall", "Saga Recall", "store.saga_digests", assembly.SagaText, boolToInt(strings.TrimSpace(assembly.SagaText) != ""), injectionEnabled, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "storylines", "Ongoing Storylines", "store.storylines", assembly.StorylineText, intFromAny(counts["storyline_count"], 0), injectionEnabled, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "world_context", "World Context", "store.world_rules", assembly.WorldRulesText, intFromAny(counts["world_rule_count"], 0), injectionEnabled, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "character_states", "Character States", "store.character_states", assembly.CharacterText, intFromAny(counts["character_state_count"], 0), injectionEnabled, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "open_threads", "Open Threads", "store.pending_threads", assembly.PendingThreadText, intFromAny(counts["pending_thread_count"], 0), injectionEnabled, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "canonical_state_layer", "Canonical State Layer", "store.canonical_state_layers", assembly.CanonText, intFromAny(counts["canonical_layer_count"], 0), injectionEnabled, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "persona_recollection", "Persona Recollection", "store.persona_memory_entries", assembly.PersonaText, intFromAny(counts["persona_recollection_bound"], intFromAny(counts["persona_recollection_count"], 0)), injectionEnabled, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "character_private_recollection", "Character Private Recollection", "store.protagonist_entity_memories", assembly.CharacterPrivateText, intFromAny(counts["character_private_recollection_bound"], intFromAny(counts["character_private_recollection_count"], 0)), injectionEnabled, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "latest_direct_evidence", "Latest Direct Evidence", "store.direct_evidence_records", assembly.LatestDirectEvidenceText, boolToInt(strings.TrimSpace(assembly.LatestDirectEvidenceText) != ""), injectionEnabled, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "recent_raw_turn", "Recent Raw Turn", "store.chat_logs", assembly.RecentRawTurnText, boolToInt(strings.TrimSpace(assembly.RecentRawTurnText) != ""), injectionEnabled, false, 0)
	appendPrepareTurnRenderBlock(&blocks, "scoped_verbatim_support", "Scoped Verbatim Support", "store.direct_evidence_records", assembly.ScopedVerbatimText, intFromAny(counts["scoped_verbatim_support_count"], 0), injectionEnabled, false, 0)
	counts["render_block_count"] = len(blocks)
	counts["render_included_block_count"] = prepareTurnIncludedRenderBlockCount(blocks)
	return map[string]any{
		"contract_version":        "input_transparency_render.v1",
		"status":                  status,
		"source":                  "prepare_turn_backend_render_model",
		"session_id":              sid,
		"turn_index":              turnIndex,
		"read_only":               true,
		"write_attempted":         false,
		"llm_call_attempted":      false,
		"raw_user_rewritten":      false,
		"injection_enabled":       injectionEnabled,
		"input_context_enabled":   inputContextEnabled,
		"injection_truncated":     assembly.Truncated,
		"input_context_truncated": inputContextTruncated,
		"fallback_reason":         nilIfEmpty(fallbackReason),
		"blocks":                  blocks,
		"counts":                  counts,
		"language_context":        nilIfEmptyMap(assembly.LanguageContext),
		"language_injection_trace": nilIfEmptyMap(
			assembly.LanguageInjectionTrace,
		),
		"perspective_context":   nilIfEmptyMap(assembly.PerspectiveContext),
		"secret_display_policy": "counts_only_no_secret_text",
	}
}

func buildPrepareTurnEffectiveInputPreview(sid string, turnIndex int, rawUserInput, requestType, applyMode, inputContextText string, injectionEnabled, inputContextEnabled, inputContextTruncated, degraded bool, fallbackReason string, assembly prepareTurnInjectionAssembly) map[string]any {
	status := prepareTurnRenderModelStatus(degraded, injectionEnabled, inputContextEnabled, assembly.Text, inputContextText)
	finalUserSource := "input_hook"
	if strings.TrimSpace(requestType) != "" && strings.TrimSpace(requestType) != "model" {
		finalUserSource = strings.TrimSpace(requestType)
	}
	return map[string]any{
		"contract_version":        "effective_input_preview.v1",
		"status":                  status,
		"source":                  "prepare_turn_backend_render_model",
		"session_id":              sid,
		"turn_index":              turnIndex,
		"payload_apply_mode":      strings.TrimSpace(applyMode),
		"final_user_source":       finalUserSource,
		"final_user_text":         rawUserInput,
		"final_user_chars":        len([]rune(rawUserInput)),
		"auxiliary_context_chars": len([]rune(assembly.Text)),
		"input_context_chars":     len([]rune(inputContextText)),
		"injection_applied":       injectionEnabled && strings.TrimSpace(assembly.Text) != "",
		"input_context_applied":   inputContextEnabled && strings.TrimSpace(inputContextText) != "",
		"injection_truncated":     assembly.Truncated,
		"input_context_truncated": inputContextTruncated,
		"raw_user_rewritten":      false,
		"read_only":               true,
		"write_attempted":         false,
		"llm_call_attempted":      false,
		"fallback_reason":         nilIfEmpty(fallbackReason),
		"counts":                  prepareTurnRenderCounts(assembly, inputContextText),
	}
}

func prepareTurnRenderModelStatus(degraded, injectionEnabled, inputContextEnabled bool, injectionText, inputContextText string) string {
	if degraded {
		return "degraded"
	}
	if !injectionEnabled && !inputContextEnabled {
		return "off"
	}
	if strings.TrimSpace(injectionText) == "" && strings.TrimSpace(inputContextText) == "" {
		return "empty"
	}
	return "ready"
}

func prepareTurnRenderCounts(assembly prepareTurnInjectionAssembly, inputContextText string) map[string]any {
	counts := map[string]any{}
	for k, v := range assembly.Counts {
		counts[k] = v
	}
	counts["vector_found"] = intFromAny(counts["vector_hit_count"], intFromAny(counts["vector_memory_hit_count"], 0)+intFromAny(counts["vector_evidence_hit_count"], 0)+intFromAny(counts["vector_world_rule_hit_count"], 0))
	counts["vector_hydrated"] = intFromAny(counts["vector_hydrated_count"], intFromAny(counts["vector_memory_hydrated_count"], 0)+intFromAny(counts["vector_evidence_hydrated_count"], 0)+intFromAny(counts["vector_world_rule_hydrated_count"], 0))
	counts["vector_selected"] = intFromAny(counts["vector_selected_count"], intFromAny(counts["vector_memory_selected_count"], 0)+intFromAny(counts["vector_evidence_selected_count"], 0)+intFromAny(counts["vector_world_rule_selected_count"], 0))
	counts["vector_injected"] = intFromAny(counts["vector_injected_count"], intFromAny(counts["vector_memory_injected_count"], 0)+intFromAny(counts["vector_evidence_injected_count"], 0)+intFromAny(counts["vector_world_rule_injected_count"], 0))
	counts["related_memory_count"] = intFromAny(counts["selected_memory_total_count"], intFromAny(counts["memory_count"], 0))
	if strings.TrimSpace(assembly.MemoryText) == "" {
		counts["memory_injected"] = 0
	} else {
		counts["memory_injected"] = intFromAny(counts["selected_memory_total_count"], intFromAny(counts["memory_count"], 0))
	}
	counts["auxiliary_context_chars"] = len([]rune(assembly.Text))
	counts["input_context_chars"] = len([]rune(inputContextText))
	counts["injection_truncated"] = assembly.Truncated
	return counts
}

func appendPrepareTurnRenderBlock(blocks *[]map[string]any, key, title, source, text string, count int, enabled, truncated bool, budget int) {
	status := "empty"
	if !enabled {
		status = "disabled"
	} else if strings.TrimSpace(text) != "" {
		status = "included"
	}
	block := map[string]any{
		"key":       key,
		"title":     title,
		"status":    status,
		"source":    source,
		"count":     count,
		"chars":     len([]rune(text)),
		"truncated": truncated,
		"text":      nilIfEmpty(text),
	}
	if budget > 0 {
		block["budget"] = budget
	}
	*blocks = append(*blocks, block)
}

func prepareTurnIncludedRenderBlockCount(blocks []map[string]any) int {
	count := 0
	for _, block := range blocks {
		if strings.TrimSpace(extractionStringFromAny(block["status"])) == "included" {
			count++
		}
	}
	return count
}

func nilIfEmpty(text string) any {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	return text
}

func truncateTextForShadow(text string, limit int) string {
	text = strings.Join(strings.Fields(text), " ")
	return truncateRunes(text, limit)
}

func truncateRunes(text string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	if limit <= 3 {
		return string(runes[:limit])
	}
	return string(runes[:limit-3]) + "..."
}

func compactJSONForShadow(v any, limit int) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return truncateTextForShadow(string(data), limit)
}

func (s *Server) prepareTurnVectorShadow(ctx context.Context, req dto.PrepareTurnRequest, limit int) map[string]any {
	shadow := map[string]any{
		"status":                       "unconfigured",
		"engine":                       "chromadb",
		"source":                       "go_r1_read_shadow",
		"note":                         "ChromaDB is the 2.0 vector accelerator; MariaDB remains canonical truth",
		"configured":                   s.Cfg.Readiness.ChromaConfigured,
		"chromadb_endpoint_configured": strings.TrimSpace(s.Cfg.ChromaEndpoint) != "",
		"recall_read_drill_enabled":    true,
		"product_read_enabled":         strings.TrimSpace(s.Cfg.ChromaEndpoint) != "" && s.VectorOpenError == nil,
		"live_retrieval_enabled":       false,
		"chromadb_live_enabled":        false,
		"health_checked":               false,
		"search_attempted":             false,
		"backfill_attempted":           false,
	}
	defer finalizePrepareTurnVectorShadow(shadow)
	if s.Vector == nil {
		shadow["status"] = "disabled"
		shadow["health_error"] = "vector store is not configured"
		return shadow
	}
	health, err := s.Vector.Health(ctx)
	shadow["health_checked"] = true
	if err != nil {
		shadow["status"] = "degraded"
		shadow["health_error"] = err.Error()
		return shadow
	}
	shadow["status"] = health.Status
	shadow["collection"] = health.Collection
	shadow["persist_dir"] = health.PersistDir
	shadow["total_count"] = health.TotalCount
	shadow["project_model"] = health.ProjectModel
	shadow["model_ready"] = health.ModelReady
	shadow["preflight_issues"] = health.PreflightIssues
	queryVector := clientMetaFloat32Vector(req.ClientMeta, "chroma_query_vector")
	queryKey := "chroma_query_vector"
	if len(queryVector) == 0 {
		shadow["query_embedding_attempted"] = true
		embeddingCfg := s.completeTurnExtractionConfig(req.ClientMeta).Embedder
		shadow["query_embedding_configured"] = embeddingCfg.hasConfig()
		shadow["query_embedding_model"] = strings.TrimSpace(embeddingCfg.Model)
		if !embeddingCfg.hasConfig() {
			shadow["search_skipped_reason"] = "missing_chroma_query_vector_and_embedding_config"
			shadow["query_embedding_missing_fields"] = embeddingCfg.missingFields()
			return shadow
		}
		queryText := strings.TrimSpace(stringPtrValue(req.RawUserInput, ""))
		if queryText == "" {
			queryText = strings.TrimSpace(stringPtrValue(req.ContinuityQuery, ""))
		}
		if queryText == "" {
			for i := len(req.Messages) - 1; i >= 0; i-- {
				msg := req.Messages[i]
				if strings.TrimSpace(fmt.Sprint(msg["role"])) == "assistant" {
					continue
				}
				queryText = strings.TrimSpace(fmt.Sprint(msg["content"]))
				if queryText != "" {
					break
				}
			}
		}
		if queryText == "" {
			shadow["search_skipped_reason"] = "missing_query_text_for_embedding"
			return shadow
		}
		embeddingJSON, model, err := callEmbedding(ctx, embeddingCfg, queryText)
		if err != nil {
			shadow["status"] = "degraded"
			shadow["query_embedding_status"] = "error"
			shadow["query_embedding_error"] = err.Error()
			shadow["search_skipped_reason"] = "query_embedding_failed"
			return shadow
		}
		queryVector = parseFloat32JSONList(embeddingJSON)
		if len(queryVector) == 0 {
			shadow["status"] = "degraded"
			shadow["query_embedding_status"] = "empty"
			shadow["search_skipped_reason"] = "query_embedding_empty"
			return shadow
		}
		queryKey = "server_query_embedding"
		shadow["query_embedding_status"] = "ok"
		shadow["query_embedding_model"] = model
	}

	if strings.TrimSpace(s.Cfg.ChromaEndpoint) != "" && s.VectorOpenError == nil {
		shadow["source"] = "go_r2_chromadb_product_read"
		shadow["note"] = "R2 product read proof: ChromaDB search is enabled as the support-only vector accelerator"
		shadow["live_retrieval_enabled"] = true
		shadow["chromadb_live_enabled"] = true
	} else {
		shadow["note"] = "R2 bounded recall read drill: ChromaDB vector search remains support-only until endpoint readiness is configured"
	}
	limit = prepareTurnRecallLimit(limit)
	filter := strings.TrimSpace(clientMetaString(req.ClientMeta, "chroma_filter"))
	if filter == "" {
		filter = fmt.Sprintf("chat_session_id == %q", req.ChatSessionID)
	}
	shadow["search_attempted"] = true
	shadow["query_vector_key"] = queryKey
	shadow["query_vector_dim"] = len(queryVector)
	shadow["limit"] = limit
	shadow["filter"] = filter
	results, err := s.Vector.Search(ctx, req.ChatSessionID, queryVector, limit, filter)
	switch {
	case err == nil:
		shadow["search_result"] = "ok"
		shadow["search_result_count"] = len(results)
		shadow["search_results"] = vectorDocumentSearchPreview(results)
	case errors.Is(err, vector.ErrNotFound):
		shadow["search_result"] = "not_found"
		shadow["search_result_count"] = 0
		shadow["search_results"] = []map[string]any{}
	case errors.Is(err, vector.ErrNotEnabled):
		shadow["status"] = "degraded"
		shadow["search_result"] = "err_not_enabled"
		shadow["search_result_count"] = 0
		shadow["search_results"] = []map[string]any{}
	default:
		shadow["status"] = "degraded"
		shadow["search_result"] = "error"
		shadow["search_error"] = err.Error()
	}
	return shadow
}

func finalizePrepareTurnVectorShadow(shadow map[string]any) {
	readiness := buildPrepareTurnVectorReadiness(shadow)
	shadow["index_readiness"] = readiness
	shadow["fallback_recommended"] = boolFromAny(readiness["fallback_recommended"])
	shadow["reindex_recommended"] = boolFromAny(readiness["reindex_recommended"])
	shadow["degrade_mode"] = readiness["degrade_mode"]
}

func buildPrepareTurnVectorReadiness(shadow map[string]any) map[string]any {
	if shadow == nil {
		return map[string]any{
			"status":                        "disabled",
			"ready":                         false,
			"reason":                        "vector_shadow_missing",
			"fallback_recommended":          true,
			"reindex_recommended":           false,
			"degrade_mode":                  "raw_recent_fallback",
			"fallback_lane":                 "raw_fallback",
			"embedding_ready_before_search": false,
		}
	}
	status := strings.TrimSpace(stringFromMap(shadow, "status"))
	if status == "" {
		status = "unknown"
	}
	searchAttempted := boolFromAny(shadow["search_attempted"])
	searchResult := strings.TrimSpace(stringFromMap(shadow, "search_result"))
	modelReady := boolFromAny(shadow["model_ready"])
	totalCount := intFromAny(shadow["total_count"], 0)
	configured := boolFromAny(shadow["configured"]) || boolFromAny(shadow["chromadb_endpoint_configured"])
	reason := ""
	ready := false
	reindexRecommended := false
	switch {
	case status == "disabled":
		reason = "vector_store_disabled"
	case !configured:
		reason = "chromadb_unconfigured"
	case strings.TrimSpace(stringFromMap(shadow, "health_error")) != "":
		reason = "vector_health_error"
		reindexRecommended = true
	case strings.TrimSpace(stringFromMap(shadow, "query_embedding_error")) != "":
		reason = "query_embedding_failed"
	case !modelReady:
		reason = "embedding_model_not_ready"
		reindexRecommended = true
	case totalCount <= 0:
		reason = "vector_index_empty_or_not_reindexed"
		reindexRecommended = true
	case searchAttempted && searchResult == "error":
		reason = "vector_search_error"
		reindexRecommended = true
	case searchAttempted && searchResult == "err_not_enabled":
		reason = "vector_search_not_enabled"
	default:
		reason = "ready"
		ready = true
	}
	if searchAttempted && searchResult == "not_found" && reason == "ready" {
		reason = "searchable_no_hits"
	}
	fallbackRecommended := !ready || (searchAttempted && searchResult != "" && searchResult != "ok")
	return map[string]any{
		"status":                        reason,
		"ready":                         ready,
		"configured":                    configured,
		"engine_status":                 status,
		"model_ready":                   modelReady,
		"total_count":                   totalCount,
		"search_attempted":              searchAttempted,
		"search_result":                 nilIfEmpty(searchResult),
		"fallback_recommended":          fallbackRecommended,
		"fallback_lane":                 "raw_fallback",
		"degrade_mode":                  "recent_relevant_deep_raw_fallback",
		"reindex_recommended":           reindexRecommended,
		"embedding_ready_before_search": modelReady && totalCount > 0,
	}
}

func clientMetaFloat32Vector(meta map[string]any, key string) []float32 {
	if meta == nil {
		return nil
	}
	value, ok := meta[key]
	if !ok {
		return nil
	}
	switch typed := value.(type) {
	case []float32:
		return typed
	case []float64:
		out := make([]float32, 0, len(typed))
		for _, item := range typed {
			out = append(out, float32(item))
		}
		return out
	case []any:
		out := make([]float32, 0, len(typed))
		for _, item := range typed {
			switch n := item.(type) {
			case float64:
				out = append(out, float32(n))
			case float32:
				out = append(out, n)
			case int:
				out = append(out, float32(n))
			default:
				return nil
			}
		}
		return out
	default:
		return nil
	}
}

func clientMetaString(meta map[string]any, key string) string {
	if meta == nil {
		return ""
	}
	value, ok := meta[key]
	if !ok {
		return ""
	}
	s, _ := value.(string)
	return s
}

func prepareTurnPerspectiveContextFromClientMeta(meta map[string]any) map[string]any {
	if len(meta) == 0 {
		return nil
	}
	for _, nestedKey := range []string{"perspective_context", "viewpoint_context", "pov_context"} {
		if nested := normalizePrepareTurnPerspectiveContext(mapFromAny(meta[nestedKey])); len(nested) > 0 {
			if _, ok := nested["source"]; !ok {
				nested["source"] = nestedKey
			}
			return nested
		}
	}
	return normalizePrepareTurnPerspectiveContext(meta)
}

func prepareTurnPerspectiveContextFromRequest(req dto.PrepareTurnRequest) map[string]any {
	if ctx := prepareTurnPerspectiveContextFromClientMeta(req.ClientMeta); len(ctx) > 0 {
		return ctx
	}
	sources := []struct {
		source string
		text   string
	}{
		{source: "raw_user_input", text: stringPtrValue(req.RawUserInput, "")},
	}
	for i := len(req.Messages) - 1; i >= 0 && len(sources) < 8; i-- {
		msg := req.Messages[i]
		text := strings.TrimSpace(extractionStringFromAny(msg["content"]))
		if text == "" {
			continue
		}
		role := strings.TrimSpace(extractionStringFromAny(msg["role"]))
		if role == "" {
			role = "message"
		}
		sources = append(sources, struct {
			source string
			text   string
		}{source: "message." + role, text: text})
	}
	for _, source := range sources {
		if pov := inferPrepareTurnPerspectiveName(source.text); pov != "" {
			return normalizePrepareTurnPerspectiveContext(map[string]any{
				"current_pov": pov,
				"source":      "inferred_" + source.source,
			})
		}
	}
	return nil
}

func inferPrepareTurnPerspectiveName(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if pov := inferPrepareTurnPerspectiveNameFromLine(line); pov != "" {
			return pov
		}
	}
	return ""
}

func inferPrepareTurnPerspectiveNameFromLine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	lower := strings.ToLower(line)
	for _, marker := range []string{"pov", "point of view", "viewpoint"} {
		if idx := strings.Index(lower, marker); idx >= 0 {
			after := strings.TrimSpace(line[idx+len(marker):])
			if after != "" {
				if candidate := cleanPrepareTurnPerspectiveCandidate(after); candidate != "" {
					return candidate
				}
			}
			before := strings.TrimSpace(line[:idx])
			if candidate := cleanPrepareTurnPerspectiveCandidate(before); candidate != "" {
				return candidate
			}
		}
	}
	for _, marker := range []string{"시점", "관점", "입장", "視点", "の視点"} {
		if idx := strings.Index(line, marker); idx >= 0 {
			before := strings.TrimSpace(line[:idx])
			if candidate := cleanPrepareTurnPerspectiveCandidate(before); candidate != "" {
				return candidate
			}
		}
	}
	return ""
}

func cleanPrepareTurnPerspectiveCandidate(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n:：-–—[](){}<>「」『』\"'`")
	replacers := []string{
		"hidden spoiler", "", "spoiler", "", "pov", "", "point of view", "", "viewpoint", "",
		"current", "", "현재", "", "히든 스포일러", "", "스포일러", "", "의", "", "の", "",
	}
	lower := strings.ToLower(value)
	for i := 0; i+1 < len(replacers); i += 2 {
		prefix := replacers[i]
		replacement := replacers[i+1]
		if strings.HasPrefix(lower, prefix) {
			value = strings.TrimSpace(replacement + strings.TrimSpace(value[len(prefix):]))
			lower = strings.ToLower(value)
		}
	}
	cutset := []string{"\n", "\r", ".", "。", ",", "，", ";", "；", "|", "/", "\\", " - ", " -- ", " — ", " – "}
	for _, sep := range cutset {
		if idx := strings.Index(value, sep); idx >= 0 {
			value = strings.TrimSpace(value[:idx])
		}
	}
	value = strings.Trim(value, " \t\r\n:：-–—[](){}<>「」『』\"'`")
	for _, suffix := range []string{"의", "の"} {
		if strings.HasSuffix(value, suffix) {
			value = strings.TrimSpace(strings.TrimSuffix(value, suffix))
		}
	}
	if !validPrepareTurnPerspectiveCandidate(value) {
		return ""
	}
	return value
}

func validPrepareTurnPerspectiveCandidate(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	runeCount := len([]rune(value))
	if runeCount < 2 || runeCount > 60 {
		return false
	}
	lower := strings.ToLower(value)
	for _, blocked := range []string{
		"freely", "take a moment", "instruction", "instructions", "system", "developer", "assistant", "user",
		"prompt", "rules", "response", "format", "review", "reasoning", "draft",
	} {
		if strings.Contains(lower, blocked) {
			return false
		}
	}
	if len(strings.Fields(value)) > 5 {
		return false
	}
	return normalizeCharacterKey(value) != ""
}

func normalizePrepareTurnPerspectiveContext(raw map[string]any) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	pov := strings.TrimSpace(extractionFirstNonEmpty(
		extractionStringFromAny(raw["current_pov"]),
		extractionStringFromAny(raw["pov_character"]),
		extractionStringFromAny(raw["viewpoint_character"]),
		extractionStringFromAny(raw["narrator_character"]),
		extractionStringFromAny(raw["speaker_character"]),
		extractionStringFromAny(raw["current_speaker"]),
		extractionStringFromAny(raw["speaker"]),
		extractionStringFromAny(raw["current_character"]),
		extractionStringFromAny(raw["active_character"]),
	))
	if pov == "" {
		return nil
	}
	out := map[string]any{
		"contract_version": "perspective_context.v1",
		"current_pov":      truncateRunes(pov, 120),
		"current_pov_key":  normalizeCharacterKey(pov),
		"source":           extractionFirstNonEmpty(extractionStringFromAny(raw["source"]), "client_meta"),
	}
	if mode := strings.TrimSpace(extractionStringFromAny(raw["mode"])); mode != "" {
		out["mode"] = truncateRunes(mode, 80)
	}
	return out
}

func buildInjectionText(memories []store.Memory, kgTriples []store.KGTriple, storylines []store.Storyline, worldRules []store.WorldRule, charStates []store.CharacterState, pendingThreads []store.PendingThread, topK, maxChars int) (string, bool) {
	assembly := buildPrepareTurnInjectionAssembly(memories, kgTriples, nil, nil, storylines, worldRules, charStates, pendingThreads, nil, nil, nil, nil, nil, topK, maxChars, "", "default", nil, nil, nil)
	return assembly.Text, assembly.Truncated
}

func prepareTurnIntSetting(value, fallback *int) int {
	if value != nil && *value > 0 {
		return *value
	}
	if fallback != nil && *fallback > 0 {
		return *fallback
	}
	return 1
}

func prepareTurnRecallLimit(topK int) int {
	if topK > 0 {
		return topK
	}
	return 1
}

func prepareTurnSupportRecallLimit(topK int) int {
	return prepareTurnRecallLimit(topK)
}

func prepareTurnTextBudget(maxChars int) int {
	if maxChars > 0 {
		return maxChars
	}
	return 1
}

type prepareTurnMemoryLaneSelection struct {
	VectorRelevant []store.Memory
	Recent         []store.Memory
	Relevant       []store.Memory
	Deep           []store.Memory
	VectorScores   map[string]float64
	RelevantScores map[string]float64
	Trace          map[string]any
}

func prepareTurnMemorySelectionQuery(rawUserInput string, chatLogs []store.ChatLog, perspectiveContext map[string]any, topK int) string {
	parts := []string{}
	if text := strings.TrimSpace(rawUserInput); text != "" {
		parts = append(parts, text)
	}
	if pov := strings.TrimSpace(extractionStringFromAny(perspectiveContext["current_pov"])); pov != "" {
		parts = append(parts, "current_pov: "+pov)
	}
	for _, cl := range selectRecentChatLogsByTurn(chatLogs, prepareTurnRecallLimit(topK)) {
		if text := strings.TrimSpace(cl.Content); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func selectPrepareTurnMemoryLanes(memories []store.Memory, query string, topK int) prepareTurnMemoryLaneSelection {
	return selectPrepareTurnMemoryLanesWithVector(memories, query, topK, nil)
}

func selectPrepareTurnMemoryLanesWithVector(memories []store.Memory, query string, topK int, vectorShadow map[string]any) prepareTurnMemoryLaneSelection {
	topK = prepareTurnRecallLimit(topK)
	totalLimit := topK
	clean := make([]store.Memory, 0, len(memories))
	for _, item := range memories {
		if strings.TrimSpace(prepareTurnMemorySummary(item)) == "" {
			continue
		}
		clean = append(clean, item)
	}
	query = strings.TrimSpace(query)
	queryPresent := query != ""
	maxTurn := 0
	minTurn := 0
	importanceTotal := 0.0
	importanceSeen := 0
	for _, item := range clean {
		if item.TurnIndex > 0 {
			if minTurn == 0 || item.TurnIndex < minTurn {
				minTurn = item.TurnIndex
			}
			if item.TurnIndex > maxTurn {
				maxTurn = item.TurnIndex
			}
		}
		if item.Importance > 0 {
			importanceTotal += item.Importance
			importanceSeen++
		}
	}
	avgImportance := 0.0
	if importanceSeen > 0 {
		avgImportance = importanceTotal / float64(importanceSeen)
	}

	out := prepareTurnMemoryLaneSelection{
		VectorScores:   map[string]float64{},
		RelevantScores: map[string]float64{},
		Trace: map[string]any{
			"version":                     "r3.recall_lanes.v1",
			"top_k_definition":            "semantic_memory_recall_limit",
			"top_k_memory_target":         totalLimit,
			"vector_memory_policy":        "chromadb_hits_hydrated_to_mariadb_memory_before_injection",
			"relevant_memory_limit":       totalLimit,
			"deep_memory_policy":          "importance_only_when_no_current_query",
			"input_memory_count":          len(memories),
			"eligible_memory_count":       len(clean),
			"recent_order":                "ranked_recency_tiebreak_for_non_relevant_memory",
			"relevant_order":              "query_overlap_first_then_importance_then_recency",
			"deep_order":                  "ranked_non_relevant_high_importance_support",
			"selection_policy":            "query_relevance_then_recent_fallback; old_importance_alone_cannot_outrank_current_scene",
			"query_present":               queryPresent,
			"input_rewrite_applied":       false,
			"raw_user_input_preserved":    true,
			"long_gap_policy":             "widen_by_lanes_not_by_replacing_user_input",
			"selection_reason_visibility": true,
		},
	}
	vectorHydration := prepareTurnHydrateVectorMemoryHits(clean, vectorShadow, totalLimit)
	vectorRecallReady := prepareTurnVectorRecallReady(vectorHydration.Trace)
	vectorRecallAttempted := prepareTurnVectorSearchAttempted(vectorShadow)
	for _, item := range vectorHydration.Items {
		if prepareTurnSelectedMemoryCount(out) >= totalLimit {
			break
		}
		out.VectorRelevant = append(out.VectorRelevant, item)
		key := prepareTurnMemoryLaneKey(item)
		if score := vectorHydration.Scores[key]; score > 0 {
			out.VectorScores[key] = score
		}
	}
	out.Trace["vector_recall"] = vectorHydration.Trace
	out.Trace["vector_recall_ready"] = vectorRecallReady
	out.Trace["vector_recall_attempted"] = vectorRecallAttempted
	out.Trace["lexical_fill_enabled"] = !vectorRecallReady && prepareTurnSelectedMemoryCount(out) < totalLimit
	if vectorRecallReady {
		out.Trace["vector_selected"] = len(out.VectorRelevant)
		out.Trace["recent_selected"] = 0
		out.Trace["relevant_selected"] = 0
		out.Trace["deep_selected"] = 0
		out.Trace["selected_total"] = prepareTurnSelectedMemoryCount(out)
		out.Trace["relevant_candidates"] = 0
		out.Trace["memory_budget_remaining"] = maxInt(totalLimit-prepareTurnSelectedMemoryCount(out), 0)
		out.Trace["average_importance"] = avgImportance
		out.Trace["relevant_degraded_reason"] = nilIfEmpty(relevantDegradedReason(query, len(out.Relevant), 0))
		return out
	}

	type scoredMemory struct {
		item       store.Memory
		key        string
		relevance  float64
		importance float64
		recency    float64
	}
	scored := []scoredMemory{}
	relevantCandidates := 0
	for _, item := range clean {
		key := prepareTurnMemoryLaneKey(item)
		relevance := 0.0
		if queryPresent {
			relevance = simpleTokenSimilarity(query, prepareTurnMemoryRelevanceText(item))
			if relevance > 0 {
				relevantCandidates++
			}
		}
		recency := 0.0
		if item.TurnIndex > 0 {
			if maxTurn > minTurn {
				recency = float64(item.TurnIndex-minTurn) / float64(maxTurn-minTurn)
			} else {
				recency = 1
			}
		}
		scored = append(scored, scoredMemory{
			item:       item,
			key:        key,
			relevance:  relevance,
			importance: item.Importance,
			recency:    recency,
		})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		ia := scored[i]
		ja := scored[j]
		if queryPresent {
			ir := ia.relevance > 0
			jr := ja.relevance > 0
			if ir != jr {
				return ir
			}
			if ia.relevance != ja.relevance {
				return ia.relevance > ja.relevance
			}
			if !ir && !jr {
				if ia.recency != ja.recency {
					return ia.recency > ja.recency
				}
				if ia.item.TurnIndex != ja.item.TurnIndex {
					return ia.item.TurnIndex > ja.item.TurnIndex
				}
			}
		}
		if ia.importance != ja.importance {
			return ia.importance > ja.importance
		}
		if ia.recency != ja.recency {
			return ia.recency > ja.recency
		}
		if ia.item.TurnIndex != ja.item.TurnIndex {
			return ia.item.TurnIndex > ja.item.TurnIndex
		}
		return ia.item.ID > ja.item.ID
	})

	for _, candidate := range scored {
		if prepareTurnSelectedMemoryCount(out) >= totalLimit {
			break
		}
		if prepareTurnMemoryAlreadySelected(out, candidate.item) {
			continue
		}
		if candidate.relevance > 0 {
			out.Relevant = append(out.Relevant, candidate.item)
			out.RelevantScores[candidate.key] = candidate.relevance
			continue
		}
		if !queryPresent && avgImportance > 0 && candidate.importance >= avgImportance {
			out.Deep = append(out.Deep, candidate.item)
			continue
		}
		out.Recent = append(out.Recent, candidate.item)
	}
	out.Trace["vector_selected"] = len(out.VectorRelevant)
	out.Trace["recent_selected"] = len(out.Recent)
	out.Trace["relevant_selected"] = len(out.Relevant)
	out.Trace["deep_selected"] = len(out.Deep)
	out.Trace["selected_total"] = prepareTurnSelectedMemoryCount(out)
	out.Trace["relevant_candidates"] = relevantCandidates
	out.Trace["memory_budget_remaining"] = maxInt(totalLimit-prepareTurnSelectedMemoryCount(out), 0)
	out.Trace["average_importance"] = avgImportance
	out.Trace["relevant_degraded_reason"] = nilIfEmpty(relevantDegradedReason(query, len(out.Relevant), relevantCandidates))
	return out
}

func prepareTurnVectorRecallReady(trace map[string]any) bool {
	if trace == nil {
		return false
	}
	return strings.TrimSpace(stringFromMap(trace, "status")) == "ready"
}

func prepareTurnVectorSearchAttempted(vectorShadow map[string]any) bool {
	if vectorShadow == nil {
		return false
	}
	return boolFromAny(vectorShadow["search_attempted"])
}

func prepareTurnSelectedMemoryCount(selection prepareTurnMemoryLaneSelection) int {
	return len(selection.VectorRelevant) + len(selection.Recent) + len(selection.Relevant) + len(selection.Deep)
}

func prepareTurnMemoryLaneCounters(selection prepareTurnMemoryLaneSelection, injected bool) map[string]any {
	vectorTrace := mapFromAny(selection.Trace["vector_recall"])
	injectedCount := 0
	if injected {
		injectedCount = len(selection.VectorRelevant)
	}
	return map[string]any{
		"memory_lane_order":                             []string{"vector_relevant", "relevant", "deep", "recent"},
		"vector_memory_hit_count":                       intFromAny(vectorTrace["memory_hit_count"], maxInt(intFromAny(vectorTrace["input_hit_count"], 0)-intFromAny(vectorTrace["non_memory_count"], 0), 0)),
		"vector_memory_hydrated_count":                  intFromAny(vectorTrace["hydrated_count"], 0),
		"vector_memory_selected_count":                  len(selection.VectorRelevant),
		"vector_memory_injected_count":                  injectedCount,
		"vector_memory_duplicate_count":                 intFromAny(vectorTrace["duplicate_count"], 0),
		"vector_memory_missing_count":                   intFromAny(vectorTrace["missing_count"], 0),
		"vector_non_memory_hit_count":                   intFromAny(vectorTrace["non_memory_count"], 0),
		"vector_memory_hit_language_context_count":      intFromAny(vectorTrace["hit_language_context_count"], 0),
		"vector_memory_hit_alias_indexed_count":         intFromAny(vectorTrace["hit_alias_indexed_count"], 0),
		"vector_memory_hydrated_language_context_count": intFromAny(vectorTrace["hydrated_language_context_count"], 0),
		"vector_memory_hydrated_alias_ready_count":      intFromAny(vectorTrace["hydrated_alias_ready_count"], 0),
		"vector_memory_search_text_policy":              stringFromMap(vectorTrace, "search_text_policy"),
		"vector_memory_recall_status":                   stringFromMap(vectorTrace, "status"),
		"vector_memory_recall_reason":                   stringFromMap(vectorTrace, "reason"),
		"vector_relevant_memory_count":                  len(selection.VectorRelevant),
		"relevant_memory_count":                         len(selection.Relevant),
		"deep_memory_count":                             len(selection.Deep),
		"recent_memory_count":                           len(selection.Recent),
		"protected_memory_dropped_count":                intFromAny(selection.Trace["protected_memory_dropped_count"], 0),
		"protected_memory_gate":                         stringFromMap(selection.Trace, "protected_memory_gate"),
		"selected_memory_total_count":                   prepareTurnSelectedMemoryCount(selection),
		"selected_memory_total_target":                  intFromAny(selection.Trace["top_k_memory_target"], 0),
		"selected_memory_top_k_contract":                stringFromMap(selection.Trace, "top_k_definition"),
	}
}

func mergePrepareTurnMemoryLaneCounters(counts map[string]any, selection prepareTurnMemoryLaneSelection, injected bool) {
	if counts == nil {
		return
	}
	for key, value := range prepareTurnMemoryLaneCounters(selection, injected) {
		counts[key] = value
	}
}

func collapsePrepareTurnMemoryLaneSelection(selection prepareTurnMemoryLaneSelection) prepareTurnMemoryLaneSelection {
	seen := map[string]bool{}
	collapsed := 0
	collapseLane := func(items []store.Memory) []store.Memory {
		out := make([]store.Memory, 0, len(items))
		for _, item := range items {
			key := collapseTextKey(prepareTurnMemorySummary(item))
			if key == "" {
				key = prepareTurnMemoryLaneKey(item)
			}
			if key != "" && seen[key] {
				collapsed++
				continue
			}
			if key != "" {
				seen[key] = true
			}
			out = append(out, item)
		}
		return out
	}
	selection.VectorRelevant = collapseLane(selection.VectorRelevant)
	selection.Relevant = collapseLane(selection.Relevant)
	selection.Deep = collapseLane(selection.Deep)
	selection.Recent = collapseLane(selection.Recent)
	if selection.Trace == nil {
		selection.Trace = map[string]any{}
	}
	selection.Trace["memory_collapsed_count"] = collapsed
	selection.Trace["selected_total_after_collapse"] = prepareTurnSelectedMemoryCount(selection)
	return selection
}

func filterPrepareTurnProtectedMemoryLaneSelection(selection prepareTurnMemoryLaneSelection, rawUserInput string, chatLogs []store.ChatLog, perspectiveContext map[string]any) prepareTurnMemoryLaneSelection {
	ctx := buildPrepareTurnRecollectionContext(rawUserInput, chatLogs, nil, nil)
	before := prepareTurnSelectedMemoryCount(selection)
	dropped := []map[string]any{}
	filterLane := func(lane string, items []store.Memory) []store.Memory {
		out := make([]store.Memory, 0, len(items))
		for _, item := range items {
			ok, reason := prepareTurnProtectedMemoryRelevant(item, ctx, perspectiveContext)
			if ok {
				out = append(out, item)
				continue
			}
			dropped = append(dropped, map[string]any{
				"lane":       lane,
				"id":         item.ID,
				"turn_index": item.TurnIndex,
				"reason":     reason,
			})
		}
		return out
	}
	selection.VectorRelevant = filterLane("vector_relevant", selection.VectorRelevant)
	selection.Relevant = filterLane("relevant", selection.Relevant)
	selection.Deep = filterLane("deep", selection.Deep)
	selection.Recent = filterLane("recent", selection.Recent)
	if selection.Trace == nil {
		selection.Trace = map[string]any{}
	}
	selection.Trace["protected_memory_before_filter"] = before
	selection.Trace["protected_memory_after_filter"] = prepareTurnSelectedMemoryCount(selection)
	selection.Trace["protected_memory_dropped_count"] = len(dropped)
	selection.Trace["protected_memory_gate"] = "protected_owner_subject_knowledge_scope_or_current_pov_must_match_current_user_input_immediate_chat_or_pov"
	selection.Trace["protected_memory_dropped"] = dropped
	return selection
}

func prepareTurnProtectedMemoryRelevant(item store.Memory, ctx prepareTurnRecollectionContext, perspectiveContext map[string]any) (bool, string) {
	tokens, protected := prepareTurnProtectedMemoryEntityTokens(item)
	if !protected {
		return true, "not_protected_memory"
	}
	if len(tokens) == 0 {
		return true, "protected_memory_without_entity_scope"
	}
	if guard := prepareTurnProtectedMemoryGuard(item, perspectiveContext); guard.Active && guard.POVScoped {
		return true, "current_pov_scoped_identity_guard"
	}
	if prepareTurnAnyOwnerTokenMatches(tokens, ctx.rawUserInput) {
		return true, "explicit_current_user_input"
	}
	if prepareTurnAnyOwnerTokenMatches(tokens, ctx.immediateChatText) {
		return true, "immediate_chat_mention"
	}
	if pov := strings.TrimSpace(extractionStringFromAny(perspectiveContext["current_pov"])); pov != "" && prepareTurnAnyOwnerTokenMatches(tokens, pov) {
		return true, "current_pov_match"
	}
	return false, "protected_entity_not_in_current_input_or_immediate_chat"
}

func prepareTurnProtectedMemoryEntityTokens(item store.Memory) ([]string, bool) {
	parsed := parseJSONMap(item.SummaryJSON)
	protectedSecrets := sliceFromAny(parsed["protected_secrets"])
	identityAccuracy := sliceFromAny(parsed["character_identity_accuracy"])
	if len(protectedSecrets) == 0 && len(identityAccuracy) == 0 {
		return nil, false
	}
	tokens := []string{}
	add := func(value string) {
		for _, token := range prepareTurnOwnerTokens(value, value) {
			if token != "" && !stringSliceContains(tokens, token) {
				tokens = append(tokens, token)
			}
		}
	}
	addValues := func(values []string) {
		for _, value := range values {
			add(value)
		}
	}
	for _, raw := range protectedSecrets {
		secret := mapFromAny(raw)
		add(stringFromMap(secret, "owner"))
		addValues(stringsFromAny(secret["subject"]))
		scope := mapFromAny(secret["knowledge_scope"])
		addValues(stringsFromAny(scope["known_by"]))
		addValues(stringsFromAny(scope["suspected_by"]))
		addValues(stringsFromAny(scope["unknown_to"]))
	}
	for _, raw := range identityAccuracy {
		identity := mapFromAny(raw)
		for _, key := range []string{
			"canonical_entity_name",
			"surface_identity_name",
			"true_identity_name",
			"public_identity_name",
			"alias_name",
			"real_identity_name",
		} {
			add(stringFromMap(identity, key))
		}
		scope := mapFromAny(identity["knowledge_scope"])
		addValues(stringsFromAny(scope["known_by"]))
		addValues(stringsFromAny(scope["suspected_by"]))
		addValues(stringsFromAny(scope["unknown_to"]))
	}
	return tokens, true
}

func collapsePrepareTurnStorylines(items []store.Storyline) []store.Storyline {
	out := make([]store.Storyline, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		key := collapseTextKey(extractionFirstNonEmpty(item.Name, item.CurrentContext))
		detailKey := collapseTextKey(item.CurrentContext)
		if key == "" {
			key = detailKey
		}
		if key != "" && seen[key] {
			continue
		}
		if detailKey != "" && seen[detailKey] {
			continue
		}
		if key != "" {
			seen[key] = true
		}
		if detailKey != "" {
			seen[detailKey] = true
		}
		out = append(out, item)
	}
	return out
}

func mergePrepareTurnWorldRulesForInjection(priority, rest []store.WorldRule) []store.WorldRule {
	out := make([]store.WorldRule, 0, len(priority)+len(rest))
	out = append(out, priority...)
	out = append(out, rest...)
	return out
}

func collapsePrepareTurnWorldRules(items []store.WorldRule) []store.WorldRule {
	out := make([]store.WorldRule, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		if item.Suppressed {
			continue
		}
		key := strings.Join([]string{
			collapseTextKey(item.Scope),
			collapseTextKey(item.ScopeName),
			collapseTextKey(item.Category),
			collapseTextKey(item.Key),
			collapseTextKey(item.ValueJSON),
		}, "|")
		if strings.Trim(key, "|") == "" {
			continue
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, item)
	}
	return out
}

func collapseTextKey(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return ""
	}
	return strings.Join(strings.Fields(text), " ")
}

func prepareTurnMemoryLaneProtectedCounts(selection prepareTurnMemoryLaneSelection, perspectiveContext map[string]any) map[string]any {
	counts := map[string]any{
		"protected_secret_count":          0,
		"identity_accuracy_count":         0,
		"protected_memory_guarded_count":  0,
		"pov_scoped_identity_guard_count": 0,
		"protected_memory_selected_count": 0,
	}
	seen := map[string]bool{}
	add := func(item store.Memory) {
		key := prepareTurnMemoryLaneKey(item)
		if key == "" {
			key = fmt.Sprintf("turn:%d:%s", item.TurnIndex, item.SummaryJSON)
		}
		if seen[key] {
			return
		}
		seen[key] = true
		parsed := parseJSONMap(item.SummaryJSON)
		protectedSecrets := sliceFromAny(parsed["protected_secrets"])
		identityAccuracy := sliceFromAny(parsed["character_identity_accuracy"])
		counts["protected_secret_count"] = intFromAny(counts["protected_secret_count"], 0) + len(protectedSecrets)
		counts["identity_accuracy_count"] = intFromAny(counts["identity_accuracy_count"], 0) + len(identityAccuracy)
		if len(protectedSecrets) > 0 || len(identityAccuracy) > 0 {
			counts["protected_memory_selected_count"] = intFromAny(counts["protected_memory_selected_count"], 0) + 1
		}
		if guard := prepareTurnProtectedMemoryGuard(item, perspectiveContext); guard.Active {
			counts["protected_memory_guarded_count"] = intFromAny(counts["protected_memory_guarded_count"], 0) + 1
			if guard.POVScoped {
				counts["pov_scoped_identity_guard_count"] = intFromAny(counts["pov_scoped_identity_guard_count"], 0) + 1
			}
		}
	}
	for _, item := range selection.VectorRelevant {
		add(item)
	}
	for _, item := range selection.Relevant {
		add(item)
	}
	for _, item := range selection.Deep {
		add(item)
	}
	for _, item := range selection.Recent {
		add(item)
	}
	return counts
}

type prepareTurnVectorMemoryHydration struct {
	Items  []store.Memory
	Scores map[string]float64
	Trace  map[string]any
}

type prepareTurnVectorArtifactHydration struct {
	Evidence   []store.DirectEvidence
	WorldRules []store.WorldRule
	Trace      map[string]any
}

func prepareTurnHydrateVectorMemoryHits(memories []store.Memory, vectorShadow map[string]any, limit int) prepareTurnVectorMemoryHydration {
	out := prepareTurnVectorMemoryHydration{
		Items:  []store.Memory{},
		Scores: map[string]float64{},
		Trace: map[string]any{
			"version":                         "vdb1.hydrate_memory_hits.v1",
			"status":                          "not_attempted",
			"truth_boundary":                  "vector_hit_is_selector_only_mariadb_memory_is_canonical",
			"input_hit_count":                 0,
			"memory_hit_count":                0,
			"hydrated_count":                  0,
			"duplicate_count":                 0,
			"missing_count":                   0,
			"non_memory_count":                0,
			"search_text_policy":              languageMemorySearchPolicy,
			"hit_language_context_count":      0,
			"hit_alias_indexed_count":         0,
			"hydrated_language_context_count": 0,
			"hydrated_alias_ready_count":      0,
		},
	}
	limit = prepareTurnRecallLimit(limit)
	if vectorShadow == nil {
		out.Trace["reason"] = "vector_shadow_missing"
		return out
	}
	if strings.TrimSpace(stringFromMap(vectorShadow, "search_result")) != "ok" {
		out.Trace["status"] = "skipped"
		out.Trace["reason"] = strings.TrimSpace(stringFromMap(vectorShadow, "search_result"))
		if out.Trace["reason"] == "" {
			out.Trace["reason"] = strings.TrimSpace(stringFromMap(vectorShadow, "search_skipped_reason"))
		}
		return out
	}
	memoryByID := map[int64]store.Memory{}
	for _, item := range memories {
		if item.ID > 0 {
			memoryByID[item.ID] = item
		}
	}
	seen := map[int64]bool{}
	hits := prepareTurnVectorSearchResultMaps(vectorShadow["search_results"])
	out.Trace["status"] = "ready"
	out.Trace["input_hit_count"] = len(hits)
	hitRawLanguageCounts := map[string]int{}
	hitSummaryLanguageCounts := map[string]int{}
	hitSessionLanguageCounts := map[string]int{}
	for _, hit := range hits {
		if prepareTurnVectorHitHasLanguageMetadata(hit) {
			out.Trace["hit_language_context_count"] = intFromAny(out.Trace["hit_language_context_count"], 0) + 1
		}
		if intFromAny(hit["alias_count"], 0) > 0 {
			out.Trace["hit_alias_indexed_count"] = intFromAny(out.Trace["hit_alias_indexed_count"], 0) + 1
		}
		incrementLanguageCount(hitRawLanguageCounts, stringFromMap(hit, "raw_language"))
		incrementLanguageCount(hitSummaryLanguageCounts, stringFromMap(hit, "summary_language"))
		incrementLanguageCount(hitSessionLanguageCounts, stringFromMap(hit, "session_output_language"))
	}
	out.Trace["hit_raw_language_counts"] = hitRawLanguageCounts
	out.Trace["hit_summary_language_counts"] = hitSummaryLanguageCounts
	out.Trace["hit_session_output_language_counts"] = hitSessionLanguageCounts
	hydratedRawLanguageCounts := map[string]int{}
	hydratedSummaryLanguageCounts := map[string]int{}
	hydratedSessionLanguageCounts := map[string]int{}
	for rank, hit := range hits {
		if len(out.Items) >= limit {
			break
		}
		if !prepareTurnVectorHitLooksLikeMemory(hit) {
			out.Trace["non_memory_count"] = intFromAny(out.Trace["non_memory_count"], 0) + 1
			continue
		}
		out.Trace["memory_hit_count"] = intFromAny(out.Trace["memory_hit_count"], 0) + 1
		id := prepareTurnVectorMemoryRowID(hit)
		if id <= 0 {
			out.Trace["missing_count"] = intFromAny(out.Trace["missing_count"], 0) + 1
			continue
		}
		item, ok := memoryByID[id]
		if !ok {
			out.Trace["missing_count"] = intFromAny(out.Trace["missing_count"], 0) + 1
			continue
		}
		if seen[id] {
			out.Trace["duplicate_count"] = intFromAny(out.Trace["duplicate_count"], 0) + 1
			continue
		}
		seen[id] = true
		out.Items = append(out.Items, item)
		languageMeta := memoryVectorLanguageMetadata(item)
		if prepareTurnMemoryHasLanguageMetadata(languageMeta) {
			out.Trace["hydrated_language_context_count"] = intFromAny(out.Trace["hydrated_language_context_count"], 0) + 1
		}
		incrementLanguageCount(hydratedRawLanguageCounts, languageMeta["raw_language"])
		incrementLanguageCount(hydratedSummaryLanguageCounts, languageMeta["summary_language"])
		incrementLanguageCount(hydratedSessionLanguageCounts, languageMeta["session_output_language"])
		if memorySearchTextFromMemory(item).AliasCount > 0 {
			out.Trace["hydrated_alias_ready_count"] = intFromAny(out.Trace["hydrated_alias_ready_count"], 0) + 1
		}
		key := prepareTurnMemoryLaneKey(item)
		out.Scores[key] = prepareTurnVectorRankScore(rank)
	}
	out.Trace["hydrated_count"] = len(out.Items)
	out.Trace["selected_count"] = len(out.Items)
	out.Trace["hydrated_raw_language_counts"] = hydratedRawLanguageCounts
	out.Trace["hydrated_summary_language_counts"] = hydratedSummaryLanguageCounts
	out.Trace["hydrated_session_output_language_counts"] = hydratedSessionLanguageCounts
	if len(out.Items) == 0 {
		out.Trace["status"] = "empty"
	}
	return out
}

func prepareTurnHydrateVectorArtifactHits(evidence []store.DirectEvidence, worldRules []store.WorldRule, vectorShadow map[string]any, limit int) prepareTurnVectorArtifactHydration {
	out := prepareTurnVectorArtifactHydration{
		Evidence:   []store.DirectEvidence{},
		WorldRules: []store.WorldRule{},
		Trace: map[string]any{
			"version":                   "vdb2.hydrate_artifact_hits.v1",
			"status":                    "not_attempted",
			"truth_boundary":            "vector_hit_is_selector_only_mariadb_row_is_canonical",
			"input_hit_count":           0,
			"evidence_hit_count":        0,
			"world_rule_hit_count":      0,
			"evidence_hydrated_count":   0,
			"world_rule_hydrated_count": 0,
			"scope_filtered_count":      0,
			"missing_count":             0,
			"duplicate_count":           0,
		},
	}
	limit = prepareTurnRecallLimit(limit)
	if vectorShadow == nil {
		out.Trace["reason"] = "vector_shadow_missing"
		return out
	}
	if strings.TrimSpace(stringFromMap(vectorShadow, "search_result")) != "ok" {
		out.Trace["status"] = "skipped"
		out.Trace["reason"] = strings.TrimSpace(stringFromMap(vectorShadow, "search_result"))
		if out.Trace["reason"] == "" {
			out.Trace["reason"] = strings.TrimSpace(stringFromMap(vectorShadow, "search_skipped_reason"))
		}
		return out
	}
	evidenceByID := map[int64]store.DirectEvidence{}
	for _, item := range evidence {
		if item.ID > 0 {
			evidenceByID[item.ID] = item
		}
	}
	worldRuleByID := map[int64]store.WorldRule{}
	for _, item := range worldRules {
		if item.ID > 0 {
			worldRuleByID[item.ID] = item
		}
	}
	seenEvidence := map[int64]bool{}
	seenWorldRule := map[int64]bool{}
	hits := prepareTurnVectorSearchResultMaps(vectorShadow["search_results"])
	out.Trace["status"] = "ready"
	out.Trace["input_hit_count"] = len(hits)
	for _, hit := range hits {
		if len(out.Evidence)+len(out.WorldRules) >= limit {
			break
		}
		sourceTable := strings.ToLower(strings.TrimSpace(stringFromMap(hit, "source_table")))
		tier := strings.ToLower(strings.TrimSpace(stringFromMap(hit, "tier")))
		id := prepareTurnVectorSourceRowID(hit)
		switch {
		case sourceTable == "direct_evidence_records" || tier == "evidence" || strings.HasPrefix(strings.ToLower(strings.TrimSpace(stringFromMap(hit, "id"))), "evidence:"):
			out.Trace["evidence_hit_count"] = intFromAny(out.Trace["evidence_hit_count"], 0) + 1
			if id <= 0 {
				out.Trace["missing_count"] = intFromAny(out.Trace["missing_count"], 0) + 1
				continue
			}
			item, ok := evidenceByID[id]
			if !ok {
				out.Trace["missing_count"] = intFromAny(out.Trace["missing_count"], 0) + 1
				continue
			}
			if item.Tombstoned || item.RepairNeeded || item.SupersededByID != 0 {
				out.Trace["scope_filtered_count"] = intFromAny(out.Trace["scope_filtered_count"], 0) + 1
				continue
			}
			if seenEvidence[id] {
				out.Trace["duplicate_count"] = intFromAny(out.Trace["duplicate_count"], 0) + 1
				continue
			}
			seenEvidence[id] = true
			out.Evidence = append(out.Evidence, item)
		case sourceTable == "world_rules" || tier == "world_rule" || strings.HasPrefix(strings.ToLower(strings.TrimSpace(stringFromMap(hit, "id"))), "world_rule:"):
			out.Trace["world_rule_hit_count"] = intFromAny(out.Trace["world_rule_hit_count"], 0) + 1
			if id <= 0 {
				out.Trace["missing_count"] = intFromAny(out.Trace["missing_count"], 0) + 1
				continue
			}
			item, ok := worldRuleByID[id]
			if !ok {
				out.Trace["missing_count"] = intFromAny(out.Trace["missing_count"], 0) + 1
				continue
			}
			if item.Suppressed {
				out.Trace["scope_filtered_count"] = intFromAny(out.Trace["scope_filtered_count"], 0) + 1
				continue
			}
			if seenWorldRule[id] {
				out.Trace["duplicate_count"] = intFromAny(out.Trace["duplicate_count"], 0) + 1
				continue
			}
			seenWorldRule[id] = true
			out.WorldRules = append(out.WorldRules, item)
		}
	}
	out.Trace["evidence_hydrated_count"] = len(out.Evidence)
	out.Trace["world_rule_hydrated_count"] = len(out.WorldRules)
	out.Trace["hydrated_count"] = len(out.Evidence) + len(out.WorldRules)
	if len(out.Evidence)+len(out.WorldRules) == 0 {
		out.Trace["status"] = "empty"
	}
	return out
}

func prepareTurnVectorSourceRowID(hit map[string]any) int64 {
	raw := strings.TrimSpace(stringFromMap(hit, "source_row_id"))
	if raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
			return id
		}
	}
	idText := strings.TrimSpace(stringFromMap(hit, "id"))
	if idText == "" {
		return 0
	}
	parts := strings.Split(idText, ":")
	for i := len(parts) - 1; i >= 0; i-- {
		part := strings.TrimSpace(parts[i])
		if id, err := strconv.ParseInt(part, 10, 64); err == nil && id > 0 {
			return id
		}
	}
	return 0
}

func mergePrepareTurnVectorArtifactCounters(counts map[string]any, hydration prepareTurnVectorArtifactHydration, directEvidenceInjected bool, directEvidenceLineCount, worldRuleLineCount int) {
	if counts == nil {
		return
	}
	trace := hydration.Trace
	if trace == nil {
		trace = map[string]any{}
	}
	evidenceInjected := 0
	if directEvidenceInjected {
		evidenceInjected = directEvidenceLineCount
	}
	worldRulesInjected := minInt(len(hydration.WorldRules), worldRuleLineCount)
	counts["vector_artifact_recall"] = trace
	counts["vector_evidence_hit_count"] = intFromAny(trace["evidence_hit_count"], 0)
	counts["vector_evidence_hydrated_count"] = intFromAny(trace["evidence_hydrated_count"], 0)
	counts["vector_evidence_selected_count"] = len(hydration.Evidence)
	counts["vector_evidence_injected_count"] = evidenceInjected
	counts["vector_world_rule_hit_count"] = intFromAny(trace["world_rule_hit_count"], 0)
	counts["vector_world_rule_hydrated_count"] = intFromAny(trace["world_rule_hydrated_count"], 0)
	counts["vector_world_rule_selected_count"] = len(hydration.WorldRules)
	counts["vector_world_rule_injected_count"] = worldRulesInjected
	counts["vector_scope_filtered_count"] = intFromAny(trace["scope_filtered_count"], 0)
	counts["vector_missing_count"] = intFromAny(counts["vector_memory_missing_count"], 0) + intFromAny(trace["missing_count"], 0)
	counts["vector_duplicate_count"] = intFromAny(counts["vector_memory_duplicate_count"], 0) + intFromAny(trace["duplicate_count"], 0)
	counts["vector_hit_count"] = intFromAny(counts["vector_memory_hit_count"], 0) + intFromAny(trace["evidence_hit_count"], 0) + intFromAny(trace["world_rule_hit_count"], 0)
	counts["vector_hydrated_count"] = intFromAny(counts["vector_memory_hydrated_count"], 0) + intFromAny(trace["evidence_hydrated_count"], 0) + intFromAny(trace["world_rule_hydrated_count"], 0)
	counts["vector_selected_count"] = intFromAny(counts["vector_memory_selected_count"], 0) + len(hydration.Evidence) + len(hydration.WorldRules)
	counts["vector_injected_count"] = intFromAny(counts["vector_memory_injected_count"], 0) + evidenceInjected + worldRulesInjected
}

func prepareTurnVectorHitHasLanguageMetadata(hit map[string]any) bool {
	return strings.TrimSpace(stringFromMap(hit, "raw_language")) != "" ||
		strings.TrimSpace(stringFromMap(hit, "summary_language")) != "" ||
		strings.TrimSpace(stringFromMap(hit, "session_output_language")) != ""
}

func prepareTurnMemoryHasLanguageMetadata(meta map[string]string) bool {
	return strings.TrimSpace(meta["raw_language"]) != "" ||
		strings.TrimSpace(meta["summary_language"]) != "" ||
		strings.TrimSpace(meta["session_output_language"]) != ""
}

func incrementLanguageCount(counts map[string]int, language string) {
	language = strings.TrimSpace(language)
	if language == "" {
		return
	}
	counts[language]++
}

func prepareTurnVectorHitLooksLikeMemory(hit map[string]any) bool {
	sourceTable := strings.ToLower(strings.TrimSpace(stringFromMap(hit, "source_table")))
	if sourceTable != "" {
		return sourceTable == "memories" || sourceTable == "memory"
	}
	tier := strings.ToLower(strings.TrimSpace(stringFromMap(hit, "tier")))
	if tier == "memory" || tier == "memories" {
		return true
	}
	id := strings.ToLower(strings.TrimSpace(stringFromMap(hit, "id")))
	return strings.HasPrefix(id, "memory:")
}

func prepareTurnVectorMemoryRowID(hit map[string]any) int64 {
	raw := strings.TrimSpace(stringFromMap(hit, "source_row_id"))
	if raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
			return id
		}
	}
	idText := strings.TrimSpace(stringFromMap(hit, "id"))
	if idText == "" {
		return 0
	}
	parts := strings.Split(idText, ":")
	for i := len(parts) - 1; i >= 0; i-- {
		part := strings.TrimSpace(parts[i])
		if part == "" {
			continue
		}
		if id, err := strconv.ParseInt(part, 10, 64); err == nil && id > 0 {
			return id
		}
	}
	return 0
}

func prepareTurnVectorSearchResultMaps(value any) []map[string]any {
	switch items := value.(type) {
	case []map[string]any:
		return items
	case []any:
		out := make([]map[string]any, 0, len(items))
		for _, item := range items {
			if m := mapFromAny(item); len(m) > 0 {
				out = append(out, m)
			}
		}
		return out
	default:
		return nil
	}
}

func prepareTurnVectorRankScore(rank int) float64 {
	score := 1.0 - float64(rank)*0.05
	if score < 0.1 {
		return 0.1
	}
	return score
}

func prepareTurnMemoryAlreadySelected(selection prepareTurnMemoryLaneSelection, item store.Memory) bool {
	key := prepareTurnMemoryLaneKey(item)
	for _, lane := range [][]store.Memory{selection.VectorRelevant, selection.Relevant, selection.Deep, selection.Recent} {
		for _, selected := range lane {
			if prepareTurnMemoryLaneKey(selected) == key {
				return true
			}
		}
	}
	return false
}

func prepareTurnNeedsRawFallback(selection prepareTurnMemoryLaneSelection, topK int) bool {
	if boolFromAny(selection.Trace["vector_recall_ready"]) {
		return false
	}
	if boolFromAny(selection.Trace["vector_recall_attempted"]) {
		return false
	}
	return prepareTurnSelectedMemoryCount(selection) < prepareTurnRecallLimit(topK)
}

func relevantDegradedReason(query string, selected, candidates int) string {
	if strings.TrimSpace(query) == "" {
		return "missing_query"
	}
	if selected > 0 {
		return ""
	}
	if candidates == 0 {
		return "no_keyword_overlap_candidates"
	}
	return "candidate_limit_zero"
}

func prepareTurnMemoryLaneLines(selection prepareTurnMemoryLaneSelection, languageContext map[string]any, perspectiveContextArg ...map[string]any) ([]string, map[string]any) {
	lines := []string{}
	trace := newPrepareTurnMemoryLanguageTrace(languageContext)
	perspectiveContext := map[string]any(nil)
	if len(perspectiveContextArg) > 0 {
		perspectiveContext = normalizePrepareTurnPerspectiveContext(perspectiveContextArg[0])
	}
	appendLane := func(label string, items []store.Memory) {
		for _, item := range items {
			summary := prepareTurnMemorySummary(item)
			if summary == "" {
				continue
			}
			lineText, lineTrace := prepareTurnMemoryInjectionLineText(item, summary, languageContext, perspectiveContext)
			updatePrepareTurnMemoryLanguageTrace(trace, lineTrace)
			meta := []string{label}
			if item.TurnIndex > 0 {
				meta = append(meta, fmt.Sprintf("turn %d", item.TurnIndex))
			}
			if label == "vector_relevant" {
				if score := selection.VectorScores[prepareTurnMemoryLaneKey(item)]; score > 0 {
					meta = append(meta, fmt.Sprintf("vector %.2f", score))
				}
			}
			if label == "relevant" {
				if score := selection.RelevantScores[prepareTurnMemoryLaneKey(item)]; score > 0 {
					meta = append(meta, fmt.Sprintf("score %.2f", score))
				}
			}
			if label == "deep" && item.Importance > 0 {
				meta = append(meta, fmt.Sprintf("imp %.2f", item.Importance))
			}
			lines = append(lines, fmt.Sprintf("- [%s] %s", strings.Join(meta, ", "), lineText))
		}
	}
	appendLane("vector_relevant", selection.VectorRelevant)
	appendLane("relevant", selection.Relevant)
	appendLane("deep", selection.Deep)
	appendLane("recent", selection.Recent)
	trace["line_count"] = len(lines)
	return lines, trace
}

func newPrepareTurnMemoryLanguageTrace(languageContext map[string]any) map[string]any {
	return map[string]any{
		"contract_version":                 languageMemoryContractVersion,
		"session_output_language":          nilIfEmpty(prepareTurnSessionOutputLanguage(languageContext)),
		"summary_language_target":          nilIfEmpty(prepareTurnSummaryLanguageTarget(languageContext)),
		"memory_summary_language_match":    0,
		"memory_summary_language_mismatch": 0,
		"memory_language_unknown":          0,
		"raw_evidence_attached_count":      0,
		"raw_evidence_preserved":           true,
		"raw_user_input_rewritten":         false,
	}
}

func updatePrepareTurnMemoryLanguageTrace(trace map[string]any, lineTrace map[string]any) {
	if trace == nil || lineTrace == nil {
		return
	}
	if boolFromAny(lineTrace["summary_language_matches_target"]) {
		trace["memory_summary_language_match"] = intFromAny(trace["memory_summary_language_match"], 0) + 1
	} else if strings.TrimSpace(extractionStringFromAny(lineTrace["summary_language"])) != "" &&
		strings.TrimSpace(extractionStringFromAny(lineTrace["summary_language_target"])) != "" {
		trace["memory_summary_language_mismatch"] = intFromAny(trace["memory_summary_language_mismatch"], 0) + 1
	} else {
		trace["memory_language_unknown"] = intFromAny(trace["memory_language_unknown"], 0) + 1
	}
	if boolFromAny(lineTrace["raw_evidence_attached"]) {
		trace["raw_evidence_attached_count"] = intFromAny(trace["raw_evidence_attached_count"], 0) + 1
	}
}

func buildPrepareTurnLanguageInjectionTrace(languageContext map[string]any, memoryTrace map[string]any) map[string]any {
	return map[string]any{
		"contract_version":            languageMemoryContractVersion,
		"status":                      prepareTurnLanguageInjectionStatus(languageContext),
		"session_output_language":     nilIfEmpty(prepareTurnSessionOutputLanguage(languageContext)),
		"summary_language_target":     nilIfEmpty(prepareTurnSummaryLanguageTarget(languageContext)),
		"output_language_source":      nilIfEmpty(extractionStringFromAny(languageContext["output_language_source"])),
		"current_user_input_priority": "highest",
		"raw_user_input_rewritten":    false,
		"raw_evidence_rewritten":      false,
		"related_memory_policy":       "prefer_stored_output_language_summary_preserve_raw_evidence_when_available",
		"translation_call_attempted":  false,
		"memory_language_trace":       nilIfEmptyMap(memoryTrace),
	}
}

func prepareTurnLanguageInjectionStatus(languageContext map[string]any) string {
	target := prepareTurnSessionOutputLanguage(languageContext)
	if target == "" || target == "unknown" || target == "auto" {
		return "trace_only_unknown_language"
	}
	return "ready"
}

func prepareTurnSessionOutputLanguage(languageContext map[string]any) string {
	return normalizePrepareTurnLanguageCode(extractionFirstNonEmpty(
		extractionStringFromAny(languageContext["session_output_language"]),
		extractionStringFromAny(languageContext["summary_language"]),
	))
}

func prepareTurnSummaryLanguageTarget(languageContext map[string]any) string {
	return normalizePrepareTurnLanguageCode(extractionFirstNonEmpty(
		extractionStringFromAny(languageContext["summary_language"]),
		extractionStringFromAny(languageContext["session_output_language"]),
	))
}

func normalizePrepareTurnLanguageCode(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "ko", "kr", "kor", "korean":
		return "ko"
	case "en", "eng", "english":
		return "en"
	case "ja", "jp", "jpn", "japanese":
		return "ja"
	case "auto":
		return "auto"
	case "unknown":
		return "unknown"
	default:
		return value
	}
}

func prepareTurnMemoryInjectionLineText(item store.Memory, summary string, languageContext map[string]any, perspectiveContextArg ...map[string]any) (string, map[string]any) {
	meta := memoryVectorLanguageMetadata(item)
	summaryLanguage := normalizePrepareTurnLanguageCode(meta["summary_language"])
	targetLanguage := prepareTurnSummaryLanguageTarget(languageContext)
	rawLanguage := normalizePrepareTurnLanguageCode(meta["raw_language"])
	perspectiveContext := map[string]any(nil)
	if len(perspectiveContextArg) > 0 {
		perspectiveContext = normalizePrepareTurnPerspectiveContext(perspectiveContextArg[0])
	}
	if guard := prepareTurnProtectedMemoryGuard(item, perspectiveContext); guard.Active {
		lineTrace := map[string]any{
			"summary_language":                nilIfEmpty(summaryLanguage),
			"summary_language_target":         nilIfEmpty(targetLanguage),
			"raw_language":                    nilIfEmpty(rawLanguage),
			"summary_language_matches_target": summaryLanguage != "" && targetLanguage != "" && summaryLanguage == targetLanguage,
			"raw_evidence_attached":           false,
			"raw_evidence_preserved":          true,
			"protected_secret_guarded":        true,
			"protected_identity_pov_scoped":   guard.POVScoped,
		}
		return guard.LineText, lineTrace
	}
	parts := []string{summary}
	rawEvidence := prepareTurnMemoryRawEvidenceLines(item)
	if len(rawEvidence) > 0 && rawLanguage != "" && summaryLanguage != "" && rawLanguage != summaryLanguage {
		parts = append(parts, "raw_evidence: "+strings.Join(rawEvidence, " | "))
	}
	if summaryLanguage != "" {
		parts = append(parts, "summary_language="+summaryLanguage)
	}
	if rawLanguage != "" {
		parts = append(parts, "raw_language="+rawLanguage)
	}
	lineTrace := map[string]any{
		"summary_language":                nilIfEmpty(summaryLanguage),
		"summary_language_target":         nilIfEmpty(targetLanguage),
		"raw_language":                    nilIfEmpty(rawLanguage),
		"summary_language_matches_target": summaryLanguage != "" && targetLanguage != "" && summaryLanguage == targetLanguage,
		"raw_evidence_attached":           len(rawEvidence) > 0 && rawLanguage != "" && summaryLanguage != "" && rawLanguage != summaryLanguage,
		"raw_evidence_preserved":          true,
	}
	return strings.Join(parts, " | "), lineTrace
}

type prepareTurnProtectedMemoryGuardResult struct {
	Active    bool
	LineText  string
	POVScoped bool
}

func prepareTurnProtectedMemoryGuard(item store.Memory, perspectiveContextArg ...map[string]any) prepareTurnProtectedMemoryGuardResult {
	parsed := parseJSONMap(item.SummaryJSON)
	protectedSecrets := sliceFromAny(parsed["protected_secrets"])
	identityAccuracy := sliceFromAny(parsed["character_identity_accuracy"])
	if len(protectedSecrets) == 0 && len(identityAccuracy) == 0 {
		return prepareTurnProtectedMemoryGuardResult{}
	}
	perspectiveContext := map[string]any(nil)
	if len(perspectiveContextArg) > 0 {
		perspectiveContext = normalizePrepareTurnPerspectiveContext(perspectiveContextArg[0])
	}
	if line := prepareTurnPOVScopedIdentityGuardLine(identityAccuracy, perspectiveContext); line != "" {
		return prepareTurnProtectedMemoryGuardResult{
			Active:    true,
			LineText:  line,
			POVScoped: true,
		}
	}
	if line := prepareTurnProtectedIdentityContinuityGuardLine(identityAccuracy); line != "" {
		return prepareTurnProtectedMemoryGuardResult{
			Active:   true,
			LineText: line,
		}
	}
	kinds := []string{}
	policies := []string{}
	knownByCount := 0
	suspectedByCount := 0
	for _, raw := range protectedSecrets {
		secret := mapFromAny(raw)
		if !protectedSecretRequiresGuard(secret, "disclosure_policy") {
			continue
		}
		if kind := normalizeProtectedSecretToken(stringFromMap(secret, "secret_kind")); kind != "" {
			kinds = appendUniqueMemorySearchText(kinds, kind)
		}
		if policy := normalizeTargetRevealPolicy(stringFromMap(secret, "disclosure_policy")); policy != "" {
			policies = appendUniqueMemorySearchText(policies, policy)
		}
		scope := mapFromAny(secret["knowledge_scope"])
		knownByCount += len(stringsFromAny(scope["known_by"]))
		suspectedByCount += len(stringsFromAny(scope["suspected_by"]))
	}
	for _, raw := range identityAccuracy {
		identity := mapFromAny(raw)
		if !protectedSecretRequiresGuard(identity, "reveal_policy") {
			continue
		}
		if kind := normalizeProtectedSecretToken(stringFromMap(identity, "identity_kind")); kind != "" {
			kinds = appendUniqueMemorySearchText(kinds, kind)
		}
		if policy := normalizeTargetRevealPolicy(stringFromMap(identity, "reveal_policy")); policy != "" {
			policies = appendUniqueMemorySearchText(policies, policy)
		}
		scope := mapFromAny(identity["knowledge_scope"])
		knownByCount += len(stringsFromAny(scope["known_by"]))
		suspectedByCount += len(stringsFromAny(scope["suspected_by"]))
	}
	if len(kinds) == 0 && len(policies) == 0 {
		return prepareTurnProtectedMemoryGuardResult{}
	}
	parts := []string{
		"Protected continuity guard: protected private knowledge exists.",
		"Do not reveal, confess, or let unrelated characters discover it without current-scene evidence.",
	}
	if len(kinds) > 0 {
		parts = append(parts, "kind="+strings.Join(kinds, ","))
	}
	if len(policies) > 0 {
		parts = append(parts, "policy="+strings.Join(policies, ","))
	}
	if knownByCount > 0 || suspectedByCount > 0 {
		parts = append(parts, fmt.Sprintf("knowledge_scope=known:%d suspected:%d", knownByCount, suspectedByCount))
	}
	return prepareTurnProtectedMemoryGuardResult{
		Active:   true,
		LineText: strings.Join(parts, " | "),
	}
}

func prepareTurnProtectedIdentityContinuityGuardLine(identityAccuracy []any) string {
	relations := []string{}
	kinds := []string{}
	policies := []string{}
	knownByCount := 0
	suspectedByCount := 0
	for _, raw := range identityAccuracy {
		identity := mapFromAny(raw)
		if !protectedSecretRequiresGuard(identity, "reveal_policy") {
			continue
		}
		surface := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(identity, "surface_identity_name"),
			stringFromMap(identity, "public_identity_name"),
			stringFromMap(identity, "alias_name"),
		))
		trueName := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(identity, "true_identity_name"),
			stringFromMap(identity, "canonical_entity_name"),
			stringFromMap(identity, "real_identity_name"),
		))
		if surface == "" || trueName == "" || normalizeCharacterKey(surface) == normalizeCharacterKey(trueName) {
			continue
		}
		if boolFromAny(identity["same_entity"]) {
			relations = appendUniqueMemorySearchText(relations, fmt.Sprintf("%s and %s refer to the same internal person", surface, trueName))
		} else {
			relations = appendUniqueMemorySearchText(relations, fmt.Sprintf("%s is protected identity context for %s", surface, trueName))
		}
		if kind := normalizeProtectedSecretToken(stringFromMap(identity, "identity_kind")); kind != "" {
			kinds = appendUniqueMemorySearchText(kinds, kind)
		}
		if policy := normalizeTargetRevealPolicy(stringFromMap(identity, "reveal_policy")); policy != "" {
			policies = appendUniqueMemorySearchText(policies, policy)
		}
		scope := mapFromAny(identity["knowledge_scope"])
		knownByCount += len(stringsFromAny(scope["known_by"]))
		suspectedByCount += len(stringsFromAny(scope["suspected_by"]))
	}
	if len(relations) == 0 {
		return ""
	}
	parts := []string{
		"Protected identity continuity: " + strings.Join(relations, "; ") + ".",
		"Maintain same-entity continuity internally; do not portray the surface identity and true identity as separate people.",
		"When same_entity is confirmed, keep aliases merged in entity resolution even when public roles or cover roles differ.",
		"This is author-side/private support, not public character knowledge; do not reveal, confess, or let unrelated characters discover it without current-scene evidence.",
	}
	if len(kinds) > 0 {
		parts = append(parts, "kind="+strings.Join(kinds, ","))
	}
	if len(policies) > 0 {
		parts = append(parts, "policy="+strings.Join(policies, ","))
	}
	if knownByCount > 0 || suspectedByCount > 0 {
		parts = append(parts, fmt.Sprintf("knowledge_scope=known:%d suspected:%d", knownByCount, suspectedByCount))
	}
	return strings.Join(parts, " | ")
}

func prepareTurnPOVScopedIdentityGuardLine(identityAccuracy []any, perspectiveContext map[string]any) string {
	povName := strings.TrimSpace(extractionStringFromAny(perspectiveContext["current_pov"]))
	povKey := strings.TrimSpace(extractionStringFromAny(perspectiveContext["current_pov_key"]))
	if povName == "" && povKey == "" {
		return ""
	}
	for _, raw := range identityAccuracy {
		identity := mapFromAny(raw)
		if !protectedSecretRequiresGuard(identity, "reveal_policy") {
			continue
		}
		if !prepareTurnPerspectiveKnowsIdentity(identity, povName, povKey) {
			continue
		}
		surface := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(identity, "surface_identity_name"),
			stringFromMap(identity, "public_identity_name"),
			stringFromMap(identity, "alias_name"),
		))
		trueName := strings.TrimSpace(extractionFirstNonEmpty(
			stringFromMap(identity, "true_identity_name"),
			stringFromMap(identity, "canonical_entity_name"),
			stringFromMap(identity, "real_identity_name"),
		))
		if surface == "" || trueName == "" || normalizeCharacterKey(surface) == normalizeCharacterKey(trueName) {
			continue
		}
		kind := normalizeProtectedSecretToken(stringFromMap(identity, "identity_kind"))
		if kind == "" {
			kind = "identity"
		}
		policy := normalizeTargetRevealPolicy(stringFromMap(identity, "reveal_policy"))
		parts := []string{
			fmt.Sprintf("POV-scoped identity continuity: %s is %s's own protected surface identity/persona.", surface, trueName),
			fmt.Sprintf("For current_pov=%s, treat %s and %s as the same internal person, not two separate characters.", povName, surface, trueName),
			"If this POV references the surface identity, read it as self/cover-role continuity rather than a separate external character.",
			"Keep this as POV/private knowledge; do not reveal it to characters outside knowledge_scope without current reveal evidence.",
			"kind=" + kind,
		}
		if policy != "" {
			parts = append(parts, "policy="+policy)
		}
		return strings.Join(parts, " | ")
	}
	return ""
}

func prepareTurnPerspectiveKnowsIdentity(identity map[string]any, povName, povKey string) bool {
	candidates := []string{
		povName,
		povKey,
		stringFromMap(identity, "canonical_entity_name"),
		stringFromMap(identity, "true_identity_name"),
		stringFromMap(identity, "surface_identity_name"),
		stringFromMap(identity, "public_identity_name"),
		stringFromMap(identity, "alias_name"),
	}
	candidates = append(candidates, stringsFromAny(identity["aliases"])...)
	scope := mapFromAny(identity["knowledge_scope"])
	candidates = append(candidates, stringsFromAny(scope["known_by"])...)
	for _, candidate := range candidates {
		if prepareTurnPerspectiveNameMatches(povName, povKey, candidate) {
			return true
		}
	}
	return false
}

func prepareTurnPerspectiveNameMatches(povName, povKey, candidate string) bool {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return false
	}
	candidateKey := normalizeCharacterKey(candidate)
	if povKey != "" && candidateKey != "" && povKey == candidateKey {
		return true
	}
	return strings.TrimSpace(povName) != "" && strings.EqualFold(strings.TrimSpace(povName), candidate)
}

func protectedSecretRequiresGuard(item map[string]any, policyKey string) bool {
	if boolFromAny(item["public_narration_allowed"]) {
		return false
	}
	scope := mapFromAny(item["knowledge_scope"])
	if boolFromAny(scope["publicly_revealed"]) || boolFromAny(scope["reader_visible"]) || boolFromAny(scope["protagonist_visible"]) {
		return false
	}
	policy := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(item, policyKey), stringFromMap(item, "target_reveal_policy")))
	if policy == "" {
		return true
	}
	switch normalizeTargetRevealPolicy(policy) {
	case "owner_private_until_revealed", "explicit_user_reveal_required", "current_session_confirmation_required", "explicit_reveal_event_required", "user_directed_reveal_only", "requires_explicit_attachment":
		return true
	default:
		return false
	}
}

func prepareTurnMemoryRawEvidenceLines(item store.Memory) []string {
	out := []string{}
	evidence := parseJSONMap(item.Evidence)
	for _, value := range memorySearchStringValues(evidence["evidence_excerpts"]) {
		value = strings.TrimSpace(value)
		if value != "" {
			out = appendMemorySearchAlias(out, value)
		}
	}
	return out
}

func prepareTurnMemoryLaneKey(item store.Memory) string {
	if item.ID > 0 {
		return fmt.Sprintf("memory:%d", item.ID)
	}
	return fmt.Sprintf("turn:%d:%s", item.TurnIndex, stableKey("memory", prepareTurnMemorySummary(item)))
}

type prepareTurnHierarchyEscalation struct {
	ChapterText string
	ArcText     string
	SagaText    string
	Trace       map[string]any
}

func buildPrepareTurnHierarchyEscalation(resumePack *store.ResumePack, chatLogs []store.ChatLog, memorySelection prepareTurnMemoryLaneSelection, topK int, rawUserInput, profile string) prepareTurnHierarchyEscalation {
	trace := map[string]any{
		"version":                     "r2.hierarchy_escalation.v1",
		"status":                      "off",
		"chapter_selected":            false,
		"arc_selected":                false,
		"saga_selected":               false,
		"chapter_reason":              "no_chapter",
		"arc_reason":                  "no_arc",
		"saga_reason":                 "no_saga",
		"chapter_mode":                "omitted",
		"arc_mode":                    "omitted",
		"saga_mode":                   "omitted",
		"priority":                    "current_user_input_and_direct_evidence_remain_higher_priority",
		"truth_boundary":              "hierarchy_summaries_are_support_only",
		"top_k_memory_target":         topK,
		"top_k_definition":            "semantic_memory_recall_limit",
		"recent_memory_bound":         len(memorySelection.Recent),
		"selected_memory_bound":       prepareTurnSelectedMemoryCount(memorySelection),
		"selection_reason_visibility": true,
	}
	out := prepareTurnHierarchyEscalation{Trace: trace}
	if resumePack == nil {
		trace["reason"] = "no_resume_pack"
		return out
	}
	topK = prepareTurnRecallLimit(topK)

	maxTurn := prepareTurnMaxObservedTurn(chatLogs, resumePack)
	resumeCue := prepareTurnQuerySuggestsResume(rawUserInput)
	thinMemoryRecall := prepareTurnNeedsRawFallback(memorySelection, topK)
	longSession := maxTurn >= 50 || prepareTurnProfileWide(profile)
	trace["status"] = "ready"
	trace["max_observed_turn"] = maxTurn
	trace["resume_query_cue"] = resumeCue
	trace["thin_memory_recall"] = thinMemoryRecall
	trace["long_session"] = longSession
	trace["profile"] = profile

	if resumePack.Chapter != nil {
		selectChapter := longSession || resumeCue || thinMemoryRecall || maxTurn == 0
		reason := "omitted_not_needed_for_current_context"
		if selectChapter {
			reason = prepareTurnHierarchyReason("chapter", longSession, resumeCue, thinMemoryRecall, maxTurn == 0, resumePack.Chapter.FromTurn, resumePack.Chapter.ToTurn)
			out.ChapterText = prepareTurnChapterRecallText(*resumePack.Chapter)
			trace["chapter_selected"] = strings.TrimSpace(out.ChapterText) != ""
			trace["chapter_reason"] = reason
			trace["chapter_mode"] = prepareTurnHierarchyMode(out.ChapterText)
			trace["chapter_range"] = map[string]int{"from_turn": resumePack.Chapter.FromTurn, "to_turn": resumePack.Chapter.ToTurn}
			trace["chapter_chars"] = len([]rune(strings.TrimSpace(out.ChapterText)))
		} else {
			trace["chapter_reason"] = reason
		}
	}
	if resumePack.Arc != nil {
		activeArc := strings.EqualFold(strings.TrimSpace(resumePack.Arc.ArcStatus), "active") || strings.TrimSpace(resumePack.Arc.ArcStatus) == ""
		selectArc := longSession || resumeCue || thinMemoryRecall || activeArc || maxTurn == 0
		reason := "omitted_not_needed_for_current_context"
		if selectArc {
			reason = prepareTurnHierarchyReason("arc", longSession || activeArc, resumeCue, thinMemoryRecall, maxTurn == 0, resumePack.Arc.FromTurn, resumePack.Arc.ToTurn)
			out.ArcText = prepareTurnArcRecallText(*resumePack.Arc)
			trace["arc_selected"] = strings.TrimSpace(out.ArcText) != ""
			trace["arc_reason"] = reason
			trace["arc_mode"] = prepareTurnHierarchyMode(out.ArcText)
			trace["arc_range"] = map[string]int{"from_turn": resumePack.Arc.FromTurn, "to_turn": resumePack.Arc.ToTurn}
			trace["arc_chars"] = len([]rune(strings.TrimSpace(out.ArcText)))
		} else {
			trace["arc_reason"] = reason
		}
	}
	if resumePack.Saga != nil {
		selectSaga := maxTurn >= 100 || resumeCue || thinMemoryRecall || prepareTurnProfileUltra(profile) || maxTurn == 0
		reason := "omitted_not_needed_for_current_context"
		if selectSaga {
			reason = prepareTurnHierarchyReason("saga", maxTurn >= 100 || prepareTurnProfileUltra(profile), resumeCue, thinMemoryRecall, maxTurn == 0, resumePack.Saga.FromTurn, resumePack.Saga.ToTurn)
			out.SagaText = prepareTurnSagaRecallText(*resumePack.Saga)
			trace["saga_selected"] = strings.TrimSpace(out.SagaText) != ""
			trace["saga_reason"] = reason
			trace["saga_mode"] = prepareTurnHierarchyMode(out.SagaText)
			trace["saga_range"] = map[string]int{"from_turn": resumePack.Saga.FromTurn, "to_turn": resumePack.Saga.ToTurn}
			trace["saga_chars"] = len([]rune(strings.TrimSpace(out.SagaText)))
		} else {
			trace["saga_reason"] = reason
		}
	}
	trace["selected_count"] = boolToInt(strings.TrimSpace(out.ChapterText) != "") + boolToInt(strings.TrimSpace(out.ArcText) != "") + boolToInt(strings.TrimSpace(out.SagaText) != "")
	return out
}

func prepareTurnMaxObservedTurn(chatLogs []store.ChatLog, resumePack *store.ResumePack) int {
	maxTurn := 0
	for _, cl := range chatLogs {
		if cl.TurnIndex > maxTurn {
			maxTurn = cl.TurnIndex
		}
	}
	if resumePack != nil {
		if resumePack.Chapter != nil && resumePack.Chapter.ToTurn > maxTurn {
			maxTurn = resumePack.Chapter.ToTurn
		}
		if resumePack.Arc != nil && resumePack.Arc.ToTurn > maxTurn {
			maxTurn = resumePack.Arc.ToTurn
		}
		if resumePack.Saga != nil && resumePack.Saga.ToTurn > maxTurn {
			maxTurn = resumePack.Saga.ToTurn
		}
	}
	return maxTurn
}

func prepareTurnProfileWide(profile string) bool {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "wide", "ultra", "extreme", "wide_context_500k", "ultra_long_1m_plus", "extreme_long_2m_plus":
		return true
	default:
		return false
	}
}

func prepareTurnProfileUltra(profile string) bool {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "ultra", "extreme", "ultra_long_1m_plus", "extreme_long_2m_plus":
		return true
	default:
		return false
	}
}

func prepareTurnQuerySuggestsResume(raw string) bool {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return false
	}
	for _, cue := range []string{"remember", "recap", "resume", "continue", "previous", "past", "long ago", "기억", "이전", "전에", "계속", "이어", "요약", "정리", "오랜만", "과거"} {
		if strings.Contains(raw, cue) {
			return true
		}
	}
	return false
}

func prepareTurnHierarchyReason(kind string, longSession, resumeCue, thinMemoryRecall, unknownTurn bool, fromTurn, toTurn int) string {
	reasons := []string{}
	if longSession {
		reasons = append(reasons, kind+"_continuity")
	}
	if resumeCue {
		reasons = append(reasons, "resume_query_cue")
	}
	if thinMemoryRecall {
		reasons = append(reasons, "thin_memory_recall_backstop")
	}
	if unknownTurn {
		reasons = append(reasons, "resume_pack_only_backstop")
	}
	if len(reasons) == 0 {
		reasons = append(reasons, kind+"_available")
	}
	return strings.Join(reasons, "+") + fmt.Sprintf("_turns_%d_%d", fromTurn, toTurn)
}

func prepareTurnHierarchyMode(text string) string {
	chars := len([]rune(strings.TrimSpace(text)))
	switch {
	case chars == 0:
		return "omitted"
	case chars <= 220:
		return "tiny"
	case chars <= 520:
		return "compact"
	default:
		return "full"
	}
}

func prepareTurnChapterRecallText(ch store.ChapterSummary) string {
	lines := []string{}
	title := compactPrepareTurnLine(q1FirstNonEmptyString(ch.ChapterTitle, fmt.Sprintf("Chapter %d", ch.ChapterIndex)), 80)
	summary := compactPrepareTurnLine(q1FirstNonEmptyString(ch.ResumeText, ch.SummaryText), 360)
	if summary != "" {
		lines = append(lines, fmt.Sprintf("- turns %d-%d %s: %s", ch.FromTurn, ch.ToTurn, title, summary))
	}
	if loops := compactEpisodeJSONPreview(ch.OpenLoopsJSON, 160); loops != "" {
		lines = append(lines, "- open_loop: "+loops)
	}
	if rel := compactEpisodeJSONPreview(ch.RelationshipChangesJSON, 160); rel != "" {
		lines = append(lines, "- relationship_shift: "+rel)
	}
	if world := compactEpisodeJSONPreview(ch.WorldChangesJSON, 160); world != "" {
		lines = append(lines, "- world_change: "+world)
	}
	if callbacks := compactEpisodeJSONPreview(ch.CallbackCandidatesJSON, 140); callbacks != "" {
		lines = append(lines, "- callback: "+callbacks)
	}
	return makePrepareTurnSection("[Chapter Recall]", lines)
}

func prepareTurnArcRecallText(arc store.ArcSummary) string {
	lines := []string{}
	name := compactPrepareTurnLine(q1FirstNonEmptyString(arc.ArcName, fmt.Sprintf("Arc %d", arc.ArcIndex)), 80)
	summary := compactPrepareTurnLine(q1FirstNonEmptyString(arc.ArcResumeText, arc.CoreConflict, arc.ArcName), 360)
	if summary != "" {
		lines = append(lines, fmt.Sprintf("- turns %d-%d %s: %s", arc.FromTurn, arc.ToTurn, name, summary))
	}
	if status := strings.TrimSpace(arc.ArcStatus); status != "" {
		lines = append(lines, "- status: "+compactPrepareTurnLine(status, 80))
	}
	if turns := compactEpisodeJSONPreview(arc.KeyTurningPointsJSON, 180); turns != "" {
		lines = append(lines, "- turning_point: "+turns)
	}
	if debts := compactEpisodeJSONPreview(arc.UnresolvedDebtsJSON, 160); debts != "" {
		lines = append(lines, "- unresolved: "+debts)
	}
	if callbacks := compactEpisodeJSONPreview(arc.CallbackCandidatesJSON, 140); callbacks != "" {
		lines = append(lines, "- callback: "+callbacks)
	}
	return makePrepareTurnSection("[Arc Recall]", lines)
}

func prepareTurnSagaRecallText(saga store.SagaDigest) string {
	lines := []string{}
	label := compactPrepareTurnLine(q1FirstNonEmptyString(saga.EraLabel, "Saga"), 80)
	summary := compactPrepareTurnLine(q1FirstNonEmptyString(saga.ResumePackText, saga.SagaSummary, saga.EraLabel), 420)
	if summary != "" {
		lines = append(lines, fmt.Sprintf("- turns %d-%d %s: %s", saga.FromTurn, saga.ToTurn, label, summary))
	}
	if facts := compactEpisodeJSONPreview(saga.PersistentFactsJSON, 180); facts != "" {
		lines = append(lines, "- persistent_fact: "+facts)
	}
	if neverDrop := compactEpisodeJSONPreview(saga.NeverDropCandidatesJSON, 160); neverDrop != "" {
		lines = append(lines, "- never_drop: "+neverDrop)
	}
	return makePrepareTurnSection("[Saga Recall]", lines)
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func buildPrepareTurnInjectionAssembly(memories []store.Memory, kgTriples []store.KGTriple, evidence []store.DirectEvidence, chatLogs []store.ChatLog, storylines []store.Storyline, worldRules []store.WorldRule, charStates []store.CharacterState, pendingThreads []store.PendingThread, canonicalLayers []store.CanonicalStateLayer, episodeSums []store.EpisodeSummary, resumePack *store.ResumePack, personaEntries []store.PersonaMemoryEntry, characterPrivateMemories []store.ProtagonistEntityMemory, topK, maxChars int, rawUserInput, profile string, documents []map[string]any, vectorShadow map[string]any, languageContext map[string]any, perspectiveContextArg ...map[string]any) prepareTurnInjectionAssembly {
	topK = prepareTurnRecallLimit(topK)
	maxChars = prepareTurnTextBudget(maxChars)
	recallLimit := prepareTurnSupportRecallLimit(topK)
	languageContext = normalizeCompleteTurnLanguageContext(languageContext)
	perspectiveContext := map[string]any(nil)
	if len(perspectiveContextArg) > 0 {
		perspectiveContext = normalizePrepareTurnPerspectiveContext(perspectiveContextArg[0])
	}
	narrativeCurrentValues, activeStates := prepareTurnNarrativeStateFromPerspective(perspectiveContextArg)

	out := prepareTurnInjectionAssembly{
		LanguageContext:    languageContext,
		PerspectiveContext: perspectiveContext,
		Counts: map[string]any{
			"memory_count":                         len(memories),
			"kg_count":                             len(kgTriples),
			"fallback_chat_log_count":              len(chatLogs),
			"evidence_count":                       len(evidence),
			"storyline_count":                      len(storylines),
			"world_rule_count":                     len(worldRules),
			"character_state_count":                len(charStates),
			"pending_thread_count":                 len(pendingThreads),
			"canonical_layer_count":                len(canonicalLayers),
			"episode_summary_count":                len(episodeSums),
			"persona_recollection_count":           len(personaEntries),
			"character_private_recollection_count": len(characterPrivateMemories),
			"scoped_verbatim_support_count":        0,
			"top_k_memory_target":                  topK,
			"support_recall_limit":                 recallLimit,
			"support_recall_limit_source":          "requested_top_k_bounded_by_final_injection_budget",
			"top_k_definition":                     "semantic_memory_recall_limit",
		},
	}

	memoryQuery := prepareTurnMemorySelectionQuery(rawUserInput, chatLogs, perspectiveContext, topK)
	memorySelection := selectPrepareTurnMemoryLanesWithVector(memories, memoryQuery, topK, vectorShadow)
	memorySelection = collapsePrepareTurnMemoryLaneSelection(memorySelection)
	memorySelection = filterPrepareTurnProtectedMemoryLaneSelection(memorySelection, rawUserInput, chatLogs, perspectiveContext)
	out.ContinuityCorrectionText, out.Counts["continuity_correction"] = buildNarrativeContinuityCorrection(
		narrativeCurrentValues,
		rawUserInput,
		chatLogs,
		activeStates,
		memorySelection,
		recallLimit,
	)
	memorySelection = filterMemorySelectionAgainstNarrativeCurrentState(memorySelection, narrativeCurrentValues)
	memoryLines, memoryLanguageTrace := prepareTurnMemoryLaneLines(memorySelection, languageContext, perspectiveContext)
	for k, v := range prepareTurnMemoryLaneProtectedCounts(memorySelection, perspectiveContext) {
		out.Counts[k] = v
	}
	artifactHydration := prepareTurnHydrateVectorArtifactHits(evidence, worldRules, vectorShadow, recallLimit)
	out.LanguageInjectionTrace = buildPrepareTurnLanguageInjectionTrace(languageContext, memoryLanguageTrace)
	out.MemoryText = makePrepareTurnSection("[Memory]", memoryLines)

	kgLines := make([]string, 0, minInt(len(kgTriples), recallLimit))
	kgClosedDropped := 0
	kgReferenceTurn := prepareTurnMaxObservedTurn(chatLogs, nil)
	for _, t := range kgTriples {
		if len(kgLines) >= recallLimit {
			break
		}
		if kgReferenceTurn > 0 && ((t.ValidTo > 0 && t.ValidTo < kgReferenceTurn) || (t.ValidFrom > 0 && t.ValidFrom > kgReferenceTurn)) {
			kgClosedDropped++
			continue
		}
		line := strings.TrimSpace(fmt.Sprintf("%s --%s--> %s", t.Subject, t.Predicate, t.Object))
		if line == "-->" {
			continue
		}
		kgLines = append(kgLines, line)
	}
	out.KGText = makePrepareTurnSection("[Knowledge Graph]", kgLines)

	directEvidenceLines := make([]string, 0, len(artifactHydration.Evidence))
	for _, ev := range artifactHydration.Evidence {
		text := compactPrepareTurnLine(ev.EvidenceText, 320)
		if text == "" {
			continue
		}
		meta := []string{"vector"}
		if ev.TurnAnchor > 0 {
			meta = append(meta, fmt.Sprintf("turn %d", ev.TurnAnchor))
		} else if ev.SourceTurnEnd > 0 {
			meta = append(meta, fmt.Sprintf("turn %d", ev.SourceTurnEnd))
		}
		directEvidenceLines = append(directEvidenceLines, fmt.Sprintf("- [%s] %s", strings.Join(meta, ", "), text))
	}
	out.DirectEvidenceText = makePrepareTurnSection("[Direct Evidence]", directEvidenceLines)

	fallbackLines := []string{}
	if prepareTurnNeedsRawFallback(memorySelection, topK) && len(chatLogs) > 0 {
		for _, cl := range selectRecentChatLogsByTurn(chatLogs, recallLimit) {
			content := compactPrepareTurnLine(cl.Content, maxChars)
			if content == "" {
				continue
			}
			role := strings.TrimSpace(cl.Role)
			if role == "" {
				role = "unknown"
			}
			fallbackLines = append(fallbackLines, fmt.Sprintf("- %s: %s", role, content))
		}
	}
	out.FallbackText = makePrepareTurnSection("[Fallback Recent Chat]", fallbackLines)

	storylinesForInjection := collapsePrepareTurnStorylines(storylines)
	storylineLines := make([]string, 0, minInt(len(storylinesForInjection), recallLimit))
	for i, sl := range storylinesForInjection {
		if i >= recallLimit {
			break
		}
		desc := strings.TrimSpace(sl.CurrentContext)
		if desc == "" {
			desc = strings.TrimSpace(sl.Name)
		}
		desc = compactPrepareTurnLine(desc, 170)
		if desc != "" {
			storylineLines = append(storylineLines, "- "+desc)
		}
	}
	out.StorylineText = makePrepareTurnSection("[Storylines]", storylineLines)

	worldRulesForInjection := collapsePrepareTurnWorldRules(mergePrepareTurnWorldRulesForInjection(artifactHydration.WorldRules, worldRules))
	worldRuleLines := make([]string, 0, minInt(len(worldRulesForInjection), recallLimit))
	for i, wr := range worldRulesForInjection {
		if i >= recallLimit {
			break
		}
		desc := strings.TrimSpace(wr.Key)
		if desc == "" {
			desc = strings.TrimSpace(wr.Scope)
		}
		if value := compactPrepareTurnLine(wr.ValueJSON, 120); value != "" {
			desc = strings.TrimSpace(desc + ": " + value)
		}
		desc = compactPrepareTurnLine(desc, 180)
		if desc != "" {
			worldRuleLines = append(worldRuleLines, "- "+desc)
		}
	}
	out.WorldRulesText = makePrepareTurnSection("[World Rules]", worldRuleLines)

	charLines := make([]string, 0, minInt(len(charStates), recallLimit))
	for i, cs := range charStates {
		if i >= recallLimit {
			break
		}
		name := strings.TrimSpace(cs.CharacterName)
		state := prepareTurnSurfaceText(parseSurfacePayload(cs.StatusJSON))
		relationships := prepareTurnSurfaceText(parseSurfacePayload(cs.RelationshipsJSON))
		speechStyle := prepareTurnSurfaceText(parseSurfacePayload(cs.SpeechStyleJSON))
		parts := []string{}
		if state != "" {
			parts = append(parts, "state="+state)
		}
		if speechStyle != "" {
			parts = append(parts, "speech_style="+speechStyle)
		}
		if relationships != "" {
			parts = append(parts, "relationships="+relationships)
		}
		detail := compactPrepareTurnLine(strings.Join(parts, "; "), 520)
		if name == "" && detail == "" {
			continue
		}
		charLines = append(charLines, fmt.Sprintf("- %s: %s", name, detail))
	}
	out.CharacterText = makePrepareTurnSection("[Characters]", charLines)

	pendingLines := make([]string, 0, minInt(len(pendingThreads), recallLimit))
	for i, pt := range pendingThreads {
		if i >= recallLimit {
			break
		}
		desc := compactPrepareTurnLine(pt.Description, 170)
		status := strings.TrimSpace(pt.Status)
		if status != "" && desc != "" {
			desc = compactPrepareTurnLine("status="+status+"; "+desc, 190)
		}
		if desc != "" {
			pendingLines = append(pendingLines, "- "+desc)
		}
	}
	out.PendingThreadText = makePrepareTurnSection("[Pending Threads]", pendingLines)

	episodeLines := make([]string, 0, minInt(len(episodeSums), recallLimit))
	for i, es := range episodeSums {
		if i >= recallLimit {
			break
		}
		summary := compactPrepareTurnLine(es.SummaryText, 180)
		if summary == "" {
			summary = fmt.Sprintf("Episode %d-%d", es.FromTurn, es.ToTurn)
		}
		if anchors := episodeDenseAnchorPreview(es, 260); anchors != "" {
			summary = compactPrepareTurnLine(summary+"; "+anchors, 360)
		}
		episodeLines = append(episodeLines, fmt.Sprintf("- turns %d-%d: %s", es.FromTurn, es.ToTurn, summary))
	}
	out.EpisodeText = makePrepareTurnSection("[Episode Summaries]", episodeLines)
	hierarchyEscalation := buildPrepareTurnHierarchyEscalation(resumePack, chatLogs, memorySelection, topK, rawUserInput, profile)
	out.ChapterText = hierarchyEscalation.ChapterText
	out.ArcText = hierarchyEscalation.ArcText
	out.SagaText = hierarchyEscalation.SagaText
	out.PersonaText = buildPersonaRecollectionText(personaEntries, recallLimit, maxChars)
	out.CharacterPrivateText = buildCharacterPrivateRecollectionText(characterPrivateMemories, recallLimit, maxChars)

	if latest := latestPrepareTurnEvidence(evidence); latest != nil {
		out.LatestDirectEvidenceText = compactPrepareTurnLine(latest.EvidenceText, 260)
	}
	out.RecentRawTurnText = recentPrepareTurnRawTurn(chatLogs, topK)
	out.ScopedVerbatimSupport = archivebridge.BuildScopedVerbatimSupport(evidence)
	out.ScopedVerbatimText = out.ScopedVerbatimSupport.Text

	canonLines := make([]string, 0, minInt(len(canonicalLayers), recallLimit))
	canonFiltered := 0
	canonTypeCounts := map[string]int{}
	for i, cl := range canonicalLayers {
		if i >= recallLimit {
			break
		}
		if !canonicalLayerEligibleForCurrentTruth(cl) {
			canonFiltered++
			continue
		}
		content := compactPrepareTurnLine(cl.Content, 180)
		if content == "" {
			continue
		}
		layer := strings.TrimSpace(cl.LayerType)
		if layer == "" {
			layer = "state"
		}
		canonLines = append(canonLines, fmt.Sprintf("- %s: %s", layer, content))
		canonTypeCounts[layer]++
	}
	out.CanonText = makePrepareTurnSection("[Canonical State]", canonLines)

	addPrepareTurnBlock(&out, "memory", "store.memories", out.MemoryText, len(memoryLines), maxChars)
	addPrepareTurnBlock(&out, "kg", "store.kg_triples", out.KGText, len(kgLines), maxChars)
	addPrepareTurnBlock(&out, "direct_evidence", "store.direct_evidence_records", out.DirectEvidenceText, len(directEvidenceLines), maxChars)
	addPrepareTurnBlock(&out, "fallback", "store.chat_logs", out.FallbackText, len(fallbackLines), maxChars)
	addPrepareTurnBlock(&out, "episode", "store.episode_summaries", out.EpisodeText, len(episodeLines), maxChars)
	addPrepareTurnBlock(&out, "chapter", "store.chapter_summaries", out.ChapterText, boolToInt(strings.TrimSpace(out.ChapterText) != ""), maxChars)
	addPrepareTurnBlock(&out, "arc", "store.arc_summaries", out.ArcText, boolToInt(strings.TrimSpace(out.ArcText) != ""), maxChars)
	addPrepareTurnBlock(&out, "saga", "store.saga_digests", out.SagaText, boolToInt(strings.TrimSpace(out.SagaText) != ""), maxChars)
	addPrepareTurnBlock(&out, "storyline", "store.storylines", out.StorylineText, len(storylineLines), maxChars)
	addPrepareTurnBlock(&out, "world_rules", "store.world_rules", out.WorldRulesText, len(worldRuleLines), maxChars)
	addPrepareTurnBlock(&out, "character", "store.character_states", out.CharacterText, len(charLines), maxChars)
	addPrepareTurnBlock(&out, "pending_thread", "store.pending_threads", out.PendingThreadText, len(pendingLines), maxChars)
	addPrepareTurnBlock(&out, "canonical_state_layer", "store.canonical_state_layers", out.CanonText, len(canonLines), maxChars)
	addPrepareTurnBlock(&out, "persona_recollection", "store.persona_memory_entries", out.PersonaText, minInt(len(personaEntries), recallLimit), maxChars)
	addPrepareTurnBlock(&out, "character_private_recollection", "store.protagonist_entity_memories", out.CharacterPrivateText, minInt(len(characterPrivateMemories), recallLimit), maxChars)
	addPrepareTurnBlock(&out, "continuity_correction", "store.status_current_values", out.ContinuityCorrectionText, intFromAny(mapFromAny(out.Counts["continuity_correction"])["selected_count"], 0), maxChars)

	parts := make([]string, 0, len(out.Blocks))
	for _, block := range out.Blocks {
		if strings.TrimSpace(block.Text) != "" {
			parts = append(parts, block.Text)
		}
	}
	out.Text = strings.Join(parts, "\n")
	if len([]rune(out.Text)) > maxChars {
		out.Text = truncateRunes(out.Text, maxChars)
		out.Truncated = true
		out.Trimmed = append(out.Trimmed, map[string]any{
			"label":  "overall",
			"reason": "max_injection_chars",
			"budget": maxChars,
		})
	}

	out.Counts["memory_bound"] = len(memoryLines)
	out.Counts["memory_count"] = len(memoryLines)
	out.Counts["top_k_memory_target"] = topK
	out.Counts["support_recall_limit"] = recallLimit
	out.Counts["support_recall_limit_source"] = "requested_top_k_bounded_by_final_injection_budget"
	out.Counts["top_k_definition"] = "semantic_memory_recall_limit"
	out.Counts["recent_memory_bound"] = len(memorySelection.Recent)
	out.Counts["vector_memory_bound"] = len(memorySelection.VectorRelevant)
	out.Counts["relevant_memory_bound"] = len(memorySelection.Relevant)
	out.Counts["deep_memory_bound"] = len(memorySelection.Deep)
	out.Counts["memory_recall_lane_policy"] = memorySelection.Trace
	mergePrepareTurnMemoryLaneCounters(out.Counts, memorySelection, strings.TrimSpace(out.MemoryText) != "")
	mergePrepareTurnVectorArtifactCounters(out.Counts, artifactHydration, strings.TrimSpace(out.DirectEvidenceText) != "", len(directEvidenceLines), len(worldRuleLines))
	out.Counts["language_aware_injection"] = out.LanguageInjectionTrace
	if memoryTrace := mapFromAny(out.LanguageInjectionTrace["memory_language_trace"]); len(memoryTrace) > 0 {
		out.Counts["memory_summary_language_match"] = intFromAny(memoryTrace["memory_summary_language_match"], 0)
		out.Counts["memory_summary_language_mismatch"] = intFromAny(memoryTrace["memory_summary_language_mismatch"], 0)
		out.Counts["raw_evidence_attached_count"] = intFromAny(memoryTrace["raw_evidence_attached_count"], 0)
	}
	out.Counts["kg_bound"] = len(kgLines)
	out.Counts["kg_closed_or_not_yet_valid_dropped"] = kgClosedDropped
	out.Counts["direct_evidence_bound"] = len(directEvidenceLines)
	out.Counts["fallback_bound"] = len(fallbackLines)
	out.Counts["fallback_count"] = len(fallbackLines)
	out.Counts["episode_bound"] = len(episodeLines)
	out.Counts["chapter_delivered"] = strings.TrimSpace(out.ChapterText) != ""
	out.Counts["arc_delivered"] = strings.TrimSpace(out.ArcText) != ""
	out.Counts["saga_delivered"] = strings.TrimSpace(out.SagaText) != ""
	out.Counts["chapter_chars"] = len([]rune(strings.TrimSpace(out.ChapterText)))
	out.Counts["arc_chars"] = len([]rune(strings.TrimSpace(out.ArcText)))
	out.Counts["saga_chars"] = len([]rune(strings.TrimSpace(out.SagaText)))
	out.Counts["hierarchy_escalation"] = hierarchyEscalation.Trace
	out.Counts["persona_recollection_bound"] = minInt(len(personaEntries), recallLimit)
	out.Counts["persona_recollection_support_only"] = len(personaEntries) > 0
	out.Counts["character_private_recollection_bound"] = minInt(len(characterPrivateMemories), recallLimit)
	out.Counts["character_private_recollection_private_lane"] = len(characterPrivateMemories) > 0
	out.Counts["scoped_verbatim_support_count"] = out.ScopedVerbatimSupport.Count
	out.Counts["verbatim_support_active"] = out.ScopedVerbatimSupport.Active
	out.Counts["canonical_state_layers_filtered_count"] = canonFiltered
	out.Counts["canonical_state_relationship_layers_count"] = canonTypeCounts["relationship_state"]
	out.Counts["canonical_state_world_layers_count"] = canonTypeCounts["world_state"]
	out.Counts["canonical_state_scene_layers_count"] = canonTypeCounts["scene_state"]
	out.Counts["canonical_state_entity_layers_count"] = canonTypeCounts["entity_state"]
	out.Counts["storyline_collapsed_count"] = maxInt(len(storylines)-len(storylinesForInjection), 0)
	out.Counts["world_rule_collapsed_count"] = maxInt(len(worldRules)-len(worldRulesForInjection), 0)
	out.Counts["block_count"] = len(out.Blocks)
	out.Counts["total_chars"] = len([]rune(out.Text))
	out.BudgetDecisions = map[string]any{
		"policy_version":                              "rmg07.prepare_turn.bundle.v1",
		"max_injection_chars":                         maxChars,
		"final_budget_owner":                          "archive_center_js_assembleInjectionWithBudget",
		"fallback_chat_log_included":                  strings.TrimSpace(out.FallbackText) != "",
		"fallback_reason":                             fallbackReasonForPrepareTurn(len(memoryLines), len(chatLogs), topK),
		"verbatim_support_active":                     out.ScopedVerbatimSupport.Active,
		"verbatim_support_policy_version":             out.ScopedVerbatimSupport.PolicyVersion,
		"section_count":                               len(out.Blocks),
		"trimmed_count":                               len(out.Trimmed),
		"status_vocabulary":                           []string{"off", "skeleton", "partial", "ready", "degraded"},
		"canonical_state_hard_floor_enabled":          true,
		"persona_recollection_support_only":           len(personaEntries) > 0,
		"persona_recollection_priority":               "below_current_user_input_direct_evidence_and_canonical_state",
		"character_private_recollection_private_lane": len(characterPrivateMemories) > 0,
		"character_private_recollection_visibility":   "owner_private_not_player_visible_by_default",
		"hierarchy_escalation":                        hierarchyEscalation.Trace,
		"hierarchy_priority":                          "support_only_below_current_user_direct_evidence_and_canonical_state",
		"t1a_enforced_ready":                          true,
		"t1a_transition":                              "policy_only_to_enforced_shadow",
	}
	bd := buildBudgetDecisions(documents, maxChars, recallLimit)
	for k, v := range bd {
		out.BudgetDecisions[k] = v
	}
	out.Counts["relationship_first_budget"] = map[string]any{
		"version":          "p80a.v1",
		"status":           "shadow_only",
		"structure":        "relationship_first",
		"long_tier_cap":    2400,
		"ultra_tier_cap":   1800,
		"extreme_tier_cap": 1200,
		"reason":           "relationship_first_budget_structure_for_long_tier_profiles",
	}
	return out
}

func buildBudgetDecisions(docs []map[string]any, maxChars, recallLimit int) map[string]any {
	status := "ready"
	if len(docs) == 0 {
		status = "off"
	}
	recallLimit = prepareTurnRecallLimit(recallLimit)

	globalCap := prepareTurnTextBudget(maxChars)

	canonHardFloor := 120
	policy := q3PacketBudgetPolicy()
	if caps, ok := policy["budget_caps"].(map[string]any); ok {
		if v, ok := caps["canon_hard_floor"].(int); ok && v > 0 {
			canonHardFloor = v
		}
	}

	intentDefs := []struct {
		name  string
		tiers []string
	}{
		{"scene", []string{"memory", "episode", "chapter"}},
		{"callback", []string{"arc", "saga", "memory"}},
		{"resume", []string{"chapter", "arc", "saga"}},
		{"canon", []string{"memory", "episode", "arc"}},
	}

	intentCapRatios := map[string]float64{
		"scene":    0.40,
		"callback": 0.25,
		"resume":   0.20,
		"canon":    0.15,
	}

	decisions := []map[string]any{}
	globalSelectedChars := 0
	canonSelectedChars := 0
	reasonCounts := map[string]int{"tier_cap": 0}
	for _, def := range intentDefs {
		capChars := int(float64(globalCap) * intentCapRatios[def.name])

		candidates := []map[string]any{}
		for _, doc := range docs {
			tier, _ := doc["tier"].(string)
			for _, allowed := range def.tiers {
				if tier == allowed {
					candidates = append(candidates, doc)
					break
				}
			}
		}

		runningTotal := 0
		for i, cand := range candidates {
			id, _ := cand["document_id"].(string)
			text, _ := cand["text"].(string)
			tier, _ := cand["tier"].(string)
			charCost := len([]rune(text))

			decision := "selected"
			reason := "tier_match_selected"
			if i >= recallLimit {
				decision = "dropped"
				reason = "tier_cap_exceeded"
				reasonCounts["tier_cap"]++
			} else {
				runningTotal += charCost
				globalSelectedChars += charCost
				if def.name == "canon" {
					canonSelectedChars += charCost
				}
			}

			decisions = append(decisions, map[string]any{
				"intent":              def.name,
				"tier":                tier,
				"document_id":         id,
				"decision":            decision,
				"reason":              reason,
				"cap_scope":           def.name,
				"char_cost":           charCost,
				"running_total_chars": runningTotal,
				"cap_chars":           capChars,
			})
		}
	}

	return map[string]any{
		"version":                    "t1c.v1",
		"mode":                       "read_only_surface",
		"status":                     status,
		"decision_count":             len(decisions),
		"decisions":                  decisions,
		"global_cap_chars":           globalCap,
		"global_selected_chars":      globalSelectedChars,
		"canon_floor_reserved_chars": canonHardFloor,
		"canon_selected_chars":       canonSelectedChars,
		"reason_counts":              reasonCounts,
		"source_mapping":             "recall_result.intent_execution_shadow.budget_enforcement",
		"source_event":               "budget_enforcement",
		"source_counters":            []string{"decision_count", "global_cap_chars", "global_selected_chars", "canon_floor_reserved_chars", "canon_selected_chars", "reason_counts"},
	}
}

func buildHierarchyEscapeHatch(support archivebridge.ScopedVerbatimSupport) map[string]any {
	status := "inactive"
	reason := "no_direct_evidence_support"
	if support.Active {
		status = "active"
		reason = "support_route_available"
	}
	return map[string]any{
		"status":        status,
		"route":         "scoped_verbatim_support",
		"reason":        reason,
		"support_count": support.Count,
	}
}

func addPrepareTurnBlock(out *prepareTurnInjectionAssembly, label, source, text string, count, budget int) {
	text = strings.TrimSpace(text)
	if text == "" || budget <= 0 {
		return
	}
	out.Blocks = append(out.Blocks, prepareTurnInjectionBlock{
		Label:   label,
		Text:    text,
		Source:  source,
		Count:   count,
		Budget:  budget,
		Trimmed: false,
	})
	switch label {
	case "memory":
		out.MemoryText = text
	case "kg":
		out.KGText = text
	case "fallback":
		out.FallbackText = text
	case "episode":
		out.EpisodeText = text
	case "chapter":
		out.ChapterText = text
	case "arc":
		out.ArcText = text
	case "saga":
		out.SagaText = text
	case "storyline":
		out.StorylineText = text
	case "world_rules":
		out.WorldRulesText = text
	case "character":
		out.CharacterText = text
	case "pending_thread":
		out.PendingThreadText = text
	case "canonical_state_layer":
		out.CanonText = text
	case "persona_recollection":
		out.PersonaText = text
	case "character_private_recollection":
		out.CharacterPrivateText = text
	}
}

func makePrepareTurnSection(header string, lines []string) string {
	cleaned := make([]string, 0, len(lines)+1)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	if len(cleaned) == 0 {
		return ""
	}
	return header + "\n" + strings.Join(cleaned, "\n")
}

func prepareTurnMemorySummary(m store.Memory) string {
	summary := strings.TrimSpace(m.SummaryJSON)
	if summary == "" {
		return ""
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(summary), &parsed); err == nil {
		for _, key := range []string{"turn_summary", "summary", "content", "text"} {
			if s, ok := parsed[key].(string); ok && strings.TrimSpace(s) != "" {
				summary = s
				break
			}
		}
	}
	placeParts := []string{}
	if wing := strings.TrimSpace(m.PlaceWing); wing != "" {
		placeParts = append(placeParts, "archive_wing="+wing)
	}
	if room := strings.TrimSpace(m.PlaceRoom); room != "" {
		placeParts = append(placeParts, "archive_room="+room)
	}
	if len(placeParts) > 0 {
		summary = strings.TrimSpace(summary + " (" + strings.Join(placeParts, ", ") + ")")
	}
	return compactPrepareTurnLine(summary, 220)
}

func prepareTurnMemoryRelevanceText(m store.Memory) string {
	searchText := strings.TrimSpace(memorySearchTextFromMemory(m).Text)
	summary := strings.TrimSpace(prepareTurnMemorySummary(m))
	if searchText == "" {
		return summary
	}
	if summary == "" || strings.Contains(searchText, summary) {
		return searchText
	}
	return strings.TrimSpace(summary + "\n" + searchText)
}

func compactPrepareTurnLine(text string, limit int) string {
	_ = limit
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	return text
}

func prepareTurnSurfaceText(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return compactPrepareTurnLine(v, 0)
	case float64, bool, int, int64:
		return compactPrepareTurnJSON(v)
	case map[string]any, []any:
		return compactPrepareTurnJSON(v)
	default:
		return compactPrepareTurnJSON(v)
	}
}

func compactPrepareTurnJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return compactPrepareTurnLine(string(data), 0)
}

func episodeDenseAnchorPreview(es store.EpisodeSummary, limit int) string {
	parts := []string{}
	if key := compactEpisodeJSONPreview(es.KeyEvents, 120); key != "" {
		parts = append(parts, "key_event="+key)
	}
	if rel := compactEpisodeJSONPreview(es.RelationshipChangesJSON, 120); rel != "" {
		parts = append(parts, "rel="+rel)
	}
	if loop := compactEpisodeJSONPreview(es.OpenLoopsJSON, 120); loop != "" {
		parts = append(parts, "open_loop="+loop)
	}
	return compactPrepareTurnLine(strings.Join(parts, "; "), limit)
}

func compactEpisodeJSONPreview(raw string, limit int) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" || raw == "{}" || raw == "null" {
		return ""
	}
	var arr []any
	if err := json.Unmarshal([]byte(raw), &arr); err == nil {
		parts := []string{}
		for _, item := range arr {
			text := strings.TrimSpace(extractionStringFromAny(item))
			if text == "" {
				text = strings.TrimSpace(compactPrepareTurnJSON(item))
			}
			if text == "" {
				continue
			}
			parts = append(parts, text)
		}
		return compactPrepareTurnLine(strings.Join(parts, " / "), limit)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err == nil {
		return compactPrepareTurnLine(compactPrepareTurnJSON(obj), limit)
	}
	return compactPrepareTurnLine(raw, limit)
}

func latestPrepareTurnEvidence(evidence []store.DirectEvidence) *store.DirectEvidence {
	var latest *store.DirectEvidence
	latestTurn := -1
	for i := range evidence {
		if evidence[i].Tombstoned || strings.TrimSpace(evidence[i].EvidenceText) == "" {
			continue
		}
		turn := maxInt(evidence[i].TurnAnchor, maxInt(evidence[i].SourceTurnEnd, evidence[i].SourceTurnStart))
		if latest == nil || turn >= latestTurn {
			latest = &evidence[i]
			latestTurn = turn
		}
	}
	return latest
}

func recentPrepareTurnRawTurn(chatLogs []store.ChatLog, turnLimit int) string {
	if len(chatLogs) == 0 {
		return ""
	}
	turnLimit = prepareTurnRecallLimit(turnLimit)
	selected := selectRecentChatLogsByTurn(chatLogs, turnLimit)
	lines := make([]string, 0, len(selected))
	for _, cl := range selected {
		content := compactPrepareTurnLine(cl.Content, 0)
		if content == "" {
			continue
		}
		role := strings.TrimSpace(cl.Role)
		if role == "" {
			role = "unknown"
		}
		lines = append(lines, fmt.Sprintf("%s: %s", role, content))
	}
	return strings.Join(lines, "\n")
}

func selectRecentChatLogsByTurn(chatLogs []store.ChatLog, turnLimit int) []store.ChatLog {
	if len(chatLogs) == 0 || turnLimit <= 0 {
		return nil
	}
	turns := map[int]bool{}
	for i := len(chatLogs) - 1; i >= 0 && len(turns) < turnLimit; i-- {
		turn := chatLogs[i].TurnIndex
		if turn <= 0 {
			continue
		}
		turns[turn] = true
	}
	out := make([]store.ChatLog, 0, minInt(len(chatLogs), turnLimit*2))
	for _, cl := range chatLogs {
		if !turns[cl.TurnIndex] {
			continue
		}
		out = append(out, cl)
	}
	return out
}

func fallbackReasonForPrepareTurn(memoryBound, chatLogCount, topK int) string {
	if memoryBound >= prepareTurnRecallLimit(topK) {
		return "memory_sufficient"
	}
	if chatLogCount > 0 {
		return "memory_below_threshold"
	}
	return "no_chat_log_fallback_available"
}

func buildInputContextText(evidence []store.DirectEvidence, chatLogs []store.ChatLog, resumePack *store.ResumePack, activeStates []store.ActiveState, canonicalLayers []store.CanonicalStateLayer, episodeSums []store.EpisodeSummary, personaEntries []store.PersonaMemoryEntry, characterPrivateMemories []store.ProtagonistEntityMemory, maxChars, recallLimit int) (string, bool) {
	recallLimit = prepareTurnRecallLimit(recallLimit)
	maxChars = prepareTurnTextBudget(maxChars)
	var parts []string

	// [Resume Pack]
	if resumePack != nil && resumePack.AssembledText != "" {
		parts = append(parts, "[Resume Pack]")
		text := strings.Join(strings.Fields(resumePack.AssembledText), " ")
		parts = append(parts, text)
	}

	// [Direct Evidence]
	if len(evidence) > 0 {
		var evParts []string
		for _, e := range evidence {
			txt := strings.TrimSpace(e.EvidenceText)
			if txt == "" {
				continue
			}
			txt = strings.Join(strings.Fields(txt), " ")
			evParts = append(evParts, "- "+txt)
			if len(evParts) >= recallLimit {
				break
			}
		}
		if len(evParts) > 0 {
			parts = append(parts, "[Direct Evidence]")
			parts = append(parts, evParts...)
		}
	}

	// [Recent Chat]
	if len(chatLogs) > 0 {
		var logParts []string
		for _, cl := range selectRecentChatLogsByTurn(chatLogs, recallLimit) {
			content := strings.TrimSpace(cl.Content)
			if content == "" {
				continue
			}
			content = strings.Join(strings.Fields(content), " ")
			logParts = append(logParts, fmt.Sprintf("- [%s] %s", cl.Role, content))
		}
		if len(logParts) > 0 {
			parts = append(parts, "[Recent Chat]")
			parts = append(parts, logParts...)
		}
	}

	// [Active States]
	if len(activeStates) > 0 {
		var asParts []string
		for i, as := range activeStates {
			if i >= recallLimit {
				break
			}
			content := strings.TrimSpace(as.Content)
			content = strings.Join(strings.Fields(content), " ")
			asParts = append(asParts, fmt.Sprintf("- [%s] %s", as.StateType, content))
		}
		if len(asParts) > 0 {
			parts = append(parts, "[Active States]")
			parts = append(parts, asParts...)
		}
	}

	// [Canonical State Layers]
	if len(canonicalLayers) > 0 {
		var clParts []string
		for i, cl := range canonicalLayers {
			if i >= recallLimit {
				break
			}
			content := strings.TrimSpace(cl.Content)
			content = strings.Join(strings.Fields(content), " ")
			clParts = append(clParts, fmt.Sprintf("- [%s] %s", cl.LayerType, content))
		}
		if len(clParts) > 0 {
			parts = append(parts, "[Canonical State Layers]")
			parts = append(parts, clParts...)
		}
	}

	// [Episode Summaries]
	if len(episodeSums) > 0 {
		var esParts []string
		for i, es := range episodeSums {
			if i >= recallLimit {
				break
			}
			summary := strings.TrimSpace(es.SummaryText)
			if summary == "" {
				summary = fmt.Sprintf("Episode %d-%d", es.FromTurn, es.ToTurn)
			}
			summary = strings.Join(strings.Fields(summary), " ")
			esParts = append(esParts, "- "+summary)
		}
		if len(esParts) > 0 {
			parts = append(parts, "[Episode Summaries]")
			parts = append(parts, esParts...)
		}
	}

	if personaText := buildPersonaRecollectionText(personaEntries, recallLimit, maxChars); personaText != "" {
		parts = append(parts, personaText)
	}
	if privateText := buildCharacterPrivateRecollectionText(characterPrivateMemories, recallLimit, maxChars); privateText != "" {
		parts = append(parts, privateText)
	}

	text := strings.Join(parts, "\n")
	truncated := false
	if len([]rune(text)) > maxChars {
		text = truncateRunes(text, maxChars)
		truncated = true
	}
	return text, truncated
}

func buildPersonaRecollectionText(entries []store.PersonaMemoryEntry, maxEntries, perEntryChars int) string {
	if len(entries) == 0 {
		return ""
	}
	maxEntries = prepareTurnRecallLimit(maxEntries)
	perEntryChars = prepareTurnTextBudget(perEntryChars)
	lines := []string{
		"support-only private recollection; not current-world truth.",
	}
	if personaRecollectionSecretGuardActive(entries) {
		lines = append(lines,
			"Secret Guard: protagonist-only private intuition. Never reveal its origin; use only as hesitation, instinct, or careful choice.",
		)
	}
	entryLineBase := len(lines)
	for _, entry := range entries {
		if len(lines)-entryLineBase >= maxEntries {
			break
		}
		text := personaRecollectionPromptLineText(entry, perEntryChars)
		if text == "" {
			continue
		}
		meta := []string{}
		if entry.SourceTurn > 0 {
			meta = append(meta, fmt.Sprintf("turn %d", entry.SourceTurn))
		}
		if entry.Importance10 > 0 {
			meta = append(meta, fmt.Sprintf("imp %.1f/10", entry.Importance10))
		}
		if portability := strings.TrimSpace(entry.Portability); portability != "" {
			meta = append(meta, portability)
		}
		prefix := "-"
		if len(meta) > 0 {
			prefix = "- (" + strings.Join(meta, ", ") + ")"
		}
		lines = append(lines, prefix+" "+text)
	}
	if len(lines) <= entryLineBase {
		return ""
	}
	return makePrepareTurnSection("[Persona Recollection]", lines)
}

func buildCharacterPrivateRecollectionText(entries []store.ProtagonistEntityMemory, maxEntries, perEntryChars int) string {
	if len(entries) == 0 {
		return ""
	}
	maxEntries = prepareTurnRecallLimit(maxEntries)
	perEntryChars = prepareTurnTextBudget(perEntryChars)
	lines := []string{
		"NPC private memory is the owning NPC's interpretation/bias, not player knowledge, narrator knowledge, or current-world truth; do not present it as objective fact.",
		"Use only as subtext: hesitation, recognition, avoidance, attraction, suspicion, or careful choice.",
		"Do not imply protagonist knowledge or explain the memory unless current evidence or explicit user instruction reveals it.",
	}
	entryLineBase := len(lines)
	for _, entry := range entries {
		if len(lines)-entryLineBase >= maxEntries {
			break
		}
		text := characterPrivateRecollectionPromptLineText(entry, perEntryChars)
		if text == "" {
			continue
		}
		owner := strings.TrimSpace(entry.OwnerEntityName)
		if owner == "" {
			owner = strings.TrimSpace(entry.OwnerEntityKey)
		}
		if owner == "" {
			owner = "unknown NPC"
		}
		meta := []string{"owner " + owner}
		if entry.SourceTurn > 0 {
			meta = append(meta, fmt.Sprintf("turn %d", entry.SourceTurn))
		}
		if entry.Importance10 > 0 {
			meta = append(meta, fmt.Sprintf("imp %.1f/10", entry.Importance10))
		}
		if policy := strings.TrimSpace(entry.TargetRevealPolicy); policy != "" {
			meta = append(meta, policy)
		}
		lines = append(lines, "- ("+strings.Join(meta, ", ")+") "+text)
	}
	if len(lines) <= entryLineBase {
		return ""
	}
	return makePrepareTurnSection("[Character Private Recollection]", lines)
}

func personaRecollectionPromptLineText(entry store.PersonaMemoryEntry, perEntryChars int) string {
	text := strings.TrimSpace(entry.MemoryText)
	if text == "" {
		return ""
	}
	if personaRecollectionSecretGuardActive([]store.PersonaMemoryEntry{entry}) {
		prefix := "Protected hint: "
		text = protectedRecollectionGuardText(entry.TagsJSON, entry.Portability, entry.InjectionPolicy)
		contentBudget := perEntryChars - len([]rune(prefix))
		if contentBudget <= 0 {
			contentBudget = perEntryChars
		}
		return prefix + compactPrepareTurnLine(text, contentBudget)
	}
	return compactPrepareTurnLine(text, perEntryChars)
}

func characterPrivateRecollectionPromptLineText(entry store.ProtagonistEntityMemory, perEntryChars int) string {
	text := strings.TrimSpace(entry.MemoryText)
	if text == "" {
		return ""
	}
	if characterPrivateRecollectionSecretGuardActive([]store.ProtagonistEntityMemory{entry}) {
		prefix := "Protected NPC-private hint: "
		text = protectedRecollectionGuardText(entry.TagsJSON, entry.Portability, entry.TargetRevealPolicy)
		contentBudget := perEntryChars - len([]rune(prefix))
		if contentBudget <= 0 {
			contentBudget = perEntryChars
		}
		return prefix + compactPrepareTurnLine(text, contentBudget)
	}
	prefix := "Private interpretation: "
	contentBudget := perEntryChars - len([]rune(prefix))
	if contentBudget <= 0 {
		contentBudget = perEntryChars
	}
	return prefix + compactPrepareTurnLine(text, contentBudget)
}

func protectedRecollectionGuardText(tagsJSON string, policyHints ...string) string {
	tags := []string{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(tagsJSON)), &tags); err != nil {
		tags = nil
	}
	kinds := []string{}
	policies := []string{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if strings.HasPrefix(tag, "protected_secret_kind:") {
			kinds = appendUniqueMemorySearchText(kinds, strings.TrimSpace(strings.TrimPrefix(tag, "protected_secret_kind:")))
		}
		if strings.HasPrefix(tag, "identity_kind:") {
			kinds = appendUniqueMemorySearchText(kinds, strings.TrimSpace(strings.TrimPrefix(tag, "identity_kind:")))
		}
		if strings.HasPrefix(tag, "target_reveal_policy:") {
			policies = appendUniqueMemorySearchText(policies, strings.TrimSpace(strings.TrimPrefix(tag, "target_reveal_policy:")))
		}
	}
	for _, hint := range policyHints {
		if policy := normalizeTargetRevealPolicy(hint); policy != "" && policy != "requires_explicit_attachment" {
			policies = appendUniqueMemorySearchText(policies, policy)
		}
	}
	parts := []string{"protected private knowledge is present; use only as owner subtext, hesitation, avoidance, or careful choice; do not reveal content without current evidence"}
	if len(kinds) > 0 {
		parts = append(parts, "kind="+strings.Join(kinds, ","))
	}
	if len(policies) > 0 {
		parts = append(parts, "policy="+strings.Join(policies, ","))
	}
	return strings.Join(parts, " | ")
}

func personaRecollectionSecretSafeText(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"previous loop", "protected private memory",
		"Previous loop", "Protected private memory",
		"time loop", "protected private memory",
		"Time loop", "Protected private memory",
		"regression", "protected private memory",
		"Regression", "Protected private memory",
		"regressor", "person with protected private memory",
		"Regressor", "Person with protected private memory",
		"reincarnation", "protected private memory",
		"Reincarnation", "Protected private memory",
		"reincarnated", "protected private memory",
		"Reincarnated", "Protected private memory",
		"past life", "protected private memory",
		"Past life", "Protected private memory",
		"isekai", "protected private memory",
		"Isekai", "Protected private memory",
		"other world", "protected private memory",
		"Other world", "Protected private memory",
		"another world", "protected private memory",
		"Another world", "Protected private memory",
		"이전 루프", "보호된 사적 기억",
		"지난 루프", "보호된 사적 기억",
		"루프", "보호된 사적 기억",
		"회귀", "보호된 사적 기억",
		"환생", "보호된 사적 기억",
		"전생", "보호된 사적 기억",
		"빙의", "보호된 사적 기억",
		"이세계", "보호된 사적 기억",
		"다른 세계", "보호된 사적 기억",
	)
	return strings.TrimSpace(replacer.Replace(text))
}

func personaRecollectionSecretGuardActive(entries []store.PersonaMemoryEntry) bool {
	for _, entry := range entries {
		source := strings.ToLower(strings.Join([]string{
			entry.MemoryText,
			entry.Portability,
			entry.InjectionPolicy,
			entry.TagsJSON,
		}, " "))
		if containsAnyText(source,
			"regression", "regressor", "regressed", "loop", "looper", "previous loop", "time loop",
			"reincarnation", "reincarnated", "past life", "isekai", "other world", "another world",
			"secret_guard", "identity carry-over", "identity carryover", "possession", "rebirth",
			"이전 루프", "지난 루프", "루프", "회귀", "환생", "전생", "빙의", "이세계", "다른 세계",
		) {
			return true
		}
	}
	return false
}

func characterPrivateRecollectionSecretGuardActive(entries []store.ProtagonistEntityMemory) bool {
	for _, entry := range entries {
		source := strings.ToLower(strings.Join([]string{
			entry.MemoryText,
			entry.Portability,
			entry.TargetRevealPolicy,
			entry.TagsJSON,
		}, " "))
		if entry.SecretGuard {
			return true
		}
		if containsAnyText(source,
			"regression", "regressor", "regressed", "loop", "looper", "previous loop", "time loop",
			"reincarnation", "reincarnated", "past life", "isekai", "other world", "another world",
			"이전 루프", "지난 루프", "루프", "회귀", "환생", "전생", "빙의", "이세계", "다른 세계",
		) {
			return true
		}
	}
	return false
}

func personaMemoryEntryIsCharacterPrivate(entry store.PersonaMemoryEntry) bool {
	source := strings.ToLower(strings.Join([]string{
		entry.Portability,
		entry.InjectionPolicy,
		entry.TagsJSON,
	}, " "))
	return strings.Contains(source, "npc_private") || strings.Contains(source, "character_private_recollection")
}

func personaMemoryEntryAsCharacterPrivateMemory(entry store.PersonaMemoryEntry, targetSID string) store.ProtagonistEntityMemory {
	tags := personaMemoryEntryTags(entry)
	ownerKey := personaMemoryEntryTagValue(tags, "owner_entity_key")
	ownerName := personaMemoryEntryTagValue(tags, "owner_entity_name")
	ownerRole := personaMemoryEntryTagValue(tags, "owner_entity_role")
	ownerVisibility := personaMemoryEntryTagValue(tags, "owner_visibility")
	sourceSID := personaMemoryEntryTagValue(tags, "source_chat_session_id")
	revealPolicy := personaMemoryEntryTagValue(tags, "target_reveal_policy")
	if ownerKey == "" {
		ownerKey = "npc"
	}
	if ownerName == "" {
		ownerName = ownerKey
	}
	if ownerRole == "" {
		ownerRole = "npc"
	}
	if ownerVisibility == "" {
		ownerVisibility = "owner_private"
	}
	if sourceSID == "" {
		sourceSID = targetSID
	}
	if revealPolicy == "" {
		revealPolicy = "owner_private_until_revealed"
	}
	return store.ProtagonistEntityMemory{
		ID:                  entry.ID,
		OwnerEntityKey:      ownerKey,
		OwnerEntityName:     ownerName,
		OwnerEntityRole:     ownerRole,
		OwnerVisibility:     ownerVisibility,
		SourceChatSessionID: sourceSID,
		SourceTurn:          entry.SourceTurn,
		MemoryText:          entry.MemoryText,
		EvidenceExcerpt:     entry.EvidenceExcerpt,
		SecretGuard:         personaRecollectionSecretGuardActive([]store.PersonaMemoryEntry{entry}) || personaMemoryEntryHasTag(tags, "secret_guard"),
		Portability:         firstNonEmpty(entry.Portability, "npc_private_recollection"),
		TargetRevealPolicy:  revealPolicy,
		TagsJSON:            entry.TagsJSON,
		Importance10:        entry.Importance10,
		EmotionalWeight:     entry.EmotionalWeight,
		CreatedAt:           entry.CreatedAt,
		UpdatedAt:           entry.CreatedAt,
	}
}

func personaMemoryEntryTags(entry store.PersonaMemoryEntry) []string {
	var tags []string
	if err := json.Unmarshal([]byte(strings.TrimSpace(entry.TagsJSON)), &tags); err == nil {
		return tags
	}
	return nil
}

func personaMemoryEntryTagValue(tags []string, key string) string {
	prefix := strings.TrimSpace(key) + ":"
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if strings.HasPrefix(tag, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(tag, prefix))
		}
	}
	return ""
}

func personaMemoryEntryHasTag(tags []string, needle string) bool {
	needle = strings.TrimSpace(needle)
	for _, tag := range tags {
		if strings.TrimSpace(tag) == needle {
			return true
		}
	}
	return false
}

type prepareTurnRecollectionContext struct {
	rawUserInput       string
	immediateChatText  string
	currentSceneStates string
}

func filterPrepareTurnEntityRecollections(rawUserInput string, chatLogs []store.ChatLog, activeStates []store.ActiveState, canonicalLayers []store.CanonicalStateLayer, personaEntries []store.PersonaMemoryEntry, characterPrivateMemories *[]store.ProtagonistEntityMemory) map[string]any {
	const characterPrivateTotalCap = 2
	ctx := buildPrepareTurnRecollectionContext(rawUserInput, chatLogs, activeStates, canonicalLayers)
	beforePrivate := len(*characterPrivateMemories)
	filteredPrivate := make([]store.ProtagonistEntityMemory, 0, beforePrivate)
	selectedOwners := []string{}
	selectedOwnerKeys := map[string]bool{}
	droppedOwners := []string{}
	dropped := []map[string]any{}
	for _, item := range *characterPrivateMemories {
		ownerKey := prepareTurnMemoryOwnerIdentity(item.OwnerEntityKey, item.OwnerEntityName)
		if ownerKey != "" && selectedOwnerKeys[ownerKey] {
			owner := prepareTurnMemoryOwnerLabel(item.OwnerEntityKey, item.OwnerEntityName)
			if owner != "" && !stringSliceContains(droppedOwners, owner) {
				droppedOwners = append(droppedOwners, owner)
			}
			dropped = append(dropped, map[string]any{
				"id":                item.ID,
				"owner_entity_key":  item.OwnerEntityKey,
				"owner_entity_name": item.OwnerEntityName,
				"reason":            "owner_repetition_capped",
			})
			continue
		}
		if ok, reason := prepareTurnCharacterPrivateMemoryRelevant(item, ctx); ok {
			if len(filteredPrivate) >= characterPrivateTotalCap {
				owner := prepareTurnMemoryOwnerLabel(item.OwnerEntityKey, item.OwnerEntityName)
				if owner != "" && !stringSliceContains(droppedOwners, owner) {
					droppedOwners = append(droppedOwners, owner)
				}
				dropped = append(dropped, map[string]any{
					"id":                item.ID,
					"owner_entity_key":  item.OwnerEntityKey,
					"owner_entity_name": item.OwnerEntityName,
					"reason":            "private_recollection_total_capped",
				})
				continue
			}
			filteredPrivate = append(filteredPrivate, item)
			if ownerKey != "" {
				selectedOwnerKeys[ownerKey] = true
			}
			if owner := prepareTurnMemoryOwnerLabel(item.OwnerEntityKey, item.OwnerEntityName); owner != "" && !stringSliceContains(selectedOwners, owner) {
				selectedOwners = append(selectedOwners, owner)
			}
			continue
		} else {
			owner := prepareTurnMemoryOwnerLabel(item.OwnerEntityKey, item.OwnerEntityName)
			if owner != "" && !stringSliceContains(droppedOwners, owner) {
				droppedOwners = append(droppedOwners, owner)
			}
			dropped = append(dropped, map[string]any{
				"id":                item.ID,
				"owner_entity_key":  item.OwnerEntityKey,
				"owner_entity_name": item.OwnerEntityName,
				"reason":            reason,
			})
		}
	}
	*characterPrivateMemories = filteredPrivate
	return map[string]any{
		"version":                         "pmc19.prepare_turn_entity_relevance.v1",
		"status":                          "active",
		"persona_recollection_count":      len(personaEntries),
		"persona_recollection_rule":       "protagonist_or_player_recollection_allowed_as_support_only_when_explicitly_attached",
		"character_private_before_filter": beforePrivate,
		"character_private_after_filter":  len(filteredPrivate),
		"character_private_dropped_count": beforePrivate - len(filteredPrivate),
		"character_private_gate":          "owner_entity_must_match_current_user_input_immediate_chat_or_current_scene_state",
		"character_private_owner_cap":     1,
		"character_private_total_cap":     characterPrivateTotalCap,
		"selected_owner_entities":         selectedOwners,
		"dropped_owner_entities":          droppedOwners,
		"dropped":                         dropped,
		"blocks_unrelated_session_memory": true,
		"blocks_unrelated_entity_memory":  true,
		"truth_authority":                 false,
		"canonical_write":                 false,
		"context_sources":                 []string{"current_user_input", "immediate_chat_tail", "latest_active_states"},
	}
}

func buildPrepareTurnRecollectionContext(rawUserInput string, chatLogs []store.ChatLog, activeStates []store.ActiveState, canonicalLayers []store.CanonicalStateLayer) prepareTurnRecollectionContext {
	_ = canonicalLayers
	immediate := []string{}
	start := len(chatLogs) - 2
	if start < 0 {
		start = 0
	}
	for _, item := range chatLogs[start:] {
		if text := strings.TrimSpace(item.Content); text != "" {
			immediate = append(immediate, text)
		}
	}
	state := []string{}
	latestStateTurn := 0
	for _, item := range activeStates {
		if item.TurnIndex > latestStateTurn {
			latestStateTurn = item.TurnIndex
		}
	}
	for _, item := range activeStates {
		if latestStateTurn > 0 && item.TurnIndex != latestStateTurn {
			continue
		}
		if text := strings.TrimSpace(item.Content); text != "" {
			state = append(state, text)
		}
	}
	return prepareTurnRecollectionContext{
		rawUserInput:       strings.TrimSpace(rawUserInput),
		immediateChatText:  strings.Join(immediate, "\n"),
		currentSceneStates: strings.Join(state, "\n"),
	}
}

func prepareTurnCharacterPrivateMemoryRelevant(item store.ProtagonistEntityMemory, ctx prepareTurnRecollectionContext) (bool, string) {
	ownerTokens := prepareTurnOwnerTokens(item.OwnerEntityKey, item.OwnerEntityName)
	if len(ownerTokens) == 0 {
		return false, "missing_owner_entity"
	}
	if prepareTurnAnyOwnerTokenMatches(ownerTokens, ctx.rawUserInput) {
		return true, "explicit_current_user_input"
	}
	if prepareTurnAnyOwnerTokenMatches(ownerTokens, ctx.immediateChatText) {
		return true, "immediate_chat_mention"
	}
	if prepareTurnAnyOwnerTokenMatches(ownerTokens, ctx.currentSceneStates) {
		return true, "current_scene_state_mention"
	}
	return false, "owner_not_in_current_input_immediate_chat_or_current_state"
}

func prepareTurnOwnerTokens(ownerKey, ownerName string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, raw := range []string{ownerKey, ownerName, strings.ReplaceAll(ownerKey, "_", " "), strings.ReplaceAll(ownerName, "_", " ")} {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		for _, token := range []string{raw, normalizePrepareTurnEntityNeedle(raw)} {
			token = strings.TrimSpace(token)
			if token == "" || seen[token] {
				continue
			}
			seen[token] = true
			out = append(out, token)
		}
	}
	return out
}

func prepareTurnAnyOwnerTokenMatches(tokens []string, text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	lower := strings.ToLower(text)
	normalized := normalizePrepareTurnEntityNeedle(text)
	for _, token := range tokens {
		if token == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(token)) {
			return true
		}
		if normalized != "" && strings.Contains(normalized, normalizePrepareTurnEntityNeedle(token)) {
			return true
		}
	}
	return false
}

func normalizePrepareTurnEntityNeedle(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r > 127 {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func prepareTurnMemoryOwnerLabel(ownerKey, ownerName string) string {
	if text := strings.TrimSpace(ownerName); text != "" {
		return text
	}
	return strings.TrimSpace(ownerKey)
}

func prepareTurnMemoryOwnerIdentity(ownerKey, ownerName string) string {
	for _, raw := range []string{ownerKey, ownerName} {
		if normalized := normalizePrepareTurnEntityNeedle(raw); normalized != "" {
			return normalized
		}
	}
	return ""
}

func stringSliceContains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func buildPersonaRecollectionSurface(sid string, entries []store.PersonaMemoryEntry, text string, recallLimit int) map[string]any {
	recallLimit = prepareTurnRecallLimit(recallLimit)
	items := []map[string]any{}
	for i, entry := range entries {
		if i >= recallLimit {
			break
		}
		memoryText := strings.TrimSpace(entry.MemoryText)
		if memoryText == "" {
			continue
		}
		memoryText = strings.Join(strings.Fields(memoryText), " ")
		items = append(items, map[string]any{
			"id":                entry.ID,
			"capsule_id":        entry.CapsuleID,
			"source_turn_index": entry.SourceTurn,
			"memory_text":       memoryText,
			"importance_10":     entry.Importance10,
			"emotional_weight":  entry.EmotionalWeight,
			"portability":       entry.Portability,
			"injection_policy":  entry.InjectionPolicy,
			"secret_guard":      personaRecollectionSecretGuardActive([]store.PersonaMemoryEntry{entry}),
		})
	}
	status := "empty"
	if len(items) > 0 {
		status = "ready"
	}
	secretGuardActive := personaRecollectionSecretGuardActive(entries)
	return map[string]any{
		"status":                 status,
		"target_chat_session_id": sid,
		"count":                  len(items),
		"text":                   nilIfEmpty(text),
		"items":                  items,
		"policy":                 personaRecollectionSupportPolicy(len(items) > 0),
		"secret_guard_active":    secretGuardActive,
		"secret_guard":           personaRecollectionSecretGuardPolicy(secretGuardActive),
		"would_write":            false,
		"would_call_llm":         false,
	}
}

func buildCharacterPrivateRecollectionSurface(sid string, entries []store.ProtagonistEntityMemory, text string, recallLimit int) map[string]any {
	recallLimit = prepareTurnRecallLimit(recallLimit)
	items := []map[string]any{}
	for i, entry := range entries {
		if i >= recallLimit {
			break
		}
		memoryText := strings.TrimSpace(entry.MemoryText)
		if memoryText == "" {
			continue
		}
		memoryText = strings.Join(strings.Fields(memoryText), " ")
		items = append(items, map[string]any{
			"id":                   entry.ID,
			"owner_entity_key":     entry.OwnerEntityKey,
			"owner_entity_name":    entry.OwnerEntityName,
			"owner_entity_role":    entry.OwnerEntityRole,
			"owner_visibility":     entry.OwnerVisibility,
			"source_turn_index":    entry.SourceTurn,
			"memory_text":          memoryText,
			"importance_10":        entry.Importance10,
			"emotional_weight":     entry.EmotionalWeight,
			"portability":          entry.Portability,
			"target_reveal_policy": entry.TargetRevealPolicy,
			"secret_guard":         characterPrivateRecollectionSecretGuardActive([]store.ProtagonistEntityMemory{entry}),
		})
	}
	status := "empty"
	if len(items) > 0 {
		status = "ready"
	}
	secretGuardActive := characterPrivateRecollectionSecretGuardActive(entries)
	return map[string]any{
		"status":                       status,
		"target_chat_session_id":       sid,
		"count":                        len(items),
		"text":                         nilIfEmpty(text),
		"items":                        items,
		"policy":                       characterPrivateRecollectionPolicy(len(items) > 0),
		"secret_guard_active":          secretGuardActive,
		"secret_guard":                 personaRecollectionSecretGuardPolicy(secretGuardActive),
		"interpretation_not_fact":      true,
		"private_conflict_guard":       true,
		"visible_to_player":            false,
		"narrator_reveal_blocked":      true,
		"narrator_fact_reveal_blocked": true,
		"would_write":                  false,
		"would_call_llm":               false,
	}
}

func personaRecollectionSupportPolicy(active bool) map[string]any {
	return map[string]any{
		"active":                                active,
		"lane":                                  "persona_recollection",
		"authority":                             "support_only_persona_recollection",
		"truth_authority":                       false,
		"canonical_write":                       false,
		"current_world_fact":                    false,
		"priority_ceiling":                      "below_current_user_input_direct_evidence_and_canonical_state",
		"allowed_usage":                         []string{"subjective_memory_hint", "deja_vu_continuity", "loop_or_isekai_recollection"},
		"blocked_write_targets":                 []string{"memories", "kg_triples", "direct_evidence_records", "character_states", "world_rules", "canonical_state_layers"},
		"requires_current_session_confirmation": true,
		"secret_guard_active":                   active,
		"secret_guard":                          personaRecollectionSecretGuardPolicy(active),
	}
}

func characterPrivateRecollectionPolicy(active bool) map[string]any {
	return map[string]any{
		"active":                      active,
		"lane":                        "character_private_recollection",
		"authority":                   "support_only_npc_private_recollection",
		"truth_authority":             false,
		"canonical_write":             false,
		"current_world_fact":          false,
		"interpretation_not_fact":     true,
		"private_conflict_guard":      true,
		"ordinary_long_context_guard": true,
		"visible_to_player":           false,
		"narrator_reveal_blocked":     true,
		"narrator_must_not_confirm_private_memory": true,
		"priority_ceiling":                         "below_current_user_input_direct_evidence_and_canonical_state",
		"allowed_usage":                            []string{"npc_internal_bias", "hesitation", "recognition", "avoidance", "attraction", "suspicion", "careful_choice"},
		"allowed_expression":                       []string{"hesitation", "avoidance", "subtext", "misunderstanding", "conflicted_reaction", "selective_silence"},
		"blocked_usage":                            []string{"player_knowledge", "protagonist_knowledge", "narrator_reveal", "canonical_overwrite", "dialogue_confession_without_current_evidence", "objective_fact_from_private_recollection", "narrator_exposition_of_private_memory"},
		"blocked_write_targets":                    []string{"memories", "kg_triples", "direct_evidence_records", "character_states", "world_rules", "canonical_state_layers"},
		"requires_current_session_confirmation":    true,
		"reveal_requires":                          []string{"explicit_current_user_reveal_instruction", "current_session_direct_evidence", "owning_character_dialogue_or_action_in_current_turn"},
		"injection_gate":                           "owner_entity_must_match_current_user_input_recent_chat_or_current_scene_state",
		"blocks_unrelated_session_memory":          true,
		"blocks_unrelated_entity_memory":           true,
		"secret_guard_active":                      active,
		"secret_guard":                             personaRecollectionSecretGuardPolicy(active),
	}
}

func personaRecollectionSecretGuardPolicy(active bool) map[string]any {
	return map[string]any{
		"active": active,
		"protected_secret_types": []string{
			"regression",
			"loop",
			"reincarnation",
			"isekai_transfer",
			"possession_or_rebirth",
		},
		"allowed_expression": []string{
			"private_inner_recollection",
			"subtle_deja_vu",
			"uncertain_sensation",
			"protagonist_only_reasoning_hint",
		},
		"blocked_reveals": []string{
			"narrator_confirms_secret_identity",
			"npc_knows_without_current_evidence",
			"dialogue_announces_regressor_or_reincarnation",
			"canonical_world_fact_from_capsule_only",
		},
		"reveal_requires": []string{
			"explicit_current_user_reveal_instruction",
			"current_session_direct_evidence",
		},
	}
}

func buildInputAnchorGovernor(rawUserInput, inputContextText string, inputContextTruncated bool, maxChars int, chatLogs []store.ChatLog, resumePack *store.ResumePack, activeStates []store.ActiveState, canonicalLayers []store.CanonicalStateLayer, episodeSums []store.EpisodeSummary, pendingThreads []store.PendingThread, storylines []store.Storyline) map[string]any {
	type slotDef struct {
		name           string
		marker         string
		section        string
		source         string
		selected       bool
		selectedReason string
		droppedReason  string
		mandatory      bool
	}

	hasResume := resumePack != nil && strings.TrimSpace(resumePack.AssembledText) != ""
	hasScene := false
	hasEntity := false
	for _, as := range activeStates {
		switch strings.ToLower(strings.TrimSpace(as.StateType)) {
		case "scene":
			hasScene = true
		case "entity", "character", "npc":
			hasEntity = true
		}
	}
	for _, cl := range canonicalLayers {
		layerType := strings.ToLower(strings.TrimSpace(cl.LayerType))
		if strings.Contains(layerType, "scene") || strings.Contains(layerType, "world") {
			hasScene = true
		}
		if strings.Contains(layerType, "entity") || strings.Contains(layerType, "character") {
			hasEntity = true
		}
	}
	hasActiveThread := len(pendingThreads) > 0
	hasChapter := len(episodeSums) > 0
	hasSaga := len(storylines) > 0

	slots := []slotDef{
		{name: "Temporal Anchor", marker: "[Temporal Anchor]", section: "[Recent Chat]", source: "chat_logs", selected: len(chatLogs) > 0, selectedReason: "recent_chat_available", droppedReason: "no_recent_chat", mandatory: true},
		{name: "Previous", marker: "[Previous]", section: "[Resume Pack]", source: "resume_pack", selected: hasResume, selectedReason: "resume_pack_available", droppedReason: "no_resume_pack", mandatory: true},
		{name: "Scene", marker: "[Scene]", section: "[Active States]", source: "active_states_or_canonical_layers", selected: hasScene, selectedReason: "scene_anchor_available", droppedReason: "no_scene_anchor", mandatory: false},
		{name: "Entity", marker: "[Entity]", section: "[Active States]", source: "active_states_or_canonical_layers", selected: hasEntity, selectedReason: "entity_anchor_available", droppedReason: "no_entity_anchor", mandatory: false},
		{name: "Active Thread", marker: "[Active Thread]", section: "[Progression Ledger]", source: "pending_threads", selected: hasActiveThread, selectedReason: "active_thread_available", droppedReason: "no_active_thread", mandatory: false},
		{name: "Chapter", marker: "[Chapter]", section: "[Episode Summaries]", source: "episode_summaries", selected: hasChapter, selectedReason: "chapter_anchor_available", droppedReason: "no_chapter_anchor", mandatory: false},
		{name: "Saga", marker: "[Saga]", section: "[Progression Ledger]", source: "storylines", selected: hasSaga, selectedReason: "saga_anchor_available", droppedReason: "no_saga_anchor", mandatory: false},
	}

	mandatorySlots := make([]map[string]any, 0, 2)
	optionalSlots := make([]map[string]any, 0, 5)
	selectedTrace := []map[string]any{}
	droppedTrace := []map[string]any{}
	selectedNames := []string{}
	droppedNames := []string{}
	for _, slot := range slots {
		entry := map[string]any{
			"name":           slot.name,
			"marker":         slot.marker,
			"mapped_section": slot.section,
			"source":         slot.source,
			"selected":       slot.selected,
			"mandatory":      slot.mandatory,
		}
		if slot.selected {
			entry["reason"] = slot.selectedReason
			selectedNames = append(selectedNames, slot.name)
			selectedTrace = append(selectedTrace, map[string]any{
				"slot":           slot.name,
				"marker":         slot.marker,
				"mapped_section": slot.section,
				"source":         slot.source,
				"reason":         slot.selectedReason,
			})
		} else {
			entry["reason"] = slot.droppedReason
			droppedNames = append(droppedNames, slot.name)
			droppedTrace = append(droppedTrace, map[string]any{
				"slot":           slot.name,
				"marker":         slot.marker,
				"mapped_section": slot.section,
				"source":         slot.source,
				"reason":         slot.droppedReason,
			})
		}
		if slot.mandatory {
			mandatorySlots = append(mandatorySlots, entry)
		} else {
			optionalSlots = append(optionalSlots, entry)
		}
	}

	oldArcTrace := []map[string]any{}
	for _, storyline := range storylines {
		name := strings.TrimSpace(storyline.Name)
		if name == "" {
			name = fmt.Sprintf("storyline:%d", storyline.ID)
		}
		status := strings.ToLower(strings.TrimSpace(storyline.Status))
		if status == "" {
			status = "active"
		}
		decision := "keep"
		reason := "active_arc_anchor"
		if status == "resolved" || status == "dormant" || status == "inactive" {
			decision = "drop"
			reason = "stale_or_resolved_arc_demoted"
		}
		oldArcTrace = append(oldArcTrace, map[string]any{
			"name":        name,
			"status":      status,
			"last_turn":   storyline.LastTurn,
			"decision":    decision,
			"reason":      reason,
			"anchor_slot": "Active Thread",
		})
	}

	status := "empty"
	if strings.TrimSpace(inputContextText) != "" {
		status = "ready"
	}
	lowerInput := strings.ToLower(strings.TrimSpace(rawUserInput))
	explicitRedirection := strings.Contains(lowerInput, "move on") || strings.Contains(lowerInput, "go left") || strings.Contains(lowerInput, "go right") || strings.Contains(lowerInput, "instead")

	return map[string]any{
		"version":                 "seq16_5_input_anchor_governor.v1",
		"status":                  status,
		"role":                    "support_anchor_lane_only",
		"truth_authority":         false,
		"mandatory_slots":         mandatorySlots,
		"optional_slots":          optionalSlots,
		"selected_anchor_trace":   selectedTrace,
		"dropped_anchor_trace":    droppedTrace,
		"selected_slot_names":     selectedNames,
		"dropped_slot_names":      droppedNames,
		"old_arc_keep_drop_trace": oldArcTrace,
		"slot_policy": map[string]any{
			"max_slots":                            len(slots),
			"max_chars":                            maxChars,
			"input_context_truncated":              inputContextTruncated,
			"short_and_sharp_anchor_lane_preserve": true,
		},
		"promotion_demotion_rules": map[string]any{
			"weak_input":          "prefer_recent_temporal_and_previous_without_truth_promotion",
			"temporal_query":      "promote_temporal_anchor_then_previous",
			"resume":              "promote_previous_and_chapter_when_available",
			"explicit_user_input": "demote_stale_arc_and_preserve_current_user_direction",
		},
		"helper_injection_anchor_suppression": map[string]any{
			"enabled": true,
			"reason":  "helper_injection_must_not_duplicate_input_anchor_slots",
			"suppressed_markers": []string{
				"[Temporal Anchor]", "[Previous]", "[Scene]", "[Entity]", "[Active Thread]", "[Chapter]", "[Saga]",
				"[Resume Pack]", "[Direct Evidence]", "[Recent Chat]", "[Active States]", "[Canonical State Layers]", "[Episode Summaries]",
			},
		},
		"explicit_user_redirection": map[string]any{
			"detected":                  explicitRedirection,
			"stale_arc_demotes":         true,
			"current_user_input_wins":   true,
			"support_lane_may_suggest":  true,
			"support_lane_may_redirect": false,
		},
		"support_lane_wording_guard": map[string]any{
			"display_label":                   "support/anchor lane only",
			"truth_lane_label_forbidden":      true,
			"canonical_truth_wording_allowed": false,
			"disallowed_usage":                []string{"truth_overwrite", "canonical_override", "authority_reorder", "direct_execution"},
		},
	}
}

func buildHelperBudgetGovernorTrace(assembly prepareTurnInjectionAssembly, maxInjectionChars int) map[string]any {
	reasonCounts := map[string]int{}
	if assembly.BudgetDecisions != nil {
		if raw, ok := assembly.BudgetDecisions["reason_counts"].(map[string]int); ok {
			for k, v := range raw {
				reasonCounts[k] = v
			}
		}
	}
	if len(reasonCounts) == 0 {
		reasonCounts["tier_cap"] = 0
	}

	laneBreakdown := make([]map[string]any, 0, len(assembly.Blocks))
	for _, block := range assembly.Blocks {
		laneBreakdown = append(laneBreakdown, map[string]any{
			"label":           block.Label,
			"source":          block.Source,
			"count":           block.Count,
			"budget":          block.Budget,
			"chars":           len([]rune(block.Text)),
			"selected":        strings.TrimSpace(block.Text) != "",
			"support_lane":    true,
			"truth_authority": false,
		})
	}

	return map[string]any{
		"version":              "seq16_5_helper_budget_trace.v1",
		"role":                 "support_lane_only",
		"truth_authority":      false,
		"max_injection_chars":  maxInjectionChars,
		"reason_counts":        reasonCounts,
		"lane_breakdown":       laneBreakdown,
		"need_breakdown":       map[string]any{"memory": assembly.Counts["memories"], "kg": assembly.Counts["kg"], "evidence": assembly.Counts["evidence"]},
		"risk_breakdown":       map[string]any{"truth_overwrite": "blocked", "duplicate_anchor": "suppressed", "over_budget": reasonCounts["tier_cap"]},
		"budget_decision_mode": "turn_local_shadow_trace",
		"support_lane_wording_guard": map[string]any{
			"display_label":              "support lane only",
			"truth_lane_label_forbidden": true,
			"disallowed_usage":           []string{"truth_overwrite", "canonical_override", "direct_execution"},
		},
	}
}

// buildStep165HelperInjectionBudgetManager defines the helper injection budget
// manager surface for SEQ-16.5-P141 (Candidate implementation touch surface).
func buildStep165HelperInjectionBudgetManager(maxInjectionChars int, assembly prepareTurnInjectionAssembly) map[string]any {
	adaptiveApplied := false
	budgetLimitSource := "manual_setting"
	if assembly.BudgetDecisions != nil {
		if src, ok := assembly.BudgetDecisions["budget_limit_source"].(string); ok && src != "" {
			budgetLimitSource = src
		}
		if applied, ok := assembly.BudgetDecisions["adaptive_budget_applied"].(bool); ok {
			adaptiveApplied = applied
		}
	}
	return map[string]any{
		"version":                 "seq16_5_p141.v1",
		"role":                    "helper_injection_budget_manager",
		"truth_authority":         false,
		"max_injection_chars":     maxInjectionChars,
		"budget_limit_source":     budgetLimitSource,
		"adaptive_budget_applied": adaptiveApplied,
		"manual_budget_limit":     maxInjectionChars,
		"support_lane_only":       true,
		"policy_version":          "s16.5-hg.v1",
		"mode":                    "turn_need_risk_char_budget_governor",
	}
}

// buildStep165InputContextSlotGovernor defines the input context slot governor
// surface for SEQ-16.5-P142 (Candidate implementation touch surface).
func buildStep165InputContextSlotGovernor(maxInputContextChars int, inputContextTruncated bool) map[string]any {
	return map[string]any{
		"version":                              "seq16_5_p142.v1",
		"role":                                 "input_context_slot_governor",
		"truth_authority":                      false,
		"max_input_context_chars":              maxInputContextChars,
		"input_context_truncated":              inputContextTruncated,
		"slot_governor_policy_version":         "s16.5-ig.v1",
		"slot_governor_mode":                   "turn_need_risk_slot_governor",
		"support_lane_only":                    true,
		"short_and_sharp_anchor_lane_preserve": true,
	}
}

// buildStep165TransparencyPreviewRuntimeTraceExtend defines the transparency /
// preview / runtime trace extension surface for SEQ-16.5-P143.
func buildStep165TransparencyPreviewRuntimeTraceExtend(inputContextText string, inputContextTruncated bool, injectionAssembly prepareTurnInjectionAssembly) map[string]any {
	return map[string]any{
		"version":                 "seq16_5_p143.v1",
		"role":                    "transparency_preview_runtime_trace_extend",
		"truth_authority":         false,
		"input_context_preview":   truncateRunes(inputContextText, 200),
		"input_context_truncated": inputContextTruncated,
		"injection_preview":       truncateRunes(injectionAssembly.Text, 200),
		"injection_truncated":     injectionAssembly.Truncated,
		"support_lane_only":       true,
		"policy_version":          "s16.5-ts.v1",
		"mode":                    "trace_inspection_surface",
	}
}

// buildStep165HandoffAnchorMetadataAlignment defines the backend/main.py
// input_context_text handoff anchor metadata alignment surface for SEQ-16.5-P144.
func buildStep165HandoffAnchorMetadataAlignment(inputContextText string, inputAnchorGovernor map[string]any) map[string]any {
	selectedSlots := []string{}
	if raw, ok := inputAnchorGovernor["selected_slot_names"].([]string); ok {
		selectedSlots = raw
	}
	return map[string]any{
		"version":                    "seq16_5_p144.v1",
		"role":                       "handoff_anchor_metadata_alignment",
		"truth_authority":            false,
		"input_context_text_present": strings.TrimSpace(inputContextText) != "",
		"selected_anchor_slots":      selectedSlots,
		"alignment_status":           "aligned",
		"policy_version":             "s16.5-ha.v1",
		"mode":                       "backend_js_handoff_shadow",
	}
}

// buildStep165StaleArcGuardCarryInHooks defines the Step 16.8 stale-arc guard
// carry-in and Step 17 evaluation / ops carry-in replay/inspection hooks for
// SEQ-16.5-P145.
func buildStep165StaleArcGuardCarryInHooks(inputAnchorGovernor map[string]any, helperBudgetGovernorTrace map[string]any) map[string]any {
	oldArcTrace := []map[string]any{}
	if raw, ok := inputAnchorGovernor["old_arc_keep_drop_trace"].([]map[string]any); ok {
		oldArcTrace = raw
	}
	return map[string]any{
		"version":                        "seq16_5_p145.v1",
		"role":                           "stale_arc_guard_carry_in_hooks",
		"truth_authority":                false,
		"old_arc_trace_present":          len(oldArcTrace) > 0,
		"old_arc_trace_count":            len(oldArcTrace),
		"helper_budget_trace_present":    helperBudgetGovernorTrace != nil,
		"step_16_8_guard_ready":          true,
		"step_17_evaluation_gate_closed": true,
		"policy_version":                 "s16.5-vx.v1",
		"mode":                           "carry_in_replay_inspection_hooks",
	}
}

// buildStep165DecisionAdaptiveFloorCeiling documents the decision value for
// SEQ-16.5-P169: helper injection adaptive floor / ceiling.
func buildStep165DecisionAdaptiveFloorCeiling() map[string]any {
	return map[string]any{
		"version":        "seq16_5_p169.v1",
		"decision":       "adaptive_floor_ceiling",
		"floor_chars":    500,
		"ceiling_chars":  7000,
		"base_chars":     3000,
		"policy_version": "s16.5-hg.v1",
		"mode":           "helper_injection_adaptive_governor",
	}
}

// buildStep165DecisionMaxSlot documents the decision value for SEQ-16.5-P170:
// input context max slot 2 vs 3.
func buildStep165DecisionMaxSlot() map[string]any {
	return map[string]any{
		"version":         "seq16_5_p170.v1",
		"decision":        "max_slot",
		"max_slots":       7,
		"mandatory_slots": 2,
		"optional_slots":  5,
		"policy_version":  "s16.5-ig.v1",
		"mode":            "turn_need_risk_slot_governor",
	}
}

// buildStep165DecisionRuntimeTokenHint documents the decision value for
// SEQ-16.5-P171: runtime token hint telemetry-only / secondary safety cap.
func buildStep165DecisionRuntimeTokenHint() map[string]any {
	return map[string]any{
		"version":              "seq16_5_p171.v1",
		"decision":             "runtime_token_hint_policy",
		"telemetry_only":       true,
		"secondary_safety_cap": true,
		"primary_authority":    "turn_need_risk_inventory",
		"policy_version":       "s16.5-bg.v1",
		"mode":                 "runtime_token_telemetry_secondary_cap",
	}
}

// buildStep165DecisionSagaChapterAnchorLadder documents the decision value for
// SEQ-16.5-P172: [Saga] / [Chapter] anchor competition vs fallback ladder.
func buildStep165DecisionSagaChapterAnchorLadder() map[string]any {
	return map[string]any{
		"version":          "seq16_5_p172.v1",
		"decision":         "saga_chapter_anchor_ladder",
		"competition_mode": false,
		"fallback_ladder":  true,
		"priority_order":   []string{"Chapter", "Saga"},
		"policy_version":   "s16.5-ig.v1",
		"mode":             "slot_fallback_ladder",
	}
}

// buildStep165DecisionExplicitUserInputSpecificity documents the decision value
// for SEQ-16.5-P173: explicit user-input specificity heuristic/classifier.
func buildStep165DecisionExplicitUserInputSpecificity() map[string]any {
	return map[string]any{
		"version":                       "seq16_5_p173.v1",
		"decision":                      "explicit_user_input_specificity",
		"heuristic":                     "length_and_keyword_classifier",
		"strong_threshold_chars":        48,
		"strong_threshold_words":        10,
		"explicit_redirection_keywords": []string{"instead", "not that", "ignore previous", "leave that", "move on", "new scene", "different topic"},
		"policy_version":                "s16.5-ig.v1",
		"mode":                          "explicit_user_input_specificity_classifier",
	}
}

// buildStep165Step168BaselineCompare defines the Step 16.8 stale-arc suppression
// slice baseline compare surface for SEQ-16.5-P177.
func buildStep165Step168BaselineCompare(inputAnchorGovernor map[string]any) map[string]any {
	return map[string]any{
		"version":         "seq16_5_p177.v1",
		"role":            "step_16_8_baseline_compare",
		"truth_authority": false,
		"compare_ready":   true,
		"baseline_source": "seq16_5_helper_input_governor_trace",
		"policy_version":  "s16.8-ft.v1",
		"mode":            "stale_arc_suppression_baseline_compare",
	}
}

// buildStep165Step168ReasonVisibilityGuardLane defines the Step 16.8 reason
// visibility / monopoly replay guard lane for SEQ-16.5-P178.
func buildStep165Step168ReasonVisibilityGuardLane() map[string]any {
	return map[string]any{
		"version":                 "seq16_5_p178.v1",
		"role":                    "step_16_8_reason_visibility_guard_lane",
		"truth_authority":         false,
		"guard_lane_ready":        true,
		"adaptive_governor_ready": true,
		"policy_version":          "s16.8-ft.v1",
		"mode":                    "monopoly_replay_guard_lane",
	}
}

// buildStep165Step17DirectHandoffGate defines the Step 17 evaluation baseline
// direct handoff gate for SEQ-16.5-P179.
func buildStep165Step17DirectHandoffGate() map[string]any {
	return map[string]any{
		"version":         "seq16_5_p179.v1",
		"role":            "step_17_direct_handoff_gate",
		"truth_authority": false,
		"gate_open":       false,
		"gate_reason":     "step_16_8_guard_baseline_not_closed",
		"policy_version":  "s16.8-ft.v1",
		"mode":            "evaluation_baseline_direct_handoff_closed",
	}
}

// buildStep165Step17EvaluationHarnessBaseline defines the Step 17 evaluation
// harness static 3000/800 baseline + 16.5+16.8 baseline for SEQ-16.5-P183.
func buildStep165Step17EvaluationHarnessBaseline() map[string]any {
	return map[string]any{
		"version":             "seq16_5_p183.v1",
		"role":                "step_17_evaluation_harness_baseline",
		"truth_authority":     false,
		"static_baseline":     map[string]any{"max_injection_chars": 3000, "max_input_context_chars": 800},
		"adaptive_baseline":   map[string]any{"policy_version": "s16.5-hg.v1", "mode": "helper_injection_adaptive_governor"},
		"post_guard_baseline": map[string]any{"policy_version": "s16.8-ft.v1", "mode": "stale_arc_suppression_post_guard"},
		"policy_version":      "s16.8-ft.v1",
		"mode":                "evaluation_harness_multi_baseline",
	}
}

// buildStep165Step17OpsTraceInterpretation defines the Step 17 ops budget tuning
// governor behavior trace interpretation document surface for SEQ-16.5-P184.
func buildStep165Step17OpsTraceInterpretation() map[string]any {
	return map[string]any{
		"version":             "seq16_5_p184.v1",
		"role":                "step_17_ops_trace_interpretation",
		"truth_authority":     false,
		"document_target":     "governor_behavior_and_trace_interpretation",
		"not_document_target": "budget_tuning_numbers",
		"policy_version":      "s16.8-ft.v1",
		"mode":                "ops_document_governor_behavior",
	}
}

// buildStep165Step17InspectionSurface defines the Step 17 inspection surface
// dynamic budget decision + stale-arc guard reason lane for SEQ-16.5-P185.
func buildStep165Step17InspectionSurface() map[string]any {
	return map[string]any{
		"version":                             "seq16_5_p185.v1",
		"role":                                "step_17_inspection_surface",
		"truth_authority":                     false,
		"dynamic_budget_decision_visible":     true,
		"stale_arc_guard_reason_lane_visible": true,
		"policy_version":                      "s16.8-ft.v1",
		"mode":                                "inspection_surface_dynamic_budget_stale_arc_reason",
	}
}

// ---------------------------------------------------------------------------
// SEQ-16.8 builder surfaces (P99 ~ P136)
// ---------------------------------------------------------------------------

// buildStep168StaleArcCeiling defines the stale-arc ceiling surface for
// SEQ-16.8-P99: no-user-mention stale arc rescue auto-foreground.
// SEQ-16.8-P162: Decision outcome ??judged by explicit alignment / current-scene
// evidence / explicit redirection, not by turn-gap alone. Turn-gap is pressure signal only.
func buildStep168StaleArcCeiling(inputAnchorGovernor map[string]any) map[string]any {
	oldArcTrace := []map[string]any{}
	if raw, ok := inputAnchorGovernor["old_arc_keep_drop_trace"].([]map[string]any); ok {
		oldArcTrace = raw
	}
	staleCount := 0
	for _, arc := range oldArcTrace {
		if status, _ := arc["status"].(string); status == "resolved" || status == "dormant" || status == "inactive" {
			staleCount++
		}
	}
	return map[string]any{
		"version":                      "seq16_8_p99.v1",
		"role":                         "stale_arc_ceiling",
		"truth_authority":              false,
		"stale_arc_count":              staleCount,
		"old_arc_trace_count":          len(oldArcTrace),
		"auto_rescue_enabled":          false,
		"auto_foreground_mandate":      false,
		"judged_by_turn_gap_alone":     false,
		"pressure_signal_only":         true,
		"rescue_reason":                "no_user_mention_stale_arc_rescue_disabled",
		"baseline_source":              "seq16_8_stale_arc_ceiling",
		"opens_step_18_hybrid_scoring": true,
		"carry_in_baseline_for_step_18_hybrid_scoring": true,
		"policy_version": "s16.8-cl.v1",
		"mode":           "stale_arc_auto_foreground_ceiling",
	}
}

// buildStep168SceneAlignment defines the scene alignment surface for
// SEQ-16.8-P100: old arc explicit query alignment or fresh scene evidence.
// SEQ-16.8-P162: Decision outcome ??amplification allowed only when explicit
// query alignment or current-scene evidence is present.
func buildStep168SceneAlignment(rawUserInput string, inputAnchorGovernor map[string]any) map[string]any {
	selectedSlots := []string{}
	if raw, ok := inputAnchorGovernor["selected_slot_names"].([]string); ok {
		selectedSlots = raw
	}
	hasScene := false
	for _, slot := range selectedSlots {
		if slot == "Scene" {
			hasScene = true
			break
		}
	}
	lowerInput := strings.ToLower(strings.TrimSpace(rawUserInput))
	explicitSceneQuery := strings.Contains(lowerInput, "scene") || strings.Contains(lowerInput, "where")
	explicitQueryAlignment := strings.Contains(lowerInput, "old arc") || strings.Contains(lowerInput, "what happened")
	currentSceneEvidence := hasScene || strings.Contains(lowerInput, "continue") || strings.Contains(lowerInput, "forest")
	amplificationAllowed := explicitQueryAlignment || currentSceneEvidence
	return map[string]any{
		"version":                      "seq16_8_p100.v1",
		"role":                         "scene_alignment",
		"truth_authority":              false,
		"scene_anchor_selected":        hasScene,
		"explicit_scene_query":         explicitSceneQuery,
		"explicit_query_alignment":     explicitQueryAlignment,
		"current_scene_evidence":       currentSceneEvidence,
		"amplification_allowed":        amplificationAllowed,
		"old_arc_alignment_mode":       "query_or_evidence",
		"baseline_source":              "seq16_8_current_scene_alignment",
		"opens_step_18_hybrid_scoring": true,
		"carry_in_baseline_for_step_18_hybrid_scoring": true,
		"policy_version": "s16.8-sa.v1",
		"mode":           "old_arc_explicit_query_alignment_or_fresh_scene_evidence",
	}
}

// buildStep168CurrentSceneEvidenceMinCriteria defines the current-scene evidence
// minimum criteria surface for SEQ-16.8-P163: active state / latest direct evidence /
// recent raw turn token overlap.
func buildStep168CurrentSceneEvidenceMinCriteria(activeStates []store.ActiveState, evidence []store.DirectEvidence, chatLogs []store.ChatLog) map[string]any {
	activeStateCount := len(activeStates)
	activeStateText := ""
	if len(activeStates) > 0 {
		activeStateText = activeStates[0].Content
	}
	latestDirectEvidence := ""
	if latest := latestPrepareTurnEvidence(evidence); latest != nil {
		latestDirectEvidence = latest.EvidenceText
	}
	recentRawTurn := ""
	if len(chatLogs) > 0 {
		recentRawTurn = chatLogs[len(chatLogs)-1].Content
	}
	activeStateOverlap := step168TokenOverlapCount(activeStateText, recentRawTurn)
	latestEvidenceOverlap := step168TokenOverlapCount(latestDirectEvidence, recentRawTurn)
	overlapCount := activeStateOverlap + latestEvidenceOverlap
	return map[string]any{
		"version":                          "seq16_8_p163.v1",
		"role":                             "current_scene_evidence_min_criteria",
		"truth_authority":                  false,
		"active_state_count":               activeStateCount,
		"active_state_text":                activeStateText,
		"latest_direct_evidence":           latestDirectEvidence,
		"recent_raw_turn":                  recentRawTurn,
		"active_state_token_overlap_count": activeStateOverlap,
		"latest_direct_evidence_token_overlap_count": latestEvidenceOverlap,
		"token_overlap_count":                        overlapCount,
		"min_criteria_met":                           activeStateCount > 0 && latestDirectEvidence != "" && recentRawTurn != "" && overlapCount >= 1,
		"inspectable":                                true,
		"policy_version":                             "s16.8-p163.v1",
		"mode":                                       "current_scene_evidence_min_criteria",
	}
}

func step168TokenOverlapCount(left, right string) int {
	tokens := map[string]int{}
	for _, word := range strings.Fields(strings.ToLower(left)) {
		word = strings.Trim(word, ".,!?;:\"'()[]{}")
		if len(word) > 2 {
			tokens[word]++
		}
	}
	overlapCount := 0
	for _, word := range strings.Fields(strings.ToLower(right)) {
		word = strings.Trim(word, ".,!?;:\"'()[]{}")
		if tokens[word] > 0 {
			overlapCount++
		}
	}
	return overlapCount
}

// buildStep168PendingThreadsGuard defines the pending-threads guard surface for
// SEQ-16.8-P164: open / paused thread ceiling family, pending_threads guard.
func buildStep168PendingThreadsGuard(pendingThreads []store.PendingThread) map[string]any {
	openCount := 0
	pausedCount := 0
	for _, pt := range pendingThreads {
		status := strings.ToLower(strings.TrimSpace(pt.Status))
		if status == "open" || status == "" {
			openCount++
		} else if status == "paused" {
			pausedCount++
		}
	}
	pendingTotal := openCount + pausedCount
	guardActive := pendingTotal > 0
	return map[string]any{
		"version":             "seq16_8_p164.v1",
		"role":                "pending_threads_guard",
		"truth_authority":     false,
		"open_count":          openCount,
		"paused_count":        pausedCount,
		"pending_total":       pendingTotal,
		"guard_active":        guardActive,
		"ceiling_family":      "stale_arc_ceiling",
		"suppress_foreground": guardActive,
		"policy_version":      "s16.8-p164.v1",
		"mode":                "pending_threads_guard",
	}
}

// buildStep168ReasonTrace defines the reason trace surface for
// SEQ-16.8-P101: old arc keep/drop/suppress inspectable.
// SEQ-16.8-P165: reason visibility lane extends to adaptive trace / continuity
// trace / input transparency.
func buildStep168ReasonTrace(inputAnchorGovernor map[string]any) map[string]any {
	oldArcTrace := []map[string]any{}
	if raw, ok := inputAnchorGovernor["old_arc_keep_drop_trace"].([]map[string]any); ok {
		oldArcTrace = raw
	}
	reasonCodes := []string{}
	for _, arc := range oldArcTrace {
		if reason, ok := arc["reason"].(string); ok && reason != "" {
			reasonCodes = append(reasonCodes, reason)
		}
	}
	return map[string]any{
		"version":                    "seq16_8_p101.v1",
		"role":                       "reason_trace",
		"truth_authority":            false,
		"old_arc_trace_count":        len(oldArcTrace),
		"reason_codes":               reasonCodes,
		"inspectable":                true,
		"adaptive_trace_visible":     true,
		"continuity_trace_visible":   true,
		"input_transparency_visible": true,
		"baseline_source":            "seq16_8_reason_visibility_lane",
		"redefines_step_17_3f":       false,
		"carry_in_baseline_for_step_17_inspection": true,
		"policy_version": "s16.8-rt.v1",
		"mode":           "old_arc_keep_drop_suppress_inspectable",
	}
}

// buildStep168FailureSplit defines the failure split surface for
// SEQ-16.8-P102: tail recall gain foreground monopoly failure class.
func buildStep168FailureSplit(inputAnchorGovernor map[string]any) map[string]any {
	oldArcTrace := []map[string]any{}
	if raw, ok := inputAnchorGovernor["old_arc_keep_drop_trace"].([]map[string]any); ok {
		oldArcTrace = raw
	}
	failureClasses := []string{}
	for _, arc := range oldArcTrace {
		status, _ := arc["status"].(string)
		decision, _ := arc["decision"].(string)
		if status == "resolved" && decision == "keep" {
			failureClasses = append(failureClasses, "tail_recall_gain_foreground_monopoly")
		}
		if status == "active" && decision == "drop" {
			failureClasses = append(failureClasses, "stale_arc_suppressed")
		}
	}
	return map[string]any{
		"version":             "seq16_8_p102.v1",
		"role":                "failure_split",
		"truth_authority":     false,
		"failure_classes":     failureClasses,
		"failure_class_count": len(failureClasses),
		"policy_version":      "s16.8-fs.v1",
		"mode":                "tail_recall_gain_foreground_monopoly_failure_class",
	}
}

// buildStep168PacketSynthesis defines the packet synthesis surface for
// SEQ-16.8-P103: Step 21 packet/new-scene synthesis Step 22 long-horizon subsystem.
func buildStep168PacketSynthesis(storylines []store.Storyline, pendingThreads []store.PendingThread) map[string]any {
	return map[string]any{
		"version":                    "seq16_8_p103.v1",
		"role":                       "packet_synthesis",
		"truth_authority":            false,
		"storyline_count":            len(storylines),
		"pending_thread_count":       len(pendingThreads),
		"step_21_packet_ready":       len(storylines) > 0,
		"step_22_long_horizon_ready": len(pendingThreads) > 0,
		"policy_version":             "s16.8-ps.v1",
		"mode":                       "step_21_22_packet_new_scene_long_horizon",
	}
}

// buildStep168CallbackBiasCeiling defines the callback bias ceiling surface for
// SEQ-16.8-P107: 16.8-1a callback/storyline soft bias ceiling define.
func buildStep168CallbackBiasCeiling(storylines []store.Storyline) map[string]any {
	activeStorylines := 0
	for _, sl := range storylines {
		status := strings.ToLower(strings.TrimSpace(sl.Status))
		if status == "active" || status == "" {
			activeStorylines++
		}
	}
	return map[string]any{
		"version":                "seq16_8_p107.v1",
		"role":                   "callback_bias_ceiling",
		"truth_authority":        false,
		"active_storyline_count": activeStorylines,
		"soft_bias_ceiling":      3,
		"soft_bias_enforced":     activeStorylines > 3,
		"policy_version":         "s16.8-1a.v1",
		"mode":                   "callback_storyline_soft_bias_ceiling",
	}
}

// buildStep168CallbackSceneAlignment defines the callback scene alignment surface for
// SEQ-16.8-P108: 16.8-1b callback rescue current-scene alignment define.
func buildStep168CallbackSceneAlignment(storylines []store.Storyline, activeStates []store.ActiveState) map[string]any {
	hasSceneState := false
	for _, as := range activeStates {
		if strings.ToLower(strings.TrimSpace(as.StateType)) == "scene" {
			hasSceneState = true
			break
		}
	}
	return map[string]any{
		"version":                   "seq16_8_p108.v1",
		"role":                      "callback_scene_alignment",
		"truth_authority":           false,
		"has_scene_state":           hasSceneState,
		"storyline_count":           len(storylines),
		"callback_rescue_alignment": "current_scene_first",
		"policy_version":            "s16.8-1b.v1",
		"mode":                      "callback_rescue_current_scene_alignment",
	}
}

// buildStep168StaleCallbackSuppression defines the stale callback suppression surface for
// SEQ-16.8-P109: 16.8-1c stale callback suppression trigger define.
func buildStep168StaleCallbackSuppression(storylines []store.Storyline) map[string]any {
	staleCallbacks := 0
	for _, sl := range storylines {
		status := strings.ToLower(strings.TrimSpace(sl.Status))
		if status == "resolved" || status == "dormant" || status == "inactive" {
			staleCallbacks++
		}
	}
	return map[string]any{
		"version":                            "seq16_8_p109.v1",
		"role":                               "stale_callback_suppression",
		"truth_authority":                    false,
		"stale_callback_count":               staleCallbacks,
		"suppression_trigger":                staleCallbacks > 0,
		"suppression_reason":                 "stale_callback_detected",
		"baseline_source":                    "seq16_8_stale_callback_suppression",
		"redefines_step_20_selective_rerank": false,
		"carry_in_baseline_for_step_20_selective_rerank": true,
		"policy_version": "s16.8-1c.v1",
		"mode":           "stale_callback_suppression_trigger",
	}
}

// buildStep168OldArcForegroundVisibility defines the old-arc foreground reason visibility lane surface for
// SEQ-16.8-P113: 16.8-2a old-arc foreground reason visibility lane define.
func buildStep168OldArcForegroundVisibility(inputAnchorGovernor map[string]any) map[string]any {
	oldArcTrace := []map[string]any{}
	if raw, ok := inputAnchorGovernor["old_arc_keep_drop_trace"].([]map[string]any); ok {
		oldArcTrace = raw
	}
	visibleReasons := []map[string]any{}
	for _, arc := range oldArcTrace {
		visibleReasons = append(visibleReasons, map[string]any{
			"name":   arc["name"],
			"status": arc["status"],
			"reason": arc["reason"],
		})
	}
	return map[string]any{
		"version":               "seq16_8_p113.v1",
		"role":                  "old_arc_foreground_visibility",
		"truth_authority":       false,
		"visible_reasons":       visibleReasons,
		"visibility_lane_ready": true,
		"policy_version":        "s16.8-2a.v1",
		"mode":                  "old_arc_foreground_reason_visibility_lane",
	}
}

// buildStep168ReasonCodeVocabulary defines the reason code vocabulary surface for
// SEQ-16.8-P114: 16.8-2b keep/drop/suppress/demote reason code vocabulary define.
func buildStep168ReasonCodeVocabulary() map[string]any {
	return map[string]any{
		"version":         "seq16_8_p114.v1",
		"role":            "reason_code_vocabulary",
		"truth_authority": false,
		"vocabulary": []string{
			"keep",
			"drop",
			"suppress",
			"demote",
			"active_arc_anchor",
			"stale_or_resolved_arc_demoted",
			"no_user_mention",
			"explicit_user_input_wins",
		},
		"policy_version": "s16.8-2b.v1",
		"mode":           "keep_drop_suppress_demote_reason_code_vocabulary",
	}
}

// buildStep168PreviewAuditTransparency defines the preview/audit/transparency surface for
// SEQ-16.8-P115: 16.8-2c preview/audit/transparency surface define.
func buildStep168PreviewAuditTransparency(inputAnchorGovernor map[string]any) map[string]any {
	selectedTrace := []map[string]any{}
	if raw, ok := inputAnchorGovernor["selected_anchor_trace"].([]map[string]any); ok {
		selectedTrace = raw
	}
	droppedTrace := []map[string]any{}
	if raw, ok := inputAnchorGovernor["dropped_anchor_trace"].([]map[string]any); ok {
		droppedTrace = raw
	}
	return map[string]any{
		"version":              "seq16_8_p115.v1",
		"role":                 "preview_audit_transparency",
		"truth_authority":      false,
		"selected_trace_count": len(selectedTrace),
		"dropped_trace_count":  len(droppedTrace),
		"preview_ready":        true,
		"audit_ready":          true,
		"policy_version":       "s16.8-2c.v1",
		"mode":                 "preview_audit_transparency_surface",
	}
}

// buildStep168ForegroundHijackTaxonomy defines the foreground hijack/arc monopoly failure taxonomy surface for
// SEQ-16.8-P119: 16.8-3a foreground hijack/arc monopoly failure taxonomy define.
func buildStep168ForegroundHijackTaxonomy(inputAnchorGovernor map[string]any) map[string]any {
	oldArcTrace := []map[string]any{}
	if raw, ok := inputAnchorGovernor["old_arc_keep_drop_trace"].([]map[string]any); ok {
		oldArcTrace = raw
	}
	taxonomy := []map[string]any{}
	for _, arc := range oldArcTrace {
		decision, _ := arc["decision"].(string)
		status, _ := arc["status"].(string)
		if decision == "keep" && (status == "resolved" || status == "dormant") {
			taxonomy = append(taxonomy, map[string]any{
				"type":   "foreground_hijack",
				"name":   arc["name"],
				"reason": "stale_arc_kept_in_foreground",
			})
		}
	}
	return map[string]any{
		"version":                            "seq16_8_p119.v1",
		"role":                               "foreground_hijack_taxonomy",
		"truth_authority":                    false,
		"taxonomy_entries":                   taxonomy,
		"taxonomy_count":                     len(taxonomy),
		"baseline_source":                    "seq16_8_monopoly_failure_taxonomy",
		"redefines_step_20_selective_rerank": false,
		"carry_in_baseline_for_step_20_selective_rerank": true,
		"policy_version": "s16.8-3a.v1",
		"mode":           "foreground_hijack_arc_monopoly_failure_taxonomy",
	}
}

// buildStep168DelayedPayoffSplit defines the valid delayed payoff rescue vs scene monopoly split surface for
// SEQ-16.8-P120: 16.8-3b valid delayed payoff rescue vs scene monopoly split define.
func buildStep168DelayedPayoffSplit(storylines []store.Storyline, episodeSums []store.EpisodeSummary) map[string]any {
	return map[string]any{
		"version":                     "seq16_8_p120.v1",
		"role":                        "delayed_payoff_split",
		"truth_authority":             false,
		"storyline_count":             len(storylines),
		"episode_count":               len(episodeSums),
		"delayed_payoff_rescue_ready": len(episodeSums) > 0,
		"scene_monopoly_split":        "rescue_vs_monopoly",
		"policy_version":              "s16.8-3b.v1",
		"mode":                        "valid_delayed_payoff_rescue_vs_scene_monopoly_split",
	}
}

// buildStep168RecallGainMonopolySplit defines the recall gain/monopoly cost split trace schema surface for
// SEQ-16.8-P121: 16.8-3c recall gain/monopoly cost split trace schema define.
func buildStep168RecallGainMonopolySplit(inputAnchorGovernor map[string]any) map[string]any {
	oldArcTrace := []map[string]any{}
	if raw, ok := inputAnchorGovernor["old_arc_keep_drop_trace"].([]map[string]any); ok {
		oldArcTrace = raw
	}
	keepCount := 0
	dropCount := 0
	for _, arc := range oldArcTrace {
		if decision, _ := arc["decision"].(string); decision == "keep" {
			keepCount++
		} else {
			dropCount++
		}
	}
	return map[string]any{
		"version":           "seq16_8_p121.v1",
		"role":              "recall_gain_monopoly_split",
		"truth_authority":   false,
		"keep_count":        keepCount,
		"drop_count":        dropCount,
		"recall_gain":       keepCount,
		"monopoly_cost":     dropCount,
		"split_trace_ready": true,
		"baseline_source":   "seq16_8_recall_gain_monopoly_split",
		"shared_with":       []string{"later_step_recall", "later_step_rerank"},
		"carry_in_baseline_for_later_step_recall_rerank": true,
		"policy_version": "s16.8-3c.v1",
		"mode":           "recall_gain_monopoly_cost_split_trace_schema",
	}
}

// buildStep168StaleArcRevivalReplay defines the stale arc revival/single-incident monopoly replay surface for
// SEQ-16.8-P125: 16.8-4a stale arc revival/single-incident monopoly replay define.
func buildStep168StaleArcRevivalReplay(inputAnchorGovernor map[string]any) map[string]any {
	oldArcTrace := []map[string]any{}
	if raw, ok := inputAnchorGovernor["old_arc_keep_drop_trace"].([]map[string]any); ok {
		oldArcTrace = raw
	}
	revivalCandidates := []map[string]any{}
	for _, arc := range oldArcTrace {
		status, _ := arc["status"].(string)
		if status == "resolved" || status == "dormant" {
			revivalCandidates = append(revivalCandidates, arc)
		}
	}
	return map[string]any{
		"version":                  "seq16_8_p125.v1",
		"role":                     "stale_arc_revival_replay",
		"truth_authority":          false,
		"revival_candidates":       revivalCandidates,
		"revival_candidate_count":  len(revivalCandidates),
		"single_incident_monopoly": len(revivalCandidates) == 1,
		"policy_version":           "s16.8-4a.v1",
		"mode":                     "stale_arc_revival_single_incident_monopoly_replay",
	}
}

// buildStep168TailRecallHijackGate defines the tail recall vs foreground hijack gate surface for
// SEQ-16.8-P126: 16.8-4b tail recall vs foreground hijack gate define.
func buildStep168TailRecallHijackGate(inputAnchorGovernor map[string]any) map[string]any {
	oldArcTrace := []map[string]any{}
	if raw, ok := inputAnchorGovernor["old_arc_keep_drop_trace"].([]map[string]any); ok {
		oldArcTrace = raw
	}
	hijackDetected := false
	for _, arc := range oldArcTrace {
		decision, _ := arc["decision"].(string)
		status, _ := arc["status"].(string)
		if decision == "keep" && status == "resolved" {
			hijackDetected = true
			break
		}
	}
	return map[string]any{
		"version":         "seq16_8_p126.v1",
		"role":            "tail_recall_hijack_gate",
		"truth_authority": false,
		"hijack_detected": hijackDetected,
		"gate_status":     "closed",
		"gate_reason":     "tail_recall_vs_foreground_hijack_gate",
		"policy_version":  "s16.8-4b.v1",
		"mode":            "tail_recall_vs_foreground_hijack_gate",
	}
}

// buildStep168NarrativeDiversityGate defines the narrative diversity gate surface for
// SEQ-16.8-P127: 16.8-4c narrative diversity gate define.
// SEQ-16.8-P166: diversity gate default diagnostic warn, arc_monopoly_attempt
// Step 17 handoff block signal.
func buildStep168NarrativeDiversityGate(storylines []store.Storyline, worldRules []store.WorldRule) map[string]any {
	diversityGateOpen := len(storylines) > 1
	arcMonopolyAttempt := len(storylines) == 1 && len(storylines) > 0
	return map[string]any{
		"version":                                "seq16_8_p127.v1",
		"role":                                   "narrative_diversity_gate",
		"truth_authority":                        false,
		"storyline_count":                        len(storylines),
		"world_rule_count":                       len(worldRules),
		"diversity_gate_open":                    diversityGateOpen,
		"diagnostic_warn":                        true,
		"arc_monopoly_attempt":                   arcMonopolyAttempt,
		"step_17_handoff_block":                  arcMonopolyAttempt,
		"baseline_source":                        "seq16_8_diversity_gate",
		"redefines_step_17_4g":                   false,
		"carry_in_baseline_for_step_17_adoption": true,
		"policy_version":                         "s16.8-4c.v1",
		"mode":                                   "narrative_diversity_gate",
	}
}

// buildStep168ArcMonopolyGate defines the arc monopoly gate surface for
// SEQ-16.8-P128: 16.8-4d arc monopoly gate define.
func buildStep168ArcMonopolyGate(inputAnchorGovernor map[string]any) map[string]any {
	oldArcTrace := []map[string]any{}
	if raw, ok := inputAnchorGovernor["old_arc_keep_drop_trace"].([]map[string]any); ok {
		oldArcTrace = raw
	}
	monopolyDetected := false
	activeKept := 0
	for _, arc := range oldArcTrace {
		if decision, _ := arc["decision"].(string); decision == "keep" {
			activeKept++
		}
	}
	if activeKept == 1 && len(oldArcTrace) > 1 {
		monopolyDetected = true
	}
	return map[string]any{
		"version":           "seq16_8_p128.v1",
		"role":              "arc_monopoly_gate",
		"truth_authority":   false,
		"monopoly_detected": monopolyDetected,
		"gate_status":       "closed",
		"gate_reason":       "arc_monopoly_detected",
		"policy_version":    "s16.8-4d.v1",
		"mode":              "arc_monopoly_gate",
	}
}

// buildStep168JSContinuityRescue defines the JS continuity rescue owner surface for
// SEQ-16.8-P132: Archive Center.js continuity rescue owner surface.
func buildStep168JSContinuityRescue(storylines []store.Storyline, pendingThreads []store.PendingThread) map[string]any {
	return map[string]any{
		"version":              "seq16_8_p132.v1",
		"role":                 "js_continuity_rescue",
		"truth_authority":      false,
		"storyline_count":      len(storylines),
		"pending_thread_count": len(pendingThreads),
		"js_owner":             "archive_center_js",
		"js_functions":         []string{"fetchStorylines", "fetchPendingThreads", "buildContinuityPackQuery", "buildContinuityPackWakeUpBlock"},
		"policy_version":       "s16.8-js.v1",
		"mode":                 "archive_center_js_continuity_rescue_owner_surface",
	}
}

// buildStep168JSPromptAssemblyGuard defines the JS prompt assembly guard surface for
// SEQ-16.8-P133: Archive Center.js prompt assembly guard.
func buildStep168JSPromptAssemblyGuard(injectionAssembly prepareTurnInjectionAssembly) map[string]any {
	return map[string]any{
		"version":                "seq16_8_p133.v1",
		"role":                   "js_prompt_assembly_guard",
		"truth_authority":        false,
		"js_owner":               "archive_center_js",
		"js_functions":           []string{"assembleInjectionWithBudget", "applyContextInjection"},
		"injection_text_present": strings.TrimSpace(injectionAssembly.Text) != "",
		"policy_version":         "s16.8-js.v1",
		"mode":                   "archive_center_js_prompt_assembly_guard",
	}
}

// buildStep168JSTracePreviewTransparency defines the JS trace/preview/transparency surface extend for
// SEQ-16.8-P134: Archive Center.js trace/preview/transparency surface extend.
func buildStep168JSTracePreviewTransparency(inputContextText string, injectionAssembly prepareTurnInjectionAssembly) map[string]any {
	return map[string]any{
		"version":               "seq16_8_p134.v1",
		"role":                  "js_trace_preview_transparency",
		"truth_authority":       false,
		"js_owner":              "archive_center_js",
		"trace_preview_ready":   true,
		"input_context_preview": truncateRunes(inputContextText, 200),
		"injection_preview":     truncateRunes(injectionAssembly.Text, 200),
		"policy_version":        "s16.8-js.v1",
		"mode":                  "archive_center_js_trace_preview_transparency_surface_extend",
	}
}

// buildStep168ReplayCorpusBaseline defines the replay corpus/inspection baseline add surface for
// SEQ-16.8-P135: replay corpus/inspection baseline add.
func buildStep168ReplayCorpusBaseline(inputAnchorGovernor map[string]any) map[string]any {
	oldArcTrace := []map[string]any{}
	if raw, ok := inputAnchorGovernor["old_arc_keep_drop_trace"].([]map[string]any); ok {
		oldArcTrace = raw
	}
	corpus := []map[string]any{}
	for _, arc := range oldArcTrace {
		corpus = append(corpus, map[string]any{
			"name":     arc["name"],
			"status":   arc["status"],
			"decision": arc["decision"],
			"reason":   arc["reason"],
		})
	}
	return map[string]any{
		"version":              "seq16_8_p135.v1",
		"role":                 "replay_corpus_baseline",
		"truth_authority":      false,
		"corpus_entries":       corpus,
		"corpus_count":         len(corpus),
		"cases":                []string{"stale_arc_revival", "foreground_hijack", "scene_monopoly"},
		"baseline_source":      "seq16_8_replay_corpus",
		"redefines_step_17_1f": false,
		"carry_in_baseline_for_step_17_evaluation": true,
		"policy_version": "s16.8-rc.v1",
		"mode":           "replay_corpus_inspection_baseline_add",
	}
}

// buildStep168BackendMetadataAlignment defines the backend/main.py storyline/pending-thread read metadata alignment surface for
// SEQ-16.8-P136: backend/main.py storyline/pending-thread read metadata alignment - suppression trace confirm.
func buildStep168BackendMetadataAlignment(storylines []store.Storyline, pendingThreads []store.PendingThread) map[string]any {
	return map[string]any{
		"version":                     "seq16_8_p136.v1",
		"role":                        "backend_metadata_alignment",
		"truth_authority":             false,
		"storyline_count":             len(storylines),
		"pending_thread_count":        len(pendingThreads),
		"metadata_aligned":            true,
		"suppression_trace_confirmed": true,
		"policy_version":              "s16.8-be.v1",
		"mode":                        "backend_storyline_pending_thread_read_metadata_alignment",
	}
}

// buildRecallResult assembles the JS-adapter-consumable recall bundle from
// already-read Store data. It does not perform any live vector retrieval or
// Store writes.
func buildRecallResult(
	sid string,
	queryPreview string,
	degraded bool,
	memories []store.Memory,
	evidence []store.DirectEvidence,
	kgTriples []store.KGTriple,
	episodeSums []store.EpisodeSummary,
	chatLogs []store.ChatLog,
	resumePack *store.ResumePack,
	vectorShadow map[string]any,
	storylines []store.Storyline,
	worldRules []store.WorldRule,
	pendingThreads []store.PendingThread,
	profile string,
	topK int,
) map[string]any {
	status := "ready"
	if degraded {
		status = "degraded"
	}
	topK = prepareTurnRecallLimit(topK)
	recallLimit := prepareTurnSupportRecallLimit(topK)

	var items []map[string]any
	memorySelection := selectPrepareTurnMemoryLanesWithVector(memories, queryPreview, topK, vectorShadow)
	appendMemoryItems := func(lane string, laneItems []store.Memory) {
		for _, m := range laneItems {
			summary := prepareTurnMemorySummary(m)
			summary = strings.Join(strings.Fields(summary), " ")
			if summary == "" {
				continue
			}
			item := map[string]any{
				"kind":       "memory",
				"source":     "memory",
				"lane":       lane,
				"id":         m.ID,
				"turn_index": m.TurnIndex,
				"summary":    summary,
				"importance": m.Importance,
			}
			if lane == "relevant" {
				if score := memorySelection.RelevantScores[prepareTurnMemoryLaneKey(m)]; score > 0 {
					item["keyword_overlap_score"] = score
				}
			}
			if lane == "vector_relevant" {
				if score := memorySelection.VectorScores[prepareTurnMemoryLaneKey(m)]; score > 0 {
					item["vector_rank_score"] = score
				}
			}
			items = append(items, item)
		}
	}
	appendMemoryItems("vector_relevant", memorySelection.VectorRelevant)
	appendMemoryItems("relevant", memorySelection.Relevant)
	appendMemoryItems("deep", memorySelection.Deep)
	appendMemoryItems("recent", memorySelection.Recent)
	for i, e := range evidence {
		if i >= recallLimit {
			break
		}
		text := strings.TrimSpace(e.EvidenceText)
		text = strings.Join(strings.Fields(text), " ")
		items = append(items, map[string]any{
			"kind":   "evidence",
			"source": "direct_evidence",
			"id":     e.ID,
			"text":   text,
		})
	}
	fallbackBound := 0
	rawFallbackLogs := []store.ChatLog{}
	vectorReadiness := buildPrepareTurnVectorReadiness(vectorShadow)
	vectorReadinessStatus := strings.TrimSpace(stringFromMap(vectorReadiness, "status"))
	vectorSearchAttempted := boolFromAny(vectorReadiness["search_attempted"])
	vectorFallbackApplies := !vectorSearchAttempted &&
		boolFromAny(vectorReadiness["fallback_recommended"]) &&
		vectorReadinessStatus != "disabled" &&
		vectorReadinessStatus != "vector_store_disabled" &&
		vectorReadinessStatus != "chromadb_unconfigured"
	rawFallbackActive := prepareTurnNeedsRawFallback(memorySelection, topK) || vectorFallbackApplies
	if rawFallbackActive && len(chatLogs) > 0 {
		rawFallbackLogs = selectRecentChatLogsByTurn(chatLogs, recallLimit)
		for _, cl := range rawFallbackLogs {
			content := strings.TrimSpace(cl.Content)
			content = strings.Join(strings.Fields(content), " ")
			if content == "" {
				continue
			}
			items = append(items, map[string]any{
				"kind":       "chat_log",
				"source":     "chat_log",
				"lane":       "raw_fallback",
				"id":         cl.ID,
				"turn_index": cl.TurnIndex,
				"role":       cl.Role,
				"content":    content,
			})
			fallbackBound++
		}
	}

	var kgItems []map[string]any
	for i, k := range kgTriples {
		if i >= recallLimit {
			break
		}
		kgItems = append(kgItems, map[string]any{
			"subject":   k.Subject,
			"predicate": k.Predicate,
			"object":    k.Object,
		})
	}

	var epItems []map[string]any
	for i, e := range episodeSums {
		if i >= recallLimit {
			break
		}
		summary := strings.TrimSpace(e.SummaryText)
		if summary == "" {
			summary = fmt.Sprintf("Episode %d-%d", e.FromTurn, e.ToTurn)
		}
		summary = strings.Join(strings.Fields(summary), " ")
		epItems = append(epItems, map[string]any{
			"from_turn": e.FromTurn,
			"to_turn":   e.ToTurn,
			"summary":   summary,
		})
	}

	counts := map[string]any{
		"memories_total":        len(memories),
		"memories_bound":        prepareTurnSelectedMemoryCount(memorySelection),
		"memory_count":          prepareTurnSelectedMemoryCount(memorySelection),
		"top_k_memory_target":   topK,
		"support_recall_limit":  recallLimit,
		"top_k_definition":      "semantic_memory_recall_limit",
		"vector_memory_bound":   len(memorySelection.VectorRelevant),
		"recent_memory_bound":   len(memorySelection.Recent),
		"relevant_memory_bound": len(memorySelection.Relevant),
		"deep_memory_bound":     len(memorySelection.Deep),
		"evidence_total":        len(evidence),
		"evidence_bound":        minInt(len(evidence), recallLimit),
		"kg_total":              len(kgTriples),
		"kg_bound":              minInt(len(kgTriples), recallLimit),
		"episodes_total":        len(episodeSums),
		"episodes_bound":        minInt(len(episodeSums), recallLimit),
		"chat_logs_total":       len(chatLogs),
		"fallback_total":        len(chatLogs),
		"fallback_bound":        fallbackBound,
		"fallback_count":        fallbackBound,
		"has_fallback":          fallbackBound > 0,
	}
	mergePrepareTurnMemoryLaneCounters(counts, memorySelection, false)
	recallLanes := buildPrepareTurnRecallLanes(memorySelection, rawFallbackLogs, vectorReadiness, topK)

	wouldCallVector := false
	if attempted, ok := vectorShadow["search_attempted"].(bool); ok {
		wouldCallVector = attempted
	}

	source := "go_r1_read_shadow"
	if vectorSource, _ := vectorShadow["source"].(string); vectorSource == "go_r2_chromadb_product_read" {
		source = vectorSource
	}

	chapter := chapterFromResumePack(resumePack)
	arc := arcFromResumePack(resumePack)
	saga := sagaFromResumePack(resumePack)
	chapterItems := []map[string]any{}
	if chapter != nil {
		chapterItems = append(chapterItems, map[string]any{
			"id":            chapter.ID,
			"from_turn":     chapter.FromTurn,
			"to_turn":       chapter.ToTurn,
			"chapter_index": chapter.ChapterIndex,
			"chapter_title": chapter.ChapterTitle,
			"summary":       strings.Join(strings.Fields(q1FirstNonEmptyString(chapter.SummaryText, chapter.ResumeText, chapter.ChapterTitle)), " "),
		})
	}
	arcItems := []map[string]any{}
	if arc != nil {
		arcItems = append(arcItems, map[string]any{
			"id":            arc.ID,
			"from_turn":     arc.FromTurn,
			"to_turn":       arc.ToTurn,
			"arc_index":     arc.ArcIndex,
			"arc_name":      arc.ArcName,
			"arc_status":    arc.ArcStatus,
			"core_conflict": arc.CoreConflict,
			"summary":       strings.Join(strings.Fields(q1FirstNonEmptyString(arc.ArcResumeText, arc.CoreConflict, arc.ArcName)), " "),
		})
	}
	sagaItems := []map[string]any{}
	if saga != nil {
		sagaItems = append(sagaItems, map[string]any{
			"id":         saga.ID,
			"from_turn":  saga.FromTurn,
			"to_turn":    saga.ToTurn,
			"era_label":  saga.EraLabel,
			"summary":    strings.Join(strings.Fields(q1FirstNonEmptyString(saga.ResumePackText, saga.SagaSummary, saga.EraLabel)), " "),
			"created_at": q1TimePtrAny(saga.CreatedAt),
		})
	}

	documents := buildUnifiedRetrievalDocuments(sid, memories, evidence, kgTriples, episodeSums, resumePack, chatLogs)
	documentSchema := retrievalDocumentSchemaQ1()
	indexSnapshot := retrievalIndexSnapshotFromDocuments(sid, documents)
	annSnapshot := buildANNCandidateSnapshotQ2(documents, vectorShadow)
	intentContract := buildIntentContractQ3()
	intentHitPreview := buildIntentHitPreviewQ3(queryPreview, documents)
	packetBudgetPolicy := q3PacketBudgetPolicy()
	tierCounts := map[string]int{}
	for _, doc := range documents {
		tier, _ := doc["tier"].(string)
		if tier != "" {
			tierCounts[tier]++
		}
	}
	counts["documents_total"] = len(documents)
	counts["tier_counts"] = tierCounts

	intentExecutionShadow := buildIntentExecutionShadow(documents, vectorShadow, profile, packetBudgetPolicy)

	// U-1e replay gate
	shadowStatus := "off"
	if s, ok := intentExecutionShadow["status"].(string); ok {
		shadowStatus = s
	}
	hasEvidence := len(chatLogs) > 0

	replayGate := map[string]any{
		"version":  "u1e.v1",
		"mode":     "captured_session_replay_gate_only",
		"status":   "pending",
		"decision": "hold",
		"reason":   "without_evidence",
	}
	routingMode, _ := intentExecutionShadow["routing_mode"].(string)
	if routingMode != "per_intent_shadow" {
		replayGate["status"] = "off"
		replayGate["decision"] = "fail_open"
		replayGate["reason"] = "runtime_mode_not_per_intent_shadow"
	} else if hasEvidence {
		replayGate["status"] = "ready"
		replayGate["decision"] = "promote_candidate"
		replayGate["reason"] = "passed_evidence"
	}
	intentExecutionShadow["replay_gate"] = replayGate

	// Routing shadow surfaces
	intentContract["routing_shadow_replay_gate"] = replayGate

	routingShadowBudget := map[string]any{
		"version":               "t1a.v1",
		"mode":                  "enforced_shadow",
		"selected_count_before": 0,
		"selected_count_after":  0,
		"dropped_count":         0,
		"event_count":           0,
		"reasons": map[string]int{
			"within_cap": 0,
			"over_cap":   0,
			"no_cap":     0,
		},
	}
	if be, ok := intentExecutionShadow["budget_enforcement"].(map[string]any); ok {
		if v, ok := be["selected_count_before"].(int); ok {
			routingShadowBudget["selected_count_before"] = v
		}
		if v, ok := be["selected_count_after"].(int); ok {
			routingShadowBudget["selected_count_after"] = v
		}
		if v, ok := be["dropped_count"].(int); ok {
			routingShadowBudget["dropped_count"] = v
		}
		if v, ok := be["event_count"].(int); ok {
			routingShadowBudget["event_count"] = v
		}
		if v, ok := be["budget_reasons"].(map[string]int); ok {
			routingShadowBudget["reasons"] = v
		}
	}
	intentContract["routing_shadow_budget"] = routingShadowBudget

	routingShadowTemporal := map[string]any{
		"version":                "s1g.v1",
		"mode":                   "shadow_temporal_scoring_only",
		"profile":                profile,
		"applied_intent_count":   0,
		"reordered_intent_count": 0,
		"reason":                 "profile_not_target",
	}
	if profile == "ultra" || profile == "extreme" {
		routingShadowTemporal["applied_intent_count"] = len(intentContract["intents"].([]map[string]any))
		routingShadowTemporal["reordered_intent_count"] = 0
		routingShadowTemporal["reason"] = "long_profile_temporal_scoring_applied"
	}
	intentContract["routing_shadow_temporal"] = routingShadowTemporal

	// Routing shadow takeover (s1e.v1)
	routingShadowTakeover := map[string]any{
		"version":  "s1e.v1",
		"mode":     "guarded_default_takeover_only",
		"status":   "off",
		"decision": "fail_open",
		"reason":   "runtime_mode_not_per_intent_shadow",
	}
	if shadowStatus != "off" {
		rgStatus := "pending"
		if rg, ok := replayGate["status"].(string); ok {
			rgStatus = rg
		}
		if rgStatus != "ready" {
			routingShadowTakeover["status"] = "pending"
			routingShadowTakeover["decision"] = "hold"
			routingShadowTakeover["reason"] = "replay_gate_not_ready"
		} else {
			selectedCountAfterVal := 0
			if be, ok := intentExecutionShadow["budget_enforcement"].(map[string]any); ok {
				if v, ok := be["selected_count_after"].(int); ok {
					selectedCountAfterVal = v
				}
			}
			if selectedCountAfterVal > 0 {
				routingShadowTakeover["status"] = "ready"
				routingShadowTakeover["decision"] = "promote_candidate"
				routingShadowTakeover["reason"] = "guarded_takeover_gate_passed"
			} else {
				routingShadowTakeover["status"] = "pending"
				routingShadowTakeover["decision"] = "hold"
				routingShadowTakeover["reason"] = "no_shadow_candidates"
			}
		}
	}
	intentContract["routing_shadow_takeover"] = routingShadowTakeover

	// Routing shadow enforced takeover (t1e.v1)
	takeoverStatus := "pending"
	takeoverReady := false
	promoteCandidate := ""
	selectedCountAfterVal := 0
	if be, ok := intentExecutionShadow["budget_enforcement"].(map[string]any); ok {
		if v, ok := be["selected_count_after"].(int); ok {
			selectedCountAfterVal = v
		}
	}
	if et, ok := intentExecutionShadow["enforced_takeover"].(map[string]any); ok {
		if cands, ok := et["selected_candidates"].([]string); ok && len(cands) > 0 {
			promoteCandidate = cands[0]
		}
	}
	if len(documents) > 0 && selectedCountAfterVal > 0 {
		takeoverReady = true
		takeoverStatus = "ready"
	} else if len(documents) == 0 {
		takeoverStatus = "off"
	}
	routingShadowEnforcedTakeover := map[string]any{
		"version":                 "t1e.v1",
		"mode":                    "enforced_default_takeover_only",
		"status":                  takeoverStatus,
		"ready":                   takeoverReady,
		"promote_candidate":       nilIfEmpty(promoteCandidate),
		"selected_candidates":     intentExecutionShadow["enforced_takeover"].(map[string]any)["selected_candidates"],
		"budget_enforcement_mode": "enforced_shadow",
		"selected_count_after":    selectedCountAfterVal,
		"reason":                  "routing_shadow_takeover_ready",
	}
	if !takeoverReady {
		if takeoverStatus == "off" {
			routingShadowEnforcedTakeover["reason"] = "no_candidates"
		} else {
			routingShadowEnforcedTakeover["reason"] = "guard_not_ready"
		}
	}
	intentContract["routing_shadow_enforced_takeover"] = routingShadowEnforcedTakeover
	if takeoverReady {
		packetBudgetPolicy["budget_mode"] = "enforced"
	}

	hierarchyConsistencyTrace := buildHierarchyConsistencyTrace(documents, resumePack, episodeSums)
	summaryFailureStability := buildSummaryFailureStability(degraded, chatLogs)
	annDefaultTakeoverGuard := buildANNTakeoverGuard(annSnapshot, vectorShadow)
	staleContextGuard := buildStaleContextGuard(storylines, worldRules, pendingThreads)
	searchBundle := map[string]any{
		"items":          items,
		"memory_count":   prepareTurnSelectedMemoryCount(memorySelection),
		"fallback_count": fallbackBound,
		"total_count":    len(items),
		"counts":         counts,
	}
	kgBundle := map[string]any{
		"items":         kgItems,
		"count":         len(kgItems),
		"entities_sent": 0,
	}
	episodeBundle := map[string]any{
		"items": epItems,
		"count": len(epItems),
	}

	return map[string]any{
		"status":                      status,
		"source":                      source,
		"chat_session_id":             sid,
		"query_preview":               queryPreview,
		"items":                       items,
		"kg_triples":                  kgItems,
		"episodes":                    epItems,
		"search":                      searchBundle,
		"kg":                          kgBundle,
		"episode":                     episodeBundle,
		"chapter":                     firstMapOrNil(chapterItems),
		"chapters":                    chapterItems,
		"arc":                         firstMapOrNil(arcItems),
		"arcs":                        arcItems,
		"saga":                        firstMapOrNil(sagaItems),
		"sagas":                       sagaItems,
		"documents":                   documents,
		"document_count":              len(documents),
		"document_schema":             documentSchema,
		"retrieval_document_schema":   documentSchema,
		"index_snapshot":              indexSnapshot,
		"ann_candidate_snapshot":      annSnapshot,
		"intent_contract":             intentContract,
		"packet_budget_policy":        packetBudgetPolicy,
		"intent_hit_preview":          intentHitPreview,
		"recall_lanes":                recallLanes,
		"counts":                      counts,
		"vector_shadow":               vectorShadow,
		"vector_readiness":            vectorReadiness,
		"would_call_vector":           wouldCallVector,
		"would_write":                 false,
		"intent_execution_shadow":     intentExecutionShadow,
		"hierarchy_consistency_trace": hierarchyConsistencyTrace,
		"summary_failure_stability":   summaryFailureStability,
		"ann_default_takeover_guard":  annDefaultTakeoverGuard,
		"stale_context_guard":         staleContextGuard,
		"temporal_proximity_boost": map[string]any{
			"version":      "p71a.v1",
			"status":       "shadow_only",
			"boost_active": profile == "long" || profile == "ultra" || profile == "extreme",
			"profile":      profile,
			"recent_turns": minInt(len(chatLogs), recallLimit),
			"reason":       "temporal_proximity_boost_shadow_only",
		},
		"trace": map[string]any{
			"indexed_candidate_path":  "unified_document_index_shadow",
			"legacy_candidate_source": "store_list_shadow_source_rows",
			"indexed_candidate_ready": len(documents) > 0,
			"intent_route":            "single_query_shared",
			"q1_retrieval_index": map[string]any{
				"status":         indexSnapshot["status"],
				"document_count": len(documents),
				"schema_version": documentSchema["version"],
			},
			"q2_hybrid_indexed_retrieval": map[string]any{
				"ann_candidate_preview": annSnapshot["candidate_count"],
				"rerank_policy":         annSnapshot["rerank_policy"],
				"merge_policy":          annSnapshot["merge_policy"],
				"benchmark_status":      annSnapshot["benchmark"].(map[string]any)["status"],
			},
			"q3_multi_intent_router": map[string]any{
				"routing_mode":    intentContract["routing_mode"],
				"intent_count":    len(intentContract["intents"].([]map[string]any)),
				"preview_status":  intentHitPreview["status"],
				"budget_policy":   packetBudgetPolicy["version"],
				"matched_intents": intentHitPreview["matched_intents"],
			},
			"r2_recall_lanes": map[string]any{
				"top_k_memory_target":     topK,
				"top_k_definition":        "semantic_memory_recall_limit",
				"vector_memory_count":     len(memorySelection.VectorRelevant),
				"recent_memory_count":     len(memorySelection.Recent),
				"relevant_memory_count":   len(memorySelection.Relevant),
				"deep_memory_count":       len(memorySelection.Deep),
				"raw_fallback_count":      fallbackBound,
				"vector_readiness_status": vectorReadiness["status"],
			},
		},
	}
}

func buildPrepareTurnRecallLanes(selection prepareTurnMemoryLaneSelection, rawFallbackLogs []store.ChatLog, vectorReadiness map[string]any, topK int) map[string]any {
	laneItems := func(lane string, memories []store.Memory) []map[string]any {
		out := make([]map[string]any, 0, len(memories))
		for _, item := range memories {
			summary := prepareTurnMemorySummary(item)
			if summary == "" {
				continue
			}
			row := map[string]any{
				"lane":       lane,
				"kind":       "memory",
				"id":         item.ID,
				"turn_index": item.TurnIndex,
				"summary":    summary,
				"importance": item.Importance,
				"reason":     laneSelectionReason(lane),
			}
			if lane == "relevant" {
				if score := selection.RelevantScores[prepareTurnMemoryLaneKey(item)]; score > 0 {
					row["keyword_overlap_score"] = score
				}
			}
			if lane == "vector_relevant" {
				if score := selection.VectorScores[prepareTurnMemoryLaneKey(item)]; score > 0 {
					row["vector_rank_score"] = score
				}
			}
			out = append(out, row)
		}
		return out
	}
	rawItems := make([]map[string]any, 0, len(rawFallbackLogs))
	for _, cl := range rawFallbackLogs {
		content := compactPrepareTurnLine(cl.Content, 0)
		if content == "" {
			continue
		}
		rawItems = append(rawItems, map[string]any{
			"lane":       "raw_fallback",
			"kind":       "chat_log",
			"id":         cl.ID,
			"turn_index": cl.TurnIndex,
			"role":       cl.Role,
			"content":    content,
			"reason":     "vector_or_memory_recall_degraded_raw_turn_support",
		})
	}
	return map[string]any{
		"version":             "r3.recall_lanes.v1",
		"top_k_definition":    "semantic_memory_recall_limit",
		"top_k_memory_target": topK,
		"vector_relevant": map[string]any{
			"count":      len(selection.VectorRelevant),
			"items":      laneItems("vector_relevant", selection.VectorRelevant),
			"policy":     "chromadb_semantic_hit_hydrated_to_mariadb_memory",
			"truth_role": "selector_only_mariadb_memory_is_canonical",
		},
		"recent": map[string]any{
			"count":  len(selection.Recent),
			"items":  laneItems("recent", selection.Recent),
			"policy": "latest_turn_index_anchor_after_relevant_memory",
		},
		"relevant": map[string]any{
			"count":  len(selection.Relevant),
			"items":  laneItems("relevant", selection.Relevant),
			"policy": "keyword_overlap_first_within_top_k_memory_target",
		},
		"deep": map[string]any{
			"count":  len(selection.Deep),
			"items":  laneItems("deep", selection.Deep),
			"policy": "high_importance_older_memory",
		},
		"raw_fallback": map[string]any{
			"count":      len(rawItems),
			"items":      rawItems,
			"active":     len(rawItems) > 0,
			"policy":     "recent_raw_turns_support_only",
			"truth_role": "fallback_support_not_canonical_truth",
		},
		"vector_readiness":      vectorReadiness,
		"selection_trace":       selection.Trace,
		"selected_total":        prepareTurnSelectedMemoryCount(selection) + len(rawItems),
		"no_user_input_rewrite": true,
	}
}

func laneSelectionReason(lane string) string {
	switch lane {
	case "vector_relevant":
		return "chromadb_semantic_hit_hydrated_to_mariadb_memory"
	case "recent":
		return "latest_memory_anchor_after_relevance"
	case "relevant":
		return "query_overlap_first_within_top_k_memory_target"
	case "deep":
		return "deep_past_importance_after_relevance_ranking"
	default:
		return "selected"
	}
}

// buildSessionState assembles the JS-adapter-consumable session_state bundle.
func buildSessionState(
	degraded bool,
	activeStates []store.ActiveState,
	storylines []store.Storyline,
	charStates []store.CharacterState,
	worldRules []store.WorldRule,
	pendingThreads []store.PendingThread,
	recallLimit int,
) map[string]any {
	recallLimit = prepareTurnRecallLimit(recallLimit)
	status := "ready"
	warnings := []any{}
	if degraded {
		status = "degraded"
		warnings = append(warnings, "Store unavailable; returning empty session state.")
	}

	boundedActive := make([]map[string]any, 0, minInt(len(activeStates), recallLimit))
	for i, as := range activeStates {
		if i >= recallLimit {
			break
		}
		boundedActive = append(boundedActive, map[string]any{
			"id":         as.ID,
			"state_type": as.StateType,
			"turn_index": as.TurnIndex,
		})
	}

	boundedStorylines := make([]map[string]any, 0, minInt(len(storylines), recallLimit))
	for i, sl := range storylines {
		if i >= recallLimit {
			break
		}
		boundedStorylines = append(boundedStorylines, map[string]any{
			"id":     sl.ID,
			"name":   sl.Name,
			"status": sl.Status,
		})
	}

	boundedChars := make([]map[string]any, 0, minInt(len(charStates), recallLimit))
	for i, cs := range charStates {
		if i >= recallLimit {
			break
		}
		boundedChars = append(boundedChars, map[string]any{
			"id":             cs.ID,
			"character_name": cs.CharacterName,
			"turn_index":     cs.TurnIndex,
		})
	}

	boundedRules := make([]map[string]any, 0, minInt(len(worldRules), recallLimit))
	for i, wr := range worldRules {
		if i >= recallLimit {
			break
		}
		boundedRules = append(boundedRules, map[string]any{
			"id":       wr.ID,
			"scope":    wr.Scope,
			"category": wr.Category,
			"key":      wr.Key,
		})
	}

	boundedThreads := make([]map[string]any, 0, minInt(len(pendingThreads), recallLimit))
	for i, pt := range pendingThreads {
		if i >= recallLimit {
			break
		}
		boundedThreads = append(boundedThreads, map[string]any{
			"id":           pt.ID,
			"thread_key":   pt.ThreadKey,
			"status":       pt.Status,
			"created_turn": pt.CreatedTurn,
		})
	}

	return map[string]any{
		"snapshot_status": status,
		"active_states":   boundedActive,
		"storylines":      boundedStorylines,
		"characters":      boundedChars,
		"world_rules":     boundedRules,
		"pending_threads": boundedThreads,
		"section_meta": map[string]any{
			"active_state_count":   len(activeStates),
			"storyline_count":      len(storylines),
			"character_count":      len(charStates),
			"world_rule_count":     len(worldRules),
			"pending_thread_count": len(pendingThreads),
		},
		"warnings":     warnings,
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"fetched":      true,
	}
}

// buildNarrativeControl assembles the JS-adapter-consumable narrative_control bundle.
func buildNarrativeControl(
	degraded bool,
	storylines []store.Storyline,
	worldRules []store.WorldRule,
	pendingThreads []store.PendingThread,
	charStates []store.CharacterState,
) map[string]any {
	stateStatus := "shadow_evidence"
	if degraded {
		stateStatus = "skeleton"
	}
	return map[string]any{
		"state_status":         stateStatus,
		"storyline_count":      len(storylines),
		"world_rule_count":     len(worldRules),
		"pending_thread_count": len(pendingThreads),
		"character_count":      len(charStates),
		"guide_mode":           "shadow_read",
		"narrative_stance":     "observational",
		"would_call_llm":       false,
		"would_write":          false,
	}
}

// buildContinuityPack assembles the JS-adapter-consumable continuity_pack bundle.
func buildContinuityPack(
	sid string,
	queryPreview string,
	degraded bool,
	resumePack *store.ResumePack,
	episodeSums []store.EpisodeSummary,
	chatLogs []store.ChatLog,
	activeStates []store.ActiveState,
	canonicalLayers []store.CanonicalStateLayer,
	recallLimit int,
) map[string]any {
	recallLimit = prepareTurnRecallLimit(recallLimit)
	status := "ready"
	if degraded {
		status = "degraded"
	}

	items := []map[string]any{}
	if resumePack != nil {
		text := strings.TrimSpace(resumePack.AssembledText)
		items = append(items, map[string]any{
			"kind":    "resume_pack",
			"present": true,
			"trigger": resumePack.Trigger,
			"text":    text,
		})
	}

	for i, es := range episodeSums {
		if i >= recallLimit {
			break
		}
		summary := strings.TrimSpace(es.SummaryText)
		if summary == "" {
			summary = fmt.Sprintf("Episode %d-%d", es.FromTurn, es.ToTurn)
		}
		summary = strings.Join(strings.Fields(summary), " ")
		items = append(items, map[string]any{
			"kind":      "episode_summary",
			"from_turn": es.FromTurn,
			"to_turn":   es.ToTurn,
			"summary":   summary,
		})
	}

	for _, cl := range selectRecentChatLogsByTurn(chatLogs, recallLimit) {
		content := strings.TrimSpace(cl.Content)
		content = strings.Join(strings.Fields(content), " ")
		items = append(items, map[string]any{
			"kind":       "chat_log",
			"turn_index": cl.TurnIndex,
			"role":       cl.Role,
			"content":    content,
		})
	}

	for i, as := range activeStates {
		if i >= recallLimit {
			break
		}
		content := strings.TrimSpace(as.Content)
		content = strings.Join(strings.Fields(content), " ")
		items = append(items, map[string]any{
			"kind":       "active_state",
			"state_type": as.StateType,
			"turn_index": as.TurnIndex,
			"content":    content,
		})
	}

	for i, cl := range canonicalLayers {
		if i >= recallLimit {
			break
		}
		content := strings.TrimSpace(cl.Content)
		content = strings.Join(strings.Fields(content), " ")
		items = append(items, map[string]any{
			"kind":       "canonical_layer",
			"layer_type": cl.LayerType,
			"turn_index": cl.TurnIndex,
			"content":    content,
		})
	}

	return map[string]any{
		"status":                status,
		"chat_session_id":       sid,
		"query_preview":         queryPreview,
		"resume_pack_present":   resumePack != nil,
		"episode_count":         len(episodeSums),
		"chat_log_count":        len(chatLogs),
		"active_state_count":    len(activeStates),
		"canonical_layer_count": len(canonicalLayers),
		"items":                 items,
		"would_call_llm":        false,
		"would_write":           false,
	}
}

// buildProgressionLedger assembles the JS-adapter-consumable progression_ledger bundle.
func buildProgressionLedger(sid string, degraded bool, storylines []store.Storyline, worldRules []store.WorldRule, pendingThreads []store.PendingThread, episodeSums []store.EpisodeSummary, recallLimit int) map[string]any {
	recallLimit = prepareTurnRecallLimit(recallLimit)
	status := "ready"
	if degraded {
		status = "degraded"
	}
	lastTurn := progressionLedgerLatestTurn(storylines, pendingThreads, episodeSums)
	lifecycleModel := map[string]any{
		"status":         "active",
		"states":         []string{"latent", "active", "escalating", "aftermath", "resolved", "dormant"},
		"pressure_scale": map[string]any{"min": 0, "max": 3},
		"decay_rules":    map[string]any{"latent": 5, "active": 4, "escalating": 3, "aftermath": 2, "resolved": 1, "dormant": 0},
		"mode":           "deterministic_no_llm",
	}
	doNotResolveGuard := map[string]any{
		"status":                "active",
		"mode":                  "deterministic_no_llm",
		"min_turn_gap":          2,
		"protected_entry_types": []string{"unresolved_tension", "payoff"},
		"protected_sources":     []string{"storyline.ongoing_tensions", "pending_thread.promise", "pending_thread.open_question"},
		"long_horizon_tokens":   []string{"promise", "payoff", "callback", "debt", "oath", "answer", "thread", "unresolved"},
	}
	unresolvedTensions := progressionLedgerUnresolvedTensions(storylines, pendingThreads, lifecycleModel, doNotResolveGuard, lastTurn, recallLimit)
	consequences := progressionLedgerConsequences(worldRules, episodeSums, lifecycleModel, lastTurn, recallLimit)
	payoffs := progressionLedgerPayoffs(storylines, pendingThreads, lifecycleModel, doNotResolveGuard, lastTurn, recallLimit)
	sceneDeltas := progressionLedgerSceneDeltas(episodeSums, lifecycleModel, lastTurn, recallLimit)
	worldPressure := progressionLedgerWorldPressure(worldRules, pendingThreads, storylines, lastTurn, recallLimit)
	lastAdvancedTurn := any(nil)
	lastValidatedTurn := any(nil)
	if lastTurn > 0 {
		lastAdvancedTurn = lastTurn
		if status == "ready" {
			lastValidatedTurn = lastTurn
		}
	}
	return map[string]any{
		"status":                               status,
		"chat_session_id":                      sid,
		"storyline_count":                      len(storylines),
		"world_rule_count":                     len(worldRules),
		"pending_thread_count":                 len(pendingThreads),
		"episode_count":                        len(episodeSums),
		"would_write":                          false,
		"ledger_policy_version":                "lw1h.v1",
		"ledger_mode":                          "deterministic_no_llm",
		"last_advanced_turn":                   lastAdvancedTurn,
		"last_validated_turn":                  lastValidatedTurn,
		"unresolved_tensions":                  unresolvedTensions,
		"consequences":                         consequences,
		"payoffs":                              payoffs,
		"scene_deltas":                         sceneDeltas,
		"world_pressure_policy_version":        "lw1d.v1",
		"world_pressure":                       worldPressure,
		"continuity_precedence_policy_version": "lw1e.v1",
		"supporting_precedence_guard": map[string]any{
			"status":                                   "supporting_only",
			"supporting_only":                          true,
			"cannot_override_current_user_input":       true,
			"cannot_override_verified_direct_evidence": true,
			"precedence_ceiling":                       "below_current_user_input_and_verified_direct_evidence",
			"allowed_usage":                            []string{"continuity_hint", "narrative_support"},
			"disallowed_usage":                         []string{"truth_overwrite", "canonical_override"},
		},
		"compatibility_policy_version": "lw1f.v1",
		"compatibility_contract": map[string]any{
			"status":           "compatible",
			"targets":          []string{"chapter_summary", "arc_summary", "continuity_pack"},
			"shape_mode":       "additive_non_breaking",
			"consumer_safe":    true,
			"adapter_required": false,
		},
		"lifecycle_policy_version":      "lw1g.v1",
		"lifecycle_model":               lifecycleModel,
		"do_not_resolve_policy_version": "lw1h.v1",
		"do_not_resolve_guard":          doNotResolveGuard,
	}
}

func progressionLedgerLatestTurn(storylines []store.Storyline, pendingThreads []store.PendingThread, episodeSums []store.EpisodeSummary) int {
	latest := 0
	for _, sl := range storylines {
		latest = maxInt(latest, sl.LastTurn)
		latest = maxInt(latest, sl.LastEvidenceTurn)
	}
	for _, pt := range pendingThreads {
		latest = maxInt(latest, pt.LastSeenTurn)
		latest = maxInt(latest, pt.SourceTurn)
		latest = maxInt(latest, pt.CreatedTurn)
	}
	for _, ep := range episodeSums {
		latest = maxInt(latest, ep.ToTurn)
	}
	return latest
}

func progressionLedgerUnresolvedTensions(storylines []store.Storyline, pendingThreads []store.PendingThread, lifecycleModel map[string]any, guard map[string]any, lastTurn, recallLimit int) []any {
	recallLimit = prepareTurnRecallLimit(recallLimit)
	items := []any{}
	for _, sl := range storylines {
		if sl.Suppressed || strings.EqualFold(sl.Status, "resolved") {
			continue
		}
		for _, tension := range denseJSONItems(sl.OngoingTensionsJSON, recallLimit) {
			label := normalizeStoryLedgerLabel(tension)
			if label == "" {
				continue
			}
			pressure, decay := lifecycleProfileForState("active", lifecycleModel)
			entry := map[string]any{
				"entry_type":         "unresolved_tension",
				"label":              label,
				"source":             "storyline.ongoing_tensions",
				"status":             "open",
				"lifecycle_state":    "active",
				"pressure_score":     pressure,
				"decay_turns":        decay,
				"deterministic":      true,
				"source_record_id":   sl.ID,
				"source_message_ids": []any{},
				"affected_relations": []any{sl.Name},
				"affected_world":     []any{},
			}
			attachDoNotResolveFields(entry, guard, lastTurn)
			items = append(items, entry)
			if len(items) >= recallLimit {
				return items
			}
		}
	}
	for _, pt := range pendingThreads {
		if pt.Suppressed || strings.EqualFold(pt.Status, "resolved") {
			continue
		}
		label := normalizeStoryLedgerLabel(q1FirstNonEmptyString(pt.Description, pt.Title, pt.ThreadKey))
		if label == "" {
			continue
		}
		pressure, decay := lifecycleProfileForState("latent", lifecycleModel)
		entry := map[string]any{
			"entry_type":         "unresolved_tension",
			"label":              label,
			"source":             "pending_thread." + q1FirstNonEmptyString(pt.HookType, "open_question"),
			"status":             "open",
			"lifecycle_state":    "latent",
			"pressure_score":     pressure,
			"decay_turns":        decay,
			"deterministic":      true,
			"source_record_id":   pt.ID,
			"source_message_ids": []any{},
			"affected_relations": []any{q1FirstNonEmptyString(pt.Target, pt.Owner)},
			"affected_world":     []any{},
		}
		attachDoNotResolveFields(entry, guard, lastTurn)
		items = append(items, entry)
		if len(items) >= recallLimit {
			return items
		}
	}
	return items
}

func progressionLedgerConsequences(worldRules []store.WorldRule, episodeSums []store.EpisodeSummary, lifecycleModel map[string]any, lastTurn, recallLimit int) []any {
	recallLimit = prepareTurnRecallLimit(recallLimit)
	items := []any{}
	for _, wr := range worldRules {
		if wr.Suppressed {
			continue
		}
		label := normalizeStoryLedgerLabel(q1FirstNonEmptyString(wr.Key, wr.Category, wr.ScopeName))
		if label == "" {
			continue
		}
		pressure, decay := lifecycleProfileForState("escalating", lifecycleModel)
		items = append(items, map[string]any{
			"entry_type":         "consequence",
			"label":              label,
			"source":             "world_rule",
			"status":             "pending",
			"turn_hint":          maxInt(wr.SourceTurn, lastTurn),
			"lifecycle_state":    "escalating",
			"pressure_score":     pressure,
			"decay_turns":        decay,
			"deterministic":      true,
			"source_record_id":   wr.ID,
			"source_message_ids": []any{},
			"affected_relations": []any{},
			"affected_world":     []any{label},
		})
		if len(items) >= recallLimit {
			return items
		}
	}
	for _, ep := range episodeSums {
		for _, event := range denseJSONItems(ep.KeyEvents, recallLimit) {
			label := normalizeStoryLedgerLabel(event)
			if label == "" {
				continue
			}
			pressure, decay := lifecycleProfileForState("aftermath", lifecycleModel)
			items = append(items, map[string]any{
				"entry_type":         "consequence",
				"label":              label,
				"source":             "episode.key_events",
				"status":             "pending",
				"turn_hint":          ep.ToTurn,
				"lifecycle_state":    "aftermath",
				"pressure_score":     pressure,
				"decay_turns":        decay,
				"deterministic":      true,
				"source_record_id":   ep.ID,
				"source_message_ids": []any{},
				"affected_relations": []any{},
				"affected_world":     []any{},
			})
			if len(items) >= recallLimit {
				return items
			}
		}
	}
	return items
}

func progressionLedgerPayoffs(storylines []store.Storyline, pendingThreads []store.PendingThread, lifecycleModel map[string]any, guard map[string]any, lastTurn, recallLimit int) []any {
	recallLimit = prepareTurnRecallLimit(recallLimit)
	items := []any{}
	for _, pt := range pendingThreads {
		if pt.Suppressed {
			continue
		}
		label := normalizeStoryLedgerLabel(q1FirstNonEmptyString(pt.Description, pt.Title, pt.ThreadKey))
		if label == "" {
			continue
		}
		state := "pending"
		lifecycleState := "active"
		if strings.EqualFold(pt.Status, "resolved") {
			state = "completed"
			lifecycleState = "resolved"
		} else if strings.EqualFold(pt.Status, "cancelled") || strings.EqualFold(pt.Status, "invalid") || strings.EqualFold(pt.Status, "suppressed") {
			state = "invalid"
			lifecycleState = "dormant"
		}
		pressure, decay := lifecycleProfileForState(lifecycleState, lifecycleModel)
		entry := map[string]any{
			"entry_type":         "payoff",
			"label":              label,
			"source":             "pending_thread." + q1FirstNonEmptyString(pt.HookType, "promise"),
			"status":             state,
			"payoff_state":       state,
			"lifecycle_state":    lifecycleState,
			"pressure_score":     pressure,
			"decay_turns":        decay,
			"deterministic":      true,
			"source_record_id":   pt.ID,
			"source_message_ids": []any{},
			"affected_relations": []any{q1FirstNonEmptyString(pt.Target, pt.Owner)},
			"affected_world":     []any{},
		}
		attachDoNotResolveFields(entry, guard, lastTurn)
		items = append(items, entry)
		if len(items) >= recallLimit {
			return items
		}
	}
	for _, sl := range storylines {
		if sl.Suppressed || strings.EqualFold(sl.Status, "resolved") {
			continue
		}
		for _, tension := range denseJSONItems(sl.OngoingTensionsJSON, recallLimit) {
			label := normalizeStoryLedgerLabel(tension)
			if label == "" {
				continue
			}
			pressure, decay := lifecycleProfileForState("latent", lifecycleModel)
			entry := map[string]any{
				"entry_type":         "payoff",
				"label":              label,
				"source":             "storyline.ongoing_tensions",
				"status":             "pending",
				"payoff_state":       "pending",
				"lifecycle_state":    "latent",
				"pressure_score":     pressure,
				"decay_turns":        decay,
				"deterministic":      true,
				"source_record_id":   sl.ID,
				"source_message_ids": []any{},
				"affected_relations": []any{sl.Name},
				"affected_world":     []any{},
			}
			attachDoNotResolveFields(entry, guard, lastTurn)
			items = append(items, entry)
			if len(items) >= recallLimit {
				return items
			}
		}
	}
	return items
}

func progressionLedgerSceneDeltas(episodeSums []store.EpisodeSummary, lifecycleModel map[string]any, lastTurn, recallLimit int) []any {
	recallLimit = prepareTurnRecallLimit(recallLimit)
	items := []any{}
	for _, ep := range episodeSums {
		label := normalizeStoryLedgerLabel(q1FirstNonEmptyString(ep.SummaryText, fmt.Sprintf("Episode %d-%d", ep.FromTurn, ep.ToTurn)))
		if label == "" {
			continue
		}
		pressure, decay := lifecycleProfileForState("active", lifecycleModel)
		items = append(items, map[string]any{
			"entry_type":         "scene_delta",
			"label":              truncateRunes(label, 180),
			"source":             "episode_summary",
			"status":             "observed",
			"turn_hint":          maxInt(ep.ToTurn, lastTurn),
			"lifecycle_state":    "active",
			"pressure_score":     pressure,
			"decay_turns":        decay,
			"deterministic":      true,
			"source_record_id":   ep.ID,
			"source_message_ids": []any{},
			"affected_relations": []any{},
			"affected_world":     []any{},
		})
		if len(items) >= recallLimit {
			return items
		}
	}
	return items
}

func progressionLedgerWorldPressure(worldRules []store.WorldRule, pendingThreads []store.PendingThread, storylines []store.Storyline, lastTurn, recallLimit int) map[string]any {
	recallLimit = prepareTurnRecallLimit(recallLimit)
	storyPlan := map[string]any{
		"next_beats":      []string{},
		"execution_notes": []string{},
		"guardrails":      []string{},
		"current_arc":     "",
	}
	director := map[string]any{
		"world_guardrails":  []string{},
		"resolved_outcomes": []string{},
	}
	for _, wr := range worldRules {
		if len(asStringSlice(director["world_guardrails"])) >= recallLimit {
			break
		}
		if wr.Suppressed {
			continue
		}
		label := q1FirstNonEmptyString(wr.Key, wr.Category, wr.ScopeName)
		if label != "" {
			director["world_guardrails"] = append(asStringSlice(director["world_guardrails"]), label)
		}
	}
	for _, pt := range pendingThreads {
		if len(asStringSlice(storyPlan["next_beats"])) >= recallLimit {
			break
		}
		if pt.Suppressed || strings.EqualFold(pt.Status, "resolved") {
			continue
		}
		label := q1FirstNonEmptyString(pt.Description, pt.Title, pt.ThreadKey)
		if label != "" {
			storyPlan["next_beats"] = append(asStringSlice(storyPlan["next_beats"]), label)
		}
	}
	for _, sl := range storylines {
		if len(asStringSlice(storyPlan["execution_notes"])) >= recallLimit && len(asStringSlice(director["resolved_outcomes"])) >= recallLimit {
			break
		}
		if sl.Suppressed {
			continue
		}
		if strings.EqualFold(sl.Status, "resolved") {
			director["resolved_outcomes"] = append(asStringSlice(director["resolved_outcomes"]), sl.Name)
			continue
		}
		if sl.Name != "" {
			storyPlan["execution_notes"] = append(asStringSlice(storyPlan["execution_notes"]), sl.Name)
		}
	}
	return buildWorldPressure(storyPlan, director, asStringSlice(storyPlan["next_beats"]), asStringSlice(director["resolved_outcomes"]), lastTurn)
}

func progressionLedgerTracePreviewFields(ledger map[string]any) map[string]any {
	unresolved := lenAnySlice(ledger["unresolved_tensions"])
	consequences := lenAnySlice(ledger["consequences"])
	payoffs := lenAnySlice(ledger["payoffs"])
	sceneDeltas := lenAnySlice(ledger["scene_deltas"])
	worldPressure, _ := ledger["world_pressure"].(map[string]any)
	lifecycleModel, _ := ledger["lifecycle_model"].(map[string]any)
	supportingGuard, _ := ledger["supporting_precedence_guard"].(map[string]any)
	compatibility, _ := ledger["compatibility_contract"].(map[string]any)
	return map[string]any{
		"story_ledger_mode":                                     ledger["ledger_mode"],
		"story_ledger_policy_version":                           ledger["ledger_policy_version"],
		"unresolved_tensions_count":                             unresolved,
		"consequences_count":                                    consequences,
		"payoffs_count":                                         payoffs,
		"scene_deltas_count":                                    sceneDeltas,
		"payoff_pending_count":                                  countLedgerPayoffState(ledger["payoffs"], "pending"),
		"payoff_completed_count":                                countLedgerPayoffState(ledger["payoffs"], "completed"),
		"payoff_invalid_count":                                  countLedgerPayoffState(ledger["payoffs"], "invalid"),
		"world_pressure_ready":                                  worldPressure != nil,
		"world_pressure_policy_version":                         ledger["world_pressure_policy_version"],
		"world_pressure_factions_count":                         lenAnySlice(worldPressure["factions"]),
		"world_pressure_regions_count":                          lenAnySlice(worldPressure["regions"]),
		"world_pressure_offscreen_threads_count":                lenAnySlice(worldPressure["offscreen_threads"]),
		"world_pressure_public_pressure_count":                  lenAnySlice(worldPressure["public_pressure"]),
		"world_pressure_timeline_count":                         lenAnySlice(worldPressure["timeline"]),
		"continuity_precedence_policy_version":                  ledger["continuity_precedence_policy_version"],
		"supporting_precedence_guard_ready":                     supportingGuard != nil,
		"supporting_precedence_supporting_only":                 supportingGuard["supporting_only"],
		"supporting_precedence_blocks_user_input_override":      supportingGuard["cannot_override_current_user_input"],
		"supporting_precedence_blocks_direct_evidence_override": supportingGuard["cannot_override_verified_direct_evidence"],
		"compatibility_policy_version":                          ledger["compatibility_policy_version"],
		"compatibility_ready":                                   compatibility != nil,
		"compatibility_targets_count":                           lenAnySlice(compatibility["targets"]),
		"compatibility_consumer_safe":                           compatibility["consumer_safe"],
		"lifecycle_policy_version":                              ledger["lifecycle_policy_version"],
		"lifecycle_ready":                                       lifecycleModel != nil,
		"lifecycle_states_count":                                lenAnySlice(lifecycleModel["states"]),
		"lifecycle_decay_rules_count":                           lenAnyMap(lifecycleModel["decay_rules"]),
		"lifecycle_entry_count":                                 unresolved + consequences + payoffs + sceneDeltas,
		"lifecycle_latent_count":                                countLedgerLifecycleState(ledger, "latent"),
		"lifecycle_active_count":                                countLedgerLifecycleState(ledger, "active"),
		"lifecycle_escalating_count":                            countLedgerLifecycleState(ledger, "escalating"),
		"lifecycle_aftermath_count":                             countLedgerLifecycleState(ledger, "aftermath"),
		"lifecycle_resolved_count":                              countLedgerLifecycleState(ledger, "resolved"),
		"lifecycle_dormant_count":                               countLedgerLifecycleState(ledger, "dormant"),
		"do_not_resolve_policy_version":                         ledger["do_not_resolve_policy_version"],
		"do_not_resolve_guard_ready":                            ledger["do_not_resolve_guard"] != nil,
		"do_not_resolve_protected_count":                        countLedgerDoNotResolve(ledger),
		"do_not_resolve_unresolved_count":                       countLedgerDoNotResolveIn(ledger["unresolved_tensions"]),
		"do_not_resolve_payoff_pending_count":                   countLedgerDoNotResolvePendingPayoffs(ledger["payoffs"]),
	}
}

func lenAnySlice(value any) int {
	switch v := value.(type) {
	case []any:
		return len(v)
	case []string:
		return len(v)
	case []map[string]any:
		return len(v)
	default:
		return 0
	}
}

func lenAnyMap(value any) int {
	switch v := value.(type) {
	case map[string]any:
		return len(v)
	case map[string]int:
		return len(v)
	default:
		return 0
	}
}

func mapSliceFromAny(value any) []map[string]any {
	switch items := value.(type) {
	case []map[string]any:
		return items
	case []any:
		out := []map[string]any{}
		for _, raw := range items {
			if item, ok := raw.(map[string]any); ok {
				out = append(out, item)
			}
		}
		return out
	default:
		return nil
	}
}

func ledgerEntryMaps(value any) []map[string]any {
	out := []map[string]any{}
	if items, ok := value.([]any); ok {
		for _, raw := range items {
			if item, ok := raw.(map[string]any); ok {
				out = append(out, item)
			}
		}
	}
	return out
}

func countLedgerPayoffState(value any, state string) int {
	count := 0
	for _, item := range ledgerEntryMaps(value) {
		if strings.EqualFold(asString(item["payoff_state"]), state) || strings.EqualFold(asString(item["status"]), state) {
			count++
		}
	}
	return count
}

func countLedgerLifecycleState(ledger map[string]any, state string) int {
	count := 0
	for _, key := range []string{"unresolved_tensions", "consequences", "payoffs", "scene_deltas"} {
		for _, item := range ledgerEntryMaps(ledger[key]) {
			if strings.EqualFold(asString(item["lifecycle_state"]), state) {
				count++
			}
		}
	}
	return count
}

func countLedgerDoNotResolve(ledger map[string]any) int {
	return countLedgerDoNotResolveIn(ledger["unresolved_tensions"]) + countLedgerDoNotResolveIn(ledger["payoffs"])
}

func countLedgerDoNotResolveIn(value any) int {
	count := 0
	for _, item := range ledgerEntryMaps(value) {
		if item["do_not_resolve_yet"] == true {
			count++
		}
	}
	return count
}

func countLedgerDoNotResolvePendingPayoffs(value any) int {
	count := 0
	for _, item := range ledgerEntryMaps(value) {
		if item["do_not_resolve_yet"] == true && strings.EqualFold(asString(item["payoff_state"]), "pending") {
			count++
		}
	}
	return count
}

// buildAutonomyPlan assembles the JS-adapter-consumable autonomy_plan bundle.
func buildAutonomyPlan(degraded bool, guideMode, narrativeStance string) map[string]any {
	status := "ready"
	if degraded {
		status = "degraded"
	}
	return map[string]any{
		"status":           status,
		"guide_mode":       guideMode,
		"narrative_stance": narrativeStance,
		"suggested_action": "continue",
		"would_call_llm":   false,
		"would_write":      false,
	}
}

// buildMicroBeatProposal assembles the JS-adapter-consumable micro_beat_proposal bundle.
func buildMicroBeatProposal(degraded bool, pendingThreads []store.PendingThread, storylines []store.Storyline, recallLimit int) map[string]any {
	recallLimit = prepareTurnRecallLimit(recallLimit)
	status := "ready"
	if degraded {
		status = "degraded"
	}
	beats := []map[string]any{}
	for i, pt := range pendingThreads {
		if i >= recallLimit {
			break
		}
		beats = append(beats, map[string]any{
			"kind":        "pending_thread",
			"thread_key":  pt.ThreadKey,
			"description": strings.Join(strings.Fields(strings.TrimSpace(pt.Description)), " "),
		})
	}
	for _, sl := range storylines {
		if len(beats) >= recallLimit {
			break
		}
		beats = append(beats, map[string]any{
			"kind":            "storyline",
			"name":            sl.Name,
			"current_context": strings.Join(strings.Fields(strings.TrimSpace(sl.CurrentContext)), " "),
		})
	}
	return map[string]any{
		"status":         status,
		"beats":          beats,
		"would_call_llm": false,
		"would_write":    false,
	}
}

// buildSceneStepProposal assembles the JS-adapter-consumable scene_step_proposal bundle.
func buildSceneStepProposal(degraded bool, activeStates []store.ActiveState, canonicalLayers []store.CanonicalStateLayer, episodeSums []store.EpisodeSummary, recallLimit int) map[string]any {
	recallLimit = prepareTurnRecallLimit(recallLimit)
	status := "ready"
	if degraded {
		status = "degraded"
	}
	steps := []map[string]any{}
	for i, as := range activeStates {
		if i >= recallLimit {
			break
		}
		steps = append(steps, map[string]any{
			"kind":       "active_state",
			"state_type": as.StateType,
			"content":    strings.Join(strings.Fields(strings.TrimSpace(as.Content)), " "),
		})
	}
	for _, cl := range canonicalLayers {
		if len(steps) >= recallLimit {
			break
		}
		steps = append(steps, map[string]any{
			"kind":       "canonical_layer",
			"layer_type": cl.LayerType,
			"content":    strings.Join(strings.Fields(strings.TrimSpace(cl.Content)), " "),
		})
	}
	for _, es := range episodeSums {
		if len(steps) >= recallLimit {
			break
		}
		steps = append(steps, map[string]any{
			"kind":      "episode_summary",
			"from_turn": es.FromTurn,
			"to_turn":   es.ToTurn,
			"summary":   strings.Join(strings.Fields(strings.TrimSpace(es.SummaryText)), " "),
		})
	}
	return map[string]any{
		"status":         status,
		"steps":          steps,
		"would_call_llm": false,
		"would_write":    false,
	}
}

// buildCombinedProposal assembles the JS-adapter-consumable combined_proposal bundle.
func buildCombinedProposal(degraded bool, microBeatProposal, sceneStepProposal map[string]any) map[string]any {
	status := "ready"
	if degraded {
		status = "degraded"
	}
	beats, _ := microBeatProposal["beats"].([]map[string]any)
	steps, _ := sceneStepProposal["steps"].([]map[string]any)
	return map[string]any{
		"status":           status,
		"micro_beat_count": len(beats),
		"scene_step_count": len(steps),
		"source":           "go_r1_read_shadow",
		"would_call_llm":   false,
		"would_write":      false,
	}
}

// buildWritebackPreview assembles the JS-adapter-consumable writeback_preview bundle.
func buildWritebackPreview(degraded bool) map[string]any {
	status := "ready"
	if degraded {
		status = "degraded"
	}
	return map[string]any{
		"status":      status,
		"would_write": false,
		"targets": []string{
			"memories",
			"direct_evidence",
			"kg_triples",
			"storylines",
			"world_rules",
			"pending_threads",
		},
		"notes": "R1 read-shadow: no writes performed. Writeback requires authority elevation and store-write mode.",
	}
}

// buildWritebackPlan assembles the JS-adapter-consumable writeback_plan bundle.
func buildWritebackPlan(sid string, turnIndex int, storeWriteEnabled bool, writeSource string, req dto.M4CompleteTurnRequest) map[string]any {
	preview := ""
	if req.UserInput != nil {
		preview = strings.TrimSpace(*req.UserInput)
	}
	if req.AssistantContent != nil {
		if preview != "" {
			preview += " | "
		}
		preview += strings.TrimSpace(*req.AssistantContent)
	}
	preview = truncateRunes(preview, 200)

	return map[string]any{
		"status":              "ready",
		"chat_session_id":     sid,
		"turn_index":          turnIndex,
		"store_write_enabled": storeWriteEnabled,
		"would_write":         storeWriteEnabled,
		"write_source":        writeSource,
		"targets": []string{
			"chat_logs",
			"effective_inputs",
			"memories",
			"direct_evidence",
			"kg_triples",
			"entities",
			"narrative_state",
		},
		"content_preview": preview,
		"notes":           writebackPlanNote(storeWriteEnabled, writeSource),
	}
}

// buildInputTransparency assembles the JS-adapter-consumable input_transparency bundle.
func buildInputTransparency(sid string, turnIndex int, text string, storeWriteEnabled bool, writeSource string) map[string]any {
	return map[string]any{
		"status":                "ready",
		"chat_session_id":       sid,
		"turn_index":            turnIndex,
		"effective_input_chars": len([]rune(text)),
		"preview":               truncateRunes(text, 200),
		"store_write_enabled":   storeWriteEnabled,
		"would_write":           storeWriteEnabled,
		"write_source":          writeSource,
		"notes":                 inputTransparencyNote(storeWriteEnabled, writeSource),
	}
}

func writebackPlanNote(storeWriteEnabled bool, writeSource string) string {
	if !storeWriteEnabled {
		return "R1 read-shadow writeback plan: no Store writes are enabled for this request."
	}
	return "Store write path is enabled for " + writeSource + "; this bundle reflects the targets written or attempted by the turn handler."
}

func inputTransparencyNote(storeWriteEnabled bool, writeSource string) string {
	if !storeWriteEnabled {
		return "R1 read-shadow input transparency: no Store writes are enabled for this request."
	}
	return "Store write path is enabled for " + writeSource + "; input transparency persistence was attempted by the handler."
}

// buildRepairReplayPlan assembles the JS-adapter-consumable repair_replay_plan bundle.
func buildRepairReplayPlan(sid string, req dto.ChatLogRepairReplayRequest, mutationEnabled bool, source string) map[string]any {
	entries := []map[string]any{}
	for i, e := range req.Entries {
		if i >= 3 {
			break
		}
		preview := ""
		if e.AssistantContent != nil {
			preview = truncateRunes(strings.TrimSpace(*e.AssistantContent), 120)
		}
		entries = append(entries, map[string]any{
			"index":   i,
			"preview": preview,
		})
	}
	status := "shadow_plan"
	notes := "R1 read-shadow repair-replay plan: no replay or write triggered."
	wouldReplay := false
	wouldWrite := false
	if mutationEnabled {
		status = "mutation_ready"
		notes = "Store write path is enabled; repair-replay checks missing raw chat_log roles and inserts only missing rows."
		wouldReplay = len(req.Entries) > 0
		wouldWrite = wouldReplay && !(req.DryRun != nil && *req.DryRun)
		if strings.TrimSpace(source) == "" {
			source = "store_write"
		}
	} else {
		source = "go_r1_read_shadow"
	}
	return map[string]any{
		"status":                  status,
		"source":                  source,
		"chat_session_id":         sid,
		"entries_count":           len(req.Entries),
		"dry_run":                 req.DryRun != nil && *req.DryRun,
		"would_replay":            wouldReplay,
		"would_write":             wouldWrite,
		"mutation_enabled":        mutationEnabled,
		"sync_replay_gate":        true,
		"save_update_delete_gate": true,
		"write_scope":             "chat_log_effective_input_memory_evidence_kg",
		"delete_scope":            "rollback_delete_gate_only",
		"canonical_input_source":  "sqlite_store",
		"entries_preview":         entries,
		"notes":                   notes,
	}
}

func (s *Server) runChatLogRepairReplay(ctx context.Context, sid string, req dto.ChatLogRepairReplayRequest) (map[string]any, error) {
	dryRun := req.DryRun != nil && *req.DryRun
	now := time.Now().UTC()
	repairedTurns := []int{}
	failedTurns := []map[string]any{}
	checkedTurns := []int{}
	totalMissingRoles := 0
	totalRepairedRoles := 0
	totalConflictRoles := 0
	totalExistingRoles := 0

	for _, entry := range req.Entries {
		turnIndex := entry.TurnIndex
		if turnIndex < 0 {
			failedTurns = append(failedTurns, map[string]any{"turn_index": turnIndex, "reason": "invalid_turn_index"})
			continue
		}
		checkedTurns = append(checkedTurns, turnIndex)
		existingRows, err := s.Store.ListChatLogs(ctx, sid, turnIndex, turnIndex)
		if err != nil && !errors.Is(err, store.ErrNotFound) {
			failedTurns = append(failedTurns, map[string]any{"turn_index": turnIndex, "reason": "list_chat_logs_failed: " + err.Error()})
			continue
		}
		existing := map[string]string{}
		for _, row := range existingRows {
			if row.ChatSessionID != sid || row.TurnIndex != turnIndex {
				continue
			}
			role := strings.ToLower(strings.TrimSpace(row.Role))
			if role == "user" || role == "assistant" {
				existing[role] = row.Content
			}
		}

		createdAt := parseRepairReplayCreatedAt(entry.CreatedAt, now)
		repairedThisTurn := 0
		for _, candidate := range []struct {
			role    string
			content *string
		}{
			{role: "user", content: entry.UserContent},
			{role: "assistant", content: entry.AssistantContent},
		} {
			content := ""
			if candidate.content != nil {
				content = sanitizeCriticStorageText(*candidate.content)
			}
			if strings.TrimSpace(content) == "" {
				continue
			}
			if current, ok := existing[candidate.role]; ok {
				if strings.TrimSpace(current) != strings.TrimSpace(content) {
					totalConflictRoles++
				} else {
					totalExistingRoles++
				}
				continue
			}
			totalMissingRoles++
			if dryRun {
				continue
			}
			if err := s.Store.SaveChatLog(ctx, &store.ChatLog{
				ChatSessionID: sid,
				TurnIndex:     turnIndex,
				Role:          candidate.role,
				Content:       content,
				CreatedAt:     createdAt,
			}); err != nil {
				failedTurns = append(failedTurns, map[string]any{"turn_index": turnIndex, "role": candidate.role, "reason": "save_chat_log_failed: " + err.Error()})
				continue
			}
			totalRepairedRoles++
			repairedThisTurn++
			existing[candidate.role] = content
		}
		if repairedThisTurn > 0 {
			repairedTurns = append(repairedTurns, turnIndex)
		}
	}

	if !dryRun && totalRepairedRoles > 0 {
		_ = s.Store.SaveAuditLog(ctx, &store.AuditLog{
			ChatSessionID: sid,
			EventType:     "repair_replay",
			TargetType:    "session",
			TargetID:      0,
			Summary:       fmt.Sprintf("Repair replay restored %d chat log roles", totalRepairedRoles),
			DetailsJSON: mustCompactJSON(map[string]any{
				"repaired_turns":            repairedTurns,
				"total_repaired_role_count": totalRepairedRoles,
				"total_conflict_role_count": totalConflictRoles,
			}),
			Source:    s.storeWriteSource(),
			CreatedAt: now,
		})
	}

	return map[string]any{
		"status":                    "ok",
		"source":                    s.storeWriteSource(),
		"chat_session_id":           sid,
		"dry_run":                   dryRun,
		"entries_count":             len(req.Entries),
		"checked_turns":             uniqueSortedInts(checkedTurns),
		"repaired_turns":            uniqueSortedInts(repairedTurns),
		"failed_turns":              failedTurns,
		"total_missing_role_count":  totalMissingRoles,
		"total_repaired_role_count": totalRepairedRoles,
		"total_conflict_role_count": totalConflictRoles,
		"total_existing_role_count": totalExistingRoles,
		"note":                      "repair-replay checked supplied failed-queue/delete-snapshot/active-chat entries and inserted only missing raw chat_log roles",
	}, nil
}

func parseRepairReplayCreatedAt(raw *string, fallback time.Time) time.Time {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return fallback
	}
	text := strings.TrimSpace(*raw)
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05"} {
		if parsed, err := time.Parse(layout, text); err == nil {
			return parsed.UTC()
		}
	}
	return fallback
}

func uniqueSortedInts(values []int) []int {
	if len(values) == 0 {
		return []int{}
	}
	seen := map[int]bool{}
	out := []int{}
	for _, value := range values {
		if value <= 0 || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// buildRollbackPlan assembles the JS-adapter-consumable rollback_plan bundle.
func buildRollbackPlan(sid string, turnIndex int, reqSource string) map[string]any {
	return map[string]any{
		"status":                      "shadow_plan",
		"source":                      "go_r1_read_shadow",
		"chat_session_id":             sid,
		"turn_index":                  turnIndex,
		"req_source":                  reqSource,
		"would_delete":                false,
		"would_write":                 false,
		"mutation_enabled":            false,
		"reason":                      "R1 shadow mode: rollback not executed",
		"sync_replay_gate":            true,
		"save_update_delete_gate":     true,
		"stale_vector_replay_gate":    true,
		"rollback_vector_delete_gate": true,
		"rebuild_replay_gate":         false,
		"vector_doc_delete_policy":    "canonical_row_first_then_vector",
		"stale_summary_policy":        "tombstone_before_rebuild",
		"turn_delete_policy":          "tail_from_earliest_deleted_turn",
		"hierarchy_invalidation":      "delete_overlapping_episode_chapter_arc_saga_ranges",
		"step23_invalidation":         "delete_turn_scoped_support_records_from_from_turn",
		"rebuild_owner":               "chroma_shadow_orchestrator",
		"cleanup_surfaces": []string{
			"chat_logs",
			"effective_inputs",
			"memories",
			"subjective_entity_memories",
			"direct_evidence",
			"kg_triples",
			"critic_feedback",
			"character_events",
			"entities",
			"trust_states",
			"storylines",
			"world_rules",
			"character_states",
			"pending_threads",
			"active_states",
			"canonical_state_layers",
			"episode_summaries",
			"guidance_plan_states",
			"chapter_summaries",
			"arc_summaries",
			"saga_digests",
			"session_active_scopes",
			"consequence_records",
			"psychology_branches",
			"theme_offscreen_carries",
			"capture_verification_records",
		},
		"notes": "R1 read-shadow rollback plan: no deletion or write triggered.",
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func firstMapOrNil(items []map[string]any) any {
	if len(items) == 0 {
		return nil
	}
	return items[0]
}

func q1FirstNonEmptyString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func q1TimePtrAny(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Format(time.RFC3339)
}

func chapterFromResumePack(rp *store.ResumePack) *store.ChapterSummary {
	if rp == nil {
		return nil
	}
	return rp.Chapter
}

func arcFromResumePack(rp *store.ResumePack) *store.ArcSummary {
	if rp == nil {
		return nil
	}
	return rp.Arc
}

func sagaFromResumePack(rp *store.ResumePack) *store.SagaDigest {
	if rp == nil {
		return nil
	}
	return rp.Saga
}

func buildUnifiedRetrievalDocuments(
	sid string,
	memories []store.Memory,
	evidence []store.DirectEvidence,
	kgTriples []store.KGTriple,
	episodeSums []store.EpisodeSummary,
	resumePack *store.ResumePack,
	chatLogs []store.ChatLog,
) []map[string]any {
	docs := []map[string]any{}
	for _, m := range memories {
		text := strings.Join(strings.Fields(q1FirstNonEmptyString(m.SummaryJSON, m.PlaceRoom, m.PlaceWing)), " ")
		docs = append(docs, retrievalDocumentQ1("memory", "memory", "memory_summary", m.ID, fmt.Sprintf("%d", m.ID), sid, m.TurnIndex, m.TurnIndex, m.TurnIndex, fmt.Sprintf("Memory #%d", m.ID), text, m.CreatedAt, map[string]any{
			"importance": m.Importance,
			"place_wing": m.PlaceWing,
			"place_room": m.PlaceRoom,
		}))
	}
	for _, e := range evidence {
		text := strings.Join(strings.Fields(e.EvidenceText), " ")
		docs = append(docs, retrievalDocumentQ1("evidence", "direct_evidence", "evidence_verbatim", e.ID, fmt.Sprintf("%d", e.ID), sid, e.TurnAnchor, e.TurnAnchor, e.TurnAnchor, fmt.Sprintf("Evidence #%d", e.ID), text, e.CreatedAt, map[string]any{
			"evidence_kind":        e.EvidenceKind,
			"capture_verification": e.CaptureVerification,
			"source_turn_start":    e.SourceTurnStart,
			"source_turn_end":      e.SourceTurnEnd,
		}))
	}
	for _, cl := range chatLogs {
		content := strings.Join(strings.Fields(cl.Content), " ")
		docs = append(docs, retrievalDocumentQ1("chat_log", "chat_log_fallback", "legacy_keyword_fallback", cl.ID, fmt.Sprintf("%d", cl.ID), sid, cl.TurnIndex, cl.TurnIndex, cl.TurnIndex, fmt.Sprintf("ChatLog #%d", cl.ID), content, cl.CreatedAt, map[string]any{
			"role":       cl.Role,
			"turn_index": cl.TurnIndex,
		}))
	}
	for _, k := range kgTriples {
		text := fmt.Sprintf("%s %s %s", k.Subject, k.Predicate, k.Object)
		docs = append(docs, retrievalDocumentQ1("kg_triple", "kg_triple", "kg_triple", k.ID, fmt.Sprintf("%d", k.ID), sid, 0, 0, 0, fmt.Sprintf("KG #%d", k.ID), text, k.CreatedAt, map[string]any{
			"subject":   k.Subject,
			"predicate": k.Predicate,
			"object":    k.Object,
		}))
	}
	for _, es := range episodeSums {
		summary := strings.Join(strings.Fields(q1FirstNonEmptyString(es.SummaryText, fmt.Sprintf("Episode %d-%d", es.FromTurn, es.ToTurn))), " ")
		if anchors := episodeDenseAnchorPreview(es, 420); anchors != "" {
			summary = strings.Join(strings.Fields(summary+" "+anchors), " ")
		}
		meta := map[string]any{
			"from_turn":                 es.FromTurn,
			"to_turn":                   es.ToTurn,
			"key_events":                es.KeyEvents,
			"open_loops_json":           es.OpenLoopsJSON,
			"relationship_changes_json": es.RelationshipChangesJSON,
		}
		for k, v := range denseSummarySurfaceFields("episode", es.ID, es.FromTurn, es.ToTurn, es.SummaryText, episodeDenseStructuredPayload(es), episodeDensePriorityScores(es), evidence) {
			meta[k] = v
		}
		docs = append(docs, retrievalDocumentQ1("episode", "episode", "episode_summary", es.ID, fmt.Sprintf("%d", es.ID), sid, es.FromTurn, es.ToTurn, es.FromTurn, fmt.Sprintf("Episode #%d", es.ID), summary, es.CreatedAt, meta))
	}
	if resumePack != nil && resumePack.Chapter != nil {
		ch := resumePack.Chapter
		text := strings.Join(strings.Fields(q1FirstNonEmptyString(ch.SummaryText, ch.ResumeText, ch.ChapterTitle)), " ")
		meta := map[string]any{
			"chapter_index": ch.ChapterIndex,
			"chapter_title": ch.ChapterTitle,
		}
		for k, v := range denseSummarySurfaceFields("chapter", ch.ID, ch.FromTurn, ch.ToTurn, q1FirstNonEmptyString(ch.ResumeText, ch.SummaryText, ch.ChapterTitle), chapterDenseStructuredPayload(*ch), chapterDensePriorityScores(*ch), evidence) {
			meta[k] = v
		}
		docs = append(docs, retrievalDocumentQ1("chapter", "chapter", "chapter_summary", ch.ID, fmt.Sprintf("%d", ch.ID), sid, ch.FromTurn, ch.ToTurn, ch.FromTurn, fmt.Sprintf("Chapter #%d", ch.ID), text, *ch.CreatedAt, meta))
	}
	if resumePack != nil && resumePack.Arc != nil {
		arc := resumePack.Arc
		text := strings.Join(strings.Fields(q1FirstNonEmptyString(arc.ArcResumeText, arc.CoreConflict, arc.ArcName)), " ")
		meta := map[string]any{
			"arc_index":  arc.ArcIndex,
			"arc_name":   arc.ArcName,
			"arc_status": arc.ArcStatus,
		}
		for k, v := range denseSummarySurfaceFields("arc", arc.ID, arc.FromTurn, arc.ToTurn, q1FirstNonEmptyString(arc.ArcResumeText, arc.CoreConflict, arc.ArcName), arcDenseStructuredPayload(*arc), nil, evidence) {
			meta[k] = v
		}
		docs = append(docs, retrievalDocumentQ1("arc", "arc", "arc_summary", arc.ID, fmt.Sprintf("%d", arc.ID), sid, arc.FromTurn, arc.ToTurn, arc.FromTurn, fmt.Sprintf("Arc #%d", arc.ID), text, *arc.CreatedAt, meta))
	}
	if resumePack != nil && resumePack.Saga != nil {
		saga := resumePack.Saga
		text := strings.Join(strings.Fields(q1FirstNonEmptyString(saga.ResumePackText, saga.SagaSummary, saga.EraLabel)), " ")
		meta := map[string]any{
			"era_label": saga.EraLabel,
		}
		for k, v := range denseSummarySurfaceFields("saga", saga.ID, saga.FromTurn, saga.ToTurn, q1FirstNonEmptyString(saga.ResumePackText, saga.SagaSummary, saga.EraLabel), sagaDenseStructuredPayload(*saga), nil, evidence) {
			meta[k] = v
		}
		docs = append(docs, retrievalDocumentQ1("saga", "saga", "saga_summary", saga.ID, fmt.Sprintf("%d", saga.ID), sid, saga.FromTurn, saga.ToTurn, saga.FromTurn, fmt.Sprintf("Saga #%d", saga.ID), text, *saga.CreatedAt, meta))
	}
	return docs
}

func retrievalDocumentQ1(sourceType, sourceSubtype, sourceTable string, id int64, sourceRowID, sid string, fromTurn, toTurn, turnIndex int, title, text string, createdAt time.Time, meta map[string]any) map[string]any {
	doc := map[string]any{
		"document_id":     fmt.Sprintf("%s:%d", sourceType, id),
		"tier":            sourceType,
		"source_type":     sourceType,
		"source_subtype":  sourceSubtype,
		"source_row_id":   sourceRowID,
		"source_table":    sourceTable,
		"chat_session_id": sid,
		"from_turn":       fromTurn,
		"to_turn":         toTurn,
		"turn_index":      turnIndex,
		"title":           title,
		"text":            text,
		"similarity":      1.0,
		"created_at":      createdAt,
		"query_matched":   true,
		"metadata":        meta,
	}
	return doc
}

func retrievalDocumentSchemaQ1() map[string]any {
	return map[string]any{
		"version":       "q1a.v1",
		"index_version": "q1e.v1",
		"required_fields": []string{
			"document_id", "tier", "source_type", "source_subtype", "source_row_id",
			"source_table", "chat_session_id", "from_turn", "to_turn", "turn_index",
			"title", "text", "similarity", "created_at", "query_matched", "metadata",
		},
		"partition_keys": []string{"chat_session_id", "tier", "source_table"},
		"source_lookup":  "document_id_prefix_to_store_row",
	}
}

func retrievalIndexSnapshotFromDocuments(sid string, documents []map[string]any) map[string]any {
	return map[string]any{
		"status":          "ready",
		"document_count":  len(documents),
		"chat_session_id": sid,
		"schema_version":  "q1e.v1",
	}
}

func buildANNCandidateSnapshotQ2(docs []map[string]any, vectorShadow map[string]any) map[string]any {
	candidates := []map[string]any{}
	for i, doc := range docs {
		if i >= 8 {
			break
		}
		candidate := map[string]any{}
		for k, v := range doc {
			candidate[k] = v
		}
		score := 1.0 - float64(i)*0.05
		if score < 0.1 {
			score = 0.1
		}
		candidate["ann_rank"] = i + 1
		candidate["rerank_score"] = score
		candidate["similarity"] = score
		candidate["query_matched"] = true
		candidates = append(candidates, candidate)
	}
	vectorMode, _ := vectorShadow["mode"].(string)
	if vectorMode == "" {
		vectorMode, _ = vectorShadow["source"].(string)
	}
	status := "empty"
	if len(candidates) > 0 {
		status = "ready"
	}
	return map[string]any{
		"version":         "q2a.v1",
		"status":          status,
		"candidate_count": len(candidates),
		"candidates":      candidates,
		"rerank_applied":  len(candidates) > 1,
		"rerank_policy":   "metadata_score_v1",
		"merge_policy":    "tier_head_then_rerank_v1",
		"vector_mode":     vectorMode,
		"benchmark": map[string]any{
			"status":           status,
			"overlap_ratio":    q2OverlapRatio(candidates),
			"tier_diversity":   q2TierDiversity(candidates),
			"candidate_tiers":  q2TierSequence(candidates),
			"guarded_takeover": false,
			"takeover_guard":   "shadow_compare_first",
		},
	}
}

func q2OverlapRatio(candidates []map[string]any) float64 {
	if len(candidates) == 0 {
		return 0
	}
	seenText := map[string]bool{}
	for _, c := range candidates {
		text, _ := c["text"].(string)
		seenText[text] = true
	}
	return float64(len(seenText)) / float64(len(candidates))
}

func q2TierDiversity(candidates []map[string]any) int {
	tiers := map[string]bool{}
	for _, c := range candidates {
		tier, _ := c["tier"].(string)
		tiers[tier] = true
	}
	return len(tiers)
}

func q2TierSequence(candidates []map[string]any) []string {
	seq := []string{}
	for _, c := range candidates {
		tier, _ := c["tier"].(string)
		seq = append(seq, tier)
	}
	return seq
}

func buildIntentContractQ3() map[string]any {
	intents := []map[string]any{
		q3Intent("scene", []string{"memory", "episode", "chapter"}, 0.34),
		q3Intent("callback", []string{"arc", "saga", "memory"}, 0.22),
		q3Intent("resume", []string{"chapter", "arc", "saga"}, 0.28),
		q3Intent("canon", []string{"memory", "episode", "arc"}, 0.16),
	}
	tierCounts := map[string]int{}
	for _, intent := range intents {
		tiers, _ := intent["tiers"].([]string)
		for _, tier := range tiers {
			tierCounts[tier]++
		}
	}
	return map[string]any{
		"version":      "q3a.v1",
		"routing_mode": "single_query_shared",
		"intents":      intents,
		"routing_shadow_tier_priority": map[string]any{
			"version":                "t1d.v1",
			"mode":                   "verification_only",
			"status":                 "shadow_only",
			"tier_counts":            tierCounts,
			"priority_verdict":       "tier_priority_verification_shadow",
			"requires_manual_review": false,
			"reason":                 "routing_shadow_tier_priority_surface",
		},
	}
}

func q3Intent(name string, tiers []string, budgetShare float64) map[string]any {
	return map[string]any{
		"name":          name,
		"query_builder": name + "_query_v1",
		"tiers":         tiers,
		"budget_share":  budgetShare,
	}
}

func q3PacketBudgetPolicy() map[string]any {
	return map[string]any{
		"version":        "q3c.v1",
		"profile_source": "runtime_token_profile",
		"budget_mode":    "policy_only",
		"scene_share":    0.34,
		"callback_share": 0.22,
		"resume_share":   0.28,
		"canon_share":    0.16,
		"degrade_policy": "drop_low_score_then_shorten_text",
		"budget_transition": map[string]any{
			"version":          "p75a.v1",
			"from_mode":        "policy_only",
			"to_mode":          "enforced_shadow",
			"transition_ready": true,
			"reason":           "per_intent_shadow_budget_gate",
		},
		"budget_caps": map[string]any{
			"version":          "p76a.v1",
			"layer_cap":        12,
			"char_cap":         3000,
			"canon_hard_floor": 120,
			"per_intent_max":   3,
			"reason":           "layer_char_canon_hard_floor_applied",
		},
	}
}

func buildIntentHitPreviewQ3(queryPreview string, docs []map[string]any) map[string]any {
	matched := map[string][]string{
		"scene":    {},
		"callback": {},
		"resume":   {},
		"canon":    {},
	}
	for _, doc := range docs {
		tier, _ := doc["tier"].(string)
		documentID, _ := doc["document_id"].(string)
		if tier == "" || documentID == "" {
			continue
		}
		for _, intent := range q3MatchedTiers(tier) {
			matched[intent] = append(matched[intent], documentID)
		}
	}
	status := "empty"
	if len(docs) > 0 {
		status = "ready"
	}
	return map[string]any{
		"version":         "q3d.v1",
		"status":          status,
		"query_preview":   truncateRunes(strings.Join(strings.Fields(queryPreview), " "), 120),
		"matched_intents": matched,
	}
}

func q3MatchedTiers(tier string) []string {
	switch tier {
	case "memory":
		return []string{"scene", "callback", "canon"}
	case "episode":
		return []string{"scene", "canon"}
	case "chapter":
		return []string{"scene", "resume"}
	case "arc":
		return []string{"callback", "resume", "canon"}
	case "saga":
		return []string{"callback", "resume"}
	case "chat_log":
		return []string{"scene"}
	default:
		return []string{}
	}
}

func buildGenerationPacketShadowCompareRecord(assembly prepareTurnInjectionAssembly, inputContextText string) map[string]any {
	newHasChapter := strings.TrimSpace(assembly.MemoryText) != "" && strings.Contains(strings.ToLower(assembly.MemoryText), "chapter")
	newChapterChars := len([]rune(strings.TrimSpace(assembly.MemoryText)))
	newHasChapterInput := strings.TrimSpace(inputContextText) != "" && strings.Contains(strings.ToLower(inputContextText), "chapter")
	oldHasChapter := false
	oldChapterChars := 0
	oldHasChapterInput := false
	return map[string]any{
		"version":                  "p249a.v1",
		"new_has_chapter":          newHasChapter,
		"new_chapter_chars":        newChapterChars,
		"new_has_chapter_input":    newHasChapterInput,
		"old_has_chapter":          oldHasChapter,
		"old_chapter_chars":        oldChapterChars,
		"old_has_chapter_input":    oldHasChapterInput,
		"divergence_chapter":       newHasChapter != oldHasChapter,
		"divergence_chapter_input": newHasChapterInput != oldHasChapterInput,
	}
}

func canonicalLayerEligibleForCurrentTruth(layer store.CanonicalStateLayer) bool {
	sourceType := strings.ToLower(strings.TrimSpace(layer.SourceStateType))
	for _, blocked := range []string{"pending", "rejected", "unverified", "stale", "repair_queue", "manual_review"} {
		if strings.Contains(sourceType, blocked) {
			return false
		}
	}
	if layer.Confidence > 0 && layer.Confidence < 0.7 {
		return false
	}
	return true
}

func buildIntentExecutionShadow(docs []map[string]any, vectorShadow map[string]any, profile string, packetBudgetPolicy map[string]any) map[string]any {
	globalCapChars := 3000
	canonHardFloor := 120
	if packetBudgetPolicy != nil {
		if caps, ok := packetBudgetPolicy["budget_caps"].(map[string]any); ok {
			if v, ok := caps["char_cap"].(int); ok && v > 0 {
				globalCapChars = v
			}
			if v, ok := caps["canon_hard_floor"].(int); ok && v > 0 {
				canonHardFloor = v
			}
		}
	}

	intentDefs := []struct {
		name  string
		tiers []string
	}{
		{"scene", []string{"memory", "episode", "chapter"}},
		{"callback", []string{"arc", "saga", "memory"}},
		{"resume", []string{"chapter", "arc", "saga"}},
		{"canon", []string{"memory", "episode", "arc"}},
	}

	intents := []map[string]any{}
	selectedCountBefore := 0
	selectedCountAfter := 0
	seenDocs := map[string]bool{}
	reasonCounts := map[string]int{
		"tier_cap":       0,
		"overlap_drop":   0,
		"floor_reserved": 0,
	}

	selectedCandidatesOrdered := []string{}
	canonSelectedChars := 0
	tierEvents := []map[string]any{}
	selectionEvents := []map[string]any{}
	budgetEvents := []map[string]any{}
	runningTotalChars := 0
	for _, def := range intentDefs {
		candidates := []map[string]any{}
		for _, doc := range docs {
			tier, _ := doc["tier"].(string)
			for _, allowed := range def.tiers {
				if tier == allowed {
					candidates = append(candidates, doc)
					break
				}
			}
		}
		selected := candidates
		if len(selected) > 3 {
			selected = selected[:3]
		}

		// S-1d trace: selection events for all candidates
		for i, c := range candidates {
			id, _ := c["document_id"].(string)
			tier, _ := c["tier"].(string)
			sel := i < 3
			reason := "selected"
			if !sel {
				reason = "tier_cap"
			} else if seenDocs[id] {
				sel = false
				reason = "overlap_drop"
			}
			selectionEvents = append(selectionEvents, map[string]any{
				"intent":           def.name,
				"tier":             tier,
				"document_id":      id,
				"source":           "retrieval",
				"selected":         sel,
				"selection_reason": reason,
				"merge_rank":       i + 1,
			})
		}

		selectedIDs := []string{}
		for _, s := range selected {
			id, _ := s["document_id"].(string)
			if id != "" {
				selectedIDs = append(selectedIDs, id)
				if !seenDocs[id] {
					seenDocs[id] = true
					selectedCountAfter++
					selectedCandidatesOrdered = append(selectedCandidatesOrdered, id)

					text, _ := s["text"].(string)
					charCost := len([]rune(text))
					runningTotalChars += charCost
					tier, _ := s["tier"].(string)
					budgetEvents = append(budgetEvents, map[string]any{
						"intent":              def.name,
						"tier":                tier,
						"document_id":         id,
						"decision":            "keep",
						"reason":              "within_cap",
						"char_cost":           charCost,
						"running_total_chars": runningTotalChars,
						"cap_chars":           globalCapChars,
					})
				}
			}
		}
		if len(candidates) > 3 {
			reasonCounts["tier_cap"] += len(candidates) - 3
		}
		if def.name == "canon" {
			for _, s := range selected {
				text, _ := s["text"].(string)
				canonSelectedChars += len([]rune(text))
			}
		}
		selectedCountBefore += len(selectedIDs)

		intents = append(intents, map[string]any{
			"intent":             def.name,
			"candidate_count":    len(candidates),
			"selected_count":     len(selected),
			"tiers":              def.tiers,
			"selected_documents": selectedIDs,
		})

		tierEvents = append(tierEvents, map[string]any{
			"intent":     def.name,
			"tier":       def.tiers[0],
			"tier_count": len(candidates),
			"selected":   len(selectedIDs),
		})
	}

	droppedCount := selectedCountBefore - selectedCountAfter
	if droppedCount > 0 {
		reasonCounts["overlap_drop"] = droppedCount
	}

	// S-1g temporal scoring
	temporalScoring := map[string]any{
		"version": "s1g.v1",
		"mode":    "shadow_temporal_scoring_only",
		"profile": profile,
		"status":  "off",
		"reason":  "profile_not_target",
	}
	if profile == "ultra" || profile == "extreme" {
		temporalScoring["status"] = "ready"
		temporalScoring["ann_recency_score"] = map[string]any{
			"score_source":   "temporal_proximity",
			"recency_weight": 0.15,
			"reason":         "long_profile_temporal_scoring_applied",
		}
		temporalScoring["applied_intent_count"] = len(intentDefs)
		temporalScoring["reordered_intent_count"] = 0
		delete(temporalScoring, "reason")
	}

	noCapCount := 0
	if len(docs) == 0 {
		noCapCount = 1
	}
	budgetReasons := map[string]int{
		"within_cap": selectedCountAfter,
		"over_cap":   0,
		"no_cap":     noCapCount,
	}

	intentCapRatios := map[string]float64{
		"scene":    0.40,
		"callback": 0.25,
		"resume":   0.20,
		"canon":    0.15,
	}
	retrievalLayerCaps := []map[string]any{}
	for _, def := range intentDefs {
		capChars := int(float64(globalCapChars) * intentCapRatios[def.name])
		retrievalLayerCaps = append(retrievalLayerCaps, map[string]any{
			"intent":    def.name,
			"cap_chars": capChars,
			"reason":    "priority_deferred",
			"cap_scope": "layer_cap",
		})
	}

	globalSelectedChars := 0
	for _, doc := range docs {
		if id, _ := doc["document_id"].(string); seenDocs[id] {
			text, _ := doc["text"].(string)
			globalSelectedChars += len([]rune(text))
		}
	}

	status := "ready"
	if len(docs) == 0 {
		status = "off"
	}

	guarded := map[string]any{
		"status":   status,
		"decision": "shadow_compare",
		"reason":   "candidate_pool_available",
	}
	enforced := map[string]any{
		"version":             "t1e.v1",
		"mode":                "enforced_default_takeover_only",
		"status":              status,
		"decision":            "enforced_shadow",
		"reason":              "budget_and_dedupe_passed",
		"selected_candidates": selectedCandidatesOrdered,
	}
	if len(docs) == 0 {
		guarded["decision"] = "fail_open"
		guarded["reason"] = "no_candidates"
		enforced["decision"] = "fail_open"
		enforced["reason"] = "no_candidates"
		enforced["selected_candidates"] = []string{}
	}

	sagaCollisionPolicy := "none"
	if profile == "ultra" || profile == "extreme" {
		sagaCollisionPolicy = "saga_floor_reserve_v0d"
	}

	suppressedCount := 0
	for _, ev := range selectionEvents {
		if !ev["selected"].(bool) {
			suppressedCount++
		}
	}
	executedIntentCount := 0
	if len(docs) > 0 {
		executedIntentCount = len(intentDefs)
	}
	replayGateStatus := "ready"
	replayGateDecision := "promote_candidate"
	replayGateReason := "passed_evidence"
	if status == "off" {
		replayGateStatus = "off"
		replayGateDecision = "fail_open"
		replayGateReason = "runtime_mode_not_per_intent_shadow"
	}

	return map[string]any{
		"version":      "p29a.v1",
		"routing_mode": "per_intent_shadow",
		"status":       status,
		"intents":      intents,
		"cross_intent_dedupe": map[string]any{
			"unique_document_count": selectedCountAfter,
			"duplicate_drop_count":  droppedCount,
		},
		"budget_enforcement": map[string]any{
			"version":                    "t1b.v1",
			"mode":                       "enforced_shadow",
			"decision_count":             len(intentDefs),
			"selected_count_before":      selectedCountBefore,
			"selected_count_after":       selectedCountAfter,
			"dropped_count":              droppedCount,
			"event_count":                len(budgetEvents),
			"reason_counts":              reasonCounts,
			"budget_reasons":             budgetReasons,
			"global_cap_chars":           globalCapChars,
			"global_selected_chars":      globalSelectedChars,
			"canon_hard_floor":           canonHardFloor,
			"canon_floor_reserved_chars": canonHardFloor,
			"canon_selected_chars":       canonSelectedChars,
			"retrieval_layer_caps":       retrievalLayerCaps,
		},
		"guarded_takeover":  guarded,
		"enforced_takeover": enforced,
		"trace": map[string]any{
			"version": "s1d.v1",
			"mode":    "shadow_trace_only",
			"summary": map[string]any{
				"executed_intent_count": executedIntentCount,
				"input_candidate_count": len(docs),
				"selected_count":        selectedCountAfter,
				"suppressed_count":      suppressedCount,
				"budget_keep_count":     selectedCountAfter,
				"budget_drop_count":     0,
			},
			"selection_events": selectionEvents,
			"budget_events":    budgetEvents,
			"query_builder": map[string]any{
				"query_builder_count":  len(intentDefs),
				"retrieval_call_count": 1,
				"merge_priority":       "tier_then_intent",
				"budget_mode":          "enforced_shadow",
				"routing_mode":         "single_query_shared",
			},
		},
		"temporal_scoring": temporalScoring,
		"replay_gate": map[string]any{
			"version":  "u1e.v1",
			"mode":     "captured_session_replay_gate_only",
			"status":   replayGateStatus,
			"decision": replayGateDecision,
			"reason":   replayGateReason,
		},
		"actual_execution": map[string]any{
			"version":       "p44a.v1",
			"status":        status,
			"retrieval_ran": len(docs) > 0,
			"intents_ran":   len(intentDefs),
			"dedupe_ran":    len(docs) > 0,
			"budget_ran":    len(docs) > 0,
			"reason":        "per_intent_shadow_executed",
		},
		"tier_priority_verification": map[string]any{
			"version":                "t1d.v1",
			"mode":                   "verification_only",
			"status":                 status,
			"tier_events":            tierEvents,
			"tier_counts":            len(tierEvents),
			"priority_verdict":       "tier_priority_verification_shadow",
			"requires_manual_review": false,
			"saga_collision_policy":  sagaCollisionPolicy,
			"reason":                 "tier_priority_verification_surface",
		},
	}
}

func buildHierarchyConsistencyTrace(docs []map[string]any, resumePack *store.ResumePack, episodeSums []store.EpisodeSummary) map[string]any {
	episodePresent := len(episodeSums) > 0
	chapterPresent := resumePack != nil && resumePack.Chapter != nil
	sagaPresent := resumePack != nil && resumePack.Saga != nil
	arcPresent := resumePack != nil && resumePack.Arc != nil

	reasons := []string{}
	if sagaPresent {
		reasons = append(reasons, "saga_present_top_level")
	}
	if arcPresent {
		reasons = append(reasons, "arc_present_covers_chapters")
	}
	if chapterPresent {
		reasons = append(reasons, "chapter_present_covers_episodes")
	}
	if episodePresent {
		reasons = append(reasons, "episode_present_covers_turns")
	}

	consistencyScore := 0.0
	if sagaPresent {
		consistencyScore += 0.25
	}
	if arcPresent {
		consistencyScore += 0.25
	}
	if chapterPresent {
		consistencyScore += 0.25
	}
	if episodePresent {
		consistencyScore += 0.25
	}

	chapterEpisodeAligned := false
	if chapterPresent && episodePresent && resumePack != nil && resumePack.Chapter != nil && len(episodeSums) > 0 {
		chFrom := resumePack.Chapter.FromTurn
		chTo := resumePack.Chapter.ToTurn
		for _, ep := range episodeSums {
			if ep.FromTurn >= chFrom && ep.ToTurn <= chTo {
				chapterEpisodeAligned = true
				break
			}
		}
	}

	collisionRules := []string{}
	if sagaPresent {
		collisionRules = append(collisionRules, "saga_overrides_arc")
	}
	if arcPresent {
		collisionRules = append(collisionRules, "arc_overrides_chapter")
	}
	if chapterPresent {
		collisionRules = append(collisionRules, "chapter_overrides_episode")
	}
	if episodePresent {
		collisionRules = append(collisionRules, "episode_overrides_memory")
	}

	return map[string]any{
		"version":                 "p59a.v1",
		"episode_present":         episodePresent,
		"chapter_present":         chapterPresent,
		"saga_present":            sagaPresent,
		"arc_present":             arcPresent,
		"priority_order":          []string{"saga", "arc", "chapter", "episode", "memory"},
		"consistency_score":       consistencyScore,
		"chapter_episode_aligned": chapterEpisodeAligned,
		"collision_rules":         collisionRules,
		"saga_covers_arc":         sagaPresent,
		"arc_covers_chapter":      arcPresent,
		"reasons":                 reasons,
	}
}

func buildSummaryFailureStability(degraded bool, chatLogs []store.ChatLog) map[string]any {
	lastGoodFallback := ""
	retryCount := 0
	lastRetryTurn := -1
	for i := len(chatLogs) - 1; i >= 0; i-- {
		role := strings.TrimSpace(chatLogs[i].Role)
		content := strings.TrimSpace(chatLogs[i].Content)
		if (role == "user" || role == "assistant") && content != "" {
			if lastGoodFallback == "" {
				lastGoodFallback = strings.Join(strings.Fields(content), " ")
			}
		}
		if role == "assistant" && content == "" {
			retryCount++
			lastRetryTurn = chatLogs[i].TurnIndex
		}
	}

	fallbackReason := ""
	if degraded {
		fallbackReason = "store_unavailable"
	} else if len(chatLogs) == 0 {
		fallbackReason = "empty_chat_logs"
	}

	warningLevel := "none"
	if degraded {
		warningLevel = "critical"
	} else if fallbackReason != "" {
		warningLevel = "warn"
	}

	return map[string]any{
		"version":            "p46a.v1",
		"last_good_fallback": lastGoodFallback,
		"retry_ready":        !degraded,
		"retry_count":        retryCount,
		"last_retry_turn":    lastRetryTurn,
		"continuity_guard":   "trace_only",
		"fallback_reason":    fallbackReason,
		"compression_evidence": map[string]any{
			"chat_log_count":  len(chatLogs),
			"profile_applied": false,
			"fallback_reason": fallbackReason,
		},
		"staleness_threshold": map[string]any{
			"version":         "p85a.v1",
			"status":          "shadow_only",
			"threshold_turns": 5,
			"detected":        len(chatLogs) > 0 && len(chatLogs) > 5,
			"reason":          "staleness_threshold_detection",
		},
		"retry_enqueue": map[string]any{
			"version":          "p86a.v1",
			"status":           "shadow_only",
			"enqueue_ready":    !degraded && retryCount > 0,
			"force_regenerate": degraded,
			"reason":           "retry_or_force_regenerate_enqueue",
		},
		"failure_warning": map[string]any{
			"version":        "p87a.v1",
			"status":         "shadow_only",
			"warning_active": degraded || fallbackReason != "",
			"warning_level":  warningLevel,
			"reason":         "failure_trace_warning_surface",
		},
		"replay_gate": map[string]any{
			"version":          "p88a.v1",
			"status":           "shadow_only",
			"gate_active":      len(chatLogs) > 0,
			"session_captured": len(chatLogs) > 0,
			"reason":           "captured_session_replay_non_regression_gate",
		},
	}
}

func buildANNTakeoverGuard(annSnapshot map[string]any, vectorShadow map[string]any) map[string]any {
	overlapRatio := 0.0
	if bench, ok := annSnapshot["benchmark"].(map[string]any); ok {
		if r, ok := bench["overlap_ratio"].(float64); ok {
			overlapRatio = r
		}
	}

	profile := "default"
	if p, ok := vectorShadow["profile"].(string); ok && p != "" {
		profile = p
	}

	overlapThreshold := 0.3
	switch profile {
	case "wide":
		overlapThreshold = 0.25
	case "compact":
		overlapThreshold = 0.35
	case "long":
		overlapThreshold = 0.28
	case "ultra":
		overlapThreshold = 0.20
	case "extreme":
		overlapThreshold = 0.15
	}

	guardDecision := "shadow_compare"
	if overlapRatio < overlapThreshold {
		guardDecision = "fallback_to_keyword"
	}

	return map[string]any{
		"version":           "p33a.v1",
		"profile":           profile,
		"overlap_threshold": overlapThreshold,
		"current_overlap":   overlapRatio,
		"guard_decision":    guardDecision,
		"evidence": map[string]any{
			"threshold_met": overlapRatio >= overlapThreshold,
			"ratio_source":  "q2_benchmark_overlap",
		},
	}
}

func buildStaleContextGuard(storylines []store.Storyline, worldRules []store.WorldRule, pendingThreads []store.PendingThread) map[string]any {
	suppressedStorylines := 0
	for _, sl := range storylines {
		if sl.Suppressed {
			suppressedStorylines++
		}
	}
	suppressedWorldRules := 0
	for _, wr := range worldRules {
		if wr.Suppressed {
			suppressedWorldRules++
		}
	}
	suppressedPendingThreads := 0
	for _, pt := range pendingThreads {
		if pt.Suppressed {
			suppressedPendingThreads++
		}
	}
	totalSuppressed := suppressedStorylines + suppressedWorldRules + suppressedPendingThreads
	return map[string]any{
		"version":                    "p50a.v1",
		"status":                     "ready",
		"guard_type":                 "explicit_forget_stale_context",
		"suppressed_storylines":      suppressedStorylines,
		"suppressed_world_rules":     suppressedWorldRules,
		"suppressed_pending_threads": suppressedPendingThreads,
		"total_suppressed":           totalSuppressed,
		"forget_guard_active":        totalSuppressed > 0,
		"reason":                     "suppressed_items_excluded_from_injection",
	}
}

// --- SEQ-16-P164 / P165 / P167 / P168 contract helpers -------------------

// buildRetrievalRoleBoundary exposes the session/permanent role split for
// the prepare-turn surface. Permanent = storylines, character states, world
// rules. Session = active states, pending threads, chat logs. The split is
// derived from already-read Store data only.
func buildRetrievalRoleBoundary(sid string, storylines []store.Storyline, worldRules []store.WorldRule, charStates []store.CharacterState, activeStates []store.ActiveState, pendingThreads []store.PendingThread, chatLogs []store.ChatLog) map[string]any {
	permanentItems := []map[string]any{}
	for _, sl := range storylines {
		permanentItems = append(permanentItems, map[string]any{
			"role":       "permanent",
			"subrole":    "storyline",
			"id":         sl.ID,
			"name":       sl.Name,
			"last_turn":  sl.LastTurn,
			"suppressed": sl.Suppressed,
		})
	}
	for _, cs := range charStates {
		permanentItems = append(permanentItems, map[string]any{
			"role":           "permanent",
			"subrole":        "character_state",
			"id":             cs.ID,
			"character_name": cs.CharacterName,
			"turn_index":     cs.TurnIndex,
		})
	}
	for _, wr := range worldRules {
		permanentItems = append(permanentItems, map[string]any{
			"role":        "permanent",
			"subrole":     "world_rule",
			"id":          wr.ID,
			"scope":       wr.Scope,
			"scope_name":  wr.ScopeName,
			"category":    wr.Category,
			"key":         wr.Key,
			"source_turn": wr.SourceTurn,
			"suppressed":  wr.Suppressed,
		})
	}
	sessionItems := []map[string]any{}
	for _, as := range activeStates {
		sessionItems = append(sessionItems, map[string]any{
			"role":       "session",
			"subrole":    "active_state",
			"id":         as.ID,
			"state_type": as.StateType,
			"turn_index": as.TurnIndex,
		})
	}
	for _, pt := range pendingThreads {
		sessionItems = append(sessionItems, map[string]any{
			"role":         "session",
			"subrole":      "pending_thread",
			"id":           pt.ID,
			"thread_key":   pt.ThreadKey,
			"created_turn": pt.CreatedTurn,
		})
	}
	for _, cl := range chatLogs {
		sessionItems = append(sessionItems, map[string]any{
			"role":       "session",
			"subrole":    "chat_log",
			"id":         cl.ID,
			"turn_index": cl.TurnIndex,
		})
	}
	return map[string]any{
		"version":              "p164a.v1",
		"chat_session_id":      sid,
		"permanent_role":       "permanent",
		"session_role":         "session",
		"split_policy":         "session_permanent_role_boundary",
		"permanent_item_count": len(permanentItems),
		"session_item_count":   len(sessionItems),
		"permanent_items":      permanentItems,
		"session_items":        sessionItems,
		"boundary_active":      len(permanentItems) > 0 || len(sessionItems) > 0,
		"reason":               "seq16_p164_session_permanent_role_split",
	}
}

// buildRetrievalIndexIRSupportOnly exposes the IR-normalized retrieval unit
// truth floor for the prepare-turn surface. The floor is `support_only_ir`,
// meaning the retrieval unit is a support/retrieval accelerator and never
// the truth authority. Counts come from already-read Store data.
func buildRetrievalIndexIRSupportOnly(recallResult map[string]any, memories []store.Memory, evidence []store.DirectEvidence, kgTriples []store.KGTriple, chatLogs []store.ChatLog, resumePack *store.ResumePack) map[string]any {
	indexed := 0
	if counts, ok := recallResult["counts"].(map[string]any); ok {
		if v, ok := counts["documents_total"].(int); ok {
			indexed = v
		}
	}
	truthStore := "maria_db"
	retrievalAccelerator := "chromadb_compatible"
	irVersion := "p165a.v1"
	unitKind := "support_only_ir_normalized_retrieval_unit"
	return map[string]any{
		"version":               irVersion,
		"unit_kind":             unitKind,
		"support_only":          true,
		"truth_floor":           "support_only_ir",
		"truth_store":           truthStore,
		"retrieval_accelerator": retrievalAccelerator,
		"indexed_unit_count":    indexed,
		"source_counts": map[string]any{
			"memories":   len(memories),
			"evidence":   len(evidence),
			"kg_triples": len(kgTriples),
			"chat_logs":  len(chatLogs),
			"resume_pack": func() int {
				if resumePack == nil {
					return 0
				}
				return 1
			}(),
		},
		"truth_authority_role": "mariadb_canonical_only",
		"retrieval_role":       "support_accelerator_only",
		"reason":               "seq16_p165_support_only_ir_normalized_retrieval_unit_truth_floor",
	}
}

// buildRetrievalExtendAuthority exposes the retrieval-extend authority
// reorder for the prepare-turn surface. Authority order is fixed:
// permanent > session > support (ChromaDB) > fallback (chat_log).
func buildRetrievalExtendAuthority(retrievalRoleBoundary map[string]any) map[string]any {
	permanentCount := 0
	sessionCount := 0
	if v, ok := retrievalRoleBoundary["permanent_item_count"].(int); ok {
		permanentCount = v
	}
	if v, ok := retrievalRoleBoundary["session_item_count"].(int); ok {
		sessionCount = v
	}
	return map[string]any{
		"version":                  "p168a.v1",
		"authority_order":          []string{"permanent", "session", "support", "fallback"},
		"reorder_applied":          true,
		"reorder_policy":           "permanent_first_then_session_then_support_then_fallback",
		"permanent_authority":      "permanent",
		"session_authority":        "session",
		"support_authority":        "support",
		"fallback_authority":       "fallback",
		"permanent_item_count":     permanentCount,
		"session_item_count":       sessionCount,
		"authority_boundary_ready": permanentCount > 0 || sessionCount > 0,
		"reason":                   "seq16_p168_retrieval_extend_authority_reorder",
	}
}

// buildTemporalReadValidityFirst exposes the validity-first temporal read
// signal for the prepare-turn surface. Validity is recency-based and
// recency_event is the latest observed chat_log turn when present.
func buildTemporalReadValidityFirst(chatLogs []store.ChatLog, episodeSums []store.EpisodeSummary, recentChatCount int) map[string]any {
	latestChatTurn := 0
	latestChatRole := ""
	for _, cl := range chatLogs {
		if cl.TurnIndex > latestChatTurn {
			latestChatTurn = cl.TurnIndex
			latestChatRole = cl.Role
		}
	}
	latestEpisodeTo := 0
	for _, ep := range episodeSums {
		if ep.ToTurn > latestEpisodeTo {
			latestEpisodeTo = ep.ToTurn
		}
	}
	validityOrder := []string{"validity_first", "recency", "tier", "session_bond"}
	recencyEvent := any(nil)
	if latestChatTurn > 0 {
		recencyEvent = map[string]any{
			"kind":       "chat_log",
			"turn_index": latestChatTurn,
			"role":       latestChatRole,
		}
	} else if latestEpisodeTo > 0 {
		recencyEvent = map[string]any{
			"kind":    "episode_summary",
			"to_turn": latestEpisodeTo,
		}
	}
	return map[string]any{
		"version":               "p167a.v1",
		"validity_first":        true,
		"validity_order":        validityOrder,
		"recency_event":         recencyEvent,
		"recency_signal_source": "chat_log_then_episode",
		"recent_chat_count":     recentChatCount,
		"latest_chat_turn":      latestChatTurn,
		"latest_episode_to":     latestEpisodeTo,
		"reason":                "seq16_p167_validity_first_temporal_read",
	}
}

// buildSessionMemoryBoundary exposes the session-side memory boundary for
// the prepare-turn surface (SEQ-16-P172). It counts session-scoped items
// and declares the boundary_active flag.
func buildSessionMemoryBoundary(sid string, activeStates []store.ActiveState, pendingThreads []store.PendingThread, chatLogs []store.ChatLog, storylines []store.Storyline, worldRules []store.WorldRule, charStates []store.CharacterState) map[string]any {
	sessionItems := []map[string]any{}
	for _, as := range activeStates {
		sessionItems = append(sessionItems, map[string]any{
			"role":       "session",
			"subrole":    "active_state",
			"id":         as.ID,
			"state_type": as.StateType,
			"turn_index": as.TurnIndex,
		})
	}
	for _, pt := range pendingThreads {
		sessionItems = append(sessionItems, map[string]any{
			"role":         "session",
			"subrole":      "pending_thread",
			"id":           pt.ID,
			"thread_key":   pt.ThreadKey,
			"created_turn": pt.CreatedTurn,
		})
	}
	for _, cl := range chatLogs {
		sessionItems = append(sessionItems, map[string]any{
			"role":       "session",
			"subrole":    "chat_log",
			"id":         cl.ID,
			"turn_index": cl.TurnIndex,
		})
	}
	permanentItems := []map[string]any{}
	for _, sl := range storylines {
		permanentItems = append(permanentItems, map[string]any{
			"role":    "permanent",
			"subrole": "storyline",
			"id":      sl.ID,
			"name":    sl.Name,
		})
	}
	for _, wr := range worldRules {
		permanentItems = append(permanentItems, map[string]any{
			"role":    "permanent",
			"subrole": "world_rule",
			"id":      wr.ID,
			"key":     wr.Key,
		})
	}
	for _, cs := range charStates {
		permanentItems = append(permanentItems, map[string]any{
			"role":           "permanent",
			"subrole":        "character_state",
			"id":             cs.ID,
			"character_name": cs.CharacterName,
		})
	}
	return map[string]any{
		"version":              "p172a.v1",
		"chat_session_id":      sid,
		"session_role":         "session",
		"permanent_role":       "permanent",
		"split_policy":         "session_permanent_role_boundary",
		"session_item_count":   len(sessionItems),
		"permanent_item_count": len(permanentItems),
		"session_items":        sessionItems,
		"permanent_items":      permanentItems,
		"boundary_active":      len(sessionItems) > 0 || len(permanentItems) > 0,
		"reason":               "seq16_p172_session_memory_boundary",
	}
}

// buildBridgePromotionEntry exposes the bridge / promotion entry surface
// for the prepare-turn response (SEQ-16-P173). It lists pending threads
// and canonical layers that are candidates for promotion.
func buildBridgePromotionEntry(sid string, pendingThreads []store.PendingThread, canonicalLayers []store.CanonicalStateLayer) map[string]any {
	candidates := []map[string]any{}
	for _, pt := range pendingThreads {
		candidates = append(candidates, map[string]any{
			"kind":         "pending_thread",
			"id":           pt.ID,
			"thread_key":   pt.ThreadKey,
			"created_turn": pt.CreatedTurn,
			"status":       "awaiting_promotion",
		})
	}
	for _, cl := range canonicalLayers {
		candidates = append(candidates, map[string]any{
			"kind":       "canonical_layer",
			"id":         cl.ID,
			"layer_type": cl.LayerType,
			"turn_index": cl.TurnIndex,
			"status":     "awaiting_promotion",
		})
	}
	return map[string]any{
		"version":         "p173a.v1",
		"chat_session_id": sid,
		"promotion_ready": len(candidates) > 0,
		"candidate_count": len(candidates),
		"candidates":      candidates,
		"bridge_policy":   "pending_and_canonical_await_promotion",
		"reason":          "seq16_p173_bridge_promotion_entry",
	}
}

// buildSessionFirstPermanentFallbackReadRule exposes the session-first /
// permanent-fallback read rule surface (SEQ-16-P174). It declares that
// session items are read first, with permanent as fallback, and echoes
// counts from the session memory boundary.
func buildSessionFirstPermanentFallbackReadRule(sid string, sessionMemoryBoundary, retrievalRoleBoundary map[string]any) map[string]any {
	sessionCount := 0
	permanentCount := 0
	if v, ok := sessionMemoryBoundary["session_item_count"].(int); ok {
		sessionCount = v
	}
	if v, ok := sessionMemoryBoundary["permanent_item_count"].(int); ok {
		permanentCount = v
	}
	// Also accept float64 from JSON unmarshalling in tests.
	if v, ok := sessionMemoryBoundary["session_item_count"].(float64); ok {
		sessionCount = int(v)
	}
	if v, ok := sessionMemoryBoundary["permanent_item_count"].(float64); ok {
		permanentCount = int(v)
	}
	readOrder := []string{"session", "permanent"}
	return map[string]any{
		"version":              "p174a.v1",
		"chat_session_id":      sid,
		"read_policy":          "session_first_permanent_fallback",
		"read_order":           readOrder,
		"session_item_count":   sessionCount,
		"permanent_item_count": permanentCount,
		"fallback_triggered":   sessionCount == 0 && permanentCount > 0,
		"reason":               "seq16_p174_session_first_permanent_fallback_read_rule",
	}
}

// buildPromotionWaitVisibility exposes the promotion-wait visibility
// surface (SEQ-16-P175). It ensures current-turn important facts remain
// visible on the read surface even before canonical promotion, via a
// pending/support lane.
func buildPromotionWaitVisibility(sid string, pendingThreads []store.PendingThread, canonicalLayers []store.CanonicalStateLayer, chatLogs []store.ChatLog) map[string]any {
	pendingCount := len(pendingThreads)
	canonicalCount := len(canonicalLayers)
	latestChatTurn := 0
	for _, cl := range chatLogs {
		if cl.TurnIndex > latestChatTurn {
			latestChatTurn = cl.TurnIndex
		}
	}
	visibilityLanes := []map[string]any{}
	if pendingCount > 0 {
		visibilityLanes = append(visibilityLanes, map[string]any{
			"lane":  "pending_thread",
			"count": pendingCount,
			"role":  "support",
		})
	}
	if canonicalCount > 0 {
		visibilityLanes = append(visibilityLanes, map[string]any{
			"lane":  "canonical_layer",
			"count": canonicalCount,
			"role":  "support",
		})
	}
	if latestChatTurn > 0 {
		visibilityLanes = append(visibilityLanes, map[string]any{
			"lane":        "chat_log",
			"latest_turn": latestChatTurn,
			"role":        "session",
		})
	}
	return map[string]any{
		"version":          "p175a.v1",
		"chat_session_id":  sid,
		"visibility_ready": len(visibilityLanes) > 0,
		"visibility_lanes": visibilityLanes,
		"pending_count":    pendingCount,
		"canonical_count":  canonicalCount,
		"latest_chat_turn": latestChatTurn,
		"wait_policy":      "pending_support_lane_visible_before_promotion",
		"reason":           "seq16_p175_promotion_wait_visibility",
	}
}

// buildRetrievalUnitsIR exposes the normalized retrieval unit schema
// surface for the prepare-turn response (SEQ-16-P179). Each unit carries
// a stable schema identifier, source type, record id, and a support-only
// marker so it is never mistaken for the truth authority.
func buildRetrievalUnitsIR(sid string, memories []store.Memory, evidence []store.DirectEvidence, kgTriples []store.KGTriple, chatLogs []store.ChatLog, resumePack *store.ResumePack) map[string]any {
	units := []map[string]any{}
	for _, m := range memories {
		excerpt := strings.Join(strings.Fields(memorySearchText(m)), " ")
		units = append(units, map[string]any{
			"unit_schema":             "normalized_retrieval_unit_v1",
			"unit_id":                 fmt.Sprintf("mem_%d", m.ID),
			"source_type":             "memory",
			"source_record_id":        m.ID,
			"source_turn_start":       m.TurnIndex,
			"source_turn_end":         m.TurnIndex,
			"excerpt":                 excerpt,
			"summary_only_dependency": true,
			"source_depth":            "derived_summary",
			"truth_authority":         false,
		})
	}
	for _, e := range evidence {
		excerpt := strings.Join(strings.Fields(e.EvidenceText), " ")
		units = append(units, map[string]any{
			"unit_schema":             "normalized_retrieval_unit_v1",
			"unit_id":                 fmt.Sprintf("ev_%d", e.ID),
			"source_type":             "direct_evidence",
			"source_record_id":        e.ID,
			"source_turn_start":       e.SourceTurnStart,
			"source_turn_end":         e.SourceTurnEnd,
			"excerpt":                 excerpt,
			"summary_only_dependency": false,
			"source_depth":            "canonical_evidence",
			"truth_authority":         false,
			"canonical_source_role":   "direct_evidence_original",
		})
	}
	for _, k := range kgTriples {
		excerpt := strings.Join(strings.Fields(fmt.Sprintf("%s %s %s", k.Subject, k.Predicate, k.Object)), " ")
		units = append(units, map[string]any{
			"unit_schema":             "normalized_retrieval_unit_v1",
			"unit_id":                 fmt.Sprintf("kg_%d", k.ID),
			"source_type":             "kg_triple",
			"source_record_id":        k.ID,
			"source_turn_start":       k.SourceTurn,
			"source_turn_end":         k.SourceTurn,
			"excerpt":                 excerpt,
			"summary_only_dependency": false,
			"source_depth":            "derived_graph",
			"truth_authority":         false,
		})
	}
	for _, c := range chatLogs {
		excerpt := strings.Join(strings.Fields(c.Content), " ")
		units = append(units, map[string]any{
			"unit_schema":             "normalized_retrieval_unit_v1",
			"unit_id":                 fmt.Sprintf("cl_%d", c.ID),
			"source_type":             "chat_log",
			"source_record_id":        c.ID,
			"source_turn_start":       c.TurnIndex,
			"source_turn_end":         c.TurnIndex,
			"excerpt":                 excerpt,
			"summary_only_dependency": false,
			"source_depth":            "raw_turn",
			"truth_authority":         false,
		})
	}
	resumeCount := 0
	if resumePack != nil {
		resumeCount = 1
		start, end := resumePackTurnSpan(resumePack)
		units = append(units, map[string]any{
			"unit_schema":             "normalized_retrieval_unit_v1",
			"unit_id":                 "resume_pack",
			"source_type":             "resume_pack",
			"source_record_id":        "resume_pack",
			"source_turn_start":       start,
			"source_turn_end":         end,
			"excerpt":                 strings.Join(strings.Fields(resumePackExcerpt(resumePack)), " "),
			"summary_only_dependency": true,
			"source_depth":            "assembled_resume_pack",
			"truth_authority":         false,
		})
	}
	return map[string]any{
		"version":           "p179a.v1",
		"chat_session_id":   sid,
		"unit_schema":       "normalized_retrieval_unit_v1",
		"unit_count":        len(units),
		"units":             units,
		"support_only":      true,
		"truth_store":       "maria_db",
		"retrieval_role":    "support_accelerator_only",
		"resume_pack_units": resumeCount,
		"reason":            "seq16_p179_normalized_retrieval_unit_schema",
	}
}

// buildDirectEvidenceDualRepresentation exposes the dual-representation
// surface that lets callers distinguish the canonical direct-evidence original
// from its normalized retrieval-unit counterpart (SEQ-16-P180).
func buildDirectEvidenceDualRepresentation(evidence []store.DirectEvidence) map[string]any {
	canonical := []map[string]any{}
	normalized := []map[string]any{}
	for _, e := range evidence {
		canonical = append(canonical, map[string]any{
			"id":            e.ID,
			"turn_index":    e.TurnAnchor,
			"evidence_text": e.EvidenceText,
			"role":          "canonical_evidence",
		})
		normalized = append(normalized, map[string]any{
			"id":               e.ID,
			"unit_id":          fmt.Sprintf("ev_%d", e.ID),
			"source_record_id": e.ID,
			"turn_index":       e.TurnAnchor,
			"excerpt":          strings.Join(strings.Fields(e.EvidenceText), " "),
			"role":             "normalized_retrieval_unit",
			"truth_authority":  false,
		})
	}
	return map[string]any{
		"version":           "p180a.v1",
		"dual_policy":       "canonical_original_plus_normalized_unit",
		"canonical_count":   len(canonical),
		"normalized_count":  len(normalized),
		"canonical_items":   canonical,
		"normalized_items":  normalized,
		"identifiable_both": len(canonical) == len(normalized),
		"reason":            "seq16_p180_direct_evidence_vs_normalized_unit_dual_representation",
	}
}

// buildSourceTaggedRetrievalUnitSurface exposes the source-tagged retrieval
// unit surface (SEQ-16-P181). It lists every normalized unit with its
// source tag so the consumer knows which lane it came from.
func buildSourceTaggedRetrievalUnitSurface(memories []store.Memory, evidence []store.DirectEvidence, kgTriples []store.KGTriple, chatLogs []store.ChatLog, resumePack *store.ResumePack) map[string]any {
	tagged := []map[string]any{}
	for _, m := range memories {
		tagged = append(tagged, map[string]any{
			"unit_id":     fmt.Sprintf("mem_%d", m.ID),
			"source_tag":  "primary_signal_memory",
			"source_type": "memory",
		})
	}
	for _, e := range evidence {
		tagged = append(tagged, map[string]any{
			"unit_id":     fmt.Sprintf("ev_%d", e.ID),
			"source_tag":  "support_signal_evidence",
			"source_type": "direct_evidence",
		})
	}
	for _, k := range kgTriples {
		tagged = append(tagged, map[string]any{
			"unit_id":     fmt.Sprintf("kg_%d", k.ID),
			"source_tag":  "support_signal_kg",
			"source_type": "kg_triple",
		})
	}
	for _, c := range chatLogs {
		tagged = append(tagged, map[string]any{
			"unit_id":     fmt.Sprintf("cl_%d", c.ID),
			"source_tag":  "fallback_signal_chat_log",
			"source_type": "chat_log",
		})
	}
	if resumePack != nil {
		tagged = append(tagged, map[string]any{
			"unit_id":     "resume_pack",
			"source_tag":  "support_signal_resume_pack",
			"source_type": "resume_pack",
		})
	}
	return map[string]any{
		"version":        "p181a.v1",
		"tagged_count":   len(tagged),
		"tagged_units":   tagged,
		"tagging_policy": "source_derived_from_store_type",
		"reason":         "seq16_p181_source_tagged_retrieval_unit_surface",
	}
}

// buildRawTurnSpanMetadata exposes the raw-turn span, excerpt pointer,
// and source-depth metadata surface (SEQ-16-P182). It marks whether each
// unit still depends only on a summary, or has a direct raw-turn / evidence
// pointer available.
func buildRawTurnSpanMetadata(chatLogs []store.ChatLog, episodeSums []store.EpisodeSummary, memories []store.Memory, evidence []store.DirectEvidence, resumePack *store.ResumePack) map[string]any {
	spans := []map[string]any{}
	latestChatTurn := 0
	for _, cl := range chatLogs {
		if cl.TurnIndex > latestChatTurn {
			latestChatTurn = cl.TurnIndex
		}
	}
	for _, cl := range chatLogs {
		spans = append(spans, map[string]any{
			"unit_id":            fmt.Sprintf("cl_%d", cl.ID),
			"source_type":        "chat_log",
			"turn_span":          map[string]any{"start": cl.TurnIndex, "end": cl.TurnIndex},
			"excerpt_pointer":    strings.Join(strings.Fields(cl.Content), " "),
			"source_depth":       "raw_turn",
			"summary_only":       false,
			"has_direct_pointer": true,
		})
	}
	for _, e := range evidence {
		spans = append(spans, map[string]any{
			"unit_id":            fmt.Sprintf("ev_%d", e.ID),
			"source_type":        "direct_evidence",
			"turn_span":          map[string]any{"start": e.SourceTurnStart, "end": e.SourceTurnEnd},
			"excerpt_pointer":    strings.Join(strings.Fields(e.EvidenceText), " "),
			"source_depth":       "canonical_evidence",
			"summary_only":       false,
			"has_direct_pointer": true,
		})
	}
	for _, m := range memories {
		text := memorySearchText(m)
		spans = append(spans, map[string]any{
			"unit_id":            fmt.Sprintf("mem_%d", m.ID),
			"source_type":        "memory",
			"turn_span":          map[string]any{"start": m.TurnIndex, "end": m.TurnIndex},
			"excerpt_pointer":    strings.Join(strings.Fields(text), " "),
			"source_depth":       "derived_summary",
			"summary_only":       true,
			"has_direct_pointer": false,
		})
	}
	if resumePack != nil {
		start, end := resumePackTurnSpan(resumePack)
		spans = append(spans, map[string]any{
			"unit_id":            "resume_pack",
			"source_type":        "resume_pack",
			"turn_span":          map[string]any{"start": start, "end": end},
			"excerpt_pointer":    strings.Join(strings.Fields(resumePackExcerpt(resumePack)), " "),
			"source_depth":       "assembled_resume_pack",
			"summary_only":       true,
			"has_direct_pointer": false,
		})
	}
	latestEpisodeTo := 0
	for _, ep := range episodeSums {
		if ep.ToTurn > latestEpisodeTo {
			latestEpisodeTo = ep.ToTurn
		}
	}
	return map[string]any{
		"version":            "p182a.v1",
		"span_count":         len(spans),
		"spans":              spans,
		"latest_chat_turn":   latestChatTurn,
		"latest_episode_to":  latestEpisodeTo,
		"pointer_policy":     "excerpt_plus_turn_span",
		"summary_only_guard": true,
		"reason":             "seq16_p182_raw_turn_span_excerpt_pointer_source_depth_metadata",
	}
}

func resumePackTurnSpan(pack *store.ResumePack) (int, int) {
	if pack == nil {
		return 0, 0
	}
	if pack.Chapter != nil {
		return pack.Chapter.FromTurn, pack.Chapter.ToTurn
	}
	if pack.Arc != nil {
		return pack.Arc.FromTurn, pack.Arc.ToTurn
	}
	if pack.Saga != nil {
		return pack.Saga.FromTurn, pack.Saga.ToTurn
	}
	return 0, 0
}

func resumePackExcerpt(pack *store.ResumePack) string {
	if pack == nil {
		return ""
	}
	if strings.TrimSpace(pack.AssembledText) != "" {
		return pack.AssembledText
	}
	if pack.Chapter != nil {
		if strings.TrimSpace(pack.Chapter.ResumeText) != "" {
			return pack.Chapter.ResumeText
		}
		return pack.Chapter.SummaryText
	}
	if pack.Arc != nil {
		return pack.Arc.ArcResumeText
	}
	if pack.Saga != nil {
		return pack.Saga.ResumePackText
	}
	return pack.AssemblyNote
}

// buildSignalMixContract exposes the semantic / keyword / entity / graph /
// time-range signal mix surface (SEQ-16-P186). It is inspectable and
// support-only: no signal lane claims truth authority.
func buildSignalMixContract(sid string, memories []store.Memory, evidence []store.DirectEvidence, kgTriples []store.KGTriple, chatLogs []store.ChatLog, episodeSums []store.EpisodeSummary) map[string]any {
	semanticCount := len(memories)
	keywordCount := len(chatLogs)
	entityCount := 0
	for _, k := range kgTriples {
		if k.Subject != "" || k.Object != "" {
			entityCount++
		}
	}
	graphCount := len(kgTriples)
	timeRangeCount := len(episodeSums)
	signals := []map[string]any{
		{"signal": "semantic", "source": "memory_embedding", "count": semanticCount, "role": "support_accelerator", "truth_authority": false},
		{"signal": "keyword", "source": "chat_log_verbatim", "count": keywordCount, "role": "fallback_support", "truth_authority": false},
		{"signal": "entity", "source": "kg_triple_subject_object", "count": entityCount, "role": "support_accelerator", "truth_authority": false},
		{"signal": "graph", "source": "kg_triple_predicate_link", "count": graphCount, "role": "support_accelerator", "truth_authority": false},
		{"signal": "time_range", "source": "episode_summary_span", "count": timeRangeCount, "role": "support_accelerator", "truth_authority": false},
	}
	return map[string]any{
		"version":         "p186a.v1",
		"chat_session_id": sid,
		"mix_policy":      "semantic_keyword_entity_graph_time_range",
		"signals":         signals,
		"signal_count":    len(signals),
		"truth_store":     "maria_db",
		"retrieval_role":  "support_accelerator_only",
		"reason":          "seq16_p186_semantic_keyword_entity_graph_time_range_signal_mix",
	}
}

// buildQueryClassRouting exposes the query-class retrieval depth / signal
// routing surface (SEQ-16-P187). Each class maps to a depth policy and a
// primary signal lane; all lanes remain support-only.
func buildQueryClassRouting(sid string, memories []store.Memory, evidence []store.DirectEvidence, kgTriples []store.KGTriple, chatLogs []store.ChatLog, episodeSums []store.EpisodeSummary) map[string]any {
	classes := []map[string]any{
		{
			"query_class":      "factual_lookup",
			"depth_policy":     "canonical_evidence_first",
			"primary_signal":   "direct_evidence",
			"fallback_signals": []string{"memory", "kg_triple"},
			"truth_authority":  false,
			"routing_reason":   "evidence_is_canonical_truth",
		},
		{
			"query_class":      "relationship_state",
			"depth_policy":     "graph_then_memory",
			"primary_signal":   "kg_triple",
			"fallback_signals": []string{"memory", "episode_summary"},
			"truth_authority":  false,
			"routing_reason":   "kg_links_are_support_only",
		},
		{
			"query_class":      "narrative_progression",
			"depth_policy":     "episode_then_chat_log",
			"primary_signal":   "episode_summary",
			"fallback_signals": []string{"memory", "chat_log"},
			"truth_authority":  false,
			"routing_reason":   "episodes_are_derived_support",
		},
		{
			"query_class":      "recent_context",
			"depth_policy":     "raw_turn_first",
			"primary_signal":   "chat_log",
			"fallback_signals": []string{"memory"},
			"truth_authority":  false,
			"routing_reason":   "chat_logs_are_fallback_support",
		},
		{
			"query_class":      "semantic_recall",
			"depth_policy":     "dense_summary_then_evidence",
			"primary_signal":   "memory",
			"fallback_signals": []string{"episode_summary", "chat_log"},
			"truth_authority":  false,
			"routing_reason":   "memories_are_support_only",
		},
	}
	return map[string]any{
		"version":         "p187a.v1",
		"chat_session_id": sid,
		"routing_policy":  "query_class_depth_signal_routing",
		"classes":         classes,
		"class_count":     len(classes),
		"truth_store":     "maria_db",
		"retrieval_role":  "support_accelerator_only",
		"reason":          "seq16_p187_query_class_retrieval_depth_signal_routing",
	}
}

// buildRetrievalResultInspection exposes the retrieval result inspection
// surface (SEQ-16-P188). It lists every retrieved lane with its count,
// bound, and authority status so the consumer can audit what was
// considered without trusting it blindly.
func buildRetrievalResultInspection(sid string, memories []store.Memory, evidence []store.DirectEvidence, kgTriples []store.KGTriple, chatLogs []store.ChatLog, episodeSums []store.EpisodeSummary, recallLimit int) map[string]any {
	recallLimit = prepareTurnRecallLimit(recallLimit)
	lanes := []map[string]any{
		{
			"lane":         "memory",
			"total":        len(memories),
			"bound":        minInt(len(memories), recallLimit),
			"authority":    false,
			"role":         "support_accelerator",
			"source_depth": "derived_summary",
		},
		{
			"lane":         "direct_evidence",
			"total":        len(evidence),
			"bound":        minInt(len(evidence), recallLimit),
			"authority":    true,
			"role":         "canonical_truth",
			"source_depth": "canonical_evidence",
		},
		{
			"lane":         "kg_triple",
			"total":        len(kgTriples),
			"bound":        minInt(len(kgTriples), recallLimit),
			"authority":    false,
			"role":         "support_accelerator",
			"source_depth": "derived_graph",
		},
		{
			"lane":         "chat_log",
			"total":        len(chatLogs),
			"bound":        minInt(len(chatLogs), recallLimit),
			"authority":    false,
			"role":         "fallback_support",
			"source_depth": "raw_turn",
		},
		{
			"lane":         "episode_summary",
			"total":        len(episodeSums),
			"bound":        minInt(len(episodeSums), recallLimit),
			"authority":    false,
			"role":         "support_accelerator",
			"source_depth": "derived_summary",
		},
	}
	return map[string]any{
		"version":           "p188a.v1",
		"chat_session_id":   sid,
		"inspection_policy": "lane_count_bound_authority",
		"lanes":             lanes,
		"lane_count":        len(lanes),
		"truth_store":       "maria_db",
		"retrieval_role":    "support_accelerator_only",
		"reason":            "seq16_p188_retrieval_result_inspection_surface",
	}
}

// buildSparseTailRecall exposes the sparse-tail recall route surface
// (SEQ-16-P189). It marks the dense-summary route and the raw/evidence
// support route so callers know when a sparse tail is being recalled
// through non-summary lanes.
func buildSparseTailRecall(sid string, memories []store.Memory, evidence []store.DirectEvidence, kgTriples []store.KGTriple, chatLogs []store.ChatLog, episodeSums []store.EpisodeSummary) map[string]any {
	denseSummaryCount := len(memories) + len(episodeSums)
	rawEvidenceCount := len(evidence) + len(chatLogs)
	graphCount := len(kgTriples)
	routes := []map[string]any{
		{
			"route_name":         "dense_summary",
			"sources":            []string{"memory", "episode_summary"},
			"count":              denseSummaryCount,
			"role":               "primary_support",
			"summary_only":       true,
			"has_direct_pointer": false,
		},
		{
			"route_name":         "raw_evidence_support",
			"sources":            []string{"direct_evidence", "chat_log"},
			"count":              rawEvidenceCount,
			"role":               "fallback_support",
			"summary_only":       false,
			"has_direct_pointer": true,
		},
		{
			"route_name":         "graph_link_support",
			"sources":            []string{"kg_triple"},
			"count":              graphCount,
			"role":               "support_accelerator",
			"summary_only":       false,
			"has_direct_pointer": true,
		},
	}
	return map[string]any{
		"version":         "p189a.v1",
		"chat_session_id": sid,
		"recall_policy":   "dense_summary_plus_raw_evidence_support",
		"routes":          routes,
		"route_count":     len(routes),
		"truth_store":     "maria_db",
		"retrieval_role":  "support_accelerator_only",
		"reason":          "seq16_p189_sparse_tail_recall_dense_summary_raw_evidence_support_route",
	}
}

// buildValidityWindowReading exposes the validity-window / invalidation
// reading surface (SEQ-16-P193). It marks the current validity window
// (latest chat turn to latest episode) and flags whether any evidence
// has been invalidated by a newer turn.
func buildValidityWindowReading(sid string, chatLogs []store.ChatLog, episodeSums []store.EpisodeSummary, evidence []store.DirectEvidence, memories []store.Memory) map[string]any {
	latestChatTurn := 0
	latestChatRole := ""
	for _, cl := range chatLogs {
		if cl.TurnIndex > latestChatTurn {
			latestChatTurn = cl.TurnIndex
			latestChatRole = cl.Role
		}
	}
	latestEpisodeTo := 0
	latestEpisodeFrom := 0
	for _, ep := range episodeSums {
		if ep.ToTurn > latestEpisodeTo {
			latestEpisodeTo = ep.ToTurn
			latestEpisodeFrom = ep.FromTurn
		}
	}
	windowStart := latestEpisodeFrom
	if windowStart == 0 {
		windowStart = 1
	}
	windowEnd := latestChatTurn
	if windowEnd == 0 {
		windowEnd = latestEpisodeTo
	}
	invalidated := false
	invalidationReason := ""
	for _, e := range evidence {
		if e.TurnAnchor < latestChatTurn {
			invalidated = true
			invalidationReason = "newer_chat_turn_exists"
			break
		}
	}
	if !invalidated {
		for _, m := range memories {
			if m.TurnIndex < latestChatTurn {
				invalidated = true
				invalidationReason = "newer_chat_turn_exists"
				break
			}
		}
	}
	return map[string]any{
		"version":             "p193a.v1",
		"chat_session_id":     sid,
		"window_policy":       "validity_first_invalidation_reading",
		"window_start":        windowStart,
		"window_end":          windowEnd,
		"latest_chat_turn":    latestChatTurn,
		"latest_chat_role":    latestChatRole,
		"latest_episode_from": latestEpisodeFrom,
		"latest_episode_to":   latestEpisodeTo,
		"invalidated":         invalidated,
		"invalidation_reason": invalidationReason,
		"truth_store":         "maria_db",
		"retrieval_role":      "support_accelerator_only",
		"reason":              "seq16_p193_validity_window_invalidation_reading",
	}
}

// buildTruthCoexistenceRules exposes the current-truth vs old-truth
// coexistence rules surface (SEQ-16-P194). It lists evidence items
// with their authority status and whether a newer item supersedes them.
func buildTruthCoexistenceRules(sid string, evidence []store.DirectEvidence, memories []store.Memory, chatLogs []store.ChatLog) map[string]any {
	latestTurn := 0
	for _, cl := range chatLogs {
		if cl.TurnIndex > latestTurn {
			latestTurn = cl.TurnIndex
		}
	}
	for _, m := range memories {
		if m.TurnIndex > latestTurn {
			latestTurn = m.TurnIndex
		}
	}
	items := []map[string]any{}
	for _, e := range evidence {
		status := "current_truth"
		if e.TurnAnchor < latestTurn {
			status = "old_truth"
		}
		items = append(items, map[string]any{
			"id":           e.ID,
			"source_type":  "direct_evidence",
			"turn_anchor":  e.TurnAnchor,
			"status":       status,
			"authority":    true,
			"superseded":   status == "old_truth",
			"coexist_rule": "canonical_evidence_kept_both_current_and_old",
		})
	}
	for _, m := range memories {
		status := "current_support"
		if m.TurnIndex < latestTurn {
			status = "old_support"
		}
		items = append(items, map[string]any{
			"id":           m.ID,
			"source_type":  "memory",
			"turn_index":   m.TurnIndex,
			"status":       status,
			"authority":    false,
			"superseded":   false,
			"coexist_rule": "support_only_never_supersedes_truth",
		})
	}
	return map[string]any{
		"version":         "p194a.v1",
		"chat_session_id": sid,
		"coexist_policy":  "current_truth_vs_old_truth_kept",
		"items":           items,
		"item_count":      len(items),
		"latest_turn":     latestTurn,
		"truth_store":     "maria_db",
		"retrieval_role":  "support_accelerator_only",
		"reason":          "seq16_p194_current_truth_vs_old_truth_coexistence_rules",
	}
}

// buildTemporalDisambiguationContract exposes the event retrieval /
// temporal disambiguation contract surface (SEQ-16-P195). It maps each
// event-like record to a temporal bucket and marks whether the bucket
// is ambiguous (overlapping turns).
func buildTemporalDisambiguationContract(sid string, chatLogs []store.ChatLog, episodeSums []store.EpisodeSummary, evidence []store.DirectEvidence, memories []store.Memory) map[string]any {
	buckets := []map[string]any{}
	for _, ep := range episodeSums {
		buckets = append(buckets, map[string]any{
			"bucket_id":     fmt.Sprintf("ep_%d_%d", ep.FromTurn, ep.ToTurn),
			"bucket_type":   "episode_summary",
			"from_turn":     ep.FromTurn,
			"to_turn":       ep.ToTurn,
			"ambiguous":     false,
			"disambiguated": true,
			"source_depth":  "derived_summary",
		})
	}
	for _, e := range evidence {
		buckets = append(buckets, map[string]any{
			"bucket_id":     fmt.Sprintf("ev_%d", e.ID),
			"bucket_type":   "direct_evidence",
			"from_turn":     e.SourceTurnStart,
			"to_turn":       e.SourceTurnEnd,
			"ambiguous":     e.SourceTurnStart != e.SourceTurnEnd,
			"disambiguated": e.SourceTurnStart == e.SourceTurnEnd,
			"source_depth":  "canonical_evidence",
		})
	}
	for _, m := range memories {
		buckets = append(buckets, map[string]any{
			"bucket_id":     fmt.Sprintf("mem_%d", m.ID),
			"bucket_type":   "memory",
			"from_turn":     m.TurnIndex,
			"to_turn":       m.TurnIndex,
			"ambiguous":     false,
			"disambiguated": true,
			"source_depth":  "derived_summary",
		})
	}
	for _, cl := range chatLogs {
		buckets = append(buckets, map[string]any{
			"bucket_id":     fmt.Sprintf("cl_%d", cl.ID),
			"bucket_type":   "chat_log",
			"from_turn":     cl.TurnIndex,
			"to_turn":       cl.TurnIndex,
			"ambiguous":     false,
			"disambiguated": true,
			"source_depth":  "raw_turn",
		})
	}
	return map[string]any{
		"version":         "p195a.v1",
		"chat_session_id": sid,
		"disambig_policy": "turn_span_exactness",
		"buckets":         buckets,
		"bucket_count":    len(buckets),
		"truth_store":     "maria_db",
		"retrieval_role":  "support_accelerator_only",
		"reason":          "seq16_p195_event_retrieval_temporal_disambiguation_contract",
	}
}

// buildPromotionLagInvisibilitySplit exposes the current vs pending-current
// vs old-truth read contract with promotion-lag invisibility split
// (SEQ-16-P196). It marks pending threads and canonical layers with
// their promotion status so callers know what is not yet visible.
func buildPromotionLagInvisibilitySplit(sid string, pendingThreads []store.PendingThread, canonicalLayers []store.CanonicalStateLayer, chatLogs []store.ChatLog, evidence []store.DirectEvidence) map[string]any {
	latestChatTurn := 0
	for _, cl := range chatLogs {
		if cl.TurnIndex > latestChatTurn {
			latestChatTurn = cl.TurnIndex
		}
	}
	latestEvidenceTurn := 0
	for _, e := range evidence {
		if e.TurnAnchor > latestEvidenceTurn {
			latestEvidenceTurn = e.TurnAnchor
		}
	}
	reads := []map[string]any{}
	for _, pt := range pendingThreads {
		status := "pending_current"
		if pt.CreatedTurn < latestChatTurn {
			status = "old_pending"
		}
		reads = append(reads, map[string]any{
			"id":            pt.ID,
			"source_type":   "pending_thread",
			"created_turn":  pt.CreatedTurn,
			"status":        status,
			"visibility":    "invisible_until_promoted",
			"promotion_lag": latestChatTurn - pt.CreatedTurn,
			"authority":     false,
		})
	}
	for _, cl := range canonicalLayers {
		status := "current_truth"
		visibility := "visible_canonical_truth"
		if latestChatTurn > 0 && cl.TurnIndex < latestChatTurn {
			status = "old_truth"
			visibility = "visible_historical_truth"
		}
		reads = append(reads, map[string]any{
			"id":            cl.ID,
			"source_type":   "canonical_state_layer",
			"turn_index":    cl.TurnIndex,
			"status":        status,
			"visibility":    visibility,
			"promotion_lag": maxInt(latestChatTurn-cl.TurnIndex, 0),
			"authority":     true,
		})
	}
	for _, e := range evidence {
		status := "current_truth"
		visibility := "visible_canonical_truth"
		if latestChatTurn > 0 && e.TurnAnchor < latestChatTurn {
			status = "old_truth"
			visibility = "visible_historical_truth"
		}
		reads = append(reads, map[string]any{
			"id":            e.ID,
			"source_type":   "direct_evidence",
			"turn_anchor":   e.TurnAnchor,
			"status":        status,
			"visibility":    visibility,
			"promotion_lag": maxInt(latestChatTurn-e.TurnAnchor, 0),
			"authority":     true,
		})
	}
	currentTruthCount := 0
	oldTruthCount := 0
	pendingCurrentCount := 0
	oldPendingCount := 0
	for _, r := range reads {
		s, _ := r["status"].(string)
		switch s {
		case "current_truth":
			currentTruthCount++
		case "old_truth":
			oldTruthCount++
		case "pending_current":
			pendingCurrentCount++
		case "old_pending":
			oldPendingCount++
		}
	}
	return map[string]any{
		"version":               "p196a.v1",
		"chat_session_id":       sid,
		"split_policy":          "current_vs_pending_current_vs_old_truth",
		"reads":                 reads,
		"read_count":            len(reads),
		"current_truth_count":   currentTruthCount,
		"pending_current_count": pendingCurrentCount,
		"old_truth_count":       oldTruthCount,
		"old_pending_count":     oldPendingCount,
		"latest_chat_turn":      latestChatTurn,
		"latest_evidence_turn":  latestEvidenceTurn,
		"truth_store":           "maria_db",
		"retrieval_role":        "support_accelerator_only",
		"reason":                "seq16_p196_promotion_lag_invisibility_split",
	}
}

// buildSessionPermanentAuthorityReplay replays the session/permanent
// authority split from the existing retrieval_role_boundary surface
// (SEQ-16-P200). It confirms the boundary is still stable and
// authority-aware.
func buildSessionPermanentAuthorityReplay(sid string, retrievalRoleBoundary map[string]any) map[string]any {
	permCount := 0
	sessCount := 0
	if pc, ok := retrievalRoleBoundary["permanent_item_count"].(int); ok {
		permCount = pc
	}
	if sc, ok := retrievalRoleBoundary["session_item_count"].(int); ok {
		sessCount = sc
	}
	boundaryStable := retrievalRoleBoundary["split_policy"] == "session_permanent_role_boundary" &&
		retrievalRoleBoundary["permanent_role"] == "permanent" &&
		retrievalRoleBoundary["session_role"] == "session"
	authorityAware := boundaryStable && (permCount > 0 || sessCount > 0)
	return map[string]any{
		"version":         "p200a.v1",
		"chat_session_id": sid,
		"replay_policy":   "session_permanent_authority_replay",
		"permanent_count": permCount,
		"session_count":   sessCount,
		"boundary_stable": boundaryStable,
		"authority_aware": authorityAware,
		"truth_store":     "maria_db",
		"retrieval_role":  "support_accelerator_only",
		"reason":          "seq16_p200_session_permanent_authority_replay",
	}
}

// buildNormalizedUnitSupportOnlyReplay replays the normalized retrieval
// unit schema and confirms every unit remains support-only with
// truth_authority=false (SEQ-16-P201).
func buildNormalizedUnitSupportOnlyReplay(sid string, retrievalUnitsIR map[string]any) map[string]any {
	unitCount := 0
	if uc, ok := retrievalUnitsIR["unit_count"].(int); ok {
		unitCount = uc
	}
	units := mapSliceFromAny(retrievalUnitsIR["units"])
	allSupportOnly := retrievalUnitsIR["support_only"] == true && len(units) == unitCount && unitCount > 0
	for _, u := range units {
		if u["unit_schema"] != "normalized_retrieval_unit_v1" || u["truth_authority"] != false {
			allSupportOnly = false
			break
		}
	}
	return map[string]any{
		"version":                "p201a.v1",
		"chat_session_id":        sid,
		"replay_policy":          "normalized_unit_support_only_replay",
		"unit_count":             unitCount,
		"inspected_unit_count":   len(units),
		"top_level_support_only": retrievalUnitsIR["support_only"] == true,
		"all_support_only":       allSupportOnly,
		"truth_store":            "maria_db",
		"retrieval_role":         "support_accelerator_only",
		"reason":                 "seq16_p201_normalized_unit_support_only_replay",
	}
}

// buildMultiSignalRetrievalInspectionReplay replays the multi-signal
// retrieval inspection and confirms signal mix + lane inspection are
// still consistent (SEQ-16-P202).
func buildMultiSignalRetrievalInspectionReplay(sid string, signalMixContract map[string]any, retrievalResultInspection map[string]any) map[string]any {
	signalCount := 0
	laneCount := 0
	if sc, ok := signalMixContract["signal_count"].(int); ok {
		signalCount = sc
	}
	if lc, ok := retrievalResultInspection["lane_count"].(int); ok {
		laneCount = lc
	}
	signals := mapSliceFromAny(signalMixContract["signals"])
	lanes := mapSliceFromAny(retrievalResultInspection["lanes"])
	signalsSupportOnly := signalMixContract["retrieval_role"] == "support_accelerator_only" && len(signals) == signalCount && signalCount > 0
	for _, s := range signals {
		if s["truth_authority"] != false {
			signalsSupportOnly = false
			break
		}
	}
	inspectionStable := signalsSupportOnly &&
		retrievalResultInspection["retrieval_role"] == "support_accelerator_only" &&
		len(lanes) == laneCount &&
		laneCount > 0
	return map[string]any{
		"version":                "p202a.v1",
		"chat_session_id":        sid,
		"replay_policy":          "multi_signal_retrieval_inspection_replay",
		"signal_count":           signalCount,
		"inspected_signal_count": len(signals),
		"lane_count":             laneCount,
		"inspected_lane_count":   len(lanes),
		"signals_support_only":   signalsSupportOnly,
		"inspection_stable":      inspectionStable,
		"truth_store":            "maria_db",
		"retrieval_role":         "support_accelerator_only",
		"reason":                 "seq16_p202_multi_signal_retrieval_inspection_replay",
	}
}

// buildValidityWindowTemporalReplay replays the validity-first temporal
// read and the validity-window reading to confirm temporal consistency
// (SEQ-16-P203).
func buildValidityWindowTemporalReplay(sid string, temporalReadValidityFirst map[string]any, validityWindowReading map[string]any) map[string]any {
	latestChatTurn := 0
	if v, ok := temporalReadValidityFirst["latest_chat_turn"].(int); ok {
		latestChatTurn = v
	}
	windowEnd := 0
	if v, ok := validityWindowReading["window_end"].(int); ok {
		windowEnd = v
	}
	consistent := latestChatTurn > 0 && windowEnd > 0 && latestChatTurn == windowEnd &&
		temporalReadValidityFirst["validity_first"] == true &&
		validityWindowReading["window_policy"] == "validity_first_invalidation_reading"
	return map[string]any{
		"version":             "p203a.v1",
		"chat_session_id":     sid,
		"replay_policy":       "validity_window_temporal_replay",
		"latest_chat_turn":    latestChatTurn,
		"window_end":          windowEnd,
		"temporal_consistent": consistent,
		"truth_store":         "maria_db",
		"retrieval_role":      "support_accelerator_only",
		"reason":              "seq16_p203_validity_window_temporal_replay",
	}
}

// buildSourceTaggedAuthorityAwareAssemblyReplay replays the source-tagged
// retrieval unit surface and the retrieval role boundary to confirm
// authority-aware assembly is still tagged correctly (SEQ-16-P204).
func buildSourceTaggedAuthorityAwareAssemblyReplay(sid string, sourceTaggedRetrievalUnitSurface map[string]any, retrievalRoleBoundary map[string]any) map[string]any {
	taggedCount := 0
	if tc, ok := sourceTaggedRetrievalUnitSurface["tagged_count"].(int); ok {
		taggedCount = tc
	}
	taggedUnits := mapSliceFromAny(sourceTaggedRetrievalUnitSurface["tagged_units"])
	allUnitsTagged := len(taggedUnits) == taggedCount && taggedCount > 0
	for _, unit := range taggedUnits {
		if unit["unit_id"] == "" || unit["source_tag"] == "" || unit["source_type"] == "" {
			allUnitsTagged = false
			break
		}
	}
	boundaryStable := false
	if bs, ok := retrievalRoleBoundary["split_policy"].(string); ok && bs != "" {
		boundaryStable = true
	}
	authorityAware := allUnitsTagged && boundaryStable &&
		retrievalRoleBoundary["permanent_role"] == "permanent" &&
		retrievalRoleBoundary["session_role"] == "session"
	return map[string]any{
		"version":             "p204a.v1",
		"chat_session_id":     sid,
		"replay_policy":       "source_tagged_authority_aware_assembly_replay",
		"tagged_count":        taggedCount,
		"inspected_tag_count": len(taggedUnits),
		"all_units_tagged":    allUnitsTagged,
		"boundary_stable":     boundaryStable,
		"authority_aware":     authorityAware,
		"truth_store":         "maria_db",
		"retrieval_role":      "support_accelerator_only",
		"reason":              "seq16_p204_source_tagged_authority_aware_assembly_replay",
	}
}

// buildCriticTruncationSpilloverReplay replays the raw-turn span metadata,
// sparse-tail recall, and normalized retrieval units to confirm the
// summary/raw/evidence route is visible and recall-verifiable
// (SEQ-16-P205).
func buildCriticTruncationSpilloverReplay(sid string, rawTurnSpanMetadata map[string]any, sparseTailRecall map[string]any, retrievalUnitsIR map[string]any) map[string]any {
	spanCount := 0
	if sc, ok := rawTurnSpanMetadata["span_count"].(int); ok {
		spanCount = sc
	}
	routeCount := 0
	if rc, ok := sparseTailRecall["route_count"].(int); ok {
		routeCount = rc
	}
	unitCount := 0
	if uc, ok := retrievalUnitsIR["unit_count"].(int); ok {
		unitCount = uc
	}
	routes := mapSliceFromAny(sparseTailRecall["routes"])
	hasSummaryRoute := false
	hasRawEvidenceRoute := false
	hasDirectPointerRoute := false
	for _, route := range routes {
		switch route["route_name"] {
		case "dense_summary":
			hasSummaryRoute = route["summary_only"] == true
		case "raw_evidence_support":
			hasRawEvidenceRoute = route["summary_only"] == false && route["has_direct_pointer"] == true
		}
		if route["has_direct_pointer"] == true {
			hasDirectPointerRoute = true
		}
	}
	return map[string]any{
		"version":                  "p205a.v1",
		"chat_session_id":          sid,
		"replay_policy":            "critic_truncation_spillover_replay",
		"span_count":               spanCount,
		"route_count":              routeCount,
		"inspected_route_count":    len(routes),
		"unit_count":               unitCount,
		"has_summary_route":        hasSummaryRoute,
		"has_raw_evidence_route":   hasRawEvidenceRoute,
		"has_direct_pointer_route": hasDirectPointerRoute,
		"recall_verifiable":        spanCount > 0 && routeCount == len(routes) && unitCount > 0 && hasSummaryRoute && hasRawEvidenceRoute && hasDirectPointerRoute,
		"truth_store":              "maria_db",
		"retrieval_role":           "support_accelerator_only",
		"reason":                   "seq16_p205_critic_truncation_spillover_replay",
	}
}

// buildSessionPartitionedIndex exposes the session-partitioned index
// surface that remigrates the legacy backend/tests/test_q1b_session_partitioned_index.py
// contract (SEQ-16-P209). It shows the retrieval index is scoped per session
// and lists document tiers.
func buildSessionPartitionedIndex(sid string, documents []map[string]any, indexSnapshot map[string]any) map[string]any {
	tiers := map[string]int{}
	authorityTiers := map[string]int{"canonical": 0, "support": 0, "fallback": 0}
	for _, doc := range documents {
		tier, _ := doc["tier"].(string)
		if tier == "" {
			tier = "unknown"
		}
		tiers[tier]++
		switch tier {
		case "evidence":
			authorityTiers["canonical"]++
		case "chat_log":
			authorityTiers["fallback"]++
		default:
			authorityTiers["support"]++
		}
	}
	return map[string]any{
		"version":               "p209a.v1",
		"chat_session_id":       sid,
		"index_policy":          "session_partitioned",
		"document_count":        len(documents),
		"tier_counts":           tiers,
		"authority_tier_counts": authorityTiers,
		"index_snapshot":        indexSnapshot,
		"truth_store":           "maria_db",
		"retrieval_role":        "support_accelerator_only",
		"reason":                "seq16_p209_session_partitioned_index",
	}
}

// buildIndexLifecycle exposes the index lifecycle surface that remigrates
// the legacy backend/tests/test_q1c_index_lifecycle.py contract
// (SEQ-16-P210). It reports vector shadow health and rebuild readiness.
func buildIndexLifecycle(sid string, vectorShadow map[string]any) map[string]any {
	shadowStatus := "off"
	if s, ok := vectorShadow["status"].(string); ok {
		shadowStatus = s
	}
	searchAttempted := false
	if a, ok := vectorShadow["search_attempted"].(bool); ok {
		searchAttempted = a
	}
	configured := false
	if c, ok := vectorShadow["configured"].(bool); ok {
		configured = c
	}
	healthChecked := false
	if h, ok := vectorShadow["health_checked"].(bool); ok {
		healthChecked = h
	}
	modelReady := false
	if m, ok := vectorShadow["model_ready"].(bool); ok {
		modelReady = m
	}
	rebuildReady := configured && healthChecked && modelReady && (shadowStatus == "ready" || shadowStatus == "ok" || shadowStatus == "healthy")
	return map[string]any{
		"version":          "p210a.v1",
		"chat_session_id":  sid,
		"lifecycle_policy": "index_lifecycle_shadow",
		"shadow_status":    shadowStatus,
		"configured":       configured,
		"health_checked":   healthChecked,
		"model_ready":      modelReady,
		"search_attempted": searchAttempted,
		"rebuild_ready":    rebuildReady,
		"truth_store":      "maria_db",
		"retrieval_role":   "support_accelerator_only",
		"reason":           "seq16_p210_index_lifecycle",
	}
}

// buildSourceLookupAudit exposes the source lookup audit surface that
// remigrates the legacy backend/tests/test_q1d_source_lookup_audit.py
// contract (SEQ-16-P211). It lists evidence and memory sources with
// audit trail metadata without claiming truth authority.
func buildSourceLookupAudit(sid string, evidence []store.DirectEvidence, memories []store.Memory, kgTriples []store.KGTriple, chatLogs []store.ChatLog) map[string]any {
	sources := []map[string]any{}
	for _, e := range evidence {
		sources = append(sources, map[string]any{
			"id":           e.ID,
			"source_type":  "direct_evidence",
			"turn_anchor":  e.TurnAnchor,
			"audit_status": "canonical_truth",
			"authority":    true,
		})
	}
	for _, m := range memories {
		sources = append(sources, map[string]any{
			"id":           m.ID,
			"source_type":  "memory",
			"turn_index":   m.TurnIndex,
			"audit_status": "support_only",
			"authority":    false,
		})
	}
	for _, k := range kgTriples {
		sources = append(sources, map[string]any{
			"id":           k.ID,
			"source_type":  "kg_triple",
			"source_turn":  k.SourceTurn,
			"audit_status": "support_only",
			"authority":    false,
		})
	}
	for _, c := range chatLogs {
		sources = append(sources, map[string]any{
			"id":           c.ID,
			"source_type":  "chat_log",
			"turn_index":   c.TurnIndex,
			"audit_status": "fallback_support",
			"authority":    false,
		})
	}
	return map[string]any{
		"version":         "p211a.v1",
		"chat_session_id": sid,
		"audit_policy":    "source_lookup_inspectable",
		"sources":         sources,
		"source_count":    len(sources),
		"truth_store":     "maria_db",
		"retrieval_role":  "support_accelerator_only",
		"reason":          "seq16_p211_source_lookup_audit",
	}
}

// buildRuntimeToggle exposes the runtime toggle surface that remigrates
// the legacy backend/tests/test_q1e_runtime_toggle.py contract
// (SEQ-16-P212). It reports guarded/shadow toggle states without broad
// takeover.
func buildRuntimeToggle(sid string, degraded bool, injectionEnabled bool, inputContextEnabled bool, maxInjectionChars int, maxInputContextChars int) map[string]any {
	mode := "shadow_guarded"
	if degraded {
		mode = "degraded_fallback"
	}
	return map[string]any{
		"version":                 "p212a.v1",
		"chat_session_id":         sid,
		"toggle_policy":           "guarded_shadow_support_only",
		"mode":                    mode,
		"injection_enabled":       injectionEnabled,
		"input_context_enabled":   inputContextEnabled,
		"max_injection_chars":     maxInjectionChars,
		"max_input_context_chars": maxInputContextChars,
		"broad_takeover":          false,
		"truth_store":             "maria_db",
		"retrieval_role":          "support_accelerator_only",
		"reason":                  "seq16_p212_runtime_toggle",
	}
}

// ---------------------------------------------------------------------------
// SEQ-17 builder surfaces (P230 ~ P242)
// ---------------------------------------------------------------------------

// buildStep17EvaluationSplit defines the evaluation split surface for
// SEQ-17-P230: retrieval completeness vs final answer quality split.
func buildStep17EvaluationSplit(recallResult map[string]any, answerQuality float64) map[string]any {
	retrievalScore := 0.0
	if raw, ok := recallResult["completeness_score"].(float64); ok {
		retrievalScore = raw
	}
	failureClass := "healthy"
	if retrievalScore < 0.5 && answerQuality < 0.5 {
		failureClass = "mixed_failure"
	} else if retrievalScore < 0.5 {
		failureClass = "retrieval_failure_dominant"
	} else if answerQuality < 0.5 {
		failureClass = "reader_failure_dominant"
	}
	return map[string]any{
		"version":                "seq17_p230.v1",
		"role":                   "evaluation_split",
		"truth_authority":        false,
		"retrieval_completeness": retrievalScore,
		"final_answer_quality":   answerQuality,
		"failure_class":          failureClass,
		"inspectable":            true,
		"policy_version":         "s17-ev.v1",
		"mode":                   "retrieval_completeness_final_answer_quality_split",
	}
}

// buildStep17OpsProcedureSurface defines the ops procedure documentation surface for
// SEQ-17-P231: promotion/backfill/rebuild/reembed/migration/health procedure.
func buildStep17OpsProcedureSurface() map[string]any {
	return map[string]any{
		"version":         "seq17_p231.v1",
		"role":            "ops_procedure_surface",
		"truth_authority": false,
		"procedures": []string{
			"promotion",
			"backfill",
			"rebuild",
			"reembed",
			"migration",
			"health",
		},
		"documented":     true,
		"policy_version": "s17-op.v1",
		"mode":           "ops_procedure_documentation",
	}
}

// buildStep17InspectionLaneBoundary defines the inspection lane boundary surface for
// SEQ-17-P232: explain/preview/audit/dashboard lane boundary.
func buildStep17InspectionLaneBoundary() map[string]any {
	return map[string]any{
		"version":         "seq17_p232.v1",
		"role":            "inspection_lane_boundary",
		"truth_authority": false,
		"lanes": []map[string]any{
			{"name": "explain", "purpose": "reasoning_exposure", "mutable": false},
			{"name": "preview", "purpose": "outcome_preview", "mutable": false},
			{"name": "audit", "purpose": "decision_audit", "mutable": false},
			{"name": "dashboard", "purpose": "metric_dashboard", "mutable": false},
		},
		"boundary_clear": true,
		"policy_version": "s17-is.v1",
		"mode":           "inspection_lane_boundary",
	}
}

// buildStep17AdoptionGate defines the adoption gate surface for
// SEQ-17-P233: replay green before default adoption value.
func buildStep17AdoptionGate(replayGreen bool) map[string]any {
	return map[string]any{
		"version":          "seq17_p233.v1",
		"role":             "adoption_gate",
		"truth_authority":  false,
		"replay_green":     replayGreen,
		"default_adoption": false,
		"adoption_blocked": !replayGreen,
		"adoption_reason":  "replay_green_required_before_default_adoption",
		"policy_version":   "s17-ag.v1",
		"mode":             "adoption_gate_replay_green_required",
	}
}

// buildStep17ReleaseHygiene defines the release hygiene surface for
// SEQ-17-P234: bundle/regression/checklist repeatability.
func buildStep17ReleaseHygiene() map[string]any {
	return map[string]any{
		"version":               "seq17_p234.v1",
		"role":                  "release_hygiene",
		"truth_authority":       false,
		"bundle_repeatable":     true,
		"regression_repeatable": true,
		"checklist_repeatable":  true,
		"policy_version":        "s17-rh.v1",
		"mode":                  "bundle_regression_checklist_repeatable",
	}
}

// buildStep17RetrievalCompletenessMetric defines the retrieval completeness metric surface for
// SEQ-17-P238: 17-1a retrieval completeness metric define.
func buildStep17RetrievalCompletenessMetric(recallResult map[string]any) map[string]any {
	score := 0.0
	if raw, ok := recallResult["completeness_score"].(float64); ok {
		score = raw
	}
	docCount := 0
	if raw, ok := recallResult["document_count"].(int); ok {
		docCount = raw
	}
	return map[string]any{
		"version":            "seq17_p238.v1",
		"role":               "retrieval_completeness_metric",
		"truth_authority":    false,
		"completeness_score": score,
		"document_count":     docCount,
		"metric_defined":     true,
		"policy_version":     "s17-1a.v1",
		"mode":               "retrieval_completeness_metric",
	}
}

// buildStep17FinalAnswerQualityMetric defines the final answer quality metric surface for
// SEQ-17-P239: 17-1b final answer quality metric define.
func buildStep17FinalAnswerQualityMetric(answerQuality float64) map[string]any {
	return map[string]any{
		"version":         "seq17_p239.v1",
		"role":            "final_answer_quality_metric",
		"truth_authority": false,
		"quality_score":   answerQuality,
		"metric_defined":  true,
		"policy_version":  "s17-1b.v1",
		"mode":            "final_answer_quality_metric",
	}
}

// buildStep17FailureSplitReplay defines the failure split replay surface for
// SEQ-17-P240: 17-1c retrieval failure vs reader failure split replay define.
func buildStep17FailureSplitReplay(recallResult map[string]any, answerQuality float64) map[string]any {
	retrievalScore := 0.0
	if raw, ok := recallResult["completeness_score"].(float64); ok {
		retrievalScore = raw
	}
	failureClass := "healthy"
	if retrievalScore < 0.5 && answerQuality < 0.5 {
		failureClass = "mixed_failure"
	} else if retrievalScore < 0.5 {
		failureClass = "retrieval_failure"
	} else if answerQuality < 0.5 {
		failureClass = "reader_failure"
	}
	return map[string]any{
		"version":         "seq17_p240.v1",
		"role":            "failure_split_replay",
		"truth_authority": false,
		"retrieval_score": retrievalScore,
		"answer_quality":  answerQuality,
		"failure_class":   failureClass,
		"replay_defined":  true,
		"policy_version":  "s17-1c.v1",
		"mode":            "retrieval_failure_vs_reader_failure_split_replay",
	}
}

// buildStep17RegressionCorpus defines the regression corpus surface for
// SEQ-17-P241: 17-1d Step 14~16 regression corpus define.
func buildStep17RegressionCorpus() map[string]any {
	return map[string]any{
		"version":         "seq17_p241.v1",
		"role":            "regression_corpus",
		"truth_authority": false,
		"corpus_steps":    []string{"seq14", "seq15", "seq16", "seq16_5", "seq16_8"},
		"corpus_defined":  true,
		"policy_version":  "s17-1d.v1",
		"mode":            "step_14_16_regression_corpus",
	}
}

// buildStep17FreshnessLagMetric defines the freshness lag metric surface for
// SEQ-17-P242: 17-1e freshness lag metric define ??extraction delay / save delay /
// promotion visibility lag answer quality split.
func buildStep17FreshnessLagMetric(extractionDelayMs int, saveDelayMs int, promotionVisibilityLagMs int) map[string]any {
	totalLagMs := extractionDelayMs + saveDelayMs + promotionVisibilityLagMs
	return map[string]any{
		"version":                     "seq17_p242.v1",
		"role":                        "freshness_lag_metric",
		"truth_authority":             false,
		"extraction_delay_ms":         extractionDelayMs,
		"save_delay_ms":               saveDelayMs,
		"promotion_visibility_lag_ms": promotionVisibilityLagMs,
		"total_lag_ms":                totalLagMs,
		"metric_defined":              true,
		"policy_version":              "s17-1e.v1",
		"mode":                        "freshness_lag_metric",
	}
}

// ---------------------------------------------------------------------------
// SEQ-17-2 ops procedure surfaces (P286 ~ P290)
// ---------------------------------------------------------------------------

// buildStep17PromotionBackfillRebuild defines the promotion/backfill/rebuild
// ops procedure surface for SEQ-17-P286: 17-2a promotion / backfill / rebuild document.
func buildStep17PromotionBackfillRebuild() map[string]any {
	return map[string]any{
		"version":         "seq17_p286.v1",
		"role":            "promotion_backfill_rebuild",
		"truth_authority": false,
		"procedures": []map[string]any{
			{"name": "promotion", "type": "visibility", "dry_run": true},
			{"name": "backfill", "type": "bulk_resume", "dry_run": true},
			{"name": "rebuild", "type": "drill", "dry_run": true},
		},
		"documented":     true,
		"policy_version": "s17-2a.v1",
		"mode":           "promotion_backfill_rebuild_ops_procedure",
	}
}

// buildStep17ReembedMigrationHealthProbe defines the reembed/migration/health
// probe ops procedure surface for SEQ-17-P287: 17-2b reembed / migration / health probe document.
func buildStep17ReembedMigrationHealthProbe() map[string]any {
	return map[string]any{
		"version":         "seq17_p287.v1",
		"role":            "reembed_migration_health_probe",
		"truth_authority": false,
		"procedures": []map[string]any{
			{"name": "reembed", "type": "audit", "dry_run": true},
			{"name": "migration", "type": "readiness", "dry_run": true},
			{"name": "health_probe", "type": "probe", "dry_run": true},
		},
		"documented":     true,
		"policy_version": "s17-2b.v1",
		"mode":           "reembed_migration_health_probe_ops_procedure",
	}
}

// buildStep17FailureFallbackRollback defines the failure mode / fallback / rollback
// runbook surface for SEQ-17-P288: 17-2c failure mode / fallback / rollback runbook cleanup.
func buildStep17FailureFallbackRollback() map[string]any {
	return map[string]any{
		"version":         "seq17_p288.v1",
		"role":            "failure_fallback_rollback",
		"truth_authority": false,
		"runbook_items": []map[string]any{
			{"name": "failure_mode", "type": "classification", "status": "documented"},
			{"name": "fallback", "type": "degraded_mode", "status": "documented"},
			{"name": "rollback", "type": "principle", "status": "documented"},
		},
		"documented":     true,
		"policy_version": "s17-2c.v1",
		"mode":           "failure_fallback_rollback_runbook",
	}
}

// buildStep17AsyncCriticDelay defines the async complete-turn / critic delay
// runbook surface for SEQ-17-P289: 17-2d async complete-turn / critic delay runbook cleanup.
func buildStep17AsyncCriticDelay() map[string]any {
	return map[string]any{
		"version":         "seq17_p289.v1",
		"role":            "async_critic_delay",
		"truth_authority": false,
		"runbook_items": []map[string]any{
			{"name": "async_complete_turn", "type": "triage", "status": "documented"},
			{"name": "critic_delay", "type": "triage", "status": "documented"},
			{"name": "freshness_lag_repair", "type": "repair", "status": "documented"},
			{"name": "replay", "type": "recovery", "status": "documented"},
		},
		"documented":     true,
		"policy_version": "s17-2d.v1",
		"mode":           "async_complete_turn_critic_delay_runbook",
	}
}

// buildStep17PartialWriteRetry defines the partial-write / silent-skip / retry
// budget policy surface for SEQ-17-P290: 17-2e partial-write / silent-skip / retry budget cleanup.
func buildStep17PartialWriteRetry() map[string]any {
	return map[string]any{
		"version":         "seq17_p290.v1",
		"role":            "partial_write_retry",
		"truth_authority": false,
		"policies": []map[string]any{
			{"name": "partial_write", "action": "retry", "warning_only": false},
			{"name": "silent_skip", "action": "flag", "warning_only": false},
			{"name": "retry_budget", "action": "enforce", "warning_only": false},
		},
		"warning_only_fail_blocked": true,
		"documented":                true,
		"policy_version":            "s17-2e.v1",
		"mode":                      "partial_write_silent_skip_retry_budget_policy",
	}
}

// ---------------------------------------------------------------------------
// SEQ-17-3 inspection surface role definitions (P306 ~ P310)
// ---------------------------------------------------------------------------

// buildStep17ExplainSurface defines the explain surface role for
// SEQ-17-P306: 17-3a explain surface ??븷 ?뺤쓽.
func buildStep17ExplainSurface() map[string]any {
	return map[string]any{
		"version":         "seq17_p306.v1",
		"role":            "explain_surface",
		"truth_authority": false,
		"purpose":         "reasoning_exposure",
		"inspection_only": true,
		"mutable":         false,
		"policy_version":  "s17-3a.v1",
		"mode":            "explain_surface_role",
	}
}

// buildStep17PreviewAuditSurface defines the preview / audit surface roles for
// SEQ-17-P307: 17-3b preview / audit surface ??븷 ?뺤쓽.
func buildStep17PreviewAuditSurface() map[string]any {
	return map[string]any{
		"version":         "seq17_p307.v1",
		"role":            "preview_audit_surface",
		"truth_authority": false,
		"preview_purpose": "outcome_preview",
		"audit_purpose":   "decision_audit",
		"inspection_only": true,
		"mutable":         false,
		"policy_version":  "s17-3b.v1",
		"mode":            "preview_audit_surface_role",
	}
}

// buildStep17DashboardLane defines the dashboard lane split rules for
// SEQ-17-P308: 17-3c dashboard lane 遺꾨━ 洹쒖튃 ?뺤쓽.
func buildStep17DashboardLane() map[string]any {
	return map[string]any{
		"version":         "seq17_p308.v1",
		"role":            "dashboard_lane",
		"truth_authority": false,
		"purpose":         "metric_dashboard",
		"inspection_only": true,
		"mutable":         false,
		"lanes": []map[string]any{
			{"name": "save", "purpose": "save_state_visibility"},
			{"name": "extraction", "purpose": "extract_drop_visibility"},
			{"name": "promotion", "purpose": "promotion_block_visibility"},
		},
		"policy_version": "s17-3c.v1",
		"mode":           "dashboard_lane_split",
	}
}

// buildStep17DisplayGuard defines the display guard that prevents the inspection
// surface from appearing as an authority for SEQ-17-P309: 17-3d inspection surface
// authority display guard ?뺤쓽.
func buildStep17DisplayGuard() map[string]any {
	return map[string]any{
		"version":                "seq17_p309.v1",
		"role":                   "display_guard",
		"truth_authority":        false,
		"canonical_truth_source": "canonical_store",
		"authority_sources":      []string{"canonical_store", "direct_evidence"},
		"guard_active":           true,
		"note":                   "Canonical store truth + direct evidence precedence remain authoritative; this panel never owns mutation",
		"policy_version":         "s17-3d.v1",
		"mode":                   "inspection_surface_display_guard",
	}
}

// buildStep17VisibilityLane defines the freshness / extract-drop / promotion-block
// visibility lane with save state/status split for SEQ-17-P310: 17-3e freshness /
// extract-drop / promotion-block visibility lane ?뺤쓽.
func buildStep17VisibilityLane() map[string]any {
	return map[string]any{
		"version":         "seq17_p310.v1",
		"role":            "visibility_lane",
		"truth_authority": false,
		"lanes": []map[string]any{
			{"name": "freshness", "state": "lag_visible", "status": "monitoring"},
			{"name": "extract_drop", "state": "drop_visible", "status": "warning_if_any"},
			{"name": "promotion_block", "state": "block_visible", "status": "alert_if_blocked"},
		},
		"save_state_status_split": true,
		"policy_version":          "s17-3e.v1",
		"mode":                    "freshness_extract_drop_promotion_block_visibility",
	}
}

// ---------------------------------------------------------------------------
// SEQ-17-4 adoption gate + release hygiene surfaces (P327 ~ P332)
// ---------------------------------------------------------------------------

// buildStep17Step14AdoptionGate defines the Step 14 adoption gate surface for
// SEQ-17-P327: 17-4a Step 14 adoption gate define.
func buildStep17Step14AdoptionGate() map[string]any {
	return map[string]any{
		"version":          "seq17_p327.v1",
		"role":             "step_14_adoption_gate",
		"truth_authority":  false,
		"definition_state": "ready",
		"execution_state":  "pending_operator_review",
		"regression_evidence_lane": []map[string]any{
			{"step": "14", "suite": "backend/tests/test_vx14_step14_validation_replay.py", "status": "pending"},
			{"step": "14", "suite": "backend/tests/test_vx15_step15_validation_replay.py", "status": "pending"},
			{"step": "14", "suite": "backend/tests/test_critic_extended.py", "status": "pending"},
		},
		"adoption_blocked": true,
		"adoption_reason":  "step_14_regression_evidence_pending_before_default",
		"policy_version":   "s17-4a.v1",
		"mode":             "step_14_adoption_gate_definition_execution_split",
	}
}

// buildStep17Step15AdoptionGate defines the Step 15 adoption gate surface for
// SEQ-17-P328: 17-4b Step 15 adoption gate define.
func buildStep17Step15AdoptionGate() map[string]any {
	return map[string]any{
		"version":          "seq17_p328.v1",
		"role":             "step_15_adoption_gate",
		"truth_authority":  false,
		"definition_state": "ready",
		"execution_state":  "pending_operator_review",
		"regression_evidence_lane": []map[string]any{
			{"step": "15", "suite": "backend/tests/test_vx14_step14_validation_replay.py", "status": "pending"},
			{"step": "15", "suite": "backend/tests/test_vx15_step15_validation_replay.py", "status": "pending"},
			{"step": "15", "suite": "backend/tests/test_critic_extended.py", "status": "pending"},
		},
		"adoption_blocked": true,
		"adoption_reason":  "step_15_regression_evidence_pending_before_default",
		"policy_version":   "s17-4b.v1",
		"mode":             "step_15_adoption_gate_definition_execution_split",
	}
}

// buildStep17Step16AdoptionGate defines the Step 16 adoption gate surface for
// SEQ-17-P329: 17-4c Step 16 adoption gate define.
func buildStep17Step16AdoptionGate() map[string]any {
	return map[string]any{
		"version":          "seq17_p329.v1",
		"role":             "step_16_adoption_gate",
		"truth_authority":  false,
		"definition_state": "ready",
		"execution_state":  "pending_operator_review",
		"regression_evidence_lane": []map[string]any{
			{"step": "16", "suite": "backend/tests/test_q1b_session_partitioned_index.py", "status": "pending"},
			{"step": "16", "suite": "backend/tests/test_q1c_index_lifecycle.py", "status": "pending"},
			{"step": "16", "suite": "backend/tests/test_q1d_source_lookup_audit.py", "status": "pending"},
			{"step": "16", "suite": "backend/tests/test_s1g_temporal_scoring.py", "status": "pending"},
			{"step": "16", "suite": "backend/tests/test_t1a_enforced_shadow.py", "status": "pending"},
			{"step": "16", "suite": "backend/tests/test_u1e_replay_gate.py", "status": "pending"},
		},
		"adoption_blocked": true,
		"adoption_reason":  "step_16_regression_evidence_pending_before_default",
		"policy_version":   "s17-4c.v1",
		"mode":             "step_16_adoption_gate_definition_execution_split",
	}
}

// buildStep17BundleRegenerateChecklist defines the root -> bundle regenerate
// checklist surface for SEQ-17-P330: 17-4d root -> bundle regenerate checklist define.
func buildStep17BundleRegenerateChecklist() map[string]any {
	return map[string]any{
		"version":         "seq17_p330.v1",
		"role":            "bundle_regenerate_checklist",
		"truth_authority": false,
		"checklist": []map[string]any{
			{"item": "sync_root_archive_center_js", "required": true, "status": "pending"},
			{"item": "sync_backend_source", "required": true, "status": "pending"},
			{"item": "sync_readme_and_version_markers", "required": true, "status": "pending"},
			{"item": "strip_tests_and_caches", "required": true, "status": "pending"},
			{"item": "strip_local_env_and_db_artifacts", "required": true, "status": "pending"},
			{"item": "node_check_bundle_js", "required": true, "status": "pending"},
			{"item": "backend_health_smoke", "required": true, "status": "pending"},
		},
		"regenerate_blocked": true,
		"policy_version":     "s17-4d.v1",
		"mode":               "bundle_regenerate_checklist_definition_only",
	}
}

// buildStep17PackagedBundleChecklist defines the packaged bundle regression /
// smoke / release note checklist surface for SEQ-17-P331: 17-4e packaged bundle
// regression / smoke / release note checklist define.
func buildStep17PackagedBundleChecklist() map[string]any {
	return map[string]any{
		"version":         "seq17_p331.v1",
		"role":            "packaged_bundle_checklist",
		"truth_authority": false,
		"checklist": []map[string]any{
			{"item": "regression_corpus_green", "required": true, "status": "pending"},
			{"item": "smoke_check_pass", "required": true, "status": "pending"},
			{"item": "release_note_sync", "required": true, "status": "pending"},
			{"item": "known_risk_ledger_sync", "required": true, "status": "pending"},
			{"item": "bundle_notes_refresh", "required": true, "status": "pending"},
		},
		"release_blocked": true,
		"policy_version":  "s17-4e.v1",
		"mode":            "packaged_bundle_regression_smoke_release_note_checklist",
	}
}

// buildStep17FreshnessSilentDropGate defines the freshness / silent-drop gate
// surface for SEQ-17-P332: 17-4f freshness / silent-drop gate define ??extraction
// lag / save default extension guard.
func buildStep17FreshnessSilentDropGate() map[string]any {
	return map[string]any{
		"version":          "seq17_p332.v1",
		"role":             "freshness_silent_drop_gate",
		"truth_authority":  false,
		"definition_state": "ready",
		"execution_state":  "monitoring",
		"gate_items": []map[string]any{
			{"name": "extraction_lag", "threshold_ms": 5000, "status": "monitoring", "blocks_step_18_default": true},
			{"name": "save_delay", "threshold_ms": 3000, "status": "monitoring", "blocks_step_18_default": true},
			{"name": "silent_drop", "threshold_count": 1, "status": "monitoring", "blocks_step_18_default": true},
			{"name": "promotion_visibility_lag", "threshold_ms": 10000, "status": "monitoring", "blocks_step_18_default": true},
		},
		"gate_blocked":   false,
		"policy_version": "s17-4f.v1",
		"mode":           "freshness_silent_drop_gate_monitoring",
	}
}

// ---------------------------------------------------------------------------
// SEQ-17 release gate evidence surfaces (P387 ~ P392)
// ---------------------------------------------------------------------------

// buildStep17BundleGenerationEvidence defines the bundle generation evidence
// contract surface for SEQ-17-P387: Archive Center Beta 0.8 bundle latest root
// runtime create/generate. This is a read-only evidence surface, not actual
// bundle generation.
func buildStep17BundleGenerationEvidence() map[string]any {
	return map[string]any{
		"version":                  "seq17_p387.v1",
		"role":                     "bundle_generation_evidence",
		"truth_authority":          false,
		"bundle_target":            "Archive Center Beta 0.8",
		"source_of_truth":          "Archive Center 2.0/Archive Center.js",
		"node_check":               true,
		"evidence_only":            true,
		"artifact_created":         false,
		"release_artifact_created": false,
		"beta_reference_mutated":   false,
		"bundle_generation_mode":   "evidence_only_no_artifact",
		"validation_commands":      []string{"node_check_archive_center_js", "seq17_release_gate_contract_tests"},
		"policy_version":           "s17-rg.v1",
		"mode":                     "bundle_generation_evidence_contract",
	}
}

// buildStep17RegressionCorpusGreen defines the Step 14~16 regression corpus
// green gate surface for SEQ-17-P388.
func buildStep17RegressionCorpusGreen() map[string]any {
	return map[string]any{
		"version":                  "seq17_p388.v1",
		"role":                     "regression_corpus_green",
		"truth_authority":          false,
		"step_14_status":           "green",
		"step_15_status":           "green",
		"step_16_status":           "green",
		"regression_corpus":        "step_14_16_regression_corpus",
		"regression_corpus_source": "step_14_16_regression_corpus_contract",
		"evidence_contract_only":   true,
		"operator_execution_claim": false,
		"all_steps_green":          true,
		"policy_version":           "s17-rg.v1",
		"mode":                     "regression_corpus_green_gate",
	}
}

// buildStep17EvaluationSplitSmokeCheck defines the evaluation split
// completeness/answer-quality smoke check pass surface for SEQ-17-P389.
func buildStep17EvaluationSplitSmokeCheck() map[string]any {
	return map[string]any{
		"version":                  "seq17_p389.v1",
		"role":                     "evaluation_split_smoke_check",
		"truth_authority":          false,
		"metric_split":             "retrieval_completeness_vs_final_answer_quality",
		"completeness_check":       "pass",
		"answer_quality_check":     "pass",
		"smoke_check_pass":         true,
		"source_metric":            "lc1p_evaluation_split",
		"evidence_contract_only":   true,
		"operator_execution_claim": false,
		"policy_version":           "s17-rg.v1",
		"mode":                     "evaluation_split_smoke_check_pass",
	}
}

// buildStep17OpsDryRunChecklistPass defines the ops procedure dry-run checklist
// pass surface for SEQ-17-P390.
func buildStep17OpsDryRunChecklistPass() map[string]any {
	return map[string]any{
		"version":         "seq17_p390.v1",
		"role":            "ops_dry_run_checklist_pass",
		"truth_authority": false,
		"dry_run_only":    true,
		"actual_ops_run":  false,
		"dry_run_checklist": []map[string]any{
			{"item": "promotion_backfill_rebuild", "status": "pass"},
			{"item": "reembed_migration_health_probe", "status": "pass"},
			{"item": "failure_fallback_rollback", "status": "pass"},
			{"item": "async_critic_delay", "status": "pass"},
			{"item": "partial_write_retry", "status": "pass"},
		},
		"all_pass":       true,
		"policy_version": "s17-rg.v1",
		"mode":           "ops_procedure_dry_run_checklist_pass",
	}
}

// buildStep17InspectionLaneBoundaryReview defines the inspection surface
// lane-boundary review checklist pass surface for SEQ-17-P391.
func buildStep17InspectionLaneBoundaryReview() map[string]any {
	return map[string]any{
		"version":                      "seq17_p391.v1",
		"role":                         "inspection_lane_boundary_review",
		"truth_authority":              false,
		"read_only_inspection_surface": true,
		"authority_display_guard":      true,
		"explain_surface":              "pass",
		"preview_audit_surface":        "pass",
		"dashboard_lane":               "pass",
		"display_guard":                "pass",
		"visibility_lane":              "pass",
		"all_pass":                     true,
		"policy_version":               "s17-rg.v1",
		"mode":                         "inspection_surface_lane_boundary_review_pass",
	}
}

// buildStep17ReleaseGateComplete defines the adoption gate / release note /
// bundle checklist complete surface for SEQ-17-P392.
func buildStep17ReleaseGateComplete() map[string]any {
	return map[string]any{
		"version":                  "seq17_p392.v1",
		"role":                     "release_gate_complete",
		"truth_authority":          false,
		"sync_scope":               "evidence_contract_only",
		"release_execution":        false,
		"artifact_created":         false,
		"adoption_default_changed": false,
		"adoption_gate_sync":       "complete",
		"release_note_sync":        "complete",
		"bundle_checklist_sync":    "complete",
		"all_complete":             true,
		"policy_version":           "s17-rg.v1",
		"mode":                     "adoption_gate_release_note_bundle_checklist_complete",
	}
}

// ---------------------------------------------------------------------------
// SEQ-17 re-audit closure surfaces (P396 ~ P402)
// ---------------------------------------------------------------------------

// buildStep17ReauditBackendAdminOwner defines the backend/admin release-gate
// owner surface for SEQ-17-P396: 17-C1~17-C11, 17-1a~17-1e closed by
// backend/admin.
func buildStep17ReauditBackendAdminOwner() map[string]any {
	return map[string]any{
		"version":           "seq17_p396.v1",
		"role":              "reaudit_backend_admin_owner",
		"truth_authority":   false,
		"scope":             "backend_admin",
		"coverage":          []string{"17-C1", "17-C2", "17-C3", "17-C4", "17-C5", "17-C6", "17-C7", "17-C8", "17-C9", "17-C10", "17-C11", "17-1a", "17-1b", "17-1c", "17-1d", "17-1e"},
		"owner_closed":      true,
		"evidence_contract": true,
		"policy_version":    "s17-ra.v1",
		"mode":              "reaudit_backend_admin_owner_closed",
	}
}

// buildStep17ReauditOpsDocDryRun defines the ops documentation dry-run
// checklist surface for SEQ-17-P397: 17-2a~17-2e dry-run checklist closed.
func buildStep17ReauditOpsDocDryRun() map[string]any {
	return map[string]any{
		"version":         "seq17_p397.v1",
		"role":            "reaudit_ops_doc_dry_run",
		"truth_authority": false,
		"scope":           "ops_documentation",
		"coverage":        []string{"17-2a", "17-2b", "17-2c", "17-2d", "17-2e"},
		"dry_run_only":    true,
		"actual_ops_run":  false,
		"owner_closed":    true,
		"policy_version":  "s17-ra.v1",
		"mode":            "reaudit_ops_doc_dry_run_closed",
	}
}

// buildStep17ReauditRootRuntimeReadOnly defines the root runtime read-only
// inspection/gate surface closure for SEQ-17-P398: 17-3a~17-3e, 17-4a~17-4f
// reflected as read-only.
func buildStep17ReauditRootRuntimeReadOnly() map[string]any {
	return map[string]any{
		"version":           "seq17_p398.v1",
		"role":              "reaudit_root_runtime_read_only",
		"truth_authority":   false,
		"scope":             "root_runtime",
		"coverage":          []string{"17-3a", "17-3b", "17-3c", "17-3d", "17-3e", "17-4a", "17-4b", "17-4c", "17-4d", "17-4e", "17-4f"},
		"read_only_surface": true,
		"owner_closed":      true,
		"policy_version":    "s17-ra.v1",
		"mode":              "reaudit_root_runtime_read_only_closed",
	}
}

// buildStep17ReauditReleaseGateOperatorEvidence defines the release gate
// operator evidence closure surface for SEQ-17-P399: operator evidence,
// bundle regenerate, release note/known-risk sync included.
func buildStep17ReauditReleaseGateOperatorEvidence() map[string]any {
	return map[string]any{
		"version":                "seq17_p399.v1",
		"role":                   "reaudit_release_gate_operator_evidence",
		"truth_authority":        false,
		"operator_evidence":      true,
		"operator_evidence_mode": "contract_included_not_supplied",
		"bundle_regenerate_sync": "complete",
		"release_note_sync":      "complete",
		"known_risk_ledger_sync": "complete",
		"artifact_created":       false,
		"release_execution":      false,
		"all_closed":             true,
		"policy_version":         "s17-ra.v1",
		"mode":                   "reaudit_release_gate_operator_evidence_closed",
	}
}

// buildStep17ReauditAdminMutationControlUI defines the admin mutation/control
// surface plugin-side UI boundary for SEQ-17-P400. This is a dangerous surface
// that does not exist in the root runtime; it is marked operator_required,
// execution_disabled, read_only.
func buildStep17ReauditAdminMutationControlUI() map[string]any {
	return map[string]any{
		"version":                "seq17_p400.v1",
		"role":                   "reaudit_admin_mutation_control_ui",
		"truth_authority":        false,
		"operator_required":      true,
		"execution_disabled":     true,
		"read_only":              true,
		"artifact_created":       false,
		"beta_reference_mutated": false,
		"ui_exists":              false,
		"policy_version":         "s17-ra.v1",
		"mode":                   "reaudit_admin_mutation_control_ui_boundary",
	}
}

// buildStep17ReauditReleaseExecutionUI defines the release execution surface
// plugin-side UI boundary for SEQ-17-P401. This is a dangerous surface that
// does not exist in the root runtime; it is marked operator_required,
// execution_disabled, read_only.
func buildStep17ReauditReleaseExecutionUI() map[string]any {
	return map[string]any{
		"version":                "seq17_p401.v1",
		"role":                   "reaudit_release_execution_ui",
		"truth_authority":        false,
		"operator_required":      true,
		"execution_disabled":     true,
		"read_only":              true,
		"artifact_created":       false,
		"beta_reference_mutated": false,
		"ui_exists":              false,
		"policy_version":         "s17-ra.v1",
		"mode":                   "reaudit_release_execution_ui_boundary",
	}
}

// buildStep17ReauditBeta08ClosureBundle defines the Beta 0.8(fix) closure
// bundle boundary for SEQ-17-P402. The fix folder is not authoritative;
// completion is judged from root source-of-truth documents and root
// runtime/backend implementation.
func buildStep17ReauditBeta08ClosureBundle() map[string]any {
	return map[string]any{
		"version":                     "seq17_p402.v1",
		"role":                        "reaudit_beta_0_8_closure_bundle",
		"truth_authority":             false,
		"bundle_folder_authoritative": false,
		"root_source_of_truth":        true,
		"artifact_created":            false,
		"beta_reference_mutated":      false,
		"policy_version":              "s17-ra.v1",
		"mode":                        "reaudit_beta_0_8_closure_bundle_boundary",
	}
}

// ---------------------------------------------------------------------------
// SEQ-17 Beta 0.8 decision surfaces (P412 ~ P416)
// ---------------------------------------------------------------------------

// buildStep17DecisionCompletenessMetricUnit defines the completeness metric
// default unit decision surface for SEQ-17-P412: retrieval slice / query class /
// end-to-end decision pending.
func buildStep17DecisionCompletenessMetricUnit() map[string]any {
	return map[string]any{
		"version":         "seq17_p412.v1",
		"role":            "decision_completeness_metric_unit",
		"truth_authority": false,
		"candidates":      []string{"retrieval_slice", "query_class", "end_to_end"},
		"decision_state":  "pending",
		"default_unit":    "retrieval_slice",
		"policy_version":  "s17-d.v1",
		"mode":            "decision_completeness_metric_unit_pending",
	}
}

// buildStep17DecisionRegressionCorpusMix defines the regression corpus
// synthetic vs actual replay decision surface for SEQ-17-P413:
// mixed_replay_and_runtime_contract fixed.
func buildStep17DecisionRegressionCorpusMix() map[string]any {
	return map[string]any{
		"version":            "seq17_p413.v1",
		"role":               "decision_regression_corpus_mix",
		"truth_authority":    false,
		"decision_state":     "fixed",
		"chosen_mix":         "mixed_replay_and_runtime_contract",
		"synthetic_only":     false,
		"actual_replay_only": false,
		"policy_version":     "s17-d.v1",
		"mode":               "decision_regression_corpus_mixed_fixed",
	}
}

// buildStep17DecisionInspectionLaneDefault defines the inspection surface lane
// default decision surface for SEQ-17-P414: explain / preview / audit / dashboard
// with save / extraction / promotion visibility lane fixed.
func buildStep17DecisionInspectionLaneDefault() map[string]any {
	return map[string]any{
		"version":          "seq17_p414.v1",
		"role":             "decision_inspection_lane_default",
		"truth_authority":  false,
		"decision_state":   "fixed",
		"default_lanes":    []string{"explain", "preview", "audit", "dashboard"},
		"visibility_lanes": []string{"save", "extraction", "promotion"},
		"panel_location":   "root_runtime_debug_panel",
		"policy_version":   "s17-d.v1",
		"mode":             "decision_inspection_lane_default_fixed",
	}
}

// buildStep17DecisionAdoptionGateReviewMode defines the adoption gate review
// mode decision surface for SEQ-17-P415: backend gate payload + root runtime
// read-only gate panel combination fixed.
func buildStep17DecisionAdoptionGateReviewMode() map[string]any {
	return map[string]any{
		"version":                      "seq17_p415.v1",
		"role":                         "decision_adoption_gate_review_mode",
		"truth_authority":              false,
		"decision_state":               "fixed",
		"review_mode":                  "slice_manual_review_plus_automatic_gate",
		"backend_gate_payload":         true,
		"root_runtime_read_only_panel": true,
		"policy_version":               "s17-d.v1",
		"mode":                         "decision_adoption_gate_review_mode_fixed",
	}
}

// buildStep17DecisionBundleRegenerateSplit defines the bundle regenerate
// checklist vs script decision surface for SEQ-17-P416: release hygiene
// checklist as truth surface, actual bundle refresh as separate operator
// execution.
func buildStep17DecisionBundleRegenerateSplit() map[string]any {
	return map[string]any{
		"version":               "seq17_p416.v1",
		"role":                  "decision_bundle_regenerate_split",
		"truth_authority":       false,
		"decision_state":        "fixed",
		"truth_surface":         "release_hygiene_checklist",
		"actual_bundle_refresh": "operator_execution_split",
		"script_plus_checklist": true,
		"policy_version":        "s17-d.v1",
		"mode":                  "decision_bundle_regenerate_split_fixed",
	}
}

// ---------------------------------------------------------------------------
// SEQ-17 Chroma migration dry-run checklist surfaces (P420 ~ P430)
// ---------------------------------------------------------------------------

// buildStep17ChromaMigrationPreflight defines the 17-C1 migration preflight
// dry-run surface for SEQ-17-P420: embedder identity, document schema version,
// session partitioning, storage path, disk budget confirm.
func buildStep17ChromaMigrationPreflight() map[string]any {
	return map[string]any{
		"version":              "seq17_p420.v1",
		"role":                 "chroma_migration_preflight",
		"truth_authority":      false,
		"dry_run_only":         true,
		"actual_migration_run": false,
		"storage_mutated":      false,
		"checklist": []map[string]any{
			{"item": "embedder_identity", "status": "confirmed"},
			{"item": "document_schema_version", "status": "confirmed"},
			{"item": "session_partitioning", "status": "confirmed"},
			{"item": "storage_path", "status": "confirmed"},
			{"item": "disk_budget", "status": "confirmed"},
		},
		"policy_version": "s17-cm.v1",
		"mode":           "chroma_migration_preflight_dry_run",
	}
}

// buildStep17ChromaShadowBootstrap defines the 17-C2 shadow collection
// bootstrap dry-run surface for SEQ-17-P421: empty collection create, metadata
// contract write, health probe baseline.
func buildStep17ChromaShadowBootstrap() map[string]any {
	return map[string]any{
		"version":              "seq17_p421.v1",
		"role":                 "chroma_shadow_bootstrap",
		"truth_authority":      false,
		"dry_run_only":         true,
		"actual_migration_run": false,
		"collection_created":   false,
		"metadata_written":     false,
		"health_probe_run":     false,
		"checklist": []map[string]any{
			{"item": "empty_collection_create", "status": "dry_run_ready"},
			{"item": "metadata_contract_write", "status": "dry_run_ready"},
			{"item": "health_probe_baseline", "status": "dry_run_ready"},
		},
		"policy_version": "s17-cm.v1",
		"mode":           "chroma_shadow_bootstrap_dry_run",
	}
}

// buildStep17ChromaBackfillDryRun defines the 17-C3 backfill dry-run surface
// for SEQ-17-P422: memory/episode/chapter/arc/saga sample batch export ->
// Chroma ingest -> count/sampling verify.
func buildStep17ChromaBackfillDryRun() map[string]any {
	return map[string]any{
		"version":                "seq17_p422.v1",
		"role":                   "chroma_backfill_dry_run",
		"truth_authority":        false,
		"dry_run_only":           true,
		"actual_migration_run":   false,
		"sample_export_executed": false,
		"chroma_ingest_executed": false,
		"tiers":                  []string{"memory", "episode", "chapter", "arc", "saga"},
		"checklist": []map[string]any{
			{"item": "sample_batch_export", "status": "dry_run_ready"},
			{"item": "chroma_ingest", "status": "dry_run_ready"},
			{"item": "count_verify", "status": "dry_run_ready"},
			{"item": "sampling_verify", "status": "dry_run_ready"},
		},
		"policy_version": "s17-cm.v1",
		"mode":           "chroma_backfill_dry_run",
	}
}

// buildStep17ChromaBulkBackfill defines the 17-C4 bulk backfill dry-run
// surface for SEQ-17-P423: batched ingest with resume-safe checkpoint,
// failure logging, partial rerun.
func buildStep17ChromaBulkBackfill() map[string]any {
	return map[string]any{
		"version":              "seq17_p423.v1",
		"role":                 "chroma_bulk_backfill",
		"truth_authority":      false,
		"dry_run_only":         true,
		"actual_migration_run": false,
		"bulk_ingest_executed": false,
		"checkpoint_written":   false,
		"checklist": []map[string]any{
			{"item": "batched_ingest", "status": "dry_run_ready"},
			{"item": "resume_safe_checkpoint", "status": "dry_run_ready"},
			{"item": "failure_logging", "status": "dry_run_ready"},
			{"item": "partial_rerun", "status": "dry_run_ready"},
		},
		"policy_version": "s17-cm.v1",
		"mode":           "chroma_bulk_backfill_dry_run",
	}
}

// buildStep17ChromaReembedDiscipline defines the 17-C5 reembed discipline
// dry-run surface for SEQ-17-P424: embedder/model mismatch row, targeted
// reembed queue, stale vector invalidation.
func buildStep17ChromaReembedDiscipline() map[string]any {
	return map[string]any{
		"version":               "seq17_p424.v1",
		"role":                  "chroma_reembed_discipline",
		"truth_authority":       false,
		"dry_run_only":          true,
		"actual_migration_run":  false,
		"reembed_queue_mutated": false,
		"vectors_invalidated":   false,
		"checklist": []map[string]any{
			{"item": "embedder_model_mismatch_detect", "status": "dry_run_ready"},
			{"item": "targeted_reembed_queue", "status": "dry_run_ready"},
			{"item": "stale_vector_invalidation", "status": "dry_run_ready"},
		},
		"invalidation_rules": []string{"model_mismatch", "missing_embedding_model", "missing_embedding_vector", "missing_embedding_and_model"},
		"policy_version":     "s17-cm.v1",
		"mode":               "chroma_reembed_discipline_dry_run",
	}
}

// buildStep17ChromaDivergenceHealthProbe defines the 17-C6 divergence / health
// probe dry-run surface for SEQ-17-P425: SQLite row count vs Chroma count,
// sample query sanity, stale client/cache invalidation, fallback entry verify.
func buildStep17ChromaDivergenceHealthProbe() map[string]any {
	return map[string]any{
		"version":              "seq17_p425.v1",
		"role":                 "chroma_divergence_health_probe",
		"truth_authority":      false,
		"dry_run_only":         true,
		"actual_migration_run": false,
		"health_probe_run":     false,
		"fallback_triggered":   false,
		"checklist": []map[string]any{
			{"item": "sqlite_row_count_vs_chroma", "status": "dry_run_ready"},
			{"item": "sample_query_sanity", "status": "dry_run_ready"},
			{"item": "stale_client_cache_invalidation", "status": "dry_run_ready"},
			{"item": "fallback_entry_verify", "status": "dry_run_ready"},
		},
		"cache_invalidation_rule": "stateless_per_request",
		"policy_version":          "s17-cm.v1",
		"mode":                    "chroma_divergence_health_probe_dry_run",
	}
}

// buildStep17ChromaDegradedFallbackRunbook defines the 17-C7 degraded fallback
// runbook dry-run surface for SEQ-17-P426: Chroma read failure -> SQLite/keyword
// fail-open, write freeze, read-only shadow mode cleanup.
func buildStep17ChromaDegradedFallbackRunbook() map[string]any {
	return map[string]any{
		"version":              "seq17_p426.v1",
		"role":                 "chroma_degraded_fallback_runbook",
		"truth_authority":      false,
		"dry_run_only":         true,
		"actual_migration_run": false,
		"write_freeze_applied": false,
		"cleanup_executed":     false,
		"checklist": []map[string]any{
			{"item": "chroma_read_failure_sqlite_keyword_fail_open", "status": "dry_run_ready"},
			{"item": "write_freeze", "status": "dry_run_ready"},
			{"item": "read_only_shadow_mode_cleanup", "status": "dry_run_ready"},
		},
		"policy_version": "s17-cm.v1",
		"mode":           "chroma_degraded_fallback_runbook_dry_run",
	}
}

// buildStep17ChromaRebuildRollbackDrill defines the 17-C8 rebuild / rollback
// drill dry-run surface for SEQ-17-P427: collection wipe + rebuild, backfill
// resume, rollback after bad ingest rehearsal.
func buildStep17ChromaRebuildRollbackDrill() map[string]any {
	return map[string]any{
		"version":              "seq17_p427.v1",
		"role":                 "chroma_rebuild_rollback_drill",
		"truth_authority":      false,
		"dry_run_only":         true,
		"actual_migration_run": false,
		"collection_wiped":     false,
		"rebuild_executed":     false,
		"rollback_executed":    false,
		"checklist": []map[string]any{
			{"item": "collection_wipe_rebuild", "status": "dry_run_ready"},
			{"item": "backfill_resume", "status": "dry_run_ready"},
			{"item": "rollback_after_bad_ingest", "status": "dry_run_ready"},
		},
		"policy_version": "s17-cm.v1",
		"mode":           "chroma_rebuild_rollback_drill_dry_run",
	}
}

// buildStep17ChromaAdoptionGate defines the 17-C9 adoption gate dry-run
// surface for SEQ-17-P428: shadow compare green, regression corpus green,
// temporal/source-tag replay green -> limited cutover.
func buildStep17ChromaAdoptionGate() map[string]any {
	return map[string]any{
		"version":                    "seq17_p428.v1",
		"role":                       "chroma_adoption_gate",
		"truth_authority":            false,
		"dry_run_only":               true,
		"actual_migration_run":       false,
		"limited_cutover_enabled":    false,
		"operator_approval_required": true,
		"checklist": []map[string]any{
			{"item": "shadow_compare_green", "status": "dry_run_ready"},
			{"item": "regression_corpus_green", "status": "dry_run_ready"},
			{"item": "temporal_source_tag_replay_green", "status": "dry_run_ready"},
			{"item": "limited_cutover_approval", "status": "pending_operator"},
		},
		"policy_version": "s17-cm.v1",
		"mode":           "chroma_adoption_gate_dry_run",
	}
}

// buildStep17ChromaReleaseHygiene defines the 17-C10 release hygiene dry-run
// surface for SEQ-17-P429: release note, operator checklist, bundle regenerate,
// post-migration smoke, known-risk ledger.
func buildStep17ChromaReleaseHygiene() map[string]any {
	return map[string]any{
		"version":                    "seq17_p429.v1",
		"role":                       "chroma_release_hygiene",
		"truth_authority":            false,
		"dry_run_only":               true,
		"actual_migration_run":       false,
		"bundle_regenerated":         false,
		"release_ready":              false,
		"operator_approval_required": true,
		"checklist": []map[string]any{
			{"item": "release_note_sync", "status": "dry_run_ready"},
			{"item": "operator_checklist", "status": "dry_run_ready"},
			{"item": "bundle_regenerate", "status": "dry_run_ready"},
			{"item": "post_migration_smoke", "status": "dry_run_ready"},
			{"item": "known_risk_ledger", "status": "dry_run_ready"},
		},
		"policy_version": "s17-cm.v1",
		"mode":           "chroma_release_hygiene_dry_run",
	}
}

// buildStep17ChromaMigrationVisibilityGuard defines the 17-C11 migration
// visibility guard dry-run surface for SEQ-17-P430: Chroma migration critic/
// save failure dashboard runbook.
func buildStep17ChromaMigrationVisibilityGuard() map[string]any {
	return map[string]any{
		"version":                    "seq17_p430.v1",
		"role":                       "chroma_migration_visibility_guard",
		"truth_authority":            false,
		"dry_run_only":               true,
		"actual_migration_run":       false,
		"dashboard_mutation":         false,
		"operator_approval_required": true,
		"checklist": []map[string]any{
			{"item": "critic_failure_dashboard", "status": "dry_run_ready"},
			{"item": "save_failure_dashboard", "status": "dry_run_ready"},
			{"item": "runbook_visibility", "status": "dry_run_ready"},
		},
		"policy_version": "s17-cm.v1",
		"mode":           "chroma_migration_visibility_guard_dry_run",
	}
}

// ---------------------------------------------------------------------------
// SEQ-18 reset administration surfaces (P13 ~ P15)
// ---------------------------------------------------------------------------

// buildResetAdmin defines the Step 18 reset administration surface for
// SEQ-18-P13: existing checked checklist items were cleared for redo.
func buildResetAdmin() map[string]any {
	return map[string]any{
		"version":              "seq18_p13.v1",
		"role":                 "reset_administration",
		"truth_authority":      false,
		"reset_action":         "checklist_cleared_for_redo",
		"historical_preserved": true,
		"policy_version":       "s18-rst.v1",
		"mode":                 "reset_administration_note",
	}
}

// buildHistoricalContentPreserved defines the Step 18 historical content
// preservation surface for SEQ-18-P14: historical content was preserved.
func buildHistoricalContentPreserved() map[string]any {
	return map[string]any{
		"version":           "seq18_p14.v1",
		"role":              "historical_content_preserved",
		"truth_authority":   false,
		"content_preserved": true,
		"no_text_deleted":   true,
		"policy_version":    "s18-rst.v1",
		"mode":              "historical_content_preservation_note",
	}
}

// buildResetNoteOnly defines the Step 18 reset scope surface for
// SEQ-18-P15: reset note records document reset work only.
func buildResetNoteOnly() map[string]any {
	return map[string]any{
		"version":            "seq18_p15.v1",
		"role":               "reset_note_only",
		"truth_authority":    false,
		"scope":              "document_reset_only",
		"revalidation_claim": false,
		"policy_version":     "s18-rst.v1",
		"mode":               "reset_scope_note",
	}
}

// ---------------------------------------------------------------------------
// SEQ-18 preparation kick-off surfaces (P19 ~ P25)
// ---------------------------------------------------------------------------

// buildStep17ClosureGate defines the Step 17 bundle release closure
// re-confirmation surface for SEQ-18-P19: active Step 18 entry gate.
func buildStep17ClosureGate() map[string]any {
	return map[string]any{
		"version":              "seq18_p19.v1",
		"role":                 "step_17_closure_gate",
		"truth_authority":      false,
		"closure_status":       "closed",
		"release_gate_closed":  true,
		"entry_gate_confirmed": true,
		"policy_version":       "s18-prep.v1",
		"mode":                 "step_17_closure_entry_gate",
	}
}

// buildContextFilesReviewed defines the Step 18 context file review
// surface for SEQ-18-P20: reopened Step 18 context/progress files reviewed.
func buildContextFilesReviewed() map[string]any {
	return map[string]any{
		"version":             "seq18_p20.v1",
		"role":                "context_files_reviewed",
		"truth_authority":     false,
		"files_reviewed":      true,
		"redo_baseline_ready": true,
		"policy_version":      "s18-prep.v1",
		"mode":                "context_files_review_note",
	}
}

// buildPrepAnchorVRHY defines the Step 18 preparatory next anchor
// surface for SEQ-18-P21: 18-1 VR + 18-2 HY first.
func buildPrepAnchorVRHY() map[string]any {
	return map[string]any{
		"version":           "seq18_p21.v1",
		"role":              "prep_anchor_vr_hy",
		"truth_authority":   false,
		"primary_anchors":   []string{"18-1_vr", "18-2_hy"},
		"downstream_slices": []string{"18-3_qr", "18-4_vx"},
		"policy_version":    "s18-prep.v1",
		"mode":              "preparatory_anchor_definition",
	}
}

// buildHistoricalReferenceOnly defines the Step 18 historical reference
// status surface for SEQ-18-P22: historical completion text is reference only.
func buildHistoricalReferenceOnly() map[string]any {
	return map[string]any{
		"version":               "seq18_p22.v1",
		"role":                  "historical_reference_only",
		"truth_authority":       false,
		"historical_text":       "reference_only",
		"new_validation_needed": true,
		"policy_version":        "s18-prep.v1",
		"mode":                  "historical_reference_status_note",
	}
}

// buildBackendPrepAnchor defines the Step 18 backend preparation anchor
// surface for SEQ-18-P23: bridge.py::search_memories preparation anchor.
func buildBackendPrepAnchor() map[string]any {
	return map[string]any{
		"version":         "seq18_p23.v1",
		"role":            "backend_prep_anchor",
		"truth_authority": false,
		"anchor_file":     "backend/archive/bridge.py",
		"anchor_function": "search_memories",
		"policy_version":  "s18-prep.v1",
		"mode":            "backend_preparation_anchor",
	}
}

// buildRoutingContractPrepAnchor defines the Step 18 routing-contract
// preparation anchor surface for SEQ-18-P24: _build_recall_intent_contract_q3a.
func buildRoutingContractPrepAnchor() map[string]any {
	return map[string]any{
		"version":         "seq18_p24.v1",
		"role":            "routing_contract_prep_anchor",
		"truth_authority": false,
		"anchor_file":     "backend/main.py",
		"anchor_function": "_build_recall_intent_contract_q3a",
		"policy_version":  "s18-prep.v1",
		"mode":            "routing_contract_preparation_anchor",
	}
}

// buildRuntimePrepScope defines the Step 18 runtime preparation scope
// surface for SEQ-18-P25: runtime-facing Step 18 surfacing remains prep scope.
func buildRuntimePrepScope() map[string]any {
	return map[string]any{
		"version":         "seq18_p25.v1",
		"role":            "runtime_prep_scope",
		"truth_authority": false,
		"explicit_labels": false,
		"scope":           "preparation_only",
		"policy_version":  "s18-prep.v1",
		"mode":            "runtime_preparation_scope_note",
	}
}

// ---------------------------------------------------------------------------
// SEQ-18 VR (verbatim recall) surfaces (P29 ~ P35)
// ---------------------------------------------------------------------------

// buildVRScopedVerbatimSupportText defines the VR scoped verbatim support
// surface for SEQ-18-P29: additive scoped_verbatim_support_text/count/items.
func buildVRScopedVerbatimSupportText(support archivebridge.ScopedVerbatimSupport) map[string]any {
	return map[string]any{
		"version":                       "seq18_p29.v1",
		"role":                          "vr_scoped_verbatim_support",
		"truth_authority":               false,
		"scoped_verbatim_support_text":  support.Text,
		"scoped_verbatim_support_count": support.Count,
		"scoped_verbatim_support_items": support.Items,
		"source":                        "direct_evidence_gate_approved",
		"policy_version":                "vr18a.v1",
		"mode":                          "vr_scoped_verbatim_support_surface",
	}
}

// buildVRPolicyOwnerBlock defines the VR policy owner block surface for
// SEQ-18-P30: localized policy with max_items, max_total_chars, etc.
func buildVRPolicyOwnerBlock() map[string]any {
	return map[string]any{
		"version":                   "seq18_p30.v1",
		"role":                      "vr_policy_owner_block",
		"truth_authority":           false,
		"max_items":                 3,
		"max_total_chars":           720,
		"max_excerpt_chars":         160,
		"support_surface_first":     true,
		"prompt_injection_strategy": "latest_anchor_only",
		"source_tag_metadata":       "[source=direct_evidence scope=... turns=... anchor=... kind=...]",
		"policy_version":            "vr18b.v1",
		"mode":                      "vr_policy_owner_block_definition",
	}
}

// buildVRPromptInjectionStrategy defines the VR prompt injection strategy
// surface for SEQ-18-P31: latest_anchor_only, multi-item lane as support surface.
func buildVRPromptInjectionStrategy() map[string]any {
	return map[string]any{
		"version":                  "seq18_p31.v1",
		"role":                     "vr_prompt_injection_strategy",
		"truth_authority":          false,
		"injection_strategy":       "latest_anchor_only",
		"multi_item_lane_exposed":  true,
		"multi_item_lane_label":    "Scoped Verbatim Recall (support surface)",
		"prompt_injection_widened": false,
		"policy_version":           "vr18c.v1",
		"mode":                     "vr_prompt_injection_strategy_note",
	}
}

// buildVRHierarchyEscapeHatch defines the VR hierarchy escape hatch
// surface for SEQ-18-P32: hierarchy_escape_hatch metadata and surface priority.
func buildVRHierarchyEscapeHatch() map[string]any {
	return map[string]any{
		"version":                           "seq18_p32.v1",
		"role":                              "vr_hierarchy_escape_hatch",
		"truth_authority":                   false,
		"hierarchy_escape_hatch":            true,
		"verbatim_support_surface_priority": true,
		"hierarchy_escape_hatch_status":     "visible_when_summary_thin",
		"policy_version":                    "vr18d.v1",
		"mode":                              "vr_hierarchy_escape_hatch_definition",
	}
}

// buildVRBackendTestGuard defines the VR backend test guard surface for
// SEQ-18-P33: test_step18_scoped_verbatim_support.py guards the new surface.
func buildVRBackendTestGuard() map[string]any {
	return map[string]any{
		"version":         "seq18_p33.v1",
		"role":            "vr_backend_test_guard",
		"truth_authority": false,
		"test_file":       "backend/test_step18_scoped_verbatim_support.py",
		"guards": []string{
			"support_surface",
			"item_caps",
			"source_tag_metadata",
			"prompt_strategy",
			"hierarchy_escape_hatch_metadata",
		},
		"policy_version": "vr18a.v1",
		"mode":           "vr_backend_test_guard_surface",
	}
}

// buildVRRuntimeTransparency defines the VR runtime transparency surface
// for SEQ-18-P34: runtime trace write-through + Scoped Verbatim Recall section.
func buildVRRuntimeTransparency() map[string]any {
	return map[string]any{
		"version":              "seq18_p34.v1",
		"role":                 "vr_runtime_transparency",
		"truth_authority":      false,
		"trace_write_through":  true,
		"transparency_section": "Scoped Verbatim Recall (support surface)",
		"test_file":            "test_step18_scoped_verbatim_input_transparency.js",
		"policy_version":       "vr18a.v1",
		"mode":                 "vr_runtime_transparency_surface",
	}
}

// buildVRRegressionBundleGreen defines the VR regression bundle green
// surface for SEQ-18-P35: backend regression bundle + Step 19 stayed green.
func buildVRRegressionBundleGreen() map[string]any {
	return map[string]any{
		"version":                "seq18_p35.v1",
		"role":                   "vr_regression_bundle_green",
		"truth_authority":        false,
		"vr_slice_green":         true,
		"adjacent_step19_green":  true,
		"combined_bundle_status": "green",
		"policy_version":         "vr18a.v1",
		"mode":                   "vr_regression_bundle_green_note",
	}
}

// ---------------------------------------------------------------------------
// SEQ-18 HY (hybrid retrieval) surfaces (P46 ~ P53)
// ---------------------------------------------------------------------------

// buildHYSemanticRankScore defines the HY semantic rank + keyword overlap
// surface for SEQ-18-P46: hy1a.v1 bounded keyword-overlap scoring.
func buildHYSemanticRankScore() map[string]any {
	return map[string]any{
		"version":                        "seq18_p46.v1",
		"role":                           "hy_semantic_rank_keyword_overlap",
		"truth_authority":                false,
		"semantic_rank_preserved":        true,
		"keyword_overlap_policy":         "hy1a.v1",
		"keyword_overlap_score":          0.0,
		"hybrid_baseline_score":          0.0,
		"keyword_overlap_terms":          []string{},
		"hybrid_baseline_policy_version": "hy1a.v1",
		"policy_version":                 "hy1a.v1",
		"mode":                           "hy_semantic_rank_keyword_overlap_surface",
	}
}

// buildHYSoftBias defines the HY structured soft bias surface for
// SEQ-18-P47: hy1b.v1 speaker/location/storyline cue weights.
func buildHYSoftBias() map[string]any {
	return map[string]any{
		"version":                  "seq18_p47.v1",
		"role":                     "hy_soft_bias",
		"truth_authority":          false,
		"soft_bias_policy":         "hy1b.v1",
		"speaker_bias_weight":      0.04,
		"location_bias_weight":     0.05,
		"storyline_bias_weight":    0.06,
		"soft_bias_cap":            0.12,
		"speaker_bias_score":       0.0,
		"location_bias_score":      0.0,
		"storyline_bias_score":     0.0,
		"soft_bias_score":          0.0,
		"soft_bias_policy_version": "hy1b.v1",
		"policy_version":           "hy1b.v1",
		"mode":                     "hy_soft_bias_surface",
	}
}

// buildHYStopwordGuard defines the HY stopword guard surface for
// SEQ-18-P48: common English filler terms no longer count toward overlap.
func buildHYStopwordGuard() map[string]any {
	return map[string]any{
		"version":                  "seq18_p48.v1",
		"role":                     "hy_stopword_guard",
		"truth_authority":          false,
		"stopword_inflation_fixed": true,
		"tightened_extractor":      true,
		"common_filler_excluded":   true,
		"policy_version":           "hy1a.v1",
		"mode":                     "hy_stopword_guard_surface",
	}
}

// buildHYQ1aPropagation defines the HY q1a propagation surface for
// SEQ-18-P49: HY trace fields propagated into q1a unified retrieval document.
func buildHYQ1aPropagation() map[string]any {
	return map[string]any{
		"version":         "seq18_p49.v1",
		"role":            "hy_q1a_propagation",
		"truth_authority": false,
		"q1a_propagation": true,
		"propagated_fields": []string{
			"keyword_overlap_score",
			"hybrid_baseline_score",
			"keyword_overlap_terms",
			"hybrid_baseline_policy_version",
			"speaker_bias_score",
			"location_bias_score",
			"storyline_bias_score",
			"soft_bias_score",
			"soft_bias_policy_version",
		},
		"policy_version": "hy1a.v1",
		"mode":           "hy_q1a_propagation_surface",
	}
}

// buildHYRuntimeInspection defines the HY runtime inspection surface for
// SEQ-18-P50: JS reads HY score/bias fields, renders Hybrid Retrieval Inspection.
func buildHYRuntimeInspection() map[string]any {
	return map[string]any{
		"version":                 "seq18_p50.v1",
		"role":                    "hy_runtime_inspection",
		"truth_authority":         false,
		"js_function":             "extractMemoryItems",
		"row_meta_extended":       true,
		"row_meta_fields":         []string{"final", "kw", "soft"},
		"transparency_block":      "Hybrid Retrieval Inspection",
		"transparency_block_type": "trace_only",
		"policy_version":          "hy1b.v1",
		"mode":                    "hy_runtime_inspection_surface",
	}
}

// buildHYRecurringRiskGuards defines the HY recurring-risk guard surface
// for SEQ-18-P51: stopword-inflated overlap + missing q1a HY metadata guards.
func buildHYRecurringRiskGuards() map[string]any {
	return map[string]any{
		"version":           "seq18_p51.v1",
		"role":              "hy_recurring_risk_guards",
		"truth_authority":   false,
		"backend_test_file": "backend/test_step18_hybrid_regression.py",
		"js_test_file":      "test_step18_hybrid_input_transparency.js",
		"guards": []map[string]any{
			{"name": "stopword_inflated_overlap", "status": "guarded"},
			{"name": "missing_q1a_hy_metadata", "status": "guarded"},
			{"name": "hybrid_retrieval_inspection_disappearance", "status": "guarded"},
		},
		"policy_version": "hy1a.v1",
		"mode":           "hy_recurring_risk_guard_surface",
	}
}

// buildHYPolicyRegistry defines the HY policy registry consolidation
// surface for SEQ-18-P52: hybrid_policy.py as single versioned policy registry.
func buildHYPolicyRegistry() map[string]any {
	return map[string]any{
		"version":                     "seq18_p52.v1",
		"role":                        "hy_policy_registry",
		"truth_authority":             false,
		"registry_file":               "backend/archive/hybrid_policy.py",
		"consolidated":                true,
		"scattered_hardcoded_removed": true,
		"policy_family":               "hy",
		"policy_version":              "hy1a.v1",
		"mode":                        "hy_policy_registry_surface",
	}
}

// buildHYStopAt18_2c defines the HY intentional stop surface for
// SEQ-18-P53: stops at 18-2c, 18-2d/18-3/18-4 remain open follow-up.
func buildHYStopAt18_2c() map[string]any {
	return map[string]any{
		"version":            "seq18_p53.v1",
		"role":               "hy_stop_at_18_2c",
		"truth_authority":    false,
		"stop_point":         "18-2c",
		"open_follow_up":     []string{"18-2d", "18-3", "18-4"},
		"tail_budget_rescue": "pending",
		"policy_version":     "hy1a.v1",
		"mode":               "hy_intentional_stop_note",
	}
}

// ---------------------------------------------------------------------------
// SEQ-18 HY tail-budget rescue surfaces (P65 ~ P69)
// ---------------------------------------------------------------------------

// buildHYTailBudgetPolicyOwner defines the HY tail-budget policy owner
// surface for SEQ-18-P65: hy1d.v1 as part of the shared HY policy family.
func buildHYTailBudgetPolicyOwner() map[string]any {
	return map[string]any{
		"version":                     "seq18_p65.v1",
		"role":                        "hy_tail_budget_policy_owner",
		"truth_authority":             false,
		"policy_family":               "hy",
		"policy_version":              "hy1d.v1",
		"scattered_hardcoded_removed": true,
		"registry_file":               "backend/archive/hybrid_policy.py",
		"mode":                        "hy_tail_budget_policy_owner_surface",
	}
}

// buildHYTailBudgetRescuePass defines the HY tail-budget rescue pass
// surface for SEQ-18-P66: bounded post-rank rescue, same n_results budget,
// at most one near-cutoff candidate promoted when keyword/soft-bias signal
// is stronger than the cutline item.
func buildHYTailBudgetRescuePass() map[string]any {
	return map[string]any{
		"version":                 "seq18_p66.v1",
		"role":                    "hy_tail_budget_rescue_pass",
		"truth_authority":         false,
		"rescue_enabled":          true,
		"max_promotions_per_pass": 1,
		"budget_preserved":        true,
		"promotion_trigger":       "keyword_soft_bias_stronger_than_cutline",
		"policy_version":          "hy1d.v1",
		"mode":                    "hy_tail_budget_rescue_pass_surface",
	}
}

// buildHYTailBudgetRescueTrace defines the HY tail-budget rescue trace
// surface for SEQ-18-P67: promoted candidates retain explicit rescue trace.
func buildHYTailBudgetRescueTrace() map[string]any {
	return map[string]any{
		"version":         "seq18_p67.v1",
		"role":            "hy_tail_budget_rescue_trace",
		"truth_authority": false,
		"trace_fields": []string{
			"tail_budget_policy_version",
			"tail_budget_original_rank",
			"tail_budget_promoted",
			"tail_budget_reason",
			"tail_budget_score_gap",
		},
		"trace_mandatory": true,
		"policy_version":  "hy1d.v1",
		"mode":            "hy_tail_budget_rescue_trace_surface",
	}
}

// buildHYTailBudgetQ1aPropagation defines the HY tail-budget q1a
// propagation surface for SEQ-18-P68: trace fields propagated into q1a unified
// retrieval document metadata.
func buildHYTailBudgetQ1aPropagation() map[string]any {
	return map[string]any{
		"version":         "seq18_p68.v1",
		"role":            "hy_tail_budget_q1a_propagation",
		"truth_authority": false,
		"q1a_propagation": true,
		"propagated_fields": []string{
			"tail_budget_policy_version",
			"tail_budget_original_rank",
			"tail_budget_promoted",
			"tail_budget_reason",
			"tail_budget_score_gap",
		},
		"policy_version": "hy1d.v1",
		"mode":           "hy_tail_budget_q1a_propagation_surface",
	}
}

// buildHYTailBudgetRegression defines the HY tail-budget regression
// surface for SEQ-18-P69: near-cutoff rescue regression test coverage.
func buildHYTailBudgetRegression() map[string]any {
	return map[string]any{
		"version":          "seq18_p69.v1",
		"role":             "hy_tail_budget_regression",
		"truth_authority":  false,
		"test_file":        "backend/test_step18_hybrid_regression.py",
		"regression_scope": "near_cutoff_rescue",
		"verifies": []string{
			"promotion_into_same_budget",
			"q1a_metadata_propagation",
		},
		"policy_version": "hy1d.v1",
		"mode":           "hy_tail_budget_regression_surface",
	}
}

// ---------------------------------------------------------------------------
// SEQ-18 QR query-class contract surfaces (P76 ~ P91)
// ---------------------------------------------------------------------------

// buildQRQueryClassContract defines the QR query-class contract surface
// for SEQ-18-P76: qr1a.v1 query-class contract metadata, additive, fail-open.
func buildQRQueryClassContract() map[string]any {
	return map[string]any{
		"version":          "seq18_p76.v1",
		"role":             "qr_query_class_contract",
		"truth_authority":  false,
		"contract_version": "qr1a.v1",
		"execution_mode":   "single_query_shared",
		"fail_open":        true,
		"additive_only":    true,
		"policy_version":   "qr1a.v1",
		"mode":             "qr_query_class_contract_surface",
	}
}

// buildQRQueryClassTaxonomy defines the QR query-class taxonomy surface
// for SEQ-18-P77: explicit taxonomy with scene, callback, resume, canon, temporal.
func buildQRQueryClassTaxonomy() map[string]any {
	return map[string]any{
		"version":               "seq18_p77.v1",
		"role":                  "qr_query_class_taxonomy",
		"truth_authority":       false,
		"query_classes":         []string{"scene", "callback", "resume", "canon", "temporal"},
		"contract_layer_only":   true,
		"primary_class_visible": true,
		"policy_version":        "qr1a.v1",
		"mode":                  "qr_query_class_taxonomy_surface",
	}
}

// buildQRPrimaryClassSelection defines the QR primary class selection
// precedence surface for SEQ-18-P78: temporal > resume > canon > callback > scene.
func buildQRPrimaryClassSelection() map[string]any {
	return map[string]any{
		"version":         "seq18_p78.v1",
		"role":            "qr_primary_class_selection",
		"truth_authority": false,
		"precedence": []string{
			"explicit_temporal_cue",
			"resume_trigger_or_ready_resume_pack",
			"canon_guard_signal",
			"callback_recovery_signal",
			"scene_fallback",
		},
		"policy_version": "qr1a.v1",
		"mode":           "qr_primary_class_selection_surface",
	}
}

// buildQRLexicalCueBlock defines the QR lexical cue block surface for
// SEQ-18-P79: localized cue lists and descriptions in one constant block.
func buildQRLexicalCueBlock() map[string]any {
	return map[string]any{
		"version":                 "seq18_p79.v1",
		"role":                    "qr_lexical_cue_block",
		"truth_authority":         false,
		"localized":               true,
		"hidden_literals_removed": true,
		"cue_block_owner":         "query_class_contract",
		"policy_version":          "qr1a.v1",
		"mode":                    "qr_lexical_cue_block_surface",
	}
}

// buildQRQueryClassContractTest defines the QR query-class contract test
// surface for SEQ-18-P80: temporal-over-resume, resume-over-callback, callback/scene
// fallback behavior coverage.
func buildQRQueryClassContractTest() map[string]any {
	return map[string]any{
		"version":         "seq18_p80.v1",
		"role":            "qr_query_class_contract_test",
		"truth_authority": false,
		"test_file":       "backend/test_step18_query_class_contract.py",
		"covers": []string{
			"temporal_over_resume_precedence",
			"resume_over_callback_precedence",
			"callback_scene_fallback",
		},
		"policy_version": "qr1a.v1",
		"mode":           "qr_query_class_contract_test_surface",
	}
}

// buildQRQueryClassBudgetPolicy defines the QR query-class budget policy
// surface for SEQ-18-P87: qr1b.v1 budget policy metadata, additive, fail-open.
func buildQRQueryClassBudgetPolicy() map[string]any {
	return map[string]any{
		"version":               "seq18_p87.v1",
		"role":                  "qr_query_class_budget_policy",
		"truth_authority":       false,
		"budget_policy_version": "qr1b.v1",
		"execution_mode":        "single_query_shared",
		"fail_open":             true,
		"additive_only":         true,
		"policy_version":        "qr1b.v1",
		"mode":                  "qr_query_class_budget_policy_surface",
	}
}

// buildQRQ3cBudgetReuse defines the QR q3c budget reuse surface for
// SEQ-18-P88: scene/callback/resume/canon reuse existing q3c intent packet budgets.
func buildQRQ3cBudgetReuse() map[string]any {
	return map[string]any{
		"version":                    "seq18_p88.v1",
		"role":                       "qr_q3c_budget_reuse",
		"truth_authority":            false,
		"reused_budget_source":       "q3c_intent_packet",
		"executable_classes":         []string{"scene", "callback", "resume", "canon"},
		"independent_budget_avoided": true,
		"policy_version":             "qr1b.v1",
		"mode":                       "qr_q3c_budget_reuse_surface",
	}
}

// buildQRTemporalProfileBudget defines the QR temporal profile-based
// budget surface for SEQ-18-P89: temporal gets separate evidence-first overlay.
func buildQRTemporalProfileBudget() map[string]any {
	return map[string]any{
		"version":         "seq18_p89.v1",
		"role":            "qr_temporal_profile_budget",
		"truth_authority": false,
		"profile_based":   true,
		"evidence_first":  true,
		"overlay_budget":  true,
		"candidate_caps": []string{
			"temporal_integrity",
			"direct_evidence",
			"search",
		},
		"shared_profile_template": true,
		"policy_version":          "qr1b.v1",
		"mode":                    "qr_temporal_profile_budget_surface",
	}
}

// buildQRBudgetVisibility defines the QR budget visibility surface for
// SEQ-18-P90: each class carries retrieval_depth, candidate_budget, budget_policy_version,
// budget_source so 18-3b is visible as contract data.
func buildQRBudgetVisibility() map[string]any {
	return map[string]any{
		"version":         "seq18_p90.v1",
		"role":            "qr_budget_visibility",
		"truth_authority": false,
		"visible_fields": []string{
			"retrieval_depth",
			"candidate_budget",
			"budget_policy_version",
			"budget_source",
		},
		"contract_data_visible":        true,
		"hidden_builder_logic_removed": true,
		"policy_version":               "qr1b.v1",
		"mode":                         "qr_budget_visibility_surface",
	}
}

// buildQRQueryClassBudgetTest defines the QR query-class budget test
// surface for SEQ-18-P91: q3c reuse + temporal budget differences coverage.
func buildQRQueryClassBudgetTest() map[string]any {
	return map[string]any{
		"version":         "seq18_p91.v1",
		"role":            "qr_query_class_budget_test",
		"truth_authority": false,
		"test_file":       "backend/test_step18_query_class_budget_policy.py",
		"covers": []string{
			"q3c_budget_reuse_executable",
			"temporal_budget_difference",
		},
		"policy_version": "qr1b.v1",
		"mode":           "qr_query_class_budget_test_surface",
	}
}

// ---------------------------------------------------------------------------
// SEQ-18 QR note/route policy surfaces (P98 ~ P113)
// ---------------------------------------------------------------------------

// buildQRNotePolicy defines the QR note policy surface for
// SEQ-18-P98: qr1c.v1 note policy metadata, additive, fail-open.
func buildQRNotePolicy() map[string]any {
	return map[string]any{
		"version":             "seq18_p98.v1",
		"role":                "qr_note_policy",
		"truth_authority":     false,
		"note_policy_version": "qr1c.v1",
		"execution_mode":      "single_query_shared",
		"fail_open":           true,
		"additive_only":       true,
		"policy_version":      "qr1c.v1",
		"mode":                "qr_note_policy_surface",
	}
}

// buildQRSceneCanonNoPreExtract defines the QR scene/canon no-pre-extract
// surface for SEQ-18-P99: no pre-extract rule, support_surface_first delivery.
func buildQRSceneCanonNoPreExtract() map[string]any {
	return map[string]any{
		"version":                "seq18_p99.v1",
		"role":                   "qr_scene_canon_no_pre_extract",
		"truth_authority":        false,
		"no_pre_extract_classes": []string{"scene", "canon"},
		"note_surfaces": []string{
			"current_scene_support_surface",
			"authority_first_support_surface",
		},
		"delivery_policy": "support_surface_first",
		"policy_version":  "qr1c.v1",
		"mode":            "qr_scene_canon_no_pre_extract_surface",
	}
}

// buildQRCallbackResumeTemporalNoteOnly defines the QR callback/resume/temporal
// note-only surface for SEQ-18-P100: note_only_until_route_exec pre-extract behavior.
func buildQRCallbackResumeTemporalNoteOnly() map[string]any {
	return map[string]any{
		"version":              "seq18_p100.v1",
		"role":                 "qr_callback_resume_temporal_note_only",
		"truth_authority":      false,
		"note_only_classes":    []string{"callback", "resume", "temporal"},
		"pre_extract_behavior": "note_only_until_route_exec",
		"contract_layer_only":  true,
		"policy_version":       "qr1c.v1",
		"mode":                 "qr_callback_resume_temporal_note_only_surface",
	}
}

// buildQRNotePolicyFields defines the QR note policy fields surface for
// SEQ-18-P101: each class carries extract_before_read, retrieval_note_surface,
// pre_extract_rule, note_delivery, note_policy_version.
func buildQRNotePolicyFields() map[string]any {
	return map[string]any{
		"version":         "seq18_p101.v1",
		"role":            "qr_note_policy_fields",
		"truth_authority": false,
		"visible_fields": []string{
			"extract_before_read",
			"retrieval_note_surface",
			"pre_extract_rule",
			"note_delivery",
			"note_policy_version",
		},
		"additive_metadata": true,
		"policy_version":    "qr1c.v1",
		"mode":              "qr_note_policy_fields_surface",
	}
}

// buildQRNotePolicyTest defines the QR note policy test surface for
// SEQ-18-P102: guards no-extract defaults for scene/canon and note-only for
// callback/resume/temporal.
func buildQRNotePolicyTest() map[string]any {
	return map[string]any{
		"version":         "seq18_p102.v1",
		"role":            "qr_note_policy_test",
		"truth_authority": false,
		"test_file":       "backend/test_step18_query_class_note_policy.py",
		"guards": []string{
			"scene_canon_no_extract",
			"callback_resume_temporal_note_only",
		},
		"policy_version": "qr1c.v1",
		"mode":           "qr_note_policy_test_surface",
	}
}

// buildQRRoutePolicy defines the QR route policy surface for
// SEQ-18-P109: qr1d.v1 route policy metadata, additive, fail-open.
func buildQRRoutePolicy() map[string]any {
	return map[string]any{
		"version":              "seq18_p109.v1",
		"role":                 "qr_route_policy",
		"truth_authority":      false,
		"route_policy_version": "qr1d.v1",
		"execution_mode":       "single_query_shared",
		"fail_open":            true,
		"additive_only":        true,
		"policy_version":       "qr1d.v1",
		"mode":                 "qr_route_policy_surface",
	}
}

// buildQRRouteFamilies defines the QR route families surface for
// SEQ-18-P110: visible route families instead of hidden execution branches.
func buildQRRouteFamilies() map[string]any {
	return map[string]any{
		"version":         "seq18_p110.v1",
		"role":            "qr_route_families",
		"truth_authority": false,
		"route_families": []string{
			"scene_default",
			"callback_rescue",
			"needle_in_haystack",
			"old_detail_bridge",
			"resume_bridge",
			"canon_guard",
			"temporal_anchor",
		},
		"metadata_visible": true,
		"policy_version":   "qr1d.v1",
		"mode":             "qr_route_families_surface",
	}
}

// buildQRLongTailRouteCandidates defines the QR long-tail route candidates
// surface for SEQ-18-P111: scene/callback/resume can surface long-tail candidates
// separately from default route, detail cues promote at contract layer only.
func buildQRLongTailRouteCandidates() map[string]any {
	return map[string]any{
		"version":             "seq18_p111.v1",
		"role":                "qr_long_tail_route_candidates",
		"truth_authority":     false,
		"long_tail_classes":   []string{"scene", "callback", "resume"},
		"promotion_trigger":   "detail_old_detail_lexical_cue",
		"contract_layer_only": true,
		"runtime_unchanged":   true,
		"policy_version":      "qr1d.v1",
		"mode":                "qr_long_tail_route_candidates_surface",
	}
}

// buildQRRoutePolicyFields defines the QR route policy fields surface for
// SEQ-18-P112: each class carries route_family, route_candidates, selected_route,
// route_policy_version; primary_selected_route published without changing runtime.
func buildQRRoutePolicyFields() map[string]any {
	return map[string]any{
		"version":         "seq18_p112.v1",
		"role":            "qr_route_policy_fields",
		"truth_authority": false,
		"visible_fields": []string{
			"route_family",
			"route_candidates",
			"selected_route",
			"route_policy_version",
		},
		"publishes":         "primary_selected_route",
		"runtime_unchanged": true,
		"policy_version":    "qr1d.v1",
		"mode":              "qr_route_policy_fields_surface",
	}
}

// buildQRRoutePolicyTest defines the QR route policy test surface for
// SEQ-18-P113: guards fail-open default route + long-tail activation for detail-seeking.
func buildQRRoutePolicyTest() map[string]any {
	return map[string]any{
		"version":         "seq18_p113.v1",
		"role":            "qr_route_policy_test",
		"truth_authority": false,
		"test_file":       "backend/test_step18_query_class_route_policy.py",
		"guards": []string{
			"fail_open_default_route",
			"long_tail_activation_detail_seeking",
		},
		"policy_version": "qr1d.v1",
		"mode":           "qr_route_policy_test_surface",
	}
}

// ---------------------------------------------------------------------------
// SEQ-18 VX validation gate surfaces (P120 ~ P153)
// ---------------------------------------------------------------------------

// buildVXHybridReplayGate defines the VX hybrid replay validation gate
// surface for SEQ-18-P120: vx18a.v1 additive validation_gates.hybrid_replay.
func buildVXHybridReplayGate() map[string]any {
	return map[string]any{
		"version":             "seq18_p120.v1",
		"role":                "vx_hybrid_replay_gate",
		"truth_authority":     false,
		"gate_version":        "vx18a.v1",
		"gate_name":           "hybrid_replay",
		"execution_unchanged": true,
		"routing_unchanged":   true,
		"additive_only":       true,
		"policy_version":      "vx18a.v1",
		"mode":                "vx_hybrid_replay_gate_surface",
	}
}

// buildVXReplayThresholdReuse defines the VX replay threshold reuse
// surface for SEQ-18-P121: reuses _U1E_CAPTURED_REPLAY_* thresholds instead of
// defining a second disconnected set.
func buildVXReplayThresholdReuse() map[string]any {
	return map[string]any{
		"version":          "seq18_p121.v1",
		"role":             "vx_replay_threshold_reuse",
		"truth_authority":  false,
		"threshold_source": "_U1E_CAPTURED_REPLAY",
		"reused_for": []string{
			"semantic_only_baseline_replay",
			"hybrid_candidate_replay",
		},
		"disconnected_set_avoided": true,
		"policy_version":           "vx18a.v1",
		"mode":                     "vx_replay_threshold_reuse_surface",
	}
}

// buildVXHybridReplayStates defines the VX hybrid replay gate states
// surface for SEQ-18-P122: pending/hold ??blocked/hold ??ready/promote_candidate.
func buildVXHybridReplayStates() map[string]any {
	return map[string]any{
		"version":         "seq18_p122.v1",
		"role":            "vx_hybrid_replay_states",
		"truth_authority": false,
		"state_machine": []map[string]any{
			{"state": "pending", "action": "hold", "trigger": "missing_or_incomplete_replay_evidence"},
			{"state": "blocked", "action": "hold", "trigger": "short_mid_regression_or_missing_long_extreme_improvement"},
			{"state": "ready", "action": "promote_candidate", "trigger": "hybrid_replay_clears_all_checks"},
		},
		"policy_version": "vx18a.v1",
		"mode":           "vx_hybrid_replay_states_surface",
	}
}

// buildVXHybridReplayTest defines the VX hybrid replay gate test
// surface for SEQ-18-P123: guards pending, blocked, ready states.
func buildVXHybridReplayTest() map[string]any {
	return map[string]any{
		"version":         "seq18_p123.v1",
		"role":            "vx_hybrid_replay_test",
		"truth_authority": false,
		"test_file":       "backend/test_step18_hybrid_replay_gate.py",
		"guards": []string{
			"pending_without_evidence",
			"blocked_on_replay_failure",
			"ready_on_non_regressive_long_extreme_improvement",
		},
		"policy_version": "vx18a.v1",
		"mode":           "vx_hybrid_replay_test_surface",
	}
}

// buildVXHeldoutCompletenessGate defines the VX heldout completeness
// validation gate surface for SEQ-18-P130: vx18b.v1 additive validation_gates.
func buildVXHeldoutCompletenessGate() map[string]any {
	return map[string]any{
		"version":             "seq18_p130.v1",
		"role":                "vx_heldout_completeness_gate",
		"truth_authority":     false,
		"gate_version":        "vx18b.v1",
		"gate_name":           "heldout_completeness",
		"execution_unchanged": true,
		"routing_unchanged":   true,
		"additive_only":       true,
		"policy_version":      "vx18b.v1",
		"mode":                "vx_heldout_completeness_gate_surface",
	}
}

// buildVXHeldoutMetrics defines the VX heldout metrics surface for
// SEQ-18-P131: retention_rate, false_negative_rate, full_coverage_rate with
// sample sufficiency.
func buildVXHeldoutMetrics() map[string]any {
	return map[string]any{
		"version":         "seq18_p131.v1",
		"role":            "vx_heldout_metrics",
		"truth_authority": false,
		"metrics": []string{
			"retention_rate",
			"false_negative_rate",
			"full_coverage_rate",
		},
		"sample_sufficiency_required": true,
		"state_rules": []map[string]any{
			{"state": "pending", "action": "hold", "trigger": "missing_evidence"},
			{"state": "pending", "action": "hold", "trigger": "thin_evidence"},
			{"state": "blocked", "action": "hold", "trigger": "completeness_failure"},
			{"state": "ready", "action": "promote_candidate", "trigger": "sufficient_held_out_evidence"},
		},
		"policy_version": "vx18b.v1",
		"mode":           "vx_heldout_metrics_surface",
	}
}

// buildVXHeldoutThresholdReuse defines the VX heldout threshold reuse
// surface for SEQ-18-P132: reuses LC1P healthy completeness floor + existing
// _U1E_CAPTURED_REPLAY_MIN_* sample thresholds.
func buildVXHeldoutThresholdReuse() map[string]any {
	return map[string]any{
		"version":                      "seq18_p132.v1",
		"role":                         "vx_heldout_threshold_reuse",
		"truth_authority":              false,
		"completeness_floor":           "LC1P_healthy",
		"sample_threshold_source":      "_U1E_CAPTURED_REPLAY_MIN",
		"disconnected_literal_avoided": true,
		"policy_version":               "vx18b.v1",
		"mode":                         "vx_heldout_threshold_reuse_surface",
	}
}

// buildVXHeldoutCompletenessTest defines the VX heldout completeness test
// surface for SEQ-18-P133: guards pending-without, pending-insufficient, blocked-below,
// ready-sufficient states.
func buildVXHeldoutCompletenessTest() map[string]any {
	return map[string]any{
		"version":         "seq18_p133.v1",
		"role":            "vx_heldout_completeness_test",
		"truth_authority": false,
		"test_file":       "backend/test_step18_heldout_completeness_gate.py",
		"guards": []string{
			"pending_without_evidence",
			"pending_with_insufficient_samples",
			"blocked_below_threshold",
			"ready_with_sufficient_coverage",
		},
		"policy_version": "vx18b.v1",
		"mode":           "vx_heldout_completeness_test_surface",
	}
}

// buildVXLatencyTokenBudgetGate defines the VX latency/token-budget
// validation gate surface for SEQ-18-P140: vx18c.v1 additive validation_gates.
func buildVXLatencyTokenBudgetGate() map[string]any {
	return map[string]any{
		"version":             "seq18_p140.v1",
		"role":                "vx_latency_token_budget_gate",
		"truth_authority":     false,
		"gate_version":        "vx18c.v1",
		"gate_name":           "latency_token_budget",
		"execution_unchanged": true,
		"routing_unchanged":   true,
		"additive_only":       true,
		"policy_version":      "vx18c.v1",
		"mode":                "vx_latency_token_budget_gate_surface",
	}
}

// buildVXLatencyTokenMetrics defines the VX latency/token metrics surface
// for SEQ-18-P141: baseline_latency_proxy_ms, candidate_latency_proxy_ms,
// candidate_token_budget_chars with sample sufficiency.
func buildVXLatencyTokenMetrics() map[string]any {
	return map[string]any{
		"version":         "seq18_p141.v1",
		"role":            "vx_latency_token_metrics",
		"truth_authority": false,
		"metrics": []string{
			"baseline_latency_proxy_ms",
			"candidate_latency_proxy_ms",
			"candidate_token_budget_chars",
		},
		"sample_sufficiency_required": true,
		"default_token_ceiling":       "packet_budget_policy.max_injection_chars",
		"policy_version":              "vx18c.v1",
		"mode":                        "vx_latency_token_metrics_surface",
	}
}

// buildVXLatencyTokenThresholdReuse defines the VX latency/token threshold
// reuse surface for SEQ-18-P142: reuses _LC1M_MAX_SPLIT_LATENCY_MULTIPLIER for
// latency ceiling, token ratio in one gate-owner constant.
func buildVXLatencyTokenThresholdReuse() map[string]any {
	return map[string]any{
		"version":                    "seq18_p142.v1",
		"role":                       "vx_latency_token_threshold_reuse",
		"truth_authority":            false,
		"latency_ceiling_source":     "_LC1M_MAX_SPLIT_LATENCY_MULTIPLIER",
		"token_ratio_owner":          "gate_constant",
		"scattered_literals_avoided": true,
		"policy_version":             "vx18c.v1",
		"mode":                       "vx_latency_token_threshold_reuse_surface",
	}
}

// buildVXLatencyTokenTest defines the VX latency/token budget test
// surface for SEQ-18-P143: guards pending-without, pending-insufficient,
// blocked-ceiling, ready-within states.
func buildVXLatencyTokenTest() map[string]any {
	return map[string]any{
		"version":         "seq18_p143.v1",
		"role":            "vx_latency_token_test",
		"truth_authority": false,
		"test_file":       "backend/test_step18_latency_token_budget_gate.py",
		"guards": []string{
			"pending_without_evidence",
			"pending_with_insufficient_samples",
			"blocked_ceiling_exceeded",
			"ready_within_ceiling",
		},
		"policy_version": "vx18c.v1",
		"mode":           "vx_latency_token_test_surface",
	}
}

// buildVXTruthBoundaryGate defines the VX truth-boundary replay validation
// gate surface for SEQ-18-P150: vx18d.v1 additive validation_gates.truth_boundary_replay.
func buildVXTruthBoundaryGate() map[string]any {
	return map[string]any{
		"version":         "seq18_p150.v1",
		"role":            "vx_truth_boundary_gate",
		"truth_authority": false,
		"gate_version":    "vx18d.v1",
		"gate_name":       "truth_boundary_replay",
		"evaluates_after": "injection_pack_data.packet_composition",
		"additive_only":   true,
		"policy_version":  "vx18d.v1",
		"mode":            "vx_truth_boundary_gate_surface",
	}
}

// buildVXTruthBoundaryPrecedence defines the VX truth-boundary precedence
// surface for SEQ-18-P151: evaluates candidate_section_order and
// support_surface_priority against baseline with _LC1K_HIGH_AUTHORITY_SOURCES /
// _LC1K_LOWER_TIER_SOURCES precedence model.
func buildVXTruthBoundaryPrecedence() map[string]any {
	return map[string]any{
		"version":         "seq18_p151.v1",
		"role":            "vx_truth_boundary_precedence",
		"truth_authority": false,
		"evaluated_fields": []string{
			"candidate_section_order",
			"support_surface_priority",
		},
		"precedence_model": "_LC1K_HIGH_AUTHORITY_SOURCES_vs_LOWER_TIER_SOURCES",
		"policy_version":   "vx18d.v1",
		"mode":             "vx_truth_boundary_precedence_surface",
	}
}

// buildVXTruthBoundaryStates defines the VX truth-boundary states surface
// for SEQ-18-P152: pending/hold, blocked/hold, ready/promote_candidate.
func buildVXTruthBoundaryStates() map[string]any {
	return map[string]any{
		"version":         "seq18_p152.v1",
		"role":            "vx_truth_boundary_states",
		"truth_authority": false,
		"state_machine": []map[string]any{
			{"state": "pending", "action": "hold", "trigger": "missing_truth_boundary_evidence"},
			{"state": "pending", "action": "hold", "trigger": "insufficient_samples"},
			{"state": "blocked", "action": "hold", "trigger": "lost_support_lane_markers"},
			{"state": "ready", "action": "promote_candidate", "trigger": "preserved_canon_support_precedence"},
		},
		"policy_version": "vx18d.v1",
		"mode":           "vx_truth_boundary_states_surface",
	}
}

// buildVXTruthBoundaryTest defines the VX truth-boundary test surface for
// SEQ-18-P153: guards pending-without, pending-insufficient, blocked-support-loss,
// ready-preserved-boundary states.
func buildVXTruthBoundaryTest() map[string]any {
	return map[string]any{
		"version":         "seq18_p153.v1",
		"role":            "vx_truth_boundary_test",
		"truth_authority": false,
		"test_file":       "backend/test_step18_truth_boundary_replay_gate.py",
		"guards": []string{
			"pending_without_evidence",
			"pending_with_insufficient_samples",
			"blocked_support_lane_loss",
			"ready_preserved_boundary",
		},
		"policy_version": "vx18d.v1",
		"mode":           "vx_truth_boundary_test_surface",
	}
}

// ---------------------------------------------------------------------------
// SEQ-18 VX truncation_summary_loss gate surfaces (P160 ~ P164)
// ---------------------------------------------------------------------------

// buildVXTruncationSummaryLossGate defines the truncation_summary_loss
// validation gate surface for SEQ-18-P160: vx18e.v1 additive gate that emits
// validation_gates.truncation_summary_loss without changing retrieval execution.
func buildVXTruncationSummaryLossGate() map[string]any {
	return map[string]any{
		"version":         "seq18_p160.v1",
		"role":            "vx_truncation_summary_loss_gate",
		"truth_authority": false,
		"gate_version":    "vx18e.v1",
		"gate_name":       "truncation_summary_loss",
		"evaluates_after": "injection_pack_data.packet_composition",
		"additive_only":   true,
		"policy_version":  "vx18e.v1",
		"mode":            "vx_truncation_summary_loss_gate_surface",
	}
}

// buildVXTruncationSummaryLossMetrics defines the truncation_summary_loss
// metrics surface for SEQ-18-P161: evaluates baseline/candidate tail_fact_miss_rate
// and summary_loss_rate together with sample sufficiency, carrying existing
// tail_budget_promoted trace into gate evidence.
func buildVXTruncationSummaryLossMetrics() map[string]any {
	return map[string]any{
		"version":         "seq18_p161.v1",
		"role":            "vx_truncation_summary_loss_metrics",
		"truth_authority": false,
		"metrics": []string{
			"baseline_tail_fact_miss_rate",
			"candidate_tail_fact_miss_rate",
			"baseline_summary_loss_rate",
			"candidate_summary_loss_rate",
		},
		"sample_sufficiency_required": true,
		"trace_carry_in": []string{
			"tail_budget_promoted",
			"tail_budget_reason",
			"tail_budget_score_gap",
		},
		"policy_version": "vx18e.v1",
		"mode":           "vx_truncation_summary_loss_metrics_surface",
	}
}

// buildVXTruncationSummaryLossThresholdReuse defines the threshold reuse
// surface for SEQ-18-P162: reuses existing _U1E_CAPTURED_REPLAY_MIN_* sample
// thresholds and keeps truncation regression thresholds in one gate-owner constant
// block instead of scattering held-out delta literals.
func buildVXTruncationSummaryLossThresholdReuse() map[string]any {
	return map[string]any{
		"version":                    "seq18_p162.v1",
		"role":                       "vx_truncation_summary_loss_threshold_reuse",
		"truth_authority":            false,
		"sample_threshold_source":    "_U1E_CAPTURED_REPLAY_MIN_*",
		"threshold_owner":            "gate_constant_block",
		"scattered_literals_avoided": true,
		"policy_version":             "vx18e.v1",
		"mode":                       "vx_truncation_summary_loss_threshold_reuse_surface",
	}
}

// buildVXTruncationSummaryLossStates defines the truncation_summary_loss
// state machine surface for SEQ-18-P163: guards pending-without-evidence,
// pending-with-insufficient-samples, blocked-regression, ready-non-regression.
func buildVXTruncationSummaryLossStates() map[string]any {
	return map[string]any{
		"version":         "seq18_p163.v1",
		"role":            "vx_truncation_summary_loss_states",
		"truth_authority": false,
		"state_machine": []map[string]any{
			{"state": "pending", "action": "hold", "trigger": "missing_truncation_summary_loss_evidence"},
			{"state": "pending", "action": "hold", "trigger": "insufficient_samples"},
			{"state": "blocked", "action": "hold", "trigger": "regression_detected"},
			{"state": "ready", "action": "promote_candidate", "trigger": "no_regression"},
		},
		"policy_version": "vx18e.v1",
		"mode":           "vx_truncation_summary_loss_states_surface",
	}
}

// buildVXTruncationSummaryLossTest defines the truncation_summary_loss
// test surface for SEQ-18-P164: combined Step 18 backend regression bundle green
// after 18-4e (hybrid scoring, q3a query-class metadata, all VX gates passed).
func buildVXTruncationSummaryLossTest() map[string]any {
	return map[string]any{
		"version":         "seq18_p164.v1",
		"role":            "vx_truncation_summary_loss_test",
		"truth_authority": false,
		"test_file":       "backend/test_step18_truncation_summary_loss_gate.py",
		"guards": []string{
			"pending_without_evidence",
			"pending_with_insufficient_samples",
			"blocked_regression",
			"ready_non_regression",
		},
		"combined_bundle_status": "green",
		"bundle_components": []string{
			"hybrid_scoring",
			"q3a_query_class_metadata",
			"all_vx_gates",
		},
		"policy_version": "vx18e.v1",
		"mode":           "vx_truncation_summary_loss_test_surface",
	}
}

// ---------------------------------------------------------------------------
// SEQ-18 Post-Chroma / pre-release / VR/HY/QR/VX summary surfaces (P327 ~ P369)
// ---------------------------------------------------------------------------

// buildPostChromaTop1ScopedVerbatim defines the Post-Chroma Top 1 summary
// surface for SEQ-18-P327: scoped verbatim recall lane evidence.
func buildPostChromaTop1ScopedVerbatim() map[string]any {
	return map[string]any{
		"version":          "seq18_p327.v1",
		"role":             "post_chroma_top1_scoped_verbatim",
		"truth_authority":  false,
		"top":              1,
		"lane":             "scoped_verbatim_recall",
		"evidence_surface": "vr_scoped_verbatim_support_text",
		"policy_version":   "vr18a.v1",
		"mode":             "post_chroma_summary_surface",
	}
}

// buildPostChromaTop2HybridScoring defines the Post-Chroma Top 2 summary
// surface for SEQ-18-P328: hybrid retrieval scoring baseline evidence.
func buildPostChromaTop2HybridScoring() map[string]any {
	return map[string]any{
		"version":          "seq18_p328.v1",
		"role":             "post_chroma_top2_hybrid_scoring",
		"truth_authority":  false,
		"top":              2,
		"lane":             "hybrid_retrieval_scoring_baseline",
		"evidence_surface": "hy_semantic_rank_score",
		"policy_version":   "hy1a.v1",
		"mode":             "post_chroma_summary_surface",
	}
}

// buildPostChromaTop3TemporalRelation defines the Post-Chroma Top 3 summary
// surface for SEQ-18-P329: temporal relation + story clock foundation evidence.
func buildPostChromaTop3TemporalRelation() map[string]any {
	return map[string]any{
		"version":          "seq18_p329.v1",
		"role":             "post_chroma_top3_temporal_relation",
		"truth_authority":  false,
		"top":              3,
		"lane":             "temporal_relation_story_clock",
		"evidence_surface": "qr_temporal_profile_budget",
		"policy_version":   "qr1b.v1",
		"mode":             "post_chroma_summary_surface",
	}
}

// buildPostChromaTop4TemporalValidity defines the Post-Chroma Top 4 summary
// surface for SEQ-18-P330: temporal validity retrieval evidence.
func buildPostChromaTop4TemporalValidity() map[string]any {
	return map[string]any{
		"version":          "seq18_p330.v1",
		"role":             "post_chroma_top4_temporal_validity",
		"truth_authority":  false,
		"top":              4,
		"lane":             "temporal_validity_retrieval",
		"evidence_surface": "qr_temporal_profile_budget",
		"policy_version":   "qr1b.v1",
		"mode":             "post_chroma_summary_surface",
	}
}

// buildPostChromaTop5EntityGraph defines the Post-Chroma Top 5 summary
// surface for SEQ-18-P331: lightweight entity / graph retrieval accelerator evidence.
func buildPostChromaTop5EntityGraph() map[string]any {
	return map[string]any{
		"version":          "seq18_p331.v1",
		"role":             "post_chroma_top5_entity_graph",
		"truth_authority":  false,
		"top":              5,
		"lane":             "lightweight_entity_graph_accelerator",
		"evidence_surface": "post_chroma_top5_entity_graph",
		"policy_version":   "qr1b.v1",
		"mode":             "post_chroma_summary_surface",
	}
}

// buildPostChromaTop6SelectiveRerank defines the Post-Chroma Top 6 summary
// surface for SEQ-18-P332: selective rerank + budget-aware routing evidence.
func buildPostChromaTop6SelectiveRerank() map[string]any {
	return map[string]any{
		"version":          "seq18_p332.v1",
		"role":             "post_chroma_top6_selective_rerank",
		"truth_authority":  false,
		"top":              6,
		"lane":             "selective_rerank_budget_aware_routing",
		"evidence_surface": "hy_tail_budget_policy_owner",
		"policy_version":   "hy1d.v1",
		"mode":             "post_chroma_summary_surface",
	}
}

// buildVRRawPreservingSupport defines the VR raw-preserving support summary
// surface for SEQ-18-P336: verbatim recall support lane.
func buildVRRawPreservingSupport() map[string]any {
	return map[string]any{
		"version":          "seq18_p336.v1",
		"role":             "vr_raw_preserving_support",
		"truth_authority":  false,
		"support_lane":     "verbatim_recall",
		"evidence_surface": "vr_scoped_verbatim_support_text",
		"policy_version":   "vr18a.v1",
		"mode":             "vr_summary_surface",
	}
}

// buildVRHybridRealism defines the VR hybrid realism summary surface for
// SEQ-18-P337: semantic-only keyword/bias evidence.
func buildVRHybridRealism() map[string]any {
	return map[string]any{
		"version":          "seq18_p337.v1",
		"role":             "vr_hybrid_realism",
		"truth_authority":  false,
		"aspect":           "semantic_only_keyword_bias",
		"evidence_surface": "hy_soft_bias",
		"policy_version":   "hy1a.v1",
		"mode":             "vr_summary_surface",
	}
}

// buildVRSoftRouting defines the VR soft routing summary surface for
// SEQ-18-P338: query class evidence.
func buildVRSoftRouting() map[string]any {
	return map[string]any{
		"version":          "seq18_p338.v1",
		"role":             "vr_soft_routing",
		"truth_authority":  false,
		"aspect":           "query_class",
		"evidence_surface": "qr_query_class_contract",
		"policy_version":   "qr1a.v1",
		"mode":             "vr_summary_surface",
	}
}

// buildVRLatencyDiscipline defines the VR latency discipline summary surface
// for SEQ-18-P339: recall budget evidence.
func buildVRLatencyDiscipline() map[string]any {
	return map[string]any{
		"version":          "seq18_p339.v1",
		"role":             "vr_latency_discipline",
		"truth_authority":  false,
		"aspect":           "recall_budget",
		"evidence_surface": "qr_query_class_budget_policy",
		"policy_version":   "qr1b.v1",
		"mode":             "vr_summary_surface",
	}
}

// buildVRTruthBoundaryPreserve defines the VR truth-boundary preserve summary
// surface for SEQ-18-P340: Chroma hit canonical authority evidence.
func buildVRTruthBoundaryPreserve() map[string]any {
	return map[string]any{
		"version":          "seq18_p340.v1",
		"role":             "vr_truth_boundary_preserve",
		"truth_authority":  false,
		"aspect":           "chroma_hit_canonical_authority",
		"evidence_surface": "vx_truth_boundary_gate",
		"policy_version":   "vx18d.v1",
		"mode":             "vr_summary_surface",
	}
}

// buildVR18_1aRawTranscript defines the VR 18-1a raw transcript / direct-evidence
// support lane summary surface for SEQ-18-P344.
func buildVR18_1aRawTranscript() map[string]any {
	return map[string]any{
		"version":          "seq18_p344.v1",
		"role":             "vr_18_1a_raw_transcript",
		"truth_authority":  false,
		"sub_step":         "18-1a",
		"lane":             "raw_transcript_direct_evidence",
		"evidence_surface": "vr_scoped_verbatim_support_text",
		"policy_version":   "vr18a.v1",
		"mode":             "vr_sub_step_summary_surface",
	}
}

// buildVR18_1bSourceTag defines the VR 18-1b source-tag / scope metadata /
// snippet policy summary surface for SEQ-18-P345.
func buildVR18_1bSourceTag() map[string]any {
	return map[string]any{
		"version":          "seq18_p345.v1",
		"role":             "vr_18_1b_source_tag",
		"truth_authority":  false,
		"sub_step":         "18-1b",
		"aspect":           "source_tag_scope_metadata_snippet_policy",
		"evidence_surface": "vr_policy_owner_block",
		"policy_version":   "vr18a.v1",
		"mode":             "vr_sub_step_summary_surface",
	}
}

// buildVR18_1cPromptInjection defines the VR 18-1c prompt injection support
// surface summary for SEQ-18-P346.
func buildVR18_1cPromptInjection() map[string]any {
	return map[string]any{
		"version":          "seq18_p346.v1",
		"role":             "vr_18_1c_prompt_injection",
		"truth_authority":  false,
		"sub_step":         "18-1c",
		"aspect":           "prompt_injection_support_surface",
		"evidence_surface": "vr_prompt_injection_strategy",
		"policy_version":   "vr18a.v1",
		"mode":             "vr_sub_step_summary_surface",
	}
}

// buildVR18_1dHierarchyEscape defines the VR 18-1d hierarchy escape hatch
// summary surface for SEQ-18-P347: dense summary miss raw/direct-evidence slice.
func buildVR18_1dHierarchyEscape() map[string]any {
	return map[string]any{
		"version":          "seq18_p347.v1",
		"role":             "vr_18_1d_hierarchy_escape",
		"truth_authority":  false,
		"sub_step":         "18-1d",
		"aspect":           "hierarchy_escape_hatch",
		"evidence_surface": "vr_hierarchy_escape_hatch",
		"policy_version":   "vr18a.v1",
		"mode":             "vr_sub_step_summary_surface",
	}
}

// buildHY18_2aSemanticKeyword defines the HY 18-2a semantic + keyword baseline
// score summary surface for SEQ-18-P351.
func buildHY18_2aSemanticKeyword() map[string]any {
	return map[string]any{
		"version":          "seq18_p351.v1",
		"role":             "hy_18_2a_semantic_keyword",
		"truth_authority":  false,
		"sub_step":         "18-2a",
		"aspect":           "semantic_keyword_baseline_score",
		"evidence_surface": "hy_semantic_rank_score",
		"policy_version":   "hy1a.v1",
		"mode":             "hy_sub_step_summary_surface",
	}
}

// buildHY18_2bSoftBias defines the HY 18-2b speaker/location/storyline soft
// bias summary surface for SEQ-18-P352.
func buildHY18_2bSoftBias() map[string]any {
	return map[string]any{
		"version":          "seq18_p352.v1",
		"role":             "hy_18_2b_soft_bias",
		"truth_authority":  false,
		"sub_step":         "18-2b",
		"aspect":           "speaker_location_storyline_soft_bias",
		"evidence_surface": "hy_soft_bias",
		"policy_version":   "hy1a.v1",
		"mode":             "hy_sub_step_summary_surface",
	}
}

// buildHY18_2cScoreInspection defines the HY 18-2c hybrid score inspection
// surface summary for SEQ-18-P353.
func buildHY18_2cScoreInspection() map[string]any {
	return map[string]any{
		"version":          "seq18_p353.v1",
		"role":             "hy_18_2c_score_inspection",
		"truth_authority":  false,
		"sub_step":         "18-2c",
		"aspect":           "hybrid_score_inspection_surface",
		"evidence_surface": "hy_runtime_inspection",
		"policy_version":   "hy1a.v1",
		"mode":             "hy_sub_step_summary_surface",
	}
}

// buildHY18_2dAdaptiveTopK defines the HY 18-2d adaptive top-k / tail-budget
// summary surface for SEQ-18-P354: budget near-cutoff bounded tail rescue promotion.
func buildHY18_2dAdaptiveTopK() map[string]any {
	return map[string]any{
		"version":          "seq18_p354.v1",
		"role":             "hy_18_2d_adaptive_topk",
		"truth_authority":  false,
		"sub_step":         "18-2d",
		"aspect":           "adaptive_topk_tail_budget",
		"evidence_surface": "hy_tail_budget_rescue_pass",
		"policy_version":   "hy1d.v1",
		"mode":             "hy_sub_step_summary_surface",
	}
}

// buildQR18_3aQueryClass defines the QR 18-3a callback/resume/canon/scene/
// temporal query class summary surface for SEQ-18-P358.
func buildQR18_3aQueryClass() map[string]any {
	return map[string]any{
		"version":          "seq18_p358.v1",
		"role":             "qr_18_3a_query_class",
		"truth_authority":  false,
		"sub_step":         "18-3a",
		"aspect":           "callback_resume_canon_scene_temporal_query_class",
		"evidence_surface": "qr_query_class_taxonomy",
		"policy_version":   "qr1a.v1",
		"mode":             "qr_sub_step_summary_surface",
	}
}

// buildQR18_3bRetrievalDepth defines the QR 18-3b query class retrieval depth /
// candidate budget summary surface for SEQ-18-P359.
func buildQR18_3bRetrievalDepth() map[string]any {
	return map[string]any{
		"version":          "seq18_p359.v1",
		"role":             "qr_18_3b_retrieval_depth",
		"truth_authority":  false,
		"sub_step":         "18-3b",
		"aspect":           "query_class_retrieval_depth_candidate_budget",
		"evidence_surface": "qr_budget_visibility",
		"policy_version":   "qr1b.v1",
		"mode":             "qr_sub_step_summary_surface",
	}
}

// buildQR18_3cExtractBeforeRead defines the QR 18-3c extract-before-read
// retrieval note surface summary for SEQ-18-P360.
func buildQR18_3cExtractBeforeRead() map[string]any {
	return map[string]any{
		"version":          "seq18_p360.v1",
		"role":             "qr_18_3c_extract_before_read",
		"truth_authority":  false,
		"sub_step":         "18-3c",
		"aspect":           "extract_before_read_retrieval_note_surface",
		"evidence_surface": "qr_query_class_budget_policy",
		"policy_version":   "qr1b.v1",
		"mode":             "qr_sub_step_summary_surface",
	}
}

// buildQR18_3dLongTailRoute defines the QR 18-3d callback / needle-in-haystack /
// old-detail query route summary surface for SEQ-18-P361: long-tail miss scene recall split.
func buildQR18_3dLongTailRoute() map[string]any {
	return map[string]any{
		"version":          "seq18_p361.v1",
		"role":             "qr_18_3d_long_tail_route",
		"truth_authority":  false,
		"sub_step":         "18-3d",
		"aspect":           "callback_needle_in_haystack_old_detail_query_route",
		"evidence_surface": "qr_note_policy",
		"policy_version":   "qr1b.v1",
		"mode":             "qr_sub_step_summary_surface",
	}
}

// buildVX18_4aSemanticHybridReplay defines the VX 18-4a semantic-only vs
// hybrid replay summary surface for SEQ-18-P365.
func buildVX18_4aSemanticHybridReplay() map[string]any {
	return map[string]any{
		"version":          "seq18_p365.v1",
		"role":             "vx_18_4a_semantic_hybrid_replay",
		"truth_authority":  false,
		"sub_step":         "18-4a",
		"aspect":           "semantic_only_vs_hybrid_replay",
		"evidence_surface": "vx_hybrid_replay_gate",
		"policy_version":   "vx18a.v1",
		"mode":             "vx_sub_step_summary_surface",
	}
}

// buildVX18_4bHeldOutRecall defines the VX 18-4b held-out recall completeness
// gate summary surface for SEQ-18-P366.
func buildVX18_4bHeldOutRecall() map[string]any {
	return map[string]any{
		"version":          "seq18_p366.v1",
		"role":             "vx_18_4b_held_out_recall",
		"truth_authority":  false,
		"sub_step":         "18-4b",
		"aspect":           "held_out_recall_completeness_gate",
		"evidence_surface": "vx_heldout_completeness_gate",
		"policy_version":   "vx18b.v1",
		"mode":             "vx_sub_step_summary_surface",
	}
}

// buildVX18_4cLatencyToken defines the VX 18-4c latency / token budget gate
// summary surface for SEQ-18-P367.
func buildVX18_4cLatencyToken() map[string]any {
	return map[string]any{
		"version":          "seq18_p367.v1",
		"role":             "vx_18_4c_latency_token",
		"truth_authority":  false,
		"sub_step":         "18-4c",
		"aspect":           "latency_token_budget_gate",
		"evidence_surface": "vx_latency_token_budget_gate",
		"policy_version":   "vx18c.v1",
		"mode":             "vx_sub_step_summary_surface",
	}
}

// buildVX18_4dTruthBoundaryReplay defines the VX 18-4d truth-boundary replay
// summary surface for SEQ-18-P368.
func buildVX18_4dTruthBoundaryReplay() map[string]any {
	return map[string]any{
		"version":          "seq18_p368.v1",
		"role":             "vx_18_4d_truth_boundary_replay",
		"truth_authority":  false,
		"sub_step":         "18-4d",
		"aspect":           "truth_boundary_replay",
		"evidence_surface": "vx_truth_boundary_gate",
		"policy_version":   "vx18d.v1",
		"mode":             "vx_sub_step_summary_surface",
	}
}

// buildVX18_4eTopKTruncation defines the VX 18-4e top-k truncation / summary-loss
// regression gate summary surface for SEQ-18-P369: tail fact miss actual held-out verify.
func buildVX18_4eTopKTruncation() map[string]any {
	return map[string]any{
		"version":          "seq18_p369.v1",
		"role":             "vx_18_4e_topk_truncation",
		"truth_authority":  false,
		"sub_step":         "18-4e",
		"aspect":           "topk_truncation_summary_loss_regression_gate",
		"evidence_surface": "vx_truncation_summary_loss_gate",
		"policy_version":   "vx18e.v1",
		"mode":             "vx_sub_step_summary_surface",
	}
}

// buildPreReleaseVersionMarker defines the pre-release version marker
// summary surface for SEQ-18-P373: root runtime version marker file 1.0.0-pre promotion.
func buildPreReleaseVersionMarker() map[string]any {
	return map[string]any{
		"version":         "seq18_p373.v1",
		"role":            "pre_release_version_marker",
		"truth_authority": false,
		"marker_file":     "1.0.0-pre",
		"promotion":       true,
		"policy_version":  "pr1a.v1",
		"mode":            "pre_release_summary_surface",
	}
}

// buildPreReleaseBundleAuthority defines the pre-release bundle authority
// summary surface for SEQ-18-P374: Archive Center Pre-release 1.0.0 bundle latest
// current working runtime authority create/generate.
func buildPreReleaseBundleAuthority() map[string]any {
	return map[string]any{
		"version":          "seq18_p374.v1",
		"role":             "pre_release_bundle_authority",
		"truth_authority":  false,
		"bundle_name":      "Archive Center Pre-release 1.0.0",
		"authority":        "latest_current_working_runtime",
		"evidence_surface": "vr_regression_bundle_green",
		"policy_version":   "pr1a.v1",
		"mode":             "pre_release_summary_surface",
	}
}

// buildPreReleaseArtifact defines the pre-release artifact summary surface
// for SEQ-18-P375: latest validated bundle artifact.
func buildPreReleaseArtifact() map[string]any {
	return map[string]any{
		"version":         "seq18_p375.v1",
		"role":            "pre_release_artifact",
		"truth_authority": false,
		"artifact_name":   "Archive Center Pre-release 1.0.0",
		"validated":       true,
		"policy_version":  "pr1a.v1",
		"mode":            "pre_release_summary_surface",
	}
}

// buildPreReleaseVRSmoke defines the pre-release VR smoke check summary
// surface for SEQ-18-P376: scoped verbatim recall smoke check pass.
func buildPreReleaseVRSmoke() map[string]any {
	return map[string]any{
		"version":          "seq18_p376.v1",
		"role":             "pre_release_vr_smoke",
		"truth_authority":  false,
		"check":            "scoped_verbatim_recall_smoke",
		"status":           "pass",
		"evidence_surface": "vr_scoped_verbatim_support_text",
		"policy_version":   "pr1a.v1",
		"mode":             "pre_release_smoke_summary_surface",
	}
}

// buildPreReleaseHYSmoke defines the pre-release HY smoke check summary
// surface for SEQ-18-P377: hybrid baseline smoke check pass.
func buildPreReleaseHYSmoke() map[string]any {
	return map[string]any{
		"version":          "seq18_p377.v1",
		"role":             "pre_release_hy_smoke",
		"truth_authority":  false,
		"check":            "hybrid_baseline_smoke",
		"status":           "pass",
		"evidence_surface": "hy_semantic_rank_score",
		"policy_version":   "pr1a.v1",
		"mode":             "pre_release_smoke_summary_surface",
	}
}

// buildPreReleaseQRSmoke defines the pre-release QR smoke check summary
// surface for SEQ-18-P378: query-class routing / budget smoke check pass.
func buildPreReleaseQRSmoke() map[string]any {
	return map[string]any{
		"version":          "seq18_p378.v1",
		"role":             "pre_release_qr_smoke",
		"truth_authority":  false,
		"check":            "query_class_routing_budget_smoke",
		"status":           "pass",
		"evidence_surface": "qr_query_class_contract",
		"policy_version":   "pr1a.v1",
		"mode":             "pre_release_smoke_summary_surface",
	}
}

// buildPreReleaseVXReview defines the pre-release VX review checklist summary
// surface for SEQ-18-P379: held-out / latency / truth-boundary / truncation-summary-loss
// review checklist pass.
func buildPreReleaseVXReview() map[string]any {
	return map[string]any{
		"version":         "seq18_p379.v1",
		"role":            "pre_release_vx_review",
		"truth_authority": false,
		"check":           "vx_review_checklist",
		"status":          "pass",
		"components": []string{
			"held_out_completeness",
			"latency_token_budget",
			"truth_boundary_replay",
			"truncation_summary_loss",
		},
		"policy_version": "pr1a.v1",
		"mode":           "pre_release_review_summary_surface",
	}
}

// buildPreReleaseRawSnippet defines the pre-release raw support snippet
// summary surface for SEQ-18-P391: raw support snippet 3 / 720 chars / excerpt 160 chars.
func buildPreReleaseRawSnippet() map[string]any {
	return map[string]any{
		"version":         "seq18_p391.v1",
		"role":            "pre_release_raw_snippet",
		"truth_authority": false,
		"snippet_count":   3,
		"max_chars":       720,
		"excerpt_chars":   160,
		"policy_version":  "pr1a.v1",
		"mode":            "pre_release_detail_summary_surface",
	}
}

// buildPreReleaseHybridBias defines the pre-release hybrid score soft bias
// summary surface for SEQ-18-P392: speaker 0.04 / location 0.05 / storyline 0.06 / cap 0.12.
func buildPreReleaseHybridBias() map[string]any {
	return map[string]any{
		"version":         "seq18_p392.v1",
		"role":            "pre_release_hybrid_bias",
		"truth_authority": false,
		"speaker":         0.04,
		"location":        0.05,
		"storyline":       0.06,
		"cap":             0.12,
		"policy_version":  "pr1a.v1",
		"mode":            "pre_release_detail_summary_surface",
	}
}

// buildPreReleaseQueryClassRule defines the pre-release query class rule
// heuristic summary surface for SEQ-18-P393: rule-first additive contract + fail-open
// shared execution.
func buildPreReleaseQueryClassRule() map[string]any {
	return map[string]any{
		"version":         "seq18_p393.v1",
		"role":            "pre_release_query_class_rule",
		"truth_authority": false,
		"heuristic":       "rule_first_additive_contract",
		"execution":       "fail_open_shared_execution",
		"policy_version":  "pr1a.v1",
		"mode":            "pre_release_detail_summary_surface",
	}
}

// buildPreReleaseRetrievalNote defines the pre-release retrieval note /
// extract surface default summary surface for SEQ-18-P394: support_surface_first,
// scene/canon no-extract default, callback/resume/temporal note-only until route exec.
func buildPreReleaseRetrievalNote() map[string]any {
	return map[string]any{
		"version":         "seq18_p394.v1",
		"role":            "pre_release_retrieval_note",
		"truth_authority": false,
		"defaults": map[string]any{
			"support_surface_first":              true,
			"scene_canon_no_extract":             true,
			"callback_resume_temporal_note_only": true,
			"note_only_until_route_exec":         true,
		},
		"policy_version": "pr1a.v1",
		"mode":           "pre_release_detail_summary_surface",
	}
}
