package httpapi

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	archivebridge "github.com/risulongmemory/archive-center-go/internal/archive"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func prepareTurnVectorRetrievalMethodStatus(vectorShadow map[string]any, selectedCount int) map[string]any {
	status := "unavailable"
	reason := "not_configured"
	switch strings.TrimSpace(stringFromMap(vectorShadow, "memory_search_result")) {
	case "ok":
		status = "ready"
		reason = ""
	case "not_found":
		status = "empty"
		reason = "not_found"
	case "error":
		status = "failed"
		reason = "search_failed"
	case "err_not_enabled":
		reason = "not_enabled"
	default:
		if strings.TrimSpace(stringFromMap(vectorShadow, "status")) == "degraded" {
			status = "failed"
			reason = "readiness_or_embedding_failed"
		} else if strings.TrimSpace(stringFromMap(vectorShadow, "search_skipped_reason")) != "" {
			status = "skipped"
			reason = "search_not_attempted"
		}
	}
	revisionCheckFailedCount := intFromAny(mapFromAny(vectorShadow["memory_source_revision_filter"])["dropped_check_error"], 0)
	if revisionCheckFailedCount > 0 {
		status = "failed"
		if selectedCount > 0 {
			status = "partial"
		}
		reason = "source_revision_check_failed"
	}
	return map[string]any{
		"status":                             status,
		"reason_code":                        nilIfEmpty(reason),
		"candidate_count":                    intFromAny(vectorShadow["memory_search_result_count"], len(prepareTurnVectorMemorySearchResultMaps(vectorShadow))),
		"selected_count":                     selectedCount,
		"source_revision_check_failed_count": revisionCheckFailedCount,
	}
}

func buildPrepareTurnInjectionAssembly(memories []store.Memory, kgTriples []store.KGTriple, evidence []store.DirectEvidence, chatLogs []store.ChatLog, storylines []store.Storyline, worldRules []store.WorldRule, charStates []store.CharacterState, pendingThreads []store.PendingThread, canonicalLayers []store.CanonicalStateLayer, episodeSums []store.EpisodeSummary, resumePack *store.ResumePack, personaEntries []store.PersonaMemoryEntry, characterPrivateMemories []store.ProtagonistEntityMemory, topK, maxChars int, rawUserInput, profile string, documents []map[string]any, vectorShadow map[string]any, languageContext map[string]any, perspectiveContextArg ...map[string]any) prepareTurnInjectionAssembly {
	return buildPrepareTurnInjectionAssemblyWithBudget(memories, kgTriples, evidence, chatLogs, storylines, worldRules, charStates, pendingThreads, canonicalLayers, episodeSums, resumePack, personaEntries, characterPrivateMemories, topK, maxChars, rawUserInput, profile, documents, vectorShadow, languageContext, "auto", nil, perspectiveContextArg...)
}

func buildPrepareTurnInjectionAssemblyWithBudget(memories []store.Memory, kgTriples []store.KGTriple, evidence []store.DirectEvidence, chatLogs []store.ChatLog, storylines []store.Storyline, worldRules []store.WorldRule, charStates []store.CharacterState, pendingThreads []store.PendingThread, canonicalLayers []store.CanonicalStateLayer, episodeSums []store.EpisodeSummary, resumePack *store.ResumePack, personaEntries []store.PersonaMemoryEntry, characterPrivateMemories []store.ProtagonistEntityMemory, topK, maxChars int, rawUserInput, profile string, documents []map[string]any, vectorShadow map[string]any, languageContext map[string]any, memoryDeliveryBudgetMode string, memoryDeliveryBudgets map[string]int, perspectiveContextArg ...map[string]any) prepareTurnInjectionAssembly {
	topK = prepareTurnRecallLimit(topK)
	maxChars = prepareTurnTextBudget(maxChars)
	canonicalMemories := memories
	generalMemories, publicProjectionTrace := projectPrepareTurnGeneralMemories(canonicalMemories)
	recallLimit := len(memories) + len(kgTriples) + len(evidence) + len(chatLogs) + len(storylines) + len(worldRules) + len(charStates) + len(pendingThreads) + len(canonicalLayers) + len(episodeSums) + len(personaEntries) + len(characterPrivateMemories)
	languageContext = normalizeCompleteTurnLanguageContext(languageContext)
	perspectiveContext := map[string]any(nil)
	perspectiveCandidateText := ""
	perspectiveCandidateCount := 0
	interactionPublicCandidateText := ""
	interactionGuardedCandidateText := ""
	interactionCandidateCount := 0
	characterMemoryReadContext := map[string]any(nil)
	entityIdentityAliases := map[string]any(nil)
	if len(perspectiveContextArg) > 0 {
		perspectiveContext = normalizePrepareTurnPerspectiveContext(perspectiveContextArg[0])
		perspectiveCandidateText = strings.TrimSpace(extractionStringFromAny(perspectiveContextArg[0]["_character_perspective_text"]))
		perspectiveCandidateCount = intFromAny(perspectiveContextArg[0]["_character_perspective_candidate_count"], 0)
		interactionPublicCandidateText = strings.TrimSpace(extractionStringFromAny(perspectiveContextArg[0]["_active_interaction_public_text"]))
		interactionGuardedCandidateText = strings.TrimSpace(extractionStringFromAny(perspectiveContextArg[0]["_active_interaction_guarded_text"]))
		interactionCandidateCount = intFromAny(perspectiveContextArg[0]["_active_interaction_candidate_count"], 0)
		characterMemoryReadContext = mapFromAny(perspectiveContextArg[0][prepareTurnCharacterMemoryContextKey])
		entityIdentityAliases = mapFromAny(perspectiveContextArg[0][prepareTurnEntityIdentityAliasesContextKey])
	}
	evidenceInputCount := len(evidence)
	evidence, perspectiveBlockedEvidenceIDs := filterPrepareTurnPerspectiveScopedEvidence(evidence, canonicalMemories)
	evidenceHydrationSource := append([]store.DirectEvidence{}, evidence...)
	narrativeCurrentValues, activeStates := prepareTurnNarrativeStateFromPerspective(perspectiveContextArg)

	out := prepareTurnInjectionAssembly{
		LanguageContext:    languageContext,
		PerspectiveContext: perspectiveContext,
		Counts: map[string]any{
			"memory_count":                         len(memories),
			"kg_count":                             len(kgTriples),
			"fallback_chat_log_count":              len(chatLogs),
			"evidence_input_count":                 evidenceInputCount,
			"evidence_count":                       len(evidence),
			"perspective_evidence_filtered_count":  evidenceInputCount - len(evidenceHydrationSource),
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
			"support_candidate_limit":              recallLimit,
			"support_candidate_limit_source":       "processing_safety_bound_independent_of_top_k_and_final_delivery",
			"top_k_definition":                     "vector_memory_search_limit_only",
		},
	}
	protectedPerspectiveContext := prepareTurnProtectedPerspectiveContext(perspectiveContext, canonicalMemories, charStates)
	for key, value := range publicProjectionTrace {
		out.Counts[key] = value
	}
	memories = generalMemories
	recollectionContext := buildPrepareTurnRecollectionContext(rawUserInput, memories, activeStates, canonicalLayers, pendingThreads, chatLogs)
	guardRecollectionContext := buildPrepareTurnRecollectionContext(rawUserInput, canonicalMemories, activeStates, canonicalLayers, pendingThreads, chatLogs)
	recollectionContext.previousEventGuardSummary = guardRecollectionContext.previousEventGuardSummary
	knownCharacterNames := make([]string, 0, len(charStates)+len(characterPrivateMemories))
	for _, state := range charStates {
		knownCharacterNames = append(knownCharacterNames, state.CharacterName)
	}
	for _, memory := range characterPrivateMemories {
		knownCharacterNames = append(knownCharacterNames, prepareTurnMemoryOwnerLabel(memory.OwnerEntityKey, memory.OwnerEntityName))
	}
	for _, memory := range memories {
		knownCharacterNames = append(knownCharacterNames, prepareTurnMemoryCharacterAnchors(memory)...)
	}
	knownCharacterNames = append(knownCharacterNames, prepareTurnCanonicalKnownCharacterNames(canonicalLayers)...)
	entityScope := buildPrepareTurnRequestEntityScopeWithAliases(
		rawUserInput,
		recollectionContext.currentEntities,
		knownCharacterNames,
		entityIdentityAliases,
		recollectionContext.currentAssistantContext,
		extractionStringFromAny(perspectiveContext["current_pov"]),
	)
	objectiveEntityNames := entityScope.Scene
	objectiveEntitySource := "stored_active_scene_state"
	if len(objectiveEntityNames) == 0 {
		objectiveEntitySource = "unobserved_no_objective_state_delivery"
	}
	memoryQuery := prepareTurnEventMemoryQuery(recollectionContext, entityScope.Direct)
	out.MemoryRecallQuery = memoryQuery
	out.Counts["recall_query_sources"] = []string{"current_user_input", "directly_referenced_entities", "relevant_previous_stored_event_summary"}
	out.Counts["directly_referenced_entities"] = entityScope.Direct
	out.Counts["stored_active_scene_entities"] = entityScope.Scene
	out.Counts["accepted_recent_context_entities"] = entityScope.Scene
	out.Counts["objective_entity_source"] = objectiveEntitySource
	out.Counts["previous_assistant_raw_used_for_search"] = false
	out.Counts["previous_assistant_used_for_entity_scope_only"] = recollectionContext.currentAssistantContext != ""
	out.Counts["previous_assistant_raw_delivery_owner"] = "input_context_only"
	out.Counts["current_scene_state_turn"] = recollectionContext.currentSceneTurn
	out.Counts["latest_assistant_turn"] = recollectionContext.latestAssistantTurn
	out.Counts["current_scene_state_is_current"] = recollectionContext.currentSceneIsCurrent
	memorySelection := selectPrepareTurnMemoryLanesWithVectorHydrationSource(memories, canonicalMemories, memoryQuery, topK, vectorShadow, entityScope.Direct, entityScope.Scene)
	memorySelection = filterPrepareTurnProtectedMemoryLaneSelection(memorySelection, recollectionContext, protectedPerspectiveContext)
	protectedSelection := prepareTurnMemoryLaneSelection{
		ProtectedAliasCanonical: map[string]string{},
		ProtectedAmbiguousAlias: map[string]bool{},
		Trace:                   map[string]any{},
	}
	protectedSelection.ProtectedAliasCanonical, protectedSelection.ProtectedAmbiguousAlias = prepareTurnProtectedAliasResolution(canonicalMemories)
	for _, item := range canonicalMemories {
		if !prepareTurnProtectedMemoryGuard(item).Active {
			continue
		}
		protectedSelection.ProtectedCandidates = append(protectedSelection.ProtectedCandidates, item)
		protectedSelection.ProtectedSelected = append(protectedSelection.ProtectedSelected, item)
	}
	protectedSelection = filterPrepareTurnProtectedMemoryLaneSelection(protectedSelection, recollectionContext, protectedPerspectiveContext)
	memorySelection.ProtectedSelected = protectedSelection.ProtectedSelected
	memorySelection.ProtectedCandidates = protectedSelection.ProtectedCandidates
	memorySelection.ProtectedAliasCanonical = protectedSelection.ProtectedAliasCanonical
	memorySelection.ProtectedAmbiguousAlias = protectedSelection.ProtectedAmbiguousAlias
	for _, key := range []string{
		"protected_memory_before_filter",
		"protected_memory_after_filter",
		"protected_memory_dropped_count",
		"protected_memory_gate",
		"protected_memory_dropped",
	} {
		memorySelection.Trace[key] = protectedSelection.Trace[key]
	}
	memorySelection.Trace["protected_guard_selected"] = len(memorySelection.ProtectedSelected)
	memorySelection.Trace["protected_guard_candidate_safety_limit"] = len(protectedSelection.ProtectedCandidates)
	exactPhraseSelected := 0
	lexicalSelected := 0
	for _, item := range memorySelection.Relevant {
		evidence := prepareTurnMemoryRecallEvidence(memoryQuery, item)
		if evidence.ExactPhrase {
			exactPhraseSelected++
		} else if evidence.LexicalOverlap {
			lexicalSelected++
		}
	}
	memorySelection.Trace["exact_phrase_selected_count"] = exactPhraseSelected
	memorySelection.Trace["lexical_selected_count"] = lexicalSelected
	// Each support lane owns its request-scoped evidence. A prior event may help
	// retrieve event memory, but cannot activate state, relationship, world,
	// evidence, or goal lanes.
	relationshipQuery := prepareTurnEntityScopeQuery(rawUserInput, entityScope.Direct, entityScope.Scene)
	rawSupportQuery := prepareTurnEntityScopeQuery(rawUserInput, entityScope.Direct)
	objectiveQuery := strings.TrimSpace(strings.Join(nonEmptyStrings([]string{
		recollectionContext.currentSceneStates,
		strings.Join(objectiveEntityNames, "\n"),
	}), "\n"))
	worldQuery := strings.TrimSpace(strings.Join(nonEmptyStrings([]string{
		rawUserInput,
		recollectionContext.currentSceneStates,
		strings.Join(entityScope.Scene, "\n"),
	}), "\n"))
	goalQuery := prepareTurnEntityScopeQuery(rawUserInput, entityScope.Direct, entityScope.Scene)
	out.ContinuityCorrectionText, out.Counts["continuity_correction"] = buildNarrativeContinuityCorrection(
		narrativeCurrentValues,
		rawUserInput,
		chatLogs,
		activeStates,
		memorySelection,
		recallLimit,
	)
	memoryLines, memoryLanguageTrace := prepareTurnMemoryLaneLines(memorySelection, languageContext, protectedPerspectiveContext)
	actualMemoryLines := stringsFromAny(memoryLanguageTrace["actual_lines"])
	protectedMemoryLines := stringsFromAny(memoryLanguageTrace["protected_lines"])
	out.MemoryDeliveryLineage = buildPrepareTurnMemoryDeliveryLineage(memorySelection, memoryLanguageTrace)
	for k, v := range prepareTurnMemoryLaneProtectedCounts(memorySelection, protectedPerspectiveContext) {
		out.Counts[k] = v
	}
	out.Counts["memory_injected_line_count"] = len(actualMemoryLines)
	out.Counts["protected_memory_injected_line_count"] = len(protectedMemoryLines)
	out.Counts["memory_final_render_duplicate_count"] = intFromAny(memoryLanguageTrace["final_render_duplicate_count"], 0)
	out.Counts["protected_perspective_recognized"] = len(protectedPerspectiveContext) > 0
	if len(perspectiveContext) > 0 && len(protectedPerspectiveContext) == 0 {
		out.Counts["protected_perspective_ignored_reason"] = "current_pov_not_recognized_as_character"
	}
	artifactHydration := prepareTurnHydrateVectorArtifactHits(
		evidenceHydrationSource, worldRules, vectorShadow, recallLimit, perspectiveBlockedEvidenceIDs,
	)
	out.LanguageInjectionTrace = buildPrepareTurnLanguageInjectionTrace(languageContext, memoryLanguageTrace)
	out.MemoryText = makePrepareTurnSection("[Memory]", memoryLines)
	out.ActualMemoryText = makePrepareTurnSection("[Memory]", actualMemoryLines)
	if perspectiveCandidateText != "" {
		for _, line := range strings.Split(perspectiveCandidateText, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || line == "[Character Perspective]" {
				continue
			}
			protectedMemoryLines = append(protectedMemoryLines, line)
		}
		out.Counts["character_perspective_candidate_count"] = perspectiveCandidateCount
		out.Counts["protected_memory_injected_line_count"] = len(protectedMemoryLines)
	}
	if interactionGuardedCandidateText != "" {
		protectedMemoryLines = append(protectedMemoryLines, prepareTurnDeliveryItems(interactionGuardedCandidateText)...)
		out.Counts["active_interaction_candidate_count"] = interactionCandidateCount
		out.Counts["protected_memory_injected_line_count"] = len(protectedMemoryLines)
	}
	out.ProtectedMemoryText = makePrepareTurnSection("[Protected Memory Guidance]", protectedMemoryLines)

	kgLines := make([]string, 0, minInt(len(kgTriples), recallLimit))
	kgClosedDropped := 0
	kgIrrelevantDropped := 0
	kgSingleEndpointDropped := 0
	kgReferenceTurn := prepareTurnMaxObservedTurn(chatLogs, nil)
	for _, t := range kgTriples {
		if len(kgLines) >= recallLimit {
			break
		}
		if kgReferenceTurn > 0 && ((t.ValidTo > 0 && t.ValidTo < kgReferenceTurn) || (t.ValidFrom > 0 && t.ValidFrom > kgReferenceTurn)) {
			kgClosedDropped++
			continue
		}
		relation := strings.TrimSpace(fmt.Sprintf("%s --%s--> %s", t.Subject, t.Predicate, t.Object))
		if relation == "-->" {
			continue
		}
		eligible, reason := prepareTurnKGRecallEligible(relationshipQuery, t)
		if !eligible {
			kgIrrelevantDropped++
			if reason == "single_endpoint_only" {
				kgSingleEndpointDropped++
			}
			continue
		}
		sourceTurn := "unrecorded"
		if t.SourceTurn > 0 {
			sourceTurn = strconv.Itoa(t.SourceTurn)
		}
		validFrom := "unrecorded"
		if t.ValidFrom > 0 {
			validFrom = strconv.Itoa(t.ValidFrom)
		}
		validTo := "end_unrecorded"
		if t.ValidTo > 0 {
			validTo = strconv.Itoa(t.ValidTo)
		}
		kgLines = append(kgLines, fmt.Sprintf(
			"- [source_turn=%s; valid=%s..%s] %s",
			sourceTurn, validFrom, validTo, relation,
		))
	}
	out.KGText = makePrepareTurnSection("[Knowledge Graph Support History; context only, not current-state authority; end_unrecorded means no closing turn recorded]", kgLines)

	directEvidenceLines := make([]string, 0, len(artifactHydration.Evidence))
	for _, ev := range artifactHydration.Evidence {
		text := compactPrepareTurnLine(ev.EvidenceText, 0)
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
	out.FallbackText = ""
	out.Counts["raw_chat_fallback_enabled"] = false
	out.Counts["raw_chat_fallback_reason"] = "chat_logs_have_no_public_memory_projection_use_input_context"

	storylinesForInjection := collapsePrepareTurnStorylines(storylines)
	storylineLines := make([]string, 0, minInt(len(storylinesForInjection), recallLimit))
	storylineIrrelevantDropped := 0
	for _, sl := range storylinesForInjection {
		if len(storylineLines) >= recallLimit {
			break
		}
		desc := strings.TrimSpace(sl.CurrentContext)
		if desc == "" {
			desc = strings.TrimSpace(sl.Name)
		}
		desc = compactPrepareTurnLine(desc, 0)
		if desc != "" {
			if !prepareTurnRequestFirstRelevant(rawSupportQuery, goalQuery, desc) {
				storylineIrrelevantDropped++
				continue
			}
			storylineLines = append(storylineLines, "- "+desc)
		}
	}
	out.StorylineText = makePrepareTurnSection("[Storylines]", storylineLines)

	worldRulesForInjection := collapsePrepareTurnWorldRules(mergePrepareTurnWorldRulesForInjection(artifactHydration.WorldRules, worldRules))
	worldRuleValueJSON := func(raw any) string {
		switch value := raw.(type) {
		case nil:
			return ""
		case string:
			value = strings.TrimSpace(value)
			if value == "" {
				return ""
			}
			return mustCompactJSON(parseSurfacePayload(value))
		default:
			return mustCompactJSON(value)
		}
	}
	worldRuleSignature := func(sessionID, scope, scopeName, category, key, valueJSON string) string {
		sessionID = strings.TrimSpace(sessionID)
		key = strings.TrimSpace(key)
		valueJSON = strings.TrimSpace(valueJSON)
		if sessionID == "" || key == "" || valueJSON == "" {
			return ""
		}
		scope = store.NormalizeWorldRuleScope(extractionFirstNonEmpty(scope, "root"))
		category = extractionFirstNonEmpty(strings.TrimSpace(category), "custom")
		return strings.Join([]string{
			sessionID,
			scope,
			strings.TrimSpace(scopeName),
			category,
			key,
			valueJSON,
		}, "\x1f")
	}
	worldStateRuleSignature := func(sessionID string, raw any) string {
		rule := mapFromAny(raw)
		if len(rule) == 0 {
			return ""
		}
		key := extractionFirstNonEmpty(stringFromMap(rule, "key"), stringFromMap(rule, "name"))
		var valueJSON string
		if value, ok := rule["value"]; ok {
			valueJSON = worldRuleValueJSON(value)
		} else if value, ok := rule["value_json"]; ok {
			valueJSON = worldRuleValueJSON(value)
		}
		return worldRuleSignature(
			sessionID,
			stringFromMap(rule, "scope"),
			stringFromMap(rule, "scope_name"),
			stringFromMap(rule, "category"),
			key,
			valueJSON,
		)
	}
	hydratedWorldRuleIDs := make(map[int64]bool, len(artifactHydration.WorldRules))
	for _, wr := range artifactHydration.WorldRules {
		if wr.ID > 0 {
			hydratedWorldRuleIDs[wr.ID] = true
		}
	}
	currentSceneWorldTerms := prepareTurnDistinctiveRecallTerms(recollectionContext.currentSceneStates, entityScope.Scene...)
	prioritizedWorldRules := make([]store.WorldRule, 0, len(worldRulesForInjection))
	for _, wr := range worldRulesForInjection {
		scope := strings.ToLower(strings.TrimSpace(wr.Scope))
		if wr.Pinned || scope == "root" || scope == "global" {
			prioritizedWorldRules = append(prioritizedWorldRules, wr)
		}
	}
	for _, wr := range worldRulesForInjection {
		scope := strings.ToLower(strings.TrimSpace(wr.Scope))
		if wr.Pinned || scope == "root" || scope == "global" {
			continue
		}
		prioritizedWorldRules = append(prioritizedWorldRules, wr)
	}
	worldRuleLines := make([]string, 0, minInt(len(worldRulesForInjection), recallLimit))
	selectedWorldRuleSignatures := map[string]bool{}
	worldRuleIrrelevantDropped := 0
	worldRulePersistentSelected := 0
	for _, wr := range prioritizedWorldRules {
		if len(worldRuleLines) >= recallLimit {
			break
		}
		desc := strings.TrimSpace(wr.Key)
		if desc == "" {
			desc = strings.TrimSpace(wr.Scope)
		}
		if value := prepareTurnSurfaceText(parseSurfacePayload(wr.ValueJSON)); value != "" {
			desc = strings.TrimSpace(desc + ": " + value)
		}
		if desc != "" {
			worldAnchors := []string{wr.ScopeName, wr.Key}
			scope := strings.ToLower(strings.TrimSpace(wr.Scope))
			persistent := wr.Pinned || scope == "root" || scope == "global"
			sceneScoped := scope == "location" || scope == "region" || scope == "area" || scope == "place"
			relevant := persistent || (wr.ID > 0 && hydratedWorldRuleIDs[wr.ID])
			currentEntityBindings := 0
			for _, entityName := range entityScope.Scene {
				if prepareTurnRecallContainsAnchor(desc, entityName) {
					currentEntityBindings++
				}
			}
			if !relevant && currentEntityBindings > 1 {
				relevant = true
			}
			if !relevant && sceneScoped {
				relevant = strings.TrimSpace(wr.ScopeName) != "" &&
					prepareTurnRecallContainsAnchor(worldQuery, wr.ScopeName)
			}
			if !relevant && len(currentSceneWorldTerms) > 0 {
				relevant = prepareTurnDistinctiveRecallOverlapCount(currentSceneWorldTerms, desc) > 0
			}
			if !relevant && !sceneScoped {
				relevant = prepareTurnRequestFirstRelevant(rawSupportQuery, worldQuery, desc, worldAnchors...)
			}
			if !relevant {
				worldRuleIrrelevantDropped++
				continue
			}
			if persistent {
				worldRulePersistentSelected++
			}
			worldRuleLines = append(worldRuleLines, "- "+desc)
			if signature := worldRuleSignature(
				wr.ChatSessionID,
				wr.Scope,
				wr.ScopeName,
				wr.Category,
				wr.Key,
				worldRuleValueJSON(wr.ValueJSON),
			); signature != "" {
				selectedWorldRuleSignatures[signature] = true
			}
		}
	}
	out.WorldRulesText = makePrepareTurnSection("[World Rules]", worldRuleLines)

	charLines := make([]string, 0, len(charStates))
	charObjectiveLines := make([]string, 0, len(charStates))
	charRelationshipLines := make([]string, 0, len(charStates))
	characterIrrelevantDropped := 0
	characterRelationshipIrrelevantDropped := 0
	typedVoiceProjectionDeferred := 0
	type characterCandidate struct {
		state             store.CharacterState
		name              string
		stateText         string
		relationships     string
		speechStyle       string
		detail            string
		sceneActive       bool
		directMentionRank int
		supportOverlap    int
		sourceOrder       int
	}
	characterCandidates := make([]characterCandidate, 0, len(charStates))
	currentEntityNames := entityScope.Known
	currentEntityAliases := prepareTurnExplicitAliasLists(entityIdentityAliases)
	currentSceneEntityNames := entityScope.Scene
	for sourceOrder, cs := range charStates {
		name := strings.TrimSpace(cs.CharacterName)
		sceneActive := prepareTurnRelationshipNameInList(name, objectiveEntityNames)
		state := ""
		speechStyle := ""
		if sceneActive {
			// Existing character-state rows remain a read-only compatibility
			// surface until source-backed rebuild has materialized their
			// reversible fields. New writes no longer put reversible current
			// values in this legacy projection.
			state = prepareTurnSurfaceText(sanitizeLegacyReversibleMap(parseSurfacePayload(cs.StatusJSON)))
			speechPayload := parseSurfacePayload(cs.SpeechStyleJSON)
			if speechMap := mapFromAny(speechPayload); extractionStringFromAny(speechMap["contract_version"]) == voiceBehaviorProjectionContractVersion {
				// 3.9-D owns durable modeling. 3.9-E will own scoped,
				// privacy-aware delivery; the legacy character surface must not
				// become a parallel injection path for the typed projection.
				typedVoiceProjectionDeferred++
			} else {
				speechStyle = prepareTurnSurfaceText(speechPayload)
			}
		}
		relationships, relationshipDropped := prepareTurnRelevantRelationshipSurface(cs.RelationshipsJSON, name, rawUserInput, currentSceneEntityNames, currentEntityNames)
		characterRelationshipIrrelevantDropped += relationshipDropped
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
		detail := compactPrepareTurnLine(strings.Join(parts, "; "), 0)
		if name == "" && detail == "" {
			continue
		}
		directMentionRank := prepareTurnDirectEntityMentionRank(rawUserInput, name, currentEntityAliases)
		if !sceneActive && relationships == "" {
			characterIrrelevantDropped++
			continue
		}
		characterCandidates = append(characterCandidates, characterCandidate{
			state:             cs,
			name:              name,
			stateText:         state,
			relationships:     relationships,
			speechStyle:       speechStyle,
			detail:            detail,
			sceneActive:       sceneActive,
			directMentionRank: directMentionRank,
			supportOverlap:    prepareTurnRecallOverlapCount(relationshipQuery, name+" "+detail),
			sourceOrder:       sourceOrder,
		})
	}
	sort.SliceStable(characterCandidates, func(i, j int) bool {
		left, right := characterCandidates[i], characterCandidates[j]
		if left.directMentionRank != right.directMentionRank {
			return left.directMentionRank > right.directMentionRank
		}
		if left.supportOverlap != right.supportOverlap {
			return left.supportOverlap > right.supportOverlap
		}
		if left.state.TurnIndex != right.state.TurnIndex {
			return left.state.TurnIndex > right.state.TurnIndex
		}
		return left.sourceOrder < right.sourceOrder
	})
	for _, candidate := range characterCandidates {
		name := candidate.name
		state := candidate.stateText
		relationships := candidate.relationships
		speechStyle := candidate.speechStyle
		detail := candidate.detail
		charLines = append(charLines, fmt.Sprintf("- %s: %s", name, detail))
		objectiveParts := []string{}
		if state != "" {
			objectiveParts = append(objectiveParts, "state="+state)
		}
		if speechStyle != "" {
			objectiveParts = append(objectiveParts, "speech_style="+speechStyle)
		}
		if candidate.sceneActive {
			if objective := compactPrepareTurnLine(strings.Join(objectiveParts, "; "), 0); objective != "" {
				charObjectiveLines = append(charObjectiveLines, fmt.Sprintf("- %s: %s", name, objective))
			}
		}
		if relationships != "" {
			charRelationshipLines = append(charRelationshipLines, fmt.Sprintf("- %s: relationships=%s", name, compactPrepareTurnLine(relationships, 0)))
		}
	}
	out.CharacterMemorySupport = buildPrepareTurnCharacterMemorySupport(
		extractionStringFromAny(characterMemoryReadContext["chat_session_id"]),
		charStates,
		entityScope,
		perspectiveContext,
		characterMemoryReadContext,
	)
	charObjectiveLines = append(charObjectiveLines, prepareTurnCharacterMemoryLines(out.CharacterMemorySupport, "character_objective")...)
	charRelationshipLines = append(charRelationshipLines, prepareTurnCharacterMemoryLines(out.CharacterMemorySupport, "subjective_relationship")...)
	out.CharacterText = makePrepareTurnSection("[Characters]", charLines)
	out.CharacterObjectiveText = makePrepareTurnSection("[Character Objective States]", charObjectiveLines)
	if interactionPublicCandidateText != "" {
		for _, line := range prepareTurnDeliveryItems(interactionPublicCandidateText) {
			charRelationshipLines = append(charRelationshipLines, line)
		}
		out.Counts["active_interaction_candidate_count"] = interactionCandidateCount
	}
	out.CharacterRelationshipText = makePrepareTurnSection("[Character Relationships]", charRelationshipLines)

	pendingLines := make([]string, 0, minInt(len(pendingThreads), recallLimit))
	pendingIrrelevantDropped := 0
	pendingPinnedActiveSelected := 0
	pendingSuppressedDropped := 0
	for _, pt := range pendingThreads {
		if len(pendingLines) >= recallLimit {
			break
		}
		if pt.Suppressed {
			pendingSuppressedDropped++
			continue
		}
		rawDescription := strings.TrimSpace(pt.Description)
		desc := compactPrepareTurnLine(rawDescription, 0)
		status := strings.TrimSpace(pt.Status)
		if status != "" && desc != "" {
			desc = compactPrepareTurnLine("status="+status+"; "+desc, 0)
		}
		if desc != "" {
			pinnedActive := pt.Pinned && strings.EqualFold(status, "open")
			if !pinnedActive && !prepareTurnRequestFirstRelevant(rawSupportQuery, goalQuery, desc) {
				pendingIrrelevantDropped++
				continue
			}
			pendingLines = append(pendingLines, "- "+desc)
			if pinnedActive {
				pendingPinnedActiveSelected++
			}
		}
	}
	out.PendingThreadText = makePrepareTurnSection("[Pending Threads]", pendingLines)

	episodeLines := make([]string, 0, minInt(len(episodeSums), recallLimit))
	episodeIrrelevantDropped := 0
	for _, es := range episodeSums {
		if len(episodeLines) >= recallLimit {
			break
		}
		summary := compactPrepareTurnLine(es.SummaryText, 0)
		if summary == "" {
			summary = fmt.Sprintf("Episode %d-%d", es.FromTurn, es.ToTurn)
		}
		if anchors := episodeDenseAnchorPreview(es, summary, 0); anchors != "" {
			summary = compactPrepareTurnLine(summary+"; "+anchors, 0)
		}
		if !prepareTurnSupportRecallEligible(memoryQuery, summary) {
			episodeIrrelevantDropped++
			continue
		}
		episodeLines = append(episodeLines, fmt.Sprintf("- turns %d-%d: %s", es.FromTurn, es.ToTurn, summary))
	}
	out.EpisodeText = makePrepareTurnSection("[Episode Summaries]", episodeLines)
	hierarchyEscalation := buildPrepareTurnHierarchyEscalation(resumePack, chatLogs, memorySelection, rawUserInput, profile)
	out.ChapterText = hierarchyEscalation.ChapterText
	out.ArcText = hierarchyEscalation.ArcText
	out.SagaText = hierarchyEscalation.SagaText
	out.PersonaText = buildPersonaRecollectionText(personaEntries, maxChars)
	out.CharacterPrivateText = buildCharacterPrivateRecollectionText(characterPrivateMemories, maxChars)

	if latest := latestPrepareTurnEvidence(evidence); latest != nil {
		out.LatestDirectEvidenceText = compactPrepareTurnLine(latest.EvidenceText, 0)
	}
	out.RecentRawTurnText = recentPrepareTurnRawTurn(chatLogs)
	out.ScopedVerbatimSupport = archivebridge.BuildScopedVerbatimSupport(evidence)
	out.ScopedVerbatimText = out.ScopedVerbatimSupport.Text

	canonLines := make([]string, 0, minInt(len(canonicalLayers), recallLimit))
	canonEventLines := []string{}
	canonCharacterLines := []string{}
	canonRelationshipLines := []string{}
	canonWorldLines := []string{}
	canonFiltered := 0
	canonIrrelevant := 0
	canonRelationshipIrrelevant := 0
	canonCharacterRosterOnlyDropped := 0
	canonTypeCounts := map[string]int{}
	canonicalObservedTurn := func(layer store.CanonicalStateLayer) int {
		return maxInt(layer.TurnIndex, maxInt(layer.SourceTurn, layer.LastVerifiedTurn))
	}
	latestObservedStateTurn := map[string]int{}
	for _, layer := range canonicalLayers {
		if !canonicalLayerEligibleForCurrentTruth(layer) {
			continue
		}
		layerType := strings.TrimSpace(layer.LayerType)
		if layerType != "scene_state" && layerType != "world_state" {
			continue
		}
		latestObservedStateTurn[layerType] = maxInt(latestObservedStateTurn[layerType], canonicalObservedTurn(layer))
	}
	stateLayerLabel := func(layerType string, layer store.CanonicalStateLayer) string {
		observedTurn := canonicalObservedTurn(layer)
		if observedTurn <= 0 {
			return layerType
		}
		status := "historical"
		if observedTurn == latestObservedStateTurn[layerType] {
			status = "latest_observed"
		}
		return fmt.Sprintf("%s [%s turn=%d]", layerType, status, observedTurn)
	}
	for _, cl := range canonicalLayers {
		if len(canonLines) >= recallLimit {
			break
		}
		if !canonicalLayerEligibleForCurrentTruth(cl) {
			canonFiltered++
			continue
		}
		content := prepareTurnSurfaceText(parseSurfacePayload(cl.Content))
		if content == "" {
			continue
		}
		layer := strings.TrimSpace(cl.LayerType)
		if layer == "" {
			layer = "state"
		}
		if layer == "relationship_state" {
			filtered, dropped := prepareTurnRelevantCanonicalRelationshipSurface(cl.Content, rawUserInput, currentSceneEntityNames, currentEntityNames)
			canonRelationshipIrrelevant += dropped
			if filtered == "" {
				canonIrrelevant++
				continue
			}
			line := fmt.Sprintf("- %s: %s", layer, filtered)
			canonLines = append(canonLines, line)
			canonRelationshipLines = append(canonRelationshipLines, line)
			canonTypeCounts[layer]++
			continue
		}
		selectedLayer := false
		switch layer {
		case "entity_state":
			if cl.TurnIndex > 0 && recollectionContext.latestAssistantTurn > 0 && cl.TurnIndex < recollectionContext.latestAssistantTurn {
				canonIrrelevant++
				continue
			}
			var entity map[string]any
			if json.Unmarshal([]byte(cl.Content), &entity) != nil || len(entity) == 0 {
				canonIrrelevant++
				continue
			}
			appendCanonicalSubset := func(target *[]string, rawQuery, fallbackQuery string, subset map[string]any) {
				if len(subset) == 0 {
					return
				}
				encoded, _ := json.Marshal(subset)
				text := compactPrepareTurnLine(string(encoded), 0)
				if text == "" || !prepareTurnRequestFirstRelevant(rawQuery, fallbackQuery, text) {
					return
				}
				line := "- entity_state: " + text
				*target = append(*target, line)
				canonLines = append(canonLines, line)
				selectedLayer = true
			}
			if value, ok := entity["events"]; ok {
				appendCanonicalSubset(&canonEventLines, memoryQuery, "", map[string]any{"events": value})
			}
			if characters, ok := entity["characters"]; ok {
				if filtered, ok := prepareTurnCanonicalCharactersForScene(characters, objectiveEntityNames); ok {
					appendCanonicalSubset(&canonCharacterLines, objectiveQuery, "", map[string]any{"characters": filtered})
				} else {
					canonCharacterRosterOnlyDropped++
				}
			}
			worldSubset := map[string]any{}
			for _, key := range []string{"background", "items"} {
				if value, ok := entity[key]; ok {
					worldSubset[key] = value
				}
			}
			appendCanonicalSubset(&canonWorldLines, rawSupportQuery, worldQuery, worldSubset)
		case "scene_state":
			if cl.TurnIndex > 0 && recollectionContext.latestAssistantTurn > 0 && cl.TurnIndex < recollectionContext.latestAssistantTurn {
				canonIrrelevant++
				continue
			}
			content = prepareTurnSceneStateWithoutUnresolvedThreads(cl.Content)
			if content == "" {
				canonIrrelevant++
				continue
			}
			if prepareTurnRequestFirstRelevant(rawSupportQuery, worldQuery, content) {
				line := fmt.Sprintf("- %s: %s", stateLayerLabel(layer, cl), content)
				canonLines = append(canonLines, line)
				canonWorldLines = append(canonWorldLines, line)
				selectedLayer = true
			}
		case "unresolved_threads":
			canonIrrelevant++
			continue
		case "world_state":
			if state := mapFromAny(parseSurfacePayload(cl.Content)); len(state) > 0 {
				if rawRules, ok := state["rules"]; ok {
					keptRules := make([]any, 0, len(sliceFromAny(rawRules)))
					for _, rawRule := range sliceFromAny(rawRules) {
						signature := worldStateRuleSignature(cl.ChatSessionID, rawRule)
						if signature != "" && selectedWorldRuleSignatures[signature] {
							continue
						}
						keptRules = append(keptRules, rawRule)
					}
					if len(keptRules) == 0 {
						delete(state, "rules")
					} else {
						state["rules"] = keptRules
					}
				}
				hasStateContent := false
				for key, value := range state {
					switch key {
					case "version", "confidence", "verification":
						continue
					}
					if hasMeaningfulPayload(value) {
						hasStateContent = true
						break
					}
				}
				if hasStateContent {
					content = prepareTurnSurfaceText(state)
				} else {
					content = ""
				}
			}
			if prepareTurnRequestFirstRelevant(rawSupportQuery, worldQuery, content) {
				line := fmt.Sprintf("- %s: %s", stateLayerLabel(layer, cl), content)
				canonLines = append(canonLines, line)
				canonWorldLines = append(canonWorldLines, line)
				selectedLayer = true
			}
		default:
			// Canonical current-state layers are already truth-filtered. One exact
			// request/scene term is sufficient here; the stricter historical-memory
			// threshold would discard concise location/state facts.
			if prepareTurnDistinctiveRecallOverlapCount(prepareTurnDistinctiveRecallTerms(strings.TrimSpace(rawSupportQuery+"\n"+worldQuery)), content) > 0 {
				line := fmt.Sprintf("- %s: %s", layer, content)
				canonLines = append(canonLines, line)
				canonWorldLines = append(canonWorldLines, line)
				selectedLayer = true
			}
		}
		if !selectedLayer {
			canonIrrelevant++
			continue
		}
		canonTypeCounts[layer]++
	}
	out.CanonText = makePrepareTurnSection("[Canonical State]", canonLines)
	out.CanonEventText = makePrepareTurnSection("[Canonical Events]", canonEventLines)
	out.CanonCharacterText = makePrepareTurnSection("[Canonical Character States]", canonCharacterLines)
	out.CanonRelationshipText = makePrepareTurnSection("[Canonical Relationships]", canonRelationshipLines)
	out.CanonWorldText = makePrepareTurnSection("[Canonical World States]", canonWorldLines)
	subjectiveRelationshipActive := false
	for _, text := range []string{
		out.CharacterPrivateText,
		out.CharacterRelationshipText,
		out.PersonaText,
		out.CanonRelationshipText,
		out.KGText,
	} {
		if strings.TrimSpace(text) != "" {
			subjectiveRelationshipActive = true
			break
		}
	}
	out.Counts["subjective_relationship_lane_active"] = subjectiveRelationshipActive
	if subjectiveRelationshipActive {
		out.Counts["subjective_relationship_lane_reason"] = "request_or_current_scene_evidence_selected"
	} else {
		out.Counts["subjective_relationship_lane_reason"] = "no_request_or_current_scene_evidence_selected"
	}
	deliveryBudgetContext := map[string]any{
		"_memory_delivery_budget_mode": memoryDeliveryBudgetMode,
		"_memory_delivery_budgets":     memoryDeliveryBudgets,
	}
	if len(perspectiveContextArg) > 0 {
		deliveryBudgetContext["_core_objective_memory_max_items_present"] = boolFromAny(perspectiveContextArg[0]["_core_objective_memory_max_items_present"])
		deliveryBudgetContext["_core_objective_memory_max_items"] = intFromAny(perspectiveContextArg[0]["_core_objective_memory_max_items"], 0)
	}
	out.MemoryDeliveryPlan = buildPrepareTurnMemoryDeliveryPlan(&out, maxChars, deliveryBudgetContext)
	out.MemoryDeliveryLineage = finalizePrepareTurnMemoryDeliveryLineage(out.MemoryDeliveryLineage, out.MemoryDeliveryPlan)
	out.CharacterMemorySupport = finalizePrepareTurnCharacterMemorySupport(out.CharacterMemorySupport, out.MemoryDeliveryPlan)

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
	addPrepareTurnBlock(&out, "persona_recollection", "store.persona_memory_entries", out.PersonaText, len(personaEntries), maxChars)
	addPrepareTurnBlock(&out, "character_private_recollection", "store.protagonist_entity_memories", out.CharacterPrivateText, len(characterPrivateMemories), maxChars)
	addPrepareTurnBlock(&out, "continuity_correction", "store.status_current_values", out.ContinuityCorrectionText, intFromAny(mapFromAny(out.Counts["continuity_correction"])["selected_count"], 0), maxChars)

	parts := make([]string, 0, len(out.Blocks))
	for _, block := range out.Blocks {
		if strings.TrimSpace(block.Text) != "" {
			parts = append(parts, block.Text)
		}
	}
	// This assembled text remains a diagnostic/legacy fallback. The actual host
	// adapter uses MemoryDeliveryPlan.final_text whenever the plan is present,
	// so only the Go-owned category plan decides final transmitted quantity.
	out.Text = strings.Join(parts, "\n")
	if len([]rune(out.Text)) > maxChars {
		out.Text = truncateRunes(out.Text, maxChars)
		out.Truncated = true
		out.Trimmed = append(out.Trimmed, map[string]any{
			"label":  "overall",
			"reason": "diagnostic_fallback_max_injection_chars",
			"budget": maxChars,
		})
	}

	out.Counts["memory_bound"] = len(memoryLines)
	out.Counts["memory_count"] = len(memoryLines)
	out.Counts["top_k_memory_target"] = topK
	out.Counts["support_candidate_limit"] = recallLimit
	out.Counts["support_candidate_limit_source"] = "processing_safety_bound_independent_of_top_k_and_final_delivery"
	out.Counts["top_k_definition"] = "vector_memory_search_limit_only"
	out.Counts["recent_memory_bound"] = len(memorySelection.Recent)
	out.Counts["vector_memory_bound"] = len(memorySelection.VectorRelevant)
	out.Counts["relevant_memory_bound"] = len(memorySelection.Relevant)
	out.Counts["deep_memory_bound"] = len(memorySelection.Deep)
	out.Counts["memory_recall_lane_policy"] = memorySelection.Trace
	mergePrepareTurnMemoryLaneCounters(out.Counts, memorySelection, strings.TrimSpace(out.MemoryText) != "")
	mergePrepareTurnVectorArtifactCounters(out.Counts, artifactHydration, strings.TrimSpace(out.DirectEvidenceText) != "", len(directEvidenceLines), len(worldRuleLines))
	out.Counts["retrieval_methods"] = map[string]any{
		"exact_phrase": map[string]any{
			"status":          "ready",
			"candidate_count": intFromAny(memorySelection.Trace["exact_phrase_candidate_count"], 0),
			"selected_count":  intFromAny(memorySelection.Trace["exact_phrase_selected_count"], 0),
		},
		"lexical": map[string]any{
			"status":          "ready",
			"candidate_count": intFromAny(memorySelection.Trace["lexical_candidate_count"], 0),
			"selected_count":  intFromAny(memorySelection.Trace["lexical_selected_count"], 0),
		},
		"vector": prepareTurnVectorRetrievalMethodStatus(vectorShadow, len(memorySelection.VectorRelevant)),
		"relationship": map[string]any{
			"status":           "ready",
			"source_row_count": len(kgTriples) + len(charStates) + len(canonicalLayers),
			"selected_count":   len(kgLines) + len(charRelationshipLines) + len(canonRelationshipLines),
		},
		"unresolved_thread": map[string]any{
			"status":          "ready",
			"candidate_count": len(pendingThreads),
			"selected_count":  len(pendingLines),
		},
	}
	out.Counts["language_aware_injection"] = out.LanguageInjectionTrace
	if memoryTrace := mapFromAny(out.LanguageInjectionTrace["memory_language_trace"]); len(memoryTrace) > 0 {
		out.Counts["memory_summary_language_match"] = intFromAny(memoryTrace["memory_summary_language_match"], 0)
		out.Counts["memory_summary_language_mismatch"] = intFromAny(memoryTrace["memory_summary_language_mismatch"], 0)
		out.Counts["raw_evidence_attached_count"] = intFromAny(memoryTrace["raw_evidence_attached_count"], 0)
	}
	out.Counts["kg_bound"] = len(kgLines)
	out.Counts["kg_closed_or_not_yet_valid_dropped"] = kgClosedDropped
	out.Counts["kg_irrelevant_dropped"] = kgIrrelevantDropped
	out.Counts["kg_single_endpoint_only_dropped"] = kgSingleEndpointDropped
	out.Counts["storyline_irrelevant_dropped"] = storylineIrrelevantDropped
	out.Counts["world_rule_irrelevant_dropped"] = worldRuleIrrelevantDropped
	out.Counts["world_rule_persistent_selected"] = worldRulePersistentSelected
	out.Counts["character_state_irrelevant_dropped"] = characterIrrelevantDropped
	out.Counts["character_state_candidate_capped"] = 0
	out.Counts["character_state_count_cap"] = nil
	out.Counts["character_state_relevance_before_cap"] = true
	out.Counts["character_state_current_input_priority"] = true
	out.Counts["character_state_reviewed_identity_alias_priority"] = true
	out.Counts["character_relationship_irrelevant_dropped"] = characterRelationshipIrrelevantDropped
	out.Counts["typed_voice_projection_deferred_to_3_9_e"] = typedVoiceProjectionDeferred
	out.Counts["character_memory_delivery"] = out.CharacterMemorySupport
	out.Counts["pending_thread_irrelevant_dropped"] = pendingIrrelevantDropped
	out.Counts["pending_thread_pinned_active_selected"] = pendingPinnedActiveSelected
	out.Counts["pending_thread_suppressed_dropped"] = pendingSuppressedDropped
	out.Counts["episode_irrelevant_dropped"] = episodeIrrelevantDropped
	out.Counts["direct_evidence_bound"] = len(directEvidenceLines)

	out.Counts["fallback_bound"] = len(fallbackLines)
	out.Counts["fallback_count"] = len(fallbackLines)
	out.Counts["episode_bound"] = len(episodeLines)
	out.Counts["episode_delivered"] = strings.TrimSpace(out.EpisodeText) != ""
	out.Counts["chapter_delivered"] = strings.TrimSpace(out.ChapterText) != ""
	out.Counts["arc_delivered"] = strings.TrimSpace(out.ArcText) != ""
	out.Counts["saga_delivered"] = strings.TrimSpace(out.SagaText) != ""
	out.Counts["chapter_chars"] = len([]rune(strings.TrimSpace(out.ChapterText)))
	out.Counts["arc_chars"] = len([]rune(strings.TrimSpace(out.ArcText)))
	out.Counts["saga_chars"] = len([]rune(strings.TrimSpace(out.SagaText)))
	out.Counts["hierarchy_escalation"] = hierarchyEscalation.Trace
	out.Counts["persona_recollection_bound"] = len(personaEntries)
	out.Counts["persona_recollection_support_only"] = len(personaEntries) > 0
	out.Counts["character_private_recollection_bound"] = len(characterPrivateMemories)
	out.Counts["character_private_recollection_private_lane"] = len(characterPrivateMemories) > 0
	out.Counts["subjective_recollection_count_cap"] = nil
	out.Counts["subjective_recollection_final_boundary"] = "semantic_scope_relevance_privacy_then_subjective_relationship_char_budget"
	out.Counts["scoped_verbatim_support_count"] = out.ScopedVerbatimSupport.Count
	out.Counts["verbatim_support_active"] = out.ScopedVerbatimSupport.Active
	out.Counts["canonical_state_layers_filtered_count"] = canonFiltered
	out.Counts["canonical_state_layers_irrelevant_count"] = canonIrrelevant
	out.Counts["canonical_relationship_irrelevant_dropped"] = canonRelationshipIrrelevant
	out.Counts["canonical_state_relationship_layers_count"] = canonTypeCounts["relationship_state"]
	out.Counts["canonical_state_world_layers_count"] = canonTypeCounts["world_state"]
	out.Counts["canonical_state_scene_layers_count"] = canonTypeCounts["scene_state"]
	out.Counts["canonical_state_entity_layers_count"] = canonTypeCounts["entity_state"]
	out.Counts["canonical_character_roster_only_dropped"] = canonCharacterRosterOnlyDropped
	out.Counts["storyline_collapsed_count"] = maxInt(len(storylines)-len(storylinesForInjection), 0)
	out.Counts["world_rule_collapsed_count"] = maxInt(len(worldRules)-len(worldRulesForInjection), 0)
	out.Counts["block_count"] = len(out.Blocks)
	out.Counts["total_chars"] = len([]rune(out.Text))
	fallbackReason := "raw_chat_fallback_removed_use_input_context"
	if len(actualMemoryLines)+len(protectedMemoryLines) > 0 {
		fallbackReason = "memory_sufficient"
	}
	out.BudgetDecisions = map[string]any{
		"policy_version":                              "rmg07.prepare_turn.bundle.v1",
		"max_injection_chars":                         maxChars,
		"final_budget_owner":                          "go_memory_delivery_plan",
		"memory_delivery_plan":                        out.MemoryDeliveryPlan,
		"fallback_chat_log_included":                  strings.TrimSpace(out.FallbackText) != "",
		"fallback_reason":                             fallbackReason,
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
		summary = memorySummaryFromParsed(parsed)
		if summary == "" {
			summary = publicMemoryProjectionNarrativeSummary(parsed)
		}
		if summary == "" {
			return ""
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
	return compactPrepareTurnLine(summary, 0)
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
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if limit <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	cut := limit
	for i := limit - 1; i > 0; i-- {
		if runes[i] == ' ' {
			cut = i
			break
		}
	}
	text = strings.TrimSpace(string(runes[:cut]))
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

func prepareTurnCanonicalCharacterStateHasDetails(value any) bool {
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			if prepareTurnCanonicalCharacterStateHasDetails(item) {
				return true
			}
		}
		return false
	case map[string]any:
		for key, item := range typed {
			switch strings.ToLower(strings.TrimSpace(key)) {
			case "name", "character_name", "display_name", "id", "key", "aliases":
				continue
			}
			if _, ok := prunePrepareTurnEmptySurface(item); ok {
				return true
			}
		}
		return false
	default:
		// A string or scalar under "characters" is only a roster entry, not a
		// current character state.
		return false
	}
}

func prepareTurnCanonicalKnownCharacterNames(layers []store.CanonicalStateLayer) []string {
	out := []string{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value != "" && !prepareTurnRelationshipNameInList(value, out) {
			out = append(out, value)
		}
	}
	for _, layer := range layers {
		payload := parseJSONMap(layer.Content)
		switch strings.TrimSpace(layer.LayerType) {
		case "relationship_state":
			left, right := prepareTurnStoredRelationshipActors(payload)
			add(left)
			add(right)
		case "entity_state":
			characters, ok := payload["characters"]
			if !ok {
				continue
			}
			switch typed := characters.(type) {
			case []any:
				for _, item := range typed {
					if name, ok := item.(string); ok {
						add(name)
						continue
					}
					entry := mapFromAny(item)
					add(extractionFirstNonEmpty(stringFromMap(entry, "name"), stringFromMap(entry, "character_name"), stringFromMap(entry, "display_name")))
				}
			case map[string]any:
				if name := extractionFirstNonEmpty(stringFromMap(typed, "name"), stringFromMap(typed, "character_name"), stringFromMap(typed, "display_name")); name != "" {
					add(name)
					continue
				}
				for name := range typed {
					add(name)
				}
			}
		}
	}
	return out
}

func prepareTurnCanonicalCharactersForScene(value any, sceneEntities []string) (any, bool) {
	if len(sceneEntities) == 0 {
		return nil, false
	}
	nameFrom := func(payload map[string]any) string {
		return extractionFirstNonEmpty(
			stringFromMap(payload, "name"),
			stringFromMap(payload, "character_name"),
			stringFromMap(payload, "display_name"),
		)
	}
	switch typed := value.(type) {
	case []any:
		kept := make([]any, 0, len(typed))
		for _, item := range typed {
			payload := mapFromAny(item)
			name := nameFrom(payload)
			if name == "" || !prepareTurnRelationshipNameInList(name, sceneEntities) || !prepareTurnCanonicalCharacterStateHasDetails(payload) {
				continue
			}
			kept = append(kept, item)
		}
		return kept, len(kept) > 0
	case map[string]any:
		if name := nameFrom(typed); name != "" {
			if prepareTurnRelationshipNameInList(name, sceneEntities) && prepareTurnCanonicalCharacterStateHasDetails(typed) {
				return typed, true
			}
			return nil, false
		}
		kept := map[string]any{}
		for name, detail := range typed {
			if !prepareTurnRelationshipNameInList(name, sceneEntities) || !prepareTurnCanonicalCharacterStateHasDetails(detail) {
				continue
			}
			kept[name] = detail
		}
		return kept, len(kept) > 0
	default:
		return nil, false
	}
}

func prepareTurnRelevantRelationshipSurface(raw, owner, rawUserInput string, currentSceneEntities, knownEntities []string) (string, int) {
	value := parseSurfacePayload(raw)
	filtered, dropped, ok := prepareTurnFilterRelationshipSurface(value, owner, rawUserInput, currentSceneEntities, knownEntities)
	if !ok {
		return "", dropped
	}
	return prepareTurnSurfaceText(filtered), dropped
}

func prepareTurnFilterRelationshipSurface(value any, owner, rawUserInput string, currentSceneEntities, knownEntities []string) (any, int, bool) {
	switch typed := value.(type) {
	case map[string]any:
		kept := map[string]any{}
		dropped := 0
		for target, detail := range typed {
			detailText := prepareTurnSurfaceText(detail)
			if prepareTurnRelationshipNameInList(target, knownEntities) ||
				prepareTurnRecallContainsAnchor(rawUserInput, target) ||
				prepareTurnRelationshipNameInList(target, currentSceneEntities) {
				if prepareTurnStructuredRelationshipRelevant(owner, target, detailText, rawUserInput, currentSceneEntities, knownEntities) {
					kept[target] = detail
				} else {
					dropped++
				}
				continue
			}
			if prepareTurnFreeRelationshipRelevant(detailText, owner, rawUserInput, currentSceneEntities, knownEntities) {
				kept[target] = detail
			} else {
				dropped++
			}
		}
		return kept, dropped, len(kept) > 0
	case []any:
		kept := make([]any, 0, len(typed))
		dropped := 0
		for _, detail := range typed {
			entryText := strings.TrimSpace(prepareTurnSurfaceText(detail))
			payload := mapFromAny(detail)
			left, right := prepareTurnStoredRelationshipActors(payload)
			relevant := false
			if left != "" || right != "" {
				if left == "" {
					left = owner
				}
				relevant = prepareTurnStructuredRelationshipRelevant(left, right, entryText, rawUserInput, currentSceneEntities, knownEntities)
			} else {
				relevant = prepareTurnFreeRelationshipRelevant(entryText, owner, rawUserInput, currentSceneEntities, knownEntities)
			}
			if relevant {
				kept = append(kept, detail)
			} else {
				dropped++
			}
		}
		return kept, dropped, len(kept) > 0
	case string:
		text := strings.TrimSpace(typed)
		if text == "" {
			return nil, 0, false
		}
		if prepareTurnFreeRelationshipRelevant(text, owner, rawUserInput, currentSceneEntities, knownEntities) {
			return text, 0, true
		}
		return nil, 1, false
	case nil:
		return nil, 0, false
	default:
		text := prepareTurnSurfaceText(typed)
		if text != "" && prepareTurnFreeRelationshipRelevant(text, owner, rawUserInput, currentSceneEntities, knownEntities) {
			return typed, 0, true
		}
		return nil, boolToInt(text != ""), false
	}
}

func prepareTurnRelevantCanonicalRelationshipSurface(raw, rawUserInput string, currentSceneEntities, knownEntities []string) (string, int) {
	value := parseSurfacePayload(raw)
	payload := mapFromAny(value)
	if len(payload) > 0 {
		left, right := prepareTurnStoredRelationshipActors(payload)
		if left != "" || right != "" {
			text := prepareTurnSurfaceText(payload)
			if prepareTurnStructuredRelationshipRelevant(left, right, text, rawUserInput, currentSceneEntities, knownEntities) {
				return text, 0
			}
			return "", 1
		}
	}
	text := prepareTurnSurfaceText(value)
	if prepareTurnFreeRelationshipRelevant(text, "", rawUserInput, currentSceneEntities, knownEntities) {
		return text, 0
	}
	return "", boolToInt(strings.TrimSpace(text) != "")
}

func prepareTurnStoredRelationshipActors(payload map[string]any) (string, string) {
	pairValues := stringsFromAny(payload["pair"])
	left, right := "", ""
	if len(pairValues) >= 2 {
		left, right = pairValues[0], pairValues[1]
	} else {
		left, right = relationshipChangeActors(payload)
	}
	if left == "" {
		left = extractionFirstNonEmpty(
			stringFromMap(payload, "owner_name"),
			stringFromMap(payload, "source_name"),
			stringFromMap(payload, "subject"),
		)
	}
	if right == "" {
		right = extractionFirstNonEmpty(
			stringFromMap(payload, "target_name"),
			stringFromMap(payload, "object"),
		)
	}
	if left == "" || right == "" {
		for _, pair := range stringsFromAny(payload["pair"]) {
			parts := relationshipPairParts(pair)
			if len(parts) == 0 && strings.TrimSpace(pair) != "" {
				parts = []string{strings.TrimSpace(pair)}
			}
			for _, part := range parts {
				if left == "" {
					left = part
				} else if right == "" && normalizePrepareTurnEntityNeedle(part) != normalizePrepareTurnEntityNeedle(left) {
					right = part
				}
			}
		}
	}
	return strings.TrimSpace(left), strings.TrimSpace(right)
}

func prepareTurnStructuredRelationshipRelevant(owner, target, detailText, rawUserInput string, currentSceneEntities, knownEntities []string) bool {
	owner = strings.TrimSpace(owner)
	target = strings.TrimSpace(target)
	ownerCurrent := owner != "" && (prepareTurnRelationshipDirectMention(rawUserInput, owner, knownEntities) ||
		prepareTurnRelationshipNameInList(owner, currentSceneEntities))
	targetCurrent := target != "" && (prepareTurnRelationshipDirectMention(rawUserInput, target, knownEntities) ||
		prepareTurnRelationshipNameInList(target, currentSceneEntities))
	if ownerCurrent && targetCurrent {
		return !prepareTurnRelationshipContainsThirdEntity(detailText, owner, target, knownEntities)
	}
	return false
}

func prepareTurnFreeRelationshipRelevant(text, owner, rawUserInput string, currentSceneEntities, knownEntities []string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	if len(knownEntities) == 0 {
		// Some verified legacy relationship_state rows predate structured actor
		// fields. Preserve them only when the current request shares at least two
		// substantive terms with the relationship itself; a single character
		// name is never enough.
		return prepareTurnRecallOverlapCount(rawUserInput, text) >= 2 &&
			prepareTurnSupportRecallEligible(rawUserInput, text)
	}
	mentioned := []string{}
	for _, name := range knownEntities {
		if name != "" && prepareTurnRecallContainsAnchor(text, name) && !prepareTurnRelationshipNameInList(name, mentioned) {
			mentioned = append(mentioned, name)
		}
	}
	if owner != "" && !prepareTurnRelationshipNameInList(owner, mentioned) {
		mentioned = append(mentioned, owner)
	}
	if len(mentioned) < 2 {
		return false
	}
	directCounterpart := false
	for _, name := range mentioned {
		if owner != "" && normalizePrepareTurnEntityNeedle(name) == normalizePrepareTurnEntityNeedle(owner) {
			continue
		}
		if !prepareTurnRelationshipDirectMention(rawUserInput, name, knownEntities) && !prepareTurnRelationshipNameInList(name, currentSceneEntities) {
			return false
		}
		if prepareTurnRelationshipDirectMention(rawUserInput, name, knownEntities) {
			directCounterpart = true
		}
	}
	return directCounterpart
}

func prepareTurnRelationshipDirectMention(rawUserInput, name string, knownEntities []string) bool {
	return prepareTurnDirectEntityMentionRank(rawUserInput, name, nil) > 0
}

func prepareTurnRelationshipContainsThirdEntity(text, owner, target string, knownEntities []string) bool {
	for _, name := range knownEntities {
		if name == "" || !prepareTurnRecallContainsAnchor(text, name) {
			continue
		}
		normalized := normalizePrepareTurnEntityNeedle(name)
		if normalized != normalizePrepareTurnEntityNeedle(owner) && normalized != normalizePrepareTurnEntityNeedle(target) {
			return true
		}
	}
	return false
}

func prepareTurnRelationshipNameInList(name string, items []string) bool {
	normalized := normalizePrepareTurnEntityNeedle(name)
	if normalized == "" {
		return false
	}
	for _, item := range items {
		if normalizePrepareTurnEntityNeedle(item) == normalized {
			return true
		}
	}
	return false
}

func compactPrepareTurnJSON(value any) string {
	value, ok := prunePrepareTurnEmptySurface(value)
	if !ok {
		return ""
	}
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return compactPrepareTurnLine(string(data), 0)
}

func prunePrepareTurnEmptySurface(value any) (any, bool) {
	switch v := value.(type) {
	case nil:
		return nil, false
	case string:
		v = strings.TrimSpace(v)
		return v, v != ""
	case map[string]any:
		out := map[string]any{}
		for key, item := range v {
			if cleaned, ok := prunePrepareTurnEmptySurface(item); ok {
				out[key] = cleaned
			}
		}
		return out, len(out) > 0
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			if cleaned, ok := prunePrepareTurnEmptySurface(item); ok {
				out = append(out, cleaned)
			}
		}
		return out, len(out) > 0
	default:
		return value, true
	}
}

func episodeDenseAnchorPreview(es store.EpisodeSummary, summary string, limit int) string {
	parts := []string{}
	componentLimit := 120
	if limit <= 0 {
		componentLimit = 0
	}
	if key := compactEpisodeJSONPreview(es.KeyEvents, componentLimit); key != "" {
		summaryKey := collapseTextKey(summary)
		keyText := collapseTextKey(key)
		if summaryKey == "" || keyText == "" || (summaryKey != keyText && !strings.Contains(summaryKey, keyText)) {
			parts = append(parts, "key_event="+key)
		}
	}
	if rel := compactEpisodeJSONPreview(es.RelationshipChangesJSON, componentLimit); rel != "" {
		parts = append(parts, "rel="+rel)
	}
	if loop := compactEpisodeJSONPreview(es.OpenLoopsJSON, componentLimit); loop != "" {
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

func recentPrepareTurnRawTurn(chatLogs []store.ChatLog) string {
	if len(chatLogs) == 0 {
		return ""
	}
	// This diagnostic surface mirrors only the immediately previous logical
	// turn. Older chat text is recalled through admitted memory/evidence, never
	// replayed as raw user instructions in a later request.
	selected := selectRecentChatLogsByTurn(chatLogs, 1)
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

func buildInputContextText(chatLogs []store.ChatLog, maxChars int) (string, bool) {
	maxChars = prepareTurnTextBudget(maxChars)
	parts := []string{}
	truncated := false
	appendSection := func(header string, lines []string) {
		cleaned := make([]string, 0, len(lines))
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" {
				cleaned = append(cleaned, line)
			}
		}
		if len(cleaned) == 0 {
			return
		}
		sectionLines := []string{header}
		for _, line := range cleaned {
			candidate := strings.Join(append(append([]string{}, parts...), append(sectionLines, line)...), "\n")
			if len([]rune(candidate)) <= maxChars {
				sectionLines = append(sectionLines, line)
				continue
			}
			truncated = true
			if len(sectionLines) == 1 {
				prefix := strings.Join(append(append([]string{}, parts...), header), "\n")
				available := maxChars - len([]rune(prefix)) - 1
				if available > 0 {
					sectionLines = append(sectionLines, truncateRunes(line, available))
				}
			}
			break
		}
		if len(sectionLines) == 1 {
			return
		}
		parts = append(parts, sectionLines...)
	}

	// Input Context has one owner and one purpose: the immediately preceding
	// completed logical user/assistant turn. States, relations, evidence,
	// hierarchy and private memory are delivered through their dedicated Go
	// classes and must never bypass those selectors here.
	if len(chatLogs) > 0 {
		var logParts []string
		recentLogs := selectRecentChatLogsByTurn(chatLogs, 1)
		if len(recentLogs) > 2 {
			recentLogs = recentLogs[len(recentLogs)-2:]
		}
		perMessageCap := maxInt((maxChars-len([]rune("[Recent Chat]"))-32)/maxInt(len(recentLogs), 1), 80)
		for _, cl := range recentLogs {
			content := strings.TrimSpace(cl.Content)
			if content == "" {
				continue
			}
			content = strings.Join(strings.Fields(content), " ")
			if len([]rune(content)) > perMessageCap {
				content = truncateRunes(content, perMessageCap)
				truncated = true
			}
			logParts = append(logParts, fmt.Sprintf("- [%s] %s", cl.Role, content))
		}
		appendSection("[Recent Chat]", logParts)
	}

	text := strings.Join(parts, "\n")
	return text, truncated
}
