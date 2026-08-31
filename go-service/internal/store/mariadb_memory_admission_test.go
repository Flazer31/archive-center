package store

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
)

type readyOutboxDocumentWithoutContext struct{}

func (readyOutboxDocumentWithoutContext) Match(value driver.Value) bool {
	var raw []byte
	switch typed := value.(type) {
	case string:
		raw = []byte(typed)
	case []byte:
		raw = typed
	default:
		return false
	}
	var document struct {
		Embedding []float32      `json:"Embedding"`
		Metadata  map[string]any `json:"Metadata"`
	}
	if err := json.Unmarshal(raw, &document); err != nil || len(document.Embedding) == 0 {
		return false
	}
	_, hasInputs := document.Metadata["contextualized_embedding_inputs"]
	_, hasIndex := document.Metadata["contextualized_embedding_index"]
	return strings.TrimSpace(fmt.Sprint(document.Metadata["embedding_model"])) != "" && !hasInputs && !hasIndex
}

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
	mock.ExpectExec("SET derived_admission_state = 'pending'").
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
		LifecycleState: "active", VectorEmbedding: []float32{0.1, 0.2},
		VectorEmbeddingModel: "voyage-context-4", CreatedAt: now, UpdatedAt: now,
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
			Embedding: []float32{0.3, 0.4}, EmbeddingModel: "voyage-context-4",
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
		WithArgs(
			MemoryVectorOutboxContract, sqlmock.AnyArg(), "upsert", "session", "revision", sqlmock.AnyArg(),
			readyOutboxDocumentWithoutContext{}, true, "active", "pending", 0, nil, nil, nil, nil, now, now,
		).
		WillReturnResult(sqlmock.NewResult(51, 1))
	mock.ExpectExec("INSERT INTO memory_vector_outbox").
		WithArgs(
			MemoryVectorOutboxContract, sqlmock.AnyArg(), "upsert", "session", "revision", sqlmock.AnyArg(),
			readyOutboxDocumentWithoutContext{}, true, "active", "pending", 0, nil, nil, nil, nil, now, now,
		).
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

func TestReplayPrivateAggregateCancelsPendingUpsertAndQueuesDeleteWithoutFakeModel(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 8, 13, 1, 0, 0, 0, time.UTC)
	admission := &MemoryAdmission{
		ChatSessionID: "session", SourceRevision: "revision", TurnIndex: 3,
		ResultHash: strings.Repeat("a", 64), CreatedAt: now,
	}
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE memory_vector_outbox").
		WithArgs("no_public_memory_projection", now, "memory:session:17").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO memory_vector_outbox").
		WillReturnResult(sqlmock.NewResult(50, 1))
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	queued, err := reconcileAdmissionAggregateVectorEligibilityTx(
		context.Background(), tx, admission, 17, nil,
	)
	if err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	mock.ExpectCommit()
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if queued != 1 {
		t.Fatalf("queued=%d, want one active-source delete", queued)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAdmissionDeleteOperationKeyIgnoresResultHashAndSeparatesLifecycleFence(t *testing.T) {
	first := &MemoryAdmission{
		ChatSessionID: "session", SourceRevision: "revision",
		ResultHash: strings.Repeat("a", 64),
	}
	second := &MemoryAdmission{
		ChatSessionID: "session", SourceRevision: "revision",
		ResultHash: strings.Repeat("b", 64),
	}
	documentID := "memory:session:17"
	want := memoryVectorOperationKey("delete:active", "session", "revision", documentID)
	if got := memoryAdmissionVectorOperationKey("delete:active", first, documentID); got != want {
		t.Fatalf("active delete key=%q, want %q", got, want)
	}
	if got := memoryAdmissionVectorOperationKey("delete:active", second, documentID); got != want {
		t.Fatalf("result hash changed active delete key: key=%q want=%q", got, want)
	}
	inactive := memoryAdmissionVectorOperationKey("delete:inactive", first, documentID)
	if inactive == want {
		t.Fatal("active cleanup and inactive source invalidation must not share a delete key")
	}
	if memoryAdmissionVectorOperationKey("upsert", first, documentID) ==
		memoryAdmissionVectorOperationKey("upsert", second, documentID) {
		t.Fatal("upsert key must continue to distinguish result hashes")
	}
}

func TestAdmissionDeleteOperationKeysRemainBoundedAcross112TurnRegeneration(t *testing.T) {
	keys := map[string]struct{}{}
	for turn := 1; turn <= 112; turn++ {
		revision := fmt.Sprintf("revision-%03d", turn)
		documentID := fmt.Sprintf("memory:session:%d", turn)
		for cycle := range 4 {
			admission := &MemoryAdmission{
				ChatSessionID: "session", SourceRevision: revision,
				ResultHash: fmt.Sprintf("%064x", turn*10+cycle),
			}
			keys[memoryAdmissionVectorOperationKey("delete:active", admission, documentID)] = struct{}{}
			keys[memoryAdmissionVectorOperationKey("delete:inactive", admission, documentID)] = struct{}{}
		}
	}
	if len(keys) != 224 {
		t.Fatalf("delete operation keys=%d, want one per source revision, document, and lifecycle fence", len(keys))
	}
}

func TestReplayRetiredEvidenceCancelsPendingUpsertBeforeDelete(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 8, 13, 1, 10, 0, 0, time.UTC)
	admission := &MemoryAdmission{
		ChatSessionID: "session", SourceRevision: "revision", TurnIndex: 3,
		ResultHash: strings.Repeat("a", 64), CreatedAt: now,
	}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id, evidence_text, tombstoned").
		WithArgs("session", 3, 3).
		WillReturnRows(sqlmock.NewRows([]string{"id", "evidence_text", "tombstoned"}).
			AddRow(22, "Previously public evidence.", false))
	mock.ExpectExec("UPDATE direct_evidence_records").
		WithArgs(int64(22), "session").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE memory_vector_outbox").
		WithArgs("retired_evidence", now, "evidence:session:22").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO memory_vector_outbox").
		WillReturnResult(sqlmock.NewResult(50, 1))
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	byText, got, err := reconcileAdmissionEvidenceTx(context.Background(), tx, admission)
	if err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	mock.ExpectCommit()
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if len(byText) != 0 || got.retired != 1 || got.vectorOperations != 1 {
		t.Fatalf("byText=%#v result=%+v", byText, got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReplayPublicAggregateDoesNotDeleteEligibleMemory(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	admission := &MemoryAdmission{
		ChatSessionID: "session", SourceRevision: "revision", TurnIndex: 3,
		Vectors: []MemoryAdmissionVector{{ArtifactType: "memory"}},
	}
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	queued, err := reconcileAdmissionAggregateVectorEligibilityTx(
		context.Background(), tx, admission, 17, nil,
	)
	if err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	mock.ExpectCommit()
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if queued != 0 {
		t.Fatalf("queued=%d, want eligible public memory preserved", queued)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAdmissionVectorReplayReusesExactCompletedOperation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 8, 13, 2, 0, 0, 0, time.UTC)
	item := &MemoryVectorOutboxItem{
		OperationKey: strings.Repeat("d", 64), Operation: "upsert",
		ChatSessionID: "session", SourceRevision: "revision",
		DocumentID: "memory:session:17", DocumentJSON: `{"ID":"memory:session:17"}`,
		EmbeddingReady: true, RequiredSourceState: "active", Status: "pending",
		CreatedAt: now, UpdatedAt: now,
	}
	for replay := 0; replay < 2; replay++ {
		mock.ExpectQuery("SELECT o.id, o.operation").
			WithArgs(item.OperationKey).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "operation", "chat_session_id", "source_revision", "document_id",
				"required_source_state", "status", "lease_until", "lifecycle_state",
			}).AddRow(44, item.Operation, item.ChatSessionID, item.SourceRevision,
				item.DocumentID, item.RequiredSourceState, "completed", nil, "active"))
		mock.ExpectQuery("SELECT COUNT\\(\\*\\)").
			WithArgs(item.ChatSessionID, item.DocumentID, int64(44)).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectExec("UPDATE memory_vector_outbox").
			WithArgs(item.DocumentJSON, true, "pending", now, item.OperationKey, sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 1))
		inserted, err := enqueueAdmissionVectorOperation(
			WithMemoryAdmissionVectorReplay(context.Background(), true, false), db, item,
		)
		if err != nil || !inserted {
			t.Fatalf("replay %d inserted=%v err=%v", replay, inserted, err)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAdmissionDeleteReplayUsesOneExistingRowAcrossTwoForcePasses(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 8, 26, 3, 30, 0, 0, time.UTC)
	admission := &MemoryAdmission{
		ChatSessionID: "session", SourceRevision: "revision",
		ResultHash: strings.Repeat("a", 64), CreatedAt: now,
	}
	item := &MemoryVectorOutboxItem{
		OperationKey: memoryAdmissionVectorOperationKey("delete:active", admission, "memory:session:17"),
		Operation:    "delete", ChatSessionID: "session", SourceRevision: "revision",
		DocumentID: "memory:session:17", DocumentJSON: memoryVectorDeleteAuditJSON("reason-a"),
		EmbeddingReady: true, RequiredSourceState: "active", Status: "pending", UpdatedAt: now,
	}
	for replay := 0; replay < 2; replay++ {
		mock.ExpectQuery("SELECT o.id, o.operation").
			WithArgs(item.OperationKey).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "operation", "chat_session_id", "source_revision", "document_id",
				"required_source_state", "status", "lease_until", "lifecycle_state",
			}).AddRow(81, "delete", "session", "revision", "memory:session:17",
				"active", "completed", nil, "active"))
		mock.ExpectQuery("SELECT COUNT\\(\\*\\)").
			WithArgs("session", "memory:session:17", int64(81)).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectExec("UPDATE memory_vector_outbox").
			WithArgs(item.DocumentJSON, true, "pending", now, item.OperationKey, sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 1))
		inserted, err := enqueueAdmissionVectorOperation(
			WithMemoryAdmissionVectorReplay(context.Background(), true, true), db, item,
		)
		if err != nil || !inserted {
			t.Fatalf("force replay %d inserted=%v err=%v", replay, inserted, err)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAdmissionVectorReplayDoesNotStealActiveLease(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	item := &MemoryVectorOutboxItem{
		OperationKey: strings.Repeat("e", 64), Operation: "delete",
		ChatSessionID: "session", SourceRevision: "revision",
		DocumentID: "memory:session:17", RequiredSourceState: "active",
		Status: "pending", UpdatedAt: time.Now().UTC(),
	}
	mock.ExpectQuery("SELECT o.id, o.operation").
		WithArgs(item.OperationKey).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "operation", "chat_session_id", "source_revision", "document_id",
			"required_source_state", "status", "lease_until", "lifecycle_state",
		}).AddRow(45, item.Operation, item.ChatSessionID, item.SourceRevision,
			item.DocumentID, item.RequiredSourceState, "leased", time.Now().Add(time.Hour), "active"))
	inserted, err := enqueueAdmissionVectorOperation(
		WithMemoryAdmissionVectorReplay(context.Background(), true, false), db, item,
	)
	if inserted || !errors.Is(err, ErrMemoryReprocessingLeased) {
		t.Fatalf("inserted=%v err=%v, want active lease preserved", inserted, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAdmissionVectorReplayReclaimsExpiredLease(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Date(2026, 8, 13, 2, 10, 0, 0, time.UTC)
	item := &MemoryVectorOutboxItem{
		OperationKey: strings.Repeat("f", 64), Operation: "upsert",
		ChatSessionID: "session", SourceRevision: "revision",
		DocumentID: "evidence:session:21", DocumentJSON: `{"ID":"evidence:session:21"}`,
		RequiredSourceState: "active", Status: "needs_embedding", UpdatedAt: now,
	}
	mock.ExpectQuery("SELECT o.id, o.operation").
		WithArgs(item.OperationKey).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "operation", "chat_session_id", "source_revision", "document_id",
			"required_source_state", "status", "lease_until", "lifecycle_state",
		}).AddRow(46, item.Operation, item.ChatSessionID, item.SourceRevision,
			item.DocumentID, item.RequiredSourceState, "leased", time.Now().Add(-time.Hour), "active"))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\)").
		WithArgs(item.ChatSessionID, item.DocumentID, int64(46)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec("UPDATE memory_vector_outbox").
		WithArgs(item.DocumentJSON, false, "needs_embedding", now, item.OperationKey, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	inserted, err := enqueueAdmissionVectorOperation(
		WithMemoryAdmissionVectorReplay(context.Background(), true, false), db, item,
	)
	if err != nil || !inserted {
		t.Fatalf("inserted=%v err=%v, want expired lease reclaimed", inserted, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAdmissionVectorReplayRejectsSupersededOrUnchangedOperation(t *testing.T) {
	tests := []struct {
		name     string
		newer    int
		affected int64
	}{
		{name: "newer operation exists", newer: 1, affected: -1},
		{name: "guarded update affects no row", newer: 0, affected: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			now := time.Date(2026, 8, 13, 2, 20, 0, 0, time.UTC)
			item := &MemoryVectorOutboxItem{
				OperationKey: strings.Repeat("1", 64), Operation: "delete",
				ChatSessionID: "session", SourceRevision: "revision",
				DocumentID: "memory:session:17", RequiredSourceState: "active",
				Status: "pending", UpdatedAt: now,
			}
			mock.ExpectQuery("SELECT o.id, o.operation").
				WithArgs(item.OperationKey).
				WillReturnRows(sqlmock.NewRows([]string{
					"id", "operation", "chat_session_id", "source_revision", "document_id",
					"required_source_state", "status", "lease_until", "lifecycle_state",
				}).AddRow(47, item.Operation, item.ChatSessionID, item.SourceRevision,
					item.DocumentID, item.RequiredSourceState, "completed", nil, "active"))
			mock.ExpectQuery("SELECT COUNT\\(\\*\\)").
				WithArgs(item.ChatSessionID, item.DocumentID, int64(47)).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(tt.newer))
			if tt.affected >= 0 {
				mock.ExpectExec("UPDATE memory_vector_outbox").
					WithArgs(nil, false, "pending", now, item.OperationKey, sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, tt.affected))
			}
			inserted, err := enqueueAdmissionVectorOperation(
				WithMemoryAdmissionVectorReplay(context.Background(), true, false), db, item,
			)
			if inserted || err == nil {
				t.Fatalf("inserted=%v err=%v, want guarded replay failure", inserted, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
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
