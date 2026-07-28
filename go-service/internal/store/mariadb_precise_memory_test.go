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

func TestMariaDBPreciseMemoryWriterReportsInsertedAndReplayHonestly(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	now := time.Unix(100, 0).UTC()
	unit := &PreciseMemoryUnit{
		UnitID: "11111111-1111-5111-8111-111111111111", ContractVersion: PreciseMemoryUnitContract,
		ChatSessionID: "session", SourceTurnStart: 1, SourceTurnEnd: 1,
		SourceContract: "source_acceptance_observation.v1", SourceRevision: "revision",
		SourceContentHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		SourceRole:        "combined_turn_pair", SourceSpanStart: 0, SourceSpanEnd: 8,
		EvidenceExcerpt: "evidence", EvidenceHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		RootEvidenceID: 9, DirectEvidenceIDsJSON: "[9]", Kind: "event",
		PayloadJSON: "{}", TruthScope: "objective", EpistemicMode: "direct",
		AuthorityClass: "objective_world_state", AdmissionState: "committed",
		ReviewState: "source_observed", Visibility: "public", Confidence: 0.9,
		IdempotencyKey: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		LifecycleState: "active", CreatedAt: now, UpdatedAt: now,
	}
	insertPattern := regexp.QuoteMeta("INSERT INTO precise_memory_units")
	mock.ExpectExec(insertPattern).WillReturnResult(sqlmock.NewResult(7, 1))
	inserted, err := m.SavePreciseMemoryUnit(context.Background(), unit)
	if err != nil || !inserted || unit.ID != 7 {
		t.Fatalf("first save inserted=%v id=%d err=%v", inserted, unit.ID, err)
	}
	mock.ExpectExec(insertPattern).WillReturnError(&mysql.MySQLError{Number: 1062, Message: "Duplicate entry"})
	inserted, err = m.SavePreciseMemoryUnit(context.Background(), unit)
	if err != nil || inserted {
		t.Fatalf("replay inserted=%v err=%v, want no-op", inserted, err)
	}
	writeErr := &mysql.MySQLError{Number: 1452, Message: "Cannot add or update a child row"}
	mock.ExpectExec(insertPattern).WillReturnError(writeErr)
	if inserted, err = m.SavePreciseMemoryUnit(context.Background(), unit); inserted || !errors.Is(err, writeErr) {
		t.Fatalf("non-duplicate error inserted=%v err=%v, want surfaced FK error", inserted, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBPreciseMemoryAvailabilityRequiresOpenDatabase(t *testing.T) {
	if (&mariadbStore{}).PreciseMemoryWritesEnabled() {
		t.Fatal("closed MariaDB store advertised precise-memory writes")
	}
}
