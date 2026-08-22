package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	mysql "github.com/go-sql-driver/mysql"
)

var reversibleStatusCurrentColumns = []string{
	"id", "chat_session_id", "registry_id", "status_key", "owner_scope", "owner_id",
	"owner_label", "value_kind", "value_json", "evidence_json", "source_turn",
	"write_state", "created_at", "updated_at",
}

var reversibleStatusEventColumns = []string{
	"id", "chat_session_id", "registry_id", "status_value_id", "status_key", "owner_scope", "owner_id",
	"event_kind", "previous_value_json", "new_value_json", "evidence_json", "source_turn",
	"story_clock_json", "event_state", "created_at",
}

func TestMariaDBStoreApplyReversibleStatusTransitionCommitsCurrentAndEventTogether(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	transition := reversibleStatusTransitionFixture()
	current := *transition.CurrentValue
	event := transition.Event

	mock.ExpectBegin()
	expectActiveReversibleSource(mock, event.ChatSessionID, transition.SourceRevision)
	expectNoReversibleStatusEvent(mock, event.ChatSessionID, transition.SourceRevision, transition.SourceUnitID)
	mock.ExpectQuery("SELECT source_turn").
		WithArgs(current.ChatSessionID, current.OwnerScope, current.OwnerID, current.StatusKey).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO status_current_values").
		WithArgs(
			current.ChatSessionID, current.RegistryID, current.StatusKey, current.OwnerScope, current.OwnerID,
			current.OwnerLabel, current.ValueKind, current.ValueJSON, current.EvidenceJSON,
			current.SourceTurn, current.WriteState, current.CreatedAt,
		).
		WillReturnResult(sqlmock.NewResult(101, 1))
	expectPriorActiveStatusEvent(mock, event, 88)
	mock.ExpectExec("INSERT INTO status_change_events").
		WithArgs(
			event.ChatSessionID, event.RegistryID, int64(101), event.StatusKey, event.OwnerScope, event.OwnerID,
			event.EventKind, event.PreviousValueJSON, event.NewValueJSON, event.EvidenceJSON,
			event.SourceTurn, event.StoryClockJSON, event.EventState, event.CreatedAt,
		).
		WillReturnResult(sqlmock.NewResult(202, 1))
	for i := 0; i < 3; i++ {
		mock.ExpectExec("INSERT INTO memory_derivation_dependencies").
			WillReturnResult(sqlmock.NewResult(int64(303+i), 1))
	}
	mock.ExpectCommit()

	got, err := m.ApplyReversibleStatusTransition(context.Background(), transition)
	if err != nil {
		t.Fatalf("ApplyReversibleStatusTransition: %v", err)
	}
	if got.Replayed {
		t.Fatal("new transition was reported as replayed")
	}
	if got.CurrentValue.ID != 101 || got.Event.ID != 202 || got.Event.StatusValueID != 101 {
		t.Fatalf("unexpected transition result: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBStoreApplyReversibleStatusTransitionRollsBackWhenEventInsertFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	transition := reversibleStatusTransitionFixture()
	current := *transition.CurrentValue
	event := transition.Event
	eventInsertErr := errors.New("event insert failed")

	mock.ExpectBegin()
	expectActiveReversibleSource(mock, event.ChatSessionID, transition.SourceRevision)
	expectNoReversibleStatusEvent(mock, event.ChatSessionID, transition.SourceRevision, transition.SourceUnitID)
	mock.ExpectQuery("SELECT source_turn").
		WithArgs(current.ChatSessionID, current.OwnerScope, current.OwnerID, current.StatusKey).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO status_current_values").
		WillReturnResult(sqlmock.NewResult(101, 1))
	expectNoPriorActiveStatusEvent(mock, event)
	mock.ExpectExec("INSERT INTO status_change_events").
		WillReturnError(eventInsertErr)
	mock.ExpectRollback()

	if _, err := m.ApplyReversibleStatusTransition(context.Background(), transition); !errors.Is(err, eventInsertErr) {
		t.Fatalf("ApplyReversibleStatusTransition error = %v, want event insert error", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBStoreApplyReversibleStatusTransitionExactReplayDoesNotWriteAgain(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	transition := reversibleStatusTransitionFixture()
	current := *transition.CurrentValue
	event := transition.Event

	mock.ExpectBegin()
	expectActiveReversibleSource(mock, event.ChatSessionID, transition.SourceRevision)
	mock.ExpectQuery("FROM status_change_events").
		WithArgs(event.ChatSessionID, transition.SourceRevision, transition.SourceUnitID).
		WillReturnRows(reversibleStatusEventRows(202, 101, event))
	mock.ExpectQuery("FROM status_current_values").
		WithArgs(current.ChatSessionID, current.OwnerScope, current.OwnerID, current.StatusKey).
		WillReturnRows(reversibleStatusCurrentRows(101, current))
	mock.ExpectCommit()

	got, err := m.ApplyReversibleStatusTransition(context.Background(), transition)
	if err != nil {
		t.Fatalf("ApplyReversibleStatusTransition: %v", err)
	}
	if !got.Replayed || got.Event.ID != 202 || got.CurrentValue.ID != 101 {
		t.Fatalf("unexpected replay result: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBStoreApplyReversibleStatusTransitionRejectsStaleSource(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	transition := reversibleStatusTransitionFixture()

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT lifecycle_state").
		WithArgs(transition.Event.ChatSessionID, transition.SourceRevision).
		WillReturnRows(sqlmock.NewRows([]string{"lifecycle_state"}).AddRow("superseded"))
	mock.ExpectRollback()

	if _, err := m.ApplyReversibleStatusTransition(context.Background(), transition); !errors.Is(err, ErrSourceRevisionStale) {
		t.Fatalf("ApplyReversibleStatusTransition error = %v, want ErrSourceRevisionStale", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBStoreApplyReversibleStatusTransitionRejectsStaleCurrentProjection(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	transition := reversibleStatusTransitionFixture()
	current := *transition.CurrentValue

	mock.ExpectBegin()
	expectActiveReversibleSource(mock, transition.Event.ChatSessionID, transition.SourceRevision)
	expectNoReversibleStatusEvent(mock, transition.Event.ChatSessionID, transition.SourceRevision, transition.SourceUnitID)
	mock.ExpectQuery("SELECT source_turn").
		WithArgs(current.ChatSessionID, current.OwnerScope, current.OwnerID, current.StatusKey).
		WillReturnRows(sqlmock.NewRows([]string{"source_turn"}).AddRow(current.SourceTurn + 1))
	mock.ExpectRollback()

	if _, err := m.ApplyReversibleStatusTransition(context.Background(), transition); !errors.Is(err, ErrStatusProjectionStale) {
		t.Fatalf("ApplyReversibleStatusTransition error = %v, want ErrStatusProjectionStale", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBStoreApplyReversibleStatusTransitionReportsLockFailureIdentity(t *testing.T) {
	tests := []struct {
		name     string
		number   uint16
		sqlState [5]byte
	}{
		{name: "deadlock", number: 1213, sqlState: [5]byte{'4', '0', '0', '0', '1'}},
		{name: "lock_wait_timeout", number: 1205, sqlState: [5]byte{'H', 'Y', '0', '0', '0'}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("sqlmock new: %v", err)
			}
			defer db.Close()

			transition := reversibleStatusTransitionFixture()
			mysqlErr := &mysql.MySQLError{
				Number:   tt.number,
				SQLState: tt.sqlState,
				Message:  "simulated lock failure",
			}
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT lifecycle_state").
				WithArgs(transition.Event.ChatSessionID, transition.SourceRevision).
				WillReturnError(mysqlErr)
			mock.ExpectRollback()

			_, gotErr := (&mariadbStore{db: db}).ApplyReversibleStatusTransition(context.Background(), transition)
			var gotMySQLError *mysql.MySQLError
			if !errors.As(gotErr, &gotMySQLError) || gotMySQLError.Number != tt.number {
				t.Fatalf("ApplyReversibleStatusTransition error = %v, want wrapped MySQL %d", gotErr, tt.number)
			}
			for _, want := range []string{
				fmt.Sprintf("mysql_error=%d", tt.number),
				"sql_state=" + string(tt.sqlState[:]),
				`source_revision="` + transition.SourceRevision + `"`,
				`source_unit_id="` + transition.SourceUnitID + `"`,
				`status_key="` + transition.Event.StatusKey + `"`,
				`owner_scope="` + transition.Event.OwnerScope + `"`,
				`owner_id="` + transition.Event.OwnerID + `"`,
			} {
				if !strings.Contains(gotErr.Error(), want) {
					t.Errorf("error %q does not contain %q", gotErr, want)
				}
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("unmet expectations: %v", err)
			}
		})
	}
}

func TestMariaDBStoreListReversibleStatusCurrentValuesReadsCompleteOwnerScopeWithoutLimit(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(requiredSQLMatcher(
		[]string{
			"from status_current_values",
			"join memory_source_revisions",
			"source_revision.lifecycle_state = 'active'",
			"write_state = 'current'",
			"owner_scope = ?",
			"status_key in (?,?)",
		},
		[]string{"limit"},
	)))
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	now := reversibleStatusTestTime()
	rows := sqlmock.NewRows(reversibleStatusCurrentColumns).
		AddRow(
			int64(101), "session-1", int64(7), "reversible_body_state", "fictional_entity", "entity-1",
			"Mina", "object", `{"slots":{"injury":"bruised"}}`, `{"source_revision":"revision-1"}`,
			3, "current", now, now,
		).
		AddRow(
			int64(102), "session-1", int64(8), "reversible_social_state", "fictional_entity", "entity-2",
			"Jin", "object", `{"slots":{"stance":"guarded"}}`, `{"source_revision":"revision-2"}`,
			4, "current", now, now,
		)
	mock.ExpectQuery("complete owner-scope current projection").
		WithArgs("session-1", "fictional_entity", "reversible_body_state", "reversible_social_state").
		WillReturnRows(rows)

	got, err := (&mariadbStore{db: db}).ListReversibleStatusCurrentValues(
		context.Background(),
		"session-1",
		"fictional_entity",
		[]string{"reversible_body_state", "reversible_social_state"},
	)
	if err != nil {
		t.Fatalf("ListReversibleStatusCurrentValues: %v", err)
	}
	if len(got) != 2 || got[0].OwnerID != "entity-1" || got[1].OwnerID != "entity-2" {
		t.Fatalf("unexpected complete owner-scope rows: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBStoreListLatestReversibleCurrentProjectionEventsUsesActiveLatestPerOwnerQuery(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(requiredSQLMatcher(
		[]string{
			"from status_change_events e",
			"join memory_source_revisions source_revision",
			"source_revision.lifecycle_state = 'active'",
			"not exists",
			"join memory_source_revisions newer_source",
			"newer_source.lifecycle_state = 'active'",
			"newer.owner_scope = e.owner_scope",
			"newer.owner_id = e.owner_id",
			"newer.source_turn > e.source_turn",
		},
		[]string{"limit"},
	)))
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	base := reversibleStatusTransitionFixture().Event
	other := base
	other.RegistryID = 8
	other.StatusKey = "reversible_social_state"
	other.OwnerID = "entity-2"
	other.SourceTurn = 4
	other.EvidenceJSON = `{"source_revision":"revision-2","source_unit_id":"unit-2","current_projection":true}`
	rows := reversibleStatusEventRows(202, 101, base).
		AddRow(
			int64(203), other.ChatSessionID, other.RegistryID, int64(102), other.StatusKey, other.OwnerScope, other.OwnerID,
			other.EventKind, other.PreviousValueJSON, other.NewValueJSON, other.EvidenceJSON, other.SourceTurn,
			other.StoryClockJSON, other.EventState, other.CreatedAt,
		)
	mock.ExpectQuery("active latest event per owner").
		WithArgs("session-1", "reversible_body_state", "reversible_social_state").
		WillReturnRows(rows)

	got, err := (&mariadbStore{db: db}).ListLatestReversibleCurrentProjectionEvents(
		context.Background(),
		"session-1",
		[]string{"reversible_body_state", "reversible_social_state"},
	)
	if err != nil {
		t.Fatalf("ListLatestReversibleCurrentProjectionEvents: %v", err)
	}
	if len(got) != 2 || got[0].OwnerID != "entity-1" || got[1].OwnerID != "entity-2" {
		t.Fatalf("unexpected latest owner events: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func reversibleStatusTransitionFixture() ReversibleStatusTransition {
	now := reversibleStatusTestTime()
	evidence := `{"source_revision":"revision-1","source_unit_id":"unit-1","current_projection":true,"direct_evidence_ids":[9]}`
	current := StatusCurrentValue{
		ChatSessionID: "session-1",
		RegistryID:    7,
		StatusKey:     "reversible_body_state",
		OwnerScope:    "fictional_entity",
		OwnerID:       "entity-1",
		OwnerLabel:    "Mina",
		ValueKind:     "object",
		ValueJSON:     `{"version":"reversible_state.v1","slots":{"injury":{"value":{"text":"bruised"}}}}`,
		EvidenceJSON:  evidence,
		SourceTurn:    3,
		WriteState:    "current",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	event := StatusChangeEvent{
		ChatSessionID:     current.ChatSessionID,
		RegistryID:        current.RegistryID,
		StatusKey:         current.StatusKey,
		OwnerScope:        current.OwnerScope,
		OwnerID:           current.OwnerID,
		EventKind:         "set",
		PreviousValueJSON: `{"version":"reversible_state.v1","slots":{}}`,
		NewValueJSON:      current.ValueJSON,
		EvidenceJSON:      evidence,
		SourceTurn:        current.SourceTurn,
		StoryClockJSON:    `{"scene":"night-watch"}`,
		EventState:        "recorded",
		CreatedAt:         now,
	}
	return ReversibleStatusTransition{
		SourceContract: acceptedSourceObservationContract,
		SourceRevision: "revision-1",
		SourceUnitID:   "unit-1",
		CurrentValue:   &current,
		Event:          event,
	}
}

func reversibleStatusTestTime() time.Time {
	return time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC)
}

func expectActiveReversibleSource(mock sqlmock.Sqlmock, chatSessionID, sourceRevision string) {
	mock.ExpectQuery("SELECT lifecycle_state").
		WithArgs(chatSessionID, sourceRevision).
		WillReturnRows(sqlmock.NewRows([]string{"lifecycle_state"}).AddRow("active"))
}

func expectNoReversibleStatusEvent(mock sqlmock.Sqlmock, chatSessionID, sourceRevision, sourceUnitID string) {
	mock.ExpectQuery("FROM status_change_events").
		WithArgs(chatSessionID, sourceRevision, sourceUnitID).
		WillReturnRows(sqlmock.NewRows(reversibleStatusEventColumns))
}

func expectNoPriorActiveStatusEvent(mock sqlmock.Sqlmock, event StatusChangeEvent) {
	mock.ExpectQuery("SELECT prior.id").
		WithArgs(event.ChatSessionID, event.StatusKey, event.OwnerScope, event.OwnerID).
		WillReturnError(sql.ErrNoRows)
}

func expectPriorActiveStatusEvent(mock sqlmock.Sqlmock, event StatusChangeEvent, id int64) {
	mock.ExpectQuery("SELECT prior.id").
		WithArgs(event.ChatSessionID, event.StatusKey, event.OwnerScope, event.OwnerID).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(id))
}

func reversibleStatusCurrentRows(id int64, current StatusCurrentValue) *sqlmock.Rows {
	return sqlmock.NewRows(reversibleStatusCurrentColumns).AddRow(
		id, current.ChatSessionID, current.RegistryID, current.StatusKey, current.OwnerScope, current.OwnerID,
		current.OwnerLabel, current.ValueKind, current.ValueJSON, current.EvidenceJSON, current.SourceTurn,
		current.WriteState, current.CreatedAt, current.UpdatedAt,
	)
}

func reversibleStatusEventRows(id, statusValueID int64, event StatusChangeEvent) *sqlmock.Rows {
	return sqlmock.NewRows(reversibleStatusEventColumns).AddRow(
		id, event.ChatSessionID, event.RegistryID, statusValueID, event.StatusKey, event.OwnerScope, event.OwnerID,
		event.EventKind, event.PreviousValueJSON, event.NewValueJSON, event.EvidenceJSON, event.SourceTurn,
		event.StoryClockJSON, event.EventState, event.CreatedAt,
	)
}

func requiredSQLMatcher(required, forbidden []string) sqlmock.QueryMatcher {
	return sqlmock.QueryMatcherFunc(func(_ string, actualSQL string) error {
		normalized := strings.Join(strings.Fields(strings.ToLower(actualSQL)), " ")
		for _, fragment := range required {
			if !strings.Contains(normalized, strings.ToLower(fragment)) {
				return fmt.Errorf("query does not contain required fragment %q: %s", fragment, normalized)
			}
		}
		for _, fragment := range forbidden {
			if strings.Contains(normalized, strings.ToLower(fragment)) {
				return fmt.Errorf("query contains forbidden fragment %q: %s", fragment, normalized)
			}
		}
		return nil
	})
}
