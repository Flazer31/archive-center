package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test36BEntityIdentitySchemaIsFreshAndCompatibilityAdditive(t *testing.T) {
	freshBytes, err := os.ReadFile(schemaPathForTest(t))
	if err != nil {
		t.Fatalf("read fresh schema: %v", err)
	}
	migrationPath := filepath.Join("..", "..", "..", "migrations", "003_entity_identity_v1.sql")
	migrationBytes, err := os.ReadFile(migrationPath)
	if err != nil {
		t.Fatalf("read 3.6-B migration: %v", err)
	}
	fresh := string(freshBytes)
	migration := string(migrationBytes)
	for _, table := range []string{
		"entity_identities",
		"entity_identity_surfaces",
		"entity_identity_links",
		"entity_identity_artifact_bindings",
		"speaker_attributions",
	} {
		marker := "CREATE TABLE IF NOT EXISTS " + table
		if !strings.Contains(fresh, marker) {
			t.Fatalf("fresh schema missing %s", table)
		}
		if !strings.Contains(migration, marker) {
			t.Fatalf("3.6-B migration missing %s", table)
		}
	}
	for _, field := range []string{
		"stable_entity_id",
		"identity_namespace",
		"lifecycle_state",
		"review_state",
		"source_revision",
		"source_span_start",
		"source_span_end",
		"evidence_excerpt",
		"idempotency_key",
	} {
		if !strings.Contains(fresh, field) || !strings.Contains(migration, field) {
			t.Fatalf("identity schema missing %q", field)
		}
	}
	if strings.Contains(migration, "UNIQUE KEY uq_entity_identity_name") ||
		strings.Contains(migration, "UNIQUE KEY uq_entity_surface_lookup") {
		t.Fatal("display names or normalized surfaces must not be unique identity keys")
	}

	compatibility := strings.Join(entityIdentitySchemaStatements(), "\n")
	for _, table := range []string{"entity_identities", "entity_identity_surfaces", "speaker_attributions"} {
		if !strings.Contains(compatibility, "CREATE TABLE IF NOT EXISTS "+table) {
			t.Fatalf("compatibility migration missing %s", table)
		}
	}
}
