package store

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func sessionMigrationExpectCurrentRelationalLedger(
	mock sqlmock.Sqlmock,
	migrationID int64,
	sourceID, targetID, status string,
	overrides map[string]sessionMigrationStoredArtifactParity,
) {
	mock.ExpectQuery("SELECT source_session_id, target_session_id, status.*FROM session_migrations.*WHERE id = \\?").
		WithArgs(migrationID).
		WillReturnRows(sqlmock.NewRows([]string{"source_session_id", "target_session_id", "status"}).
			AddRow(sourceID, targetID, status))
	parityRows := sqlmock.NewRows([]string{
		"table_name", "source_row_count", "source_content_hash", "target_row_count", "target_content_hash",
	})
	for _, entry := range SessionMigrationManifest() {
		plan, _ := SessionMigrationExecutionPlanFor(entry.Table)
		emptyHash := sessionMigrationCanonicalRowsHash(
			entry, plan, nil, sourceID, false, newSessionMigrationKeyMaps(),
		)
		parity := sessionMigrationStoredArtifactParity{
			SourceCount: 0,
			SourceHash:  emptyHash,
			TargetCount: 0,
			TargetHash:  emptyHash,
		}
		if override, ok := overrides[entry.Table]; ok {
			parity = override
		}
		parityRows.AddRow(
			entry.Table,
			parity.SourceCount, parity.SourceHash,
			parity.TargetCount, parity.TargetHash,
		)
	}
	mock.ExpectQuery("SELECT table_name, source_row_count, source_content_hash.*target_row_count, target_content_hash.*FROM session_migration_artifact_parity").
		WithArgs(migrationID, SessionMigrationManifestVersion).
		WillReturnRows(parityRows)
	mock.ExpectQuery("SELECT table_name, key_column_name, source_key, target_key.*FROM session_migration_artifact_row_map").
		WithArgs(migrationID).
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "key_column_name", "source_key", "target_key"}))
}

func sessionMigrationExpectEmptyCurrentManifestReads(
	mock sqlmock.Sqlmock,
	sourceID, targetID string,
) string {
	fingerprintParts := []string{SessionMigrationManifestVersion, sourceID, targetID}
	for _, entry := range SessionMigrationManifest() {
		plan, _ := SessionMigrationExecutionPlanFor(entry.Table)
		pattern := "SELECT .* FROM " + regexp.QuoteMeta("`"+entry.Table+"`") + " t0"
		mock.ExpectQuery(pattern).
			WithArgs(sourceID).
			WillReturnRows(sqlmock.NewRows(plan.Columns))
		mock.ExpectQuery(pattern).
			WithArgs(targetID).
			WillReturnRows(sqlmock.NewRows(plan.Columns))
		emptyHash := sessionMigrationCanonicalRowsHash(
			entry, plan, nil, sourceID, false, newSessionMigrationKeyMaps(),
		)
		fingerprintParts = append(
			fingerprintParts,
			entry.Table, "0", emptyHash, "0", emptyHash,
		)
	}
	return sessionMigrationStringHash(fingerprintParts...)
}

func sessionMigrationRequireBlockerCode(t *testing.T, err error, code string) {
	t.Helper()
	var blocker *SessionMigrationBlockerError
	if !errors.As(err, &blocker) || blocker.Code != code {
		t.Fatalf("error = %v, want typed blocker %q", err, code)
	}
}

func TestSessionMigrationSchemaMismatchFailsBeforeMutation(t *testing.T) {
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
	plan, ok := SessionMigrationExecutionPlanFor("chat_logs")
	if !ok {
		t.Fatal("chat_logs execution plan missing")
	}
	mock.ExpectQuery("SELECT COLUMN_NAME, EXTRA, GENERATION_EXPRESSION.*INFORMATION_SCHEMA.COLUMNS").
		WithArgs("chat_logs").
		WillReturnRows(sqlmock.NewRows([]string{"COLUMN_NAME", "EXTRA", "GENERATION_EXPRESSION"}).
			AddRow("id", "", nil).
			AddRow("chat_session_id", "", nil).
			AddRow("unexpected_drift", "", nil))
	err = sessionMigrationValidateSchemaTx(context.Background(), tx, plan)
	if err == nil || !strings.Contains(err.Error(), "schema mismatch") {
		t.Fatalf("schema validation error = %v, want mismatch", err)
	}
	mock.ExpectRollback()
	if err := tx.Rollback(); err != nil && err != sql.ErrTxDone {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("schema mismatch performed an unexpected write: %v", err)
	}
}

func TestSessionMigrationPolicyEvaluationUsesProductionParityOwner(t *testing.T) {
	chatPlan, _ := SessionMigrationExecutionPlanFor("chat_logs")
	source := sessionMigrationTestRow(map[string]string{
		"id": "1", "chat_session_id": "source", "turn_index": "1",
		"role": "user", "content": "hello", "created_at": "2026-07-30 00:00:00.000",
	})
	target := sessionMigrationTestRow(map[string]string{
		"id": "11", "chat_session_id": "target", "turn_index": "1",
		"role": "user", "content": "hello", "created_at": "2026-07-30 00:00:00.000",
	})
	maps := newSessionMigrationKeyMaps()
	if err := maps.put("chat_logs", "id", "1", "11"); err != nil {
		t.Fatal(err)
	}
	copyEntry, _ := sessionMigrationManifestEntryByTable("chat_logs")
	sourceHash := sessionMigrationCanonicalRowsHash(copyEntry, chatPlan, []sessionMigrationRow{source}, "source", false, maps)
	targetHash := sessionMigrationCanonicalRowsHash(copyEntry, chatPlan, []sessionMigrationRow{target}, "target", true, maps)
	evaluation, err := sessionMigrationEvaluateArtifactParity(
		copyEntry, chatPlan, []sessionMigrationRow{source}, []sessionMigrationRow{target}, sourceHash, targetHash, maps,
	)
	if err != nil {
		t.Fatal(err)
	}
	if evaluation.ParityState != "verified_copy" || evaluation.RowMapExpected != 1 || evaluation.RowMapVerified != 1 {
		t.Fatalf("copy evaluation = %+v", evaluation)
	}

	for _, tc := range []struct {
		table string
		want  string
	}{
		{"audit_logs", "verified_retain_audit"},
		{"session_reference_runtime", "verified_regenerate"},
		{"memory_vector_outbox", "verified_delete_pending"},
	} {
		entry, _ := sessionMigrationManifestEntryByTable(tc.table)
		plan, _ := SessionMigrationExecutionPlanFor(tc.table)
		evaluation, err := sessionMigrationEvaluateArtifactParity(
			entry, plan, []sessionMigrationRow{sessionMigrationTestRow(map[string]string{})}, nil, "source-hash", "empty-hash", newSessionMigrationKeyMaps(),
		)
		if err != nil {
			t.Fatalf("%s policy evaluation: %v", tc.table, err)
		}
		if evaluation.ParityState != tc.want {
			t.Errorf("%s parity state = %q, want %q", tc.table, evaluation.ParityState, tc.want)
		}
	}
}

func TestSessionMigrationCopyParityRejectsCountAndHashMismatch(t *testing.T) {
	entry, _ := sessionMigrationManifestEntryByTable("chat_logs")
	plan, _ := SessionMigrationExecutionPlanFor("chat_logs")
	source := sessionMigrationTestRow(map[string]string{"id": "1", "chat_session_id": "source"})
	maps := newSessionMigrationKeyMaps()
	_ = maps.put("chat_logs", "id", "1", "11")
	if _, err := sessionMigrationEvaluateArtifactParity(
		entry, plan, []sessionMigrationRow{source}, nil, "source-hash", "target-hash", maps,
	); err == nil || !strings.Contains(err.Error(), "parity mismatch") {
		t.Fatalf("count/hash mismatch error = %v", err)
	}
}

func TestSessionMigrationInsertCreatesRowMapAndDefersSelfFK(t *testing.T) {
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
	entry, _ := sessionMigrationManifestEntryByTable("direct_evidence_records")
	plan, _ := SessionMigrationExecutionPlanFor(entry.Table)
	source := sessionMigrationTestRow(map[string]string{
		"id": "1", "chat_session_id": "source", "evidence_kind": "fact_event",
		"evidence_text": "evidence", "superseded_by_id": "1",
	})
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `direct_evidence_records`")).
		WillReturnResult(sqlmock.NewResult(20, 1))
	mock.ExpectExec("INSERT INTO session_migration_artifact_row_map").
		WithArgs(int64(7), "direct_evidence_records", "id", "1", "20", "copied").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO session_migration_row_map").
		WithArgs(int64(7), "direct_evidence_records", int64(1), int64(20)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	maps := newSessionMigrationKeyMaps()
	target, targetKey, deferred, err := sessionMigrationInsertManifestRow(
		context.Background(), tx, 7, entry, plan, source, "target", maps,
	)
	if err != nil {
		t.Fatal(err)
	}
	if targetKey != "20" || target.Values["chat_session_id"].Text != "target" {
		t.Fatalf("target row key/session = %q/%q", targetKey, target.Values["chat_session_id"].Text)
	}
	if len(deferred) != 1 || deferred[0].Column != "superseded_by_id" || deferred[0].SourceValue != "1" {
		t.Fatalf("deferred FK = %+v", deferred)
	}
	if mapped, ok := maps.target("direct_evidence_records", "id", deferred[0].SourceValue); !ok || mapped != "20" {
		t.Fatalf("self FK map = %q/%t, want 20/true", mapped, ok)
	}
	mock.ExpectRollback()
	_ = tx.Rollback()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSessionMigrationExactVectorIDComparisonRejectsSameCountWrongIDs(t *testing.T) {
	result := sessionMigrationCompareVectorIDs(
		42,
		"target",
		[]string{"memory:target:1", "episode:target:2"},
		[]string{"memory:target:1", "episode:target:wrong"},
	)
	if result.Verified {
		t.Fatal("same vector count with different IDs must not verify")
	}
	if len(result.MissingIDs) != 1 || result.MissingIDs[0] != "episode:target:2" {
		t.Fatalf("missing IDs = %v", result.MissingIDs)
	}
	if len(result.UnexpectedIDs) != 1 || result.UnexpectedIDs[0] != "episode:target:wrong" {
		t.Fatalf("unexpected IDs = %v", result.UnexpectedIDs)
	}
	if result.ExpectedIDHash == result.ActualIDHash {
		t.Fatal("different exact IDs produced the same canonical hash")
	}
}

func TestSessionMigrationPostCutoverVectorParityAllowsAdditionsButNeverMissingMigratedIDs(t *testing.T) {
	expected := []string{"memory:target:1"}
	actualWithLiveAddition := []string{"memory:target:1", "memory:target:live-2"}
	exact := sessionMigrationCompareVectorIDsWithPolicy(
		42, "target", expected, actualWithLiveAddition, false,
	)
	if exact.Verified {
		t.Fatal("pre-cutover exact parity accepted an unexpected live document")
	}
	postCutover := sessionMigrationCompareVectorIDsWithPolicy(
		42, "target", expected, actualWithLiveAddition, true,
	)
	if !postCutover.Verified || len(postCutover.UnexpectedIDs) != 1 {
		t.Fatalf("post-cutover subset parity = %+v", postCutover)
	}
	missing := sessionMigrationCompareVectorIDsWithPolicy(
		42, "target", expected, []string{"memory:target:live-2"}, true,
	)
	if missing.Verified || len(missing.MissingIDs) != 1 {
		t.Fatalf("post-cutover parity accepted missing migrated document: %+v", missing)
	}
}

func TestSessionMigrationCleanupHashesOnlyMigrationOwnedTargetRows(t *testing.T) {
	entry, _ := sessionMigrationManifestEntryByTable("chat_logs")
	plan, _ := SessionMigrationExecutionPlanFor(entry.Table)
	maps := newSessionMigrationKeyMaps()
	if err := maps.put("chat_logs", "id", "1", "11"); err != nil {
		t.Fatal(err)
	}
	migrated := sessionMigrationTestRow(map[string]string{
		"id": "11", "chat_session_id": "target", "turn_index": "1", "role": "assistant", "content": "copied",
	})
	live := sessionMigrationTestRow(map[string]string{
		"id": "12", "chat_session_id": "target", "turn_index": "2", "role": "user", "content": "continued",
	})
	owned := sessionMigrationOwnedTargetRows(entry, plan, []sessionMigrationRow{migrated, live}, maps)
	if len(owned) != 1 || owned[0].Values["id"].Text != "11" {
		t.Fatalf("migration-owned target rows = %+v, want only copied row", owned)
	}
}

func TestSessionMigrationExpectedVectorDocumentsMatchAllManagedTierContracts(t *testing.T) {
	tests := []struct {
		table      string
		target     map[string]string
		wantID     string
		wantText   string
		wantSchema string
	}{
		{
			table: "memories",
			target: map[string]string{
				"id": "101", "summary_json": `{"turn_summary":"Mina found the gate.","characters":["Mina"],"archive_hint":{"room":"Gate Room"}}`,
				"evidence": `{"evidence_excerpts":["The gate opened."]}`, "place_wing": "East Wing", "place_room": "Gate Room",
			},
			wantID:     "memory:target:101",
			wantText:   "[Canonical Summary]\nMina found the gate.\n\n[Raw Evidence]\nThe gate opened.\n\n[Aliases]\nMina\nGate Room\nEast Wing",
			wantSchema: "memory.v2",
		},
		{
			table: "direct_evidence_records",
			target: map[string]string{
				"id": "102", "evidence_kind": "fact_event", "evidence_text": "The gate opened.",
				"source_turn_start": "4", "source_turn_end": "5", "turn_anchor": "5",
				"repair_needed": "0", "tombstoned": "0",
			},
			wantID:     "evidence:target:102",
			wantText:   "kind: fact_event\nThe gate opened.\nturns: 4-5 anchor:5",
			wantSchema: "direct_evidence.v1",
		},
		{
			table: "world_rules",
			target: map[string]string{
				"id": "103", "scope": "world", "scope_name": "Archive", "category": "physics",
				"key": "gate", "value_json": `{"opens":true}`, "suppressed": "0",
			},
			wantID:     "world_rule:target:103",
			wantText:   "world\nArchive\nphysics\ngate\n{\"opens\":true}",
			wantSchema: "world_rule.v1",
		},
		{
			table: "kg_triples",
			target: map[string]string{
				"id": "104", "subject": "Mina", "predicate": "opened", "object": "Gate",
			},
			wantID:     "kg_triple:target:104",
			wantText:   "Mina\nopened\nGate",
			wantSchema: "kg_triple.v1",
		},
		{table: "episode_summaries", target: map[string]string{"id": "105", "summary_text": "Episode summary"}, wantID: "episode:target:105", wantText: "Episode summary", wantSchema: "episode.v1"},
		{table: "chapter_summaries", target: map[string]string{"id": "106", "summary_text": "Chapter summary", "resume_text": "Resume chapter"}, wantID: "chapter:target:106", wantText: "Chapter summary\nResume chapter", wantSchema: "chapter.v1"},
		{table: "arc_summaries", target: map[string]string{"id": "107", "core_conflict": "Core conflict", "arc_resume_text": "Resume arc"}, wantID: "arc:target:107", wantText: "Core conflict\nResume arc", wantSchema: "arc.v1"},
		{table: "saga_digests", target: map[string]string{"id": "108", "saga_summary": "Saga summary", "resume_pack_text": "Resume saga"}, wantID: "saga:target:108", wantText: "Saga summary\nResume saga", wantSchema: "saga.v1"},
		{
			table: "precise_memory_units",
			target: map[string]string{
				"id": "109", "unit_id": "target-unit", "evidence_excerpt": "Exact accepted evidence.", "lifecycle_state": "active",
			},
			wantID:     "precise_memory:target:target-unit",
			wantText:   "Exact accepted evidence.",
			wantSchema: "precise_memory_unit.v1",
		},
	}
	for _, tc := range tests {
		t.Run(tc.table, func(t *testing.T) {
			plan, ok := SessionMigrationExecutionPlanFor(tc.table)
			if !ok || plan.Vector == nil {
				t.Fatalf("%s vector plan missing", tc.table)
			}
			target := sessionMigrationTestRow(tc.target)
			doc, ok := sessionMigrationExpectedVectorDocument(
				7, tc.table, plan, sessionMigrationTestRow(map[string]string{}), target, "source", "target",
			)
			if !ok {
				t.Fatal("expected vector document was skipped")
			}
			if doc.ID != tc.wantID || doc.SourceRowID != strings.TrimPrefix(tc.wantID, plan.Vector.Tier+":target:") {
				t.Errorf("document ID/source row = %q/%q, want %q", doc.ID, doc.SourceRowID, tc.wantID)
			}
			if doc.DocumentText != tc.wantText || doc.SchemaVersion != tc.wantSchema {
				t.Errorf("document text/schema = %q/%q, want %q/%q", doc.DocumentText, doc.SchemaVersion, tc.wantText, tc.wantSchema)
			}
		})
	}
}

func TestResumeCompletedSessionMigrationReturnsDurableLedger(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	countsJSON := `{"chat_logs":2,"canonical_total":2,"canonical_and_subjective_total":2}`
	mock.ExpectQuery("SELECT sm.id, sm.status, sm.counts_json").
		WithArgs("source", "target", SessionMigrationModeCopyThenLockSource).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "status", "counts_json", "chroma_reindexed_count", "locked_at",
		}).AddRow(int64(44), "copied", countsJSON, 0, nil))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT.*COUNT\\(\\*\\).*FROM session_migration_artifact_parity").
		WithArgs(int64(44), SessionMigrationManifestVersion).
		WillReturnRows(sqlmock.NewRows([]string{"total", "relational", "row_maps"}).
			AddRow(50, 50, 2))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\).*session_migration_artifact_row_map").
		WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
	mock.ExpectQuery("SELECT source_table, document_id.*session_migration_vector_expected_ids").
		WithArgs(int64(44)).
		WillReturnRows(sqlmock.NewRows([]string{"source_table", "document_id"}))
	parityRows := sqlmock.NewRows([]string{"table_name", "vector_expected_count", "vector_expected_id_hash"})
	for _, entry := range SessionMigrationManifest() {
		parityRows.AddRow(entry.Table, 0, "")
	}
	mock.ExpectQuery("SELECT table_name, COALESCE\\(vector_expected_count").
		WithArgs(int64(44), SessionMigrationManifestVersion).
		WillReturnRows(parityRows)
	sessionMigrationExpectCurrentRelationalLedger(
		mock, 44, "source", "target", "copied", nil,
	)
	sessionMigrationExpectEmptyCurrentManifestReads(mock, "source", "target")
	mock.ExpectCommit()
	result, err := resumeCompletedSessionMigration(
		context.Background(), db, "source", "target", SessionMigrationModeCopyThenLockSource,
	)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.MigrationID != 44 || result.Status != "copied" || result.Counts.ChatLogs != 2 || result.RowMapCount != 2 {
		t.Fatalf("resume result = %+v", result)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestResumeCompletedSessionMigrationRejectsLegacyManifestProof(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT sm.id, sm.status, sm.counts_json").
		WithArgs("source", "target", SessionMigrationModeCopyThenLockSource).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "status", "counts_json", "chroma_reindexed_count", "locked_at",
		}).AddRow(int64(43), "copied", `{}`, 0, nil))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT.*COUNT\\(\\*\\).*FROM session_migration_artifact_parity").
		WithArgs(int64(43), SessionMigrationManifestVersion).
		WillReturnRows(sqlmock.NewRows([]string{"total", "relational", "row_maps"}).
			AddRow(0, 0, 0))
	mock.ExpectRollback()
	result, err := resumeCompletedSessionMigration(
		context.Background(), db, "source", "target", SessionMigrationModeCopyThenLockSource,
	)
	if err == nil || !strings.Contains(err.Error(), "current manifest relational proof") || result != nil {
		t.Fatalf("legacy resume result=%+v error=%v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestResumeCompletedSessionMigrationRejectsMissingExpectedVectorID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT sm.id, sm.status, sm.counts_json").
		WithArgs("source", "target", SessionMigrationModeCopyThenLockSource).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "status", "counts_json", "chroma_reindexed_count", "locked_at",
		}).AddRow(int64(45), "copied", `{}`, 0, nil))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT.*COUNT\\(\\*\\).*FROM session_migration_artifact_parity").
		WithArgs(int64(45), SessionMigrationManifestVersion).
		WillReturnRows(sqlmock.NewRows([]string{"total", "relational", "row_maps"}).
			AddRow(50, 50, 0))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\).*session_migration_artifact_row_map").
		WithArgs(int64(45)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT source_table, document_id.*session_migration_vector_expected_ids").
		WithArgs(int64(45)).
		WillReturnRows(sqlmock.NewRows([]string{"source_table", "document_id"}))
	parityRows := sqlmock.NewRows([]string{"table_name", "vector_expected_count", "vector_expected_id_hash"})
	for _, entry := range SessionMigrationManifest() {
		if entry.Table == "memories" {
			parityRows.AddRow(entry.Table, 1, sessionMigrationHashIDs([]string{"memory:target:1"}))
		} else {
			parityRows.AddRow(entry.Table, 0, "")
		}
	}
	mock.ExpectQuery("SELECT table_name, COALESCE\\(vector_expected_count").
		WithArgs(int64(45), SessionMigrationManifestVersion).
		WillReturnRows(parityRows)
	mock.ExpectRollback()
	result, err := resumeCompletedSessionMigration(
		context.Background(), db, "source", "target", SessionMigrationModeCopyThenLockSource,
	)
	if err == nil || !strings.Contains(err.Error(), "vector expected-ID ledger mismatch") || result != nil {
		t.Fatalf("missing vector resume result=%+v error=%v", result, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSessionMigrationNormalizeDatabaseValuePreservesMariaDBTypes(t *testing.T) {
	stamp := time.Date(2026, 7, 30, 1, 2, 3, 456789000, time.FixedZone("KST", 9*60*60))
	tests := []struct {
		name  string
		value any
		valid bool
		text  string
	}{
		{name: "null", value: nil},
		{name: "datetime", value: stamp, valid: true, text: "2026-07-30 01:02:03.456789"},
		{name: "bytes", value: []byte(`{"ok":true}`), valid: true, text: `{"ok":true}`},
		{name: "integer", value: int64(42), valid: true, text: "42"},
		{name: "float", value: float64(0.75), valid: true, text: "0.75"},
		{name: "boolean", value: true, valid: true, text: "1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cell, err := sessionMigrationNormalizeDatabaseValue(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			if cell.Valid != tc.valid || cell.Text != tc.text {
				t.Fatalf("cell=%+v want valid=%t text=%q", cell, tc.valid, tc.text)
			}
		})
	}
}

func TestSessionMigrationSemanticReferencesRemapAndRejectDanglingLineage(t *testing.T) {
	plan, _ := SessionMigrationExecutionPlanFor("precise_memory_units")
	source := sessionMigrationTestRow(map[string]string{
		"direct_evidence_ids_json": `[1,2]`,
	})
	target := sessionMigrationTestRow(map[string]string{
		"direct_evidence_ids_json": `[1,2]`,
	})
	maps := newSessionMigrationKeyMaps()
	_ = maps.put("direct_evidence_records", "id", "1", "11")
	_ = maps.put("direct_evidence_records", "id", "2", "12")
	if err := sessionMigrationRemapSemanticReferences(plan, source, &target, maps); err != nil {
		t.Fatal(err)
	}
	if target.Values["direct_evidence_ids_json"].Text != `[11,12]` {
		t.Fatalf("evidence IDs = %q", target.Values["direct_evidence_ids_json"].Text)
	}
	source.Values["direct_evidence_ids_json"] = sessionMigrationCell{Valid: true, Text: `[1, 2]`}
	if got := sessionMigrationVerifiedSemanticReferenceCount(
		source, target, plan.SemanticReferences[0], maps,
	); got != 2 {
		t.Fatalf("semantic parity count = %d, want 2 despite JSON whitespace", got)
	}

	dangling := sessionMigrationTestRow(map[string]string{"direct_evidence_ids_json": `[1,3]`})
	if err := sessionMigrationRemapSemanticReferences(plan, dangling, &target, maps); err == nil || !strings.Contains(err.Error(), "no row map") {
		t.Fatalf("dangling semantic reference error = %v", err)
	}
}

func TestSessionMigrationDeterministicUUIDStableAndTargetScoped(t *testing.T) {
	first := sessionMigrationDeterministicUUID(SessionMigrationManifestVersion, "target-a", "entity_identities", "stable_entity_id", "source")
	second := sessionMigrationDeterministicUUID(SessionMigrationManifestVersion, "target-a", "entity_identities", "stable_entity_id", "source")
	otherTarget := sessionMigrationDeterministicUUID(SessionMigrationManifestVersion, "target-b", "entity_identities", "stable_entity_id", "source")
	if first != second || first == otherTarget {
		t.Fatalf("deterministic UUIDs first=%q second=%q other=%q", first, second, otherTarget)
	}
}

func TestSessionMigrationDatabaseGeneratedColumnIsNotWritableOrSourceOwned(t *testing.T) {
	entry, _ := sessionMigrationManifestEntryByTable("memory_source_revisions")
	plan, _ := SessionMigrationExecutionPlanFor(entry.Table)
	if len(plan.DatabaseGenerated) != 1 || plan.DatabaseGenerated[0] != "active_logical_turn_slot" {
		t.Fatalf("database-generated columns = %v", plan.DatabaseGenerated)
	}
	if !sessionMigrationPlanDatabaseGenerated(plan, "active_logical_turn_slot") {
		t.Fatal("active_logical_turn_slot must be excluded from INSERT")
	}
	source := sessionMigrationTestRow(map[string]string{
		"id": "1", "chat_session_id": "source", "source_revision": "revision-source",
		"active_logical_turn_slot": "logical-source",
	})
	target := sessionMigrationTestRow(map[string]string{
		"id": "11", "chat_session_id": "target", "source_revision": "revision-target",
		"active_logical_turn_slot": "logical-generated-by-target",
	})
	maps := newSessionMigrationKeyMaps()
	_ = maps.put(entry.Table, "id", "1", "11")
	_ = maps.put(entry.Table, "source_revision", "revision-source", "revision-target")
	sourceHash := sessionMigrationCanonicalRowsHash(entry, plan, []sessionMigrationRow{source}, "source", false, maps)
	targetHash := sessionMigrationCanonicalRowsHash(entry, plan, []sessionMigrationRow{target}, "target", true, maps)
	if sourceHash != targetHash {
		t.Fatalf("generated column incorrectly changed owned hash: source=%s target=%s", sourceHash, targetHash)
	}
}

func TestSessionMigrationCurrentRelationalStateRejectsSourceAndTargetDriftBeforeMutation(t *testing.T) {
	entry, _ := sessionMigrationManifestEntryByTable("chat_logs")
	plan, _ := SessionMigrationExecutionPlanFor(entry.Table)
	emptyHash := sessionMigrationCanonicalRowsHash(
		entry, plan, nil, "source", false, newSessionMigrationKeyMaps(),
	)
	tests := []struct {
		name     string
		override sessionMigrationStoredArtifactParity
		wantCode string
	}{
		{
			name: "source drift",
			override: sessionMigrationStoredArtifactParity{
				SourceCount: 1, SourceHash: "stored-source-hash",
				TargetCount: 0, TargetHash: emptyHash,
			},
			wantCode: "current_source_snapshot_drift",
		},
		{
			name: "target drift",
			override: sessionMigrationStoredArtifactParity{
				SourceCount: 0, SourceHash: emptyHash,
				TargetCount: 1, TargetHash: "stored-target-hash",
			},
			wantCode: "current_target_snapshot_drift",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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
			sessionMigrationExpectCurrentRelationalLedger(
				mock, 42, "source", "target", "vector_reindexed",
				map[string]sessionMigrationStoredArtifactParity{"chat_logs": tc.override},
			)
			query := "SELECT .* FROM " + regexp.QuoteMeta("`chat_logs`") + " t0.*FOR UPDATE"
			mock.ExpectQuery(query).
				WithArgs("source").
				WillReturnRows(sqlmock.NewRows(plan.Columns))
			mock.ExpectQuery(query).
				WithArgs("target").
				WillReturnRows(sqlmock.NewRows(plan.Columns))
			_, err = sessionMigrationRevalidateCurrentRelationalStateTx(
				context.Background(), tx, 42, "source_lock", true,
			)
			sessionMigrationRequireBlockerCode(t, err, tc.wantCode)
			mock.ExpectRollback()
			if err := tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSessionMigrationCurrentStateProofIsOperationScopedAndOneShot(t *testing.T) {
	t.Run("missing proof", func(t *testing.T) {
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
		mock.ExpectExec("UPDATE session_migration_saga_steps").
			WithArgs(
				int64(42),
				"current_state_revalidation_"+SessionMigrationProofOperationSourceLock,
				"relational-hash",
			).
			WillReturnResult(sqlmock.NewResult(0, 0))
		err = sessionMigrationConsumeCurrentStateProofTx(
			context.Background(), tx, 42,
			SessionMigrationProofOperationSourceLock, "relational-hash",
		)
		sessionMigrationRequireBlockerCode(t, err, "current_vector_snapshot_required")
		mock.ExpectRollback()
		_ = tx.Rollback()
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("proof cannot be reused", func(t *testing.T) {
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
		args := []driver.Value{
			int64(42),
			"current_state_revalidation_" + SessionMigrationProofOperationCleanupPrepare,
			"relational-hash",
		}
		mock.ExpectExec("UPDATE session_migration_saga_steps").
			WithArgs(args...).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec("UPDATE session_migration_saga_steps").
			WithArgs(args...).
			WillReturnResult(sqlmock.NewResult(0, 0))
		if err := sessionMigrationConsumeCurrentStateProofTx(
			context.Background(), tx, 42,
			SessionMigrationProofOperationCleanupPrepare, "relational-hash",
		); err != nil {
			t.Fatalf("first proof consumption failed: %v", err)
		}
		err = sessionMigrationConsumeCurrentStateProofTx(
			context.Background(), tx, 42,
			SessionMigrationProofOperationCleanupPrepare, "relational-hash",
		)
		sessionMigrationRequireBlockerCode(t, err, "current_vector_snapshot_required")
		mock.ExpectRollback()
		_ = tx.Rollback()
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("operation phase mismatch", func(t *testing.T) {
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
		mock.ExpectQuery("SELECT status FROM session_migrations WHERE id = \\?").
			WithArgs(int64(42)).
			WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow("source_locked"))
		_, err = sessionMigrationValidateProofOperationTx(
			context.Background(), tx, 42,
			SessionMigrationProofOperationCleanupFinalize,
		)
		sessionMigrationRequireBlockerCode(t, err, "current_vector_proof_phase_mismatch")
		mock.ExpectRollback()
		_ = tx.Rollback()
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})
}

func TestResumeCompletedSessionMigrationRequiresFreshOneShotVectorProofAfterVectorPhase(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT sm.id, sm.status, sm.counts_json").
		WithArgs("source", "target", SessionMigrationModeCopyThenLockSource).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "status", "counts_json", "chroma_reindexed_count", "locked_at",
		}).AddRow(int64(46), "vector_reindexed", `{}`, 0, nil))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT.*COUNT\\(\\*\\).*FROM session_migration_artifact_parity").
		WithArgs(int64(46), SessionMigrationManifestVersion).
		WillReturnRows(sqlmock.NewRows([]string{"total", "relational", "row_maps"}).
			AddRow(50, 50, 0))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\).*session_migration_artifact_row_map").
		WithArgs(int64(46)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT source_table, document_id.*session_migration_vector_expected_ids").
		WithArgs(int64(46)).
		WillReturnRows(sqlmock.NewRows([]string{"source_table", "document_id"}))
	parityRows := sqlmock.NewRows([]string{"table_name", "vector_expected_count", "vector_expected_id_hash"})
	for _, entry := range SessionMigrationManifest() {
		parityRows.AddRow(entry.Table, 0, "")
	}
	mock.ExpectQuery("SELECT table_name, COALESCE\\(vector_expected_count").
		WithArgs(int64(46), SessionMigrationManifestVersion).
		WillReturnRows(parityRows)
	sessionMigrationExpectCurrentRelationalLedger(
		mock, 46, "source", "target", "vector_reindexed", nil,
	)
	relationalHash := sessionMigrationExpectEmptyCurrentManifestReads(mock, "source", "target")
	mock.ExpectQuery("SELECT.*COUNT\\(\\*\\).*FROM session_migration_artifact_parity").
		WithArgs(int64(46), SessionMigrationManifestVersion).
		WillReturnRows(sqlmock.NewRows([]string{"total", "relational", "vector"}).
			AddRow(50, 50, 50))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\), COALESCE\\(SUM.*session_migration_vector_expected_ids").
		WithArgs(int64(46)).
		WillReturnRows(sqlmock.NewRows([]string{"expected", "observed"}).AddRow(0, 0))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\).*session_migration_saga_steps").
		WithArgs(int64(46)).
		WillReturnRows(sqlmock.NewRows([]string{"completed"}).AddRow(1))
	mock.ExpectExec("UPDATE session_migration_saga_steps").
		WithArgs(
			int64(46),
			"current_state_revalidation_"+SessionMigrationProofOperationResume,
			relationalHash,
		).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()
	result, err := resumeCompletedSessionMigration(
		context.Background(), db, "source", "target", SessionMigrationModeCopyThenLockSource,
	)
	if result != nil {
		t.Fatalf("resume result = %+v, want nil", result)
	}
	sessionMigrationRequireBlockerCode(t, err, "current_vector_snapshot_required")
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSessionMigrationDurableParityGateRejectsEachMissingProof(t *testing.T) {
	tests := []struct {
		name               string
		total              int
		relationalVerified int
		vectorVerified     int
		expectedVectors    int
		observedVectors    int
		completedSaga      int
		want               string
	}{
		{name: "manifest", total: 49, relationalVerified: 49, vectorVerified: 49, want: "manifest parity rows 49/50"},
		{name: "relational", total: 50, relationalVerified: 49, vectorVerified: 50, want: "relational count/hash/row-map/FK parity 49/50"},
		{name: "vector artifact", total: 50, relationalVerified: 50, vectorVerified: 49, want: "vector artifact parity 49/50"},
		{name: "expected ids", total: 50, relationalVerified: 50, vectorVerified: 50, expectedVectors: 2, observedVectors: 1, want: "exact vector expected-ID parity 1/2"},
		{name: "saga", total: 50, relationalVerified: 50, vectorVerified: 50, expectedVectors: 2, observedVectors: 2, completedSaga: 0, want: "vector exact-ID saga is not completed"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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
			mock.ExpectQuery("SELECT.*COUNT\\(\\*\\).*FROM session_migration_artifact_parity").
				WithArgs(int64(42), SessionMigrationManifestVersion).
				WillReturnRows(sqlmock.NewRows([]string{"total", "relational", "vector"}).
					AddRow(tc.total, tc.relationalVerified, tc.vectorVerified))
			if tc.total == 50 && tc.relationalVerified == 50 && tc.vectorVerified == 50 {
				mock.ExpectQuery("SELECT COUNT\\(\\*\\), COALESCE\\(SUM.*session_migration_vector_expected_ids").
					WithArgs(int64(42)).
					WillReturnRows(sqlmock.NewRows([]string{"expected", "observed"}).
						AddRow(tc.expectedVectors, tc.observedVectors))
				if tc.expectedVectors == tc.observedVectors {
					mock.ExpectQuery("SELECT COUNT\\(\\*\\).*session_migration_saga_steps").
						WithArgs(int64(42)).
						WillReturnRows(sqlmock.NewRows([]string{"completed"}).AddRow(tc.completedSaga))
				}
			}
			err = sessionMigrationVerifyDurableParityTx(context.Background(), tx, 42)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("durable parity error = %v, want %q", err, tc.want)
			}
			mock.ExpectRollback()
			_ = tx.Rollback()
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func sessionMigrationTestRow(values map[string]string) sessionMigrationRow {
	row := sessionMigrationRow{Values: map[string]sessionMigrationCell{}}
	for column, value := range values {
		row.Values[column] = sessionMigrationCell{Valid: true, Text: value}
	}
	return row
}
