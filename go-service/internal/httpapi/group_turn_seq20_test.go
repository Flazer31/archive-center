package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func seq165Slice(t *testing.T, parent map[string]any, key string) []string {
	t.Helper()
	value, ok := parent[key].([]any)
	if !ok {
		t.Fatalf("missing %s slice", key)
	}
	out := make([]string, 0, len(value))
	for _, v := range value {
		s, ok := v.(string)
		if !ok {
			t.Fatalf("non-string element in %s: %v", key, v)
		}
		out = append(out, s)
	}
	return out
}

func sliceContains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func TestSeq20P9ResetAdminNote(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p9","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq20_reset_admin_note")
	if s["version"] != "s20-p9.v1" {
		t.Fatalf("version=%v, want s20-p9.v1", s["version"])
	}
	if s["sub_step"] != "preparatory" {
		t.Fatalf("sub_step=%v, want preparatory", s["sub_step"])
	}
	if s["action_taken"] != "reset_cleared" {
		t.Fatalf("action_taken=%v, want reset_cleared", s["action_taken"])
	}
	if s["mode"] != "seq20_reset_admin_note_definition" {
		t.Fatalf("mode=%v, want seq20_reset_admin_note_definition", s["mode"])
	}
}

func TestSeq20P10HistoricalContentPreserved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p10","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq20_historical_content_preserved")
	if s["version"] != "s20-p10.v1" {
		t.Fatalf("version=%v, want s20-p10.v1", s["version"])
	}
	if s["sub_step"] != "preparatory" {
		t.Fatalf("sub_step=%v, want preparatory", s["sub_step"])
	}
	if s["action_taken"] != "preserved" {
		t.Fatalf("action_taken=%v, want preserved", s["action_taken"])
	}
	if s["mode"] != "seq20_historical_content_preserved_definition" {
		t.Fatalf("mode=%v, want seq20_historical_content_preserved_definition", s["mode"])
	}
}

func TestSeq20P11ResetNoteOnly(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p11","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq20_reset_note_only")
	if s["version"] != "s20-p11.v1" {
		t.Fatalf("version=%v, want s20-p11.v1", s["version"])
	}
	if s["sub_step"] != "preparatory" {
		t.Fatalf("sub_step=%v, want preparatory", s["sub_step"])
	}
	if s["action_taken"] != "document_only" {
		t.Fatalf("action_taken=%v, want document_only", s["action_taken"])
	}
	if s["mode"] != "seq20_reset_note_only_definition" {
		t.Fatalf("mode=%v, want seq20_reset_note_only_definition", s["mode"])
	}
}

func TestSeq20P21Q20aTemporalQueryExpansionPreparatory(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p21","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20a_temporal_query_expansion_preparatory")
	if s["version"] != "q20a-p21.v1" {
		t.Fatalf("version=%v, want q20a-p21.v1", s["version"])
	}
	if s["sub_step"] != "q20a" {
		t.Fatalf("sub_step=%v, want q20a", s["sub_step"])
	}
	if s["status"] != "preparatory_closed" {
		t.Fatalf("status=%v, want preparatory_closed", s["status"])
	}
	if s["mode"] != "q20a_temporal_query_expansion_preparatory_definition" {
		t.Fatalf("mode=%v, want q20a_temporal_query_expansion_preparatory_definition", s["mode"])
	}
}

func TestSeq20P22Q20aV1TemporalQueryExpansion(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p22","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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
	if s["sub_step"] != "q20a" {
		t.Fatalf("sub_step=%v, want q20a", s["sub_step"])
	}
	if s["replaces"] != "boolean_only_temporal_class_signal" {
		t.Fatalf("replaces=%v, want boolean_only_temporal_class_signal", s["replaces"])
	}
	if s["mode"] != "q20a_v1_temporal_query_expansion_definition" {
		t.Fatalf("mode=%v, want q20a_v1_temporal_query_expansion_definition", s["mode"])
	}
}

func TestSeq20P23Q20aRuleSurfaceFocusRange(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p23","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20a_rule_surface_focus_range")
	if s["version"] != "q20a-p23.v1" {
		t.Fatalf("version=%v, want q20a-p23.v1", s["version"])
	}
	focusModes, ok := s["focus_modes"].([]any)
	if !ok || len(focusModes) != 4 {
		t.Fatalf("focus_modes=%v, want 4 items", s["focus_modes"])
	}
	granularities, ok := s["granularities"].([]any)
	if !ok || len(granularities) != 5 {
		t.Fatalf("granularities=%v, want 5 items", s["granularities"])
	}
	preferFlags, ok := s["prefer_flags"].([]any)
	if !ok || len(preferFlags) != 3 {
		t.Fatalf("prefer_flags=%v, want 3 items", s["prefer_flags"])
	}
	if s["mode"] != "q20a_rule_surface_focus_range_definition" {
		t.Fatalf("mode=%v, want q20a_rule_surface_focus_range_definition", s["mode"])
	}
}

func TestSeq20P24Q20aDerivesFromSc19RelationSchema(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p24","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20a_derives_from_sc19_relation_schema")
	if s["version"] != "q20a-p24.v1" {
		t.Fatalf("version=%v, want q20a-p24.v1", s["version"])
	}
	if s["source_schema"] != "_SC19_RELATION_SCHEMA" {
		t.Fatalf("source_schema=%v, want _SC19_RELATION_SCHEMA", s["source_schema"])
	}
	if s["duplicated_table"] != false {
		t.Fatalf("duplicated_table=%v, want false", s["duplicated_table"])
	}
	if s["week_labels_extended"] != true {
		t.Fatalf("week_labels_extended=%v, want true", s["week_labels_extended"])
	}
	if s["mode"] != "q20a_derives_from_sc19_relation_schema_definition" {
		t.Fatalf("mode=%v, want q20a_derives_from_sc19_relation_schema_definition", s["mode"])
	}
}

func TestSeq20P25Q20aMirroredAtRecallIntent(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p25","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20a_mirrored_at_recall_intent")
	if s["version"] != "q20a-p25.v1" {
		t.Fatalf("version=%v, want q20a-p25.v1", s["version"])
	}
	if s["mirror_target"] != "_build_recall_intent_contract_q3a" {
		t.Fatalf("mirror_target=%v, want _build_recall_intent_contract_q3a", s["mirror_target"])
	}
	if s["mirror_payload"] != "temporal_query_expansion" {
		t.Fatalf("mirror_payload=%v, want temporal_query_expansion", s["mirror_payload"])
	}
	if s["mode"] != "q20a_mirrored_at_recall_intent_definition" {
		t.Fatalf("mode=%v, want q20a_mirrored_at_recall_intent_definition", s["mode"])
	}
}

func TestSeq20P26Q20aCurrentClockOverlayCuePack(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p26","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20a_current_clock_overlay_cue_pack")
	if s["version"] != "q20a-p26.v1" {
		t.Fatalf("version=%v, want q20a-p26.v1", s["version"])
	}
	if s["cue_pack_scope"] != "question_shape_metadata" {
		t.Fatalf("cue_pack_scope=%v, want question_shape_metadata", s["cue_pack_scope"])
	}
	if s["not_owned_by"] != "Step 19 relative_label_canon" {
		t.Fatalf("not_owned_by=%v, want Step 19 relative_label_canon", s["not_owned_by"])
	}
	exampleCues, ok := s["example_cues"].([]any)
	if !ok || len(exampleCues) != 4 {
		t.Fatalf("example_cues=%v, want 4 items", s["example_cues"])
	}
	if s["mode"] != "q20a_current_clock_overlay_cue_pack_definition" {
		t.Fatalf("mode=%v, want q20a_current_clock_overlay_cue_pack_definition", s["mode"])
	}
}

func TestSeq20P27Q20aQr1aLexicalRoutingNormalized(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p27","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20a_qr1a_lexical_routing_normalized")
	if s["version"] != "q20a-p27.v1" {
		t.Fatalf("version=%v, want q20a-p27.v1", s["version"])
	}
	if s["owner_block"] != "_QR1A_QUERY_CLASS_SIGNAL_RULES" {
		t.Fatalf("owner_block=%v, want _QR1A_QUERY_CLASS_SIGNAL_RULES", s["owner_block"])
	}
	cueCategories, ok := s["cue_categories"].([]any)
	if !ok || len(cueCategories) != 3 {
		t.Fatalf("cue_categories=%v, want 3 items", s["cue_categories"])
	}
	if s["mode"] != "q20a_qr1a_lexical_routing_normalized_definition" {
		t.Fatalf("mode=%v, want q20a_qr1a_lexical_routing_normalized_definition", s["mode"])
	}
}

func TestSeq20P28Q20aContractOnlyGroundwork(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p28","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20a_contract_only_groundwork")
	if s["version"] != "q20a-p28.v1" {
		t.Fatalf("version=%v, want q20a-p28.v1", s["version"])
	}
	if s["scope"] != "rule_definition_groundwork" {
		t.Fatalf("scope=%v, want rule_definition_groundwork", s["scope"])
	}
	if s["live_execution"] != false {
		t.Fatalf("live_execution=%v, want false", s["live_execution"])
	}
	if s["mode"] != "q20a_contract_only_groundwork_definition" {
		t.Fatalf("mode=%v, want q20a_contract_only_groundwork_definition", s["mode"])
	}
}

func TestSeq20P36Q20bTemporalValidityReadPolicyPreparatory(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p36","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20b_temporal_validity_read_policy_preparatory")
	if s["version"] != "q20b-p36.v1" {
		t.Fatalf("version=%v, want q20b-p36.v1", s["version"])
	}
	if s["status"] != "contract_only_closed" {
		t.Fatalf("status=%v, want contract_only_closed", s["status"])
	}
	if s["mode"] != "q20b_temporal_validity_read_policy_preparatory_definition" {
		t.Fatalf("mode=%v, want q20b_temporal_validity_read_policy_preparatory_definition", s["mode"])
	}
}

func TestSeq20P37Q20bV1TemporalValidityReadPolicy(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p37","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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
	if s["derives_from"] != "q20a.v1" {
		t.Fatalf("derives_from=%v, want q20a.v1", s["derives_from"])
	}
	if s["mode"] != "q20b_v1_temporal_validity_read_policy_definition" {
		t.Fatalf("mode=%v, want q20b_v1_temporal_validity_read_policy_definition", s["mode"])
	}
}

func TestSeq20P38Q20bReadPriorityModes(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p38","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20b_read_priority_modes")
	if s["version"] != "q20b-p38.v1" {
		t.Fatalf("version=%v, want q20b-p38.v1", s["version"])
	}
	readModes, ok := s["read_modes"].([]any)
	if !ok || len(readModes) != 4 {
		t.Fatalf("read_modes=%v, want 4 items", s["read_modes"])
	}
	if s["mode"] != "q20b_read_priority_modes_definition" {
		t.Fatalf("mode=%v, want q20b_read_priority_modes_definition", s["mode"])
	}
}

func TestSeq20P39Q20bMirroredAtRecallIntentAndQueryClass(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p39","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20b_mirrored_at_recall_intent_and_query_class")
	if s["version"] != "q20b-p39.v1" {
		t.Fatalf("version=%v, want q20b-p39.v1", s["version"])
	}
	mirrorTargets, ok := s["mirror_targets"].([]any)
	if !ok || len(mirrorTargets) != 2 {
		t.Fatalf("mirror_targets=%v, want 2 items", s["mirror_targets"])
	}
	if s["mode"] != "q20b_mirrored_at_recall_intent_and_query_class_definition" {
		t.Fatalf("mode=%v, want q20b_mirrored_at_recall_intent_and_query_class_definition", s["mode"])
	}
}

func TestSeq20P40Q20bStopsBeforeLaterTVWork(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p40","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20b_stops_before_later_tv_work")
	if s["version"] != "q20b-p40.v1" {
		t.Fatalf("version=%v, want q20b-p40.v1", s["version"])
	}
	stopsBefore, ok := s["stops_before"].([]any)
	if !ok || len(stopsBefore) != 3 {
		t.Fatalf("stops_before=%v, want 3 items", s["stops_before"])
	}
	if s["no_backfill"] != true {
		t.Fatalf("no_backfill=%v, want true", s["no_backfill"])
	}
	if s["mode"] != "q20b_stops_before_later_tv_work_definition" {
		t.Fatalf("mode=%v, want q20b_stops_before_later_tv_work_definition", s["mode"])
	}
}

func TestSeq20P47Q20cTemporalEventInvalidationPreparatory(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p47","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20c_temporal_event_invalidation_preparatory")
	if s["version"] != "q20c-p47.v1" {
		t.Fatalf("version=%v, want q20c-p47.v1", s["version"])
	}
	if s["status"] != "contract_only_closed" {
		t.Fatalf("status=%v, want contract_only_closed", s["status"])
	}
	if s["mode"] != "q20c_temporal_event_invalidation_preparatory_definition" {
		t.Fatalf("mode=%v, want q20c_temporal_event_invalidation_preparatory_definition", s["mode"])
	}
}

func TestSeq20P48Q20cV1TemporalEventInvalidationSupport(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p48","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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
	if s["replaces"] != "implicit_event_compare_vs_blocked_current_truth" {
		t.Fatalf("replaces=%v, want implicit_event_compare_vs_blocked_current_truth", s["replaces"])
	}
	if s["mode"] != "q20c_v1_temporal_event_invalidation_support_definition" {
		t.Fatalf("mode=%v, want q20c_v1_temporal_event_invalidation_support_definition", s["mode"])
	}
}

func TestSeq20P49Q20cInvalidationModes(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p49","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20c_invalidation_modes")
	if s["version"] != "q20c-p49.v1" {
		t.Fatalf("version=%v, want q20c-p49.v1", s["version"])
	}
	modes, ok := s["modes"].([]any)
	if !ok || len(modes) != 4 {
		t.Fatalf("modes=%v, want 4 items", s["modes"])
	}
	if s["reuses_vocabulary"] != "direct_evidence_owner" {
		t.Fatalf("reuses_vocabulary=%v, want direct_evidence_owner", s["reuses_vocabulary"])
	}
	if s["mode"] != "q20c_invalidation_modes_definition" {
		t.Fatalf("mode=%v, want q20c_invalidation_modes_definition", s["mode"])
	}
}

func TestSeq20P50Q20cMirroredAtRecallIntent(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p50","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20c_mirrored_at_recall_intent")
	if s["version"] != "q20c-p50.v1" {
		t.Fatalf("version=%v, want q20c-p50.v1", s["version"])
	}
	if s["mirror_payload"] != "temporal_event_invalidation_support" {
		t.Fatalf("mirror_payload=%v, want temporal_event_invalidation_support", s["mirror_payload"])
	}
	if s["mode"] != "q20c_mirrored_at_recall_intent_definition" {
		t.Fatalf("mode=%v, want q20c_mirrored_at_recall_intent_definition", s["mode"])
	}
}

func TestSeq20P51Q20cSeparateFromPromotionLag(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p51","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20c_separate_from_promotion_lag")
	if s["version"] != "q20c-p51.v1" {
		t.Fatalf("version=%v, want q20c-p51.v1", s["version"])
	}
	if s["q20c_owns"] != "event_invalidation_support" {
		t.Fatalf("q20c_owns=%v, want event_invalidation_support", s["q20c_owns"])
	}
	if s["q20d_owns"] != "pending_current_note_criteria" {
		t.Fatalf("q20d_owns=%v, want pending_current_note_criteria", s["q20d_owns"])
	}
	if s["no_overlap"] != true {
		t.Fatalf("no_overlap=%v, want true", s["no_overlap"])
	}
	if s["mode"] != "q20c_separate_from_promotion_lag_definition" {
		t.Fatalf("mode=%v, want q20c_separate_from_promotion_lag_definition", s["mode"])
	}
}

func TestSeq20P57Q20dTemporalPromotionLagPreparatory(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p57","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20d_temporal_promotion_lag_preparatory")
	if s["version"] != "q20d-p57.v1" {
		t.Fatalf("version=%v, want q20d-p57.v1", s["version"])
	}
	if s["status"] != "contract_only_closed" {
		t.Fatalf("status=%v, want contract_only_closed", s["status"])
	}
	if s["mode"] != "q20d_temporal_promotion_lag_preparatory_definition" {
		t.Fatalf("mode=%v, want q20d_temporal_promotion_lag_preparatory_definition", s["mode"])
	}
}

func TestSeq20P58Q20dV1TemporalPromotionLagSupport(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p58","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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
	if s["scope"] != "pending_current_note_emission" {
		t.Fatalf("scope=%v, want pending_current_note_emission", s["scope"])
	}
	activeFor, ok := s["active_for"].([]any)
	if !ok || len(activeFor) != 2 {
		t.Fatalf("active_for=%v, want 2 items", s["active_for"])
	}
	inactiveFor, ok := s["inactive_for"].([]any)
	if !ok || len(inactiveFor) != 1 {
		t.Fatalf("inactive_for=%v, want 1 item", s["inactive_for"])
	}
	if s["mode"] != "q20d_v1_temporal_promotion_lag_support_definition" {
		t.Fatalf("mode=%v, want q20d_v1_temporal_promotion_lag_support_definition", s["mode"])
	}
}

func TestSeq20P59Q20dAnchorPrecedence(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p59","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20d_anchor_precedence")
	if s["version"] != "q20d-p59.v1" {
		t.Fatalf("version=%v, want q20d-p59.v1", s["version"])
	}
	anchorPrecedence, ok := s["anchor_precedence"].([]any)
	if !ok || len(anchorPrecedence) != 2 {
		t.Fatalf("anchor_precedence=%v, want 2 items", s["anchor_precedence"])
	}
	if s["multi_turn_widening"] != "latest_turn_only_deferred_to_q20e" {
		t.Fatalf("multi_turn_widening=%v, want latest_turn_only_deferred_to_q20e", s["multi_turn_widening"])
	}
	if s["current_clock"] != "off" {
		t.Fatalf("current_clock=%v, want off", s["current_clock"])
	}
	if s["chronology"] != "deferred" {
		t.Fatalf("chronology=%v, want deferred", s["chronology"])
	}
	if s["mode"] != "q20d_anchor_precedence_definition" {
		t.Fatalf("mode=%v, want q20d_anchor_precedence_definition", s["mode"])
	}
}

func TestSeq20P60Q20dMirroredAtRecallIntent(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p60","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20d_mirrored_at_recall_intent")
	if s["version"] != "q20d-p60.v1" {
		t.Fatalf("version=%v, want q20d-p60.v1", s["version"])
	}
	if s["mirror_payload"] != "temporal_promotion_lag_support" {
		t.Fatalf("mirror_payload=%v, want temporal_promotion_lag_support", s["mirror_payload"])
	}
	if s["purpose"] != "one_owner_pending_current" {
		t.Fatalf("purpose=%v, want one_owner_pending_current", s["purpose"])
	}
	if s["mode"] != "q20d_mirrored_at_recall_intent_definition" {
		t.Fatalf("mode=%v, want q20d_mirrored_at_recall_intent_definition", s["mode"])
	}
}

func TestSeq20P66Q20eTemporalHotRecallBufferPreparatory(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p66","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20e_temporal_hot_recall_buffer_preparatory")
	if s["version"] != "q20e-p66.v1" {
		t.Fatalf("version=%v, want q20e-p66.v1", s["version"])
	}
	if s["status"] != "contract_only_closed" {
		t.Fatalf("status=%v, want contract_only_closed", s["status"])
	}
	if s["mode"] != "q20e_temporal_hot_recall_buffer_preparatory_definition" {
		t.Fatalf("mode=%v, want q20e_temporal_hot_recall_buffer_preparatory_definition", s["mode"])
	}
}

func TestSeq20P67Q20eV1TemporalHotRecallBuffer(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p67","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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
	if s["widens_from"] != "q20d_single_turn_anchor" {
		t.Fatalf("widens_from=%v, want q20d_single_turn_anchor", s["widens_from"])
	}
	if s["widens_to"] != "recent_multi_turn_bridge" {
		t.Fatalf("widens_to=%v, want recent_multi_turn_bridge", s["widens_to"])
	}
	if s["mode"] != "q20e_v1_temporal_hot_recall_buffer_definition" {
		t.Fatalf("mode=%v, want q20e_v1_temporal_hot_recall_buffer_definition", s["mode"])
	}
}

func TestSeq20P68Q20eBridgeSourceSet(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p68","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20e_bridge_source_set")
	if s["version"] != "q20e-p68.v1" {
		t.Fatalf("version=%v, want q20e-p68.v1", s["version"])
	}
	bridgeSources, ok := s["bridge_sources"].([]any)
	if !ok || len(bridgeSources) != 3 {
		t.Fatalf("bridge_sources=%v, want 3 items", s["bridge_sources"])
	}
	hotWindow, ok := s["hot_window_turns"].(map[string]any)
	if !ok {
		t.Fatalf("hot_window_turns missing or not map")
	}
	if hotWindow["min"] != float64(2) {
		t.Fatalf("hot_window_turns.min=%v, want 2", hotWindow["min"])
	}
	if hotWindow["default"] != float64(3) {
		t.Fatalf("hot_window_turns.default=%v, want 3", hotWindow["default"])
	}
	if hotWindow["max"] != float64(4) {
		t.Fatalf("hot_window_turns.max=%v, want 4", hotWindow["max"])
	}
	if s["support_only"] != true {
		t.Fatalf("support_only=%v, want true", s["support_only"])
	}
	if s["truth_override"] != false {
		t.Fatalf("truth_override=%v, want false", s["truth_override"])
	}
	if s["mode"] != "q20e_bridge_source_set_definition" {
		t.Fatalf("mode=%v, want q20e_bridge_source_set_definition", s["mode"])
	}
}

func TestSeq20P69Q20eMirroredAtRecallIntent(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p69","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20e_mirrored_at_recall_intent")
	if s["version"] != "q20e-p69.v1" {
		t.Fatalf("version=%v, want q20e-p69.v1", s["version"])
	}
	if s["mirror_payload"] != "temporal_hot_recall_buffer" {
		t.Fatalf("mirror_payload=%v, want temporal_hot_recall_buffer", s["mirror_payload"])
	}
	if s["purpose"] != "one_owner_hot_bridge_policy" {
		t.Fatalf("purpose=%v, want one_owner_hot_bridge_policy", s["purpose"])
	}
	if s["mode"] != "q20e_mirrored_at_recall_intent_definition" {
		t.Fatalf("mode=%v, want q20e_mirrored_at_recall_intent_definition", s["mode"])
	}
}

func TestSeq20P76Q20fLightweightEntityIndexPreparatory(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p76","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20f_lightweight_entity_index_preparatory")
	if s["version"] != "q20f-p76.v1" {
		t.Fatalf("version=%v, want q20f-p76.v1", s["version"])
	}
	if s["status"] != "contract_only_closed" {
		t.Fatalf("status=%v, want contract_only_closed", s["status"])
	}
	if s["mode"] != "q20f_lightweight_entity_index_preparatory_definition" {
		t.Fatalf("mode=%v, want q20f_lightweight_entity_index_preparatory_definition", s["mode"])
	}
}

func TestSeq20P77Q20fV1LightweightEntityIndex(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p77","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20f_v1_lightweight_entity_index")
	if s["version"] != "q20f.v1" {
		t.Fatalf("version=%v, want q20f.v1", s["version"])
	}
	if s["replaces"] != "prompt_side_entity_digest_formatting" {
		t.Fatalf("replaces=%v, want prompt_side_entity_digest_formatting", s["replaces"])
	}
	if s["mode"] != "q20f_v1_lightweight_entity_index_definition" {
		t.Fatalf("mode=%v, want q20f_v1_lightweight_entity_index_definition", s["mode"])
	}
}

func TestSeq20P78Q20fStructuredStateSurfaces(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p78","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20f_structured_state_surfaces")
	if s["version"] != "q20f-p78.v1" {
		t.Fatalf("version=%v, want q20f-p78.v1", s["version"])
	}
	labels, ok := s["indexed_labels"].([]any)
	if !ok || len(labels) != 5 {
		t.Fatalf("indexed_labels=%v, want 5 items", s["indexed_labels"])
	}
	if s["generic_entity_tokens_trusted"] != false {
		t.Fatalf("generic_entity_tokens_trusted=%v, want false", s["generic_entity_tokens_trusted"])
	}
	if s["retrieval_boost_only"] != true {
		t.Fatalf("retrieval_boost_only=%v, want true", s["retrieval_boost_only"])
	}
	if s["mode"] != "q20f_structured_state_surfaces_definition" {
		t.Fatalf("mode=%v, want q20f_structured_state_surfaces_definition", s["mode"])
	}
}

func TestSeq20P79Q20fMirroredAtQueryClass(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p79","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20f_mirrored_at_query_class")
	if s["version"] != "q20f-p79.v1" {
		t.Fatalf("version=%v, want q20f-p79.v1", s["version"])
	}
	if s["mirror_payload"] != "lightweight_entity_index" {
		t.Fatalf("mirror_payload=%v, want lightweight_entity_index", s["mirror_payload"])
	}
	if s["mode"] != "q20f_mirrored_at_query_class_definition" {
		t.Fatalf("mode=%v, want q20f_mirrored_at_query_class_definition", s["mode"])
	}
}

func TestSeq20P80Q20fStopsBeforeGraphLikeSupport(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p80","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20f_stops_before_graph_like_support")
	if s["version"] != "q20f-p80.v1" {
		t.Fatalf("version=%v, want q20f-p80.v1", s["version"])
	}
	stopsBefore, ok := s["stops_before"].([]any)
	if !ok || len(stopsBefore) != 2 {
		t.Fatalf("stops_before=%v, want 2 items", s["stops_before"])
	}
	if s["scope"] != "entity_side_index_only" {
		t.Fatalf("scope=%v, want entity_side_index_only", s["scope"])
	}
	if s["no_backfill"] != true {
		t.Fatalf("no_backfill=%v, want true", s["no_backfill"])
	}
	if s["mode"] != "q20f_stops_before_graph_like_support_definition" {
		t.Fatalf("mode=%v, want q20f_stops_before_graph_like_support_definition", s["mode"])
	}
}
