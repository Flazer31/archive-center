package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func seq185RequireString(t *testing.T, surface map[string]any, key, want string) {
	t.Helper()
	if surface[key] != want {
		t.Fatalf("%s=%v, want %s", key, surface[key], want)
	}
}

func seq185RequireStringSliceContains(t *testing.T, surface map[string]any, key, want string) {
	t.Helper()
	values, ok := surface[key].([]any)
	if !ok {
		t.Fatalf("%s=%T, want []any containing %q", key, surface[key], want)
	}
	for _, value := range values {
		if value == want {
			return
		}
	}
	t.Fatalf("%s=%v, want item %q", key, values, want)
}

// ---------------------------------------------------------------------------
// SEQ-18.5 reset administration tests (P9 ~ P11)
// ---------------------------------------------------------------------------

func TestSeq185P9ResetAdmin(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p9","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "reset_admin_185")
	if s["version"] != "seq185_p9.v1" {
		t.Fatalf("version=%v, want seq185_p9.v1", s["version"])
	}
	if s["role"] != "reset_administration" {
		t.Fatalf("role=%v, want reset_administration", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["reset_action"] != "checklist_cleared_for_redo" {
		t.Fatalf("reset_action=%v, want checklist_cleared_for_redo", s["reset_action"])
	}
	if s["historical_preserved"] != true {
		t.Fatalf("historical_preserved=%v, want true", s["historical_preserved"])
	}
	if s["mode"] != "reset_administration_note" {
		t.Fatalf("mode=%v, want reset_administration_note", s["mode"])
	}
}

func TestSeq185P10HistoricalContentPreserved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p10","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "historical_content_preserved_185")
	if s["version"] != "seq185_p10.v1" {
		t.Fatalf("version=%v, want seq185_p10.v1", s["version"])
	}
	if s["role"] != "historical_content_preserved" {
		t.Fatalf("role=%v, want historical_content_preserved", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["content_preserved"] != true {
		t.Fatalf("content_preserved=%v, want true", s["content_preserved"])
	}
	if s["no_text_deleted"] != true {
		t.Fatalf("no_text_deleted=%v, want true", s["no_text_deleted"])
	}
	if s["mode"] != "historical_content_preservation_note" {
		t.Fatalf("mode=%v, want historical_content_preservation_note", s["mode"])
	}
}

func TestSeq185P11ResetNoteOnly(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p11","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "reset_note_only_185")
	if s["version"] != "seq185_p11.v1" {
		t.Fatalf("version=%v, want seq185_p11.v1", s["version"])
	}
	if s["role"] != "reset_note_only" {
		t.Fatalf("role=%v, want reset_note_only", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["scope"] != "document_reset_only" {
		t.Fatalf("scope=%v, want document_reset_only", s["scope"])
	}
	if s["revalidation_claim"] != false {
		t.Fatalf("revalidation_claim=%v, want false", s["revalidation_claim"])
	}
	if s["mode"] != "reset_scope_note" {
		t.Fatalf("mode=%v, want reset_scope_note", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-18.5 six criteria tests (P163 ~ P168)
// ---------------------------------------------------------------------------

func TestSeq185P163BoundedLiveScope(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p163","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "bounded_live_scope")
	if s["version"] != "seq185_p163.v1" {
		t.Fatalf("version=%v, want seq185_p163.v1", s["version"])
	}
	if s["role"] != "bounded_live_scope" {
		t.Fatalf("role=%v, want bounded_live_scope", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	seq185RequireString(t, s, "canonical_truth_authority", "mariadb")
	seq185RequireString(t, s, "vector_accelerator", "chromadb")
	if s["scope"] != "limited_memory" {
		t.Fatalf("scope=%v, want limited_memory", s["scope"])
	}
	if s["tier"] != "memory_only" {
		t.Fatalf("tier=%v, want memory_only", s["tier"])
	}
	if s["mode"] != "bounded_live_scope_definition" {
		t.Fatalf("mode=%v, want bounded_live_scope_definition", s["mode"])
	}
}

func TestSeq185P164SQLiteTruthPreserve(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p164","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "sqlite_truth_preserve")
	if s["version"] != "seq185_p164.v1" {
		t.Fatalf("version=%v, want seq185_p164.v1", s["version"])
	}
	if s["role"] != "sqlite_truth_preserve" {
		t.Fatalf("role=%v, want sqlite_truth_preserve", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	seq185RequireString(t, s, "source_truth_label", "sqlite")
	seq185RequireString(t, s, "canonical_truth_authority", "mariadb")
	seq185RequireString(t, s, "vector_accelerator", "chromadb")
	seq185RequireString(t, s, "canonical_hydration_target", "mariadb_row")
	seq185RequireString(t, s, "remigration_translation", "sqlite_source_contract_to_mariadb_canonical_truth")
	if s["hydration_required"] != true {
		t.Fatalf("hydration_required=%v, want true", s["hydration_required"])
	}
	if s["final_authority"] != "sqlite" {
		t.Fatalf("final_authority=%v, want sqlite", s["final_authority"])
	}
	if s["mode"] != "sqlite_truth_preservation_rule" {
		t.Fatalf("mode=%v, want sqlite_truth_preservation_rule", s["mode"])
	}
}

func TestSeq185P165FailOpenSafety(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p165","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "fail_open_safety")
	if s["version"] != "seq185_p165.v1" {
		t.Fatalf("version=%v, want seq185_p165.v1", s["version"])
	}
	if s["role"] != "fail_open_safety" {
		t.Fatalf("role=%v, want fail_open_safety", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	seq185RequireString(t, s, "canonical_truth_authority", "mariadb")
	if s["fallback_lane"] != "sqlite_scan_preserved" {
		t.Fatalf("fallback_lane=%v, want sqlite_scan_preserved", s["fallback_lane"])
	}
	seq185RequireString(t, s, "fallback_lane_current", "mariadb_scan_preserved")
	if s["mode"] != "fail_open_safety_rule" {
		t.Fatalf("mode=%v, want fail_open_safety_rule", s["mode"])
	}
}

func TestSeq185P166OperatorVisibility(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p166","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "operator_visibility")
	if s["version"] != "seq185_p166.v1" {
		t.Fatalf("version=%v, want seq185_p166.v1", s["version"])
	}
	if s["role"] != "operator_visibility" {
		t.Fatalf("role=%v, want operator_visibility", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	seq185RequireStringSliceContains(t, s, "state_vocabulary", "mariadb_fallback")
	if s["mode"] != "operator_visibility_surface" {
		t.Fatalf("mode=%v, want operator_visibility_surface", s["mode"])
	}
}

func TestSeq185P167SilentAuthorityDriftGuard(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p167","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "silent_authority_drift_guard")
	if s["version"] != "seq185_p167.v1" {
		t.Fatalf("version=%v, want seq185_p167.v1", s["version"])
	}
	if s["role"] != "silent_authority_drift_guard" {
		t.Fatalf("role=%v, want silent_authority_drift_guard", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["authority_drift_block"] != true {
		t.Fatalf("authority_drift_block=%v, want true", s["authority_drift_block"])
	}
	if s["mode"] != "silent_authority_drift_guard_rule" {
		t.Fatalf("mode=%v, want silent_authority_drift_guard_rule", s["mode"])
	}
}

func TestSeq185P168ReleaseHonesty(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p168","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "release_honesty")
	if s["version"] != "seq185_p168.v1" {
		t.Fatalf("version=%v, want seq185_p168.v1", s["version"])
	}
	if s["role"] != "release_honesty" {
		t.Fatalf("role=%v, want release_honesty", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["default_mode"] != "off" {
		t.Fatalf("default_mode=%v, want off", s["default_mode"])
	}
	if s["step_21_promotion"] != false {
		t.Fatalf("step_21_promotion=%v, want false", s["step_21_promotion"])
	}
	if s["operator_gated"] != true {
		t.Fatalf("operator_gated=%v, want true", s["operator_gated"])
	}
	if s["mode"] != "release_honesty_note" {
		t.Fatalf("mode=%v, want release_honesty_note", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-18.5 sub-step 18.5-1 LR tests (P172 ~ P175)
// ---------------------------------------------------------------------------

func TestSeq185P172LiveChromaToggleConfig(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p172","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "live_chroma_toggle_config")
	if s["version"] != "seq185_p172.v1" {
		t.Fatalf("version=%v, want seq185_p172.v1", s["version"])
	}
	if s["role"] != "live_chroma_toggle_config" {
		t.Fatalf("role=%v, want live_chroma_toggle_config", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["config_surface"] != "chroma_live_cutover_mode" {
		t.Fatalf("config_surface=%v, want chroma_live_cutover_mode", s["config_surface"])
	}
	if s["mode"] != "live_chroma_toggle_config_definition" {
		t.Fatalf("mode=%v, want live_chroma_toggle_config_definition", s["mode"])
	}
}

func TestSeq185P173LiveScopeMemoryOnly(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p173","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "live_scope_memory_only")
	if s["version"] != "seq185_p173.v1" {
		t.Fatalf("version=%v, want seq185_p173.v1", s["version"])
	}
	if s["role"] != "live_scope_memory_only" {
		t.Fatalf("role=%v, want live_scope_memory_only", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["scope"] != "memory_only" {
		t.Fatalf("scope=%v, want memory_only", s["scope"])
	}
	if s["mode_fixed"] != "limited_memory" {
		t.Fatalf("mode_fixed=%v, want limited_memory", s["mode_fixed"])
	}
	if s["mode"] != "live_scope_memory_only_definition" {
		t.Fatalf("mode=%v, want live_scope_memory_only_definition", s["mode"])
	}
}

func TestSeq185P174LiveChromaTopkCap(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p174","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "live_chroma_topk_cap")
	if s["version"] != "seq185_p174.v1" {
		t.Fatalf("version=%v, want seq185_p174.v1", s["version"])
	}
	if s["role"] != "live_chroma_topk_cap" {
		t.Fatalf("role=%v, want live_chroma_topk_cap", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["top_k_bounded"] != true {
		t.Fatalf("top_k_bounded=%v, want true", s["top_k_bounded"])
	}
	if s["candidate_cap"] != float64(1) {
		t.Fatalf("candidate_cap=%v, want 1", s["candidate_cap"])
	}
	if s["mode"] != "live_chroma_topk_cap_definition" {
		t.Fatalf("mode=%v, want live_chroma_topk_cap_definition", s["mode"])
	}
}

func TestSeq185P175ShadowDisabledDegradeRule(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p175","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "shadow_disabled_degrade_rule")
	if s["version"] != "seq185_p175.v1" {
		t.Fatalf("version=%v, want seq185_p175.v1", s["version"])
	}
	if s["role"] != "shadow_disabled_degrade_rule" {
		t.Fatalf("role=%v, want shadow_disabled_degrade_rule", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	seq185RequireString(t, s, "canonical_truth_authority", "mariadb")
	if s["fallback_path"] != "sqlite_only_scan" {
		t.Fatalf("fallback_path=%v, want sqlite_only_scan", s["fallback_path"])
	}
	seq185RequireString(t, s, "fallback_path_current", "mariadb_only_scan")
	if s["mode"] != "shadow_disabled_degrade_rule_definition" {
		t.Fatalf("mode=%v, want shadow_disabled_degrade_rule_definition", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-18.5 sub-step 18.5-2 HJ tests (P179 ~ P182)
// ---------------------------------------------------------------------------

func TestSeq185P179ChromaIdentitySQLiteHydration(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p179","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "chroma_identity_sqlite_hydration")
	if s["version"] != "seq185_p179.v1" {
		t.Fatalf("version=%v, want seq185_p179.v1", s["version"])
	}
	if s["role"] != "chroma_identity_sqlite_hydration" {
		t.Fatalf("role=%v, want chroma_identity_sqlite_hydration", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	seq185RequireString(t, s, "canonical_truth_authority", "mariadb")
	seq185RequireString(t, s, "canonical_hydration_target", "mariadb_row")
	if s["hydration_required"] != true {
		t.Fatalf("hydration_required=%v, want true", s["hydration_required"])
	}
	if s["mode"] != "chroma_identity_sqlite_hydration_definition" {
		t.Fatalf("mode=%v, want chroma_identity_sqlite_hydration_definition", s["mode"])
	}
}

func TestSeq185P180ChromaSQLiteDedupeMerge(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p180","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "chroma_sqlite_dedupe_merge")
	if s["version"] != "seq185_p180.v1" {
		t.Fatalf("version=%v, want seq185_p180.v1", s["version"])
	}
	if s["role"] != "chroma_sqlite_dedupe_merge" {
		t.Fatalf("role=%v, want chroma_sqlite_dedupe_merge", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	seq185RequireString(t, s, "canonical_baseline_authority", "mariadb")
	if s["dedupe_key"] != "row_id_uniqueness" {
		t.Fatalf("dedupe_key=%v, want row_id_uniqueness", s["dedupe_key"])
	}
	if s["merge_limit"] != float64(1) {
		t.Fatalf("merge_limit=%v, want 1", s["merge_limit"])
	}
	if s["mode"] != "chroma_sqlite_dedupe_merge_definition" {
		t.Fatalf("mode=%v, want chroma_sqlite_dedupe_merge_definition", s["mode"])
	}
}

func TestSeq185P181CanonicalPrecedenceFormatting(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p181","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "canonical_precedence_formatting")
	if s["version"] != "seq185_p181.v1" {
		t.Fatalf("version=%v, want seq185_p181.v1", s["version"])
	}
	if s["role"] != "canonical_precedence_formatting" {
		t.Fatalf("role=%v, want canonical_precedence_formatting", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["formatting_stack"] != "existing_storyteller_format_for_injection" {
		t.Fatalf("formatting_stack=%v, want existing_storyteller_format_for_injection", s["formatting_stack"])
	}
	if s["mode"] != "canonical_precedence_formatting_definition" {
		t.Fatalf("mode=%v, want canonical_precedence_formatting_definition", s["mode"])
	}
}

func TestSeq185P182ChromaMissFallbackPreserve(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p182","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "chroma_miss_fallback_preserve")
	if s["version"] != "seq185_p182.v1" {
		t.Fatalf("version=%v, want seq185_p182.v1", s["version"])
	}
	if s["role"] != "chroma_miss_fallback_preserve" {
		t.Fatalf("role=%v, want chroma_miss_fallback_preserve", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["fallback_source"] != "chat_log" {
		t.Fatalf("fallback_source=%v, want chat_log", s["fallback_source"])
	}
	if s["fallback_source_table"] != "chat_logs" {
		t.Fatalf("fallback_source_table=%v, want chat_logs", s["fallback_source_table"])
	}
	if s["mode"] != "chroma_miss_fallback_preserve_definition" {
		t.Fatalf("mode=%v, want chroma_miss_fallback_preserve_definition", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-18.5 sub-step 18.5-3 SG tests (P186 ~ P189)
// ---------------------------------------------------------------------------

func TestSeq185P186OperatorInspectionSurface(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p186","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "operator_inspection_surface")
	if s["version"] != "seq185_p186.v1" {
		t.Fatalf("version=%v, want seq185_p186.v1", s["version"])
	}
	if s["role"] != "operator_inspection_surface" {
		t.Fatalf("role=%v, want operator_inspection_surface", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	seq185RequireStringSliceContains(t, s, "inspection_states", "mariadb_fallback")
	if s["mode"] != "operator_inspection_surface_definition" {
		t.Fatalf("mode=%v, want operator_inspection_surface_definition", s["mode"])
	}
}

func TestSeq185P187LiveLimitedModeToggle(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p187","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "live_limited_mode_toggle")
	if s["version"] != "seq185_p187.v1" {
		t.Fatalf("version=%v, want seq185_p187.v1", s["version"])
	}
	if s["role"] != "live_limited_mode_toggle" {
		t.Fatalf("role=%v, want live_limited_mode_toggle", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["sync_route"] != "/config/update" {
		t.Fatalf("sync_route=%v, want /config/update", s["sync_route"])
	}
	if s["mode"] != "live_limited_mode_toggle_definition" {
		t.Fatalf("mode=%v, want live_limited_mode_toggle_definition", s["mode"])
	}
}

func TestSeq185P188HealthAdoptionPrerequisite(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p188","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "health_adoption_prerequisite")
	if s["version"] != "seq185_p188.v1" {
		t.Fatalf("version=%v, want seq185_p188.v1", s["version"])
	}
	if s["role"] != "health_adoption_prerequisite" {
		t.Fatalf("role=%v, want health_adoption_prerequisite", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["read_only_link"] != true {
		t.Fatalf("read_only_link=%v, want true", s["read_only_link"])
	}
	if s["mode"] != "health_adoption_prerequisite_definition" {
		t.Fatalf("mode=%v, want health_adoption_prerequisite_definition", s["mode"])
	}
}

func TestSeq185P189NarrowRolloutRule(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p189","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "narrow_rollout_rule")
	if s["version"] != "seq185_p189.v1" {
		t.Fatalf("version=%v, want seq185_p189.v1", s["version"])
	}
	if s["role"] != "narrow_rollout_rule" {
		t.Fatalf("role=%v, want narrow_rollout_rule", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	seq185RequireStringSliceContains(t, s, "rollout_rules", "mariadb_truth_retained")
	seq185RequireStringSliceContains(t, s, "rollout_rules", "fail_open_mariadb_scan")
	if s["mode"] != "narrow_rollout_rule_definition" {
		t.Fatalf("mode=%v, want narrow_rollout_rule_definition", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-18.5 sub-step 18.5-4 VX tests (P193 ~ P197)
// ---------------------------------------------------------------------------

func TestSeq185P193ChromaEnabledSmokeCheck(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p193","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "chroma_enabled_smoke_check")
	if s["version"] != "seq185_p193.v1" {
		t.Fatalf("version=%v, want seq185_p193.v1", s["version"])
	}
	if s["role"] != "chroma_enabled_smoke_check" {
		t.Fatalf("role=%v, want chroma_enabled_smoke_check", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	seq185RequireStringSliceContains(t, s, "ready_conditions", "mariadb_truth_authority")
	if s["mode"] != "chroma_enabled_smoke_check_definition" {
		t.Fatalf("mode=%v, want chroma_enabled_smoke_check_definition", s["mode"])
	}
}

func TestSeq185P194DegradedFailOpenReplay(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p194","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "degraded_fail_open_replay")
	if s["version"] != "seq185_p194.v1" {
		t.Fatalf("version=%v, want seq185_p194.v1", s["version"])
	}
	if s["role"] != "degraded_fail_open_replay" {
		t.Fatalf("role=%v, want degraded_fail_open_replay", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["fallback_preserved"] != true {
		t.Fatalf("fallback_preserved=%v, want true", s["fallback_preserved"])
	}
	if s["mariadb_truth_authority_preserved"] != true {
		t.Fatalf("mariadb_truth_authority_preserved=%v, want true", s["mariadb_truth_authority_preserved"])
	}
	if s["mode"] != "degraded_fail_open_replay_definition" {
		t.Fatalf("mode=%v, want degraded_fail_open_replay_definition", s["mode"])
	}
}

func TestSeq185P195SQLiteBaselineParityReplay(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p195","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "sqlite_baseline_parity_replay")
	if s["version"] != "seq185_p195.v1" {
		t.Fatalf("version=%v, want seq185_p195.v1", s["version"])
	}
	if s["role"] != "sqlite_baseline_parity_replay" {
		t.Fatalf("role=%v, want sqlite_baseline_parity_replay", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	seq185RequireString(t, s, "current_baseline_authority", "mariadb")
	if s["parity_mode"] != "exact_or_bounded_one_slot_delta" {
		t.Fatalf("parity_mode=%v, want exact_or_bounded_one_slot_delta", s["parity_mode"])
	}
	if s["mode"] != "sqlite_baseline_parity_replay_definition" {
		t.Fatalf("mode=%v, want sqlite_baseline_parity_replay_definition", s["mode"])
	}
}

func TestSeq185P196TruthBoundarySourceOrderReplay(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p196","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "truth_boundary_source_order_replay")
	if s["version"] != "seq185_p196.v1" {
		t.Fatalf("version=%v, want seq185_p196.v1", s["version"])
	}
	if s["role"] != "truth_boundary_source_order_replay" {
		t.Fatalf("role=%v, want truth_boundary_source_order_replay", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	seq185RequireStringSliceContains(t, s, "boundary_rules", "mariadb_truth_support_only_precedence_ceiling")
	if s["mode"] != "truth_boundary_source_order_replay_definition" {
		t.Fatalf("mode=%v, want truth_boundary_source_order_replay_definition", s["mode"])
	}
}

func TestSeq185P197ReleaseNoteHonestyChecklist(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p197","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "release_note_honesty_checklist")
	if s["version"] != "seq185_p197.v1" {
		t.Fatalf("version=%v, want seq185_p197.v1", s["version"])
	}
	if s["role"] != "release_note_honesty_checklist" {
		t.Fatalf("role=%v, want release_note_honesty_checklist", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["mode"] != "release_note_honesty_checklist_definition" {
		t.Fatalf("mode=%v, want release_note_honesty_checklist_definition", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-18.5 release gate tests (P201 ~ P205)
// ---------------------------------------------------------------------------

func TestSeq185P201BundleReleaseGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p201","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "bundle_release_gate_201")
	if s["version"] != "seq185_p201.v1" {
		t.Fatalf("version=%v, want seq185_p201.v1", s["version"])
	}
	if s["role"] != "bundle_release_gate" {
		t.Fatalf("role=%v, want bundle_release_gate", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["artifact_created"] != false {
		t.Fatalf("artifact_created=%v, want false (dry-run only)", s["artifact_created"])
	}
	if s["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", s["dry_run_only"])
	}
	if s["operator_approval_required"] != true {
		t.Fatalf("operator_approval_required=%v, want true", s["operator_approval_required"])
	}
	seq185RequireString(t, s, "canonical_truth_authority", "mariadb")
	seq185RequireString(t, s, "vector_accelerator", "chromadb")
	if s["mode"] != "bundle_release_gate_dry_run" {
		t.Fatalf("mode=%v, want bundle_release_gate_dry_run", s["mode"])
	}
}

func TestSeq185P202LimitedLiveChromaSmokeCheck(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p202","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "limited_live_chroma_smoke_check_202")
	if s["version"] != "seq185_p202.v1" {
		t.Fatalf("version=%v, want seq185_p202.v1", s["version"])
	}
	if s["role"] != "limited_live_chroma_smoke_check" {
		t.Fatalf("role=%v, want limited_live_chroma_smoke_check", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["smoke_status"] != "contract_pass" {
		t.Fatalf("smoke_status=%v, want contract_pass", s["smoke_status"])
	}
	if s["contract_smoke_pass"] != true {
		t.Fatalf("contract_smoke_pass=%v, want true", s["contract_smoke_pass"])
	}
	if s["actual_live_chroma_smoke_run"] != false {
		t.Fatalf("actual_live_chroma_smoke_run=%v, want false", s["actual_live_chroma_smoke_run"])
	}
	if s["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", s["dry_run_only"])
	}
	if s["ready_conditions_verified"] != true {
		t.Fatalf("ready_conditions_verified=%v, want true", s["ready_conditions_verified"])
	}
	if s["mariadb_truth_authority"] != true {
		t.Fatalf("mariadb_truth_authority=%v, want true", s["mariadb_truth_authority"])
	}
	seq185RequireString(t, s, "canonical_truth_authority", "mariadb")
	seq185RequireString(t, s, "source_truth_label", "sqlite")
	if s["blocked_from_live_cutover"] != true {
		t.Fatalf("blocked_from_live_cutover=%v, want true", s["blocked_from_live_cutover"])
	}
	if s["mode"] != "limited_live_chroma_smoke_check_contract" {
		t.Fatalf("mode=%v, want limited_live_chroma_smoke_check_contract", s["mode"])
	}
}

func TestSeq185P203SQLiteFailOpenReplayPass(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p203","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "sqlite_fail_open_replay_pass_203")
	if s["version"] != "seq185_p203.v1" {
		t.Fatalf("version=%v, want seq185_p203.v1", s["version"])
	}
	if s["role"] != "sqlite_fail_open_replay_pass" {
		t.Fatalf("role=%v, want sqlite_fail_open_replay_pass", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["replay_status"] != "contract_pass" {
		t.Fatalf("replay_status=%v, want contract_pass", s["replay_status"])
	}
	if s["contract_replay_pass"] != true {
		t.Fatalf("contract_replay_pass=%v, want true", s["contract_replay_pass"])
	}
	if s["actual_sqlite_replay_run"] != false {
		t.Fatalf("actual_sqlite_replay_run=%v, want false", s["actual_sqlite_replay_run"])
	}
	if s["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", s["dry_run_only"])
	}
	if s["fallback_preserved"] != true {
		t.Fatalf("fallback_preserved=%v, want true", s["fallback_preserved"])
	}
	seq185RequireString(t, s, "source_truth_label", "sqlite")
	seq185RequireString(t, s, "canonical_truth_authority", "mariadb")
	seq185RequireString(t, s, "fallback_lane_current", "mariadb_scan_preserved")
	if s["mariadb_truth_authority_preserved"] != true {
		t.Fatalf("mariadb_truth_authority_preserved=%v, want true", s["mariadb_truth_authority_preserved"])
	}
	if s["mode"] != "sqlite_fail_open_replay_contract" {
		t.Fatalf("mode=%v, want sqlite_fail_open_replay_contract", s["mode"])
	}
}

func TestSeq185P204OperatorVisibilityFallbackChecklist(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p204","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "operator_visibility_fallback_checklist_204")
	if s["version"] != "seq185_p204.v1" {
		t.Fatalf("version=%v, want seq185_p204.v1", s["version"])
	}
	if s["role"] != "operator_visibility_fallback_checklist" {
		t.Fatalf("role=%v, want operator_visibility_fallback_checklist", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["checklist_status"] != "pass" {
		t.Fatalf("checklist_status=%v, want pass", s["checklist_status"])
	}
	if s["fallback_reason_traced"] != true {
		t.Fatalf("fallback_reason_traced=%v, want true", s["fallback_reason_traced"])
	}
	seq185RequireStringSliceContains(t, s, "inspection_states_verified", "mariadb_fallback")
	if s["mode"] != "operator_visibility_fallback_checklist_pass" {
		t.Fatalf("mode=%v, want operator_visibility_fallback_checklist_pass", s["mode"])
	}
}

func TestSeq185P205ReleaseNoteBundleNotesComplete(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p205","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "release_note_bundle_notes_complete_205")
	if s["version"] != "seq185_p205.v1" {
		t.Fatalf("version=%v, want seq185_p205.v1", s["version"])
	}
	if s["role"] != "release_note_bundle_notes_complete" {
		t.Fatalf("role=%v, want release_note_bundle_notes_complete", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["completion_status"] != "complete" {
		t.Fatalf("completion_status=%v, want complete", s["completion_status"])
	}
	if s["artifact_created"] != false {
		t.Fatalf("artifact_created=%v, want false (dry-run only)", s["artifact_created"])
	}
	if s["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", s["dry_run_only"])
	}
	if s["operator_approval_required"] != true {
		t.Fatalf("operator_approval_required=%v, want true", s["operator_approval_required"])
	}
	seq185RequireStringSliceContains(t, s, "checklist_items_closed", "bundle_regenerate_intentionally_pending")
	seq185RequireStringSliceContains(t, s, "checklist_items_closed", "beta_mirror_refresh_intentionally_pending")
	if s["mode"] != "release_note_bundle_notes_complete" {
		t.Fatalf("mode=%v, want release_note_bundle_notes_complete", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-18.5 decision tests (P209 ~ P212)
// ---------------------------------------------------------------------------

func TestSeq185P209FirstLiveScopeDecision(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p209","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "first_live_scope_decision_209")
	if s["version"] != "seq185_p209.v1" {
		t.Fatalf("version=%v, want seq185_p209.v1", s["version"])
	}
	if s["role"] != "first_live_scope_decision" {
		t.Fatalf("role=%v, want first_live_scope_decision", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	seq185RequireString(t, s, "decision", "memory_only")
	seq185RequireString(t, s, "broader_memory_episode", "blocked")
	seq185RequireString(t, s, "canonical_truth_authority", "mariadb")
	seq185RequireString(t, s, "source_scope_label", "sqlite_limited_memory")
	seq185RequireString(t, s, "current_scope_label", "mariadb_limited_memory")
	if s["operator_approval_required"] != true {
		t.Fatalf("operator_approval_required=%v, want true", s["operator_approval_required"])
	}
	if s["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", s["dry_run_only"])
	}
	if s["mode"] != "first_live_scope_decision_memory_only" {
		t.Fatalf("mode=%v, want first_live_scope_decision_memory_only", s["mode"])
	}
}

func TestSeq185P210ChromaCandidateMergeReplaceDecision(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p210","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "chroma_candidate_merge_replace_decision_210")
	if s["version"] != "seq185_p210.v1" {
		t.Fatalf("version=%v, want seq185_p210.v1", s["version"])
	}
	if s["role"] != "chroma_candidate_merge_replace_decision" {
		t.Fatalf("role=%v, want chroma_candidate_merge_replace_decision", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["replace"] != false {
		t.Fatalf("replace=%v, want false", s["replace"])
	}
	if s["merge"] != true {
		t.Fatalf("merge=%v, want true", s["merge"])
	}
	seq185RequireString(t, s, "merge_mode", "support_only_additive")
	seq185RequireString(t, s, "canonical_baseline_authority", "mariadb")
	seq185RequireString(t, s, "source_baseline_label", "sqlite")
	seq185RequireString(t, s, "chroma_candidate_role", "accelerator_not_authority")
	if s["operator_approval_required"] != true {
		t.Fatalf("operator_approval_required=%v, want true", s["operator_approval_required"])
	}
	if s["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", s["dry_run_only"])
	}
	if s["mode"] != "chroma_candidate_merge_replace_decision" {
		t.Fatalf("mode=%v, want chroma_candidate_merge_replace_decision", s["mode"])
	}
}

func TestSeq185P211DegradedThresholdDecision(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p211","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "degraded_threshold_decision_211")
	if s["version"] != "seq185_p211.v1" {
		t.Fatalf("version=%v, want seq185_p211.v1", s["version"])
	}
	if s["role"] != "degraded_threshold_decision" {
		t.Fatalf("role=%v, want degraded_threshold_decision", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	seq185RequireString(t, s, "decision", "reuse_health_probe_threshold")
	if s["health_probe_threshold_reused"] != true {
		t.Fatalf("health_probe_threshold_reused=%v, want true", s["health_probe_threshold_reused"])
	}
	if s["live_stricter_gate"] != false {
		t.Fatalf("live_stricter_gate=%v, want false", s["live_stricter_gate"])
	}
	if s["live_cutover_default"] != false {
		t.Fatalf("live_cutover_default=%v, want false", s["live_cutover_default"])
	}
	seq185RequireString(t, s, "fallback_lane", "mariadb_scan_preserved")
	seq185RequireString(t, s, "source_fallback_label", "sqlite_scan_preserved")
	seq185RequireString(t, s, "canonical_truth_authority", "mariadb")
	if s["operator_approval_required"] != true {
		t.Fatalf("operator_approval_required=%v, want true", s["operator_approval_required"])
	}
	if s["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", s["dry_run_only"])
	}
	if s["mode"] != "degraded_threshold_decision_health_probe_reused" {
		t.Fatalf("mode=%v, want degraded_threshold_decision_health_probe_reused", s["mode"])
	}
}

func TestSeq185P212OperatorVisibilityScopeDecision(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq185-p212","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "operator_visibility_scope_decision_212")
	if s["version"] != "seq185_p212.v1" {
		t.Fatalf("version=%v, want seq185_p212.v1", s["version"])
	}
	if s["role"] != "operator_visibility_scope_decision" {
		t.Fatalf("role=%v, want operator_visibility_scope_decision", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["inspection_only"] != true {
		t.Fatalf("inspection_only=%v, want true", s["inspection_only"])
	}
	if s["no_mutation"] != true {
		t.Fatalf("no_mutation=%v, want true", s["no_mutation"])
	}
	if s["no_truth_authority"] != true {
		t.Fatalf("no_truth_authority=%v, want true", s["no_truth_authority"])
	}
	seq185RequireStringSliceContains(t, s, "exposed_states", "mariadb_fallback")
	seq185RequireStringSliceContains(t, s, "exposed_states", "chroma_candidate")
	seq185RequireStringSliceContains(t, s, "exposure_locations", "settings_panel")
	seq185RequireStringSliceContains(t, s, "exposure_locations", "search_response")
	seq185RequireStringSliceContains(t, s, "exposure_locations", "prepare_turn_response")
	seq185RequireStringSliceContains(t, s, "exposure_locations", "root_runtime_trace_row")
	seq185RequireString(t, s, "canonical_truth_authority", "mariadb")
	if s["operator_approval_required"] != true {
		t.Fatalf("operator_approval_required=%v, want true", s["operator_approval_required"])
	}
	if s["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", s["dry_run_only"])
	}
	if s["mode"] != "operator_visibility_scope_decision" {
		t.Fatalf("mode=%v, want operator_visibility_scope_decision", s["mode"])
	}
}
