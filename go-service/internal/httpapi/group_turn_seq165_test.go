package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func seq165Map(t *testing.T, parent map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := parent[key].(map[string]any)
	if !ok {
		t.Fatalf("missing %s map", key)
	}
	return value
}

func seq165FindSlot(t *testing.T, slots []any, name string) map[string]any {
	t.Helper()
	for _, raw := range slots {
		slot, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if slot["name"] == name {
			return slot
		}
	}
	t.Fatalf("slot %q not found in %v", name, slots)
	return nil
}

// SEQ-16.5-P90: turn-pressure budget authority trust / session turn need-risk —
// validates that progression_ledger.world_pressure and injection_pack.budget_decisions
// expose need/risk markers without claiming canonical truth authority.
func TestSeq165P90TurnPressureBudgetAuthorityTrust(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 10, ChatSessionID: "seq165-p90", Name: "main arc", LastTurn: 4, Status: "active"},
		},
		returnWorldRules: []store.WorldRule{
			{ID: 20, ChatSessionID: "seq165-p90", Scope: "session", Category: "law", Key: "doors_need_keys", SourceTurn: 2},
		},
		returnPendingThreads: []store.PendingThread{
			{ID: 50, ChatSessionID: "seq165-p90", ThreadKey: "locked-door", CreatedTurn: 4, Status: "open"},
		},
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq165-p90", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 60, ChatSessionID: "seq165-p90", TurnIndex: 4, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p90","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	pl, ok := resp["progression_ledger"].(map[string]any)
	if !ok {
		t.Fatalf("missing progression_ledger")
	}
	wp, ok := pl["world_pressure"].(map[string]any)
	if !ok {
		t.Fatalf("missing world_pressure")
	}

	if wp["status"] != "structured_support" {
		t.Fatalf("world_pressure status=%v, want structured_support", wp["status"])
	}

	if wp["authority"] == "canonical" {
		t.Fatalf("world_pressure must not claim canonical authority")
	}

	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing injection_pack")
	}
	bd, ok := ip["budget_decisions"].(map[string]any)
	if !ok {
		t.Fatalf("missing budget_decisions")
	}
	if bd["canonical_state_hard_floor_enabled"] != true {
		t.Fatalf("canonical_state_hard_floor_enabled=%v, want true", bd["canonical_state_hard_floor_enabled"])
	}
}

// SEQ-16.5-P91: lane split preserve — validates that helper injection,
// input context, and governor logic are separate lanes with distinct budgets.
func TestSeq165P91LaneSplitPreserve(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq165-p91", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq165-p91", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p91","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing injection_pack")
	}
	if ip["injection_text"] == nil && ip["memory_text"] == nil {
		t.Fatalf("injection_pack must expose helper injection lane")
	}

	if ip["input_context_text"] == nil {
		t.Fatalf("injection_pack must expose input_context lane")
	}

	rt, ok := resp["runtime_toggle"].(map[string]any)
	if !ok {
		t.Fatalf("missing runtime_toggle")
	}
	if rt["version"] != "p212a.v1" {
		t.Fatalf("runtime_toggle version=%v, want p212a.v1", rt["version"])
	}

	if _, ok := rt["injection_enabled"]; !ok {
		t.Fatalf("runtime_toggle missing injection_enabled")
	}
	if _, ok := rt["input_context_enabled"]; !ok {
		t.Fatalf("runtime_toggle missing input_context_enabled")
	}
}

// SEQ-16.5-P92: dedupe — budget extend stale/dup/conflict cleanup — validates
// that injection_pack.trimmed and budget_decisions expose dedupe markers.
func TestSeq165P92DedupeBudgetExtendStaleDupConflict(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq165-p92", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
			{ID: 2, ChatSessionID: "seq165-p92", TurnIndex: 3, SummaryJSON: `{"text":"found a key"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq165-p92", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p92","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing injection_pack")
	}

	if _, ok := ip["trimmed"]; !ok {
		t.Fatalf("injection_pack missing trimmed key")
	}

	bd, ok := ip["budget_decisions"].(map[string]any)
	if !ok {
		t.Fatalf("missing budget_decisions")
	}
	if _, ok := bd["reason_counts"]; !ok {
		t.Fatalf("budget_decisions missing reason_counts")
	}

	blocks, _ := ip["section_blocks"].([]any)
	if blocks == nil {
		t.Fatalf("injection_pack.section_blocks must not be nil")
	}
}

// SEQ-16.5-P93: trace — budget inspectable — validates that generation_packet
// trace_summary and injection_pack budget_decisions are inspectable surfaces.
func TestSeq165P93TraceBudgetInspectable(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq165-p93", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq165-p93", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p93","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("missing generation_packet")
	}
	traceSummary, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("missing trace_summary")
	}

	for _, k := range []string{"max_injection_chars", "max_input_context_chars", "injection_truncated", "input_context_truncated"} {
		if _, ok := traceSummary[k]; !ok {
			t.Fatalf("trace_summary missing %q", k)
		}
	}

	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing injection_pack")
	}
	bd, ok := ip["budget_decisions"].(map[string]any)
	if !ok {
		t.Fatalf("missing budget_decisions")
	}
	for _, k := range []string{"max_injection_chars", "global_cap_chars", "final_budget_owner"} {
		if _, ok := bd[k]; !ok {
			t.Fatalf("budget_decisions missing %q", k)
		}
	}
	if bd["final_budget_owner"] != "archive_center_js_assembleInjectionWithBudget" {
		t.Fatalf("final_budget_owner=%v, want archive_center_js_assembleInjectionWithBudget", bd["final_budget_owner"])
	}
}

// SEQ-16.5-P94: support lane truth authority reorder preserve — validates that
// retrieval_extend_authority and progression_ledger.supporting_precedence_guard
// preserve support-only lane ordering without canonical takeover.
func TestSeq165P94SupportLaneTruthAuthorityReorderPreserve(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 10, ChatSessionID: "seq165-p94", Name: "main arc", LastTurn: 4},
		},
		returnWorldRules: []store.WorldRule{
			{ID: 20, ChatSessionID: "seq165-p94", Scope: "session", Category: "law", Key: "doors_need_keys", SourceTurn: 2},
		},
		returnCharStates: []store.CharacterState{
			{ID: 30, ChatSessionID: "seq165-p94", CharacterName: "Iris", TurnIndex: 3},
		},
		returnActiveStates: []store.ActiveState{
			{ID: 40, ChatSessionID: "seq165-p94", StateType: "scene", TurnIndex: 4},
		},
		returnPendingThreads: []store.PendingThread{
			{ID: 50, ChatSessionID: "seq165-p94", ThreadKey: "locked-door", CreatedTurn: 4},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 60, ChatSessionID: "seq165-p94", TurnIndex: 4, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p94","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	rea, ok := resp["retrieval_extend_authority"].(map[string]any)
	if !ok {
		t.Fatalf("missing retrieval_extend_authority")
	}
	if rea["version"] != "p168a.v1" {
		t.Fatalf("version=%v, want p168a.v1", rea["version"])
	}

	order, _ := rea["authority_order"].([]any)
	if len(order) < 4 || order[0] != "permanent" || order[1] != "session" || order[2] != "support" || order[3] != "fallback" {
		t.Fatalf("authority_order=%v, want [permanent session support fallback]", order)
	}

	pl, ok := resp["progression_ledger"].(map[string]any)
	if !ok {
		t.Fatalf("missing progression_ledger")
	}
	guard, ok := pl["supporting_precedence_guard"].(map[string]any)
	if !ok {
		t.Fatalf("missing supporting_precedence_guard")
	}
	if guard["supporting_only"] != true {
		t.Fatalf("supporting_only=%v, want true", guard["supporting_only"])
	}
	if guard["cannot_override_current_user_input"] != true {
		t.Fatalf("cannot_override_current_user_input=%v, want true", guard["cannot_override_current_user_input"])
	}
	if guard["cannot_override_verified_direct_evidence"] != true {
		t.Fatalf("cannot_override_verified_direct_evidence=%v, want true", guard["cannot_override_verified_direct_evidence"])
	}
}

// SEQ-16.5-P98: need signal inventory — validates that progression_ledger
// exposes unresolved_tensions, payoffs, and scene_deltas as need signals.
func TestSeq165P98NeedSignalInventory(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 10, ChatSessionID: "seq165-p98", Name: "main arc", LastTurn: 4, Status: "active", OngoingTensionsJSON: `["locked door","missing key"]`},
		},
		returnPendingThreads: []store.PendingThread{
			{ID: 50, ChatSessionID: "seq165-p98", ThreadKey: "locked-door", CreatedTurn: 4, Status: "open", Description: "find the key"},
		},
		returnEpisodeSums: []store.EpisodeSummary{
			{ID: 70, ChatSessionID: "seq165-p98", FromTurn: 1, ToTurn: 4, SummaryText: "searching for key"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 60, ChatSessionID: "seq165-p98", TurnIndex: 4, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p98","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	pl, ok := resp["progression_ledger"].(map[string]any)
	if !ok {
		t.Fatalf("missing progression_ledger")
	}

	ut, _ := pl["unresolved_tensions"].([]any)
	if len(ut) == 0 {
		t.Fatalf("unresolved_tensions must not be empty")
	}

	if _, ok := pl["payoffs"]; !ok {
		t.Fatalf("missing payoffs")
	}

	if _, ok := pl["scene_deltas"]; !ok {
		t.Fatalf("missing scene_deltas")
	}

	for _, raw := range ut {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if _, ok := entry["pressure_score"]; !ok {
			t.Fatalf("unresolved_tension missing pressure_score: %v", entry)
		}
		if _, ok := entry["lifecycle_state"]; !ok {
			t.Fatalf("unresolved_tension missing lifecycle_state: %v", entry)
		}
	}
}

// SEQ-16.5-P99: risk signal inventory — validates that progression_ledger
// exposes world_pressure and consequences as risk signals.
func TestSeq165P99RiskSignalInventory(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 10, ChatSessionID: "seq165-p99", Name: "main arc", LastTurn: 4, Status: "active"},
		},
		returnWorldRules: []store.WorldRule{
			{ID: 20, ChatSessionID: "seq165-p99", Scope: "session", Category: "law", Key: "doors_need_keys", SourceTurn: 2},
		},
		returnPendingThreads: []store.PendingThread{
			{ID: 50, ChatSessionID: "seq165-p99", ThreadKey: "locked-door", CreatedTurn: 4, Status: "open"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 60, ChatSessionID: "seq165-p99", TurnIndex: 4, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p99","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	pl, ok := resp["progression_ledger"].(map[string]any)
	if !ok {
		t.Fatalf("missing progression_ledger")
	}

	wp, ok := pl["world_pressure"].(map[string]any)
	if !ok {
		t.Fatalf("missing world_pressure")
	}

	if wp["status"] != "structured_support" {
		t.Fatalf("world_pressure status=%v, want structured_support", wp["status"])
	}

	if _, ok := pl["consequences"]; !ok {
		t.Fatalf("missing consequences")
	}

	if wp["authority"] == "canonical" {
		t.Fatalf("world_pressure must not claim canonical authority")
	}
}

// SEQ-16.5-P100: helper budget vs input anchor budget — validates that
// injection_pack.budget_decisions distinguishes helper injection budget
// from input context anchor budget.
func TestSeq165P100HelperBudgetVsInputAnchorBudget(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq165-p100", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq165-p100", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p100","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing injection_pack")
	}
	bd, ok := ip["budget_decisions"].(map[string]any)
	if !ok {
		t.Fatalf("missing budget_decisions")
	}

	if _, ok := bd["max_injection_chars"]; !ok {
		t.Fatalf("budget_decisions missing max_injection_chars (helper budget)")
	}

	if ip["input_context_text"] == nil {
		t.Fatalf("input_context_text must not be nil (input anchor lane)")
	}

	if ip["apply_verdict"] != "shadow_only" {
		t.Fatalf("apply_verdict=%v, want shadow_only", ip["apply_verdict"])
	}
}

// SEQ-16.5-P101: runtime token hint demotion policy — validates that
// generation_packet.trace_summary exposes runtime_token_profile with
// demotion markers and that low-confidence hints do not claim primary authority.
func TestSeq165P101RuntimeTokenHintDemotionPolicy(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq165-p101", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq165-p101", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p101","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("missing generation_packet")
	}
	traceSummary, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("missing trace_summary")
	}

	rtp, ok := traceSummary["runtime_token_profile"].(map[string]any)
	if !ok {
		t.Fatalf("missing runtime_token_profile")
	}
	if rtp["version"] != "p61a.v1" {
		t.Fatalf("runtime_token_profile version=%v, want p61a.v1", rtp["version"])
	}
	if rtp["status"] != "shadow_only" {
		t.Fatalf("runtime_token_profile status=%v, want shadow_only", rtp["status"])
	}

	if rtp["profile_source"] != "client_meta_shadow" {
		t.Fatalf("profile_source=%v, want client_meta_shadow", rtp["profile_source"])
	}

	profile, _ := rtp["context_window_profile"].(string)
	if profile == "default" && rtp["auto_optimized"] != false {
		t.Fatalf("auto_optimized=%v, want false for default profile", rtp["auto_optimized"])
	}
}

// SEQ-16.5-P102: dominant-arc saturation / resolved-afterglow risk signal —
// validates that progression_ledger.lifecycle_model exposes saturation/afterglow
// states and that resolved storylines are handled correctly.
func TestSeq165P102DominantArcSaturationResolvedAfterglow(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 10, ChatSessionID: "seq165-p102", Name: "main arc", LastTurn: 4, Status: "resolved"},
			{ID: 11, ChatSessionID: "seq165-p102", Name: "side arc", LastTurn: 3, Status: "active"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 60, ChatSessionID: "seq165-p102", TurnIndex: 4, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p102","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	pl, ok := resp["progression_ledger"].(map[string]any)
	if !ok {
		t.Fatalf("missing progression_ledger")
	}
	lm, ok := pl["lifecycle_model"].(map[string]any)
	if !ok {
		t.Fatalf("missing lifecycle_model")
	}

	states, _ := lm["states"].([]any)
	if len(states) == 0 {
		t.Fatalf("lifecycle_model.states must not be empty")
	}
	hasResolved := false
	hasDormant := false
	for _, s := range states {
		if s == "resolved" {
			hasResolved = true
		}
		if s == "dormant" {
			hasDormant = true
		}
	}
	if !hasResolved {
		t.Fatalf("lifecycle_model.states missing 'resolved'")
	}
	if !hasDormant {
		t.Fatalf("lifecycle_model.states missing 'dormant'")
	}

	decay, _ := lm["decay_rules"].(map[string]any)
	if decay == nil {
		t.Fatalf("lifecycle_model missing decay_rules")
	}
	if resolvedDecay, ok := decay["resolved"].(float64); !ok || resolvedDecay >= 4 {
		t.Fatalf("resolved decay=%v, want < 4 (afterglow suppression)", resolvedDecay)
	}

	ut, _ := pl["unresolved_tensions"].([]any)
	for _, raw := range ut {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if source, _ := entry["source"].(string); strings.Contains(source, "storyline") {
			if status, _ := entry["status"].(string); status == "open" {

				if label, _ := entry["label"].(string); label == "main arc" {
					t.Fatalf("resolved storyline 'main arc' should not produce open tensions")
				}
			}
		}
	}
}

// SEQ-16.5-P106: helper injection base budget / floor / ceiling — validates
// that injection_pack.budget_decisions exposes base budget, floor, and ceiling.
func TestSeq165P106HelperInjectionBaseBudgetFloorCeiling(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq165-p106", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq165-p106", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p106","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing injection_pack")
	}
	bd, ok := ip["budget_decisions"].(map[string]any)
	if !ok {
		t.Fatalf("missing budget_decisions")
	}

	capChars, ok := bd["max_injection_chars"].(float64)
	if !ok || capChars <= 0 {
		t.Fatalf("max_injection_chars=%v, want > 0", bd["max_injection_chars"])
	}

	floor, ok := bd["canon_floor_reserved_chars"].(float64)
	if !ok || floor <= 0 {
		t.Fatalf("canon_floor_reserved_chars=%v, want > 0", bd["canon_floor_reserved_chars"])
	}

	if capChars < floor {
		t.Fatalf("max_injection_chars (%v) < canon_floor_reserved_chars (%v)", capChars, floor)
	}

	blocks, _ := ip["section_blocks"].([]any)
	if len(blocks) == 0 {
		t.Fatalf("section_blocks must not be empty")
	}
	for _, raw := range blocks {
		block, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if _, ok := block["budget"]; !ok {
			t.Fatalf("section_block missing budget: %v", block)
		}
	}
}

// SEQ-16.5-P107: lane floor / ceiling / redistribution — validates that
// injection_pack.section_blocks expose per-lane floor/ceiling and that
// redistribution is traceable via trimmed.
func TestSeq165P107LaneFloorCeilingRedistribution(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq165-p107", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq165-p107", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p107","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing injection_pack")
	}
	blocks, _ := ip["section_blocks"].([]any)
	if len(blocks) == 0 {
		t.Fatalf("section_blocks must not be empty")
	}

	for _, raw := range blocks {
		block, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		chars, ok := block["chars"].(float64)
		if !ok {
			t.Fatalf("section_block missing chars: %v", block)
		}
		budget, ok := block["budget"].(float64)
		if !ok {
			t.Fatalf("section_block missing budget: %v", block)
		}

		if chars > budget {
			t.Fatalf("section_block chars (%v) > budget (%v): %v", chars, budget, block)
		}
	}

	if _, ok := ip["trimmed"]; !ok {
		t.Fatalf("injection_pack missing trimmed key")
	}
}

// SEQ-16.5-P108: dedupe-first trim order — validates that injection_pack.trimmed
// exposes trim order with dedupe-first policy.
func TestSeq165P108DedupeFirstTrimOrder(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq165-p108", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
			{ID: 2, ChatSessionID: "seq165-p108", TurnIndex: 3, SummaryJSON: `{"text":"found a key"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq165-p108", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p108","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing injection_pack")
	}

	if _, ok := ip["trimmed"]; !ok {
		t.Fatalf("injection_pack missing trimmed key")
	}

	trimmed, _ := ip["trimmed"].([]any)
	for _, raw := range trimmed {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if _, ok := entry["reason"]; !ok {
			t.Fatalf("trim entry missing reason: %v", entry)
		}
		if _, ok := entry["label"]; !ok {
			t.Fatalf("trim entry missing label: %v", entry)
		}
	}

	bd, ok := ip["budget_decisions"].(map[string]any)
	if !ok {
		t.Fatalf("missing budget_decisions")
	}
	if _, ok := bd["reason_counts"]; !ok {
		t.Fatalf("budget_decisions missing reason_counts")
	}
}

// SEQ-16.5-P109: high-need + high-risk conservative shrink rule — validates
// that when world_pressure signals high risk, the budget remains conservative
// (no broad takeover, no expansion beyond cap).
func TestSeq165P109HighNeedHighRiskConservativeShrinkRule(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 10, ChatSessionID: "seq165-p109", Name: "main arc", LastTurn: 4, Status: "active", OngoingTensionsJSON: `["locked door","missing key","trap ahead"]`},
		},
		returnWorldRules: []store.WorldRule{
			{ID: 20, ChatSessionID: "seq165-p109", Scope: "session", Category: "law", Key: "doors_need_keys", SourceTurn: 2},
			{ID: 21, ChatSessionID: "seq165-p109", Scope: "global", Category: "danger", Key: "trap_damage", SourceTurn: 3},
		},
		returnPendingThreads: []store.PendingThread{
			{ID: 50, ChatSessionID: "seq165-p109", ThreadKey: "locked-door", CreatedTurn: 4, Status: "open"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 60, ChatSessionID: "seq165-p109", TurnIndex: 4, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p109","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	pl, ok := resp["progression_ledger"].(map[string]any)
	if !ok {
		t.Fatalf("missing progression_ledger")
	}
	ut, _ := pl["unresolved_tensions"].([]any)
	if len(ut) == 0 {
		t.Fatalf("unresolved_tensions must not be empty (high need)")
	}

	if _, ok := pl["world_pressure"]; !ok {
		t.Fatalf("missing world_pressure")
	}

	rt, ok := resp["runtime_toggle"].(map[string]any)
	if !ok {
		t.Fatalf("missing runtime_toggle")
	}
	if rt["broad_takeover"] != false {
		t.Fatalf("broad_takeover=%v, want false (conservative shrink)", rt["broad_takeover"])
	}

	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing injection_pack")
	}
	bd, ok := ip["budget_decisions"].(map[string]any)
	if !ok {
		t.Fatalf("missing budget_decisions")
	}
	capChars, _ := bd["max_injection_chars"].(float64)
	if capChars <= 0 {
		t.Fatalf("max_injection_chars=%v, want > 0", capChars)
	}

	if ip["apply_verdict"] != "shadow_only" {
		t.Fatalf("apply_verdict=%v, want shadow_only", ip["apply_verdict"])
	}
}

// SEQ-16.5-P110: manual setting / adaptive governor / optional telemetry cap
// relationship — validates that runtime_toggle, autonomy_plan, and trace_summary
// expose manual/adaptive/telemetry surfaces without claiming canonical authority.
func TestSeq165P110ManualSettingAdaptiveGovernorTelemetryCap(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq165-p110", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq165-p110", TurnIndex: 4, Role: "user", Content: "open the door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p110","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	rt, ok := resp["runtime_toggle"].(map[string]any)
	if !ok {
		t.Fatalf("missing runtime_toggle")
	}
	if _, ok := rt["injection_enabled"]; !ok {
		t.Fatalf("runtime_toggle missing injection_enabled (manual setting)")
	}
	if _, ok := rt["input_context_enabled"]; !ok {
		t.Fatalf("runtime_toggle missing input_context_enabled (manual setting)")
	}

	ap, ok := resp["autonomy_plan"].(map[string]any)
	if !ok {
		t.Fatalf("missing autonomy_plan")
	}

	if ap["status"] != "ready" && ap["status"] != "degraded" {
		t.Fatalf("autonomy_plan status=%v, want ready or degraded", ap["status"])
	}
	if ap["would_call_llm"] != false {
		t.Fatalf("autonomy_plan would_call_llm=%v, want false", ap["would_call_llm"])
	}

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("missing generation_packet")
	}
	traceSummary, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("missing trace_summary")
	}
	for _, k := range []string{"memory_count", "evidence_count", "chat_log_count"} {
		if _, ok := traceSummary[k]; !ok {
			t.Fatalf("trace_summary missing %q (telemetry cap)", k)
		}
	}

	if rt["truth_store"] != "maria_db" {
		t.Fatalf("runtime_toggle truth_store=%v, want maria_db", rt["truth_store"])
	}
	if rt["retrieval_role"] != "support_accelerator_only" {
		t.Fatalf("runtime_toggle retrieval_role=%v, want support_accelerator_only", rt["retrieval_role"])
	}
}

// SEQ-16.5-P114: mandatory anchor slot define — [Temporal Anchor] / [Previous]
// validates that input_context_text contains mandatory anchor slots when
// resumePack or recent chat logs are present.
func TestSeq165P114MandatoryAnchorSlot(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq165-p114", TurnIndex: 3, Role: "user", Content: "hello again"},
			{ID: 2, ChatSessionID: "seq165-p114", TurnIndex: 4, Role: "assistant", Content: "hi there"},
		},
		returnResumePack: &store.ResumePack{
			Trigger: "long_gap", AssembledText: "Previous session summary here.",
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p114","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	ict, ok := resp["input_context_text"].(string)
	if !ok || ict == "" {
		t.Fatalf("input_context_text missing or empty")
	}

	if !strings.Contains(ict, "[Recent Chat]") {
		t.Fatalf("input_context_text missing [Recent Chat] mandatory slot")
	}
	if !strings.Contains(ict, "[Resume Pack]") {
		t.Fatalf("input_context_text missing [Resume Pack] mandatory slot")
	}

	cp, ok := resp["continuity_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing continuity_pack")
	}
	items, _ := cp["items"].([]any)
	if len(items) == 0 {
		t.Fatalf("continuity_pack.items must not be empty")
	}
	gov := seq165Map(t, resp, "input_anchor_governor")
	if gov["version"] != "seq16_5_input_anchor_governor.v1" {
		t.Fatalf("input_anchor_governor version=%v", gov["version"])
	}
	mandatory, _ := gov["mandatory_slots"].([]any)
	temporal := seq165FindSlot(t, mandatory, "Temporal Anchor")
	if temporal["marker"] != "[Temporal Anchor]" || temporal["mapped_section"] != "[Recent Chat]" || temporal["selected"] != true {
		t.Fatalf("Temporal Anchor slot mismatch: %v", temporal)
	}
	previous := seq165FindSlot(t, mandatory, "Previous")
	if previous["marker"] != "[Previous]" || previous["mapped_section"] != "[Resume Pack]" || previous["selected"] != true {
		t.Fatalf("Previous slot mismatch: %v", previous)
	}
}
