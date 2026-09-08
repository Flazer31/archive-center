package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/pdfmemory"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
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
		t.Fatalf("high-importance completion fact did not reach final payload text: %q items=%#v", finalText, plan["priority_items"])
	}
	if !strings.Contains(finalText, "casual conversation") || strings.Contains(finalText, "unrelated harbor") {
		t.Fatalf("independent summary/fact K did not keep the next related fact ahead of unrelated memory: %q", finalText)
	}
	if intFromAny(plan["turn_summary_selected_count"], 0) != 2 || intFromAny(plan["priority_fact_selected_count"], 0) != 0 || !boolFromAny(plan["low_score_backfill_after_k"]) {
		t.Fatalf("core target / remaining character budget contract mismatch: %#v", plan)
	}

	foundStoredImportance := false
	for _, raw := range prepareTurnMemoryLineageSlice(plan["priority_items"]) {
		item := mapFromAny(raw)
		if strings.Contains(extractionStringFromAny(item["complete_text"]), "already planted rapeseed") {
			foundStoredImportance = extractionFloatFromAny(item["importance_score"], 0) == 0.9 &&
				extractionStringFromAny(item["selection_status"]) == "deferred" &&
				extractionStringFromAny(item["selection_reason"]) == "turn_summary_exact_duplicate" &&
				extractionFloatFromAny(item["final_score"], 0) > 0
		}
	}
	if !foundStoredImportance {
		t.Fatalf("stored importance was not preserved on the selected fact: %#v", plan["priority_items"])
	}
}

func Test42PriorityMemorySelectsCompleteTurnSummaryByHighestChildFactScore(t *testing.T) {
	const summary = "Mira already sealed the archive door. A harbor vendor rearranged empty baskets."
	memories := []store.Memory{{
		ID: 81, ChatSessionID: "turn-summary-score", TurnIndex: 40, Importance: 8,
		SummaryJSON: `{"turn_summary":"` + summary + `","narrative_events":[{"event":"Mira already sealed the archive door.","visibility":"public"},{"event":"A harbor vendor rearranged empty baskets.","visibility":"public"}]}`,
	}}
	assembly := buildPrepareTurnInjectionAssembly(
		memories, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		5, 60000, "Mira checks the sealed archive door.", "default", nil,
		map[string]any{"memory_search_result": "not_found", "search_result": "not_found"}, nil,
		priorityMemoryTestContext(1),
	)
	plan := assembly.MemoryDeliveryPlan
	if intFromAny(plan["turn_summary_candidate_count"], 0) != 1 || intFromAny(plan["turn_summary_selected_count"], 0) != 1 {
		t.Fatalf("complete turn summary did not receive its independent K: %#v", plan)
	}
	if !strings.Contains(extractionStringFromAny(plan["final_text"]), summary) {
		t.Fatalf("selected turn was not rendered as the complete stored turn_summary: %q", plan["final_text"])
	}
	summaries := prepareTurnMemoryLineageSlice(plan["turn_summary_items"])
	if len(summaries) != 1 {
		t.Fatalf("turn-summary trace mismatch: %#v", summaries)
	}
	maxChildScore := 0.0
	for _, raw := range prepareTurnMemoryLineageSlice(plan["priority_items"]) {
		item := mapFromAny(raw)
		if score := extractionFloatFromAny(item["final_score"], 0); score > maxChildScore {
			maxChildScore = score
		}
	}
	if extractionFloatFromAny(mapFromAny(summaries[0])["final_score"], -1) != maxChildScore {
		t.Fatalf("turn summary did not inherit the highest child fact score: summary=%#v facts=%#v", summaries, plan["priority_items"])
	}
}

func Test43PriorityMemoryUsesIndependentCoreTargetPerFactLane(t *testing.T) {
	out := &prepareTurnInjectionAssembly{
		CharacterObjectiveText: "[Character Objective States]\n- Mira guards the sealed archive door.\n- Rook catalogs a distant observatory.",
		CanonWorldText:         "[Item, Location, and World States]\n- archive_door status: sealed\n- desert_observatory status: mapped",
		Counts:                 map[string]any{},
	}
	plan := buildPrepareTurnMemoryDeliveryPlan(out, 6000, map[string]any{
		"_priority_memory_enabled": true, "_priority_memory_max_items": 1,
		"_priority_memory_query": "Mira checks the sealed archive door.",
	})
	finalText := extractionStringFromAny(plan["final_text"])
	if !strings.Contains(finalText, "Mira guards") || !strings.Contains(finalText, "archive_door status: sealed") {
		t.Fatalf("one global K still prevented independent fact lanes from contributing: %q", finalText)
	}
	if intFromAny(plan["priority_fact_selected_count"], 0) != 4 {
		t.Fatalf("core target prevented remaining details from using space: %#v", plan["core_objective_memory"])
	}
	if boolFromAny(plan["unused_k_transfer_between_groups"]) {
		t.Fatalf("unused K was transferable between groups: %#v", plan)
	}
}

func Test42PriorityMemoryCustomBudgetsStayOnPriorityPathAndCapEachLane(t *testing.T) {
	out := &prepareTurnInjectionAssembly{
		CharacterObjectiveText: "[Character Objective States]\n- Mira guards the sealed archive door with an especially long ceremonial description.",
		CanonWorldText:         "[Item, Location, and World States]\n- archive_door status: sealed",
		Counts:                 map[string]any{},
	}
	plan := buildPrepareTurnMemoryDeliveryPlan(out, 2000, map[string]any{
		"_priority_memory_enabled": true, "_priority_memory_max_items": 1,
		"_priority_memory_query":       "Mira checks the sealed archive door.",
		"_memory_delivery_budget_mode": "custom",
		"_memory_delivery_budgets":     map[string]int{"character_objective": 40, "world_state": 500},
	})
	if plan["contract_version"] != prepareTurnPriorityMemoryPlanVersion || plan["mode"] != "custom" {
		t.Fatalf("custom budgets bypassed the Priority plan: %#v", plan)
	}
	finalText := extractionStringFromAny(plan["final_text"])
	if strings.Contains(finalText, "ceremonial description") || !strings.Contains(finalText, "archive_door status: sealed") {
		t.Fatalf("per-lane UI character budgets were not applied independently: %q classes=%#v", finalText, plan["classes"])
	}
	reasons, _ := plan["exclusion_reasons"].(map[string]int)
	if reasons["memory_lane_char_budget_reached"] == 0 {
		t.Fatalf("lane budget deferral was not diagnosed: %#v", plan)
	}
}

func Test42PriorityMemoryTurnSummaryAndEventFactsShareEventBudget(t *testing.T) {
	const summary = "Mira sealed the archive door. The brass key remained in her sleeve."
	capChars := len([]rune("[Event and Recent Memories]\n- [turn 40] " + summary))
	memories := []store.Memory{{
		ID: 82, ChatSessionID: "turn-summary-budget", TurnIndex: 40, Importance: 8,
		SummaryJSON: `{"turn_summary":"` + summary + `","narrative_events":[{"event":"Mira sealed the archive door.","visibility":"public"}]}`,
	}}
	assembly := buildPrepareTurnInjectionAssemblyWithBudget(
		memories, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		5, 6000, "Mira checks the sealed archive door.", "default", nil,
		map[string]any{"memory_search_result": "not_found", "search_result": "not_found"}, nil,
		"custom", map[string]int{"event_recent": capChars}, priorityMemoryTestContext(2),
	)
	plan := assembly.MemoryDeliveryPlan
	if intFromAny(plan["turn_summary_selected_count"], 0) != 1 || intFromAny(plan["priority_fact_selected_count"], 0) != 0 {
		t.Fatalf("turn summary and event facts did not share the existing event_recent character budget: %#v", plan)
	}
	if !strings.Contains(extractionStringFromAny(plan["final_text"]), summary) {
		t.Fatalf("complete turn summary was not retained within the shared event budget: %q", plan["final_text"])
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
	if !strings.Contains(finalText, "must return") || !strings.Contains(finalText, "may polish") || strings.Index(finalText, "must return") >= strings.Index(finalText, "may polish") {
		t.Fatalf("stored pending-thread priority did not order the core before the additional detail: %q", finalText)
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

func Test42PriorityMemoryArrayOrdinalsDoNotMergeUnrelatedCanonicalRules(t *testing.T) {
	const disguiseRule = "Bae Sang-mun's expelled foundry-master persona is a deliberate disguise."
	const technologyRule = "Haneul's inventions must remain within contemporary Joseon workshop technology."
	canonical := []store.CanonicalStateLayer{
		{ID: 81, ChatSessionID: "typed-array-rules", LayerType: "world_state", Content: `{"rules":[{"scope_name":"Bae Sang-mun disguise","rule":"` + disguiseRule + `"}]}`, TurnIndex: 68, SourceTurn: 68, LastVerifiedTurn: 68, Confidence: 0.9},
		{ID: 82, ChatSessionID: "typed-array-rules", LayerType: "world_state", Content: `{"rules":[{"scope_name":"Joseon technology boundary","rule":"` + technologyRule + `"}]}`, TurnIndex: 104, SourceTurn: 104, LastVerifiedTurn: 104, Confidence: 0.9},
	}
	assembly := buildPrepareTurnInjectionAssembly(
		nil, nil, nil, nil, nil, nil, nil, nil, canonical, nil, nil, nil, nil,
		5, 60000, "Recall both the Bae Sang-mun disguise and Joseon technology boundary.", "default", nil,
		map[string]any{"memory_search_result": "not_found", "search_result": "not_found"}, nil,
		priorityMemoryTestContext(8),
	)
	plan := assembly.MemoryDeliveryPlan
	finalText := extractionStringFromAny(plan["final_text"])
	if !strings.Contains(finalText, disguiseRule) || !strings.Contains(finalText, technologyRule) {
		t.Fatalf("unrelated array rules were merged by ordinal path: %q items=%#v", finalText, plan["priority_items"])
	}

	canonicalIDs := map[string]string{}
	for _, raw := range prepareTurnMemoryLineageSlice(plan["priority_items"]) {
		item := mapFromAny(raw)
		text := extractionStringFromAny(item["complete_text"])
		if text != "world_state · rules · item_1 · rule: "+disguiseRule && text != "world_state · rules · item_1 · rule: "+technologyRule {
			continue
		}
		if extractionStringFromAny(item["selection_reason"]) == "canonical_current_resolution" {
			t.Fatalf("unrelated array rule was superseded as the same canonical fact: %#v", item)
		}
		canonicalIDs[text] = extractionStringFromAny(item["canonical_fact_id"])
	}
	if len(canonicalIDs) != 2 {
		t.Fatalf("array rule lineage missing: %#v", plan["priority_items"])
	}
	ids := []string{}
	for _, id := range canonicalIDs {
		ids = append(ids, id)
	}
	if ids[0] == "" || ids[0] == ids[1] {
		t.Fatalf("unrelated array rules received the same canonical fact ID: %#v", canonicalIDs)
	}
}

func Test42PriorityMemoryReviewedAliasesShareCharacterStateIdentity(t *testing.T) {
	out := &prepareTurnInjectionAssembly{
		CharacterObjectiveText: "[Character Objective States]\n- Mask: state={\"mood\":\"ready\"}\n- Mina: state={\"mood\":\"ready\"}",
		PriorityEntityAliases:  map[string]any{"Mask": "Mina", "Mina": "Mina"},
		Counts:                 map[string]any{},
	}
	appendPrepareTurnPrioritySourceMetadata(out, "character_objective", "character_states", "required", `- Mask: state={"mood":"ready"}`, "character_states:81:objective", int64(81), 20, 0, false, "general", "", nil)
	appendPrepareTurnPrioritySourceMetadata(out, "character_objective", "character_states", "required", `- Mina: state={"mood":"ready"}`, "character_states:82:objective", int64(82), 21, 0, false, "general", "", nil)
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

func Test42PriorityMemoryKeepsChangedNaturalLanguageFactsUnderOneOccurrence(t *testing.T) {
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
	if !strings.Contains(finalText, "completed") || !strings.Contains(finalText, "planned") {
		t.Fatalf("changed facts under one occurrence were not kept as independent score candidates: %q", finalText)
	}
	if intFromAny(plan["priority_candidate_count"], 0) != 4 || intFromAny(plan["priority_resolved_count"], 0) != 2 ||
		intFromAny(plan["turn_summary_candidate_count"], 0) != 2 {
		t.Fatalf("occurrence fact trace mismatch: %#v", plan)
	}
}

func Test42PriorityMemoryKeepsStructuredLifecycleStagesAsScoredFacts(t *testing.T) {
	const lifecycleKey = "park-dojun-loan-settlement"
	memories := []store.Memory{
		{ID: 201, ChatSessionID: "structured-lifecycle", TurnIndex: 8, Importance: 0.8, SummaryJSON: mustCompactJSON(map[string]any{
			"turn_summary": "The Park Dojun loan remained outstanding.",
			"state_claims": []any{map[string]any{
				"subject": "Park Dojun loan", "state_slot": "debt_status", "lifecycle_key": lifecycleKey,
				"value": "outstanding", "transition": "set", "evidence_excerpt": "The loan remained outstanding.",
			}},
		})},
		{ID: 202, ChatSessionID: "structured-lifecycle", TurnIndex: 12, Importance: 0.8, SummaryJSON: mustCompactJSON(map[string]any{
			"turn_summary": "The liquor settled the debt and the note was burned.",
			"state_claims": []any{map[string]any{
				"subject": "Burned loan note", "state_slot": "document_status", "lifecycle_key": lifecycleKey,
				"value": "settled and burned", "transition": "resolve", "evidence_excerpt": "The debt was settled.",
			}},
		})},
	}
	out := &prepareTurnInjectionAssembly{Counts: map[string]any{}}
	for _, memory := range memories {
		facts, projectionSource := prepareTurnPriorityFactsFromMemory(memory)
		appendPrepareTurnPriorityFactSeeds(out, prepareTurnPrioritySourceMetadata{
			Lane: "event_recent", SourceTable: "memories", Tier: "required",
			SourceRowID: memory.ID, SourceOccurrence: fmt.Sprintf("memory-row:%d", memory.ID),
			SourceTurn: memory.TurnIndex, Importance: memory.Importance, ImportancePresent: true,
			Visibility: "public_projection",
		}, prepareTurnMemorySummary(memory), facts, projectionSource)
	}
	plan := buildPrepareTurnMemoryDeliveryPlan(out, 60000, map[string]any{
		"_priority_memory_enabled": true, "_priority_memory_max_items": 5,
		"_priority_memory_query": "What is the current status of Park Dojun's loan?",
	})
	finalText := extractionStringFromAny(plan["final_text"])
	if !strings.Contains(finalText, "settled and burned") || !strings.Contains(finalText, "outstanding") {
		t.Fatalf("structured lifecycle stages were not preserved for score competition: %q items=%#v", finalText, plan["priority_items"])
	}
	if intFromAny(plan["priority_candidate_count"], 0) != 4 || intFromAny(plan["priority_resolved_count"], 0) != 2 ||
		intFromAny(plan["turn_summary_selected_count"], 0) != 2 || intFromAny(plan["priority_fact_selected_count"], 0) != 2 {
		t.Fatalf("structured lifecycle candidate counts mismatch: %#v", plan)
	}
	items := prepareTurnMemoryLineageSlice(plan["priority_items"])
	transitions := map[string]bool{}
	for _, raw := range items {
		item := mapFromAny(raw)
		if item["lifecycle_key"] == lifecycleKey {
			transitions[extractionStringFromAny(item["lifecycle_transition"])] = true
		}
	}
	if !transitions["set"] || !transitions["resolve"] {
		t.Fatalf("lifecycle lineage did not retain both stages: %#v", items)
	}
}

func Test42PriorityMemoryLifecycleMetadataCannotPreemptFinalScore(t *testing.T) {
	const lifecycleKey = "archive-repair"
	out := &prepareTurnInjectionAssembly{Counts: map[string]any{}}
	out.PriorityFactSeeds = []prepareTurnPriorityFactSeed{
		{
			Lane: "event_recent", SourceTable: "memories", Tier: "required", SourceTurn: 10,
			Importance: 0.5, ImportancePresent: true,
			SemanticSimilarity: 0.20, SemanticSimilarityObserved: true,
			Fact: prepareTurnPriorityMemoryFact{
				Text: "The archive repair was completed long ago.", LifecycleKey: lifecycleKey,
				LifecycleTransition: "complete", Structured: true,
			},
		},
		{
			Lane: "event_recent", SourceTable: "memories", Tier: "required", SourceTurn: 99,
			Importance: 0.5, ImportancePresent: true,
			SemanticSimilarity: 0.95, SemanticSimilarityObserved: true,
			Fact: prepareTurnPriorityMemoryFact{
				Text: "The archive repair resumed in the current scene.", LifecycleKey: lifecycleKey,
				LifecycleTransition: "resume", Structured: true,
			},
		},
	}

	plan := buildPrepareTurnMemoryDeliveryPlan(out, 4000, map[string]any{
		"_priority_memory_enabled": true, "_priority_memory_max_items": 1,
		"_priority_memory_query":        "The archive repair resumed in the current scene.",
		"_priority_memory_current_turn": 100,
	})
	finalText := extractionStringFromAny(plan["final_text"])
	if !strings.Contains(finalText, "resumed in the current scene") || !strings.Contains(finalText, "completed long ago") || strings.Index(finalText, "resumed in the current scene") >= strings.Index(finalText, "completed long ago") {
		t.Fatalf("lifecycle rank still overrode the higher final score: %q items=%#v", finalText, plan["priority_items"])
	}
	items := prepareTurnMemoryLineageSlice(plan["priority_items"])
	if len(items) != 2 {
		t.Fatalf("lifecycle candidate trace mismatch: %#v", items)
	}
	selected := mapFromAny(items[0])
	if extractionStringFromAny(selected["selection_status"]) != "selected" ||
		extractionStringFromAny(selected["lifecycle_transition"]) != "resume" {
		t.Fatalf("higher-scoring current fact was not selected: %#v", items)
	}
	deferred := mapFromAny(items[1])
	if extractionStringFromAny(deferred["selection_status"]) != "selected" ||
		extractionStringFromAny(deferred["lifecycle_transition"]) != "complete" {
		t.Fatalf("older lifecycle fact was not retained after the higher-scoring core fact: %#v", items)
	}
}

func Test42PriorityMemoryDoesNotInferLifecycleRankFromWording(t *testing.T) {
	completed := prepareTurnPriorityContinuityBonus("event_recent", "memories", "The work was already completed.")
	plain := prepareTurnPriorityContinuityBonus("event_recent", "memories", "The work continued.")
	if completed != plain {
		t.Fatalf("completion-like wording created hidden lifecycle rank: completed=%v plain=%v", completed, plain)
	}
}

func Test42PriorityMemoryLifecycleTransitionDoesNotChangeScore(t *testing.T) {
	out := &prepareTurnInjectionAssembly{Counts: map[string]any{}}
	out.PriorityFactSeeds = []prepareTurnPriorityFactSeed{
		{
			Lane: "event_recent", SourceTable: "memories", Tier: "required", SourceTurn: 50,
			Importance: 0.8, ImportancePresent: true, SemanticSimilarity: 0.7, SemanticSimilarityObserved: true,
			Fact: prepareTurnPriorityMemoryFact{Text: "The work remains active.", FamilyKey: "active", ValueKey: "active", LifecycleKey: "same-work", LifecycleTransition: "set", Structured: true},
		},
		{
			Lane: "event_recent", SourceTable: "memories", Tier: "required", SourceTurn: 50,
			Importance: 0.8, ImportancePresent: true, SemanticSimilarity: 0.7, SemanticSimilarityObserved: true,
			Fact: prepareTurnPriorityMemoryFact{Text: "The work is complete.", FamilyKey: "complete", ValueKey: "complete", LifecycleKey: "same-work", LifecycleTransition: "complete", Structured: true},
		},
	}
	plan := buildPrepareTurnMemoryDeliveryPlan(out, 4000, map[string]any{
		"_priority_memory_enabled": true, "_priority_memory_max_items": 2,
		"_priority_memory_query": "work", "_priority_memory_current_turn": 51,
	})
	items := prepareTurnMemoryLineageSlice(plan["priority_items"])
	if len(items) != 2 {
		t.Fatalf("lifecycle diagnostic candidates missing: %#v", items)
	}
	first, second := mapFromAny(items[0]), mapFromAny(items[1])
	if extractionFloatFromAny(first["final_score"], -1) != extractionFloatFromAny(second["final_score"], -2) {
		t.Fatalf("AI lifecycle transition changed final score: %#v", items)
	}
	if plan["lifecycle_ranking_policy"] != "diagnostic_only_not_scored" {
		t.Fatalf("lifecycle ranking policy mismatch: %#v", plan)
	}
}

func Test43PriorityMemoryTurnDistancePreservesImportance(t *testing.T) {
	out := &prepareTurnInjectionAssembly{Counts: map[string]any{}}
	out.PriorityFactSeeds = []prepareTurnPriorityFactSeed{
		{
			Lane: "event_recent", SourceTable: "memories", Tier: "required", SourceTurn: 99,
			Importance: 0.6, ImportancePresent: true, SemanticSimilarity: 0.7, SemanticSimilarityObserved: true,
			Fact: prepareTurnPriorityMemoryFact{Text: "A recent relevant event.", FamilyKey: "recent", ValueKey: "recent", Structured: true},
		},
		{
			Lane: "event_recent", SourceTable: "memories", Tier: "required", SourceTurn: 1,
			Importance: 0.6, ImportancePresent: true, SemanticSimilarity: 0.7, SemanticSimilarityObserved: true,
			Fact: prepareTurnPriorityMemoryFact{Text: "An old relevant event.", FamilyKey: "old", ValueKey: "old", Structured: true},
		},
	}
	plan := buildPrepareTurnMemoryDeliveryPlan(out, 4000, map[string]any{
		"_priority_memory_enabled": true, "_priority_memory_max_items": 2,
		"_priority_memory_query": "relevant event", "_priority_memory_current_turn": 100,
	})
	items := prepareTurnMemoryLineageSlice(plan["priority_items"])
	if len(items) != 2 {
		t.Fatalf("turn-decay candidates missing: %#v", items)
	}
	recent, old := mapFromAny(items[0]), mapFromAny(items[1])
	if !strings.Contains(extractionStringFromAny(recent["complete_text"]), "recent") ||
		extractionFloatFromAny(recent["final_score"], 0) <= extractionFloatFromAny(old["final_score"], 0) {
		t.Fatalf("separate recency contribution did not favor the otherwise equal recent fact: %#v", items)
	}
	for _, item := range []map[string]any{recent, old} {
		if extractionFloatFromAny(item["importance_after_turn_decay"], -1) != out.PriorityFactSeeds[0].Importance {
			t.Fatalf("age changed the importance contribution: %#v", item)
		}
	}
	if plan["importance_decay_policy"] != "stored_importance_preserved_recency_separate" {
		t.Fatalf("importance decay policy mismatch: %#v", plan)
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

func Test42PriorityMemoryScoresAtomicSentencesWithoutParentScoreLeak(t *testing.T) {
	memories := []store.Memory{{
		ID: 91, ChatSessionID: "atomic-parent-score", TurnIndex: 40, Importance: 8,
		SummaryJSON: `{"turn_summary":"Mira already sealed the archive door. An unrelated harbor vendor rearranged empty baskets.","narrative_events":[{"event":"Mira already sealed the archive door.","visibility":"public"},{"event":"An unrelated harbor vendor rearranged empty baskets.","visibility":"public"}]}`,
	}}
	assembly := buildPrepareTurnInjectionAssembly(
		memories, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		5, 60000, "Mira checks the sealed archive door.", "default", nil,
		map[string]any{"memory_search_result": "not_found", "search_result": "not_found"}, nil,
		priorityMemoryTestContext(1),
	)
	plan := assembly.MemoryDeliveryPlan
	if intFromAny(plan["priority_candidate_count"], 0) != 3 || intFromAny(plan["priority_fact_selected_count"], 0) != 2 ||
		intFromAny(plan["turn_summary_selected_count"], 0) != 1 {
		t.Fatalf("one stored row was not projected as one complete summary plus two atomic facts: %#v", plan)
	}
	if intFromAny(plan["candidate_chars"], 0) <= 0 || plan["candidate_unit"] != "complete_turn_summary+atomic_fact" {
		t.Fatalf("atomic candidate diagnostics were not populated: %#v", plan)
	}
	if intFromAny(plan["candidate_count"], 0) < intFromAny(plan["selected_count"], 0) ||
		intFromAny(plan["candidate_chars"], 0) < intFromAny(plan["selected_chars"], 0) {
		t.Fatalf("candidate diagnostics contradict the selected fact payload: %#v", plan)
	}
	finalText := extractionStringFromAny(plan["final_text"])
	if !strings.Contains(finalText, "Mira already sealed the archive door. An unrelated harbor vendor rearranged empty baskets.") {
		t.Fatalf("the selected turn was not delivered as a complete turn summary: %q items=%#v", finalText, plan["priority_items"])
	}
	relevantScore := -1.0
	unrelatedScore := -1.0
	for _, raw := range prepareTurnMemoryLineageSlice(plan["priority_items"]) {
		item := mapFromAny(raw)
		text := extractionStringFromAny(item["complete_text"])
		if strings.Contains(text, "sealed the archive door") {
			relevantScore = extractionFloatFromAny(item["relevance_score"], -1)
			if item["visibility"] != "public" || item["projection_source"] != "memory_public_projection:structured" {
				t.Fatalf("structured fact lost its source visibility or projection lineage: %#v", item)
			}
		}
		if strings.Contains(text, "harbor vendor") {
			unrelatedScore = extractionFloatFromAny(item["relevance_score"], -1)
			lineage := mapFromAny(item["score_lineage"])
			if boolFromAny(lineage["source_selection_score_used_as_fact_relevance"]) {
				t.Fatalf("parent selection score leaked into an atomic child: %#v", item)
			}
		}
	}
	if relevantScore <= unrelatedScore || unrelatedScore < 0 {
		t.Fatalf("atomic relevance did not separate related and unrelated sibling facts: related=%v unrelated=%v items=%#v", relevantScore, unrelatedScore, plan["priority_items"])
	}
}

func Test42PriorityMemoryCarriesVisibilityAndPerspectivePerFactWithoutNewGate(t *testing.T) {
	private := []store.ProtagonistEntityMemory{{
		ID: 73, OwnerEntityKey: "mira", OwnerEntityName: "Mira", OwnerVisibility: "owner_private",
		SourceTurn: 30, Importance10: 9,
		MemoryText: "Mira remembers the hidden promise. Mira remains wary of the northern door.",
	}}
	assembly := buildPrepareTurnInjectionAssembly(
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, private,
		5, 60000, "Mira approaches the northern door.", "default", nil,
		map[string]any{"memory_search_result": "not_found", "search_result": "not_found"}, nil,
		priorityMemoryTestContext(4),
	)
	found := 0
	for _, raw := range prepareTurnMemoryLineageSlice(assembly.MemoryDeliveryPlan["priority_items"]) {
		item := mapFromAny(raw)
		if extractionStringFromAny(item["source_table"]) != "protagonist_entity_memories" {
			continue
		}
		found++
		if item["visibility"] != "owner_private" || item["perspective_owner"] != "Mira" {
			t.Fatalf("private fact lost source visibility or perspective owner: %#v", item)
		}
		viewers := stringSliceFromAny(item["allowed_viewers"])
		if len(viewers) != 1 || viewers[0] != "Mira" {
			t.Fatalf("private fact viewer scope drifted: %#v", item)
		}
	}
	if found != 2 {
		t.Fatalf("private source sentences were not independently readable: found=%d items=%#v", found, assembly.MemoryDeliveryPlan["priority_items"])
	}
	if assembly.MemoryDeliveryPlan["visibility_handling"] != "existing_source_scope_carried_per_fact_without_new_rejection_gate" {
		t.Fatalf("visibility handling did not preserve the no-new-gate contract: %#v", assembly.MemoryDeliveryPlan)
	}
}

func Test42PriorityMemoryKeepsUnseededSourceOnLegacySentenceFallback(t *testing.T) {
	out := &prepareTurnInjectionAssembly{
		ChapterText: "[Chapter Recall]\n- Mira sealed the archive door. Rowan kept the brass key.",
		Counts:      map[string]any{},
	}
	plan := buildPrepareTurnMemoryDeliveryPlan(out, 6000, map[string]any{
		"_priority_memory_enabled": true, "_priority_memory_max_items": 2,
		"_priority_memory_query": "Mira Rowan archive brass key",
	})
	if intFromAny(plan["priority_candidate_count"], 0) != 2 || intFromAny(plan["priority_selected_count"], 0) != 2 {
		t.Fatalf("legacy source did not remain available as atomic sentence fallback: %#v", plan)
	}
	for _, raw := range prepareTurnMemoryLineageSlice(plan["priority_items"]) {
		if mapFromAny(raw)["projection_source"] != "legacy_rendered_line" {
			t.Fatalf("unseeded compatibility source was not identified as legacy fallback: %#v", raw)
		}
	}
}

func Test42PriorityMemoryKeepsTrailingSourceMetadataWithItsFact(t *testing.T) {
	line := "- [Directional Interaction Evidence] relationship | Alice -> Bob | domain=respect | Alice explicitly respects Bob. | source_turn=2"
	facts := prepareTurnPrioritySplitFact(line)
	if len(facts) != 1 {
		t.Fatalf("fact count=%d facts=%#v", len(facts), facts)
	}
	want := prepareTurnPriorityCleanLine(line)
	if facts[0].Text != want {
		t.Fatalf("fact=%q want=%q", facts[0].Text, want)
	}
}

func Test42PriorityMemoryKeepsProtectedRecollectionGuardWithItsMemoryFact(t *testing.T) {
	line := "- Protected hint: 시우는 잠긴 책상 안의 은색 로켓을 기억한다. | protected private knowledge is present; use only as owner subtext, hesitation, avoidance, or careful choice; do not reveal content without current evidence | kind=full_loop_memory"
	facts := prepareTurnPrioritySplitFact(line)
	if len(facts) != 1 || facts[0].Text != prepareTurnPriorityCleanLine(line) {
		t.Fatalf("protected recollection guard split away from its memory: %#v", facts)
	}
}

func Test42PriorityMemoryRecognizesOneRuneNonASCIIInflectionWithoutEnglishPrefixMatch(t *testing.T) {
	if got := prepareTurnPriorityRelevance("책상이 이상하게 낯익다.", "시우는 잠긴 책상 안의 은색 로켓을 기억한다."); got <= 0 {
		t.Fatalf("Korean particle inflection lost a directly related fact: relevance=%v", got)
	}
	if got := prepareTurnPriorityRelevance("Mira calls", "Mirage appeared in the desert."); got != 0 {
		t.Fatalf("English prefix was treated as an inflected non-ASCII term: relevance=%v", got)
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
	oversized := strings.Repeat("Mira archive brass key ", 80)
	out := &prepareTurnInjectionAssembly{
		LatestDirectEvidenceText: "- Mira already locked the archive door.",
		ActualMemoryText: strings.Join([]string{
			"- Mira already locked the archive door.",
			"- " + oversized,
			"- Mira kept the brass key.",
			"- Unrelated low-ranked filler.",
		}, "\n"),
		MemoryDeliveryLineage: map[string]any{"items": []map[string]any{
			{"source_row_id": 1, "final_text": "Mira already locked the archive door.", "importance_score": 10.0, "selection_score": 1.0},
			{"source_row_id": 2, "final_text": oversized, "importance_score": 9.0, "selection_score": 0.9},
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
	if !strings.Contains(finalText, "Unrelated low-ranked filler.") {
		t.Fatalf("the second independent fact-K slot was not used after exact summary dedupe and oversized deferral: %q", finalText)
	}
}

func Test42PriorityMemoryDoesNotUseZeroRelevanceAsAHardRejectionGate(t *testing.T) {
	out := &prepareTurnInjectionAssembly{
		CharacterObjectiveText: "[Character Objective States]\n- Mira already sealed the archive door.",
		Counts:                 map[string]any{},
	}
	appendPrepareTurnPrioritySourceMetadata(out, "character_objective", "character_states", "required", "- Mira already sealed the archive door.", "character_states:201:objective", int64(201), 20, 3, true, "general", "", nil)

	plan := buildPrepareTurnMemoryDeliveryPlan(out, 6000, map[string]any{
		"_priority_memory_enabled": true, "_priority_memory_max_items": 1,
		"_priority_memory_query": "A semantically phrased request with no lexical token overlap.",
	})
	finalText := extractionStringFromAny(plan["final_text"])
	if !strings.Contains(finalText, "Mira already sealed the archive door") {
		t.Fatalf("zero lexical relevance became a hard delivery rejection: %q items=%#v", finalText, plan["priority_items"])
	}
	if intFromAny(plan["priority_candidate_count"], 0) != 1 || intFromAny(plan["priority_selected_count"], 0) != 1 {
		t.Fatalf("candidate diagnostics drifted: %#v", plan)
	}
	for _, raw := range prepareTurnMemoryLineageSlice(plan["priority_items"]) {
		item := mapFromAny(raw)
		if extractionStringFromAny(item["selection_reason"]) == "no_current_context_affinity" {
			t.Fatalf("forbidden hard affinity rejection remains: %#v", item)
		}
	}
}

func Test42PriorityMemoryProductionPoolDoesNotBypassExistingMemoryRecall(t *testing.T) {
	memories := []store.Memory{
		{ID: 301, ChatSessionID: "recall-pool", TurnIndex: 20, Importance: 0.8, SummaryJSON: `{"turn_summary":"Mira sealed the eastern archive door."}`},
		{ID: 302, ChatSessionID: "recall-pool", TurnIndex: 21, Importance: 1.0, SummaryJSON: `{"turn_summary":"Cedric rehearsed a violin piece in a distant capital."}`},
	}
	assembly := buildPrepareTurnInjectionAssembly(
		memories, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		5, 6000, "Mira checks the sealed eastern archive door.", "default", nil,
		map[string]any{"memory_search_result": "not_found", "search_result": "not_found"}, nil,
		map[string]any{
			"_priority_memory_enabled": true, "_priority_memory_max_items": 5,
			"_priority_memory_query": "Mira checks the sealed eastern archive door.",
		},
	)
	plan := assembly.MemoryDeliveryPlan
	finalText := extractionStringFromAny(plan["final_text"])
	if !strings.Contains(finalText, "Mira sealed") || strings.Contains(finalText, "Cedric") {
		t.Fatalf("priority pool bypassed existing memory recall: %q items=%#v", finalText, plan["priority_items"])
	}
	if intFromAny(plan["priority_candidate_count"], 0) != 2 || intFromAny(plan["priority_resolved_count"], 0) != 1 ||
		intFromAny(plan["turn_summary_candidate_count"], 0) != 1 {
		t.Fatalf("unrecalled session-wide Memory rows entered the priority pool: %#v", plan["priority_items"])
	}
}

func Test42PriorityMemoryShortContinueUsesLatestAcceptedAssistantForFactAffinity(t *testing.T) {
	query := prepareTurnPriorityContextQuery(
		"Continue.",
		"Mira sealed the eastern archive door and kept the silver latch.",
		nil,
		[]string{"Mira"},
	)
	out := &prepareTurnInjectionAssembly{
		CharacterObjectiveText: strings.Join([]string{
			"[Character Objective States]",
			"- Mira kept the silver latch after sealing the eastern archive door.",
			"- Rook mapped an unrelated desert observatory.",
		}, "\n"),
		Counts: map[string]any{},
	}
	plan := buildPrepareTurnMemoryDeliveryPlan(out, 6000, map[string]any{
		"_priority_memory_enabled": true, "_priority_memory_max_items": 1,
		"_priority_memory_query": query,
	})
	finalText := extractionStringFromAny(plan["final_text"])
	if !strings.Contains(finalText, "Mira kept the silver latch") || !strings.Contains(finalText, "Rook") || strings.Index(finalText, "Mira kept the silver latch") >= strings.Index(finalText, "Rook") {
		t.Fatalf("short continuation did not reuse the latest accepted dialogue context precisely: %q items=%#v", finalText, plan["priority_items"])
	}
	if plan["relevance_query_source"] != "assembly_context" {
		t.Fatalf("priority relevance source was not diagnosed: %#v", plan)
	}
}

func Test42PriorityMemoryProductionAssemblyCarriesLatestAcceptedAssistantIntoShortContinue(t *testing.T) {
	private := []store.ProtagonistEntityMemory{
		{ID: 301, OwnerEntityKey: "mira", OwnerEntityName: "Mira", OwnerEntityRole: "npc", SourceTurn: 30, Importance10: 7, MemoryText: "Mira kept the silver latch after sealing the eastern archive door."},
		{ID: 302, OwnerEntityKey: "rook", OwnerEntityName: "Rook", OwnerEntityRole: "npc", SourceTurn: 30, Importance10: 10, MemoryText: "Rook mapped an unrelated desert observatory."},
	}
	chatLogs := []store.ChatLog{{TurnIndex: 30, Role: "assistant", Content: "Mira sealed the eastern archive door and kept the silver latch."}}
	context := priorityMemoryTestContext(1)
	context["_priority_memory_query"] = "Mira sealed the eastern archive door and kept the silver latch."
	context["_priority_memory_query_source"] = "continuity_query"
	context["_priority_memory_current_turn"] = 31
	assembly := buildPrepareTurnInjectionAssembly(
		nil, nil, nil, chatLogs, nil, nil, nil, nil, nil, nil, nil, nil, private,
		5, 6000, "Continue.", "default", nil,
		map[string]any{"memory_search_result": "not_found", "search_result": "not_found"}, nil,
		context,
	)
	finalText := extractionStringFromAny(assembly.MemoryDeliveryPlan["final_text"])
	if !strings.Contains(finalText, "Mira kept the silver latch") || !strings.Contains(finalText, "Rook") || strings.Index(finalText, "Mira kept the silver latch") >= strings.Index(finalText, "Rook") {
		t.Fatalf("production assembly lost or broadened short-continuation affinity: %q items=%#v", finalText, assembly.MemoryDeliveryPlan["priority_items"])
	}
	if !boolFromAny(assembly.Counts["previous_assistant_used_for_priority_fact_affinity"]) || boolFromAny(assembly.Counts["previous_assistant_raw_used_for_search"]) {
		t.Fatalf("assistant context crossed the ranking/retrieval boundary: %#v", assembly.Counts)
	}
}

func Test42PriorityMemoryUsesSeparateRecentConversationQueriesWithoutPromptPhraseRules(t *testing.T) {
	raw := "이어서 적어주세요."
	continuity := "월하방 누각에서 한얼이 이미 편지를 내민 직후의 장면"
	query, source := prepareTurnEffectiveContinuityQuery(dto.PrepareTurnRequest{
		RawUserInput:    &raw,
		ContinuityQuery: &continuity,
	}, 5)
	if query != continuity || source != "continuity_query" {
		t.Fatalf("effective query=%q source=%q, want the observed continuity query", query, source)
	}

	ordinaryRaw := "편지를 건넨 뒤 상대의 반응을 살핀다."
	previous1 := "단주는 한얼이 내민 편지를 받아 들고 배상문의 일을 물었다."
	previous2 := "한얼은 월하방 누각에서 편지를 꺼내기 전 소월과 마주 앉았다."
	previous3 := "그보다 앞서 한얼은 저잣거리에서 약재를 샀다."
	user1 := "배상문의 소식을 전한다."
	user2 := "월하방에서 편지를 꺼낸다."
	user3 := "저잣거리에서 약재를 고른다."
	query, source = prepareTurnEffectiveContinuityQuery(dto.PrepareTurnRequest{
		RawUserInput: &ordinaryRaw,
		Messages: []map[string]any{
			{"role": "user", "content": user3},
			{"role": "assistant", "content": previous3},
			{"role": "user", "content": user2},
			{"role": "assistant", "content": previous2},
			{"role": "user", "content": user1},
			{"role": "assistant", "content": previous1},
			{"role": "user", "content": ordinaryRaw},
		},
	}, 2)
	wantQuery := ordinaryRaw + "\nuser:\n" + user1 + "\nassistant:\n" + previous1 + "\nuser:\n" + user2 + "\nassistant:\n" + previous2
	if query != wantQuery || source != "raw_user_input+recent_conversation_turns" {
		t.Fatalf("ordinary effective query=%q source=%q", query, source)
	}

	query, source = prepareTurnEffectiveContinuityQuery(dto.PrepareTurnRequest{
		Messages: []map[string]any{{"role": "assistant", "content": previous1}},
	}, 1)
	if query != "assistant:\n"+previous1 || source != "recent_conversation_turns" {
		t.Fatalf("assistant-only effective query=%q source=%q", query, source)
	}
	if relevance := prepareTurnPriorityQuerySetRelevance(
		[]string{"항구의 종소리를 살핀다.", "배상문은 천기대 일을 위해 일부러 좌천된 척했다."},
		"항구의 종소리를 살핀다.\n배상문은 천기대 일을 위해 일부러 좌천된 척했다.",
		"배상문은 천기대 일을 위해 일부러 좌천된 척했다.",
	); relevance != 1 {
		t.Fatalf("query-set fact relevance=%v, want the strongest individual query score", relevance)
	}
}

func Test42PriorityMemoryChromaEmbeddingUsesTheSameContinuityQuery(t *testing.T) {
	raw := "이어서 적어주세요."
	continuity := "월하방 누각에서 한얼이 이미 편지를 내민 직후의 장면"
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), continuity) || strings.Contains(string(body), raw) {
			t.Fatalf("vector embedding query drifted from continuity query: %s", body)
		}
		return &http.Response{
			StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{"model":"embed-model","data":[{"embedding":[0.1,0.2,0.3]}]}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	cfg := config.Default()
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.Readiness.ChromaConfigured = true
	srv := NewServer(cfg)
	srv.Vector = &turnRecordingVectorStore{}
	shadow := srv.prepareTurnVectorShadow(context.Background(), dto.PrepareTurnRequest{
		ChatSessionID: "same-query", RawUserInput: &raw, ContinuityQuery: &continuity,
		ClientMeta: map[string]any{"embedding": map[string]any{
			"api_key": "embed-key", "endpoint": "https://api.example.test/v1", "model": "embed-model", "provider": "openai", "timeout_ms": 30000,
		}},
	}, 5)
	if shadow["query_text_source"] != "continuity_query" || shadow["query_embedding_status"] != "ok" {
		t.Fatalf("vector query trace mismatch: %#v", shadow)
	}
}

func Test42PriorityMemoryChromaEmbeddingUsesSeparateRecentConversationCount(t *testing.T) {
	raw := "편지를 건넨 뒤 상대의 반응을 살핀다."
	previous1 := "단주는 한얼이 내민 편지를 받아 들고 배상문의 일을 물었다."
	previous2 := "한얼은 월하방 누각에서 편지를 꺼내기 전 소월과 마주 앉았다."
	previous3 := "그보다 앞서 한얼은 저잣거리에서 약재를 샀다."
	user1 := "배상문의 소식을 전한다."
	user2 := "월하방에서 편지를 꺼낸다."
	user3 := "저잣거리에서 약재를 고른다."
	recentConversationLimit := 2
	embeddingInputs := []string{}
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode embedding request: %v body=%s", err, body)
		}
		embeddingInputs = append(embeddingInputs, extractionStringFromAny(payload["input"]))
		return &http.Response{
			StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{"model":"embed-model","data":[{"embedding":[0.1,0.2,0.3]}]}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	cfg := config.Default()
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.Readiness.ChromaConfigured = true
	srv := NewServer(cfg)
	srv.Vector = &turnRecordingVectorStore{}
	shadow := srv.prepareTurnVectorShadow(context.Background(), dto.PrepareTurnRequest{
		ChatSessionID: "ordinary-previous-turn-query", RawUserInput: &raw,
		Messages: []map[string]any{
			{"role": "user", "content": user3},
			{"role": "assistant", "content": previous3},
			{"role": "user", "content": user2},
			{"role": "assistant", "content": previous2},
			{"role": "user", "content": user1},
			{"role": "assistant", "content": previous1},
			{"role": "user", "content": raw},
		},
		Settings: dto.PrepareTurnSettings{RecentConversationReferenceCount: &recentConversationLimit},
		ClientMeta: map[string]any{"embedding": map[string]any{
			"api_key": "embed-key", "endpoint": "https://api.example.test/v1", "model": "embed-model", "provider": "openai", "timeout_ms": 30000,
		}},
	}, 1)
	if shadow["query_text_source"] != "raw_user_input+recent_conversation_turns" || shadow["query_embedding_status"] != "ok" {
		t.Fatalf("ordinary vector query trace mismatch: %#v", shadow)
	}
	wantLatest := "user:\n" + user1 + "\nassistant:\n" + previous1
	wantPrevious := "user:\n" + user2 + "\nassistant:\n" + previous2
	if len(embeddingInputs) != 3 || embeddingInputs[0] != raw || embeddingInputs[1] != wantLatest || embeddingInputs[2] != wantPrevious {
		t.Fatalf("recent conversation query inputs=%#v, want current input plus latest two completed conversations", embeddingInputs)
	}
	if intFromAny(shadow["recent_conversation_query_limit"], 0) != 2 || intFromAny(shadow["recent_conversation_query_count"], 0) != 2 || intFromAny(shadow["query_vector_count"], 0) != 3 {
		t.Fatalf("recent conversation query counts mismatch: %#v", shadow)
	}
}

type priorityMultiQueryVector struct {
	vector.VectorStore
}

func (v *priorityMultiQueryVector) Health(context.Context) (vector.HealthSnapshot, error) {
	return vector.HealthSnapshot{Status: "ok", Collection: "multi-query-test", ModelReady: true, TotalCount: 1}, nil
}

func (v *priorityMultiQueryVector) Search(_ context.Context, sid string, query []float32, _ int, _ string) ([]vector.VectorDocument, error) {
	similarity := 0.0
	if len(query) > 0 {
		similarity = float64(query[0])
	}
	return []vector.VectorDocument{{
		ID: "shared-memory", ChatSessionID: sid, Tier: "memory", SourceTable: "memories", SourceRowID: "1",
		Similarity: similarity, SimilarityAvailable: true, SimilaritySource: "cosine",
	}}, nil
}

func Test42PriorityMemoryMultiQueryMergeKeepsStrongestSimilarity(t *testing.T) {
	raw := "현재 입력"
	previous1 := "가장 가까운 직전 원문"
	previous2 := "두 번째 직전 원문"
	previousQuery1 := "assistant:\n" + previous1
	previousQuery2 := "assistant:\n" + previous2
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode embedding request: %v body=%s", err, body)
		}
		value := 0.1
		switch extractionStringFromAny(payload["input"]) {
		case previousQuery1:
			value = 0.9
		case previousQuery2:
			value = 0.4
		}
		return &http.Response{
			StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header),
			Body: io.NopCloser(strings.NewReader(fmt.Sprintf(`{"model":"embed-model","data":[{"embedding":[%.1f,0.2,0.3]}]}`, value))),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	cfg := config.Default()
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.Readiness.ChromaConfigured = true
	srv := NewServer(cfg)
	srv.Vector = &priorityMultiQueryVector{}
	shadow := srv.prepareTurnVectorShadow(context.Background(), dto.PrepareTurnRequest{
		ChatSessionID: "multi-query-merge", RawUserInput: &raw,
		Messages: []map[string]any{
			{"role": "assistant", "content": previous2},
			{"role": "assistant", "content": previous1},
		},
		ClientMeta: map[string]any{"embedding": map[string]any{
			"api_key": "embed-key", "endpoint": "https://api.example.test/v1", "model": "embed-model", "provider": "openai", "timeout_ms": 30000,
		}},
	}, 2)
	results := prepareTurnVectorSearchResultMaps(shadow["memory_search_results"])
	if len(results) != 1 || math.Abs(floatFromAny(results[0]["similarity"])-0.9) > 0.0001 {
		t.Fatalf("duplicate merge did not retain strongest similarity: %#v", shadow)
	}
}

func Test42PriorityMemorySupplementalAssistantEmbeddingFailureKeepsPrimaryRecall(t *testing.T) {
	raw := "현재 입력"
	previous := "직전 assistant 원문"
	requestCount := 0
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		requestCount++
		if requestCount == 2 {
			return &http.Response{
				StatusCode: http.StatusInternalServerError, Status: "500 Internal Server Error", Header: make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{"error":"history embedding failed"}`)),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK, Status: "200 OK", Header: make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{"model":"embed-model","data":[{"embedding":[0.1,0.2,0.3]}]}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	cfg := config.Default()
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.Readiness.ChromaConfigured = true
	srv := NewServer(cfg)
	srv.Vector = &turnRecordingVectorStore{}
	shadow := srv.prepareTurnVectorShadow(context.Background(), dto.PrepareTurnRequest{
		ChatSessionID: "supplemental-history-failure", RawUserInput: &raw,
		Messages: []map[string]any{{"role": "assistant", "content": previous}},
		ClientMeta: map[string]any{"embedding": map[string]any{
			"api_key": "embed-key", "endpoint": "https://api.example.test/v1", "model": "embed-model", "provider": "openai", "timeout_ms": 30000,
		}},
	}, 1)
	if shadow["query_embedding_status"] != "ok" || !boolFromAny(shadow["search_attempted"]) {
		t.Fatalf("supplemental history failure blocked the existing primary recall path: %#v", shadow)
	}
	if intFromAny(shadow["query_vector_count"], 0) != 1 || intFromAny(shadow["query_history_embedding_error_count"], 0) != 1 {
		t.Fatalf("supplemental history failure trace mismatch: %#v", shadow)
	}
}

type priorityPreciseMemoryReader struct {
	unitsBySession map[string][]store.PreciseMemoryUnit
}

type priorityPrepareTurnStore struct {
	*turnRecordingStore
	precise []store.PreciseMemoryUnit
}

func (s *priorityPrepareTurnStore) ListGeneralVectorPreciseMemoryUnits(_ context.Context, _ string) ([]store.PreciseMemoryUnit, error) {
	return append([]store.PreciseMemoryUnit(nil), s.precise...), nil
}

type priorityPrepareTurnVector struct {
	vector.VectorStore
	doc   vector.VectorDocument
	calls []priorityPreciseCandidateSearchCall
}

type priorityPreciseCandidateSearchCall struct {
	Limit  int
	Filter string
}

type priorityPreciseCandidateVector struct {
	vector.VectorStore
	calls []priorityPreciseCandidateSearchCall
	docs  []vector.VectorDocument
}

func (v *priorityPreciseCandidateVector) Health(context.Context) (vector.HealthSnapshot, error) {
	return vector.HealthSnapshot{Status: "ok", Collection: "priority-candidate-test", ModelReady: true}, nil
}

func (v *priorityPreciseCandidateVector) Search(_ context.Context, _ string, _ []float32, limit int, filter string) ([]vector.VectorDocument, error) {
	v.calls = append(v.calls, priorityPreciseCandidateSearchCall{Limit: limit, Filter: filter})
	if !strings.Contains(filter, `source_table == "precise_memory_units"`) {
		return nil, vector.ErrNotFound
	}
	if limit > len(v.docs) {
		limit = len(v.docs)
	}
	return append([]vector.VectorDocument(nil), v.docs[:limit]...), nil
}

func Test42PriorityMemoryPreciseCandidateRecallIsNotLimitedByFinalK(t *testing.T) {
	const sid = "precise-candidate-limit"
	vec := &priorityPreciseCandidateVector{docs: []vector.VectorDocument{
		{ID: "precise_memory:" + sid + ":one", ChatSessionID: sid, SourceTable: "precise_memory_units", SourceRowID: "one", Similarity: 0.9, SimilarityAvailable: true},
		{ID: "precise_memory:" + sid + ":two", ChatSessionID: sid, SourceTable: "precise_memory_units", SourceRowID: "two", Similarity: 0.8, SimilarityAvailable: true},
		{ID: "precise_memory:" + sid + ":three", ChatSessionID: sid, SourceTable: "precise_memory_units", SourceRowID: "three", Similarity: 0.7, SimilarityAvailable: true},
	}}
	cfg := config.Default()
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.Readiness.ChromaConfigured = true
	srv := NewServer(cfg)
	srv.Vector = vec
	shadow := srv.prepareTurnVectorShadowWithPreciseCandidateLimits(
		context.Background(),
		dto.PrepareTurnRequest{ChatSessionID: sid, ClientMeta: map[string]any{"chroma_query_vector": []float64{0.1, 0.2}}},
		1,
		map[string]int{sid: 3},
	)
	if intFromAny(shadow["precise_memory_search_result_count"], 0) != 3 {
		t.Fatalf("precise fact recall was limited by final K: %#v", shadow)
	}
	found := false
	for _, call := range vec.calls {
		if strings.Contains(call.Filter, `source_table == "precise_memory_units"`) {
			found = call.Limit == 3
		}
	}
	if !found {
		t.Fatalf("dedicated precise fact search did not use canonical candidate count: %#v", vec.calls)
	}
}

func (v *priorityPrepareTurnVector) Health(context.Context) (vector.HealthSnapshot, error) {
	return vector.HealthSnapshot{Status: "ok", Collection: "priority-test", ModelReady: true}, nil
}

func (v *priorityPrepareTurnVector) Search(_ context.Context, _ string, _ []float32, limit int, filter string) ([]vector.VectorDocument, error) {
	v.calls = append(v.calls, priorityPreciseCandidateSearchCall{Limit: limit, Filter: filter})
	if strings.Contains(filter, `tier == "memory"`) {
		return nil, vector.ErrNotFound
	}
	return []vector.VectorDocument{v.doc}, nil
}

func (r *priorityPreciseMemoryReader) ListGeneralVectorPreciseMemoryUnits(_ context.Context, sid string) ([]store.PreciseMemoryUnit, error) {
	return append([]store.PreciseMemoryUnit(nil), r.unitsBySession[sid]...), nil
}

func Test42PriorityMemoryHydratesFactVectorSimilarityFromCanonicalPreciseUnit(t *testing.T) {
	reader := &priorityPreciseMemoryReader{unitsBySession: map[string][]store.PreciseMemoryUnit{
		"current": {{
			UnitID: "letter-current", ChatSessionID: "current", SourceTurnStart: 98, SourceTurnEnd: 98,
			SourceRevision: "rev-98", Kind: "event", Subtype: "observed_event",
			PayloadJSON: `{"summary":"한얼은 월하방 누각에서 이미 편지를 내밀었다.","actor":"한얼","location":"월하방 누각","storyline":"편지 전달"}`,
			Visibility:  "public", EpistemicMode: "direct", LifecycleState: "active",
		}},
	}}
	shadow := map[string]any{
		"search_result": "ok",
		"search_results": []map[string]any{{
			"id": "precise_memory:current:letter-current", "source_table": "precise_memory_units",
			"source_row_id": "letter-current", "chat_session_id": "current",
			"similarity": 0.91, "similarity_source": "cosine",
		}},
	}
	history := prepareTurnHistoryScope{Segments: []prepareTurnHistorySegment{{SessionID: "current", FromTurn: 1, ToTurn: 99}}}
	facts, trace := prepareTurnHydratePreciseMemoryVectorFacts(context.Background(), reader, shadow, history)
	if len(facts) != 1 || facts[0].UnitID != "letter-current" || facts[0].Similarity != 0.91 {
		t.Fatalf("fact vector hydration=%#v trace=%#v", facts, trace)
	}
	if facts[0].Fact.Text != "한얼은 월하방 누각에서 이미 편지를 내밀었다." || facts[0].Fact.LocationSurface != "월하방 누각" {
		t.Fatalf("canonical precise unit was not hydrated as its own fact: %#v", facts[0])
	}
	if trace["hydrated_count"] != 1 || trace["score_owner"] != "precise_memory_unit_vector_similarity" {
		t.Fatalf("fact-vector trace mismatch: %#v", trace)
	}
}

func Test42PriorityMemoryHTTPPathCarriesPreciseSimilarityIntoFinalPlan(t *testing.T) {
	const sid = "priority-http-precise"
	unit := store.PreciseMemoryUnit{
		UnitID: "letter-current", ChatSessionID: sid, SourceTurnStart: 98, SourceTurnEnd: 98,
		SourceRevision: "rev-98", Kind: "event", Subtype: "observed_event",
		PayloadJSON: `{"summary":"한얼은 월하방 누각에서 이미 편지를 내밀었다.","actor":"한얼","location":"월하방 누각","storyline":"편지 전달"}`,
		Visibility:  "public", EpistemicMode: "direct", AdmissionState: "committed", ReviewState: "source_observed", LifecycleState: "active",
	}
	fake := &priorityPrepareTurnStore{
		turnRecordingStore: &turnRecordingStore{returnMemories: []store.Memory{{
			ID: 501, ChatSessionID: sid, TurnIndex: 12, Importance: 0.9,
			SummaryJSON: `{"turn_summary":"한얼은 월하방 누각에서 배상문을 처음 만났다."}`,
		}}},
		precise: []store.PreciseMemoryUnit{unit},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.Readiness.ChromaConfigured = true
	srv := NewServer(cfg)
	srv.Store = fake
	vec := &priorityPrepareTurnVector{doc: vector.VectorDocument{
		ID: "precise_memory:" + sid + ":letter-current", ChatSessionID: sid,
		SourceTable: "precise_memory_units", SourceRowID: "letter-current", SchemaVersion: store.PreciseMemoryUnitContract,
		Similarity: 0.93, SimilarityAvailable: true, SimilaritySource: "cosine",
	}}
	srv.Vector = vec
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := map[string]any{
		"chat_session_id": sid, "turn_index": 99, "raw_user_input": "이어서 적어주세요.",
		"continuity_query": "월하방 누각에서 한얼이 이미 편지를 내민 직후의 장면",
		"client_meta":      map[string]any{"chroma_query_vector": []float64{0.1, 0.2}},
		"settings":         map[string]any{"max_injection_chars": 6000, "injection_enabled": true, "input_context_enabled": false, "top_k": 5, "core_objective_memory_max_items": 1},
	}
	encoded, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader(encoded))
	req.Header.Set("Content-Type", "application/json")
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	response := map[string]any{}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	plan := mapFromAny(mapFromAny(response["injection_pack"])["memory_delivery_plan"])
	if !strings.Contains(extractionStringFromAny(plan["final_text"]), "편지를 내밀었다") {
		t.Fatalf("HTTP prepare-turn lost the precise fact vector hit: %#v", plan)
	}
	if plan["relevance_query_source"] != "continuity_query" || intFromAny(plan["semantic_fact_vector_count"], 0) != 1 {
		t.Fatalf("HTTP query/fact score lineage mismatch: %#v", plan)
	}
	preciseCallFound := false
	for _, call := range vec.calls {
		if strings.Contains(call.Filter, `source_table == "precise_memory_units"`) && call.Limit == 1 {
			preciseCallFound = true
		}
	}
	if !preciseCallFound {
		t.Fatalf("HTTP path did not use canonical precise fact count with existing metadata: %#v", vec.calls)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte(prepareTurnPrivatePreciseMemorySearchResultsKey)) {
		t.Fatalf("private precise-memory vector handoff leaked into prepare-turn response: %s", rec.Body.String())
	}
}

func Test42PriorityMemoryFactVectorSeparatesSamePersonAndPlaceEvents(t *testing.T) {
	memories := []store.Memory{
		{ID: 401, ChatSessionID: "same-scene-hard-negative", TurnIndex: 98, Importance: 0.8,
			SummaryJSON: `{"narrative_events":[{"event":"한얼은 월하방 누각에서 이미 편지를 내밀었다.","actor":"한얼","location":"월하방 누각","storyline":"편지 전달"}]}`},
		{ID: 402, ChatSessionID: "same-scene-hard-negative", TurnIndex: 12, Importance: 0.8,
			SummaryJSON: `{"narrative_events":[{"event":"한얼은 월하방 누각에서 배상문을 처음 만났다.","actor":"한얼","location":"월하방 누각","storyline":"첫 방문"}]}`},
	}
	context := priorityMemoryTestContext(1)
	context["_priority_memory_query"] = "월하방 누각에서 한얼이 이미 편지를 내민 직후의 장면"
	context["_priority_memory_query_source"] = "continuity_query"
	context["_priority_memory_current_turn"] = 99
	context[prepareTurnPrioritySemanticFactsContextKey] = []prepareTurnPrioritySemanticFact{
		{UnitID: "letter", SourceTurn: 98, Similarity: 0.92, SimilaritySource: "cosine", Fact: prepareTurnPriorityMemoryFact{Text: "한얼: 한얼은 월하방 누각에서 이미 편지를 내밀었다.", FamilyKey: collapseTextKey("narrative_events\x1f한얼\x1f\x1f한얼은 월하방 누각에서 이미 편지를 내밀었다."), ValueKey: collapseTextKey("한얼은 월하방 누각에서 이미 편지를 내밀었다."), EntitySurface: "한얼", LocationSurface: "월하방 누각", StorylineSurface: "편지 전달", Structured: true}},
		{UnitID: "first-visit", SourceTurn: 12, Similarity: 0.34, SimilaritySource: "cosine", Fact: prepareTurnPriorityMemoryFact{Text: "한얼: 한얼은 월하방 누각에서 배상문을 처음 만났다.", FamilyKey: collapseTextKey("narrative_events\x1f한얼\x1f\x1f한얼은 월하방 누각에서 배상문을 처음 만났다."), ValueKey: collapseTextKey("한얼은 월하방 누각에서 배상문을 처음 만났다."), EntitySurface: "한얼", LocationSurface: "월하방 누각", StorylineSurface: "첫 방문", Structured: true}},
	}
	assembly := buildPrepareTurnInjectionAssembly(
		memories, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		5, 6000, "이어서 적어주세요.", "default", nil,
		map[string]any{"memory_search_result": "not_found", "search_result": "not_found"}, nil,
		context,
	)
	finalText := extractionStringFromAny(assembly.MemoryDeliveryPlan["final_text"])
	if !strings.Contains(finalText, "편지를 내밀었다") || !strings.Contains(finalText, "처음 만났다") || strings.Index(finalText, "편지를 내밀었다") >= strings.Index(finalText, "처음 만났다") {
		t.Fatalf("same-character/location hard negative beat the current event: %q items=%#v", finalText, assembly.MemoryDeliveryPlan["priority_items"])
	}
	foundSemantic := false
	for _, rawItem := range prepareTurnMemoryLineageSlice(assembly.MemoryDeliveryPlan["priority_items"]) {
		item := mapFromAny(rawItem)
		if strings.Contains(extractionStringFromAny(item["complete_text"]), "편지를 내밀었다") {
			lineage := mapFromAny(item["score_lineage"])
			foundSemantic = extractionStringFromAny(lineage["relevance_source"]) == "precise_memory_unit_vector_similarity" &&
				extractionFloatFromAny(item["relevance_score"], 0) == 0.92
		}
	}
	if !foundSemantic {
		t.Fatalf("fact-level vector similarity did not reach final score lineage: %#v", assembly.MemoryDeliveryPlan["priority_items"])
	}
}

func Test42PriorityMemoryExplicitOldEventQueryCanStillWin(t *testing.T) {
	out := &prepareTurnInjectionAssembly{Counts: map[string]any{}}
	out.PriorityFactSeeds = []prepareTurnPriorityFactSeed{
		{Lane: "event_recent", SourceTable: "memories", Tier: "required", SourceTurn: 98, Importance: 0.8, ImportancePresent: true, Fact: prepareTurnPriorityMemoryFact{Text: "한얼은 월하방 누각에서 편지를 내밀었다.", FamilyKey: "letter", ValueKey: "letter", EntitySurface: "한얼", LocationSurface: "월하방 누각", StorylineSurface: "편지 전달", Structured: true}},
		{Lane: "event_recent", SourceTable: "memories", Tier: "required", SourceTurn: 12, Importance: 0.8, ImportancePresent: true, Fact: prepareTurnPriorityMemoryFact{Text: "한얼은 월하방 누각에서 배상문을 처음 만났다.", FamilyKey: "first-visit", ValueKey: "first-visit", EntitySurface: "한얼", LocationSurface: "월하방 누각", StorylineSurface: "첫 방문", Structured: true}},
	}
	perspective := map[string]any{
		"_priority_memory_enabled": true, "_priority_memory_max_items": 1,
		"_priority_memory_query":        "한얼과 배상문이 월하방 누각에서 처음 만난 일을 회상한다.",
		"_priority_memory_query_source": "raw_user_input", "_priority_memory_current_turn": 99,
		prepareTurnPrioritySemanticFactsContextKey: []prepareTurnPrioritySemanticFact{
			{UnitID: "letter", SourceTurn: 98, Similarity: 0.38, SimilaritySource: "cosine", Fact: out.PriorityFactSeeds[0].Fact},
			{UnitID: "first-visit", SourceTurn: 12, Similarity: 0.96, SimilaritySource: "cosine", Fact: out.PriorityFactSeeds[1].Fact},
		},
	}
	plan := buildPrepareTurnMemoryDeliveryPlan(out, 6000, perspective)
	finalText := extractionStringFromAny(plan["final_text"])
	if !strings.Contains(finalText, "배상문을 처음 만났다") || !strings.Contains(finalText, "편지를 내밀었다") || strings.Index(finalText, "배상문을 처음 만났다") >= strings.Index(finalText, "편지를 내밀었다") {
		t.Fatalf("explicit old-event request was erased by recency: %q items=%#v", finalText, plan["priority_items"])
	}
}

func Test42PriorityMemoryTurnDistanceRecencyHasFloor(t *testing.T) {
	latest := prepareTurnPriorityTurnDistanceRecency(99, 100)
	older := prepareTurnPriorityTurnDistanceRecency(1, 100)
	if latest <= older || older < prepareTurnPriorityRecencyFloor || latest > 1 {
		t.Fatalf("turn-distance recency latest=%v older=%v floor=%v", latest, older, prepareTurnPriorityRecencyFloor)
	}
	if prepareTurnPriorityTurnDistanceRecency(0, 100) != 0.5 {
		t.Fatal("unknown source turn must remain neutral rather than becoming a rejection signal")
	}
}

func Test42PriorityMemoryIdentityMetadataAttachesWithoutConsumingK(t *testing.T) {
	out := &prepareTurnInjectionAssembly{
		CanonCharacterText: `[Canonical Character States]\n- entity_state: {"characters":[{"name":"Mira","aliases":["Silver Mask"],"identity_evidence_excerpt":"called the Silver Mask","current_goal":"guard the eastern door"}]}`,
		Counts:             map[string]any{},
	}
	plan := buildPrepareTurnMemoryDeliveryPlan(out, 6000, map[string]any{
		"_priority_memory_enabled": true, "_priority_memory_max_items": 1,
		"_priority_memory_query": "Mira guards the eastern door.", "_priority_memory_current_turn": 50,
	})
	finalText := extractionStringFromAny(plan["final_text"])
	if !strings.Contains(finalText, "current_goal: guard the eastern door") {
		t.Fatalf("narrative fact was not selected: %q items=%#v", finalText, plan["priority_items"])
	}
	if !strings.Contains(finalText, "Silver Mask") || intFromAny(plan["priority_selected_count"], 0) != 1 {
		t.Fatalf("identity metadata was not attached to one fact/K slot: %q plan=%#v", finalText, plan)
	}
	if intFromAny(plan["identity_metadata_count"], 0) < 2 || intFromAny(plan["priority_candidate_count"], 0) != 1 {
		t.Fatalf("identity metadata still counted as event facts: %#v", plan)
	}
}

func Test43PriorityAggregateVectorReachesSummaryWithoutBoostingSiblingFacts(t *testing.T) {
	const sid = "semantic-summary-repair"
	for _, k := range []int{1, 5} {
		t.Run(fmt.Sprintf("K%d", k), func(t *testing.T) {
			memories := []store.Memory{{ID: 701, ChatSessionID: sid, TurnIndex: 10, Importance: .9,
				SummaryJSON: `{"turn_summary":"Mira unmasked herself before Rowan, ending the masquerade. A harbor vendor arranged empty baskets.","narrative_events":[{"event":"Mira unmasked herself before Rowan, ending the masquerade.","actor":"Mira","visibility":"public"},{"event":"A harbor vendor arranged empty baskets.","visibility":"public"}]}`}}
			for i := 0; i < 2*k+1; i++ {
				raw, err := json.Marshal(map[string]any{"turn_summary": fmt.Sprintf("Rowan reviews notice number %d about the identity of a visiting merchant.", i)})
				if err != nil {
					t.Fatal(err)
				}
				memories = append(memories, store.Memory{ID: int64(710 + i), ChatSessionID: sid, TurnIndex: 80 + i, Importance: .5, SummaryJSON: string(raw)})
			}
			original, err := json.Marshal(memories)
			if err != nil {
				t.Fatal(err)
			}
			similarity := .97
			hits := []map[string]any{{"id": "memory:" + sid + ":701", "tier": "memory", "source_table": "memories", "source_row_id": "701", "chat_session_id": sid, "similarity": similarity, "similarity_source": "cosine_from_query_and_stored_embedding"}}
			shadow := map[string]any{"status": "ready", "memory_search_result": "ok", "search_result": "ok", "memory_search_results": hits, "search_results": hits}
			query := "What does Rowan know about the hidden identity?"
			assembly := buildPrepareTurnInjectionAssembly(memories, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
				k, 30000, query, "default", nil, shadow, nil, map[string]any{
					"_priority_memory_enabled": true, "_priority_memory_max_items": k,
					"_priority_memory_current_turn": 100, "_priority_memory_query": query,
				})
			plan := assembly.MemoryDeliveryPlan
			if !strings.Contains(extractionStringFromAny(plan["final_text"]), "unmasked herself") {
				t.Errorf("retrieved disclosure lost in final memory: %s", plan["final_text"])
			}
			foundSummary, foundSibling := false, false
			for _, raw := range prepareTurnMemoryLineageSlice(plan["turn_summary_items"]) {
				item := mapFromAny(raw)
				if strings.Contains(extractionStringFromAny(item["complete_text"]), "unmasked herself") {
					foundSummary = true
					if item["selection_status"] != "selected" || extractionFloatFromAny(item["source_vector_similarity"], 0) != similarity || item["score_source"] != "aggregate_memory_vector_similarity" {
						t.Errorf("summary lost its own vector observation: %#v", item)
					}
				}
			}
			for _, raw := range prepareTurnMemoryLineageSlice(plan["priority_items"]) {
				item := mapFromAny(raw)
				if strings.Contains(extractionStringFromAny(item["complete_text"]), "harbor vendor") {
					foundSibling = true
					lineage := mapFromAny(item["score_lineage"])
					if extractionFloatFromAny(item["relevance_score"], 0) >= similarity || boolFromAny(lineage["source_selection_score_used_as_fact_relevance"]) {
						t.Errorf("aggregate similarity leaked into unrelated sibling: %#v", item)
					}
				}
			}
			if !foundSummary || !foundSibling {
				t.Fatal("fixture did not expose both source summary and sibling candidate")
			}
			after, err := json.Marshal(memories)
			if err != nil || !bytes.Equal(original, after) {
				t.Fatal("selection mutated the supplied canonical memories")
			}
		})
	}
}

func Test43PriorityLexicalParentScoreIsNotAnAggregateVector(t *testing.T) {
	memories := []store.Memory{{ID: 733, ChatSessionID: "lexical-summary", TurnIndex: 9, Importance: .6, SummaryJSON: `{"turn_summary":"Mira sealed the archive door."}`}}
	assembly := buildPrepareTurnInjectionAssembly(memories, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		5, 30000, "Mira sealed the archive door.", "default", nil, nil, nil, priorityMemoryTestContext(5))
	items := prepareTurnMemoryLineageSlice(assembly.MemoryDeliveryPlan["turn_summary_items"])
	if len(items) == 0 {
		t.Fatal("fixture produced no summary")
	}
	for _, raw := range items {
		item := mapFromAny(raw)
		if boolFromAny(item["source_vector_similarity_observed"]) || item["score_source"] == "aggregate_memory_vector_similarity" {
			t.Errorf("lexical source score was presented as vector similarity: %#v", item)
		}
	}
}

func Test43PriorityAllImportanceValuesRemainIndependentOfAge(t *testing.T) {
	for points := 1; points <= 10; points++ {
		t.Run(fmt.Sprintf("points_%d", points), func(t *testing.T) {
			importance := float64(points) / 10
			for _, current := range []int{2, 100, 10000} {
				out := &prepareTurnInjectionAssembly{Counts: map[string]any{}}
				out.PriorityFactSeeds = []prepareTurnPriorityFactSeed{{
					Lane: "event_recent", SourceTable: "memories", Tier: "required", SourceTurn: 1,
					Importance: importance, ImportancePresent: true, SemanticSimilarity: .8, SemanticSimilarityObserved: true,
					Fact: prepareTurnPriorityMemoryFact{Text: "Mira remembers the promise.", FamilyKey: "promise", ValueKey: "promise", Structured: true},
				}}
				plan := buildPrepareTurnMemoryDeliveryPlan(out, 6000, map[string]any{
					"_priority_memory_enabled": true, "_priority_memory_max_items": 1, "_priority_memory_current_turn": current,
				})
				items := prepareTurnMemoryLineageSlice(plan["priority_items"])
				if len(items) != 1 {
					t.Fatalf("missing scoring fixture at turn %d", current)
				}
				item := mapFromAny(items[0])
				if extractionFloatFromAny(item["importance_score"], -1) != importance || extractionFloatFromAny(item["importance_after_turn_decay"], -1) != importance {
					t.Errorf("turn %d reduced the stored importance or its contribution: %#v", current, item)
				}
			}
		})
	}
}
