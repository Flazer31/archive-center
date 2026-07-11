package httpapi

import (
	archivebridge "github.com/risulongmemory/archive-center-go/internal/archive"
)

// ---------------------------------------------------------------------------
// SEQ-18 reset administration surfaces (P13 ~ P15)
// ---------------------------------------------------------------------------

// buildResetAdmin defines the Step 18 reset administration surface for
// SEQ-18-P13: existing checked checklist items were cleared for redo.
func buildResetAdmin() map[string]any {
	return map[string]any{
		"version":              "seq18_p13.v1",
		"role":                 "reset_administration",
		"truth_authority":      false,
		"reset_action":         "checklist_cleared_for_redo",
		"historical_preserved": true,
		"policy_version":       "s18-rst.v1",
		"mode":                 "reset_administration_note",
	}
}

// buildHistoricalContentPreserved defines the Step 18 historical content
// preservation surface for SEQ-18-P14: historical content was preserved.
func buildHistoricalContentPreserved() map[string]any {
	return map[string]any{
		"version":           "seq18_p14.v1",
		"role":              "historical_content_preserved",
		"truth_authority":   false,
		"content_preserved": true,
		"no_text_deleted":   true,
		"policy_version":    "s18-rst.v1",
		"mode":              "historical_content_preservation_note",
	}
}

// buildResetNoteOnly defines the Step 18 reset scope surface for
// SEQ-18-P15: reset note records document reset work only.
func buildResetNoteOnly() map[string]any {
	return map[string]any{
		"version":            "seq18_p15.v1",
		"role":               "reset_note_only",
		"truth_authority":    false,
		"scope":              "document_reset_only",
		"revalidation_claim": false,
		"policy_version":     "s18-rst.v1",
		"mode":               "reset_scope_note",
	}
}

// ---------------------------------------------------------------------------
// SEQ-18 preparation kick-off surfaces (P19 ~ P25)
// ---------------------------------------------------------------------------

// buildStep17ClosureGate defines the Step 17 bundle release closure
// re-confirmation surface for SEQ-18-P19: active Step 18 entry gate.
func buildStep17ClosureGate() map[string]any {
	return map[string]any{
		"version":              "seq18_p19.v1",
		"role":                 "step_17_closure_gate",
		"truth_authority":      false,
		"closure_status":       "closed",
		"release_gate_closed":  true,
		"entry_gate_confirmed": true,
		"policy_version":       "s18-prep.v1",
		"mode":                 "step_17_closure_entry_gate",
	}
}

// buildContextFilesReviewed defines the Step 18 context file review
// surface for SEQ-18-P20: reopened Step 18 context/progress files reviewed.
func buildContextFilesReviewed() map[string]any {
	return map[string]any{
		"version":             "seq18_p20.v1",
		"role":                "context_files_reviewed",
		"truth_authority":     false,
		"files_reviewed":      true,
		"redo_baseline_ready": true,
		"policy_version":      "s18-prep.v1",
		"mode":                "context_files_review_note",
	}
}

// buildPrepAnchorVRHY defines the Step 18 preparatory next anchor
// surface for SEQ-18-P21: 18-1 VR + 18-2 HY first.
func buildPrepAnchorVRHY() map[string]any {
	return map[string]any{
		"version":           "seq18_p21.v1",
		"role":              "prep_anchor_vr_hy",
		"truth_authority":   false,
		"primary_anchors":   []string{"18-1_vr", "18-2_hy"},
		"downstream_slices": []string{"18-3_qr", "18-4_vx"},
		"policy_version":    "s18-prep.v1",
		"mode":              "preparatory_anchor_definition",
	}
}

// buildHistoricalReferenceOnly defines the Step 18 historical reference
// status surface for SEQ-18-P22: historical completion text is reference only.
func buildHistoricalReferenceOnly() map[string]any {
	return map[string]any{
		"version":               "seq18_p22.v1",
		"role":                  "historical_reference_only",
		"truth_authority":       false,
		"historical_text":       "reference_only",
		"new_validation_needed": true,
		"policy_version":        "s18-prep.v1",
		"mode":                  "historical_reference_status_note",
	}
}

// buildBackendPrepAnchor defines the Step 18 backend preparation anchor
// surface for SEQ-18-P23: bridge.py::search_memories preparation anchor.
func buildBackendPrepAnchor() map[string]any {
	return map[string]any{
		"version":         "seq18_p23.v1",
		"role":            "backend_prep_anchor",
		"truth_authority": false,
		"anchor_file":     "backend/archive/bridge.py",
		"anchor_function": "search_memories",
		"policy_version":  "s18-prep.v1",
		"mode":            "backend_preparation_anchor",
	}
}

// buildRoutingContractPrepAnchor defines the Step 18 routing-contract
// preparation anchor surface for SEQ-18-P24: _build_recall_intent_contract_q3a.
func buildRoutingContractPrepAnchor() map[string]any {
	return map[string]any{
		"version":         "seq18_p24.v1",
		"role":            "routing_contract_prep_anchor",
		"truth_authority": false,
		"anchor_file":     "backend/main.py",
		"anchor_function": "_build_recall_intent_contract_q3a",
		"policy_version":  "s18-prep.v1",
		"mode":            "routing_contract_preparation_anchor",
	}
}

// buildRuntimePrepScope defines the Step 18 runtime preparation scope
// surface for SEQ-18-P25: runtime-facing Step 18 surfacing remains prep scope.
func buildRuntimePrepScope() map[string]any {
	return map[string]any{
		"version":         "seq18_p25.v1",
		"role":            "runtime_prep_scope",
		"truth_authority": false,
		"explicit_labels": false,
		"scope":           "preparation_only",
		"policy_version":  "s18-prep.v1",
		"mode":            "runtime_preparation_scope_note",
	}
}

// ---------------------------------------------------------------------------
// SEQ-18 VR (verbatim recall) surfaces (P29 ~ P35)
// ---------------------------------------------------------------------------

// buildVRScopedVerbatimSupportText defines the VR scoped verbatim support
// surface for SEQ-18-P29: additive scoped_verbatim_support_text/count/items.
func buildVRScopedVerbatimSupportText(support archivebridge.ScopedVerbatimSupport) map[string]any {
	return map[string]any{
		"version":                       "seq18_p29.v1",
		"role":                          "vr_scoped_verbatim_support",
		"truth_authority":               false,
		"scoped_verbatim_support_text":  support.Text,
		"scoped_verbatim_support_count": support.Count,
		"scoped_verbatim_support_items": support.Items,
		"source":                        "direct_evidence_gate_approved",
		"policy_version":                "vr18a.v1",
		"mode":                          "vr_scoped_verbatim_support_surface",
	}
}

// buildVRPolicyOwnerBlock defines the VR policy owner block surface for
// SEQ-18-P30: localized policy with max_items, max_total_chars, etc.
func buildVRPolicyOwnerBlock() map[string]any {
	return map[string]any{
		"version":                   "seq18_p30.v1",
		"role":                      "vr_policy_owner_block",
		"truth_authority":           false,
		"max_items":                 3,
		"max_total_chars":           720,
		"max_excerpt_chars":         160,
		"support_surface_first":     true,
		"prompt_injection_strategy": "latest_anchor_only",
		"source_tag_metadata":       "[source=direct_evidence scope=... turns=... anchor=... kind=...]",
		"policy_version":            "vr18b.v1",
		"mode":                      "vr_policy_owner_block_definition",
	}
}

// buildVRPromptInjectionStrategy defines the VR prompt injection strategy
// surface for SEQ-18-P31: latest_anchor_only, multi-item lane as support surface.
func buildVRPromptInjectionStrategy() map[string]any {
	return map[string]any{
		"version":                  "seq18_p31.v1",
		"role":                     "vr_prompt_injection_strategy",
		"truth_authority":          false,
		"injection_strategy":       "latest_anchor_only",
		"multi_item_lane_exposed":  true,
		"multi_item_lane_label":    "Scoped Verbatim Recall (support surface)",
		"prompt_injection_widened": false,
		"policy_version":           "vr18c.v1",
		"mode":                     "vr_prompt_injection_strategy_note",
	}
}

// buildVRHierarchyEscapeHatch defines the VR hierarchy escape hatch
// surface for SEQ-18-P32: hierarchy_escape_hatch metadata and surface priority.
func buildVRHierarchyEscapeHatch() map[string]any {
	return map[string]any{
		"version":                           "seq18_p32.v1",
		"role":                              "vr_hierarchy_escape_hatch",
		"truth_authority":                   false,
		"hierarchy_escape_hatch":            true,
		"verbatim_support_surface_priority": true,
		"hierarchy_escape_hatch_status":     "visible_when_summary_thin",
		"policy_version":                    "vr18d.v1",
		"mode":                              "vr_hierarchy_escape_hatch_definition",
	}
}

// buildVRBackendTestGuard defines the VR backend test guard surface for
// SEQ-18-P33: test_step18_scoped_verbatim_support.py guards the new surface.
func buildVRBackendTestGuard() map[string]any {
	return map[string]any{
		"version":         "seq18_p33.v1",
		"role":            "vr_backend_test_guard",
		"truth_authority": false,
		"test_file":       "backend/test_step18_scoped_verbatim_support.py",
		"guards": []string{
			"support_surface",
			"item_caps",
			"source_tag_metadata",
			"prompt_strategy",
			"hierarchy_escape_hatch_metadata",
		},
		"policy_version": "vr18a.v1",
		"mode":           "vr_backend_test_guard_surface",
	}
}

// buildVRRuntimeTransparency defines the VR runtime transparency surface
// for SEQ-18-P34: runtime trace write-through + Scoped Verbatim Recall section.
func buildVRRuntimeTransparency() map[string]any {
	return map[string]any{
		"version":              "seq18_p34.v1",
		"role":                 "vr_runtime_transparency",
		"truth_authority":      false,
		"trace_write_through":  true,
		"transparency_section": "Scoped Verbatim Recall (support surface)",
		"test_file":            "test_step18_scoped_verbatim_input_transparency.js",
		"policy_version":       "vr18a.v1",
		"mode":                 "vr_runtime_transparency_surface",
	}
}

// buildVRRegressionBundleGreen defines the VR regression bundle green
// surface for SEQ-18-P35: backend regression bundle + Step 19 stayed green.
func buildVRRegressionBundleGreen() map[string]any {
	return map[string]any{
		"version":                "seq18_p35.v1",
		"role":                   "vr_regression_bundle_green",
		"truth_authority":        false,
		"vr_slice_green":         true,
		"adjacent_step19_green":  true,
		"combined_bundle_status": "green",
		"policy_version":         "vr18a.v1",
		"mode":                   "vr_regression_bundle_green_note",
	}
}

// ---------------------------------------------------------------------------
// SEQ-18 HY (hybrid retrieval) surfaces (P46 ~ P53)
// ---------------------------------------------------------------------------

// buildHYSemanticRankScore defines the HY semantic rank + keyword overlap
// surface for SEQ-18-P46: hy1a.v1 bounded keyword-overlap scoring.
func buildHYSemanticRankScore() map[string]any {
	return map[string]any{
		"version":                        "seq18_p46.v1",
		"role":                           "hy_semantic_rank_keyword_overlap",
		"truth_authority":                false,
		"semantic_rank_preserved":        true,
		"keyword_overlap_policy":         "hy1a.v1",
		"keyword_overlap_score":          0.0,
		"hybrid_baseline_score":          0.0,
		"keyword_overlap_terms":          []string{},
		"hybrid_baseline_policy_version": "hy1a.v1",
		"policy_version":                 "hy1a.v1",
		"mode":                           "hy_semantic_rank_keyword_overlap_surface",
	}
}

// buildHYSoftBias defines the HY structured soft bias surface for
// SEQ-18-P47: hy1b.v1 speaker/location/storyline cue weights.
func buildHYSoftBias() map[string]any {
	return map[string]any{
		"version":                  "seq18_p47.v1",
		"role":                     "hy_soft_bias",
		"truth_authority":          false,
		"soft_bias_policy":         "hy1b.v1",
		"speaker_bias_weight":      0.04,
		"location_bias_weight":     0.05,
		"storyline_bias_weight":    0.06,
		"soft_bias_cap":            0.12,
		"speaker_bias_score":       0.0,
		"location_bias_score":      0.0,
		"storyline_bias_score":     0.0,
		"soft_bias_score":          0.0,
		"soft_bias_policy_version": "hy1b.v1",
		"policy_version":           "hy1b.v1",
		"mode":                     "hy_soft_bias_surface",
	}
}

// buildHYStopwordGuard defines the HY stopword guard surface for
// SEQ-18-P48: common English filler terms no longer count toward overlap.
func buildHYStopwordGuard() map[string]any {
	return map[string]any{
		"version":                  "seq18_p48.v1",
		"role":                     "hy_stopword_guard",
		"truth_authority":          false,
		"stopword_inflation_fixed": true,
		"tightened_extractor":      true,
		"common_filler_excluded":   true,
		"policy_version":           "hy1a.v1",
		"mode":                     "hy_stopword_guard_surface",
	}
}

// buildHYQ1aPropagation defines the HY q1a propagation surface for
// SEQ-18-P49: HY trace fields propagated into q1a unified retrieval document.
func buildHYQ1aPropagation() map[string]any {
	return map[string]any{
		"version":         "seq18_p49.v1",
		"role":            "hy_q1a_propagation",
		"truth_authority": false,
		"q1a_propagation": true,
		"propagated_fields": []string{
			"keyword_overlap_score",
			"hybrid_baseline_score",
			"keyword_overlap_terms",
			"hybrid_baseline_policy_version",
			"speaker_bias_score",
			"location_bias_score",
			"storyline_bias_score",
			"soft_bias_score",
			"soft_bias_policy_version",
		},
		"policy_version": "hy1a.v1",
		"mode":           "hy_q1a_propagation_surface",
	}
}

// buildHYRuntimeInspection defines the HY runtime inspection surface for
// SEQ-18-P50: JS reads HY score/bias fields, renders Hybrid Retrieval Inspection.
func buildHYRuntimeInspection() map[string]any {
	return map[string]any{
		"version":                 "seq18_p50.v1",
		"role":                    "hy_runtime_inspection",
		"truth_authority":         false,
		"js_function":             "extractMemoryItems",
		"row_meta_extended":       true,
		"row_meta_fields":         []string{"final", "kw", "soft"},
		"transparency_block":      "Hybrid Retrieval Inspection",
		"transparency_block_type": "trace_only",
		"policy_version":          "hy1b.v1",
		"mode":                    "hy_runtime_inspection_surface",
	}
}

// buildHYRecurringRiskGuards defines the HY recurring-risk guard surface
// for SEQ-18-P51: stopword-inflated overlap + missing q1a HY metadata guards.
func buildHYRecurringRiskGuards() map[string]any {
	return map[string]any{
		"version":           "seq18_p51.v1",
		"role":              "hy_recurring_risk_guards",
		"truth_authority":   false,
		"backend_test_file": "backend/test_step18_hybrid_regression.py",
		"js_test_file":      "test_step18_hybrid_input_transparency.js",
		"guards": []map[string]any{
			{"name": "stopword_inflated_overlap", "status": "guarded"},
			{"name": "missing_q1a_hy_metadata", "status": "guarded"},
			{"name": "hybrid_retrieval_inspection_disappearance", "status": "guarded"},
		},
		"policy_version": "hy1a.v1",
		"mode":           "hy_recurring_risk_guard_surface",
	}
}

// buildHYPolicyRegistry defines the HY policy registry consolidation
// surface for SEQ-18-P52: hybrid_policy.py as single versioned policy registry.
func buildHYPolicyRegistry() map[string]any {
	return map[string]any{
		"version":                     "seq18_p52.v1",
		"role":                        "hy_policy_registry",
		"truth_authority":             false,
		"registry_file":               "backend/archive/hybrid_policy.py",
		"consolidated":                true,
		"scattered_hardcoded_removed": true,
		"policy_family":               "hy",
		"policy_version":              "hy1a.v1",
		"mode":                        "hy_policy_registry_surface",
	}
}

// buildHYStopAt18_2c defines the HY intentional stop surface for
// SEQ-18-P53: stops at 18-2c, 18-2d/18-3/18-4 remain open follow-up.
func buildHYStopAt18_2c() map[string]any {
	return map[string]any{
		"version":            "seq18_p53.v1",
		"role":               "hy_stop_at_18_2c",
		"truth_authority":    false,
		"stop_point":         "18-2c",
		"open_follow_up":     []string{"18-2d", "18-3", "18-4"},
		"tail_budget_rescue": "pending",
		"policy_version":     "hy1a.v1",
		"mode":               "hy_intentional_stop_note",
	}
}

// ---------------------------------------------------------------------------
// SEQ-18 HY tail-budget rescue surfaces (P65 ~ P69)
// ---------------------------------------------------------------------------

// buildHYTailBudgetPolicyOwner defines the HY tail-budget policy owner
// surface for SEQ-18-P65: hy1d.v1 as part of the shared HY policy family.
func buildHYTailBudgetPolicyOwner() map[string]any {
	return map[string]any{
		"version":                     "seq18_p65.v1",
		"role":                        "hy_tail_budget_policy_owner",
		"truth_authority":             false,
		"policy_family":               "hy",
		"policy_version":              "hy1d.v1",
		"scattered_hardcoded_removed": true,
		"registry_file":               "backend/archive/hybrid_policy.py",
		"mode":                        "hy_tail_budget_policy_owner_surface",
	}
}

// buildHYTailBudgetRescuePass defines the HY tail-budget rescue pass
// surface for SEQ-18-P66: bounded post-rank rescue, same n_results budget,
// at most one near-cutoff candidate promoted when keyword/soft-bias signal
// is stronger than the cutline item.
func buildHYTailBudgetRescuePass() map[string]any {
	return map[string]any{
		"version":                 "seq18_p66.v1",
		"role":                    "hy_tail_budget_rescue_pass",
		"truth_authority":         false,
		"rescue_enabled":          true,
		"max_promotions_per_pass": 1,
		"budget_preserved":        true,
		"promotion_trigger":       "keyword_soft_bias_stronger_than_cutline",
		"policy_version":          "hy1d.v1",
		"mode":                    "hy_tail_budget_rescue_pass_surface",
	}
}

// buildHYTailBudgetRescueTrace defines the HY tail-budget rescue trace
// surface for SEQ-18-P67: promoted candidates retain explicit rescue trace.
func buildHYTailBudgetRescueTrace() map[string]any {
	return map[string]any{
		"version":         "seq18_p67.v1",
		"role":            "hy_tail_budget_rescue_trace",
		"truth_authority": false,
		"trace_fields": []string{
			"tail_budget_policy_version",
			"tail_budget_original_rank",
			"tail_budget_promoted",
			"tail_budget_reason",
			"tail_budget_score_gap",
		},
		"trace_mandatory": true,
		"policy_version":  "hy1d.v1",
		"mode":            "hy_tail_budget_rescue_trace_surface",
	}
}

// buildHYTailBudgetQ1aPropagation defines the HY tail-budget q1a
// propagation surface for SEQ-18-P68: trace fields propagated into q1a unified
// retrieval document metadata.
func buildHYTailBudgetQ1aPropagation() map[string]any {
	return map[string]any{
		"version":         "seq18_p68.v1",
		"role":            "hy_tail_budget_q1a_propagation",
		"truth_authority": false,
		"q1a_propagation": true,
		"propagated_fields": []string{
			"tail_budget_policy_version",
			"tail_budget_original_rank",
			"tail_budget_promoted",
			"tail_budget_reason",
			"tail_budget_score_gap",
		},
		"policy_version": "hy1d.v1",
		"mode":           "hy_tail_budget_q1a_propagation_surface",
	}
}

// buildHYTailBudgetRegression defines the HY tail-budget regression
// surface for SEQ-18-P69: near-cutoff rescue regression test coverage.
func buildHYTailBudgetRegression() map[string]any {
	return map[string]any{
		"version":          "seq18_p69.v1",
		"role":             "hy_tail_budget_regression",
		"truth_authority":  false,
		"test_file":        "backend/test_step18_hybrid_regression.py",
		"regression_scope": "near_cutoff_rescue",
		"verifies": []string{
			"promotion_into_same_budget",
			"q1a_metadata_propagation",
		},
		"policy_version": "hy1d.v1",
		"mode":           "hy_tail_budget_regression_surface",
	}
}

// ---------------------------------------------------------------------------
// SEQ-18 QR query-class contract surfaces (P76 ~ P91)
// ---------------------------------------------------------------------------

// buildQRQueryClassContract defines the QR query-class contract surface
// for SEQ-18-P76: qr1a.v1 query-class contract metadata, additive, fail-open.
