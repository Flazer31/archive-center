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

func TestMariaDBPerspectiveReaderRequiresExactHolderAndIncludesLatestReviewBlocker(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	now := time.Unix(300, 0).UTC()
	mock.ExpectQuery(`FROM precise_memory_units unit[\s\S]+unit\.knowledge_holder_entity_id = \?[\s\S]+unit\.admission_state IN \('committed', 'review_required'\)[\s\S]+unit\.review_state IN \('source_observed', 'needs_review'\)[\s\S]+unit\.lifecycle_state = 'active'`).
		WithArgs("session", "holder-rowan").
		WillReturnRows(sqlmock.NewRows([]string{
			"unit_id", "chat_session_id", "source_turn_start", "source_turn_end",
			"source_revision", "memory_kind", "memory_subtype", "payload_json",
			"actor_entity_id", "subject_entity_id", "truth_scope", "epistemic_mode",
			"authority_class", "admission_state", "review_state", "visibility",
			"knowledge_holder_entity_id", "reveal_condition", "lifecycle_state",
			"created_at", "updated_at",
		}).AddRow(
			"unit-1", "session", 3, 3, "revision", "observation", "access",
			`{"contract_version":"perspective_memory.v1"}`, "speaker", "subject",
			"owner_scoped", "known", "subjective_episodic", "committed",
			"source_observed", "owner_private", "holder-rowan", "", "active", now, now,
		).AddRow(
			"unit-2", "session", 4, 4, "revision-2", "observation", "access",
			`{"contract_version":"perspective_memory.v1"}`, "speaker", "subject",
			"owner_scoped", "suspected", "subjective_episodic", "review_required",
			"needs_review", "owner_private", "holder-rowan", "", "active", now, now,
		))
	items, err := m.ListCharacterPerspectiveMemoryUnits(context.Background(), "session", "holder-rowan")
	if err != nil || len(items) != 2 ||
		items[0].KnowledgeHolderEntityID != "holder-rowan" ||
		items[1].AdmissionState != "review_required" {
		t.Fatalf("exact holder read items=%#v err=%v", items, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBActiveInteractionReaderReturnsOnlyCommittedActiveSourceUnits(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	now := time.Unix(350, 0).UTC()
	mock.ExpectQuery(`FROM precise_memory_units unit[\s\S]+unit\.memory_kind IN \('observation', 'boundary'\)[\s\S]+unit\.admission_state = 'committed'[\s\S]+unit\.review_state = 'source_observed'[\s\S]+unit\.lifecycle_state = 'active'`).
		WithArgs("session").
		WillReturnRows(sqlmock.NewRows([]string{
			"unit_id", "chat_session_id", "source_turn_start", "source_turn_end",
			"source_revision", "memory_kind", "memory_subtype", "payload_json",
			"actor_entity_id", "subject_entity_id", "affected_entity_id",
			"object_entity_id", "relationship_key", "truth_scope", "epistemic_mode",
			"authority_class", "admission_state", "review_state", "visibility",
			"knowledge_holder_entity_id", "reveal_condition", "lifecycle_state",
			"created_at", "updated_at",
		}).AddRow(
			"relation", "session", 2, 2, "revision-2", "observation", "relationship_trust",
			`{"contract_version":"relationship_observation.v1"}`,
			"alice-id", "", "bob-id", "", "alice-id->bob-id/trust",
			"source_scoped", "direct", "subjective_episodic", "committed",
			"source_observed", "public", "", "", "active", now, now,
		).AddRow(
			"boundary", "session", 3, 3, "revision-3", "boundary", "withdrawn",
			`{"contract_version":"interaction_boundary.v1"}`,
			"alice-id", "", "bob-id", "", "alice-id->bob-id/touch",
			"actor_scoped", "explicit_boundary", "subjective_episodic", "committed",
			"source_observed", "owner_private", "", "", "active", now, now,
		))
	items, err := m.ListActiveInteractionMemoryUnits(context.Background(), "session")
	if err != nil || len(items) != 2 || items[0].ActorEntityID != "alice-id" ||
		items[0].AffectedEntityID != "bob-id" || items[1].Kind != "boundary" {
		t.Fatalf("items=%#v err=%v", items, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPreciseMemoryPrivatePerspectiveSkipsGeneralVector(t *testing.T) {
	for _, item := range []*PreciseMemoryUnit{
		{Kind: "observation", Visibility: "owner_private", EpistemicMode: "known", KnowledgeHolderEntityID: "holder"},
		{Kind: "observation", Visibility: "hidden", EpistemicMode: "hidden"},
		{Kind: "observation", Visibility: "private", EpistemicMode: "misinformed"},
	} {
		if preciseMemoryGeneralVectorEligible(item) {
			t.Fatalf("private perspective became general-vector eligible: %+v", item)
		}
	}
	if !preciseMemoryGeneralVectorEligible(&PreciseMemoryUnit{Kind: "event", Visibility: "public", EpistemicMode: "direct"}) {
		t.Fatal("public objective event lost general-vector eligibility")
	}
}

func TestPreciseMemoryUserProfileSkipsGeneralVector(t *testing.T) {
	item := &PreciseMemoryUnit{
		Kind:           "profile",
		Subtype:        "user_interaction",
		Visibility:     "user_private",
		EpistemicMode:  "explicit_ooc_setting",
		AdmissionState: "committed",
		LifecycleState: "active",
	}
	if preciseMemoryGeneralVectorEligible(item) {
		t.Fatal("user interaction profile entered the general vector lane")
	}
}

func TestMariaDBPreciseMemoryRejectsStaleSourceRevision(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	unit := &PreciseMemoryUnit{ChatSessionID: "session", SourceRevision: "stale"}
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT lifecycle_state").
		WithArgs("session", "stale").
		WillReturnRows(sqlmock.NewRows([]string{"lifecycle_state"}))
	mock.ExpectRollback()
	if inserted, err := m.SavePreciseMemoryUnit(context.Background(), unit); inserted || !errors.Is(err, ErrSourceRevisionStale) {
		t.Fatalf("stale source inserted=%v err=%v", inserted, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
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
