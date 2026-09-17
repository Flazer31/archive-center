package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestCompleteTurnMariaDBAuthorityStartsAtTurnOneAndAdvances(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	postCompleteTurn := func(body map[string]any) map[string]any {
		effectiveInput := "assembled final payload for " + body["user_input"].(string)
		body["client_meta"] = map[string]any{
			"effective_input_observation": map[string]any{
				"contract_version":      "effective_input_observation.v1",
				"status":                "verified",
				"capture_stage":         "before_request_return",
				"effective_input":       effectiveInput,
				"effective_input_hash":  prepareOR1CHash(effectiveInput),
				"hash_algorithm":        "or1c_utf16_djb2.v1",
				"payload_content_match": true,
			},
		}
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return resp
	}

	first := postCompleteTurn(map[string]any{
		"chat_session_id":   "sess-turn-index",
		"user_input":        "first user",
		"assistant_content": "first assistant",
	})
	if first["turn_index"] != float64(1) {
		t.Fatalf("first turn_index = %v, want 1", first["turn_index"])
	}
	requireBackendTimingStages(t, first, "preflight", "raw_and_audit_store")
	if len(fake.savedChatLogs) != 2 {
		t.Fatalf("after first turn savedChatLogs = %d, want 2", len(fake.savedChatLogs))
	}
	for _, log := range fake.savedChatLogs {
		if log.ChatSessionID != "sess-turn-index" || log.TurnIndex != 1 {
			t.Fatalf("first turn log mismatch: %#v", log)
		}
	}
	if len(fake.savedEffectiveInputs) != 1 || fake.savedEffectiveInputs[0].TurnIndex != 1 {
		t.Fatalf("first effective input mismatch: %#v", fake.savedEffectiveInputs)
	}

	fake.returnChatLogs = make([]store.ChatLog, 0, len(fake.savedChatLogs))
	for _, log := range fake.savedChatLogs {
		fake.returnChatLogs = append(fake.returnChatLogs, *log)
	}

	second := postCompleteTurn(map[string]any{
		"chat_session_id":   "sess-turn-index",
		"user_input":        "second user",
		"assistant_content": "second assistant",
	})
	if second["turn_index"] != float64(2) {
		t.Fatalf("second turn_index = %v, want 2", second["turn_index"])
	}
	if len(fake.savedChatLogs) != 4 {
		t.Fatalf("after second turn savedChatLogs = %d, want 4", len(fake.savedChatLogs))
	}
	wantTurns := []int{1, 1, 2, 2}
	wantRoles := []string{"user", "assistant", "user", "assistant"}
	for i, log := range fake.savedChatLogs {
		if log.ChatSessionID != "sess-turn-index" || log.TurnIndex != wantTurns[i] || log.Role != wantRoles[i] {
			t.Fatalf("chat log[%d] mismatch: %#v", i, log)
		}
	}
	if len(fake.savedEffectiveInputs) != 2 {
		t.Fatalf("savedEffectiveInputs = %d, want 2", len(fake.savedEffectiveInputs))
	}
	if fake.savedEffectiveInputs[0].TurnIndex != 1 || fake.savedEffectiveInputs[1].TurnIndex != 2 {
		t.Fatalf("effective input turn indexes = %#v, want 1 then 2", fake.savedEffectiveInputs)
	}
}

func TestCompleteTurnExplicitStaleTurnIndexWithoutPreserveDoesNotAutoAdvance(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-stale-explicit", TurnIndex: 2, Role: "user", Content: "previous user"},
			{ID: 2, ChatSessionID: "sess-stale-explicit", TurnIndex: 2, Role: "assistant", Content: "previous assistant"},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"sess-stale-explicit","turn_index":2,"user_input":"new user","assistant_content":"new assistant","client_meta":{"critic":{"api_key":"","endpoint":"","model":""}}}`
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["turn_index"] != float64(2) || resp["chat_logs_saved"] != float64(0) || resp["derived_artifacts_saved"] != float64(0) {
		t.Fatalf("stale explicit turn must fail closed without auto-advance: %+v", resp)
	}
	if len(fake.savedChatLogs) != 0 || len(fake.savedMemories) != 0 || len(fake.savedEvidence) != 0 || len(fake.savedKGTriples) != 0 {
		t.Fatalf("stale explicit turn must not save duplicate rows, logs=%d memories=%d evidence=%d kg=%d", len(fake.savedChatLogs), len(fake.savedMemories), len(fake.savedEvidence), len(fake.savedKGTriples))
	}
	failReasons, _ := resp["fail_reasons"].([]any)
	if len(failReasons) == 0 || failReasons[0] != "raw_turn_content_conflict" {
		t.Fatalf("fail_reasons = %#v, want raw_turn_content_conflict", resp["fail_reasons"])
	}
}

func TestCompleteTurnCriticWaitsForAssistantOutput(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	called := 0
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		called++
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"{}"}}]}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := map[string]any{
		"chat_session_id": "sess-critic-waits",
		"turn_index":      1,
		"user_input":      "user typed, but assistant has not answered yet",
		"client_meta":     map[string]any{"critic": map[string]any{"api_key": "sk-test", "endpoint": "https://api.example.com/v1", "model": "critic-model", "provider": "openai", "timeout_ms": 45000}},
		"request_type":    "model",
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if called != 0 {
		t.Fatalf("critic HTTP calls = %d, want 0 before assistant output", called)
	}
	if resp["critic_triggered"] != false {
		t.Fatalf("critic_triggered = %v, want false before assistant output", resp["critic_triggered"])
	}
	failReasons, _ := resp["fail_reasons"].([]any)
	if len(failReasons) != 1 || failReasons[0] != "critic_skipped: assistant_content_missing" {
		t.Fatalf("fail_reasons = %#v, want assistant_content_missing skip", failReasons)
	}
	if len(fake.savedChatLogs) != 2 || fake.savedChatLogs[0].TurnIndex != 1 || fake.savedChatLogs[1].TurnIndex != 1 {
		t.Fatalf("chat logs should still save turn 1, got %#v", fake.savedChatLogs)
	}
	if len(fake.savedMemories) != 0 || len(fake.savedEvidence) != 0 || len(fake.savedKGTriples) != 0 {
		t.Fatalf("critic artifacts should not save without assistant output, memories=%d evidence=%d kg=%d", len(fake.savedMemories), len(fake.savedEvidence), len(fake.savedKGTriples))
	}
}

func TestCompleteTurnRejectsAssistantOnlyWithoutUserInput(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := map[string]any{
		"chat_session_id":   "sess-assistant-only",
		"turn_index":        3,
		"user_input":        "",
		"assistant_content": "assistant text arrived without the matching user input",
		"request_type":      "model",
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "error" || resp["save_ok"] != false {
		t.Fatalf("assistant-only complete-turn should be rejected, resp=%+v", resp)
	}
	failReasons, _ := resp["fail_reasons"].([]any)
	if len(failReasons) != 1 || failReasons[0] != "user_input_missing" {
		t.Fatalf("fail_reasons = %#v, want user_input_missing", resp["fail_reasons"])
	}
	if len(fake.savedChatLogs) != 0 || len(fake.savedEffectiveInputs) != 0 || len(fake.savedMemories) != 0 || len(fake.savedEvidence) != 0 || len(fake.savedKGTriples) != 0 {
		t.Fatalf("assistant-only turn must not write artifacts, logs=%d effective=%d memories=%d evidence=%d kg=%d",
			len(fake.savedChatLogs), len(fake.savedEffectiveInputs), len(fake.savedMemories), len(fake.savedEvidence), len(fake.savedKGTriples))
	}
}

func TestCompleteTurnAllowsExplicitAutoContinueEmptyInput(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := map[string]any{
		"chat_session_id":   "sess-auto-continue",
		"turn_index":        2,
		"user_input":        "",
		"assistant_content": "assistant continued the current scene from context",
		"request_type":      "model",
		"client_meta": map[string]any{
			"actual_empty_user_input": true,
			"logical_user_turn_key":   "[auto-continue]",
			"user_input_kind":         "auto_continue",
		},
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "ok" || resp["save_ok"] != true {
		t.Fatalf("explicit auto-continue empty input should save, resp=%+v", resp)
	}
	if len(fake.savedChatLogs) != 2 {
		t.Fatalf("chat logs saved = %d, want user+assistant rows", len(fake.savedChatLogs))
	}
	if fake.savedChatLogs[0].Role != "user" || fake.savedChatLogs[0].Content != "[auto-continue]" {
		t.Fatalf("auto-continue user row = %#v, want [auto-continue]", fake.savedChatLogs[0])
	}
	failReasons, _ := resp["fail_reasons"].([]any)
	for _, reason := range failReasons {
		if reason == "user_input_missing" {
			t.Fatalf("explicit auto-continue must not report user_input_missing: %#v", failReasons)
		}
	}
}

func TestCompleteTurnSanitizesThoughtTagsBeforeSave(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := map[string]any{
		"chat_session_id":   "sess-sanitize",
		"turn_index":        1,
		"user_input":        "Visible user text. <thinking>hidden user chain</thinking> Still visible.",
		"assistant_content": "Visible assistant text. <filter>hidden assistant trace",
		"request_type":      "model",
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedChatLogs) != 2 {
		t.Fatalf("savedChatLogs count = %d, want 2", len(fake.savedChatLogs))
	}
	for _, log := range fake.savedChatLogs {
		lower := strings.ToLower(log.Content)
		for _, blocked := range []string{"hidden", "thinking", "filter"} {
			if strings.Contains(lower, blocked) {
				t.Fatalf("saved chat log leaked %q in %#v", blocked, log)
			}
		}
	}
	if len(fake.savedEffectiveInputs) != 0 {
		t.Fatalf("complete-turn fabricated effective input without a verified payload observation: %#v", fake.savedEffectiveInputs)
	}
}

func TestCompleteTurnWithCriticConfigWritesExtractedArtifacts(t *testing.T) {
	fake := &turnRecordingStore{
		returnCharStates: []store.CharacterState{{
			ChatSessionID:     "sess-live",
			CharacterName:     "Alice",
			RelationshipsJSON: `{"Carol":{"affection":20}}`,
		}},
		returnEvidence: []store.DirectEvidence{{EvidenceKind: "turn_excerpt", EvidenceText: "Alice previously accepted Bob's help.", SourceTurnStart: 1, SourceTurnEnd: 1, TurnAnchor: 1}},
		returnChatLogs: []store.ChatLog{
			{ChatSessionID: "sess-live", TurnIndex: 1, Role: "user", Content: "Alice hesitated before trusting Bob."},
			{ChatSessionID: "sess-live", TurnIndex: 1, Role: "assistant", Content: "Bob helped Alice escape."},
		},
	}
	vec := &turnRecordingVectorStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.Readiness.ChromaConfigured = true
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil
	srv.Vector = vec
	srv.VectorOpenError = nil

	extraction := map[string]any{
		"turn_summary":        "Alice decided to trust Bob after the rescue.",
		"importance_score":    8,
		"relationship_memory": map[string]any{"bond_and_distance": "Alice trusts Bob more after he helped her.", "trust": 0.8},
		"entities": map[string]any{"characters": []any{map[string]any{
			"name": "Alice", "aliases": []any{"I"}, "role": "protagonist", "status_emotion": "relieved",
			"reference_contract": "critic_entity_reference.v1", "reference_scope": "session_stable", "name_expression": "Alice", "evidence_excerpt": "Alice relaxed after Bob helped her.",
		}}},
		"kg_triples": []any{},
		"relationship_observations": []any{map[string]any{
			"source_entity": "Alice", "source_entity_expression": "I", "target_entity": "Bob", "target_entity_expression": "Bob",
			"domain": "trust", "domain_expression": "trust", "observation": "I trust Bob", "support_kind": "explicit_statement", "evidence_excerpt": "I trust Bob.",
		}},
		"archive_hint":           map[string]any{"wing": "wing_general", "room": "hall_relationships"},
		"evidence_excerpts":      []any{"I trust Bob."},
		"emotional_intensity":    0.7,
		"narrative_significance": 0.9,
		"state_deltas":           map[string]any{"scene_state": map[string]any{"mood": "warm"}},
		"character_deltas": []any{map[string]any{
			"name":               "Alice",
			"reference_contract": "critic_entity_reference.v1",
			"reference_scope":    "session_stable",
			"name_expression":    "Alice",
			"evidence_excerpt":   "Alice relaxed after Bob helped her.",
			"status":             map[string]any{"emotion": "relieved"},
			"relationships":      map[string]any{"Bob": map[string]any{"affection": 70, "tension": 15}},
			"events":             []any{map[string]any{"type": "relationship_shift", "detail": "Alice's trust in Bob increased."}},
		}},
		"pending_threads": []any{
			map[string]any{"thread_type": "promise", "title": "Alice thanks Bob later", "confidence": 0.85},
			map[string]any{"thread_type": "misc", "title": "Invalid hook should be skipped", "confidence": 0.9},
			map[string]any{"thread_type": "open_question", "title": "Too weak to keep", "confidence": 0.2},
		},
		"world_rules": []any{map[string]any{"scope": "session", "category": "relationship", "key": "trust_changes_need_evidence", "value": "Trust shifts should be grounded in visible actions."}},
	}
	extractionBytes := []byte(criticWireJSONForTest(extraction))
	chatResp, _ := json.Marshal(map[string]any{
		"model":   "critic-model",
		"choices": []any{map[string]any{"message": map[string]any{"content": string(extractionBytes)}}},
	})
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/embeddings") {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"model":"embed-model","data":[{"embedding":[0.1,0.2,0.3]}]}`)),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(chatResp)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := map[string]any{
		"chat_session_id":   "sess-live",
		"turn_index":        2,
		"user_input":        "I trust Bob.",
		"assistant_content": "Alice relaxed after Bob helped her.",
		"context_messages":  []map[string]any{{"role": "user", "content": "Alice hesitated before trusting Bob."}, {"role": "assistant", "content": "Bob helped Alice escape."}},
		"improvement_trace": map[string]any{"score": 9},
		"client_meta":       map[string]any{"critic": map[string]any{"api_key": "sk-test", "endpoint": "https://api.example.com/v1", "model": "critic-model", "provider": "openai", "max_tokens": 1200, "timeout_ms": 45000}, "embedding": map[string]any{"api_key": "sk-test", "endpoint": "https://api.example.com/v1", "model": "embed-model", "provider": "openai", "timeout_ms": 30000}},
		"request_type":      "model",
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["critic_triggered"] != true {
		t.Fatalf("critic_triggered = %v, want true", resp["critic_triggered"])
	}
	wantCounts := map[string]float64{
		"memories_saved":         1,
		"evidence_saved":         1,
		"kg_triples_saved":       0,
		"entities_saved":         1,
		"trust_states_saved":     0,
		"world_rules_saved":      1,
		"storylines_saved":       3,
		"character_states_saved": 1,
		"character_events_saved": 0,
		"pending_threads_saved":  3,
		"active_states_saved":    7,
	}
	for key, want := range wantCounts {
		if resp[key] != want {
			t.Fatalf("%s = %v, want %.0f", key, resp[key], want)
		}
	}
	if len(fake.savedMemories) != 1 || fake.savedMemories[0].EmbeddingModel == "fake" {
		t.Fatalf("expected one non-fake memory, got %#v", fake.savedMemories)
	}
	if fake.savedMemories[0].Importance != 0.9 || fake.savedMemories[0].EmotionalBoost != 1.0 || fake.savedMemories[0].EmotionalIntensity != 0.7 || fake.savedMemories[0].NarrativeSignificance != 0.9 {
		t.Fatalf("expected emotional boost memory fields, got %#v", fake.savedMemories[0])
	}
	if len(fake.savedEvidence) != 1 || fake.savedEvidence[0].EvidenceText != "I trust Bob." {
		t.Fatalf("expected excerpt evidence only, got %#v", fake.savedEvidence)
	}
	if ev := fake.savedEvidence[0]; ev.SourceTurnStart != 2 || ev.SourceTurnEnd != 2 || ev.TurnAnchor != 2 || !strings.Contains(ev.SourceMessageIDsJSON, "turn:2") || !strings.Contains(ev.LineageJSON, "critic.evidence_excerpts") {
		t.Fatalf("expected evidence source lineage for turn 2, got %#v", ev)
	}
	if len(fake.savedKGTriples) != 0 {
		t.Fatalf("directional relationship must not enter generic KG, got %#v", fake.savedKGTriples)
	}
	if len(fake.savedEntities) != 1 || fake.savedEntities[0].Name != "Alice" {
		t.Fatalf("expected extracted entity, got %#v", fake.savedEntities)
	}
	if len(fake.savedTrusts) != 0 {
		t.Fatalf("legacy directionless trust state was persisted: %#v", fake.savedTrusts)
	}
	if len(fake.savedCharacterEvents) != 0 || len(fake.savedCharacterStates) != 1 || len(fake.savedPendingThreads) != 3 || len(fake.savedActiveStates) != 7 {
		t.Fatalf("expected character/state/thread artifacts, events=%d states=%d threads=%d active=%d", len(fake.savedCharacterEvents), len(fake.savedCharacterStates), len(fake.savedPendingThreads), len(fake.savedActiveStates))
	}
	if rel := fake.savedCharacterStates[0].RelationshipsJSON; strings.Contains(rel, "Bob") || !strings.Contains(rel, "Carol") {
		t.Fatalf("legacy Critic relation was added or existing canonical relationship was lost, got %s", rel)
	}
	if len(fake.savedWorldRules) != 1 || len(fake.savedStorylines) != 3 {
		t.Fatalf("expected world/story artifacts, world=%d story=%d", len(fake.savedWorldRules), len(fake.savedStorylines))
	}
	if wr := fake.savedWorldRules[0]; wr.Scope != "session" || wr.Category != "relationship" || wr.Key != "trust_changes_need_evidence" || !strings.Contains(wr.ValueJSON, "Trust shifts") {
		t.Fatalf("expected normalized world rule fields, got %#v", wr)
	}
	foundPromise := false
	for _, sl := range fake.savedStorylines {
		if sl.Name == "Alice thanks Bob later" && sl.Status == "active" && strings.Contains(sl.KeyPointsJSON, "Alice thanks Bob later") && strings.Contains(sl.OngoingTensionsJSON, "promise") {
			foundPromise = true
			break
		}
	}
	if !foundPromise {
		t.Fatalf("expected normalized promise storyline among broad candidates, got %#v", fake.savedStorylines)
	}
	if len(vec.docs) != 3 {
		t.Fatalf("public relationship evidence was not retained for general recall; got %#v", vec.docs)
	}
	tiers := map[string]bool{}
	for _, doc := range vec.docs {
		tiers[doc.Tier] = true
		if doc.ChatSessionID != "sess-live" || len(doc.Embedding) != 3 {
			t.Fatalf("unexpected vector doc: %#v", doc)
		}
	}
	if !tiers["memory"] || !tiers["evidence"] || !tiers["world_rule"] {
		t.Fatalf("expected public memory, evidence, and world-rule vectors, got %#v", vec.docs)
	}
	trace := resp["trace_handoff"].(map[string]any)
	if trace["vector_status"] != "ok" || resp["vectors_upserted"] != float64(3) || resp["vectors_evidence_upserted"] != float64(1) || resp["vectors_world_rule_upserted"] != float64(1) {
		t.Fatalf("vector status/count mismatch: trace=%+v resp=%+v", trace, resp)
	}
	if resp["maintenance_enqueued"] != false || resp["maintenance_audit_recorded"] != true {
		t.Fatalf("maintenance audit/queue truth mismatch: enqueued=%v audit=%v", resp["maintenance_enqueued"], resp["maintenance_audit_recorded"])
	}
	if trace["critic_pipeline_version"] != completeTurnCriticPipelineVersion || trace["critic_pipeline_split_enabled"] != true || trace["critic_pipeline_all_in_single_call"] != false {
		t.Fatalf("critic pipeline handoff mismatch: %+v", trace)
	}
	if trace["direct_evidence_retention_policy_version"] != "ea1l.v1" {
		t.Fatalf("direct evidence retention handoff mismatch: %+v", trace)
	}
	for _, key := range []string{"critic_preview_pass_version", "critic_preview_pass_enabled", "critic_preview_pass_scope", "critic_preview_compaction_mode"} {
		if _, ok := trace[key]; ok {
			t.Fatalf("removed critic preview handoff %q remained: %+v", key, trace)
		}
	}
	criticTrace, ok := trace["critic_trace"].(map[string]any)
	if !ok {
		t.Fatalf("critic_trace missing: %+v", trace)
	}
	pipeline, ok := criticTrace["pipeline"].(map[string]any)
	if !ok || pipeline["policy_version"] != completeTurnCriticPipelineVersion {
		t.Fatalf("critic pipeline trace missing: %+v", criticTrace)
	}
	stages, ok := pipeline["stages"].(map[string]any)
	if !ok || stages["evidence_extractor"] == nil || stages["deterministic_reducer"] == nil || stages["summary_compactor_background"] == nil {
		t.Fatalf("critic split stages missing: %+v", pipeline)
	}
	if _, ok := criticTrace["preview_pass"]; ok {
		t.Fatalf("removed preview_pass remained in critic trace: %+v", criticTrace)
	}
	if trace["maintenance_queue_status"] != "audit_recorded" || trace["maintenance_queue_depth"] != float64(0) {
		t.Fatalf("maintenance handoff mismatch: %+v", trace)
	}
	maintenance, ok := trace["maintenance_handoff"].(map[string]any)
	if !ok || maintenance["owner"] != "complete_turn" || maintenance["worker_enabled"] != false {
		t.Fatalf("maintenance_handoff owner/worker flag mismatch: %#v", trace["maintenance_handoff"])
	}
	foundMaintenanceAudit := false
	for _, item := range fake.savedAuditLogs {
		if item.EventType == "maintenance_audit_recorded" && item.TargetID == 2 {
			foundMaintenanceAudit = true
			break
		}
	}
	if !foundMaintenanceAudit {
		t.Fatalf("expected maintenance_audit_recorded audit log, got %#v", fake.savedAuditLogs)
	}
}

func TestCompleteTurnEpisodeCheckpointGeneratesAtIntervalBoundary(t *testing.T) {
	fake := &adminRegeneratedArtifactStore{
		adminEpisodeBackfillStore: &adminEpisodeBackfillStore{
			turnRecordingStore: &turnRecordingStore{
				returnChatLogs: []store.ChatLog{
					{ChatSessionID: "sess-live-episode", TurnIndex: 1, Role: "user", Content: "Luka draws the bridge plan."},
					{ChatSessionID: "sess-live-episode", TurnIndex: 1, Role: "assistant", Content: "Hank studies the marked routes."},
					{ChatSessionID: "sess-live-episode", TurnIndex: 2, Role: "user", Content: "Wren checks the oxygen tanks."},
					{ChatSessionID: "sess-live-episode", TurnIndex: 2, Role: "assistant", Content: "The supply crew marks the tanks."},
					{ChatSessionID: "sess-live-episode", TurnIndex: 3, Role: "user", Content: "Luka confirms the convoy route."},
					{ChatSessionID: "sess-live-episode", TurnIndex: 3, Role: "assistant", Content: "Hank approves the north approach."},
					{ChatSessionID: "sess-live-episode", TurnIndex: 4, Role: "user", Content: "The demolition team prepares the first charge."},
					{ChatSessionID: "sess-live-episode", TurnIndex: 4, Role: "assistant", Content: "The bridge plan is ready for final timing."},
				},
			},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	extractionBytes := []byte(criticWireJSONForTest(map[string]any{
		"turn_summary":     "Wren loads oxygen tanks for Operation Ice Wedge.",
		"importance_score": 7,
		"evidence_excerpts": []any{
			"Wren loads oxygen tanks",
		},
	}))
	chatResp, _ := json.Marshal(map[string]any{
		"model":   "critic-model",
		"choices": []any{map[string]any{"message": map[string]any{"content": string(extractionBytes)}}},
	})
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(chatResp)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := map[string]any{
		"chat_session_id":   "sess-live-episode",
		"turn_index":        5,
		"user_input":        "Wren starts loading oxygen tanks.",
		"assistant_content": "Wren loads oxygen tanks while Hank confirms Operation Ice Wedge.",
		"request_type":      "model",
		"client_meta": map[string]any{
			"episode_interval_turns": 5,
			"critic": map[string]any{
				"api_key":    "sk-test",
				"endpoint":   "https://api.example.com/v1",
				"model":      "critic-model",
				"provider":   "openai",
				"timeout_ms": 45000,
			},
		},
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedEpisodes) != 1 {
		t.Fatalf("saved episodes = %d, want 1: %#v", len(fake.savedEpisodes), fake.savedEpisodes)
	}
	if !strings.Contains(fake.savedEpisodes[0].SummaryText, "Operation Ice Wedge") {
		t.Fatalf("episode did not use checkpoint turn memory: %q", fake.savedEpisodes[0].SummaryText)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	episodeResult := resp["episode_result"].(map[string]any)
	if episodeResult["triggered"] != true || episodeResult["generated"] != float64(1) {
		t.Fatalf("episode_result mismatch: %+v", episodeResult)
	}
}

func TestCompleteTurnBlocksLegacyCharacterRelationshipAccumulation(t *testing.T) {
	const sid = "sess-rel-accumulate"
	fake := newRelationshipAccumulatingTurnStore([]store.CharacterState{{
		ChatSessionID:     sid,
		CharacterName:     "Alice",
		RelationshipsJSON: `{"Bob":{"affection":10,"tension":60,"last_change":"uneasy truce"}}`,
		TurnIndex:         1,
	}})
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	extractions := []map[string]any{
		{
			"turn_summary":     "Alice accepts Bob's help but remains guarded.",
			"importance_score": 6,
			"character_deltas": []any{map[string]any{
				"name":          "Alice",
				"relationships": map[string]any{"Bob": map[string]any{"affection": 25, "tension": 50}},
				"events":        []any{map[string]any{"type": "relationship_shift", "detail": "Alice accepts Bob's help."}},
			}},
		},
		{
			"turn_summary":     "Alice warms to Bob after he keeps watch.",
			"importance_score": 6,
			"character_deltas": []any{map[string]any{
				"name":          "Alice",
				"relationships": map[string]any{"Bob": map[string]any{"affection": 35}},
				"events":        []any{map[string]any{"type": "relationship_shift", "detail": "Alice warms to Bob."}},
			}},
		},
		{
			"turn_summary":     "Alice accepts Bob's apology and the tension drops.",
			"importance_score": 7,
			"character_deltas": []any{map[string]any{
				"name":          "Alice",
				"relationships": map[string]any{"Bob": map[string]any{"tension": 12, "last_change": "Alice accepted Bob's apology."}},
				"events":        []any{map[string]any{"type": "relationship_shift", "detail": "Alice accepted Bob's apology."}},
			}},
		},
	}
	criticCall := 0
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if criticCall >= len(extractions) {
			t.Fatalf("unexpected extra critic call %d", criticCall+1)
		}
		extractionBytes := []byte(criticWireJSONForTest(extractions[criticCall]))
		criticCall++
		resp, _ := json.Marshal(map[string]any{
			"model":   "critic-model",
			"choices": []any{map[string]any{"message": map[string]any{"content": string(extractionBytes)}}},
		})
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(resp)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	for i := range extractions {
		turnIndex := i + 2
		body := map[string]any{
			"chat_session_id":   sid,
			"turn_index":        turnIndex,
			"user_input":        fmt.Sprintf("relationship check turn %d", turnIndex),
			"assistant_content": fmt.Sprintf("Alice and Bob relationship beat %d.", turnIndex),
			"client_meta": map[string]any{
				"critic": map[string]any{
					"api_key":    "sk-critic-test",
					"endpoint":   "https://api.example.com/v1",
					"model":      "critic-model",
					"provider":   "openai",
					"timeout_ms": 45000,
				},
			},
		}
		raw, _ := json.Marshal(body)
		req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("turn %d status = %d, want 200: %s", turnIndex, rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("turn %d decode response: %v", turnIndex, err)
		}
		if resp["critic_triggered"] != true || resp["character_states_saved"] != float64(1) || resp["character_events_saved"] != float64(0) {
			t.Fatalf("turn %d did not block legacy relationship event: %+v", turnIndex, resp)
		}
	}
	if criticCall != 3 || len(fake.savedCharacterStates) != 3 || len(fake.savedCharacterEvents) != 0 {
		t.Fatalf("expected 3 critic/state/event calls, critic=%d states=%d events=%d", criticCall, len(fake.savedCharacterStates), len(fake.savedCharacterEvents))
	}

	finalState, err := fake.GetCharacterState(context.Background(), sid, "Alice")
	if err != nil {
		t.Fatalf("GetCharacterState final: %v", err)
	}
	if finalState.TurnIndex != 4 {
		t.Fatalf("final turn_index = %d, want 4", finalState.TurnIndex)
	}
	var relationships map[string]any
	if err := json.Unmarshal([]byte(finalState.RelationshipsJSON), &relationships); err != nil {
		t.Fatalf("decode relationships_json %q: %v", finalState.RelationshipsJSON, err)
	}
	if _, ok := relationships["affection"]; ok {
		t.Fatalf("relationship fields leaked to top level instead of target key: %+v", relationships)
	}
	bob, ok := relationships["Bob"].(map[string]any)
	if !ok {
		t.Fatalf("missing Bob relationship target in %+v", relationships)
	}
	if got := extractionFloatFromAny(bob["affection"], 0); got != 10 {
		t.Fatalf("Bob affection = %v, want original preserved 10 in %+v", got, bob)
	}
	if got := extractionFloatFromAny(bob["tension"], 0); got != 60 {
		t.Fatalf("Bob tension = %v, want original preserved 60 in %+v", got, bob)
	}
	if got := extractionStringFromAny(bob["last_change"]); got != "uneasy truce" {
		t.Fatalf("Bob last_change = %q, want original relationship state in %+v", got, bob)
	}
}

func TestCompleteTurnCriticGuardsEvidenceKGAndEntityTypes(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	userText := "Mina searches the old library."
	assistantText := "Mina found a brass key. Rowan promised to help her open the cellar."
	fullTurn := strings.TrimSpace(userText + "\n" + assistantText)
	extraction := map[string]any{
		"turn_summary":        "Mina found a brass key and Rowan offered help.",
		"importance_score":    7,
		"evidence_excerpts":   []any{fullTurn, "Mina found a brass key."},
		"relationship_memory": map[string]any{"bond_and_distance": "Mina trusts Rowan more after the promise.", "target_name": "Rowan", "trust": 0.6},
		"entities": map[string]any{
			"characters": []any{map[string]any{"name": "Mina", "role": "protagonist"}},
			"locations":  []any{map[string]any{"name": "old library", "description": "quiet archive room"}},
			"items":      []any{map[string]any{"name": "brass key", "description": "cellar key"}},
		},
		"kg_triples": []any{
			map[string]any{"subject": "char_59_cid_fb179fa9-3a73-496e-8df5-35c621338f9f", "predicate": "has_turn", "object": "turn_1"},
			testEntityScalarKG("item_fact", "Mina", "character", "found", "a brass key", "string", "Mina found a brass key."),
		},
		"world_rules": []any{map[string]any{"scope": "location", "scope_name": "old library", "category": "access", "key": "cellar_needs_key", "value": "The cellar can be opened with the brass key."}},
	}
	extractionBytes := []byte(criticWireJSONForTest(extraction))
	chatResp, _ := json.Marshal(map[string]any{
		"model":   "critic-model",
		"choices": []any{map[string]any{"message": map[string]any{"content": string(extractionBytes)}}},
	})
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(chatResp)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := map[string]any{
		"chat_session_id":   "sess-guard",
		"turn_index":        1,
		"user_input":        userText,
		"assistant_content": assistantText,
		"request_type":      "model",
		"client_meta":       map[string]any{"critic": map[string]any{"api_key": "sk-test", "endpoint": "https://api.example.com/v1", "model": "critic-model", "provider": "openai", "timeout_ms": 45000}},
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	if len(fake.savedEvidence) != 1 || fake.savedEvidence[0].EvidenceText != "Mina found a brass key." {
		t.Fatalf("expected only grounded short evidence, got %#v", fake.savedEvidence)
	}
	if fake.savedEvidence[0].ArchiveState != "verified_direct" || fake.savedEvidence[0].CaptureVerification != "verified" || fake.savedEvidence[0].CommittedGate != "auto_grounded_excerpt" {
		t.Fatalf("grounded direct evidence was not auto-verified: %#v", fake.savedEvidence[0])
	}
	if len(fake.savedKGTriples) != 2 {
		t.Fatalf("KG content was filtered before storage: %#v", fake.savedKGTriples)
	}
	if len(fake.savedEntities) != 3 {
		t.Fatalf("expected character/location/item entities, got %#v", fake.savedEntities)
	}
	types := map[string]bool{}
	for _, item := range fake.savedEntities {
		types[item.EntityType] = true
	}
	for _, want := range []string{"protagonist", "location", "item"} {
		if !types[want] {
			t.Fatalf("missing entity type %q in %#v", want, fake.savedEntities)
		}
	}
	if len(fake.savedTrusts) != 0 {
		t.Fatalf("unsafe legacy trust state was persisted: %#v", fake.savedTrusts)
	}
	if len(fake.savedWorldRules) != 1 {
		t.Fatalf("expected world rule, got %#v", fake.savedWorldRules)
	}
}

func TestCompleteTurnLocationTimeGroundingSeparatesSceneResidenceAndSeason(t *testing.T) {
	t.Run("critic_prompt_names_location_time_lanes", func(t *testing.T) {
		prompt := combinedCriticPromptForTest(t, buildCompleteTurnCriticPrompt(
			"sess-loc-time", 8,
			"Rowan lives in London.",
			"The current scene stays on the school rooftop as summer vacation begins.",
			nil, nil,
		))
		for _, needle := range []string{
			"Location and time typed lanes do not suppress compatible kg_triples",
			"Do not treat 'X lives in London' as 'the current scene is London'",
			"Story calendar facts such as 'summer vacation has started'",
			"Do not infer an immediate return to school",
		} {
			if !strings.Contains(prompt, needle) {
				t.Fatalf("critic prompt missing location/time grounding marker %q", needle)
			}
		}
	})

	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	content := `Rowan told Mina, "I live in London, not at the academy." The current scene stayed on the school rooftop. Summer vacation had just begun, so classes were over.`
	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":     "Rowan's residence is London, while the current scene remains on the school rooftop as summer vacation begins.",
		"importance_score": 7,
		"evidence_excerpts": []any{
			"I live in London",
			"current scene stayed on the school rooftop",
			"Summer vacation had just begun",
		},
		"entities": map[string]any{
			"characters": []any{map[string]any{"name": "Rowan"}, map[string]any{"name": "Mina"}},
			"locations":  []any{map[string]any{"name": "London"}, map[string]any{"name": "school rooftop"}},
		},
		"kg_triples": []any{
			map[string]any{"semantic_class": "location_fact", "subject": "Rowan", "predicate": "residence", "object": "London", "valid_from": 8},
		},
		"character_deltas": []any{map[string]any{
			"name":   "Rowan",
			"status": map[string]any{"residence": "London"},
		}},
		"state_deltas": map[string]any{
			"scene_state":  map[string]any{"location": "school rooftop", "time_state": "summer_vacation_started", "school_status": "classes_over"},
			"confidence":   0.86,
			"verification": "verified",
		},
		"world_rules": []any{map[string]any{
			"scope":      "session",
			"category":   "time",
			"key":        "summer_vacation_started",
			"value":      "Summer vacation has started; classes are over until direct evidence says otherwise.",
			"confidence": 0.85,
		}},
		"world_state": map[string]any{
			"time_state":    "summer_vacation_started",
			"season":        "summer",
			"school_status": "classes_over",
			"confidence":    0.85,
			"verification":  "verified",
		},
	})

	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-loc-time", 8, extraction, content, completeTurnEmbeddingConfig{}, time.Unix(800, 0))
	if result.Errors != 0 {
		t.Fatalf("saveCriticExtractionArtifacts errors=%d details=%#v", result.Errors, result.ErrorDetails)
	}
	if result.Evidence != 3 || len(fake.savedEvidence) != 3 {
		t.Fatalf("expected 3 grounded evidence excerpts, got result=%d saved=%d skip=%#v", result.Evidence, len(fake.savedEvidence), result.SkipReasons)
	}
	if len(fake.savedKGTriples) != 1 {
		t.Fatalf("expected one residence KG triple, got %#v", fake.savedKGTriples)
	}
	kg := fake.savedKGTriples[0]
	if kg.Subject != "Rowan" || kg.Predicate != "residence" || kg.Object != "London" || kg.ValidFrom != 8 || kg.SourceTurn != 8 {
		t.Fatalf("residence KG triple mismatch: %#v", kg)
	}
	if len(fake.savedCharacterStates) != 1 {
		t.Fatalf("expected one character state, got %#v", fake.savedCharacterStates)
	}
	statusJSON := fake.savedCharacterStates[0].StatusJSON
	if !strings.Contains(statusJSON, "London") || !strings.Contains(statusJSON, "residence") {
		t.Fatalf("character status should carry durable residence, got %s", statusJSON)
	}
	if strings.Contains(statusJSON, "school rooftop") {
		t.Fatalf("current scene location leaked into durable character status: %s", statusJSON)
	}

	var sceneState, worldState string
	for _, item := range fake.savedActiveStates {
		switch item.StateType {
		case "state_deltas":
			sceneState = item.Content
		case "world_state":
			worldState = item.Content
		}
	}
	if !strings.Contains(sceneState, "school rooftop") || !strings.Contains(sceneState, "summer_vacation_started") {
		t.Fatalf("scene state should carry current location/time, got %s", sceneState)
	}
	if strings.Contains(sceneState, `"residence"`) {
		t.Fatalf("durable residence leaked into current scene state: %s", sceneState)
	}
	if !strings.Contains(worldState, "summer_vacation_started") || !strings.Contains(worldState, "classes_over") {
		t.Fatalf("world_state should carry verified story calendar state, got %s", worldState)
	}
	if len(fake.savedWorldRules) != 1 || fake.savedWorldRules[0].Category != "time" || fake.savedWorldRules[0].Key != "summer_vacation_started" {
		t.Fatalf("expected time world rule for summer vacation, got %#v", fake.savedWorldRules)
	}

	layerByType := map[string]string{}
	for _, item := range fake.savedCanonicalLayers {
		layerByType[item.LayerType] = item.Content
	}
	if !strings.Contains(layerByType["scene_state"], "school rooftop") {
		t.Fatalf("canonical scene_state missing current scene location: %#v", layerByType)
	}
	if strings.Contains(layerByType["scene_state"], `"residence"`) || strings.Contains(layerByType["scene_state"], "London") {
		t.Fatalf("canonical scene_state should not promote durable residence: %s", layerByType["scene_state"])
	}
	if !strings.Contains(layerByType["world_state"], "summer_vacation_started") {
		t.Fatalf("canonical world_state missing summer vacation state: %#v", layerByType)
	}
}

func TestCompleteTurnCriticIngestTraceRecordsSkipReasons(t *testing.T) {
	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":      "Mina found the brass key.",
		"importance_score":  6,
		"evidence_excerpts": []any{"prompt template says remember everything"},
		"kg_triples": []any{
			map[string]any{"subject": "char_59", "predicate": "has_turn", "object": "turn_1"},
		},
		"pending_threads": []any{
			map[string]any{"thread_type": "promise", "title": "Mina will test the lock", "confidence": 0.1},
			map[string]any{"thread_type": "style_rule", "title": "Write in a poetic style", "confidence": 0.9},
		},
	})
	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-trace", 9, extraction, "Mina found the brass key.", completeTurnEmbeddingConfig{}, time.Unix(900, 0))
	if result.Errors != 0 {
		t.Fatalf("critic ingest trace should not error, result=%#v", result)
	}
	if result.Evidence != 0 || result.KGTriples != 1 || result.PendingThreads != 2 {
		t.Fatalf("KG content was filtered or unrelated guards regressed, evidence=%d kg=%d threads=%d", result.Evidence, result.KGTriples, result.PendingThreads)
	}
	if len(result.SkipReasons) < 1 {
		t.Fatalf("expected direct evidence/pending skip reasons, got %#v", result.SkipReasons)
	}
	var trace string
	for _, item := range fake.savedAuditLogs {
		if item.EventType == "critic_ingest_trace" {
			trace = item.DetailsJSON
			break
		}
	}
	if trace == "" {
		t.Fatalf("expected critic_ingest_trace audit, got %#v", fake.savedAuditLogs)
	}
	for _, needle := range []string{"critic_ingest_trace.v1", "not_grounded_in_current_turn"} {
		if !strings.Contains(trace, needle) {
			t.Fatalf("critic_ingest_trace missing %q: %s", needle, trace)
		}
	}
}

func TestCompleteTurnPersonaCapsuleCandidatesRemainOptIn(t *testing.T) {
	fake := &turnRecordingStore{}
	srv := NewServer(config.Default())
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := normalizeCriticExtraction(map[string]any{
		"turn_summary":      "",
		"importance_score":  8,
		"evidence_excerpts": []any{},
		"persona_capsule_candidates": []any{
			map[string]any{
				"memory_text":       "The protagonist remembers dying before the loop reset.",
				"source_turn_index": 11,
				"importance_10":     9,
				"emotional_weight":  0.8,
				"portability":       "cross_world",
				"mode":              "full_loop_memory",
				"secret_guard":      "true",
				"tags":              []any{"loop", "protagonist_private"},
				"evidence_excerpt":  "I remember dying before everything reset.",
			},
		},
	})

	result := srv.saveCriticExtractionArtifacts(context.Background(), "sess-pmc5", 11, extraction, "I remember dying before everything reset.", completeTurnEmbeddingConfig{}, time.Unix(1100, 0))
	if result.PersonaCapsuleCandidates != 1 {
		t.Fatalf("PersonaCapsuleCandidates = %d, want 1", result.PersonaCapsuleCandidates)
	}
	if len(fake.createdPersonaCapsules) != 0 || len(fake.createdPersonaEntries) != 0 {
		t.Fatalf("persona capsule candidates must not auto-create capsules, capsules=%d entries=%d", len(fake.createdPersonaCapsules), len(fake.createdPersonaEntries))
	}
	if len(fake.savedMemories) != 0 || len(fake.savedEvidence) != 0 || len(fake.savedKGTriples) != 0 {
		t.Fatalf("persona capsule candidates must not auto-promote to canonical rows, memories=%d evidence=%d kg=%d", len(fake.savedMemories), len(fake.savedEvidence), len(fake.savedKGTriples))
	}
	warnings := strings.Join(result.Warnings, "\n")
	if !strings.Contains(warnings, "persona_capsule_candidates_detected:auto_create_disabled") {
		t.Fatalf("missing opt-in warning, got %#v", result.Warnings)
	}
	var foundSkip bool
	for _, skip := range result.SkipReasons {
		if skip["surface"] == "persona_capsule_candidates" && skip["reason"] == "requires_explicit_user_or_operator_approval" {
			foundSkip = true
			break
		}
	}
	if !foundSkip {
		t.Fatalf("missing persona capsule approval skip reason: %#v", result.SkipReasons)
	}
	var trace string
	for _, item := range fake.savedAuditLogs {
		if item.EventType == "critic_ingest_trace" {
			trace = item.DetailsJSON
			break
		}
	}
	for _, needle := range []string{"persona_capsule_candidates", "auto_create_disabled", "requires_explicit_user_or_operator_approval"} {
		if !strings.Contains(trace, needle) {
			t.Fatalf("critic ingest trace missing %q: %s", needle, trace)
		}
	}
}
