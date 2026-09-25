package httpapi

import (
	"fmt"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func Benchmark47CurrentStateContext(b *testing.B) {
	seeds := make([]prepareTurnPriorityFactSeed, 500)
	values := make([]store.StatusCurrentValue, 100)
	for i := range seeds {
		seeds[i] = prepareTurnPriorityFactSeed{SourceTable: "memories", Fact: prepareTurnPriorityMemoryFact{Text: fmt.Sprintf("Traveler%d carried the old compass through the northern gate and described the former expedition route.", i%100)}}
	}
	for i := range values {
		values[i] = store.StatusCurrentValue{ID: int64(i + 1), StatusKey: narrativeStateStatusKey, WriteState: "current", OwnerID: fmt.Sprint(i), ValueJSON: mustCompactJSON(map[string]any{"subject": fmt.Sprintf("Traveler%d", i), "state_slot": "inventory", "value": "The compass has been returned."})}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out := prepareTurnInjectionAssembly{PriorityFactSeeds: append([]prepareTurnPriorityFactSeed(nil), seeds...)}
		prepareTurnAttachCurrentStateContext(&out, values, nil)
	}
}

func Test47RecalledSourceCarriesStoredCurrentStateIntoFactsAndSummary(t *testing.T) {
	for _, tc := range []struct{ name, subject, slot, old, current, query string }{
		{"object", "야항패", "ownership", "야항패는 푸른 귀환 도구다. 세린이 소지한다.", "야항패는 리오에게 양도되었으며 세린의 가방에는 없다.", "세린은 예전의 푸른 귀환 도구를 꺼내려고 한다."},
		{"person", "Kael", "affiliation", "Kael was the silver-haired captain at the northern gate.", "Kael left the guard and joined the merchants.", "We seek the former silver-haired captain."},
		{"place", "청연정", "condition", "청연정은 호숫가의 푸른 지붕 정자다.", "청연정은 홍수로 파괴되어 출입이 통제되어 있다.", "일행은 호수 옆 푸른 지붕 아래에서 쉬려 한다."},
		{"multiword", "Brass Compass", "ownership", "The Brass Compass was the expedition's compass, carried by Mira.", "The Brass Compass now belongs to Rowan.", "The expedition needs its compass again."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := &turnRecordingStore{}
			save46LifecycleFixture(t, st, 30, map[string]any{"state_claims": []any{map[string]any{"subject": tc.subject, "state_slot": tc.slot, "value": tc.current, "transition": "change", "confidence": .9, "evidence_excerpt": tc.current}}}, "They review what has changed. "+tc.current+" The conversation continues.")
			if len(st.returnStatusCurrent) == 0 {
				t.Fatal("production save did not supply current state")
			}
			memory := store.Memory{ID: 701, ChatSessionID: "lifecycle-46", TurnIndex: 3, Importance: .8, SummaryJSON: mustCompactJSON(map[string]any{"narrative_events": []any{map[string]any{"actor": "세린", "event": tc.old, "visibility": "public"}}})}
			in := prepareTurnAssemblyInput{Memories: []store.Memory{memory}, TopK: 5, MaxChars: 18000, UserInput: tc.query, Profile: "default", BudgetMode: "auto", Perspective: testPrepareTurnAssemblyPerspective(priorityMemoryTestContext(5))}
			in.Perspective.Selection.Query = tc.query
			plain := buildPrepareTurnInjectionAssemblyWithBudget(in)
			in.Perspective.NarrativeValues = st.returnStatusCurrent
			before := mustCompactJSON(in)
			out := buildPrepareTurnInjectionAssemblyWithBudget(in)
			facts, summaries := multiAgentCandidatePool(&out)
			base, _ := multiAgentCandidatePool(&plain)
			byID := map[string]prepareTurnPriorityMemoryCandidate{}
			for _, f := range base {
				byID[f.CanonicalFactID] = f
			}
			for _, f := range facts {
				if f.SourceRef != "memories:701" {
					continue
				}
				if !strings.Contains(prepareTurnMemoryReadingText(f), tc.current) {
					t.Fatalf("source reading lost current state: %s", prepareTurnMemoryReadingText(f))
				}
				if p, ok := byID[f.CanonicalFactID]; !ok || p.CompleteText != f.CompleteText || p.SourceTurn != f.SourceTurn || p.OriginalScore != f.OriginalScore {
					t.Fatal("link changed source identity, time or original score")
				}
			}
			if len(facts) == 0 || len(summaries) != 1 {
				t.Fatalf("invalid fixture facts=%d summaries=%d base=%d", len(facts), len(summaries), len(base))
			}
			selected := out
			selected.Preprocessing = &multiAgentSelection{Candidates: facts, Summaries: summaries, Roles: []multiAgentRoleResult{{Role: "event_recent", Source: "ai", Selection: multiAgentRecommendation{SelectedSummaryIDs: []string{summaries[0].SummaryID}}}}}
			plan := finalizePrepareTurnPriorityMemoryDeliveryPlan(&selected, in.MaxChars, 5, "auto", nil, in.Perspective.Selection)
			text := extractionStringFromAny(plan["final_text"])
			if !containsAll(text, tc.old, tc.current, "state evidence", "source turn 30") {
				t.Fatalf("summary-only selection lost complete bundle: %s", text)
			}
			assert45Budget(t, plan, in.MaxChars)
			if before != mustCompactJSON(in) {
				t.Fatal("source input mutated")
			}
		})
	}
}

func Test47CurrentSourceContextDoesNotChainOrDisclosePrivateState(t *testing.T) {
	makeState := func(id int64, subject, scope, value string) store.StatusCurrentValue {
		return store.StatusCurrentValue{ID: id, StatusKey: narrativeStateStatusKey, WriteState: "current", OwnerID: subject, ValueJSON: mustCompactJSON(map[string]any{"subject": subject, "state_slot": "location", "claim_scope": scope, "value": value})}
	}
	for _, reverse := range []bool{false, true} {
		values := []store.StatusCurrentValue{makeState(1, "Ann", "objective", "Ann visits Bob."), makeState(2, "Bob", "objective", "Bob lives in the remote fortress."), makeState(3, "Ann", "secret", "HIDDEN_VAULT")}
		if reverse {
			values[0], values[1] = values[1], values[0]
		}
		out := prepareTurnInjectionAssembly{PriorityFactSeeds: []prepareTurnPriorityFactSeed{{Fact: prepareTurnPriorityMemoryFact{Text: "Ann carried the red umbrella."}}, {Fact: prepareTurnPriorityMemoryFact{Text: "Anna carried a green umbrella."}}}}
		prepareTurnAttachCurrentStateContext(&out, values, nil)
		text := mustCompactJSON(out.PriorityFactSeeds[0].Fact.Reading.Parts)
		if !strings.Contains(text, "Ann visits Bob.") || strings.Contains(text, "remote fortress") || strings.Contains(text, "HIDDEN_VAULT") {
			t.Fatalf("linked unrelated/private state: %s", text)
		}
		if out.PriorityFactSeeds[1].Fact.Reading != nil {
			t.Fatal("name prefix linked a different subject")
		}
	}
}

func Test47StoredStateSharedAcrossSourcesRetainsIndependentHistory(t *testing.T) {
	const current = "Mira owns no compass now; it belongs to Rowan."
	for _, mode := range []string{"auto", "custom"} {
		t.Run(mode, func(t *testing.T) {
			out := prepareTurnInjectionAssembly{PriorityFactSeeds: []prepareTurnPriorityFactSeed{
				{Lane: "event_recent", SourceTable: "memories", SourceRowID: 10, SourceTurn: 3, Importance: .8, ImportancePresent: true, Fact: prepareTurnPriorityMemoryFact{Text: "Mira received the compass at the harbor."}},
				{Lane: "world_state", SourceTable: "world_rules", SourceRowID: 20, SourceTurn: 4, Importance: .8, ImportancePresent: true, Fact: prepareTurnPriorityMemoryFact{Text: "Mira may use the compass only beneath open skies."}},
			}}
			state := store.StatusCurrentValue{ID: 31, StatusKey: narrativeStateStatusKey, WriteState: "current", OwnerScope: "entity", OwnerID: "mira-compass", SourceTurn: 12, ValueJSON: mustCompactJSON(map[string]any{"subject": "Mira", "state_slot": "inventory", "value": current}), EvidenceJSON: mustCompactJSON(map[string]any{"evidence_excerpt": "Mira handed the compass to Rowan.", "source_revision": "observed-12"})}
			prepareTurnAttachCurrentStateContext(&out, []store.StatusCurrentValue{state}, nil)
			before := mustCompactJSON(out.PriorityFactSeeds)
			for _, seed := range out.PriorityFactSeeds {
				if !strings.Contains(mustCompactJSON(seed.Fact.Reading), "observed-12") {
					t.Fatal("source reading lost the internal evidence revision")
				}
			}
			plan := buildPrepareTurnPriorityMemoryDeliveryPlan(&out, 18000, 5, mode, nil, prepareTurnMemorySelectionContext{Query: "Mira compass", CurrentTurn: 300})
			text := extractionStringFromAny(plan["final_text"])
			if strings.Count(text, current) != 1 || !containsAll(text, "received the compass", "only beneath open skies", "source turn 3", "source turn 4", "source turn 12", "Mira handed the compass to Rowan.") {
				t.Fatalf("current state duplicated or independent evidence lost: %s", text)
			}
			if strings.Contains(text, "observed-12") {
				t.Fatal("final delivery leaked the internal evidence revision")
			}
			assert45Budget(t, plan, 18000)
			if before != mustCompactJSON(out.PriorityFactSeeds) {
				t.Fatal("packing modified source readings")
			}
		})
	}
}

func Test47CurrentContextPreservesRestoredOriginAndFutureValidity(t *testing.T) {
	state := store.StatusCurrentValue{ID: 31, StatusKey: narrativeStateStatusKey, WriteState: "current", OwnerScope: "entity", OwnerID: "mira-balance", SourceTurn: 90,
		ValueJSON:    `{"subject":"Mira","state_slot":"balance","value":"The credit will be available next month.","validity":{"valid_from":{"date":"2902-02-01"}}}`,
		EvidenceJSON: `{"source_revision":"restored-90","repair_recorded_turn":90,"restored_source_turn":4,"restored_evidence":{"source_revision":"original-4","evidence_excerpt":"The clerk promised a credit next month."}}`}
	out := prepareTurnInjectionAssembly{PriorityFactSeeds: []prepareTurnPriorityFactSeed{{SourceTable: "memories", SourceTurn: 3, Fact: prepareTurnPriorityMemoryFact{Text: "Mira discussed the account with the clerk."}}}}
	before := mustCompactJSON(state)
	prepareTurnAttachCurrentStateContext(&out, []store.StatusCurrentValue{state}, map[string]any{"absolute": map[string]any{"date": "2902-01-03"}})
	text := mustCompactJSON(out.PriorityFactSeeds[0].Fact.Reading)
	if !containsAll(text, "source turn 4", "original-4", "restored-90", "valid_from", "2902-02-01", "next month") || strings.Contains(text, "source turn 90;") {
		t.Fatalf("restoration rewrote origin or validity: %s", text)
	}
	if before != mustCompactJSON(state) || out.PriorityFactSeeds[0].SourceTurn != 3 {
		t.Fatal("state or source time mutated")
	}
}

func Test47CurrentContextPreviewDoesNotConsumeRejectedBudget(t *testing.T) {
	seen := map[prepareTurnMemoryPartIdentity]bool{}
	a, b := prepareTurnMemoryReadingLayout{currentParts: seen}, prepareTurnMemoryReadingLayout{currentParts: seen}
	part := prepareTurnMemoryFormPart{Key: "@current/entity/mira/balance", Text: "The account has 11 coins; status_current_values:31."}
	row := prepareTurnMemoryReadingRow{Group: "old", Header: "older source", Parts: []prepareTurnMemoryFormPart{part}}
	a.preview(row) // Rejected reservation; no apply.
	row.Group = "new"
	edit := b.preview(row)
	if !strings.Contains(edit.row.rendered, part.Text) {
		t.Fatal("uncommitted preview consumed state")
	}
	b.apply(edit)
	row.Group = "later"
	if next := a.preview(row); strings.Contains(next.row.rendered, part.Text) || next.delta != 0 {
		t.Fatalf("already delivered state was charged again: %+v", next)
	}
}

func Test47OldValidStateAndHistoricalEventRemainTogether(t *testing.T) {
	for _, tc := range []struct{ name, subject, slot, history, current, nowQuery, pastQuery string }{
		{"balance", "Mira", "balance", "Mira received 40 silver coins from a loan repayment at the harbor.", "Mira has 11 silver coins available after paying the ferry.", "Mira checks how many silver coins are available before paying.", "Mira recalls the loan repayment at the harbor."},
		{"inventory", "Rowan", "inventory", "Rowan found three blue crystals inside the abandoned tower.", "Rowan has one blue crystal remaining after trading two away.", "Rowan checks the remaining blue crystals before trading.", "Rowan recalls finding the blue crystals inside the abandoned tower."},
	} {
		for _, mode := range []string{"auto", "custom"} {
			for _, query := range []string{tc.nowQuery, tc.pastQuery} {
				t.Run(tc.name+"/"+mode+"/"+query, func(t *testing.T) {
					out := prepareTurnInjectionAssembly{PriorityFactSeeds: []prepareTurnPriorityFactSeed{{Lane: "event_recent", SourceTable: "memories", SourceRowID: 10, SourceTurn: 3, Importance: .8, ImportancePresent: true, Fact: prepareTurnPriorityMemoryFact{Text: tc.history}}}}
					for i := 0; i < 60; i++ {
						out.PriorityFactSeeds = append(out.PriorityFactSeeds, prepareTurnPriorityFactSeed{Lane: "event_recent", SourceTable: "memories", SourceRowID: int64(100 + i), SourceTurn: 300 + i, Importance: .9, ImportancePresent: true, Fact: prepareTurnPriorityMemoryFact{Text: fmt.Sprintf("Traveler%d inspected the new red banners on the palace wall.", i)}})
					}
					state := store.StatusCurrentValue{ID: 31, StatusKey: narrativeStateStatusKey, WriteState: "current", OwnerScope: "entity", OwnerID: tc.subject, SourceTurn: 8, ValueJSON: mustCompactJSON(map[string]any{"subject": tc.subject, "state_slot": tc.slot, "value": tc.current}), EvidenceJSON: mustCompactJSON(map[string]any{"evidence_excerpt": tc.current})}
					prepareTurnAttachCurrentStateContext(&out, []store.StatusCurrentValue{state}, nil)
					selection := prepareTurnMemorySelectionContext{Query: query, CurrentTurn: 400}
					plan := buildPrepareTurnPriorityMemoryDeliveryPlan(&out, 6000, 1, mode, nil, selection)
					text := extractionStringFromAny(plan["final_text"])
					if !containsAll(text, tc.history, tc.current, "source turn 3", "source turn 8") {
						t.Fatalf("old-but-valid state or historical transaction lost: %s", text)
					}
					assert45Budget(t, plan, 6000)
				})
			}
		}
	}
}

func Test47DirectEvidenceDuplicateKeepsUndeliveredCurrentContext(t *testing.T) {
	const history = "Mira received the blue return device at the harbor."
	const current = "Mira no longer carries the blue return device; she gave it to Rowan."
	for _, mode := range []string{"auto", "custom"} {
		t.Run(mode, func(t *testing.T) {
			out := prepareTurnInjectionAssembly{LatestDirectEvidenceText: "- " + history, PriorityFactSeeds: []prepareTurnPriorityFactSeed{{Lane: "event_recent", SourceTable: "memories", SourceRowID: 10, SourceTurn: 3, Importance: .8, ImportancePresent: true, Fact: prepareTurnPriorityMemoryFact{Text: history}}}}
			state := store.StatusCurrentValue{ID: 31, StatusKey: narrativeStateStatusKey, WriteState: "current", OwnerScope: "entity", OwnerID: "Mira", SourceTurn: 8, ValueJSON: mustCompactJSON(map[string]any{"subject": "Mira", "state_slot": "inventory", "value": current})}
			prepareTurnAttachCurrentStateContext(&out, []store.StatusCurrentValue{state}, nil)
			plan := buildPrepareTurnPriorityMemoryDeliveryPlan(&out, 6000, 1, mode, nil, prepareTurnMemorySelectionContext{Query: "Mira blue return device", CurrentTurn: 400})
			text := extractionStringFromAny(plan["final_text"])
			if !containsAll(text, history, current, "source turn 8") || strings.Count(text, current) != 1 {
				t.Fatalf("same quotation discarded a distinct current-state bundle: %s", text)
			}
			assert45Budget(t, plan, 6000)
		})
	}
}
