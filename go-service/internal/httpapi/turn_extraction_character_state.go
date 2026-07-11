package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func (s *Server) saveCharacterAndStateArtifacts(ctx context.Context, sid string, turnIndex int, extraction map[string]any, embCfg completeTurnEmbeddingConfig, now time.Time, result *artifactSaveResult, existingCanonicalLayers []store.CanonicalStateLayer, cost *canonicalStateWriteCostMeasurement) {
	entities := mapFromAny(extraction["entities"])
	physicalConditions := normalizePhysicalConditionItems(extraction["physical_conditions"])
	entityConditions := normalizePhysicalConditionItems(extraction["entity_conditions"])
	seenExactEntities := map[string]bool{}
	saveEntityItems := func(items []any, entityType string) {
		for idx, item := range items {
			entity := mapFromAny(item)
			name := s.canonicalCharacterName(ctx, sid, strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(entity, "name"), stringFromMap(entity, "label"), stringFromMap(entity, "title"))))
			if name == "" || isPlaceholderKGPart(name) {
				continue
			}
			exactKey := strings.ToLower(strings.TrimSpace(entityType)) + "\x1f" + comparableEntityKey(name)
			if seenExactEntities[exactKey] {
				result.addSkipReason("entities", "duplicate_exact_entity_name_type", map[string]any{
					"index":       idx,
					"name":        name,
					"entity_type": entityType,
				})
				continue
			}
			seenExactEntities[exactKey] = true
			if saver, ok := s.Store.(entitySaver); ok {
				localType := extractionFirstNonEmpty(stringFromMap(entity, "entity_type"), stringFromMap(entity, "role"), entityType)
				description := entityDescriptionWithConditions(
					extractionFirstNonEmpty(stringFromMap(entity, "status_emotion"), stringFromMap(entity, "description"), stringFromMap(entity, "summary")),
					name,
					localType,
					physicalConditions,
					entityConditions,
				)
				result.trySave("SaveEntity", func() error {
					return saver.SaveEntity(ctx, &store.Entity{
						ChatSessionID: sid,
						Name:          name,
						EntityType:    localType,
						Description:   description,
						AliasesJSON:   mustCompactJSON(stringsFromAny(entity["aliases"])),
						FirstSeenTurn: turnIndex,
						LastSeenTurn:  turnIndex,
						Confidence:    clampFloat(extractionFloatFromAny(entity["confidence"], 0.7), 0, 1),
						CreatedAt:     now,
						UpdatedAt:     now,
					})
				}, result, func() { result.Entities++ })
			}
		}
	}
	saveEntityItems(sliceFromAny(entities["characters"]), "character")
	saveEntityItems(sliceFromAny(entities["locations"]), "location")
	saveEntityItems(sliceFromAny(entities["places"]), "location")
	saveEntityItems(sliceFromAny(entities["items"]), "item")
	saveEntityItems(sliceFromAny(entities["objects"]), "item")
	characterNames := extractedEntityNames(ctx, s, sid, entities)

	relationshipMemory := mapFromAny(extraction["relationship_memory"])
	if trustText := strings.TrimSpace(stringFromMap(relationshipMemory, "bond_and_distance")); trustText != "" {
		if saver, ok := s.Store.(trustSaver); ok {
			for _, target := range relationshipMemoryTargets(relationshipMemory, characterNames) {
				result.trySave("SaveTrust", func() error {
					return saver.SaveTrust(ctx, &store.Trust{
						ChatSessionID: sid,
						TargetName:    target,
						TargetType:    "relationship",
						Score:         clampFloat(extractionFloatFromAny(relationshipMemory["trust"], 0.5), 0, 1),
						ReasonJSON:    mustCompactJSON(relationshipMemory),
						SourceTurn:    turnIndex,
						CreatedAt:     now,
						UpdatedAt:     now,
					})
				}, result, func() { result.TrustStates++ })
			}
		}
	}

	for _, item := range sliceFromAny(extraction["character_deltas"]) {
		charDelta := mapFromAny(item)
		name := s.canonicalCharacterName(ctx, sid, strings.TrimSpace(stringFromMap(charDelta, "name")))
		if name == "" {
			continue
		}
		if looksLikeTransientDescriptorCharacterName(name) && !characterDeltaHasContinuityAnchor(charDelta) {
			continue
		}
		var currentState *store.CharacterState
		if current, err := s.Store.GetCharacterState(ctx, sid, name); err == nil {
			currentState = current
		}
		if saver, ok := s.Store.(characterStateSaver); ok {
			appearanceJSON := mergeCharacterStateJSONField(currentCharacterJSON(currentState, "appearance"), charDelta["appearance"])
			personalityJSON := mergeCharacterStateJSONField(currentCharacterJSON(currentState, "personality"), charDelta["personality"])
			statusJSON := mergeCharacterStateJSONField(currentCharacterJSON(currentState, "status"), charDelta["status"])
			relationshipsJSON := mergeCharacterStateJSONField(currentCharacterJSON(currentState, "relationships"), charDelta["relationships"])
			speechStyleJSON := mergeCharacterStateJSONField(currentCharacterJSON(currentState, "speech_style"), charDelta["speech_style"])
			result.trySave("SaveCharacterState", func() error {
				return saver.SaveCharacterState(ctx, &store.CharacterState{
					ChatSessionID:     sid,
					CharacterName:     name,
					AppearanceJSON:    appearanceJSON,
					PersonalityJSON:   personalityJSON,
					StatusJSON:        statusJSON,
					RelationshipsJSON: relationshipsJSON,
					SpeechStyleJSON:   speechStyleJSON,
					TurnIndex:         turnIndex,
					CreatedAt:         now,
					UpdatedAt:         now,
				})
			}, result, func() { result.CharacterStates++ })
		}
		for _, ev := range sliceFromAny(charDelta["events"]) {
			evMap := mapFromAny(ev)
			detail := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(evMap, "detail"), stringFromMap(evMap, "summary"), mustCompactJSON(evMap)))
			if detail == "" {
				continue
			}
			result.trySave("SaveCharacterEvent", func() error {
				return s.Store.SaveCharacterEvent(ctx, &store.CharacterEvent{
					ChatSessionID: sid,
					CharacterName: name,
					TurnIndex:     turnIndex,
					EventType:     extractionFirstNonEmpty(stringFromMap(evMap, "type"), "critic_delta"),
					DetailsJSON:   mustCompactJSON(map[string]any{"detail": detail, "delta": charDelta}),
					CreatedAt:     now,
				})
			}, result, func() { result.CharacterEvents++ })
		}
	}

	s.savePhysicalConditionsFromExtraction(ctx, sid, turnIndex, extraction, now, result)
	s.saveEntityConditionsFromExtraction(ctx, sid, turnIndex, extraction, now, result)

	if saver, ok := s.Store.(activeStateSaver); ok {
		for _, key := range []string{"relationship_memory", "state_deltas", "entities"} {
			rawState, present := extraction[key]
			if !present {
				continue
			}
			if key == "state_deltas" {
				rawState = sanitizeStateDeltasForParticipant(rawState)
			}
			if key == "relationship_memory" {
				rawState = normalizeRelationshipStateV2(mapFromAny(rawState))
			}
			if !hasMeaningfulPayload(rawState) {
				continue
			}
			stateType := key
			result.trySave("SaveActiveState", func() error {
				return saver.SaveActiveState(ctx, &store.ActiveState{
					ChatSessionID: sid,
					StateType:     stateType,
					Content:       mustCompactJSON(rawState),
					TurnIndex:     turnIndex,
					CreatedAt:     now,
				})
			}, result, func() { result.ActiveStates++ })
			// P358 HS-1a: canonical state layer from active state with provenance (P407)
			if clSaver, ok2 := s.Store.(canonicalStateLayerSaver); ok2 {
				layerType := mapKeyToCanonicalLayerType(key)
				confidence := extractConfidenceForStateKey(extraction, key)
				if canonicalStatePromotionAllowed(rawState, confidence) {
					result.trySave("SaveCanonicalStateLayer", func() error {
						return saveCanonicalStateLayerWithCost(ctx, clSaver, sid, &store.CanonicalStateLayer{
							ChatSessionID:    sid,
							LayerType:        layerType,
							Content:          mustCompactJSON(rawState),
							SourceStateType:  stateType,
							TurnIndex:        turnIndex,
							SourceTurn:       turnIndex,
							SourceRecord:     0,
							LastVerifiedTurn: turnIndex,
							Confidence:       confidence,
							CreatedAt:        now,
						}, existingCanonicalLayers, cost)
					}, result, func() { result.CanonicalStateLayers++ })
				}
			}
		}
	}

	// P469 HS-1h: world current state minimal canonical snapshot
	if wsPayload, ok := extractWorldStatePayload(extraction); ok && hasMeaningfulPayload(wsPayload) {
		if saver, ok := s.Store.(activeStateSaver); ok {
			result.trySave("SaveActiveState(world_state)", func() error {
				return saver.SaveActiveState(ctx, &store.ActiveState{
					ChatSessionID: sid,
					StateType:     "world_state",
					Content:       mustCompactJSON(wsPayload),
					TurnIndex:     turnIndex,
					CreatedAt:     now,
				})
			}, result, func() { result.ActiveStates++ })
		}
		if clSaver, ok2 := s.Store.(canonicalStateLayerSaver); ok2 {
			confidence := extractConfidenceForStateKey(extraction, "world_state")
			if canonicalStatePromotionAllowed(wsPayload, confidence) {
				result.trySave("SaveCanonicalStateLayer(world_state)", func() error {
					return saveCanonicalStateLayerWithCost(ctx, clSaver, sid, &store.CanonicalStateLayer{
						ChatSessionID:    sid,
						LayerType:        "world_state",
						Content:          mustCompactJSON(wsPayload),
						SourceStateType:  "world_state",
						TurnIndex:        turnIndex,
						SourceTurn:       turnIndex,
						SourceRecord:     0,
						LastVerifiedTurn: turnIndex,
						Confidence:       confidence,
						CreatedAt:        now,
					}, existingCanonicalLayers, cost)
				}, result, func() { result.CanonicalStateLayers++ })
			}
		}
	}

	for _, item := range sliceFromAny(extraction["pending_threads"]) {
		thread := mapFromAny(item)
		title := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(thread, "title"), stringFromMap(thread, "description"), stringFromMap(thread, "thread_type")))
		if title == "" {
			result.addSkipReason("pending_threads", "missing_title", thread)
			continue
		}
		threadType := strings.TrimSpace(stringFromMap(thread, "thread_type"))
		if threadType == "" {
			threadType = "open_question"
		}
		if !validPendingThreadType(threadType) {
			result.addSkipReason("pending_threads", "invalid_thread_type", thread)
			continue
		}
		confidence := clampFloat(extractionFloatFromAny(thread["confidence"], 0), 0, 1)
		if _, hasConfidence := thread["confidence"]; hasConfidence && confidence < 0.3 {
			result.addSkipReason("pending_threads", "low_confidence", thread)
			continue
		}
		if saver, ok := s.Store.(pendingThreadSaver); ok {
			result.trySave("SavePendingThread", func() error {
				return saver.SavePendingThread(ctx, &store.PendingThread{
					ChatSessionID:    sid,
					ThreadKey:        stableKey("thread", title),
					Description:      extractionFirstNonEmpty(stringFromMap(thread, "details"), title),
					Status:           "open",
					CreatedTurn:      turnIndex,
					SourceTurn:       turnIndex,
					Priority:         intFromAny(thread["priority"], 0),
					HookType:         threadType,
					HookMetadataJSON: mustCompactJSON(thread),
					ThreadType:       threadType,
					Title:            title,
					Owner:            sanitizeParticipantActorName(stringFromMap(thread, "owner")),
					Target:           sanitizeParticipantActorName(stringFromMap(thread, "target")),
					LastSeenTurn:     turnIndex,
					Confidence:       confidence,
					DetailsJSON:      mustCompactJSON(thread),
					CreatedAt:        now,
					UpdatedAt:        now,
				})
			}, result, func() { result.PendingThreads++ })
		}
		threadState := map[string]any{
			"thread_type": threadType,
			"title":       title,
			"status":      "open",
			"confidence":  confidence,
			"source_turn": turnIndex,
		}
		if saver, ok := s.Store.(activeStateSaver); ok {
			result.trySave("SaveActiveState(unresolved_threads)", func() error {
				return saver.SaveActiveState(ctx, &store.ActiveState{
					ChatSessionID: sid,
					StateType:     "unresolved_threads",
					Content:       mustCompactJSON(threadState),
					TurnIndex:     turnIndex,
					CreatedAt:     now,
				})
			}, result, func() { result.ActiveStates++ })
		}
		// P358 HS-1a: canonical state layer for unresolved threads with provenance (P407)
		if clSaver, ok2 := s.Store.(canonicalStateLayerSaver); ok2 && confidence >= 0.7 {
			result.trySave("SaveCanonicalStateLayer", func() error {
				return saveCanonicalStateLayerWithCost(ctx, clSaver, sid, &store.CanonicalStateLayer{
					ChatSessionID:    sid,
					LayerType:        "unresolved_threads",
					Content:          mustCompactJSON(threadState),
					SourceStateType:  "pending_threads",
					TurnIndex:        turnIndex,
					SourceTurn:       turnIndex,
					SourceRecord:     0,
					LastVerifiedTurn: turnIndex,
					Confidence:       confidence,
					CreatedAt:        now,
				}, existingCanonicalLayers, cost)
			}, result, func() { result.CanonicalStateLayers++ })
		}
		if saver, ok := s.Store.(storylineSaver); ok {
			result.trySave("SaveStoryline", func() error {
				return saver.SaveStoryline(ctx, &store.Storyline{
					ChatSessionID:       sid,
					Name:                title,
					Status:              "active",
					EntitiesJSON:        mustCompactJSON(extraction["entities"]),
					CurrentContext:      extractionFirstNonEmpty(stringFromMap(thread, "details"), title),
					KeyPointsJSON:       mustCompactJSON([]string{title}),
					OngoingTensionsJSON: mustCompactJSON(thread),
					Confidence:          clampFloat(extractionFloatFromAny(thread["confidence"], 0), 0, 1),
					EvidenceCount:       len(stringsFromAny(extraction["evidence_excerpts"])),
					LastEvidenceTurn:    turnIndex,
					FirstTurn:           turnIndex,
					LastTurn:            turnIndex,
					CreatedAt:           now,
					UpdatedAt:           now,
				})
			}, result, func() { result.Storylines++ })
		}
	}

	if saver, ok := s.Store.(worldRuleSaver); ok {
		for _, item := range worldRuleItemsForSave(extraction) {
			rule := mapFromAny(item)
			key := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(rule, "key"), stringFromMap(rule, "name")))
			if key == "" {
				continue
			}
			wr := &store.WorldRule{
				ChatSessionID: sid,
				Scope:         extractionFirstNonEmpty(stringFromMap(rule, "scope"), "session"),
				ScopeName:     stringFromMap(rule, "scope_name"),
				Category:      extractionFirstNonEmpty(stringFromMap(rule, "category"), "critic"),
				Key:           key,
				ValueJSON:     mustCompactJSON(extractionFirstNonEmpty(stringFromMap(rule, "value"), stringFromMap(rule, "value_json"), mustCompactJSON(rule))),
				Genre:         stringFromMap(rule, "genre"),
				SourceTurn:    turnIndex,
				CreatedAt:     now,
				UpdatedAt:     now,
			}
			result.trySave("SaveWorldRule", func() error {
				return saver.SaveWorldRule(ctx, wr)
			}, result, func() {
				result.WorldRules++
				s.upsertDerivedArtifactVector(ctx, sid, turnIndex, "world_rule", "world_rules", wr.ID, "world_rule.v1", worldRuleVectorDocumentText(*wr), embCfg, result)
			})
		}
	}
	s.saveCriticIngestTrace(ctx, sid, turnIndex, now, result)
}

func (r *artifactSaveResult) addSkipReason(surface, reason string, input any) {
	if r == nil {
		return
	}
	r.SkipReasons = append(r.SkipReasons, map[string]any{
		"surface": surface,
		"reason":  reason,
		"input":   input,
	})
}

func (s *Server) saveCriticIngestTrace(ctx context.Context, sid string, turnIndex int, now time.Time, result *artifactSaveResult) {
	if s.Store == nil || result == nil {
		return
	}
	details := map[string]any{
		"policy_version":              "critic_ingest_trace.v1",
		"turn_index":                  turnIndex,
		"memories":                    result.Memories,
		"direct_evidence":             result.Evidence,
		"kg_triples":                  result.KGTriples,
		"persona_capsule_candidates":  result.PersonaCapsuleCandidates,
		"subjective_entity_memories":  result.SubjectiveEntityMemories,
		"character_states":            result.CharacterStates,
		"physical_conditions":         result.PhysicalConditions,
		"entity_conditions":           result.EntityConditions,
		"status_schema_definitions":   result.StatusSchemaDefinitions,
		"status_effects":              result.StatusEffects,
		"narrative_current_states":    result.NarrativeCurrentStates,
		"narrative_state_events":      result.NarrativeStateEvents,
		"pending_threads":             result.PendingThreads,
		"active_states":               result.ActiveStates,
		"canonical_layers":            result.CanonicalStateLayers,
		"skip_reasons":                result.SkipReasons,
		"warnings":                    result.Warnings,
		"embedding_status":            result.EmbeddingStatus,
		"vector_status":               result.VectorStatus,
		"vectors_upserted":            result.VectorsUpserted,
		"vectors_memory_upserted":     result.VectorsMemoryUpserted,
		"vectors_evidence_upserted":   result.VectorsEvidenceUpserted,
		"vectors_world_rule_upserted": result.VectorsWorldRuleUpserted,
		"artifact_save_errors":        result.ErrorDetails,
	}
	result.trySave("SaveAuditLog(critic_ingest_trace)", func() error {
		return s.Store.SaveAuditLog(ctx, &store.AuditLog{
			ChatSessionID: sid,
			EventType:     "critic_ingest_trace",
			TargetType:    "turn",
			TargetID:      int64(turnIndex),
			Source:        "critic",
			Summary:       fmt.Sprintf("critic ingest trace turn %d", turnIndex),
			DetailsJSON:   mustCompactJSON(details),
			CreatedAt:     now,
		})
	}, result, func() {})
}

func currentCharacterJSON(current *store.CharacterState, field string) string {
	if current == nil {
		return ""
	}
	switch field {
	case "appearance":
		return current.AppearanceJSON
	case "personality":
		return current.PersonalityJSON
	case "status":
		return current.StatusJSON
	case "relationships":
		return current.RelationshipsJSON
	case "speech_style":
		return current.SpeechStyleJSON
	default:
		return ""
	}
}

func mergeCharacterStateJSONField(existing string, incoming any) string {
	if !hasMeaningfulPayload(incoming) {
		return strings.TrimSpace(existing)
	}
	incomingJSON := mustCompactJSON(incoming)
	if strings.TrimSpace(existing) == "" {
		return incomingJSON
	}
	var existingMap map[string]any
	var incomingMap map[string]any
	if json.Unmarshal([]byte(existing), &existingMap) != nil || json.Unmarshal([]byte(incomingJSON), &incomingMap) != nil {
		return incomingJSON
	}
	return mustCompactJSON(mergeJSONMaps(existingMap, incomingMap))
}

func mergeJSONMaps(base, overlay map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(overlay))
	for key, value := range base {
		out[key] = value
	}
	for key, value := range overlay {
		if overlayMap, ok := value.(map[string]any); ok {
			if baseMap, ok := out[key].(map[string]any); ok {
				out[key] = mergeJSONMaps(baseMap, overlayMap)
				continue
			}
		}
		out[key] = value
	}
	return out
}

func (r *artifactSaveResult) trySave(label string, save func() error, result *artifactSaveResult, onOK func()) {
	result.Attempted++
	if err := save(); err != nil {
		result.Errors++
		result.ErrorDetails = append(result.ErrorDetails, label+": "+err.Error())
		return
	}
	onOK()
}
