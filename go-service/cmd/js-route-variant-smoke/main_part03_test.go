package main

import (
	"strings"
	"testing"
)

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
