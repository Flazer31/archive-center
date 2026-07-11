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

func (s *Server) handleCompleteTurn(w http.ResponseWriter, r *http.Request) {
	var req dto.M4CompleteTurnRequest
	if err := dto.DecodeWithDefaults(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	s.executeCompleteTurnIdempotent(r.Context(), w, completeTurnIdempotencyKey(req.ClientMeta), func(target http.ResponseWriter) {
		s.handleCompleteTurnDecoded(target, r, req)
	})
}

func (s *Server) handleCompleteTurnDecoded(w http.ResponseWriter, r *http.Request, req dto.M4CompleteTurnRequest) {
	timing := newBackendTimingTrace("complete_turn.backend_timing.v1")
	preflightStartedAt := time.Now()
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
	timing.addElapsed("preflight", preflightStartedAt)
	if s.usesShadowWriteStore() && content != "" {
		if extractionCfg.Critic.hasConfig() {
			if assistantText == "" {
				failReasons = append(failReasons, "critic_skipped: assistant_content_missing")
			} else {
				criticStartedAt := time.Now()
				result, trace, err := s.runCompleteTurnCriticWithInputPolicy(ctx, sid, turnIndex, userText, assistantText, req.ContextMessages, req.OutputLanguageOverride, extractionCfg.Critic, true, languageContext)
				timing.addElapsed("critic_llm", criticStartedAt)
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

	rawStoreStartedAt := time.Now()
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
		timing.addElapsed("raw_and_audit_store", rawStoreStartedAt)
		if criticResult != nil {
			artifactStartedAt := time.Now()
			artifactResult := s.saveCriticExtractionArtifacts(ctx, sid, turnIndex, criticResult, content, extractionCfg.Embedder, now, existingEvidence)
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
	if !s.usesShadowWriteStore() {
		timing.addElapsed("raw_and_audit_store", rawStoreStartedAt)
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
		failReasons = append(failReasons, "maintenance_enqueue")
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
	backendTiming := timing.snapshot()

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
		"backend_timing":                   backendTiming,
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
