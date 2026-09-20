package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestSessionMigrationBodyTrackingOperationalIdentityOnly46(t *testing.T) {
	maps := newSessionMigrationKeyMaps()
	_ = maps.put("entity_identities", "stable_entity_id", "original-character", "branch-character")
	_ = maps.put("memory_source_revisions", "source_revision", "original-source", "branch-source")
	valueJSON := `{"contract_version":"body_tracking.v1", "subject_entity_id":"original-character","origin_entity_id":"original-character","observed_facts":{"period_start":{"character_id":"original-character","origin_entity_id":"original-character","source":{"character_id":"original-character"}},"unstructured":"original-character","malformed":{"character_id":17}},"pregnancy":{"character_id":"original-character","event_token":"keep-token","draw_hex":"ffffffffffffffff"},"recovery":{"character_id":"original-character"},"history_observation":{"character_id":"original-character"},"arbitrary":{"character_id":"original-character"},"large":9007199254740993}`
	evidenceJSON := `{"source_revision":"original-source", "source_unit_id":"keep-unit","subject_entity_id":"original-character","origin_entity_id":"original-character","history_observation":{"character_id":"original-character","origin_entity_id":"original-character"},"pregnancy":{"character_id":"original-character"},"source":{"subject_entity_id":"original-character"}}`
	var sourceEvidence map[string]any
	_ = json.Unmarshal([]byte(evidenceJSON), &sourceEvidence)
	sourceEvidence["repair_before"] = map[string]any{"owner_id": "original-character", "chat_session_id": "original-session", "value_json": valueJSON, "evidence_json": `{"source_revision":"original-source"}`}
	evidenceBytes, _ := json.Marshal(sourceEvidence)
	evidenceJSON = string(evidenceBytes)
	for _, table := range []string{"status_current_values", "status_change_events"} {
		for _, key := range []string{"body_tracking", "unrelated_status"} {
			t.Run(table+"/"+key, func(t *testing.T) {
				plan, _ := SessionMigrationExecutionPlanFor(table)
				source := sessionMigrationRow{Values: map[string]sessionMigrationCell{
					"status_key": {Valid: true, Text: key}, "owner_id": {Valid: true, Text: "original-character"},
					"evidence_json": {Valid: true, Text: evidenceJSON},
				}}
				valueColumns := []string{"value_json"}
				if table == "status_change_events" {
					valueColumns = []string{"previous_value_json", "new_value_json"}
				}
				for _, column := range valueColumns {
					source.Values[column] = sessionMigrationCell{Valid: true, Text: valueJSON}
				}
				target := sessionMigrationRow{Values: map[string]sessionMigrationCell{}}
				for column, cell := range source.Values {
					target.Values[column] = cell
				}
				if err := sessionMigrationRemapSemanticReferences(plan, source, &target, maps); err != nil {
					t.Fatal(err)
				}
				if key == "body_tracking" {
					if target.Values["owner_id"].Text != "branch-character" {
						t.Fatal("body current owner was not remapped")
					}
					var value, evidence map[string]any
					_ = json.Unmarshal([]byte(target.Values[valueColumns[0]].Text), &value)
					_ = json.Unmarshal([]byte(target.Values["evidence_json"].Text), &evidence)
					if value["subject_entity_id"] != "branch-character" || value["origin_entity_id"] != "original-character" || value["arbitrary"].(map[string]any)["character_id"] != "original-character" || value["history_observation"].(map[string]any)["character_id"] != "original-character" {
						t.Fatalf("body operational/provenance fields conflated: %#v", value)
					}
					facts := value["observed_facts"].(map[string]any)
					period := facts["period_start"].(map[string]any)
					if period["character_id"] != "branch-character" || period["source"].(map[string]any)["character_id"] != "original-character" || facts["unstructured"] != "original-character" || facts["malformed"].(map[string]any)["character_id"] != float64(17) {
						t.Fatalf("body nested identity remap escaped exact paths: %#v", facts)
					}
					if evidence["source_revision"] != "branch-source" || evidence["subject_entity_id"] != "branch-character" || evidence["source_unit_id"] != "keep-unit" || evidence["history_observation"].(map[string]any)["character_id"] != "branch-character" || evidence["pregnancy"].(map[string]any)["character_id"] != "original-character" {
						t.Fatalf("body evidence operational/origin fields conflated: %#v", evidence)
					}
					if !strings.Contains(target.Values[valueColumns[0]].Text, "9007199254740993") || value["pregnancy"].(map[string]any)["draw_hex"] != "ffffffffffffffff" {
						t.Fatal("copy rounded source numbers or resampled immutable outcome")
					}
					before := evidence["repair_before"].(map[string]any)
					var beforeValue map[string]any
					_ = json.Unmarshal([]byte(before["value_json"].(string)), &beforeValue)
					if beforeValue["subject_entity_id"] != "branch-character" || beforeValue["origin_entity_id"] != "original-character" || before["owner_id"] != "original-character" || before["chat_session_id"] != "original-session" || before["evidence_json"] != `{"source_revision":"original-source"}` {
						t.Fatalf("manual undo snapshot operational identity or audit changed: %#v", before)
					}
				} else if target.Values["owner_id"].Text != "original-character" || target.Values[valueColumns[0]].Text != valueJSON {
					t.Fatal("unrelated status identity/value was remapped")
				}
				for _, semantic := range plan.SemanticReferences {
					if got, want := sessionMigrationVerifiedSemanticReferenceCount(source, target, semantic, maps), sessionMigrationSemanticReferenceValueCount(source, semantic); got != want {
						t.Fatalf("inverse parity %s: got=%d want=%d", semantic.Column, got, want)
					}
				}
			})
		}
	}
}

func TestSessionMigrationDirectEvidenceAbsentSupersession46(t *testing.T) {
	entry, _ := sessionMigrationManifestEntryByTable("direct_evidence_records")
	plan, _ := SessionMigrationExecutionPlanFor(entry.Table)
	for _, sourceID := range []string{"0", "9"} {
		t.Run(sourceID, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectBegin()
			tx, err := db.BeginTx(context.Background(), nil)
			if err != nil {
				t.Fatal(err)
			}
			mock.ExpectExec("INSERT INTO `direct_evidence_records`").WillReturnResult(sqlmock.NewResult(20, 1))
			mock.ExpectExec("INSERT INTO session_migration_artifact_row_map").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("INSERT INTO session_migration_row_map").WillReturnResult(sqlmock.NewResult(0, 1))
			source := sessionMigrationTestRow(map[string]string{"id": "10", "chat_session_id": "origin", "superseded_by_id": sourceID})
			maps := newSessionMigrationKeyMaps()
			target, _, deferred, err := sessionMigrationInsertManifestRow(context.Background(), tx, 46, entry, plan, source, "branch", maps)
			if err != nil {
				t.Fatal(err)
			}
			if sourceID == "0" {
				if len(deferred) != 0 || target.Values["superseded_by_id"] != source.Values["superseded_by_id"] || sessionMigrationFKValueCount([]sessionMigrationRow{source}, plan) != 0 {
					t.Fatalf("absent supersession became an unresolved reference: target=%+v deferred=%+v", target, deferred)
				}
			} else if len(deferred) != 1 || deferred[0].SourceValue != sourceID || sessionMigrationFKValueCount([]sessionMigrationRow{source}, plan) != 1 {
				t.Fatalf("nonzero supersession was silently dropped: %+v", deferred)
			}
			mock.ExpectRollback()
			_ = tx.Rollback()
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSessionMigrationMissingEvidenceMappingRemainsStrict46(t *testing.T) {
	plan, _ := SessionMigrationExecutionPlanFor("memory_derivation_dependencies")
	maps := newSessionMigrationKeyMaps()
	_ = maps.put("status_change_events", "id", "8", "80")
	for _, state := range []string{"active", "unknown", "invalidated"} {
		t.Run(state, func(t *testing.T) {
			source := sessionMigrationTestRow(map[string]string{"lifecycle_state": state, "source_revision": "foreign-or-unknown-source", "child_artifact_type": "status_change_event", "child_artifact_id": "8", "parent_artifact_type": "direct_evidence", "parent_artifact_id": "99"})
			target := sessionMigrationRow{Values: map[string]sessionMigrationCell{}}
			if err := sessionMigrationRemapSemanticReferences(plan, source, &target, maps); err == nil || !strings.Contains(err.Error(), "direct_evidence_records.id") {
				t.Fatalf("missing evidence acquired an implicit mapping for state %q: %v", state, err)
			}
		})
	}
}

func TestSessionMigrationStatusSourceBindingAndDependencyRemap46(t *testing.T) {
	maps := newSessionMigrationKeyMaps()
	for _, item := range []struct{ table, column, source, target string }{
		{"memory_source_revisions", "source_revision", "original-revision", "copied-revision"},
		{"status_change_events", "id", "10", "20"},
		{"status_change_events", "id", "9", "19"},
	} {
		if err := maps.put(item.table, item.column, item.source, item.target); err != nil {
			t.Fatal(err)
		}
	}
	for _, table := range []string{"status_current_values", "status_change_events"} {
		plan, _ := SessionMigrationExecutionPlanFor(table)
		raw := `{"source_revision":"original-revision","source_fields":[{"source_revision":"historical-origin"}],"occurrence_time":{"day":"market"}}`
		source := sessionMigrationRow{Values: map[string]sessionMigrationCell{"evidence_json": {Valid: true, Text: raw}}}
		target := sessionMigrationRow{Values: map[string]sessionMigrationCell{}}
		if err := sessionMigrationRemapSemanticReferences(plan, source, &target, maps); err != nil {
			t.Fatal(err)
		}
		var mapped map[string]any
		if err := json.Unmarshal([]byte(target.Values["evidence_json"].Text), &mapped); err != nil {
			t.Fatal(err)
		}
		if mapped["source_revision"] != "copied-revision" || mapped["source_fields"].([]any)[0].(map[string]any)["source_revision"] != "historical-origin" {
			t.Fatalf("source binding/origin confusion: %+v", mapped)
		}
		if count := sessionMigrationVerifiedSemanticReferenceCount(source, target, plan.SemanticReferences[0], maps); count != 1 {
			t.Fatalf("source binding parity count=%d", count)
		}
		for _, legacy := range []string{`{}`, `{"source_revision":"external-origin"}`} {
			source.Values["evidence_json"] = sessionMigrationCell{Valid: true, Text: legacy}
			if err := sessionMigrationRemapSemanticReferences(plan, source, &target, maps); err != nil || target.Values["evidence_json"].Text != legacy {
				t.Fatalf("optional original binding was rejected or altered: %+v err=%v", target, err)
			}
		}
	}
	plan, _ := SessionMigrationExecutionPlanFor("memory_derivation_dependencies")
	source := sessionMigrationRow{Values: map[string]sessionMigrationCell{
		"root_source_pointer": {Valid: true, Text: "source_revision:original-revision"},
		"child_artifact_type": {Valid: true, Text: "status_change_event"}, "child_artifact_id": {Valid: true, Text: "10"},
		"parent_artifact_type": {Valid: true, Text: "status_change_event"}, "parent_artifact_id": {Valid: true, Text: "9"},
	}}
	target := sessionMigrationRow{Values: map[string]sessionMigrationCell{}}
	if err := sessionMigrationRemapSemanticReferences(plan, source, &target, maps); err != nil {
		t.Fatal(err)
	}
	if target.Values["child_artifact_id"].Text != "20" || target.Values["parent_artifact_id"].Text != "19" {
		t.Fatalf("status dependency IDs not remapped: %+v", target)
	}
}

func TestCleanupSessionMigrationSourceRequiresEnabledMariaDBStore(t *testing.T) {
	m := &mariadbStore{}
	result, err := m.CleanupSessionMigrationSource(context.Background(), 7, "must not delete")
	if result != nil {
		t.Fatalf("cleanup result = %+v, want nil", result)
	}
	if !errors.Is(err, ErrNotEnabled) {
		t.Fatalf("cleanup error = %v, want %v", err, ErrNotEnabled)
	}
}

func TestSessionMigrationBodyParentageReferences46(t *testing.T) {
	maps := newSessionMigrationKeyMaps()
	_ = maps.put("entity_identities", "stable_entity_id", "person-a", "branch-a")
	person := map[string]any{"entity_id": "person-a", "character_name": "Arin"}
	parentage := map[string]any{"status": "modeled_link", "candidates": []any{person}, "basis_events": []any{map[string]any{"semantic_event_key": "keep-event"}}}
	fact := map[string]any{"partners": []any{person}, "paternity": parentage}
	value := map[string]any{"observed_facts": map[string]any{"conception_exposure": fact}, "pregnancy": fact, "modeled_pregnancy": map[string]any{"paternity": parentage}, "latest_model_result": map[string]any{"selected_pregnancy": map[string]any{"paternity": parentage}}, "origin_entity_id": "person-a", "arbitrary": fact}
	evidence := map[string]any{"history_observation": fact, "model_result": map[string]any{"selected_pregnancy": map[string]any{"paternity": parentage}}, "repair_before": map[string]any{"value_json": mustMigrationJSON46(t, value)}}
	for _, tc := range []struct {
		value    map[string]any
		evidence bool
	}{{value, false}, {evidence, true}} {
		original := mustMigrationJSON46(t, tc.value)
		mapped, count := sessionMigrationRemapBodyTrackingJSON(original, maps, false, tc.evidence)
		if count == 0 || !strings.Contains(mapped, "branch-a") {
			t.Fatal("parentage identity not mapped")
		}
		reversed, inverseCount := sessionMigrationRemapBodyTrackingJSON(mapped, maps, true, tc.evidence)
		canonical, _ := sessionMigrationRemapBodyTrackingJSON(original, nil, false, tc.evidence)
		if reversed != canonical || count != inverseCount {
			t.Fatalf("parentage mapping parity failed: %s / %s", reversed, canonical)
		}
		if !tc.evidence {
			var v map[string]any
			_ = json.Unmarshal([]byte(mapped), &v)
			p := v["pregnancy"].(map[string]any)["paternity"].(map[string]any)
			if p["candidates"].([]any)[0].(map[string]any)["entity_id"] != "branch-a" || v["origin_entity_id"] != "person-a" {
				t.Fatal("operational and origin identities conflated")
			}
			if v["arbitrary"].(map[string]any)["partners"].([]any)[0].(map[string]any)["entity_id"] != "person-a" {
				t.Fatal("unrelated JSON was changed")
			}
		}
	}
}

func mustMigrationJSON46(t *testing.T, value any) string {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
