package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	archiveStore "github.com/risulongmemory/archive-center-go/internal/store"
)

type characterStateWriter46 interface {
	SaveCharacterState(context.Context, *archiveStore.CharacterState) error
}

func characterFieldMetadata46(t *testing.T, state *archiveStore.CharacterState, path string) map[string]any {
	t.Helper()
	if state == nil {
		t.Fatal("missing character state")
	}
	return archiveStore.CharacterFieldProvenanceForPath(archiveStore.DecodeCharacterFieldProvenance(state.FieldProvenanceJSON), path)
}

func TestCharacterProvenance46MariaDBMixedSnapshotsCopyImportAndRollback(t *testing.T) {
	db, st := feedback43Database(t)
	ctx := context.Background()
	writer := st.(characterStateWriter46)
	const sid = "field-origin"
	firstRevision := state46Source(t, db, sid, 1)
	definition := state46Registry(t, st, sid, "entity")
	firstTransition := state46Transition(t, definition, firstRevision, "Mina-state", 1, "active", nil)
	firstTransition.CurrentValue.ValueJSON = `{"value":"five coins","source_fields":[{"path":"/status/coins","source_turn":1}],"occurrence_time":{"day":"arrival"}}`
	firstTransition.Event.NewValueJSON = firstTransition.CurrentValue.ValueJSON
	state46Apply(t, st, firstTransition)
	first := archiveStore.CharacterState{ChatSessionID: sid, CharacterName: "Mina", AppearanceJSON: `{"hair":"brown"}`, StatusJSON: `{"coins":5,"mood":"calm"}`, TurnIndex: 1}
	metadata, _ := json.Marshal(map[string]any{"fields": map[string]any{"/appearance/hair": map[string]any{"source_revision": firstRevision, "evidence_excerpt": "brown hair", "effective_time": map[string]any{"day": "arrival"}}}})
	first.FieldProvenanceJSON = string(metadata)
	if err := writer.SaveCharacterState(ctx, &first); err != nil {
		t.Fatal(err)
	}
	secondRevision := state46Source(t, db, sid, 3)
	secondTransition := state46Transition(t, definition, secondRevision, "Mina-state", 3, "active", nil)
	secondTransition.CurrentValue.ValueJSON = `{"value":"four coins","source_fields":[{"path":"/status/coins","source_turn":3}],"occurrence_time":{"day":"market"}}`
	secondTransition.Event.NewValueJSON = secondTransition.CurrentValue.ValueJSON
	state46Apply(t, st, secondTransition)
	second := archiveStore.CharacterState{ChatSessionID: sid, CharacterName: "Mina", StatusJSON: `{"coins":4,"mood":"calm"}`, TurnIndex: 3}
	if err := writer.SaveCharacterState(ctx, &second); err != nil {
		t.Fatal(err)
	}
	current, err := st.GetCharacterState(ctx, sid, "Mina")
	if err != nil {
		t.Fatal(err)
	}
	hair := characterFieldMetadata46(t, current, "/appearance/hair")
	if hair["source_turn"] != float64(first.TurnIndex) || hair["source_revision"] != firstRevision || hair["effective_time"] == nil {
		t.Fatalf("old appearance moved to row update: %+v", hair)
	}
	if coin := characterFieldMetadata46(t, current, "/status/coins"); coin["source_turn"] != float64(second.TurnIndex) || coin["occurrence_time"] != nil {
		t.Fatalf("changed field lacks observation or invented occurrence: %+v", coin)
	}
	if mood := characterFieldMetadata46(t, current, "/status/mood"); mood["source_turn"] != float64(first.TurnIndex) {
		t.Fatalf("unchanged same category field moved: %+v", mood)
	}
	history, err := st.(archiveStore.CharacterStateHistoryStore).ListCharacterStateHistory(ctx, sid, "Mina", 50, 0)
	if err != nil || len(history) != 2 || history[0].FieldProvenanceJSON != current.FieldProvenanceJSON {
		t.Fatalf("history lost metadata: %+v err=%v", history, err)
	}
	states, err := st.ListCharacterStates(ctx, sid)
	if err != nil || len(states) != 1 || states[0].FieldProvenanceJSON != current.FieldProvenanceJSON {
		t.Fatalf("latest list lost metadata: %+v err=%v", states, err)
	}
	snapshot, err := st.(archiveStore.SessionStateSnapshotReader).ReadSessionStateSnapshot(ctx, sid)
	if err != nil || len(snapshot.CharacterStates) != 1 || snapshot.CharacterStates[0].FieldProvenanceJSON != current.FieldProvenanceJSON {
		t.Fatalf("session snapshot lost metadata: %+v err=%v", snapshot, err)
	}
	before, err := st.(archiveStore.PrepareTurnRangeStore).ListCharacterStatesCurrentBefore(ctx, sid, second.TurnIndex)
	if err != nil || len(before) != 1 || before[0].TurnIndex != first.TurnIndex || before[0].FieldProvenanceJSON == "" {
		t.Fatalf("before-turn read lost metadata: %+v err=%v", before, err)
	}
	copyResult, err := st.(archiveStore.SessionMigrationStore).CompleteSessionMigration(ctx, archiveStore.SessionMigrationCompleteRequest{SourceSessionID: sid, TargetSessionID: "field-branch", Mode: archiveStore.SessionMigrationModeCopyKeepSource})
	if err != nil {
		t.Fatalf("copy to branch: %v", err)
	}
	if copyResult.MigrationID == 0 {
		t.Fatal("copy did not create existing migration record")
	}
	branched, err := st.GetCharacterState(ctx, "field-branch", "Mina")
	if err != nil || branched.FieldProvenanceJSON != current.FieldProvenanceJSON {
		t.Fatalf("branch changed origin: %+v err=%v", branched, err)
	}
	branchedCurrent, err := st.(archiveStore.StatusCurrentValueStore).ListStatusCurrentValues(ctx, "field-branch", "", "", "narrative_state", -1)
	if err != nil || len(branchedCurrent) != 1 || branchedCurrent[0].ValueJSON != secondTransition.CurrentValue.ValueJSON {
		t.Fatalf("branch operational source binding lost current state: %+v err=%v", branchedCurrent, err)
	}
	var branchEvidence map[string]any
	if err := json.Unmarshal([]byte(branchedCurrent[0].EvidenceJSON), &branchEvidence); err != nil {
		t.Fatal(err)
	}
	if branchEvidence["source_revision"] == secondRevision {
		t.Fatal("copied current retained original operational source revision")
	}
	branchEvents, err := st.(archiveStore.ReversibleStatusTransitionStore).ListLatestReversibleCurrentProjectionEvents(ctx, "field-branch", []string{"narrative_state"})
	if err != nil || len(branchEvents) != 1 || branchEvents[0].NewValueJSON != secondTransition.Event.NewValueJSON {
		t.Fatalf("branch event source binding lost active history: %+v err=%v", branchEvents, err)
	}
	serialized, _ := json.Marshal(current)
	var imported archiveStore.CharacterState
	if err := json.Unmarshal(serialized, &imported); err != nil {
		t.Fatal(err)
	}
	imported.ChatSessionID, imported.ID = "field-json-import", 0
	if err := writer.SaveCharacterState(ctx, &imported); err != nil {
		t.Fatal(err)
	}
	importedCurrent, err := st.GetCharacterState(ctx, imported.ChatSessionID, imported.CharacterName)
	if err != nil {
		t.Fatal(err)
	}
	if origin := characterFieldMetadata46(t, importedCurrent, "/appearance/hair"); origin["source_session_id"] != sid || origin["source_revision"] != firstRevision {
		t.Fatalf("JSON roundtrip imported origin moved: %+v", origin)
	}
	changed := archiveStore.CharacterState{ChatSessionID: imported.ChatSessionID, CharacterName: imported.CharacterName, StatusJSON: `{"coins":3,"mood":"calm"}`, TurnIndex: 4}
	if err := writer.SaveCharacterState(ctx, &changed); err != nil {
		t.Fatal(err)
	}
	importedCurrent, err = st.GetCharacterState(ctx, imported.ChatSessionID, imported.CharacterName)
	if err != nil {
		t.Fatal(err)
	}
	if origin := characterFieldMetadata46(t, importedCurrent, "/status/coins"); origin["source_session_id"] != imported.ChatSessionID || origin["source_turn"] != float64(changed.TurnIndex) {
		t.Fatalf("new imported-session observation kept old source: %+v", origin)
	}
	replacement := &archiveStore.MemorySourceRevision{ContractVersion: "source_acceptance_observation.v1", SourceRevision: "field-replaced-source", ChatSessionID: sid, LogicalTurnID: "turn-3", TurnIndex: second.TurnIndex, BranchState: "observed", UserContent: "Revised user", AssistantContent: "Revised output", CombinedContentHash: strings.Repeat("c", 64), HashAlgorithm: "sha256", HostObservedAtMS: 1, LifecycleState: "active"}
	if err := st.(archiveStore.LogicalTurnReplacementStore).ReplaceLogicalTurn(ctx, archiveStore.LogicalTurnReplacement{ChatSessionID: sid, TurnIndex: second.TurnIndex, UserContent: replacement.UserContent, AssistantContent: replacement.AssistantContent, SourceRevision: replacement}); err != nil {
		t.Fatal(err)
	}
	replacedState, err := st.GetCharacterState(ctx, sid, "Mina")
	if err != nil || replacedState.FieldProvenanceJSON != first.FieldProvenanceJSON {
		t.Fatalf("same-turn replacement lost exact previous field provenance: %+v err=%v", replacedState, err)
	}
	currentStates, err := st.(archiveStore.StatusCurrentValueStore).ListStatusCurrentValues(ctx, sid, "", "", "narrative_state", -1)
	if err != nil || len(currentStates) != 1 || currentStates[0].ValueJSON != firstTransition.CurrentValue.ValueJSON {
		t.Fatalf("replacement retained later source_fields or occurrence time: %+v err=%v", currentStates, err)
	}
	second.FieldProvenanceJSON = ""
	if err := writer.SaveCharacterState(ctx, &second); err != nil {
		t.Fatal(err)
	}
	if err := st.(archiveStore.LogicalTurnReplacementStore).RollbackCanonicalTail(ctx, archiveStore.LogicalTurnRollback{ChatSessionID: sid, TurnIndex: second.TurnIndex, LifecycleAction: archiveStore.LogicalTurnLifecycleDeleted}); err != nil {
		t.Fatal(err)
	}
	rolledBack, err := st.GetCharacterState(ctx, sid, "Mina")
	if err != nil || rolledBack.TurnIndex != first.TurnIndex || rolledBack.FieldProvenanceJSON != first.FieldProvenanceJSON {
		t.Fatalf("rollback did not restore prior snapshot metadata: %+v err=%v", rolledBack, err)
	}
}

func TestCharacterProvenance46MariaDBUpgradeRepairReplayAndUndo(t *testing.T) {
	db, st := feedback43Database(t)
	ctx := context.Background()
	if _, err := db.Exec(`ALTER TABLE character_states DROP COLUMN field_provenance_json`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO character_states (chat_session_id, character_name, appearance_json, status_json, turn_index) VALUES ('legacy-fields','Mina','{"hair":"brown"}','{"coins":5}',2)`); err != nil {
		t.Fatal(err)
	}
	migration := filepath.Join("..", "..", "..", "migrations", "014_character_state_field_provenance.sql")
	statements, err := loadStatements(migration)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := applyStatements(ctx, db, statements, newReport(migration, true)); err != nil {
			t.Fatal(err)
		}
	}
	var unknown bool
	if err := db.QueryRow(`SELECT field_provenance_json IS NULL FROM character_states WHERE chat_session_id='legacy-fields'`).Scan(&unknown); err != nil || !unknown {
		t.Fatalf("upgrade invented provenance: null=%v err=%v", unknown, err)
	}
	next := archiveStore.CharacterState{ChatSessionID: "legacy-fields", CharacterName: "Mina", StatusJSON: `{"coins":4}`, TurnIndex: 7}
	if err := st.(characterStateWriter46).SaveCharacterState(ctx, &next); err != nil {
		t.Fatal(err)
	}
	before, err := st.GetCharacterState(ctx, next.ChatSessionID, next.CharacterName)
	if err != nil {
		t.Fatal(err)
	}
	if hair := characterFieldMetadata46(t, before, "/appearance/hair"); len(hair) != 0 {
		t.Fatalf("legacy hair was assigned latest source: %+v", hair)
	}
	history, err := st.(archiveStore.CharacterStateHistoryStore).ListCharacterStateHistory(ctx, next.ChatSessionID, next.CharacterName, 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	after := *before
	after.FieldProvenanceJSON = archiveStore.BuildCharacterFieldProvenanceFromHistory(history)
	if hair := characterFieldMetadata46(t, &after, "/appearance/hair"); hair["source_turn"] != float64(2) || hair["occurrence_time"] != nil || hair["provenance_basis"] != "earliest_continuous_snapshot" {
		t.Fatalf("repair origin = %+v", hair)
	}
	repair := st.(archiveStore.CharacterProvenanceRepairStore)
	event, err := repair.ApplyCharacterProvenanceRepair(ctx, *before, after, "repair-hair")
	if err != nil || event.ID == 0 {
		t.Fatalf("repair event=%+v err=%v", event, err)
	}
	replay, err := repair.ApplyCharacterProvenanceRepair(ctx, *before, after, "repair-hair")
	if err != nil || replay.ID != event.ID {
		t.Fatalf("repair replay=%+v err=%v", replay, err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM character_states WHERE chat_session_id=?`, before.ChatSessionID).Scan(&count); err != nil || count != 3 {
		t.Fatalf("replay inserted extra snapshot: count=%d err=%v", count, err)
	}
	current, err := st.GetCharacterState(ctx, before.ChatSessionID, before.CharacterName)
	if err != nil || current.FieldProvenanceJSON != after.FieldProvenanceJSON {
		t.Fatalf("repaired snapshot=%+v err=%v", current, err)
	}
	if _, err := repair.ApplyCharacterProvenanceRepair(ctx, *current, *before, "undo-hair"); err != nil {
		t.Fatal(err)
	}
	current, err = st.GetCharacterState(ctx, before.ChatSessionID, before.CharacterName)
	if err != nil || current.FieldProvenanceJSON != before.FieldProvenanceJSON || current.AppearanceJSON != before.AppearanceJSON || current.StatusJSON != before.StatusJSON {
		t.Fatalf("undo changed values or failed exact metadata: %+v err=%v", current, err)
	}
	if _, err := db.Exec(`CREATE TRIGGER fail_field_provenance_audit BEFORE INSERT ON character_events FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'synthetic audit failure'`); err != nil {
		t.Fatal(err)
	}
	if _, err := repair.ApplyCharacterProvenanceRepair(ctx, *before, after, "failed-repair"); err == nil {
		t.Fatal("audit failure was hidden")
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM character_states WHERE chat_session_id=?`, before.ChatSessionID).Scan(&count); err != nil || count != 4 {
		t.Fatalf("failed audit committed partial snapshot: count=%d err=%v", count, err)
	}
}
