package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
