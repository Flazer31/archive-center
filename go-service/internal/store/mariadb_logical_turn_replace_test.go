package store

import (
	"context"
	"errors"
	"regexp"
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
	for i := 0; i < 33; i++ {
		mock.ExpectExec(`(?s).+`).WillReturnResult(sqlmock.NewResult(0, 1))
	}
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
	for i := 0; i < 33; i++ {
		mock.ExpectExec(`(?s).+`).WillReturnResult(sqlmock.NewResult(0, 1))
	}
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
