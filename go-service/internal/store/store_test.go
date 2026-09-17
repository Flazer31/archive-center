package store

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// ---------------------------------------------------------------------------
// No-op store tests
// ---------------------------------------------------------------------------

func TestNoopStoreImplementsInterface(t *testing.T) {
	var _ Store = NewNoopStore()
}

func TestMariaDBUpdateDirectEvidenceExplorerFieldsWritesEvidenceText(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	value := "corrected evidence"
	mock.ExpectExec(regexp.QuoteMeta(`
		UPDATE direct_evidence_records
		SET evidence_text = ?
		WHERE id = ? AND chat_session_id = ?
	`)).WithArgs(value, int64(9), "sess-edit").WillReturnResult(sqlmock.NewResult(0, 1))

	if err := m.UpdateDirectEvidenceExplorerFields(context.Background(), "sess-edit", 9, DirectEvidenceExplorerPatch{EvidenceText: &value}); err != nil {
		t.Fatalf("update direct evidence: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestNoopStoreSaveChatLog(t *testing.T) {
	s := NewNoopStore()
	if err := s.SaveChatLog(context.Background(), &ChatLog{ChatSessionID: "s1", TurnIndex: 1, Role: "user", Content: "hi"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNoopStoreListChatLogs(t *testing.T) {
	s := NewNoopStore()
	logs, err := s.ListChatLogs(context.Background(), "s1", 0, 10)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if logs != nil {
		t.Errorf("expected nil, got %v", logs)
	}
}

func TestNoopStoreSaveMemory(t *testing.T) {
	s := NewNoopStore()
	if err := s.SaveMemory(context.Background(), &Memory{ChatSessionID: "s1", TurnIndex: 1}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNoopStoreListMemories(t *testing.T) {
	s := NewNoopStore()
	mems, err := s.ListMemories(context.Background(), "s1", 0, 10)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if mems != nil {
		t.Errorf("expected nil, got %v", mems)
	}
}

func TestNoopStoreSaveEvidence(t *testing.T) {
	s := NewNoopStore()
	if err := s.SaveEvidence(context.Background(), &DirectEvidence{ChatSessionID: "s1", EvidenceText: "e1"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNoopStoreSaveKGTriple(t *testing.T) {
	s := NewNoopStore()
	if err := s.SaveKGTriple(context.Background(), &KGTriple{ChatSessionID: "s1", Subject: "A", Predicate: "B", Object: "C"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNoopStoreSaveAuditLog(t *testing.T) {
	s := NewNoopStore()
	if err := s.SaveAuditLog(context.Background(), &AuditLog{EventType: "test"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNoopStoreSaveCriticFeedback(t *testing.T) {
	s := NewNoopStore()
	if err := s.SaveCriticFeedback(context.Background(), &CriticFeedback{ChatSessionID: "s1", TargetType: "memory", TargetID: 1}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNoopStoreSaveCharacterEvent(t *testing.T) {
	s := NewNoopStore()
	if err := s.SaveCharacterEvent(context.Background(), &CharacterEvent{ChatSessionID: "s1", CharacterName: "X"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestNoopStoreGetEffectiveInputNotFound(t *testing.T) {
	s := NewNoopStore()
	in, err := s.GetEffectiveInput(context.Background(), "s1", 1)
	if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
	if in != nil {
		t.Errorf("expected nil, got %v", in)
	}
}

// ---------------------------------------------------------------------------
// MariaDB disabled tests
// ---------------------------------------------------------------------------

func TestMariaDBOpenEmptyDSNDisabled(t *testing.T) {
	_, err := OpenMariaDB("")
	if !errors.Is(err, ErrNotEnabled) {
		t.Errorf("expected ErrNotEnabled, got %v", err)
	}
}

func TestMariaDBOpenWithDSNReturnsStore(t *testing.T) {
	s, err := OpenMariaDB("user:pass@tcp(127.0.0.1:3306)/archive_center?parseTime=true")
	if err != nil {
		t.Fatalf("OpenMariaDB failed: %v", err)
	}
	if s == nil {
		t.Fatal("expected store, got nil")
	}
}

func TestMariaDBStoreImplementsInterface(t *testing.T) {
	// Compile-time check: mariadbStore satisfies Store.
	// We cannot instantiate it via OpenMariaDB, so we assert via type conversion.
	var _ Store = &mariadbStore{}
}

func TestMariaDBStoreAllMethodsDisabled(t *testing.T) {
	m := &mariadbStore{}
	ctx := context.Background()

	if err := m.SaveChatLog(ctx, &ChatLog{}); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("SaveChatLog: expected ErrNotEnabled, got %v", err)
	}
	if _, err := m.ListChatLogs(ctx, "", 0, 0); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("ListChatLogs: expected ErrNotEnabled, got %v", err)
	}
	if err := m.SaveEffectiveInput(ctx, &EffectiveInput{}); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("SaveEffectiveInput: expected ErrNotEnabled, got %v", err)
	}
	if _, err := m.GetEffectiveInput(ctx, "", 0); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("GetEffectiveInput: expected ErrNotEnabled, got %v", err)
	}
	if err := m.SaveMemory(ctx, &Memory{}); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("SaveMemory: expected ErrNotEnabled, got %v", err)
	}
	if _, err := m.ListMemories(ctx, "", 0, 0); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("ListMemories: expected ErrNotEnabled, got %v", err)
	}
	if err := m.SaveEvidence(ctx, &DirectEvidence{}); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("SaveEvidence: expected ErrNotEnabled, got %v", err)
	}
	if _, err := m.ListEvidence(ctx, ""); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("ListEvidence: expected ErrNotEnabled, got %v", err)
	}
	if err := m.SaveKGTriple(ctx, &KGTriple{}); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("SaveKGTriple: expected ErrNotEnabled, got %v", err)
	}
	if _, err := m.ListKGTriples(ctx, ""); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("ListKGTriples: expected ErrNotEnabled, got %v", err)
	}
	if err := m.SaveAuditLog(ctx, &AuditLog{}); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("SaveAuditLog: expected ErrNotEnabled, got %v", err)
	}
	if _, err := m.ListAuditLogs(ctx, "", "", 0); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("ListAuditLogs: expected ErrNotEnabled, got %v", err)
	}
	if err := m.SaveCriticFeedback(ctx, &CriticFeedback{}); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("SaveCriticFeedback: expected ErrNotEnabled, got %v", err)
	}
	if _, err := m.ListCriticFeedback(ctx, "", "", 0); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("ListCriticFeedback: expected ErrNotEnabled, got %v", err)
	}
	if err := m.SaveCharacterEvent(ctx, &CharacterEvent{}); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("SaveCharacterEvent: expected ErrNotEnabled, got %v", err)
	}
	if _, err := m.ListCharacterEvents(ctx, "", ""); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("ListCharacterEvents: expected ErrNotEnabled, got %v", err)
	}
}

func TestNoopStoreStats(t *testing.T) {
	s := NewNoopStore()
	stats, err := s.Stats(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if stats.ChatLogs != 0 || stats.Memories != 0 || stats.KgTriples != 0 {
		t.Errorf("expected zero stats, got %+v", stats)
	}
}

func TestNoopStoreListSessions(t *testing.T) {
	s := NewNoopStore()
	sessions, err := s.ListSessions(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if sessions != nil {
		t.Errorf("expected nil, got %v", sessions)
	}
}

func TestMariaDBStoreStatsDisabled(t *testing.T) {
	m := &mariadbStore{}
	_, err := m.Stats(context.Background())
	if !errors.Is(err, ErrNotEnabled) {
		t.Errorf("expected ErrNotEnabled, got %v", err)
	}
}

func TestMariaDBStoreListSessionsDisabled(t *testing.T) {
	m := &mariadbStore{}
	_, err := m.ListSessions(context.Background())
	if !errors.Is(err, ErrNotEnabled) {
		t.Errorf("expected ErrNotEnabled, got %v", err)
	}
}
func TestNoopStoreGetResumePack(t *testing.T) {
	s := NewNoopStore()
	pack, err := s.GetResumePack(context.Background(), "s1", "resume")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if pack != nil {
		t.Errorf("expected nil, got %v", pack)
	}
}

func TestMariaDBStoreGetResumePackDisabled(t *testing.T) {
	m := &mariadbStore{}
	_, err := m.GetResumePack(context.Background(), "s1", "resume")
	if !errors.Is(err, ErrNotEnabled) {
		t.Errorf("expected ErrNotEnabled, got %v", err)
	}
}

func TestMariaDBStoreSaveChatLogExecutesInsert(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	created := time.Date(2026, 5, 24, 10, 0, 0, 0, time.UTC)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO chat_logs")).
		WithArgs("sess-1", 1, "user", "hello", created).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, content")).
		WithArgs("sess-1", 1, "user").
		WillReturnRows(sqlmock.NewRows([]string{"id", "content"}).AddRow(1, "hello"))

	log := &ChatLog{
		ChatSessionID: "sess-1",
		TurnIndex:     1,
		Role:          "user",
		Content:       "hello",
		CreatedAt:     created,
	}
	err = m.SaveChatLog(context.Background(), log)
	if err != nil {
		t.Fatalf("SaveChatLog failed: %v", err)
	}
	if log.ID != 1 {
		t.Fatalf("inserted log ID = %d, want 1", log.ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBStoreSaveChatLogSkipsExactDuplicate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO chat_logs")).
		WithArgs("sess-1", 1, "assistant", "hello", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(42, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, content")).
		WithArgs("sess-1", 1, "assistant").
		WillReturnRows(sqlmock.NewRows([]string{"id", "content"}).AddRow(42, "hello"))

	log := &ChatLog{ChatSessionID: "sess-1", TurnIndex: 1, Role: "assistant", Content: "hello"}
	if err := m.SaveChatLog(context.Background(), log); err != nil {
		t.Fatalf("SaveChatLog duplicate failed: %v", err)
	}
	if log.ID != 42 {
		t.Fatalf("duplicate log ID = %d, want 42", log.ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBStoreSaveChatLogRejectsRoleConflict(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO chat_logs")).
		WithArgs("sess-1", 1, "assistant", "new text", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(42, 0))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, content")).
		WithArgs("sess-1", 1, "assistant").
		WillReturnRows(sqlmock.NewRows([]string{"id", "content"}).AddRow(42, "old text"))

	err = m.SaveChatLog(context.Background(), &ChatLog{ChatSessionID: "sess-1", TurnIndex: 1, Role: "assistant", Content: "new text"})
	if err == nil || !strings.Contains(err.Error(), "chat log role conflict") {
		t.Fatalf("expected chat log role conflict, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBStoreSaveChatLogConcurrentExactDuplicateConvergesOnOneRow(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.MatchExpectationsInOrder(false)

	m := &mariadbStore{db: db}
	for i := 0; i < 2; i++ {
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO chat_logs")).
			WithArgs("sess-concurrent", 7, "assistant", "same final", sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(91, int64(1-i)))
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id, content")).
			WithArgs("sess-concurrent", 7, "assistant").
			WillReturnRows(sqlmock.NewRows([]string{"id", "content"}).AddRow(91, "same final"))
	}

	logs := []*ChatLog{
		{ChatSessionID: "sess-concurrent", TurnIndex: 7, Role: " Assistant ", Content: "same final"},
		{ChatSessionID: "sess-concurrent", TurnIndex: 7, Role: "assistant", Content: "same final"},
	}
	errs := make(chan error, len(logs))
	var wg sync.WaitGroup
	for _, log := range logs {
		wg.Add(1)
		go func(log *ChatLog) {
			defer wg.Done()
			errs <- m.SaveChatLog(context.Background(), log)
		}(log)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent SaveChatLog failed: %v", err)
		}
	}
	for _, log := range logs {
		if log.ID != 91 || log.Role != "assistant" {
			t.Fatalf("concurrent log = %#v, want ID 91 and normalized assistant role", log)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBStoreSaveEffectiveInputExecutesInsert(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	created := time.Date(2026, 5, 24, 10, 1, 0, 0, time.UTC)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO effective_input_logs")).
		WithArgs("sess-1", 2, "refined", created).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = m.SaveEffectiveInput(context.Background(), &EffectiveInput{
		ChatSessionID:  "sess-1",
		TurnIndex:      2,
		EffectiveInput: "refined",
		CreatedAt:      created,
	})
	if err != nil {
		t.Fatalf("SaveEffectiveInput failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBStoreSaveCharacterStateAppendsMergedSnapshot(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	updated := time.Date(2026, 5, 24, 10, 2, 0, 0, time.UTC)
	mock.ExpectQuery("FROM character_states").
		WithArgs("sess-1", "Chloe").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "chat_session_id", "character_name", "appearance_json", "personality_json", "status_json",
			"relationships_json", "speech_style_json", "turn_index", "created_at", "updated_at",
		}).AddRow(11, "sess-1", "Chloe", `{"hair":"brown"}`, `{"kind":"sharp"}`, `{"emotion":"calm"}`, `{"Hero":{"affection":40}}`, `{"tone":"soft"}`, 8, updated, updated))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO character_states")).
		WithArgs("sess-1", "Chloe", `{"hair":"black"}`, `{"kind":"sharp"}`, `{"emotion":"focused"}`, `{"Hero":{"affection":70}}`, `{"tone":"dry"}`, 9, updated, updated).
		WillReturnResult(sqlmock.NewResult(12, 1))

	err = m.SaveCharacterState(context.Background(), &CharacterState{
		ChatSessionID:     "sess-1",
		CharacterName:     "Chloe",
		AppearanceJSON:    `{"hair":"black"}`,
		StatusJSON:        `{"emotion":"focused"}`,
		RelationshipsJSON: `{"Hero":{"affection":70}}`,
		SpeechStyleJSON:   `{"tone":"dry"}`,
		TurnIndex:         9,
		UpdatedAt:         updated,
	})
	if err != nil {
		t.Fatalf("SaveCharacterState append failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBStoreSaveCharacterStateInsertsWhenNoExistingRow(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	created := time.Date(2026, 5, 24, 10, 3, 0, 0, time.UTC)
	mock.ExpectQuery("FROM character_states").
		WithArgs("sess-1", "Mina").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "chat_session_id", "character_name", "appearance_json", "personality_json", "status_json",
			"relationships_json", "speech_style_json", "turn_index", "created_at", "updated_at",
		}))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO character_states")).
		WithArgs("sess-1", "Mina", nil, nil, `{"emotion":"new"}`, nil, nil, 1, created, created).
		WillReturnResult(sqlmock.NewResult(7, 1))

	err = m.SaveCharacterState(context.Background(), &CharacterState{
		ChatSessionID: "sess-1",
		CharacterName: "Mina",
		StatusJSON:    `{"emotion":"new"}`,
		TurnIndex:     1,
		CreatedAt:     created,
		UpdatedAt:     created,
	})
	if err != nil {
		t.Fatalf("SaveCharacterState insert failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBStoreListCharacterStatesReturnsLatestSnapshotPerCharacter(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	now := time.Date(2026, 5, 24, 10, 4, 0, 0, time.UTC)
	mock.ExpectQuery("FROM character_states").
		WithArgs("sess-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "chat_session_id", "character_name", "appearance_json", "personality_json", "status_json",
			"relationships_json", "speech_style_json", "turn_index", "created_at", "updated_at",
		}).
			AddRow(12, "sess-1", "Chloe", `{"hair":"black"}`, `{"kind":"sharp"}`, `{"emotion":"focused"}`, `{}`, `{}`, 9, now, now).
			AddRow(11, "sess-1", "Chloe", `{"hair":"brown"}`, `{"kind":"sharp"}`, `{"emotion":"calm"}`, `{}`, `{}`, 8, now, now).
			AddRow(10, "sess-1", "Mina", `{}`, `{}`, `{"emotion":"new"}`, `{}`, `{}`, 1, now, now))

	items, err := m.ListCharacterStates(context.Background(), "sess-1")
	if err != nil {
		t.Fatalf("ListCharacterStates failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items len = %d, want 2: %+v", len(items), items)
	}
	if items[0].ID != 12 || items[0].CharacterName != "Chloe" || items[0].TurnIndex != 9 {
		t.Fatalf("first item = %+v, want latest Chloe snapshot", items[0])
	}
	if items[1].ID != 10 || items[1].CharacterName != "Mina" {
		t.Fatalf("second item = %+v, want Mina", items[1])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBStoreListCharacterStateHistoryReturnsSnapshotsNewestFirst(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	now := time.Date(2026, 5, 24, 10, 5, 0, 0, time.UTC)
	mock.ExpectQuery("FROM character_states").
		WithArgs("sess-1", "Chloe", 2, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "chat_session_id", "character_name", "appearance_json", "personality_json", "status_json",
			"relationships_json", "speech_style_json", "turn_index", "created_at", "updated_at",
		}).
			AddRow(12, "sess-1", "Chloe", `{"hair":"black"}`, `{"kind":"sharp"}`, `{"emotion":"focused"}`, `{}`, `{}`, 9, now, now).
			AddRow(11, "sess-1", "Chloe", `{"hair":"brown"}`, `{"kind":"sharp"}`, `{"emotion":"calm"}`, `{}`, `{}`, 8, now, now))

	items, err := m.ListCharacterStateHistory(context.Background(), "sess-1", "Chloe", 2, 0)
	if err != nil {
		t.Fatalf("ListCharacterStateHistory failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("items len = %d, want 2: %+v", len(items), items)
	}
	if items[0].ID != 12 || items[0].TurnIndex != 9 || items[1].ID != 11 || items[1].TurnIndex != 8 {
		t.Fatalf("history order = %+v, want newest snapshots first", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBStoreStatsQueriesCanonicalCounts(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM chat_logs")).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM memories")).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(4))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM kg_triples")).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))

	stats, err := m.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats failed: %v", err)
	}
	if stats.ChatLogs != 3 || stats.Memories != 4 || stats.KgTriples != 5 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBStoreLockSessionMigrationSourceFailsClosedAfterVectorReindexUntilManifestParity(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	manifestEntries := len(SessionMigrationManifest())
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT source_session_id, target_session_id, mode, status, chroma_reindexed_count").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{
			"source_session_id", "target_session_id", "mode", "status", "chroma_reindexed_count",
		}).AddRow("source", "target", SessionMigrationModeCopyThenLockSource, "vector_reindexed", 2))
	mock.ExpectQuery(`(?s)FROM session_migration_locks.*source_session_id = \?.*FOR UPDATE`).
		WithArgs("source").
		WillReturnRows(sqlmock.NewRows([]string{
			"migration_id", "source_session_id", "target_session_id", "locked", "lock_status", "reason", "locked_at",
		}).AddRow(int64(42), "source", "target", true, "lock_pending_verification", "operator confirmed", time.Now().UTC()))
	mock.ExpectQuery(`(?s)FROM memory_reprocessing_jobs.*chat_session_id = \?.*status = 'leased'.*FOR UPDATE`).
		WithArgs("source").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery(`(?s)FROM memory_vector_outbox.*chat_session_id = \?.*status = 'leased'.*FOR UPDATE`).
		WithArgs("source").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery("SELECT.*COUNT\\(\\*\\).*FROM session_migration_artifact_parity").
		WithArgs(int64(42), SessionMigrationManifestVersion).
		WillReturnRows(sqlmock.NewRows([]string{"total", "relational_verified", "vector_verified"}).AddRow(manifestEntries-1, manifestEntries-1, manifestEntries-1))
	mock.ExpectRollback()
	result, err := m.LockSessionMigrationSource(context.Background(), 42, "operator confirmed")
	want := fmt.Sprintf("manifest parity rows %d/%d", manifestEntries-1, manifestEntries)
	if result != nil || err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("result=%+v err=%v, want %d-entry manifest parity blocker", result, err, manifestEntries)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBStoreLockSessionMigrationSourceBlocksWhenDerivationLeaseIsActive(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT source_session_id, target_session_id, mode, status, chroma_reindexed_count").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{
			"source_session_id", "target_session_id", "mode", "status", "chroma_reindexed_count",
		}).AddRow("source", "target", SessionMigrationModeCopyThenLockSource, "vector_reindexed", 2))
	mock.ExpectQuery(`(?s)FROM session_migration_locks.*source_session_id = \?.*FOR UPDATE`).
		WithArgs("source").
		WillReturnRows(sqlmock.NewRows([]string{
			"migration_id", "source_session_id", "target_session_id", "locked", "lock_status", "reason", "locked_at",
		}).AddRow(int64(42), "source", "target", true, "lock_pending_verification", "operator confirmed", time.Now().UTC()))
	mock.ExpectQuery(`(?s)FROM memory_reprocessing_jobs.*chat_session_id = \?.*status = 'leased'.*FOR UPDATE`).
		WithArgs("source").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(17)))
	mock.ExpectRollback()

	result, err := m.LockSessionMigrationSource(context.Background(), 42, "operator confirmed")
	var blocker *SessionMigrationBlockerError
	if result != nil || !errors.As(err, &blocker) || blocker.Code != "source_derivation_lease_active" || blocker.Phase != "memory_reprocessing_drain" {
		t.Fatalf("result=%+v err=%v blocker=%+v", result, err, blocker)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBStoreGetsRoutingBaselineFromCopiedChatLogLedger(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	mock.ExpectQuery(`(?s)FROM session_migrations sm.*JOIN session_migration_row_map rm.*JOIN chat_logs cl.*WHERE sm.target_session_id = \?`).
		WithArgs("target-session").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "source_session_id", "target_session_id", "mode", "imported_through_turn",
		}).AddRow(int64(42), "source-session", "target-session", SessionMigrationModeCopyKeepSource, 8))

	baseline, err := m.GetSessionRoutingBaseline(context.Background(), "target-session")
	if err != nil {
		t.Fatalf("GetSessionRoutingBaseline failed: %v", err)
	}
	if baseline.MigrationID != 42 || baseline.SourceSessionID != "source-session" || baseline.TargetSessionID != "target-session" || baseline.ImportedThroughTurn != 8 {
		t.Fatalf("unexpected routing baseline: %+v", baseline)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBStoreLockSessionMigrationSourceBlocksBeforeVectorReindex(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT source_session_id, target_session_id, mode, status, chroma_reindexed_count").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{
			"source_session_id", "target_session_id", "mode", "status", "chroma_reindexed_count",
		}).AddRow("source", "target", SessionMigrationModeCopyThenLockSource, "copied", 0))
	mock.ExpectRollback()
	_, err = m.LockSessionMigrationSource(context.Background(), 42, "too early")
	if err == nil || !strings.Contains(err.Error(), `migration status "copied" is not vector_reindexed`) {
		t.Fatalf("expected vector phase block, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBStoreLockSessionMigrationSourceBlocksCopyKeepSourceMode(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT source_session_id, target_session_id, mode, status, chroma_reindexed_count").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{
			"source_session_id", "target_session_id", "mode", "status", "chroma_reindexed_count",
		}).AddRow("source", "target", SessionMigrationModeCopyKeepSource, "vector_reindexed", 2))
	mock.ExpectRollback()
	_, err = m.LockSessionMigrationSource(context.Background(), 42, "should not lock")
	if err == nil || !strings.Contains(err.Error(), "does not lock source") {
		t.Fatalf("expected copy-keep mode block, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBStoreListMemoriesQueriesRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	created := time.Date(2026, 5, 24, 10, 2, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("FROM memories")).
		WithArgs("sess-1", 1, 1, 5, 5).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "chat_session_id", "turn_index", "summary_json", "embedding", "embedding_model",
			"importance", "emotional_boost", "evidence", "emotional_intensity",
			"narrative_significance", "place_wing", "place_room", "created_at",
		}).AddRow(11, "sess-1", 3, `{"summary":"ok"}`, `[0.1]`, "model-a", 0.7, 0.2, `{"quote":"x"}`, 0.3, 0.4, "trust", "world", created))

	items, err := m.ListMemories(context.Background(), "sess-1", 1, 5)
	if err != nil {
		t.Fatalf("ListMemories failed: %v", err)
	}
	if len(items) != 1 || items[0].ID != 11 || items[0].EmbeddingModel != "model-a" || items[0].PlaceRoom != "world" {
		t.Fatalf("unexpected memories: %+v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBStoreListEvidenceQueriesRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	created := time.Date(2026, 5, 24, 10, 3, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("FROM direct_evidence_records")).
		WithArgs("sess-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "chat_session_id", "evidence_kind", "evidence_text", "source_turn_start",
			"source_turn_end", "turn_anchor", "source_message_ids_json", "source_hash",
			"archive_state", "capture_stage", "capture_verification", "committed_gate",
			"lineage_json", "repair_needed", "tombstoned", "superseded_by_id", "created_at",
		}).AddRow(21, "sess-1", "fact_event", "evidence text", 2, 4, 3, `["m1"]`, "hash-a",
			"committed", "critic_extract", "verified", "gate-a", `{"lineage":1}`, false, false, nil, created))

	items, err := m.ListEvidence(context.Background(), "sess-1")
	if err != nil {
		t.Fatalf("ListEvidence failed: %v", err)
	}
	if len(items) != 1 || items[0].ID != 21 || items[0].TurnAnchor != 3 || items[0].CommittedGate != "gate-a" {
		t.Fatalf("unexpected evidence: %+v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBStoreListKGTriplesQueriesRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	created := time.Date(2026, 5, 24, 10, 4, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("FROM kg_triples")).
		WithArgs("sess-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "chat_session_id", "subject", "predicate", "object", "valid_from", "valid_to", "source_turn", "created_at",
		}).AddRow(31, "sess-1", "Chloe", "trusts", "user", 1, nil, 2, created))

	items, err := m.ListKGTriples(context.Background(), "sess-1")
	if err != nil {
		t.Fatalf("ListKGTriples failed: %v", err)
	}
	if len(items) != 1 || items[0].Subject != "Chloe" || items[0].ValidFrom != 1 || items[0].ValidTo != 0 {
		t.Fatalf("unexpected triples: %+v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBStoreListAuditLogsQueriesRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	created := time.Date(2026, 5, 24, 10, 5, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("FROM audit_logs")).
		WithArgs("sess-1", "sess-1", "memory_write", "memory_write", 25).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "created_at", "event_type", "chat_session_id", "target_type", "target_id", "summary", "details_json", "source",
		}).AddRow(41, created, "memory_write", "sess-1", "memory", 11, "summary", `{"ok":true}`, "api"))

	items, err := m.ListAuditLogs(context.Background(), "sess-1", "memory_write", 25)
	if err != nil {
		t.Fatalf("ListAuditLogs failed: %v", err)
	}
	if len(items) != 1 || items[0].EventType != "memory_write" || items[0].TargetID != 11 {
		t.Fatalf("unexpected audit logs: %+v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBStoreListAuditLogsZeroLimitReadsAllRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	mock.ExpectQuery(`(?s)FROM audit_logs.*ORDER BY created_at DESC, id DESC\s*$`).
		WithArgs("sess-all", "sess-all", "", "").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "created_at", "event_type", "chat_session_id", "target_type",
			"target_id", "summary", "details_json", "source",
		}))
	if _, err := m.ListAuditLogs(context.Background(), "sess-all", "", 0); err != nil {
		t.Fatalf("ListAuditLogs zero limit: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBStoreSaveSupersessionResolutionWritesAuditAndEvidenceState(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO audit_logs")).
		WithArgs(sqlmock.AnyArg(), "supersession_resolution", "sess-1", "direct_evidence", int64(21), sqlmock.AnyArg(), sqlmock.AnyArg(), "manual_ui").
		WillReturnResult(sqlmock.NewResult(61, 1))
	mock.ExpectExec("UPDATE direct_evidence_records").
		WithArgs("superseded_archive", "superseded", "supersede_by_resolution", int64(22), int64(21), "sess-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	record, err := m.SaveSupersessionResolution(context.Background(), &SupersessionResolutionDecision{
		ChatSessionID:   "sess-1",
		TargetType:      "direct_evidence",
		TargetID:        21,
		SourceTurn:      9,
		ResolutionClass: "supersede",
		NewTargetType:   "direct_evidence",
		NewTargetID:     22,
		RelationshipKey: "alice_trust_bob",
		Reason:          "newer verified evidence wins",
		Operator:        "manual_ui",
	})
	if err != nil {
		t.Fatalf("SaveSupersessionResolution failed: %v", err)
	}
	if record.ID != 61 || record.ResolutionClass != "supersede" || !strings.Contains(record.DetailsJSON, SupersessionResolutionContractVersion) {
		t.Fatalf("unexpected record: %+v", record)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBStoreListCriticFeedbackQueriesRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	created := time.Date(2026, 5, 24, 10, 6, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("FROM critic_feedback")).
		WithArgs("sess-1", "memory", "memory", int64(11), int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "created_at", "chat_session_id", "target_type", "target_id", "feedback_value", "feedback_note", "source",
		}).AddRow(51, created, "sess-1", "memory", 11, "accept", "good", "manual_ui"))

	items, err := m.ListCriticFeedback(context.Background(), "sess-1", "memory", 11)
	if err != nil {
		t.Fatalf("ListCriticFeedback failed: %v", err)
	}
	if len(items) != 1 || items[0].FeedbackValue != "accept" || items[0].FeedbackNote != "good" {
		t.Fatalf("unexpected critic feedback: %+v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBStoreListCharacterEventsQueriesRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	created := time.Date(2026, 5, 24, 10, 7, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("FROM character_events")).
		WithArgs("sess-1", "Chloe", "Chloe").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "chat_session_id", "character_name", "turn_index", "event_type", "details_json", "created_at",
		}).AddRow(61, "sess-1", "Chloe", 8, "mood_shift", `{"mood":"calm"}`, created))

	items, err := m.ListCharacterEvents(context.Background(), "sess-1", "Chloe")
	if err != nil {
		t.Fatalf("ListCharacterEvents failed: %v", err)
	}
	if len(items) != 1 || items[0].CharacterName != "Chloe" || items[0].TurnIndex != 8 {
		t.Fatalf("unexpected character events: %+v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBSaveWorldRuleCreatesNewTurnVersion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	now := time.Date(2026, 8, 8, 15, 0, 0, 0, time.UTC)
	rule := &WorldRule{
		ChatSessionID: "sess-world",
		Scope:         "root",
		Category:      "society",
		Key:           "strict_status_society",
		ValueJSON:     `{"statement":"status matters"}`,
		Genre:         "historical",
		SourceTurn:    2,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	mock.ExpectQuery("SELECT id FROM world_rules .*source_turn = \\?").
		WithArgs(rule.ChatSessionID, rule.Scope, rule.Key, nil, rule.ValueJSON, rule.SourceTurn, rule.SourceTurn).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectExec("INSERT INTO world_rules").
		WithArgs(rule.ChatSessionID, rule.Scope, nil, rule.Category, rule.Key, rule.ValueJSON,
			rule.Genre, rule.SourceTurn, false, false, false, now, now).
		WillReturnResult(sqlmock.NewResult(44, 1))

	if err := m.SaveWorldRule(context.Background(), rule); err != nil {
		t.Fatalf("SaveWorldRule: %v", err)
	}
	if rule.ID != 44 {
		t.Fatalf("rule ID = %d, want 44", rule.ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBSaveWorldRuleUpdatesSameTurnWithoutDuplicate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	now := time.Date(2026, 8, 8, 15, 5, 0, 0, time.UTC)
	rule := &WorldRule{
		ChatSessionID: "sess-world",
		Scope:         "root",
		Category:      "society",
		Key:           "strict_status_society",
		ValueJSON:     `{"statement":"status matters"}`,
		SourceTurn:    2,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	mock.ExpectQuery("SELECT id FROM world_rules .*source_turn = \\?").
		WithArgs(rule.ChatSessionID, rule.Scope, rule.Key, nil, rule.ValueJSON, rule.SourceTurn, rule.SourceTurn).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(33))
	mock.ExpectExec("UPDATE world_rules .*WHERE id = \\?").
		WithArgs(nil, rule.Category, rule.ValueJSON, nil, rule.SourceTurn, rule.SourceTurn,
			false, false, false, now, int64(33)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := m.SaveWorldRule(context.Background(), rule); err != nil {
		t.Fatalf("SaveWorldRule: %v", err)
	}
	if rule.ID != 33 {
		t.Fatalf("rule ID = %d, want 33", rule.ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBListWorldRulesReadsOnlyLatestTurnVersion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	now := time.Date(2026, 8, 8, 15, 10, 0, 0, time.UTC)
	mock.ExpectQuery("FROM world_rules AS current_rule .*ORDER BY COALESCE\\(candidate.source_turn, 0\\) DESC, candidate.id DESC").
		WithArgs("sess-world").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "chat_session_id", "scope", "scope_name", "category", "key", "value_json", "genre", "source_turn",
			"pinned", "suppressed", "user_corrected", "created_at", "updated_at",
		}).AddRow(44, "sess-world", "root", nil, "society", "strict_status_society",
			`{"statement":"status matters"}`, nil, 2, false, false, false, now, now))

	items, err := m.ListWorldRules(context.Background(), "sess-world")
	if err != nil {
		t.Fatalf("ListWorldRules: %v", err)
	}
	if len(items) != 1 || items[0].ID != 44 || items[0].SourceTurn != 2 {
		t.Fatalf("unexpected world rules: %+v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBStoreReadSessionStateSnapshotUsesSingleReadOnlyTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	now := time.Date(2026, 5, 31, 9, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery("FROM active_states").
		WithArgs("sess-agg", "", "").
		WillReturnRows(sqlmock.NewRows([]string{"id", "chat_session_id", "state_type", "content", "turn_index", "created_at"}).
			AddRow(1, "sess-agg", "mood", "tense", 12, now))
	mock.ExpectQuery("FROM canonical_state_layers").
		WithArgs("sess-agg", "", "").
		WillReturnRows(sqlmock.NewRows([]string{"id", "chat_session_id", "layer_type", "content", "source_state_type", "turn_index", "source_turn", "source_record", "last_verified_turn", "confidence", "created_at"}))
	mock.ExpectQuery("FROM storylines").
		WithArgs("sess-agg").
		WillReturnRows(sqlmock.NewRows([]string{"id", "chat_session_id", "name", "status", "entities_json", "current_context", "key_points_json", "ongoing_tensions_json", "confidence", "evidence_count", "last_evidence_turn", "first_turn", "last_turn", "pinned", "suppressed", "user_corrected", "created_at", "updated_at"}).
			AddRow(2, "sess-agg", "Rooftop", "active", "[]", "confession", `["hesitation"]`, `["answer"]`, 0.8, 3, 11, 1, 12, false, false, false, now, now))
	mock.ExpectQuery("FROM character_states").
		WithArgs("sess-agg").
		WillReturnRows(sqlmock.NewRows([]string{"id", "chat_session_id", "character_name", "appearance_json", "personality_json", "status_json", "relationships_json", "speech_style_json", "turn_index", "created_at", "updated_at"}))
	mock.ExpectQuery("FROM world_rules").
		WithArgs("sess-agg").
		WillReturnRows(sqlmock.NewRows([]string{"id", "chat_session_id", "scope", "scope_name", "category", "key", "value_json", "genre", "source_turn", "pinned", "suppressed", "user_corrected", "created_at", "updated_at"}))
	mock.ExpectQuery("FROM pending_threads").
		WithArgs("sess-agg").
		WillReturnRows(sqlmock.NewRows([]string{"id", "chat_session_id", "thread_key", "description", "status", "created_turn", "resolved_turn", "source_turn", "priority", "hook_type", "hook_metadata_json", "pinned", "suppressed", "user_corrected", "created_at", "updated_at"}).
			AddRow(3, "sess-agg", "thread_answer", "Need an answer", "open", 10, 0, 10, 2, "open_question", `{"title":"Need an answer"}`, false, false, false, now, now))
	mock.ExpectQuery("FROM character_events").
		WithArgs("sess-agg", "", "").
		WillReturnRows(sqlmock.NewRows([]string{"id", "chat_session_id", "character_name", "turn_index", "event_type", "details_json", "created_at"}))
	mock.ExpectQuery("FROM chat_logs").
		WithArgs("sess-agg", 0, 0, 0, 0).
		WillReturnRows(sqlmock.NewRows([]string{"id", "chat_session_id", "turn_index", "role", "content", "created_at"}).
			AddRow(4, "sess-agg", 12, "assistant", "She hesitated.", now))
	mock.ExpectCommit()

	snapshot, err := m.ReadSessionStateSnapshot(context.Background(), "sess-agg")
	if err != nil {
		t.Fatalf("ReadSessionStateSnapshot: %v", err)
	}
	if !snapshot.SingleConnection {
		t.Fatal("snapshot should prove single connection aggregate path")
	}
	if len(snapshot.ActiveStates) != 1 || len(snapshot.Storylines) != 1 || len(snapshot.PendingThreads) != 1 || len(snapshot.RecentChatLogs) != 1 {
		t.Fatalf("unexpected snapshot counts: active=%d story=%d threads=%d logs=%d",
			len(snapshot.ActiveStates), len(snapshot.Storylines), len(snapshot.PendingThreads), len(snapshot.RecentChatLogs))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBPatchPendingThreadUsesLiveColumnsAndMetadata(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	ctx := context.Background()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT hook_metadata_json FROM pending_threads WHERE id = ?")).
		WithArgs(int64(11)).
		WillReturnRows(sqlmock.NewRows([]string{"hook_metadata_json"}).AddRow(`{"owner":"Nia"}`))
	mock.ExpectExec(regexp.QuoteMeta("UPDATE pending_threads SET status = ?, hook_type = ?, description = ?, hook_metadata_json = ?, updated_at = ? WHERE id = ?")).
		WithArgs("paused", "open_question", "Ask Mira why she hesitated", sqlmock.AnyArg(), sqlmock.AnyArg(), int64(11)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	fields, err := m.PatchPendingThread(ctx, 11, map[string]any{
		"status":      "paused",
		"thread_type": "open_question",
		"title":       "Ask Mira why she hesitated",
	})
	if err != nil {
		t.Fatalf("PatchPendingThread: %v", err)
	}
	if got := strings.Join(fields, ","); got != "status,thread_type,title" {
		t.Fatalf("updated fields = %q", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBSavePendingThreadUpdatesExistingOpenHook(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	ctx := context.Background()

	mock.ExpectExec("UPDATE pending_threads").
		WithArgs("Ask Mira why she hesitated", "open", 0, 9, 0, "open_question", `{"title":"Ask Mira why she hesitated"}`, sqlmock.AnyArg(), "sess-1", "thread_ask_mira").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = m.SavePendingThread(ctx, &PendingThread{
		ChatSessionID:    "sess-1",
		ThreadKey:        "thread_ask_mira",
		Description:      "Ask Mira why she hesitated",
		Status:           "open",
		SourceTurn:       9,
		HookType:         "open_question",
		HookMetadataJSON: `{"title":"Ask Mira why she hesitated"}`,
	})
	if err != nil {
		t.Fatalf("SavePendingThread: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBSavePendingThreadResolvedReplayIsIdempotent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	ctx := context.Background()

	mock.ExpectExec("UPDATE pending_threads").
		WithArgs("Repay Park Dojun", "resolved", 12, 12, 0, "obligation", `{"lifecycle_key":"loan-settlement"}`, sqlmock.AnyArg(), "sess-1", "thread_lifecycle_abc").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT id").
		WithArgs("sess-1", "thread_lifecycle_abc").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)))

	err = m.SavePendingThread(ctx, &PendingThread{
		ChatSessionID: "sess-1", ThreadKey: "thread_lifecycle_abc", Description: "Repay Park Dojun",
		Status: "resolved", ResolvedTurn: 12, SourceTurn: 12, HookType: "obligation",
		HookMetadataJSON: `{"lifecycle_key":"loan-settlement"}`,
	})
	if err != nil {
		t.Fatalf("SavePendingThread resolved replay: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("resolved replay inserted a duplicate row: %v", err)
	}
}

func TestMariaDBSavePendingThreadDoesNotOverwriteManualTrustFlags(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	ctx := context.Background()

	query := `
		UPDATE pending_threads
		SET description = COALESCE(?, description),
			status = COALESCE(?, status),
			resolved_turn = NULLIF(?, 0),
			source_turn = NULLIF(?, 0),
			priority = NULLIF(?, 0),
			hook_type = COALESCE(?, hook_type),
			hook_metadata_json = COALESCE(?, hook_metadata_json),
			updated_at = ?
		WHERE chat_session_id = ? AND thread_key = ? AND status <> 'resolved'
		ORDER BY id DESC
		LIMIT 1
	`
	mock.ExpectExec(query).
		WithArgs("Daily cube rendezvous", "open", 0, 10, 0, "promise", `{"title":"Daily cube rendezvous"}`, sqlmock.AnyArg(), "sess-1", "thread_daily_cube").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = m.SavePendingThread(ctx, &PendingThread{
		ChatSessionID:    "sess-1",
		ThreadKey:        "thread_daily_cube",
		Description:      "Daily cube rendezvous",
		Status:           "open",
		SourceTurn:       10,
		HookType:         "promise",
		HookMetadataJSON: `{"title":"Daily cube rendezvous"}`,
	})
	if err != nil {
		t.Fatalf("SavePendingThread: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("automatic pending-thread save changed the trust-owned update shape: %v", err)
	}
}

func TestMariaDBDeletePendingThreadReportsMissingRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	ctx := context.Background()

	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM pending_threads WHERE id = ?")).
		WithArgs(int64(99)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	if err := m.DeletePendingThread(ctx, 99); !errors.Is(err, ErrNotFound) {
		t.Fatalf("DeletePendingThread error = %v, want ErrNotFound", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBStoreSaveEpisodeSummaryPersistsDS1aFields(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	created := time.Date(2026, 6, 4, 10, 0, 0, 0, time.UTC)
	item := &EpisodeSummary{
		ChatSessionID:           "sess-ds1a",
		FromTurn:                1,
		ToTurn:                  2,
		SummaryText:             "Alice opens the sealed gate.",
		KeyEntities:             `["Alice","Bob"]`,
		KeyEvents:               `["Alice opens the sealed gate"]`,
		OpenLoopsJSON:           `["sealed gate remains unresolved"]`,
		RelationshipChangesJSON: `["Alice trusts Bob"]`,
		EmbeddingVector:         "[]",
		EmbeddingModel:          "none",
		CreatedAt:               created,
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO episode_summaries")).
		WithArgs("sess-ds1a", 1, 2, "Alice opens the sealed gate.", `["Alice","Bob"]`, `["Alice opens the sealed gate"]`, `["sealed gate remains unresolved"]`, `["Alice trusts Bob"]`, "[]", "none", created).
		WillReturnResult(sqlmock.NewResult(77, 1))
	if err := m.SaveEpisodeSummary(context.Background(), item); err != nil {
		t.Fatalf("SaveEpisodeSummary: %v", err)
	}
	if item.ID != 77 {
		t.Fatalf("item.ID = %d, want 77", item.ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBStoreGetResumePackReturnsEmptyPackWhenNoChapter(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, chat_session_id, from_turn, to_turn, chapter_index, chapter_title, summary_text,")).
		WithArgs("sess-empty").
		WillReturnRows(sqlmock.NewRows([]string{}))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, chat_session_id, from_turn, to_turn, arc_index, arc_name, arc_status, core_conflict,")).
		WithArgs("sess-empty").
		WillReturnRows(sqlmock.NewRows([]string{}))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, chat_session_id, from_turn, to_turn, era_label, saga_summary,")).
		WithArgs("sess-empty").
		WillReturnRows(sqlmock.NewRows([]string{}))
	pack, err := m.GetResumePack(context.Background(), "sess-empty", "resume")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pack == nil {
		t.Fatal("expected non-nil skeleton pack")
	}
	if pack.PackStatus != "empty" {
		t.Errorf("pack_status = %q, want empty", pack.PackStatus)
	}
	if pack.Chapter != nil {
		t.Errorf("chapter should be nil, got %+v", pack.Chapter)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBStoreGetResumePackReturnsChapter(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	created := time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, chat_session_id, from_turn, to_turn, chapter_index, chapter_title, summary_text,")).
		WithArgs("sess-ch1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "chat_session_id", "from_turn", "to_turn", "chapter_index", "chapter_title", "summary_text",
			"open_loops_json", "relationship_changes_json", "world_changes_json", "callback_candidates_json",
			"resume_text", "embedding_vector", "embedding_model", "created_at",
		}).AddRow(1, "sess-ch1", 1, 10, 1, "The Tower", "Alice climbs the tower.",
			`["locked door"]`, `{"Alice":"resolved"}`, `{"tower":"opened"}`, `["bell"]`,
			"Resume: Alice is determined.", `[0.1,0.2]`, "test-embed", created))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, chat_session_id, from_turn, to_turn, arc_index, arc_name, arc_status, core_conflict,")).
		WithArgs("sess-ch1").
		WillReturnRows(sqlmock.NewRows([]string{}))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, chat_session_id, from_turn, to_turn, era_label, saga_summary,")).
		WithArgs("sess-ch1").
		WillReturnRows(sqlmock.NewRows([]string{}))
	pack, err := m.GetResumePack(context.Background(), "sess-ch1", "resume")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pack == nil {
		t.Fatal("expected non-nil pack")
	}
	if pack.PackStatus != "ready" {
		t.Errorf("pack_status = %q, want ready", pack.PackStatus)
	}
	if pack.Chapter == nil {
		t.Fatal("expected chapter")
	}
	if pack.Chapter.ChapterTitle != "The Tower" {
		t.Errorf("chapter_title = %q, want The Tower", pack.Chapter.ChapterTitle)
	}
	if pack.Chapter.ChatSessionID != "sess-ch1" {
		t.Errorf("chat_session_id = %q, want sess-ch1", pack.Chapter.ChatSessionID)
	}
	if pack.Chapter.FromTurn != 1 || pack.Chapter.ToTurn != 10 {
		t.Errorf("turn range = %d-%d, want 1-10", pack.Chapter.FromTurn, pack.Chapter.ToTurn)
	}
	if pack.Chapter.OpenLoopsJSON != `["locked door"]` {
		t.Errorf("open_loops_json = %q", pack.Chapter.OpenLoopsJSON)
	}
	if pack.Chapter.CallbackCandidatesJSON != `["bell"]` {
		t.Errorf("callback_candidates_json = %q", pack.Chapter.CallbackCandidatesJSON)
	}
	if pack.Chapter.EmbeddingModel != "test-embed" {
		t.Errorf("embedding_model = %q, want test-embed", pack.Chapter.EmbeddingModel)
	}
	if !strings.Contains(pack.AssembledText, "The Tower") {
		t.Errorf("assembled_text missing title: %q", pack.AssembledText)
	}
	if !strings.Contains(pack.AssembledText, "Alice is determined") {
		t.Errorf("assembled_text missing resume: %q", pack.AssembledText)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBStoreSaveChapterSummaryExecutesInsert(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO chapter_summaries")).
		WithArgs("sess-save", 1, 60, 1, "Chapter 1", "Summary", "[]", nil, nil, "[]", "Resume", "[]", "none").
		WillReturnResult(sqlmock.NewResult(42, 1))
	item := &ChapterSummary{
		ChatSessionID:          "sess-save",
		FromTurn:               1,
		ToTurn:                 60,
		ChapterIndex:           1,
		ChapterTitle:           "Chapter 1",
		SummaryText:            "Summary",
		OpenLoopsJSON:          "[]",
		CallbackCandidatesJSON: "[]",
		ResumeText:             "Resume",
		EmbeddingVector:        "[]",
		EmbeddingModel:         "none",
	}
	if err := m.SaveChapterSummary(context.Background(), item); err != nil {
		t.Fatalf("SaveChapterSummary failed: %v", err)
	}
	if item.ID != 42 {
		t.Fatalf("ID = %d, want 42", item.ID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBStoreSearchChapterSummariesReturnsRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	created := time.Date(2026, 6, 2, 13, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, chat_session_id, from_turn, to_turn, chapter_index, chapter_title, summary_text,")).
		WithArgs("sess-search", "gate", "%gate%", "%gate%", "%gate%", "%gate%", "%gate%", "%gate%", "%gate%", 0, 0, 0, 0, 5).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "chat_session_id", "from_turn", "to_turn", "chapter_index", "chapter_title", "summary_text",
			"open_loops_json", "relationship_changes_json", "world_changes_json", "callback_candidates_json",
			"resume_text", "embedding_vector", "embedding_model", "created_at",
		}).AddRow(7, "sess-search", 1, 60, 1, "Archive Gate", "Gate summary", nil, nil, nil, `["gate"]`, "Gate resume", nil, "none", created))
	items, err := m.SearchChapterSummaries(context.Background(), "sess-search", "gate", 0, 0, 5)
	if err != nil {
		t.Fatalf("SearchChapterSummaries failed: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d, want 1", len(items))
	}
	if items[0].ChapterTitle != "Archive Gate" || items[0].CallbackCandidatesJSON != `["gate"]` {
		t.Fatalf("unexpected item: %+v", items[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBHierarchyZeroLimitDoesNotAddHiddenLimit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}

	mock.ExpectQuery(`(?s)FROM chapter_summaries.*ORDER BY chapter_index DESC, id DESC\s*$`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	if _, err := m.SearchChapterSummaries(context.Background(), "session", "", 0, 0, 0); err != nil {
		t.Fatalf("chapter search: %v", err)
	}

	mock.ExpectQuery(`(?s)FROM arc_summaries.*ORDER BY arc_index DESC, id DESC\s*$`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	if _, err := m.SearchArcSummaries(context.Background(), "session", "", 0, 0, 0); err != nil {
		t.Fatalf("arc search: %v", err)
	}

	mock.ExpectQuery(`(?s)FROM saga_digests.*ORDER BY to_turn DESC, id DESC\s*$`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	if _, err := m.SearchSagaDigests(context.Background(), "session", "", 0, 0, 0); err != nil {
		t.Fatalf("saga search: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
