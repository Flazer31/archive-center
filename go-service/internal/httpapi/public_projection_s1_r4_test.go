package httpapi

import (
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func s1R4Fixture() map[string]any {
	quote := "The parcel is only a harmless sample."
	return map[string]any{
		"turn_summary": "A public delivery arrives.",
		"protected_secrets": []any{
			map[string]any{"secret_id": "parcel-cover", "owner": "Coordinator", "disclosure_policy": "owner_private_until_revealed", "evidence_excerpt": quote, "knowledge_scope": map[string]any{"known_by": []string{"Coordinator"}, "misinformed_by": []string{"Courier"}}},
			map[string]any{"secret_id": "independent-source", "owner": "Archivist", "disclosure_policy": "owner_private_until_revealed", "evidence_excerpt": quote, "knowledge_scope": map[string]any{"known_by": []string{"Archivist"}, "unknown_to": []string{"Observer"}}},
		},
		"narrative_events": []any{
			map[string]any{"event": "Coordinator describes the parcel and orders its delivery", "evidence_excerpt": quote, "visibility": "public", "knowledge_scope": map[string]any{"known_by": []string{"Courier", "Observer"}}},
			map[string]any{"event": "Coordinator inspects a harmless parcel publicly", "evidence_excerpt": "The courier checks the label."},
			map[string]any{"event": "An unrelated cart arrives", "evidence_excerpt": "A cart arrives."},
		},
	}
}

// Exercise the existing public projection, source pool, summary model candidate,
// selected summary delivery and preprocessing notes; only selection is recorded.
func s1R4Delivery(t *testing.T, extraction map[string]any, evidence, event string) {
	t.Helper()
	mem := store.Memory{ID: 91, ChatSessionID: "s1-r4-synthetic", TurnIndex: 3, SummaryJSON: mustCompactJSON(extraction), Evidence: evidence}
	before := mustCompactJSON(mem)
	public, ok := publicMemoryFromCanonical(mem)
	if !ok {
		t.Fatal("setup: public memory absent")
	}
	out := prepareTurnInjectionAssembly{}
	appendPrepareTurnPriorityMemoryFactSeeds(&out, prepareTurnMemoryLaneSelection{Recent: []store.Memory{public}}, nil)
	prepareTurnCarryKnowledgeBoundaries(&out, prepareTurnAssemblyInput{Memories: []store.Memory{mem}})
	facts, summaries, _, _ := prepareTurnResolvePrioritySourcePool(&out, event, nil, 4, nil)
	var chosen prepareTurnPriorityTurnSummaryCandidate
	found := false
	for _, summary := range summaries {
		if strings.Contains(summary.CompleteText, event) {
			chosen, found = summary, true
		}
	}
	if !found {
		t.Fatal("setup: generated summary candidate absent")
	}
	projected := buildPublicMemoryProjection(extraction, evidence).Extraction
	var want []map[string]any
	for _, raw := range sliceFromAny(projected["narrative_events"]) {
		item := mapFromAny(raw)
		if stringFromMap(item, "event") == event {
			want = prepareTurnOwnKnowledgeBoundary(map[string]any{"knowledge_boundaries": item["knowledge_boundaries"]})
		}
	}
	if len(want) == 0 || len(chosen.KnowledgeBoundaries) < len(want) {
		t.Fatal("citation boundary absent from generated summary candidate")
	}
	atomicFound := false
	for _, fact := range facts {
		if strings.Contains(fact.CompleteText, event) {
			atomicFound = true
			for _, boundary := range want {
				if !strings.Contains(mustCompactJSON(prepareTurnPriorityCandidateMap(fact, true)), mustCompactJSON(boundary)) {
					t.Error("atomic candidate lost attributed citation boundary")
				}
			}
		}
	}
	if !atomicFound {
		t.Fatal("setup: event atomic candidate absent")
	}
	selection := &multiAgentSelection{Candidates: facts, Summaries: []prepareTurnPriorityTurnSummaryCandidate{chosen}, Roles: []multiAgentRoleResult{{Role: "event_recent", Source: "ai", SelectionRound: 1, Selection: multiAgentRecommendation{SelectedSummaryIDs: []string{chosen.SummaryID}, Reasons: map[string]string{chosen.SummaryID: event}}}}}
	plan := renderPrepareTurnPriorityMemoryDeliveryPlan(&prepareTurnInjectionAssembly{Preprocessing: selection, ProtectedSecretBudgetChars: 1}, 36000, 10, "auto", nil, prepareTurnMemorySelectionContext{Query: event, CurrentTurn: 4}, facts, []prepareTurnPriorityTurnSummaryCandidate{chosen}, nil, nil)
	notes := buildPrepareTurnPreprocessingNotes(selection, plan, nil)
	for name, text := range map[string]string{"summary_candidate": mustCompactJSON(prepareTurnPrioritySummaryMap(chosen)), "main": stringFromMap(plan, "main_memory_text"), "notes": stringFromMap(notes, "final_text")} {
		if !strings.Contains(text, event) {
			t.Errorf("%s lost the independently public event", name)
		}
		for _, boundary := range want {
			if !strings.Contains(text, mustCompactJSON(boundary)) {
				t.Errorf("%s lost attributed citation boundary", name)
			}
		}
	}
	if before != mustCompactJSON(mem) {
		t.Fatal("canonical input changed during delivery")
	}
}

func TestPublicProjectionS1R4StoredReceipts(t *testing.T) {
	ex := s1R4Fixture()
	source := mapFromAny(sliceFromAny(ex["protected_secrets"])[0])
	source["knowledge_source"] = map[string]any{"source": "critic.protected_secrets", "source_index": 4,
		"source_revision": "synthetic-recorded-revision", "source_session": "synthetic-session", "source_turn": 3,
		"root_evidence_id": 41, "direct_evidence_ids": []int{41, 42}, "source_span_start": 7, "source_span_end": 49}
	before := mustCompactJSON(ex)
	projected := buildPublicMemoryProjection(ex, "").Extraction
	event := mapFromAny(sliceFromAny(projected["narrative_events"])[0])
	boundaries := sliceFromAny(event["knowledge_boundaries"])
	if len(boundaries) == 0 {
		t.Fatal("stored source receipt boundary absent")
	}
	boundary := mapFromAny(boundaries[0])
	if mustCompactJSON(boundary["knowledge_source"]) != mustCompactJSON(source["knowledge_source"]) {
		t.Error("existing source quotation receipts changed")
	}
	if stringFromMap(boundary, "protected_fact_ref") != "source-revision:synthetic-recorded-revision/protected_secrets/4" {
		t.Error("recorded source revision attribution lost")
	}
	if before != mustCompactJSON(ex) {
		t.Error("canonical input changed")
	}
}

func s1R4SiblingBytes(t *testing.T, ex map[string]any, event string) []string {
	t.Helper()
	mem := store.Memory{ID: 91, ChatSessionID: "s1-r4-synthetic", TurnIndex: 3, SummaryJSON: mustCompactJSON(ex)}
	public, ok := publicMemoryFromCanonical(mem)
	if !ok {
		t.Fatal("setup: sibling memory absent")
	}
	out := prepareTurnInjectionAssembly{}
	appendPrepareTurnPriorityMemoryFactSeeds(&out, prepareTurnMemoryLaneSelection{Recent: []store.Memory{public}}, nil)
	prepareTurnCarryKnowledgeBoundaries(&out, prepareTurnAssemblyInput{Memories: []store.Memory{mem}})
	facts, _, _, _ := prepareTurnResolvePrioritySourcePool(&out, event, nil, 4, nil)
	for _, chosen := range facts {
		if chosen.CompleteText != event {
			continue
		}
		if len(chosen.KnowledgeBoundaries) != 0 {
			t.Error("public sibling acquired source authority")
		}
		selection := &multiAgentSelection{Candidates: []prepareTurnPriorityMemoryCandidate{chosen}, Roles: []multiAgentRoleResult{{Role: chosen.Lane, Source: "ai", SelectionRound: 1, Selection: multiAgentRecommendation{SelectedIDs: []string{chosen.CanonicalFactID}, Reasons: map[string]string{chosen.CanonicalFactID: event}}}}}
		plan := renderPrepareTurnPriorityMemoryDeliveryPlan(&prepareTurnInjectionAssembly{Preprocessing: selection, ProtectedSecretBudgetChars: 1}, 36000, 10, "auto", nil, prepareTurnMemorySelectionContext{Query: event, CurrentTurn: 4}, nil, nil, nil, nil)
		notes := buildPrepareTurnPreprocessingNotes(selection, plan, nil)
		return []string{mustCompactJSON(prepareTurnPriorityCandidateMap(chosen, true)), stringFromMap(plan, "main_memory_text"), mustCompactJSON(notes)}
	}
	t.Fatal("setup: sibling candidate absent")
	return nil
}

func TestPublicProjectionS1R4SiblingDeliveryParity(t *testing.T) {
	linked := s1R4Fixture()
	unlinked := parseJSONMap(mustCompactJSON(linked))
	delete(mapFromAny(sliceFromAny(unlinked["narrative_events"])[0]), "evidence_excerpt")
	for _, index := range []int{1, 2} {
		event := stringFromMap(mapFromAny(sliceFromAny(linked["narrative_events"])[index]), "event")
		before, after := s1R4SiblingBytes(t, unlinked, event), s1R4SiblingBytes(t, linked, event)
		for i := range before {
			if before[i] != after[i] {
				t.Errorf("unaffected sibling candidate/main/notes bytes changed at %d", i)
			}
		}
	}
}

func TestPublicProjectionS1R4StoredCitation(t *testing.T) {
	ex := s1R4Fixture()
	before := mustCompactJSON(ex)
	projection := buildPublicMemoryProjection(ex, "")
	items := sliceFromAny(projection.Extraction["narrative_events"])
	if len(items) != len(sliceFromAny(ex["narrative_events"])) {
		t.Fatal("public sibling/event lost")
	}
	event := mapFromAny(items[0])
	if _, exists := event["evidence_excerpt"]; exists {
		t.Error("protected citation body entered public projection")
	}
	if mustCompactJSON(event["knowledge_scope"]) != mustCompactJSON(mapFromAny(sliceFromAny(ex["narrative_events"])[0])["knowledge_scope"]) {
		t.Error("citation scope replaced the claim's own scope")
	}
	boundaries := sliceFromAny(event["knowledge_boundaries"])
	if len(boundaries) != len(sliceFromAny(ex["protected_secrets"])) {
		t.Fatalf("stored citation lost separate source boundaries: %s", mustCompactJSON(boundaries))
	}
	for i, raw := range boundaries {
		boundary := mapFromAny(raw)
		source := mapFromAny(sliceFromAny(ex["protected_secrets"])[i])
		if mustCompactJSON(boundary["knowledge_scope"]) != mustCompactJSON(source["knowledge_scope"]) || boundary["secret_id"] != source["secret_id"] {
			t.Error("source scopes were merged or attribution lost")
		}
		origin := mapFromAny(boundary["knowledge_source"])
		if stringFromMap(boundary, "protected_fact_ref") == "" || intFromAny(origin["source_index"], -1) != i {
			t.Error("stored citation origin missing")
		}
		for _, key := range []string{"source_span_start", "source_span_end", "root_evidence_id", "direct_evidence_ids"} {
			if _, exists := origin[key]; exists {
				t.Error("fabricated raw receipt: " + key)
			}
		}
		if strings.Contains(mustCompactJSON(boundary), stringFromMap(source, "evidence_excerpt")) {
			t.Error("protected body cloned into boundary")
		}
	}
	for i := 1; i < len(items); i++ {
		if mustCompactJSON(items[i]) != mustCompactJSON(sliceFromAny(ex["narrative_events"])[i]) {
			t.Error("public sibling/unlinked item bytes changed")
		}
	}
	if before != mustCompactJSON(ex) {
		t.Fatal("canonical extraction mutated")
	}
	s1R4Delivery(t, ex, "", stringFromMap(event, "event"))
}

func TestPublicProjectionS1R4UnlinkedParity(t *testing.T) {
	ex := s1R4Fixture()
	// Similar prose and a shared character do not supply a stored citation link.
	mapFromAny(sliceFromAny(ex["narrative_events"])[0])["evidence_excerpt"] = "The parcel is only a harmless sample, according to Coordinator."
	withSources := buildPublicMemoryProjection(ex, "")
	withoutSources := parseJSONMap(mustCompactJSON(ex))
	withoutSources["protected_secrets"] = []any{}
	// Both projections rebuild from the same events, independently of raw summary.
	delete(withoutSources, "turn_summary")
	control := buildPublicMemoryProjection(withoutSources, "")
	for i, raw := range sliceFromAny(withSources.Extraction["narrative_events"]) {
		if len(sliceFromAny(mapFromAny(raw)["knowledge_boundaries"])) != 0 {
			t.Error("unlinked prose acquired citation authority")
		}
		// The existing policy strips containing private citations, so compare the
		// surviving public expression and every independent sibling byte-for-byte.
		if i > 0 && mustCompactJSON(raw) != mustCompactJSON(sliceFromAny(control.Extraction["narrative_events"])[i]) {
			t.Error("unaffected sibling bytes changed")
		}
	}
	if withSources.SearchText.Text != control.SearchText.Text {
		t.Error("unaffected public search text changed")
	}
}
