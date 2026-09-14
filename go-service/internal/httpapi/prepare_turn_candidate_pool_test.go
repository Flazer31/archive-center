package httpapi

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// Preserve the established fixture inputs while exercising the typed production
// API. Only tests translate historical fixture annotation maps; runtime callers
// construct fields directly and never serialize this internal context.
func testPrepareTurnMemorySelectionContext(raw map[string]any) prepareTurnMemorySelectionContext {
	budgets, budgetsPresent := raw["_memory_delivery_budgets"].(map[string]int)
	result := prepareTurnMemorySelectionContext{
		BudgetMode: extractionStringFromAny(raw["_memory_delivery_budget_mode"]), Budgets: budgets,
		PriorityEnabled: boolFromAny(raw["_priority_memory_enabled"]), MaxItems: intFromAny(raw["_priority_memory_max_items"], 5),
		Query: extractionStringFromAny(raw["_priority_memory_query"]), QuerySource: extractionStringFromAny(raw["_priority_memory_query_source"]),
		QuerySet:           stringsFromAny(raw[prepareTurnPriorityQuerySetContextKey]),
		CurrentTurn:        intFromAny(raw["_priority_memory_current_turn"], 0),
		SemanticFacts:      prepareTurnPrioritySemanticFactsFromAny(raw[prepareTurnPrioritySemanticFactsContextKey]),
		PreciseVectorTrace: raw["_priority_precise_vector_trace"],
	}
	if !budgetsPresent {
		result.BudgetMode = "auto"
	}
	if boolFromAny(raw["_core_objective_memory_max_items_present"]) {
		n := intFromAny(raw["_core_objective_memory_max_items"], 0)
		result.CoreObjectiveLimit = &n
	}
	return result
}

func testPrepareTurnAssemblyPerspective(raw map[string]any) *prepareTurnAssemblyPerspective {
	values, _ := raw["_narrative_current_values"].([]store.StatusCurrentValue)
	states, _ := raw["_narrative_active_states"].([]store.ActiveState)
	seeds, _ := raw["_character_perspective_fact_seeds"].([]prepareTurnPriorityFactSeed)
	return &prepareTurnAssemblyPerspective{
		Public: raw, NarrativeValues: values, ActiveStates: states,
		CharacterText: extractionStringFromAny(raw["_character_perspective_text"]), CharacterSeeds: seeds, CharacterCount: intFromAny(raw["_character_perspective_candidate_count"], 0),
		InteractionPublicText: extractionStringFromAny(raw["_active_interaction_public_text"]), InteractionGuardedText: extractionStringFromAny(raw["_active_interaction_guarded_text"]), InteractionCount: intFromAny(raw["_active_interaction_candidate_count"], 0),
		CharacterMemory: mapFromAny(raw[prepareTurnCharacterMemoryContextKey]), EntityAliases: mapFromAny(raw[prepareTurnEntityIdentityAliasesContextKey]),
		Selection: testPrepareTurnMemorySelectionContext(raw),
	}
}

func priorityCandidatePoolTestAssembly(perspective map[string]any) prepareTurnInjectionAssembly {
	return buildPrepareTurnInjectionAssemblyWithBudget(priorityCandidatePoolTestInput(perspective))
}

func priorityCandidatePoolTestInput(perspective map[string]any) prepareTurnAssemblyInput {
	memories := []store.Memory{
		{ID: 91, ChatSessionID: "candidate-pool", TurnIndex: 30, Importance: 8,
			SummaryJSON: `{"turn_summary":"Mira sealed the archive door. Mira kept the brass key.","narrative_events":[{"event":"Mira sealed the archive door.","visibility":"public"},{"event":"Mira kept the brass key.","visibility":"public"}]}`},
		{ID: 92, ChatSessionID: "candidate-pool", TurnIndex: 20, Importance: 6,
			SummaryJSON: `{"turn_summary":"Mira first visited the archive with Rowan."}`},
	}
	private := []store.ProtagonistEntityMemory{{
		ID: 73, OwnerEntityKey: "mira", OwnerEntityName: "Mira", OwnerVisibility: "owner_private",
		SourceTurn: 30, Importance10: 9, MemoryText: "Mira remembers the hidden promise. Mira remains wary of the archive door.",
	}}
	history := []store.ChatLog{{TurnIndex: 30, Role: "assistant", Content: "Mira sealed the archive door and kept the brass key."}}
	return prepareTurnAssemblyInput{
		Memories:                 memories,
		ChatLogs:                 history,
		CharacterPrivateMemories: private,
		TopK:                     5,
		MaxChars:                 60000,
		UserInput:                "Mira checks the archive door.",
		Profile:                  "default",
		VectorTrace:              map[string]any{"memory_search_result": "not_found", "search_result": "not_found"},
		BudgetMode:               "auto",
		Perspective:              testPrepareTurnAssemblyPerspective(perspective),
	}
}

func Test43PriorityCandidatePoolPreservesProductionAssembly(t *testing.T) {
	perspective := priorityMemoryTestContext(1)
	perspective["_priority_memory_query"] = "Mira checks the archive door."
	perspective[prepareTurnPriorityQuerySetContextKey] = []string{"Mira checks the archive door.", "Mira kept the brass key."}
	perspective["_priority_memory_current_turn"] = 31
	for _, supplemental := range []bool{false, true} {
		name := "initial"
		if supplemental {
			name = "supplemental"
			perspective[prepareTurnPrioritySemanticFactsContextKey] = []prepareTurnPrioritySemanticFact{{
				UnitID: "archive-key-unit", SourceTurn: 30, Lane: "event_recent", Visibility: "public",
				Similarity: 0.8, SimilaritySource: "cosine", Fact: prepareTurnPriorityMemoryFact{Text: "Mira kept the brass key."},
			}}
		}
		t.Run(name, func(t *testing.T) {
			assembly := priorityCandidatePoolTestAssembly(perspective)
			fresh, _, _ := prepareTurnBuildPriorityCandidates(&assembly,
				extractionStringFromAny(perspective["_priority_memory_query"]),
				prepareTurnPriorityQuerySetFromAny(perspective[prepareTurnPriorityQuerySetContextKey]),
				intFromAny(perspective["_priority_memory_current_turn"], 0),
				prepareTurnPrioritySemanticFactsFromAny(perspective[prepareTurnPrioritySemanticFactsContextKey]))
			freshSummaries := prepareTurnBuildPriorityTurnSummaries(fresh)
			facts, summaries := multiAgentCandidatePool(&assembly)
			if !reflect.DeepEqual(facts, fresh) || !reflect.DeepEqual(summaries, freshSummaries) {
				t.Fatalf("request snapshot differs from fresh production candidates: cached=%+v fresh=%+v", facts, fresh)
			}
			if len(summaries) == 0 || len(facts) <= len(summaries) || assembly.Preprocessing != nil {
				t.Fatal("fixture lacks full facts/summaries or snapshot activated preprocessing")
			}
			privateIndex := -1
			for index, fact := range facts {
				if fact.SelectionStatus != "" || fact.SelectionReason != "" || fact.RenderedText != "" || len(fact.IdentityMetadata) != 0 {
					t.Fatalf("selection/rendering mutated the pristine pool: %+v", fact)
				}
				if fact.SourceTable == "protagonist_entity_memories" {
					privateIndex = index
					if fact.CompleteText == "" || fact.Visibility != "owner_private" || fact.PerspectiveOwner != "Mira" || len(fact.AllowedViewers) == 0 {
						t.Fatalf("typed private source text/scope was lost: %+v", fact)
					}
				}
			}
			if privateIndex < 0 {
				t.Fatal("fixture lacks private scoped facts")
			}
			facts[privateIndex].AllowedViewers[0] = "changed returned viewer"
			facts[privateIndex].CompleteText = "changed returned text"
			summaries[0].MemberFactIDs[0] = "changed returned member"
			summaries[0].SelectionStatus = "changed returned status"
			gotFacts, gotSummaries := multiAgentCandidatePool(&assembly)
			if !reflect.DeepEqual(gotFacts, fresh) || !reflect.DeepEqual(gotSummaries, freshSummaries) {
				t.Fatal("a returned candidate pool aliases the request snapshot")
			}

			// A later source mutation distinguishes snapshot consumption from the
			// former duplicate computation without replacing the production owner.
			assembly.CanonWorldText += "\n- The archive stairway now opens onto a courtyard."
			changed, _, _ := prepareTurnBuildPriorityCandidates(&assembly,
				extractionStringFromAny(perspective["_priority_memory_query"]),
				prepareTurnPriorityQuerySetFromAny(perspective[prepareTurnPriorityQuerySetContextKey]),
				intFromAny(perspective["_priority_memory_current_turn"], 0),
				prepareTurnPrioritySemanticFactsFromAny(perspective[prepareTurnPrioritySemanticFactsContextKey]))
			if reflect.DeepEqual(changed, fresh) {
				t.Fatal("negative assertion did not alter the production fresh computation")
			}
			gotFacts, gotSummaries = multiAgentCandidatePool(&assembly)
			if !reflect.DeepEqual(gotFacts, fresh) || !reflect.DeepEqual(gotSummaries, freshSummaries) {
				t.Fatal("candidate pool recomputed the already assembled request sources")
			}
		})
	}
}

func Test43PriorityCandidatePoolPreservesBaselineAndAIOverride(t *testing.T) {
	perspective := priorityMemoryTestContext(1)
	perspective["_priority_memory_query"] = "Mira guards the eastern archive door."
	perspective["_priority_memory_current_turn"] = 31
	assembly := priorityCandidatePoolTestAssembly(perspective)
	assembly.CanonCharacterText = `[Canonical Character States]
- entity_state: {"characters":[{"name":"Mira","aliases":["Silver Mask"],"identity_evidence_excerpt":"called the Silver Mask","current_goal":"guard the eastern archive door"}]}`
	baseline := buildPrepareTurnPriorityMemoryDeliveryPlan(&assembly, 60000, 1, "auto", nil, testPrepareTurnMemorySelectionContext(perspective))
	facts, summaries := multiAgentCandidatePool(&assembly)
	if !strings.Contains(extractionStringFromAny(baseline["final_text"]), "identity_metadata:") {
		t.Fatal("fixture did not exercise identity metadata rendering")
	}
	for _, fact := range facts {
		if fact.RenderedText != "" || len(fact.IdentityMetadata) != 0 || fact.SelectionStatus != "" {
			t.Fatalf("rendered metadata contaminated the source pool: %+v", fact)
		}
	}
	// OFF uses the ordinary Go selector even though the request has a snapshot.
	repeated := buildPrepareTurnPriorityMemoryDeliveryPlan(&assembly, 60000, 1, "auto", nil, testPrepareTurnMemorySelectionContext(perspective))
	if !reflect.DeepEqual(repeated, baseline) || assembly.Preprocessing != nil {
		t.Fatal("snapshot creation changed the OFF baseline")
	}
	selected := []string{}
	for i := len(facts) - 1; i >= 0; i-- {
		if facts[i].Lane == "subjective_relationship" {
			selected = append(selected, facts[i].CanonicalFactID)
		}
	}
	if len(selected) < 2 {
		t.Fatal("fixture lacks competing private facts")
	}
	selection := &multiAgentSelection{Contract: multiAgentContract, Candidates: facts, Summaries: summaries,
		Roles: []multiAgentRoleResult{{Role: "subjective_relationship", Source: "ai", Selection: multiAgentRecommendation{SelectedIDs: selected}}}}
	selection.captureBaseline(baseline)
	assembly.Preprocessing = selection
	aiPlan := buildPrepareTurnPriorityMemoryDeliveryPlan(&assembly, 60000, 1, "auto", nil, testPrepareTurnMemorySelectionContext(perspective))
	gotOrder := []string{}
	for _, raw := range prepareTurnMemoryLineageSlice(aiPlan["priority_items"]) {
		item := mapFromAny(raw)
		if item["selection_reason"] == "ai_recommendation" {
			gotOrder = append(gotOrder, extractionStringFromAny(item["canonical_fact_id"]))
		}
	}
	if !reflect.DeepEqual(gotOrder, selected) {
		t.Fatalf("AI source order changed: got=%v want=%v", gotOrder, selected)
	}
	for _, fact := range facts {
		if fact.Lane == "subjective_relationship" && !strings.Contains(extractionStringFromAny(aiPlan["final_text"]), fact.CompleteText) {
			t.Fatalf("AI private source text disappeared: %q", fact.CompleteText)
		}
	}
	gotFacts, gotSummaries := multiAgentCandidatePool(&assembly)
	if !reflect.DeepEqual(gotFacts, facts) || !reflect.DeepEqual(gotSummaries, summaries) ||
		!reflect.DeepEqual(selection.Candidates, facts) || !reflect.DeepEqual(selection.Summaries, summaries) {
		t.Fatal("AI selection/order/metadata rendering mutated the source pool")
	}
	selection.Roles[0].Source = "go_default"
	goPlan := buildPrepareTurnPriorityMemoryDeliveryPlan(&assembly, 60000, 1, "auto", nil, testPrepareTurnMemorySelectionContext(perspective))
	if goPlan["final_text"] != baseline["final_text"] {
		t.Fatal("no-recommendation Go delivery changed")
	}
}

func Test43PriorityCandidatePoolEmptyAndJSON(t *testing.T) {
	var unassembled prepareTurnInjectionAssembly
	facts, summaries := multiAgentCandidatePool(&unassembled)
	if facts != nil || summaries != nil {
		t.Fatal("unassembled request should not manufacture a candidate pool")
	}
	assembly := buildPrepareTurnInjectionAssemblyWithBudget(prepareTurnAssemblyInput{
		TopK:        5,
		MaxChars:    60000,
		UserInput:   "",
		Profile:     "default",
		BudgetMode:  "auto",
		Perspective: testPrepareTurnAssemblyPerspective(priorityMemoryTestContext(1)),
	})
	facts, summaries = multiAgentCandidatePool(&assembly)
	if facts == nil || summaries == nil || len(facts) != 0 || len(summaries) != 0 || assembly.Preprocessing != nil {
		t.Fatal("empty assembled request lost its empty snapshot or activated preprocessing")
	}
	perspective := priorityMemoryTestContext(1)
	perspective["_priority_memory_query"] = "Mira checks the archive door."
	populated := priorityCandidatePoolTestAssembly(perspective)
	snapshotOnly := prepareTurnInjectionAssembly{priorityCandidates: populated.priorityCandidates, priorityTurnSummaries: populated.priorityTurnSummaries}
	encoded, err := json.Marshal(snapshotOnly)
	if err != nil {
		t.Fatal(err)
	}
	zeroEncoded, err := json.Marshal(prepareTurnInjectionAssembly{})
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != string(zeroEncoded) {
		t.Fatal("internal request pool added source data to JSON")
	}
}

func Benchmark43PriorityCandidatePool(b *testing.B) {
	perspective := priorityMemoryTestContext(1)
	perspective["_priority_memory_query"] = "Mira checks the archive door."
	perspective["_priority_memory_current_turn"] = 31
	assembly := priorityCandidatePoolTestAssembly(perspective)
	b.Run("fresh_production_resolution", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			facts, _, _ := prepareTurnBuildPriorityCandidates(&assembly, extractionStringFromAny(perspective["_priority_memory_query"]), nil, 31, nil)
			_ = prepareTurnBuildPriorityTurnSummaries(facts)
		}
	})
	b.Run("request_snapshot", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			_, _ = multiAgentCandidatePool(&assembly)
		}
	})
}

func Benchmark44SupplementAssembly(b *testing.B) {
	input := priorityCandidatePoolTestInput(priorityMemoryTestContext(1))
	b.ReportAllocs()
	for b.Loop() {
		out := buildPrepareTurnInjectionAssemblyWithBudget(input)
		_, _ = multiAgentCandidatePool(&out)
	}
}

func Benchmark44SupplementCandidates(b *testing.B) {
	input := priorityCandidatePoolTestInput(priorityMemoryTestContext(1))
	b.ReportAllocs()
	for b.Loop() {
		_, _ = buildPrepareTurnSupplementCandidates(input)
	}
}

func Test44SupplementCandidatesMatchFullAssembly(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		for _, chars := range []int{0, 100, 60000} {
			for _, custom := range []bool{false, true} {
				input := priorityCandidatePoolTestInput(priorityMemoryTestContext(1))
				input.Perspective.Selection.PriorityEnabled = enabled
				input.MaxChars = chars
				if custom {
					input.BudgetMode = "custom"
					input.Budgets = map[string]int{"event_recent": 20, "subjective_relationship": 60}
				}
				full := buildPrepareTurnInjectionAssemblyWithBudget(input)
				wantFacts, wantSummaries := multiAgentCandidatePool(&full)
				gotFacts, gotSummaries := buildPrepareTurnSupplementCandidates(input)
				if !reflect.DeepEqual(gotFacts, wantFacts) || !reflect.DeepEqual(gotSummaries, wantSummaries) {
					t.Fatalf("candidate/source/order changed enabled=%v chars=%d custom=%v", enabled, chars, custom)
				}
			}
		}
	}
}

func prepareTurnPerspectiveWithNarrativeState(base map[string]any, values []store.StatusCurrentValue, activeStates []store.ActiveState) map[string]any {
	out := map[string]any{}
	for key, value := range base {
		out[key] = value
	}
	out["_narrative_current_values"] = values
	out["_narrative_active_states"] = activeStates
	return out
}
