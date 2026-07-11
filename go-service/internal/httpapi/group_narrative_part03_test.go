package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestCharacterStatePatchAndSpeechRoutesUseLiveStore(t *testing.T) {
	fake := &narrativeFakeStore{
		characterState: &store.CharacterState{
			ID:            4,
			ChatSessionID: "sess-char",
			CharacterName: "Chloe",
			StatusJSON:    `{"emotion":"calm"}`,
			TurnIndex:     6,
		},
		characterStates: []store.CharacterState{
			{ID: 4, ChatSessionID: "sess-char", CharacterName: "Chloe", StatusJSON: `{"emotion":"calm"}`, TurnIndex: 6},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPatch, "/characters/sess-char/Chloe", strings.NewReader(`{"status":{"emotion":"angry"},"relationships":{"Hero":{"affection":70,"tension":45}},"turn_index":7}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedCharacterStates) != 1 {
		t.Fatalf("saved states = %d, want 1", len(fake.savedCharacterStates))
	}
	saved := fake.savedCharacterStates[0]
	if saved.TurnIndex != 7 || !strings.Contains(saved.StatusJSON, "angry") || !strings.Contains(saved.RelationshipsJSON, "affection") {
		t.Fatalf("saved character state = %#v", saved)
	}
	if len(fake.savedCharacterEvents) != 1 || fake.savedCharacterEvents[0].EventType != "manual_patch" {
		t.Fatalf("manual patch event = %#v", fake.savedCharacterEvents)
	}

	req = httptest.NewRequest(http.MethodPatch, "/characters/sess-char/Chloe/speech", strings.NewReader(`{"speech_style":{"default_tone":"dry","speech_notes":"short replies"}}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("speech status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedCharacterStates) != 2 {
		t.Fatalf("saved states after speech = %d, want 2", len(fake.savedCharacterStates))
	}
	if !strings.Contains(fake.savedCharacterStates[1].SpeechStyleJSON, "dry") {
		t.Fatalf("speech style was not saved: %#v", fake.savedCharacterStates[1])
	}
	if len(fake.savedCharacterEvents) != 2 || fake.savedCharacterEvents[1].EventType != "speech_style_patch" {
		t.Fatalf("speech patch event = %#v", fake.savedCharacterEvents)
	}
}

func TestCharacterDeleteUsesLiveStoreAndWritesAudit(t *testing.T) {
	fake := &narrativeFakeStore{
		characterState: &store.CharacterState{
			ID:            8,
			ChatSessionID: "sess-char",
			CharacterName: "Noise",
			StatusJSON:    `{"emotion":"unknown"}`,
			TurnIndex:     9,
			CreatedAt:     time.Date(2026, 6, 1, 1, 2, 3, 0, time.UTC),
			UpdatedAt:     time.Date(2026, 6, 1, 1, 2, 4, 0, time.UTC),
		},
		characterStates: []store.CharacterState{
			{ID: 8, ChatSessionID: "sess-char", CharacterName: "Noise", TurnIndex: 9},
			{ID: 9, ChatSessionID: "sess-char", CharacterName: "Chloe", TurnIndex: 9},
		},
		characterEvents: []store.CharacterEvent{
			{ID: 1, ChatSessionID: "sess-char", CharacterName: "Noise", EventType: "snapshot"},
			{ID: 2, ChatSessionID: "sess-char", CharacterName: "Chloe", EventType: "snapshot"},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/characters/sess-char/Noise", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if fake.deletedCharacterName != "Noise" {
		t.Fatalf("deletedCharacterName = %q, want Noise", fake.deletedCharacterName)
	}
	if len(fake.characterStates) != 1 || fake.characterStates[0].CharacterName != "Chloe" {
		t.Fatalf("character state delete was not scoped: %#v", fake.characterStates)
	}
	if len(fake.characterEvents) != 1 || fake.characterEvents[0].CharacterName != "Chloe" {
		t.Fatalf("character events were not scoped: %#v", fake.characterEvents)
	}
	if len(fake.auditLogs) == 0 {
		t.Fatal("expected manual_delete audit log")
	}
	audit := fake.auditLogs[0]
	if audit.EventType != "manual_delete" || audit.TargetType != "character" || audit.TargetID != 8 {
		t.Fatalf("unexpected audit: %#v", audit)
	}
	if !strings.Contains(audit.DetailsJSON, "character_events_deleted") || !strings.Contains(audit.DetailsJSON, "Noise") {
		t.Fatalf("audit details missing character delete history: %s", audit.DetailsJSON)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["deleted"] != true || resp["audit_written"] != true {
		t.Fatalf("response missing delete/audit proof: %#v", resp)
	}
}

func TestSpeechStylePatchFlowsIntoPrepareTurnPromptWithDistinctTone(t *testing.T) {
	fake := &narrativeFakeStore{
		characterState: &store.CharacterState{
			ID:            9,
			ChatSessionID: "sess-speech-flow",
			CharacterName: "Chloe",
			StatusJSON:    `{"emotion":"focused"}`,
			TurnIndex:     11,
		},
		characterStates: []store.CharacterState{
			{ID: 9, ChatSessionID: "sess-speech-flow", CharacterName: "Chloe", StatusJSON: `{"emotion":"focused"}`, TurnIndex: 11},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	patchSpeech := func(body string) {
		req := httptest.NewRequest(http.MethodPatch, "/characters/sess-speech-flow/Chloe/speech", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("speech patch status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
	}
	prepare := func() (string, string) {
		body := `{"chat_session_id":"sess-speech-flow","turn_index":12,"raw_user_input":"How does Chloe answer the same question?","settings":{"max_injection_chars":1200,"injection_enabled":true,"input_context_enabled":false}}`
		req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("prepare-turn status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode prepare-turn response: %v", err)
		}
		pack, ok := resp["injection_pack"].(map[string]any)
		if !ok {
			t.Fatalf("injection_pack missing: %+v", resp)
		}
		gp, ok := resp["generation_packet"].(map[string]any)
		if !ok {
			t.Fatalf("generation_packet missing: %+v", resp)
		}
		return extractionStringFromAny(pack["character_text"]), extractionStringFromAny(gp["injection_text"])
	}

	patchSpeech(`{"speech_style":{"default_tone":"dry","honorific_style":"plain","speech_notes":"short"}}`)
	dryCharacterText, dryInjection := prepare()
	for _, want := range []string{"speech_style", "dry", "plain", "short"} {
		if !strings.Contains(dryCharacterText, want) || !strings.Contains(dryInjection, want) {
			t.Fatalf("dry speech style %q missing from character/injection text:\ncharacter=%s\ninjection=%s", want, dryCharacterText, dryInjection)
		}
	}

	patchSpeech(`{"speech_style":{"default_tone":"warm","honorific_style":"formal","speech_notes":"careful"}}`)
	warmCharacterText, warmInjection := prepare()
	for _, want := range []string{"speech_style", "warm", "formal", "careful"} {
		if !strings.Contains(warmCharacterText, want) || !strings.Contains(warmInjection, want) {
			t.Fatalf("warm speech style %q missing from character/injection text:\ncharacter=%s\ninjection=%s", want, warmCharacterText, warmInjection)
		}
	}
	for _, old := range []string{"dry", "plain", "short"} {
		if strings.Contains(warmCharacterText, old) || strings.Contains(warmInjection, old) {
			t.Fatalf("old speech style %q leaked after second patch:\ncharacter=%s\ninjection=%s", old, warmCharacterText, warmInjection)
		}
	}
	if dryInjection == warmInjection {
		t.Fatalf("same situation should produce distinct prompt text after speech style patch")
	}

	oldClient := proxyHTTPClient
	defer func() { proxyHTTPClient = oldClient }()
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(r.Body)
		body := string(raw)
		reply := ""
		switch {
		case strings.Contains(body, "dry") && strings.Contains(body, "plain") && strings.Contains(body, "short"):
			reply = "Dry. Short answer."
		case strings.Contains(body, "warm") && strings.Contains(body, "formal") && strings.Contains(body, "careful"):
			reply = "I will answer carefully in a warm, formal tone."
		default:
			t.Fatalf("proxy prompt did not carry expected speech style: %s", body)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"model":"style-replay","choices":[{"message":{"content":"` + reply + `"}}]}`)),
		}, nil
	})}
	callMainProxy := func(injection string) string {
		req := httptest.NewRequest(http.MethodPost, "/proxy/plugin-main", strings.NewReader(`{
			"endpoint":"https://api.example.com/v1",
			"api_key":"sk-style",
			"model":"style-replay",
			"provider":"openai",
			"messages":[
				{"role":"system","content":"Answer according to the supplied character speech_style."},
				{"role":"user","content":`+strconv.Quote(injection)+`}
			]
		}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("plugin-main status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode plugin-main response: %v", err)
		}
		choices, _ := resp["choices"].([]any)
		if len(choices) == 0 {
			t.Fatalf("plugin-main response missing choices: %+v", resp)
		}
		choice, _ := choices[0].(map[string]any)
		message, _ := choice["message"].(map[string]any)
		return extractionStringFromAny(message["content"])
	}
	dryReply := callMainProxy(dryInjection)
	warmReply := callMainProxy(warmInjection)
	if !strings.Contains(dryReply, "Dry") || !strings.Contains(dryReply, "Short") {
		t.Fatalf("dry reply did not reflect dry/short speech style: %q", dryReply)
	}
	if !strings.Contains(warmReply, "warm") || !strings.Contains(warmReply, "formal") {
		t.Fatalf("warm reply did not reflect warm/formal speech style: %q", warmReply)
	}
	if dryReply == warmReply {
		t.Fatalf("controlled provider replies should differ by speech style")
	}
}

func TestContinuityPackAssemblesPythonReferenceSources(t *testing.T) {
	now := time.Date(2026, 5, 31, 10, 0, 0, 0, time.UTC)
	fake := &narrativeFakeStore{
		storylines: []store.Storyline{
			{ID: 1, ChatSessionID: "sess-1", Name: "Fresh gate arc", Status: "active", CurrentContext: "Gate pressure rises.", LastTurn: 12, LastEvidenceTurn: 12},
		},
		characterEvents: []store.CharacterEvent{
			{ID: 1, ChatSessionID: "sess-1", CharacterName: "Alice", TurnIndex: 8, EventType: "speech_style_patch", DetailsJSON: `{"detail":"ignore"}`, CreatedAt: now.Add(-3 * time.Minute)},
			{ID: 2, ChatSessionID: "sess-1", CharacterName: "Bob", TurnIndex: 10, EventType: "relationship_shift", DetailsJSON: `{"detail":"trust warms"}`, CreatedAt: now.Add(-2 * time.Minute)},
			{ID: 3, ChatSessionID: "sess-1", CharacterName: "Alice", TurnIndex: 11, EventType: "relationship_shift", DetailsJSON: `{"detail":"trust sharpens"}`, CreatedAt: now.Add(-1 * time.Minute)},
		},
		pendingThreads: []store.PendingThread{
			{ID: 1, ChatSessionID: "sess-1", ThreadKey: "suppressed", Status: "open", SourceTurn: 13, Suppressed: true},
			{ID: 2, ChatSessionID: "sess-1", ThreadKey: "resolved", Status: "resolved", SourceTurn: 14},
			{ID: 3, ChatSessionID: "sess-1", ThreadKey: "promise", Status: "open", SourceTurn: 9, LastSeenTurn: 15, Pinned: true},
			{ID: 4, ChatSessionID: "sess-1", ThreadKey: "risk", Status: "paused", SourceTurn: 8, LastSeenTurn: 12},
		},
		episodeSummaries: []store.EpisodeSummary{
			{ID: 1, ChatSessionID: "sess-1", FromTurn: 1, ToTurn: 20, SummaryText: "They reach the gate."},
		},
		worldRules: []store.WorldRule{
			{ID: 1, ChatSessionID: "sess-1", Scope: "location", ScopeName: "Archive", Category: "access", Key: "sealed", SourceTurn: 12},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/continuity-pack/sess-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode continuity pack: %v", err)
	}
	if resp["pack_status"] != "ready" || resp["skeleton_only"] != false {
		t.Fatalf("pack status = %#v skeleton=%#v, want ready/false", resp["pack_status"], resp["skeleton_only"])
	}
	relationshipShifts := resp["relationship_shifts"].([]any)
	if len(relationshipShifts) != 2 {
		t.Fatalf("relationship_shifts len = %d, want 2: %#v", len(relationshipShifts), relationshipShifts)
	}
	firstShift := relationshipShifts[0].(map[string]any)
	if firstShift["event_type"] != "relationship_shift" || firstShift["character_name"] != "Alice" || firstShift["turn_index"] != float64(11) {
		t.Fatalf("relationship shift ordering/shape mismatch: %#v", firstShift)
	}
	pendingThreads := resp["pending_threads"].([]any)
	if len(pendingThreads) != 2 {
		t.Fatalf("pending_threads len = %d, want 2: %#v", len(pendingThreads), pendingThreads)
	}
	firstThread := pendingThreads[0].(map[string]any)
	if firstThread["thread_key"] != "promise" && firstThread["title"] != "promise" {
		t.Fatalf("pinned/open pending thread should sort first and exclude suppressed/resolved: %#v", pendingThreads)
	}
	sectionStatus := resp["section_status"].(map[string]any)
	for _, key := range []string{"active_storylines", "relationship_shifts", "pending_threads", "continuity_hooks", "latest_episode", "world_constraints"} {
		if _, ok := sectionStatus[key]; !ok {
			t.Fatalf("section_status missing %s: %#v", key, sectionStatus)
		}
	}
	if resp["latest_episode"] == nil {
		t.Fatal("latest_episode missing")
	}
	worldConstraints := resp["world_constraints"].([]any)
	if len(worldConstraints) != 1 {
		t.Fatalf("world_constraints len = %d, want 1", len(worldConstraints))
	}
}

func TestContinuityPackEmptySessionKeepsHooksAlias(t *testing.T) {
	fake := &narrativeFakeStore{}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/continuity-pack/sess-empty", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode continuity pack: %v", err)
	}
	if resp["pack_status"] != "empty" || resp["skeleton_only"] != false {
		t.Fatalf("empty pack status = %#v skeleton=%#v, want empty/false", resp["pack_status"], resp["skeleton_only"])
	}
	sectionStatus := resp["section_status"].(map[string]any)
	hooks, ok := sectionStatus["continuity_hooks"].(map[string]any)
	if !ok {
		t.Fatalf("continuity_hooks status missing: %#v", sectionStatus)
	}
	if hooks["count"] != float64(0) {
		t.Fatalf("continuity_hooks count = %#v, want 0", hooks["count"])
	}
	warnings := resp["warnings"].([]any)
	if len(warnings) == 0 {
		t.Fatal("empty continuity pack should include a warning")
	}
}

func TestWorldRulesSyncPatchTrustDeleteUseLiveStore(t *testing.T) {
	fake := &narrativeFakeStore{}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	syncBody := `{
		"chat_session_id":"sess-world",
		"mode":"apply",
		"turn_index":9,
		"supervisor_response":{
			"section_world":{
				"genre_hint":"mystery",
				"constants":[{"category":"physics","key":"sealed_gate","value":{"opens":"brass_key"}}],
				"rules":["The archive cellar stays locked after midnight."],
				"world_rules":["Legacy fallback rule is still accepted."],
				"confidence_notes":["Confidence note fallback is still accepted."]
			}
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/world-rules/sync", strings.NewReader(syncBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("sync status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedWorldRules) != 4 {
		t.Fatalf("saved world rules = %d, want 4: %#v", len(fake.savedWorldRules), fake.savedWorldRules)
	}
	if got := fake.savedWorldRules[0]; got.ChatSessionID != "sess-world" || got.Scope != "root" || got.Category != "physics" || got.Key != "sealed_gate" || got.Genre != "mystery" || got.SourceTurn != 9 || !strings.Contains(got.ValueJSON, "brass_key") {
		t.Fatalf("constant world rule = %#v", got)
	}
	if got := fake.savedWorldRules[1]; got.Scope != "root" || got.Category != "custom" || got.Key != "The archive cellar stays locked after midnight." || got.ValueJSON != "" {
		t.Fatalf("string world rule = %#v", got)
	}
	if got := fake.savedWorldRules[2]; got.Scope != "root" || got.Category != "custom" || got.Key != "Legacy fallback rule is still accepted." {
		t.Fatalf("world_rules fallback rule = %#v", got)
	}
	if got := fake.savedWorldRules[3]; got.Scope != "root" || got.Category != "custom" || got.Key != "Confidence note fallback is still accepted." {
		t.Fatalf("confidence_notes fallback rule = %#v", got)
	}

	req = httptest.NewRequest(http.MethodPatch, "/world-rules/7", strings.NewReader(`{"scope":"location","scope_name":"Archive","value":"The cellar rule changed.","source_turn":10}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.worldRulePatches) != 1 || fake.worldRulePatches[0]["scope"] != "location" || fake.worldRulePatches[0]["scope_name"] != "Archive" || fake.worldRulePatches[0]["source_turn"] != 10 || !strings.Contains(fmt.Sprint(fake.worldRulePatches[0]["value_json"]), "cellar") {
		t.Fatalf("world rule patch = %#v", fake.worldRulePatches)
	}

	req = httptest.NewRequest(http.MethodPatch, "/world-rules/7/trust", strings.NewReader(`{"pinned":true,"suppressed":false,"user_corrected":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("trust status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if fake.worldRuleTrustPatch["pinned"] != true || fake.worldRuleTrustPatch["suppressed"] != false || fake.worldRuleTrustPatch["user_corrected"] != true {
		t.Fatalf("world rule trust patch = %#v", fake.worldRuleTrustPatch)
	}

	req = httptest.NewRequest(http.MethodDelete, "/world-rules/7", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if fake.deletedWorldRuleID != 7 {
		t.Fatalf("deletedWorldRuleID = %d, want 7", fake.deletedWorldRuleID)
	}
}

func TestWorldRulesInheritedIncludesSessionScopedRules(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	fake := &narrativeFakeStore{
		worldRules: []store.WorldRule{
			{ID: 1, ChatSessionID: "sess-world", Scope: "root", Category: "cosmology", Key: "cassia_created_world", ValueJSON: `{"rule":"Cassia ordered chaos into the world."}`, UpdatedAt: now},
			{ID: 2, ChatSessionID: "sess-world", Scope: "session", ScopeName: "Cassia Doctrine", Category: "cosmology", Key: "apostles_borrow_divine_power", ValueJSON: `{"rule":"Apostles borrow divine power against monsters."}`, UpdatedAt: now},
			{ID: 3, ChatSessionID: "sess-world", Scope: "session", Category: "hidden", Key: "suppressed_rule", Suppressed: true, UpdatedAt: now},
			{ID: 4, ChatSessionID: "other", Scope: "session", Category: "other", Key: "other_session_rule", UpdatedAt: now},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/world-rules/sess-world/inherited?active_scope=root", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("inherited status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Status     string           `json:"status"`
		ScopeChain []string         `json:"scope_chain"`
		Rules      []map[string]any `json:"rules"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "ok" {
		t.Fatalf("status = %q", body.Status)
	}
	if !reflect.DeepEqual(body.ScopeChain, []string{"root", "session"}) {
		t.Fatalf("scope chain = %#v, want root+session", body.ScopeChain)
	}
	if len(body.Rules) != 2 {
		t.Fatalf("rules = %d, want 2: %#v", len(body.Rules), body.Rules)
	}
	var sawSession bool
	for _, rule := range body.Rules {
		if rule["scope"] == "session" && rule["key"] == "apostles_borrow_divine_power" {
			sawSession = true
			if rule["inherited"] != true {
				t.Fatalf("session rule should be marked inherited under root active scope: %#v", rule)
			}
		}
	}
	if !sawSession {
		t.Fatalf("session scoped rule missing from inherited response: %#v", body.Rules)
	}
}

func TestStorylineAndWorldRuleReadExposeFreshnessFields(t *testing.T) {
	now := time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC)
	fake := &narrativeFakeStore{
		storylines: []store.Storyline{
			{
				ID:                  1,
				ChatSessionID:       "sess-fresh",
				Name:                "Rooftop Promise",
				Status:              "active",
				Confidence:          0.82,
				EvidenceCount:       4,
				LastEvidenceTurn:    11,
				KeyPointsJSON:       `[" clue ","clue","Clue"]`,
				OngoingTensionsJSON: `["answer pending","answer pending"," answer pending "]`,
				LastTurn:            12,
				UpdatedAt:           now,
			},
		},
		worldRules: []store.WorldRule{
			{
				ID:            2,
				ChatSessionID: "sess-fresh",
				Scope:         "root",
				Category:      "custom",
				Key:           "The archive cellar stays locked.",
				SourceTurn:    12,
				UpdatedAt:     now,
			},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/storylines/sess-fresh", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("storylines status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var storylineResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &storylineResp); err != nil {
		t.Fatalf("decode storylines: %v", err)
	}
	storylines, _ := storylineResp["storylines"].([]any)
	if len(storylines) != 1 {
		t.Fatalf("storyline count = %d, want 1: %#v", len(storylines), storylineResp)
	}
	storyline := storylines[0].(map[string]any)
	if storyline["last_turn"] != float64(12) || strings.TrimSpace(fmt.Sprint(storyline["updated_at"])) == "" {
		t.Fatalf("storyline freshness fields missing: %#v", storyline)
	}
	if storyline["confidence"] != float64(0.82) || storyline["evidence_count"] != float64(4) || storyline["last_evidence_turn"] != float64(11) {
		t.Fatalf("storyline quality fields missing: %#v", storyline)
	}
	if storyline["last_observed_turn"] != float64(11) || storyline["stale_after_turns"] != float64(6) {
		t.Fatalf("storyline stale snapshot fields missing: %#v", storyline)
	}
	if storyline["key_points_json"] != `["clue"]` || storyline["ongoing_tensions_json"] != `["answer pending"]` {
		t.Fatalf("storyline read path did not normalize key/tension lists: %#v", storyline)
	}

	req = httptest.NewRequest(http.MethodGet, "/world-rules/sess-fresh", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("world-rules status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var worldResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &worldResp); err != nil {
		t.Fatalf("decode world-rules: %v", err)
	}
	items, _ := worldResp["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("world-rule count = %d, want 1: %#v", len(items), worldResp)
	}
	rule := items[0].(map[string]any)
	if rule["source_turn"] != float64(12) || strings.TrimSpace(fmt.Sprint(rule["updated_at"])) == "" {
		t.Fatalf("world-rule freshness fields missing: %#v", rule)
	}
}

func TestNarrativeRoutesErrNotEnabledFallback(t *testing.T) {
	fake := &narrativeFakeStore{errNotEnabled: true}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	tests := []string{
		"/storylines/sess-1",
		"/world-rules/sess-1",
		"/world-rules/sess-1/inherited",
		"/characters/sess-1",
		"/characters/sess-1/Alice",
		"/characters/sess-1/Alice/events",
		"/pending-threads/sess-1",
		"/active-states/sess-1",
		"/canonical-state-layer/sess-1",
		"/episodes/sess-1",
		"/session-state/sess-1",
		"/continuity-pack/sess-1",
	}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
			}
			var resp map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if resp["status"] != "ok" {
				t.Errorf("status = %v, want ok", resp["status"])
			}
		})
	}
}

func TestFiveNarrativeHandlersStoreBacked(t *testing.T) {
	fake := &narrativeFakeStore{
		storylines: []store.Storyline{
			{ID: 1, ChatSessionID: "sess-1", Name: "Arc 1", Status: "active"},
		},
		worldRules: []store.WorldRule{
			{ID: 1, ChatSessionID: "sess-1", Scope: "global", Category: "magic", Key: "mana"},
		},
		characterStates: []store.CharacterState{
			{ID: 1, ChatSessionID: "sess-1", CharacterName: "Alice", TurnIndex: 5},
		},
		pendingThreads: []store.PendingThread{
			{ID: 1, ChatSessionID: "sess-1", ThreadKey: "hook-1", Status: "open"},
		},
		activeStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "sess-1", StateType: "scene_state", Content: `{\"loc\":\"temple\"}`, TurnIndex: 7},
		},
		episodeSummaries: []store.EpisodeSummary{
			{ID: 1, ChatSessionID: "sess-1", FromTurn: 1, ToTurn: 10, SummaryText: "Alice arrives."},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	tests := []struct {
		path        string
		wantCode    int
		wantSource  string
		wantPresent []string
		wantAbsent  []string
	}{
		{
			path:        "/sessions/sess-1/guidance-snapshot",
			wantCode:    http.StatusOK,
			wantPresent: []string{"story_plan", "director", "compact_records", "maintenance_last", "last_turn"},
			wantAbsent:  []string{"source", "storyline_count", "trace_summary"},
		},
		{
			path:        "/sessions/sess-1/step7-health",
			wantCode:    http.StatusOK,
			wantPresent: []string{"total_turns", "guidance_state", "drift_summary", "compaction_summary", "maintenance_summary", "regression_checks"},
			wantAbsent:  []string{"source", "storyline_count", "trace_summary"},
		},
		{
			path:        "/session/sess-1/active-scope",
			wantCode:    http.StatusOK,
			wantSource:  "default",
			wantPresent: []string{"active_scope", "scope_chain", "updated_at"},
		},
		{
			path:        "/momentum-packet/sess-1",
			wantCode:    http.StatusOK,
			wantPresent: []string{"next_pressure", "payoff_candidates", "tension_to_reuse", "beats_to_avoid", "generated_at"},
			wantAbsent:  []string{"source", "storyline_count", "trace_summary"},
		},
		{
			path:        "/narrative-control/sess-1",
			wantCode:    http.StatusOK,
			wantPresent: []string{"story_plan", "director", "progression_ledger", "story_guidance", "generated_at"},
			wantAbsent:  []string{"source", "storyline_count", "trace_summary"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tt.wantCode, rec.Body.String())
			}
			var resp map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if resp["status"] != "ok" {
				t.Errorf("status = %v, want ok", resp["status"])
			}
			if tt.wantSource != "" && resp["source"] != tt.wantSource {
				t.Errorf("source = %v, want %v", resp["source"], tt.wantSource)
			}
			for _, field := range tt.wantPresent {
				if _, ok := resp[field]; !ok {
					t.Errorf("missing field %s", field)
				}
			}
			for _, field := range tt.wantAbsent {
				if _, ok := resp[field]; ok {
					t.Errorf("unexpected field %s", field)
				}
			}
			if tt.path == "/session/sess-1/active-scope" && resp["active_scope"] != "root" {
				t.Errorf("active_scope = %v, want root", resp["active_scope"])
			}
		})
	}
}

func TestFiveNarrativeHandlersErrNotEnabledFallback(t *testing.T) {
	fake := &narrativeFakeStore{errNotEnabled: true}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	tests := []struct {
		path             string
		wantPacketStatus string
		wantStateStatus  string
	}{
		{path: "/sessions/sess-1/guidance-snapshot"},
		{path: "/sessions/sess-1/step7-health"},
		{path: "/session/sess-1/active-scope"},
		{path: "/momentum-packet/sess-1", wantPacketStatus: "empty"},
		{path: "/narrative-control/sess-1", wantStateStatus: "skeleton"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
			}
			var resp map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if resp["status"] != "ok" {
				t.Errorf("status = %v, want ok", resp["status"])
			}
			if tt.wantStateStatus != "" {
				if resp["state_status"] != tt.wantStateStatus {
					t.Errorf("state_status = %v, want %v", resp["state_status"], tt.wantStateStatus)
				}
			}
			if tt.wantPacketStatus != "" {
				if resp["packet_status"] != tt.wantPacketStatus {
					t.Errorf("packet_status = %v, want %v", resp["packet_status"], tt.wantPacketStatus)
				}
			}
		})
	}
}

func TestMetricsRoutesStoreBackedEvidence(t *testing.T) {
	fake := &narrativeFakeStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-1", TurnIndex: 1, Role: "user"},
			{ID: 2, ChatSessionID: "sess-1", TurnIndex: 2, Role: "assistant"},
		},
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-1", TurnIndex: 1},
		},
		evidence: []store.DirectEvidence{
			{ID: 1, ChatSessionID: "sess-1", EvidenceKind: "fact"},
		},
		kgTriples: []store.KGTriple{
			{ID: 1, ChatSessionID: "sess-1", Subject: "Alice", Predicate: "trusts", Object: "Bob"},
		},
		storylines: []store.Storyline{
			{ID: 1, ChatSessionID: "sess-1", Name: "Arc 1", Status: "active"},
		},
		worldRules: []store.WorldRule{
			{ID: 1, ChatSessionID: "sess-1", Scope: "global", Key: "rule"},
		},
		characterStates: []store.CharacterState{
			{ID: 1, ChatSessionID: "sess-1", CharacterName: "Alice"},
		},
		pendingThreads: []store.PendingThread{
			{ID: 1, ChatSessionID: "sess-1", ThreadKey: "hook-1", Status: "open"},
		},
		activeStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "sess-1", StateType: "scene_state"},
		},
		canonicalStateLayers: []store.CanonicalStateLayer{
			{ID: 1, ChatSessionID: "sess-1", LayerType: "scene_state"},
		},
		episodeSummaries: []store.EpisodeSummary{
			{ID: 1, ChatSessionID: "sess-1", FromTurn: 1, ToTurn: 2},
		},
		resumePack: &store.ResumePack{PackStatus: "ok", Trigger: "resume"},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	tests := []struct {
		path        string
		wantCount   string
		wantValue   float64
		wantPayload string
	}{
		{path: "/metrics/lc1d/sess-1", wantPayload: "integrity_replay"},
		{path: "/metrics/lc1e/sess-1", wantCount: "kg_triple_count", wantValue: 1},
		{path: "/metrics/lc1f/sess-1", wantCount: "storyline_count", wantValue: 1},
		{path: "/metrics/lc1g/sess-1", wantCount: "world_rule_count", wantValue: 1},
		{path: "/metrics/lc1h/sess-1", wantCount: "character_state_count", wantValue: 1},
		{path: "/metrics/lc1i/sess-1", wantCount: "pending_thread_count", wantValue: 1},
		{path: "/metrics/lc1j/sess-1", wantCount: "resume_pack_present", wantValue: 1},
		{path: "/metrics/lc1k/sess-1", wantCount: "memory_count", wantValue: 1},
		{path: "/metrics/lc1l/sess-1", wantCount: "evidence_count", wantValue: 1},
		{path: "/metrics/lc1m/sess-1", wantCount: "episode_summary_count", wantValue: 1},
		{path: "/metrics/lc1n/sess-1", wantCount: "active_state_count", wantValue: 1},
		{path: "/metrics/lc1o/sess-1", wantCount: "canonical_state_layer_count", wantValue: 1},
		{path: "/metrics/lc1p/sess-1", wantCount: "storyline_count", wantValue: 1},
		{path: "/metrics/lc1q/sess-1", wantPayload: "freshness_lag_summary"},
		{path: "/metrics/tm1d/sess-1", wantPayload: "truth_maintenance_audit_replay"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
			}
			var resp map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if resp["status"] != "ok" {
				t.Errorf("status = %v, want ok", resp["status"])
			}
			if tt.wantPayload != "" {
				if _, ok := resp[tt.wantPayload].(map[string]any); !ok {
					t.Fatalf("%s missing or wrong type: %#v", tt.wantPayload, resp[tt.wantPayload])
				}
				if _, ok := resp["counts"]; ok {
					t.Fatalf("unexpected counts in Python-compatible metric shape: %#v", resp)
				}
				return
			}
			if resp["store_status"] != "active" {
				t.Errorf("store_status = %v, want active", resp["store_status"])
			}
			counts, ok := resp["counts"].(map[string]any)
			if !ok {
				t.Fatalf("counts missing or wrong type: %#v", resp["counts"])
			}
			if counts[tt.wantCount] != tt.wantValue {
				t.Errorf("counts[%s] = %v, want %v", tt.wantCount, counts[tt.wantCount], tt.wantValue)
			}
		})
	}
}

func TestSeq17MetricsLC1PEvaluationSplitSurface(t *testing.T) {
	fake := &narrativeFakeStore{
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-lc1p", SummaryJSON: `{"text":"memory"}`},
		},
		evidence: []store.DirectEvidence{
			{ID: 1, ChatSessionID: "sess-lc1p", EvidenceText: "direct evidence", TurnAnchor: 2},
		},
		kgTriples: []store.KGTriple{
			{ID: 1, ChatSessionID: "sess-lc1p", Subject: "Alice", Predicate: "knows", Object: "Bob"},
		},
		episodeSummaries: []store.EpisodeSummary{
			{ID: 1, ChatSessionID: "sess-lc1p", FromTurn: 1, ToTurn: 3, SummaryText: "episode"},
		},
		activeStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "sess-lc1p", StateType: "scene", Content: "active"},
		},
		storylines: []store.Storyline{
			{ID: 1, ChatSessionID: "sess-lc1p", Name: "arc", Status: "active"},
		},
		pendingThreads: []store.PendingThread{
			{ID: 1, ChatSessionID: "sess-lc1p", ThreadKey: "hook", Status: "open"},
		},
		worldRules: []store.WorldRule{
			{ID: 1, ChatSessionID: "sess-lc1p", Scope: "global", Key: "rule"},
		},
		characterStates: []store.CharacterState{
			{ID: 1, ChatSessionID: "sess-lc1p", CharacterName: "Alice"},
		},
		canonicalStateLayers: []store.CanonicalStateLayer{
			{ID: 1, ChatSessionID: "sess-lc1p", LayerType: "scene"},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/metrics/lc1p/sess-lc1p", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	split, ok := resp["evaluation_split"].(map[string]any)
	if !ok {
		t.Fatalf("evaluation_split missing: %#v", resp)
	}
	retrieval, ok := resp["retrieval_completeness"].(map[string]any)
	if !ok {
		t.Fatalf("retrieval_completeness missing: %#v", resp)
	}
	quality, ok := resp["final_answer_quality"].(map[string]any)
	if !ok {
		t.Fatalf("final_answer_quality missing: %#v", resp)
	}
	failure, ok := resp["failure_split"].(map[string]any)
	if !ok {
		t.Fatalf("failure_split missing: %#v", resp)
	}
	if split["policy_version"] != "lc1p.v1" {
		t.Fatalf("policy_version=%v, want lc1p.v1", split["policy_version"])
	}
	if retrieval["policy_version"] != "s17-1a.v1" || retrieval["metric_defined"] != true {
		t.Fatalf("retrieval metric mismatch: %#v", retrieval)
	}
	if quality["policy_version"] != "s17-1b.v1" || quality["metric_defined"] != true {
		t.Fatalf("quality metric mismatch: %#v", quality)
	}
	if failure["policy_version"] != "s17-1c.v1" || failure["replay_defined"] != true {
		t.Fatalf("failure split mismatch: %#v", failure)
	}
	if failure["classification"] != "healthy" {
		t.Fatalf("classification=%v, want healthy", failure["classification"])
	}
}

func TestSeq17MetricsLC1QFreshnessLagAnswerQualitySplit(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = &narrativeFakeStore{}
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/metrics/lc1q/sess-lc1q", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	freshness, ok := resp["freshness_lag_summary"].(map[string]any)
	if !ok {
		t.Fatalf("freshness_lag_summary missing: %#v", resp)
	}
	if freshness["policy_version"] != "lc1q.v1" || freshness["metric_defined"] != true {
		t.Fatalf("freshness metric mismatch: %#v", freshness)
	}
	lags, ok := freshness["lags_seconds"].(map[string]any)
	if !ok {
		t.Fatalf("lags_seconds missing: %#v", freshness)
	}
	for _, key := range []string{"save_delay", "extraction_delay", "promotion_visibility_lag"} {
		if _, ok := lags[key]; !ok {
			t.Fatalf("lags_seconds[%s] missing: %#v", key, lags)
		}
	}
	qualitySplit, ok := freshness["answer_quality_split"].(map[string]any)
	if !ok {
		t.Fatalf("answer_quality_split missing: %#v", freshness)
	}
	if qualitySplit["extraction_delay_affects_answer_quality"] != true ||
		qualitySplit["save_delay_affects_answer_quality"] != true ||
		qualitySplit["promotion_visibility_lag_affects_answer_quality"] != true {
		t.Fatalf("answer_quality_split mismatch: %#v", qualitySplit)
	}
}

func TestMetricsLC1CMeasuresCanonicalDenseLedgerFootprint(t *testing.T) {
	chatLogs := make([]store.ChatLog, 300)
	for i := range chatLogs {
		chatLogs[i] = store.ChatLog{
			ID:            int64(i + 1),
			ChatSessionID: "sess-lc1c",
			TurnIndex:     i + 1,
			Role:          "assistant",
			Content:       "turn content",
		}
	}
	chapter := &store.ChapterSummary{SummaryText: "chapter dense summary", ResumeText: "chapter resume"}
	arc := &store.ArcSummary{CoreConflict: "arc conflict", ArcResumeText: "arc resume"}
	saga := &store.SagaDigest{SagaSummary: "saga summary", ResumePackText: "saga resume"}
	expectedDenseChars := len([]rune("episode dense summary")) +
		len([]rune(chapter.SummaryText)) + len([]rune(chapter.ResumeText)) +
		len([]rune(arc.CoreConflict)) + len([]rune(arc.ArcResumeText)) +
		len([]rune(saga.SagaSummary)) + len([]rune(saga.ResumePackText))
	expectedCanonicalChars := len([]rune(`{"scene_state":{"mood":"tense"}}`)) + len([]rune(`{"relationship_state":{"trust":"rising"}}`))

	fake := &narrativeFakeStore{
		chatLogs: chatLogs,
		canonicalStateLayers: []store.CanonicalStateLayer{
			{ID: 1, ChatSessionID: "sess-lc1c", LayerType: "scene_state", Content: `{"scene_state":{"mood":"tense"}}`, TurnIndex: 299},
			{ID: 2, ChatSessionID: "sess-lc1c", LayerType: "relationship_state", Content: `{"relationship_state":{"trust":"rising"}}`, TurnIndex: 300},
		},
		episodeSummaries: []store.EpisodeSummary{
			{ID: 1, ChatSessionID: "sess-lc1c", FromTurn: 1, ToTurn: 300, SummaryText: "episode dense summary"},
		},
		resumePack: &store.ResumePack{PackStatus: "ok", Trigger: "resume", Chapter: chapter, Arc: arc, Saga: saga},
		storylines: []store.Storyline{
			{ID: 1, ChatSessionID: "sess-lc1c", Name: "Sealed ledger", Status: "active", CurrentContext: "The ledger remains dangerous.", LastTurn: 300, Confidence: 0.9},
		},
		worldRules: []store.WorldRule{
			{ID: 1, ChatSessionID: "sess-lc1c", Scope: "session", Category: "world", Key: "ledger_seal", ValueJSON: `{"rule":"Seal changes require evidence."}`, SourceTurn: 280},
		},
		pendingThreads: []store.PendingThread{
			{ID: 1, ChatSessionID: "sess-lc1c", ThreadKey: "ledger-payoff", Status: "open", Description: "Pay off the ledger promise", SourceTurn: 295},
		},
		activeStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "sess-lc1c", StateType: "scene_state", Content: `{"pressure":"high"}`, TurnIndex: 300},
		},
		characterStates: []store.CharacterState{
			{ID: 1, ChatSessionID: "sess-lc1c", CharacterName: "Mina", StatusJSON: `{"summary":"Mina protects the ledger"}`, TurnIndex: 300},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/metrics/lc1c/sess-lc1c", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	mfp, ok := resp["memory_footprint"].(map[string]any)
	if !ok {
		t.Fatalf("memory_footprint missing or wrong type: %#v", resp["memory_footprint"])
	}
	if mfp["policy_version"] != "lc1c.v1" {
		t.Fatalf("policy_version = %v, want lc1c.v1", mfp["policy_version"])
	}
	if mfp["turn_window"] != float64(300) || mfp["latest_turn_index"] != float64(300) || mfp["window_start_turn"] != float64(1) {
		t.Fatalf("window metrics mismatch: %#v", mfp)
	}
	if mfp["canonical_state_chars"] != float64(expectedCanonicalChars) {
		t.Fatalf("canonical_state_chars = %v, want %d", mfp["canonical_state_chars"], expectedCanonicalChars)
	}
	if mfp["dense_summary_chars"] != float64(expectedDenseChars) {
		t.Fatalf("dense_summary_chars = %v, want %d", mfp["dense_summary_chars"], expectedDenseChars)
	}
	if live := mfp["live_ledger_chars"].(float64); live <= 0 {
		t.Fatalf("live_ledger_chars = %v, want > 0", live)
	}
	total := mfp["total_chars"].(float64)
	if total <= float64(expectedCanonicalChars+expectedDenseChars) {
		t.Fatalf("total_chars = %v, want canonical+dense+ledger footprint", total)
	}
	counts, ok := mfp["counts"].(map[string]any)
	if !ok {
		t.Fatalf("counts missing or wrong type: %#v", mfp["counts"])
	}
	wantCounts := map[string]float64{
		"canonical_layers": 2,
		"episodes":         1,
		"chapters":         1,
		"arcs":             1,
		"sagas":            1,
	}
	for key, want := range wantCounts {
		if counts[key] != want {
			t.Fatalf("counts[%s] = %v, want %v", key, counts[key], want)
		}
	}
}
