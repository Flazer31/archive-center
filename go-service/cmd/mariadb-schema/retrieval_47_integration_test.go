package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// Real disposable MariaDB and registered production HTTP routes. The Critic
// and vector boundaries are deterministic; this does not test model accuracy.
func TestRetrieval47HTTPMariaDBHistoricalSourceKeepsCurrentState(t *testing.T) {
	for _, slot := range []string{"ownership", "소유자", "所有者"} {
		t.Run(slot, func(t *testing.T) { retrieval47HTTPCurrentState(t, slot) })
	}
}

func retrieval47HTTPCurrentState(t *testing.T, slot string) {
	t.Helper()
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	_, st := feedback43Database(t)
	routes, provider, endpoint := storyTime46Server(t, st)
	const sid = "retrieval-47-history-current"
	const history = "The Brass Compass is a blue return device that Mira received at the harbor."
	const current = "The Brass Compass now belongs to Rowan; Mira no longer carries it."
	for i, line := range []string{history, current} {
		extraction := map[string]any{
			"turn_summary": line, "importance_score": 8, "evidence_excerpts": []any{line},
			"narrative_events": []any{map[string]any{"actor": "Mira", "event": line, "visibility": "public", "evidence_excerpt": line}},
		}
		if i == 1 {
			extraction["state_claims"] = []any{map[string]any{"subject": "Brass Compass", "state_slot": slot, "value": current, "transition": "change", "evidence_excerpt": line}}
		}
		bodyTracking46Complete(t, routes, provider, endpoint, sid, i+1, int64(10000*(i+1)), []string{sid + string(rune('A'+i))}, []string{"Continue the synthetic scene."}, "They review the journey. "+line+" Then they leave the harbor.", extraction)
	}
	before, err := st.ListMemories(context.Background(), sid, -1, -1)
	if err != nil || len(before) != 2 {
		t.Fatalf("stored source count=%d error=%v", len(before), err)
	}
	for _, mode := range []string{"auto", "custom"} {
		query := "Mira tries to use the old blue return device from the harbor."
		result := storyTime46Request(t, routes, http.MethodPost, "/prepare-turn", map[string]any{
			"chat_session_id": sid, "turn_index": 3, "raw_user_input": query,
			"messages": []any{map[string]any{"role": "user", "content": query}},
			"settings": map[string]any{"apply_mode": "live", "guide_mode": "off", "max_injection_chars": 18000, "memory_delivery_budget_mode": mode, "core_objective_memory_max_items": 8},
		})
		plan := storyTime46Map(storyTime46Map(result["injection_pack"])["memory_delivery_plan"])
		text, _ := plan["final_text"].(string)
		if !strings.Contains(text, history) || !strings.Contains(text, current) || !strings.Contains(text, "linked stored state") {
			raw, _ := json.Marshal(result)
			t.Fatalf("%s HTTP source/current bundle missing: %s", mode, raw)
		}
		if !strings.Contains(text, "state evidence") || !strings.Contains(text, "source turn 2") {
			t.Fatalf("state provenance lost: %s", text)
		}
	}
	after, err := st.ListMemories(context.Background(), sid, -1, -1)
	if err != nil || len(before) != len(after) {
		t.Fatalf("reading mutated stored sources: %v", err)
	}
	for i := range before {
		if before[i].SummaryJSON != after[i].SummaryJSON {
			t.Fatal("retrieval rewrote history")
		}
	}
}
