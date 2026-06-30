// mariadb-schema applies the Archive Center 2.0 MariaDB schema.
//
// This is a real target execution command for the MariaDB migration lane. It
// refuses to touch a database unless --execute and a DSN are both provided.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type schemaReport struct {
	Status                     string   `json:"status"`
	SchemaPath                 string   `json:"schema_path"`
	Executed                   bool     `json:"executed"`
	GeneratedAt                string   `json:"generated_at"`
	StatementsTotal            int      `json:"statements_total"`
	StatementsRun              int      `json:"statements_run"`
	CompatibilityStatementsRun int      `json:"compatibility_statements_run,omitempty"`
	Errors                     []string `json:"errors,omitempty"`
}

type sqlExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func main() {
	dsn := flag.String("dsn", os.Getenv("AC_MARIADB_DSN"), "MariaDB DSN. Defaults to AC_MARIADB_DSN.")
	schemaPath := flag.String("schema", defaultSchemaPath(), "Path to schema SQL file.")
	outPath := flag.String("out", "", "Path to write schema JSON report. Defaults to stdout.")
	execute := flag.Bool("execute", false, "Required to apply schema statements.")
	timeout := flag.Duration("timeout", 60*time.Second, "Schema apply timeout.")
	flag.Parse()
	executeRequested := *execute || executeArgPresent(os.Args[1:])

	absSchemaPath, err := filepath.Abs(*schemaPath)
	if err != nil {
		report := newReport(*schemaPath, executeRequested)
		report.Status = "failed"
		report.Errors = append(report.Errors, fmt.Sprintf("resolve schema path: %v", err))
		writeReport(report, *outPath)
		os.Exit(1)
	}

	report, exitCode := run(absSchemaPath, *dsn, executeRequested, *timeout)
	writeReport(report, *outPath)
	os.Exit(exitCode)
}

func executeArgPresent(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "-execute", "--execute", "-execute=true", "--execute=true":
			return true
		}
	}
	return false
}

func defaultSchemaPath() string {
	const schemaRel = "migrations/001_schema.sql"
	if workDir, err := os.Getwd(); err == nil {
		if schemaPath, ok := findSchemaUp(workDir, 6); ok {
			return schemaPath
		}
	}
	if exePath, err := os.Executable(); err == nil {
		if schemaPath, ok := findSchemaUp(filepath.Dir(exePath), 6); ok {
			return schemaPath
		}
	}

	return filepath.FromSlash(schemaRel)
}

func findSchemaUp(startDir string, maxDepth int) (string, bool) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", false
	}
	for i := 0; i <= maxDepth; i++ {
		candidate := filepath.Join(dir, "migrations", "001_schema.sql")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", false
}

func newReport(schemaPath string, executed bool) *schemaReport {
	return &schemaReport{
		Status:      "ok",
		SchemaPath:  schemaPath,
		Executed:    executed,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

func run(schemaPath, dsn string, execute bool, timeout time.Duration) (*schemaReport, int) {
	report := newReport(schemaPath, execute)
	statements, err := loadStatements(schemaPath)
	if err != nil {
		report.Status = "failed"
		report.Errors = append(report.Errors, err.Error())
		return report, 1
	}
	report.StatementsTotal = len(statements)

	if !execute {
		report.Status = "guarded"
		report.Errors = append(report.Errors, "--execute is required before schema statements are applied")
		return report, 2
	}
	if strings.TrimSpace(dsn) == "" {
		report.Status = "failed"
		report.Errors = append(report.Errors, "missing DSN: provide --dsn or AC_MARIADB_DSN")
		return report, 2
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		report.Status = "failed"
		report.Errors = append(report.Errors, fmt.Sprintf("open db: %v", err))
		return report, 1
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := applyStatements(ctx, db, statements, report); err != nil {
		report.Status = "failed"
		report.Errors = append(report.Errors, err.Error())
		return report, 1
	}
	if err := applyCompatibilityMigrations(ctx, db, report); err != nil {
		report.Status = "failed"
		report.Errors = append(report.Errors, err.Error())
		return report, 1
	}
	return report, 0
}

func loadStatements(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read schema: %w", err)
	}
	return splitSQLStatements(strings.TrimPrefix(string(data), "\ufeff")), nil
}

func splitSQLStatements(sqlText string) []string {
	var cleaned []string
	for _, line := range strings.Split(sqlText, "\n") {
		line = strings.TrimPrefix(line, "\ufeff")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		cleaned = append(cleaned, line)
	}

	var out []string
	for _, part := range strings.Split(strings.Join(cleaned, "\n"), ";") {
		stmt := strings.TrimSpace(part)
		if stmt != "" {
			out = append(out, stmt)
		}
	}
	return out
}

func applyStatements(ctx context.Context, db sqlExecer, statements []string, report *schemaReport) error {
	for i, stmt := range statements {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("statement %d failed: %w", i+1, err)
		}
		report.StatementsRun++
	}
	return nil
}

func compatibilityMigrationStatements() []string {
	return []string{
		"ALTER TABLE storylines ADD COLUMN IF NOT EXISTS confidence DOUBLE",
		"ALTER TABLE storylines ADD COLUMN IF NOT EXISTS evidence_count INT",
		"ALTER TABLE storylines ADD COLUMN IF NOT EXISTS last_evidence_turn INT",
		"CREATE TABLE IF NOT EXISTS guidance_plan_states (id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, chat_session_id VARCHAR(255) NOT NULL, story_plan_json LONGTEXT, director_json LONGTEXT, state_status VARCHAR(50) NOT NULL DEFAULT 'empty', last_turn INT NOT NULL DEFAULT -1, warnings_json LONGTEXT, created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL, updated_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL, UNIQUE KEY uq_guidance_plan_session (chat_session_id(180)), INDEX idx_guidance_plan_updated (updated_at)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci",
		"CREATE TABLE IF NOT EXISTS chapter_summaries (id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, chat_session_id VARCHAR(255) NOT NULL, from_turn INT NOT NULL, to_turn INT NOT NULL, chapter_index INT NOT NULL DEFAULT 0, chapter_title VARCHAR(500), summary_text LONGTEXT NOT NULL, open_loops_json JSON, relationship_changes_json JSON, world_changes_json JSON, callback_candidates_json JSON, resume_text LONGTEXT, embedding_vector JSON, embedding_model VARCHAR(255), created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL, INDEX idx_session_turns (chat_session_id, from_turn, to_turn), INDEX idx_session_chapter (chat_session_id, chapter_index)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci",
		"CREATE TABLE IF NOT EXISTS arc_summaries (id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, chat_session_id VARCHAR(255) NOT NULL, from_turn INT NOT NULL, to_turn INT NOT NULL, arc_index INT NOT NULL DEFAULT 0, arc_name VARCHAR(500), arc_status VARCHAR(50) NOT NULL DEFAULT 'active', core_conflict LONGTEXT, key_turning_points_json JSON, active_promises_json JSON, unresolved_debts_json JSON, resolved_payoffs_json JSON, callback_candidates_json JSON, future_payoff_candidates_json JSON, irreversible_turns_json JSON, callback_debts_json JSON, relationship_pivots_json JSON, arc_resume_text LONGTEXT, embedding_vector JSON, embedding_model VARCHAR(255), created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL, INDEX idx_session_turns (chat_session_id, from_turn, to_turn), INDEX idx_session_arc (chat_session_id, arc_index), INDEX idx_session_status (chat_session_id, arc_status)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci",
		"CREATE TABLE IF NOT EXISTS saga_digests (id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, chat_session_id VARCHAR(255) NOT NULL, from_turn INT NOT NULL, to_turn INT NOT NULL, era_label VARCHAR(500), saga_summary LONGTEXT NOT NULL, persistent_facts_json JSON, never_drop_candidates_json JSON, resume_pack_text LONGTEXT, embedding_vector JSON, embedding_model VARCHAR(255), created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL, INDEX idx_session_turns (chat_session_id, from_turn, to_turn), INDEX idx_session_created (chat_session_id, created_at)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci",
		"ALTER TABLE arc_summaries ADD COLUMN IF NOT EXISTS irreversible_turns_json JSON",
		"ALTER TABLE arc_summaries ADD COLUMN IF NOT EXISTS callback_debts_json JSON",
		"ALTER TABLE arc_summaries ADD COLUMN IF NOT EXISTS relationship_pivots_json JSON",
		"ALTER TABLE persona_memory_entries ADD COLUMN IF NOT EXISTS source_memory_type VARCHAR(80) NULL",
		"ALTER TABLE persona_memory_entries ADD COLUMN IF NOT EXISTS source_memory_id BIGINT UNSIGNED NULL",
		"ALTER TABLE persona_memory_entries ADD INDEX IF NOT EXISTS idx_source_memory_ref (source_memory_type, source_memory_id)",
		"ALTER TABLE protagonist_entity_memories ADD COLUMN IF NOT EXISTS owner_entity_key VARCHAR(255) NOT NULL DEFAULT ''",
		"ALTER TABLE protagonist_entity_memories ADD COLUMN IF NOT EXISTS owner_entity_name VARCHAR(255) NOT NULL DEFAULT ''",
		"ALTER TABLE protagonist_entity_memories ADD COLUMN IF NOT EXISTS owner_entity_role VARCHAR(80) NOT NULL DEFAULT 'protagonist'",
		"ALTER TABLE protagonist_entity_memories ADD COLUMN IF NOT EXISTS owner_visibility VARCHAR(80) NOT NULL DEFAULT 'player_known'",
		"ALTER TABLE protagonist_entity_memories ADD COLUMN IF NOT EXISTS target_reveal_policy VARCHAR(120) NOT NULL DEFAULT 'requires_explicit_attachment'",
		"UPDATE protagonist_entity_memories SET owner_entity_key = persona_entity_key, owner_entity_name = persona_entity_name WHERE owner_entity_key = ''",
		"CREATE TABLE IF NOT EXISTS session_migrations (id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, source_session_id VARCHAR(255) NOT NULL, target_session_id VARCHAR(255) NOT NULL, mode VARCHAR(80) NOT NULL DEFAULT 'copy_then_lock_source', status VARCHAR(50) NOT NULL DEFAULT 'previewed', preview_hash VARCHAR(128) NULL, operator_note TEXT, counts_json JSON, chroma_reindexed_count INT NOT NULL DEFAULT 0, errors_json JSON, started_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL, completed_at DATETIME(3) NULL, locked_at DATETIME(3) NULL, cleanup_at DATETIME(3) NULL, created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL, updated_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL, INDEX idx_session_migration_source (source_session_id, status), INDEX idx_session_migration_target (target_session_id, status), INDEX idx_session_migration_status (status, updated_at)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci",
		"CREATE TABLE IF NOT EXISTS session_migration_row_map (id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, migration_id BIGINT UNSIGNED NOT NULL, table_name VARCHAR(100) NOT NULL, source_row_id BIGINT UNSIGNED NOT NULL, target_row_id BIGINT UNSIGNED NULL, row_status VARCHAR(50) NOT NULL DEFAULT 'copied', created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL, UNIQUE KEY uq_session_migration_row (migration_id, table_name, source_row_id), INDEX idx_session_migration_target_row (table_name, target_row_id), CONSTRAINT fk_session_migration_row_map_migration FOREIGN KEY (migration_id) REFERENCES session_migrations(id) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci",
		"CREATE TABLE IF NOT EXISTS session_migration_locks (id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, migration_id BIGINT UNSIGNED NOT NULL, source_session_id VARCHAR(255) NOT NULL, target_session_id VARCHAR(255) NOT NULL, locked BOOLEAN NOT NULL DEFAULT TRUE, lock_status VARCHAR(50) NOT NULL DEFAULT 'migrated_away', reason TEXT, locked_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL, unlocked_at DATETIME(3) NULL, created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL, updated_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL, INDEX idx_session_migration_lock_source (source_session_id, locked), INDEX idx_session_migration_lock_target (target_session_id, locked), INDEX idx_session_migration_lock_status (lock_status, updated_at), CONSTRAINT fk_session_migration_locks_migration FOREIGN KEY (migration_id) REFERENCES session_migrations(id) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci",
		"CREATE TABLE IF NOT EXISTS status_schema_proposals (id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, chat_session_id VARCHAR(255) NOT NULL, input_channel VARCHAR(50) NOT NULL DEFAULT 'bootstrap', proposal_state VARCHAR(50) NOT NULL DEFAULT 'pending_review', schema_name VARCHAR(255) NOT NULL, ruleset_label VARCHAR(255) NULL, schema_json JSON NOT NULL, provenance_json JSON NULL, review_note TEXT NULL, reviewer VARCHAR(255) NULL, reviewed_at DATETIME(3) NULL, created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL, updated_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL, INDEX idx_status_schema_session (chat_session_id, updated_at), INDEX idx_status_schema_state (chat_session_id, proposal_state, updated_at), INDEX idx_status_schema_input_channel (chat_session_id, input_channel, updated_at)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci",
		"CREATE TABLE IF NOT EXISTS status_schema_registry (id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, chat_session_id VARCHAR(255) NOT NULL, source_proposal_id BIGINT UNSIGNED NULL, schema_name VARCHAR(255) NOT NULL DEFAULT 'status_schema', ruleset_label VARCHAR(255) NULL, status_key VARCHAR(255) NOT NULL, label VARCHAR(255) NOT NULL, owner_scope VARCHAR(80) NOT NULL, value_kind VARCHAR(80) NOT NULL, bounds_json JSON NULL, options_json JSON NULL, default_value_json JSON NULL, registry_state VARCHAR(50) NOT NULL DEFAULT 'active', created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL, updated_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL, UNIQUE KEY uq_status_registry_key (chat_session_id(180), schema_name(120), status_key(120), owner_scope), INDEX idx_status_registry_session (chat_session_id, registry_state, status_key), INDEX idx_status_registry_proposal (source_proposal_id), CONSTRAINT fk_status_registry_proposal FOREIGN KEY (source_proposal_id) REFERENCES status_schema_proposals(id) ON DELETE SET NULL) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci",
		"CREATE TABLE IF NOT EXISTS status_current_values (id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, chat_session_id VARCHAR(255) NOT NULL, registry_id BIGINT UNSIGNED NOT NULL, status_key VARCHAR(255) NOT NULL, owner_scope VARCHAR(80) NOT NULL, owner_id VARCHAR(255) NOT NULL, owner_label VARCHAR(255) NULL, value_kind VARCHAR(80) NOT NULL, value_json JSON NOT NULL, evidence_json JSON NOT NULL, source_turn INT NULL, write_state VARCHAR(50) NOT NULL DEFAULT 'current', created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL, updated_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL, UNIQUE KEY uq_status_current_owner (chat_session_id(180), registry_id, owner_scope, owner_id(180)), INDEX idx_status_current_session (chat_session_id, write_state, updated_at), INDEX idx_status_current_owner (chat_session_id, owner_scope, owner_id(180), status_key(120)), INDEX idx_status_current_key (chat_session_id, status_key(120), owner_scope), CONSTRAINT fk_status_current_registry FOREIGN KEY (registry_id) REFERENCES status_schema_registry(id) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci",
		"CREATE TABLE IF NOT EXISTS status_change_events (id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, chat_session_id VARCHAR(255) NOT NULL, registry_id BIGINT UNSIGNED NOT NULL, status_value_id BIGINT UNSIGNED NULL, status_key VARCHAR(255) NOT NULL, owner_scope VARCHAR(80) NOT NULL, owner_id VARCHAR(255) NOT NULL, event_kind VARCHAR(80) NOT NULL, previous_value_json JSON NULL, new_value_json JSON NULL, evidence_json JSON NOT NULL, source_turn INT NULL, story_clock_json JSON NULL, event_state VARCHAR(50) NOT NULL DEFAULT 'recorded', created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL, INDEX idx_status_event_session (chat_session_id, created_at), INDEX idx_status_event_owner (chat_session_id, owner_scope, owner_id(180), status_key(120), created_at), INDEX idx_status_event_registry (registry_id, created_at), CONSTRAINT fk_status_event_registry FOREIGN KEY (registry_id) REFERENCES status_schema_registry(id) ON DELETE CASCADE, CONSTRAINT fk_status_event_current FOREIGN KEY (status_value_id) REFERENCES status_current_values(id) ON DELETE SET NULL) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci",
		"CREATE TABLE IF NOT EXISTS status_effects (id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY, chat_session_id VARCHAR(255) NOT NULL, registry_id BIGINT UNSIGNED NOT NULL, status_key VARCHAR(255) NOT NULL, owner_scope VARCHAR(80) NOT NULL, owner_id VARCHAR(255) NOT NULL, effect_kind VARCHAR(80) NOT NULL, effect_label VARCHAR(255) NULL, effect_payload_json JSON NULL, evidence_json JSON NOT NULL, source_turn INT NULL, start_clock_json JSON NOT NULL, duration_json JSON NULL, expires_at_clock_json JSON NULL, effect_state VARCHAR(50) NOT NULL DEFAULT 'active', cleared_evidence_json JSON NULL, cleared_turn INT NULL, created_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) NOT NULL, updated_at DATETIME(3) DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) NOT NULL, INDEX idx_status_effect_session (chat_session_id, effect_state, updated_at), INDEX idx_status_effect_owner (chat_session_id, owner_scope, owner_id(180), status_key(120), effect_state), INDEX idx_status_effect_registry (registry_id, effect_state), CONSTRAINT fk_status_effect_registry FOREIGN KEY (registry_id) REFERENCES status_schema_registry(id) ON DELETE CASCADE) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci",
	}
}

func applyCompatibilityMigrations(ctx context.Context, db sqlExecer, report *schemaReport) error {
	statements := compatibilityMigrationStatements()
	for i, stmt := range statements {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("compatibility statement %d failed: %w", i+1, err)
		}
		report.CompatibilityStatementsRun++
	}
	return nil
}

func writeReport(report *schemaReport, outPath string) {
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
