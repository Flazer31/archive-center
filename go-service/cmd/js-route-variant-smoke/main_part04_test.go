package main

import (
	"strings"
	"testing"
)

func TestArchiveCenterJSSeq12P373ReleaseGateModuleDefaultMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`default_mode: "off"`,
		`default_mode: "experimental"`,
		`default_mode: "conservative"`,
		`default_reason: "entity_sidecar_stays_off_until_vx_validation_and_release_gate_close"`,
		`default_reason: "world_sidecar_stays_off_until_vx_validation_and_release_gate_close"`,
		`default_reason: "quality hints stay experimental until vx_validation_and_release_gate_close"`,
		`default_reason: "diagnostic_path_stays_conservative_and_fail_open"`,
		`default_reason: "audit_recall_should_remain_conservative_until_release_gate"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P373 module default marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P374ReleaseGatePackagedBundleSmokeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const coprocessorFeatureControlMatrix = [`,
		`policyVersion: "or1b.v1"`,
		`timeoutWindowSource: "getOrchestrationTimeoutMs"`,
		`step13GovernorMaxParallelism = 1`,
		`step13GovernorFailureRuntimeAction = "trace_only_no_prompt_mutation"`,
		`entityCoprocessorTraceDisplayPolicyVersion = "ec1d.v1"`,
		`worldCoprocessorTraceDisplayPolicyVersion = "wc1c.v1"`,
		`narrativeQualityCoprocessorTraceDisplayPolicyVersion = "nq1d.v1"`,
		`coprocessorOrchestrationRollbackInvalidationPolicyVersion = "or1g.v1"`,
		`staleServingPolicy: "deny_stale_sidecar_on_rebuild_pending"`,
		`entityCoprocessorKillSwitch`,
		`worldCoprocessorKillSwitch`,
		`narrativeQualityCoprocessorKillSwitch`,
		`kill_switch_default_state: "armed_standby"`,
		`kill_switch_action: "fail_open_skip"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P374 packaged bundle smoke marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P375ReleaseGateStep12CompleteStep13DeferredMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13PlanningRuntimeMode = "guidance_only_schema_draft"`,
		`step13PlanningRolloutStage = "experimental_shadow"`,
		`step13PlanningTakeoverGate = "stay_guidance_only_until_vx_green"`,
		`step13PlanningTruthWriteAllowed = false`,
		`step13PlanningReducerReentryAllowed = false`,
		`advancedCachePolicyStatus: "deferred_to_step13"`,
		`cache_advanced_policy_status: "deferred_to_step13"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P375 Step12/Step13 scope marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P379CoprocessorOutputSchemaDecisionMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`entityCoprocessorOutputMode = "proposal_trace_only"`,
		`entityCoprocessorOutputProposalTypes = ["relation_drift_hint"`,
		`entityCoprocessorOutputRequiredFields`,
		`worldCoprocessorOutputMode = "proposal_trace_only"`,
		`worldCoprocessorOutputProposalTypes = ["offscreen_pressure_hint"`,
		`narrativeQualityCoprocessorOutputMode = "guidance_trace_only"`,
		`narrativeQualityCoprocessorOutputHintTypes = ["pacing_hint"`,
		`output_contract_mode: entityCoprocessorOutputMode`,
		`output_contract_proposal_types: entityCoprocessorOutputProposalTypes.slice()`,
		`output_contract_mode: worldCoprocessorOutputMode`,
		`output_contract_proposal_types: worldCoprocessorOutputProposalTypes.slice()`,
		`output_contract_mode: narrativeQualityCoprocessorOutputMode`,
		`output_contract_hint_types: narrativeQualityCoprocessorOutputHintTypes.slice()`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P379 coprocessor output schema decision marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P380PluginOnlyBackendSaveBoundaryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`coprocessorOrchestrationBackendRequiredModules = [`,
		`coprocessorOrchestrationBackendBundleAssistedSurfaces = [`,
		`coprocessorOrchestrationPluginOnlyModules = [`,
		`coprocessorOrchestrationPluginOnlyExecutionMode = "local_runtime_contract_only"`,
		`coprocessorOrchestrationModuleTransportRuntimeStatus = "trace_contract_only"`,
		`backend_required_modules: coprocessorOrchestrationBackendRequiredModules.slice()`,
		`backend_bundle_assisted_surfaces: coprocessorOrchestrationBackendBundleAssistedSurfaces.slice()`,
		`plugin_only_modules: coprocessorOrchestrationPluginOnlyModules.slice()`,
		`current_fact_write: false`,
		`canonical_write: false`,
		`write_targets: ["proposal_trace"]`,
		`write_targets: ["guidance_trace"]`,
		`denied_write_targets: coprocessorTruthWriteTargets.slice()`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P380 plugin-only backend save boundary marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P381SidecarTurnLocalSurfaceSaveMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`coprocessorSidecarWritableTargets = ["proposal_trace", "guidance_trace", "audit_trace", "maintenance_metadata"]`,
		`entityCoprocessorTraceDisplayMode = "hint_vs_current_fact_split"`,
		`worldCoprocessorTraceDisplayMode = "hint_vs_current_world_state_split"`,
		`narrativeQualityCoprocessorTraceDisplayMode = "quality_hint_vs_factual_state_split"`,
		`entityCoprocessorTraceDisplayDisallowedAliases = ["proposal_trace_as_current_fact"`,
		`trace_display_mode: entityCoprocessorTraceDisplayMode`,
		`trace_display_disallowed_aliases: entityCoprocessorTraceDisplayDisallowedAliases.slice()`,
		`trace_display_mode: worldCoprocessorTraceDisplayMode`,
		`trace_display_disallowed_aliases: worldCoprocessorTraceDisplayDisallowedAliases.slice()`,
		`trace_display_mode: narrativeQualityCoprocessorTraceDisplayMode`,
		`trace_display_disallowed_aliases: narrativeQualityCoprocessorTraceDisplayDisallowedAliases.slice()`,
		`coprocessorOrchestrationStaleProposalBlockedPromptTargets = ["pending_ready_orchestration_result", "proposal_trace", "guidance_trace", "sidecar_cache"]`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P381 sidecar turn-local surface save marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P382RebuildFullVsSelectiveCheckpointMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`coprocessorOrchestrationRebuildStaleDropTargets = ["pending_ready_orchestration_result", "sidecar_cache", "guidance_trace"]`,
		`coprocessorOrchestrationRebuildHardResetTargets = ["prepare_turn_bundle"`,
		`coprocessorOrchestrationRebuildStartPointPrecedence = ["checkpoint_full_rebuild", "rollback_turn_anchor_then_prepare_turn", "next_narrative_control_fetch", "next_prepare_turn_fetch", "step11_truth_stack"]`,
		`schema_migration: "full"`,
		`turn_deletion: "rollback_turn_anchor_then_prepare_turn"`,
		`schema_migration: "checkpoint_full_rebuild"`,
		`rebuildMode: "full"`,
		`rebuildMode: "selective"`,
		`startPoint: "checkpoint_full_rebuild"`,
		`startPoint: "rollback_turn_anchor_then_prepare_turn"`,
		`staleServingPolicy: "deny_stale_sidecar_on_rebuild_pending"`,
		`cacheReuseAllowed: pendingReasons.length === 0`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P382 rebuild full vs selective checkpoint marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq13P43SidecarCallEconomyGovernorMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`coprocessorAnalysisProviderCallFrequencyOwner = "step13_governor"`,
		`step13GovernorPolicyVersion = "gv1a.v1"`,
		`step13CacheKeeperPolicyVersion = "gv1b.v1"`,
		`step13GovernorApprovalStates = ["run", "reuse", "skip", "suspend"]`,
		`step13GovernorMaxParallelism = 1`,
		`step13GovernorRuntimeMode = "trace_and_transport_aligned"`,
		`analysis_provider_call_frequency_owner: coprocessorAnalysisProviderCallFrequencyOwner`,
		`call_frequency_owner: coprocessorAnalysisProviderCallFrequencyOwner`,
		`call_frequency_policy_version: step13GovernorPolicyVersion`,
		`call_frequency_policy_status: "governor_contract_fixed"`,
		`forced_refresh_signals: step13CacheKeeperForcedRefreshSignals.slice()`,
		`no_serve_conditions: step13CacheKeeperNoServeConditions.slice()`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P43 sidecar call economy marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq13P44TruthFloorTokenBudgetMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13TokenTruthFloorPolicyVersion = "tb1b.v1"`,
		`step13TokenTruthFloorMode = "char_floor_projected_minimums"`,
		`step13TokenTruthFloorProjectionStatus = "contract_only_no_runtime_enforcement"`,
		`step13TokenTruthFloorReferenceCharsPerToken = 4`,
		`step13TokenTruthFloorCoreLabels = ["latest_direct_evidence", "recent_raw_turn", "active_state", "canonical_state_layer"]`,
		`step13TokenTruthFloorContinuityLabels = ["storylines", "episode", "chapter", "arc", "saga"]`,
		`truth_floor_reservation_mode: "hard_floor_reserved_first"`,
		`truth_floor_min_tokens_by_label: Object.assign({}, step13TokenTruthFloorMinTokensByLabel)`,
		`truth_floor_core_min_tokens: step13TokenTruthFloorCoreMinTokens`,
		`truth_floor_continuity_min_tokens: step13TokenTruthFloorContinuityMinTokens`,
		`truth_floor_reliability_guard_min_tokens: step13TokenTruthFloorReliabilityGuardMinTokens`,
		`truth_floor_total_min_tokens: step13TokenTruthFloorTotalMinTokens`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P44 truth floor token budget marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq13P45PlanningSurfaceGuidanceOnlyMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13PlanningRuntimeMode = "guidance_only_schema_draft"`,
		`step13PlanningRolloutStage = "experimental_shadow"`,
		`step13PlanningTakeoverGate = "stay_guidance_only_until_vx_green"`,
		`step13PlanningTruthWriteAllowed = false`,
		`step13PlanningReducerReentryAllowed = false`,
		`step13BeatPlannerDraftRequiredFields = ["narrative_goal", "tension_axis", "open_question", "payoff_candidate"]`,
		`step13ScenePilotDraftRequiredFields = ["execution_mode", "scene_target", "emphasis_axis", "pacing", "forbidden_move"]`,
		`step13SettingFrameSchemaPolicyVersion = "ps1c.v1"`,
		`worldCoprocessorSceneSliceDeliverySurface = "guidance_only_setting_frame"`,
		`truth_write_allowed: step13PlanningTruthWriteAllowed`,
		`reducer_reentry_allowed: step13PlanningReducerReentryAllowed`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P45 planning guidance-only marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq13P46LocalNamingGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13NamingGatePolicyVersion = "ps1f.v1"`,
		`step13NamingGateMode = "local_vocab_review_only"`,
		`step13NamingGateLegacyRenameScope = "step_owned_contract_values_only"`,
		`step13NamingGateReviewScope = ["planning", "governor", "portability"]`,
		`step13NamingGateSourceDraft = "STEP13_NAMING_MAP_DRAFT.md"`,
		`step13NamingGateApprovalRule = "allow_local_vocab_block_external_like_reuse"`,
		`step13NamingGateRuntimeAction = "review_only_no_runtime_rename"`,
		`reviewed_function_names: step13NamingGateReviewedFunctionNames.slice()`,
		`reviewed_helper_names: step13NamingGateReviewedHelperNames.slice()`,
		`blocked_legacy_values: step13NamingGateBlockedLegacyValues.slice()`,
		`"book_author"`,
		`"director_directive"`,
		`"guidance_only_world_slice"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P46 local naming gate marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq13P56MemOrchNullReturnHardeningMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`function buildIntentionalOrchestrationSkipResult(reason, trace)`,
		`_orchestrationSkipped: true`,
		`_skipReason: normalizedReason`,
		`function isIntentionalOrchestrationSkipResult(result)`,
		`return !!(result && result._orchestrationSkipped === true)`,
		`return buildIntentionalOrchestrationSkipResult("empty_input_no_continuity", trace)`,
		`if (!lastOrchResult) {`,
		`orchestration returned null`,
		`detail: "llm_gate_blocked"`,
		`if (isIntentionalOrchestrationSkipResult(lastOrchResult)) {`,
		`resolveOrchestrationFallbackRouteOr1b("intentional_skip")`,
		`status: "skipped"`,
		`code: skipFallbackRoute.route`,
		`return applyProtectionOnlyInjection(payload, userInput)`,
		`supportedRoutes: ["blocked_empty_result", "cached_result", "skip_protection_only", "direct_result"]`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P56 MemOrch null-return hardening marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P129GV1aModuleGovernorContractMarkers verifies GV-1a:
// step13.governor module policy contract markers are present in Archive Center.js
func TestArchiveCenterJSSeq13P129GV1aModuleGovernorContractMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`policyVersion: "gv1a.v1"`,
		`owner: "step13.governor"`,
		`governedModules: ["entity_coprocessor", "world_coprocessor", "narrative_quality_coprocessor"]`,
		`approvalStates: ["run", "reuse", "skip", "suspend"]`,
		`cooldownTurnsByModule:`,
		`entity_coprocessor: 0`,
		`world_coprocessor: 0`,
		`narrative_quality_coprocessor: 0`,
		`minDirtySeverityByModule:`,
		`entity_coprocessor: "medium"`,
		`world_coprocessor: "medium"`,
		`narrative_quality_coprocessor: "low"`,
		`maxParallelism: 1`,
		`callEntryGate: "after_step11_truth_stack"`,
		`singleFlightScope: "same_chat_session_after_request_only"`,
		`runtimeMode: "trace_and_transport_aligned"`,
		`dirtySeverityOrder: ["none", "low", "medium", "high", "critical"]`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P129 GV-1a marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P132GV1bCacheKeeperPolicyMarkers verifies GV-1b:
// Cache keeper policy contract markers are present in Archive Center.js
func TestArchiveCenterJSSeq13P132GV1bCacheKeeperPolicyMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`policyVersion: "gv1b.v1"`,
		`owner: "step13.governor"`,
		`cacheUnit:`,
		`reuseScope:`,
		`reuseRoute:`,
		`staleGuard:`,
		`staleDiscardReasons:`,
		`"cache_key_mismatch"`,
		`"session_scope_mismatch"`,
		`"rollback_invalidation_drift"`,
		`"guidance_invalidation_drift"`,
		`"rebuild_pending"`,
		`"chapter_evidence_mismatch"`,
		`forcedRefreshSignals:`,
		`noServeConditions:`,
		`"blocked_empty_result"`,
		`"stale_sidecar_blocked"`,
		`"backend_offline"`,
		`runtimeServeMode:`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P132 GV-1b marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P135GV1cFailureBudgetSuspensionMarkers verifies GV-1c:
// Failure budget suspension policy contract markers are present in Archive Center.js
func TestArchiveCenterJSSeq13P135GV1cFailureBudgetSuspensionMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`policyVersion: "gv1c.v1"`,
		`owner: "step13.governor"`,
		`governedModules:`,
		`failureBucketMode: "shared_sidecar_channel"`,
		`trackedFailureClasses:`,
		`"plugin_main_error"`,
		`"sub_review_error"`,
		`"supervisor_unavailable"`,
		`"delivery_gate_blocked"`,
		`failureWindowTurns: 4`,
		`consecutiveFailureThreshold: 2`,
		`resumeSuccessTurnsRequired: 1`,
		`suspensionDecisionMode: "shared_suspend_all_governed_modules"`,
		`suspensionReasonCode: "failure_budget_suspended"`,
		`runtimeAction: "trace_only_no_prompt_mutation"`,
		`failOpenBehavior: "keep_current_turn_execution_and_truth_floor"`,
		`historyScope: "same_chat_session_recent_turns"`,
		`function resolveStep13FailureBudgetStateGv1c`,
		`function applyStep13GovernorFailureBudgetTraceGv1c`,
		`function recordStep13GovernorTurnOutcomeGv1c`,
		`function buildStep13GovernorTurnOutcomeGv1c`,
		`function peekStep13FailureBudgetStateGv1c`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P135 GV-1c marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P138GV1dModuleRunLedgerMarkers verifies GV-1d:
// Module run ledger contract markers are present in Archive Center.js
func TestArchiveCenterJSSeq13P138GV1dModuleRunLedgerMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`policyVersion: "gv1d.v1"`,
		`governorPolicyVersion:`,
		`cachePolicyVersion:`,
		`runtimeMode:`,
		`maxParallelism:`,
		`primaryDecision:`,
		`primaryReason:`,
		`counts:`,
		`run:`,
		`reuse:`,
		`skip:`,
		`suspend:`,
		`entries:`,
		`module:`,
		`decision:`,
		`approvalState:`,
		`reason:`,
		`cooldownTurns:`,
		`minDirtySeverity:`,
		`reusedCache:`,
		`forcedRefresh:`,
		`noServe:`,
		`function resolveStep13GovernorLedgerGv1d`,
		`function applyStep13GovernorTraceGv1d`,
		`runLedgerPolicyVersion: ledger.policyVersion`,
		`primaryDecision: ledger.primaryDecision`,
		`primaryReason: ledger.primaryReason`,
		`entries: Array.isArray(ledger.entries)`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P138 GV-1d marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P141GV1eGovernorBypassGuardMarkers verifies GV-1e:
// Governor bypass guard contract markers are present in Archive Center.js
func TestArchiveCenterJSSeq13P141GV1eGovernorBypassGuardMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`policyVersion: "gv1e.v1"`,
		`owner: "step13.governor"`,
		`governedModules:`,
		`protectedPromptTargets:`,
		`"proposal_trace"`,
		`"guidance_trace"`,
		`"pending_ready_orchestration_result"`,
		`blockedRoutes:`,
		`"self_triggered_sidecar_rerun"`,
		`"proposal_trace_direct_reentry"`,
		`"guidance_trace_recursive_requeue"`,
		`"pending_ready_self_rearm"`,
		`requiredEntryGate:`,
		`requiredSingleFlightScope:`,
		`requiredReuseRoute:`,
		`requiredRuntimeStage: "residual_guidance"`,
		`action: "deny_and_trace_only"`,
		`runtimeStatus: "contract_only"`,
		`function getStep13GovernorBypassPolicyGv1e`,
		`function applyStep13GovernorBypassTraceGv1e`,
		`trace.step13Governor.bypassGuard`,
		`bypassPolicyVersion:`,
		`bypassGuard:`,
		`protectedModules:`,
		`protectedPromptTargets:`,
		`blockedRoutes:`,
		`requiredEntryGate:`,
		`requiredSingleFlightScope:`,
		`requiredReuseRoute:`,
		`requiredRuntimeStage:`,
		`action:`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P141 GV-1e marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P147TB1aProfileModelTokenEstimatorMarkers verifies TB-1a:
// profile/model token estimator contract markers are present in Archive Center.js.
func TestArchiveCenterJSSeq13P147TB1aProfileModelTokenEstimatorMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13TokenEstimatorPolicyVersion = "tb1a.v1"`,
		`step13TokenEstimatorMode = "profile_first_shared_thresholds"`,
		`step13TokenEstimatorModelMode = "shared_thresholds_all_models"`,
		`step13TokenEstimatorModelOverrides = {}`,
		`step13TokenEstimatorProfiles = [`,
		`"mid_context_300k"`,
		`"wide_context_500k"`,
		`"ultra_long_1m_plus"`,
		`"extreme_long_2m_plus"`,
		`step13TokenEstimatorProfileSourceOrder = ["injection_pack_hint", "plugin_setting", "runtime_tokens", "max_injection_chars_proxy"]`,
		`step13TokenEstimatorRuntimeThresholds = {`,
		`wide_context_500k: 300000`,
		`ultra_long_1m_plus: 900000`,
		`extreme_long_2m_plus: 1700000`,
		`const step13TokenEstimatorBudgetLimitByProfile = adaptiveInjectionBudgetProfileLimits();`,
		`step13TokenEstimatorLowConfidenceSources = ["none", "message_char_estimate"]`,
		`step13TokenEstimatorDriftTelemetryFields = [`,
		`"manualBudgetLimit"`,
		`"budgetLimit"`,
		`"budgetLimitSource"`,
		`"runtimeCurrentChatTokens"`,
		`"runtimeCurrentChatTokensEffective"`,
		`"contextProfile"`,
		`"contextProfileSource"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P147 TB-1a marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P150TB1bTruthFloorMinimumTokenMarkers verifies TB-1b:
// truth-floor minimum token contract markers are present in Archive Center.js.
func TestArchiveCenterJSSeq13P150TB1bTruthFloorMinimumTokenMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13TokenTruthFloorPolicyVersion = "tb1b.v1"`,
		`step13TokenTruthFloorMode = "char_floor_projected_minimums"`,
		`step13TokenTruthFloorProjectionStatus = "contract_only_no_runtime_enforcement"`,
		`step13TokenTruthFloorReferenceCharsPerToken = 4`,
		`step13TokenTruthFloorCoreLabels = ["latest_direct_evidence", "recent_raw_turn", "active_state", "canonical_state_layer"]`,
		`step13TokenTruthFloorContinuityLabels = ["storylines", "episode", "chapter", "arc", "saga"]`,
		`step13TokenTruthFloorReliabilityGuardLabels = step13TokenTruthFloorCoreLabels.slice()`,
		`function _projectTruthFloorTokensTb1b(charCount)`,
		`step13TokenTruthFloorMinTokensByLabel[label] = projectedTokens`,
		`step13TokenTruthFloorCoreMinTokens`,
		`step13TokenTruthFloorContinuityMinTokens`,
		`step13TokenTruthFloorReliabilityGuardMinTokens`,
		`truth_floor_budget_lane: truthFloorBudgetLane`,
		`truth_floor_hint_lane_claim_allowed: false`,
		`truth_floor_min_tokens_by_label: Object.assign({}, step13TokenTruthFloorMinTokensByLabel)`,
		`truth_floor_reliability_guard_min_tokens: step13TokenTruthFloorReliabilityGuardMinTokens`,
		`truthFloorBudgetReservedTokens: step13TokenTruthFloorTotalMinTokens`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P150 TB-1b marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P153TB1cDensityProfileMarkers verifies TB-1c:
// token density profile markers for ledger/world/guidance are present in Archive Center.js.
func TestArchiveCenterJSSeq13P153TB1cDensityProfileMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13TokenDensityProfilePolicyVersion = "tb1c.v1"`,
		`step13TokenDensityProfileLevels = ["light", "balanced", "heavy"]`,
		`step13TokenDensityProfileThresholds = {`,
		`light_max_ratio: 0.08`,
		`balanced_max_ratio: 0.15`,
		`function _resolveTokenDensityLevelTb1c(totalRatio)`,
		`function _sumTokenDensityRatiosTb1c(ratioByLabel)`,
		`step13TokenDensityLedgerLabels = ["storylines", "episode", "chapter", "arc", "saga"]`,
		`step13TokenDensityWorldLabels = ["world_context", "location_context", "kg_relations"]`,
		`step13TokenDensityGuidanceLabels = ["narrative_guide"]`,
		`step13TokenDensityRatioByFamily = {`,
		`ledger: {`,
		`world: {`,
		`guidance: {`,
		`density_profile_status: "density_contract_fixed"`,
		`density_profile_families: step13TokenDensityFamilies`,
		`step13TokenDensityProfileByFamily: {`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P153 TB-1c marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P156TB1dBudgetFallbackTokenEstimateMarkers verifies TB-1d:
// budget fallback/token estimate mode markers are present in Archive Center.js.
func TestArchiveCenterJSSeq13P156TB1dBudgetFallbackTokenEstimateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13TokenBudgetFallbackPolicyVersion = "tb1d.v1"`,
		`step13TokenBudgetFallbackRules = {`,
		`reliable_runtime_tokens: {`,
		`low_confidence_runtime_source: {`,
		`runtime_tokens_unavailable: {`,
		`manual_setting_only: {`,
		`budget_limit_source: "runtime_tokens"`,
		`budget_limit_source: "manual_setting"`,
		`profile_source_fallback: "max_injection_chars_proxy"`,
		`adaptive_budget_applied: true`,
		`adaptive_budget_applied: false`,
		`const step13TokenBudgetFallbackDecision = (function resolveStep13TokenBudgetFallbackDecision()`,
		`if (runtimeBudgetAdaptiveEligible) return "reliable_runtime_tokens";`,
		`if (normalizedRuntimeTokenSource === "message_char_estimate") return "low_confidence_runtime_source";`,
		`if (runtimeTieringEnabled) return "runtime_tokens_unavailable";`,
		`return "manual_setting_only";`,
		`step13TokenBudgetFallbackDecision: step13TokenBudgetFallbackDecision`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P156 TB-1d marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P159TB1eTokenReplayTruthFloorMarkers verifies TB-1e:
// token replay window and truth-floor verification markers are present in Archive Center.js.
func TestArchiveCenterJSSeq13P159TB1eTokenReplayTruthFloorMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`runtimeTokenSmoothingWindow = 4`,
		`assembleInjectionWithBudget._runtimeTokenSmoothingState`,
		`const runtimeTokenSmoothingKey = (function resolveRuntimeSmoothingKey()`,
		`runtimeTokenSmoothingState.set(runtimeTokenSmoothingKey, {`,
		`runtimeTokenSmoothingApplied: runtimeTokenSmoothingApplied`,
		`runtimeTokenSmoothingWindow: runtimeTokenSmoothingWindow`,
		`runtimeCurrentChatTokens: Number.isFinite(Number(runtimeTokenHint))`,
		`runtimeCurrentChatTokensEffective: runtimeBudgetAdaptiveEligible ? runtimeTokensForTiering : 0`,
		`drift_telemetry_fields: step13TokenEstimatorDriftTelemetryFields.slice()`,
		`truthFloorBudgetReservedTokens: step13TokenTruthFloorTotalMinTokens`,
		`hardFloorLabels: Object.keys(hardFloorTargetChars)`,
		`tokens: Number(step13TokenTruthFloorMinTokensByLabel[label] || 0)`,
		`hardFloorApplied: hardFloorReserveTotalChars > 0`,
		`residualSupportBudgetChars: Math.max(0, budgetLimit - hardFloorReserveTotalChars)`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P159 TB-1e marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P164PS1aBeatPlannerLaneSchemaMarkers verifies PS-1a:
// Beat Planner lane schema, guidance-only, required fields, truth-write blocked.
func TestArchiveCenterJSSeq13P164PS1aBeatPlannerLaneSchemaMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13BeatPlannerSchemaPolicyVersion = "ps1a.v1"`,
		`narrativeQualityCoprocessorPlannerRole = "beat_planner"`,
		`step13BeatPlannerDraftRequiredFields = ["narrative_goal", "tension_axis", "open_question", "payoff_candidate"]`,
		`narrativeQualityCoprocessorPlannerAuthorityPromotionAllowed = false`,
		`narrativeQualityCoprocessorCrossRoleTruthBorrowAllowed = false`,
		`step13PlanningTruthWriteAllowed = false`,
		`step13PlanningReducerReentryAllowed = false`,
		`narrativeQualityCoprocessorPlannerFields`,
		`narrativeQualityCoprocessorOutputMode`,
		`delivery_surface: narrativeQualityCoprocessorOutputMode`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P164 PS-1a marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P166PS1bScenePilotLaneSchemaMarkers verifies PS-1b:
// Scene Pilot lane schema, guidance-only / experimental shadow, truth-write blocked.
func TestArchiveCenterJSSeq13P166PS1bScenePilotLaneSchemaMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13ScenePilotSchemaPolicyVersion = "ps1b.v1"`,
		`narrativeQualityCoprocessorExecutionRole = "scene_pilot"`,
		`step13ScenePilotDraftRequiredFields = ["execution_mode", "scene_target", "emphasis_axis", "pacing", "forbidden_move"]`,
		`narrativeQualityCoprocessorExecutionAuthorityPromotionAllowed = false`,
		`step13PlanningTruthWriteAllowed = false`,
		`step13PlanningReducerReentryAllowed = false`,
		`step13PlanningRuntimeMode = "guidance_only_schema_draft"`,
		`step13PlanningRolloutStage = "experimental_shadow"`,
		`delivery_surface: narrativeQualityCoprocessorOutputMode`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P166 PS-1b marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P168PS1cSceneScopedSettingFrameMarkers verifies PS-1c:
// Scene-scoped setting frame, world coprocessor slice delivery, truth alias blocked.
func TestArchiveCenterJSSeq13P168PS1cSceneScopedSettingFrameMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13SettingFrameSchemaPolicyVersion = "ps1c.v1"`,
		`worldCoprocessorSceneSliceSelectorSignal = "scene_scope"`,
		`worldCoprocessorSceneSliceExtractionMode = "current_scene_thin_frame_only"`,
		`worldCoprocessorSceneSliceOutputType = "scene_scoped_setting_hint"`,
		`worldCoprocessorSceneSliceDeliverySurface = "guidance_only_setting_frame"`,
		`worldCoprocessorSceneSliceTruthAliasBlocked = true`,
		`worldCoprocessorSceneSliceTruthBorrowAllowed = false`,
		`world_pressure_hint`,
		`guidance_only_setting_frame`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P168 PS-1c marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P170PS1dPlanningKeepDropConflictMarkers verifies PS-1d:
// Keep/drop/conflict policy, evaluation order, truth-floor fallback.
func TestArchiveCenterJSSeq13P170PS1dPlanningKeepDropConflictMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13PlanningKeepDropConflictPolicyVersion = "ps1d.v1"`,
		`step13PlanningConflictEvaluationOrder = ["beat_planner", "scene_pilot", "setting_frame"]`,
		`step13PlanningConflictBlockedTargets`,
		`step13PlanningBeatPlannerConflictAction`,
		`step13PlanningScenePilotConflictAction`,
		`step13PlanningSettingFrameConflictAction`,
		`step13PlanningConflictTruthFloorFallback = "preserve_step11_truth_floor"`,
		`narrativeQualityCoprocessorConflictGuardTruthFloorFallback = "preserve_step11_factual_state"`,
		`preserve_step11_truth_floor`,
		`drop_conflicting_planning_hint`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P170 PS-1d marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P172PS1ePlanningMonolithRegressionMarkers verifies PS-1e:
// Anti-monolith guard: split lanes, forbidden shapes, no reducer re-entry.
func TestArchiveCenterJSSeq13P172PS1ePlanningMonolithRegressionMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13PlanningMonolithGuardPolicyVersion = "ps1e.v1"`,
		`step13PlanningMonolithGuardMode = "split_lanes_not_unified_core"`,
		`step13PlanningMonolithGuardLaneNames`,
		`step13PlanningMonolithGuardForbiddenShapes`,
		`unified_planning_core`,
		`truth_path_frontload`,
		`authority_promotion_bridge`,
		`step13PlanningMonolithGuardRequiredDeliverySurfaces`,
		`step13PlanningMonolithGuardDefaultAction = "keep_split_guidance_only_surfaces"`,
		`step13PlanningTruthWriteAllowed = false`,
		`step13PlanningReducerReentryAllowed = false`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P172 PS-1e marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P174PS1fPlanningGovernorPortabilityNamingMarkers verifies PS-1f:
// Naming gate: local vocab review only, approved labels, blocked legacy, no runtime rename.
func TestArchiveCenterJSSeq13P174PS1fPlanningGovernorPortabilityNamingMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13NamingGatePolicyVersion = "ps1f.v1"`,
		`step13NamingGateMode = "local_vocab_review_only"`,
		`step13NamingGateReviewScope`,
		`step13NamingGateApprovedLabelsByPhase`,
		`step13NamingGateReviewedFunctionNames`,
		`step13NamingGateReviewedHelperNames`,
		`step13NamingGateBlockedLegacyValues`,
		`step13NamingGateApprovalRule = "allow_local_vocab_block_external_like_reuse"`,
		`step13NamingGateRuntimeAction = "review_only_no_runtime_rename"`,
		`review_only_no_runtime_rename`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P174 PS-1f marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P179VX1aPortabilityRoundtripReplayMarkers verifies VX-1a:
// Portability validation slice: roundtrip replay, lineage, selective rebuild, manual-first.
func TestArchiveCenterJSSeq13P179VX1aPortabilityRoundtripReplayMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13PortabilityValidationPolicyVersions = ["sp1a.v1", "sp1b.v1", "sp1c.v1", "sp1d.v1", "sp1e.v1"]`,
		`step13ValidationGateDefaultsBySlice.portability`,
		`default_action: "stay_manual_first_review_only_until_vx_green"`,
		`rollout_stage: "manual_review_only"`,
		`takeover_allowed_by_default: false`,
		`default_state: step13ValidationGateDefaultState`,
		`source_hash`,
		`source_turn`,
		`step13ValidationGateSliceOrder`,
		`"portability"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P179 VX-1a marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P181VX1bCopiedSessionDeletionReplayMarkers verifies VX-1b:
// Copied/imported session deletion replay, stale truth blocked, tombstone lineage.
func TestArchiveCenterJSSeq13P181VX1bCopiedSessionDeletionReplayMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`entityCoprocessorStaleSceneGuardSuppressionRoute = "drop_entity_hint_keep_truth_floor"`,
		`stale_scene_truth_promotion_blocked`,
		`disallowed_usage: ["canonical_relationship_overwrite", "direct_canonical_relationship_patch_apply", "stale_scene_truth_promotion"`,
		`historicalDeletionDetected: false`,
		`tombstoned`,
		`source_turn`,
		`source_hash`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P181 VX-1b marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P183VX1cReembedLifecycleReplayMarkers verifies VX-1c:
// Reembed validation slice: model-switch replay, manual admin batch, truth floor preserve.
func TestArchiveCenterJSSeq13P183VX1cReembedLifecycleReplayMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13ReembedValidationPolicyVersions = ["em1a.v1", "em1b.v1", "em1c.v1", "em1d.v1"]`,
		`step13ValidationGateDefaultsBySlice.reembed`,
		`default_action: "stay_manual_admin_batch_until_vx_green"`,
		`rollout_stage: "manual_review_only"`,
		`takeover_allowed_by_default: false`,
		`default_state: step13ValidationGateDefaultState`,
		`"reembed"`,
		`step13ValidationGateSliceOrder`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P183 VX-1c marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P185VX1dGovernorLoadReplayMarkers verifies VX-1d:
// Governor validation slice: cooldown, failure budget, suspension, fail-open.
func TestArchiveCenterJSSeq13P185VX1dGovernorLoadReplayMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13GovernorValidationPolicyVersions`,
		`step13GovernorCooldownTurnsByModule`,
		`step13GovernorApprovalStates`,
		`approvalStates: ["run", "reuse", "skip", "suspend"]`,
		`failure_budget_policy_version`,
		`step13GovernorFailureFailOpenBehavior = "keep_current_turn_execution_and_truth_floor"`,
		`step13ValidationGateDefaultsBySlice.governor`,
		`default_action: "stay_trace_only_no_prompt_mutation_until_vx_green"`,
		`rollout_stage: "trace_contract_only"`,
		`takeover_allowed_by_default: false`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P185 VX-1d marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P187VX1eTokenFloorReplayMarkers verifies VX-1e:
// Token budget validation slice: truth floor preserve, hard floor, residual budget, drift telemetry.
func TestArchiveCenterJSSeq13P187VX1eTokenFloorReplayMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13TokenValidationPolicyVersions`,
		`hardFloorReserveTotalChars`,
		`truth_floor_reserved_chars: hardFloorReserveTotalChars`,
		`residualSupportBudgetChars: Math.max(0, budgetLimit - hardFloorReserveTotalChars)`,
		`drift_telemetry_fields: step13TokenEstimatorDriftTelemetryFields.slice()`,
		`step13ValidationGateDefaultsBySlice.token_budget`,
		`default_action: "stay_contract_only_no_runtime_enforcement_until_vx_green"`,
		`rollout_stage: "contract_review_only"`,
		`takeover_allowed_by_default: false`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P187 VX-1e marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P189VX1fPlanningSurfaceLeakageReplayMarkers verifies VX-1f:
// Planning validation slice: authority leak guard, guidance-only, truth-write blocked, conflict fallback.
func TestArchiveCenterJSSeq13P189VX1fPlanningSurfaceLeakageReplayMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13PlanningValidationPolicyVersions`,
		`step13PlanningTruthWriteAllowed = false`,
		`step13PlanningReducerReentryAllowed = false`,
		`step13PlanningTakeoverGate = "stay_guidance_only_until_vx_green"`,
		`step13ValidationGateDefaultsBySlice.planning`,
		`default_action: step13PlanningTakeoverGate`,
		`rollout_stage: step13PlanningRolloutStage`,
		`takeover_allowed_by_default: false`,
		`preserve_step11_truth_floor`,
		`guidance_only_setting_frame`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P189 VX-1f marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P191VX1gDefaultTakeoverGateMarkers verifies VX-1g:
// Validation gate: default takeover blocked, required signals, slice order, review-only runtime action.
func TestArchiveCenterJSSeq13P191VX1gDefaultTakeoverGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13ValidationGatePolicyVersion = "vx1g.v1"`,
		`step13ValidationGateMode = "default_takeover_blocked_until_slice_green"`,
		`step13ValidationGateRequiredSignals = ["slice_replay_green", "slice_ablation_green_or_not_applicable", "release_gate_green"]`,
		`step13ValidationGateSliceOrder = ["portability", "reembed", "governor", "token_budget", "planning"]`,
		`step13ValidationGateDefaultState = "draft_locked"`,
		`step13ValidationGateRuntimeAction = "review_only_no_runtime_takeover"`,
		`step13ValidationGateDefaultsBySlice`,
		`takeover_allowed_by_default: false`,
		`default_takeover_gate_fixed`,
		`inherited_module_takeover_policy_version`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P191 VX-1g marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P214ReleaseGateBundleLatestRuntimeMarkers verifies P214:
// Beta 0.4 release-gate bundle regeneration remains a contract marker, not a generated artifact.
func TestArchiveCenterJSSeq13P214ReleaseGateBundleLatestRuntimeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`seq13P214BundleLatestRootRuntimeGate = "bundle_latest_root_runtime_draft"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P214 bundle latest root runtime marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P215ReleaseGateRootBundleRegenerateMarkers verifies P215:
// Root-to-bundle regeneration is kept as a repeatable checklist/script gate.
func TestArchiveCenterJSSeq13P215ReleaseGateRootBundleRegenerateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`seq13P215RootBundleRegenerateGate = "root_bundle_regenerate_draft"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P215 root bundle regenerate marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P216ReleaseGatePackagedSmokeMarkers verifies P216:
// Packaged smoke evidence remains marker-based without producing release artifacts.
func TestArchiveCenterJSSeq13P216ReleaseGatePackagedSmokeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`seq13P216PackagedBundleRoundtripGate = "packaged_bundle_import_export_roundtrip"`,
		`seq13P216ReembedFallbackSmokePass = "reembed_fallback_smoke_check_pass"`,
		`seq13P216GovernorTraceSmokePass = "governor_trace_smoke_check_pass"`,
		`seq13P216TokenProfileTraceSmokePass = "token_profile_trace_smoke_check_pass"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P216 packaged smoke marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P217ReleaseGatePlanningGuidanceOnlyMarkers verifies P217:
// Planning surfaces stay guidance-only/experimental with authority leak protection.
func TestArchiveCenterJSSeq13P217ReleaseGatePlanningGuidanceOnlyMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`seq13P217PlanningSurfaceGuidanceOnlyLeakGuard = "guidance_only_experimental_authority_leak_guard"`,
		`step13PlanningRuntimeMode = "guidance_only_schema_draft"`,
		`step13PlanningTakeoverGate = "stay_guidance_only_until_vx_green"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P217 planning guidance-only leak guard marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P218ReleaseGateNamingReviewMarkers verifies P218:
// Step 13 naming remains reviewed local vocabulary with no runtime rename.
func TestArchiveCenterJSSeq13P218ReleaseGateNamingReviewMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`seq13P218NamingReviewChecklistConfirmed = "step13_naming_review_checklist_confirmed"`,
		`step13NamingGatePolicyVersion = "ps1f.v1"`,
		`step13NamingGateMode = "local_vocab_review_only"`,
		`step13NamingGateRuntimeAction = "review_only_no_runtime_rename"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P218 naming review marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P224GovernorPluginOnlyBoundaryMarkers verifies P224:
// Governor plugin-only memory/backend save boundary with local runtime contract only mode.
func TestArchiveCenterJSSeq13P224GovernorPluginOnlyBoundaryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const coprocessorOrchestrationPluginOnlyModules = [`,
		`const coprocessorOrchestrationPluginOnlyExecutionMode = "local_runtime_contract_only";`,
		`plugin_only_modules: coprocessorOrchestrationPluginOnlyModules.slice(),`,
		`plugin_only_execution_mode: coprocessorOrchestrationPluginOnlyExecutionMode,`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P224 governor plugin-only boundary marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P225TokenEstimatorFollowUpMarkers verifies P225:
// Token estimator policy versions, fallback, truth floor, density profile, and shared threshold mode.
func TestArchiveCenterJSSeq13P225TokenEstimatorFollowUpMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const step13TokenEstimatorPolicyVersion = "tb1a.v1";`,
		`const step13TokenBudgetFallbackPolicyVersion = "tb1d.v1";`,
		`const step13TokenTruthFloorPolicyVersion = "tb1b.v1";`,
		`const step13TokenDensityProfilePolicyVersion = "tb1c.v1";`,
		`const step13TokenEstimatorMode = "profile_first_shared_thresholds";`,
		`const step13TokenEstimatorModelMode = "shared_thresholds_all_models";`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P225 token estimator follow-up marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P226PlanningSurfaceFollowUpMarkers verifies P226:
// Planning surface schema policy versions, runtime mode, rollout stage, and conflict actions.
func TestArchiveCenterJSSeq13P226PlanningSurfaceFollowUpMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13PlanningBeatPlannerSchemaPolicyVersion`,
		`step13PlanningScenePilotSchemaPolicyVersion`,
		`step13PlanningSettingFrameSchemaPolicyVersion`,
		`const step13PlanningRuntimeMode = "guidance_only_schema_draft";`,
		`const step13PlanningRolloutStage = "experimental_shadow";`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P226 planning surface follow-up marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P227BundleRegenerateManifestScriptOnlyMarkers verifies P227:
// Root bundle regenerate gate remains a script-only manifest checklist marker.
func TestArchiveCenterJSSeq13P227BundleRegenerateManifestScriptOnlyMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`seq13P215RootBundleRegenerateGate = "root_bundle_regenerate_draft"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P227 bundle regenerate manifest script-only marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P228ExternalReferenceNamingMapMarkers verifies P228:
// External reference naming map draft remains a local vocab review source.
func TestArchiveCenterJSSeq13P228ExternalReferenceNamingMapMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const step13NamingGateSourceDraft = "STEP13_NAMING_MAP_DRAFT.md";`,
		`step13NamingGateApprovalRule = "allow_local_vocab_block_external_like_reuse"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P228 external reference naming map marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq14P85BundleLatestRootRuntimeContractMarkers verifies P85:
// Step 14 bundle latest root runtime remains a release-gate contract marker
// without generating exe, zip, bundle, or DB snapshot artifacts.
func TestArchiveCenterJSSeq14P85BundleLatestRootRuntimeContractMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`seq14P85BundleLatestRootRuntimeGate = "step14_bundle_latest_root_runtime_contract_only"`,
		`seq14P85BundleArtifactGenerationAllowed = false`,
		`seq14P85BundleGeneratedArtifactTypes = []`,
		`seq14P85BundleGateArtifactPolicy = "no_exe_zip_bundle_or_db_snapshot_generated"`,
		`seq14P85BundleGateValidationMode = "contract_marker_smoke_only"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-14-P85 bundle latest root runtime contract marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq15P144BundleLatestRootRuntimeContractMarkers verifies P144:
// Step 15 bundle latest root runtime release-gate contract marker.
// 2.0 remigration prohibits actual artifact generation.
func TestArchiveCenterJSSeq15P144BundleLatestRootRuntimeContractMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`seq15P144BundleLatestRootRuntimeGate = "step15_bundle_latest_root_runtime_contract_only"`,
		`seq15P144BundleArtifactGenerationAllowed = false`,
		`seq15P144BundleGeneratedArtifactTypes = []`,
		`seq15P144BundleGateArtifactPolicy = "no_exe_zip_bundle_or_db_snapshot_generated"`,
		`seq15P144BundleGateValidationMode = "contract_marker_smoke_only"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-15-P144 bundle latest root runtime contract marker %q", needle)
		}
	}
}

// SEQ-16.5-P141: helper injection budget manager define — validates that
// Archive Center.js contains the helper injection budget manager surface markers.
func TestArchiveCenterJSSeq165P141HelperInjectionBudgetManagerMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function assembleInjectionWithBudget(",
		"const manualBudgetLimit = Math.max(500, settings.maxInjectionChars || DEFAULT_SETTINGS.maxInjectionChars);",
		"const step13TokenEstimatorPolicyVersion = \"tb1a.v1\";",
		"const step13TokenBudgetFallbackPolicyVersion = \"tb1d.v1\";",
		"const step13TokenEstimatorMode = \"profile_first_shared_thresholds\";",
		"const step13TokenEstimatorProfiles = [",
		"mid_context_300k",
		"wide_context_500k",
		"ultra_long_1m_plus",
		"extreme_long_2m_plus",
		"const step13TokenEstimatorBudgetLimitByProfile = adaptiveInjectionBudgetProfileLimits();",
		"const step13TokenBudgetFallbackRules = {",
		"reliable_runtime_tokens",
		"low_confidence_runtime_source",
		"runtime_tokens_unavailable",
		"manual_setting_only",
		"const step13TokenTruthFloorPolicyVersion = \"tb1b.v1\";",
		"const step13TokenTruthFloorMode = \"char_floor_projected_minimums\";",
		"const step13TokenDensityProfilePolicyVersion = \"tb1c.v1\";",
		"const runtimeTokenSmoothingWindow = 4;",
		"const runtimeTokenSmoothingState = assembleInjectionWithBudget._runtimeTokenSmoothingState || new Map();",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.5-P141 helper injection budget manager marker %q", needle)
		}
	}
}
