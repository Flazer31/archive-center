package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSeq168P176CarryInStep18HybridScoring validates that Step 16.8 stale arc
// ceiling and scene alignment are present as baseline for Step 18 hybrid
// scoring without redefining them.
// SEQ-16.8-P176: Step 18 hybrid scoring stale callback ceiling / current-scene alignment baseline.
func TestSeq168P176CarryInStep18HybridScoring(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p176","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	ceiling := seq165Map(t, resp, "stale_arc_ceiling")
	if ceiling["version"] != "seq16_8_p99.v1" {
		t.Fatalf("stale_arc_ceiling version=%v, want seq16_8_p99.v1", ceiling["version"])
	}
	if ceiling["carry_in_baseline_for_step_18_hybrid_scoring"] != true {
		t.Fatalf("stale_arc_ceiling carry_in_baseline_for_step_18_hybrid_scoring=%v, want true", ceiling["carry_in_baseline_for_step_18_hybrid_scoring"])
	}
	if ceiling["baseline_source"] != "seq16_8_stale_arc_ceiling" {
		t.Fatalf("stale_arc_ceiling baseline_source=%v, want seq16_8_stale_arc_ceiling", ceiling["baseline_source"])
	}
	if ceiling["opens_step_18_hybrid_scoring"] != true {
		t.Fatalf("stale_arc_ceiling opens_step_18_hybrid_scoring=%v, want true", ceiling["opens_step_18_hybrid_scoring"])
	}
	alignment := seq165Map(t, resp, "scene_alignment")
	if alignment["version"] != "seq16_8_p100.v1" {
		t.Fatalf("scene_alignment version=%v, want seq16_8_p100.v1", alignment["version"])
	}
	if alignment["carry_in_baseline_for_step_18_hybrid_scoring"] != true {
		t.Fatalf("scene_alignment carry_in_baseline_for_step_18_hybrid_scoring=%v, want true", alignment["carry_in_baseline_for_step_18_hybrid_scoring"])
	}
	if alignment["baseline_source"] != "seq16_8_current_scene_alignment" {
		t.Fatalf("scene_alignment baseline_source=%v, want seq16_8_current_scene_alignment", alignment["baseline_source"])
	}
	if alignment["opens_step_18_hybrid_scoring"] != true {
		t.Fatalf("scene_alignment opens_step_18_hybrid_scoring=%v, want true", alignment["opens_step_18_hybrid_scoring"])
	}
}

// TestSeq168P177CarryInStep20SelectiveRerank validates that Step 16.8 stale
// callback suppression and foreground hijack taxonomy are present as baseline
// for Step 20 selective rerank without redefining them.
// SEQ-16.8-P177: Step 20 selective rerank stale callback suppression trigger /
// monopoly failure taxonomy baseline.
func TestSeq168P177CarryInStep20SelectiveRerank(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p177","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	suppression := seq165Map(t, resp, "stale_callback_suppression")
	if suppression["version"] != "seq16_8_p109.v1" {
		t.Fatalf("stale_callback_suppression version=%v, want seq16_8_p109.v1", suppression["version"])
	}
	if suppression["carry_in_baseline_for_step_20_selective_rerank"] != true {
		t.Fatalf("stale_callback_suppression carry_in_baseline_for_step_20_selective_rerank=%v, want true", suppression["carry_in_baseline_for_step_20_selective_rerank"])
	}
	if suppression["baseline_source"] != "seq16_8_stale_callback_suppression" {
		t.Fatalf("stale_callback_suppression baseline_source=%v, want seq16_8_stale_callback_suppression", suppression["baseline_source"])
	}
	if suppression["redefines_step_20_selective_rerank"] != false {
		t.Fatalf("stale_callback_suppression redefines_step_20_selective_rerank=%v, want false", suppression["redefines_step_20_selective_rerank"])
	}
	taxonomy := seq165Map(t, resp, "foreground_hijack_taxonomy")
	if taxonomy["version"] != "seq16_8_p119.v1" {
		t.Fatalf("foreground_hijack_taxonomy version=%v, want seq16_8_p119.v1", taxonomy["version"])
	}
	if taxonomy["carry_in_baseline_for_step_20_selective_rerank"] != true {
		t.Fatalf("foreground_hijack_taxonomy carry_in_baseline_for_step_20_selective_rerank=%v, want true", taxonomy["carry_in_baseline_for_step_20_selective_rerank"])
	}
	if taxonomy["baseline_source"] != "seq16_8_monopoly_failure_taxonomy" {
		t.Fatalf("foreground_hijack_taxonomy baseline_source=%v, want seq16_8_monopoly_failure_taxonomy", taxonomy["baseline_source"])
	}
	if taxonomy["redefines_step_20_selective_rerank"] != false {
		t.Fatalf("foreground_hijack_taxonomy redefines_step_20_selective_rerank=%v, want false", taxonomy["redefines_step_20_selective_rerank"])
	}
}

// TestSeq168P178CarryInLaterStepRecallRerank validates that Step 16.8 recall
// gain / monopoly cost split is present as baseline trace for later-step recall
// / rerank gain without redefining it.
// SEQ-16.8-P178: later-step recall / rerank gain foreground monopoly cost baseline trace.
func TestSeq168P178CarryInLaterStepRecallRerank(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq168-p178","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	split := seq165Map(t, resp, "recall_gain_monopoly_split")
	if split["version"] != "seq16_8_p121.v1" {
		t.Fatalf("version=%v, want seq16_8_p121.v1", split["version"])
	}
	if split["role"] != "recall_gain_monopoly_split" {
		t.Fatalf("role=%v, want recall_gain_monopoly_split", split["role"])
	}
	if split["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", split["truth_authority"])
	}
	if split["carry_in_baseline_for_later_step_recall_rerank"] != true {
		t.Fatalf("carry_in_baseline_for_later_step_recall_rerank=%v, want true", split["carry_in_baseline_for_later_step_recall_rerank"])
	}
	if split["baseline_source"] != "seq16_8_recall_gain_monopoly_split" {
		t.Fatalf("baseline_source=%v, want seq16_8_recall_gain_monopoly_split", split["baseline_source"])
	}
	sharedWith, ok := split["shared_with"].([]any)
	if !ok || len(sharedWith) == 0 {
		t.Fatalf("shared_with missing or empty")
	}
	sharedSeen := map[string]bool{}
	for _, raw := range sharedWith {
		if s, ok := raw.(string); ok {
			sharedSeen[s] = true
		}
	}
	for _, name := range []string{"later_step_recall", "later_step_rerank"} {
		if !sharedSeen[name] {
			t.Fatalf("shared_with missing %q: %#v", name, sharedWith)
		}
	}
	if split["mode"] != "recall_gain_monopoly_cost_split_trace_schema" {
		t.Fatalf("mode=%v, want recall_gain_monopoly_cost_split_trace_schema", split["mode"])
	}
}

// TestSeq17P387BundleGenerationEvidence validates the bundle generation
// evidence contract surface for SEQ-17-P387: Archive Center Beta 0.8 bundle
// latest root runtime create/generate. This is read-only evidence, not actual
// bundle generation.
func TestSeq17P387BundleGenerationEvidence(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p387","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	evidence := seq165Map(t, resp, "bundle_generation_evidence")
	if evidence["version"] != "seq17_p387.v1" {
		t.Fatalf("version=%v, want seq17_p387.v1", evidence["version"])
	}
	if evidence["role"] != "bundle_generation_evidence" {
		t.Fatalf("role=%v, want bundle_generation_evidence", evidence["role"])
	}
	if evidence["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", evidence["truth_authority"])
	}
	if evidence["bundle_target"] != "Archive Center Beta 0.8" {
		t.Fatalf("bundle_target=%v, want Archive Center Beta 0.8", evidence["bundle_target"])
	}
	if evidence["evidence_only"] != true {
		t.Fatalf("evidence_only=%v, want true", evidence["evidence_only"])
	}
	if evidence["artifact_created"] != false {
		t.Fatalf("artifact_created=%v, want false", evidence["artifact_created"])
	}
	if evidence["release_artifact_created"] != false {
		t.Fatalf("release_artifact_created=%v, want false", evidence["release_artifact_created"])
	}
	if evidence["beta_reference_mutated"] != false {
		t.Fatalf("beta_reference_mutated=%v, want false", evidence["beta_reference_mutated"])
	}
	if evidence["bundle_generation_mode"] != "evidence_only_no_artifact" {
		t.Fatalf("bundle_generation_mode=%v, want evidence_only_no_artifact", evidence["bundle_generation_mode"])
	}
	if evidence["mode"] != "bundle_generation_evidence_contract" {
		t.Fatalf("mode=%v, want bundle_generation_evidence_contract", evidence["mode"])
	}
}

// TestSeq17P388RegressionCorpusGreen validates the Step 14~16 regression
// corpus green gate surface for SEQ-17-P388.
func TestSeq17P388RegressionCorpusGreen(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p388","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	gate := seq165Map(t, resp, "regression_corpus_green")
	if gate["version"] != "seq17_p388.v1" {
		t.Fatalf("version=%v, want seq17_p388.v1", gate["version"])
	}
	if gate["role"] != "regression_corpus_green" {
		t.Fatalf("role=%v, want regression_corpus_green", gate["role"])
	}
	if gate["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", gate["truth_authority"])
	}
	if gate["step_14_status"] != "green" {
		t.Fatalf("step_14_status=%v, want green", gate["step_14_status"])
	}
	if gate["step_15_status"] != "green" {
		t.Fatalf("step_15_status=%v, want green", gate["step_15_status"])
	}
	if gate["step_16_status"] != "green" {
		t.Fatalf("step_16_status=%v, want green", gate["step_16_status"])
	}
	if gate["all_steps_green"] != true {
		t.Fatalf("all_steps_green=%v, want true", gate["all_steps_green"])
	}
	if gate["regression_corpus_source"] != "step_14_16_regression_corpus_contract" {
		t.Fatalf("regression_corpus_source=%v, want step_14_16_regression_corpus_contract", gate["regression_corpus_source"])
	}
	if gate["evidence_contract_only"] != true {
		t.Fatalf("evidence_contract_only=%v, want true", gate["evidence_contract_only"])
	}
	if gate["operator_execution_claim"] != false {
		t.Fatalf("operator_execution_claim=%v, want false", gate["operator_execution_claim"])
	}
	if gate["mode"] != "regression_corpus_green_gate" {
		t.Fatalf("mode=%v, want regression_corpus_green_gate", gate["mode"])
	}
}

// TestSeq17P389EvaluationSplitSmokeCheck validates the evaluation split
// completeness/answer-quality smoke check pass surface for SEQ-17-P389.
func TestSeq17P389EvaluationSplitSmokeCheck(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p389","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	check := seq165Map(t, resp, "evaluation_split_smoke_check")
	if check["version"] != "seq17_p389.v1" {
		t.Fatalf("version=%v, want seq17_p389.v1", check["version"])
	}
	if check["role"] != "evaluation_split_smoke_check" {
		t.Fatalf("role=%v, want evaluation_split_smoke_check", check["role"])
	}
	if check["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", check["truth_authority"])
	}
	if check["metric_split"] != "retrieval_completeness_vs_final_answer_quality" {
		t.Fatalf("metric_split=%v, want retrieval_completeness_vs_final_answer_quality", check["metric_split"])
	}
	if check["completeness_check"] != "pass" {
		t.Fatalf("completeness_check=%v, want pass", check["completeness_check"])
	}
	if check["answer_quality_check"] != "pass" {
		t.Fatalf("answer_quality_check=%v, want pass", check["answer_quality_check"])
	}
	if check["smoke_check_pass"] != true {
		t.Fatalf("smoke_check_pass=%v, want true", check["smoke_check_pass"])
	}
	if check["source_metric"] != "lc1p_evaluation_split" {
		t.Fatalf("source_metric=%v, want lc1p_evaluation_split", check["source_metric"])
	}
	if check["evidence_contract_only"] != true {
		t.Fatalf("evidence_contract_only=%v, want true", check["evidence_contract_only"])
	}
	if check["operator_execution_claim"] != false {
		t.Fatalf("operator_execution_claim=%v, want false", check["operator_execution_claim"])
	}
	if check["mode"] != "evaluation_split_smoke_check_pass" {
		t.Fatalf("mode=%v, want evaluation_split_smoke_check_pass", check["mode"])
	}
}

// TestSeq17P390OpsDryRunChecklistPass validates the ops procedure dry-run
// checklist pass surface for SEQ-17-P390.
func TestSeq17P390OpsDryRunChecklistPass(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p390","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	checklist := seq165Map(t, resp, "ops_dry_run_checklist_pass")
	if checklist["version"] != "seq17_p390.v1" {
		t.Fatalf("version=%v, want seq17_p390.v1", checklist["version"])
	}
	if checklist["role"] != "ops_dry_run_checklist_pass" {
		t.Fatalf("role=%v, want ops_dry_run_checklist_pass", checklist["role"])
	}
	if checklist["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", checklist["truth_authority"])
	}
	if checklist["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", checklist["dry_run_only"])
	}
	if checklist["actual_ops_run"] != false {
		t.Fatalf("actual_ops_run=%v, want false", checklist["actual_ops_run"])
	}
	if checklist["all_pass"] != true {
		t.Fatalf("all_pass=%v, want true", checklist["all_pass"])
	}
	items, ok := checklist["dry_run_checklist"].([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("dry_run_checklist missing or empty")
	}
	if checklist["mode"] != "ops_procedure_dry_run_checklist_pass" {
		t.Fatalf("mode=%v, want ops_procedure_dry_run_checklist_pass", checklist["mode"])
	}
}

// TestSeq17P391InspectionLaneBoundaryReview validates the inspection surface
// lane-boundary review checklist pass surface for SEQ-17-P391.
func TestSeq17P391InspectionLaneBoundaryReview(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p391","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	review := seq165Map(t, resp, "inspection_lane_boundary_review")
	if review["version"] != "seq17_p391.v1" {
		t.Fatalf("version=%v, want seq17_p391.v1", review["version"])
	}
	if review["role"] != "inspection_lane_boundary_review" {
		t.Fatalf("role=%v, want inspection_lane_boundary_review", review["role"])
	}
	if review["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", review["truth_authority"])
	}
	if review["read_only_inspection_surface"] != true {
		t.Fatalf("read_only_inspection_surface=%v, want true", review["read_only_inspection_surface"])
	}
	if review["authority_display_guard"] != true {
		t.Fatalf("authority_display_guard=%v, want true", review["authority_display_guard"])
	}
	if review["explain_surface"] != "pass" {
		t.Fatalf("explain_surface=%v, want pass", review["explain_surface"])
	}
	if review["preview_audit_surface"] != "pass" {
		t.Fatalf("preview_audit_surface=%v, want pass", review["preview_audit_surface"])
	}
	if review["dashboard_lane"] != "pass" {
		t.Fatalf("dashboard_lane=%v, want pass", review["dashboard_lane"])
	}
	if review["display_guard"] != "pass" {
		t.Fatalf("display_guard=%v, want pass", review["display_guard"])
	}
	if review["visibility_lane"] != "pass" {
		t.Fatalf("visibility_lane=%v, want pass", review["visibility_lane"])
	}
	if review["all_pass"] != true {
		t.Fatalf("all_pass=%v, want true", review["all_pass"])
	}
	if review["mode"] != "inspection_surface_lane_boundary_review_pass" {
		t.Fatalf("mode=%v, want inspection_surface_lane_boundary_review_pass", review["mode"])
	}
}

// TestSeq17P392ReleaseGateComplete validates the adoption gate / release note /
// bundle checklist complete surface for SEQ-17-P392.
func TestSeq17P392ReleaseGateComplete(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p392","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	gate := seq165Map(t, resp, "release_gate_complete")
	if gate["version"] != "seq17_p392.v1" {
		t.Fatalf("version=%v, want seq17_p392.v1", gate["version"])
	}
	if gate["role"] != "release_gate_complete" {
		t.Fatalf("role=%v, want release_gate_complete", gate["role"])
	}
	if gate["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", gate["truth_authority"])
	}
	if gate["sync_scope"] != "evidence_contract_only" {
		t.Fatalf("sync_scope=%v, want evidence_contract_only", gate["sync_scope"])
	}
	if gate["release_execution"] != false {
		t.Fatalf("release_execution=%v, want false", gate["release_execution"])
	}
	if gate["artifact_created"] != false {
		t.Fatalf("artifact_created=%v, want false", gate["artifact_created"])
	}
	if gate["adoption_default_changed"] != false {
		t.Fatalf("adoption_default_changed=%v, want false", gate["adoption_default_changed"])
	}
	if gate["adoption_gate_sync"] != "complete" {
		t.Fatalf("adoption_gate_sync=%v, want complete", gate["adoption_gate_sync"])
	}
	if gate["release_note_sync"] != "complete" {
		t.Fatalf("release_note_sync=%v, want complete", gate["release_note_sync"])
	}
	if gate["bundle_checklist_sync"] != "complete" {
		t.Fatalf("bundle_checklist_sync=%v, want complete", gate["bundle_checklist_sync"])
	}
	if gate["all_complete"] != true {
		t.Fatalf("all_complete=%v, want true", gate["all_complete"])
	}
	if gate["mode"] != "adoption_gate_release_note_bundle_checklist_complete" {
		t.Fatalf("mode=%v, want adoption_gate_release_note_bundle_checklist_complete", gate["mode"])
	}
}

// TestSeq17P396ReauditBackendAdminOwner validates the backend/admin release-gate
// owner closure surface for SEQ-17-P396.
func TestSeq17P396ReauditBackendAdminOwner(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p396","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	owner := seq165Map(t, resp, "reaudit_backend_admin_owner")
	if owner["version"] != "seq17_p396.v1" {
		t.Fatalf("version=%v, want seq17_p396.v1", owner["version"])
	}
	if owner["role"] != "reaudit_backend_admin_owner" {
		t.Fatalf("role=%v, want reaudit_backend_admin_owner", owner["role"])
	}
	if owner["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", owner["truth_authority"])
	}
	if owner["owner_closed"] != true {
		t.Fatalf("owner_closed=%v, want true", owner["owner_closed"])
	}
	if owner["mode"] != "reaudit_backend_admin_owner_closed" {
		t.Fatalf("mode=%v, want reaudit_backend_admin_owner_closed", owner["mode"])
	}
}

// TestSeq17P397ReauditOpsDocDryRun validates the ops documentation dry-run
// checklist closure surface for SEQ-17-P397.
func TestSeq17P397ReauditOpsDocDryRun(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p397","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	doc := seq165Map(t, resp, "reaudit_ops_doc_dry_run")
	if doc["version"] != "seq17_p397.v1" {
		t.Fatalf("version=%v, want seq17_p397.v1", doc["version"])
	}
	if doc["role"] != "reaudit_ops_doc_dry_run" {
		t.Fatalf("role=%v, want reaudit_ops_doc_dry_run", doc["role"])
	}
	if doc["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", doc["truth_authority"])
	}
	if doc["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", doc["dry_run_only"])
	}
	if doc["actual_ops_run"] != false {
		t.Fatalf("actual_ops_run=%v, want false", doc["actual_ops_run"])
	}
	if doc["owner_closed"] != true {
		t.Fatalf("owner_closed=%v, want true", doc["owner_closed"])
	}
	if doc["mode"] != "reaudit_ops_doc_dry_run_closed" {
		t.Fatalf("mode=%v, want reaudit_ops_doc_dry_run_closed", doc["mode"])
	}
}

// TestSeq17P398ReauditRootRuntimeReadOnly validates the root runtime read-only
// inspection/gate surface closure for SEQ-17-P398.
func TestSeq17P398ReauditRootRuntimeReadOnly(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p398","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	rt := seq165Map(t, resp, "reaudit_root_runtime_read_only")
	if rt["version"] != "seq17_p398.v1" {
		t.Fatalf("version=%v, want seq17_p398.v1", rt["version"])
	}
	if rt["role"] != "reaudit_root_runtime_read_only" {
		t.Fatalf("role=%v, want reaudit_root_runtime_read_only", rt["role"])
	}
	if rt["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", rt["truth_authority"])
	}
	if rt["read_only_surface"] != true {
		t.Fatalf("read_only_surface=%v, want true", rt["read_only_surface"])
	}
	if rt["owner_closed"] != true {
		t.Fatalf("owner_closed=%v, want true", rt["owner_closed"])
	}
	if rt["mode"] != "reaudit_root_runtime_read_only_closed" {
		t.Fatalf("mode=%v, want reaudit_root_runtime_read_only_closed", rt["mode"])
	}
}

// TestSeq17P399ReauditReleaseGateOperatorEvidence validates the release gate
// operator evidence closure surface for SEQ-17-P399.
func TestSeq17P399ReauditReleaseGateOperatorEvidence(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p399","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	ev := seq165Map(t, resp, "reaudit_release_gate_operator_evidence")
	if ev["version"] != "seq17_p399.v1" {
		t.Fatalf("version=%v, want seq17_p399.v1", ev["version"])
	}
	if ev["role"] != "reaudit_release_gate_operator_evidence" {
		t.Fatalf("role=%v, want reaudit_release_gate_operator_evidence", ev["role"])
	}
	if ev["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", ev["truth_authority"])
	}
	if ev["operator_evidence"] != true {
		t.Fatalf("operator_evidence=%v, want true", ev["operator_evidence"])
	}
	if ev["operator_evidence_mode"] != "contract_included_not_supplied" {
		t.Fatalf("operator_evidence_mode=%v, want contract_included_not_supplied", ev["operator_evidence_mode"])
	}
	if ev["bundle_regenerate_sync"] != "complete" {
		t.Fatalf("bundle_regenerate_sync=%v, want complete", ev["bundle_regenerate_sync"])
	}
	if ev["release_note_sync"] != "complete" {
		t.Fatalf("release_note_sync=%v, want complete", ev["release_note_sync"])
	}
	if ev["known_risk_ledger_sync"] != "complete" {
		t.Fatalf("known_risk_ledger_sync=%v, want complete", ev["known_risk_ledger_sync"])
	}
	if ev["artifact_created"] != false {
		t.Fatalf("artifact_created=%v, want false", ev["artifact_created"])
	}
	if ev["release_execution"] != false {
		t.Fatalf("release_execution=%v, want false", ev["release_execution"])
	}
	if ev["all_closed"] != true {
		t.Fatalf("all_closed=%v, want true", ev["all_closed"])
	}
	if ev["mode"] != "reaudit_release_gate_operator_evidence_closed" {
		t.Fatalf("mode=%v, want reaudit_release_gate_operator_evidence_closed", ev["mode"])
	}
}

// TestSeq17P400ReauditAdminMutationControlUI validates the admin mutation/control
// UI boundary surface for SEQ-17-P400. Must be operator_required,
// execution_disabled, read_only, artifact_created:false, beta_reference_mutated:false.
func TestSeq17P400ReauditAdminMutationControlUI(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p400","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	ui := seq165Map(t, resp, "reaudit_admin_mutation_control_ui")
	if ui["version"] != "seq17_p400.v1" {
		t.Fatalf("version=%v, want seq17_p400.v1", ui["version"])
	}
	if ui["role"] != "reaudit_admin_mutation_control_ui" {
		t.Fatalf("role=%v, want reaudit_admin_mutation_control_ui", ui["role"])
	}
	if ui["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", ui["truth_authority"])
	}
	if ui["operator_required"] != true {
		t.Fatalf("operator_required=%v, want true", ui["operator_required"])
	}
	if ui["execution_disabled"] != true {
		t.Fatalf("execution_disabled=%v, want true", ui["execution_disabled"])
	}
	if ui["read_only"] != true {
		t.Fatalf("read_only=%v, want true", ui["read_only"])
	}
	if ui["artifact_created"] != false {
		t.Fatalf("artifact_created=%v, want false", ui["artifact_created"])
	}
	if ui["beta_reference_mutated"] != false {
		t.Fatalf("beta_reference_mutated=%v, want false", ui["beta_reference_mutated"])
	}
	if ui["ui_exists"] != false {
		t.Fatalf("ui_exists=%v, want false", ui["ui_exists"])
	}
	if ui["mode"] != "reaudit_admin_mutation_control_ui_boundary" {
		t.Fatalf("mode=%v, want reaudit_admin_mutation_control_ui_boundary", ui["mode"])
	}
}

// TestSeq17P401ReauditReleaseExecutionUI validates the release execution UI
// boundary surface for SEQ-17-P401. Must be operator_required,
// execution_disabled, read_only, artifact_created:false, beta_reference_mutated:false.
func TestSeq17P401ReauditReleaseExecutionUI(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p401","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	ui := seq165Map(t, resp, "reaudit_release_execution_ui")
	if ui["version"] != "seq17_p401.v1" {
		t.Fatalf("version=%v, want seq17_p401.v1", ui["version"])
	}
	if ui["role"] != "reaudit_release_execution_ui" {
		t.Fatalf("role=%v, want reaudit_release_execution_ui", ui["role"])
	}
	if ui["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", ui["truth_authority"])
	}
	if ui["operator_required"] != true {
		t.Fatalf("operator_required=%v, want true", ui["operator_required"])
	}
	if ui["execution_disabled"] != true {
		t.Fatalf("execution_disabled=%v, want true", ui["execution_disabled"])
	}
	if ui["read_only"] != true {
		t.Fatalf("read_only=%v, want true", ui["read_only"])
	}
	if ui["artifact_created"] != false {
		t.Fatalf("artifact_created=%v, want false", ui["artifact_created"])
	}
	if ui["beta_reference_mutated"] != false {
		t.Fatalf("beta_reference_mutated=%v, want false", ui["beta_reference_mutated"])
	}
	if ui["ui_exists"] != false {
		t.Fatalf("ui_exists=%v, want false", ui["ui_exists"])
	}
	if ui["mode"] != "reaudit_release_execution_ui_boundary" {
		t.Fatalf("mode=%v, want reaudit_release_execution_ui_boundary", ui["mode"])
	}
}

// TestSeq17P402ReauditBeta08ClosureBundle validates the Beta 0.8 closure bundle
// boundary surface for SEQ-17-P402. Must have bundle_folder_authoritative:false,
// root_source_of_truth:true, artifact_created:false, beta_reference_mutated:false.
func TestSeq17P402ReauditBeta08ClosureBundle(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p402","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	b := seq165Map(t, resp, "reaudit_beta_0_8_closure_bundle")
	if b["version"] != "seq17_p402.v1" {
		t.Fatalf("version=%v, want seq17_p402.v1", b["version"])
	}
	if b["role"] != "reaudit_beta_0_8_closure_bundle" {
		t.Fatalf("role=%v, want reaudit_beta_0_8_closure_bundle", b["role"])
	}
	if b["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", b["truth_authority"])
	}
	if b["bundle_folder_authoritative"] != false {
		t.Fatalf("bundle_folder_authoritative=%v, want false", b["bundle_folder_authoritative"])
	}
	if b["root_source_of_truth"] != true {
		t.Fatalf("root_source_of_truth=%v, want true", b["root_source_of_truth"])
	}
	if b["artifact_created"] != false {
		t.Fatalf("artifact_created=%v, want false", b["artifact_created"])
	}
	if b["beta_reference_mutated"] != false {
		t.Fatalf("beta_reference_mutated=%v, want false", b["beta_reference_mutated"])
	}
	if b["mode"] != "reaudit_beta_0_8_closure_bundle_boundary" {
		t.Fatalf("mode=%v, want reaudit_beta_0_8_closure_bundle_boundary", b["mode"])
	}
}

// TestSeq17P412DecisionCompletenessMetricUnit validates the completeness metric
// default unit decision surface for SEQ-17-P412.
func TestSeq17P412DecisionCompletenessMetricUnit(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p412","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	d := seq165Map(t, resp, "decision_completeness_metric_unit")
	if d["version"] != "seq17_p412.v1" {
		t.Fatalf("version=%v, want seq17_p412.v1", d["version"])
	}
	if d["role"] != "decision_completeness_metric_unit" {
		t.Fatalf("role=%v, want decision_completeness_metric_unit", d["role"])
	}
	if d["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", d["truth_authority"])
	}
	if d["decision_state"] != "pending" {
		t.Fatalf("decision_state=%v, want pending", d["decision_state"])
	}
	if d["default_unit"] != "retrieval_slice" {
		t.Fatalf("default_unit=%v, want retrieval_slice", d["default_unit"])
	}
	if d["mode"] != "decision_completeness_metric_unit_pending" {
		t.Fatalf("mode=%v, want decision_completeness_metric_unit_pending", d["mode"])
	}
}

// TestSeq17P413DecisionRegressionCorpusMix validates the regression corpus mix
// decision surface for SEQ-17-P413.
func TestSeq17P413DecisionRegressionCorpusMix(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p413","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	d := seq165Map(t, resp, "decision_regression_corpus_mix")
	if d["version"] != "seq17_p413.v1" {
		t.Fatalf("version=%v, want seq17_p413.v1", d["version"])
	}
	if d["role"] != "decision_regression_corpus_mix" {
		t.Fatalf("role=%v, want decision_regression_corpus_mix", d["role"])
	}
	if d["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", d["truth_authority"])
	}
	if d["decision_state"] != "fixed" {
		t.Fatalf("decision_state=%v, want fixed", d["decision_state"])
	}
	if d["chosen_mix"] != "mixed_replay_and_runtime_contract" {
		t.Fatalf("chosen_mix=%v, want mixed_replay_and_runtime_contract", d["chosen_mix"])
	}
	if d["synthetic_only"] != false {
		t.Fatalf("synthetic_only=%v, want false", d["synthetic_only"])
	}
	if d["actual_replay_only"] != false {
		t.Fatalf("actual_replay_only=%v, want false", d["actual_replay_only"])
	}
	if d["mode"] != "decision_regression_corpus_mixed_fixed" {
		t.Fatalf("mode=%v, want decision_regression_corpus_mixed_fixed", d["mode"])
	}
}

// TestSeq17P414DecisionInspectionLaneDefault validates the inspection lane
// default decision surface for SEQ-17-P414.
func TestSeq17P414DecisionInspectionLaneDefault(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p414","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	d := seq165Map(t, resp, "decision_inspection_lane_default")
	if d["version"] != "seq17_p414.v1" {
		t.Fatalf("version=%v, want seq17_p414.v1", d["version"])
	}
	if d["role"] != "decision_inspection_lane_default" {
		t.Fatalf("role=%v, want decision_inspection_lane_default", d["role"])
	}
	if d["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", d["truth_authority"])
	}
	if d["decision_state"] != "fixed" {
		t.Fatalf("decision_state=%v, want fixed", d["decision_state"])
	}
	if d["panel_location"] != "root_runtime_debug_panel" {
		t.Fatalf("panel_location=%v, want root_runtime_debug_panel", d["panel_location"])
	}
	if d["mode"] != "decision_inspection_lane_default_fixed" {
		t.Fatalf("mode=%v, want decision_inspection_lane_default_fixed", d["mode"])
	}
}

// TestSeq17P415DecisionAdoptionGateReviewMode validates the adoption gate review
// mode decision surface for SEQ-17-P415.
func TestSeq17P415DecisionAdoptionGateReviewMode(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p415","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	d := seq165Map(t, resp, "decision_adoption_gate_review_mode")
	if d["version"] != "seq17_p415.v1" {
		t.Fatalf("version=%v, want seq17_p415.v1", d["version"])
	}
	if d["role"] != "decision_adoption_gate_review_mode" {
		t.Fatalf("role=%v, want decision_adoption_gate_review_mode", d["role"])
	}
	if d["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", d["truth_authority"])
	}
	if d["decision_state"] != "fixed" {
		t.Fatalf("decision_state=%v, want fixed", d["decision_state"])
	}
	if d["review_mode"] != "slice_manual_review_plus_automatic_gate" {
		t.Fatalf("review_mode=%v, want slice_manual_review_plus_automatic_gate", d["review_mode"])
	}
	if d["backend_gate_payload"] != true {
		t.Fatalf("backend_gate_payload=%v, want true", d["backend_gate_payload"])
	}
	if d["root_runtime_read_only_panel"] != true {
		t.Fatalf("root_runtime_read_only_panel=%v, want true", d["root_runtime_read_only_panel"])
	}
	if d["mode"] != "decision_adoption_gate_review_mode_fixed" {
		t.Fatalf("mode=%v, want decision_adoption_gate_review_mode_fixed", d["mode"])
	}
}

// TestSeq17P416DecisionBundleRegenerateSplit validates the bundle regenerate
// split decision surface for SEQ-17-P416.
func TestSeq17P416DecisionBundleRegenerateSplit(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p416","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	d := seq165Map(t, resp, "decision_bundle_regenerate_split")
	if d["version"] != "seq17_p416.v1" {
		t.Fatalf("version=%v, want seq17_p416.v1", d["version"])
	}
	if d["role"] != "decision_bundle_regenerate_split" {
		t.Fatalf("role=%v, want decision_bundle_regenerate_split", d["role"])
	}
	if d["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", d["truth_authority"])
	}
	if d["decision_state"] != "fixed" {
		t.Fatalf("decision_state=%v, want fixed", d["decision_state"])
	}
	if d["truth_surface"] != "release_hygiene_checklist" {
		t.Fatalf("truth_surface=%v, want release_hygiene_checklist", d["truth_surface"])
	}
	if d["actual_bundle_refresh"] != "operator_execution_split" {
		t.Fatalf("actual_bundle_refresh=%v, want operator_execution_split", d["actual_bundle_refresh"])
	}
	if d["script_plus_checklist"] != true {
		t.Fatalf("script_plus_checklist=%v, want true", d["script_plus_checklist"])
	}
	if d["mode"] != "decision_bundle_regenerate_split_fixed" {
		t.Fatalf("mode=%v, want decision_bundle_regenerate_split_fixed", d["mode"])
	}
}

// TestSeq17P420ChromaMigrationPreflight validates the 17-C1 migration preflight
// dry-run surface for SEQ-17-P420.
func TestSeq17P420ChromaMigrationPreflight(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p420","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "chroma_migration_preflight")
	if s["version"] != "seq17_p420.v1" {
		t.Fatalf("version=%v, want seq17_p420.v1", s["version"])
	}
	if s["role"] != "chroma_migration_preflight" {
		t.Fatalf("role=%v, want chroma_migration_preflight", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", s["dry_run_only"])
	}
	if s["actual_migration_run"] != false {
		t.Fatalf("actual_migration_run=%v, want false", s["actual_migration_run"])
	}
	if s["storage_mutated"] != false {
		t.Fatalf("storage_mutated=%v, want false", s["storage_mutated"])
	}
	if s["mode"] != "chroma_migration_preflight_dry_run" {
		t.Fatalf("mode=%v, want chroma_migration_preflight_dry_run", s["mode"])
	}
}

// TestSeq17P421ChromaShadowBootstrap validates the 17-C2 shadow collection
// bootstrap dry-run surface for SEQ-17-P421.
func TestSeq17P421ChromaShadowBootstrap(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"seq17-p421","turn_index":1,"raw_user_input":"hello","settings":{"injection_enabled":true,"input_context_enabled":true}}`
	req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	s := seq165Map(t, resp, "chroma_shadow_bootstrap")
	if s["version"] != "seq17_p421.v1" {
		t.Fatalf("version=%v, want seq17_p421.v1", s["version"])
	}
	if s["role"] != "chroma_shadow_bootstrap" {
		t.Fatalf("role=%v, want chroma_shadow_bootstrap", s["role"])
	}
	if s["truth_authority"] != false {
		t.Fatalf("truth_authority=%v, want false", s["truth_authority"])
	}
	if s["dry_run_only"] != true {
		t.Fatalf("dry_run_only=%v, want true", s["dry_run_only"])
	}
	if s["actual_migration_run"] != false {
		t.Fatalf("actual_migration_run=%v, want false", s["actual_migration_run"])
	}
	if s["collection_created"] != false {
		t.Fatalf("collection_created=%v, want false", s["collection_created"])
	}
	if s["metadata_written"] != false {
		t.Fatalf("metadata_written=%v, want false", s["metadata_written"])
	}
	if s["health_probe_run"] != false {
		t.Fatalf("health_probe_run=%v, want false", s["health_probe_run"])
	}
	if s["mode"] != "chroma_shadow_bootstrap_dry_run" {
		t.Fatalf("mode=%v, want chroma_shadow_bootstrap_dry_run", s["mode"])
	}
}
