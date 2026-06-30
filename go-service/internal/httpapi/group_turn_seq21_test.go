package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// SEQ-21 Preparatory reset/admin tests (P9 ~ P11)
// ---------------------------------------------------------------------------

func TestSeq21P9ResetAdminNote(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p9","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_reset_admin_note")
	if s["version"] != "s21-p9.v1" {
		t.Fatalf("version=%v, want s21-p9.v1", s["version"])
	}
	if s["sub_step"] != "preparatory" {
		t.Fatalf("sub_step=%v, want preparatory", s["sub_step"])
	}
	if s["action_taken"] != "reset_cleared" {
		t.Fatalf("action_taken=%v, want reset_cleared", s["action_taken"])
	}
	if s["mode"] != "seq21_reset_admin_note_definition" {
		t.Fatalf("mode=%v, want seq21_reset_admin_note_definition", s["mode"])
	}
}

func TestSeq21P10HistoricalContentPreserved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p10","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_historical_content_preserved")
	if s["version"] != "s21-p10.v1" {
		t.Fatalf("version=%v, want s21-p10.v1", s["version"])
	}
	if s["sub_step"] != "preparatory" {
		t.Fatalf("sub_step=%v, want preparatory", s["sub_step"])
	}
	if s["action_taken"] != "preserved" {
		t.Fatalf("action_taken=%v, want preserved", s["action_taken"])
	}
	if s["mode"] != "seq21_historical_content_preserved_definition" {
		t.Fatalf("mode=%v, want seq21_historical_content_preserved_definition", s["mode"])
	}
}

func TestSeq21P11ResetNoteOnly(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p11","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_reset_note_only")
	if s["version"] != "s21-p11.v1" {
		t.Fatalf("version=%v, want s21-p11.v1", s["version"])
	}
	if s["sub_step"] != "preparatory" {
		t.Fatalf("sub_step=%v, want preparatory", s["sub_step"])
	}
	if s["action_taken"] != "document_only" {
		t.Fatalf("action_taken=%v, want document_only", s["action_taken"])
	}
	if s["mode"] != "seq21_reset_note_only_definition" {
		t.Fatalf("mode=%v, want seq21_reset_note_only_definition", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-21 Six-criteria summary tests (P181 ~ P186)
// ---------------------------------------------------------------------------

func TestSeq21P181RerankClassSummary(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p181","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_rerank_class_summary")
	if s["version"] != "s21-p181.v1" {
		t.Fatalf("version=%v, want s21-p181.v1", s["version"])
	}
	if s["criterion"] != "rerank_restraint" {
		t.Fatalf("criterion=%v, want rerank_restraint", s["criterion"])
	}
	if summary, _ := s["summary"].(string); !strings.Contains(summary, "not default") {
		t.Fatalf("summary=%q, want rerank not-default wording", summary)
	}
	if s["mode"] != "seq21_rerank_class_summary_definition" {
		t.Fatalf("mode=%v, want seq21_rerank_class_summary_definition", s["mode"])
	}
}

func TestSeq21P182BudgetConfigSummary(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p182","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_budget_config_summary")
	if s["version"] != "s21-p182.v1" {
		t.Fatalf("version=%v, want s21-p182.v1", s["version"])
	}
	if s["criterion"] != "budget_durability" {
		t.Fatalf("criterion=%v, want budget_durability", s["criterion"])
	}
	if s["mode"] != "seq21_budget_config_summary_definition" {
		t.Fatalf("mode=%v, want seq21_budget_config_summary_definition", s["mode"])
	}
}

func TestSeq21P183FailureClassSplitSummary(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p183","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_failure_class_split_summary")
	if s["version"] != "s21-p183.v1" {
		t.Fatalf("version=%v, want s21-p183.v1", s["version"])
	}
	if s["criterion"] != "failure_class_separation" {
		t.Fatalf("criterion=%v, want failure_class_separation", s["criterion"])
	}
	if s["mode"] != "seq21_failure_class_split_summary_definition" {
		t.Fatalf("mode=%v, want seq21_failure_class_split_summary_definition", s["mode"])
	}
}

func TestSeq21P184HeldOutHygieneSummary(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p184","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_held_out_hygiene_summary")
	if s["version"] != "s21-p184.v1" {
		t.Fatalf("version=%v, want s21-p184.v1", s["version"])
	}
	if s["criterion"] != "dev_held_out_split" {
		t.Fatalf("criterion=%v, want dev_held_out_split", s["criterion"])
	}
	if s["mode"] != "seq21_held_out_hygiene_summary_definition" {
		t.Fatalf("mode=%v, want seq21_held_out_hygiene_summary_definition", s["mode"])
	}
}

func TestSeq21P185TruthBoundaryPreserveSummary(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p185","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_truth_boundary_preserve_summary")
	if s["version"] != "s21-p185.v1" {
		t.Fatalf("version=%v, want s21-p185.v1", s["version"])
	}
	if s["criterion"] != "truth_boundary_intact" {
		t.Fatalf("criterion=%v, want truth_boundary_intact", s["criterion"])
	}
	if summary, _ := s["summary"].(string); !strings.Contains(summary, "MariaDB canonical truth precedence") {
		t.Fatalf("summary=%q, want MariaDB canonical truth precedence wording", summary)
	}
	if s["mode"] != "seq21_truth_boundary_preserve_summary_definition" {
		t.Fatalf("mode=%v, want seq21_truth_boundary_preserve_summary_definition", s["mode"])
	}
}

func TestSeq21P186DensityDisciplineSummary(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p186","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_density_discipline_summary")
	if s["version"] != "s21-p186.v1" {
		t.Fatalf("version=%v, want s21-p186.v1", s["version"])
	}
	if s["criterion"] != "density_tier_budget" {
		t.Fatalf("criterion=%v, want density_tier_budget", s["criterion"])
	}
	if summary, _ := s["summary"].(string); !strings.Contains(summary, "not authority stratification") {
		t.Fatalf("summary=%q, want density not-authority wording", summary)
	}
	if s["mode"] != "seq21_density_discipline_summary_definition" {
		t.Fatalf("mode=%v, want seq21_density_discipline_summary_definition", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-21 21-1 selective rerank gate tests (P190 ~ P193)
// ---------------------------------------------------------------------------

func TestSeq21P190RerankTriggerClass(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p190","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_rerank_trigger_class")
	if s["version"] != "s21-p190.v1" {
		t.Fatalf("version=%v, want s21-p190.v1", s["version"])
	}
	classes := seq165Slice(t, s, "trigger_classes")
	for _, expected := range []string{"temporal_ambiguity", "dense_tie", "canon_conflict", "vocabulary_gap"} {
		if !sliceContains(classes, expected) {
			t.Fatalf("trigger_classes missing %q: %v", expected, classes)
		}
	}
	denied := seq165Slice(t, s, "denied_classes")
	if !sliceContains(denied, "scene") {
		t.Fatalf("denied_classes missing scene: %v", denied)
	}
	allowed := seq165Slice(t, s, "allowed_query_classes")
	for _, expected := range []string{"temporal", "callback", "resume", "canon"} {
		if !sliceContains(allowed, expected) {
			t.Fatalf("allowed_query_classes missing %q: %v", expected, allowed)
		}
	}
	if s["mode"] != "seq21_rerank_trigger_class_definition" {
		t.Fatalf("mode=%v, want seq21_rerank_trigger_class_definition", s["mode"])
	}
}

func TestSeq21P191RerankSupportOnlySchema(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p191","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_rerank_support_only_schema")
	if s["version"] != "s21-p191.v1" {
		t.Fatalf("version=%v, want s21-p191.v1", s["version"])
	}
	if s["truth_write_mode"] != "forbidden" {
		t.Fatalf("truth_write_mode=%v, want forbidden", s["truth_write_mode"])
	}
	inputs := seq165Slice(t, s, "input_surfaces")
	for _, expected := range []string{"query_class_contract", "ann_candidate_snapshot"} {
		if !sliceContains(inputs, expected) {
			t.Fatalf("input_surfaces missing %q: %v", expected, inputs)
		}
	}
	outputs := seq165Slice(t, s, "output_fields")
	for _, expected := range []string{"support_lane_status", "support_lane_summary"} {
		if !sliceContains(outputs, expected) {
			t.Fatalf("output_fields missing %q: %v", expected, outputs)
		}
	}
	if s["canonical_truth_authority"] != "mariadb_canonical_precedence" {
		t.Fatalf("canonical_truth_authority=%v, want mariadb_canonical_precedence", s["canonical_truth_authority"])
	}
	if s["mode"] != "seq21_rerank_support_only_schema_definition" {
		t.Fatalf("mode=%v, want seq21_rerank_support_only_schema_definition", s["mode"])
	}
}

func TestSeq21P192RerankOffFallback(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p192","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_rerank_off_fallback")
	if s["version"] != "s21-p192.v1" {
		t.Fatalf("version=%v, want s21-p192.v1", s["version"])
	}
	if s["default_takeover"] != false {
		t.Fatalf("default_takeover=%v, want false", s["default_takeover"])
	}
	if s["fallback_boundary"] != "hybrid_recall_fail_open" {
		t.Fatalf("fallback_boundary=%v, want hybrid_recall_fail_open", s["fallback_boundary"])
	}
	offStates := seq165Slice(t, s, "off_states")
	for _, expected := range []string{"off", "inactive", "gated_off"} {
		if !sliceContains(offStates, expected) {
			t.Fatalf("off_states missing %q: %v", expected, offStates)
		}
	}
	if s["inactive_reason"] != "no_bounded_trigger" || s["gated_off_reason"] != "ann_snapshot_not_ready" {
		t.Fatalf("unexpected fallback reasons: inactive=%v gated=%v", s["inactive_reason"], s["gated_off_reason"])
	}
	if s["mode"] != "seq21_rerank_off_fallback_definition" {
		t.Fatalf("mode=%v, want seq21_rerank_off_fallback_definition", s["mode"])
	}
}

func TestSeq21P193RerankNearMissTrigger(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p193","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_rerank_near_miss_trigger")
	if s["version"] != "s21-p193.v1" {
		t.Fatalf("version=%v, want s21-p193.v1", s["version"])
	}
	triggers := seq165Slice(t, s, "trigger_surfaces")
	for _, expected := range []string{"top_k_near_miss", "sparse_callback", "canon_support_tie"} {
		if !sliceContains(triggers, expected) {
			t.Fatalf("trigger_surfaces missing %q: %v", expected, triggers)
		}
	}
	if s["rescue_only"] != true {
		t.Fatalf("rescue_only=%v, want true", s["rescue_only"])
	}
	if s["top_k_near_miss_source"] != "ann_candidate_snapshot.filtered_out_total" {
		t.Fatalf("top_k_near_miss_source=%v, want ann_candidate_snapshot.filtered_out_total", s["top_k_near_miss_source"])
	}
	if s["sparse_callback_source"] != "recall_cue_rescue_rule.callback_rescue_enabled" {
		t.Fatalf("sparse_callback_source=%v, want recall_cue_rescue_rule.callback_rescue_enabled", s["sparse_callback_source"])
	}
	if s["canon_support_tie_source"] != "canonical_pending_stale_current_conflict_note.source_precedence" {
		t.Fatalf("canon_support_tie_source=%v, want canonical_pending_stale_current_conflict_note.source_precedence", s["canon_support_tie_source"])
	}
	if s["mode"] != "seq21_rerank_near_miss_trigger_definition" {
		t.Fatalf("mode=%v, want seq21_rerank_near_miss_trigger_definition", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-21 21-2 retrieval economics tests (P197 ~ P202)
// ---------------------------------------------------------------------------

func TestSeq21P197QueryClassCandidateCap(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p197","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_query_class_candidate_cap")
	if s["version"] != "s21-p197.v1" {
		t.Fatalf("version=%v, want s21-p197.v1", s["version"])
	}
	if s["support_only"] != true {
		t.Fatalf("support_only=%v, want true", s["support_only"])
	}
	if s["budget_owner"] != "query_class_budget_policy" || s["packet_owner"] != "packet_budget_policy" {
		t.Fatalf("unexpected budget/packet owners: budget=%v packet=%v", s["budget_owner"], s["packet_owner"])
	}
	caps := seq165Map(t, s, "query_class_caps")
	scene := seq165Map(t, caps, "scene")
	temporal := seq165Map(t, caps, "temporal")
	if scene["cap"] != float64(0) || scene["reason"] != "rerank_denied" {
		t.Fatalf("scene cap=%v reason=%v, want cap 0 rerank_denied", scene["cap"], scene["reason"])
	}
	if temporal["cap"] != float64(12) || temporal["reason"] != "evidence_first_overlay" {
		t.Fatalf("temporal cap=%v reason=%v, want cap 12 evidence_first_overlay", temporal["cap"], temporal["reason"])
	}
	if s["mode"] != "seq21_query_class_candidate_cap_definition" {
		t.Fatalf("mode=%v, want seq21_query_class_candidate_cap_definition", s["mode"])
	}
}

func TestSeq21P198LatencyBudgetDegrade(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p198","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_latency_budget_degrade")
	if s["version"] != "s21-p198.v1" {
		t.Fatalf("version=%v, want s21-p198.v1", s["version"])
	}
	if s["latency_ceiling_source"] != "vx18c_upstream_gate" {
		t.Fatalf("latency_ceiling_source=%v, want vx18c_upstream_gate", s["latency_ceiling_source"])
	}
	if s["support_only"] != true {
		t.Fatalf("support_only=%v, want true", s["support_only"])
	}
	if s["timeout_budget_ms_source"] != "ann_candidate_snapshot.benchmark.timeout_budget_ms" {
		t.Fatalf("timeout_budget_ms_source=%v, want ann_candidate_snapshot.benchmark.timeout_budget_ms", s["timeout_budget_ms_source"])
	}
	degradePath := seq165Slice(t, s, "degrade_path")
	for _, expected := range []string{"latency_or_budget_hold_then_fail_open", "hybrid_recall_fail_open"} {
		if !sliceContains(degradePath, expected) {
			t.Fatalf("degrade_path missing %q: %v", expected, degradePath)
		}
	}
	if s["mode"] != "seq21_latency_budget_degrade_definition" {
		t.Fatalf("mode=%v, want seq21_latency_budget_degrade_definition", s["mode"])
	}
}

func TestSeq21P199RetrievalCacheReuse(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p199","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_retrieval_cache_reuse")
	if s["version"] != "s21-p199.v1" {
		t.Fatalf("version=%v, want s21-p199.v1", s["version"])
	}
	if s["runtime_mode"] != "shadow_off" {
		t.Fatalf("runtime_mode=%v, want shadow_off", s["runtime_mode"])
	}
	if s["support_only"] != true {
		t.Fatalf("support_only=%v, want true", s["support_only"])
	}
	if s["reuse_surface"] != "retrieval_index_registry.snapshot" || s["refresh_event"] != "prepare_turn_refresh" {
		t.Fatalf("unexpected reuse/refresh: reuse=%v refresh=%v", s["reuse_surface"], s["refresh_event"])
	}
	paths := seq165Slice(t, s, "invalidation_paths")
	for _, expected := range []string{"mark_dirty", "rollback_discard"} {
		if !sliceContains(paths, expected) {
			t.Fatalf("invalidation_paths missing %q: %v", expected, paths)
		}
	}
	if s["mode"] != "seq21_retrieval_cache_reuse_definition" {
		t.Fatalf("mode=%v, want seq21_retrieval_cache_reuse_definition", s["mode"])
	}
}

func TestSeq21P200FailureClassAdaptiveCap(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p200","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_failure_class_adaptive_cap")
	if s["version"] != "s21-p200.v1" {
		t.Fatalf("version=%v, want s21-p200.v1", s["version"])
	}
	if s["non_shrinking_baseline"] != true {
		t.Fatalf("non_shrinking_baseline=%v, want true", s["non_shrinking_baseline"])
	}
	if s["support_only"] != true {
		t.Fatalf("support_only=%v, want true", s["support_only"])
	}
	if s["cap_separation"] != "raw_profile_cap_vs_active_top_k" {
		t.Fatalf("cap_separation=%v, want raw_profile_cap_vs_active_top_k", s["cap_separation"])
	}
	profiles := seq165Map(t, s, "failure_class_profiles")
	for _, expected := range []string{"temporal_miss", "callback_miss", "canon_conflict", "alias_confusion"} {
		if _, ok := profiles[expected]; !ok {
			t.Fatalf("failure_class_profiles missing %q: %v", expected, profiles)
		}
	}
	if s["mode"] != "seq21_failure_class_adaptive_cap_definition" {
		t.Fatalf("mode=%v, want seq21_failure_class_adaptive_cap_definition", s["mode"])
	}
}

func TestSeq21P201DualDensityDeliveryBudget(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p201","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_dual_density_delivery_budget")
	if s["version"] != "s21-p201.v1" {
		t.Fatalf("version=%v, want s21-p201.v1", s["version"])
	}
	if s["support_only"] != true {
		t.Fatalf("support_only=%v, want true", s["support_only"])
	}
	heavy := seq165Map(t, s, "heavy_packet")
	light := seq165Map(t, s, "light_tag")
	if heavy["density"] != "full_support_surface" || heavy["char_budget"] != float64(2048) {
		t.Fatalf("heavy_packet density=%v char_budget=%v, want full_support_surface 2048", heavy["density"], heavy["char_budget"])
	}
	if light["density"] != "metadata_only" || light["char_budget"] != float64(0) {
		t.Fatalf("light_tag density=%v char_budget=%v, want metadata_only 0", light["density"], light["char_budget"])
	}
	if s["mode"] != "seq21_dual_density_delivery_budget_definition" {
		t.Fatalf("mode=%v, want seq21_dual_density_delivery_budget_definition", s["mode"])
	}
}

func TestSeq21P202HeavyPromotionRule(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p202","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_heavy_promotion_rule")
	if s["version"] != "s21-p202.v1" {
		t.Fatalf("version=%v, want s21-p202.v1", s["version"])
	}
	if s["auto_promote_without_failure_class"] != false {
		t.Fatalf("auto_promote_without_failure_class=%v, want false", s["auto_promote_without_failure_class"])
	}
	if s["support_only"] != true {
		t.Fatalf("support_only=%v, want true", s["support_only"])
	}
	prereqs := seq165Slice(t, s, "promotion_prerequisites")
	for _, expected := range []string{"active_qualifying_failure_class", "strong_temporal_relation_signal", "pending_current_or_canonical_anchor"} {
		if !sliceContains(prereqs, expected) {
			t.Fatalf("promotion_prerequisites missing %q: %v", expected, prereqs)
		}
	}
	blockers := seq165Slice(t, s, "weak_linkage_blockers")
	for _, expected := range []string{"thin_tag_only", "no_active_failure_class", "missing_temporal_or_relation_signal"} {
		if !sliceContains(blockers, expected) {
			t.Fatalf("weak_linkage_blockers missing %q: %v", expected, blockers)
		}
	}
	if s["mode"] != "seq21_heavy_promotion_rule_definition" {
		t.Fatalf("mode=%v, want seq21_heavy_promotion_rule_definition", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-21 21-3 failure-class tuning loop tests (P206 ~ P209)
// ---------------------------------------------------------------------------

func TestSeq21P206FailureTaxonomy(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p206","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_failure_taxonomy")
	if s["version"] != "s21-p206.v1" {
		t.Fatalf("version=%v, want s21-p206.v1", s["version"])
	}
	classes := seq165Slice(t, s, "failure_classes")
	for _, expected := range []string{"temporal_miss", "callback_miss", "canon_conflict", "alias_confusion"} {
		if !sliceContains(classes, expected) {
			t.Fatalf("failure_classes missing %q: %v", expected, classes)
		}
	}
	if s["support_only"] != true {
		t.Fatalf("support_only=%v, want true", s["support_only"])
	}
	if s["mode"] != "seq21_failure_taxonomy_definition" {
		t.Fatalf("mode=%v, want seq21_failure_taxonomy_definition", s["mode"])
	}
}

func TestSeq21P207DevSplitTuningLoop(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p207","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_dev_split_tuning_loop")
	if s["version"] != "s21-p207.v1" {
		t.Fatalf("version=%v, want s21-p207.v1", s["version"])
	}
	if s["dev_split"] != true {
		t.Fatalf("dev_split=%v, want true", s["dev_split"])
	}
	if s["default_promotion_stays_pending"] != true {
		t.Fatalf("default_promotion_stays_pending=%v, want true", s["default_promotion_stays_pending"])
	}
	if s["support_only"] != true {
		t.Fatalf("support_only=%v, want true", s["support_only"])
	}
	if s["mode"] != "seq21_dev_split_tuning_loop_definition" {
		t.Fatalf("mode=%v, want seq21_dev_split_tuning_loop_definition", s["mode"])
	}
}

func TestSeq21P208HeldOutConfirmationGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p208","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_held_out_confirmation_gate")
	if s["version"] != "s21-p208.v1" {
		t.Fatalf("version=%v, want s21-p208.v1", s["version"])
	}
	if s["held_out_separate"] != true {
		t.Fatalf("held_out_separate=%v, want true", s["held_out_separate"])
	}
	if s["no_single_replay_promotion"] != true {
		t.Fatalf("no_single_replay_promotion=%v, want true", s["no_single_replay_promotion"])
	}
	if s["support_only"] != true {
		t.Fatalf("support_only=%v, want true", s["support_only"])
	}
	if s["mode"] != "seq21_held_out_confirmation_gate_definition" {
		t.Fatalf("mode=%v, want seq21_held_out_confirmation_gate_definition", s["mode"])
	}
}

func TestSeq21P209ResidualLongTailLoop(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p209","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_residual_long_tail_loop")
	if s["version"] != "s21-p209.v1" {
		t.Fatalf("version=%v, want s21-p209.v1", s["version"])
	}
	if s["residual_only"] != true {
		t.Fatalf("residual_only=%v, want true", s["residual_only"])
	}
	if s["support_only"] != true {
		t.Fatalf("support_only=%v, want true", s["support_only"])
	}
	if s["mode"] != "seq21_residual_long_tail_loop_definition" {
		t.Fatalf("mode=%v, want seq21_residual_long_tail_loop_definition", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-21 21-4 validation/adoption gate tests (P213 ~ P219)
// ---------------------------------------------------------------------------

func TestSeq21P213CostVsGainReplay(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p213","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_cost_vs_gain_replay")
	if s["version"] != "s21-p213.v1" {
		t.Fatalf("version=%v, want s21-p213.v1", s["version"])
	}
	if s["cost_side"] != "replay_surface_only" {
		t.Fatalf("cost_side=%v, want replay_surface_only", s["cost_side"])
	}
	if s["support_only"] != true {
		t.Fatalf("support_only=%v, want true", s["support_only"])
	}
	if s["mode"] != "seq21_cost_vs_gain_replay_definition" {
		t.Fatalf("mode=%v, want seq21_cost_vs_gain_replay_definition", s["mode"])
	}
}

func TestSeq21P214LatencyTokenEnvelopeReplay(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p214","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_latency_token_envelope_replay")
	if s["version"] != "s21-p214.v1" {
		t.Fatalf("version=%v, want s21-p214.v1", s["version"])
	}
	if s["binds_to"] != "vx18c_upstream_gate" {
		t.Fatalf("binds_to=%v, want vx18c_upstream_gate", s["binds_to"])
	}
	if s["no_new_latency_owner"] != true {
		t.Fatalf("no_new_latency_owner=%v, want true", s["no_new_latency_owner"])
	}
	if s["support_only"] != true {
		t.Fatalf("support_only=%v, want true", s["support_only"])
	}
	if s["mode"] != "seq21_latency_token_envelope_replay_definition" {
		t.Fatalf("mode=%v, want seq21_latency_token_envelope_replay_definition", s["mode"])
	}
}

func TestSeq21P215HeldOutRegressionGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p215","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_held_out_regression_gate")
	if s["version"] != "s21-p215.v1" {
		t.Fatalf("version=%v, want s21-p215.v1", s["version"])
	}
	if s["reuses_tuning_loop"] != true {
		t.Fatalf("reuses_tuning_loop=%v, want true", s["reuses_tuning_loop"])
	}
	if s["support_only"] != true {
		t.Fatalf("support_only=%v, want true", s["support_only"])
	}
	if s["mode"] != "seq21_held_out_regression_gate_definition" {
		t.Fatalf("mode=%v, want seq21_held_out_regression_gate_definition", s["mode"])
	}
}

func TestSeq21P216PostChromaDefaultPromotionCriteria(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p216","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_post_chroma_default_promotion_criteria")
	if s["version"] != "s21-p216.v1" {
		t.Fatalf("version=%v, want s21-p216.v1", s["version"])
	}
	if s["promotion_status"] != "hold" {
		t.Fatalf("promotion_status=%v, want hold", s["promotion_status"])
	}
	if s["no_deadlock_on_inactive_baseline"] != true {
		t.Fatalf("no_deadlock_on_inactive_baseline=%v, want true", s["no_deadlock_on_inactive_baseline"])
	}
	if s["support_only"] != true {
		t.Fatalf("support_only=%v, want true", s["support_only"])
	}
	if s["mode"] != "seq21_post_chroma_default_promotion_criteria_definition" {
		t.Fatalf("mode=%v, want seq21_post_chroma_default_promotion_criteria_definition", s["mode"])
	}
}

func TestSeq21P217CostNormalizedTailRecallGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p217","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_cost_normalized_tail_recall_gate")
	if s["version"] != "s21-p217.v1" {
		t.Fatalf("version=%v, want s21-p217.v1", s["version"])
	}
	if s["verification_target"] != "actual_tail_miss_reduction" {
		t.Fatalf("verification_target=%v, want actual_tail_miss_reduction", s["verification_target"])
	}
	if s["support_only"] != true {
		t.Fatalf("support_only=%v, want true", s["support_only"])
	}
	if s["mode"] != "seq21_cost_normalized_tail_recall_gate_definition" {
		t.Fatalf("mode=%v, want seq21_cost_normalized_tail_recall_gate_definition", s["mode"])
	}
}

func TestSeq21P218DensityMixReplay(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p218","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_density_mix_replay")
	if s["version"] != "s21-p218.v1" {
		t.Fatalf("version=%v, want s21-p218.v1", s["version"])
	}
	if s["token_ceiling_check"] != true {
		t.Fatalf("token_ceiling_check=%v, want true", s["token_ceiling_check"])
	}
	if s["arc_monopoly_check"] != true {
		t.Fatalf("arc_monopoly_check=%v, want true", s["arc_monopoly_check"])
	}
	if s["does_not_reopen_bridge"] != true {
		t.Fatalf("does_not_reopen_bridge=%v, want true", s["does_not_reopen_bridge"])
	}
	if s["support_only"] != true {
		t.Fatalf("support_only=%v, want true", s["support_only"])
	}
	if s["mode"] != "seq21_density_mix_replay_definition" {
		t.Fatalf("mode=%v, want seq21_density_mix_replay_definition", s["mode"])
	}
}

func TestSeq21P219SharedRunnerCorpusRule(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p219","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_shared_runner_corpus_rule")
	if s["version"] != "s21-p219.v1" {
		t.Fatalf("version=%v, want s21-p219.v1", s["version"])
	}
	if s["shared_runner_allowed"] != true {
		t.Fatalf("shared_runner_allowed=%v, want true", s["shared_runner_allowed"])
	}
	if s["step21_corpus_isolated"] != true {
		t.Fatalf("step21_corpus_isolated=%v, want true", s["step21_corpus_isolated"])
	}
	if s["adoption_checklist_isolated"] != true {
		t.Fatalf("adoption_checklist_isolated=%v, want true", s["adoption_checklist_isolated"])
	}
	if s["support_only"] != true {
		t.Fatalf("support_only=%v, want true", s["support_only"])
	}
	if s["mode"] != "seq21_shared_runner_corpus_rule_definition" {
		t.Fatalf("mode=%v, want seq21_shared_runner_corpus_rule_definition", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-21 Beta 1.2 release gate tests (P223 ~ P227)
// ---------------------------------------------------------------------------

func TestSeq21P223Beta12BundleDryRun(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p223","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_beta12_bundle_dry_run")
	if s["version"] != "s21-p223.v1" {
		t.Fatalf("version=%v, want s21-p223.v1", s["version"])
	}
	if s["dry_run"] != true {
		t.Fatalf("dry_run=%v, want true", s["dry_run"])
	}
	if s["mode"] != "seq21_beta12_bundle_dry_run_definition" {
		t.Fatalf("mode=%v, want seq21_beta12_bundle_dry_run_definition", s["mode"])
	}
}

func TestSeq21P224SelectiveRerankTriggerSmoke(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p224","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_selective_rerank_trigger_smoke")
	if s["version"] != "s21-p224.v1" {
		t.Fatalf("version=%v, want s21-p224.v1", s["version"])
	}
	if s["smoke_target"] != "selective_rerank_trigger" {
		t.Fatalf("smoke_target=%v, want selective_rerank_trigger", s["smoke_target"])
	}
	refs := seq165Slice(t, s, "references")
	for _, expected := range []string{"seq21_rerank_trigger_class", "seq21_rerank_support_only_schema", "seq21_rerank_off_fallback"} {
		if !sliceContains(refs, expected) {
			t.Fatalf("references missing %q: %v", expected, refs)
		}
	}
	if s["mode"] != "seq21_selective_rerank_trigger_smoke_definition" {
		t.Fatalf("mode=%v, want seq21_selective_rerank_trigger_smoke_definition", s["mode"])
	}
}

func TestSeq21P225CandidateBudgetLatencySmoke(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p225","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_candidate_budget_latency_smoke")
	if s["version"] != "s21-p225.v1" {
		t.Fatalf("version=%v, want s21-p225.v1", s["version"])
	}
	if s["smoke_target"] != "candidate_budget_latency" {
		t.Fatalf("smoke_target=%v, want candidate_budget_latency", s["smoke_target"])
	}
	refs := seq165Slice(t, s, "references")
	for _, expected := range []string{"seq21_query_class_candidate_cap", "seq21_latency_budget_degrade", "seq21_retrieval_cache_reuse"} {
		if !sliceContains(refs, expected) {
			t.Fatalf("references missing %q: %v", expected, refs)
		}
	}
	if s["mode"] != "seq21_candidate_budget_latency_smoke_definition" {
		t.Fatalf("mode=%v, want seq21_candidate_budget_latency_smoke_definition", s["mode"])
	}
}

func TestSeq21P226FailureClassTuningReviewChecklist(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p226","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_failure_class_tuning_review_checklist")
	if s["version"] != "s21-p226.v1" {
		t.Fatalf("version=%v, want s21-p226.v1", s["version"])
	}
	items := seq165Slice(t, s, "checklist_items")
	for _, expected := range []string{"failure_taxonomy_defined", "dev_split_tuning_loop_active", "held_out_confirmation_gate_present", "residual_long_tail_tracked"} {
		if !sliceContains(items, expected) {
			t.Fatalf("checklist_items missing %q: %v", expected, items)
		}
	}
	if s["mode"] != "seq21_failure_class_tuning_review_checklist_definition" {
		t.Fatalf("mode=%v, want seq21_failure_class_tuning_review_checklist_definition", s["mode"])
	}
}

func TestSeq21P227HeldOutCostAdoptionGateComplete(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p227","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_held_out_cost_adoption_gate_complete")
	if s["version"] != "s21-p227.v1" {
		t.Fatalf("version=%v, want s21-p227.v1", s["version"])
	}
	if s["gate_status"] != "complete" {
		t.Fatalf("gate_status=%v, want complete", s["gate_status"])
	}
	evidence := seq165Slice(t, s, "required_evidence")
	for _, expected := range []string{"held_out_regression_gate_green", "cost_vs_gain_replay_green", "density_mix_replay_green", "shared_runner_corpus_isolated"} {
		if !sliceContains(evidence, expected) {
			t.Fatalf("required_evidence missing %q: %v", expected, evidence)
		}
	}
	if s["mode"] != "seq21_held_out_cost_adoption_gate_complete_definition" {
		t.Fatalf("mode=%v, want seq21_held_out_cost_adoption_gate_complete_definition", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-21 final preserve decision tests (P238 ~ P241)
// ---------------------------------------------------------------------------

func TestSeq21P238BoundedTriggerClassesPreserve(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p238","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_bounded_trigger_classes_preserve")
	if s["version"] != "s21-p238.v1" {
		t.Fatalf("version=%v, want s21-p238.v1", s["version"])
	}
	preserved := seq165Slice(t, s, "preserved")
	for _, expected := range []string{"temporal_callback_resume_canon_classes_only", "bounded_trigger_surface", "scene_remains_off_path"} {
		if !sliceContains(preserved, expected) {
			t.Fatalf("preserved missing %q: %v", expected, preserved)
		}
	}
	if s["mode"] != "seq21_bounded_trigger_classes_preserve_definition" {
		t.Fatalf("mode=%v, want seq21_bounded_trigger_classes_preserve_definition", s["mode"])
	}
}

func TestSeq21P239QueryClassCandidateCapPreserve(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p239","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_query_class_candidate_cap_preserve")
	if s["version"] != "s21-p239.v1" {
		t.Fatalf("version=%v, want s21-p239.v1", s["version"])
	}
	preserved := seq165Slice(t, s, "preserved")
	for _, expected := range []string{"query_class_budget_profile", "shared_top_k_baseline_never_shrinks"} {
		if !sliceContains(preserved, expected) {
			t.Fatalf("preserved missing %q: %v", expected, preserved)
		}
	}
	if s["mode"] != "seq21_query_class_candidate_cap_preserve_definition" {
		t.Fatalf("mode=%v, want seq21_query_class_candidate_cap_preserve_definition", s["mode"])
	}
}

func TestSeq21P240LatencyDegradePathPreserve(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p240","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_latency_degrade_path_preserve")
	if s["version"] != "s21-p240.v1" {
		t.Fatalf("version=%v, want s21-p240.v1", s["version"])
	}
	preserved := seq165Slice(t, s, "preserved")
	for _, expected := range []string{"latency_or_budget_hold_then_fail_open", "hybrid_recall_fail_open"} {
		if !sliceContains(preserved, expected) {
			t.Fatalf("preserved missing %q: %v", expected, preserved)
		}
	}
	if s["mode"] != "seq21_latency_degrade_path_preserve_definition" {
		t.Fatalf("mode=%v, want seq21_latency_degrade_path_preserve_definition", s["mode"])
	}
}

func TestSeq21P241TuningDeferredPreserve(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq21-p241","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "seq21_tuning_deferred_preserve")
	if s["version"] != "s21-p241.v1" {
		t.Fatalf("version=%v, want s21-p241.v1", s["version"])
	}
	preserved := seq165Slice(t, s, "preserved")
	for _, expected := range []string{"route_matrix_auto_apply_deferred", "held_out_cost_adoption_gate_green_required"} {
		if !sliceContains(preserved, expected) {
			t.Fatalf("preserved missing %q: %v", expected, preserved)
		}
	}
	if s["mode"] != "seq21_tuning_deferred_preserve_definition" {
		t.Fatalf("mode=%v, want seq21_tuning_deferred_preserve_definition", s["mode"])
	}
}
