package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestCompleteTurnCriticProviderFailureRetriesWithRedactedInput(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	extractionBytes, _ := json.Marshal(map[string]any{
		"turn_summary":      "Mina and Rowan crossed an intimate threshold while Rowan stayed reassuring.",
		"importance_score":  7,
		"evidence_excerpts": []any{"Rowan stayed reassuring."},
		"kg_triples":        []any{map[string]any{"subject": "Rowan", "predicate": "reassures", "object": "Mina"}},
	})
	chatResp, _ := json.Marshal(map[string]any{
		"model":   "critic-model",
		"choices": []any{map[string]any{"message": map[string]any{"content": string(extractionBytes)}}},
	})
	oldClient := proxyHTTPClient
	callCount := 0
	secondPrompt := ""
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		callCount++
		var upstreamReq map[string]any
		if err := json.NewDecoder(r.Body).Decode(&upstreamReq); err != nil {
			t.Fatalf("decode upstream request: %v", err)
		}
		if callCount == 1 {
			return &http.Response{
				StatusCode: http.StatusInternalServerError,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"upstream rejected"}}`)),
			}, nil
		}
		msgs, _ := upstreamReq["messages"].([]any)
		if len(msgs) >= 2 {
			if msg, _ := msgs[1].(map[string]any); msg != nil {
				secondPrompt = fmt.Sprint(msg["content"])
			}
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
		"chat_session_id":   "sess-critic-redacted-retry",
		"turn_index":        1,
		"user_input":        "Mina asks Rowan to be gentle.",
		"assistant_content": "Rowan stayed reassuring. The intimate scene involved penetration.",
		"client_meta": map[string]any{"critic": map[string]any{
			"api_key":  "sk-redacted-retry",
			"endpoint": "https://api.example.com/v1",
			"model":    "critic-model",
			"provider": "openai",
		}},
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("complete-turn status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if callCount != 2 {
		t.Fatalf("critic call count = %d, want 2", callCount)
	}
	if strings.Contains(strings.ToLower(secondPrompt), "penetration") || !strings.Contains(secondPrompt, "redacted for critic retry") {
		t.Fatalf("second critic prompt was not redacted as expected: %s", secondPrompt)
	}
	if resp["critic_triggered"] != true {
		t.Fatalf("critic_triggered = %v, want true after redacted retry: %+v", resp["critic_triggered"], resp)
	}
	if resp["derived_artifacts_saved"].(float64) < 3 {
		t.Fatalf("derived_artifacts_saved = %v, want memory/evidence/KG after retry: %+v", resp["derived_artifacts_saved"], resp)
	}
	trace, _ := resp["trace_handoff"].(map[string]any)
	criticTrace, _ := trace["critic_trace"].(map[string]any)
	if _, ok := criticTrace["provider_retry"].(map[string]any); !ok {
		t.Fatalf("provider_retry trace missing: %+v", criticTrace)
	}
}

func TestCompleteTurnEmbeddingProviderFailureReportsWarning(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	cfg.ChromaEndpoint = "http://127.0.0.1:8000"
	cfg.Readiness.ChromaConfigured = true
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil
	srv.Vector = &turnRecordingVectorStore{}

	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "/embeddings") {
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"embedding unauthorized"}}`)),
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(`{
				"model":"critic-model",
				"choices":[{"message":{"content":"{\"turn_summary\":\"Mina and Rowan commit to the blue key.\",\"importance_score\":7,\"evidence_excerpts\":[\"blue key safe\"],\"kg_triples\":[{\"subject\":\"Rowan\",\"predicate\":\"protects\",\"object\":\"blue key\"}],\"entities\":{\"characters\":[{\"name\":\"Rowan\"}],\"items\":[{\"name\":\"blue key\"}]}}"}}]
			}`)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := map[string]any{
		"chat_session_id":   "sess-embedding-provider-fail",
		"turn_index":        1,
		"user_input":        "Mina asks Rowan to remember the blue key.",
		"assistant_content": "Rowan promises to keep the blue key safe.",
		"client_meta": map[string]any{
			"critic": map[string]any{
				"api_key":  "sk-critic-ok",
				"endpoint": "https://api.example.com/v1",
				"model":    "critic-model",
				"provider": "openai",
			},
			"embedding": map[string]any{
				"api_key":  "sk-embedding-fail",
				"endpoint": "https://api.example.com/v1/embeddings",
				"model":    "embedding-model",
				"provider": "openai",
			},
		},
	}
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("complete-turn status = %d, want 200 with embedding warning: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["critic_triggered"] != true || resp["memories_saved"] != float64(1) || resp["vectors_upserted"] != float64(0) {
		t.Fatalf("expected critic artifacts but no vector upsert, got %+v", resp)
	}
	trace, _ := resp["trace_handoff"].(map[string]any)
	if !strings.Contains(fmt.Sprint(trace["embedding_status"]), "error:") || !strings.Contains(fmt.Sprint(trace["vector_status"]), "error:") {
		t.Fatalf("embedding/vector failure was not visible in trace: %+v", trace)
	}
	warnings, _ := resp["warnings"].([]any)
	found := false
	for _, item := range warnings {
		if fmt.Sprint(item) == "embedding_call_failed" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected embedding_call_failed warning, got %+v", resp["warnings"])
	}
}

func TestCompleteTurnOOCGuardSkipsWrites(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-ooc","turn_index":1,"user_input":"OOC: please change the plugin setting","assistant_content":"Sure, I will help.","context_messages":[]}`
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
	if resp["save_error"] != "skipped_by_ooc_guard" || resp["critic_triggered"] != false {
		t.Fatalf("unexpected OOC response: %+v", resp)
	}
	if len(fake.savedChatLogs) != 0 || len(fake.savedMemories) != 0 || len(fake.savedEvidence) != 0 || len(fake.savedKGTriples) != 0 {
		t.Fatalf("OOC guard should skip all writes, logs=%d memories=%d evidence=%d kg=%d", len(fake.savedChatLogs), len(fake.savedMemories), len(fake.savedEvidence), len(fake.savedKGTriples))
	}
}

func TestCompleteTurnKoreanOOCGuardSkipsWrites(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-ooc-ko","turn_index":1,"user_input":"OOC: this is a setting change request, not story dialogue.","assistant_content":"Understood.","context_messages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedChatLogs) != 0 || len(fake.savedMemories) != 0 || len(fake.savedEvidence) != 0 || len(fake.savedKGTriples) != 0 {
		t.Fatalf("Korean OOC guard should skip all writes, logs=%d memories=%d evidence=%d kg=%d", len(fake.savedChatLogs), len(fake.savedMemories), len(fake.savedEvidence), len(fake.savedKGTriples))
	}
}

func TestCompleteTurnStructuredCanonicalContentDoesNotSkipDerivedIngest(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	calls := 0
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		extraction := `{"turn_summary":"The scene continues under the requested constraints.","importance_score":6}`
		response := `{"model":"critic-model","choices":[{"message":{"content":` + strconv.Quote(extraction) + `}}]}`
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(response))}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := map[string]any{
		"chat_session_id":   "sess-source-aware",
		"turn_index":        1,
		"user_input":        "[Narrative Guide]\nScene Mandate: keep the mood stable\nForbidden Moves:\n- sudden battle",
		"assistant_content": "Response Template\n{{char}} should answer in the requested style.",
		"client_meta":       map[string]any{"critic": map[string]any{"api_key": "sk-test", "endpoint": "https://api.example.com/v1", "model": "critic-model", "provider": "openai"}},
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
	if resp["critic_triggered"] != true || calls != 1 {
		t.Fatalf("structured canonical turn should reach critic, triggered=%v calls=%d resp=%+v", resp["critic_triggered"], calls, resp)
	}
	if len(fake.savedChatLogs) != 2 || len(fake.savedMemories) != 1 {
		t.Fatalf("structured canonical turn should save raw and derived memory, logs=%d memories=%d", len(fake.savedChatLogs), len(fake.savedMemories))
	}
}

func TestEffectiveInputsNoopModeHasTransparency(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-ei-noop","turn_index":2,"effective_input":"user refined intent here"}`
	req := httptest.NewRequest(http.MethodPost, "/effective-inputs", bytes.NewReader([]byte(body)))
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

	it, ok := resp["input_transparency"].(map[string]any)
	if !ok {
		t.Fatalf("input_transparency is not an object")
	}
	if it["status"] != "ready" {
		t.Errorf("input_transparency.status = %v, want ready", it["status"])
	}
	if it["store_write_enabled"] != false {
		t.Errorf("input_transparency.store_write_enabled = %v, want false", it["store_write_enabled"])
	}
	if it["would_write"] != false {
		t.Errorf("input_transparency.would_write = %v, want false", it["would_write"])
	}
	if it["effective_input_chars"] != float64(24) {
		t.Errorf("input_transparency.effective_input_chars = %v, want 24", it["effective_input_chars"])
	}
	preview, _ := it["preview"].(string)
	if !strings.Contains(preview, "refined intent") {
		t.Errorf("input_transparency.preview missing expected text: %q", preview)
	}
	notes, _ := it["notes"].(string)
	if !strings.Contains(notes, "R1 read-shadow") {
		t.Errorf("input_transparency.notes missing R1 marker: %q", notes)
	}
	if resp["save_ok"] != false {
		t.Errorf("save_ok = %v, want false", resp["save_ok"])
	}
}

func TestEffectiveInputsDualShadowHasTransparency(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-ei-dual","turn_index":5,"effective_input":"dual shadow text"}`
	req := httptest.NewRequest(http.MethodPost, "/effective-inputs", bytes.NewReader([]byte(body)))
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

	it, ok := resp["input_transparency"].(map[string]any)
	if !ok {
		t.Fatalf("input_transparency is not an object")
	}
	if it["status"] != "ready" {
		t.Errorf("input_transparency.status = %v, want ready", it["status"])
	}
	if it["store_write_enabled"] != true {
		t.Errorf("input_transparency.store_write_enabled = %v, want true", it["store_write_enabled"])
	}
	if it["would_write"] != true {
		t.Errorf("input_transparency.would_write = %v, want true", it["would_write"])
	}
	if it["effective_input_chars"] != float64(16) {
		t.Errorf("input_transparency.effective_input_chars = %v, want 16", it["effective_input_chars"])
	}
	preview, _ := it["preview"].(string)
	if !strings.Contains(preview, "dual shadow") {
		t.Errorf("input_transparency.preview missing expected text: %q", preview)
	}
	if resp["save_ok"] != true {
		t.Errorf("save_ok = %v, want true", resp["save_ok"])
	}
}

func TestPrepareTurnStoreBackedAssembly(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-prep", TurnIndex: 2, SummaryJSON: `{"turn_summary":"A warm evening in the garden"}`, Importance: 0.9},
			{ID: 2, ChatSessionID: "sess-prep", TurnIndex: 3, SummaryJSON: `{"turn_summary":"A confrontation at the gate"}`, Importance: 0.8},
		},
		returnKGTriples: []store.KGTriple{
			{ID: 10, ChatSessionID: "sess-prep", Subject: "Alice", Predicate: "knows", Object: "Bob"},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 20, ChatSessionID: "sess-prep", EvidenceText: "The letter was sealed with red wax.", EvidenceKind: "verbatim", SourceTurnStart: 5, SourceTurnEnd: 5, TurnAnchor: 5},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 100, ChatSessionID: "sess-prep", TurnIndex: 5, Role: "user", Content: "What happens next?"},
			{ID: 101, ChatSessionID: "sess-prep", TurnIndex: 5, Role: "assistant", Content: "The door creaks open."},
		},
		returnResumePack: &store.ResumePack{
			PackStatus:    "ready",
			AssembledText: "Session resume: Alice and Bob are investigating the old manor.",
		},
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "sess-prep", Name: "The Manor Mystery", CurrentContext: "Alice investigates the old manor"},
		},
		returnWorldRules: []store.WorldRule{
			{ID: 1, ChatSessionID: "sess-prep", Key: "magic_requires_blood", Scope: "session"},
		},
		returnCharStates: []store.CharacterState{
			{ID: 1, ChatSessionID: "sess-prep", CharacterName: "Alice", StatusJSON: `{"health":"injured","mood":"determined"}`},
		},
		returnPendingThreads: []store.PendingThread{
			{ID: 1, ChatSessionID: "sess-prep", Description: "Who sent the letter?"},
		},
		returnActiveStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "sess-prep", StateType: "location", Content: "Old manor library"},
		},
		returnCanonicalLayers: []store.CanonicalStateLayer{
			{ID: 1, ChatSessionID: "sess-prep", LayerType: "location", Content: "The library smells of old books"},
		},
		returnEpisodeSums: []store.EpisodeSummary{
			{ID: 1, ChatSessionID: "sess-prep", FromTurn: 1, ToTurn: 3, SummaryText: "Alice arrives at the manor and finds clues"},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-prep","turn_index":6,"raw_user_input":"Open the door","settings":{"max_injection_chars":500,"max_input_context_chars":400,"injection_enabled":true,"input_context_enabled":true,"top_k":2}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
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

	if resp["status"] != "ok" {
		t.Errorf("status = %v, want ok", resp["status"])
	}
	requireBackendTimingStages(t, resp, "store_reads", "vector_recall", "injection_assembly", "response_assembly")

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("generation_packet is not an object")
	}

	if gp["packet_mode"] != "store_backed_shadow" {
		t.Errorf("packet_mode = %v, want store_backed_shadow", gp["packet_mode"])
	}
	if gp["degraded"] != false {
		t.Errorf("degraded = %v, want false", gp["degraded"])
	}

	injection, _ := gp["injection_text"].(string)
	if injection == "" {
		t.Error("injection_text is empty")
	}
	if len(injection) > 500 {
		t.Errorf("injection_text length %d exceeds cap 500", len(injection))
	}

	ict, _ := resp["input_context_text"].(string)
	if ict == "" {
		t.Error("input_context_text is empty")
	}
	if len(ict) > 400 {
		t.Errorf("input_context_text length %d exceeds cap 400", len(ict))
	}

	trace, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("trace_summary is not an object")
	}

	if trace["reads_ok"] != float64(14) {
		t.Errorf("reads_ok = %v, want 14", trace["reads_ok"])
	}
	if trace["memory_count"] != float64(2) {
		t.Errorf("memory_count = %v, want 2", trace["memory_count"])
	}
	if trace["storyline_count"] != float64(1) {
		t.Errorf("storyline_count = %v, want 1", trace["storyline_count"])
	}
	if trace["world_rule_count"] != float64(1) {
		t.Errorf("world_rule_count = %v, want 1", trace["world_rule_count"])
	}
	if trace["character_state_count"] != float64(1) {
		t.Errorf("character_state_count = %v, want 1", trace["character_state_count"])
	}
	if trace["pending_thread_count"] != float64(1) {
		t.Errorf("pending_thread_count = %v, want 1", trace["pending_thread_count"])
	}
	if trace["active_state_count"] != float64(1) {
		t.Errorf("active_state_count = %v, want 1", trace["active_state_count"])
	}
	if trace["canonical_layer_count"] != float64(1) {
		t.Errorf("canonical_layer_count = %v, want 1", trace["canonical_layer_count"])
	}
	if trace["episode_summary_count"] != float64(1) {
		t.Errorf("episode_summary_count = %v, want 1", trace["episode_summary_count"])
	}
	if trace["scoped_verbatim_support_count"] != float64(1) {
		t.Errorf("scoped_verbatim_support_count = %v, want 1", trace["scoped_verbatim_support_count"])
	}
	verbatimSupport, ok := trace["verbatim_support"].(map[string]any)
	if !ok {
		t.Fatalf("verbatim_support is not an object")
	}
	if verbatimSupport["active"] != true {
		t.Errorf("verbatim_support.active = %v, want true", verbatimSupport["active"])
	}
	if verbatimSupport["policy_version"] != "vr18a.v1" {
		t.Errorf("verbatim_support.policy_version = %v, want vr18a.v1", verbatimSupport["policy_version"])
	}
	earlyInjectionPack, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}
	if earlyInjectionPack["scoped_verbatim_support_count"] != float64(1) {
		t.Errorf("injection_pack scoped count = %v, want 1", earlyInjectionPack["scoped_verbatim_support_count"])
	}
	if strings.Contains(injection, "Scoped Verbatim Recall") {
		t.Errorf("injection text should not inline support surface: %q", injection)
	}

	if !strings.Contains(injection, "[Memory]") {
		t.Error("injection_text missing [Memory] section")
	}
	if !strings.Contains(injection, "[Knowledge Graph]") {
		t.Error("injection_text missing [Knowledge Graph] section")
	}
	if !strings.Contains(injection, "[Storylines]") {
		t.Error("injection_text missing [Storylines] section")
	}
	if !strings.Contains(injection, "[World Rules]") {
		t.Error("injection_text missing [World Rules] section")
	}
	if !strings.Contains(injection, "[Characters]") {
		t.Error("injection_text missing [Characters] section")
	}
	if !strings.Contains(injection, "[Pending Threads]") {
		t.Error("injection_text missing [Pending Threads] section")
	}

	if !strings.Contains(ict, "[Resume Pack]") {
		t.Error("input_context_text missing [Resume Pack] section")
	}
	if !strings.Contains(ict, "[Direct Evidence]") {
		t.Error("input_context_text missing [Direct Evidence] section")
	}
	if !strings.Contains(ict, "[Recent Chat]") {
		t.Error("input_context_text missing [Recent Chat] section")
	}
	if !strings.Contains(ict, "[Active States]") {
		t.Error("input_context_text missing [Active States] section")
	}
	if !strings.Contains(ict, "[Canonical State Layers]") {
		t.Error("input_context_text missing [Canonical State Layers] section")
	}
	if !strings.Contains(ict, "[Episode Summaries]") {
		t.Error("input_context_text missing [Episode Summaries] section")
	}

	supervisorPack, ok := resp["supervisor_input_pack"].(map[string]any)
	if !ok {
		t.Fatalf("supervisor_input_pack is not an object")
	}
	if supervisorPack["status"] != "ready" {
		t.Errorf("supervisor_input_pack.status = %v, want ready", supervisorPack["status"])
	}
	if supervisorPack["would_call_llm"] != false {
		t.Errorf("supervisor_input_pack.would_call_llm = %v, want false", supervisorPack["would_call_llm"])
	}
	if supervisorPack["would_write"] != false {
		t.Errorf("supervisor_input_pack.would_write = %v, want false", supervisorPack["would_write"])
	}
	if supervisorPack["prompt_source"] != "not_configured" {
		t.Errorf("supervisor_input_pack.prompt_source = %v, want not_configured", supervisorPack["prompt_source"])
	}
	if suffix, _ := supervisorPack["final_guidance_suffix"].(string); !strings.Contains(suffix, "Go R1 Supervisor Read Shadow") {
		t.Errorf("final_guidance_suffix missing read-shadow marker: %q", suffix)
	}

	criticPack, ok := resp["critic_input_pack"].(map[string]any)
	if !ok {
		t.Fatalf("critic_input_pack is not an object")
	}
	if criticPack["status"] != "ready" {
		t.Errorf("critic_input_pack.status = %v, want ready", criticPack["status"])
	}
	if criticPack["would_call_llm"] != false {
		t.Errorf("critic_input_pack.would_call_llm = %v, want false", criticPack["would_call_llm"])
	}
	if criticPack["verdict"] != "not_executed" {
		t.Errorf("critic_input_pack.verdict = %v, want not_executed", criticPack["verdict"])
	}

	injectionPack, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}
	if injectionPack["would_inject"] != true {
		t.Errorf("injection_pack.would_inject = %v, want true", injectionPack["would_inject"])
	}
	if injectionPack["would_write"] != false {
		t.Errorf("injection_pack.would_write = %v, want false", injectionPack["would_write"])
	}
	if injectionPack["final_budget_owner"] != "archive_center_js_assembleInjectionWithBudget" {
		t.Errorf("final_budget_owner = %v, want JS budget owner", injectionPack["final_budget_owner"])
	}
	if _, ok := injectionPack["budget_decisions"].(map[string]any); !ok {
		t.Fatalf("injection_pack.budget_decisions is not an object")
	}
	if memoryText, _ := injectionPack["memory_text"].(string); !strings.Contains(memoryText, "A warm evening in the garden") {
		t.Errorf("memory_text missing readable memory summary: %q", memoryText)
	}
	if kgText, _ := injectionPack["kg_text"].(string); !strings.Contains(kgText, "Alice --knows--> Bob") {
		t.Errorf("kg_text missing KG triple: %q", kgText)
	}
	if fallback := injectionPack["fallback_text"]; fallback != nil {
		t.Errorf("fallback_text = %v, want nil when two memories are available", fallback)
	}
	if deText, _ := injectionPack["latest_direct_evidence_text"].(string); !strings.Contains(deText, "red wax") {
		t.Errorf("latest_direct_evidence_text missing direct evidence: %q", deText)
	}
	if rawText, _ := injectionPack["recent_raw_turn_text"].(string); !strings.Contains(rawText, "The door creaks open") {
		t.Errorf("recent_raw_turn_text missing recent raw turn: %q", rawText)
	}
	if canonText, _ := injectionPack["canon_text"].(string); !strings.Contains(canonText, "library smells") {
		t.Errorf("canon_text missing canonical layer: %q", canonText)
	}

	recall, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}
	if recall["status"] != "ready" {
		t.Errorf("recall_result.status = %v, want ready", recall["status"])
	}
	items, ok := recall["items"].([]any)
	if !ok {
		t.Fatalf("recall_result.items is not an array")
	}
	if len(items) != 3 {
		t.Errorf("recall_result.items length = %d, want 3", len(items))
	}
	kgRecall, ok := recall["kg_triples"].([]any)
	if !ok {
		t.Fatalf("recall_result.kg_triples is not an array")
	}
	if len(kgRecall) != 1 {
		t.Errorf("recall_result.kg_triples length = %d, want 1", len(kgRecall))
	}
	episodeRecall, ok := recall["episodes"].([]any)
	if !ok {
		t.Fatalf("recall_result.episodes is not an array")
	}
	if len(episodeRecall) != 1 {
		t.Errorf("recall_result.episodes length = %d, want 1", len(episodeRecall))
	}
	counts, ok := recall["counts"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result.counts is not an object")
	}
	if counts["memories_total"] != float64(2) {
		t.Errorf("recall_result.counts.memories_total = %v, want 2", counts["memories_total"])
	}
	if counts["kg_total"] != float64(1) {
		t.Errorf("recall_result.counts.kg_total = %v, want 1", counts["kg_total"])
	}
	vectorShadow, ok := recall["vector_shadow"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result.vector_shadow is not an object")
	}
	if vectorShadow["status"] != "shadow" {
		t.Errorf("recall_result.vector_shadow.status = %v, want shadow", vectorShadow["status"])
	}
	if recall["would_call_vector"] != false {
		t.Errorf("recall_result.would_call_vector = %v, want false", recall["would_call_vector"])
	}
	if recall["would_write"] != false {
		t.Errorf("recall_result.would_write = %v, want false", recall["would_write"])
	}

	ss, ok := resp["session_state"].(map[string]any)
	if !ok {
		t.Fatalf("session_state is not an object")
	}
	if ss["snapshot_status"] != "ready" {
		t.Errorf("session_state.snapshot_status = %v, want ready", ss["snapshot_status"])
	}
	if ss["fetched"] != true {
		t.Errorf("session_state.fetched = %v, want true", ss["fetched"])
	}
	ssMeta, _ := ss["section_meta"].(map[string]any)
	if ssMeta["storyline_count"] != float64(1) {
		t.Errorf("session_state.section_meta.storyline_count = %v, want 1", ssMeta["storyline_count"])
	}
	if ssMeta["character_count"] != float64(1) {
		t.Errorf("session_state.section_meta.character_count = %v, want 1", ssMeta["character_count"])
	}
	if ssMeta["world_rule_count"] != float64(1) {
		t.Errorf("session_state.section_meta.world_rule_count = %v, want 1", ssMeta["world_rule_count"])
	}
	if ssMeta["pending_thread_count"] != float64(1) {
		t.Errorf("session_state.section_meta.pending_thread_count = %v, want 1", ssMeta["pending_thread_count"])
	}
	if ssMeta["active_state_count"] != float64(1) {
		t.Errorf("session_state.section_meta.active_state_count = %v, want 1", ssMeta["active_state_count"])
	}

	nc, ok := resp["narrative_control"].(map[string]any)
	if !ok {
		t.Fatalf("narrative_control is not an object")
	}
	if nc["state_status"] != "shadow_evidence" {
		t.Errorf("narrative_control.state_status = %v, want shadow_evidence", nc["state_status"])
	}
	if nc["storyline_count"] != float64(1) {
		t.Errorf("narrative_control.storyline_count = %v, want 1", nc["storyline_count"])
	}
	if nc["world_rule_count"] != float64(1) {
		t.Errorf("narrative_control.world_rule_count = %v, want 1", nc["world_rule_count"])
	}
	if nc["pending_thread_count"] != float64(1) {
		t.Errorf("narrative_control.pending_thread_count = %v, want 1", nc["pending_thread_count"])
	}
	if nc["character_count"] != float64(1) {
		t.Errorf("narrative_control.character_count = %v, want 1", nc["character_count"])
	}
	if nc["would_call_llm"] != false {
		t.Errorf("narrative_control.would_call_llm = %v, want false", nc["would_call_llm"])
	}
	if nc["would_write"] != false {
		t.Errorf("narrative_control.would_write = %v, want false", nc["would_write"])
	}

	cp, ok := resp["continuity_pack"].(map[string]any)
	if !ok {
		t.Fatalf("continuity_pack is not an object")
	}
	if cp["status"] != "ready" {
		t.Errorf("continuity_pack.status = %v, want ready", cp["status"])
	}
	if cp["resume_pack_present"] != true {
		t.Errorf("continuity_pack.resume_pack_present = %v, want true", cp["resume_pack_present"])
	}
	if cp["episode_count"] != float64(1) {
		t.Errorf("continuity_pack.episode_count = %v, want 1", cp["episode_count"])
	}
	if cp["chat_log_count"] != float64(2) {
		t.Errorf("continuity_pack.chat_log_count = %v, want 2", cp["chat_log_count"])
	}
	if cp["active_state_count"] != float64(1) {
		t.Errorf("continuity_pack.active_state_count = %v, want 1", cp["active_state_count"])
	}
	if cp["canonical_layer_count"] != float64(1) {
		t.Errorf("continuity_pack.canonical_layer_count = %v, want 1", cp["canonical_layer_count"])
	}
	cpItems, _ := cp["items"].([]any)
	if len(cpItems) == 0 {
		t.Errorf("continuity_pack.items is empty")
	}
	if cp["would_call_llm"] != false {
		t.Errorf("continuity_pack.would_call_llm = %v, want false", cp["would_call_llm"])
	}
	if cp["would_write"] != false {
		t.Errorf("continuity_pack.would_write = %v, want false", cp["would_write"])
	}

	progressionLedger, ok := resp["progression_ledger"].(map[string]any)
	if !ok {
		t.Fatalf("progression_ledger is not an object")
	}
	if progressionLedger["status"] != "ready" {
		t.Errorf("progression_ledger.status = %v, want ready", progressionLedger["status"])
	}
	if progressionLedger["chat_session_id"] != "sess-prep" {
		t.Errorf("progression_ledger.chat_session_id = %v, want sess-prep", progressionLedger["chat_session_id"])
	}
	if progressionLedger["storyline_count"] != float64(1) {
		t.Errorf("progression_ledger.storyline_count = %v, want 1", progressionLedger["storyline_count"])
	}
	if progressionLedger["world_rule_count"] != float64(1) {
		t.Errorf("progression_ledger.world_rule_count = %v, want 1", progressionLedger["world_rule_count"])
	}
	if progressionLedger["pending_thread_count"] != float64(1) {
		t.Errorf("progression_ledger.pending_thread_count = %v, want 1", progressionLedger["pending_thread_count"])
	}
	if progressionLedger["episode_count"] != float64(1) {
		t.Errorf("progression_ledger.episode_count = %v, want 1", progressionLedger["episode_count"])
	}
	if progressionLedger["would_write"] != false {
		t.Errorf("progression_ledger.would_write = %v, want false", progressionLedger["would_write"])
	}
	if progressionLedger["ledger_policy_version"] != "lw1h.v1" || progressionLedger["ledger_mode"] != "deterministic_no_llm" {
		t.Fatalf("progression_ledger LW policy/mode mismatch: %#v", progressionLedger)
	}
	for _, key := range []string{"unresolved_tensions", "consequences", "payoffs", "scene_deltas"} {
		items, ok := progressionLedger[key].([]any)
		if !ok || len(items) == 0 {
			t.Fatalf("progression_ledger.%s missing generated items: %#v", key, progressionLedger[key])
		}
		first, ok := items[0].(map[string]any)
		if !ok {
			t.Fatalf("progression_ledger.%s first item shape: %#v", key, items[0])
		}
		for _, field := range []string{"entry_type", "label", "source", "status", "lifecycle_state", "pressure_score", "decay_turns", "source_record_id", "source_message_ids", "affected_relations", "affected_world"} {
			if _, exists := first[field]; !exists {
				t.Fatalf("progression_ledger.%s first item missing %s: %#v", key, field, first)
			}
		}
	}
	payoffs, _ := progressionLedger["payoffs"].([]any)
	payoff := payoffs[0].(map[string]any)
	if payoff["payoff_state"] != "pending" || payoff["do_not_resolve_yet"] != true || payoff["resolve_guard_reason"] != "long_horizon_candidate" {
		t.Fatalf("progression_ledger payoff guard mismatch: %#v", payoff)
	}
	worldPressure, ok := progressionLedger["world_pressure"].(map[string]any)
	if !ok || worldPressure["status"] != "structured_support" {
		t.Fatalf("progression_ledger world_pressure missing: %#v", progressionLedger["world_pressure"])
	}
	for _, key := range []string{"factions", "regions", "offscreen_threads", "public_pressure", "timeline"} {
		if _, exists := worldPressure[key]; !exists {
			t.Fatalf("world_pressure missing %s: %#v", key, worldPressure)
		}
	}
	guard, ok := progressionLedger["supporting_precedence_guard"].(map[string]any)
	if !ok || guard["cannot_override_current_user_input"] != true || guard["cannot_override_verified_direct_evidence"] != true {
		t.Fatalf("supporting precedence guard mismatch: %#v", progressionLedger["supporting_precedence_guard"])
	}
	compat, ok := progressionLedger["compatibility_contract"].(map[string]any)
	if !ok || compat["shape_mode"] != "additive_non_breaking" || compat["consumer_safe"] != true {
		t.Fatalf("compatibility contract mismatch: %#v", progressionLedger["compatibility_contract"])
	}
	lifecycle, ok := progressionLedger["lifecycle_model"].(map[string]any)
	if !ok || lifecycle["status"] != "active" || lenAnySlice(lifecycle["states"]) != 6 || lenAnyMap(lifecycle["decay_rules"]) == 0 {
		t.Fatalf("lifecycle model mismatch: %#v", progressionLedger["lifecycle_model"])
	}
	doNotResolveGuard, ok := progressionLedger["do_not_resolve_guard"].(map[string]any)
	if !ok || doNotResolveGuard["status"] != "active" || doNotResolveGuard["mode"] != "deterministic_no_llm" {
		t.Fatalf("do_not_resolve_guard mismatch: %#v", progressionLedger["do_not_resolve_guard"])
	}
	tracePreview, ok := resp["trace_preview"].(map[string]any)
	if !ok {
		t.Fatalf("trace_preview is not an object")
	}
	for _, key := range []string{
		"story_ledger_policy_version", "unresolved_tensions_count", "consequences_count", "payoffs_count", "scene_deltas_count",
		"payoff_pending_count", "world_pressure_ready", "world_pressure_policy_version", "continuity_precedence_policy_version",
		"supporting_precedence_guard_ready", "compatibility_policy_version", "compatibility_ready", "lifecycle_policy_version",
		"lifecycle_ready", "lifecycle_entry_count", "do_not_resolve_policy_version", "do_not_resolve_guard_ready", "do_not_resolve_protected_count",
	} {
		if _, exists := tracePreview[key]; !exists {
			t.Fatalf("trace_preview missing LW field %s: %#v", key, tracePreview)
		}
	}
	if tracePreview["story_ledger_policy_version"] != "lw1h.v1" || tracePreview["world_pressure_policy_version"] != "lw1d.v1" || tracePreview["do_not_resolve_policy_version"] != "lw1h.v1" {
		t.Fatalf("trace_preview LW policy mismatch: %#v", tracePreview)
	}

	autonomyPlan, ok := resp["autonomy_plan"].(map[string]any)
	if !ok {
		t.Fatalf("autonomy_plan is not an object")
	}
	if autonomyPlan["status"] != "ready" {
		t.Errorf("autonomy_plan.status = %v, want ready", autonomyPlan["status"])
	}
	if autonomyPlan["guide_mode"] != "off" {
		t.Errorf("autonomy_plan.guide_mode = %v, want off", autonomyPlan["guide_mode"])
	}
	if autonomyPlan["narrative_stance"] != "balanced" {
		t.Errorf("autonomy_plan.narrative_stance = %v, want balanced", autonomyPlan["narrative_stance"])
	}
	if autonomyPlan["would_call_llm"] != false {
		t.Errorf("autonomy_plan.would_call_llm = %v, want false", autonomyPlan["would_call_llm"])
	}

	microBeat, ok := resp["micro_beat_proposal"].(map[string]any)
	if !ok {
		t.Fatalf("micro_beat_proposal is not an object")
	}
	if microBeat["status"] != "ready" {
		t.Errorf("micro_beat_proposal.status = %v, want ready", microBeat["status"])
	}
	beats, _ := microBeat["beats"].([]any)
	if len(beats) != 2 {
		t.Errorf("micro_beat_proposal.beats len = %d, want 2", len(beats))
	}
	if microBeat["would_call_llm"] != false {
		t.Errorf("micro_beat_proposal.would_call_llm = %v, want false", microBeat["would_call_llm"])
	}
	if microBeat["would_write"] != false {
		t.Errorf("micro_beat_proposal.would_write = %v, want false", microBeat["would_write"])
	}

	sceneStep, ok := resp["scene_step_proposal"].(map[string]any)
	if !ok {
		t.Fatalf("scene_step_proposal is not an object")
	}
	if sceneStep["status"] != "ready" {
		t.Errorf("scene_step_proposal.status = %v, want ready", sceneStep["status"])
	}
	steps, _ := sceneStep["steps"].([]any)
	if len(steps) != 2 {
		t.Errorf("scene_step_proposal.steps len = %d, want topK 2", len(steps))
	}
	if sceneStep["would_call_llm"] != false {
		t.Errorf("scene_step_proposal.would_call_llm = %v, want false", sceneStep["would_call_llm"])
	}
	if sceneStep["would_write"] != false {
		t.Errorf("scene_step_proposal.would_write = %v, want false", sceneStep["would_write"])
	}

	combinedProposal, ok := resp["combined_proposal"].(map[string]any)
	if !ok {
		t.Fatalf("combined_proposal is not an object")
	}
	if combinedProposal["status"] != "ready" {
		t.Errorf("combined_proposal.status = %v, want ready", combinedProposal["status"])
	}
	if combinedProposal["micro_beat_count"] != float64(2) {
		t.Errorf("combined_proposal.micro_beat_count = %v, want 2", combinedProposal["micro_beat_count"])
	}
	if combinedProposal["scene_step_count"] != float64(2) {
		t.Errorf("combined_proposal.scene_step_count = %v, want topK 2", combinedProposal["scene_step_count"])
	}
	if combinedProposal["source"] != "go_r1_read_shadow" {
		t.Errorf("combined_proposal.source = %v, want go_r1_read_shadow", combinedProposal["source"])
	}
	if combinedProposal["would_write"] != false {
		t.Errorf("combined_proposal.would_write = %v, want false", combinedProposal["would_write"])
	}

	writebackPreview, ok := resp["writeback_preview"].(map[string]any)
	if !ok {
		t.Fatalf("writeback_preview is not an object")
	}
	if writebackPreview["status"] != "ready" {
		t.Errorf("writeback_preview.status = %v, want ready", writebackPreview["status"])
	}
	targets, _ := writebackPreview["targets"].([]any)
	if len(targets) != 6 {
		t.Errorf("writeback_preview.targets len = %d, want 6", len(targets))
	}
	if writebackPreview["would_write"] != false {
		t.Errorf("writeback_preview.would_write = %v, want false", writebackPreview["would_write"])
	}
}

func TestPrepareTurnPersonaRecollectionSupportLane(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "target-loop", TurnIndex: 1, Role: "user", Content: "Why does this feel familiar?"},
			{ID: 2, ChatSessionID: "target-loop", TurnIndex: 1, Role: "assistant", Content: "Chloe pauses near the mirror."},
		},
		returnPersonaEntries: []store.PersonaMemoryEntry{
			{
				ID:              7,
				CapsuleID:       3,
				SourceTurn:      12,
				MemoryText:      "Siwoo remembers that Chloe hid the brass key behind the cracked mirror in the previous loop.",
				Importance10:    8.5,
				EmotionalWeight: 0.7,
				Portability:     "cross_world",
				InjectionPolicy: "support_only",
			},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"target-loop","turn_index":2,"raw_user_input":"Look around the room.","settings":{"max_injection_chars":1200,"max_input_context_chars":800,"injection_enabled":true,"input_context_enabled":true,"top_k":2}}`
	body = `{"chat_session_id":"sess-weak-plan","turn_index":8,"raw_user_input":"continue","client_meta":{"language_context":{"session_output_language":"ko","output_language_source":"plugin_setting"}},"settings":{"max_injection_chars":1600,"max_input_context_chars":900,"injection_enabled":true,"input_context_enabled":true,"top_k":2,"guide_mode":"standard","guide_strength":"weak","narrative_stance":"balanced"}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	injectionText, _ := resp["injection_text"].(string)
	if !strings.Contains(injectionText, "[Persona Recollection]") {
		t.Fatalf("injection_text missing persona recollection: %q", injectionText)
	}
	if strings.Contains(injectionText, "brass key") {
		t.Fatalf("injection_text leaked protected persona recollection content: %q", injectionText)
	}
	if !strings.Contains(injectionText, "Secret Guard") || !strings.Contains(injectionText, "protagonist-only private intuition") || !strings.Contains(injectionText, "Never reveal its origin") {
		t.Fatalf("injection_text missing persona secret guard: %q", injectionText)
	}
	if strings.Contains(injectionText, "previous loop") || strings.Contains(injectionText, "regressor") || strings.Contains(injectionText, "regression") || strings.Contains(injectionText, "loop") {
		t.Fatalf("injection_text leaked explicit loop secret instead of masked protagonist-private hint: %q", injectionText)
	}
	if !strings.Contains(injectionText, "Protected hint") {
		t.Fatalf("injection_text missing masked persona secret hint: %q", injectionText)
	}
	inputContextText, _ := resp["input_context_text"].(string)
	if !strings.Contains(inputContextText, "[Persona Recollection]") || !strings.Contains(inputContextText, "support-only private recollection") {
		t.Fatalf("input_context_text missing support-only persona lane: %q", inputContextText)
	}
	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}
	if ip["persona_recollection_active"] != true {
		t.Fatalf("persona_recollection_active = %v, want true", ip["persona_recollection_active"])
	}
	policy, ok := ip["persona_recollection_policy"].(map[string]any)
	if !ok {
		t.Fatalf("persona_recollection_policy is not an object")
	}
	if policy["truth_authority"] != false || policy["canonical_write"] != false {
		t.Fatalf("persona policy must be support-only, got %+v", policy)
	}
	if policy["secret_guard_active"] != true {
		t.Fatalf("persona policy missing active secret guard: %+v", policy)
	}
	secretGuard, ok := policy["secret_guard"].(map[string]any)
	if !ok || secretGuard["active"] != true {
		t.Fatalf("persona policy secret_guard mismatch: %+v", policy["secret_guard"])
	}
	surface, ok := resp["persona_recollection"].(map[string]any)
	if !ok {
		t.Fatalf("persona_recollection surface missing")
	}
	if surface["status"] != "ready" || surface["count"] != float64(1) || surface["would_write"] != false {
		t.Fatalf("persona surface mismatch: %+v", surface)
	}
	if surface["secret_guard_active"] != true {
		t.Fatalf("persona surface missing active secret guard: %+v", surface)
	}
	if len(fake.savedMemories) != 0 || len(fake.savedKGTriples) != 0 || len(fake.savedEvidence) != 0 {
		t.Fatalf("prepare-turn persona recollection must not write canonical rows: mem=%d kg=%d evi=%d", len(fake.savedMemories), len(fake.savedKGTriples), len(fake.savedEvidence))
	}
}
