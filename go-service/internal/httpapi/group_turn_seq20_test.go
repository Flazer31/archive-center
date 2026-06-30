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

// ---------------------------------------------------------------------------
// SEQ-20 Preparatory reset/admin tests (P9 ~ P11)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// SEQ-20 q20a temporal query expansion tests (P21 ~ P28)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// SEQ-20 q20b temporal validity read policy tests (P36 ~ P40)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// SEQ-20 q20c temporal event invalidation support tests (P47 ~ P51)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// SEQ-20 q20d temporal promotion-lag support tests (P57 ~ P60)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// SEQ-20 q20e temporal hot recall buffer tests (P66 ~ P69)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// SEQ-20 q20f lightweight entity index tests (P76 ~ P81)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// SEQ-20 q20g graph-like support signal tests (P89 ~ P93)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// SEQ-20 q20h entity/graph boost inspection surface tests (P99 ~ P102)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// SEQ-20 q20i lagging current state boost tests (P109 ~ P112)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// SEQ-20 q20j motive-shadow hint tests (P118 ~ P121)
// ---------------------------------------------------------------------------

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

// ===========================================================================
// SEQ-20 q20k motive-shadow non-escalation guard tests (P127 ~ P129)
// ===========================================================================

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

// ===========================================================================
// SEQ-20 q20l relation edge support ledger tests (P135 ~ P138)
// ===========================================================================

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

// ===========================================================================
// SEQ-20 aggregate summary tests (P231 ~ P236)
// ===========================================================================

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

// ===========================================================================
// SEQ-20 rollup confirmation tests (P240 ~ P254)
// ===========================================================================

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

// TestSeq20P248RollupQ20fLightweightEntityIndex confirms that the q20f
// lightweight entity index surface is present for SEQ-20-P248.
func TestSeq20P248RollupQ20fLightweightEntityIndex(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p248","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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
	if s["mode"] != "q20f_v1_lightweight_entity_index_definition" {
		t.Fatalf("mode=%v, want q20f_v1_lightweight_entity_index_definition", s["mode"])
	}
}

// TestSeq20P249RollupQ20gGraphLikeSupportSignal confirms that the q20g
// graph-like support signal surface is present for SEQ-20-P249.
func TestSeq20P249RollupQ20gGraphLikeSupportSignal(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p249","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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
	if s["mode"] != "q20g_v1_graph_like_support_signal_definition" {
		t.Fatalf("mode=%v, want q20g_v1_graph_like_support_signal_definition", s["mode"])
	}
}

// TestSeq20P250RollupQ20hEntityGraphBoostInspection confirms that the q20h
// entity/graph boost inspection surface is present for SEQ-20-P250.
func TestSeq20P250RollupQ20hEntityGraphBoostInspection(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p250","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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
	if s["mode"] != "q20h_v1_entity_graph_boost_inspection_surface_definition" {
		t.Fatalf("mode=%v, want q20h_v1_entity_graph_boost_inspection_surface_definition", s["mode"])
	}
}

// TestSeq20P251RollupQ20iLaggingCurrentStateBoost confirms that the q20i
// lagging current state boost surface is present for SEQ-20-P251.
func TestSeq20P251RollupQ20iLaggingCurrentStateBoost(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p251","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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
	if s["mode"] != "q20i_v1_lagging_current_state_boost_definition" {
		t.Fatalf("mode=%v, want q20i_v1_lagging_current_state_boost_definition", s["mode"])
	}
}

// TestSeq20P252RollupQ20jMotiveShadowHint confirms that the q20j motive-shadow
// hint surface is present for SEQ-20-P252.
func TestSeq20P252RollupQ20jMotiveShadowHint(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p252","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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
	if s["mode"] != "q20j_v1_motive_shadow_hint_definition" {
		t.Fatalf("mode=%v, want q20j_v1_motive_shadow_hint_definition", s["mode"])
	}
}

// TestSeq20P253RollupQ20kMotiveShadowNonEscalationGuard confirms that the q20k
// motive-shadow non-escalation guard surface is present for SEQ-20-P253.
func TestSeq20P253RollupQ20kMotiveShadowNonEscalationGuard(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p253","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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
	if s["mode"] != "q20k_v1_motive_shadow_non_escalation_guard_definition" {
		t.Fatalf("mode=%v, want q20k_v1_motive_shadow_non_escalation_guard_definition", s["mode"])
	}
}

// TestSeq20P254RollupQ20lRelationEdgeSupportLedger confirms that the q20l
// relation edge support ledger surface is present for SEQ-20-P254.
func TestSeq20P254RollupQ20lRelationEdgeSupportLedger(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p254","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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
	if s["mode"] != "q20l_v1_relation_edge_support_ledger_definition" {
		t.Fatalf("mode=%v, want q20l_v1_relation_edge_support_ledger_definition", s["mode"])
	}
}

// ===========================================================================
// SEQ-20 q20m temporal ambiguity support note tests (P258 ~ P259)
// ===========================================================================

func TestSeq20P258Q20mTemporalAmbiguitySupportNotePreparatory(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p258","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20m_temporal_ambiguity_support_note_preparatory")
	if s["version"] != "q20m-p258.v1" {
		t.Fatalf("version=%v, want q20m-p258.v1", s["version"])
	}
	if s["status"] != "contract_only_closed" {
		t.Fatalf("status=%v, want contract_only_closed", s["status"])
	}
	if s["mode"] != "q20m_temporal_ambiguity_support_note_preparatory_definition" {
		t.Fatalf("mode=%v, want q20m_temporal_ambiguity_support_note_preparatory_definition", s["mode"])
	}
}

func TestSeq20P259Q20mV1TemporalAmbiguitySupportNote(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p259","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20m_v1_temporal_ambiguity_support_note")
	if s["version"] != "q20m.v1" {
		t.Fatalf("version=%v, want q20m.v1", s["version"])
	}
	composes := seq165Slice(t, s, "composes")
	for _, expected := range []string{"q20c_exact_event_compare_note", "q20d_bounded_window_compare_note", "q20e_chronology_support_note"} {
		if !sliceContains(composes, expected) {
			t.Fatalf("composes missing %q: %v", expected, composes)
		}
	}
	if s["disabled_for_current_clock_query"] != true {
		t.Fatalf("disabled_for_current_clock_query=%v, want true", s["disabled_for_current_clock_query"])
	}
	if s["deferred_chronology_gap_visible"] != true {
		t.Fatalf("deferred_chronology_gap_visible=%v, want true", s["deferred_chronology_gap_visible"])
	}
	if s["fake_fill_blocked"] != true {
		t.Fatalf("fake_fill_blocked=%v, want true", s["fake_fill_blocked"])
	}
	if s["mode"] != "q20m_v1_temporal_ambiguity_support_note_definition" {
		t.Fatalf("mode=%v, want q20m_v1_temporal_ambiguity_support_note_definition", s["mode"])
	}
}

// ===========================================================================
// SEQ-20 q20n alias/entity conflict disambiguation tests (P260 ~ P261)
// ===========================================================================

func TestSeq20P260Q20nAliasEntityConflictDisambiguationPreparatory(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p260","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20n_alias_entity_conflict_disambiguation_preparatory")
	if s["version"] != "q20n-p260.v1" {
		t.Fatalf("version=%v, want q20n-p260.v1", s["version"])
	}
	if s["status"] != "contract_only_closed" {
		t.Fatalf("status=%v, want contract_only_closed", s["status"])
	}
	if s["mode"] != "q20n_alias_entity_conflict_disambiguation_preparatory_definition" {
		t.Fatalf("mode=%v, want q20n_alias_entity_conflict_disambiguation_preparatory_definition", s["mode"])
	}
}

func TestSeq20P261Q20nV1AliasEntityConflictDisambiguation(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p261","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20n_v1_alias_entity_conflict_disambiguation")
	if s["version"] != "q20n.v1" {
		t.Fatalf("version=%v, want q20n.v1", s["version"])
	}
	if s["explicit_alias_table"] != false {
		t.Fatalf("explicit_alias_table=%v, want false", s["explicit_alias_table"])
	}
	if s["structured_label_collision_only"] != true {
		t.Fatalf("structured_label_collision_only=%v, want true", s["structured_label_collision_only"])
	}
	if s["auto_resolution"] != false {
		t.Fatalf("auto_resolution=%v, want false", s["auto_resolution"])
	}
	if s["output"] != "candidate_entries_only" {
		t.Fatalf("output=%v, want candidate_entries_only", s["output"])
	}
	if s["mode"] != "q20n_v1_alias_entity_conflict_disambiguation_definition" {
		t.Fatalf("mode=%v, want q20n_v1_alias_entity_conflict_disambiguation_definition", s["mode"])
	}
}

// ===========================================================================
// SEQ-20 q20o temporal/entity support block source-tag rule tests (P262 ~ P263)
// ===========================================================================

func TestSeq20P262Q20oTemporalEntitySourceTagRulePreparatory(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p262","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20o_temporal_entity_source_tag_rule_preparatory")
	if s["version"] != "q20o-p262.v1" {
		t.Fatalf("version=%v, want q20o-p262.v1", s["version"])
	}
	if s["status"] != "contract_only_closed" {
		t.Fatalf("status=%v, want contract_only_closed", s["status"])
	}
	if s["mode"] != "q20o_temporal_entity_source_tag_rule_preparatory_definition" {
		t.Fatalf("mode=%v, want q20o_temporal_entity_source_tag_rule_preparatory_definition", s["mode"])
	}
}

func TestSeq20P263Q20oV1TemporalEntitySourceTagRule(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p263","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20o_v1_temporal_entity_source_tag_rule")
	if s["version"] != "q20o.v1" {
		t.Fatalf("version=%v, want q20o.v1", s["version"])
	}
	catalogs := seq165Slice(t, s, "source_catalogs")
	for _, expected := range []string{"source_surfaces", "source_catalogs", "source_lanes"} {
		if !sliceContains(catalogs, expected) {
			t.Fatalf("source_catalogs missing %q: %v", expected, catalogs)
		}
	}
	if s["relabeling_blocked"] != true {
		t.Fatalf("relabeling_blocked=%v, want true", s["relabeling_blocked"])
	}
	if s["fake_alias_blocked"] != true {
		t.Fatalf("fake_alias_blocked=%v, want true", s["fake_alias_blocked"])
	}
	if s["mode"] != "q20o_v1_temporal_entity_source_tag_rule_definition" {
		t.Fatalf("mode=%v, want q20o_v1_temporal_entity_source_tag_rule_definition", s["mode"])
	}
}

// ===========================================================================
// SEQ-20 q20p canonical-pending/stale-current conflict note tests (P264 ~ P265)
// ===========================================================================

func TestSeq20P264Q20pCanonicalPendingStaleCurrentConflictNotePreparatory(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p264","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20p_canonical_pending_stale_current_conflict_note_preparatory")
	if s["version"] != "q20p-p264.v1" {
		t.Fatalf("version=%v, want q20p-p264.v1", s["version"])
	}
	if s["status"] != "contract_only_closed" {
		t.Fatalf("status=%v, want contract_only_closed", s["status"])
	}
	if s["mode"] != "q20p_canonical_pending_stale_current_conflict_note_preparatory_definition" {
		t.Fatalf("mode=%v, want q20p_canonical_pending_stale_current_conflict_note_preparatory_definition", s["mode"])
	}
}

func TestSeq20P265Q20pV1CanonicalPendingStaleCurrentConflictNote(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p265","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20p_v1_canonical_pending_stale_current_conflict_note")
	if s["version"] != "q20p.v1" {
		t.Fatalf("version=%v, want q20p.v1", s["version"])
	}
	assembles := seq165Slice(t, s, "assembles")
	for _, expected := range []string{"pending_current_gap", "hot_buffer", "optional_lagging_boost"} {
		if !sliceContains(assembles, expected) {
			t.Fatalf("assembles missing %q: %v", expected, assembles)
		}
	}
	if s["read_only_conflict_note"] != true {
		t.Fatalf("read_only_conflict_note=%v, want true", s["read_only_conflict_note"])
	}
	if s["mode"] != "q20p_v1_canonical_pending_stale_current_conflict_note_definition" {
		t.Fatalf("mode=%v, want q20p_v1_canonical_pending_stale_current_conflict_note_definition", s["mode"])
	}
}

// ===========================================================================
// SEQ-20 q20q recall cue rescue rule tests (P266 ~ P267)
// ===========================================================================

func TestSeq20P266Q20qRecallCueRescueRulePreparatory(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p266","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20q_recall_cue_rescue_rule_preparatory")
	if s["version"] != "q20q-p266.v1" {
		t.Fatalf("version=%v, want q20q-p266.v1", s["version"])
	}
	if s["status"] != "contract_only_closed" {
		t.Fatalf("status=%v, want contract_only_closed", s["status"])
	}
	if s["mode"] != "q20q_recall_cue_rescue_rule_preparatory_definition" {
		t.Fatalf("mode=%v, want q20q_recall_cue_rescue_rule_preparatory_definition", s["mode"])
	}
}

func TestSeq20P267Q20qV1RecallCueRescueRule(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p267","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20q_v1_recall_cue_rescue_rule")
	if s["version"] != "q20q.v1" {
		t.Fatalf("version=%v, want q20q.v1", s["version"])
	}
	reuses := seq165Slice(t, s, "reuses")
	for _, expected := range []string{"q20a_temporal_expansion", "qr1a_callback_lexical_signal", "qr1d_old_detail_signal"} {
		if !sliceContains(reuses, expected) {
			t.Fatalf("reuses missing %q: %v", expected, reuses)
		}
	}
	if s["new_cue_table"] != false {
		t.Fatalf("new_cue_table=%v, want false", s["new_cue_table"])
	}
	if s["recall_widening_only"] != true {
		t.Fatalf("recall_widening_only=%v, want true", s["recall_widening_only"])
	}
	if s["plain_detail_request_excluded"] != true {
		t.Fatalf("plain_detail_request_excluded=%v, want true", s["plain_detail_request_excluded"])
	}
	if s["mode"] != "q20q_v1_recall_cue_rescue_rule_definition" {
		t.Fatalf("mode=%v, want q20q_v1_recall_cue_rescue_rule_definition", s["mode"])
	}
}

// ===========================================================================
// SEQ-20 q20r wide gather -> validity join rule tests (P268 ~ P269)
// ===========================================================================

func TestSeq20P268Q20rWideGatherValidityJoinRulePreparatory(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p268","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20r_wide_gather_validity_join_rule_preparatory")
	if s["version"] != "q20r-p268.v1" {
		t.Fatalf("version=%v, want q20r-p268.v1", s["version"])
	}
	if s["status"] != "contract_only_closed" {
		t.Fatalf("status=%v, want contract_only_closed", s["status"])
	}
	if s["mode"] != "q20r_wide_gather_validity_join_rule_preparatory_definition" {
		t.Fatalf("mode=%v, want q20r_wide_gather_validity_join_rule_preparatory_definition", s["mode"])
	}
}

func TestSeq20P269Q20rV1WideGatherValidityJoinRule(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p269","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20r_v1_wide_gather_validity_join_rule")
	if s["version"] != "q20r.v1" {
		t.Fatalf("version=%v, want q20r.v1", s["version"])
	}
	reuses := seq165Slice(t, s, "reuses")
	for _, expected := range []string{"q20b_read_priority", "q20m_compare_note_surface", "q20q_rescue_surface"} {
		if !sliceContains(reuses, expected) {
			t.Fatalf("reuses missing %q: %v", expected, reuses)
		}
	}
	authorities := seq165Slice(t, s, "validity_join_authorities")
	for _, expected := range []string{"mariadb_canonical_truth", "storyline_status", "temporal_validity"} {
		if !sliceContains(authorities, expected) {
			t.Fatalf("validity_join_authorities missing %q: %v", expected, authorities)
		}
	}
	if s["bounded_wide_gather"] != true {
		t.Fatalf("bounded_wide_gather=%v, want true", s["bounded_wide_gather"])
	}
	if s["validity_join_for_temporal_queries_only"] != true {
		t.Fatalf("validity_join_for_temporal_queries_only=%v, want true", s["validity_join_for_temporal_queries_only"])
	}
	if s["callback_only_recall_fail_open"] != true {
		t.Fatalf("callback_only_recall_fail_open=%v, want true", s["callback_only_recall_fail_open"])
	}
	if s["mode"] != "q20r_v1_wide_gather_validity_join_rule_definition" {
		t.Fatalf("mode=%v, want q20r_v1_wide_gather_validity_join_rule_definition", s["mode"])
	}
}

// ===========================================================================
// SEQ-20 q20s thin support tag fallback tests (P270 ~ P271)
// ===========================================================================

func TestSeq20P270Q20sThinSupportTagFallbackPreparatory(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p270","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20s_thin_support_tag_fallback_preparatory")
	if s["version"] != "q20s-p270.v1" {
		t.Fatalf("version=%v, want q20s-p270.v1", s["version"])
	}
	if s["status"] != "contract_only_closed" {
		t.Fatalf("status=%v, want contract_only_closed", s["status"])
	}
	if s["mode"] != "q20s_thin_support_tag_fallback_preparatory_definition" {
		t.Fatalf("mode=%v, want q20s_thin_support_tag_fallback_preparatory_definition", s["mode"])
	}
}

func TestSeq20P271Q20sV1ThinSupportTagFallback(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p271","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "q20s_v1_thin_support_tag_fallback")
	if s["version"] != "q20s.v1" {
		t.Fatalf("version=%v, want q20s.v1", s["version"])
	}
	reuses := seq165Slice(t, s, "reuses")
	for _, expected := range []string{"q20e_thin_tag_mode", "q20o_source_tags"} {
		if !sliceContains(reuses, expected) {
			t.Fatalf("reuses missing %q: %v", expected, reuses)
		}
	}
	if s["low_density_support_visibility"] != true {
		t.Fatalf("low_density_support_visibility=%v, want true", s["low_density_support_visibility"])
	}
	if s["requires_prior_validity_join"] != true {
		t.Fatalf("requires_prior_validity_join=%v, want true", s["requires_prior_validity_join"])
	}
	if s["drop_replacement"] != "thin_support_tag" {
		t.Fatalf("drop_replacement=%v, want thin_support_tag", s["drop_replacement"])
	}
	if s["mode"] != "q20s_v1_thin_support_tag_fallback_definition" {
		t.Fatalf("mode=%v, want q20s_v1_thin_support_tag_fallback_definition", s["mode"])
	}
}

// ===========================================================================
// SEQ-20 vx20a~vx20g validation replay gate tests (P286 ~ P299)
// ===========================================================================

func TestSeq20P286Vx20aTemporalValidityReplayGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p286","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vx20a_temporal_validity_replay_gate")
	if s["version"] != "vx20a.v1" {
		t.Fatalf("version=%v, want vx20a.v1", s["version"])
	}
	refs := seq165Slice(t, s, "references")
	for _, expected := range []string{"q20b_read_priority", "q20m_compare_note_surface", "q20r_join_mode"} {
		if !sliceContains(refs, expected) {
			t.Fatalf("references missing %q: %v", expected, refs)
		}
	}
	if s["check_target"] != "temporal_validity_contract_drift_only" {
		t.Fatalf("check_target=%v, want temporal_validity_contract_drift_only", s["check_target"])
	}
	if s["mode"] != "vx20a_v1_temporal_validity_replay_gate_definition" {
		t.Fatalf("mode=%v, want vx20a_v1_temporal_validity_replay_gate_definition", s["mode"])
	}
}

func TestSeq20P288Vx20bEntityBoostFalsePositiveGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p288","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vx20b_entity_boost_false_positive_gate")
	if s["version"] != "vx20b.v1" {
		t.Fatalf("version=%v, want vx20b.v1", s["version"])
	}
	refs := seq165Slice(t, s, "references")
	for _, expected := range []string{"q20h_inspection_surface", "q20i_lagging_boost_surface"} {
		if !sliceContains(refs, expected) {
			t.Fatalf("references missing %q: %v", expected, refs)
		}
	}
	if s["false_positive_ceiling"] != "lc1j_reuse" {
		t.Fatalf("false_positive_ceiling=%v, want lc1j_reuse", s["false_positive_ceiling"])
	}
	if s["mode"] != "vx20b_v1_entity_boost_false_positive_gate_definition" {
		t.Fatalf("mode=%v, want vx20b_v1_entity_boost_false_positive_gate_definition", s["mode"])
	}
}

func TestSeq20P290Vx20cGraphAcceleratorDegradeGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p290","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vx20c_graph_accelerator_degrade_gate")
	if s["version"] != "vx20c.v1" {
		t.Fatalf("version=%v, want vx20c.v1", s["version"])
	}
	refs := seq165Slice(t, s, "references")
	for _, expected := range []string{"q20g_graph_like_support", "q20h_inspection_surface", "q20i_lagging_boost_surface"} {
		if !sliceContains(refs, expected) {
			t.Fatalf("references missing %q: %v", expected, refs)
		}
	}
	if s["degrade_scenario"] != "graph_accelerator_off" {
		t.Fatalf("degrade_scenario=%v, want graph_accelerator_off", s["degrade_scenario"])
	}
	if s["fail_open_required"] != true {
		t.Fatalf("fail_open_required=%v, want true", s["fail_open_required"])
	}
	if s["entity_boost_survives"] != true {
		t.Fatalf("entity_boost_survives=%v, want true", s["entity_boost_survives"])
	}
	if s["mode"] != "vx20c_v1_graph_accelerator_degrade_gate_definition" {
		t.Fatalf("mode=%v, want vx20c_v1_graph_accelerator_degrade_gate_definition", s["mode"])
	}
}

func TestSeq20P292Vx20dCanonicalPrecedenceReplayGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p292","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vx20d_canonical_precedence_replay_gate")
	if s["version"] != "vx20d.v1" {
		t.Fatalf("version=%v, want vx20d.v1", s["version"])
	}
	refs := seq165Slice(t, s, "references")
	for _, expected := range []string{"q20p_source_precedence", "q20r_join_authority_ordering"} {
		if !sliceContains(refs, expected) {
			t.Fatalf("references missing %q: %v", expected, refs)
		}
	}
	if s["check_target"] != "canon_support_precedence_drift_only" {
		t.Fatalf("check_target=%v, want canon_support_precedence_drift_only", s["check_target"])
	}
	if s["mode"] != "vx20d_v1_canonical_precedence_replay_gate_definition" {
		t.Fatalf("mode=%v, want vx20d_v1_canonical_precedence_replay_gate_definition", s["mode"])
	}
}

func TestSeq20P294Vx20ePromotionBlockedFreshnessReplayGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p294","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vx20e_promotion_blocked_freshness_replay_gate")
	if s["version"] != "vx20e.v1" {
		t.Fatalf("version=%v, want vx20e.v1", s["version"])
	}
	refs := seq165Slice(t, s, "references")
	for _, expected := range []string{"q20d_promotion_lag_support", "q20e_hot_recall_buffer", "q20p_canonical_pending_conflict", "q20i_lagging_boost"} {
		if !sliceContains(refs, expected) {
			t.Fatalf("references missing %q: %v", expected, refs)
		}
	}
	if s["check_target"] != "pending_current_visibility_preservation" {
		t.Fatalf("check_target=%v, want pending_current_visibility_preservation", s["check_target"])
	}
	if s["mode"] != "vx20e_v1_promotion_blocked_freshness_replay_gate_definition" {
		t.Fatalf("mode=%v, want vx20e_v1_promotion_blocked_freshness_replay_gate_definition", s["mode"])
	}
}

func TestSeq20P296Vx20fRecallCueRescueReplayGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p296","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vx20f_recall_cue_rescue_replay_gate")
	if s["version"] != "vx20f.v1" {
		t.Fatalf("version=%v, want vx20f.v1", s["version"])
	}
	refs := seq165Slice(t, s, "references")
	if !sliceContains(refs, "q20q_rescue_surface") {
		t.Fatalf("references missing q20q_rescue_surface: %v", refs)
	}
	if s["check_target"] != "over_filter_miss_reduction" {
		t.Fatalf("check_target=%v, want over_filter_miss_reduction", s["check_target"])
	}
	if s["stale_arc_auto_foreground"] != false {
		t.Fatalf("stale_arc_auto_foreground=%v, want false", s["stale_arc_auto_foreground"])
	}
	if s["mode"] != "vx20f_v1_recall_cue_rescue_replay_gate_definition" {
		t.Fatalf("mode=%v, want vx20f_v1_recall_cue_rescue_replay_gate_definition", s["mode"])
	}
}

func TestSeq20P298Vx20gHotBufferWideGatherNonRegressionGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p298","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vx20g_hot_buffer_wide_gather_non_regression_gate")
	if s["version"] != "vx20g.v1" {
		t.Fatalf("version=%v, want vx20g.v1", s["version"])
	}
	refs := seq165Slice(t, s, "references")
	for _, expected := range []string{"q20e_hot_recall_buffer", "q20r_wide_gather_validity_join", "q20s_thin_support_tag_fallback", "vx18c_upstream_gate", "vx18d_upstream_gate"} {
		if !sliceContains(refs, expected) {
			t.Fatalf("references missing %q: %v", expected, refs)
		}
	}
	if s["check_target"] != "truth_boundary_latency_ceiling" {
		t.Fatalf("check_target=%v, want truth_boundary_latency_ceiling", s["check_target"])
	}
	if s["step20_closeout"] != true {
		t.Fatalf("step20_closeout=%v, want true", s["step20_closeout"])
	}
	if s["mode"] != "vx20g_v1_hot_buffer_wide_gather_non_regression_gate_definition" {
		t.Fatalf("mode=%v, want vx20g_v1_hot_buffer_wide_gather_non_regression_gate_definition", s["mode"])
	}
}

// ===========================================================================
// SEQ-20 Beta 1.1 release smoke gate tests (P312 ~ P316)
// ===========================================================================

func TestSeq20P312Beta11BundleDryRun(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p312","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq20_beta11_bundle_dry_run")
	if s["version"] != "s20-p312.v1" {
		t.Fatalf("version=%v, want s20-p312.v1", s["version"])
	}
	if s["dry_run"] != true {
		t.Fatalf("dry_run=%v, want true", s["dry_run"])
	}
	if s["mode"] != "seq20_beta11_bundle_dry_run_definition" {
		t.Fatalf("mode=%v, want seq20_beta11_bundle_dry_run_definition", s["mode"])
	}
}

func TestSeq20P313TemporalValidityRecallSmoke(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p313","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq20_temporal_validity_recall_smoke")
	if s["version"] != "s20-p313.v1" {
		t.Fatalf("version=%v, want s20-p313.v1", s["version"])
	}
	if s["smoke_target"] != "temporal_validity_recall" {
		t.Fatalf("smoke_target=%v, want temporal_validity_recall", s["smoke_target"])
	}
	refs := seq165Slice(t, s, "references")
	for _, expected := range []string{"q20a_temporal_query_expansion", "q20b_temporal_validity_read_policy", "q20m_temporal_ambiguity_support_note"} {
		if !sliceContains(refs, expected) {
			t.Fatalf("references missing %q: %v", expected, refs)
		}
	}
	if s["mode"] != "seq20_temporal_validity_recall_smoke_definition" {
		t.Fatalf("mode=%v, want seq20_temporal_validity_recall_smoke_definition", s["mode"])
	}
}

func TestSeq20P314EntityGraphAcceleratorSmoke(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p314","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq20_entity_graph_accelerator_smoke")
	if s["version"] != "s20-p314.v1" {
		t.Fatalf("version=%v, want s20-p314.v1", s["version"])
	}
	if s["smoke_target"] != "entity_graph_accelerator" {
		t.Fatalf("smoke_target=%v, want entity_graph_accelerator", s["smoke_target"])
	}
	refs := seq165Slice(t, s, "references")
	for _, expected := range []string{"q20f_lightweight_entity_index", "q20g_graph_like_support_signal", "q20h_entity_graph_boost_inspection", "q20i_lagging_current_state_boost"} {
		if !sliceContains(refs, expected) {
			t.Fatalf("references missing %q: %v", expected, refs)
		}
	}
	if s["mode"] != "seq20_entity_graph_accelerator_smoke_definition" {
		t.Fatalf("mode=%v, want seq20_entity_graph_accelerator_smoke_definition", s["mode"])
	}
}

func TestSeq20P315TemporalEntityDisambiguationSmoke(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p315","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq20_temporal_entity_disambiguation_smoke")
	if s["version"] != "s20-p315.v1" {
		t.Fatalf("version=%v, want s20-p315.v1", s["version"])
	}
	if s["smoke_target"] != "temporal_entity_disambiguation" {
		t.Fatalf("smoke_target=%v, want temporal_entity_disambiguation", s["smoke_target"])
	}
	refs := seq165Slice(t, s, "references")
	for _, expected := range []string{"q20n_alias_entity_conflict_disambiguation", "q20o_temporal_entity_source_tag_rule", "q20q_recall_cue_rescue_rule", "q20r_wide_gather_validity_join_rule"} {
		if !sliceContains(refs, expected) {
			t.Fatalf("references missing %q: %v", expected, refs)
		}
	}
	if s["mode"] != "seq20_temporal_entity_disambiguation_smoke_definition" {
		t.Fatalf("mode=%v, want seq20_temporal_entity_disambiguation_smoke_definition", s["mode"])
	}
}

func TestSeq20P316PrecedenceAmbiguityReviewChecklist(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p316","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq20_precedence_ambiguity_review_checklist")
	if s["version"] != "s20-p316.v1" {
		t.Fatalf("version=%v, want s20-p316.v1", s["version"])
	}
	items := seq165Slice(t, s, "checklist_items")
	for _, expected := range []string{"canonical_precedence_preserved", "support_lane_read_only", "ambiguity_reduction_active", "truth_boundary_intact"} {
		if !sliceContains(items, expected) {
			t.Fatalf("checklist_items missing %q: %v", expected, items)
		}
	}
	if s["mode"] != "seq20_precedence_ambiguity_review_checklist_definition" {
		t.Fatalf("mode=%v, want seq20_precedence_ambiguity_review_checklist_definition", s["mode"])
	}
}

// ===========================================================================
// SEQ-20 final preserve summary tests (P330 ~ P333)
// ===========================================================================

func TestSeq20P330TemporalQueryExpansionPreserve(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p330","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq20_temporal_query_expansion_preserve")
	if s["version"] != "s20-p330.v1" {
		t.Fatalf("version=%v, want s20-p330.v1", s["version"])
	}
	preserved := seq165Slice(t, s, "preserved")
	for _, expected := range []string{"rule_first_temporal_expansion", "metadata_support_confirmation"} {
		if !sliceContains(preserved, expected) {
			t.Fatalf("preserved missing %q: %v", expected, preserved)
		}
	}
	if s["mode"] != "seq20_temporal_query_expansion_preserve_definition" {
		t.Fatalf("mode=%v, want seq20_temporal_query_expansion_preserve_definition", s["mode"])
	}
}

func TestSeq20P331EntityIndexPreserve(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p331","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq20_entity_index_preserve")
	if s["version"] != "s20-p331.v1" {
		t.Fatalf("version=%v, want s20-p331.v1", s["version"])
	}
	preserved := seq165Slice(t, s, "preserved")
	for _, expected := range []string{"lightweight_entity_event_axis", "bounded_relation_edge_summary_granularity"} {
		if !sliceContains(preserved, expected) {
			t.Fatalf("preserved missing %q: %v", expected, preserved)
		}
	}
	if s["mode"] != "seq20_entity_index_preserve_definition" {
		t.Fatalf("mode=%v, want seq20_entity_index_preserve_definition", s["mode"])
	}
}

func TestSeq20P332GraphAcceleratorPreserve(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p332","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq20_graph_accelerator_preserve")
	if s["version"] != "s20-p332.v1" {
		t.Fatalf("version=%v, want s20-p332.v1", s["version"])
	}
	preserved := seq165Slice(t, s, "preserved")
	for _, expected := range []string{"structured_edge_signal_degraded_optional_off", "entity_side_fail_open_path"} {
		if !sliceContains(preserved, expected) {
			t.Fatalf("preserved missing %q: %v", expected, preserved)
		}
	}
	if s["mode"] != "seq20_graph_accelerator_preserve_definition" {
		t.Fatalf("mode=%v, want seq20_graph_accelerator_preserve_definition", s["mode"])
	}
}

func TestSeq20P333AmbiguitySupportNotePreserve(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq20-p333","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq20_ambiguity_support_note_preserve")
	if s["version"] != "s20-p333.v1" {
		t.Fatalf("version=%v, want s20-p333.v1", s["version"])
	}
	preserved := seq165Slice(t, s, "preserved")
	for _, expected := range []string{"primary_compare_source_tag_join_context", "bounded_semi_structured_note"} {
		if !sliceContains(preserved, expected) {
			t.Fatalf("preserved missing %q: %v", expected, preserved)
		}
	}
	if s["mode"] != "seq20_ambiguity_support_note_preserve_definition" {
		t.Fatalf("mode=%v, want seq20_ambiguity_support_note_preserve_definition", s["mode"])
	}
}
