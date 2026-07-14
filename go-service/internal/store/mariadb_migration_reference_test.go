package store

import (
	"context"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCopySessionMigrationReferenceBindingsCopiesLinkRuntimeAndLedger(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT binding_id\n\t\tFROM session_reference_bindings")).
		WithArgs("source").
		WillReturnRows(sqlmock.NewRows([]string{"binding_id"}).AddRow("source-binding"))
	mock.ExpectExec("(?s)INSERT INTO session_reference_bindings.*reference_mode").
		WithArgs("target", "source-binding", "source").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT target.binding_id").
		WithArgs("target", "source-binding", "source").
		WillReturnRows(sqlmock.NewRows([]string{"binding_id"}).AddRow("target-binding"))
	mock.ExpectExec("INSERT INTO session_reference_runtime").
		WithArgs("target-binding", "source-binding").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO session_migration_reference_binding_map").
		WithArgs(int64(7), "source-binding", "target-binding").
		WillReturnResult(sqlmock.NewResult(0, 1))

	bindings, runtimes, err := copySessionMigrationReferenceBindings(context.Background(), tx, 7, "source", "target")
	if err != nil || bindings != 1 || runtimes != 1 {
		t.Fatalf("copy result bindings=%d runtimes=%d err=%v", bindings, runtimes, err)
	}
	mock.ExpectRollback()
	_ = tx.Rollback()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	_ = m
}

func TestDeleteSessionMigrationReferenceBindingsDeletesOnlyMappedTargets(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("SELECT target_binding_id").WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"target_binding_id"}).AddRow("target-binding"))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM session_reference_bindings WHERE binding_id = ?")).
		WithArgs("target-binding").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE session_migration_reference_binding_map").WithArgs(int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	deleted, err := deleteSessionMigrationReferenceBindings(context.Background(), tx, 9)
	if err != nil || deleted != 1 {
		t.Fatalf("delete result deleted=%d err=%v", deleted, err)
	}
	mock.ExpectRollback()
	_ = tx.Rollback()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
