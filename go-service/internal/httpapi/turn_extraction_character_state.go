package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
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
	characterObservation := map[string]any{}
	if len(sliceFromAny(extraction["character_deltas"])) > 0 {
		characterObservation = reversibleObservationContext(ctx, s.Store, sid)
	}
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
		nextState := store.CharacterState{
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
		}
		nextState.FieldProvenanceJSON = characterDeltaFieldProvenance(ctx, currentState, nextState, currentDelta, completedTurnText, characterObservation)
		pendingCharacterStates[characterKey] = pendingCharacterStateProjection{
			state:       nextState,
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

	currentLifecycles := map[string]store.StatusCurrentValue{}
	sourceContext, _ := ctx.Value(entityIdentitySourceContextKey{}).(entityIdentitySourceContext)
	_, reversibleStore := s.Store.(store.ReversibleStatusTransitionStore)
	atomicLifecycleOwner := reversibleStore && sourceContext.ContractVersion == completeTurnSourceAcceptanceContract && sourceContext.Revision != ""
	storedPendingByKey := map[string]store.PendingThread{}
	if pending, err := s.Store.ListPendingThreads(ctx, sid, "all"); err == nil {
		for _, thread := range pending {
			if _, found := storedPendingByKey[thread.ThreadKey]; !found {
				storedPendingByKey[thread.ThreadKey] = thread
			}
		}
	}
	pendingItems := append([]any(nil), sliceFromAny(extraction["pending_threads"])...)
	if atomicLifecycleOwner {
		// Accepted-source pending writes belong to the atomic transition owner.
		// These dependent representations consume its current snapshots only.
		pendingItems = nil
	}
	if currentStore, ok := s.Store.(store.StatusCurrentValueStore); ok {
		values, err := currentStore.ListStatusCurrentValues(ctx, sid, "entity", "", narrativeStateStatusKey, -1)
		if err != nil {
			result.addSkipReason("pending_threads", "current_lifecycle_read_failed", err.Error())
		}
		seenKeys := map[string]bool{}
		for _, item := range pendingItems {
			thread := narrativePendingThreadForExtraction(sid, turnIndex, mapFromAny(item), now)
			seenKeys[thread.ThreadKey] = true
		}
		for _, value := range values {
			payload := parseJSONMap(value.ValueJSON)
			if normalizeNarrativeClaimScope(stringFromMap(payload, "claim_scope")) == "objective" {
				key := narrativeLifecycleStorageKey(stringFromMap(payload, "lifecycle_key"))
				if key == "" && stringFromMap(payload, "subject_type") == "entity" && stringFromMap(payload, "state_slot") == "goal_status" {
					key = stableKey("thread", stringFromMap(payload, "subject"))
				}
				if key != "" {
					currentLifecycles[key] = value
				}
			}
			if snapshot := narrativePendingSnapshot(payload); snapshot != nil {
				currentLifecycles[snapshot.ThreadKey] = value
				if value.SourceTurn == turnIndex && !seenKeys[snapshot.ThreadKey] {
					metadata := parseJSONMap(snapshot.HookMetadataJSON)
					metadata["title"] = extractionFirstNonEmpty(snapshot.Title, snapshot.Description)
					pendingItems = append(pendingItems, metadata)
					seenKeys[snapshot.ThreadKey] = true
				}
			}
		}
	}
	for _, item := range pendingItems {
		thread := mapFromAny(item)
		title := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(thread, "title"), stringFromMap(thread, "description"), stringFromMap(thread, "thread_type")))
		if title == "" {
			result.addSkipReason("pending_threads", "missing_title", thread)
			continue
		}
		lifecycleKey := normalizeNarrativeLifecycleKey(stringFromMap(thread, "lifecycle_key"))
		threadKey := stableKey("thread", title)
		if lifecycleStorageKey := narrativeLifecycleStorageKey(lifecycleKey); lifecycleStorageKey != "" {
			threadKey = lifecycleStorageKey
			thread["lifecycle_key"] = lifecycleKey
		}
		threadRecord := narrativePendingThreadForExtraction(sid, turnIndex, thread, now)
		if current, exists := currentLifecycles[threadKey]; exists {
			if lifecycleKey != "" && current.SourceTurn < turnIndex {
				// This is an unchanged current projection. The mention remains in
				// source memory; it does not materialize another current goal row.
				continue
			}
			payload := parseJSONMap(current.ValueJSON)
			if snapshot := narrativePendingSnapshot(payload); snapshot != nil {
				threadRecord = *snapshot
				thread = parseJSONMap(snapshot.HookMetadataJSON)
				title = extractionFirstNonEmpty(snapshot.Title, snapshot.Description)
			} else {
				threadRecord.Status = narrativeLifecycleProjectionStatus(stringFromMap(payload, "transition"))
				threadRecord.SourceTurn = current.SourceTurn
				if threadRecord.Status == "resolved" {
					threadRecord.ResolvedTurn = current.SourceTurn
					threadRecord.ResolutionNote = stringFromMap(payload, "value")
				}
				thread["status"], thread["transition"], thread["value"] = threadRecord.Status, payload["transition"], payload["value"]
				threadRecord.HookMetadataJSON = mustCompactJSON(thread)
			}
		}
		if existing, ok := storedPendingByKey[threadKey]; ok {
			threadRecord.ID, threadRecord.CreatedTurn, threadRecord.CreatedAt = existing.ID, existing.CreatedTurn, existing.CreatedAt
		}
		if saver, ok := s.Store.(pendingThreadSaver); ok && !atomicLifecycleOwner {
			result.trySave("SavePendingThread", func() error {
				return saver.SavePendingThread(ctx, &threadRecord)
			}, result, func() { result.PendingThreads++ })
		}
		s.projectNarrativePendingArtifacts(ctx, sid, turnIndex, threadRecord, extraction, now, result, existingCanonicalLayers, cost)
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

func (s *Server) projectNarrativePendingArtifacts(ctx context.Context, sid string, turnIndex int, threadRecord store.PendingThread, extraction map[string]any, now time.Time, result *artifactSaveResult, existingCanonicalLayers []store.CanonicalStateLayer, cost *canonicalStateWriteCostMeasurement) {
	thread := parseJSONMap(threadRecord.HookMetadataJSON)
	title := extractionFirstNonEmpty(threadRecord.Title, threadRecord.Description)
	threadType := strings.TrimSpace(extractionFirstNonEmpty(threadRecord.ThreadType, threadRecord.HookType))
	confidence := threadRecord.Confidence
	lifecycleKey := normalizeNarrativeLifecycleKey(stringFromMap(thread, "lifecycle_key"))
	projectionTurn := threadRecord.SourceTurn
	projectionRecordedTurn := intFromAny(thread["repair_recorded_turn"], projectionTurn)
	threadState := map[string]any{
		"thread_type": threadType,
		"title":       title,
		"status":      threadRecord.Status,
		"confidence":  confidence,
		"source_turn": projectionTurn,
	}
	threadState["transition"] = stringFromMap(thread, "transition")
	threadState["value"] = stringFromMap(thread, "value")
	if recorded, exists := thread["repair_recorded_turn"]; exists {
		threadState["repair_recorded_turn"] = recorded
	}
	if lifecycleKey != "" {
		threadState["lifecycle_key"] = lifecycleKey
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
				TurnIndex:     projectionRecordedTurn,
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
				TurnIndex:        projectionRecordedTurn,
				SourceTurn:       projectionTurn,
				SourceRecord:     0,
				LastVerifiedTurn: turnIndex,
				Confidence:       confidence,
				CreatedAt:        now,
			}, existingCanonicalLayers, cost)
		}, result, func() { result.CanonicalStateLayers++ })
	}
	if saver, ok := s.Store.(storylineSaver); ok {
		storylineStatus := threadRecord.Status
		if storylineStatus == "open" {
			storylineStatus = "active"
		}
		result.trySave("SaveStoryline", func() error {
			return saver.SaveStoryline(ctx, &store.Storyline{
				ChatSessionID:       sid,
				Name:                title,
				Status:              storylineStatus,
				EntitiesJSON:        mustCompactJSON(extraction["entities"]),
				CurrentContext:      extractionFirstNonEmpty(stringFromMap(thread, "details"), title),
				KeyPointsJSON:       mustCompactJSON([]string{title}),
				OngoingTensionsJSON: mustCompactJSON(thread),
				Confidence:          clampFloat(extractionFloatFromAny(thread["confidence"], 0), 0, 1),
				EvidenceCount:       len(stringsFromAny(extraction["evidence_excerpts"])),
				LastEvidenceTurn:    projectionTurn,
				FirstTurn:           threadRecord.CreatedTurn,
				LastTurn:            projectionRecordedTurn,
				CreatedAt:           now,
				UpdatedAt:           now,
			})
		}, result, func() { result.Storylines++ })
	}
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
		strings.TrimSpace(left.SpeechStyleJSON) == strings.TrimSpace(right.SpeechStyleJSON) &&
		strings.TrimSpace(left.FieldProvenanceJSON) == strings.TrimSpace(right.FieldProvenanceJSON)
}

func characterDeltaFieldProvenance(ctx context.Context, previous *store.CharacterState, next store.CharacterState, delta map[string]any, content string, observedAt map[string]any) string {
	next.FieldProvenanceJSON = mustCompactJSON(mapFromAny(delta["field_provenance"]))
	fields := store.DecodeCharacterFieldProvenance(store.MergeCharacterStateFieldProvenance(previous, next))
	priorValues := map[string]any{}
	if previous != nil {
		priorValues = store.CharacterStateFieldValues(*previous)
	}
	source, _ := ctx.Value(entityIdentitySourceContextKey{}).(entityIdentitySourceContext)
	for path, value := range store.CharacterStateFieldValues(next) {
		if prior, exists := priorValues[path]; exists && reflect.DeepEqual(prior, value) {
			continue
		}
		metadata := fields[path]
		if source.Revision != "" {
			metadata["source_revision"] = source.Revision
		}
		if _, explicit := metadata["observed_at"]; !explicit {
			metadata["observed_at"] = observedAt
		}
		if _, explicit := metadata["evidence_excerpt"]; !explicit {
			if excerpt := sanitizeEvidenceExcerptForTurn(stringFromMap(delta, "evidence_excerpt"), content); excerpt != "" {
				metadata["evidence_excerpt"] = excerpt
			}
		}
	}
	return mustCompactJSON(map[string]any{"contract_version": store.CharacterFieldProvenanceContract, "fields": fields})
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
