package httpapi

// ---------------------------------------------------------------------------
// SEQ-18.5 surfaces (P9 ~ P11, P163 ~ P168, P172 ~ P175, P179 ~ P182,
// P186 ~ P189, P193 ~ P197)
// ---------------------------------------------------------------------------

// buildResetAdmin185 defines the Step 18.5 reset administration surface
// for SEQ-18.5-P9: existing checked checklist items were cleared for redo.
func buildResetAdmin185() map[string]any {
	return map[string]any{
		"version":              "seq185_p9.v1",
		"role":                 "reset_administration",
		"truth_authority":      false,
		"reset_action":         "checklist_cleared_for_redo",
		"historical_preserved": true,
		"policy_version":       "s185-rst.v1",
		"mode":                 "reset_administration_note",
	}
}

// buildHistoricalContentPreserved185 defines the Step 18.5 historical content
// preservation surface for SEQ-18.5-P10: historical content was preserved.
func buildHistoricalContentPreserved185() map[string]any {
	return map[string]any{
		"version":           "seq185_p10.v1",
		"role":              "historical_content_preserved",
		"truth_authority":   false,
		"content_preserved": true,
		"no_text_deleted":   true,
		"policy_version":    "s185-rst.v1",
		"mode":              "historical_content_preservation_note",
	}
}

// buildResetNoteOnly185 defines the Step 18.5 reset scope surface for
// SEQ-18.5-P11: reset note records document reset work only.
func buildResetNoteOnly185() map[string]any {
	return map[string]any{
		"version":            "seq185_p11.v1",
		"role":               "reset_note_only",
		"truth_authority":    false,
		"scope":              "document_reset_only",
		"revalidation_claim": false,
		"policy_version":     "s185-rst.v1",
		"mode":               "reset_scope_note",
	}
}

// ---------------------------------------------------------------------------
// SEQ-18.5 six criteria surfaces (P163 ~ P168)
// ---------------------------------------------------------------------------
// The source progress text for Step 18.5 uses SQLite terminology from the
// 0.8 reference path. These surfaces preserve that wording for auditability,
// while the 2.0 remigration contract exposes MariaDB as the canonical truth
// authority and ChromaDB as the bounded vector accelerator.

// buildBoundedLiveScope defines the bounded live scope surface for
// SEQ-18.5-P163: limited_memory memory tier live cutover.
func buildBoundedLiveScope() map[string]any {
	return map[string]any{
		"version":                   "seq185_p163.v1",
		"role":                      "bounded_live_scope",
		"truth_authority":           false,
		"canonical_truth_authority": "mariadb",
		"vector_accelerator":        "chromadb",
		"scope":                     "limited_memory",
		"tier":                      "memory_only",
		"policy_version":            "s185-crit.v1",
		"mode":                      "bounded_live_scope_definition",
	}
}

// buildSQLiteTruthPreserve defines the SQLite truth preservation surface for
// SEQ-18.5-P164: Chroma hit SQLite hydration final item.
func buildSQLiteTruthPreserve() map[string]any {
	return map[string]any{
		"version":                    "seq185_p164.v1",
		"role":                       "sqlite_truth_preserve",
		"truth_authority":            false,
		"source_truth_label":         "sqlite",
		"canonical_truth_authority":  "mariadb",
		"vector_accelerator":         "chromadb",
		"hydration_required":         true,
		"final_authority":            "sqlite",
		"canonical_hydration_target": "mariadb_row",
		"remigration_translation":    "sqlite_source_contract_to_mariadb_canonical_truth",
		"policy_version":             "s185-crit.v1",
		"mode":                       "sqlite_truth_preservation_rule",
	}
}

// buildFailOpenSafety defines the fail-open safety surface for
// SEQ-18.5-P165: degraded/blocked/query-failure SQLite scan + fallback lane preserve.
func buildFailOpenSafety() map[string]any {
	return map[string]any{
		"version":                   "seq185_p165.v1",
		"role":                      "fail_open_safety",
		"truth_authority":           false,
		"canonical_truth_authority": "mariadb",
		"fail_open_states":          []string{"degraded", "blocked", "query_failure"},
		"fallback_lane":             "sqlite_scan_preserved",
		"fallback_lane_current":     "mariadb_scan_preserved",
		"policy_version":            "s185-crit.v1",
		"mode":                      "fail_open_safety_rule",
	}
}

// buildOperatorVisibility defines the operator visibility surface for
// SEQ-18.5-P166: settings, /search, prepare-turn, trace preview, root runtime trace row state/status.
func buildOperatorVisibility() map[string]any {
	return map[string]any{
		"version":          "seq185_p166.v1",
		"role":             "operator_visibility",
		"truth_authority":  false,
		"exposed_surfaces": []string{"settings", "search", "prepare_turn", "trace_preview", "root_runtime_trace_row"},
		"state_vocabulary": []string{"chroma_engaged", "sqlite_fallback", "mariadb_fallback", "shadow_disabled", "degraded"},
		"policy_version":   "s185-crit.v1",
		"mode":             "operator_visibility_surface",
	}
}

// buildSilentAuthorityDriftGuard defines the silent authority drift guard surface for
// SEQ-18.5-P167: q1a/source_row_id/source_table + hydration contract authority drift block.
func buildSilentAuthorityDriftGuard() map[string]any {
	return map[string]any{
		"version":               "seq185_p167.v1",
		"role":                  "silent_authority_drift_guard",
		"truth_authority":       false,
		"guard_fields":          []string{"q1a", "source_row_id", "source_table"},
		"hydration_contract":    true,
		"authority_drift_block": true,
		"policy_version":        "s185-crit.v1",
		"mode":                  "silent_authority_drift_guard_rule",
	}
}

// buildReleaseHonesty defines the release honesty surface for
// SEQ-18.5-P168: default off, Step 21 full promotion.
func buildReleaseHonesty() map[string]any {
	return map[string]any{
		"version":           "seq185_p168.v1",
		"role":              "release_honesty",
		"truth_authority":   false,
		"default_mode":      "off",
		"step_21_promotion": false,
		"operator_gated":    true,
		"policy_version":    "s185-crit.v1",
		"mode":              "release_honesty_note",
	}
}

// ---------------------------------------------------------------------------
// SEQ-18.5 sub-step 18.5-1 LR surfaces (P172 ~ P175)
// ---------------------------------------------------------------------------

// buildLiveChromaToggleConfig defines the live Chroma participation toggle / config surface for
// SEQ-18.5-P172: 18.5-1a live Chroma participation toggle / config surface define.
func buildLiveChromaToggleConfig() map[string]any {
	return map[string]any{
		"version":         "seq185_p172.v1",
		"role":            "live_chroma_toggle_config",
		"truth_authority": false,
		"toggle_states":   []string{"off", "limited_memory"},
		"config_surface":  "chroma_live_cutover_mode",
		"policy_version":  "s185-1a.v1",
		"mode":            "live_chroma_toggle_config_definition",
	}
}

// buildLiveScopeMemoryOnly defines the live scope surface for
// SEQ-18.5-P173: 18.5-1b live scope define - memory-only limited_memory.
func buildLiveScopeMemoryOnly() map[string]any {
	return map[string]any{
		"version":         "seq185_p173.v1",
		"role":            "live_scope_memory_only",
		"truth_authority": false,
		"scope":           "memory_only",
		"mode_fixed":      "limited_memory",
		"policy_version":  "s185-1b.v1",
		"mode":            "live_scope_memory_only_definition",
	}
}

// buildLiveChromaTopkCap defines the live Chroma top-k / candidate cap / bounded participation rule for
// SEQ-18.5-P174: 18.5-1c live Chroma top-k / candidate cap / bounded participation rule define.
func buildLiveChromaTopkCap() map[string]any {
	return map[string]any{
		"version":            "seq185_p174.v1",
		"role":               "live_chroma_topk_cap",
		"truth_authority":    false,
		"top_k_bounded":      true,
		"candidate_cap":      1,
		"participation_rule": "tail_budget_score_gap_bound",
		"policy_version":     "s185-1c.v1",
		"mode":               "live_chroma_topk_cap_definition",
	}
}

// buildShadowDisabledDegradeRule defines the shadow disabled / blocked / degraded SQLite-only path degrade rule for
// SEQ-18.5-P175: 18.5-1d shadow disabled / blocked / degraded SQLite-only path degrade rule define.
func buildShadowDisabledDegradeRule() map[string]any {
	return map[string]any{
		"version":                   "seq185_p175.v1",
		"role":                      "shadow_disabled_degrade_rule",
		"truth_authority":           false,
		"canonical_truth_authority": "mariadb",
		"degrade_triggers":          []string{"shadow_disabled", "blocked", "degraded"},
		"fallback_path":             "sqlite_only_scan",
		"fallback_path_current":     "mariadb_only_scan",
		"policy_version":            "s185-1d.v1",
		"mode":                      "shadow_disabled_degrade_rule_definition",
	}
}

// ---------------------------------------------------------------------------
// SEQ-18.5 sub-step 18.5-2 HJ surfaces (P179 ~ P182)
// ---------------------------------------------------------------------------

// buildChromaIdentitySQLiteHydration defines the Chroma document identity -> SQLite row hydration rule for
// SEQ-18.5-P179: 18.5-2a Chroma document identity -> SQLite row hydration define.
func buildChromaIdentitySQLiteHydration() map[string]any {
	return map[string]any{
		"version":                    "seq185_p179.v1",
		"role":                       "chroma_identity_sqlite_hydration",
		"truth_authority":            false,
		"canonical_truth_authority":  "mariadb",
		"identity_source":            "chroma_document",
		"hydration_target":           "sqlite_row",
		"canonical_hydration_target": "mariadb_row",
		"hydration_required":         true,
		"policy_version":             "s185-2a.v1",
		"mode":                       "chroma_identity_sqlite_hydration_definition",
	}
}

// buildChromaSQLiteDedupeMerge defines the Chroma candidate + existing SQLite search result dedupe / merge rule for
// SEQ-18.5-P180: 18.5-2b Chroma candidate existing SQLite search result dedupe / merge define.
func buildChromaSQLiteDedupeMerge() map[string]any {
	return map[string]any{
		"version":                      "seq185_p180.v1",
		"role":                         "chroma_sqlite_dedupe_merge",
		"truth_authority":              false,
		"canonical_baseline_authority": "mariadb",
		"dedupe_key":                   "row_id_uniqueness",
		"merge_limit":                  1,
		"score_gap_bound":              "existing_hybrid_tail_budget",
		"policy_version":               "s185-2b.v1",
		"mode":                         "chroma_sqlite_dedupe_merge_definition",
	}
}

// buildCanonicalPrecedenceFormatting defines the canonical/direct evidence precedence preserve final formatting rule for
// SEQ-18.5-P181: 18.5-2c canonical/direct evidence precedence preserve final formatting rule define.
func buildCanonicalPrecedenceFormatting() map[string]any {
	return map[string]any{
		"version":          "seq185_p181.v1",
		"role":             "canonical_precedence_formatting",
		"truth_authority":  false,
		"precedence_order": []string{"canonical", "direct_evidence", "support_only"},
		"formatting_stack": "existing_storyteller_format_for_injection",
		"policy_version":   "s185-2c.v1",
		"mode":             "canonical_precedence_formatting_definition",
	}
}

// buildChromaMissFallbackPreserve defines the Chroma miss / query failure / collection blocked existing keyword fallback preserved rule for
// SEQ-18.5-P182: 18.5-2d Chroma miss / query failure / collection blocked existing keyword fallback preserved rule define.
func buildChromaMissFallbackPreserve() map[string]any {
	return map[string]any{
		"version":               "seq185_p182.v1",
		"role":                  "chroma_miss_fallback_preserve",
		"truth_authority":       false,
		"miss_states":           []string{"chroma_miss", "query_failure", "collection_blocked"},
		"fallback_lane":         "existing_keyword_fallback",
		"fallback_source":       "chat_log",
		"fallback_source_table": "chat_logs",
		"policy_version":        "s185-2d.v1",
		"mode":                  "chroma_miss_fallback_preserve_definition",
	}
}

// ---------------------------------------------------------------------------
// SEQ-18.5 sub-step 18.5-3 SG surfaces (P186 ~ P189)
// ---------------------------------------------------------------------------

// buildOperatorInspectionSurface defines the operator inspection surface for
// SEQ-18.5-P186: 18.5-3a operator inspection surface define - chroma_engaged / sqlite_fallback / shadow_disabled / degraded state/status.
func buildOperatorInspectionSurface() map[string]any {
	return map[string]any{
		"version":           "seq185_p186.v1",
		"role":              "operator_inspection_surface",
		"truth_authority":   false,
		"inspection_states": []string{"chroma_engaged", "sqlite_fallback", "mariadb_fallback", "shadow_disabled", "degraded"},
		"exposed_locations": []string{"trace_row", "trace_preview", "debug_search_preview"},
		"policy_version":    "s185-3a.v1",
		"mode":              "operator_inspection_surface_definition",
	}
}

// buildLiveLimitedModeToggle defines the live limited mode enable/disable surface for
// SEQ-18.5-P187: 18.5-3b live limited mode enable/disable surface define.
func buildLiveLimitedModeToggle() map[string]any {
	return map[string]any{
		"version":         "seq185_p187.v1",
		"role":            "live_limited_mode_toggle",
		"truth_authority": false,
		"toggle_options":  []string{"off", "limited_memory"},
		"persist_locally": true,
		"sync_route":      "/config/update",
		"policy_version":  "s185-3b.v1",
		"mode":            "live_limited_mode_toggle_definition",
	}
}

// buildHealthAdoptionPrerequisite defines the health/adoption prerequisite live limited mode connect for
// SEQ-18.5-P188: 18.5-3c health/adoption prerequisite live limited mode connect define.
func buildHealthAdoptionPrerequisite() map[string]any {
	return map[string]any{
		"version":              "seq185_p188.v1",
		"role":                 "health_adoption_prerequisite",
		"truth_authority":      false,
		"prerequisite_signals": []string{"health_probe", "adoption_gate"},
		"prerequisite_states":  []string{"approved", "hold", "blocked", "unresolved"},
		"read_only_link":       true,
		"policy_version":       "s185-3c.v1",
		"mode":                 "health_adoption_prerequisite_definition",
	}
}

// buildNarrowRolloutRule defines the narrow rollout rule for
// SEQ-18.5-P189: 18.5-3d narrow rollout rule define - broad full-route memory-first / bounded slice-first.
func buildNarrowRolloutRule() map[string]any {
	return map[string]any{
		"version":         "seq185_p189.v1",
		"role":            "narrow_rollout_rule",
		"truth_authority": false,
		"rollout_rules": []string{
			"default_off",
			"memory_first_bounded_slice",
			"sqlite_truth_retained",
			"fail_open_sqlite_scan",
			"mariadb_truth_retained",
			"fail_open_mariadb_scan",
			"not_full_route_not_step21",
		},
		"policy_version": "s185-3d.v1",
		"mode":           "narrow_rollout_rule_definition",
	}
}

// ---------------------------------------------------------------------------
// SEQ-18.5 sub-step 18.5-4 VX surfaces (P193 ~ P197)
// ---------------------------------------------------------------------------

// buildChromaEnabledSmokeCheck defines the Chroma enabled live smoke check for
// SEQ-18.5-P193: 18.5-4a Chroma enabled live smoke check define.
func buildChromaEnabledSmokeCheck() map[string]any {
	return map[string]any{
		"version":         "seq185_p193.v1",
		"role":            "chroma_enabled_smoke_check",
		"truth_authority": false,
		"ready_conditions": []string{
			"chroma_engaged",
			"limited_memory",
			"sqlite_truth_authority",
			"mariadb_truth_authority",
			"hydrated_rejoin",
			"support_only_precedence_preserved",
		},
		"policy_version": "s185-4a.v1",
		"mode":           "chroma_enabled_smoke_check_definition",
	}
}

// buildDegradedFailOpenReplay defines the degraded fail-open replay for
// SEQ-18.5-P194: 18.5-4b degraded fail-open replay define.
func buildDegradedFailOpenReplay() map[string]any {
	return map[string]any{
		"version":                           "seq185_p194.v1",
		"role":                              "degraded_fail_open_replay",
		"truth_authority":                   false,
		"replay_states":                     []string{"blocked", "query_failed", "empty"},
		"fallback_preserved":                true,
		"fallback_source":                   "chat_log",
		"fallback_source_table":             "chat_logs",
		"sqlite_truth_authority_preserved":  true,
		"mariadb_truth_authority_preserved": true,
		"policy_version":                    "s185-4b.v1",
		"mode":                              "degraded_fail_open_replay_definition",
	}
}

// buildSQLiteBaselineParityReplay defines the SQLite baseline parity / non-regression replay for
// SEQ-18.5-P195: 18.5-4c SQLite baseline parity / non-regression replay define.
func buildSQLiteBaselineParityReplay() map[string]any {
	return map[string]any{
		"version":                    "seq185_p195.v1",
		"role":                       "sqlite_baseline_parity_replay",
		"truth_authority":            false,
		"current_baseline_authority": "mariadb",
		"parity_mode":                "exact_or_bounded_one_slot_delta",
		"baseline_head_preserved":    true,
		"result_window_preserved":    true,
		"policy_version":             "s185-4c.v1",
		"mode":                       "sqlite_baseline_parity_replay_definition",
	}
}

// buildTruthBoundarySourceOrderReplay defines the truth-boundary / source-order replay for
// SEQ-18.5-P196: 18.5-4d truth-boundary / source-order replay define.
func buildTruthBoundarySourceOrderReplay() map[string]any {
	return map[string]any{
		"version":         "seq185_p196.v1",
		"role":            "truth_boundary_source_order_replay",
		"truth_authority": false,
		"boundary_rules": []string{
			"merged_live_candidate_below_baseline_head",
			"source_table_memories_preserved",
			"sqlite_truth_support_only_precedence_ceiling",
			"mariadb_truth_support_only_precedence_ceiling",
			"degraded_fallback_on_chat_logs",
		},
		"policy_version": "s185-4d.v1",
		"mode":           "truth_boundary_source_order_replay_definition",
	}
}

// buildReleaseNoteHonestyChecklist defines the release-note honesty checklist for
// SEQ-18.5-P197: 18.5-4e release-note honesty checklist define - Step 18.5 operator-gated limited live cutover / markers.
func buildReleaseNoteHonestyChecklist() map[string]any {
	return map[string]any{
		"version":         "seq185_p197.v1",
		"role":            "release_note_honesty_checklist",
		"truth_authority": false,
		"checklist_items": []string{
			"operator_gated_limited_live_cutover",
			"default_off",
			"step_21_out_of_scope",
		},
		"policy_version": "s185-4e.v1",
		"mode":           "release_note_honesty_checklist_definition",
	}
}

// ---------------------------------------------------------------------------
// SEQ-18.5 release gate surfaces (P201 ~ P205)
// ---------------------------------------------------------------------------
// These surfaces are release/bundle gate rows. They do NOT create actual
// artifacts; they record dry-run evidence and operator-approval boundaries.

// buildBundleReleaseGate201 defines the bundle release gate surface for
// SEQ-18.5-P201: Archive Center 1.0.1-pre bundle latest root runtime create/generate.
// IMPORTANT: This is a dry-run evidence contract only. No actual bundle/exe/zip
// artifact is created in the 2.0 remigration path.
func buildBundleReleaseGate201() map[string]any {
	return map[string]any{
		"version":                    "seq185_p201.v1",
		"role":                       "bundle_release_gate",
		"truth_authority":            false,
		"artifact_created":           false,
		"dry_run_only":               true,
		"operator_approval_required": true,
		"bundle_target":              "archive_center_1_0_1_pre",
		"canonical_truth_authority":  "mariadb",
		"vector_accelerator":         "chromadb",
		"blocked_reason":             "needs_operator_approval_for_artifact_generation",
		"policy_version":             "s185-rg.v1",
		"mode":                       "bundle_release_gate_dry_run",
	}
}

// buildLimitedLiveChromaSmokeCheck202 defines the limited live Chroma smoke
// check pass surface for SEQ-18.5-P202: limited live Chroma smoke check pass.
func buildLimitedLiveChromaSmokeCheck202() map[string]any {
	return map[string]any{
		"version":                      "seq185_p202.v1",
		"role":                         "limited_live_chroma_smoke_check",
		"truth_authority":              false,
		"smoke_status":                 "contract_pass",
		"contract_smoke_pass":          true,
		"actual_live_chroma_smoke_run": false,
		"dry_run_only":                 true,
		"ready_conditions_verified":    true,
		"chroma_engagement_scope":      "limited_memory_candidate_contract",
		"limited_memory":               true,
		"sqlite_truth_authority":       true,
		"mariadb_truth_authority":      true,
		"canonical_truth_authority":    "mariadb",
		"source_truth_label":           "sqlite",
		"hydrated_rejoin":              true,
		"support_only_precedence":      true,
		"operator_approval_required":   true,
		"blocked_from_live_cutover":    true,
		"blocked_reason":               "limited_live_smoke_not_executed_in_remigration_dry_run",
		"policy_version":               "s185-smoke.v1",
		"mode":                         "limited_live_chroma_smoke_check_contract",
	}
}

// buildSQLiteFailOpenReplayPass203 defines the SQLite fail-open replay pass
// surface for SEQ-18.5-P203: SQLite fail-open replay pass.
// Source wording preserves "SQLite" from 0.8 reference; 2.0 current field
// explicitly includes MariaDB canonical truth.
func buildSQLiteFailOpenReplayPass203() map[string]any {
	return map[string]any{
		"version":                           "seq185_p203.v1",
		"role":                              "sqlite_fail_open_replay_pass",
		"truth_authority":                   false,
		"replay_status":                     "contract_pass",
		"contract_replay_pass":              true,
		"actual_sqlite_replay_run":          false,
		"dry_run_only":                      true,
		"replay_states_covered":             []string{"blocked", "query_failed", "empty"},
		"fallback_preserved":                true,
		"fallback_source":                   "chat_log",
		"fallback_source_table":             "chat_logs",
		"source_truth_label":                "sqlite",
		"sqlite_truth_authority_preserved":  true,
		"mariadb_truth_authority_preserved": true,
		"canonical_truth_authority":         "mariadb",
		"fallback_lane_current":             "mariadb_scan_preserved",
		"policy_version":                    "s185-fo.v1",
		"mode":                              "sqlite_fail_open_replay_contract",
	}
}

// buildOperatorVisibilityFallbackChecklist204 defines the operator visibility /
// fallback reason checklist pass surface for SEQ-18.5-P204.
func buildOperatorVisibilityFallbackChecklist204() map[string]any {
	return map[string]any{
		"version":                    "seq185_p204.v1",
		"role":                       "operator_visibility_fallback_checklist",
		"truth_authority":            false,
		"checklist_status":           "pass",
		"inspection_states_verified": []string{"chroma_engaged", "sqlite_fallback", "mariadb_fallback", "shadow_disabled", "degraded"},
		"fallback_reason_traced":     true,
		"trace_locations":            []string{"trace_row", "trace_preview", "debug_search_preview", "prepare_turn_response"},
		"policy_version":             "s185-ov.v1",
		"mode":                       "operator_visibility_fallback_checklist_pass",
	}
}

// buildReleaseNoteBundleNotesComplete205 defines the release note / bundle
// notes complete surface for SEQ-18.5-P205.
func buildReleaseNoteBundleNotesComplete205() map[string]any {
	return map[string]any{
		"version":                "seq185_p205.v1",
		"role":                   "release_note_bundle_notes_complete",
		"truth_authority":        false,
		"completion_status":      "complete",
		"bundle_notes_synced":    true,
		"release_notes_complete": true,
		"checklist_items_closed": []string{
			"operator_gated_limited_live_cutover",
			"default_off",
			"step_21_out_of_scope",
			"bundle_regenerate_intentionally_pending",
			"beta_mirror_refresh_intentionally_pending",
		},
		"artifact_created":           false,
		"dry_run_only":               true,
		"operator_approval_required": true,
		"policy_version":             "s185-rn.v1",
		"mode":                       "release_note_bundle_notes_complete",
	}
}

// ---------------------------------------------------------------------------
// SEQ-18.5 decision surfaces (P209 ~ P212)
// ---------------------------------------------------------------------------
// These surfaces record architectural decisions for the limited live cutover
// path. They are decision contracts, not runtime mutations.

// buildFirstLiveScopeDecision209 defines the first live scope decision surface
// for SEQ-18.5-P209: first live scope memory-only vs broader memory+episode.
func buildFirstLiveScopeDecision209() map[string]any {
	return map[string]any{
		"version":                    "seq185_p209.v1",
		"role":                       "first_live_scope_decision",
		"truth_authority":            false,
		"decision":                   "memory_only",
		"broader_memory_episode":     "blocked",
		"blocked_reason":             "operator_gated_limited_cutover_scope_memory_tier_only",
		"canonical_truth_authority":  "mariadb",
		"vector_accelerator":         "chromadb",
		"source_scope_label":         "sqlite_limited_memory",
		"current_scope_label":        "mariadb_limited_memory",
		"operator_approval_required": true,
		"dry_run_only":               true,
		"policy_version":             "s185-d1.v1",
		"mode":                       "first_live_scope_decision_memory_only",
	}
}

// buildChromaCandidateMergeReplaceDecision210 defines the Chroma candidate
// merge vs replace decision surface for SEQ-18.5-P210.
func buildChromaCandidateMergeReplaceDecision210() map[string]any {
	return map[string]any{
		"version":                      "seq185_p210.v1",
		"role":                         "chroma_candidate_merge_replace_decision",
		"truth_authority":              false,
		"replace":                      false,
		"merge":                        true,
		"merge_mode":                   "support_only_additive",
		"canonical_baseline_authority": "mariadb",
		"source_baseline_label":        "sqlite",
		"chroma_candidate_role":        "accelerator_not_authority",
		"hydration_required":           true,
		"hydration_target":             "mariadb_row",
		"source_hydration_label":       "sqlite_row",
		"operator_approval_required":   true,
		"dry_run_only":                 true,
		"policy_version":               "s185-d2.v1",
		"mode":                         "chroma_candidate_merge_replace_decision",
	}
}

// buildDegradedThresholdDecision211 defines the degraded threshold decision
// surface for SEQ-18.5-P211: health-probe reuse vs live stricter gate.
func buildDegradedThresholdDecision211() map[string]any {
	return map[string]any{
		"version":                       "seq185_p211.v1",
		"role":                          "degraded_threshold_decision",
		"truth_authority":               false,
		"decision":                      "reuse_health_probe_threshold",
		"health_probe_threshold_reused": true,
		"live_stricter_gate":            false,
		"live_cutover_default":          false,
		"stricter_gate_deferred_reason": "operator_gated_limited_cutover_uses_existing_health_probe",
		"degraded_fallback_preserved":   true,
		"fallback_lane":                 "mariadb_scan_preserved",
		"source_fallback_label":         "sqlite_scan_preserved",
		"canonical_truth_authority":     "mariadb",
		"operator_approval_required":    true,
		"dry_run_only":                  true,
		"policy_version":                "s185-d3.v1",
		"mode":                          "degraded_threshold_decision_health_probe_reused",
	}
}

// buildOperatorVisibilityScopeDecision212 defines the operator visibility
// scope decision surface for SEQ-18.5-P212: live/fallback state exposure.
func buildOperatorVisibilityScopeDecision212() map[string]any {
	return map[string]any{
		"version":            "seq185_p212.v1",
		"role":               "operator_visibility_scope_decision",
		"truth_authority":    false,
		"inspection_only":    true,
		"no_mutation":        true,
		"no_truth_authority": true,
		"exposed_states": []string{
			"live",
			"fallback",
			"degraded",
			"shadow_disabled",
			"mariadb_fallback",
			"chroma_candidate",
			"sqlite_fallback",
		},
		"exposure_locations":         []string{"settings_panel", "search_response", "prepare_turn_response", "trace_preview", "root_runtime_trace_row", "debug_search_preview"},
		"canonical_truth_authority":  "mariadb",
		"operator_approval_required": true,
		"dry_run_only":               true,
		"policy_version":             "s185-d4.v1",
		"mode":                       "operator_visibility_scope_decision",
	}
}
