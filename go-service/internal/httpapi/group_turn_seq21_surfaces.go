package httpapi

// ---------------------------------------------------------------------------
// SEQ-21 surfaces — Beta 1.2 Post-Chroma Selective Rerank + Retrieval Economics
// ---------------------------------------------------------------------------
// This file contains builders for SEQ-21 selective rerank and retrieval
// economics surfaces (P9~P202).
//
// Step 21 is retrieval economics hardening on top of Step 18~20 substrate:
//   - 21-1: selective rerank gate (SR)
//   - 21-2: retrieval economics / budget / latency envelope (RE)
//
// All surfaces are contract-only / support-only and do not overwrite Step 19
// canonical truth. MariaDB remains canonical truth authority; ChromaDB is shadow
// accelerator only.
// ---------------------------------------------------------------------------

// ===========================================================================
// SEQ-21 Preparatory reset/admin surfaces (P9 ~ P11)
// ===========================================================================

// buildSeq21ResetAdminNote exposes the reset administration note surface for
// SEQ-21-P9. This is preparatory/document-only; no production code is changed.
func buildSeq21ResetAdminNote() map[string]any {
	return map[string]any{
		"version":         "s21-p9.v1",
		"role":            "seq21_reset_admin_note",
		"truth_authority": false,
		"sub_step":        "preparatory",
		"note":            "Existing checked checklist items in this file were cleared for redo.",
		"action_taken":    "reset_cleared",
		"policy_version":  "s21-sr.v1",
		"mode":            "seq21_reset_admin_note_definition",
	}
}

// buildSeq21HistoricalContentPreserved exposes the historical content
// preservation note surface for SEQ-21-P10. Preparatory/document-only.
func buildSeq21HistoricalContentPreserved() map[string]any {
	return map[string]any{
		"version":         "s21-p10.v1",
		"role":            "seq21_historical_content_preserved",
		"truth_authority": false,
		"sub_step":        "preparatory",
		"note":            "Historical content in this file was preserved; no step text was deleted.",
		"action_taken":    "preserved",
		"policy_version":  "s21-sr.v1",
		"mode":            "seq21_historical_content_preserved_definition",
	}
}

// buildSeq21ResetNoteOnly exposes the reset work-only note surface for
// SEQ-21-P11. Preparatory/document-only.
func buildSeq21ResetNoteOnly() map[string]any {
	return map[string]any{
		"version":         "s21-p11.v1",
		"role":            "seq21_reset_note_only",
		"truth_authority": false,
		"sub_step":        "preparatory",
		"note":            "This note records document reset work only, not revalidation of the step itself.",
		"action_taken":    "document_only",
		"policy_version":  "s21-sr.v1",
		"mode":            "seq21_reset_note_only_definition",
	}
}

// ===========================================================================
// SEQ-21 Six-criteria summary surfaces (P181 ~ P186)
// ===========================================================================

// buildSeq21P181RerankClassSummary exposes the rerank class criterion summary
// for SEQ-21-P181: rerank stays exceptional, not default.
func buildSeq21P181RerankClassSummary() map[string]any {
	return map[string]any{
		"version":         "s21-p181.v1",
		"role":            "seq21_rerank_class_summary",
		"truth_authority": false,
		"sub_step":        "criteria_summary",
		"criterion":       "rerank_restraint",
		"summary":         "rerank is exceptional, not default; only temporal/callback/resume/canon classes are gated",
		"policy_version":  "s21-sr.v1",
		"mode":            "seq21_rerank_class_summary_definition",
	}
}

// buildSeq21P182BudgetConfigSummary exposes the budget/latency configuration
// criterion summary for SEQ-21-P182.
func buildSeq21P182BudgetConfigSummary() map[string]any {
	return map[string]any{
		"version":         "s21-p182.v1",
		"role":            "seq21_budget_config_summary",
		"truth_authority": false,
		"sub_step":        "criteria_summary",
		"criterion":       "budget_durability",
		"summary":         "recall improvement must not break latency/token budget; degrade path is latency_or_budget_hold_then_fail_open",
		"policy_version":  "s21-re.v1",
		"mode":            "seq21_budget_config_summary_definition",
	}
}

// buildSeq21P183FailureClassSplitSummary exposes the failure-class split
// criterion summary for SEQ-21-P183.
func buildSeq21P183FailureClassSplitSummary() map[string]any {
	return map[string]any{
		"version":         "s21-p183.v1",
		"role":            "seq21_failure_class_split_summary",
		"truth_authority": false,
		"sub_step":        "criteria_summary",
		"criterion":       "failure_class_separation",
		"summary":         "miss types are named before tuned: temporal_miss, callback_miss, canon_conflict, alias_confusion",
		"policy_version":  "s21-re.v1",
		"mode":            "seq21_failure_class_split_summary_definition",
	}
}

// buildSeq21P184HeldOutHygieneSummary exposes the dev/held-out hygiene
// criterion summary for SEQ-21-P184.
func buildSeq21P184HeldOutHygieneSummary() map[string]any {
	return map[string]any{
		"version":         "s21-p184.v1",
		"role":            "seq21_held_out_hygiene_summary",
		"truth_authority": false,
		"sub_step":        "criteria_summary",
		"criterion":       "dev_held_out_split",
		"summary":         "tuning uses dev split; held-out confirmation is separate; no single-replay default promotion",
		"policy_version":  "s21-re.v1",
		"mode":            "seq21_held_out_hygiene_summary_definition",
	}
}

// buildSeq21P185TruthBoundaryPreserveSummary exposes the truth boundary
// preservation criterion summary for SEQ-21-P185.
func buildSeq21P185TruthBoundaryPreserveSummary() map[string]any {
	return map[string]any{
		"version":         "s21-p185.v1",
		"role":            "seq21_truth_boundary_preserve_summary",
		"truth_authority": false,
		"sub_step":        "criteria_summary",
		"criterion":       "truth_boundary_intact",
		"summary":         "rerank does not become truth arbiter; MariaDB canonical truth precedence is preserved",
		"policy_version":  "s21-sr.v1",
		"mode":            "seq21_truth_boundary_preserve_summary_definition",
	}
}

// buildSeq21P186DensityDisciplineSummary exposes the density discipline
// criterion summary for SEQ-21-P186.
func buildSeq21P186DensityDisciplineSummary() map[string]any {
	return map[string]any{
		"version":         "s21-p186.v1",
		"role":            "seq21_density_discipline_summary",
		"truth_authority": false,
		"sub_step":        "criteria_summary",
		"criterion":       "density_tier_budget",
		"summary":         "heavy packet and light tag are budget surfaces, not authority stratification",
		"policy_version":  "s21-re.v1",
		"mode":            "seq21_density_discipline_summary_definition",
	}
}

// ===========================================================================
// SEQ-21 21-1 selective rerank gate surfaces (P190 ~ P193)
// ===========================================================================

// buildSeq21P190RerankTriggerClass exposes the 21-1a selective rerank trigger
// class contract surface for SEQ-21-P190.
func buildSeq21P190RerankTriggerClass() map[string]any {
	return map[string]any{
		"version":         "s21-p190.v1",
		"role":            "seq21_rerank_trigger_class",
		"truth_authority": false,
		"sub_step":        "21-1a",
		"trigger_classes": []string{
			"temporal_ambiguity",
			"dense_tie",
			"canon_conflict",
			"vocabulary_gap",
		},
		"denied_classes": []string{
			"scene",
		},
		"allowed_query_classes": []string{
			"temporal",
			"callback",
			"resume",
			"canon",
		},
		"policy_version": "s21-sr.v1",
		"mode":           "seq21_rerank_trigger_class_definition",
	}
}

// buildSeq21P191RerankSupportOnlySchema exposes the 21-1b rerank input/output
// support-only schema contract surface for SEQ-21-P191.
func buildSeq21P191RerankSupportOnlySchema() map[string]any {
	return map[string]any{
		"version":         "s21-p191.v1",
		"role":            "seq21_rerank_support_only_schema",
		"truth_authority": false,
		"sub_step":        "21-1b",
		"input_surfaces": []string{
			"query_class_contract",
			"ann_candidate_snapshot",
		},
		"output_fields": []string{
			"support_lane_status",
			"support_lane_summary",
		},
		"truth_write_mode":          "forbidden",
		"canonical_truth_authority": "mariadb_canonical_precedence",
		"policy_version":            "s21-sr.v1",
		"mode":                      "seq21_rerank_support_only_schema_definition",
	}
}

// buildSeq21P192RerankOffFallback exposes the 21-1c rerank off/fallback rule
// contract surface for SEQ-21-P192.
func buildSeq21P192RerankOffFallback() map[string]any {
	return map[string]any{
		"version":         "s21-p192.v1",
		"role":            "seq21_rerank_off_fallback",
		"truth_authority": false,
		"sub_step":        "21-1c",
		"off_states": []string{
			"off",
			"inactive",
			"gated_off",
		},
		"inactive_reason":   "no_bounded_trigger",
		"gated_off_reason":  "ann_snapshot_not_ready",
		"fallback_boundary": "hybrid_recall_fail_open",
		"default_takeover":  false,
		"policy_version":    "s21-sr.v1",
		"mode":              "seq21_rerank_off_fallback_definition",
	}
}

// buildSeq21P193RerankNearMissTrigger exposes the 21-1d top-k near-miss /
// sparse callback / canon-vs-support tie trigger surface for SEQ-21-P193.
func buildSeq21P193RerankNearMissTrigger() map[string]any {
	return map[string]any{
		"version":         "s21-p193.v1",
		"role":            "seq21_rerank_near_miss_trigger",
		"truth_authority": false,
		"sub_step":        "21-1d",
		"trigger_surfaces": []string{
			"top_k_near_miss",
			"sparse_callback",
			"canon_support_tie",
		},
		"top_k_near_miss_source":   "ann_candidate_snapshot.filtered_out_total",
		"sparse_callback_source":   "recall_cue_rescue_rule.callback_rescue_enabled",
		"canon_support_tie_source": "canonical_pending_stale_current_conflict_note.source_precedence",
		"rescue_only":              true,
		"policy_version":           "s21-sr.v1",
		"mode":                     "seq21_rerank_near_miss_trigger_definition",
	}
}

// ===========================================================================
// SEQ-21 21-2 retrieval economics surfaces (P197 ~ P202)
// ===========================================================================

// buildSeq21P197QueryClassCandidateCap exposes the 21-2a query-class candidate
// cap contract surface for SEQ-21-P197.
func buildSeq21P197QueryClassCandidateCap() map[string]any {
	return map[string]any{
		"version":         "s21-p197.v1",
		"role":            "seq21_query_class_candidate_cap",
		"truth_authority": false,
		"sub_step":        "21-2a",
		"budget_owner":    "query_class_budget_policy",
		"packet_owner":    "packet_budget_policy",
		"query_class_caps": map[string]any{
			"scene":    map[string]any{"cap": 0, "reason": "rerank_denied"},
			"callback": map[string]any{"cap": 8, "reason": "standard"},
			"resume":   map[string]any{"cap": 8, "reason": "standard"},
			"canon":    map[string]any{"cap": 8, "reason": "standard"},
			"temporal": map[string]any{"cap": 12, "reason": "evidence_first_overlay"},
		},
		"support_only":   true,
		"policy_version": "s21-re.v1",
		"mode":           "seq21_query_class_candidate_cap_definition",
	}
}

// buildSeq21P198LatencyBudgetDegrade exposes the 21-2b latency budget /
// timeout / degrade rule contract surface for SEQ-21-P198.
func buildSeq21P198LatencyBudgetDegrade() map[string]any {
	return map[string]any{
		"version":                  "s21-p198.v1",
		"role":                     "seq21_latency_budget_degrade",
		"truth_authority":          false,
		"sub_step":                 "21-2b",
		"latency_ceiling_source":   "vx18c_upstream_gate",
		"token_ceiling_source":     "vx18c_upstream_gate",
		"timeout_budget_ms_source": "ann_candidate_snapshot.benchmark.timeout_budget_ms",
		"degrade_path": []string{
			"latency_or_budget_hold_then_fail_open",
			"hybrid_recall_fail_open",
		},
		"support_only":   true,
		"policy_version": "s21-re.v1",
		"mode":           "seq21_latency_budget_degrade_definition",
	}
}

// buildSeq21P199RetrievalCacheReuse exposes the 21-2c retrieval cache / reuse /
// invalidation rule contract surface for SEQ-21-P199.
func buildSeq21P199RetrievalCacheReuse() map[string]any {
	return map[string]any{
		"version":         "s21-p199.v1",
		"role":            "seq21_retrieval_cache_reuse",
		"truth_authority": false,
		"sub_step":        "21-2c",
		"runtime_mode":    "shadow_off",
		"reuse_surface":   "retrieval_index_registry.snapshot",
		"refresh_event":   "prepare_turn_refresh",
		"invalidation_paths": []string{
			"mark_dirty",
			"rollback_discard",
		},
		"support_only":   true,
		"policy_version": "s21-re.v1",
		"mode":           "seq21_retrieval_cache_reuse_definition",
	}
}

// buildSeq21P200FailureClassAdaptiveCap exposes the 21-2d failure-class
// adaptive cap contract surface for SEQ-21-P200.
func buildSeq21P200FailureClassAdaptiveCap() map[string]any {
	return map[string]any{
		"version":                "s21-p200.v1",
		"role":                   "seq21_failure_class_adaptive_cap",
		"truth_authority":        false,
		"sub_step":               "21-2d",
		"cap_separation":         "raw_profile_cap_vs_active_top_k",
		"non_shrinking_baseline": true,
		"failure_class_profiles": map[string]any{
			"temporal_miss":   map[string]any{"raw_cap": 12, "active_cap": 12},
			"callback_miss":   map[string]any{"raw_cap": 10, "active_cap": 10},
			"canon_conflict":  map[string]any{"raw_cap": 8, "active_cap": 8},
			"alias_confusion": map[string]any{"raw_cap": 8, "active_cap": 8},
		},
		"support_only":   true,
		"policy_version": "s21-re.v1",
		"mode":           "seq21_failure_class_adaptive_cap_definition",
	}
}

// buildSeq21P201DualDensityDeliveryBudget exposes the 21-2e dual-density
// delivery budget contract surface for SEQ-21-P201.
func buildSeq21P201DualDensityDeliveryBudget() map[string]any {
	return map[string]any{
		"version":         "s21-p201.v1",
		"role":            "seq21_dual_density_delivery_budget",
		"truth_authority": false,
		"sub_step":        "21-2e",
		"heavy_packet": map[string]any{
			"source":      "wide_gather_validity_join_rule + primary_support_surface_caps",
			"char_budget": 2048,
			"density":     "full_support_surface",
		},
		"light_tag": map[string]any{
			"source":      "thin_support_tag_fallback",
			"char_budget": 0,
			"density":     "metadata_only",
		},
		"support_only":   true,
		"policy_version": "s21-re.v1",
		"mode":           "seq21_dual_density_delivery_budget_definition",
	}
}

// buildSeq21P202HeavyPromotionRule exposes the 21-2f heavy promotion rule
// contract surface for SEQ-21-P202.
func buildSeq21P202HeavyPromotionRule() map[string]any {
	return map[string]any{
		"version":         "s21-p202.v1",
		"role":            "seq21_heavy_promotion_rule",
		"truth_authority": false,
		"sub_step":        "21-2f",
		"promotion_prerequisites": []string{
			"active_qualifying_failure_class",
			"strong_temporal_relation_signal",
			"pending_current_or_canonical_anchor",
		},
		"weak_linkage_blockers": []string{
			"thin_tag_only",
			"no_active_failure_class",
			"missing_temporal_or_relation_signal",
		},
		"auto_promote_without_failure_class": false,
		"support_only":                       true,
		"policy_version":                     "s21-re.v1",
		"mode":                               "seq21_heavy_promotion_rule_definition",
	}
}

// ===========================================================================
// SEQ-21 21-3 failure-class tuning loop surfaces (P206 ~ P209)
// ===========================================================================

// buildSeq21P206FailureTaxonomy exposes the 21-3a failure taxonomy contract
// surface for SEQ-21-P206.
func buildSeq21P206FailureTaxonomy() map[string]any {
	return map[string]any{
		"version":         "s21-p206.v1",
		"role":            "seq21_failure_taxonomy",
		"truth_authority": false,
		"sub_step":        "21-3a",
		"failure_classes": []string{
			"temporal_miss",
			"callback_miss",
			"canon_conflict",
			"alias_confusion",
		},
		"named_from_existing_surfaces": []string{
			"temporal_ambiguity_support_note",
			"recall_cue_rescue_rule.callback_rescue_enabled",
			"canonical_pending_stale_current_conflict_note",
			"alias_entity_conflict_disambiguation",
		},
		"support_only":   true,
		"policy_version": "s21-ft.v1",
		"mode":           "seq21_failure_taxonomy_definition",
	}
}

// buildSeq21P207DevSplitTuningLoop exposes the 21-3b dev split tuning loop
// contract surface for SEQ-21-P207.
func buildSeq21P207DevSplitTuningLoop() map[string]any {
	return map[string]any{
		"version":         "s21-p207.v1",
		"role":            "seq21_dev_split_tuning_loop",
		"truth_authority": false,
		"sub_step":        "21-3b",
		"dev_split":       true,
		"replay_owners": []string{
			"u1e_captured_session_replay",
			"vx18a_hybrid_replay",
			"vx18b_held_out_completeness",
			"vx18c_latency_token_budget",
			"vx18d_truth_boundary_replay",
			"vx20g_step20_non_regression",
		},
		"default_promotion_stays_pending": true,
		"support_only":                    true,
		"policy_version":                  "s21-ft.v1",
		"mode":                            "seq21_dev_split_tuning_loop_definition",
	}
}

// buildSeq21P208HeldOutConfirmationGate exposes the 21-3c held-out
// confirmation gate contract surface for SEQ-21-P208.
func buildSeq21P208HeldOutConfirmationGate() map[string]any {
	return map[string]any{
		"version":           "s21-p208.v1",
		"role":              "seq21_held_out_confirmation_gate",
		"truth_authority":   false,
		"sub_step":          "21-3c",
		"held_out_separate": true,
		"confirmation_requires": []string{
			"dev_tuning_replay_green",
			"non_regression_gate_green",
			"cost_gate_green",
		},
		"no_single_replay_promotion": true,
		"support_only":               true,
		"policy_version":             "s21-ft.v1",
		"mode":                       "seq21_held_out_confirmation_gate_definition",
	}
}

// buildSeq21P209ResidualLongTailLoop exposes the 21-3d residual long-tail miss
// tuning loop contract surface for SEQ-21-P209.
func buildSeq21P209ResidualLongTailLoop() map[string]any {
	return map[string]any{
		"version":         "s21-p209.v1",
		"role":            "seq21_residual_long_tail_loop",
		"truth_authority": false,
		"sub_step":        "21-3d",
		"reuse_surfaces": []string{
			"query_class_route_policy.long_tail_route_active",
			"top_k_near_miss",
			"sparse_callback",
		},
		"residual_only":  true,
		"support_only":   true,
		"policy_version": "s21-ft.v1",
		"mode":           "seq21_residual_long_tail_loop_definition",
	}
}

// ===========================================================================
// SEQ-21 21-4 validation/adoption gate surfaces (P213 ~ P219)
// ===========================================================================

// buildSeq21P213CostVsGainReplay exposes the 21-4a rerank cost-vs-gain replay
// contract surface for SEQ-21-P213.
func buildSeq21P213CostVsGainReplay() map[string]any {
	return map[string]any{
		"version":         "s21-p213.v1",
		"role":            "seq21_cost_vs_gain_replay",
		"truth_authority": false,
		"sub_step":        "21-4a",
		"aggregates": []string{
			"failure_class_adaptive_caps",
			"dual_density_delivery_budget",
			"dev_split_tuning_loop",
		},
		"cost_side":      "replay_surface_only",
		"support_only":   true,
		"policy_version": "s21-vx.v1",
		"mode":           "seq21_cost_vs_gain_replay_definition",
	}
}

// buildSeq21P214LatencyTokenEnvelopeReplay exposes the 21-4b latency/token
// envelope replay contract surface for SEQ-21-P214.
func buildSeq21P214LatencyTokenEnvelopeReplay() map[string]any {
	return map[string]any{
		"version":              "s21-p214.v1",
		"role":                 "seq21_latency_token_envelope_replay",
		"truth_authority":      false,
		"sub_step":             "21-4b",
		"binds_to":             "vx18c_upstream_gate",
		"envelope_surface":     "latency_token_budget_replay",
		"no_new_latency_owner": true,
		"support_only":         true,
		"policy_version":       "s21-vx.v1",
		"mode":                 "seq21_latency_token_envelope_replay_definition",
	}
}

// buildSeq21P215HeldOutRegressionGate exposes the 21-4c held-out regression
// gate contract surface for SEQ-21-P215.
func buildSeq21P215HeldOutRegressionGate() map[string]any {
	return map[string]any{
		"version":            "s21-p215.v1",
		"role":               "seq21_held_out_regression_gate",
		"truth_authority":    false,
		"sub_step":           "21-4c",
		"reuses_tuning_loop": true,
		"regression_checks": []string{
			"held_out_completeness",
			"non_regression_gate",
			"truth_boundary_intact",
		},
		"support_only":   true,
		"policy_version": "s21-vx.v1",
		"mode":           "seq21_held_out_regression_gate_definition",
	}
}

// buildSeq21P216PostChromaDefaultPromotionCriteria exposes the 21-4d
// post-Chroma default promotion criteria contract surface for SEQ-21-P216.
func buildSeq21P216PostChromaDefaultPromotionCriteria() map[string]any {
	return map[string]any{
		"version":          "s21-p216.v1",
		"role":             "seq21_post_chroma_default_promotion_criteria",
		"truth_authority":  false,
		"sub_step":         "21-4d",
		"promotion_status": "hold",
		"promotion_requires": []string{
			"held_out_gate_green",
			"non_regression_gate_green",
			"cost_gate_green",
		},
		"no_deadlock_on_inactive_baseline": true,
		"support_only":                     true,
		"policy_version":                   "s21-vx.v1",
		"mode":                             "seq21_post_chroma_default_promotion_criteria_definition",
	}
}

// buildSeq21P217CostNormalizedTailRecallGate exposes the 21-4e cost-normalized
// tail-recall gate contract surface for SEQ-21-P217.
func buildSeq21P217CostNormalizedTailRecallGate() map[string]any {
	return map[string]any{
		"version":             "s21-p217.v1",
		"role":                "seq21_cost_normalized_tail_recall_gate",
		"truth_authority":     false,
		"sub_step":            "21-4e",
		"verification_target": "actual_tail_miss_reduction",
		"rejects": []string{
			"candidate_expansion_alone",
			"cost_increase_without_tail_miss_reduction",
		},
		"support_only":   true,
		"policy_version": "s21-vx.v1",
		"mode":           "seq21_cost_normalized_tail_recall_gate_definition",
	}
}

// buildSeq21P218DensityMixReplay exposes the 21-4f density-mix replay
// contract surface for SEQ-21-P218.
func buildSeq21P218DensityMixReplay() map[string]any {
	return map[string]any{
		"version":         "s21-p218.v1",
		"role":            "seq21_density_mix_replay",
		"truth_authority": false,
		"sub_step":        "21-4f",
		"replay_surfaces": []string{
			"heavy_packet_delivery_mode",
			"light_tag_delivery_mode",
			"heavy_promotion_readiness",
		},
		"token_ceiling_check":    true,
		"arc_monopoly_check":     true,
		"does_not_reopen_bridge": true,
		"support_only":           true,
		"policy_version":         "s21-vx.v1",
		"mode":                   "seq21_density_mix_replay_definition",
	}
}

// buildSeq21P219SharedRunnerCorpusRule exposes the 21-4g shared-runner /
// step-specific corpus rule contract surface for SEQ-21-P219.
func buildSeq21P219SharedRunnerCorpusRule() map[string]any {
	return map[string]any{
		"version":                     "s21-p219.v1",
		"role":                        "seq21_shared_runner_corpus_rule",
		"truth_authority":             false,
		"sub_step":                    "21-4g",
		"shared_runner_allowed":       true,
		"step21_corpus_isolated":      true,
		"adoption_checklist_isolated": true,
		"support_only":                true,
		"policy_version":              "s21-vx.v1",
		"mode":                        "seq21_shared_runner_corpus_rule_definition",
	}
}

// ===========================================================================
// SEQ-21 Beta 1.2 release gate surfaces (P223 ~ P227)
// ===========================================================================

// buildSeq21P223Beta12BundleDryRun exposes the Beta 1.2 bundle dry-run
// evidence-only surface for SEQ-21-P223. Actual bundle creation is forbidden.
func buildSeq21P223Beta12BundleDryRun() map[string]any {
	return map[string]any{
		"version":         "s21-p223.v1",
		"role":            "seq21_beta12_bundle_dry_run",
		"truth_authority": false,
		"sub_step":        "release_gate",
		"note":            "Dry-run evidence only; actual bundle artifact creation is forbidden by policy. Stable baseline green and Step 21.5 split are current close criteria.",
		"dry_run":         true,
		"policy_version":  "s21-vx.v1",
		"mode":            "seq21_beta12_bundle_dry_run_definition",
	}
}

// buildSeq21P224SelectiveRerankTriggerSmoke exposes the selective rerank
// trigger smoke check pass surface for SEQ-21-P224.
func buildSeq21P224SelectiveRerankTriggerSmoke() map[string]any {
	return map[string]any{
		"version":         "s21-p224.v1",
		"role":            "seq21_selective_rerank_trigger_smoke",
		"truth_authority": false,
		"sub_step":        "release_gate",
		"smoke_target":    "selective_rerank_trigger",
		"references": []string{
			"seq21_rerank_trigger_class",
			"seq21_rerank_support_only_schema",
			"seq21_rerank_off_fallback",
		},
		"policy_version": "s21-sr.v1",
		"mode":           "seq21_selective_rerank_trigger_smoke_definition",
	}
}

// buildSeq21P225CandidateBudgetLatencySmoke exposes the candidate budget /
// latency smoke check pass surface for SEQ-21-P225.
func buildSeq21P225CandidateBudgetLatencySmoke() map[string]any {
	return map[string]any{
		"version":         "s21-p225.v1",
		"role":            "seq21_candidate_budget_latency_smoke",
		"truth_authority": false,
		"sub_step":        "release_gate",
		"smoke_target":    "candidate_budget_latency",
		"references": []string{
			"seq21_query_class_candidate_cap",
			"seq21_latency_budget_degrade",
			"seq21_retrieval_cache_reuse",
		},
		"policy_version": "s21-re.v1",
		"mode":           "seq21_candidate_budget_latency_smoke_definition",
	}
}

// buildSeq21P226FailureClassTuningReviewChecklist exposes the failure-class
// tuning review checklist pass surface for SEQ-21-P226.
func buildSeq21P226FailureClassTuningReviewChecklist() map[string]any {
	return map[string]any{
		"version":         "s21-p226.v1",
		"role":            "seq21_failure_class_tuning_review_checklist",
		"truth_authority": false,
		"sub_step":        "release_gate",
		"checklist_items": []string{
			"failure_taxonomy_defined",
			"dev_split_tuning_loop_active",
			"held_out_confirmation_gate_present",
			"residual_long_tail_tracked",
		},
		"policy_version": "s21-ft.v1",
		"mode":           "seq21_failure_class_tuning_review_checklist_definition",
	}
}

// buildSeq21P227HeldOutCostAdoptionGateComplete exposes the held-out / cost /
// adoption gate complete surface for SEQ-21-P227.
func buildSeq21P227HeldOutCostAdoptionGateComplete() map[string]any {
	return map[string]any{
		"version":         "s21-p227.v1",
		"role":            "seq21_held_out_cost_adoption_gate_complete",
		"truth_authority": false,
		"sub_step":        "release_gate",
		"gate_status":     "complete",
		"required_evidence": []string{
			"held_out_regression_gate_green",
			"cost_vs_gain_replay_green",
			"density_mix_replay_green",
			"shared_runner_corpus_isolated",
		},
		"policy_version": "s21-vx.v1",
		"mode":           "seq21_held_out_cost_adoption_gate_complete_definition",
	}
}

// ===========================================================================
// SEQ-21 final preserve decision surfaces (P238 ~ P241)
// ===========================================================================

// buildSeq21P238BoundedTriggerClassesPreserve exposes the bounded trigger
// classes preserve surface for SEQ-21-P238.
func buildSeq21P238BoundedTriggerClassesPreserve() map[string]any {
	return map[string]any{
		"version":         "s21-p238.v1",
		"role":            "seq21_bounded_trigger_classes_preserve",
		"truth_authority": false,
		"sub_step":        "preserve_summary",
		"preserved": []string{
			"temporal_callback_resume_canon_classes_only",
			"bounded_trigger_surface",
			"scene_remains_off_path",
		},
		"policy_version": "s21-sr.v1",
		"mode":           "seq21_bounded_trigger_classes_preserve_definition",
	}
}

// buildSeq21P239QueryClassCandidateCapPreserve exposes the query-class
// candidate cap / shared top-k baseline preserve surface for SEQ-21-P239.
func buildSeq21P239QueryClassCandidateCapPreserve() map[string]any {
	return map[string]any{
		"version":         "s21-p239.v1",
		"role":            "seq21_query_class_candidate_cap_preserve",
		"truth_authority": false,
		"sub_step":        "preserve_summary",
		"preserved": []string{
			"query_class_budget_profile",
			"shared_top_k_baseline_never_shrinks",
		},
		"policy_version": "s21-re.v1",
		"mode":           "seq21_query_class_candidate_cap_preserve_definition",
	}
}

// buildSeq21P240LatencyDegradePathPreserve exposes the latency default degrade
// path preserve surface for SEQ-21-P240.
func buildSeq21P240LatencyDegradePathPreserve() map[string]any {
	return map[string]any{
		"version":         "s21-p240.v1",
		"role":            "seq21_latency_degrade_path_preserve",
		"truth_authority": false,
		"sub_step":        "preserve_summary",
		"preserved": []string{
			"latency_or_budget_hold_then_fail_open",
			"hybrid_recall_fail_open",
		},
		"policy_version": "s21-re.v1",
		"mode":           "seq21_latency_degrade_path_preserve_definition",
	}
}

// buildSeq21P241TuningDeferredPreserve exposes the automatic tuning deferred
// until gates green preserve surface for SEQ-21-P241.
func buildSeq21P241TuningDeferredPreserve() map[string]any {
	return map[string]any{
		"version":         "s21-p241.v1",
		"role":            "seq21_tuning_deferred_preserve",
		"truth_authority": false,
		"sub_step":        "preserve_summary",
		"preserved": []string{
			"route_matrix_auto_apply_deferred",
			"held_out_cost_adoption_gate_green_required",
		},
		"policy_version": "s21-vx.v1",
		"mode":           "seq21_tuning_deferred_preserve_definition",
	}
}
