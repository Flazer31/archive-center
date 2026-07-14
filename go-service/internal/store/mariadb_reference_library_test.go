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

func newReferenceLibraryMock(t *testing.T) (*mariadbStore, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return &mariadbStore{db: db}, mock
}

func TestCreateReferenceWorkAndDuplicateDocumentGuard(t *testing.T) {
	store, mock := newReferenceLibraryMock(t)
	mock.ExpectExec("INSERT INTO reference_works").
		WithArgs("work-1", "Example", "novel", "ko", "draft", nil).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.CreateReferenceWork(context.Background(), &ReferenceWork{
		WorkID: "work-1", Title: "Example", WorkType: "novel", DefaultLanguage: "ko", Status: "draft",
	}); err != nil {
		t.Fatalf("CreateReferenceWork: %v", err)
	}

	mock.ExpectExec("INSERT INTO reference_documents").
		WithArgs("doc-2", "work-1", "continuity-1", "manual_text", nil, "same-hash", "full", "body", "pending", nil).
		WillReturnError(&mysql.MySQLError{Number: 1062, Message: "duplicate content hash"})
	err := store.SaveReferenceDocument(context.Background(), &ReferenceDocument{
		DocumentID: "doc-2", WorkID: "work-1", ContinuityID: "continuity-1",
		ContentHash: "same-hash", RawText: "body",
	})
	if !errors.Is(err, ErrReferenceConflict) {
		t.Fatalf("SaveReferenceDocument error = %v, want ErrReferenceConflict", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteReferenceWorkBlocksLinkedSession(t *testing.T) {
	store, mock := newReferenceLibraryMock(t)
	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT binding_id FROM session_reference_bindings
		WHERE work_id = ? LIMIT 1 FOR UPDATE`)).
		WithArgs("work-1").
		WillReturnRows(sqlmock.NewRows([]string{"binding_id"}).AddRow("binding-1"))
	mock.ExpectRollback()

	err := store.DeleteReferenceWork(context.Background(), "work-1")
	if !errors.Is(err, ErrReferenceWorkInUse) {
		t.Fatalf("DeleteReferenceWork error = %v, want ErrReferenceWorkInUse", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteReferenceWorkAfterUnlink(t *testing.T) {
	store, mock := newReferenceLibraryMock(t)
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT binding_id FROM session_reference_bindings").
		WithArgs("work-1").
		WillReturnRows(sqlmock.NewRows([]string{"binding_id"}))
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM reference_works WHERE work_id = ?")).
		WithArgs("work-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	if err := store.DeleteReferenceWork(context.Background(), "work-1"); err != nil {
		t.Fatalf("DeleteReferenceWork: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestReferenceWorkRevisionConflict(t *testing.T) {
	store, mock := newReferenceLibraryMock(t)
	mock.ExpectExec("UPDATE reference_works").
		WithArgs("Changed", "novel", "ko", "ready", nil, "work-1", int64(4)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	err := store.UpdateReferenceWork(context.Background(), &ReferenceWork{
		WorkID: "work-1", Title: "Changed", WorkType: "novel", DefaultLanguage: "ko", Status: "ready",
	}, 4)
	if !errors.Is(err, ErrReferenceConflict) {
		t.Fatalf("UpdateReferenceWork error = %v, want ErrReferenceConflict", err)
	}
}

func TestSessionDeleteStartsByRemovingBindingOnly(t *testing.T) {
	store, mock := newReferenceLibraryMock(t)
	stop := errors.New("stop after binding cleanup probe")
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM session_reference_bindings WHERE chat_session_id = ?")).
		WithArgs("session-1").
		WillReturnError(stop)
	err := store.DeleteSession(context.Background(), "session-1")
	if !errors.Is(err, stop) {
		t.Fatalf("DeleteSession error = %v, want probe error", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSessionBindingUsesOptimisticRevision(t *testing.T) {
	store, mock := newReferenceLibraryMock(t)
	mock.ExpectExec("UPDATE session_reference_bindings").
		WithArgs("primary", true, false, "manual", nil, nil, nil, "block", 0, "binding-1", "session-1", int64(2)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	err := store.UpsertSessionReferenceBinding(context.Background(), &SessionReferenceBinding{
		BindingID: "binding-1", ChatSessionID: "session-1", WorkID: "work-1", ContinuityID: "continuity-1",
		BindingRole: "primary", Enabled: true, AnchorMode: "manual", FuturePolicy: "block",
	}, 2)
	if !errors.Is(err, ErrReferenceConflict) {
		t.Fatalf("UpsertSessionReferenceBinding error = %v, want ErrReferenceConflict", err)
	}
}

func TestReferenceCandidateReviewIsScopedToWork(t *testing.T) {
	store, mock := newReferenceLibraryMock(t)
	mock.ExpectExec(regexp.QuoteMeta("UPDATE reference_entities SET review_status = ?, review_source = ?, review_reason = ?, reviewed_at = CURRENT_TIMESTAMP(3) WHERE work_id = ? AND entity_id = ?")).
		WithArgs("approved", "critic_auto", "direct evidence", "work-1", "entity-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := store.UpdateReferenceCandidateReview(context.Background(), "work-1", "entity", "entity-1", "approved", "critic_auto", "direct evidence"); err != nil {
		t.Fatalf("UpdateReferenceCandidateReview: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateReferenceLibraryEntityMarksUserEdit(t *testing.T) {
	store, mock := newReferenceLibraryMock(t)
	mock.ExpectExec("UPDATE reference_entities").
		WithArgs("location", "Correct Name", "Corrected", "work-1", "entity-1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	err := store.UpdateReferenceLibraryItem(context.Background(), &ReferenceLibraryItemUpdate{
		WorkID: "work-1", Kind: "entity", ID: "entity-1", EntityType: "location",
		CanonicalName: "Correct Name", DescriptionText: "Corrected",
	})
	if err != nil {
		t.Fatalf("UpdateReferenceLibraryItem: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestListReferenceEntitiesReturnsReviewAudit(t *testing.T) {
	store, mock := newReferenceLibraryMock(t)
	now := time.Date(2026, 7, 12, 12, 30, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT entity_id, work_id, continuity_id, entity_type, canonical_name").
		WithArgs("work-1", "continuity-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"entity_id", "work_id", "continuity_id", "entity_type", "canonical_name",
			"description_text", "metadata_json", "review_status", "review_source",
			"review_reason", "reviewed_at", "created_at", "updated_at",
		}).AddRow("entity-1", "work-1", "continuity-1", "faction", "HUNTR/X", "Hunters", nil,
			"approved", "critic_auto", "direct evidence", now, now, now))
	items, err := store.ListReferenceEntities(context.Background(), "work-1", "continuity-1", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ReviewSource != "critic_auto" || items[0].ReviewReason != "direct evidence" || items[0].ReviewedAt == nil {
		t.Fatalf("review audit was not returned: %#v", items)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAdminResetIncludesReferenceTablesChildFirst(t *testing.T) {
	wantOrder := []string{
		"session_reference_runtime", "session_reference_bindings", "reference_claim_knowers",
		"reference_claims", "reference_entity_aliases", "reference_entities",
		"reference_timeline_nodes", "reference_documents", "reference_continuities", "reference_works",
	}
	if len(mariaAdminResetTables) < len(wantOrder) {
		t.Fatalf("admin reset table count = %d", len(mariaAdminResetTables))
	}
	for i, want := range wantOrder {
		if mariaAdminResetTables[i] != want {
			t.Fatalf("admin reset table %d = %q, want %q", i, mariaAdminResetTables[i], want)
		}
	}
}
