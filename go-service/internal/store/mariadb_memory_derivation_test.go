package store

import (
	"context"
	"crypto/sha256"
	"database/sql/driver"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
)

func TestMariaDBSourceRevisionRegistrationIsIdempotentAndExact(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	now := time.Date(2026, 7, 28, 1, 2, 3, 0, time.UTC)
	source := testMemorySourceRevision(now)

	mock.ExpectBegin()
	expectSourceRegistrationTailLock(mock, source)
	mock.ExpectQuery("SELECT source_revision, combined_content_hash, raw_user_content, raw_assistant_content").
		WithArgs(source.ChatSessionID, source.LogicalTurnID, source.TurnIndex).
		WillReturnRows(sqlmock.NewRows([]string{"source_revision", "combined_content_hash", "raw_user_content", "raw_assistant_content"}))
	expectSourceRegistrationCanonicalPair(mock, source)
	mock.ExpectExec("INSERT INTO memory_source_revisions").WillReturnResult(sqlmock.NewResult(11, 1))
	mock.ExpectCommit()
	registered, err := m.RegisterAcceptedSourceRevision(context.Background(), source)
	if err != nil || !registered.Inserted || registered.Idempotent || source.ID != 11 {
		t.Fatalf("first registration = %+v id=%d err=%v", registered, source.ID, err)
	}

	mock.ExpectBegin()
	expectSourceRegistrationTailLock(mock, source)
	mock.ExpectQuery("SELECT source_revision, combined_content_hash, raw_user_content, raw_assistant_content").
		WithArgs(source.ChatSessionID, source.LogicalTurnID, source.TurnIndex).
		WillReturnRows(sqlmock.NewRows([]string{"source_revision", "combined_content_hash", "raw_user_content", "raw_assistant_content"}).
			AddRow(source.SourceRevision, source.CombinedContentHash, source.UserContent, source.AssistantContent))
	expectSourceRegistrationCanonicalPair(mock, source)
	mock.ExpectCommit()
	registered, err = m.RegisterAcceptedSourceRevision(context.Background(), source)
	if err != nil || registered.Inserted || !registered.Idempotent {
		t.Fatalf("replay registration = %+v err=%v", registered, err)
	}

	conflict := *source
	conflict.SourceRevision = "sar_newer"
	mock.ExpectBegin()
	expectSourceRegistrationTailLock(mock, source)
	mock.ExpectQuery("SELECT source_revision, combined_content_hash, raw_user_content, raw_assistant_content").
		WithArgs(source.ChatSessionID, source.LogicalTurnID, source.TurnIndex).
		WillReturnRows(sqlmock.NewRows([]string{"source_revision", "combined_content_hash", "raw_user_content", "raw_assistant_content"}).
			AddRow(source.SourceRevision, source.CombinedContentHash, source.UserContent, source.AssistantContent))
	expectSourceRegistrationCanonicalPair(mock, source)
	mock.ExpectRollback()
	if _, err := m.RegisterAcceptedSourceRevision(context.Background(), &conflict); !errors.Is(err, ErrSourceRevisionConflict) {
		t.Fatalf("conflict error = %v", err)
	}

	concurrent := *source
	concurrent.SourceRevision = "sar_concurrent"
	concurrent.LogicalTurnID = "logical-concurrent"
	mock.ExpectBegin()
	expectSourceRegistrationTailLock(mock, &concurrent)
	mock.ExpectQuery("SELECT source_revision, combined_content_hash, raw_user_content, raw_assistant_content").
		WithArgs(concurrent.ChatSessionID, concurrent.LogicalTurnID, concurrent.TurnIndex).
		WillReturnRows(sqlmock.NewRows([]string{"source_revision", "combined_content_hash", "raw_user_content", "raw_assistant_content"}).
			AddRow(source.SourceRevision, source.CombinedContentHash, source.UserContent, source.AssistantContent))
	expectSourceRegistrationCanonicalPair(mock, &concurrent)
	mock.ExpectRollback()
	if _, err := m.RegisterAcceptedSourceRevision(context.Background(), &concurrent); !errors.Is(err, ErrSourceRevisionConflict) {
		t.Fatalf("database active-slot conflict error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBSourceRevisionRegistrationRejectsAmbiguousActiveSources(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	source := testMemorySourceRevision(time.Date(2026, 7, 30, 4, 5, 6, 0, time.UTC))

	mock.ExpectBegin()
	expectSourceRegistrationTailLock(mock, source)
	mock.ExpectQuery("SELECT source_revision, combined_content_hash, raw_user_content, raw_assistant_content").
		WithArgs(source.ChatSessionID, source.LogicalTurnID, source.TurnIndex).
		WillReturnRows(sqlmock.NewRows([]string{"source_revision", "combined_content_hash", "raw_user_content", "raw_assistant_content"}).
			AddRow(source.SourceRevision, source.CombinedContentHash, source.UserContent, source.AssistantContent).
			AddRow("sar_duplicate_active", source.CombinedContentHash, source.UserContent, source.AssistantContent))
	mock.ExpectRollback()

	if _, err := m.RegisterAcceptedSourceRevision(context.Background(), source); !errors.Is(err, ErrSourceRevisionConflict) {
		t.Fatalf("ambiguous active source error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBSourceRevisionRegistrationRejectsStaleCanonicalRaw(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	source := testMemorySourceRevision(time.Date(2026, 7, 30, 4, 5, 6, 0, time.UTC))

	mock.ExpectBegin()
	expectSourceRegistrationTailLock(mock, source)
	mock.ExpectQuery("SELECT source_revision, combined_content_hash, raw_user_content, raw_assistant_content").
		WithArgs(source.ChatSessionID, source.LogicalTurnID, source.TurnIndex).
		WillReturnRows(sqlmock.NewRows([]string{"source_revision", "combined_content_hash", "raw_user_content", "raw_assistant_content"}))
	mock.ExpectQuery("SELECT role, content").
		WithArgs(source.ChatSessionID, source.TurnIndex).
		WillReturnRows(sqlmock.NewRows([]string{"role", "content"}).
			AddRow("user", source.UserContent).
			AddRow("assistant", "replacement assistant"))
	mock.ExpectRollback()

	if _, err := m.RegisterAcceptedSourceRevision(context.Background(), source); !errors.Is(err, ErrSourceRevisionConflict) {
		t.Fatalf("stale canonical raw error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func expectSourceRegistrationTailLock(mock sqlmock.Sqlmock, source *MemorySourceRevision) {
	mock.ExpectQuery("SELECT turn_index").
		WithArgs(source.ChatSessionID).
		WillReturnRows(sqlmock.NewRows([]string{"turn_index"}).AddRow(source.TurnIndex))
}

func expectSourceRegistrationCanonicalPair(mock sqlmock.Sqlmock, source *MemorySourceRevision) {
	mock.ExpectQuery("SELECT role, content").
		WithArgs(source.ChatSessionID, source.TurnIndex).
		WillReturnRows(sqlmock.NewRows([]string{"role", "content"}).
			AddRow("user", source.UserContent).
			AddRow("assistant", source.AssistantContent))
}

func TestMariaDBSourceRevisionReadsCommittedAdmissionSnapshot(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	now := time.Date(2026, 7, 28, 1, 2, 3, 0, time.UTC)
	resultJSON := `{"turn_summary":"first result"}`
	mock.ExpectQuery("SELECT id, contract_version, source_revision").
		WithArgs("session", "revision").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "contract_version", "source_revision", "chat_session_id",
			"logical_turn_id", "turn_index", "source_message_id",
			"source_generation_id", "branch_id", "branch_state",
			"raw_user_content", "raw_assistant_content", "combined_content_hash",
			"user_observed_content_hash", "assistant_observed_content_hash",
			"hash_algorithm", "host_observed_at_ms", "lifecycle_state",
			"superseded_by_revision", "invalidation_reason",
			"derived_admission_state", "derived_admission_version",
			"derived_extractor_version", "derived_index_version",
			"derived_result_hash", "derived_result_json", "derived_admitted_at",
			"critic_input_snapshot_json", "critic_input_snapshot_hash",
			"created_at", "updated_at",
		}).AddRow(
			11, MemorySourceRevisionContract, "revision", "session",
			"turn:4", 4, "message:4", "generation:4", nil, "not_exposed",
			"user", "assistant", strings.Repeat("a", 64),
			nil, nil, "sha256", int64(1234), "active",
			nil, nil, "committed", MemoryAdmissionContract,
			"critic.v1", MemoryVectorOutboxContract, strings.Repeat("b", 64),
			resultJSON, now, `{"contract_version":"critic_reprocessing_input.v1"}`,
			strings.Repeat("c", 64), now, now,
		))
	got, err := m.GetSourceRevision(context.Background(), "session", "revision")
	if err != nil {
		t.Fatal(err)
	}
	if got.DerivedAdmissionState != "committed" ||
		got.DerivedResultJSON != resultJSON ||
		got.DerivedAdmittedAt != now ||
		got.CriticInputSnapshotHash != strings.Repeat("c", 64) {
		t.Fatalf("source=%+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBCriticInputSnapshotIsImmutableAndIdempotent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	now := time.Date(2026, 8, 8, 12, 0, 0, 0, time.UTC)
	snapshotJSON := `{"contract_version":"critic_reprocessing_input.v1","source_revision":"revision"}`
	snapshotDigest := sha256.Sum256([]byte(snapshotJSON))
	snapshotHash := hex.EncodeToString(snapshotDigest[:])

	mock.ExpectExec("UPDATE memory_source_revisions").
		WithArgs(snapshotJSON, snapshotHash, now, "session", "revision").
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := m.SaveCriticInputSnapshot(context.Background(), "session", "revision", snapshotJSON, snapshotHash, now); err != nil {
		t.Fatal(err)
	}

	mock.ExpectExec("UPDATE memory_source_revisions").
		WithArgs(snapshotJSON, snapshotHash, now, "session", "revision").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT lifecycle_state, critic_input_snapshot_hash").
		WithArgs("session", "revision").
		WillReturnRows(sqlmock.NewRows([]string{"lifecycle_state", "critic_input_snapshot_hash"}).AddRow("active", snapshotHash))
	if err := m.SaveCriticInputSnapshot(context.Background(), "session", "revision", snapshotJSON, snapshotHash, now); err != nil {
		t.Fatalf("same snapshot must be idempotent: %v", err)
	}

	conflictingJSON := `{"contract_version":"critic_reprocessing_input.v1","source_revision":"revision","changed":true}`
	conflictingDigest := sha256.Sum256([]byte(conflictingJSON))
	conflictingHash := hex.EncodeToString(conflictingDigest[:])
	mock.ExpectExec("UPDATE memory_source_revisions").
		WithArgs(conflictingJSON, conflictingHash, now, "session", "revision").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT lifecycle_state, critic_input_snapshot_hash").
		WithArgs("session", "revision").
		WillReturnRows(sqlmock.NewRows([]string{"lifecycle_state", "critic_input_snapshot_hash"}).AddRow("active", snapshotHash))
	if err := m.SaveCriticInputSnapshot(context.Background(), "session", "revision", conflictingJSON, conflictingHash, now); !errors.Is(err, ErrSourceRevisionConflict) {
		t.Fatalf("conflicting snapshot error=%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBReprocessingJobReplayLeaseRecoveryAndStaleCompletion(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	now := time.Date(2026, 7, 28, 2, 0, 0, 0, time.UTC)
	job := &MemoryReprocessingJob{
		ContractVersion: MemoryReprocessingJobContract, IdempotencyKey: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ChatSessionID: "session", SourceRevision: "sar_active", SourceContract: "source_acceptance_observation.v1",
		DerivationVersion: PreciseMemoryUnitContract, ExtractorVersion: "critic.v1",
		IndexVersion: "index.v1", Status: "pending", CreatedAt: now, UpdatedAt: now,
	}
	mock.ExpectExec("INSERT INTO memory_reprocessing_jobs").WillReturnResult(sqlmock.NewResult(1, 1))
	inserted, err := m.EnqueueMemoryReprocessingJob(context.Background(), job)
	if err != nil || !inserted {
		t.Fatalf("enqueue inserted=%v err=%v", inserted, err)
	}
	mock.ExpectExec("INSERT INTO memory_reprocessing_jobs").
		WillReturnError(&mysql.MySQLError{Number: 1062, Message: "Duplicate entry"})
	mock.ExpectQuery("SELECT chat_session_id, source_revision, source_contract").
		WithArgs(job.IdempotencyKey).
		WillReturnRows(sqlmock.NewRows([]string{
			"chat_session_id", "source_revision", "source_contract",
			"derivation_version", "extractor_version", "index_version",
		}).AddRow(job.ChatSessionID, job.SourceRevision, job.SourceContract,
			job.DerivationVersion, job.ExtractorVersion, job.IndexVersion))
	inserted, err = m.EnqueueMemoryReprocessingJob(context.Background(), job)
	if err != nil || inserted {
		t.Fatalf("replay inserted=%v err=%v", inserted, err)
	}
	jobWriteErr := &mysql.MySQLError{Number: 1452, Message: "Cannot add child row"}
	mock.ExpectExec("INSERT INTO memory_reprocessing_jobs").WillReturnError(jobWriteErr)
	if inserted, err = m.EnqueueMemoryReprocessingJob(context.Background(), job); inserted || !errors.Is(err, jobWriteErr) {
		t.Fatalf("non-duplicate job error inserted=%v err=%v", inserted, err)
	}

	expired := now.Add(-time.Minute)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE memory_reprocessing_jobs j").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery(`(?s)SELECT j\.id, j\.contract_version.*j\.retry_after IS NULL OR j\.retry_after < \?.*j\.lease_until < \?`).
		WithArgs(now, now).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "contract_version", "idempotency_key", "chat_session_id",
			"source_revision", "source_contract", "derivation_version",
			"extractor_version", "index_version", "status", "attempts",
			"retry_after", "lease_owner", "lease_until", "last_error",
			"created_at", "updated_at",
		}).AddRow(5, job.ContractVersion, job.IdempotencyKey, job.ChatSessionID,
			job.SourceRevision, job.SourceContract, job.DerivationVersion,
			job.ExtractorVersion, job.IndexVersion, "leased", 2,
			nil, "dead-worker", expired, nil, now.Add(-time.Hour), expired))
	mock.ExpectExec("UPDATE memory_reprocessing_jobs").
		WithArgs("worker-2", now.Add(time.Minute), now, int64(5)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	claimed, err := m.ClaimMemoryReprocessingJob(context.Background(), "worker-2", now, time.Minute)
	if err != nil || claimed.ID != 5 || claimed.Attempts != 3 || claimed.LeaseOwner != "worker-2" {
		t.Fatalf("claimed=%+v err=%v", claimed, err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT j.lease_owner, j.lease_until, s.lifecycle_state").
		WithArgs(int64(5)).
		WillReturnRows(sqlmock.NewRows([]string{"lease_owner", "lease_until", "lifecycle_state"}).
			AddRow("worker-2", now.Add(time.Minute), "superseded"))
	mock.ExpectExec("UPDATE memory_reprocessing_jobs").
		WithArgs(now, int64(5)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := m.CompleteMemoryReprocessingJob(context.Background(), 5, "worker-2", now); !errors.Is(err, ErrSourceRevisionStale) {
		t.Fatalf("stale completion error=%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBReopenMemoryReprocessingJobResetsExactAdmissionSnapshotAndPreservesRaw(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	now := time.Date(2026, 7, 30, 3, 4, 5, 0, time.UTC)
	idempotencyKey := strings.Repeat("c", 64)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT j.id, j.chat_session_id, j.source_revision").
		WithArgs(idempotencyKey).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "chat_session_id", "source_revision", "status",
			"lease_until", "lifecycle_state",
		}).AddRow(17, "session", "revision", "completed", nil, "active"))
	mock.ExpectExec(regexp.QuoteMeta(`
		UPDATE memory_source_revisions
		SET derived_admission_state = 'pending',
		    derived_admission_version = '',
		    derived_extractor_version = '',
		    derived_index_version = '',
		    derived_result_hash = NULL,
		    derived_result_json = NULL,
		    derived_admitted_at = NULL,
		    updated_at = ?
		WHERE chat_session_id = ?
		  AND source_revision = ?
		  AND lifecycle_state = 'active'
	`)).
		WithArgs(now, "session", "revision").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE memory_reprocessing_jobs").
		WithArgs(now, int64(17), idempotencyKey).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	reopened, err := m.ReopenMemoryReprocessingJob(
		context.Background(), idempotencyKey, "session", "revision", now,
	)
	if err != nil || !reopened {
		t.Fatalf("reopened=%v err=%v", reopened, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBReopenMemoryReprocessingJobReportsActiveLease(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	now := time.Date(2026, 8, 3, 3, 4, 5, 0, time.UTC)
	idempotencyKey := strings.Repeat("d", 64)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT j.id, j.chat_session_id, j.source_revision").
		WithArgs(idempotencyKey).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "chat_session_id", "source_revision", "status",
			"lease_until", "lifecycle_state",
		}).AddRow(18, "session", "revision", "leased", now.Add(time.Minute), "active"))
	mock.ExpectRollback()

	reopened, err := m.ReopenMemoryReprocessingJob(
		context.Background(), idempotencyKey, "session", "revision", now,
	)
	if reopened || !errors.Is(err, ErrMemoryReprocessingLeased) {
		t.Fatalf("reopened=%v err=%v", reopened, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBVectorOutboxReplayLeaseRecoveryAndSourceFence(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	now := time.Date(2026, 7, 28, 2, 30, 0, 0, time.UTC)
	item := &MemoryVectorOutboxItem{
		ContractVersion: MemoryVectorOutboxContract,
		OperationKey:    "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Operation:       "upsert", ChatSessionID: "session", SourceRevision: "sar_active",
		DocumentID: "precise_memory:session:unit", DocumentJSON: `{"ID":"precise_memory:session:unit","Embedding":[0.1]}`,
		EmbeddingReady: true, RequiredSourceState: "active", Status: "pending",
		CreatedAt: now, UpdatedAt: now,
	}
	mock.ExpectExec("INSERT INTO memory_vector_outbox").WillReturnResult(sqlmock.NewResult(1, 1))
	inserted, err := m.EnqueueMemoryVectorOperation(context.Background(), item)
	if err != nil || !inserted {
		t.Fatalf("enqueue inserted=%v err=%v", inserted, err)
	}
	mock.ExpectExec("INSERT INTO memory_vector_outbox").
		WillReturnError(&mysql.MySQLError{Number: 1062, Message: "Duplicate entry"})
	mock.ExpectQuery("SELECT operation, chat_session_id, source_revision, document_id").
		WithArgs(item.OperationKey).
		WillReturnRows(sqlmock.NewRows([]string{
			"operation", "chat_session_id", "source_revision", "document_id",
			"document_json", "embedding_ready", "required_source_state",
		}).AddRow(item.Operation, item.ChatSessionID, item.SourceRevision,
			item.DocumentID, item.DocumentJSON, item.EmbeddingReady,
			item.RequiredSourceState))
	inserted, err = m.EnqueueMemoryVectorOperation(context.Background(), item)
	if err != nil || inserted {
		t.Fatalf("replay inserted=%v err=%v", inserted, err)
	}
	vectorWriteErr := &mysql.MySQLError{Number: 1452, Message: "Cannot add child row"}
	mock.ExpectExec("INSERT INTO memory_vector_outbox").WillReturnError(vectorWriteErr)
	if inserted, err = m.EnqueueMemoryVectorOperation(context.Background(), item); inserted || !errors.Is(err, vectorWriteErr) {
		t.Fatalf("non-duplicate vector error inserted=%v err=%v", inserted, err)
	}

	expired := now.Add(-time.Minute)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE memory_vector_outbox o").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT o.id, o.contract_version").
		WithArgs(now, now, now).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "contract_version", "operation_key", "operation",
			"chat_session_id", "source_revision", "document_id", "document_json",
			"embedding_ready", "required_source_state", "status", "attempts",
			"retry_after", "lease_owner", "lease_until", "last_error",
			"created_at", "updated_at",
		}).AddRow(9, item.ContractVersion, item.OperationKey, item.Operation,
			item.ChatSessionID, item.SourceRevision, item.DocumentID, item.DocumentJSON,
			true, item.RequiredSourceState, "leased", 1, nil, "dead-worker",
			expired, nil, now.Add(-time.Hour), expired))
	mock.ExpectExec("UPDATE memory_vector_outbox").
		WithArgs("worker", now.Add(time.Minute), now, int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	claimed, err := m.ClaimMemoryVectorOperations(context.Background(), "worker", now, time.Minute)
	if err != nil || len(claimed) != 1 || claimed[0].ID != 9 || claimed[0].Attempts != 2 {
		t.Fatalf("claimed=%+v err=%v", claimed, err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT o.status, o.lease_owner, o.lease_until, o.required_source_state, s.lifecycle_state").
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"status", "lease_owner", "lease_until", "required_source_state", "lifecycle_state"}).
			AddRow("leased", "worker", now.Add(time.Minute), "active", "superseded"))
	mock.ExpectExec("UPDATE memory_vector_outbox").
		WithArgs(now, int64(9)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := m.CompleteMemoryVectorOperation(context.Background(), 9, "worker", now); !errors.Is(err, ErrSourceRevisionStale) {
		t.Fatalf("stale completion error=%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBVectorOutboxFinishReportsInvalidationRaceAsSourceStale(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	now := time.Date(2026, 7, 28, 3, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT o.status, o.lease_owner, o.lease_until, o.required_source_state, s.lifecycle_state").
		WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{
			"status", "lease_owner", "lease_until", "required_source_state", "lifecycle_state",
		}).AddRow("stale_rejected", nil, nil, "active", "superseded"))
	mock.ExpectRollback()

	if err := m.CompleteMemoryVectorOperation(context.Background(), 44, "worker", now); !errors.Is(err, ErrSourceRevisionStale) {
		t.Fatalf("completion after invalidation error=%v, want ErrSourceRevisionStale", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBVectorOutboxClaimPreservesPerDocumentCausalOrder(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	now := time.Date(2026, 7, 28, 3, 30, 0, 0, time.UTC)
	leaseUntil := now.Add(time.Minute)
	documentID := "precise_memory:session:ordered"

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE memory_vector_outbox o").WillReturnResult(sqlmock.NewResult(0, 0))
	claimWithCausalGuard := `(?s)FROM memory_vector_outbox o.*AND NOT EXISTS \(\s*SELECT 1\s*FROM memory_vector_outbox prior\s*WHERE prior\.document_id = o\.document_id\s*AND prior\.id < o\.id\s*AND prior\.status IN \('pending', 'leased', 'retryable', 'needs_embedding'\)\s*\).*ORDER BY o\.created_at, o\.id`
	mock.ExpectQuery(claimWithCausalGuard).
		WithArgs(now, now, now).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "contract_version", "operation_key", "operation",
			"chat_session_id", "source_revision", "document_id", "document_json",
			"embedding_ready", "required_source_state", "status", "attempts",
			"retry_after", "lease_owner", "lease_until", "last_error",
			"created_at", "updated_at",
		}).AddRow(
			int64(41), MemoryVectorOutboxContract, strings.Repeat("d", 64), "delete",
			"session", "sar_old", documentID, nil, true, "inactive", "pending", 0,
			nil, nil, nil, nil, now.Add(-time.Minute), now.Add(-time.Minute),
		))
	mock.ExpectExec("UPDATE memory_vector_outbox").
		WithArgs("worker", leaseUntil, now, int64(41)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	claimed, err := m.ClaimMemoryVectorOperations(context.Background(), "worker", now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 || claimed[0].ID != 41 || claimed[0].DocumentID != documentID || claimed[0].Operation != "delete" {
		t.Fatalf("claim did not preserve oldest document operation: %#v", claimed)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBVectorOutboxClaimsDeferredRevisionSiblingsAsOneLeaseGroup(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	now := time.Date(2026, 8, 10, 3, 30, 0, 0, time.UTC)
	leaseUntil := now.Add(time.Minute)
	columns := []string{
		"id", "contract_version", "operation_key", "operation",
		"chat_session_id", "source_revision", "document_id", "document_json",
		"embedding_ready", "required_source_state", "status", "attempts",
		"retry_after", "lease_owner", "lease_until", "last_error",
		"created_at", "updated_at",
	}
	row := func(id int64, documentID string) []driver.Value {
		return []driver.Value{
			id, MemoryVectorOutboxContract, strings.Repeat("e", 64), "upsert",
			"session", "revision-turn", documentID, `{"ID":"` + documentID + `"}`,
			false, "active", "needs_embedding", 0, nil, nil, nil, nil,
			now.Add(-time.Minute), now.Add(-time.Minute),
		}
	}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE memory_vector_outbox o").WillReturnResult(sqlmock.NewResult(0, 0))
	seedRows := sqlmock.NewRows(columns)
	seedRows.AddRow(row(51, "memory:session:one")...)
	mock.ExpectQuery("SELECT o.id, o.contract_version").
		WithArgs(now, now, now).
		WillReturnRows(seedRows)
	siblingRows := sqlmock.NewRows(columns)
	siblingRows.AddRow(row(52, "memory:session:two")...)
	siblingRows.AddRow(row(53, "memory:session:three")...)
	mock.ExpectQuery(`(?s)WHERE o.source_revision = \?.*o.chat_session_id = \?.*o.operation = 'upsert'.*o.embedding_ready = FALSE.*ORDER BY o.created_at, o.id`).
		WithArgs("revision-turn", "session", int64(51), now, now).
		WillReturnRows(siblingRows)
	for _, id := range []int64{51, 52, 53} {
		mock.ExpectExec("UPDATE memory_vector_outbox").
			WithArgs("worker", leaseUntil, now, id).
			WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectCommit()

	claimed, err := m.ClaimMemoryVectorOperations(context.Background(), "worker", now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 3 {
		t.Fatalf("claimed=%#v", claimed)
	}
	for index, item := range claimed {
		if item.ID != int64(51+index) || item.SourceRevision != "revision-turn" || item.EmbeddingReady || item.Attempts != 1 {
			t.Fatalf("claimed item %d=%#v", index, item)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBVectorOutboxRejectsInvalidDocumentJSONBeforeWrite(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	item := &MemoryVectorOutboxItem{
		OperationKey:        "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		Operation:           "upsert",
		ChatSessionID:       "session",
		SourceRevision:      "revision",
		DocumentID:          "memory:session:1",
		DocumentJSON:        `{"broken":`,
		RequiredSourceState: "active",
	}
	if inserted, err := m.EnqueueMemoryVectorOperation(context.Background(), item); inserted || err == nil {
		t.Fatalf("invalid JSON inserted=%v err=%v", inserted, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBListsOnlyActiveSourceRevisionsForDurableRescan(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	st := &mariadbStore{db: db}
	mock.ExpectQuery("SELECT source_revision, chat_session_id, logical_turn_id, turn_index").
		WithArgs("session", 3, 7).
		WillReturnRows(sqlmock.NewRows([]string{
			"source_revision", "chat_session_id", "logical_turn_id", "turn_index",
			"source_message_id", "source_generation_id", "branch_id", "branch_state",
			"raw_user_content", "raw_assistant_content", "combined_content_hash",
			"host_observed_at_ms", "lifecycle_state",
		}).AddRow(
			"revision", "session", "turn:4", 4, "message:4", "generation:4",
			"branch:4", "observed", "user", "assistant",
			"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			int64(1234), "active",
		))
	items, err := st.ListActiveSourceRevisions(
		context.Background(), "session", 3, 7,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].SourceRevision != "revision" ||
		items[0].TurnIndex != 4 || items[0].HostObservedAtMS != 1234 ||
		items[0].BranchID != "branch:4" ||
		items[0].ContractVersion != MemorySourceRevisionContract {
		t.Fatalf("items=%+v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBLogicalReplacementInvalidatesDescendantsAndQueuesVectorDeletes(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	now := time.Date(2026, 7, 28, 3, 0, 0, 0, time.UTC)
	source := testMemorySourceRevision(now)
	source.SourceRevision = "sar_new"

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT turn_index FROM chat_logs WHERE chat_session_id = ? ORDER BY turn_index DESC, id DESC LIMIT 1 FOR UPDATE")).
		WithArgs(source.ChatSessionID).
		WillReturnRows(sqlmock.NewRows([]string{"turn_index"}).AddRow(source.TurnIndex))
	mock.ExpectQuery("SELECT source_revision").
		WithArgs(source.ChatSessionID, source.TurnIndex).
		WillReturnRows(sqlmock.NewRows([]string{"source_revision"}).AddRow("sar_old"))
	mock.ExpectQuery("SELECT id FROM memories").
		WithArgs(source.ChatSessionID, source.TurnIndex).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(7))
	mock.ExpectQuery("SELECT id FROM direct_evidence_records").
		WithArgs(source.ChatSessionID, source.TurnIndex).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(8))
	mock.ExpectQuery("SELECT id FROM world_rules").
		WithArgs(source.ChatSessionID, source.TurnIndex).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery("SELECT DISTINCT document_id").
		WithArgs(source.ChatSessionID, "sar_old").
		WillReturnRows(sqlmock.NewRows([]string{"document_id"}).AddRow("precise_memory:session:unit"))
	for range 5 {
		mock.ExpectExec("INSERT INTO memory_vector_outbox").WillReturnResult(sqlmock.NewResult(1, 1))
	}
	mock.ExpectExec("UPDATE memory_derivation_dependencies").WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectExec("UPDATE precise_memory_units").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE memory_reprocessing_jobs").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE memory_vector_outbox").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE memory_source_revisions").
		WithArgs("superseded", source.SourceRevision, "logical_turn_replaced", now, now,
			"superseded", "superseded", "superseded", "superseded", "superseded",
			"superseded", "superseded",
			source.ChatSessionID, "sar_old").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE memory_source_revisions").
		WithArgs(source.SourceRevision, now, now, source.ChatSessionID, source.TurnIndex, source.SourceRevision).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO memory_source_revisions").WillReturnResult(sqlmock.NewResult(12, 1))
	mock.ExpectExec("DELETE FROM effective_input_logs").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM memories").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM direct_evidence_records").WillReturnResult(sqlmock.NewResult(0, 1))
	for range 34 {
		mock.ExpectExec(`(?s).+`).WillReturnResult(sqlmock.NewResult(0, 1))
	}
	mock.ExpectExec("INSERT INTO status_current_values").
		WithArgs(source.ChatSessionID).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO chat_logs")).
		WithArgs(source.ChatSessionID, source.TurnIndex, source.UserContent, now).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO chat_logs")).
		WithArgs(source.ChatSessionID, source.TurnIndex, source.AssistantContent, now).
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()

	if err := m.ReplaceLogicalTurn(context.Background(), LogicalTurnReplacement{
		ChatSessionID: source.ChatSessionID, TurnIndex: source.TurnIndex,
		UserContent: source.UserContent, AssistantContent: source.AssistantContent,
		CreatedAt: now, SourceRevision: source,
	}); err != nil {
		t.Fatalf("replacement: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func testMemorySourceRevision(now time.Time) *MemorySourceRevision {
	return &MemorySourceRevision{
		ContractVersion: MemorySourceRevisionContract, SourceRevision: "sar_active",
		ChatSessionID: "session", LogicalTurnID: "lt_turn", TurnIndex: 3,
		SourceMessageID: "chat:index:2", SourceGenerationID: "generation",
		BranchState: "not_exposed", UserContent: "user text",
		AssistantContent:        "assistant text",
		CombinedContentHash:     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		UserObservedContentHash: "or1c_user", AssistantObservedContentHash: "or1c_assistant",
		HashAlgorithm: "djb2.v1", HostObservedAtMS: now.UnixMilli(),
		LifecycleState: "active", CreatedAt: now, UpdatedAt: now,
	}
}
