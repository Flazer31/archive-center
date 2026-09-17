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
	if !strings.Contains(compatibility, "memory_kind IN ('event', 'state', 'utterance', 'observation', 'boundary', 'profile')") {
		t.Fatal("compatibility schema does not permit all production precise-memory kinds")
	}
	if !strings.Contains(compatibility, "DROP CONSTRAINT IF EXISTS chk_precise_memory_kind") {
		t.Fatal("compatibility schema does not repair an existing restrictive memory kind constraint")
	}
	for label, sqlText := range map[string]string{"fresh": fresh, "standalone": migration, "compatibility": compatibility} {
		if !strings.Contains(sqlText, "root_evidence_id           BIGINT UNSIGNED NULL") ||
			!strings.Contains(sqlText, "fk_precise_memory_root_evidence FOREIGN KEY (root_evidence_id) REFERENCES direct_evidence_records(id) ON DELETE SET NULL") {
			t.Fatalf("%s schema can destroy precise history when root evidence is removed", label)
		}
		for _, forbidden := range []string{
			"fk_precise_memory_root_evidence FOREIGN KEY (root_evidence_id) REFERENCES direct_evidence_records(id) ON DELETE CASCADE",
			"fk_precise_memory_actor FOREIGN KEY (actor_entity_id) REFERENCES entity_identities(stable_entity_id) ON DELETE RESTRICT",
			"fk_precise_memory_subject FOREIGN KEY (subject_entity_id) REFERENCES entity_identities(stable_entity_id) ON DELETE RESTRICT",
			"fk_precise_memory_affected FOREIGN KEY (affected_entity_id) REFERENCES entity_identities(stable_entity_id) ON DELETE RESTRICT",
			"fk_precise_memory_location FOREIGN KEY (location_entity_id) REFERENCES entity_identities(stable_entity_id) ON DELETE RESTRICT",
			"fk_precise_memory_object FOREIGN KEY (object_entity_id) REFERENCES entity_identities(stable_entity_id) ON DELETE RESTRICT",
			"fk_precise_memory_knower FOREIGN KEY (knowledge_holder_entity_id) REFERENCES entity_identities(stable_entity_id) ON DELETE RESTRICT",
		} {
			if strings.Contains(sqlText, forbidden) {
				t.Fatalf("%s precise schema retains history-destroying foreign-key action %q", label, forbidden)
			}
		}
	}
}
