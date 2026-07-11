package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestPrepareTurnWeakInputPlannerContract(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-weak-plan", TurnIndex: 7, Role: "user", Content: "Mina asks Rowan what they should do next."},
			{ID: 2, ChatSessionID: "sess-weak-plan", TurnIndex: 7, Role: "assistant", Content: "Rowan pauses at the shrine gate and waits for Mina's lead."},
		},
		returnResumePack: &store.ResumePack{
			Trigger:       "resume",
			AssembledText: "Mina and Rowan are paused at the shrine gate.",
		},
	}
	srv := NewServer(config.Default())
	srv.Store = fake
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-weak-plan","turn_index":8,"raw_user_input":"계속","client_meta":{"language_context":{"session_output_language":"ko","output_language_source":"plugin_setting"}},"settings":{"max_injection_chars":1600,"max_input_context_chars":900,"injection_enabled":true,"input_context_enabled":true,"top_k":2,"guide_mode":"standard","guide_strength":"weak","narrative_stance":"balanced"}}`
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
	planner, ok := resp["weak_input_planner"].(map[string]any)
	if !ok {
		t.Fatalf("weak_input_planner missing: %#v", resp["weak_input_planner"])
	}
	if planner["contract_version"] != "step25_weak_input_planner.v1" || planner["taxonomy"] != "continuation_trigger" || planner["status"] != "ready" {
		t.Fatalf("unexpected planner contract: %#v", planner)
	}
	if planner["active"] != true || planner["truth_authority"] != false || planner["would_write"] != false || planner["would_call_llm"] != false {
		t.Fatalf("planner must be active support-only, got %#v", planner)
	}
	boundary, ok := planner["initiative_boundary"].(map[string]any)
	if !ok || intFromAny(boundary["max_new_beats"], -1) != 1 || boolFromAny(boundary["allow_scene_jump"]) {
		t.Fatalf("planner initiative boundary mismatch: %#v", planner["initiative_boundary"])
	}
	brief, ok := planner["acting_brief"].(map[string]any)
	if !ok || extractionStringFromAny(brief["main_failure_risk"]) != "stall_or_stale_replay" || extractionStringFromAny(brief["reply_strategy"]) == "" {
		t.Fatalf("planner acting brief mismatch: %#v", planner["acting_brief"])
	}
	if len(stringSliceFromAny(planner["selected_anchor_names"])) == 0 {
		t.Fatalf("expected selected anchor trace, got %#v", planner["selected_anchor_names"])
	}
	execContract, ok := resp["planner_execution_contract"].(map[string]any)
	if !ok {
		t.Fatalf("planner_execution_contract missing: %#v", resp["planner_execution_contract"])
	}
	if execContract["contract_version"] != "step25_planner_execution_contract.v1" || execContract["status"] != "ready" || execContract["truth_authority"] != false {
		t.Fatalf("unexpected execution contract: %#v", execContract)
	}
	sceneMandate, ok := execContract["scene_mandate"].(map[string]any)
	if !ok || extractionStringFromAny(sceneMandate["value"]) == "" {
		t.Fatalf("execution contract missing scene mandate: %#v", execContract["scene_mandate"])
	}
	requiredOutcome, ok := execContract["required_outcome"].(map[string]any)
	if !ok || intFromAny(requiredOutcome["count"], 0) <= 0 || len(stringSliceFromAny(requiredOutcome["items"])) == 0 {
		t.Fatalf("execution contract missing required outcomes: %#v", execContract["required_outcome"])
	}
	forbiddenMove, ok := execContract["forbidden_move"].(map[string]any)
	if !ok || intFromAny(forbiddenMove["count"], 0) <= 0 || len(stringSliceFromAny(forbiddenMove["items"])) == 0 {
		t.Fatalf("execution contract missing forbidden moves: %#v", execContract["forbidden_move"])
	}
	pacing, ok := execContract["pacing_pressure"].(map[string]any)
	if !ok || intFromAny(pacing["max_new_beats"], -1) != 1 || boolFromAny(pacing["allow_scene_jump"]) {
		t.Fatalf("execution contract pacing mismatch: %#v", execContract["pacing_pressure"])
	}
	consumeRule, ok := execContract["consume_rule"].(map[string]any)
	if !ok || len(stringSliceFromAny(consumeRule["blocked_usage"])) == 0 {
		t.Fatalf("execution contract missing consume rule: %#v", execContract["consume_rule"])
	}
	progressChoice, ok := resp["progression_choice_ledger"].(map[string]any)
	if !ok {
		t.Fatalf("progression_choice_ledger missing: %#v", resp["progression_choice_ledger"])
	}
	if progressChoice["contract_version"] != "step25_progression_choice_ledger.v1" || progressChoice["status"] != "ready" || progressChoice["truth_authority"] != false {
		t.Fatalf("unexpected progression choice ledger: %#v", progressChoice)
	}
	if progressChoice["choice"] != "advance" {
		t.Fatalf("weak input with live anchors should choose bounded advance, got %#v", progressChoice["choice"])
	}
	sceneLedger, ok := progressChoice["scene_advancement_ledger"].(map[string]any)
	if !ok || intFromAny(sceneLedger["selected_anchor_count"], 0) <= 0 {
		t.Fatalf("progression choice missing scene ledger anchors: %#v", progressChoice["scene_advancement_ledger"])
	}
	callbackEval, ok := progressChoice["callback_evaluation"].(map[string]any)
	if !ok || boolFromAny(callbackEval["stale_revival_suppressed"]) {
		t.Fatalf("progression callback evaluation mismatch: %#v", progressChoice["callback_evaluation"])
	}
	stall, ok := progressChoice["same_incident_stall_detection"].(map[string]any)
	if !ok || boolFromAny(stall["detected"]) {
		t.Fatalf("progression same-incident detector should not trip: %#v", progressChoice["same_incident_stall_detection"])
	}
	step25Gate, ok := resp["step25_validation_gate"].(map[string]any)
	if !ok {
		t.Fatalf("step25_validation_gate missing: %#v", resp["step25_validation_gate"])
	}
	if step25Gate["contract_version"] != "step25_validation_gate.v1" || step25Gate["gate_status"] != "pass" || step25Gate["adoption_ready"] != true {
		t.Fatalf("unexpected Step 25 validation gate: %#v", step25Gate)
	}
	if intFromAny(step25Gate["passed_count"], 0) != intFromAny(step25Gate["total_count"], -1) || intFromAny(step25Gate["total_count"], 0) < 8 {
		t.Fatalf("Step 25 validation gate should pass all checks: %#v", step25Gate)
	}
	if blocking := stringSliceFromAny(step25Gate["blocking_check_ids"]); len(blocking) > 0 {
		t.Fatalf("Step 25 validation gate had blockers: %#v", blocking)
	}
	supervisor, ok := resp["supervisor_input_pack"].(map[string]any)
	if !ok {
		t.Fatalf("supervisor_input_pack missing")
	}
	if _, ok := supervisor["weak_input_planner"].(map[string]any); !ok {
		t.Fatalf("supervisor pack missing weak planner contract: %#v", supervisor["weak_input_planner"])
	}
	if _, ok := supervisor["planner_execution_contract"].(map[string]any); !ok {
		t.Fatalf("supervisor pack missing planner execution contract: %#v", supervisor["planner_execution_contract"])
	}
	if _, ok := supervisor["progression_choice_ledger"].(map[string]any); !ok {
		t.Fatalf("supervisor pack missing progression choice ledger: %#v", supervisor["progression_choice_ledger"])
	}
	if _, ok := supervisor["step25_validation_gate"].(map[string]any); !ok {
		t.Fatalf("supervisor pack missing Step 25 validation gate: %#v", supervisor["step25_validation_gate"])
	}
	guidance := extractionStringFromAny(supervisor["final_guidance_suffix"])
	if !strings.Contains(guidance, "[Weak Input Planner]") ||
		!strings.Contains(guidance, "[Planner Execution Contract]") ||
		!strings.Contains(guidance, "[Progression Choice Ledger]") ||
		!strings.Contains(guidance, "current_user_input_priority=highest") ||
		!strings.Contains(guidance, "truth_authority=false") {
		t.Fatalf("supervisor guidance missing support-only planner contracts: %q", guidance)
	}
}
