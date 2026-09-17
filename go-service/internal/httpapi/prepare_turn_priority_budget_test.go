package httpapi

import (
	"fmt"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func autoBudgetEvidenceFixture() *prepareTurnInjectionAssembly {
	lines := []string{}
	for i := 0; i < 160; i++ {
		lines = append(lines, fmt.Sprintf("- [vector, turn %d] Archive conversation %03d: %s", i+1, i, strings.Repeat("quoted detail ", 12)))
	}
	return &prepareTurnInjectionAssembly{
		DirectEvidenceText:     strings.Join(lines, "\n"),
		ActualMemoryText:       "- Mira already visited the archive and returned the borrowed key to Rook before the winter festival.",
		CharacterObjectiveText: "- Mira is the archive curator recognized by all the staff and responsible for the sealed collection.",
		PersonaText:            "- Mira remembers Rook as the trusted colleague who helped her catalogue the archive through the winter.",
		CanonWorldText:         "- archive_door status: sealed after the inspection performed by the curator and the winter watch.",
		PendingThreadText:      "- Mira still needs to deliver the archive inventory to Rook after the next scheduled council meeting.",
		ProtectedMemoryText:    "- Rook's private recollection remains known only to Rook and must not become public character knowledge.",
		Counts:                 map[string]any{},
	}
}

func TestAutoMemoryBudgetEvidenceCannotStarveOtherLanes(t *testing.T) {
	for _, source := range []string{"off", "ai", "no_recommendation", "call_failed_without_recommendation", "mixed"} {
		t.Run(source, func(t *testing.T) {
			a := autoBudgetEvidenceFixture()
			selection := prepareTurnMemorySelectionContext{Query: "Mira archive", CurrentTurn: 170}
			baseline := buildPrepareTurnPriorityMemoryDeliveryPlan(a, 18000, 5, "auto", nil, selection)
			plan := baseline
			if source != "off" {
				m := &multiAgentSelection{Candidates: a.priorityCandidates, Summaries: a.priorityTurnSummaries}
				m.captureBaseline(baseline)
				for _, lane := range multiAgentRoles {
					r := multiAgentRoleResult{Role: lane, Source: "go", Reason: source}
					if source == "ai" || (source == "mixed" && lane == "event_recent") {
						r.Source = "ai"
						for _, c := range m.Candidates {
							if c.Lane == lane {
								r.Selection.SelectedIDs = append(r.Selection.SelectedIDs, c.CanonicalFactID)
							}
						}
					}
					m.Roles = append(m.Roles, r)
				}
				a.Preprocessing = m
				plan = buildPrepareTurnPriorityMemoryDeliveryPlan(a, 18000, 5, "auto", nil, selection)
			}
			for _, raw := range prepareTurnMemoryLineageSlice(plan["classes"]) {
				lane := mapFromAny(raw)
				if intFromAny(lane["selected_count"], 0) == 0 {
					t.Errorf("populated lane %s starved: used=%v cap=%v", lane["key"], lane["used_chars"], lane["reserved_chars"])
				}
			}
			if source == "off" && intFromAny(plan["final_delivery_chars"], 0) > 18000 {
				t.Fatal("Go delivery exceeded the whole memory budget")
			}
			if boolFromAny(plan["source_rows_mutated"]) {
				t.Fatal("budget selection changed stored memory")
			}
			wantCandidates := 161 + len(prepareTurnMemoryLineageSlice(plan["priority_items"])) + len(prepareTurnMemoryLineageSlice(plan["turn_summary_items"]))
			if intFromAny(plan["candidate_count"], 0) != wantCandidates {
				t.Fatal("second allocation pass counted the same evidence again")
			}
			excluded := 0
			for _, count := range plan["exclusion_reasons"].(map[string]int) {
				excluded += count
			}
			if excluded != intFromAny(plan["excluded_count"], 0) {
				t.Fatal("intermediate budget deferrals survived after those entries were delivered")
			}
		})
	}
}

func TestAutoMemoryBudgetUnusedSharesRemainAvailable(t *testing.T) {
	for _, lane := range []string{"direct_evidence", "world_state", "protected_secret"} {
		t.Run(lane, func(t *testing.T) {
			text := strings.TrimSpace("Complete archive record " + strings.Repeat("with unchanged details ", 90))
			a := &prepareTurnInjectionAssembly{Counts: map[string]any{}}
			switch lane {
			case "direct_evidence":
				a.DirectEvidenceText = "- " + text
			case "world_state":
				a.CanonWorldText = "- " + text
			case "protected_secret":
				a.ProtectedMemoryText = "- " + text
			}
			plan := buildPrepareTurnPriorityMemoryDeliveryPlan(a, 4000, 5, "auto", nil, prepareTurnMemorySelectionContext{})
			final := extractionStringFromAny(plan["final_text"])
			if !strings.Contains(final, text) {
				t.Fatal("an entry larger than its initial share lost otherwise idle space or was truncated")
			}
			if len([]rune(final)) > 4000 {
				t.Fatal("borrowing exceeded the global budget")
			}
			if intFromAny(plan["candidate_count"], 0) != intFromAny(plan["selected_count"], 0)+intFromAny(plan["excluded_count"], 0) {
				t.Fatal("budget retry inflated candidate accounting")
			}
		})
	}
}

func TestAutoMemoryBudgetSharesScaleWithEnvelope(t *testing.T) {
	for _, cap := range []int{0, 12, 200, 18000, 36000} {
		caps, _ := prepareTurnPriorityDeliveryCaps(cap, "auto", nil)
		total := 0
		for _, lane := range prepareTurnMemoryDeliveryOrder {
			total += caps[lane]
			if caps[lane] < 0 || caps[lane] > cap {
				t.Fatalf("invalid initial share %s=%d for %d", lane, caps[lane], cap)
			}
		}
		if total > cap {
			t.Fatalf("automatic initial shares sum to %d for envelope %d", total, cap)
		}
	}
}

func TestAutoMemoryBudgetDeferredDuplicateAccounting(t *testing.T) {
	text := strings.TrimSpace(strings.Repeat("The recorded archive conversation continues ", 24))
	a := &prepareTurnInjectionAssembly{DirectEvidenceText: "- " + text + "\n- " + text, Counts: map[string]any{}}
	plan := buildPrepareTurnPriorityMemoryDeliveryPlan(a, 4000, 5, "auto", nil, prepareTurnMemorySelectionContext{})
	if strings.Count(extractionStringFromAny(plan["final_text"]), text) != 1 {
		t.Fatal("the borrowed evidence was lost or duplicated")
	}
	excluded := 0
	for _, count := range plan["exclusion_reasons"].(map[string]int) {
		excluded += count
	}
	if excluded != intFromAny(plan["excluded_count"], 0) {
		t.Fatal("deduplication in the borrowing pass was absent from final diagnostics")
	}
}

func TestAutoMemoryBudgetProductionAssemblyKeepsRetrievedSummary(t *testing.T) {
	const sid = "auto-budget-retrieved"
	const summary = "Mira already visited the archive and returned Rook's brass key during the winter festival."
	evidence := []store.DirectEvidence{}
	hits := []map[string]any{}
	for i := 1; i <= 160; i++ {
		evidence = append(evidence, store.DirectEvidence{ID: int64(i), ChatSessionID: sid, TurnAnchor: i, SourceTurnStart: i, SourceTurnEnd: i,
			EvidenceText: fmt.Sprintf("Archive quotation %03d %s", i, strings.Repeat("a detail of the archive conversation ", 8))})
		hits = append(hits, map[string]any{"id": fmt.Sprintf("evidence:%s:%d", sid, i), "tier": "evidence", "source_table": "direct_evidence_records", "source_row_id": fmt.Sprint(i), "similarity": .9})
	}
	a := buildPrepareTurnInjectionAssemblyWithBudget(prepareTurnAssemblyInput{
		Memories: []store.Memory{{ID: 900, ChatSessionID: sid, TurnIndex: 11, Importance: 9, SummaryJSON: mustCompactJSON(map[string]any{"turn_summary": summary})}},
		Evidence: evidence, TopK: len(evidence), MaxChars: 18000, UserInput: "Mira recalls her archive visit and the brass key.", Profile: "default", BudgetMode: "auto",
		VectorTrace: map[string]any{"search_result": "ok", "memory_search_result": "ok", "search_results": hits, "memory_search_results": []map[string]any{
			{"id": "memory:" + sid + ":900", "tier": "memory", "source_table": "memories", "source_row_id": "900", "similarity": .99},
		}},
		Perspective: &prepareTurnAssemblyPerspective{Selection: prepareTurnMemorySelectionContext{PriorityEnabled: true, MaxItems: 5, Query: "Mira archive visit brass key", CurrentTurn: 170}},
	})
	if len([]rune(a.DirectEvidenceText)) <= 18000 {
		t.Fatalf("fixture did not exercise evidence overflow: chars=%d", len([]rune(a.DirectEvidenceText)))
	}
	final := extractionStringFromAny(a.MemoryDeliveryPlan["final_text"])
	if !strings.Contains(final, summary) || !strings.Contains(final, "[turn 11]") {
		t.Fatal("retrieved summary or its source turn disappeared between hydration and final delivery")
	}
	if !strings.Contains(final, "[vector, turn") || len([]rune(final)) > 18000 {
		t.Fatal("evidence disappeared or the global budget was exceeded")
	}
	lane := prepareTurnPayloadLane("long_term_memory", "Long-term Memory Context", final, 18000, true, nil)
	if lane["text"] != final {
		t.Fatal("payload changed the selected memory")
	}
}

func TestAutoMemoryBudgetBorrowKeepsCandidateOrderAndCustomCaps(t *testing.T) {
	long := strings.TrimSpace("Mira archive " + strings.Repeat("unchanged historical detail ", 50))
	short := "Mira archive remains accessible."
	for _, mode := range []string{"auto", "custom"} {
		a := &prepareTurnInjectionAssembly{CanonWorldText: "- " + long + "\n- " + short, Counts: map[string]any{}}
		plan := buildPrepareTurnPriorityMemoryDeliveryPlan(a, 4000, 5, mode, map[string]int{"world_state": 700}, prepareTurnMemorySelectionContext{Query: "Mira archive"})
		final := extractionStringFromAny(plan["final_text"])
		if !strings.Contains(final, short) {
			t.Fatal("small whole memory disappeared")
		}
		if mode == "custom" {
			if strings.Contains(final, long) {
				t.Fatal("automatic borrowing changed the explicit custom cap")
			}
			continue
		}
		if !strings.Contains(final, long) {
			t.Fatal("long entry was not reconsidered with unused shares")
		}
		previous := -1
		for _, raw := range prepareTurnMemoryLineageSlice(plan["priority_items"]) {
			item := mapFromAny(raw)
			if item["selection_status"] != "selected" {
				continue
			}
			text := extractionStringFromAny(item["rendered_text"])
			if text == "" {
				text = extractionStringFromAny(item["complete_text"])
			}
			position := strings.Index(final, text)
			if position <= previous {
				t.Fatalf("borrowing reordered the original candidate ranking: previous=%d current=%d text=%q final=%q", previous, position, item["rendered_text"], final)
			}
			previous = position
		}
	}
}
