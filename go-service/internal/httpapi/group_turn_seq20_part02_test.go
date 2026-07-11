package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSeq20P81Q20fTokenBoundaryStructuredLabels(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p81","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20f_token_boundary_structured_labels")
	if s["version"] != "q20f-p81.v1" {
		t.Fatalf("version=%v, want q20f-p81.v1", s["version"])
	}
	if s["matching_rule"] != "token_boundary_only" {
		t.Fatalf("matching_rule=%v, want token_boundary_only", s["matching_rule"])
	}
	if s["preserves_attached_forms"] != true {
		t.Fatalf("preserves_attached_forms=%v, want true", s["preserves_attached_forms"])
	}
	if s["blocks_mid_token_substrings"] != true {
		t.Fatalf("blocks_mid_token_substrings=%v, want true", s["blocks_mid_token_substrings"])
	}
	attachedOk, ok := s["example_attached_ok"].([]any)
	if !ok || len(attachedOk) != 3 {
		t.Fatalf("example_attached_ok=%v, want 3 items", s["example_attached_ok"])
	}
	blocked, ok := s["example_mid_token_blocked"].([]any)
	if !ok || len(blocked) != 1 {
		t.Fatalf("example_mid_token_blocked=%v, want 1 item", s["example_mid_token_blocked"])
	}
	if s["mode"] != "q20f_token_boundary_structured_labels_definition" {
		t.Fatalf("mode=%v, want q20f_token_boundary_structured_labels_definition", s["mode"])
	}
}

func TestSeq20P89Q20gGraphLikeSupportSignalPreparatory(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p89","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20g_graph_like_support_signal_preparatory")
	if s["version"] != "q20g-p89.v1" {
		t.Fatalf("version=%v, want q20g-p89.v1", s["version"])
	}
	if s["status"] != "contract_only_closed" {
		t.Fatalf("status=%v, want contract_only_closed", s["status"])
	}
	if s["mode"] != "q20g_graph_like_support_signal_preparatory_definition" {
		t.Fatalf("mode=%v, want q20g_graph_like_support_signal_preparatory_definition", s["mode"])
	}
}

func TestSeq20P90Q20gV1GraphLikeSupportSignal(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p90","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20g_v1_graph_like_support_signal")
	if s["version"] != "q20g.v1" {
		t.Fatalf("version=%v, want q20g.v1", s["version"])
	}
	if s["activation_gate"] != "q20f.v1_structured_entity_focus_terms_and_structured_pair_link" {
		t.Fatalf("activation_gate=%v, want q20f.v1_structured_entity_focus_terms_and_structured_pair_link", s["activation_gate"])
	}
	if s["mode"] != "q20g_v1_graph_like_support_signal_definition" {
		t.Fatalf("mode=%v, want q20g_v1_graph_like_support_signal_definition", s["mode"])
	}
}

func TestSeq20P91Q20gPairSourcesAndFailOpen(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p91","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20g_pair_sources_and_fail_open")
	if s["version"] != "q20g-p91.v1" {
		t.Fatalf("version=%v, want q20g-p91.v1", s["version"])
	}
	pairSources, ok := s["pair_sources"].([]any)
	if !ok || len(pairSources) != 2 {
		t.Fatalf("pair_sources=%v, want 2 items", s["pair_sources"])
	}
	if s["graph_support_mode"] != "optional_graph_accelerator" {
		t.Fatalf("graph_support_mode=%v, want optional_graph_accelerator", s["graph_support_mode"])
	}
	if s["fail_open_no_pair"] != true {
		t.Fatalf("fail_open_no_pair=%v, want true", s["fail_open_no_pair"])
	}
	if s["required_read_lane"] != false {
		t.Fatalf("required_read_lane=%v, want false", s["required_read_lane"])
	}
	if s["mode"] != "q20g_pair_sources_and_fail_open_definition" {
		t.Fatalf("mode=%v, want q20g_pair_sources_and_fail_open_definition", s["mode"])
	}
}

func TestSeq20P92Q20gMirroredAtQueryClass(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p92","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20g_mirrored_at_query_class")
	if s["version"] != "q20g-p92.v1" {
		t.Fatalf("version=%v, want q20g-p92.v1", s["version"])
	}
	if s["mirror_payload"] != "graph_like_support_signal" {
		t.Fatalf("mirror_payload=%v, want graph_like_support_signal", s["mirror_payload"])
	}
	if s["mode"] != "q20g_mirrored_at_query_class_definition" {
		t.Fatalf("mode=%v, want q20g_mirrored_at_query_class_definition", s["mode"])
	}
}

func TestSeq20P93Q20gStopsBeforeInspectionFormatting(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p93","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20g_stops_before_inspection_formatting")
	if s["version"] != "q20g-p93.v1" {
		t.Fatalf("version=%v, want q20g-p93.v1", s["version"])
	}
	stopsBefore, ok := s["stops_before"].([]any)
	if !ok || len(stopsBefore) != 2 {
		t.Fatalf("stops_before=%v, want 2 items", s["stops_before"])
	}
	if s["scope"] != "optional_pair_link_accelerator_only" {
		t.Fatalf("scope=%v, want optional_pair_link_accelerator_only", s["scope"])
	}
	if s["no_backfill"] != true {
		t.Fatalf("no_backfill=%v, want true", s["no_backfill"])
	}
	if s["mode"] != "q20g_stops_before_inspection_formatting_definition" {
		t.Fatalf("mode=%v, want q20g_stops_before_inspection_formatting_definition", s["mode"])
	}
}

func TestSeq20P99Q20hEntityGraphBoostInspectionSurfacePreparatory(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p99","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20h_entity_graph_boost_inspection_surface_preparatory")
	if s["version"] != "q20h-p99.v1" {
		t.Fatalf("version=%v, want q20h-p99.v1", s["version"])
	}
	if s["status"] != "contract_only_closed" {
		t.Fatalf("status=%v, want contract_only_closed", s["status"])
	}
	if s["mode"] != "q20h_entity_graph_boost_inspection_surface_preparatory_definition" {
		t.Fatalf("mode=%v, want q20h_entity_graph_boost_inspection_surface_preparatory_definition", s["mode"])
	}
}

func TestSeq20P100Q20hV1EntityGraphBoostInspectionSurface(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p100","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20h_v1_entity_graph_boost_inspection_surface")
	if s["version"] != "q20h.v1" {
		t.Fatalf("version=%v, want q20h.v1", s["version"])
	}
	mirrors, ok := s["mirrors"].([]any)
	if !ok || len(mirrors) != 4 {
		t.Fatalf("mirrors=%v, want 4 items", s["mirrors"])
	}
	if s["mode"] != "q20h_v1_entity_graph_boost_inspection_surface_definition" {
		t.Fatalf("mode=%v, want q20h_v1_entity_graph_boost_inspection_surface_definition", s["mode"])
	}
}

func TestSeq20P101Q20hInspectionRoleAndAuthorityNotice(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p101","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20h_inspection_role_and_authority_notice")
	if s["version"] != "q20h-p101.v1" {
		t.Fatalf("version=%v, want q20h-p101.v1", s["version"])
	}
	if s["inspection_surface_mode"] != "entity_graph_boost_trace" {
		t.Fatalf("inspection_surface_mode=%v, want entity_graph_boost_trace", s["inspection_surface_mode"])
	}
	if s["inspection_role"] != "read_only_support_trace" {
		t.Fatalf("inspection_role=%v, want read_only_support_trace", s["inspection_role"])
	}
	if s["authority_notice"] != "support_only_accelerator_not_truth" {
		t.Fatalf("authority_notice=%v, want support_only_accelerator_not_truth", s["authority_notice"])
	}
	if s["mode"] != "q20h_inspection_role_and_authority_notice_definition" {
		t.Fatalf("mode=%v, want q20h_inspection_role_and_authority_notice_definition", s["mode"])
	}
}

func TestSeq20P102Q20hMirroredAtQueryClass(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p102","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20h_mirrored_at_query_class")
	if s["version"] != "q20h-p102.v1" {
		t.Fatalf("version=%v, want q20h-p102.v1", s["version"])
	}
	if s["mirror_payload"] != "entity_graph_boost_inspection_surface" {
		t.Fatalf("mirror_payload=%v, want entity_graph_boost_inspection_surface", s["mirror_payload"])
	}
	if s["mode"] != "q20h_mirrored_at_query_class_definition" {
		t.Fatalf("mode=%v, want q20h_mirrored_at_query_class_definition", s["mode"])
	}
}

func TestSeq20P109Q20iLaggingCurrentStateBoostPreparatory(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p109","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20i_lagging_current_state_boost_preparatory")
	if s["version"] != "q20i-p109.v1" {
		t.Fatalf("version=%v, want q20i-p109.v1", s["version"])
	}
	if s["status"] != "contract_only_closed" {
		t.Fatalf("status=%v, want contract_only_closed", s["status"])
	}
	if s["mode"] != "q20i_lagging_current_state_boost_preparatory_definition" {
		t.Fatalf("mode=%v, want q20i_lagging_current_state_boost_preparatory_definition", s["mode"])
	}
}

func TestSeq20P110Q20iV1LaggingCurrentStateBoost(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p110","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20i_v1_lagging_current_state_boost")
	if s["version"] != "q20i.v1" {
		t.Fatalf("version=%v, want q20i.v1", s["version"])
	}
	composes, ok := s["composes"].([]any)
	if !ok || len(composes) != 4 {
		t.Fatalf("composes=%v, want 4 items", s["composes"])
	}
	if s["rescue_rule"] != "support_only" {
		t.Fatalf("rescue_rule=%v, want support_only", s["rescue_rule"])
	}
	if s["mode"] != "q20i_v1_lagging_current_state_boost_definition" {
		t.Fatalf("mode=%v, want q20i_v1_lagging_current_state_boost_definition", s["mode"])
	}
}

func TestSeq20P111Q20iActivationAndPrecedence(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p111","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20i_activation_and_precedence")
	if s["version"] != "q20i-p111.v1" {
		t.Fatalf("version=%v, want q20i-p111.v1", s["version"])
	}
	activationRequires, ok := s["activation_requires"].([]any)
	if !ok || len(activationRequires) != 2 {
		t.Fatalf("activation_requires=%v, want 2 items", s["activation_requires"])
	}
	if s["chronology"] != "deferred" {
		t.Fatalf("chronology=%v, want deferred", s["chronology"])
	}
	boostPrecedence, ok := s["boost_precedence"].([]any)
	if !ok || len(boostPrecedence) != 3 {
		t.Fatalf("boost_precedence=%v, want 3 items", s["boost_precedence"])
	}
	if s["support_only_accelerator_not_truth"] != true {
		t.Fatalf("support_only_accelerator_not_truth=%v, want true", s["support_only_accelerator_not_truth"])
	}
	if s["mode"] != "q20i_activation_and_precedence_definition" {
		t.Fatalf("mode=%v, want q20i_activation_and_precedence_definition", s["mode"])
	}
}

func TestSeq20P112Q20iMirroredAtQueryClass(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p112","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20i_mirrored_at_query_class")
	if s["version"] != "q20i-p112.v1" {
		t.Fatalf("version=%v, want q20i-p112.v1", s["version"])
	}
	if s["mirror_payload"] != "lagging_current_state_boost" {
		t.Fatalf("mirror_payload=%v, want lagging_current_state_boost", s["mirror_payload"])
	}
	if s["mode"] != "q20i_mirrored_at_query_class_definition" {
		t.Fatalf("mode=%v, want q20i_mirrored_at_query_class_definition", s["mode"])
	}
}

func TestSeq20P118Q20jMotiveShadowHintPreparatory(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p118","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20j_motive_shadow_hint_preparatory")
	if s["version"] != "q20j-p118.v1" {
		t.Fatalf("version=%v, want q20j-p118.v1", s["version"])
	}
	if s["status"] != "contract_only_closed" {
		t.Fatalf("status=%v, want contract_only_closed", s["status"])
	}
	if s["mode"] != "q20j_motive_shadow_hint_preparatory_definition" {
		t.Fatalf("mode=%v, want q20j_motive_shadow_hint_preparatory_definition", s["mode"])
	}
}

func TestSeq20P119Q20jV1MotiveShadowHint(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p119","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20j_v1_motive_shadow_hint")
	if s["version"] != "q20j.v1" {
		t.Fatalf("version=%v, want q20j.v1", s["version"])
	}
	whitelist, ok := s["signal_whitelist"].([]any)
	if !ok || len(whitelist) != 5 {
		t.Fatalf("signal_whitelist=%v, want 5 items", s["signal_whitelist"])
	}
	if s["source"] != "structured_character_personality_state" {
		t.Fatalf("source=%v, want structured_character_personality_state", s["source"])
	}
	if s["query_anchor"] != "character_anchored_only" {
		t.Fatalf("query_anchor=%v, want character_anchored_only", s["query_anchor"])
	}
	if s["mode"] != "q20j_v1_motive_shadow_hint_definition" {
		t.Fatalf("mode=%v, want q20j_v1_motive_shadow_hint_definition", s["mode"])
	}
}

func TestSeq20P120Q20jTruthWriteForbidden(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p120","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20j_truth_write_forbidden")
	if s["version"] != "q20j-p120.v1" {
		t.Fatalf("version=%v, want q20j-p120.v1", s["version"])
	}
	if s["truth_write_mode"] != "forbidden" {
		t.Fatalf("truth_write_mode=%v, want forbidden", s["truth_write_mode"])
	}
	if s["hint_role"] != "support_only" {
		t.Fatalf("hint_role=%v, want support_only", s["hint_role"])
	}
	if s["stops_before_branch_escalation"] != true {
		t.Fatalf("stops_before_branch_escalation=%v, want true", s["stops_before_branch_escalation"])
	}
	if s["stops_before_foreground"] != true {
		t.Fatalf("stops_before_foreground=%v, want true", s["stops_before_foreground"])
	}
	if s["mode"] != "q20j_truth_write_forbidden_definition" {
		t.Fatalf("mode=%v, want q20j_truth_write_forbidden_definition", s["mode"])
	}
}

func TestSeq20P121Q20jMirroredAtQueryClass(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p121","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20j_mirrored_at_query_class")
	if s["version"] != "q20j-p121.v1" {
		t.Fatalf("version=%v, want q20j-p121.v1", s["version"])
	}
	if s["mirror_payload"] != "motive_shadow_hint" {
		t.Fatalf("mirror_payload=%v, want motive_shadow_hint", s["mirror_payload"])
	}
	if s["mode"] != "q20j_mirrored_at_query_class_definition" {
		t.Fatalf("mode=%v, want q20j_mirrored_at_query_class_definition", s["mode"])
	}
}

func TestSeq20P127Q20kMotiveShadowNonEscalationGuardPreparatory(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p127","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20k_motive_shadow_non_escalation_guard_preparatory")
	if s["version"] != "q20k-p127.v1" {
		t.Fatalf("version=%v, want q20k-p127.v1", s["version"])
	}
	if s["status"] != "contract_only_closed" {
		t.Fatalf("status=%v, want contract_only_closed", s["status"])
	}
	if s["mode"] != "q20k_motive_shadow_non_escalation_guard_preparatory_definition" {
		t.Fatalf("mode=%v, want q20k_motive_shadow_non_escalation_guard_preparatory_definition", s["mode"])
	}
}

func TestSeq20P128Q20kV1MotiveShadowNonEscalationGuard(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p128","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20k_v1_motive_shadow_non_escalation_guard")
	if s["version"] != "q20k.v1" {
		t.Fatalf("version=%v, want q20k.v1", s["version"])
	}
	if s["lane"] != "support_only_disambiguation_hint" {
		t.Fatalf("lane=%v, want support_only_disambiguation_hint", s["lane"])
	}
	blocked := seq165Slice(t, s, "blocked_write_targets")
	if !sliceContains(blocked, "current_fact") {
		t.Fatalf("blocked_write_targets missing current_fact: %v", blocked)
	}
	if !sliceContains(blocked, "canonical_relationship_state") {
		t.Fatalf("blocked_write_targets missing canonical_relationship_state: %v", blocked)
	}
	if s["prevents_stale_arc_auto_foreground"] != true {
		t.Fatalf("prevents_stale_arc_auto_foreground=%v, want true", s["prevents_stale_arc_auto_foreground"])
	}
	if s["mode"] != "q20k_v1_motive_shadow_non_escalation_guard_definition" {
		t.Fatalf("mode=%v, want q20k_v1_motive_shadow_non_escalation_guard_definition", s["mode"])
	}
}

func TestSeq20P129Q20kMirroredAtQueryClass(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p129","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20k_mirrored_at_query_class")
	if s["version"] != "q20k-p129.v1" {
		t.Fatalf("version=%v, want q20k-p129.v1", s["version"])
	}
	if s["mirror_payload"] != "motive_shadow_non_escalation_guard" {
		t.Fatalf("mirror_payload=%v, want motive_shadow_non_escalation_guard", s["mirror_payload"])
	}
	if s["mode"] != "q20k_mirrored_at_query_class_definition" {
		t.Fatalf("mode=%v, want q20k_mirrored_at_query_class_definition", s["mode"])
	}
}

func TestSeq20P135Q20lRelationEdgeSupportLedgerPreparatory(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p135","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20l_relation_edge_support_ledger_preparatory")
	if s["version"] != "q20l-p135.v1" {
		t.Fatalf("version=%v, want q20l-p135.v1", s["version"])
	}
	if s["status"] != "contract_only_closed" {
		t.Fatalf("status=%v, want contract_only_closed", s["status"])
	}
	if s["mode"] != "q20l_relation_edge_support_ledger_preparatory_definition" {
		t.Fatalf("mode=%v, want q20l_relation_edge_support_ledger_preparatory_definition", s["mode"])
	}
}

func TestSeq20P136Q20lV1RelationEdgeSupportLedger(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p136","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20l_v1_relation_edge_support_ledger")
	if s["version"] != "q20l.v1" {
		t.Fatalf("version=%v, want q20l.v1", s["version"])
	}
	if s["source"] != "structured_relationship_state_only" {
		t.Fatalf("source=%v, want structured_relationship_state_only", s["source"])
	}
	fields := seq165Slice(t, s, "summary_fields")
	expectedFields := []string{"pair", "current_dynamic", "trust_level", "imbalance", "recent_shift"}
	for _, f := range expectedFields {
		if !sliceContains(fields, f) {
			t.Fatalf("summary_fields missing %q: %v", f, fields)
		}
	}
	if s["pending_thread_pair_alone_blocked"] != true {
		t.Fatalf("pending_thread_pair_alone_blocked=%v, want true", s["pending_thread_pair_alone_blocked"])
	}
	if s["mode"] != "q20l_v1_relation_edge_support_ledger_definition" {
		t.Fatalf("mode=%v, want q20l_v1_relation_edge_support_ledger_definition", s["mode"])
	}
}

func TestSeq20P137Q20lGraphTruthWriteForbidden(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p137","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20l_graph_truth_write_forbidden")
	if s["version"] != "q20l-p137.v1" {
		t.Fatalf("version=%v, want q20l-p137.v1", s["version"])
	}
	if s["graph_truth_write_mode"] != "forbidden" {
		t.Fatalf("graph_truth_write_mode=%v, want forbidden", s["graph_truth_write_mode"])
	}
	if s["support_only"] != true {
		t.Fatalf("support_only=%v, want true", s["support_only"])
	}
	if s["graph_truth_promotion_blocked"] != true {
		t.Fatalf("graph_truth_promotion_blocked=%v, want true", s["graph_truth_promotion_blocked"])
	}
	if s["mode"] != "q20l_graph_truth_write_forbidden_definition" {
		t.Fatalf("mode=%v, want q20l_graph_truth_write_forbidden_definition", s["mode"])
	}
}

func TestSeq20P138Q20lMirroredAtQueryClass(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p138","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20l_mirrored_at_query_class")
	if s["version"] != "q20l-p138.v1" {
		t.Fatalf("version=%v, want q20l-p138.v1", s["version"])
	}
	if s["mirror_payload"] != "relation_edge_support_ledger" {
		t.Fatalf("mirror_payload=%v, want relation_edge_support_ledger", s["mirror_payload"])
	}
	if s["mode"] != "q20l_mirrored_at_query_class_definition" {
		t.Fatalf("mode=%v, want q20l_mirrored_at_query_class_definition", s["mode"])
	}
}

func TestSeq20P231ValidityPriority(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p231","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq20_validity_priority")
	if s["version"] != "s20-p231.v1" {
		t.Fatalf("version=%v, want s20-p231.v1", s["version"])
	}
	if s["priority_rule"] != "validity_event_before_recency" {
		t.Fatalf("priority_rule=%v, want validity_event_before_recency", s["priority_rule"])
	}
	if s["mode"] != "seq20_validity_priority_definition" {
		t.Fatalf("mode=%v, want seq20_validity_priority_definition", s["mode"])
	}
}

func TestSeq20P232SupportOnlyAccelerator(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p232","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq20_support_only_accelerator")
	if s["version"] != "s20-p232.v1" {
		t.Fatalf("version=%v, want s20-p232.v1", s["version"])
	}
	if s["accelerator_lane"] != "support_only_boost" {
		t.Fatalf("accelerator_lane=%v, want support_only_boost", s["accelerator_lane"])
	}
	disallowed := seq165Slice(t, s, "disallowed")
	for _, d := range []string{"truth_write", "canonical_overwrite", "direct_override"} {
		if !sliceContains(disallowed, d) {
			t.Fatalf("disallowed missing %q: %v", d, disallowed)
		}
	}
	if s["mode"] != "seq20_support_only_accelerator_definition" {
		t.Fatalf("mode=%v, want seq20_support_only_accelerator_definition", s["mode"])
	}
}

func TestSeq20P233AmbiguityReduction(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p233","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq20_ambiguity_reduction")
	if s["version"] != "s20-p233.v1" {
		t.Fatalf("version=%v, want s20-p233.v1", s["version"])
	}
	if s["goal"] != "narrow_candidates_without_truth_manipulation" {
		t.Fatalf("goal=%v, want narrow_candidates_without_truth_manipulation", s["goal"])
	}
	if s["mode"] != "seq20_ambiguity_reduction_definition" {
		t.Fatalf("mode=%v, want seq20_ambiguity_reduction_definition", s["mode"])
	}
}

func TestSeq20P234InspectionVisibility(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p234","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq20_inspection_visibility")
	if s["version"] != "s20-p234.v1" {
		t.Fatalf("version=%v, want s20-p234.v1", s["version"])
	}
	surfaces := seq165Slice(t, s, "inspection_surfaces")
	for _, expected := range []string{"temporal_boost_rationale", "entity_boost_rationale", "graph_boost_rationale"} {
		if !sliceContains(surfaces, expected) {
			t.Fatalf("inspection_surfaces missing %q: %v", expected, surfaces)
		}
	}
	if s["mode"] != "seq20_inspection_visibility_definition" {
		t.Fatalf("mode=%v, want seq20_inspection_visibility_definition", s["mode"])
	}
}

func TestSeq20P235TruthPrecedencePreserve(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p235","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq20_truth_precedence_preserve")
	if s["version"] != "s20-p235.v1" {
		t.Fatalf("version=%v, want s20-p235.v1", s["version"])
	}
	order := seq165Slice(t, s, "precedence_order")
	expectedOrder := []string{"canonical_state", "direct_evidence", "support_lane"}
	for i, expected := range expectedOrder {
		if i >= len(order) || order[i] != expected {
			t.Fatalf("precedence_order[%d]=%v, want %v (full=%v)", i, order[i], expected, order)
		}
	}
	if s["support_lane_ceiling"] != "read_only_boost" {
		t.Fatalf("support_lane_ceiling=%v, want read_only_boost", s["support_lane_ceiling"])
	}
	if s["mode"] != "seq20_truth_precedence_preserve_definition" {
		t.Fatalf("mode=%v, want seq20_truth_precedence_preserve_definition", s["mode"])
	}
}

func TestSeq20P236HotBridge(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p236","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq20_hot_bridge")
	if s["version"] != "s20-p236.v1" {
		t.Fatalf("version=%v, want s20-p236.v1", s["version"])
	}
	if s["bridge_lane"] != "recent_turn_recall_bridge" {
		t.Fatalf("bridge_lane=%v, want recent_turn_recall_bridge", s["bridge_lane"])
	}
	if s["is_truth_store"] != false {
		t.Fatalf("is_truth_store=%v, want false", s["is_truth_store"])
	}
	srcSet := seq165Slice(t, s, "source_set")
	for _, expected := range []string{"latest_direct_evidence", "scoped_verbatim_support", "recent_raw_turn"} {
		if !sliceContains(srcSet, expected) {
			t.Fatalf("source_set missing %q: %v", expected, srcSet)
		}
	}
	if s["mode"] != "seq20_hot_bridge_definition" {
		t.Fatalf("mode=%v, want seq20_hot_bridge_definition", s["mode"])
	}
}

// TestSeq20P240RollupQ20aTemporalQueryExpansion confirms that the q20a temporal
// query expansion surface is present and correctly versioned for SEQ-20-P240.
func TestSeq20P240RollupQ20aTemporalQueryExpansion(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p240","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20a_v1_temporal_query_expansion")
	if s["version"] != "q20a.v1" {
		t.Fatalf("version=%v, want q20a.v1", s["version"])
	}
	if s["mode"] != "q20a_v1_temporal_query_expansion_definition" {
		t.Fatalf("mode=%v, want q20a_v1_temporal_query_expansion_definition", s["mode"])
	}
}

// TestSeq20P241RollupQ20bTemporalValidityReadPolicy confirms that the q20b
// temporal validity read policy surface is present for SEQ-20-P241.
func TestSeq20P241RollupQ20bTemporalValidityReadPolicy(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p241","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20b_v1_temporal_validity_read_policy")
	if s["version"] != "q20b.v1" {
		t.Fatalf("version=%v, want q20b.v1", s["version"])
	}
	if s["mode"] != "q20b_v1_temporal_validity_read_policy_definition" {
		t.Fatalf("mode=%v, want q20b_v1_temporal_validity_read_policy_definition", s["mode"])
	}
}

// TestSeq20P242RollupQ20cTemporalEventInvalidation confirms that the q20c
// temporal event invalidation support surface is present for SEQ-20-P242.
func TestSeq20P242RollupQ20cTemporalEventInvalidation(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p242","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20c_v1_temporal_event_invalidation_support")
	if s["version"] != "q20c.v1" {
		t.Fatalf("version=%v, want q20c.v1", s["version"])
	}
	if s["mode"] != "q20c_v1_temporal_event_invalidation_support_definition" {
		t.Fatalf("mode=%v, want q20c_v1_temporal_event_invalidation_support_definition", s["mode"])
	}
}

// TestSeq20P243RollupQ20dTemporalPromotionLag confirms that the q20d temporal
// promotion-lag support surface is present for SEQ-20-P243.
func TestSeq20P243RollupQ20dTemporalPromotionLag(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p243","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20d_v1_temporal_promotion_lag_support")
	if s["version"] != "q20d.v1" {
		t.Fatalf("version=%v, want q20d.v1", s["version"])
	}
	if s["mode"] != "q20d_v1_temporal_promotion_lag_support_definition" {
		t.Fatalf("mode=%v, want q20d_v1_temporal_promotion_lag_support_definition", s["mode"])
	}
}

// TestSeq20P244RollupQ20eTemporalHotRecallBuffer confirms that the q20e
// temporal hot recall buffer surface is present for SEQ-20-P244.
func TestSeq20P244RollupQ20eTemporalHotRecallBuffer(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p244","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "q20e_v1_temporal_hot_recall_buffer")
	if s["version"] != "q20e.v1" {
		t.Fatalf("version=%v, want q20e.v1", s["version"])
	}
	if s["mode"] != "q20e_v1_temporal_hot_recall_buffer_definition" {
		t.Fatalf("mode=%v, want q20e_v1_temporal_hot_recall_buffer_definition", s["mode"])
	}
}
