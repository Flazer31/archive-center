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
	mock.ExpectQuery("SELECT source_revision, combined_content_hash, raw_user_content, raw_assistant_content").
		WithArgs(source.ChatSessionID, source.LogicalTurnID).
		WillReturnRows(sqlmock.NewRows([]string{"source_revision", "combined_content_hash", "raw_user_content", "raw_assistant_content"}))
	mock.ExpectExec("INSERT INTO memory_source_revisions").WillReturnResult(sqlmock.NewResult(11, 1))
	mock.ExpectCommit()
	registered, err := m.RegisterAcceptedSourceRevision(context.Background(), source)
	if err != nil || !registered.Inserted || registered.Idempotent || source.ID != 11 {
		t.Fatalf("first registration = %+v id=%d err=%v", registered, source.ID, err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT source_revision, combined_content_hash, raw_user_content, raw_assistant_content").
		WithArgs(source.ChatSessionID, source.LogicalTurnID).
		WillReturnRows(sqlmock.NewRows([]string{"source_revision", "combined_content_hash", "raw_user_content", "raw_assistant_content"}).
			AddRow(source.SourceRevision, source.CombinedContentHash, source.UserContent, source.AssistantContent))
	mock.ExpectCommit()
	registered, err = m.RegisterAcceptedSourceRevision(context.Background(), source)
	if err != nil || registered.Inserted || !registered.Idempotent {
		t.Fatalf("replay registration = %+v err=%v", registered, err)
	}

	conflict := *source
	conflict.SourceRevision = "sar_newer"
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT source_revision, combined_content_hash, raw_user_content, raw_assistant_content").
		WithArgs(source.ChatSessionID, source.LogicalTurnID).
		WillReturnRows(sqlmock.NewRows([]string{"source_revision", "combined_content_hash", "raw_user_content", "raw_assistant_content"}).
			AddRow(source.SourceRevision, source.CombinedContentHash, source.UserContent, source.AssistantContent))
	mock.ExpectRollback()
	if _, err := m.RegisterAcceptedSourceRevision(context.Background(), &conflict); !errors.Is(err, ErrSourceRevisionConflict) {
		t.Fatalf("conflict error = %v", err)
	}

	concurrent := *source
	concurrent.SourceRevision = "sar_concurrent"
	concurrent.LogicalTurnID = "logical-concurrent"
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT source_revision, combined_content_hash, raw_user_content, raw_assistant_content").
		WithArgs(concurrent.ChatSessionID, concurrent.LogicalTurnID).
		WillReturnRows(sqlmock.NewRows([]string{"source_revision", "combined_content_hash", "raw_user_content", "raw_assistant_content"}))
	mock.ExpectExec("INSERT INTO memory_source_revisions").
		WillReturnError(&mysql.MySQLError{Number: 1062, Message: "uq_memory_source_active_turn"})
	mock.ExpectRollback()
	if _, err := m.RegisterAcceptedSourceRevision(context.Background(), &concurrent); !errors.Is(err, ErrSourceRevisionConflict) {
		t.Fatalf("database active-slot conflict error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
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
			"created_at", "updated_at",
		}).AddRow(
			11, MemorySourceRevisionContract, "revision", "session",
			"turn:4", 4, "message:4", "generation:4", nil, "not_exposed",
			"user", "assistant", strings.Repeat("a", 64),
			nil, nil, "sha256", int64(1234), "active",
			nil, nil, "committed", MemoryAdmissionContract,
			"critic.v1", MemoryVectorOutboxContract, strings.Repeat("b", 64),
			resultJSON, now, now, now,
		))
	got, err := m.GetSourceRevision(context.Background(), "session", "revision")
	if err != nil {
		t.Fatal(err)
	}
	if got.DerivedAdmissionState != "committed" ||
		got.DerivedResultJSON != resultJSON ||
		got.DerivedAdmittedAt != now {
		t.Fatalf("source=%+v", got)
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
	mock.ExpectQuery("SELECT j.id, j.contract_version").
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
	claimed, err := m.ClaimMemoryVectorOperation(context.Background(), "worker", now, time.Minute)
	if err != nil || claimed.ID != 9 || claimed.Attempts != 2 {
		t.Fatalf("claimed=%+v err=%v", claimed, err)
	}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT o.lease_owner, o.lease_until, o.required_source_state, s.lifecycle_state").
		WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"lease_owner", "lease_until", "required_source_state", "lifecycle_state"}).
			AddRow("worker", now.Add(time.Minute), "active", "superseded"))
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
			"source_message_id", "source_generation_id", "raw_user_content",
			"raw_assistant_content", "combined_content_hash", "lifecycle_state",
		}).AddRow(
			"revision", "session", "turn:4", 4, "message:4", "generation:4",
			"user", "assistant",
			"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"active",
		))
	items, err := st.ListActiveSourceRevisions(
		context.Background(), "session", 3, 7,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].SourceRevision != "revision" ||
		items[0].TurnIndex != 4 || items[0].ContractVersion != MemorySourceRevisionContract {
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
			source.ChatSessionID, "sar_old").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO memory_source_revisions").WillReturnResult(sqlmock.NewResult(12, 1))
	mock.ExpectExec("DELETE FROM effective_input_logs").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM memories").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM direct_evidence_records").WillReturnResult(sqlmock.NewResult(0, 1))
	for range 33 {
		mock.ExpectExec(`(?s).+`).WillReturnResult(sqlmock.NewResult(0, 1))
	}
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
