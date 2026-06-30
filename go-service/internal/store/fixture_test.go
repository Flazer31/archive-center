package store

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestFixtureStoreLoadsCanonicalAndNarrativeRows(t *testing.T) {
	dir := t.TempDir()
	writeFixtureTable(t, dir, "chat_logs", []map[string]any{
		{"id": 1, "chat_session_id": "s1", "turn_index": 1, "role": "user", "content": "hello", "created_at": "2026-05-26 00:00:00"},
		{"id": 2, "chat_session_id": "s2", "turn_index": 1, "role": "assistant", "content": "world", "created_at": "2026-05-26 00:00:01"},
	})
	writeFixtureTable(t, dir, "memories", []map[string]any{
		{"id": 10, "chat_session_id": "s1", "turn_index": 1, "summary_json": `{"summary":"hello"}`, "importance": 0.7, "created_at": "2026-05-26 00:00:02"},
	})
	writeFixtureTable(t, dir, "kg_triples", []map[string]any{
		{"id": 20, "chat_session_id": "s1", "subject": "Chloe", "predicate": "knows", "object": "Risu", "valid_from": 1, "source_turn": 1, "created_at": "2026-05-26 00:00:03"},
	})
	writeFixtureTable(t, dir, "pending_threads", []map[string]any{
		{"id": 30, "chat_session_id": "s1", "thread_type": "quest", "title": "Find key", "status": "open", "source_turn": 1, "last_seen_turn": 2, "confidence": 0.8, "details_json": `{}`, "pinned": true, "suppressed": false, "user_corrected": false, "created_at": "2026-05-26 00:00:04", "updated_at": "2026-05-26 00:00:05"},
	})
	writeFixtureTable(t, dir, "character_states", []map[string]any{
		{"id": 40, "chat_session_id": "s1", "character_name": "Chloe", "status_json": `{}`, "turn_index": 2, "created_at": "2026-05-26 00:00:06", "updated_at": "2026-05-26 00:00:07"},
	})

	st, err := NewFixtureStoreFromExportDir(dir)
	if err != nil {
		t.Fatalf("NewFixtureStoreFromExportDir: %v", err)
	}

	stats, err := st.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.ChatLogs != 2 || stats.Memories != 1 || stats.KgTriples != 1 {
		t.Fatalf("stats mismatch: %#v", stats)
	}

	allLogs, err := st.ListChatLogs(context.Background(), "", 0, 0)
	if err != nil {
		t.Fatalf("ListChatLogs all: %v", err)
	}
	if len(allLogs) != 2 {
		t.Fatalf("all logs count = %d, want 2", len(allLogs))
	}

	s1Logs, err := st.ListChatLogs(context.Background(), "s1", 0, 0)
	if err != nil {
		t.Fatalf("ListChatLogs s1: %v", err)
	}
	if len(s1Logs) != 1 || s1Logs[0].ChatSessionID != "s1" {
		t.Fatalf("s1 logs mismatch: %#v", s1Logs)
	}

	sessions, err := st.ListSessions(context.Background())
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("sessions count = %d, want 2 separate sessions: %#v", len(sessions), sessions)
	}
	byID := map[string]SessionSummary{}
	for _, session := range sessions {
		byID[session.ChatSessionID] = session
	}
	if byID["s1"].ChatLogsCount != 1 || byID["s1"].MemoriesCount != 1 || byID["s1"].KGTriplesCount != 1 {
		t.Fatalf("s1 session counts mismatch: %#v", byID["s1"])
	}
	if byID["s2"].ChatLogsCount != 1 || byID["s2"].MemoriesCount != 0 || byID["s2"].KGTriplesCount != 0 {
		t.Fatalf("s2 session counts mismatch: %#v", byID["s2"])
	}
	if !byID["s1"].LastActivity.After(byID["s2"].LastActivity) {
		t.Fatalf("s1 last activity should include memory/KG timestamps and sort after s2: s1=%v s2=%v", byID["s1"].LastActivity, byID["s2"].LastActivity)
	}

	hooks, err := st.ListPendingThreads(context.Background(), "s1", "")
	if err != nil {
		t.Fatalf("ListPendingThreads: %v", err)
	}
	if len(hooks) != 1 || hooks[0].ThreadKey != "Find key" || hooks[0].HookType != "quest" {
		t.Fatalf("pending thread mapping mismatch: %#v", hooks)
	}

	characters, err := st.ListCharacterStates(context.Background(), "s1")
	if err != nil {
		t.Fatalf("ListCharacterStates: %v", err)
	}
	if len(characters) != 1 || characters[0].CharacterName != "Chloe" {
		t.Fatalf("character mapping mismatch: %#v", characters)
	}

	if err := st.SaveChatLog(context.Background(), &ChatLog{}); err != ErrNotEnabled {
		t.Fatalf("fixture writes should be disabled, got %v", err)
	}
}

func TestFixtureStoreMissingTablesAreAllowed(t *testing.T) {
	st, err := NewFixtureStoreFromExportDir(t.TempDir())
	if err != nil {
		t.Fatalf("empty fixture dir should load: %v", err)
	}
	stats, err := st.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats != (StatsResult{}) {
		t.Fatalf("empty stats mismatch: %#v", stats)
	}
}

func writeFixtureTable(t *testing.T, dir, table string, rows []map[string]any) {
	t.Helper()
	path := filepath.Join(dir, table+".ndjson")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create fixture table: %v", err)
	}
	defer f.Close()
	meta := map[string]any{"_export_meta": map[string]any{"table_name": table, "row_count": len(rows)}}
	if err := json.NewEncoder(f).Encode(meta); err != nil {
		t.Fatalf("write fixture meta: %v", err)
	}
	for _, row := range rows {
		if err := json.NewEncoder(f).Encode(row); err != nil {
			t.Fatalf("write fixture row: %v", err)
		}
	}
}
