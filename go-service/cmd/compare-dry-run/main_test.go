package main

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func createTestDB(t *testing.T, tables map[string][]string, rows map[string]int) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	defer db.Close()

	for table, cols := range tables {
		var colDefs string
		if len(cols) > 0 {
			colDefs = "\"" + cols[0] + "\" TEXT"
			for i := 1; i < len(cols); i++ {
				colDefs += ", \"" + cols[i] + "\" TEXT"
			}
		} else {
			colDefs = "id INTEGER PRIMARY KEY"
		}
		_, err := db.Exec("CREATE TABLE \"" + table + "\" (" + colDefs + ")")
		if err != nil {
			t.Fatalf("creating table %s: %v", table, err)
		}
		for i := 0; i < rows[table]; i++ {
			_, err := db.Exec("INSERT INTO \"" + table + "\" DEFAULT VALUES")
			if err != nil {
				t.Fatalf("inserting into %s: %v", table, err)
			}
		}
	}
	return dbPath
}

func writeReport(t *testing.T, dir string, report dryRunReport) string {
	t.Helper()
	path := filepath.Join(dir, "report.json")
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSuccessAllMatch(t *testing.T) {
	tables := map[string][]string{"chat_logs": []string{"content"}}
	rows := map[string]int{"chat_logs": 2}
	dbPath := createTestDB(t, tables, rows)

	report := dryRunReport{
		Status: "ok",
		Tables: []tableReport{
			{
				TableName:          "chat_logs",
				RowsDiscovered:     2,
				ChecksumExpected:   "abc123",
				ChecksumCalculated: "abc123",
			},
		},
	}
	reportPath := writeReport(t, t.TempDir(), report)

	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	result, err := compare(db, &report)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("expected ok, got %s, errors=%v", result.Status, result.Errors)
	}
	if len(result.Details) != 1 || !result.Details[0].RowCountMatch || !result.Details[0].ChecksumMatch {
		t.Fatalf("unexpected details: %+v", result.Details)
	}
	_ = reportPath
}

func TestRowCountMismatch(t *testing.T) {
	tables := map[string][]string{"chat_logs": []string{"content"}}
	rows := map[string]int{"chat_logs": 3}
	dbPath := createTestDB(t, tables, rows)

	report := dryRunReport{
		Status: "ok",
		Tables: []tableReport{
			{
				TableName:          "chat_logs",
				RowsDiscovered:     2,
				ChecksumExpected:   "abc123",
				ChecksumCalculated: "abc123",
			},
		},
	}
	reportPath := writeReport(t, t.TempDir(), report)

	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	result, err := compare(db, &report)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
	found := false
	for _, e := range result.Errors {
		if e == "table chat_logs: row count mismatch (sqlite=3, report=2)" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected row count mismatch error, got %v", result.Errors)
	}
	_ = reportPath
}

func TestMissingSQLiteTableSkippedReport(t *testing.T) {
	tables := map[string][]string{"chat_logs": []string{"content"}}
	rows := map[string]int{"chat_logs": 1}
	dbPath := createTestDB(t, tables, rows)

	report := dryRunReport{
		Status:               "ok",
		SkippedMissingTables: []string{"effective_input_logs"},
		Tables: []tableReport{
			{
				TableName:          "chat_logs",
				RowsDiscovered:     1,
				ChecksumExpected:   "abc123",
				ChecksumCalculated: "abc123",
			},
		},
	}
	reportPath := writeReport(t, t.TempDir(), report)

	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	result, err := compare(db, &report)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("expected ok, got %s, errors=%v", result.Status, result.Errors)
	}
	found := false
	for _, w := range result.Warnings {
		if w == "table effective_input_logs: missing in both SQLite and report (skipped)" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected skipped missing warning, got %v", result.Warnings)
	}
	_ = reportPath
}

func TestPresentSQLiteTableSkippedReportFails(t *testing.T) {
	tables := map[string][]string{
		"chat_logs":            []string{"content"},
		"effective_input_logs": []string{"content"},
	}
	rows := map[string]int{"chat_logs": 1, "effective_input_logs": 1}
	dbPath := createTestDB(t, tables, rows)

	report := dryRunReport{
		Status:               "ok",
		SkippedMissingTables: []string{"effective_input_logs"},
		Tables: []tableReport{
			{
				TableName:          "chat_logs",
				RowsDiscovered:     1,
				ChecksumExpected:   "abc123",
				ChecksumCalculated: "abc123",
			},
		},
	}
	reportPath := writeReport(t, t.TempDir(), report)

	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	result, err := compare(db, &report)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
	found := false
	for _, e := range result.Errors {
		if e == "table effective_input_logs: present in SQLite but skipped in report" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected present skipped-table error, got %v", result.Errors)
	}
	_ = reportPath
}

func TestCanonicalSQLiteTableMissingFromReportFails(t *testing.T) {
	tables := map[string][]string{
		"chat_logs": []string{"content"},
		"memories":  []string{"content"},
	}
	rows := map[string]int{"chat_logs": 1, "memories": 1}
	dbPath := createTestDB(t, tables, rows)

	report := dryRunReport{
		Status: "ok",
		Tables: []tableReport{
			{
				TableName:          "chat_logs",
				RowsDiscovered:     1,
				ChecksumExpected:   "abc123",
				ChecksumCalculated: "abc123",
			},
		},
	}
	reportPath := writeReport(t, t.TempDir(), report)

	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	result, err := compare(db, &report)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
	found := false
	for _, e := range result.Errors {
		if e == "canonical table memories: present in SQLite but missing from report" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected canonical missing-table error, got %v", result.Errors)
	}
	_ = reportPath
}

func TestChecksumMismatch(t *testing.T) {
	tables := map[string][]string{"chat_logs": []string{"content"}}
	rows := map[string]int{"chat_logs": 1}
	dbPath := createTestDB(t, tables, rows)

	report := dryRunReport{
		Status: "ok",
		Tables: []tableReport{
			{
				TableName:          "chat_logs",
				RowsDiscovered:     1,
				ChecksumExpected:   "expected123",
				ChecksumCalculated: "calculated456",
			},
		},
	}
	reportPath := writeReport(t, t.TempDir(), report)

	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	result, err := compare(db, &report)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
	found := false
	for _, e := range result.Errors {
		if e == "table chat_logs: checksum mismatch (expected=expected123, calculated=calculated456)" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected checksum mismatch error, got %v", result.Errors)
	}
	_ = reportPath
}

func TestInvalidReportJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := loadReport(path)
	if err == nil {
		t.Fatal("expected error for invalid json")
	}
}

func TestOldReportWithoutNoncanonicalTablesField(t *testing.T) {
	// Simulate a report JSON produced by an older validator that lacks the
	// noncanonical_tables field entirely.
	const oldReportJSON = `{
  "status": "ok",
  "export_dir": "/tmp/export",
  "checked_at": "2026-01-01T00:00:00Z",
  "tables": [
    {
      "table_name": "chat_logs",
      "rows_discovered": 1,
      "rows_accepted": 1,
      "rows_rejected": 0,
      "checksum_expected": "abc123",
      "checksum_calculated": "abc123",
      "errors": []
    }
  ],
  "skipped_missing_tables": [],
  "ignored_noncanonical_tables": [],
  "summary": {
    "tables_checked": 1,
    "tables_passed": 1,
    "tables_failed": 0,
    "total_rows_discovered": 1,
    "total_rows_accepted": 1,
    "total_rows_rejected": 0
  }
}`

	tables := map[string][]string{"chat_logs": []string{"content"}}
	rows := map[string]int{"chat_logs": 1}
	dbPath := createTestDB(t, tables, rows)

	dir := t.TempDir()
	reportPath := filepath.Join(dir, "old-report.json")
	if err := os.WriteFile(reportPath, []byte(oldReportJSON), 0644); err != nil {
		t.Fatal(err)
	}

	report, err := loadReport(reportPath)
	if err != nil {
		t.Fatalf("loadReport failed: %v", err)
	}
	if report.NoncanonicalTables != nil {
		t.Fatalf("expected nil NoncanonicalTables, got %v", report.NoncanonicalTables)
	}

	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	result, err := compare(db, report)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("expected ok, got %s, errors=%v", result.Status, result.Errors)
	}
	if len(result.Details) != 1 {
		t.Fatalf("expected 1 detail, got %d", len(result.Details))
	}
	for _, d := range result.Details {
		if d.TableName == "chat_logs" && (!d.RowCountMatch || !d.ChecksumMatch) {
			t.Errorf("chat_logs should match: %+v", d)
		}
	}
}

func TestNoncanonicalTableInReportCompared(t *testing.T) {
	tables := map[string][]string{
		"chat_logs":         []string{"content"},
		"episode_summaries": []string{"title"},
	}
	rows := map[string]int{"chat_logs": 1, "episode_summaries": 2}
	dbPath := createTestDB(t, tables, rows)

	report := dryRunReport{
		Status: "ok",
		Tables: []tableReport{
			{
				TableName:          "chat_logs",
				RowsDiscovered:     1,
				ChecksumExpected:   "abc123",
				ChecksumCalculated: "abc123",
			},
		},
		NoncanonicalTables: []tableReport{
			{
				TableName:          "episode_summaries",
				RowsDiscovered:     2,
				ChecksumExpected:   "def456",
				ChecksumCalculated: "def456",
			},
		},
	}

	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	result, err := compare(db, &report)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "ok" {
		t.Fatalf("expected ok, got %s, errors=%v", result.Status, result.Errors)
	}
	if len(result.Details) != 2 {
		t.Fatalf("expected 2 details, got %d", len(result.Details))
	}
	foundCanonical := false
	foundNoncanonical := false
	for _, d := range result.Details {
		if d.TableName == "chat_logs" {
			foundCanonical = true
			if !d.RowCountMatch || !d.ChecksumMatch {
				t.Errorf("chat_logs should match: %+v", d)
			}
		}
		if d.TableName == "episode_summaries" {
			foundNoncanonical = true
			if !d.RowCountMatch || !d.ChecksumMatch {
				t.Errorf("episode_summaries should match: %+v", d)
			}
		}
	}
	if !foundCanonical {
		t.Error("expected chat_logs in details")
	}
	if !foundNoncanonical {
		t.Error("expected episode_summaries in details")
	}
}

func TestNoncanonicalTableRowCountMismatch(t *testing.T) {
	tables := map[string][]string{
		"episode_summaries": []string{"title"},
	}
	rows := map[string]int{"episode_summaries": 3}
	dbPath := createTestDB(t, tables, rows)

	report := dryRunReport{
		Status: "ok",
		NoncanonicalTables: []tableReport{
			{
				TableName:          "episode_summaries",
				RowsDiscovered:     2,
				ChecksumExpected:   "def456",
				ChecksumCalculated: "def456",
			},
		},
	}

	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	result, err := compare(db, &report)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
	found := false
	for _, e := range result.Errors {
		if e == "table episode_summaries: row count mismatch (sqlite=3, report=2)" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected row count mismatch error, got %v", result.Errors)
	}
}

func TestNoncanonicalTableChecksumMismatch(t *testing.T) {
	tables := map[string][]string{"episode_summaries": []string{"title"}}
	rows := map[string]int{"episode_summaries": 2}
	dbPath := createTestDB(t, tables, rows)

	report := dryRunReport{
		Status: "ok",
		NoncanonicalTables: []tableReport{
			{
				TableName:          "episode_summaries",
				RowsDiscovered:     2,
				ChecksumExpected:   "expected789",
				ChecksumCalculated: "actual321",
			},
		},
	}

	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	result, err := compare(db, &report)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "failed" {
		t.Fatalf("expected failed, got %s", result.Status)
	}
	found := false
	for _, e := range result.Errors {
		if e == "table episode_summaries: checksum mismatch (expected=expected789, calculated=actual321)" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected checksum mismatch error, got %v", result.Errors)
	}
}
