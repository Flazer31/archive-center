package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestBuildInsertPreservesIDAndUpdatesNonIDColumns(t *testing.T) {
	query, args, err := buildInsert("chat_logs", map[string]any{
		"id":              json.Number("7"),
		"chat_session_id": "sess-1",
		"turn_index":      json.Number("3"),
		"role":            "user",
		"content":         "hello",
		"_row_checksum":   "ignored",
	})
	if err != nil {
		t.Fatalf("buildInsert failed: %v", err)
	}
	if !strings.Contains(query, "INSERT INTO `chat_logs`") {
		t.Fatalf("query does not target chat_logs: %s", query)
	}
	if !strings.Contains(query, "`id`, `chat_session_id`, `turn_index`, `role`, `content`") {
		t.Fatalf("query does not preserve expected columns: %s", query)
	}
	if !strings.Contains(query, "ON DUPLICATE KEY UPDATE") || strings.Contains(query, "`id`=VALUES(`id`)") {
		t.Fatalf("query update clause unsafe: %s", query)
	}
	if len(args) != 5 {
		t.Fatalf("args len = %d, want 5", len(args))
	}
	if args[0] != int64(7) || args[2] != int64(3) {
		t.Fatalf("json numbers were not normalized: %#v", args)
	}
}

func TestBuildInsertJSONColumnsMarshalStructuredValues(t *testing.T) {
	query, args, err := buildInsert("memories", map[string]any{
		"chat_session_id": "sess-1",
		"turn_index":      json.Number("1"),
		"summary_json": map[string]any{
			"summary": "ok",
		},
		"embedding": []any{json.Number("0.1"), json.Number("0.2")},
	})
	if err != nil {
		t.Fatalf("buildInsert failed: %v", err)
	}
	if !strings.Contains(query, "`summary_json`") || !strings.Contains(query, "`embedding`") {
		t.Fatalf("query missing JSON columns: %s", query)
	}
	if args[2] != `{"summary":"ok"}` {
		t.Fatalf("summary_json arg = %#v", args[2])
	}
	if args[3] != `[0.1,0.2]` {
		t.Fatalf("embedding arg = %#v", args[3])
	}
}

func TestNormalizeValueConvertsRFC3339DateTime(t *testing.T) {
	got := normalizeValue("chat_logs", "created_at", "2026-05-19T05:01:04Z")
	if got != "2026-05-19 05:01:04.000" {
		t.Fatalf("normalized datetime = %#v, want MySQL DATETIME(3)", got)
	}
}

func TestNormalizePendingThreadsWrapsTextDetailsAsJSON(t *testing.T) {
	row := normalizeExportRow("pending_threads", map[string]any{
		"title":        "open loop",
		"details_json": "plain text detail",
	})
	query, args, err := buildInsert("pending_threads", row)
	if err != nil {
		t.Fatalf("buildInsert failed: %v", err)
	}
	if !strings.Contains(query, "`hook_metadata_json`") {
		t.Fatalf("query missing hook_metadata_json: %s", query)
	}
	found := false
	for _, arg := range args {
		if arg == `{"details":"plain text detail"}` {
			found = true
		}
	}
	if !found {
		t.Fatalf("hook_metadata_json was not wrapped as JSON object: %#v", args)
	}
}

func TestImportExportExecutesCanonicalRows(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, manifest{
		TablesExported:       []string{"chat_logs"},
		SkippedMissingTables: allCanonicalExcept("chat_logs"),
		RowCounts:            map[string]int{"chat_logs": 1},
	})
	writeNDJSON(t, dir, "chat_logs", []map[string]any{{
		"id":              1,
		"chat_session_id": "sess-1",
		"turn_index":      1,
		"role":            "user",
		"content":         "hello",
		"_row_checksum":   "ignored",
	}})

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `chat_logs`")).
		WithArgs(int64(1), "sess-1", int64(1), "user", "hello").
		WillReturnResult(sqlmock.NewResult(1, 1))

	report, err := importExport(context.Background(), db, dir, true)
	if err != nil {
		t.Fatalf("importExport failed: %v", err)
	}
	if report.Status != "ok" {
		t.Fatalf("status = %q, want ok: %+v", report.Status, report)
	}
	if report.TotalRows != 1 {
		t.Fatalf("total rows = %d, want 1", report.TotalRows)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestImportExportFailsMissingCanonicalTable(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, manifest{
		TablesExported: []string{"chat_logs"},
		RowCounts:      map[string]int{"chat_logs": 0},
	})
	writeNDJSON(t, dir, "chat_logs", nil)

	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	report, err := importExport(context.Background(), db, dir, true)
	if err != nil {
		t.Fatalf("importExport returned unexpected top-level error: %v", err)
	}
	if report.Status != "failed" {
		t.Fatalf("status = %q, want failed", report.Status)
	}
	if len(report.Errors) == 0 {
		t.Fatal("expected missing table errors")
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

func allCanonicalExcept(except string) []string {
	var out []string
	for _, table := range canonicalTables {
		if table != except {
			out = append(out, table)
		}
	}
	return out
}

func parseSchemaColumns(path string) (map[string][]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	columnDefRe := regexp.MustCompile("(?i)^\\s*`?(\\w+)`?\\s+(BIGINT|INT|VARCHAR|LONGTEXT|TEXT|JSON|DATETIME|BOOLEAN|FLOAT|DOUBLE|CHAR|DECIMAL|TINYINT|SMALLINT|MEDIUMINT|DATE|TIME|YEAR|BLOB|ENUM|SET)\\b")
	createTableRe := regexp.MustCompile(`(?i)CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(\w+)`)

	result := make(map[string][]string)
	var currentTable string
	var currentCols []string
	inTable := false
	parenDepth := 0

	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}

		if !inTable {
			matches := createTableRe.FindStringSubmatch(trimmed)
			if len(matches) > 1 {
				currentTable = matches[1]
				inTable = true
				currentCols = nil
				parenDepth = strings.Count(trimmed, "(") - strings.Count(trimmed, ")")
			}
			continue
		}

		parenDepth += strings.Count(trimmed, "(") - strings.Count(trimmed, ")")

		if parenDepth <= 0 {
			result[currentTable] = currentCols
			inTable = false
			currentTable = ""
			currentCols = nil
			continue
		}

		if idx := strings.Index(trimmed, "--"); idx >= 0 {
			trimmed = strings.TrimSpace(trimmed[:idx])
		}
		if trimmed == "" {
			continue
		}

		colMatch := columnDefRe.FindStringSubmatch(trimmed)
		if len(colMatch) > 1 {
			currentCols = append(currentCols, colMatch[1])
		}
	}

	return result, nil
}

func TestTableColumnsMatchSchema(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	schemaPath := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "migrations", "001_schema.sql")
	schemaCols, err := parseSchemaColumns(schemaPath)
	if err != nil {
		t.Fatalf("parseSchemaColumns(%q): %v", schemaPath, err)
	}

	for _, table := range canonicalTables {
		expected, ok := schemaCols[table]
		if !ok {
			t.Fatalf("schema missing table %q", table)
		}
		actual, ok := tableColumns[table]
		if !ok {
			t.Fatalf("tableColumns missing table %q", table)
		}
		if len(expected) != len(actual) {
			t.Errorf("table %q: column count mismatch: schema has %d, tableColumns has %d", table, len(expected), len(actual))
			continue
		}
		for i := range expected {
			if expected[i] != actual[i] {
				t.Errorf("table %q: column %d mismatch: schema=%q, tableColumns=%q", table, i, expected[i], actual[i])
			}
		}
	}

	oldPendingCols := []string{"thread_type", "title", "details_json", "resolution_note", "owner", "target", "last_seen_turn", "confidence"}
	for _, col := range oldPendingCols {
		for _, tc := range tableColumns["pending_threads"] {
			if tc == col {
				t.Errorf("pending_threads tableColumns must not contain old export-only column %q", col)
			}
		}
	}

	for _, table := range canonicalTables {
		for _, col := range tableColumns[table] {
			if col == "arc_id" {
				t.Errorf("table %q tableColumns must not contain obsolete column %q", table, col)
			}
		}
	}
	for _, col := range tableColumns["world_rules"] {
		if col == "rule_category" {
			t.Errorf("world_rules tableColumns must not contain obsolete column %q", col)
		}
	}
}
