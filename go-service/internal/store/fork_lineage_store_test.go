package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMariaDBStoreSaveForkLineageRecordPersistsManualProvenance(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	importedAt := time.Date(2026, 6, 23, 3, 0, 0, 0, time.UTC)
	created := time.Date(2026, 6, 23, 3, 1, 0, 0, time.UTC)
	record := ForkLineageRecord{
		ChatSessionID:       "sess-fork",
		ScopeID:             "scope-child",
		ParentScopeID:       "scope-parent",
		CopiedFromSessionID: "sess-parent",
		ImportedAt:          importedAt,
		DivergenceMarker:    `{"turn":12}`,
		ProvenanceSource:    "manual",
		InheritanceMode:     "conservative_import",
		InheritedItemsJSON:  `["consequence_records"]`,
		CreatedAt:           created,
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO session_fork_lineage")).
		WithArgs(
			ForkLineageContractVersion, "manual", "sess-fork",
			"scope-child", "scope-parent", nil, "sess-parent", nil, nil, nil, nil,
			importedAt, `{"turn":12}`, "manual", "conservative_import",
			`["consequence_records"]`, created,
		).
		WillReturnResult(sqlmock.NewResult(88, 1))

	saved, err := m.SaveForkLineageRecord(context.Background(), record)
	if err != nil {
		t.Fatalf("SaveForkLineageRecord: %v", err)
	}
	if saved.ID != 88 || saved.ImportedAt != importedAt || saved.CreatedAt != created || saved.UpdatedAt != created {
		t.Fatalf("unexpected saved record: %+v", saved)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBStoreListForkLineageRecordsScansSupportBoundaryFields(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	importedAt := time.Date(2026, 6, 23, 3, 0, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"id", "contract_version", "lineage_state", "chat_session_id",
		"scope_id", "parent_scope_id", "copied_from_scope_id", "copied_from_session_id",
		"fork_turn", "fork_source_message_id", "fork_source_role", "idempotency_key",
		"imported_at", "divergence_marker", "provenance_source",
		"inheritance_mode", "inherited_items_json", "created_at", "updated_at",
	}).AddRow(
		int64(88), ForkLineageContractVersion, "manual", "sess-fork",
		"scope-child", "scope-parent", nil, "sess-parent", nil, nil, nil, nil,
		importedAt, `{"turn":12}`, "manual",
		"conservative_import", `["consequence_records"]`, importedAt, importedAt,
	)
	mock.ExpectQuery("FROM session_fork_lineage").
		WithArgs("sess-fork", "scope-child", "scope-child", 25).
		WillReturnRows(rows)

	records, err := m.ListForkLineageRecords(context.Background(), "sess-fork", "scope-child", 25)
	if err != nil {
		t.Fatalf("ListForkLineageRecords: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("len(records)=%d", len(records))
	}
	got := records[0]
	if got.ID != 88 || got.ScopeID != "scope-child" || got.ParentScopeID != "scope-parent" || got.CopiedFromSessionID != "sess-parent" {
		t.Fatalf("unexpected record: %+v", got)
	}
	if got.InheritanceMode != "conservative_import" || got.InheritedItemsJSON == "" || got.DivergenceMarker == "" {
		t.Fatalf("missing support fields: %+v", got)
	}
	if got.ForkSourceRole != "" {
		t.Fatalf("manual lineage source role=%q, want empty legacy-compatible value", got.ForkSourceRole)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBStoreSaveConfirmedV2ForkLineageValidatesSourceRole(t *testing.T) {
	tests := []struct {
		name     string
		role     string
		wantText string
	}{
		{name: "missing", role: "", wantText: "fork_source_role is required"},
		{name: "unsupported", role: "assistant", wantText: "fork_source_role must be user or char"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db, _, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			record := automaticForkLineageFixture(time.Date(2026, 8, 18, 1, 0, 0, 0, time.UTC))
			record.ForkSourceRole = tc.role

			if _, err := (&mariadbStore{db: db}).SaveForkLineageRecord(context.Background(), record); err == nil ||
				!strings.Contains(err.Error(), tc.wantText) {
				t.Fatalf("source role validation error = %v", err)
			}
		})
	}
}

func TestMariaDBStoreSaveAutomaticForkLineageRecordIsIdempotent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	importedAt := time.Date(2026, 8, 18, 1, 2, 3, 0, time.UTC)
	record := automaticForkLineageFixture(importedAt)
	record.ForkSourceRole = "char"

	mock.ExpectBegin()
	mock.ExpectQuery("(?s)FROM session_fork_lineage.*FOR UPDATE").
		WithArgs(record.ChatSessionID, record.IdempotencyKey).
		WillReturnRows(forkLineageRows().AddRow(forkLineageRowValues(record, 91, importedAt)...))
	mock.ExpectQuery("(?s)FROM session_fork_lineage.*WHERE chat_session_id = \\? AND idempotency_key = \\?").
		WithArgs(record.ChatSessionID, record.IdempotencyKey).
		WillReturnRows(forkLineageRows().AddRow(forkLineageRowValues(record, 91, importedAt)...))
	mock.ExpectCommit()

	saved, err := m.SaveForkLineageRecord(context.Background(), record)
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID != 91 || saved.LineageState != "confirmed" || saved.ForkTurn != 7 || saved.ForkSourceRole != "char" {
		t.Fatalf("saved=%+v", saved)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBStoreSaveAutomaticForkLineageRecordUpgradesConfirmedV1ToV2(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	importedAt := time.Date(2026, 8, 18, 1, 30, 0, 0, time.UTC)
	incoming := automaticForkLineageFixture(importedAt)
	existing := incoming
	existing.ContractVersion = ForkLineageContractVersion
	existing.ForkSourceRole = ""

	mock.ExpectBegin()
	mock.ExpectQuery("(?s)FROM session_fork_lineage.*FOR UPDATE").
		WithArgs(incoming.ChatSessionID, incoming.IdempotencyKey).
		WillReturnRows(forkLineageRows().AddRow(forkLineageRowValues(existing, 96, importedAt)...))
	mock.ExpectExec("(?s)INSERT INTO session_fork_lineage.*fork_source_role.*ON DUPLICATE KEY UPDATE").
		WithArgs(
			RisuWorldlineForkLineageContractVersion, "confirmed", "child-session",
			nil, nil, nil, "parent-session", 7, "source-message", "user", "risu-worldline:key",
			importedAt, nil, "automatic_hook", "none", nil, importedAt,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("(?s)FROM session_fork_lineage.*WHERE chat_session_id = \\? AND idempotency_key = \\?").
		WithArgs(incoming.ChatSessionID, incoming.IdempotencyKey).
		WillReturnRows(forkLineageRows().AddRow(forkLineageRowValues(incoming, 96, importedAt)...))
	mock.ExpectCommit()

	saved, err := m.SaveForkLineageRecord(context.Background(), incoming)
	if err != nil {
		t.Fatal(err)
	}
	if saved.ContractVersion != RisuWorldlineForkLineageContractVersion || saved.ForkSourceRole != "user" {
		t.Fatalf("confirmed v1 lineage was not upgraded: %+v", saved)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBStoreSaveAutomaticForkLineageRecordRejectsStaleOverwrite(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	newerAt := time.Date(2026, 8, 18, 2, 0, 0, 0, time.UTC)
	existing := automaticForkLineageFixture(newerAt)
	existing.ForkTurn = 8
	stale := automaticForkLineageFixture(newerAt.Add(-time.Minute))

	mock.ExpectBegin()
	mock.ExpectQuery("(?s)FROM session_fork_lineage.*FOR UPDATE").
		WithArgs(stale.ChatSessionID, stale.IdempotencyKey).
		WillReturnRows(forkLineageRows().AddRow(forkLineageRowValues(existing, 92, newerAt)...))
	mock.ExpectQuery("(?s)FROM session_fork_lineage.*WHERE chat_session_id = \\? AND idempotency_key = \\?").
		WithArgs(stale.ChatSessionID, stale.IdempotencyKey).
		WillReturnRows(forkLineageRows().AddRow(forkLineageRowValues(existing, 92, newerAt)...))
	mock.ExpectCommit()

	saved, err := m.SaveForkLineageRecord(context.Background(), stale)
	if err != nil {
		t.Fatal(err)
	}
	if saved.ForkTurn != 8 || !saved.ImportedAt.Equal(newerAt) {
		t.Fatalf("stale save overwrote newer lineage: %+v", saved)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBStoreSaveAutomaticForkLineageRecordPromotesUnresolvedToConfirmed(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	confirmedAt := time.Date(2026, 8, 18, 2, 30, 0, 0, time.UTC)
	confirmed := automaticForkLineageFixture(confirmedAt)
	unresolved := confirmed
	unresolved.LineageState = "unresolved"
	unresolved.CopiedFromSessionID = ""
	unresolved.ForkTurn = 0
	unresolved.ForkSourceMessageID = "source-message"
	unresolved.ImportedAt = confirmedAt.Add(-time.Minute)

	mock.ExpectBegin()
	mock.ExpectQuery("(?s)FROM session_fork_lineage.*FOR UPDATE").
		WithArgs(confirmed.ChatSessionID, confirmed.IdempotencyKey).
		WillReturnRows(forkLineageRows().AddRow(forkLineageRowValues(unresolved, 94, unresolved.ImportedAt)...))
	mock.ExpectExec("(?s)INSERT INTO session_fork_lineage.*ON DUPLICATE KEY UPDATE").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("(?s)FROM session_fork_lineage.*WHERE chat_session_id = \\? AND idempotency_key = \\?").
		WithArgs(confirmed.ChatSessionID, confirmed.IdempotencyKey).
		WillReturnRows(forkLineageRows().AddRow(forkLineageRowValues(confirmed, 94, confirmedAt)...))
	mock.ExpectCommit()

	saved, err := m.SaveForkLineageRecord(context.Background(), confirmed)
	if err != nil {
		t.Fatal(err)
	}
	if saved.LineageState != "confirmed" || saved.ForkTurn != 7 {
		t.Fatalf("saved=%+v", saved)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBStoreSaveAutomaticForkLineageRecordKeepsConfirmedImmutable(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	confirmedAt := time.Date(2026, 8, 18, 2, 45, 0, 0, time.UTC)
	confirmed := automaticForkLineageFixture(confirmedAt)
	laterUnresolved := confirmed
	laterUnresolved.LineageState = "unresolved"
	laterUnresolved.CopiedFromSessionID = ""
	laterUnresolved.ForkTurn = 0
	laterUnresolved.ForkSourceRole = ""
	laterUnresolved.ImportedAt = confirmedAt.Add(time.Minute)

	mock.ExpectBegin()
	mock.ExpectQuery("(?s)FROM session_fork_lineage.*FOR UPDATE").
		WithArgs(confirmed.ChatSessionID, confirmed.IdempotencyKey).
		WillReturnRows(forkLineageRows().AddRow(forkLineageRowValues(confirmed, 95, confirmedAt)...))
	mock.ExpectQuery("(?s)FROM session_fork_lineage.*WHERE chat_session_id = \\? AND idempotency_key = \\?").
		WithArgs(confirmed.ChatSessionID, confirmed.IdempotencyKey).
		WillReturnRows(forkLineageRows().AddRow(forkLineageRowValues(confirmed, 95, confirmedAt)...))
	mock.ExpectCommit()

	saved, err := m.SaveForkLineageRecord(context.Background(), laterUnresolved)
	if err != nil {
		t.Fatal(err)
	}
	if saved.LineageState != "confirmed" || saved.ForkTurn != 7 || saved.ForkSourceRole != "user" || !saved.ImportedAt.Equal(confirmedAt) {
		t.Fatalf("confirmed lineage downgraded: %+v", saved)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMariaDBStoreSaveAutomaticForkLineageRecordRejectsReadbackMismatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	importedAt := time.Date(2026, 8, 18, 3, 0, 0, 0, time.UTC)
	record := automaticForkLineageFixture(importedAt)
	mismatched := record
	mismatched.ForkSourceRole = "char"

	mock.ExpectBegin()
	mock.ExpectQuery("(?s)FROM session_fork_lineage.*FOR UPDATE").
		WithArgs(record.ChatSessionID, record.IdempotencyKey).
		WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("(?s)INSERT INTO session_fork_lineage.*ON DUPLICATE KEY UPDATE").
		WillReturnResult(sqlmock.NewResult(93, 1))
	mock.ExpectQuery("(?s)FROM session_fork_lineage.*WHERE chat_session_id = \\? AND idempotency_key = \\?").
		WithArgs(record.ChatSessionID, record.IdempotencyKey).
		WillReturnRows(forkLineageRows().AddRow(forkLineageRowValues(mismatched, 93, importedAt)...))
	mock.ExpectRollback()
	if _, err := m.SaveForkLineageRecord(context.Background(), record); err == nil ||
		!strings.Contains(err.Error(), "session fork lineage readback mismatch") {
		t.Fatalf("readback mismatch error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func automaticForkLineageFixture(importedAt time.Time) ForkLineageRecord {
	return ForkLineageRecord{
		ContractVersion:     RisuWorldlineForkLineageContractVersion,
		LineageState:        "confirmed",
		ChatSessionID:       "child-session",
		CopiedFromSessionID: "parent-session",
		ForkTurn:            7,
		ForkSourceMessageID: "source-message",
		ForkSourceRole:      "user",
		IdempotencyKey:      "risu-worldline:key",
		ImportedAt:          importedAt,
		ProvenanceSource:    "automatic_hook",
		InheritanceMode:     "none",
		CreatedAt:           importedAt,
	}
}

func TestWorldline43ConfirmedLineageOnlyEnrichesEmptyOriginMetadata(t *testing.T) {
	for _, existingJSON := range []string{"", "[]", `[{"contract_version":"risu_message_origins.v1","items":[]}]`} {
		t.Run(existingJSON, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			m := &mariadbStore{db: db}
			at := time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC)
			existing := automaticForkLineageFixture(at)
			existing.InheritedItemsJSON = existingJSON
			incoming := existing
			incoming.CopiedFromSessionID = "must-not-replace-parent"
			incoming.ForkTurn = 99
			incoming.InheritedItemsJSON = `[{"contract_version":"risu_message_origins.v1","parent_host_chat_id":"parent","items":[{"child_message_id":"new","parent_message_id":"old","role":"char"}]}]`
			expected := existing
			mock.ExpectBegin()
			mock.ExpectQuery("(?s)FROM session_fork_lineage.*FOR UPDATE").WithArgs(existing.ChatSessionID, existing.IdempotencyKey).WillReturnRows(forkLineageRows().AddRow(forkLineageRowValues(existing, 93, at)...))
			if existingJSON == "" || existingJSON == "[]" {
				expected.InheritedItemsJSON = incoming.InheritedItemsJSON
				mock.ExpectExec("UPDATE session_fork_lineage SET inherited_items_json = \\? WHERE id = \\?").WithArgs(incoming.InheritedItemsJSON, int64(93)).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			mock.ExpectQuery("(?s)FROM session_fork_lineage.*WHERE chat_session_id = \\? AND idempotency_key = \\?").WithArgs(existing.ChatSessionID, existing.IdempotencyKey).WillReturnRows(forkLineageRows().AddRow(forkLineageRowValues(expected, 93, at)...))
			mock.ExpectCommit()
			got, err := m.SaveForkLineageRecord(context.Background(), incoming)
			if err != nil {
				t.Fatal(err)
			}
			if got.InheritedItemsJSON != expected.InheritedItemsJSON || got.CopiedFromSessionID != existing.CopiedFromSessionID || got.ForkTurn != existing.ForkTurn {
				t.Fatalf("got=%+v", got)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func forkLineageRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"id", "contract_version", "lineage_state", "chat_session_id",
		"scope_id", "parent_scope_id", "copied_from_scope_id", "copied_from_session_id",
		"fork_turn", "fork_source_message_id", "fork_source_role", "idempotency_key", "imported_at",
		"divergence_marker", "provenance_source", "inheritance_mode", "inherited_items_json",
		"created_at", "updated_at",
	})
}

func forkLineageRowValues(record ForkLineageRecord, id int64, importedAt time.Time) []driver.Value {
	var forkSourceRole driver.Value
	if record.ForkSourceRole != "" {
		forkSourceRole = record.ForkSourceRole
	}
	return []driver.Value{
		id, record.ContractVersion, record.LineageState, record.ChatSessionID,
		nil, nil, nil, record.CopiedFromSessionID, record.ForkTurn,
		record.ForkSourceMessageID, forkSourceRole, record.IdempotencyKey, importedAt,
		nil, record.ProvenanceSource, record.InheritanceMode, nullableString(record.InheritedItemsJSON),
		record.CreatedAt, record.CreatedAt,
	}
}
