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
	helpersEnd := strings.Index(src, "function makeEmptyContinuityPackResult")
	if helpersStart < 0 || helpersEnd < 0 || helpersEnd <= helpersStart {
		t.Fatalf("Archive Center.js missing same-turn overlay helper block")
	}

	script := src[helpersStart:helpersEnd] + "\n" +
		extractJSFunctionBlockForTest(t, src, "function normalizeStorylineDetailCompareText") + "\n" +
		extractJSFunctionBlockForTest(t, src, "function isStorylineSelfEchoDetail") + "\n" +
		extractJSFunctionBlockForTest(t, src, "function appendStorylineDetailLine") + "\n" +
		extractJSFunctionBlockForTest(t, src, "function appendStorylineDetailLines") + "\n" +
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
assert(html.includes("Publisher LLM") && html.includes("fail"), "backend-off publisher status missing");
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

func TestArchiveCenterJSSameTurnOverlayDoesNotPersistSupervisorProposals(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"const predictedTurnIndex = peekNextTurnIndex(chatSessionId);",
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
	if strings.Contains(src, "sourceAcceptanceActiveChatQueueDrain") {
		t.Fatal("native afterRequest confirmation must persist from the confirmed RisuAI chat, not retry the premature queue item")
	}
	for _, officialAPI := range []string{"getCurrentCharacterIndex", "getCurrentChatIndex", "getChatFromIndex"} {
		if !strings.Contains(src, `typeof R.`+officialAPI+` === "function"`) {
			t.Fatalf("active-chat confirmation must require official RisuAI API %s", officialAPI)
		}
	}
	for _, forbidden := range []string{
		"async function postStorylineSync(",
		"async function postWorldRulesSync(",
		`bridgeFetch("/storylines/sync"`,
		`bridgeFetch("/world-rules/sync"`,
		`postStorylineSync(supervisorResult, chatSessionId, predictedTurnIndex, "apply").catch`,
		"postWorldRulesSync(supervisorResult, chatSessionId, predictedTurnIndex).catch",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("Archive Center.js still persists supervisor proposal through %q", forbidden)
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
		"payloadMutated",
		"beforeMessageCount",
		"afterMessageCount",
		"finalUserInputPreview",
		"assembledPreview",
		"capturedBeforeRequestReturn",
		"effectiveInputHash",
		"outboundPayloadHash",
		"payloadContentMatch",
		"attachFinalPayloadParityTrace(lastOrchResult && lastOrchResult._trace, payload, outgoingPayload",
		"attachFinalPayloadParityTrace(lastOrchResult && lastOrchResult._trace, payload, payload",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing final payload parity marker %q", needle)
		}
	}
}

func TestArchiveCenterJSBackendOwnsInputContextAndVerifiedPreview(t *testing.T) {
	src := readArchiveCenterJS(t)
	for _, needle := range []string{
		"return applyGoPayloadApplicationPlan(payload, orchResult, emptyResult);",
		`plan.owner === "go"`,
		`plan.apply_rule === "apply_exact_text_without_reassembly"`,
		`injectInputContextBeforeUser(finalPayload, inputContextText)`,
		"pre_request_payload_verification_missing_or_mismatch",
	} {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing backend-owned input/parity marker %q", needle)
		}
	}
}

func TestArchiveCenterJSMemoryDeliveryBudgetsUseSynchronizedSliders(t *testing.T) {
	src := readArchiveCenterJS(t)
	ids := []string{
		"mo-memoryBudgetEventRecent",
		"mo-memoryBudgetCharacterObjective",
		"mo-memoryBudgetSubjectiveRelationship",
		"mo-memoryBudgetWorldState",
		"mo-memoryBudgetProtectedSecret",
		"mo-memoryBudgetUnresolvedGoal",
		"mo-memoryBudgetDirectEvidence",
	}
	for _, id := range ids {
		if !strings.Contains(src, `data-sync-input="`+id+`"`) {
			t.Fatalf("missing synchronized range for %s", id)
		}
	}
	for _, marker := range []string{
		"function syncMemoryDeliveryBudgetControls()",
		`document.querySelectorAll("[data-memory-budget-control]")`,
		`el.disabled = !custom`,
	} {
		if !strings.Contains(src, marker) {
			t.Fatalf("missing memory budget slider mode marker %q", marker)
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
		"return applyGoPayloadApplicationPlan(payload, orchResult, emptyResult);",
		"function applyGoPayloadApplicationPlan(payload, orchResult, emptyResult)",
		"plan.apply_rule === \"apply_exact_text_without_reassembly\"",
		"injectionTextSource: \"go_payload_application_plan.v1\"",
		"const supervisorResult = (preparedBundle && preparedBundle.supervisorResult)",
		"payloadApplicationPlan: result.payload_application_plan",
		"narrative_support_max_chars:",
		"const injectionPack = orchResult && orchResult._injectionPack",
		`response_projection: "prepare_turn.production_compact.v1"`,
		`responseProjection: result.response_projection || ""`,
		`preparedBundle.tracePreview.compact_orchestration`,
		`trace.search = compactSearchResult`,
		`trace.supervisor = compactSupervisorTrace`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing prepare-turn injection pack marker %q", needle)
		}
	}
	if strings.Contains(src, "await runSupervisor(") {
		t.Fatal("Archive Center.js still performs a separate supervisor call after /prepare-turn")
	}
	for _, forbidden := range []string{"compactMemoryLineage", "compactMemoryCount", "compactSupervisorProposal", "compactSupervisorStatus"} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("Archive Center.js still derives Go-owned compact status/count %q", forbidden)
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

func TestArchiveCenterJSStructuredOutputSanitizeRuntime(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for JS output sanitizer runtime behavior")
		}
	}
	src := readArchiveCenterJS(t)
	names := []string{
		"normalizeReasoningEnvelopeName",
		"isReasoningEnvelopeName",
		"stripHiddenReasoningEnvelopes",
		"sanitizeForCritic",
		"sanitizeNarrativeOutputForDisplay",
	}
	functions := make([]string, 0, len(names))
	for _, name := range names {
		functions = append(functions, extractArchiveCenterJSFunction(t, src, name))
	}
	script := strings.Join(functions, "\n\n") + `
function assertEqual(actual, expected, label) {
  if (actual !== expected) throw new Error(label + ": got=" + JSON.stringify(actual) + " want=" + JSON.stringify(expected));
}
assertEqual(sanitizeNarrativeOutputForDisplay("<analysis>private plan</analysis>Visible narrative."), "Visible narrative.", "closed structured envelope");
assertEqual(sanitizeNarrativeOutputForDisplay("<internal-deliberation>private plan</internal-deliberation>Visible narrative."), "Visible narrative.", "provider-neutral structured envelope");
assertEqual(sanitizeNarrativeOutputForDisplay("<Thoughts\nprivate plan only"), "", "malformed unclosed structured envelope");
assertEqual(sanitizeNarrativeOutputForDisplay("<scene>I need the user to see this ordinary narrative.</scene>"), "<scene>I need the user to see this ordinary narrative.</scene>", "ordinary creator output is not prose-classified");
assertEqual(sanitizeNarrativeOutputForDisplay("I need to decide what happens next in this scene."), "I need to decide what happens next in this scene.", "ordinary prose remains output");
assertEqual(sanitizeForCritic("<reasoning>private</reasoning>Final text"), "Final text", "critic receives the same structured output boundary");
`
	cmd := exec.Command(nodePath, "-e", script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("structured output sanitizer runtime fixture failed: %v\n%s", err, output)
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

func TestArchiveCenterJSRG1jRisuLifecycleOutputBoundaryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function stripHiddenReasoningEnvelopes(text)",
		"function sanitizeForCritic(text)",
		"function sanitizeNarrativeOutputForDisplay(text)",
		`responseReturnContent = typeof displayContent === "string" ? displayContent : content;`,
		`await R.addRisuReplacer("beforeRequest", onBeforeRequest);`,
		`await R.addRisuReplacer("afterRequest", onAfterRequest);`,
		"sanitizeForCritic(rawSeed)",
		"sanitizeForCritic(safeUserContent)",
		"sanitizeForCritic(String(assistantContent || \"\"))",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing RG-1j lifecycle output boundary marker %q", needle)
		}
	}
	for _, forbidden := range []string{
		"function looksLikeHiddenReasoningBody(text)",
		"function looksLikePromptTemplateScaffold(text)",
		"function isPromptTemplateScaffoldLine(line)",
		"function findVisibleOutputBoundaryAfterReasoningPreamble(text)",
		"function isLikelyEffectiveInputScaffoldText(text)",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("Archive Center.js must not classify RisuAI output from creator-specific prose: %q", forbidden)
		}
	}
}

func TestArchiveCenterJSPluginVersionMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"//@name Archive Center",
		"//@display-name Archive Center",
		"//@version 3.9.9",
		`const VERSION = "3.9.9";`,
		`"settings.title": ` + "`🗂️ Archive Center ${VERSION} 설정`",
		`"settings.title": ` + "`🗂️ Archive Center ${VERSION} Settings`",
		`"settings.title": ` + "`🗂️ Archive Center ${VERSION} 設定`",
		`const VERSION_STR = typeof VERSION !== "undefined" ? String(VERSION) : "unknown";`,
		"source_version:    VERSION_STR",
		`bridgeFetch("/update/check", {`,
		"const body = {};",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing plugin version marker %q", needle)
		}
	}
	for _, staleClientVersion := range []string{
		`params.set("current_version", VERSION)`,
		`const body = { current_version: VERSION }`,
	} {
		if strings.Contains(src, staleClientVersion) {
			t.Fatalf("Archive Center.js must not override backend update version with %q", staleClientVersion)
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

func TestArchiveCenterJSTurnWorkflowHUDSettingMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`turnWorkflowHUDEnabled: true`,
		`merged.turnWorkflowHUDEnabled = merged.turnWorkflowHUDEnabled !== false`,
		`<input type="checkbox" id="mo-turnWorkflowHUDEnabled"`,
		`turnWorkflowHUDEnabled: readChecked("mo-turnWorkflowHUDEnabled", true)`,
		`setCheckedIfPresent("mo-turnWorkflowHUDEnabled", settings.turnWorkflowHUDEnabled !== false)`,
		`if (prevTurnWorkflowHUDEnabled && settings.turnWorkflowHUDEnabled === false)`,
		`function turnWorkflowHUDIsEnabled()`,
		`if (!turnWorkflowHUDIsEnabled())`,
		`function consumeTurnWorkflowHUDNotice(view)`,
		`"turn_hud.stage.publisher_llm": "출판사 LLM 호출"`,
		`"turn_hud.stage.raw_persist": "입력 저장"`,
		`"turn_hud.stage.critic_llm": "평론가 호출"`,
		`"turn_hud.count.knowledge_graph": "관계 지식"`,
		`"turn_hud.count.relationship_state": "관계 상태"`,
		`"explorer.tabs.kg_triples.label": "관계 지식"`,
		`"turn_hud.stage.publisher_llm": "Publisher LLM call"`,
		`"turn_hud.stage.raw_persist": "Saving input"`,
		`"turn_hud.stage.critic_llm": "Critic call"`,
		`"turn_hud.stage.publisher_llm": "Publisher LLM 呼び出し"`,
		`"turn_hud.stage.raw_persist": "入力を保存"`,
		`"turn_hud.stage.critic_llm": "批評家を呼び出し"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing turn workflow HUD setting marker %q", needle)
		}
	}
	for _, forbidden := range []string{
		"function showTurnWorkflowHUDOOCRecognition(",
		`kind: "ooc_input_cancelled"`,
		`"turn_hud.count.relationship_knowledge"`,
		"`ooc-observation:${sessionId}:",
		`"turn_hud.stage.publisher_llm": "감독관 LLM 호출"`,
		`"turn_hud.stage.raw_persist": "사용자·Assistant 원문 저장"`,
		`"turn_hud.stage.critic_llm": "평론가 LLM 호출"`,
		`"turn_hud.stage.publisher_llm": "Supervisor LLM call"`,
		`"turn_hud.stage.raw_persist": "Saving user and Assistant source"`,
		`"turn_hud.stage.critic_llm": "Critic LLM call"`,
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("Archive Center.js must not fabricate an OOC decision or notice: %q", forbidden)
		}
	}
}

func TestArchiveCenterJSRetiredLocalInitiativePolicyIsAbsent(t *testing.T) {
	src := readArchiveCenterJS(t)
	forbidden := []string{
		`storyNarrativeStance: "balanced"`,
		"merged.storyNarrativeStance = sanitizeEnumValue(",
		"NARRATIVE_STANCE_MODES",
		`<select id="mo-storyNarrativeStance"`,
		`storyNarrativeStance: $("mo-storyNarrativeStance").value`,
		"function buildInitiativeModeSuffix(mode)",
		"function buildInitiativeModeBounds(mode)",
		`narrative_stance: settings.storyNarrativeStance || "balanced"`,
		"extractNarrativeStanceSummary(_narrativeStance)",
		"initiativeSummaryRaw",
		"initiativeSuffixRaw",
		"initiativeBoundsRaw",
		`debugLog("initiative:", _initiativeSummary.mode`,
		"storyInitiativeMode",
	}
	for _, needle := range forbidden {
		if strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js still contains retired local initiative policy %q", needle)
		}
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
		"function buildPrepareTurnHostObservations(sessionId, requestId, type, rawInputObservation",
		`source_kind: String(sourceKind || "host")`,
		`currentInputDecision.selected_observation_ref`,
		`policy: "go_current_input_decision.v1"`,
		"recoverCurrentUserInputFromActiveChatTail",
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
		"continuity_trigger_mode: (continuityInfo && continuityInfo.triggerMode)",
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
		`let currentInputDecision = sourceDecisionResult && sourceDecisionResult.currentInputDecision`,
		`const fullCurrentInputDecision = preparedTurnResult && preparedTurnResult.currentInputDecision`,
		`current_user_input_backend_unavailable`,
		`original_payload_preserved: true`,
		`updateRuntimeState("lastBridgeHealth", "ok"`,
		"async function tryCompleteTurn(turnIdx, userInput, assistantContent, contextMessages, chatSessionId, improvementTrace, prebuiltBody)",
		`bridgeFetchWithRetry("/complete-turn"`,
		"function fireMaintenancePass(turnIdx, chatSessionId, assistantContent, traceRef, recentResponses, supervisorResult)",
		"`/maintenance/enqueue`",
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
	for _, forbidden := range []string{
		"trace.autonomyPlan",
		"trace.microBeatProposal",
		"trace.sceneStepProposal",
		"trace.combinedProposal",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("Archive Center.js retains removed story-control trace marker %q", forbidden)
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
		`request_type: String(type || "model")`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing Gemini AI Studio OtherAx persistence marker %q", needle)
		}
	}
}

func TestArchiveCenterJSModelSourceOwnershipComesFromGo(t *testing.T) {
	src := readArchiveCenterJS(t)
	for _, marker := range []string{
		`let currentInputDecision = sourceDecisionResult && sourceDecisionResult.currentInputDecision`,
		`const fullCurrentInputDecision = preparedTurnResult && preparedTurnResult.currentInputDecision`,
		`allowed: !!currentInputDecision.context_injection_eligible`,
		`policy: "go_current_input_decision.v1"`,
		`hostObservations,`,
		`bootstrapObservation,`,
	} {
		if !strings.Contains(src, marker) {
			t.Fatalf("Archive Center.js missing Go-owned source contract %q", marker)
		}
	}
	for _, obsolete := range []string{
		"function buildMainRequestOwnershipDecision(",
		"function matchAuxiliaryModuleRequestMarker(",
		"function isSubstantiveUserPayloadText(",
		"function refreshMainRequestActiveChatForNewUser(",
	} {
		if strings.Contains(src, obsolete) {
			t.Fatalf("Archive Center.js retains obsolete JavaScript source policy %q", obsolete)
		}
	}
}

func TestArchiveCenterJSPersistenceRequestsDoNotBlindRetry(t *testing.T) {
	src := readArchiveCenterJS(t)
	for _, marker := range []string{
		`bridgeFetchWithRetry("/turns", { method: "POST", body }, 1)`,
		`bridgeFetchWithRetry("/turns/complete", { method: "POST", body }, 1)`,
		`bridgeFetchWithRetry("/complete-turn", { method: "POST", body, timeoutMs: 0 }, 1)`,
		`/complete-turn/request-status?idempotency_key=`,
		`idempotency_key: idempotencyKey`,
		`contract_version: "effective_input_observation.v1"`,
		`Number(_ctResult.effective_input_saved || 0) > 0`,
	} {
		if !strings.Contains(src, marker) {
			t.Fatalf("Archive Center.js missing persistence idempotency marker %q", marker)
		}
	}
	if strings.Contains(src, `bridgeFetchWithRetry("/effective-inputs"`) {
		t.Fatal("Archive Center.js retains the duplicate effective-input persistence request")
	}
}

func TestBackendOwnedLongOperationsDoNotUsePluginRequestTimeout(t *testing.T) {
	src := readArchiveCenterJS(t)
	cases := []struct {
		functionName string
		pathFragment string
	}{
		{`notifyBackendSessionDeletedFromRisu`, `req_source=risu_plugin_chat_delete`},
		{`applyReadySessionMigration`, `/admin/session-migrate`},
		{`referenceLibraryDeleteWork`, `bridgeFetch(path`},
		{`referenceLibraryImportFile`, `/documents`},
		{`referenceLibrarySearchVectors`, `/vector/search`},
		{`referenceCanonPreviewFile`, `/canon-packs/preview/v1`},
		{`referenceCanonInstallFile`, `/canon-packs/install/v1`},
		{`referenceCanonLifecycle`, `/lifecycle/v1`},
		{`referenceDiscoveryRunFromUI`, `/source-discovery/jobs/v1`},
		{`referenceDiscoveryAdmitFromUI`, `/admit/v1`},
		{`applyArchiveCenterUpdate`, `/update/apply`},
		{`tryPrepareTurn`, `/prepare-turn`},
		{`drainOneFailedQueueItem`, `bridgeFetchWithRetry("/complete-turn"`},
		{`queuePendingCompleteTurnPayload`, `result = await bridgeFetchWithRetry(`},
		{`executeAutoRollback`, `/rollback/`},
		{`tryCompleteTurn`, `/complete-turn`},
		{`resetArchiveDatabaseFromDebugUI`, `/admin/database-reset`},
		{`exportSession`, `/export`},
		{`runEpisodeBackfillOnlyForSession`, `/admin/rescan`},
		{`runDerivedArtifactBackfillOnlyForSession`, `/admin/rescan`},
		{`explorerRepairChatLogs`, `/turns/repair-replay`},
		{`importHypaMemory`, `/import/hypamemory`},
		{`explorerRegenerateMemory`, `/explorer/memories/regenerate`},
		{`explorerRegenerateEpisode`, `/episodes/regenerate`},
		{`explorerMergeEpisodes`, `/episodes/merge`},
		{`deleteTimelineSessionFromBackend`, `req_source=timeline_manual_delete`},
		{`runTimelineSessionCopy`, `/sessions/migrate-preview`},
		{`runTimelineSessionMigration`, `/sessions/migrate-preview`},
		{`runTimelineSessionMigrationRollback`, `/sessions/migrate-rollback`},
		{`runTimelineSessionMigrationCleanup`, `/sessions/migrate-cleanup-source`},
		{`loadSubjectiveEntityBundlesForPersonaCapsule`, `/subjective-entity-memories/entities`},
		{`createPersonaCapsuleFromSelectedEntityBundle`, `/subjective-entity-memories/capsule`},
		{`createPersonaCapsuleFromSelectedEntityMemories`, `/subjective-entity-memories/capsule`},
	}
	for _, tc := range cases {
		block := extractArchiveCenterJSAsyncFunction(t, src, tc.functionName)
		pathIndex := strings.Index(block, tc.pathFragment)
		if pathIndex < 0 {
			t.Fatalf("%s missing long-operation path %s", tc.functionName, tc.pathFragment)
		}
		end := pathIndex + 2000
		if end > len(block) {
			end = len(block)
		}
		if !strings.Contains(block[pathIndex:end], `timeoutMs: 0`) {
			t.Fatalf("%s still applies Plugin Timeout to %s", tc.functionName, tc.pathFragment)
		}
	}
}

func TestArchiveCenterJSImmediateUpdateUsesOneServerAuthoritativeApplyCall(t *testing.T) {
	src := readArchiveCenterJS(t)
	check := extractArchiveCenterJSAsyncFunction(t, src, "checkArchiveCenterUpdate")
	if strings.Count(check, `bridgeFetch("/update/check"`) != 1 || !strings.Contains(check, `timeoutMs: 0`) {
		t.Fatal("checkArchiveCenterUpdate must allow the backend to finish verified package preflight")
	}
	apply := extractArchiveCenterJSAsyncFunction(t, src, "applyArchiveCenterUpdate")
	if strings.Count(apply, `bridgeFetch("/update/apply"`) != 1 {
		t.Fatal("applyArchiveCenterUpdate must issue exactly one POST /update/apply request")
	}
	for _, forbidden := range []string{"checkArchiveCenterUpdate", "/update/check", "/update/download", "asset_name", "expected_sha256", "current_version", "platform"} {
		if strings.Contains(apply, forbidden) {
			t.Fatalf("applyArchiveCenterUpdate retains client-side update selection %q", forbidden)
		}
	}
	start := strings.Index(src, `const updateDownloadBtn = $("mo-update-download");`)
	if start < 0 {
		t.Fatal("immediate update button binding is missing")
	}
	endOffset := strings.Index(src[start:], "void refreshArchiveCenterUpdateStatus();")
	if endOffset < 0 {
		t.Fatal("update button binding end marker is missing")
	}
	binding := src[start : start+endOffset]
	if strings.Contains(binding, "showConfirmModal") {
		t.Fatal("Update Now still requires a second confirmation click")
	}
	if strings.Count(binding, "applyArchiveCenterUpdate()") != 1 {
		t.Fatal("Update Now click must invoke applyArchiveCenterUpdate exactly once")
	}
}

func TestArchiveCenterJSFinalConfirmationUsesAfterRequestWithoutOutputListener(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"const _pendingFinalConfirmations = new Map();",
		"const _risuHookLifecycle = {",
		"async function captureFinalConfirmationRequestContext(sessionId, type, requestId)",
		"function acceptRisuAfterRequestFinal(sessionId, type, pendingContext, requestContext, assistantContent)",
		"function observePendingFinalConfirmationAtHostSignal(sessionId, signalSource)",
		"async function drainPendingFinalConfirmations(signalSource)",
		`contract_version: "source_acceptance_observation.v3"`,
		`finality_source: "risu_afterRequest"`,
		`finality_state: "received_final_response"`,
		`host_signal_source: "afterRequest"`,
		`prompt_memory_availability: "same_turn"`,
		"function persistAfterRequestContent()",
		`contract_version: "source_acceptance_observation.v2"`,
		`host_lifecycle_contract_version: "risu_host_lifecycle_observation.v1"`,
		`finality_source: "risu_next_host_signal_active_chat"`,
		`finality_state: "committed_assistant_observed"`,
		`position_observation: "committed_before_next_host_signal"`,
		`next_signal_user_observed_content_hash: nextSignalUserObservedContentHash`,
		`request_id_provenance: "archive_center_correlation"`,
		"persistAcceptedHostFinalWithoutBlockingRequest",
		`prompt_memory_availability: "one_turn_late"`,
		`recordRisuHookLifecycle("input", "callback_observed");`,
		`recordRisuHookLifecycle("beforeRequest", "callback_observed");`,
		`recordRisuHookLifecycle("afterRequest", "callback_observed");`,
		`recordRisuHookLifecycle("beforeRequest", "registration_requested_unconfirmed");`,
		"requested (host acceptance unconfirmed)",
		"return responseReturnContent;",
		"_pendingFinalConfirmationDrainRequested = true",
		"pending.requestContext !== observation.requestContext",
		"await captureFinalConfirmationRequestContext(orchSessionId, type, orchRequestId);",
		"ensureActiveChatCompletedTurnsBackfilled(orchSessionId",
		"async function removeRegisteredRisuHooksOnUnload()",
		`await R.removeRisuScriptHandler("input", onInputHook);`,
		`await R.removeRisuReplacer("beforeRequest", onBeforeRequest);`,
		`await R.removeRisuReplacer("afterRequest", onAfterRequest);`,
		"await R.onUnload(removeRegisteredRisuHooksOnUnload);",
		`hostLifecycleObservation: "before_request_observed"`,
		`host_lifecycle_observation: String(observed.hostLifecycleObservation || "")`,
		"Streaming Hook",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing event-driven final confirmation marker %q", needle)
		}
	}
	for _, forbidden := range []string{
		"STREAMING_AFTER_REQUEST_START_DELAY_MS",
		"STREAMING_AFTER_REQUEST_POLL_INTERVAL_MS",
		"STREAMING_AFTER_REQUEST_STABLE_POLLS",
		"STREAMING_AFTER_REQUEST_OBSERVED_STREAM_QUIET_MS",
		"STREAMING_AFTER_REQUEST_TIMEOUT_MS",
		"STREAMING_AFTER_REQUEST_RECENT_RECOVERY_GRACE_MS",
		"ROLLBACK_PROMOTED_ASSISTANT_SYNC_GRACE_MS",
		"assessPromotedAssistantSyncRollbackGuard",
		"pending_active_chat_confirmation",
		"_streamingAfterRequestSyntheticCallDepth",
		"pollStreamingAfterRequestWatch",
		"armStreamingAfterRequestWatch",
		"synthetic_after_request",
		"characterData: true",
		`kind: "host_candidate"`,
		"async function observePendingFinalConfirmation(",
		"onDisplayFinalitySignal",
		`addRisuScriptHandler("display"`,
		`drainPendingFinalConfirmations("native_afterRequest")`,
		`drainPendingFinalConfirmations("risu_display")`,
		"waiting for RisuAI active assistant tail",
		"registerFinalConfirmationObserver",
		"acceptRisuAfterRequestFinality",
		"persistAcceptedAfterRequestWithoutBlockingDisplay",
		"acceptRisuOutputFinal",
		"onRisuOutput",
		"persistOfficialRisuOutputWithoutBlockingHost",
		`recordRisuHookLifecycle("output", "callback_observed");`,
		`addRisuChatListener("output"`,
		`removeRisuChatListener("output"`,
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("Archive Center.js retains forbidden timer/synthetic finality marker %q", forbidden)
		}
	}
	onBeforeRequestAt := strings.Index(src, "async function onBeforeRequest")
	if onBeforeRequestAt < 0 {
		t.Fatal("Archive Center.js missing onBeforeRequest")
	}
	onBeforeRequest := src[onBeforeRequestAt:]
	captureAt := strings.Index(onBeforeRequest, "await captureFinalConfirmationRequestContext(orchSessionId, type, orchRequestId);")
	rollbackAt := strings.Index(onBeforeRequest, "await checkAndAutoRollback(orchSessionId, rollbackComparable.messages")
	if captureAt < 0 || rollbackAt < 0 || captureAt > rollbackAt {
		t.Fatal("RisuAI request coordinates must be captured before removed-tail evaluation")
	}
}

func TestArchiveCenterJSAfterRequestStartsPersistenceWithoutBlockingVisibleOutput(t *testing.T) {
	src := readArchiveCenterJS(t)
	afterRequest := extractArchiveCenterJSFunction(t, src, "onAfterRequest")
	acceptAt := strings.Index(afterRequest, "const finalObservation = acceptRisuAfterRequestFinal(")
	scheduleAt := strings.Index(afterRequest, "Promise.resolve().then(function persistAfterRequestContent()")
	returnRelativeAt := -1
	if scheduleAt >= 0 {
		returnRelativeAt = strings.Index(afterRequest[scheduleAt:], "return responseReturnContent;")
	}
	if acceptAt < 0 || scheduleAt < 0 || returnRelativeAt < 0 {
		t.Fatal("afterRequest final acceptance, persistence scheduling, or response return is missing")
	}
	returnAt := scheduleAt + returnRelativeAt
	if !(acceptAt < scheduleAt && scheduleAt < returnAt) {
		t.Fatal("afterRequest must accept the final response, schedule persistence, and return in order")
	}
	if strings.Contains(src, "async function onAfterRequest") {
		t.Fatal("afterRequest remains async and can withhold the replacement response")
	}
	if strings.Contains(afterRequest[:returnAt], "await ") {
		t.Fatal("afterRequest performs awaited work before returning the visible response")
	}
	if strings.Contains(afterRequest[:returnAt], "resolveAfterRequestWriteSessionId") {
		t.Fatal("afterRequest performs session routing before returning the visible response")
	}
	if strings.Count(afterRequest, "function persistAfterRequestContent()") != 1 {
		t.Fatal("afterRequest persistence schedule must have exactly one entry point")
	}
	for _, forbidden := range []string{"acceptRisuOutputFinal(", "onRisuOutput", `addRisuChatListener("output"`} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("removed output-listener finality path returned: %q", forbidden)
		}
	}
}

func TestArchiveCenterJSAfterRequestUsesBeforeRequestSessionCoordinates(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"const capturedWriteSessionId = normalizeSessionId(",
		"latestOrchResult && latestOrchResult._chatSessionId",
		"const chatSessionId = capturedWriteSessionId || cachedWriteSessionId || SESSION_FALLBACK;",
	}
	for _, marker := range required {
		if !strings.Contains(src, marker) {
			t.Fatalf("Archive Center.js missing captured afterRequest session marker %q", marker)
		}
	}
	if strings.Contains(src, "resolveAfterRequestWriteSessionId") {
		t.Fatal("obsolete afterRequest session reread path remains")
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
		"payloadApplicationPlan: result.payload_application_plan",
		"supervisorResult:   result.supervisor_result",
		"return applyGoPayloadApplicationPlan(payload, orchResult, emptyResult);",
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
		"supervisorInputPack: result.supervisor_input_pack",
		"supervisorResult:   result.supervisor_result",
		"const supervisorResult = (preparedBundle && preparedBundle.supervisorResult)",
		"payloadApplicationPlan: result.payload_application_plan",
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
