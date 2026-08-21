package main

import (
	"strings"
	"testing"
)

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
// TestArchiveCenterJSSeq13P150TB1bTruthFloorMinimumTokenMarkers verifies TB-1b:
// truth-floor minimum token contract markers are present in Archive Center.js.
// TestArchiveCenterJSSeq13P153TB1cDensityProfileMarkers verifies TB-1c:
// token density profile markers for ledger/world/guidance are present in Archive Center.js.
// TestArchiveCenterJSSeq13P156TB1dBudgetFallbackTokenEstimateMarkers verifies TB-1d:
// budget fallback/token estimate mode markers are present in Archive Center.js.
// TestArchiveCenterJSSeq13P159TB1eTokenReplayTruthFloorMarkers verifies TB-1e:
// token replay window and truth-floor verification markers are present in Archive Center.js.
// TestArchiveCenterJSSeq13P164PS1aBeatPlannerLaneSchemaMarkers verifies PS-1a:
// Beat Planner lane schema, guidance-only, required fields, truth-write blocked.
// TestArchiveCenterJSSeq13P166PS1bScenePilotLaneSchemaMarkers verifies PS-1b:
// Scene Pilot lane schema, guidance-only / experimental shadow, truth-write blocked.
// TestArchiveCenterJSSeq13P168PS1cSceneScopedSettingFrameMarkers verifies PS-1c:
// Scene-scoped setting frame, world coprocessor slice delivery, truth alias blocked.
// TestArchiveCenterJSSeq13P170PS1dPlanningKeepDropConflictMarkers verifies PS-1d:
// Keep/drop/conflict policy, evaluation order, truth-floor fallback.
// TestArchiveCenterJSSeq13P172PS1ePlanningMonolithRegressionMarkers verifies PS-1e:
// Anti-monolith guard: split lanes, forbidden shapes, no reducer re-entry.
// TestArchiveCenterJSSeq13P174PS1fPlanningGovernorPortabilityNamingMarkers verifies PS-1f:
// Naming gate: local vocab review only, approved labels, blocked legacy, no runtime rename.
// TestArchiveCenterJSSeq13P179VX1aPortabilityRoundtripReplayMarkers verifies VX-1a:
// Portability validation slice: roundtrip replay, lineage, selective rebuild, manual-first.
// TestArchiveCenterJSSeq13P181VX1bCopiedSessionDeletionReplayMarkers verifies VX-1b:
// Copied/imported session deletion replay, stale truth blocked, tombstone lineage.
// TestArchiveCenterJSSeq13P183VX1cReembedLifecycleReplayMarkers verifies VX-1c:
// Reembed validation slice: model-switch replay, manual admin batch, truth floor preserve.
// TestArchiveCenterJSSeq13P185VX1dGovernorLoadReplayMarkers verifies VX-1d:
// Governor validation slice: cooldown, failure budget, suspension, fail-open.
// TestArchiveCenterJSSeq13P187VX1eTokenFloorReplayMarkers verifies VX-1e:
// Token budget validation slice: truth floor preserve, hard floor, residual budget, drift telemetry.
// TestArchiveCenterJSSeq13P189VX1fPlanningSurfaceLeakageReplayMarkers verifies VX-1f:
// Planning validation slice: authority leak guard, guidance-only, truth-write blocked, conflict fallback.
// TestArchiveCenterJSSeq13P191VX1gDefaultTakeoverGateMarkers verifies VX-1g:
// Validation gate: default takeover blocked, required signals, slice order, review-only runtime action.
// TestArchiveCenterJSSeq13P214ReleaseGateBundleLatestRuntimeMarkers verifies P214:
// Beta 0.4 release-gate bundle regeneration remains a contract marker, not a generated artifact.
// TestArchiveCenterJSSeq13P215ReleaseGateRootBundleRegenerateMarkers verifies P215:
// Root-to-bundle regeneration is kept as a repeatable checklist/script gate.
// TestArchiveCenterJSSeq13P216ReleaseGatePackagedSmokeMarkers verifies P216:
// Packaged smoke evidence remains marker-based without producing release artifacts.
// TestArchiveCenterJSSeq13P217ReleaseGatePlanningGuidanceOnlyMarkers verifies P217:
// Planning surfaces stay guidance-only/experimental with authority leak protection.
// TestArchiveCenterJSSeq13P218ReleaseGateNamingReviewMarkers verifies P218:
// Step 13 naming remains reviewed local vocabulary with no runtime rename.
// TestArchiveCenterJSSeq13P224GovernorPluginOnlyBoundaryMarkers verifies P224:
// Governor plugin-only memory/backend save boundary with local runtime contract only mode.
// TestArchiveCenterJSSeq13P225TokenEstimatorFollowUpMarkers verifies P225:
// Token estimator policy versions, fallback, truth floor, density profile, and shared threshold mode.
// TestArchiveCenterJSSeq13P226PlanningSurfaceFollowUpMarkers verifies P226:
// Planning surface schema policy versions, runtime mode, rollout stage, and conflict actions.
// TestArchiveCenterJSSeq13P227BundleRegenerateManifestScriptOnlyMarkers verifies P227:
// Root bundle regenerate gate remains a script-only manifest checklist marker.
// TestArchiveCenterJSSeq13P228ExternalReferenceNamingMapMarkers verifies P228:
// External reference naming map draft remains a local vocab review source.
// TestArchiveCenterJSSeq14P85BundleLatestRootRuntimeContractMarkers verifies P85:
// Step 14 bundle latest root runtime remains a release-gate contract marker
// without generating exe, zip, bundle, or DB snapshot artifacts.
// TestArchiveCenterJSSeq15P144BundleLatestRootRuntimeContractMarkers verifies P144:
// Step 15 bundle latest root runtime release-gate contract marker.
// 2.0 remigration prohibits actual artifact generation.
// SEQ-16.5-P141: helper injection budget manager define — validates that
// Archive Center.js contains the helper injection budget manager surface markers.
