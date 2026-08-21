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

func TestBindSessionRouteCreatesAndReadbacksOfficialIdentity(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	now := time.Now().UTC()

	mock.ExpectBegin()
	mock.ExpectQuery("(?s)FROM session_route_bindings.*FOR UPDATE").
		WithArgs("character-stable", "chat-opaque").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("(?s)FROM session_migration_locks.*LIMIT 1").
		WithArgs("session-canonical").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("(?s)INSERT INTO session_route_bindings").
		WithArgs(SessionRouteBindingContractVersion, "character-stable", "chat-opaque", "session-canonical",
			SessionRouteBindingModeResolveOrCreate, nil, nil).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("(?s)FROM session_route_bindings.*host_chat_id = \\?").
		WithArgs("character-stable", "chat-opaque").
		WillReturnRows(sessionRouteBindingRows(now).AddRow(
			SessionRouteBindingContractVersion, "character-stable", "chat-opaque",
			"session-canonical", "active", SessionRouteBindingModeResolveOrCreate,
			"", int64(0), uint64(1), now, now,
		))
	mock.ExpectCommit()

	result, err := m.BindSessionRoute(context.Background(), SessionRouteBindingRequest{
		StableCharacterID:  "character-stable",
		HostChatID:         "chat-opaque",
		RequestedSessionID: "session-canonical",
		Mode:               SessionRouteBindingModeResolveOrCreate,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Created || result.Updated || !result.ReadbackVerified || result.Binding.CanonicalSessionID != "session-canonical" {
		t.Fatalf("binding result = %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBindSessionRouteKeepsSameHostChatSeparateAcrossStableCharacters(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	now := time.Now().UTC()

	mock.ExpectBegin()
	mock.ExpectQuery("(?s)FROM session_route_bindings.*FOR UPDATE").
		WithArgs("different-character", "same-chat").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("(?s)FROM session_migration_locks.*LIMIT 1").
		WithArgs("different-session").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("(?s)INSERT INTO session_route_bindings").
		WithArgs(SessionRouteBindingContractVersion, "different-character", "same-chat", "different-session",
			SessionRouteBindingModeResolveOrCreate, nil, nil).
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectQuery("(?s)FROM session_route_bindings.*host_chat_id = \\?").
		WithArgs("different-character", "same-chat").
		WillReturnRows(sessionRouteBindingRows(now).AddRow(
			SessionRouteBindingContractVersion, "different-character", "same-chat",
			"different-session", "active", SessionRouteBindingModeResolveOrCreate,
			"", int64(0), uint64(1), now, now,
		))
	mock.ExpectCommit()

	result, err := m.BindSessionRoute(context.Background(), SessionRouteBindingRequest{
		StableCharacterID:  "different-character",
		HostChatID:         "same-chat",
		RequestedSessionID: "different-session",
	})
	if err != nil || result.Binding.CanonicalSessionID != "different-session" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBindSessionRouteRedirectsLockedSourceAndPersistsTarget(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	now := time.Now().UTC()

	mock.ExpectBegin()
	mock.ExpectQuery("(?s)FROM session_route_bindings.*FOR UPDATE").
		WithArgs("character-stable", "chat-opaque").
		WillReturnRows(sessionRouteBindingRows(now).AddRow(
			SessionRouteBindingContractVersion, "character-stable", "chat-opaque",
			"locked-source", "active", "existing_readback", "", int64(0), uint64(2), now, now,
		))
	mock.ExpectQuery("(?s)FROM session_migration_locks.*LIMIT 1").
		WithArgs("locked-source").
		WillReturnRows(sqlmock.NewRows([]string{"target_session_id", "migration_id", "lock_status"}).
			AddRow("canonical-target", int64(41), "migrated_away"))
	mock.ExpectQuery("(?s)FROM session_migration_locks.*LIMIT 1").
		WithArgs("canonical-target").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("(?s)UPDATE session_route_bindings").
		WithArgs(SessionRouteBindingContractVersion, "canonical-target", "locked_source_redirect",
			"locked-source", int64(41), "character-stable", "chat-opaque").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("(?s)FROM session_route_bindings.*host_chat_id = \\?").
		WithArgs("character-stable", "chat-opaque").
		WillReturnRows(sessionRouteBindingRows(now).AddRow(
			SessionRouteBindingContractVersion, "character-stable", "chat-opaque",
			"canonical-target", "active", "locked_source_redirect",
			"locked-source", int64(41), uint64(3), now, now,
		))
	mock.ExpectCommit()

	result, err := m.BindSessionRoute(context.Background(), SessionRouteBindingRequest{
		StableCharacterID: "character-stable",
		HostChatID:        "chat-opaque",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Updated || !result.LockedSourceRedirect || result.Binding.CanonicalSessionID != "canonical-target" {
		t.Fatalf("redirect result = %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBindSessionRouteBlocksPendingMigrationFenceWithoutRedirect(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	now := time.Now().UTC()

	mock.ExpectBegin()
	mock.ExpectQuery("(?s)FROM session_route_bindings.*FOR UPDATE").
		WithArgs("character-stable", "chat-opaque").
		WillReturnRows(sessionRouteBindingRows(now).AddRow(
			SessionRouteBindingContractVersion, "character-stable", "chat-opaque",
			"locked-source", "active", "existing_readback", "", int64(0), uint64(2), now, now,
		))
	mock.ExpectQuery("(?s)FROM session_migration_locks.*LIMIT 1").
		WithArgs("locked-source").
		WillReturnRows(sqlmock.NewRows([]string{"target_session_id", "migration_id", "lock_status"}).
			AddRow("canonical-target", int64(41), "lock_pending_verification"))
	mock.ExpectRollback()

	result, err := m.BindSessionRoute(context.Background(), SessionRouteBindingRequest{
		StableCharacterID: "character-stable",
		HostChatID:        "chat-opaque",
	})
	if result != nil {
		t.Fatalf("pending lock unexpectedly routed: %+v", result)
	}
	var blocker *SessionMigrationBlockerError
	if !errors.As(err, &blocker) || blocker.Code != "source_lock_verification_in_progress" {
		t.Fatalf("error = %v, want pending verification blocker", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBindSessionRouteReadbackMismatchRollsBack(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	now := time.Now().UTC()

	mock.ExpectBegin()
	mock.ExpectQuery("(?s)FROM session_route_bindings.*FOR UPDATE").
		WithArgs("character-stable", "chat-opaque").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery("(?s)FROM session_migration_locks.*LIMIT 1").
		WithArgs("requested-session").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO session_route_bindings")).
		WithArgs(SessionRouteBindingContractVersion, "character-stable", "chat-opaque", "requested-session",
			SessionRouteBindingModeResolveOrCreate, nil, nil).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectQuery("(?s)FROM session_route_bindings.*host_chat_id = \\?").
		WithArgs("character-stable", "chat-opaque").
		WillReturnRows(sessionRouteBindingRows(now).AddRow(
			SessionRouteBindingContractVersion, "character-stable", "chat-opaque",
			"wrong-session", "active", SessionRouteBindingModeResolveOrCreate,
			"", int64(0), uint64(1), now, now,
		))
	mock.ExpectRollback()

	result, err := m.BindSessionRoute(context.Background(), SessionRouteBindingRequest{
		StableCharacterID:  "character-stable",
		HostChatID:         "chat-opaque",
		RequestedSessionID: "requested-session",
	})
	if result != nil || err == nil || !strings.Contains(err.Error(), "session route binding readback mismatch") {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBindSessionRouteResolveExistingReturnsExactReadOnlyBinding(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	now := time.Now().UTC()
	mock.ExpectBegin()
	mock.ExpectQuery("(?s)FROM session_route_bindings.*FOR UPDATE").
		WithArgs("character-stable", "parent-chat").
		WillReturnRows(sessionRouteBindingRows(now).AddRow(
			SessionRouteBindingContractVersion, "character-stable", "parent-chat",
			"parent-session", "active", "existing_readback", "", int64(0), uint64(4), now, now,
		))
	mock.ExpectCommit()

	result, err := m.BindSessionRoute(context.Background(), SessionRouteBindingRequest{
		StableCharacterID: "character-stable",
		HostChatID:        "parent-chat",
		Mode:              SessionRouteBindingModeResolveExisting,
	})
	if err != nil || result == nil || !result.ReadbackVerified || result.Created || result.Updated || result.LockedSourceRedirect {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if result.Binding.CanonicalSessionID != "parent-session" {
		t.Fatalf("canonical session=%q", result.Binding.CanonicalSessionID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestBindSessionRouteResolveExistingNeverCreatesMissingParent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	mock.ExpectBegin()
	mock.ExpectQuery("(?s)FROM session_route_bindings.*FOR UPDATE").
		WithArgs("character-stable", "missing-parent-chat").
		WillReturnError(sql.ErrNoRows)
	mock.ExpectRollback()

	result, err := m.BindSessionRoute(context.Background(), SessionRouteBindingRequest{
		StableCharacterID: "character-stable",
		HostChatID:        "missing-parent-chat",
		Mode:              SessionRouteBindingModeResolveExisting,
	})
	if result != nil || !errors.Is(err, ErrNotFound) {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetWorldlineTopologySnapshotReadsBoundedFamilyAndLineageInOneTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	now := time.Date(2026, 8, 19, 1, 2, 3, 0, time.UTC)
	record := automaticForkLineageFixture(now)
	record.ChatSessionID = "selected-session"
	record.CopiedFromSessionID = "root-session"
	record.ForkSourceMessageID = "fork-message"
	record.ForkSourceRole = "char"
	record.IdempotencyKey = "risu-worldline:selected"

	mock.ExpectBegin()
	mock.ExpectQuery("(?s)SELECT DISTINCT stable_character_id.*canonical_session_id = \\?.*LIMIT 2").
		WithArgs("selected-session").
		WillReturnRows(sqlmock.NewRows([]string{"stable_character_id"}).AddRow("stable-character"))
	mock.ExpectQuery("(?s)SELECT canonical_session_id.*stable_character_id = \\?.*CASE WHEN canonical_session_id = \\?.*LIMIT \\?").
		WithArgs("stable-character", "selected-session", 4).
		WillReturnRows(sqlmock.NewRows([]string{"canonical_session_id"}).
			AddRow("selected-session").
			AddRow("root-session").
			AddRow("route-only-session"))
	mock.ExpectQuery("(?s)FROM session_fork_lineage.*chat_session_id IN \\(\\?,\\?,\\?\\).*ORDER BY chat_session_id ASC").
		WithArgs("selected-session", "root-session", "route-only-session").
		WillReturnRows(forkLineageRows().AddRow(forkLineageRowValues(record, 41, now)...))
	mock.ExpectQuery("(?s)FROM chat_logs.*turn_index > 0.*GROUP BY chat_session_id, turn_index.*HAVING MAX\\(CASE.*LOWER\\(TRIM\\(role\\)\\) = 'user'.*CHAR_LENGTH\\(TRIM\\(content\\)\\) > 0.*END\\) = 1.*LOWER\\(TRIM\\(role\\)\\) = 'assistant'.*CHAR_LENGTH\\(TRIM\\(content\\)\\) > 0.*END\\) = 1.*ORDER BY turn_index ASC, chat_session_id ASC.*LIMIT \\?").
		WithArgs("selected-session", "root-session", "route-only-session", worldlineTopologyCompletedTurnLimit+1).
		WillReturnRows(sqlmock.NewRows([]string{"chat_session_id", "turn_index"}).
			AddRow("root-session", 1).
			AddRow("selected-session", 3))
	mock.ExpectCommit()

	snapshot, err := m.GetWorldlineTopologySnapshot(context.Background(), "selected-session", 3)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.StableCharacterID != "stable-character" || snapshot.AnchorSessionID != "selected-session" || snapshot.Truncated {
		t.Fatalf("snapshot envelope=%+v", snapshot)
	}
	if strings.Join(snapshot.SessionIDs, ",") != "selected-session,root-session,route-only-session" {
		t.Fatalf("route family=%v", snapshot.SessionIDs)
	}
	if len(snapshot.LineageRecords) != 1 || snapshot.LineageRecords[0].ChatSessionID != "selected-session" || snapshot.LineageRecords[0].ForkSourceRole != "char" {
		t.Fatalf("lineage=%+v", snapshot.LineageRecords)
	}
	if len(snapshot.CompletedTurns) != 2 || snapshot.CompletedTurns[0].ChatSessionID != "root-session" || snapshot.CompletedTurns[0].TurnIndex != 1 || snapshot.CompletedTurns[1].ChatSessionID != "selected-session" || snapshot.CompletedTurns[1].TurnIndex != 3 || snapshot.TurnsTruncated {
		t.Fatalf("completed turns=%+v truncated=%v", snapshot.CompletedTurns, snapshot.TurnsTruncated)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetWorldlineTopologySnapshotSignalsFamilyTruncationBeforeLineageRead(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	now := time.Date(2026, 8, 19, 2, 3, 4, 0, time.UTC)
	record := automaticForkLineageFixture(now)
	record.ChatSessionID = "selected-session"
	record.CopiedFromSessionID = "z-omitted-parent"
	record.IdempotencyKey = "risu-worldline:truncated"

	mock.ExpectBegin()
	mock.ExpectQuery("(?s)SELECT DISTINCT stable_character_id.*canonical_session_id = \\?.*LIMIT 2").
		WithArgs("selected-session").
		WillReturnRows(sqlmock.NewRows([]string{"stable_character_id"}).AddRow("stable-character"))
	mock.ExpectQuery("(?s)SELECT canonical_session_id.*stable_character_id = \\?.*CASE WHEN canonical_session_id = \\?.*LIMIT \\?").
		WithArgs("stable-character", "selected-session", 3).
		WillReturnRows(sqlmock.NewRows([]string{"canonical_session_id"}).
			AddRow("selected-session").
			AddRow("a-route-only").
			AddRow("z-omitted-parent"))
	mock.ExpectQuery("(?s)FROM session_fork_lineage.*chat_session_id IN \\(\\?,\\?\\)").
		WithArgs("selected-session", "a-route-only").
		WillReturnRows(forkLineageRows().AddRow(forkLineageRowValues(record, 42, now)...))
	mock.ExpectQuery("(?s)FROM chat_logs.*chat_session_id IN \\(\\?,\\?\\).*turn_index > 0.*GROUP BY chat_session_id, turn_index.*HAVING MAX\\(CASE.*'user'.*END\\) = 1.*MAX\\(CASE.*'assistant'.*END\\) = 1.*LIMIT \\?").
		WithArgs("selected-session", "a-route-only", worldlineTopologyCompletedTurnLimit+1).
		WillReturnRows(sqlmock.NewRows([]string{"chat_session_id", "turn_index"}))
	mock.ExpectCommit()

	snapshot, err := m.GetWorldlineTopologySnapshot(context.Background(), "selected-session", 2)
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.Truncated || strings.Join(snapshot.SessionIDs, ",") != "selected-session,a-route-only" {
		t.Fatalf("truncated snapshot=%+v", snapshot)
	}
	if len(snapshot.LineageRecords) != 1 || snapshot.LineageRecords[0].CopiedFromSessionID != "z-omitted-parent" {
		t.Fatalf("bounded lineage=%+v", snapshot.LineageRecords)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetWorldlineTopologySnapshotSignalsCompletedTurnTruncationInSameTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	completedRows := sqlmock.NewRows([]string{"chat_session_id", "turn_index"})
	for turnIndex := 1; turnIndex <= worldlineTopologyCompletedTurnLimit+1; turnIndex++ {
		completedRows.AddRow("selected-session", turnIndex)
	}

	mock.ExpectBegin()
	mock.ExpectQuery("(?s)SELECT DISTINCT stable_character_id.*canonical_session_id = \\?.*LIMIT 2").
		WithArgs("selected-session").
		WillReturnRows(sqlmock.NewRows([]string{"stable_character_id"}).AddRow("stable-character"))
	mock.ExpectQuery("(?s)SELECT canonical_session_id.*stable_character_id = \\?.*LIMIT \\?").
		WithArgs("stable-character", "selected-session", 2).
		WillReturnRows(sqlmock.NewRows([]string{"canonical_session_id"}).AddRow("selected-session"))
	mock.ExpectQuery("(?s)FROM session_fork_lineage.*chat_session_id IN \\(\\?\\)").
		WithArgs("selected-session").
		WillReturnRows(forkLineageRows())
	mock.ExpectQuery("(?s)FROM chat_logs.*chat_session_id IN \\(\\?\\).*turn_index > 0.*GROUP BY chat_session_id, turn_index.*HAVING MAX\\(CASE.*'user'.*END\\) = 1.*MAX\\(CASE.*'assistant'.*END\\) = 1.*LIMIT \\?").
		WithArgs("selected-session", worldlineTopologyCompletedTurnLimit+1).
		WillReturnRows(completedRows)
	mock.ExpectCommit()

	snapshot, err := m.GetWorldlineTopologySnapshot(context.Background(), "selected-session", 1)
	if err != nil {
		t.Fatal(err)
	}
	if !snapshot.TurnsTruncated || len(snapshot.CompletedTurns) != worldlineTopologyCompletedTurnLimit || snapshot.CompletedTurns[len(snapshot.CompletedTurns)-1].TurnIndex != worldlineTopologyCompletedTurnLimit {
		t.Fatalf("completed turn cap snapshot=%+v count=%d", snapshot, len(snapshot.CompletedTurns))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func sessionRouteBindingRows(now time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"contract_version", "stable_character_id", "host_chat_id",
		"canonical_session_id", "binding_state", "binding_reason",
		"redirected_from_session_id", "redirect_migration_id",
		"revision", "created_at", "updated_at",
	})
}
