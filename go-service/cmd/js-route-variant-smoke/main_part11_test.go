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
], "previous player input", "genuine next player input", [
  {role:"user", content:"previous player input"},
  {role:"assistant", content:"previous assistant output"}
]);
assertEqual(genuineNextTurn.allowed, true, "new raw input must win over stale post-output context");
assertEqual(genuineNextTurn.reason, "raw_input_tail_match", "new raw input ownership reason");

const realPostprocessor = buildMainRequestOwnershipDecision("model", [
  {role:"user", content:"rewrite the previous assistant output"}
], "previous player input", "", [
  {role:"user", content:"previous player input"},
  {role:"assistant", content:"previous assistant output"}
]);
assertEqual(realPostprocessor.allowed, false, "postprocessor request remains excluded from a new turn");
assertEqual(realPostprocessor.reason, "post_output_secondary_request", "postprocessor ownership reason");
`

	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Archive Center core regression JS runtime fixture failed: %v\n%s", err, out)
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
