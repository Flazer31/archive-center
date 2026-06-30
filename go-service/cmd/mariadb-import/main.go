// mariadb-import loads a validated SQLite-to-NDJSON export into MariaDB.
//
// This is the real target import executor for the 2.0 MariaDB shadow lane.
// It requires --execute plus a DSN so accidental local runs do not write data.
package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
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
	"session_active_scopes",
	"character_states",
	"pending_threads",
	"active_states",
	"canonical_state_layers",
	"episode_summaries",
	"guidance_plan_states",
}

var tableColumns = map[string][]string{
	"chat_logs": {
		"id", "chat_session_id", "turn_index", "role", "content", "created_at",
	},
	"effective_input_logs": {
		"id", "chat_session_id", "turn_index", "effective_input", "created_at",
	},
	"memories": {
		"id", "chat_session_id", "turn_index", "summary_json", "embedding", "embedding_model",
		"importance", "emotional_boost", "evidence", "emotional_intensity",
		"narrative_significance", "place_wing", "place_room", "created_at",
	},
	"direct_evidence_records": {
		"id", "chat_session_id", "evidence_kind", "evidence_text", "source_turn_start",
		"source_turn_end", "turn_anchor", "source_message_ids_json", "source_hash",
		"archive_state", "capture_stage", "capture_verification", "committed_gate",
		"lineage_json", "repair_needed", "tombstoned", "superseded_by_id", "created_at",
	},
	"kg_triples": {
		"id", "chat_session_id", "subject", "predicate", "object", "valid_from", "valid_to", "source_turn", "created_at",
	},
	"audit_logs": {
		"id", "created_at", "event_type", "chat_session_id", "target_type", "target_id", "summary", "details_json", "source",
	},
	"critic_feedback": {
		"id", "created_at", "chat_session_id", "target_type", "target_id", "feedback_value", "feedback_note", "source",
	},
	"character_events": {
		"id", "chat_session_id", "character_name", "turn_index", "event_type", "details_json", "created_at",
	},
	"storylines": {
		"id", "chat_session_id", "name", "status", "entities_json", "current_context",
		"key_points_json", "ongoing_tensions_json", "confidence", "evidence_count",
		"last_evidence_turn", "first_turn", "last_turn", "pinned", "suppressed",
		"user_corrected", "created_at", "updated_at",
	},
	"world_rules": {
		"id", "chat_session_id", "scope", "scope_name", "category", "key",
		"value_json", "genre", "source_turn", "pinned", "suppressed",
		"user_corrected", "created_at", "updated_at",
	},
	"session_active_scopes": {
		"id", "chat_session_id", "active_scope", "scope_name", "updated_at",
	},
	"character_states": {
		"id", "chat_session_id", "character_name", "appearance_json", "personality_json",
		"status_json", "relationships_json", "speech_style_json", "turn_index", "created_at", "updated_at",
	},
	"pending_threads": {
		"id", "chat_session_id", "thread_key", "description", "status", "created_turn",
		"resolved_turn", "source_turn", "priority", "hook_type", "hook_metadata_json",
		"pinned", "suppressed", "user_corrected", "created_at", "updated_at",
	},
	"active_states": {
		"id", "chat_session_id", "state_type", "content", "turn_index", "created_at",
	},
	"canonical_state_layers": {
		"id", "chat_session_id", "layer_type", "content", "source_state_type", "turn_index",
		"source_turn", "source_record", "last_verified_turn", "confidence", "created_at",
	},
	"episode_summaries": {
		"id", "chat_session_id", "from_turn", "to_turn", "summary_text", "key_entities",
		"key_events", "open_loops_json", "relationship_changes_json", "embedding_vector",
		"embedding_model", "created_at",
	},
	"guidance_plan_states": {
		"id", "chat_session_id", "story_plan_json", "director_json", "state_status",
		"last_turn", "warnings_json", "created_at", "updated_at",
	},
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
	"guidance_plan_states": {
		"story_plan_json": true,
		"director_json":   true,
		"warnings_json":   true,
	},
}

var datetimeColumns = map[string]map[string]bool{
	"chat_logs":               {"created_at": true},
	"effective_input_logs":    {"created_at": true},
	"memories":                {"created_at": true},
	"direct_evidence_records": {"created_at": true},
	"kg_triples":              {"created_at": true},
	"audit_logs":              {"created_at": true},
	"critic_feedback":         {"created_at": true},
	"character_events":        {"created_at": true},
	"storylines":              {"created_at": true, "updated_at": true},
	"world_rules":             {"created_at": true, "updated_at": true},
	"session_active_scopes":   {"updated_at": true},
	"character_states":        {"created_at": true, "updated_at": true},
	"pending_threads":         {"created_at": true, "updated_at": true},
	"active_states":           {"created_at": true},
	"canonical_state_layers":  {"created_at": true},
	"episode_summaries":       {"created_at": true},
	"guidance_plan_states":    {"created_at": true, "updated_at": true},
}

type manifest struct {
	TablesExported       []string       `json:"tables_exported"`
	SkippedMissingTables []string       `json:"skipped_missing_tables"`
	RowCounts            map[string]int `json:"row_counts"`
}

type tableReport struct {
	TableName    string `json:"table_name"`
	Status       string `json:"status"`
	RowsRead     int    `json:"rows_read"`
	RowsImported int    `json:"rows_imported"`
	Error        string `json:"error,omitempty"`
}

type importReport struct {
	Status      string        `json:"status"`
	ExportDir   string        `json:"export_dir"`
	Executed    bool          `json:"executed"`
	GeneratedAt string        `json:"generated_at"`
	Tables      []tableReport `json:"tables"`
	TotalRows   int           `json:"total_rows"`
	Errors      []string      `json:"errors,omitempty"`
}

type sqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func main() {
	exportDir := flag.String("export-dir", "", "Path to export directory containing manifest.json and canonical NDJSON files.")
	dsn := flag.String("dsn", os.Getenv("AC_MARIADB_DSN"), "MariaDB DSN. Defaults to AC_MARIADB_DSN.")
	outPath := flag.String("out", "", "Path to write import JSON report. Defaults to stdout.")
	execute := flag.Bool("execute", false, "Required to write rows into MariaDB.")
	timeout := flag.Duration("timeout", 60*time.Second, "Import timeout.")
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

	report := newReport(absExportDir, *execute)
	if !*execute {
		report.Status = "guarded"
		report.Errors = append(report.Errors, "--execute is required before MariaDB writes are allowed")
		writeReport(report, *outPath)
		os.Exit(2)
	}
	if strings.TrimSpace(*dsn) == "" {
		report.Status = "failed"
		report.Errors = append(report.Errors, "missing DSN: provide --dsn or AC_MARIADB_DSN")
		writeReport(report, *outPath)
		os.Exit(2)
	}

	db, err := sql.Open("mysql", *dsn)
	if err != nil {
		report.Status = "failed"
		report.Errors = append(report.Errors, err.Error())
		writeReport(report, *outPath)
		os.Exit(1)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	report, err = importExport(ctx, db, absExportDir, *execute)
	if err != nil {
		report.Status = "failed"
		report.Errors = append(report.Errors, err.Error())
	}
	writeReport(report, *outPath)
	if report.Status != "ok" {
		os.Exit(1)
	}
}

func newReport(exportDir string, executed bool) *importReport {
	return &importReport{
		Status:      "ok",
		ExportDir:   exportDir,
		Executed:    executed,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

func importExport(ctx context.Context, db sqlExecer, exportDir string, executed bool) (*importReport, error) {
	report := newReport(exportDir, executed)
	mf, err := readManifest(exportDir)
	if err != nil {
		report.Status = "failed"
		return report, err
	}

	exported := map[string]bool{}
	for _, table := range mf.TablesExported {
		exported[table] = true
	}
	skipped := map[string]bool{}
	for _, table := range mf.SkippedMissingTables {
		skipped[table] = true
	}

	for _, table := range canonicalTables {
		if skipped[table] {
			report.Tables = append(report.Tables, tableReport{TableName: table, Status: "skipped"})
			continue
		}
		if !exported[table] {
			msg := fmt.Sprintf("canonical table %q missing from export", table)
			report.Tables = append(report.Tables, tableReport{TableName: table, Status: "missing", Error: msg})
			report.Errors = append(report.Errors, msg)
			report.Status = "failed"
			continue
		}

		tr, err := importTable(ctx, db, exportDir, table)
		report.Tables = append(report.Tables, tr)
		report.TotalRows += tr.RowsImported
		if err != nil {
			report.Errors = append(report.Errors, err.Error())
			report.Status = "failed"
		}
	}

	if report.Status == "" {
		report.Status = "ok"
	}
	return report, nil
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

func importTable(ctx context.Context, db sqlExecer, exportDir, table string) (tableReport, error) {
	report := tableReport{TableName: table, Status: "ok"}
	path := filepath.Join(exportDir, table+".ndjson")
	f, err := os.Open(path)
	if err != nil {
		report.Status = "failed"
		report.Error = err.Error()
		return report, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 64*1024*1024)
	if !scanner.Scan() {
		err := errors.New("missing NDJSON meta line")
		report.Status = "failed"
		report.Error = err.Error()
		return report, err
	}

	for scanner.Scan() {
		report.RowsRead++
		row, err := decodeRow(scanner.Bytes())
		if err != nil {
			report.Status = "failed"
			report.Error = fmt.Sprintf("row %d: %v", report.RowsRead, err)
			return report, err
		}
		row = normalizeExportRow(table, row)
		query, args, err := buildInsert(table, row)
		if err != nil {
			report.Status = "failed"
			report.Error = fmt.Sprintf("row %d: %v", report.RowsRead, err)
			return report, err
		}
		if _, err := db.ExecContext(ctx, query, args...); err != nil {
			report.Status = "failed"
			report.Error = fmt.Sprintf("row %d: %v", report.RowsRead, err)
			return report, err
		}
		report.RowsImported++
	}
	if err := scanner.Err(); err != nil {
		report.Status = "failed"
		report.Error = err.Error()
		return report, err
	}
	return report, nil
}

func decodeRow(data []byte) (map[string]any, error) {
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.UseNumber()
	var row map[string]any
	if err := dec.Decode(&row); err != nil {
		return nil, err
	}
	delete(row, "_row_checksum")
	return row, nil
}

// normalizeExportRow maps old export shapes to current schema shapes.
// It is applied before INSERT/buildInsert so canonical tableColumns match.
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

func buildInsert(table string, row map[string]any) (string, []any, error) {
	allowed, ok := tableColumns[table]
	if !ok {
		return "", nil, fmt.Errorf("unknown canonical table %q", table)
	}

	var cols []string
	var args []any
	for _, col := range allowed {
		value, exists := row[col]
		if !exists {
			continue
		}
		cols = append(cols, col)
		args = append(args, normalizeValue(table, col, value))
	}
	if len(cols) == 0 {
		return "", nil, errors.New("row contains no importable columns")
	}

	quotedCols := make([]string, 0, len(cols))
	placeholders := make([]string, 0, len(cols))
	updates := make([]string, 0, len(cols))
	for _, col := range cols {
		quotedCols = append(quotedCols, quoteIdent(col))
		placeholders = append(placeholders, "?")
		if col != "id" {
			updates = append(updates, fmt.Sprintf("%s=VALUES(%s)", quoteIdent(col), quoteIdent(col)))
		}
	}
	if len(updates) == 0 {
		updates = append(updates, "id=id")
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON DUPLICATE KEY UPDATE %s",
		quoteIdent(table), strings.Join(quotedCols, ", "), strings.Join(placeholders, ", "), strings.Join(updates, ", "))
	return query, args, nil
}

func normalizeValue(table, col string, value any) any {
	if value == nil {
		return nil
	}
	if jsonColumns[table][col] {
		switch v := value.(type) {
		case string:
			if strings.TrimSpace(v) == "" {
				return nil
			}
			return v
		default:
			data, err := json.Marshal(v)
			if err != nil {
				return fmt.Sprint(v)
			}
			return string(data)
		}
	}
	if datetimeColumns[table][col] {
		switch v := value.(type) {
		case string:
			return normalizeDateTime(v)
		default:
			return v
		}
	}
	switch v := value.(type) {
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return i
		}
		if f, err := v.Float64(); err == nil {
			return f
		}
		return v.String()
	default:
		return v
	}
}

func normalizeDateTime(value string) string {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return raw
	}
	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000Z07:00",
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC().Format("2006-01-02 15:04:05.000")
		}
	}
	return raw
}

func quoteIdent(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

func writeReport(report *importReport, outPath string) {
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
