package httpapi

import (
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestBuildSessionStateCarriesCompleteRuntimeSupportSections(t *testing.T) {
	state := buildSessionState(
		"sess-runtime-support",
		false,
		[]store.ActiveState{{ID: 1, StateType: "scene", Content: "The gate remains closed.", TurnIndex: 8}},
		[]store.Storyline{{ID: 2, Name: "Gate dispute", Status: "active", CurrentContext: "The guard refuses entry.", KeyPointsJSON: `["refusal"]`, OngoingTensionsJSON: `["identity doubt"]`, LastTurn: 8}},
		[]store.CharacterState{{ID: 3, CharacterName: "Mina", PersonalityJSON: `{"kind":"patient"}`, StatusJSON: `{"emotion":"wary"}`, RelationshipsJSON: `[{"target":"Siwoo","sentiment":"uncertain"}]`, SpeechStyleJSON: `{"default_tone":"formal"}`, TurnIndex: 8}},
		nil,
		[]store.ChatLog{{ID: 6, Role: "assistant", Content: "Mina waits beside the gate.", TurnIndex: 8}},
		[]store.WorldRule{{ID: 4, Scope: "location", ScopeName: "Gate", Category: "access", Key: "entry_requires_pass", ValueJSON: `true`, SourceTurn: 3}},
		[]store.PendingThread{{ID: 5, ThreadKey: "show-pass", Description: "Prove the travel pass is valid.", Status: "open", CreatedTurn: 4, SourceTurn: 8, HookType: "open_question", ThreadType: "open_question", Title: "Travel pass", Owner: "Siwoo", LastSeenTurn: 8}},
		map[string]bool{"active_states": true, "storylines": true, "character_states": true, "character_events": true, "chat_logs": true, "pending_threads": true},
	)

	if state["contract_version"] != "session_state.runtime_support.v1" || state["chat_session_id"] != "sess-runtime-support" {
		t.Fatalf("runtime support contract mismatch: %#v", state)
	}
	complete := mapFromAny(state["complete_sections"])
	for _, key := range []string{"active_states", "storylines", "characters", "pending_threads"} {
		if complete[key] != true {
			t.Fatalf("complete_sections[%s] = %v, want true", key, complete[key])
		}
	}
	if complete["world_rules"] != false {
		t.Fatalf("world_rules must remain session-only, got %v", complete["world_rules"])
	}

	active := state["active_states"].([]map[string]any)[0]
	if active["content"] != "The gate remains closed." {
		t.Fatalf("active-state content missing: %#v", active)
	}
	storyline := state["storylines"].([]map[string]any)[0]
	if storyline["current_context"] != "The guard refuses entry." || storyline["key_points_json"] != `["refusal"]` {
		t.Fatalf("storyline runtime fields missing: %#v", storyline)
	}
	character := state["characters"].([]map[string]any)[0]
	if character["status_json"] == "" || character["speech_style_json"] == "" {
		t.Fatalf("character runtime fields missing: %#v", character)
	}
	thread := state["pending_threads"].([]map[string]any)[0]
	if thread["description"] != "Prove the travel pass is valid." || thread["title"] != "Travel pass" {
		t.Fatalf("pending-thread runtime fields missing: %#v", thread)
	}
}

func TestBuildSessionStateCompletenessAndEndpointParity(t *testing.T) {
	state := buildSessionState(
		"sess-runtime-parity",
		false,
		[]store.ActiveState{
			{ID: 1, StateType: "scene", Content: "Courtyard", TurnIndex: 12},
			{ID: 2, StateType: "clock", Content: "midnight", TurnIndex: 12},
		},
		[]store.Storyline{
			{ID: 3, Name: "Visible", Status: "active", CurrentContext: "Still moving", LastTurn: 12},
			{ID: 4, Name: "Hidden", Status: "active", Suppressed: true, LastTurn: 13},
		},
		[]store.CharacterState{
			{ID: 5, CharacterName: "Mina", StatusJSON: `{"emotion":"old"}`, TurnIndex: 4},
			{ID: 6, CharacterName: "Mina", StatusJSON: `{"emotion":"current"}`, TurnIndex: 12},
		},
		nil,
		[]store.ChatLog{{ID: 7, Role: "assistant", Content: "Mina remains here.", TurnIndex: 12}},
		nil,
		[]store.PendingThread{
			{ID: 8, ThreadKey: "open", Title: "Open thread", ThreadType: "open_question", Status: "open", LastSeenTurn: 12},
			{ID: 9, ThreadKey: "done", Title: "Done thread", ThreadType: "open_question", Status: "resolved", ResolvedTurn: 11},
		},
		map[string]bool{
			"active_states":    true,
			"storylines":       true,
			"character_states": true,
			"chat_logs":        true,
			"pending_threads":  true,
		},
	)

	complete := mapFromAny(state["complete_sections"])
	if complete["characters"] != false {
		t.Fatalf("characters must be incomplete without character-events read: %#v", complete)
	}
	if got := len(state["active_states"].([]map[string]any)); got != 2 {
		t.Fatalf("active states truncated: got %d want 2", got)
	}
	storylines := state["storylines"].([]map[string]any)
	if len(storylines) != 2 || storylines[0]["name"] != "Visible" || storylines[1]["name"] != "Hidden" {
		t.Fatalf("storyline endpoint parity changed: %#v", storylines)
	}
	characters := state["characters"].([]map[string]any)
	if len(characters) != 1 || characters[0]["status_json"] != `{"emotion":"current"}` {
		t.Fatalf("latest character state was not selected: %#v", characters)
	}
	threads := state["pending_threads"].([]map[string]any)
	if len(threads) != 2 || threads[0]["title"] != "Open thread" || threads[1]["title"] != "Done thread" {
		t.Fatalf("pending-thread endpoint parity changed: %#v", threads)
	}
}
