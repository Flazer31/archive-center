package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSeq18P121VXReplayThresholdReuse validates the VX replay threshold reuse surface.
func TestSeq18P121VXReplayThresholdReuse(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p121","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vx_replay_threshold_reuse")
	if s["version"] != "seq18_p121.v1" {
		t.Fatalf("version=%v, want seq18_p121.v1", s["version"])
	}
	if s["role"] != "vx_replay_threshold_reuse" {
		t.Fatalf("role=%v, want vx_replay_threshold_reuse", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["threshold_source"] != "_U1E_CAPTURED_REPLAY" {
		t.Fatalf("threshold_source=%v, want _U1E_CAPTURED_REPLAY", s["threshold_source"])
	}
	reusedFor, ok := s["reused_for"].([]any)
	if !ok || len(reusedFor) != 2 {
		t.Fatalf("reused_for=%v, want 2 items", s["reused_for"])
	}
	if s["disconnected_set_avoided"] != true {
		t.Fatalf("disconnected_set_avoided=%v, want true", s["disconnected_set_avoided"])
	}
	if s["policy_version"] != "vx18a.v1" {
		t.Fatalf("policy_version=%v, want vx18a.v1", s["policy_version"])
	}
	if s["mode"] != "vx_replay_threshold_reuse_surface" {
		t.Fatalf("mode=%v, want vx_replay_threshold_reuse_surface", s["mode"])
	}
}

// TestSeq18P122VXHybridReplayStates validates the VX hybrid replay gate states surface.
func TestSeq18P122VXHybridReplayStates(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p122","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vx_hybrid_replay_states")
	if s["version"] != "seq18_p122.v1" {
		t.Fatalf("version=%v, want seq18_p122.v1", s["version"])
	}
	if s["role"] != "vx_hybrid_replay_states" {
		t.Fatalf("role=%v, want vx_hybrid_replay_states", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	states, ok := s["state_machine"].([]any)
	if !ok || len(states) != 3 {
		t.Fatalf("state_machine=%v, want 3 items", s["state_machine"])
	}
	if s["policy_version"] != "vx18a.v1" {
		t.Fatalf("policy_version=%v, want vx18a.v1", s["policy_version"])
	}
	if s["mode"] != "vx_hybrid_replay_states_surface" {
		t.Fatalf("mode=%v, want vx_hybrid_replay_states_surface", s["mode"])
	}
}

// TestSeq18P123VXHybridReplayTest validates the VX hybrid replay gate test surface.
func TestSeq18P123VXHybridReplayTest(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p123","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vx_hybrid_replay_test")
	if s["version"] != "seq18_p123.v1" {
		t.Fatalf("version=%v, want seq18_p123.v1", s["version"])
	}
	if s["role"] != "vx_hybrid_replay_test" {
		t.Fatalf("role=%v, want vx_hybrid_replay_test", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["test_file"] != "backend/test_step18_hybrid_replay_gate.py" {
		t.Fatalf("test_file=%v, want backend/test_step18_hybrid_replay_gate.py", s["test_file"])
	}
	guards, ok := s["guards"].([]any)
	if !ok || len(guards) != 3 {
		t.Fatalf("guards=%v, want 3 items", s["guards"])
	}
	if s["policy_version"] != "vx18a.v1" {
		t.Fatalf("policy_version=%v, want vx18a.v1", s["policy_version"])
	}
	if s["mode"] != "vx_hybrid_replay_test_surface" {
		t.Fatalf("mode=%v, want vx_hybrid_replay_test_surface", s["mode"])
	}
}

// TestSeq18P130VXHeldoutCompletenessGate validates the VX heldout completeness gate surface.
func TestSeq18P130VXHeldoutCompletenessGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p130","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vx_heldout_completeness_gate")
	if s["version"] != "seq18_p130.v1" {
		t.Fatalf("version=%v, want seq18_p130.v1", s["version"])
	}
	if s["role"] != "vx_heldout_completeness_gate" {
		t.Fatalf("role=%v, want vx_heldout_completeness_gate", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["gate_version"] != "vx18b.v1" {
		t.Fatalf("gate_version=%v, want vx18b.v1", s["gate_version"])
	}
	if s["gate_name"] != "heldout_completeness" {
		t.Fatalf("gate_name=%v, want heldout_completeness", s["gate_name"])
	}
	if s["execution_unchanged"] != true {
		t.Fatalf("execution_unchanged=%v, want true", s["execution_unchanged"])
	}
	if s["routing_unchanged"] != true {
		t.Fatalf("routing_unchanged=%v, want true", s["routing_unchanged"])
	}
	if s["additive_only"] != true {
		t.Fatalf("additive_only=%v, want true", s["additive_only"])
	}
	if s["policy_version"] != "vx18b.v1" {
		t.Fatalf("policy_version=%v, want vx18b.v1", s["policy_version"])
	}
	if s["mode"] != "vx_heldout_completeness_gate_surface" {
		t.Fatalf("mode=%v, want vx_heldout_completeness_gate_surface", s["mode"])
	}
}

// TestSeq18P131VXHeldoutMetrics validates the VX heldout metrics surface.
func TestSeq18P131VXHeldoutMetrics(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p131","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vx_heldout_metrics")
	if s["version"] != "seq18_p131.v1" {
		t.Fatalf("version=%v, want seq18_p131.v1", s["version"])
	}
	if s["role"] != "vx_heldout_metrics" {
		t.Fatalf("role=%v, want vx_heldout_metrics", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	metrics, ok := s["metrics"].([]any)
	if !ok || len(metrics) != 3 {
		t.Fatalf("metrics=%v, want 3 items", s["metrics"])
	}
	if s["sample_sufficiency_required"] != true {
		t.Fatalf("sample_sufficiency_required=%v, want true", s["sample_sufficiency_required"])
	}
	states, ok := s["state_rules"].([]any)
	if !ok || len(states) != 4 {
		t.Fatalf("state_rules=%v, want 4 items", s["state_rules"])
	}
	if s["policy_version"] != "vx18b.v1" {
		t.Fatalf("policy_version=%v, want vx18b.v1", s["policy_version"])
	}
	if s["mode"] != "vx_heldout_metrics_surface" {
		t.Fatalf("mode=%v, want vx_heldout_metrics_surface", s["mode"])
	}
}

// TestSeq18P132VXHeldoutThresholdReuse validates the VX heldout threshold reuse surface.
func TestSeq18P132VXHeldoutThresholdReuse(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p132","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vx_heldout_threshold_reuse")
	if s["version"] != "seq18_p132.v1" {
		t.Fatalf("version=%v, want seq18_p132.v1", s["version"])
	}
	if s["role"] != "vx_heldout_threshold_reuse" {
		t.Fatalf("role=%v, want vx_heldout_threshold_reuse", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["completeness_floor"] != "LC1P_healthy" {
		t.Fatalf("completeness_floor=%v, want LC1P_healthy", s["completeness_floor"])
	}
	if s["sample_threshold_source"] != "_U1E_CAPTURED_REPLAY_MIN" {
		t.Fatalf("sample_threshold_source=%v, want _U1E_CAPTURED_REPLAY_MIN", s["sample_threshold_source"])
	}
	if s["disconnected_literal_avoided"] != true {
		t.Fatalf("disconnected_literal_avoided=%v, want true", s["disconnected_literal_avoided"])
	}
	if s["policy_version"] != "vx18b.v1" {
		t.Fatalf("policy_version=%v, want vx18b.v1", s["policy_version"])
	}
	if s["mode"] != "vx_heldout_threshold_reuse_surface" {
		t.Fatalf("mode=%v, want vx_heldout_threshold_reuse_surface", s["mode"])
	}
}

// TestSeq18P133VXHeldoutCompletenessTest validates the VX heldout completeness test surface.
func TestSeq18P133VXHeldoutCompletenessTest(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p133","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vx_heldout_completeness_test")
	if s["version"] != "seq18_p133.v1" {
		t.Fatalf("version=%v, want seq18_p133.v1", s["version"])
	}
	if s["role"] != "vx_heldout_completeness_test" {
		t.Fatalf("role=%v, want vx_heldout_completeness_test", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["test_file"] != "backend/test_step18_heldout_completeness_gate.py" {
		t.Fatalf("test_file=%v, want backend/test_step18_heldout_completeness_gate.py", s["test_file"])
	}
	guards, ok := s["guards"].([]any)
	if !ok || len(guards) != 4 {
		t.Fatalf("guards=%v, want 4 items", s["guards"])
	}
	if s["policy_version"] != "vx18b.v1" {
		t.Fatalf("policy_version=%v, want vx18b.v1", s["policy_version"])
	}
	if s["mode"] != "vx_heldout_completeness_test_surface" {
		t.Fatalf("mode=%v, want vx_heldout_completeness_test_surface", s["mode"])
	}
}

// TestSeq18P140VXLatencyTokenBudgetGate validates the VX latency/token budget gate surface.
func TestSeq18P140VXLatencyTokenBudgetGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p140","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vx_latency_token_budget_gate")
	if s["version"] != "seq18_p140.v1" {
		t.Fatalf("version=%v, want seq18_p140.v1", s["version"])
	}
	if s["role"] != "vx_latency_token_budget_gate" {
		t.Fatalf("role=%v, want vx_latency_token_budget_gate", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["gate_version"] != "vx18c.v1" {
		t.Fatalf("gate_version=%v, want vx18c.v1", s["gate_version"])
	}
	if s["gate_name"] != "latency_token_budget" {
		t.Fatalf("gate_name=%v, want latency_token_budget", s["gate_name"])
	}
	if s["execution_unchanged"] != true {
		t.Fatalf("execution_unchanged=%v, want true", s["execution_unchanged"])
	}
	if s["routing_unchanged"] != true {
		t.Fatalf("routing_unchanged=%v, want true", s["routing_unchanged"])
	}
	if s["additive_only"] != true {
		t.Fatalf("additive_only=%v, want true", s["additive_only"])
	}
	if s["policy_version"] != "vx18c.v1" {
		t.Fatalf("policy_version=%v, want vx18c.v1", s["policy_version"])
	}
	if s["mode"] != "vx_latency_token_budget_gate_surface" {
		t.Fatalf("mode=%v, want vx_latency_token_budget_gate_surface", s["mode"])
	}
}

// TestSeq18P141VXLatencyTokenMetrics validates the VX latency/token metrics surface.
func TestSeq18P141VXLatencyTokenMetrics(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p141","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vx_latency_token_metrics")
	if s["version"] != "seq18_p141.v1" {
		t.Fatalf("version=%v, want seq18_p141.v1", s["version"])
	}
	if s["role"] != "vx_latency_token_metrics" {
		t.Fatalf("role=%v, want vx_latency_token_metrics", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	metrics, ok := s["metrics"].([]any)
	if !ok || len(metrics) != 3 {
		t.Fatalf("metrics=%v, want 3 items", s["metrics"])
	}
	if s["sample_sufficiency_required"] != true {
		t.Fatalf("sample_sufficiency_required=%v, want true", s["sample_sufficiency_required"])
	}
	if s["default_token_ceiling"] != "packet_budget_policy.max_injection_chars" {
		t.Fatalf("default_token_ceiling=%v, want packet_budget_policy.max_injection_chars", s["default_token_ceiling"])
	}
	if s["policy_version"] != "vx18c.v1" {
		t.Fatalf("policy_version=%v, want vx18c.v1", s["policy_version"])
	}
	if s["mode"] != "vx_latency_token_metrics_surface" {
		t.Fatalf("mode=%v, want vx_latency_token_metrics_surface", s["mode"])
	}
}

// TestSeq18P142VXLatencyTokenThresholdReuse validates the VX latency/token threshold reuse surface.
func TestSeq18P142VXLatencyTokenThresholdReuse(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p142","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vx_latency_token_threshold_reuse")
	if s["version"] != "seq18_p142.v1" {
		t.Fatalf("version=%v, want seq18_p142.v1", s["version"])
	}
	if s["role"] != "vx_latency_token_threshold_reuse" {
		t.Fatalf("role=%v, want vx_latency_token_threshold_reuse", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["latency_ceiling_source"] != "_LC1M_MAX_SPLIT_LATENCY_MULTIPLIER" {
		t.Fatalf("latency_ceiling_source=%v, want _LC1M_MAX_SPLIT_LATENCY_MULTIPLIER", s["latency_ceiling_source"])
	}
	if s["token_ratio_owner"] != "gate_constant" {
		t.Fatalf("token_ratio_owner=%v, want gate_constant", s["token_ratio_owner"])
	}
	if s["scattered_literals_avoided"] != true {
		t.Fatalf("scattered_literals_avoided=%v, want true", s["scattered_literals_avoided"])
	}
	if s["policy_version"] != "vx18c.v1" {
		t.Fatalf("policy_version=%v, want vx18c.v1", s["policy_version"])
	}
	if s["mode"] != "vx_latency_token_threshold_reuse_surface" {
		t.Fatalf("mode=%v, want vx_latency_token_threshold_reuse_surface", s["mode"])
	}
}

// TestSeq18P143VXLatencyTokenTest validates the VX latency/token budget test surface.
func TestSeq18P143VXLatencyTokenTest(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p143","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vx_latency_token_test")
	if s["version"] != "seq18_p143.v1" {
		t.Fatalf("version=%v, want seq18_p143.v1", s["version"])
	}
	if s["role"] != "vx_latency_token_test" {
		t.Fatalf("role=%v, want vx_latency_token_test", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["test_file"] != "backend/test_step18_latency_token_budget_gate.py" {
		t.Fatalf("test_file=%v, want backend/test_step18_latency_token_budget_gate.py", s["test_file"])
	}
	guards, ok := s["guards"].([]any)
	if !ok || len(guards) != 4 {
		t.Fatalf("guards=%v, want 4 items", s["guards"])
	}
	if s["policy_version"] != "vx18c.v1" {
		t.Fatalf("policy_version=%v, want vx18c.v1", s["policy_version"])
	}
	if s["mode"] != "vx_latency_token_test_surface" {
		t.Fatalf("mode=%v, want vx_latency_token_test_surface", s["mode"])
	}
}

// TestSeq18P150VXTruthBoundaryGate validates the VX truth-boundary gate surface.
func TestSeq18P150VXTruthBoundaryGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p150","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vx_truth_boundary_gate")
	if s["version"] != "seq18_p150.v1" {
		t.Fatalf("version=%v, want seq18_p150.v1", s["version"])
	}
	if s["role"] != "vx_truth_boundary_gate" {
		t.Fatalf("role=%v, want vx_truth_boundary_gate", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["gate_version"] != "vx18d.v1" {
		t.Fatalf("gate_version=%v, want vx18d.v1", s["gate_version"])
	}
	if s["gate_name"] != "truth_boundary_replay" {
		t.Fatalf("gate_name=%v, want truth_boundary_replay", s["gate_name"])
	}
	if s["evaluates_after"] != "injection_pack_data.packet_composition" {
		t.Fatalf("evaluates_after=%v, want injection_pack_data.packet_composition", s["evaluates_after"])
	}
	if s["additive_only"] != true {
		t.Fatalf("additive_only=%v, want true", s["additive_only"])
	}
	if s["policy_version"] != "vx18d.v1" {
		t.Fatalf("policy_version=%v, want vx18d.v1", s["policy_version"])
	}
	if s["mode"] != "vx_truth_boundary_gate_surface" {
		t.Fatalf("mode=%v, want vx_truth_boundary_gate_surface", s["mode"])
	}
}

// TestSeq18P151VXTruthBoundaryPrecedence validates the VX truth-boundary precedence surface.
func TestSeq18P151VXTruthBoundaryPrecedence(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p151","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vx_truth_boundary_precedence")
	if s["version"] != "seq18_p151.v1" {
		t.Fatalf("version=%v, want seq18_p151.v1", s["version"])
	}
	if s["role"] != "vx_truth_boundary_precedence" {
		t.Fatalf("role=%v, want vx_truth_boundary_precedence", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	evalFields, ok := s["evaluated_fields"].([]any)
	if !ok || len(evalFields) != 2 {
		t.Fatalf("evaluated_fields=%v, want 2 items", s["evaluated_fields"])
	}
	if s["precedence_model"] != "_LC1K_HIGH_AUTHORITY_SOURCES_vs_LOWER_TIER_SOURCES" {
		t.Fatalf("precedence_model=%v, want _LC1K_HIGH_AUTHORITY_SOURCES_vs_LOWER_TIER_SOURCES", s["precedence_model"])
	}
	if s["policy_version"] != "vx18d.v1" {
		t.Fatalf("policy_version=%v, want vx18d.v1", s["policy_version"])
	}
	if s["mode"] != "vx_truth_boundary_precedence_surface" {
		t.Fatalf("mode=%v, want vx_truth_boundary_precedence_surface", s["mode"])
	}
}

// TestSeq18P152VXTruthBoundaryStates validates the VX truth-boundary states surface.
func TestSeq18P152VXTruthBoundaryStates(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p152","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vx_truth_boundary_states")
	if s["version"] != "seq18_p152.v1" {
		t.Fatalf("version=%v, want seq18_p152.v1", s["version"])
	}
	if s["role"] != "vx_truth_boundary_states" {
		t.Fatalf("role=%v, want vx_truth_boundary_states", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	states, ok := s["state_machine"].([]any)
	if !ok || len(states) != 4 {
		t.Fatalf("state_machine=%v, want 4 items", s["state_machine"])
	}
	if s["policy_version"] != "vx18d.v1" {
		t.Fatalf("policy_version=%v, want vx18d.v1", s["policy_version"])
	}
	if s["mode"] != "vx_truth_boundary_states_surface" {
		t.Fatalf("mode=%v, want vx_truth_boundary_states_surface", s["mode"])
	}
}

// TestSeq18P153VXTruthBoundaryTest validates the VX truth-boundary test surface.
func TestSeq18P153VXTruthBoundaryTest(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p153","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vx_truth_boundary_test")
	if s["version"] != "seq18_p153.v1" {
		t.Fatalf("version=%v, want seq18_p153.v1", s["version"])
	}
	if s["role"] != "vx_truth_boundary_test" {
		t.Fatalf("role=%v, want vx_truth_boundary_test", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["test_file"] != "backend/test_step18_truth_boundary_replay_gate.py" {
		t.Fatalf("test_file=%v, want backend/test_step18_truth_boundary_replay_gate.py", s["test_file"])
	}
	guards, ok := s["guards"].([]any)
	if !ok || len(guards) != 4 {
		t.Fatalf("guards=%v, want 4 items", s["guards"])
	}
	if s["policy_version"] != "vx18d.v1" {
		t.Fatalf("policy_version=%v, want vx18d.v1", s["policy_version"])
	}
	if s["mode"] != "vx_truth_boundary_test_surface" {
		t.Fatalf("mode=%v, want vx_truth_boundary_test_surface", s["mode"])
	}
}

// TestSeq18P160VXTruncationSummaryLossGate validates the truncation_summary_loss
// gate surface.
func TestSeq18P160VXTruncationSummaryLossGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p160","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vx_truncation_summary_loss_gate")
	if s["version"] != "seq18_p160.v1" {
		t.Fatalf("version=%v, want seq18_p160.v1", s["version"])
	}
	if s["role"] != "vx_truncation_summary_loss_gate" {
		t.Fatalf("role=%v, want vx_truncation_summary_loss_gate", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["gate_version"] != "vx18e.v1" {
		t.Fatalf("gate_version=%v, want vx18e.v1", s["gate_version"])
	}
	if s["gate_name"] != "truncation_summary_loss" {
		t.Fatalf("gate_name=%v, want truncation_summary_loss", s["gate_name"])
	}
	if s["additive_only"] != true {
		t.Fatalf("additive_only=%v, want true", s["additive_only"])
	}
	if s["policy_version"] != "vx18e.v1" {
		t.Fatalf("policy_version=%v, want vx18e.v1", s["policy_version"])
	}
	if s["mode"] != "vx_truncation_summary_loss_gate_surface" {
		t.Fatalf("mode=%v, want vx_truncation_summary_loss_gate_surface", s["mode"])
	}
}

// TestSeq18P161VXTruncationSummaryLossMetrics validates the truncation_summary_loss
// metrics surface.
func TestSeq18P161VXTruncationSummaryLossMetrics(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p161","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vx_truncation_summary_loss_metrics")
	if s["version"] != "seq18_p161.v1" {
		t.Fatalf("version=%v, want seq18_p161.v1", s["version"])
	}
	if s["role"] != "vx_truncation_summary_loss_metrics" {
		t.Fatalf("role=%v, want vx_truncation_summary_loss_metrics", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	metrics, ok := s["metrics"].([]any)
	if !ok || len(metrics) != 4 {
		t.Fatalf("metrics=%v, want 4 items", s["metrics"])
	}
	if s["sample_sufficiency_required"] != true {
		t.Fatalf("sample_sufficiency_required=%v, want true", s["sample_sufficiency_required"])
	}
	trace, ok := s["trace_carry_in"].([]any)
	if !ok || len(trace) != 3 {
		t.Fatalf("trace_carry_in=%v, want 3 items", s["trace_carry_in"])
	}
	if s["policy_version"] != "vx18e.v1" {
		t.Fatalf("policy_version=%v, want vx18e.v1", s["policy_version"])
	}
	if s["mode"] != "vx_truncation_summary_loss_metrics_surface" {
		t.Fatalf("mode=%v, want vx_truncation_summary_loss_metrics_surface", s["mode"])
	}
}

// TestSeq18P162VXTruncationSummaryLossThresholdReuse validates the threshold reuse
// surface.
func TestSeq18P162VXTruncationSummaryLossThresholdReuse(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p162","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vx_truncation_summary_loss_threshold_reuse")
	if s["version"] != "seq18_p162.v1" {
		t.Fatalf("version=%v, want seq18_p162.v1", s["version"])
	}
	if s["role"] != "vx_truncation_summary_loss_threshold_reuse" {
		t.Fatalf("role=%v, want vx_truncation_summary_loss_threshold_reuse", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["sample_threshold_source"] != "_U1E_CAPTURED_REPLAY_MIN_*" {
		t.Fatalf("sample_threshold_source=%v, want _U1E_CAPTURED_REPLAY_MIN_*", s["sample_threshold_source"])
	}
	if s["threshold_owner"] != "gate_constant_block" {
		t.Fatalf("threshold_owner=%v, want gate_constant_block", s["threshold_owner"])
	}
	if s["scattered_literals_avoided"] != true {
		t.Fatalf("scattered_literals_avoided=%v, want true", s["scattered_literals_avoided"])
	}
	if s["policy_version"] != "vx18e.v1" {
		t.Fatalf("policy_version=%v, want vx18e.v1", s["policy_version"])
	}
	if s["mode"] != "vx_truncation_summary_loss_threshold_reuse_surface" {
		t.Fatalf("mode=%v, want vx_truncation_summary_loss_threshold_reuse_surface", s["mode"])
	}
}

// TestSeq18P163VXTruncationSummaryLossStates validates the state machine surface.
func TestSeq18P163VXTruncationSummaryLossStates(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p163","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vx_truncation_summary_loss_states")
	if s["version"] != "seq18_p163.v1" {
		t.Fatalf("version=%v, want seq18_p163.v1", s["version"])
	}
	if s["role"] != "vx_truncation_summary_loss_states" {
		t.Fatalf("role=%v, want vx_truncation_summary_loss_states", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	states, ok := s["state_machine"].([]any)
	if !ok || len(states) != 4 {
		t.Fatalf("state_machine=%v, want 4 items", s["state_machine"])
	}
	if s["policy_version"] != "vx18e.v1" {
		t.Fatalf("policy_version=%v, want vx18e.v1", s["policy_version"])
	}
	if s["mode"] != "vx_truncation_summary_loss_states_surface" {
		t.Fatalf("mode=%v, want vx_truncation_summary_loss_states_surface", s["mode"])
	}
}

// TestSeq18P164VXTruncationSummaryLossTest validates the combined regression bundle
// test surface.
func TestSeq18P164VXTruncationSummaryLossTest(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p164","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vx_truncation_summary_loss_test")
	if s["version"] != "seq18_p164.v1" {
		t.Fatalf("version=%v, want seq18_p164.v1", s["version"])
	}
	if s["role"] != "vx_truncation_summary_loss_test" {
		t.Fatalf("role=%v, want vx_truncation_summary_loss_test", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["test_file"] != "backend/test_step18_truncation_summary_loss_gate.py" {
		t.Fatalf("test_file=%v, want backend/test_step18_truncation_summary_loss_gate.py", s["test_file"])
	}
	guards, ok := s["guards"].([]any)
	if !ok || len(guards) != 4 {
		t.Fatalf("guards=%v, want 4 items", s["guards"])
	}
	if s["combined_bundle_status"] != "green" {
		t.Fatalf("combined_bundle_status=%v, want green", s["combined_bundle_status"])
	}
	components, ok := s["bundle_components"].([]any)
	if !ok || len(components) != 3 {
		t.Fatalf("bundle_components=%v, want 3 items", s["bundle_components"])
	}
	if s["policy_version"] != "vx18e.v1" {
		t.Fatalf("policy_version=%v, want vx18e.v1", s["policy_version"])
	}
	if s["mode"] != "vx_truncation_summary_loss_test_surface" {
		t.Fatalf("mode=%v, want vx_truncation_summary_loss_test_surface", s["mode"])
	}
}

// TestSeq18P327PostChromaTop1ScopedVerbatim validates the Post-Chroma Top 1 summary.
func TestSeq18P327PostChromaTop1ScopedVerbatim(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p327","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "post_chroma_top1_scoped_verbatim")
	if s["version"] != "seq18_p327.v1" {
		t.Fatalf("version=%v, want seq18_p327.v1", s["version"])
	}
	if s["role"] != "post_chroma_top1_scoped_verbatim" {
		t.Fatalf("role=%v, want post_chroma_top1_scoped_verbatim", s["role"])
	}
	if s["top"] != float64(1) {
		t.Fatalf("top=%v, want 1", s["top"])
	}
	if s["policy_version"] != "vr18a.v1" {
		t.Fatalf("policy_version=%v, want vr18a.v1", s["policy_version"])
	}
	if s["mode"] != "post_chroma_summary_surface" {
		t.Fatalf("mode=%v, want post_chroma_summary_surface", s["mode"])
	}
}

// TestSeq18P328PostChromaTop2HybridScoring validates the Post-Chroma Top 2 summary.
func TestSeq18P328PostChromaTop2HybridScoring(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p328","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "post_chroma_top2_hybrid_scoring")
	if s["version"] != "seq18_p328.v1" {
		t.Fatalf("version=%v, want seq18_p328.v1", s["version"])
	}
	if s["role"] != "post_chroma_top2_hybrid_scoring" {
		t.Fatalf("role=%v, want post_chroma_top2_hybrid_scoring", s["role"])
	}
	if s["top"] != float64(2) {
		t.Fatalf("top=%v, want 2", s["top"])
	}
	if s["policy_version"] != "hy1a.v1" {
		t.Fatalf("policy_version=%v, want hy1a.v1", s["policy_version"])
	}
}

// TestSeq18P329PostChromaTop3TemporalRelation validates the Post-Chroma Top 3 summary.
func TestSeq18P329PostChromaTop3TemporalRelation(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p329","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "post_chroma_top3_temporal_relation")
	if s["version"] != "seq18_p329.v1" {
		t.Fatalf("version=%v, want seq18_p329.v1", s["version"])
	}
	if s["role"] != "post_chroma_top3_temporal_relation" {
		t.Fatalf("role=%v, want post_chroma_top3_temporal_relation", s["role"])
	}
	if s["top"] != float64(3) {
		t.Fatalf("top=%v, want 3", s["top"])
	}
	if s["policy_version"] != "qr1b.v1" {
		t.Fatalf("policy_version=%v, want qr1b.v1", s["policy_version"])
	}
}

// TestSeq18P330PostChromaTop4TemporalValidity validates the Post-Chroma Top 4 summary.
func TestSeq18P330PostChromaTop4TemporalValidity(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p330","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "post_chroma_top4_temporal_validity")
	if s["version"] != "seq18_p330.v1" {
		t.Fatalf("version=%v, want seq18_p330.v1", s["version"])
	}
	if s["role"] != "post_chroma_top4_temporal_validity" {
		t.Fatalf("role=%v, want post_chroma_top4_temporal_validity", s["role"])
	}
	if s["top"] != float64(4) {
		t.Fatalf("top=%v, want 4", s["top"])
	}
}

// TestSeq18P331PostChromaTop5EntityGraph validates the Post-Chroma Top 5 summary.
func TestSeq18P331PostChromaTop5EntityGraph(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p331","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "post_chroma_top5_entity_graph")
	if s["version"] != "seq18_p331.v1" {
		t.Fatalf("version=%v, want seq18_p331.v1", s["version"])
	}
	if s["role"] != "post_chroma_top5_entity_graph" {
		t.Fatalf("role=%v, want post_chroma_top5_entity_graph", s["role"])
	}
	if s["top"] != float64(5) {
		t.Fatalf("top=%v, want 5", s["top"])
	}
}

// TestSeq18P332PostChromaTop6SelectiveRerank validates the Post-Chroma Top 6 summary.
func TestSeq18P332PostChromaTop6SelectiveRerank(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p332","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "post_chroma_top6_selective_rerank")
	if s["version"] != "seq18_p332.v1" {
		t.Fatalf("version=%v, want seq18_p332.v1", s["version"])
	}
	if s["role"] != "post_chroma_top6_selective_rerank" {
		t.Fatalf("role=%v, want post_chroma_top6_selective_rerank", s["role"])
	}
	if s["top"] != float64(6) {
		t.Fatalf("top=%v, want 6", s["top"])
	}
}

// TestSeq18P336VRRawPreservingSupport validates the VR raw-preserving support summary.
func TestSeq18P336VRRawPreservingSupport(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p336","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vr_raw_preserving_support")
	if s["version"] != "seq18_p336.v1" {
		t.Fatalf("version=%v, want seq18_p336.v1", s["version"])
	}
	if s["role"] != "vr_raw_preserving_support" {
		t.Fatalf("role=%v, want vr_raw_preserving_support", s["role"])
	}
	if s["support_lane"] != "verbatim_recall" {
		t.Fatalf("support_lane=%v, want verbatim_recall", s["support_lane"])
	}
	if s["policy_version"] != "vr18a.v1" {
		t.Fatalf("policy_version=%v, want vr18a.v1", s["policy_version"])
	}
}
