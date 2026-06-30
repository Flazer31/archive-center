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
	// world_pressure status must be structured_support (not canonical).
	if wp["status"] != "structured_support" {
		t.Fatalf("world_pressure status=%v, want structured_support", wp["status"])
	}
	// Must not claim canonical truth authority.
	if wp["authority"] == "canonical" {
		t.Fatalf("world_pressure must not claim canonical authority")
	}
	// injection_pack budget decisions must be present.
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
	// Helper injection lane.
	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing injection_pack")
	}
	if ip["injection_text"] == nil && ip["memory_text"] == nil {
		t.Fatalf("injection_pack must expose helper injection lane")
	}
	// Input context lane.
	if ip["input_context_text"] == nil {
		t.Fatalf("injection_pack must expose input_context lane")
	}
	// Governor logic: runtime_toggle must be present.
	rt, ok := resp["runtime_toggle"].(map[string]any)
	if !ok {
		t.Fatalf("missing runtime_toggle")
	}
	if rt["version"] != "p212a.v1" {
		t.Fatalf("runtime_toggle version=%v, want p212a.v1", rt["version"])
	}
	// Both lanes must be independently toggleable.
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
			{ID: 2, ChatSessionID: "seq165-p92", TurnIndex: 3, SummaryJSON: `{"text":"found a key"}`}, // duplicate
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
	// trimmed key must exist (value may be nil when no trimming occurs).
	if _, ok := ip["trimmed"]; !ok {
		t.Fatalf("injection_pack missing trimmed key")
	}
	// budget_decisions must expose reason_counts for dedupe tracking.
	bd, ok := ip["budget_decisions"].(map[string]any)
	if !ok {
		t.Fatalf("missing budget_decisions")
	}
	if _, ok := bd["reason_counts"]; !ok {
		t.Fatalf("budget_decisions missing reason_counts")
	}
	// section_blocks must be present for lane-level dedupe inspection.
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
	// Budget inspectability: max_injection_chars and max_input_context_chars must be present.
	for _, k := range []string{"max_injection_chars", "max_input_context_chars", "injection_truncated", "input_context_truncated"} {
		if _, ok := traceSummary[k]; !ok {
			t.Fatalf("trace_summary missing %q", k)
		}
	}
	// injection_pack budget decisions must be inspectable.
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
	// Authority order: permanent > session > support > fallback.
	order, _ := rea["authority_order"].([]any)
	if len(order) < 4 || order[0] != "permanent" || order[1] != "session" || order[2] != "support" || order[3] != "fallback" {
		t.Fatalf("authority_order=%v, want [permanent session support fallback]", order)
	}
	// progression_ledger supporting_precedence_guard must enforce support-only.
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
	// Need signals: unresolved_tensions must be present and non-empty.
	ut, _ := pl["unresolved_tensions"].([]any)
	if len(ut) == 0 {
		t.Fatalf("unresolved_tensions must not be empty")
	}
	// payoffs must be present.
	if _, ok := pl["payoffs"]; !ok {
		t.Fatalf("missing payoffs")
	}
	// scene_deltas must be present.
	if _, ok := pl["scene_deltas"]; !ok {
		t.Fatalf("missing scene_deltas")
	}
	// Each unresolved tension must have need markers.
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
	// Risk signals: world_pressure must be present.
	wp, ok := pl["world_pressure"].(map[string]any)
	if !ok {
		t.Fatalf("missing world_pressure")
	}
	// world_pressure status must be structured_support (not canonical).
	if wp["status"] != "structured_support" {
		t.Fatalf("world_pressure status=%v, want structured_support", wp["status"])
	}
	// consequences must be present.
	if _, ok := pl["consequences"]; !ok {
		t.Fatalf("missing consequences")
	}
	// Must not claim canonical truth authority.
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
	// Helper budget: max_injection_chars is the helper injection cap.
	if _, ok := bd["max_injection_chars"]; !ok {
		t.Fatalf("budget_decisions missing max_injection_chars (helper budget)")
	}
	// Input anchor budget: input_context is a separate lane.
	if ip["input_context_text"] == nil {
		t.Fatalf("input_context_text must not be nil (input anchor lane)")
	}
	// Both must be shadow-only.
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
	// runtime_token_profile must be present.
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
	// Demotion: profile_source is client_meta_shadow (not primary authority).
	if rtp["profile_source"] != "client_meta_shadow" {
		t.Fatalf("profile_source=%v, want client_meta_shadow", rtp["profile_source"])
	}
	// Low-confidence: auto_optimized must be false when profile is default.
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
	// Lifecycle model must include resolved and dormant states (afterglow handling).
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
	// Decay rules must assign lower weight to resolved/dormant (afterglow suppression).
	decay, _ := lm["decay_rules"].(map[string]any)
	if decay == nil {
		t.Fatalf("lifecycle_model missing decay_rules")
	}
	if resolvedDecay, ok := decay["resolved"].(float64); !ok || resolvedDecay >= 4 {
		t.Fatalf("resolved decay=%v, want < 4 (afterglow suppression)", resolvedDecay)
	}
	// Resolved storyline must not dominate unresolved tensions.
	ut, _ := pl["unresolved_tensions"].([]any)
	for _, raw := range ut {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if source, _ := entry["source"].(string); strings.Contains(source, "storyline") {
			if status, _ := entry["status"].(string); status == "open" {
				// Active storyline still produces open tensions; resolved one should not.
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
	// Base budget: max_injection_chars is the ceiling.
	capChars, ok := bd["max_injection_chars"].(float64)
	if !ok || capChars <= 0 {
		t.Fatalf("max_injection_chars=%v, want > 0", bd["max_injection_chars"])
	}
	// Floor: canon_floor_reserved_chars must be > 0.
	floor, ok := bd["canon_floor_reserved_chars"].(float64)
	if !ok || floor <= 0 {
		t.Fatalf("canon_floor_reserved_chars=%v, want > 0", bd["canon_floor_reserved_chars"])
	}
	// Base budget must be >= floor.
	if capChars < floor {
		t.Fatalf("max_injection_chars (%v) < canon_floor_reserved_chars (%v)", capChars, floor)
	}
	// section_blocks must show per-lane budgets.
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
	// Each block must have chars and budget (ceiling).
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
		// chars must be <= budget (lane ceiling enforced).
		if chars > budget {
			t.Fatalf("section_block chars (%v) > budget (%v): %v", chars, budget, block)
		}
	}
	// Redistribution trace: trimmed key must exist.
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
			{ID: 2, ChatSessionID: "seq165-p108", TurnIndex: 3, SummaryJSON: `{"text":"found a key"}`}, // duplicate
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
	// trimmed key must exist (value may be nil when no trimming occurs).
	if _, ok := ip["trimmed"]; !ok {
		t.Fatalf("injection_pack missing trimmed key")
	}
	// Trim entries must have reason and label for order inspection.
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
	// budget_decisions must expose reason_counts for dedupe tracking.
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
	// High need: unresolved_tensions must be present.
	pl, ok := resp["progression_ledger"].(map[string]any)
	if !ok {
		t.Fatalf("missing progression_ledger")
	}
	ut, _ := pl["unresolved_tensions"].([]any)
	if len(ut) == 0 {
		t.Fatalf("unresolved_tensions must not be empty (high need)")
	}
	// High risk: world_pressure must be present.
	if _, ok := pl["world_pressure"]; !ok {
		t.Fatalf("missing world_pressure")
	}
	// Conservative shrink: broad_takeover must be false.
	rt, ok := resp["runtime_toggle"].(map[string]any)
	if !ok {
		t.Fatalf("missing runtime_toggle")
	}
	if rt["broad_takeover"] != false {
		t.Fatalf("broad_takeover=%v, want false (conservative shrink)", rt["broad_takeover"])
	}
	// Budget must not expand beyond cap.
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
	// apply_verdict must be shadow_only (no expansion).
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
	// Manual setting: runtime_toggle exposes injection_enabled and input_context_enabled.
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
	// Adaptive governor: autonomy_plan must be present.
	ap, ok := resp["autonomy_plan"].(map[string]any)
	if !ok {
		t.Fatalf("missing autonomy_plan")
	}
	// autonomy_plan status is ready/degraded (not versioned as p72a.v1 in Go surface).
	if ap["status"] != "ready" && ap["status"] != "degraded" {
		t.Fatalf("autonomy_plan status=%v, want ready or degraded", ap["status"])
	}
	if ap["would_call_llm"] != false {
		t.Fatalf("autonomy_plan would_call_llm=%v, want false", ap["would_call_llm"])
	}
	// Optional telemetry cap: trace_summary must expose counts without claiming authority.
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
	// All must be shadow-only.
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
	// Mandatory anchor slots must be present.
	if !strings.Contains(ict, "[Recent Chat]") {
		t.Fatalf("input_context_text missing [Recent Chat] mandatory slot")
	}
	if !strings.Contains(ict, "[Resume Pack]") {
		t.Fatalf("input_context_text missing [Resume Pack] mandatory slot")
	}
	// continuity_pack must also expose resume and chat items.
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

// SEQ-16.5-P115: optional anchor slot define — [Scene], [Entity], [Active Thread], [Chapter], [Saga]
// validates that input_context_text may contain optional anchor slots when
// activeStates, canonicalLayers, or episodeSums are present.
func TestSeq165P115OptionalAnchorSlot(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnActiveStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "seq165-p115", StateType: "scene", TurnIndex: 2, Content: "dark forest"},
			{ID: 2, ChatSessionID: "seq165-p115", StateType: "entity", TurnIndex: 3, Content: "mysterious stranger"},
		},
		returnCanonicalLayers: []store.CanonicalStateLayer{
			{ID: 10, ChatSessionID: "seq165-p115", LayerType: "world_state", TurnIndex: 1, Content: "raining"},
		},
		returnEpisodeSums: []store.EpisodeSummary{
			{ID: 20, ChatSessionID: "seq165-p115", FromTurn: 1, ToTurn: 3, SummaryText: "entering the woods"},
		},
		returnPendingThreads: []store.PendingThread{
			{ID: 25, ChatSessionID: "seq165-p115", ThreadKey: "north-path", CreatedTurn: 3, Status: "open"},
		},
		returnStorylines: []store.Storyline{
			{ID: 26, ChatSessionID: "seq165-p115", Name: "forest saga", LastTurn: 3, Status: "active"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 30, ChatSessionID: "seq165-p115", TurnIndex: 4, Role: "user", Content: "go north"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p115","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	// Optional anchor slots must be present when data exists.
	if !strings.Contains(ict, "[Active States]") {
		t.Fatalf("input_context_text missing [Active States] optional slot")
	}
	if !strings.Contains(ict, "[Canonical State Layers]") {
		t.Fatalf("input_context_text missing [Canonical State Layers] optional slot")
	}
	if !strings.Contains(ict, "[Episode Summaries]") {
		t.Fatalf("input_context_text missing [Episode Summaries] optional slot")
	}
	// continuity_pack must expose active_state and canonical_layer items.
	cp, ok := resp["continuity_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing continuity_pack")
	}
	items, _ := cp["items"].([]any)
	foundActive := false
	foundCanon := false
	foundEpisode := false
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		kind, _ := item["kind"].(string)
		switch kind {
		case "active_state":
			foundActive = true
		case "canonical_layer":
			foundCanon = true
		case "episode_summary":
			foundEpisode = true
		}
	}
	if !foundActive {
		t.Fatalf("continuity_pack.items missing active_state")
	}
	if !foundCanon {
		t.Fatalf("continuity_pack.items missing canonical_layer")
	}
	if !foundEpisode {
		t.Fatalf("continuity_pack.items missing episode_summary")
	}
	gov := seq165Map(t, resp, "input_anchor_governor")
	optional, _ := gov["optional_slots"].([]any)
	for _, name := range []string{"Scene", "Entity", "Active Thread", "Chapter", "Saga"} {
		slot := seq165FindSlot(t, optional, name)
		if slot["marker"] != "["+name+"]" {
			t.Fatalf("%s marker mismatch: %v", name, slot)
		}
		if slot["selected"] != true {
			t.Fatalf("%s slot selected=%v, want true: %v", name, slot["selected"], slot)
		}
	}
}

// SEQ-16.5-P116: weak-input / temporal / resume / explicit-user-input slot promotion/demotion
// validates that trace_preview and generation_packet.trace_summary expose
// slot promotion/demotion decisions (e.g., storyline_selected_count vs dropped_count).
func TestSeq165P116SlotPromotionDemotion(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq165-p116", Name: "main arc", LastTurn: 10, Status: "active"},
			{ID: 2, ChatSessionID: "seq165-p116", Name: "old arc", LastTurn: 2, Status: "dormant"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 10, ChatSessionID: "seq165-p116", TurnIndex: 11, Role: "user", Content: "continue"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p116","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	// trace_preview must expose storyline selection with promotion/demotion counts.
	tp, ok := resp["trace_preview"].(map[string]any)
	if !ok {
		t.Fatalf("missing trace_preview")
	}
	ss, ok := tp["storyline_selection"].(map[string]any)
	if !ok {
		t.Fatalf("missing trace_preview.storyline_selection")
	}
	if _, ok := ss["selected_count"]; !ok {
		t.Fatalf("storyline_selection missing selected_count")
	}
	if _, ok := ss["dropped_count"]; !ok {
		t.Fatalf("storyline_selection missing dropped_count")
	}
	// generation_packet trace_summary must also expose counts.
	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("missing generation_packet")
	}
	traceSummary, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("missing trace_summary")
	}
	if _, ok := traceSummary["storyline_selected_count"]; !ok {
		t.Fatalf("trace_summary missing storyline_selected_count")
	}
	if _, ok := traceSummary["storyline_dropped_count"]; !ok {
		t.Fatalf("trace_summary missing storyline_dropped_count")
	}
	if _, ok := traceSummary["storyline_stale_dropped_count"]; !ok {
		t.Fatalf("trace_summary missing storyline_stale_dropped_count")
	}
	gov := seq165Map(t, resp, "input_anchor_governor")
	rules := seq165Map(t, gov, "promotion_demotion_rules")
	for _, key := range []string{"weak_input", "temporal_query", "resume", "explicit_user_input"} {
		if _, ok := rules[key]; !ok {
			t.Fatalf("promotion_demotion_rules missing %q", key)
		}
	}
}

// SEQ-16.5-P117: input context max slot / max char policy — short-and-sharp anchor lane preserve
// validates that generation_packet.trace_summary exposes max_input_context_chars
// and input_context_truncated flags.
func TestSeq165P117InputContextMaxSlotMaxCharPolicy(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq165-p117", TurnIndex: 1, Role: "user", Content: strings.Repeat("a", 500)},
			{ID: 2, ChatSessionID: "seq165-p117", TurnIndex: 2, Role: "user", Content: strings.Repeat("b", 500)},
			{ID: 3, ChatSessionID: "seq165-p117", TurnIndex: 3, Role: "user", Content: strings.Repeat("c", 500)},
			{ID: 4, ChatSessionID: "seq165-p117", TurnIndex: 4, Role: "user", Content: strings.Repeat("d", 500)},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p117","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_input_context_chars":400}}`
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
	maxChars, ok := traceSummary["max_input_context_chars"].(float64)
	if !ok || maxChars <= 0 {
		t.Fatalf("max_input_context_chars=%v, want > 0", traceSummary["max_input_context_chars"])
	}
	if maxChars != 400 {
		t.Fatalf("max_input_context_chars=%v, want 400", maxChars)
	}
	// With large chat logs and low cap, truncation should occur.
	if traceSummary["input_context_truncated"] != true {
		t.Fatalf("input_context_truncated=%v, want true", traceSummary["input_context_truncated"])
	}
	// input_context_text must still be present and non-empty.
	ict, ok := resp["input_context_text"].(string)
	if !ok || ict == "" {
		t.Fatalf("input_context_text missing or empty")
	}
	gov := seq165Map(t, resp, "input_anchor_governor")
	policy := seq165Map(t, gov, "slot_policy")
	if policy["max_chars"] != float64(400) {
		t.Fatalf("slot_policy.max_chars=%v, want 400", policy["max_chars"])
	}
	if policy["input_context_truncated"] != true {
		t.Fatalf("slot_policy.input_context_truncated=%v, want true", policy["input_context_truncated"])
	}
	if policy["short_and_sharp_anchor_lane_preserve"] != true {
		t.Fatalf("slot_policy short_and_sharp_anchor_lane_preserve=%v, want true", policy["short_and_sharp_anchor_lane_preserve"])
	}
}

// SEQ-16.5-P118: helper injection anchor suppression — validates that
// injection_pack helper text does not contain anchor slot markers, and that
// input_context and injection are separate lanes.
func TestSeq165P118HelperInjectionAnchorSuppression(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq165-p118", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 10, ChatSessionID: "seq165-p118", TurnIndex: 4, Role: "user", Content: "open door"},
		},
		returnResumePack: &store.ResumePack{
			Trigger: "resume", AssembledText: "Resume text here.",
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p118","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	// Helper injection text must not contain anchor slot markers.
	injText, _ := ip["injection_text"].(string)
	for _, marker := range []string{"[Temporal Anchor]", "[Previous]", "[Scene]", "[Entity]", "[Active Thread]", "[Chapter]", "[Saga]", "[Resume Pack]", "[Direct Evidence]", "[Recent Chat]", "[Active States]", "[Canonical State Layers]", "[Episode Summaries]"} {
		if strings.Contains(injText, marker) {
			t.Fatalf("injection_text contains anchor marker %q (helper must be suppressed from anchors)", marker)
		}
	}
	// input_context_text must exist as a separate lane.
	if _, ok := resp["input_context_text"]; !ok {
		t.Fatalf("missing input_context_text (input context lane must be separate)")
	}
	// apply_verdict must be shadow_only (no helper takeover of anchors).
	if ip["apply_verdict"] != "shadow_only" {
		t.Fatalf("apply_verdict=%v, want shadow_only", ip["apply_verdict"])
	}
	gov := seq165Map(t, resp, "input_anchor_governor")
	suppression := seq165Map(t, gov, "helper_injection_anchor_suppression")
	if suppression["enabled"] != true {
		t.Fatalf("helper_injection_anchor_suppression.enabled=%v, want true", suppression["enabled"])
	}
	suppressed, _ := suppression["suppressed_markers"].([]any)
	seq165FindSlotLikeMarker := false
	for _, marker := range suppressed {
		if marker == "[Temporal Anchor]" {
			seq165FindSlotLikeMarker = true
		}
	}
	if !seq165FindSlotLikeMarker {
		t.Fatalf("suppressed_markers missing [Temporal Anchor]: %v", suppressed)
	}
}

// SEQ-16.5-P119: explicit user redirection stale arc anchor demotion — validates that
// resolved/dormant storylines do not dominate input_context or injection_pack,
// and that progression_ledger.lifecycle_model handles stale arcs.
func TestSeq165P119ExplicitUserRedirectionStaleArcAnchorDemotion(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq165-p119", Name: "current arc", LastTurn: 10, Status: "active"},
			{ID: 2, ChatSessionID: "seq165-p119", Name: "stale arc", LastTurn: 2, Status: "resolved"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 10, ChatSessionID: "seq165-p119", TurnIndex: 11, Role: "user", Content: "let's move on"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p119","turn_index":1,"raw_user_input":"let's move on","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	// progression_ledger lifecycle_model must include resolved/dormant states.
	pl, ok := resp["progression_ledger"].(map[string]any)
	if !ok {
		t.Fatalf("missing progression_ledger")
	}
	lm, ok := pl["lifecycle_model"].(map[string]any)
	if !ok {
		t.Fatalf("missing lifecycle_model")
	}
	states, _ := lm["states"].([]any)
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
	// storyline_selection must drop stale arcs.
	tp, ok := resp["trace_preview"].(map[string]any)
	if !ok {
		t.Fatalf("missing trace_preview")
	}
	ss, ok := tp["storyline_selection"].(map[string]any)
	if !ok {
		t.Fatalf("missing trace_preview.storyline_selection")
	}
	if ss["dropped_count"] == 0 {
		t.Fatalf("storyline_selection dropped_count=%v, want > 0 (stale arc should be dropped)", ss["dropped_count"])
	}
	// injection_pack must not claim canonical authority.
	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing injection_pack")
	}
	if ip["apply_verdict"] != "shadow_only" {
		t.Fatalf("apply_verdict=%v, want shadow_only", ip["apply_verdict"])
	}
	gov := seq165Map(t, resp, "input_anchor_governor")
	redirection := seq165Map(t, gov, "explicit_user_redirection")
	if redirection["detected"] != true || redirection["stale_arc_demotes"] != true || redirection["current_user_input_wins"] != true {
		t.Fatalf("explicit_user_redirection mismatch: %v", redirection)
	}
	oldArcTrace, _ := gov["old_arc_keep_drop_trace"].([]any)
	foundStaleDrop := false
	for _, raw := range oldArcTrace {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if item["status"] == "resolved" && item["decision"] == "drop" && item["reason"] == "stale_or_resolved_arc_demoted" {
			foundStaleDrop = true
		}
	}
	if !foundStaleDrop {
		t.Fatalf("old_arc_keep_drop_trace missing resolved drop: %v", oldArcTrace)
	}
}

// SEQ-16.5-P123: helper budget need/risk breakdown trace schema — validates that
// injection_pack.budget_decisions exposes reason_counts and section_blocks
// show per-lane budget breakdown.
func TestSeq165P123HelperBudgetNeedRiskBreakdownTraceSchema(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq165-p123", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 10, ChatSessionID: "seq165-p123", TurnIndex: 4, Role: "user", Content: "open door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p123","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	// reason_counts must exist for need/risk breakdown trace.
	if _, ok := bd["reason_counts"]; !ok {
		t.Fatalf("budget_decisions missing reason_counts")
	}
	// section_blocks must expose per-lane budget.
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
		if _, ok := block["chars"]; !ok {
			t.Fatalf("section_block missing chars: %v", block)
		}
	}
	helperTrace := seq165Map(t, resp, "helper_budget_governor_trace")
	if helperTrace["version"] != "seq16_5_helper_budget_trace.v1" {
		t.Fatalf("helper_budget_governor_trace version=%v", helperTrace["version"])
	}
	if helperTrace["truth_authority"] != false {
		t.Fatalf("helper_budget_governor_trace truth_authority=%v, want false", helperTrace["truth_authority"])
	}
	if _, ok := helperTrace["need_breakdown"].(map[string]any); !ok {
		t.Fatalf("helper_budget_governor_trace missing need_breakdown")
	}
	if _, ok := helperTrace["risk_breakdown"].(map[string]any); !ok {
		t.Fatalf("helper_budget_governor_trace missing risk_breakdown")
	}
}

// SEQ-16.5-P124: input anchor selected/dropped reason trace — validates that
// continuity_pack.items expose kind labels (selected) and that
// input_context_text construction implies slot selection.
func TestSeq165P124InputAnchorSelectedDroppedReasonTrace(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq165-p124", TurnIndex: 3, Role: "user", Content: "hello"},
		},
		returnResumePack: &store.ResumePack{
			Trigger: "resume", AssembledText: "Resume summary.",
		},
		returnActiveStates: []store.ActiveState{
			{ID: 10, ChatSessionID: "seq165-p124", StateType: "scene", TurnIndex: 2, Content: "forest"},
		},
		returnCanonicalLayers: []store.CanonicalStateLayer{
			{ID: 20, ChatSessionID: "seq165-p124", LayerType: "world_state", TurnIndex: 1, Content: "rain"},
		},
		returnEpisodeSums: []store.EpisodeSummary{
			{ID: 30, ChatSessionID: "seq165-p124", FromTurn: 1, ToTurn: 2, SummaryText: "episode one"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p124","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	cp, ok := resp["continuity_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing continuity_pack")
	}
	items, _ := cp["items"].([]any)
	if len(items) == 0 {
		t.Fatalf("continuity_pack.items must not be empty")
	}
	// Each item must have a kind label (selected/dropped traceability).
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if _, ok := item["kind"]; !ok {
			t.Fatalf("continuity_pack item missing kind: %v", item)
		}
	}
	// input_context_text must contain selected slots.
	ict, ok := resp["input_context_text"].(string)
	if !ok || ict == "" {
		t.Fatalf("input_context_text missing or empty")
	}
	if !strings.Contains(ict, "[Recent Chat]") {
		t.Fatalf("input_context_text missing [Recent Chat] (selected slot)")
	}
	gov := seq165Map(t, resp, "input_anchor_governor")
	selected, _ := gov["selected_anchor_trace"].([]any)
	dropped, _ := gov["dropped_anchor_trace"].([]any)
	if len(selected) == 0 {
		t.Fatalf("selected_anchor_trace must not be empty")
	}
	if len(dropped) == 0 {
		t.Fatalf("dropped_anchor_trace must not be empty")
	}
	for _, raw := range append(selected, dropped...) {
		trace, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if trace["slot"] == "" || trace["reason"] == "" {
			t.Fatalf("anchor trace missing slot/reason: %v", trace)
		}
	}
}

// SEQ-16.5-P125: preview / dashboard / transparency surface — validates that
// trace_preview exists and exposes key preview fields for dashboard consumption.
func TestSeq165P125PreviewDashboardTransparencySurface(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq165-p125", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 10, ChatSessionID: "seq165-p125", TurnIndex: 4, Role: "user", Content: "open door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p125","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	// trace_preview must exist.
	tp, ok := resp["trace_preview"].(map[string]any)
	if !ok {
		t.Fatalf("missing trace_preview")
	}
	// Key preview fields for dashboard.
	for _, k := range []string{"source", "evidence_counts", "section_summary", "supervisor_status", "critic_status"} {
		if _, ok := tp[k]; !ok {
			t.Fatalf("trace_preview missing %q", k)
		}
	}
	// generation_packet.trace_summary must also exist for transparency.
	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("missing generation_packet")
	}
	traceSummary, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("missing trace_summary")
	}
	for _, k := range []string{"memory_count", "evidence_count", "chat_log_count", "max_injection_chars", "max_input_context_chars"} {
		if _, ok := traceSummary[k]; !ok {
			t.Fatalf("trace_summary missing %q", k)
		}
	}
	if _, ok := resp["input_anchor_governor"].(map[string]any); !ok {
		t.Fatalf("missing input_anchor_governor transparency surface")
	}
	if _, ok := resp["helper_budget_governor_trace"].(map[string]any); !ok {
		t.Fatalf("missing helper_budget_governor_trace transparency surface")
	}
}

// SEQ-16.5-P126: support lane truth lane wording/display guard — validates that
// retrieval_extend_authority and progression_ledger.supporting_precedence_guard
// use support-only wording and do not claim canonical truth.
func TestSeq165P126SupportLaneTruthWordingDisplayGuard(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq165-p126", Name: "arc", LastTurn: 4, Status: "active"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 10, ChatSessionID: "seq165-p126", TurnIndex: 4, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p126","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	// retrieval_extend_authority must use support-only wording.
	rea, ok := resp["retrieval_extend_authority"].(map[string]any)
	if !ok {
		t.Fatalf("missing retrieval_extend_authority")
	}
	if rea["version"] != "p168a.v1" {
		t.Fatalf("retrieval_extend_authority version=%v, want p168a.v1", rea["version"])
	}
	// authority_order must be present with support in the list.
	order, _ := rea["authority_order"].([]any)
	foundSupport := false
	for _, o := range order {
		if o == "support" {
			foundSupport = true
		}
	}
	if !foundSupport {
		t.Fatalf("authority_order missing 'support': %v", order)
	}
	// supporting_precedence_guard must enforce support-only display.
	pl, ok := resp["progression_ledger"].(map[string]any)
	if !ok {
		t.Fatalf("missing progression_ledger")
	}
	guard, ok := pl["supporting_precedence_guard"].(map[string]any)
	if !ok {
		t.Fatalf("missing supporting_precedence_guard")
	}
	if guard["supporting_only"] != true {
		t.Fatalf("supporting_precedence_guard supporting_only=%v, want true", guard["supporting_only"])
	}
	// Must not allow truth_overwrite or canonical_override.
	disallowed, _ := guard["disallowed_usage"].([]any)
	for _, d := range disallowed {
		if d == "truth_overwrite" || d == "canonical_override" {
			// expected
		} else {
			continue
		}
	}
	foundTruthOverwrite := false
	foundCanonicalOverride := false
	for _, d := range disallowed {
		if d == "truth_overwrite" {
			foundTruthOverwrite = true
		}
		if d == "canonical_override" {
			foundCanonicalOverride = true
		}
	}
	if !foundTruthOverwrite {
		t.Fatalf("supporting_precedence_guard disallowed_usage missing truth_overwrite")
	}
	if !foundCanonicalOverride {
		t.Fatalf("supporting_precedence_guard disallowed_usage missing canonical_override")
	}
	gov := seq165Map(t, resp, "input_anchor_governor")
	wording := seq165Map(t, gov, "support_lane_wording_guard")
	if wording["truth_lane_label_forbidden"] != true {
		t.Fatalf("support_lane_wording_guard truth_lane_label_forbidden=%v, want true", wording["truth_lane_label_forbidden"])
	}
	if wording["canonical_truth_wording_allowed"] != false {
		t.Fatalf("support_lane_wording_guard canonical_truth_wording_allowed=%v, want false", wording["canonical_truth_wording_allowed"])
	}
}

// SEQ-16.5-P127: old-arc keep/drop reason trace — validates that
// trace_preview.storyline_selection exposes keep/drop reasons for old arcs.
func TestSeq165P127OldArcKeepDropReasonTrace(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq165-p127", Name: "active arc", LastTurn: 10, Status: "active"},
			{ID: 2, ChatSessionID: "seq165-p127", Name: "stale arc", LastTurn: 2, Status: "dormant"},
			{ID: 3, ChatSessionID: "seq165-p127", Name: "resolved arc", LastTurn: 3, Status: "resolved"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 10, ChatSessionID: "seq165-p127", TurnIndex: 11, Role: "user", Content: "continue"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p127","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	tp, ok := resp["trace_preview"].(map[string]any)
	if !ok {
		t.Fatalf("missing trace_preview")
	}
	ss, ok := tp["storyline_selection"].(map[string]any)
	if !ok {
		t.Fatalf("missing trace_preview.storyline_selection")
	}
	// Must expose selected and dropped with counts.
	if _, ok := ss["selected_count"]; !ok {
		t.Fatalf("storyline_selection missing selected_count")
	}
	if _, ok := ss["dropped_count"]; !ok {
		t.Fatalf("storyline_selection missing dropped_count")
	}
	// generation_packet trace_summary must expose stale_dropped_count.
	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("missing generation_packet")
	}
	traceSummary, ok := gp["trace_summary"].(map[string]any)
	if !ok {
		t.Fatalf("missing trace_summary")
	}
	if _, ok := traceSummary["storyline_stale_dropped_count"]; !ok {
		t.Fatalf("trace_summary missing storyline_stale_dropped_count")
	}
	gov := seq165Map(t, resp, "input_anchor_governor")
	oldArcTrace, _ := gov["old_arc_keep_drop_trace"].([]any)
	kept := false
	dropped := false
	for _, raw := range oldArcTrace {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if item["decision"] == "keep" && item["reason"] == "active_arc_anchor" {
			kept = true
		}
		if item["decision"] == "drop" && item["reason"] == "stale_or_resolved_arc_demoted" {
			dropped = true
		}
	}
	if !kept || !dropped {
		t.Fatalf("old_arc_keep_drop_trace kept=%v dropped=%v trace=%v", kept, dropped, oldArcTrace)
	}
}

// SEQ-16.5-P131: weak-input continuity replay — validates that continuity_pack
// exposes resume_pack, episode_summary, and chat_log items for weak-input continuity.
func TestSeq165P131WeakInputContinuityReplay(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnResumePack: &store.ResumePack{
			Trigger: "weak_input", AssembledText: "Weak input resume.",
		},
		returnEpisodeSums: []store.EpisodeSummary{
			{ID: 1, ChatSessionID: "seq165-p131", FromTurn: 1, ToTurn: 3, SummaryText: "early episode"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 10, ChatSessionID: "seq165-p131", TurnIndex: 4, Role: "user", Content: "hi"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p131","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	cp, ok := resp["continuity_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing continuity_pack")
	}
	items, _ := cp["items"].([]any)
	foundResume := false
	foundEpisode := false
	foundChat := false
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		kind, _ := item["kind"].(string)
		switch kind {
		case "resume_pack":
			foundResume = true
		case "episode_summary":
			foundEpisode = true
		case "chat_log":
			foundChat = true
		}
	}
	if !foundResume {
		t.Fatalf("continuity_pack.items missing resume_pack")
	}
	if !foundEpisode {
		t.Fatalf("continuity_pack.items missing episode_summary")
	}
	if !foundChat {
		t.Fatalf("continuity_pack.items missing chat_log")
	}
	gov := seq165Map(t, resp, "input_anchor_governor")
	mandatory, _ := gov["mandatory_slots"].([]any)
	if seq165FindSlot(t, mandatory, "Temporal Anchor")["selected"] != true {
		t.Fatalf("Temporal Anchor should be selected for weak-input continuity")
	}
	if seq165FindSlot(t, mandatory, "Previous")["selected"] != true {
		t.Fatalf("Previous should be selected for weak-input continuity")
	}
}

// SEQ-16.5-P132: temporal query replay — validates that temporal_read_validity_first,
// validity_window_reading, and temporal_disambiguation_contract surfaces are present.
func TestSeq165P132TemporalQueryReplay(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq165-p132", TurnIndex: 1, Role: "user", Content: "start"},
			{ID: 2, ChatSessionID: "seq165-p132", TurnIndex: 2, Role: "user", Content: "middle"},
			{ID: 3, ChatSessionID: "seq165-p132", TurnIndex: 3, Role: "user", Content: "end"},
		},
		returnEpisodeSums: []store.EpisodeSummary{
			{ID: 10, ChatSessionID: "seq165-p132", FromTurn: 1, ToTurn: 3, SummaryText: "full episode"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p132","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	for _, key := range []string{"temporal_read_validity_first", "validity_window_reading", "temporal_disambiguation_contract"} {
		if _, ok := resp[key]; !ok {
			t.Fatalf("missing %s", key)
		}
	}
	// temporal_read_validity_first must have temporal validity fields.
	trvf, ok := resp["temporal_read_validity_first"].(map[string]any)
	if !ok {
		t.Fatalf("temporal_read_validity_first not a map")
	}
	if _, ok := trvf["latest_chat_turn"]; !ok {
		t.Fatalf("temporal_read_validity_first missing latest_chat_turn")
	}
	gov := seq165Map(t, resp, "input_anchor_governor")
	rules := seq165Map(t, gov, "promotion_demotion_rules")
	if !strings.Contains(rules["temporal_query"].(string), "temporal_anchor") {
		t.Fatalf("temporal_query rule does not promote temporal anchor: %v", rules["temporal_query"])
	}
}

// SEQ-16.5-P133: long-gap resume replay — validates that resumePack is present
// in both continuity_pack and input_context_text for long-gap resume scenarios.
func TestSeq165P133LongGapResumeReplay(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnResumePack: &store.ResumePack{
			Trigger: "long_gap", AssembledText: "Long gap resume summary.",
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq165-p133", TurnIndex: 1, Role: "user", Content: "old"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p133","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	// input_context_text must contain resume pack.
	ict, ok := resp["input_context_text"].(string)
	if !ok || ict == "" {
		t.Fatalf("input_context_text missing or empty")
	}
	if !strings.Contains(ict, "[Resume Pack]") {
		t.Fatalf("input_context_text missing [Resume Pack]")
	}
	// continuity_pack must contain resume_pack item.
	cp, ok := resp["continuity_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing continuity_pack")
	}
	items, _ := cp["items"].([]any)
	foundResume := false
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if item["kind"] == "resume_pack" {
			foundResume = true
		}
	}
	if !foundResume {
		t.Fatalf("continuity_pack.items missing resume_pack")
	}
	gov := seq165Map(t, resp, "input_anchor_governor")
	previous := seq165FindSlot(t, gov["mandatory_slots"].([]any), "Previous")
	if previous["selected"] != true || previous["reason"] != "resume_pack_available" {
		t.Fatalf("Previous slot mismatch for long-gap resume: %v", previous)
	}
}

// SEQ-16.5-P134: multi-entity / multi-thread replay — validates that
// session_state and continuity_pack expose multiple entities and threads.
func TestSeq165P134MultiEntityMultiThreadReplay(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnCharStates: []store.CharacterState{
			{ID: 1, ChatSessionID: "seq165-p134", CharacterName: "Alice", TurnIndex: 1},
			{ID: 2, ChatSessionID: "seq165-p134", CharacterName: "Bob", TurnIndex: 2},
		},
		returnPendingThreads: []store.PendingThread{
			{ID: 10, ChatSessionID: "seq165-p134", ThreadKey: "thread-a", CreatedTurn: 1, Status: "open"},
			{ID: 11, ChatSessionID: "seq165-p134", ThreadKey: "thread-b", CreatedTurn: 2, Status: "open"},
		},
		returnActiveStates: []store.ActiveState{
			{ID: 20, ChatSessionID: "seq165-p134", StateType: "scene", TurnIndex: 1, Content: "forest"},
			{ID: 21, ChatSessionID: "seq165-p134", StateType: "entity", TurnIndex: 2, Content: "wolf"},
		},
		returnCanonicalLayers: []store.CanonicalStateLayer{
			{ID: 30, ChatSessionID: "seq165-p134", LayerType: "world_state", TurnIndex: 1, Content: "night"},
			{ID: 31, ChatSessionID: "seq165-p134", LayerType: "entity_state", TurnIndex: 2, Content: "angry"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 40, ChatSessionID: "seq165-p134", TurnIndex: 3, Role: "user", Content: "run"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p134","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	// session_state must expose multiple characters and threads.
	ss, ok := resp["session_state"].(map[string]any)
	if !ok {
		t.Fatalf("missing session_state")
	}
	sectionMeta, ok := ss["section_meta"].(map[string]any)
	if !ok {
		t.Fatalf("missing session_state.section_meta")
	}
	if sectionMeta["character_count"].(float64) != 2 {
		t.Fatalf("character_count=%v, want 2", sectionMeta["character_count"])
	}
	if sectionMeta["pending_thread_count"].(float64) != 2 {
		t.Fatalf("pending_thread_count=%v, want 2", sectionMeta["pending_thread_count"])
	}
	// continuity_pack must expose multiple active_states and canonical_layers.
	cp, ok := resp["continuity_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing continuity_pack")
	}
	items, _ := cp["items"].([]any)
	activeCount := 0
	canonCount := 0
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		kind, _ := item["kind"].(string)
		if kind == "active_state" {
			activeCount++
		}
		if kind == "canonical_layer" {
			canonCount++
		}
	}
	if activeCount < 2 {
		t.Fatalf("continuity_pack active_state items=%d, want >= 2", activeCount)
	}
	if canonCount < 2 {
		t.Fatalf("continuity_pack canonical_layer items=%d, want >= 2", canonCount)
	}
	gov := seq165Map(t, resp, "input_anchor_governor")
	optional := gov["optional_slots"].([]any)
	for _, name := range []string{"Scene", "Entity", "Active Thread"} {
		if seq165FindSlot(t, optional, name)["selected"] != true {
			t.Fatalf("%s should be selected for multi-entity/multi-thread replay", name)
		}
	}
}

// SEQ-16.5-P135: explicit user-intent suppression replay — validates that
// runtime_toggle.broad_takeover is false and injection_pack.apply_verdict
// is shadow_only, ensuring explicit user intent is not overridden.
func TestSeq165P135ExplicitUserIntentSuppressionReplay(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq165-p135", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 10, ChatSessionID: "seq165-p135", TurnIndex: 4, Role: "user", Content: "I want to go left"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p135","turn_index":1,"raw_user_input":"I want to go left","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	if rt["broad_takeover"] != false {
		t.Fatalf("broad_takeover=%v, want false (explicit user intent must not be overridden)", rt["broad_takeover"])
	}
	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing injection_pack")
	}
	if ip["apply_verdict"] != "shadow_only" {
		t.Fatalf("apply_verdict=%v, want shadow_only", ip["apply_verdict"])
	}
	// input_context_text must preserve the explicit user input.
	ict, ok := resp["input_context_text"].(string)
	if !ok || ict == "" {
		t.Fatalf("input_context_text missing or empty")
	}
	if !strings.Contains(ict, "[Recent Chat]") {
		t.Fatalf("input_context_text missing [Recent Chat] (user input must be preserved)")
	}
	gov := seq165Map(t, resp, "input_anchor_governor")
	redirection := seq165Map(t, gov, "explicit_user_redirection")
	if redirection["current_user_input_wins"] != true || redirection["support_lane_may_redirect"] != false {
		t.Fatalf("explicit_user_redirection guard mismatch: %v", redirection)
	}
}

// SEQ-16.5-P136: stale/conflict/subsystem-failure conservative replay — validates that
// degraded or partial-read mode produces conservative surfaces with fallback_reason.
func TestSeq165P136StaleConflictSubsystemFailureConservativeReplay(t *testing.T) {
	srv := setupTestServer()
	// Store is nil → degraded mode with no reads.
	srv.Store = nil
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p136","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	// fallback_reason must indicate store unavailability.
	if resp["fallback_reason"] != "store_unavailable" {
		t.Fatalf("fallback_reason=%v, want store_unavailable", resp["fallback_reason"])
	}
	// generation_packet must be off mode.
	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("missing generation_packet")
	}
	if gp["packet_mode"] != "off" {
		t.Fatalf("packet_mode=%v, want off", gp["packet_mode"])
	}
	// injection_pack status must be off or skeleton.
	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing injection_pack")
	}
	status, _ := ip["status"].(string)
	if status != "off" && status != "skeleton" {
		t.Fatalf("injection_pack status=%v, want off or skeleton", status)
	}
	// progression_ledger status must be degraded.
	pl, ok := resp["progression_ledger"].(map[string]any)
	if !ok {
		t.Fatalf("missing progression_ledger")
	}
	if pl["status"] != "degraded" {
		t.Fatalf("progression_ledger status=%v, want degraded", pl["status"])
	}
	gov := seq165Map(t, resp, "input_anchor_governor")
	if gov["truth_authority"] != false || gov["role"] != "support_anchor_lane_only" {
		t.Fatalf("degraded input_anchor_governor authority mismatch: %v", gov)
	}
}

// SEQ-16.5-P137: static budget vs adaptive governor compare replay — validates that
// generation_packet.trace_summary exposes static max_injection_chars and
// runtime_token_profile.auto_optimized for adaptive governor comparison.
func TestSeq165P137StaticBudgetVsAdaptiveGovernorCompareReplay(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq165-p137", TurnIndex: 2, SummaryJSON: `{"text":"found a key"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 10, ChatSessionID: "seq165-p137", TurnIndex: 4, Role: "user", Content: "open door"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p137","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true,"max_injection_chars":2500}}`
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
	// Static budget must be exposed.
	maxChars, ok := traceSummary["max_injection_chars"].(float64)
	if !ok || maxChars <= 0 {
		t.Fatalf("max_injection_chars=%v, want > 0", traceSummary["max_injection_chars"])
	}
	if maxChars != 2500 {
		t.Fatalf("max_injection_chars=%v, want 2500", maxChars)
	}
	// Adaptive governor marker: runtime_token_profile.auto_optimized.
	rtp, ok := traceSummary["runtime_token_profile"].(map[string]any)
	if !ok {
		t.Fatalf("missing runtime_token_profile")
	}
	if _, ok := rtp["auto_optimized"]; !ok {
		t.Fatalf("runtime_token_profile missing auto_optimized")
	}
	if rtp["status"] != "shadow_only" {
		t.Fatalf("runtime_token_profile status=%v, want shadow_only", rtp["status"])
	}
	helperTrace := seq165Map(t, resp, "helper_budget_governor_trace")
	if helperTrace["max_injection_chars"] != float64(2500) {
		t.Fatalf("helper_budget_governor_trace max_injection_chars=%v, want 2500", helperTrace["max_injection_chars"])
	}
	if helperTrace["budget_decision_mode"] != "turn_local_shadow_trace" {
		t.Fatalf("helper_budget_governor_trace budget_decision_mode=%v", helperTrace["budget_decision_mode"])
	}
}

// SEQ-16.5-P141: helper injection budget manager define — validates that
// helper_injection_budget_manager surface exists with correct version and role.
func TestSeq165P141HelperInjectionBudgetManager(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq165-p141", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p141","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	mgr := seq165Map(t, resp, "helper_injection_budget_manager")
	if mgr["version"] != "seq16_5_p141.v1" {
		t.Fatalf("version=%v, want seq16_5_p141.v1", mgr["version"])
	}
	if mgr["role"] != "helper_injection_budget_manager" {
		t.Fatalf("role=%v, want helper_injection_budget_manager", mgr["role"])
	}
	if mgr["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", mgr["truth_authority"])
	}
	if mgr["support_lane_only"] != true {
		t.Fatalf("support_lane_only=%v, want true", mgr["support_lane_only"])
	}
	if mgr["policy_version"] != "s16.5-hg.v1" {
		t.Fatalf("policy_version=%v, want s16.5-hg.v1", mgr["policy_version"])
	}
	if mgr["mode"] != "turn_need_risk_char_budget_governor" {
		t.Fatalf("mode=%v, want turn_need_risk_char_budget_governor", mgr["mode"])
	}
}

// SEQ-16.5-P142: input context builder slot governor — validates that
// input_context_slot_governor surface exists with correct version and mode.
func TestSeq165P142InputContextSlotGovernor(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq165-p142", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p142","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	gov := seq165Map(t, resp, "input_context_slot_governor")
	if gov["version"] != "seq16_5_p142.v1" {
		t.Fatalf("version=%v, want seq16_5_p142.v1", gov["version"])
	}
	if gov["role"] != "input_context_slot_governor" {
		t.Fatalf("role=%v, want input_context_slot_governor", gov["role"])
	}
	if gov["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", gov["truth_authority"])
	}
	if gov["slot_governor_policy_version"] != "s16.5-ig.v1" {
		t.Fatalf("slot_governor_policy_version=%v, want s16.5-ig.v1", gov["slot_governor_policy_version"])
	}
	if gov["slot_governor_mode"] != "turn_need_risk_slot_governor" {
		t.Fatalf("slot_governor_mode=%v, want turn_need_risk_slot_governor", gov["slot_governor_mode"])
	}
	if gov["short_and_sharp_anchor_lane_preserve"] != true {
		t.Fatalf("short_and_sharp_anchor_lane_preserve=%v, want true", gov["short_and_sharp_anchor_lane_preserve"])
	}
}

// SEQ-16.5-P143: transparency / preview / runtime trace extend — validates that
// transparency_preview_runtime_trace_extend surface exists with correct version.
func TestSeq165P143TransparencyPreviewRuntimeTraceExtend(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq165-p143", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p143","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	ext := seq165Map(t, resp, "transparency_preview_runtime_trace_extend")
	if ext["version"] != "seq16_5_p143.v1" {
		t.Fatalf("version=%v, want seq16_5_p143.v1", ext["version"])
	}
	if ext["role"] != "transparency_preview_runtime_trace_extend" {
		t.Fatalf("role=%v, want transparency_preview_runtime_trace_extend", ext["role"])
	}
	if ext["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", ext["truth_authority"])
	}
	if ext["support_lane_only"] != true {
		t.Fatalf("support_lane_only=%v, want true", ext["support_lane_only"])
	}
	if ext["policy_version"] != "s16.5-ts.v1" {
		t.Fatalf("policy_version=%v, want s16.5-ts.v1", ext["policy_version"])
	}
	if ext["mode"] != "trace_inspection_surface" {
		t.Fatalf("mode=%v, want trace_inspection_surface", ext["mode"])
	}
}

// SEQ-16.5-P144: backend/main.py input_context_text handoff anchor metadata
// alignment — validates that handoff_anchor_metadata_alignment surface exists.
func TestSeq165P144HandoffAnchorMetadataAlignment(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq165-p144", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p144","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	hal := seq165Map(t, resp, "handoff_anchor_metadata_alignment")
	if hal["version"] != "seq16_5_p144.v1" {
		t.Fatalf("version=%v, want seq16_5_p144.v1", hal["version"])
	}
	if hal["role"] != "handoff_anchor_metadata_alignment" {
		t.Fatalf("role=%v, want handoff_anchor_metadata_alignment", hal["role"])
	}
	if hal["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", hal["truth_authority"])
	}
	if hal["alignment_status"] != "aligned" {
		t.Fatalf("alignment_status=%v, want aligned", hal["alignment_status"])
	}
	if hal["policy_version"] != "s16.5-ha.v1" {
		t.Fatalf("policy_version=%v, want s16.5-ha.v1", hal["policy_version"])
	}
	if hal["mode"] != "backend_js_handoff_shadow" {
		t.Fatalf("mode=%v, want backend_js_handoff_shadow", hal["mode"])
	}
}

// SEQ-16.5-P145: Step 16.8 stale-arc guard carry-in / Step 17 evaluation ops
// carry-in replay/inspection hooks — validates stale_arc_guard_carry_in_hooks.
func TestSeq165P145StaleArcGuardCarryInHooks(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq165-p145", TurnIndex: 1, Role: "user", Content: "hello"},
		},
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq165-p145", Name: "Old Arc", Status: "resolved", LastTurn: 5},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p145","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	hooks := seq165Map(t, resp, "stale_arc_guard_carry_in_hooks")
	if hooks["version"] != "seq16_5_p145.v1" {
		t.Fatalf("version=%v, want seq16_5_p145.v1", hooks["version"])
	}
	if hooks["role"] != "stale_arc_guard_carry_in_hooks" {
		t.Fatalf("role=%v, want stale_arc_guard_carry_in_hooks", hooks["role"])
	}
	if hooks["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", hooks["truth_authority"])
	}
	if hooks["step_16_8_guard_ready"] != true {
		t.Fatalf("step_16_8_guard_ready=%v, want true", hooks["step_16_8_guard_ready"])
	}
	if hooks["step_17_evaluation_gate_closed"] != true {
		t.Fatalf("step_17_evaluation_gate_closed=%v, want true", hooks["step_17_evaluation_gate_closed"])
	}
	if hooks["policy_version"] != "s16.5-vx.v1" {
		t.Fatalf("policy_version=%v, want s16.5-vx.v1", hooks["policy_version"])
	}
}

// SEQ-16.5-P169: helper injection adaptive floor / ceiling decision value.
func TestSeq165P169DecisionAdaptiveFloorCeiling(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq165-p169", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p169","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	dec := seq165Map(t, resp, "decision_adaptive_floor_ceiling")
	if dec["version"] != "seq16_5_p169.v1" {
		t.Fatalf("version=%v, want seq16_5_p169.v1", dec["version"])
	}
	if dec["decision"] != "adaptive_floor_ceiling" {
		t.Fatalf("decision=%v, want adaptive_floor_ceiling", dec["decision"])
	}
	if dec["floor_chars"] != float64(500) {
		t.Fatalf("floor_chars=%v, want 500", dec["floor_chars"])
	}
	if dec["ceiling_chars"] != float64(7000) {
		t.Fatalf("ceiling_chars=%v, want 7000", dec["ceiling_chars"])
	}
	if dec["base_chars"] != float64(3000) {
		t.Fatalf("base_chars=%v, want 3000", dec["base_chars"])
	}
}

// SEQ-16.5-P170: input context max slot 2 vs 3 decision value.
func TestSeq165P170DecisionMaxSlot(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq165-p170", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p170","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	dec := seq165Map(t, resp, "decision_max_slot")
	if dec["version"] != "seq16_5_p170.v1" {
		t.Fatalf("version=%v, want seq16_5_p170.v1", dec["version"])
	}
	if dec["decision"] != "max_slot" {
		t.Fatalf("decision=%v, want max_slot", dec["decision"])
	}
	if dec["max_slots"] != float64(7) {
		t.Fatalf("max_slots=%v, want 7", dec["max_slots"])
	}
	if dec["mandatory_slots"] != float64(2) {
		t.Fatalf("mandatory_slots=%v, want 2", dec["mandatory_slots"])
	}
	if dec["optional_slots"] != float64(5) {
		t.Fatalf("optional_slots=%v, want 5", dec["optional_slots"])
	}
}

// SEQ-16.5-P171: runtime token hint telemetry-only / secondary safety cap
// decision value.
func TestSeq165P171DecisionRuntimeTokenHint(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq165-p171", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p171","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	dec := seq165Map(t, resp, "decision_runtime_token_hint")
	if dec["version"] != "seq16_5_p171.v1" {
		t.Fatalf("version=%v, want seq16_5_p171.v1", dec["version"])
	}
	if dec["decision"] != "runtime_token_hint_policy" {
		t.Fatalf("decision=%v, want runtime_token_hint_policy", dec["decision"])
	}
	if dec["telemetry_only"] != true {
		t.Fatalf("telemetry_only=%v, want true", dec["telemetry_only"])
	}
	if dec["secondary_safety_cap"] != true {
		t.Fatalf("secondary_safety_cap=%v, want true", dec["secondary_safety_cap"])
	}
	if dec["primary_authority"] != "turn_need_risk_inventory" {
		t.Fatalf("primary_authority=%v, want turn_need_risk_inventory", dec["primary_authority"])
	}
}

// SEQ-16.5-P172: [Saga] / [Chapter] anchor competition vs fallback ladder
// decision value.
func TestSeq165P172DecisionSagaChapterAnchorLadder(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq165-p172", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p172","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	dec := seq165Map(t, resp, "decision_saga_chapter_anchor_ladder")
	if dec["version"] != "seq16_5_p172.v1" {
		t.Fatalf("version=%v, want seq16_5_p172.v1", dec["version"])
	}
	if dec["decision"] != "saga_chapter_anchor_ladder" {
		t.Fatalf("decision=%v, want saga_chapter_anchor_ladder", dec["decision"])
	}
	if dec["competition_mode"] != false {
		t.Fatalf("competition_mode=%v, want false", dec["competition_mode"])
	}
	if dec["fallback_ladder"] != true {
		t.Fatalf("fallback_ladder=%v, want true", dec["fallback_ladder"])
	}
}

// SEQ-16.5-P173: explicit user-input specificity heuristic/classifier decision
// value.
func TestSeq165P173DecisionExplicitUserInputSpecificity(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq165-p173", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p173","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	dec := seq165Map(t, resp, "decision_explicit_user_input_specificity")
	if dec["version"] != "seq16_5_p173.v1" {
		t.Fatalf("version=%v, want seq16_5_p173.v1", dec["version"])
	}
	if dec["decision"] != "explicit_user_input_specificity" {
		t.Fatalf("decision=%v, want explicit_user_input_specificity", dec["decision"])
	}
	if dec["heuristic"] != "length_and_keyword_classifier" {
		t.Fatalf("heuristic=%v, want length_and_keyword_classifier", dec["heuristic"])
	}
	if dec["strong_threshold_chars"] != float64(48) {
		t.Fatalf("strong_threshold_chars=%v, want 48", dec["strong_threshold_chars"])
	}
	if dec["strong_threshold_words"] != float64(10) {
		t.Fatalf("strong_threshold_words=%v, want 10", dec["strong_threshold_words"])
	}
}

// SEQ-16.5-P177: Step 16.8 stale-arc suppression slice baseline compare.
func TestSeq165P177Step168BaselineCompare(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq165-p177", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p177","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	bc := seq165Map(t, resp, "step_16_8_baseline_compare")
	if bc["version"] != "seq16_5_p177.v1" {
		t.Fatalf("version=%v, want seq16_5_p177.v1", bc["version"])
	}
	if bc["role"] != "step_16_8_baseline_compare" {
		t.Fatalf("role=%v, want step_16_8_baseline_compare", bc["role"])
	}
	if bc["compare_ready"] != true {
		t.Fatalf("compare_ready=%v, want true", bc["compare_ready"])
	}
	if bc["baseline_source"] != "seq16_5_helper_input_governor_trace" {
		t.Fatalf("baseline_source=%v, want seq16_5_helper_input_governor_trace", bc["baseline_source"])
	}
}

// SEQ-16.5-P178: Step 16.8 reason visibility / monopoly replay guard lane.
func TestSeq165P178Step168ReasonVisibilityGuardLane(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq165-p178", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p178","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	gl := seq165Map(t, resp, "step_16_8_reason_visibility_guard_lane")
	if gl["version"] != "seq16_5_p178.v1" {
		t.Fatalf("version=%v, want seq16_5_p178.v1", gl["version"])
	}
	if gl["role"] != "step_16_8_reason_visibility_guard_lane" {
		t.Fatalf("role=%v, want step_16_8_reason_visibility_guard_lane", gl["role"])
	}
	if gl["guard_lane_ready"] != true {
		t.Fatalf("guard_lane_ready=%v, want true", gl["guard_lane_ready"])
	}
	if gl["adaptive_governor_ready"] != true {
		t.Fatalf("adaptive_governor_ready=%v, want true", gl["adaptive_governor_ready"])
	}
}

// SEQ-16.5-P179: Step 16.8 completion Step 17 evaluation baseline direct
// handoff gate.
func TestSeq165P179Step17DirectHandoffGate(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq165-p179", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p179","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	gate := seq165Map(t, resp, "step_17_direct_handoff_gate")
	if gate["version"] != "seq16_5_p179.v1" {
		t.Fatalf("version=%v, want seq16_5_p179.v1", gate["version"])
	}
	if gate["role"] != "step_17_direct_handoff_gate" {
		t.Fatalf("role=%v, want step_17_direct_handoff_gate", gate["role"])
	}
	if gate["gate_open"] != false {
		t.Fatalf("gate_open=%v, want false", gate["gate_open"])
	}
	if gate["gate_reason"] != "step_16_8_guard_baseline_not_closed" {
		t.Fatalf("gate_reason=%v, want step_16_8_guard_baseline_not_closed", gate["gate_reason"])
	}
}

// SEQ-16.5-P183: Step 17 evaluation harness static 3000/800 baseline + 16.5+16.8
// baseline.
func TestSeq165P183Step17EvaluationHarnessBaseline(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq165-p183", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p183","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	ehb := seq165Map(t, resp, "step_17_evaluation_harness_baseline")
	if ehb["version"] != "seq16_5_p183.v1" {
		t.Fatalf("version=%v, want seq16_5_p183.v1", ehb["version"])
	}
	if ehb["role"] != "step_17_evaluation_harness_baseline" {
		t.Fatalf("role=%v, want step_17_evaluation_harness_baseline", ehb["role"])
	}
	static := seq165Map(t, ehb, "static_baseline")
	if static["max_injection_chars"] != float64(3000) {
		t.Fatalf("static max_injection_chars=%v, want 3000", static["max_injection_chars"])
	}
	if static["max_input_context_chars"] != float64(800) {
		t.Fatalf("static max_input_context_chars=%v, want 800", static["max_input_context_chars"])
	}
}

// SEQ-16.5-P184: Step 17 ops budget tuning governor behavior trace
// interpretation document.
func TestSeq165P184Step17OpsTraceInterpretation(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq165-p184", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p184","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	ops := seq165Map(t, resp, "step_17_ops_trace_interpretation")
	if ops["version"] != "seq16_5_p184.v1" {
		t.Fatalf("version=%v, want seq16_5_p184.v1", ops["version"])
	}
	if ops["role"] != "step_17_ops_trace_interpretation" {
		t.Fatalf("role=%v, want step_17_ops_trace_interpretation", ops["role"])
	}
	if ops["document_target"] != "governor_behavior_and_trace_interpretation" {
		t.Fatalf("document_target=%v, want governor_behavior_and_trace_interpretation", ops["document_target"])
	}
	if ops["not_document_target"] != "budget_tuning_numbers" {
		t.Fatalf("not_document_target=%v, want budget_tuning_numbers", ops["not_document_target"])
	}
}

// SEQ-16.5-P185: Step 17 inspection surface dynamic budget decision stale-arc
// guard reason lane.
func TestSeq165P185Step17InspectionSurface(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq165-p185", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"seq165-p185","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	ins := seq165Map(t, resp, "step_17_inspection_surface")
	if ins["version"] != "seq16_5_p185.v1" {
		t.Fatalf("version=%v, want seq16_5_p185.v1", ins["version"])
	}
	if ins["role"] != "step_17_inspection_surface" {
		t.Fatalf("role=%v, want step_17_inspection_surface", ins["role"])
	}
	if ins["dynamic_budget_decision_visible"] != true {
		t.Fatalf("dynamic_budget_decision_visible=%v, want true", ins["dynamic_budget_decision_visible"])
	}
	if ins["stale_arc_guard_reason_lane_visible"] != true {
		t.Fatalf("stale_arc_guard_reason_lane_visible=%v, want true", ins["stale_arc_guard_reason_lane_visible"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-16.8 contract tests (P99 ~ P136)
// ---------------------------------------------------------------------------

func TestSeq168P99StaleArcCeiling(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq168-p99", Name: "old arc", LastTurn: 2, Status: "resolved"},
			{ID: 2, ChatSessionID: "seq168-p99", Name: "current arc", LastTurn: 5, Status: "active"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p99", TurnIndex: 5, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p99","turn_index":5,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	ceiling := seq165Map(t, resp, "stale_arc_ceiling")
	if ceiling["version"] != "seq16_8_p99.v1" {
		t.Fatalf("version=%v, want seq16_8_p99.v1", ceiling["version"])
	}
	if ceiling["role"] != "stale_arc_ceiling" {
		t.Fatalf("role=%v, want stale_arc_ceiling", ceiling["role"])
	}
	if ceiling["auto_rescue_enabled"] != false {
		t.Fatalf("auto_rescue_enabled=%v, want false", ceiling["auto_rescue_enabled"])
	}
	if ceiling["stale_arc_count"] != float64(1) {
		t.Fatalf("stale_arc_count=%v, want 1", ceiling["stale_arc_count"])
	}
}

func TestSeq168P100SceneAlignment(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnActiveStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "seq168-p100", StateType: "scene", Content: "forest"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p100", TurnIndex: 1, Role: "user", Content: "where are we"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p100","turn_index":1,"raw_user_input":"where are we","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	sa := seq165Map(t, resp, "scene_alignment")
	if sa["version"] != "seq16_8_p100.v1" {
		t.Fatalf("version=%v, want seq16_8_p100.v1", sa["version"])
	}
	if sa["role"] != "scene_alignment" {
		t.Fatalf("role=%v, want scene_alignment", sa["role"])
	}
	if sa["scene_anchor_selected"] != true {
		t.Fatalf("scene_anchor_selected=%v, want true", sa["scene_anchor_selected"])
	}
	if sa["explicit_scene_query"] != true {
		t.Fatalf("explicit_scene_query=%v, want true", sa["explicit_scene_query"])
	}
}

func TestSeq168P101ReasonTrace(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq168-p101", Name: "arc1", LastTurn: 2, Status: "resolved"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p101", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p101","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	rt := seq165Map(t, resp, "reason_trace")
	if rt["version"] != "seq16_8_p101.v1" {
		t.Fatalf("version=%v, want seq16_8_p101.v1", rt["version"])
	}
	if rt["role"] != "reason_trace" {
		t.Fatalf("role=%v, want reason_trace", rt["role"])
	}
	if rt["inspectable"] != true {
		t.Fatalf("inspectable=%v, want true", rt["inspectable"])
	}
	codes, ok := rt["reason_codes"].([]any)
	if !ok || len(codes) == 0 {
		t.Fatalf("reason_codes empty or missing")
	}
}

func TestSeq168P102FailureSplit(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq168-p102", Name: "arc1", LastTurn: 2, Status: "resolved"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p102", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p102","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	fs := seq165Map(t, resp, "failure_split")
	if fs["version"] != "seq16_8_p102.v1" {
		t.Fatalf("version=%v, want seq16_8_p102.v1", fs["version"])
	}
	if fs["role"] != "failure_split" {
		t.Fatalf("role=%v, want failure_split", fs["role"])
	}
	if _, ok := fs["failure_classes"]; !ok {
		t.Fatalf("missing failure_classes")
	}
}

func TestSeq168P103PacketSynthesis(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq168-p103", Name: "arc1", LastTurn: 2, Status: "active"},
		},
		returnPendingThreads: []store.PendingThread{
			{ID: 1, ChatSessionID: "seq168-p103", ThreadKey: "t1", CreatedTurn: 1, Status: "open"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p103", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p103","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	ps := seq165Map(t, resp, "packet_synthesis")
	if ps["version"] != "seq16_8_p103.v1" {
		t.Fatalf("version=%v, want seq16_8_p103.v1", ps["version"])
	}
	if ps["role"] != "packet_synthesis" {
		t.Fatalf("role=%v, want packet_synthesis", ps["role"])
	}
	if ps["step_21_packet_ready"] != true {
		t.Fatalf("step_21_packet_ready=%v, want true", ps["step_21_packet_ready"])
	}
	if ps["step_22_long_horizon_ready"] != true {
		t.Fatalf("step_22_long_horizon_ready=%v, want true", ps["step_22_long_horizon_ready"])
	}
}

func TestSeq168P107CallbackBiasCeiling(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq168-p107", Name: "a1", LastTurn: 1, Status: "active"},
			{ID: 2, ChatSessionID: "seq168-p107", Name: "a2", LastTurn: 2, Status: "active"},
			{ID: 3, ChatSessionID: "seq168-p107", Name: "a3", LastTurn: 3, Status: "active"},
			{ID: 4, ChatSessionID: "seq168-p107", Name: "a4", LastTurn: 4, Status: "active"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p107", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p107","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	cb := seq165Map(t, resp, "callback_bias_ceiling")
	if cb["version"] != "seq16_8_p107.v1" {
		t.Fatalf("version=%v, want seq16_8_p107.v1", cb["version"])
	}
	if cb["role"] != "callback_bias_ceiling" {
		t.Fatalf("role=%v, want callback_bias_ceiling", cb["role"])
	}
	if cb["soft_bias_enforced"] != true {
		t.Fatalf("soft_bias_enforced=%v, want true", cb["soft_bias_enforced"])
	}
}

func TestSeq168P108CallbackSceneAlignment(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq168-p108", Name: "a1", LastTurn: 1, Status: "active"},
		},
		returnActiveStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "seq168-p108", StateType: "scene", Content: "forest"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p108", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p108","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	ca := seq165Map(t, resp, "callback_scene_alignment")
	if ca["version"] != "seq16_8_p108.v1" {
		t.Fatalf("version=%v, want seq16_8_p108.v1", ca["version"])
	}
	if ca["role"] != "callback_scene_alignment" {
		t.Fatalf("role=%v, want callback_scene_alignment", ca["role"])
	}
	if ca["has_scene_state"] != true {
		t.Fatalf("has_scene_state=%v, want true", ca["has_scene_state"])
	}
	if ca["callback_rescue_alignment"] != "current_scene_first" {
		t.Fatalf("callback_rescue_alignment=%v, want current_scene_first", ca["callback_rescue_alignment"])
	}
}

func TestSeq168P109StaleCallbackSuppression(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq168-p109", Name: "a1", LastTurn: 1, Status: "resolved"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p109", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p109","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	sc := seq165Map(t, resp, "stale_callback_suppression")
	if sc["version"] != "seq16_8_p109.v1" {
		t.Fatalf("version=%v, want seq16_8_p109.v1", sc["version"])
	}
	if sc["role"] != "stale_callback_suppression" {
		t.Fatalf("role=%v, want stale_callback_suppression", sc["role"])
	}
	if sc["suppression_trigger"] != true {
		t.Fatalf("suppression_trigger=%v, want true", sc["suppression_trigger"])
	}
}

func TestSeq168P113OldArcForegroundVisibility(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq168-p113", Name: "arc1", LastTurn: 2, Status: "resolved"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p113", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p113","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	ov := seq165Map(t, resp, "old_arc_foreground_visibility")
	if ov["version"] != "seq16_8_p113.v1" {
		t.Fatalf("version=%v, want seq16_8_p113.v1", ov["version"])
	}
	if ov["role"] != "old_arc_foreground_visibility" {
		t.Fatalf("role=%v, want old_arc_foreground_visibility", ov["role"])
	}
	if ov["visibility_lane_ready"] != true {
		t.Fatalf("visibility_lane_ready=%v, want true", ov["visibility_lane_ready"])
	}
	vr, ok := ov["visible_reasons"].([]any)
	if !ok || len(vr) == 0 {
		t.Fatalf("visible_reasons empty or missing")
	}
}

func TestSeq168P114ReasonCodeVocabulary(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p114", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p114","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	vc := seq165Map(t, resp, "reason_code_vocabulary")
	if vc["version"] != "seq16_8_p114.v1" {
		t.Fatalf("version=%v, want seq16_8_p114.v1", vc["version"])
	}
	if vc["role"] != "reason_code_vocabulary" {
		t.Fatalf("role=%v, want reason_code_vocabulary", vc["role"])
	}
	vocab, ok := vc["vocabulary"].([]any)
	if !ok || len(vocab) == 0 {
		t.Fatalf("vocabulary empty or missing")
	}
}

func TestSeq168P115PreviewAuditTransparency(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p115", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p115","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	pt := seq165Map(t, resp, "preview_audit_transparency")
	if pt["version"] != "seq16_8_p115.v1" {
		t.Fatalf("version=%v, want seq16_8_p115.v1", pt["version"])
	}
	if pt["role"] != "preview_audit_transparency" {
		t.Fatalf("role=%v, want preview_audit_transparency", pt["role"])
	}
	if pt["preview_ready"] != true {
		t.Fatalf("preview_ready=%v, want true", pt["preview_ready"])
	}
	if pt["audit_ready"] != true {
		t.Fatalf("audit_ready=%v, want true", pt["audit_ready"])
	}
}

func TestSeq168P119ForegroundHijackTaxonomy(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq168-p119", Name: "arc1", LastTurn: 2, Status: "resolved"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p119", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p119","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	ft := seq165Map(t, resp, "foreground_hijack_taxonomy")
	if ft["version"] != "seq16_8_p119.v1" {
		t.Fatalf("version=%v, want seq16_8_p119.v1", ft["version"])
	}
	if ft["role"] != "foreground_hijack_taxonomy" {
		t.Fatalf("role=%v, want foreground_hijack_taxonomy", ft["role"])
	}
	if _, ok := ft["taxonomy_entries"]; !ok {
		t.Fatalf("missing taxonomy_entries")
	}
}

func TestSeq168P120DelayedPayoffSplit(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq168-p120", Name: "a1", LastTurn: 1, Status: "active"},
		},
		returnEpisodeSums: []store.EpisodeSummary{
			{ID: 1, ChatSessionID: "seq168-p120", FromTurn: 1, ToTurn: 5, SummaryText: "episode one"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p120", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p120","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	dp := seq165Map(t, resp, "delayed_payoff_split")
	if dp["version"] != "seq16_8_p120.v1" {
		t.Fatalf("version=%v, want seq16_8_p120.v1", dp["version"])
	}
	if dp["role"] != "delayed_payoff_split" {
		t.Fatalf("role=%v, want delayed_payoff_split", dp["role"])
	}
	if dp["delayed_payoff_rescue_ready"] != true {
		t.Fatalf("delayed_payoff_rescue_ready=%v, want true", dp["delayed_payoff_rescue_ready"])
	}
}

func TestSeq168P121RecallGainMonopolySplit(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq168-p121", Name: "a1", LastTurn: 1, Status: "active"},
			{ID: 2, ChatSessionID: "seq168-p121", Name: "a2", LastTurn: 2, Status: "resolved"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p121", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p121","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	rs := seq165Map(t, resp, "recall_gain_monopoly_split")
	if rs["version"] != "seq16_8_p121.v1" {
		t.Fatalf("version=%v, want seq16_8_p121.v1", rs["version"])
	}
	if rs["role"] != "recall_gain_monopoly_split" {
		t.Fatalf("role=%v, want recall_gain_monopoly_split", rs["role"])
	}
	if rs["split_trace_ready"] != true {
		t.Fatalf("split_trace_ready=%v, want true", rs["split_trace_ready"])
	}
}

func TestSeq168P125StaleArcRevivalReplay(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq168-p125", Name: "a1", LastTurn: 2, Status: "resolved"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p125", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p125","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	sr := seq165Map(t, resp, "stale_arc_revival_replay")
	if sr["version"] != "seq16_8_p125.v1" {
		t.Fatalf("version=%v, want seq16_8_p125.v1", sr["version"])
	}
	if sr["role"] != "stale_arc_revival_replay" {
		t.Fatalf("role=%v, want stale_arc_revival_replay", sr["role"])
	}
	if sr["single_incident_monopoly"] != true {
		t.Fatalf("single_incident_monopoly=%v, want true", sr["single_incident_monopoly"])
	}
	cands, ok := sr["revival_candidates"].([]any)
	if !ok || len(cands) == 0 {
		t.Fatalf("revival_candidates empty or missing")
	}
}

func TestSeq168P126TailRecallHijackGate(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq168-p126", Name: "a1", LastTurn: 2, Status: "resolved"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p126", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p126","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	th := seq165Map(t, resp, "tail_recall_hijack_gate")
	if th["version"] != "seq16_8_p126.v1" {
		t.Fatalf("version=%v, want seq16_8_p126.v1", th["version"])
	}
	if th["role"] != "tail_recall_hijack_gate" {
		t.Fatalf("role=%v, want tail_recall_hijack_gate", th["role"])
	}
	if th["gate_status"] != "closed" {
		t.Fatalf("gate_status=%v, want closed", th["gate_status"])
	}
}

func TestSeq168P127NarrativeDiversityGate(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq168-p127", Name: "a1", LastTurn: 1, Status: "active"},
			{ID: 2, ChatSessionID: "seq168-p127", Name: "a2", LastTurn: 2, Status: "active"},
		},
		returnWorldRules: []store.WorldRule{
			{ID: 1, ChatSessionID: "seq168-p127", Scope: "session", Category: "law", Key: "rule1"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p127", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p127","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	nd := seq165Map(t, resp, "narrative_diversity_gate")
	if nd["version"] != "seq16_8_p127.v1" {
		t.Fatalf("version=%v, want seq16_8_p127.v1", nd["version"])
	}
	if nd["role"] != "narrative_diversity_gate" {
		t.Fatalf("role=%v, want narrative_diversity_gate", nd["role"])
	}
	if nd["diversity_gate_open"] != true {
		t.Fatalf("diversity_gate_open=%v, want true", nd["diversity_gate_open"])
	}
}

func TestSeq168P128ArcMonopolyGate(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq168-p128", Name: "a1", LastTurn: 1, Status: "active"},
			{ID: 2, ChatSessionID: "seq168-p128", Name: "a2", LastTurn: 2, Status: "resolved"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p128", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p128","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	am := seq165Map(t, resp, "arc_monopoly_gate")
	if am["version"] != "seq16_8_p128.v1" {
		t.Fatalf("version=%v, want seq16_8_p128.v1", am["version"])
	}
	if am["role"] != "arc_monopoly_gate" {
		t.Fatalf("role=%v, want arc_monopoly_gate", am["role"])
	}
	if am["gate_status"] != "closed" {
		t.Fatalf("gate_status=%v, want closed", am["gate_status"])
	}
}

func TestSeq168P132JSContinuityRescue(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq168-p132", Name: "a1", LastTurn: 1, Status: "active"},
		},
		returnPendingThreads: []store.PendingThread{
			{ID: 1, ChatSessionID: "seq168-p132", ThreadKey: "t1", CreatedTurn: 1, Status: "open"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p132", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p132","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	jr := seq165Map(t, resp, "js_continuity_rescue")
	if jr["version"] != "seq16_8_p132.v1" {
		t.Fatalf("version=%v, want seq16_8_p132.v1", jr["version"])
	}
	if jr["role"] != "js_continuity_rescue" {
		t.Fatalf("role=%v, want js_continuity_rescue", jr["role"])
	}
	if jr["js_owner"] != "archive_center_js" {
		t.Fatalf("js_owner=%v, want archive_center_js", jr["js_owner"])
	}
	funcs, ok := jr["js_functions"].([]any)
	if !ok || len(funcs) == 0 {
		t.Fatalf("js_functions empty or missing")
	}
}

func TestSeq168P133JSPromptAssemblyGuard(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnMemories: []store.Memory{
			{ID: 1, ChatSessionID: "seq168-p133", TurnIndex: 1, SummaryJSON: `{"text":"memory"}`},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p133", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p133","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	jg := seq165Map(t, resp, "js_prompt_assembly_guard")
	if jg["version"] != "seq16_8_p133.v1" {
		t.Fatalf("version=%v, want seq16_8_p133.v1", jg["version"])
	}
	if jg["role"] != "js_prompt_assembly_guard" {
		t.Fatalf("role=%v, want js_prompt_assembly_guard", jg["role"])
	}
	if jg["js_owner"] != "archive_center_js" {
		t.Fatalf("js_owner=%v, want archive_center_js", jg["js_owner"])
	}
}

func TestSeq168P134JSTracePreviewTransparency(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p134", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p134","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	jt := seq165Map(t, resp, "js_trace_preview_transparency")
	if jt["version"] != "seq16_8_p134.v1" {
		t.Fatalf("version=%v, want seq16_8_p134.v1", jt["version"])
	}
	if jt["role"] != "js_trace_preview_transparency" {
		t.Fatalf("role=%v, want js_trace_preview_transparency", jt["role"])
	}
	if jt["trace_preview_ready"] != true {
		t.Fatalf("trace_preview_ready=%v, want true", jt["trace_preview_ready"])
	}
}

func TestSeq168P135ReplayCorpusBaseline(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq168-p135", Name: "a1", LastTurn: 2, Status: "resolved"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p135", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p135","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	rc := seq165Map(t, resp, "replay_corpus_baseline")
	if rc["version"] != "seq16_8_p135.v1" {
		t.Fatalf("version=%v, want seq16_8_p135.v1", rc["version"])
	}
	if rc["role"] != "replay_corpus_baseline" {
		t.Fatalf("role=%v, want replay_corpus_baseline", rc["role"])
	}
	if _, ok := rc["corpus_entries"]; !ok {
		t.Fatalf("missing corpus_entries")
	}
	cases, ok := rc["cases"].([]any)
	if !ok || len(cases) == 0 {
		t.Fatalf("cases empty or missing")
	}
}

func TestSeq168P136BackendMetadataAlignment(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq168-p136", Name: "a1", LastTurn: 1, Status: "active"},
		},
		returnPendingThreads: []store.PendingThread{
			{ID: 1, ChatSessionID: "seq168-p136", ThreadKey: "t1", CreatedTurn: 1, Status: "open"},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p136", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p136","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	bm := seq165Map(t, resp, "backend_metadata_alignment")
	if bm["version"] != "seq16_8_p136.v1" {
		t.Fatalf("version=%v, want seq16_8_p136.v1", bm["version"])
	}
	if bm["role"] != "backend_metadata_alignment" {
		t.Fatalf("role=%v, want backend_metadata_alignment", bm["role"])
	}
	if bm["metadata_aligned"] != true {
		t.Fatalf("metadata_aligned=%v, want true", bm["metadata_aligned"])
	}
	if bm["suppression_trace_confirmed"] != true {
		t.Fatalf("suppression_trace_confirmed=%v, want true", bm["suppression_trace_confirmed"])
	}
}

// SEQ-16.8-P162: Decision outcome — no-user-mention stale arc ceiling is judged
// by explicit alignment / current-scene evidence / explicit redirection, not by
// turn-gap alone. Turn-gap is used only as a pressure signal.
func TestSeq168P162DecisionOutcomeCeilingNotTurnGap(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	// Case A: large turn-gap but NO explicit alignment / scene evidence / redirection
	// → stale arc should NOT get auto-foreground mandate (ceiling applies)
	bodyA := `{"chat_session_id":"seq168-p162-a","turn_index":10,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	reqA := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(bodyA))
	reqA.Header.Set("Content-Type", "application/json")
	recA := httptest.NewRecorder()
	mux.ServeHTTP(recA, reqA)
	if recA.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recA.Code, recA.Body.String())
	}
	var respA map[string]any
	if err := json.Unmarshal(recA.Body.Bytes(), &respA); err != nil {
		t.Fatalf("decode: %v", err)
	}
	ceilingA := seq165Map(t, respA, "stale_arc_ceiling")
	if ceilingA["auto_foreground_mandate"] != false {
		t.Fatalf("case A: auto_foreground_mandate=%v, want false (no explicit alignment)", ceilingA["auto_foreground_mandate"])
	}
	if ceilingA["judged_by_turn_gap_alone"] != false {
		t.Fatalf("case A: judged_by_turn_gap_alone=%v, want false", ceilingA["judged_by_turn_gap_alone"])
	}
	if ceilingA["pressure_signal_only"] != true {
		t.Fatalf("case A: pressure_signal_only=%v, want true", ceilingA["pressure_signal_only"])
	}

	// Case B: explicit query alignment present → amplification allowed
	bodyB := `{"chat_session_id":"seq168-p162-b","turn_index":5,"raw_user_input":"what happened to the old arc","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	reqB := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(bodyB))
	reqB.Header.Set("Content-Type", "application/json")
	recB := httptest.NewRecorder()
	mux.ServeHTTP(recB, reqB)
	if recB.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recB.Code, recB.Body.String())
	}
	var respB map[string]any
	if err := json.Unmarshal(recB.Body.Bytes(), &respB); err != nil {
		t.Fatalf("decode: %v", err)
	}
	alignB := seq165Map(t, respB, "scene_alignment")
	if alignB["explicit_query_alignment"] != true {
		t.Fatalf("case B: explicit_query_alignment=%v, want true", alignB["explicit_query_alignment"])
	}
	if alignB["amplification_allowed"] != true {
		t.Fatalf("case B: amplification_allowed=%v, want true", alignB["amplification_allowed"])
	}

	// Case C: current-scene evidence present → amplification allowed
	bodyC := `{"chat_session_id":"seq168-p162-c","turn_index":3,"raw_user_input":"continue the forest scene","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	reqC := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(bodyC))
	reqC.Header.Set("Content-Type", "application/json")
	recC := httptest.NewRecorder()
	mux.ServeHTTP(recC, reqC)
	if recC.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recC.Code, recC.Body.String())
	}
	var respC map[string]any
	if err := json.Unmarshal(recC.Body.Bytes(), &respC); err != nil {
		t.Fatalf("decode: %v", err)
	}
	alignC := seq165Map(t, respC, "scene_alignment")
	if alignC["current_scene_evidence"] != true {
		t.Fatalf("case C: current_scene_evidence=%v, want true", alignC["current_scene_evidence"])
	}
	if alignC["amplification_allowed"] != true {
		t.Fatalf("case C: amplification_allowed=%v, want true", alignC["amplification_allowed"])
	}

	// Case D: explicit user redirection is a separate decision axis. It should
	// demote stale arcs and preserve the current user direction without relying
	// on turn-gap pressure.
	bodyD := `{"chat_session_id":"seq168-p162-d","turn_index":4,"raw_user_input":"move on instead","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	reqD := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(bodyD))
	reqD.Header.Set("Content-Type", "application/json")
	recD := httptest.NewRecorder()
	mux.ServeHTTP(recD, reqD)
	if recD.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recD.Code, recD.Body.String())
	}
	var respD map[string]any
	if err := json.Unmarshal(recD.Body.Bytes(), &respD); err != nil {
		t.Fatalf("decode: %v", err)
	}
	govD := seq165Map(t, respD, "input_anchor_governor")
	redirectionD := seq165Map(t, govD, "explicit_user_redirection")
	if redirectionD["detected"] != true {
		t.Fatalf("case D: explicit_user_redirection.detected=%v, want true", redirectionD["detected"])
	}
	if redirectionD["stale_arc_demotes"] != true || redirectionD["current_user_input_wins"] != true || redirectionD["support_lane_may_redirect"] != false {
		t.Fatalf("case D: explicit_user_redirection guard mismatch: %v", redirectionD)
	}
	ceilingD := seq165Map(t, respD, "stale_arc_ceiling")
	if ceilingD["judged_by_turn_gap_alone"] != false || ceilingD["pressure_signal_only"] != true {
		t.Fatalf("case D: stale_arc_ceiling turn-gap guard mismatch: %v", ceilingD)
	}
}

// SEQ-16.8-P163: current-scene evidence minimum criteria.
// active state / latest direct evidence / recent raw turn token overlap.
func TestSeq168P163CurrentSceneEvidenceMinCriteria(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnActiveStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "seq168-p163", StateType: "scene", Content: "forest dark path"},
		},
		returnEvidence: []store.DirectEvidence{
			{ID: 1, ChatSessionID: "seq168-p163", EvidenceText: "dark path lantern is visible", TurnAnchor: 5},
		},
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "seq168-p163", TurnIndex: 5, Role: "user", Content: "walk along the dark path with the lantern in the forest"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p163","turn_index":5,"raw_user_input":"walk along the dark path with the lantern in the forest","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	criteria := seq165Map(t, resp, "current_scene_evidence_min_criteria")
	if criteria["version"] != "seq16_8_p163.v1" {
		t.Fatalf("version=%v, want seq16_8_p163.v1", criteria["version"])
	}
	if criteria["role"] != "current_scene_evidence_min_criteria" {
		t.Fatalf("role=%v, want current_scene_evidence_min_criteria", criteria["role"])
	}
	if criteria["active_state_count"] != float64(1) {
		t.Fatalf("active_state_count=%v, want 1", criteria["active_state_count"])
	}
	if criteria["active_state_text"] != "forest dark path" {
		t.Fatalf("active_state_text=%v, want forest dark path", criteria["active_state_text"])
	}
	if criteria["latest_direct_evidence"] != "dark path lantern is visible" {
		t.Fatalf("latest_direct_evidence=%v, want dark path lantern is visible", criteria["latest_direct_evidence"])
	}
	if criteria["recent_raw_turn"] != "walk along the dark path with the lantern in the forest" {
		t.Fatalf("recent_raw_turn=%v, want walk along the dark path with the lantern in the forest", criteria["recent_raw_turn"])
	}
	if criteria["active_state_token_overlap_count"] == float64(0) {
		t.Fatalf("active_state_token_overlap_count=%v, want > 0", criteria["active_state_token_overlap_count"])
	}
	if criteria["latest_direct_evidence_token_overlap_count"] == float64(0) {
		t.Fatalf("latest_direct_evidence_token_overlap_count=%v, want > 0", criteria["latest_direct_evidence_token_overlap_count"])
	}
	if criteria["min_criteria_met"] != true {
		t.Fatalf("min_criteria_met=%v, want true", criteria["min_criteria_met"])
	}
	if criteria["inspectable"] != true {
		t.Fatalf("inspectable=%v, want true", criteria["inspectable"])
	}
}

// SEQ-16.8-P164: open / paused thread ceiling family, pending_threads guard.
func TestSeq168P164PendingThreadsGuard(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnPendingThreads: []store.PendingThread{
			{ID: 1, ChatSessionID: "seq168-p164", Title: "thread1", Status: "open"},
			{ID: 2, ChatSessionID: "seq168-p164", Title: "thread2", Status: "paused"},
			{ID: 3, ChatSessionID: "seq168-p164", Title: "thread3", Status: "resolved"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p164","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	guard := seq165Map(t, resp, "pending_threads_guard")
	if guard["version"] != "seq16_8_p164.v1" {
		t.Fatalf("version=%v, want seq16_8_p164.v1", guard["version"])
	}
	if guard["role"] != "pending_threads_guard" {
		t.Fatalf("role=%v, want pending_threads_guard", guard["role"])
	}
	if guard["open_count"] != float64(1) {
		t.Fatalf("open_count=%v, want 1", guard["open_count"])
	}
	if guard["paused_count"] != float64(1) {
		t.Fatalf("paused_count=%v, want 1", guard["paused_count"])
	}
	if guard["pending_total"] != float64(2) {
		t.Fatalf("pending_total=%v, want 2", guard["pending_total"])
	}
	if guard["guard_active"] != true {
		t.Fatalf("guard_active=%v, want true", guard["guard_active"])
	}
	if guard["ceiling_family"] != "stale_arc_ceiling" {
		t.Fatalf("ceiling_family=%v, want stale_arc_ceiling", guard["ceiling_family"])
	}
	if guard["suppress_foreground"] != true {
		t.Fatalf("suppress_foreground=%v, want true", guard["suppress_foreground"])
	}
}

// SEQ-16.8-P165: reason visibility lane extends to adaptive trace / continuity
// trace / input transparency.
func TestSeq168P165ReasonVisibilityLane(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p165","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	trace := seq165Map(t, resp, "reason_trace")
	if trace["version"] != "seq16_8_p101.v1" {
		t.Fatalf("version=%v, want seq16_8_p101.v1", trace["version"])
	}
	if trace["role"] != "reason_trace" {
		t.Fatalf("role=%v, want reason_trace", trace["role"])
	}
	if trace["inspectable"] != true {
		t.Fatalf("inspectable=%v, want true", trace["inspectable"])
	}
	if trace["adaptive_trace_visible"] != true {
		t.Fatalf("adaptive_trace_visible=%v, want true", trace["adaptive_trace_visible"])
	}
	if trace["continuity_trace_visible"] != true {
		t.Fatalf("continuity_trace_visible=%v, want true", trace["continuity_trace_visible"])
	}
	if trace["input_transparency_visible"] != true {
		t.Fatalf("input_transparency_visible=%v, want true", trace["input_transparency_visible"])
	}
}

// SEQ-16.8-P166: diversity gate default diagnostic warn, arc_monopoly_attempt
// Step 17 handoff block signal.
func TestSeq168P166DiversityGateDiagnosticWarn(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{
		returnStorylines: []store.Storyline{
			{ID: 1, ChatSessionID: "seq168-p166", Name: "arc1", Status: "active"},
		},
	}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p166","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	gate := seq165Map(t, resp, "narrative_diversity_gate")
	if gate["version"] != "seq16_8_p127.v1" {
		t.Fatalf("version=%v, want seq16_8_p127.v1", gate["version"])
	}
	if gate["role"] != "narrative_diversity_gate" {
		t.Fatalf("role=%v, want narrative_diversity_gate", gate["role"])
	}
	if gate["diagnostic_warn"] != true {
		t.Fatalf("diagnostic_warn=%v, want true", gate["diagnostic_warn"])
	}
	if gate["arc_monopoly_attempt"] != true {
		t.Fatalf("arc_monopoly_attempt=%v, want true (single storyline)", gate["arc_monopoly_attempt"])
	}
	if gate["step_17_handoff_block"] != true {
		t.Fatalf("step_17_handoff_block=%v, want true", gate["step_17_handoff_block"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-17 contract tests (P230 ~ P242)
// ---------------------------------------------------------------------------

// SEQ-17-P230: retrieval completeness vs final answer quality split.
func TestSeq17P230EvaluationSplit(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p230","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	split := seq165Map(t, resp, "evaluation_split")
	if split["version"] != "seq17_p230.v1" {
		t.Fatalf("version=%v, want seq17_p230.v1", split["version"])
	}
	if split["role"] != "evaluation_split" {
		t.Fatalf("role=%v, want evaluation_split", split["role"])
	}
	if _, ok := split["retrieval_completeness"]; !ok {
		t.Fatalf("retrieval_completeness missing")
	}
	if _, ok := split["final_answer_quality"]; !ok {
		t.Fatalf("final_answer_quality missing")
	}
	if split["inspectable"] != true {
		t.Fatalf("inspectable=%v, want true", split["inspectable"])
	}
}

// SEQ-17-P231: ops procedure documentation surface.
func TestSeq17P231OpsProcedureSurface(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p231","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	ops := seq165Map(t, resp, "ops_procedure_surface")
	if ops["version"] != "seq17_p231.v1" {
		t.Fatalf("version=%v, want seq17_p231.v1", ops["version"])
	}
	if ops["role"] != "ops_procedure_surface" {
		t.Fatalf("role=%v, want ops_procedure_surface", ops["role"])
	}
	if ops["documented"] != true {
		t.Fatalf("documented=%v, want true", ops["documented"])
	}
	procedures, ok := ops["procedures"].([]any)
	if !ok || len(procedures) == 0 {
		t.Fatalf("procedures missing or empty")
	}
}

// SEQ-17-P232: inspection lane boundary surface.
func TestSeq17P232InspectionLaneBoundary(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p232","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	boundary := seq165Map(t, resp, "inspection_lane_boundary")
	if boundary["version"] != "seq17_p232.v1" {
		t.Fatalf("version=%v, want seq17_p232.v1", boundary["version"])
	}
	if boundary["role"] != "inspection_lane_boundary" {
		t.Fatalf("role=%v, want inspection_lane_boundary", boundary["role"])
	}
	if boundary["boundary_clear"] != true {
		t.Fatalf("boundary_clear=%v, want true", boundary["boundary_clear"])
	}
	lanes, ok := boundary["lanes"].([]any)
	if !ok || len(lanes) == 0 {
		t.Fatalf("lanes missing or empty")
	}
}

// SEQ-17-P233: adoption gate — replay green before default adoption value.
func TestSeq17P233AdoptionGate(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p233","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	gate := seq165Map(t, resp, "adoption_gate")
	if gate["version"] != "seq17_p233.v1" {
		t.Fatalf("version=%v, want seq17_p233.v1", gate["version"])
	}
	if gate["role"] != "adoption_gate" {
		t.Fatalf("role=%v, want adoption_gate", gate["role"])
	}
	if gate["default_adoption"] != false {
		t.Fatalf("default_adoption=%v, want false", gate["default_adoption"])
	}
	if gate["replay_green"] != false {
		t.Fatalf("replay_green=%v, want false by default", gate["replay_green"])
	}
	if gate["adoption_blocked"] != true {
		t.Fatalf("adoption_blocked=%v, want true before replay green", gate["adoption_blocked"])
	}
}

// SEQ-17-P234: release hygiene — bundle/regression/checklist repeatability.
func TestSeq17P234ReleaseHygiene(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p234","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	hygiene := seq165Map(t, resp, "release_hygiene")
	if hygiene["version"] != "seq17_p234.v1" {
		t.Fatalf("version=%v, want seq17_p234.v1", hygiene["version"])
	}
	if hygiene["role"] != "release_hygiene" {
		t.Fatalf("role=%v, want release_hygiene", hygiene["role"])
	}
	if hygiene["bundle_repeatable"] != true {
		t.Fatalf("bundle_repeatable=%v, want true", hygiene["bundle_repeatable"])
	}
	if hygiene["regression_repeatable"] != true {
		t.Fatalf("regression_repeatable=%v, want true", hygiene["regression_repeatable"])
	}
	if hygiene["checklist_repeatable"] != true {
		t.Fatalf("checklist_repeatable=%v, want true", hygiene["checklist_repeatable"])
	}
}

// SEQ-17-P238: 17-1a retrieval completeness metric define.
func TestSeq17P238RetrievalCompletenessMetric(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p238","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	metric := seq165Map(t, resp, "retrieval_completeness_metric")
	if metric["version"] != "seq17_p238.v1" {
		t.Fatalf("version=%v, want seq17_p238.v1", metric["version"])
	}
	if metric["role"] != "retrieval_completeness_metric" {
		t.Fatalf("role=%v, want retrieval_completeness_metric", metric["role"])
	}
	if metric["metric_defined"] != true {
		t.Fatalf("metric_defined=%v, want true", metric["metric_defined"])
	}
}

// SEQ-17-P239: 17-1b final answer quality metric define.
func TestSeq17P239FinalAnswerQualityMetric(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p239","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	metric := seq165Map(t, resp, "final_answer_quality_metric")
	if metric["version"] != "seq17_p239.v1" {
		t.Fatalf("version=%v, want seq17_p239.v1", metric["version"])
	}
	if metric["role"] != "final_answer_quality_metric" {
		t.Fatalf("role=%v, want final_answer_quality_metric", metric["role"])
	}
	if metric["metric_defined"] != true {
		t.Fatalf("metric_defined=%v, want true", metric["metric_defined"])
	}
}

// SEQ-17-P240: 17-1c retrieval failure vs reader failure split replay define.
func TestSeq17P240FailureSplitReplay(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p240","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	replay := seq165Map(t, resp, "failure_split_replay")
	if replay["version"] != "seq17_p240.v1" {
		t.Fatalf("version=%v, want seq17_p240.v1", replay["version"])
	}
	if replay["role"] != "failure_split_replay" {
		t.Fatalf("role=%v, want failure_split_replay", replay["role"])
	}
	if replay["replay_defined"] != true {
		t.Fatalf("replay_defined=%v, want true", replay["replay_defined"])
	}
	if _, ok := replay["failure_class"]; !ok {
		t.Fatalf("failure_class missing")
	}
}

// SEQ-17-P241: 17-1d Step 14~16 regression corpus define.
func TestSeq17P241RegressionCorpus(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p241","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	corpus := seq165Map(t, resp, "regression_corpus")
	if corpus["version"] != "seq17_p241.v1" {
		t.Fatalf("version=%v, want seq17_p241.v1", corpus["version"])
	}
	if corpus["role"] != "regression_corpus" {
		t.Fatalf("role=%v, want regression_corpus", corpus["role"])
	}
	if corpus["corpus_defined"] != true {
		t.Fatalf("corpus_defined=%v, want true", corpus["corpus_defined"])
	}
	steps, ok := corpus["corpus_steps"].([]any)
	if !ok || len(steps) == 0 {
		t.Fatalf("corpus_steps missing or empty")
	}
}

// SEQ-17-P242: 17-1e freshness lag metric define.
func TestSeq17P242FreshnessLagMetric(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p242","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	metric := seq165Map(t, resp, "freshness_lag_metric")
	if metric["version"] != "seq17_p242.v1" {
		t.Fatalf("version=%v, want seq17_p242.v1", metric["version"])
	}
	if metric["role"] != "freshness_lag_metric" {
		t.Fatalf("role=%v, want freshness_lag_metric", metric["role"])
	}
	if metric["metric_defined"] != true {
		t.Fatalf("metric_defined=%v, want true", metric["metric_defined"])
	}
	if _, ok := metric["extraction_delay_ms"]; !ok {
		t.Fatalf("extraction_delay_ms missing")
	}
	if _, ok := metric["save_delay_ms"]; !ok {
		t.Fatalf("save_delay_ms missing")
	}
	if _, ok := metric["promotion_visibility_lag_ms"]; !ok {
		t.Fatalf("promotion_visibility_lag_ms missing")
	}
}

// ---------------------------------------------------------------------------
// SEQ-17-2 ops procedure tests (P286 ~ P290)
// ---------------------------------------------------------------------------

// SEQ-17-P286: 17-2a promotion / backfill / rebuild document.
func TestSeq17P286PromotionBackfillRebuild(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p286","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	surface := seq165Map(t, resp, "promotion_backfill_rebuild")
	if surface["version"] != "seq17_p286.v1" {
		t.Fatalf("version=%v, want seq17_p286.v1", surface["version"])
	}
	if surface["role"] != "promotion_backfill_rebuild" {
		t.Fatalf("role=%v, want promotion_backfill_rebuild", surface["role"])
	}
	if surface["documented"] != true {
		t.Fatalf("documented=%v, want true", surface["documented"])
	}
	procedures, ok := surface["procedures"].([]any)
	if !ok || len(procedures) == 0 {
		t.Fatalf("procedures missing or empty")
	}
	seen := map[string]bool{}
	for _, raw := range procedures {
		item, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("procedure is %T, want map", raw)
		}
		name, _ := item["name"].(string)
		seen[name] = true
		if item["dry_run"] != true {
			t.Fatalf("procedure %q dry_run=%v, want true", name, item["dry_run"])
		}
	}
	for _, name := range []string{"promotion", "backfill", "rebuild"} {
		if !seen[name] {
			t.Fatalf("procedures missing %q: %#v", name, procedures)
		}
	}
}

// SEQ-17-P287: 17-2b reembed / migration / health probe document.
func TestSeq17P287ReembedMigrationHealthProbe(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p287","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	surface := seq165Map(t, resp, "reembed_migration_health_probe")
	if surface["version"] != "seq17_p287.v1" {
		t.Fatalf("version=%v, want seq17_p287.v1", surface["version"])
	}
	if surface["role"] != "reembed_migration_health_probe" {
		t.Fatalf("role=%v, want reembed_migration_health_probe", surface["role"])
	}
	if surface["documented"] != true {
		t.Fatalf("documented=%v, want true", surface["documented"])
	}
	procedures, ok := surface["procedures"].([]any)
	if !ok || len(procedures) == 0 {
		t.Fatalf("procedures missing or empty")
	}
	seen := map[string]bool{}
	for _, raw := range procedures {
		item, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("procedure is %T, want map", raw)
		}
		name, _ := item["name"].(string)
		seen[name] = true
		if item["dry_run"] != true {
			t.Fatalf("procedure %q dry_run=%v, want true", name, item["dry_run"])
		}
	}
	for _, name := range []string{"reembed", "migration", "health_probe"} {
		if !seen[name] {
			t.Fatalf("procedures missing %q: %#v", name, procedures)
		}
	}
}

// SEQ-17-P288: 17-2c failure mode / fallback / rollback runbook cleanup.
func TestSeq17P288FailureFallbackRollback(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p288","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	surface := seq165Map(t, resp, "failure_fallback_rollback")
	if surface["version"] != "seq17_p288.v1" {
		t.Fatalf("version=%v, want seq17_p288.v1", surface["version"])
	}
	if surface["role"] != "failure_fallback_rollback" {
		t.Fatalf("role=%v, want failure_fallback_rollback", surface["role"])
	}
	if surface["documented"] != true {
		t.Fatalf("documented=%v, want true", surface["documented"])
	}
	items, ok := surface["runbook_items"].([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("runbook_items missing or empty")
	}
	seen := map[string]bool{}
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("runbook item is %T, want map", raw)
		}
		seen[item["name"].(string)] = true
		if item["status"] != "documented" {
			t.Fatalf("runbook item status=%v, want documented", item["status"])
		}
	}
	for _, name := range []string{"failure_mode", "fallback", "rollback"} {
		if !seen[name] {
			t.Fatalf("runbook_items missing %q: %#v", name, items)
		}
	}
}

// SEQ-17-P289: 17-2d async complete-turn / critic delay runbook cleanup.
func TestSeq17P289AsyncCriticDelay(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p289","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	surface := seq165Map(t, resp, "async_critic_delay")
	if surface["version"] != "seq17_p289.v1" {
		t.Fatalf("version=%v, want seq17_p289.v1", surface["version"])
	}
	if surface["role"] != "async_critic_delay" {
		t.Fatalf("role=%v, want async_critic_delay", surface["role"])
	}
	if surface["documented"] != true {
		t.Fatalf("documented=%v, want true", surface["documented"])
	}
	items, ok := surface["runbook_items"].([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("runbook_items missing or empty")
	}
	seen := map[string]bool{}
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("runbook item is %T, want map", raw)
		}
		seen[item["name"].(string)] = true
		if item["status"] != "documented" {
			t.Fatalf("runbook item status=%v, want documented", item["status"])
		}
	}
	for _, name := range []string{"async_complete_turn", "critic_delay", "freshness_lag_repair", "replay"} {
		if !seen[name] {
			t.Fatalf("runbook_items missing %q: %#v", name, items)
		}
	}
}

// SEQ-17-P290: 17-2e partial-write / silent-skip / retry budget cleanup.
func TestSeq17P290PartialWriteRetry(t *testing.T) {
	srv := setupTestServer()
	srv.Store = &turnRecordingStore{}
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p290","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	surface := seq165Map(t, resp, "partial_write_retry")
	if surface["version"] != "seq17_p290.v1" {
		t.Fatalf("version=%v, want seq17_p290.v1", surface["version"])
	}
	if surface["role"] != "partial_write_retry" {
		t.Fatalf("role=%v, want partial_write_retry", surface["role"])
	}
	if surface["documented"] != true {
		t.Fatalf("documented=%v, want true", surface["documented"])
	}
	if surface["warning_only_fail_blocked"] != true {
		t.Fatalf("warning_only_fail_blocked=%v, want true", surface["warning_only_fail_blocked"])
	}
	policies, ok := surface["policies"].([]any)
	if !ok || len(policies) == 0 {
		t.Fatalf("policies missing or empty")
	}
	seen := map[string]bool{}
	for _, raw := range policies {
		item, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("policy is %T, want map", raw)
		}
		name, _ := item["name"].(string)
		seen[name] = true
		if item["warning_only"] != false {
			t.Fatalf("policy %q warning_only=%v, want false", name, item["warning_only"])
		}
	}
	for _, name := range []string{"partial_write", "silent_skip", "retry_budget"} {
		if !seen[name] {
			t.Fatalf("policies missing %q: %#v", name, policies)
		}
	}
}

// TestSeq17P306ExplainSurface validates the explain surface role for
// SEQ-17-P306: 17-3a explain surface 역할 정의.
func TestSeq17P306ExplainSurface(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p306","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	surface := seq165Map(t, resp, "explain_surface")
	if surface["version"] != "seq17_p306.v1" {
		t.Fatalf("version=%v, want seq17_p306.v1", surface["version"])
	}
	if surface["role"] != "explain_surface" {
		t.Fatalf("role=%v, want explain_surface", surface["role"])
	}
	if surface["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", surface["truth_authority"])
	}
	if surface["purpose"] != "reasoning_exposure" {
		t.Fatalf("purpose=%v, want reasoning_exposure", surface["purpose"])
	}
	if surface["inspection_only"] != true {
		t.Fatalf("inspection_only=%v, want true", surface["inspection_only"])
	}
	if surface["mutable"] != false {
		t.Fatalf("mutable=%v, want false", surface["mutable"])
	}
	if surface["mode"] != "explain_surface_role" {
		t.Fatalf("mode=%v, want explain_surface_role", surface["mode"])
	}
}

// TestSeq17P307PreviewAuditSurface validates the preview / audit surface roles for
// SEQ-17-P307: 17-3b preview / audit surface 역할 정의.
func TestSeq17P307PreviewAuditSurface(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p307","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	surface := seq165Map(t, resp, "preview_audit_surface")
	if surface["version"] != "seq17_p307.v1" {
		t.Fatalf("version=%v, want seq17_p307.v1", surface["version"])
	}
	if surface["role"] != "preview_audit_surface" {
		t.Fatalf("role=%v, want preview_audit_surface", surface["role"])
	}
	if surface["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", surface["truth_authority"])
	}
	if surface["preview_purpose"] != "outcome_preview" {
		t.Fatalf("preview_purpose=%v, want outcome_preview", surface["preview_purpose"])
	}
	if surface["audit_purpose"] != "decision_audit" {
		t.Fatalf("audit_purpose=%v, want decision_audit", surface["audit_purpose"])
	}
	if surface["inspection_only"] != true {
		t.Fatalf("inspection_only=%v, want true", surface["inspection_only"])
	}
	if surface["mutable"] != false {
		t.Fatalf("mutable=%v, want false", surface["mutable"])
	}
	if surface["mode"] != "preview_audit_surface_role" {
		t.Fatalf("mode=%v, want preview_audit_surface_role", surface["mode"])
	}
}

// TestSeq17P308DashboardLane validates the dashboard lane split rules for
// SEQ-17-P308: 17-3c dashboard lane 분리 규칙 정의.
func TestSeq17P308DashboardLane(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p308","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	surface := seq165Map(t, resp, "dashboard_lane")
	if surface["version"] != "seq17_p308.v1" {
		t.Fatalf("version=%v, want seq17_p308.v1", surface["version"])
	}
	if surface["role"] != "dashboard_lane" {
		t.Fatalf("role=%v, want dashboard_lane", surface["role"])
	}
	if surface["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", surface["truth_authority"])
	}
	if surface["purpose"] != "metric_dashboard" {
		t.Fatalf("purpose=%v, want metric_dashboard", surface["purpose"])
	}
	if surface["inspection_only"] != true {
		t.Fatalf("inspection_only=%v, want true", surface["inspection_only"])
	}
	if surface["mutable"] != false {
		t.Fatalf("mutable=%v, want false", surface["mutable"])
	}
	lanes, ok := surface["lanes"].([]any)
	if !ok || len(lanes) == 0 {
		t.Fatalf("lanes missing or empty")
	}
	seen := map[string]bool{}
	for _, raw := range lanes {
		item, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("lane is %T, want map", raw)
		}
		name, _ := item["name"].(string)
		seen[name] = true
	}
	for _, name := range []string{"save", "extraction", "promotion"} {
		if !seen[name] {
			t.Fatalf("lanes missing %q: %#v", name, lanes)
		}
	}
	if surface["mode"] != "dashboard_lane_split" {
		t.Fatalf("mode=%v, want dashboard_lane_split", surface["mode"])
	}
}

// TestSeq17P309DisplayGuard validates the display guard that prevents the
// inspection surface from appearing as an authority for SEQ-17-P309: 17-3d
// inspection surface authority display guard 정의.
func TestSeq17P309DisplayGuard(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p309","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	surface := seq165Map(t, resp, "display_guard")
	if surface["version"] != "seq17_p309.v1" {
		t.Fatalf("version=%v, want seq17_p309.v1", surface["version"])
	}
	if surface["role"] != "display_guard" {
		t.Fatalf("role=%v, want display_guard", surface["role"])
	}
	if surface["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", surface["truth_authority"])
	}
	if surface["guard_active"] != true {
		t.Fatalf("guard_active=%v, want true", surface["guard_active"])
	}
	if surface["canonical_truth_source"] != "canonical_store" {
		t.Fatalf("canonical_truth_source=%v, want canonical_store", surface["canonical_truth_source"])
	}
	sources, ok := surface["authority_sources"].([]any)
	if !ok || len(sources) == 0 {
		t.Fatalf("authority_sources missing or empty")
	}
	sourceSeen := map[string]bool{}
	for _, raw := range sources {
		if s, ok := raw.(string); ok {
			sourceSeen[s] = true
		}
	}
	for _, name := range []string{"canonical_store", "direct_evidence"} {
		if !sourceSeen[name] {
			t.Fatalf("authority_sources missing %q: %#v", name, sources)
		}
	}
	note, _ := surface["note"].(string)
	if !strings.Contains(note, "Canonical store truth") {
		t.Fatalf("note missing canonical store truth reference: %q", note)
	}
	if !strings.Contains(note, "never owns mutation") {
		t.Fatalf("note missing never owns mutation reference: %q", note)
	}
	if surface["mode"] != "inspection_surface_display_guard" {
		t.Fatalf("mode=%v, want inspection_surface_display_guard", surface["mode"])
	}
}

// TestSeq17P310VisibilityLane validates the freshness / extract-drop /
// promotion-block visibility lane with save state/status split for
// SEQ-17-P310: 17-3e freshness / extract-drop / promotion-block visibility lane
// 정의.
func TestSeq17P310VisibilityLane(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p310","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	surface := seq165Map(t, resp, "visibility_lane")
	if surface["version"] != "seq17_p310.v1" {
		t.Fatalf("version=%v, want seq17_p310.v1", surface["version"])
	}
	if surface["role"] != "visibility_lane" {
		t.Fatalf("role=%v, want visibility_lane", surface["role"])
	}
	if surface["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", surface["truth_authority"])
	}
	if surface["save_state_status_split"] != true {
		t.Fatalf("save_state_status_split=%v, want true", surface["save_state_status_split"])
	}
	lanes, ok := surface["lanes"].([]any)
	if !ok || len(lanes) == 0 {
		t.Fatalf("lanes missing or empty")
	}
	seen := map[string]bool{}
	for _, raw := range lanes {
		item, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("lane is %T, want map", raw)
		}
		name, _ := item["name"].(string)
		seen[name] = true
		state, _ := item["state"].(string)
		status, _ := item["status"].(string)
		if state == "" {
			t.Fatalf("lane %q missing state", name)
		}
		if status == "" {
			t.Fatalf("lane %q missing status", name)
		}
	}
	for _, name := range []string{"freshness", "extract_drop", "promotion_block"} {
		if !seen[name] {
			t.Fatalf("lanes missing %q: %#v", name, lanes)
		}
	}
	if surface["mode"] != "freshness_extract_drop_promotion_block_visibility" {
		t.Fatalf("mode=%v, want freshness_extract_drop_promotion_block_visibility", surface["mode"])
	}
}

// TestSeq17P327Step14AdoptionGate validates the Step 14 adoption gate surface
// for SEQ-17-P327: 17-4a Step 14 adoption gate define.
func TestSeq17P327Step14AdoptionGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p327","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	gate := seq165Map(t, resp, "step_14_adoption_gate")
	if gate["version"] != "seq17_p327.v1" {
		t.Fatalf("version=%v, want seq17_p327.v1", gate["version"])
	}
	if gate["role"] != "step_14_adoption_gate" {
		t.Fatalf("role=%v, want step_14_adoption_gate", gate["role"])
	}
	if gate["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", gate["truth_authority"])
	}
	if gate["definition_state"] != "ready" {
		t.Fatalf("definition_state=%v, want ready", gate["definition_state"])
	}
	if gate["execution_state"] != "pending_operator_review" {
		t.Fatalf("execution_state=%v, want pending_operator_review", gate["execution_state"])
	}
	if gate["adoption_blocked"] != true {
		t.Fatalf("adoption_blocked=%v, want true", gate["adoption_blocked"])
	}
	lanes, ok := gate["regression_evidence_lane"].([]any)
	if !ok || len(lanes) == 0 {
		t.Fatalf("regression_evidence_lane missing or empty")
	}
	if gate["mode"] != "step_14_adoption_gate_definition_execution_split" {
		t.Fatalf("mode=%v, want step_14_adoption_gate_definition_execution_split", gate["mode"])
	}
}

// TestSeq17P328Step15AdoptionGate validates the Step 15 adoption gate surface
// for SEQ-17-P328: 17-4b Step 15 adoption gate define.
func TestSeq17P328Step15AdoptionGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p328","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	gate := seq165Map(t, resp, "step_15_adoption_gate")
	if gate["version"] != "seq17_p328.v1" {
		t.Fatalf("version=%v, want seq17_p328.v1", gate["version"])
	}
	if gate["role"] != "step_15_adoption_gate" {
		t.Fatalf("role=%v, want step_15_adoption_gate", gate["role"])
	}
	if gate["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", gate["truth_authority"])
	}
	if gate["definition_state"] != "ready" {
		t.Fatalf("definition_state=%v, want ready", gate["definition_state"])
	}
	if gate["execution_state"] != "pending_operator_review" {
		t.Fatalf("execution_state=%v, want pending_operator_review", gate["execution_state"])
	}
	if gate["adoption_blocked"] != true {
		t.Fatalf("adoption_blocked=%v, want true", gate["adoption_blocked"])
	}
	lanes, ok := gate["regression_evidence_lane"].([]any)
	if !ok || len(lanes) == 0 {
		t.Fatalf("regression_evidence_lane missing or empty")
	}
	if gate["mode"] != "step_15_adoption_gate_definition_execution_split" {
		t.Fatalf("mode=%v, want step_15_adoption_gate_definition_execution_split", gate["mode"])
	}
}

// TestSeq17P329Step16AdoptionGate validates the Step 16 adoption gate surface
// for SEQ-17-P329: 17-4c Step 16 adoption gate define.
func TestSeq17P329Step16AdoptionGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p329","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	gate := seq165Map(t, resp, "step_16_adoption_gate")
	if gate["version"] != "seq17_p329.v1" {
		t.Fatalf("version=%v, want seq17_p329.v1", gate["version"])
	}
	if gate["role"] != "step_16_adoption_gate" {
		t.Fatalf("role=%v, want step_16_adoption_gate", gate["role"])
	}
	if gate["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", gate["truth_authority"])
	}
	if gate["definition_state"] != "ready" {
		t.Fatalf("definition_state=%v, want ready", gate["definition_state"])
	}
	if gate["execution_state"] != "pending_operator_review" {
		t.Fatalf("execution_state=%v, want pending_operator_review", gate["execution_state"])
	}
	if gate["adoption_blocked"] != true {
		t.Fatalf("adoption_blocked=%v, want true", gate["adoption_blocked"])
	}
	lanes, ok := gate["regression_evidence_lane"].([]any)
	if !ok || len(lanes) == 0 {
		t.Fatalf("regression_evidence_lane missing or empty")
	}
	if gate["mode"] != "step_16_adoption_gate_definition_execution_split" {
		t.Fatalf("mode=%v, want step_16_adoption_gate_definition_execution_split", gate["mode"])
	}
}

// TestSeq17P330BundleRegenerateChecklist validates the root -> bundle regenerate
// checklist surface for SEQ-17-P330: 17-4d root -> bundle regenerate checklist define.
func TestSeq17P330BundleRegenerateChecklist(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p330","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	checklist := seq165Map(t, resp, "bundle_regenerate_checklist")
	if checklist["version"] != "seq17_p330.v1" {
		t.Fatalf("version=%v, want seq17_p330.v1", checklist["version"])
	}
	if checklist["role"] != "bundle_regenerate_checklist" {
		t.Fatalf("role=%v, want bundle_regenerate_checklist", checklist["role"])
	}
	if checklist["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", checklist["truth_authority"])
	}
	if checklist["regenerate_blocked"] != true {
		t.Fatalf("regenerate_blocked=%v, want true", checklist["regenerate_blocked"])
	}
	items, ok := checklist["checklist"].([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("checklist missing or empty")
	}
	if checklist["mode"] != "bundle_regenerate_checklist_definition_only" {
		t.Fatalf("mode=%v, want bundle_regenerate_checklist_definition_only", checklist["mode"])
	}
}

// TestSeq17P331PackagedBundleChecklist validates the packaged bundle regression /
// smoke / release note checklist surface for SEQ-17-P331: 17-4e packaged bundle
// regression / smoke / release note checklist define.
func TestSeq17P331PackagedBundleChecklist(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p331","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	checklist := seq165Map(t, resp, "packaged_bundle_checklist")
	if checklist["version"] != "seq17_p331.v1" {
		t.Fatalf("version=%v, want seq17_p331.v1", checklist["version"])
	}
	if checklist["role"] != "packaged_bundle_checklist" {
		t.Fatalf("role=%v, want packaged_bundle_checklist", checklist["role"])
	}
	if checklist["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", checklist["truth_authority"])
	}
	if checklist["release_blocked"] != true {
		t.Fatalf("release_blocked=%v, want true", checklist["release_blocked"])
	}
	items, ok := checklist["checklist"].([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("checklist missing or empty")
	}
	if checklist["mode"] != "packaged_bundle_regression_smoke_release_note_checklist" {
		t.Fatalf("mode=%v, want packaged_bundle_regression_smoke_release_note_checklist", checklist["mode"])
	}
}

// TestSeq17P332FreshnessSilentDropGate validates the freshness / silent-drop gate
// surface for SEQ-17-P332: 17-4f freshness / silent-drop gate define.
func TestSeq17P332FreshnessSilentDropGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p332","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	gate := seq165Map(t, resp, "freshness_silent_drop_gate")
	if gate["version"] != "seq17_p332.v1" {
		t.Fatalf("version=%v, want seq17_p332.v1", gate["version"])
	}
	if gate["role"] != "freshness_silent_drop_gate" {
		t.Fatalf("role=%v, want freshness_silent_drop_gate", gate["role"])
	}
	if gate["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", gate["truth_authority"])
	}
	if gate["definition_state"] != "ready" {
		t.Fatalf("definition_state=%v, want ready", gate["definition_state"])
	}
	if gate["execution_state"] != "monitoring" {
		t.Fatalf("execution_state=%v, want monitoring", gate["execution_state"])
	}
	if gate["gate_blocked"] != false {
		t.Fatalf("gate_blocked=%v, want false", gate["gate_blocked"])
	}
	items, ok := gate["gate_items"].([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("gate_items missing or empty")
	}
	seen := map[string]bool{}
	blockingSeen := map[string]bool{}
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("gate_item is %T, want map", raw)
		}
		name, _ := item["name"].(string)
		seen[name] = true
		if item["blocks_step_18_default"] == true {
			blockingSeen[name] = true
		}
	}
	for _, name := range []string{"extraction_lag", "save_delay", "silent_drop", "promotion_visibility_lag"} {
		if !seen[name] {
			t.Fatalf("gate_items missing %q: %#v", name, items)
		}
	}
	for _, name := range []string{"extraction_lag", "save_delay", "silent_drop"} {
		if !blockingSeen[name] {
			t.Fatalf("gate item %q must block Step 18 default extension when threshold is exceeded: %#v", name, items)
		}
	}
	if gate["mode"] != "freshness_silent_drop_gate_monitoring" {
		t.Fatalf("mode=%v, want freshness_silent_drop_gate_monitoring", gate["mode"])
	}
}

// TestSeq168P170CarryInEvaluationHarness validates that the Step 16.8 replay
// corpus baseline is present and consumable by Step 17 evaluation harness
// without defining a new 17-1f surface.
// SEQ-16.8-P170: Step 17 evaluation harness consumes Step 16.8 replay corpus baseline.
func TestSeq168P170CarryInEvaluationHarness(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p170","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	baseline := seq165Map(t, resp, "replay_corpus_baseline")
	if baseline["version"] != "seq16_8_p135.v1" {
		t.Fatalf("version=%v, want seq16_8_p135.v1", baseline["version"])
	}
	if baseline["role"] != "replay_corpus_baseline" {
		t.Fatalf("role=%v, want replay_corpus_baseline", baseline["role"])
	}
	if baseline["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", baseline["truth_authority"])
	}
	if baseline["carry_in_baseline_for_step_17_evaluation"] != true {
		t.Fatalf("carry_in_baseline_for_step_17_evaluation=%v, want true", baseline["carry_in_baseline_for_step_17_evaluation"])
	}
	if baseline["baseline_source"] != "seq16_8_replay_corpus" {
		t.Fatalf("baseline_source=%v, want seq16_8_replay_corpus", baseline["baseline_source"])
	}
	if baseline["redefines_step_17_1f"] != false {
		t.Fatalf("redefines_step_17_1f=%v, want false", baseline["redefines_step_17_1f"])
	}
	if baseline["mode"] != "replay_corpus_inspection_baseline_add" {
		t.Fatalf("mode=%v, want replay_corpus_inspection_baseline_add", baseline["mode"])
	}
}

// TestSeq168P171CarryInInspectionSurface validates that the Step 16.8 reason
// trace is present and consumable by Step 17 inspection surface without
// defining a new 17-3f surface.
// SEQ-16.8-P171: Step 17 inspection surface uses Step 16.8 reason visibility lane baseline.
func TestSeq168P171CarryInInspectionSurface(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p171","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	trace := seq165Map(t, resp, "reason_trace")
	if trace["version"] != "seq16_8_p101.v1" {
		t.Fatalf("version=%v, want seq16_8_p101.v1", trace["version"])
	}
	if trace["role"] != "reason_trace" {
		t.Fatalf("role=%v, want reason_trace", trace["role"])
	}
	if trace["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", trace["truth_authority"])
	}
	if trace["carry_in_baseline_for_step_17_inspection"] != true {
		t.Fatalf("carry_in_baseline_for_step_17_inspection=%v, want true", trace["carry_in_baseline_for_step_17_inspection"])
	}
	if trace["baseline_source"] != "seq16_8_reason_visibility_lane" {
		t.Fatalf("baseline_source=%v, want seq16_8_reason_visibility_lane", trace["baseline_source"])
	}
	if trace["redefines_step_17_3f"] != false {
		t.Fatalf("redefines_step_17_3f=%v, want false", trace["redefines_step_17_3f"])
	}
	if trace["mode"] != "old_arc_keep_drop_suppress_inspectable" {
		t.Fatalf("mode=%v, want old_arc_keep_drop_suppress_inspectable", trace["mode"])
	}
}

// TestSeq168P172CarryInAdoptionGate validates that the Step 16.8 narrative
// diversity gate is present and consumable by Step 17 adoption gate without
// defining a new 17-4g surface.
// SEQ-16.8-P172: Step 17 adoption gate uses Step 16.8 diversity gate baseline.
func TestSeq168P172CarryInAdoptionGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p172","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	gate := seq165Map(t, resp, "narrative_diversity_gate")
	if gate["version"] != "seq16_8_p127.v1" {
		t.Fatalf("version=%v, want seq16_8_p127.v1", gate["version"])
	}
	if gate["role"] != "narrative_diversity_gate" {
		t.Fatalf("role=%v, want narrative_diversity_gate", gate["role"])
	}
	if gate["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", gate["truth_authority"])
	}
	if gate["carry_in_baseline_for_step_17_adoption"] != true {
		t.Fatalf("carry_in_baseline_for_step_17_adoption=%v, want true", gate["carry_in_baseline_for_step_17_adoption"])
	}
	if gate["baseline_source"] != "seq16_8_diversity_gate" {
		t.Fatalf("baseline_source=%v, want seq16_8_diversity_gate", gate["baseline_source"])
	}
	if gate["redefines_step_17_4g"] != false {
		t.Fatalf("redefines_step_17_4g=%v, want false", gate["redefines_step_17_4g"])
	}
	if gate["mode"] != "narrative_diversity_gate" {
		t.Fatalf("mode=%v, want narrative_diversity_gate", gate["mode"])
	}
}

// TestSeq168P176CarryInStep18HybridScoring validates that Step 16.8 stale arc
// ceiling and scene alignment are present as baseline for Step 18 hybrid
// scoring without redefining them.
// SEQ-16.8-P176: Step 18 hybrid scoring stale callback ceiling / current-scene alignment baseline.
func TestSeq168P176CarryInStep18HybridScoring(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p176","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	ceiling := seq165Map(t, resp, "stale_arc_ceiling")
	if ceiling["version"] != "seq16_8_p99.v1" {
		t.Fatalf("stale_arc_ceiling version=%v, want seq16_8_p99.v1", ceiling["version"])
	}
	if ceiling["carry_in_baseline_for_step_18_hybrid_scoring"] != true {
		t.Fatalf("stale_arc_ceiling carry_in_baseline_for_step_18_hybrid_scoring=%v, want true", ceiling["carry_in_baseline_for_step_18_hybrid_scoring"])
	}
	if ceiling["baseline_source"] != "seq16_8_stale_arc_ceiling" {
		t.Fatalf("stale_arc_ceiling baseline_source=%v, want seq16_8_stale_arc_ceiling", ceiling["baseline_source"])
	}
	if ceiling["opens_step_18_hybrid_scoring"] != true {
		t.Fatalf("stale_arc_ceiling opens_step_18_hybrid_scoring=%v, want true", ceiling["opens_step_18_hybrid_scoring"])
	}
	alignment := seq165Map(t, resp, "scene_alignment")
	if alignment["version"] != "seq16_8_p100.v1" {
		t.Fatalf("scene_alignment version=%v, want seq16_8_p100.v1", alignment["version"])
	}
	if alignment["carry_in_baseline_for_step_18_hybrid_scoring"] != true {
		t.Fatalf("scene_alignment carry_in_baseline_for_step_18_hybrid_scoring=%v, want true", alignment["carry_in_baseline_for_step_18_hybrid_scoring"])
	}
	if alignment["baseline_source"] != "seq16_8_current_scene_alignment" {
		t.Fatalf("scene_alignment baseline_source=%v, want seq16_8_current_scene_alignment", alignment["baseline_source"])
	}
	if alignment["opens_step_18_hybrid_scoring"] != true {
		t.Fatalf("scene_alignment opens_step_18_hybrid_scoring=%v, want true", alignment["opens_step_18_hybrid_scoring"])
	}
}

// TestSeq168P177CarryInStep20SelectiveRerank validates that Step 16.8 stale
// callback suppression and foreground hijack taxonomy are present as baseline
// for Step 20 selective rerank without redefining them.
// SEQ-16.8-P177: Step 20 selective rerank stale callback suppression trigger /
// monopoly failure taxonomy baseline.
func TestSeq168P177CarryInStep20SelectiveRerank(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p177","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	suppression := seq165Map(t, resp, "stale_callback_suppression")
	if suppression["version"] != "seq16_8_p109.v1" {
		t.Fatalf("stale_callback_suppression version=%v, want seq16_8_p109.v1", suppression["version"])
	}
	if suppression["carry_in_baseline_for_step_20_selective_rerank"] != true {
		t.Fatalf("stale_callback_suppression carry_in_baseline_for_step_20_selective_rerank=%v, want true", suppression["carry_in_baseline_for_step_20_selective_rerank"])
	}
	if suppression["baseline_source"] != "seq16_8_stale_callback_suppression" {
		t.Fatalf("stale_callback_suppression baseline_source=%v, want seq16_8_stale_callback_suppression", suppression["baseline_source"])
	}
	if suppression["redefines_step_20_selective_rerank"] != false {
		t.Fatalf("stale_callback_suppression redefines_step_20_selective_rerank=%v, want false", suppression["redefines_step_20_selective_rerank"])
	}
	taxonomy := seq165Map(t, resp, "foreground_hijack_taxonomy")
	if taxonomy["version"] != "seq16_8_p119.v1" {
		t.Fatalf("foreground_hijack_taxonomy version=%v, want seq16_8_p119.v1", taxonomy["version"])
	}
	if taxonomy["carry_in_baseline_for_step_20_selective_rerank"] != true {
		t.Fatalf("foreground_hijack_taxonomy carry_in_baseline_for_step_20_selective_rerank=%v, want true", taxonomy["carry_in_baseline_for_step_20_selective_rerank"])
	}
	if taxonomy["baseline_source"] != "seq16_8_monopoly_failure_taxonomy" {
		t.Fatalf("foreground_hijack_taxonomy baseline_source=%v, want seq16_8_monopoly_failure_taxonomy", taxonomy["baseline_source"])
	}
	if taxonomy["redefines_step_20_selective_rerank"] != false {
		t.Fatalf("foreground_hijack_taxonomy redefines_step_20_selective_rerank=%v, want false", taxonomy["redefines_step_20_selective_rerank"])
	}
}

// TestSeq168P178CarryInLaterStepRecallRerank validates that Step 16.8 recall
// gain / monopoly cost split is present as baseline trace for later-step recall
// / rerank gain without redefining it.
// SEQ-16.8-P178: later-step recall / rerank gain foreground monopoly cost baseline trace.
func TestSeq168P178CarryInLaterStepRecallRerank(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p178","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	split := seq165Map(t, resp, "recall_gain_monopoly_split")
	if split["version"] != "seq16_8_p121.v1" {
		t.Fatalf("version=%v, want seq16_8_p121.v1", split["version"])
	}
	if split["role"] != "recall_gain_monopoly_split" {
		t.Fatalf("role=%v, want recall_gain_monopoly_split", split["role"])
	}
	if split["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", split["truth_authority"])
	}
	if split["carry_in_baseline_for_later_step_recall_rerank"] != true {
		t.Fatalf("carry_in_baseline_for_later_step_recall_rerank=%v, want true", split["carry_in_baseline_for_later_step_recall_rerank"])
	}
	if split["baseline_source"] != "seq16_8_recall_gain_monopoly_split" {
		t.Fatalf("baseline_source=%v, want seq16_8_recall_gain_monopoly_split", split["baseline_source"])
	}
	sharedWith, ok := split["shared_with"].([]any)
	if !ok || len(sharedWith) == 0 {
		t.Fatalf("shared_with missing or empty")
	}
	sharedSeen := map[string]bool{}
	for _, raw := range sharedWith {
		if s, ok := raw.(string); ok {
			sharedSeen[s] = true
		}
	}
	for _, name := range []string{"later_step_recall", "later_step_rerank"} {
		if !sharedSeen[name] {
			t.Fatalf("shared_with missing %q: %#v", name, sharedWith)
		}
	}
	if split["mode"] != "recall_gain_monopoly_cost_split_trace_schema" {
		t.Fatalf("mode=%v, want recall_gain_monopoly_cost_split_trace_schema", split["mode"])
	}
}

// TestSeq17P387BundleGenerationEvidence validates the bundle generation
// evidence contract surface for SEQ-17-P387: Archive Center Beta 0.8 bundle
// latest root runtime create/generate. This is read-only evidence, not actual
// bundle generation.
func TestSeq17P387BundleGenerationEvidence(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p387","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	evidence := seq165Map(t, resp, "bundle_generation_evidence")
	if evidence["version"] != "seq17_p387.v1" {
		t.Fatalf("version=%v, want seq17_p387.v1", evidence["version"])
	}
	if evidence["role"] != "bundle_generation_evidence" {
		t.Fatalf("role=%v, want bundle_generation_evidence", evidence["role"])
	}
	if evidence["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", evidence["truth_authority"])
	}
	if evidence["bundle_target"] != "Archive Center Beta 0.8" {
		t.Fatalf("bundle_target=%v, want Archive Center Beta 0.8", evidence["bundle_target"])
	}
	if evidence["evidence_only"] != true {
		t.Fatalf("evidence_only=%v, want true", evidence["evidence_only"])
	}
	if evidence["artifact_created"] != false {
		t.Fatalf("artifact_created=%v, want false", evidence["artifact_created"])
	}
	if evidence["release_artifact_created"] != false {
		t.Fatalf("release_artifact_created=%v, want false", evidence["release_artifact_created"])
	}
	if evidence["beta_reference_mutated"] != false {
		t.Fatalf("beta_reference_mutated=%v, want false", evidence["beta_reference_mutated"])
	}
	if evidence["bundle_generation_mode"] != "evidence_only_no_artifact" {
		t.Fatalf("bundle_generation_mode=%v, want evidence_only_no_artifact", evidence["bundle_generation_mode"])
	}
	if evidence["mode"] != "bundle_generation_evidence_contract" {
		t.Fatalf("mode=%v, want bundle_generation_evidence_contract", evidence["mode"])
	}
}

// TestSeq17P388RegressionCorpusGreen validates the Step 14~16 regression
// corpus green gate surface for SEQ-17-P388.
func TestSeq17P388RegressionCorpusGreen(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p388","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	gate := seq165Map(t, resp, "regression_corpus_green")
	if gate["version"] != "seq17_p388.v1" {
		t.Fatalf("version=%v, want seq17_p388.v1", gate["version"])
	}
	if gate["role"] != "regression_corpus_green" {
		t.Fatalf("role=%v, want regression_corpus_green", gate["role"])
	}
	if gate["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", gate["truth_authority"])
	}
	if gate["step_14_status"] != "green" {
		t.Fatalf("step_14_status=%v, want green", gate["step_14_status"])
	}
	if gate["step_15_status"] != "green" {
		t.Fatalf("step_15_status=%v, want green", gate["step_15_status"])
	}
	if gate["step_16_status"] != "green" {
		t.Fatalf("step_16_status=%v, want green", gate["step_16_status"])
	}
	if gate["all_steps_green"] != true {
		t.Fatalf("all_steps_green=%v, want true", gate["all_steps_green"])
	}
	if gate["regression_corpus_source"] != "step_14_16_regression_corpus_contract" {
		t.Fatalf("regression_corpus_source=%v, want step_14_16_regression_corpus_contract", gate["regression_corpus_source"])
	}
	if gate["evidence_contract_only"] != true {
		t.Fatalf("evidence_contract_only=%v, want true", gate["evidence_contract_only"])
	}
	if gate["operator_execution_claim"] != false {
		t.Fatalf("operator_execution_claim=%v, want false", gate["operator_execution_claim"])
	}
	if gate["mode"] != "regression_corpus_green_gate" {
		t.Fatalf("mode=%v, want regression_corpus_green_gate", gate["mode"])
	}
}

// TestSeq17P389EvaluationSplitSmokeCheck validates the evaluation split
// completeness/answer-quality smoke check pass surface for SEQ-17-P389.
func TestSeq17P389EvaluationSplitSmokeCheck(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p389","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	check := seq165Map(t, resp, "evaluation_split_smoke_check")
	if check["version"] != "seq17_p389.v1" {
		t.Fatalf("version=%v, want seq17_p389.v1", check["version"])
	}
	if check["role"] != "evaluation_split_smoke_check" {
		t.Fatalf("role=%v, want evaluation_split_smoke_check", check["role"])
	}
	if check["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", check["truth_authority"])
	}
	if check["metric_split"] != "retrieval_completeness_vs_final_answer_quality" {
		t.Fatalf("metric_split=%v, want retrieval_completeness_vs_final_answer_quality", check["metric_split"])
	}
	if check["completeness_check"] != "pass" {
		t.Fatalf("completeness_check=%v, want pass", check["completeness_check"])
	}
	if check["answer_quality_check"] != "pass" {
		t.Fatalf("answer_quality_check=%v, want pass", check["answer_quality_check"])
	}
	if check["smoke_check_pass"] != true {
		t.Fatalf("smoke_check_pass=%v, want true", check["smoke_check_pass"])
	}
	if check["source_metric"] != "lc1p_evaluation_split" {
		t.Fatalf("source_metric=%v, want lc1p_evaluation_split", check["source_metric"])
	}
	if check["evidence_contract_only"] != true {
		t.Fatalf("evidence_contract_only=%v, want true", check["evidence_contract_only"])
	}
	if check["operator_execution_claim"] != false {
		t.Fatalf("operator_execution_claim=%v, want false", check["operator_execution_claim"])
	}
	if check["mode"] != "evaluation_split_smoke_check_pass" {
		t.Fatalf("mode=%v, want evaluation_split_smoke_check_pass", check["mode"])
	}
}

// TestSeq17P390OpsDryRunChecklistPass validates the ops procedure dry-run
// checklist pass surface for SEQ-17-P390.
func TestSeq17P390OpsDryRunChecklistPass(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p390","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	checklist := seq165Map(t, resp, "ops_dry_run_checklist_pass")
	if checklist["version"] != "seq17_p390.v1" {
		t.Fatalf("version=%v, want seq17_p390.v1", checklist["version"])
	}
	if checklist["role"] != "ops_dry_run_checklist_pass" {
		t.Fatalf("role=%v, want ops_dry_run_checklist_pass", checklist["role"])
	}
	if checklist["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", checklist["truth_authority"])
	}
	if checklist["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", checklist["dry_run_only"])
	}
	if checklist["actual_ops_run"] != false {
		t.Fatalf("actual_ops_run=%v, want false", checklist["actual_ops_run"])
	}
	if checklist["all_pass"] != true {
		t.Fatalf("all_pass=%v, want true", checklist["all_pass"])
	}
	items, ok := checklist["dry_run_checklist"].([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("dry_run_checklist missing or empty")
	}
	if checklist["mode"] != "ops_procedure_dry_run_checklist_pass" {
		t.Fatalf("mode=%v, want ops_procedure_dry_run_checklist_pass", checklist["mode"])
	}
}

// TestSeq17P391InspectionLaneBoundaryReview validates the inspection surface
// lane-boundary review checklist pass surface for SEQ-17-P391.
func TestSeq17P391InspectionLaneBoundaryReview(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p391","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	review := seq165Map(t, resp, "inspection_lane_boundary_review")
	if review["version"] != "seq17_p391.v1" {
		t.Fatalf("version=%v, want seq17_p391.v1", review["version"])
	}
	if review["role"] != "inspection_lane_boundary_review" {
		t.Fatalf("role=%v, want inspection_lane_boundary_review", review["role"])
	}
	if review["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", review["truth_authority"])
	}
	if review["read_only_inspection_surface"] != true {
		t.Fatalf("read_only_inspection_surface=%v, want true", review["read_only_inspection_surface"])
	}
	if review["authority_display_guard"] != true {
		t.Fatalf("authority_display_guard=%v, want true", review["authority_display_guard"])
	}
	if review["explain_surface"] != "pass" {
		t.Fatalf("explain_surface=%v, want pass", review["explain_surface"])
	}
	if review["preview_audit_surface"] != "pass" {
		t.Fatalf("preview_audit_surface=%v, want pass", review["preview_audit_surface"])
	}
	if review["dashboard_lane"] != "pass" {
		t.Fatalf("dashboard_lane=%v, want pass", review["dashboard_lane"])
	}
	if review["display_guard"] != "pass" {
		t.Fatalf("display_guard=%v, want pass", review["display_guard"])
	}
	if review["visibility_lane"] != "pass" {
		t.Fatalf("visibility_lane=%v, want pass", review["visibility_lane"])
	}
	if review["all_pass"] != true {
		t.Fatalf("all_pass=%v, want true", review["all_pass"])
	}
	if review["mode"] != "inspection_surface_lane_boundary_review_pass" {
		t.Fatalf("mode=%v, want inspection_surface_lane_boundary_review_pass", review["mode"])
	}
}

// TestSeq17P392ReleaseGateComplete validates the adoption gate / release note /
// bundle checklist complete surface for SEQ-17-P392.
func TestSeq17P392ReleaseGateComplete(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p392","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	gate := seq165Map(t, resp, "release_gate_complete")
	if gate["version"] != "seq17_p392.v1" {
		t.Fatalf("version=%v, want seq17_p392.v1", gate["version"])
	}
	if gate["role"] != "release_gate_complete" {
		t.Fatalf("role=%v, want release_gate_complete", gate["role"])
	}
	if gate["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", gate["truth_authority"])
	}
	if gate["sync_scope"] != "evidence_contract_only" {
		t.Fatalf("sync_scope=%v, want evidence_contract_only", gate["sync_scope"])
	}
	if gate["release_execution"] != false {
		t.Fatalf("release_execution=%v, want false", gate["release_execution"])
	}
	if gate["artifact_created"] != false {
		t.Fatalf("artifact_created=%v, want false", gate["artifact_created"])
	}
	if gate["adoption_default_changed"] != false {
		t.Fatalf("adoption_default_changed=%v, want false", gate["adoption_default_changed"])
	}
	if gate["adoption_gate_sync"] != "complete" {
		t.Fatalf("adoption_gate_sync=%v, want complete", gate["adoption_gate_sync"])
	}
	if gate["release_note_sync"] != "complete" {
		t.Fatalf("release_note_sync=%v, want complete", gate["release_note_sync"])
	}
	if gate["bundle_checklist_sync"] != "complete" {
		t.Fatalf("bundle_checklist_sync=%v, want complete", gate["bundle_checklist_sync"])
	}
	if gate["all_complete"] != true {
		t.Fatalf("all_complete=%v, want true", gate["all_complete"])
	}
	if gate["mode"] != "adoption_gate_release_note_bundle_checklist_complete" {
		t.Fatalf("mode=%v, want adoption_gate_release_note_bundle_checklist_complete", gate["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-17 re-audit closure tests (P396 ~ P402)
// ---------------------------------------------------------------------------

// TestSeq17P396ReauditBackendAdminOwner validates the backend/admin release-gate
// owner closure surface for SEQ-17-P396.
func TestSeq17P396ReauditBackendAdminOwner(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p396","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	owner := seq165Map(t, resp, "reaudit_backend_admin_owner")
	if owner["version"] != "seq17_p396.v1" {
		t.Fatalf("version=%v, want seq17_p396.v1", owner["version"])
	}
	if owner["role"] != "reaudit_backend_admin_owner" {
		t.Fatalf("role=%v, want reaudit_backend_admin_owner", owner["role"])
	}
	if owner["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", owner["truth_authority"])
	}
	if owner["owner_closed"] != true {
		t.Fatalf("owner_closed=%v, want true", owner["owner_closed"])
	}
	if owner["mode"] != "reaudit_backend_admin_owner_closed" {
		t.Fatalf("mode=%v, want reaudit_backend_admin_owner_closed", owner["mode"])
	}
}

// TestSeq17P397ReauditOpsDocDryRun validates the ops documentation dry-run
// checklist closure surface for SEQ-17-P397.
func TestSeq17P397ReauditOpsDocDryRun(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p397","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	doc := seq165Map(t, resp, "reaudit_ops_doc_dry_run")
	if doc["version"] != "seq17_p397.v1" {
		t.Fatalf("version=%v, want seq17_p397.v1", doc["version"])
	}
	if doc["role"] != "reaudit_ops_doc_dry_run" {
		t.Fatalf("role=%v, want reaudit_ops_doc_dry_run", doc["role"])
	}
	if doc["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", doc["truth_authority"])
	}
	if doc["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", doc["dry_run_only"])
	}
	if doc["actual_ops_run"] != false {
		t.Fatalf("actual_ops_run=%v, want false", doc["actual_ops_run"])
	}
	if doc["owner_closed"] != true {
		t.Fatalf("owner_closed=%v, want true", doc["owner_closed"])
	}
	if doc["mode"] != "reaudit_ops_doc_dry_run_closed" {
		t.Fatalf("mode=%v, want reaudit_ops_doc_dry_run_closed", doc["mode"])
	}
}

// TestSeq17P398ReauditRootRuntimeReadOnly validates the root runtime read-only
// inspection/gate surface closure for SEQ-17-P398.
func TestSeq17P398ReauditRootRuntimeReadOnly(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p398","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	rt := seq165Map(t, resp, "reaudit_root_runtime_read_only")
	if rt["version"] != "seq17_p398.v1" {
		t.Fatalf("version=%v, want seq17_p398.v1", rt["version"])
	}
	if rt["role"] != "reaudit_root_runtime_read_only" {
		t.Fatalf("role=%v, want reaudit_root_runtime_read_only", rt["role"])
	}
	if rt["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", rt["truth_authority"])
	}
	if rt["read_only_surface"] != true {
		t.Fatalf("read_only_surface=%v, want true", rt["read_only_surface"])
	}
	if rt["owner_closed"] != true {
		t.Fatalf("owner_closed=%v, want true", rt["owner_closed"])
	}
	if rt["mode"] != "reaudit_root_runtime_read_only_closed" {
		t.Fatalf("mode=%v, want reaudit_root_runtime_read_only_closed", rt["mode"])
	}
}

// TestSeq17P399ReauditReleaseGateOperatorEvidence validates the release gate
// operator evidence closure surface for SEQ-17-P399.
func TestSeq17P399ReauditReleaseGateOperatorEvidence(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p399","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	ev := seq165Map(t, resp, "reaudit_release_gate_operator_evidence")
	if ev["version"] != "seq17_p399.v1" {
		t.Fatalf("version=%v, want seq17_p399.v1", ev["version"])
	}
	if ev["role"] != "reaudit_release_gate_operator_evidence" {
		t.Fatalf("role=%v, want reaudit_release_gate_operator_evidence", ev["role"])
	}
	if ev["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", ev["truth_authority"])
	}
	if ev["operator_evidence"] != true {
		t.Fatalf("operator_evidence=%v, want true", ev["operator_evidence"])
	}
	if ev["operator_evidence_mode"] != "contract_included_not_supplied" {
		t.Fatalf("operator_evidence_mode=%v, want contract_included_not_supplied", ev["operator_evidence_mode"])
	}
	if ev["bundle_regenerate_sync"] != "complete" {
		t.Fatalf("bundle_regenerate_sync=%v, want complete", ev["bundle_regenerate_sync"])
	}
	if ev["release_note_sync"] != "complete" {
		t.Fatalf("release_note_sync=%v, want complete", ev["release_note_sync"])
	}
	if ev["known_risk_ledger_sync"] != "complete" {
		t.Fatalf("known_risk_ledger_sync=%v, want complete", ev["known_risk_ledger_sync"])
	}
	if ev["artifact_created"] != false {
		t.Fatalf("artifact_created=%v, want false", ev["artifact_created"])
	}
	if ev["release_execution"] != false {
		t.Fatalf("release_execution=%v, want false", ev["release_execution"])
	}
	if ev["all_closed"] != true {
		t.Fatalf("all_closed=%v, want true", ev["all_closed"])
	}
	if ev["mode"] != "reaudit_release_gate_operator_evidence_closed" {
		t.Fatalf("mode=%v, want reaudit_release_gate_operator_evidence_closed", ev["mode"])
	}
}

// TestSeq17P400ReauditAdminMutationControlUI validates the admin mutation/control
// UI boundary surface for SEQ-17-P400. Must be operator_required,
// execution_disabled, read_only, artifact_created:false, beta_reference_mutated:false.
func TestSeq17P400ReauditAdminMutationControlUI(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p400","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	ui := seq165Map(t, resp, "reaudit_admin_mutation_control_ui")
	if ui["version"] != "seq17_p400.v1" {
		t.Fatalf("version=%v, want seq17_p400.v1", ui["version"])
	}
	if ui["role"] != "reaudit_admin_mutation_control_ui" {
		t.Fatalf("role=%v, want reaudit_admin_mutation_control_ui", ui["role"])
	}
	if ui["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", ui["truth_authority"])
	}
	if ui["operator_required"] != true {
		t.Fatalf("operator_required=%v, want true", ui["operator_required"])
	}
	if ui["execution_disabled"] != true {
		t.Fatalf("execution_disabled=%v, want true", ui["execution_disabled"])
	}
	if ui["read_only"] != true {
		t.Fatalf("read_only=%v, want true", ui["read_only"])
	}
	if ui["artifact_created"] != false {
		t.Fatalf("artifact_created=%v, want false", ui["artifact_created"])
	}
	if ui["beta_reference_mutated"] != false {
		t.Fatalf("beta_reference_mutated=%v, want false", ui["beta_reference_mutated"])
	}
	if ui["ui_exists"] != false {
		t.Fatalf("ui_exists=%v, want false", ui["ui_exists"])
	}
	if ui["mode"] != "reaudit_admin_mutation_control_ui_boundary" {
		t.Fatalf("mode=%v, want reaudit_admin_mutation_control_ui_boundary", ui["mode"])
	}
}

// TestSeq17P401ReauditReleaseExecutionUI validates the release execution UI
// boundary surface for SEQ-17-P401. Must be operator_required,
// execution_disabled, read_only, artifact_created:false, beta_reference_mutated:false.
func TestSeq17P401ReauditReleaseExecutionUI(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p401","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	ui := seq165Map(t, resp, "reaudit_release_execution_ui")
	if ui["version"] != "seq17_p401.v1" {
		t.Fatalf("version=%v, want seq17_p401.v1", ui["version"])
	}
	if ui["role"] != "reaudit_release_execution_ui" {
		t.Fatalf("role=%v, want reaudit_release_execution_ui", ui["role"])
	}
	if ui["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", ui["truth_authority"])
	}
	if ui["operator_required"] != true {
		t.Fatalf("operator_required=%v, want true", ui["operator_required"])
	}
	if ui["execution_disabled"] != true {
		t.Fatalf("execution_disabled=%v, want true", ui["execution_disabled"])
	}
	if ui["read_only"] != true {
		t.Fatalf("read_only=%v, want true", ui["read_only"])
	}
	if ui["artifact_created"] != false {
		t.Fatalf("artifact_created=%v, want false", ui["artifact_created"])
	}
	if ui["beta_reference_mutated"] != false {
		t.Fatalf("beta_reference_mutated=%v, want false", ui["beta_reference_mutated"])
	}
	if ui["ui_exists"] != false {
		t.Fatalf("ui_exists=%v, want false", ui["ui_exists"])
	}
	if ui["mode"] != "reaudit_release_execution_ui_boundary" {
		t.Fatalf("mode=%v, want reaudit_release_execution_ui_boundary", ui["mode"])
	}
}

// TestSeq17P402ReauditBeta08ClosureBundle validates the Beta 0.8 closure bundle
// boundary surface for SEQ-17-P402. Must have bundle_folder_authoritative:false,
// root_source_of_truth:true, artifact_created:false, beta_reference_mutated:false.
func TestSeq17P402ReauditBeta08ClosureBundle(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p402","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	b := seq165Map(t, resp, "reaudit_beta_0_8_closure_bundle")
	if b["version"] != "seq17_p402.v1" {
		t.Fatalf("version=%v, want seq17_p402.v1", b["version"])
	}
	if b["role"] != "reaudit_beta_0_8_closure_bundle" {
		t.Fatalf("role=%v, want reaudit_beta_0_8_closure_bundle", b["role"])
	}
	if b["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", b["truth_authority"])
	}
	if b["bundle_folder_authoritative"] != false {
		t.Fatalf("bundle_folder_authoritative=%v, want false", b["bundle_folder_authoritative"])
	}
	if b["root_source_of_truth"] != true {
		t.Fatalf("root_source_of_truth=%v, want true", b["root_source_of_truth"])
	}
	if b["artifact_created"] != false {
		t.Fatalf("artifact_created=%v, want false", b["artifact_created"])
	}
	if b["beta_reference_mutated"] != false {
		t.Fatalf("beta_reference_mutated=%v, want false", b["beta_reference_mutated"])
	}
	if b["mode"] != "reaudit_beta_0_8_closure_bundle_boundary" {
		t.Fatalf("mode=%v, want reaudit_beta_0_8_closure_bundle_boundary", b["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-17 Beta 0.8 decision tests (P412 ~ P416)
// ---------------------------------------------------------------------------

// TestSeq17P412DecisionCompletenessMetricUnit validates the completeness metric
// default unit decision surface for SEQ-17-P412.
func TestSeq17P412DecisionCompletenessMetricUnit(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p412","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	d := seq165Map(t, resp, "decision_completeness_metric_unit")
	if d["version"] != "seq17_p412.v1" {
		t.Fatalf("version=%v, want seq17_p412.v1", d["version"])
	}
	if d["role"] != "decision_completeness_metric_unit" {
		t.Fatalf("role=%v, want decision_completeness_metric_unit", d["role"])
	}
	if d["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", d["truth_authority"])
	}
	if d["decision_state"] != "pending" {
		t.Fatalf("decision_state=%v, want pending", d["decision_state"])
	}
	if d["default_unit"] != "retrieval_slice" {
		t.Fatalf("default_unit=%v, want retrieval_slice", d["default_unit"])
	}
	if d["mode"] != "decision_completeness_metric_unit_pending" {
		t.Fatalf("mode=%v, want decision_completeness_metric_unit_pending", d["mode"])
	}
}

// TestSeq17P413DecisionRegressionCorpusMix validates the regression corpus mix
// decision surface for SEQ-17-P413.
func TestSeq17P413DecisionRegressionCorpusMix(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p413","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	d := seq165Map(t, resp, "decision_regression_corpus_mix")
	if d["version"] != "seq17_p413.v1" {
		t.Fatalf("version=%v, want seq17_p413.v1", d["version"])
	}
	if d["role"] != "decision_regression_corpus_mix" {
		t.Fatalf("role=%v, want decision_regression_corpus_mix", d["role"])
	}
	if d["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", d["truth_authority"])
	}
	if d["decision_state"] != "fixed" {
		t.Fatalf("decision_state=%v, want fixed", d["decision_state"])
	}
	if d["chosen_mix"] != "mixed_replay_and_runtime_contract" {
		t.Fatalf("chosen_mix=%v, want mixed_replay_and_runtime_contract", d["chosen_mix"])
	}
	if d["synthetic_only"] != false {
		t.Fatalf("synthetic_only=%v, want false", d["synthetic_only"])
	}
	if d["actual_replay_only"] != false {
		t.Fatalf("actual_replay_only=%v, want false", d["actual_replay_only"])
	}
	if d["mode"] != "decision_regression_corpus_mixed_fixed" {
		t.Fatalf("mode=%v, want decision_regression_corpus_mixed_fixed", d["mode"])
	}
}

// TestSeq17P414DecisionInspectionLaneDefault validates the inspection lane
// default decision surface for SEQ-17-P414.
func TestSeq17P414DecisionInspectionLaneDefault(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p414","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	d := seq165Map(t, resp, "decision_inspection_lane_default")
	if d["version"] != "seq17_p414.v1" {
		t.Fatalf("version=%v, want seq17_p414.v1", d["version"])
	}
	if d["role"] != "decision_inspection_lane_default" {
		t.Fatalf("role=%v, want decision_inspection_lane_default", d["role"])
	}
	if d["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", d["truth_authority"])
	}
	if d["decision_state"] != "fixed" {
		t.Fatalf("decision_state=%v, want fixed", d["decision_state"])
	}
	if d["panel_location"] != "root_runtime_debug_panel" {
		t.Fatalf("panel_location=%v, want root_runtime_debug_panel", d["panel_location"])
	}
	if d["mode"] != "decision_inspection_lane_default_fixed" {
		t.Fatalf("mode=%v, want decision_inspection_lane_default_fixed", d["mode"])
	}
}

// TestSeq17P415DecisionAdoptionGateReviewMode validates the adoption gate review
// mode decision surface for SEQ-17-P415.
func TestSeq17P415DecisionAdoptionGateReviewMode(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p415","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	d := seq165Map(t, resp, "decision_adoption_gate_review_mode")
	if d["version"] != "seq17_p415.v1" {
		t.Fatalf("version=%v, want seq17_p415.v1", d["version"])
	}
	if d["role"] != "decision_adoption_gate_review_mode" {
		t.Fatalf("role=%v, want decision_adoption_gate_review_mode", d["role"])
	}
	if d["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", d["truth_authority"])
	}
	if d["decision_state"] != "fixed" {
		t.Fatalf("decision_state=%v, want fixed", d["decision_state"])
	}
	if d["review_mode"] != "slice_manual_review_plus_automatic_gate" {
		t.Fatalf("review_mode=%v, want slice_manual_review_plus_automatic_gate", d["review_mode"])
	}
	if d["backend_gate_payload"] != true {
		t.Fatalf("backend_gate_payload=%v, want true", d["backend_gate_payload"])
	}
	if d["root_runtime_read_only_panel"] != true {
		t.Fatalf("root_runtime_read_only_panel=%v, want true", d["root_runtime_read_only_panel"])
	}
	if d["mode"] != "decision_adoption_gate_review_mode_fixed" {
		t.Fatalf("mode=%v, want decision_adoption_gate_review_mode_fixed", d["mode"])
	}
}

// TestSeq17P416DecisionBundleRegenerateSplit validates the bundle regenerate
// split decision surface for SEQ-17-P416.
func TestSeq17P416DecisionBundleRegenerateSplit(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p416","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	d := seq165Map(t, resp, "decision_bundle_regenerate_split")
	if d["version"] != "seq17_p416.v1" {
		t.Fatalf("version=%v, want seq17_p416.v1", d["version"])
	}
	if d["role"] != "decision_bundle_regenerate_split" {
		t.Fatalf("role=%v, want decision_bundle_regenerate_split", d["role"])
	}
	if d["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", d["truth_authority"])
	}
	if d["decision_state"] != "fixed" {
		t.Fatalf("decision_state=%v, want fixed", d["decision_state"])
	}
	if d["truth_surface"] != "release_hygiene_checklist" {
		t.Fatalf("truth_surface=%v, want release_hygiene_checklist", d["truth_surface"])
	}
	if d["actual_bundle_refresh"] != "operator_execution_split" {
		t.Fatalf("actual_bundle_refresh=%v, want operator_execution_split", d["actual_bundle_refresh"])
	}
	if d["script_plus_checklist"] != true {
		t.Fatalf("script_plus_checklist=%v, want true", d["script_plus_checklist"])
	}
	if d["mode"] != "decision_bundle_regenerate_split_fixed" {
		t.Fatalf("mode=%v, want decision_bundle_regenerate_split_fixed", d["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-17 Chroma migration dry-run checklist tests (P420 ~ P430)
// ---------------------------------------------------------------------------

// TestSeq17P420ChromaMigrationPreflight validates the 17-C1 migration preflight
// dry-run surface for SEQ-17-P420.
func TestSeq17P420ChromaMigrationPreflight(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p420","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	s := seq165Map(t, resp, "chroma_migration_preflight")
	if s["version"] != "seq17_p420.v1" {
		t.Fatalf("version=%v, want seq17_p420.v1", s["version"])
	}
	if s["role"] != "chroma_migration_preflight" {
		t.Fatalf("role=%v, want chroma_migration_preflight", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", s["dry_run_only"])
	}
	if s["actual_migration_run"] != false {
		t.Fatalf("actual_migration_run=%v, want false", s["actual_migration_run"])
	}
	if s["storage_mutated"] != false {
		t.Fatalf("storage_mutated=%v, want false", s["storage_mutated"])
	}
	if s["mode"] != "chroma_migration_preflight_dry_run" {
		t.Fatalf("mode=%v, want chroma_migration_preflight_dry_run", s["mode"])
	}
}

// TestSeq17P421ChromaShadowBootstrap validates the 17-C2 shadow collection
// bootstrap dry-run surface for SEQ-17-P421.
func TestSeq17P421ChromaShadowBootstrap(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p421","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	s := seq165Map(t, resp, "chroma_shadow_bootstrap")
	if s["version"] != "seq17_p421.v1" {
		t.Fatalf("version=%v, want seq17_p421.v1", s["version"])
	}
	if s["role"] != "chroma_shadow_bootstrap" {
		t.Fatalf("role=%v, want chroma_shadow_bootstrap", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", s["dry_run_only"])
	}
	if s["actual_migration_run"] != false {
		t.Fatalf("actual_migration_run=%v, want false", s["actual_migration_run"])
	}
	if s["collection_created"] != false {
		t.Fatalf("collection_created=%v, want false", s["collection_created"])
	}
	if s["metadata_written"] != false {
		t.Fatalf("metadata_written=%v, want false", s["metadata_written"])
	}
	if s["health_probe_run"] != false {
		t.Fatalf("health_probe_run=%v, want false", s["health_probe_run"])
	}
	if s["mode"] != "chroma_shadow_bootstrap_dry_run" {
		t.Fatalf("mode=%v, want chroma_shadow_bootstrap_dry_run", s["mode"])
	}
}

// TestSeq17P422ChromaBackfillDryRun validates the 17-C3 backfill dry-run
// surface for SEQ-17-P422.
func TestSeq17P422ChromaBackfillDryRun(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p422","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	s := seq165Map(t, resp, "chroma_backfill_dry_run")
	if s["version"] != "seq17_p422.v1" {
		t.Fatalf("version=%v, want seq17_p422.v1", s["version"])
	}
	if s["role"] != "chroma_backfill_dry_run" {
		t.Fatalf("role=%v, want chroma_backfill_dry_run", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", s["dry_run_only"])
	}
	if s["actual_migration_run"] != false {
		t.Fatalf("actual_migration_run=%v, want false", s["actual_migration_run"])
	}
	if s["sample_export_executed"] != false {
		t.Fatalf("sample_export_executed=%v, want false", s["sample_export_executed"])
	}
	if s["chroma_ingest_executed"] != false {
		t.Fatalf("chroma_ingest_executed=%v, want false", s["chroma_ingest_executed"])
	}
	if s["mode"] != "chroma_backfill_dry_run" {
		t.Fatalf("mode=%v, want chroma_backfill_dry_run", s["mode"])
	}
}

// TestSeq17P423ChromaBulkBackfill validates the 17-C4 bulk backfill dry-run
// surface for SEQ-17-P423.
func TestSeq17P423ChromaBulkBackfill(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p423","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	s := seq165Map(t, resp, "chroma_bulk_backfill")
	if s["version"] != "seq17_p423.v1" {
		t.Fatalf("version=%v, want seq17_p423.v1", s["version"])
	}
	if s["role"] != "chroma_bulk_backfill" {
		t.Fatalf("role=%v, want chroma_bulk_backfill", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", s["dry_run_only"])
	}
	if s["actual_migration_run"] != false {
		t.Fatalf("actual_migration_run=%v, want false", s["actual_migration_run"])
	}
	if s["bulk_ingest_executed"] != false {
		t.Fatalf("bulk_ingest_executed=%v, want false", s["bulk_ingest_executed"])
	}
	if s["checkpoint_written"] != false {
		t.Fatalf("checkpoint_written=%v, want false", s["checkpoint_written"])
	}
	if s["mode"] != "chroma_bulk_backfill_dry_run" {
		t.Fatalf("mode=%v, want chroma_bulk_backfill_dry_run", s["mode"])
	}
}

// TestSeq17P424ChromaReembedDiscipline validates the 17-C5 reembed discipline
// dry-run surface for SEQ-17-P424.
func TestSeq17P424ChromaReembedDiscipline(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p424","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	s := seq165Map(t, resp, "chroma_reembed_discipline")
	if s["version"] != "seq17_p424.v1" {
		t.Fatalf("version=%v, want seq17_p424.v1", s["version"])
	}
	if s["role"] != "chroma_reembed_discipline" {
		t.Fatalf("role=%v, want chroma_reembed_discipline", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", s["dry_run_only"])
	}
	if s["actual_migration_run"] != false {
		t.Fatalf("actual_migration_run=%v, want false", s["actual_migration_run"])
	}
	if s["reembed_queue_mutated"] != false {
		t.Fatalf("reembed_queue_mutated=%v, want false", s["reembed_queue_mutated"])
	}
	if s["vectors_invalidated"] != false {
		t.Fatalf("vectors_invalidated=%v, want false", s["vectors_invalidated"])
	}
	if s["mode"] != "chroma_reembed_discipline_dry_run" {
		t.Fatalf("mode=%v, want chroma_reembed_discipline_dry_run", s["mode"])
	}
}

// TestSeq17P425ChromaDivergenceHealthProbe validates the 17-C6 divergence / health
// probe dry-run surface for SEQ-17-P425.
func TestSeq17P425ChromaDivergenceHealthProbe(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p425","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	s := seq165Map(t, resp, "chroma_divergence_health_probe")
	if s["version"] != "seq17_p425.v1" {
		t.Fatalf("version=%v, want seq17_p425.v1", s["version"])
	}
	if s["role"] != "chroma_divergence_health_probe" {
		t.Fatalf("role=%v, want chroma_divergence_health_probe", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", s["dry_run_only"])
	}
	if s["actual_migration_run"] != false {
		t.Fatalf("actual_migration_run=%v, want false", s["actual_migration_run"])
	}
	if s["health_probe_run"] != false {
		t.Fatalf("health_probe_run=%v, want false", s["health_probe_run"])
	}
	if s["fallback_triggered"] != false {
		t.Fatalf("fallback_triggered=%v, want false", s["fallback_triggered"])
	}
	if s["cache_invalidation_rule"] != "stateless_per_request" {
		t.Fatalf("cache_invalidation_rule=%v, want stateless_per_request", s["cache_invalidation_rule"])
	}
	if s["mode"] != "chroma_divergence_health_probe_dry_run" {
		t.Fatalf("mode=%v, want chroma_divergence_health_probe_dry_run", s["mode"])
	}
}

// TestSeq17P426ChromaDegradedFallbackRunbook validates the 17-C7 degraded
// fallback runbook dry-run surface for SEQ-17-P426.
func TestSeq17P426ChromaDegradedFallbackRunbook(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p426","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	s := seq165Map(t, resp, "chroma_degraded_fallback_runbook")
	if s["version"] != "seq17_p426.v1" {
		t.Fatalf("version=%v, want seq17_p426.v1", s["version"])
	}
	if s["role"] != "chroma_degraded_fallback_runbook" {
		t.Fatalf("role=%v, want chroma_degraded_fallback_runbook", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", s["dry_run_only"])
	}
	if s["actual_migration_run"] != false {
		t.Fatalf("actual_migration_run=%v, want false", s["actual_migration_run"])
	}
	if s["write_freeze_applied"] != false {
		t.Fatalf("write_freeze_applied=%v, want false", s["write_freeze_applied"])
	}
	if s["cleanup_executed"] != false {
		t.Fatalf("cleanup_executed=%v, want false", s["cleanup_executed"])
	}
	if s["mode"] != "chroma_degraded_fallback_runbook_dry_run" {
		t.Fatalf("mode=%v, want chroma_degraded_fallback_runbook_dry_run", s["mode"])
	}
}

// TestSeq17P427ChromaRebuildRollbackDrill validates the 17-C8 rebuild / rollback
// drill dry-run surface for SEQ-17-P427.
func TestSeq17P427ChromaRebuildRollbackDrill(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p427","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	s := seq165Map(t, resp, "chroma_rebuild_rollback_drill")
	if s["version"] != "seq17_p427.v1" {
		t.Fatalf("version=%v, want seq17_p427.v1", s["version"])
	}
	if s["role"] != "chroma_rebuild_rollback_drill" {
		t.Fatalf("role=%v, want chroma_rebuild_rollback_drill", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", s["dry_run_only"])
	}
	if s["actual_migration_run"] != false {
		t.Fatalf("actual_migration_run=%v, want false", s["actual_migration_run"])
	}
	if s["collection_wiped"] != false {
		t.Fatalf("collection_wiped=%v, want false", s["collection_wiped"])
	}
	if s["rebuild_executed"] != false {
		t.Fatalf("rebuild_executed=%v, want false", s["rebuild_executed"])
	}
	if s["rollback_executed"] != false {
		t.Fatalf("rollback_executed=%v, want false", s["rollback_executed"])
	}
	if s["mode"] != "chroma_rebuild_rollback_drill_dry_run" {
		t.Fatalf("mode=%v, want chroma_rebuild_rollback_drill_dry_run", s["mode"])
	}
}

// TestSeq17P428ChromaAdoptionGate validates the 17-C9 adoption gate dry-run
// surface for SEQ-17-P428.
func TestSeq17P428ChromaAdoptionGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p428","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	s := seq165Map(t, resp, "chroma_adoption_gate")
	if s["version"] != "seq17_p428.v1" {
		t.Fatalf("version=%v, want seq17_p428.v1", s["version"])
	}
	if s["role"] != "chroma_adoption_gate" {
		t.Fatalf("role=%v, want chroma_adoption_gate", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", s["dry_run_only"])
	}
	if s["actual_migration_run"] != false {
		t.Fatalf("actual_migration_run=%v, want false", s["actual_migration_run"])
	}
	if s["limited_cutover_enabled"] != false {
		t.Fatalf("limited_cutover_enabled=%v, want false", s["limited_cutover_enabled"])
	}
	if s["operator_approval_required"] != true {
		t.Fatalf("operator_approval_required=%v, want true", s["operator_approval_required"])
	}
	if s["mode"] != "chroma_adoption_gate_dry_run" {
		t.Fatalf("mode=%v, want chroma_adoption_gate_dry_run", s["mode"])
	}
}

// TestSeq17P429ChromaReleaseHygiene validates the 17-C10 release hygiene dry-run
// surface for SEQ-17-P429.
func TestSeq17P429ChromaReleaseHygiene(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p429","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	s := seq165Map(t, resp, "chroma_release_hygiene")
	if s["version"] != "seq17_p429.v1" {
		t.Fatalf("version=%v, want seq17_p429.v1", s["version"])
	}
	if s["role"] != "chroma_release_hygiene" {
		t.Fatalf("role=%v, want chroma_release_hygiene", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", s["dry_run_only"])
	}
	if s["actual_migration_run"] != false {
		t.Fatalf("actual_migration_run=%v, want false", s["actual_migration_run"])
	}
	if s["bundle_regenerated"] != false {
		t.Fatalf("bundle_regenerated=%v, want false", s["bundle_regenerated"])
	}
	if s["release_ready"] != false {
		t.Fatalf("release_ready=%v, want false", s["release_ready"])
	}
	if s["operator_approval_required"] != true {
		t.Fatalf("operator_approval_required=%v, want true", s["operator_approval_required"])
	}
	if s["mode"] != "chroma_release_hygiene_dry_run" {
		t.Fatalf("mode=%v, want chroma_release_hygiene_dry_run", s["mode"])
	}
}

// TestSeq17P430ChromaMigrationVisibilityGuard validates the 17-C11 migration
// visibility guard dry-run surface for SEQ-17-P430.
func TestSeq17P430ChromaMigrationVisibilityGuard(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p430","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
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
	s := seq165Map(t, resp, "chroma_migration_visibility_guard")
	if s["version"] != "seq17_p430.v1" {
		t.Fatalf("version=%v, want seq17_p430.v1", s["version"])
	}
	if s["role"] != "chroma_migration_visibility_guard" {
		t.Fatalf("role=%v, want chroma_migration_visibility_guard", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", s["dry_run_only"])
	}
	if s["actual_migration_run"] != false {
		t.Fatalf("actual_migration_run=%v, want false", s["actual_migration_run"])
	}
	if s["dashboard_mutation"] != false {
		t.Fatalf("dashboard_mutation=%v, want false", s["dashboard_mutation"])
	}
	if s["operator_approval_required"] != true {
		t.Fatalf("operator_approval_required=%v, want true", s["operator_approval_required"])
	}
	if s["mode"] != "chroma_migration_visibility_guard_dry_run" {
		t.Fatalf("mode=%v, want chroma_migration_visibility_guard_dry_run", s["mode"])
	}
}
