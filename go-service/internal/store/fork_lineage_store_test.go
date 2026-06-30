package store

import (
	"context"
	"regexp"
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
			"sess-fork", "scope-child", "scope-parent", nil, "sess-parent",
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
		"id", "chat_session_id", "scope_id", "parent_scope_id", "copied_from_scope_id",
		"copied_from_session_id", "imported_at", "divergence_marker", "provenance_source",
		"inheritance_mode", "inherited_items_json", "created_at", "updated_at",
	}).AddRow(
		int64(88), "sess-fork", "scope-child", "scope-parent", nil,
		"sess-parent", importedAt, `{"turn":12}`, "manual",
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
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
