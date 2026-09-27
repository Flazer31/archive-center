package main

import (
	"context"
	"net/http"
	"strings"
	"testing"

	archiveStore "github.com/risulongmemory/archive-center-go/internal/store"
)

// Uses real complete/prepare HTTP, MariaDB and lifecycle storage. Provider
// responses and vectors are controlled external boundaries, not real-model QA.
func TestCriticPreservation48HTTPMariaDBCompletionAndReroll(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	_, st := feedback43Database(t)
	routes, provider, endpoint := storyTime46Server(t, st)
	ctx := context.Background()
	const sid, key = "critic-preservation48", "ferry-repair"
	complete := func(turn, generation int, phase, value string) {
		line := "Rowan reports: " + value + "."
		reaction := "Mira feels relieved after Rowan completed the ferry repairs."
		x := map[string]any{
			"turn_summary": value, "importance_score": 8, "evidence_excerpts": []any{line, reaction},
			"entities":                   map[string]any{"characters": []any{map[string]any{"name": "Rowan"}, map[string]any{"name": "Mira"}}},
			"state_claims":               []any{map[string]any{"subject": "Rowan", "state_slot": "repair_status", "lifecycle_key": key, "value": value, "transition": phase, "evidence_excerpt": line}},
			"subjective_entity_memories": []any{map[string]any{"owner_entity_name": "Mira", "owner_visibility": "owner_private", "memory_text": reaction, "evidence_excerpt": reaction}},
		}
		storyTime46Complete(t, routes, provider, endpoint, sid, turn, generation, "At the ferry, the crew gathers. "+line+" "+reaction+" The crew departs.", x)
	}
	assertState := func(phase, value string, count int) {
		t.Helper()
		rows, err := st.(archiveStore.StatusCurrentValueStore).ListStatusCurrentValues(ctx, sid, "", "", "narrative_state", -1)
		if err != nil || len(rows) != 1 {
			t.Fatalf("current rows=%d err=%v", len(rows), err)
		}
		p := storyTime46JSON(t, rows[0].ValueJSON)
		if p["transition"] != phase || p["value"] != value {
			t.Fatalf("wrong lifecycle: %#v", p)
		}
		memories, err := st.ListMemories(ctx, sid, -1, -1)
		if err != nil || len(memories) != count {
			t.Fatalf("retry/replacement count=%d want=%d err=%v", len(memories), count, err)
		}
		query := "Mira asks about Rowan's ferry repair progress."
		response := storyTime46Request(t, routes, http.MethodPost, "/prepare-turn", map[string]any{"chat_session_id": sid, "turn_index": 4, "raw_user_input": query, "messages": []any{map[string]any{"role": "user", "content": query}}, "settings": map[string]any{"apply_mode": "live", "guide_mode": "off", "max_injection_chars": 32000, "memory_delivery_budget_mode": "auto"}})
		text, _ := storyTime46Map(storyTime46Map(response["injection_pack"])["memory_delivery_plan"])["final_text"].(string)
		if !strings.Contains(text, value) {
			t.Fatalf("stored current state did not reach final delivery: %s", text)
		}
	}
	complete(1, 1, "partial", "Deck repaired; railing still needs work")
	assertState("partial", "Deck repaired; railing still needs work", 1)
	complete(2, 1, "complete", "All ferry repairs finished")
	assertState("complete", "All ferry repairs finished", 2)
	complete(2, 1, "complete", "All ferry repairs finished")
	assertState("complete", "All ferry repairs finished", 2)
	complete(2, 2, "partial", "Railing remains unfinished after inspection")
	assertState("partial", "Railing remains unfinished after inspection", 2)
	complete(3, 1, "complete", "Railing repaired and ferry reopened")
	assertState("complete", "Railing repaired and ferry reopened", 3)
}
