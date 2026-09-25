package httpapi

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

// This fixed production fixture exposed test.17 expanding preprocessing from
// 37 to 53 candidates when final-only formatting shortened the shared reading.
// Preserve the test.16 provider packet AND test.17's cleaner final Go delivery.
// These snapshots concern transport, not an assertion about fresh AI quality.
func Test47FinalPresentationDoesNotChangePreprocessingPacket(t *testing.T) {
	query := "Mira and Rowan discuss returning the brass compasses after the voyage."
	input := prepareTurnAssemblyInput{TopK: 100, MaxChars: 100000, BudgetMode: "auto", UserInput: query, Profile: "default", Perspective: testPrepareTurnAssemblyPerspective(priorityMemoryTestContext(100))}
	input.Perspective.Selection.Query = query
	input.Perspective.Selection.CurrentTurn = 31
	for i := 0; i < 80; i++ {
		promise, current := lifecycle46Fixture("local-paired-diagnostic")
		promise.ID, current.ID = int64(1000+i), int64(2000+i)
		key := fmt.Sprintf("compass-voyage-%d", i)
		promise.SummaryJSON = strings.ReplaceAll(promise.SummaryJSON, "compass-voyage", key)
		current.OwnerID = key
		current.ValueJSON = strings.ReplaceAll(current.ValueJSON, "compass-voyage", key)
		input.Memories = append(input.Memories, promise)
		input.Perspective.NarrativeValues = append(input.Perspective.NarrativeValues, current)
	}
	out := buildPrepareTurnInjectionAssemblyWithBudget(input)
	facts, summaries := multiAgentCandidatePool(&out)
	cfg := defaultMultiAgentSettings()
	cfg.CandidateChars = 32000
	packet := multiAgentInput("event_recent", facts, summaries, dto.PrepareTurnRequest{RawUserInput: &query}, cfg, 36000, 100, map[string]int{"event_recent": 36000})
	for _, check := range []struct{ name, text, sha string }{
		{"test16 preprocessing", multiAgentModelInput(packet, 1), "1df8abda0cb40b513839854a9fbeb12c15e06cdb6391827af12c36b3e9004593"},
		{"test17 final delivery", stringFromMap(out.MemoryDeliveryPlan, "final_text"), "a0016cc1e1cf0ff7c96729aa63c607cfb25414323c97953b787cf942d605b7e6"},
	} {
		if got := fmt.Sprintf("%x", sha256.Sum256([]byte(mustCompactJSON(check.text)))); got != check.sha {
			t.Errorf("%s changed in the fixed source/budget fixture: %s", check.name, got)
		}
	}
}

func Test47PresentationKeepsConditionalMemoryAndDiagnosticIdentity(t *testing.T) {
	quote := `Mira read {"source_revision":"story-code","observed_at":"not a date"}; she refused.`
	details := map[string]any{"description": "Mira may stay only if she repairs the bridge.", "status": "open", "condition": "Only after the storm ends.", "exceptions": []any{"Not during flooding."}, "future_custom_field": map[string]any{"holder": "Mira", "permission": false, "unknown": nil}, "evidence_excerpt": quote}
	reading := &prepareTurnMemoryContext{Path: "/promise", Parts: []prepareTurnMemoryPart{
		{Key: "/promise", Value: "Mira has not accepted the offer.", FactTexts: []string{"Mira has not accepted the offer."}},
		{Key: "@lifecycle/bridge/entity/mira/details", Label: "progression details", Value: mustCompactJSON(details)},
		{Key: "@lifecycle/bridge/entity/mira/source", Label: "progression source", Value: "critic.pending_threads; source revision internal-rev; direct evidence [8841]"},
		{Key: "@lifecycle/bridge/entity/mira/repair_source_revision", Label: "restoration audit repair_source_revision", Value: "repair-rev"},
		{Key: "@current/entity/mira/condition/source_revision", Label: "state source_revision", Value: "internal-rev"},
		{Key: "@current/entity/mira/condition/observed_at", Label: "state observed_at", Value: `{"kind":"source_observation","story_clock":{"version":"story_clock.v1","absolute":{"date":"2002-06-04","time":"16:18"},"precision":"exact"}}`},
		{Key: "@field/status/permission/evidence_excerpt", Label: "evidence_excerpt", Value: quote},
		{Key: "/custom/source_revision", Label: "source_revision", Value: "in-story registration number"},
	}}
	before := mustCompactJSON(reading.Parts)
	facts := []prepareTurnPriorityMemoryCandidate{{CanonicalFactID: "promise-fact", SourceRef: "memories:8841", SourceTable: "memories", SourceTurn: 4, Lane: "unresolved_goal", PerspectiveOwner: "Mira", Visibility: "owner_private", AllowedViewers: []string{"Mira", "Rowan"}, CompleteText: "Mira has not accepted the offer.", Reading: reading}}
	prepareTurnBuildReadingForms(facts, func(string) float64 { return 1 }, nil)
	if before != mustCompactJSON(reading.Parts) || facts[0].SourceRef != "memories:8841" || !strings.Contains(facts[0].Minimum.Meaning, "internal-rev") {
		t.Fatal("display erased internal source material")
	}
	input := prepareTurnMemoryModelCandidate(facts[0], map[string]string{"promise-fact": "F1"})
	if input["source_ref"] != "memories:8841" || input["ref"] != "F1" {
		t.Fatal("preprocessing lost reference resolution")
	}
	for _, want := range []string{"internal-rev", "repair-rev", `progression details: {`, `"permission":false`} {
		if !strings.Contains(stringFromMap(input, "text"), want) {
			t.Fatalf("final presentation altered preprocessing reading: missing %q", want)
		}
	}
	for _, ref := range []string{"", "F1"} {
		layout := &prepareTurnMemoryReadingLayout{}
		edit := layout.preview(prepareTurnMemoryCandidateRow(facts[0], 1, ref))
		layout.apply(edit)
		final := strings.Join(layout.texts(), "\n")
		if edit.delta != 1+utf8.RuneCountInString(final) {
			t.Fatal("final layout charged the preprocessing reading size")
		}
		for _, want := range []string{"has not accepted", details["description"].(string), details["condition"].(string), "Not during flooding.", "permission: false", "unknown: null", quote, "in-story registration number", "2002-06-04 16:18", "source turn 4", "owner Mira", "owner_private", "viewers Mira, Rowan", "status: open"} {
			if !strings.Contains(final, want) {
				t.Errorf("lost content %q: %s", want, final)
			}
		}
		for _, unwanted := range []string{"internal-rev", "repair-rev", "memories:8841", "direct evidence [8841]", "story_clock.v1", "progression details: {"} {
			if strings.Contains(final, unwanted) {
				t.Errorf("internal representation reached final text: %s", unwanted)
			}
		}
		if ref != "" && !strings.Contains(final, "[F1]") {
			t.Fatal("selected reference disappeared")
		}
	}
	if facts[0].Minimum.Chars != utf8.RuneCountInString(facts[0].Minimum.Text) {
		t.Fatal("incorrect displayed budget")
	}
}

func Test47PreprocessingPresentationRetainsFullTraceAndKnowledge(t *testing.T) {
	item := map[string]any{"canonical_fact_id": "fact-a", "selection_status": "selected", "source_table": "precise_memory_facts", "source_ref": "precise_memory_facts:987654", "source_turn": 17, "visibility": "owner_private", "perspective_owner": "Mira", "allowed_viewers": []string{"Mira"}}
	selection := &multiAgentSelection{Roles: []multiAgentRoleResult{{Role: "subjective_relationship", Source: "ai", SelectionRound: 1, Selection: multiAgentRecommendation{SelectedIDs: []string{"fact-a"}, Reasons: map[string]string{"fact-a": "Mira suspects a betrayal; it is not an established fact."}, Unresolved: []string{"Rowan's knowledge remains unknown."}}, Calls: []multiAgentCall{{Round: 1, Input: map[string]any{"candidates": []any{item}}}}}}}
	before := mustCompactJSON(selection)
	plan := map[string]any{"priority_items": []any{item}}
	notes := buildPrepareTurnPreprocessingNotes(selection, plan, nil)
	text := stringFromMap(notes, "final_text")
	for _, want := range []string{"Mira suspects a betrayal; it is not an established fact.", "Rowan's knowledge remains unknown.", "turn: 17", "owner: Mira", "viewers: [Mira]", "visibility: owner_private"} {
		if !strings.Contains(text, want) {
			t.Fatalf("lost %q: %s", want, text)
		}
	}
	if strings.Contains(text, "precise_memory_facts") || strings.Contains(text, "Source scope catalog: {") {
		t.Fatal("database source catalog reached model text")
	}
	if before != mustCompactJSON(selection) {
		t.Fatal("presentation mutated selected memories or AI output")
	}
	if !strings.Contains(mustCompactJSON(notes["source_catalog"]), "precise_memory_facts:987654") {
		t.Fatal("diagnostic source mapping was removed")
	}
	wantScope := map[string]any{"source_table": "precise_memory_facts", "source_turn": 17, "source_refs": []string{"precise_memory_facts:987654"}, "visibility": "owner_private", "perspective_owner": "Mira", "allowed_viewers": []string{"Mira"}}
	if mustCompactJSON(mapFromAny(notes["source_catalog"])["P1"]) != mustCompactJSON(wantScope) {
		t.Fatal("diagnostic source, turn or knowledge scope changed")
	}
	if intFromAny(notes["used_chars"], 0) != utf8.RuneCountInString(text) {
		t.Fatal("note budget does not measure displayed text")
	}
}

func Test47SessionIDStaysInProvenanceOutsideModelReading(t *testing.T) {
	for _, sid := range []string{"origin", "char_27_cid_78180485-a6ea-4a6e-a89e-0c1a0da3fa85"} {
		t.Run(sid, func(t *testing.T) {
			state, current := characterFields46Fixture()
			state.FieldProvenanceJSON = strings.ReplaceAll(state.FieldProvenanceJSON, `"source_session_id":"origin"`, `"source_session_id":"`+sid+`"`)
			before := state.FieldProvenanceJSON
			out := characterFields46Assembly(state, []store.StatusCurrentValue{current})
			facts, _ := multiAgentCandidatePool(&out)
			found := false
			for _, fact := range facts {
				if fact.SourceFieldPath != "/state/mobility" {
					continue
				}
				found = true
				if fact.SourceRef != "character_states:466" || fact.SourceTurn != 10 {
					t.Fatal("presentation changed the observation's source identity")
				}
				internal := false
				for _, part := range fact.Reading.Parts {
					if part.Label == "source_session_id" && part.Value == sid {
						internal = true
					}
				}
				if !internal || !strings.Contains(fact.Minimum.Meaning, sid) {
					t.Fatal("internal provenance or relevance material was erased")
				}
				if strings.Contains(prepareTurnMemoryReadingText(fact), "source_session_id:") {
					t.Error("session metadata reached the model reading")
				}
				if fact.Minimum.Chars != utf8.RuneCountInString(fact.Minimum.Text) {
					t.Error("reading budget does not measure rendered text")
				}
			}
			if !found {
				t.Fatal("expected character memory did not reach the candidate pool")
			}
			final := stringFromMap(out.MemoryDeliveryPlan, "final_text")
			if strings.Contains(final, "source_session_id:") || strings.Contains(final, sid) {
				t.Error("session ID reached final memory delivery")
			}
			for _, want := range []string{"source turn 10", "needs a sling", "full use of the left arm", "Mira lifted the crate freely with both hands", "owner Mira"} {
				if !strings.Contains(final, want) {
					t.Errorf("presentation lost story evidence or perspective: %s", want)
				}
			}
			if state.FieldProvenanceJSON != before {
				t.Fatal("rendering modified stored provenance")
			}
			assert45Budget(t, out.MemoryDeliveryPlan, 18000)
		})
	}
}

func Test47SessionMetadataOmissionDoesNotRewriteQuotedStoryText(t *testing.T) {
	state, current := characterFields46Fixture()
	quote := "Mira read source_session_id: story-ticket on the terminal."
	state.FieldProvenanceJSON = strings.ReplaceAll(state.FieldProvenanceJSON, "Mira supported the injured arm with a sling.", quote)
	out := characterFields46Assembly(state, []store.StatusCurrentValue{current})
	final := stringFromMap(out.MemoryDeliveryPlan, "final_text")
	if !strings.Contains(final, quote) {
		t.Fatal("metadata presentation rewrote an exact story quotation")
	}
	if strings.Contains(final, "source_session_id: origin") {
		t.Fatal("backend metadata still appears alongside the quotation")
	}
}

func Test47BodySessionIdentityDoesNotEnterFinalMemoryText(t *testing.T) {
	body, clock, character := bodyDeliveryFixture46()
	out := buildPrepareTurnInjectionAssemblyWithBudget(bodyAssemblyInput46(body, clock, character))
	final := stringFromMap(out.MemoryDeliveryPlan, "final_text")
	if strings.Contains(final, body.SessionID) {
		for _, line := range strings.Split(final, "\n") {
			if strings.Contains(line, body.SessionID) {
				t.Errorf("body model source session reached final delivery: %s", line)
			}
		}
	}
	assertBodyReading46(t, final)
}
