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
	expectActiveSource := func() {
		mock.ExpectBegin()
		mock.ExpectQuery("SELECT lifecycle_state").
			WithArgs(unit.ChatSessionID, unit.SourceRevision).
			WillReturnRows(sqlmock.NewRows([]string{"lifecycle_state"}).AddRow("active"))
	}
	expectActiveSource()
	mock.ExpectExec(insertPattern).WillReturnResult(sqlmock.NewResult(7, 1))
	mock.ExpectExec("INSERT INTO memory_derivation_dependencies").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO memory_derivation_dependencies").WillReturnResult(sqlmock.NewResult(2, 1))
	preciseDocumentID := "precise_memory:" + unit.ChatSessionID + ":" + unit.UnitID
	mock.ExpectExec("INSERT INTO memory_vector_outbox").
		WithArgs(
			MemoryVectorOutboxContract,
			preciseMemoryVectorOperationKey(
				unit.ChatSessionID, unit.SourceRevision, unit.DerivationVersion,
				unit.ExtractorVersion, unit.IndexVersion, preciseDocumentID,
			),
			"upsert", unit.ChatSessionID, unit.SourceRevision, preciseDocumentID,
			sqlmock.AnyArg(), false, "active", "needs_embedding", 0,
			nil, nil, nil, nil, now, now,
		).
		WillReturnResult(sqlmock.NewResult(3, 1))
	mock.ExpectCommit()
	inserted, err := m.SavePreciseMemoryUnit(context.Background(), unit)
	if err != nil || !inserted || unit.ID != 7 {
		t.Fatalf("first save inserted=%v id=%d err=%v", inserted, unit.ID, err)
	}
	expectActiveSource()
	mock.ExpectExec(insertPattern).WillReturnError(&mysql.MySQLError{Number: 1062, Message: "Duplicate entry"})
	mock.ExpectQuery("SELECT unit_id, source_revision, idempotency_key").
		WithArgs(unit.UnitID, unit.IdempotencyKey, unit.UnitID).
		WillReturnRows(sqlmock.NewRows([]string{"unit_id", "source_revision", "idempotency_key"}).
			AddRow(unit.UnitID, unit.SourceRevision, unit.IdempotencyKey))
	mock.ExpectCommit()
	inserted, err = m.SavePreciseMemoryUnit(context.Background(), unit)
	if err != nil || inserted {
		t.Fatalf("replay inserted=%v err=%v, want no-op", inserted, err)
	}
	writeErr := &mysql.MySQLError{Number: 1452, Message: "Cannot add or update a child row"}
	expectActiveSource()
	mock.ExpectExec(insertPattern).WillReturnError(writeErr)
	mock.ExpectRollback()
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

func TestMariaDBPreciseMemoryDependencyWriteSurfacesNonDuplicateError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	now := time.Unix(200, 0).UTC()
	unit := &PreciseMemoryUnit{
		UnitID: "22222222-2222-5222-8222-222222222222", ContractVersion: PreciseMemoryUnitContract,
		ChatSessionID: "session", SourceTurnStart: 2, SourceTurnEnd: 2,
		SourceContract: "source_acceptance_observation.v1", SourceRevision: "revision",
		SourceContentHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		SourceRole:        "combined_turn_pair", SourceSpanStart: 0, SourceSpanEnd: 8,
		EvidenceExcerpt: "evidence", EvidenceHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		DirectEvidenceIDsJSON: "[]", Kind: "event", PayloadJSON: "{}",
		TruthScope: "objective", EpistemicMode: "direct", AuthorityClass: "objective_world_state",
		AdmissionState: "committed", ReviewState: "source_observed", Visibility: "public",
		Confidence: 0.9, IdempotencyKey: "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
		LifecycleState: "active", CreatedAt: now, UpdatedAt: now,
	}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT lifecycle_state").
		WithArgs(unit.ChatSessionID, unit.SourceRevision).
		WillReturnRows(sqlmock.NewRows([]string{"lifecycle_state"}).AddRow("active"))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO precise_memory_units")).
		WillReturnResult(sqlmock.NewResult(8, 1))
	dependencyErr := &mysql.MySQLError{Number: 1452, Message: "Cannot add child row"}
	mock.ExpectExec("INSERT INTO memory_derivation_dependencies").WillReturnError(dependencyErr)
	mock.ExpectRollback()
	if inserted, err := m.SavePreciseMemoryUnit(context.Background(), unit); inserted || !errors.Is(err, dependencyErr) {
		t.Fatalf("dependency error inserted=%v err=%v", inserted, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
