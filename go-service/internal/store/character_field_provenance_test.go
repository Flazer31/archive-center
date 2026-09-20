package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCharacterFieldProvenancePreservesUnchangedLeaves46(t *testing.T) {
	previous := CharacterState{ChatSessionID: "origin", TurnIndex: 2, AppearanceJSON: `{"hair/color~":"brown"}`, StatusJSON: `{"health":"well","wallet":{"silver":12},"tools":["rope","lamp"]}`}
	previous.FieldProvenanceJSON = MergeCharacterStateFieldProvenance(nil, previous)
	next := previous
	next.ChatSessionID, next.TurnIndex = "branch", 8
	next.StatusJSON = `{"health":"well","wallet":{"silver":11},"tools":["lamp","rope"]}`
	next.FieldProvenanceJSON = `{"fields":{"/status/wallet/silver":{"evidence_excerpt":"Paid one coin","claim_scope":"objective","effective_time":{"day":"market day"}},"/status/health":{"claim_scope":"belief","perspective_owner":"Mina","learned_time":{"scene":"clinic"}}}}`
	metadata := DecodeCharacterFieldProvenance(MergeCharacterStateFieldProvenance(&previous, next))
	for _, path := range []string{"/appearance/hair~1color~0", "/status/health"} {
		if metadata[path]["source_turn"] != float64(previous.TurnIndex) || metadata[path]["source_session_id"] != previous.ChatSessionID {
			t.Fatalf("unchanged field origin moved: %s %+v", path, metadata[path])
		}
	}
	for _, path := range []string{"/status/wallet/silver", "/status/tools"} {
		if metadata[path]["source_turn"] != float64(next.TurnIndex) || metadata[path]["source_session_id"] != next.ChatSessionID {
			t.Fatalf("changed field origin = %s %+v", path, metadata[path])
		}
	}
	if _, exists := metadata["/status/tools/0"]; exists {
		t.Fatal("array index received a persistent identity")
	}
	if !reflect.DeepEqual(CharacterFieldProvenanceForPath(metadata, "/status/tools/0"), metadata["/status/tools"]) {
		t.Fatal("array member did not resolve whole-array provenance")
	}
	if metadata["/status/health"]["claim_scope"] != "belief" || metadata["/status/health"]["perspective_owner"] != "Mina" {
		t.Fatal("explicit perspective was lost")
	}
	if metadata["/status/wallet/silver"]["effective_time"] == nil || metadata["/status/wallet/silver"]["occurrence_time"] != nil {
		t.Fatal("explicit time lost or occurrence invented")
	}
}

func TestCharacterFieldProvenanceLegacyUnknownStaysUnknown46(t *testing.T) {
	previous := CharacterState{ChatSessionID: "legacy", TurnIndex: 50, AppearanceJSON: `{"hair":"brown"}`, StatusJSON: `{"coins":4}`}
	next := previous
	next.TurnIndex, next.StatusJSON = 60, `{"coins":3}`
	metadata := DecodeCharacterFieldProvenance(MergeCharacterStateFieldProvenance(&previous, next))
	if len(metadata["/appearance/hair"]) != 0 {
		t.Fatalf("legacy origin invented: %+v", metadata["/appearance/hair"])
	}
	if metadata["/status/coins"]["source_turn"] != float64(next.TurnIndex) {
		t.Fatalf("new observation missing: %+v", metadata)
	}
	if metadata["/status/coins"]["claim_scope"] != nil || metadata["/status/coins"]["occurrence_time"] != nil {
		t.Fatal("scope or event time inferred from snapshot")
	}
}

func TestCharacterFieldProvenanceHistoryUsesContinuousFinalSnapshots46(t *testing.T) {
	history := []CharacterState{
		{ID: 5, ChatSessionID: "legacy", TurnIndex: 5, AppearanceJSON: `{"hair":"brown","ring":"gold"}`},
		{ID: 4, ChatSessionID: "legacy", TurnIndex: 3, AppearanceJSON: `{"hair":"brown"}`},
		{ID: 3, ChatSessionID: "legacy", TurnIndex: 2, AppearanceJSON: `{"hair":"black","ring":"gold"}`},
		{ID: 2, ChatSessionID: "legacy", TurnIndex: 2, AppearanceJSON: `{"hair":"brown","ring":"gold"}`},
		{ID: 1, ChatSessionID: "legacy", TurnIndex: 1, AppearanceJSON: `{"hair":"brown","ring":"gold"}`},
	}
	before, _ := json.Marshal(history)
	metadata := DecodeCharacterFieldProvenance(BuildCharacterFieldProvenanceFromHistory(history))
	if metadata["/appearance/hair"]["source_turn"] != float64(history[1].TurnIndex) {
		t.Fatalf("discarded same-turn value extended continuity: %+v", metadata)
	}
	if metadata["/appearance/ring"]["source_turn"] != float64(history[0].TurnIndex) {
		t.Fatalf("missing prior field failed to break continuity: %+v", metadata)
	}
	for _, value := range metadata {
		if value["provenance_basis"] != "earliest_continuous_snapshot" || value["occurrence_time"] != nil {
			t.Fatalf("repair invented actual date: %+v", value)
		}
	}
	after, _ := json.Marshal(history)
	if string(before) != string(after) {
		t.Fatal("repair helper mutated historical snapshots")
	}
}

func TestCharacterFieldProvenanceHistoryPreservesKnownOriginalSource46(t *testing.T) {
	history := []CharacterState{
		{ID: 2, ChatSessionID: "branch", TurnIndex: 10, StatusJSON: `{"knowledge":"door code"}`},
		{ID: 1, ChatSessionID: "branch", TurnIndex: 4, StatusJSON: `{"knowledge":"door code"}`, FieldProvenanceJSON: `{"fields":{"/status/knowledge":{"source_turn":2,"recorded_turn":3,"source_session_id":"origin","source_revision":"original-revision","claim_scope":"belief","perspective_owner":"Mina","occurrence_time":{"day":"unknown"}}}}`},
	}
	metadata := DecodeCharacterFieldProvenance(BuildCharacterFieldProvenanceFromHistory(history))["/status/knowledge"]
	if metadata["source_turn"] != float64(2) || metadata["recorded_turn"] != float64(3) || metadata["source_session_id"] != "origin" || metadata["source_revision"] != "original-revision" || metadata["claim_scope"] != "belief" {
		t.Fatalf("known origin replaced by copied snapshot: %+v", metadata)
	}
}

func TestCharacterProvenanceRepairAuditFailureRollsBackSnapshot46(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	before := CharacterState{ChatSessionID: "session-1", CharacterName: "Mina", TurnIndex: 4, StatusJSON: `{"coins":4}`}
	after := before
	after.FieldProvenanceJSON = `{"fields":{"/status/coins":{"source_turn":2}}}`
	failure := errors.New("audit write failed")
	mock.ExpectBegin()
	mock.ExpectQuery("FROM character_events").WithArgs(before.ChatSessionID, before.CharacterName, "repair-coins").WillReturnError(sql.ErrNoRows)
	mock.ExpectExec("INSERT INTO character_states").WithArgs(after.ChatSessionID, after.CharacterName, nil, nil, after.StatusJSON, nil, nil, after.FieldProvenanceJSON, after.TurnIndex, sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(10, 1))
	mock.ExpectExec("INSERT INTO character_events").WillReturnError(failure)
	mock.ExpectRollback()
	if _, err := (&mariadbStore{db: db}).ApplyCharacterProvenanceRepair(context.Background(), before, after, "repair-coins"); !errors.Is(err, failure) {
		t.Fatalf("repair error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCharacterProvenanceRepairReplayDoesNotAppend46(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	state := CharacterState{ChatSessionID: "session-1", CharacterName: "Mina", TurnIndex: 4}
	mock.ExpectBegin()
	mock.ExpectQuery("FROM character_events").WithArgs(state.ChatSessionID, state.CharacterName, "repair-coins").WillReturnRows(sqlmock.NewRows([]string{"id", "chat_session_id", "character_name", "turn_index", "event_type", "details_json", "created_at"}).AddRow(10, state.ChatSessionID, state.CharacterName, state.TurnIndex, "field_provenance_repair", `{"operation_id":"repair-coins"}`, reversibleStatusTestTime()))
	mock.ExpectCommit()
	event, err := (&mariadbStore{db: db}).ApplyCharacterProvenanceRepair(context.Background(), state, state, "repair-coins")
	if err != nil || event.ID != 10 {
		t.Fatalf("replay event=%+v err=%v", event, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCharacterHistoryWrappersPreservePaginationAndMetadata46(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := &mariadbStore{db: db}
	metadata := `{"fields":{"/status/coins":{"source_turn":2}}}`
	for _, wrapper := range []Store{NewReadOnlyStore(m), NewDualWriteStore(m, nil)} {
		mock.ExpectQuery("FROM character_states").WithArgs("session-1", "Mina", 50, 100).WillReturnRows(sqlmock.NewRows([]string{"id", "chat_session_id", "character_name", "appearance_json", "personality_json", "status_json", "relationships_json", "speech_style_json", "field_provenance_json", "turn_index", "created_at", "updated_at"}).AddRow(10, "session-1", "Mina", nil, nil, `{"coins":4}`, nil, nil, metadata, 4, reversibleStatusTestTime(), reversibleStatusTestTime()))
		rows, err := wrapper.(CharacterStateHistoryStore).ListCharacterStateHistory(context.Background(), "session-1", "Mina", 50, 100)
		if err != nil || len(rows) != 1 || rows[0].FieldProvenanceJSON != metadata {
			t.Fatalf("wrapper lost history: %+v err=%v", rows, err)
		}
	}
	if _, writable := NewReadOnlyStore(m).(CharacterProvenanceRepairStore); writable {
		t.Fatal("read-only wrapper exposes repair writer")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
