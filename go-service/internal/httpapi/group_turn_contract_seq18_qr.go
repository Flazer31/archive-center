package httpapi

func buildQRQueryClassContract() map[string]any {
	return map[string]any{
		"version":          "seq18_p76.v1",
		"role":             "qr_query_class_contract",
		"truth_authority":  false,
		"contract_version": "qr1a.v1",
		"execution_mode":   "single_query_shared",
		"fail_open":        true,
		"additive_only":    true,
		"policy_version":   "qr1a.v1",
		"mode":             "qr_query_class_contract_surface",
	}
}

// buildQRQueryClassTaxonomy defines the QR query-class taxonomy surface
// for SEQ-18-P77: explicit taxonomy with scene, callback, resume, canon, temporal.
func buildQRQueryClassTaxonomy() map[string]any {
	return map[string]any{
		"version":               "seq18_p77.v1",
		"role":                  "qr_query_class_taxonomy",
		"truth_authority":       false,
		"query_classes":         []string{"scene", "callback", "resume", "canon", "temporal"},
		"contract_layer_only":   true,
		"primary_class_visible": true,
		"policy_version":        "qr1a.v1",
		"mode":                  "qr_query_class_taxonomy_surface",
	}
}

// buildQRPrimaryClassSelection defines the QR primary class selection
// precedence surface for SEQ-18-P78: temporal > resume > canon > callback > scene.
func buildQRPrimaryClassSelection() map[string]any {
	return map[string]any{
		"version":         "seq18_p78.v1",
		"role":            "qr_primary_class_selection",
		"truth_authority": false,
		"precedence": []string{
			"explicit_temporal_cue",
			"resume_trigger_or_ready_resume_pack",
			"canon_guard_signal",
			"callback_recovery_signal",
			"scene_fallback",
		},
		"policy_version": "qr1a.v1",
		"mode":           "qr_primary_class_selection_surface",
	}
}

// buildQRLexicalCueBlock defines the QR lexical cue block surface for
// SEQ-18-P79: localized cue lists and descriptions in one constant block.
func buildQRLexicalCueBlock() map[string]any {
	return map[string]any{
		"version":                 "seq18_p79.v1",
		"role":                    "qr_lexical_cue_block",
		"truth_authority":         false,
		"localized":               true,
		"hidden_literals_removed": true,
		"cue_block_owner":         "query_class_contract",
		"policy_version":          "qr1a.v1",
		"mode":                    "qr_lexical_cue_block_surface",
	}
}

// buildQRQueryClassContractTest defines the QR query-class contract test
// surface for SEQ-18-P80: temporal-over-resume, resume-over-callback, callback/scene
// fallback behavior coverage.
func buildQRQueryClassContractTest() map[string]any {
	return map[string]any{
		"version":         "seq18_p80.v1",
		"role":            "qr_query_class_contract_test",
		"truth_authority": false,
		"test_file":       "backend/test_step18_query_class_contract.py",
		"covers": []string{
			"temporal_over_resume_precedence",
			"resume_over_callback_precedence",
			"callback_scene_fallback",
		},
		"policy_version": "qr1a.v1",
		"mode":           "qr_query_class_contract_test_surface",
	}
}

// buildQRQueryClassBudgetPolicy defines the QR query-class budget policy
// surface for SEQ-18-P87: qr1b.v1 budget policy metadata, additive, fail-open.
func buildQRQueryClassBudgetPolicy() map[string]any {
	return map[string]any{
		"version":               "seq18_p87.v1",
		"role":                  "qr_query_class_budget_policy",
		"truth_authority":       false,
		"budget_policy_version": "qr1b.v1",
		"execution_mode":        "single_query_shared",
		"fail_open":             true,
		"additive_only":         true,
		"policy_version":        "qr1b.v1",
		"mode":                  "qr_query_class_budget_policy_surface",
	}
}

// buildQRQ3cBudgetReuse defines the QR q3c budget reuse surface for
// SEQ-18-P88: scene/callback/resume/canon reuse existing q3c intent packet budgets.
func buildQRQ3cBudgetReuse() map[string]any {
	return map[string]any{
		"version":                    "seq18_p88.v1",
		"role":                       "qr_q3c_budget_reuse",
		"truth_authority":            false,
		"reused_budget_source":       "q3c_intent_packet",
		"executable_classes":         []string{"scene", "callback", "resume", "canon"},
		"independent_budget_avoided": true,
		"policy_version":             "qr1b.v1",
		"mode":                       "qr_q3c_budget_reuse_surface",
	}
}

// buildQRTemporalProfileBudget defines the QR temporal profile-based
// budget surface for SEQ-18-P89: temporal gets separate evidence-first overlay.
func buildQRTemporalProfileBudget() map[string]any {
	return map[string]any{
		"version":         "seq18_p89.v1",
		"role":            "qr_temporal_profile_budget",
		"truth_authority": false,
		"profile_based":   true,
		"evidence_first":  true,
		"overlay_budget":  true,
		"candidate_caps": []string{
			"temporal_integrity",
			"direct_evidence",
			"search",
		},
		"shared_profile_template": true,
		"policy_version":          "qr1b.v1",
		"mode":                    "qr_temporal_profile_budget_surface",
	}
}

// buildQRBudgetVisibility defines the QR budget visibility surface for
// SEQ-18-P90: each class carries retrieval_depth, candidate_budget, budget_policy_version,
// budget_source so 18-3b is visible as contract data.
func buildQRBudgetVisibility() map[string]any {
	return map[string]any{
		"version":         "seq18_p90.v1",
		"role":            "qr_budget_visibility",
		"truth_authority": false,
		"visible_fields": []string{
			"retrieval_depth",
			"candidate_budget",
			"budget_policy_version",
			"budget_source",
		},
		"contract_data_visible":        true,
		"hidden_builder_logic_removed": true,
		"policy_version":               "qr1b.v1",
		"mode":                         "qr_budget_visibility_surface",
	}
}

// buildQRQueryClassBudgetTest defines the QR query-class budget test
// surface for SEQ-18-P91: q3c reuse + temporal budget differences coverage.
func buildQRQueryClassBudgetTest() map[string]any {
	return map[string]any{
		"version":         "seq18_p91.v1",
		"role":            "qr_query_class_budget_test",
		"truth_authority": false,
		"test_file":       "backend/test_step18_query_class_budget_policy.py",
		"covers": []string{
			"q3c_budget_reuse_executable",
			"temporal_budget_difference",
		},
		"policy_version": "qr1b.v1",
		"mode":           "qr_query_class_budget_test_surface",
	}
}

// ---------------------------------------------------------------------------
// SEQ-18 QR note/route policy surfaces (P98 ~ P113)
// ---------------------------------------------------------------------------

// buildQRNotePolicy defines the QR note policy surface for
// SEQ-18-P98: qr1c.v1 note policy metadata, additive, fail-open.
func buildQRNotePolicy() map[string]any {
	return map[string]any{
		"version":             "seq18_p98.v1",
		"role":                "qr_note_policy",
		"truth_authority":     false,
		"note_policy_version": "qr1c.v1",
		"execution_mode":      "single_query_shared",
		"fail_open":           true,
		"additive_only":       true,
		"policy_version":      "qr1c.v1",
		"mode":                "qr_note_policy_surface",
	}
}

// buildQRSceneCanonNoPreExtract defines the QR scene/canon no-pre-extract
// surface for SEQ-18-P99: no pre-extract rule, support_surface_first delivery.
func buildQRSceneCanonNoPreExtract() map[string]any {
	return map[string]any{
		"version":                "seq18_p99.v1",
		"role":                   "qr_scene_canon_no_pre_extract",
		"truth_authority":        false,
		"no_pre_extract_classes": []string{"scene", "canon"},
		"note_surfaces": []string{
			"current_scene_support_surface",
			"authority_first_support_surface",
		},
		"delivery_policy": "support_surface_first",
		"policy_version":  "qr1c.v1",
		"mode":            "qr_scene_canon_no_pre_extract_surface",
	}
}

// buildQRCallbackResumeTemporalNoteOnly defines the QR callback/resume/temporal
// note-only surface for SEQ-18-P100: note_only_until_route_exec pre-extract behavior.
func buildQRCallbackResumeTemporalNoteOnly() map[string]any {
	return map[string]any{
		"version":              "seq18_p100.v1",
		"role":                 "qr_callback_resume_temporal_note_only",
		"truth_authority":      false,
		"note_only_classes":    []string{"callback", "resume", "temporal"},
		"pre_extract_behavior": "note_only_until_route_exec",
		"contract_layer_only":  true,
		"policy_version":       "qr1c.v1",
		"mode":                 "qr_callback_resume_temporal_note_only_surface",
	}
}

// buildQRNotePolicyFields defines the QR note policy fields surface for
// SEQ-18-P101: each class carries extract_before_read, retrieval_note_surface,
// pre_extract_rule, note_delivery, note_policy_version.
func buildQRNotePolicyFields() map[string]any {
	return map[string]any{
		"version":         "seq18_p101.v1",
		"role":            "qr_note_policy_fields",
		"truth_authority": false,
		"visible_fields": []string{
			"extract_before_read",
			"retrieval_note_surface",
			"pre_extract_rule",
			"note_delivery",
			"note_policy_version",
		},
		"additive_metadata": true,
		"policy_version":    "qr1c.v1",
		"mode":              "qr_note_policy_fields_surface",
	}
}

// buildQRNotePolicyTest defines the QR note policy test surface for
// SEQ-18-P102: guards no-extract defaults for scene/canon and note-only for
// callback/resume/temporal.
func buildQRNotePolicyTest() map[string]any {
	return map[string]any{
		"version":         "seq18_p102.v1",
		"role":            "qr_note_policy_test",
		"truth_authority": false,
		"test_file":       "backend/test_step18_query_class_note_policy.py",
		"guards": []string{
			"scene_canon_no_extract",
			"callback_resume_temporal_note_only",
		},
		"policy_version": "qr1c.v1",
		"mode":           "qr_note_policy_test_surface",
	}
}

// buildQRRoutePolicy defines the QR route policy surface for
// SEQ-18-P109: qr1d.v1 route policy metadata, additive, fail-open.
func buildQRRoutePolicy() map[string]any {
	return map[string]any{
		"version":              "seq18_p109.v1",
		"role":                 "qr_route_policy",
		"truth_authority":      false,
		"route_policy_version": "qr1d.v1",
		"execution_mode":       "single_query_shared",
		"fail_open":            true,
		"additive_only":        true,
		"policy_version":       "qr1d.v1",
		"mode":                 "qr_route_policy_surface",
	}
}

// buildQRRouteFamilies defines the QR route families surface for
// SEQ-18-P110: visible route families instead of hidden execution branches.
func buildQRRouteFamilies() map[string]any {
	return map[string]any{
		"version":         "seq18_p110.v1",
		"role":            "qr_route_families",
		"truth_authority": false,
		"route_families": []string{
			"scene_default",
			"callback_rescue",
			"needle_in_haystack",
			"old_detail_bridge",
			"resume_bridge",
			"canon_guard",
			"temporal_anchor",
		},
		"metadata_visible": true,
		"policy_version":   "qr1d.v1",
		"mode":             "qr_route_families_surface",
	}
}

// buildQRLongTailRouteCandidates defines the QR long-tail route candidates
// surface for SEQ-18-P111: scene/callback/resume can surface long-tail candidates
// separately from default route, detail cues promote at contract layer only.
func buildQRLongTailRouteCandidates() map[string]any {
	return map[string]any{
		"version":             "seq18_p111.v1",
		"role":                "qr_long_tail_route_candidates",
		"truth_authority":     false,
		"long_tail_classes":   []string{"scene", "callback", "resume"},
		"promotion_trigger":   "detail_old_detail_lexical_cue",
		"contract_layer_only": true,
		"runtime_unchanged":   true,
		"policy_version":      "qr1d.v1",
		"mode":                "qr_long_tail_route_candidates_surface",
	}
}

// buildQRRoutePolicyFields defines the QR route policy fields surface for
// SEQ-18-P112: each class carries route_family, route_candidates, selected_route,
// route_policy_version; primary_selected_route published without changing runtime.
func buildQRRoutePolicyFields() map[string]any {
	return map[string]any{
		"version":         "seq18_p112.v1",
		"role":            "qr_route_policy_fields",
		"truth_authority": false,
		"visible_fields": []string{
			"route_family",
			"route_candidates",
			"selected_route",
			"route_policy_version",
		},
		"publishes":         "primary_selected_route",
		"runtime_unchanged": true,
		"policy_version":    "qr1d.v1",
		"mode":              "qr_route_policy_fields_surface",
	}
}

// buildQRRoutePolicyTest defines the QR route policy test surface for
// SEQ-18-P113: guards fail-open default route + long-tail activation for detail-seeking.
func buildQRRoutePolicyTest() map[string]any {
	return map[string]any{
		"version":         "seq18_p113.v1",
		"role":            "qr_route_policy_test",
		"truth_authority": false,
		"test_file":       "backend/test_step18_query_class_route_policy.py",
		"guards": []string{
			"fail_open_default_route",
			"long_tail_activation_detail_seeking",
		},
		"policy_version": "qr1d.v1",
		"mode":           "qr_route_policy_test_surface",
	}
}

// ---------------------------------------------------------------------------
// SEQ-18 VX validation gate surfaces (P120 ~ P153)
// ---------------------------------------------------------------------------

// buildVXHybridReplayGate defines the VX hybrid replay validation gate
// surface for SEQ-18-P120: vx18a.v1 additive validation_gates.hybrid_replay.
