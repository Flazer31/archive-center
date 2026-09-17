package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSeq215P483SimplifySupervisorPlanner(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p483","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_simplify_supervisor_planner")
	if s["version"] != "s215-p483.v1" {
		t.Fatalf("version=%v, want s215-p483.v1", s["version"])
	}
	items, ok := s["simplified_paths"].([]any)
	if !ok || len(items) != 3 {
		t.Fatalf("simplified_paths=%v, want 3 items", s["simplified_paths"])
	}
	if s["hardcoded_inference_removed"] != true {
		t.Fatalf("hardcoded_inference_removed=%v, want true", s["hardcoded_inference_removed"])
	}
	if s["mode"] != "seq215_simplify_supervisor_planner_definition" {
		t.Fatalf("mode=%v, want seq215_simplify_supervisor_planner_definition", s["mode"])
	}
}

func TestSeq215P484KeepAutoAdvanceExplicit(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p484","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_keep_auto_advance_explicit")
	if s["version"] != "s215-p484.v1" {
		t.Fatalf("version=%v, want s215-p484.v1", s["version"])
	}
	if s["auto_advance_source"] != "explicit_trigger_fields" {
		t.Fatalf("auto_advance_source=%v, want explicit_trigger_fields", s["auto_advance_source"])
	}
	if s["backend_phrase_detection"] != false {
		t.Fatalf("backend_phrase_detection=%v, want false", s["backend_phrase_detection"])
	}
	if s["mode"] != "seq215_keep_auto_advance_explicit_definition" {
		t.Fatalf("mode=%v, want seq215_keep_auto_advance_explicit_definition", s["mode"])
	}
}

func TestSeq215P485PyCompilePass(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p485","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_py_compile_pass")
	if s["version"] != "s215-p485.v1" {
		t.Fatalf("version=%v, want s215-p485.v1", s["version"])
	}
	if s["compile_status"] != "pass" {
		t.Fatalf("compile_status=%v, want pass", s["compile_status"])
	}
	if s["mode"] != "seq215_py_compile_pass_definition" {
		t.Fatalf("mode=%v, want seq215_py_compile_pass_definition", s["mode"])
	}
}

func TestSeq215P486FocusedBackendTests(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p486","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_focused_backend_tests")
	if s["version"] != "s215-p486.v1" {
		t.Fatalf("version=%v, want s215-p486.v1", s["version"])
	}
	if s["tests_run"] != float64(32) {
		t.Fatalf("tests_run=%v, want 32", s["tests_run"])
	}
	if s["tests_passed"] != float64(32) {
		t.Fatalf("tests_passed=%v, want 32", s["tests_passed"])
	}
	if s["mode"] != "seq215_focused_backend_tests_definition" {
		t.Fatalf("mode=%v, want seq215_focused_backend_tests_definition", s["mode"])
	}
}

func TestSeq215P487JSUntouched(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p487","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_js_untouched")
	if s["version"] != "s215-p487.v1" {
		t.Fatalf("version=%v, want s215-p487.v1", s["version"])
	}
	if s["js_edited"] != false {
		t.Fatalf("js_edited=%v, want false", s["js_edited"])
	}
	if s["node_check"] != "pass" {
		t.Fatalf("node_check=%v, want pass", s["node_check"])
	}
	if s["mode"] != "seq215_js_untouched_definition" {
		t.Fatalf("mode=%v, want seq215_js_untouched_definition", s["mode"])
	}
}

func TestSeq215P488AfterCount(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p488","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_after_count")
	if s["version"] != "s215-p488.v1" {
		t.Fatalf("version=%v, want s215-p488.v1", s["version"])
	}
	if s["after_lines"] != float64(27942) {
		t.Fatalf("after_lines=%v, want 27942", s["after_lines"])
	}
	if s["delta_lines"] != float64(-274) {
		t.Fatalf("delta_lines=%v, want -274", s["delta_lines"])
	}
	if s["mode"] != "seq215_after_count_definition" {
		t.Fatalf("mode=%v, want seq215_after_count_definition", s["mode"])
	}
}

func TestSeq215P556JSAuthority(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p556","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_js_authority")
	if s["version"] != "s215-p556.v1" {
		t.Fatalf("version=%v, want s215-p556.v1", s["version"])
	}
	if s["authority_path"] != "Archive Center Beta 0.8(fix)/Archive Center.js" {
		t.Fatalf("authority_path=%v, want Archive Center Beta 0.8(fix)/Archive Center.js", s["authority_path"])
	}
	if s["mode"] != "seq215_js_authority_definition" {
		t.Fatalf("mode=%v, want seq215_js_authority_definition", s["mode"])
	}
}

func TestSeq215P557BackendAuthority(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p557","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_backend_authority")
	if s["version"] != "s215-p557.v1" {
		t.Fatalf("version=%v, want s215-p557.v1", s["version"])
	}
	if s["authority_path"] != "Archive Center Beta 0.8(fix)/backend/main.py" {
		t.Fatalf("authority_path=%v, want Archive Center Beta 0.8(fix)/backend/main.py", s["authority_path"])
	}
	if s["mode"] != "seq215_backend_authority_definition" {
		t.Fatalf("mode=%v, want seq215_backend_authority_definition", s["mode"])
	}
}

func TestSeq215P558NoRootStandalonePair(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p558","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_no_root_standalone_pair")
	if s["version"] != "s215-p558.v1" {
		t.Fatalf("version=%v, want s215-p558.v1", s["version"])
	}
	if s["root_has_standalone"] != false {
		t.Fatalf("root_has_standalone=%v, want false", s["root_has_standalone"])
	}
	if s["root_backend_pair"] != false {
		t.Fatalf("root_backend_pair=%v, want false", s["root_backend_pair"])
	}
	if s["mode"] != "seq215_no_root_standalone_pair_definition" {
		t.Fatalf("mode=%v, want seq215_no_root_standalone_pair_definition", s["mode"])
	}
}

func TestSeq215P559BackupNotAuthority(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p559","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_backup_not_authority")
	if s["version"] != "s215-p559.v1" {
		t.Fatalf("version=%v, want s215-p559.v1", s["version"])
	}
	if s["is_current_authority"] != false {
		t.Fatalf("is_current_authority=%v, want false", s["is_current_authority"])
	}
	if s["mode"] != "seq215_backup_not_authority_definition" {
		t.Fatalf("mode=%v, want seq215_backup_not_authority_definition", s["mode"])
	}
}

func TestSeq215P560DeployNotAuthority(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p560","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_deploy_not_authority")
	if s["version"] != "s215-p560.v1" {
		t.Fatalf("version=%v, want s215-p560.v1", s["version"])
	}
	if s["is_current_authority"] != false {
		t.Fatalf("is_current_authority=%v, want false", s["is_current_authority"])
	}
	if s["mode"] != "seq215_deploy_not_authority_definition" {
		t.Fatalf("mode=%v, want seq215_deploy_not_authority_definition", s["mode"])
	}
}

func TestSeq215P561NoBroadSplitBeforeNarrow(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p561","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_no_broad_split_before_narrow")
	if s["version"] != "s215-p561.v1" {
		t.Fatalf("version=%v, want s215-p561.v1", s["version"])
	}
	if s["broad_routes_landed"] != false {
		t.Fatalf("broad_routes_landed=%v, want false", s["broad_routes_landed"])
	}
	if s["broad_services_landed"] != false {
		t.Fatalf("broad_services_landed=%v, want false", s["broad_services_landed"])
	}
	if s["broad_schemas_landed"] != false {
		t.Fatalf("broad_schemas_landed=%v, want false", s["broad_schemas_landed"])
	}
	if s["narrow_proxy_config"] != true {
		t.Fatalf("narrow_proxy_config=%v, want true", s["narrow_proxy_config"])
	}
	if s["mode"] != "seq215_no_broad_split_before_narrow_definition" {
		t.Fatalf("mode=%v, want seq215_no_broad_split_before_narrow_definition", s["mode"])
	}
}

func TestSeq215P562StaleSplitRejectedContext(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p562","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_stale_split_rejected_context")
	if s["version"] != "s215-p562.v1" {
		t.Fatalf("version=%v, want s215-p562.v1", s["version"])
	}
	if s["rejected_claim"] != "broad_route_service_schema_split_landed" {
		t.Fatalf("rejected_claim=%v, want broad_route_service_schema_split_landed", s["rejected_claim"])
	}
	if s["mode"] != "seq215_stale_split_rejected_context_definition" {
		t.Fatalf("mode=%v, want seq215_stale_split_rejected_context_definition", s["mode"])
	}
}

func TestSeq215P563StaleSplitRejectedProgress(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p563","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_stale_split_rejected_progress")
	if s["version"] != "s215-p563.v1" {
		t.Fatalf("version=%v, want s215-p563.v1", s["version"])
	}
	if s["rejected_claim"] != "broad_route_service_schema_split_landed" {
		t.Fatalf("rejected_claim=%v, want broad_route_service_schema_split_landed", s["rejected_claim"])
	}
	if s["mode"] != "seq215_stale_split_rejected_progress_definition" {
		t.Fatalf("mode=%v, want seq215_stale_split_rejected_progress_definition", s["mode"])
	}
}

func TestSeq215P564Beta08Metrics(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p564","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_beta08_metrics")
	if s["version"] != "s215-p564.v1" {
		t.Fatalf("version=%v, want s215-p564.v1", s["version"])
	}
	if s["js_lines"] != float64(31946) {
		t.Fatalf("js_lines=%v, want 31946", s["js_lines"])
	}
	if s["backend_lines"] != float64(23720) {
		t.Fatalf("backend_lines=%v, want 23720", s["backend_lines"])
	}
	if s["route_decorators"] != float64(129) {
		t.Fatalf("route_decorators=%v, want 129", s["route_decorators"])
	}
	if s["app_decorators_total"] != float64(130) {
		t.Fatalf("app_decorators_total=%v, want 130", s["app_decorators_total"])
	}
	if s["base_model_classes"] != float64(49) {
		t.Fatalf("base_model_classes=%v, want 49", s["base_model_classes"])
	}
	if s["top_level_functions"] != float64(408) {
		t.Fatalf("top_level_functions=%v, want 408", s["top_level_functions"])
	}
	if s["metrics_source"] != "PROGRESS/CONTEXT 21.5th step final metrics" {
		t.Fatalf("metrics_source=%v", s["metrics_source"])
	}
	if s["mode"] != "seq215_beta08_metrics_definition" {
		t.Fatalf("mode=%v, want seq215_beta08_metrics_definition", s["mode"])
	}
}

func TestSeq215P565RestateGuard(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p565","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_restate_guard")
	if s["version"] != "s215-p565.v1" {
		t.Fatalf("version=%v, want s215-p565.v1", s["version"])
	}
	if s["guard_rule"] != "restate_active_edit_folder_before_each_slice" {
		t.Fatalf("guard_rule=%v, want restate_active_edit_folder_before_each_slice", s["guard_rule"])
	}
	if s["mode"] != "seq215_restate_guard_definition" {
		t.Fatalf("mode=%v, want seq215_restate_guard_definition", s["mode"])
	}
}

func TestSeq215P566PromoteGuard(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p566","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_promote_guard")
	if s["version"] != "s215-p566.v1" {
		t.Fatalf("version=%v, want s215-p566.v1", s["version"])
	}
	if s["guard_rule"] != "update_context_before_promoting_backup_deploy" {
		t.Fatalf("guard_rule=%v, want update_context_before_promoting_backup_deploy", s["guard_rule"])
	}
	if s["mode"] != "seq215_promote_guard_definition" {
		t.Fatalf("mode=%v, want seq215_promote_guard_definition", s["mode"])
	}
}

func TestSeq215P589TurnContractsCreated(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p589","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_turn_contracts_created")
	if s["version"] != "s215-p589.v1" {
		t.Fatalf("version=%v, want s215-p589.v1", s["version"])
	}
	if s["file_created"] != "backend/turn_contracts.py" {
		t.Fatalf("file_created=%v, want backend/turn_contracts.py", s["file_created"])
	}
	if s["module_created"] != "backend.turn_contracts" {
		t.Fatalf("module_created=%v, want backend.turn_contracts", s["module_created"])
	}
	if s["mode"] != "seq215_turn_contracts_created_definition" {
		t.Fatalf("mode=%v, want seq215_turn_contracts_created_definition", s["mode"])
	}
}

func TestSeq215P590CompleteTurnRequestMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p590","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_complete_turn_request_moved")
	if s["version"] != "s215-p590.v1" {
		t.Fatalf("version=%v, want s215-p590.v1", s["version"])
	}
	if s["class_name"] != "CompleteTurnRequest" {
		t.Fatalf("class_name=%v, want CompleteTurnRequest", s["class_name"])
	}
	if s["target_file"] != "backend/turn_contracts.py" {
		t.Fatalf("target_file=%v, want backend/turn_contracts.py", s["target_file"])
	}
	if s["target_module"] != "backend.turn_contracts" {
		t.Fatalf("target_module=%v, want backend.turn_contracts", s["target_module"])
	}
	if s["mode"] != "seq215_complete_turn_request_moved_definition" {
		t.Fatalf("mode=%v, want seq215_complete_turn_request_moved_definition", s["mode"])
	}
}

func TestSeq215P591M4CompleteTurnRequestMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p591","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_m4_complete_turn_request_moved")
	if s["version"] != "s215-p591.v1" {
		t.Fatalf("version=%v, want s215-p591.v1", s["version"])
	}
	if s["class_name"] != "M4CompleteTurnRequest" {
		t.Fatalf("class_name=%v, want M4CompleteTurnRequest", s["class_name"])
	}
	if s["target_module"] != "backend.turn_contracts" {
		t.Fatalf("target_module=%v, want backend.turn_contracts", s["target_module"])
	}
	if s["mode"] != "seq215_m4_complete_turn_request_moved_definition" {
		t.Fatalf("mode=%v, want seq215_m4_complete_turn_request_moved_definition", s["mode"])
	}
}

func TestSeq215P592M4CompleteTurnResponseMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p592","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_m4_complete_turn_response_moved")
	if s["version"] != "s215-p592.v1" {
		t.Fatalf("version=%v, want s215-p592.v1", s["version"])
	}
	if s["class_name"] != "M4CompleteTurnResponse" {
		t.Fatalf("class_name=%v, want M4CompleteTurnResponse", s["class_name"])
	}
	if s["target_module"] != "backend.turn_contracts" {
		t.Fatalf("target_module=%v, want backend.turn_contracts", s["target_module"])
	}
	if s["mode"] != "seq215_m4_complete_turn_response_moved_definition" {
		t.Fatalf("mode=%v, want seq215_m4_complete_turn_response_moved_definition", s["mode"])
	}
}

func TestSeq215P593PrepareTurnSettingsMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p593","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_prepare_turn_settings_moved")
	if s["version"] != "s215-p593.v1" {
		t.Fatalf("version=%v, want s215-p593.v1", s["version"])
	}
	if s["class_name"] != "PrepareTurnSettings" {
		t.Fatalf("class_name=%v, want PrepareTurnSettings", s["class_name"])
	}
	if s["target_module"] != "backend.turn_contracts" {
		t.Fatalf("target_module=%v, want backend.turn_contracts", s["target_module"])
	}
	if s["mode"] != "seq215_prepare_turn_settings_moved_definition" {
		t.Fatalf("mode=%v, want seq215_prepare_turn_settings_moved_definition", s["mode"])
	}
}

func TestSeq215P594PrepareTurnRequestMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p594","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_prepare_turn_request_moved")
	if s["version"] != "s215-p594.v1" {
		t.Fatalf("version=%v, want s215-p594.v1", s["version"])
	}
	if s["class_name"] != "PrepareTurnRequest" {
		t.Fatalf("class_name=%v, want PrepareTurnRequest", s["class_name"])
	}
	if s["target_module"] != "backend.turn_contracts" {
		t.Fatalf("target_module=%v, want backend.turn_contracts", s["target_module"])
	}
	if s["mode"] != "seq215_prepare_turn_request_moved_definition" {
		t.Fatalf("mode=%v, want seq215_prepare_turn_request_moved_definition", s["mode"])
	}
}

func TestSeq215P595RetrievalDocumentQ1AMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p595","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_retrieval_document_q1a_moved")
	if s["version"] != "s215-p595.v1" {
		t.Fatalf("version=%v, want s215-p595.v1", s["version"])
	}
	if s["class_name"] != "RetrievalDocumentQ1A" {
		t.Fatalf("class_name=%v, want RetrievalDocumentQ1A", s["class_name"])
	}
	if s["schema_symbol"] != "_RETRIEVAL_DOCUMENT_SCHEMA_Q1A" {
		t.Fatalf("schema_symbol=%v, want _RETRIEVAL_DOCUMENT_SCHEMA_Q1A", s["schema_symbol"])
	}
	if s["target_file"] != "backend/retrieval_contracts.py" {
		t.Fatalf("target_file=%v, want backend/retrieval_contracts.py", s["target_file"])
	}
	if s["target_module"] != "backend.retrieval_contracts" {
		t.Fatalf("target_module=%v, want backend.retrieval_contracts", s["target_module"])
	}
	if s["direct_test"] != "backend/test_step21_5_retrieval_contracts.py" {
		t.Fatalf("direct_test=%v, want backend/test_step21_5_retrieval_contracts.py", s["direct_test"])
	}
	if s["mode"] != "seq215_retrieval_document_q1a_moved_definition" {
		t.Fatalf("mode=%v, want seq215_retrieval_document_q1a_moved_definition", s["mode"])
	}
}

func TestSeq215P596GenerationPacketMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p596","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_generation_packet_moved")
	if s["version"] != "s215-p596.v1" {
		t.Fatalf("version=%v, want s215-p596.v1", s["version"])
	}
	if s["class_name"] != "GenerationPacket" {
		t.Fatalf("class_name=%v, want GenerationPacket", s["class_name"])
	}
	if s["target_module"] != "backend.turn_contracts" {
		t.Fatalf("target_module=%v, want backend.turn_contracts", s["target_module"])
	}
	if s["mode"] != "seq215_generation_packet_moved_definition" {
		t.Fatalf("mode=%v, want seq215_generation_packet_moved_definition", s["mode"])
	}
}

func TestSeq215P597PrepareTurnResponseMoved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p597","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_prepare_turn_response_moved")
	if s["version"] != "s215-p597.v1" {
		t.Fatalf("version=%v, want s215-p597.v1", s["version"])
	}
	if s["class_name"] != "PrepareTurnResponse" {
		t.Fatalf("class_name=%v, want PrepareTurnResponse", s["class_name"])
	}
	if s["target_module"] != "backend.turn_contracts" {
		t.Fatalf("target_module=%v, want backend.turn_contracts", s["target_module"])
	}
	if s["mode"] != "seq215_prepare_turn_response_moved_definition" {
		t.Fatalf("mode=%v, want seq215_prepare_turn_response_moved_definition", s["mode"])
	}
}

func TestSeq215P598MovedClassesImportedBack(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p598","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_moved_classes_imported_back")
	if s["version"] != "s215-p598.v1" {
		t.Fatalf("version=%v, want s215-p598.v1", s["version"])
	}
	if s["import_target"] != "main.py" {
		t.Fatalf("import_target=%v, want main.py", s["import_target"])
	}
	importSources := seq165Slice(t, s, "import_sources")
	for _, expected := range []string{"backend.turn_contracts", "backend.retrieval_contracts"} {
		if !sliceContains(importSources, expected) {
			t.Fatalf("import_sources missing %s: %v", expected, importSources)
		}
	}
	imported := seq165Slice(t, s, "imported_symbols")
	for _, expected := range []string{
		"CompleteTurnRequest",
		"M4CompleteTurnRequest",
		"M4CompleteTurnResponse",
		"PrepareTurnSettings",
		"PrepareTurnRequest",
		"GenerationPacket",
		"PrepareTurnResponse",
		"_RETRIEVAL_DOCUMENT_SCHEMA_Q1A",
		"RetrievalDocumentQ1A",
	} {
		if !sliceContains(imported, expected) {
			t.Fatalf("imported_symbols missing %s: %v", expected, imported)
		}
	}
	if s["mode"] != "seq215_moved_classes_imported_back_definition" {
		t.Fatalf("mode=%v, want seq215_moved_classes_imported_back_definition", s["mode"])
	}
}

func TestSeq215P599RouteDecoratorsStay(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p599","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_route_decorators_stay")
	if s["version"] != "s215-p599.v1" {
		t.Fatalf("version=%v, want s215-p599.v1", s["version"])
	}
	if s["stayed_in"] != "main.py" {
		t.Fatalf("stayed_in=%v, want main.py", s["stayed_in"])
	}
	if s["mode"] != "seq215_route_decorators_stay_definition" {
		t.Fatalf("mode=%v, want seq215_route_decorators_stay_definition", s["mode"])
	}
}

func TestSeq215P600PrepareTurnStays(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p600","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_prepare_turn_stays")
	if s["version"] != "s215-p600.v1" {
		t.Fatalf("version=%v, want s215-p600.v1", s["version"])
	}
	if s["function_name"] != "prepare_turn" {
		t.Fatalf("function_name=%v, want prepare_turn", s["function_name"])
	}
	if s["mode"] != "seq215_prepare_turn_stays_definition" {
		t.Fatalf("mode=%v, want seq215_prepare_turn_stays_definition", s["mode"])
	}
}

func TestSeq215P601CompleteTurnM4Stays(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p601","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_complete_turn_m4_stays")
	if s["version"] != "s215-p601.v1" {
		t.Fatalf("version=%v, want s215-p601.v1", s["version"])
	}
	if s["function_name"] != "complete_turn_m4" {
		t.Fatalf("function_name=%v, want complete_turn_m4", s["function_name"])
	}
	if s["mode"] != "seq215_complete_turn_m4_stays_definition" {
		t.Fatalf("mode=%v, want seq215_complete_turn_m4_stays_definition", s["mode"])
	}
}

func TestSeq215P602PublicRoutePathsUnchanged(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p602","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_public_route_paths_unchanged")
	if s["version"] != "s215-p602.v1" {
		t.Fatalf("version=%v, want s215-p602.v1", s["version"])
	}
	if s["paths_unchanged"] != true {
		t.Fatalf("paths_unchanged=%v, want true", s["paths_unchanged"])
	}
	paths := seq165Slice(t, s, "route_paths")
	for _, expected := range []string{"/turns/complete", "/complete-turn", "/prepare-turn"} {
		if !sliceContains(paths, expected) {
			t.Fatalf("route_paths missing %s: %v", expected, paths)
		}
	}
	if s["mode"] != "seq215_public_route_paths_unchanged_definition" {
		t.Fatalf("mode=%v, want seq215_public_route_paths_unchanged_definition", s["mode"])
	}
}

func TestSeq215P603ResponseFieldsUnchanged(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p603","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_response_fields_unchanged")
	if s["version"] != "s215-p603.v1" {
		t.Fatalf("version=%v, want s215-p603.v1", s["version"])
	}
	if s["fields_unchanged"] != true {
		t.Fatalf("fields_unchanged=%v, want true", s["fields_unchanged"])
	}
	if s["defaults_unchanged"] != true {
		t.Fatalf("defaults_unchanged=%v, want true", s["defaults_unchanged"])
	}
	models := seq165Slice(t, s, "response_models")
	for _, expected := range []string{"M4CompleteTurnResponse", "PrepareTurnResponse", "GenerationPacket"} {
		if !sliceContains(models, expected) {
			t.Fatalf("response_models missing %s: %v", expected, models)
		}
	}
	defaults, ok := s["default_checks"].(map[string]any)
	if !ok {
		t.Fatalf("default_checks missing or wrong type: %T", s["default_checks"])
	}
	for key, want := range map[string]string{
		"GenerationPacket.packet_mode":        "off",
		"PrepareTurnResponse.source":          "skeleton",
		"PrepareTurnResponse.fallback_reason": "skeleton_only",
	} {
		if defaults[key] != want {
			t.Fatalf("default_checks[%s]=%v, want %s", key, defaults[key], want)
		}
	}
	if s["mode"] != "seq215_response_fields_unchanged_definition" {
		t.Fatalf("mode=%v, want seq215_response_fields_unchanged_definition", s["mode"])
	}
}

func TestSeq215P604NoBroadTreeCreated(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p604","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_no_broad_tree_created")
	if s["version"] != "s215-p604.v1" {
		t.Fatalf("version=%v, want s215-p604.v1", s["version"])
	}
	if s["broad_routes_created"] != false {
		t.Fatalf("broad_routes_created=%v, want false", s["broad_routes_created"])
	}
	if s["broad_services_created"] != false {
		t.Fatalf("broad_services_created=%v, want false", s["broad_services_created"])
	}
	if s["broad_schemas_created"] != false {
		t.Fatalf("broad_schemas_created=%v, want false", s["broad_schemas_created"])
	}
	if s["mode"] != "seq215_no_broad_tree_created_definition" {
		t.Fatalf("mode=%v, want seq215_no_broad_tree_created_definition", s["mode"])
	}
}

func TestSeq215P605PyCompileTurnContracts(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p605","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_py_compile_turn_contracts")
	if s["version"] != "s215-p605.v1" {
		t.Fatalf("version=%v, want s215-p605.v1", s["version"])
	}
	if s["compile_status"] != "pass" {
		t.Fatalf("compile_status=%v, want pass", s["compile_status"])
	}
	if s["mode"] != "seq215_py_compile_turn_contracts_definition" {
		t.Fatalf("mode=%v, want seq215_py_compile_turn_contracts_definition", s["mode"])
	}
}

func TestSeq215P606FocusedImportCheck(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p606","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_focused_import_check")
	if s["version"] != "s215-p606.v1" {
		t.Fatalf("version=%v, want s215-p606.v1", s["version"])
	}
	if s["check_status"] != "pass" {
		t.Fatalf("check_status=%v, want pass", s["check_status"])
	}
	if s["mode"] != "seq215_focused_import_check_definition" {
		t.Fatalf("mode=%v, want seq215_focused_import_check_definition", s["mode"])
	}
}
