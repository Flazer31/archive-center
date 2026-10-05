package httpapi

import (
	"crypto/sha256"
	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"strings"
	"testing"
)

func carry49Memory(link map[string]any) store.Memory {
	secret := map[string]any{"summary": "Mira keeps an amber contingency", "owner": "Mira", "secret_kind": "plan", "knowledge_scope": map[string]any{"known_by": []string{"Mira"}, "unknown_to": []string{"Oren"}, "suspected_by": []string{"Lio"}, "misinformed_by": []string{"Tess"}, "revealed_to": []string{"Paz"}}}
	for k, v := range link {
		secret[k] = v
	}
	return store.Memory{ID: 11, ChatSessionID: "carry-synthetic", TurnIndex: 2, SummaryJSON: mustCompactJSON(map[string]any{"turn_summary": "The amber cart arrives publicly", "protected_secrets": []any{secret}})}
}
func carry49Assembly(memory store.Memory, metadata map[string]any) prepareTurnInjectionAssembly {
	return buildPrepareTurnInjectionAssemblyWithBudget(prepareTurnAssemblyInput{Memories: []store.Memory{memory}, PendingThreads: []store.PendingThread{{ID: 23, ChatSessionID: "carry-synthetic", ThreadKey: "contingency-23", Description: "Mira continues the amber contingency", Status: "open", Pinned: true, SourceTurn: 9, HookMetadataJSON: mustCompactJSON(metadata)}}, UserInput: "Mira amber contingency", TopK: 5, MaxChars: 36000, ProtectedSecretBudgetChars: 4000})
}
func carry49PendingText(t *testing.T, out *prepareTurnInjectionAssembly) string {
	t.Helper()
	prepareTurnResolvePrioritySourcePool(out, "Mira amber contingency", []string{"Mira amber contingency"}, 10, nil)
	facts, _ := multiAgentCandidatePool(out)
	for _, c := range facts {
		if c.SourceTable == "pending_threads" {
			return mustCompactJSON(prepareTurnMemoryModelCandidate(c, map[string]string{c.CanonicalFactID: "F1"}))
		}
	}
	t.Fatal("pending positive control missing")
	return ""
}
func TestBoundaryCarry49ExplicitLinks(t *testing.T) {
	for _, tc := range []struct {
		name           string
		source, target map[string]any
	}{
		{"lifecycle", map[string]any{"lifecycle_key": "contingency-life"}, map[string]any{"lifecycle_key": "contingency-life"}},
		{"pending_key", map[string]any{"pending_thread_key": "contingency-23"}, nil},
		{"target_record_ref", map[string]any{"source_refs": []string{"pending_threads:23"}}, nil},
		{"fact_ref", map[string]any{"fact_id": "F41"}, map[string]any{"fact_refs": []string{"F41"}}},
		{"summary_ref", map[string]any{"source_ref": "S6"}, map[string]any{"source_refs": []string{"S6"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := carry49Memory(tc.source)
			before := mustCompactJSON(m)
			out := carry49Assembly(m, tc.target)
			text := carry49PendingText(t, &out)
			for _, field := range []string{"known_by", "unknown_to", "suspected_by", "misinformed_by", "revealed_to"} {
				if !strings.Contains(text, field) {
					t.Errorf("explicit linked candidate lost %s: %s", field, text)
				}
			}
			if before != mustCompactJSON(m) {
				t.Fatal("source mutated")
			}
		})
	}
}
func TestBoundaryCarry49NoCoTurnOrNameMatch(t *testing.T) {
	base := carry49Memory(nil)
	a := carry49Assembly(base, nil)
	want := carry49PendingText(t, &a)
	for _, meta := range []map[string]any{nil, {"source_turn": 2}, {"lifecycle_key": "other-life"}, {"subject": "Mira"}, {"source_ref": "F999"}} {
		out := carry49Assembly(base, meta)
		got := carry49PendingText(t, &out)
		if got != want {
			t.Fatal("unlinked candidate changed")
		}
		if strings.Contains(got, "unknown_to") {
			t.Fatal("unlinked same-character/word/turn became scoped")
		}
	}
}

func carry49Input(m store.Memory) prepareTurnAssemblyInput {
	in := prepareTurnAssemblyInput{Memories: []store.Memory{m}, UserInput: "Mira amber contingency", TopK: 5, MaxChars: 36000, ProtectedSecretBudgetChars: 4000, Perspective: testPrepareTurnAssemblyPerspective(priorityMemoryTestContext(5))}
	in.Perspective.Selection.Query = in.UserInput
	return in
}
func TestBoundaryCarry49DownstreamSurfaces(t *testing.T) {
	for _, kind := range []string{"storyline", "episode", "direct", "precise", "perspective"} {
		t.Run(kind, func(t *testing.T) {
			in := carry49Input(carry49Memory(map[string]any{"fact_ref": "F41"}))
			refs := mustCompactJSON(map[string]any{"source_refs": []string{"F41"}})
			switch kind {
			case "storyline":
				in.Storylines = []store.Storyline{{ID: 30, ChatSessionID: "carry-synthetic", CurrentContext: "Mira continues amber contingency", KeyPointsJSON: refs}}
			case "episode":
				in.EpisodeSummaries = []store.EpisodeSummary{{ID: 31, ChatSessionID: "carry-synthetic", FromTurn: 3, ToTurn: 5, SummaryText: "Mira considers amber contingency", OpenLoopsJSON: refs}}
			case "direct":
				in.Evidence = []store.DirectEvidence{{ID: 32, ChatSessionID: "carry-synthetic", EvidenceText: "Mira mentions amber contingency quietly", TurnAnchor: 5, LineageJSON: refs}}
				in.VectorTrace = map[string]any{"search_result": "ok", "search_results": []any{map[string]any{"id": "evidence:32", "metadata": map[string]any{"source_table": "direct_evidence_records", "source_row_id": 32, "tier": "evidence"}, "similarity": .9}}}
			case "precise", "perspective":
				unit := store.PreciseMemoryUnit{UnitID: "amber-unit", ChatSessionID: "carry-synthetic", Kind: "event", Visibility: "public", SourceTurnStart: 5, SourceTurnEnd: 5, PayloadJSON: mustCompactJSON(map[string]any{"summary": "Mira considers amber contingency", "fact_refs": []string{"F41"}})}
				semantic, ok := prepareTurnPrioritySemanticFactFromPreciseUnit(unit, .8, "synthetic")
				if !ok {
					t.Fatal("precise positive control missing")
				}
				if kind == "precise" {
					in.Perspective.Selection.SemanticFacts = []prepareTurnPrioritySemanticFact{semantic}
				} else {
					in.Perspective.CharacterSeeds = []prepareTurnPriorityFactSeed{{SourceTable: "precise_memory_units", SourceRowID: unit.UnitID, Lane: "subjective_relationship", Fact: semantic.Fact, SourceTurn: 5, ParentLineKey: semantic.Fact.Text}}
				}
			}
			before := mustCompactJSON(in)
			out := buildPrepareTurnInjectionAssemblyWithBudget(in)
			text := stringFromMap(out.MemoryDeliveryPlan, "final_text")
			if kind == "direct" {
				text = out.DirectEvidenceText
			} else {
				expectedTable := map[string]string{"storyline": "storylines", "episode": "episode_summaries", "precise": "precise_memory_units", "perspective": "precise_memory_units"}[kind]
				candidates, _ := multiAgentCandidatePool(&out)
				found := false
				for _, candidate := range candidates {
					if candidate.SourceTable == expectedTable {
						found = true
						if !strings.Contains(prepareTurnMemoryReadingText(candidate), "unknown_to") {
							t.Fatalf("%s target candidate lost named boundary", kind)
						}
					}
				}
				if !found {
					t.Fatalf("%s target candidate positive control missing", kind)
				}
			}
			if !strings.Contains(text, "unknown_to") || !strings.Contains(text, "Oren") {
				t.Fatalf("%s final boundary missing: %s", kind, text)
			}
			if before != mustCompactJSON(in) {
				t.Fatal("source input mutated")
			}
		})
	}
}
func TestBoundaryCarry49RecordObjectsAndSeparateScopes(t *testing.T) {
	m := carry49Memory(map[string]any{"source_refs": []any{map[string]any{"source_table": "pending_threads", "source_row_id": 23}}})
	parsed := parseJSONMap(m.SummaryJSON)
	second := map[string]any{"summary": "Oren secretly owns a blue map", "owner": "Oren", "secret_kind": "identity", "source_refs": []string{"pending_threads:23"}, "knowledge_scope": map[string]any{"known_by": []string{"Oren"}, "unknown_to": []string{"Mira"}}}
	parsed["protected_secrets"] = append(sliceFromAny(parsed["protected_secrets"]), second)
	m.SummaryJSON = mustCompactJSON(parsed)
	out := carry49Assembly(m, nil)
	text := carry49PendingText(t, &out)
	if !strings.Contains(text, "unknown_to") || !strings.Contains(text, "Oren") || !strings.Contains(text, "Mira") {
		t.Fatal("separate boundaries missing")
	}
	facts, _ := multiAgentCandidatePool(&out)
	for _, c := range facts {
		if c.SourceTable != "pending_threads" {
			continue
		}
		row := prepareTurnPriorityCandidateMap(c, true)
		bounds := prepareTurnMemoryLineageSlice(row["knowledge_boundaries"])
		if len(bounds) != 2 {
			t.Fatalf("distinct scopes count=%d want 2", len(bounds))
		}
		for _, b := range bounds {
			if len(stringsFromAny(mapFromAny(mapFromAny(b)["knowledge_scope"])["known_by"])) != 1 {
				t.Fatal("knowers unioned")
			}
		}
		if len(c.AllowedViewers) != 0 {
			t.Fatal("readers unioned into pending viewers")
		}
	}
}
func TestBoundaryCarry49PreprocessingNotesAndPacket(t *testing.T) {
	m := carry49Memory(map[string]any{"pending_thread_key": "contingency-23"})
	out := carry49Assembly(m, nil)
	carry49PendingText(t, &out)
	facts, summaries := multiAgentCandidatePool(&out)
	input := multiAgentInput("unresolved_goal", facts, summaries, dto.PrepareTurnRequest{}, multiAgentSettings{}, 36000, 5, nil)
	packet := multiAgentModelInput(input, 1)
	if !strings.Contains(packet, "unknown_to") {
		t.Fatal("production preprocessing packet lost named boundary")
	}
	for _, c := range facts {
		if c.SourceTable != "pending_threads" {
			continue
		}
		selection := &multiAgentSelection{Roles: []multiAgentRoleResult{{Role: "unresolved_goal", SelectionRound: 1, Selection: multiAgentRecommendation{SelectedIDs: []string{c.CanonicalFactID}, Reasons: map[string]string{c.CanonicalFactID: "The contingency matters"}}}}}
		c.SelectionStatus = "selected"
		notes := buildPrepareTurnPreprocessingNotes(selection, map[string]any{"priority_items": []any{prepareTurnPriorityCandidateMap(c, true)}}, nil)
		if !strings.Contains(stringFromMap(notes, "final_text"), "unknown_to") {
			t.Fatal("production notes lost boundary")
		}
	}
}
func TestBoundaryCarry49UnlinkedByteIdenticalAndCeilings(t *testing.T) {
	a := carry49Assembly(carry49Memory(nil), nil)
	b := carry49Assembly(carry49Memory(map[string]any{"source_refs": []string{"pending_threads:999"}}), nil)
	t.Logf("unaffected_output_sha256=%x", sha256.Sum256([]byte(stringFromMap(a.MemoryDeliveryPlan, "final_text"))))
	if stringFromMap(a.MemoryDeliveryPlan, "final_text") != stringFromMap(b.MemoryDeliveryPlan, "final_text") {
		t.Fatal("unaffected output bytes changed")
	}
	m := carry49Memory(map[string]any{"fact_id": "F41"})
	in := carry49Input(m)
	in.PendingThreads = []store.PendingThread{{ID: 23, ChatSessionID: "carry-synthetic", ThreadKey: "contingency-23", Status: "open", Pinned: true, Description: "Mira continues amber contingency", HookMetadataJSON: mustCompactJSON(map[string]any{"fact_refs": []string{"F41"}})}}
	for _, cap := range []int{80, 4000, 8000} {
		in.ProtectedSecretBudgetChars = cap
		out := buildPrepareTurnInjectionAssemblyWithBudget(in)
		budget := mapFromAny(out.MemoryDeliveryPlan["protected_secret_budget"])
		used := intFromAny(budget["used_chars"], 0)
		if used > cap {
			t.Fatalf("secret %d exceeds ceiling %d", used, cap)
		}
		if cap >= 4000 && used >= 4000 {
			t.Fatal("sparse secret fixture filled available budget")
		}
		if intFromAny(budget["candidate_count"], 0) != 1 {
			t.Fatal("carry added a protected card")
		}
		t.Logf("cap=%d used=%d selected=%d protected-card-delta=0", cap, used, intFromAny(budget["selected_count"], 0))
	}
}

func TestBoundaryCarry49SummaryKeepsFactScopesAndPublicSibling(t *testing.T) {
	in := carry49Input(carry49Memory(map[string]any{"fact_ref": "F41"}))
	row := store.Memory{ID: 22, ChatSessionID: "carry-synthetic", TurnIndex: 5, SummaryJSON: mustCompactJSON(map[string]any{"turn_summary": "Mira discusses an amber contingency and a public amber cart", "narrative_events": []any{map[string]any{"summary": "Mira considers the amber contingency", "fact_refs": []string{"F41"}}, map[string]any{"summary": "Mira parks the public amber cart"}}})}
	in.Memories = append(in.Memories, row)
	out := buildPrepareTurnInjectionAssemblyWithBudget(in)
	facts, summaries := multiAgentCandidatePool(&out)
	private, public := false, false
	for _, c := range facts {
		if c.SourceTable != "memories" || intFromAny(c.SourceRowID, 0) != 22 {
			continue
		}
		text := prepareTurnMemoryReadingText(c)
		if strings.Contains(c.CompleteText, "contingency") {
			private = true
			if !strings.Contains(text, "unknown_to") {
				t.Fatal("linked summary child lost scope")
			}
		}
		if strings.Contains(c.CompleteText, "public amber cart") {
			public = true
			if strings.Contains(text, "unknown_to") {
				t.Fatal("public sibling was scoped by its source turn")
			}
		}
	}
	if !private || !public {
		t.Fatalf("child positive controls private=%v public=%v", private, public)
	}
	summaryFound := false
	for _, s := range summaries {
		if intFromAny(s.SourceRowID, 0) == 22 {
			summaryFound = true
			input := multiAgentInput("event_recent", nil, []prepareTurnPriorityTurnSummaryCandidate{s}, dto.PrepareTurnRequest{}, multiAgentSettings{}, 36000, 5, nil)
			if !strings.Contains(multiAgentModelInput(input, 1), "unknown_to") {
				t.Fatal("summary model packet lost scope")
			}
			s.SelectionStatus = "selected"
			selection := &multiAgentSelection{Roles: []multiAgentRoleResult{{Role: "event_recent", SelectionRound: 1, Selection: multiAgentRecommendation{SelectedSummaryIDs: []string{s.SummaryID}, Reasons: map[string]string{s.SummaryID: "Remember this event"}}}}}
			notes := buildPrepareTurnPreprocessingNotes(selection, map[string]any{"turn_summary_items": []any{prepareTurnPrioritySummaryMap(s)}}, nil)
			if !strings.Contains(stringFromMap(notes, "final_text"), "unknown_to") {
				t.Fatal("summary notes lost scope")
			}
			if s.Minimum == nil || !strings.Contains(s.Minimum.Text, "unknown_to") {
				t.Fatal("whole summary route lost attributed boundary")
			}
		}
	}
	if !summaryFound {
		t.Fatal("summary positive control missing")
	}
}
func TestBoundaryCarry49SessionAndExplicitPublicControls(t *testing.T) {
	in := carry49Input(carry49Memory(map[string]any{"source_refs": []string{"pending_threads:23"}}))
	in.PendingThreads = []store.PendingThread{{ID: 23, ChatSessionID: "different-session", Status: "open", Pinned: true, Description: "Mira continues amber contingency"}}
	out := buildPrepareTurnInjectionAssemblyWithBudget(in)
	facts, _ := multiAgentCandidatePool(&out)
	for _, c := range facts {
		if c.SourceTable == "pending_threads" && strings.Contains(prepareTurnMemoryReadingText(c), "unknown_to") {
			t.Fatal("same row ID in another session acquired scope")
		}
	}
	in = carry49Input(carry49Memory(map[string]any{"source_refs": []string{"pending_threads:23"}, "public_narration_allowed": true}))
	in.PendingThreads = []store.PendingThread{{ID: 23, ChatSessionID: "carry-synthetic", Status: "open", Pinned: true, Description: "Mira continues amber contingency"}}
	out = buildPrepareTurnInjectionAssemblyWithBudget(in)
	facts, _ = multiAgentCandidatePool(&out)
	for _, c := range facts {
		if c.SourceTable == "pending_threads" && strings.Contains(prepareTurnMemoryReadingText(c), "unknown_to") {
			t.Fatal("explicit public disclosure acquired protected scope")
		}
	}
}

func TestBoundaryCarry49ExplicitRecordChain(t *testing.T) {
	in := carry49Input(carry49Memory(map[string]any{"fact_ref": "F41"}))
	in.PendingThreads = []store.PendingThread{{ID: 23, ChatSessionID: "carry-synthetic", Status: "open", Pinned: true, Description: "Mira continues amber contingency", HookMetadataJSON: mustCompactJSON(map[string]any{"fact_refs": []string{"F41"}})}}
	in.Storylines = []store.Storyline{{ID: 30, ChatSessionID: "carry-synthetic", CurrentContext: "Mira follows amber contingency", KeyPointsJSON: mustCompactJSON(map[string]any{"source_refs": []string{"pending_threads:23"}})}}
	out := buildPrepareTurnInjectionAssemblyWithBudget(in)
	facts, _ := multiAgentCandidatePool(&out)
	found := false
	for _, c := range facts {
		if c.SourceTable == "storylines" {
			found = true
			if !strings.Contains(prepareTurnMemoryReadingText(c), "unknown_to") {
				t.Fatal("explicit pending-to-storyline chain lost scope")
			}
		}
	}
	if !found {
		t.Fatal("chain storyline positive control missing")
	}
}

func TestBoundaryCarry49OwnPendingScopeAndIdentityDisclosure(t *testing.T) {
	m := carry49Memory(nil)
	p := parseJSONMap(m.SummaryJSON)
	secret := mapFromAny(sliceFromAny(p["protected_secrets"])[0])
	scope := secret["knowledge_scope"]
	own := carry49Assembly(m, map[string]any{"knowledge_scope": scope})
	if !strings.Contains(carry49PendingText(t, &own), "unknown_to") {
		t.Fatal("already scoped pending source lost its own scope")
	}
	for _, public := range []bool{false, true} {
		item := map[string]any{}
		for k, v := range secret {
			item[k] = v
		}
		item["source_refs"] = []string{"pending_threads:23"}
		if public {
			item["public_narration_allowed"] = true
		}
		parsed := map[string]any{"turn_summary": "A public amber cart arrives", "character_identity_accuracy": []any{item}}
		identity := m
		identity.SummaryJSON = mustCompactJSON(parsed)
		out := carry49Assembly(identity, nil)
		text := carry49PendingText(t, &out)
		if strings.Contains(text, "unknown_to") == public {
			t.Fatalf("identity public=%v scope carry did not honor existing disclosure policy", public)
		}
	}
}

func TestBoundaryCarry49ProjectionIndicesDoNotScopePublicSibling(t *testing.T) {
	in := carry49Input(carry49Memory(map[string]any{"fact_ref": "F41"}))
	row := store.Memory{ID: 22, ChatSessionID: "carry-synthetic", TurnIndex: 5, SummaryJSON: mustCompactJSON(map[string]any{"turn_summary": "Mira parks the public amber cart", "narrative_events": []any{
		map[string]any{"summary": "Mira privately reads a violet map", "visibility": "owner_private", "fact_refs": []string{"F41"}, "knowledge_scope": map[string]any{"known_by": []string{"Mira"}, "unknown_to": []string{"Oren"}}},
		map[string]any{"summary": "Mira parks the public amber cart", "visibility": "public"},
		map[string]any{"summary": "Mira continues amber contingency", "visibility": "public", "fact_refs": []string{"F41"}},
	}})}
	in.Memories = append(in.Memories, row)
	out := buildPrepareTurnInjectionAssemblyWithBudget(in)
	facts, _ := multiAgentCandidatePool(&out)
	found := false
	for _, c := range facts {
		if c.SourceTable == "memories" && intFromAny(c.SourceRowID, 0) == 22 && strings.Contains(c.CompleteText, "public amber cart") {
			found = true
			if strings.Contains(prepareTurnMemoryReadingText(c), "unknown_to") {
				t.Fatal("a filtered private sibling's canonical array index scoped the public occurrence")
			}
		}
	}
	if !found {
		t.Fatal("public occurrence positive control missing")
	}
}
