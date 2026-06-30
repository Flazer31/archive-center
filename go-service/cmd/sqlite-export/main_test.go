package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"database/sql"

	_ "modernc.org/sqlite"
)

func createTempDB(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open temp db: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE chat_logs (
		id INTEGER PRIMARY KEY,
		session_id TEXT,
		message TEXT
	)`); err != nil {
		t.Fatalf("create chat_logs: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE memories (
		id INTEGER PRIMARY KEY,
		content TEXT,
		embedding BLOB
	)`); err != nil {
		t.Fatalf("create memories: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO chat_logs (session_id, message) VALUES ('s1', 'hello')`); err != nil {
		t.Fatalf("insert chat_logs: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO memories (content) VALUES ('memory1')`); err != nil {
		t.Fatalf("insert memories: %v", err)
	}

	return dbPath, func() {}
}

func createTempDBWithOneTable(t *testing.T) (string, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open temp db: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE chat_logs (id INTEGER PRIMARY KEY, msg TEXT)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO chat_logs (msg) VALUES ('a')`); err != nil {
		t.Fatalf("insert: %v", err)
	}

	return dbPath, func() {}
}

func TestExportMultipleTables(t *testing.T) {
	dbPath, cleanup := createTempDB(t)
	defer cleanup()

	outDir := t.TempDir()

	if err := runExport(dbPath, outDir, true, false); err != nil {
		t.Fatalf("runExport failed: %v", err)
	}

	for _, table := range []string{"chat_logs", "memories"} {
		path := filepath.Join(outDir, table+".ndjson")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Fatalf("missing ndjson for %s", table)
		}
	}

	manifestPath := filepath.Join(outDir, "manifest.json")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest map[string]interface{}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}

	if manifest["source_db_path"] != dbPath {
		t.Errorf("source_db_path mismatch")
	}
	if manifest["mode"] != "canonical-only" {
		t.Errorf("mode mismatch")
	}

	exported, ok := manifest["tables_exported"].([]interface{})
	if !ok || len(exported) != 2 {
		t.Fatalf("tables_exported expected 2, got %v", exported)
	}

	rowCounts, ok := manifest["row_counts"].(map[string]interface{})
	if !ok {
		t.Fatalf("row_counts type mismatch")
	}
	if rowCounts["chat_logs"] != float64(1) || rowCounts["memories"] != float64(1) {
		t.Errorf("row_counts mismatch: %v", rowCounts)
	}

	chatPath := filepath.Join(outDir, "chat_logs.ndjson")
	lines, err := readLines(chatPath)
	if err != nil {
		t.Fatalf("read chat_logs.ndjson: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	var metaWrapper map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &metaWrapper); err != nil {
		t.Fatalf("unmarshal meta: %v", err)
	}
	meta, ok := metaWrapper["_export_meta"].(map[string]interface{})
	if !ok {
		t.Fatalf("missing _export_meta")
	}
	if meta["table_name"] != "chat_logs" {
		t.Errorf("table_name mismatch")
	}
	if meta["row_count"] != float64(1) {
		t.Errorf("row_count mismatch")
	}
	cols, ok := meta["columns"].([]interface{})
	if !ok || len(cols) != 3 {
		t.Errorf("columns mismatch")
	}

	var row map[string]interface{}
	if err := json.Unmarshal([]byte(lines[1]), &row); err != nil {
		t.Fatalf("unmarshal row: %v", err)
	}
	if row["session_id"] != "s1" || row["message"] != "hello" {
		t.Errorf("row data mismatch")
	}
	if _, ok := row["_row_checksum"]; !ok {
		t.Errorf("missing _row_checksum")
	}
	if len(row["_row_checksum"].(string)) != 64 {
		t.Errorf("_row_checksum not 64 hex chars")
	}
}

func TestAllModeExportsNoncanonicalTables(t *testing.T) {
	dbPath, cleanup := createTempDB(t)
	defer cleanup()

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE extra_runtime_table (id INTEGER PRIMARY KEY, value TEXT)`); err != nil {
		db.Close()
		t.Fatalf("create extra table: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO extra_runtime_table (value) VALUES ('extra')`); err != nil {
		db.Close()
		t.Fatalf("insert extra table: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	outDir := t.TempDir()
	if err := runExport(dbPath, outDir, false, true); err != nil {
		t.Fatalf("runExport all mode failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(outDir, "extra_runtime_table.ndjson")); err != nil {
		t.Fatalf("expected noncanonical table export: %v", err)
	}

	manifestBytes, err := os.ReadFile(filepath.Join(outDir, "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest map[string]interface{}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if manifest["mode"] != "all" {
		t.Fatalf("mode mismatch: %v", manifest["mode"])
	}
	skipped, ok := manifest["skipped_missing_tables"].([]interface{})
	if !ok || len(skipped) != 0 {
		t.Fatalf("all mode should not skip canonical tables: %v", skipped)
	}
}

func TestMissingCanonicalTableSkipped(t *testing.T) {
	dbPath, cleanup := createTempDBWithOneTable(t)
	defer cleanup()

	outDir := t.TempDir()
	if err := runExport(dbPath, outDir, true, false); err != nil {
		t.Fatalf("runExport failed: %v", err)
	}

	manifestPath := filepath.Join(outDir, "manifest.json")
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var manifest map[string]interface{}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}

	skipped, ok := manifest["skipped_missing_tables"].([]interface{})
	if !ok {
		t.Fatalf("skipped_missing_tables type mismatch")
	}

	expectedSkipped := []string{
		"effective_input_logs",
		"memories",
		"direct_evidence_records",
		"kg_triples",
		"audit_logs",
		"critic_feedback",
		"character_events",
	}
	for _, exp := range expectedSkipped {
		found := false
		for _, s := range skipped {
			if s == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %s in skipped_missing_tables", exp)
		}
	}

	exported, ok := manifest["tables_exported"].([]interface{})
	if !ok || len(exported) != 1 || exported[0] != "chat_logs" {
		t.Errorf("tables_exported mismatch: %v", exported)
	}
}

func TestMutuallyExclusiveFlags(t *testing.T) {
	outDir := t.TempDir()
	dbPath, _ := createTempDBWithOneTable(t)

	err := runExport(dbPath, outDir, true, true)
	if err == nil {
		t.Fatal("expected error for mutually exclusive flags")
	}
	if !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRequiredFlags(t *testing.T) {
	if err := runExport("", "out", true, false); err == nil {
		t.Fatal("expected error for empty dbPath")
	}
	if err := runExport("db", "", true, false); err == nil {
		t.Fatal("expected error for empty outDir")
	}
}

func TestReadOnlyBehavior(t *testing.T) {
	dbPath, cleanup := createTempDB(t)
	defer cleanup()

	preInfo, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("stat db: %v", err)
	}

	outDir := t.TempDir()
	if err := runExport(dbPath, outDir, true, false); err != nil {
		t.Fatalf("runExport failed: %v", err)
	}

	postInfo, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("stat db after export: %v", err)
	}

	if preInfo.ModTime() != postInfo.ModTime() || preInfo.Size() != postInfo.Size() {
		t.Error("source DB was modified during export")
	}
}

func TestDeterministicChecksumShape(t *testing.T) {
	dbPath, cleanup := createTempDB(t)
	defer cleanup()

	outDir1 := t.TempDir()
	outDir2 := t.TempDir()

	if err := runExport(dbPath, outDir1, true, false); err != nil {
		t.Fatalf("runExport 1 failed: %v", err)
	}
	if err := runExport(dbPath, outDir2, true, false); err != nil {
		t.Fatalf("runExport 2 failed: %v", err)
	}

	for _, table := range []string{"chat_logs", "memories"} {
		lines1, err := readLines(filepath.Join(outDir1, table+".ndjson"))
		if err != nil {
			t.Fatalf("read %s 1: %v", table, err)
		}
		lines2, err := readLines(filepath.Join(outDir2, table+".ndjson"))
		if err != nil {
			t.Fatalf("read %s 2: %v", table, err)
		}

		if len(lines1) != len(lines2) {
			t.Fatalf("line count mismatch for %s", table)
		}

		for i := 1; i < len(lines1); i++ {
			var r1, r2 map[string]interface{}
			json.Unmarshal([]byte(lines1[i]), &r1)
			json.Unmarshal([]byte(lines2[i]), &r2)

			c1 := r1["_row_checksum"].(string)
			c2 := r2["_row_checksum"].(string)
			if c1 != c2 {
				t.Errorf("row checksum mismatch for %s row %d", table, i)
			}
		}

		m1, _ := os.ReadFile(filepath.Join(outDir1, "manifest.json"))
		m2, _ := os.ReadFile(filepath.Join(outDir2, "manifest.json"))
		var man1, man2 map[string]interface{}
		json.Unmarshal(m1, &man1)
		json.Unmarshal(m2, &man2)

		cs1 := man1["checksums"].(map[string]interface{})
		cs2 := man2["checksums"].(map[string]interface{})
		if cs1[table] != cs2[table] {
			t.Errorf("table checksum mismatch for %s", table)
		}
	}
}

func readLines(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines, nil
}
