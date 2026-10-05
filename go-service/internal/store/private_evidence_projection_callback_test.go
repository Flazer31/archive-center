package store

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func privateEvidenceProjectionSource(turn, revision string) sessionMigrationRow {
	const extraction = `{"evidence_excerpts":["synthetic crossing quote","Public beacon."]}`
	a := &MemoryAdmission{SourceRevision: revision, DerivationVersion: MemoryAdmissionContract, ExtractorVersion: "synthetic", IndexVersion: MemoryPublicProjectionIndex, ResultJSON: extraction}
	return stitchTestRow(map[string]string{
		"id": turn, "chat_session_id": "source", "turn_index": turn, "source_revision": revision, "lifecycle_state": "active",
		"derived_admission_state": "committed", "derived_admission_version": a.DerivationVersion, "derived_extractor_version": a.ExtractorVersion,
		"derived_index_version": a.IndexVersion, "derived_result_json": a.ResultJSON, "derived_result_hash": memoryAdmissionExpectedResultHash(a),
	})
}

// The HTTP package tests the actual projection. These store tests verify that
// each missing-receipt row supplies its own evidence to that production callback
// and publishes exactly the callback's result, without a live database.
func TestPrivateEvidenceProjectionStitchCallbackReachesOutbox(t *testing.T) {
	const evidence = `{"source_quote_scope":{"synthetic crossing quote":true}}`
	const public = "[Canonical Summary]\nPublic beacon."
	for _, withEvidence := range []bool{true, false} {
		t.Run(map[bool]string{true: "stored_scope", false: "legacy_empty_evidence"}[withEvidence], func(t *testing.T) {
			stored := evidence
			if !withEvidence {
				stored = ""
			}
			source := map[string][]sessionMigrationRow{
				"memories":                {stitchTestRow(map[string]string{"id": "101", "chat_session_id": "source", "turn_index": "1", "summary_json": `{"turn_summary":"canonical full text"}`, "evidence": stored})},
				"memory_source_revisions": {privateEvidenceProjectionSource("1", "rev-source")},
			}
			calls := 0
			rows, _, err := sessionStitchSnapshot([]string{"source"}, []map[string][]sessionMigrationRow{source}, "target", func(raw, gotEvidence string) string {
				calls++
				if gotEvidence != stored || raw != source["memory_source_revisions"][0].Values["derived_result_json"].Text {
					t.Errorf("matching row evidence/extraction lost: %q %q", raw, gotEvidence)
				}
				return public
			})
			if err != nil || calls != 1 {
				t.Fatalf("callback not executed: calls=%d err=%v", calls, err)
			}
			if len(rows["memory_vector_outbox"]) != 1 {
				t.Fatal("projection receipt absent")
			}
			var doc struct{ DocumentText, ChatSessionID string }
			if err := json.Unmarshal([]byte(rows["memory_vector_outbox"][0].Values["document_json"].Text), &doc); err != nil {
				t.Fatal(err)
			}
			if doc.DocumentText != public || doc.ChatSessionID != "target" {
				t.Fatalf("outbox changed callback result: %+v", doc)
			}
			if rows["memories"][0].Values["evidence"].Text != stored {
				t.Error("canonical evidence changed")
			}
		})
	}
}

type privateEvidenceProjectionDocument struct{ text string }

func (m privateEvidenceProjectionDocument) Match(v driver.Value) bool {
	text, ok := v.(string)
	if !ok {
		return false
	}
	var doc struct{ DocumentText, SourceRowID string }
	return json.Unmarshal([]byte(text), &doc) == nil && doc.DocumentText == m.text && doc.SourceRowID == "101"
}

func TestPrivateEvidenceProjectionMigrationCallbackReachesOutbox(t *testing.T) {
	const evidence = `{"source_quote_scope":{"synthetic crossing quote":true}}`
	const public = "[Canonical Summary]\nPublic beacon."
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectExec("INSERT INTO session_migrations").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO session_migration_saga_steps").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO session_migration_saga_steps").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO memory_vector_outbox").WithArgs(
		MemoryVectorOutboxContract, sqlmock.AnyArg(), "upsert", "source", "rev-source", "memory:source:101", privateEvidenceProjectionDocument{text: public},
		sqlmock.AnyArg(), "active", "completed", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
	).WillReturnResult(sqlmock.NewResult(1, 1))
	stop := errors.New("synthetic stop after outbox enqueue")
	mock.ExpectQuery("SELECT .* FROM `memory_vector_outbox`").WithArgs("source").WillReturnError(stop)
	mock.ExpectRollback()
	source := map[string][]sessionMigrationRow{
		"memories":                {stitchTestRow(map[string]string{"id": "101", "chat_session_id": "source", "turn_index": "1", "evidence": evidence})},
		"memory_source_revisions": {privateEvidenceProjectionSource("1", "rev-source")},
	}
	calls := 0
	_, err = completeSessionMigrationSnapshotTx(context.Background(), tx, SessionMigrationCompleteRequest{
		SourceSessionID: "source", TargetSessionID: "target", Mode: SessionMigrationModeCopyKeepSource,
		RebuildPublicProjection: func(raw, stored string) string {
			calls++
			if stored != evidence || !strings.Contains(raw, "synthetic crossing quote") {
				t.Errorf("stored scope not passed: %q %q", raw, stored)
			}
			return public
		},
	}, source, map[string][]sessionMigrationRow{})
	if !errors.Is(err, stop) || calls != 1 {
		t.Fatalf("production callback/enqueue path not reached: calls=%d err=%v", calls, err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
