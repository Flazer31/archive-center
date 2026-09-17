package store

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMariaDBExplorerDeleteByID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	ctx := context.Background()

	mock.ExpectExec("DELETE FROM memories").WithArgs(int64(42), "sess-1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM kg_triples").WithArgs(int64(7), "sess-1").WillReturnResult(sqlmock.NewResult(0, 1))

	if err := m.DeleteMemoryByID(ctx, "sess-1", 42); err != nil {
		t.Fatalf("DeleteMemoryByID: %v", err)
	}
	if err := m.DeleteKGTripleByID(ctx, "sess-1", 7); err != nil {
		t.Fatalf("DeleteKGTripleByID: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
