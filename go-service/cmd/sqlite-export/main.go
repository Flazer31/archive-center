package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
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

func main() {
	var (
		dbPath        = flag.String("db", "", "Path to SQLite database")
		outDir        = flag.String("out", "", "Output directory")
		canonicalOnly = flag.Bool("canonical-only", false, "Export only the canonical 8 tables")
		allTables     = flag.Bool("all", false, "Export all user tables")
	)
	flag.Parse()

	canonicalOnlySet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "canonical-only" {
			canonicalOnlySet = true
		}
	})

	effectiveCanonicalOnly := *canonicalOnly || !*allTables
	if canonicalOnlySet && *allTables {
		effectiveCanonicalOnly = true
	}

	if err := runExport(*dbPath, *outDir, effectiveCanonicalOnly, *allTables); err != nil {
		log.Fatal(err)
	}
}

func runExport(dbPath, outDir string, canonicalOnly, allTables bool) error {
	if dbPath == "" {
		return fmt.Errorf("error: -db is required")
	}
	if outDir == "" {
		return fmt.Errorf("error: -out is required")
	}
	if canonicalOnly && allTables {
		return fmt.Errorf("error: -canonical-only and -all are mutually exclusive")
	}

	mode := "canonical-only"
	if allTables {
		mode = "all"
	}

	absDB, err := filepath.Abs(dbPath)
	if err != nil {
		return fmt.Errorf("resolving db path: %w", err)
	}

	if err := os.MkdirAll(outDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	dbURI := "file:" + filepath.ToSlash(absDB) + "?mode=ro"
	db, err := sql.Open("sqlite", dbURI)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("cannot open database read-only: %w", err)
	}

	userTables, err := listUserTables(db)
	if err != nil {
		return fmt.Errorf("listing tables: %w", err)
	}

	tablesToExport := []string{}
	skipped := []string{}
	if mode == "canonical-only" {
		for _, t := range canonicalTables {
			if contains(userTables, t) {
				tablesToExport = append(tablesToExport, t)
			} else {
				skipped = append(skipped, t)
			}
		}
	} else {
		tablesToExport = append([]string{}, userTables...)
	}

	manifest := map[string]interface{}{
		"source_db_path":         dbPath,
		"export_timestamp":       time.Now().UTC().Format("2006-01-02T15:04:05.999999999+00:00"),
		"mode":                   mode,
		"tables_exported":        []string{},
		"skipped_missing_tables": skipped,
		"row_counts":             map[string]int{},
		"checksums":              map[string]string{},
	}

	tablesExported := []string{}
	rowCounts := map[string]int{}
	checksums := map[string]string{}

	for _, tableName := range tablesToExport {
		meta, rows, err := exportTable(db, tableName, dbPath)
		if err != nil {
			return fmt.Errorf("exporting table %s: %w", tableName, err)
		}
		tablesExported = append(tablesExported, tableName)
		rowCounts[tableName] = meta.RowCount
		checksums[tableName] = meta.TableChecksum

		outPath := filepath.Join(outDir, tableName+".ndjson")
		f, err := os.Create(outPath)
		if err != nil {
			return fmt.Errorf("creating output file: %w", err)
		}
		// write meta
		metaObj := map[string]interface{}{"_export_meta": meta}
		metaBytes, err := json.Marshal(metaObj)
		if err != nil {
			f.Close()
			return fmt.Errorf("encoding metadata for %s: %w", tableName, err)
		}
		if _, err := f.Write(metaBytes); err != nil {
			f.Close()
			return fmt.Errorf("writing metadata for %s: %w", tableName, err)
		}
		if _, err := f.Write([]byte("\n")); err != nil {
			f.Close()
			return fmt.Errorf("writing metadata newline for %s: %w", tableName, err)
		}
		// write rows
		for _, row := range rows {
			rowBytes, err := json.Marshal(row)
			if err != nil {
				f.Close()
				return fmt.Errorf("encoding row for %s: %w", tableName, err)
			}
			if _, err := f.Write(rowBytes); err != nil {
				f.Close()
				return fmt.Errorf("writing row for %s: %w", tableName, err)
			}
			if _, err := f.Write([]byte("\n")); err != nil {
				f.Close()
				return fmt.Errorf("writing row newline for %s: %w", tableName, err)
			}
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("closing output file for %s: %w", tableName, err)
		}
	}

	manifest["tables_exported"] = tablesExported
	manifest["row_counts"] = rowCounts
	manifest["checksums"] = checksums

	manifestPath := filepath.Join(outDir, "manifest.json")
	f, err := os.Create(manifestPath)
	if err != nil {
		return fmt.Errorf("creating manifest: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(manifest); err != nil {
		return fmt.Errorf("encoding manifest: %w", err)
	}

	fmt.Printf("Exported %d table(s) to %s\n", len(tablesExported), outDir)
	if len(skipped) > 0 {
		fmt.Printf("Skipped missing table(s): %s\n", strings.Join(skipped, ", "))
	}
	return nil
}

type exportMeta struct {
	TableName       string   `json:"table_name"`
	ExportTimestamp string   `json:"export_timestamp"`
	SourceDBPath    string   `json:"source_db_path"`
	RowCount        int      `json:"row_count"`
	Columns         []string `json:"columns"`
	TableChecksum   string   `json:"table_checksum"`
}

func exportTable(db *sql.DB, tableName, sourceDBPath string) (*exportMeta, []map[string]interface{}, error) {
	columns, err := getColumns(db, tableName)
	if err != nil {
		return nil, nil, err
	}

	quoted := quoteIdent(tableName)
	rows, err := db.Query("SELECT * FROM " + quoted)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var rowChecksums []string
	var exportRows []map[string]interface{}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, nil, err
		}

		rowDict := make(map[string]interface{}, len(columns))
		for i, col := range columns {
			rowDict[col] = values[i]
		}

		rcs := rowChecksum(rowDict)
		rowChecksums = append(rowChecksums, rcs)

		outRow := make(map[string]interface{}, len(rowDict)+1)
		for k, v := range rowDict {
			outRow[k] = v
		}
		outRow["_row_checksum"] = rcs
		exportRows = append(exportRows, outRow)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	tcs := tableChecksum(rowChecksums)

	meta := &exportMeta{
		TableName:       tableName,
		ExportTimestamp: time.Now().UTC().Format("2006-01-02T15:04:05.999999999+00:00"),
		SourceDBPath:    sourceDBPath,
		RowCount:        len(exportRows),
		Columns:         columns,
		TableChecksum:   tcs,
	}

	return meta, exportRows, nil
}

func rowChecksum(rowDict map[string]interface{}) string {
	data := make(map[string]interface{}, len(rowDict))
	for k, v := range rowDict {
		if k == "id" {
			continue
		}
		data[k] = v
	}
	canon := canonicalJSON(data)
	h := sha256.Sum256([]byte(canon))
	return fmt.Sprintf("%x", h)
}

func tableChecksum(rowChecksums []string) string {
	if len(rowChecksums) == 0 {
		return fmt.Sprintf("%x", sha256.Sum256(nil))
	}
	sorted := make([]string, len(rowChecksums))
	copy(sorted, rowChecksums)
	sort.Strings(sorted)
	joined := strings.Join(sorted, "\n")
	h := sha256.Sum256([]byte(joined))
	return fmt.Sprintf("%x", h)
}

func canonicalJSON(data map[string]interface{}) string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	sb.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		sb.Write(kb)
		sb.WriteByte(':')
		vb, _ := json.Marshal(data[k])
		sb.Write(vb)
	}
	sb.WriteByte('}')
	return sb.String()
}

func getColumns(db *sql.DB, tableName string) ([]string, error) {
	quoted := quoteIdent(tableName)
	rows, err := db.Query("PRAGMA table_info(" + quoted + ")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var cid int
		var name string
		var typ string
		var notNull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dfltValue, &pk); err != nil {
			return nil, err
		}
		columns = append(columns, name)
	}
	return columns, rows.Err()
}

func listUserTables(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		if !strings.HasPrefix(name, "sqlite_") {
			tables = append(tables, name)
		}
	}
	return tables, rows.Err()
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
