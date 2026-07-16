package main

import (
	"strings"
	"testing"
)

// SEQ-17-P329: 17-4c Step 16 adoption gate define.
func TestArchiveCenterJSSeq17P329Step16AdoptionGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"adoption",
		"gate",
		"regression",
		"corpus",
		"definition=",
		"execution=",
		"limited_cutover",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P329 marker %q", needle)
		}
	}
}

// SEQ-17-P330: 17-4d root -> bundle regenerate checklist define.
func TestArchiveCenterJSSeq17P330BundleRegenerateChecklistMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"bundle",
		"regenerate",
		"checklist",
		"Bundle Closure",
		"release_gate_closed",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P330 marker %q", needle)
		}
	}
}

// SEQ-17-P331: 17-4e packaged bundle regression / smoke / release note checklist define.
func TestArchiveCenterJSSeq17P331PackagedBundleChecklistMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"bundle",
		"regression",
		"smoke",
		"release",
		"Release Hygiene",
		"release_ready",
		"known",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P331 marker %q", needle)
		}
	}
}

// SEQ-17-P332: 17-4f freshness / silent-drop gate define.
func TestArchiveCenterJSSeq17P332FreshnessSilentDropGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"freshness",
		"silent",
		"drop",
		"gate",
		"runtime defaults",
		"Visibility Guard",
		"visible_failures",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P332 marker %q", needle)
		}
	}
}

// SEQ-16.8-P170: Step 17 evaluation harness consumes Step 16.8 replay corpus baseline.
func TestArchiveCenterJSSeq168P170CarryInEvaluationHarnessMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"evaluationHarnessConsumesStep168ReplayCorpus",
		"redefines17_1f",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P170 marker %q", needle)
		}
	}
}

// SEQ-16.8-P171: Step 17 inspection surface uses Step 16.8 reason visibility lane baseline.
func TestArchiveCenterJSSeq168P171CarryInInspectionSurfaceMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"inspectionSurfaceUsesStep168ReasonVisibilityLane",
		"redefines17_3f",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P171 marker %q", needle)
		}
	}
}

// SEQ-16.8-P172: Step 17 adoption gate uses Step 16.8 diversity gate baseline.
func TestArchiveCenterJSSeq168P172CarryInAdoptionGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"adoptionGateUsesStep168DiversityGate",
		"redefines17_4g",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P172 marker %q", needle)
		}
	}
}

// SEQ-16.8-P176: Step 18 hybrid scoring stale callback ceiling / current-scene alignment baseline.
func TestArchiveCenterJSSeq168P176CarryInStep18HybridScoringMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"stale",
		"arc",
		"ceiling",
		"alignment",
		"current_scene_evidence",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P176 marker %q", needle)
		}
	}
}

// SEQ-16.8-P177: Step 20 selective rerank stale callback suppression trigger / monopoly failure taxonomy baseline.
func TestArchiveCenterJSSeq168P177CarryInStep20SelectiveRerankMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"selectiveRerankConsumesStep168Suppression",
		"selectiveRerankConsumesStep168MonopolyTaxonomy",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P177 marker %q", needle)
		}
	}
}

// SEQ-16.8-P178: later-step recall / rerank gain foreground monopoly cost baseline trace.
func TestArchiveCenterJSSeq168P178CarryInLaterStepRecallRerankMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"laterRecallRerankSharesMonopolyCostTrace",
		"recallGainCount",
		"monopolyCostCount",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P178 marker %q", needle)
		}
	}
}

// SEQ-17-P387: Archive Center Beta 0.8 bundle latest root runtime create/generate.
func TestArchiveCenterJSSeq17P387BundleGenerationEvidenceMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"_step17ReleaseGate",
		"step17ReleaseGateFetch",
		"/metrics/lc1s/step17-bundle-closure",
		"step17_bundle_closure",
		"Bundle Closure",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P387 marker %q", needle)
		}
	}
}

// SEQ-17-P388: Step 14~16 regression corpus green.
func TestArchiveCenterJSSeq17P388RegressionCorpusGreenMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"/metrics/lc1r/regression-corpus",
		"regression_corpus_manifest",
		"Regression Corpus",
		"release_gate_ready",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P388 marker %q", needle)
		}
	}
}

// SEQ-17-P389: evaluation split completeness/answer-quality smoke check pass.
func TestArchiveCenterJSSeq17P389EvaluationSplitSmokeCheckMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"closure_status",
		"release_gate_closed",
		"summarizeChecklist",
		"Release Hygiene",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P389 marker %q", needle)
		}
	}
}

// SEQ-17-P390: ops procedure dry-run checklist pass.
func TestArchiveCenterJSSeq17P390OpsDryRunChecklistPassMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"operator_checklist",
		"Missing Release Evidence",
		"release_hygiene",
		"Release Reasons",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P390 marker %q", needle)
		}
	}
}

// SEQ-17-P391: inspection surface lane-boundary review checklist pass.
func TestArchiveCenterJSSeq17P391InspectionLaneBoundaryReviewMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"renderStep17ReleaseGateSection",
		"This panel is read-only",
		"Bundle closure and session-local adoption are separate truths",
		"live limited-cutover gates remain hold/pending",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P391 marker %q", needle)
		}
	}
}

// SEQ-17-P392: adoption gate / release note / bundle checklist complete.
func TestArchiveCenterJSSeq17P392ReleaseGateCompleteMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"/chroma-shadow/adoption-gate",
		"/chroma-shadow/release-hygiene",
		"adoption-gate",
		"release-hygiene",
		"Adoption Gate",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P392 marker %q", needle)
		}
	}
}

// SEQ-17-P396: backend/admin release-gate owner closure.
func TestArchiveCenterJSSeq17P396ReauditBackendAdminOwnerMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"step17ReleaseGateFetch",
		"/metrics/lc1s/step17-bundle-closure",
		"/metrics/lc1r/regression-corpus",
		"/chroma-shadow/adoption-gate",
		"/chroma-shadow/release-hygiene",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P396 marker %q", needle)
		}
	}
}

// SEQ-17-P397: ops documentation dry-run checklist closure.
func TestArchiveCenterJSSeq17P397ReauditOpsDocDryRunMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"release_hygiene",
		"operator_checklist",
		"Missing Release Evidence",
		"Release Hygiene",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P397 marker %q", needle)
		}
	}
}

// SEQ-17-P398: root runtime read-only inspection/gate surface closure.
func TestArchiveCenterJSSeq17P398ReauditRootRuntimeReadOnlyMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"renderStep17InspectionRolesSection",
		"Step 17 inspection remains read-only",
		"This panel does not open adoption, change routing, or bypass direct evidence",
		"renderStep17ReleaseGateSection",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P398 marker %q", needle)
		}
	}
}

// SEQ-17-P399: release gate operator evidence closure.
func TestArchiveCenterJSSeq17P399ReauditReleaseGateOperatorEvidenceMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"operator_evidence",
		"release_evidence",
		"missing_operator_evidence",
		"missing_release_evidence",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P399 marker %q", needle)
		}
	}
}

// SEQ-17-P400: admin mutation/control UI boundary (dangerous surface).
func TestArchiveCenterJSSeq17P400ReauditAdminMutationControlUIMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"This panel is read-only",
		"does not open adoption",
		"change routing",
		"bypass direct evidence",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P400 marker %q", needle)
		}
	}
	forbidden := []string{
		"mo-step17-admin-execute",
		"step17AdminMutationExecute",
		"step17AdminControlExecute",
	}
	for _, needle := range forbidden {
		if strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js unexpectedly exposes SEQ-17-P400 execution marker %q", needle)
		}
	}
}

// SEQ-17-P401: release execution UI boundary (dangerous surface).
func TestArchiveCenterJSSeq17P401ReauditReleaseExecutionUIMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"This panel is read-only",
		"release gate can stay closed",
		"operator evidence is supplied",
		"live limited-cutover gates remain hold/pending",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P401 marker %q", needle)
		}
	}
	forbidden := []string{
		"mo-step17-release-execute",
		"step17ReleaseExecutionRun",
		"step17BundleRegenerateExecute",
	}
	for _, needle := range forbidden {
		if strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js unexpectedly exposes SEQ-17-P401 execution marker %q", needle)
		}
	}
}

// SEQ-17-P402: Beta 0.8 closure bundle boundary.
func TestArchiveCenterJSSeq17P402ReauditBeta08ClosureBundleMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"step17_bundle_closure",
		"closure_status",
		"closure_scope",
		"release_gate_closed",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P402 marker %q", needle)
		}
	}
}

// SEQ-17-P412: completeness metric default unit decision.
func TestArchiveCenterJSSeq17P412DecisionCompletenessMetricUnitMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"Chroma Live Retrieval",
		"summarizeChromaLiveInspection",
		"candidateCount",
		"supportingOnlyCount",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P412 marker %q", needle)
		}
	}
}

// SEQ-17-P413: regression corpus mix decision.
func TestArchiveCenterJSSeq17P413DecisionRegressionCorpusMixMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"/metrics/lc1r/regression-corpus",
		"regression_corpus_manifest",
		"release_gate_ready",
		"Regression Corpus",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P413 marker %q", needle)
		}
	}
}

// SEQ-17-P414: inspection lane default decision.
func TestArchiveCenterJSSeq17P414DecisionInspectionLaneDefaultMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"renderStep17InspectionRolesSection",
		"renderStep17VisibilitySection",
		"Freshness Summary",
		"Visibility Guard",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P414 marker %q", needle)
		}
	}
}

// SEQ-17-P415: adoption gate review mode decision.
func TestArchiveCenterJSSeq17P415DecisionAdoptionGateReviewModeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"/chroma-shadow/adoption-gate",
		"limited_cutover_approved",
		"missing_operator_evidence",
		"Adoption Reasons",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P415 marker %q", needle)
		}
	}
}

// SEQ-17-P416: bundle regenerate split decision.
func TestArchiveCenterJSSeq17P416DecisionBundleRegenerateSplitMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"/chroma-shadow/release-hygiene",
		"release_hygiene_status",
		"release_ready",
		"Release Reasons",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P416 marker %q", needle)
		}
	}
}

// SEQ-17-P420: 17-C1 migration preflight dry-run.
func TestArchiveCenterJSSeq17P420ChromaMigrationPreflightMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"summarizeChromaLiveInspection",
		"chroma_live_state",
		"chroma_live_mode",
		"chroma_live_reason",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P420 marker %q", needle)
		}
	}
}

// SEQ-17-P421: 17-C2 shadow collection bootstrap dry-run.
func TestArchiveCenterJSSeq17P421ChromaShadowBootstrapMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"shadow_disabled",
		"Chroma Live Retrieval",
		"chromaLive",
		"buildChromaLiveTraceDetail",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P421 marker %q", needle)
		}
	}
}

// SEQ-17-P422: 17-C3 backfill dry-run.
func TestArchiveCenterJSSeq17P422ChromaBackfillDryRunMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"backfill_import",
		"schema_migration",
		"sidecar_cache",
		"next_prepare_turn_fetch",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P422 marker %q", needle)
		}
	}
}

// SEQ-17-P423: 17-C4 bulk backfill dry-run.
func TestArchiveCenterJSSeq17P423ChromaBulkBackfillMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"backfill_import",
		"checkpoint_full_rebuild",
		"selective",
		"full",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P423 marker %q", needle)
		}
	}
}

// SEQ-17-P424: 17-C5 reembed discipline dry-run.
func TestArchiveCenterJSSeq17P424ChromaReembedDisciplineMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"seq13P216ReembedFallbackSmokePass",
		"reembed_fallback_smoke_check_pass",
		"stale",
		"invalidation",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P424 marker %q", needle)
		}
	}
}

// SEQ-17-P425: 17-C6 divergence / health probe dry-run.
func TestArchiveCenterJSSeq17P425ChromaDivergenceHealthProbeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"resolveChromaLiveTraceStatus",
		"buildChromaLiveTraceDetail",
		"statePriority",
		"mariadb_fallback",
		"sqlite_fallback",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P425 marker %q", needle)
		}
	}
}

// SEQ-17-P426: 17-C7 degraded fallback runbook dry-run.
func TestArchiveCenterJSSeq17P426ChromaDegradedFallbackRunbookMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"mariadb_fallback",
		"sqlite_fallback",
		"degraded",
		"blocked",
		"fallbackCount",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P426 marker %q", needle)
		}
	}
}

// SEQ-17-P427: 17-C8 rebuild / rollback drill dry-run.
func TestArchiveCenterJSSeq17P427ChromaRebuildRollbackDrillMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"schema_migration",
		"checkpoint_full_rebuild",
		"backfill_import",
		"full",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P427 marker %q", needle)
		}
	}
}

// SEQ-17-P428: 17-C9 adoption gate dry-run.
func TestArchiveCenterJSSeq17P428ChromaAdoptionGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"/chroma-shadow/adoption-gate",
		"limited_cutover_approved",
		"operator_checklist",
		"hold_reasons",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P428 marker %q", needle)
		}
	}
}

// SEQ-17-P429: 17-C10 release hygiene dry-run.
func TestArchiveCenterJSSeq17P429ChromaReleaseHygieneMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"/chroma-shadow/release-hygiene",
		"release_hygiene_status",
		"missing_release_evidence",
		"pending_reasons",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P429 marker %q", needle)
		}
	}
}

// SEQ-17-P430: 17-C11 migration visibility guard dry-run.
func TestArchiveCenterJSSeq17P430ChromaMigrationVisibilityGuardMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"/chroma-shadow/visibility-guard",
		"freshness_lag_summary",
		"Visibility Guard",
		"visible_failure_count",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P430 marker %q", needle)
		}
	}
}

// SEQ-18-P13: reset administration marker.
func TestArchiveCenterJSSeq18P13ResetAdminMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"reset_administration",
		"checklist_cleared_for_redo",
		"historical_content_preserved",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P13 marker %q", needle)
		}
	}
}

// SEQ-18-P19: Step 17 closure gate marker.
func TestArchiveCenterJSSeq18P19Step17ClosureGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"step_17_closure_gate",
		"closed",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P19 marker %q", needle)
		}
	}
}

// SEQ-18-P21: prep anchor VR+HY marker.
func TestArchiveCenterJSSeq18P21PrepAnchorVRHYMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"prep_anchor_vr_hy",
		"18-1_vr",
		"18-2_hy",
		"downstream_slices",
		"18-3_qr",
		"18-4_vx",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P21 marker %q", needle)
		}
	}
}

// SEQ-18-P23: backend prep anchor marker.
func TestArchiveCenterJSSeq18P23BackendPrepAnchorMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"backend_prep_anchor_file",
		"backend/archive/bridge.py",
		"search_memories",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P23 marker %q", needle)
		}
	}
}

// SEQ-18-P24: routing contract prep anchor marker.
func TestArchiveCenterJSSeq18P24RoutingContractPrepAnchorMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"routing_contract_prep_anchor_file",
		"backend/main.py",
		"_build_recall_intent_contract_q3a",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P24 marker %q", needle)
		}
	}
}

// SEQ-18-P29: VR scoped verbatim support marker.
func TestArchiveCenterJSSeq18P29VRScopedVerbatimSupportMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"vr_scoped_verbatim_support",
		"Scoped Verbatim Recall (support surface)",
		"direct_evidence_gate_approved",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P29 marker %q", needle)
		}
	}
}

// SEQ-18-P30: VR policy owner block marker.
func TestArchiveCenterJSSeq18P30VRPolicyOwnerBlockMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"vr_policy_owner_block",
		"max_items=3",
		"max_total_chars=720",
		"max_excerpt_chars=160",
		"support_surface_first=true",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P30 marker %q", needle)
		}
	}
}

// SEQ-18-P31: VR prompt injection strategy marker.
func TestArchiveCenterJSSeq18P31VRPromptInjectionStrategyMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"vr_prompt_injection_strategy",
		"latest_anchor_only",
		"vr_multi_item_lane_exposed",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P31 marker %q", needle)
		}
	}
}

// SEQ-18-P32: VR hierarchy escape hatch marker.
func TestArchiveCenterJSSeq18P32VRHierarchyEscapeHatchMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"vr_hierarchy_escape_hatch",
		"visible_when_summary_thin",
		"verbatim_support_surface_priority",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P32 marker %q", needle)
		}
	}
}

// SEQ-18-P33: VR backend test guard marker.
func TestArchiveCenterJSSeq18P33VRBackendTestGuardMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"vr_backend_test_file",
		"backend/test_step18_scoped_verbatim_support.py",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P33 marker %q", needle)
		}
	}
}

// SEQ-18-P34: VR runtime transparency marker.
func TestArchiveCenterJSSeq18P34VRRuntimeTransparencyMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"vr_runtime_transparency_test_file",
		"test_step18_scoped_verbatim_input_transparency.js",
		"Scoped Verbatim Recall (support surface)",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P34 marker %q", needle)
		}
	}
}

// SEQ-18-P35: VR regression bundle green marker.
func TestArchiveCenterJSSeq18P35VRRegressionBundleGreenMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"vr_regression_bundle_green",
		"adjacent_step19_regression_green",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P35 marker %q", needle)
		}
	}
}

// SEQ-18-P46: HY semantic rank + keyword overlap marker.
func TestArchiveCenterJSSeq18P46HYSemanticRankKeywordOverlapMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"hy_semantic_rank_preserved",
		"hy_keyword_overlap_policy",
		"hy1a.v1",
		"hy_hybrid_baseline_policy_version",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P46 marker %q", needle)
		}
	}
}

// SEQ-18-P47: HY soft bias marker.
func TestArchiveCenterJSSeq18P47HYSoftBiasMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"hy_soft_bias_policy",
		"hy1b.v1",
		"hy_speaker_bias_weight",
		"hy_location_bias_weight",
		"hy_storyline_bias_weight",
		"hy_soft_bias_cap",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P47 marker %q", needle)
		}
	}
}

// SEQ-18-P48: HY stopword guard marker.
func TestArchiveCenterJSSeq18P48HYStopwordGuardMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"hy_stopword_inflation_fixed",
		"hy_tightened_extractor",
		"hy_common_filler_excluded",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P48 marker %q", needle)
		}
	}
}

// SEQ-18-P49: HY q1a propagation marker.
func TestArchiveCenterJSSeq18P49HYQ1aPropagationMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"hy_q1a_propagation",
		"hy_q1a_propagated_fields",
		"keyword_overlap_score",
		"soft_bias_policy_version",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P49 marker %q", needle)
		}
	}
}

// SEQ-18-P50: HY runtime inspection marker.
func TestArchiveCenterJSSeq18P50HYRuntimeInspectionMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"hy_js_function",
		"extractMemoryItems",
		"hy_row_meta_extended",
		"hy_transparency_block",
		"Hybrid Retrieval Inspection",
		"trace_only",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P50 marker %q", needle)
		}
	}
}

// SEQ-18-P51: HY recurring risk guards marker.
func TestArchiveCenterJSSeq18P51HYRecurringRiskGuardsMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"hy_backend_regression_test_file",
		"backend/test_step18_hybrid_regression.py",
		"hy_js_transparency_test_file",
		"test_step18_hybrid_input_transparency.js",
		"hy_recurring_risk_guard_stopword",
		"guarded",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P51 marker %q", needle)
		}
	}
}

// SEQ-18-P52: HY policy registry marker.
func TestArchiveCenterJSSeq18P52HYPolicyRegistryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"hy_policy_registry_file",
		"backend/archive/hybrid_policy.py",
		"hy_policy_registry_consolidated",
		"hy_policy_family",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P52 marker %q", needle)
		}
	}
}

// SEQ-18-P53: HY stop at 18-2c marker.
func TestArchiveCenterJSSeq18P53HYStopAt18_2cMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"hy_stop_at_18_2c",
		"hy_open_follow_up",
		"step18_hy_18-2d",
		"hy_tail_budget_rescue",
		"enabled",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P53 marker %q", needle)
		}
	}
}

// SEQ-18-P65~P69: HY tail-budget rescue markers.
func TestArchiveCenterJSSeq18P65ToP69HYTailBudgetMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"hy_tail_budget_policy_version",
		"hy1d.v1",
		"hy_tail_budget_policy_owner",
		"hy_tail_budget_rescue_pass",
		"bounded_post_rank_same_n_results_budget",
		"keyword_soft_bias_stronger_than_cutline",
		"tail_budget_original_rank",
		"tail_budget_score_gap",
		"hy_tail_budget_q1a_propagation",
		"near_cutoff_rescue",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P65~P69 marker %q", needle)
		}
	}
}

// SEQ-18-P76~P91: QR query-class contract and budget markers.
func TestArchiveCenterJSSeq18P76ToP91QRQueryClassBudgetMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"qr_query_class_contract_version",
		"qr1a.v1",
		"single_query_shared",
		"qr_query_classes",
		"explicit_temporal_cue",
		"resume_trigger_or_ready_resume_pack",
		"qr_cue_block_owner",
		"backend/test_step18_query_class_contract.py",
		"qr_budget_policy_version",
		"qr1b.v1",
		"qr_q3c_budget_reuse",
		"qr_temporal_profile_budget",
		"retrieval_depth",
		"candidate_budget",
		"backend/test_step18_query_class_budget_policy.py",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P76~P91 marker %q", needle)
		}
	}
}

// SEQ-18-P98~P113: QR note and route policy markers.
func TestArchiveCenterJSSeq18P98ToP113QRNoteRouteMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"qr_note_policy_version",
		"qr1c.v1",
		"qr_support_surface_first",
		"qr_note_only_until_route_exec",
		"extract_before_read",
		"retrieval_note_surface",
		"backend/test_step18_query_class_note_policy.py",
		"qr_route_policy_version",
		"qr1d.v1",
		"scene_default",
		"needle_in_haystack",
		"old_detail_bridge",
		"qr_long_tail_route_candidates",
		"selected_route",
		"backend/test_step18_query_class_route_policy.py",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P98~P113 marker %q", needle)
		}
	}
}

// SEQ-18-P120~P153: VX validation gate markers.
func TestArchiveCenterJSSeq18P120ToP153VXValidationGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"vx18a_hybrid_replay_gate",
		"validation_gates.hybrid_replay",
		"_U1E_CAPTURED_REPLAY",
		"backend/test_step18_hybrid_replay_gate.py",
		"vx18b_heldout_completeness_gate",
		"retention_rate",
		"false_negative_rate",
		"full_coverage_rate",
		"backend/test_step18_heldout_completeness_gate.py",
		"vx18c_latency_token_budget_gate",
		"baseline_latency_proxy_ms",
		"candidate_token_budget_chars",
		"_LC1M_MAX_SPLIT_LATENCY_MULTIPLIER",
		"backend/test_step18_latency_token_budget_gate.py",
		"vx18d_truth_boundary_replay_gate",
		"_LC1K_HIGH_AUTHORITY_SOURCES",
		"_LC1K_LOWER_TIER_SOURCES",
		"backend/test_step18_truth_boundary_replay_gate.py",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P120~P153 marker %q", needle)
		}
	}
}

// SEQ-18-P160~P164: VX truncation / summary-loss gate markers.
func TestArchiveCenterJSSeq18P160ToP164VXTruncationSummaryLossMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"vx18e_truncation_summary_loss_gate",
		"validation_gates.truncation_summary_loss",
		"vx18e.v1",
		"baseline_tail_fact_miss_rate",
		"candidate_tail_fact_miss_rate",
		"baseline_summary_loss_rate",
		"candidate_summary_loss_rate",
		"tail_budget_promoted",
		"_U1E_CAPTURED_REPLAY_MIN",
		"backend/test_step18_truncation_summary_loss_gate.py",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P160~P164 marker %q", needle)
		}
	}
}

// SEQ-18-P327~P369: Post-Chroma and Step 18 summary row markers.
func TestArchiveCenterJSSeq18P327ToP369SummaryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"post_chroma_top6",
		"scoped_verbatim_recall_lane",
		"hybrid_retrieval_scoring_baseline",
		"temporal_relation_story_clock_foundation",
		"temporal_validity_retrieval",
		"lightweight_entity_graph_retrieval_accelerator",
		"selective_rerank_budget_aware_routing",
		"step18_summary_priority_rows",
		"step18_vr_raw_preserving_support",
		"step18_vr_truth_boundary_preserve",
		"step18_vr_summary_rows",
		"step18_vr_18-1d",
		"step18_hy_summary_rows",
		"step18_hy_18-2d",
		"step18_qr_summary_rows",
		"step18_qr_18-3d",
		"step18_vx_summary_rows",
		"step18_vx_18-4e",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P327~P369 marker %q", needle)
		}
	}
}

// SEQ-18-P373~P392: Pre-release 1.0.0 evidence markers.
func TestArchiveCenterJSSeq18P373ToP392PreReleaseMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"pre_release_1_0_0_marker",
		"1.0.0-pre",
		"pre_release_bundle_authority",
		"Archive Center 3.0.0 Release",
		"pre_release_smoke_checks",
		"scoped_verbatim_recall",
		"hybrid_baseline",
		"query_class_routing_budget",
		"vx_review_checklist",
		"pre_release_raw_support_limits",
		"max_items=3 max_total_chars=720 max_excerpt_chars=160",
		"pre_release_hybrid_soft_bias",
		"speaker=0.04 location=0.05 storyline=0.06 cap=0.12",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P373~P392 marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq19P114ValidatorHelperClusterMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function extractTemporalRelationEntriesStep19",
		"function buildTemporalStateSurfaceStep19",
		"function validateResponseTemporalDeicticStep19",
		"function readSceneTemporalStateFromOrchResultStep19",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P114 validator helper cluster marker %q", needle)
		}
	}
}
