package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSeq19P332AmbiguityPrecedenceReviewChecklistPass(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p332","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "ambiguity_precedence_review_checklist_pass")
	if s["version"] != "s19-p332.v1" {
		t.Fatalf("version=%v, want s19-p332.v1", s["version"])
	}
	if s["sub_step"] != "release_gate" {
		t.Fatalf("sub_step=%v, want release_gate", s["sub_step"])
	}
	if s["review_status"] != "pass" {
		t.Fatalf("review_status=%v, want pass", s["review_status"])
	}
	items, ok := s["check_items"].([]any)
	if !ok || len(items) != 5 {
		t.Fatalf("check_items=%v, want 5 items", s["check_items"])
	}
	if s["mode"] != "ambiguity_precedence_review_checklist_pass_definition" {
		t.Fatalf("mode=%v, want ambiguity_precedence_review_checklist_pass_definition", s["mode"])
	}
}

func TestSeq19P333MultilingualTemporalParitySmokeCheckPass(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p333","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "multilingual_temporal_parity_smoke_check_pass")
	if s["version"] != "s19-p333.v1" {
		t.Fatalf("version=%v, want s19-p333.v1", s["version"])
	}
	if s["sub_step"] != "release_gate" {
		t.Fatalf("sub_step=%v, want release_gate", s["sub_step"])
	}
	if s["parity_status"] != "pass" {
		t.Fatalf("parity_status=%v, want pass", s["parity_status"])
	}
	items, ok := s["check_items"].([]any)
	if !ok || len(items) != 6 {
		t.Fatalf("check_items=%v, want 6 items", s["check_items"])
	}
	if s["mode"] != "multilingual_temporal_parity_smoke_check_pass_definition" {
		t.Fatalf("mode=%v, want multilingual_temporal_parity_smoke_check_pass_definition", s["mode"])
	}
}

func TestSeq19P337CurrentStoryClockAbsoluteDatetimeBoundedStoryDay(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p337","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "current_story_clock_absolute_datetime_bounded_story_day")
	if s["version"] != "s19-p337.v1" {
		t.Fatalf("version=%v, want s19-p337.v1", s["version"])
	}
	if s["sub_step"] != "decision" {
		t.Fatalf("sub_step=%v, want decision", s["sub_step"])
	}
	if s["decision"] != "bounded_story_day" {
		t.Fatalf("decision=%v, want bounded_story_day", s["decision"])
	}
	if s["rejected_alternative"] != "absolute_datetime" {
		t.Fatalf("rejected_alternative=%v, want absolute_datetime", s["rejected_alternative"])
	}
	rationale, ok := s["rationale"].([]any)
	if !ok || len(rationale) != 4 {
		t.Fatalf("rationale=%v, want 4 items", s["rationale"])
	}
	if s["mode"] != "current_story_clock_absolute_datetime_bounded_story_day_definition" {
		t.Fatalf("mode=%v, want current_story_clock_absolute_datetime_bounded_story_day_definition", s["mode"])
	}
}

func TestSeq19P338RelativeTimeNormalizationNumericOffsetVocabularyFirst(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p338","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "relative_time_normalization_numeric_offset_vocabulary_first")
	if s["version"] != "s19-p338.v1" {
		t.Fatalf("version=%v, want s19-p338.v1", s["version"])
	}
	if s["sub_step"] != "decision" {
		t.Fatalf("sub_step=%v, want decision", s["sub_step"])
	}
	if s["decision"] != "vocabulary_first" {
		t.Fatalf("decision=%v, want vocabulary_first", s["decision"])
	}
	if s["rejected_alternative"] != "numeric_offset_first" {
		t.Fatalf("rejected_alternative=%v, want numeric_offset_first", s["rejected_alternative"])
	}
	rationale, ok := s["rationale"].([]any)
	if !ok || len(rationale) != 4 {
		t.Fatalf("rationale=%v, want 4 items", s["rationale"])
	}
	if s["mode"] != "relative_time_normalization_numeric_offset_vocabulary_first_definition" {
		t.Fatalf("mode=%v, want relative_time_normalization_numeric_offset_vocabulary_first_definition", s["mode"])
	}
}

func TestSeq19P339ElapsedTimeAdvanceConservativeManualSceneClassifier(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p339","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "elapsed_time_advance_conservative_manual_scene_classifier")
	if s["version"] != "s19-p339.v1" {
		t.Fatalf("version=%v, want s19-p339.v1", s["version"])
	}
	if s["sub_step"] != "decision" {
		t.Fatalf("sub_step=%v, want decision", s["sub_step"])
	}
	if s["decision"] != "conservative_manual_rules" {
		t.Fatalf("decision=%v, want conservative_manual_rules", s["decision"])
	}
	if s["rejected_alternative"] != "scene_classifier_mixed" {
		t.Fatalf("rejected_alternative=%v, want scene_classifier_mixed", s["rejected_alternative"])
	}
	rationale, ok := s["rationale"].([]any)
	if !ok || len(rationale) != 4 {
		t.Fatalf("rationale=%v, want 4 items", s["rationale"])
	}
	if s["mode"] != "elapsed_time_advance_conservative_manual_scene_classifier_definition" {
		t.Fatalf("mode=%v, want elapsed_time_advance_conservative_manual_scene_classifier_definition", s["mode"])
	}
}

func TestSeq19P340MissingAnchorDegrade(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p340","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "missing_anchor_degrade")
	if s["version"] != "s19-p340.v1" {
		t.Fatalf("version=%v, want s19-p340.v1", s["version"])
	}
	if s["sub_step"] != "degrade" {
		t.Fatalf("sub_step=%v, want degrade", s["sub_step"])
	}
	if s["degrade_status"] != "explicit" {
		t.Fatalf("degrade_status=%v, want explicit", s["degrade_status"])
	}
	items, ok := s["degrade_items"].([]any)
	if !ok || len(items) != 4 {
		t.Fatalf("degrade_items=%v, want 4 items", s["degrade_items"])
	}
	if s["mode"] != "missing_anchor_degrade_definition" {
		t.Fatalf("mode=%v, want missing_anchor_degrade_definition", s["mode"])
	}
}

func TestSeq19P341LocaleParsingSingleDetectorActiveLocalesMerge(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p341","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "locale_parsing_single_detector_active_locales_merge")
	if s["version"] != "s19-p341.v1" {
		t.Fatalf("version=%v, want s19-p341.v1", s["version"])
	}
	if s["sub_step"] != "decision" {
		t.Fatalf("sub_step=%v, want decision", s["sub_step"])
	}
	if s["decision"] != "active_locales_merge" {
		t.Fatalf("decision=%v, want active_locales_merge", s["decision"])
	}
	if s["rejected_alternative"] != "single_detector" {
		t.Fatalf("rejected_alternative=%v, want single_detector", s["rejected_alternative"])
	}
	rationale, ok := s["rationale"].([]any)
	if !ok || len(rationale) != 4 {
		t.Fatalf("rationale=%v, want 4 items", s["rationale"])
	}
	if s["mode"] != "locale_parsing_single_detector_active_locales_merge_definition" {
		t.Fatalf("mode=%v, want locale_parsing_single_detector_active_locales_merge_definition", s["mode"])
	}
}

func TestSeq19P342KoEnBootstrapExtractorLocalePackParserReplaceCutover(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p342","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "ko_en_bootstrap_extractor_locale_pack_parser_replace_cutover")
	if s["version"] != "s19-p342.v1" {
		t.Fatalf("version=%v, want s19-p342.v1", s["version"])
	}
	if s["sub_step"] != "cutover" {
		t.Fatalf("sub_step=%v, want cutover", s["sub_step"])
	}
	if s["cutover_status"] != "completed" {
		t.Fatalf("cutover_status=%v, want completed", s["cutover_status"])
	}
	if s["from"] != "ko_en_bootstrap_extractor" {
		t.Fatalf("from=%v, want ko_en_bootstrap_extractor", s["from"])
	}
	if s["to"] != "locale_pack_parser_ko_en_ja_zh" {
		t.Fatalf("to=%v, want locale_pack_parser_ko_en_ja_zh", s["to"])
	}
	evidence, ok := s["evidence"].([]any)
	if !ok || len(evidence) != 4 {
		t.Fatalf("evidence=%v, want 4 items", s["evidence"])
	}
	if s["mode"] != "ko_en_bootstrap_extractor_locale_pack_parser_replace_cutover_definition" {
		t.Fatalf("mode=%v, want ko_en_bootstrap_extractor_locale_pack_parser_replace_cutover_definition", s["mode"])
	}
}

func TestSeq19P343UnspecifiedTimeFallbackNoAdvanceCarryForwardDiscipline(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p343","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "unspecified_time_fallback_no_advance_carry_forward_discipline")
	if s["version"] != "s19-p343.v1" {
		t.Fatalf("version=%v, want s19-p343.v1", s["version"])
	}
	if s["sub_step"] != "decision" {
		t.Fatalf("sub_step=%v, want decision", s["sub_step"])
	}
	if s["decision"] != "no_advance_carry_forward" {
		t.Fatalf("decision=%v, want no_advance_carry_forward", s["decision"])
	}
	if s["rejected_alternative"] != "exact_0_day_truth" {
		t.Fatalf("rejected_alternative=%v, want exact_0_day_truth", s["rejected_alternative"])
	}
	rationale, ok := s["rationale"].([]any)
	if !ok || len(rationale) != 4 {
		t.Fatalf("rationale=%v, want 4 items", s["rationale"])
	}
	if s["mode"] != "unspecified_time_fallback_no_advance_carry_forward_discipline_definition" {
		t.Fatalf("mode=%v, want unspecified_time_fallback_no_advance_carry_forward_discipline_definition", s["mode"])
	}
}

func TestSeq19P344RelationOnlyFuturePastReferenceCurrentSceneAdvanceEvidenceGateSplit(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p344","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "relation_only_future_past_reference_current_scene_advance_evidence_gate_split")
	if s["version"] != "s19-p344.v1" {
		t.Fatalf("version=%v, want s19-p344.v1", s["version"])
	}
	if s["sub_step"] != "gate_split" {
		t.Fatalf("sub_step=%v, want gate_split", s["sub_step"])
	}
	gateRules, ok := s["gate_rules"].(map[string]any)
	if !ok {
		t.Fatalf("gate_rules missing or not map")
	}
	requiredGates := []string{"current_scene_advance", "current_scene_anchor_no_advance", "relation_only_future", "relation_only_past", "no_temporal_signal"}
	for _, gate := range requiredGates {
		g, ok := gateRules[gate].(map[string]any)
		if !ok {
			t.Fatalf("gate_rules[%s] missing or not map", gate)
		}
		if _, ok := g["write_mode"]; !ok {
			t.Fatalf("gate_rules[%s].write_mode missing", gate)
		}
		if _, ok := g["allow_write"]; !ok {
			t.Fatalf("gate_rules[%s].allow_write missing", gate)
		}
	}
	if s["mode"] != "relation_only_future_past_reference_current_scene_advance_evidence_gate_split_definition" {
		t.Fatalf("mode=%v, want relation_only_future_past_reference_current_scene_advance_evidence_gate_split_definition", s["mode"])
	}
}
