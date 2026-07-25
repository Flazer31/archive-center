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

func TestPrepareTurnDoesNotExposeStoryCompositionPlanner(t *testing.T) {
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
	for _, key := range []string{"weak_input_planner", "progression_choice_ledger", "step25_validation_gate", "autonomy_plan", "micro_beat_proposal", "scene_step_proposal", "combined_proposal"} {
		if _, exists := resp[key]; exists {
			t.Fatalf("prepare-turn exposes story-composition surface %q: %#v", key, resp[key])
		}
	}
	execContract, ok := resp["response_execution_contract"].(map[string]any)
	if !ok {
		t.Fatalf("response_execution_contract missing: %#v", resp["response_execution_contract"])
	}
	if execContract["contract_version"] != "response_execution_contract.v1" || execContract["status"] != "ready" || execContract["truth_authority"] != false {
		t.Fatalf("unexpected execution contract: %#v", execContract)
	}
	for _, key := range []string{"must_preserve", "must_respond", "must_account", "must_not_assert", "source_refs"} {
		if _, ok := execContract[key].(map[string]any); !ok {
			t.Fatalf("response execution contract missing %s: %#v", key, execContract[key])
		}
	}
	for _, key := range []string{"scene_mandate", "required_outcome", "forbidden_move", "pacing_pressure", "ending_requirement", "role_lens_consumption"} {
		if _, exists := execContract[key]; exists {
			t.Fatalf("execution contract controls story composition through %q: %#v", key, execContract[key])
		}
	}
	consumeRule, ok := execContract["consume_rule"].(map[string]any)
	if !ok || len(stringSliceFromAny(consumeRule["blocked_usage"])) == 0 {
		t.Fatalf("execution contract missing consume rule: %#v", execContract["consume_rule"])
	}
	supervisor, ok := resp["supervisor_input_pack"].(map[string]any)
	if !ok {
		t.Fatalf("supervisor_input_pack missing")
	}
	if _, ok := supervisor["response_execution_contract"].(map[string]any); !ok {
		t.Fatalf("supervisor pack missing response execution contract: %#v", supervisor["response_execution_contract"])
	}
	for _, key := range []string{"weak_input_planner", "progression_choice_ledger", "step25_validation_gate"} {
		if _, exists := supervisor[key]; exists {
			t.Fatalf("supervisor pack exposes story-composition planner %q: %#v", key, supervisor[key])
		}
	}
	guidance := extractionStringFromAny(supervisor["final_guidance_suffix"])
	for _, forbidden := range []string{"[Weak Input Planner]", "[Response Execution Contract]", "[Progression Choice Ledger]", "scene_mandate=", "pacing=", "ending_requirement="} {
		if strings.Contains(guidance, forbidden) {
			t.Fatalf("supervisor guidance controls story composition through %q: %q", forbidden, guidance)
		}
	}
}
