package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMariaDBEntityIdentityWriteRequiresActiveAcceptedSourceRevision(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	now := time.Date(2026, 7, 30, 1, 2, 3, 0, time.UTC)
	item := &EntityIdentity{
		StableEntityID: "entity-1",
		ChatSessionID:  "session-1",
		SourceContract: acceptedSourceObservationContract,
		SourceRevision: "revision-1",
		SourceTurn:     3,
		IdempotencyKey: "identity-key",
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT lifecycle_state").
		WithArgs("session-1", "revision-1").
		WillReturnRows(sqlmock.NewRows([]string{"lifecycle_state"}).AddRow("active"))
	mock.ExpectExec("INSERT INTO entity_identities").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := m.SaveEntityIdentity(context.Background(), item); err != nil {
		t.Fatalf("active source write failed: %v", err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT lifecycle_state").
		WithArgs("session-1", "revision-1").
		WillReturnRows(sqlmock.NewRows([]string{"lifecycle_state"}).AddRow("superseded"))
	mock.ExpectRollback()
	if err := m.SaveEntityIdentity(context.Background(), item); !errors.Is(err, ErrSourceRevisionStale) {
		t.Fatalf("stale source error = %v, want ErrSourceRevisionStale", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBEntityIdentityLegacyWriteKeepsLegacyPath(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	item := &EntityIdentity{
		StableEntityID: "entity-legacy",
		ChatSessionID:  "session-legacy",
		SourceContract: "legacy_unverified",
		SourceRevision: "legacy-revision",
		SourceTurn:     2,
		IdempotencyKey: "legacy-key",
	}

	mock.ExpectExec("INSERT INTO entity_identities").
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := m.SaveEntityIdentity(context.Background(), item); err != nil {
		t.Fatalf("legacy write failed: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
