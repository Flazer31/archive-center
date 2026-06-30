package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var canonicalTables = []string{
	"chat_logs",
	"effective_input_logs",
	"memories",
	"direct_evidence_records",
	"kg_triples",
	"audit_logs",
	"critic_feedback",
	"character_events",
	"storylines",
	"world_rules",
	"character_states",
	"pending_threads",
	"active_states",
	"canonical_state_layers",
	"episode_summaries",
}

type manifest struct {
	TablesExported       []string       `json:"tables_exported"`
	SkippedMissingTables []string       `json:"skipped_missing_tables"`
	RowCounts            map[string]int `json:"row_counts"`
}

type tableCompareResult struct {
	TableName       string `json:"table_name"`
	Status          string `json:"status"`
	ExportCount     int    `json:"export_count"`
	MariaDBCount    int    `json:"mariadb_count"`
	ChecksumMatch   *bool  `json:"checksum_match,omitempty"`
	ExportChecksum  string `json:"export_checksum,omitempty"`
	MariaDBChecksum string `json:"mariadb_checksum,omitempty"`
	Error           string `json:"error,omitempty"`
}

type compareReport struct {
	Status      string               `json:"status"`
	ExportDir   string               `json:"export_dir"`
	GeneratedAt string               `json:"generated_at"`
	Tables      []tableCompareResult `json:"tables"`
	Errors      []string             `json:"errors,omitempty"`
	Warnings    []string             `json:"warnings,omitempty"`
}

var jsonColumns = map[string]map[string]bool{
	"memories": {
		"summary_json": true,
		"embedding":    true,
		"evidence":     true,
	},
	"direct_evidence_records": {
		"source_message_ids_json": true,
		"lineage_json":            true,
	},
	"audit_logs": {
		"details_json": true,
	},
	"character_events": {
		"details_json": true,
	},
	"storylines": {
		"entities_json":         true,
		"key_points_json":       true,
		"ongoing_tensions_json": true,
	},
	"world_rules": {
		"value_json": true,
	},
	"character_states": {
		"appearance_json":    true,
		"personality_json":   true,
		"status_json":        true,
		"relationships_json": true,
		"speech_style_json":  true,
	},
	"pending_threads": {
		"hook_metadata_json": true,
	},
	"episode_summaries": {
		"key_entities":              true,
		"key_events":                true,
		"open_loops_json":           true,
		"relationship_changes_json": true,
		"embedding_vector":          true,
	},
}

func main() {
	exportDir := flag.String("export-dir", "", "Path to export directory containing manifest.json and canonical NDJSON files.")
	dsn := flag.String("dsn", os.Getenv("AC_MARIADB_DSN"), "MariaDB DSN. Defaults to AC_MARIADB_DSN.")
	outPath := flag.String("out", "", "Path to write compare JSON report. Defaults to stdout.")
	timeout := flag.Duration("timeout", 60*time.Second, "Compare timeout.")
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

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	report, err := runCompare(ctx, absExportDir, *dsn)
	if err != nil {
		report.Errors = append(report.Errors, err.Error())
		if report.Status == "" {
			report.Status = "failed"
		}
	}

	writeReport(report, *outPath)
	if report.Status == "failed" {
		os.Exit(2)
	}
	if report.Status == "mismatch" || report.Status == "missing_export" {
		os.Exit(1)
	}
}

func runCompare(ctx context.Context, exportDir, dsn string) (*compareReport, error) {
	report := &compareReport{
		ExportDir: exportDir,
	}
	if strings.TrimSpace(dsn) == "" {
		report.Status = "failed"
		report.Errors = append(report.Errors, "missing DSN: provide --dsn or AC_MARIADB_DSN")
		return report, nil
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		report.Status = "failed"
		report.Errors = append(report.Errors, fmt.Sprintf("open db: %v", err))
		return report, err
	}
	defer db.Close()

	return compareExport(ctx, db, exportDir)
}

func compareExport(ctx context.Context, db *sql.DB, exportDir string) (*compareReport, error) {
	report := &compareReport{
		ExportDir:   exportDir,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}

	mf, err := readManifest(exportDir)
	if err != nil {
		report.Status = "failed"
		report.Errors = append(report.Errors, err.Error())
		return report, err
	}

	exported := make(map[string]bool)
	for _, t := range mf.TablesExported {
		exported[t] = true
	}
	skipped := make(map[string]bool)
	for _, t := range mf.SkippedMissingTables {
		skipped[t] = true
	}

	for _, table := range canonicalTables {
		if skipped[table] {
			report.Tables = append(report.Tables, tableCompareResult{
				TableName: table,
				Status:    "skipped",
			})
			continue
		}
		if !exported[table] {
			msg := fmt.Sprintf("canonical table %q missing from export", table)
			report.Tables = append(report.Tables, tableCompareResult{
				TableName: table,
				Status:    "missing_export",
				Error:     msg,
			})
			report.Errors = append(report.Errors, msg)
			report.Status = "failed"
			continue
		}

		tr, err := compareTable(ctx, db, exportDir, table)
		report.Tables = append(report.Tables, tr)
		if err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("table %s: %v", table, err))
			report.Status = "failed"
		} else if tr.Status != "ok" && report.Status != "failed" {
			report.Status = "mismatch"
		}
	}

	if report.Status == "" {
		allOk := true
		for _, tr := range report.Tables {
			if tr.Status != "ok" && tr.Status != "missing_export" {
				allOk = false
				break
			}
		}
		if allOk {
			report.Status = "ok"
		} else {
			report.Status = "mismatch"
		}
	}

	return report, nil
}

func compareTable(ctx context.Context, db *sql.DB, exportDir, table string) (tableCompareResult, error) {
	result := tableCompareResult{
		TableName: table,
		Status:    "ok",
	}

	exportRows, err := readExportRows(exportDir, table)
	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()
		return result, err
	}
	for i := range exportRows {
		exportRows[i] = normalizeExportRow(table, exportRows[i])
		exportRows[i] = normalizeComparableRow(table, exportRows[i])
	}
	result.ExportCount = len(exportRows)

	var count int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM `%s`", table)
	if err := db.QueryRowContext(ctx, countQuery).Scan(&count); err != nil {
		result.Status = "failed"
		result.Error = fmt.Sprintf("count query: %v", err)
		return result, err
	}
	result.MariaDBCount = count

	if result.ExportCount != result.MariaDBCount {
		result.Status = "count_mismatch"
		result.Error = fmt.Sprintf("count mismatch: export=%d, mariadb=%d", result.ExportCount, result.MariaDBCount)
	}

	selectQuery := fmt.Sprintf("SELECT * FROM `%s`", table)
	rows, err := db.QueryContext(ctx, selectQuery)
	if err != nil {
		if result.Status == "ok" {
			result.Status = "failed"
		}
		result.Error = fmt.Sprintf("select query: %v", err)
		return result, err
	}
	defer rows.Close()

	mariadbRows, err := rowsToMaps(rows)
	if err != nil {
		if result.Status == "ok" {
			result.Status = "failed"
		}
		result.Error = fmt.Sprintf("scan rows: %v", err)
		return result, err
	}
	for i := range mariadbRows {
		mariadbRows[i] = normalizeComparableRow(table, mariadbRows[i])
	}

	exportChecksum, err := computeChecksums(exportRows)
	if err != nil {
		if result.Status == "ok" {
			result.Status = "failed"
		}
		result.Error = fmt.Sprintf("export checksum: %v", err)
		return result, err
	}

	mariadbChecksum, err := computeChecksums(mariadbRows)
	if err != nil {
		if result.Status == "ok" {
			result.Status = "failed"
		}
		result.Error = fmt.Sprintf("mariadb checksum: %v", err)
		return result, err
	}

	match := exportChecksum == mariadbChecksum
	result.ChecksumMatch = &match
	result.ExportChecksum = exportChecksum
	result.MariaDBChecksum = mariadbChecksum

	if !match && result.Status == "ok" {
		result.Status = "checksum_mismatch"
	}

	return result, nil
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

// normalizeExportRow maps old export shapes to current schema shapes.
// It is applied before checksum computation so export and MariaDB rows align.
func normalizeExportRow(table string, row map[string]any) map[string]any {
	if table != "pending_threads" {
		return row
	}
	out := make(map[string]any, len(row))
	for k, v := range row {
		out[k] = v
	}

	// thread_key = title
	if title, ok := out["title"]; ok {
		out["thread_key"] = title
	}

	// description = title (or details_json if title is empty)
	desc := ""
	if title, ok := out["title"]; ok {
		if s, ok := title.(string); ok && strings.TrimSpace(s) != "" {
			desc = s
		}
	}
	if desc == "" {
		if details, ok := out["details_json"]; ok && details != nil {
			if s, ok := details.(string); ok && strings.TrimSpace(s) != "" {
				desc = s
			} else {
				if b, err := json.Marshal(details); err == nil {
					desc = string(b)
				}
			}
		}
	}
	if desc != "" {
		out["description"] = desc
	}

	// created_turn = source_turn
	if srcTurn, ok := out["source_turn"]; ok {
		out["created_turn"] = srcTurn
	}

	// resolved_turn = last_seen_turn
	if lastSeen, ok := out["last_seen_turn"]; ok {
		out["resolved_turn"] = lastSeen
	}

	// priority = int(confidence * 100) when confidence is present
	if conf, ok := out["confidence"]; ok && conf != nil {
		var f float64
		switch v := conf.(type) {
		case json.Number:
			if fv, err := v.Float64(); err == nil {
				f = fv
			}
		case float64:
			f = v
		case int:
			f = float64(v)
		case int64:
			f = float64(v)
		}
		if f != 0 {
			out["priority"] = int(f * 100)
		}
	}

	// hook_type = thread_type
	if tt, ok := out["thread_type"]; ok {
		out["hook_type"] = tt
	}

	// hook_metadata_json = details_json
	if details, ok := out["details_json"]; ok {
		if s, ok := details.(string); ok {
			trimmed := strings.TrimSpace(s)
			if trimmed == "" {
				out["hook_metadata_json"] = nil
			} else if json.Valid([]byte(trimmed)) {
				out["hook_metadata_json"] = trimmed
			} else {
				out["hook_metadata_json"] = map[string]any{"details": s}
			}
		} else {
			out["hook_metadata_json"] = details
		}
	}

	// Remove old export-only columns
	delete(out, "thread_type")
	delete(out, "title")
	delete(out, "owner")
	delete(out, "target")
	delete(out, "last_seen_turn")
	delete(out, "confidence")
	delete(out, "details_json")
	delete(out, "resolution_note")

	// MariaDB BOOLEAN is TINYINT(1); align export booleans to int64 so checksums match.
	for _, key := range []string{"pinned", "suppressed", "user_corrected"} {
		if v, ok := out[key]; ok {
			switch val := v.(type) {
			case bool:
				if val {
					out[key] = int64(1)
				} else {
					out[key] = int64(0)
				}
			}
		}
	}

	return out
}

func normalizeComparableRow(table string, row map[string]any) map[string]any {
	out := make(map[string]any, len(row))
	for k, v := range row {
		if jsonColumns[table][k] {
			out[k] = normalizeJSONComparable(v)
			continue
		}
		out[k] = v
	}
	return out
}

func normalizeJSONComparable(value any) any {
	switch v := value.(type) {
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return nil
		}
		var decoded any
		dec := json.NewDecoder(strings.NewReader(trimmed))
		dec.UseNumber()
		if err := dec.Decode(&decoded); err == nil {
			return decoded
		}
		return v
	case []byte:
		return normalizeJSONComparable(string(v))
	default:
		return v
	}
}

func readExportRows(exportDir, table string) ([]map[string]any, error) {
	path := filepath.Join(exportDir, table+".ndjson")
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
	if !scanner.Scan() {
		return nil, errors.New("missing NDJSON meta line")
	}

	var rows []map[string]any
	for scanner.Scan() {
		dec := json.NewDecoder(strings.NewReader(scanner.Text()))
		dec.UseNumber()
		var row map[string]any
		if err := dec.Decode(&row); err != nil {
			return nil, fmt.Errorf("decode row: %w", err)
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return rows, nil
}

func rowsToMaps(rows *sql.Rows) ([]map[string]any, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var out []map[string]any
	for rows.Next() {
		values := make([]any, len(cols))
		valuePtrs := make([]any, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			row[col] = normalizeMySQLValue(values[i])
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func normalizeMySQLValue(v any) any {
	switch val := v.(type) {
	case []byte:
		return string(val)
	case string:
		return val
	case int64:
		return val
	case uint64:
		if val <= uint64(^uint64(0)>>1) {
			return int64(val)
		}
		return json.Number(strconv.FormatUint(val, 10))
	case uint:
		return int64(val)
	case uint32:
		return int64(val)
	case float64:
		return val
	case bool:
		// MariaDB BOOLEAN is TINYINT(1); canonicalize to int64 for checksum parity.
		if val {
			return int64(1)
		}
		return int64(0)
	case time.Time:
		return val.Format(time.RFC3339Nano)
	case nil:
		return nil
	default:
		return fmt.Sprint(val)
	}
}

func computeChecksums(rows []map[string]any) (string, error) {
	if len(rows) == 0 {
		return "", nil
	}
	var checksums []string
	for _, row := range rows {
		rowCopy := make(map[string]any, len(row))
		for k, v := range row {
			if k == "id" || k == "_row_checksum" {
				continue
			}
			rowCopy[k] = v
		}
		b, err := canonicalJSONBytes(rowCopy)
		if err != nil {
			return "", err
		}
		h := sha256.Sum256(b)
		checksums = append(checksums, hex.EncodeToString(h[:]))
	}
	sort.Strings(checksums)
	overall := sha256.Sum256([]byte(strings.Join(checksums, "\n")))
	return hex.EncodeToString(overall[:]), nil
}

func canonicalJSONBytes(v map[string]any) ([]byte, error) {
	var buf strings.Builder
	if err := encodeCanonicalMap(&buf, v); err != nil {
		return nil, err
	}
	return []byte(buf.String()), nil
}

func encodeCanonicalMap(w *strings.Builder, v map[string]any) error {
	w.WriteByte('{')
	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		if i > 0 {
			w.WriteByte(',')
		}
		w.WriteString(encodeJSONString(k))
		w.WriteByte(':')
		if err := encodeCanonicalValue(w, v[k]); err != nil {
			return err
		}
	}
	w.WriteByte('}')
	return nil
}

func encodeCanonicalValue(w *strings.Builder, v any) error {
	switch val := v.(type) {
	case nil:
		w.WriteString("null")
	case bool:
		if val {
			w.WriteString("true")
		} else {
			w.WriteString("false")
		}
	case float64:
		w.WriteString(strconv.FormatFloat(val, 'g', -1, 64))
	case json.Number:
		w.WriteString(string(val))
	case int:
		w.WriteString(strconv.Itoa(val))
	case int64:
		w.WriteString(strconv.FormatInt(val, 10))
	case string:
		w.WriteString(encodeJSONString(val))
	case []any:
		w.WriteByte('[')
		for i, item := range val {
			if i > 0 {
				w.WriteByte(',')
			}
			if err := encodeCanonicalValue(w, item); err != nil {
				return err
			}
		}
		w.WriteByte(']')
	case map[string]any:
		if err := encodeCanonicalMap(w, val); err != nil {
			return err
		}
	default:
		b, err := json.Marshal(val)
		if err != nil {
			return err
		}
		w.Write(b)
	}
	return nil
}

func encodeJSONString(s string) string {
	var buf strings.Builder
	buf.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		case '\b':
			buf.WriteString(`\b`)
		case '\f':
			buf.WriteString(`\f`)
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\t':
			buf.WriteString(`\t`)
		default:
			if r < 0x20 {
				buf.WriteString(fmt.Sprintf(`\u%04x`, r))
			} else {
				buf.WriteRune(r)
			}
		}
	}
	buf.WriteByte('"')
	return buf.String()
}

func writeReport(report *compareReport, outPath string) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: encoding report: %v\n", err)
		return
	}
	data = append(data, '\n')
	if outPath == "" {
		_, _ = os.Stdout.Write(data)
		return
	}
	if err := os.WriteFile(outPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error: writing report: %v\n", err)
	}
}
