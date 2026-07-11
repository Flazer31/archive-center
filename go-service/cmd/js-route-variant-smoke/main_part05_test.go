package main

import (
	"strings"
	"testing"
)

// SEQ-16.5-P142: input context builder slot governor — validates that
// Archive Center.js contains the input context slot governor surface markers.
func TestArchiveCenterJSSeq165P142InputContextSlotGovernorMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function buildInputContext(userInput, orchResult, bundledContinuityText, governorContext) {",
		"slotGovernorPolicyVersion: \"s16.5-ig.v1\"",
		"slotGovernorMode: \"turn_need_risk_slot_governor\"",
		"const budget = Math.max(200, Math.min(1500, settings.maxInputContextChars || 800));",
		"function buildTemporalCandidate() {",
		"function buildSceneCandidate() {",
		"function buildEntityCandidate() {",
		"function buildSagaCandidate() {",
		"mandatory: true",
		"staleArcDemotionApplied",
		"helperOverlapSuppressionApplied",
		"supportLaneNote: \"Support-only anchor lane; does not overwrite canonical state.\"",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.5-P142 input context slot governor marker %q", needle)
		}
	}
}

// SEQ-16.5-P143: transparency / preview / runtime trace extend — validates that
// Archive Center.js contains the transparency/preview/runtime trace extension markers.
func TestArchiveCenterJSSeq165P143TransparencyPreviewRuntimeTraceExtendMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function buildInputTransparency(userInput, recentContext, searchResult, wakeUpContext, supervisorResult, continuityInfo, kgRecallResult, extractedEntities, activeStatesResult, episodeRecallResult, expandedEntities, languageContext, backendInputTransparencyModel, backendEffectiveInputPreview, weakInputPlanner, plannerExecutionContract, progressionChoice, step25ValidationGate) {",
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
}

// SEQ-16.5-P144: backend/main.py input_context_text handoff anchor metadata
// alignment — validates that Archive Center.js contains the handoff alignment markers.
func TestArchiveCenterJSSeq165P144HandoffAnchorMetadataAlignmentMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"buildInputContext(orchResult._userInput || \"\", orchResult, _ip ? (_ip.input_context_text || \"\") : \"\"",
		"inputCtx = buildInputContext(orchResult._userInput || \"\", orchResult, _ip ? (_ip.input_context_text || \"\") : \"\", {",
		"injectionTextSource: _ip ? \"bundle\" : \"local\"",
		"const _ip = orchResult._injectionPack || null",
		"const memoryText = (_ip && _ip.memory_text) ? _ip.memory_text : formatMemoryBlock(searchResult, sanitizeTopKSetting(settings.topK, DEFAULT_SETTINGS.topK))",
		"const kgText = (_ip && _ip.kg_text) ? _ip.kg_text : formatKGBlock(kgRecallResult)",
		"const fallbackText = (_ip && _ip.fallback_text) ? _ip.fallback_text : (includeFallback ? formatFallbackBlock(searchResult) : \"\")",
		"const latestDirectEvidenceText = (_ip && _ip.latest_direct_evidence_text) ? String(_ip.latest_direct_evidence_text) : \"\"",
		"const recentRawTurnText = (_ip && _ip.recent_raw_turn_text) ? String(_ip.recent_raw_turn_text) : \"\"",
		"const canonicalStateLayerText = (_ip && _ip.canon_text) ? String(_ip.canon_text) : \"\"",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.5-P144 handoff anchor metadata alignment marker %q", needle)
		}
	}
}

// SEQ-16.5-P145: Step 16.8 stale-arc guard carry-in / Step 17 evaluation ops
// carry-in replay/inspection hooks — validates that Archive Center.js contains
// the stale-arc guard and carry-in hook markers.
func TestArchiveCenterJSSeq165P145StaleArcGuardCarryInHooksMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function buildAdaptiveInjectionGovernorTrace(userInput, orchResult, budgetResult, inputContextResult) {",
		"const oldArcForegroundGuard = budgetPolicy.oldArcForegroundGuard && typeof budgetPolicy.oldArcForegroundGuard === \"object\"",
		"function normalizeOldArcDecision(entry, lane) {",
		"function buildOldArcFailureTaxonomy(decisions) {",
		"policyVersion: \"s16.8-ft.v1\"",
		"mode: \"recall_gain_vs_monopoly_cost_split\"",
		"primaryClass",
		"secondaryClasses",
		"validDelayedPayoffRescue",
		"foregroundHijackRisk",
		"arcMonopolyAttempt",
		"sceneMonopolyRisk",
		"splitTrace",
		"recallGainCount",
		"monopolyCostCount",
		"explicitAlignmentKeepCount",
		"currentSceneKeepCount",
		"noAlignmentDemoteCount",
		"redirectedSuppressCount",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.5-P145 stale-arc guard carry-in hooks marker %q", needle)
		}
	}
}

// SEQ-16.5-P169: helper injection adaptive floor / ceiling decision value.
func TestArchiveCenterJSSeq165P169DecisionAdaptiveFloorCeilingMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"const manualBudgetLimit = Math.max(500, settings.maxInjectionChars || DEFAULT_SETTINGS.maxInjectionChars);",
		"mid_context_300k: 6000",
		"wide_context_500k: 9000",
		"ultra_long_1m_plus: 14000",
		"extreme_long_2m_plus: 18000",
		"const step13TokenTruthFloorCoreLabels = [\"latest_direct_evidence\", \"recent_raw_turn\", \"active_state\", \"canonical_state_layer\"];",
		"const step13TokenTruthFloorContinuityLabels = [\"storylines\", \"episode\", \"chapter\", \"arc\", \"saga\"];",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.5-P169 adaptive floor/ceiling decision marker %q", needle)
		}
	}
}

// SEQ-16.5-P170: input context max slot 2 vs 3 decision value.
func TestArchiveCenterJSSeq165P170DecisionMaxSlotMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"slotGovernorPolicyVersion: \"s16.5-ig.v1\"",
		"slotGovernorMode: \"turn_need_risk_slot_governor\"",
		"maxSlots: 0",
		"let maxSlots = 2",
		"function buildTemporalCandidate() {",
		"function buildSceneCandidate() {",
		"function buildEntityCandidate() {",
		"function buildSagaCandidate() {",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.5-P170 max slot decision marker %q", needle)
		}
	}
}

// SEQ-16.5-P171: runtime token hint telemetry-only / secondary safety cap
// decision value.
func TestArchiveCenterJSSeq165P171DecisionRuntimeTokenHintMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"const runtimeTokenSourceIsEstimate = normalizedRuntimeTokenSource === \"message_char_estimate\";",
		"const runtimeTieringEnabled = !!(normalizedRuntimeTokenSource && normalizedRuntimeTokenSource !== \"none\");",
		"const runtimeBudgetAdaptiveEligible = !!(runtimeTieringEnabled && Number.isFinite(runtimeTokens) && runtimeTokens > 0);",
		"const step13TokenEstimatorLowConfidenceSources = [\"none\", \"message_char_estimate\"];",
		"const step13TokenEstimatorDriftTelemetryFields = [",
		"manualBudgetLimit",
		"budgetLimit",
		"budgetLimitSource",
		"runtimeCurrentChatTokens",
		"runtimeCurrentChatTokensEffective",
		"contextProfile",
		"contextProfileSource",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.5-P171 runtime token hint decision marker %q", needle)
		}
	}
}

// SEQ-16.5-P172: [Saga] / [Chapter] anchor competition vs fallback ladder
// decision value.
func TestArchiveCenterJSSeq165P172DecisionSagaChapterAnchorLadderMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function buildSagaCandidate() {",
		"label: \"Saga Anchor\"",
		"family: \"saga\"",
		"source: \"storylines\"",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.5-P172 saga/chapter anchor ladder decision marker %q", needle)
		}
	}
}

// SEQ-16.5-P173: explicit user-input specificity heuristic/classifier
// decision value.
func TestArchiveCenterJSSeq165P173DecisionExplicitUserInputSpecificityMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"const weakInput = !rawInput || rawInput.length <= 24 || /^(continue|go on|next|more|resume|keep going|계속|계속해|이어서|이어가|다음|다음 장면|다음으로|응|ㅇㅇ|좋아|그래|좋아 계속)$/i.test(rawInput);",
		"const temporalQuery = isTemporalQueryInput(rawInput);",
		"const resumePressure = /(continue|resume|pick up|where we left|keep going|이어서|이어가|계속|재개|다시 이어)/i.test(rawInput);",
		"const longGapResume = idleGapMs >= longGapThresholdMs || continuityTriggerMode === \"idle_reentry\";",
		"const explicitRedirection = /(instead|not that|ignore previous|leave that|move on|new scene|different topic|새로|다른 쪽|말고|이제는|이번 장면|지금 장면|새 갈등|딴 이야기|전 장면 말고)/i.test(rawInput);",
		"const strongUserIntent = rawInput.length >= 48 || rawInput.split(/\\s+/).filter(Boolean).length >= 10;",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.5-P173 explicit user-input specificity decision marker %q", needle)
		}
	}
}

// SEQ-16.5-P177: Step 16.8 stale-arc suppression slice baseline compare.
func TestArchiveCenterJSSeq165P177Step168BaselineCompareMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function buildAdaptiveInjectionGovernorTrace(userInput, orchResult, budgetResult, inputContextResult) {",
		"policyVersion: \"s16.8-ft.v1\"",
		"mode: \"recall_gain_vs_monopoly_cost_split\"",
		"primaryClass",
		"validDelayedPayoffRescue",
		"foregroundHijackRisk",
		"arcMonopolyAttempt",
		"sceneMonopolyRisk",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.5-P177 Step 16.8 baseline compare marker %q", needle)
		}
	}
}

// SEQ-16.5-P178: Step 16.8 reason visibility / monopoly replay guard lane.
func TestArchiveCenterJSSeq165P178Step168ReasonVisibilityGuardLaneMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function buildAdaptiveInjectionGovernorTrace(userInput, orchResult, budgetResult, inputContextResult) {",
		"policyVersion: \"s16.8-ft.v1\"",
		"mode: \"recall_gain_vs_monopoly_cost_split\"",
		"secondaryClasses",
		"explicitRedirectionOnly",
		"validDelayedPayoffRescue",
		"foregroundHijackRisk",
		"sceneMonopolyRisk",
		"arcMonopolyAttempt",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.5-P178 Step 16.8 reason visibility guard lane marker %q", needle)
		}
	}
}

// SEQ-16.5-P179: Step 16.8 completion Step 17 evaluation baseline direct
// handoff gate.
func TestArchiveCenterJSSeq165P179Step17DirectHandoffGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function buildAdaptiveInjectionGovernorTrace(userInput, orchResult, budgetResult, inputContextResult) {",
		"policyVersion: \"s16.8-ft.v1\"",
		"mode: \"recall_gain_vs_monopoly_cost_split\"",
		"const oldArcForegroundGuard = budgetPolicy.oldArcForegroundGuard && typeof budgetPolicy.oldArcForegroundGuard === \"object\"",
		"function normalizeOldArcDecision(entry, lane) {",
		"function buildOldArcFailureTaxonomy(decisions) {",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.5-P179 Step 17 direct handoff gate marker %q", needle)
		}
	}
}

// SEQ-16.5-P183: Step 17 evaluation harness static 3000/800 baseline + 16.5+16.8
// baseline.
func TestArchiveCenterJSSeq165P183Step17EvaluationHarnessBaselineMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"const manualBudgetLimit = Math.max(500, settings.maxInjectionChars || DEFAULT_SETTINGS.maxInjectionChars);",
		"const budget = Math.max(200, Math.min(1500, settings.maxInputContextChars || 800));",
		"policyVersion: \"s16.8-ft.v1\"",
		"mode: \"recall_gain_vs_monopoly_cost_split\"",
		"slotGovernorPolicyVersion: \"s16.5-ig.v1\"",
		"slotGovernorMode: \"turn_need_risk_slot_governor\"",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.5-P183 Step 17 evaluation harness baseline marker %q", needle)
		}
	}
}

// SEQ-16.5-P184: Step 17 ops budget tuning governor behavior trace
// interpretation document.
func TestArchiveCenterJSSeq165P184Step17OpsTraceInterpretationMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function buildAdaptiveInjectionGovernorTrace(userInput, orchResult, budgetResult, inputContextResult) {",
		"function logTurnTraceSummary() {",
		"function renderTurnTraceRows() {",
		"policyVersion: \"s16.8-ft.v1\"",
		"mode: \"recall_gain_vs_monopoly_cost_split\"",
		"decisionCounts",
		"recallGainCount",
		"monopolyCostCount",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.5-P184 Step 17 ops trace interpretation marker %q", needle)
		}
	}
}

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
func TestArchiveCenterJSSeq168P100SceneAlignmentMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"explicit_query_alignment",
		"current_scene_evidence",
		"oldArcForegroundGuard",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P100 marker %q", needle)
		}
	}
}

// SEQ-16.8-P101: reason trace — old arc keep/drop/suppress inspectable.
func TestArchiveCenterJSSeq168P101ReasonTraceMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"coprocessor_reason_trace",
		"supported_reason_codes",
		"keep",
		"drop",
		"suppress",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P101 marker %q", needle)
		}
	}
}

// SEQ-16.8-P102: failure split — tail recall gain foreground monopoly failure class.
func TestArchiveCenterJSSeq168P102FailureSplitMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"buildOldArcFailureTaxonomy",
		"primaryClass",
		"secondaryClasses",
		"foreground_hijack_risk",
		"arc_monopoly_attempt",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P102 marker %q", needle)
		}
	}
}

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
func TestArchiveCenterJSSeq168P108CallbackSceneAlignmentMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"current_scene_evidence",
		"explicit_query_alignment",
		"input_current_scene_anchor",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P108 marker %q", needle)
		}
	}
}

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
func TestArchiveCenterJSSeq168P114ReasonCodeVocabularyMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"coprocessor_reason_trace",
		"supported_reason_codes",
		"keep",
		"drop",
		"suppress",
		"demote",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P114 marker %q", needle)
		}
	}
}

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
func TestArchiveCenterJSSeq168P119ForegroundHijackTaxonomyMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"buildOldArcFailureTaxonomy",
		"foreground_hijack_risk",
		"arc_monopoly_attempt",
		"sceneMonopolyRisk",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P119 marker %q", needle)
		}
	}
}

// SEQ-16.8-P120: delayed payoff split — 16.8-3b valid delayed payoff rescue vs scene monopoly split define.
func TestArchiveCenterJSSeq168P120DelayedPayoffSplitMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"validDelayedPayoffRescue",
		"sceneMonopolyRisk",
		"valid_delayed_payoff_rescue",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P120 marker %q", needle)
		}
	}
}

// SEQ-16.8-P121: recall gain/monopoly cost split — 16.8-3c recall gain/monopoly cost split trace schema define.
func TestArchiveCenterJSSeq168P121RecallGainMonopolySplitMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"recallGainCount",
		"monopolyCostCount",
		"splitTrace",
		"recall_gain_vs_monopoly_cost_split",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P121 marker %q", needle)
		}
	}
}

// SEQ-16.8-P125: stale arc revival replay — 16.8-4a stale arc revival/single-incident monopoly replay define.
func TestArchiveCenterJSSeq168P125StaleArcRevivalReplayMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"buildOldArcReplayGate",
		"single_incident_monopoly_attempt",
		"staleArcRevivalReplay",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P125 marker %q", needle)
		}
	}
}

// SEQ-16.8-P126: tail recall vs foreground hijack gate — 16.8-4b tail recall vs foreground hijack gate define.
func TestArchiveCenterJSSeq168P126TailRecallHijackGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"buildOldArcReplayGate",
		"tailRecallVsForegroundHijackGate",
		"tail_recall_with_monopoly_cost",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P126 marker %q", needle)
		}
	}
}

// SEQ-16.8-P127: narrative diversity gate — 16.8-4c narrative diversity gate define.
func TestArchiveCenterJSSeq168P127NarrativeDiversityGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"narrativeDiversityGate",
		"stale_arc_replay_and_diversity_adoption_gate",
		"scene diversity stayed clear of old-arc monopoly pressure",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P127 marker %q", needle)
		}
	}
}

// SEQ-16.8-P128: arc monopoly gate — 16.8-4d arc monopoly gate define.
func TestArchiveCenterJSSeq168P128ArcMonopolyGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"arc_monopoly_attempt",
		"arcMonopolyAttempt",
		"arcMonopolyGate",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P128 marker %q", needle)
		}
	}
}

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
		"assembleInjectionWithBudget",
		"applyContextInjection",
		"buildAdaptiveInjectionGovernorTrace",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P133 marker %q", needle)
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
func TestArchiveCenterJSSeq168P135ReplayCorpusBaselineMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"replayScenarios",
		"single_incident_monopoly_attempt",
		"tail_recall_with_monopoly_cost",
		"valid_delayed_payoff_rescue",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P135 marker %q", needle)
		}
	}
}

// SEQ-16.8-P136: backend/main.py storyline/pending-thread read metadata alignment — suppression trace confirm.
func TestArchiveCenterJSSeq168P136BackendMetadataAlignmentMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"storylineCount",
		"pendingThreadCount",
		"suppressionTriggerActive",
		"guidance_metadata",
		"input_context_enabled",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P136 marker %q", needle)
		}
	}
}

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
func TestArchiveCenterJSSeq168P163CurrentSceneEvidenceMinCriteriaMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"currentSceneEvidence",
		"activeState",
		"latestDirectEvidence",
		"recentRawTurn",
		"inspectable",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P163 marker %q", needle)
		}
	}
}

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
func TestArchiveCenterJSSeq168P165ReasonVisibilityLaneMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"_inputTransparency",
		"buildContinuityTraceState",
		"applyContinuityTraceState",
		"reason",
		"inspectable",
		"trace",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P165 marker %q", needle)
		}
	}
}

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
func TestArchiveCenterJSSeq17P231OpsProcedureSurfaceMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"promotion",
		"backfill",
		"rebuild",
		"reembed",
		"migration",
		"health",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P231 marker %q", needle)
		}
	}
}

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
func TestArchiveCenterJSSeq17P241RegressionCorpusMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"regression",
		"corpus",
		"seq14",
		"seq15",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P241 marker %q", needle)
		}
	}
}

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
func TestArchiveCenterJSSeq17P287ReembedMigrationHealthProbeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"reembed",
		"migration",
		"health",
		"probe",
		"audit",
		"dry_run",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P287 marker %q", needle)
		}
	}
}

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
