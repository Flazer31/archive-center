package httpapi

import (
	"fmt"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func Test30ProtectedFactGroupingPreservesDistinctClaimsAndBoundaries(t *testing.T) {
	secret := func(id int64, turn int, owner, kind, subject, claim string, unknown []string) store.Memory {
		return store.Memory{ID: id, ChatSessionID: "secret-facts", TurnIndex: turn, SummaryJSON: mustCompactJSON(map[string]any{
			"turn_summary": claim, "protected_secrets": []any{map[string]any{"owner": owner, "secret_kind": kind, "subject": []string{subject}, "summary": claim,
				"disclosure_policy": "owner_private_until_revealed", "knowledge_scope": map[string]any{"known_by": []string{owner}, "unknown_to": unknown}}},
		})}
	}
	items := []store.Memory{
		secret(1, 1, "Mask", "identity", "Mask", "old cover wording", []string{"Guard"}),
		secret(2, 3, "Mira", "identity", "Mira", "latest complete wording", []string{"Guard", "Visitor"}),
		secret(3, 3, "Mask", "identity", "Mask", "same-turn sparse wording", nil),
		secret(4, 2, "Mira", "identity", "Nia", "different person's disguise", []string{"Guard"}),
		secret(5, 2, "Mira", "awareness", "Mira", "different fact kind", []string{"Guard"}),
		secret(6, 2, "Nia", "identity", "Mira", "different owner", []string{"Guard"}),
	}
	identity := store.Memory{SummaryJSON: `{"character_identity_accuracy":[{"surface_identity_name":"Mask","canonical_entity_name":"Mira","same_entity":true}]}`}
	before := mustCompactJSON(items)
	selection := prepareTurnMemoryLaneSelection{ProtectedSelected: items, ProtectedCandidates: items}
	groups, _ := buildPrepareTurnProtectedDeliveryGroups(selection, identity)
	count := 0
	for _, g := range groups {
		count += len(g)
	}
	// Test.30 inferred one fact from owner/kind/subject alone. These six
	// fixtures actually have distinct statements or knowledge boundaries;
	// resolving Mask to Mira does not make their claims duplicates.
	if count != len(items) {
		t.Fatalf("distinct claim or knowledge boundary was merged: %d", count)
	}
	retained := groups[prepareTurnMemoryLaneKey(items[1])]
	if len(retained) != 1 || len(retained[0].SourceRowIDs) != 1 || !strings.Contains(retained[0].Memory.SummaryJSON, "Visitor") {
		t.Fatalf("recorded card boundary or source lineage lost: %#v", retained)
	}
	lines, trace := prepareTurnMemoryLaneLines(selection, nil, []store.Memory{identity})
	if len(lines) != count || !strings.Contains(strings.Join(lines, "\n"), "old cover wording") {
		t.Fatal("grouping did not reach the production renderer")
	}
	if mustCompactJSON(items) != before {
		t.Fatal("stored rows mutated")
	}
	// Make every independent subject scene-relevant and give this identity
	// regression enough protected space. Relevance and budget limits have their
	// own controls; neither should be mistaken for fact consolidation here.
	assembly := buildPrepareTurnInjectionAssemblyWithBudget(prepareTurnAssemblyInput{Memories: items, TopK: 20, MaxChars: 12000, UserInput: "Mira Mask Nia", BudgetMode: "auto",
		ProtectedSecretBudgetChars: 5000,
		Perspective:                &prepareTurnAssemblyPerspective{EntityAliases: map[string]any{"Mask": "Mira"}, Selection: prepareTurnMemorySelectionContext{PriorityEnabled: true, Query: "Mira Mask Nia"}},
	})
	if !strings.Contains(assembly.ProtectedMemoryText, "old cover wording") || !strings.Contains(assembly.ProtectedMemoryText, "latest complete wording") {
		t.Fatalf("scene-relevant distinct protected claims were collapsed: %s", assembly.ProtectedMemoryText)
	}
	out := &prepareTurnInjectionAssembly{ProtectedMemoryText: strings.Join(stringsFromAny(trace["protected_lines"]), "\n"), ProtectedSecretBudgetChars: 5000}
	plan := buildPrepareTurnPriorityMemoryDeliveryPlan(out, 6000, 1, "auto", nil, prepareTurnMemorySelectionContext{Query: "Mira Mask Nia"})
	lineage := finalizePrepareTurnMemoryDeliveryLineage(buildPrepareTurnMemoryDeliveryLineage(selection, trace), plan)
	delivered := 0
	for _, row := range prepareTurnMemoryLineageSlice(lineage["items"]) {
		if boolFromAny(mapFromAny(row)["delivered"]) {
			delivered++
		}
	}
	if delivered != count {
		t.Fatalf("shared guard lost delivered lineage: %d != %d", delivered, count)
	}
}

func Test33ProtectedSubjectBeforeSceneOwner(t *testing.T) {
	other := "- [turn 1] " + prepareTurnProtectedCardGuard + "silver archive clue; subject=old route; owner=Mira"
	scene := "- [turn 2] " + prepareTurnProtectedCardGuard + "hidden garden; subject=silver archive clue; owner=Unknown"
	out := &prepareTurnInjectionAssembly{ProtectedMemoryText: other + "\n" + scene, PriorityEntityAliases: map[string]any{"Mask": "Mira"}, Counts: map[string]any{"stored_active_scene_entities": []string{"Mira"}}}
	plan := buildPrepareTurnPriorityMemoryDeliveryPlan(out, 1500, 1, "auto", nil, prepareTurnMemorySelectionContext{Query: "silver archive clue"})
	final := stringFromMap(plan, "final_text")
	if strings.Index(final, "hidden garden") < 0 || strings.Index(final, "hidden garden") > strings.Index(final, "silver archive clue") {
		t.Fatal("matching fact subject did not outrank the scene-owner-only card")
	}
}

func Test30SharedProtectedGuard(t *testing.T) {
	lines := []string{}
	for i := 0; i < 3; i++ {
		lines = append(lines, fmt.Sprintf("- [turn %d] %ssecret %d; owner=Mira", i+1, prepareTurnProtectedCardGuard, i))
	}
	plain := []string{strings.TrimSpace(prepareTurnProtectedCardGuard)}
	for _, line := range lines {
		plain = append(plain, strings.ReplaceAll(line, prepareTurnProtectedCardGuard, ""))
	}
	want := makePrepareTurnSection("["+prepareTurnMemoryDeliveryTitles["protected_secret"]+"]", plain)
	cap := len([]rune(want))
	out := &prepareTurnInjectionAssembly{ProtectedMemoryText: strings.Join(lines, "\n"), Counts: map[string]any{"stored_active_scene_entities": []string{"Mira"}}}
	plan := buildPrepareTurnPriorityMemoryDeliveryPlan(out, cap, 1, "custom", map[string]int{"protected_secret": cap}, prepareTurnMemorySelectionContext{})
	if stringFromMap(plan, "final_text") != want || intFromAny(plan["used_chars"], 0) != cap {
		t.Fatalf("shared guard/card cost mismatch: %s", stringFromMap(plan, "final_text"))
	}
	general := prepareTurnProtectedCardFromParsed(map[string]any{"protected_secrets": []any{map[string]any{"summary": "private route", "disclosure_policy": "owner_private_until_revealed"}}}, "")
	if !strings.HasPrefix(general, prepareTurnProtectedCardGuard) {
		t.Fatal("general-card path lost guard")
	}
}

func Test33DirectEvidenceReservationBeforeOrdinary(t *testing.T) {
	first := "- [turn 8] " + strings.Repeat("first recall quote ", 12)
	second := "- [turn 2] " + strings.Repeat("second recall quote ", 12)
	out := &prepareTurnInjectionAssembly{DirectEvidenceText: first + "\n" + second, CanonWorldText: "- " + strings.Repeat("ordinary world detail ", 5)}
	cap := len([]rune(makePrepareTurnSection("["+prepareTurnMemoryDeliveryTitles["direct_evidence"]+"]", []string{first, second})))
	plan := buildPrepareTurnPriorityMemoryDeliveryPlan(out, cap, 1, "auto", nil, prepareTurnMemorySelectionContext{Query: "ordinary world detail"})
	final := stringFromMap(plan, "final_text")
	if !strings.Contains(final, "ordinary world detail") || !strings.Contains(final, strings.TrimSpace(first)) || strings.Contains(final, strings.TrimSpace(second)) {
		t.Fatal("direct evidence consumed ordinary reservation or recall order changed", final)
	}
	if len([]rune(final)) > cap {
		t.Fatal("global budget exceeded")
	}
	custom := buildPrepareTurnPriorityMemoryDeliveryPlan(out, cap, 1, "custom", map[string]int{"direct_evidence": 100, "world_state": cap - 100}, prepareTurnMemorySelectionContext{})
	if strings.Contains(stringFromMap(custom, "final_text"), "recall quote") {
		t.Fatal("custom direct cap changed")
	}
}
