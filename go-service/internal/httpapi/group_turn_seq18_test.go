package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// SEQ-18 reset administration tests (P13 ~ P15)
// ---------------------------------------------------------------------------

// TestSeq18P13ResetAdmin validates the Step 18 reset administration surface.
func TestSeq18P13ResetAdmin(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p13","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "reset_admin")
	if s["version"] != "seq18_p13.v1" {
		t.Fatalf("version=%v, want seq18_p13.v1", s["version"])
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

// TestSeq18P14HistoricalContentPreserved validates the historical content
// preservation surface.
func TestSeq18P14HistoricalContentPreserved(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p14","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "historical_content_preserved")
	if s["version"] != "seq18_p14.v1" {
		t.Fatalf("version=%v, want seq18_p14.v1", s["version"])
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

// TestSeq18P15ResetNoteOnly validates the reset scope note surface.
func TestSeq18P15ResetNoteOnly(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p15","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "reset_note_only")
	if s["version"] != "seq18_p15.v1" {
		t.Fatalf("version=%v, want seq18_p15.v1", s["version"])
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
// SEQ-18 preparation kick-off tests (P19 ~ P25)
// ---------------------------------------------------------------------------

// TestSeq18P19Step17ClosureGate validates the Step 17 closure entry gate surface.
func TestSeq18P19Step17ClosureGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p19","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "step17_closure_gate")
	if s["version"] != "seq18_p19.v1" {
		t.Fatalf("version=%v, want seq18_p19.v1", s["version"])
	}
	if s["role"] != "step_17_closure_gate" {
		t.Fatalf("role=%v, want step_17_closure_gate", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["closure_status"] != "closed" {
		t.Fatalf("closure_status=%v, want closed", s["closure_status"])
	}
	if s["release_gate_closed"] != true {
		t.Fatalf("release_gate_closed=%v, want true", s["release_gate_closed"])
	}
	if s["entry_gate_confirmed"] != true {
		t.Fatalf("entry_gate_confirmed=%v, want true", s["entry_gate_confirmed"])
	}
	if s["mode"] != "step_17_closure_entry_gate" {
		t.Fatalf("mode=%v, want step_17_closure_entry_gate", s["mode"])
	}
}

// TestSeq18P20ContextFilesReviewed validates the context files reviewed surface.
func TestSeq18P20ContextFilesReviewed(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p20","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "context_files_reviewed")
	if s["version"] != "seq18_p20.v1" {
		t.Fatalf("version=%v, want seq18_p20.v1", s["version"])
	}
	if s["role"] != "context_files_reviewed" {
		t.Fatalf("role=%v, want context_files_reviewed", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["files_reviewed"] != true {
		t.Fatalf("files_reviewed=%v, want true", s["files_reviewed"])
	}
	if s["redo_baseline_ready"] != true {
		t.Fatalf("redo_baseline_ready=%v, want true", s["redo_baseline_ready"])
	}
	if s["mode"] != "context_files_review_note" {
		t.Fatalf("mode=%v, want context_files_review_note", s["mode"])
	}
}

// TestSeq18P21PrepAnchorVRHY validates the preparatory anchor VR+HY surface.
func TestSeq18P21PrepAnchorVRHY(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p21","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "prep_anchor_vrhy")
	if s["version"] != "seq18_p21.v1" {
		t.Fatalf("version=%v, want seq18_p21.v1", s["version"])
	}
	if s["role"] != "prep_anchor_vr_hy" {
		t.Fatalf("role=%v, want prep_anchor_vr_hy", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	primary, ok := s["primary_anchors"].([]any)
	if !ok || len(primary) != 2 {
		t.Fatalf("primary_anchors=%v, want 2 items", s["primary_anchors"])
	}
	downstream, ok := s["downstream_slices"].([]any)
	if !ok || len(downstream) != 2 {
		t.Fatalf("downstream_slices=%v, want 2 items", s["downstream_slices"])
	}
	if s["mode"] != "preparatory_anchor_definition" {
		t.Fatalf("mode=%v, want preparatory_anchor_definition", s["mode"])
	}
}

// TestSeq18P22HistoricalReferenceOnly validates the historical reference only surface.
func TestSeq18P22HistoricalReferenceOnly(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p22","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "historical_reference_only")
	if s["version"] != "seq18_p22.v1" {
		t.Fatalf("version=%v, want seq18_p22.v1", s["version"])
	}
	if s["role"] != "historical_reference_only" {
		t.Fatalf("role=%v, want historical_reference_only", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["historical_text"] != "reference_only" {
		t.Fatalf("historical_text=%v, want reference_only", s["historical_text"])
	}
	if s["new_validation_needed"] != true {
		t.Fatalf("new_validation_needed=%v, want true", s["new_validation_needed"])
	}
	if s["mode"] != "historical_reference_status_note" {
		t.Fatalf("mode=%v, want historical_reference_status_note", s["mode"])
	}
}

// TestSeq18P23BackendPrepAnchor validates the backend preparation anchor surface.
func TestSeq18P23BackendPrepAnchor(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p23","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "backend_prep_anchor")
	if s["version"] != "seq18_p23.v1" {
		t.Fatalf("version=%v, want seq18_p23.v1", s["version"])
	}
	if s["role"] != "backend_prep_anchor" {
		t.Fatalf("role=%v, want backend_prep_anchor", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["anchor_file"] != "backend/archive/bridge.py" {
		t.Fatalf("anchor_file=%v, want backend/archive/bridge.py", s["anchor_file"])
	}
	if s["anchor_function"] != "search_memories" {
		t.Fatalf("anchor_function=%v, want search_memories", s["anchor_function"])
	}
	if s["mode"] != "backend_preparation_anchor" {
		t.Fatalf("mode=%v, want backend_preparation_anchor", s["mode"])
	}
}

// TestSeq18P24RoutingContractPrepAnchor validates the routing-contract prep anchor surface.
func TestSeq18P24RoutingContractPrepAnchor(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p24","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "routing_contract_prep_anchor")
	if s["version"] != "seq18_p24.v1" {
		t.Fatalf("version=%v, want seq18_p24.v1", s["version"])
	}
	if s["role"] != "routing_contract_prep_anchor" {
		t.Fatalf("role=%v, want routing_contract_prep_anchor", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["anchor_file"] != "backend/main.py" {
		t.Fatalf("anchor_file=%v, want backend/main.py", s["anchor_file"])
	}
	if s["anchor_function"] != "_build_recall_intent_contract_q3a" {
		t.Fatalf("anchor_function=%v, want _build_recall_intent_contract_q3a", s["anchor_function"])
	}
	if s["mode"] != "routing_contract_preparation_anchor" {
		t.Fatalf("mode=%v, want routing_contract_preparation_anchor", s["mode"])
	}
}

// TestSeq18P25RuntimePrepScope validates the runtime preparation scope surface.
func TestSeq18P25RuntimePrepScope(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p25","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "runtime_prep_scope")
	if s["version"] != "seq18_p25.v1" {
		t.Fatalf("version=%v, want seq18_p25.v1", s["version"])
	}
	if s["role"] != "runtime_prep_scope" {
		t.Fatalf("role=%v, want runtime_prep_scope", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["explicit_labels"] != false {
		t.Fatalf("explicit_labels=%v, want false", s["explicit_labels"])
	}
	if s["scope"] != "preparation_only" {
		t.Fatalf("scope=%v, want preparation_only", s["scope"])
	}
	if s["mode"] != "runtime_preparation_scope_note" {
		t.Fatalf("mode=%v, want runtime_preparation_scope_note", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-18 VR (verbatim recall) tests (P29 ~ P35)
// ---------------------------------------------------------------------------

// TestSeq18P29VRScopedVerbatimSupportText validates the VR scoped verbatim
// support text surface.
func TestSeq18P29VRScopedVerbatimSupportText(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p29","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vr_scoped_verbatim_support_text")
	if s["version"] != "seq18_p29.v1" {
		t.Fatalf("version=%v, want seq18_p29.v1", s["version"])
	}
	if s["role"] != "vr_scoped_verbatim_support" {
		t.Fatalf("role=%v, want vr_scoped_verbatim_support", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["source"] != "direct_evidence_gate_approved" {
		t.Fatalf("source=%v, want direct_evidence_gate_approved", s["source"])
	}
	if s["policy_version"] != "vr18a.v1" {
		t.Fatalf("policy_version=%v, want vr18a.v1", s["policy_version"])
	}
	if s["mode"] != "vr_scoped_verbatim_support_surface" {
		t.Fatalf("mode=%v, want vr_scoped_verbatim_support_surface", s["mode"])
	}
}

// TestSeq18P30VRPolicyOwnerBlock validates the VR policy owner block surface.
func TestSeq18P30VRPolicyOwnerBlock(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p30","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vr_policy_owner_block")
	if s["version"] != "seq18_p30.v1" {
		t.Fatalf("version=%v, want seq18_p30.v1", s["version"])
	}
	if s["role"] != "vr_policy_owner_block" {
		t.Fatalf("role=%v, want vr_policy_owner_block", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["max_items"] != float64(3) {
		t.Fatalf("max_items=%v, want 3", s["max_items"])
	}
	if s["max_total_chars"] != float64(720) {
		t.Fatalf("max_total_chars=%v, want 720", s["max_total_chars"])
	}
	if s["max_excerpt_chars"] != float64(160) {
		t.Fatalf("max_excerpt_chars=%v, want 160", s["max_excerpt_chars"])
	}
	if s["support_surface_first"] != true {
		t.Fatalf("support_surface_first=%v, want true", s["support_surface_first"])
	}
	if s["prompt_injection_strategy"] != "latest_anchor_only" {
		t.Fatalf("prompt_injection_strategy=%v, want latest_anchor_only", s["prompt_injection_strategy"])
	}
	if s["policy_version"] != "vr18b.v1" {
		t.Fatalf("policy_version=%v, want vr18b.v1", s["policy_version"])
	}
	if s["mode"] != "vr_policy_owner_block_definition" {
		t.Fatalf("mode=%v, want vr_policy_owner_block_definition", s["mode"])
	}
}

// TestSeq18P31VRPromptInjectionStrategy validates the VR prompt injection
// strategy surface.
func TestSeq18P31VRPromptInjectionStrategy(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p31","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vr_prompt_injection_strategy")
	if s["version"] != "seq18_p31.v1" {
		t.Fatalf("version=%v, want seq18_p31.v1", s["version"])
	}
	if s["role"] != "vr_prompt_injection_strategy" {
		t.Fatalf("role=%v, want vr_prompt_injection_strategy", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["injection_strategy"] != "latest_anchor_only" {
		t.Fatalf("injection_strategy=%v, want latest_anchor_only", s["injection_strategy"])
	}
	if s["multi_item_lane_exposed"] != true {
		t.Fatalf("multi_item_lane_exposed=%v, want true", s["multi_item_lane_exposed"])
	}
	if s["multi_item_lane_label"] != "Scoped Verbatim Recall (support surface)" {
		t.Fatalf("multi_item_lane_label=%v, want Scoped Verbatim Recall (support surface)", s["multi_item_lane_label"])
	}
	if s["prompt_injection_widened"] != false {
		t.Fatalf("prompt_injection_widened=%v, want false", s["prompt_injection_widened"])
	}
	if s["policy_version"] != "vr18c.v1" {
		t.Fatalf("policy_version=%v, want vr18c.v1", s["policy_version"])
	}
	if s["mode"] != "vr_prompt_injection_strategy_note" {
		t.Fatalf("mode=%v, want vr_prompt_injection_strategy_note", s["mode"])
	}
}

// TestSeq18P32VRHierarchyEscapeHatch validates the VR hierarchy escape hatch surface.
func TestSeq18P32VRHierarchyEscapeHatch(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p32","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vr_hierarchy_escape_hatch")
	if s["version"] != "seq18_p32.v1" {
		t.Fatalf("version=%v, want seq18_p32.v1", s["version"])
	}
	if s["role"] != "vr_hierarchy_escape_hatch" {
		t.Fatalf("role=%v, want vr_hierarchy_escape_hatch", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["hierarchy_escape_hatch"] != true {
		t.Fatalf("hierarchy_escape_hatch=%v, want true", s["hierarchy_escape_hatch"])
	}
	if s["verbatim_support_surface_priority"] != true {
		t.Fatalf("verbatim_support_surface_priority=%v, want true", s["verbatim_support_surface_priority"])
	}
	if s["hierarchy_escape_hatch_status"] != "visible_when_summary_thin" {
		t.Fatalf("hierarchy_escape_hatch_status=%v, want visible_when_summary_thin", s["hierarchy_escape_hatch_status"])
	}
	if s["policy_version"] != "vr18d.v1" {
		t.Fatalf("policy_version=%v, want vr18d.v1", s["policy_version"])
	}
	if s["mode"] != "vr_hierarchy_escape_hatch_definition" {
		t.Fatalf("mode=%v, want vr_hierarchy_escape_hatch_definition", s["mode"])
	}
}

// TestSeq18P33VRBackendTestGuard validates the VR backend test guard surface.
func TestSeq18P33VRBackendTestGuard(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p33","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vr_backend_test_guard")
	if s["version"] != "seq18_p33.v1" {
		t.Fatalf("version=%v, want seq18_p33.v1", s["version"])
	}
	if s["role"] != "vr_backend_test_guard" {
		t.Fatalf("role=%v, want vr_backend_test_guard", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["test_file"] != "backend/test_step18_scoped_verbatim_support.py" {
		t.Fatalf("test_file=%v, want backend/test_step18_scoped_verbatim_support.py", s["test_file"])
	}
	guards, ok := s["guards"].([]any)
	if !ok || len(guards) != 5 {
		t.Fatalf("guards=%v, want 5 items", s["guards"])
	}
	if s["policy_version"] != "vr18a.v1" {
		t.Fatalf("policy_version=%v, want vr18a.v1", s["policy_version"])
	}
	if s["mode"] != "vr_backend_test_guard_surface" {
		t.Fatalf("mode=%v, want vr_backend_test_guard_surface", s["mode"])
	}
}

// TestSeq18P34VRRuntimeTransparency validates the VR runtime transparency surface.
func TestSeq18P34VRRuntimeTransparency(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p34","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vr_runtime_transparency")
	if s["version"] != "seq18_p34.v1" {
		t.Fatalf("version=%v, want seq18_p34.v1", s["version"])
	}
	if s["role"] != "vr_runtime_transparency" {
		t.Fatalf("role=%v, want vr_runtime_transparency", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["trace_write_through"] != true {
		t.Fatalf("trace_write_through=%v, want true", s["trace_write_through"])
	}
	if s["transparency_section"] != "Scoped Verbatim Recall (support surface)" {
		t.Fatalf("transparency_section=%v, want Scoped Verbatim Recall (support surface)", s["transparency_section"])
	}
	if s["test_file"] != "test_step18_scoped_verbatim_input_transparency.js" {
		t.Fatalf("test_file=%v, want test_step18_scoped_verbatim_input_transparency.js", s["test_file"])
	}
	if s["policy_version"] != "vr18a.v1" {
		t.Fatalf("policy_version=%v, want vr18a.v1", s["policy_version"])
	}
	if s["mode"] != "vr_runtime_transparency_surface" {
		t.Fatalf("mode=%v, want vr_runtime_transparency_surface", s["mode"])
	}
}

// TestSeq18P35VRRegressionBundleGreen validates the VR regression bundle green surface.
func TestSeq18P35VRRegressionBundleGreen(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p35","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vr_regression_bundle_green")
	if s["version"] != "seq18_p35.v1" {
		t.Fatalf("version=%v, want seq18_p35.v1", s["version"])
	}
	if s["role"] != "vr_regression_bundle_green" {
		t.Fatalf("role=%v, want vr_regression_bundle_green", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["vr_slice_green"] != true {
		t.Fatalf("vr_slice_green=%v, want true", s["vr_slice_green"])
	}
	if s["adjacent_step19_green"] != true {
		t.Fatalf("adjacent_step19_green=%v, want true", s["adjacent_step19_green"])
	}
	if s["combined_bundle_status"] != "green" {
		t.Fatalf("combined_bundle_status=%v, want green", s["combined_bundle_status"])
	}
	if s["policy_version"] != "vr18a.v1" {
		t.Fatalf("policy_version=%v, want vr18a.v1", s["policy_version"])
	}
	if s["mode"] != "vr_regression_bundle_green_note" {
		t.Fatalf("mode=%v, want vr_regression_bundle_green_note", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-18 HY (hybrid retrieval) tests (P46 ~ P53)
// ---------------------------------------------------------------------------

// TestSeq18P46HYSemanticRankKeywordOverlap validates the HY semantic rank +
// keyword overlap surface.
func TestSeq18P46HYSemanticRankKeywordOverlap(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p46","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "hy_semantic_rank_score")
	if s["version"] != "seq18_p46.v1" {
		t.Fatalf("version=%v, want seq18_p46.v1", s["version"])
	}
	if s["role"] != "hy_semantic_rank_keyword_overlap" {
		t.Fatalf("role=%v, want hy_semantic_rank_keyword_overlap", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["semantic_rank_preserved"] != true {
		t.Fatalf("semantic_rank_preserved=%v, want true", s["semantic_rank_preserved"])
	}
	if s["keyword_overlap_policy"] != "hy1a.v1" {
		t.Fatalf("keyword_overlap_policy=%v, want hy1a.v1", s["keyword_overlap_policy"])
	}
	if s["hybrid_baseline_policy_version"] != "hy1a.v1" {
		t.Fatalf("hybrid_baseline_policy_version=%v, want hy1a.v1", s["hybrid_baseline_policy_version"])
	}
	if s["policy_version"] != "hy1a.v1" {
		t.Fatalf("policy_version=%v, want hy1a.v1", s["policy_version"])
	}
	if s["mode"] != "hy_semantic_rank_keyword_overlap_surface" {
		t.Fatalf("mode=%v, want hy_semantic_rank_keyword_overlap_surface", s["mode"])
	}
}

// TestSeq18P47HYSoftBias validates the HY structured soft bias surface.
func TestSeq18P47HYSoftBias(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p47","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "hy_soft_bias")
	if s["version"] != "seq18_p47.v1" {
		t.Fatalf("version=%v, want seq18_p47.v1", s["version"])
	}
	if s["role"] != "hy_soft_bias" {
		t.Fatalf("role=%v, want hy_soft_bias", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["soft_bias_policy"] != "hy1b.v1" {
		t.Fatalf("soft_bias_policy=%v, want hy1b.v1", s["soft_bias_policy"])
	}
	if s["speaker_bias_weight"] != float64(0.04) {
		t.Fatalf("speaker_bias_weight=%v, want 0.04", s["speaker_bias_weight"])
	}
	if s["location_bias_weight"] != float64(0.05) {
		t.Fatalf("location_bias_weight=%v, want 0.05", s["location_bias_weight"])
	}
	if s["storyline_bias_weight"] != float64(0.06) {
		t.Fatalf("storyline_bias_weight=%v, want 0.06", s["storyline_bias_weight"])
	}
	if s["soft_bias_cap"] != float64(0.12) {
		t.Fatalf("soft_bias_cap=%v, want 0.12", s["soft_bias_cap"])
	}
	if s["soft_bias_policy_version"] != "hy1b.v1" {
		t.Fatalf("soft_bias_policy_version=%v, want hy1b.v1", s["soft_bias_policy_version"])
	}
	if s["policy_version"] != "hy1b.v1" {
		t.Fatalf("policy_version=%v, want hy1b.v1", s["policy_version"])
	}
	if s["mode"] != "hy_soft_bias_surface" {
		t.Fatalf("mode=%v, want hy_soft_bias_surface", s["mode"])
	}
}

// TestSeq18P48HYStopwordGuard validates the HY stopword guard surface.
func TestSeq18P48HYStopwordGuard(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p48","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "hy_stopword_guard")
	if s["version"] != "seq18_p48.v1" {
		t.Fatalf("version=%v, want seq18_p48.v1", s["version"])
	}
	if s["role"] != "hy_stopword_guard" {
		t.Fatalf("role=%v, want hy_stopword_guard", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["stopword_inflation_fixed"] != true {
		t.Fatalf("stopword_inflation_fixed=%v, want true", s["stopword_inflation_fixed"])
	}
	if s["tightened_extractor"] != true {
		t.Fatalf("tightened_extractor=%v, want true", s["tightened_extractor"])
	}
	if s["common_filler_excluded"] != true {
		t.Fatalf("common_filler_excluded=%v, want true", s["common_filler_excluded"])
	}
	if s["policy_version"] != "hy1a.v1" {
		t.Fatalf("policy_version=%v, want hy1a.v1", s["policy_version"])
	}
	if s["mode"] != "hy_stopword_guard_surface" {
		t.Fatalf("mode=%v, want hy_stopword_guard_surface", s["mode"])
	}
}

// TestSeq18P49HYQ1aPropagation validates the HY q1a propagation surface.
func TestSeq18P49HYQ1aPropagation(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p49","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "hy_q1a_propagation")
	if s["version"] != "seq18_p49.v1" {
		t.Fatalf("version=%v, want seq18_p49.v1", s["version"])
	}
	if s["role"] != "hy_q1a_propagation" {
		t.Fatalf("role=%v, want hy_q1a_propagation", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["q1a_propagation"] != true {
		t.Fatalf("q1a_propagation=%v, want true", s["q1a_propagation"])
	}
	fields, ok := s["propagated_fields"].([]any)
	if !ok || len(fields) != 9 {
		t.Fatalf("propagated_fields=%v, want 9 items", s["propagated_fields"])
	}
	if s["policy_version"] != "hy1a.v1" {
		t.Fatalf("policy_version=%v, want hy1a.v1", s["policy_version"])
	}
	if s["mode"] != "hy_q1a_propagation_surface" {
		t.Fatalf("mode=%v, want hy_q1a_propagation_surface", s["mode"])
	}
}

// TestSeq18P50HYRuntimeInspection validates the HY runtime inspection surface.
func TestSeq18P50HYRuntimeInspection(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p50","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "hy_runtime_inspection")
	if s["version"] != "seq18_p50.v1" {
		t.Fatalf("version=%v, want seq18_p50.v1", s["version"])
	}
	if s["role"] != "hy_runtime_inspection" {
		t.Fatalf("role=%v, want hy_runtime_inspection", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["js_function"] != "extractMemoryItems" {
		t.Fatalf("js_function=%v, want extractMemoryItems", s["js_function"])
	}
	if s["row_meta_extended"] != true {
		t.Fatalf("row_meta_extended=%v, want true", s["row_meta_extended"])
	}
	rowFields, ok := s["row_meta_fields"].([]any)
	if !ok || len(rowFields) != 3 {
		t.Fatalf("row_meta_fields=%v, want 3 items", s["row_meta_fields"])
	}
	if s["transparency_block"] != "Hybrid Retrieval Inspection" {
		t.Fatalf("transparency_block=%v, want Hybrid Retrieval Inspection", s["transparency_block"])
	}
	if s["transparency_block_type"] != "trace_only" {
		t.Fatalf("transparency_block_type=%v, want trace_only", s["transparency_block_type"])
	}
	if s["policy_version"] != "hy1b.v1" {
		t.Fatalf("policy_version=%v, want hy1b.v1", s["policy_version"])
	}
	if s["mode"] != "hy_runtime_inspection_surface" {
		t.Fatalf("mode=%v, want hy_runtime_inspection_surface", s["mode"])
	}
}

// TestSeq18P51HYRecurringRiskGuards validates the HY recurring-risk guard surface.
func TestSeq18P51HYRecurringRiskGuards(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p51","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "hy_recurring_risk_guards")
	if s["version"] != "seq18_p51.v1" {
		t.Fatalf("version=%v, want seq18_p51.v1", s["version"])
	}
	if s["role"] != "hy_recurring_risk_guards" {
		t.Fatalf("role=%v, want hy_recurring_risk_guards", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["backend_test_file"] != "backend/test_step18_hybrid_regression.py" {
		t.Fatalf("backend_test_file=%v, want backend/test_step18_hybrid_regression.py", s["backend_test_file"])
	}
	if s["js_test_file"] != "test_step18_hybrid_input_transparency.js" {
		t.Fatalf("js_test_file=%v, want test_step18_hybrid_input_transparency.js", s["js_test_file"])
	}
	guards, ok := s["guards"].([]any)
	if !ok || len(guards) != 3 {
		t.Fatalf("guards=%v, want 3 items", s["guards"])
	}
	if s["policy_version"] != "hy1a.v1" {
		t.Fatalf("policy_version=%v, want hy1a.v1", s["policy_version"])
	}
	if s["mode"] != "hy_recurring_risk_guard_surface" {
		t.Fatalf("mode=%v, want hy_recurring_risk_guard_surface", s["mode"])
	}
}

// TestSeq18P52HYPolicyRegistry validates the HY policy registry consolidation surface.
func TestSeq18P52HYPolicyRegistry(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p52","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "hy_policy_registry")
	if s["version"] != "seq18_p52.v1" {
		t.Fatalf("version=%v, want seq18_p52.v1", s["version"])
	}
	if s["role"] != "hy_policy_registry" {
		t.Fatalf("role=%v, want hy_policy_registry", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["registry_file"] != "backend/archive/hybrid_policy.py" {
		t.Fatalf("registry_file=%v, want backend/archive/hybrid_policy.py", s["registry_file"])
	}
	if s["consolidated"] != true {
		t.Fatalf("consolidated=%v, want true", s["consolidated"])
	}
	if s["scattered_hardcoded_removed"] != true {
		t.Fatalf("scattered_hardcoded_removed=%v, want true", s["scattered_hardcoded_removed"])
	}
	if s["policy_family"] != "hy" {
		t.Fatalf("policy_family=%v, want hy", s["policy_family"])
	}
	if s["policy_version"] != "hy1a.v1" {
		t.Fatalf("policy_version=%v, want hy1a.v1", s["policy_version"])
	}
	if s["mode"] != "hy_policy_registry_surface" {
		t.Fatalf("mode=%v, want hy_policy_registry_surface", s["mode"])
	}
}

// TestSeq18P53HYStopAt18_2c validates the HY intentional stop surface.
func TestSeq18P53HYStopAt18_2c(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p53","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "hy_stop_at_18_2c")
	if s["version"] != "seq18_p53.v1" {
		t.Fatalf("version=%v, want seq18_p53.v1", s["version"])
	}
	if s["role"] != "hy_stop_at_18_2c" {
		t.Fatalf("role=%v, want hy_stop_at_18_2c", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["stop_point"] != "18-2c" {
		t.Fatalf("stop_point=%v, want 18-2c", s["stop_point"])
	}
	openFollowUp, ok := s["open_follow_up"].([]any)
	if !ok || len(openFollowUp) != 3 {
		t.Fatalf("open_follow_up=%v, want 3 items", s["open_follow_up"])
	}
	if s["tail_budget_rescue"] != "pending" {
		t.Fatalf("tail_budget_rescue=%v, want pending", s["tail_budget_rescue"])
	}
	if s["policy_version"] != "hy1a.v1" {
		t.Fatalf("policy_version=%v, want hy1a.v1", s["policy_version"])
	}
	if s["mode"] != "hy_intentional_stop_note" {
		t.Fatalf("mode=%v, want hy_intentional_stop_note", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-18 HY tail-budget rescue tests (P65 ~ P69)
// ---------------------------------------------------------------------------

// TestSeq18P65HYTailBudgetPolicyOwner validates the HY tail-budget policy owner surface.
func TestSeq18P65HYTailBudgetPolicyOwner(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p65","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "hy_tail_budget_policy_owner")
	if s["version"] != "seq18_p65.v1" {
		t.Fatalf("version=%v, want seq18_p65.v1", s["version"])
	}
	if s["role"] != "hy_tail_budget_policy_owner" {
		t.Fatalf("role=%v, want hy_tail_budget_policy_owner", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["policy_family"] != "hy" {
		t.Fatalf("policy_family=%v, want hy", s["policy_family"])
	}
	if s["policy_version"] != "hy1d.v1" {
		t.Fatalf("policy_version=%v, want hy1d.v1", s["policy_version"])
	}
	if s["scattered_hardcoded_removed"] != true {
		t.Fatalf("scattered_hardcoded_removed=%v, want true", s["scattered_hardcoded_removed"])
	}
	if s["mode"] != "hy_tail_budget_policy_owner_surface" {
		t.Fatalf("mode=%v, want hy_tail_budget_policy_owner_surface", s["mode"])
	}
}

// TestSeq18P66HYTailBudgetRescuePass validates the HY tail-budget rescue pass surface.
func TestSeq18P66HYTailBudgetRescuePass(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p66","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "hy_tail_budget_rescue_pass")
	if s["version"] != "seq18_p66.v1" {
		t.Fatalf("version=%v, want seq18_p66.v1", s["version"])
	}
	if s["role"] != "hy_tail_budget_rescue_pass" {
		t.Fatalf("role=%v, want hy_tail_budget_rescue_pass", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["rescue_enabled"] != true {
		t.Fatalf("rescue_enabled=%v, want true", s["rescue_enabled"])
	}
	if s["max_promotions_per_pass"] != float64(1) {
		t.Fatalf("max_promotions_per_pass=%v, want 1", s["max_promotions_per_pass"])
	}
	if s["budget_preserved"] != true {
		t.Fatalf("budget_preserved=%v, want true", s["budget_preserved"])
	}
	if s["promotion_trigger"] != "keyword_soft_bias_stronger_than_cutline" {
		t.Fatalf("promotion_trigger=%v, want keyword_soft_bias_stronger_than_cutline", s["promotion_trigger"])
	}
	if s["policy_version"] != "hy1d.v1" {
		t.Fatalf("policy_version=%v, want hy1d.v1", s["policy_version"])
	}
	if s["mode"] != "hy_tail_budget_rescue_pass_surface" {
		t.Fatalf("mode=%v, want hy_tail_budget_rescue_pass_surface", s["mode"])
	}
}

// TestSeq18P67HYTailBudgetRescueTrace validates the HY tail-budget rescue trace surface.
func TestSeq18P67HYTailBudgetRescueTrace(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p67","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "hy_tail_budget_rescue_trace")
	if s["version"] != "seq18_p67.v1" {
		t.Fatalf("version=%v, want seq18_p67.v1", s["version"])
	}
	if s["role"] != "hy_tail_budget_rescue_trace" {
		t.Fatalf("role=%v, want hy_tail_budget_rescue_trace", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	traceFields, ok := s["trace_fields"].([]any)
	if !ok || len(traceFields) != 5 {
		t.Fatalf("trace_fields=%v, want 5 items", s["trace_fields"])
	}
	if s["trace_mandatory"] != true {
		t.Fatalf("trace_mandatory=%v, want true", s["trace_mandatory"])
	}
	if s["policy_version"] != "hy1d.v1" {
		t.Fatalf("policy_version=%v, want hy1d.v1", s["policy_version"])
	}
	if s["mode"] != "hy_tail_budget_rescue_trace_surface" {
		t.Fatalf("mode=%v, want hy_tail_budget_rescue_trace_surface", s["mode"])
	}
}

// TestSeq18P68HYTailBudgetQ1aPropagation validates the HY tail-budget q1a propagation surface.
func TestSeq18P68HYTailBudgetQ1aPropagation(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p68","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "hy_tail_budget_q1a_propagation")
	if s["version"] != "seq18_p68.v1" {
		t.Fatalf("version=%v, want seq18_p68.v1", s["version"])
	}
	if s["role"] != "hy_tail_budget_q1a_propagation" {
		t.Fatalf("role=%v, want hy_tail_budget_q1a_propagation", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["q1a_propagation"] != true {
		t.Fatalf("q1a_propagation=%v, want true", s["q1a_propagation"])
	}
	propFields, ok := s["propagated_fields"].([]any)
	if !ok || len(propFields) != 5 {
		t.Fatalf("propagated_fields=%v, want 5 items", s["propagated_fields"])
	}
	if s["policy_version"] != "hy1d.v1" {
		t.Fatalf("policy_version=%v, want hy1d.v1", s["policy_version"])
	}
	if s["mode"] != "hy_tail_budget_q1a_propagation_surface" {
		t.Fatalf("mode=%v, want hy_tail_budget_q1a_propagation_surface", s["mode"])
	}
}

// TestSeq18P69HYTailBudgetRegression validates the HY tail-budget regression surface.
func TestSeq18P69HYTailBudgetRegression(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p69","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "hy_tail_budget_regression")
	if s["version"] != "seq18_p69.v1" {
		t.Fatalf("version=%v, want seq18_p69.v1", s["version"])
	}
	if s["role"] != "hy_tail_budget_regression" {
		t.Fatalf("role=%v, want hy_tail_budget_regression", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["test_file"] != "backend/test_step18_hybrid_regression.py" {
		t.Fatalf("test_file=%v, want backend/test_step18_hybrid_regression.py", s["test_file"])
	}
	if s["regression_scope"] != "near_cutoff_rescue" {
		t.Fatalf("regression_scope=%v, want near_cutoff_rescue", s["regression_scope"])
	}
	verifies, ok := s["verifies"].([]any)
	if !ok || len(verifies) != 2 {
		t.Fatalf("verifies=%v, want 2 items", s["verifies"])
	}
	if s["policy_version"] != "hy1d.v1" {
		t.Fatalf("policy_version=%v, want hy1d.v1", s["policy_version"])
	}
	if s["mode"] != "hy_tail_budget_regression_surface" {
		t.Fatalf("mode=%v, want hy_tail_budget_regression_surface", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-18 QR query-class contract tests (P76 ~ P91)
// ---------------------------------------------------------------------------

// TestSeq18P76QRQueryClassContract validates the QR query-class contract surface.
func TestSeq18P76QRQueryClassContract(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p76","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "qr_query_class_contract")
	if s["version"] != "seq18_p76.v1" {
		t.Fatalf("version=%v, want seq18_p76.v1", s["version"])
	}
	if s["role"] != "qr_query_class_contract" {
		t.Fatalf("role=%v, want qr_query_class_contract", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["contract_version"] != "qr1a.v1" {
		t.Fatalf("contract_version=%v, want qr1a.v1", s["contract_version"])
	}
	if s["execution_mode"] != "single_query_shared" {
		t.Fatalf("execution_mode=%v, want single_query_shared", s["execution_mode"])
	}
	if s["fail_open"] != true {
		t.Fatalf("fail_open=%v, want true", s["fail_open"])
	}
	if s["additive_only"] != true {
		t.Fatalf("additive_only=%v, want true", s["additive_only"])
	}
	if s["policy_version"] != "qr1a.v1" {
		t.Fatalf("policy_version=%v, want qr1a.v1", s["policy_version"])
	}
	if s["mode"] != "qr_query_class_contract_surface" {
		t.Fatalf("mode=%v, want qr_query_class_contract_surface", s["mode"])
	}
}

// TestSeq18P77QRQueryClassTaxonomy validates the QR query-class taxonomy surface.
func TestSeq18P77QRQueryClassTaxonomy(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p77","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "qr_query_class_taxonomy")
	if s["version"] != "seq18_p77.v1" {
		t.Fatalf("version=%v, want seq18_p77.v1", s["version"])
	}
	if s["role"] != "qr_query_class_taxonomy" {
		t.Fatalf("role=%v, want qr_query_class_taxonomy", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	classes, ok := s["query_classes"].([]any)
	if !ok || len(classes) != 5 {
		t.Fatalf("query_classes=%v, want 5 items", s["query_classes"])
	}
	if s["contract_layer_only"] != true {
		t.Fatalf("contract_layer_only=%v, want true", s["contract_layer_only"])
	}
	if s["primary_class_visible"] != true {
		t.Fatalf("primary_class_visible=%v, want true", s["primary_class_visible"])
	}
	if s["policy_version"] != "qr1a.v1" {
		t.Fatalf("policy_version=%v, want qr1a.v1", s["policy_version"])
	}
	if s["mode"] != "qr_query_class_taxonomy_surface" {
		t.Fatalf("mode=%v, want qr_query_class_taxonomy_surface", s["mode"])
	}
}

// TestSeq18P78QRPrimaryClassSelection validates the QR primary class selection precedence surface.
func TestSeq18P78QRPrimaryClassSelection(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p78","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "qr_primary_class_selection")
	if s["version"] != "seq18_p78.v1" {
		t.Fatalf("version=%v, want seq18_p78.v1", s["version"])
	}
	if s["role"] != "qr_primary_class_selection" {
		t.Fatalf("role=%v, want qr_primary_class_selection", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	prec, ok := s["precedence"].([]any)
	if !ok || len(prec) != 5 {
		t.Fatalf("precedence=%v, want 5 items", s["precedence"])
	}
	if prec[0] != "explicit_temporal_cue" {
		t.Fatalf("precedence[0]=%v, want explicit_temporal_cue", prec[0])
	}
	if prec[4] != "scene_fallback" {
		t.Fatalf("precedence[4]=%v, want scene_fallback", prec[4])
	}
	if s["policy_version"] != "qr1a.v1" {
		t.Fatalf("policy_version=%v, want qr1a.v1", s["policy_version"])
	}
	if s["mode"] != "qr_primary_class_selection_surface" {
		t.Fatalf("mode=%v, want qr_primary_class_selection_surface", s["mode"])
	}
}

// TestSeq18P79QRLexicalCueBlock validates the QR lexical cue block surface.
func TestSeq18P79QRLexicalCueBlock(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p79","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "qr_lexical_cue_block")
	if s["version"] != "seq18_p79.v1" {
		t.Fatalf("version=%v, want seq18_p79.v1", s["version"])
	}
	if s["role"] != "qr_lexical_cue_block" {
		t.Fatalf("role=%v, want qr_lexical_cue_block", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["localized"] != true {
		t.Fatalf("localized=%v, want true", s["localized"])
	}
	if s["hidden_literals_removed"] != true {
		t.Fatalf("hidden_literals_removed=%v, want true", s["hidden_literals_removed"])
	}
	if s["policy_version"] != "qr1a.v1" {
		t.Fatalf("policy_version=%v, want qr1a.v1", s["policy_version"])
	}
	if s["mode"] != "qr_lexical_cue_block_surface" {
		t.Fatalf("mode=%v, want qr_lexical_cue_block_surface", s["mode"])
	}
}

// TestSeq18P80QRQueryClassContractTest validates the QR query-class contract test surface.
func TestSeq18P80QRQueryClassContractTest(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p80","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "qr_query_class_contract_test")
	if s["version"] != "seq18_p80.v1" {
		t.Fatalf("version=%v, want seq18_p80.v1", s["version"])
	}
	if s["role"] != "qr_query_class_contract_test" {
		t.Fatalf("role=%v, want qr_query_class_contract_test", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["test_file"] != "backend/test_step18_query_class_contract.py" {
		t.Fatalf("test_file=%v, want backend/test_step18_query_class_contract.py", s["test_file"])
	}
	covers, ok := s["covers"].([]any)
	if !ok || len(covers) != 3 {
		t.Fatalf("covers=%v, want 3 items", s["covers"])
	}
	if s["policy_version"] != "qr1a.v1" {
		t.Fatalf("policy_version=%v, want qr1a.v1", s["policy_version"])
	}
	if s["mode"] != "qr_query_class_contract_test_surface" {
		t.Fatalf("mode=%v, want qr_query_class_contract_test_surface", s["mode"])
	}
}

// TestSeq18P87QRQueryClassBudgetPolicy validates the QR query-class budget policy surface.
func TestSeq18P87QRQueryClassBudgetPolicy(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p87","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "qr_query_class_budget_policy")
	if s["version"] != "seq18_p87.v1" {
		t.Fatalf("version=%v, want seq18_p87.v1", s["version"])
	}
	if s["role"] != "qr_query_class_budget_policy" {
		t.Fatalf("role=%v, want qr_query_class_budget_policy", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["budget_policy_version"] != "qr1b.v1" {
		t.Fatalf("budget_policy_version=%v, want qr1b.v1", s["budget_policy_version"])
	}
	if s["execution_mode"] != "single_query_shared" {
		t.Fatalf("execution_mode=%v, want single_query_shared", s["execution_mode"])
	}
	if s["fail_open"] != true {
		t.Fatalf("fail_open=%v, want true", s["fail_open"])
	}
	if s["policy_version"] != "qr1b.v1" {
		t.Fatalf("policy_version=%v, want qr1b.v1", s["policy_version"])
	}
	if s["mode"] != "qr_query_class_budget_policy_surface" {
		t.Fatalf("mode=%v, want qr_query_class_budget_policy_surface", s["mode"])
	}
}

// TestSeq18P88QRQ3cBudgetReuse validates the QR q3c budget reuse surface.
func TestSeq18P88QRQ3cBudgetReuse(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p88","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "qr_q3c_budget_reuse")
	if s["version"] != "seq18_p88.v1" {
		t.Fatalf("version=%v, want seq18_p88.v1", s["version"])
	}
	if s["role"] != "qr_q3c_budget_reuse" {
		t.Fatalf("role=%v, want qr_q3c_budget_reuse", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["reused_budget_source"] != "q3c_intent_packet" {
		t.Fatalf("reused_budget_source=%v, want q3c_intent_packet", s["reused_budget_source"])
	}
	execClasses, ok := s["executable_classes"].([]any)
	if !ok || len(execClasses) != 4 {
		t.Fatalf("executable_classes=%v, want 4 items", s["executable_classes"])
	}
	if s["independent_budget_avoided"] != true {
		t.Fatalf("independent_budget_avoided=%v, want true", s["independent_budget_avoided"])
	}
	if s["policy_version"] != "qr1b.v1" {
		t.Fatalf("policy_version=%v, want qr1b.v1", s["policy_version"])
	}
	if s["mode"] != "qr_q3c_budget_reuse_surface" {
		t.Fatalf("mode=%v, want qr_q3c_budget_reuse_surface", s["mode"])
	}
}

// TestSeq18P89QRTemporalProfileBudget validates the QR temporal profile-based budget surface.
func TestSeq18P89QRTemporalProfileBudget(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p89","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "qr_temporal_profile_budget")
	if s["version"] != "seq18_p89.v1" {
		t.Fatalf("version=%v, want seq18_p89.v1", s["version"])
	}
	if s["role"] != "qr_temporal_profile_budget" {
		t.Fatalf("role=%v, want qr_temporal_profile_budget", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["profile_based"] != true {
		t.Fatalf("profile_based=%v, want true", s["profile_based"])
	}
	if s["evidence_first"] != true {
		t.Fatalf("evidence_first=%v, want true", s["evidence_first"])
	}
	if s["overlay_budget"] != true {
		t.Fatalf("overlay_budget=%v, want true", s["overlay_budget"])
	}
	caps, ok := s["candidate_caps"].([]any)
	if !ok || len(caps) != 3 {
		t.Fatalf("candidate_caps=%v, want 3 items", s["candidate_caps"])
	}
	if s["shared_profile_template"] != true {
		t.Fatalf("shared_profile_template=%v, want true", s["shared_profile_template"])
	}
	if s["policy_version"] != "qr1b.v1" {
		t.Fatalf("policy_version=%v, want qr1b.v1", s["policy_version"])
	}
	if s["mode"] != "qr_temporal_profile_budget_surface" {
		t.Fatalf("mode=%v, want qr_temporal_profile_budget_surface", s["mode"])
	}
}

// TestSeq18P90QRBudgetVisibility validates the QR budget visibility surface.
func TestSeq18P90QRBudgetVisibility(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p90","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "qr_budget_visibility")
	if s["version"] != "seq18_p90.v1" {
		t.Fatalf("version=%v, want seq18_p90.v1", s["version"])
	}
	if s["role"] != "qr_budget_visibility" {
		t.Fatalf("role=%v, want qr_budget_visibility", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	visibleFields, ok := s["visible_fields"].([]any)
	if !ok || len(visibleFields) != 4 {
		t.Fatalf("visible_fields=%v, want 4 items", s["visible_fields"])
	}
	if s["contract_data_visible"] != true {
		t.Fatalf("contract_data_visible=%v, want true", s["contract_data_visible"])
	}
	if s["hidden_builder_logic_removed"] != true {
		t.Fatalf("hidden_builder_logic_removed=%v, want true", s["hidden_builder_logic_removed"])
	}
	if s["policy_version"] != "qr1b.v1" {
		t.Fatalf("policy_version=%v, want qr1b.v1", s["policy_version"])
	}
	if s["mode"] != "qr_budget_visibility_surface" {
		t.Fatalf("mode=%v, want qr_budget_visibility_surface", s["mode"])
	}
}

// TestSeq18P91QRQueryClassBudgetTest validates the QR query-class budget test surface.
func TestSeq18P91QRQueryClassBudgetTest(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p91","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "qr_query_class_budget_test")
	if s["version"] != "seq18_p91.v1" {
		t.Fatalf("version=%v, want seq18_p91.v1", s["version"])
	}
	if s["role"] != "qr_query_class_budget_test" {
		t.Fatalf("role=%v, want qr_query_class_budget_test", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["test_file"] != "backend/test_step18_query_class_budget_policy.py" {
		t.Fatalf("test_file=%v, want backend/test_step18_query_class_budget_policy.py", s["test_file"])
	}
	covers, ok := s["covers"].([]any)
	if !ok || len(covers) != 2 {
		t.Fatalf("covers=%v, want 2 items", s["covers"])
	}
	if s["policy_version"] != "qr1b.v1" {
		t.Fatalf("policy_version=%v, want qr1b.v1", s["policy_version"])
	}
	if s["mode"] != "qr_query_class_budget_test_surface" {
		t.Fatalf("mode=%v, want qr_query_class_budget_test_surface", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-18 QR note/route policy tests (P98 ~ P113)
// ---------------------------------------------------------------------------

// TestSeq18P98QRNotePolicy validates the QR note policy surface.
func TestSeq18P98QRNotePolicy(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p98","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "qr_note_policy")
	if s["version"] != "seq18_p98.v1" {
		t.Fatalf("version=%v, want seq18_p98.v1", s["version"])
	}
	if s["role"] != "qr_note_policy" {
		t.Fatalf("role=%v, want qr_note_policy", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["note_policy_version"] != "qr1c.v1" {
		t.Fatalf("note_policy_version=%v, want qr1c.v1", s["note_policy_version"])
	}
	if s["execution_mode"] != "single_query_shared" {
		t.Fatalf("execution_mode=%v, want single_query_shared", s["execution_mode"])
	}
	if s["fail_open"] != true {
		t.Fatalf("fail_open=%v, want true", s["fail_open"])
	}
	if s["policy_version"] != "qr1c.v1" {
		t.Fatalf("policy_version=%v, want qr1c.v1", s["policy_version"])
	}
	if s["mode"] != "qr_note_policy_surface" {
		t.Fatalf("mode=%v, want qr_note_policy_surface", s["mode"])
	}
}

// TestSeq18P99QRSceneCanonNoPreExtract validates the QR scene/canon no-pre-extract surface.
func TestSeq18P99QRSceneCanonNoPreExtract(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p99","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "qr_scene_canon_no_pre_extract")
	if s["version"] != "seq18_p99.v1" {
		t.Fatalf("version=%v, want seq18_p99.v1", s["version"])
	}
	if s["role"] != "qr_scene_canon_no_pre_extract" {
		t.Fatalf("role=%v, want qr_scene_canon_no_pre_extract", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	noPreClasses, ok := s["no_pre_extract_classes"].([]any)
	if !ok || len(noPreClasses) != 2 {
		t.Fatalf("no_pre_extract_classes=%v, want 2 items", s["no_pre_extract_classes"])
	}
	noteSurfaces, ok := s["note_surfaces"].([]any)
	if !ok || len(noteSurfaces) != 2 {
		t.Fatalf("note_surfaces=%v, want 2 items", s["note_surfaces"])
	}
	if s["delivery_policy"] != "support_surface_first" {
		t.Fatalf("delivery_policy=%v, want support_surface_first", s["delivery_policy"])
	}
	if s["policy_version"] != "qr1c.v1" {
		t.Fatalf("policy_version=%v, want qr1c.v1", s["policy_version"])
	}
	if s["mode"] != "qr_scene_canon_no_pre_extract_surface" {
		t.Fatalf("mode=%v, want qr_scene_canon_no_pre_extract_surface", s["mode"])
	}
}

// TestSeq18P100QRCallbackResumeTemporalNoteOnly validates the QR callback/resume/temporal note-only surface.
func TestSeq18P100QRCallbackResumeTemporalNoteOnly(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p100","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "qr_callback_resume_temporal_note_only")
	if s["version"] != "seq18_p100.v1" {
		t.Fatalf("version=%v, want seq18_p100.v1", s["version"])
	}
	if s["role"] != "qr_callback_resume_temporal_note_only" {
		t.Fatalf("role=%v, want qr_callback_resume_temporal_note_only", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	noteOnlyClasses, ok := s["note_only_classes"].([]any)
	if !ok || len(noteOnlyClasses) != 3 {
		t.Fatalf("note_only_classes=%v, want 3 items", s["note_only_classes"])
	}
	if s["pre_extract_behavior"] != "note_only_until_route_exec" {
		t.Fatalf("pre_extract_behavior=%v, want note_only_until_route_exec", s["pre_extract_behavior"])
	}
	if s["contract_layer_only"] != true {
		t.Fatalf("contract_layer_only=%v, want true", s["contract_layer_only"])
	}
	if s["policy_version"] != "qr1c.v1" {
		t.Fatalf("policy_version=%v, want qr1c.v1", s["policy_version"])
	}
	if s["mode"] != "qr_callback_resume_temporal_note_only_surface" {
		t.Fatalf("mode=%v, want qr_callback_resume_temporal_note_only_surface", s["mode"])
	}
}

// TestSeq18P101QRNotePolicyFields validates the QR note policy fields surface.
func TestSeq18P101QRNotePolicyFields(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p101","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "qr_note_policy_fields")
	if s["version"] != "seq18_p101.v1" {
		t.Fatalf("version=%v, want seq18_p101.v1", s["version"])
	}
	if s["role"] != "qr_note_policy_fields" {
		t.Fatalf("role=%v, want qr_note_policy_fields", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	visibleFields, ok := s["visible_fields"].([]any)
	if !ok || len(visibleFields) != 5 {
		t.Fatalf("visible_fields=%v, want 5 items", s["visible_fields"])
	}
	if s["additive_metadata"] != true {
		t.Fatalf("additive_metadata=%v, want true", s["additive_metadata"])
	}
	if s["policy_version"] != "qr1c.v1" {
		t.Fatalf("policy_version=%v, want qr1c.v1", s["policy_version"])
	}
	if s["mode"] != "qr_note_policy_fields_surface" {
		t.Fatalf("mode=%v, want qr_note_policy_fields_surface", s["mode"])
	}
}

// TestSeq18P102QRNotePolicyTest validates the QR note policy test surface.
func TestSeq18P102QRNotePolicyTest(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p102","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "qr_note_policy_test")
	if s["version"] != "seq18_p102.v1" {
		t.Fatalf("version=%v, want seq18_p102.v1", s["version"])
	}
	if s["role"] != "qr_note_policy_test" {
		t.Fatalf("role=%v, want qr_note_policy_test", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["test_file"] != "backend/test_step18_query_class_note_policy.py" {
		t.Fatalf("test_file=%v, want backend/test_step18_query_class_note_policy.py", s["test_file"])
	}
	guards, ok := s["guards"].([]any)
	if !ok || len(guards) != 2 {
		t.Fatalf("guards=%v, want 2 items", s["guards"])
	}
	if s["policy_version"] != "qr1c.v1" {
		t.Fatalf("policy_version=%v, want qr1c.v1", s["policy_version"])
	}
	if s["mode"] != "qr_note_policy_test_surface" {
		t.Fatalf("mode=%v, want qr_note_policy_test_surface", s["mode"])
	}
}

// TestSeq18P109QRRoutePolicy validates the QR route policy surface.
func TestSeq18P109QRRoutePolicy(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p109","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "qr_route_policy")
	if s["version"] != "seq18_p109.v1" {
		t.Fatalf("version=%v, want seq18_p109.v1", s["version"])
	}
	if s["role"] != "qr_route_policy" {
		t.Fatalf("role=%v, want qr_route_policy", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["route_policy_version"] != "qr1d.v1" {
		t.Fatalf("route_policy_version=%v, want qr1d.v1", s["route_policy_version"])
	}
	if s["execution_mode"] != "single_query_shared" {
		t.Fatalf("execution_mode=%v, want single_query_shared", s["execution_mode"])
	}
	if s["fail_open"] != true {
		t.Fatalf("fail_open=%v, want true", s["fail_open"])
	}
	if s["policy_version"] != "qr1d.v1" {
		t.Fatalf("policy_version=%v, want qr1d.v1", s["policy_version"])
	}
	if s["mode"] != "qr_route_policy_surface" {
		t.Fatalf("mode=%v, want qr_route_policy_surface", s["mode"])
	}
}

// TestSeq18P110QRRouteFamilies validates the QR route families surface.
func TestSeq18P110QRRouteFamilies(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p110","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "qr_route_families")
	if s["version"] != "seq18_p110.v1" {
		t.Fatalf("version=%v, want seq18_p110.v1", s["version"])
	}
	if s["role"] != "qr_route_families" {
		t.Fatalf("role=%v, want qr_route_families", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	families, ok := s["route_families"].([]any)
	if !ok || len(families) != 7 {
		t.Fatalf("route_families=%v, want 7 items", s["route_families"])
	}
	if s["metadata_visible"] != true {
		t.Fatalf("metadata_visible=%v, want true", s["metadata_visible"])
	}
	if s["policy_version"] != "qr1d.v1" {
		t.Fatalf("policy_version=%v, want qr1d.v1", s["policy_version"])
	}
	if s["mode"] != "qr_route_families_surface" {
		t.Fatalf("mode=%v, want qr_route_families_surface", s["mode"])
	}
}

// TestSeq18P111QRLongTailRouteCandidates validates the QR long-tail route candidates surface.
func TestSeq18P111QRLongTailRouteCandidates(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p111","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "qr_long_tail_route_candidates")
	if s["version"] != "seq18_p111.v1" {
		t.Fatalf("version=%v, want seq18_p111.v1", s["version"])
	}
	if s["role"] != "qr_long_tail_route_candidates" {
		t.Fatalf("role=%v, want qr_long_tail_route_candidates", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	ltClasses, ok := s["long_tail_classes"].([]any)
	if !ok || len(ltClasses) != 3 {
		t.Fatalf("long_tail_classes=%v, want 3 items", s["long_tail_classes"])
	}
	if s["promotion_trigger"] != "detail_old_detail_lexical_cue" {
		t.Fatalf("promotion_trigger=%v, want detail_old_detail_lexical_cue", s["promotion_trigger"])
	}
	if s["contract_layer_only"] != true {
		t.Fatalf("contract_layer_only=%v, want true", s["contract_layer_only"])
	}
	if s["runtime_unchanged"] != true {
		t.Fatalf("runtime_unchanged=%v, want true", s["runtime_unchanged"])
	}
	if s["policy_version"] != "qr1d.v1" {
		t.Fatalf("policy_version=%v, want qr1d.v1", s["policy_version"])
	}
	if s["mode"] != "qr_long_tail_route_candidates_surface" {
		t.Fatalf("mode=%v, want qr_long_tail_route_candidates_surface", s["mode"])
	}
}

// TestSeq18P112QRRoutePolicyFields validates the QR route policy fields surface.
func TestSeq18P112QRRoutePolicyFields(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p112","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "qr_route_policy_fields")
	if s["version"] != "seq18_p112.v1" {
		t.Fatalf("version=%v, want seq18_p112.v1", s["version"])
	}
	if s["role"] != "qr_route_policy_fields" {
		t.Fatalf("role=%v, want qr_route_policy_fields", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	visibleFields, ok := s["visible_fields"].([]any)
	if !ok || len(visibleFields) != 4 {
		t.Fatalf("visible_fields=%v, want 4 items", s["visible_fields"])
	}
	if s["publishes"] != "primary_selected_route" {
		t.Fatalf("publishes=%v, want primary_selected_route", s["publishes"])
	}
	if s["runtime_unchanged"] != true {
		t.Fatalf("runtime_unchanged=%v, want true", s["runtime_unchanged"])
	}
	if s["policy_version"] != "qr1d.v1" {
		t.Fatalf("policy_version=%v, want qr1d.v1", s["policy_version"])
	}
	if s["mode"] != "qr_route_policy_fields_surface" {
		t.Fatalf("mode=%v, want qr_route_policy_fields_surface", s["mode"])
	}
}

// TestSeq18P113QRRoutePolicyTest validates the QR route policy test surface.
func TestSeq18P113QRRoutePolicyTest(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p113","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "qr_route_policy_test")
	if s["version"] != "seq18_p113.v1" {
		t.Fatalf("version=%v, want seq18_p113.v1", s["version"])
	}
	if s["role"] != "qr_route_policy_test" {
		t.Fatalf("role=%v, want qr_route_policy_test", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["test_file"] != "backend/test_step18_query_class_route_policy.py" {
		t.Fatalf("test_file=%v, want backend/test_step18_query_class_route_policy.py", s["test_file"])
	}
	guards, ok := s["guards"].([]any)
	if !ok || len(guards) != 2 {
		t.Fatalf("guards=%v, want 2 items", s["guards"])
	}
	if s["policy_version"] != "qr1d.v1" {
		t.Fatalf("policy_version=%v, want qr1d.v1", s["policy_version"])
	}
	if s["mode"] != "qr_route_policy_test_surface" {
		t.Fatalf("mode=%v, want qr_route_policy_test_surface", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-18 VX validation gate tests (P120 ~ P143)
// ---------------------------------------------------------------------------

// TestSeq18P120VXHybridReplayGate validates the VX hybrid replay gate surface.
func TestSeq18P120VXHybridReplayGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p120","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vx_hybrid_replay_gate")
	if s["version"] != "seq18_p120.v1" {
		t.Fatalf("version=%v, want seq18_p120.v1", s["version"])
	}
	if s["role"] != "vx_hybrid_replay_gate" {
		t.Fatalf("role=%v, want vx_hybrid_replay_gate", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["gate_version"] != "vx18a.v1" {
		t.Fatalf("gate_version=%v, want vx18a.v1", s["gate_version"])
	}
	if s["gate_name"] != "hybrid_replay" {
		t.Fatalf("gate_name=%v, want hybrid_replay", s["gate_name"])
	}
	if s["execution_unchanged"] != true {
		t.Fatalf("execution_unchanged=%v, want true", s["execution_unchanged"])
	}
	if s["routing_unchanged"] != true {
		t.Fatalf("routing_unchanged=%v, want true", s["routing_unchanged"])
	}
	if s["additive_only"] != true {
		t.Fatalf("additive_only=%v, want true", s["additive_only"])
	}
	if s["policy_version"] != "vx18a.v1" {
		t.Fatalf("policy_version=%v, want vx18a.v1", s["policy_version"])
	}
	if s["mode"] != "vx_hybrid_replay_gate_surface" {
		t.Fatalf("mode=%v, want vx_hybrid_replay_gate_surface", s["mode"])
	}
}

// TestSeq18P121VXReplayThresholdReuse validates the VX replay threshold reuse surface.
func TestSeq18P121VXReplayThresholdReuse(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p121","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vx_replay_threshold_reuse")
	if s["version"] != "seq18_p121.v1" {
		t.Fatalf("version=%v, want seq18_p121.v1", s["version"])
	}
	if s["role"] != "vx_replay_threshold_reuse" {
		t.Fatalf("role=%v, want vx_replay_threshold_reuse", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["threshold_source"] != "_U1E_CAPTURED_REPLAY" {
		t.Fatalf("threshold_source=%v, want _U1E_CAPTURED_REPLAY", s["threshold_source"])
	}
	reusedFor, ok := s["reused_for"].([]any)
	if !ok || len(reusedFor) != 2 {
		t.Fatalf("reused_for=%v, want 2 items", s["reused_for"])
	}
	if s["disconnected_set_avoided"] != true {
		t.Fatalf("disconnected_set_avoided=%v, want true", s["disconnected_set_avoided"])
	}
	if s["policy_version"] != "vx18a.v1" {
		t.Fatalf("policy_version=%v, want vx18a.v1", s["policy_version"])
	}
	if s["mode"] != "vx_replay_threshold_reuse_surface" {
		t.Fatalf("mode=%v, want vx_replay_threshold_reuse_surface", s["mode"])
	}
}

// TestSeq18P122VXHybridReplayStates validates the VX hybrid replay gate states surface.
func TestSeq18P122VXHybridReplayStates(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p122","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vx_hybrid_replay_states")
	if s["version"] != "seq18_p122.v1" {
		t.Fatalf("version=%v, want seq18_p122.v1", s["version"])
	}
	if s["role"] != "vx_hybrid_replay_states" {
		t.Fatalf("role=%v, want vx_hybrid_replay_states", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	states, ok := s["state_machine"].([]any)
	if !ok || len(states) != 3 {
		t.Fatalf("state_machine=%v, want 3 items", s["state_machine"])
	}
	if s["policy_version"] != "vx18a.v1" {
		t.Fatalf("policy_version=%v, want vx18a.v1", s["policy_version"])
	}
	if s["mode"] != "vx_hybrid_replay_states_surface" {
		t.Fatalf("mode=%v, want vx_hybrid_replay_states_surface", s["mode"])
	}
}

// TestSeq18P123VXHybridReplayTest validates the VX hybrid replay gate test surface.
func TestSeq18P123VXHybridReplayTest(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p123","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vx_hybrid_replay_test")
	if s["version"] != "seq18_p123.v1" {
		t.Fatalf("version=%v, want seq18_p123.v1", s["version"])
	}
	if s["role"] != "vx_hybrid_replay_test" {
		t.Fatalf("role=%v, want vx_hybrid_replay_test", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["test_file"] != "backend/test_step18_hybrid_replay_gate.py" {
		t.Fatalf("test_file=%v, want backend/test_step18_hybrid_replay_gate.py", s["test_file"])
	}
	guards, ok := s["guards"].([]any)
	if !ok || len(guards) != 3 {
		t.Fatalf("guards=%v, want 3 items", s["guards"])
	}
	if s["policy_version"] != "vx18a.v1" {
		t.Fatalf("policy_version=%v, want vx18a.v1", s["policy_version"])
	}
	if s["mode"] != "vx_hybrid_replay_test_surface" {
		t.Fatalf("mode=%v, want vx_hybrid_replay_test_surface", s["mode"])
	}
}

// TestSeq18P130VXHeldoutCompletenessGate validates the VX heldout completeness gate surface.
func TestSeq18P130VXHeldoutCompletenessGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p130","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vx_heldout_completeness_gate")
	if s["version"] != "seq18_p130.v1" {
		t.Fatalf("version=%v, want seq18_p130.v1", s["version"])
	}
	if s["role"] != "vx_heldout_completeness_gate" {
		t.Fatalf("role=%v, want vx_heldout_completeness_gate", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["gate_version"] != "vx18b.v1" {
		t.Fatalf("gate_version=%v, want vx18b.v1", s["gate_version"])
	}
	if s["gate_name"] != "heldout_completeness" {
		t.Fatalf("gate_name=%v, want heldout_completeness", s["gate_name"])
	}
	if s["execution_unchanged"] != true {
		t.Fatalf("execution_unchanged=%v, want true", s["execution_unchanged"])
	}
	if s["routing_unchanged"] != true {
		t.Fatalf("routing_unchanged=%v, want true", s["routing_unchanged"])
	}
	if s["additive_only"] != true {
		t.Fatalf("additive_only=%v, want true", s["additive_only"])
	}
	if s["policy_version"] != "vx18b.v1" {
		t.Fatalf("policy_version=%v, want vx18b.v1", s["policy_version"])
	}
	if s["mode"] != "vx_heldout_completeness_gate_surface" {
		t.Fatalf("mode=%v, want vx_heldout_completeness_gate_surface", s["mode"])
	}
}

// TestSeq18P131VXHeldoutMetrics validates the VX heldout metrics surface.
func TestSeq18P131VXHeldoutMetrics(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p131","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vx_heldout_metrics")
	if s["version"] != "seq18_p131.v1" {
		t.Fatalf("version=%v, want seq18_p131.v1", s["version"])
	}
	if s["role"] != "vx_heldout_metrics" {
		t.Fatalf("role=%v, want vx_heldout_metrics", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	metrics, ok := s["metrics"].([]any)
	if !ok || len(metrics) != 3 {
		t.Fatalf("metrics=%v, want 3 items", s["metrics"])
	}
	if s["sample_sufficiency_required"] != true {
		t.Fatalf("sample_sufficiency_required=%v, want true", s["sample_sufficiency_required"])
	}
	states, ok := s["state_rules"].([]any)
	if !ok || len(states) != 4 {
		t.Fatalf("state_rules=%v, want 4 items", s["state_rules"])
	}
	if s["policy_version"] != "vx18b.v1" {
		t.Fatalf("policy_version=%v, want vx18b.v1", s["policy_version"])
	}
	if s["mode"] != "vx_heldout_metrics_surface" {
		t.Fatalf("mode=%v, want vx_heldout_metrics_surface", s["mode"])
	}
}

// TestSeq18P132VXHeldoutThresholdReuse validates the VX heldout threshold reuse surface.
func TestSeq18P132VXHeldoutThresholdReuse(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p132","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vx_heldout_threshold_reuse")
	if s["version"] != "seq18_p132.v1" {
		t.Fatalf("version=%v, want seq18_p132.v1", s["version"])
	}
	if s["role"] != "vx_heldout_threshold_reuse" {
		t.Fatalf("role=%v, want vx_heldout_threshold_reuse", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["completeness_floor"] != "LC1P_healthy" {
		t.Fatalf("completeness_floor=%v, want LC1P_healthy", s["completeness_floor"])
	}
	if s["sample_threshold_source"] != "_U1E_CAPTURED_REPLAY_MIN" {
		t.Fatalf("sample_threshold_source=%v, want _U1E_CAPTURED_REPLAY_MIN", s["sample_threshold_source"])
	}
	if s["disconnected_literal_avoided"] != true {
		t.Fatalf("disconnected_literal_avoided=%v, want true", s["disconnected_literal_avoided"])
	}
	if s["policy_version"] != "vx18b.v1" {
		t.Fatalf("policy_version=%v, want vx18b.v1", s["policy_version"])
	}
	if s["mode"] != "vx_heldout_threshold_reuse_surface" {
		t.Fatalf("mode=%v, want vx_heldout_threshold_reuse_surface", s["mode"])
	}
}

// TestSeq18P133VXHeldoutCompletenessTest validates the VX heldout completeness test surface.
func TestSeq18P133VXHeldoutCompletenessTest(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p133","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vx_heldout_completeness_test")
	if s["version"] != "seq18_p133.v1" {
		t.Fatalf("version=%v, want seq18_p133.v1", s["version"])
	}
	if s["role"] != "vx_heldout_completeness_test" {
		t.Fatalf("role=%v, want vx_heldout_completeness_test", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["test_file"] != "backend/test_step18_heldout_completeness_gate.py" {
		t.Fatalf("test_file=%v, want backend/test_step18_heldout_completeness_gate.py", s["test_file"])
	}
	guards, ok := s["guards"].([]any)
	if !ok || len(guards) != 4 {
		t.Fatalf("guards=%v, want 4 items", s["guards"])
	}
	if s["policy_version"] != "vx18b.v1" {
		t.Fatalf("policy_version=%v, want vx18b.v1", s["policy_version"])
	}
	if s["mode"] != "vx_heldout_completeness_test_surface" {
		t.Fatalf("mode=%v, want vx_heldout_completeness_test_surface", s["mode"])
	}
}

// TestSeq18P140VXLatencyTokenBudgetGate validates the VX latency/token budget gate surface.
func TestSeq18P140VXLatencyTokenBudgetGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p140","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vx_latency_token_budget_gate")
	if s["version"] != "seq18_p140.v1" {
		t.Fatalf("version=%v, want seq18_p140.v1", s["version"])
	}
	if s["role"] != "vx_latency_token_budget_gate" {
		t.Fatalf("role=%v, want vx_latency_token_budget_gate", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["gate_version"] != "vx18c.v1" {
		t.Fatalf("gate_version=%v, want vx18c.v1", s["gate_version"])
	}
	if s["gate_name"] != "latency_token_budget" {
		t.Fatalf("gate_name=%v, want latency_token_budget", s["gate_name"])
	}
	if s["execution_unchanged"] != true {
		t.Fatalf("execution_unchanged=%v, want true", s["execution_unchanged"])
	}
	if s["routing_unchanged"] != true {
		t.Fatalf("routing_unchanged=%v, want true", s["routing_unchanged"])
	}
	if s["additive_only"] != true {
		t.Fatalf("additive_only=%v, want true", s["additive_only"])
	}
	if s["policy_version"] != "vx18c.v1" {
		t.Fatalf("policy_version=%v, want vx18c.v1", s["policy_version"])
	}
	if s["mode"] != "vx_latency_token_budget_gate_surface" {
		t.Fatalf("mode=%v, want vx_latency_token_budget_gate_surface", s["mode"])
	}
}

// TestSeq18P141VXLatencyTokenMetrics validates the VX latency/token metrics surface.
func TestSeq18P141VXLatencyTokenMetrics(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p141","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vx_latency_token_metrics")
	if s["version"] != "seq18_p141.v1" {
		t.Fatalf("version=%v, want seq18_p141.v1", s["version"])
	}
	if s["role"] != "vx_latency_token_metrics" {
		t.Fatalf("role=%v, want vx_latency_token_metrics", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	metrics, ok := s["metrics"].([]any)
	if !ok || len(metrics) != 3 {
		t.Fatalf("metrics=%v, want 3 items", s["metrics"])
	}
	if s["sample_sufficiency_required"] != true {
		t.Fatalf("sample_sufficiency_required=%v, want true", s["sample_sufficiency_required"])
	}
	if s["default_token_ceiling"] != "packet_budget_policy.max_injection_chars" {
		t.Fatalf("default_token_ceiling=%v, want packet_budget_policy.max_injection_chars", s["default_token_ceiling"])
	}
	if s["policy_version"] != "vx18c.v1" {
		t.Fatalf("policy_version=%v, want vx18c.v1", s["policy_version"])
	}
	if s["mode"] != "vx_latency_token_metrics_surface" {
		t.Fatalf("mode=%v, want vx_latency_token_metrics_surface", s["mode"])
	}
}

// TestSeq18P142VXLatencyTokenThresholdReuse validates the VX latency/token threshold reuse surface.
func TestSeq18P142VXLatencyTokenThresholdReuse(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p142","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vx_latency_token_threshold_reuse")
	if s["version"] != "seq18_p142.v1" {
		t.Fatalf("version=%v, want seq18_p142.v1", s["version"])
	}
	if s["role"] != "vx_latency_token_threshold_reuse" {
		t.Fatalf("role=%v, want vx_latency_token_threshold_reuse", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["latency_ceiling_source"] != "_LC1M_MAX_SPLIT_LATENCY_MULTIPLIER" {
		t.Fatalf("latency_ceiling_source=%v, want _LC1M_MAX_SPLIT_LATENCY_MULTIPLIER", s["latency_ceiling_source"])
	}
	if s["token_ratio_owner"] != "gate_constant" {
		t.Fatalf("token_ratio_owner=%v, want gate_constant", s["token_ratio_owner"])
	}
	if s["scattered_literals_avoided"] != true {
		t.Fatalf("scattered_literals_avoided=%v, want true", s["scattered_literals_avoided"])
	}
	if s["policy_version"] != "vx18c.v1" {
		t.Fatalf("policy_version=%v, want vx18c.v1", s["policy_version"])
	}
	if s["mode"] != "vx_latency_token_threshold_reuse_surface" {
		t.Fatalf("mode=%v, want vx_latency_token_threshold_reuse_surface", s["mode"])
	}
}

// TestSeq18P143VXLatencyTokenTest validates the VX latency/token budget test surface.
func TestSeq18P143VXLatencyTokenTest(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p143","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vx_latency_token_test")
	if s["version"] != "seq18_p143.v1" {
		t.Fatalf("version=%v, want seq18_p143.v1", s["version"])
	}
	if s["role"] != "vx_latency_token_test" {
		t.Fatalf("role=%v, want vx_latency_token_test", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["test_file"] != "backend/test_step18_latency_token_budget_gate.py" {
		t.Fatalf("test_file=%v, want backend/test_step18_latency_token_budget_gate.py", s["test_file"])
	}
	guards, ok := s["guards"].([]any)
	if !ok || len(guards) != 4 {
		t.Fatalf("guards=%v, want 4 items", s["guards"])
	}
	if s["policy_version"] != "vx18c.v1" {
		t.Fatalf("policy_version=%v, want vx18c.v1", s["policy_version"])
	}
	if s["mode"] != "vx_latency_token_test_surface" {
		t.Fatalf("mode=%v, want vx_latency_token_test_surface", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-18 VX truth-boundary tests (P150 ~ P153)
// ---------------------------------------------------------------------------

// TestSeq18P150VXTruthBoundaryGate validates the VX truth-boundary gate surface.
func TestSeq18P150VXTruthBoundaryGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p150","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vx_truth_boundary_gate")
	if s["version"] != "seq18_p150.v1" {
		t.Fatalf("version=%v, want seq18_p150.v1", s["version"])
	}
	if s["role"] != "vx_truth_boundary_gate" {
		t.Fatalf("role=%v, want vx_truth_boundary_gate", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["gate_version"] != "vx18d.v1" {
		t.Fatalf("gate_version=%v, want vx18d.v1", s["gate_version"])
	}
	if s["gate_name"] != "truth_boundary_replay" {
		t.Fatalf("gate_name=%v, want truth_boundary_replay", s["gate_name"])
	}
	if s["evaluates_after"] != "injection_pack_data.packet_composition" {
		t.Fatalf("evaluates_after=%v, want injection_pack_data.packet_composition", s["evaluates_after"])
	}
	if s["additive_only"] != true {
		t.Fatalf("additive_only=%v, want true", s["additive_only"])
	}
	if s["policy_version"] != "vx18d.v1" {
		t.Fatalf("policy_version=%v, want vx18d.v1", s["policy_version"])
	}
	if s["mode"] != "vx_truth_boundary_gate_surface" {
		t.Fatalf("mode=%v, want vx_truth_boundary_gate_surface", s["mode"])
	}
}

// TestSeq18P151VXTruthBoundaryPrecedence validates the VX truth-boundary precedence surface.
func TestSeq18P151VXTruthBoundaryPrecedence(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p151","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vx_truth_boundary_precedence")
	if s["version"] != "seq18_p151.v1" {
		t.Fatalf("version=%v, want seq18_p151.v1", s["version"])
	}
	if s["role"] != "vx_truth_boundary_precedence" {
		t.Fatalf("role=%v, want vx_truth_boundary_precedence", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	evalFields, ok := s["evaluated_fields"].([]any)
	if !ok || len(evalFields) != 2 {
		t.Fatalf("evaluated_fields=%v, want 2 items", s["evaluated_fields"])
	}
	if s["precedence_model"] != "_LC1K_HIGH_AUTHORITY_SOURCES_vs_LOWER_TIER_SOURCES" {
		t.Fatalf("precedence_model=%v, want _LC1K_HIGH_AUTHORITY_SOURCES_vs_LOWER_TIER_SOURCES", s["precedence_model"])
	}
	if s["policy_version"] != "vx18d.v1" {
		t.Fatalf("policy_version=%v, want vx18d.v1", s["policy_version"])
	}
	if s["mode"] != "vx_truth_boundary_precedence_surface" {
		t.Fatalf("mode=%v, want vx_truth_boundary_precedence_surface", s["mode"])
	}
}

// TestSeq18P152VXTruthBoundaryStates validates the VX truth-boundary states surface.
func TestSeq18P152VXTruthBoundaryStates(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p152","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vx_truth_boundary_states")
	if s["version"] != "seq18_p152.v1" {
		t.Fatalf("version=%v, want seq18_p152.v1", s["version"])
	}
	if s["role"] != "vx_truth_boundary_states" {
		t.Fatalf("role=%v, want vx_truth_boundary_states", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	states, ok := s["state_machine"].([]any)
	if !ok || len(states) != 4 {
		t.Fatalf("state_machine=%v, want 4 items", s["state_machine"])
	}
	if s["policy_version"] != "vx18d.v1" {
		t.Fatalf("policy_version=%v, want vx18d.v1", s["policy_version"])
	}
	if s["mode"] != "vx_truth_boundary_states_surface" {
		t.Fatalf("mode=%v, want vx_truth_boundary_states_surface", s["mode"])
	}
}

// TestSeq18P153VXTruthBoundaryTest validates the VX truth-boundary test surface.
func TestSeq18P153VXTruthBoundaryTest(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p153","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vx_truth_boundary_test")
	if s["version"] != "seq18_p153.v1" {
		t.Fatalf("version=%v, want seq18_p153.v1", s["version"])
	}
	if s["role"] != "vx_truth_boundary_test" {
		t.Fatalf("role=%v, want vx_truth_boundary_test", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["test_file"] != "backend/test_step18_truth_boundary_replay_gate.py" {
		t.Fatalf("test_file=%v, want backend/test_step18_truth_boundary_replay_gate.py", s["test_file"])
	}
	guards, ok := s["guards"].([]any)
	if !ok || len(guards) != 4 {
		t.Fatalf("guards=%v, want 4 items", s["guards"])
	}
	if s["policy_version"] != "vx18d.v1" {
		t.Fatalf("policy_version=%v, want vx18d.v1", s["policy_version"])
	}
	if s["mode"] != "vx_truth_boundary_test_surface" {
		t.Fatalf("mode=%v, want vx_truth_boundary_test_surface", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-18 VX truncation_summary_loss gate tests (P160 ~ P164)
// ---------------------------------------------------------------------------

// TestSeq18P160VXTruncationSummaryLossGate validates the truncation_summary_loss
// gate surface.
func TestSeq18P160VXTruncationSummaryLossGate(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p160","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vx_truncation_summary_loss_gate")
	if s["version"] != "seq18_p160.v1" {
		t.Fatalf("version=%v, want seq18_p160.v1", s["version"])
	}
	if s["role"] != "vx_truncation_summary_loss_gate" {
		t.Fatalf("role=%v, want vx_truncation_summary_loss_gate", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["gate_version"] != "vx18e.v1" {
		t.Fatalf("gate_version=%v, want vx18e.v1", s["gate_version"])
	}
	if s["gate_name"] != "truncation_summary_loss" {
		t.Fatalf("gate_name=%v, want truncation_summary_loss", s["gate_name"])
	}
	if s["additive_only"] != true {
		t.Fatalf("additive_only=%v, want true", s["additive_only"])
	}
	if s["policy_version"] != "vx18e.v1" {
		t.Fatalf("policy_version=%v, want vx18e.v1", s["policy_version"])
	}
	if s["mode"] != "vx_truncation_summary_loss_gate_surface" {
		t.Fatalf("mode=%v, want vx_truncation_summary_loss_gate_surface", s["mode"])
	}
}

// TestSeq18P161VXTruncationSummaryLossMetrics validates the truncation_summary_loss
// metrics surface.
func TestSeq18P161VXTruncationSummaryLossMetrics(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p161","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vx_truncation_summary_loss_metrics")
	if s["version"] != "seq18_p161.v1" {
		t.Fatalf("version=%v, want seq18_p161.v1", s["version"])
	}
	if s["role"] != "vx_truncation_summary_loss_metrics" {
		t.Fatalf("role=%v, want vx_truncation_summary_loss_metrics", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	metrics, ok := s["metrics"].([]any)
	if !ok || len(metrics) != 4 {
		t.Fatalf("metrics=%v, want 4 items", s["metrics"])
	}
	if s["sample_sufficiency_required"] != true {
		t.Fatalf("sample_sufficiency_required=%v, want true", s["sample_sufficiency_required"])
	}
	trace, ok := s["trace_carry_in"].([]any)
	if !ok || len(trace) != 3 {
		t.Fatalf("trace_carry_in=%v, want 3 items", s["trace_carry_in"])
	}
	if s["policy_version"] != "vx18e.v1" {
		t.Fatalf("policy_version=%v, want vx18e.v1", s["policy_version"])
	}
	if s["mode"] != "vx_truncation_summary_loss_metrics_surface" {
		t.Fatalf("mode=%v, want vx_truncation_summary_loss_metrics_surface", s["mode"])
	}
}

// TestSeq18P162VXTruncationSummaryLossThresholdReuse validates the threshold reuse
// surface.
func TestSeq18P162VXTruncationSummaryLossThresholdReuse(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p162","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vx_truncation_summary_loss_threshold_reuse")
	if s["version"] != "seq18_p162.v1" {
		t.Fatalf("version=%v, want seq18_p162.v1", s["version"])
	}
	if s["role"] != "vx_truncation_summary_loss_threshold_reuse" {
		t.Fatalf("role=%v, want vx_truncation_summary_loss_threshold_reuse", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["sample_threshold_source"] != "_U1E_CAPTURED_REPLAY_MIN_*" {
		t.Fatalf("sample_threshold_source=%v, want _U1E_CAPTURED_REPLAY_MIN_*", s["sample_threshold_source"])
	}
	if s["threshold_owner"] != "gate_constant_block" {
		t.Fatalf("threshold_owner=%v, want gate_constant_block", s["threshold_owner"])
	}
	if s["scattered_literals_avoided"] != true {
		t.Fatalf("scattered_literals_avoided=%v, want true", s["scattered_literals_avoided"])
	}
	if s["policy_version"] != "vx18e.v1" {
		t.Fatalf("policy_version=%v, want vx18e.v1", s["policy_version"])
	}
	if s["mode"] != "vx_truncation_summary_loss_threshold_reuse_surface" {
		t.Fatalf("mode=%v, want vx_truncation_summary_loss_threshold_reuse_surface", s["mode"])
	}
}

// TestSeq18P163VXTruncationSummaryLossStates validates the state machine surface.
func TestSeq18P163VXTruncationSummaryLossStates(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p163","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vx_truncation_summary_loss_states")
	if s["version"] != "seq18_p163.v1" {
		t.Fatalf("version=%v, want seq18_p163.v1", s["version"])
	}
	if s["role"] != "vx_truncation_summary_loss_states" {
		t.Fatalf("role=%v, want vx_truncation_summary_loss_states", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	states, ok := s["state_machine"].([]any)
	if !ok || len(states) != 4 {
		t.Fatalf("state_machine=%v, want 4 items", s["state_machine"])
	}
	if s["policy_version"] != "vx18e.v1" {
		t.Fatalf("policy_version=%v, want vx18e.v1", s["policy_version"])
	}
	if s["mode"] != "vx_truncation_summary_loss_states_surface" {
		t.Fatalf("mode=%v, want vx_truncation_summary_loss_states_surface", s["mode"])
	}
}

// TestSeq18P164VXTruncationSummaryLossTest validates the combined regression bundle
// test surface.
func TestSeq18P164VXTruncationSummaryLossTest(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p164","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vx_truncation_summary_loss_test")
	if s["version"] != "seq18_p164.v1" {
		t.Fatalf("version=%v, want seq18_p164.v1", s["version"])
	}
	if s["role"] != "vx_truncation_summary_loss_test" {
		t.Fatalf("role=%v, want vx_truncation_summary_loss_test", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["test_file"] != "backend/test_step18_truncation_summary_loss_gate.py" {
		t.Fatalf("test_file=%v, want backend/test_step18_truncation_summary_loss_gate.py", s["test_file"])
	}
	guards, ok := s["guards"].([]any)
	if !ok || len(guards) != 4 {
		t.Fatalf("guards=%v, want 4 items", s["guards"])
	}
	if s["combined_bundle_status"] != "green" {
		t.Fatalf("combined_bundle_status=%v, want green", s["combined_bundle_status"])
	}
	components, ok := s["bundle_components"].([]any)
	if !ok || len(components) != 3 {
		t.Fatalf("bundle_components=%v, want 3 items", s["bundle_components"])
	}
	if s["policy_version"] != "vx18e.v1" {
		t.Fatalf("policy_version=%v, want vx18e.v1", s["policy_version"])
	}
	if s["mode"] != "vx_truncation_summary_loss_test_surface" {
		t.Fatalf("mode=%v, want vx_truncation_summary_loss_test_surface", s["mode"])
	}
}

// ---------------------------------------------------------------------------
// SEQ-18 Post-Chroma / pre-release / VR/HY/QR/VX summary tests (P327 ~ P369)
// ---------------------------------------------------------------------------

// TestSeq18P327PostChromaTop1ScopedVerbatim validates the Post-Chroma Top 1 summary.
func TestSeq18P327PostChromaTop1ScopedVerbatim(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p327","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "post_chroma_top1_scoped_verbatim")
	if s["version"] != "seq18_p327.v1" {
		t.Fatalf("version=%v, want seq18_p327.v1", s["version"])
	}
	if s["role"] != "post_chroma_top1_scoped_verbatim" {
		t.Fatalf("role=%v, want post_chroma_top1_scoped_verbatim", s["role"])
	}
	if s["top"] != float64(1) {
		t.Fatalf("top=%v, want 1", s["top"])
	}
	if s["policy_version"] != "vr18a.v1" {
		t.Fatalf("policy_version=%v, want vr18a.v1", s["policy_version"])
	}
	if s["mode"] != "post_chroma_summary_surface" {
		t.Fatalf("mode=%v, want post_chroma_summary_surface", s["mode"])
	}
}

// TestSeq18P328PostChromaTop2HybridScoring validates the Post-Chroma Top 2 summary.
func TestSeq18P328PostChromaTop2HybridScoring(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p328","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "post_chroma_top2_hybrid_scoring")
	if s["version"] != "seq18_p328.v1" {
		t.Fatalf("version=%v, want seq18_p328.v1", s["version"])
	}
	if s["role"] != "post_chroma_top2_hybrid_scoring" {
		t.Fatalf("role=%v, want post_chroma_top2_hybrid_scoring", s["role"])
	}
	if s["top"] != float64(2) {
		t.Fatalf("top=%v, want 2", s["top"])
	}
	if s["policy_version"] != "hy1a.v1" {
		t.Fatalf("policy_version=%v, want hy1a.v1", s["policy_version"])
	}
}

// TestSeq18P329PostChromaTop3TemporalRelation validates the Post-Chroma Top 3 summary.
func TestSeq18P329PostChromaTop3TemporalRelation(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p329","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "post_chroma_top3_temporal_relation")
	if s["version"] != "seq18_p329.v1" {
		t.Fatalf("version=%v, want seq18_p329.v1", s["version"])
	}
	if s["role"] != "post_chroma_top3_temporal_relation" {
		t.Fatalf("role=%v, want post_chroma_top3_temporal_relation", s["role"])
	}
	if s["top"] != float64(3) {
		t.Fatalf("top=%v, want 3", s["top"])
	}
	if s["policy_version"] != "qr1b.v1" {
		t.Fatalf("policy_version=%v, want qr1b.v1", s["policy_version"])
	}
}

// TestSeq18P330PostChromaTop4TemporalValidity validates the Post-Chroma Top 4 summary.
func TestSeq18P330PostChromaTop4TemporalValidity(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p330","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "post_chroma_top4_temporal_validity")
	if s["version"] != "seq18_p330.v1" {
		t.Fatalf("version=%v, want seq18_p330.v1", s["version"])
	}
	if s["role"] != "post_chroma_top4_temporal_validity" {
		t.Fatalf("role=%v, want post_chroma_top4_temporal_validity", s["role"])
	}
	if s["top"] != float64(4) {
		t.Fatalf("top=%v, want 4", s["top"])
	}
}

// TestSeq18P331PostChromaTop5EntityGraph validates the Post-Chroma Top 5 summary.
func TestSeq18P331PostChromaTop5EntityGraph(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p331","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "post_chroma_top5_entity_graph")
	if s["version"] != "seq18_p331.v1" {
		t.Fatalf("version=%v, want seq18_p331.v1", s["version"])
	}
	if s["role"] != "post_chroma_top5_entity_graph" {
		t.Fatalf("role=%v, want post_chroma_top5_entity_graph", s["role"])
	}
	if s["top"] != float64(5) {
		t.Fatalf("top=%v, want 5", s["top"])
	}
}

// TestSeq18P332PostChromaTop6SelectiveRerank validates the Post-Chroma Top 6 summary.
func TestSeq18P332PostChromaTop6SelectiveRerank(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p332","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "post_chroma_top6_selective_rerank")
	if s["version"] != "seq18_p332.v1" {
		t.Fatalf("version=%v, want seq18_p332.v1", s["version"])
	}
	if s["role"] != "post_chroma_top6_selective_rerank" {
		t.Fatalf("role=%v, want post_chroma_top6_selective_rerank", s["role"])
	}
	if s["top"] != float64(6) {
		t.Fatalf("top=%v, want 6", s["top"])
	}
}

// TestSeq18P336VRRawPreservingSupport validates the VR raw-preserving support summary.
func TestSeq18P336VRRawPreservingSupport(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p336","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vr_raw_preserving_support")
	if s["version"] != "seq18_p336.v1" {
		t.Fatalf("version=%v, want seq18_p336.v1", s["version"])
	}
	if s["role"] != "vr_raw_preserving_support" {
		t.Fatalf("role=%v, want vr_raw_preserving_support", s["role"])
	}
	if s["support_lane"] != "verbatim_recall" {
		t.Fatalf("support_lane=%v, want verbatim_recall", s["support_lane"])
	}
	if s["policy_version"] != "vr18a.v1" {
		t.Fatalf("policy_version=%v, want vr18a.v1", s["policy_version"])
	}
}

// TestSeq18P337VRHybridRealism validates the VR hybrid realism summary.
func TestSeq18P337VRHybridRealism(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p337","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vr_hybrid_realism")
	if s["version"] != "seq18_p337.v1" {
		t.Fatalf("version=%v, want seq18_p337.v1", s["version"])
	}
	if s["role"] != "vr_hybrid_realism" {
		t.Fatalf("role=%v, want vr_hybrid_realism", s["role"])
	}
	if s["policy_version"] != "hy1a.v1" {
		t.Fatalf("policy_version=%v, want hy1a.v1", s["policy_version"])
	}
}

// TestSeq18P338VRSoftRouting validates the VR soft routing summary.
func TestSeq18P338VRSoftRouting(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p338","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vr_soft_routing")
	if s["version"] != "seq18_p338.v1" {
		t.Fatalf("version=%v, want seq18_p338.v1", s["version"])
	}
	if s["role"] != "vr_soft_routing" {
		t.Fatalf("role=%v, want vr_soft_routing", s["role"])
	}
	if s["policy_version"] != "qr1a.v1" {
		t.Fatalf("policy_version=%v, want qr1a.v1", s["policy_version"])
	}
}

// TestSeq18P339VRLatencyDiscipline validates the VR latency discipline summary.
func TestSeq18P339VRLatencyDiscipline(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p339","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vr_latency_discipline")
	if s["version"] != "seq18_p339.v1" {
		t.Fatalf("version=%v, want seq18_p339.v1", s["version"])
	}
	if s["role"] != "vr_latency_discipline" {
		t.Fatalf("role=%v, want vr_latency_discipline", s["role"])
	}
	if s["policy_version"] != "qr1b.v1" {
		t.Fatalf("policy_version=%v, want qr1b.v1", s["policy_version"])
	}
}

// TestSeq18P340VRTruthBoundaryPreserve validates the VR truth-boundary preserve summary.
func TestSeq18P340VRTruthBoundaryPreserve(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p340","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vr_truth_boundary_preserve")
	if s["version"] != "seq18_p340.v1" {
		t.Fatalf("version=%v, want seq18_p340.v1", s["version"])
	}
	if s["role"] != "vr_truth_boundary_preserve" {
		t.Fatalf("role=%v, want vr_truth_boundary_preserve", s["role"])
	}
	if s["policy_version"] != "vx18d.v1" {
		t.Fatalf("policy_version=%v, want vx18d.v1", s["policy_version"])
	}
}

// TestSeq18P344VR18_1aRawTranscript validates the VR 18-1a raw transcript summary.
func TestSeq18P344VR18_1aRawTranscript(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p344","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vr_18_1a_raw_transcript")
	if s["version"] != "seq18_p344.v1" {
		t.Fatalf("version=%v, want seq18_p344.v1", s["version"])
	}
	if s["role"] != "vr_18_1a_raw_transcript" {
		t.Fatalf("role=%v, want vr_18_1a_raw_transcript", s["role"])
	}
	if s["sub_step"] != "18-1a" {
		t.Fatalf("sub_step=%v, want 18-1a", s["sub_step"])
	}
}

// TestSeq18P345VR18_1bSourceTag validates the VR 18-1b source-tag summary.
func TestSeq18P345VR18_1bSourceTag(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p345","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vr_18_1b_source_tag")
	if s["version"] != "seq18_p345.v1" {
		t.Fatalf("version=%v, want seq18_p345.v1", s["version"])
	}
	if s["role"] != "vr_18_1b_source_tag" {
		t.Fatalf("role=%v, want vr_18_1b_source_tag", s["role"])
	}
	if s["sub_step"] != "18-1b" {
		t.Fatalf("sub_step=%v, want 18-1b", s["sub_step"])
	}
}

// TestSeq18P346VR18_1cPromptInjection validates the VR 18-1c prompt injection summary.
func TestSeq18P346VR18_1cPromptInjection(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p346","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vr_18_1c_prompt_injection")
	if s["version"] != "seq18_p346.v1" {
		t.Fatalf("version=%v, want seq18_p346.v1", s["version"])
	}
	if s["role"] != "vr_18_1c_prompt_injection" {
		t.Fatalf("role=%v, want vr_18_1c_prompt_injection", s["role"])
	}
	if s["sub_step"] != "18-1c" {
		t.Fatalf("sub_step=%v, want 18-1c", s["sub_step"])
	}
}

// TestSeq18P347VR18_1dHierarchyEscape validates the VR 18-1d hierarchy escape summary.
func TestSeq18P347VR18_1dHierarchyEscape(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p347","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vr_18_1d_hierarchy_escape")
	if s["version"] != "seq18_p347.v1" {
		t.Fatalf("version=%v, want seq18_p347.v1", s["version"])
	}
	if s["role"] != "vr_18_1d_hierarchy_escape" {
		t.Fatalf("role=%v, want vr_18_1d_hierarchy_escape", s["role"])
	}
	if s["sub_step"] != "18-1d" {
		t.Fatalf("sub_step=%v, want 18-1d", s["sub_step"])
	}
}

// TestSeq18P351HY18_2aSemanticKeyword validates the HY 18-2a semantic+keyword summary.
func TestSeq18P351HY18_2aSemanticKeyword(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p351","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "hy_18_2a_semantic_keyword")
	if s["version"] != "seq18_p351.v1" {
		t.Fatalf("version=%v, want seq18_p351.v1", s["version"])
	}
	if s["role"] != "hy_18_2a_semantic_keyword" {
		t.Fatalf("role=%v, want hy_18_2a_semantic_keyword", s["role"])
	}
	if s["sub_step"] != "18-2a" {
		t.Fatalf("sub_step=%v, want 18-2a", s["sub_step"])
	}
}

// TestSeq18P352HY18_2bSoftBias validates the HY 18-2b soft bias summary.
func TestSeq18P352HY18_2bSoftBias(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p352","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "hy_18_2b_soft_bias")
	if s["version"] != "seq18_p352.v1" {
		t.Fatalf("version=%v, want seq18_p352.v1", s["version"])
	}
	if s["role"] != "hy_18_2b_soft_bias" {
		t.Fatalf("role=%v, want hy_18_2b_soft_bias", s["role"])
	}
	if s["sub_step"] != "18-2b" {
		t.Fatalf("sub_step=%v, want 18-2b", s["sub_step"])
	}
}

// TestSeq18P353HY18_2cScoreInspection validates the HY 18-2c score inspection summary.
func TestSeq18P353HY18_2cScoreInspection(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p353","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "hy_18_2c_score_inspection")
	if s["version"] != "seq18_p353.v1" {
		t.Fatalf("version=%v, want seq18_p353.v1", s["version"])
	}
	if s["role"] != "hy_18_2c_score_inspection" {
		t.Fatalf("role=%v, want hy_18_2c_score_inspection", s["role"])
	}
	if s["sub_step"] != "18-2c" {
		t.Fatalf("sub_step=%v, want 18-2c", s["sub_step"])
	}
}

// TestSeq18P354HY18_2dAdaptiveTopK validates the HY 18-2d adaptive top-k summary.
func TestSeq18P354HY18_2dAdaptiveTopK(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p354","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "hy_18_2d_adaptive_top_k")
	if s["version"] != "seq18_p354.v1" {
		t.Fatalf("version=%v, want seq18_p354.v1", s["version"])
	}
	if s["role"] != "hy_18_2d_adaptive_topk" {
		t.Fatalf("role=%v, want hy_18_2d_adaptive_topk", s["role"])
	}
	if s["sub_step"] != "18-2d" {
		t.Fatalf("sub_step=%v, want 18-2d", s["sub_step"])
	}
}

// TestSeq18P358QR18_3aQueryClass validates the QR 18-3a query class summary.
func TestSeq18P358QR18_3aQueryClass(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p358","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "qr_18_3a_query_class")
	if s["version"] != "seq18_p358.v1" {
		t.Fatalf("version=%v, want seq18_p358.v1", s["version"])
	}
	if s["role"] != "qr_18_3a_query_class" {
		t.Fatalf("role=%v, want qr_18_3a_query_class", s["role"])
	}
	if s["sub_step"] != "18-3a" {
		t.Fatalf("sub_step=%v, want 18-3a", s["sub_step"])
	}
}

// TestSeq18P359QR18_3bRetrievalDepth validates the QR 18-3b retrieval depth summary.
func TestSeq18P359QR18_3bRetrievalDepth(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p359","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "qr_18_3b_retrieval_depth")
	if s["version"] != "seq18_p359.v1" {
		t.Fatalf("version=%v, want seq18_p359.v1", s["version"])
	}
	if s["role"] != "qr_18_3b_retrieval_depth" {
		t.Fatalf("role=%v, want qr_18_3b_retrieval_depth", s["role"])
	}
	if s["sub_step"] != "18-3b" {
		t.Fatalf("sub_step=%v, want 18-3b", s["sub_step"])
	}
}

// TestSeq18P360QR18_3cExtractBeforeRead validates the QR 18-3c extract-before-read summary.
func TestSeq18P360QR18_3cExtractBeforeRead(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p360","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "qr_18_3c_extract_before_read")
	if s["version"] != "seq18_p360.v1" {
		t.Fatalf("version=%v, want seq18_p360.v1", s["version"])
	}
	if s["role"] != "qr_18_3c_extract_before_read" {
		t.Fatalf("role=%v, want qr_18_3c_extract_before_read", s["role"])
	}
	if s["sub_step"] != "18-3c" {
		t.Fatalf("sub_step=%v, want 18-3c", s["sub_step"])
	}
}

// TestSeq18P361QR18_3dLongTailRoute validates the QR 18-3d long-tail route summary.
func TestSeq18P361QR18_3dLongTailRoute(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p361","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "qr_18_3d_long_tail_route")
	if s["version"] != "seq18_p361.v1" {
		t.Fatalf("version=%v, want seq18_p361.v1", s["version"])
	}
	if s["role"] != "qr_18_3d_long_tail_route" {
		t.Fatalf("role=%v, want qr_18_3d_long_tail_route", s["role"])
	}
	if s["sub_step"] != "18-3d" {
		t.Fatalf("sub_step=%v, want 18-3d", s["sub_step"])
	}
}

// TestSeq18P365VX18_4aSemanticHybridReplay validates the VX 18-4a semantic hybrid replay summary.
func TestSeq18P365VX18_4aSemanticHybridReplay(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p365","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vx_18_4a_semantic_hybrid_replay")
	if s["version"] != "seq18_p365.v1" {
		t.Fatalf("version=%v, want seq18_p365.v1", s["version"])
	}
	if s["role"] != "vx_18_4a_semantic_hybrid_replay" {
		t.Fatalf("role=%v, want vx_18_4a_semantic_hybrid_replay", s["role"])
	}
	if s["sub_step"] != "18-4a" {
		t.Fatalf("sub_step=%v, want 18-4a", s["sub_step"])
	}
}

// TestSeq18P366VX18_4bHeldOutRecall validates the VX 18-4b held-out recall summary.
func TestSeq18P366VX18_4bHeldOutRecall(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p366","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vx_18_4b_held_out_recall")
	if s["version"] != "seq18_p366.v1" {
		t.Fatalf("version=%v, want seq18_p366.v1", s["version"])
	}
	if s["role"] != "vx_18_4b_held_out_recall" {
		t.Fatalf("role=%v, want vx_18_4b_held_out_recall", s["role"])
	}
	if s["sub_step"] != "18-4b" {
		t.Fatalf("sub_step=%v, want 18-4b", s["sub_step"])
	}
}

// TestSeq18P367VX18_4cLatencyToken validates the VX 18-4c latency token summary.
func TestSeq18P367VX18_4cLatencyToken(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p367","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vx_18_4c_latency_token")
	if s["version"] != "seq18_p367.v1" {
		t.Fatalf("version=%v, want seq18_p367.v1", s["version"])
	}
	if s["role"] != "vx_18_4c_latency_token" {
		t.Fatalf("role=%v, want vx_18_4c_latency_token", s["role"])
	}
	if s["sub_step"] != "18-4c" {
		t.Fatalf("sub_step=%v, want 18-4c", s["sub_step"])
	}
}

// TestSeq18P368VX18_4dTruthBoundaryReplay validates the VX 18-4d truth-boundary replay summary.
func TestSeq18P368VX18_4dTruthBoundaryReplay(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p368","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vx_18_4d_truth_boundary_replay")
	if s["version"] != "seq18_p368.v1" {
		t.Fatalf("version=%v, want seq18_p368.v1", s["version"])
	}
	if s["role"] != "vx_18_4d_truth_boundary_replay" {
		t.Fatalf("role=%v, want vx_18_4d_truth_boundary_replay", s["role"])
	}
	if s["sub_step"] != "18-4d" {
		t.Fatalf("sub_step=%v, want 18-4d", s["sub_step"])
	}
}

// TestSeq18P369VX18_4eTopKTruncation validates the VX 18-4e top-k truncation summary.
func TestSeq18P369VX18_4eTopKTruncation(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p369","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "vx_18_4e_top_k_truncation")
	if s["version"] != "seq18_p369.v1" {
		t.Fatalf("version=%v, want seq18_p369.v1", s["version"])
	}
	if s["role"] != "vx_18_4e_topk_truncation" {
		t.Fatalf("role=%v, want vx_18_4e_topk_truncation", s["role"])
	}
	if s["sub_step"] != "18-4e" {
		t.Fatalf("sub_step=%v, want 18-4e", s["sub_step"])
	}
}

// TestSeq18P373PreReleaseVersionMarker validates the pre-release version marker summary.
func TestSeq18P373PreReleaseVersionMarker(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p373","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "pre_release_version_marker")
	if s["version"] != "seq18_p373.v1" {
		t.Fatalf("version=%v, want seq18_p373.v1", s["version"])
	}
	if s["role"] != "pre_release_version_marker" {
		t.Fatalf("role=%v, want pre_release_version_marker", s["role"])
	}
	if s["marker_file"] != "1.0.0-pre" {
		t.Fatalf("marker_file=%v, want 1.0.0-pre", s["marker_file"])
	}
	if s["promotion"] != true {
		t.Fatalf("promotion=%v, want true", s["promotion"])
	}
}

// TestSeq18P374PreReleaseBundleAuthority validates the pre-release bundle authority summary.
func TestSeq18P374PreReleaseBundleAuthority(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p374","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "pre_release_bundle_authority")
	if s["version"] != "seq18_p374.v1" {
		t.Fatalf("version=%v, want seq18_p374.v1", s["version"])
	}
	if s["role"] != "pre_release_bundle_authority" {
		t.Fatalf("role=%v, want pre_release_bundle_authority", s["role"])
	}
	if s["bundle_name"] != "Archive Center Pre-release 1.0.0" {
		t.Fatalf("bundle_name=%v, want Archive Center Pre-release 1.0.0", s["bundle_name"])
	}
}

// TestSeq18P375PreReleaseArtifact validates the pre-release artifact summary.
func TestSeq18P375PreReleaseArtifact(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p375","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "pre_release_artifact")
	if s["version"] != "seq18_p375.v1" {
		t.Fatalf("version=%v, want seq18_p375.v1", s["version"])
	}
	if s["role"] != "pre_release_artifact" {
		t.Fatalf("role=%v, want pre_release_artifact", s["role"])
	}
	if s["validated"] != true {
		t.Fatalf("validated=%v, want true", s["validated"])
	}
}

// TestSeq18P376PreReleaseVRSmoke validates the pre-release VR smoke summary.
func TestSeq18P376PreReleaseVRSmoke(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p376","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "pre_release_vr_smoke")
	if s["version"] != "seq18_p376.v1" {
		t.Fatalf("version=%v, want seq18_p376.v1", s["version"])
	}
	if s["role"] != "pre_release_vr_smoke" {
		t.Fatalf("role=%v, want pre_release_vr_smoke", s["role"])
	}
	if s["status"] != "pass" {
		t.Fatalf("status=%v, want pass", s["status"])
	}
}

// TestSeq18P377PreReleaseHYSmoke validates the pre-release HY smoke summary.
func TestSeq18P377PreReleaseHYSmoke(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p377","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "pre_release_hy_smoke")
	if s["version"] != "seq18_p377.v1" {
		t.Fatalf("version=%v, want seq18_p377.v1", s["version"])
	}
	if s["role"] != "pre_release_hy_smoke" {
		t.Fatalf("role=%v, want pre_release_hy_smoke", s["role"])
	}
	if s["status"] != "pass" {
		t.Fatalf("status=%v, want pass", s["status"])
	}
}

// TestSeq18P378PreReleaseQRSmoke validates the pre-release QR smoke summary.
func TestSeq18P378PreReleaseQRSmoke(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p378","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "pre_release_qr_smoke")
	if s["version"] != "seq18_p378.v1" {
		t.Fatalf("version=%v, want seq18_p378.v1", s["version"])
	}
	if s["role"] != "pre_release_qr_smoke" {
		t.Fatalf("role=%v, want pre_release_qr_smoke", s["role"])
	}
	if s["status"] != "pass" {
		t.Fatalf("status=%v, want pass", s["status"])
	}
}

// TestSeq18P379PreReleaseVXReview validates the pre-release VX review checklist summary.
func TestSeq18P379PreReleaseVXReview(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p379","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "pre_release_vx_review")
	if s["version"] != "seq18_p379.v1" {
		t.Fatalf("version=%v, want seq18_p379.v1", s["version"])
	}
	if s["role"] != "pre_release_vx_review" {
		t.Fatalf("role=%v, want pre_release_vx_review", s["role"])
	}
	if s["status"] != "pass" {
		t.Fatalf("status=%v, want pass", s["status"])
	}
	components, ok := s["components"].([]any)
	if !ok || len(components) != 4 {
		t.Fatalf("components=%v, want 4 items", s["components"])
	}
}

// TestSeq18P391PreReleaseRawSnippet validates the pre-release raw snippet summary.
func TestSeq18P391PreReleaseRawSnippet(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p391","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "pre_release_raw_snippet")
	if s["version"] != "seq18_p391.v1" {
		t.Fatalf("version=%v, want seq18_p391.v1", s["version"])
	}
	if s["role"] != "pre_release_raw_snippet" {
		t.Fatalf("role=%v, want pre_release_raw_snippet", s["role"])
	}
	if s["snippet_count"] != float64(3) {
		t.Fatalf("snippet_count=%v, want 3", s["snippet_count"])
	}
	if s["max_chars"] != float64(720) {
		t.Fatalf("max_chars=%v, want 720", s["max_chars"])
	}
	if s["excerpt_chars"] != float64(160) {
		t.Fatalf("excerpt_chars=%v, want 160", s["excerpt_chars"])
	}
}

// TestSeq18P392PreReleaseHybridBias validates the pre-release hybrid bias summary.
func TestSeq18P392PreReleaseHybridBias(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p392","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "pre_release_hybrid_bias")
	if s["version"] != "seq18_p392.v1" {
		t.Fatalf("version=%v, want seq18_p392.v1", s["version"])
	}
	if s["role"] != "pre_release_hybrid_bias" {
		t.Fatalf("role=%v, want pre_release_hybrid_bias", s["role"])
	}
	if s["speaker"] != float64(0.04) {
		t.Fatalf("speaker=%v, want 0.04", s["speaker"])
	}
	if s["location"] != float64(0.05) {
		t.Fatalf("location=%v, want 0.05", s["location"])
	}
	if s["storyline"] != float64(0.06) {
		t.Fatalf("storyline=%v, want 0.06", s["storyline"])
	}
	if s["cap"] != float64(0.12) {
		t.Fatalf("cap=%v, want 0.12", s["cap"])
	}
}

// TestSeq18P393PreReleaseQueryClassRule validates the pre-release query class rule summary.
func TestSeq18P393PreReleaseQueryClassRule(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p393","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "pre_release_query_class_rule")
	if s["version"] != "seq18_p393.v1" {
		t.Fatalf("version=%v, want seq18_p393.v1", s["version"])
	}
	if s["role"] != "pre_release_query_class_rule" {
		t.Fatalf("role=%v, want pre_release_query_class_rule", s["role"])
	}
	if s["heuristic"] != "rule_first_additive_contract" {
		t.Fatalf("heuristic=%v, want rule_first_additive_contract", s["heuristic"])
	}
	if s["execution"] != "fail_open_shared_execution" {
		t.Fatalf("execution=%v, want fail_open_shared_execution", s["execution"])
	}
}

// TestSeq18P394PreReleaseRetrievalNote validates the pre-release retrieval note summary.
func TestSeq18P394PreReleaseRetrievalNote(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq18-p394","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "pre_release_retrieval_note")
	if s["version"] != "seq18_p394.v1" {
		t.Fatalf("version=%v, want seq18_p394.v1", s["version"])
	}
	if s["role"] != "pre_release_retrieval_note" {
		t.Fatalf("role=%v, want pre_release_retrieval_note", s["role"])
	}
	defaults, ok := s["defaults"].(map[string]any)
	if !ok {
		t.Fatalf("defaults missing or not map")
	}
	if defaults["support_surface_first"] != true {
		t.Fatalf("support_surface_first=%v, want true", defaults["support_surface_first"])
	}
	if defaults["scene_canon_no_extract"] != true {
		t.Fatalf("scene_canon_no_extract=%v, want true", defaults["scene_canon_no_extract"])
	}
	if defaults["callback_resume_temporal_note_only"] != true {
		t.Fatalf("callback_resume_temporal_note_only=%v, want true", defaults["callback_resume_temporal_note_only"])
	}
}
