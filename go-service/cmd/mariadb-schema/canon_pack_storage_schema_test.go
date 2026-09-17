package main

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

var canonPackStorageTables = []string{
	"reference_work_editions",
	"canon_pack_installs",
	"reference_source_observations",
	"reference_work_titles",
	"reference_item_origins",
	"reference_item_evidence",
	"reference_logical_facts",
	"reference_fact_identities",
	"reference_overlay_rules",
	"source_discovery_jobs",
}

func TestCanonPackStorageProductionRegistrationMatchesMigration(t *testing.T) {
	migrationStatements, err := loadStatements(canonPackStorageMigrationPath(t))
	if err != nil {
		t.Fatal(err)
	}
	compatibilityStatements := canonPackStorageSchemaStatements()
	if len(compatibilityStatements) != len(migrationStatements) {
		t.Fatalf("compatibility statement count=%d want=%d", len(compatibilityStatements), len(migrationStatements))
	}
	for i := range migrationStatements {
		if normalizeCanonPackSQL(compatibilityStatements[i]) != normalizeCanonPackSQL(migrationStatements[i]) {
			t.Fatalf("compatibility statement %d differs from 002 migration", i+1)
		}
	}

	schemaPath, ok := findSchemaUp(".", 6)
	if !ok {
		t.Fatal("fresh-install schema was not found")
	}
	freshStatements, err := loadStatements(schemaPath)
	if err != nil {
		t.Fatal(err)
	}
	freshByTable := canonPackCreateStatementsByTable(freshStatements)
	for i, table := range canonPackStorageTables {
		statements := freshByTable[table]
		if len(statements) != 1 {
			t.Fatalf("fresh schema table %s statement count=%d want=1", table, len(statements))
		}
		if normalizeCanonPackSQL(statements[0]) != normalizeCanonPackSQL(migrationStatements[i]) {
			t.Fatalf("fresh schema table %s differs from 002 migration", table)
		}
	}

	allCompatibility := compatibilityMigrationStatements()
	start := -1
	for i, statement := range allCompatibility {
		if canonPackCreateTableName(statement) == canonPackStorageTables[0] {
			start = i
			break
		}
	}
	if start < 0 || start+len(canonPackStorageTables) > len(allCompatibility) {
		t.Fatal("Canon Pack compatibility statements are not registered")
	}
	for i, table := range canonPackStorageTables {
		if got := canonPackCreateTableName(allCompatibility[start+i]); got != table {
			t.Fatalf("compatibility FK order[%d]=%q want=%q", i, got, table)
		}
	}
}

func normalizeCanonPackSQL(statement string) string {
	return strings.Join(strings.Fields(statement), " ")
}

func canonPackCreateStatementsByTable(statements []string) map[string][]string {
	out := map[string][]string{}
	for _, statement := range statements {
		if table := canonPackCreateTableName(statement); table != "" {
			out[table] = append(out[table], statement)
		}
	}
	return out
}

func canonPackCreateTableName(statement string) string {
	fields := strings.Fields(statement)
	if len(fields) < 6 || !strings.EqualFold(strings.Join(fields[:5], " "), "CREATE TABLE IF NOT EXISTS") {
		return ""
	}
	return strings.Trim(fields[5], "`")
}

func canonPackStorageMigrationPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "..", "migrations", "002_canon_pack_storage.sql")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("canon pack migration path: %v", err)
	}
	return path
}

func TestCanonPackStorageMigrationIsAdditiveAndComplete(t *testing.T) {
	statements, err := loadStatements(canonPackStorageMigrationPath(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(statements) != len(canonPackStorageTables) {
		t.Fatalf("statement count=%d want=%d", len(statements), len(canonPackStorageTables))
	}

	joined := strings.Join(statements, "\n")
	for _, table := range canonPackStorageTables {
		marker := "CREATE TABLE IF NOT EXISTS " + table
		if strings.Count(joined, marker) != 1 {
			t.Fatalf("migration must create %s exactly once", table)
		}
	}
	for _, statement := range statements {
		upper := strings.ToUpper(strings.TrimSpace(statement))
		if !strings.HasPrefix(upper, "CREATE TABLE IF NOT EXISTS ") {
			t.Fatalf("non-additive statement: %s", statement)
		}
	}

	requiredFragments := []string{
		"chk_reference_item_origin_target",
		"chk_reference_item_origin_install_owner",
		"chk_reference_item_evidence_target",
		"chk_reference_source_install_owner",
		"chk_reference_overlay_target",
		"rule_status = 'dormant'",
		"fk_reference_source_install FOREIGN KEY (install_id) REFERENCES canon_pack_installs(install_id) ON DELETE RESTRICT",
		"fk_reference_title_edition FOREIGN KEY (edition_row_id) REFERENCES reference_work_editions(edition_row_id) ON DELETE RESTRICT",
		"edition_scope_key CHAR(36) NOT NULL DEFAULT ''",
		"chk_reference_title_edition_scope",
		"fk_reference_evidence_source FOREIGN KEY (source_observation_id) REFERENCES reference_source_observations(observation_id) ON DELETE RESTRICT",
		"fk_reference_overlay_target_fact FOREIGN KEY (target_logical_fact_id) REFERENCES reference_logical_facts(logical_fact_id) ON DELETE RESTRICT",
		"fk_reference_overlay_target_entity FOREIGN KEY (target_entity_id) REFERENCES reference_entities(entity_id) ON DELETE RESTRICT",
		"fk_reference_overlay_target_node FOREIGN KEY (target_node_id) REFERENCES reference_timeline_nodes(node_id) ON DELETE RESTRICT",
		"fk_reference_fact_identity_claim FOREIGN KEY (claim_id) REFERENCES reference_claims(claim_id) ON DELETE CASCADE",
		"active_generation_marker TINYINT AS (CASE WHEN lifecycle_status = 'active' THEN 1 ELSE NULL END) STORED",
		"uq_canon_pack_active_slot (pack_id, edition_row_id, active_generation_marker)",
		"uq_reference_node_evidence (node_id, source_observation_id, document_hash_contract, document_sha256, locator_digest)",
		"uq_reference_entity_evidence (entity_id, source_observation_id, document_hash_contract, document_sha256, locator_digest)",
		"uq_reference_claim_evidence (claim_id, source_observation_id, document_hash_contract, document_sha256, locator_digest)",
		"uq_reference_work_title_identity",
		"normalized_lookup_digest CHAR(64) NOT NULL",
		"uq_reference_fact_fingerprint (fingerprint_contract, exact_fingerprint)",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(joined, fragment) {
			t.Fatalf("migration contract missing %q", fragment)
		}
	}
}

func TestCanonPackStorageMigrationUsesProductionLoaderAndIsRetryable(t *testing.T) {
	statements, err := loadStatements(canonPackStorageMigrationPath(t))
	if err != nil {
		t.Fatal(err)
	}
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for attempt := 0; attempt < 2; attempt++ {
		for _, statement := range statements {
			mock.ExpectExec(regexp.QuoteMeta(statement)).WillReturnResult(sqlmock.NewResult(0, 0))
		}
		report := newReport(canonPackStorageMigrationPath(t), true)
		if err := applyStatements(context.Background(), db, statements, report); err != nil {
			t.Fatalf("attempt %d: %v", attempt+1, err)
		}
		if report.StatementsRun != len(statements) {
			t.Fatalf("attempt %d ran %d statements want %d", attempt+1, report.StatementsRun, len(statements))
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
