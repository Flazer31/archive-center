package httpapi

import (
	"context"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func Test47CriticSelectedMemoryKeepsRecordedStateAndPromiseIdentity(t *testing.T) {
	const raw = `{"turn_summary":"Mira holds the brass compass and promises to return it.","state_claims":[{"subject":"Brass Compass","state_slot":"보관","value":"Mira","transition":"set","evidence_excerpt":"Mira holds the brass compass."}],"pending_threads":[{"title":"Return brass compass","lifecycle_key":"return-compass-1","status":"open","description":"Mira promises to return the brass compass.","evidence_excerpt":"Mira promises to return the brass compass."}]}`
	st := &turnRecordingStore{
		returnMemories: []store.Memory{{ID: 1, TurnIndex: 1, SummaryJSON: raw, Importance: .8}},
		returnChatLogs: []store.ChatLog{
			{ChatSessionID: "refs47", TurnIndex: 1, Role: "user", Content: "Mira takes the brass compass."},
			{ChatSessionID: "refs47", TurnIndex: 1, Role: "assistant", Content: "Mira holds the brass compass. Mira promises to return the brass compass."},
		},
	}
	s := NewServer(config.Default()) // Ledger switch remains off.
	s.Store = st
	_, memories, _ := s.buildCompleteTurnCriticCanonicalContext(context.Background(), "refs47", 2, "Mira returns the brass compass.", 0)
	if len(memories) != 1 {
		t.Fatalf("selected memory count=%d", len(memories))
	}
	state := sliceFromAny(memories[0]["recorded_state_claims"])
	threads := sliceFromAny(memories[0]["recorded_pending_threads"])
	if len(state) != 1 || stringFromMap(mapFromAny(state[0]), "state_slot") != "보관" || len(threads) != 1 || stringFromMap(mapFromAny(threads[0]), "lifecycle_key") != "return-compass-1" {
		t.Fatalf("identity was discarded before Critic could reuse it: %#v", memories)
	}
	if !boolFromAny(memories[0]["support_only"]) || intFromAny(memories[0]["turn_index"], 0) != 1 {
		t.Fatal("historical reference lost support-only source attribution")
	}
	if strings.Contains(mustCompactJSON(memories), "evidence_excerpt") {
		t.Fatal("identity support unnecessarily duplicated raw evidence")
	}
	if st.returnMemories[0].SummaryJSON != raw {
		t.Fatal("support projection mutated stored history")
	}
}
