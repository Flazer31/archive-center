package httpapi

import (
	"fmt"
	"sort"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

const (
	sourceToPayloadLineageContract          = "source_to_payload_lineage.v1"
	payloadApplicationObservationContract   = "payload_application_observation.v1"
	sourceToFinalLineageObservationContract = "source_to_final_lineage_observation.v1"
	sourceToFinalLineageContract            = "source_to_final_lineage.v1"
)

func prepareTurnMemoryLineageSourceRef(sessionID string, rowID any) string {
	sid := strings.TrimSpace(sessionID)
	row := strings.TrimSpace(fmt.Sprint(rowID))
	if sid == "" || row == "" || row == "<nil>" || row == "0" {
		return ""
	}
	return "memory:" + sid + ":" + row
}

func boundedPrepareTurnMemoryDeliveryLineage(sessionID string, raw map[string]any, chatLogs []store.ChatLog) map[string]any {
	assistantByTurn := map[int][]store.ChatLog{}
	for _, item := range chatLogs {
		role := strings.ToLower(strings.TrimSpace(item.Role))
		if item.ChatSessionID != sessionID || (role != "assistant" && role != "char") {
			continue
		}
		assistantByTurn[item.TurnIndex] = append(assistantByTurn[item.TurnIndex], item)
	}

	items := make([]any, 0)
	for _, rawItem := range outputFidelityLineageSlice(raw["items"]) {
		item := mapFromAny(rawItem)
		if !boolFromAny(item["delivered"]) {
			continue
		}
		ref := prepareTurnMemoryLineageSourceRef(sessionID, item["source_row_id"])
		if ref == "" {
			continue
		}
		turnIndex := intFromAny(item["turn_index"], 0)
		sourceSpan := map[string]any{
			"status":           "unobserved",
			"reason_code":      "source_span_not_observed",
			"source_kind":      "unobserved",
			"source_lifecycle": "unobserved",
			"semantic_outcome": "unobserved",
			"link_precision":   "turn_only",
			"turn_index":       turnIndex,
		}
		bounded := make(map[string]any, len(item)+6)
		for key, value := range item {
			if key == "final_text" {
				continue
			}
			bounded[key] = value
		}
		bounded["source_ref"] = ref
		bounded["source_table"] = "memories"
		bounded["source_row_id"] = item["source_row_id"]
		bounded["source_turn_index"] = turnIndex
		bounded["selection_lane"] = extractionStringFromAny(item["selection_lane"])
		bounded["delivery_status"] = extractionStringFromAny(item["delivery_status"])
		bounded["delivered"] = true
		bounded["protected_guard"] = boolFromAny(item["protected_guard"])
		bounded["precision"] = "turn_only"
		bounded["link_precision"] = "turn_only"
		bounded["source_span"] = sourceSpan
		if rows := assistantByTurn[turnIndex]; len(rows) == 1 {
			sourceSpan["status"] = "partially_observed"
			sourceSpan["reason_code"] = "turn_only_source_span_observed"
			sourceSpan["source_kind"] = "assistant_output"
			sourceSpan["content_hash"] = prepareTurnTextHash(rows[0].Content)
			sourceSpan["hash_algorithm"] = "sha256"
		} else if len(rows) > 1 {
			sourceSpan["status"] = "ambiguous"
			sourceSpan["reason_code"] = "multiple_assistant_spans_at_turn"
			sourceSpan["source_kind"] = "assistant_output"
		}
		items = append(items, bounded)
	}

	out := make(map[string]any, len(raw)+4)
	for key, value := range raw {
		if key == "items" {
			continue
		}
		out[key] = value
	}
	out["contract_version"] = "memory_delivery_lineage.v1"
	out["status"] = extractionStringFromAny(raw["status"])
	out["precision"] = "turn_only"
	out["source_text_exposed"] = false
	out["final_delivered_count"] = len(items)
	out["items"] = items
	return out
}

func outputFidelityLineageSlice(value any) []any {
	if items, ok := value.([]any); ok {
		return items
	}
	if items, ok := value.([]map[string]any); ok {
		out := make([]any, 0, len(items))
		for _, item := range items {
			out = append(out, item)
		}
		return out
	}
	return []any{}
}

type memoryInjectionBaselineCandidate struct {
	Surface     string
	SourceRef   string
	Fingerprint string
}

func memoryInjectionBaselineFingerprint(text string) string {
	normalized := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(text)), " "))
	if normalized == "" {
		return ""
	}
	return prepareTurnTextHash(normalized)
}

func memoryInjectionBaselineSurface(
	name string,
	candidateCount, selectedCount, renderedCount, payloadChars int,
	provenance []string,
	orderingPositions []int,
) map[string]any {
	if selectedCount < 0 {
		selectedCount = 0
	}
	if renderedCount < 0 {
		renderedCount = 0
	}
	if payloadChars < 0 {
		payloadChars = 0
	}
	excluded := candidateCount - selectedCount
	if excluded < 0 {
		excluded = 0
	}
	renderExcluded := selectedCount - renderedCount
	if renderExcluded < 0 {
		renderExcluded = 0
	}
	candidateReason := "none"
	switch {
	case candidateCount == 0:
		candidateReason = "no_candidates"
	case selectedCount == 0:
		candidateReason = "not_selected_by_current_policy"
	case excluded > 0:
		candidateReason = "current_retrieval_or_surface_policy_exclusion"
	}
	renderReason := "none"
	switch {
	case selectedCount == 0:
		renderReason = "no_selected_items"
	case renderedCount == 0:
		renderReason = "not_rendered_by_current_delivery_policy"
	case renderExcluded > 0:
		renderReason = "current_delivery_budget_or_exact_line_dedupe"
	}
	return map[string]any{
		"surface":                       name,
		"candidate_count":               candidateCount,
		"selected_count":                selectedCount,
		"rendered_count":                renderedCount,
		"payload_character_count":       payloadChars,
		"payload_character_count_state": "matched_rendered_source_item_chars_non_additive_across_surfaces",
		"token_estimate":                (payloadChars + 3) / 4,
		"token_count_state":             "estimated_chars_div_4",
		"excluded_count":                excluded,
		"exclusion_reason":              candidateReason,
		"render_excluded_count":         renderExcluded,
		"render_exclusion_reason":       renderReason,
		"count_units": map[string]any{
			"candidate": "store_or_prepare_record",
			"selected":  "surface_source_item",
			"rendered":  "matched_final_lane_item",
		},
		"provenance":         provenance,
		"ordering_positions": orderingPositions,
		"stages": map[string]any{
			"candidate":        map[string]any{"status": "observed", "count": candidateCount},
			"selected":         map[string]any{"status": "observed", "count": selectedCount},
			"rendered":         map[string]any{"status": "observed", "count": renderedCount, "chars": payloadChars},
			"payload_applied":  map[string]any{"status": "planned_unobserved"},
			"displayed_effect": map[string]any{"status": "unobserved"},
		},
	}
}

type memoryInjectionBaselineRenderMatch struct {
	SelectedCount int
	RenderedCount int
	PayloadChars  int
	Positions     []int
}

func memoryInjectionBaselineFinalLaneItems(deliveryPlan map[string]any) []string {
	items := []string{}
	for _, raw := range outputFidelityLineageSlice(deliveryPlan["classes"]) {
		items = append(items, prepareTurnDeliveryItems(extractionStringFromAny(mapFromAny(raw)["text"]))...)
	}
	return items
}

func memoryInjectionBaselineMatch(finalItems []string, sourceTexts ...string) memoryInjectionBaselineRenderMatch {
	sourceItems := prepareTurnDeliveryItems(sourceTexts...)
	positions := map[string][]int{}
	for index, item := range finalItems {
		positions[item] = append(positions[item], index)
	}
	used := map[string]int{}
	match := memoryInjectionBaselineRenderMatch{SelectedCount: len(sourceItems), Positions: []int{}}
	for _, item := range sourceItems {
		itemPositions := positions[item]
		usedIndex := used[item]
		if usedIndex >= len(itemPositions) {
			continue
		}
		used[item] = usedIndex + 1
		match.RenderedCount++
		match.PayloadChars += len([]rune(item))
		match.Positions = append(match.Positions, itemPositions[usedIndex])
	}
	return match
}

func buildMemoryInjectionBaseline41(
	sessionID, requestCorrelationID, rawUserInput string,
	memories []store.Memory,
	evidence []store.DirectEvidence,
	kgTriples []store.KGTriple,
	worldRules []store.WorldRule,
	charStates []store.CharacterState,
	activeStates []store.ActiveState,
	canonicalLayers []store.CanonicalStateLayer,
	personaEntries []store.PersonaMemoryEntry,
	characterPrivateMemories []store.ProtagonistEntityMemory,
	storylines []store.Storyline,
	pendingThreads []store.PendingThread,
	episodeSums []store.EpisodeSummary,
	chatLogs []store.ChatLog,
	reversibleStateText string,
	assembly prepareTurnInjectionAssembly,
	memoryLineage map[string]any,
) map[string]any {
	relationshipCandidates := 0
	for _, item := range charStates {
		if strings.TrimSpace(item.RelationshipsJSON) != "" {
			relationshipCandidates++
		}
	}
	stateCandidates := len(worldRules) + len(charStates) + len(activeStates) + len(canonicalLayers)
	personaCandidates := len(personaEntries) + len(characterPrivateMemories)
	finalLaneItems := memoryInjectionBaselineFinalLaneItems(assembly.MemoryDeliveryPlan)
	memoryMatch := memoryInjectionBaselineMatch(finalLaneItems, assembly.ActualMemoryText, assembly.ProtectedMemoryText)
	directEvidenceMatch := memoryInjectionBaselineMatch(finalLaneItems, assembly.LatestDirectEvidenceText, assembly.DirectEvidenceText)
	kgMatch := memoryInjectionBaselineMatch(finalLaneItems, assembly.KGText)
	stateMatch := memoryInjectionBaselineMatch(finalLaneItems, assembly.WorldRulesText, assembly.CharacterObjectiveText, assembly.CanonCharacterText, assembly.CanonWorldText, assembly.ContinuityCorrectionText)
	reversibleStateItems := prepareTurnDeliveryItems(reversibleStateText)
	stateMatch.SelectedCount += len(reversibleStateItems)
	for index, item := range reversibleStateItems {
		stateMatch.RenderedCount++
		stateMatch.PayloadChars += len([]rune(item))
		stateMatch.Positions = append(stateMatch.Positions, len(finalLaneItems)+index)
	}
	personaMatch := memoryInjectionBaselineMatch(finalLaneItems, assembly.PersonaText, assembly.CharacterPrivateText)
	relationshipMatch := memoryInjectionBaselineMatch(finalLaneItems, assembly.CharacterRelationshipText, assembly.CanonRelationshipText)
	storylineMatch := memoryInjectionBaselineMatch(finalLaneItems, assembly.StorylineText)
	pendingThreadMatch := memoryInjectionBaselineMatch(finalLaneItems, assembly.PendingThreadText)
	hierarchyMatch := memoryInjectionBaselineMatch(finalLaneItems, assembly.EpisodeText, assembly.ChapterText, assembly.ArcText, assembly.SagaText, assembly.CanonEventText)

	surfaces := []any{
		memoryInjectionBaselineSurface("memory", len(memories), memoryMatch.SelectedCount, memoryMatch.RenderedCount, memoryMatch.PayloadChars, []string{"store.memories", "memory_delivery_lineage.v1"}, memoryMatch.Positions),
		memoryInjectionBaselineSurface("direct_evidence", len(evidence), directEvidenceMatch.SelectedCount, directEvidenceMatch.RenderedCount, directEvidenceMatch.PayloadChars, []string{"store.direct_evidence_records"}, directEvidenceMatch.Positions),
		memoryInjectionBaselineSurface("kg", len(kgTriples), kgMatch.SelectedCount, kgMatch.RenderedCount, kgMatch.PayloadChars, []string{"store.kg_triples"}, kgMatch.Positions),
		memoryInjectionBaselineSurface("state", stateCandidates, stateMatch.SelectedCount, stateMatch.RenderedCount, stateMatch.PayloadChars, []string{"store.world_rules", "store.character_states", "store.active_states", "store.canonical_state_layers", "store.status_current_values"}, stateMatch.Positions),
		memoryInjectionBaselineSurface("persona", personaCandidates, personaMatch.SelectedCount, personaMatch.RenderedCount, personaMatch.PayloadChars, []string{"store.persona_memory_entries", "store.protagonist_entity_memories"}, personaMatch.Positions),
		memoryInjectionBaselineSurface("relationship", relationshipCandidates, relationshipMatch.SelectedCount, relationshipMatch.RenderedCount, relationshipMatch.PayloadChars, []string{"store.character_states.relationships_json", "store.canonical_state_layers.relationships"}, relationshipMatch.Positions),
		memoryInjectionBaselineSurface("storyline", len(storylines), storylineMatch.SelectedCount, storylineMatch.RenderedCount, storylineMatch.PayloadChars, []string{"store.storylines"}, storylineMatch.Positions),
		memoryInjectionBaselineSurface("pending_thread", len(pendingThreads), pendingThreadMatch.SelectedCount, pendingThreadMatch.RenderedCount, pendingThreadMatch.PayloadChars, []string{"store.pending_threads"}, pendingThreadMatch.Positions),
		memoryInjectionBaselineSurface("hierarchy_summary", len(episodeSums), hierarchyMatch.SelectedCount, hierarchyMatch.RenderedCount, hierarchyMatch.PayloadChars, []string{"store.episode_summaries", "store.chapter_summaries", "store.arc_summaries", "store.saga_digests"}, hierarchyMatch.Positions),
	}

	candidates := make([]memoryInjectionBaselineCandidate, 0)
	appendCandidate := func(surface, ref, text string) {
		if fingerprint := memoryInjectionBaselineFingerprint(text); fingerprint != "" {
			candidates = append(candidates, memoryInjectionBaselineCandidate{Surface: surface, SourceRef: ref, Fingerprint: fingerprint})
		}
	}
	for _, item := range memories {
		appendCandidate("memory", prepareTurnMemoryLineageSourceRef(sessionID, item.ID), prepareTurnMemorySummary(item))
	}
	for _, item := range evidence {
		appendCandidate("direct_evidence", fmt.Sprintf("direct_evidence:%s:%d", sessionID, item.ID), item.EvidenceText)
	}
	for _, item := range kgTriples {
		appendCandidate("kg", fmt.Sprintf("kg:%s:%d", sessionID, item.ID), strings.Join([]string{item.Subject, item.Predicate, item.Object}, " "))
	}
	for _, item := range worldRules {
		appendCandidate("state", fmt.Sprintf("world_rule:%s:%d", sessionID, item.ID), strings.Join([]string{item.Key, item.ValueJSON}, " "))
	}
	for _, item := range charStates {
		appendCandidate("state", fmt.Sprintf("character_state:%s:%d", sessionID, item.ID), strings.Join([]string{item.CharacterName, item.StatusJSON, item.AppearanceJSON, item.PersonalityJSON}, " "))
		appendCandidate("relationship", fmt.Sprintf("character_relationship:%s:%d", sessionID, item.ID), item.RelationshipsJSON)
	}
	for _, item := range activeStates {
		appendCandidate("state", fmt.Sprintf("active_state:%s:%d", sessionID, item.ID), item.Content)
	}
	for _, item := range canonicalLayers {
		appendCandidate("state", fmt.Sprintf("canonical_state:%s:%d", sessionID, item.ID), item.Content)
	}
	for _, item := range personaEntries {
		appendCandidate("persona", fmt.Sprintf("persona_memory:%s:%d", sessionID, item.ID), item.MemoryText)
	}
	for _, item := range characterPrivateMemories {
		appendCandidate("persona", fmt.Sprintf("protagonist_memory:%s:%d", sessionID, item.ID), item.MemoryText)
	}
	for _, item := range storylines {
		appendCandidate("storyline", fmt.Sprintf("storyline:%s:%d", sessionID, item.ID), strings.Join([]string{item.Name, item.CurrentContext}, " "))
	}
	for _, item := range pendingThreads {
		appendCandidate("pending_thread", fmt.Sprintf("pending_thread:%s:%d", sessionID, item.ID), strings.Join([]string{item.Title, item.Description}, " "))
	}
	for _, item := range episodeSums {
		appendCandidate("hierarchy_summary", fmt.Sprintf("episode_summary:%s:%d", sessionID, item.ID), item.SummaryText)
	}

	byFingerprint := map[string][]memoryInjectionBaselineCandidate{}
	for _, candidate := range candidates {
		byFingerprint[candidate.Fingerprint] = append(byFingerprint[candidate.Fingerprint], candidate)
	}
	semanticDuplicates := make([]any, 0)
	for fingerprint, group := range byFingerprint {
		surfaceSet := map[string]bool{}
		refs := make([]string, 0, len(group))
		for _, candidate := range group {
			surfaceSet[candidate.Surface] = true
			refs = append(refs, candidate.SourceRef)
		}
		if len(surfaceSet) < 2 {
			continue
		}
		surfaceNames := make([]string, 0, len(surfaceSet))
		for surface := range surfaceSet {
			surfaceNames = append(surfaceNames, surface)
		}
		sort.Strings(surfaceNames)
		sort.Strings(refs)
		semanticDuplicates = append(semanticDuplicates, map[string]any{"fingerprint": fingerprint, "surfaces": surfaceNames, "source_refs": refs})
	}
	sort.Slice(semanticDuplicates, func(i, j int) bool {
		return extractionStringFromAny(mapFromAny(semanticDuplicates[i])["fingerprint"]) < extractionStringFromAny(mapFromAny(semanticDuplicates[j])["fingerprint"])
	})

	sameRowLanes := make([]any, 0)
	rowLanes := map[string][]string{}
	for _, raw := range outputFidelityLineageSlice(memoryLineage["items"]) {
		item := mapFromAny(raw)
		ref := extractionStringFromAny(item["source_ref"])
		if ref == "" {
			ref = prepareTurnMemoryLineageSourceRef(sessionID, item["source_row_id"])
		}
		if ref != "" {
			rowLanes[ref] = append(rowLanes[ref], extractionStringFromAny(item["selection_lane"]))
		}
	}
	for ref, lanes := range rowLanes {
		if len(lanes) > 1 {
			sort.Strings(lanes)
			sameRowLanes = append(sameRowLanes, map[string]any{"source_ref": ref, "lanes": lanes, "occurrence_count": len(lanes)})
		}
	}
	sort.Slice(sameRowLanes, func(i, j int) bool {
		return extractionStringFromAny(mapFromAny(sameRowLanes[i])["source_ref"]) < extractionStringFromAny(mapFromAny(sameRowLanes[j])["source_ref"])
	})

	inputFingerprint := memoryInjectionBaselineFingerprint(rawUserInput)
	recentFingerprints := map[string][]string{}
	for _, item := range chatLogs {
		if fingerprint := memoryInjectionBaselineFingerprint(item.Content); fingerprint != "" {
			recentFingerprints[fingerprint] = append(recentFingerprints[fingerprint], fmt.Sprintf("chat_log:%s:%d", sessionID, item.ID))
		}
	}
	contextDuplicates := make([]any, 0)
	for _, candidate := range candidates {
		if candidate.Surface != "memory" {
			continue
		}
		if inputFingerprint != "" && candidate.Fingerprint == inputFingerprint {
			contextDuplicates = append(contextDuplicates, map[string]any{"source_ref": candidate.SourceRef, "duplicate_of": "current_user_input", "fingerprint": candidate.Fingerprint})
		}
		if refs := recentFingerprints[candidate.Fingerprint]; len(refs) > 0 {
			contextDuplicates = append(contextDuplicates, map[string]any{"source_ref": candidate.SourceRef, "duplicate_of": "recent_risu_context", "recent_source_refs": refs, "fingerprint": candidate.Fingerprint})
		}
	}
	sort.Slice(contextDuplicates, func(i, j int) bool {
		left := mapFromAny(contextDuplicates[i])
		right := mapFromAny(contextDuplicates[j])
		leftKey := extractionStringFromAny(left["source_ref"]) + "\x1f" + extractionStringFromAny(left["duplicate_of"])
		rightKey := extractionStringFromAny(right["source_ref"]) + "\x1f" + extractionStringFromAny(right["duplicate_of"])
		return leftKey < rightKey
	})

	baselineSeed := strings.Join([]string{
		sessionID,
		requestCorrelationID,
		mustCompactJSON(surfaces),
		mustCompactJSON(sameRowLanes),
		mustCompactJSON(semanticDuplicates),
		mustCompactJSON(contextDuplicates),
	}, "\x1f")
	deliveryContract := extractionStringFromAny(assembly.MemoryDeliveryPlan["contract_version"])
	priorityActive := deliveryContract == prepareTurnPriorityMemoryPlanVersion
	policyMode := "observation_only_4_1"
	if priorityActive {
		policyMode = "4_1_baseline_with_4_2_priority_result"
	}
	return map[string]any{
		"contract_version":                    "memory_injection_baseline.v1",
		"baseline_id":                         "mib_" + strings.TrimPrefix(prepareTurnTextHash(baselineSeed), "sha256:"),
		"status":                              "observed_pre_payload",
		"owner":                               "go",
		"policy_mode":                         policyMode,
		"selection_policy_changed":            priorityActive,
		"active_delivery_contract":            nilIfEmpty(deliveryContract),
		"active_score_version":                nilIfEmpty(extractionStringFromAny(assembly.MemoryDeliveryPlan["score_version"])),
		"active_selected_fact_ids":            stringSliceFromAny(assembly.MemoryDeliveryPlan["selected_fact_ids"]),
		"active_selected_fact_count":          intFromAny(assembly.MemoryDeliveryPlan["priority_fact_selected_count"], 0),
		"active_selected_turn_summary_ids":    stringSliceFromAny(assembly.MemoryDeliveryPlan["selected_turn_summary_ids"]),
		"active_selected_turn_summary_count":  intFromAny(assembly.MemoryDeliveryPlan["turn_summary_selected_count"], 0),
		"active_selected_priority_item_count": intFromAny(assembly.MemoryDeliveryPlan["priority_selected_count"], 0),
		"canonical_mutation":                  false,
		"vector_mutation":                     false,
		"surface_order":                       []string{"memory", "direct_evidence", "kg", "state", "persona", "relationship", "storyline", "pending_thread", "hierarchy_summary"},
		"surfaces":                            surfaces,
		"exact_same_row_lane_duplicates":      sameRowLanes,
		"semantic_duplicate_candidates":       semanticDuplicates,
		"current_input_or_recent_context_duplicate_candidates": contextDuplicates,
		"duplicate_detection": map[string]any{"method": "normalized_casefolded_text_fingerprint.v1", "automatic_suppression": false, "semantic_claim": "candidate_only"},
		"payload_application": map[string]any{"status": "planned_unobserved", "observation_owner": "risu_adapter"},
		"displayed_effect":    map[string]any{"status": "unobserved", "automatic_text_judgement": false},
	}
}

func deliveredPrepareTurnMemorySourceRefs(sessionID string, lineage map[string]any) []string {
	refs := make([]string, 0)
	for _, raw := range outputFidelityLineageSlice(lineage["items"]) {
		item := mapFromAny(raw)
		if !boolFromAny(item["delivered"]) {
			continue
		}
		ref := strings.TrimSpace(extractionStringFromAny(item["source_ref"]))
		if ref == "" {
			ref = prepareTurnMemoryLineageSourceRef(sessionID, item["source_row_id"])
		}
		if ref != "" {
			refs = appendUniqueMemorySearchText(refs, ref)
		}
	}
	return refs
}

func attachPrepareTurnOutputFidelityLineage(
	requestCorrelationID string,
	payloadPlan map[string]any,
	responseExecutionContract map[string]any,
	memoryLineage map[string]any,
) map[string]any {
	correlationID := strings.TrimSpace(requestCorrelationID)
	trace := mapFromAny(payloadPlan["guidance_application_trace"])
	traceItems := outputFidelityLineageSlice(trace["items"])
	for index, raw := range traceItems {
		item := mapFromAny(raw)
		refs := stringSliceFromAny(item["source_refs"])
		sort.Strings(refs)
		itemIDSeed := strings.Join([]string{
			correlationID,
			extractionStringFromAny(item["key"]),
			extractionStringFromAny(item["content_hash"]),
			strings.Join(refs, "\x1f"),
		}, "\x1f")
		item["item_id"] = "gi_" + strings.TrimPrefix(prepareTurnTextHash(itemIDSeed), "sha256:")
		traceItems[index] = item
	}

	planSeed := strings.Join([]string{
		correlationID,
		extractionStringFromAny(payloadPlan["auxiliary_hash"]),
		extractionStringFromAny(payloadPlan["input_context_hash"]),
		fmt.Sprint(traceItems),
	}, "\x1f")
	planID := "stp_" + strings.TrimPrefix(prepareTurnTextHash(planSeed), "sha256:")
	lineageID := "stl_" + strings.TrimPrefix(prepareTurnTextHash(strings.Join([]string{
		correlationID,
		planID,
		strings.Join(stringSliceFromAny(mapFromAny(responseExecutionContract["source_refs"])["all"]), "\x1f"),
	}, "\x1f")), "sha256:")
	payloadPlan["plan_id"] = planID
	payloadPlan["payload_plan_id"] = planID
	payloadPlan["archive_center_request_correlation_id"] = nilIfEmpty(correlationID)
	payloadPlan["official_risu_request_id_state"] = "official_risu_request_id_not_exposed"
	payloadPlan["generation_id_state"] = "unobserved"
	payloadPlan["auxiliary_observation_hash"] = prepareOR1CHash(prepareTurnAuxiliaryMessageHeader + "\n\n" + extractionStringFromAny(payloadPlan["auxiliary_text"]))
	payloadPlan["input_context_observation_hash"] = prepareOR1CHash("[Archive Center — Input Context]\n\n" + extractionStringFromAny(payloadPlan["input_context_text"]))
	payloadPlan["observation_hash_algorithm"] = "or1c_utf16_djb2.v1"

	trace["archive_center_request_correlation_id"] = nilIfEmpty(correlationID)
	trace["official_risu_request_id_state"] = "official_risu_request_id_not_exposed"
	trace["generation_id_state"] = "unobserved"
	trace["plan_id"] = planID
	trace["payload_plan_id"] = planID
	trace["prepare_lineage_id"] = lineageID
	trace["items"] = traceItems
	trace["final_output_status"] = "unobserved"
	trace["semantic_outcome"] = "unobserved"
	payloadPlan["guidance_application_trace"] = trace

	efficacyItemResults := make([]any, 0, len(traceItems))
	for _, raw := range traceItems {
		item := mapFromAny(raw)
		efficacyItemResults = append(efficacyItemResults, map[string]any{
			"item_id":           item["item_id"],
			"key":               item["key"],
			"source_refs":       stringSliceFromAny(item["source_refs"]),
			"application_state": extractionStringFromAny(item["status"]),
			"semantic_outcome":  "unobserved",
		})
	}
	guideEligibility := mapFromAny(payloadPlan["guide_eligibility"])
	guideEligibilityStatus := extractionStringFromAny(guideEligibility["status"])
	efficacyStatus := "unobserved"
	switch guideEligibilityStatus {
	case "eligible":
		efficacyStatus = "awaiting_final_and_explicit_live_review"
	case "off":
		efficacyStatus = "not_applicable_guide_off"
	case "no_support":
		efficacyStatus = "not_applicable_no_support"
	case "injection_disabled":
		efficacyStatus = "not_applicable_injection_disabled"
	case "budget_disabled":
		efficacyStatus = "not_applicable_budget_disabled"
	}
	if guideEligibilityStatus == "eligible" && intFromAny(trace["applied_count"], 0) == 0 {
		efficacyStatus = "not_applied_no_guidance_item"
	}
	guideEfficacyEvaluation := map[string]any{
		"contract_version":                      "guide_efficacy_evaluation.v1",
		"status":                                efficacyStatus,
		"archive_center_request_correlation_id": nilIfEmpty(correlationID),
		"official_risu_request_id_state":        "official_risu_request_id_not_exposed",
		"payload_plan_id":                       planID,
		"guide_mode":                            guideEligibility["guide_mode"],
		"guide_strength":                        guideEligibility["guide_strength"],
		"eligibility":                           guideEligibilityStatus,
		"eligibility_source_refs":               stringSliceFromAny(guideEligibility["source_refs"]),
		"coverage":                              guideEligibility["coverage"],
		"effective_budget_chars":                intFromAny(trace["budget_chars"], 0),
		"used_chars":                            intFromAny(trace["used_chars"], 0),
		"output_guidance_hash":                  trace["final_hash"],
		"item_results":                          efficacyItemResults,
		"final_output_status":                   "unobserved",
		"first_output_disposition":              "unobserved",
		"reroll_reason":                         "unobserved",
		"provider":                              "unobserved",
		"model":                                 "unobserved",
		"seed_or_cohort_id":                     "unobserved",
		"automatic_text_judgement":              false,
		"body_difference_is_efficacy_evidence":  false,
		"aggregate_score_allowed":               false,
		"semantic_outcome":                      "unobserved",
	}
	payloadPlan["guide_efficacy_evaluation"] = guideEfficacyEvaluation

	executionItems := make([]any, 0)
	for _, key := range []string{"must_preserve", "must_respond", "must_account", "must_not_assert"} {
		group := mapFromAny(responseExecutionContract[key])
		for index, raw := range outputFidelityLineageSlice(group["items"]) {
			item := mapFromAny(raw)
			refs := stringSliceFromAny(item["source_refs"])
			sort.Strings(refs)
			seed := strings.Join([]string{correlationID, key, fmt.Sprint(index), strings.Join(refs, "\x1f")}, "\x1f")
			executionItems = append(executionItems, map[string]any{
				"item_id":           "ei_" + strings.TrimPrefix(prepareTurnTextHash(seed), "sha256:"),
				"kind":              key,
				"source_refs":       refs,
				"application_state": "planned",
				"semantic_outcome":  "unobserved",
			})
		}
	}

	sourceRefs := stringSliceFromAny(mapFromAny(responseExecutionContract["source_refs"])["all"])
	return map[string]any{
		"contract_version":                      sourceToPayloadLineageContract,
		"status":                                "observed",
		"archive_center_request_correlation_id": nilIfEmpty(correlationID),
		"official_risu_request_id_state":        "official_risu_request_id_not_exposed",
		"generation_id_state":                   "unobserved",
		"lineage_id":                            lineageID,
		"payload_plan_id":                       planID,
		"plan_id":                               planID,
		"source_refs":                           sourceRefs,
		"sources":                               outputFidelityLineageSlice(memoryLineage["items"]),
		"execution_items":                       executionItems,
		"guide_efficacy_evaluation":             guideEfficacyEvaluation,
		"payload": map[string]any{
			"status":                         "planned",
			"observation_stage":              "archive_center_before_request_plan",
			"final_provider_payload_state":   "not_exposed",
			"auxiliary_hash":                 payloadPlan["auxiliary_hash"],
			"auxiliary_observation_hash":     payloadPlan["auxiliary_observation_hash"],
			"input_context_hash":             payloadPlan["input_context_hash"],
			"input_context_observation_hash": payloadPlan["input_context_observation_hash"],
			"observation_hash_algorithm":     "or1c_utf16_djb2.v1",
		},
		"final_output": map[string]any{
			"status":           "unobserved",
			"semantic_outcome": "unobserved",
		},
	}
}

func boundedLineageStringSlice(value any, limit int) []string {
	out := make([]string, 0)
	for _, item := range stringSliceFromAny(value) {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out = appendUniqueMemorySearchText(out, item)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func boundedMemoryInjectionSurfacePayloadApplication(value any) []any {
	allowed := map[string]bool{
		"memory": true, "direct_evidence": true, "kg": true, "state": true,
		"persona": true, "relationship": true, "storyline": true,
		"pending_thread": true, "hierarchy_summary": true,
	}
	out := make([]any, 0, 9)
	seen := map[string]bool{}
	for _, raw := range outputFidelityLineageSlice(value) {
		item := mapFromAny(raw)
		surface := strings.TrimSpace(extractionStringFromAny(item["surface"]))
		status := strings.TrimSpace(extractionStringFromAny(item["status"]))
		if !allowed[surface] || seen[surface] || (status != "applied" && status != "empty" && status != "missing" && status != "ambiguous") {
			continue
		}
		seen[surface] = true
		out = append(out, map[string]any{
			"surface":                 surface,
			"status":                  status,
			"rendered_count":          maxInt(0, intFromAny(item["rendered_count"], 0)),
			"payload_character_count": maxInt(0, intFromAny(item["payload_character_count"], 0)),
			"displayed_effect":        "unobserved",
		})
	}
	return out
}

func buildSourceToFinalLineage(req dto.M4CompleteTurnRequest, decision completeTurnSourceAcceptanceDecision) map[string]any {
	rawObservation := mapFromAny(req.ClientMeta["source_to_final_lineage_observation"])
	refs := boundedLineageStringSlice(rawObservation["source_refs"], 128)
	surfacePayloadApplication := boundedMemoryInjectionSurfacePayloadApplication(rawObservation["surface_payload_application"])
	itemResults := make([]any, 0, len(refs))
	for _, ref := range refs {
		itemResults = append(itemResults, map[string]any{
			"source_ref":       ref,
			"semantic_outcome": "unobserved",
		})
	}

	status := "unattached"
	reason := "source_to_final_correlation_missing"
	switch {
	case extractionStringFromAny(rawObservation["contract_version"]) != sourceToFinalLineageObservationContract:
		reason = "source_to_final_correlation_missing"
	case strings.TrimSpace(extractionStringFromAny(rawObservation["archive_center_request_correlation_id"])) == "" ||
		strings.TrimSpace(extractionStringFromAny(rawObservation["prepare_lineage_id"])) == "" ||
		strings.TrimSpace(extractionStringFromAny(rawObservation["payload_plan_id"])) == "":
		reason = "source_to_final_correlation_missing"
	case extractionStringFromAny(rawObservation["status"]) != "ready":
		reason = firstNonEmpty(extractionStringFromAny(rawObservation["reason_code"]), "source_to_final_observation_not_ready")
	case extractionStringFromAny(rawObservation["payload_application_status"]) != "applied" &&
		extractionStringFromAny(rawObservation["payload_application_status"]) != "empty":
		reason = "source_to_final_payload_application_unobserved"
	case extractionStringFromAny(rawObservation["payload_observation_stage"]) != "archive_center_before_request_return":
		reason = "source_to_final_payload_stage_unobserved"
	case extractionStringFromAny(rawObservation["final_provider_payload_state"]) != "not_exposed":
		reason = "source_to_final_provider_payload_boundary_invalid"
	case len(refs) == 0:
		reason = "source_to_final_source_refs_missing"
	case !decision.Enabled || !decision.Accepted:
		reason = firstNonEmpty(decision.Reason, "active_final_not_accepted")
	case decision.Observation.GenerationIDState != "observed" ||
		strings.TrimSpace(decision.Observation.GenerationID) == "":
		reason = "source_to_final_generation_unobserved"
	case extractionFirstNonEmpty(
		extractionStringFromAny(rawObservation["generation_id"]),
		extractionStringFromAny(rawObservation["final_generation_id"]),
	) != decision.Observation.GenerationID:
		reason = "source_to_final_generation_mismatch"
	default:
		status = "attached"
		reason = "source_to_final_active_final_attached"
	}
	attached := status == "attached"

	return map[string]any{
		"contract_version":                      sourceToFinalLineageContract,
		"status":                                status,
		"attached":                              attached,
		"reason_code":                           reason,
		"archive_center_request_correlation_id": nilIfEmpty(extractionStringFromAny(rawObservation["archive_center_request_correlation_id"])),
		"request_id_state": extractionFirstNonEmpty(
			extractionStringFromAny(rawObservation["request_id_state"]),
			"official_risu_request_id_not_exposed",
		),
		"official_risu_request_id_state": "official_risu_request_id_not_exposed",
		"prepare_lineage_id":             nilIfEmpty(extractionFirstNonEmpty(extractionStringFromAny(rawObservation["prepare_lineage_id"]), extractionStringFromAny(rawObservation["lineage_id"]))),
		"payload_plan_id":                nilIfEmpty(extractionFirstNonEmpty(extractionStringFromAny(rawObservation["payload_plan_id"]), extractionStringFromAny(rawObservation["plan_id"]))),
		"plan_id":                        nilIfEmpty(extractionFirstNonEmpty(extractionStringFromAny(rawObservation["payload_plan_id"]), extractionStringFromAny(rawObservation["plan_id"]))),
		"payload_application_status":     nilIfEmpty(extractionStringFromAny(rawObservation["payload_application_status"])),
		"payload_observation_stage":      nilIfEmpty(extractionStringFromAny(rawObservation["payload_observation_stage"])),
		"final_provider_payload_state":   nilIfEmpty(extractionStringFromAny(rawObservation["final_provider_payload_state"])),
		"payload_guidance_hash":          nilIfEmpty(extractionStringFromAny(rawObservation["payload_guidance_hash"])),
		"memory_injection_baseline_id":   nilIfEmpty(extractionStringFromAny(rawObservation["memory_injection_baseline_id"])),
		"surface_payload_application":    surfacePayloadApplication,
		"generation_id":                  nilIfEmpty(decision.Observation.GenerationID),
		"generation_id_state":            decision.Observation.GenerationIDState,
		"source_refs":                    refs,
		"final_output": map[string]any{
			"status":           map[bool]string{true: "observed", false: "unattached"}[attached],
			"lifecycle":        map[bool]string{true: "active_final", false: "unobserved"}[attached],
			"generation_id":    nilIfEmpty(decision.Observation.GenerationID),
			"content_hash":     nilIfEmpty(decision.Observation.ObservedContentHash),
			"hash_algorithm":   nilIfEmpty(decision.Observation.HashAlgorithm),
			"semantic_outcome": "unobserved",
		},
		"item_results":     itemResults,
		"displayed_effect": map[string]any{"status": "unobserved", "automatic_text_judgement": false},
		"semantic_outcome": "unobserved",
		"guide_efficacy_evaluation": map[string]any{
			"contract_version":                     "guide_efficacy_evaluation.v1",
			"status":                               map[bool]string{true: "awaiting_explicit_live_review", false: "unobserved"}[attached],
			"official_risu_request_id_state":       "official_risu_request_id_not_exposed",
			"active_final_message_observed":        attached,
			"active_final_display_observed":        "unobserved",
			"first_output_display_observed":        "unobserved",
			"first_output_disposition":             "unobserved",
			"reroll_reason":                        "unobserved",
			"guide_item_results_state":             "unobserved_not_transmitted",
			"item_results":                         []any{},
			"guide_mode":                           "unobserved",
			"guide_strength":                       "unobserved",
			"eligibility":                          "unobserved",
			"provider":                             "unobserved",
			"model":                                "unobserved",
			"seed_or_cohort_id":                    "unobserved",
			"automatic_text_judgement":             false,
			"body_difference_is_efficacy_evidence": false,
			"aggregate_score_allowed":              false,
			"semantic_outcome":                     "unobserved",
		},
	}
}
