package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestIntentRoutingRuntimeConfig(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-irc","turn_index":1}`
	req := httptest.NewRequest(http.MethodPost, "/intent-routing/runtime-config", bytes.NewReader([]byte(body)))
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
		t.Fatalf("status = %v, want ok", resp["status"])
	}
	if resp["routing_version"] != "p58a.v1" {
		t.Fatalf("routing_version = %v, want p58a.v1", resp["routing_version"])
	}
	if resp["routing_mode"] != "per_intent_shadow" {
		t.Fatalf("routing_mode = %v, want per_intent_shadow", resp["routing_mode"])
	}
	if resp["default_route"] != "single_query_shared" {
		t.Fatalf("default_route = %v, want single_query_shared", resp["default_route"])
	}

	intents, ok := resp["intents"].([]any)
	if !ok {
		t.Fatalf("intents is not an array")
	}
	if len(intents) != 4 {
		t.Fatalf("intents len = %d, want 4", len(intents))
	}
}

func TestBuildRecallResultHierarchyCollisionRules(t *testing.T) {
	episodeSums := []store.EpisodeSummary{
		{ID: 10, ChatSessionID: "s1", FromTurn: 1, ToTurn: 3, SummaryText: "ep1"},
	}
	resumePack := &store.ResumePack{
		Chapter: &store.ChapterSummary{
			ID: 20, ChatSessionID: "s1", FromTurn: 1, ToTurn: 5, ChapterIndex: 1, ChapterTitle: "Ch1", SummaryText: "ch1",
		},
		Arc: &store.ArcSummary{
			ID: 30, ChatSessionID: "s1", FromTurn: 1, ToTurn: 10, ArcName: "Arc1", CoreConflict: "conflict",
		},
		Saga: &store.SagaDigest{
			ID: 40, ChatSessionID: "s1", FromTurn: 1, ToTurn: 20, EraLabel: "E1", SagaSummary: "s1",
		},
	}
	trace := buildHierarchyConsistencyTrace(nil, resumePack, episodeSums)
	if trace["version"] != "p59a.v1" {
		t.Fatalf("version mismatch: %v", trace["version"])
	}
	if trace["saga_covers_arc"] != true {
		t.Fatalf("saga_covers_arc = %v, want true", trace["saga_covers_arc"])
	}
	if trace["arc_covers_chapter"] != true {
		t.Fatalf("arc_covers_chapter = %v, want true", trace["arc_covers_chapter"])
	}
	collisionRules, ok := trace["collision_rules"].([]string)
	if !ok {
		t.Fatalf("collision_rules is not a string slice")
	}
	if len(collisionRules) != 4 {
		t.Fatalf("collision_rules len = %d, want 4", len(collisionRules))
	}
}

func TestPrepareTurnArcDeliveryPath(t *testing.T) {
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-arc", TurnIndex: 2, SummaryJSON: `{"turn_summary":"A warm evening"}`, Importance: 0.9},
		},
		returnResumePack: &store.ResumePack{
			PackStatus:    "ready",
			AssembledText: "Session resume text.",
			Arc: &store.ArcSummary{
				ID:            1,
				ChatSessionID: "sess-arc",
				FromTurn:      1,
				ToTurn:        20,
				ArcName:       "The Great Arc",
				CoreConflict:  "Man vs Nature",
				CreatedAt:     &now,
			},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-arc","turn_index":3,"raw_user_input":"What happens next?","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
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

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("generation_packet is not an object")
	}

	trace, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("trace_summary is not an object")
	}

	if trace["arc_delivered"] != true {
		t.Fatalf("arc_delivered = %v, want true", trace["arc_delivered"])
	}
	if trace["arc_text_chars"].(float64) <= 0 {
		t.Fatalf("arc_text_chars = %v, want > 0", trace["arc_text_chars"])
	}

	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}

	if ip["arc_delivered"] != true {
		t.Fatalf("injection_pack.arc_delivered = %v, want true", ip["arc_delivered"])
	}

	at, ok := ip["arc_text"].(string)
	if !ok || at == "" {
		t.Fatalf("injection_pack.arc_text missing or empty")
	}
}

func TestPrepareTurnRuntimeTokenProfile(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-rt", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-rt","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2},"client_meta":{"context_window_profile":"ultra"}}`
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

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("generation_packet is not an object")
	}

	trace, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("trace_summary is not an object")
	}

	rtp, ok := trace["runtime_token_profile"].(map[string]any)
	if !ok {
		t.Fatalf("runtime_token_profile is not an object")
	}

	if rtp["version"] != "p61a.v1" {
		t.Fatalf("version = %v, want p61a.v1", rtp["version"])
	}
	if rtp["context_window_profile"] != "ultra" {
		t.Fatalf("context_window_profile = %v, want ultra", rtp["context_window_profile"])
	}
	if rtp["auto_optimized"] != true {
		t.Fatalf("auto_optimized = %v, want true", rtp["auto_optimized"])
	}
}

func TestBuildRecallResultTemporalProximityBoost(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-tp", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-tp", TurnIndex: 1, Role: "user", Content: "hello"},
			{ID: 2, ChatSessionID: "sess-tp", TurnIndex: 2, Role: "assistant", Content: "hi"},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-tp","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2},"client_meta":{"context_window_profile":"ultra"}}`
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

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}

	tpb, ok := rr["temporal_proximity_boost"].(map[string]any)
	if !ok {
		t.Fatalf("temporal_proximity_boost is not an object")
	}

	if tpb["version"] != "p71a.v1" {
		t.Fatalf("version = %v, want p71a.v1", tpb["version"])
	}
	if tpb["boost_active"] != true {
		t.Fatalf("boost_active = %v, want true", tpb["boost_active"])
	}
	if tpb["profile"] != "ultra" {
		t.Fatalf("profile = %v, want ultra", tpb["profile"])
	}
}

func TestBuildRecallResultBudgetTransitionEvidence(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-bt", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-bt","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
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

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}

	pbp, ok := rr["packet_budget_policy"].(map[string]any)
	if !ok {
		t.Fatalf("packet_budget_policy is not an object")
	}

	bt, ok := pbp["budget_transition"].(map[string]any)
	if !ok {
		t.Fatalf("budget_transition is not an object")
	}

	if bt["version"] != "p75a.v1" {
		t.Fatalf("version = %v, want p75a.v1", bt["version"])
	}
	if bt["from_mode"] != "policy_only" {
		t.Fatalf("from_mode = %v, want policy_only", bt["from_mode"])
	}
	if bt["to_mode"] != "enforced_shadow" {
		t.Fatalf("to_mode = %v, want enforced_shadow", bt["to_mode"])
	}
	if bt["transition_ready"] != true {
		t.Fatalf("transition_ready = %v, want true", bt["transition_ready"])
	}
}

func TestBuildRecallResultBudgetCaps(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-bc", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-bc","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
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

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}

	pbp, ok := rr["packet_budget_policy"].(map[string]any)
	if !ok {
		t.Fatalf("packet_budget_policy is not an object")
	}

	bc, ok := pbp["budget_caps"].(map[string]any)
	if !ok {
		t.Fatalf("budget_caps is not an object")
	}

	if bc["version"] != "p76a.v1" {
		t.Fatalf("version = %v, want p76a.v1", bc["version"])
	}
	if bc["layer_cap"].(float64) != 12 {
		t.Fatalf("layer_cap = %v, want 12", bc["layer_cap"])
	}
	if bc["char_cap"].(float64) != 3000 {
		t.Fatalf("char_cap = %v, want 3000", bc["char_cap"])
	}
	if bc["canon_hard_floor"].(float64) != 120 {
		t.Fatalf("canon_hard_floor = %v, want 120", bc["canon_hard_floor"])
	}
}

func TestPrepareTurnBudgetDecisionsT1a(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-bd", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-bd","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
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

	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}

	bd, ok := ip["budget_decisions"].(map[string]any)
	if !ok {
		t.Fatalf("budget_decisions is not an object")
	}

	if bd["t1a_enforced_ready"] != true {
		t.Fatalf("t1a_enforced_ready = %v, want true", bd["t1a_enforced_ready"])
	}
	if bd["t1a_transition"] != "policy_only_to_enforced_shadow" {
		t.Fatalf("t1a_transition = %v, want policy_only_to_enforced_shadow", bd["t1a_transition"])
	}
}

func TestBuildRecallResultRelationshipFirstBudget(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-rf", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-rf","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
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

	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}

	counts, ok := ip["counts"].(map[string]any)
	if !ok {
		t.Fatalf("counts is not an object")
	}

	rfb, ok := counts["relationship_first_budget"].(map[string]any)
	if !ok {
		t.Fatalf("relationship_first_budget is not an object")
	}

	if rfb["version"] != "p80a.v1" {
		t.Fatalf("version = %v, want p80a.v1", rfb["version"])
	}
	if rfb["status"] != "shadow_only" {
		t.Fatalf("status = %v, want shadow_only", rfb["status"])
	}
	if rfb["structure"] != "relationship_first" {
		t.Fatalf("structure = %v, want relationship_first", rfb["structure"])
	}
	if rfb["long_tier_cap"].(float64) != 2400 {
		t.Fatalf("long_tier_cap = %v, want 2400", rfb["long_tier_cap"])
	}
	if rfb["ultra_tier_cap"].(float64) != 1800 {
		t.Fatalf("ultra_tier_cap = %v, want 1800", rfb["ultra_tier_cap"])
	}
	if rfb["extreme_tier_cap"].(float64) != 1200 {
		t.Fatalf("extreme_tier_cap = %v, want 1200", rfb["extreme_tier_cap"])
	}
}

func TestPrepareTurnThreeHundredTurnRelationshipRecallKeepsCurrentState(t *testing.T) {
	chatLogs := make([]store.ChatLog, 0, 300)
	for i := 1; i <= 300; i++ {
		role := "user"
		if i%2 == 0 {
			role = "assistant"
		}
		chatLogs = append(chatLogs, store.ChatLog{
			ID:            int64(i),
			ChatSessionID: "sess-rel300",
			TurnIndex:     i,
			Role:          role,
			Content:       fmt.Sprintf("turn %03d old archive corridor scene", i),
		})
	}
	fake := &turnRecordingStore{
		returnChatLogs: chatLogs,
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-rel300", TurnIndex: 42, SummaryJSON: `{"turn_summary":"old unrelated archive corridor memory"}`, Importance: 0.35},
		},
		returnCharStates: []store.CharacterState{{
			ChatSessionID:     "sess-rel300",
			CharacterName:     "Chloe",
			StatusJSON:        `{"mood":"guarded but attentive"}`,
			RelationshipsJSON: `{"Hero":{"affection":82,"tension":18,"last_change":"Chloe chose to trust Hero in the current scene"}}`,
			TurnIndex:         300,
		}},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-rel300","turn_index":301,"raw_user_input":"Continue the current relationship scene with Chloe.","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":900,"max_input_context_chars":500,"top_k":5}}`
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
	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}
	characterText, _ := ip["character_text"].(string)
	if !strings.Contains(characterText, "Chloe") || !strings.Contains(characterText, "relationships") || !strings.Contains(characterText, "affection") || !strings.Contains(characterText, "82") {
		t.Fatalf("character_text did not preserve current relationship state after 300 turns: %q", characterText)
	}
	trace, ok := resp["trace_preview"].(map[string]any)
	if !ok {
		t.Fatalf("trace_preview is not an object")
	}
	evidenceCounts, ok := trace["evidence_counts"].(map[string]any)
	if !ok {
		t.Fatalf("trace_preview.evidence_counts is not an object")
	}
	if evidenceCounts["chat_logs"].(float64) != 300 {
		t.Fatalf("chat_logs = %v, want 300", evidenceCounts["chat_logs"])
	}
	counts, ok := ip["counts"].(map[string]any)
	if !ok {
		t.Fatalf("counts is not an object")
	}
	rfb, ok := counts["relationship_first_budget"].(map[string]any)
	if !ok || rfb["structure"] != "relationship_first" {
		t.Fatalf("relationship_first_budget missing or wrong: %#v", counts["relationship_first_budget"])
	}
}

func TestBuildRecallResultStalenessThreshold(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-st", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-st", TurnIndex: 1, Role: "user", Content: "a"},
			{ID: 2, ChatSessionID: "sess-st", TurnIndex: 2, Role: "assistant", Content: "b"},
			{ID: 3, ChatSessionID: "sess-st", TurnIndex: 3, Role: "user", Content: "c"},
			{ID: 4, ChatSessionID: "sess-st", TurnIndex: 4, Role: "assistant", Content: "d"},
			{ID: 5, ChatSessionID: "sess-st", TurnIndex: 5, Role: "user", Content: "e"},
			{ID: 6, ChatSessionID: "sess-st", TurnIndex: 6, Role: "assistant", Content: "f"},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-st","turn_index":7,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
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

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}

	sfs, ok := rr["summary_failure_stability"].(map[string]any)
	if !ok {
		t.Fatalf("summary_failure_stability is not an object")
	}

	st, ok := sfs["staleness_threshold"].(map[string]any)
	if !ok {
		t.Fatalf("staleness_threshold is not an object")
	}

	if st["version"] != "p85a.v1" {
		t.Fatalf("version = %v, want p85a.v1", st["version"])
	}
	if st["threshold_turns"].(float64) != 5 {
		t.Fatalf("threshold_turns = %v, want 5", st["threshold_turns"])
	}
	if st["detected"] != true {
		t.Fatalf("detected = %v, want true", st["detected"])
	}
}

func TestBuildRecallResultRetryEnqueue(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-re", TurnIndex: 1, Role: "assistant", Content: ""},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-re","turn_index":2,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
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

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}

	sfs, ok := rr["summary_failure_stability"].(map[string]any)
	if !ok {
		t.Fatalf("summary_failure_stability is not an object")
	}

	re, ok := sfs["retry_enqueue"].(map[string]any)
	if !ok {
		t.Fatalf("retry_enqueue is not an object")
	}

	if re["version"] != "p86a.v1" {
		t.Fatalf("version = %v, want p86a.v1", re["version"])
	}
	if re["enqueue_ready"] != true {
		t.Fatalf("enqueue_ready = %v, want true", re["enqueue_ready"])
	}
	if re["force_regenerate"] != false {
		t.Fatalf("force_regenerate = %v, want false", re["force_regenerate"])
	}
}

func TestBuildRecallResultFailureWarning(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-fw","turn_index":2,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
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

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}

	sfs, ok := rr["summary_failure_stability"].(map[string]any)
	if !ok {
		t.Fatalf("summary_failure_stability is not an object")
	}

	fw, ok := sfs["failure_warning"].(map[string]any)
	if !ok {
		t.Fatalf("failure_warning is not an object")
	}

	if fw["version"] != "p87a.v1" {
		t.Fatalf("version = %v, want p87a.v1", fw["version"])
	}
	if fw["warning_active"] != true {
		t.Fatalf("warning_active = %v, want true", fw["warning_active"])
	}
	if fw["warning_level"] != "warn" {
		t.Fatalf("warning_level = %v, want warn", fw["warning_level"])
	}
}

func TestBuildRecallResultReplayGate(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-rg", TurnIndex: 2, SummaryJSON: `{"turn_summary":"test"}`, Importance: 0.9},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-rg", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-rg","turn_index":3,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
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

	rr, ok := resp["recall_result"].(map[string]any)
	if !ok {
		t.Fatalf("recall_result is not an object")
	}

	sfs, ok := rr["summary_failure_stability"].(map[string]any)
	if !ok {
		t.Fatalf("summary_failure_stability is not an object")
	}

	rg, ok := sfs["replay_gate"].(map[string]any)
	if !ok {
		t.Fatalf("replay_gate is not an object")
	}

	if rg["version"] != "p88a.v1" {
		t.Fatalf("version = %v, want p88a.v1", rg["version"])
	}
	if rg["gate_active"] != true {
		t.Fatalf("gate_active = %v, want true", rg["gate_active"])
	}
	if rg["session_captured"] != true {
		t.Fatalf("session_captured = %v, want true", rg["session_captured"])
	}
}

func TestPrepareTurnInjectionPackBudgetDecisionsOffMode(t *testing.T) {
	fake := &turnRecordingStore{}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-bd-off","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":false,"input_context_enabled":false,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
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

	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}

	bd, ok := ip["budget_decisions"].(map[string]any)
	if !ok {
		t.Fatalf("budget_decisions is not an object")
	}

	if bd["version"] != "t1c.v1" {
		t.Fatalf("version = %v, want t1c.v1", bd["version"])
	}
	if bd["mode"] != "read_only_surface" {
		t.Fatalf("mode = %v, want read_only_surface", bd["mode"])
	}
	if bd["status"] != "off" {
		t.Fatalf("status = %v, want off", bd["status"])
	}
	if bd["decision_count"] != float64(0) {
		t.Fatalf("decision_count = %v, want 0", bd["decision_count"])
	}
	if bd["source_mapping"] != "recall_result.intent_execution_shadow.budget_enforcement" {
		t.Fatalf("source_mapping = %v, want recall_result.intent_execution_shadow.budget_enforcement", bd["source_mapping"])
	}
	if bd["source_event"] != "budget_enforcement" {
		t.Fatalf("source_event = %v, want budget_enforcement", bd["source_event"])
	}
	decisions, ok := bd["decisions"].([]any)
	if !ok {
		t.Fatalf("decisions is not an array")
	}
	if len(decisions) != 0 {
		t.Fatalf("decisions len = %d, want 0", len(decisions))
	}
}

func TestPrepareTurnInjectionPackBudgetDecisionsReadyMode(t *testing.T) {
	fake := &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-bd-ready", TurnIndex: 2, SummaryJSON: `{"turn_summary":"A warm evening in the garden"}`, Importance: 0.9, PlaceWing: "East", PlaceRoom: "Garden"},
			{ID: 2, ChatSessionID: "sess-bd-ready", TurnIndex: 3, SummaryJSON: `{"turn_summary":"The door creaks open"}`, Importance: 0.8, PlaceWing: "North", PlaceRoom: "Hall"},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 1, ChatSessionID: "sess-bd-ready", TurnAnchor: 2, EvidenceText: "The red wax seal was broken.", EvidenceKind: "item"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-bd-ready", TurnIndex: 1, Role: "user", Content: "Hello there"},
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-bd-ready","turn_index":4,"raw_user_input":"continue","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":500,"max_input_context_chars":400,"top_k":2}}`
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

	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("injection_pack is not an object")
	}

	bd, ok := ip["budget_decisions"].(map[string]any)
	if !ok {
		t.Fatalf("budget_decisions is not an object")
	}

	if bd["version"] != "t1c.v1" {
		t.Fatalf("version = %v, want t1c.v1", bd["version"])
	}
	if bd["mode"] != "read_only_surface" {
		t.Fatalf("mode = %v, want read_only_surface", bd["mode"])
	}
	if bd["status"] != "ready" {
		t.Fatalf("status = %v, want ready", bd["status"])
	}
	decisionCount, ok := bd["decision_count"].(float64)
	if !ok || decisionCount <= 0 {
		t.Fatalf("decision_count = %v, want > 0", bd["decision_count"])
	}
	decisions, ok := bd["decisions"].([]any)
	if !ok {
		t.Fatalf("decisions is not an array")
	}
	if len(decisions) == 0 {
		t.Fatalf("decisions len = 0, want > 0")
	}

	if bd["source_mapping"] != "recall_result.intent_execution_shadow.budget_enforcement" {
		t.Fatalf("source_mapping = %v, want recall_result.intent_execution_shadow.budget_enforcement", bd["source_mapping"])
	}
	if bd["source_event"] != "budget_enforcement" {
		t.Fatalf("source_event = %v, want budget_enforcement", bd["source_event"])
	}
	sourceCounters, ok := bd["source_counters"].([]any)
	if !ok || len(sourceCounters) == 0 {
		t.Fatalf("source_counters = %v, want non-empty array", bd["source_counters"])
	}

	requiredFields := []string{"intent", "tier", "document_id", "decision", "reason", "cap_scope", "char_cost", "running_total_chars", "cap_chars"}
	for i, d := range decisions {
		dec, ok := d.(map[string]any)
		if !ok {
			t.Fatalf("decision[%d] is not an object", i)
		}
		for _, field := range requiredFields {
			if _, ok := dec[field]; !ok {
				t.Fatalf("decision[%d] missing field %s", i, field)
			}
		}
	}
}

func TestBuildRecallResultIntentExecutionShadowBudgetEnforcementT1b(t *testing.T) {
	docs := []map[string]any{
		{"tier": "memory", "document_id": "d1", "text": "hello world"},
		{"tier": "episode", "document_id": "d2", "text": "episode summary here"},
		{"tier": "saga", "document_id": "d3", "text": "saga text"},
		{"tier": "arc", "document_id": "d4", "text": "arc content"},
	}
	shadow := buildIntentExecutionShadow(docs, map[string]any{}, "long", q3PacketBudgetPolicy())
	be, ok := shadow["budget_enforcement"].(map[string]any)
	if !ok {
		t.Fatal("budget_enforcement missing")
	}
	if be["version"] != "t1b.v1" {
		t.Fatalf("version = %v, want t1b.v1", be["version"])
	}
	if be["mode"] != "enforced_shadow" {
		t.Fatalf("mode = %v, want enforced_shadow", be["mode"])
	}
	if be["canon_hard_floor"] != 120 {
		t.Fatalf("canon_hard_floor = %v, want 120", be["canon_hard_floor"])
	}
	if be["canon_floor_reserved_chars"] != 120 {
		t.Fatalf("canon_floor_reserved_chars = %v, want 120", be["canon_floor_reserved_chars"])
	}
	if _, ok := be["canon_selected_chars"]; !ok {
		t.Fatal("canon_selected_chars missing")
	}
	if _, ok := be["retrieval_layer_caps"]; !ok {
		t.Fatal("retrieval_layer_caps missing")
	}
	rlc, ok := be["retrieval_layer_caps"].([]map[string]any)
	if !ok || len(rlc) != 4 {
		t.Fatalf("retrieval_layer_caps mismatch: %v", be["retrieval_layer_caps"])
	}
	for _, cap := range rlc {
		if cap["reason"] != "priority_deferred" {
			t.Fatalf("reason = %v, want priority_deferred", cap["reason"])
		}
		if cap["cap_scope"] != "layer_cap" {
			t.Fatalf("cap_scope = %v, want layer_cap", cap["cap_scope"])
		}
	}
	if _, ok := be["reason_counts"]; !ok {
		t.Fatal("reason_counts missing")
	}
	rc, ok := be["reason_counts"].(map[string]int)
	if !ok {
		t.Fatal("reason_counts type mismatch")
	}
	if _, ok := rc["floor_reserved"]; !ok {
		t.Fatal("floor_reserved missing in reason_counts")
	}
	et, ok := shadow["enforced_takeover"].(map[string]any)
	if !ok {
		t.Fatal("enforced_takeover missing")
	}
	if et["version"] != "t1e.v1" {
		t.Fatalf("enforced_takeover version = %v, want t1e.v1", et["version"])
	}
	if et["mode"] != "enforced_default_takeover_only" {
		t.Fatalf("enforced_takeover mode = %v, want enforced_default_takeover_only", et["mode"])
	}
	if _, ok := et["selected_candidates"]; !ok {
		t.Fatal("selected_candidates missing in enforced_takeover")
	}
}

func TestBuildRecallResultBudgetDecisionsCallbackSagaLayerCap(t *testing.T) {
	docs := []map[string]any{
		{"tier": "memory", "document_id": "d1", "text": "a"},
		{"tier": "arc", "document_id": "d2", "text": "b"},
		{"tier": "saga", "document_id": "d3", "text": "c"},
	}
	shadow := buildIntentExecutionShadow(docs, map[string]any{}, "long", q3PacketBudgetPolicy())
	be, ok := shadow["budget_enforcement"].(map[string]any)
	if !ok {
		t.Fatal("budget_enforcement missing")
	}
	rlc, ok := be["retrieval_layer_caps"].([]map[string]any)
	if !ok || len(rlc) != 4 {
		t.Fatalf("retrieval_layer_caps len = %v, want 4", len(rlc))
	}
	callbackFound := false
	for _, c := range rlc {
		if c["intent"] == "callback" {
			callbackFound = true
			if c["cap_scope"] != "layer_cap" {
				t.Fatalf("callback cap_scope = %v, want layer_cap", c["cap_scope"])
			}
			if c["reason"] != "priority_deferred" {
				t.Fatalf("callback reason = %v, want priority_deferred", c["reason"])
			}
		}
	}
	if !callbackFound {
		t.Fatal("callback layer cap not found")
	}
}

func TestBuildRecallResultBudgetDecisionsCanonHardFloor(t *testing.T) {
	docs := []map[string]any{
		{"tier": "memory", "document_id": "d1", "text": "canon memory text"},
		{"tier": "episode", "document_id": "d2", "text": "episode text"},
		{"tier": "arc", "document_id": "d3", "text": "arc text"},
	}
	shadow := buildIntentExecutionShadow(docs, map[string]any{}, "long", q3PacketBudgetPolicy())
	be, ok := shadow["budget_enforcement"].(map[string]any)
	if !ok {
		t.Fatal("budget_enforcement missing")
	}
	if be["canon_hard_floor"] != 120 {
		t.Fatalf("canon_hard_floor = %v, want 120", be["canon_hard_floor"])
	}
	if be["canon_floor_reserved_chars"] != 120 {
		t.Fatalf("canon_floor_reserved_chars = %v, want 120", be["canon_floor_reserved_chars"])
	}
	if _, ok := be["canon_selected_chars"]; !ok {
		t.Fatal("canon_selected_chars missing")
	}
}
