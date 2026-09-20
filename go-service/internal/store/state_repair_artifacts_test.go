package store

import (
	"context"
	"database/sql/driver"
	"time"

	"encoding/json"
	"github.com/DATA-DOG/go-sqlmock"
	"strings"
	"testing"
)

func TestBodyRepairPrunesOnlySelectedCharacterState(t *testing.T) {
	raw := `{"Mina":{"pregnancy":"confirmed","job":"maid","appearance":{"gender":"female"}},"Vera":{"pregnancy":"confirmed"},"weather":"clear"}`
	got := bodyRepairPruneJSON(raw, "mina-id", "Mina", false)
	var value map[string]any
	if err := json.Unmarshal([]byte(got), &value); err != nil {
		t.Fatal(err)
	}
	mina := value["Mina"].(map[string]any)
	vera := value["Vera"].(map[string]any)
	if mina["pregnancy"] != nil || mina["job"] != "maid" || vera["pregnancy"] != "confirmed" || value["weather"] != "clear" {
		t.Fatalf("unrelated state damaged: %s", got)
	}
	if !strings.Contains(got, "female") {
		t.Fatal("body cleanup erased character gender")
	}
}

func TestBodyRepairUnrelatedStatePreservesExactBytes(t *testing.T) {
	raw := `{ "subject_label": "Mina", "job": "maid", "age": 900 }`
	if got := bodyRepairPruneJSON(raw, "mina-id", "Mina", true); got != raw {
		t.Fatalf("unrelated fields rewritten: %s", got)
	}
}

func Test46BodyDeletionPreservesExplicitNestedOtherSubject(t *testing.T) {
	raw := `{"subject_label":"Mina","status":{"pregnancy":"confirmed","job":"archivist"},"nearby":{"subject_label":"Vera","status":{"pregnancy":"confirmed","job":"medic"}}}`
	got := bodyRepairPruneJSON(raw, "mina-id", "Mina", false)
	var result map[string]any
	if err := json.Unmarshal([]byte(got), &result); err != nil {
		t.Fatal(err)
	}
	if result["status"].(map[string]any)["pregnancy"] != nil {
		t.Fatal("selected subject was not cleaned")
	}
	other := result["nearby"].(map[string]any)["status"].(map[string]any)
	if other["pregnancy"] != "confirmed" {
		t.Fatalf("another explicit subject was altered: %s", got)
	}
}

type repairJSONWriteCapture46 struct{ value *string }

func (c repairJSONWriteCapture46) Match(v driver.Value) bool {
	if s, ok := v.(string); ok {
		*c.value = s
		return true
	}
	if b, ok := v.([]byte); ok {
		*c.value = string(b)
		return true
	}
	return false
}

func Test46BodyRestoreKeepsLaterUnrelatedStatusField(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	beforeDelete := `{"pregnancy":"confirmed","job":"archivist"}`
	afterDelete := `{"job":"archivist"}`
	currentLater := `{"job":"captain"}`
	// This is the exact field swap emitted by planAdminStateRepair for undo.
	change := StateRepairArtifactChange{Table: "character_states", ID: 7, Before: map[string]*string{"status_json": &afterDelete}, After: map[string]*string{"status_json": &beforeDelete}}
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("SELECT appearance_json,field_provenance_json,status_json FROM character_states WHERE chat_session_id = ? AND id = ? FOR UPDATE").WithArgs("audit", int64(7)).WillReturnRows(sqlmock.NewRows([]string{"appearance_json", "field_provenance_json", "status_json"}).AddRow(`{"gender":"female"}`, `{}`, currentLater))
	var written string
	mock.ExpectExec("UPDATE character_states SET status_json = ? WHERE chat_session_id = ? AND id = ?").WithArgs(repairJSONWriteCapture46{&written}, "audit", int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectRollback()
	_, err = applyStateRepairArtifactsTx(context.Background(), tx, "audit", "restore", []StateRepairArtifactChange{change}, time.Unix(1, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
	var state map[string]any
	if err = json.Unmarshal([]byte(written), &state); err != nil {
		t.Fatal(err)
	}
	if state["job"] != "captain" {
		t.Fatalf("body restore overwrote later unrelated job: current=%s written=%s", currentLater, written)
	}
	if state["pregnancy"] != "confirmed" {
		t.Fatal("body fact did not restore")
	}
}
func Test46BodyRestoreKeepsLaterFieldsInsideArray(t *testing.T) {
	before := `{"people":[{"name":"Mina","job":"maid"},{"name":"Vera","job":"medic"}]}`
	after := `{"people":[{"name":"Mina","job":"maid","pregnancy":"confirmed"},{"name":"Vera","job":"medic"}]}`
	current := `{"people":[{"name":"Vera","job":"doctor"},{"name":"Mina","job":"captain"},{"name":"Lena","job":"guard"}]}`
	restored := bodyRepairApplyJSONDelta(current, before, after)
	var value map[string]any
	if err := json.Unmarshal([]byte(restored), &value); err != nil {
		t.Fatal(err)
	}
	people := value["people"].([]any)
	if len(people) != 3 {
		t.Fatalf("later array member lost: %s", restored)
	}
	if people[0].(map[string]any)["job"] != "doctor" || people[1].(map[string]any)["job"] != "captain" || people[1].(map[string]any)["pregnancy"] != "confirmed" {
		t.Fatalf("restore rewound array field or attached body data to another character: %s", restored)
	}
}

func Test46BodyRestoreRebasesRemovedContainer(t *testing.T) {
	before := `{"job":"maid"}`
	after := `{"job":"maid","notes":["pregnancy confirmed"]}`
	current := `{"job":"captain","notes":["promoted"]}`
	restored := bodyRepairApplyJSONDelta(current, before, after)
	var value map[string]any
	if err := json.Unmarshal([]byte(restored), &value); err != nil {
		t.Fatal(err)
	}
	notes := value["notes"].([]any)
	if len(notes) != 2 || notes[0] != "promoted" || notes[1] != "pregnancy confirmed" {
		t.Fatalf("later notes erased by restored container: %s", restored)
	}
	undone := bodyRepairApplyJSONDelta(restored, after, before)
	var undoneValue map[string]any
	if err := json.Unmarshal([]byte(undone), &undoneValue); err != nil {
		t.Fatal(err)
	}
	if undoneValue["notes"] == nil || len(undoneValue["notes"].([]any)) != 1 || undoneValue["notes"].([]any)[0] != "promoted" {
		t.Fatalf("recorded removal erased unrelated current notes: %s", undone)
	}
}
