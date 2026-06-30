package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// SEQ-19 reset administration tests (P9 ~ P11)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// SEQ-19 temporal state tests (P15 ~ P22)
// ---------------------------------------------------------------------------

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
	// Verify current_story_clock exists
	if _, ok := s["current_story_clock"]; !ok {
		t.Fatal("missing current_story_clock")
	}
	// Verify temporal_relation_ledger exists
	if _, ok := s["temporal_relation_ledger"]; !ok {
		t.Fatal("missing temporal_relation_ledger")
	}
	// Verify elapsed_time_decision exists
	if _, ok := s["elapsed_time_decision"]; !ok {
		t.Fatal("missing elapsed_time_decision")
	}
	// Verify clock_write_directive exists
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
	// Verify all four sc19a.v1 fields are present
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

// ---------------------------------------------------------------------------
// SEQ-19 temporal relation ledger schema tests (P30 ~ P42)
// ---------------------------------------------------------------------------

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
	// Verify P41 (schema + normalizer completeness)
	s := seq165Map(t, resp, "temporal_relation_ledger_complete")
	if s["schema_complete"] != true {
		t.Fatalf("schema_complete=%v, want true (P41)", s["schema_complete"])
	}
	if s["normalizer_complete"] != true {
		t.Fatalf("normalizer_complete=%v, want true (P41)", s["normalizer_complete"])
	}
	// Verify P42 (locale pack + inspect completeness)
	if s["locale_pack_complete"] != true {
		t.Fatalf("locale_pack_complete=%v, want true (P42)", s["locale_pack_complete"])
	}
	if s["inspect_complete"] != true {
		t.Fatalf("inspect_complete=%v, want true (P42)", s["inspect_complete"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-19 elapsed-time normalization tests (P50 ~ P57)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// SEQ-19 locale pack + replay tests (P66 ~ P69)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// SEQ-19 mixed-lane VX replay tests (P78 ~ P81)
// ---------------------------------------------------------------------------

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
	// commit_current_scene_anchor: today + yesterday
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
	// commit_explicit_advance: tomorrow + yesterday
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

// ---------------------------------------------------------------------------
// SEQ-19 degrade replay / VX coverage tests (P90 ~ P93)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// SEQ-19 temporal packet truth-boundary / precedence tests (P102 ~ P105)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// SEQ-19 response-time validator helper cluster / trace-only tests (P114 ~ P117)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// SEQ-19 classification / write-discipline tests (P125 ~ P128)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// SEQ-19 locale-aware extraction / multilingual parity tests (P137 ~ P139)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// SEQ-19 activeLocales fail-open gating test (P140)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// SEQ-19 finish-line criteria tests (P288 ~ P292)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// SEQ-19 sub-step 19-1 schema definition tests (P296 ~ P299)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// SEQ-19 sub-step 19-2 schema definition tests (P303 ~ P307)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// SEQ-19 sub-step 19-3 schema definition tests (P311 ~ P314)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// SEQ-19 sub-step 19-4 VX replay tests (P318 ~ P322)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// SEQ-19 sub-step 19-4f/19-4g replay tests (P323 ~ P324)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// SEQ-19 Beta 1.0 release gate tests (P328 ~ P332)
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// SEQ-19 Beta 1.0 release gate + decision tests (P333, P337 ~ P344)
// ---------------------------------------------------------------------------

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
