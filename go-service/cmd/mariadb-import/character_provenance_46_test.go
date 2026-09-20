package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func Test46CharacterProvenanceImportPreservesStructuredMetadata(t *testing.T) {
	metadata := map[string]any{"version": "character_field_provenance.v1", "fields": map[string]any{"/status/possession": map[string]any{"source_turn": 3, "source_session_id": "origin"}}}
	query, args, err := buildInsert("character_states", map[string]any{"character_name": "Mina", "field_provenance_json": metadata})
	if err != nil || !strings.Contains(query, "`field_provenance_json`") || len(args) != 2 {
		t.Fatalf("provenance dropped: %s %#v %v", query, args, err)
	}
	var restored map[string]any
	if err := json.Unmarshal([]byte(args[1].(string)), &restored); err != nil {
		t.Fatal(err)
	}
	if restored["fields"].(map[string]any)["/status/possession"].(map[string]any)["source_turn"] != float64(3) {
		t.Fatalf("source origin changed: %#v", restored)
	}
	_, legacy, err := buildInsert("character_states", map[string]any{"character_name": "Mina"})
	if err != nil || len(legacy) != 1 {
		t.Fatalf("legacy missing metadata became required: %#v %v", legacy, err)
	}
}
