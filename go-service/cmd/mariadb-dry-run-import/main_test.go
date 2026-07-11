package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildPlanAllTablesPresent(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, manifest{
		TablesExported: allCanonicalTables(),
		RowCounts: map[string]int{
			"chat_logs":               2,
			"effective_input_logs":    0,
			"memories":                1,
			"direct_evidence_records": 0,
			"kg_triples":              0,
			"audit_logs":              0,
			"critic_feedback":         0,
			"character_events":        0,
		},
		Checksums: map[string]string{
			"chat_logs":               "abc123",
			"effective_input_logs":    "",
			"memories":                "def456",
			"direct_evidence_records": "",
			"kg_triples":              "",
			"audit_logs":              "",
			"critic_feedback":         "",
			"character_events":        "",
		},
	})
	for _, tableName := range allCanonicalTables() {
		var checksum string
		if tableName == "chat_logs" {
			checksum = "abc123"
		} else if tableName == "memories" {
			checksum = "def456"
		}
		writeNDJSON(t, dir, tableName, exportMeta{TableName: tableName, RowCount: 0, TableChecksum: checksum})
	}
	// Override chat_logs row count to match manifest
	writeNDJSON(t, dir, "chat_logs", exportMeta{TableName: "chat_logs", RowCount: 2, TableChecksum: "abc123"})
	writeNDJSON(t, dir, "memories", exportMeta{TableName: "memories", RowCount: 1, TableChecksum: "def456"})

	plan, err := buildPlan(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if plan.Status != "ok" {
		t.Errorf("status = %q, want ok", plan.Status)
	}
	if plan.TotalRows != 3 {
		t.Errorf("total_rows = %d, want 3", plan.TotalRows)
	}

	for _, tp := range plan.Tables {
		if tp.TableName == "chat_logs" {
			if tp.Status != "planned" {
				t.Errorf("chat_logs status = %q, want planned", tp.Status)
			}
			if tp.RowCount != 2 {
				t.Errorf("chat_logs row_count = %d, want 2", tp.RowCount)
			}
		}
		if tp.TableName == "memories" {
			if tp.Status != "planned" {
				t.Errorf("memories status = %q, want planned", tp.Status)
			}
			if tp.RowCount != 1 {
				t.Errorf("memories row_count = %d, want 1", tp.RowCount)
			}
		}
		if tp.PlannedOperation != "insert_or_update" {
			t.Errorf("table %q planned_operation = %q, want insert_or_update", tp.TableName, tp.PlannedOperation)
		}
		if tp.SampleStatementShape == "" {
			t.Errorf("table %q sample_statement_shape empty", tp.TableName)
		}
	}

	if len(plan.Errors) != 0 {
		t.Errorf("expected 0 errors, got %v", plan.Errors)
	}
}

func TestBuildPlanMissingCanonicalTable(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, manifest{
		TablesExported: []string{"chat_logs"},
		RowCounts:      map[string]int{"chat_logs": 1},
	})
	writeNDJSON(t, dir, "chat_logs", exportMeta{TableName: "chat_logs", RowCount: 1})

	plan, err := buildPlan(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if plan.Status != "failed" {
		t.Errorf("status = %q, want failed", plan.Status)
	}
	if len(plan.Errors) == 0 {
		t.Error("expected errors for missing canonical tables")
	}

	foundMissing := false
	for _, tp := range plan.Tables {
		if tp.Status == "missing" {
			foundMissing = true
			if tp.Error == "" {
				t.Error("missing table should have an error message")
			}
		}
	}
	if !foundMissing {
		t.Error("expected at least one table with status missing")
	}
}

func TestBuildPlanSkippedTable(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, manifest{
		TablesExported:       allCanonicalTables(),
		SkippedMissingTables: []string{"memories"},
		RowCounts: map[string]int{
			"chat_logs":               1,
			"effective_input_logs":    0,
			"memories":                0,
			"direct_evidence_records": 0,
			"kg_triples":              0,
			"audit_logs":              0,
			"critic_feedback":         0,
			"character_events":        0,
		},
	})
	for _, tableName := range allCanonicalTables() {
		if tableName == "memories" {
			continue
		}
		writeNDJSON(t, dir, tableName, exportMeta{TableName: tableName, RowCount: 0})
	}
	writeNDJSON(t, dir, "chat_logs", exportMeta{TableName: "chat_logs", RowCount: 1})

	plan, err := buildPlan(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundSkipped := false
	for _, tp := range plan.Tables {
		if tp.TableName == "memories" && tp.Status == "skipped" {
			foundSkipped = true
		}
	}
	if !foundSkipped {
		t.Error("expected memories to be marked skipped")
	}
	if len(plan.Errors) != 0 {
		t.Errorf("expected 0 errors when missing table is skipped, got %v", plan.Errors)
	}
}

func TestBuildPlanRowCountMismatch(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, manifest{
		TablesExported: allCanonicalTables(),
		RowCounts: map[string]int{
			"chat_logs":               99,
			"effective_input_logs":    0,
			"memories":                0,
			"direct_evidence_records": 0,
			"kg_triples":              0,
			"audit_logs":              0,
			"critic_feedback":         0,
			"character_events":        0,
		},
	})
	for _, tableName := range allCanonicalTables() {
		writeNDJSON(t, dir, tableName, exportMeta{TableName: tableName, RowCount: 0})
	}
	writeNDJSON(t, dir, "chat_logs", exportMeta{TableName: "chat_logs", RowCount: 2})

	plan, err := buildPlan(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if plan.Status != "degraded" {
		t.Errorf("status = %q, want degraded", plan.Status)
	}
	if len(plan.Warnings) == 0 {
		t.Error("expected warnings for row count mismatch")
	}
}

func TestBuildPlanChecksumMismatch(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, manifest{
		TablesExported: allCanonicalTables(),
		RowCounts: map[string]int{
			"chat_logs":               1,
			"effective_input_logs":    0,
			"memories":                0,
			"direct_evidence_records": 0,
			"kg_triples":              0,
			"audit_logs":              0,
			"critic_feedback":         0,
			"character_events":        0,
		},
		Checksums: map[string]string{
			"chat_logs":               "wrong",
			"effective_input_logs":    "",
			"memories":                "",
			"direct_evidence_records": "",
			"kg_triples":              "",
			"audit_logs":              "",
			"critic_feedback":         "",
			"character_events":        "",
		},
	})
	for _, tableName := range allCanonicalTables() {
		writeNDJSON(t, dir, tableName, exportMeta{TableName: tableName, RowCount: 0})
	}
	writeNDJSON(t, dir, "chat_logs", exportMeta{TableName: "chat_logs", RowCount: 1, TableChecksum: "correct"})

	plan, err := buildPlan(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if plan.Status != "degraded" {
		t.Errorf("status = %q, want degraded", plan.Status)
	}
	if len(plan.Warnings) == 0 {
		t.Error("expected warnings for checksum mismatch")
	}
}

func TestBuildPlanMissingManifest(t *testing.T) {
	dir := t.TempDir()
	_, err := buildPlan(dir)
	if err == nil {
		t.Fatal("expected error for missing manifest")
	}
}

func allCanonicalTables() []string {
	return []string{
		"chat_logs",
		"effective_input_logs",
		"memories",
		"direct_evidence_records",
		"kg_triples",
		"audit_logs",
		"critic_feedback",
		"character_events",
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

func writeNDJSON(t *testing.T, dir string, tableName string, meta exportMeta) {
	t.Helper()
	wrapper := map[string]any{"_export_meta": meta}
	metaLine, err := json.Marshal(wrapper)
	if err != nil {
		t.Fatalf("marshal meta: %v", err)
	}
	path := filepath.Join(dir, tableName+".ndjson")
	if err := os.WriteFile(path, metaLine, 0644); err != nil {
		t.Fatalf("write ndjson: %v", err)
	}
}

func TestBuildPlanDerivedTableIgnoredByDefault(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, manifest{
		Mode:                 "all",
		TablesExported:       []string{"chat_logs", "episode_summaries"},
		SkippedMissingTables: allCanonicalExcept("chat_logs"),
		RowCounts:            map[string]int{"chat_logs": 1, "episode_summaries": 1},
		Checksums:            map[string]string{"chat_logs": "abc", "episode_summaries": "def"},
	})
	writeNDJSON(t, dir, "chat_logs", exportMeta{TableName: "chat_logs", RowCount: 1, TableChecksum: "abc"})
	writeNDJSON(t, dir, "episode_summaries", exportMeta{TableName: "episode_summaries", RowCount: 1, TableChecksum: "def"})

	plan, err := buildPlan(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundDerived := false
	for _, tp := range plan.Tables {
		if tp.TableName == "episode_summaries" {
			foundDerived = true
		}
	}
	if foundDerived {
		t.Error("expected episode_summaries to be absent when report-derived is false")
	}
	if plan.Status != "ok" {
		t.Errorf("status = %q, want ok", plan.Status)
	}
}

func allCanonicalExcept(except string) []string {
	var out []string
	for _, t := range []string{
		"chat_logs",
		"effective_input_logs",
		"memories",
		"direct_evidence_records",
		"kg_triples",
		"audit_logs",
		"critic_feedback",
		"character_events",
	} {
		if t != except {
			out = append(out, t)
		}
	}
	return out
}
