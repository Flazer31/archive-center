package store

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"strings"
	"testing"
)

func expectNoCharacterManualEdits(mock sqlmock.Sqlmock, sid, name string) {
	mock.ExpectQuery("SELECT e.character_name, e.details_json FROM character_events e").WithArgs(sid, name, name).
		WillReturnRows(sqlmock.NewRows([]string{"character_name", "details_json"}))
}

func TestCharacterManualEditsKeepOnlyEditedLeavesAndClearValues(t *testing.T) {
	before := CharacterState{AppearanceJSON: `{"hair":"brown","coat":"red"}`, SpeechStyleJSON: `{"manual_overrides":{"default_tone":"dry","speech_notes":"old"}}`}
	after := before
	after.AppearanceJSON = `{"hair":"black","coat":"red"}`
	after.SpeechStyleJSON = `{"manual_overrides":{"default_tone":"warm","optional":null}}`
	edits := CharacterManualPatch(before, after)
	auto := CharacterState{AppearanceJSON: `{"hair":"white","coat":"blue","hat":"new"}`, SpeechStyleJSON: before.SpeechStyleJSON}
	applyCharacterManualEdits(&auto, edits)
	if auto.AppearanceJSON != `{"coat":"blue","hair":"black","hat":"new"}` || strings.Contains(auto.SpeechStyleJSON, "old") || !strings.Contains(auto.SpeechStyleJSON, `"optional":null`) {
		t.Fatalf("manual merge clobbered auto fields or failed clear: %+v", auto)
	}
	second := auto
	second.SpeechStyleJSON = `{"manual_overrides":{}}`
	edits = mergeCharacterManualEdits(edits, CharacterManualPatch(auto, second))
	applyCharacterManualEdits(&auto, edits)
	if strings.Contains(auto.SpeechStyleJSON, "warm") {
		t.Fatal("cleared tone restored")
	}
	applyCharacterManualEdits(&auto, edits)
	if strings.Contains(auto.SpeechStyleJSON, "warm") {
		t.Fatal("repeated read restored cleared tone")
	}
	// A child removal must not mutate a prior whole-object edit in memory.
	parent := []CharacterManualFieldEdit{{Path: []string{"speech_style", "manual_overrides"}, Value: map[string]any{"speech_notes": "retained"}}}
	child := mergeCharacterManualEdits(parent, []CharacterManualFieldEdit{{Path: []string{"speech_style", "manual_overrides", "speech_notes"}, Remove: true}})
	applyCharacterManualEdits(&auto, child)
	if parent[0].Value.(map[string]any)["speech_notes"] != "retained" {
		t.Fatal("read mutated durable operation")
	}
}

func TestCharacterManualSaveRollsBackSettingWhenSnapshotWriteFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	mock.ExpectQuery("FROM character_states").WithArgs("manual-atomic", "Mira").WillReturnRows(sqlmock.NewRows([]string{"id", "chat_session_id", "character_name", "appearance_json", "personality_json", "status_json", "relationships_json", "speech_style_json", "field_provenance_json", "turn_index", "created_at", "updated_at"}))
	expectNoCharacterManualEdits(mock, "manual-atomic", "Mira")
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO character_events").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("INSERT INTO character_states").WillReturnError(errors.New("synthetic snapshot failure"))
	mock.ExpectRollback()
	next := CharacterState{ChatSessionID: "manual-atomic", CharacterName: "Mira", TurnIndex: 1, SpeechStyleJSON: `{"speech_notes":"manual"}`}
	next.ManualPatch = CharacterManualPatch(CharacterState{}, next)
	if err := m.SaveCharacterState(context.Background(), &next); err == nil {
		t.Fatal("save falsely succeeded")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCharacterManualPrincipleEditDoesNotFreezeOtherPrinciples(t *testing.T) {
	before := CharacterState{SpeechStyleJSON: `{"principles":[{"principle_key":"brief","contexts":["home"]},{"principle_key":"formal"}]}`}
	after := CharacterState{SpeechStyleJSON: `{"principles":[{"principle_key":"brief-but-kind","contexts":["home"]},{"principle_key":"formal"}]}`}
	edits := CharacterManualPatch(before, after)
	auto := CharacterState{SpeechStyleJSON: `{"principles":[{"principle_key":"new-automatic"},{"principle_key":"formal","contexts":["new-context"]},{"principle_key":"brief"}]}`}
	applyCharacterManualEdits(&auto, edits)
	applyCharacterManualEdits(&auto, edits)
	if !strings.Contains(auto.SpeechStyleJSON, "new-automatic") || !strings.Contains(auto.SpeechStyleJSON, "new-context") || strings.Count(auto.SpeechStyleJSON, "brief-but-kind") != 1 {
		t.Fatalf("manual entry clobbered/repeated automatic entries: %s", auto.SpeechStyleJSON)
	}
	manual := CharacterManualVoicePrinciples(auto)
	if len(manual) != 1 || manual[0]["principle_key"] != "brief-but-kind" {
		t.Fatalf("manual guidance includes other principles: %#v", manual)
	}
	raw, _ := json.Marshal(auto)
	var roundTrip CharacterState
	_ = json.Unmarshal(raw, &roundTrip)
	if got := CharacterManualVoicePrinciples(roundTrip); len(got) != 1 || got[0]["principle_key"] != "brief-but-kind" {
		t.Fatalf("manual delivery lost after DTO round trip: %#v", got)
	}
	created := CharacterState{SpeechStyleJSON: `{"principles":[{"principle_key":"new-user-rule"}]}`}
	newEdits := CharacterManualPatch(CharacterState{SpeechStyleJSON: `{}`}, created)
	applyCharacterManualEdits(&created, newEdits)
	if got := CharacterManualVoicePrinciples(created); len(got) != 1 || got[0]["principle_key"] != "new-user-rule" {
		t.Fatalf("first manual principle was not deliverable: %#v", got)
	}
}

func TestCharacterManualBodyDeletionCannotRetargetPatchOrRestoreRemovedState(t *testing.T) {
	raw := `{"source":"manual_patch","edits":[{"path":["status","pregnancy"],"value":"confirmed"},{"path":["appearance","hair"],"value":"black"}]}`
	clean := bodyRepairPruneJSON(raw, "entity-mira", "Mira", true)
	var record struct {
		Edits []CharacterManualFieldEdit `json:"edits"`
	}
	if err := json.Unmarshal([]byte(clean), &record); err != nil {
		t.Fatal(err)
	}
	state := CharacterState{StatusJSON: `{"mood":"calm"}`, AppearanceJSON: `{"hair":"white"}`}
	applyCharacterManualEdits(&state, record.Edits)
	if state.StatusJSON != `{"mood":"calm"}` || state.AppearanceJSON != `{"hair":"black"}` {
		t.Fatalf("body edit leaked or retargeted parent: %+v clean=%s", state, clean)
	}
}
