package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func (s *Server) saveCharacterAndStateArtifacts(ctx context.Context, sid string, turnIndex int, extraction map[string]any, completedTurnText string, embCfg completeTurnEmbeddingConfig, now time.Time, result *artifactSaveResult, existingCanonicalLayers []store.CanonicalStateLayer, cost *canonicalStateWriteCostMeasurement, identityProjectionArg ...*entityIdentityProjection) {
	var identityProjection *entityIdentityProjection
	if len(identityProjectionArg) > 0 {
		identityProjection = identityProjectionArg[0]
	}
	entities := mapFromAny(extraction["entities"])
	seenExactEntities := map[string]bool{}
	saveEntityItems := func(items []any, entityType string) {
		for idx, item := range items {
			entity := mapFromAny(item)
			rawName := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(entity, "name"), stringFromMap(entity, "label"), stringFromMap(entity, "title")))
			if rawName == "" {
				rawName, _ = item.(string)
				rawName = strings.TrimSpace(rawName)
			}
			if rawName == "" {
				result.addSkipReason("entities", "missing_name", map[string]any{"index": idx, "entity_type": entityType})
				continue
			}
			name := s.canonicalCharacterName(ctx, sid, rawName)
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
				description := extractionFirstNonEmpty(stringFromMap(entity, "description"), stringFromMap(entity, "summary"))
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

	priorCharacterStates := map[string]store.CharacterState{}
	sameTurnCharacterStates := map[string]store.CharacterState{}
	timelineReadAvailable := false
	if turnIndex > 0 {
		if reader, ok := s.Store.(interface {
			ListCharacterStatesCurrentBefore(context.Context, string, int) ([]store.CharacterState, error)
		}); ok {
			before, beforeErr := reader.ListCharacterStatesCurrentBefore(ctx, sid, turnIndex)
			through, throughErr := reader.ListCharacterStatesCurrentBefore(ctx, sid, turnIndex+1)
			if beforeErr == nil && throughErr == nil {
				timelineReadAvailable = true
				for _, state := range before {
					priorCharacterStates[comparableEntityKey(state.CharacterName)] = state
				}
				for _, state := range through {
					if state.TurnIndex == turnIndex {
						sameTurnCharacterStates[comparableEntityKey(state.CharacterName)] = state
					}
				}
			}
		}
	}
	type pendingCharacterStateProjection struct {
		state       store.CharacterState
		sourceIndex int
	}
	pendingCharacterStates := map[string]pendingCharacterStateProjection{}
	pendingCharacterOrder := []string{}
	for characterDeltaIndex, item := range sliceFromAny(extraction["character_deltas"]) {
		charDelta := mapFromAny(item)
		rawName := strings.TrimSpace(stringFromMap(charDelta, "name"))
		if rawName == "" {
			result.addSkipReason("character_deltas", "missing_name", map[string]any{"index": characterDeltaIndex})
			continue
		}
		currentItems := sanitizeLegacyReversibleCharacterDeltas([]any{charDelta})
		if len(currentItems) == 0 {
			continue
		}
		currentDelta := mapFromAny(currentItems[0])
		if identityProjection != nil && rawName != "" {
			identityProjection.bindCharacterState(ctx, rawName, characterDeltaIndex, result)
		}
		name := s.canonicalCharacterName(ctx, sid, rawName)
		if name == "" {
			continue
		}
		var currentState *store.CharacterState
		characterKey := comparableEntityKey(name)
		if pending, ok := pendingCharacterStates[characterKey]; ok {
			current := pending.state
			currentState = &current
		} else if timelineReadAvailable {
			if prior, ok := priorCharacterStates[characterKey]; ok {
				current := prior
				currentState = &current
			}
		} else if current, err := s.Store.GetCharacterState(ctx, sid, name); err == nil {
			currentState = current
		}
		appearanceJSON := mergeCharacterStateJSONField(currentCharacterJSON(currentState, "appearance"), currentDelta["appearance"])
		personalityJSON := mergeCharacterStateJSONField(currentCharacterJSON(currentState, "personality"), currentDelta["personality"])
		statusJSON := mergeCharacterStateJSONField(currentCharacterJSON(currentState, "status"), currentDelta["status"])
		relationshipsJSON := mergeCharacterStateJSONField(currentCharacterJSON(currentState, "relationships"), nil)
		speechStyleJSON := mergeCharacterStateJSONField(currentCharacterJSON(currentState, "speech_style"), currentDelta["speech_style"])
		if timelineReadAvailable {
			appearanceJSON = characterStateJSONOrEmptyObject(appearanceJSON)
			personalityJSON = characterStateJSONOrEmptyObject(personalityJSON)
			statusJSON = characterStateJSONOrEmptyObject(statusJSON)
			relationshipsJSON = characterStateJSONOrEmptyObject(relationshipsJSON)
			speechStyleJSON = characterStateJSONOrEmptyObject(speechStyleJSON)
		}
		if _, exists := pendingCharacterStates[characterKey]; !exists {
			pendingCharacterOrder = append(pendingCharacterOrder, characterKey)
		}
		pendingCharacterStates[characterKey] = pendingCharacterStateProjection{
			state: store.CharacterState{
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
			},
			sourceIndex: characterDeltaIndex,
		}
		for _, ev := range sliceFromAny(currentDelta["events"]) {
			evMap := mapFromAny(ev)
			if legacyRelationshipShiftToken(extractionFirstNonEmpty(
				stringFromMap(evMap, "type"),
				stringFromMap(evMap, "event_type"),
			)) {
				continue
			}
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
	if saver, ok := s.Store.(characterStateSaver); ok {
		for _, characterKey := range pendingCharacterOrder {
			pending := pendingCharacterStates[characterKey]
			if existing, found := sameTurnCharacterStates[characterKey]; found && sameCharacterStateProjection(existing, pending.state) {
				result.addSkipReason("character_deltas", "duplicate_same_turn_state", map[string]any{
					"index": pending.sourceIndex,
					"name":  pending.state.CharacterName,
				})
				continue
			}
			next := pending.state
			result.trySave("SaveCharacterState", func() error {
				return saver.SaveCharacterState(ctx, &next)
			}, result, func() { result.CharacterStates++ })
		}
	}

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
		confidence := clampFloat(extractionFloatFromAny(thread["confidence"], 0), 0, 1)
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
		subject := strings.TrimSpace(stringFromMap(thread, "subject"))
		stateSlot := normalizeNarrativeStateSlot(stringFromMap(thread, "state_slot"))
		if subject != "" &&
			normalizeArtifactDedupeText(subject) == normalizeArtifactDedupeText(title) &&
			stateSlot == "goal_status" {
			threadState["subject"] = subject
			threadState["state_slot"] = stateSlot
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
		worldRuleItems := worldRuleItemsForSave(extraction)
		existingWorldRules, existingWorldRulesErr := s.Store.ListWorldRules(ctx, sid)
		if existingWorldRulesErr != nil && len(worldRuleItems) > 0 {
			result.addSkipReason("world_rules", "existing_world_rules_read_failed", map[string]any{
				"count": len(worldRuleItems), "error": existingWorldRulesErr.Error(),
			})
			result.Warnings = append(result.Warnings, "world_rule_existing_read_failed")
			existingWorldRules = nil
		}
		for _, item := range worldRuleItems {
			rule := mapFromAny(item)
			key := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(rule, "key"), stringFromMap(rule, "name")))
			if key == "" {
				continue
			}
			scope := store.NormalizeWorldRuleScope(extractionFirstNonEmpty(stringFromMap(rule, "scope"), "root"))
			scopeName := stringFromMap(rule, "scope_name")
			category := extractionFirstNonEmpty(stringFromMap(rule, "category"), "custom")
			valueJSON := mustCompactJSON(extractionFirstNonEmpty(stringFromMap(rule, "value"), stringFromMap(rule, "value_json"), mustCompactJSON(rule)))
			unchanged := false
			for _, existing := range existingWorldRules {
				if !existing.Suppressed && existing.Scope == scope && existing.ScopeName == scopeName &&
					existing.Category == category && existing.Key == key && strings.TrimSpace(existing.ValueJSON) == strings.TrimSpace(valueJSON) {
					unchanged = true
					break
				}
			}
			if unchanged {
				result.addSkipReason("world_rules", "unchanged_existing_rule", map[string]any{
					"scope": scope, "scope_name": scopeName, "category": category, "key": key,
				})
				continue
			}
			wr := &store.WorldRule{
				ChatSessionID: sid,
				Scope:         scope,
				ScopeName:     scopeName,
				Category:      category,
				Key:           key,
				ValueJSON:     valueJSON,
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
		"pipeline_complete":           result.Errors == 0,
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
		"relationship_current_states": result.RelationCurrentStates,
		"relationship_state_events":   result.RelationStateEvents,
		"habit_evidence_current":      result.HabitEvidenceCurrent,
		"habit_evidence_events":       result.HabitEvidenceEvents,
		"character_profiles":          result.CharacterProfiles,
		"voice_behavior_projections":  result.VoiceBehaviorProjections,
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
	if source, ok := ctx.Value(entityIdentitySourceContextKey{}).(entityIdentitySourceContext); ok &&
		source.ContractVersion == completeTurnSourceAcceptanceContract &&
		strings.TrimSpace(source.Revision) != "" {
		details["source_revision"] = source.Revision
		details["derivation_version"] = store.MemoryAdmissionContract
		details["extractor_version"] = completeTurnCriticPipelineVersion
		details["index_version"] = memoryAdmissionIndexVersion
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

func characterStateJSONOrEmptyObject(value string) string {
	if strings.TrimSpace(value) == "" {
		return "{}"
	}
	return value
}

func sameCharacterStateProjection(left, right store.CharacterState) bool {
	return comparableEntityKey(left.CharacterName) == comparableEntityKey(right.CharacterName) &&
		strings.TrimSpace(left.AppearanceJSON) == strings.TrimSpace(right.AppearanceJSON) &&
		strings.TrimSpace(left.PersonalityJSON) == strings.TrimSpace(right.PersonalityJSON) &&
		strings.TrimSpace(left.StatusJSON) == strings.TrimSpace(right.StatusJSON) &&
		strings.TrimSpace(left.RelationshipsJSON) == strings.TrimSpace(right.RelationshipsJSON) &&
		strings.TrimSpace(left.SpeechStyleJSON) == strings.TrimSpace(right.SpeechStyleJSON)
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
