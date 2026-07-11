package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveCenterJSSameTurnOverlayInjectionAndTraceRuntime(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for JS runtime behavior smoke")
	}
	src := readArchiveCenterJS(t)
	helpersStart := strings.Index(src, "function normalizeStorylineStatus")
	helpersEnd := strings.Index(src, "// E-1d: Storyline Sync")
	if helpersStart < 0 || helpersEnd < 0 || helpersEnd <= helpersStart {
		t.Fatalf("Archive Center.js missing same-turn overlay helper block")
	}

	script := src[helpersStart:helpersEnd] + "\n" +
		extractJSFunctionBlockForTest(t, src, "function formatStorylineBlock") + "\n" +
		extractJSFunctionBlockForTest(t, src, "function formatWorldRulesBlock") + "\n" +
		extractJSFunctionBlockForTest(t, src, "function renderTurnTraceRows") + `
const assert = (cond, msg) => { if (!cond) throw new Error(msg); };
const t = (key) => key;
const statusDotClass = (st) => "dot-" + st;
const escapeAttr = (value) => String(value == null ? "" : value);
const truncPreview = (value, n) => String(value == null ? "" : value).slice(0, n);
let lastTurnTrace = null;

const supervisor = {
  directive: {
    storylines: [{
      name: "Rooftop Promise",
      status: "active",
      current_context: "same-turn confession must affect the immediate injection",
      key_points: ["answer is pending"],
      ongoing_tensions: ["hesitation"]
    }],
    section_world: {
      applies: true,
      genre_hint: "romance",
      rules: ["The rooftop is private tonight."],
      world_rules: ["Confessions leave visible social consequences."]
    }
  }
};
const storylineOverlay = buildStorylineOverlay(supervisor);
const worldRuleOverlay = buildWorldRuleOverlay(supervisor);
const storylines = mergeStorylineOverlay({
  items: [{ name: "Existing Arc", status: "active", current_context: "older active row", last_turn: 4 }],
  count: 1
}, storylineOverlay);
const worldRules = mergeWorldRuleOverlay({ items: [], count: 0 }, worldRuleOverlay);
const storylineText = formatStorylineBlock(storylines);
const worldRulesText = formatWorldRulesBlock(worldRules);
assert(storylineText.includes("Rooftop Promise"), "same-turn storyline was not included in injection text");
assert(storylineText.includes("same-turn confession"), "same-turn storyline context missing from injection text");
assert(worldRulesText.includes("The rooftop is private tonight."), "same-turn world rule missing from injection text");
assert(worldRulesText.includes("Confessions leave visible social consequences."), "world_rules fallback missing from injection text");

lastTurnTrace = {
  turnIndex: 10,
  contextSize: 4,
  chatSessionId: "char_1",
  search: { status: "ok", itemCount: 0, memoryCount: 0, fallbackCount: 0, paths: [] },
  wakeUpContext: { status: "empty", length: 0 },
  kgRecall: { status: "skipped", triplesReturned: 0, entitiesExtracted: 0 },
  activeStates: { status: "empty", count: 0, types: [] },
  supervisor: { status: "ok", hasDirective: true, hasAuthor: true, hasDirector: true, hasSectionWorld: true },
  storylines: { status: "ok", count: 2, activeCount: 2, usedOverlay: true, overlayCount: 1, freshness: { latestTurn: 10 }, selection: { totalActiveCount: 3, selectedCount: 2, droppedCount: 1, staleDroppedCount: 1 } },
  worldRules: { status: "ok", count: 2, usedOverlay: true, overlayCount: 2, freshness: { latestTurn: 10 } },
  pendingThreads: { status: "empty", count: 0 },
  save: { status: "ok" },
  complete: { status: "ok" },
  critic: { memorySaved: true, kgSaved: true }
};
let html = renderTurnTraceRows();
assert(html.includes("directive") && html.includes("author") && html.includes("director") && html.includes("sw"), "supervisor directive trace missing");
assert(html.includes("2 total") && html.includes("2 active") && html.includes("overlay+1"), "storyline overlay trace missing");
assert(html.includes("sel:2/3") && html.includes("drop:1") && html.includes("staleDrop:1"), "storyline selection trace missing");
assert(html.includes("2 rules") && html.includes("overlay+2"), "world-rule overlay trace missing");

lastTurnTrace = {
  turnIndex: 11,
  contextSize: 5,
  chatSessionId: "char_1",
  search: { status: "ok", itemCount: 0, memoryCount: 0, fallbackCount: 0, paths: [] },
  wakeUpContext: { status: "empty", length: 0 },
  kgRecall: { status: "skipped", triplesReturned: 0, entitiesExtracted: 0 },
  activeStates: { status: "ok", count: 3, types: ["relationship", "promise", "scene"] },
  supervisor: { status: "ok", hasDirective: true, hasAuthor: true, hasDirector: true, hasSectionWorld: true },
  storylines: { status: "ok", count: 3, activeCount: 3, usedOverlay: true, overlayCount: 1, freshness: { latestTurn: 11 } },
  worldRules: { status: "ok", count: 2, usedOverlay: false, overlayCount: 0, freshness: { latestTurn: 10 } },
  pendingThreads: { status: "empty", count: 0 },
  save: { status: "ok" },
  complete: { status: "ok" },
  critic: { memorySaved: true, kgSaved: true }
};
html = renderTurnTraceRows();
assert(html.includes("3 states (relationship, promise, scene)"), "active-state next-turn trace missing");
assert(html.includes("3 total") && html.includes("3 active") && html.includes("overlay+1"), "next-turn storyline trace missing");

lastTurnTrace = {
  turnIndex: 12,
  contextSize: 3,
  chatSessionId: "char_1",
  search: { status: "fail", itemCount: 0, memoryCount: 0, fallbackCount: 0, paths: [] },
  wakeUpContext: { status: "fail", length: 0 },
  kgRecall: { status: "skipped", triplesReturned: 0, entitiesExtracted: 0 },
  activeStates: { status: "empty", count: 0, types: [] },
  supervisor: { status: "fail", hasDirective: false, hasAuthor: false, hasDirector: false, hasSectionWorld: false },
  storylines: { status: "empty", count: 0, activeCount: 0, usedOverlay: false, overlayCount: 0, freshness: { mode: "empty" } },
  worldRules: { status: "empty", count: 0, usedOverlay: false, overlayCount: 0, freshness: { mode: "empty" } },
  pendingThreads: { status: "skip", count: 0 },
  save: { status: "skipped" },
  complete: { status: "skipped" },
  critic: { memorySaved: false, kgSaved: false }
};
html = renderTurnTraceRows();
assert(html.includes("Supervisor") && html.includes("fail"), "backend-off supervisor fallback missing");
assert(html.includes("Storylines") && html.includes("no storylines"), "backend-off storyline fallback missing");
assert(html.includes("World Rules") && html.includes("no rules"), "backend-off world-rule fallback missing");
console.log(JSON.stringify({ storylineText, worldRulesText, ok: true }));
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node same-turn injection/trace runtime smoke failed: %v\n%s", err, out)
	}
}

func TestArchiveCenterJSSameTurnOverlaySyncFailureIsolationMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`postStorylineSync(supervisorResult, chatSessionId, predictedTurnIndex, "apply").catch`,
		"const predictedTurnIndex = peekNextTurnIndex(chatSessionId);",
		"postWorldRulesSync(supervisorResult, chatSessionId, predictedTurnIndex).catch",
		"storylineResult = mergeStorylineOverlay(storylineBaseResult, storylineOverlay);",
		"worldRulesResult = mergeWorldRuleOverlay(worldRulesBaseResult, worldRuleOverlay);",
		"storylineSelectionRaw: supervisorResult && supervisorResult.storyline_selection",
		"usedOverlay: !!storylineResult.usedOverlay",
		"usedOverlay: !!worldRulesResult.usedOverlay",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing same-turn sync isolation marker %q", needle)
		}
	}
}

func TestArchiveCenterJSPromptEnglishOnlyBoundaryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	forbidden := []string{
		"function tp(",
		"tp(",
		"promptLanguage",
		"settings.label.promptLanguage",
		"settings.desc.promptLanguage",
		`"prompt.`,
	}
	for _, needle := range forbidden {
		if strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js should not contain F-3 retired prompt-language marker %q", needle)
		}
	}

	root := filepath.Join(archiveCenterRoot(t), "prompts")
	for _, dir := range []string{"ko", "ja", "en"} {
		if _, err := os.Stat(filepath.Join(root, dir)); !os.IsNotExist(err) {
			t.Fatalf("prompt language dir %q should not exist under root prompts", dir)
		}
	}
	for _, name := range []string{"supervisor_system.txt", "critic_system.txt"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatalf("root prompt file %q missing: %v", name, err)
		}
	}
}

func TestArchiveCenterJSSeq04SessionTurnTransitionProof(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"const _sessionTurnIndices = new Map()",
		"const SESSION_TURN_MAP_MAX = 50",
		"function getSessionTurnIndex(sessionId)",
		"function setSessionTurnIndex(sessionId, idx)",
		"const idx = Math.max(loadTurnCounter(sessionId), getSessionTurnIndex(sessionId), 0) + 1",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-04 session turn marker %q", needle)
		}
	}

	stored := map[string]int{}
	tracked := map[string]int{}
	next := func(sessionID string) int {
		max := stored[sessionID]
		if tracked[sessionID] > max {
			max = tracked[sessionID]
		}
		idx := max + 1
		stored[sessionID] = idx
		return idx
	}
	track := func(sessionID string, idx int) {
		tracked[sessionID] = idx
	}

	track("char_1_cid_A", 15)
	if got := next("char_2_cid_B"); got != 1 {
		t.Fatalf("new session B next turn = %d, want 1", got)
	}
	track("char_2_cid_B", 1)
	if got := next("char_1_cid_A"); got != 16 {
		t.Fatalf("returning session A next turn = %d, want 16", got)
	}
	if got := next("char_2_cid_B"); got != 2 {
		t.Fatalf("session B second turn = %d, want 2", got)
	}
}

func TestArchiveCenterJSFinalPayloadParityMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function buildFinalPayloadParityTrace(originalPayload, finalPayload, meta)",
		"function attachFinalPayloadParityTrace(trace, originalPayload, finalPayload, meta)",
		`source: "js_host_adapter"`,
		"finalPayloadParity",
		"SEQ-02 / RMG-22 Final Payload Parity",
		"payloadMutated",
		"beforeMessageCount",
		"afterMessageCount",
		"finalUserInputPreview",
		"assembledPreview",
		"attachFinalPayloadParityTrace(lastOrchResult && lastOrchResult._trace, payload, outgoingPayload",
		"attachFinalPayloadParityTrace(lastOrchResult && lastOrchResult._trace, payload, payload",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing final payload parity marker %q", needle)
		}
	}
}

func TestArchiveCenterJSW1RewriteLastUserMessage(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function rewriteLastUserMessage(payload, nextUserInput)",
		"empty_next_input",
		"no_messages",
		"no_user_message",
		"unchanged",
		"rewritten",
		"exception",
		"lastUserIdx",
		"content: rewrittenText",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing W-1 rewrite helper marker %q", needle)
		}
	}
}

func TestArchiveCenterJSW1FinalPayloadParityRewriteEvidence(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"attachFinalPayloadParityTrace(lastOrchResult && lastOrchResult._trace, payload, outgoingPayload",
		"attachFinalPayloadParityTrace(lastOrchResult && lastOrchResult._trace, payload, payload",
		"payloadMutated",
		"finalUserInputPreview",
		"inputImprovementApplied",
		"rewriteAllowed",
		"lastOrchResult._trace.applyMode.payloadReplaced = true",
		"lastOrchResult._trace.applyMode.rewriteReason = rewriteResult.reason || \"rewritten\"",
		"lastOrchResult._trace.inputImprovement.applyReason = \"payload_rewritten\"",
		"lastOrchResult._trace.inputImprovement.applyReason = \"rewrite_failed:\"",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing W-1 final payload parity rewrite marker %q", needle)
		}
	}
}

func TestArchiveCenterJSJ3ApplyModeGateAndTraceRecord(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function getApplyModeGate()",
		"function applyModeAllowsApply(verdict)",
		"if (mode !== \"reviewed_apply\") return false;",
		"verdict === \"approve\" || verdict === \"partial\" || verdict === \"first-pass-only\"",
		"trace.applyMode = { mode: _applyGate.mode, payloadReplaced: false }",
		"trace.inputImprovement.applyReason = \"improvement_accepted\"",
		"trace.inputImprovement.payloadRewritten = false",
		"trace.applyMode.payloadReplaced = false",
		"settings.pluginMainApplyMode",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing J-3 apply-mode marker %q", needle)
		}
	}
}

func TestArchiveCenterJSJ3TraceBlocksAndFailureSafety(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"mode_not_reviewed_apply",
		"not_improved",
		"empty_final_input",
		"verdict_not_allowed:",
		"trace_only",
		"no_apply",
		"rewrite_failed:",
		"payloadReplaced = false",
		"payloadRewritten = false",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing J-3 failure-safety marker %q", needle)
		}
	}
}

func TestArchiveCenterJSJ4TraceRecordFormat(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function buildImprovementTraceRecord(applyGate, mergeResult, originalInput, payloadReplaced)",
		"keepCandidates",
		"dropCandidates",
		"previewBefore",
		"previewAfter",
		"mergeNote",
		"recordedAt",
		"async function tryCompleteTurn(turnIdx, userInput, assistantContent, contextMessages, chatSessionId, improvementTrace, prebuiltBody)",
		"trace.improvementHandoff = {",
		"hintAttached:",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing J-4 trace record marker %q", needle)
		}
	}
}

func TestArchiveCenterJSPrepareTurnInjectionPackMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function applyContextInjection(payload, orchResult)",
		"const _ip = orchResult._injectionPack || null",
		"const memoryText = (_ip && _ip.memory_text) ? _ip.memory_text : formatMemoryBlock(searchResult, sanitizeTopKSetting(settings.topK, DEFAULT_SETTINGS.topK))",
		"const kgText = (_ip && _ip.kg_text) ? _ip.kg_text : formatKGBlock(kgRecallResult)",
		"const fallbackText = (_ip && _ip.fallback_text) ? _ip.fallback_text : (includeFallback ? formatFallbackBlock(searchResult) : \"\")",
		"const latestDirectEvidenceText = (_ip && _ip.latest_direct_evidence_text) ? String(_ip.latest_direct_evidence_text) : \"\"",
		"const recentRawTurnText = (_ip && _ip.recent_raw_turn_text) ? String(_ip.recent_raw_turn_text) : \"\"",
		"const canonicalStateLayerText = (_ip && _ip.canon_text) ? String(_ip.canon_text) : \"\"",
		"assembleInjectionWithBudget(",
		"injectionTextSource: _ip ? \"bundle\" : \"local\"",
		"buildInputContext(orchResult._userInput || \"\", orchResult, _ip ? (_ip.input_context_text || \"\") : \"\"",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing prepare-turn injection pack marker %q", needle)
		}
	}
}

func TestArchiveCenterJSStep11HybridMemoryPolicyMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const precedencePolicyVersion = "ea1a.v1";`,
		`"verified_direct_evidence",`,
		`"canonical_state",`,
		`"dense_summary",`,
		`"retrieval_supporting_inference",`,
		`role: "detail_recall_audit_only",`,
		"supporting_guidance_guard: {",
		"narrative_quality_layer: {",
		`status: "quality_hint_only",`,
		`"truth_arbitration"`,
		`"canonical_overwrite"`,
		`const hybridPolicyVersion = "ea1f.v1";`,
		`const recentPriorityPolicyVersion = "ea1g.v1";`,
		"hybridPolicyVersion: hybridPolicyVersion,",
		"recentPriorityPolicyVersion: recentPriorityPolicyVersion,",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing Step 11 hybrid memory marker %q", needle)
		}
	}
}

func TestArchiveCenterJSHypaMemoryImportOnlyMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"async function importHypaMemory(sessionId)",
		"function normalizeHypaImportSourceTurnIndex",
		`"/import/hypamemory"`,
		"hypaV3Data",
		"source_turn_index: normalizeHypaImportSourceTurnIndex(s, idx)",
		"Please disable RisuAI's HypaMemory after import.",
		`const hypaLoreIngestPolicyVersion = "rg1d.v1";`,
		`const hypaLorePolicyTag = "rg1d_ingest_only";`,
		`const legacyAlwaysOnSourceLabels = ["wake_up", "lorebook", "hypamemory"];`,
		"hypaLoreIngestOnlyEnabled: true,",
		`hypaLoreAlwaysOnMode: "discouraged",`,
		"hypaLoreAlwaysOnSourceLabels: legacyAlwaysOnSourceLabels.slice(),",
		`"ingest_only_not_injected"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing HypaMemory ingest-only marker %q", needle)
		}
	}
}

func TestArchiveCenterJSEA1PrecedenceAndHybridBudgetMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const precedencePolicyVersion = "ea1a.v1";`,
		`"current_user_input",`,
		`"explicit_correction",`,
		`"hard_rule",`,
		`"verified_direct_evidence",`,
		`"canonical_state",`,
		`"dense_summary",`,
		`"retrieval_supporting_inference",`,
		`const hybridPolicyVersion = "ea1f.v1";`,
		`const allocationStrategy = "priority_then_residual_ratio";`,
		"const hardFloorShareCap = 0.55;",
		"latestDirectEvidencePriorityEnabled: hasLatestDirectEvidence,",
		"recentRawTurnPriorityEnabled: hasRecentRawTurn,",
		"hardFloorShareCap: hardFloorShareCap,",
		"hardFloorReserveTotalChars: hardFloorReserveTotalChars,",
		"residualSupportBudgetChars: Math.max(0, budgetLimit - hardFloorReserveTotalChars),",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing EA-1 precedence/hybrid budget marker %q", needle)
		}
	}
}

func TestArchiveCenterJSPromptTemplateScaffoldSanitizeRuntime(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function looksLikePromptTemplateScaffold(text)",
		"function isPromptTemplateScaffoldLine(line)",
		"function sanitizeForCritic(text)",
		"response template|template guidelines",
		"return !isPromptTemplateScaffoldLine(line);",
		`if (looksLikePromptTemplateScaffold(clean)) return "";`,
		`stage.scaffoldLineDetected ? "template-scaffold" : null`,
		"scaffoldLineDetected,",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing prompt-template scaffold sanitize marker %q", needle)
		}
	}
}

func TestArchiveCenterJSDenseSummaryProfileBudgetMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const denseSummarySlotPolicyVersion = "ds1e.v1";`,
		"const denseSummarySlotProfile = (function resolveDenseSummarySlotProfile()",
		"const denseSummarySlotRatios = (function resolveDenseSummarySlotRatios()",
		`return { episode: 0.045, chapter: 0.04, arc: 0.035, saga: 0.03 };`,
		"denseSummarySlotPolicyVersion: denseSummarySlotPolicyVersion,",
		"denseSummarySlotProfile: denseSummarySlotProfile,",
		"denseSummarySlotRatios: {",
		"denseSummaryHardFloorChars: {",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing dense-summary profile budget marker %q", needle)
		}
	}
}

func TestArchiveCenterJSRG1aRetrievalRoleAuditMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const retrievalRolePolicyVersion = "rg1a.v1";`,
		`const retrievalRolePolicyTag = "rg1a_retrieval_audit_only";`,
		`const retrievalAllowedUsage = ["detail_recall", "audit_reference"];`,
		`const retrievalDisallowedUsage = ["truth_overwrite", "canonical_override"];`,
		`role: "detail_recall_audit_only",`,
		`retrievalRoleMode: "detail_recall_audit_only",`,
		"retrievalAuditOnlyEnabled: true,",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing RG-1a retrieval role audit marker %q", needle)
		}
	}
}

func TestArchiveCenterJSRG1bToRG1dPromotionTraceAndIngestMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const retrievalPromotionPolicyVersion = "rg1b.v1";`,
		`const retrievalPromotionTargets = ["canonical_state", "dense_summary"];`,
		"retrievalPromotionCandidatesRaw",
		"retrievalPromotionCandidateCount",
		"retrievalPromotionCanonicalCount",
		"retrievalPromotionDenseSummaryCount",
		`const retrievalConflictPolicyVersion = "rg1c.v1";`,
		"retrievalKeepDropTraceEnabled: true,",
		"retrievalKeepDropTrace: retrievalKeepDropTrace.slice(0, 20),",
		`const hypaLoreIngestPolicyVersion = "rg1d.v1";`,
		`const hypaLorePolicyTag = "rg1d_ingest_only";`,
		`"ingest_only_not_injected"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing RG-1b/RG-1c/RG-1d marker %q", needle)
		}
	}
}

func TestArchiveCenterJSRG1fToRG1iAuthorityGuardMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const verifiedCurrentStatePolicyVersion = verifiedCurrentStatePrecedenceEnabled ? "rg1f.v1" : "";`,
		`const verifiedCurrentStatePolicyTag = verifiedCurrentStatePrecedenceEnabled ? "rg1f_verified_current_state" : "";`,
		"verifiedCurrentStatePrecedenceEnabled: verifiedCurrentStatePrecedenceEnabled,",
		`const reliabilityGuardPolicyVersion = "rg1g.v1";`,
		`const reliabilityGuardPolicyTag = "rg1g_conservative_hold";`,
		`reliabilityGuardMode: reliabilityGuardTriggered ? "conservative_hold" : "normal",`,
		`const supportingGuidanceGuardPolicyVersion = "rg1h.v1";`,
		`const supportingGuidanceGuardPolicyTag = "rg1h_supporting_guidance_guard";`,
		"supportingGuidanceEvidenceCeilingEnabled: supportingGuidanceEvidenceCeilingEnabled,",
		`const narrativeQualityLayerPolicyVersion = "rg1i.v1";`,
		`const narrativeQualityLayerMode = "quality_hint_only";`,
		"narrativeQualityLayerTruthArbitrationAllowed: false,",
		"narrativeQualityLayerCanonicalOverwriteAllowed: false,",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing RG-1f/RG-1g/RG-1h/RG-1i marker %q", needle)
		}
	}
}

func TestArchiveCenterJSRG1jPresetTemplateContaminationMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function looksLikePromptTemplateScaffold(text)",
		"function isPromptTemplateScaffoldLine(line)",
		"function sanitizeForCritic(text)",
		"function sanitizeNarrativeOutputForDisplay(text)",
		`"template-scaffold"`,
		"scaffoldLineDetected",
		"sanitizeForCritic(rawSeed)",
		"sanitizeForCritic(safeUserContent)",
		"sanitizeForCritic(String(assistantContent || \"\"))",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing RG-1j preset/template contamination marker %q", needle)
		}
	}
}

func TestArchiveCenterJSPluginVersionMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"//@name Archive Center",
		"//@display-name Archive Center",
		"//@version 2.5.0",
		`const VERSION = "2.5.0";`,
		`const VERSION_STR = typeof VERSION !== "undefined" ? String(VERSION) : "unknown";`,
		"source_version:    VERSION_STR",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing plugin version marker %q", needle)
		}
	}
}

func TestArchiveCenterJSRenderRegression47Markers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function renderTurnTraceRows()",
		"function renderInputTransparencySection()",
		"function renderExplorerDirectEvidence()",
		"function renderExplorerEntities()",
		"function renderExplorerWorldGraph()",
		"function renderTimelinePanel()",
		"function renderPromptEditorSection()",
		"async function renderSettingsPanel()",
		`renderItBlockRaw("3.5. Hybrid Retrieval Inspection"`,
		`renderItBlockRaw("7.5. Protection Patterns"`,
		`renderItBlockRaw("8. Context Injection (`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing render regression marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq08P703UIDetailModeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const UI_DETAIL_MODE_OPTIONS = Object.freeze(["full", "reduced_info", "status_only"])`,
		`uiDetailMode: "full"`,
		`"settings.uiDetailMode.reduced_info"`,
		`"settings.uiDetailMode.status_only"`,
		`const detailMode = sanitizeEnumValue(settings.uiDetailMode, DEFAULT_SETTINGS.uiDetailMode, UI_DETAIL_MODE_OPTIONS)`,
		`<select id="mo-uiDetailMode">`,
		`<option value="reduced_info"`,
		`<option value="status_only"`,
		`uiDetailMode: $("mo-uiDetailMode").value`,
		`$("mo-uiDetailMode").value = settings.uiDetailMode || "full"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-08-P703 UI detail mode marker %q", needle)
		}
	}
}

func TestArchiveCenterJSInitiativeControlLatestEquivalentMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`storyNarrativeStance: "balanced"`,
		"merged.storyNarrativeStance = sanitizeEnumValue(",
		"NARRATIVE_STANCE_MODES",
		`<select id="mo-storyNarrativeStance"`,
		`<option value="reactive"`,
		`<option value="balanced"`,
		`<option value="proactive"`,
		`const stanceEl = $("mo-storyNarrativeStance");`,
		`storyNarrativeStance: $("mo-storyNarrativeStance").value`,
		`$("mo-storyNarrativeStance").value = settings.storyNarrativeStance || "balanced";`,
		"function buildInitiativeModeSuffix(mode)",
		"function buildInitiativeModeBounds(mode)",
		`narrative_stance: narrativeStance`,
		`narrative_stance_suffix: initiativeSuffix || ""`,
		"narrative_stance_bounds: initiativeBounds",
		"extractNarrativeStanceSummary(_narrativeStance)",
		"initiativeSummaryRaw",
		"initiativeSuffixRaw",
		"initiativeBoundsRaw",
		`debugLog("initiative:", _initiativeSummary.mode`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing H-3 initiative latest-equivalent marker %q", needle)
		}
	}
	if strings.Contains(src, "storyInitiativeMode") {
		t.Fatal("Archive Center.js should not reintroduce retired storyInitiativeMode beside storyNarrativeStance")
	}
}

func TestArchiveCenterJSContinuityPackLatestEquivalentMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function normalizeContinuityPackResult(rawResult, chatSessionId)",
		"async function fetchContinuityPack(chatSessionId)",
		`"/continuity-pack/" + encodeURIComponent(sid)`,
		"function buildStorylineResultFromContinuityPack(continuityPackResult)",
		"function buildWorldRulesResultFromContinuityPack(continuityPackResult)",
		"function applyContinuityPackFallback(primaryResult, continuityResult, label)",
		"async function resolveContinuityTriggerInfo(userInput, messages, chatSessionId, options = {})",
		`"empty_input"`,
		`"manual_resume"`,
		`"idle_reentry"`,
		"function upgradeContinuityInfoWithPack(continuityInfo, continuityPackResult, guardContext)",
		`querySource: packQuery ? "continuity_pack"`,
		"if (cont.packUsedAsQuery) contParts.push(\"packQuery",
		"if (cont.packUsedAsWakeUp) contParts.push(\"wakeUp",
		"if (cont.debugForced) contParts.push(\"debugForced\")",
		"[DEBUG] Current Turn Input",
		"[DEBUG] Current Turn Source",
		"const _rawInputBySession = new Map();",
		"async function onInputHook(rawInput)",
		"cacheRawInputForSession(sessionId, rawInput)",
		"function resolveCurrentTurnUserInputInfo(payload, messages, sessionId)",
		`source: "input_hook"`,
		`source: "messages.meta_only"`,
		"shouldRejectLowTrustCurrentInput(auxiliaryMessageContentText(lastUserMsg.content))",
		"metaOnlyInput: !!userInputInfo.metaOnly",
		"t('settings.debug.forceIdleBtn')",
		"debugForced: true",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing H-4 continuity marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq07PersistentGuidanceMaintenanceMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"async function fetchNarrativeControl(chatSessionId)",
		`bridgeFetch("/narrative-control/" + encodeURIComponent(sessionId)`,
		"function buildPersistentGuidanceSuffix(ncResult)",
		"function buildPersistentGuidanceHintsCompact(ncResult, suppressedForbidden)",
		"function resolveAutoAdvanceTrigger(continuityInfo)",
		"function buildAutoAdvanceHint(ncResult, trigger)",
		"function fireMaintenancePass(turnIdx, chatSessionId, assistantContent, traceRef, recentResponses, supervisorResult)",
		"`/maintenance/enqueue`",
		"shadow_only: true",
		"recent_responses: Array.isArray(recentResponses)",
		"trace.guidanceState = {",
		"trace.guidanceState.suppressedCount = (_guidanceArbitration.suppressed || []).length;",
		"guidanceTransitionRaw:",
		"[DEBUG] Guidance Transition (K-4d)",
		"[DEBUG] Auto-advance Hint (L-4b)",
		"auto_advance_trigger: autoAdvanceTrigger || \"none\"",
		"autoAdvanceHintApplied: !!_autoAdvanceHint",
		"Treat this as a gentle nudge only",
		"never override explicit user input",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-07 persistent guidance/maintenance marker %q", needle)
		}
	}
	if strings.Contains(src, "await fireMaintenancePass(") {
		t.Fatal("maintenance pass must stay fire-and-forget and must not block chat flow")
	}
}

func TestArchiveCenterJSSeq08BackendTurnEngineFailOpenMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"async function tryPrepareTurn(sessionId, userInput, messages, continuityInfo, type, languageContext, options = {})",
		`bridgeFetch("/prepare-turn"`,
		`return { source: "backend-off", fallback_reason: "backend_off", status: "error" }`,
		`return { source: "backend-error", fallback_reason: "backend_error", status: "error" }`,
		"const prepareTurnBackendUnavailable = _lastPrepareTurnSource === \"backend-off\"",
		`updateRuntimeState("lastBridgeHealth", "fail"`,
		`debugLog("prepare-turn unavailable - local fallback continues:"`,
		`updateRuntimeState("lastBridgeHealth", "ok"`,
		"async function tryCompleteTurn(turnIdx, userInput, assistantContent, contextMessages, chatSessionId, improvementTrace, prebuiltBody)",
		`bridgeFetchWithRetry("/complete-turn"`,
		"function fireMaintenancePass(turnIdx, chatSessionId, assistantContent, traceRef, recentResponses, supervisorResult)",
		"`/maintenance/enqueue`",
		"trace.autonomyPlan = {",
		"trace.microBeatProposal =",
		"trace.sceneStepProposal =",
		"trace.combinedProposal =",
		"ShadowCmp",
		"divergence_injection",
		"default_takeover",
		"packetMode",
		"degraded",
		"fallbackReason",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-08 backend turn-engine marker %q", needle)
		}
	}
	if strings.Contains(src, "return buildBlockedPayload(payload, backendBlock.userMessage || backendBlock.reason)") {
		t.Fatal("backend-off prepare-turn path must remain fail-open and must not return a blocked payload")
	}
	if strings.Contains(src, "await fireMaintenancePass(") {
		t.Fatal("maintenance pass must stay fire-and-forget and must not block chat flow")
	}
}

func TestArchiveCenterJSOnlyModelTypeEntersPersistence(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function isNarrativeType(type)",
		"function isSaveType(type)",
		"function isContextInjectionType(type)",
		`return !type || type === "model";`,
		"archive_center_ignores_non_model_risu_requests",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing Gemini AI Studio OtherAx persistence marker %q", needle)
		}
	}
}

func TestArchiveCenterJSModelPayloadIsNotBlockedByPromptMarkersOrTailMismatch(t *testing.T) {
	src := readArchiveCenterJS(t)
	for _, marker := range []string{
		`reason: "model_request_auxiliary_marker_trace_only"`,
		`policy: "risu_model_type_is_authoritative_marker_does_not_block"`,
		`reason: "model_payload_tail_authoritative"`,
		`policy: "risu_model_type_and_latest_payload_user_are_authoritative"`,
		`reason: "post_output_secondary_request"`,
	} {
		if !strings.Contains(src, marker) {
			t.Fatalf("Archive Center.js missing model ownership contract %q", marker)
		}
	}
	for _, obsolete := range []string{
		`reason: "auxiliary_module_request"`,
		`reason: "payload_user_tail_mismatch_active_tail_block"`,
	} {
		if strings.Contains(src, obsolete) {
			t.Fatalf("Archive Center.js still blocks model requests with obsolete content classifier %q", obsolete)
		}
	}
}

func TestArchiveCenterJSPersistenceRequestsDoNotBlindRetry(t *testing.T) {
	src := readArchiveCenterJS(t)
	for _, marker := range []string{
		`bridgeFetchWithRetry("/turns", { method: "POST", body }, 1)`,
		`bridgeFetchWithRetry("/effective-inputs", { method: "POST", body }, 1)`,
		`bridgeFetchWithRetry("/turns/complete", { method: "POST", body }, 1)`,
		`bridgeFetchWithRetry("/complete-turn", { method: "POST", body, timeoutMs: getCompleteTurnTimeoutMs() }, 1)`,
		`/complete-turn/request-status?idempotency_key=`,
		`idempotency_key: idempotencyKey`,
	} {
		if !strings.Contains(src, marker) {
			t.Fatalf("Archive Center.js missing persistence idempotency marker %q", marker)
		}
	}
}

func TestArchiveCenterJSStreamingAfterRequestPollerMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"const STREAMING_AFTER_REQUEST_POLL_INTERVAL_MS = 800;",
		"const STREAMING_AFTER_REQUEST_STABLE_POLLS = 3;",
		"const STREAMING_AFTER_REQUEST_OBSERVED_STREAM_STABLE_POLLS = 4;",
		"const STREAMING_AFTER_REQUEST_OBSERVED_STREAM_QUIET_MS = 8 * 1000;",
		"const _streamingAfterRequestWatchers = new Map();",
		"function armStreamingAfterRequestWatch(sessionId, type, requestId)",
		"function pollStreamingAfterRequestWatch(sessionId)",
		"function markNativeAfterRequestObserved(sessionId, type)",
		"function stopStreamingAfterRequestWatch(sessionId, detail)",
		"native afterRequest missing; recovered from active chat",
		"late native afterRequest ignored after poller recovery",
		"armStreamingAfterRequestWatch(orchSessionId, type, orchRequestId);",
		"Streaming Hook",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing streaming afterRequest poller marker %q", needle)
		}
	}
}

func TestArchiveCenterJSAfterRequestKeepsEstablishedOriginCID(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"async function resolveAfterRequestWriteSessionId(orchResult)",
		"!isCidSessionId(orchSessionId) && isCidSessionId(currentSessionId) && currentSessionId !== orchSessionId",
		`reason: "fresh_active_cid_after_request"`,
		"return orchSessionId || currentSessionId;",
	}
	for _, marker := range required {
		if !strings.Contains(src, marker) {
			t.Fatalf("Archive Center.js missing afterRequest origin-session guard %q", marker)
		}
	}
}

func TestArchiveCenterJSReusesPreparedActiveStatesBeforeEndpointFallback(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function activeStatesResultFromPreparedBundle(preparedBundle)",
		`source: "prepare_turn_bundle"`,
		"const bundledActiveStates = activeStatesResultFromPreparedBundle(_preparedBundleCanReplaceSessionReads ? preparedBundle : null);",
		"activeStatesResult = await runActiveStatesFetch(chatSessionId);",
		`source: activeStatesResult.source || (activeStatesResult.fetched ? "active_states_endpoint" : "none")`,
	}
	for _, marker := range required {
		if !strings.Contains(src, marker) {
			t.Fatalf("Archive Center.js missing prepared active-state reuse marker %q", marker)
		}
	}

	bundleAt := strings.Index(src, "const bundledActiveStates = activeStatesResultFromPreparedBundle(_preparedBundleCanReplaceSessionReads ? preparedBundle : null);")
	if bundleAt < 0 {
		t.Fatal("prepared active-state reuse branch is missing")
	}
	fallbackAt := strings.Index(src[bundleAt:], "activeStatesResult = await runActiveStatesFetch(chatSessionId);")
	if fallbackAt < 0 {
		t.Fatal("prepared active-state reuse must be evaluated before endpoint fallback")
	}
}

func TestArchiveCenterJSReusesCompletePreparedStoreSections(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"completeSections: ss.complete_sections",
		`source: "prepare_turn_bundle"`,
		"function sessionSnapshotSectionIsComplete(snapshot, sectionName)",
		"function preparedSessionStateMatchesReadPlan(preparedBundle, plan)",
		"_preparedBundleCanReplaceSessionReads = preparedSessionStateMatchesReadPlan",
		`sessionSnapshotSectionIsComplete(_aggregateSnapshot, "storylines")`,
		`sessionSnapshotSectionIsComplete(_aggregateSnapshot, "characters")`,
		`sessionSnapshotSectionIsComplete(_aggregateSnapshot, "pending_threads")`,
		"const storylineText = formatStorylineBlock(storylineResult);",
		"const characterBaseText = formatCharacterBlock(characterResult);",
		"const pendingThreadText = formatPendingThreadBlock(pendingThreadsResult",
	}
	for _, marker := range required {
		if !strings.Contains(src, marker) {
			t.Fatalf("Archive Center.js missing complete prepared-section reuse marker %q", marker)
		}
	}

	worldRuleFallback := `sessionSnapshotSectionIsComplete(_aggregateSnapshot, "world_rules")`
	if !strings.Contains(src, worldRuleFallback) || !strings.Contains(src, "world_rules: true") {
		t.Fatal("world-rule endpoint aggregate fallback contract is missing")
	}
}

func TestArchiveCenterJSEpisodeGenerateOkStatusMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"const episodeGenerated = !!(result && (",
		`result.code === "episode_generated"`,
		`result.status === "ok" && result.saved === true && result.episode`,
		`if (episodeGenerated) {`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing episode generated response marker %q", needle)
		}
	}
}

func TestArchiveCenterJSWorldGraphLiteActiveScopeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"async function fetchWorldRules",
		"\"/world-rules/\" + encodeURIComponent(sid) + \"/inherited\"",
		"active_scope: fetchWorldRules._lastScopeMeta && fetchWorldRules._lastScopeMeta.active_scope",
		"Scope chain: \" + worldRulesResult.scope_chain.join(\" > \")",
		"if (rule.inherited) entry += \"(inherited) \";",
		"async function explorerFetchWorldGraph",
		"\"/session/\" + encodeURIComponent(sid) + \"/active-scope\"",
		"async function explorerSetWorldScope",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing I-4 World Graph Lite marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSessionStateAggregateReadRoundTripEvidence(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"let _aggregateSnapshot = null;",
		"(_preparedBundleCanReplaceSessionReads && preparedBundle && preparedBundle.sessionState && preparedBundle.sessionState.fetched) ||",
		"const _snap = await fetchSessionState(chatSessionId);",
		`sessionSnapshotSectionIsComplete(_aggregateSnapshot, "storylines")`,
		`sessionSnapshotSectionIsComplete(_aggregateSnapshot, "characters")`,
		`sessionSnapshotSectionIsComplete(_aggregateSnapshot, "world_rules")`,
		"worldRulesResult = { items: _wrItems, count: _wrItems.length, fetched: true, source: \"aggregate\", continuityPackFallback: false };",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing aggregate round-trip marker %q", needle)
		}
	}

	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for aggregate read round-trip smoke")
	}
	script := `
const calls = [];
async function fetchSessionState() {
  calls.push("/session-state");
  return { fetched: true, sections: { storylines: [1], characters: [2], world_rules: [3] } };
}
async function fetchStorylines() { calls.push("/storylines"); return { items: [1], count: 1, fetched: true }; }
async function fetchCharacterStates() { calls.push("/characters"); return { items: [2], count: 1 }; }
async function fetchWorldRules() { calls.push("/world-rules"); return { items: [3], count: 1, fetched: true }; }
function sectionComplete(snapshot, name) {
  return !!(snapshot && snapshot.fetched && snapshot.sections && snapshot.completeSections && snapshot.completeSections[name] === true);
}
async function simulate(useAggregateRead, preparedSnapshot) {
  calls.length = 0;
  const settings = { useAggregateRead };
  const chatSessionId = "sess";
  let _aggregateSnapshot = preparedSnapshot || null;
  if (!_aggregateSnapshot && settings.useAggregateRead) {
    const _snap = await fetchSessionState(chatSessionId);
    _snap.completeSections = { storylines: true, characters: true, world_rules: true };
    if (_snap.fetched) _aggregateSnapshot = _snap;
  }
  let storylineResult;
  if (sectionComplete(_aggregateSnapshot, "storylines")) {
    const _slItems = _aggregateSnapshot.sections.storylines;
    storylineResult = { items: _slItems, count: _slItems.length, fetched: true, source: "aggregate" };
  } else {
    storylineResult = await fetchStorylines(chatSessionId);
  }
  let characterResult;
  if (sectionComplete(_aggregateSnapshot, "characters")) {
    const _chItems = _aggregateSnapshot.sections.characters;
    characterResult = { items: _chItems, count: _chItems.length };
  } else {
    characterResult = await fetchCharacterStates(chatSessionId);
  }
  let worldRulesResult;
  if (sectionComplete(_aggregateSnapshot, "world_rules")) {
    const _wrItems = _aggregateSnapshot.sections.world_rules;
    worldRulesResult = { items: _wrItems, count: _wrItems.length, fetched: true, source: "aggregate" };
  } else {
    worldRulesResult = await fetchWorldRules(chatSessionId);
  }
  return { count: calls.length, calls: calls.slice(), storylineResult, characterResult, worldRulesResult };
}
(async () => {
  const prepared = await simulate(false, {
    fetched: true,
    sections: { storylines: [1], characters: [2], world_rules: [3] },
    completeSections: { storylines: true, characters: true, world_rules: false }
  });
  const aggregate = await simulate(true, null);
  const direct = await simulate(false, null);
  if (prepared.count !== 1 || prepared.calls[0] !== "/world-rules") {
    throw new Error("prepared calls = " + JSON.stringify(prepared));
  }
  if (aggregate.count !== 1 || aggregate.calls[0] !== "/session-state") {
    throw new Error("aggregate calls = " + JSON.stringify(aggregate));
  }
  if (direct.count !== 3 || direct.calls.join(",") !== "/storylines,/characters,/world-rules") {
    throw new Error("direct calls = " + JSON.stringify(direct));
  }
  console.log(JSON.stringify({ aggregate, direct }));
})().catch((err) => { console.error(err.stack || err.message); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("aggregate read round-trip smoke failed: %v\n%s", err, out)
	}
}

func TestArchiveCenterJSMomentumPacketSupervisorAndTraceMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"// M-2c: supervisor input pack (persistent guidance + guide/initiative + momentum)",
		"// I-2c: momentum packet 조건부 fetch",
		"momentumPacket   = optSupervisorPack.momentum_packet   || null;",
		`bridgeFetch("/momentum-packet/" + encodeURIComponent(sessionId)`,
		"momentum_packet: momentumPacket || null",
		"result._momentumApplied = true;",
		"result._momentumPacketStatus = momentumPacket.packet_status || null;",
		"trace.momentum = {",
		"packetStatus: (supervisorResult && supervisorResult._momentumPacketStatus) || null",
		`rows.push(r("Momentum"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing momentum packet supervisor/trace marker %q", needle)
		}
	}
	if strings.Contains(src, "const _mp = await fetchSessionState(sessionId);") {
		t.Fatal("momentum packet path must not perform an extra fetchSessionState round-trip before /momentum-packet")
	}
}

func TestArchiveCenterJSTrustControlExplorerMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`"explorer.tabs.trust.label"`,
		`activeTab: "chat_logs",   // "chat_logs" | "memories" | "direct_evidence" | "kg_triples" | "episodes" | "trust" | "world" | "entities"`,
		"async function explorerFetchTrust",
		"async function explorerPatchTrust",
		`bridgeFetch("/storylines/" + encodeURIComponent(sid)`,
		`bridgeFetch("/world-rules/" + encodeURIComponent(sid)`,
		`bridgeFetch("/pending-threads/" + encodeURIComponent(sid) + "?status=all"`,
		`"/storylines/"`,
		`"/world-rules/"`,
		`"/pending-threads/"`,
		"function renderExplorerTrust()",
		`const cls = "mo-trust-btn"`,
		`data-trust-field="`,
		`data-trust-val="`,
		`document.querySelectorAll(".mo-trust-btn[data-trust-model]")`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing Trust Control Explorer marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P32PermissionSplitMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const coprocessorAuthorityModes = ["truth_writer", "proposal_only", "guidance_only", "diagnostic_only"]`,
		`const coprocessorTruthWriteTargets = ["current_fact", "canonical_state", "canonical_state_layer", "dense_summary"]`,
		`module: "step11_truth_core"`,
		`authority_mode: "truth_writer"`,
		`current_fact_write: true`,
		`canonical_write: true`,
		`module: "truth_maintenance"`,
		`authority_mode: "diagnostic_only"`,
		`module: "entity_coprocessor"`,
		`authority_mode: "proposal_only"`,
		`module: "world_coprocessor"`,
		`module: "narrative_quality_coprocessor"`,
		`authority_mode: "guidance_only"`,
		`denied_write_targets: coprocessorTruthWriteTargets.slice()`,
		`canonical_write_authority_after_reentry: "step11_truth_core_only"`,
		`analysis_provider_autonomous_truth_write: false`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P32 permission split marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P34TraceSeparationMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const coprocessorSidecarWritableTargets = ["proposal_trace", "guidance_trace", "audit_trace", "maintenance_metadata"]`,
		`const entityCoprocessorTraceDisplayMode = "hint_vs_current_fact_split"`,
		`const worldCoprocessorTraceDisplayMode = "hint_vs_current_world_state_split"`,
		`const narrativeQualityCoprocessorTraceDisplayMode = "quality_hint_vs_factual_state_split"`,
		`trace_display_mode: entityCoprocessorTraceDisplayMode`,
		`trace_display_hint_lane: entityCoprocessorTraceDisplayHintLane`,
		`trace_display_truth_lane: entityCoprocessorTraceDisplayTruthLane`,
		`trace_display_mode: worldCoprocessorTraceDisplayMode`,
		`trace_display_hint_lane: worldCoprocessorTraceDisplayHintLane`,
		`trace_display_truth_lane: worldCoprocessorTraceDisplayTruthLane`,
		`trace_display_mode: narrativeQualityCoprocessorTraceDisplayMode`,
		`trace_display_hint_lane: narrativeQualityCoprocessorTraceDisplayHintLane`,
		`trace_display_truth_lane: narrativeQualityCoprocessorTraceDisplayTruthLane`,
		`analysis_provider_trace_target: coprocessorAnalysisProviderTraceTargetsByModule.entity_coprocessor`,
		`analysis_provider_trace_target: coprocessorAnalysisProviderTraceTargetsByModule.world_coprocessor`,
		`analysis_provider_trace_target: coprocessorAnalysisProviderTraceTargetsByModule.narrative_quality_coprocessor`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P34 trace separation marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P35FeatureControlAblationMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const coprocessorFeatureControlPolicyVersion = "mg1c.v1"`,
		`const coprocessorFeatureFlagModes = ["always_on", "conservative", "experimental", "off"]`,
		`const coprocessorRolloutStages = ["truth_floor_locked", "diagnostic_default_on", "experimental_shadow", "manual_enable_required"]`,
		`const coprocessorKillSwitchStates = ["not_applicable", "armed_standby", "engaged"]`,
		`const coprocessorFeatureControlMatrix = [`,
		`module: "step11_truth_core"`,
		`default_mode: "always_on"`,
		`ablation_supported: false`,
		`wiring_status: "hard_enabled_runtime"`,
		`module: "truth_maintenance"`,
		`default_mode: "conservative"`,
		`kill_switch_action: "fail_open_skip"`,
		`module: "entity_coprocessor"`,
		`default_mode: "off"`,
		`wiring_status: "trace_contract_only"`,
		`module: "narrative_quality_coprocessor"`,
		`default_mode: "experimental"`,
		`ablation_supported: true`,
		`coprocessorFeatureControlByMode[featureMode].push(entry.module)`,
		`const coprocessorRuntimeActiveModules = coprocessorFeatureControlMatrix.filter`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P35 feature control marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P120AuthorityMatrixMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const coprocessorAuthorityModes = ["truth_writer", "proposal_only", "guidance_only", "diagnostic_only"]`,
		`const coprocessorAuthorityMatrix = [`,
		`module: "step11_truth_core"`,
		`authority_mode: "truth_writer"`,
		`module: "truth_maintenance"`,
		`authority_mode: "diagnostic_only"`,
		`module: "retrieval_supporting_inference"`,
		`authority_mode: "diagnostic_only"`,
		`module: "entity_coprocessor"`,
		`authority_mode: "proposal_only"`,
		`module: "world_coprocessor"`,
		`authority_mode: "proposal_only"`,
		`module: "narrative_quality_coprocessor"`,
		`authority_mode: "guidance_only"`,
		`coprocessor_authority_matrix: {`,
		`status: "authority_matrix_fixed"`,
		`const coprocessorAuthorityMatrixByMode = {}`,
		`coprocessorAuthorityMatrixByMode[authorityMode].push(entry.module)`,
		`const coprocessorAuthorityTruthWriterModules = (coprocessorAuthorityMatrixByMode.truth_writer || []).slice()`,
		`const coprocessorAuthoritySidecarModules = coprocessorAuthorityMatrix`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P120 authority matrix marker %q", needle)
		}
	}
}
