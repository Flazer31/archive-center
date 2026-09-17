package store

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func newLorebookReferenceMock(t *testing.T) (*mariadbStore, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return &mariadbStore{db: db}, mock
}

func expectLorebookReferenceSessionLock(mock sqlmock.Sqlmock, chatSessionID string) {
	mock.ExpectExec("INSERT INTO lorebook_reference_session_locks").
		WithArgs(chatSessionID).
		WillReturnResult(sqlmock.NewResult(0, 1))
}

func expectLatestLorebookReferenceAuthorityObservation(mock sqlmock.Sqlmock, scopeID int64, observedAt time.Time) {
	mock.ExpectQuery("(?s)FROM lorebook_reference_snapshots.*complete_snapshot").
		WithArgs(scopeID, LorebookConsentRevoked, LorebookConsentActive, LorebookObservationObserved).
		WillReturnRows(sqlmock.NewRows([]string{"observed_at"}).AddRow(observedAt))
}

func TestApplyLorebookReferenceCompleteSnapshotReplacesOnlyExactScope(t *testing.T) {
	maria, mock := newLorebookReferenceMock(t)
	characterIndex := int64(2)
	chatIndex := int64(7)
	observedAt := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	scope := LorebookReferenceScope{
		ChatSessionID: "session-a", CharacterIndex: &characterIndex, ChatIndex: &chatIndex,
		EnabledModuleIDs: []string{"mod-b", "mod-a", "mod-a"}, EnabledModulesObserved: true,
	}
	modulesJSON, identityJSON, err := lorebookScopeJSON(scope)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectBegin()
	expectLorebookReferenceSessionLock(mock, "session-a")
	mock.ExpectQuery("FROM lorebook_reference_scopes").
		WithArgs("session-a", int64(2), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"scope_id", "enabled_modules_json", "scope_identity_json", "created_at", "updated_at"}))
	mock.ExpectExec("INSERT INTO lorebook_reference_scopes").
		WithArgs("session-a", int64(2), int64(7), modulesJSON, identityJSON).
		WillReturnResult(sqlmock.NewResult(11, 1))
	mock.ExpectExec("INSERT INTO lorebook_reference_snapshots").
		WithArgs("0123456789abcdef0123456789abcdef", int64(11), LorebookReferenceSnapshotContractV1,
			LorebookConsentActive, LorebookObservationObserved, true, 1, `{"source":"test"}`, observedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE lorebook_reference_entries").
		WithArgs(LorebookLifecycleStale, observedAt, int64(11)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	wantHash := lorebookDiagnosticContentHash("Han-eol will take the exam.")
	mock.ExpectExec("INSERT INTO lorebook_reference_entries").
		WithArgs(int64(11), "0123456789abcdef0123456789abcdef", "entry-1", 0,
			"current_host_aggregate", nil, "Han-eol", "exam", "profile", "Han-eol will take the exam.",
			"han-eol exam profile han-eol will take the exam.", "normal", false, nil, nil, 0, nil, int64(3),
			"cast", `{"risu_case_sensitive":false}`, wantHash, LorebookLifecycleCurrent, true, observedAt, observedAt).
		WillReturnResult(sqlmock.NewResult(41, 1))
	mock.ExpectCommit()

	alwaysActive := false
	insertOrder := 0
	bookVersion := int64(3)
	result, err := maria.ApplyLorebookReferenceSnapshot(context.Background(), &LorebookReferenceSnapshot{
		SnapshotID: "0123456789abcdef0123456789abcdef", ContractVersion: LorebookReferenceSnapshotContractV1,
		ConsentState: LorebookConsentActive, ObservationState: LorebookObservationObserved, CompleteSnapshot: true,
		Scope: scope, ProvenanceJSON: `{"source":"test"}`, ObservedAt: observedAt,
		Entries: []LorebookReferenceEntryObservation{{
			HostEntryID: "entry-1", SourceKind: "current_host_aggregate", Key: "Han-eol", SecondKey: "exam",
			Comment: "profile", Content: "Han-eol will take the exam.", Mode: "normal",
			AlwaysActive: &alwaysActive, InsertOrder: &insertOrder, BookVersion: &bookVersion,
			Folder: "cast", ExtensionsJSON: `{"risu_case_sensitive":false}`,
		}},
	})
	if err != nil {
		t.Fatalf("ApplyLorebookReferenceSnapshot: %v", err)
	}
	if result.ScopeID != 11 || result.PreviousCurrentCount != 2 || result.CurrentEntryCount != 1 || result.LifecycleAction != "current_projection_replaced" {
		t.Fatalf("result=%#v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyLorebookReferencePartialSnapshotKeepsLastConfirmedProjection(t *testing.T) {
	maria, mock := newLorebookReferenceMock(t)
	observedAt := time.Date(2026, 8, 12, 10, 5, 0, 0, time.UTC)
	scope := LorebookReferenceScope{ChatSessionID: "session-a", EnabledModuleIDs: []string{}, EnabledModulesObserved: false}
	_, identityJSON, _ := lorebookScopeJSON(scope)
	mock.ExpectBegin()
	expectLorebookReferenceSessionLock(mock, "session-a")
	mock.ExpectQuery("FROM lorebook_reference_scopes").
		WithArgs("session-a", nil, nil).
		WillReturnRows(sqlmock.NewRows([]string{"scope_id", "enabled_modules_json", "scope_identity_json", "created_at", "updated_at"}).
			AddRow(12, `[]`, identityJSON, observedAt, observedAt))
	mock.ExpectExec("INSERT INTO lorebook_reference_snapshots").
		WithArgs("fedcba9876543210fedcba9876543210", int64(12), LorebookReferenceSnapshotContractV1,
			LorebookConsentActive, LorebookObservationPartial, false, 1, `{}`, observedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO lorebook_reference_entries").
		WithArgs(int64(12), "fedcba9876543210fedcba9876543210", nil, 0,
			"current_host_aggregate", nil, "partial", "", "", "partial body", "partial partial body",
			nil, nil, nil, nil, nil, nil, nil, nil, `{}`, lorebookDiagnosticContentHash("partial body"),
			LorebookLifecyclePartial, false, observedAt, observedAt).
		WillReturnResult(sqlmock.NewResult(51, 1))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM lorebook_reference_entries`).
		WithArgs(int64(12)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
	mock.ExpectCommit()

	result, err := maria.ApplyLorebookReferenceSnapshot(context.Background(), &LorebookReferenceSnapshot{
		SnapshotID: "fedcba9876543210fedcba9876543210", ContractVersion: LorebookReferenceSnapshotContractV1,
		ConsentState: LorebookConsentActive, ObservationState: LorebookObservationPartial,
		Scope: scope, ProvenanceJSON: `{}`, ObservedAt: observedAt,
		Entries: []LorebookReferenceEntryObservation{{Key: "partial", Content: "partial body"}},
	})
	if err != nil {
		t.Fatalf("ApplyLorebookReferenceSnapshot: %v", err)
	}
	if result.PreviousCurrentCount != 0 || result.CurrentEntryCount != 3 || result.LifecycleAction != "observation_recorded" {
		t.Fatalf("partial observation changed current projection: %#v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateLorebookReferenceUnavailableCannotPretendToBeComplete(t *testing.T) {
	err := ValidateLorebookReferenceSnapshot(&LorebookReferenceSnapshot{
		SnapshotID: "snapshot", ContractVersion: LorebookReferenceSnapshotContractV1,
		ConsentState: LorebookConsentActive, ObservationState: LorebookObservationUnavailable,
		CompleteSnapshot: true, Scope: LorebookReferenceScope{ChatSessionID: "session-a"},
	})
	if err != ErrInvalidLorebookReference {
		t.Fatalf("err=%v", err)
	}
}

func TestValidateLorebookReferenceCompleteActiveSnapshotRequiresFullyObservedScope(t *testing.T) {
	characterIndex := int64(1)
	chatIndex := int64(2)
	tests := []LorebookReferenceScope{
		{ChatSessionID: "session-a", ChatIndex: &chatIndex, EnabledModulesObserved: true},
		{ChatSessionID: "session-a", CharacterIndex: &characterIndex, EnabledModulesObserved: true},
		{ChatSessionID: "session-a", CharacterIndex: &characterIndex, ChatIndex: &chatIndex, EnabledModulesObserved: false},
	}
	for _, scope := range tests {
		err := ValidateLorebookReferenceSnapshot(&LorebookReferenceSnapshot{
			SnapshotID: "snapshot", ContractVersion: LorebookReferenceSnapshotContractV1,
			ConsentState: LorebookConsentActive, ObservationState: LorebookObservationObserved,
			CompleteSnapshot: true, Scope: scope,
		})
		if err != ErrInvalidLorebookReference {
			t.Fatalf("incomplete scope was accepted: scope=%#v err=%v", scope, err)
		}
	}
}

func TestApplyLorebookReferenceUnavailableObservationKeepsLastConfirmedProjection(t *testing.T) {
	maria, mock := newLorebookReferenceMock(t)
	observedAt := time.Date(2026, 8, 12, 10, 10, 0, 0, time.UTC)
	scope := LorebookReferenceScope{ChatSessionID: "session-a", EnabledModuleIDs: []string{"module-a"}, EnabledModulesObserved: true}
	_, identityJSON, _ := lorebookScopeJSON(scope)
	mock.ExpectBegin()
	expectLorebookReferenceSessionLock(mock, "session-a")
	mock.ExpectQuery("FROM lorebook_reference_scopes").
		WithArgs("session-a", nil, nil).
		WillReturnRows(sqlmock.NewRows([]string{"scope_id", "enabled_modules_json", "scope_identity_json", "created_at", "updated_at"}).
			AddRow(13, `["module-a"]`, identityJSON, observedAt, observedAt))
	mock.ExpectExec("INSERT INTO lorebook_reference_snapshots").
		WithArgs("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", int64(13), LorebookReferenceSnapshotContractV1,
			LorebookConsentActive, LorebookObservationUnavailable, false, 0, `{}`, observedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM lorebook_reference_entries`).
		WithArgs(int64(13)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(4))
	mock.ExpectCommit()

	result, err := maria.ApplyLorebookReferenceSnapshot(context.Background(), &LorebookReferenceSnapshot{
		SnapshotID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", ContractVersion: LorebookReferenceSnapshotContractV1,
		ConsentState: LorebookConsentActive, ObservationState: LorebookObservationUnavailable,
		Scope: scope, ProvenanceJSON: `{}`, ObservedAt: observedAt,
	})
	if err != nil {
		t.Fatalf("ApplyLorebookReferenceSnapshot: %v", err)
	}
	if result.CurrentEntryCount != 4 || result.PreviousCurrentCount != 0 || result.LifecycleAction != "observation_recorded" {
		t.Fatalf("unavailable observation changed current projection: %#v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyLorebookReferenceConsentRevocationDisablesOnlyExactScope(t *testing.T) {
	maria, mock := newLorebookReferenceMock(t)
	observedAt := time.Date(2026, 8, 12, 10, 15, 0, 0, time.UTC)
	scope := LorebookReferenceScope{ChatSessionID: "session-a", EnabledModuleIDs: []string{}, EnabledModulesObserved: true}
	_, identityJSON, _ := lorebookScopeJSON(scope)
	mock.ExpectBegin()
	expectLorebookReferenceSessionLock(mock, "session-a")
	mock.ExpectQuery("FROM lorebook_reference_scopes").
		WithArgs("session-a", nil, nil).
		WillReturnRows(sqlmock.NewRows([]string{"scope_id", "enabled_modules_json", "scope_identity_json", "created_at", "updated_at"}).
			AddRow(14, `[]`, identityJSON, observedAt, observedAt))
	expectLatestLorebookReferenceAuthorityObservation(mock, 14, observedAt.Add(-time.Minute))
	mock.ExpectExec("INSERT INTO lorebook_reference_snapshots").
		WithArgs("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", int64(14), LorebookReferenceSnapshotContractV1,
			LorebookConsentRevoked, LorebookObservationObserved, false, 0, `{}`, observedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE lorebook_reference_entries").
		WithArgs(LorebookLifecycleConsentRevoked, observedAt, int64(14)).
		WillReturnResult(sqlmock.NewResult(0, 5))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM lorebook_reference_entries`).
		WithArgs(int64(14)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectCommit()

	result, err := maria.ApplyLorebookReferenceSnapshot(context.Background(), &LorebookReferenceSnapshot{
		SnapshotID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", ContractVersion: LorebookReferenceSnapshotContractV1,
		ConsentState: LorebookConsentRevoked, ObservationState: LorebookObservationObserved,
		Scope: scope, ProvenanceJSON: `{}`, ObservedAt: observedAt,
	})
	if err != nil {
		t.Fatalf("ApplyLorebookReferenceSnapshot: %v", err)
	}
	if result.PreviousCurrentCount != 5 || result.CurrentEntryCount != 0 || result.LifecycleAction != "consent_revoked" {
		t.Fatalf("consent revocation result=%#v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyLorebookReferenceCompleteEmptySnapshotClearsOnlyExactCurrentProjection(t *testing.T) {
	maria, mock := newLorebookReferenceMock(t)
	observedAt := time.Date(2026, 8, 12, 10, 20, 0, 0, time.UTC)
	characterIndex := int64(3)
	chatIndex := int64(8)
	scope := LorebookReferenceScope{
		ChatSessionID: "session-a", CharacterIndex: &characterIndex, ChatIndex: &chatIndex,
		EnabledModuleIDs: []string{}, EnabledModulesObserved: true,
	}
	_, identityJSON, _ := lorebookScopeJSON(scope)
	mock.ExpectBegin()
	expectLorebookReferenceSessionLock(mock, "session-a")
	mock.ExpectQuery("FROM lorebook_reference_scopes").
		WithArgs("session-a", int64(3), int64(8)).
		WillReturnRows(sqlmock.NewRows([]string{"scope_id", "enabled_modules_json", "scope_identity_json", "created_at", "updated_at"}).
			AddRow(15, `[]`, identityJSON, observedAt, observedAt))
	expectLatestLorebookReferenceAuthorityObservation(mock, 15, observedAt.Add(-time.Minute))
	mock.ExpectExec("INSERT INTO lorebook_reference_snapshots").
		WithArgs("cccccccccccccccccccccccccccccccc", int64(15), LorebookReferenceSnapshotContractV1,
			LorebookConsentActive, LorebookObservationObserved, true, 0, `{}`, observedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE lorebook_reference_entries").
		WithArgs(LorebookLifecycleStale, observedAt, int64(15)).
		WillReturnResult(sqlmock.NewResult(0, 6))
	mock.ExpectCommit()

	result, err := maria.ApplyLorebookReferenceSnapshot(context.Background(), &LorebookReferenceSnapshot{
		SnapshotID: "cccccccccccccccccccccccccccccccc", ContractVersion: LorebookReferenceSnapshotContractV1,
		ConsentState: LorebookConsentActive, ObservationState: LorebookObservationObserved, CompleteSnapshot: true,
		Scope: scope, ProvenanceJSON: `{}`, ObservedAt: observedAt,
	})
	if err != nil {
		t.Fatalf("ApplyLorebookReferenceSnapshot: %v", err)
	}
	if result.PreviousCurrentCount != 6 || result.CurrentEntryCount != 0 || result.LifecycleAction != "current_projection_replaced" {
		t.Fatalf("complete empty snapshot result=%#v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyLorebookReferenceOlderCompleteSnapshotCannotReplaceNewerProjection(t *testing.T) {
	maria, mock := newLorebookReferenceMock(t)
	observedAt := time.Date(2026, 8, 12, 10, 25, 0, 0, time.UTC)
	characterIndex := int64(3)
	chatIndex := int64(8)
	scope := LorebookReferenceScope{
		ChatSessionID: "session-a", CharacterIndex: &characterIndex, ChatIndex: &chatIndex,
		EnabledModuleIDs: []string{}, EnabledModulesObserved: true,
	}
	_, identityJSON, _ := lorebookScopeJSON(scope)
	mock.ExpectBegin()
	expectLorebookReferenceSessionLock(mock, "session-a")
	mock.ExpectQuery("FROM lorebook_reference_scopes").
		WithArgs("session-a", int64(3), int64(8)).
		WillReturnRows(sqlmock.NewRows([]string{"scope_id", "enabled_modules_json", "scope_identity_json", "created_at", "updated_at"}).
			AddRow(16, `[]`, identityJSON, observedAt, observedAt))
	expectLatestLorebookReferenceAuthorityObservation(mock, 16, observedAt.Add(time.Minute))
	mock.ExpectExec("INSERT INTO lorebook_reference_snapshots").
		WithArgs("dddddddddddddddddddddddddddddddd", int64(16), LorebookReferenceSnapshotContractV1,
			LorebookConsentActive, LorebookObservationObserved, true, 1, `{}`, observedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO lorebook_reference_entries").
		WithArgs(int64(16), "dddddddddddddddddddddddddddddddd", "old-entry", 0,
			"current_host_aggregate", nil, "old", "", "", "old body", "old old body",
			nil, nil, nil, nil, nil, nil, nil, nil, `{}`, lorebookDiagnosticContentHash("old body"),
			LorebookLifecycleStale, false, observedAt, observedAt).
		WillReturnResult(sqlmock.NewResult(61, 1))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM lorebook_reference_entries`).
		WithArgs(int64(16)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(4))
	mock.ExpectCommit()

	result, err := maria.ApplyLorebookReferenceSnapshot(context.Background(), &LorebookReferenceSnapshot{
		SnapshotID: "dddddddddddddddddddddddddddddddd", ContractVersion: LorebookReferenceSnapshotContractV1,
		ConsentState: LorebookConsentActive, ObservationState: LorebookObservationObserved, CompleteSnapshot: true,
		Scope: scope, ProvenanceJSON: `{}`, ObservedAt: observedAt,
		Entries: []LorebookReferenceEntryObservation{{HostEntryID: "old-entry", Key: "old", Content: "old body"}},
	})
	if err != nil {
		t.Fatalf("ApplyLorebookReferenceSnapshot: %v", err)
	}
	if result.CurrentEntryCount != 4 || result.PreviousCurrentCount != 0 || result.LifecycleAction != "out_of_order_observation_recorded" {
		t.Fatalf("older complete snapshot replaced current projection: %#v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyLorebookReferenceOlderRevocationCannotDisableNewerProjection(t *testing.T) {
	maria, mock := newLorebookReferenceMock(t)
	observedAt := time.Date(2026, 8, 12, 10, 30, 0, 0, time.UTC)
	scope := LorebookReferenceScope{ChatSessionID: "session-a", EnabledModuleIDs: []string{}, EnabledModulesObserved: true}
	_, identityJSON, _ := lorebookScopeJSON(scope)
	mock.ExpectBegin()
	expectLorebookReferenceSessionLock(mock, "session-a")
	mock.ExpectQuery("FROM lorebook_reference_scopes").
		WithArgs("session-a", nil, nil).
		WillReturnRows(sqlmock.NewRows([]string{"scope_id", "enabled_modules_json", "scope_identity_json", "created_at", "updated_at"}).
			AddRow(17, `[]`, identityJSON, observedAt, observedAt))
	expectLatestLorebookReferenceAuthorityObservation(mock, 17, observedAt.Add(time.Minute))
	mock.ExpectExec("INSERT INTO lorebook_reference_snapshots").
		WithArgs("eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", int64(17), LorebookReferenceSnapshotContractV1,
			LorebookConsentRevoked, LorebookObservationObserved, false, 0, `{}`, observedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM lorebook_reference_entries`).
		WithArgs(int64(17)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))
	mock.ExpectCommit()

	result, err := maria.ApplyLorebookReferenceSnapshot(context.Background(), &LorebookReferenceSnapshot{
		SnapshotID: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", ContractVersion: LorebookReferenceSnapshotContractV1,
		ConsentState: LorebookConsentRevoked, ObservationState: LorebookObservationObserved,
		Scope: scope, ProvenanceJSON: `{}`, ObservedAt: observedAt,
	})
	if err != nil {
		t.Fatalf("ApplyLorebookReferenceSnapshot: %v", err)
	}
	if result.CurrentEntryCount != 5 || result.PreviousCurrentCount != 0 || result.LifecycleAction != "out_of_order_observation_recorded" {
		t.Fatalf("older revocation disabled newer projection: %#v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetLorebookReferenceCurrentPageReadsOnlyRequestedWindow(t *testing.T) {
	maria, mock := newLorebookReferenceMock(t)
	characterIndex := int64(2)
	chatIndex := int64(7)
	observedAt := time.Date(2026, 8, 12, 11, 0, 0, 0, time.UTC)
	scope := LorebookReferenceScope{
		ChatSessionID: "session-a", CharacterIndex: &characterIndex, ChatIndex: &chatIndex,
		EnabledModuleIDs: []string{"module-a"}, EnabledModulesObserved: true,
	}
	_, identityJSON, err := lorebookScopeJSON(scope)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("FROM lorebook_reference_scopes").
		WithArgs("session-a", int64(2), int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"scope_id", "enabled_modules_json", "scope_identity_json", "created_at", "updated_at"}).
			AddRow(21, `["module-a"]`, identityJSON, observedAt, observedAt))
	mock.ExpectQuery("FROM lorebook_reference_snapshots").
		WithArgs(int64(21)).
		WillReturnRows(sqlmock.NewRows([]string{"snapshot_id", "contract_version", "consent_state", "observation_state", "complete_snapshot", "provenance_json", "observed_at"}).
			AddRow("snapshot-current", LorebookReferenceSnapshotContractV1, LorebookConsentActive, LorebookObservationObserved, true, `{}`, observedAt))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\).*FROM lorebook_reference_entries").
		WithArgs(int64(21), LorebookLifecycleCurrent).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(41))
	mock.ExpectQuery("FROM lorebook_reference_entries.*LIMIT \\? OFFSET \\?").
		WithArgs(int64(21), LorebookLifecycleCurrent, 20, 20).
		WillReturnRows(sqlmock.NewRows([]string{
			"host_entry_id", "entry_ordinal", "source_kind", "source_identity", "entry_key", "second_key",
			"entry_comment", "content", "normalized_search_text", "entry_mode", "always_active", "selective",
			"use_regex", "insert_order", "activation_percent", "book_version", "folder", "extensions_json", "content_hash",
		}).AddRow("entry-20", 20, "current_host_aggregate", nil, "Han-eol", "exam", "profile", "stored lore",
			"han-eol exam profile stored lore", "normal", false, nil, nil, 0, nil, int64(3), "cast", `{}`, "hash"))

	page, err := maria.GetLorebookReferenceCurrentPage(context.Background(), scope, 20, 20)
	if err != nil {
		t.Fatalf("GetLorebookReferenceCurrentPage: %v", err)
	}
	if page.ScopeID != 21 || page.Total != 41 || page.Limit != 20 || page.Offset != 20 || len(page.Entries) != 1 || page.Entries[0].Key != "Han-eol" {
		t.Fatalf("page=%#v", page)
	}
	if page.LatestSnapshot == nil || page.LatestSnapshot.SnapshotID != "snapshot-current" {
		t.Fatalf("snapshot=%#v", page.LatestSnapshot)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGetLorebookReferenceLatestSessionPageResolvesNewestStoredScope(t *testing.T) {
	maria, mock := newLorebookReferenceMock(t)
	characterIndex := int64(3)
	chatIndex := int64(8)
	observedAt := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)
	scope := LorebookReferenceScope{
		ChatSessionID: "session-b", CharacterIndex: &characterIndex, ChatIndex: &chatIndex,
		EnabledModuleIDs: []string{"module-b"}, EnabledModulesObserved: true,
	}
	_, identityJSON, err := lorebookScopeJSON(scope)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("(?s)SELECT scope.scope_identity_json.*JOIN lorebook_reference_snapshots.*WHERE scope.chat_session_id").
		WithArgs("session-b").
		WillReturnRows(sqlmock.NewRows([]string{"scope_identity_json"}).AddRow(identityJSON))
	mock.ExpectQuery("FROM lorebook_reference_scopes").
		WithArgs("session-b", int64(3), int64(8)).
		WillReturnRows(sqlmock.NewRows([]string{"scope_id", "enabled_modules_json", "scope_identity_json", "created_at", "updated_at"}).
			AddRow(31, `["module-b"]`, identityJSON, observedAt, observedAt))
	mock.ExpectQuery("FROM lorebook_reference_snapshots").
		WithArgs(int64(31)).
		WillReturnRows(sqlmock.NewRows([]string{"snapshot_id", "contract_version", "consent_state", "observation_state", "complete_snapshot", "provenance_json", "observed_at"}).
			AddRow("snapshot-latest", LorebookReferenceSnapshotContractV1, LorebookConsentActive, LorebookObservationObserved, true, `{}`, observedAt))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\).*FROM lorebook_reference_entries").
		WithArgs(int64(31), LorebookLifecycleCurrent).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("FROM lorebook_reference_entries.*LIMIT \\? OFFSET \\?").
		WithArgs(int64(31), LorebookLifecycleCurrent, 20, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"host_entry_id", "entry_ordinal", "source_kind", "source_identity", "entry_key", "second_key",
			"entry_comment", "content", "normalized_search_text", "entry_mode", "always_active", "selective",
			"use_regex", "insert_order", "activation_percent", "book_version", "folder", "extensions_json", "content_hash",
		}).AddRow("entry-0", 0, "current_host_aggregate", nil, "Archive", "", "profile", "stored session lore",
			"archive profile stored session lore", "normal", false, nil, nil, 0, nil, int64(1), "cast", `{}`, "hash"))

	page, err := maria.GetLorebookReferenceLatestSessionPage(context.Background(), "session-b", 20, 0)
	if err != nil {
		t.Fatalf("GetLorebookReferenceLatestSessionPage: %v", err)
	}
	if page.ScopeID != 31 || page.Total != 1 || len(page.Entries) != 1 || page.Entries[0].Content != "stored session lore" {
		t.Fatalf("page=%#v", page)
	}
	if page.Scope.ChatSessionID != "session-b" || page.LatestSnapshot == nil || page.LatestSnapshot.SnapshotID != "snapshot-latest" {
		t.Fatalf("scope=%#v snapshot=%#v", page.Scope, page.LatestSnapshot)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
