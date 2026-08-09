package store

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
)

func TestMariaDBReplaceLogicalTurnAtomicallyReplacesCanonicalTail(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	created := time.Date(2026, 7, 22, 1, 2, 3, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT turn_index FROM chat_logs WHERE chat_session_id = ? ORDER BY turn_index DESC, id DESC LIMIT 1 FOR UPDATE")).
		WithArgs("session-1").
		WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(3))
	mock.ExpectExec("DELETE FROM effective_input_logs").WithArgs("session-1", 3).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM precise_memory_units").WithArgs("session-1", 3).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM memories").WithArgs("session-1", 3).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM direct_evidence_records").WithArgs("session-1", 3).WillReturnResult(sqlmock.NewResult(0, 1))
	for i := 0; i < 35; i++ {
		mock.ExpectExec(`(?s).+`).WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectExec("INSERT INTO status_current_values").
		WithArgs("session-1").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO chat_logs")).
		WithArgs("session-1", 3, "user text", created).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO chat_logs")).
		WithArgs("session-1", 3, "final assistant", created).
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()
	if err := m.ReplaceLogicalTurn(context.Background(), LogicalTurnReplacement{
		ChatSessionID: "session-1", TurnIndex: 3, UserContent: "user text",
		AssistantContent: "final assistant", CreatedAt: created,
	}); err != nil {
		t.Fatalf("ReplaceLogicalTurn: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBReplaceLogicalTurnRefusesHistoricalTurn(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT turn_index FROM chat_logs WHERE chat_session_id = ? ORDER BY turn_index DESC, id DESC LIMIT 1 FOR UPDATE")).
		WithArgs("session-1").
		WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(4))
	mock.ExpectRollback()
	err = m.ReplaceLogicalTurn(context.Background(), LogicalTurnReplacement{
		ChatSessionID: "session-1", TurnIndex: 3, UserContent: "user", AssistantContent: "assistant",
	})
	if err == nil {
		t.Fatal("historical logical turn replacement unexpectedly succeeded")
	}
	var typed *LogicalTurnReplacementError
	if !errors.As(err, &typed) || typed.Code != "logical_turn_not_current_tail" || typed.Retryable || typed.CommitState != "not_committed" {
		t.Fatalf("historical replacement error is not terminal and typed: %+v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLogicalTurnReplacementStoreErrorClassification(t *testing.T) {
	tests := []struct {
		name            string
		err             error
		commitAttempted bool
		code            string
		retryable       bool
		commitState     string
	}{
		{
			name: "permission",
			err:  &mysql.MySQLError{Number: 1142, Message: "command denied"},
			code: "logical_turn_db_permission_denied", commitState: "not_committed",
		},
		{
			name: "deadlock",
			err:  &mysql.MySQLError{Number: 1213, Message: "deadlock"},
			code: "logical_turn_transaction_temporarily_blocked", retryable: true, commitState: "not_committed",
		},
		{
			name: "commit unknown",
			err:  errors.New("connection lost during commit"), commitAttempted: true,
			code: "logical_turn_commit_outcome_unknown", commitState: "unknown",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := classifyLogicalTurnReplacementStoreError(tc.err, "test_stage", tc.commitAttempted)
			var typed *LogicalTurnReplacementError
			if !errors.As(err, &typed) {
				t.Fatalf("error is not typed: %v", err)
			}
			if typed.Code != tc.code || typed.Retryable != tc.retryable || typed.CommitState != tc.commitState {
				t.Fatalf("unexpected classification: %+v", typed)
			}
		})
	}
}

func TestMariaDBReplaceLogicalTurnRecreatesDeletedImmediateTail(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	created := time.Date(2026, 7, 22, 2, 25, 44, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT turn_index FROM chat_logs WHERE chat_session_id = ? ORDER BY turn_index DESC, id DESC LIMIT 1 FOR UPDATE")).
		WithArgs("session-1").
		WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(14))
	mock.ExpectExec("DELETE FROM effective_input_logs").WithArgs("session-1", 15).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM precise_memory_units").WithArgs("session-1", 15).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM memories").WithArgs("session-1", 15).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM direct_evidence_records").WithArgs("session-1", 15).WillReturnResult(sqlmock.NewResult(0, 1))
	for i := 0; i < 35; i++ {
		mock.ExpectExec(`(?s).+`).WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectExec("INSERT INTO status_current_values").
		WithArgs("session-1").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO chat_logs")).
		WithArgs("session-1", 15, "user text", created).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO chat_logs")).
		WithArgs("session-1", 15, "regenerated assistant", created).
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()

	if err := m.ReplaceLogicalTurn(context.Background(), LogicalTurnReplacement{
		ChatSessionID: "session-1", TurnIndex: 15, UserContent: "user text",
		AssistantContent: "regenerated assistant", CreatedAt: created,
	}); err != nil {
		t.Fatalf("ReplaceLogicalTurn deleted immediate tail: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBReplaceLogicalTurnRecreatesDeletedFirstTurnInEmptySession(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	created := time.Date(2026, 7, 31, 3, 8, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT turn_index FROM chat_logs WHERE chat_session_id = ? ORDER BY turn_index DESC, id DESC LIMIT 1 FOR UPDATE")).
		WithArgs("session-empty").
		WillReturnRows(sqlmock.NewRows([]string{"turn_index"}))
	mock.ExpectExec("DELETE FROM effective_input_logs").WithArgs("session-empty", 1).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM precise_memory_units").WithArgs("session-empty", 1).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM memories").WithArgs("session-empty", 1).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM direct_evidence_records").WithArgs("session-empty", 1).WillReturnResult(sqlmock.NewResult(0, 0))
	for i := 0; i < 35; i++ {
		mock.ExpectExec(`(?s).+`).WillReturnResult(sqlmock.NewResult(0, 0))
	}
	mock.ExpectExec("INSERT INTO status_current_values").
		WithArgs("session-empty").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO chat_logs")).
		WithArgs("session-empty", 1, "first user", created).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO chat_logs")).
		WithArgs("session-empty", 1, "regenerated first assistant", created).
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()

	if err := m.ReplaceLogicalTurn(context.Background(), LogicalTurnReplacement{
		ChatSessionID: "session-empty", TurnIndex: 1, UserContent: "first user",
		AssistantContent: "regenerated first assistant", CreatedAt: created,
	}); err != nil {
		t.Fatalf("ReplaceLogicalTurn empty first turn: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBRollbackCanonicalTailIsAtomicAndIdempotent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	rollback := LogicalTurnRollback{ChatSessionID: "session-1", TurnIndex: 4, Reason: "turn_rollback", CreatedAt: now}

	for range 2 {
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta("SELECT turn_index FROM chat_logs WHERE chat_session_id = ? ORDER BY turn_index DESC, id DESC LIMIT 1 FOR UPDATE")).
			WithArgs("session-1").
			WillReturnRows(sqlmock.NewRows([]string{"turn_index"}).AddRow(3))
		mock.ExpectQuery("SELECT source_revision").
			WithArgs("session-1", 4).
			WillReturnRows(sqlmock.NewRows([]string{"source_revision"}))
		for range canonicalTailDeleteCommands("session-1", 4, false, true) {
			mock.ExpectExec(`(?s).+`).WillReturnResult(sqlmock.NewResult(0, 0))
		}
		mock.ExpectExec("(?s)INSERT INTO status_current_values.*JOIN memory_source_revisions source_revision.*source_revision.lifecycle_state = 'active'.*NOT EXISTS").
			WithArgs("session-1").
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectCommit()
	}

	if err := m.RollbackCanonicalTail(context.Background(), rollback); err != nil {
		t.Fatalf("first rollback: %v", err)
	}
	if err := m.RollbackCanonicalTail(context.Background(), rollback); err != nil {
		t.Fatalf("idempotent replay: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBRollbackCanonicalTailRollsBackOnDerivedDeleteFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	failure := errors.New("derived delete failed")
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT turn_index FROM chat_logs WHERE chat_session_id = ? ORDER BY turn_index DESC, id DESC LIMIT 1 FOR UPDATE")).
		WithArgs("session-1").
		WillReturnRows(sqlmock.NewRows([]string{"turn_index"}).AddRow(4))
	mock.ExpectQuery("SELECT source_revision").
		WithArgs("session-1", 4).
		WillReturnRows(sqlmock.NewRows([]string{"source_revision"}))
	mock.ExpectExec("DELETE FROM effective_input_logs").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM memories").WillReturnError(failure)
	mock.ExpectRollback()

	err = m.RollbackCanonicalTail(context.Background(), LogicalTurnRollback{
		ChatSessionID: "session-1", TurnIndex: 4, Reason: "turn_rollback",
	})
	if err == nil || !strings.Contains(err.Error(), failure.Error()) {
		t.Fatalf("rollback error = %v, want derived failure", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCanonicalTailLifecycleCleanupDeletesDirectEvidenceButPreservesLifecycleHistory(t *testing.T) {
	queries := []string{}
	for _, command := range canonicalTailDeleteCommands("session-1", 4, false, true) {
		queries = append(queries, strings.Join(strings.Fields(command.query), " "))
	}
	joined := strings.Join(queries, "\n")
	for _, required := range []string{
		"DELETE FROM direct_evidence_records",
		"DELETE FROM status_current_values",
		"DELETE FROM status_change_events",
		"JSON_EXTRACT(evidence_json, '$.source_revision')",
	} {
		if !strings.Contains(joined, required) {
			t.Fatalf("lifecycle cleanup missing %q:\n%s", required, joined)
		}
	}
	for _, forbidden := range []string{
		"DELETE FROM precise_memory_units",
		"UPDATE direct_evidence_records",
	} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("lifecycle cleanup destroys invalidated history via %q:\n%s", forbidden, joined)
		}
	}

	legacyQueries := []string{}
	for _, command := range canonicalTailDeleteCommands("session-1", 4, true, true) {
		legacyQueries = append(legacyQueries, strings.Join(strings.Fields(command.query), " "))
	}
	legacy := strings.Join(legacyQueries, "\n")
	for _, required := range []string{
		"DELETE FROM precise_memory_units",
		"DELETE FROM direct_evidence_records",
		"DELETE FROM status_change_events",
	} {
		if !strings.Contains(legacy, required) {
			t.Fatalf("legacy cleanup missing compatibility delete %q", required)
		}
	}
}

func TestCanonicalTailCleanupDeletesEveryOverlappingHierarchySummary(t *testing.T) {
	queries := map[string]canonicalTailDeleteCommand{}
	for _, command := range canonicalTailDeleteCommands("session-1", 4, false, true) {
		normalized := strings.Join(strings.Fields(command.query), " ")
		for _, table := range []string{"episode_summaries", "chapter_summaries", "arc_summaries", "saga_digests"} {
			if strings.Contains(normalized, "DELETE FROM "+table) {
				queries[table] = command
			}
		}
	}
	for _, table := range []string{"episode_summaries", "chapter_summaries", "arc_summaries", "saga_digests"} {
		command, ok := queries[table]
		if !ok {
			t.Fatalf("hierarchy cleanup missing %s", table)
		}
		normalized := strings.Join(strings.Fields(command.query), " ")
		if !strings.Contains(normalized, "(to_turn >= ? OR from_turn >= ?)") {
			t.Fatalf("%s cleanup does not delete ranges overlapping the rollback turn: %s", table, normalized)
		}
		if len(command.args) != 3 || command.args[0] != "session-1" || command.args[1] != 4 || command.args[2] != 4 {
			t.Fatalf("%s cleanup args = %#v, want session and rollback turn twice", table, command.args)
		}
	}
}
