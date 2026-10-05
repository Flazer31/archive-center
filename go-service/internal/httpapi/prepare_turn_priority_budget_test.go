package httpapi

import (
	"fmt"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestAuthorityRankingProductionAssembly(t *testing.T) {
	const sid = "authority-ranking"
	memory := func(id int64, turn int, claim, known string) store.Memory {
		return store.Memory{ID: id, ChatSessionID: sid, TurnIndex: turn, Importance: .8, SummaryJSON: mustCompactJSON(map[string]any{
			"narrative_events": []any{map[string]any{"event": fmt.Sprintf("Mira visits archive room %d.", id)}},
			"turn_summary":     fmt.Sprintf("Mira visits archive room %d.", id), "protected_secrets": []any{map[string]any{
				"owner": "Mira", "secret_kind": "route", "secret_summary": claim,
				"disclosure_policy": "owner_private_until_revealed", "knowledge_scope": map[string]any{"known_by": []string{known}},
			}},
		})}
	}
	memories := []store.Memory{memory(1, 1, "old hidden route", "Mira"), memory(2, 2, "current hidden route", "Mira"), memory(3, 3, "old hidden route", "Mira"), memory(4, 4, "current hidden route", "Rowan")}
	original := mustCompactJSON(memories)
	// Input/DB order and importance do not stand in for the observed scores.
	scores := []float64{.95, .9, .4, .6}
	hits := []map[string]any{}
	for i, m := range memories {
		hits = append(hits, map[string]any{"id": fmt.Sprintf("memory:%s:%d", sid, m.ID), "tier": "memory", "source_table": "memories", "source_row_id": fmt.Sprint(m.ID), "similarity": scores[i]})
	}
	evidence := []store.DirectEvidence{
		{ID: 1, ChatSessionID: sid, TurnAnchor: 1, EvidenceText: "Mira remembers the earlier quotation."},
		{ID: 2, ChatSessionID: sid, TurnAnchor: 2, EvidenceText: "Mira reads the relevant quotation."},
	}
	for _, mode := range []string{"auto", "custom"} {
		a := buildPrepareTurnInjectionAssemblyWithBudget(prepareTurnAssemblyInput{
			Memories: memories, Evidence: evidence, TopK: 20, MaxChars: 18000, UserInput: "Mira visits the archive and remembers the hidden route.", Profile: "default", BudgetMode: mode,
			VectorTrace: map[string]any{"search_result": "ok", "memory_search_result": "ok", "memory_search_results": hits, "search_results": []map[string]any{
				{"id": "evidence:" + sid + ":1", "tier": "evidence", "source_table": "direct_evidence_records", "source_row_id": "1", "similarity": .4},
				{"id": "evidence:" + sid + ":2", "tier": "evidence", "source_table": "direct_evidence_records", "source_row_id": "2", "similarity": .9},
			}},
			Perspective: &prepareTurnAssemblyPerspective{Selection: prepareTurnMemorySelectionContext{PriorityEnabled: true, MaxItems: 1, Query: "Mira hidden route", CurrentTurn: 5}},
		})
		for _, raw := range prepareTurnMemoryLineageSlice(a.MemoryDeliveryPlan["classes"]) {
			lane := mapFromAny(raw)
			key, text := stringFromMap(lane, "key"), stringFromMap(lane, "text")
			switch key {
			case "protected_secret":
				if strings.Count(text, "old hidden route") != 1 || !strings.Contains(text, "turn 3") || strings.Contains(text, "turn 1]") || strings.Count(text, "current hidden route") != 2 {
					t.Fatalf("%s: latest exact secret or distinct scope lost: %s", mode, text)
				}
				if strings.Index(text, "turn 2]") > strings.Index(text, "turn 4]") || strings.Index(text, "turn 4]") > strings.Index(text, "turn 3]") {
					t.Fatalf("%s: authority cards are not in existing-score order: %s", mode, text)
				}
			case "direct_evidence":
				if !containsAll(text, evidence[0].EvidenceText, evidence[1].EvidenceText) || strings.Index(text, evidence[1].EvidenceText) > strings.Index(text, evidence[0].EvidenceText) {
					t.Fatalf("%s: direct evidence ignored existing vector score: %s", mode, text)
				}
			default:
				continue
			}
			if lane["selection_policy"] != "authority_exempt" || intFromAny(lane["selected_count"], 0) <= 1 {
				t.Fatalf("%s: authority became ordinary K competition: %#v", mode, lane)
			}
		}
	}
	if mustCompactJSON(memories) != original {
		t.Fatal("delivery changed stored memories")
	}
}

func TestAuthorityRankingStableMissingScoresAndBudget(t *testing.T) {
	for _, lane := range []string{"protected_secret", "direct_evidence"} {
		t.Run(lane, func(t *testing.T) {
			items := []string{"- archive missing first", "- archive lower scored", "- archive higher scored", "- archive equal higher", "- archive missing last"}
			out := &prepareTurnInjectionAssembly{authoritySelectionScores: map[string]float64{items[1]: 0, items[2]: .8, items[3]: .8}}
			if lane == "protected_secret" {
				out.ProtectedMemoryText = strings.Join(items, "\n")
			} else {
				out.DirectEvidenceText = strings.Join(items, "\n")
			}
			// Exactly the two equal top-scoring cards fit this existing custom cap.
			// Direct evidence keeps its recall (input) order instead.
			want := []string{items[2], items[3]}
			full := []string{items[2], items[3], items[1], items[0], items[4]}
			if lane == "direct_evidence" {
				want, full = items[:2], items
			} else {
				want = full // Protected cards now use their independent budget in custom mode too.
			}
			cap := len([]rune(makePrepareTurnSection("["+prepareTurnMemoryDeliveryTitles[lane]+"]", want)))
			plan := buildPrepareTurnPriorityMemoryDeliveryPlan(out, cap, 1, "custom", map[string]int{lane: cap}, prepareTurnMemorySelectionContext{Query: "archive"})
			if got := stringFromMap(plan, "final_text"); got != makePrepareTurnSection("["+prepareTurnMemoryDeliveryTitles[lane]+"]", want) {
				t.Fatalf("%s: score order/budget got %q", lane, got)
			}
			want = full
			cap = len([]rune(makePrepareTurnSection("["+prepareTurnMemoryDeliveryTitles[lane]+"]", want)))
			plan = buildPrepareTurnPriorityMemoryDeliveryPlan(out, cap, 1, "auto", nil, prepareTurnMemorySelectionContext{Query: "archive"})
			if got := stringFromMap(plan, "final_text"); got != makePrepareTurnSection("["+prepareTurnMemoryDeliveryTitles[lane]+"]", want) {
				t.Fatalf("%s: stable scoreless tail got %q", lane, got)
			}
			for _, raw := range prepareTurnMemoryLineageSlice(plan["classes"]) {
				class := mapFromAny(raw)
				if lane == "direct_evidence" && stringFromMap(class, "key") == lane && intFromAny(class["borrowed_chars"], 0) <= 0 {
					t.Fatal("fixture did not exercise authority borrowing")
				}
			}
		})
	}
}

func TestAuthorityCardRelevanceProductionAssembly(t *testing.T) {
	const query = "silver compass vault"
	secret := func(claim string) map[string]any {
		return map[string]any{"owner": "Mira", "secret_kind": "plan", "secret_summary": claim,
			"disclosure_policy": "owner_private_until_revealed", "knowledge_scope": map[string]any{"known_by": []string{"Mira"}, "unknown_to": []string{"Rowan"}}}
	}
	memories := []store.Memory{
		{ID: 1, ChatSessionID: "cards", TurnIndex: 1, Importance: .9, SummaryJSON: mustCompactJSON(map[string]any{
			"turn_summary": "Mira has two private plans.", "protected_secrets": []any{secret("orchard picnic tomorrow"), secret(query)},
		})},
		{ID: 2, ChatSessionID: "cards", TurnIndex: 2, Importance: .1, SummaryJSON: mustCompactJSON(map[string]any{
			"turn_summary": "Mira remembers a compass.", "protected_secrets": []any{secret("silver compass location")},
		})},
	}
	evidence := []store.DirectEvidence{
		{ID: 1, ChatSessionID: "cards", TurnAnchor: 1, EvidenceText: "orchard picnic tomorrow"},
		{ID: 2, ChatSessionID: "cards", TurnAnchor: 2, EvidenceText: query},
	}
	for _, mode := range []string{"auto", "custom"} {
		a := buildPrepareTurnInjectionAssemblyWithBudget(prepareTurnAssemblyInput{
			Memories: memories, Evidence: evidence, UserInput: "Mira searches for the " + query, TopK: 20, MaxChars: 18000, Profile: "default", BudgetMode: mode,
			VectorTrace: map[string]any{"search_result": "ok", "memory_search_result": "ok", "memory_search_results": []map[string]any{
				{"id": "memory:cards:1", "tier": "memory", "source_table": "memories", "source_row_id": "1", "similarity": .95},
				{"id": "memory:cards:2", "tier": "memory", "source_table": "memories", "source_row_id": "2", "similarity": .2},
			}, "search_results": []map[string]any{
				{"id": "evidence:cards:1", "tier": "evidence", "source_table": "direct_evidence_records", "source_row_id": "1", "similarity": .95},
				{"id": "evidence:cards:2", "tier": "evidence", "source_table": "direct_evidence_records", "source_row_id": "2", "similarity": .2},
			}},
			Perspective: &prepareTurnAssemblyPerspective{Selection: prepareTurnMemorySelectionContext{PriorityEnabled: true, MaxItems: 1, Query: query, CurrentTurn: 3}},
		})
		seen := map[string]bool{}
		for _, raw := range prepareTurnMemoryLineageSlice(a.MemoryDeliveryPlan["classes"]) {
			lane := mapFromAny(raw)
			key, text := stringFromMap(lane, "key"), stringFromMap(lane, "text")
			if key != "protected_secret" && key != "direct_evidence" {
				continue
			}
			seen[key] = true
			want := []string{query, "orchard picnic tomorrow"}
			if key == "protected_secret" {
				want = []string{query, "silver compass location"}
				if strings.Contains(text, "orchard picnic tomorrow") {
					t.Fatal("unrelated secret filled spare independent budget")
				}
			}
			last := -1
			for _, claim := range want {
				at := strings.Index(text, claim)
				if at <= last {
					t.Fatalf("%s/%s: card relevance did not separate same-row facts and override row scores: %s", mode, key, text)
				}
				last = at
			}
			if lane["selection_policy"] != "authority_exempt" || intFromAny(lane["selected_count"], 0) != len(want) {
				t.Fatalf("authority cards became K-limited: %#v", lane)
			}
		}
		if len(seen) != 2 {
			t.Fatal("authority lane missing")
		}
	}
}

func TestAuthorityCardRelevanceCurrentQueryAndNamedScope(t *testing.T) {
	// Card-relevance ranking applies to protected cards only; direct evidence
	// order is covered by TestAuthorityRankingStableMissingScoresAndBudget.
	for _, field := range []string{"subject", "owner", "known_by", "unknown_to", "suspected_by", "misinformed_by", "revealed_to", "policy", "kind"} {
		for _, lane := range []string{"protected_secret"} {
			t.Run(field+"/"+lane, func(t *testing.T) {
				unrelated := "- [turn 1] orchard picnic; owner=Rowan"
				related := "- [turn 1] silver vault location; " + field + "=Mira"
				out := &prepareTurnInjectionAssembly{authoritySelectionScores: map[string]float64{unrelated: .9}}
				if lane == "protected_secret" {
					out.ProtectedMemoryText = unrelated + "\n" + related
				} else {
					out.DirectEvidenceText = unrelated + "\n" + related
				}
				selected := related
				if field != "subject" && field != "owner" {
					selected = "" // Knowledge metadata is not scene relevance.
				}
				want := makePrepareTurnSection("["+prepareTurnMemoryDeliveryTitles[lane]+"]", []string{selected})
				cap := len([]rune(makePrepareTurnSection("["+prepareTurnMemoryDeliveryTitles[lane]+"]", []string{related})))
				// Earlier context must not replace the current query. Only a subject
				// or owner match may outrank the row score; knowledge names remain rendered.
				plan := buildPrepareTurnPriorityMemoryDeliveryPlan(out, cap, 1, "custom", map[string]int{lane: cap}, prepareTurnMemorySelectionContext{Query: "Mira", QuerySet: []string{"Mira", "orchard picnic"}})
				if got := stringFromMap(plan, "final_text"); got != want {
					t.Fatalf("card content/owner must match the current scene: %q", got)
				}
			})
		}
	}
}

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
		ProtectedMemoryText:    "- Mira archive recollection remains known only to Rook and must not become public character knowledge.",
		Counts:                 map[string]any{},
	}
}

func TestAutoMemoryBudgetOrdinarySharesAfterDirectEvidence(t *testing.T) {
	for _, source := range []string{"off", "ai", "no_recommendation", "call_failed_without_recommendation", "mixed"} {
		t.Run(source, func(t *testing.T) {
			a := autoBudgetEvidenceFixture()
			// Quotations are delivered first; enough total room remains for ordinary shares.
			a.DirectEvidenceText = strings.Join(strings.Split(a.DirectEvidenceText, "\n")[:10], "\n")
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
			wantCandidates := 11 + len(prepareTurnMemoryLineageSlice(plan["priority_items"])) + len(prepareTurnMemoryLineageSlice(plan["turn_summary_items"]))
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
			plan := buildPrepareTurnPriorityMemoryDeliveryPlan(a, 4000, 5, "auto", nil, prepareTurnMemorySelectionContext{Query: "archive"})
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
	for i := 1; i <= 40; i++ {
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
	if len([]rune(a.DirectEvidenceText)) <= 3500 || len([]rune(a.DirectEvidenceText)) >= 18000 {
		t.Fatalf("fixture must exceed the old direct share while leaving total room for the summary: chars=%d", len([]rune(a.DirectEvidenceText)))
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

// The unchanged ordinary scorer counts matching query forms; authority ranking
// must count the single card word only once. Both authority lanes call the real
// allocator, so replacing its scorer with test.26 reverses this ordering.
func TestAuthorityContentScoringInflectionsProduction(t *testing.T) {
	query := "세인 세인은 세인과 silver vault"
	name := "- [turn 1] 세인"
	content := "- [turn 2] silver vault"
	for _, lane := range []string{"protected_secret"} {
		for _, mode := range []string{"auto", "custom"} {
			out := &prepareTurnInjectionAssembly{authoritySelectionScores: map[string]float64{name: .9, content: .1}}
			if lane == "protected_secret" {
				out.ProtectedMemoryText = name + "\n" + content
			} else {
				out.DirectEvidenceText = name + "\n" + content
			}
			want := makePrepareTurnSection("["+prepareTurnMemoryDeliveryTitles[lane]+"]", []string{content, name})
			cap := len([]rune(want))
			plan := buildPrepareTurnPriorityMemoryDeliveryPlan(out, cap, 1, mode, map[string]int{lane: cap}, prepareTurnMemorySelectionContext{Query: query})
			if got := stringFromMap(plan, "final_text"); got != want {
				t.Fatalf("%s/%s: repeated query forms inflated one card word: %q", lane, mode, got)
			}
		}
	}
	if prepareTurnPriorityRelevance(query, "세인") <= prepareTurnPriorityRelevance(query, "silver vault") {
		t.Fatal("ordinary candidate scorer was changed")
	}
}

func TestAuthorityContentScoringProjectionAndMatching(t *testing.T) {
	claim := "sealed orchard; keep the key hidden"
	subject := "silver vault"
	item := map[string]any{"secret_summary": claim, "subject": []string{subject}, "owner": "Rowan", "secret_kind": "plan", "disclosure_policy": "owner_private_until_revealed", "knowledge_scope": map[string]any{}}
	scope := mapFromAny(item["knowledge_scope"])
	for _, field := range []string{"known_by", "unknown_to", "suspected_by", "misinformed_by", "revealed_to"} {
		scope[field] = []string{"Mira", "Rowan"}
	}
	item["knowledge_scope"] = scope
	card := prepareTurnProtectedCardFromParsed(map[string]any{"protected_secrets": []any{item}}, "")
	original := card
	if got := prepareTurnAuthorityCardContent(card); got != strings.ReplaceAll(claim, "; ", " ")+" "+subject {
		t.Fatalf("content projection = %q", got)
	}
	for _, query := range []string{"Mira", "Rowan", "plan", "owner_private_until_revealed", "known_by", "subject"} {
		if got := prepareTurnAuthorityRelevanceScorer(query)(card); got != 0 {
			t.Fatalf("metadata %q contributes %v", query, got)
		}
	}
	if prepareTurnAuthorityRelevanceScorer(subject)(card) != 1 {
		t.Fatal("subject content lost")
	}
	if card != original || !strings.Contains(card, "known_by=Mira, Rowan") {
		t.Fatal("rendered scope changed")
	}
	queries := prepareTurnRecallTerms("세인 세인은 세인과")
	for _, cardText := range []string{"세인", "세인 세인 세인", "세인 세인은 세인과", "세인에게서"} {
		if got := prepareTurnAuthorityOverlapCount(queries, cardText, true); got != 1 {
			t.Fatalf("one card word counted %d times", got)
		}
	}
	for _, pair := range [][2]string{{"세인", "세인은"}, {"세인은", "세인"}, {"세인과", "세인"}, {"세인은", "세인과"}, {"Mira", "Miras"}} {
		want := 0
		if prepareTurnPriorityInflectedNonASCIIOverlapCount([]string{pair[0]}, pair[1]) > 0 {
			want = 1
		}
		if got := prepareTurnAuthorityOverlapCount([]string{pair[0]}, pair[1], false); got != want {
			t.Fatalf("existing inflection rule changed: %v", pair)
		}
	}
	// A derived form can itself end in a suffix. It still belongs to one word;
	// flattening forms and stripping again would count this original twice.
	if got := prepareTurnAuthorityOverlapCount(prepareTurnRecallTerms("모몬 모몬가"), "모몬가가", true); got != 1 {
		t.Fatalf("one original word's derived forms counted %d times", got)
	}
}
