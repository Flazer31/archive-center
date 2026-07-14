package store

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestReplaceSessionReferenceCoverageSnapshotWritesOneCurrentGeneration(t *testing.T) {
	store, mock := newReferenceLibraryMock(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT snapshot_hash, revision").
		WithArgs("binding-1").
		WillReturnRows(sqlmock.NewRows([]string{"snapshot_hash", "revision"}))
	mock.ExpectExec("INSERT INTO session_reference_coverage_snapshots").
		WithArgs("binding-1", "coverage_field_index.v1", "context-hash", "inventory-hash", "snapshot-hash", 2, 1, 1, `{"literal":true}`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM session_reference_coverage_fields WHERE binding_id = ?")).
		WithArgs("binding-1").
		WillReturnResult(sqlmock.NewResult(0, 0))
	prepared := mock.ExpectPrepare("INSERT INTO session_reference_coverage_fields")
	prepared.ExpectExec().WithArgs(
		"binding-1", "field-key", "work-1", "continuity-1", "entity", "entity-1",
		"canonical_name", "Mira", "mira", `["Mira"]`, true, `["system#0"]`, true, "eligible",
	).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	changed, err := store.ReplaceSessionReferenceCoverageSnapshot(context.Background(), &SessionReferenceCoverageSnapshot{
		BindingID: "binding-1", ContractVersion: "coverage_field_index.v1",
		ContextHash: "context-hash", InventoryHash: "inventory-hash", SnapshotHash: "snapshot-hash",
		SourceMessageCount: 2, FieldCount: 1, CoveredFieldCount: 1, StatsJSON: `{"literal":true}`,
	}, []SessionReferenceCoverageField{{
		FieldKey: "field-key", WorkID: "work-1", ContinuityID: "continuity-1",
		ReferenceKind: "entity", SourceID: "entity-1", FieldName: "canonical_name",
		FieldValue: "Mira", NormalizedValue: "mira", MatchValuesJSON: `["Mira"]`,
		PresentInContext: true, MatchedLocationsJSON: `["system#0"]`, Eligible: true, EligibilityReason: "eligible",
	}})
	if err != nil || !changed {
		t.Fatalf("ReplaceSessionReferenceCoverageSnapshot changed=%v err=%v", changed, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReplaceSessionReferenceCoverageSnapshotReusesSameHashWithoutRewrite(t *testing.T) {
	store, mock := newReferenceLibraryMock(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT snapshot_hash, revision").
		WithArgs("binding-1").
		WillReturnRows(sqlmock.NewRows([]string{"snapshot_hash", "revision"}).AddRow("snapshot-hash", int64(4)))
	mock.ExpectRollback()

	changed, err := store.ReplaceSessionReferenceCoverageSnapshot(context.Background(), &SessionReferenceCoverageSnapshot{
		BindingID: "binding-1", ContractVersion: "coverage_field_index.v1",
		ContextHash: "context-hash", InventoryHash: "inventory-hash", SnapshotHash: "snapshot-hash",
	}, nil)
	if err != nil || changed {
		t.Fatalf("same snapshot changed=%v err=%v", changed, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListReferenceEntityAliasesByScopeUsesOneScopedQuery(t *testing.T) {
	store, mock := newReferenceLibraryMock(t)
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	mock.ExpectQuery("FROM reference_entity_aliases").
		WithArgs("work-1", "continuity-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"alias_id", "work_id", "continuity_id", "entity_id", "alias_text", "normalized_alias", "language_code", "created_at",
		}).AddRow(1, "work-1", "continuity-1", "entity-1", "미라", "미라", "ko", now))
	items, err := store.ListReferenceEntityAliasesByScope(context.Background(), "work-1", "continuity-1")
	if err != nil || len(items) != 1 || items[0].AliasText != "미라" {
		t.Fatalf("aliases=%#v err=%v", items, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
