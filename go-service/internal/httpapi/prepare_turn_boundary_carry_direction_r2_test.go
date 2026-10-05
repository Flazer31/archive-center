package httpapi

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func carryDirectionR2Input() prepareTurnAssemblyInput {
	in := carry49Input(store.Memory{ID: 22, ChatSessionID: "carry-synthetic", TurnIndex: 5, SummaryJSON: `{"turn_summary":"Mira amber contingency catalog records public progress","narrative_events":[{"summary":"Mira amber contingency public cart","fact_id":"F42"},{"summary":"Mira amber contingency public lantern","fact_id":"F99"}]}`})
	in.PendingThreads = []store.PendingThread{
		{ID: 23, ChatSessionID: "carry-synthetic", ThreadKey: "amber-private", Status: "open", Pinned: true, Description: "Mira continues amber contingency", HookMetadataJSON: `{"fact_id":"F43","source_refs":["F42"],"visibility":"owner_private","owner":"Mira","knowledge_scope":{"known_by":["Mira"],"unknown_to":["Oren"]}}`},
		{ID: 24, ChatSessionID: "carry-synthetic", ThreadKey: "blue-private", Status: "open", Pinned: true, Description: "Tess follows the blue route", HookMetadataJSON: `{"fact_id":"F44","source_refs":["F42"],"visibility":"owner_private","owner":"Tess","knowledge_scope":{"known_by":["Tess"],"unknown_to":["Mira"]}}`},
		{ID: 25, ChatSessionID: "carry-synthetic", ThreadKey: "public-parent-child", Status: "open", Pinned: true, Description: "Mira checks public cart wheels", HookMetadataJSON: `{"fact_id":"F45","source_refs":["F42"]}`},
		{ID: 26, ChatSessionID: "carry-synthetic", ThreadKey: "unlinked", Status: "open", Pinned: true, Description: "Mira continues amber contingency"},
		{ID: 27, ChatSessionID: "other-session", ThreadKey: "other", Status: "open", Pinned: true, Description: "Mira continues amber contingency", HookMetadataJSON: `{"source_refs":["F43"]}`},
	}
	in.Storylines = []store.Storyline{
		{ID: 30, ChatSessionID: "carry-synthetic", CurrentContext: "Mira follows amber contingency", KeyPointsJSON: `{"source_refs":["pending_threads:23"]}`},
		{ID: 31, ChatSessionID: "carry-synthetic", CurrentContext: "Mira amber contingency next step follows Tess blue route", KeyPointsJSON: `{"source_refs":["F44"]}`},
	}
	return in
}

func carryDirectionR2Main(t *testing.T, out *prepareTurnInjectionAssembly, c prepareTurnPriorityMemoryCandidate) string {
	t.Helper()
	out.Preprocessing = &multiAgentSelection{Candidates: []prepareTurnPriorityMemoryCandidate{c}, Roles: []multiAgentRoleResult{{Role: c.Lane, Source: "ai", SelectionRound: 1, Selection: multiAgentRecommendation{SelectedIDs: []string{c.CanonicalFactID}, Reasons: map[string]string{c.CanonicalFactID: "Continue this recorded task"}}}}}
	plan := renderPrepareTurnPriorityMemoryDeliveryPlan(out, 36000, 10, "auto", nil, prepareTurnMemorySelectionContext{Query: "Mira amber contingency"}, nil, nil, nil, nil)
	main := stringFromMap(plan, "main_memory_text")
	if !strings.Contains(main, strings.TrimPrefix(c.CompleteText, "status=open; ")) {
		t.Fatalf("selected production candidate absent: %s", main)
	}
	return main
}

func TestBoundaryCarryDirectionR2OwnScopeAndPublicBytes(t *testing.T) {
	in := carryDirectionR2Input()
	before := mustCompactJSON(in)
	out := buildPrepareTurnInjectionAssemblyWithBudget(in)
	facts, _ := multiAgentCandidatePool(&out)
	// Remove both private children and their actual descendants. The original
	// parent, public sibling, unlinked and other-session records stay identical.
	control := carryDirectionR2Input()
	control.PendingThreads = control.PendingThreads[2:]
	control.Storylines = nil
	plain := buildPrepareTurnInjectionAssemblyWithBudget(control)
	plainFacts, _ := multiAgentCandidatePool(&plain)
	key := func(c prepareTurnPriorityMemoryCandidate) string {
		return fmt.Sprintf("%s:%v/%s", c.SourceTable, c.SourceRowID, c.CompleteText)
	}
	plainByKey := map[string]prepareTurnPriorityMemoryCandidate{}
	for _, c := range plainFacts {
		plainByKey[key(c)] = c
	}
	seen := map[string]bool{}
	publicCount, scopedCount := 0, 0
	for _, c := range facts {
		want, source := "", ""
		switch c.SourceTable {
		case "pending_threads":
			switch intFromAny(c.SourceRowID, 0) {
			case 23:
				want, source = "Mira", "pending_threads:23"
			case 24:
				want, source = "Tess", "pending_threads:24"
			}
		case "storylines":
			want, source = "Mira", "pending_threads:23"
			if intFromAny(c.SourceRowID, 0) == 31 {
				want, source = "Tess", "pending_threads:24"
			}
		case "memories":
			if c.CompleteText != "Mira amber contingency public cart" && c.CompleteText != "Mira amber contingency public lantern" {
				continue
			}
		default:
			continue
		}
		seen[key(c)] = true
		main := carryDirectionR2Main(t, &out, c)
		if want != "" {
			scopedCount++
			if len(c.KnowledgeBoundaries) != 1 {
				t.Errorf("%s must carry only its parent/own scope: %s", key(c), mustCompactJSON(c.KnowledgeBoundaries))
				continue
			}
			b := c.KnowledgeBoundaries[0]
			readers := stringsFromAny(mapFromAny(b["knowledge_scope"])["known_by"])
			if len(readers) != 1 || readers[0] != want || stringFromMap(b, "protected_fact_ref") != source {
				t.Errorf("wrong independently attributed scope at %s: %s", key(c), mustCompactJSON(b))
			}
			if !strings.Contains(main, mustCompactJSON(b)) || !strings.Contains(main, "unknown_to") {
				t.Errorf("final main lost scope at %s: %s", key(c), main)
			}
			continue
		}
		publicCount++
		p, ok := plainByKey[key(c)]
		if !ok {
			t.Fatalf("no-private-child positive control missing: %s", key(c))
		}
		if len(c.KnowledgeBoundaries) != 0 || strings.Contains(main, "linked fact knowledge boundary") {
			t.Errorf("public record acquired child scope: %s", key(c))
		}
		if prepareTurnMemoryReadingText(c) != prepareTurnMemoryReadingText(p) {
			t.Errorf("public reading changed vs no-private-child control: %s", key(c))
		}
		if wantMain := carryDirectionR2Main(t, &plain, p); main != wantMain {
			t.Errorf("public final main bytes changed at %s\ngot=%s\nwant=%s", key(c), main, wantMain)
		}
		t.Logf("public %s final_sha256=%x", key(c), sha256.Sum256([]byte(main)))
	}
	if publicCount != len(plainByKey) || publicCount != 5 || scopedCount != len(in.Storylines)+2 || len(seen) != 9 {
		t.Fatalf("production controls missing: public=%d plain=%d scoped=%d seen=%d", publicCount, len(plainByKey), scopedCount, len(seen))
	}
	if before != mustCompactJSON(in) {
		t.Fatal("source inputs mutated")
	}
}

func TestBoundaryCarryDirectionR2OriginalExplicitBindings(t *testing.T) {
	for _, tc := range []struct {
		name           string
		source, target map[string]any
	}{
		{"same_fact_S6", map[string]any{"source_ref": "S6"}, map[string]any{"source_refs": []string{"S6"}}},
		{"same_fact_declared_ref", map[string]any{"source_ref": "F41"}, map[string]any{"fact_ref": "F41"}},
		{"target_pending", map[string]any{"source_refs": []string{"pending_threads:23"}}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := carry49Assembly(carry49Memory(tc.source), tc.target)
			prepareTurnResolvePrioritySourcePool(&out, "Mira amber contingency", []string{"Mira amber contingency"}, 10, nil)
			facts, _ := multiAgentCandidatePool(&out)
			found := false
			for _, c := range facts {
				if c.SourceTable != "pending_threads" {
					continue
				}
				found = true
				if len(c.KnowledgeBoundaries) != 1 || !strings.Contains(carryDirectionR2Main(t, &out, c), "unknown_to") {
					t.Fatal("original classified source binding lost in candidate/final main")
				}
			}
			if !found {
				t.Fatal("binding positive control missing")
			}
		})
	}
}

func TestBoundaryCarryDirectionR2ProtectedDerivedFact(t *testing.T) {
	for _, identityField := range []string{"fact_id", "canonical_fact_id", "fact_ref"} {
		t.Run(identityField, func(t *testing.T) {
			in := carry49Input(carry49Memory(map[string]any{identityField: "F41", "source_refs": []string{"F42"}}))
			in.Memories = append(in.Memories, store.Memory{ID: 22, ChatSessionID: "carry-synthetic", TurnIndex: 5, SummaryJSON: `{"narrative_events":[{"summary":"Mira amber contingency public cart","fact_id":"F42"}]}`})
			in.PendingThreads = []store.PendingThread{
				{ID: 23, ChatSessionID: "carry-synthetic", Status: "open", Pinned: true, Description: "Mira follows amber contingency", HookMetadataJSON: `{"source_refs":["F41"]}`},
				{ID: 24, ChatSessionID: "carry-synthetic", Status: "open", Pinned: true, Description: "Mira checks public cart wheels", HookMetadataJSON: `{"source_refs":["F42"]}`},
			}
			before := mustCompactJSON(in)
			out := buildPrepareTurnInjectionAssemblyWithBudget(in)
			facts, _ := multiAgentCandidatePool(&out)
			control := in
			control.Memories = in.Memories[1:]
			plain := buildPrepareTurnInjectionAssemblyWithBudget(control)
			plainFacts, _ := multiAgentCandidatePool(&plain)
			seen := map[int]bool{}
			for _, c := range facts {
				id := intFromAny(c.SourceRowID, 0)
				if c.SourceTable != "pending_threads" && !(c.SourceTable == "memories" && id == 22) {
					continue
				}
				seen[id] = true
				main := carryDirectionR2Main(t, &out, c)
				if id == 23 {
					if len(c.KnowledgeBoundaries) != 1 || !strings.Contains(main, "unknown_to") {
						t.Fatal("the protected fact's own descendant lost its boundary")
					}
					continue
				}
				if len(c.KnowledgeBoundaries) != 0 || strings.Contains(main, "linked fact knowledge boundary") {
					t.Errorf("derived fact scope flowed back to public source or sibling row %d", id)
				}
				found := false
				for _, p := range plainFacts {
					if p.SourceTable == c.SourceTable && p.SourceRowID == c.SourceRowID && p.CompleteText == c.CompleteText {
						found = true
						if prepareTurnMemoryReadingText(c) != prepareTurnMemoryReadingText(p) || main != carryDirectionR2Main(t, &plain, p) {
							t.Errorf("public row %d changed from the no-private-derived-fact control", id)
						}
					}
				}
				if !found {
					t.Fatalf("public control row %d absent", id)
				}
			}
			if len(seen) != len(in.PendingThreads)+1 || before != mustCompactJSON(in) {
				t.Fatal("positive control missing or source mutated")
			}
		})
	}
}
