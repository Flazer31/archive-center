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

func TestMariaDBStoreSaveCaptureVerificationPersistsCompactRecord(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	created := time.Date(2026, 6, 23, 5, 20, 0, 0, time.UTC)
	record := CaptureVerificationRecord{
		ChatSessionID:       "sess-cap",
		TurnIndex:           12,
		StageName:           "afterRequest",
		VerificationState:   "verified-final",
		CompactMetadataJSON: `{"source":"native_afterRequest"}`,
		ContentHash:         "sha256:final",
		EvidenceJSON:        `{"stage":"finalize"}`,
		PreviousRecordID:    11,
		UserInputPreserved:  true,
		PayloadRewrite:      false,
		CreatedAt:           created,
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO capture_verification_records")).
		WithArgs(
			"sess-cap", 12, "afterRequest", "verified-final", nil,
			`{"source":"native_afterRequest"}`, "sha256:final", `{"stage":"finalize"}`,
			int64(11), int64(0), 0, nil, nil, true, false, created,
		).
		WillReturnResult(sqlmock.NewResult(77, 1))

	saved, err := m.SaveCaptureVerification(context.Background(), record)
	if err != nil {
		t.Fatalf("SaveCaptureVerification: %v", err)
	}
	if saved.ID != 77 || saved.CreatedAt != created || saved.UpdatedAt != created {
		t.Fatalf("unexpected saved record: %+v", saved)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBStoreListCaptureVerificationsScansRepairLineage(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	created := time.Date(2026, 6, 23, 5, 30, 0, 0, time.UTC)
	repaired := time.Date(2026, 6, 23, 5, 31, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{
		"id", "chat_session_id", "turn_index", "stage_name", "verification_state", "degraded_reason",
		"compact_metadata_json", "content_hash", "evidence_json", "previous_record_id", "repaired_by_record_id",
		"repair_attempt_count", "repair_evidence_json", "repaired_at", "user_input_preserved", "payload_rewrite",
		"created_at", "updated_at",
	}).AddRow(
		int64(77), "sess-cap", 12, "recovery", "degraded", "thinking_only_fragment",
		`{"source":"streaming_fallback"}`, "sha256:partial", `{"tail":"hash"}`, int64(76), int64(78),
		2, `{"matched_stage":"finalize"}`, repaired, true, false, created, created,
	)
	mock.ExpectQuery("FROM capture_verification_records").
		WithArgs("sess-cap", 25).
		WillReturnRows(rows)

	records, err := m.ListCaptureVerifications(context.Background(), "sess-cap", 25)
	if err != nil {
		t.Fatalf("ListCaptureVerifications: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("len(records)=%d", len(records))
	}
	got := records[0]
	if got.ID != 77 || got.StageName != "recovery" || got.VerificationState != "degraded" || got.PreviousRecordID != 76 || got.RepairedByRecordID != 78 {
		t.Fatalf("unexpected record: %+v", got)
	}
	if got.RepairAttemptCount != 2 || got.RepairEvidenceJSON == "" || got.RepairedAt.IsZero() || got.PayloadRewrite {
		t.Fatalf("missing repair lineage fields: %+v", got)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBStoreUpdateCaptureVerificationRepairRequiresEvidenceForSuccess(t *testing.T) {
	m := &mariadbStore{db: &sql.DB{}}
	err := m.UpdateCaptureVerificationRepair(context.Background(), 77, "verified-final", "", "", 76, true)
	if err == nil || !strings.Contains(err.Error(), "repair_evidence_json") {
		t.Fatalf("expected repair evidence error, got %v", err)
	}
}

func TestMariaDBStoreUpdateCaptureVerificationRepairPersistsEvidenceBoundLineage(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock new: %v", err)
	}
	defer db.Close()

	m := &mariadbStore{db: db}
	mock.ExpectExec("UPDATE capture_verification_records").
		WithArgs("verified-final", "", `{"matched_stage":"finalize"}`, int64(76), true, int64(77)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = m.UpdateCaptureVerificationRepair(context.Background(), 77, "verified-final", "", `{"matched_stage":"finalize"}`, 76, true)
	if err != nil {
		t.Fatalf("UpdateCaptureVerificationRepair: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestMariaDBStoreUpdateCaptureVerificationRepairRejectsFalsePreservationWithoutDegradation(t *testing.T) {
	m := &mariadbStore{db: &sql.DB{}}
	err := m.UpdateCaptureVerificationRepair(context.Background(), 77, "verified", "", `{"matched_stage":"finalize"}`, 76, false)
	if err == nil || !strings.Contains(err.Error(), "user_input_preserved=false") {
		t.Fatalf("expected user input preservation error, got %v", err)
	}
	if errors.Is(err, ErrNotEnabled) {
		t.Fatalf("validation should run before db update: %v", err)
	}
}
