package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Canonical tables as defined by the migration contract.
var canonicalTables = []string{
	"chat_logs",
	"effective_input_logs",
	"memories",
	"direct_evidence_records",
	"kg_triples",
	"audit_logs",
	"critic_feedback",
	"character_events",
}

// Required columns per table, excluding id and created_at.
var requiredColumns = map[string][]string{
	"chat_logs":               {"chat_session_id", "turn_index", "role", "content"},
	"effective_input_logs":    {"chat_session_id", "turn_index", "effective_input"},
	"memories":                {"chat_session_id", "turn_index"},
	"direct_evidence_records": {"chat_session_id", "evidence_kind", "evidence_text", "source_turn_start", "source_turn_end", "archive_state", "capture_stage", "capture_verification", "repair_needed", "tombstoned"},
	"kg_triples":              {"chat_session_id", "subject", "predicate", "object"},
	"audit_logs":              {"event_type"},
	"critic_feedback":         {"chat_session_id", "target_type", "target_id", "feedback_value"},
	"character_events":        {"chat_session_id", "character_name", "event_type"},
}

// JSON columns per table that should be validated if present and non-empty string.
var jsonColumns = map[string][]string{
	"memories":                {"summary_json", "embedding", "evidence"},
	"direct_evidence_records": {"source_message_ids_json", "lineage_json"},
	"audit_logs":              {"details_json"},
	"character_events":        {"details_json"},
}

type manifest struct {
	SourceDBPath         string            `json:"source_db_path"`
	ExportTimestamp      string            `json:"export_timestamp"`
	Mode                 string            `json:"mode"`
	TablesExported       []string          `json:"tables_exported"`
	SkippedMissingTables []string          `json:"skipped_missing_tables"`
	RowCounts            map[string]int    `json:"row_counts"`
	Checksums            map[string]string `json:"checksums"`
}

type exportMeta struct {
	TableName       string   `json:"table_name"`
	ExportTimestamp string   `json:"export_timestamp"`
	SourceDBPath    string   `json:"source_db_path"`
	RowCount        int      `json:"row_count"`
	Columns         []string `json:"columns"`
	TableChecksum   string   `json:"table_checksum"`
}

type tableReport struct {
	TableName          string   `json:"table_name"`
	RowsDiscovered     int      `json:"rows_discovered"`
	RowsAccepted       int      `json:"rows_accepted"`
	RowsRejected       int      `json:"rows_rejected"`
	ChecksumExpected   string   `json:"checksum_expected"`
	ChecksumCalculated string   `json:"checksum_calculated"`
	Errors             []string `json:"errors"`
}

type summary struct {
	TablesChecked       int `json:"tables_checked"`
	TablesPassed        int `json:"tables_passed"`
	TablesFailed        int `json:"tables_failed"`
	TotalRowsDiscovered int `json:"total_rows_discovered"`
	TotalRowsAccepted   int `json:"total_rows_accepted"`
	TotalRowsRejected   int `json:"total_rows_rejected"`
}

type report struct {
	Status                    string        `json:"status"`
	ExportDir                 string        `json:"export_dir"`
	CheckedAt                 string        `json:"checked_at"`
	Tables                    []tableReport `json:"tables"`
	NoncanonicalTables        []tableReport `json:"noncanonical_tables,omitempty"`
	SkippedMissingTables      []string      `json:"skipped_missing_tables"`
	IgnoredNoncanonicalTables []string      `json:"ignored_noncanonical_tables"`
	Summary                   summary       `json:"summary"`
}

func main() {
	exportDir := flag.String("export-dir", "", "Path to export directory containing manifest.json and table NDJSON files")
	reportPath := flag.String("report", "", "Path to write the JSON validation report")
	strictCanonical := flag.Bool("strict-canonical", false, "Fail if any canonical table is missing from export and not listed as skipped in manifest")
	reportDerived := flag.Bool("report-derived", false, "Include non-canonical tables in noncanonical_tables with row-count and checksum validation (no required-field checks)")
	flag.Parse()

	if *exportDir == "" {
		fmt.Fprintln(os.Stderr, "error: --export-dir is required")
		os.Exit(2)
	}
	if *reportPath == "" {
		fmt.Fprintln(os.Stderr, "error: --report is required")
		os.Exit(2)
	}

	absExportDir, err := filepath.Abs(*exportDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: resolving export-dir: %v\n", err)
		os.Exit(1)
	}

	rep, err := validateExport(absExportDir, *strictCanonical, *reportDerived)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: validation failed: %v\n", err)
	}

	reportData, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: encoding report: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(*reportPath, append(reportData, '\n'), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error: writing report: %v\n", err)
		os.Exit(1)
	}

	if rep.Status != "ok" {
		os.Exit(1)
	}
}

func validateExport(exportDir string, strictCanonical bool, reportDerived bool) (report, error) {
	manifestPath := filepath.Join(exportDir, "manifest.json")
	manifestData, err := os.ReadFile(manifestPath)
	if err != nil {
		return report{Status: "failed", ExportDir: exportDir, CheckedAt: time.Now().UTC().Format(time.RFC3339)}, fmt.Errorf("reading manifest: %w", err)
	}

	var mf manifest
	if err := json.Unmarshal(manifestData, &mf); err != nil {
		return report{Status: "failed", ExportDir: exportDir, CheckedAt: time.Now().UTC().Format(time.RFC3339)}, fmt.Errorf("parsing manifest: %w", err)
	}

	canonicalSet := make(map[string]bool)
	for _, t := range canonicalTables {
		canonicalSet[t] = true
	}

	skippedSet := make(map[string]bool, len(mf.SkippedMissingTables))
	for _, t := range mf.SkippedMissingTables {
		skippedSet[t] = true
	}

	var ignoredNoncanonical []string
	var tablesToValidate []string
	var tablesToReport []string

	for _, t := range mf.TablesExported {
		if canonicalSet[t] {
			tablesToValidate = append(tablesToValidate, t)
		} else if reportDerived {
			tablesToReport = append(tablesToReport, t)
		} else {
			ignoredNoncanonical = append(ignoredNoncanonical, t)
		}
	}

	var skippedMissing []string
	var globalErrors []string
	var missingCanonicalResults []tableReport
	for _, t := range canonicalTables {
		if skippedSet[t] {
			skippedMissing = append(skippedMissing, t)
			continue
		}
		found := false
		for _, exported := range mf.TablesExported {
			if exported == t {
				found = true
				break
			}
		}
		if found {
			continue
		}
		skippedMissing = append(skippedMissing, t)
		if strictCanonical {
			msg := fmt.Sprintf("canonical table %q is missing from export and not listed as skipped in manifest", t)
			globalErrors = append(globalErrors, msg)
			missingCanonicalResults = append(missingCanonicalResults, tableReport{
				TableName: t,
				Errors:    []string{msg},
			})
		}
	}

	for _, t := range mf.SkippedMissingTables {
		if !canonicalSet[t] {
			ignoredNoncanonical = append(ignoredNoncanonical, t)
		}
	}

	tableResults := make([]tableReport, 0, len(tablesToValidate))
	derivedResults := make([]tableReport, 0, len(tablesToReport))
	sum := summary{}
	overallOK := len(globalErrors) == 0

	for _, t := range tablesToValidate {
		tr := validateTable(exportDir, t, mf.RowCounts[t], mf.Checksums[t], true)
		tableResults = append(tableResults, tr)
		sum.TablesChecked++
		sum.TotalRowsDiscovered += tr.RowsDiscovered
		sum.TotalRowsAccepted += tr.RowsAccepted
		sum.TotalRowsRejected += tr.RowsRejected
		if len(tr.Errors) == 0 {
			sum.TablesPassed++
		} else {
			sum.TablesFailed++
			overallOK = false
		}
	}
	for _, tr := range missingCanonicalResults {
		tableResults = append(tableResults, tr)
		sum.TablesFailed++
		overallOK = false
	}

	for _, t := range tablesToReport {
		tr := validateTable(exportDir, t, mf.RowCounts[t], mf.Checksums[t], false)
		derivedResults = append(derivedResults, tr)
		sum.TablesChecked++
		sum.TotalRowsDiscovered += tr.RowsDiscovered
		sum.TotalRowsAccepted += tr.RowsAccepted
		sum.TotalRowsRejected += tr.RowsRejected
		if len(tr.Errors) == 0 {
			sum.TablesPassed++
		} else {
			sum.TablesFailed++
			overallOK = false
		}
	}

	if len(globalErrors) > 0 {
		overallOK = false
	}

	rep := report{
		Status:                    "ok",
		ExportDir:                 exportDir,
		CheckedAt:                 time.Now().UTC().Format(time.RFC3339),
		Tables:                    tableResults,
		SkippedMissingTables:      skippedMissing,
		IgnoredNoncanonicalTables: ignoredNoncanonical,
		Summary:                   sum,
	}
	if reportDerived {
		rep.NoncanonicalTables = derivedResults
	}
	if !overallOK {
		rep.Status = "failed"
	}

	return rep, nil
}

func validateTable(exportDir string, tableName string, expectedRowCount int, expectedChecksum string, isCanonical bool) tableReport {
	tr := tableReport{
		TableName:        tableName,
		ChecksumExpected: expectedChecksum,
	}

	filePath := filepath.Join(exportDir, tableName+".ndjson")
	f, err := os.Open(filePath)
	if err != nil {
		tr.Errors = append(tr.Errors, fmt.Sprintf("opening NDJSON: %v", err))
		return tr
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	const maxTokenSize = 64 * 1024 * 1024 // 64 MiB
	scanner.Buffer(make([]byte, 0, 64*1024), maxTokenSize)
	if !scanner.Scan() {
		tr.Errors = append(tr.Errors, "NDJSON is empty, missing meta line")
		return tr
	}

	var metaWrapper map[string]json.RawMessage
	if err := json.Unmarshal(scanner.Bytes(), &metaWrapper); err != nil {
		tr.Errors = append(tr.Errors, fmt.Sprintf("parsing meta line: %v", err))
		return tr
	}

	metaRaw, ok := metaWrapper["_export_meta"]
	if !ok {
		tr.Errors = append(tr.Errors, "missing _export_meta in first line")
		return tr
	}

	var meta exportMeta
	if err := json.Unmarshal(metaRaw, &meta); err != nil {
		tr.Errors = append(tr.Errors, fmt.Sprintf("parsing _export_meta: %v", err))
		return tr
	}

	if meta.TableName != tableName {
		tr.Errors = append(tr.Errors, fmt.Sprintf("meta table_name %q does not match expected %q", meta.TableName, tableName))
	}

	rowCount := 0
	rowChecksums := make([]string, 0)
	required := requiredColumns[tableName]
	jsonCols := jsonColumns[tableName]

	for scanner.Scan() {
		rowCount++
		line := scanner.Bytes()

		var row map[string]any
		dec := json.NewDecoder(bytes.NewReader(line))
		dec.UseNumber()
		if err := dec.Decode(&row); err != nil {
			tr.Errors = append(tr.Errors, fmt.Sprintf("row %d: parse error: %v", rowCount, err))
			tr.RowsRejected++
			continue
		}

		rowChecksumForTable := computeRowChecksum(row)
		if rcs, ok := row["_row_checksum"].(string); ok && rcs != "" {
			rowChecksumForTable = rcs
		}
		rowChecksums = append(rowChecksums, rowChecksumForTable)

		if isCanonical {
			missingRequired := false
			for _, col := range required {
				if _, ok := row[col]; !ok {
					tr.Errors = append(tr.Errors, fmt.Sprintf("row %d: missing required field %q", rowCount, col))
					missingRequired = true
				}
			}
			if missingRequired {
				tr.RowsRejected++
				continue
			}

			invalidJSON := false
			for _, col := range jsonCols {
				val, ok := row[col]
				if !ok || val == nil {
					continue
				}
				s, ok := val.(string)
				if !ok || s == "" {
					continue
				}
				if !json.Valid([]byte(s)) {
					tr.Errors = append(tr.Errors, fmt.Sprintf("row %d: invalid JSON in field %q", rowCount, col))
					invalidJSON = true
				}
			}
			if invalidJSON {
				tr.RowsRejected++
				continue
			}

		}

		tr.RowsAccepted++
	}

	if err := scanner.Err(); err != nil {
		tr.Errors = append(tr.Errors, fmt.Sprintf("reading NDJSON: %v", err))
	}

	tr.RowsDiscovered = rowCount

	if meta.RowCount != rowCount {
		tr.Errors = append(tr.Errors, fmt.Sprintf("row count mismatch: meta says %d, discovered %d", meta.RowCount, rowCount))
	}

	if expectedRowCount != rowCount {
		tr.Errors = append(tr.Errors, fmt.Sprintf("row count mismatch: manifest says %d, discovered %d", expectedRowCount, rowCount))
	}

	calculatedTableChecksum := computeTableChecksum(rowChecksums)
	tr.ChecksumCalculated = calculatedTableChecksum

	if meta.TableChecksum != "" && meta.TableChecksum != calculatedTableChecksum {
		tr.Errors = append(tr.Errors, fmt.Sprintf("table checksum mismatch: meta says %q, calculated %q", meta.TableChecksum, calculatedTableChecksum))
	}

	if expectedChecksum != "" && expectedChecksum != calculatedTableChecksum {
		tr.Errors = append(tr.Errors, fmt.Sprintf("table checksum mismatch: manifest says %q, calculated %q", expectedChecksum, calculatedTableChecksum))
	}

	return tr
}

func computeRowChecksum(row map[string]any) string {
	data := make(map[string]any, len(row))
	for k, v := range row {
		if k == "id" || k == "_row_checksum" {
			continue
		}
		data[k] = v
	}

	s, err := canonicalJSON(data)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(s)))
}

func computeTableChecksum(rowChecksums []string) string {
	if len(rowChecksums) == 0 {
		return fmt.Sprintf("%x", sha256.Sum256(nil))
	}
	sorted := make([]string, len(rowChecksums))
	copy(sorted, rowChecksums)
	sort.Strings(sorted)
	joined := strings.Join(sorted, "\n")
	return fmt.Sprintf("%x", sha256.Sum256([]byte(joined)))
}

func canonicalJSON(v any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	s := buf.String()
	if len(s) > 0 && s[len(s)-1] == '\n' {
		s = s[:len(s)-1]
	}
	return s, nil
}
