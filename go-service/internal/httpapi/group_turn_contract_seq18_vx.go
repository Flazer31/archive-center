package httpapi

func buildVXHybridReplayGate() map[string]any {
	return map[string]any{
		"version":             "seq18_p120.v1",
		"role":                "vx_hybrid_replay_gate",
		"truth_authority":     false,
		"gate_version":        "vx18a.v1",
		"gate_name":           "hybrid_replay",
		"execution_unchanged": true,
		"routing_unchanged":   true,
		"additive_only":       true,
		"policy_version":      "vx18a.v1",
		"mode":                "vx_hybrid_replay_gate_surface",
	}
}

// buildVXReplayThresholdReuse defines the VX replay threshold reuse
// surface for SEQ-18-P121: reuses _U1E_CAPTURED_REPLAY_* thresholds instead of
// defining a second disconnected set.
func buildVXReplayThresholdReuse() map[string]any {
	return map[string]any{
		"version":          "seq18_p121.v1",
		"role":             "vx_replay_threshold_reuse",
		"truth_authority":  false,
		"threshold_source": "_U1E_CAPTURED_REPLAY",
		"reused_for": []string{
			"semantic_only_baseline_replay",
			"hybrid_candidate_replay",
		},
		"disconnected_set_avoided": true,
		"policy_version":           "vx18a.v1",
		"mode":                     "vx_replay_threshold_reuse_surface",
	}
}

// buildVXHybridReplayStates defines the VX hybrid replay gate states
// surface for SEQ-18-P122: pending/hold ??blocked/hold ??ready/promote_candidate.
func buildVXHybridReplayStates() map[string]any {
	return map[string]any{
		"version":         "seq18_p122.v1",
		"role":            "vx_hybrid_replay_states",
		"truth_authority": false,
		"state_machine": []map[string]any{
			{"state": "pending", "action": "hold", "trigger": "missing_or_incomplete_replay_evidence"},
			{"state": "blocked", "action": "hold", "trigger": "short_mid_regression_or_missing_long_extreme_improvement"},
			{"state": "ready", "action": "promote_candidate", "trigger": "hybrid_replay_clears_all_checks"},
		},
		"policy_version": "vx18a.v1",
		"mode":           "vx_hybrid_replay_states_surface",
	}
}

// buildVXHybridReplayTest defines the VX hybrid replay gate test
// surface for SEQ-18-P123: guards pending, blocked, ready states.
func buildVXHybridReplayTest() map[string]any {
	return map[string]any{
		"version":         "seq18_p123.v1",
		"role":            "vx_hybrid_replay_test",
		"truth_authority": false,
		"test_file":       "backend/test_step18_hybrid_replay_gate.py",
		"guards": []string{
			"pending_without_evidence",
			"blocked_on_replay_failure",
			"ready_on_non_regressive_long_extreme_improvement",
		},
		"policy_version": "vx18a.v1",
		"mode":           "vx_hybrid_replay_test_surface",
	}
}

// buildVXHeldoutCompletenessGate defines the VX heldout completeness
// validation gate surface for SEQ-18-P130: vx18b.v1 additive validation_gates.
func buildVXHeldoutCompletenessGate() map[string]any {
	return map[string]any{
		"version":             "seq18_p130.v1",
		"role":                "vx_heldout_completeness_gate",
		"truth_authority":     false,
		"gate_version":        "vx18b.v1",
		"gate_name":           "heldout_completeness",
		"execution_unchanged": true,
		"routing_unchanged":   true,
		"additive_only":       true,
		"policy_version":      "vx18b.v1",
		"mode":                "vx_heldout_completeness_gate_surface",
	}
}

// buildVXHeldoutMetrics defines the VX heldout metrics surface for
// SEQ-18-P131: retention_rate, false_negative_rate, full_coverage_rate with
// sample sufficiency.
func buildVXHeldoutMetrics() map[string]any {
	return map[string]any{
		"version":         "seq18_p131.v1",
		"role":            "vx_heldout_metrics",
		"truth_authority": false,
		"metrics": []string{
			"retention_rate",
			"false_negative_rate",
			"full_coverage_rate",
		},
		"sample_sufficiency_required": true,
		"state_rules": []map[string]any{
			{"state": "pending", "action": "hold", "trigger": "missing_evidence"},
			{"state": "pending", "action": "hold", "trigger": "thin_evidence"},
			{"state": "blocked", "action": "hold", "trigger": "completeness_failure"},
			{"state": "ready", "action": "promote_candidate", "trigger": "sufficient_held_out_evidence"},
		},
		"policy_version": "vx18b.v1",
		"mode":           "vx_heldout_metrics_surface",
	}
}

// buildVXHeldoutThresholdReuse defines the VX heldout threshold reuse
// surface for SEQ-18-P132: reuses LC1P healthy completeness floor + existing
// _U1E_CAPTURED_REPLAY_MIN_* sample thresholds.
func buildVXHeldoutThresholdReuse() map[string]any {
	return map[string]any{
		"version":                      "seq18_p132.v1",
		"role":                         "vx_heldout_threshold_reuse",
		"truth_authority":              false,
		"completeness_floor":           "LC1P_healthy",
		"sample_threshold_source":      "_U1E_CAPTURED_REPLAY_MIN",
		"disconnected_literal_avoided": true,
		"policy_version":               "vx18b.v1",
		"mode":                         "vx_heldout_threshold_reuse_surface",
	}
}

// buildVXHeldoutCompletenessTest defines the VX heldout completeness test
// surface for SEQ-18-P133: guards pending-without, pending-insufficient, blocked-below,
// ready-sufficient states.
func buildVXHeldoutCompletenessTest() map[string]any {
	return map[string]any{
		"version":         "seq18_p133.v1",
		"role":            "vx_heldout_completeness_test",
		"truth_authority": false,
		"test_file":       "backend/test_step18_heldout_completeness_gate.py",
		"guards": []string{
			"pending_without_evidence",
			"pending_with_insufficient_samples",
			"blocked_below_threshold",
			"ready_with_sufficient_coverage",
		},
		"policy_version": "vx18b.v1",
		"mode":           "vx_heldout_completeness_test_surface",
	}
}

// buildVXLatencyTokenBudgetGate defines the VX latency/token-budget
// validation gate surface for SEQ-18-P140: vx18c.v1 additive validation_gates.
func buildVXLatencyTokenBudgetGate() map[string]any {
	return map[string]any{
		"version":             "seq18_p140.v1",
		"role":                "vx_latency_token_budget_gate",
		"truth_authority":     false,
		"gate_version":        "vx18c.v1",
		"gate_name":           "latency_token_budget",
		"execution_unchanged": true,
		"routing_unchanged":   true,
		"additive_only":       true,
		"policy_version":      "vx18c.v1",
		"mode":                "vx_latency_token_budget_gate_surface",
	}
}

// buildVXLatencyTokenMetrics defines the VX latency/token metrics surface
// for SEQ-18-P141: baseline_latency_proxy_ms, candidate_latency_proxy_ms,
// candidate_token_budget_chars with sample sufficiency.
func buildVXLatencyTokenMetrics() map[string]any {
	return map[string]any{
		"version":         "seq18_p141.v1",
		"role":            "vx_latency_token_metrics",
		"truth_authority": false,
		"metrics": []string{
			"baseline_latency_proxy_ms",
			"candidate_latency_proxy_ms",
			"candidate_token_budget_chars",
		},
		"sample_sufficiency_required": true,
		"default_token_ceiling":       "packet_budget_policy.max_injection_chars",
		"policy_version":              "vx18c.v1",
		"mode":                        "vx_latency_token_metrics_surface",
	}
}

// buildVXLatencyTokenThresholdReuse defines the VX latency/token threshold
// reuse surface for SEQ-18-P142: reuses _LC1M_MAX_SPLIT_LATENCY_MULTIPLIER for
// latency ceiling, token ratio in one gate-owner constant.
func buildVXLatencyTokenThresholdReuse() map[string]any {
	return map[string]any{
		"version":                    "seq18_p142.v1",
		"role":                       "vx_latency_token_threshold_reuse",
		"truth_authority":            false,
		"latency_ceiling_source":     "_LC1M_MAX_SPLIT_LATENCY_MULTIPLIER",
		"token_ratio_owner":          "gate_constant",
		"scattered_literals_avoided": true,
		"policy_version":             "vx18c.v1",
		"mode":                       "vx_latency_token_threshold_reuse_surface",
	}
}

// buildVXLatencyTokenTest defines the VX latency/token budget test
// surface for SEQ-18-P143: guards pending-without, pending-insufficient,
// blocked-ceiling, ready-within states.
func buildVXLatencyTokenTest() map[string]any {
	return map[string]any{
		"version":         "seq18_p143.v1",
		"role":            "vx_latency_token_test",
		"truth_authority": false,
		"test_file":       "backend/test_step18_latency_token_budget_gate.py",
		"guards": []string{
			"pending_without_evidence",
			"pending_with_insufficient_samples",
			"blocked_ceiling_exceeded",
			"ready_within_ceiling",
		},
		"policy_version": "vx18c.v1",
		"mode":           "vx_latency_token_test_surface",
	}
}

// buildVXTruthBoundaryGate defines the VX truth-boundary replay validation
// gate surface for SEQ-18-P150: vx18d.v1 additive validation_gates.truth_boundary_replay.
func buildVXTruthBoundaryGate() map[string]any {
	return map[string]any{
		"version":         "seq18_p150.v1",
		"role":            "vx_truth_boundary_gate",
		"truth_authority": false,
		"gate_version":    "vx18d.v1",
		"gate_name":       "truth_boundary_replay",
		"evaluates_after": "injection_pack_data.packet_composition",
		"additive_only":   true,
		"policy_version":  "vx18d.v1",
		"mode":            "vx_truth_boundary_gate_surface",
	}
}

// buildVXTruthBoundaryPrecedence defines the VX truth-boundary precedence
// surface for SEQ-18-P151: evaluates candidate_section_order and
// support_surface_priority against baseline with _LC1K_HIGH_AUTHORITY_SOURCES /
// _LC1K_LOWER_TIER_SOURCES precedence model.
func buildVXTruthBoundaryPrecedence() map[string]any {
	return map[string]any{
		"version":         "seq18_p151.v1",
		"role":            "vx_truth_boundary_precedence",
		"truth_authority": false,
		"evaluated_fields": []string{
			"candidate_section_order",
			"support_surface_priority",
		},
		"precedence_model": "_LC1K_HIGH_AUTHORITY_SOURCES_vs_LOWER_TIER_SOURCES",
		"policy_version":   "vx18d.v1",
		"mode":             "vx_truth_boundary_precedence_surface",
	}
}

// buildVXTruthBoundaryStates defines the VX truth-boundary states surface
// for SEQ-18-P152: pending/hold, blocked/hold, ready/promote_candidate.
func buildVXTruthBoundaryStates() map[string]any {
	return map[string]any{
		"version":         "seq18_p152.v1",
		"role":            "vx_truth_boundary_states",
		"truth_authority": false,
		"state_machine": []map[string]any{
			{"state": "pending", "action": "hold", "trigger": "missing_truth_boundary_evidence"},
			{"state": "pending", "action": "hold", "trigger": "insufficient_samples"},
			{"state": "blocked", "action": "hold", "trigger": "lost_support_lane_markers"},
			{"state": "ready", "action": "promote_candidate", "trigger": "preserved_canon_support_precedence"},
		},
		"policy_version": "vx18d.v1",
		"mode":           "vx_truth_boundary_states_surface",
	}
}

// buildVXTruthBoundaryTest defines the VX truth-boundary test surface for
// SEQ-18-P153: guards pending-without, pending-insufficient, blocked-support-loss,
// ready-preserved-boundary states.
func buildVXTruthBoundaryTest() map[string]any {
	return map[string]any{
		"version":         "seq18_p153.v1",
		"role":            "vx_truth_boundary_test",
		"truth_authority": false,
		"test_file":       "backend/test_step18_truth_boundary_replay_gate.py",
		"guards": []string{
			"pending_without_evidence",
			"pending_with_insufficient_samples",
			"blocked_support_lane_loss",
			"ready_preserved_boundary",
		},
		"policy_version": "vx18d.v1",
		"mode":           "vx_truth_boundary_test_surface",
	}
}

// ---------------------------------------------------------------------------
// SEQ-18 VX truncation_summary_loss gate surfaces (P160 ~ P164)
// ---------------------------------------------------------------------------

// buildVXTruncationSummaryLossGate defines the truncation_summary_loss
// validation gate surface for SEQ-18-P160: vx18e.v1 additive gate that emits
// validation_gates.truncation_summary_loss without changing retrieval execution.
func buildVXTruncationSummaryLossGate() map[string]any {
	return map[string]any{
		"version":         "seq18_p160.v1",
		"role":            "vx_truncation_summary_loss_gate",
		"truth_authority": false,
		"gate_version":    "vx18e.v1",
		"gate_name":       "truncation_summary_loss",
		"evaluates_after": "injection_pack_data.packet_composition",
		"additive_only":   true,
		"policy_version":  "vx18e.v1",
		"mode":            "vx_truncation_summary_loss_gate_surface",
	}
}

// buildVXTruncationSummaryLossMetrics defines the truncation_summary_loss
// metrics surface for SEQ-18-P161: evaluates baseline/candidate tail_fact_miss_rate
// and summary_loss_rate together with sample sufficiency, carrying existing
// tail_budget_promoted trace into gate evidence.
func buildVXTruncationSummaryLossMetrics() map[string]any {
	return map[string]any{
		"version":         "seq18_p161.v1",
		"role":            "vx_truncation_summary_loss_metrics",
		"truth_authority": false,
		"metrics": []string{
			"baseline_tail_fact_miss_rate",
			"candidate_tail_fact_miss_rate",
			"baseline_summary_loss_rate",
			"candidate_summary_loss_rate",
		},
		"sample_sufficiency_required": true,
		"trace_carry_in": []string{
			"tail_budget_promoted",
			"tail_budget_reason",
			"tail_budget_score_gap",
		},
		"policy_version": "vx18e.v1",
		"mode":           "vx_truncation_summary_loss_metrics_surface",
	}
}

// buildVXTruncationSummaryLossThresholdReuse defines the threshold reuse
// surface for SEQ-18-P162: reuses existing _U1E_CAPTURED_REPLAY_MIN_* sample
// thresholds and keeps truncation regression thresholds in one gate-owner constant
// block instead of scattering held-out delta literals.
func buildVXTruncationSummaryLossThresholdReuse() map[string]any {
	return map[string]any{
		"version":                    "seq18_p162.v1",
		"role":                       "vx_truncation_summary_loss_threshold_reuse",
		"truth_authority":            false,
		"sample_threshold_source":    "_U1E_CAPTURED_REPLAY_MIN_*",
		"threshold_owner":            "gate_constant_block",
		"scattered_literals_avoided": true,
		"policy_version":             "vx18e.v1",
		"mode":                       "vx_truncation_summary_loss_threshold_reuse_surface",
	}
}

// buildVXTruncationSummaryLossStates defines the truncation_summary_loss
// state machine surface for SEQ-18-P163: guards pending-without-evidence,
// pending-with-insufficient-samples, blocked-regression, ready-non-regression.
func buildVXTruncationSummaryLossStates() map[string]any {
	return map[string]any{
		"version":         "seq18_p163.v1",
		"role":            "vx_truncation_summary_loss_states",
		"truth_authority": false,
		"state_machine": []map[string]any{
			{"state": "pending", "action": "hold", "trigger": "missing_truncation_summary_loss_evidence"},
			{"state": "pending", "action": "hold", "trigger": "insufficient_samples"},
			{"state": "blocked", "action": "hold", "trigger": "regression_detected"},
			{"state": "ready", "action": "promote_candidate", "trigger": "no_regression"},
		},
		"policy_version": "vx18e.v1",
		"mode":           "vx_truncation_summary_loss_states_surface",
	}
}

// buildVXTruncationSummaryLossTest defines the truncation_summary_loss
// test surface for SEQ-18-P164: combined Step 18 backend regression bundle green
// after 18-4e (hybrid scoring, q3a query-class metadata, all VX gates passed).
func buildVXTruncationSummaryLossTest() map[string]any {
	return map[string]any{
		"version":         "seq18_p164.v1",
		"role":            "vx_truncation_summary_loss_test",
		"truth_authority": false,
		"test_file":       "backend/test_step18_truncation_summary_loss_gate.py",
		"guards": []string{
			"pending_without_evidence",
			"pending_with_insufficient_samples",
			"blocked_regression",
			"ready_non_regression",
		},
		"combined_bundle_status": "green",
		"bundle_components": []string{
			"hybrid_scoring",
			"q3a_query_class_metadata",
			"all_vx_gates",
		},
		"policy_version": "vx18e.v1",
		"mode":           "vx_truncation_summary_loss_test_surface",
	}
}

// ---------------------------------------------------------------------------
// SEQ-18 Post-Chroma / pre-release / VR/HY/QR/VX summary surfaces (P327 ~ P369)
// ---------------------------------------------------------------------------

// buildPostChromaTop1ScopedVerbatim defines the Post-Chroma Top 1 summary
// surface for SEQ-18-P327: scoped verbatim recall lane evidence.
