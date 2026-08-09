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

func TestMariaDBMemoryAdmissionRetriesDeadlockTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	st := &mariadbStore{db: db}
	admission := &MemoryAdmission{
		ContractVersion:   MemoryAdmissionContract,
		ChatSessionID:     "session",
		SourceRevision:    "revision",
		TurnIndex:         2,
		DerivationVersion: MemoryAdmissionContract,
		ExtractorVersion:  "critic.v1",
		IndexVersion:      MemoryVectorOutboxContract,
		ResultJSON:        `{}`,
		CreatedAt:         time.Date(2026, 8, 7, 5, 0, 0, 0, time.UTC),
	}
	admission.ResultHash = memoryAdmissionExpectedResultHash(admission)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT lifecycle_state, derived_admission_state").
		WithArgs("session", "revision", 2).
		WillReturnError(&mysql.MySQLError{Number: 1213, Message: "deadlock"})
	mock.ExpectRollback()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT lifecycle_state, derived_admission_state").
		WithArgs("session", "revision", 2).
		WillReturnRows(sqlmock.NewRows([]string{
			"lifecycle_state", "derived_admission_state",
			"derived_admission_version", "derived_extractor_version",
			"derived_index_version", "derived_result_hash", "derived_result_json",
		}).AddRow("active", "pending", "", "", "", nil, nil))
	mock.ExpectQuery("SELECT id, evidence_text, tombstoned").
		WithArgs("session", 2, 2).
		WillReturnRows(sqlmock.NewRows([]string{"id", "evidence_text", "tombstoned"}))
	mock.ExpectQuery("SELECT id, unit_id, idempotency_key, lifecycle_state").
		WithArgs("session", "revision").
		WillReturnRows(sqlmock.NewRows([]string{"id", "unit_id", "idempotency_key", "lifecycle_state"}))
	mock.ExpectExec("UPDATE memory_source_revisions").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	got, err := st.CommitMemoryAdmission(context.Background(), admission)
	if err != nil {
		t.Fatal(err)
	}
	if got.CommittedResultHash != admission.ResultHash {
		t.Fatalf("committed hash=%q, want %q", got.CommittedResultHash, admission.ResultHash)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBMemoryAdmissionStopsAfterDeadlockRetryLimit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	st := &mariadbStore{db: db}
	admission := &MemoryAdmission{
		ContractVersion:   MemoryAdmissionContract,
		ChatSessionID:     "session",
		SourceRevision:    "revision",
		TurnIndex:         2,
		DerivationVersion: MemoryAdmissionContract,
		ExtractorVersion:  "critic.v1",
		IndexVersion:      MemoryVectorOutboxContract,
		ResultJSON:        `{}`,
		CreatedAt:         time.Date(2026, 8, 7, 6, 0, 0, 0, time.UTC),
	}
	admission.ResultHash = memoryAdmissionExpectedResultHash(admission)
	for attempt := 0; attempt < memoryAdmissionTransactionMaxAttempts; attempt++ {
		mock.ExpectBegin()
		mock.ExpectQuery("SELECT lifecycle_state, derived_admission_state").
			WithArgs("session", "revision", 2).
			WillReturnError(&mysql.MySQLError{Number: 1213, Message: "deadlock"})
		mock.ExpectRollback()
	}
	mock.ExpectExec("UPDATE memory_source_revisions").
		WithArgs(admission.DerivationVersion, admission.ExtractorVersion,
			admission.IndexVersion, admission.ResultHash, admission.ResultJSON,
			admission.CreatedAt, admission.ChatSessionID, admission.SourceRevision,
			admission.TurnIndex).
		WillReturnResult(sqlmock.NewResult(0, 1))

	_, err = st.CommitMemoryAdmission(context.Background(), admission)
	var mysqlErr *mysql.MySQLError
	if !errors.As(err, &mysqlErr) || mysqlErr.Number != 1213 {
		t.Fatalf("err=%v, want MySQL 1213 after bounded retries", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBMemoryAdmissionDoesNotStageSuccessfulCommit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	st := &mariadbStore{db: db}
	admission := &MemoryAdmission{
		ContractVersion: MemoryAdmissionContract, ChatSessionID: "session",
		SourceRevision: "revision", TurnIndex: 2,
		DerivationVersion: MemoryAdmissionContract, ExtractorVersion: "critic.v1",
		IndexVersion: MemoryVectorOutboxContract, ResultJSON: `{}`,
		CreatedAt: time.Date(2026, 8, 7, 6, 30, 0, 0, time.UTC),
	}
	admission.ResultHash = memoryAdmissionExpectedResultHash(admission)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT lifecycle_state, derived_admission_state").
		WithArgs("session", "revision", 2).
		WillReturnRows(sqlmock.NewRows([]string{
			"lifecycle_state", "derived_admission_state",
			"derived_admission_version", "derived_extractor_version",
			"derived_index_version", "derived_result_hash", "derived_result_json",
		}).AddRow("active", "pending", "", "", "", nil, nil))
	mock.ExpectQuery("SELECT id, evidence_text, tombstoned").
		WithArgs("session", 2, 2).
		WillReturnRows(sqlmock.NewRows([]string{"id", "evidence_text", "tombstoned"}))
	mock.ExpectQuery("SELECT id, unit_id, idempotency_key, lifecycle_state").
		WithArgs("session", "revision").
		WillReturnRows(sqlmock.NewRows([]string{"id", "unit_id", "idempotency_key", "lifecycle_state"}))
	mock.ExpectExec("UPDATE memory_source_revisions").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if _, err := st.CommitMemoryAdmission(context.Background(), admission); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBMemoryAdmissionCommitsCoreProjectionsAndOutboxAtomically(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	st := &mariadbStore{db: db}
	now := time.Date(2026, 7, 28, 7, 0, 0, 0, time.UTC)
	evidence := &DirectEvidence{
		ID: 1, ChatSessionID: "session", EvidenceKind: "turn_excerpt",
		EvidenceText: "Mina found the key.", SourceTurnStart: 4, SourceTurnEnd: 4,
		TurnAnchor: 4, ArchiveState: "verified_direct",
		CaptureStage: "critic_extract", CaptureVerification: "verified",
		CommittedGate: "auto_grounded_excerpt", SourceMessageIDsJSON: `["turn:4"]`,
		LineageJSON: `{}`, CreatedAt: now,
	}
	unit := &PreciseMemoryUnit{
		UnitID: "11111111-1111-5111-8111-111111111111", ContractVersion: PreciseMemoryUnitContract,
		ChatSessionID: "session", SourceTurnStart: 4, SourceTurnEnd: 4,
		SourceContract: "source_acceptance_observation.v1", SourceRevision: "revision",
		SourceContentHash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		SourceRole:        "combined_turn_pair", SourceSpanStart: 0, SourceSpanEnd: 19,
		EvidenceExcerpt: evidence.EvidenceText,
		EvidenceHash:    "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		RootEvidenceID:  1, DirectEvidenceIDsJSON: "[1]", Kind: "event",
		PayloadJSON: "{}", TruthScope: "objective", EpistemicMode: "direct",
		AuthorityClass: "objective_world_state", AdmissionState: "committed",
		ReviewState: "source_observed", Visibility: "public", Confidence: 0.9,
		IdempotencyKey:    "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		DerivationVersion: MemoryAdmissionContract,
		ExtractorVersion:  "critic.v1", IndexVersion: MemoryVectorOutboxContract,
		LifecycleState: "active", CreatedAt: now, UpdatedAt: now,
	}
	admission := &MemoryAdmission{
		ContractVersion: MemoryAdmissionContract, ChatSessionID: "session",
		SourceRevision: "revision", TurnIndex: 4,
		DerivationVersion: MemoryAdmissionContract, ExtractorVersion: "critic.v1",
		IndexVersion: MemoryVectorOutboxContract,
		ResultJSON:   `{"turn_summary":"Mina found the key."}`,
		Memory: &Memory{
			ChatSessionID: "session", TurnIndex: 4, SummaryJSON: `{"turn_summary":"Mina found the key."}`,
			Embedding: "[]", EmbeddingModel: "not_configured", Importance: 0.7,
			Evidence: `{}`, CreatedAt: now,
		},
		Evidence: []*DirectEvidence{evidence}, PreciseUnits: []*PreciseMemoryUnit{unit},
		Vectors: []MemoryAdmissionVector{{
			ArtifactType: "memory", Tier: "memory", SourceTable: "memories",
			SchemaVersion: "memory.v2", DocumentText: "Mina found the key.",
		}},
		CreatedAt: now,
	}
	admission.ResultHash = memoryAdmissionExpectedResultHash(admission)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT lifecycle_state, derived_admission_state").
		WithArgs("session", "revision", 4).
		WillReturnRows(sqlmock.NewRows([]string{
			"lifecycle_state", "derived_admission_state",
			"derived_admission_version", "derived_extractor_version",
			"derived_index_version", "derived_result_hash", "derived_result_json",
		}).AddRow("active", "pending", "", "", "", nil, nil))
	mock.ExpectQuery("SELECT id[\\s\\S]+FROM memories").
		WithArgs("session", 4).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO memories")).
		WillReturnResult(sqlmock.NewResult(11, 1))
	mock.ExpectQuery("SELECT id, evidence_text, tombstoned").
		WithArgs("session", 4, 4).
		WillReturnRows(sqlmock.NewRows([]string{"id", "evidence_text", "tombstoned"}))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO direct_evidence_records")).
		WillReturnResult(sqlmock.NewResult(21, 1))
	mock.ExpectQuery("SELECT id, unit_id, idempotency_key, lifecycle_state").
		WithArgs("session", "revision").
		WillReturnRows(sqlmock.NewRows([]string{"id", "unit_id", "idempotency_key", "lifecycle_state"}))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO precise_memory_units")).
		WillReturnResult(sqlmock.NewResult(31, 1))
	mock.ExpectExec("INSERT INTO memory_derivation_dependencies").
		WillReturnError(&mysql.MySQLError{Number: 1213, Message: "deadlock after projection writes"})
	mock.ExpectRollback()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT lifecycle_state, derived_admission_state").
		WithArgs("session", "revision", 4).
		WillReturnRows(sqlmock.NewRows([]string{
			"lifecycle_state", "derived_admission_state",
			"derived_admission_version", "derived_extractor_version",
			"derived_index_version", "derived_result_hash", "derived_result_json",
		}).AddRow("active", "pending", "", "", "", nil, nil))
	mock.ExpectQuery("SELECT id[\\s\\S]+FROM memories").
		WithArgs("session", 4).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO memories")).
		WillReturnResult(sqlmock.NewResult(12, 1))
	mock.ExpectQuery("SELECT id, evidence_text, tombstoned").
		WithArgs("session", 4, 4).
		WillReturnRows(sqlmock.NewRows([]string{"id", "evidence_text", "tombstoned"}))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO direct_evidence_records")).
		WillReturnResult(sqlmock.NewResult(22, 1))
	mock.ExpectQuery("SELECT id, unit_id, idempotency_key, lifecycle_state").
		WithArgs("session", "revision").
		WillReturnRows(sqlmock.NewRows([]string{"id", "unit_id", "idempotency_key", "lifecycle_state"}))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO precise_memory_units")).
		WillReturnResult(sqlmock.NewResult(32, 1))
	mock.ExpectExec("INSERT INTO memory_derivation_dependencies").
		WillReturnResult(sqlmock.NewResult(41, 1))
	mock.ExpectExec("INSERT INTO memory_derivation_dependencies").
		WillReturnResult(sqlmock.NewResult(42, 1))
	mock.ExpectExec("INSERT INTO memory_vector_outbox").
		WillReturnResult(sqlmock.NewResult(51, 1))
	mock.ExpectExec("INSERT INTO memory_vector_outbox").
		WillReturnResult(sqlmock.NewResult(52, 1))
	mock.ExpectExec("UPDATE memory_source_revisions").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	got, err := st.CommitMemoryAdmission(context.Background(), admission)
	if err != nil {
		t.Fatal(err)
	}
	if !got.MemoryInserted || got.EvidenceInserted != 1 ||
		got.PreciseInserted != 1 || got.VectorOperations != 2 ||
		admission.Memory.ID != 12 || evidence.ID != 22 ||
		unit.ID != 32 || unit.RootEvidenceID != 22 ||
		unit.DirectEvidenceIDsJSON != "[22]" {
		t.Fatalf("unexpected result=%+v memory=%d evidence=%d unit=%d root=%d",
			got, admission.Memory.ID, evidence.ID, unit.ID, unit.RootEvidenceID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReconcileAdmissionPrivatePerspectiveDoesNotClaimGeneralVectorUpsert(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 7, 31, 1, 0, 0, 0, time.UTC)
	unit := &PreciseMemoryUnit{
		UnitID: "private-unit", ContractVersion: PreciseMemoryUnitContract,
		ChatSessionID: "session", SourceRevision: "revision",
		SourceTurnStart: 3, SourceTurnEnd: 3,
		SourceContract:    acceptedSourceObservationContract,
		SourceContentHash: strings.Repeat("a", 64), SourceRole: "combined_turn_pair",
		EvidenceExcerpt: "Mira alone knows the map.", EvidenceHash: strings.Repeat("b", 64),
		DirectEvidenceIDsJSON: "[]", Kind: "observation",
		PayloadJSON: `{"contract_version":"perspective_memory.v1"}`,
		TruthScope:  "owner_scoped", EpistemicMode: "known",
		AuthorityClass: "subjective_episodic", AdmissionState: "committed",
		ReviewState: "source_observed", Visibility: "owner_private",
		KnowledgeHolderEntityID: "holder-mira", Confidence: 0.9,
		IdempotencyKey:    strings.Repeat("c", 64),
		DerivationVersion: PreciseMemoryUnitContract,
		ExtractorVersion:  "critic.v1", IndexVersion: "not_materialized",
		LifecycleState: "active", CreatedAt: now, UpdatedAt: now,
	}
	admission := &MemoryAdmission{
		ChatSessionID: "session", SourceRevision: "revision",
		PreciseUnits: []*PreciseMemoryUnit{unit}, CreatedAt: now,
	}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id, unit_id, idempotency_key, lifecycle_state").
		WithArgs("session", "revision").
		WillReturnRows(sqlmock.NewRows([]string{"id", "unit_id", "idempotency_key", "lifecycle_state"}))
	mock.ExpectExec("INSERT INTO precise_memory_units").
		WillReturnResult(sqlmock.NewResult(31, 1))
	mock.ExpectExec("INSERT INTO memory_derivation_dependencies").
		WillReturnResult(sqlmock.NewResult(41, 1))
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := reconcileAdmissionPreciseMemoryTx(context.Background(), tx, admission)
	if err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	mock.ExpectCommit()
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if got.inserted != 1 || got.vectorOperations != 0 {
		t.Fatalf("private insert result=%+v, want inserted without general vector operation", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBMemoryAdmissionFreezesFirstCommittedResultForReplay(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	st := &mariadbStore{db: db}
	admission := &MemoryAdmission{
		ContractVersion: MemoryAdmissionContract, ChatSessionID: "session",
		SourceRevision: "revision", TurnIndex: 2,
		DerivationVersion: MemoryAdmissionContract, ExtractorVersion: "critic.v1",
		IndexVersion: MemoryVectorOutboxContract,
		ResultJSON:   `{"turn_summary":"second"}`,
	}
	admission.ResultHash = memoryAdmissionExpectedResultHash(admission)
	firstHash := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	firstJSON := `{"turn_summary":"first"}`
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT lifecycle_state, derived_admission_state").
		WithArgs("session", "revision", 2).
		WillReturnRows(sqlmock.NewRows([]string{
			"lifecycle_state", "derived_admission_state",
			"derived_admission_version", "derived_extractor_version",
			"derived_index_version", "derived_result_hash", "derived_result_json",
		}).AddRow("active", "committed", MemoryAdmissionContract, "critic.v1",
			MemoryVectorOutboxContract, firstHash, firstJSON))
	mock.ExpectCommit()
	got, err := st.CommitMemoryAdmission(context.Background(), admission)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Idempotent || got.CommittedResultHash != firstHash ||
		got.ExistingResultJSON != firstJSON {
		t.Fatalf("result=%+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBMemoryAdmissionRejectsStaleSourceBeforeAnyProjectionWrite(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	st := &mariadbStore{db: db}
	admission := &MemoryAdmission{
		ContractVersion: MemoryAdmissionContract, ChatSessionID: "session",
		SourceRevision: "old", TurnIndex: 2,
		DerivationVersion: MemoryAdmissionContract, ExtractorVersion: "critic.v1",
		IndexVersion: MemoryVectorOutboxContract,
		ResultJSON:   `{}`,
	}
	admission.ResultHash = memoryAdmissionExpectedResultHash(admission)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT lifecycle_state, derived_admission_state").
		WithArgs("session", "old", 2).
		WillReturnRows(sqlmock.NewRows([]string{
			"lifecycle_state", "derived_admission_state",
			"derived_admission_version", "derived_extractor_version",
			"derived_index_version", "derived_result_hash", "derived_result_json",
		}).AddRow("superseded", "pending", "", "", "", nil, nil))
	mock.ExpectRollback()
	if _, err := st.CommitMemoryAdmission(context.Background(), admission); !errors.Is(err, ErrSourceRevisionStale) {
		t.Fatalf("err=%v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
