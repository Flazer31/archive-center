package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSeq19P289AnchorBoundRelationContract(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p289","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "anchor_bound_relation_contract")
	if s["version"] != "s19-p289.v1" {
		t.Fatalf("version=%v, want s19-p289.v1", s["version"])
	}
	if s["anchor_required"] != true {
		t.Fatalf("anchor_required=%v, want true", s["anchor_required"])
	}
	if s["source_turn_linked"] != true {
		t.Fatalf("source_turn_linked=%v, want true", s["source_turn_linked"])
	}
	if s["anchor_ref_linked"] != true {
		t.Fatalf("anchor_ref_linked=%v, want true", s["anchor_ref_linked"])
	}
	if s["mixing_blocked"] != true {
		t.Fatalf("mixing_blocked=%v, want true", s["mixing_blocked"])
	}
	if s["mode"] != "anchor_bound_relation_contract_definition" {
		t.Fatalf("mode=%v, want anchor_bound_relation_contract_definition", s["mode"])
	}
}

func TestSeq19P290BoundedAmbiguityContract(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p290","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "bounded_ambiguity_contract")
	if s["version"] != "s19-p290.v1" {
		t.Fatalf("version=%v, want s19-p290.v1", s["version"])
	}
	if s["exact_day_forge_blocked"] != true {
		t.Fatalf("exact_day_forge_blocked=%v, want true", s["exact_day_forge_blocked"])
	}
	labels, ok := s["preserve_labels"].([]any)
	if !ok || len(labels) != 3 {
		t.Fatalf("preserve_labels=%v, want 3 items", s["preserve_labels"])
	}
	phrases, ok := s["example_phrases"].([]any)
	if !ok || len(phrases) != 3 {
		t.Fatalf("example_phrases=%v, want 3 items", s["example_phrases"])
	}
	if s["mode"] != "bounded_ambiguity_contract_definition" {
		t.Fatalf("mode=%v, want bounded_ambiguity_contract_definition", s["mode"])
	}
}

func TestSeq19P291AdvanceDisciplineContract(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p291","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "advance_discipline_contract")
	if s["version"] != "s19-p291.v1" {
		t.Fatalf("version=%v, want s19-p291.v1", s["version"])
	}
	candidates, ok := s["advance_candidates_only"].([]any)
	if !ok || len(candidates) != 2 {
		t.Fatalf("advance_candidates_only=%v, want 2 items", s["advance_candidates_only"])
	}
	blocked, ok := s["blocked_from_clock_write"].([]any)
	if !ok || len(blocked) != 4 {
		t.Fatalf("blocked_from_clock_write=%v, want 4 items", s["blocked_from_clock_write"])
	}
	if s["scene_progression_required"] != true {
		t.Fatalf("scene_progression_required=%v, want true", s["scene_progression_required"])
	}
	if s["mode"] != "advance_discipline_contract_definition" {
		t.Fatalf("mode=%v, want advance_discipline_contract_definition", s["mode"])
	}
}

func TestSeq19P292TruthBoundaryPreserveContract(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p292","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "truth_boundary_preserve_contract")
	if s["version"] != "s19-p292.v1" {
		t.Fatalf("version=%v, want s19-p292.v1", s["version"])
	}
	if s["response_prose_promotion_blocked"] != true {
		t.Fatalf("response_prose_promotion_blocked=%v, want true", s["response_prose_promotion_blocked"])
	}
	if s["validator_authority"] != "trace_warning_only" {
		t.Fatalf("validator_authority=%v, want trace_warning_only", s["validator_authority"])
	}
	if s["validator_blocks_write"] != false {
		t.Fatalf("validator_blocks_write=%v, want false", s["validator_blocks_write"])
	}
	if s["validator_blocks_save"] != false {
		t.Fatalf("validator_blocks_save=%v, want false", s["validator_blocks_save"])
	}
	if s["mode"] != "truth_boundary_preserve_contract_definition" {
		t.Fatalf("mode=%v, want truth_boundary_preserve_contract_definition", s["mode"])
	}
}

func TestSeq19P296CurrentStoryClockSchemaDefine(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p296","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "current_story_clock_schema_define")
	if s["version"] != "s19-p296.v1" {
		t.Fatalf("version=%v, want s19-p296.v1", s["version"])
	}
	if s["sub_step"] != "19-1a" {
		t.Fatalf("sub_step=%v, want 19-1a", s["sub_step"])
	}
	fields, ok := s["schema_fields"].([]any)
	if !ok || len(fields) != 7 {
		t.Fatalf("schema_fields=%v, want 7 items", s["schema_fields"])
	}
	if s["canonical_anchor_only"] != true {
		t.Fatalf("canonical_anchor_only=%v, want true", s["canonical_anchor_only"])
	}
	if s["mode"] != "current_story_clock_schema_define_definition" {
		t.Fatalf("mode=%v, want current_story_clock_schema_define_definition", s["mode"])
	}
}

func TestSeq19P297SessionStateTimelineAnchorPrecedenceDefine(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p297","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "session_state_timeline_anchor_precedence_define")
	if s["version"] != "s19-p297.v1" {
		t.Fatalf("version=%v, want s19-p297.v1", s["version"])
	}
	if s["sub_step"] != "19-1b" {
		t.Fatalf("sub_step=%v, want 19-1b", s["sub_step"])
	}
	order, ok := s["precedence_order"].([]any)
	if !ok || len(order) != 4 {
		t.Fatalf("precedence_order=%v, want 4 items", s["precedence_order"])
	}
	wantOrder := []string{"session_state_clock", "input_current_scene_anchor", "timeline_anchor", "carry_forward"}
	for i, w := range wantOrder {
		if order[i] != w {
			t.Fatalf("precedence_order[%d]=%v, want %s", i, order[i], w)
		}
	}
	if s["mode"] != "session_state_timeline_anchor_precedence_define_definition" {
		t.Fatalf("mode=%v, want session_state_timeline_anchor_precedence_define_definition", s["mode"])
	}
}

func TestSeq19P298PrecisionLabelDefine(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p298","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "precision_label_define")
	if s["version"] != "s19-p298.v1" {
		t.Fatalf("version=%v, want s19-p298.v1", s["version"])
	}
	if s["sub_step"] != "19-1c" {
		t.Fatalf("sub_step=%v, want 19-1c", s["sub_step"])
	}
	labels, ok := s["precision_labels"].([]any)
	if !ok || len(labels) != 4 {
		t.Fatalf("precision_labels=%v, want 4 items", s["precision_labels"])
	}
	if s["coarse_collapsed_to"] != "bounded_range" {
		t.Fatalf("coarse_collapsed_to=%v, want bounded_range", s["coarse_collapsed_to"])
	}
	if s["fake_precision_blocked"] != true {
		t.Fatalf("fake_precision_blocked=%v, want true", s["fake_precision_blocked"])
	}
	if s["mode"] != "precision_label_define_definition" {
		t.Fatalf("mode=%v, want precision_label_define_definition", s["mode"])
	}
}

func TestSeq19P299CurrentSceneRecalledPastSplitDefine(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p299","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "current_scene_recalled_past_split_define")
	if s["version"] != "s19-p299.v1" {
		t.Fatalf("version=%v, want s19-p299.v1", s["version"])
	}
	if s["sub_step"] != "19-1d" {
		t.Fatalf("sub_step=%v, want 19-1d", s["sub_step"])
	}
	if s["write_lane"] != "current_scene" {
		t.Fatalf("write_lane=%v, want current_scene", s["write_lane"])
	}
	targets, ok := s["relation_only_targets"].([]any)
	if !ok || len(targets) != 4 {
		t.Fatalf("relation_only_targets=%v, want 4 items", s["relation_only_targets"])
	}
	if s["same_write_lane_merge_blocked"] != true {
		t.Fatalf("same_write_lane_merge_blocked=%v, want true", s["same_write_lane_merge_blocked"])
	}
	if s["mode"] != "current_scene_recalled_past_split_define_definition" {
		t.Fatalf("mode=%v, want current_scene_recalled_past_split_define_definition", s["mode"])
	}
}

func TestSeq19P303TemporalRelationSchemaDefine(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p303","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "temporal_relation_schema_define")
	if s["version"] != "s19-p303.v1" {
		t.Fatalf("version=%v, want s19-p303.v1", s["version"])
	}
	if s["sub_step"] != "19-2a" {
		t.Fatalf("sub_step=%v, want 19-2a", s["sub_step"])
	}
	keys, ok := s["schema_keys"].([]any)
	if !ok || len(keys) != 9 {
		t.Fatalf("schema_keys=%v, want 9 items", s["schema_keys"])
	}
	keySet := map[string]bool{}
	for _, key := range keys {
		keySet[fmt.Sprint(key)] = true
	}
	for _, required := range []string{"relative_label", "anchor_ref", "target_kind", "offset_value_min", "offset_value_max", "offset_unit", "precision", "status", "source_turn"} {
		if !keySet[required] {
			t.Fatalf("schema_keys missing %q: %v", required, s["schema_keys"])
		}
	}
	aliases := seq165Map(t, s, "compat_aliases")
	if aliases["anchor"] != "anchor_ref" {
		t.Fatalf("compat_aliases.anchor=%v, want anchor_ref", aliases["anchor"])
	}
	if s["canonical_format"] != "snake_case" {
		t.Fatalf("canonical_format=%v, want snake_case", s["canonical_format"])
	}
	if s["mode"] != "temporal_relation_schema_define_definition" {
		t.Fatalf("mode=%v, want temporal_relation_schema_define_definition", s["mode"])
	}
}

func TestSeq19P304PhraseIngressNormalizationDefine(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p304","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "phrase_ingress_normalization_define")
	if s["version"] != "s19-p304.v1" {
		t.Fatalf("version=%v, want s19-p304.v1", s["version"])
	}
	if s["sub_step"] != "19-2b" {
		t.Fatalf("sub_step=%v, want 19-2b", s["sub_step"])
	}
	phrases, ok := s["supported_phrases"].([]any)
	if !ok || len(phrases) < 6 {
		t.Fatalf("supported_phrases=%v, want at least 6 items", s["supported_phrases"])
	}
	phraseSet := map[string]bool{}
	for _, phrase := range phrases {
		phraseSet[fmt.Sprint(phrase)] = true
	}
	for _, required := range []string{"어제", "그저께", "사흘 뒤", "저번 달", "지난 겨울", "몇 달 전"} {
		if !phraseSet[required] {
			t.Fatalf("supported_phrases missing %q: %v", required, s["supported_phrases"])
		}
	}
	if s["fallback_behavior"] != "carry_forward_unresolved" {
		t.Fatalf("fallback_behavior=%v, want carry_forward_unresolved", s["fallback_behavior"])
	}
	if s["mode"] != "phrase_ingress_normalization_define_definition" {
		t.Fatalf("mode=%v, want phrase_ingress_normalization_define_definition", s["mode"])
	}
}

func TestSeq19P305TemporalRelationSurfaceDefine(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p305","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "temporal_relation_surface_define")
	if s["version"] != "s19-p305.v1" {
		t.Fatalf("version=%v, want s19-p305.v1", s["version"])
	}
	if s["sub_step"] != "19-2c" {
		t.Fatalf("sub_step=%v, want 19-2c", s["sub_step"])
	}
	rk, ok := s["range_kinds"].([]any)
	if !ok || len(rk) != 3 {
		t.Fatalf("range_kinds=%v, want 3 items", s["range_kinds"])
	}
	if s["bounded_ambiguity_preserved"] != true {
		t.Fatalf("bounded_ambiguity_preserved=%v, want true", s["bounded_ambiguity_preserved"])
	}
	if s["mode"] != "temporal_relation_surface_define_definition" {
		t.Fatalf("mode=%v, want temporal_relation_surface_define_definition", s["mode"])
	}
}

func TestSeq19P306AnchorAmbiguityCarryForwardDefine(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p306","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "anchor_ambiguity_carry_forward_define")
	if s["version"] != "s19-p306.v1" {
		t.Fatalf("version=%v, want s19-p306.v1", s["version"])
	}
	if s["sub_step"] != "19-2d" {
		t.Fatalf("sub_step=%v, want 19-2d", s["sub_step"])
	}
	if s["missing_anchor_degrades_to"] != "carry_forward" {
		t.Fatalf("missing_anchor_degrades_to=%v, want carry_forward", s["missing_anchor_degrades_to"])
	}
	if s["precision_degrades_to"] != "unknown" {
		t.Fatalf("precision_degrades_to=%v, want unknown", s["precision_degrades_to"])
	}
	if s["false_precision_blocked"] != true {
		t.Fatalf("false_precision_blocked=%v, want true", s["false_precision_blocked"])
	}
	if s["mode"] != "anchor_ambiguity_carry_forward_define_definition" {
		t.Fatalf("mode=%v, want anchor_ambiguity_carry_forward_define_definition", s["mode"])
	}
}

func TestSeq19P307LocaleParserPackBoundaryDefine(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p307","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "locale_parser_pack_boundary_define")
	if s["version"] != "s19-p307.v1" {
		t.Fatalf("version=%v, want s19-p307.v1", s["version"])
	}
	if s["sub_step"] != "19-2e" {
		t.Fatalf("sub_step=%v, want 19-2e", s["sub_step"])
	}
	locales, ok := s["locale_packs"].([]any)
	if !ok || len(locales) != 4 {
		t.Fatalf("locale_packs=%v, want 4 items", s["locale_packs"])
	}
	if s["canonical_normalizer_separated"] != true {
		t.Fatalf("canonical_normalizer_separated=%v, want true", s["canonical_normalizer_separated"])
	}
	if s["fail_open_unsupported_locale"] != true {
		t.Fatalf("fail_open_unsupported_locale=%v, want true", s["fail_open_unsupported_locale"])
	}
	if s["mode"] != "locale_parser_pack_boundary_define_definition" {
		t.Fatalf("mode=%v, want locale_parser_pack_boundary_define_definition", s["mode"])
	}
}

func TestSeq19P311AdvanceTriggerDefine(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p311","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "advance_trigger_define")
	if s["version"] != "s19-p311.v1" {
		t.Fatalf("version=%v, want s19-p311.v1", s["version"])
	}
	if s["sub_step"] != "19-3a" {
		t.Fatalf("sub_step=%v, want 19-3a", s["sub_step"])
	}
	cats, ok := s["trigger_categories"].([]any)
	if !ok || len(cats) != 6 {
		t.Fatalf("trigger_categories=%v, want 6 items", s["trigger_categories"])
	}
	if s["mode"] != "advance_trigger_define_definition" {
		t.Fatalf("mode=%v, want advance_trigger_define_definition", s["mode"])
	}
}

func TestSeq19P312SceneTransitionDefine(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p312","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "scene_transition_define")
	if s["version"] != "s19-p312.v1" {
		t.Fatalf("version=%v, want s19-p312.v1", s["version"])
	}
	if s["sub_step"] != "19-3b" {
		t.Fatalf("sub_step=%v, want 19-3b", s["sub_step"])
	}
	adv, ok := s["advance_actions"].([]any)
	if !ok || len(adv) != 2 {
		t.Fatalf("advance_actions=%v, want 2 items", s["advance_actions"])
	}
	noAdv, ok := s["no_advance_actions"].([]any)
	if !ok || len(noAdv) != 3 {
		t.Fatalf("no_advance_actions=%v, want 3 items", s["no_advance_actions"])
	}
	if s["scene_progression_required"] != true {
		t.Fatalf("scene_progression_required=%v, want true", s["scene_progression_required"])
	}
	if s["mode"] != "scene_transition_define_definition" {
		t.Fatalf("mode=%v, want scene_transition_define_definition", s["mode"])
	}
}

func TestSeq19P313ElapsedTimeWriteDisciplineDefine(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p313","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "elapsed_time_write_discipline_define")
	if s["version"] != "s19-p313.v1" {
		t.Fatalf("version=%v, want s19-p313.v1", s["version"])
	}
	if s["sub_step"] != "19-3c" {
		t.Fatalf("sub_step=%v, want 19-3c", s["sub_step"])
	}
	disciplines, ok := s["write_disciplines"].([]any)
	if !ok || len(disciplines) != 4 {
		t.Fatalf("write_disciplines=%v, want 4 items", s["write_disciplines"])
	}
	if s["relation_only_blocked"] != true {
		t.Fatalf("relation_only_blocked=%v, want true", s["relation_only_blocked"])
	}
	if s["figurative_duration_blocked"] != true {
		t.Fatalf("figurative_duration_blocked=%v, want true", s["figurative_duration_blocked"])
	}
	if s["mode"] != "elapsed_time_write_discipline_define_definition" {
		t.Fatalf("mode=%v, want elapsed_time_write_discipline_define_definition", s["mode"])
	}
}

func TestSeq19P314TemporalSupportPacketDefine(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p314","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "temporal_support_packet_define")
	if s["version"] != "s19-p314.v1" {
		t.Fatalf("version=%v, want s19-p314.v1", s["version"])
	}
	if s["sub_step"] != "19-3d" {
		t.Fatalf("sub_step=%v, want 19-3d", s["sub_step"])
	}
	fields, ok := s["packet_fields"].([]any)
	if !ok || len(fields) != 4 {
		t.Fatalf("packet_fields=%v, want 4 items", s["packet_fields"])
	}
	if s["support_only"] != true {
		t.Fatalf("support_only=%v, want true", s["support_only"])
	}
	if s["carry_forward_only"] != true {
		t.Fatalf("carry_forward_only=%v, want true", s["carry_forward_only"])
	}
	if s["mode"] != "temporal_support_packet_define_definition" {
		t.Fatalf("mode=%v, want temporal_support_packet_define_definition", s["mode"])
	}
}

func TestSeq19P318TemporalReplayDefine19_4a(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p318","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "temporal_replay_define_19_4a")
	if s["version"] != "s19-p318.v1" {
		t.Fatalf("version=%v, want s19-p318.v1", s["version"])
	}
	if s["sub_step"] != "19-4a" {
		t.Fatalf("sub_step=%v, want 19-4a", s["sub_step"])
	}
	phrases, ok := s["replay_phrases"].([]any)
	if !ok || len(phrases) != 3 {
		t.Fatalf("replay_phrases=%v, want 3 items", s["replay_phrases"])
	}
	if s["week_month_write_lane"] != "carry_forward_only" {
		t.Fatalf("week_month_write_lane=%v, want carry_forward_only", s["week_month_write_lane"])
	}
	if s["mode"] != "temporal_replay_define_19_4a_definition" {
		t.Fatalf("mode=%v, want temporal_replay_define_19_4a_definition", s["mode"])
	}
}

func TestSeq19P319CurrentSceneRecalledPastConflictReplayDefine(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p319","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "current_scene_recalled_past_conflict_replay_define")
	if s["version"] != "s19-p319.v1" {
		t.Fatalf("version=%v, want s19-p319.v1", s["version"])
	}
	if s["sub_step"] != "19-4b" {
		t.Fatalf("sub_step=%v, want 19-4b", s["sub_step"])
	}
	if s["recalled_past_preserved"] != true {
		t.Fatalf("recalled_past_preserved=%v, want true", s["recalled_past_preserved"])
	}
	if s["overwrite_protection"] != true {
		t.Fatalf("overwrite_protection=%v, want true", s["overwrite_protection"])
	}
	if s["mode"] != "current_scene_recalled_past_conflict_replay_define_definition" {
		t.Fatalf("mode=%v, want current_scene_recalled_past_conflict_replay_define_definition", s["mode"])
	}
}

func TestSeq19P320MissingAnchorLowPrecisionDegradeReplayDefine(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p320","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "missing_anchor_low_precision_degrade_replay_define")
	if s["version"] != "s19-p320.v1" {
		t.Fatalf("version=%v, want s19-p320.v1", s["version"])
	}
	if s["sub_step"] != "19-4c" {
		t.Fatalf("sub_step=%v, want 19-4c", s["sub_step"])
	}
	if s["no_fake_anchored_certainty"] != true {
		t.Fatalf("no_fake_anchored_certainty=%v, want true", s["no_fake_anchored_certainty"])
	}
	if s["mode"] != "missing_anchor_low_precision_degrade_replay_define_definition" {
		t.Fatalf("mode=%v, want missing_anchor_low_precision_degrade_replay_define_definition", s["mode"])
	}
}

func TestSeq19P321TemporalPacketTruthBoundaryPrecedenceReplayDefine(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p321","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "temporal_packet_truth_boundary_precedence_replay_define")
	if s["version"] != "s19-p321.v1" {
		t.Fatalf("version=%v, want s19-p321.v1", s["version"])
	}
	if s["sub_step"] != "19-4d" {
		t.Fatalf("sub_step=%v, want 19-4d", s["sub_step"])
	}
	if s["packet_built_backend_first"] != true {
		t.Fatalf("packet_built_backend_first=%v, want true", s["packet_built_backend_first"])
	}
	if s["mode"] != "temporal_packet_truth_boundary_precedence_replay_define_definition" {
		t.Fatalf("mode=%v, want temporal_packet_truth_boundary_precedence_replay_define_definition", s["mode"])
	}
}

func TestSeq19P322ResponseTimeDeicticValidatorReplayDefine(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p322","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "response_time_deictic_validator_replay_define")
	if s["version"] != "s19-p322.v1" {
		t.Fatalf("version=%v, want s19-p322.v1", s["version"])
	}
	if s["sub_step"] != "19-4e" {
		t.Fatalf("sub_step=%v, want 19-4e", s["sub_step"])
	}
	if s["latest_timestamp_shortcut"] != false {
		t.Fatalf("latest_timestamp_shortcut=%v, want false", s["latest_timestamp_shortcut"])
	}
	if s["trace_only_warning_surface"] != true {
		t.Fatalf("trace_only_warning_surface=%v, want true", s["trace_only_warning_surface"])
	}
	warnings, ok := s["warning_classes"].([]any)
	if !ok || len(warnings) != 3 {
		t.Fatalf("warning_classes=%v, want 3 items", s["warning_classes"])
	}
	if s["mode"] != "response_time_deictic_validator_replay_define_definition" {
		t.Fatalf("mode=%v, want response_time_deictic_validator_replay_define_definition", s["mode"])
	}
}

func TestSeq19P323FigurativeDurationPlannedFutureRecalledPastClassificationReplayDefine(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p323","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "figurative_duration_planned_future_recalled_past_classification_replay_define")
	if s["version"] != "s19-p323.v1" {
		t.Fatalf("version=%v, want s19-p323.v1", s["version"])
	}
	if s["sub_step"] != "19-4f" {
		t.Fatalf("sub_step=%v, want 19-4f", s["sub_step"])
	}
	cases, ok := s["classification_cases"].([]any)
	if !ok || len(cases) != 5 {
		t.Fatalf("classification_cases=%v, want 5 items", s["classification_cases"])
	}
	casesByPhrase := map[string]map[string]any{}
	for _, item := range cases {
		m, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("classification case is not map: %v", item)
		}
		phrase, _ := m["phrase"].(string)
		casesByPhrase[phrase] = m
	}
	assertClassification := func(phrase, primaryClass, reason string, clockWriteBlocked bool) {
		t.Helper()
		m, ok := casesByPhrase[phrase]
		if !ok {
			t.Fatalf("missing classification case for %q in %v", phrase, cases)
		}
		if m["primary_class"] != primaryClass {
			t.Fatalf("%s primary_class=%v, want %s", phrase, m["primary_class"], primaryClass)
		}
		if m["clock_write_blocked"] != clockWriteBlocked {
			t.Fatalf("%s clock_write_blocked=%v, want %v", phrase, m["clock_write_blocked"], clockWriteBlocked)
		}
		if m["reason"] != reason {
			t.Fatalf("%s reason=%v, want %s", phrase, m["reason"], reason)
		}
	}
	assertClassification("it felt like a week", "figurative_duration", "figurative_duration_excluded", true)
	assertClassification("내일", "planned_event", "block_relation_only_write", true)
	assertClassification("tomorrow", "planned_event", "block_relation_only_write", true)
	assertClassification("어제", "recalled_event", "block_relation_only_write", true)
	assertClassification("yesterday", "recalled_event", "block_relation_only_write", true)
	wd, ok := s["write_discipline"].(map[string]any)
	if !ok {
		t.Fatalf("write_discipline missing or not map")
	}
	if wd["block_figurative_only_write"] != true {
		t.Fatalf("block_figurative_only_write=%v, want true", wd["block_figurative_only_write"])
	}
	if wd["block_relation_only_write"] != true {
		t.Fatalf("block_relation_only_write=%v, want true", wd["block_relation_only_write"])
	}
	if wd["allow_planned_future_write"] != false {
		t.Fatalf("allow_planned_future_write=%v, want false", wd["allow_planned_future_write"])
	}
	if s["mode"] != "figurative_duration_planned_future_recalled_past_classification_replay_define_definition" {
		t.Fatalf("mode=%v, want figurative_duration_planned_future_recalled_past_classification_replay_define_definition", s["mode"])
	}
}

func TestSeq19P324MultilingualParityMixedLanguageFailOpenReplayDefine(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p324","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "multilingual_parity_mixed_language_fail_open_replay_define")
	if s["version"] != "s19-p324.v1" {
		t.Fatalf("version=%v, want s19-p324.v1", s["version"])
	}
	if s["sub_step"] != "19-4g" {
		t.Fatalf("sub_step=%v, want 19-4g", s["sub_step"])
	}
	phrases, ok := s["parity_phrases"].([]any)
	if !ok || len(phrases) != 5 {
		t.Fatalf("parity_phrases=%v, want 5 items", s["parity_phrases"])
	}
	assertParityPhrase := func(canonical, ko, en, ja, zh string) {
		t.Helper()
		for _, item := range phrases {
			m, ok := item.(map[string]any)
			if !ok {
				t.Fatalf("parity phrase is not map: %v", item)
			}
			if m["canonical"] == canonical && m["ko"] == ko && m["en"] == en && m["ja"] == ja && m["zh"] == zh {
				return
			}
		}
		t.Fatalf("missing parity phrase canonical=%s ko=%s en=%s ja=%s zh=%s in %v", canonical, ko, en, ja, zh, phrases)
	}
	assertParityPhrase("recalled_event", "어제", "yesterday", "昨日", "昨天")
	assertParityPhrase("recalled_event", "지난 겨울", "last winter", "去年の冬", "去年冬天")
	assertParityPhrase("recalled_event", "몇 주 전", "few weeks ago", "数週間前", "几周前")
	assertParityPhrase("recalled_event", "몇 달 전", "few months ago", "数ヶ月前", "几个月前")
	assertParityPhrase("planned_event", "내일", "tomorrow", "明日", "明天")
	failOpen, ok := s["mixed_language_fail_open"].(map[string]any)
	if !ok {
		t.Fatalf("mixed_language_fail_open missing or not map")
	}
	if failOpen["policy"] != "extract_only_supported_locale_tokens" {
		t.Fatalf("policy=%v, want extract_only_supported_locale_tokens", failOpen["policy"])
	}
	if failOpen["ignore_unsupported"] != true {
		t.Fatalf("ignore_unsupported=%v, want true", failOpen["ignore_unsupported"])
	}
	if failOpen["no_hallucination"] != true {
		t.Fatalf("no_hallucination=%v, want true", failOpen["no_hallucination"])
	}
	locales, ok := s["active_locales_gating"].([]any)
	if !ok || len(locales) != 4 {
		t.Fatalf("active_locales_gating=%v, want 4 items", s["active_locales_gating"])
	}
	for _, locale := range []string{"ko", "en", "ja", "zh"} {
		found := false
		for _, got := range locales {
			if got == locale {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("active_locales_gating missing %s in %v", locale, locales)
		}
	}
	if s["mode"] != "multilingual_parity_mixed_language_fail_open_replay_define_definition" {
		t.Fatalf("mode=%v, want multilingual_parity_mixed_language_fail_open_replay_define_definition", s["mode"])
	}
}

func TestSeq19P328Beta10BundleLatestRootRuntimeDefine(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p328","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "beta_1_0_bundle_latest_root_runtime_define")
	if s["version"] != "s19-p328.v1" {
		t.Fatalf("version=%v, want s19-p328.v1", s["version"])
	}
	if s["sub_step"] != "release_gate" {
		t.Fatalf("sub_step=%v, want release_gate", s["sub_step"])
	}
	if s["artifact_generation"] != false {
		t.Fatalf("artifact_generation=%v, want false", s["artifact_generation"])
	}
	if s["contract_only_surface"] != true {
		t.Fatalf("contract_only_surface=%v, want true", s["contract_only_surface"])
	}
	if s["mode"] != "beta_1_0_bundle_latest_root_runtime_define_definition" {
		t.Fatalf("mode=%v, want beta_1_0_bundle_latest_root_runtime_define_definition", s["mode"])
	}
}

func TestSeq19P329StoryClockSmokeCheckPass(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p329","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "story_clock_smoke_check_pass")
	if s["version"] != "s19-p329.v1" {
		t.Fatalf("version=%v, want s19-p329.v1", s["version"])
	}
	if s["sub_step"] != "release_gate" {
		t.Fatalf("sub_step=%v, want release_gate", s["sub_step"])
	}
	if s["smoke_status"] != "pass" {
		t.Fatalf("smoke_status=%v, want pass", s["smoke_status"])
	}
	items, ok := s["check_items"].([]any)
	if !ok || len(items) != 4 {
		t.Fatalf("check_items=%v, want 4 items", s["check_items"])
	}
	if s["mode"] != "story_clock_smoke_check_pass_definition" {
		t.Fatalf("mode=%v, want story_clock_smoke_check_pass_definition", s["mode"])
	}
}

func TestSeq19P330RelativeTimeNormalizationSmokeCheckPass(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p330","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "relative_time_normalization_smoke_check_pass")
	if s["version"] != "s19-p330.v1" {
		t.Fatalf("version=%v, want s19-p330.v1", s["version"])
	}
	if s["sub_step"] != "release_gate" {
		t.Fatalf("sub_step=%v, want release_gate", s["sub_step"])
	}
	if s["smoke_status"] != "pass" {
		t.Fatalf("smoke_status=%v, want pass", s["smoke_status"])
	}
	items, ok := s["check_items"].([]any)
	if !ok || len(items) != 5 {
		t.Fatalf("check_items=%v, want 5 items", s["check_items"])
	}
	if s["mode"] != "relative_time_normalization_smoke_check_pass_definition" {
		t.Fatalf("mode=%v, want relative_time_normalization_smoke_check_pass_definition", s["mode"])
	}
}

func TestSeq19P331ElapsedTimeAdvanceReplayPass(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p331","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
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
	s := seq165Map(t, resp, "elapsed_time_advance_replay_pass")
	if s["version"] != "s19-p331.v1" {
		t.Fatalf("version=%v, want s19-p331.v1", s["version"])
	}
	if s["sub_step"] != "release_gate" {
		t.Fatalf("sub_step=%v, want release_gate", s["sub_step"])
	}
	if s["replay_status"] != "pass" {
		t.Fatalf("replay_status=%v, want pass", s["replay_status"])
	}
	items, ok := s["check_items"].([]any)
	if !ok || len(items) != 5 {
		t.Fatalf("check_items=%v, want 5 items", s["check_items"])
	}
	if s["mode"] != "elapsed_time_advance_replay_pass_definition" {
		t.Fatalf("mode=%v, want elapsed_time_advance_replay_pass_definition", s["mode"])
	}
}
