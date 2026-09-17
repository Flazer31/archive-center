package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func copyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func rowsWithChecksums(rows []map[string]any) []map[string]any {
	out := make([]map[string]any, len(rows))
	for i, row := range rows {
		r := copyMap(row)
		r["_row_checksum"] = computeRowChecksum(r)
		out[i] = r
	}
	return out
}

func tableChecksum(rows []map[string]any) string {
	rcs := make([]string, len(rows))
	for i, row := range rows {
		rcs[i] = computeRowChecksum(row)
	}
	return computeTableChecksum(rcs)
}

func writeManifest(t *testing.T, dir string, mf manifest) {
	t.Helper()
	data, err := json.MarshalIndent(mf, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), append(data, '\n'), 0644); err != nil {
		t.Fatal(err)
	}
}

func writeNDJSON(t *testing.T, dir, tableName string, meta exportMeta, rows []map[string]any) {
	t.Helper()
	path := filepath.Join(dir, tableName+".ndjson")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	wrapper := map[string]any{"_export_meta": meta}
	metaBytes, err := canonicalJSON(wrapper)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(metaBytes + "\n"); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		rowBytes, err := canonicalJSON(row)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.WriteString(rowBytes + "\n"); err != nil {
			t.Fatal(err)
		}
	}
}

func TestValidMinimalExportPasses(t *testing.T) {
	dir := t.TempDir()

	row := map[string]any{
		"id":              json.Number("1"),
		"chat_session_id": "sess-1",
		"turn_index":      json.Number("0"),
		"role":            "user",
		"content":         "hello",
	}
	rows := rowsWithChecksums([]map[string]any{row})
	tcs := tableChecksum([]map[string]any{row})

	meta := exportMeta{
		TableName:     "chat_logs",
		RowCount:      1,
		TableChecksum: tcs,
		Columns:       []string{"id", "chat_session_id", "turn_index", "role", "content"},
	}
	writeNDJSON(t, dir, "chat_logs", meta, rows)

	mf := manifest{
		Mode:           "canonical-only",
		TablesExported: []string{"chat_logs"},
		RowCounts:      map[string]int{"chat_logs": 1},
		Checksums:      map[string]string{"chat_logs": tcs},
	}
	writeManifest(t, dir, mf)

	rep, err := validateExport(dir, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.Status != "ok" {
		t.Fatalf("expected status ok, got %q; tables: %v", rep.Status, rep.Tables)
	}
	if len(rep.Tables) != 1 || rep.Tables[0].TableName != "chat_logs" {
		t.Fatalf("expected 1 table result for chat_logs, got %v", rep.Tables)
	}
	if rep.Tables[0].RowsAccepted != 1 {
		t.Fatalf("expected 1 accepted row, got %d", rep.Tables[0].RowsAccepted)
	}
	if rep.Tables[0].RowsRejected != 0 {
		t.Fatalf("expected 0 rejected rows, got %d", rep.Tables[0].RowsRejected)
	}
	if rep.Summary.TablesPassed != 1 {
		t.Fatalf("expected 1 passed table, got %d", rep.Summary.TablesPassed)
	}
}

func TestChecksumMismatchFails(t *testing.T) {
	dir := t.TempDir()

	row := map[string]any{
		"id":              json.Number("1"),
		"chat_session_id": "sess-1",
		"turn_index":      json.Number("0"),
		"role":            "user",
		"content":         "hello",
	}
	rows := rowsWithChecksums([]map[string]any{row})
	tcs := tableChecksum([]map[string]any{row})

	// Corrupt the row checksum.
	rows[0]["_row_checksum"] = "0000000000000000000000000000000000000000000000000000000000000000"

	meta := exportMeta{
		TableName:     "chat_logs",
		RowCount:      1,
		TableChecksum: tcs,
		Columns:       []string{"id", "chat_session_id", "turn_index", "role", "content"},
	}
	writeNDJSON(t, dir, "chat_logs", meta, rows)

	mf := manifest{
		Mode:           "canonical-only",
		TablesExported: []string{"chat_logs"},
		RowCounts:      map[string]int{"chat_logs": 1},
		Checksums:      map[string]string{"chat_logs": tcs},
	}
	writeManifest(t, dir, mf)

	rep, err := validateExport(dir, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.Status != "failed" {
		t.Fatalf("expected status failed, got %q", rep.Status)
	}
	if len(rep.Tables) != 1 {
		t.Fatalf("expected 1 table result, got %d", len(rep.Tables))
	}
	found := false
	for _, e := range rep.Tables[0].Errors {
		if strings.Contains(e, "table checksum mismatch") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected table checksum mismatch error, got errors: %v", rep.Tables[0].Errors)
	}
	if rep.Tables[0].RowsRejected != 0 {
		t.Fatalf("checksum mismatch should not reject otherwise valid rows, got %d rejected", rep.Tables[0].RowsRejected)
	}
}

func TestMissingRequiredFieldFails(t *testing.T) {
	dir := t.TempDir()

	// Missing "content" required field.
	row := map[string]any{
		"id":              json.Number("1"),
		"chat_session_id": "sess-1",
		"turn_index":      json.Number("0"),
		"role":            "user",
	}
	rows := rowsWithChecksums([]map[string]any{row})
	tcs := tableChecksum([]map[string]any{row})

	meta := exportMeta{
		TableName:     "chat_logs",
		RowCount:      1,
		TableChecksum: tcs,
		Columns:       []string{"id", "chat_session_id", "turn_index", "role", "content"},
	}
	writeNDJSON(t, dir, "chat_logs", meta, rows)

	mf := manifest{
		Mode:           "canonical-only",
		TablesExported: []string{"chat_logs"},
		RowCounts:      map[string]int{"chat_logs": 1},
		Checksums:      map[string]string{"chat_logs": tcs},
	}
	writeManifest(t, dir, mf)

	rep, err := validateExport(dir, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.Status != "failed" {
		t.Fatalf("expected status failed, got %q", rep.Status)
	}
	found := false
	for _, e := range rep.Tables[0].Errors {
		if strings.Contains(e, "missing required field") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected missing required field error, got errors: %v", rep.Tables[0].Errors)
	}
	if rep.Tables[0].RowsRejected != 1 {
		t.Fatalf("expected 1 rejected row, got %d", rep.Tables[0].RowsRejected)
	}
}

func TestNoncanonicalTableIgnored(t *testing.T) {
	dir := t.TempDir()

	row := map[string]any{
		"id":    json.Number("1"),
		"title": "episode one",
	}
	rows := rowsWithChecksums([]map[string]any{row})
	tcs := tableChecksum([]map[string]any{row})

	meta := exportMeta{
		TableName:     "episode_summaries",
		RowCount:      1,
		TableChecksum: tcs,
		Columns:       []string{"id", "title"},
	}
	writeNDJSON(t, dir, "episode_summaries", meta, rows)

	mf := manifest{
		Mode:                 "all",
		TablesExported:       []string{"episode_summaries"},
		RowCounts:            map[string]int{"episode_summaries": 1},
		Checksums:            map[string]string{"episode_summaries": tcs},
		SkippedMissingTables: []string{"chat_logs", "effective_input_logs", "memories", "direct_evidence_records", "kg_triples", "audit_logs", "critic_feedback", "character_events"},
	}
	writeManifest(t, dir, mf)

	rep, err := validateExport(dir, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.Status != "ok" {
		t.Fatalf("expected status ok, got %q; tables: %v", rep.Status, rep.Tables)
	}
	if len(rep.Tables) != 0 {
		t.Fatalf("expected 0 canonical tables, got %v", rep.Tables)
	}
	if !sliceContains(rep.IgnoredNoncanonicalTables, "episode_summaries") {
		t.Fatalf("expected episode_summaries in ignored_noncanonical_tables, got %v", rep.IgnoredNoncanonicalTables)
	}
}

func TestStrictCanonicalSkippedOK(t *testing.T) {
	dir := t.TempDir()

	row := map[string]any{
		"id":              json.Number("1"),
		"chat_session_id": "sess-1",
		"turn_index":      json.Number("0"),
		"role":            "user",
		"content":         "hello",
	}
	rows := rowsWithChecksums([]map[string]any{row})
	tcs := tableChecksum([]map[string]any{row})

	meta := exportMeta{
		TableName:     "chat_logs",
		RowCount:      1,
		TableChecksum: tcs,
		Columns:       []string{"id", "chat_session_id", "turn_index", "role", "content"},
	}
	writeNDJSON(t, dir, "chat_logs", meta, rows)

	mf := manifest{
		Mode:                 "canonical-only",
		TablesExported:       []string{"chat_logs"},
		SkippedMissingTables: []string{"effective_input_logs", "memories", "direct_evidence_records", "kg_triples", "audit_logs", "critic_feedback", "character_events"},
		RowCounts:            map[string]int{"chat_logs": 1},
		Checksums:            map[string]string{"chat_logs": tcs},
	}
	writeManifest(t, dir, mf)

	rep, err := validateExport(dir, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.Status != "ok" {
		t.Fatalf("expected status ok with strict canonical when missing tables are listed as skipped, got %q; errors: %v", rep.Status, rep.Tables)
	}
	if !sliceContains(rep.SkippedMissingTables, "effective_input_logs") {
		t.Fatalf("expected skipped missing tables to contain effective_input_logs, got %v", rep.SkippedMissingTables)
	}
}

func TestStrictCanonicalUnexpectedMissingFails(t *testing.T) {
	dir := t.TempDir()

	row := map[string]any{
		"id":              json.Number("1"),
		"chat_session_id": "sess-1",
		"turn_index":      json.Number("0"),
		"role":            "user",
		"content":         "hello",
	}
	rows := rowsWithChecksums([]map[string]any{row})
	tcs := tableChecksum([]map[string]any{row})

	meta := exportMeta{
		TableName:     "chat_logs",
		RowCount:      1,
		TableChecksum: tcs,
		Columns:       []string{"id", "chat_session_id", "turn_index", "role", "content"},
	}
	writeNDJSON(t, dir, "chat_logs", meta, rows)

	mf := manifest{
		Mode:           "canonical-only",
		TablesExported: []string{"chat_logs"},
		RowCounts:      map[string]int{"chat_logs": 1},
		Checksums:      map[string]string{"chat_logs": tcs},
	}
	writeManifest(t, dir, mf)

	rep, err := validateExport(dir, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.Status != "failed" {
		t.Fatalf("expected status failed when strict canonical and unexpected missing table, got %q", rep.Status)
	}
	found := false
	for _, tr := range rep.Tables {
		for _, e := range tr.Errors {
			if strings.Contains(e, "effective_input_logs") && strings.Contains(e, "missing from export") {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatalf("expected error about unexpected missing effective_input_logs, got tables: %v", rep.Tables)
	}
}

func sliceContains(sl []string, s string) bool {
	for _, v := range sl {
		if v == s {
			return true
		}
	}
	return false
}

func TestLargeRowPasses(t *testing.T) {
	dir := t.TempDir()

	// Build content larger than default Scanner token limit (64 KiB).
	largeContent := strings.Repeat("A", 128*1024)

	row := map[string]any{
		"id":              json.Number("1"),
		"chat_session_id": "sess-1",
		"turn_index":      json.Number("0"),
		"role":            "user",
		"content":         largeContent,
	}
	rows := rowsWithChecksums([]map[string]any{row})
	tcs := tableChecksum([]map[string]any{row})

	meta := exportMeta{
		TableName:     "chat_logs",
		RowCount:      1,
		TableChecksum: tcs,
		Columns:       []string{"id", "chat_session_id", "turn_index", "role", "content"},
	}
	writeNDJSON(t, dir, "chat_logs", meta, rows)

	mf := manifest{
		Mode:                 "canonical-only",
		TablesExported:       []string{"chat_logs"},
		SkippedMissingTables: []string{"effective_input_logs", "memories", "direct_evidence_records", "kg_triples", "audit_logs", "critic_feedback", "character_events"},
		RowCounts:            map[string]int{"chat_logs": 1},
		Checksums:            map[string]string{"chat_logs": tcs},
	}
	writeManifest(t, dir, mf)

	rep, err := validateExport(dir, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.Status != "ok" {
		t.Fatalf("expected status ok, got %q; errors: %v", rep.Status, rep.Tables)
	}
	if len(rep.Tables) != 1 || rep.Tables[0].TableName != "chat_logs" {
		t.Fatalf("expected 1 table result for chat_logs, got %v", rep.Tables)
	}
	if rep.Tables[0].RowsAccepted != 1 {
		t.Fatalf("expected 1 accepted row, got %d", rep.Tables[0].RowsAccepted)
	}
	if rep.Tables[0].RowsRejected != 0 {
		t.Fatalf("expected 0 rejected rows, got %d", rep.Tables[0].RowsRejected)
	}
}

func TestDerivedTableIgnoredByDefault(t *testing.T) {
	dir := t.TempDir()

	row := map[string]any{
		"id":    json.Number("1"),
		"title": "episode one",
	}
	rows := rowsWithChecksums([]map[string]any{row})
	tcs := tableChecksum([]map[string]any{row})

	meta := exportMeta{
		TableName:     "episode_summaries",
		RowCount:      1,
		TableChecksum: tcs,
		Columns:       []string{"id", "title"},
	}
	writeNDJSON(t, dir, "episode_summaries", meta, rows)

	mf := manifest{
		Mode:           "all",
		TablesExported: []string{"episode_summaries"},
		RowCounts:      map[string]int{"episode_summaries": 1},
		Checksums:      map[string]string{"episode_summaries": tcs},
	}
	writeManifest(t, dir, mf)

	rep, err := validateExport(dir, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.Status != "ok" {
		t.Fatalf("expected status ok, got %q; tables: %v", rep.Status, rep.Tables)
	}
	if len(rep.Tables) != 0 {
		t.Fatalf("expected 0 tables (derived ignored by default), got %v", rep.Tables)
	}
	if !sliceContains(rep.IgnoredNoncanonicalTables, "episode_summaries") {
		t.Fatalf("expected episode_summaries in ignored_noncanonical_tables, got %v", rep.IgnoredNoncanonicalTables)
	}
}

func TestDerivedTableReportedWithDerivedFlag(t *testing.T) {
	dir := t.TempDir()

	row := map[string]any{
		"id":    json.Number("1"),
		"title": "episode one",
	}
	rows := rowsWithChecksums([]map[string]any{row})
	tcs := tableChecksum([]map[string]any{row})

	meta := exportMeta{
		TableName:     "episode_summaries",
		RowCount:      1,
		TableChecksum: tcs,
		Columns:       []string{"id", "title"},
	}
	writeNDJSON(t, dir, "episode_summaries", meta, rows)

	mf := manifest{
		Mode:           "all",
		TablesExported: []string{"episode_summaries"},
		RowCounts:      map[string]int{"episode_summaries": 1},
		Checksums:      map[string]string{"episode_summaries": tcs},
	}
	writeManifest(t, dir, mf)

	rep, err := validateExport(dir, false, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.Status != "ok" {
		t.Fatalf("expected status ok, got %q; tables: %v", rep.Status, rep.Tables)
	}
	if len(rep.NoncanonicalTables) != 1 {
		t.Fatalf("expected 1 noncanonical table result, got %v", rep.NoncanonicalTables)
	}
	if rep.NoncanonicalTables[0].TableName != "episode_summaries" {
		t.Fatalf("expected episode_summaries, got %q", rep.NoncanonicalTables[0].TableName)
	}
	if rep.NoncanonicalTables[0].RowsAccepted != 1 {
		t.Fatalf("expected 1 accepted row, got %d", rep.NoncanonicalTables[0].RowsAccepted)
	}
	if rep.NoncanonicalTables[0].RowsRejected != 0 {
		t.Fatalf("expected 0 rejected rows for derived table, got %d", rep.NoncanonicalTables[0].RowsRejected)
	}
	if sliceContains(rep.IgnoredNoncanonicalTables, "episode_summaries") {
		t.Fatalf("episode_summaries should not be in ignored_noncanonical_tables when report-derived is true")
	}
}

func TestDerivedTableMissingRequiredFieldNotRejected(t *testing.T) {
	dir := t.TempDir()

	// Derived table row missing a field that would be "required" for canonical tables
	row := map[string]any{
		"id":    json.Number("1"),
		"title": "episode one",
	}
	rows := rowsWithChecksums([]map[string]any{row})
	tcs := tableChecksum([]map[string]any{row})

	meta := exportMeta{
		TableName:     "episode_summaries",
		RowCount:      1,
		TableChecksum: tcs,
		Columns:       []string{"id", "title"},
	}
	writeNDJSON(t, dir, "episode_summaries", meta, rows)

	mf := manifest{
		Mode:           "all",
		TablesExported: []string{"episode_summaries"},
		RowCounts:      map[string]int{"episode_summaries": 1},
		Checksums:      map[string]string{"episode_summaries": tcs},
	}
	writeManifest(t, dir, mf)

	rep, err := validateExport(dir, false, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rep.Status != "ok" {
		t.Fatalf("expected status ok, got %q; tables: %v", rep.Status, rep.Tables)
	}
	if rep.NoncanonicalTables[0].RowsRejected != 0 {
		t.Fatalf("expected 0 rejected rows for derived table (no required-field enforcement), got %d with errors %v", rep.NoncanonicalTables[0].RowsRejected, rep.NoncanonicalTables[0].Errors)
	}
}
