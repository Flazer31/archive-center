package httpapi

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestFactCardsBoundStateAndUnchangedReadingScores(t *testing.T) {
	history := "Mira's pass permits entry only with Rowan; it never permits entry alone."
	out := prepareTurnInjectionAssembly{PriorityFactSeeds: []prepareTurnPriorityFactSeed{{Lane: "event_recent", SourceTable: "memories", SourceRowID: 9, SourceTurn: 2, Fact: prepareTurnPriorityMemoryFact{Text: history, EntitySurface: "Mira", StateSlot: "access"}}}}
	states := []store.StatusCurrentValue{
		{ID: 20, StatusKey: narrativeStateStatusKey, OwnerScope: "entity", OwnerID: "mira-access", SourceTurn: 5, WriteState: "current", ValueJSON: `{"subject":"Mira","state_slot":"access","value":"The pass is suspended until the council meets.","validity":{"valid_from":{"date":"2902-02-01"}}}`, EvidenceJSON: `{"evidence_excerpt":"Entry is suspended until the council meets."}`},
		{ID: 21, StatusKey: narrativeStateStatusKey, OwnerScope: "entity", OwnerID: "mira-coat", SourceTurn: 3, WriteState: "current", ValueJSON: `{"subject":"Mira","state_slot":"clothing","value":"UNRELATED_COAT"}`, EvidenceJSON: mustCompactJSON(map[string]any{"evidence_excerpt": strings.Repeat("A description of the old coat. ", 100)})},
	}
	before := mustCompactJSON(states)
	prepareTurnAttachCurrentStateContext(&out, states, nil)
	legacy := out
	legacy.PriorityFactSeeds = append([]prepareTurnPriorityFactSeed(nil), out.PriorityFactSeeds...)
	reading := *out.PriorityFactSeeds[0].Fact.Reading
	reading.Parts = append([]prepareTurnMemoryPart(nil), reading.Parts...)
	for i := range reading.Parts {
		reading.Parts[i].ReferenceOnly = false
	}
	legacy.PriorityFactSeeds[0].Fact.Reading = &reading
	card, _, _ := prepareTurnBuildPriorityCandidates(&out, "Mira pass council coat", nil, 9, nil)
	full, _, _ := prepareTurnBuildPriorityCandidates(&legacy, "Mira pass council coat", nil, 9, nil)
	if len(card) != 1 || len(full) != 1 {
		t.Fatalf("fixture candidates: %d / %d", len(card), len(full))
	}
	if card[0].CanonicalFactID != full[0].CanonicalFactID || card[0].FinalScore != full[0].FinalScore || card[0].Minimum.Meaning != full[0].Minimum.Meaning || prepareTurnMemoryReadingText(card[0]) != prepareTurnMemoryReadingText(full[0]) {
		t.Fatal("delivery formatting changed fact identity, ranking or preprocessing reading")
	}
	render := func(c prepareTurnPriorityMemoryCandidate) string {
		layout := prepareTurnMemoryReadingLayout{}
		return layout.apply(layout.preview(prepareTurnMemoryCandidateRow(c, 0, "")))
	}
	text, oldText := render(card[0]), render(full[0])
	if !containsAll(text, history, "pass is suspended", "until the council", "source turn 5", "valid_from", "2902-02-01") || strings.Contains(text, "UNRELATED_COAT") {
		t.Fatalf("incomplete or unrelated card: %s", text)
	}
	if utf8.RuneCountInString(text) >= utf8.RuneCountInString(oldText) || !strings.Contains(oldText, "UNRELATED_COAT") {
		t.Fatal("negative control did not reproduce the large subject-wide context")
	}
	if mustCompactJSON(states) != before {
		t.Fatal("stored state mutated")
	}
}

func TestFactCardsSameSourceSharedAcrossLanesAndPreview(t *testing.T) {
	reading := &prepareTurnMemoryContext{Path: "/rule/0", Parts: []prepareTurnMemoryPart{{Key: "/rule/0/condition", Value: "동행인이 있을 때만 입장; 혼자서는 금지 🌙"}}}
	candidates := []prepareTurnPriorityMemoryCandidate{
		{CanonicalFactID: "first", SourceRef: "memory:1", SourceOccurrence: "turn2-revision", SourceTurn: 2, Lane: "event_recent", Visibility: "public_projection", Reading: reading},
		{CanonicalFactID: "second", SourceRef: "memory:1", SourceOccurrence: "turn2-revision", SourceTurn: 2, Lane: "world_state", Visibility: "public_projection", Reading: reading},
		{CanonicalFactID: "different", SourceRef: "memory:1", SourceOccurrence: "turn3-revision", SourceTurn: 3, Lane: "event_recent", Visibility: "public_projection", Reading: reading},
		{CanonicalFactID: "private", SourceRef: "memory:1", SourceOccurrence: "turn2-revision", SourceTurn: 2, Lane: "subjective_relationship", Visibility: "private", PerspectiveOwner: "Mira", Reading: reading},
	}
	prepareTurnBuildReadingForms(candidates, func(string) float64 { return .5 }, nil)
	shared := map[prepareTurnMemoryPartIdentity]bool{}
	first, second := prepareTurnMemoryReadingLayout{currentParts: shared}, prepareTurnMemoryReadingLayout{currentParts: shared}
	row := prepareTurnMemoryCandidateRow(candidates[0], 0, "")
	preview := first.preview(row)
	if len(shared) != 0 {
		t.Fatal("a rejected preview consumed shared text")
	}
	otherPreview := second.preview(prepareTurnMemoryCandidateRow(candidates[1], 0, ""))
	if otherPreview.delta == 0 {
		t.Fatal("uncommitted source was treated as already delivered")
	}
	text := first.apply(preview)
	if preview.delta != utf8.RuneCountInString(text)+1 {
		t.Fatal("Unicode cost differs from rendered body")
	}
	duplicate := second.preview(prepareTurnMemoryCandidateRow(candidates[1], 0, ""))
	if duplicate.delta != 0 || second.apply(duplicate) != "" {
		t.Fatal("same source context was rendered/charged again across lanes")
	}
	for _, c := range candidates[2:] {
		layout := prepareTurnMemoryReadingLayout{currentParts: shared}
		edit := layout.preview(prepareTurnMemoryCandidateRow(c, 0, ""))
		if edit.delta <= 0 || !strings.Contains(layout.apply(edit), "혼자서는 금지") {
			t.Fatal("independent occurrence or perspective merged")
		}
	}
}

func TestFactCardsPublicEvidenceSurvivesDerivedSummary(t *testing.T) {
	const publicQuote = "The west relay cannot reach any outside station; no reply is possible."
	const privateQuote = "PRIVATE_TUNNEL_CODE"
	memory := store.Memory{ID: 31, ChatSessionID: "cards", TurnIndex: 4, SummaryJSON: mustCompactJSON(map[string]any{
		"turn_summary":      "The meeting ended. " + publicQuote + " " + privateQuote,
		"narrative_events":  []any{map[string]any{"event": "The meeting ended."}},
		"evidence_excerpts": []string{publicQuote, privateQuote},
		"protected_secrets": []any{map[string]any{"owner": "Mira", "secret_kind": "plan", "summary": privateQuote, "evidence_excerpt": privateQuote, "disclosure_policy": "owner_private_until_revealed"}},
	})}
	before := memory.SummaryJSON
	p := newPrepareTurnRequestPreparation(&prepareTurnAssemblyCommon{})
	seeds := p.publicMemorySeeds(memory)
	public := mustCompactJSON(seeds)
	if !strings.Contains(public, publicQuote) || strings.Contains(public, privateQuote) {
		t.Fatal("admitted quote lost, or canonical private text copied into public candidates")
	}
	out := prepareTurnInjectionAssembly{PriorityFactSeeds: seeds}
	plan := buildPrepareTurnPriorityMemoryDeliveryPlan(&out, 18000, 8, "auto", nil, prepareTurnMemorySelectionContext{Query: "outside relay station", CurrentTurn: 5})
	if !strings.Contains(stringFromMap(plan, "final_text"), publicQuote) || strings.Contains(stringFromMap(plan, "final_text"), privateQuote) {
		t.Fatal("public evidence did not survive production selection/rendering")
	}
	if memory.SummaryJSON != before {
		t.Fatal("canonical source mutated")
	}
}

func TestFactCardsWriterSecretKeepsContentAndNamedKnowledge(t *testing.T) {
	secret := map[string]any{"protected_secrets": []any{map[string]any{
		"owner": "Mira", "subject": []string{"archive route"}, "secret_kind": "plan", "summary": "The northern tunnel is the escape route.",
		"disclosure_policy": "owner_private_until_revealed", "knowledge_scope": map[string]any{"known_by": []string{"Mira", "Rowan"}, "unknown_to": []string{"Visitor"}, "suspected_by": []string{"Guard"}, "misinformed_by": []string{"Clerk"}, "revealed_to": []string{"Pilot"}},
	}}}
	memory := store.Memory{SummaryJSON: mustCompactJSON(secret)}
	for _, pov := range []string{"", "Mira", "Visitor", "Guard", "Clerk"} {
		line, _ := prepareTurnMemoryInjectionLineText(memory, "ignored summary", nil, map[string]any{"current_pov": pov})
		for _, required := range []string{"Author-only", "not public character knowledge", "northern tunnel", "known_by=Mira, Rowan", "unknown_to=Visitor", "suspected_by=Guard", "misinformed_by=Clerk", "revealed_to=Pilot"} {
			if !strings.Contains(line, required) {
				t.Fatalf("POV %q lost %q: %s", pov, required, line)
			}
		}
	}
	missing := map[string]any{"protected_secrets": []any{map[string]any{"summary": "The empty scope is not universal knowledge.", "disclosure_policy": "owner_private_until_revealed"}}}
	line, _ := prepareTurnMemoryInjectionLineText(store.Memory{SummaryJSON: mustCompactJSON(missing)}, "", nil)
	if !strings.Contains(line, "empty scope") || strings.Contains(line, "known_by=") || strings.Contains(line, "unknown_to=") || strings.Contains(line, "owner=") {
		t.Fatalf("missing metadata was invented: %s", line)
	}
	if mustCompactJSON(secret) != memory.SummaryJSON {
		t.Fatal("knowledge scope mutated")
	}
}

func TestFactCardsEvidenceDoesNotReplaceLegacySummary(t *testing.T) {
	memory := store.Memory{SummaryJSON: `{"turn_summary":"The library stays closed until Mira returns.","evidence_excerpts":["The western gate has opened."]}`}
	facts, _ := prepareTurnPriorityFactsFromMemory(memory)
	text := mustCompactJSON(facts)
	if !containsAll(text, "library stays closed", "until Mira returns", "western gate has opened") {
		t.Fatal("adding source evidence replaced the existing unstructured summary")
	}
}
