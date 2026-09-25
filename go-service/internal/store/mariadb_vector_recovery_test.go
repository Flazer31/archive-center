package store

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestVectorRecoveryCacheIsReadOnlyAndScopedToRequestedIDs(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := &mariadbStore{db: db}
	mock.ExpectQuery(`SELECT document_json FROM memory_vector_outbox`).WithArgs("memory:session-a:1", "precise:session-b:2").
		WillReturnRows(sqlmock.NewRows([]string{"document_json"}).AddRow(`{"ID":"memory:session-a:1","Embedding":[1,0]}`))
	values, err := s.ReadVectorRecoveryCache(context.Background(), []string{"memory:session-a:1", "precise:session-b:2"})
	if err != nil || len(values) != 1 {
		t.Fatalf("values=%v err=%v", values, err)
	}
	if _, err := s.ReadVectorRecoveryCache(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
