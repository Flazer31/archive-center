package httpapi

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func criticStateSave(t *testing.T, st store.Store, turn int, extraction map[string]any, content string) artifactSaveResult {
	t.Helper()
	ctx := context.WithValue(context.Background(), entityIdentitySourceContextKey{}, entityIdentitySourceContext{
		Revision: fmt.Sprintf("critic-state-%d", turn), ContractVersion: completeTurnSourceAcceptanceContract,
	})
	result := artifactSaveResult{}
	(&Server{Store: st}).saveNarrativeStateFromExtraction(ctx, "critic-state", turn, extraction, content, nil, time.Unix(int64(turn), 0), &result)
	if result.Errors != 0 {
		t.Fatalf("production save failed: %+v", result)
	}
	return result
}

func criticStateClaim(quote string) map[string]any {
	return map[string]any{"state_claims": []any{map[string]any{
		"subject": "Mira", "subject_type": "character", "state_slot": "location", "value": "northern quay", "transition": "change", "evidence_excerpt": quote,
	}}}
}

func TestCriticStateNarrowEvidenceAndSharedSanitizer(t *testing.T) {
	for _, tc := range []struct {
		name, quote, source, want, shared string
	}{
		{"exact", "Mira reached the quay.", "Mira reached the quay.", "Mira reached the quay.", "Mira reached the quay."},
		{"whitespace", "Mira reached the quay.", "Mira\n\t reached  the quay.", "Mira\n\t reached  the quay.", "Mira reached the quay."},
		{"outer_quotes", "『“Mira reached the quay.”』", "Mira reached the quay.", "Mira reached the quay.", ""},
		{"brackets", "【(Mira reached the quay.)】", "Mira reached the quay.", "Mira reached the quay.", ""},
		{"inner_quotes", "Mira said 'arrived'.", "Mira said ‘arrived’.", "Mira said ‘arrived’.", ""},
		{"single_double_quotes", "Mira said 'arrived'.", "Mira said \"arrived\".", "Mira said \"arrived\".", ""},
		{"japanese_quotes", "Mira said 「arrived」.", "Mira said \"arrived\".", "Mira said \"arrived\".", ""},
		{"punctuation", "Mira reached the quay", "Mira, reached: the quay!", "", ""},
		{"split_ellipsis", "Mira ... reached the quay.", "Mira left home and reached the quay.", "", ""},
		{"split_no_marker", "Mira left home. Mira reached the quay.", "Mira left home. Hours passed. Mira reached the quay.", "", ""},
		{"paraphrase", "Mira arrived at the quay.", "Mira reached the quay.", "", ""},
		{"pronoun", "She reached the quay.", "Mira reached the quay.", "", ""},
		{"parenthetical", "Mira reached the quay.", "Mira (the captain) reached the quay.", "", ""},
		{"empty", "", "Mira reached the quay.", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			content := "At dawn. " + tc.source + " The bell rang."
			st := &turnRecordingStore{}
			criticStateSave(t, st, 1, criticStateClaim(tc.quote), content)
			if tc.want == "" {
				if len(st.savedStatusEvents) != 0 {
					t.Fatal("unsupported quote was accepted")
				}
			} else {
				if len(st.savedStatusEvents) != 1 {
					t.Fatal("supported quote was lost")
				}
				got := stringFromMap(parseJSONMap(st.savedStatusEvents[0].EvidenceJSON), "evidence_excerpt")
				if got != tc.want || !strings.Contains(content, got) {
					t.Fatalf("stored quote is not the actual source span: %q", got)
				}
			}
			if got := sanitizeEvidenceExcerptForTurn(tc.quote, content); got != tc.shared {
				t.Fatalf("shared callers changed: %q want %q", got, tc.shared)
			}
			events := &turnRecordingStore{}
			criticStateSave(t, events, 1, map[string]any{"narrative_events": []any{map[string]any{"summary": "The arrival", "evidence_excerpt": tc.quote}}}, content)
			wantCount := 0
			if tc.shared != "" {
				wantCount = 1
			}
			if len(events.savedStatusEvents) != wantCount {
				t.Fatal("shared narrative-event caller was relaxed")
			}
		})
	}
}

func TestCriticStatePreviousInputSourceKeepsCurrentObservation(t *testing.T) {
	const turn = 5
	quote := "Mira reached the quay."
	for _, tc := range []struct {
		name, source       string
		inputTurn          int
		included, accepted bool
	}{
		{"previous_canonical", "previous_canonical_turn", turn - 1, true, true},
		{"whole_previous_message", "previous_canonical_turn", turn - 1, true, true},
		{"older", "previous_canonical_turn", turn - 2, true, false},
		{"summary", "relevant_turn_memory", turn - 1, true, false},
		{"host_context", "host_context", turn - 1, true, false},
		{"not_supplied", "previous_canonical_turn", turn - 1, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			messages := []map[string]any{}
			if tc.included {
				content := "At dawn. " + quote + " The bell rang."
				if tc.name == "whole_previous_message" {
					content = quote
				}
				messages = append(messages, map[string]any{"source": tc.source, "turn_index": tc.inputTurn, "content": content})
			}
			st := &completeTurnReprocessingStore{turnRecordingStore: &turnRecordingStore{}, sources: map[string]*store.MemorySourceRevision{
				fmt.Sprintf("critic-state-%d", turn): {ChatSessionID: "critic-state", CriticInputSnapshotJSON: mustCompactJSON(completeTurnCriticInputSnapshot{ContextMessages: messages})},
			}}
			// The DB's current raw row is deliberately available but was not
			// necessarily part of the frozen Critic input. It is not a substitute.
			st.returnChatLogs = []store.ChatLog{{TurnIndex: turn - 1, Role: "assistant", Content: quote}}
			criticStateSave(t, st, turn, criticStateClaim("「"+quote+"」"), "The crew rested on the quay.")
			if !tc.accepted {
				if len(st.savedStatusEvents) != 0 {
					t.Fatal("unseen or older source was admitted")
				}
				return
			}
			if len(st.savedStatusEvents) != 1 {
				t.Fatal("previous input source lost")
			}
			event := st.savedStatusEvents[0]
			evidence, value := parseJSONMap(event.EvidenceJSON), parseJSONMap(event.NewValueJSON)
			if event.SourceTurn != turn || intFromAny(value["source_turn"], 0) != turn || intFromAny(evidence["source_turn"], 0) != turn || intFromAny(evidence["evidence_source_turn"], 0) != turn-1 {
				t.Fatalf("source time was confused with state time: %+v", event)
			}
			if evidence["evidence_excerpt"] != quote || value["occurrence_time"] != nil {
				t.Fatal("fabricated source text or event date")
			}
		})
	}
}

func criticDescriptionSeed(t *testing.T, st *turnRecordingStore, phase, description string) {
	t.Helper()
	criticStateSave(t, st, 1, map[string]any{
		"state_claims":    []any{map[string]any{"subject": "Canal survey", "lifecycle_key": "canal-survey", "state_slot": "goal_status", "value": "western bank surveyed", "transition": phase, "confidence": .9, "evidence_excerpt": "The western bank was surveyed."}},
		"pending_threads": []any{map[string]any{"title": "Canal survey", "lifecycle_key": "canal-survey", "description": description, "remaining_obligations": "inspect eastern bank"}},
	}, "The crew reported. The western bank was surveyed. They rested.")
}

func TestCriticStateDescriptionFiveCases(t *testing.T) {
	for _, tc := range []struct {
		name, before, after string
		reaffirm            bool
	}{
		{"plan_detail", "Choose the route", "Choose the northern crossing before surveying the eastern bank", false},
		{"partial_rescue", "Recover all instruments", "Two instruments recovered; the eastern instrument remains missing", false},
		{"schedule_uncertainty", "The inspection is expected this weekend", "A new assignment may alter the inspection schedule", false},
		{"same_event_today", "The ceremony is planned for tomorrow at noon", "The ceremony is planned for today at noon", true},
		{"same_event_repeated", "The ceremony is planned for tomorrow at noon", "The same ceremony is planned for this noon", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			st := &turnRecordingStore{}
			criticDescriptionSeed(t, st, "progress", tc.before)
			before := st.returnStatusCurrent[0]
			x := map[string]any{"pending_threads": []any{map[string]any{"lifecycle_key": "canal-survey", "title": "Canal survey", "description": tc.after}}}
			if tc.reaffirm {
				x["state_claims"] = []any{map[string]any{"subject": "Canal survey", "state_slot": "goal_status", "lifecycle_key": "canal-survey", "transition": "reaffirm", "value": tc.after, "evidence_excerpt": "The crew confirmed the plan."}}
			}
			criticStateSave(t, st, 2, x, "At dawn. The crew confirmed the plan. They left.")
			after := st.returnStatusCurrent[0]
			if tc.reaffirm {
				if after.ValueJSON != before.ValueJSON || len(st.savedStatusEvents) != 1 {
					t.Fatal("explicit reconfirmation was overwritten by its pending description")
				}
				return
			}
			payload := parseJSONMap(after.ValueJSON)
			thread := narrativePendingSnapshot(payload)
			if thread.Description != tc.after || len(st.savedStatusEvents) != 2 {
				t.Fatal("description update missing")
			}
			prior := parseJSONMap(before.ValueJSON)
			for _, field := range []string{"value", "transition", "observed_at", "source_turn"} {
				if mustCompactJSON(payload[field]) != mustCompactJSON(prior[field]) {
					t.Fatalf("description changed effective %s", field)
				}
			}
			if stringFromMap(parseJSONMap(thread.HookMetadataJSON), "remaining_obligations") != "inspect eastern bank" {
				t.Fatal("omitted obligations erased")
			}
			criticStateSave(t, st, 3, x, "The same plan is mentioned again.")
			if len(st.savedStatusEvents) != 2 {
				t.Fatal("identical description created another event")
			}
		})
	}
}

func TestCriticStateDescriptionNineProtectedEffects(t *testing.T) {
	for _, phase := range []string{"set", "progress", "partial", "complete", "resolve", "abandon", "defer", "pause", "supersede"} {
		t.Run(phase, func(t *testing.T) {
			st := &turnRecordingStore{}
			criticDescriptionSeed(t, st, phase, "Survey both banks")
			before := st.returnStatusCurrent[0]
			criticStateSave(t, st, 2, map[string]any{"pending_threads": []any{map[string]any{"title": "Canal survey", "lifecycle_key": "canal-survey", "description": "Survey the eastern bank after dusk"}}}, "The original survey is recalled.")
			after := st.returnStatusCurrent[0]
			payload, prior := parseJSONMap(after.ValueJSON), parseJSONMap(before.ValueJSON)
			for _, field := range []string{"value", "transition", "observed_at", "source_turn"} {
				if mustCompactJSON(payload[field]) != mustCompactJSON(prior[field]) {
					t.Fatalf("changed protected %s", field)
				}
			}
			if narrativeLifecycleProjectionStatus(phase) != "open" && (before.ValueJSON != after.ValueJSON || len(st.savedStatusEvents) != 1) {
				t.Fatal("closed goal was rewritten")
			}
		})
	}
}

func TestCriticStateDescriptionWhitespaceAndExplicitValueStayRejected(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		st := &turnRecordingStore{}
		criticDescriptionSeed(t, st, "progress", "Survey the eastern bank")
		pending := map[string]any{"title": "Canal survey", "lifecycle_key": "canal-survey", "description": "Survey  the\n eastern bank."}
		if explicit {
			pending["value"], pending["description"] = "all banks surveyed", "The work is complete"
		}
		criticStateSave(t, st, 2, map[string]any{"pending_threads": []any{pending}}, "The survey was discussed.")
		if len(st.savedStatusEvents) != 1 {
			t.Fatal("reaffirmation or non-final value replacement was accepted")
		}
	}
}

func TestCriticStateReaffirmWithSeparateDescription(t *testing.T) {
	st := &turnRecordingStore{}
	criticStateSave(t, st, 1, map[string]any{"pending_threads": []any{map[string]any{"title": "Canal survey", "lifecycle_key": "canal-survey", "description": "The survey is planned this weekend"}}}, "The survey was scheduled.")
	prior := st.returnStatusCurrent[0]
	criticStateSave(t, st, 2, map[string]any{
		"state_claims":    []any{map[string]any{"subject": "Canal survey", "state_slot": "goal_status", "lifecycle_key": "canal-survey", "transition": "reaffirm", "value": "The survey is planned", "evidence_excerpt": "The survey is still planned."}},
		"pending_threads": []any{map[string]any{"title": "Canal survey", "lifecycle_key": "canal-survey", "description": "The survey is planned. A new assignment may alter its schedule."}},
	}, "At dawn. The survey is still planned. The crew has another assignment.")
	if len(st.savedStatusEvents) != 2 {
		t.Fatal("separate descriptive sentence was lost with reaffirmed value")
	}
	next := parseJSONMap(st.returnStatusCurrent[0].ValueJSON)
	if next["value"] != parseJSONMap(prior.ValueJSON)["value"] {
		t.Fatal("reaffirmation replaced the value")
	}
}

func TestCriticStateLegacyDescriptionValueRemainsNonFinal(t *testing.T) {
	st := &turnRecordingStore{}
	pending := map[string]any{"title": "Canal survey", "lifecycle_key": "canal-survey", "description": "Survey both banks after dusk and chart the stone passages"}
	criticStateSave(t, st, 1, map[string]any{"pending_threads": []any{pending}}, "The route was chosen.")
	pending["description"] = "Survey the banks after arrival"
	criticStateSave(t, st, 2, map[string]any{"pending_threads": []any{pending}}, "The crew arrived at camp.")
	if len(st.savedStatusEvents) != 1 {
		t.Fatal("a legacy plan/value rewrite was treated as an independent description")
	}
}

func TestCriticStateEarlierDescriptionIsAReaffirmation(t *testing.T) {
	st := &turnRecordingStore{}
	criticDescriptionSeed(t, st, "progress", "Inspect the channels, radios and engines")
	criticStateSave(t, st, 2, map[string]any{
		"state_claims":    []any{map[string]any{"subject": "Canal survey", "state_slot": "goal_status", "lifecycle_key": "canal-survey", "transition": "progress", "value": "The western channel is clear", "evidence_excerpt": "The western channel is clear."}},
		"pending_threads": []any{map[string]any{"title": "Canal survey", "lifecycle_key": "canal-survey", "description": "Complete the remaining checks"}},
	}, "The report arrived. The western channel is clear. The crew rested.")
	before := st.returnStatusCurrent[0]
	criticStateSave(t, st, 3, map[string]any{"pending_threads": []any{map[string]any{"title": "Canal survey", "lifecycle_key": "canal-survey", "description": "Inspect the channels, radios and engines"}}}, "The original checklist is recalled.")
	if len(st.savedStatusEvents) != 2 || st.returnStatusCurrent[0].ValueJSON != before.ValueJSON {
		t.Fatal("old checklist was accepted as new description")
	}
}

func TestCriticStatePreviousQuoteDoesNotReopenOrReplaceNewerState(t *testing.T) {
	for _, phase := range []string{"complete", "progress"} {
		t.Run(phase, func(t *testing.T) {
			base := &turnRecordingStore{}
			criticDescriptionSeed(t, base, phase, "Survey both banks")
			if phase == "progress" {
				base.returnStatusCurrent[0].SourceTurn = 8
			}
			quote := "The western bank was surveyed."
			st := &completeTurnReprocessingStore{turnRecordingStore: base, sources: map[string]*store.MemorySourceRevision{
				"critic-state-5": {ChatSessionID: "critic-state", CriticInputSnapshotJSON: mustCompactJSON(completeTurnCriticInputSnapshot{ContextMessages: []map[string]any{{"source": "previous_canonical_turn", "turn_index": 4, "content": "At dawn. " + quote + " They rested."}}})},
			}}
			criticStateSave(t, st, 5, map[string]any{"state_claims": []any{map[string]any{"subject": "Canal survey", "lifecycle_key": "canal-survey", "state_slot": "goal_status", "value": "survey in progress", "transition": "progress", "confidence": .9, "evidence_excerpt": quote}}}, "The report is read again.")
			if len(st.savedStatusEvents) != 1 {
				t.Fatal("previous source reopened completion or replaced a newer observation")
			}
		})
	}
}
