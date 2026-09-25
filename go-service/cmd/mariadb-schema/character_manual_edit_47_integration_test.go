package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/httpapi"
	archiveStore "github.com/risulongmemory/archive-center-go/internal/store"
)

func TestCharacterManualEdit47HTTPMariaDBReprocessRollbackCopyAndDelivery(t *testing.T) {
	db, st := feedback43Database(t)
	ctx := context.Background()
	const sid = "manual-edit-47"
	const name = "Mira"
	writer := st.(characterStateWriter46)
	server := httpapi.NewServer(config.Default())
	server.Cfg.StoreMode = config.StoreModeMariaDBAuthority
	server.Store = st
	routes := http.NewServeMux()
	server.RegisterRoutes(routes)
	call := func(method, path, body string) map[string]any {
		t.Helper()
		rec := httptest.NewRecorder()
		routes.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s: %d %s", method, path, rec.Code, rec.Body.String())
		}
		var result map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	get := func(session string) *archiveStore.CharacterState {
		t.Helper()
		state, err := st.GetCharacterState(ctx, session, name)
		if err != nil {
			t.Fatal(err)
		}
		return state
	}
	const typedVoice = `{"contract_version":"voice_behavior_projection.v1","subject_entity_id":"entity-mira","subject_label":"Mira","principles":[{"principle_key":"automatic-old","support_refs":[]}]}`
	first := archiveStore.CharacterState{ChatSessionID: sid, CharacterName: name, TurnIndex: 5, AppearanceJSON: `{"hair":"brown","coat":"red"}`, PersonalityJSON: `{"value":"honesty"}`, SpeechStyleJSON: typedVoice}
	if err := writer.SaveCharacterState(ctx, &first); err != nil {
		t.Fatal(err)
	}
	call(http.MethodPatch, "/characters/"+sid+"/Mira/speech", `{"speech_style":{"default_tone":"manual-warm-47","speech_notes":"manual-note-47"}}`)
	if !strings.Contains(get(sid).SpeechStyleJSON, "manual-note-47") {
		t.Fatal("HTTP edit not stored")
	}
	// Exercise final generation-packet delivery with the real HTTP assembler and
	// MariaDB. No LLM or embedding provider is configured for this synthetic case.
	if err := st.(interface {
		SaveActiveState(context.Context, *archiveStore.ActiveState) error
	}).SaveActiveState(ctx, &archiveStore.ActiveState{ChatSessionID: sid, StateType: "scene", Content: `{"present_entities":["Mira"]}`, TurnIndex: first.TurnIndex}); err != nil {
		t.Fatal(err)
	}
	prepared := call(http.MethodPost, "/prepare-turn", `{"chat_session_id":"manual-edit-47","turn_index":6,"raw_user_input":"Mira speaks.","settings":{"max_injection_chars":32000,"injection_enabled":true,"input_context_enabled":false,"narrative_guide_mode":"none"}}`)
	packet, _ := prepared["generation_packet"].(map[string]any)
	text, _ := packet["injection_text"].(string)
	if !strings.Contains(text, "manual-note-47") || !strings.Contains(text, "manual_override") {
		t.Fatalf("typed manual edit absent from generation packet: %s", text)
	}
	if strings.Contains(text, "automatic-old") {
		t.Fatal("ungrounded automatic principle bypassed typed delivery")
	}
	// An automatic snapshot based on the prior turn must not erase the edit.
	for _, turn := range []int{first.TurnIndex, first.TurnIndex + 1} {
		auto := archiveStore.CharacterState{ChatSessionID: sid, CharacterName: name, TurnIndex: turn, AppearanceJSON: `{"hair":"white","coat":"blue"}`, SpeechStyleJSON: `{}`}
		if err := writer.SaveCharacterState(ctx, &auto); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(get(sid).SpeechStyleJSON, "manual-note-47") {
			t.Fatalf("manual edit lost at turn %d", turn)
		}
	}
	// Full form sends unchanged fields as well. Only the changed hair/value
	// becomes an override; the coat remains free to follow later story facts.
	call(http.MethodPatch, "/characters/"+sid+"/Mira", `{"appearance_json":{"hair":"black","coat":"blue"},"personality_json":{"value":"manual-value-47"}}`)
	auto := archiveStore.CharacterState{ChatSessionID: sid, CharacterName: name, TurnIndex: first.TurnIndex + 2, AppearanceJSON: `{"hair":"white","coat":"green"}`, PersonalityJSON: `{"value":"auto-value","mood":"new-mood"}`, SpeechStyleJSON: typedVoice}
	if err := writer.SaveCharacterState(ctx, &auto); err != nil {
		t.Fatal(err)
	}
	state := get(sid)
	if state.AppearanceJSON != `{"coat":"green","hair":"black"}` || !strings.Contains(state.PersonalityJSON, "new-mood") || !strings.Contains(state.PersonalityJSON, "manual-value-47") {
		t.Fatalf("edited/automatic separation: %+v", state)
	}
	// Registered full form also retains an explicit edit of the principles array.
	voice := map[string]any{}
	_ = json.Unmarshal([]byte(state.SpeechStyleJSON), &voice)
	voice["principles"] = []any{map[string]any{"principle_key": "manual-principle-47", "support_refs": []any{}}}
	fullBody, _ := json.Marshal(map[string]any{"speech_style_json": voice})
	call(http.MethodPatch, "/characters/"+sid+"/Mira", string(fullBody))
	if err := writer.SaveCharacterState(ctx, &auto); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(get(sid).SpeechStyleJSON, "manual-principle-47") {
		t.Fatal("full-form principle overwritten")
	}
	prepared = call(http.MethodPost, "/prepare-turn", `{"chat_session_id":"manual-edit-47","turn_index":8,"raw_user_input":"Mira speaks.","settings":{"max_injection_chars":32000,"injection_enabled":true,"input_context_enabled":false,"narrative_guide_mode":"none"}}`)
	packet, _ = prepared["generation_packet"].(map[string]any)
	text, _ = packet["injection_text"].(string)
	if !strings.Contains(text, "manual-principle-47") {
		t.Fatalf("edited principle did not reach generation packet: %s", text)
	}
	// Turn replacement and rollback use their production transactions, not
	// direct deletes invented by the test.
	lifecycle := st.(archiveStore.LogicalTurnReplacementStore)
	if err := lifecycle.ReplaceLogicalTurn(ctx, archiveStore.LogicalTurnReplacement{ChatSessionID: sid, TurnIndex: 1, UserContent: "Mira speaks", AssistantContent: "Mira replies", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(get(sid).SpeechStyleJSON, "manual-note-47") {
		t.Fatal("replacement erased setting")
	}
	if err := lifecycle.RollbackCanonicalTail(ctx, archiveStore.LogicalTurnRollback{ChatSessionID: sid, TurnIndex: 1, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	state = get(sid)
	if !strings.Contains(state.SpeechStyleJSON, "manual-note-47") || strings.Contains(state.AppearanceJSON, "green") {
		t.Fatalf("rollback lost manual value or resurrected automatic tail: %+v", state)
	}
	call(http.MethodPatch, "/characters/"+sid+"/Mira/speech", `{"speech_style":{"default_tone":"revised-after-rollback","speech_notes":"manual-note-47"}}`)
	if !strings.Contains(get(sid).SpeechStyleJSON, "manual-principle-47") {
		t.Fatal("editing a manual-only note erased its sibling principle")
	}
	before, err := st.(archiveStore.PrepareTurnRangeStore).ListCharacterStatesCurrentBefore(ctx, sid, 1)
	if err != nil || len(before) != 1 || !strings.Contains(before[0].SpeechStyleJSON, "manual-note-47") {
		t.Fatalf("range read lost settings: %+v %v", before, err)
	}
	snapshot, err := st.(archiveStore.SessionStateSnapshotReader).ReadSessionStateSnapshot(ctx, sid)
	if err != nil || len(snapshot.CharacterStates) != 1 || !strings.Contains(snapshot.CharacterStates[0].SpeechStyleJSON, "manual-note-47") {
		t.Fatalf("snapshot read lost settings: %+v %v", snapshot, err)
	}
	if _, err := st.(archiveStore.SessionMigrationStore).CompleteSessionMigration(ctx, archiveStore.SessionMigrationCompleteRequest{SourceSessionID: sid, TargetSessionID: "manual-edit-branch", Mode: archiveStore.SessionMigrationModeCopyKeepSource}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(get("manual-edit-branch").SpeechStyleJSON, "manual-note-47") {
		t.Fatal("copy lost manual edits")
	}
	// Explicit emptying must remain empty on later snapshots, including old
	// incoming content captured before the user's clear action.
	call(http.MethodPatch, "/characters/"+sid+"/Mira/speech", `{"speech_style":{}}`)
	if err := writer.SaveCharacterState(ctx, &auto); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(get(sid).SpeechStyleJSON, "manual-note-47") {
		t.Fatal("clear was undone")
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM character_events WHERE chat_session_id=? AND event_type='manual_character_override' AND turn_index<>0`, sid).Scan(&count); err != nil || count != 0 {
		t.Fatalf("manual settings attached to a rollback turn: %d %v", count, err)
	}
	call(http.MethodDelete, "/characters/"+sid+"/Mira", "")
	if _, err := st.GetCharacterState(ctx, sid, name); err != archiveStore.ErrNotFound {
		t.Fatalf("explicit character deletion resurrected a manual shell: %v", err)
	}
	t.Log("HTTP edit -> MariaDB -> generation packet; same/new turn; replacement; rollback; range/snapshot; copy; clear/delete passed")
}

func TestCharacterManualEdit47LegacyVoiceSurvivesFirstRollback(t *testing.T) {
	db, st := feedback43Database(t)
	ctx := context.Background()
	const sid = "legacy-manual-47"
	state := archiveStore.CharacterState{ChatSessionID: sid, CharacterName: "Mira", TurnIndex: 1, AppearanceJSON: `{"coat":"old-auto"}`, SpeechStyleJSON: `{"contract_version":"voice_behavior_projection.v1","subject_entity_id":"old-identity","manual_overrides":{"speech_notes":"legacy-operator"},"principles":[{"principle_key":"old-auto-principle"}]}`}
	if err := st.(characterStateWriter46).SaveCharacterState(ctx, &state); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM character_events WHERE chat_session_id=?`, sid).Scan(&count); err != nil || count != 0 {
		t.Fatalf("not a legacy fixture: %d %v", count, err)
	}
	if err := st.(archiveStore.LogicalTurnReplacementStore).RollbackCanonicalTail(ctx, archiveStore.LogicalTurnRollback{ChatSessionID: sid, TurnIndex: 1, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	current, err := st.GetCharacterState(ctx, sid, "Mira")
	if err != nil || !strings.Contains(current.SpeechStyleJSON, "legacy-operator") || strings.Contains(current.SpeechStyleJSON, "old-auto-principle") || strings.Contains(current.SpeechStyleJSON, "old-identity") || strings.Contains(current.AppearanceJSON, "old-auto") {
		t.Fatalf("legacy manual recovery carried derived facts or lost setting: %+v %v", current, err)
	}
}
