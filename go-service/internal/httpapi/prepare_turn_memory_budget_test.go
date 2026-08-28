package httpapi

import (
	"reflect"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func prepareTurnTestDeliveryClassText(plan map[string]any, classKey string) string {
	for _, rawClass := range prepareTurnMemoryLineageSlice(plan["classes"]) {
		class := mapFromAny(rawClass)
		if extractionStringFromAny(class["key"]) == classKey {
			return extractionStringFromAny(class["text"])
		}
	}
	return ""
}

func TestMemoryRecallPlanConsolidatesExistingRecallWithoutChangingDelivery(t *testing.T) {
	const sessionID = "recall-plan-production"
	memories := []store.Memory{
		{ID: 1, ChatSessionID: sessionID, TurnIndex: 4, SummaryJSON: `{"turn_summary":"Mina hid the brass key under the old shrine."}`, Importance: 8},
		{ID: 2, ChatSessionID: sessionID, TurnIndex: 5, SummaryJSON: `{"turn_summary":"Mina promised Lia that the brass key would be returned after the gate opened."}`, Importance: 7},
		{ID: 3, ChatSessionID: sessionID, TurnIndex: 6, SummaryJSON: `{"turn_summary":"A distant market discussed the summer weather."}`, Importance: 9},
	}
	assembly := buildPrepareTurnInjectionAssembly(
		memories,
		nil,
		nil,
		nil,
		nil,
		nil,
		[]store.CharacterState{{ChatSessionID: sessionID, TurnIndex: 6, CharacterName: "Rowan", StatusJSON: `{"location":"gate"}`}},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		3,
		80,
		"Mina asks Rowan where the brass key was hidden.",
		"default",
		nil,
		map[string]any{"memory_search_result": "not_found", "search_result": "not_found"},
		nil,
	)

	finalTextBefore := extractionStringFromAny(assembly.MemoryDeliveryPlan["final_text"])
	finalHashBefore := extractionStringFromAny(assembly.MemoryDeliveryPlan["final_text_sha256"])
	orderBefore := append([]string(nil), stringsFromAny(assembly.MemoryDeliveryPlan["order"])...)
	plan := buildPrepareTurnMemoryRecallPlan(sessionID, map[string]any{
		"chat_session_id":        sessionID,
		"turn_index":             7,
		"source_observation_ref": "source:7",
		"unapproved_binding":     "must-not-escape",
	}, assembly)

	if plan["contract_version"] != "memory_recall_plan.v1" || plan["owner"] != "go" || !boolFromAny(plan["read_only"]) {
		t.Fatalf("invalid recall plan contract: %#v", plan)
	}
	if _, exists := plan["plan_id"]; exists {
		t.Fatalf("diagnostic recall plan must not create an acceptance identity: %#v", plan)
	}
	bindings := mapFromAny(plan["request_bindings"])
	if bindings["chat_session_id"] != sessionID || intFromAny(bindings["turn_index"], 0) != 7 || bindings["source_observation_ref"] != "source:7" {
		t.Fatalf("observed request bindings missing: %#v", bindings)
	}
	if _, exists := bindings["unapproved_binding"]; exists {
		t.Fatalf("unapproved binding escaped allowlist: %#v", bindings)
	}
	if !reflect.DeepEqual(plan["retrieval_methods"], assembly.Counts["retrieval_methods"]) {
		t.Fatalf("recall plan reinterpreted retrieval methods: plan=%#v assembly=%#v", plan["retrieval_methods"], assembly.Counts["retrieval_methods"])
	}
	rejections := mapFromAny(plan["lane_rejection_observations"])
	if !boolFromAny(rejections["not_distinct_global_exclusions"]) {
		t.Fatalf("lane rejection counters were presented as global exclusions: %#v", rejections)
	}
	if _, exists := plan["excluded_count"]; exists {
		t.Fatalf("recall plan invented a distinct global exclusion count: %#v", plan)
	}

	selection := mapFromAny(plan["selection"])
	lineageItems := prepareTurnMemoryLineageSlice(assembly.MemoryDeliveryLineage["items"])
	delivered := 0
	deferred := 0
	for _, rawItem := range lineageItems {
		if boolFromAny(mapFromAny(rawItem)["delivered"]) {
			delivered++
		} else {
			deferred++
		}
	}
	if intFromAny(selection["selected_memory_row_count"], -1) != intFromAny(assembly.Counts["selected_memory_total_count"], -2) ||
		intFromAny(selection["lineage_item_count"], -1) != len(lineageItems) ||
		intFromAny(selection["delivered_lineage_item_count"], -1) != delivered ||
		intFromAny(selection["deferred_lineage_item_count"], -1) != deferred {
		t.Fatalf("recall plan invented selection counts: selection=%#v lineage=%#v", selection, assembly.MemoryDeliveryLineage)
	}
	if deferred == 0 || delivered >= len(lineageItems) {
		t.Fatalf("tiny delivery budget fixture did not preserve selected versus delivered: selection=%#v", selection)
	}
	for _, rawItem := range prepareTurnMemoryLineageSlice(selection["selected_lineage_items"]) {
		item := mapFromAny(rawItem)
		for _, forbidden := range []string{"final_text", "summary", "protected_coverage_key"} {
			if _, exists := item[forbidden]; exists {
				t.Fatalf("bounded recall item exposed %s: %#v", forbidden, item)
			}
		}
	}

	directCoverage := mapFromAny(mapFromAny(plan["coverage"])["direct_entities"])
	if intFromAny(directCoverage["requested_count"], -1) != intFromAny(assembly.MemoryDeliveryPlan["direct_entity_memory_requested_count"], -2) ||
		intFromAny(directCoverage["delivered_count"], -1) != intFromAny(assembly.MemoryDeliveryPlan["direct_entity_memory_delivered_count"], -2) ||
		intFromAny(directCoverage["gap_count"], -1) != intFromAny(assembly.MemoryDeliveryPlan["direct_entity_memory_gap"], -2) {
		t.Fatalf("direct-entity coverage drifted from delivery plan: %#v", directCoverage)
	}
	if intFromAny(directCoverage["gap_count"], 0) == 0 {
		t.Fatalf("missing direct-entity coverage was hidden: %#v", directCoverage)
	}
	if strings.Contains(finalTextBefore, "summer weather") {
		t.Fatalf("unrelated memory filled an unused slot: %q", finalTextBefore)
	}
	if finalTextBefore != extractionStringFromAny(assembly.MemoryDeliveryPlan["final_text"]) ||
		finalHashBefore != extractionStringFromAny(assembly.MemoryDeliveryPlan["final_text_sha256"]) ||
		!reflect.DeepEqual(orderBefore, stringsFromAny(assembly.MemoryDeliveryPlan["order"])) {
		t.Fatalf("diagnostic recall plan mutated final delivery: before=%q after=%q", finalTextBefore, assembly.MemoryDeliveryPlan["final_text"])
	}
}

func TestMemoryRecallPlanSeparatesDeliveryGapFromUnusedRequestedSlots(t *testing.T) {
	out := prepareTurnInjectionAssembly{
		MemoryRecallQuery: "Mina checks the brass key.",
		ActualMemoryText:  "[Memory]\n- Mina secured the brass key.",
		Counts: map[string]any{
			"recall_query_sources":        []string{"current_user_input"},
			"selected_memory_total_count": 1,
			"retrieval_methods": map[string]any{
				"lexical": map[string]any{"status": "ready", "candidate_count": 1, "selected_count": 1},
			},
		},
		MemoryDeliveryLineage: map[string]any{
			"eligible_memory_count": 1,
			"items":                 []map[string]any{{"source_row_id": 1, "turn_index": 2, "selection_lane": "relevant", "delivered": true, "delivery_status": "delivered_final"}},
		},
	}
	out.MemoryDeliveryPlan = buildPrepareTurnMemoryDeliveryPlan(&out, 9000, map[string]any{
		"_core_objective_memory_max_items_present": true,
		"_core_objective_memory_max_items":         4,
	})
	plan := buildPrepareTurnMemoryRecallPlan("recall-gap-separation", map[string]any{"chat_session_id": "recall-gap-separation"}, out)
	core := mapFromAny(mapFromAny(plan["coverage"])["core_objective_memory"])
	if intFromAny(core["delivery_gap_count"], -1) != 0 || intFromAny(core["unused_requested_slots"], 0) == 0 || core["status"] != "covered" {
		t.Fatalf("unused requested slots were misreported as a memory delivery gap: %#v", core)
	}
}

func TestMEMDSevenClassPlanSeparatesProtectedGuidanceFromActualMemory(t *testing.T) {
	out := prepareTurnInjectionAssembly{
		ActualMemoryText:          "[Memory]\n- actual event memory",
		ProtectedMemoryText:       "[Protected Memory Guidance]\n- protected continuity guard",
		CharacterObjectiveText:    "[Character Objective States]\n- Mina: injured hand",
		CharacterRelationshipText: "[Character Relationships]\n- Mina trusts Lia",
		KGText:                    "[Knowledge Graph]\nMina --trusts--> Lia",
		CanonWorldText:            "[Canonical World States]\n- the brass key is on the desk",
		WorldRulesText:            "[World Rules]\n- winter road is closed",
		PendingThreadText:         "[Pending Threads]\n- find the missing key",
		DirectEvidenceText:        "[Direct Evidence]\n- the key was seen on the desk",
	}
	plan := buildPrepareTurnMemoryDeliveryPlan(&out, 9000, map[string]any{})
	if plan["final_budget_owner"] != "go_memory_delivery_plan" {
		t.Fatalf("owner=%v", plan["final_budget_owner"])
	}
	if hash := extractionStringFromAny(plan["final_text_sha256"]); len(hash) != 64 {
		t.Fatalf("final text hash=%q, want sha256", hash)
	}
	classes, _ := plan["classes"].([]map[string]any)
	if len(classes) != 7 {
		t.Fatalf("class count=%d, want 7", len(classes))
	}
	byKey := map[string]map[string]any{}
	for _, class := range classes {
		byKey[extractionStringFromAny(class["key"])] = class
	}
	eventText := extractionStringFromAny(byKey["event_recent"]["text"])
	protectedText := extractionStringFromAny(byKey["protected_secret"]["text"])
	subjectiveText := extractionStringFromAny(byKey["subjective_relationship"]["text"])
	if !strings.Contains(eventText, "actual event memory") || strings.Contains(eventText, "protected continuity guard") {
		t.Fatalf("event class mixed protected guidance: %q", eventText)
	}
	if !strings.Contains(protectedText, "protected continuity guard") || strings.Contains(protectedText, "actual event memory") {
		t.Fatalf("protected class mixed actual memory: %q", protectedText)
	}
	if !strings.Contains(subjectiveText, "Mina trusts Lia") ||
		!strings.Contains(subjectiveText, "Mina --trusts--> Lia") {
		t.Fatalf("stored relationship evidence missing: %q", subjectiveText)
	}
	if got := intFromAny(byKey["subjective_relationship"]["selected_count"], 0); got != 2 {
		t.Fatalf("subjective relationship selected count=%d, want only the two stored evidence rows: %q", got, subjectiveText)
	}
}

func TestMEMDKeepsTextOnlyFactsAcrossClassesWithoutSourceCoordinates(t *testing.T) {
	out := prepareTurnInjectionAssembly{
		LatestDirectEvidenceText: "the brass key is on the desk",
		DirectEvidenceText:       "[Direct Evidence]\n- [vector, turn 9] the brass key is on the desk",
		CanonWorldText:           "[Canonical World States]\n- [turn 9] the brass key is on the desk",
		CharacterPrivateText:     "[Character Private Recollection]\n- the brass key is on the desk",
	}
	plan := buildPrepareTurnMemoryDeliveryPlan(&out, 9000, map[string]any{})
	finalText := extractionStringFromAny(plan["final_text"])
	if strings.Count(finalText, "the brass key is on the desk") != 4 {
		t.Fatalf("text-only facts were merged without source coordinates: %q", finalText)
	}
	classes, _ := plan["classes"].([]map[string]any)
	for _, class := range classes {
		if class["key"] == "world_state" && intFromAny(class["deduplicated_count"], 0) != 0 {
			t.Fatalf("world-state text-only fact was deduplicated without coordinates: %#v", class)
		}
	}
}

func TestMEMDKeepsCoordinateMissingRepeatedTextWithinClasses(t *testing.T) {
	const repeated = "the bell rang twice"
	out := prepareTurnInjectionAssembly{
		LatestDirectEvidenceText: repeated,
		DirectEvidenceText:       "[Direct Evidence]\n- " + repeated,
		CharacterObjectiveText:   "[Character Objective States]\n- " + repeated,
		CanonCharacterText:       "[Canonical Characters]\n- " + repeated,
		CanonWorldText:           "[Canonical World States]\n- " + repeated,
		WorldRulesText:           "[World Rules]\n- " + repeated,
	}
	plan := buildPrepareTurnMemoryDeliveryPlan(&out, 9000, map[string]any{})
	for _, classKey := range []string{"direct_evidence", "character_objective", "world_state"} {
		text := prepareTurnTestDeliveryClassText(plan, classKey)
		if strings.Count(text, repeated) != 2 {
			t.Fatalf("%s text-only projections were merged without source coordinates: %q", classKey, text)
		}
	}
}

func TestPrepareTurnDistinctDeliveryItemsDropsOnlyExactTrimmedLineDuplicates(t *testing.T) {
	firstOrder := `- relationship_state: {"observations":[{"from":"Mira"},{"to":"Noah"}]}`
	reversedOrder := `- relationship_state: {"observations":[{"to":"Noah"},{"from":"Mira"}]}`
	got := prepareTurnDistinctDeliveryItems(
		"[Character Private Recollection]\n- Mira trusts Noah\n  - Mira trusts Noah  \n- [turn 8] Mira trusts Noah",
		"[Canonical Relationships]\n- Mira trusts Noah\n- [turn 9] Mira trusts Noah\n"+firstOrder,
		"[Knowledge Graph]\n"+reversedOrder+"\n- Noah doubts Mira",
	)
	want := []string{
		"- Mira trusts Noah",
		"- [turn 8] Mira trusts Noah",
		"- [turn 9] Mira trusts Noah",
		firstOrder,
		reversedOrder,
		"- Noah doubts Mira",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("distinct delivery items = %#v, want %#v", got, want)
	}
}

func TestMEMDSubjectiveRelationshipDropsOnlyExactRepeatedText(t *testing.T) {
	out := prepareTurnInjectionAssembly{
		CharacterPrivateText: "[Character Private Recollection]\n- Mira trusts Noah",
		KGText:               "[Knowledge Graph]\n- Mira trusts Noah",
	}
	plan := buildPrepareTurnMemoryDeliveryPlan(&out, 9000, map[string]any{})
	classText := prepareTurnTestDeliveryClassText(plan, "subjective_relationship")
	if got := strings.Count(classText, "Mira trusts Noah"); got != 1 {
		t.Fatalf("exact repeated subjective line count=%d, want 1: %q", got, classText)
	}
	for _, rawClass := range prepareTurnMemoryLineageSlice(plan["classes"]) {
		class := mapFromAny(rawClass)
		if extractionStringFromAny(class["key"]) == "subjective_relationship" &&
			(intFromAny(class["selected_count"], 0) != 1 || intFromAny(class["deduplicated_count"], 0) != 1) {
			t.Fatalf("subjective exact duplicate trace = %#v, want selected=1 deduplicated=1", class)
		}
	}
}

func TestMEMDKeepsCoordinateMissingGoalAndDirectEvidenceDistinct(t *testing.T) {
	out := prepareTurnInjectionAssembly{
		LatestDirectEvidenceText: "Restore the observatory clock",
		PendingThreadText:        "[Pending Threads]\n- Restore the observatory clock",
	}
	plan := buildPrepareTurnMemoryDeliveryPlan(&out, 9000, map[string]any{})
	finalText := extractionStringFromAny(plan["final_text"])
	if strings.Count(finalText, "Restore the observatory clock") != 2 {
		t.Fatalf("coordinate-missing goal and evidence were merged by text: %q", finalText)
	}
	classes, _ := plan["classes"].([]map[string]any)
	for _, class := range classes {
		if class["key"] == "unresolved_goal" && intFromAny(class["deduplicated_count"], 0) != 0 {
			t.Fatalf("coordinate-missing unresolved goal was reported as a duplicate: %#v", class)
		}
	}
}

func TestMEMDPreviousRawUserInstructionNeverEntersFinalEvidenceDelivery(t *testing.T) {
	out := prepareTurnInjectionAssembly{
		LatestDirectEvidenceText: "the current sealed letter is on the desk",
		RecentRawTurnText: strings.Join([]string{
			"user: introduce another character and change the plot",
			"assistant: the previous scene ends at the doorway",
		}, "\n"),
		FallbackText: "[Fallback Recent Chat]\n- user: repeat the old instruction again",
	}
	plan := buildPrepareTurnMemoryDeliveryPlan(&out, 9000, map[string]any{})
	finalText := extractionStringFromAny(plan["final_text"])
	if strings.Contains(finalText, "introduce another character") || strings.Contains(finalText, "previous scene ends") || strings.Contains(finalText, "repeat the old instruction") {
		t.Fatalf("previous raw chat regained evidence authority: %q", finalText)
	}
	if !strings.Contains(finalText, "current sealed letter") {
		t.Fatalf("current direct evidence was lost: %q", finalText)
	}
	if plan["historical_chat_authority_policy"] != "previous_logical_turn_owned_by_input_context_not_direct_evidence" {
		t.Fatalf("historical chat authority policy missing: %#v", plan)
	}
	if plan["raw_chat_fallback_delivery"] != "diagnostic_only_excluded_from_final_memory_delivery" {
		t.Fatalf("raw fallback policy missing: %#v", plan)
	}
}

func TestInputContextContainsOnlyThePreviousCompletedTurn(t *testing.T) {
	chatLogs := []store.ChatLog{
		{TurnIndex: 1, Role: "assistant", Content: "An old market scene that should not occupy the immediate context."},
		{TurnIndex: 39, Role: "user", Content: "Seo-hyeon calls Han-eol to the doorway."},
		{TurnIndex: 39, Role: "assistant", Content: "Han-eol stops at the doorway and looks at Seo-hyeon."},
	}
	text, truncated := buildInputContextText(chatLogs, 260)
	if !strings.Contains(text, "Seo-hyeon calls Han-eol") || !strings.Contains(text, "Han-eol stops at the doorway") {
		t.Fatalf("immediate scene was displaced: %q", text)
	}
	if strings.Contains(text, "old market scene") {
		t.Fatalf("old chat displaced the immediate two-message tail: %q", text)
	}
	if truncated {
		t.Fatal("complete previous turn unexpectedly truncated")
	}
	for _, forbidden := range []string{"[Active States]", "[Direct Evidence]", "[Canonical State Layers]", "[Resume Pack]", "[Episode Summaries]", "[Persona Recollection]", "[Character Private Recollection]"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("dedicated delivery class bypassed through Input Context: %q", text)
		}
	}
}

func TestMEMDAutomaticDeliversRequiredBeforeEarlierAuxiliaryClass(t *testing.T) {
	out := prepareTurnInjectionAssembly{
		ActualMemoryText:   "[Memory]\n- Mina must remember the current doorway promise.",
		DirectEvidenceText: "[Direct Evidence]\n- " + strings.Repeat("older supporting excerpt ", 30),
	}
	plan := buildPrepareTurnMemoryDeliveryPlan(&out, 500, map[string]any{})
	if plan["borrowing_policy"] != "required_then_auxiliary_global_envelope" {
		t.Fatalf("borrowing policy=%v", plan["borrowing_policy"])
	}
	finalText := extractionStringFromAny(plan["final_text"])
	if !strings.Contains(finalText, "current doorway promise") {
		t.Fatalf("required current memory was crowded out by auxiliary evidence: %q", finalText)
	}
	if strings.Contains(finalText, "older supporting excerpt") {
		t.Fatalf("oversized auxiliary evidence displaced required memory: %q", finalText)
	}
	if intFromAny(plan["candidate_chars"], 0) <= intFromAny(plan["selected_chars"], 0) ||
		intFromAny(plan["selected_chars"], 0) != intFromAny(plan["final_delivery_chars"], -1) ||
		intFromAny(plan["excluded_count"], 0) != 1 ||
		prepareTurnPayloadBudgetReasonCounts(plan["exclusion_reasons"])["memory_char_budget"] != 1 {
		t.Fatalf("memory candidate-to-final budget trace = %#v", plan)
	}
	classes, _ := plan["classes"].([]map[string]any)
	for _, class := range classes {
		if class["key"] != "direct_evidence" {
			continue
		}
		if intFromAny(class["selected_count"], 0) != 0 || intFromAny(class["deferred_count"], 0) != 1 {
			t.Fatalf("oversized auxiliary evidence should remain deferred: %#v", class)
		}
		if intFromAny(class["auxiliary_eligible_count"], 0) != 1 || intFromAny(class["auxiliary_selected_count"], 0) != 0 {
			t.Fatalf("auxiliary tier trace=%#v", class)
		}
	}
}

func TestPrepareTurnSurfaceTextDropsEmptyNestedStructureWithoutDroppingFalseOrZero(t *testing.T) {
	text := prepareTurnSurfaceText(map[string]any{
		"empty_object": map[string]any{},
		"empty_array":  []any{},
		"empty_text":   "  ",
		"state": map[string]any{
			"location": "doorway",
			"unused":   map[string]any{"note": ""},
		},
		"visible": false,
		"count":   float64(0),
	})
	for _, unwanted := range []string{"empty_object", "empty_array", "empty_text", "unused"} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("empty structure %q survived: %s", unwanted, text)
		}
	}
	for _, wanted := range []string{`"location":"doorway"`, `"visible":false`, `"count":0`} {
		if !strings.Contains(text, wanted) {
			t.Fatalf("meaningful value %q missing: %s", wanted, text)
		}
	}
}

func TestMEMDCustomZeroUsesGlobalSemanticSelectionAndWholeItems(t *testing.T) {
	out := prepareTurnInjectionAssembly{
		ActualMemoryText:    "[Memory]\n- first complete event\n- second complete event",
		ProtectedMemoryText: "[Protected Memory Guidance]\n- keep this secret protected",
	}
	perspective := map[string]any{
		"_memory_delivery_budget_mode": "custom",
		"_memory_delivery_budgets": map[string]int{
			"event_recent":     40,
			"protected_secret": 0,
		},
	}
	plan := buildPrepareTurnMemoryDeliveryPlan(&out, 500, perspective)
	if plan["mode"] != "custom" {
		t.Fatalf("mode=%v", plan["mode"])
	}
	classes, _ := plan["classes"].([]map[string]any)
	for _, class := range classes {
		if class["key"] == "protected_secret" && intFromAny(class["configured_reserved_chars"], -1) != 0 {
			t.Fatalf("zero custom reservation must remain an explicit automatic-within-global request: %#v", class)
		}
	}
	finalText := extractionStringFromAny(plan["final_text"])
	if !strings.Contains(finalText, "keep this secret protected") {
		t.Fatalf("zero custom reservation did not use available global space: %q", finalText)
	}
	if strings.Contains(finalText, "first complete even") && !strings.Contains(finalText, "first complete event") {
		t.Fatalf("item was truncated: %q", finalText)
	}
}

func TestMEMDAutomaticHasNoFixedPerClassReservations(t *testing.T) {
	out := prepareTurnInjectionAssembly{
		ActualMemoryText:          "[Memory]\n- current event",
		CharacterRelationshipText: "[Character Relationships]\n- Mina distrusts Lia",
	}
	plan := buildPrepareTurnMemoryDeliveryPlan(&out, 18000, map[string]any{})
	if boolFromAny(plan["automatic_class_quotas"]) {
		t.Fatalf("automatic per-class quotas survived: %#v", plan)
	}
	classes, _ := plan["classes"].([]map[string]any)
	for _, class := range classes {
		if class["reserved_chars"] != nil || class["configured_reserved_chars"] != nil {
			t.Fatalf("automatic class retained a numeric reservation: %#v", class)
		}
	}
}

func TestPayloadApplicationPlanKeepsNarrativeBudgetIndependentAndOmitsHostRecentChat(t *testing.T) {
	plan := buildPrepareTurnPayloadApplicationPlan(
		"current user",
		"[Original Work]\ncanon",
		"[Memory]\nevent",
		"[Input Context]\nprevious",
		true,
		true,
		9000,
		2000,
		20,
		[]prepareTurnGuidanceItem{
			{Key: "short", Title: "Short", Text: "1234567890", SourceRefs: []string{"input:1"}},
			{Key: "too_long", Title: "Too long", Text: "this item must stay whole", SourceRefs: []string{"input:1"}},
		},
		"not_configured",
	)
	auxiliary := extractionStringFromAny(plan["auxiliary_text"])
	for _, want := range []string{"[Original Work]\ncanon", "[Memory]\nevent", "1234567890"} {
		if !strings.Contains(auxiliary, want) {
			t.Fatalf("auxiliary missing %q: %q", want, auxiliary)
		}
	}
	if strings.Contains(auxiliary, "this item") {
		t.Fatalf("over-budget guidance was partially or wholly injected: %q", auxiliary)
	}
	if plan["auxiliary_hash"] != prepareTurnTextHash(auxiliary) {
		t.Fatalf("auxiliary hash mismatch: %v", plan["auxiliary_hash"])
	}
	trace := mapFromAny(plan["guidance_application_trace"])
	if intFromAny(trace["deferred_count"], 0) != 1 || boolFromAny(trace["mid_item_truncation"]) {
		t.Fatalf("guidance trace=%#v", trace)
	}
	if extractionStringFromAny(plan["input_context_text"]) != "" || intFromAny(plan["input_context_chars"], -1) != 0 {
		t.Fatalf("host recent chat survived in payload plan: %#v", plan)
	}
}

func TestPrepareTurnAdaptiveMemoryBudgetIsResolvedByGo(t *testing.T) {
	effective, trace := resolvePrepareTurnMemoryBudget(9000, map[string]any{
		"memory_budget_observation": map[string]any{
			"extra_chars":         2500,
			"current_chat_tokens": 350000,
			"token_source":        "risu_observed",
		},
	})
	if effective != 11500 {
		t.Fatalf("effective=%d, want configured UI budget 11500 when context capacity is unobserved", effective)
	}
	if trace["owner"] != "go" || boolFromAny(trace["calculation_in_javascript"]) {
		t.Fatalf("budget owner trace=%#v", trace)
	}

	effective, trace = resolvePrepareTurnMemoryBudget(9000, map[string]any{
		"memory_budget_observation": map[string]any{
			"extra_chars":           2500,
			"current_chat_tokens":   5000,
			"context_window_tokens": 20000,
			"current_chat_chars":    20000,
			"token_source":          "risu_observed",
			"context_window_source": "model_observed",
		},
	})
	if effective != 20000 || trace["calculation"] != "observed_capacity_request_demand" {
		t.Fatalf("observed larger context did not expand the delivery budget dynamically: effective=%d trace=%#v", effective, trace)
	}
}
