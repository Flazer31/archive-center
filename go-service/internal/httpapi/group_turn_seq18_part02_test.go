package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSeq18P66HYTailBudgetRescuePass validates the HY tail-budget rescue pass surface.
func TestSeq18P66HYTailBudgetRescuePass(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p66","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "hy_tail_budget_rescue_pass")
	if s["version"] != "seq18_p66.v1" {
		t.Fatalf("version=%v, want seq18_p66.v1", s["version"])
	}
	if s["role"] != "hy_tail_budget_rescue_pass" {
		t.Fatalf("role=%v, want hy_tail_budget_rescue_pass", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["rescue_enabled"] != true {
		t.Fatalf("rescue_enabled=%v, want true", s["rescue_enabled"])
	}
	if s["max_promotions_per_pass"] != float64(1) {
		t.Fatalf("max_promotions_per_pass=%v, want 1", s["max_promotions_per_pass"])
	}
	if s["budget_preserved"] != true {
		t.Fatalf("budget_preserved=%v, want true", s["budget_preserved"])
	}
	if s["promotion_trigger"] != "keyword_soft_bias_stronger_than_cutline" {
		t.Fatalf("promotion_trigger=%v, want keyword_soft_bias_stronger_than_cutline", s["promotion_trigger"])
	}
	if s["policy_version"] != "hy1d.v1" {
		t.Fatalf("policy_version=%v, want hy1d.v1", s["policy_version"])
	}
	if s["mode"] != "hy_tail_budget_rescue_pass_surface" {
		t.Fatalf("mode=%v, want hy_tail_budget_rescue_pass_surface", s["mode"])
	}
}

// TestSeq18P67HYTailBudgetRescueTrace validates the HY tail-budget rescue trace surface.
func TestSeq18P67HYTailBudgetRescueTrace(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p67","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "hy_tail_budget_rescue_trace")
	if s["version"] != "seq18_p67.v1" {
		t.Fatalf("version=%v, want seq18_p67.v1", s["version"])
	}
	if s["role"] != "hy_tail_budget_rescue_trace" {
		t.Fatalf("role=%v, want hy_tail_budget_rescue_trace", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	traceFields, ok := s["trace_fields"].([]any)
	if !ok || len(traceFields) != 5 {
		t.Fatalf("trace_fields=%v, want 5 items", s["trace_fields"])
	}
	if s["trace_mandatory"] != true {
		t.Fatalf("trace_mandatory=%v, want true", s["trace_mandatory"])
	}
	if s["policy_version"] != "hy1d.v1" {
		t.Fatalf("policy_version=%v, want hy1d.v1", s["policy_version"])
	}
	if s["mode"] != "hy_tail_budget_rescue_trace_surface" {
		t.Fatalf("mode=%v, want hy_tail_budget_rescue_trace_surface", s["mode"])
	}
}

// TestSeq18P68HYTailBudgetQ1aPropagation validates the HY tail-budget q1a propagation surface.
func TestSeq18P68HYTailBudgetQ1aPropagation(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p68","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "hy_tail_budget_q1a_propagation")
	if s["version"] != "seq18_p68.v1" {
		t.Fatalf("version=%v, want seq18_p68.v1", s["version"])
	}
	if s["role"] != "hy_tail_budget_q1a_propagation" {
		t.Fatalf("role=%v, want hy_tail_budget_q1a_propagation", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["q1a_propagation"] != true {
		t.Fatalf("q1a_propagation=%v, want true", s["q1a_propagation"])
	}
	propFields, ok := s["propagated_fields"].([]any)
	if !ok || len(propFields) != 5 {
		t.Fatalf("propagated_fields=%v, want 5 items", s["propagated_fields"])
	}
	if s["policy_version"] != "hy1d.v1" {
		t.Fatalf("policy_version=%v, want hy1d.v1", s["policy_version"])
	}
	if s["mode"] != "hy_tail_budget_q1a_propagation_surface" {
		t.Fatalf("mode=%v, want hy_tail_budget_q1a_propagation_surface", s["mode"])
	}
}

// TestSeq18P69HYTailBudgetRegression validates the HY tail-budget regression surface.
func TestSeq18P69HYTailBudgetRegression(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p69","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "hy_tail_budget_regression")
	if s["version"] != "seq18_p69.v1" {
		t.Fatalf("version=%v, want seq18_p69.v1", s["version"])
	}
	if s["role"] != "hy_tail_budget_regression" {
		t.Fatalf("role=%v, want hy_tail_budget_regression", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["test_file"] != "backend/test_step18_hybrid_regression.py" {
		t.Fatalf("test_file=%v, want backend/test_step18_hybrid_regression.py", s["test_file"])
	}
	if s["regression_scope"] != "near_cutoff_rescue" {
		t.Fatalf("regression_scope=%v, want near_cutoff_rescue", s["regression_scope"])
	}
	verifies, ok := s["verifies"].([]any)
	if !ok || len(verifies) != 2 {
		t.Fatalf("verifies=%v, want 2 items", s["verifies"])
	}
	if s["policy_version"] != "hy1d.v1" {
		t.Fatalf("policy_version=%v, want hy1d.v1", s["policy_version"])
	}
	if s["mode"] != "hy_tail_budget_regression_surface" {
		t.Fatalf("mode=%v, want hy_tail_budget_regression_surface", s["mode"])
	}
}

// TestSeq18P76QRQueryClassContract validates the QR query-class contract surface.
func TestSeq18P76QRQueryClassContract(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p76","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "qr_query_class_contract")
	if s["version"] != "seq18_p76.v1" {
		t.Fatalf("version=%v, want seq18_p76.v1", s["version"])
	}
	if s["role"] != "qr_query_class_contract" {
		t.Fatalf("role=%v, want qr_query_class_contract", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["contract_version"] != "qr1a.v1" {
		t.Fatalf("contract_version=%v, want qr1a.v1", s["contract_version"])
	}
	if s["execution_mode"] != "single_query_shared" {
		t.Fatalf("execution_mode=%v, want single_query_shared", s["execution_mode"])
	}
	if s["fail_open"] != true {
		t.Fatalf("fail_open=%v, want true", s["fail_open"])
	}
	if s["additive_only"] != true {
		t.Fatalf("additive_only=%v, want true", s["additive_only"])
	}
	if s["policy_version"] != "qr1a.v1" {
		t.Fatalf("policy_version=%v, want qr1a.v1", s["policy_version"])
	}
	if s["mode"] != "qr_query_class_contract_surface" {
		t.Fatalf("mode=%v, want qr_query_class_contract_surface", s["mode"])
	}
}

// TestSeq18P77QRQueryClassTaxonomy validates the QR query-class taxonomy surface.
func TestSeq18P77QRQueryClassTaxonomy(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p77","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "qr_query_class_taxonomy")
	if s["version"] != "seq18_p77.v1" {
		t.Fatalf("version=%v, want seq18_p77.v1", s["version"])
	}
	if s["role"] != "qr_query_class_taxonomy" {
		t.Fatalf("role=%v, want qr_query_class_taxonomy", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	classes, ok := s["query_classes"].([]any)
	if !ok || len(classes) != 5 {
		t.Fatalf("query_classes=%v, want 5 items", s["query_classes"])
	}
	if s["contract_layer_only"] != true {
		t.Fatalf("contract_layer_only=%v, want true", s["contract_layer_only"])
	}
	if s["primary_class_visible"] != true {
		t.Fatalf("primary_class_visible=%v, want true", s["primary_class_visible"])
	}
	if s["policy_version"] != "qr1a.v1" {
		t.Fatalf("policy_version=%v, want qr1a.v1", s["policy_version"])
	}
	if s["mode"] != "qr_query_class_taxonomy_surface" {
		t.Fatalf("mode=%v, want qr_query_class_taxonomy_surface", s["mode"])
	}
}

// TestSeq18P78QRPrimaryClassSelection validates the QR primary class selection precedence surface.
func TestSeq18P78QRPrimaryClassSelection(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p78","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "qr_primary_class_selection")
	if s["version"] != "seq18_p78.v1" {
		t.Fatalf("version=%v, want seq18_p78.v1", s["version"])
	}
	if s["role"] != "qr_primary_class_selection" {
		t.Fatalf("role=%v, want qr_primary_class_selection", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	prec, ok := s["precedence"].([]any)
	if !ok || len(prec) != 5 {
		t.Fatalf("precedence=%v, want 5 items", s["precedence"])
	}
	if prec[0] != "explicit_temporal_cue" {
		t.Fatalf("precedence[0]=%v, want explicit_temporal_cue", prec[0])
	}
	if prec[4] != "scene_fallback" {
		t.Fatalf("precedence[4]=%v, want scene_fallback", prec[4])
	}
	if s["policy_version"] != "qr1a.v1" {
		t.Fatalf("policy_version=%v, want qr1a.v1", s["policy_version"])
	}
	if s["mode"] != "qr_primary_class_selection_surface" {
		t.Fatalf("mode=%v, want qr_primary_class_selection_surface", s["mode"])
	}
}

// TestSeq18P79QRLexicalCueBlock validates the QR lexical cue block surface.
func TestSeq18P79QRLexicalCueBlock(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p79","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "qr_lexical_cue_block")
	if s["version"] != "seq18_p79.v1" {
		t.Fatalf("version=%v, want seq18_p79.v1", s["version"])
	}
	if s["role"] != "qr_lexical_cue_block" {
		t.Fatalf("role=%v, want qr_lexical_cue_block", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["localized"] != true {
		t.Fatalf("localized=%v, want true", s["localized"])
	}
	if s["hidden_literals_removed"] != true {
		t.Fatalf("hidden_literals_removed=%v, want true", s["hidden_literals_removed"])
	}
	if s["policy_version"] != "qr1a.v1" {
		t.Fatalf("policy_version=%v, want qr1a.v1", s["policy_version"])
	}
	if s["mode"] != "qr_lexical_cue_block_surface" {
		t.Fatalf("mode=%v, want qr_lexical_cue_block_surface", s["mode"])
	}
}

// TestSeq18P80QRQueryClassContractTest validates the QR query-class contract test surface.
func TestSeq18P80QRQueryClassContractTest(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p80","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "qr_query_class_contract_test")
	if s["version"] != "seq18_p80.v1" {
		t.Fatalf("version=%v, want seq18_p80.v1", s["version"])
	}
	if s["role"] != "qr_query_class_contract_test" {
		t.Fatalf("role=%v, want qr_query_class_contract_test", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["test_file"] != "backend/test_step18_query_class_contract.py" {
		t.Fatalf("test_file=%v, want backend/test_step18_query_class_contract.py", s["test_file"])
	}
	covers, ok := s["covers"].([]any)
	if !ok || len(covers) != 3 {
		t.Fatalf("covers=%v, want 3 items", s["covers"])
	}
	if s["policy_version"] != "qr1a.v1" {
		t.Fatalf("policy_version=%v, want qr1a.v1", s["policy_version"])
	}
	if s["mode"] != "qr_query_class_contract_test_surface" {
		t.Fatalf("mode=%v, want qr_query_class_contract_test_surface", s["mode"])
	}
}

// TestSeq18P87QRQueryClassBudgetPolicy validates the QR query-class budget policy surface.
func TestSeq18P87QRQueryClassBudgetPolicy(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p87","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "qr_query_class_budget_policy")
	if s["version"] != "seq18_p87.v1" {
		t.Fatalf("version=%v, want seq18_p87.v1", s["version"])
	}
	if s["role"] != "qr_query_class_budget_policy" {
		t.Fatalf("role=%v, want qr_query_class_budget_policy", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["budget_policy_version"] != "qr1b.v1" {
		t.Fatalf("budget_policy_version=%v, want qr1b.v1", s["budget_policy_version"])
	}
	if s["execution_mode"] != "single_query_shared" {
		t.Fatalf("execution_mode=%v, want single_query_shared", s["execution_mode"])
	}
	if s["fail_open"] != true {
		t.Fatalf("fail_open=%v, want true", s["fail_open"])
	}
	if s["policy_version"] != "qr1b.v1" {
		t.Fatalf("policy_version=%v, want qr1b.v1", s["policy_version"])
	}
	if s["mode"] != "qr_query_class_budget_policy_surface" {
		t.Fatalf("mode=%v, want qr_query_class_budget_policy_surface", s["mode"])
	}
}

// TestSeq18P88QRQ3cBudgetReuse validates the QR q3c budget reuse surface.
func TestSeq18P88QRQ3cBudgetReuse(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p88","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "qr_q3c_budget_reuse")
	if s["version"] != "seq18_p88.v1" {
		t.Fatalf("version=%v, want seq18_p88.v1", s["version"])
	}
	if s["role"] != "qr_q3c_budget_reuse" {
		t.Fatalf("role=%v, want qr_q3c_budget_reuse", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["reused_budget_source"] != "q3c_intent_packet" {
		t.Fatalf("reused_budget_source=%v, want q3c_intent_packet", s["reused_budget_source"])
	}
	execClasses, ok := s["executable_classes"].([]any)
	if !ok || len(execClasses) != 4 {
		t.Fatalf("executable_classes=%v, want 4 items", s["executable_classes"])
	}
	if s["independent_budget_avoided"] != true {
		t.Fatalf("independent_budget_avoided=%v, want true", s["independent_budget_avoided"])
	}
	if s["policy_version"] != "qr1b.v1" {
		t.Fatalf("policy_version=%v, want qr1b.v1", s["policy_version"])
	}
	if s["mode"] != "qr_q3c_budget_reuse_surface" {
		t.Fatalf("mode=%v, want qr_q3c_budget_reuse_surface", s["mode"])
	}
}

// TestSeq18P89QRTemporalProfileBudget validates the QR temporal profile-based budget surface.
func TestSeq18P89QRTemporalProfileBudget(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p89","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "qr_temporal_profile_budget")
	if s["version"] != "seq18_p89.v1" {
		t.Fatalf("version=%v, want seq18_p89.v1", s["version"])
	}
	if s["role"] != "qr_temporal_profile_budget" {
		t.Fatalf("role=%v, want qr_temporal_profile_budget", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["profile_based"] != true {
		t.Fatalf("profile_based=%v, want true", s["profile_based"])
	}
	if s["evidence_first"] != true {
		t.Fatalf("evidence_first=%v, want true", s["evidence_first"])
	}
	if s["overlay_budget"] != true {
		t.Fatalf("overlay_budget=%v, want true", s["overlay_budget"])
	}
	caps, ok := s["candidate_caps"].([]any)
	if !ok || len(caps) != 3 {
		t.Fatalf("candidate_caps=%v, want 3 items", s["candidate_caps"])
	}
	if s["shared_profile_template"] != true {
		t.Fatalf("shared_profile_template=%v, want true", s["shared_profile_template"])
	}
	if s["policy_version"] != "qr1b.v1" {
		t.Fatalf("policy_version=%v, want qr1b.v1", s["policy_version"])
	}
	if s["mode"] != "qr_temporal_profile_budget_surface" {
		t.Fatalf("mode=%v, want qr_temporal_profile_budget_surface", s["mode"])
	}
}

// TestSeq18P90QRBudgetVisibility validates the QR budget visibility surface.
func TestSeq18P90QRBudgetVisibility(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p90","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "qr_budget_visibility")
	if s["version"] != "seq18_p90.v1" {
		t.Fatalf("version=%v, want seq18_p90.v1", s["version"])
	}
	if s["role"] != "qr_budget_visibility" {
		t.Fatalf("role=%v, want qr_budget_visibility", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	visibleFields, ok := s["visible_fields"].([]any)
	if !ok || len(visibleFields) != 4 {
		t.Fatalf("visible_fields=%v, want 4 items", s["visible_fields"])
	}
	if s["contract_data_visible"] != true {
		t.Fatalf("contract_data_visible=%v, want true", s["contract_data_visible"])
	}
	if s["hidden_builder_logic_removed"] != true {
		t.Fatalf("hidden_builder_logic_removed=%v, want true", s["hidden_builder_logic_removed"])
	}
	if s["policy_version"] != "qr1b.v1" {
		t.Fatalf("policy_version=%v, want qr1b.v1", s["policy_version"])
	}
	if s["mode"] != "qr_budget_visibility_surface" {
		t.Fatalf("mode=%v, want qr_budget_visibility_surface", s["mode"])
	}
}

// TestSeq18P91QRQueryClassBudgetTest validates the QR query-class budget test surface.
func TestSeq18P91QRQueryClassBudgetTest(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p91","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "qr_query_class_budget_test")
	if s["version"] != "seq18_p91.v1" {
		t.Fatalf("version=%v, want seq18_p91.v1", s["version"])
	}
	if s["role"] != "qr_query_class_budget_test" {
		t.Fatalf("role=%v, want qr_query_class_budget_test", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["test_file"] != "backend/test_step18_query_class_budget_policy.py" {
		t.Fatalf("test_file=%v, want backend/test_step18_query_class_budget_policy.py", s["test_file"])
	}
	covers, ok := s["covers"].([]any)
	if !ok || len(covers) != 2 {
		t.Fatalf("covers=%v, want 2 items", s["covers"])
	}
	if s["policy_version"] != "qr1b.v1" {
		t.Fatalf("policy_version=%v, want qr1b.v1", s["policy_version"])
	}
	if s["mode"] != "qr_query_class_budget_test_surface" {
		t.Fatalf("mode=%v, want qr_query_class_budget_test_surface", s["mode"])
	}
}

// TestSeq18P98QRNotePolicy validates the QR note policy surface.
func TestSeq18P98QRNotePolicy(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p98","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "qr_note_policy")
	if s["version"] != "seq18_p98.v1" {
		t.Fatalf("version=%v, want seq18_p98.v1", s["version"])
	}
	if s["role"] != "qr_note_policy" {
		t.Fatalf("role=%v, want qr_note_policy", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["note_policy_version"] != "qr1c.v1" {
		t.Fatalf("note_policy_version=%v, want qr1c.v1", s["note_policy_version"])
	}
	if s["execution_mode"] != "single_query_shared" {
		t.Fatalf("execution_mode=%v, want single_query_shared", s["execution_mode"])
	}
	if s["fail_open"] != true {
		t.Fatalf("fail_open=%v, want true", s["fail_open"])
	}
	if s["policy_version"] != "qr1c.v1" {
		t.Fatalf("policy_version=%v, want qr1c.v1", s["policy_version"])
	}
	if s["mode"] != "qr_note_policy_surface" {
		t.Fatalf("mode=%v, want qr_note_policy_surface", s["mode"])
	}
}

// TestSeq18P99QRSceneCanonNoPreExtract validates the QR scene/canon no-pre-extract surface.
func TestSeq18P99QRSceneCanonNoPreExtract(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p99","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "qr_scene_canon_no_pre_extract")
	if s["version"] != "seq18_p99.v1" {
		t.Fatalf("version=%v, want seq18_p99.v1", s["version"])
	}
	if s["role"] != "qr_scene_canon_no_pre_extract" {
		t.Fatalf("role=%v, want qr_scene_canon_no_pre_extract", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	noPreClasses, ok := s["no_pre_extract_classes"].([]any)
	if !ok || len(noPreClasses) != 2 {
		t.Fatalf("no_pre_extract_classes=%v, want 2 items", s["no_pre_extract_classes"])
	}
	noteSurfaces, ok := s["note_surfaces"].([]any)
	if !ok || len(noteSurfaces) != 2 {
		t.Fatalf("note_surfaces=%v, want 2 items", s["note_surfaces"])
	}
	if s["delivery_policy"] != "support_surface_first" {
		t.Fatalf("delivery_policy=%v, want support_surface_first", s["delivery_policy"])
	}
	if s["policy_version"] != "qr1c.v1" {
		t.Fatalf("policy_version=%v, want qr1c.v1", s["policy_version"])
	}
	if s["mode"] != "qr_scene_canon_no_pre_extract_surface" {
		t.Fatalf("mode=%v, want qr_scene_canon_no_pre_extract_surface", s["mode"])
	}
}

// TestSeq18P100QRCallbackResumeTemporalNoteOnly validates the QR callback/resume/temporal note-only surface.
func TestSeq18P100QRCallbackResumeTemporalNoteOnly(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p100","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "qr_callback_resume_temporal_note_only")
	if s["version"] != "seq18_p100.v1" {
		t.Fatalf("version=%v, want seq18_p100.v1", s["version"])
	}
	if s["role"] != "qr_callback_resume_temporal_note_only" {
		t.Fatalf("role=%v, want qr_callback_resume_temporal_note_only", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	noteOnlyClasses, ok := s["note_only_classes"].([]any)
	if !ok || len(noteOnlyClasses) != 3 {
		t.Fatalf("note_only_classes=%v, want 3 items", s["note_only_classes"])
	}
	if s["pre_extract_behavior"] != "note_only_until_route_exec" {
		t.Fatalf("pre_extract_behavior=%v, want note_only_until_route_exec", s["pre_extract_behavior"])
	}
	if s["contract_layer_only"] != true {
		t.Fatalf("contract_layer_only=%v, want true", s["contract_layer_only"])
	}
	if s["policy_version"] != "qr1c.v1" {
		t.Fatalf("policy_version=%v, want qr1c.v1", s["policy_version"])
	}
	if s["mode"] != "qr_callback_resume_temporal_note_only_surface" {
		t.Fatalf("mode=%v, want qr_callback_resume_temporal_note_only_surface", s["mode"])
	}
}

// TestSeq18P101QRNotePolicyFields validates the QR note policy fields surface.
func TestSeq18P101QRNotePolicyFields(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p101","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "qr_note_policy_fields")
	if s["version"] != "seq18_p101.v1" {
		t.Fatalf("version=%v, want seq18_p101.v1", s["version"])
	}
	if s["role"] != "qr_note_policy_fields" {
		t.Fatalf("role=%v, want qr_note_policy_fields", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	visibleFields, ok := s["visible_fields"].([]any)
	if !ok || len(visibleFields) != 5 {
		t.Fatalf("visible_fields=%v, want 5 items", s["visible_fields"])
	}
	if s["additive_metadata"] != true {
		t.Fatalf("additive_metadata=%v, want true", s["additive_metadata"])
	}
	if s["policy_version"] != "qr1c.v1" {
		t.Fatalf("policy_version=%v, want qr1c.v1", s["policy_version"])
	}
	if s["mode"] != "qr_note_policy_fields_surface" {
		t.Fatalf("mode=%v, want qr_note_policy_fields_surface", s["mode"])
	}
}

// TestSeq18P102QRNotePolicyTest validates the QR note policy test surface.
func TestSeq18P102QRNotePolicyTest(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p102","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "qr_note_policy_test")
	if s["version"] != "seq18_p102.v1" {
		t.Fatalf("version=%v, want seq18_p102.v1", s["version"])
	}
	if s["role"] != "qr_note_policy_test" {
		t.Fatalf("role=%v, want qr_note_policy_test", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["test_file"] != "backend/test_step18_query_class_note_policy.py" {
		t.Fatalf("test_file=%v, want backend/test_step18_query_class_note_policy.py", s["test_file"])
	}
	guards, ok := s["guards"].([]any)
	if !ok || len(guards) != 2 {
		t.Fatalf("guards=%v, want 2 items", s["guards"])
	}
	if s["policy_version"] != "qr1c.v1" {
		t.Fatalf("policy_version=%v, want qr1c.v1", s["policy_version"])
	}
	if s["mode"] != "qr_note_policy_test_surface" {
		t.Fatalf("mode=%v, want qr_note_policy_test_surface", s["mode"])
	}
}

// TestSeq18P109QRRoutePolicy validates the QR route policy surface.
func TestSeq18P109QRRoutePolicy(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p109","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "qr_route_policy")
	if s["version"] != "seq18_p109.v1" {
		t.Fatalf("version=%v, want seq18_p109.v1", s["version"])
	}
	if s["role"] != "qr_route_policy" {
		t.Fatalf("role=%v, want qr_route_policy", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["route_policy_version"] != "qr1d.v1" {
		t.Fatalf("route_policy_version=%v, want qr1d.v1", s["route_policy_version"])
	}
	if s["execution_mode"] != "single_query_shared" {
		t.Fatalf("execution_mode=%v, want single_query_shared", s["execution_mode"])
	}
	if s["fail_open"] != true {
		t.Fatalf("fail_open=%v, want true", s["fail_open"])
	}
	if s["policy_version"] != "qr1d.v1" {
		t.Fatalf("policy_version=%v, want qr1d.v1", s["policy_version"])
	}
	if s["mode"] != "qr_route_policy_surface" {
		t.Fatalf("mode=%v, want qr_route_policy_surface", s["mode"])
	}
}

// TestSeq18P110QRRouteFamilies validates the QR route families surface.
func TestSeq18P110QRRouteFamilies(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p110","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "qr_route_families")
	if s["version"] != "seq18_p110.v1" {
		t.Fatalf("version=%v, want seq18_p110.v1", s["version"])
	}
	if s["role"] != "qr_route_families" {
		t.Fatalf("role=%v, want qr_route_families", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	families, ok := s["route_families"].([]any)
	if !ok || len(families) != 7 {
		t.Fatalf("route_families=%v, want 7 items", s["route_families"])
	}
	if s["metadata_visible"] != true {
		t.Fatalf("metadata_visible=%v, want true", s["metadata_visible"])
	}
	if s["policy_version"] != "qr1d.v1" {
		t.Fatalf("policy_version=%v, want qr1d.v1", s["policy_version"])
	}
	if s["mode"] != "qr_route_families_surface" {
		t.Fatalf("mode=%v, want qr_route_families_surface", s["mode"])
	}
}

// TestSeq18P111QRLongTailRouteCandidates validates the QR long-tail route candidates surface.
func TestSeq18P111QRLongTailRouteCandidates(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p111","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "qr_long_tail_route_candidates")
	if s["version"] != "seq18_p111.v1" {
		t.Fatalf("version=%v, want seq18_p111.v1", s["version"])
	}
	if s["role"] != "qr_long_tail_route_candidates" {
		t.Fatalf("role=%v, want qr_long_tail_route_candidates", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	ltClasses, ok := s["long_tail_classes"].([]any)
	if !ok || len(ltClasses) != 3 {
		t.Fatalf("long_tail_classes=%v, want 3 items", s["long_tail_classes"])
	}
	if s["promotion_trigger"] != "detail_old_detail_lexical_cue" {
		t.Fatalf("promotion_trigger=%v, want detail_old_detail_lexical_cue", s["promotion_trigger"])
	}
	if s["contract_layer_only"] != true {
		t.Fatalf("contract_layer_only=%v, want true", s["contract_layer_only"])
	}
	if s["runtime_unchanged"] != true {
		t.Fatalf("runtime_unchanged=%v, want true", s["runtime_unchanged"])
	}
	if s["policy_version"] != "qr1d.v1" {
		t.Fatalf("policy_version=%v, want qr1d.v1", s["policy_version"])
	}
	if s["mode"] != "qr_long_tail_route_candidates_surface" {
		t.Fatalf("mode=%v, want qr_long_tail_route_candidates_surface", s["mode"])
	}
}

// TestSeq18P112QRRoutePolicyFields validates the QR route policy fields surface.
func TestSeq18P112QRRoutePolicyFields(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p112","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "qr_route_policy_fields")
	if s["version"] != "seq18_p112.v1" {
		t.Fatalf("version=%v, want seq18_p112.v1", s["version"])
	}
	if s["role"] != "qr_route_policy_fields" {
		t.Fatalf("role=%v, want qr_route_policy_fields", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	visibleFields, ok := s["visible_fields"].([]any)
	if !ok || len(visibleFields) != 4 {
		t.Fatalf("visible_fields=%v, want 4 items", s["visible_fields"])
	}
	if s["publishes"] != "primary_selected_route" {
		t.Fatalf("publishes=%v, want primary_selected_route", s["publishes"])
	}
	if s["runtime_unchanged"] != true {
		t.Fatalf("runtime_unchanged=%v, want true", s["runtime_unchanged"])
	}
	if s["policy_version"] != "qr1d.v1" {
		t.Fatalf("policy_version=%v, want qr1d.v1", s["policy_version"])
	}
	if s["mode"] != "qr_route_policy_fields_surface" {
		t.Fatalf("mode=%v, want qr_route_policy_fields_surface", s["mode"])
	}
}

// TestSeq18P113QRRoutePolicyTest validates the QR route policy test surface.
func TestSeq18P113QRRoutePolicyTest(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p113","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "qr_route_policy_test")
	if s["version"] != "seq18_p113.v1" {
		t.Fatalf("version=%v, want seq18_p113.v1", s["version"])
	}
	if s["role"] != "qr_route_policy_test" {
		t.Fatalf("role=%v, want qr_route_policy_test", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["test_file"] != "backend/test_step18_query_class_route_policy.py" {
		t.Fatalf("test_file=%v, want backend/test_step18_query_class_route_policy.py", s["test_file"])
	}
	guards, ok := s["guards"].([]any)
	if !ok || len(guards) != 2 {
		t.Fatalf("guards=%v, want 2 items", s["guards"])
	}
	if s["policy_version"] != "qr1d.v1" {
		t.Fatalf("policy_version=%v, want qr1d.v1", s["policy_version"])
	}
	if s["mode"] != "qr_route_policy_test_surface" {
		t.Fatalf("mode=%v, want qr_route_policy_test_surface", s["mode"])
	}
}

// TestSeq18P120VXHybridReplayGate validates the VX hybrid replay gate surface.
func TestSeq18P120VXHybridReplayGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p120","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "vx_hybrid_replay_gate")
	if s["version"] != "seq18_p120.v1" {
		t.Fatalf("version=%v, want seq18_p120.v1", s["version"])
	}
	if s["role"] != "vx_hybrid_replay_gate" {
		t.Fatalf("role=%v, want vx_hybrid_replay_gate", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["gate_version"] != "vx18a.v1" {
		t.Fatalf("gate_version=%v, want vx18a.v1", s["gate_version"])
	}
	if s["gate_name"] != "hybrid_replay" {
		t.Fatalf("gate_name=%v, want hybrid_replay", s["gate_name"])
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
	if s["policy_version"] != "vx18a.v1" {
		t.Fatalf("policy_version=%v, want vx18a.v1", s["policy_version"])
	}
	if s["mode"] != "vx_hybrid_replay_gate_surface" {
		t.Fatalf("mode=%v, want vx_hybrid_replay_gate_surface", s["mode"])
	}
}
