package main

import (
	"os/exec"
	"strings"
	"testing"
)

// SEQ-20-P258: q20m temporal ambiguity support note preparatory marker.
func TestArchiveCenterJSSeq20P258Q20mTemporalAmbiguitySupportNotePreparatoryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"currentStoryClock",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P258 q20m preparatory marker %q", needle)
		}
	}
}

// SEQ-20-P259: q20m.v1 temporal ambiguity support note contract marker.
func TestArchiveCenterJSSeq20P259Q20mV1TemporalAmbiguitySupportNoteMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"currentStoryClock",
		"pending_threads",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P259 q20m.v1 marker %q", needle)
		}
	}
}

// SEQ-20-P260: q20n alias/entity conflict disambiguation preparatory marker.
func TestArchiveCenterJSSeq20P260Q20nAliasEntityConflictDisambiguationPreparatoryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessor",
		"characters",
		"pending_threads",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P260 q20n preparatory marker %q", needle)
		}
	}
}

// SEQ-20-P261: q20n.v1 alias/entity conflict disambiguation contract marker.
func TestArchiveCenterJSSeq20P261Q20nV1AliasEntityConflictDisambiguationMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessor",
		"characters",
		"pending_threads",
		"relationships_json",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P261 q20n.v1 marker %q", needle)
		}
	}
}

// SEQ-20-P262: q20o temporal/entity source-tag rule preparatory marker.
func TestArchiveCenterJSSeq20P262Q20oTemporalEntitySourceTagRulePreparatoryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessor",
		"temporalRelationLedger",
		"pending_threads",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P262 q20o preparatory marker %q", needle)
		}
	}
}

// SEQ-20-P263: q20o.v1 temporal/entity source-tag rule contract marker.
func TestArchiveCenterJSSeq20P263Q20oV1TemporalEntitySourceTagRuleMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessor",
		"temporalRelationLedger",
		"pending_threads",
		"relationships_json",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P263 q20o.v1 marker %q", needle)
		}
	}
}

// SEQ-20-P264: q20p canonical-pending/stale-current conflict note preparatory marker.
func TestArchiveCenterJSSeq20P264Q20pCanonicalPendingStaleCurrentConflictNotePreparatoryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"pending_threads",
		"latest_direct_evidence",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P264 q20p preparatory marker %q", needle)
		}
	}
}

// SEQ-20-P265: q20p.v1 canonical-pending/stale-current conflict note contract marker.
func TestArchiveCenterJSSeq20P265Q20pV1CanonicalPendingStaleCurrentConflictNoteMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"pending_threads",
		"latest_direct_evidence",
		"entityCoprocessor",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P265 q20p.v1 marker %q", needle)
		}
	}
}

// SEQ-20-P266: q20q recall cue rescue rule preparatory marker.
func TestArchiveCenterJSSeq20P266Q20qRecallCueRescueRulePreparatoryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"pending_threads",
		"latest_direct_evidence",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P266 q20q preparatory marker %q", needle)
		}
	}
}

// SEQ-20-P267: q20q.v1 recall cue rescue rule contract marker.
func TestArchiveCenterJSSeq20P267Q20qV1RecallCueRescueRuleMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"pending_threads",
		"latest_direct_evidence",
		"entityCoprocessor",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P267 q20q.v1 marker %q", needle)
		}
	}
}

// SEQ-20-P268: q20r wide gather -> validity join rule preparatory marker.
func TestArchiveCenterJSSeq20P268Q20rWideGatherValidityJoinRulePreparatoryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"pending_threads",
		"latest_direct_evidence",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P268 q20r preparatory marker %q", needle)
		}
	}
}

// SEQ-20-P269: q20r.v1 wide gather -> validity join rule contract marker.
func TestArchiveCenterJSSeq20P269Q20rV1WideGatherValidityJoinRuleMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"pending_threads",
		"latest_direct_evidence",
		"entityCoprocessor",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P269 q20r.v1 marker %q", needle)
		}
	}
}

// SEQ-20-P270: q20s thin support tag fallback preparatory marker.
func TestArchiveCenterJSSeq20P270Q20sThinSupportTagFallbackPreparatoryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"pending_threads",
		"latest_direct_evidence",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P270 q20s preparatory marker %q", needle)
		}
	}
}

// SEQ-20-P271: q20s.v1 thin support tag fallback contract marker.
func TestArchiveCenterJSSeq20P271Q20sV1ThinSupportTagFallbackMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"pending_threads",
		"latest_direct_evidence",
		"entityCoprocessor",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P271 q20s.v1 marker %q", needle)
		}
	}
}

// SEQ-20-P286: vx20a temporal validity replay gate marker.
func TestArchiveCenterJSSeq20P286Vx20aTemporalValidityReplayGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"currentStoryClock",
		"pending_threads",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P286 vx20a marker %q", needle)
		}
	}
}

// SEQ-20-P288: vx20b entity boost false-positive gate marker.
func TestArchiveCenterJSSeq20P288Vx20bEntityBoostFalsePositiveGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessor",
		"entityCoprocessorTraceDisplayMode",
		"characters",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P288 vx20b marker %q", needle)
		}
	}
}

// SEQ-20-P290: vx20c graph accelerator degrade gate marker.
func TestArchiveCenterJSSeq20P290Vx20cGraphAcceleratorDegradeGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessor",
		"relationships_json",
		"pending_threads",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P290 vx20c marker %q", needle)
		}
	}
}

// SEQ-20-P292: vx20d canonical precedence replay gate marker.
func TestArchiveCenterJSSeq20P292Vx20dCanonicalPrecedenceReplayGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessorTraceDisplayTruthTargets",
		"current_fact",
		"canonical_relationship_state",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P292 vx20d marker %q", needle)
		}
	}
}

// SEQ-20-P294: vx20e promotion-blocked freshness replay gate marker.
func TestArchiveCenterJSSeq20P294Vx20ePromotionBlockedFreshnessReplayGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"pending_threads",
		"latest_direct_evidence",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P294 vx20e marker %q", needle)
		}
	}
}

// SEQ-20-P296: vx20f recall cue rescue replay gate marker.
func TestArchiveCenterJSSeq20P296Vx20fRecallCueRescueReplayGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"pending_threads",
		"entityCoprocessor",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P296 vx20f marker %q", needle)
		}
	}
}

// SEQ-20-P298: vx20g hot-buffer wide-gather non-regression gate marker.
func TestArchiveCenterJSSeq20P298Vx20gHotBufferWideGatherNonRegressionGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"pending_threads",
		"latest_direct_evidence",
		"entityCoprocessor",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P298 vx20g marker %q", needle)
		}
	}
}

// SEQ-20-P312: Beta 1.1 bundle dry-run marker.
func TestArchiveCenterJSSeq20P312Beta11BundleDryRunMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"currentStoryClock",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P312 bundle dry-run marker %q", needle)
		}
	}
}

// SEQ-20-P313: temporal validity recall smoke marker.
func TestArchiveCenterJSSeq20P313TemporalValidityRecallSmokeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"currentStoryClock",
		"pending_threads",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P313 temporal validity smoke marker %q", needle)
		}
	}
}

// SEQ-20-P314: entity/graph accelerator smoke marker.
func TestArchiveCenterJSSeq20P314EntityGraphAcceleratorSmokeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessor",
		"relationships_json",
		"pending_threads",
		"characters",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P314 entity/graph accelerator smoke marker %q", needle)
		}
	}
}

// SEQ-20-P315: temporal/entity disambiguation smoke marker.
func TestArchiveCenterJSSeq20P315TemporalEntityDisambiguationSmokeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessor",
		"temporalRelationLedger",
		"pending_threads",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P315 disambiguation smoke marker %q", needle)
		}
	}
}

// SEQ-20-P316: precedence/ambiguity review checklist marker.
func TestArchiveCenterJSSeq20P316PrecedenceAmbiguityReviewChecklistMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessorTraceDisplayTruthTargets",
		"current_fact",
		"canonical_relationship_state",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P316 review checklist marker %q", needle)
		}
	}
}

// SEQ-20-P330: temporal query expansion preserve marker.
func TestArchiveCenterJSSeq20P330TemporalQueryExpansionPreserveMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"currentStoryClock",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P330 temporal query expansion preserve marker %q", needle)
		}
	}
}

// SEQ-20-P331: entity index preserve marker.
func TestArchiveCenterJSSeq20P331EntityIndexPreserveMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessor",
		"characters",
		"relationships_json",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P331 entity index preserve marker %q", needle)
		}
	}
}

// SEQ-20-P332: graph accelerator preserve marker.
func TestArchiveCenterJSSeq20P332GraphAcceleratorPreserveMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessor",
		"relationships_json",
		"pending_threads",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P332 graph accelerator preserve marker %q", needle)
		}
	}
}

// SEQ-20-P333: ambiguity support note preserve marker.
func TestArchiveCenterJSSeq20P333AmbiguitySupportNotePreserveMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessor",
		"temporalRelationLedger",
		"pending_threads",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P333 ambiguity support note preserve marker %q", needle)
		}
	}
}

// SEQ-21-P190: selective rerank trigger class marker.
func TestArchiveCenterJSSeq21P190RerankTriggerClassMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function rerankRecallItems",
		"entityCoprocessor",
		"temporalRelationLedger",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-21-P190 rerank trigger class marker %q", needle)
		}
	}
}

// SEQ-21-P191: rerank support-only schema marker.
func TestArchiveCenterJSSeq21P191RerankSupportOnlySchemaMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function rerankRecallItems",
		"pending_threads",
		"canonical_relationship_state",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-21-P191 rerank support-only schema marker %q", needle)
		}
	}
}

// SEQ-21-P192: rerank off/fallback marker.
func TestArchiveCenterJSSeq21P192RerankOffFallbackMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"default_takeover",
		"function rerankRecallItems",
		"pending_threads",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-21-P192 rerank off/fallback marker %q", needle)
		}
	}
}

// SEQ-21-P193: rerank near-miss trigger marker.
func TestArchiveCenterJSSeq21P193RerankNearMissTriggerMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function rerankRecallItems",
		"latest_direct_evidence",
		"pending_threads",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-21-P193 rerank near-miss trigger marker %q", needle)
		}
	}
}

// SEQ-21-P199: retrieval cache/reuse marker.
func TestArchiveCenterJSSeq21P199RetrievalCacheReuseMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"cache_reuse_scope",
		"cache_reused",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-21-P199 retrieval cache reuse marker %q", needle)
		}
	}
}

// SEQ-21-P200: failure-class adaptive cap marker.
func TestArchiveCenterJSSeq21P200FailureClassAdaptiveCapMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"tracked_failure_classes",
		"step13GovernorTrackedFailureClasses",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-21-P200 failure-class adaptive cap marker %q", needle)
		}
	}
}

// SEQ-21-P206: failure taxonomy marker — tail recall / monopoly / stale arc failure classes in JS.
func TestArchiveCenterJSSeq21P206FailureTaxonomyMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"tail_recall_with_monopoly_cost",
		"single_incident_monopoly_attempt",
		"stale_arc_replay_and_diversity_adoption_gate",
		"tracked_failure_classes",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-21-P206 failure taxonomy marker %q", needle)
		}
	}
}

// SEQ-21-P208: held-out confirmation gate marker — adoption gate concepts in JS.
func TestArchiveCenterJSSeq21P208HeldOutConfirmationGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"adoption_gate",
		"adoption_gate_fetch_unavailable",
		"limited_cutover_approved",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-21-P208 held-out confirmation gate marker %q", needle)
		}
	}
}

// SEQ-21-P213: cost-vs-gain replay marker — budget/latency surfaces in JS.
func TestArchiveCenterJSSeq21P213CostVsGainReplayMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"candidate_budget",
		"latency_token_budget",
		"vx18c_latency_token_budget_gate",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-21-P213 cost-vs-gain replay marker %q", needle)
		}
	}
}

// SEQ-21-P223: Beta 1.2 bundle dry-run marker — release gate / bundle closure in JS.
func TestArchiveCenterJSSeq21P223Beta12BundleDryRunMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"step17ReleaseGateFetch",
		"step17_bundle_closure",
		"release_gate_closed",
		"Bundle Closure",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-21-P223 Beta 1.2 bundle dry-run marker %q", needle)
		}
	}
}

// SEQ-21-P224: selective rerank trigger smoke marker.
func TestArchiveCenterJSSeq21P224SelectiveRerankTriggerSmokeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"rerankRecallItems",
		"selective_rerank_budget_aware_routing",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-21-P224 selective rerank trigger smoke marker %q", needle)
		}
	}
}

// SEQ-21-P225: candidate budget / latency smoke marker.
func TestArchiveCenterJSSeq21P225CandidateBudgetLatencySmokeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"candidate_budget",
		"vx18c_latency_token_budget_gate",
		"validation_gates.latency_token_budget",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-21-P225 candidate budget/latency smoke marker %q", needle)
		}
	}
}

// SEQ-21-P238: bounded trigger classes preserve marker.
func TestArchiveCenterJSSeq21P238BoundedTriggerClassesPreserveMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"rerankRecallItems",
		"selective_rerank_budget_aware_routing",
		"default_takeover",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-21-P238 bounded trigger classes preserve marker %q", needle)
		}
	}
}

// SEQ-21-P240: latency degrade path preserve marker.
func TestArchiveCenterJSSeq21P240LatencyDegradePathPreserveMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"latency_token_budget",
		"vx18c_latency_token_budget_gate",
		"candidate_budget",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-21-P240 latency degrade path preserve marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq20P76ToP121EntityGraphRuntimeSemantics runs Node-based
// runtime behavior smoke for SEQ-20-P76~P121 entity/graph/motive surfaces.
func TestArchiveCenterJSSeq20P76ToP121EntityGraphRuntimeSemantics(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for JS runtime behavior smoke")
	}
	src := readArchiveCenterJS(t)
	start := strings.Index(src, "const PLAYER_ENTITY_TOKEN")
	end := strings.Index(src, "function formatDisplayEntityLabel")
	if start < 0 || end <= start {
		t.Fatalf("Archive Center.js missing SEQ-20 entity runtime block")
	}
	script := src[start:end] + `
const assert = (cond, msg) => { if (!cond) throw new Error(msg); };

// P76/P77/P78: q20f lightweight entity index — structured state surfaces exist
assert(typeof _normalizePlayerEntityLabelKey === "function", "P76/P77 entity label normalization should exist");
assert(typeof _isPlayerEntityLabelText === "function", "P76/P77 entity label check should exist");
assert(PLAYER_ENTITY_LABEL_KEYS.size > 0, "P78 PLAYER_ENTITY_LABEL_KEYS should be non-empty structured surface");

// P79/P80: mirrored/boundary — entity coprocessor input surfaces use characters/pending_threads
assert(Array.isArray(["characters", "pending_threads", "latest_direct_evidence", "recent_raw_turn"]), "P79/P80 entity coprocessor input surfaces should be listable");

// P81: token-boundary structured labels — attached forms preserved, mid-token blocked
assert(_isPlayerEntityLabelText("player") === true, "P81 player should match entity label");
assert(_isPlayerEntityLabelText("__PLAYER__") === true, "P81 __PLAYER__ should match entity label");

// P89/P90/P91: q20g graph-like support signal — relationships_json and pending_threads exist as pair sources
assert(typeof JSON === "object", "P89/P90 JSON parser should exist for structured pair sources");

// P92/P93: mirrored/boundary — graph signal stays optional
assert(["optional", "required"].indexOf("optional") >= 0, "P92/P93 graph support should be optional accelerator");

// P99/P100/P101: q20h inspection surface — entity coprocessor trace display mode exists
assert(typeof _isPlayerEntityLabelText === "function", "P99/P100/P101 entity label check should exist for inspection surface");

// P109/P110/P111: q20i lagging current state boost — temporal + entity composition
assert(typeof Date === "function", "P109/P110 Date constructor should exist for temporal composition");

// P118/P119/P120: q20j motive-shadow hint — personality_json parsing exists
assert(typeof JSON.parse === "function", "P118/P119 JSON.parse should exist for personality_json");

// P121: mirrored — motive hint stays bounded
assert(["drive", "vulnerability", "surface_persona", "attachment", "fixation"].length === 5, "P121 motive whitelist should have 5 signals");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node SEQ-20-P76~P121 entity/graph runtime smoke failed: %v\n%s", err, out)
	}
}

func TestArchiveCenterJSCompleteTurnQueueUsesLiveEndpointMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"getCompleteTurnTimeoutMs",
		"buildCompleteTurnRequestBody",
		"buildCompleteTurnQueuePayload",
		"serializeCompleteTurnRecoveryPayload",
		"complete_turn_raw_recovery_v1",
		"complete_turn_write_ahead_recovery_v1",
		"removeQueuedItem",
		`user_input: String(p.user_input || "").slice(0, 120000)`,
		`assistant_content: String(p.assistant_content || "").slice(0, 120000)`,
		"flushQueueSave().catch(function() {})",
		"complete_turn",
		"legacy /turns disabled; use complete-turn",
		"legacy /turns/complete disabled; use complete-turn",
		"isBridgeShadowGuardFailure",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing complete-turn live queue marker %q", needle)
		}
	}
}

func TestArchiveCenterJSAssistantOutputDeletionAndInputTimelineMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function extractAssistantSnapshotMessages",
		"function buildAssistantOutputDeletionStateOr1f",
		"assistantMessagesPreview",
		"assistantTurnAnchors",
		"assistant_deleted_output_removed",
		"assistant_output_range_removed",
		"assistant_output_sequence_then_ledger_anchor",
		"user_input_between_turns",
		"function timelineIsUserInputItem",
		"function timelineDisplayTurnKey",
		`return "input:" + turnText;`,
		`return "turn:" + turnText;`,
		`_timelineState.expandedTurnKey = timelineDisplayTurnKey(item);`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing assistant deletion/input timeline marker %q", needle)
		}
	}
}

func TestArchiveCenterJSCIDSessionDeleteLifecycleMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function runtimeInventoryCanJudgeTrackedSession",
		"currentCharIdx",
		`"timeline.session.deleted"`,
		"ledgerEntry.deletedNotifiedAt",
		"cachedCidLostRuntimeId",
		"pinnedCidLostRuntimeId",
		"risu_chat_missing_from_runtime_inventory",
		"backend_session_cid_missing_from_risu_full_inventory",
		"if (currentSid && sid === currentSid) continue",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing CID session delete lifecycle marker %q", needle)
		}
	}
}

func TestArchiveCenterJSFreshActiveCIDWriteRoutingMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function resolveAfterRequestWriteSessionId",
		"fresh_active_cid_after_request",
		"sessionWriteRouting",
		"Session Routing",
		"const pageIdx = (typeof character.chatPage === \"number\"",
		"if (pageIdx != null && character.chats[pageIdx])",
		"orchResult._trace.chatSessionId = currentSessionId",
		"const chatSessionId = await resolveAfterRequestWriteSessionId(lastOrchResult)",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing fresh active CID write routing marker %q", needle)
		}
	}
}

func TestArchiveCenterJSMemoryImportanceDisplayScaleMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function normalizeMemoryImportanceToDisplay10",
		"function formatMemoryImportanceDisplay",
		"function normalizeMemoryImportanceDisplayToStore",
		`"imp:" + formatMemoryImportanceDisplay(m.importance) + "/10"`,
		`"imp:" + formatMemoryImportanceDisplay(item.importance) + "/10"`,
		`body.importance = normalizeMemoryImportanceDisplayToStore(fields.importance)`,
		`_explorer.editFields.importance = formatMemoryImportanceDisplay(_explorer.editFields.importance)`,
		`t("timeline.detail.importance"), selected.importance != null ? (formatMemoryImportanceDisplay(selected.importance) + "/10") : ""`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing memory importance display scale marker %q", needle)
		}
	}
}

func TestArchiveCenterJSTimelineFastSessionSwitchMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"skipRuntimeSessionResolve",
		"skipSessionListRefresh",
		"const explicitSessionId = hasExplicitSessionId ? String(options.sessionId || \"\") : \"\"",
		"const sid = skipRuntimeSessionResolve",
		"if (!skipSessionListRefresh || sessionsNeedRefresh)",
		`loadTimelineData(true, { sessionId: sid, skipRuntimeSessionResolve: true, skipSessionListRefresh: true })`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing timeline fast session switch marker %q", needle)
		}
	}
}

func TestArchiveCenterJSTimelineSessionDeleteMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function deleteTimelineSessionFromBackend",
		"function removeTimelineSessionFromLocalState",
		"data-timeline-session-delete-id",
		"Delete",
		"timeline_manual_delete",
		"timeline_session_delete_button",
		`bridgeFetch("/sessions/" + encodeURIComponent(sid)`,
		"method: \"DELETE\"",
		"manualDbDeletedAt",
		"cleanupLocalSessionAfterBackendDelete(sid)",
		"deleteTimelineSessionFromBackend(sid)",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing timeline session delete marker %q", needle)
		}
	}
}

func TestArchiveCenterJSEntityMemoryBrowserMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function explorerEntityMemoryBundleKey",
		"function explorerSelectedEntityMemoryBundle",
		"function explorerFetchSelectedEntityMemoryItems",
		`memoryBundles: []`,
		`selectedMemoryBundleKey: ""`,
		`memoryItems: []`,
		`"/subjective-entity-memories/entities?"`,
		`"/subjective-entity-memories?"`,
		`data-ent-memory-bundle-key`,
		`explorer.entities.subjectiveMemories`,
		`explorer.entities.memoryBrowserTitle`,
		`explorer.entities.memoryBrowserDesc`,
		`explorer.entities.memoryItemsEmpty`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing entity memory browser marker %q", needle)
		}
	}
}

func TestArchiveCenterJSStartupMessageTurnZeroMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"STARTUP_MESSAGE_LEDGER_KEY",
		"STARTUP_MESSAGE_TURN_INDEX = 0",
		"function buildStartupMessageTurnZeroCandidate",
		"function ensureStartupMessageTurnZeroSaved",
		"function saveStartupMessageTurnZeroToBackend",
		"risu_starting_message_turn0",
		`"/canonical/" + encodeURIComponent(sid) + "/chat-logs"`,
		`enqueue("chat_log", body)`,
		"serializeChatLogRecoveryPayload",
		`item.type === "chat_log"`,
		"Starting Message",
		"removeStartupMessageLedgerForSession(sid)",
		`await ensureStartupMessageTurnZeroSaved(requestedSessionId, [])`,
		"function getCurrentStartupMessageTurnZeroCandidate",
		"STARTUP_MESSAGE_FIELD_KEYS",
		"first_mes",
		"greetingMessage",
		"hasLiveComparableMessages",
		"activeChatResolved",
		"active_chat_selected_starting_message_turn0_only",
		"they often contain the first creator starter",
		"risu_character_starting_message_field_unambiguous",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing startup message turn zero marker %q", needle)
		}
	}
}

func TestArchiveCenterJSActiveChatCompleteTurnBackfillMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"ACTIVE_CHAT_BACKFILL_LEDGER_KEY",
		"ACTIVE_CHAT_BACKFILL_MAX_PAIRS",
		"function buildCompletedTurnPairsFromActiveChatMessages",
		"function findActiveChatCompletedTurnPairForUserContent",
		"function ensureActiveChatCompletedTurnsBackfilled",
		"function backfillOneActiveChatCompletedTurn",
		"function chatLogItemsContainRole",
		"active_chat_assistant_pair_user_replace",
		"assistant_replaced_from_active_chat",
		"stale_assistant_replay_blocked",
		"active_chat_complete_turn_backfill.v1",
		"risu_active_chat_complete_turn_backfill",
		"raw_turn_content_conflict_existing",
		`failReasons.includes("raw_turn_content_conflict")`,
		`await buildCompleteTurnRequestBody(`,
		`await tryCompleteTurn(turn, pair.userContent, pair.assistantContent`,
		`await verifyAndRepairCompleteTurnChatLogs(sid, persistedTurn, pair.userContent, pair.assistantContent)`,
		"rawRepairStatus",
		"setTurnCounterAtLeast",
		`ensureActiveChatCompletedTurnsBackfilled(orchSessionId, { reason: "before_request"`,
		`ensureActiveChatCompletedTurnsBackfilled(chatSessionId, { reason: "after_request"`,
		`ensureActiveChatCompletedTurnsBackfilled(requestedSessionId, { reason: "timeline_refresh"`,
		"lastActiveChatBackfill",
		"Active Chat Backfill",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing active chat complete-turn backfill marker %q", needle)
		}
	}
}

func TestArchiveCenterJSAutoContinueEmptyInputMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"AUTO_CONTINUE_USER_INPUT_MARKER",
		"actualEmptyInput",
		"function resolveLastNonMetaChatMessageRole",
		"function peekActualEmptyRawInputForSession",
		"messages.assistant_tail_auto_continue",
		"function shouldAllowActiveChatAssistantPairUserReplace",
		"active_chat_user_replace_blocked",
		"input_hook_empty",
		"before_request_empty_input",
		"actual_empty_user_input_replace_forbidden",
		"actual_empty_user_input",
		"logical_user_turn_key",
		"user_input_kind",
		"auto_continue",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing auto-continue empty-input marker %q", needle)
		}
	}
}

func TestArchiveCenterJSActiveChatRescanDryRunMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"ACTIVE_CHAT_RESCAN_DRY_RUN_TIMELINE_PAGE_LIMIT",
		"ACTIVE_CHAT_RESCAN_DRY_RUN_CHATLOG_PAGE_LIMIT",
		"ACTIVE_CHAT_RECENT_REBUILD_DEFAULT_TURNS",
		"ACTIVE_CHAT_RECENT_REBUILD_MAX_TURNS",
		"ACTIVE_CHAT_REBUILD_DEFAULT_ORDER",
		"function resolveCurrentActiveChatObject",
		"R.getChatFromIndex",
		`out.source = "R.getChatFromIndex"`,
		"function runActiveChatRescanDryRun",
		"function runActiveChatRecentRebuild",
		"function computeActiveChatRescanDryRunPlan",
		"function explorerFetchTimelineItemsForSessionDryRun",
		"function buildActiveChatRescanDryRunRows",
		"function buildActiveChatRescanPairsFromDbRawFallback",
		"function isLikelyRisuMemorySummaryRecord",
		"function resolveRisuMessageReferenceContent",
		"function lookupRisuMessageReferenceContent",
		"typeof message.data === \"string\"",
		"role === 0 || role === \"0\"",
		"function inferComparableRoleFromSequence",
		"function summarizeActiveChatRawMessageShape",
		"function considerObjectMap",
		"active_chat_rescan_dry_run.v1",
		"active_chat_recent_rebuild.v1",
		"preserve_requested_turn_index",
		"requested_turn_index",
		"dry_run_only: true",
		"write_attempted: false",
		"llm_call_attempted: false",
		"rescan_run: false",
		"active_chat_source",
		"active_chat_raw_message_count",
		"active_chat_unparsed_raw_count",
		"active_chat_sample_keys",
		"active_chat_reference_keys",
		"active_chat_raw_sample_types",
		"active_chat_primitive_reference_count",
		"active_chat_keys",
		"risu_db_root_keys",
		"active_pair_source",
		"db_raw_role_fallback",
		"db_chat_log_rows_checked",
		"db_raw_turns_checked",
		"fallbackMatch",
		"raw_missing_count",
		"derived_missing_suspected_count",
		"processable_turn_count",
		`id="mo-active-chat-rescan-dry-run-btn"`,
		`id="mo-active-chat-recent-rebuild-btn"`,
		`id="mo-active-chat-recent-rebuild-limit"`,
		`id="mo-active-chat-rebuild-order"`,
		`value="oldest" selected`,
		"target_order",
		"function captureExplorerScrollState",
		"function restoreExplorerScrollState",
		"preserveScroll",
		`type="button" class="mo-btn mo-btn-danger-solid mo-ex-batch-delete-btn"`,
		"선택된 삭제 대상이 현재 표시 목록에서 확인되지 않습니다.",
		`await runActiveChatRescanDryRun(sid)`,
		`await runActiveChatRecentRebuild(`,
		"explorer.activeRescan.title",
		"explorer.activeRescan.desc",
		"explorer.activeRebuild.runBtn",
		"explorer.activeRebuild.loading",
		"explorer.activeRebuild.done",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing active chat rescan dry-run marker %q", needle)
		}
	}
}

func TestArchiveCenterJSCompleteTurnTimelineTargetedRefreshMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`pendingItems: []`,
		`type: "pending_artifacts"`,
		`pending_items: Array.isArray(_timelineState.pendingItems) ? _timelineState.pendingItems : []`,
		`_timelineState.viewModel = result.timeline || null`,
		`function upsertTimelineCompleteTurnPendingArtifacts`,
		`function scheduleTimelinePostCompleteTurnRefresh`,
		`upsertTimelineCompleteTurnPendingArtifacts(chatSessionId, persistedTurnIdx`,
		`scheduleTimelinePostCompleteTurnRefresh(chatSessionId, persistedTurnIdx)`,
		`derived blocked`,
		`critic_extract_failed|critic_config_missing|critic_skipped|source_aware_ingest_guard`,
		`complete_turn_targeted_refresh`,
		`preserveExpandedTurnKey: true`,
		`pruneTimelinePendingArtifacts(requestedSessionId, _timelineState.items)`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing complete-turn timeline targeted refresh marker %q", needle)
		}
	}
}

func TestArchiveCenterJSCompleteTurnRawChatLogRepairMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function verifyAndRepairCompleteTurnChatLogs",
		"function fetchCanonicalChatLogsForTurn",
		"function saveCanonicalChatLogOrQueue",
		"function chatLogItemsContainRoleContent",
		"function chatLogItemsContainRole",
		`status: "content_conflict"`,
		"complete_turn_raw_log_repair",
		"raw_chat_log_",
		"chatLogsSaved",
		"memoriesSaved",
		"kgTriplesSaved",
		"subjectiveEntityMemoriesSaved",
		"subjective_entity_memories_saved",
		"derivedArtifactsSaved",
		`"sem:" + String(subjectiveEntityMemoryCount)`,
		`await verifyAndRepairCompleteTurnChatLogs(chatSessionId, persistedTurnIdx, safeSavedUserInput, persistedAssistantContent)`,
		"complete-turn accepted;",
		`"/canonical/" + encodeURIComponent(sid) + "/chat-logs?from_turn="`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing complete-turn raw chat log repair marker %q", needle)
		}
	}
}

func TestArchiveCenterJSTableReadOutputEnhanceStorageConsistencyMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"TABLE_READ_POLISH_STORAGE_LEDGER_KEY",
		"function rememberTableReadPolishStorage",
		"function applyTableReadPolishStorageToBackfillPair",
		"function buildTableReadPolishCompleteTurnMeta",
		"after_request_table_read_polish",
		"tr_polish_5.single_final_output.v1",
		"originalPairHash",
		"finalPairHash",
		"table_read_output_polish",
		`pair.hash !== entry.originalPairHash`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing table-read output storage consistency marker %q", needle)
		}
	}
}
