package store

import (
	"context"
	"database/sql/driver"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
)

func expectCanonicalTailCleanup44(mock sqlmock.Sqlmock, sid string, turn int, legacyPhysicalCleanup, deleteTailChat bool, affected int64) {
	mock.ExpectExec(`(?s)INSERT INTO character_events.*SELECT state.chat_session_id.*manual_character_override.*FROM character_states state`).WithArgs(sid, turn).WillReturnResult(sqlmock.NewResult(0, 0))
	// Independent SQL contract: do not obtain this list from the production
	// command builder. A missing table, changed range/argument or reordered
	// dependency must be visible to this production transaction test.
	for _, command := range []struct {
		query string
		args  []driver.Value
	}{
		{`DELETE FROM effective_input_logs WHERE chat_session_id = ? AND turn_index >= ?`, []driver.Value{sid, turn}},
		{`DELETE FROM memories WHERE chat_session_id = ? AND turn_index >= ?`, []driver.Value{sid, turn}},
		{`DELETE FROM direct_evidence_records WHERE chat_session_id = ? AND source_turn_end >= ?`, []driver.Value{sid, turn}},
		{`DELETE FROM kg_triples WHERE chat_session_id = ? AND (source_turn >= ? OR valid_from >= ?)`, []driver.Value{sid, turn, turn}},
		{`DELETE FROM critic_feedback WHERE chat_session_id = ? AND target_type = 'turn' AND target_id >= ?`, []driver.Value{sid, turn}},
		{`DELETE FROM character_events WHERE chat_session_id = ? AND turn_index >= ?`, []driver.Value{sid, turn}},
		{`DELETE FROM speaker_attributions WHERE chat_session_id = ? AND source_turn >= ?`, []driver.Value{sid, turn}},
		{`DELETE FROM entity_identity_artifact_bindings WHERE chat_session_id = ? AND source_turn >= ?`, []driver.Value{sid, turn}},
		{`DELETE FROM entity_identity_surfaces WHERE chat_session_id = ? AND source_turn >= ?`, []driver.Value{sid, turn}},
		{`UPDATE entity_identities identity_row SET last_seen_turn = GREATEST(identity_row.first_seen_turn, COALESCE((SELECT MAX(surface.source_turn) FROM entity_identity_surfaces surface WHERE surface.chat_session_id = identity_row.chat_session_id AND surface.stable_entity_id = identity_row.stable_entity_id), identity_row.first_seen_turn)), updated_at = CURRENT_TIMESTAMP(3) WHERE identity_row.chat_session_id = ? AND identity_row.source_turn < ? AND identity_row.last_seen_turn >= ?`, []driver.Value{sid, turn, turn}},
		{`DELETE FROM entity_identity_links WHERE chat_session_id = ? AND (source_entity_id IN (SELECT stable_entity_id FROM entity_identities WHERE chat_session_id = ? AND source_turn >= ?) OR target_entity_id IN (SELECT stable_entity_id FROM entity_identities WHERE chat_session_id = ? AND source_turn >= ?))`, []driver.Value{sid, sid, turn, sid, turn}},
		{`DELETE FROM entity_identities WHERE chat_session_id = ? AND source_turn >= ?`, []driver.Value{sid, turn}},
		{`UPDATE entities SET last_seen_turn = ?, updated_at = CURRENT_TIMESTAMP(3) WHERE chat_session_id = ? AND (first_seen_turn IS NULL OR first_seen_turn < ?) AND last_seen_turn >= ?`, []driver.Value{turn - 1, sid, turn, turn}},
		{`DELETE FROM entities WHERE chat_session_id = ? AND first_seen_turn >= ?`, []driver.Value{sid, turn}},
		{`DELETE FROM trust_states WHERE chat_session_id = ? AND source_turn >= ?`, []driver.Value{sid, turn}},
		{`DELETE FROM storylines WHERE chat_session_id = ? AND (last_turn >= ? OR first_turn >= ?)`, []driver.Value{sid, turn, turn}},
		{`DELETE FROM world_rules WHERE chat_session_id = ? AND source_turn >= ?`, []driver.Value{sid, turn}},
		{`DELETE FROM character_states WHERE chat_session_id = ? AND turn_index >= ?`, []driver.Value{sid, turn}},
		{`DELETE FROM pending_threads WHERE chat_session_id = ? AND (source_turn >= ? OR created_turn >= ? OR resolved_turn >= ?)`, []driver.Value{sid, turn, turn, turn}},
		{`DELETE FROM active_states WHERE chat_session_id = ? AND turn_index >= ?`, []driver.Value{sid, turn}},
		{`DELETE FROM canonical_state_layers WHERE chat_session_id = ? AND turn_index >= ?`, []driver.Value{sid, turn}},
		{`DELETE FROM episode_summaries WHERE chat_session_id = ? AND (to_turn >= ? OR from_turn >= ?)`, []driver.Value{sid, turn, turn}},
		{`UPDATE guidance_plan_states SET story_plan_json = NULL, director_json = NULL, warnings_json = NULL, state_status = 'empty', last_turn = -1, updated_at = CURRENT_TIMESTAMP(3) WHERE chat_session_id = ? AND last_turn >= ?`, []driver.Value{sid, turn}},
		{`DELETE FROM chapter_summaries WHERE chat_session_id = ? AND (to_turn >= ? OR from_turn >= ?)`, []driver.Value{sid, turn, turn}},
		{`DELETE FROM arc_summaries WHERE chat_session_id = ? AND (to_turn >= ? OR from_turn >= ?)`, []driver.Value{sid, turn, turn}},
		{`DELETE FROM saga_digests WHERE chat_session_id = ? AND (to_turn >= ? OR from_turn >= ?)`, []driver.Value{sid, turn, turn}},
		{`DELETE FROM session_active_scopes WHERE chat_session_id = ?`, []driver.Value{sid}},
		{`DELETE FROM protagonist_entity_memories WHERE source_chat_session_id = ? AND source_turn_index >= ?`, []driver.Value{sid, turn}},
		{`DELETE FROM consequence_records WHERE chat_session_id = ? AND source_turn_end >= ?`, []driver.Value{sid, turn}},
		{`DELETE FROM psychology_branches WHERE chat_session_id = ? AND source_turn_end >= ?`, []driver.Value{sid, turn}},
		{`DELETE FROM theme_offscreen_carries WHERE chat_session_id = ? AND source_turn_end >= ?`, []driver.Value{sid, turn}},
		{`DELETE FROM capture_verification_records WHERE chat_session_id = ? AND turn_index >= ?`, []driver.Value{sid, turn}},
		{`DELETE FROM status_current_values WHERE chat_session_id = ? AND source_turn >= ?`, []driver.Value{sid, turn}},
		{`DELETE FROM status_change_events WHERE chat_session_id = ? AND source_turn >= ? AND NULLIF(TRIM(JSON_UNQUOTE(JSON_EXTRACT(evidence_json, '$.source_revision'))), '') IS NULL`, []driver.Value{sid, turn}},
		{`UPDATE status_effects SET effect_state = 'active', cleared_evidence_json = NULL, cleared_turn = NULL, updated_at = CURRENT_TIMESTAMP(3) WHERE chat_session_id = ? AND cleared_turn >= ?`, []driver.Value{sid, turn}},
		{`DELETE FROM status_effects WHERE chat_session_id = ? AND source_turn >= ?`, []driver.Value{sid, turn}},
		{`DELETE FROM chat_logs WHERE chat_session_id = ? AND turn_index >= ?`, []driver.Value{sid, turn}},
	} {
		if strings.HasPrefix(command.query, "DELETE FROM chat_logs") {
			if legacyPhysicalCleanup {
				mock.ExpectExec(regexp.QuoteMeta("DELETE FROM status_change_events WHERE chat_session_id = ? AND source_turn >= ?")).WithArgs(sid, turn).WillReturnResult(sqlmock.NewResult(0, affected))
			}
			if !deleteTailChat {
				command.query = "DELETE FROM chat_logs WHERE chat_session_id = ? AND turn_index = ?"
			}
		}
		mock.ExpectExec(regexp.QuoteMeta(command.query)).WithArgs(command.args...).WillReturnResult(sqlmock.NewResult(0, affected))
		if legacyPhysicalCleanup && strings.HasPrefix(command.query, "DELETE FROM effective_input_logs") {
			mock.ExpectExec(regexp.QuoteMeta("DELETE FROM precise_memory_units WHERE chat_session_id = ? AND source_turn_end >= ?")).WithArgs(sid, turn).WillReturnResult(sqlmock.NewResult(0, affected))
		}
	}
}

func TestMariaDBReplaceLogicalTurnAtomicallyReplacesCanonicalTail(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	created := time.Date(2026, 7, 22, 1, 2, 3, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT turn_index FROM chat_logs WHERE chat_session_id = ? ORDER BY turn_index DESC, id DESC LIMIT 1 FOR UPDATE")).
		WithArgs("session-1").
		WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(3))
	expectCanonicalTailCleanup44(mock, "session-1", 3, true, false, 1)
	mock.ExpectExec("INSERT INTO status_current_values").
		WithArgs("session-1").
		WillReturnResult(sqlmock.NewResult(0, 0))
	expectNarrativePendingSnapshots46(mock, "session-1")
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO chat_logs")).
		WithArgs("session-1", 3, "user text", created).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO chat_logs")).
		WithArgs("session-1", 3, "final assistant", created).
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()
	if err := m.ReplaceLogicalTurn(context.Background(), LogicalTurnReplacement{
		ChatSessionID: "session-1", TurnIndex: 3, UserContent: "user text",
		AssistantContent: "final assistant", CreatedAt: created,
	}); err != nil {
		t.Fatalf("ReplaceLogicalTurn: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBReplaceLogicalTurnRefusesHistoricalTurn(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT turn_index FROM chat_logs WHERE chat_session_id = ? ORDER BY turn_index DESC, id DESC LIMIT 1 FOR UPDATE")).
		WithArgs("session-1").
		WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(4))
	mock.ExpectRollback()
	err = m.ReplaceLogicalTurn(context.Background(), LogicalTurnReplacement{
		ChatSessionID: "session-1", TurnIndex: 3, UserContent: "user", AssistantContent: "assistant",
	})
	if err == nil {
		t.Fatal("historical logical turn replacement unexpectedly succeeded")
	}
	var typed *LogicalTurnReplacementError
	if !errors.As(err, &typed) || typed.Code != "logical_turn_not_current_tail" || typed.Retryable || typed.CommitState != "not_committed" {
		t.Fatalf("historical replacement error is not terminal and typed: %+v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLogicalTurnReplacementStoreErrorClassification(t *testing.T) {
	tests := []struct {
		name            string
		err             error
		commitAttempted bool
		code            string
		retryable       bool
		commitState     string
	}{
		{
			name: "permission",
			err:  &mysql.MySQLError{Number: 1142, Message: "command denied"},
			code: "logical_turn_db_permission_denied", commitState: "not_committed",
		},
		{
			name: "deadlock",
			err:  &mysql.MySQLError{Number: 1213, Message: "deadlock"},
			code: "logical_turn_transaction_temporarily_blocked", retryable: true, commitState: "not_committed",
		},
		{
			name: "commit unknown",
			err:  errors.New("connection lost during commit"), commitAttempted: true,
			code: "logical_turn_commit_outcome_unknown", commitState: "unknown",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := classifyLogicalTurnReplacementStoreError(tc.err, "test_stage", tc.commitAttempted)
			var typed *LogicalTurnReplacementError
			if !errors.As(err, &typed) {
				t.Fatalf("error is not typed: %v", err)
			}
			if typed.Code != tc.code || typed.Retryable != tc.retryable || typed.CommitState != tc.commitState {
				t.Fatalf("unexpected classification: %+v", typed)
			}
		})
	}
}

func TestMariaDBReplaceLogicalTurnRecreatesDeletedImmediateTail(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	created := time.Date(2026, 7, 22, 2, 25, 44, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT turn_index FROM chat_logs WHERE chat_session_id = ? ORDER BY turn_index DESC, id DESC LIMIT 1 FOR UPDATE")).
		WithArgs("session-1").
		WillReturnRows(sqlmock.NewRows([]string{"max"}).AddRow(14))
	expectCanonicalTailCleanup44(mock, "session-1", 15, true, false, 1)
	mock.ExpectExec("INSERT INTO status_current_values").
		WithArgs("session-1").
		WillReturnResult(sqlmock.NewResult(0, 0))
	expectNarrativePendingSnapshots46(mock, "session-1")
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO chat_logs")).
		WithArgs("session-1", 15, "user text", created).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO chat_logs")).
		WithArgs("session-1", 15, "regenerated assistant", created).
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()

	if err := m.ReplaceLogicalTurn(context.Background(), LogicalTurnReplacement{
		ChatSessionID: "session-1", TurnIndex: 15, UserContent: "user text",
		AssistantContent: "regenerated assistant", CreatedAt: created,
	}); err != nil {
		t.Fatalf("ReplaceLogicalTurn deleted immediate tail: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBReplaceLogicalTurnRecreatesDeletedFirstTurnInEmptySession(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	created := time.Date(2026, 7, 31, 3, 8, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT turn_index FROM chat_logs WHERE chat_session_id = ? ORDER BY turn_index DESC, id DESC LIMIT 1 FOR UPDATE")).
		WithArgs("session-empty").
		WillReturnRows(sqlmock.NewRows([]string{"turn_index"}))
	expectCanonicalTailCleanup44(mock, "session-empty", 1, true, false, 0)
	mock.ExpectExec("INSERT INTO status_current_values").
		WithArgs("session-empty").
		WillReturnResult(sqlmock.NewResult(0, 0))
	expectNarrativePendingSnapshots46(mock, "session-empty")
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO chat_logs")).
		WithArgs("session-empty", 1, "first user", created).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO chat_logs")).
		WithArgs("session-empty", 1, "regenerated first assistant", created).
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectCommit()

	if err := m.ReplaceLogicalTurn(context.Background(), LogicalTurnReplacement{
		ChatSessionID: "session-empty", TurnIndex: 1, UserContent: "first user",
		AssistantContent: "regenerated first assistant", CreatedAt: created,
	}); err != nil {
		t.Fatalf("ReplaceLogicalTurn empty first turn: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBRollbackCanonicalTailIsAtomicAndIdempotent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	rollback := LogicalTurnRollback{ChatSessionID: "session-1", TurnIndex: 4, Reason: "turn_rollback", CreatedAt: now}

	for range 2 {
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta("SELECT turn_index FROM chat_logs WHERE chat_session_id = ? ORDER BY turn_index DESC, id DESC LIMIT 1 FOR UPDATE")).
			WithArgs("session-1").
			WillReturnRows(sqlmock.NewRows([]string{"turn_index"}).AddRow(3))
		mock.ExpectQuery("SELECT source_revision").
			WithArgs("session-1", 4).
			WillReturnRows(sqlmock.NewRows([]string{"source_revision"}))
		expectCanonicalTailCleanup44(mock, "session-1", 4, false, true, 0)
		mock.ExpectExec("(?s)INSERT INTO status_current_values.*JOIN memory_source_revisions source_revision.*source_revision.lifecycle_state = 'active'.*NOT EXISTS").
			WithArgs("session-1").
			WillReturnResult(sqlmock.NewResult(0, 0))
		expectNarrativePendingSnapshots46(mock, "session-1")
		mock.ExpectCommit()
	}

	if err := m.RollbackCanonicalTail(context.Background(), rollback); err != nil {
		t.Fatalf("first rollback: %v", err)
	}
	if err := m.RollbackCanonicalTail(context.Background(), rollback); err != nil {
		t.Fatalf("idempotent replay: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBRollbackCanonicalTailRollsBackOnDerivedDeleteFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	failure := errors.New("derived delete failed")
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT turn_index FROM chat_logs WHERE chat_session_id = ? ORDER BY turn_index DESC, id DESC LIMIT 1 FOR UPDATE")).
		WithArgs("session-1").
		WillReturnRows(sqlmock.NewRows([]string{"turn_index"}).AddRow(4))
	mock.ExpectQuery("SELECT source_revision").
		WithArgs("session-1", 4).
		WillReturnRows(sqlmock.NewRows([]string{"source_revision"}))
	mock.ExpectExec(`(?s)INSERT INTO character_events.*SELECT state.chat_session_id`).WithArgs("session-1", 4).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("DELETE FROM effective_input_logs").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM memories").WillReturnError(failure)
	mock.ExpectRollback()

	err = m.RollbackCanonicalTail(context.Background(), LogicalTurnRollback{
		ChatSessionID: "session-1", TurnIndex: 4, Reason: "turn_rollback",
	})
	if err == nil || !strings.Contains(err.Error(), failure.Error()) {
		t.Fatalf("rollback error = %v, want derived failure", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCanonicalTailLifecycleCleanupDeletesDirectEvidenceButPreservesLifecycleHistory(t *testing.T) {
	queries := []string{}
	for _, command := range canonicalTailDeleteCommands("session-1", 4, false, true) {
		queries = append(queries, strings.Join(strings.Fields(command.query), " "))
	}
	joined := strings.Join(queries, "\n")
	for _, required := range []string{
		"DELETE FROM direct_evidence_records",
		"DELETE FROM status_current_values",
		"DELETE FROM status_change_events",
		"JSON_EXTRACT(evidence_json, '$.source_revision')",
	} {
		if !strings.Contains(joined, required) {
			t.Fatalf("lifecycle cleanup missing %q:\n%s", required, joined)
		}
	}
	for _, forbidden := range []string{
		"DELETE FROM precise_memory_units",
		"UPDATE direct_evidence_records",
	} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("lifecycle cleanup destroys invalidated history via %q:\n%s", forbidden, joined)
		}
	}

	legacyQueries := []string{}
	for _, command := range canonicalTailDeleteCommands("session-1", 4, true, true) {
		legacyQueries = append(legacyQueries, strings.Join(strings.Fields(command.query), " "))
	}
	legacy := strings.Join(legacyQueries, "\n")
	for _, required := range []string{
		"DELETE FROM precise_memory_units",
		"DELETE FROM direct_evidence_records",
		"DELETE FROM status_change_events",
	} {
		if !strings.Contains(legacy, required) {
			t.Fatalf("legacy cleanup missing compatibility delete %q", required)
		}
	}
}

func TestCanonicalTailCleanupDeletesEveryOverlappingHierarchySummary(t *testing.T) {
	queries := map[string]canonicalTailDeleteCommand{}
	for _, command := range canonicalTailDeleteCommands("session-1", 4, false, true) {
		normalized := strings.Join(strings.Fields(command.query), " ")
		for _, table := range []string{"episode_summaries", "chapter_summaries", "arc_summaries", "saga_digests"} {
			if strings.Contains(normalized, "DELETE FROM "+table) {
				queries[table] = command
			}
		}
	}
	for _, table := range []string{"episode_summaries", "chapter_summaries", "arc_summaries", "saga_digests"} {
		command, ok := queries[table]
		if !ok {
			t.Fatalf("hierarchy cleanup missing %s", table)
		}
		normalized := strings.Join(strings.Fields(command.query), " ")
		if !strings.Contains(normalized, "(to_turn >= ? OR from_turn >= ?)") {
			t.Fatalf("%s cleanup does not delete ranges overlapping the rollback turn: %s", table, normalized)
		}
		if len(command.args) != 3 || command.args[0] != "session-1" || command.args[1] != 4 || command.args[2] != 4 {
			t.Fatalf("%s cleanup args = %#v, want session and rollback turn twice", table, command.args)
		}
	}
}
