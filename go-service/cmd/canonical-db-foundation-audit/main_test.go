package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSchemaTables(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "schema.sql")
	sql := `
-- comment
CREATE TABLE IF NOT EXISTS chat_logs (
  id BIGINT PRIMARY KEY
);
CREATE TABLE IF NOT EXISTS ` + "`memories`" + ` (
  id BIGINT PRIMARY KEY
);
`
	if err := os.WriteFile(path, []byte(sql), 0644); err != nil {
		t.Fatal(err)
	}

	tables, err := parseSchemaTables(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 2 || tables[0] != "chat_logs" || tables[1] != "memories" {
		t.Fatalf("unexpected tables: %#v", tables)
	}
}

func TestBuildReportCompleteForFoundationFixture(t *testing.T) {
	root := makeFoundationFixture(t, true)

	rep := buildReport(root)
	if rep.Status != "complete" {
		t.Fatalf("status = %q, errors=%v warnings=%v", rep.Status, rep.Errors, rep.Warnings)
	}
	if rep.FoundationStatus != "complete" {
		t.Fatalf("foundation_status = %q", rep.FoundationStatus)
	}
	if rep.ProductGateGreen {
		t.Fatal("product gate must stay red for an audit-only foundation report")
	}
	if len(rep.Schema.MissingTables) != 0 {
		t.Fatalf("missing schema tables: %v", rep.Schema.MissingTables)
	}
	if len(rep.StoreCoverage.MissingSaveInsertTables) != 0 {
		t.Fatalf("missing save tables: %v", rep.StoreCoverage.MissingSaveInsertTables)
	}
	if len(rep.StoreCoverage.MissingReadSelectTables) != 0 {
		t.Fatalf("missing read tables: %v", rep.StoreCoverage.MissingReadSelectTables)
	}
	if len(rep.ToolCoverage.MissingExecutors) != 0 {
		t.Fatalf("missing executors: %v", rep.ToolCoverage.MissingExecutors)
	}
	if len(rep.ToolCoverage.ImportMissingTables) != 0 {
		t.Fatalf("missing import tables: %v", rep.ToolCoverage.ImportMissingTables)
	}
	if len(rep.ToolCoverage.CompareMissingTables) != 0 {
		t.Fatalf("missing compare tables: %v", rep.ToolCoverage.CompareMissingTables)
	}
	if len(rep.ToolCoverage.MissingGuardChecks) != 0 {
		t.Fatalf("missing guard checks: %v", rep.ToolCoverage.MissingGuardChecks)
	}
}

func TestBuildReportDetectsMissingExecutor(t *testing.T) {
	root := makeFoundationFixture(t, false)

	rep := buildReport(root)
	if rep.Status != "incomplete" {
		t.Fatalf("status = %q, want incomplete", rep.Status)
	}
	if len(rep.ToolCoverage.MissingExecutors) == 0 {
		t.Fatal("expected missing executor")
	}
}

func makeFoundationFixture(t *testing.T, includeAllExecutors bool) string {
	t.Helper()
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "go-service", "go.mod"), "module fixture\n")
	mustWrite(t, filepath.Join(root, "docs", "2.0-prep-readiness.md"), "# readiness\n")
	mustWrite(t, filepath.Join(root, "migrations", "001_schema.sql"), schemaFor(canonicalTables))
	mustWrite(t, filepath.Join(root, "go-service", "internal", "store", "mariadb.go"), mariadbStoreSourceFor(canonicalTables, storeSaveTables, storeReadTables))
	mustWrite(t, filepath.Join(root, "go-service", "internal", "store", "store.go"), storeInterfaceSource())
	mustWrite(t, filepath.Join(root, "go-service", "cmd", "mariadb-import", "main.go"), canonicalArraySource()+"\n// --execute is required\n")
	mustWrite(t, filepath.Join(root, "go-service", "cmd", "mariadb-compare", "main.go"), canonicalArraySource())
	mustWrite(t, filepath.Join(root, "go-service", "cmd", "mariadb-schema", "main.go"), "// --execute is required\n")
	mustWrite(t, filepath.Join(root, "go-service", "cmd", "managed-mariadb-e2e", "main.go"), "// does not accept a user-prepared DSN\n")
	mustWrite(t, filepath.Join(root, "go-service", "internal", "config", "config.go"), "mariadb_read_shadow\nnot allowed in this slice\n")

	for _, ex := range requiredExecutors {
		if !includeAllExecutors && ex.Name == "mariadb_shadow_smoke" {
			continue
		}
		p := filepath.Join(root, ex.Path)
		if fileExists(p) {
			continue
		}
		mustWrite(t, p, "// executor\n")
	}
	return root
}

func schemaFor(tables []string) string {
	var out string
	for _, table := range tables {
		out += "CREATE TABLE IF NOT EXISTS " + table + " (\n  id BIGINT PRIMARY KEY\n);\n"
	}
	return out
}

func mariadbStoreSourceFor(allTables, saveTables, readTables []string) string {
	var out string
	for _, table := range saveTables {
		out += "INSERT INTO " + table + "\n"
	}
	for _, table := range readTables {
		out += "FROM " + table + "\n"
	}
	return out
}

func canonicalArraySource() string {
	out := "package main\n\nvar canonicalTables = []string{\n"
	for _, table := range canonicalTables {
		out += "\t\"" + table + "\",\n"
	}
	return out + "}\n"
}

func storeInterfaceSource() string {
	return `
package store

type Store interface {
	SaveChatLog(); ListChatLogs()
	SaveEffectiveInput(); GetEffectiveInput()
	SaveMemory(); ListMemories()
	SaveEvidence(); ListEvidence()
	SaveKGTriple(); ListKGTriples()
	SaveAuditLog(); ListAuditLogs()
	SaveCriticFeedback(); ListCriticFeedback()
	SaveCharacterEvent(); ListCharacterEvents()
	ListStorylines()
	ListWorldRules(); ListInheritedWorldRules()
	ListCharacterStates(); GetCharacterState()
	ListPendingThreads()
	ListActiveStates()
	ListCanonicalStateLayers()
	ListEpisodeSummaries(); GetEpisodeSummary()
}
`
}

func mustWrite(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
