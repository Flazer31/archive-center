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

func sessionRouteBindingRows(now time.Time) *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"contract_version", "stable_character_id", "host_chat_id",
		"canonical_session_id", "binding_state", "binding_reason",
		"redirected_from_session_id", "redirect_migration_id",
		"revision", "created_at", "updated_at",
	})
}
