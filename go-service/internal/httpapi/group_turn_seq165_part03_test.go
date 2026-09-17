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

	if resp["fallback_reason"] != "store_unavailable" {
		t.Fatalf("fallback_reason=%v, want store_unavailable", resp["fallback_reason"])
	}

	gp, ok := resp["generation_packet"].(map[string]any)
	if !ok {
		t.Fatalf("missing generation_packet")
	}
	if gp["packet_mode"] != "off" {
		t.Fatalf("packet_mode=%v, want off", gp["packet_mode"])
	}

	ip, ok := resp["injection_pack"].(map[string]any)
	if !ok {
		t.Fatalf("missing injection_pack")
	}
	status, _ := ip["status"].(string)
	if status != "off" && status != "skeleton" {
		t.Fatalf("injection_pack status=%v, want off or skeleton", status)
	}

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

	maxChars, ok := traceSummary["max_injection_chars"].(float64)
	if !ok || maxChars <= 0 {
		t.Fatalf("max_injection_chars=%v, want > 0", traceSummary["max_injection_chars"])
	}
	if maxChars != 2500 {
		t.Fatalf("max_injection_chars=%v, want 2500", maxChars)
	}

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
