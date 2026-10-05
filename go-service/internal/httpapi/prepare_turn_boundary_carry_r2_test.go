package httpapi

import (
	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"strings"
	"testing"
)

func TestBoundaryCarryR2OwnPendingChain(t *testing.T) {
	in := carry49Input(store.Memory{})
	in.Memories = nil
	in.PendingThreads = []store.PendingThread{{ID: 23, ChatSessionID: "carry-synthetic", ThreadKey: "amber", Status: "open", Pinned: true, Description: "Mira continues amber contingency", HookMetadataJSON: `{"visibility":"owner_private","knowledge_scope":{"known_by":["Mira"],"unknown_to":["Oren"]}}`}}
	in.Storylines = []store.Storyline{{ID: 30, ChatSessionID: "carry-synthetic", CurrentContext: "Mira follows amber contingency", KeyPointsJSON: `{"source_refs":["pending_threads:23"]}`}}
	out := buildPrepareTurnInjectionAssemblyWithBudget(in)
	facts, _ := multiAgentCandidatePool(&out)
	pending, story := false, false
	for _, c := range facts {
		if c.SourceTable == "pending_threads" {
			pending = true
			if len(c.KnowledgeBoundaries) == 0 {
				t.Fatal("pending own-scope positive control absent")
			}
		}
		if c.SourceTable == "storylines" {
			story = true
			if len(c.KnowledgeBoundaries) == 0 {
				t.Errorf("explicitly linked storyline lost pending own scope: %s", prepareTurnMemoryReadingText(c))
			}
		}
	}
	if !pending || !story {
		t.Fatalf("candidate controls pending=%v story=%v", pending, story)
	}
	carryR2SelectedChild(t, &out, facts, "storylines")
}

func TestBoundaryCarryR2MemoryFactChain(t *testing.T) {
	in := carry49Input(carry49Memory(map[string]any{"fact_ref": "F41"}))
	in.Memories = append(in.Memories, store.Memory{ID: 22, ChatSessionID: "carry-synthetic", TurnIndex: 5, SummaryJSON: `{"narrative_events":[{"summary":"Mira continues amber contingency","fact_id":"F42","source_refs":["F41"]}]}`})
	in.PendingThreads = []store.PendingThread{{ID: 23, ChatSessionID: "carry-synthetic", ThreadKey: "amber", Status: "open", Pinned: true, Description: "Mira follows amber contingency", HookMetadataJSON: `{"fact_refs":["F42"]}`}}
	out := buildPrepareTurnInjectionAssemblyWithBudget(in)
	facts, _ := multiAgentCandidatePool(&out)
	intermediate, child := false, false
	for _, c := range facts {
		if c.SourceTable == "memories" && intFromAny(c.SourceRowID, 0) == 22 {
			intermediate = true
			if len(c.KnowledgeBoundaries) == 0 {
				t.Fatal("first-hop positive control absent")
			}
		}
		if c.SourceTable == "pending_threads" {
			child = true
			if len(c.KnowledgeBoundaries) == 0 {
				t.Errorf("explicit F41 -> F42 -> pending chain lost boundary: %s", prepareTurnMemoryReadingText(c))
			}
		}
	}
	if !intermediate || !child {
		t.Fatalf("controls intermediate=%v child=%v", intermediate, child)
	}
	carryR2SelectedChild(t, &out, facts, "pending_threads")
}

func carryR2SelectedChild(t *testing.T, out *prepareTurnInjectionAssembly, facts []prepareTurnPriorityMemoryCandidate, table string) {
	for _, c := range facts {
		if c.SourceTable != table {
			continue
		}
		out.Preprocessing = &multiAgentSelection{Candidates: facts, Roles: []multiAgentRoleResult{{Role: c.Lane, Source: "ai", SelectionRound: 1, Selection: multiAgentRecommendation{SelectedIDs: []string{c.CanonicalFactID}, Reasons: map[string]string{c.CanonicalFactID: "Continue this task"}}}}}
		plan := renderPrepareTurnPriorityMemoryDeliveryPlan(out, 36000, 10, "auto", nil, prepareTurnMemorySelectionContext{Query: "Mira amber contingency"}, nil, nil, nil, nil)
		main := stringFromMap(plan, "main_memory_text")
		t.Logf("selected-child main=%s", main)
		if !strings.Contains(main, "Mira follows amber contingency") {
			t.Fatal("selected child missing from final main")
		}
		if !strings.Contains(main, "unknown_to") {
			t.Error("selected child reached final main without unknown_to")
		}
		notes := buildPrepareTurnPreprocessingNotes(out.Preprocessing, plan, nil)
		if !strings.Contains(stringFromMap(notes, "final_text"), "unknown_to") {
			t.Error("selected child's preprocessing notes lost boundary")
		}
		packet := multiAgentModelInput(multiAgentInput(c.Lane, facts, nil, dto.PrepareTurnRequest{}, multiAgentSettings{}, 36000, 10, nil), 1)
		if !strings.Contains(packet, "unknown_to") {
			t.Error("preprocessing input lost boundary")
		}
		return
	}
	t.Fatal("selected child not found")
}

func TestBoundaryCarryR2ProtectedBudgetRoute(t *testing.T) {
	const secret = "Mira secretly plans to open the amber vault using code ORCHID-927."
	in := carry49Input(carry49Memory(map[string]any{"fact_ref": "F41", "summary": secret}))
	in.ProtectedSecretBudgetChars = 80
	in.PendingThreads = []store.PendingThread{{ID: 23, ChatSessionID: "carry-synthetic", ThreadKey: "amber", Status: "open", Pinned: true, Description: "Mira continues amber contingency", HookMetadataJSON: `{"source_refs":["F41"]}`}}
	out := buildPrepareTurnInjectionAssemblyWithBudget(in)
	budget := mapFromAny(out.MemoryDeliveryPlan["protected_secret_budget"])
	t.Logf("protected_budget=%s", mustCompactJSON(budget))
	if intFromAny(budget["selected_count"], 0) != 0 {
		t.Fatal("small protected-budget control unexpectedly selected card")
	}
	main := stringFromMap(out.MemoryDeliveryPlan, "main_memory_text")
	if strings.Contains(main, secret) {
		t.Errorf("protected fact copied into general main after protected card omitted: %s", main)
	}
	if intFromAny(budget["candidate_count"], 0) != 1 || intFromAny(budget["budget_deferred_count"], 0) != 1 || intFromAny(budget["used_chars"], -1) != 0 {
		t.Fatal("independent protected deferral changed")
	}
	if !strings.Contains(main, "Mira continues amber contingency") || !strings.Contains(main, "unknown_to") || !strings.Contains(main, "memories:11/protected_secrets/0") {
		t.Fatalf("linked child or attributed scope lost: %s", main)
	}
	facts, summaries := multiAgentCandidatePool(&out)
	for _, c := range facts {
		if c.SourceTable != "pending_threads" {
			continue
		}
		out.Preprocessing = &multiAgentSelection{Candidates: facts, Roles: []multiAgentRoleResult{{Role: c.Lane, Source: "ai", SelectionRound: 1, Selection: multiAgentRecommendation{SelectedIDs: []string{c.CanonicalFactID}, Reasons: map[string]string{c.CanonicalFactID: "Continue the contingency"}}}}}
		plan := renderPrepareTurnPriorityMemoryDeliveryPlan(&out, 36000, 10, "auto", nil, prepareTurnMemorySelectionContext{Query: in.UserInput}, nil, nil, nil, nil)
		notes := buildPrepareTurnPreprocessingNotes(out.Preprocessing, plan, nil)
		packet := multiAgentModelInput(multiAgentInput(c.Lane, []prepareTurnPriorityMemoryCandidate{c}, nil, dto.PrepareTurnRequest{}, multiAgentSettings{}, 36000, 10, nil), 1)
		for _, text := range []string{prepareTurnMemoryReadingText(c), mustCompactJSON(c.KnowledgeBoundaries), stringFromMap(plan, "main_memory_text"), stringFromMap(notes, "final_text"), packet} {
			if strings.Contains(text, secret) || strings.Contains(text, "ORCHID-927") || strings.Contains(text, "protected_fact\"") {
				t.Fatalf("protected body copied outside independent lane: %s", text)
			}
			if !strings.Contains(text, "unknown_to") {
				t.Fatalf("no-bypass path lost scope: %s", text)
			}
		}
		if intFromAny(mapFromAny(plan["protected_secret_budget"])["selected_count"], -1) != 0 {
			t.Fatal("preprocessing rerender bypassed protected deferral")
		}
	}
	for _, s := range summaries {
		if strings.Contains(mustCompactJSON(s.KnowledgeBoundaries), secret) {
			t.Fatal("summary metadata copied protected body")
		}
	}
}

// Scope originates on the existing record, without a recalled parent memory.
func TestBoundaryCarryR2OwnRecordSources(t *testing.T) {
	const scoped = `{"visibility":"owner_private","knowledge_scope":{"known_by":["Mira"],"unknown_to":["Oren"]}}`
	for _, table := range []string{"pending_threads", "storylines", "episode_summaries", "direct_evidence_records", "world_rules", "canonical_state_layers"} {
		t.Run(table, func(t *testing.T) {
			in := prepareTurnAssemblyInput{}
			switch table {
			case "pending_threads":
				in.PendingThreads = []store.PendingThread{{ID: 23, ChatSessionID: "own-record", HookMetadataJSON: scoped}}
			case "storylines":
				in.Storylines = []store.Storyline{{ID: 23, ChatSessionID: "own-record", KeyPointsJSON: scoped}}
			case "episode_summaries":
				in.EpisodeSummaries = []store.EpisodeSummary{{ID: 23, ChatSessionID: "own-record", OpenLoopsJSON: scoped}}
			case "direct_evidence_records":
				in.Evidence = []store.DirectEvidence{{ID: 23, ChatSessionID: "own-record", LineageJSON: scoped}}
			case "world_rules":
				in.WorldRules = []store.WorldRule{{ID: 23, ChatSessionID: "own-record", ValueJSON: scoped}}
			case "canonical_state_layers":
				in.CanonicalLayers = []store.CanonicalStateLayer{{ID: 23, ChatSessionID: "own-record", Content: scoped}}
			}
			// A downstream record follows this explicit record reference.
			child := store.Storyline{ID: 40, ChatSessionID: "own-record", CurrentContext: "Mira continues amber contingency", KeyPointsJSON: mustCompactJSON(map[string]any{"source_refs": []string{table + ":23"}})}
			in.Storylines = append(in.Storylines, child)
			out := prepareTurnInjectionAssembly{PriorityFactSeeds: []prepareTurnPriorityFactSeed{carryIntegrationR1Seed("storylines", 40, nil)}}
			prepareTurnCarryKnowledgeBoundaries(&out, in)
			b := out.PriorityFactSeeds[0].Fact.KnowledgeBoundaries
			if len(b) != 1 || stringFromMap(b[0], "protected_fact_ref") != table+":23" {
				t.Fatalf("own record not registered or attributed: %s", mustCompactJSON(b))
			}
			if !strings.Contains(mustCompactJSON(b), "unknown_to") {
				t.Fatal("own record scope lost")
			}
		})
	}
}

func TestBoundaryCarryR2FactAliasesCyclesAndSiblingIsolation(t *testing.T) {
	in := carry49Input(carry49Memory(map[string]any{"fact_ref": "F41"}))
	// The sibling has its own linked fact, different readers and a separate alias.
	second := carry49Memory(map[string]any{"fact_ref": "F51", "summary": "Tess holds the blue map", "owner": "Tess", "knowledge_scope": map[string]any{"known_by": []string{"Tess"}, "unknown_to": []string{"Mira"}}})
	second.ID = 12
	in.Memories = append(in.Memories, second, store.Memory{ID: 22, ChatSessionID: "carry-synthetic", TurnIndex: 5, SummaryJSON: `{"turn_summary":"Mira moves the public amber cart","narrative_events":[{"summary":"Mira continues amber contingency","fact_id":"F42","source_refs":["F41","F43"]},{"summary":"Mira follows amber contingency","fact_id":"F43","source_refs":["F42"]},{"summary":"Tess follows the blue route","fact_id":"F52","source_refs":["F51"]},{"summary":"Mira moves the public amber cart","fact_id":"F99"}]}`})
	for id, ref := range map[int64]string{23: "F43", 24: "F52", 25: "F99"} {
		in.PendingThreads = append(in.PendingThreads, store.PendingThread{ID: id, ChatSessionID: "carry-synthetic", Status: "open", Pinned: true, Description: "Mira follows amber contingency", HookMetadataJSON: mustCompactJSON(map[string]any{"fact_refs": []string{ref}})})
	}
	in.PendingThreads = append(in.PendingThreads, store.PendingThread{ID: 26, ChatSessionID: "other-session", Status: "open", Pinned: true, Description: "Mira follows amber contingency", HookMetadataJSON: `{"fact_refs":["F43"]}`})
	// Referencing the aggregate row is not referencing either acquired child
	// alias. Only original scoped facts can be addressed through that row.
	in.PendingThreads = append(in.PendingThreads, store.PendingThread{ID: 27, ChatSessionID: "carry-synthetic", Status: "open", Pinned: true, Description: "Mira follows amber contingency", HookMetadataJSON: `{"source_memory_id":22}`})
	before := mustCompactJSON(in)
	out := buildPrepareTurnInjectionAssemblyWithBudget(in)
	facts, _ := multiAgentCandidatePool(&out)
	found := map[int]bool{}
	for _, c := range facts {
		if c.SourceTable != "pending_threads" {
			continue
		}
		id := intFromAny(c.SourceRowID, 0)
		found[id] = true
		want := ""
		if id == 23 {
			want = "Mira"
		}
		if id == 24 {
			want = "Tess"
		}
		if want == "" {
			if len(c.KnowledgeBoundaries) != 0 {
				t.Fatalf("unlinked sibling/session got scopes: %s", mustCompactJSON(c.KnowledgeBoundaries))
			}
			continue
		}
		if len(c.KnowledgeBoundaries) != 1 {
			t.Fatalf("cycle/individual alias scopes count=%d row=%d", len(c.KnowledgeBoundaries), id)
		}
		readers := stringsFromAny(mapFromAny(c.KnowledgeBoundaries[0]["knowledge_scope"])["known_by"])
		if len(readers) != 1 || readers[0] != want {
			t.Fatalf("sibling scope mixed at row %d: %v", id, readers)
		}
	}
	for _, id := range []int{23, 24, 25, 26, 27} {
		if !found[id] {
			t.Fatalf("pending positive control %d absent", id)
		}
	}
	if before != mustCompactJSON(in) {
		t.Fatal("request source mutated")
	}
	// Repeat attachment converges rather than adding another copy of each scope.
	old := mustCompactJSON(out.PriorityFactSeeds)
	prepareTurnCarryKnowledgeBoundaries(&out, in)
	if old != mustCompactJSON(out.PriorityFactSeeds) {
		t.Fatal("repeat carry changed stable attributed records")
	}
}
