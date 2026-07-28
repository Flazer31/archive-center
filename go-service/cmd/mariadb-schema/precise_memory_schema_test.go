package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPreciseMemorySchemaIsFreshStandaloneAndCompatible(t *testing.T) {
	freshBytes, err := os.ReadFile(schemaPathForTest(t))
	if err != nil {
		t.Fatalf("read fresh schema: %v", err)
	}
	migrationPath := filepath.Join("..", "..", "..", "migrations", "004_precise_memory_units.sql")
	migrationBytes, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read precise memory migration: %v", err)
	}
	fresh := string(freshBytes)
	migration := string(migrationBytes)
	compatibility := strings.Join(preciseMemorySchemaStatements(), "\n")
	for label, sqlText := range map[string]string{
		"fresh": fresh, "standalone": migration, "compatibility": compatibility,
	} {
		for _, required := range []string{
			"CREATE TABLE IF NOT EXISTS precise_memory_units",
			"precise_memory_unit.v1",
			"source_revision",
			"source_span_start",
			"source_span_end",
			"root_evidence_id",
			"direct_evidence_ids_json",
			"authority_class",
			"admission_state",
			"review_state",
			"visibility",
			"idempotency_key",
			"fk_precise_memory_root_evidence",
			"fk_precise_memory_subject",
		} {
			if !strings.Contains(sqlText, required) {
				t.Fatalf("%s schema missing %q", label, required)
			}
		}
	}
	if !strings.Contains(compatibility, "memory_kind IN ('event', 'state', 'utterance', 'observation')") {
		t.Fatal("compatibility schema does not preserve the minimum semantic kinds")
	}
}
