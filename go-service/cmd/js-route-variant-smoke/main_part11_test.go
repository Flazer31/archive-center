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

func TestResolveCurrentActiveChatObjectPrefersOfficialDirectChatAPI(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for Archive Center active chat runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	fn := extractArchiveCenterJSFunction(t, src, "resolveCurrentActiveChatObject")
	script := fn + `
let characterFallbackCalls = 0;
function parseSessionDisplayIdentity() { return null; }
function extractActiveChatMessageCount(chat) { return Array.isArray(chat && chat.messages) ? chat.messages.length : 0; }
function resolveActiveChatFromCharacter(char) { return char && Array.isArray(char.chats) ? char.chats[0] : null; }
function getRisuCharacterListSnapshot() { return []; }
function debugLog() {}
let R = {
  async getCurrentChatIndex() { return 2; },
  async getCurrentCharacterIndex() { return 3; },
  async getChatFromIndex(charIndex, chatIndex) {
    if (charIndex !== 3 || chatIndex !== 2) throw new Error("wrong direct chat indices");
    return {messages:[{role:"user",content:"one"},{role:"assistant",content:"two"}]};
  },
  async getCharacter() { characterFallbackCalls++; return {chats:[{messages:[{role:"user",content:"fallback"}]}]}; }
};
(async function() {
  const direct = await resolveCurrentActiveChatObject("fixture-session");
  if (direct.source !== "R.getChatFromIndex") throw new Error("direct chat API was not preferred: " + direct.source);
  if (characterFallbackCalls !== 0) throw new Error("character fallback ran despite a valid direct chat");

  R.getChatFromIndex = async function() { throw new Error("unsupported"); };
  const fallback = await resolveCurrentActiveChatObject("fixture-session");
  if (fallback.source !== "R.getCharacter") throw new Error("legacy fallback was not preserved: " + fallback.source);
  if (characterFallbackCalls !== 1) throw new Error("character fallback call count mismatch");
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Archive Center direct active chat runtime fixture failed: %v\n%s", err, out)
	}
}
