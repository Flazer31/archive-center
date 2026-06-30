package store

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMariaDBStoreSaveStatusSchemaProposalPersistsProposalOnlyRecord(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	created := time.Date(2026, 6, 28, 10, 0, 0, 0, time.UTC)
	proposal := StatusSchemaProposal{
		ChatSessionID:  "sess-schema",
		InputChannel:   "portable_import",
		ProposalState:  "pending_review",
		SchemaName:     "dark-fantasy-status",
		RulesetLabel:   "Dark Fantasy",
		SchemaJSON:     `{"stats":[{"key":"custom_metric","kind":"number"}]}`,
		ProvenanceJSON: `{"source":"user_import"}`,
		CreatedAt:      created,
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO status_schema_proposals")).
		WithArgs(
			"sess-schema", "portable_import", "pending_review", "dark-fantasy-status", "Dark Fantasy",
			`{"stats":[{"key":"custom_metric","kind":"number"}]}`, `{"source":"user_import"}`, nil, nil, nil, created,
		).
		WillReturnResult(sqlmock.NewResult(24, 1))

	saved, err := m.SaveStatusSchemaProposal(context.Background(), proposal)
	if err != nil {
		t.Fatalf("SaveStatusSchemaProposal: %v", err)
	}
	if saved.ID != 24 || saved.ProposalState != "pending_review" || saved.CreatedAt != created || saved.UpdatedAt != created {
		t.Fatalf("unexpected saved proposal: %+v", saved)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBStoreListStatusSchemaProposalsScansReviewMetadata(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	created := time.Date(2026, 6, 28, 10, 10, 0, 0, time.UTC)
	reviewed := time.Date(2026, 6, 28, 10, 11, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"id", "chat_session_id", "input_channel", "proposal_state", "schema_name", "ruleset_label",
		"schema_json", "provenance_json", "review_note", "reviewer", "reviewed_at", "created_at", "updated_at",
	}).AddRow(
		int64(24), "sess-schema", "bootstrap", "approved", "status-core", "session-start",
		`{"stats":[]}`, `{"source":"llm_proposal"}`, "approved for registry step", "user", reviewed, created, reviewed,
	)
	mock.ExpectQuery("FROM status_schema_proposals").
		WithArgs("sess-schema", "approved", 50).
		WillReturnRows(rows)

	proposals, err := m.ListStatusSchemaProposals(context.Background(), "sess-schema", "approved", 50)
	if err != nil {
		t.Fatalf("ListStatusSchemaProposals: %v", err)
	}
	if len(proposals) != 1 {
		t.Fatalf("len(proposals)=%d", len(proposals))
	}
	got := proposals[0]
	if got.ID != 24 || got.InputChannel != "bootstrap" || got.ProposalState != "approved" || got.Reviewer != "user" || got.ReviewedAt.IsZero() {
		t.Fatalf("unexpected proposal: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBStoreGetStatusSchemaProposalHydratesVectorSourceRow(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	created := time.Date(2026, 6, 28, 10, 10, 0, 0, time.UTC)
	reviewed := time.Date(2026, 6, 28, 10, 11, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"id", "chat_session_id", "input_channel", "proposal_state", "schema_name", "ruleset_label",
		"schema_json", "provenance_json", "review_note", "reviewer", "reviewed_at", "created_at", "updated_at",
	}).AddRow(
		int64(24), "sess-schema", "direct_json", "approved", "status-core", "session-start",
		`{"stats":[{"key":"hp"}]}`, `{"source":"settings"}`, "approved for vector support", "user", reviewed, created, reviewed,
	)
	mock.ExpectQuery("FROM status_schema_proposals").
		WithArgs(int64(24)).
		WillReturnRows(rows)

	proposal, err := m.GetStatusSchemaProposal(context.Background(), 24)
	if err != nil {
		t.Fatalf("GetStatusSchemaProposal: %v", err)
	}
	if proposal.ID != 24 || proposal.ChatSessionID != "sess-schema" || proposal.ProposalState != "approved" || proposal.SchemaJSON == "" {
		t.Fatalf("unexpected proposal: %+v", proposal)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBStoreUpdateStatusSchemaProposalReviewRequiresState(t *testing.T) {
	m := &mariadbStore{db: &sql.DB{}}
	err := m.UpdateStatusSchemaProposalReview(context.Background(), 24, "", "", "")
	if err == nil || !strings.Contains(err.Error(), "proposal_state") {
		t.Fatalf("expected proposal_state error, got %v", err)
	}
	if errors.Is(err, ErrNotEnabled) {
		t.Fatalf("validation should run before db update: %v", err)
	}
}

func TestMariaDBStoreUpdateStatusSchemaProposalReviewPersistsReviewOnlyState(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	mock.ExpectExec("UPDATE status_schema_proposals").
		WithArgs("approved", "looks correct", "user", int64(24)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = m.UpdateStatusSchemaProposalReview(context.Background(), 24, "approved", "looks correct", "user")
	if err != nil {
		t.Fatalf("UpdateStatusSchemaProposalReview: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBStoreSaveStatusSchemaDefinitionsPersistsRegistryRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	created := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	defs := []StatusSchemaDefinition{
		{
			ChatSessionID:    "sess-schema",
			SourceProposalID: 24,
			SchemaName:       "combat-core",
			RulesetLabel:     "Combat",
			StatusKey:        "hp",
			Label:            "Health",
			OwnerScope:       "character",
			ValueKind:        "resource",
			BoundsJSON:       `{"min":0,"max":100}`,
			DefaultValueJSON: "100",
			RegistryState:    "active",
			CreatedAt:        created,
		},
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO status_schema_registry")).
		WithArgs(
			"sess-schema", int64(24), "combat-core", "Combat", "hp", "Health", "character", "resource",
			`{"min":0,"max":100}`, nil, "100", "active", created,
		).
		WillReturnResult(sqlmock.NewResult(100, 1))

	saved, err := m.SaveStatusSchemaDefinitions(context.Background(), defs)
	if err != nil {
		t.Fatalf("SaveStatusSchemaDefinitions: %v", err)
	}
	if len(saved) != 1 || saved[0].ID != 100 || saved[0].RegistryState != "active" {
		t.Fatalf("unexpected saved definitions: %+v", saved)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBStoreListStatusSchemaDefinitionsScansRegistryRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	created := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"id", "chat_session_id", "source_proposal_id", "schema_name", "ruleset_label",
		"status_key", "label", "owner_scope", "value_kind", "bounds_json", "options_json",
		"default_value_json", "registry_state", "created_at", "updated_at",
	}).AddRow(
		int64(100), "sess-schema", int64(24), "combat-core", "Combat",
		"hp", "Health", "character", "resource", `{"min":0,"max":100}`, nil,
		"100", "active", created, created,
	)
	mock.ExpectQuery("FROM status_schema_registry").
		WithArgs("sess-schema", "active", 50).
		WillReturnRows(rows)

	defs, err := m.ListStatusSchemaDefinitions(context.Background(), "sess-schema", "active", 50)
	if err != nil {
		t.Fatalf("ListStatusSchemaDefinitions: %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("len(defs)=%d", len(defs))
	}
	got := defs[0]
	if got.ID != 100 || got.SourceProposalID != 24 || got.StatusKey != "hp" || got.BoundsJSON == "" {
		t.Fatalf("unexpected definition: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBStoreGetStatusSchemaDefinitionByKeyRequiresActiveRegistry(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	created := time.Date(2026, 6, 28, 12, 0, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"id", "chat_session_id", "source_proposal_id", "schema_name", "ruleset_label",
		"status_key", "label", "owner_scope", "value_kind", "bounds_json", "options_json",
		"default_value_json", "registry_state", "created_at", "updated_at",
	}).AddRow(
		int64(100), "sess-schema", int64(24), "combat-core", "Combat",
		"hp", "Health", "character", "resource", `{"min":0,"max":100}`, nil,
		"100", "active", created, created,
	)
	mock.ExpectQuery("FROM status_schema_registry").
		WithArgs("sess-schema", "hp", "character").
		WillReturnRows(rows)

	def, err := m.GetStatusSchemaDefinitionByKey(context.Background(), "sess-schema", "hp", "character")
	if err != nil {
		t.Fatalf("GetStatusSchemaDefinitionByKey: %v", err)
	}
	if def.ID != 100 || def.StatusKey != "hp" || def.OwnerScope != "character" || def.RegistryState != "active" {
		t.Fatalf("unexpected definition: %+v", def)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBStoreSaveStatusCurrentValuePersistsEvidenceBoundCurrentValue(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	created := time.Date(2026, 6, 28, 13, 0, 0, 0, time.UTC)
	value := StatusCurrentValue{
		ChatSessionID: "sess-schema",
		RegistryID:    100,
		StatusKey:     "hp",
		OwnerScope:    "character",
		OwnerID:       "siwoo",
		OwnerLabel:    "이시우",
		ValueKind:     "resource",
		ValueJSON:     "75",
		EvidenceJSON:  `{"turn":2}`,
		SourceTurn:    2,
		WriteState:    "current",
		CreatedAt:     created,
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO status_current_values")).
		WithArgs(
			"sess-schema", int64(100), "hp", "character", "siwoo", "이시우",
			"resource", "75", `{"turn":2}`, 2, "current", created,
		).
		WillReturnResult(sqlmock.NewResult(200, 1))

	saved, err := m.SaveStatusCurrentValue(context.Background(), value)
	if err != nil {
		t.Fatalf("SaveStatusCurrentValue: %v", err)
	}
	if saved.ID != 200 || saved.RegistryID != 100 || saved.WriteState != "current" {
		t.Fatalf("unexpected current value: %+v", saved)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBStoreListStatusCurrentValuesScansCurrentRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	created := time.Date(2026, 6, 28, 13, 0, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"id", "chat_session_id", "registry_id", "status_key", "owner_scope", "owner_id",
		"owner_label", "value_kind", "value_json", "evidence_json", "source_turn",
		"write_state", "created_at", "updated_at",
	}).AddRow(
		int64(200), "sess-schema", int64(100), "hp", "character", "siwoo",
		"이시우", "resource", "75", `{"turn":2}`, 2,
		"current", created, created,
	)
	mock.ExpectQuery("FROM status_current_values").
		WithArgs("sess-schema", "character", "siwoo", "hp", 50).
		WillReturnRows(rows)

	values, err := m.ListStatusCurrentValues(context.Background(), "sess-schema", "character", "siwoo", "hp", 50)
	if err != nil {
		t.Fatalf("ListStatusCurrentValues: %v", err)
	}
	if len(values) != 1 || values[0].ID != 200 || values[0].StatusKey != "hp" || values[0].EvidenceJSON == "" {
		t.Fatalf("unexpected current values: %+v", values)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBStoreSaveStatusChangeEventPersistsLedgerRow(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	created := time.Date(2026, 6, 28, 14, 0, 0, 0, time.UTC)
	event := StatusChangeEvent{
		ChatSessionID:     "sess-schema",
		RegistryID:        100,
		StatusValueID:     200,
		StatusKey:         "hp",
		OwnerScope:        "character",
		OwnerID:           "siwoo",
		EventKind:         "decrease",
		PreviousValueJSON: "75",
		NewValueJSON:      "63",
		EvidenceJSON:      `{"turn":3}`,
		SourceTurn:        3,
		StoryClockJSON:    `{"day":2}`,
		EventState:        "recorded",
		CreatedAt:         created,
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO status_change_events")).
		WithArgs(
			"sess-schema", int64(100), int64(200), "hp", "character", "siwoo",
			"decrease", "75", "63", `{"turn":3}`, 3, `{"day":2}`, "recorded", created,
		).
		WillReturnResult(sqlmock.NewResult(300, 1))

	saved, err := m.SaveStatusChangeEvent(context.Background(), event)
	if err != nil {
		t.Fatalf("SaveStatusChangeEvent: %v", err)
	}
	if saved.ID != 300 || saved.EventKind != "decrease" || saved.EventState != "recorded" {
		t.Fatalf("unexpected event: %+v", saved)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBStoreListStatusChangeEventsScansLedgerRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	created := time.Date(2026, 6, 28, 14, 0, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"id", "chat_session_id", "registry_id", "status_value_id", "status_key", "owner_scope", "owner_id",
		"event_kind", "previous_value_json", "new_value_json", "evidence_json", "source_turn",
		"story_clock_json", "event_state", "created_at",
	}).AddRow(
		int64(300), "sess-schema", int64(100), int64(200), "hp", "character", "siwoo",
		"decrease", "75", "63", `{"turn":3}`, 3,
		`{"day":2}`, "recorded", created,
	)
	mock.ExpectQuery("FROM status_change_events").
		WithArgs("sess-schema", "character", "siwoo", "hp", 50).
		WillReturnRows(rows)

	events, err := m.ListStatusChangeEvents(context.Background(), "sess-schema", "character", "siwoo", "hp", 50)
	if err != nil {
		t.Fatalf("ListStatusChangeEvents: %v", err)
	}
	if len(events) != 1 || events[0].ID != 300 || events[0].NewValueJSON != "63" || events[0].StoryClockJSON == "" {
		t.Fatalf("unexpected events: %+v", events)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBStoreSaveStatusEffectPersistsLifecycleRow(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	created := time.Date(2026, 6, 28, 14, 10, 0, 0, time.UTC)
	effect := StatusEffect{
		ChatSessionID:     "sess-schema",
		RegistryID:        100,
		StatusKey:         "hp",
		OwnerScope:        "character",
		OwnerID:           "siwoo",
		EffectKind:        "cooldown",
		EffectLabel:       "dash cooldown",
		EffectPayloadJSON: `{"skill":"dash"}`,
		EvidenceJSON:      `{"turn":4}`,
		SourceTurn:        4,
		StartClockJSON:    `{"day":2}`,
		DurationJSON:      `{"turns":2}`,
		EffectState:       "active",
		CreatedAt:         created,
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO status_effects")).
		WithArgs(
			"sess-schema", int64(100), "hp", "character", "siwoo",
			"cooldown", "dash cooldown", `{"skill":"dash"}`, `{"turn":4}`, 4,
			`{"day":2}`, `{"turns":2}`, nil, "active", created,
		).
		WillReturnResult(sqlmock.NewResult(400, 1))

	saved, err := m.SaveStatusEffect(context.Background(), effect)
	if err != nil {
		t.Fatalf("SaveStatusEffect: %v", err)
	}
	if saved.ID != 400 || saved.EffectKind != "cooldown" || saved.EffectState != "active" {
		t.Fatalf("unexpected effect: %+v", saved)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBStoreUpdateStatusEffectStatePersistsClearEvidence(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	mock.ExpectExec("UPDATE status_effects").
		WithArgs("cleared", `{"turn":5}`, 5, int64(400)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := m.UpdateStatusEffectState(context.Background(), 400, "cleared", `{"turn":5}`, 5); err != nil {
		t.Fatalf("UpdateStatusEffectState: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
