package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/httpapi"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestSourceDiscoveryRendersRecordedBridgeFailure(t *testing.T) {
	src := readArchiveCenterJS(t)
	fn := extractArchiveCenterJSAsyncFunction(t, src, "referenceDiscoveryRunFromUI")
	for _, expected := range []string{
		`_lastBridgeFailureByPath.get("/source-discovery/preview/v1")`,
		`_lastBridgeFailureByPath.get("/source-discovery/jobs/v1")`,
		`String(failure.detail`,
	} {
		if !strings.Contains(fn, expected) {
			t.Fatalf("source discovery transport failure detail missing %q", expected)
		}
	}
}

func TestGuideNoneSkipsSupervisorCallRuntime(t *testing.T) {
	src := readArchiveCenterJS(t)
	if strings.Contains(src, "await runSupervisor(") || strings.Contains(src, `bridgeFetch("/supervisor"`) {
		t.Fatal("JavaScript must not perform a supervisor call outside /prepare-turn")
	}
	for _, marker := range []string{
		`const guideDisabled = normalizeNarrativeGuideStrength(settings.narrativeGuideStrength) === "none";`,
		`supervisor_enabled: !guideDisabled`,
		`guide_strength: settings.narrativeGuideStrength || "weak"`,
	} {
		if !strings.Contains(src, marker) {
			t.Fatalf("guide-off /prepare-turn gate marker missing %q", marker)
		}
	}
}

func TestLanguageContextIgnoresRisuPromptScaffoldAssistantRuntime(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for language-source runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	script := extractArchiveCenterJSFunction(t, src, "normalizeLanguageCodeForTrace") + "\n" +
		extractArchiveCenterJSFunction(t, src, "detectTextLanguageForTrace") + "\n" +
		extractArchiveCenterJSFunction(t, src, "detectRecentAssistantOutputLanguage") + "\n" +
		extractArchiveCenterJSFunction(t, src, "isSupportedMemoryLanguageCode") + "\n" +
		extractArchiveCenterJSFunction(t, src, "buildLanguageFallbackChain") + "\n" +
		extractArchiveCenterJSAsyncFunction(t, src, "buildLanguageContextTrace") + `
const LANGUAGE_MEMORY_CONTRACT_VERSION = "language_memory.v1";
const LANGUAGE_MEMORY_SEARCH_TEXT_POLICY = "summary_plus_raw_plus_aliases";
const settings = {uiLanguage:"en"};
function getPayloadMessageRoleAndText(message) { return {role:String(message.role||""),text:String(message.content||"")}; }
function isMetaPromptLikeMessage(text) { return /^system\s*:/i.test(String(text||"").trim()); }
async function resolveRuntimeOutputLanguageOverride() { return ""; }
function normalizeLanguageContextTrace(value) { return value; }
(async function() {
  const scaffoldOnly = await buildLanguageContextTrace({
    userInput:"한얼은 숯불에 손을 다쳤다.",
    messages:[{role:"assistant",content:"system: POV instructions. Respond in Korean after reviewing these English instructions."}],
    stage:"beforeRequest"
  });
  if (scaffoldOnly.session_output_language !== "ko" || scaffoldOnly.output_language_source !== "current_user") {
    throw new Error("Risu scaffold contaminated language source: "+JSON.stringify(scaffoldOnly));
  }
  const realAssistant = await buildLanguageContextTrace({
    userInput:"한국어로 쓴 입력",
    messages:[{role:"assistant",content:"The previous actual assistant response remains in English."}],
    stage:"beforeRequest"
  });
  if (realAssistant.session_output_language !== "en" || realAssistant.output_language_source !== "recent_assistant") {
    throw new Error("real assistant continuity language was not preserved: "+JSON.stringify(realAssistant));
  }
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("language-source JS fixture failed: %v\n%s", err, out)
	}
}

func TestFinalPayloadParitySeparatesActualUserFromRisuPromptTailRuntime(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for final-payload parity runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	hashFn := extractArchiveCenterJSFunction(t, src, "computeOrchestrationDirtyHashOr1c")
	fn := extractArchiveCenterJSFunction(t, src, "buildFinalPayloadParityTrace")
	script := hashFn + "\n" + fn + `
const settings = {pluginMainApplyMode:"shadow",pluginMainRewriteLegacyOptIn:false};
function extractMessages(payload) { return {messages:payload.messages||[]}; }
function findLastPayloadMessage(messages, role) { for (let i=messages.length-1;i>=0;i--) if (!role || messages[i].role===role) return messages[i]; return null; }
function truncPreview(value, limit) { return String(value||"").slice(0,limit); }
function isMetaUserMessage(value) { return /^system\s*:/i.test(String(value||"").trim()); }
const payload = {messages:[{role:"user",content:"system: POV instructions and host prompt"}]};
const trace = buildFinalPayloadParityTrace(payload, payload, {
  chatSessionId:"session-copy", userInputSource:"active_chat:0", effectiveUserInput:"한얼은 숯불에 손을 다쳤다.",
  applyMode:{mode:"shadow",payloadReplaced:false},payloadMutated:false
});
if (trace.finalUserInputPreview !== "한얼은 숯불에 손을 다쳤다.") throw new Error("actual user preview was replaced by host prompt: "+JSON.stringify(trace));
if (trace.payloadUserRoleTailKind !== "risu_host_prompt_scaffold") throw new Error("Risu payload tail was not classified separately: "+JSON.stringify(trace));
if (!trace.capturedBeforeRequestReturn || !trace.outboundPayloadHash) throw new Error("pre-request fingerprint missing: "+JSON.stringify(trace));
const mismatch = buildFinalPayloadParityTrace(payload, payload, {
  effectiveInputText:"required auxiliary", injectionResult:{mainInjectionPreview:"required auxiliary"}
});
if (mismatch.status !== "mismatch" || mismatch.payloadContentMatch !== false) throw new Error("missing payload component was accepted: "+JSON.stringify(mismatch));
const matchedPayload = {messages:[{role:"system",content:"host scaffold\nrequired auxiliary"}]};
const matched = buildFinalPayloadParityTrace(payload, matchedPayload, {
  effectiveInputText:"required auxiliary", injectionResult:{mainInjectionPreview:"required auxiliary"}
});
if (matched.status !== "ready" || matched.payloadContentMatch !== true) throw new Error("present payload component was rejected: "+JSON.stringify(matched));
if (!matched.effectiveInputHash || !matched.outboundPayloadHash) throw new Error("non-empty verified input fingerprint missing: "+JSON.stringify(matched));
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("final-payload parity JS fixture failed: %v\n%s", err, out)
	}
}

func TestSourceDiscoveryUsesSelectedWorkWithoutDuplicateTitleInput(t *testing.T) {
	src := readArchiveCenterJS(t)
	remember := extractArchiveCenterJSFunction(t, src, "referenceDiscoveryRememberDraft")
	panel := extractArchiveCenterJSFunction(t, src, "renderReferenceCanonPackPanel")
	library := extractArchiveCenterJSFunction(t, src, "renderReferenceLibrarySection")
	timeout := extractArchiveCenterJSFunction(t, src, "getSourceDiscoveryRequestTimeoutMs")
	if !strings.Contains(remember, `state.works.find`) || strings.Contains(remember, `mo-discovery-work-query`) {
		t.Fatalf("discovery draft must derive the title from the selected work: %s", remember)
	}
	if !strings.Contains(panel, `대상 작품`) || strings.Contains(panel, `id="mo-discovery-work-query"`) {
		t.Fatalf("discovery panel still exposes a duplicate title input: %s", panel)
	}
	if !strings.Contains(library, `tabs + selector + renderReferenceCanonPackPanel()`) {
		t.Fatalf("work selector is missing from the discovery panel")
	}
	if !strings.Contains(timeout, `resolveRequestTimeoutMs(600000)`) {
		t.Fatalf("source discovery transport must outlive the bounded backend pipeline: %s", timeout)
	}
}

func TestSourceDiscoveryDisplaysRetainedDocumentsAndDoesNotHidePartialCandidates(t *testing.T) {
	src := readArchiveCenterJS(t)
	run := extractArchiveCenterJSAsyncFunction(t, src, "referenceDiscoveryRunFromUI")
	render := extractArchiveCenterJSFunction(t, src, "renderReferenceLibrarySection")
	if !strings.Contains(run, `else if (candidateCount === 0)`) || strings.Contains(run, `insufficient_source_coverage" || candidateCount === 0`) {
		t.Fatalf("partial discovery candidates are still hidden by coverage state: %s", run)
	}
	for _, expected := range []string{"원문 문서", "raw_retention", "raw_text_length", `libraryView === "documents"`} {
		if !strings.Contains(render, expected) {
			t.Fatalf("retained source document UI missing %q", expected)
		}
	}
	panel := extractArchiveCenterJSFunction(t, src, "renderReferenceCanonPackPanel")
	for _, expected := range []string{`documents_retained`, `documents_upgraded`, `원문 DB 저장 실패`} {
		if !strings.Contains(panel, expected) {
			t.Fatalf("source body staging diagnostics missing %q", expected)
		}
	}
}

func TestRetainedDocumentsExposeIndependentCriticActions(t *testing.T) {
	src := readArchiveCenterJS(t)
	render := extractArchiveCenterJSFunction(t, src, "renderReferenceLibrarySection")
	start := extractArchiveCenterJSAsyncFunction(t, src, "referenceLibraryStartExtraction")
	for _, expected := range []string{`data-reference-document-extract`, `mo-reference-document`, `mo-reference-document-action`, `평론가 정밀 분석`, `평론가 다시 분석`} {
		if !strings.Contains(render, expected) {
			t.Fatalf("per-document critic UI missing %q", expected)
		}
	}
	for _, expected := range []string{`selectedDocumentId`, `"/documents/" + referenceLibraryPath(documentId) + "/extract"`, `reference_document_extract`} {
		if !strings.Contains(start, expected) {
			t.Fatalf("per-document extraction request missing %q", expected)
		}
	}
	if !strings.Contains(src, `referenceLibraryStartExtraction(button.getAttribute("data-reference-document-extract"))`) {
		t.Fatalf("per-document critic button is not connected to its document ID")
	}
}

func TestSourceDiscoveryResumesStoredAnalysisWithoutStartingAnotherSearch(t *testing.T) {
	src := readArchiveCenterJS(t)
	loadLatest := extractArchiveCenterJSAsyncFunction(t, src, "referenceDiscoveryLoadLatestJob")
	resume := extractArchiveCenterJSAsyncFunction(t, src, "referenceDiscoveryResumeFromUI")
	render := extractArchiveCenterJSFunction(t, src, "renderReferenceCanonPackPanel")
	if !strings.Contains(loadLatest, `/source-discovery/latest-job/v1`) || !strings.Contains(loadLatest, `state.discoveryJob = data && data.job_id ? data : null`) {
		t.Fatalf("latest discovery job is not restored after plugin reload: %s", loadLatest)
	}
	for _, expected := range []string{`/complete/v1`, `source_discovery_corpus_analysis`, `referenceLibraryPollJob`} {
		if !strings.Contains(resume, expected) {
			t.Fatalf("source discovery resume adapter missing %q", expected)
		}
	}
	for _, expected := range []string{`mo-discovery-resume`, `remaining_document_count`, `duplicate_analysis_sections`} {
		if !strings.Contains(render, expected) {
			t.Fatalf("source discovery resume UI missing %q", expected)
		}
	}
	if !strings.Contains(render, `남은 원문 통합 분석`) || !strings.Contains(render, `원문 수집 완료 · 통합 분석 대기`) || !strings.Contains(render, `discoveryPlannedBatches`) {
		t.Fatalf("source discovery does not expose one-click corpus analysis: %s", render)
	}
	if strings.Contains(resume, `/source-discovery/jobs/v1`) {
		t.Fatalf("resume must not start another search job: %s", resume)
	}
}

func extractArchiveCenterJSFunction(t *testing.T, src, name string) string {
	t.Helper()
	marker := "  function " + name + "("
	start := strings.Index(src, marker)
	if start < 0 {
		t.Fatalf("Archive Center.js function %s not found", name)
	}
	nextFunction := strings.Index(src[start+len(marker):], "\n  function ")
	nextAsyncFunction := strings.Index(src[start+len(marker):], "\n  async function ")
	next := nextFunction
	if next < 0 || (nextAsyncFunction >= 0 && nextAsyncFunction < next) {
		next = nextAsyncFunction
	}
	if next < 0 {
		t.Fatalf("Archive Center.js function %s has no following function boundary", name)
	}
	end := start + len(marker) + next
	return strings.TrimSpace(src[start:end])
}

func TestPrepareTurnEmptyObservationProductionJSAndGoRoute(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for Archive Center prepare-turn observation runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	hashFunction := extractArchiveCenterJSFunction(t, src, "computeOrchestrationDirtyHashOr1c")
	observationFunction := extractArchiveCenterJSFunction(t, src, "buildPrepareTurnSourceObservations")
	script := hashFunction + "\n" + observationFunction + `
const result = {
  empty: buildPrepareTurnSourceObservations("session-a", "request-empty", 0, "user", "", '["messages"]', "chat-a"),
  non_empty: buildPrepareTurnSourceObservations("session-a", "request-non-empty", 0, "user", "hello", '["messages"]', "chat-a")
};
process.stdout.write(JSON.stringify(result));
`
	command := exec.Command(nodePath, "-e", script)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("prepare-turn observation runtime fixture failed: %v\n%s", err, output)
	}
	var observations map[string]map[string]any
	if err := json.Unmarshal(output, &observations); err != nil {
		t.Fatalf("decode production JS observations: %v\n%s", err, output)
	}
	emptySource := observations["empty"]["sourceObservation"].(map[string]any)
	emptyCapabilities := observations["empty"]["capabilityObservation"].(map[string]any)
	if _, exists := emptySource["raw_input_hash"]; exists {
		t.Fatalf("empty observation must omit raw_input_hash: %+v", emptySource)
	}
	if got := emptyCapabilities["capabilities"].(map[string]any)["raw_input_hash"]; got != "unavailable" {
		t.Fatalf("empty hash capability=%v, want unavailable", got)
	}
	nonEmptySource := observations["non_empty"]["sourceObservation"].(map[string]any)
	nonEmptyCapabilities := observations["non_empty"]["capabilityObservation"].(map[string]any)
	if hash, _ := nonEmptySource["raw_input_hash"].(string); hash == "" {
		t.Fatalf("non-empty observation lost hash: %+v", nonEmptySource)
	}
	if got := nonEmptyCapabilities["capabilities"].(map[string]any)["raw_input_hash"]; got != "observed" {
		t.Fatalf("non-empty hash capability=%v, want observed", got)
	}

	server := httpapi.NewServer(config.Default())
	server.Store = store.NewNoopStore()
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)
	requestPrepare := func(t *testing.T, rawInput string, observation map[string]any) map[string]any {
		t.Helper()
		body, err := json.Marshal(map[string]any{
			"chat_session_id":        "session-a",
			"raw_user_input":         rawInput,
			"messages":               []map[string]any{{"role": "user", "content": rawInput}},
			"source_observation":     observation["sourceObservation"],
			"capability_observation": observation["capabilityObservation"],
		})
		if err != nil {
			t.Fatal(err)
		}
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader(body)))
		if recorder.Code != http.StatusOK {
			t.Fatalf("prepare-turn status=%d body=%s", recorder.Code, recorder.Body.String())
		}
		var response map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		return response["source_contract"].(map[string]any)["lane_status"].(map[string]any)
	}
	if lane := requestPrepare(t, "", observations["empty"]); lane["status"] != "empty" || lane["reason_code"] != "source_observation_empty" {
		t.Fatalf("empty production lane=%+v", lane)
	}
	if lane := requestPrepare(t, "hello", observations["non_empty"]); lane["status"] != "eligible" || lane["reason_code"] != "source_observation_eligible" {
		t.Fatalf("non-empty production lane=%+v", lane)
	}
	malformed := map[string]any{}
	encodedEmpty, err := json.Marshal(observations["empty"])
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encodedEmpty, &malformed); err != nil {
		t.Fatal(err)
	}
	malformedCapabilities := malformed["capabilityObservation"].(map[string]any)["capabilities"].(map[string]any)
	malformedCapabilities["raw_input_hash"] = "observed"
	if lane := requestPrepare(t, "", malformed); lane["status"] != "failed" || lane["reason_code"] != "source_observation_malformed" {
		t.Fatalf("malformed observed-hash lane=%+v", lane)
	}
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

func TestPrepareTurnSourceCapabilityContractRuntime(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for Archive Center prepare-turn runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	fn := extractArchiveCenterJSFunction(t, src, "adaptiveInjectionBudgetProfileLimits") + "\n" +
		extractArchiveCenterJSFunction(t, src, "adaptiveInjectionAutomaticCap") + "\n" +
		extractArchiveCenterJSFunction(t, src, "estimateContextGrowthInjectionBudget") + "\n" +
		extractArchiveCenterJSFunction(t, src, "estimateAdaptiveInjectionBudgetParts") + "\n" +
		extractArchiveCenterJSAsyncFunction(t, src, "tryPrepareTurn")
	script := fn + `
const settings = {
  narrativeGuideMode: "off", narrativeGuideStrength: "weak", storyNarrativeStance: "balanced",
  pluginMainApplyMode: "shadow", inputContextEnabled: true, maxInjectionChars: 1000, injectionBudgetExtraChars: 0,
  topK: 3, injectionEnabled: true, primaryCanonBaseMaxChars: 1000,
  maxInputContextChars: 800, episodeIntervalTurns: 10,
  embeddingApiKey: "", embeddingEndpoint: "", embeddingModel: "", embeddingProvider: "off", embeddingTimeout: 1
};
const DEFAULT_SETTINGS = {maxInjectionChars: 1000, topK: 3, episodeIntervalTurns: 10, embeddingProvider: "off"};
function resolveNarrativeGuideMode() { return "off"; }
function normalizeNarrativeGuideStrength(value) { return value === "none" ? "none" : "weak"; }
function getPayloadMessageRoleAndText(message) { return {role: message.role || "", text: message.content || ""}; }
function sanitizeTopKSetting(value) { return Number(value || 0); }
function normalizeLanguageContextTrace(value) { return value || null; }
function normalizeEmbeddingProvider(value) { return value; }
function getEmbeddingTimeoutMs() { return 1000; }
function getRequestTimeoutSettingMs() { return 1000; }
function debugLog() {}
let capturedBody = null;
const capturedBodies = [];
let expectedLane = {
  status: "degraded", reason_code: "source_observation_optional_capability_missing",
  retryable: false, affected_lane: "source_observation", original_payload_preserved: true,
  contract_version: "source_lane_status.v1", capability_coverage: {}, request_correlation_id: "request-a"
};
async function bridgeFetch(path, options) {
  if (path !== "/prepare-turn") throw new Error("unexpected path " + path);
  capturedBody = options.body;
  capturedBodies.push(options.body);
  return {
    status: "ok", source: "shadow", fallback_reason: "",
    source_contract: {contract_version: "prepare_source_projection.v1", lane_status: expectedLane},
    current_input_decision: {status: "eligible", reason_code: "current_user_input_observed"},
    session_bootstrap: {status: "preserved", reason_code: "bootstrap_sources_preserved"}
  };
}

(async function() {
  const sourceObservation = {
    contract_version: "message_source_observation.v1", session_id: "session-a", request_id: "request-a",
    message_index: 0, observed_role: "user", observable: true, evidence_state: "observed"
  };
  const capabilityObservation = {
    contract_version: "host_source_capabilities.v1",
    capabilities: {session_identity: "observed", request_correlation: "observed", message_position: "observed", message_role: "observed"}
  };
  const hostObservations = {contract_version: "prepare_host_observations.v1", session_id: "session-a", request_id: "request-a"};
  const bootstrapObservation = {contract_version: "session_bootstrap_observation.v1", session_id: "session-a", request_id: "request-a", leading_messages: []};
  for (const status of ["degraded", "failed", "incompatible"]) {
    expectedLane = Object.assign({}, expectedLane, {status, reason_code: "source_observation_" + status});
    const result = await tryPrepareTurn("session-a", "hello", [{role: "user", content: "hello"}], null, "model", null, {
      sourceObservation, capabilityObservation, hostObservations, bootstrapObservation
    });
    if (!capturedBody) throw new Error("request body not captured");
    if (capturedBody.source_observation !== sourceObservation || capturedBody.capability_observation !== capabilityObservation) {
      throw new Error("host observations were not forwarded unchanged");
    }
    if (capturedBody.host_observations !== hostObservations || capturedBody.bootstrap_observation !== bootstrapObservation) {
      throw new Error("3.3-D host/bootstrap observations were not forwarded unchanged");
    }
    for (const forbidden of ["authority", "stable_identity", "canonical_truth", "lifecycle_acceptance", "persistence"]) {
      if (Object.prototype.hasOwnProperty.call(capturedBody.source_observation, forbidden)) throw new Error("forbidden inference " + forbidden);
    }
    if (!result.sourceContract || result.sourceContract.lane_status.status !== status) {
      throw new Error("stable " + status + " status was not preserved");
    }
    if (!result.bundle.sourceContract || result.bundle.sourceContract.lane_status.reason_code !== expectedLane.reason_code) {
      throw new Error("stable reason code was not preserved in bundle");
    }
    if (!result.currentInputDecision || result.currentInputDecision.status !== "eligible" || !result.sessionBootstrap || result.sessionBootstrap.status !== "preserved") {
      throw new Error("Go-owned 3.3-D decisions were not returned to the adapter");
    }
  }
  const decisionResult = await tryPrepareTurn("session-a", "", [{role: "user", content: "hello"}], null, "model", null, {
    sourceDecisionOnly: true, sourceObservation, capabilityObservation, hostObservations, bootstrapObservation
  });
  const decisionBody = capturedBodies[capturedBodies.length - 1];
  if (decisionBody.source_decision_only !== true || decisionBody.host_observations !== hostObservations) {
    throw new Error("read-free source decision phase was not transported with the same host observations");
  }
  if (!decisionResult.currentInputDecision || decisionResult.currentInputDecision.status !== "eligible") {
    throw new Error("source decision response was not returned to the production adapter");
  }
  const languageContext = {session_output_language: "ko", output_language_override: "ko"};
  await tryPrepareTurn("session-a", "hello", [{role: "user", content: "hello"}], {
    triggerMode: "manual_resume", query: "continue the unresolved thread"
  }, "model", languageContext, {
    freshFirstTurnLightMode: true,
    freshFirstTurnLightModeMeta: {activeCompletedPairs: 0, latestBackendTurn: 0, routingBaselineBackendTurn: 0},
    sourceObservation, capabilityObservation, hostObservations, bootstrapObservation
  });
  const fullBody = capturedBodies[capturedBodies.length - 1];
  if (fullBody.source_decision_only === true) throw new Error("full prepare was mislabeled as decision-only");
  if (fullBody.continuity_trigger_mode !== "manual_resume" || fullBody.continuity_query !== "continue the unresolved thread") {
    throw new Error("continuityInfo was not delivered to full prepare");
  }
  if (!fullBody.client_meta || fullBody.client_meta.language_context !== languageContext || fullBody.output_language_override !== "ko") {
    throw new Error("languageContext was not delivered to full prepare");
  }
  if (fullBody.host_observations !== hostObservations || fullBody.bootstrap_observation !== bootstrapObservation) {
    throw new Error("full prepare did not repeat the correlated host observations");
  }
  if (fullBody.settings.top_k !== 0 || fullBody.settings.max_injection_chars !== 0 || fullBody.settings.max_input_context_chars !== 0 || fullBody.settings.input_context_enabled !== false) {
    throw new Error("fresh-first-turn memory budgets were not suppressed");
  }
  await tryPrepareTurn("session-a", "hello", [{role: "user", content: "hello"}], null, "model", null, {
    sourceObservation, capabilityObservation, hostObservations, bootstrapObservation
  });
  const existingSessionBody = capturedBodies[capturedBodies.length - 1];
  if (existingSessionBody.settings.top_k !== 3 || existingSessionBody.settings.max_injection_chars !== 1000 || existingSessionBody.settings.input_context_enabled !== true) {
    throw new Error("existing-session prepare budget regressed");
  }
  settings.injectionBudgetExtraChars = 2500;
  await tryPrepareTurn("session-a", "hello", [{role: "user", content: "hello"}], null, "model", null, {
    runtimeTokenInfo: {currentChatTokens: 9135, source: "message_char_estimate"},
    sourceObservation, capabilityObservation, hostObservations, bootstrapObservation
  });
  const adaptiveBudgetBody = capturedBodies[capturedBodies.length - 1];
  if (adaptiveBudgetBody.settings.max_injection_chars !== 11500) {
    throw new Error("adaptive plus extra budget was not forwarded to Go: " + adaptiveBudgetBody.settings.max_injection_chars);
  }
  settings.narrativeGuideMode = "auto";
  settings.narrativeGuideStrength = "none";
  await tryPrepareTurn("session-a", "hello", [{role: "user", content: "hello"}], null, "model", null, {
    sourceObservation, capabilityObservation, hostObservations, bootstrapObservation
  });
  const guideOffBody = capturedBodies[capturedBodies.length - 1];
  if (guideOffBody.settings.guide_mode !== "off" || guideOffBody.settings.guide_strength !== "none" || guideOffBody.settings.supervisor_enabled !== false) {
    throw new Error("guide none was not transported as an explicit OFF contract: "+JSON.stringify(guideOffBody.settings));
  }
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-e", script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("prepare-turn source/capability runtime fixture failed: %v\n%s", err, output)
	}
}

func TestLegacyAutomaticInjectionBudgetMigratesOnceToCurrentBase(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for injection budget migration runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	fn := extractArchiveCenterJSFunction(t, src, "migrateLegacyInjectionBudgetSettings")
	script := "const DEFAULT_SETTINGS = {injectionBudgetProfileVersion: \"p34_9000_base_v1\"};\n" + fn + `
const legacy = migrateLegacyInjectionBudgetSettings({maxInjectionChars: 6000});
if (legacy.maxInjectionChars !== 9000) throw new Error("legacy default was not migrated: " + JSON.stringify(legacy));
const custom = migrateLegacyInjectionBudgetSettings({maxInjectionChars: 7500});
if (custom.maxInjectionChars !== 7500) throw new Error("non-default user value was overwritten: " + JSON.stringify(custom));
const versioned = migrateLegacyInjectionBudgetSettings({maxInjectionChars: 6000, injectionBudgetProfileVersion: "p34_9000_base_v1"});
if (versioned.maxInjectionChars !== 6000) throw new Error("versioned user value was migrated repeatedly: " + JSON.stringify(versioned));
`
	cmd := exec.Command(nodePath, "-e", script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("injection budget migration runtime fixture failed: %v\n%s", err, output)
	}
}

func TestBeforeRequestNonModelSkipsPrepareTurnRuntime(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required")
		}
	}
	src := readArchiveCenterJS(t)
	fn := extractArchiveCenterJSAsyncFunction(t, src, "onBeforeRequest")
	script := fn + `
const settings = {enabled: true};
function debugLog() {}
function isSaveType(type) { return type === "model"; }
let prepareCalls = 0;
async function tryPrepareTurn() { prepareCalls++; throw new Error("prepare-turn must not run"); }
(async function() {
  const payload = {messages: [{role: "user", content: "auxiliary"}]};
  const result = await onBeforeRequest(payload, "submodel");
  if (result !== payload) throw new Error("non-model payload identity changed");
  if (prepareCalls !== 0) throw new Error("non-model prepare calls=" + prepareCalls);
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("non-model beforeRequest runtime fixture failed: %v\n%s", err, output)
	}
}

func TestBeforeRequestModelRunsDecisionThenFullWithContextRuntime(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required")
		}
	}
	src := readArchiveCenterJS(t)
	fn := extractArchiveCenterJSAsyncFunction(t, src, "onBeforeRequest")
	script := fn + `
const settings = {enabled: true, debug: false};
const ACTIVE_CHAT_BACKFILL_MAX_PAIRS = 20;
let _sessionCache = null;
let _effectiveInputAwaitingNewTurn = false;
const _pendingPersistenceSkipBySession = new Map();
let activePairs = 0;
let latestBackendTurn = 0;
let prepareCalls = [];
let preFullSideEffects = 0;
const hostObservationsFixture = {request_id: "request-runtime", payload: [{role: "user", raw_content: "actual input", message_index: 1}]};
const bootstrapObservationFixture = {request_id: "request-runtime"};
const languageContextFixture = {session_output_language: "ko", output_language_override: "ko"};
const continuityFixture = {triggerMode: "manual_resume", query: "unresolved thread"};
let currentContinuity = null;
function debugLog() {}
function warnLog() {}
function isSaveType(type) { return type === "model"; }
function extractMessages(payload) { return {messages: payload.messages, path: ["messages"], hasMessageSlot: true}; }
function normalizeMessagesForOrchestration(messages) { return messages; }
function extractRuntimeCurrentChatTokenInfo() { return {}; }
async function getCurrentChatSessionId() { return "session-runtime"; }
async function resolveCanonicalWriteSessionId(value) { return value; }
async function getCurrentActiveChatSourceObservationMessages() { return [{role: "user", content: "actual input", risuMessageIndex: 1}]; }
function peekRawInputForSession() { return {text: "actual input", capturedAt: 1}; }
function makeOrchRequestId() { return "request-runtime"; }
function buildPrepareTurnHostObservations() { return hostObservationsFixture; }
async function observePrepareTurnBootstrap() { return bootstrapObservationFixture; }
function buildPrepareTurnSourceObservations() { return {sourceObservation: {request_id: "request-runtime"}, capabilityObservation: {capabilities: {}}}; }
function updateRuntimeState() {}
function ensureActiveChatCompletedTurnsBackfilled() { preFullSideEffects++; return Promise.resolve(); }
async function captureAssistantPrefillSeedForSession() { preFullSideEffects++; }
async function resolveRollbackComparableMessages() { preFullSideEffects++; return {messages: null, source: "fixture"}; }
function scrubOocDirectivesFromUserInput(text) { return {fullyOoc: false, changed: false, text}; }
function detectCurrentTurnOocInfo() { return {isOoc: false}; }
async function buildLanguageContextTrace() { return languageContextFixture; }
async function resolveContinuityTriggerInfo() { return currentContinuity; }
function getSessionRoutingTurnBaseline() { return {backendTurnAtRoute: 0}; }
async function safeCall(call, fallback) { try { return await call(); } catch { return fallback; } }
async function resolveActiveChatCompletedTurnsForRoutingBaseline() { return activePairs; }
async function fetchBackendLatestTurnIndexForSession() { return latestBackendTurn; }
async function tryPrepareTurn(sessionId, userInput, messages, continuityInfo, type, languageContext, options) {
  prepareCalls.push({sessionId, userInput, messages, continuityInfo, type, languageContext, options});
  if (options && options.sourceDecisionOnly === true) {
    return {source: "backend-source-decision", currentInputDecision: {
      status: "eligible", reason_code: "current_user_input_observed", effective_user_input: "actual input",
      selected_observation_ref: "input-hook:request-runtime", context_injection_eligible: true, memory_reads_allowed: true
    }, sessionBootstrap: {status: "preserved"}};
  }
  return null;
}
async function runFixture(expectedFresh, expectedContinuity) {
  prepareCalls = [];
  preFullSideEffects = 0;
  const payload = {messages: [{role: "system", content: "preset"}, {role: "user", content: "actual input"}, {role: "user", content: "later host prompt"}]};
  const result = await onBeforeRequest(payload, "model");
  if (result !== payload) throw new Error("fixture stop did not preserve original payload");
  if (prepareCalls.length !== 2) throw new Error("model prepare calls=" + prepareCalls.length + ", want 2");
  if (preFullSideEffects !== 0) throw new Error("full source failure allowed pre-validation side effects=" + preFullSideEffects);
  const decision = prepareCalls[0];
  const full = prepareCalls[1];
  if (!decision.options.sourceDecisionOnly || decision.userInput !== "" || decision.continuityInfo !== null || decision.languageContext !== null) {
    throw new Error("first call was not a source-decision-only phase");
  }
  if (full.options.sourceDecisionOnly === true || full.userInput !== "actual input") throw new Error("full call did not use Go effective input");
  if (full.continuityInfo !== expectedContinuity) throw new Error("onBeforeRequest did not pass continuityInfo to full prepare");
  if (full.languageContext !== languageContextFixture) throw new Error("onBeforeRequest did not pass languageContext to full prepare");
  if (!!full.options.freshFirstTurnLightMode !== expectedFresh) throw new Error("fresh mode=" + full.options.freshFirstTurnLightMode + ", want " + expectedFresh);
  if (full.options.hostObservations !== decision.options.hostObservations || full.options.bootstrapObservation !== decision.options.bootstrapObservation) {
    throw new Error("decision/full calls did not repeat identical correlated observations");
  }
}
(async function() {
  activePairs = 0; latestBackendTurn = 0; currentContinuity = null;
  await runFixture(true, null);
  activePairs = 2; latestBackendTurn = 2; currentContinuity = continuityFixture;
  await runFixture(false, continuityFixture);
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("model beforeRequest two-phase runtime fixture failed: %v\n%s", err, output)
	}
}

func TestBeforeRequestBuildsObservationOnlySourceEnvelope(t *testing.T) {
	src := readArchiveCenterJS(t)
	fn := extractArchiveCenterJSAsyncFunction(t, src, "onBeforeRequest")
	observationFunction := extractArchiveCenterJSFunction(t, src, "buildPrepareTurnSourceObservations")
	dotClass := extractArchiveCenterJSFunction(t, src, "statusDotClass")
	for _, required := range []string{
		`if (!settings.enabled || !isSaveType(type)) return payload;`,
		`sourceDecisionOnly: true`,
		`const preparedTurnResult = await tryPrepareTurn(orchSessionId, userInput, messages, continuityInfo, type, turnLanguageContext, {`,
		`freshFirstTurnLightMode,`,
		`freshFirstTurnLightModeMeta,`,
		`const prepareSourceObservations = buildPrepareTurnSourceObservations(`,
		`const hostObservations = buildPrepareTurnHostObservations(`,
		`const bootstrapObservation = await observePrepareTurnBootstrap(`,
		`sourceObservation: prepareSourceObservations.sourceObservation`,
		`capabilityObservation: prepareSourceObservations.capabilityObservation`,
		`hostObservations,`,
		`bootstrapObservation,`,
	} {
		if !strings.Contains(fn, required) {
			t.Fatalf("beforeRequest source observation missing %q", required)
		}
	}
	if count := strings.Count(fn, "await tryPrepareTurn("); count != 2 {
		t.Fatalf("model beforeRequest must contain exactly decision-only + full prepare calls, got %d", count)
	}
	for _, forbidden := range []string{"scrubOocDirectivesFromUserInput(userInput)", "detectCurrentTurnOocInfo(payload, messages"} {
		if strings.Contains(fn, forbidden) {
			t.Fatalf("JavaScript changed or classified Go effective input before full prepare: %q", forbidden)
		}
	}
	for _, forbidden := range []string{"source_authority", "stable_identity", "canonical_truth", "lifecycle_acceptance", "persistence"} {
		if strings.Contains(observationFunction, forbidden) {
			t.Fatalf("JavaScript inferred forbidden source policy %q", forbidden)
		}
	}
	for _, required := range []string{
		`contract_version: "message_source_observation.v1"`,
		`contract_version: "host_source_capabilities.v1"`,
		`raw_input_hash: observedRawInputHash`,
		`raw_input_hash: observedRawInputHash ? "observed" : "unavailable"`,
	} {
		if !strings.Contains(observationFunction, required) {
			t.Fatalf("production observation helper missing %q", required)
		}
	}
	for _, required := range []string{
		`const observedSourcePath = !beforeRequestRecoveredForRead`,
		`const ptStatus = _prepareTurnEverContacted`,
		`? String(ptLaneStatus && ptLaneStatus.status || ptResult.status || "ok")`,
		`reason_code: ptLaneStatus && ptLaneStatus.reason_code || null`,
		`original_payload_preserved: ptLaneStatus ? !!ptLaneStatus.original_payload_preserved : null`,
		`detail: ptLaneStatus ? {`,
	} {
		if !strings.Contains(fn, required) {
			t.Fatalf("beforeRequest stable lane projection missing %q", required)
		}
	}
	for _, status := range []string{"eligible", "not_applicable", "empty", "deferred", "degraded", "failed", "incompatible"} {
		if !strings.Contains(dotClass, status) {
			t.Fatalf("stable source lane status %q is not represented by the UI adapter", status)
		}
	}
}

func TestActualInputAndBootstrapProductionObservationRuntime(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required")
		}
	}
	src := readArchiveCenterJS(t)
	hashFn := extractArchiveCenterJSFunction(t, src, "computeOrchestrationDirtyHashOr1c")
	messageFn := extractArchiveCenterJSFunction(t, src, "buildPrepareMessageObservation")
	hostFn := extractArchiveCenterJSFunction(t, src, "buildPrepareTurnHostObservations")
	bootstrapFn := extractArchiveCenterJSAsyncFunction(t, src, "observePrepareTurnBootstrap")
	script := `
"use strict";
const R = {
  getCharacter: async () => ({
    firstMessage: "first greeting",
    alternateGreetings: ["alternate zero", "alternate one"],
  }),
};
async function resolveCurrentActiveChatObject() { return { chat: { fmIndex: 1, data: {} } }; }
function getPayloadMessageRoleAndText(message) {
  return { role: String(message && message.role || ""), text: String(message && message.content || "") };
}
` + hashFn + "\n" + messageFn + "\n" + hostFn + "\n" + bootstrapFn + `
(async function() {
  const actual = '{"supervisor":"return JSON only","npc":"list"}';
  const active = [
    { role: "assistant", content: "start A", risuMessageIndex: 0, raw: { role: "char", data: "start A", chatId: "risu-start-0", time: 101, generationInfo: { generationId: "gen-start-0" } } },
    { role: "assistant", content: "start A", risuMessageIndex: 1, raw: { role: "char", data: "start A", chatId: "risu-start-1", time: 102 } },
    { role: "user", content: actual, risuMessageIndex: 2, raw: { role: "user", data: actual, chatId: "risu-user-2", time: 103, id: "unsupported-id", revision: "unsupported-revision" } },
  ];
  const payload = [
    { role: "system", content: "preset" },
    { role: "user", content: actual },
    { role: "system", content: "host suffix" },
  ];
  const host = buildPrepareTurnHostObservations(
    "session-d", "request-d", "model",
    { text: actual, capturedAt: 123456 }, active, payload,
    '["messages"]', true, "chat-d"
  );
  if (!host.input_hook || host.input_hook.raw_content !== actual || host.input_hook.role !== "user") {
    throw new Error("input-hook observation was classified or changed");
  }
  if (host.input_hook.source_kind !== "input_hook_intermediate" || host.input_hook.observation_stage !== "input_script_handler" || host.input_hook.lifecycle_kind !== "user_input_hook_intermediate") {
    throw new Error("input hook was presented as a raw/final host observation");
  }
  if (host.payload_observation_stage !== "before_request_replacer" || host.final_payload_observation !== "not_exposed") {
    throw new Error("beforeRequest payload was presented as the final sent payload");
  }
  if (host.active_chat[2].message_id !== "risu-user-2" || host.active_chat[2].message_time !== 103 || host.active_chat[2].observed_revision !== null) {
    throw new Error("official Risu message identity was not preserved exactly");
  }
  if (host.active_chat[0].generation_id !== "gen-start-0" || host.active_chat[0].message_id !== "risu-start-0") {
    throw new Error("official Risu generation identity was not observed");
  }
  if (host.input_hook.content_hash !== computeOrchestrationDirtyHashOr1c(actual)) {
    throw new Error("input-hook hash mismatch");
  }
  if (host.payload.length !== 3 || host.payload[2].role !== "system") {
    throw new Error("payload lifecycle observations were collapsed");
  }
  const bootstrap = await observePrepareTurnBootstrap("session-d", "request-d", active, "chat-d");
  if (bootstrap.leading_messages.length !== 2) throw new Error("multiple starts were not preserved");
  if (bootstrap.leading_messages[0].message_index !== 0 || bootstrap.leading_messages[1].message_index !== 1) {
    throw new Error("bootstrap order/boundaries changed");
  }
  if (bootstrap.leading_messages[0].raw_content !== "start A" || bootstrap.leading_messages[1].raw_content !== "start A") {
    throw new Error("duplicate bootstrap text was deduplicated");
  }
  if (!bootstrap.selection_exposed || bootstrap.selected_greeting_index !== 1) {
    throw new Error("selected greeting observation missing");
  }
  if (bootstrap.first_greeting !== "first greeting" || bootstrap.alternate_greetings[1] !== "alternate one") {
    throw new Error("host greeting fields were selected or normalized in JavaScript");
  }
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-e", script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("actual-input/bootstrap production observation runtime failed: %v\n%s", err, output)
	}
}

func TestActiveChatSourceObservationUsesOfficialRisuMessageShape(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required")
		}
	}
	src := readArchiveCenterJS(t)
	fn := extractArchiveCenterJSAsyncFunction(t, src, "getCurrentActiveChatSourceObservationMessages")
	script := fn + `
const active = {message: [
  {role: "user", data: "official user", chatId: "u-1", time: 10},
  {role: "char", data: "official character", chatId: "c-1", time: 11, generationInfo: {generationId: "g-1"}},
  {role: "assistant", content: "generic compatibility shape"},
  {role: "user", data: 42},
]};
async function resolveCurrentActiveChatObject() { return {chat: active}; }
function extractActiveChatMessageList(chat) { return chat.message; }
(async function() {
  const observed = await getCurrentActiveChatSourceObservationMessages("session-risu");
  if (observed.length !== 2) throw new Error("non-official message shape was inferred");
  if (observed[0].role !== "user" || observed[0].content !== "official user" || observed[0].raw.chatId !== "u-1") {
    throw new Error("official user message was not preserved");
  }
  if (observed[1].role !== "assistant" || observed[1].content !== "official character" || observed[1].raw.generationInfo.generationId !== "g-1") {
    throw new Error("official character message was not preserved");
  }
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("official Risu active-chat observation fixture failed: %v\n%s", err, output)
	}
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

func TestSessionNormalizeRepairEntriesPreserveExistingTurnZeroAndRepairMissingPairs(t *testing.T) {
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
function assertEqual(actual, expected, label) {
  if (actual !== expected) throw new Error(label + ": got=" + JSON.stringify(actual) + " want=" + JSON.stringify(expected));
}
const plan = {
  dbRows: [],
  rawMissingTurns: [1,2,3,4],
  pairs: [1,2,3,4].map(turn => ({turnIndex:turn,userContent:"user "+turn,assistantContent:"assistant "+turn})),
};
let entries = buildSessionNormalizeRepairEntriesFromDryRunPlan(plan);
assertEqual(JSON.stringify(entries.map(entry => entry.turn_index)), JSON.stringify([1,2,3,4]), "bootstrap must not be fabricated as canonical turn zero");
entries = buildSessionNormalizeRepairEntriesFromDryRunPlan({...plan, dbRows:[{turn_index:0,role:"assistant",content:"already stored"}]});
assertEqual(JSON.stringify(entries.map(entry => entry.turn_index)), JSON.stringify([1,2,3,4]), "existing turn zero must remain untouched");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("session-normalize repair JS fixture failed: %v\n%s", err, out)
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
  exactTurn = 0;
  const rerolledTurn = await reserveAfterRequestPersistenceTurnIndex("s", fixture.user, fixture.assistant);
  if (rerolledTurn !== 3 || exactTurn !== 3) throw new Error("reroll must reuse backend turn 3");
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

func TestCompleteTurnObservationUsesRealUserAnchorForAppendStyleReroll(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for source acceptance observation fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functionBody := extractArchiveCenterJSAsyncFunction(t, src, "buildCompleteTurnSourceAcceptanceObservation")
	script := functionBody + `
const _streamingAfterRequestSyntheticCallDepth = 0;
const chat = {id:"chat-1",isStreaming:false,message:[
  {role:"user",data:"same user",chatId:"user-1",time:100},
  {role:"char",data:"old answer",chatId:"assistant-old",time:200,generationInfo:{generationId:"generation-old"}},
  {role:"char",data:"new answer",chatId:"assistant-new",time:300,generationInfo:{generationId:"generation-new"}},
  {role:"comment",data:"host metadata",disabled:true},
]};
function computeOrchestrationDirtyHashOr1c(value) { return "hash:"+String(value || "").trim(); }
async function resolveCurrentActiveChatObject() { return {chat}; }
function normalizeAssistantPersistenceCandidate(value) { return String(value || "").trim(); }
function isSameAssistantComparableText(a,b) { return a === b; }
function getSessionSnapshot() { return {msgCount:0}; }
function debugLog() {}
(async function() {
  const oldObservation = await buildCompleteTurnSourceAcceptanceObservation("session-1", "old answer", {allowExistingActiveMessage:true,userInput:"same user"});
  const newObservation = await buildCompleteTurnSourceAcceptanceObservation("session-1", "new answer", {allowExistingActiveMessage:true,userInput:"same user"});
  if (oldObservation.user_message_index !== 0 || newObservation.user_message_index !== 0) throw new Error("reroll candidates must share user anchor");
  if (oldObservation.user_message_chat_id !== "user-1" || newObservation.user_message_chat_id !== "user-1") throw new Error("host user id must be observed without synthesis");
  if (oldObservation.user_observed_content_hash !== newObservation.user_observed_content_hash) throw new Error("user anchor hashes differ");
  if (newObservation.position_observation !== "current_active_assistant_tail") throw new Error("new final is not active assistant tail");
  if (newObservation.later_active_turn_message_count !== 0 || newObservation.later_non_turn_message_count !== 1) throw new Error("later host metadata observation is wrong");
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("source acceptance observation JS fixture failed: %v\n%s", err, out)
	}
}

func TestLateNativeAfterRequestRearmsExpiredActiveChatWatcher(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for streaming watcher fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functionBody := extractArchiveCenterJSFunction(t, src, "markNativeAfterRequestObserved")
	script := functionBody + `
const _streamingAfterRequestWatchers = new Map();
let armCalls = 0;
function armStreamingAfterRequestWatch(sessionId, type, requestId) {
  armCalls++;
  if (requestId !== "late_native_after_request") throw new Error("unexpected rearm reason");
  _streamingAfterRequestWatchers.set(sessionId, {nativeAfterRequestObserved:false,nativeAfterRequestObservedAt:0,type});
}
function updateRuntimeState() {}
markNativeAfterRequestObserved("session-1", "model");
const watcher = _streamingAfterRequestWatchers.get("session-1");
if (armCalls !== 1 || !watcher || watcher.nativeAfterRequestObserved !== true || watcher.nativeAfterRequestObservedAt <= 0) {
  throw new Error("late native afterRequest did not rearm active chat confirmation");
}
markNativeAfterRequestObserved("session-1", "model");
if (armCalls !== 1) throw new Error("live watcher was unnecessarily rearmed");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("late native afterRequest watcher fixture failed: %v\n%s", err, out)
	}
}

func TestRollbackDecisionForwardsObservedHostGenerationLifecycle(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for rollback observation fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functionBody := extractArchiveCenterJSAsyncFunction(t, src, "requestBackendRollbackDecision")
	script := functionBody + `
let capturedBody = null;
function getRequestTimeoutSettingMs() { return 90000; }
function serializeSessionRoutingBaselineForBackend() { return null; }
async function bridgeFetch(path, options) {
  if (path !== "/rollback/decision") throw new Error("unexpected path");
  capturedBody = options.body;
  return {status:"ok",contract_version:"rollback.decision.v1",allowed:false,reason:"pending_output_guard"};
}

(async function() {
  await requestBackendRollbackDecision("session-1", 4, "active_chat_tail_missing_from_runtime", {
    backendLatestTurnIndex:4,
    hostLifecycleObservation:"before_request_observed",
  }, "auto");
  if (!capturedBody || capturedBody.host_lifecycle_observation !== "before_request_observed") {
    throw new Error("observed host generation lifecycle was not forwarded");
  }
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rollback host lifecycle observation fixture failed: %v\n%s", err, out)
	}
}

func TestQueuedCompleteTurnRebuildsFromSameTurnActiveHostFinal(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for queued active-final fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functionBody := extractArchiveCenterJSAsyncFunction(t, src, "refreshQueuedCompleteTurnSourceObservation")
	script := functionBody + `
let buildAssistant = "";
let buildOptions = null;
let resolvedTurn = 15;
async function findActiveChatCompletedTurnPairForContent() { return null; }
async function findActiveChatCompletedTurnPairForUserContent() {
  return {observedPairOrdinal:15,userContent:"user",assistantContent:"host final",pairCount:15};
}
async function requestBackendSessionRoutingTurnResolution() { return {turnIndex:resolvedTurn,status:"normal"}; }
async function buildCompleteTurnRequestBody(turn,user,assistant,context,sid,trace,options) {
  buildAssistant = assistant;
  buildOptions = options;
  return {chat_session_id:sid,turn_index:turn,user_input:user,assistant_content:assistant,context_messages:context,
    improvement_trace:trace,request_type:"model",client_meta:{idempotency_key:"host-final-key",source_acceptance_observation:{
      observed_content_hash:"host-final-hash",position_observation:"current_active_chat_tail"
    }}};
}
function buildCompleteTurnQueuePayload(body) { return JSON.parse(JSON.stringify(body)); }
(async function() {
  const payload={chat_session_id:"session-1",turn_index:15,user_input:"user",assistant_content:"native before host apply",context_messages:[],client_meta:{
    idempotency_key:"old-key",source_acceptance_observation:{active_message_count:151}
  }};
  const ok=await refreshQueuedCompleteTurnSourceObservation(payload);
  if(!ok) throw new Error("same-turn active host final did not refresh");
  if(buildAssistant!=="host final" || payload.assistant_content!=="host final") throw new Error("queued assistant was not replaced by host final");
  if(!buildOptions || buildOptions.allowExistingActiveMessage!==true) throw new Error("retry did not allow live existing message observation");
  if(payload.client_meta.idempotency_key!=="host-final-key") throw new Error("idempotency key was not rebuilt");
  resolvedTurn = 16;
  const wrongTurn={chat_session_id:"session-1",turn_index:15,user_input:"user",assistant_content:"native before host apply",context_messages:[],client_meta:{}};
  if(await refreshQueuedCompleteTurnSourceObservation(wrongTurn)) throw new Error("different logical turn was adopted");
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("queued active host final fixture failed: %v\n%s", err, out)
	}
}

func TestPersistedCompleteTurnQueueKeepsSourceFenceWithoutCredentials(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for complete-turn queue fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functions := extractArchiveCenterJSFunction(t, src, "buildCompleteTurnQueuePayload") +
		extractArchiveCenterJSFunction(t, src, "serializeCompleteTurnRecoveryPayload")
	script := functions + `
function normalizeLanguageContextTrace(value) { return value; }
const sourceObservation = {contract_version:"source_acceptance_observation.v1",observed_at_ms:123,message_index:4,active_message_count:5};
const saved = serializeCompleteTurnRecoveryPayload({
  chat_session_id:"session-1",turn_index:3,user_input:"user",assistant_content:"assistant",context_messages:[],
  client_meta:{source_acceptance_required:true,source_acceptance_observation:sourceObservation,idempotency_key:"key-1",critic:{api_key:"secret"}}
});
if (!saved || saved.client_meta.source_acceptance_required !== true) throw new Error("source fence requirement was lost");
if (!saved.client_meta.source_acceptance_observation || saved.client_meta.source_acceptance_observation.message_index !== 4) throw new Error("source observation was lost");
if (saved.client_meta.idempotency_key !== "key-1") throw new Error("idempotency key was lost");
if (saved.client_meta.critic || JSON.stringify(saved).includes("secret")) throw new Error("credential-bearing critic config was persisted");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("persisted complete-turn source fence fixture failed: %v\n%s", err, out)
	}
}

func TestRestoredCompleteTurnQueueRebuildsLiveConfigAndSourceObservation(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for restored complete-turn queue fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functionBody := extractArchiveCenterJSAsyncFunction(t, src, "refreshQueuedCompleteTurnSourceObservation")
	script := functionBody + `
async function findActiveChatCompletedTurnPairForContent(session, user, assistant) {
  return {observedPairOrdinal:3,userContent:user,assistantContent:assistant,pairCount:3};
}
async function findActiveChatCompletedTurnPairForUserContent() { return null; }
async function requestBackendSessionRoutingTurnResolution() { return {turnIndex:3,status:"normal"}; }
async function buildCompleteTurnRequestBody(turn, user, assistant, context, session) {
  if (turn !== 3 || user !== "user" || assistant !== "assistant" || session !== "session-1") throw new Error("queue payload was not rebuilt");
  return {chat_session_id:session,turn_index:turn,user_input:user,assistant_content:assistant,context_messages:context,request_type:"model",client_meta:{
    source_acceptance_required:true,
    source_acceptance_observation:{observed_content_hash:"hash-final",host_chat_id:"chat-1",generation_id:"generation-final",message_chat_id:"message-final",message_time_state:"observed",message_time_ms:300},
    critic:{api_key:"current-live-key"},
    idempotency_key:"rebuilt-key",
    request_id:"rebuilt-key"
  }};
}
function buildCompleteTurnQueuePayload(body) { return JSON.parse(JSON.stringify(body)); }
function debugLog() {}
(async function() {
  const restored = {chat_session_id:"session-1",turn_index:3,user_input:"user",assistant_content:"assistant",context_messages:[],client_meta:{}};
  if (!await refreshQueuedCompleteTurnSourceObservation(restored)) throw new Error("restored queue was not refreshed");
  if (restored.client_meta.source_acceptance_required !== true || restored.client_meta.idempotency_key !== "rebuilt-key") throw new Error("source fence or request key was not rebuilt");
  if (!restored.client_meta.critic || restored.client_meta.critic.api_key !== "current-live-key") throw new Error("current live critic config was not restored in memory");

  const stale = {chat_session_id:"session-1",turn_index:3,user_input:"user",assistant_content:"assistant",context_messages:[],client_meta:{source_acceptance_observation:{observed_content_hash:"old",host_chat_id:"chat-1",generation_id:"generation-old"}}};
  if (await refreshQueuedCompleteTurnSourceObservation(stale)) throw new Error("stale generation was refreshed as current");
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("restored complete-turn queue refresh fixture failed: %v\n%s", err, out)
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

func TestEmptyActiveChatRecognizesIncompleteUserOnlyTailCandidate(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for incomplete tail rollback fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functionBody := extractArchiveCenterJSFunction(t, src, "buildLedgerVerifiedTailRollback")
	script := functionBody + `
let fixture = {trackedTurnIndex:1,entries:[{role:"user",turnIndex:1,fingerprint:"u1"}]};
function loadRollbackTurnLedgerOr1f() { return fixture; }
function compactSnapshotMessages(messages) { return Array.isArray(messages) ? messages : []; }
function computeLedgerCurrentPrefixLengthOr1f(_ledger, current) { return current.length; }
function computeTailHash() { return "empty"; }
function debugLog() {}
let result = buildLedgerVerifiedTailRollback("s", [], 0, 1);
if (!result || result.status !== "incomplete_user_only_tail_candidate" || result.rollbackFrom !== 1 || result.removedUserCount !== 1 || result.removedAssistantCount !== 0) {
  throw new Error("user-only tail was not recognized: " + JSON.stringify(result));
}
fixture = {trackedTurnIndex:2,entries:[{role:"user",turnIndex:1},{role:"user",turnIndex:2}]};
result = buildLedgerVerifiedTailRollback("s", [], 0, 2);
if (result !== null) throw new Error("multi-message deletion must not become an incomplete-tail candidate: " + JSON.stringify(result));
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("incomplete user-only tail JS runtime fixture failed: %v\n%s", err, out)
	}
}

func TestRollbackDecisionTransportIncludesNestedLedgerCounts(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for rollback decision transport fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functionBody := extractArchiveCenterJSAsyncFunction(t, src, "requestBackendRollbackDecision")
	script := functionBody + `
let sentBody = null;
function getRequestTimeoutSettingMs() { return 1000; }
async function fetchBackendLatestTurnIndexForSession() { return 9; }
async function bridgeFetch(path, options) {
  if (path !== "/rollback/decision") throw new Error("unexpected path " + path);
  sentBody = options.body;
  return {status:"ok",contract_version:"rollback.decision.v1"};
}
function serializeSessionRoutingBaselineForBackend() { return null; }
(async function() {
  await requestBackendRollbackDecision("s", 9, "delete", {
    backendLatestTurnIndex:9,
    tailReconcileVerification:{
      status:"incomplete_user_only_tail_candidate",
      removedAssistantCount:0,
      removedUserCount:1,
      removedMessageCount:1
    }
  }, "auto");
  if (!sentBody || sentBody.removed_user_count !== 1 || sentBody.removed_message_count !== 1 || sentBody.removed_assistant_count !== 0 || sentBody.incomplete_tail_candidate !== true || sentBody.ledger_verified !== false) {
    throw new Error("nested rollback verification was not transported: " + JSON.stringify(sentBody));
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
		t.Fatalf("rollback decision transport JS runtime fixture failed: %v\n%s", err, out)
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
