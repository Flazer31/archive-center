package store

import (
	"encoding/json"
	"strings"
	"testing"
)

func stitchTestRow(values map[string]string) sessionMigrationRow {
	r := sessionMigrationRow{Values: map[string]sessionMigrationCell{}}
	for k, v := range values {
		r.Values[k] = sessionMigrationCell{Valid: true, Text: v}
	}
	return r
}

func TestSessionStitchSnapshotCoordinatesAndReferences(t *testing.T) {
	ids := []string{"old", "new"}
	sources := []map[string][]sessionMigrationRow{
		{"chat_logs": {stitchTestRow(map[string]string{"id": "1", "turn_index": "3", "role": "assistant", "content": "Old scene"})},
			"entity_identities":      {stitchTestRow(map[string]string{"stable_entity_id": "old-courier", "entity_kind": "character", "canonical_label": "Courier", "identity_namespace": "story"})},
			"status_schema_registry": {stitchTestRow(map[string]string{"id": "1", "schema_name": "narrative", "status_key": "state", "owner_scope": "character"})},
			"status_current_values":  {stitchTestRow(map[string]string{"id": "1", "registry_id": "1", "owner_scope": "character", "owner_id": "Courier", "value_json": `{"state":"injured"}`})},
			"status_change_events":   {stitchTestRow(map[string]string{"id": "1", "registry_id": "1", "status_value_id": "1", "source_turn": "3"})},
			"precise_memory_units":   {stitchTestRow(map[string]string{"id": "1", "subject_entity_id": "old-courier", "source_turn_start": "3", "source_turn_end": "3", "payload_json": `{"summary":"Old injury","source_turn":3}`})}},
		{"chat_logs": {stitchTestRow(map[string]string{"id": "2", "turn_index": "0", "role": "assistant", "content": "New greeting"}), stitchTestRow(map[string]string{"id": "3", "turn_index": "1", "role": "assistant", "content": "Recovered"})},
			"memory_source_revisions": {stitchTestRow(map[string]string{"id": "2", "turn_index": "1", "critic_input_snapshot_json": "{\n  \"turn_index\": 1\n}", "critic_input_snapshot_hash": "original-hash"})},
			"entity_identities":       {stitchTestRow(map[string]string{"stable_entity_id": "new-courier", "entity_kind": "character", "canonical_label": "Courier", "identity_namespace": "story"})},
			"status_schema_registry":  {stitchTestRow(map[string]string{"id": "2", "schema_name": "narrative", "status_key": "state", "owner_scope": "character"})},
			"status_current_values":   {stitchTestRow(map[string]string{"id": "2", "registry_id": "2", "owner_scope": "character", "owner_id": "Courier", "value_json": `{"state":"recovered","source_turn":1,"age":20,"duration_days":7}`})},
			"character_events":        {stitchTestRow(map[string]string{"id": "7", "turn_index": "0", "event_type": "manual_patch", "details_json": `{"speech_style_json":{"notes":"Soft voice"}}`})}},
	}
	rows, segments, err := sessionStitchSnapshot(ids, sources, "new", nil)
	if err != nil {
		t.Fatal(err)
	}
	if segments[1].Offset != 4 {
		t.Fatalf("greeting overwrites previous assistant: %+v", segments)
	}
	logs := rows["chat_logs"]
	if logs[0].Values["turn_index"].Text != "3" || logs[1].Values["turn_index"].Text != "4" || logs[2].Values["turn_index"].Text != "5" {
		t.Fatalf("bad turns: %+v", logs)
	}
	if len(rows["status_schema_registry"]) != 1 || len(rows["status_current_values"]) != 1 {
		t.Fatal("duplicated current slots")
	}
	if rows["status_change_events"][0].Values["registry_id"].Text != "2" || rows["status_change_events"][0].Values["status_value_id"].Text != "2" {
		t.Fatal("history FK did not follow current slot")
	}
	if len(rows["entity_identities"]) != 2 || rows["precise_memory_units"][0].Values["subject_entity_id"].Text != "old-courier" {
		t.Fatal("distinct source identities merged by display name")
	}
	var value map[string]any
	if err := json.Unmarshal([]byte(rows["status_current_values"][0].Values["value_json"].Text), &value); err != nil {
		t.Fatal(err)
	}
	if value["source_turn"] != float64(5) || value["age"] != float64(20) || value["duration_days"] != float64(7) {
		t.Fatalf("not a turn coordinate: %+v", value)
	}
	if rows["character_events"][0].Values["turn_index"].Text != "0" || !strings.Contains(rows["character_events"][0].Values["details_json"].Text, "Soft voice") {
		t.Fatal("manual settings lost")
	}
	if rows["memory_source_revisions"][0].Values["critic_input_snapshot_json"].Text != "{\n  \"turn_index\": 1\n}" || rows["memory_source_revisions"][0].Values["critic_input_snapshot_hash"].Text != "original-hash" {
		t.Fatal("hashed original critic input was rewritten")
	}
}

func TestSessionStitchDoesNotChangeDialogueDatesOrDurations(t *testing.T) {
	raw := `{"summary":"At turn 4 she stayed seven days, year 1400.","turn_index":4,"duration":7,"date":"1400-01-01","source_turn":0,"nested":[{"source_turn_start":2,"source_turn_end":3}]}`
	var got map[string]any
	json.Unmarshal([]byte(sessionStitchJSON(raw, 10, nil)), &got)
	if got["turn_index"] != float64(14) || got["source_turn"] != float64(0) || got["duration"] != float64(7) || got["date"] != "1400-01-01" || got["summary"] != "At turn 4 she stayed seven days, year 1400." {
		t.Fatalf("rewrote narrative: %+v", got)
	}
	item := got["nested"].([]any)[0].(map[string]any)
	if item["source_turn_start"] != float64(12) || item["source_turn_end"] != float64(13) {
		t.Fatal("nested coordinates not moved")
	}
}

func TestSessionStitchCarriesOnlyInputGroupMembership(t *testing.T) {
	rows := []sessionMigrationRow{
		stitchTestRow(map[string]string{"event_type": "source_acceptance_transition", "details_json": `{"logical_turn_id":"group-B","user_logical_turn_ids":["group-A","group-B"],"revision":"old-request"}`}),
		stitchTestRow(map[string]string{"event_type": "unrelated", "details_json": `{"logical_turn_id":"wrong","user_logical_turn_ids":["wrong"]}`}),
	}
	got := sessionStitchInputGroupAliases(rows, map[string][]string{"earlier-group": {"earlier-member"}, "group-B": {"group-A"}})
	if len(got) != 2 || len(got["group-B"]) != 2 || got["group-B"][0] != "group-A" || got["group-B"][1] != "group-B" || len(got["earlier-group"]) != 1 {
		t.Fatalf("membership lost or unrelated lifecycle imported: %+v", got)
	}
}
