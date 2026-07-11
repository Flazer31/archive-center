package main

import (
	"strings"
	"testing"
)

func TestArchiveCenterJSSeq12P125BoundaryDefineMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const coprocessorTruthWriteTargets = [`,
		`const coprocessorSidecarWritableTargets = [`,
		`const coprocessorBoundaryPolicyVersion = "mg1b.v1"`,
		`boundary_scope: "step11_truth_owner"`,
		`write_targets: coprocessorTruthWriteTargets.slice()`,
		`current_fact_write: true`,
		`canonical_write: true`,
		`boundary_scope: "step12_diagnostic_sidecar"`,
		`boundary_scope: "step12_proposal_sidecar"`,
		`boundary_scope: "step12_guidance_sidecar"`,
		`denied_write_targets: coprocessorTruthWriteTargets.slice()`,
		`current_fact_write: false`,
		`canonical_write: false`,
		`const coprocessorReadBoundaryBySurface = {}`,
		`const coprocessorWriteBoundaryByTarget = {}`,
		`sidecar_writable_targets: coprocessorSidecarWritableTargets.slice()`,
		`sidecar_denied_truth_targets: coprocessorTruthWriteTargets.slice()`,
		`read_boundary_by_surface: coprocessorReadBoundaryBySurface`,
		`write_boundary_by_target: coprocessorWriteBoundaryByTarget`,
		`canonical_write_authority_after_reentry: "step11_truth_core_only"`,
		`canonical_write_authority: "step11_truth_core_only"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P125 boundary define marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P131FeatureRolloutKillSwitchMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const coprocessorFeatureControlPolicyVersion = "mg1c.v1"`,
		`const coprocessorFeatureFlagModes = ["always_on", "conservative", "experimental", "off"]`,
		`const coprocessorRolloutStages = ["truth_floor_locked", "diagnostic_default_on", "experimental_shadow", "manual_enable_required"]`,
		`const coprocessorKillSwitchStates = ["not_applicable", "armed_standby", "engaged"]`,
		`const coprocessorFeatureControlMatrix = [`,
		`module: "step11_truth_core"`,
		`default_mode: "always_on"`,
		`rollout_stage: "truth_floor_locked"`,
		`kill_switch_supported: false`,
		`ablation_supported: false`,
		`wiring_status: "hard_enabled_runtime"`,
		`module: "truth_maintenance"`,
		`default_mode: "conservative"`,
		`rollout_stage: "diagnostic_default_on"`,
		`kill_switch_action: "fail_open_skip"`,
		`module: "retrieval_supporting_inference"`,
		`default_mode: "conservative"`,
		`rollout_stage: "diagnostic_default_on"`,
		`kill_switch_action: "fail_open_skip"`,
		`module: "entity_coprocessor"`,
		`default_mode: "off"`,
		`rollout_stage: "manual_enable_required"`,
		`wiring_status: "trace_contract_only"`,
		`kill_switch_action: "fail_open_skip"`,
		`module: "world_coprocessor"`,
		`default_mode: "off"`,
		`rollout_stage: "manual_enable_required"`,
		`kill_switch_action: "fail_open_skip"`,
		`module: "narrative_quality_coprocessor"`,
		`default_mode: "experimental"`,
		`rollout_stage: "experimental_shadow"`,
		`kill_switch_action: "fail_open_skip"`,
		`coprocessor_feature_control: {`,
		`policy_version: coprocessorFeatureControlPolicyVersion`,
		`supported_feature_flag_modes: coprocessorFeatureFlagModes.slice()`,
		`supported_rollout_stages: coprocessorRolloutStages.slice()`,
		`supported_kill_switch_states: coprocessorKillSwitchStates.slice()`,
		`kill_switchable_modules: coprocessorKillSwitchableModules.slice()`,
		`modules: coprocessorFeatureControlMatrix.slice()`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P131 feature/rollout/kill-switch marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P137BudgetSplitMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const coprocessorBudgetIsolationPolicyVersion = "mg1d.v1"`,
		`const truthFloorBudgetLane = "truth_floor"`,
		`const coprocessorHintBudgetLane = "coprocessor_hint"`,
		`const truthFloorBudgetLabels = [`,
		`const coprocessorHintBudgetPromptLabels = [`,
		`const coprocessorHintBudgetPromptModules = [`,
		`narrativeQualityCoprocessorAblationProtectedTruthLabels = truthFloorBudgetLabels.slice()`,
		`worldCoprocessorGuidanceBudgetLane = coprocessorHintBudgetLane`,
		`worldCoprocessorGuidanceBudgetProtectedTruthLabels = truthFloorBudgetLabels.slice()`,
		`coprocessor_budget_isolation: {`,
		`status: "truth_floor_reserved_hint_residual"`,
		`policy_version: coprocessorBudgetIsolationPolicyVersion`,
		`budget_isolation_strategy: "truth_floor_reserved_then_hint_residual"`,
		`truth_floor_lane: truthFloorBudgetLane`,
		`truth_floor_owner_modules: coprocessorAuthorityTruthWriterModules.slice()`,
		`truth_floor_labels: truthFloorBudgetLabels.slice()`,
		`truth_floor_reservation_mode: "hard_floor_reserved_first"`,
		`hint_budget_lane: coprocessorHintBudgetLane`,
		`hint_budget_modules: coprocessorAuthoritySidecarModules.slice()`,
		`hint_budget_prompt_modules: coprocessorHintBudgetPromptModules.slice()`,
		`hint_budget_non_prompt_modules: coprocessorHintBudgetNonPromptModules.slice()`,
		`hint_budget_prompt_labels: coprocessorHintBudgetPromptLabels.slice()`,
		`hint_budget_reservation_mode: "residual_only"`,
		`label_lane_map: Object.assign({}, coprocessorBudgetLaneByLabel)`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P137 budget split marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P145MG1eKeepDropDegradeReasonTraceMarker(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const coprocessorReasonTracePolicyVersion = "mg1e.v1"`,
		`const coprocessorReasonTraceActions = ["keep", "drop", "degrade"]`,
		`const coprocessorReasonTraceKeepReasonCodes = [`,
		`truth_floor_reserved`,
		`hint_budget_delivered`,
		`delivered_full`,
		`no_canonical_conflict`,
		`canonical_guard_disabled`,
		`const coprocessorReasonTraceDropReasonCodes = [`,
		`budget_exhausted`,
		`supporting_guidance_conflict_blocked`,
		`canonical_polarity_conflict`,
		`budget_collision_saga_reserve`,
		`reliability_guard_hold`,
		`ingest_only_not_injected`,
		`explicit_user_redirection`,
		`const coprocessorReasonTraceDegradeReasonCodes = [`,
		`item_truncated`,
		`hard_cap_unstructured`,
		`no_alignment_rescue_ceiling`,
		`const coprocessorReasonTraceBudgetReasonCodes = [`,
		`const coprocessorReasonTraceReasonCodes = Array.from(new Set(`,
		`coprocessor_reason_trace: {`,
		`status: "keep_drop_degrade_trace_fixed"`,
		`policy_version: coprocessorReasonTracePolicyVersion`,
		`supported_actions: coprocessorReasonTraceActions.slice()`,
		`supported_reason_codes: coprocessorReasonTraceReasonCodes.slice()`,
		`keep_reason_codes: coprocessorReasonTraceKeepReasonCodes.slice()`,
		`drop_reason_codes: coprocessorReasonTraceDropReasonCodes.slice()`,
		`degrade_reason_codes: coprocessorReasonTraceDegradeReasonCodes.slice()`,
		`budget_reason_codes: coprocessorReasonTraceBudgetReasonCodes.slice()`,
		`truth_floor_lane: truthFloorBudgetLane`,
		`hint_budget_lane: coprocessorHintBudgetLane`,
		`trace_sources: ["blocks", "trimmed", "retrievalKeepDropTrace"]`,
		`const coprocessorReasonTraceByAction = { keep: [], drop: [], degrade: [] }`,
		`degrade: coprocessorReasonTraceByAction.degrade.length`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P145 MG-1e keep/drop/degrade reason trace marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P152MG1fAntiCopyReviewChecklistMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const coprocessorAntiCopyReviewPolicyVersion = "mg1f.v1"`,
		`const coprocessorAntiCopyReviewScope = "pr_unit"`,
		`const coprocessorAntiCopyAllowedReferenceMode = "behavioral_reference_only"`,
		`const coprocessorAntiCopyReviewEvidenceFields = [`,
		`"local_owner_surface"`,
		`"behavioral_source_note"`,
		`"diff_review_note"`,
		`"constant_origin_note"`,
		`const coprocessorAntiCopyReviewChecklist = [`,
		`check_id: "external_structure"`,
		`carryover_class: "structure"`,
		`check_id: "function_body"`,
		`carryover_class: "function_body"`,
		`check_id: "call_flow"`,
		`carryover_class: "call_flow"`,
		`check_id: "constant_values"`,
		`carryover_class: "constant_value"`,
		`check_id: "thresholds"`,
		`carryover_class: "threshold"`,
		`check_id: "ratios"`,
		`carryover_class: "ratio"`,
		`check_id: "token_budget_defaults"`,
		`carryover_class: "token_budget_default"`,
		`blocking: true`,
		`forbidden_direct_carryover: true`,
		`const coprocessorAntiCopyReviewCheckIds`,
		`const coprocessorAntiCopyReviewBlockingChecks`,
		`const coprocessorAntiCopyForbiddenCarryoverClasses`,
		`coprocessor_anti_copy_review: {`,
		`status: "review_checklist_fixed"`,
		`policy_version: coprocessorAntiCopyReviewPolicyVersion`,
		`review_scope: coprocessorAntiCopyReviewScope`,
		`allowed_reference_mode: coprocessorAntiCopyAllowedReferenceMode`,
		`required_evidence_fields: coprocessorAntiCopyReviewEvidenceFields.slice()`,
		`forbidden_carryover_classes: coprocessorAntiCopyForbiddenCarryoverClasses.slice()`,
		`blocking_checks: coprocessorAntiCopyReviewBlockingChecks.slice()`,
		`checklist: coprocessorAntiCopyReviewChecklist.slice()`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P152 MG-1f anti-copy review checklist marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P160MG1gProposalReentryGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const coprocessorProposalReentryPolicyVersion = "mg1g.v1"`,
		`const coprocessorProposalTraceRequiredFields = ["evidence_refs", "source_turns", "confidence"]`,
		`const coprocessorProposalReentryRequiredModules = ["entity_coprocessor", "world_coprocessor"]`,
		`const coprocessorProposalAdoptionPath = ["proposal_trace", "reducer_reentry", "step11_truth_core"]`,
		`proposal_trace_schema_required_fields: coprocessorProposalTraceRequiredFields.slice()`,
		`truth_path_reentry_gate: "reducer_reentry_required"`,
		`truth_path_entry_requirements: coprocessorProposalTraceRequiredFields.concat(["reducer_reentry"])`,
		`truth_path_blocked_without_required_fields: true`,
		`canonical_write_before_reducer_reentry: false`,
		`canonical_write_authority_after_reentry: "step11_truth_core_only"`,
		`coprocessor_proposal_reentry: {`,
		`status: "proposal_reentry_gate_fixed"`,
		`policy_version: coprocessorProposalReentryPolicyVersion`,
		`proposal_trace_modules: coprocessorProposalTraceModules.slice()`,
		`required_trace_fields: coprocessorProposalTraceRequiredFields.slice()`,
		`reducer_reentry_required_modules: coprocessorProposalReentryRequiredModules.slice()`,
		`canonical_write_before_reducer_reentry: false`,
		`canonical_write_authority: "step11_truth_core_only"`,
		`adoption_path: coprocessorProposalAdoptionPath.slice()`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P160 MG-1g proposal re-entry gate marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P168MG1hAnalysisProviderContractMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const coprocessorAnalysisProviderPolicyVersion = "mg1h.v1"`,
		`const coprocessorAnalysisProviderModules = ["entity_coprocessor", "world_coprocessor", "narrative_quality_coprocessor"]`,
		`const coprocessorAnalysisProviderRawOutputMode = "proposal_only"`,
		`const coprocessorAnalysisProviderCallFrequencyOwner = "step13_governor"`,
		`analysis_provider_class: "llm_based_sidecar"`,
		`analysis_provider_raw_output_mode: coprocessorAnalysisProviderRawOutputMode`,
		`analysis_provider_autonomous_truth_write: false`,
		`analysis_provider_call_frequency_owner: coprocessorAnalysisProviderCallFrequencyOwner`,
		`coprocessor_analysis_provider: {`,
		`status: "analysis_provider_contract_fixed"`,
		`policy_version: coprocessorAnalysisProviderPolicyVersion`,
		`provider_class: "llm_based_sidecar"`,
		`provider_backed_modules: coprocessorAnalysisProviderModules.slice()`,
		`raw_output_mode: coprocessorAnalysisProviderRawOutputMode`,
		`trace_target_by_module: Object.assign({}, coprocessorAnalysisProviderTraceTargets)`,
		`normalization_by_module: Object.assign({}, coprocessorAnalysisProviderNormalizationModes)`,
		`autonomous_truth_write_allowed: false`,
		`disallowed_usage: coprocessorAnalysisProviderDisallowedUsage.slice()`,
		`canonical_write_authority: "step11_truth_core_only"`,
		`call_frequency_owner: coprocessorAnalysisProviderCallFrequencyOwner`,
		`call_frequency_policy_version: step13GovernorPolicyVersion`,
		`call_frequency_policy_status: "governor_contract_fixed"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P168 MG-1h analysis provider contract marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P179(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const coprocessorOrchestrationPolicyVersion = "or1a.v1"`,
		`const coprocessorOrchestrationRuntimeStages = ["step11_truth_stack", "residual_guidance"]`,
		`const coprocessorOrchestrationTruthStackOwner = "step11_truth_core"`,
		`const coprocessorOrchestrationCallEntryGate = "after_step11_truth_stack"`,
		`const coprocessorOrchestrationResidualBudgetSource = "coprocessor_hint_residual"`,
		`const coprocessorOrchestrationProviderCallOrder = ["entity_coprocessor", "world_coprocessor", "narrative_quality_coprocessor"]`,
		`entity_coprocessor: "residual_guidance"`,
		`world_coprocessor: "residual_guidance"`,
		`narrative_quality_coprocessor: "residual_guidance"`,
		`orchestration_stage: coprocessorOrchestrationStageByModule.entity_coprocessor`,
		`orchestration_entry_gate: coprocessorOrchestrationCallEntryGate`,
		`orchestration_budget_source: coprocessorOrchestrationResidualBudgetSource`,
		`coprocessor_orchestration: {`,
		`status: "residual_guidance_stage_fixed"`,
		`policy_version: coprocessorOrchestrationPolicyVersion`,
		`runtime_stage_order: coprocessorOrchestrationRuntimeStages.slice()`,
		`truth_stack_owner: coprocessorOrchestrationTruthStackOwner`,
		`truth_stack_labels: truthFloorBudgetLabels.slice()`,
		`call_entry_gate: coprocessorOrchestrationCallEntryGate`,
		`residual_guidance_stage: "residual_guidance"`,
		`residual_guidance_budget_source: coprocessorOrchestrationResidualBudgetSource`,
		`provider_call_order: coprocessorOrchestrationProviderCallOrder.slice()`,
		`stage_by_module: Object.assign({}, coprocessorOrchestrationStageByModule)`,
		`call_allowed_before_truth_stack: false`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P179 OR-1a coprocessor call order marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P187OR1bFallbackRouteMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`function getOrchestrationFallbackPolicyOr1b()`,
		`policyVersion: "or1b.v1"`,
		`owner: "step12.orchestration"`,
		`supportedRoutes: ["blocked_empty_result", "cached_result", "skip_protection_only", "direct_result"]`,
		`emptyResultRoute: "direct_result"`,
		`emptyResultPayloadAction: "preserve_original_payload"`,
		`readyCacheRoute: "cached_result"`,
		`readyCachePayloadAction: "consume_pending_ready_result"`,
		`readyCacheReuseScope: "same_chat_session_after_request_only"`,
		`overlapRunningRoute: "skip_protection_only"`,
		`overlapRunningPayloadAction: "applyProtectionOnlyInjection"`,
		`stalePendingAction: "clear_stale_then_recompute"`,
		`timeoutWindowSource: "getOrchestrationTimeoutMs"`,
		`function resolveOrchestrationFallbackRouteOr1b`,
		`function applyOrchestrationFallbackTraceOr1b`,
		`trace.orchestrationFallback = {`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P187 OR-1b fallback route marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P195OR1cDirtySignalMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`function computeOrchestrationDirtyHashOr1c`,
		`function getPrepareTurnBundleSignatureOr1c`,
		`function getOrchestrationDirtySignalPolicyOr1c()`,
		`policyVersion: "or1c.v1"`,
		`evaluationMode: "trace_contract_only"`,
		`"session_bootstrap"`,
		`"user_input_changed"`,
		`"input_source_changed"`,
		`"continuity_mode_changed"`,
		`"continuity_query_changed"`,
		`"prepare_turn_source_changed"`,
		`"prepare_turn_bundle_changed"`,
		`"rollback_invalidation"`,
		`"guidance_invalidation"`,
		`"stable_session_snapshot"`,
		`stableSignals: ["stable_session_snapshot"]`,
		`requiredSnapshotFields: [`,
		`"chat_session_id"`,
		`"user_input_hash"`,
		`"input_source"`,
		`"continuity_mode"`,
		`"continuity_query_hash"`,
		`"prepare_turn_source"`,
		`"prepare_turn_bundle_signature"`,
		`"rollback_token"`,
		`"guidance_token"`,
		`runtimeExecutionMode: "always_recompute_until_or1d"`,
		`runtimeSkipBehavior: "trace_only_policy_contract"`,
		`dirty_signal_policy_version: coprocessorOrchestrationDirtySignalPolicyVersion`,
		`dirty_signal_evaluation_mode: coprocessorOrchestrationDirtySignalEvaluationMode`,
		`dirty_signal_snapshot_fields: coprocessorOrchestrationDirtySnapshotFields.slice()`,
		`dirty_signal_supported: coprocessorOrchestrationDirtySignals.slice()`,
		`dirty_signal_recompute: coprocessorOrchestrationDirtyRecomputeSignals.slice()`,
		`dirty_signal_stable: coprocessorOrchestrationDirtyStableSignals.slice()`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P195 OR-1c dirty signal marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P203OR1dCacheInvalidationMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`function getOrchestrationCachePolicyOr1d()`,
		`policyVersion: "or1d.v1"`,
		`cacheUnit: "pending_ready_orchestration_result"`,
		`cacheKeySourcePolicyVersion: dirtyPolicy.policyVersion`,
		`cacheKeyFields: dirtyPolicy.requiredSnapshotFields.slice().concat(["dirty_policy_version"])`,
		`reuseScope: fallbackPolicy.readyCacheReuseScope`,
		`reuseRoute: fallbackPolicy.readyCacheRoute`,
		`invalidationSignals: dirtyPolicy.recomputeSignals.slice()`,
		`invalidationTokens: ["rollback_token", "guidance_token"]`,
		`stalePendingAction: fallbackPolicy.stalePendingAction`,
		`staleServingGuard: "deny_reuse_on_cache_key_mismatch_or_invalidation_drift"`,
		`runtimeReuseMode: "pending_ready_only"`,
		`advancedCachePolicyStatus: "deferred_to_step13"`,
		`function buildOrchestrationCacheDescriptorOr1d`,
		`function assessOrchestrationCacheReuseOr1d`,
		`cache_policy_version: coprocessorOrchestrationCachePolicyVersion`,
		`cache_unit: coprocessorOrchestrationCacheUnit`,
		`cache_reuse_scope: coprocessorOrchestrationCacheReuseScope`,
		`cache_key_fields: coprocessorOrchestrationCacheKeyFields.slice()`,
		`cache_invalidation_signals: coprocessorOrchestrationCacheInvalidationSignals.slice()`,
		`cache_invalidation_tokens: coprocessorOrchestrationCacheInvalidationTokens.slice()`,
		`cache_stale_guard: coprocessorOrchestrationCacheStaleGuard`,
		`cache_advanced_policy_status: "deferred_to_step13"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P203 OR-1d cache invalidation marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P211OR1eModuleTransportMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`function getOrchestrationModuleTransportPolicyOr1e()`,
		`policyVersion: "or1e.v1"`,
		`backendRequiredModules: [`,
		`"prepare_turn_probe"`,
		`"memory_search"`,
		`"kg_recall"`,
		`"episode_recall"`,
		`"active_states_fetch"`,
		`"narrative_control_fetch"`,
		`"maintenance_pass_writeback"`,
		`"turn_complete_commit"`,
		`"rollback_invalidation"`,
		`backendBundleAssistedSurfaces: [`,
		`backendProxyAssistedModules: [`,
		`pluginOnlyModules: [`,
		`backendBundleEntryPoint: "/prepare-turn"`,
		`backendProxyRoute: "/proxy/plugin-main"`,
		`pluginOnlyExecutionMode: "local_runtime_contract_only"`,
		`runtimeSplitStatus: "trace_contract_only"`,
		`function buildOrchestrationModuleTransportStateOr1e`,
		`function applyOrchestrationModuleTransportTraceOr1e`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P211 OR-1e module transport marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P218OR1fTurnDeletionDetectionMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`function getRollbackDetectionPolicyOr1f()`,
		`policyVersion: "or1f.v1"`,
		`detectionSources: [`,
		`"history_diff_common_prefix_suffix"`,
		`"persisted_turn_ledger"`,
		`"tail_hash_guard"`,
		`"tail_delete"`,
		`"assistant_deleted_before_next_user_turn"`,
		`"historical_contiguous_delete"`,
		`duplicateGuard: "session_history_diff_signature"`,
		`function buildRollbackTurnLedgerOr1f`,
		`function resolveRollbackTurnAnchorOr1f`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P218 OR-1f turn deletion detection marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P223OR1gHistoricalDeletionInvalidationMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`function getRollbackInvalidationPolicyOr1g()`,
		`policyVersion: "or1g.v1"`,
		`invalidationRoute: "/rollback/{turn_index}"`,
		`triggerSources: ["auto_rollback", "manual_ui"]`,
		`"rollback_token"`,
		`"guidance_token_on_backend_reset"`,
		`guidanceCleanupMode: "invalidate_guidance_plan_and_delete_compacts_and_maintenance"`,
		`cacheReuseGuard: "rollback_and_guidance_token_drift_block_pending_ready_reuse"`,
		`staleSidecarGuard: "backend_cleanup_then_dirty_signal_recompute"`,
		`function buildRollbackInvalidationStateOr1g`,
		`function applyRollbackInvalidationTraceOr1g`,
		`historicalDeletionDetected: String(triggerReason || "") === "historical_turn_gap_detected"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P223 OR-1g historical deletion invalidation marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P230OR1hDirtySignalMatrixMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`function getOrchestrationDirtyMatrixPolicyOr1h()`,
		`policyVersion: "or1h.v1"`,
		`dirtyTargets: [`,
		`"guidance_state"`,
		`"entity_coprocessor"`,
		`"world_coprocessor"`,
		`"narrative_quality"`,
		`"sidecar_cache"`,
		`runtimeObservedEventTypes: ["turn_deletion"]`,
		`runtimeEventTargetMatrix: {`,
		`user_correction: [`,
		`canonical_update: [`,
		`world_state_update: [`,
		`turn_deletion: [`,
		`backfill_import: [`,
		`schema_migration: [`,
		`delegatedEventMatrixVersions: {`,
		`truth_maintenance_drift: "or1h.tm1d.v1"`,
		`truth_maintenance_importance: "or1h.tm1d.v1"`,
		`function buildOrchestrationDirtyMatrixStateOr1h`,
		`function applyOrchestrationDirtyMatrixTraceOr1h`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P230 OR-1h dirty signal matrix marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P235OR1iRebuildInvalidationOrchestrationMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`function getOrchestrationRebuildPolicyOr1i()`,
		`policyVersion: "or1i.v1"`,
		`staleServingPolicy: "deny_stale_sidecar_on_rebuild_pending"`,
		`staleDropTargets: [`,
		`"pending_ready_orchestration_result"`,
		`"sidecar_cache"`,
		`"guidance_trace"`,
		`hardResetTargets: [`,
		`"prepare_turn_bundle"`,
		`"session_snapshot_cache"`,
		`"persisted_turn_ledger"`,
		`startPointPrecedence: [`,
		`"checkpoint_full_rebuild"`,
		`"rollback_turn_anchor_then_prepare_turn"`,
		`"next_narrative_control_fetch"`,
		`"next_prepare_turn_fetch"`,
		`"step11_truth_stack"`,
		`planByEventType: {`,
		`rebuildMode: "selective"`,
		`rebuildMode: "full"`,
		`function buildOrchestrationRebuildStateOr1i`,
		`function applyOrchestrationRebuildTraceOr1i`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P235 OR-1i rebuild invalidation orchestration marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P242OR1jStaleProposalServingMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`function getOrchestrationStaleProposalPolicyOr1j()`,
		`policyVersion: "or1j.v1"`,
		`blockedPromptTargets: [`,
		`"pending_ready_orchestration_result"`,
		`"proposal_trace"`,
		`"guidance_trace"`,
		`"sidecar_cache"`,
		`evidenceMismatchSignals: ["chapter_block_mismatch", "chapter_input_anchor_mismatch"]`,
		`runtimeEnforcement: "after_request_pending_salvage_guard"`,
		`staleServingAction: "drop_stale_pending_before_prompt_and_persistence"`,
		`function assessOrchestrationStaleProposalServingOr1j`,
		`function selectAfterRequestOrchestrationResultOr1j`,
		`function applyOrchestrationStaleProposalTraceOr1j`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P242 OR-1j stale proposal serving marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P252EC1aEntityCoprocessorContractMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`entityCoprocessorContractPolicyVersion = "ec1a.v1"`,
		`entityCoprocessorInputSurfaces = ["characters", "pending_threads", "latest_direct_evidence", "recent_raw_turn"]`,
		`entityCoprocessorInputFocusSignals = ["relation_change", "emotion_drift", "continuity_risk", "scene_carryover"]`,
		`entityCoprocessorOutputProposalTypes = ["relation_drift_hint", "emotion_drift_hint", "continuity_risk_hint", "scene_carryover_candidate", "evidence_bound_patch_proposal"]`,
		`input_contract_policy_version: entityCoprocessorContractPolicyVersion`,
		`input_contract_surfaces: entityCoprocessorInputSurfaces.slice()`,
		`input_contract_focus_signals: entityCoprocessorInputFocusSignals.slice()`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P252 EC-1a entity coprocessor contract marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P255EC1bEntityEvidenceFieldsMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`entityCoprocessorEvidenceContractPolicyVersion = "ec1b.v1"`,
		`entityCoprocessorOutputRequiredFields = coprocessorProposalTraceRequiredFields.slice()`,
		`entityCoprocessorOutputMode = "proposal_trace_only"`,
		`entityCoprocessorEvidenceBindingMode = "required_for_all_outputs"`,
		`output_contract_policy_version: entityCoprocessorEvidenceContractPolicyVersion`,
		`output_contract_required_fields: entityCoprocessorOutputRequiredFields.slice()`,
		`output_contract_evidence_binding: entityCoprocessorEvidenceBindingMode`,
		`output_contract_mode: entityCoprocessorOutputMode`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P255 EC-1b evidence fields marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P260EC1cPatchProposalTruthGuardMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`entityCoprocessorPatchDirectWriteGuardPolicyVersion = "ec1c.v1"`,
		`entityCoprocessorPatchDirectWriteBlockedTarget = "canonical_relationship_state"`,
		`entityCoprocessorPatchDirectWriteAllowedRoute = "proposal_trace_to_reducer_reentry_only"`,
		`entityCoprocessorPatchDirectWriteAuthorityCeiling = "step11_truth_core_only"`,
		`patch_direct_write_guard_policy_version: entityCoprocessorPatchDirectWriteGuardPolicyVersion`,
		`patch_direct_write_guard_blocked_target: entityCoprocessorPatchDirectWriteBlockedTarget`,
		`patch_direct_write_guard_authority_ceiling: entityCoprocessorPatchDirectWriteAuthorityCeiling`,
		`patch_direct_write_guard_allowed_route: entityCoprocessorPatchDirectWriteAllowedRoute`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P260 EC-1c patch proposal truth guard marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P266EC1dRelationDriftTraceSplitMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`entityCoprocessorTraceDisplayPolicyVersion = "ec1d.v1"`,
		`entityCoprocessorTraceDisplayMode = "hint_vs_current_fact_split"`,
		`entityCoprocessorTraceDisplayHintLane = "entity_proposal_hint"`,
		`entityCoprocessorTraceDisplayTruthLane = "step11_current_fact"`,
		`entityCoprocessorTraceDisplayTruthTargets = ["current_fact", "canonical_relationship_state"]`,
		`entityCoprocessorTraceDisplayDisallowedAliases = ["proposal_trace_as_current_fact", "relation_drift_hint_as_canonical_relationship_state"]`,
		`trace_display_policy_version: entityCoprocessorTraceDisplayPolicyVersion`,
		`trace_display_mode: entityCoprocessorTraceDisplayMode`,
		`trace_display_hint_lane: entityCoprocessorTraceDisplayHintLane`,
		`trace_display_truth_lane: entityCoprocessorTraceDisplayTruthLane`,
		`trace_display_disallowed_aliases: entityCoprocessorTraceDisplayDisallowedAliases.slice()`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P266 EC-1d relation drift trace split marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P272EC1eStaleSceneTruthGuardMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`entityCoprocessorStaleSceneGuardPolicyVersion = "ec1e.v1"`,
		`entityCoprocessorStaleSceneGuardMode = "canonical_conflict_guard_bridge"`,
		`entityCoprocessorStaleSceneGuardInputLabels = ["characters", "pending_threads"]`,
		`entityCoprocessorStaleSceneGuardAnchorSource = "canonical_state_layer"`,
		`entityCoprocessorStaleSceneGuardConflictReason = "canonical_polarity_conflict"`,
		`entityCoprocessorStaleSceneGuardSuppressionTarget = "scene_carryover_candidate"`,
		`entityCoprocessorStaleSceneGuardSuppressionRoute = "drop_entity_hint_keep_truth_floor"`,
		`stale_scene_guard_policy_version: entityCoprocessorStaleSceneGuardPolicyVersion`,
		`stale_scene_guard_mode: entityCoprocessorStaleSceneGuardMode`,
		`stale_scene_guard_conflict_reason: entityCoprocessorStaleSceneGuardConflictReason`,
		`stale_scene_guard_suppression_target: entityCoprocessorStaleSceneGuardSuppressionTarget`,
		`stale_scene_guard_suppression_route: entityCoprocessorStaleSceneGuardSuppressionRoute`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P272 EC-1e stale scene truth guard marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P278EC1fDriveLatticeGuidanceOnlyMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`entityCoprocessorBranchRegistryPolicyVersion = "ec1f.v1"`,
		`entityCoprocessorBranchRegistryCanonicalSource = "relationship_state.drive_lattice"`,
		`entityCoprocessorBranchRegistrySignalKeys = ["pull", "alarm", "scar", "veil", "tether", "lock"]`,
		`entityCoprocessorBranchRegistrySignalClass = "relationship_dynamics"`,
		`entityCoprocessorBranchRegistryProposalScope = "guidance_only"`,
		`entityCoprocessorBranchRegistryTruthPathAllowed = false`,
		`entityCoprocessorBranchRegistryReducerReentryAllowed = false`,
		`entityCoprocessorBranchRegistryCanonicalWriteAllowed = false`,
		`branch_registry_policy_version: entityCoprocessorBranchRegistryPolicyVersion`,
		`branch_registry_canonical_source: entityCoprocessorBranchRegistryCanonicalSource`,
		`branch_registry_signal_keys: entityCoprocessorBranchRegistrySignalKeys.slice()`,
		`branch_registry_signal_class: entityCoprocessorBranchRegistrySignalClass`,
		`branch_registry_proposal_scope: entityCoprocessorBranchRegistryProposalScope`,
		`branch_registry_truth_path_allowed: entityCoprocessorBranchRegistryTruthPathAllowed`,
		`branch_registry_canonical_write_allowed: entityCoprocessorBranchRegistryCanonicalWriteAllowed`,
		`branch_registry_reducer_reentry_allowed: entityCoprocessorBranchRegistryReducerReentryAllowed`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P278 EC-1f drive lattice guidance-only marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P287WC1aWorldCoprocessorContractMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`worldCoprocessorContractPolicyVersion = "wc1a.v1"`,
		`worldCoprocessorInputSurfaces = ["world_context"`,
		`"storylines"`,
		`"latest_direct_evidence"`,
		`"recent_raw_turn"`,
		`worldCoprocessorInputFocusSignals = ["faction_pressure"`,
		`"region_pressure"`,
		`"offscreen_thread_pressure"`,
		`"public_pressure"`,
		`"propagation_risk"`,
		`"scene_scope"`,
		`worldCoprocessorOutputProposalTypes = ["offscreen_pressure_hint"`,
		`"faction_region_pressure_summary"`,
		`"public_pressure_summary"`,
		`"propagation_risk_proposal"`,
		`"scene_scoped_setting_hint"`,
		`input_contract_policy_version: worldCoprocessorContractPolicyVersion`,
		`input_contract_surfaces: worldCoprocessorInputSurfaces.slice()`,
		`input_contract_focus_signals: worldCoprocessorInputFocusSignals.slice()`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P287 WC-1a world coprocessor contract marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P290WC1bWorldWriteGuardMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`worldCoprocessorWriteGuardPolicyVersion = "wc1b.v1"`,
		`worldCoprocessorWriteGuardBlockedTargets = ["scene_rules"`,
		`"persistent_rules"`,
		`"canonical_world_current_state"`,
		`patch_direct_write_guard_policy_version: worldCoprocessorWriteGuardPolicyVersion`,
		`patch_direct_write_guard_blocked_targets: worldCoprocessorWriteGuardBlockedTargets.slice()`,
		`patch_direct_write_guard_allowed_route: worldCoprocessorWriteGuardAllowedRoute`,
		`patch_direct_write_guard_authority_ceiling: worldCoprocessorWriteGuardAuthorityCeiling`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P290 WC-1b world write guard marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P293WC1cWorldPressureTraceSplitMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`worldCoprocessorTraceDisplayPolicyVersion = "wc1c.v1"`,
		`worldCoprocessorTraceDisplayMode = "hint_vs_current_world_state_split"`,
		`worldCoprocessorTraceDisplayHintLane = "world_pressure_hint"`,
		`worldCoprocessorTraceDisplayTruthLane = "step11_world_current_state"`,
		`worldCoprocessorTraceDisplayTruthTargets = ["scene_rules", "persistent_rules", "canonical_world_current_state"]`,
		`worldCoprocessorTraceDisplayDisallowedAliases = ["world_pressure_hint_as_current_world_state", "scene_scoped_setting_hint_as_scene_rule"]`,
		`trace_display_policy_version: worldCoprocessorTraceDisplayPolicyVersion`,
		`trace_display_mode: worldCoprocessorTraceDisplayMode`,
		`trace_display_hint_lane: worldCoprocessorTraceDisplayHintLane`,
		`trace_display_truth_lane: worldCoprocessorTraceDisplayTruthLane`,
		`trace_display_disallowed_aliases: worldCoprocessorTraceDisplayDisallowedAliases.slice()`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P293 WC-1c world pressure trace split marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P298WC1dPropagationRiskGuidanceBudgetMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`worldCoprocessorGuidanceBudgetPolicyVersion = "wc1d.v1"`,
		`worldCoprocessorGuidanceBudgetMode = "guidance_only_competition"`,
		`worldCoprocessorGuidanceBudgetSource = coprocessorOrchestrationResidualBudgetSource`,
		`worldCoprocessorGuidanceBudgetLane = coprocessorHintBudgetLane`,
		`guidance_budget_policy_version: worldCoprocessorGuidanceBudgetPolicyVersion`,
		`guidance_budget_mode: worldCoprocessorGuidanceBudgetMode`,
		`guidance_budget_lane: worldCoprocessorGuidanceBudgetLane`,
		`guidance_budget_source: worldCoprocessorGuidanceBudgetSource`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P298 WC-1d propagation risk guidance budget marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P301WC1eConservativeDegradeGuardMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`worldCoprocessorConservativeDegradePolicyVersion = "wc1e.v1"`,
		`worldCoprocessorConservativeDegradeMode = "evidence_bound_conservative_degrade"`,
		`worldCoprocessorConservativeDegradeReasonCodes = ["reliability_guard_hold", "ingest_only_not_injected"]`,
		`worldCoprocessorConservativeDegradeMissingEvidenceAction = "drop_world_patch_proposal"`,
		`worldCoprocessorConservativeDegradeLowConfidenceAction = "degrade_to_hint_only_or_skip"`,
		`worldCoprocessorConservativeDegradeTruthFloorFallback = "preserve_step11_world_current_state"`,
		`conservative_degrade_policy_version: worldCoprocessorConservativeDegradePolicyVersion`,
		`conservative_degrade_mode: worldCoprocessorConservativeDegradeMode`,
		`conservative_degrade_missing_evidence_action: worldCoprocessorConservativeDegradeMissingEvidenceAction`,
		`conservative_degrade_low_confidence_action: worldCoprocessorConservativeDegradeLowConfidenceAction`,
		`conservative_degrade_truth_floor_fallback: worldCoprocessorConservativeDegradeTruthFloorFallback`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P301 WC-1e conservative degrade guard marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P304WC1fSceneScopedSettingFrameMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`worldCoprocessorSceneSlicePolicyVersion = "wc1f.v1"`,
		`worldCoprocessorSceneSliceSelectorSignal = "scene_scope"`,
		`worldCoprocessorSceneSliceExtractionMode = "current_scene_thin_frame_only"`,
		`worldCoprocessorSceneSliceOutputType = "scene_scoped_setting_hint"`,
		`worldCoprocessorSceneSliceRequiredFields = ["scope_name", "scene_rule_anchor"]`,
		`worldCoprocessorSceneSliceDeliverySurface = "guidance_only_setting_frame"`,
		`worldCoprocessorSceneSliceTruthAliasBlocked = true`,
		`worldCoprocessorSceneSliceTruthBorrowAllowed = false`,
		`scene_slice_policy_version: worldCoprocessorSceneSlicePolicyVersion`,
		`scene_slice_selector_signal: worldCoprocessorSceneSliceSelectorSignal`,
		`scene_slice_extraction_mode: worldCoprocessorSceneSliceExtractionMode`,
		`scene_slice_output_type: worldCoprocessorSceneSliceOutputType`,
		`scene_slice_truth_alias_blocked: worldCoprocessorSceneSliceTruthAliasBlocked`,
		`scene_slice_truth_borrow_allowed: worldCoprocessorSceneSliceTruthBorrowAllowed`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P304 WC-1f scene-scoped setting frame marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P312NQ1aNarrativeQualityContractMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`narrativeQualityCoprocessorContractPolicyVersion = "nq1a.v1"`,
		`narrativeQualityLayerLabels = ["narrative_guide"`,
		`narrativeQualityLayerPolicyVersion`,
		`narrativeQualityCoprocessorSourceRoles`,
		`narrativeQualityLayerMode = "quality_hint_only"`,
		`narrativeQualityLayerDisallowedUsage = ["truth_arbitration"`,
		`"canonical_overwrite"`,
		`"current_fact_override"`,
		`input_contract_policy_version: narrativeQualityCoprocessorContractPolicyVersion`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P312 NQ-1a narrative quality contract marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P315NQ1bPacingCallbackObligationMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`narrativeQualityCoprocessorOutputPolicyVersion = "nq1b.v1"`,
		`narrativeQualityCoprocessorOutputMode = "guidance_trace_only"`,
		`narrativeQualityCoprocessorOutputHintTypes = ["pacing_hint"`,
		`"scene_obligation_reminder"`,
		`"callback_opportunity_hint"`,
		`"emphasis_ordering_proposal"`,
		`narrativeQualityCoprocessorOutputRequiredFields = ["hint_type"`,
		`"hint_text"`,
		`"priority"`,
		`output_contract_policy_version: narrativeQualityCoprocessorOutputPolicyVersion`,
		`output_contract_hint_types: narrativeQualityCoprocessorOutputHintTypes.slice()`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P315 NQ-1b pacing/callback/obligation marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P318NQ1cConflictGuardMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`narrativeQualityCoprocessorConflictGuardPolicyVersion = "nq1c.v1"`,
		`narrativeQualityCoprocessorConflictGuardMode = "current_fact_conflict_auto_degrade"`,
		`narrativeQualityCoprocessorConflictGuardAnchorSource = "step11_truth_core"`,
		`narrativeQualityCoprocessorConflictGuardConflictReason = "supporting_guidance_conflict_blocked"`,
		`narrativeQualityCoprocessorConflictGuardTruthFloorFallback = "preserve_step11_factual_state"`,
		`conflict_guard_policy_version: narrativeQualityCoprocessorConflictGuardPolicyVersion`,
		`conflict_guard_mode: narrativeQualityCoprocessorConflictGuardMode`,
		`conflict_guard_anchor_source: narrativeQualityCoprocessorConflictGuardAnchorSource`,
		`conflict_guard_truth_floor_fallback: narrativeQualityCoprocessorConflictGuardTruthFloorFallback`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P318 NQ-1c conflict guard marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P321NQ1dQualityTraceSplitMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`narrativeQualityCoprocessorTraceDisplayPolicyVersion = "nq1d.v1"`,
		`narrativeQualityCoprocessorTraceDisplayMode = "quality_hint_vs_factual_state_split"`,
		`narrativeQualityCoprocessorTraceDisplayHintLane = "response_quality_hint"`,
		`narrativeQualityCoprocessorTraceDisplayTruthLane = "step11_factual_state"`,
		`"quality_hint_as_current_fact"`,
		`"quality_hint_as_canonical_state"`,
		`trace_display_policy_version: narrativeQualityCoprocessorTraceDisplayPolicyVersion`,
		`trace_display_mode: narrativeQualityCoprocessorTraceDisplayMode`,
		`trace_display_hint_lane: narrativeQualityCoprocessorTraceDisplayHintLane`,
		`trace_display_truth_lane: narrativeQualityCoprocessorTraceDisplayTruthLane`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P321 NQ-1d quality trace split marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P324NQ1eShortLongAblationMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`narrativeQualityCoprocessorAblationPolicyVersion = "nq1e.v1"`,
		`narrativeQualityCoprocessorAblationBaselineMode = "narrative_quality_off"`,
		`narrativeQualityCoprocessorAblationProfiles = ["short_session"`,
		`"long_session"`,
		`narrativeQualityCoprocessorAblationPrimaryMetrics = ["hint_acceptance_delta"`,
		`"conflict_free_guidance_rate"`,
		`"response_coherence_delta"`,
		`narrativeQualityCoprocessorAblationTruthLeakBudget = "zero_tolerance"`,
		`ablation_policy_version: narrativeQualityCoprocessorAblationPolicyVersion`,
		`ablation_baseline_mode: narrativeQualityCoprocessorAblationBaselineMode`,
		`ablation_profiles: narrativeQualityCoprocessorAblationProfiles.slice()`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P324 NQ-1e short/long ablation marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P327NQ1fPlannerScenePilotSplitMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`narrativeQualityCoprocessorPlannerSplitPolicyVersion = "nq1f.v1"`,
		`narrativeQualityCoprocessorPlannerSplitMode = "beat_planner_vs_scene_pilot_guidance_only"`,
		`narrativeQualityCoprocessorPlannerRole = "beat_planner"`,
		`narrativeQualityCoprocessorExecutionRole = "scene_pilot"`,
		`planner_split_policy_version: narrativeQualityCoprocessorPlannerSplitPolicyVersion`,
		`planner_split_mode: narrativeQualityCoprocessorPlannerSplitMode`,
		`planner_role: narrativeQualityCoprocessorPlannerRole`,
		`execution_role: narrativeQualityCoprocessorExecutionRole`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P327 NQ-1f planner/scene-pilot split marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P336VX1aAuthorityLeakReplayMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`coprocessorAuthorityMatrixPolicyVersion = "mg1a.v1"`,
		`coprocessorTruthWriteTargets = ["current_fact"`,
		`"canonical_state"`,
		`"canonical_state_layer"`,
		`"dense_summary"`,
		`truthFloorBudgetLane = "truth_floor"`,
		`coprocessorHintBudgetLane = "coprocessor_hint"`,
		`narrativeQualityCoprocessorAblationProtectedTruthLabels = truthFloorBudgetLabels.slice()`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P336 VX-1a authority leak replay marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P339VX1bRuntimeBlastRadiusMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13GovernorFailureRuntimeAction = "trace_only_no_prompt_mutation"`,
		`step13GovernorFailureFailOpenBehavior = "keep_current_turn_execution_and_truth_floor"`,
		`failure_runtime_action: step13GovernorFailureRuntimeAction`,
		`failure_fail_open_behavior: step13GovernorFailureFailOpenBehavior`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P339 VX-1b runtime blast radius marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P342VX1cTraceObservabilityMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`coprocessorAnalysisProviderTraceTargetsByModule = {`,
		`entity_coprocessor: "proposal_trace"`,
		`world_coprocessor: "proposal_trace"`,
		`narrative_quality_coprocessor: "guidance_trace"`,
		`coprocessorAntiCopyAllowedReferenceMode = "behavioral_reference_only"`,
		`allowed_reference_mode: coprocessorAntiCopyAllowedReferenceMode`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P342 VX-1c trace observability marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P345VX1dBudgetPollutionMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`truthFloorBudgetLabels = [`,
		`coprocessorHintBudgetPromptLabels = [`,
		`coprocessorHintBudgetPromptModules = [`,
		`truthFloorBudgetLane = "truth_floor"`,
		`coprocessorHintBudgetLane = "coprocessor_hint"`,
		`coprocessorBudgetIsolationPolicyVersion = "mg1d.v1"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P345 VX-1d budget pollution marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P348VX1eModuleAblationCompareMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`entityCoprocessorAblationPolicyVersion = "vx1e.entity.v1"`,
		`worldCoprocessorAblationPolicyVersion = "vx1e.world.v1"`,
		`narrativeQualityCoprocessorAblationPolicyVersion = "nq1e.v1"`,
		`entityCoprocessorAblationDecisionGate`,
		`worldCoprocessorAblationDecisionGate`,
		`narrativeQualityCoprocessorAblationDecisionGate`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P348 VX-1e module ablation compare marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P352VX1fDefaultTakeoverGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`coprocessorTakeoverGatePolicyVersion = "vx1f.v1"`,
		`coprocessorTakeoverGateRequiredSignals = ["module_replay_green"`,
		`"module_ablation_green"`,
		`"release_gate_green"`,
		`entityCoprocessorTakeoverGateDefaultAction = "stay_off_until_gate_green"`,
		`worldCoprocessorTakeoverGateDefaultAction = "stay_off_until_gate_green"`,
		`narrativeQualityCoprocessorTakeoverGateDefaultAction = "stay_experimental_shadow_until_gate_green"`,
		`takeover_gate_policy_version: coprocessorTakeoverGatePolicyVersion`,
		`takeover_gate_required_signals: coprocessorTakeoverGateRequiredSignals.slice()`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P352 VX-1f default takeover gate marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P356VX1gProposalReentryLeakReplayMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`coprocessorProposalReentryPolicyVersion = "mg1g.v1"`,
		`coprocessorProposalTraceRequiredFields = ["evidence_refs"`,
		`"source_turns"`,
		`"confidence"`,
		`coprocessorProposalAdoptionPath = ["proposal_trace"`,
		`"reducer_reentry"`,
		`"step11_truth_core"]`,
		`truth_path_blocked_without_required_fields: true`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P356 VX-1g proposal re-entry leak replay marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P359VX1hRebuildInvalidationReplayMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`coprocessorOrchestrationRebuildPolicyVersion = "or1i.v1"`,
		`coprocessorOrchestrationStaleProposalPolicyVersion = "or1j.v1"`,
		`coprocessorOrchestrationRollbackInvalidationPolicyVersion = "or1g.v1"`,
		`coprocessorOrchestrationDirtyMatrixPolicyVersion = "or1h.v1"`,
		`staleServingPolicy: "deny_stale_sidecar_on_rebuild_pending"`,
		`cacheReuseGuard: "rollback_and_guidance_token_drift_block_pending_ready_reuse"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P359 VX-1h rebuild/invalidation replay marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P372ReleaseGateBundleRuntimeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const _step17ReleaseGate = {`,
		`step17ReleaseGateFetch()`,
		`bridgeFetch("/metrics/lc1s/step17-bundle-closure"`,
		`bundleClosure`,
		`release_gate_green`,
		`step17_bundle_closure`,
		`Bundle closure and session-local adoption are separate truths here`,
		`branch`,
		`commit`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P372 release gate bundle runtime marker %q", needle)
		}
	}
}
