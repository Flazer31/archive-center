package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestDefaultSchemaIncludesHierarchySummariesContract(t *testing.T) {
	data, err := os.ReadFile(schemaPathForTest(t))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	schema := string(data)
	if !regexp.MustCompile(`CREATE TABLE IF NOT EXISTS chapter_summaries`).MatchString(schema) {
		t.Fatal("chapter_summaries table missing from canonical schema")
	}
	if !regexp.MustCompile(`CREATE TABLE IF NOT EXISTS arc_summaries`).MatchString(schema) {
		t.Fatal("arc_summaries table missing from canonical schema")
	}
	if !regexp.MustCompile(`CREATE TABLE IF NOT EXISTS saga_digests`).MatchString(schema) {
		t.Fatal("saga_digests table missing from canonical schema")
	}
	for _, col := range []string{
		"chat_session_id", "from_turn", "to_turn", "chapter_index",
		"chapter_title", "summary_text", "open_loops_json",
		"relationship_changes_json", "world_changes_json",
		"callback_candidates_json", "resume_text", "embedding_vector",
		"embedding_model", "created_at",
		"arc_index", "arc_name", "arc_status", "core_conflict",
		"key_turning_points_json", "active_promises_json", "unresolved_debts_json",
		"resolved_payoffs_json", "future_payoff_candidates_json", "arc_resume_text",
		"era_label", "saga_summary", "persistent_facts_json",
		"never_drop_candidates_json", "resume_pack_text",
	} {
		if !strings.Contains(schema, col) {
			t.Fatalf("hierarchy summary contract missing column %q", col)
		}
	}
}

func TestDefaultSchemaIncludesSessionMigrationLedgerContract(t *testing.T) {
	data, err := os.ReadFile(schemaPathForTest(t))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	schema := string(data)
	for _, table := range []string{
		"session_migrations",
		"session_migration_row_map",
		"session_migration_locks",
	} {
		if !regexp.MustCompile(`CREATE TABLE IF NOT EXISTS ` + table).MatchString(schema) {
			t.Fatalf("%s table missing from canonical schema", table)
		}
	}
	for _, col := range []string{
		"source_session_id", "target_session_id", "mode", "status",
		"preview_hash", "counts_json", "chroma_reindexed_count",
		"errors_json", "migration_id", "table_name", "source_row_id",
		"target_row_id", "row_status", "locked", "lock_status",
		"migrated_away",
	} {
		if !strings.Contains(schema, col) {
			t.Fatalf("session migration contract missing %q", col)
		}
	}
	for _, forbidden := range []string{
		"UPDATE chat_logs SET chat_session_id",
		"UPDATE memories SET chat_session_id",
		"UPDATE kg_triples SET chat_session_id",
	} {
		if strings.Contains(schema, forbidden) {
			t.Fatalf("schema contains forbidden blind session rewrite marker %q", forbidden)
		}
	}
}

func schemaPathForTest(t *testing.T) string {
	t.Helper()
	candidates := []string{
		defaultSchemaPath(),
		filepath.Join("..", "..", "..", "migrations", "001_schema.sql"),
		filepath.Join("..", "migrations", "001_schema.sql"),
		filepath.Join("migrations", "001_schema.sql"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return candidates[0]
}

func TestFindSchemaUpFindsPackageRootMigrations(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "migrations"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "migrations", "001_schema.sql"), []byte("SET NAMES utf8mb4;"), 0644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(dir, "nested", "package", "bin")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}

	got, ok := findSchemaUp(nested, 4)
	if !ok {
		t.Fatal("schema path was not found")
	}
	want := filepath.Join(dir, "migrations", "001_schema.sql")
	if got != want {
		t.Fatalf("schema path = %q, want %q", got, want)
	}
}

func TestExecuteArgPresentAcceptsInstallerForms(t *testing.T) {
	for _, args := range [][]string{
		{"-execute"},
		{"--execute"},
		{"-execute=true"},
		{"--execute=true"},
		{"-dsn", "user:pass@tcp(127.0.0.1:3307)/archive_center", "-execute"},
	} {
		if !executeArgPresent(args) {
			t.Fatalf("executeArgPresent(%v) = false, want true", args)
		}
	}
	for _, args := range [][]string{
		nil,
		{"-execute=false"},
		{"--execute=false"},
		{"-schema", "migrations/001_schema.sql"},
	} {
		if executeArgPresent(args) {
			t.Fatalf("executeArgPresent(%v) = true, want false", args)
		}
	}
}

func TestSplitSQLStatementsSkipsCommentsAndBlankLines(t *testing.T) {
	sqlText := `
-- comment
SET NAMES utf8mb4;

CREATE TABLE IF NOT EXISTS chat_logs (
  id BIGINT PRIMARY KEY
) ENGINE=InnoDB;
-- another comment
`
	stmts := splitSQLStatements(sqlText)
	if len(stmts) != 2 {
		t.Fatalf("statement count = %d, want 2: %#v", len(stmts), stmts)
	}
	if stmts[0] != "SET NAMES utf8mb4" {
		t.Fatalf("first statement = %q", stmts[0])
	}
	if !regexp.MustCompile(`CREATE TABLE IF NOT EXISTS chat_logs`).MatchString(stmts[1]) {
		t.Fatalf("second statement = %q", stmts[1])
	}
}

func TestSplitSQLStatementsKeepsSemicolonsInsideQuotedStrings(t *testing.T) {
	sqlText := `
CREATE TABLE example (
    id BIGINT PRIMARY KEY
) ENGINE=InnoDB COMMENT='one; two';
CREATE TABLE next_table (
    id BIGINT PRIMARY KEY
) ENGINE=InnoDB;
`
	stmts := splitSQLStatements(sqlText)
	if len(stmts) != 2 {
		t.Fatalf("statement count = %d, want 2: %#v", len(stmts), stmts)
	}
	if !strings.Contains(stmts[0], "COMMENT='one; two'") {
		t.Fatalf("first statement lost quoted semicolon: %q", stmts[0])
	}
	if !strings.Contains(stmts[1], "next_table") {
		t.Fatalf("second statement = %q", stmts[1])
	}
}

func TestSplitSQLStatementsStripsUTF8BOMBeforeComment(t *testing.T) {
	sqlText := "\ufeff-- comment with UTF-8 BOM\r\nSET NAMES utf8mb4;\r\n"
	stmts := splitSQLStatements(sqlText)
	if len(stmts) != 1 {
		t.Fatalf("statement count = %d, want 1: %#v", len(stmts), stmts)
	}
	if stmts[0] != "SET NAMES utf8mb4" {
		t.Fatalf("first statement = %q", stmts[0])
	}
}

func TestRunGuardedWithoutExecuteDoesNotRequireDSN(t *testing.T) {
	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "schema.sql")
	if err := os.WriteFile(schemaPath, []byte("SET NAMES utf8mb4;"), 0644); err != nil {
		t.Fatal(err)
	}

	report, exitCode := run(schemaPath, "", false, time.Second)
	if exitCode != 2 {
		t.Fatalf("exit code = %d, want 2", exitCode)
	}
	if report.Status != "guarded" {
		t.Fatalf("status = %q, want guarded", report.Status)
	}
	if report.StatementsTotal != 1 || report.StatementsRun != 0 {
		t.Fatalf("unexpected statement counts: %+v", report)
	}
}

func TestApplyStatementsExecutesAllStatements(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("SET NAMES utf8mb4")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(regexp.QuoteMeta("CREATE TABLE IF NOT EXISTS chat_logs (id BIGINT PRIMARY KEY)")).
		WillReturnResult(sqlmock.NewResult(0, 0))

	report := newReport("schema.sql", true)
	err = applyStatements(context.Background(), db, []string{
		"SET NAMES utf8mb4",
		"CREATE TABLE IF NOT EXISTS chat_logs (id BIGINT PRIMARY KEY)",
	}, report)
	if err != nil {
		t.Fatalf("applyStatements failed: %v", err)
	}
	if report.StatementsRun != 2 {
		t.Fatalf("statements run = %d, want 2", report.StatementsRun)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyStatementsReportsFailedStatementNumber(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	mock.ExpectExec(regexp.QuoteMeta("SET NAMES utf8mb4")).
		WillReturnError(errors.New("boom"))

	report := newReport("schema.sql", true)
	err = applyStatements(context.Background(), db, []string{"SET NAMES utf8mb4"}, report)
	if err == nil {
		t.Fatal("expected error")
	}
	if !regexp.MustCompile(`statement 1 failed`).MatchString(err.Error()) {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.StatementsRun != 0 {
		t.Fatalf("statements run = %d, want 0", report.StatementsRun)
	}
}

func TestApplyCompatibilityMigrationsAddsStorylineQualityColumns(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	statements := compatibilityMigrationStatements()
	for _, stmt := range statements {
		mock.ExpectExec(regexp.QuoteMeta(stmt)).
			WillReturnResult(sqlmock.NewResult(0, 0))
	}

	report := newReport("schema.sql", true)
	if err := applyCompatibilityMigrations(context.Background(), db, report); err != nil {
		t.Fatalf("applyCompatibilityMigrations failed: %v", err)
	}
	if report.CompatibilityStatementsRun != len(statements) {
		t.Fatalf("compatibility statements run = %d, want %d", report.CompatibilityStatementsRun, len(statements))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCompatibilityMigrationsIncludeSessionMigrationTables(t *testing.T) {
	joined := strings.Join(compatibilityMigrationStatements(), "\n")
	for _, required := range []string{
		"CREATE TABLE IF NOT EXISTS session_migrations",
		"CREATE TABLE IF NOT EXISTS session_migration_row_map",
		"CREATE TABLE IF NOT EXISTS session_migration_locks",
		"copy_then_lock_source",
		"migrated_away",
		"chroma_reindexed_count",
	} {
		if !strings.Contains(joined, required) {
			t.Fatalf("compatibility migration statements missing %q", required)
		}
	}
}
