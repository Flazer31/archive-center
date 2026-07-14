package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func extractArchiveCenterJSFunction(t *testing.T, src, name string) string {
	t.Helper()
	marker := "  function " + name + "("
	start := strings.Index(src, marker)
	if start < 0 {
		t.Fatalf("Archive Center.js function %s not found", name)
	}
	next := strings.Index(src[start+len(marker):], "\n  function ")
	if next < 0 {
		t.Fatalf("Archive Center.js function %s has no following function boundary", name)
	}
	end := start + len(marker) + next
	return strings.TrimSpace(src[start:end])
}

func extractArchiveCenterJSAsyncFunction(t *testing.T, src, name string) string {
	t.Helper()
	marker := "  async function " + name + "("
	start := strings.Index(src, marker)
	if start < 0 {
		t.Fatalf("Archive Center.js async function %s not found", name)
	}
	nextFunction := strings.Index(src[start+len(marker):], "\n  function ")
	nextAsyncFunction := strings.Index(src[start+len(marker):], "\n  async function ")
	next := nextFunction
	if next < 0 || (nextAsyncFunction >= 0 && nextAsyncFunction < next) {
		next = nextAsyncFunction
	}
	if next < 0 {
		t.Fatalf("Archive Center.js async function %s has no following function boundary", name)
	}
	end := start + len(marker) + next
	return strings.TrimSpace(src[start:end])
}

func TestRollbackRequestPreservesZeroVisibleTurnsAfterCopiedSessionDelete(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for Archive Center rollback request runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	fn := extractArchiveCenterJSAsyncFunction(t, src, "requestBackendRollbackDecision")
	script := fn + `
let capturedBody = null;
function getRequestTimeoutSettingMs() { return 1000; }
async function fetchBackendLatestTurnIndexForSession() { return 9; }
async function safeCall(call) { return call(); }
function serializeSessionRoutingBaselineForBackend() {
  return {backend_turn_at_route: 8, local_pairs_at_route: 0, reason: "timeline_copy"};
}
async function bridgeFetch(path, options) {
  capturedBody = options.body;
  return {status: "ok", contract_version: "rollback.decision.v1", allowed: true, from_turn: 9, decision_token: "fixture"};
}
(async function() {
  await requestBackendRollbackDecision("char_1_cid_target", 9, "assistant_deleted_output_removed", {
    visibleCompletedTurnCount: 0,
    activeCompletedTurnCount: 8,
    backendLatestTurnIndex: 9
  }, "auto");
  if (!capturedBody) throw new Error("rollback request body was not captured");
  if (capturedBody.visible_completed_turns !== 0) {
    throw new Error("visible_completed_turns=" + capturedBody.visible_completed_turns + ", want 0");
  }
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-e", script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("rollback request runtime fixture failed: %v\n%s", err, output)
	}
}

func TestCopiedSessionFinalOutputRecoveryUsesCurrentChatIndex(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for Archive Center copied-session recovery runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	fn := extractArchiveCenterJSAsyncFunction(t, src, "recoverAssistantContentFromActiveChat")
	script := fn + `
async function resolveCurrentActiveChatObject() { return {chat: {message: []}, source: "R.getChatFromIndex"}; }
function extractActiveChatComparableMessages() {
  return [{role: "user", content: "new copied-session input"}, {role: "assistant", content: "final copied-session output"}];
}
function normalizeMainTurnCompareText(value) { return String(value || "").trim(); }
function mainTurnTextMatchesOriginal(left, right) { return String(left || "").trim() === String(right || "").trim(); }
function buildCompletedTurnPairsFromActiveChatMessages() {
  return [{userContent: "new copied-session input", assistantContent: "final copied-session output"}];
}
function normalizeAssistantPersistenceCandidate(value) { return String(value || "").trim(); }
function isAssistantPrefillSeedText() { return false; }
function debugLog() {}
(async function() {
  const recovered = await recoverAssistantContentFromActiveChat("char_1_cid_target", null, "new copied-session input");
  if (recovered !== "final copied-session output") {
    throw new Error("recovered=" + JSON.stringify(recovered));
  }
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-e", script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("copied-session final output recovery fixture failed: %v\n%s", err, output)
	}
}

func TestCoreRegressionInputOwnershipAndCanonicalOutputRuntime(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for Archive Center core regression runtime fixtures")
		}
	}
	src := readArchiveCenterJS(t)
	names := []string{
		"extractGigaTransCanonicalAssistantText",
		"extractAssistantTaggedBlocks",
		"removeAssistantTaggedBlocks",
		"extractPostprocessorCanonicalAssistantText",
		"canonicalizeAssistantTranslationDisplayForPersistence",
		"canonicalizeAssistantOutputForPersistence",
		"resolveCurrentTurnUserInputInfo",
		"buildMainRequestOwnershipDecision",
		"rememberNonMainRequestSkip",
	}
	functions := make([]string, 0, len(names))
	for _, name := range names {
		functions = append(functions, extractArchiveCenterJSFunction(t, src, name))
	}

	script := strings.Join(functions, "\n\n") + `
function attachTranslationDisplayCanonicalizationTrace() {}
function attachPostprocessorCanonicalizationTrace() {}
let fixtureRawCached = null;
let fixturePayloadCandidate = null;
let fixtureMessageInput = "";
function peekRawInputForSession() { return fixtureRawCached; }
function extractPayloadUserInputCandidate() { return fixturePayloadCandidate; }
function resolveCurrentTurnUserInput() { return fixtureMessageInput; }
function requestHasCurrentInputCandidate(payloadCandidate, messageInput) { return !!((payloadCandidate && payloadCandidate.text) || messageInput); }
function rawInputCacheMatchesCurrentCandidates(rawCached, payloadCandidate, messageInput) {
  if (!rawCached || !rawCached.text) return false;
  return !!((payloadCandidate && payloadCandidate.text === rawCached.text) || messageInput === rawCached.text);
}

function isFreshStrongRawInput(rawCached) { return !!(rawCached && rawCached.text && rawCached.strong); }
function isRisuPromptScaffoldMessage(text) { return /Sealed\. Conduct|cached context control/i.test(String(text || "")); }
function isMetaPromptLikeMessage(text) { return isRisuPromptScaffoldMessage(text); }
function resolveLastNonMetaChatMessageRole() { return "user"; }
function extractCurrentInputBeyondAssistant() { return null; }
function shouldRejectLowTrustCurrentInput(text) { return isRisuPromptScaffoldMessage(text); }
function auxiliaryMessageContentText(content) { return String(content || ""); }
function extractManualResumeTrigger() { return {matched:false}; }
function warnLog() {}
function isNarrativeType(type) { return !type || type === "model"; }
function getLastPayloadUserText(messages) {
  for (let i = (messages || []).length - 1; i >= 0; i--) {
    if (messages[i] && messages[i].role === "user") return String(messages[i].content || "");
  }
  return "";
}
function normalizeMainTurnCompareText(text) { return String(text || "").trim().replace(/\s+/g, " "); }
function matchAuxiliaryModuleRequestMarker(messages) {
  return (messages || []).some(m => /AUXILIARY_MODULE_MARKER/.test(String(m && m.content || ""))) ? "fixture_auxiliary" : "";
}
let fixturePostOutputContext = null;
function buildPostOutputSecondaryRequestContext() { return fixturePostOutputContext; }
function mainTurnTextMatchesOriginal(left, right) {
  return left === right || (!!left && !!right && (left.startsWith(right) || right.startsWith(left)));
}
function isSubstantiveUserPayloadText(text) { return normalizeMainTurnCompareText(text).length >= 2; }
function truncPreview(text, limit) { return String(text || "").slice(0, limit || 120); }
function assertEqual(actual, expected, label) {
  if (actual !== expected) throw new Error(label + ": got=" + JSON.stringify(actual) + " want=" + JSON.stringify(expected));
}
function assertTrue(value, label) { if (!value) throw new Error(label); }
const _nonMainRequestSkipBySession = new Map();
const NON_MAIN_REQUEST_SKIP_TTL_MS = 30000;
let restoredRawInput = null;
function cacheRawInputForSession(sessionId, text) { restoredRawInput = {sessionId, text}; }
function getPluginMainTimeoutSettingMs() { return 15000; }

const giga = "<GT-SEP/>번역 표시<GigaTrans>Original narrative only.</GigaTrans><GT-CTRL/>";
assertEqual(canonicalizeAssistantOutputForPersistence(giga, {}, "fixture"), "Original narrative only.", "gigatrans canonical original");
const malformed = "번역 표시<GigaTrans>closing tag missing";
assertEqual(canonicalizeAssistantOutputForPersistence(malformed, {}, "fixture"), malformed, "malformed gigatrans fail-open");
const ordinary = "ordinary assistant output";
assertEqual(canonicalizeAssistantOutputForPersistence(ordinary, {}, "fixture"), ordinary, "ordinary output unchanged");

const contract = "draft<ArchiveCenterFinalOutput>polished final</ArchiveCenterFinalOutput>";
assertEqual(canonicalizeAssistantOutputForPersistence(contract, {}, "fixture"), "polished final", "postprocessor contract final");
const reko = "<ReKoCompare><ReKoBefore>draft</ReKoBefore><ReKoAfter>reko final</ReKoAfter><ReKoMeta>mode=KR</ReKoMeta></ReKoCompare>";
assertEqual(canonicalizeAssistantOutputForPersistence(reko, {}, "fixture"), "reko final", "legacy compare final");
const nested = "<ArchiveCenterFinalOutput>번역<GigaTrans>nested original</GigaTrans></ArchiveCenterFinalOutput>";
assertEqual(canonicalizeAssistantOutputForPersistence(nested, {}, "fixture"), "nested original", "postprocessor then translation canonicalization");

fixtureRawCached = null;
fixturePayloadCandidate = {text:"actual player input", source:"payload.input"};
fixtureMessageInput = "Sealed. Conduct the next bar.";
let inputInfo = resolveCurrentTurnUserInputInfo({}, [{role:"user",content:fixtureMessageInput}], "fixture-session");
assertEqual(inputInfo.text, "actual player input", "user-role scaffold must not replace actual input");
assertEqual(inputInfo.source, "payload.input", "user-role scaffold input source");

fixturePayloadCandidate = {text:"actual cached-turn input", source:"payload.current_input"};
fixtureMessageInput = "cached context control payload";
inputInfo = resolveCurrentTurnUserInputInfo({}, [{role:"user",content:fixtureMessageInput}], "fixture-session");
assertEqual(inputInfo.text, "actual cached-turn input", "cache-control tail must not replace actual input");

fixturePayloadCandidate = null;
fixtureMessageInput = "선택지로 생성된 실제 입력";
inputInfo = resolveCurrentTurnUserInputInfo({}, [{role:"user",content:fixtureMessageInput}], "fixture-session");
assertEqual(inputInfo.text, "선택지로 생성된 실제 입력", "selection-generated input preserved");
assertEqual(inputInfo.source, "messages.tail", "selection-generated input source");

const nonModel = buildMainRequestOwnershipDecision("submodel", [{role:"user", content:"hello"}], "hello", "hello", []);
assertEqual(nonModel.allowed, false, "non-model excluded");

const userRolePrompt = buildMainRequestOwnershipDecision("model", [
  {role:"user", content:"actual player input"},
  {role:"user", content:"Sealed. Conduct the next bar."}
], "actual player input", "actual player input", []);
assertTrue(userRolePrompt.allowed, "user-role prompt must not block model request");
assertTrue(userRolePrompt.reason !== "auxiliary_module_request" && userRolePrompt.reason !== "payload_user_tail_mismatch_active_tail_block", "obsolete user-role block reason returned");

const cachingPayload = buildMainRequestOwnershipDecision("model", [
  {role:"user", content:"actual player input"},
  {role:"user", content:"cached context control payload"}
], "actual player input", "actual player input", []);
assertTrue(cachingPayload.allowed, "context-caching shaped model request must remain allowed");

const selectedInput = buildMainRequestOwnershipDecision("model", [
  {role:"system", content:"AUXILIARY_MODULE_MARKER"},
  {role:"user", content:"선택지로 생성된 실제 입력"}
], "", "", []);
assertEqual(selectedInput.allowed, true, "selected input allowed");
assertEqual(selectedInput.contextInjectionAllowed, true, "selected input injectable");

const auxiliaryOnly = buildMainRequestOwnershipDecision("model", [
  {role:"user", content:"AUXILIARY_MODULE_MARKER"}
], "", "", []);
assertEqual(auxiliaryOnly.allowed, true, "model marker is trace-only");
assertEqual(auxiliaryOnly.contextInjectionAllowed, false, "auxiliary-only marker not injected");

fixturePostOutputContext = {
  userContent: "previous player input",
  assistantContent: "previous assistant output",
  contextMessages: []
};
const genuineNextTurn = buildMainRequestOwnershipDecision("model", [
  {role:"user", content:"genuine next player input"}
], "genuine next player input", "genuine next player input", [
  {role:"user", content:"previous player input"},
  {role:"assistant", content:"previous assistant output"},
  {role:"user", content:"genuine next player input"}
]);
assertEqual(genuineNextTurn.allowed, true, "new raw input must win over stale post-output context");
assertEqual(genuineNextTurn.reason, "active_tail_match", "new active-chat user input ownership reason");

const realPostprocessor = buildMainRequestOwnershipDecision("model", [
  {role:"user", content:"rewrite the previous assistant output"}
], "previous player input", "", [
  {role:"user", content:"previous player input"},
  {role:"assistant", content:"previous assistant output"}
]);
assertEqual(realPostprocessor.allowed, false, "postprocessor request remains excluded from a new turn");
assertEqual(realPostprocessor.reason, "post_output_secondary_request", "postprocessor ownership reason");

const postprocessorInputHookReplay = buildMainRequestOwnershipDecision("model", [
  {role:"user", content:"rewrite the previous assistant output"}
], "previous player input", "rewrite the previous assistant output", [
  {role:"user", content:"previous player input"},
  {role:"assistant", content:"previous assistant output"}
]);
assertEqual(postprocessorInputHookReplay.allowed, false, "postprocessor input hook must not create a new user turn");
assertEqual(postprocessorInputHookReplay.reason, "post_output_secondary_request", "completed active-chat pair wins over postprocessor raw cache");
rememberNonMainRequestSkip("fixture-session", postprocessorInputHookReplay, "beforeRequest");
assertEqual(restoredRawInput && restoredRawInput.text, "previous player input", "postprocessor prompt must restore the pinned player input");
assertEqual(_nonMainRequestSkipBySession.get("fixture-session").reason, "post_output_secondary_request", "postprocessor skip remains paired with afterRequest");
`

	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Archive Center core regression JS runtime fixture failed: %v\n%s", err, out)
	}
}

func TestPostprocessorOwnershipWaitsForCanonicalNewUserMessage(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for postprocessor ownership runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functionBody := extractArchiveCenterJSAsyncFunction(t, src, "refreshMainRequestActiveChatForNewUser")
	script := functionBody + `
function getLastPayloadUserText(messages) {
  for (let i = (messages || []).length - 1; i >= 0; i--) {
    if (messages[i] && messages[i].role === "user") return String(messages[i].content || "");
  }
  return "";
}
function normalizeMainTurnCompareText(text) { return String(text || "").trim(); }
function mainTurnTextMatchesOriginal(left, right) { return normalizeMainTurnCompareText(left) === normalizeMainTurnCompareText(right); }
function getLastNonEmptyComparableMessage(messages) {
  for (let i = (messages || []).length - 1; i >= 0; i--) {
    if (messages[i] && String(messages[i].content || "").trim()) return messages[i];
  }
  return null;
}
function buildPostOutputSecondaryRequestContext(messages) {
  const latest = getLastNonEmptyComparableMessage(messages);
  return latest && latest.role === "assistant"
    ? {userContent:"previous player input", assistantContent:latest.content, contextMessages:messages}
    : null;
}
const previousPair = [
  {role:"user",content:"previous player input"},
  {role:"assistant",content:"previous assistant output"},
];
let reads = 0;
async function getCurrentActiveChatComparableMessages() {
  reads++;
  return reads < 2 ? previousPair : previousPair.concat([{role:"user",content:"genuine next player input"}]);
}
(async function() {
  let result = await refreshMainRequestActiveChatForNewUser(
    "fixture-session",
    [{role:"user",content:"genuine next player input"}],
    "genuine next player input",
    previousPair
  );
  let latest = getLastNonEmptyComparableMessage(result);
  if (!latest || latest.role !== "user" || latest.content !== "genuine next player input") {
    throw new Error("delayed canonical user message was not recovered: " + JSON.stringify(result));
  }

  reads = 0;
  getCurrentActiveChatComparableMessages = async function() { return previousPair; };
  result = await refreshMainRequestActiveChatForNewUser(
    "fixture-session",
    [{role:"user",content:"rewrite the previous assistant output"}],
    "rewrite the previous assistant output",
    previousPair
  );
  latest = getLastNonEmptyComparableMessage(result);
  if (!latest || latest.role !== "assistant") {
    throw new Error("postprocessor request must remain anchored to the completed pair: " + JSON.stringify(result));
  }
})().catch(function(err) {
  console.error(err && err.stack || err);
  process.exit(1);
});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("postprocessor ownership JS runtime fixture failed: %v\n%s", err, out)
	}
}

func TestHistoryTrimGuardRequiresObservedSlashCommand(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for Archive Center history trim runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functions := []string{
		extractArchiveCenterJSFunction(t, src, "recordRisuHistoryTrimGuard"),
		extractArchiveCenterJSFunction(t, src, "getRecentRisuHistoryTrimGuard"),
		extractArchiveCenterJSFunction(t, src, "buildSnapshotHistoryTrimGuardOr1f"),
	}
	script := strings.Join(functions, "\n\n") + `
const _rollbackHistoryTrimGuardBySession = new Map();
const ROLLBACK_HISTORY_TRIM_GUARD_MS = 5 * 60 * 1000;
function compactSnapshotMessages(messages) { return Array.isArray(messages) ? messages : []; }
function computeTailHash() { return "fixture-tail"; }
function assertTrue(value, label) { if (!value) throw new Error(label); }

recordRisuHistoryTrimGuard("s", "active_chat_suffix_window_trim", {});
assertTrue(getRecentRisuHistoryTrimGuard("s") === null, "inferred trim must not become command intent");
const withoutCommand = buildSnapshotHistoryTrimGuardOr1f(
  "s",
  {messagesPreview:[{role:"user",content:"old"},{role:"assistant",content:"kept"}], turnIndex:1, tailHash:"old"},
  [{role:"assistant",content:"kept"}],
  {commonPrefixLen:0, commonSuffixLen:1, removedMsgCount:1, insertedMsgCount:0},
  {previousAssistantCount:1, currentAssistantCount:1}
);
assertTrue(withoutCommand === null, "ordinary deletion must not be protected as slash trim");

recordRisuHistoryTrimGuard("s", "risu_slash_command", {commandPreview:"/cut"});
assertTrue(getRecentRisuHistoryTrimGuard("s") !== null, "observed slash command must enable trim protection");
const withCommand = buildSnapshotHistoryTrimGuardOr1f(
  "s",
  {messagesPreview:[{role:"user",content:"old"},{role:"assistant",content:"kept"}], turnIndex:1, tailHash:"old"},
  [{role:"assistant",content:"kept"}],
  {commonPrefixLen:0, commonSuffixLen:1, removedMsgCount:1, insertedMsgCount:0},
  {previousAssistantCount:1, currentAssistantCount:1}
);
assertTrue(!!withCommand && withCommand.explicitCommandObserved === true, "slash trim protection missing");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Archive Center history trim runtime fixture failed: %v\n%s", err, out)
	}
}

func TestRollbackComparablePrefersActiveChatWhenCurrentInputIsConfirmed(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for Archive Center rollback source runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functionBody := extractArchiveCenterJSAsyncFunction(t, src, "resolveRollbackComparableMessages")
	script := functionBody + `
let fixtureActiveMessages = [];
let fixturePreviousSnapshot = null;
async function getCurrentActiveChatRollbackMessages() { return fixtureActiveMessages; }
function compactSnapshotMessages(messages) { return Array.isArray(messages) ? messages : []; }
function getSessionSnapshot() { return fixturePreviousSnapshot; }
function getLastPayloadUserText(messages) {
  for (let i = (messages || []).length - 1; i >= 0; i--) {
    if (messages[i] && messages[i].role === "user") return String(messages[i].content || "");
  }
  return "";
}

function mainTurnTextMatchesOriginal(left, right) {
  const a = String(left || "").trim();
  const b = String(right || "").trim();
  return a === b || (!!a && !!b && (a.startsWith(b) || b.startsWith(a)));
}
function assertEqual(actual, expected, label) {
  if (actual !== expected) throw new Error(label + ": got=" + JSON.stringify(actual) + " want=" + JSON.stringify(expected));
}

(async function() {
  fixturePreviousSnapshot = {msgCount: 8};
  fixtureActiveMessages = [
    {role:"user",content:"u1"}, {role:"assistant",content:"a1"},
    {role:"user",content:"u2"}, {role:"assistant",content:"a2"},
    {role:"user",content:"u3"}, {role:"assistant",content:"a3"},
    {role:"user",content:"new player input"}
  ];
  const longPayload = Array.from({length:20}, (_, i) => ({role:i % 2 ? "assistant" : "user", content:"prompt-" + i}));
  longPayload.push({role:"user",content:"rewritten payload input"});
  let result = await resolveRollbackComparableMessages("s", longPayload, "new player input");
  assertEqual(result.source, "active_chat_current_input_confirmed", "deleted active history with current input must remain authoritative");
  assertEqual(result.messages.length, 7, "active chat deletion shape lost");

  fixtureActiveMessages[fixtureActiveMessages.length - 1] = {role:"user",content:"stale previous input"};
  result = await resolveRollbackComparableMessages("s", longPayload, "new player input");
  assertEqual(result.source, "payload_newer_than_stale_active_chat", "unconfirmed stale active chat must keep payload fallback");
})().catch(function(err) {
  console.error(err && err.stack || err);
  process.exit(1);
});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Archive Center rollback source JS runtime fixture failed: %v\n%s", err, out)
	}
}

func TestRisuMessageIndexesDriveLogicalTurnPairs(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for Risu message index runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functionBody := extractArchiveCenterJSFunction(t, src, "buildCompletedTurnPairsFromActiveChatMessages")
	script := functionBody + `
const ACTIVE_CHAT_BACKFILL_MAX_CONTEXT_MESSAGES = 20;
const AUTO_CONTINUE_USER_INPUT_MARKER = "[continue]";
function extractComparableMessageRoleAndContent(msg) { return msg; }
function selectBestAssistantCandidateRecord(items) { return items[items.length - 1] || null; }
function normalizeAssistantPersistenceCandidate(text) { return String(text || "").trim(); }
function shouldSkipUserInputPersistence(text) { return text === "skip indexed turn"; }
function shouldSkipTurnPersistenceForOoc() { return false; }
function computeOrchestrationDirtyHashOr1c(text) { return String(text).length; }
function getActiveChatMessageStreamingState() { return "done"; }
function debugLog() {}
function assertEqual(actual, expected, label) {
  if (actual !== expected) throw new Error(label + ": got=" + JSON.stringify(actual) + " want=" + JSON.stringify(expected));
}
const messages = [];
for (let turn = 1; turn <= 7; turn++) {
  const userIndex = (turn - 1) * 2;
  messages.push(
    {role:"user",content:turn === 2 ? "skip indexed turn" : "u"+turn,risuMessageIndex:userIndex},
    {role:"assistant",content:"a"+turn,risuMessageIndex:userIndex+1}
  );
}
let pairs = buildCompletedTurnPairsFromActiveChatMessages(messages);
assertEqual(JSON.stringify(pairs.map(p => p.risuUserMessageIndex)), JSON.stringify([0,4,6,8,10,12]), "adapter preserves raw Risu indexes after a filtered pair");
assertEqual(JSON.stringify(pairs.map(p => p.observedPairOrdinal)), JSON.stringify([1,2,3,4,5,6]), "adapter reports observation order without deriving logical turns");
assertEqual(pairs.some(p => Object.prototype.hasOwnProperty.call(p, "turnIndex")), false, "adapter must not calculate authoritative turn indexes");
pairs = buildCompletedTurnPairsFromActiveChatMessages(messages.slice(0, -2));
assertEqual(pairs[pairs.length - 1].risuUserMessageIndex, 10, "tail deletion exposes the preceding raw Risu index");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Risu message index JS runtime fixture failed: %v\n%s", err, out)
	}
}

func TestRisuIndexLatestTurnFeedsRoutingAndRollbackObservation(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for Risu index routing runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	script := extractArchiveCenterJSAsyncFunction(t, src, "requestBackendSessionRoutingTurnResolution") + `
let captured = null;
async function bridgeFetch(path, options) {
  if (path !== "/session-routing/turn-resolution") throw new Error("unexpected path: "+path);
  captured = options.body;
  return {status:"ok",contract_version:"session-routing.turn-resolution.v1",resolution:"normal",turn_index:7,completed_turns:7,local_turn_index:7,local_turn_source:"risu_user_message_index",baseline_applied:false,resolved_observations:[]};
}
function getRequestTimeoutSettingMs() { return 1000; }
function serializeSessionRoutingBaselineForBackend() { return null; }
function assertEqual(actual, expected, label) {
  if (actual !== expected) throw new Error(label+": got="+JSON.stringify(actual)+" want="+JSON.stringify(expected));
}
(async function() {
  const result = await requestBackendSessionRoutingTurnResolution("session", "pair", {risuUserMessageIndex:12,observedPairOrdinal:6});
  assertEqual(captured.risu_user_message_index, 12, "raw Risu user index is forwarded");
  assertEqual(captured.observed_pair_ordinal, 6, "observed pair ordinal is forwarded");
  assertEqual(Object.prototype.hasOwnProperty.call(captured, "local_turn_index"), false, "adapter must not send a calculated local turn");
  assertEqual(Object.prototype.hasOwnProperty.call(captured, "visible_completed_turns"), false, "adapter must not send a calculated visible turn count");
  assertEqual(result.localTurnIndex, 7, "backend local turn is applied");
})().catch(err => { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Risu index routing runtime fixture failed: %v\n%s", err, out)
	}
}

func TestSessionNormalizeUsesCanonicalRisuChatPairsWithoutLiveFilters(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for canonical Risu chat runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	script := extractArchiveCenterJSFunction(t, src, "computeOrchestrationDirtyHashOr1c") + "\n" +
		extractArchiveCenterJSFunction(t, src, "buildCompletedTurnPairsFromActiveChatMessages") + "\n" +
		extractArchiveCenterJSFunction(t, src, "buildSessionNormalizeCompletedTurnPairs") + `
const ACTIVE_CHAT_BACKFILL_MAX_CONTEXT_MESSAGES = 20;
const AUTO_CONTINUE_USER_INPUT_MARKER = "[continue]";
function extractActiveChatComparableMessages(chat) {
  return chat.message.map(function(item, index) {
    const role = item.role === "user" ? "user" : item.role === "char" ? "assistant" : item.role;
    return {role,content:String(item.data || ""),risuMessageIndex:index,raw:item};
  });
}
function extractComparableMessageRoleAndContent(msg) { return msg; }
function selectBestAssistantCandidateRecord(items) { return items[items.length - 1] || null; }
function normalizeAssistantPersistenceCandidate(text) { return String(text || "").trim(); }
function shouldSkipUserInputPersistence() { return false; }
function shouldSkipTurnPersistenceForOoc() { return false; }
function getActiveChatMessageStreamingState() { return "done"; }
function debugLog() {}
function assertEqual(actual, expected, label) {
  if (actual !== expected) throw new Error(label + ": got=" + JSON.stringify(actual) + " want=" + JSON.stringify(expected));
}
const chat = {message:[
  {role:"user",data:"first user"}, {role:"char",data:"first final output"},
  {role:"user",data:"# SYSTEM is visible user text here"}, {role:"char",data:"second final output"},
  {role:"user",data:"third user"}, {role:"char",data:"third draft"}, {role:"char",data:"third final output"},
  {role:"user",data:"fourth user"}, {role:"char",data:"fourth final output"},
]};
let result = buildSessionNormalizeCompletedTurnPairs(chat);
assertEqual(result.available, true, "canonical chat availability");
assertEqual(result.pairs.length, 4, "four visible turns must remain four turns");
assertEqual(result.pairs[1].userContent, "# SYSTEM is visible user text here", "content filters must not erase canonical raw turns");
assertEqual(result.pairs[2].assistantContent, "third final output", "latest visible assistant content wins within one turn");
assertEqual(JSON.stringify(result.pairs.map(p => p.risuUserMessageIndex)), JSON.stringify([0,2,4,7]), "canonical raw Risu indexes");
assertEqual(JSON.stringify(result.pairs.map(p => p.observedPairOrdinal)), JSON.stringify([1,2,3,4]), "canonical observation order");
assertEqual(result.pairs.some(p => Object.prototype.hasOwnProperty.call(p, "turnIndex")), false, "session normalize adapter does not calculate turns");
result = buildSessionNormalizeCompletedTurnPairs({message:chat.message.concat([{role:"user",data:"unfinished fifth user"}])});
assertEqual(result.pairs.length, 4, "unfinished trailing user must not become a completed turn");
result = buildSessionNormalizeCompletedTurnPairs({messages:chat.message});
assertEqual(result.available, false, "noncanonical fallback must stay explicit");
const longMessages = [];
for (let turn = 1; turn <= 120; turn++) {
  longMessages.push({role:"user",data:"user "+turn}, {role:"char",data:"assistant "+turn});
}
result = buildSessionNormalizeCompletedTurnPairs({message:longMessages});
assertEqual(result.pairs.length, 120, "long canonical chat must not collapse to its recent tail");
assertEqual(result.pairs[0].userContent, "user 1", "long chat first turn");
assertEqual(result.pairs[119].assistantContent, "assistant 120", "long chat last turn");
const indexedGapChat = {message:[
  {role:"user",data:"gap user one"}, {role:"char",data:"gap assistant one"},
  {role:"system",data:"host metadata"}, {role:"system",data:"host metadata 2"},
  {role:"user",data:"gap user three"}, {role:"char",data:"gap assistant three"},
]};
result = buildSessionNormalizeCompletedTurnPairs(indexedGapChat);
assertEqual(JSON.stringify(result.pairs.map(p => p.risuUserMessageIndex)), JSON.stringify([0,4]), "session normalize preserves raw Risu index gaps");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("canonical Risu session-normalize JS fixture failed: %v\n%s", err, out)
	}
}

func TestSessionNormalizeRepairEntriesRestoreTurnZeroAndEveryMissingPair(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for session-normalize repair runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	script := extractArchiveCenterJSFunction(t, src, "sanitizeChatLogRepairEntry") + "\n" +
		extractArchiveCenterJSFunction(t, src, "buildSessionNormalizeRepairEntriesFromDryRunPlan") + `
const STARTUP_MESSAGE_TURN_INDEX = 0;
const STARTUP_MESSAGE_MAX_CHARS = 100000;
function assertEqual(actual, expected, label) {
  if (actual !== expected) throw new Error(label + ": got=" + JSON.stringify(actual) + " want=" + JSON.stringify(expected));
}
const plan = {
  dbRows: [],
  rawMissingTurns: [1,2,3,4],
  pairs: [1,2,3,4].map(turn => ({turnIndex:turn,userContent:"user "+turn,assistantContent:"assistant "+turn})),
};
let entries = buildSessionNormalizeRepairEntriesFromDryRunPlan(plan, {content:"opening story",source:"selected greeting"});
assertEqual(JSON.stringify(entries.map(entry => entry.turn_index)), JSON.stringify([0,1,2,3,4]), "turn zero and all missing visible turns");
assertEqual(entries[0].assistant_content, "opening story", "starter content");
entries = buildSessionNormalizeRepairEntriesFromDryRunPlan({...plan, dbRows:[{turn_index:0,role:"assistant",content:"already stored"}]}, {content:"opening story"});
assertEqual(JSON.stringify(entries.map(entry => entry.turn_index)), JSON.stringify([1,2,3,4]), "existing starter must not be replayed");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("session-normalize repair JS fixture failed: %v\n%s", err, out)
	}
}

func TestSelectedRisuGreetingBecomesTurnZero(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for Risu starter runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functions := []string{
		extractArchiveCenterJSFunction(t, src, "computeOrchestrationDirtyHashOr1c"),
		extractArchiveCenterJSFunction(t, src, "normalizeStartupMessageContent"),
		extractArchiveCenterJSFunction(t, src, "buildStartupMessageTurnZeroCandidate"),
		extractArchiveCenterJSFunction(t, src, "getStringFieldValue"),
		extractArchiveCenterJSFunction(t, src, "buildStartupMessageTurnZeroCandidateFromText"),
		extractArchiveCenterJSFunction(t, src, "resolveSelectedStartupMessageTurnZeroCandidate"),
	}
	script := strings.Join(functions, "\n\n") + `
const STARTUP_MESSAGE_TURN_INDEX = 0;
const STARTUP_MESSAGE_MAX_CHARS = 20000;
function assertEqual(actual, expected, label) {
  if (actual !== expected) throw new Error(label + ": got=" + JSON.stringify(actual) + " want=" + JSON.stringify(expected));
}
const character = {firstMessage:"default starter", alternateGreetings:["alternate zero", "alternate one"]};
let candidate = resolveSelectedStartupMessageTurnZeroCandidate(character, {fmIndex:1, message:[]});
assertEqual(candidate.turnIndex, 0, "starter turn index");
assertEqual(candidate.content, "alternate one", "selected alternate greeting");
candidate = resolveSelectedStartupMessageTurnZeroCandidate(character, {fmIndex:-1, message:[]});
assertEqual(candidate.content, "default starter", "default first message");
candidate = buildStartupMessageTurnZeroCandidate([
  {role:"assistant",content:"candidate one"},
  {role:"assistant",content:"candidate two"},
  {role:"user",content:"first user"},
]);
assertEqual(candidate, null, "multiple leading assistant candidates must not be concatenated");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Risu selected starter JS runtime fixture failed: %v\n%s", err, out)
	}
}

func TestExistingMigratedTurnZeroSuppressesAutomaticStarterAppend(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for migrated starter runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	script := extractArchiveCenterJSFunction(t, src, "chatLogItemsContainRole") + "\n" +
		extractArchiveCenterJSAsyncFunction(t, src, "ensureStartupMessageTurnZeroSaved") + `
const settings = {enabled:true,dbEnabled:true};
const SESSION_FALLBACK = "default";
const STARTUP_MESSAGE_TURN_INDEX = 0;
const _startupMessageSaveInFlight = new Set();
let marked = 0;
let saved = 0;
async function getCurrentStartupMessageTurnZeroCandidate() { return {content:"target visible starter",hash:"target-hash"}; }
async function loadStartupMessageLedger() { return {entries:{}}; }
async function fetchCanonicalChatLogsForTurn() { return [{turn_index:0,role:"assistant",content:"copied source starter"}]; }
async function markStartupMessageLedgerSaved() { marked++; }
async function saveStartupMessageTurnZeroToBackend() { saved++; return true; }
function assertEqual(actual, expected, label) {
  if (actual !== expected) throw new Error(label + ": got=" + JSON.stringify(actual) + " want=" + JSON.stringify(expected));
}
(async function() {
  const result = await ensureStartupMessageTurnZeroSaved("target-session", []);
  assertEqual(result, false, "existing migrated turn zero result");
  assertEqual(marked, 1, "existing migrated turn zero marks local ledger");
  assertEqual(saved, 0, "target visible starter must not append over migrated turn zero");
})().catch(err => { console.error(err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("migrated starter JS fixture failed: %v\n%s", err, out)
	}
}

func TestRisuLogicalTurnReservationUsesImportedSessionBaseline(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for Risu imported baseline runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functionBody := extractArchiveCenterJSAsyncFunction(t, src, "reserveAfterRequestPersistenceTurnIndex")
	script := functionBody + `
let exactTurn = 0;
let fixture = null;
let rollbackCall = null;
const lastOrchResult = null;
async function safeCall(fn, fallback) { try { return await fn(); } catch { return fallback; } }
async function fetchBackendLatestTurnIndexForSession() { return fixture.backendLatest; }
function setTurnCounterAtLeast() {}
function peekNextTurnIndex() { return fixture.backendLatest + 1; }
async function findActiveChatCompletedTurnPairForContent() {
  return {observedPairOrdinal:fixture.ordinal || 1,pairCount:fixture.ordinal || 1,userContent:fixture.user,assistantContent:fixture.assistant,risuUserMessageIndex:fixture.userIndex || 0,risuAssistantMessageIndex:(fixture.userIndex || 0)+1,source:"active_chat_user_assistant_pair"};
}
function normalizeTurnPairCompareText(text) { return String(text || "").trim(); }
async function findActiveChatCompletedTurnPairForUserContent() { return null; }
async function findLatestActiveChatCompletedTurnPair() { return null; }
async function requestBackendSessionRoutingTurnResolution(_sid, mode, observation) {
  if (mode !== "pair" || observation.risuUserMessageIndex !== (fixture.userIndex || 0) || observation.observedPairOrdinal !== (fixture.ordinal || 1)) {
    throw new Error("unexpected routing request: "+JSON.stringify(observation));
  }
  return fixture.routing || {status:"backend_unavailable",turnIndex:0,baseline:null};
}
async function fetchCanonicalChatLogsForTurn() { return fixture.existing || null; }
function chatLogItemsContainRoleContent(items, role, content) { return items.some(item => item.role === role && item.content === content); }
async function executeAutoRollback(sid, turn, reason) { rollbackCall = {sid,turn,reason}; return true; }
function setTurnCounterExact(_sid, turn) { exactTurn = turn; }
function nextTurnIndex() { return 99; }
function debugLog() {}
(async function() {
  for (const backendLatest of [1, 5, 13, 34]) {
    fixture = {backendLatest,user:"u"+backendLatest,assistant:"a"+backendLatest};
    exactTurn = 0;
    const turn = await reserveAfterRequestPersistenceTurnIndex("s", fixture.user, fixture.assistant);
    const expected = backendLatest + 1;
    if (turn !== expected || exactTurn !== expected) {
      throw new Error("unavailable routing baseline must append after backend tail: backend="+backendLatest+" got="+turn);
    }
  }
  fixture = {
    backendLatest:3,
    user:"rerolled user",
    assistant:"new assistant",
    userIndex:4,
    ordinal:3,
    routing:{status:"normal",turnIndex:3,localTurnIndex:3,baseline:null},
    existing:[{role:"user",content:"rerolled user"},{role:"assistant",content:"old assistant"}],
  };
  rollbackCall = null;
  exactTurn = 0;
  const rerolledTurn = await reserveAfterRequestPersistenceTurnIndex("s", fixture.user, fixture.assistant);
  if (rerolledTurn !== 3 || exactTurn !== 3) throw new Error("reroll must reuse backend turn 3");
  if (!rollbackCall || rollbackCall.turn !== 3 || rollbackCall.reason !== "risu_message_index_turn_replaced") {
    throw new Error("changed content must reconcile the existing logical turn: "+JSON.stringify(rollbackCall));
  }
})().catch(function(err) {
  console.error(err && err.stack || err);
  process.exit(1);
});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Risu imported baseline JS runtime fixture failed: %v\n%s", err, out)
	}
}

func TestEmptyActiveChatCanVerifyFirstTurnDeletionFromLedger(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for empty active chat rollback fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functionBody := extractArchiveCenterJSFunction(t, src, "buildLedgerVerifiedTailRollback")
	script := functionBody + `
function loadRollbackTurnLedgerOr1f() {
  return {trackedTurnIndex:1,entries:[{role:"user",content:"u1"},{role:"assistant",content:"a1"}]};
}
function compactSnapshotMessages(messages) { return Array.isArray(messages) ? messages : []; }
function computeLedgerCurrentPrefixLengthOr1f(_ledger, current) { return current.length; }
function computeTailHash() { return "empty"; }
function debugLog() {}
const result = buildLedgerVerifiedTailRollback("s", [], 0, 1);
if (!result || result.status !== "verified_tail_delete" || result.rollbackFrom !== 1 || result.removedAssistantCount !== 1) {
  throw new Error("empty active chat deletion was not verified: " + JSON.stringify(result));
}
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("empty active chat rollback JS runtime fixture failed: %v\n%s", err, out)
	}
}

func TestRollbackReadsCanonicalRisuChatAfterDeletion(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for canonical rollback chat fixture")
		}
	}
	src := readArchiveCenterJS(t)
	script := extractArchiveCenterJSAsyncFunction(t, src, "resolveCurrentActiveChatObject") + "\n" +
		extractArchiveCenterJSAsyncFunction(t, src, "getCurrentActiveChatRollbackMessages") + `
const R = {
  getCurrentCharacterIndex: async () => 4,
  getCurrentChatIndex: async () => 2,
  getChatFromIndex: async () => ({message:[]}),
};
function parseSessionDisplayIdentity() { return null; }
function extractActiveChatRollbackMessages(chat) {
  return (chat && Array.isArray(chat.message) ? chat.message : []).map(item => ({role:item.role,content:item.data}));
}
function debugLog() {}
(async function() {
  const messages = await getCurrentActiveChatRollbackMessages();
  if (!Array.isArray(messages) || messages.length !== 0) {
    throw new Error("rollback used stale character cache instead of canonical empty chat: " + JSON.stringify(messages));
  }
})().catch(function(err) {
  console.error(err && err.stack || err);
  process.exit(1);
});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("canonical rollback chat JS runtime fixture failed: %v\n%s", err, out)
	}
}

func TestCurrentChatIndexFailureFallsBackOnlyToIdentityMatchedCurrentCharacter(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for current chat index fallback fixture")
		}
	}
	src := readArchiveCenterJS(t)
	script := extractArchiveCenterJSFunction(t, src, "resolveIdentityVerifiedCurrentCharacterChat") + "\n" +
		extractArchiveCenterJSFunction(t, src, "parseSessionDisplayIdentity") + "\n" +
		extractArchiveCenterJSAsyncFunction(t, src, "resolveCurrentActiveChatObject") + `
let getCharacterCalls = 0;
let activeChatId = "target";
const R = {
  getCurrentCharacterIndex: async () => 4,
  getCurrentChatIndex: async () => 2,
  getChatFromIndex: async () => { throw new Error("index read unavailable"); },
  getCharacter: async () => {
    getCharacterCalls += 1;
    return {chatPage: 2, chats: [{}, {}, {id:activeChatId, message:[{role:"assistant", data:"current output"}]}]};
  },
};
function debugLog() {}
(async function() {
  const resolved = await resolveCurrentActiveChatObject("char_4_cid_target");
  if (!resolved.chat || resolved.source !== "R.getCharacter.identity_verified") {
    throw new Error("identity-matched current chat was not recovered: " + JSON.stringify(resolved));
  }
  activeChatId = "different-chat";
  const mismatched = await resolveCurrentActiveChatObject("char_4_cid_target");
  if (mismatched.chat !== null || mismatched.source !== "none") {
    throw new Error("CID-mismatched current chat was reused: " + JSON.stringify(mismatched));
  }
  const wrongCharacter = await resolveCurrentActiveChatObject("char_9_cid_target");
  if (wrongCharacter.chat !== null || wrongCharacter.source !== "none") {
    throw new Error("character-mismatched current chat was reused: " + JSON.stringify(wrongCharacter));
  }
  if (getCharacterCalls !== 2) {
    throw new Error("getCharacter fallback call count=" + getCharacterCalls);
  }
})().catch(function(err) {
  console.error(err && err.stack || err);
  process.exit(1);
});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("current chat index fallback JS runtime fixture failed: %v\n%s", err, out)
	}
}

func TestNormalSessionFinalOutputRecoverySurvivesCurrentChatIndexReadFailure(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for normal-session final-output recovery fixture")
		}
	}
	src := readArchiveCenterJS(t)
	script := extractArchiveCenterJSFunction(t, src, "resolveIdentityVerifiedCurrentCharacterChat") + "\n" +
		extractArchiveCenterJSFunction(t, src, "parseSessionDisplayIdentity") + "\n" +
		extractArchiveCenterJSAsyncFunction(t, src, "resolveCurrentActiveChatObject") + "\n" +
		extractArchiveCenterJSAsyncFunction(t, src, "recoverAssistantContentFromActiveChat") + `
const R = {
  getCurrentCharacterIndex: async () => 4,
  getCurrentChatIndex: async () => 2,
  getChatFromIndex: async () => { throw new Error("transient index read failure"); },
  getCharacter: async () => ({
    chatPage: 2,
    chats: [{}, {}, {
      id: "target",
      message: [
        {role: "user", data: "normal user input"},
        {role: "assistant", data: "normal final assistant output"},
      ],
    }],
  }),
};
function extractActiveChatComparableMessages(chat) {
  return (chat && Array.isArray(chat.message) ? chat.message : []).map(function(item, index) {
    return {role: item.role, content: item.data, risuMessageIndex: index};
  });
}
function buildCompletedTurnPairsFromActiveChatMessages(messages) {
  return [{
    userContent: messages[0].content,
    assistantContent: messages[1].content,
  }];
}
function normalizeMainTurnCompareText(value) { return String(value || "").trim(); }
function mainTurnTextMatchesOriginal(left, right) { return normalizeMainTurnCompareText(left) === normalizeMainTurnCompareText(right); }
function normalizeAssistantPersistenceCandidate(value) { return String(value || "").trim(); }
function isAssistantPrefillSeedText() { return false; }
function getSessionSnapshot() { return null; }
function getLastNonEmptyAssistantComparableContent() { return ""; }
function isSameAssistantComparableText(left, right) { return left === right; }
function debugLog() {}
(async function() {
  const recovered = await recoverAssistantContentFromActiveChat(
    "char_4_cid_target",
    null,
    "normal user input"
  );
  if (recovered !== "normal final assistant output") {
    throw new Error("normal-session final output was not recovered: " + JSON.stringify(recovered));
  }
})().catch(function(err) {
  console.error(err && err.stack || err);
  process.exit(1);
});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("normal-session final-output recovery fixture failed: %v\n%s", err, out)
	}
}

func TestCopiedEightPlusOneDeletionKeepsImportedEightTurns(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for copied-session rollback fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functionBody := extractArchiveCenterJSAsyncFunction(t, src, "reconcileActiveChatTailDeletionWithBackend")
	script := functionBody + `
const settings = {enabled:true,dbEnabled:true};
const SESSION_FALLBACK = "default";
const ROLLBACK_TAIL_RECONCILE_INTERVAL_MS = 0;
const ROLLBACK_TAIL_RECONCILE_MAX_BLIND_GAP_TURNS = 2;
let _rollbackTailReconcileInFlight = false;
let _rollbackTailReconcileLastAt = 0;
let _lastAutoRollbackSignature = null;
let rollbackFrom = 0;
function hasRecentPromotedAssistantSyncPending() { return false; }
function extractActiveChatMessageList(chat) { return chat.message; }
function extractActiveChatComparableMessages(chat) { return chat.message; }
function buildCompletedTurnPairsFromActiveChatMessages() { return []; }
async function requestBackendSessionRoutingTurnResolution() {
  return {completedTurnCount:8,baseline:{backendTurnAtRoute:8,localPairCountAtRoute:0,reason:"timeline_copy"}};
}
async function fetchBackendLatestTurnIndexForSession() { return 9; }
function buildLedgerVerifiedTailRollback() { return null; }
function getRecentRisuHistoryTrimGuard() { return null; }
function updateRuntimeState() {}
async function executeAutoRollback(_sid, turn) { rollbackFrom = turn; return true; }
function updateSessionSnapshot() {}
function debugLog() {}
(async function() {
  const ok = await reconcileActiveChatTailDeletionWithBackend("copy-target", {message:[]}, {force:true});
  if (!ok || rollbackFrom !== 9) {
    throw new Error("8+1 -> 8+0 must delete only turn 9: ok=" + ok + " rollbackFrom=" + rollbackFrom);
  }
})().catch(function(err) {
  console.error(err && err.stack || err);
  process.exit(1);
});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("copied-session rollback JS runtime fixture failed: %v\n%s", err, out)
	}
}

func TestPostprocessorReplacementRebuildsDeletionSnapshot(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for postprocessor snapshot fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functionBody := extractArchiveCenterJSAsyncFunction(t, src, "replacePersistedTurnWithPostOutputFinal")
	script := functionBody + `
let snapshotMessages = null;
async function resolvePostOutputFinalAssistant() { return "final output"; }
function normalizeMainTurnCompareText(text) { return String(text || "").trim(); }
function normalizeAssistantPersistenceCandidate(text) { return String(text || "").trim(); }
function isSameAssistantComparableText(left, right) { return left === right; }
async function findRecentPersistedCompleteTurnPairForContent() { return {turnIndex:9,latestBackendTurn:9}; }
async function buildCompleteTurnRequestBody() { return {client_meta:{}}; }
function computeAssistantSnapshotFingerprint() { return "old"; }
function buildCompleteTurnQueuePayload() { return null; }
function enqueue() {}
async function flushQueueSave() {}
async function executeAutoRollback() { return true; }
async function tryCompleteTurn() { return {status:"ok",save_ok:true}; }
function removeQueuedItem() { return false; }
function trackTurnIndex() {}
async function getCurrentActiveChatComparableMessages() {
  return [{role:"user",content:"player input"},{role:"assistant",content:"final output"}];
}
function updateSessionSnapshot(_sid, messages) { snapshotMessages = messages; }
function upsertTimelineCompleteTurnPendingArtifacts() {}
function scheduleTimelinePostCompleteTurnRefresh() {}
(async function() {
  const result = await replacePersistedTurnWithPostOutputFinal("s", {
    postOutputReplacement:{userContent:"player input",assistantContent:"draft output",contextMessages:[]}
  }, "final output");
  if (!result || !result.replaced) throw new Error("postprocessor replacement did not complete");
  if (!Array.isArray(snapshotMessages) || snapshotMessages.length !== 2 || snapshotMessages[1].content !== "final output") {
    throw new Error("postprocessor replacement did not rebuild final deletion snapshot: " + JSON.stringify(snapshotMessages));
  }
})().catch(function(err) {
  console.error(err && err.stack || err);
  process.exit(1);
});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("postprocessor snapshot JS runtime fixture failed: %v\n%s", err, out)
	}
}
