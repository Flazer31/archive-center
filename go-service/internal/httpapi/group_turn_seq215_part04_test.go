package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSeq215P778JSInjectionBudgetOwner(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p778","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_js_injection_budget_owner")
	if s["version"] != "s215-p778.v1" {
		t.Fatalf("version=%v, want s215-p778.v1", s["version"])
	}
	if s["owner"] != "js_runtime" {
		t.Fatalf("owner=%v, want js_runtime", s["owner"])
	}
	if s["responsibility"] != "injection_budget_application" {
		t.Fatalf("responsibility=%v, want injection_budget_application", s["responsibility"])
	}
	source := seq215ArchiveCenterJSSource(t)
	seq215RequireJSSourceContains(t, source,
		"function assembleInjectionWithBudget(",
		"const budgetResult = assembleInjectionWithBudget(",
	)
	if s["mode"] != "seq215_js_injection_budget_owner_definition" {
		t.Fatalf("mode=%v, want seq215_js_injection_budget_owner_definition", s["mode"])
	}
}

func TestSeq215P779JSInputContextSlottingOwner(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p779","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_js_input_context_slotting_owner")
	if s["version"] != "s215-p779.v1" {
		t.Fatalf("version=%v, want s215-p779.v1", s["version"])
	}
	if s["owner"] != "js_runtime" {
		t.Fatalf("owner=%v, want js_runtime", s["owner"])
	}
	if s["responsibility"] != "input_context_slotting" {
		t.Fatalf("responsibility=%v, want input_context_slotting", s["responsibility"])
	}
	source := seq215ArchiveCenterJSSource(t)
	seq215RequireJSSourceContains(t, source,
		"function buildInputContext(",
		"_ip.input_context_text",
		"inputCtx = buildInputContext(",
	)
	if s["mode"] != "seq215_js_input_context_slotting_owner_definition" {
		t.Fatalf("mode=%v, want seq215_js_input_context_slotting_owner_definition", s["mode"])
	}
}

func TestSeq215P780JSProtectionBlocksOwner(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p780","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_js_protection_blocks_owner")
	if s["version"] != "s215-p780.v1" {
		t.Fatalf("version=%v, want s215-p780.v1", s["version"])
	}
	if s["owner"] != "js_runtime" {
		t.Fatalf("owner=%v, want js_runtime", s["owner"])
	}
	if s["responsibility"] != "protection_blocks" {
		t.Fatalf("responsibility=%v, want protection_blocks", s["responsibility"])
	}
	source := seq215ArchiveCenterJSSource(t)
	seq215RequireJSSourceContains(t, source,
		"protection: protection",
		"baseRulesIncluded",
		"reliabilityGuardIncluded",
	)
	if s["mode"] != "seq215_js_protection_blocks_owner_definition" {
		t.Fatalf("mode=%v, want seq215_js_protection_blocks_owner_definition", s["mode"])
	}
}

func TestSeq215P781JSHookUIIntegrationOwner(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p781","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_js_hook_ui_integration_owner")
	if s["version"] != "s215-p781.v1" {
		t.Fatalf("version=%v, want s215-p781.v1", s["version"])
	}
	if s["owner"] != "js_runtime" {
		t.Fatalf("owner=%v, want js_runtime", s["owner"])
	}
	if s["responsibility"] != "hook_ui_integration" {
		t.Fatalf("responsibility=%v, want hook_ui_integration", s["responsibility"])
	}
	source := seq215ArchiveCenterJSSource(t)
	seq215RequireJSSourceContains(t, source,
		"addRisuReplacer(\"beforeRequest\", onBeforeRequest)",
		"addRisuReplacer(\"afterRequest\", onAfterRequest)",
		"renderSettingsPanel",
	)
	if s["mode"] != "seq215_js_hook_ui_integration_owner_definition" {
		t.Fatalf("mode=%v, want seq215_js_hook_ui_integration_owner_definition", s["mode"])
	}
}

func TestSeq215P782JSOfflineFailOpenOwner(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p782","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_js_offline_fail_open_owner")
	if s["version"] != "s215-p782.v1" {
		t.Fatalf("version=%v, want s215-p782.v1", s["version"])
	}
	if s["owner"] != "js_runtime" {
		t.Fatalf("owner=%v, want js_runtime", s["owner"])
	}
	if s["responsibility"] != "offline_fail_open_fallback" {
		t.Fatalf("responsibility=%v, want offline_fail_open_fallback", s["responsibility"])
	}
	source := seq215ArchiveCenterJSSource(t)
	seq215RequireJSSourceContains(t, source,
		"return { source: \"backend-off\", fallback_reason: \"backend_off\", status: \"error\" }",
		"return { source: \"backend-error\", fallback_reason: \"backend_error\", status: \"error\" }",
		"backend off / timeout 시 fail-open으로 진행",
		"prepare-turn unavailable - local fallback continues:",
	)
	if strings.Contains(source, "return buildBlockedPayload(payload, backendBlock.userMessage || backendBlock.reason)") {
		t.Fatal("backend-off prepare-turn path must stay fail-open and must not return a blocked payload")
	}
	if s["mode"] != "seq215_js_offline_fail_open_owner_definition" {
		t.Fatalf("mode=%v, want seq215_js_offline_fail_open_owner_definition", s["mode"])
	}
}

func TestSeq215P783BuildInputContextPreserved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p783","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_build_input_context_preserved")
	if s["version"] != "s215-p783.v1" {
		t.Fatalf("version=%v, want s215-p783.v1", s["version"])
	}
	if s["function_name"] != "buildInputContext" {
		t.Fatalf("function_name=%v, want buildInputContext", s["function_name"])
	}
	if s["preserved"] != true {
		t.Fatalf("preserved=%v, want true", s["preserved"])
	}
	source := seq215ArchiveCenterJSSource(t)
	seq215RequireJSSourceContains(t, source, "function buildInputContext(")
	if s["mode"] != "seq215_build_input_context_preserved_definition" {
		t.Fatalf("mode=%v, want seq215_build_input_context_preserved_definition", s["mode"])
	}
}

func TestSeq215P784AssembleInjectionWithBudgetPreserved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p784","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_assemble_injection_with_budget_preserved")
	if s["version"] != "s215-p784.v1" {
		t.Fatalf("version=%v, want s215-p784.v1", s["version"])
	}
	if s["function_name"] != "assembleInjectionWithBudget" {
		t.Fatalf("function_name=%v, want assembleInjectionWithBudget", s["function_name"])
	}
	if s["preserved"] != true {
		t.Fatalf("preserved=%v, want true", s["preserved"])
	}
	source := seq215ArchiveCenterJSSource(t)
	seq215RequireJSSourceContains(t, source, "function assembleInjectionWithBudget(")
	if s["mode"] != "seq215_assemble_injection_with_budget_preserved_definition" {
		t.Fatalf("mode=%v, want seq215_assemble_injection_with_budget_preserved_definition", s["mode"])
	}
}

func TestSeq215P785ApplyContextInjectionPreserved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p785","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_apply_context_injection_preserved")
	if s["version"] != "s215-p785.v1" {
		t.Fatalf("version=%v, want s215-p785.v1", s["version"])
	}
	if s["function_name"] != "applyContextInjection" {
		t.Fatalf("function_name=%v, want applyContextInjection", s["function_name"])
	}
	if s["preserved"] != true {
		t.Fatalf("preserved=%v, want true", s["preserved"])
	}
	source := seq215ArchiveCenterJSSource(t)
	seq215RequireJSSourceContains(t, source, "function applyContextInjection(")
	if s["mode"] != "seq215_apply_context_injection_preserved_definition" {
		t.Fatalf("mode=%v, want seq215_apply_context_injection_preserved_definition", s["mode"])
	}
}

func TestSeq215P786TryPrepareTurnTakeoverOff(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p786","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_try_prepare_turn_takeover_off")
	if s["version"] != "s215-p786.v1" {
		t.Fatalf("version=%v, want s215-p786.v1", s["version"])
	}
	if s["function_name"] != "tryPrepareTurn" {
		t.Fatalf("function_name=%v, want tryPrepareTurn", s["function_name"])
	}
	if s["takeover_mode"] != "off" {
		t.Fatalf("takeover_mode=%v, want off", s["takeover_mode"])
	}
	if s["fixed"] != true {
		t.Fatalf("fixed=%v, want true", s["fixed"])
	}
	source := seq215ArchiveCenterJSSource(t)
	seq215RequireJSSourceContains(t, source,
		"async function tryPrepareTurn(",
		"takeover_mode: \"off\"",
	)
	if s["mode"] != "seq215_try_prepare_turn_takeover_off_definition" {
		t.Fatalf("mode=%v, want seq215_try_prepare_turn_takeover_off_definition", s["mode"])
	}
}

func TestSeq215P787JSNodeCheck(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p787","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_js_node_check")
	if s["version"] != "s215-p787.v1" {
		t.Fatalf("version=%v, want s215-p787.v1", s["version"])
	}
	if s["js_edited"] != false {
		t.Fatalf("js_edited=%v, want false", s["js_edited"])
	}
	if s["node_check"] != "pass" {
		t.Fatalf("node_check=%v, want pass", s["node_check"])
	}
	if s["mode"] != "seq215_js_node_check_definition" {
		t.Fatalf("mode=%v, want seq215_js_node_check_definition", s["mode"])
	}
}

func TestSeq215P788JSFocusedContractTests(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p788","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_js_focused_contract_tests")
	if s["version"] != "s215-p788.v1" {
		t.Fatalf("version=%v, want s215-p788.v1", s["version"])
	}
	if s["js_edited"] != false {
		t.Fatalf("js_edited=%v, want false", s["js_edited"])
	}
	if s["test_suite"] != "js-route-variant-smoke" {
		t.Fatalf("test_suite=%v, want js-route-variant-smoke", s["test_suite"])
	}
	if s["mode"] != "seq215_js_focused_contract_tests_definition" {
		t.Fatalf("mode=%v, want seq215_js_focused_contract_tests_definition", s["mode"])
	}
}

func TestSeq215P789ValidationRecord(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p789","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_p789_validation_record")
	if s["version"] != "s215-p789.v1" {
		t.Fatalf("version=%v, want s215-p789.v1", s["version"])
	}
	if s["record_type"] != "validation_output_and_changed_files" {
		t.Fatalf("record_type=%v, want validation_output_and_changed_files", s["record_type"])
	}
	if s["mode"] != "seq215_p789_validation_record_definition" {
		t.Fatalf("mode=%v, want seq215_p789_validation_record_definition", s["mode"])
	}
}

func TestSeq215P830RuntimeSplitStatus(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p830","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_runtime_split_status")
	if s["version"] != "s215-p830.v1" {
		t.Fatalf("version=%v, want s215-p830.v1", s["version"])
	}
	if s["runtime_split_status"] != "trace_contract_only" {
		t.Fatalf("runtime_split_status=%v, want trace_contract_only", s["runtime_split_status"])
	}
	if s["matches_reality"] != true {
		t.Fatalf("matches_reality=%v, want true", s["matches_reality"])
	}
	if s["mode"] != "seq215_runtime_split_status_definition" {
		t.Fatalf("mode=%v, want seq215_runtime_split_status_definition", s["mode"])
	}
}

func TestSeq215P831BackendBundleAssisted(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p831","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_backend_bundle_assisted")
	if s["version"] != "s215-p831.v1" {
		t.Fatalf("version=%v, want s215-p831.v1", s["version"])
	}
	if s["backend_exclusive"] != false {
		t.Fatalf("backend_exclusive=%v, want false", s["backend_exclusive"])
	}
	if s["mode"] != "seq215_backend_bundle_assisted_definition" {
		t.Fatalf("mode=%v, want seq215_backend_bundle_assisted_definition", s["mode"])
	}
}

func TestSeq215P832PluginOnlyModules(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p832","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_plugin_only_modules")
	if s["version"] != "s215-p832.v1" {
		t.Fatalf("version=%v, want s215-p832.v1", s["version"])
	}
	if s["ownership"] != "plugin_local_runtime" {
		t.Fatalf("ownership=%v, want plugin_local_runtime", s["ownership"])
	}
	if s["mode"] != "seq215_plugin_only_modules_definition" {
		t.Fatalf("mode=%v, want seq215_plugin_only_modules_definition", s["mode"])
	}
}

func TestSeq215P833OR1eWording(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p833","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_or1e_wording")
	if s["version"] != "s215-p833.v1" {
		t.Fatalf("version=%v, want s215-p833.v1", s["version"])
	}
	if s["claims_step_22"] != false {
		t.Fatalf("claims_step_22=%v, want false", s["claims_step_22"])
	}
	if s["claims_2_0_migration"] != false {
		t.Fatalf("claims_2_0_migration=%v, want false", s["claims_2_0_migration"])
	}
	if s["mode"] != "seq215_or1e_wording_definition" {
		t.Fatalf("mode=%v, want seq215_or1e_wording_definition", s["mode"])
	}
}

func TestSeq215P834OR1eNodeCheck(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p834","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_or1e_node_check")
	if s["version"] != "s215-p834.v1" {
		t.Fatalf("version=%v, want s215-p834.v1", s["version"])
	}
	if s["or1e_edited"] != false {
		t.Fatalf("or1e_edited=%v, want false", s["or1e_edited"])
	}
	if s["node_check"] != "pass" {
		t.Fatalf("node_check=%v, want pass", s["node_check"])
	}
	if s["mode"] != "seq215_or1e_node_check_definition" {
		t.Fatalf("mode=%v, want seq215_or1e_node_check_definition", s["mode"])
	}
}

func TestSeq215P835ValidationRecord(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p835","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_validation_record_p835")
	if s["version"] != "s215-p835.v1" {
		t.Fatalf("version=%v, want s215-p835.v1", s["version"])
	}
	if s["record_type"] != "validation_output_and_changed_files" {
		t.Fatalf("record_type=%v, want validation_output_and_changed_files", s["record_type"])
	}
	if s["mode"] != "seq215_p835_validation_record_definition" {
		t.Fatalf("mode=%v, want seq215_p835_validation_record_definition", s["mode"])
	}
}

func TestSeq215P869Phase1Complete(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p869","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_phase1_complete")
	if s["version"] != "s215-p869.v1" {
		t.Fatalf("version=%v, want s215-p869.v1", s["version"])
	}
	if s["phase"] != "1" {
		t.Fatalf("phase=%v, want 1", s["phase"])
	}
	if s["status"] != "complete" {
		t.Fatalf("status=%v, want complete", s["status"])
	}
	if s["mode"] != "seq215_phase1_complete_definition" {
		t.Fatalf("mode=%v, want seq215_phase1_complete_definition", s["mode"])
	}
}

func TestSeq215P870Phase2Complete(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p870","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_phase2_complete")
	if s["version"] != "s215-p870.v1" {
		t.Fatalf("version=%v, want s215-p870.v1", s["version"])
	}
	if s["phase"] != "2" {
		t.Fatalf("phase=%v, want 2", s["phase"])
	}
	if s["status"] != "complete" {
		t.Fatalf("status=%v, want complete", s["status"])
	}
	if s["mode"] != "seq215_phase2_complete_definition" {
		t.Fatalf("mode=%v, want seq215_phase2_complete_definition", s["mode"])
	}
}

func TestSeq215P871Phase3Complete(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p871","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_phase3_complete")
	if s["version"] != "s215-p871.v1" {
		t.Fatalf("version=%v, want s215-p871.v1", s["version"])
	}
	if s["phase"] != "3" {
		t.Fatalf("phase=%v, want 3", s["phase"])
	}
	if s["status"] != "complete" {
		t.Fatalf("status=%v, want complete", s["status"])
	}
	if s["mode"] != "seq215_phase3_complete_definition" {
		t.Fatalf("mode=%v, want seq215_phase3_complete_definition", s["mode"])
	}
}

func TestSeq215P872Phase4Complete(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p872","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_phase4_complete")
	if s["version"] != "s215-p872.v1" {
		t.Fatalf("version=%v, want s215-p872.v1", s["version"])
	}
	if s["phase"] != "4" {
		t.Fatalf("phase=%v, want 4", s["phase"])
	}
	if s["status"] != "complete" {
		t.Fatalf("status=%v, want complete", s["status"])
	}
	if s["mode"] != "seq215_phase4_complete_definition" {
		t.Fatalf("mode=%v, want seq215_phase4_complete_definition", s["mode"])
	}
}

func TestSeq215P873ContextReadback(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p873","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_context_readback")
	if s["version"] != "s215-p873.v1" {
		t.Fatalf("version=%v, want s215-p873.v1", s["version"])
	}
	if s["document"] != "CONTEXT 21.5th step.md" {
		t.Fatalf("document=%v, want CONTEXT 21.5th step.md", s["document"])
	}
	if s["readback_status"] != "ok" {
		t.Fatalf("readback_status=%v, want ok", s["readback_status"])
	}
	if s["mode"] != "seq215_context_readback_definition" {
		t.Fatalf("mode=%v, want seq215_context_readback_definition", s["mode"])
	}
}

func TestSeq215P874ProgressReadback(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p874","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_progress_readback")
	if s["version"] != "s215-p874.v1" {
		t.Fatalf("version=%v, want s215-p874.v1", s["version"])
	}
	if s["document"] != "PROGRESS 21.5th step.md" {
		t.Fatalf("document=%v, want PROGRESS 21.5th step.md", s["document"])
	}
	if s["readback_status"] != "ok" {
		t.Fatalf("readback_status=%v, want ok", s["readback_status"])
	}
	if s["mode"] != "seq215_progress_readback_definition" {
		t.Fatalf("mode=%v, want seq215_progress_readback_definition", s["mode"])
	}
}

func TestSeq215P875StaleAuthoritySearch(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p875","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_stale_authority_search")
	if s["version"] != "s215-p875.v1" {
		t.Fatalf("version=%v, want s215-p875.v1", s["version"])
	}
	if s["search_result"] != "no_unresolved_stale_claims" {
		t.Fatalf("search_result=%v, want no_unresolved_stale_claims", s["search_result"])
	}
	if s["historical_mentions_reviewed"] != true {
		t.Fatalf("historical_mentions_reviewed=%v, want true", s["historical_mentions_reviewed"])
	}
	if s["current_authority_claim_clean"] != true {
		t.Fatalf("current_authority_claim_clean=%v, want true", s["current_authority_claim_clean"])
	}
	if s["mode"] != "seq215_stale_authority_search_definition" {
		t.Fatalf("mode=%v, want seq215_stale_authority_search_definition", s["mode"])
	}
}

func TestSeq215P876FalseBackendTreeSearch(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p876","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_false_backend_tree_search")
	if s["version"] != "s215-p876.v1" {
		t.Fatalf("version=%v, want s215-p876.v1", s["version"])
	}
	if s["search_result"] != "no_unresolved_false_landed_claims" {
		t.Fatalf("search_result=%v, want no_unresolved_false_landed_claims", s["search_result"])
	}
	if s["historical_backend_tree_mentions_present"] != true {
		t.Fatalf("historical_backend_tree_mentions_present=%v, want true", s["historical_backend_tree_mentions_present"])
	}
	if s["historical_backend_tree_mentions_classified"] != true {
		t.Fatalf("historical_backend_tree_mentions_classified=%v, want true", s["historical_backend_tree_mentions_classified"])
	}
	if s["active_remigration_track_marks_historical_drift"] != true {
		t.Fatalf("active_remigration_track_marks_historical_drift=%v, want true", s["active_remigration_track_marks_historical_drift"])
	}
	if s["mode"] != "seq215_false_backend_tree_search_definition" {
		t.Fatalf("mode=%v, want seq215_false_backend_tree_search_definition", s["mode"])
	}
}

func TestSeq215P877NoBackupDeployEdited(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p877","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_no_backup_deploy_edited")
	if s["version"] != "s215-p877.v1" {
		t.Fatalf("version=%v, want s215-p877.v1", s["version"])
	}
	if s["backup_deploy_edited"] != false {
		t.Fatalf("backup_deploy_edited=%v, want false", s["backup_deploy_edited"])
	}
	if s["package_folder_edited"] != false {
		t.Fatalf("package_folder_edited=%v, want false", s["package_folder_edited"])
	}
	if s["mode"] != "seq215_no_backup_deploy_edited_definition" {
		t.Fatalf("mode=%v, want seq215_no_backup_deploy_edited_definition", s["mode"])
	}
}

func TestSeq215P878ChangedFilesList(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p878","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_changed_files_list")
	if s["version"] != "s215-p878.v1" {
		t.Fatalf("version=%v, want s215-p878.v1", s["version"])
	}
	files := seq165Slice(t, s, "changed_files")
	for _, f := range []string{"group_turn_seq215_surfaces.go", "group_turn_seq215_test.go", "group_turn.go"} {
		if !sliceContains(files, f) {
			t.Fatalf("changed_files missing %q: %v", f, files)
		}
	}
	if s["mode"] != "seq215_changed_files_list_definition" {
		t.Fatalf("mode=%v, want seq215_changed_files_list_definition", s["mode"])
	}
}

func TestSeq215P879ValidationCommands(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p879","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_validation_commands")
	if s["version"] != "s215-p879.v1" {
		t.Fatalf("version=%v, want s215-p879.v1", s["version"])
	}
	if s["mode"] != "seq215_validation_commands_definition" {
		t.Fatalf("mode=%v, want seq215_validation_commands_definition", s["mode"])
	}
}

func TestSeq215P880AdditionalOwnerSplitBounded(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p880","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_additional_owner_split_bounded")
	if s["version"] != "s215-p880.v1" {
		t.Fatalf("version=%v, want s215-p880.v1", s["version"])
	}
	if s["beta_0_8_modified"] != false {
		t.Fatalf("beta_0_8_modified=%v, want false", s["beta_0_8_modified"])
	}
	if s["mode"] != "seq215_additional_owner_split_bounded_definition" {
		t.Fatalf("mode=%v, want seq215_additional_owner_split_bounded_definition", s["mode"])
	}
}

func TestSeq215P881JSBackendOffloadPluginOnly(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p881","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_js_backend_offload_plugin_only")
	if s["version"] != "s215-p881.v1" {
		t.Fatalf("version=%v, want s215-p881.v1", s["version"])
	}
	if s["claimed_offload"] != false {
		t.Fatalf("claimed_offload=%v, want false", s["claimed_offload"])
	}
	if s["actual_state"] != "plugin_only_deferral" {
		t.Fatalf("actual_state=%v, want plugin_only_deferral", s["actual_state"])
	}
	if s["mode"] != "seq215_js_backend_offload_plugin_only_definition" {
		t.Fatalf("mode=%v, want seq215_js_backend_offload_plugin_only_definition", s["mode"])
	}
}

func TestSeq215P882MasterChecklistOpenZero(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p882","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_master_checklist_open_zero")
	if s["version"] != "s215-p882.v1" {
		t.Fatalf("version=%v, want s215-p882.v1", s["version"])
	}
	if s["open_rows"] != float64(0) {
		t.Fatalf("open_rows=%v, want 0", s["open_rows"])
	}
	if s["mode"] != "seq215_master_checklist_open_zero_definition" {
		t.Fatalf("mode=%v, want seq215_master_checklist_open_zero_definition", s["mode"])
	}
}

func TestSeq215P883StepComplete(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq215-p883","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "seq215_step_complete_p883")
	if s["version"] != "s215-p883.v1" {
		t.Fatalf("version=%v, want s215-p883.v1", s["version"])
	}
	if s["step"] != "21.5" {
		t.Fatalf("step=%v, want 21.5", s["step"])
	}
	if s["status"] != "complete" {
		t.Fatalf("status=%v, want complete", s["status"])
	}
	if s["all_slices_have_evidence"] != true {
		t.Fatalf("all_slices_have_evidence=%v, want true", s["all_slices_have_evidence"])
	}
	if s["size_reduction_goal_met"] != true {
		t.Fatalf("size_reduction_goal_met=%v, want true", s["size_reduction_goal_met"])
	}
	if s["mode"] != "seq215_step_complete_definition" {
		t.Fatalf("mode=%v, want seq215_step_complete_definition", s["mode"])
	}
}
