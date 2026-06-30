package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

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

type dryRunReport struct {
	Status                    string        `json:"status"`
	ExportDir                 string        `json:"export_dir"`
	CheckedAt                 string        `json:"checked_at"`
	Tables                    []tableReport `json:"tables"`
	NoncanonicalTables        []tableReport `json:"noncanonical_tables,omitempty"`
	SkippedMissingTables      []string      `json:"skipped_missing_tables"`
	IgnoredNoncanonicalTables []string      `json:"ignored_noncanonical_tables"`
	Summary                   summary       `json:"summary"`
}

type comparisonResult struct {
	TableName        string `json:"table_name"`
	SQLiteRowCount   int    `json:"sqlite_row_count"`
	ReportRowCount   int    `json:"report_row_count"`
	RowCountMatch    bool   `json:"row_count_match"`
	ChecksumExpected string `json:"checksum_expected"`
	ChecksumCalc     string `json:"checksum_calculated"`
	ChecksumMatch    bool   `json:"checksum_match"`
	Status           string `json:"status"`
	Details          string `json:"details"`
}

type overallResult struct {
	Status   string             `json:"status"`
	Details  []comparisonResult `json:"details"`
	Warnings []string           `json:"warnings"`
	Errors   []string           `json:"errors"`
}

func main() {
	sqliteDB := flag.String("sqlite-db", "", "Path to SQLite database (read-only)")
	reportPath := flag.String("dry-run-report", "", "Path to dry-run validator JSON report")
	jsonOut := flag.Bool("json", false, "Output JSON instead of table")
	flag.Parse()

	if *sqliteDB == "" {
		fmt.Fprintln(os.Stderr, "error: --sqlite-db is required")
		os.Exit(2)
	}
	if *reportPath == "" {
		fmt.Fprintln(os.Stderr, "error: --dry-run-report is required")
		os.Exit(2)
	}

	absDB, err := filepath.Abs(*sqliteDB)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: resolving sqlite-db: %v\n", err)
		os.Exit(1)
	}
	absReport, err := filepath.Abs(*reportPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: resolving dry-run-report: %v\n", err)
		os.Exit(1)
	}

	report, err := loadReport(absReport)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: loading report: %v\n", err)
		os.Exit(1)
	}

	dsn := fmt.Sprintf("file:%s?mode=ro", absDB)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: opening sqlite: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	result, err := compare(db, report)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: comparison failed: %v\n", err)
		os.Exit(1)
	}

	if *jsonOut {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
	} else {
		printTable(result)
	}

	if result.Status != "ok" {
		os.Exit(1)
	}
}

func loadReport(path string) (*dryRunReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r dryRunReport
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func compare(db *sql.DB, report *dryRunReport) (*overallResult, error) {
	result := &overallResult{
		Status:   "ok",
		Details:  make([]comparisonResult, 0),
		Warnings: make([]string, 0),
		Errors:   make([]string, 0),
	}

	if report.Status != "ok" {
		result.Status = "failed"
		result.Errors = append(result.Errors, fmt.Sprintf("dry-run report status is %q", report.Status))
	}

	sqliteTables, err := listTables(db)
	if err != nil {
		return nil, err
	}

	reportTableSet := make(map[string]struct{})
	for _, tr := range report.Tables {
		reportTableSet[tr.TableName] = struct{}{}
		cr := compareTable(db, tr)
		result.Details = append(result.Details, cr)
		if !cr.RowCountMatch {
			result.Status = "failed"
			result.Errors = append(result.Errors, fmt.Sprintf("table %s: row count mismatch (sqlite=%d, report=%d)", tr.TableName, cr.SQLiteRowCount, cr.ReportRowCount))
		}
		if !cr.ChecksumMatch && tr.ChecksumExpected != "" {
			result.Status = "failed"
			result.Errors = append(result.Errors, fmt.Sprintf("table %s: checksum mismatch (expected=%s, calculated=%s)", tr.TableName, cr.ChecksumExpected, cr.ChecksumCalc))
		}
		if cr.Status == "error" {
			result.Status = "failed"
			result.Errors = append(result.Errors, fmt.Sprintf("table %s: query error (%s)", tr.TableName, cr.Details))
		}
	}
	for _, tr := range report.NoncanonicalTables {
		reportTableSet[tr.TableName] = struct{}{}
		cr := compareTable(db, tr)
		result.Details = append(result.Details, cr)
		if !cr.RowCountMatch {
			result.Status = "failed"
			result.Errors = append(result.Errors, fmt.Sprintf("table %s: row count mismatch (sqlite=%d, report=%d)", tr.TableName, cr.SQLiteRowCount, cr.ReportRowCount))
		}
		if !cr.ChecksumMatch && tr.ChecksumExpected != "" {
			result.Status = "failed"
			result.Errors = append(result.Errors, fmt.Sprintf("table %s: checksum mismatch (expected=%s, calculated=%s)", tr.TableName, cr.ChecksumExpected, cr.ChecksumCalc))
		}
		if cr.Status == "error" {
			result.Status = "failed"
			result.Errors = append(result.Errors, fmt.Sprintf("table %s: query error (%s)", tr.TableName, cr.Details))
		}
	}

	for _, skipped := range report.SkippedMissingTables {
		if _, ok := sqliteTables[skipped]; ok {
			result.Status = "failed"
			result.Errors = append(result.Errors, fmt.Sprintf("table %s: present in SQLite but skipped in report", skipped))
		} else {
			result.Warnings = append(result.Warnings, fmt.Sprintf("table %s: missing in both SQLite and report (skipped)", skipped))
		}
	}

	for t := range sqliteTables {
		if _, ok := reportTableSet[t]; !ok {
			if sliceContains(report.SkippedMissingTables, t) {
				continue
			}
			if isCanonicalTable(t) {
				result.Status = "failed"
				result.Errors = append(result.Errors, fmt.Sprintf("canonical table %s: present in SQLite but missing from report", t))
				continue
			}
			if !sliceContains(report.IgnoredNoncanonicalTables, t) {
				result.Warnings = append(result.Warnings, fmt.Sprintf("table %s: present in SQLite but not in report", t))
			}
		}
	}

	return result, nil
}

func listTables(db *sql.DB) (map[string]struct{}, error) {
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := make(map[string]struct{})
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables[name] = struct{}{}
	}
	return tables, rows.Err()
}

func quoteSQLiteIdent(ident string) string {
	return "\"" + strings.ReplaceAll(ident, "\"", "\"\"") + "\""
}

func isCanonicalTable(tableName string) bool {
	switch tableName {
	case "chat_logs",
		"effective_input_logs",
		"memories",
		"direct_evidence_records",
		"kg_triples",
		"audit_logs",
		"critic_feedback",
		"character_events":
		return true
	default:
		return false
	}
}

func compareTable(db *sql.DB, tr tableReport) comparisonResult {
	cr := comparisonResult{
		TableName:        tr.TableName,
		ReportRowCount:   tr.RowsDiscovered,
		ChecksumExpected: tr.ChecksumExpected,
		ChecksumCalc:     tr.ChecksumCalculated,
	}

	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteSQLiteIdent(tr.TableName))
	err := db.QueryRow(query).Scan(&count)
	if err != nil {
		cr.Status = "error"
		cr.Details = fmt.Sprintf("SQLite query error: %v", err)
		cr.RowCountMatch = false
		cr.ChecksumMatch = false
		return cr
	}
	cr.SQLiteRowCount = count
	cr.RowCountMatch = count == tr.RowsDiscovered

	if tr.ChecksumExpected != "" {
		cr.ChecksumMatch = tr.ChecksumExpected == tr.ChecksumCalculated
	} else {
		cr.ChecksumMatch = true
	}

	if cr.RowCountMatch && cr.ChecksumMatch {
		cr.Status = "ok"
	} else {
		cr.Status = "mismatch"
	}

	if !cr.RowCountMatch {
		if cr.Details != "" {
			cr.Details += "; "
		}
		cr.Details += "row count mismatch"
	}
	if !cr.ChecksumMatch && tr.ChecksumExpected != "" {
		if cr.Details != "" {
			cr.Details += "; "
		}
		cr.Details += "checksum mismatch"
	}

	return cr
}

func printTable(result *overallResult) {
	fmt.Println("Table                  SQLite  Report  Match  Status")
	fmt.Println(strings.Repeat("-", 60))
	for _, d := range result.Details {
		matchStr := "yes"
		if !d.RowCountMatch || !d.ChecksumMatch {
			matchStr = "NO"
		}
		fmt.Printf("%-22s %6d  %6d  %-5s  %s\n", d.TableName, d.SQLiteRowCount, d.ReportRowCount, matchStr, d.Status)
	}
	if len(result.Warnings) > 0 {
		fmt.Println()
		fmt.Println("Warnings:")
		for _, w := range result.Warnings {
			fmt.Printf("  - %s\n", w)
		}
	}
	if len(result.Errors) > 0 {
		fmt.Println()
		fmt.Println("Errors:")
		for _, e := range result.Errors {
			fmt.Printf("  - %s\n", e)
		}
	}
	fmt.Println()
	fmt.Printf("Overall: %s\n", result.Status)
}

func sliceContains(sl []string, s string) bool {
	for _, v := range sl {
		if v == s {
			return true
		}
	}
	return false
}
