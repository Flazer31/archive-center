package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSeq19P68BoundedWeekMonthWriteGuard(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p68","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "bounded_week_month_write_guard")
	if s["version"] != "s19-p68.v1" {
		t.Fatalf("version=%v, want s19-p68.v1", s["version"])
	}
	if s["bounded_week_write_lane"] != "carry_forward_only" {
		t.Fatalf("bounded_week_write_lane=%v, want carry_forward_only", s["bounded_week_write_lane"])
	}
	if s["bounded_month_write_lane"] != "carry_forward_only" {
		t.Fatalf("bounded_month_write_lane=%v, want carry_forward_only", s["bounded_month_write_lane"])
	}
	if s["current_scene_write_blocked"] != true {
		t.Fatalf("current_scene_write_blocked=%v, want true", s["current_scene_write_blocked"])
	}
}

func TestSeq19P69RegressionBundle(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p69","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "step18_plus_19_regression_bundle_69")
	if s["version"] != "s19-p69.v1" {
		t.Fatalf("version=%v, want s19-p69.v1", s["version"])
	}
	if s["regression_status"] != "green" {
		t.Fatalf("regression_status=%v, want green", s["regression_status"])
	}
	if s["replay_slice"] != "19-4a_landed" {
		t.Fatalf("replay_slice=%v, want 19-4a_landed", s["replay_slice"])
	}
}

func TestSeq19P78MixedLanePrecedenceContract(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p78","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "mixed_lane_precedence_contract")
	if s["version"] != "s19-p78.v1" {
		t.Fatalf("version=%v, want s19-p78.v1", s["version"])
	}
	if s["precedence_rule"] != "current_scene_over_recalled_past" {
		t.Fatalf("precedence_rule=%v, want current_scene_over_recalled_past", s["precedence_rule"])
	}
	if s["effective_write_lane"] != "current_scene" {
		t.Fatalf("effective_write_lane=%v, want current_scene", s["effective_write_lane"])
	}
	if s["overwrite_protection"] != true {
		t.Fatalf("overwrite_protection=%v, want true", s["overwrite_protection"])
	}
	if s["downgrade_protection"] != true {
		t.Fatalf("downgrade_protection=%v, want true", s["downgrade_protection"])
	}
	if s["mode"] != "mixed_lane_precedence_contract_definition" {
		t.Fatalf("mode=%v, want mixed_lane_precedence_contract_definition", s["mode"])
	}
}

func TestSeq19P79MixedLaneReplayCases(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p79","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "mixed_lane_replay_cases")
	if s["version"] != "s19-p79.v1" {
		t.Fatalf("version=%v, want s19-p79.v1", s["version"])
	}
	cases, ok := s["replay_cases"].([]any)
	if !ok || len(cases) != 2 {
		t.Fatalf("replay_cases count=%v, want 2", len(cases))
	}
	byCase := map[string]map[string]any{}
	for _, item := range cases {
		entry, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("replay case is not an object: %T", item)
		}
		c, _ := entry["case"].(string)
		byCase[c] = entry
	}

	anchorCase := byCase["commit_current_scene_anchor"]
	if anchorCase == nil {
		t.Fatalf("commit_current_scene_anchor case missing")
	}
	if anchorCase["current_scene_anchor"] != "today" {
		t.Fatalf("current_scene_anchor=%v, want today", anchorCase["current_scene_anchor"])
	}
	if anchorCase["recalled_past"] != "yesterday" {
		t.Fatalf("recalled_past=%v, want yesterday", anchorCase["recalled_past"])
	}
	if anchorCase["expected_write_lane"] != "current_scene" {
		t.Fatalf("expected_write_lane=%v, want current_scene", anchorCase["expected_write_lane"])
	}
	if anchorCase["expected_action"] != "no_advance" {
		t.Fatalf("expected_action=%v, want no_advance", anchorCase["expected_action"])
	}

	advanceCase := byCase["commit_explicit_advance"]
	if advanceCase == nil {
		t.Fatalf("commit_explicit_advance case missing")
	}
	if advanceCase["current_scene_anchor"] != "tomorrow" {
		t.Fatalf("current_scene_anchor=%v, want tomorrow", advanceCase["current_scene_anchor"])
	}
	if advanceCase["recalled_past"] != "yesterday" {
		t.Fatalf("recalled_past=%v, want yesterday", advanceCase["recalled_past"])
	}
	if advanceCase["expected_write_lane"] != "current_scene" {
		t.Fatalf("expected_write_lane=%v, want current_scene", advanceCase["expected_write_lane"])
	}
	if advanceCase["expected_action"] != "advance" {
		t.Fatalf("expected_action=%v, want advance", advanceCase["expected_action"])
	}
}

func TestSeq19P80MixedLaneSplitRuleOutcome(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p80","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "mixed_lane_split_rule_outcome")
	if s["version"] != "s19-p80.v1" {
		t.Fatalf("version=%v, want s19-p80.v1", s["version"])
	}
	if s["current_scene_write_allowed"] != true {
		t.Fatalf("current_scene_write_allowed=%v, want true", s["current_scene_write_allowed"])
	}
	if s["effective_write_lane"] != "current_scene" {
		t.Fatalf("effective_write_lane=%v, want current_scene", s["effective_write_lane"])
	}
	if s["recalled_past_target_kind"] != "recalled_event" {
		t.Fatalf("recalled_past_target_kind=%v, want recalled_event", s["recalled_past_target_kind"])
	}
	if s["recalled_past_preserved"] != true {
		t.Fatalf("recalled_past_preserved=%v, want true", s["recalled_past_preserved"])
	}
	if s["current_scene_authority_overwrite_blocked"] != true {
		t.Fatalf("current_scene_authority_overwrite_blocked=%v, want true", s["current_scene_authority_overwrite_blocked"])
	}
	if s["current_scene_authority_downgrade_blocked"] != true {
		t.Fatalf("current_scene_authority_downgrade_blocked=%v, want true", s["current_scene_authority_downgrade_blocked"])
	}
	if s["mode"] != "mixed_lane_split_rule_outcome_definition" {
		t.Fatalf("mode=%v, want mixed_lane_split_rule_outcome_definition", s["mode"])
	}
}

func TestSeq19P81RegressionBundle(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p81","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "step18_plus_19_regression_bundle_81")
	if s["version"] != "s19-p81.v1" {
		t.Fatalf("version=%v, want s19-p81.v1", s["version"])
	}
	if s["regression_status"] != "green" {
		t.Fatalf("regression_status=%v, want green", s["regression_status"])
	}
	if s["replay_slice"] != "19-4b_landed" {
		t.Fatalf("replay_slice=%v, want 19-4b_landed", s["replay_slice"])
	}
}

func TestSeq19P90MissingAnchorDegradeContract(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p90","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "missing_anchor_degrade_contract")
	if s["version"] != "s19-p90.v1" {
		t.Fatalf("version=%v, want s19-p90.v1", s["version"])
	}
	if s["missing_anchor_degrade"] != true {
		t.Fatalf("missing_anchor_degrade=%v, want true", s["missing_anchor_degrade"])
	}
	if s["low_precision_degrade"] != true {
		t.Fatalf("low_precision_degrade=%v, want true", s["low_precision_degrade"])
	}
	if s["degrade_to_unresolved"] != true {
		t.Fatalf("degrade_to_unresolved=%v, want true", s["degrade_to_unresolved"])
	}
	if s["degrade_to_carry_forward"] != true {
		t.Fatalf("degrade_to_carry_forward=%v, want true", s["degrade_to_carry_forward"])
	}
	if s["no_fake_anchored_certainty"] != true {
		t.Fatalf("no_fake_anchored_certainty=%v, want true", s["no_fake_anchored_certainty"])
	}
	if s["mode"] != "missing_anchor_degrade_contract_definition" {
		t.Fatalf("mode=%v, want missing_anchor_degrade_contract_definition", s["mode"])
	}
}

func TestSeq19P91MissingAnchorExactPhraseDegrade(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p91","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "missing_anchor_exact_phrase_degrade")
	if s["version"] != "s19-p91.v1" {
		t.Fatalf("version=%v, want s19-p91.v1", s["version"])
	}
	if s["phrase_example"] != "yesterday" {
		t.Fatalf("phrase_example=%v, want yesterday", s["phrase_example"])
	}
	if s["phrase_example_ko"] != "어제" {
		t.Fatalf("phrase_example_ko=%v, want 어제", s["phrase_example_ko"])
	}
	whenAbsent := seq165Map(t, s, "when_clock_absent")
	if whenAbsent["status"] != "unresolved" {
		t.Fatalf("status=%v, want unresolved", whenAbsent["status"])
	}
	if whenAbsent["range_kind"] != "unresolved" {
		t.Fatalf("range_kind=%v, want unresolved", whenAbsent["range_kind"])
	}
	if whenAbsent["anchor_resolution_status"] != "carry_forward" {
		t.Fatalf("anchor_resolution_status=%v, want carry_forward", whenAbsent["anchor_resolution_status"])
	}
	if whenAbsent["write_lane"] != "carry_forward" {
		t.Fatalf("write_lane=%v, want carry_forward", whenAbsent["write_lane"])
	}
	if whenAbsent["fabricated_certainty"] != false {
		t.Fatalf("fabricated_certainty=%v, want false", whenAbsent["fabricated_certainty"])
	}
}

func TestSeq19P92LowPrecisionRecalledRelationGuard(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p92","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "low_precision_recalled_relation_guard")
	if s["version"] != "s19-p92.v1" {
		t.Fatalf("version=%v, want s19-p92.v1", s["version"])
	}
	if s["phrase_example"] != "last winter" {
		t.Fatalf("phrase_example=%v, want last winter", s["phrase_example"])
	}
	if s["anchored_precision"] != "coarse" {
		t.Fatalf("anchored_precision=%v, want coarse", s["anchored_precision"])
	}
	if s["flatten_to_exact_blocked"] != true {
		t.Fatalf("flatten_to_exact_blocked=%v, want true", s["flatten_to_exact_blocked"])
	}
	if s["current_scene_write_blocked"] != true {
		t.Fatalf("current_scene_write_blocked=%v, want true", s["current_scene_write_blocked"])
	}
	if s["requires_scene_progression_evidence"] != true {
		t.Fatalf("requires_scene_progression_evidence=%v, want true", s["requires_scene_progression_evidence"])
	}
	if s["mode"] != "low_precision_recalled_relation_guard_definition" {
		t.Fatalf("mode=%v, want low_precision_recalled_relation_guard_definition", s["mode"])
	}
}

func TestSeq19P93RegressionBundle(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p93","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "step18_plus_19_regression_bundle_93")
	if s["version"] != "s19-p93.v1" {
		t.Fatalf("version=%v, want s19-p93.v1", s["version"])
	}
	if s["regression_status"] != "green" {
		t.Fatalf("regression_status=%v, want green", s["regression_status"])
	}
	if s["replay_slice"] != "19-4c_landed" {
		t.Fatalf("replay_slice=%v, want 19-4c_landed", s["replay_slice"])
	}
}

func TestSeq19P102TemporalPacketTruthBoundaryContract(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p102","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "temporal_packet_truth_boundary_contract")
	if s["version"] != "s19-p102.v1" {
		t.Fatalf("version=%v, want s19-p102.v1", s["version"])
	}
	if s["owner_path"] != "backend_packet_builder" {
		t.Fatalf("owner_path=%v, want backend_packet_builder", s["owner_path"])
	}
	if s["precedence_explicit"] != true {
		t.Fatalf("precedence_explicit=%v, want true", s["precedence_explicit"])
	}
	if s["no_implicit_generic_summary"] != true {
		t.Fatalf("no_implicit_generic_summary=%v, want true", s["no_implicit_generic_summary"])
	}
	if s["packet_built_backend_first"] != true {
		t.Fatalf("packet_built_backend_first=%v, want true", s["packet_built_backend_first"])
	}
	if s["js_consumes_passive_only"] != true {
		t.Fatalf("js_consumes_passive_only=%v, want true", s["js_consumes_passive_only"])
	}
	if s["mode"] != "temporal_packet_truth_boundary_contract_definition" {
		t.Fatalf("mode=%v, want temporal_packet_truth_boundary_contract_definition", s["mode"])
	}
}

func TestSeq19P103TemporalPacketMixedPrecedence(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p103","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "temporal_packet_mixed_precedence")
	if s["version"] != "s19-p103.v1" {
		t.Fatalf("version=%v, want s19-p103.v1", s["version"])
	}
	mixed, ok := s["mixed_case"].(map[string]any)
	if !ok {
		t.Fatalf("mixed_case missing or not map")
	}
	if mixed["current_scene_anchor"] != "today" {
		t.Fatalf("current_scene_anchor=%v, want today", mixed["current_scene_anchor"])
	}
	if mixed["recalled_past"] != "어제" {
		t.Fatalf("recalled_past=%v, want 어제", mixed["recalled_past"])
	}
	if mixed["recalled_past_en"] != "yesterday" {
		t.Fatalf("recalled_past_en=%v, want yesterday", mixed["recalled_past_en"])
	}
	clockSummary, ok := s["clock_summary"].(map[string]any)
	if !ok {
		t.Fatalf("clock_summary missing or not map")
	}
	if clockSummary["day"] != float64(18) {
		t.Fatalf("clock_summary.day=%v, want 18", clockSummary["day"])
	}
	if clockSummary["daypart"] != "morning" {
		t.Fatalf("clock_summary.daypart=%v, want morning", clockSummary["daypart"])
	}
	if clockSummary["precision"] != "daypart" {
		t.Fatalf("clock_summary.precision=%v, want daypart", clockSummary["precision"])
	}
	ws, ok := s["write_summary"].(map[string]any)
	if !ok {
		t.Fatalf("write_summary missing or not map")
	}
	if ws["lane"] != "current_scene" {
		t.Fatalf("write_summary.lane=%v, want current_scene", ws["lane"])
	}
	rel, ok := s["relation_samples"].([]any)
	if !ok || len(rel) != 2 {
		t.Fatalf("relation_samples missing or len != 2")
	}
	current, ok := rel[0].(map[string]any)
	if !ok {
		t.Fatalf("relation_samples[0] not map")
	}
	if current["kind"] != "current" || current["value"] != "today" {
		t.Fatalf("relation_samples[0]=%v, want current=today", current)
	}
	other, ok := rel[1].(map[string]any)
	if !ok {
		t.Fatalf("relation_samples[1] not map")
	}
	if other["kind"] != "other" || other["value"] != "어제" || other["target_kind"] != "recalled_event" {
		t.Fatalf("relation_samples[1]=%v, want other=어제<recalled_event>", other)
	}
	if s["mode"] != "temporal_packet_mixed_precedence_definition" {
		t.Fatalf("mode=%v, want temporal_packet_mixed_precedence_definition", s["mode"])
	}
}

func TestSeq19P104TemporalPacketClockMissingBoundary(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p104","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "temporal_packet_clock_missing_boundary")
	if s["version"] != "s19-p104.v1" {
		t.Fatalf("version=%v, want s19-p104.v1", s["version"])
	}
	if s["case"] != "clock_missing_lone_recall" {
		t.Fatalf("case=%v, want clock_missing_lone_recall", s["case"])
	}
	if s["recalled_past"] != "어제" {
		t.Fatalf("recalled_past=%v, want 어제", s["recalled_past"])
	}
	if s["recalled_past_en"] != "yesterday" {
		t.Fatalf("recalled_past_en=%v, want yesterday", s["recalled_past_en"])
	}
	clockSeg, ok := s["clock_segment"].(map[string]any)
	if !ok {
		t.Fatalf("clock_segment missing or not map")
	}
	if clockSeg["precision"] != "unknown" {
		t.Fatalf("clock_segment.precision=%v, want unknown", clockSeg["precision"])
	}
	writeSeg, ok := s["write_segment"].(map[string]any)
	if !ok {
		t.Fatalf("write_segment missing or not map")
	}
	if writeSeg["lane"] != "carry_forward" {
		t.Fatalf("write_segment.lane=%v, want carry_forward", writeSeg["lane"])
	}
	rel, ok := s["relation_sample"].(map[string]any)
	if !ok {
		t.Fatalf("relation_sample missing or not map")
	}
	if rel["status"] != "unresolved" {
		t.Fatalf("relation_sample.status=%v, want unresolved", rel["status"])
	}
	if rel["value"] != "어제" {
		t.Fatalf("relation_sample.value=%v, want 어제", rel["value"])
	}
	if rel["target_kind"] != "recalled_event" {
		t.Fatalf("relation_sample.target_kind=%v, want recalled_event", rel["target_kind"])
	}
	if s["no_fabricated_day_index"] != true {
		t.Fatalf("no_fabricated_day_index=%v, want true", s["no_fabricated_day_index"])
	}
	if s["mode"] != "temporal_packet_clock_missing_boundary_definition" {
		t.Fatalf("mode=%v, want temporal_packet_clock_missing_boundary_definition", s["mode"])
	}
}

func TestSeq19P105RegressionBundle(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p105","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "step18_plus_19_regression_bundle_105")
	if s["version"] != "s19-p105.v1" {
		t.Fatalf("version=%v, want s19-p105.v1", s["version"])
	}
	if s["regression_status"] != "green" {
		t.Fatalf("regression_status=%v, want green", s["regression_status"])
	}
	if s["replay_slice"] != "19-4d_landed" {
		t.Fatalf("replay_slice=%v, want 19-4d_landed", s["replay_slice"])
	}
}

func TestSeq19P114ValidatorHelperClusterContract(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p114","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "step19_validator_helper_cluster_contract")
	if s["version"] != "s19-p114.v1" {
		t.Fatalf("version=%v, want s19-p114.v1", s["version"])
	}
	helpers, ok := s["helpers_present"].([]any)
	if !ok || len(helpers) != 3 {
		t.Fatalf("helpers_present missing or len != 3")
	}
	if s["implementation_not_replay_only"] != true {
		t.Fatalf("implementation_not_replay_only=%v, want true", s["implementation_not_replay_only"])
	}
	if s["active_runtime_file"] != "Archive Center 2.0/Archive Center.js" {
		t.Fatalf("active_runtime_file=%v, want Archive Center 2.0/Archive Center.js", s["active_runtime_file"])
	}
	if s["mode"] != "step19_validator_helper_cluster_contract_definition" {
		t.Fatalf("mode=%v, want step19_validator_helper_cluster_contract_definition", s["mode"])
	}
}

func TestSeq19P115TemporalPrecedenceResolutionOrder(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p115","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "temporal_precedence_resolution_order")
	if s["version"] != "s19-p115.v1" {
		t.Fatalf("version=%v, want s19-p115.v1", s["version"])
	}
	order, ok := s["resolution_order"].([]any)
	if !ok || len(order) != 4 {
		t.Fatalf("resolution_order missing or len != 4")
	}
	wantOrder := []string{"session_state_clock", "input_current_scene_anchor", "timeline_anchor", "carry_forward"}
	for i, w := range wantOrder {
		if order[i] != w {
			t.Fatalf("resolution_order[%d]=%v, want %s", i, order[i], w)
		}
	}
	if s["validation_basis"] != "current_story_clock + temporal_relation_ledger" {
		t.Fatalf("validation_basis=%v, want current_story_clock + temporal_relation_ledger", s["validation_basis"])
	}
	if s["ignore_latest_timestamp_shortcut"] != true {
		t.Fatalf("ignore_latest_timestamp_shortcut=%v, want true", s["ignore_latest_timestamp_shortcut"])
	}
	if s["mode"] != "temporal_precedence_resolution_order_definition" {
		t.Fatalf("mode=%v, want temporal_precedence_resolution_order_definition", s["mode"])
	}
}

func TestSeq19P116TemporalDeicticWarningClasses(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p116","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "temporal_deictic_warning_classes")
	if s["version"] != "s19-p116.v1" {
		t.Fatalf("version=%v, want s19-p116.v1", s["version"])
	}
	classes, ok := s["warning_classes"].([]any)
	if !ok || len(classes) != 3 {
		t.Fatalf("warning_classes missing or len != 3")
	}
	wantClasses := []string{"current_scene_deictic_mismatch", "relation_only_promoted_to_current_scene", "exact_current_scene_without_resolved_clock"}
	for i, w := range wantClasses {
		if classes[i] != w {
			t.Fatalf("warning_classes[%d]=%v, want %s", i, classes[i], w)
		}
	}
	if s["mode"] != "temporal_deictic_warning_classes_definition" {
		t.Fatalf("mode=%v, want temporal_deictic_warning_classes_definition", s["mode"])
	}
}

func TestSeq19P117TemporalDeicticTraceOnlyWarningSurface(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p117","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "temporal_deictic_trace_only_warning_surface")
	if s["version"] != "s19-p117.v1" {
		t.Fatalf("version=%v, want s19-p117.v1", s["version"])
	}
	if s["trace_key"] != "temporalDeicticValidation" {
		t.Fatalf("trace_key=%v, want temporalDeicticValidation", s["trace_key"])
	}
	if s["trace_only"] != true {
		t.Fatalf("trace_only=%v, want true", s["trace_only"])
	}
	if s["blocks_response_delivery"] != false {
		t.Fatalf("blocks_response_delivery=%v, want false", s["blocks_response_delivery"])
	}
	if s["blocks_save"] != false {
		t.Fatalf("blocks_save=%v, want false", s["blocks_save"])
	}
	if s["blocks_critic"] != false {
		t.Fatalf("blocks_critic=%v, want false", s["blocks_critic"])
	}
	if s["mode"] != "temporal_deictic_trace_only_warning_surface_definition" {
		t.Fatalf("mode=%v, want temporal_deictic_trace_only_warning_surface_definition", s["mode"])
	}
}

func TestSeq19P125TemporalClassificationWriteDisciplineSurface(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p125","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "temporal_classification_write_discipline_surface")
	if s["version"] != "s19-p125.v1" {
		t.Fatalf("version=%v, want s19-p125.v1", s["version"])
	}
	if s["thin_precedence_marker_only"] != false {
		t.Fatalf("thin_precedence_marker_only=%v, want false", s["thin_precedence_marker_only"])
	}
	if s["inspectable_policy"] != true {
		t.Fatalf("inspectable_policy=%v, want true", s["inspectable_policy"])
	}
	if s["mode"] != "temporal_classification_write_discipline_surface_definition" {
		t.Fatalf("mode=%v, want temporal_classification_write_discipline_surface_definition", s["mode"])
	}
}

func TestSeq19P126TemporalClassificationExceptions(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p126","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "temporal_classification_exceptions")
	if s["version"] != "s19-p126.v1" {
		t.Fatalf("version=%v, want s19-p126.v1", s["version"])
	}
	exc, ok := s["exceptions"].([]any)
	if !ok || len(exc) != 3 {
		t.Fatalf("exceptions missing or len != 3")
	}
	wantKinds := []string{"planned_event", "recalled_event", "figurative_duration"}
	for i, w := range wantKinds {
		m, ok := exc[i].(map[string]any)
		if !ok || m["kind"] != w {
			t.Fatalf("exceptions[%d].kind=%v, want %s", i, m["kind"], w)
		}
	}
	if s["mode"] != "temporal_classification_exceptions_definition" {
		t.Fatalf("mode=%v, want temporal_classification_exceptions_definition", s["mode"])
	}
}

func TestSeq19P127TemporalWriteDisciplineRules(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p127","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "temporal_write_discipline_rules")
	if s["version"] != "s19-p127.v1" {
		t.Fatalf("version=%v, want s19-p127.v1", s["version"])
	}
	if s["planned_future_rule"] != "block_relation_only_write" {
		t.Fatalf("planned_future_rule=%v, want block_relation_only_write", s["planned_future_rule"])
	}
	if s["recalled_past_rule"] != "block_relation_only_write" {
		t.Fatalf("recalled_past_rule=%v, want block_relation_only_write", s["recalled_past_rule"])
	}
	if s["figurative_duration_rule"] != "figurative_duration_excluded" {
		t.Fatalf("figurative_duration_rule=%v, want figurative_duration_excluded", s["figurative_duration_rule"])
	}
	if s["block_figurative_only_write"] != true {
		t.Fatalf("block_figurative_only_write=%v, want true", s["block_figurative_only_write"])
	}
	if s["mode"] != "temporal_write_discipline_rules_definition" {
		t.Fatalf("mode=%v, want temporal_write_discipline_rules_definition", s["mode"])
	}
}

func TestSeq19P128TemporalRelationEntryMetadataSurface(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p128","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "temporal_relation_entry_metadata_surface")
	if s["version"] != "s19-p128.v1" {
		t.Fatalf("version=%v, want s19-p128.v1", s["version"])
	}
	fields, ok := s["entry_fields"].([]any)
	if !ok || len(fields) != 5 {
		t.Fatalf("entry_fields missing or len != 5")
	}
	wantFields := []string{"status", "rangeKind", "sourceTurn", "validFromTurn", "validToTurn"}
	for i, w := range wantFields {
		if fields[i] != w {
			t.Fatalf("entry_fields[%d]=%v, want %s", i, fields[i], w)
		}
	}
	if s["no_fake_precision"] != true {
		t.Fatalf("no_fake_precision=%v, want true", s["no_fake_precision"])
	}
	if s["mode"] != "temporal_relation_entry_metadata_surface_definition" {
		t.Fatalf("mode=%v, want temporal_relation_entry_metadata_surface_definition", s["mode"])
	}
}

func TestSeq19P137LocaleAwareExtractorOwnerBlock(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p137","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "locale_aware_extractor_owner_block")
	if s["version"] != "s19-p137.v1" {
		t.Fatalf("version=%v, want s19-p137.v1", s["version"])
	}
	locales, ok := s["supported_locales"].([]any)
	if !ok || len(locales) != 4 {
		t.Fatalf("supported_locales missing or len != 4")
	}
	wantLocales := []string{"ko", "en", "ja", "zh"}
	for i, w := range wantLocales {
		if locales[i] != w {
			t.Fatalf("supported_locales[%d]=%v, want %s", i, locales[i], w)
		}
	}
	if s["same_contract_all_locales"] != true {
		t.Fatalf("same_contract_all_locales=%v, want true", s["same_contract_all_locales"])
	}
	if s["fail_open_mixed_input"] != true {
		t.Fatalf("fail_open_mixed_input=%v, want true", s["fail_open_mixed_input"])
	}
	if s["mode"] != "locale_aware_extractor_owner_block_definition" {
		t.Fatalf("mode=%v, want locale_aware_extractor_owner_block_definition", s["mode"])
	}
}

func TestSeq19P138RecalledPastParitySurface(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p138","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "recalled_past_parity_surface")
	if s["version"] != "s19-p138.v1" {
		t.Fatalf("version=%v, want s19-p138.v1", s["version"])
	}
	if s["canonical_signature"] != "recalled_event_exact_day_minus1" {
		t.Fatalf("canonical_signature=%v, want recalled_event_exact_day_minus1", s["canonical_signature"])
	}
	if s["canonical_offset_min"] != float64(-1) {
		t.Fatalf("canonical_offset_min=%v, want -1", s["canonical_offset_min"])
	}
	if s["canonical_offset_max"] != float64(-1) {
		t.Fatalf("canonical_offset_max=%v, want -1", s["canonical_offset_max"])
	}
	if s["canonical_offset_unit"] != "day" {
		t.Fatalf("canonical_offset_unit=%v, want day", s["canonical_offset_unit"])
	}
	if s["canonical_precision"] != "exact" {
		t.Fatalf("canonical_precision=%v, want exact", s["canonical_precision"])
	}
	variants, ok := s["variants"].([]any)
	if !ok || len(variants) != 4 {
		t.Fatalf("variants missing or len != 4")
	}
	wantTexts := map[string]string{"ko": "어제", "en": "yesterday", "ja": "昨日", "zh": "昨天"}
	for _, v := range variants {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		locale := m["locale"].(string)
		if wantTexts[locale] != m["text"] {
			t.Fatalf("variant locale=%s text=%v, want %s", locale, m["text"], wantTexts[locale])
		}
	}
	if s["mode"] != "recalled_past_parity_surface_definition" {
		t.Fatalf("mode=%v, want recalled_past_parity_surface_definition", s["mode"])
	}
}

func TestSeq19P139CurrentSceneNextMorningParitySurface(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p139","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "current_scene_next_morning_parity_surface")
	if s["version"] != "s19-p139.v1" {
		t.Fatalf("version=%v, want s19-p139.v1", s["version"])
	}
	if s["canonical_signature"] != "current_scene_daypart_advance_plus1" {
		t.Fatalf("canonical_signature=%v, want current_scene_daypart_advance_plus1", s["canonical_signature"])
	}
	if s["canonical_offset_min"] != float64(1) {
		t.Fatalf("canonical_offset_min=%v, want 1", s["canonical_offset_min"])
	}
	if s["canonical_offset_max"] != float64(1) {
		t.Fatalf("canonical_offset_max=%v, want 1", s["canonical_offset_max"])
	}
	if s["canonical_offset_unit"] != "day" {
		t.Fatalf("canonical_offset_unit=%v, want day", s["canonical_offset_unit"])
	}
	if s["canonical_precision"] != "daypart" {
		t.Fatalf("canonical_precision=%v, want daypart", s["canonical_precision"])
	}
	if s["canonical_daypart"] != "morning" {
		t.Fatalf("canonical_daypart=%v, want morning", s["canonical_daypart"])
	}
	variants, ok := s["variants"].([]any)
	if !ok || len(variants) != 4 {
		t.Fatalf("variants missing or len != 4")
	}
	wantTexts := map[string]string{"ko": "다음날 아침", "en": "the next morning", "ja": "翌朝", "zh": "第二天早上"}
	for _, v := range variants {
		m, ok := v.(map[string]any)
		if !ok {
			continue
		}
		locale := m["locale"].(string)
		if wantTexts[locale] != m["text"] {
			t.Fatalf("variant locale=%s text=%v, want %s", locale, m["text"], wantTexts[locale])
		}
	}
	if s["mode"] != "current_scene_next_morning_parity_surface_definition" {
		t.Fatalf("mode=%v, want current_scene_next_morning_parity_surface_definition", s["mode"])
	}
}

func TestSeq19P140ActiveLocalesFailOpenGatingContract(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p140","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "active_locales_fail_open_gating_contract")
	if s["version"] != "s19-p140.v1" {
		t.Fatalf("version=%v, want s19-p140.v1", s["version"])
	}
	if s["gate_behavior"] != "activeLocales filters extraction path" {
		t.Fatalf("gate_behavior=%v, want activeLocales filters extraction path", s["gate_behavior"])
	}
	if s["mixed_language_behavior"] != "fail_open" {
		t.Fatalf("mixed_language_behavior=%v, want fail_open", s["mixed_language_behavior"])
	}
	if s["unsupported_phrase_action"] != "ignore" {
		t.Fatalf("unsupported_phrase_action=%v, want ignore", s["unsupported_phrase_action"])
	}
	if s["no_hallucination"] != true {
		t.Fatalf("no_hallucination=%v, want true", s["no_hallucination"])
	}
	locales, ok := s["default_locales"].([]any)
	if !ok || len(locales) != 4 {
		t.Fatalf("default_locales missing or len != 4")
	}
	wantLocales := []string{"ko", "en", "ja", "zh"}
	for i, w := range wantLocales {
		if locales[i] != w {
			t.Fatalf("default_locales[%d]=%v, want %s", i, locales[i], w)
		}
	}
	if s["mode"] != "active_locales_fail_open_gating_contract_definition" {
		t.Fatalf("mode=%v, want active_locales_fail_open_gating_contract_definition", s["mode"])
	}
}

func TestSeq19P288CurrentTimeExplicitnessContract(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p288","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "current_time_explicitness_contract")
	if s["version"] != "s19-p288.v1" {
		t.Fatalf("version=%v, want s19-p288.v1", s["version"])
	}
	if s["hidden_guess_forbidden"] != true {
		t.Fatalf("hidden_guess_forbidden=%v, want true", s["hidden_guess_forbidden"])
	}
	if s["implicit_story_day_index_0"] != false {
		t.Fatalf("implicit_story_day_index_0=%v, want false", s["implicit_story_day_index_0"])
	}
	allowed, ok := s["allowed_states"].([]any)
	if !ok || len(allowed) != 3 {
		t.Fatalf("allowed_states=%v, want 3 items", s["allowed_states"])
	}
	if s["mode"] != "current_time_explicitness_contract_definition" {
		t.Fatalf("mode=%v, want current_time_explicitness_contract_definition", s["mode"])
	}
}
