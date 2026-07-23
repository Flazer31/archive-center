package store

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
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
	for i := 0; i < 31; i++ {
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
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
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
	for i := 0; i < 31; i++ {
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
