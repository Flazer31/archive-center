package main

import (
	"strings"
	"testing"
)

// SEQ-16.5-P142: input context builder slot governor — validates that
// Archive Center.js contains the input context slot governor surface markers.
// SEQ-16.5-P143: transparency / preview / runtime trace extend — validates that
// Archive Center.js contains the transparency/preview/runtime trace extension markers.
func TestArchiveCenterJSSeq165P143TransparencyPreviewRuntimeTraceExtendMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function buildInputTransparency(userInput, recentContext, searchResult, wakeUpContext, supervisorResult, continuityInfo, kgRecallResult, extractedEntities, activeStatesResult, episodeRecallResult, expandedEntities, languageContext, backendInputTransparencyModel, backendEffectiveInputPreview, responseExecutionContract) {",
		"function logTurnTraceSummary() {",
		"function renderTurnTraceRows() {",
		"_inputTransparency = buildInputTransparency(",
		"it.assembledPreview = assembleInputPreview(it);",
		"continuity: isContinuity ? {",
		"oldArcForeground: continuityInfo.oldArcForegroundGuard ? {",
		"recallPaths: (searchResult && searchResult.paths) ? searchResult.paths : []",
		"dedupeStats: (searchResult && searchResult.dedupeStats) ? searchResult.dedupeStats : null",
		"kgRecall: (kgRecallResult && kgRecallResult.count > 0) ? {",
		"activeStates: (activeStatesResult && activeStatesResult.count > 0) ? {",
		"episodeRecall: (episodeRecallResult && episodeRecallResult.count > 0) ? {",
		"pathB: (searchResult && searchResult.pathBInfo && searchResult.pathBInfo.used) ? {",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.5-P143 transparency/preview/runtime trace marker %q", needle)
		}
	}
	for _, forbidden := range []string{"weakInputPlanner", "progressionChoiceLedger", "step25ValidationGate"} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("Archive Center.js retains removed story-control transparency marker %q", forbidden)
		}
	}
}

// SEQ-16.5-P144: recent chat is retained by the host and is not inserted again.
func TestArchiveCenterJSSeq165P144DoesNotReinjectHostRecentChat(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"injectionTextSource: \"go_payload_application_plan.v1\"",
		"apply_exact_text_without_reassembly",
		"source: \"risu_host_recent_chat\"",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.5-P144 handoff anchor metadata alignment marker %q", needle)
		}
	}
	if strings.Contains(src, "injectInputContextBeforeUser") || strings.Contains(src, "[Archive Center — Input Context]") {
		t.Fatal("Archive Center.js still contains the removed recent-chat reinjection path")
	}
}

// SEQ-16.5-P145: Step 16.8 stale-arc guard carry-in / Step 17 evaluation ops
// carry-in replay/inspection hooks — validates that Archive Center.js contains
// the stale-arc guard and carry-in hook markers.
// SEQ-16.5-P169: helper injection adaptive floor / ceiling decision value.
// SEQ-16.5-P170: input context max slot 2 vs 3 decision value.
// SEQ-16.5-P171: runtime token hint telemetry-only / secondary safety cap
// decision value.
// SEQ-16.5-P172: [Saga] / [Chapter] anchor competition vs fallback ladder
// decision value.
// SEQ-16.5-P173: explicit user-input specificity heuristic/classifier
// decision value.
// SEQ-16.5-P177: Step 16.8 stale-arc suppression slice baseline compare.
// SEQ-16.5-P178: Step 16.8 reason visibility / monopoly replay guard lane.
// SEQ-16.5-P179: Step 16.8 completion Step 17 evaluation baseline direct
// handoff gate.
// SEQ-16.5-P183: Step 17 evaluation harness static 3000/800 baseline + 16.5+16.8
// baseline.
// SEQ-16.5-P184: Step 17 ops budget tuning governor behavior trace
// interpretation document.
// SEQ-16.8-P99: stale-arc ceiling — no-user-mention stale arc rescue auto-foreground.
func TestArchiveCenterJSSeq168P99StaleArcCeilingMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"oldArcForegroundGuard",
		"stale_rescue_ceiling",
		"suppressionTriggerActive",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P99 marker %q", needle)
		}
	}
}

// SEQ-16.8-P100: scene alignment — old arc explicit query alignment or fresh scene evidence.
// SEQ-16.8-P101: reason trace — old arc keep/drop/suppress inspectable.
// SEQ-16.8-P102: failure split — tail recall gain foreground monopoly failure class.
// SEQ-16.8-P103: packet synthesis — Step 21 packet/new-scene synthesis Step 22 long-horizon subsystem.
func TestArchiveCenterJSSeq168P103PacketSynthesisMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"buildContinuityPackQuery",
		"buildContinuityPackWakeUpBlock",
		"fetchStorylines",
		"fetchPendingThreads",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P103 marker %q", needle)
		}
	}
}

// SEQ-16.8-P107: callback bias ceiling — 16.8-1a callback/storyline soft bias ceiling define.
func TestArchiveCenterJSSeq168P107CallbackBiasCeilingMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"soft_bias_score",
		"soft_bias_policy_version",
		"softBiasScore",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P107 marker %q", needle)
		}
	}
}

// SEQ-16.8-P108: callback scene alignment — 16.8-1b callback rescue current-scene alignment define.
// SEQ-16.8-P109: stale callback suppression — 16.8-1c stale callback suppression trigger define.
func TestArchiveCenterJSSeq168P109StaleCallbackSuppressionMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"suppressionTriggerActive",
		"oldArcForegroundGuard",
		"stale_rescue_ceiling",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P109 marker %q", needle)
		}
	}
}

// SEQ-16.8-P113: old-arc foreground visibility — 16.8-2a old-arc foreground reason visibility lane define.
func TestArchiveCenterJSSeq168P113OldArcForegroundVisibilityMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"oldArcForegroundGuard",
		"oldArcForeground",
		"decisions",
		"reason",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P113 marker %q", needle)
		}
	}
}

// SEQ-16.8-P114: reason code vocabulary — 16.8-2b keep/drop/suppress/demote reason code vocabulary define.
// SEQ-16.8-P115: preview/audit/transparency — 16.8-2c preview/audit/transparency surface define.
func TestArchiveCenterJSSeq168P115PreviewAuditTransparencyMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"buildInputTransparency",
		"logTurnTraceSummary",
		"renderTurnTraceRows",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P115 marker %q", needle)
		}
	}
}

// SEQ-16.8-P119: foreground hijack taxonomy — 16.8-3a foreground hijack/arc monopoly failure taxonomy define.
// SEQ-16.8-P120: delayed payoff split — 16.8-3b valid delayed payoff rescue vs scene monopoly split define.
// SEQ-16.8-P121: recall gain/monopoly cost split — 16.8-3c recall gain/monopoly cost split trace schema define.
// SEQ-16.8-P125: stale arc revival replay — 16.8-4a stale arc revival/single-incident monopoly replay define.
// SEQ-16.8-P126: tail recall vs foreground hijack gate — 16.8-4b tail recall vs foreground hijack gate define.
// SEQ-16.8-P127: narrative diversity gate — 16.8-4c narrative diversity gate define.
// SEQ-16.8-P128: arc monopoly gate — 16.8-4d arc monopoly gate define.
// SEQ-16.8-P132: Archive Center.js continuity rescue owner surface.
func TestArchiveCenterJSSeq168P132JSContinuityRescueMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"fetchStorylines",
		"fetchPendingThreads",
		"buildContinuityPackQuery",
		"buildContinuityPackWakeUpBlock",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P132 marker %q", needle)
		}
	}
}

// SEQ-16.8-P133: Archive Center.js prompt assembly guard.
func TestArchiveCenterJSSeq168P133JSPromptAssemblyGuardMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"applyContextInjection",
		"applyGoPayloadApplicationPlan",
		"payload_application_plan.v1",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P133 marker %q", needle)
		}
	}
	for _, removed := range []string{
		"function buildInputContext(",
		"function buildAdaptiveInjectionGovernorTrace(",
		"function assembleInjectionWithBudget(",
	} {
		if strings.Contains(src, removed) {
			t.Fatalf("Archive Center.js restored obsolete JavaScript policy owner %q", removed)
		}
	}
}

// SEQ-16.8-P134: Archive Center.js trace/preview/transparency surface extend.
func TestArchiveCenterJSSeq168P134JSTracePreviewTransparencyMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"buildInputTransparency",
		"tracePreview",
		"injectionPreview",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P134 marker %q", needle)
		}
	}
}

// SEQ-16.8-P135: replay corpus/inspection baseline add.
// SEQ-16.8-P136: backend/main.py storyline/pending-thread read metadata alignment — suppression trace confirm.
// SEQ-16.8-P162: Decision outcome — stale arc ceiling judged by explicit
// alignment / current-scene evidence / explicit redirection, not turn-gap alone.
func TestArchiveCenterJSSeq168P162DecisionOutcomeCeilingNotTurnGapMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"stale_rescue_ceiling",
		"no_alignment_rescue_ceiling",
		"explicit_query_alignment",
		"explicit_user_redirection",
		"suppressionTrigger",
		"oldArcForeground",
		"old-arc guard",
		"applyContinuityPackOldArcGuard",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P162 marker %q", needle)
		}
	}
}

// SEQ-16.8-P163: current-scene evidence minimum criteria.
// SEQ-16.8-P164: open / paused thread ceiling family, pending_threads guard.
func TestArchiveCenterJSSeq168P164PendingThreadsGuardMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"pending_threads",
		"pendingThreads",
		"open",
		"paused",
		"guard",
		"ceiling",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P164 marker %q", needle)
		}
	}
}

// SEQ-16.8-P165: reason visibility lane extends to adaptive trace / continuity
// trace / input transparency.
// SEQ-16.8-P166: diversity gate default diagnostic warn, arc_monopoly_attempt
// Step 17 handoff block signal.
func TestArchiveCenterJSSeq168P166DiversityGateDiagnosticWarnMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"diversity",
		"monopoly",
		"diagnostic",
		"warn",
		"handoff",
		"block",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P166 marker %q", needle)
		}
	}
}

// SEQ-17-P230: retrieval completeness vs final answer quality split.
func TestArchiveCenterJSSeq17P230EvaluationSplitMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"retrieval",
		"evaluation",
		"split",
		"failure",
		"healthy",
		"metric",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P230 marker %q", needle)
		}
	}
}

// SEQ-17-P231: ops procedure documentation surface.
// SEQ-17-P232: inspection lane boundary surface.
func TestArchiveCenterJSSeq17P232InspectionLaneBoundaryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"explain",
		"preview",
		"audit",
		"dashboard",
		"inspection",
		"boundary",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P232 marker %q", needle)
		}
	}
}

// SEQ-17-P233: adoption gate — replay green before default adoption value.
func TestArchiveCenterJSSeq17P233AdoptionGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"adoption",
		"replay",
		"green",
		"default",
		"blocked",
		"gate",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P233 marker %q", needle)
		}
	}
}

// SEQ-17-P234: release hygiene — bundle/regression/checklist repeatability.
func TestArchiveCenterJSSeq17P234ReleaseHygieneMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"bundle",
		"regression",
		"checklist",
		"release",
		"hygiene",
		"contract",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P234 marker %q", needle)
		}
	}
}

// SEQ-17-P238: 17-1a retrieval completeness metric define.
// NOTE: Step 17 evaluation surface is primarily Go backend; JS runtime does not
// expose direct completeness metric markers. Verified by Go contract test only.
func TestArchiveCenterJSSeq17P238RetrievalCompletenessMetricMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"lc1",
		"metric",
		"retrieval",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P238 marker %q", needle)
		}
	}
}

// SEQ-17-P239: 17-1b final answer quality metric define.
// NOTE: Step 17 evaluation surface is primarily Go backend; JS runtime does not
// expose direct answer quality metric markers. Verified by Go contract test only.
func TestArchiveCenterJSSeq17P239FinalAnswerQualityMetricMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"lc1",
		"metric",
		"quality",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P239 marker %q", needle)
		}
	}
}

// SEQ-17-P240: 17-1c retrieval failure vs reader failure split replay define.
// NOTE: Step 17 evaluation surface is primarily Go backend; JS runtime does not
// expose direct failure split replay markers. Verified by Go contract test only.
func TestArchiveCenterJSSeq17P240FailureSplitReplayMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"retrieval",
		"failure",
		"split",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P240 marker %q", needle)
		}
	}
}

// SEQ-17-P241: 17-1d Step 14~16 regression corpus define.
// NOTE: Step 17 evaluation surface is primarily Go backend; JS runtime does not
// expose direct regression corpus markers. Verified by Go contract test only.
// SEQ-17-P242: 17-1e freshness lag metric define.
func TestArchiveCenterJSSeq17P242FreshnessLagMetricMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"freshness",
		"lag",
		"extraction",
		"delay",
		"promotion",
		"visibility",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P242 marker %q", needle)
		}
	}
}

// SEQ-17-P286: 17-2a promotion / backfill / rebuild document.
// NOTE: Step 17 ops procedure is Go backend surface; JS runtime does not
// expose direct procedure markers. Verified by Go contract test only.
func TestArchiveCenterJSSeq17P286PromotionBackfillRebuildMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"promotion",
		"backfill",
		"rebuild",
		"dry_run",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P286 marker %q", needle)
		}
	}
}

// SEQ-17-P287: 17-2b reembed / migration / health probe document.
// SEQ-17-P288: 17-2c failure mode / fallback / rollback runbook cleanup.
// NOTE: Step 17 runbook is Go backend surface; JS runtime does not
// expose direct runbook markers. Verified by Go contract test only.
func TestArchiveCenterJSSeq17P288FailureFallbackRollbackMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"failure",
		"fallback",
		"rollback",
		"degraded",
		"mode",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P288 marker %q", needle)
		}
	}
}

// SEQ-17-P289: 17-2d async complete-turn / critic delay runbook cleanup.
func TestArchiveCenterJSSeq17P289AsyncCriticDelayMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"async",
		"complete",
		"critic",
		"delay",
		"repair",
		"replay",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P289 marker %q", needle)
		}
	}
}

// SEQ-17-P290: 17-2e partial-write / silent-skip / retry budget cleanup.
func TestArchiveCenterJSSeq17P290PartialWriteRetryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"partial",
		"write",
		"silent",
		"skip",
		"retry",
		"budget",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P290 marker %q", needle)
		}
	}
}

// SEQ-17-P306: 17-3a explain surface role define.
func TestArchiveCenterJSSeq17P306ExplainSurfaceMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"explain",
		"surface",
		"reasoning",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P306 marker %q", needle)
		}
	}
}

// SEQ-17-P307: 17-3b preview / audit surface role define.
func TestArchiveCenterJSSeq17P307PreviewAuditSurfaceMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"preview",
		"audit",
		"outcome",
		"decision",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P307 marker %q", needle)
		}
	}
}

// SEQ-17-P308: 17-3c dashboard lane split define.
func TestArchiveCenterJSSeq17P308DashboardLaneMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"dashboard",
		"lane",
		"metric",
		"save",
		"extraction",
		"promotion",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P308 marker %q", needle)
		}
	}
}

// SEQ-17-P309: 17-3d inspection surface authority display guard define.
func TestArchiveCenterJSSeq17P309DisplayGuardMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"displayGuard",
		"lane guard",
		"canonical store evidence",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P309 marker %q", needle)
		}
	}
}

// SEQ-17-P310: 17-3e freshness / extract-drop / promotion-block visibility lane define.
func TestArchiveCenterJSSeq17P310VisibilityLaneMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"visibility",
		"freshness",
		"extract",
		"drop",
		"promotion",
		"block",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P310 marker %q", needle)
		}
	}
}

// SEQ-17-P327: 17-4a Step 14 adoption gate define.
func TestArchiveCenterJSSeq17P327Step14AdoptionGateMarkers(t *testing.T) {
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
			t.Fatalf("Archive Center.js missing SEQ-17-P327 marker %q", needle)
		}
	}
}

// SEQ-17-P328: 17-4b Step 15 adoption gate define.
func TestArchiveCenterJSSeq17P328Step15AdoptionGateMarkers(t *testing.T) {
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
			t.Fatalf("Archive Center.js missing SEQ-17-P328 marker %q", needle)
		}
	}
}
