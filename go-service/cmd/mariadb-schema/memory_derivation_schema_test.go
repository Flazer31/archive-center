package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMemoryDerivationSchemaIsFreshStandaloneAndCompatible(t *testing.T) {
	freshPath := filepath.Join("..", "..", "..", "migrations", "001_schema.sql")
	standalonePath := filepath.Join("..", "..", "..", "migrations", "005_memory_derivation_lifecycle.sql")
	admissionPath := filepath.Join("..", "..", "..", "migrations", "006_memory_admission_writer.sql")
	fresh, err := os.ReadFile(freshPath)
	if err != nil {
		t.Fatal(err)
	}
	standalone, err := os.ReadFile(standalonePath)
	if err != nil {
		t.Fatal(err)
	}
	admission, err := os.ReadFile(admissionPath)
	if err != nil {
		t.Fatal(err)
	}
	compatibility := strings.Join(memoryDerivationSchemaStatements(), "\n")
	required := []string{
		"memory_source_revision.v1",
		"memory_derivation_dependency.v1",
		"memory_reprocessing_job.v1",
		"memory_vector_outbox.v1",
		"raw_user_content LONGTEXT NOT NULL",
		"raw_assistant_content LONGTEXT NOT NULL",
		"branch_state VARCHAR(50) NOT NULL DEFAULT 'not_exposed'",
		"active_logical_turn_slot",
		"uq_memory_source_active_turn",
		"derived_admission_state",
		"derived_admission_version",
		"derived_result_hash",
		"derived_result_json",
		"derived_admitted_at",
		"'pending', 'leased', 'retryable', 'permanent', 'completed', 'stale_rejected'",
		"'needs_embedding'",
		"required_source_state",
		"ON DELETE SET NULL",
	}
	for label, body := range map[string]string{
		"fresh":         string(fresh),
		"standalone":    string(standalone) + "\n" + string(admission),
		"compatibility": compatibility,
	} {
		for _, needle := range required {
			if !strings.Contains(body, needle) {
				t.Errorf("%s schema missing %q", label, needle)
			}
		}
	}
}
