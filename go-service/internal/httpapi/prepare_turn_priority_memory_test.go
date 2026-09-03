package httpapi

import (
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/pdfmemory"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func priorityMemoryTestContext(maxItems int) map[string]any {
	return map[string]any{
		"_priority_memory_enabled":   true,
		"_priority_memory_max_items": maxItems,
	}
}

func Test42PriorityMemoryProductionAssemblyKeepsStoredScoreThroughPayloadPlan(t *testing.T) {
	const sessionID = "priority-score-production"
	memories := []store.Memory{
		{ID: 1, ChatSessionID: sessionID, TurnIndex: 20, SummaryJSON: `{"turn_summary":"Haneul already planted rapeseed in the home garden."}`, Importance: 9},
		{ID: 2, ChatSessionID: sessionID, TurnIndex: 21, SummaryJSON: `{"turn_summary":"Haneul mentioned the home garden during a casual conversation."}`, Importance: 4},
		{ID: 3, ChatSessionID: sessionID, TurnIndex: 22, SummaryJSON: `{"turn_summary":"An unrelated harbor changed its evening bell."}`, Importance: 10},
	}
	assembly := buildPrepareTurnInjectionAssembly(
		memories, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		5, 60000,
		"Haneul checks what to do next with rapeseed in the home garden.",
		"default", nil,
		map[string]any{"memory_search_result": "not_found", "search_result": "not_found"},
		nil,
		priorityMemoryTestContext(1),
	)
	plan := assembly.MemoryDeliveryPlan
	if plan["contract_version"] != prepareTurnPriorityMemoryPlanVersion || plan["score_version"] != prepareTurnPriorityMemoryScoreVersion {
		t.Fatalf("4.2 priority plan not used: %#v", plan)
	}
	if plan["final_budget_owner"] != "go_priority_memory_delivery_plan" {
		t.Fatalf("priority budget owner mismatch: %#v", plan["final_budget_owner"])
	}
	finalText := extractionStringFromAny(plan["final_text"])
	if !strings.Contains(finalText, "already planted rapeseed") {
		t.Fatalf("high-importance completion fact did not reach final payload text: %q", finalText)
	}
	if strings.Contains(finalText, "casual conversation") || strings.Contains(finalText, "unrelated harbor") {
		t.Fatalf("global K was filled with lower priority or unrelated memory: %q", finalText)
	}
	if intFromAny(plan["priority_selected_count"], 0) != 1 || boolFromAny(plan["low_score_backfill_after_k"]) {
		t.Fatalf("global K/no-backfill contract mismatch: %#v", plan)
	}

	foundStoredImportance := false
	for _, raw := range prepareTurnMemoryLineageSlice(plan["priority_items"]) {
		item := mapFromAny(raw)
		if strings.Contains(extractionStringFromAny(item["complete_text"]), "already planted rapeseed") {
			foundStoredImportance = extractionFloatFromAny(item["importance_score"], 0) == 0.9 &&
				extractionStringFromAny(item["selection_status"]) == "selected" &&
				extractionFloatFromAny(item["final_score"], 0) > 0
		}
	}
	if !foundStoredImportance {
		t.Fatalf("stored importance was not preserved on the selected fact: %#v", plan["priority_items"])
	}
}

func Test42PriorityMemoryProductionAssemblyPreservesTypedSourceScores(t *testing.T) {
	pending := []store.PendingThread{
		{ID: 41, ThreadKey: "ledger-return", Description: "Haneul must return the red archive ledger.", Status: "open", SourceTurn: 30, Priority: 9},
		{ID: 42, ThreadKey: "ledger-polish", Description: "Haneul may polish the red archive ledger cover.", Status: "open", SourceTurn: 31, Priority: 2},
	}
	assembly := buildPrepareTurnInjectionAssembly(
		nil, nil, nil, nil, nil, nil, nil, pending, nil, nil, nil, nil, nil,
		5, 60000, "Haneul handles the red archive ledger.", "default", nil,
		map[string]any{"memory_search_result": "not_found", "search_result": "not_found"}, nil,
		priorityMemoryTestContext(1),
	)
	plan := assembly.MemoryDeliveryPlan
	finalText := extractionStringFromAny(plan["final_text"])
	if !strings.Contains(finalText, "must return") || strings.Contains(finalText, "may polish") {
		t.Fatalf("stored pending-thread priority did not control global K: %q", finalText)
	}
	found := false
	for _, raw := range prepareTurnMemoryLineageSlice(plan["priority_items"]) {
		item := mapFromAny(raw)
		if intFromAny(item["source_row_id"], 0) == 41 {
			found = extractionFloatFromAny(item["importance_score"], 0) == 0.9 &&
				extractionStringFromAny(item["selection_status"]) == "selected"
		}
	}
	if !found {
		t.Fatalf("typed source score lineage was not preserved: %#v", plan["priority_items"])
	}
}

func Test42PriorityMemorySplitsStructuredStateAndResolvesCurrentValue(t *testing.T) {
	out := &prepareTurnInjectionAssembly{
		CanonWorldText: strings.Join([]string{
			"[Canonical World States]",
			`- scene_state [historical turn=10]: {"garden":{"rapeseed":{"status":"planned","location":"home"}}}`,
			`- scene_state [latest_observed turn=12]: {"garden":{"rapeseed":{"status":"completed","location":"home"}}}`,
		}, "\n"),
		Counts: map[string]any{},
	}
	plan := buildPrepareTurnMemoryDeliveryPlan(out, 60000, map[string]any{
		"_priority_memory_enabled":   true,
		"_priority_memory_max_items": 8,
		"_priority_memory_query":     "rapeseed home garden status",
	})
	if plan["contract_version"] != prepareTurnPriorityMemoryPlanVersion {
		t.Fatalf("priority plan not selected: %#v", plan)
	}
	finalText := extractionStringFromAny(plan["final_text"])
	if !strings.Contains(finalText, "status: completed") || strings.Contains(finalText, "status: planned") {
		t.Fatalf("request-scoped current resolution did not prefer completed status: %q", finalText)
	}
	if strings.Count(finalText, "location: home") != 1 {
		t.Fatalf("same canonical location consumed more than one K slot: %q", finalText)
	}
	if intFromAny(plan["priority_candidate_count"], 0) <= intFromAny(plan["priority_resolved_count"], 0) {
		t.Fatalf("canonical supersession was not recorded: %#v", plan)
	}
}

func Test42PriorityMemoryProductionTypedRowsResolveByCurrentFieldIdentity(t *testing.T) {
	canonical := []store.CanonicalStateLayer{
		{ID: 71, ChatSessionID: "typed-current", LayerType: "scene_state", Content: `{"garden":{"rapeseed":{"status":"planned","location":"home"}}}`, TurnIndex: 10, SourceTurn: 10, LastVerifiedTurn: 10, Confidence: 0.9},
		{ID: 72, ChatSessionID: "typed-current", LayerType: "scene_state", Content: `{"garden":{"rapeseed":{"status":"completed","location":"home"}}}`, TurnIndex: 12, SourceTurn: 12, LastVerifiedTurn: 12, Confidence: 0.9},
	}
	assembly := buildPrepareTurnInjectionAssembly(
		nil, nil, nil, nil, nil, nil, nil, nil, canonical, nil, nil, nil, nil,
		5, 60000, "Check the home garden rapeseed status.", "default", nil,
		map[string]any{"memory_search_result": "not_found", "search_result": "not_found"}, nil,
		priorityMemoryTestContext(8),
	)
	finalText := extractionStringFromAny(assembly.MemoryDeliveryPlan["final_text"])
	if !strings.Contains(finalText, "status: completed") || strings.Contains(finalText, "status: planned") {
		t.Fatalf("typed source provenance prevented current-field resolution: %q", finalText)
	}
	if strings.Count(finalText, "location: home") != 1 {
		t.Fatalf("unchanged typed field consumed duplicate K slots: %q", finalText)
	}
}

func Test42PriorityMemoryReviewedAliasesShareCharacterStateIdentity(t *testing.T) {
	out := &prepareTurnInjectionAssembly{
		CharacterObjectiveText: "[Character Objective States]\n- Mask: state={\"mood\":\"ready\"}\n- Mina: state={\"mood\":\"ready\"}",
		PriorityEntityAliases:  map[string]any{"Mask": "Mina", "Mina": "Mina"},
		Counts:                 map[string]any{},
	}
	appendPrepareTurnPrioritySourceMetadata(out, "character_states", `- Mask: state={"mood":"ready"}`, "character_states:81:objective", int64(81), 20, 0, false)
	appendPrepareTurnPrioritySourceMetadata(out, "character_states", `- Mina: state={"mood":"ready"}`, "character_states:82:objective", int64(82), 21, 0, false)
	plan := buildPrepareTurnMemoryDeliveryPlan(out, 4000, map[string]any{
		"_priority_memory_enabled": true, "_priority_memory_max_items": 4,
		"_priority_memory_query": "Mina mood ready",
	})
	if intFromAny(plan["priority_candidate_count"], 0) != 2 || intFromAny(plan["priority_resolved_count"], 0) != 1 {
		t.Fatalf("reviewed aliases did not share one character-state identity: %#v", plan["priority_items"])
	}
	if strings.Count(extractionStringFromAny(plan["final_text"]), "mood: ready") != 1 {
		t.Fatalf("alias duplicate consumed more than one K slot: %q", plan["final_text"])
	}
}

func Test42PriorityMemoryUsesStoredOccurrenceIdentityForNaturalLanguageLifecycleResolution(t *testing.T) {
	out := &prepareTurnInjectionAssembly{
		ActualMemoryText: "- Han-eol planned the rapeseed planting.\n- Han-eol completed the rapeseed planting.",
		MemoryDeliveryLineage: map[string]any{"items": []map[string]any{
			{"source_row_id": 20, "source_occurrence_key": "event:rapeseed-home", "turn_index": 20, "final_text": "Han-eol planned the rapeseed planting.", "importance_score": 8.0, "selection_score": 0.8},
			{"source_row_id": 22, "source_occurrence_key": "event:rapeseed-home", "turn_index": 22, "final_text": "Han-eol completed the rapeseed planting.", "importance_score": 8.0, "selection_score": 0.8},
		}},
		Counts: map[string]any{},
	}
	plan := buildPrepareTurnMemoryDeliveryPlan(out, 2000, map[string]any{
		"_priority_memory_enabled": true, "_priority_memory_max_items": 4,
		"_priority_memory_query": "rapeseed planting",
	})
	finalText := extractionStringFromAny(plan["final_text"])
	if !strings.Contains(finalText, "completed") || strings.Contains(finalText, "planned") {
		t.Fatalf("stored occurrence lifecycle did not resolve to its current expression: %q", finalText)
	}
	if intFromAny(plan["priority_candidate_count"], 0) != 2 || intFromAny(plan["priority_resolved_count"], 0) != 1 {
		t.Fatalf("occurrence identity resolution trace mismatch: %#v", plan)
	}
}

func Test42PriorityMemoryTextAndPDFConsumeSameFinalSelection(t *testing.T) {
	out := &prepareTurnInjectionAssembly{
		ActualMemoryText: "[Memory]\n- [relevant, turn 8] Mira already sealed the archive door.",
		MemoryDeliveryLineage: map[string]any{"items": []map[string]any{{
			"source_row_id": 8, "turn_index": 8, "final_text": "Mira already sealed the archive door.",
			"importance_score": 9.0, "selection_score": 0.8, "delivered": true,
		}}},
		Counts: map[string]any{},
	}
	out.MemoryDeliveryPlan = buildPrepareTurnMemoryDeliveryPlan(out, 6000, map[string]any{
		"_priority_memory_enabled": true, "_priority_memory_max_items": 1,
		"_priority_memory_query": "Mira archive door",
	})
	payloadPlan := map[string]any{
		"auxiliary_text": extractionStringFromAny(out.MemoryDeliveryPlan["final_text"]),
		"lanes": []map[string]any{{
			"key": "long_term_memory", "applied": true,
			"text": extractionStringFromAny(out.MemoryDeliveryPlan["final_text"]),
		}},
	}
	textPlan, _ := buildPrepareTurnMemoryTransport("text", payloadPlan, "req-priority", nil)
	generatePDF := func(text string) (doc pdfmemory.Document, err error) {
		return pdfmemory.Document{Bytes: []byte("pdf:" + text), PageCount: 1}, nil
	}
	if textPlan["logical_text_hash"] != out.MemoryDeliveryPlan["final_text_sha256"] &&
		extractionStringFromAny(textPlan["logical_text_hash"]) != prepareTurnTextHash(extractionStringFromAny(out.MemoryDeliveryPlan["final_text"])) {
		t.Fatalf("text transport did not consume priority final text: plan=%#v delivery=%#v", textPlan, out.MemoryDeliveryPlan)
	}
	for _, mode := range []string{"google_pdf", "llm_gateway_pdf", "provider_manager_pdf"} {
		transportPlan, _ := buildPrepareTurnMemoryTransport(mode, payloadPlan, "req-priority", generatePDF)
		if transportPlan["logical_text_hash"] != textPlan["logical_text_hash"] || transportPlan["logical_memory_chars"] != textPlan["logical_memory_chars"] {
			t.Fatalf("text/%s selection drift: text=%#v transport=%#v", mode, textPlan, transportPlan)
		}
	}
}

func Test42TurnFinalizationPolicyPreservesImmediateDefaultAndAllowsNextInputPipeline(t *testing.T) {
	immediate := buildPrepareTurnFinalizationPolicy("")
	if immediate["mode"] != prepareTurnFinalizationImmediate || immediate["current_generation_blocked"] != false {
		t.Fatalf("immediate default changed: %#v", immediate)
	}
	next := buildPrepareTurnFinalizationPolicy(prepareTurnFinalizationNextInput)
	if next["mode"] != prepareTurnFinalizationNextInput ||
		next["previous_turn_critic"] != "pipeline_with_current_generation" ||
		next["confirmed_memory_horizon"] != "through_previous_confirmed_turn" ||
		next["current_generation_blocked"] != false {
		t.Fatalf("next-input pipeline policy mismatch: %#v", next)
	}
	unknown := buildPrepareTurnFinalizationPolicy("unknown")
	if unknown["mode"] != prepareTurnFinalizationImmediate {
		t.Fatalf("unknown finalization mode must preserve the 4.1 default: %#v", unknown)
	}
}

func Test42PriorityMemoryDoesNotRepeatAuthorityOrLetOneOversizedFactBlockOtherTopKFacts(t *testing.T) {
	out := &prepareTurnInjectionAssembly{
		LatestDirectEvidenceText: "- Mira already locked the archive door.",
		ActualMemoryText: strings.Join([]string{
			"- Mira already locked the archive door.",
			"- " + strings.Repeat("oversized ", 80),
			"- Mira kept the brass key.",
			"- Unrelated low-ranked filler.",
		}, "\n"),
		MemoryDeliveryLineage: map[string]any{"items": []map[string]any{
			{"source_row_id": 1, "final_text": "Mira already locked the archive door.", "importance_score": 10.0, "selection_score": 1.0},
			{"source_row_id": 2, "final_text": strings.Repeat("oversized ", 80), "importance_score": 9.0, "selection_score": 0.9},
			{"source_row_id": 3, "final_text": "Mira kept the brass key.", "importance_score": 8.0, "selection_score": 0.8},
			{"source_row_id": 4, "final_text": "Unrelated low-ranked filler.", "importance_score": 1.0, "selection_score": 0.1},
		}},
		Counts: map[string]any{},
	}
	plan := buildPrepareTurnMemoryDeliveryPlan(out, 180, map[string]any{
		"_priority_memory_enabled": true, "_priority_memory_max_items": 2,
		"_priority_memory_query": "Mira archive brass key",
	})
	finalText := extractionStringFromAny(plan["final_text"])
	if strings.Count(finalText, "Mira already locked the archive door.") != 1 {
		t.Fatalf("authority fact was duplicated: %q", finalText)
	}
	if !strings.Contains(finalText, "Mira kept the brass key.") {
		t.Fatalf("one oversized top-K fact blocked another top-K fact: %q", finalText)
	}
	if strings.Contains(finalText, "Unrelated low-ranked filler.") {
		t.Fatalf("rank below global K backfilled the remaining character budget: %q", finalText)
	}
}
