package httpapi

import (
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

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

func TestMEMDRemovesObjectiveFactDuplicatesAcrossDeliveryClasses(t *testing.T) {
	out := prepareTurnInjectionAssembly{
		LatestDirectEvidenceText: "the brass key is on the desk",
		DirectEvidenceText:       "[Direct Evidence]\n- [vector, turn 9] the brass key is on the desk",
		CanonWorldText:           "[Canonical World States]\n- [turn 9] the brass key is on the desk",
		CharacterPrivateText:     "[Character Private Recollection]\n- the brass key is on the desk",
	}
	plan := buildPrepareTurnMemoryDeliveryPlan(&out, 9000, map[string]any{})
	finalText := extractionStringFromAny(plan["final_text"])
	if strings.Count(finalText, "the brass key is on the desk") != 2 {
		t.Fatalf("objective duplicate was not removed or subjective meaning was lost: %q", finalText)
	}
	classes, _ := plan["classes"].([]map[string]any)
	for _, class := range classes {
		if class["key"] == "world_state" && intFromAny(class["deduplicated_count"], 0) == 0 {
			t.Fatalf("world-state duplicate was not traced: %#v", class)
		}
	}
}

func TestMEMDRemovesExactGoalDuplicateFromDirectEvidence(t *testing.T) {
	out := prepareTurnInjectionAssembly{
		LatestDirectEvidenceText: "Restore the observatory clock",
		PendingThreadText:        "[Pending Threads]\n- Restore the observatory clock",
	}
	plan := buildPrepareTurnMemoryDeliveryPlan(&out, 9000, map[string]any{})
	finalText := extractionStringFromAny(plan["final_text"])
	if strings.Count(finalText, "Restore the observatory clock") != 1 {
		t.Fatalf("exact goal evidence was delivered through two classes: %q", finalText)
	}
	classes, _ := plan["classes"].([]map[string]any)
	for _, class := range classes {
		if class["key"] == "unresolved_goal" && intFromAny(class["deduplicated_count"], 0) == 0 {
			t.Fatalf("unresolved-goal duplicate was not traced: %#v", class)
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

func TestMEMDDoesNotBorrowUnusedBudgetForUnprovenEventLeftovers(t *testing.T) {
	out := prepareTurnInjectionAssembly{
		EpisodeText: "[Episode Summaries]\n- " + strings.Repeat("old unrelated episode ", 90),
	}
	plan := buildPrepareTurnMemoryDeliveryPlan(&out, 9000, map[string]any{})
	if plan["borrowing_policy"] != "current_entity_relevance_selected_classes_only" {
		t.Fatalf("borrowing policy=%v", plan["borrowing_policy"])
	}
	classes, _ := plan["classes"].([]map[string]any)
	for _, class := range classes {
		if class["key"] != "event_recent" {
			continue
		}
		if intFromAny(class["selected_count"], 0) != 0 || intFromAny(class["deferred_count"], 0) != 1 {
			t.Fatalf("oversized unrelated leftover should remain deferred: %#v", class)
		}
		if intFromAny(class["borrowed_chars"], -1) != 0 {
			t.Fatalf("borrowed chars=%v, want 0", class["borrowed_chars"])
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

func TestMEMDCustomZeroUsesAutomaticReservationAndWholeItems(t *testing.T) {
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
		if class["key"] == "protected_secret" && intFromAny(class["reserved_chars"], 0) <= 0 {
			t.Fatalf("zero custom secret reservation must fall back to automatic: %#v", class)
		}
	}
	finalText := extractionStringFromAny(plan["final_text"])
	if strings.Contains(finalText, "first complete even") && !strings.Contains(finalText, "first complete event") {
		t.Fatalf("item was truncated: %q", finalText)
	}
}

func TestMEMDStandardAutomaticBudgetProfile(t *testing.T) {
	got := prepareTurnAutomaticMemoryBudgets(18000)
	want := map[string]int{
		"event_recent": 3500, "character_objective": 2500, "subjective_relationship": 3000,
		"world_state": 2500, "protected_secret": 1200, "unresolved_goal": 1800, "direct_evidence": 3500,
	}
	for key, value := range want {
		if got[key] != value {
			t.Fatalf("budget[%s]=%d, want %d", key, got[key], value)
		}
	}
}

func TestMEMDAutomaticBudgetDistributesRaisedGlobalCapAcrossClasses(t *testing.T) {
	raised := prepareTurnAutomaticMemoryBudgets(11500)
	total := 0
	for _, key := range prepareTurnMemoryDeliveryOrder {
		if raised[key] <= 0 {
			t.Fatalf("raised budget[%s]=%d, want positive allocation", key, raised[key])
		}
		total += raised[key]
	}
	if total > 11500 || total < 11490 {
		t.Fatalf("distributed total=%d, want integer-rounded allocation near global cap 11500", total)
	}
}

func TestPayloadApplicationPlanKeepsNarrativeBudgetIndependentAndWhole(t *testing.T) {
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
	if extractionStringFromAny(plan["input_context_text"]) != "[Input Context]\nprevious" {
		t.Fatalf("input context changed: %q", plan["input_context_text"])
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
	if effective != 20500 {
		t.Fatalf("effective=%d, want 20500", effective)
	}
	if trace["owner"] != "go" || boolFromAny(trace["calculation_in_javascript"]) {
		t.Fatalf("budget owner trace=%#v", trace)
	}
}
