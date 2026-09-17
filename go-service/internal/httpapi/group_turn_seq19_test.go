package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSeq19P9ResetAdmin(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p9","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "reset_admin_19")
	if s["version"] != "seq19_p9.v1" {
		t.Fatalf("version=%v, want seq19_p9.v1", s["version"])
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

func TestSeq19P10HistoricalContentPreserved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p10","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "historical_content_preserved_19")
	if s["version"] != "seq19_p10.v1" {
		t.Fatalf("version=%v, want seq19_p10.v1", s["version"])
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

func TestSeq19P11ResetNoteOnly(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p11","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "reset_note_only_19")
	if s["version"] != "seq19_p11.v1" {
		t.Fatalf("version=%v, want seq19_p11.v1", s["version"])
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

func TestSeq19P15TemporalStateSurface(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p15","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "temporal_state")
	if s["version"] != "sc19a.v1" {
		t.Fatalf("version=%v, want sc19a.v1", s["version"])
	}
	if s["role"] != "temporal_state" {
		t.Fatalf("role=%v, want temporal_state", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}

	if _, ok := s["current_story_clock"]; !ok {
		t.Fatal("missing current_story_clock")
	}

	if _, ok := s["temporal_relation_ledger"]; !ok {
		t.Fatal("missing temporal_relation_ledger")
	}

	if _, ok := s["elapsed_time_decision"]; !ok {
		t.Fatal("missing elapsed_time_decision")
	}

	if _, ok := s["clock_write_directive"]; !ok {
		t.Fatal("missing clock_write_directive")
	}
	if s["mode"] != "temporal_state_surface" {
		t.Fatalf("mode=%v, want temporal_state_surface", s["mode"])
	}
}

func TestSeq19P16Sc19aV1Surface(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p16","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "temporal_state")
	if s["version"] != "sc19a.v1" {
		t.Fatalf("version=%v, want sc19a.v1", s["version"])
	}

	clock, ok := s["current_story_clock"].(map[string]any)
	if !ok {
		t.Fatal("current_story_clock is not a map")
	}
	if _, ok := clock["raw_value"]; !ok {
		t.Fatal("current_story_clock missing raw_value")
	}
	if _, ok := clock["resolution_source"]; !ok {
		t.Fatal("current_story_clock missing resolution_source")
	}
	if _, ok := clock["precision_label"]; !ok {
		t.Fatal("current_story_clock missing precision_label")
	}
}

func TestSeq19P17ResolutionPrecedence(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p17","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "current_story_clock_resolution")
	if s["version"] != "s19-p17.v1" {
		t.Fatalf("version=%v, want s19-p17.v1", s["version"])
	}
	if s["role"] != "current_story_clock_resolution" {
		t.Fatalf("role=%v, want current_story_clock_resolution", s["role"])
	}
	precedence, ok := s["precedence_chain"].([]any)
	if !ok || len(precedence) < 4 {
		t.Fatalf("precedence_chain=%v, want 4 items", s["precedence_chain"])
	}
	if precedence[0] != "session_state_clock" {
		t.Fatalf("precedence[0]=%v, want session_state_clock", precedence[0])
	}
	if precedence[1] != "input_current_scene_anchor" {
		t.Fatalf("precedence[1]=%v, want input_current_scene_anchor", precedence[1])
	}
	if precedence[2] != "timeline_anchor" {
		t.Fatalf("precedence[2]=%v, want timeline_anchor", precedence[2])
	}
	if precedence[3] != "carry_forward" {
		t.Fatalf("precedence[3]=%v, want carry_forward", precedence[3])
	}
	if s["mode"] != "current_story_clock_resolution_precedence" {
		t.Fatalf("mode=%v, want current_story_clock_resolution_precedence", s["mode"])
	}
}

func TestSeq19P18PrecisionLabelContract(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p18","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "precision_label_contract")
	if s["version"] != "s19-p18.v1" {
		t.Fatalf("version=%v, want s19-p18.v1", s["version"])
	}
	labels, ok := s["canonical_labels"].([]any)
	if !ok || len(labels) != 4 {
		t.Fatalf("canonical_labels=%v, want 4 items", s["canonical_labels"])
	}
	if s["coarse_collapsed_to"] != "bounded_range" {
		t.Fatalf("coarse_collapsed_to=%v, want bounded_range", s["coarse_collapsed_to"])
	}
	if s["mode"] != "precision_label_contract_definition" {
		t.Fatalf("mode=%v, want precision_label_contract_definition", s["mode"])
	}
}

func TestSeq19P19InvalidUnknownDegradation(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p19","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "invalid_unknown_degradation")
	if s["version"] != "s19-p19.v1" {
		t.Fatalf("version=%v, want s19-p19.v1", s["version"])
	}
	if s["invalid_degrades_to"] != "unknown" {
		t.Fatalf("invalid_degrades_to=%v, want unknown", s["invalid_degrades_to"])
	}
	if s["unknown_action"] != "no_advance" {
		t.Fatalf("unknown_action=%v, want no_advance", s["unknown_action"])
	}
	if s["coarse_collapsed_to"] != "bounded_range" {
		t.Fatalf("coarse_collapsed_to=%v, want bounded_range", s["coarse_collapsed_to"])
	}
	if s["mode"] != "invalid_unknown_degradation_rule" {
		t.Fatalf("mode=%v, want invalid_unknown_degradation_rule", s["mode"])
	}
}

func TestSeq19P20TemporalSplitRule(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p20","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "temporal_split_rule")
	if s["version"] != "s19-p20.v1" {
		t.Fatalf("version=%v, want s19-p20.v1", s["version"])
	}
	if s["write_lane"] != "current_scene" {
		t.Fatalf("write_lane=%v, want current_scene", s["write_lane"])
	}
	targets, ok := s["relation_only_targets"].([]any)
	if !ok || len(targets) != 4 {
		t.Fatalf("relation_only_targets=%v, want 4 items", s["relation_only_targets"])
	}
	if s["mode"] != "temporal_split_rule_definition" {
		t.Fatalf("mode=%v, want temporal_split_rule_definition", s["mode"])
	}
}

func TestSeq19P21StoryClockSurfaceGuard(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p21","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "story_clock_surface_guard")
	if s["version"] != "s19-p21.v1" {
		t.Fatalf("version=%v, want s19-p21.v1", s["version"])
	}
	cases, ok := s["guard_cases"].([]any)
	if !ok || len(cases) != 6 {
		t.Fatalf("guard_cases=%v, want 6 items", s["guard_cases"])
	}
	if s["mode"] != "story_clock_surface_guard_definition" {
		t.Fatalf("mode=%v, want story_clock_surface_guard_definition", s["mode"])
	}
}

func TestSeq19P22RegressionBundle(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p22","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "step18_plus_19_regression_bundle")
	if s["version"] != "s19-p22.v1" {
		t.Fatalf("version=%v, want s19-p22.v1", s["version"])
	}
	if s["regression_status"] != "green" {
		t.Fatalf("regression_status=%v, want green", s["regression_status"])
	}
	if s["combined_read_path"] != true {
		t.Fatalf("combined_read_path=%v, want true", s["combined_read_path"])
	}
	if s["mode"] != "step18_plus_19_regression_bundle_status" {
		t.Fatalf("mode=%v, want step18_plus_19_regression_bundle_status", s["mode"])
	}
}

func TestSeq19P30TemporalRelationLedgerCanonical(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p30","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "temporal_relation_ledger_canonical")
	if s["version"] != "s19-p30.v1" {
		t.Fatalf("version=%v, want s19-p30.v1", s["version"])
	}
	if s["schema_format"] != "snake_case" {
		t.Fatalf("schema_format=%v, want snake_case", s["schema_format"])
	}
	keys, ok := s["canonical_keys"].([]any)
	if !ok || len(keys) != 13 {
		t.Fatalf("canonical_keys count=%v, want 13", len(keys))
	}
	if s["mode"] != "temporal_relation_ledger_canonical_schema" {
		t.Fatalf("mode=%v, want temporal_relation_ledger_canonical_schema", s["mode"])
	}
}

func TestSeq19P31SchemaPhraseIngress(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p31","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "schema_phrase_ingress")
	if s["version"] != "s19-p31.v1" {
		t.Fatalf("version=%v, want s19-p31.v1", s["version"])
	}
	if s["ingress_supported"] != true {
		t.Fatalf("ingress_supported=%v, want true", s["ingress_supported"])
	}
	if s["mode"] != "schema_phrase_ingress_normalization" {
		t.Fatalf("mode=%v, want schema_phrase_ingress_normalization", s["mode"])
	}
}

func TestSeq19P32SchemaOwnerBlock(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p32","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "schema_owner_block")
	if s["version"] != "s19-p32.v1" {
		t.Fatalf("version=%v, want s19-p32.v1", s["version"])
	}
	contains, ok := s["contains"].([]any)
	if !ok || len(contains) != 4 {
		t.Fatalf("contains=%v, want 4 items", s["contains"])
	}
	if s["mode"] != "schema_owner_block_definition" {
		t.Fatalf("mode=%v, want schema_owner_block_definition", s["mode"])
	}
}

func TestSeq19P33CanonicalDataOverrideGuard(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p33","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "canonical_data_override_guard")
	if s["version"] != "s19-p33.v1" {
		t.Fatalf("version=%v, want s19-p33.v1", s["version"])
	}
	if s["override_allowed"] != false {
		t.Fatalf("override_allowed=%v, want false", s["override_allowed"])
	}
	if s["explicit_fields_win"] != true {
		t.Fatalf("explicit_fields_win=%v, want true", s["explicit_fields_win"])
	}
	if s["mode"] != "canonical_data_override_guard_definition" {
		t.Fatalf("mode=%v, want canonical_data_override_guard_definition", s["mode"])
	}
}

func TestSeq19P34LocalePackSplit(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p34","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "locale_pack_split")
	if s["version"] != "s19-p34.v1" {
		t.Fatalf("version=%v, want s19-p34.v1", s["version"])
	}
	packs, ok := s["locale_packs"].([]any)
	if !ok || len(packs) != 4 {
		t.Fatalf("locale_packs=%v, want 4 items", s["locale_packs"])
	}
	if s["unsupported_label_policy"] != "fail_open_carry_forward" {
		t.Fatalf("unsupported_label_policy=%v, want fail_open_carry_forward", s["unsupported_label_policy"])
	}
	if s["mode"] != "locale_pack_split_definition" {
		t.Fatalf("mode=%v, want locale_pack_split_definition", s["mode"])
	}
}

func TestSeq19P35MultilingualDeicticParity(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p35","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "multilingual_deictic_parity")
	if s["version"] != "s19-p35.v1" {
		t.Fatalf("version=%v, want s19-p35.v1", s["version"])
	}
	phrases, ok := s["supported_phrases"].([]any)
	if !ok || len(phrases) != 5 {
		t.Fatalf("supported_phrases=%v, want 5 items", s["supported_phrases"])
	}
	if s["normalization_target"] != "canonical_offset_unit_precision" {
		t.Fatalf("normalization_target=%v, want canonical_offset_unit_precision", s["normalization_target"])
	}
	if s["mode"] != "multilingual_deictic_parity_definition" {
		t.Fatalf("mode=%v, want multilingual_deictic_parity_definition", s["mode"])
	}
}

func TestSeq19P36ActiveLocalesGating(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p36","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "active_locales_gating")
	if s["version"] != "s19-p36.v1" {
		t.Fatalf("version=%v, want s19-p36.v1", s["version"])
	}
	gatingKeys, ok := s["gating_keys"].([]any)
	if !ok || len(gatingKeys) != 2 {
		t.Fatalf("gating_keys=%v, want 2 items", s["gating_keys"])
	}
	if s["outside_locale_policy"] != "unresolved_carry_forward" {
		t.Fatalf("outside_locale_policy=%v, want unresolved_carry_forward", s["outside_locale_policy"])
	}
	if s["no_fake_exact_time"] != true {
		t.Fatalf("no_fake_exact_time=%v, want true", s["no_fake_exact_time"])
	}
	if s["mode"] != "active_locales_gating_definition" {
		t.Fatalf("mode=%v, want active_locales_gating_definition", s["mode"])
	}
}

func TestSeq19P37SnakeCaseCamelCaseInspect(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p37","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "snake_case_camel_case_inspect")
	if s["version"] != "s19-p37.v1" {
		t.Fatalf("version=%v, want s19-p37.v1", s["version"])
	}
	keys, ok := s["supported_input_keys"].([]any)
	if !ok || len(keys) != 20 {
		t.Fatalf("supported_input_keys count=%v, want 20", len(keys))
	}
	if s["inspect_both"] != true {
		t.Fatalf("inspect_both=%v, want true", s["inspect_both"])
	}
	if s["no_write_path_cutover"] != true {
		t.Fatalf("no_write_path_cutover=%v, want true", s["no_write_path_cutover"])
	}
	if s["mode"] != "snake_case_camel_case_inspect_definition" {
		t.Fatalf("mode=%v, want snake_case_camel_case_inspect_definition", s["mode"])
	}
}

func TestSeq19P38ValidFromToTurnRange(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p38","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "valid_from_to_turn_range")
	if s["version"] != "s19-p38.v1" {
		t.Fatalf("version=%v, want s19-p38.v1", s["version"])
	}
	fields, ok := s["fields"].([]any)
	if !ok || len(fields) != 4 {
		t.Fatalf("fields=%v, want 4 items", s["fields"])
	}
	if s["mode"] != "valid_from_to_turn_range_definition" {
		t.Fatalf("mode=%v, want valid_from_to_turn_range_definition", s["mode"])
	}
}

func TestSeq19P39MissingAnchorDegradation(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p39","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "missing_anchor_degradation")
	if s["version"] != "s19-p39.v1" {
		t.Fatalf("version=%v, want s19-p39.v1", s["version"])
	}
	if s["missing_anchor_degrades_to"] != "explicit_ambiguity" {
		t.Fatalf("missing_anchor_degrades_to=%v, want explicit_ambiguity", s["missing_anchor_degrades_to"])
	}
	if s["anchor_resolution_status"] != "carry_forward" {
		t.Fatalf("anchor_resolution_status=%v, want carry_forward", s["anchor_resolution_status"])
	}
	if s["false_exact_precision_blocked"] != true {
		t.Fatalf("false_exact_precision_blocked=%v, want true", s["false_exact_precision_blocked"])
	}
	if s["mode"] != "missing_anchor_degradation_definition" {
		t.Fatalf("mode=%v, want missing_anchor_degradation_definition", s["mode"])
	}
}

func TestSeq19P40TemporalRelationLedgerComplete(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p40","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "temporal_relation_ledger_complete")
	if s["version"] != "s19-p40.v1" {
		t.Fatalf("version=%v, want s19-p40.v1", s["version"])
	}
	if s["schema_complete"] != true {
		t.Fatalf("schema_complete=%v, want true", s["schema_complete"])
	}
	if s["normalizer_complete"] != true {
		t.Fatalf("normalizer_complete=%v, want true", s["normalizer_complete"])
	}
	if s["locale_pack_complete"] != true {
		t.Fatalf("locale_pack_complete=%v, want true", s["locale_pack_complete"])
	}
	if s["inspect_complete"] != true {
		t.Fatalf("inspect_complete=%v, want true", s["inspect_complete"])
	}
	if s["mode"] != "temporal_relation_ledger_complete_definition" {
		t.Fatalf("mode=%v, want temporal_relation_ledger_complete_definition", s["mode"])
	}
}

// P41 and P42 are covered by the combined ledger complete surface (P40).
// They represent the final closure of the schema + normalizer + locale pack + inspect completeness.
func TestSeq19P41P42CombinedLedgerClosure(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p41p42","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	s := seq165Map(t, resp, "temporal_relation_ledger_complete")
	if s["schema_complete"] != true {
		t.Fatalf("schema_complete=%v, want true (P41)", s["schema_complete"])
	}
	if s["normalizer_complete"] != true {
		t.Fatalf("normalizer_complete=%v, want true (P41)", s["normalizer_complete"])
	}

	if s["locale_pack_complete"] != true {
		t.Fatalf("locale_pack_complete=%v, want true (P42)", s["locale_pack_complete"])
	}
	if s["inspect_complete"] != true {
		t.Fatalf("inspect_complete=%v, want true (P42)", s["inspect_complete"])
	}
}

func TestSeq19P50ElapsedPolicyOwner(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p50","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "sc19_elapsed_policy_owner")
	if s["version"] != "s19-p50.v1" {
		t.Fatalf("version=%v, want s19-p50.v1", s["version"])
	}
	cats, ok := s["trigger_categories"].([]any)
	if !ok || len(cats) != 6 {
		t.Fatalf("trigger_categories count=%v, want 6", len(cats))
	}
	aliases, ok := s["trigger_aliases"].(map[string]any)
	if !ok || len(aliases) < 4 {
		t.Fatalf("trigger_aliases count=%v, want >=4", len(aliases))
	}
	codes, ok := s["structured_codes"].([]any)
	if !ok || len(codes) != 6 {
		t.Fatalf("structured_codes count=%v, want 6", len(codes))
	}
	if s["mode"] != "sc19_elapsed_policy_owner_definition" {
		t.Fatalf("mode=%v, want sc19_elapsed_policy_owner_definition", s["mode"])
	}
}

func TestSeq19P51ElapsedTimeDecisionExtended(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p51","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	ts := seq165Map(t, resp, "temporal_state")
	elapsed := seq165Map(t, ts, "elapsed_time_decision")
	if elapsed["version"] != "s19-et.v2" {
		t.Fatalf("version=%v, want s19-et.v2", elapsed["version"])
	}
	if _, ok := elapsed["trigger_category"]; !ok {
		t.Fatalf("trigger_category missing")
	}
	if _, ok := elapsed["trigger_category_source"]; !ok {
		t.Fatalf("trigger_category_source missing")
	}
	if _, ok := elapsed["scene_progression_evidence"]; !ok {
		t.Fatalf("scene_progression_evidence missing")
	}
}

func TestSeq19P52ClockWriteDirectiveExtended(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p52","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	ts := seq165Map(t, resp, "temporal_state")
	cwd := seq165Map(t, ts, "clock_write_directive")
	if cwd["version"] != "s19-cwd.v2" {
		t.Fatalf("version=%v, want s19-cwd.v2", cwd["version"])
	}
	if _, ok := cwd["write_discipline"]; !ok {
		t.Fatalf("write_discipline missing")
	}
	if _, ok := cwd["write_allowed"]; !ok {
		t.Fatalf("write_allowed missing")
	}
	if _, ok := cwd["normalized_status"]; !ok {
		t.Fatalf("normalized_status missing")
	}
}

func TestSeq19P53TemporalSupportPacket(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p53","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "temporal_support_packet")
	if s["version"] != "s19-p53.v1" {
		t.Fatalf("version=%v, want s19-p53.v1", s["version"])
	}
	if _, ok := s["temporal_packet"]; !ok {
		t.Fatalf("temporal_packet missing")
	}
	packetText, ok := s["temporal_packet_text"].(string)
	if !ok || packetText == "" {
		t.Fatalf("temporal_packet_text empty or missing")
	}
	ip := seq165Map(t, resp, "injection_pack")
	ipPacketText, ok := ip["temporal_packet_text"].(string)
	if !ok || ipPacketText == "" {
		t.Fatalf("injection_pack.temporal_packet_text empty or missing")
	}
	if ipPacketText != packetText {
		t.Fatalf("injection_pack.temporal_packet_text = %q, want temporal support packet text %q", ipPacketText, packetText)
	}
	if _, ok := ip["temporal_packet"].(map[string]any); !ok {
		t.Fatalf("injection_pack.temporal_packet missing")
	}
}

func TestSeq19P54TemporalWriteDiscipline(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p54","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "temporal_write_discipline")
	if s["version"] != "s19-p54.v1" {
		t.Fatalf("version=%v, want s19-p54.v1", s["version"])
	}
	cases, ok := s["discipline_cases"].([]any)
	if !ok || len(cases) != 4 {
		t.Fatalf("discipline_cases count=%v, want 4", len(cases))
	}
}

func TestSeq19P55ElapsedPolicyCompactness(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p55","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "elapsed_policy_compactness")
	if s["version"] != "s19-p55.v1" {
		t.Fatalf("version=%v, want s19-p55.v1", s["version"])
	}
	if s["no_scattered_literals"] != true {
		t.Fatalf("no_scattered_literals=%v, want true", s["no_scattered_literals"])
	}
	fields, ok := s["localized_fields"].([]any)
	if !ok || len(fields) < 4 {
		t.Fatalf("localized_fields count=%v, want >=4", len(fields))
	}
}

func TestSeq19P56TemporalGuardBundle(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p56","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "temporal_guard_bundle")
	if s["version"] != "s19-p56.v1" {
		t.Fatalf("version=%v, want s19-p56.v1", s["version"])
	}
	cases, ok := s["guard_cases"].([]any)
	if !ok || len(cases) != 5 {
		t.Fatalf("guard_cases count=%v, want 5", len(cases))
	}
}

func TestSeq19P57RegressionBundle(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p57","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "step18_plus_19_regression_bundle_57")
	if s["version"] != "s19-p57.v1" {
		t.Fatalf("version=%v, want s19-p57.v1", s["version"])
	}
	if s["regression_status"] != "green" {
		t.Fatalf("regression_status=%v, want green", s["regression_status"])
	}
	if s["elapsed_time_slice"] != "landed" {
		t.Fatalf("elapsed_time_slice=%v, want landed", s["elapsed_time_slice"])
	}
}

func TestSeq19P66WeekUnitSupport(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p66","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "week_unit_support")
	if s["version"] != "s19-p66.v1" {
		t.Fatalf("version=%v, want s19-p66.v1", s["version"])
	}
	units, ok := s["offset_units"].([]any)
	if !ok || len(units) != 5 {
		t.Fatalf("offset_units count=%v, want 5", len(units))
	}
	locales, ok := s["locale_packs_with_week"].([]any)
	if !ok || len(locales) != 4 {
		t.Fatalf("locale_packs_with_week count=%v, want 4", len(locales))
	}
	if s["bounded_week_relation"] != true {
		t.Fatalf("bounded_week_relation=%v, want true", s["bounded_week_relation"])
	}
}

func TestSeq19P67TemporalReplayCases(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq19-p67","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "temporal_replay_cases")
	if s["version"] != "s19-p67.v1" {
		t.Fatalf("version=%v, want s19-p67.v1", s["version"])
	}
	cases, ok := s["replay_cases"].([]any)
	if !ok || len(cases) != 3 {
		t.Fatalf("replay_cases count=%v, want 3", len(cases))
	}
	byPhrase := map[string]map[string]any{}
	for _, item := range cases {
		entry, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("replay case is not an object: %T", item)
		}
		phrase, _ := entry["phrase"].(string)
		byPhrase[phrase] = entry
	}
	if byPhrase["today"]["offset_unit"] != "day" || byPhrase["today"]["precision"] != "exact" {
		t.Fatalf("today replay = %+v, want exact day", byPhrase["today"])
	}
	if byPhrase["last_week"]["offset_unit"] != "week" || byPhrase["last_week"]["precision"] != "bounded_range" || byPhrase["last_week"]["write_lane"] != "carry_forward_only" {
		t.Fatalf("last_week replay = %+v, want bounded week carry-forward", byPhrase["last_week"])
	}
	if byPhrase["last_month"]["offset_unit"] != "month" || byPhrase["last_month"]["precision"] != "bounded_range" || byPhrase["last_month"]["write_lane"] != "carry_forward_only" {
		t.Fatalf("last_month replay = %+v, want bounded month carry-forward", byPhrase["last_month"])
	}
}
