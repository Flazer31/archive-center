package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
