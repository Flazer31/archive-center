// mariadb-dry-run-import reads a validated SQLite-to-NDJSON export directory
// and produces a MariaDB import plan summary without connecting to any database.
//
// It is safe to run: no DSN, no SQL execution, no live DB access.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
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

// Sample statement shapes per table for the dry-run plan.
var sampleStatementShapes = map[string]string{
	"chat_logs":               "INSERT INTO chat_logs (...) VALUES (...) ON DUPLICATE KEY UPDATE content=VALUES(content), updated_at=NOW()",
	"effective_input_logs":    "INSERT INTO effective_input_logs (...) VALUES (...) ON DUPLICATE KEY UPDATE effective_input=VALUES(effective_input)",
	"memories":                "INSERT INTO memories (...) VALUES (...) ON DUPLICATE KEY UPDATE summary_json=VALUES(summary_json), embedding=VALUES(embedding)",
	"direct_evidence_records": "INSERT INTO direct_evidence_records (...) VALUES (...) ON DUPLICATE KEY UPDATE evidence_text=VALUES(evidence_text), archive_state=VALUES(archive_state)",
	"kg_triples":              "INSERT INTO kg_triples (...) VALUES (...) ON DUPLICATE KEY UPDATE valid_to=VALUES(valid_to)",
	"audit_logs":              "INSERT INTO audit_logs (...) VALUES (...)",
	"critic_feedback":         "INSERT INTO critic_feedback (...) VALUES (...) ON DUPLICATE KEY UPDATE feedback_value=VALUES(feedback_value)",
	"character_events":        "INSERT INTO character_events (...) VALUES (...) ON DUPLICATE KEY UPDATE details_json=VALUES(details_json)",
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
	TableName     string `json:"table_name"`
	RowCount      int    `json:"row_count"`
	TableChecksum string `json:"table_checksum"`
}

type tablePlan struct {
	TableName            string `json:"table_name"`
	RowCount             int    `json:"row_count"`
	PlannedOperation     string `json:"planned_operation"`
	ChecksumExpected     string `json:"checksum_expected,omitempty"`
	ChecksumAvailable    bool   `json:"checksum_available"`
	SampleStatementShape string `json:"sample_statement_shape,omitempty"`
	Status               string `json:"status"`
	Error                string `json:"error,omitempty"`
}

type planReport struct {
	Status      string      `json:"status"`
	ExportDir   string      `json:"export_dir"`
	GeneratedAt string      `json:"generated_at"`
	Scope       string      `json:"scope"`
	Tables      []tablePlan `json:"tables"`
	TotalRows   int         `json:"total_rows"`
	Warnings    []string    `json:"warnings"`
	Errors      []string    `json:"errors"`
}

func main() {
	exportDir := flag.String("export-dir", "", "Path to export directory containing manifest.json and table NDJSON files")
	outPath := flag.String("out", "", "Path to write the JSON import plan (default: stdout)")
	flag.Parse()

	if *exportDir == "" {
		fmt.Fprintln(os.Stderr, "error: --export-dir is required")
		os.Exit(2)
	}

	absExportDir, err := filepath.Abs(*exportDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: resolving export-dir: %v\n", err)
		os.Exit(1)
	}

	plan, err := buildPlan(absExportDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: building plan: %v\n", err)
		os.Exit(1)
	}

	reportData, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: encoding plan: %v\n", err)
		os.Exit(1)
	}
	reportData = append(reportData, '\n')

	if *outPath != "" {
		if err := os.WriteFile(*outPath, reportData, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "error: writing plan: %v\n", err)
			os.Exit(1)
		}
	} else {
		if _, err := os.Stdout.Write(reportData); err != nil {
			fmt.Fprintf(os.Stderr, "error: writing stdout: %v\n", err)
			os.Exit(1)
		}
	}

	if plan.Status != "ok" {
		os.Exit(1)
	}
}

func buildPlan(exportDir string) (*planReport, error) {
	mf, err := readManifest(exportDir)
	if err != nil {
		return nil, err
	}

	plan := &planReport{
		Status:      "ok",
		ExportDir:   exportDir,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Scope:       "mariadb_dry_run_import_plan",
		Tables:      make([]tablePlan, 0, len(canonicalTables)),
	}

	exportedSet := make(map[string]bool)
	for _, t := range mf.TablesExported {
		exportedSet[t] = true
	}
	skippedSet := make(map[string]bool)
	for _, t := range mf.SkippedMissingTables {
		skippedSet[t] = true
	}

	for _, tableName := range canonicalTables {
		tp := tablePlan{
			TableName:            tableName,
			PlannedOperation:     "insert_or_update",
			SampleStatementShape: sampleStatementShapes[tableName],
		}

		if skippedSet[tableName] {
			tp.Status = "skipped"
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("canonical table %q was skipped in export (missing in source)", tableName))
			plan.Tables = append(plan.Tables, tp)
			continue
		}

		if !exportedSet[tableName] {
			tp.Status = "missing"
			tp.Error = fmt.Sprintf("canonical table %q not found in export and not listed as skipped", tableName)
			plan.Errors = append(plan.Errors, tp.Error)
			plan.Tables = append(plan.Tables, tp)
			plan.Status = "failed"
			continue
		}

		rowCount, checksum, err := readNDJSONMeta(exportDir, tableName)
		if err != nil {
			tp.Status = "error"
			tp.Error = err.Error()
			plan.Errors = append(plan.Errors, tp.Error)
			plan.Tables = append(plan.Tables, tp)
			if plan.Status == "ok" {
				plan.Status = "degraded"
			}
			continue
		}

		tp.RowCount = rowCount
		plan.TotalRows += rowCount
		if checksum != "" {
			if mf.Checksums[tableName] != "" && mf.Checksums[tableName] != checksum {
				warn := fmt.Sprintf("table %q: NDJSON meta checksum %q differs from manifest checksum %q", tableName, checksum, mf.Checksums[tableName])
				plan.Warnings = append(plan.Warnings, warn)
				if plan.Status == "ok" {
					plan.Status = "degraded"
				}
			}
			if mf.Checksums[tableName] == "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("table %q: checksum present in NDJSON but missing in manifest", tableName))
			}
			tp.ChecksumAvailable = true
			tp.ChecksumExpected = checksum
		} else {
			if mf.Checksums[tableName] != "" {
				plan.Warnings = append(plan.Warnings, fmt.Sprintf("table %q: checksum present in manifest but missing in NDJSON", tableName))
			}
		}

		manifestRowCount := mf.RowCounts[tableName]
		if manifestRowCount != rowCount {
			warn := fmt.Sprintf("table %q: manifest row count %d differs from NDJSON meta row count %d", tableName, manifestRowCount, rowCount)
			plan.Warnings = append(plan.Warnings, warn)
			if plan.Status == "ok" {
				plan.Status = "degraded"
			}
		}

		tp.Status = "planned"
		plan.Tables = append(plan.Tables, tp)
	}

	if len(plan.Errors) > 0 {
		plan.Status = "failed"
	}

	return plan, nil
}

func readManifest(exportDir string) (*manifest, error) {
	path := filepath.Join(exportDir, "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest.json: %w", err)
	}
	var mf manifest
	if err := json.Unmarshal(data, &mf); err != nil {
		return nil, fmt.Errorf("parsing manifest.json: %w", err)
	}
	return &mf, nil
}

func readNDJSONMeta(exportDir, tableName string) (int, string, error) {
	path := filepath.Join(exportDir, tableName+".ndjson")
	f, err := os.Open(path)
	if err != nil {
		return 0, "", fmt.Errorf("opening NDJSON: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	const maxTokenSize = 64 * 1024 * 1024 // 64 MiB
	scanner.Buffer(make([]byte, 0, 64*1024), maxTokenSize)
	if !scanner.Scan() {
		return 0, "", errors.New("NDJSON is empty, missing meta line")
	}

	var metaWrapper map[string]json.RawMessage
	if err := json.Unmarshal(scanner.Bytes(), &metaWrapper); err != nil {
		return 0, "", fmt.Errorf("parsing meta line: %w", err)
	}

	metaRaw, ok := metaWrapper["_export_meta"]
	if !ok {
		return 0, "", errors.New("missing _export_meta in first line")
	}

	var meta exportMeta
	if err := json.Unmarshal(metaRaw, &meta); err != nil {
		return 0, "", fmt.Errorf("parsing _export_meta: %w", err)
	}

	if meta.TableName != tableName {
		return 0, "", fmt.Errorf("meta table_name %q does not match expected %q", meta.TableName, tableName)
	}

	return meta.RowCount, meta.TableChecksum, scanner.Err()
}
