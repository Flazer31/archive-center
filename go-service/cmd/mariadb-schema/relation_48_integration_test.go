package main

import (
	"context"
	"net/http"
	"strings"
	"testing"

	archiveStore "github.com/risulongmemory/archive-center-go/internal/store"
)

// Real source acceptance, SQL projection/copy/rollback and HTTP final delivery.
// Only model output and vectors are synthetic; no user's session is accessed.
func TestRelation48HTTPMariaDBLifecycleDelivery(t *testing.T) {
	t.Setenv("ARCHIVE_CENTER_DATA_DIR", t.TempDir())
	_, st := feedback43Database(t)
	routes, provider, endpoint := storyTime46Server(t, st)
	ctx := context.Background()
	const sid, copied = "relation48-source", "relation48-copy"
	const old = "Mira owns the Brass Compass."
	const next = "Rowan owns the Brass Compass; Mira no longer carries it."
	const replacement = "Lina owns the Brass Compass; Rowan never received it."
	complete := func(turn, generation int, line string) {
		t.Helper()
		extraction := map[string]any{
			"turn_summary": line, "importance_score": 8, "evidence_excerpts": []any{line},
			"narrative_events": []any{map[string]any{"actor": "Mira", "event": line, "visibility": "public", "evidence_excerpt": line}},
			"state_claims":     []any{map[string]any{"subject": "Brass Compass", "state_slot": "ownership", "value": line, "transition": "change", "evidence_excerpt": line}},
		}
		storyTime46Complete(t, routes, provider, endpoint, sid, turn, generation, "They examine the blue return device. "+line+" They record the change before leaving.", extraction)
	}
	prepare := func(session, current, absent string, memoryCount int) {
		t.Helper()
		rows, err := st.(archiveStore.StatusCurrentValueStore).ListStatusCurrentValues(ctx, session, "", "", "narrative_state", -1)
		if err != nil || len(rows) != 1 || !strings.Contains(rows[0].ValueJSON, current) {
			t.Fatalf("canonical current: rows=%+v err=%v", rows, err)
		}
		memories, err := st.ListMemories(ctx, session, -1, -1)
		if err != nil || len(memories) != memoryCount {
			t.Fatalf("canonical memory count=%d want=%d err=%v", len(memories), memoryCount, err)
		}
		query := "Mira asks who owns the Brass Compass, the blue return device."
		result := storyTime46Request(t, routes, http.MethodPost, "/prepare-turn", map[string]any{
			"chat_session_id": session, "turn_index": 4, "raw_user_input": query,
			"messages": []any{map[string]any{"role": "user", "content": query}},
			"settings": map[string]any{"apply_mode": "live", "guide_mode": "off", "max_injection_chars": 32000, "memory_delivery_budget_mode": "auto", "core_objective_memory_max_items": 8},
		})
		plan := storyTime46Map(storyTime46Map(result["injection_pack"])["memory_delivery_plan"])
		text, _ := plan["final_text"].(string)
		if !strings.Contains(text, current) || (absent != "" && strings.Contains(text, absent)) {
			t.Fatalf("HTTP delivery current=%q absent=%q\n%s", current, absent, text)
		}
		if strings.Contains(text, sid) || strings.Contains(text, copied) {
			t.Fatalf("internal session ID reached delivery: %s", text)
		}
		t.Logf("session=%s memories=%d final_chars=%d", session, memoryCount, len([]rune(text)))
	}
	complete(1, 1, old)
	complete(2, 1, next)
	prepare(sid, next, replacement, 2)
	// Identical retry must not duplicate source rows or change the winner.
	complete(2, 1, next)
	prepare(sid, next, replacement, 2)
	result, err := st.(archiveStore.SessionMigrationStore).CompleteSessionMigration(ctx, archiveStore.SessionMigrationCompleteRequest{SourceSessionID: sid, TargetSessionID: copied, Mode: archiveStore.SessionMigrationModeCopyKeepSource})
	if err != nil || result.Status != "copied" {
		t.Fatalf("copy: %+v %v", result, err)
	}
	prepare(copied, next, replacement, 2)
	// A new answer for the same logical turn must replace its old derivation.
	complete(2, 2, replacement)
	prepare(sid, replacement, next, 2)
	prepare(copied, next, replacement, 2)
	// Removing that turn restores the surviving source, not the copied winner.
	if err := st.(archiveStore.LogicalTurnReplacementStore).RollbackCanonicalTail(ctx, archiveStore.LogicalTurnRollback{ChatSessionID: sid, TurnIndex: 2, LifecycleAction: archiveStore.LogicalTurnLifecycleDeleted}); err != nil {
		t.Fatal(err)
	}
	prepare(sid, old, replacement, 1)
	prepare(copied, next, replacement, 2)
	// A normal new turn can establish a new value after the deletion.
	complete(2, 3, replacement)
	prepare(sid, replacement, next, 2)
}
