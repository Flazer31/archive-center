package httpapi

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestMemoryAuditSecretIdentifiesOwnerAndKnowledgeBoundary(t *testing.T) {
	secret := map[string]any{"protected_secrets": []any{map[string]any{
		"owner": "Mira", "subject": []string{"archive route"}, "secret_kind": "plan",
		"summary": "Mira will use the northern tunnel", "disclosure_policy": "owner_private_until_revealed",
		"knowledge_scope": map[string]any{"known_by": []string{"Mira", "Rowan"}, "suspected_by": []string{"Guard"}, "unknown_to": []string{"Visitor"}, "misinformed_by": []string{"Clerk"}},
	}}}
	before := mustCompactJSON(secret)
	for _, pov := range []string{"Mira", "Rowan", "Guard", "Visitor", "Clerk", ""} {
		t.Run(pov, func(t *testing.T) {
			guard := prepareTurnProtectedMemoryGuardFromParsed(secret, map[string]any{"current_pov": pov})
			for _, name := range []string{"owner=Mira", "known_by=Mira, Rowan", "suspected_by=Guard", "unknown_to=Visitor", "misinformed_by=Clerk"} {
				if !strings.Contains(guard.LineText, name) {
					t.Errorf("guidance lost %q: %s", name, guard.LineText)
				}
			}
			knows := pov == "Mira" || pov == "Rowan"
			if strings.Contains(guard.LineText, "northern tunnel") != knows || strings.Contains(guard.LineText, "archive route") != knows {
				t.Errorf("wrong POV disclosure: %s", guard.LineText)
			}
		})
	}
	if before != mustCompactJSON(secret) {
		t.Fatal("guard mutated stored source")
	}
}

func TestMemoryAuditProtectedDisplayNamesReachDelivery(t *testing.T) {
	for _, tc := range []struct {
		name, pov, owner, knower string
		aliases                  map[string]any
		knows                    bool
	}{
		{name: "reported bilingual display", pov: "Sein(세인)", owner: "세인", knows: true},
		{name: "different bilingual name", pov: "Mira (미라)", owner: "Mira", knows: true},
		{name: "fullwidth display", pov: "MIRA（미라）", owner: "미라", knows: true},
		{name: "stored bilingual owner", pov: "유나", owner: "Yuna(유나)", knows: true},
		{name: "known viewer", pov: "Rowan(로완)", owner: "Mira", knower: "Rowan", knows: true},
		{name: "registered alias", pov: "Silver Finch", owner: "Mira", aliases: map[string]any{"Silver Finch": "Mira"}, knows: true},
		{name: "owner uses registered alias", pov: "Mira", owner: "Silver Finch", aliases: map[string]any{"Silver Finch": "Mira"}, knows: true},
		{name: "suspected only", pov: "Yuna(유나)", owner: "Mira"},
		{name: "unknown viewer", pov: "Rowan(로완)", owner: "Mira"},
		{name: "similar name", pov: "Mirabel", owner: "Mira"},
		{name: "different parenthesized names", pov: "Rowan(Mira)", owner: "Mira"},
		{name: "suffix is significant", pov: "Mira(미라) Jr", owner: "Mira"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			claim := "The map is hidden under the north stair."
			summary := mustCompactJSON(map[string]any{
				"turn_summary": "The archive meeting ended.",
				"characters":   []string{tc.owner, "Yuna", "Rowan"},
				"protected_secrets": []any{map[string]any{
					"owner": tc.owner, "subject": []string{"hidden map"}, "summary": claim,
					"secret_kind": "plan", "disclosure_policy": "owner_private_until_revealed",
					"knowledge_scope": map[string]any{"known_by": []string{tc.owner, tc.knower}, "suspected_by": []string{"Yuna"}, "unknown_to": []string{"Visitor"}},
				}},
			})
			perspective := testPrepareTurnAssemblyPerspective(map[string]any{"current_pov": tc.pov})
			perspective.EntityAliases = tc.aliases
			out := buildPrepareTurnInjectionAssemblyWithBudget(prepareTurnAssemblyInput{
				Memories: []store.Memory{{ID: 71, TurnIndex: 12, SummaryJSON: summary, Importance: .8}},
				TopK:     5, MaxChars: 32000, UserInput: "The hidden map at the archive", Profile: "default", BudgetMode: "auto", Perspective: perspective,
			})
			for surface, text := range map[string]string{
				"preprocessing guidance": out.ProtectedMemoryText,
				"final delivery":         stringFromMap(out.MemoryDeliveryPlan, "final_text"),
			} {
				// The author now reads the fact even when the current character
				// does not know it. Character scope and name resolution stay local.
				if !strings.Contains(text, claim) || !strings.Contains(text, "not public character knowledge") {
					t.Errorf("%s lost author content or knowledge distinction: knows=%v, text=%s", surface, tc.knows, text)
				}
				if !strings.Contains(text, "owner="+tc.owner) || !strings.Contains(text, "suspected_by=Yuna") || !strings.Contains(text, "unknown_to=Visitor") {
					t.Errorf("%s lost recorded disclosure boundaries: %s", surface, text)
				}
			}
		})
	}
}

func TestMemoryAuditProtectedDisplayNamesPreserveCanonicalIdentity(t *testing.T) {
	for _, name := range []string{"Mira(미라)", "MIRA（미라）", "유나(Yuna)"} {
		before := normalizeCharacterKey(name)
		if prepareTurnPerspectiveNameKey(name) == before {
			t.Errorf("repeated localized display was not resolved: %s", name)
		}
		if normalizeCharacterKey(name) != before {
			t.Fatal("private delivery changed canonical identity")
		}
	}
	if prepareTurnPerspectiveNameMatches("!!!", "", "???") {
		t.Fatal("empty normalization created an identity match")
	}
	// Explicit alias records resolve both directions without rewriting source rows.
	memories := []store.Memory{{ID: 8, SummaryJSON: mustCompactJSON(map[string]any{
		"characters": []string{"Mira"}, "character_identity_accuracy": []any{map[string]any{
			"canonical_entity_name": "Mira", "surface_identity_name": "Moonbird", "same_entity": true,
		}},
	})}}
	before := memories[0].SummaryJSON
	ctx := prepareTurnProtectedPerspectiveContext(map[string]any{"current_pov": "Moonbird"}, memories, nil)
	guard := prepareTurnProtectedMemoryGuardFromParsed(map[string]any{"protected_secrets": []any{map[string]any{
		"owner": "Mira", "secret_kind": "plan", "summary": "The safe contains a map.", "disclosure_policy": "owner_private_until_revealed",
	}}}, ctx)
	if !strings.Contains(guard.LineText, "The safe contains a map.") || memories[0].SummaryJSON != before {
		t.Fatalf("recorded alias did not retain private content and original source: %s", guard.LineText)
	}
}

func TestMemoryAuditLanguageMetadataDoesNotCreateSecondSummary(t *testing.T) {
	text := "Mira approved the plan. Rowan prepared the journey. Guardians deployment: suspended."
	for _, mode := range []string{"go", "ai"} {
		t.Run(mode, func(t *testing.T) {
			out := buildPrepareTurnInjectionAssemblyWithBudget(prepareTurnAssemblyInput{
				Memories: []store.Memory{{ID: 71, ChatSessionID: "memory-audit", TurnIndex: 20, Importance: .8,
					SummaryJSON: mustCompactJSON(map[string]any{"turn_summary": text, "language_context": map[string]any{"summary_language": "ko", "raw_language": "ko"}})}},
				TopK: 5, MaxChars: 32000, UserInput: "Mira reviews the plan and journey", Profile: "default", BudgetMode: "auto",
				Perspective: testPrepareTurnAssemblyPerspective(priorityMemoryTestContext(5)),
			})
			if mode == "ai" {
				facts, summaries := multiAgentCandidatePool(&out)
				ids, summaryIDs := []string{}, []string{}
				for _, fact := range facts {
					ids = append(ids, fact.CanonicalFactID)
				}
				for _, summary := range summaries {
					summaryIDs = append(summaryIDs, summary.SummaryID)
				}
				out.Preprocessing = &multiAgentSelection{Candidates: facts, Summaries: summaries, Roles: []multiAgentRoleResult{{Role: "event_recent", Source: "ai", Selection: multiAgentRecommendation{SelectedIDs: ids, SelectedSummaryIDs: summaryIDs}}}}
				out.MemoryDeliveryPlan = finalizePrepareTurnPriorityMemoryDeliveryPlan(&out, 32000, 5, "auto", nil, prepareTurnMemorySelectionContext{})
			}
			final := stringFromMap(out.MemoryDeliveryPlan, "final_text")
			if strings.Count(final, text) != 1 {
				t.Errorf("same original repeated: %s", final)
			}
			if !strings.Contains(final, "suspended") {
				t.Fatal("original detail lost")
			}
			if strings.Contains(final, "summary_language=") || strings.Contains(final, "raw_language=") {
				t.Fatal("diagnostic language metadata was rendered as another source")
			}
		})
	}
}

func TestMemoryAuditCriticReferencesKeepFulfillmentTerms(t *testing.T) {
	description := "Receive the silver badge AND obtain a mount registration certificate."
	remaining := "Obtain the mount registration certificate."
	memory := map[string]any{"id": 11, "source": "mariadb_memory", "turn_index": 4, "support_only": true,
		"summary":                  strings.Repeat("Historical scenery. ", 1500),
		"recorded_pending_threads": []any{map[string]any{"lifecycle_key": "badge-and-mount", "title": "Silver badge ceremony", "status": "open", "description": description, "remaining_obligations": remaining}},
	}
	_, ledger, _ := applyCompleteTurnCriticAuxiliaryBudget(nil, []map[string]any{memory}, nil, nil, "The silver badge ceremony finished; the mount certificate is still missing", completeTurnCriticInputPolicy{AuxiliaryMaxChars: 4000})
	encoded := mustCompactJSON(ledger)
	if !strings.Contains(encoded, description) || !strings.Contains(encoded, remaining) {
		t.Fatalf("compact identity lost fulfillment conditions: %s", encoded)
	}
}

func TestMemoryAuditReadingPartsAreSharedWithoutLosingBindings(t *testing.T) {
	state := prepareTurnMemoryFormPart{Key: "@current/entity/mira/equipment", Text: "linked stored state [source turn 3]: Mira equipment: copper badge"}
	facts := []prepareTurnPriorityMemoryCandidate{}
	refs := map[string]string{}
	for _, id := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p"} {
		facts = append(facts, prepareTurnPriorityMemoryCandidate{CanonicalFactID: id, SourceRef: "memories:" + id, SourceTurn: 10,
			Minimum: &prepareTurnMemoryForm{Heading: "Independent event " + id, Text: "Independent event " + id + "\n  Original event " + id + "\n  " + state.Text, Parts: []prepareTurnMemoryFormPart{{Key: "/event", Text: "Original event " + id}, state}}})
		refs[id] = "F" + id
	}
	items := []map[string]any{}
	for _, fact := range facts {
		items = append(items, prepareTurnMemoryModelCandidate(fact, refs))
	}
	input := map[string]any{"role": "event_recent", "candidates": items}
	diagnosticBefore := mustCompactJSON(input)
	wire := multiAgentModelInput(input, 1)
	var packet map[string]any
	if err := json.Unmarshal([]byte(wire), &packet); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(packet)
	expectedCandidates := mustCompactJSON(packet["candidates"])
	wire = multiAgentShareReadingRecords(packet, b)
	if strings.Count(wire, "copper badge") != 1 {
		t.Fatalf("linked state repeated on wire: %s", wire)
	}
	var shared map[string]any
	if err := json.Unmarshal([]byte(wire), &shared); err != nil {
		t.Fatal(err)
	}
	expanded := mapFromAny(expand45Shared(shared, mapFromAny(shared["shared_records"])))
	if mustCompactJSON(expanded["candidates"]) != expectedCandidates {
		t.Fatal("sharing changed source bindings or candidate contents")
	}
	if diagnosticBefore != mustCompactJSON(input) {
		t.Fatal("wire packing mutated diagnostic candidates")
	}
}
