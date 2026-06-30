package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCompareMissingDSN(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, manifest{
		TablesExported: []string{"chat_logs"},
		RowCounts:      map[string]int{"chat_logs": 0},
	})

	report, err := runCompare(context.Background(), dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Status != "failed" {
		t.Fatalf("status = %q, want failed", report.Status)
	}
	if len(report.Errors) == 0 || !sliceContains(report.Errors, "missing DSN") {
		t.Fatalf("expected missing DSN error, got %+v", report.Errors)
	}
}

func TestCompareCountMatch(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, manifest{
		TablesExported: []string{"chat_logs"},
		RowCounts:      map[string]int{"chat_logs": 1},
	})
	writeNDJSON(t, dir, "chat_logs", []map[string]any{{
		"id":              1,
		"chat_session_id": "sess-1",
		"turn_index":      1,
		"role":            "user",
		"content":         "hello",
		"created_at":      "2024-01-01T00:00:00Z",
	}})

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM `chat_logs`")).
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `chat_logs`")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "chat_session_id", "turn_index", "role", "content", "created_at"}).
			AddRow(1, "sess-1", 1, "user", "hello", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)))

	report, err := compareExport(context.Background(), db, dir)
	if err != nil {
		t.Fatalf("compareExport failed: %v", err)
	}

	tr := findTable(report.Tables, "chat_logs")
	if tr == nil {
		t.Fatal("chat_logs table result missing")
	}
	if tr.Status != "ok" {
		t.Fatalf("status = %q, want ok, error=%q", tr.Status, tr.Error)
	}
	if tr.ExportCount != 1 || tr.MariaDBCount != 1 {
		t.Fatalf("counts mismatch: export=%d, mariadb=%d", tr.ExportCount, tr.MariaDBCount)
	}
	if tr.ChecksumMatch == nil || !*tr.ChecksumMatch {
		t.Fatalf("checksum mismatch: export=%s, mariadb=%s", tr.ExportChecksum, tr.MariaDBChecksum)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeComparableRowDecodesJSONColumns(t *testing.T) {
	row := normalizeComparableRow("pending_threads", map[string]any{
		"hook_metadata_json": `{"details":"plain text detail"}`,
	})
	got, ok := row["hook_metadata_json"].(map[string]any)
	if !ok {
		t.Fatalf("hook_metadata_json = %#v, want decoded object", row["hook_metadata_json"])
	}
	if got["details"] != "plain text detail" {
		t.Fatalf("details = %#v", got["details"])
	}
}

func TestNormalizeMySQLValueUnsignedIntegerRemainsNumeric(t *testing.T) {
	got := normalizeMySQLValue(uint64(52))
	if got != int64(52) {
		t.Fatalf("normalized uint64 = %#v, want int64", got)
	}
}

func TestCompareCountMismatch(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, manifest{
		TablesExported: []string{"chat_logs"},
		RowCounts:      map[string]int{"chat_logs": 2},
	})
	writeNDJSON(t, dir, "chat_logs", []map[string]any{
		{"id": 1, "chat_session_id": "sess-1", "turn_index": 1, "role": "user", "content": "hello", "created_at": "2024-01-01T00:00:00Z"},
		{"id": 2, "chat_session_id": "sess-1", "turn_index": 2, "role": "assistant", "content": "hi", "created_at": "2024-01-01T00:00:00Z"},
	})

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM `chat_logs`")).
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `chat_logs`")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "chat_session_id", "turn_index", "role", "content", "created_at"}).
			AddRow(1, "sess-1", 1, "user", "hello", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)))

	report, err := compareExport(context.Background(), db, dir)
	if err != nil {
		t.Fatalf("compareExport failed: %v", err)
	}

	tr := findTable(report.Tables, "chat_logs")
	if tr == nil {
		t.Fatal("chat_logs table result missing")
	}
	if tr.Status != "count_mismatch" {
		t.Fatalf("status = %q, want count_mismatch", tr.Status)
	}
	if tr.ExportCount != 2 || tr.MariaDBCount != 1 {
		t.Fatalf("counts wrong: export=%d, mariadb=%d", tr.ExportCount, tr.MariaDBCount)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCompareChecksumMatchTwoTables(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, manifest{
		TablesExported: []string{"chat_logs", "kg_triples"},
		RowCounts:      map[string]int{"chat_logs": 1, "kg_triples": 1},
	})
	writeNDJSON(t, dir, "chat_logs", []map[string]any{{
		"id":              1,
		"chat_session_id": "sess-1",
		"turn_index":      1,
		"role":            "user",
		"content":         "hello",
		"created_at":      "2024-01-01T00:00:00Z",
	}})
	writeNDJSON(t, dir, "kg_triples", []map[string]any{{
		"id":              1,
		"chat_session_id": "sess-1",
		"subject":         "Alice",
		"predicate":       "knows",
		"object":          "Bob",
		"valid_from":      1,
		"valid_to":        nil,
		"source_turn":     1,
		"created_at":      "2024-01-01T00:00:00Z",
	}})

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM `chat_logs`")).
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `chat_logs`")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "chat_session_id", "turn_index", "role", "content", "created_at"}).
			AddRow(1, "sess-1", 1, "user", "hello", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)))

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM `kg_triples`")).
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `kg_triples`")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "chat_session_id", "subject", "predicate", "object", "valid_from", "valid_to", "source_turn", "created_at"}).
			AddRow(1, "sess-1", "Alice", "knows", "Bob", 1, nil, 1, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)))

	report, err := compareExport(context.Background(), db, dir)
	if err != nil {
		t.Fatalf("compareExport failed: %v", err)
	}

	tr1 := findTable(report.Tables, "chat_logs")
	if tr1 == nil || tr1.Status != "ok" || tr1.ChecksumMatch == nil || !*tr1.ChecksumMatch {
		t.Fatalf("chat_logs mismatch: %+v", tr1)
	}
	tr2 := findTable(report.Tables, "kg_triples")
	if tr2 == nil || tr2.Status != "ok" || tr2.ChecksumMatch == nil || !*tr2.ChecksumMatch {
		t.Fatalf("kg_triples mismatch: %+v", tr2)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCompareStorylinesJSONMatch(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, manifest{
		TablesExported: []string{"storylines"},
		RowCounts:      map[string]int{"storylines": 1},
	})
	writeNDJSON(t, dir, "storylines", []map[string]any{{
		"id":                    1,
		"chat_session_id":       "sess-1",
		"name":                  "The Beginning",
		"status":                "active",
		"entities_json":         `{"protagonist":"Alice"}`,
		"key_points_json":       `["point1"]`,
		"ongoing_tensions_json": `["tension1"]`,
		"created_at":            "2024-01-01T00:00:00Z",
	}})

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM `storylines`")).
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `storylines`")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "chat_session_id", "name", "status", "entities_json", "key_points_json", "ongoing_tensions_json", "created_at"}).
			AddRow(1, "sess-1", "The Beginning", "active", `{"protagonist":"Alice"}`, `["point1"]`, `["tension1"]`, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)))

	report, err := compareExport(context.Background(), db, dir)
	if err != nil {
		t.Fatalf("compareExport failed: %v", err)
	}

	tr := findTable(report.Tables, "storylines")
	if tr == nil {
		t.Fatal("storylines table result missing")
	}
	if tr.Status != "ok" {
		t.Fatalf("status = %q, want ok, error=%q", tr.Status, tr.Error)
	}
	if tr.ExportCount != 1 || tr.MariaDBCount != 1 {
		t.Fatalf("counts mismatch: export=%d, mariadb=%d", tr.ExportCount, tr.MariaDBCount)
	}
	if tr.ChecksumMatch == nil || !*tr.ChecksumMatch {
		t.Fatalf("checksum mismatch: export=%s, mariadb=%s", tr.ExportChecksum, tr.MariaDBChecksum)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestComparePendingThreadsNormalized(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, manifest{
		TablesExported: []string{"pending_threads"},
		RowCounts:      map[string]int{"pending_threads": 1},
	})
	// Write old export shape; normalization should map it to schema shape
	writeNDJSON(t, dir, "pending_threads", []map[string]any{{
		"id":              1,
		"chat_session_id": "sess-1",
		"thread_type":     "continuity",
		"title":           "Lost Key",
		"status":          "open",
		"source_turn":     5,
		"last_seen_turn":  10,
		"confidence":      0.75,
		"details_json":    `{"hint":"check drawer"}`,
		"pinned":          true,
		"suppressed":      false,
		"user_corrected":  false,
		"created_at":      "2024-01-01T00:00:00Z",
		"updated_at":      "2024-01-01T00:00:00Z",
	}})

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectQuery(regexp.QuoteMeta("SELECT COUNT(*) FROM `pending_threads`")).
		WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(1))
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `pending_threads`")).
		WillReturnRows(sqlmock.NewRows([]string{"id", "chat_session_id", "thread_key", "description", "status", "created_turn", "resolved_turn", "source_turn", "priority", "hook_type", "hook_metadata_json", "pinned", "suppressed", "user_corrected", "created_at", "updated_at"}).
			AddRow(1, "sess-1", "Lost Key", "Lost Key", "open", 5, 10, 5, 75, "continuity", `{"hint":"check drawer"}`, true, false, false, time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)))

	report, err := compareExport(context.Background(), db, dir)
	if err != nil {
		t.Fatalf("compareExport failed: %v", err)
	}

	tr := findTable(report.Tables, "pending_threads")
	if tr == nil {
		t.Fatal("pending_threads table result missing")
	}
	if tr.Status != "ok" {
		t.Fatalf("status = %q, want ok, error=%q", tr.Status, tr.Error)
	}
	if tr.ExportCount != 1 || tr.MariaDBCount != 1 {
		t.Fatalf("counts mismatch: export=%d, mariadb=%d", tr.ExportCount, tr.MariaDBCount)
	}
	if tr.ChecksumMatch == nil || !*tr.ChecksumMatch {
		t.Fatalf("checksum mismatch: export=%s, mariadb=%s", tr.ExportChecksum, tr.MariaDBChecksum)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCompareMissingCanonicalExport(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, manifest{
		TablesExported: []string{"chat_logs"},
		RowCounts:      map[string]int{"chat_logs": 0},
	})

	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	report, err := compareExport(context.Background(), db, dir)
	if err != nil {
		t.Fatalf("compareExport failed: %v", err)
	}

	tr := findTable(report.Tables, "effective_input_logs")
	if tr == nil {
		t.Fatal("effective_input_logs table result missing")
	}
	if tr.Status != "missing_export" {
		t.Fatalf("status = %q, want missing_export", tr.Status)
	}
	if report.Status != "failed" {
		t.Fatalf("report status = %q, want failed", report.Status)
	}
	if len(report.Errors) == 0 {
		t.Fatal("expected errors for missing tables")
	}
}

func writeManifest(t *testing.T, dir string, mf manifest) {
	t.Helper()
	data, err := json.Marshal(mf)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

func writeNDJSON(t *testing.T, dir, table string, rows []map[string]any) {
	t.Helper()
	f, err := os.Create(filepath.Join(dir, table+".ndjson"))
	if err != nil {
		t.Fatalf("create ndjson: %v", err)
	}
	defer f.Close()

	meta := map[string]any{"_export_meta": map[string]any{"table_name": table, "row_count": len(rows)}}
	writeJSONLine(t, f, meta)
	for _, row := range rows {
		writeJSONLine(t, f, row)
	}
}

func writeJSONLine(t *testing.T, f *os.File, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal line: %v", err)
	}
	if _, err := f.Write(append(data, '\n')); err != nil {
		t.Fatalf("write line: %v", err)
	}
}

func findTable(tables []tableCompareResult, name string) *tableCompareResult {
	for i := range tables {
		if tables[i].TableName == name {
			return &tables[i]
		}
	}
	return nil
}

func sliceContains(slice []string, substr string) bool {
	for _, s := range slice {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}
