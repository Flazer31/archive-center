package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSessionRouteAdapterUsesOfficialStableHostIdentityAndReadback(t *testing.T) {
	src := readArchiveCenterJS(t)
	for _, needle := range []string{
		`typeof char.chaId === "string"`,
		`identity.stableCharacterId = char.chaId.trim()`,
		`identity.chatUniqueId = String(activeChat.id || "").trim()`,
		`stable_character_id: String(observed.stableCharacterId`,
		`host_chat_id: String(observed.hostChatId`,
		`binding_acknowledged`,
		`session_pin_readback_mismatch`,
		`legacy_index_fallback`,
		`branch_id_state: "not_exposed_by_risuai"`,
		`message_swipe_id_state: Number.isInteger(message.swipeId) ? "observed" : "not_present"`,
	} {
		if !strings.Contains(src, needle) {
			t.Errorf("session route adapter missing %q", needle)
		}
	}
}

func TestPluginStartupDoesNotRequireOpeningArchiveCenterAndHUDIsPrimed(t *testing.T) {
	src := readArchiveCenterJS(t)
	initSource := extractJSFunctionBlockForTest(t, src, "async function init()")
	syncIndex := strings.Index(initSource, `const syncAck = await syncConfigToBackend(settings)`)
	queueRestoreIndex := strings.Index(initSource, `await loadFailedQueueFromStorage()`)
	if syncIndex < 0 || queueRestoreIndex < 0 || syncIndex > queueRestoreIndex {
		t.Fatal("persisted backend config is not synchronized before optional startup restoration")
	}
	if !strings.Contains(initSource, `ensureActiveChatCompletedTurnsBackfilled(startupSessionId, { reason: "plugin_init" })`) {
		t.Fatal("plugin startup still relies on opening Timeline to recover completed active-chat pairs")
	}

	beforeRequestSource := extractJSFunctionBlockForTest(t, src, "async function onBeforeRequest(payload, type)")
	if !strings.Contains(beforeRequestSource, `primeTurnWorkflowHUD(orchRequestId)`) {
		t.Fatal("beforeRequest does not render a host-observed HUD state immediately")
	}
	afterRequestSource := extractJSFunctionBlockForTest(t, src, "function onAfterRequest(content, type)")
	if !strings.Contains(afterRequestSource, `ensureActiveChatCompletedTurnsBackfilled(chatSessionId, {`) ||
		!strings.Contains(afterRequestSource, `reason: "after_request_user_input_missing"`) ||
		!strings.Contains(afterRequestSource, `hostContext: persistenceHostContext`) {
		t.Fatal("missing startup input capture is not handed to the existing active-chat recovery owner")
	}
	primeSource := extractJSFunctionBlockForTest(t, src, "function primeTurnWorkflowHUD(requestId)")
	if !strings.Contains(primeSource, `label_key: "turn_hud.stage.prepare_source"`) ||
		!strings.Contains(primeSource, `consumeTurnWorkflowHUD({`) {
		t.Fatal("HUD priming can still leave an empty surface before the first backend revision")
	}
}

func TestCompleteTurnHUDUsesObservedRequestIDWithoutPublisherLineage(t *testing.T) {
	src := readArchiveCenterJS(t)
	bodySource := extractJSFunctionBlockForTest(t, src, "async function buildCompleteTurnRequestBody(turnIdx, userInput, assistantContent, contextMessages, chatSessionId, improvementTrace, sourceObservationOptions)")
	if !strings.Contains(bodySource, `turn_workflow_request_id: sourceAcceptanceObservation.archive_center_request_correlation_id || ""`) {
		t.Fatal("complete-turn HUD correlation still depends on optional Publisher lineage")
	}
	if !strings.Contains(src, `ARCHIVE CENTER · ${BUILD_ID}`) ||
		!strings.Contains(src, `const BUILD_ID = "4.0.9"`) ||
		!strings.Contains(src, `const BUILD_CHANNEL = "release"`) {
		t.Fatal("4.0.9 release build identity is not visible in the HUD")
	}
	for _, expected := range []string{
		`critic_input_budget_observation: {`,
		`contract_version: "critic_input_budget_observation.v1"`,
		`max_input_context_chars: Math.max(0, Math.floor(Number(settings.maxInputContextChars)))`,
	} {
		if !strings.Contains(bodySource, expected) {
			t.Fatalf("complete-turn does not forward the Critic input budget observation %q", expected)
		}
	}
}

func TestWebRisuDirectBridgeUsesOnlyRequestScopedPlainFetch(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for Web Risu direct bridge fixture")
		}
	}
	src := readArchiveCenterJS(t)
	bridgeFetchSource := extractArchiveCenterJSAsyncFunction(t, src, "bridgeFetch")
	script := `
const settings = {
  bridgeUrl: "https://archive-test.example.ts.net",
  webDirectBridgeEnabled: true,
};
const _lastBridgeFailureByPath = new Map();
function resolveRequestTimeoutMs(){ return 15000; }
function resolveBridgeRuntimeRoute(url){
  return {url, configuredUrl:url, mode:"configured", remoteAuto:false, pageHost:"risuai.xyz", loopbackOnHostedPage:false, mixedContentRisk:false};
}
function warnLog(){}
function debugLog(){}
function extractBridgeErrorDetail(value, fallback){ return value && value.detail || fallback; }
let nativeFetchCalled = false;
let directCall = null;
let directCallCount = 0;
const R = {
  async nativeFetch(){ nativeFetchCalled = true; throw new Error("nativeFetch must not be used"); },
  async risuFetch(url, init){
    directCallCount++;
    directCall = {url, init};
    return {
      ok: true,
      status: 200,
      data: new TextEncoder().encode(JSON.stringify({ready:true, route:"browser_direct"})),
      headers: {"content-type":"application/json"},
    };
  },
};
` + bridgeFetchSource + `
(async()=>{
  const result = await bridgeFetch("/ready", {method:"POST", body:{probe:"web-risu"}});
  if(!result || result.ready !== true) throw new Error("direct response was not decoded");
  if(nativeFetchCalled) throw new Error("nativeFetch was called in direct mode");
  if(!directCall || directCall.url !== "https://archive-test.example.ts.net/ready") throw new Error("direct URL mismatch");
  if(directCall.init.plainFetchForce !== true || directCall.init.rawResponse !== true) throw new Error("request-scoped direct flags missing");
  if(!directCall.init.body || directCall.init.body.probe !== "web-risu") throw new Error("request body was stringified before Risu globalFetch");
  const rawResult = await bridgeFetch("/canon-packs/preview/v1", {method:"POST", body:new Uint8Array([1,2,3]), rawBody:true});
  if(rawResult !== null || nativeFetchCalled) throw new Error("unsupported binary request used a hidden fallback");
  const failure = _lastBridgeFailureByPath.get("/canon-packs/preview/v1");
  if(!failure || failure.error_code !== "web_direct_raw_body_unsupported" || failure.route_mode !== "web_direct_experimental") {
    throw new Error("typed Web direct binary limitation was not recorded");
  }
  settings.webDirectBridgeEnabled = false;
  R.nativeFetch = async function(){
    nativeFetchCalled = true;
    return {ok:true, status:200, async json(){ return {ready:true, route:"native"}; }, async text(){ return ""; }};
  };
  const nativeResult = await bridgeFetch("/ready");
  if(!nativeResult || nativeResult.route !== "native" || !nativeFetchCalled) throw new Error("default nativeFetch route changed");
  if(directCallCount !== 1) throw new Error("disabled direct mode still called risuFetch");
})().catch(err=>{ console.error(err); process.exitCode=1; });
`
	cmd := exec.Command(nodePath, "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Web Risu direct bridge fixture failed: %v\n%s", err, out)
	}
}

func TestReferenceSearchSettingsPanelSavesThroughExistingSettingsOwner(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for reference search settings fixture")
		}
	}
	src := readArchiveCenterJS(t)
	renderSource := extractArchiveCenterJSFunction(t, src, "renderReferenceSearchLlmSettingsPanel")
	attachSource := extractArchiveCenterJSFunction(t, src, "attachReferenceSearchLlmSettingsEvents")
	script := `
const DEFAULT_SETTINGS = {
  sourceSearchPlannerProvider:"openai",
  sourceSearchPlannerTimeoutMs:60000,
};
const settings = {
  sourceSearchPlannerProvider:"openai",
  sourceSearchPlannerApiKey:"old-key",
  sourceSearchPlannerEndpoint:"https://old.example/v1",
  sourceSearchPlannerModel:"old-model",
  sourceSearchPlannerTimeoutMs:60000,
  sourceSearchPlannerTemperature:0.1,
  sourceSearchPlannerReasoningPreset:"auto",
  sourceSearchPlannerReasoningEffort:"none",
  sourceSearchPlannerReasoningBudgetTokens:0,
  sourceSearchPlannerMaxCompletionTokens:512,
};
function escapeAttr(value){ return String(value == null ? "" : value); }
function normalizeSourceSearchLlmProvider(value){ return String(value || "openai"); }
function getAllowedReasoningPresetsForProvider(){ return ["auto","gpt","gemini","claude","glm","custom"]; }
const rendered = (` + renderSource + `)();
if(!rendered.includes('id="mo-sourceSearchPlannerSave"')) throw new Error("reference search save button is not rendered");

function element(value=""){
  return {value, type:"text", style:{}, options:[], disabled:false, textContent:"", listeners:{}, addEventListener(type, handler){ this.listeners[type]=handler; }};
}
const elements = {
  "mo-sourceSearchPlannerProvider":element("ollama"),
  "mo-sourceSearchPlannerApiKey":element("new-key"),
  "mo-sourceSearchPlannerEndpoint":element("https://search.example/v1"),
  "mo-sourceSearchPlannerModel":element("search-model"),
  "mo-sourceSearchPlannerTimeoutMs":element("125000"),
  "mo-sourceSearchPlannerTemperature":element("0.3"),
  "mo-sourceSearchPlannerReasoningPreset":element("glm"),
  "mo-sourceSearchPlannerReasoningEffort":element("low"),
  "mo-sourceSearchPlannerReasoningBudgetTokens":element("2048"),
  "mo-sourceSearchPlannerMaxCompletionTokens":element("4096"),
  "mo-sourceSearchPlannerReasoningGuide":element(),
  "mo-sourceSearchPlannerReasoningBudgetTokensRow":element(),
  "mo-sourceSearchPlannerGenerationOptions":element(),
  "mo-sourceSearchPlannerApiKeyToggle":element(),
  "mo-sourceSearchPlannerSave":element(),
  "mo-sourceSearchPlannerSaveStatus":element(),
};
elements["mo-sourceSearchPlannerReasoningPreset"].options = ["auto","gpt","gemini","claude","glm","custom"].map(value=>({value,hidden:false}));
const document = {getElementById(id){ return elements[id] || null; }};
let savedPatch = null;
async function updateSettings(patch){ savedPatch = patch; return true; }
` + attachSource + `
(async()=>{
  attachReferenceSearchLlmSettingsEvents();
  const save = elements["mo-sourceSearchPlannerSave"];
  if(typeof save.listeners.click !== "function") throw new Error("reference search save action is not attached");
  await save.listeners.click();
  if(!savedPatch || savedPatch.sourceSearchPlannerProvider!=="ollama" ||
      savedPatch.sourceSearchPlannerApiKey!=="new-key" ||
      savedPatch.sourceSearchPlannerEndpoint!=="https://search.example/v1" ||
      savedPatch.sourceSearchPlannerModel!=="search-model" ||
      savedPatch.sourceSearchPlannerTimeoutMs!=="125000" ||
      savedPatch.sourceSearchPlannerTemperature!=="0.3" ||
      savedPatch.sourceSearchPlannerReasoningPreset!=="glm" ||
      savedPatch.sourceSearchPlannerReasoningEffort!=="low" ||
      savedPatch.sourceSearchPlannerReasoningBudgetTokens!=="2048" ||
      savedPatch.sourceSearchPlannerMaxCompletionTokens!=="4096") {
    throw new Error("reference search settings were not forwarded intact: "+JSON.stringify(savedPatch));
  }
  if(save.disabled || elements["mo-sourceSearchPlannerSaveStatus"].textContent!=="저장됨") {
    throw new Error("reference search save completion was not shown");
  }
})().catch(err=>{ console.error(err); process.exitCode=1; });
`
	cmd := exec.Command(nodePath, "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("reference search settings save fixture failed: %v\n%s", err, out)
	}
}

func TestPocketRisuSwipeIdentityIsObservedWithoutInventingAnEditSignal(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for PocketRisu swipe observation fixture")
		}
	}
	src := readArchiveCenterJS(t)
	activeWindow := extractArchiveCenterJSFunction(t, src, "getRisuActiveMessageWindowStart")
	observe := extractArchiveCenterJSAsyncFunction(t, src, "buildCompleteTurnSourceAcceptanceObservation")
	script := `
const _streamingAfterRequestSyntheticCallDepth = 0;
const message = {
  role:"char",
  data:"second swipe",
  chatId:"generation-2",
  time:200,
  generationInfo:{generationId:"generation-2"},
  swipes:["first swipe","second swipe"],
  swipeId:1,
};
const chat = {
  id:"pocket-chat",
  isStreaming:false,
  message:[
    {role:"user",data:"same user",chatId:"user-1",time:100},
    message,
  ],
};
function computeOrchestrationDirtyHashOr1c(value){ return "hash:"+String(value||"").trim(); }
async function resolveCurrentActiveChatObject(){ return {chat}; }
function normalizeAssistantPersistenceCandidate(value){ return String(value||"").trim(); }
function isSameAssistantComparableText(a,b){ return a===b; }
function getSessionSnapshot(){ return {msgCount:0}; }
function debugLog(){}
` + activeWindow + observe + `
(async()=>{
  const second = await buildCompleteTurnSourceAcceptanceObservation(
    "session-1","second swipe",{allowExistingActiveMessage:true,userInput:"same user"}
  );
  if(second.message_swipe_id_state!=="observed" || second.message_swipe_id!==1) {
    throw new Error("PocketRisu current swipe was not observed: "+JSON.stringify(second));
  }
  message.data = "first swipe";
  message.swipeId = 0;
  const first = await buildCompleteTurnSourceAcceptanceObservation(
    "session-1","first swipe",{allowExistingActiveMessage:true,userInput:"same user"}
  );
  if(first.message_chat_id!=="generation-2" || first.generation_id!=="generation-2" ||
      first.message_swipe_id_state!=="observed" || first.message_swipe_id!==0) {
    throw new Error("PocketRisu swipe transition lost official identity: "+JSON.stringify(first));
  }
})().catch(err=>{ console.error(err); process.exitCode=1; });
`
	cmd := exec.Command(nodePath, "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("PocketRisu swipe observation fixture failed: %v\n%s", err, out)
	}
}

func TestCopiedHostChatDoesNotInheritUnscopedLegacySessionPin(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for copied host chat isolation fixture")
		}
	}
	src := readArchiveCenterJS(t)
	resolver := extractArchiveCenterJSAsyncFunction(t, src, "getCurrentChatSessionId")
	script := `
const SESSION_FALLBACK = "default";
const R = {
  async getCurrentChatIndex(){ return 4; },
  async getCurrentCharacterIndex(){ return 3; },
};
let _sessionCache = {charIdx:null,chatIdx:null,sessionId:null,stableCharacterId:"",observedChatUniqueId:""};
async function getActiveChatSessionIdentity(){
  return {
    stableCharacterId:"stable-character",
    stableCharacterIdState:"observed",
    chatUniqueId:"new-host-chat",
    latestUserHash:"user-hash",
    latestAssistantHash:"assistant-hash",
    completedTurnCount:2,
    messageCount:4,
  };
}
async function loadPinnedSessionId(){
  return {
    sessionId:"shared-legacy-session",
    observedChatUniqueId:"",
    stableCharacterId:"",
    pinKeyMode:"legacy_index_fallback",
  };
}
let routedSessionId = "";
let routedBindingMode = "unset";
async function requestBackendSessionRoutingTurnResolution(sessionId, mode, observed){
  routedSessionId = sessionId;
  routedBindingMode = observed.bindingMode;
  return {
    canonicalSessionId:sessionId,
    bindingAcknowledged:true,
    identityResolution:"durable_binding_created",
  };
}
async function savePinnedSessionId(charIdx,chatIdx,sessionId,chatId){
  if (chatId !== "new-host-chat") throw new Error("durable host id missing");
  return sessionId === "char_3_cid_new-host-chat";
}
function isCidSessionId(value){ return /^char_\d+_cid_/.test(String(value||"")); }
function isIndexSessionId(value){ return /^char_\d+_chat_\d+$/.test(String(value||"")); }
function recordRisuForkCopyProvenanceCapture(){}
function recordActiveSessionForDeleteSync(){}
async function reconcileDeletedActiveSessionsWithBackend(){}
function warnLog(){}
` + resolver + `
(async()=>{
  const resolved = await getCurrentChatSessionId();
  if (resolved !== "char_3_cid_new-host-chat") throw new Error("copied chat inherited legacy session: "+resolved);
  if (routedSessionId !== "char_3_cid_new-host-chat") throw new Error("backend route received legacy session: "+routedSessionId);
  if (routedBindingMode !== "") throw new Error("unscoped legacy pin was promoted: "+routedBindingMode);
})().catch(err=>{ console.error(err); process.exitCode=1; });
`
	cmd := exec.Command(nodePath, "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("copied host chat isolation fixture failed: %v\n%s", err, out)
	}
}

func TestOfficialActiveTailContentChangeCanReachCanonicalReplacement(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for active-tail edit fixture")
		}
	}
	src := readArchiveCenterJS(t)
	backfill := extractArchiveCenterJSAsyncFunction(t, src, "backfillOneActiveChatCompletedTurn")
	preflight := extractArchiveCenterJSAsyncFunction(t, src, "preflightActiveChatBackfillIdentity")
	ensure := extractArchiveCenterJSAsyncFunction(t, src, "ensureActiveChatCompletedTurnsBackfilled")
	script := `
const SESSION_FALLBACK = "default";
const settings = {enabled:true,dbEnabled:true};
const _activeChatBackfillInFlight = new Set();
let completeTurnCalls = 0;
let builtOptions = null;
let postedBody = null;
let routingResult = {status:"resolved",turnIndex:2,localTurnIndex:2,baseline:null};
async function requestBackendSessionRoutingTurnResolution(){
  return routingResult;
}
function extractActiveChatMessageList(){ return []; }
function buildRisuWorldlineObservationFromMessages(){ return null; }
function buildRollbackAssistantObservations(){ return []; }
async function fetchCanonicalChatLogsForTurn(){
  return [
    {role:"user",content:"old user"},
    {role:"assistant",content:"old answer"},
  ];
}
function chatLogItemsContainRole(items,role){ return items.some(item=>item.role===role); }
function chatLogItemsContainRoleContent(items,role,content){ return items.some(item=>item.role===role&&item.content===content); }
function setTurnCounterAtLeast(){}
async function markActiveChatBackfillSaved(){}
let ledgerEntries = {"session-1:2":{hash:"saved-hash"}};
async function loadActiveChatBackfillLedger(){ return {entries:ledgerEntries}; }
async function buildCompleteTurnRequestBody(turn,user,assistant,context,sid,unused,options){
  builtOptions = options;
  return {chat_session_id:sid,turn_index:turn,user_content:user,assistant_content:assistant,client_meta:{}};
}
async function tryCompleteTurn(turn,user,assistant,context,sid,unused,body){
  completeTurnCalls++;
  postedBody = body;
  return {status:"ok",save_ok:true,turn_index:turn};
}
function completeTurnNeedsFreshReconciliationRetry(){ return false; }
async function verifyAndRepairCompleteTurnChatLogs(){ return {status:"ok"}; }
function buildCompleteTurnQueuePayload(){ return null; }
async function persistFailedQueueAdmission(){ throw new Error("queue must not run"); }
function enqueue(){ throw new Error("queue must not run"); }
function updateRuntimeState(){}
async function resolveCurrentActiveChatObject(){ return {chat:{message:[]}}; }
function extractActiveChatComparableMessages(){ return []; }
function buildCompletedTurnPairsFromActiveChatMessages(){
  return [
    {userContent:"oldest user",assistantContent:"oldest answer",risuUserMessageIndex:0,risuAssistantMessageIndex:1,hash:"unsaved-oldest"},
    {userContent:"saved user",assistantContent:"saved answer",risuUserMessageIndex:2,risuAssistantMessageIndex:3,hash:"saved-hash"},
    {userContent:"older user",assistantContent:"older answer",risuUserMessageIndex:4,risuAssistantMessageIndex:5,hash:"unsaved-older"},
    {userContent:"edited user",assistantContent:"edited answer",risuUserMessageIndex:6,risuAssistantMessageIndex:7,hash:"unsaved-tail"},
  ];
}
` + backfill + "\n" + preflight + "\n" + ensure + `
(async()=>{
  const pair = {
    userContent:"edited user",
    assistantContent:"edited answer",
    contextMessages:[],
    risuUserMessageIndex:2,
    risuAssistantMessageIndex:3,
    hash:"pair-hash",
    source:"risu_active_chat_complete_turn_backfill",
  };
  routingResult = {status:"worldline_ownership_unresolved",turnIndex:0,localTurnIndex:1,baseline:null};
  const unresolved = await backfillOneActiveChatCompletedTurn("session-1",pair,{
    reason:"before_request",
    routingContext:"automatic_active_chat_full_sweep",
  });
  if (unresolved.status !== "skipped" || unresolved.reason !== "worldline_ownership_unresolved") {
    throw new Error("unresolved worldline reached backfill: "+JSON.stringify(unresolved));
  }
  if (completeTurnCalls !== 0) throw new Error("unresolved worldline reached complete-turn");
  routingResult = {status:"resolved",turnIndex:2,localTurnIndex:2,baseline:null};
  const ordinary = await backfillOneActiveChatCompletedTurn("session-1",pair,{reason:"timeline_refresh"});
  if (ordinary.status !== "exists" || ordinary.reason !== "raw_turn_content_conflict_existing") {
    throw new Error("ordinary historical conflict was allowed: "+JSON.stringify(ordinary));
  }
  if (completeTurnCalls !== 0) throw new Error("ordinary conflict reached complete-turn");

  const replacement = await backfillOneActiveChatCompletedTurn("session-1",pair,{
    reason:"before_request",
    hostObservedActiveTailReplacement:true,
  });
  if (replacement.status !== "saved" || completeTurnCalls !== 1) {
    throw new Error("official active-tail change did not reach complete-turn: "+JSON.stringify(replacement));
  }
  if (!builtOptions || builtOptions.allowExistingActiveMessage !== true) {
    throw new Error("official active message observation was not enabled");
  }
  const meta = postedBody && postedBody.client_meta && postedBody.client_meta.active_chat_backfill;
  if (!meta || meta.replacement_observation_state !== "observed" ||
      meta.replacement_observation !== "host_observed_active_completed_tail_content_change") {
    throw new Error("replacement observation missing: "+JSON.stringify(meta));
  }

  const flags = [];
  const routingContexts = [];
  const observedHashes = [];
  const original = backfillOneActiveChatCompletedTurn;
  backfillOneActiveChatCompletedTurn = async function(sid,observedPair,options){
    flags.push(options.hostObservedActiveTailReplacement === true);
    routingContexts.push(String(options.routingContext||""));
    observedHashes.push(observedPair.hash);
    return {status:"exists",turnIndex:flags.length};
  };
  await ensureActiveChatCompletedTurnsBackfilled("session-1",{reason:"before_request"});
  if (JSON.stringify(observedHashes) !== JSON.stringify(["unsaved-oldest","unsaved-older","unsaved-tail"])) {
    throw new Error("all and only unsaved pairs must be considered: "+JSON.stringify(observedHashes));
  }
  if (JSON.stringify(flags) !== JSON.stringify([false,false,true])) {
    throw new Error("only the actual latest visible pair must be replacement-eligible: "+JSON.stringify(flags));
  }
  if (routingContexts.some(Boolean)) throw new Error("ordinary chat unexpectedly received worldline routing context");
  flags.length = 0;
  routingContexts.length = 0;
  observedHashes.length = 0;
  ledgerEntries = {
    "session-1:1":{hash:"unsaved-oldest"},
    "session-1:2":{hash:"saved-hash"},
    "session-1:4":{hash:"unsaved-tail"},
  };
  await ensureActiveChatCompletedTurnsBackfilled("session-1",{reason:"before_request"});
  backfillOneActiveChatCompletedTurn = original;
  if (JSON.stringify(observedHashes) !== JSON.stringify(["unsaved-older"]) || flags[0] !== false) {
    throw new Error("a saved active tail must not make an older unsaved pair replacement-eligible: "+JSON.stringify({observedHashes,flags}));
  }
})().catch(err=>{ console.error(err); process.exitCode=1; });
`
	cmd := exec.Command(nodePath, "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("active-tail edit fixture failed: %v\n%s", err, out)
	}
}

func TestActiveChatWorldlinePreflightSeparatesInheritedPrefixBeforeBackfill(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for active-chat worldline preflight fixture")
		}
	}
	src := readArchiveCenterJS(t)
	builder := extractJSFunctionBlockForTest(t, src, "function buildRisuWorldlineObservationFromMessages(messages, observedAtMs, hostSignalSource)")
	preflight := extractArchiveCenterJSAsyncFunction(t, src, "preflightActiveChatBackfillIdentity")
	ensure := extractArchiveCenterJSAsyncFunction(t, src, "ensureActiveChatCompletedTurnsBackfilled")
	script := `
const SESSION_FALLBACK = "default";
const settings = {enabled:true,dbEnabled:true};
const _activeChatBackfillInFlight = new Set();
let routeState = "confirmed";
let rawChat = {id:"child-chat",message:[
  {role:"user",chatId:"user-anchor",data:"u"},
  {role:"char",chatId:"fork-source",data:"a"},
  {role:"comment",disabled:true,data:"{{specialcomment::branchedfrom::parent-chat::Parent::fork-source::}}"},
  {role:"user",chatId:"child-user",data:"u2"},
  {role:"char",chatId:"child-answer",data:"a2"},
]};
let order = [];
let routed = [];
let backfilled = [];
async function resolveCurrentActiveChatObject(){ return {chat:rawChat}; }
function extractActiveChatMessageList(chat){ return chat && Array.isArray(chat.message) ? chat.message : []; }
function extractActiveChatComparableMessages(){ return []; }
function buildRollbackAssistantObservations(){ return []; }
function buildCompletedTurnPairsFromActiveChatMessages(){
  return [
    {hash:"pair-a",risuUserMessageIndex:0,observedPairOrdinal:1},
    {hash:"pair-b",risuUserMessageIndex:2,observedPairOrdinal:2},
  ];
}
async function requestBackendSessionRoutingTurnResolution(sid,mode,facts){
  order.push("route");
  routed.push({sid,mode,facts});
  return {status:"normal",worldline:{state:routeState,reason:routeState}};
}
async function loadActiveChatBackfillLedger(){ return {entries:{}}; }
async function backfillOneActiveChatCompletedTurn(sid,pair,options){
  order.push("backfill");
  backfilled.push({sid,pair,options});
  return {status:"skipped",turnIndex:0};
}
function updateRuntimeState(){}
` + builder + "\n" + preflight + "\n" + ensure + `
const assert = (condition,message) => { if (!condition) throw new Error(message); };
(async()=>{
  const confirmed = await ensureActiveChatCompletedTurnsBackfilled("child-session",{reason:"plugin_init"});
  assert(confirmed.status === "skipped", "fixture backfills should report skipped");
  assert(order[0] === "route" && order.slice(1).every(item=>item === "backfill"), "worldline preflight did not run first: "+JSON.stringify(order));
  assert(routed.length === 1 && routed[0].mode === "identity", "preflight must use the existing identity route");
  const observation = routed[0].facts.worldlineObservation;
  assert(observation.contract_version === "risu_worldline_observation.v2", "preflight contract mismatch");
  assert(observation.host_signal_source === "active_chat_pre_backfill", "preflight was falsely labeled as output");
  assert(backfilled.length === 2 && backfilled.every(item=>item.options.routingContext === "automatic_active_chat_full_sweep"), "confirmed branch pairs lack Go routing context");

  routeState = "unresolved";
  order = []; routed = []; backfilled = [];
  const unresolved = await ensureActiveChatCompletedTurnsBackfilled("child-session",{reason:"before_request"});
  assert(unresolved.reason === "worldline_ownership_unresolved", "unresolved marker did not fail closed");
  assert(routed.length === 1 && backfilled.length === 0, "unresolved marker reached pair backfill");

  rawChat = {id:"ordinary-chat",message:[{role:"user",chatId:"ordinary-user",data:"u"},{role:"char",chatId:"ordinary-answer",data:"a"}]};
  order = []; routed = []; backfilled = [];
  await ensureActiveChatCompletedTurnsBackfilled("ordinary-session",{reason:"plugin_init"});
  assert(routed.length === 0, "ordinary chat created a worldline preflight");
  assert(backfilled.length === 2 && backfilled.every(item=>!item.options.routingContext), "ordinary backfill behavior changed");
})().catch(err=>{ console.error(err); process.exitCode=1; });
`
	cmd := exec.Command(nodePath, "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("active-chat worldline preflight fixture failed: %v\n%s", err, out)
	}
}

func TestSessionRouteBindingOrPinFailureCannotCacheCanonicalRoute(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for session route acknowledgement fixture")
		}
	}
	src := readArchiveCenterJS(t)
	ackFailureHelper := extractArchiveCenterJSSyncFunction(t, src, "isSessionRouteAcknowledgementFailure")
	resolver := extractArchiveCenterJSAsyncFunction(t, src, "resolveCanonicalWriteSessionId")
	script := `
const SESSION_FALLBACK = "default";
const SESSION_WRITE_CANONICAL_CACHE_MS = 10000;
const settings = {enabled:true,dbEnabled:true};
const R = {
  async getCurrentChatIndex(){ return 8; },
  async getCurrentCharacterIndex(){ return 3; },
};
function normalizeSessionId(value){ return String(value||"").trim(); }
async function getActiveChatSessionIdentity(){
  return {
    stableCharacterId:"stable-character",
    stableCharacterIdState:"observed",
    chatUniqueId:"opaque-chat",
    latestUserHash:"user-hash",
    latestAssistantHash:"assistant-hash",
    completedTurnCount:4,
    messageCount:8,
    isFreshChat:false,
  };
}
let bindingAcknowledged = false;
let pinResult = true;
let pinCalls = 0;
async function requestBackendSessionRoutingTurnResolution(){
  return {
    canonicalSessionId:"canonical-session",
    bindingAcknowledged,
    identityResolution:"durable_binding_existing",
  };
}
async function savePinnedSessionId(){ pinCalls++; return pinResult; }
let _sessionCache = {charIdx:null,chatIdx:null,sessionId:null,stableCharacterId:"",observedChatUniqueId:""};
let _sessionWriteCanonicalCache = {cacheKey:"",sessionId:"",rawSessionId:"",reason:"",cachedAt:0};
function updateRuntimeState(){}
function warnLog(){}
function isCidSessionId(){ return false; }
function buildCompatSessionReadPlan(){ throw new Error("legacy path must not run"); }
` + ackFailureHelper + "\n" + resolver + `
(async()=>{
  const pristineSessionCache = JSON.stringify(_sessionCache);
  const pristineWriteCache = JSON.stringify(_sessionWriteCanonicalCache);
  let failed = false;
  try { await resolveCanonicalWriteSessionId("requested-session"); } catch (err) {
    failed = err && err.message === "session_route_binding_readback_unverified";
  }
  if (!failed) throw new Error("binding failure did not propagate");
  if (pinCalls !== 0) throw new Error("pin write ran after binding failure");
  if (JSON.stringify(_sessionCache) !== pristineSessionCache || JSON.stringify(_sessionWriteCanonicalCache) !== pristineWriteCache) {
    throw new Error("binding failure mutated route cache");
  }

  bindingAcknowledged = true;
  pinResult = false;
  failed = false;
  try { await resolveCanonicalWriteSessionId("requested-session"); } catch (err) {
    failed = err && err.message === "session_pin_readback_unverified";
  }
  if (!failed) throw new Error("pin readback failure did not propagate");
  if (pinCalls !== 1) throw new Error("unexpected pin call count: "+pinCalls);
  if (JSON.stringify(_sessionCache) !== pristineSessionCache || JSON.stringify(_sessionWriteCanonicalCache) !== pristineWriteCache) {
    throw new Error("pin failure mutated route cache");
  }
})().catch(err=>{ console.error(err); process.exitCode=1; });
`
	cmd := exec.Command(nodePath, "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("session route acknowledgement fixture failed: %v\n%s", err, out)
	}
}

func TestDurableSessionPinRequiresExactReadbackAndUsesV3Record(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for durable pin readback fixture")
		}
	}
	src := readArchiveCenterJS(t)
	makeKey := extractArchiveCenterJSSyncFunction(t, src, "makeSessionPinKey")
	parseRecord := extractArchiveCenterJSSyncFunction(t, src, "parseSessionPinRecord")
	savePin := extractArchiveCenterJSAsyncFunction(t, src, "savePinnedSessionId")
	script := `
const PLUGIN_ID = "archive";
const SESSION_ID_PIN_PREFIX = PLUGIN_ID+"_session_id_pin_v1";
const SESSION_DURABLE_PIN_PREFIX = PLUGIN_ID+"_session_id_pin_v3";
const SESSION_PIN_RECORD_VERSION = "v3";
const SESSION_FALLBACK = "default";
let savedKey = "";
let savedPayload = "";
async function persistentSet(key,payload){ savedKey=key; savedPayload=payload; }
async function persistentGet(){ return JSON.stringify({version:"v3",sessionId:"wrong-session",observedChatUniqueId:"opaque-chat",stableCharacterId:"stable-character"}); }
function warnLog(){}
` + makeKey + "\n" + parseRecord + "\n" + savePin + `
(async()=>{
  const ok = await savePinnedSessionId(9,4,"canonical-session","opaque-chat","stable-character");
  if (ok !== false) throw new Error("mismatched pin readback was accepted");
  if (!savedKey.includes("session_id_pin_v3_character_stable-character_chat_opaque-chat")) {
    throw new Error("durable host identity key not used: "+savedKey);
  }
  const payload = JSON.parse(savedPayload);
  if (payload.version !== "v3" || payload.stableCharacterId !== "stable-character") {
    throw new Error("pin record version/identity mismatch");
  }
})().catch(err=>{ console.error(err); process.exitCode=1; });
`
	cmd := exec.Command(nodePath, "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("durable pin readback fixture failed: %v\n%s", err, out)
	}
}

func TestSessionRouteAcknowledgementFailureCannotReachTurnSavePayload(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for session route save-block fixture")
		}
	}
	src := readArchiveCenterJS(t)
	saveTurn := extractArchiveCenterJSAsyncFunction(t, src, "saveTurnToBackend")
	script := `
const settings = {enabled:true,dbEnabled:true};
let bridgeCalls = 0;
let queued = 0;
async function resolveCanonicalWriteSessionId(){ throw new Error("session_route_binding_readback_unverified"); }
async function getCurrentChatSessionId(){ throw new Error("must not run"); }
function sanitizeForCritic(v){ return v; }
function shouldSkipUserInputPersistence(){ return false; }
async function bridgeFetchWithRetry(){ bridgeCalls++; return {status:"ok"}; }
async function safeCall(fn){ return await fn(); }
function enqueue(){ queued++; }
function updateRuntimeState(){}
function debugLog(){}
function warnLog(){}
function trackTurnIndex(){}
` + saveTurn + `
(async()=>{
  let failed = false;
  try { await saveTurnToBackend(4,"user","assistant","requested-session"); } catch (err) {
    failed = err && err.message === "session_route_binding_readback_unverified";
  }
  if (!failed) throw new Error("unacknowledged route did not block save");
  if (bridgeCalls !== 0 || queued !== 0) throw new Error("save payload or queue reached after route failure");
})().catch(err=>{ console.error(err); process.exitCode=1; });
`
	cmd := exec.Command(nodePath, "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("session route save-block fixture failed: %v\n%s", err, out)
	}
}

func TestBeforeRequestSessionRouteFailureKeepsRisuPayloadRuntime(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for beforeRequest session-route fail-open fixture")
		}
	}
	src := readArchiveCenterJS(t)
	beforeRequest := extractArchiveCenterJSAsyncFunction(t, src, "onBeforeRequest")
	script := beforeRequest + `
const SESSION_FALLBACK = "default";
const settings = {enabled:true};
const _pendingOrchBySession = new Map([[SESSION_FALLBACK, {pending:true}]]);
let _effectiveInputAwaitingNewTurn = true;
let _lastPrepareTurnSource = "backend-off";
let _lastPrepareTurnBundle = null;
let lastTurnTrace = null;
let sessionCalls = 0;
function recordRisuHookLifecycle() {}
function debugLog() {}
function warnLog() {}
function clearArchiveCenterRecomposerBridge() {}
function isSaveType(type) { return type === "model"; }
function extractMessages(payload) { return {messages:payload.messages, hasMessageSlot:true}; }
function normalizeMessagesForOrchestration(messages) { return messages; }
function extractRuntimeCurrentChatTokenInfo() { return {}; }
async function getCurrentChatSessionId() {
  sessionCalls++;
  throw new Error("session_route_binding_readback_unverified");
}
async function resolveCanonicalWriteSessionId() { throw new Error("must not run without a session"); }
function updateRuntimeState() {}
function buildLlmGateBlock() { return {code:"before_request_exception", reason:"route unavailable"}; }
function newTurnTrace() { return {}; }
function applyOrchestrationModuleTransportTraceOr1e() {}
function buildOrchestrationModuleTransportStateOr1e() { return {}; }
function pushTurnHistory() {}
(async()=>{
  const payload = {messages:[{role:"user",content:"keep Risu request"}]};
  const result = await onBeforeRequest(payload,"model");
  if (result !== payload) throw new Error("Risu payload identity changed");
  if (sessionCalls !== 1) throw new Error("failed session route was called again: "+sessionCalls);
  if (_pendingOrchBySession.has(SESSION_FALLBACK)) throw new Error("fallback pending state was not cleared");
  if (!lastTurnTrace || !lastTurnTrace.deliveryGate || lastTurnTrace.deliveryGate.failOpenMainPayload !== true) {
    throw new Error("Risu fail-open trace was not retained");
  }
})().catch(err=>{ console.error(err && err.stack || err); process.exitCode=1; });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("beforeRequest session-route fail-open fixture failed: %v\n%s", err, out)
	}
}

func TestRisuLifecycleRegistrationAndRemovalAreIndependent(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for lifecycle registration fixture")
		}
	}
	src := readArchiveCenterJS(t)
	register := extractJSFunctionBlockForTest(t, src, "async function registerRisuLifecycleHooks()")
	remove := extractJSFunctionBlockForTest(t, src, "async function removeRegisteredRisuHooksOnUnload()")
	script := `
const calls = [];
const R = {
  async addRisuScriptHandler(name){ calls.push("add:"+name); },
  async addRisuReplacer(name){ calls.push("add:"+name); if(name === "beforeRequest") throw new Error("before unavailable"); },
  async onUnload(){ calls.push("add:unload"); },
  async removeRisuScriptHandler(name){ calls.push("remove:"+name); },
  async removeRisuReplacer(name){ calls.push("remove:"+name); if(name === "beforeRequest") throw new Error("before removal unavailable"); },
};
const LOG_PREFIX = "[test]";
const onInputHook = ()=>{};
const onBeforeRequest = ()=>{};
const onAfterRequest = ()=>{};
const _pendingFinalConfirmations = new Map();
const _finalConfirmationRequestBySession = new Map();
let _pendingFinalConfirmationDrainRequested = false;
const lifecycleStates = {};
function recordRisuHookLifecycle(name,state){ lifecycleStates[name]=state; }
function warnLog(){}
function debugLog(){}
function cancelTurnWorkflowHUDStream(){}
function cancelAllAdminBackgroundJobStreams(){}
function clearArchiveCenterRecomposerBridge(){ calls.push("clear:recomposer"); }
async function unloadTurnWorkflowHUD(){}
` + register + "\n" + remove + `
(async()=>{
  await registerRisuLifecycleHooks();
  if (!calls.includes("add:afterRequest") || !calls.includes("add:unload")) {
    throw new Error("beforeRequest registration failure skipped later hooks: "+calls.join(","));
  }
  if (lifecycleStates.beforeRequest !== "registration_failed") {
    throw new Error("registration failure was not exposed: "+JSON.stringify(lifecycleStates));
  }
  await removeRegisteredRisuHooksOnUnload();
  if (!calls.includes("remove:afterRequest")) {
    throw new Error("beforeRequest removal failure skipped afterRequest removal: "+calls.join(","));
  }
  if (!calls.includes("clear:recomposer")) throw new Error("unload left Recomposer bridge live");
})().catch(err=>{ console.error(err); process.exitCode=1; });
`
	cmd := exec.Command(nodePath, "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("lifecycle registration fixture failed: %v\n%s", err, out)
	}
}

func TestAcceptedFinalQueueAdmissionFailureKeepsHostContextRecoverable(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for accepted-final recovery fixture")
		}
	}
	src := readArchiveCenterJS(t)
	serialize := extractJSFunctionBlockForTest(t, src, "function serializeAcceptedFinalRecoveryPayload(payload)")
	build := extractJSFunctionBlockForTest(t, src, "function buildAcceptedFinalRecoveryPayload(sessionId, pair, sourceAcceptanceFinality, reason, turnIndex)")
	admit := extractJSFunctionBlockForTest(t, src, "async function admitAcceptedFinalTransportRecovery(sessionId, pair, sourceAcceptanceFinality, reason, turnIndex)")
	backfill := extractArchiveCenterJSAsyncFunction(t, src, "backfillOneActiveChatCompletedTurn")
	observe := extractJSFunctionBlockForTest(t, src, "function observePendingFinalConfirmationAtHostSignal(sessionId, signalSource, hostSnapshot = null)")
	script := `
const _finalConfirmationRequestBySession = new Map();
const _pendingOrchBySession = new Map();
let lastOrchResult = null;
let admissionType = "";
let admissionPayload = null;
function computeOrchestrationDirtyHashOr1c(value){ return "h"+String(value||"").length; }
function normalizeAssistantPersistenceCandidate(value){ return String(value||"").trim(); }
function extractActiveChatComparableMessages(){
  return [
    {role:"user",content:"hello",risuMessageIndex:0},
    {role:"assistant",content:"committed answer",risuMessageIndex:1},
  ];
}
const R = {
  async getCurrentCharacterIndex(){ return 2; },
  async getCurrentChatIndex(){ return 3; },
  async getChatFromIndex(){ return {
    id:"host-chat",
    isStreaming:false,
    message:[
      {role:"user",data:"hello",disabled:false,chatId:"u1",time:1},
      {role:"char",data:"committed answer",disabled:false,chatId:"a1",time:2,generationInfo:{generationId:"g1"}},
    ],
  }; },
};
async function requestBackendSessionRoutingTurnResolution(){ return {status:"resolved",turnIndex:4,localTurnIndex:4,baseline:null}; }
async function fetchCanonicalChatLogsForTurn(){ return []; }
function chatLogItemsContainRoleContent(){ return false; }
function chatLogItemsContainRole(){ return false; }
async function buildCompleteTurnRequestBody(){ return null; }
function enqueue(type,payload){ admissionType=type; admissionPayload=payload; return {queued:true,admitted:true,code:"queued"}; }
async function persistFailedQueueAdmission(){ return {queued:false,admitted:false,code:"failed_queue_persistence_failed"}; }
function updateRuntimeState(){}
function warnLog(){}
function setTurnCounterAtLeast(){}
async function markActiveChatBackfillSaved(){}
async function tryCompleteTurn(){ throw new Error("must not post without a request body"); }
async function verifyAndRepairCompleteTurnChatLogs(){}
function buildCompleteTurnQueuePayload(){ return null; }
` + serialize + "\n" + build + "\n" + admit + "\n" + backfill + "\n" + observe + `
(async()=>{
  const exactRecoveryContext = Array.from({length:25},(_,index)=>({
    role:index%2===0?"user":"assistant",
    content:index===0?"x".repeat(2101):"context-"+index,
  }));
  const exactRecovery = serializeAcceptedFinalRecoveryPayload({
    chat_session_id:"session-1",
    context_messages:exactRecoveryContext,
  });
  if (exactRecovery.context_messages.length !== exactRecoveryContext.length ||
      exactRecovery.context_messages[0].content !== exactRecoveryContext[0].content) {
    throw new Error("accepted-final recovery truncated exact critic context");
  }
  const context = {
    sessionId:"session-1",
    state:"candidate_observed",
    characterIndex:2,
    chatIndex:3,
    hostChatId:"host-chat",
    userMessageIndex:0,
    userObservedContentHash:computeOrchestrationDirtyHashOr1c("hello"),
    requestMessageCount:1,
    requestId:"request-1",
    requestType:"model",
    baselineAssistantIndex:-1,
    baselineAssistantContentHash:"",
    baselineGenerationId:"",
    baselineAssistantTimeMs:0,
    afterRequestCandidateHash:computeOrchestrationDirtyHashOr1c("committed answer"),
    userMessageChatId:"u1",
    userMessageTimeMs:1,
  };
  _finalConfirmationRequestBySession.set("session-1", context);
  const accepted = await observePendingFinalConfirmationAtHostSignal("session-1", "beforeRequest");
  if (!accepted.accepted || context.state !== "accepted") throw new Error("candidate was not accepted before scheduling");
  await new Promise(resolve=>setTimeout(resolve,0));
  if (admissionType !== "accepted_final") throw new Error("accepted final was not sent to durable recovery queue");
  if (!admissionPayload || admissionPayload.assistant_content !== "committed answer") throw new Error("accepted content was not preserved");
  if (context.state !== "candidate_observed" || context.acceptedObservationKey) {
    throw new Error("failed durable admission left context unrecoverably accepted");
  }
})().catch(err=>{ console.error(err); process.exitCode=1; });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("accepted-final recovery fixture failed: %v\n%s", err, out)
	}
}

func TestSessionNormalizeResultRenderingSeparatesCompletionErrorsAndDeferredWork(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for session-normalize rendering fixture")
		}
	}
	src := readArchiveCenterJS(t)
	normalizeFailure := extractArchiveCenterJSFunction(t, src, "normalizeSessionNormalizeFailure")
	localizeFailure := extractArchiveCenterJSFunction(t, src, "localizeSessionNormalizeFailure")
	render := extractArchiveCenterJSFunction(t, src, "renderSessionNormalizeResultHtml")
	script := normalizeFailure + "\n" + localizeFailure + "\n" + render + `
function escapeAttr(value){ return String(value || ""); }
function formatHierarchyBlockedSummary(){ return ""; }
function formatTurnIndexPreview(){ return ""; }
const labels = {
  "sessionNormalize.completed":"콜드 스타트 완료",
  "sessionNormalize.completedWithErrors":"오류를 포함해 종료됨",
  "sessionNormalize.failed":"콜드 스타트 실패",
  "sessionNormalize.close":"닫기",
  "sessionNormalize.succeeded":"성공",
  "sessionNormalize.failures":"실패",
  "sessionNormalize.skipped":"건너뜀",
  "sessionNormalize.deferred":"대기",
  "sessionNormalize.technicalDetails":"기술 정보",
  "sessionNormalize.failureTurn":"{turn}턴 실패",
  "sessionNormalize.moreFailures":"외 {count}건",
  "sessionNormalize.error.critic_provider_timeout":"평론가 LLM 응답이 설정된 제한시간을 넘겼습니다.",
  "sessionNormalize.error.generic":"이 턴을 처리하지 못했습니다.",
};
[
  ["raw","원문"],["memories","기억"],["evidence","직접 근거"],["kg","관계 지식"],
  ["rules","세계 규칙"],["episodes","에피소드"],["chapters","챕터"],["arcs","아크"],
  ["sagas","사가"],["vector","벡터"],
].forEach(([key,value])=>labels["sessionNormalize.count."+key]=value);
function t(key){ return labels[key] || key; }
function tf(key,vars){
  let text=t(key);
  Object.keys(vars||{}).forEach(name=>{ text=text.replaceAll("{"+name+"}",String(vars[name])); });
  return text;
}
function assertIncludes(text, needle, label) {
  if (!String(text).includes(needle)) throw new Error(label + ": " + text);
}
const ok = renderSessionNormalizeResultHtml({
  status:"ok",
  counts_after:{},
  rescan:{candidate_count:2,succeeded:2,deferred:0,queued:0},
  reindex:{},
});
assertIncludes(ok, "콜드 스타트 완료", "ok heading");
assertIncludes(ok, 'data-session-normalize-dismiss', "terminal close button");
const partial = renderSessionNormalizeResultHtml({
  status:"partial_error",
  counts_after:{},
  rescan:{
    candidate_count:3,succeeded:1,failed:1,skipped:0,deferred:1,queued:2,
    failed_turns:[{turn_index:1,reason:"CRITIC_PROVIDER_TIMEOUT: context deadline exceeded"}],
  },
  reindex:{},
});
assertIncludes(partial, "오류를 포함해 종료됨", "partial-error heading");
assertIncludes(partial, "mo-session-normalize-result is-fail", "partial-error severity");
assertIncludes(partial, "평론가 LLM 응답이 설정된 제한시간을 넘겼습니다.", "localized failure cause");
assertIncludes(partial, "queued=2", "deferred queue technical count");
if (partial.includes("콜드 스타트 완료")) throw new Error("partial error was rendered as completed");
const failed = renderSessionNormalizeResultHtml({status:"failed",counts_after:{},rescan:{},reindex:{}});
assertIncludes(failed, "콜드 스타트 실패", "failed heading");
`
	cmd := exec.Command(nodePath, "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("session-normalize rendering fixture failed: %v\n%s", err, out)
	}
}

func TestPostOutputSecondaryPersistenceKeepsFullHostContext(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for post-output persistence context fixture")
		}
	}
	src := readArchiveCenterJS(t)
	build := extractArchiveCenterJSFunction(t, src, "buildPostOutputSecondaryRequestContext")
	script := build + `
const messages=Array.from({length:50},(_,index)=>({
  role:index%2===0?"user":"assistant",
  content:index===0?"x".repeat(2101):"message-"+index,
}));
function getLastNonEmptyComparableMessage(list){ return list[list.length-1]; }
function buildCompletedTurnPairsFromActiveChatMessages(){
  return [{userContent:"user-tail",assistantContent:"message-49"}];
}
function normalizeAssistantPersistenceCandidate(value){ return String(value || "").trim(); }
function isSameAssistantComparableText(left,right){ return left===right; }
function buildRollbackAssistantObservations(list){
  return list.filter(item=>item.role==="assistant").map((item,index)=>({
    message_id:"assistant-"+index,
    message_index:index,
    content_hash:String(item.content || ""),
  }));
}
const result=buildPostOutputSecondaryRequestContext(messages);
if (!result || result.contextMessages.length!==messages.length ||
    result.contextMessages[0].content!==messages[0].content ||
    result.assistantObservationScope!=="full_active_chat" ||
    !Array.isArray(result.assistantObservations) || result.assistantObservations.length!==25) {
  throw new Error("post-output persistence truncated exact host context");
}
`
	cmd := exec.Command(nodePath, "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("post-output persistence context fixture failed: %v\n%s", err, out)
	}
}

func TestTurnWorkflowHUDStopsAfterNonterminalEOF(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for HUD EOF fixture")
		}
	}
	src := readArchiveCenterJS(t)
	line := extractJSFunctionBlockForTest(t, src, "async function consumeTurnWorkflowHUDStreamLine(line, token, requestId)")
	consume := extractJSFunctionBlockForTest(t, src, "async function consumeTurnWorkflowHUDStream(reader, token, requestId)")
	prime := extractJSFunctionBlockForTest(t, src, "function primeTurnWorkflowHUD(requestId)")
	start := extractJSFunctionBlockForTest(t, src, "function startTurnWorkflowHUDWatch(requestId)")
	script := `
const settings = {bridgeUrl:"http://bridge"};
const TURN_WORKFLOW_HUD_CONTRACT = "turn_workflow_hud.v3";
let _turnWorkflowHUDWatchToken = 0;
let _turnWorkflowHUDActiveRequestId = "";
let _turnWorkflowHUDWatchRunning = false;
let _turnWorkflowHUDLastRevision = 0;
let _turnWorkflowHUDTerminalRequestId = "";
let _turnWorkflowHUDStreamAbortController = null;
let _turnWorkflowHUDStreamReader = null;
let _turnWorkflowHUDRenderChain = Promise.resolve();
const _turnWorkflowHUDHostWarningsByRequestId = new Map();
const opened = [];
let transportError = "";
let dismissedRequest = "";
function turnWorkflowHUDIsEnabled(){ return true; }
function dismissTurnWorkflowHUD(requestId){ dismissedRequest=String(requestId||""); _turnWorkflowHUDActiveRequestId=""; }
function cancelTurnWorkflowHUDStream(){
  if (_turnWorkflowHUDStreamAbortController) _turnWorkflowHUDStreamAbortController.abort();
  _turnWorkflowHUDStreamAbortController = null;
  _turnWorkflowHUDStreamReader = null;
}
function clearTurnWorkflowHUDTimer(){}
function turnWorkflowHUDHasHostWarning(requestId){
  const warnings=_turnWorkflowHUDHostWarningsByRequestId.get(String(requestId||"").trim());
  return Array.isArray(warnings) && warnings.length>0;
}
function queueTurnWorkflowHUDOperation(name,fn){ Promise.resolve().then(fn); }
async function removeTurnWorkflowHUDDismissListeners(){}
async function ensureTurnWorkflowHUDRoot(){ return {async setInnerHTML(){}}; }
function getRequestTimeoutSettingMs(){ return 1000; }
function resolveBridgeRuntimeRoute(){ return {url:"http://bridge"}; }
function turnWorkflowHUDStreamFailure(code,message){ const err=new Error(message); err.code=code; return err; }
function consumeTurnWorkflowHUD(view){ _turnWorkflowHUDLastRevision=Number(view.revision||0); return true; }
function renderTurnWorkflowHUDTransportError(id,code){ transportError=code||"error"; }
function debugLog(){}
function readerFor(view){
  let step=0;
  return {
    async read(){
      if(step++===0) return {done:false,value:new TextEncoder().encode(JSON.stringify(view)+"\n")};
      return {done:true};
    },
    async cancel(){},
  };
}
async function openTurnWorkflowHUDStream(url){
  opened.push(url);
  return readerFor({request_id:"req-1",revision:1,status:"running"});
}
` + line + "\n" + consume + "\n" + prime + "\n" + start + `
(async()=>{
  startTurnWorkflowHUDWatch("req-1");
  for(let i=0;i<50 && _turnWorkflowHUDWatchRunning;i++) await new Promise(resolve=>setTimeout(resolve,1));
  if (transportError) throw new Error("nonterminal EOF was misreported as turn failure: "+transportError);
  if (dismissedRequest) throw new Error("HUD-only stream loss discarded the active workflow identity: "+dismissedRequest);
  if (_turnWorkflowHUDActiveRequestId !== "req-1") throw new Error("HUD-only stream loss cleared the active request");
  if (opened.length !== 1) throw new Error("nonterminal EOF caused hidden reconnect count="+opened.length);
})().catch(err=>{ console.error(err); process.exitCode=1; });
`
	cmd := exec.Command(nodePath, "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("HUD EOF fixture failed: %v\n%s", err, out)
	}
}

func TestAdminJobCancelAndColdStartProgressUseBackendSnapshot(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for admin job fixture")
		}
	}
	src := readArchiveCenterJS(t)
	value := extractJSFunctionBlockForTest(t, src, "function adminJobProgressValue(progress, keys, fallback)")
	stageLabel := extractJSFunctionBlockForTest(t, src, "function sessionNormalizeStageLabel(stage)")
	normalizeFailure := extractJSFunctionBlockForTest(t, src, "function normalizeSessionNormalizeFailure(item)")
	localizeFailure := extractJSFunctionBlockForTest(t, src, "function localizeSessionNormalizeFailure(failure)")
	renderNormalize := extractJSFunctionBlockForTest(t, src, "function renderSessionNormalizeJobProgressHtml(job)")
	render := extractJSFunctionBlockForTest(t, src, "function renderAdminJobProgressHtml(job, label, kind)")
	applySnapshot := extractJSFunctionBlockForTest(t, src, "function applyAdminBackgroundJobSnapshot(kind, state, jobId, snapshot)")
	cancel := extractJSFunctionBlockForTest(t, src, "async function cancelAdminBackgroundJob(kind, state, jobId)")
	script := `
let requestPath = "";
let requestMethod = "";
const _adminBackgroundJobStreams = new Map();
function escapeAttr(value){ return String(value==null?"":value); }
const labels = {
  "sessionNormalize.running":"콜드 스타트 진행 중",
  "sessionNormalize.failed":"콜드 스타트 실패",
  "sessionNormalize.stageLabel":"현재 단계",
  "sessionNormalize.stage.inspect_before":"저장 상태 확인",
  "sessionNormalize.progress":"진행",
  "sessionNormalize.succeeded":"성공",
  "sessionNormalize.failures":"실패",
  "sessionNormalize.skipped":"건너뜀",
  "sessionNormalize.backgroundNote":"이 화면을 이동해도 백엔드에서 계속 진행됩니다.",
  "sessionNormalize.technicalDetails":"기술 정보",
  "sessionNormalize.refresh":"상태 새로고침",
  "sessionNormalize.cancel":"작업 취소",
  "sessionNormalize.failureTurn":"{turn}턴 실패",
  "sessionNormalize.moreFailures":"외 {count}건",
  "sessionNormalize.error.critic_provider_timeout":"평론가 LLM 응답이 설정된 제한시간을 넘겼습니다.",
  "sessionNormalize.error.generic":"이 턴을 처리하지 못했습니다.",
};
function t(key){ return labels[key] || key; }
function tf(key,vars){
  let text=t(key);
  Object.keys(vars||{}).forEach(name=>{ text=text.replaceAll("{"+name+"}",String(vars[name])); });
  return text;
}
function refreshExplorerUI(){}
function cancelAdminBackgroundJobStream(){ return true; }
function markAdminBackgroundJobStreamUnavailable(){ throw new Error("unexpected cancel transport failure"); }
function getRequestTimeoutSettingMs(){ return 1000; }
async function bridgeFetch(path,options){
  requestPath=path; requestMethod=options.method;
  return {job_id:"job-1",status:"cancelled",terminal:true,progress:{stage:"cancelled"}};
}
async function safeCall(fn){ return await fn(); }
` + value + "\n" + stageLabel + "\n" + normalizeFailure + "\n" + localizeFailure + "\n" + renderNormalize + "\n" + render + "\n" + applySnapshot + "\n" + cancel + `
(async()=>{
  const html = renderAdminJobProgressHtml({
    job_id:"job-1",
    status:"running",
    terminal:false,
    request:{repair_entry_count:99},
    progress:{progress_percent:8,processed:0,display_total:3,candidate_count:3},
  },"Normalize","session_normalize");
  if (!html.includes("8%") || !html.includes("진행 <strong>0/3</strong>")) {
    throw new Error("cold-start total did not render backend progress ViewModel: "+html);
  }
  const laterStage = renderAdminJobProgressHtml({
    job_id:"job-1",status:"running",terminal:false,request:{repair_entry_count:99},
    progress:{stage:"inspect_after",progress_percent:90,processed:0,display_total:0},
  },"Normalize","session_normalize");
  if (!laterStage.includes("90%") || !laterStage.includes("진행 <strong>0/0</strong>") || laterStage.includes("0/99")) {
    throw new Error("request raw-repair count leaked into another stage: "+laterStage);
  }
  const failed = renderAdminJobProgressHtml({
    job_id:"job-1",status:"running",terminal:false,
    progress:{
      stage:"critic_rescan_backfill",progress_percent:23,processed:3,display_total:26,
      succeeded:2,failed_count:1,failed_turns:[
        {turn_index:1,reason:"CRITIC_PROVIDER_TIMEOUT: context deadline exceeded"},
      ],
    },
  },"Normalize","session_normalize");
  if (!failed.includes("mo-session-normalize-status-fail") ||
      !failed.includes("평론가 LLM 응답이 설정된 제한시간을 넘겼습니다.")) {
    throw new Error("localized cold-start failure was not emphasized: "+failed);
  }
  if (!html.includes('data-admin-job-cancel="session_normalize"')) throw new Error("cancel UI is missing");
  const state={loading:true,error:null,result:null,job:{job_id:"job-1",status:"running",terminal:false}};
  await cancelAdminBackgroundJob("session_normalize",state,"job-1");
  if(requestPath !== "/admin/jobs/job-1" || requestMethod !== "DELETE") throw new Error("wrong cancel request");
  if(state.loading !== false || state.job.status !== "cancelled" || state.error !== null) {
    throw new Error("cancel snapshot did not leave the job restartable");
  }

  const deferredState={loading:true,error:"old",result:null,job:{job_id:"job-deferred",status:"running",terminal:false}};
  if (!applyAdminBackgroundJobSnapshot("session_normalize",deferredState,"job-deferred",{
    job_id:"job-deferred",status:"deferred",terminal:true,result:{status:"partial_deferred",pending_count:2},
  })) throw new Error("deferred terminal snapshot stayed open");
  if (deferredState.loading !== false || deferredState.error !== null || deferredState.result.pending_count !== 2) {
    throw new Error("deferred terminal snapshot was not preserved: "+JSON.stringify(deferredState));
  }

  const partialState={loading:true,error:null,result:null,job:{job_id:"job-partial",status:"running",terminal:false}};
  if (!applyAdminBackgroundJobSnapshot("session_normalize",partialState,"job-partial",{
    job_id:"job-partial",status:"partial_error",terminal:true,
    result:{status:"partial_error",failed_count:1},progress:{error:"one turn failed"},
  })) throw new Error("partial_error terminal snapshot stayed open");
  if (partialState.loading !== false || partialState.result.failed_count !== 1 || partialState.error !== "one turn failed") {
    throw new Error("partial_error terminal snapshot was not preserved: "+JSON.stringify(partialState));
  }
})().catch(err=>{ console.error(err); process.exitCode=1; });
`
	cmd := exec.Command(nodePath, "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("admin job fixture failed: %v\n%s", err, out)
	}
}

func TestRepairReplayKeepsConflictEvidenceAndRescansOnlyFullyRepairedTurns(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for Repair Replay fixture")
		}
	}
	src := readArchiveCenterJS(t)
	normalizeTurns := extractArchiveCenterJSSyncFunction(t, src, "normalizeTurnIndexList")
	repair := extractArchiveCenterJSAsyncFunction(t, src, "explorerRepairChatLogs")
	script := `
const _chatLogRepairState={loading:false,error:null,result:null};
const removed=[];
const cleared=[];
const rescanned=[];
let requestCount=0;
function explorerSessionId(){ return "session-a"; }
function buildChatLogRepairReplayCandidateBundle(){
  return {entries:[{turn_index:2},{turn_index:3}],candidateTurnIndices:[2,3],journalCount:2,deletedSnapshotCount:0,sourceType:"journal"};
}
async function buildChatLogRepairReplayFallbackBundleFromActiveChat(){ throw new Error("unexpected fallback"); }
function setChatLogRepairProgress(){}
function refreshExplorerUI(){}
function t(key){ return key; }
function formatTurnIndexPreview(turns){ return turns.join(","); }
async function bridgeFetch(path,options){
  if (path !== "/turns/repair-replay") throw new Error("wrong route: "+path);
  requestCount++;
  if (options.body.dry_run) return {status:"ok",total_missing_role_count:2,total_conflict_role_count:1};
  return {status:"ok",repaired_turns:[2,3],conflict_turns:[2],failed_turns:[],total_repaired_role_count:2,total_conflict_role_count:1};
}
async function showConfirmModal(){ return true; }
async function removeFailedQueueItemsByTurn(sid,turns,kind){ removed.push({sid,turns:[...turns],kind}); }
function clearChatLogRestoreSnapshotEntries(sid,turns){ cleared.push({sid,turns:[...turns]}); }
async function explorerFetchChatLogs(){}
async function maybeRescanDerivedArtifactsForTurns(sid,turns){ rescanned.push({sid,turns:[...turns]}); return {ran:true,ok:true,result:{succeeded:1,failed:0}}; }
` + normalizeTurns + "\n" + repair + `
(async()=>{
  const ok=await explorerRepairChatLogs();
  if (!ok || requestCount !== 2) throw new Error("Repair Replay production path did not complete");
  const expected=JSON.stringify([3]);
  if (JSON.stringify(removed[0]&&removed[0].turns)!==expected) throw new Error("conflict turn was cleared from failed queue: "+JSON.stringify(removed));
  if (JSON.stringify(cleared[0]&&cleared[0].turns)!==expected) throw new Error("conflict turn lost local evidence: "+JSON.stringify(cleared));
  if (JSON.stringify(rescanned[0]&&rescanned[0].turns)!==expected) throw new Error("conflict turn reached derived rescan: "+JSON.stringify(rescanned));
})().catch(err=>{ console.error(err); process.exitCode=1; });
`
	cmd := exec.Command(nodePath, "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Repair Replay conflict fixture failed: %v\n%s", err, out)
	}
}

func TestExistingLLMRetryZeroReachesRuntimeConfigAndAdminCritic(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for retry ownership fixture")
		}
	}
	src := readArchiveCenterJS(t)
	syncConfig := extractJSFunctionBlockForTest(t, src, "async function syncConfigToBackend(s)")
	ensureBinding := extractJSFunctionBlockForTest(t, src, "async function ensureBackendRuntimeConfigBinding(backendInstanceId)")
	markDirty := extractJSFunctionBlockForTest(t, src, "function markBackendRuntimeConfigDirty(reason)")
	adminMeta := extractArchiveCenterJSSyncFunction(t, src, "buildAdminRuntimeClientMeta")
	script := `
const DEFAULT_SETTINGS={llmRetryCount:3,embeddingProvider:"openai",episodeIntervalTurns:8};
const settings={llmRetryCount:0,pluginMainProvider:"openai",subLlmProvider:"openai",pluginMainTimeoutMs:185000,subLlmTimeoutMs:245000};
const _backendRuntimeConfigBinding={instanceId:"",dirty:true,configReady:false,code:"runtime_config_not_bound",missingRoles:[]};
let syncedBody=null;
let bridgeCalls=0;
let bridgeResponse={status:"ok",backend_instance_id:"backend-a",runtime_config_trace:{synced:true}};
function sanitizeNumber(value,fallback,min,max){ const n=Number(value); return Number.isFinite(n)?Math.max(min,Math.min(max,n)):fallback; }
function resolveEffectiveCriticConfig(){ return {apiKey:"",endpoint:"",model:""}; }
function getPluginMainProviderSetting(v){ return v||"openai"; }
function getSubLlmProviderSetting(v){ return v||"openai"; }
function providerRequestOverrideSettingsForProvider(){ return {vertexFlexMode:"off",llmGatewayServiceTier:"standard",claudePromptCacheMode:"off",extraHeadersJson:"",extraBodyJson:""}; }
function getPluginMainTemperatureSetting(){ return 0; }
function getPluginMainMaxCompletionTokensSetting(){ return 1; }
function getPluginMainReasoningPresetSetting(){ return "auto"; }
function getPluginMainReasoningEffortSetting(){ return "none"; }
function getPluginMainReasoningBudgetTokensSetting(){ return 0; }
function getSubLlmTemperatureSetting(){ return 0; }
function getSubLlmMaxCompletionTokensSetting(){ return 1; }
function getSubLlmReasoningPresetSetting(){ return "auto"; }
function getSubLlmReasoningEffortSetting(){ return "none"; }
function getSubLlmReasoningBudgetTokensSetting(){ return 0; }
function normalizeEmbeddingProvider(v){ return v||"openai"; }
function normalizeSourceSearchLlmProvider(){ return "openai"; }
function normalizeReasoningPreset(){ return "auto"; }
function normalizeReasoningEffort(){ return "none"; }
function normalizeReasoningBudgetTokens(){ return 0; }
function getPluginMainTimeoutSettingMs(value){ return value == null ? 60000 : Number(value); }
function getSubLlmTimeoutSettingMs(value){ return value == null ? 90000 : Number(value); }
function failedQueueMaxAttempts(){ return 3; }
function getRequestTimeoutSettingMs(){ return 1000; }
function getCriticTimeoutMs(value){ return getSubLlmTimeoutSettingMs(value == null ? settings.subLlmTimeoutMs : value); }
function getEmbeddingTimeoutMs(){ return 1000; }
async function bridgeFetch(path,options){ bridgeCalls++; syncedBody=options.body; return bridgeResponse; }
async function safeCall(fn){ return await fn(); }
` + syncConfig + "\n" + ensureBinding + "\n" + markDirty + "\n" + adminMeta + `
(async()=>{
  const result=await syncConfigToBackend({llmRetryCount:0,pluginMainProvider:"openai",subLlmProvider:"openai",pluginMainTimeoutMs:185000,subLlmTimeoutMs:245000});
  if(!result.ok || !syncedBody || syncedBody.llmRetryCount !== 0) throw new Error("runtime config lost retry=0");
  if(syncedBody.mainTimeout !== 185 || syncedBody.supervisorTimeout !== 185 || syncedBody.criticTimeout !== 245) {
    throw new Error("UI timeout values did not reach backend roles: "+JSON.stringify(syncedBody));
  }
  bridgeResponse={status:"ok",backend_instance_id:"backend-a",runtime_config_trace:{synced:true,main:{configured:true,missing_fields:[]},supervisor:{configured:false,missing_fields:["timeout_ms"]}}};
  const incomplete=await syncConfigToBackend({llmRetryCount:0,pluginMainProvider:"openai",pluginMainApiKey:"key",pluginMainEndpoint:"https://example.test/v1",pluginMainModel:"model",subLlmProvider:"openai"});
  if(incomplete.ok || !incomplete.code.includes("supervisor[timeout_ms]")) throw new Error("runtime role incompleteness was accepted: "+incomplete.code);
  const callsAfterIncomplete=bridgeCalls;
  const unchangedIncomplete=await ensureBackendRuntimeConfigBinding("backend-a");
  if(!unchangedIncomplete.skipped || unchangedIncomplete.ok || bridgeCalls!==callsAfterIncomplete) throw new Error("unchanged incomplete config was retransmitted");
  bridgeResponse={status:"ok",backend_instance_id:"backend-a",runtime_config_trace:{synced:true,main:{configured:true,missing_fields:[]},supervisor:{configured:true,missing_fields:[]}}};
  const complete=await syncConfigToBackend({llmRetryCount:0,pluginMainProvider:"openai",pluginMainApiKey:"key",pluginMainEndpoint:"https://example.test/v1",pluginMainModel:"model",subLlmProvider:"openai"});
  if(!complete.ok) throw new Error("complete runtime role trace was rejected: "+complete.code);
  const callsAfterComplete=bridgeCalls;
  const unchanged=await ensureBackendRuntimeConfigBinding("backend-a");
  if(!unchanged.ok || !unchanged.skipped || bridgeCalls!==callsAfterComplete) throw new Error("same backend instance retransmitted runtime config");
  bridgeResponse={status:"ok",backend_instance_id:"backend-b",runtime_config_trace:{synced:true}};
  const restarted=await ensureBackendRuntimeConfigBinding("backend-b");
  if(!restarted.ok || restarted.skipped || bridgeCalls!==callsAfterComplete+1) throw new Error("backend restart did not trigger one config bind");
  markBackendRuntimeConfigDirty("settings_changed");
  const saved=await ensureBackendRuntimeConfigBinding("backend-b");
  if(!saved.ok || saved.skipped || bridgeCalls!==callsAfterComplete+2) throw new Error("settings save did not trigger one config bind");
  const meta=buildAdminRuntimeClientMeta();
  if(meta.critic.retry_count !== 0) throw new Error("admin critic meta lost retry=0");
  if(meta.critic.timeout_ms !== 245000) throw new Error("admin critic timeout diverged from UI value: "+meta.critic.timeout_ms);
})().catch(err=>{ console.error(err); process.exitCode=1; });
`
	cmd := exec.Command(nodePath, "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("retry ownership fixture failed: %v\n%s", err, out)
	}
}

func TestRecomposerLifecycleEnvelopeSurvivesLongGenerationWithoutTTL(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for Recomposer lifecycle fixture")
		}
	}
	data, err := os.ReadFile(filepath.Join(archiveCenterRoot(t), "AC Recomposer Agent.js"))
	if err != nil {
		t.Fatalf("read AC Recomposer Agent.js: %v", err)
	}
	src := string(data)
	safeString := extractJSFunctionBlockForTest(t, src, "function safeString(value, fallback)")
	stableDigest := extractJSFunctionBlockForTest(t, src, "function stableDigest(value)")
	asObject := extractJSFunctionBlockForTest(t, src, "function asObject(value)")
	arrayFromCollection := extractJSFunctionBlockForTest(t, src, "function arrayFromCollection(value)")
	deepClone := extractJSFunctionBlockForTest(t, src, "function deepClone(value)")
	readEnhancement := extractJSFunctionBlockForTest(t, src, "function readArchiveCenterEnhancement(latestUserInput)")
	script := `
const ARCHIVE_CENTER_BRIDGE_KEY="__RISU_ARCHIVE_CENTER_RECOMPOSER_V1__";
const ARCHIVE_CENTER_BRIDGE_CONTRACT="archive_center.recomposer_bridge.v1";
const ARCHIVE_CENTER_ENHANCEMENT_CONTRACT="archive_center.recomposer_enhancement.v1";
` + safeString + "\n" + stableDigest + "\n" + asObject + "\n" + arrayFromCollection + "\n" + deepClone + "\n" + readEnhancement + `
const input="long generation input";
const lifecycleState="current_request_payload_applied";
const sessionId="session-long";
const turnIndex=7;
const payloadPlanId="plan-long";
const inputDigest=stableDigest(input);
const lifecycleDigest=stableDigest([sessionId,turnIndex,inputDigest,input.length,payloadPlanId,lifecycleState].join("|"));
globalThis[ARCHIVE_CENTER_BRIDGE_KEY]={
  contract_version:ARCHIVE_CENTER_BRIDGE_CONTRACT,
  owner:"archive_center_host_adapter",
  transport_only:true,
  lifecycle_state:lifecycleState,
  published_at_ms:Date.now()-(60*60*1000),
  input_digest:inputDigest,
  input_chars:input.length,
  lifecycle_digest:lifecycleDigest,
  input_bindings:[{digest:inputDigest,chars:input.length,lifecycle_digest:lifecycleDigest}],
  session_id:sessionId,
  turn_index:turnIndex,
  payload_plan_id:payloadPlanId,
  enhancement_contract:{
    contract_version:ARCHIVE_CENTER_ENHANCEMENT_CONTRACT,
    owner:"go",read_only:true,optional_enhancement:true,standalone_fallback_required:true,
    session_id:sessionId,turn_index:turnIndex,
  },
  payload_application_observation:{
    payload_application_status:"applied",lifecycle_state:lifecycleState,payload_plan_id:payloadPlanId,
  },
};
if(!readArchiveCenterEnhancement(input)) throw new Error("long generation envelope was rejected by elapsed time");
globalThis[ARCHIVE_CENTER_BRIDGE_KEY].enhancement_contract.turn_index=8;
if(readArchiveCenterEnhancement(input)!==null) throw new Error("cross-turn lifecycle envelope was accepted");
`
	cmd := exec.Command(nodePath, "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Recomposer lifecycle fixture failed: %v\n%s", err, out)
	}
}

func TestRawCommittedReconciliationRetryUsesFreshIdempotencyAndStaysQueued(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for reconciliation retry fixture")
		}
	}
	src := readArchiveCenterJS(t)
	classify := extractJSFunctionBlockForTest(t, src, "function completeTurnNeedsFreshReconciliationRetry(result)")
	applyRetryKey := extractJSFunctionBlockForTest(t, src, "function applyBackendCompleteTurnReconciliationRetryKey(payload, result)")
	buildQueue := extractJSFunctionBlockForTest(t, src, "function buildCompleteTurnQueuePayload(body)")
	backfill := extractArchiveCenterJSAsyncFunction(t, src, "backfillOneActiveChatCompletedTurn")
	script := `
let queuedType="";
let queuedPayload=null;
function computeOrchestrationDirtyHashOr1c(value){ return "h"+String(value||"").length; }
function normalizeLanguageContextTrace(value){ return value; }
async function requestBackendSessionRoutingTurnResolution(){ return {status:"resolved",turnIndex:5,localTurnIndex:5,baseline:null}; }
async function fetchCanonicalChatLogsForTurn(){ return []; }
function chatLogItemsContainRoleContent(){ return false; }
function chatLogItemsContainRole(){ return false; }
async function buildCompleteTurnRequestBody(){
  return {
    chat_session_id:"session-raw",
    turn_index:5,
    user_input:"user",
    assistant_content:"assistant",
    context_messages:[],
    client_meta:{request_id:"old-key",idempotency_key:"old-key"},
  };
}
async function tryCompleteTurn(){
  return {
    status:"partial",
    code:"derived_reconciliation_required",
    save_ok:true,
    raw_committed:true,
    reconciliation_required:true,
    derived_retry_required:false,
    queue_action:"retry",
    retryable:true,
    reconciliation_retry_idempotency_key:"reconcile:backend-owned-key",
  };
}
function enqueue(type,payload){ queuedType=type; queuedPayload=payload; return {queued:true,admitted:true,code:"queued"}; }
async function persistFailedQueueAdmission(type,payload,admission){ return admission; }
function setTurnCounterAtLeast(){}
async function markActiveChatBackfillSaved(){}
async function verifyAndRepairCompleteTurnChatLogs(){}
` + classify + "\n" + applyRetryKey + "\n" + buildQueue + "\n" + backfill + `
(async()=>{
  const result=await backfillOneActiveChatCompletedTurn("session-raw",{
    userContent:"user",assistantContent:"assistant",contextMessages:[],hash:"pair",
  });
  if(result.status!=="queued" || !result.rawCommitted || queuedType!=="complete_turn") {
    throw new Error("raw partial response was removed instead of retained");
  }
  const key=queuedPayload && queuedPayload.client_meta && queuedPayload.client_meta.idempotency_key;
  if(key!=="reconcile:backend-owned-key") {
    throw new Error("reconciliation retry did not copy backend-owned key: "+key);
  }
  if(queuedPayload.client_meta.reconciliation_retry_pending!==true) {
    throw new Error("typed retry metadata missing");
  }
})().catch(err=>{ console.error(err); process.exitCode=1; });
`
	cmd := exec.Command(nodePath, "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("reconciliation retry fixture failed: %v\n%s", err, out)
	}
}

func TestCapturedSessionHostContextDoesNotFollowVisibleChat(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for immutable session host-context fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functions := strings.Join([]string{
		extractArchiveCenterJSFunction(t, src, "parseSessionDisplayIdentity"),
		extractArchiveCenterJSFunction(t, src, "resolveIdentityVerifiedCurrentCharacterChat"),
		extractArchiveCenterJSFunction(t, src, "captureSessionHostContextFromCache"),
		extractArchiveCenterJSFunction(t, src, "activeChatMatchesCapturedSession"),
		extractArchiveCenterJSAsyncFunction(t, src, "resolveCurrentActiveChatObject"),
	}, "\n")
	script := functions + `
let currentCoordinateReads = 0;
let indexedChat = {id:"chat-a",message:[{role:"char",data:"A output"}]};
let indexedCoordinates = [];
let _sessionCache = {
  sessionId:"char_1_cid_chat-a",charIdx:1,chatIdx:2,
  observedChatUniqueId:"chat-a",stableCharacterId:"character-a",
};
const R = {
  async getCurrentCharacterIndex(){ currentCoordinateReads++; return 9; },
  async getCurrentChatIndex(){ currentCoordinateReads++; return 9; },
  async getChatFromIndex(charIdx,chatIdx){ indexedCoordinates.push([charIdx,chatIdx]); return indexedChat; },
  async getCharacter(){ throw new Error("current character fallback must not run for captured A"); },
};
function debugLog() {}
(async()=>{
  const resolved = await resolveCurrentActiveChatObject("char_1_cid_chat-a");
  if(!resolved.chat || resolved.chat.id!=="chat-a" || resolved.source!=="R.getChatFromIndex.captured") {
    throw new Error("captured A chat was not resolved: "+JSON.stringify(resolved));
  }
  if(currentCoordinateReads!==0 || JSON.stringify(indexedCoordinates)!==JSON.stringify([[1,2]])) {
    throw new Error("resolver followed visible B coordinates: "+JSON.stringify({currentCoordinateReads,indexedCoordinates}));
  }
  indexedChat = {id:"chat-b",message:[{role:"char",data:"B output"}]};
  const mismatch = await resolveCurrentActiveChatObject("char_1_cid_chat-a", {
    sessionId:"char_1_cid_chat-a",charIdx:1,chatIdx:2,hostChatId:"chat-a",
  });
  if(mismatch.chat!==null || mismatch.reason!=="captured_chat_identity_mismatch") {
    throw new Error("B chat was accepted under A owner: "+JSON.stringify(mismatch));
  }
})().catch(err=>{ console.error(err && err.stack || err); process.exitCode=1; });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("immutable session host-context fixture failed: %v\n%s", err, out)
	}
}

func TestRisuOutputPersistsExactCommittedOutputDespiteUnresolvedWorldline(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for output-listener finality fixture")
		}
	}
	src := readArchiveCenterJS(t)
	onOutput := extractJSFunctionBlockForTest(t, src, "function onRisuOutput(snapshot)")
	script := onOutput + `
const _risuHookLifecycle = {output:"registered"};
const _finalConfirmationRequestBySession = new Map([
  ["session-a",{sessionId:"session-a",state:"captured",requestType:"model",characterIndex:1,chatIndex:2,hostChatId:"chat-a"}],
  ["session-b",{sessionId:"session-b",state:"captured",requestType:"model",characterIndex:8,chatIndex:9,hostChatId:"chat-b"}],
]);
const delegated = [];
let observationCalls = 0;
let directPersistenceCalls = 0;
let routeCalls = 0;
function observePendingFinalConfirmationAtHostSignal(sessionId,source,snapshot){
  observationCalls++;
  if(sessionId!=="session-a" || source!=="output" || !snapshot || snapshot.messageIndex!==1) {
    throw new Error("output observation escaped exact A coordinates");
  }
  return Promise.resolve({
    accepted:true,
    readyForPersistence:false,
    assistantContent:"A output",
    observation:{
      accepted:true,
      contract_version:"source_acceptance_observation.v1",
      session_id:"session-a",
      finality_source:"risu_output",
      host_signal_source:"output",
    },
  });
}
function onAfterRequest(content,type,observation){ delegated.push({content,type,observation}); return content; }
function backfillOneActiveChatCompletedTurn(){ directPersistenceCalls++; throw new Error("output used an alternate persistence path"); }
function buildRisuWorldlineObservationFromMessages(){ return {contract_version:"risu_worldline_observation.v2"}; }
function requestBackendSessionRoutingTurnResolution(sessionId,mode){
  routeCalls++;
  if(sessionId!=="session-a" || mode!=="identity") throw new Error("worldline diagnostic escaped exact A coordinates");
  return Promise.resolve({status:"worldline_ownership_unresolved",worldline:{state:"unresolved",reason:"parent_active_fork_source_unresolved"}});
}
function recordRisuHookLifecycle() {}
function debugLog() {}
function warnLog() {}
(async()=>{
  const result = onRisuOutput({
    characterIndex:1,chatIndex:2,messageIndex:1,
    chat:{id:"chat-a",message:[{role:"user",data:"A input"},{role:"char",data:"A output"}]},
  });
  if(result!==undefined) throw new Error("output listener became blocking");
  await new Promise(resolve=>setTimeout(resolve,0));
  if(observationCalls!==1 || delegated.length!==1 || directPersistenceCalls!==0 || routeCalls!==1) {
    throw new Error("unresolved worldline blocked or rerouted exact output persistence: "+JSON.stringify({observationCalls,delegated,directPersistenceCalls,routeCalls}));
  }
  if(delegated[0].content!=="A output" || delegated[0].type!=="model" ||
     !delegated[0].observation || delegated[0].observation.session_id!=="session-a") {
    throw new Error("output delegated the wrong content/session: "+JSON.stringify(delegated[0]));
  }
  if(delegated.some(row=>row.observation && row.observation.session_id==="session-b")) {
    throw new Error("A output delegated to B");
  }
})().catch(err=>{ console.error(err && err.stack || err); process.exitCode=1; });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("output-listener finality fixture failed: %v\n%s", err, out)
	}
}

func TestAfterRequestWithoutExactOwnerDefersWithoutCurrentSessionFallback(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for afterRequest ownership fixture")
		}
	}
	src := readArchiveCenterJS(t)
	onAfterRequest := extractJSFunctionBlockForTest(t, src, "function onAfterRequest(content, type)")
	script := onAfterRequest + `
const settings = {enabled:true};
const _finalConfirmationRequestBySession = new Map([
  ["session-a",{sessionId:"session-a",state:"captured",requestType:"model"}],
  ["session-b",{sessionId:"session-b",state:"captured",requestType:"model"}],
]);
const updates = [];
function recordRisuHookLifecycle() {}
function debugLog() {}
function isNarrativeType(type) { return type === "model"; }
function isSaveType(type) { return type === "model"; }
function normalizeAssistantPersistenceCandidate(value) { return String(value || "").trim(); }
function sanitizeNarrativeOutputForDisplay(value) { return String(value || ""); }
function computeOrchestrationDirtyHashOr1c(value) { return "h:" + String(value || ""); }
function updateRuntimeState(key,status,detail) { updates.push({key,status,detail}); }
(function(){
  const content = "committed output";
  const returned = onAfterRequest(content,"model");
  if (returned !== content) throw new Error("deferred owner path changed displayed output");
  if (updates.length !== 1 || updates[0].key !== "lastStreamingAfterRequest" || updates[0].status !== "deferred") {
    throw new Error("missing non-spinning deferred state: "+JSON.stringify(updates));
  }
  if (!updates[0].detail || updates[0].detail.reason_code !== "after_request_session_owner_ambiguous") {
    throw new Error("ambiguous exact-owner reason missing: "+JSON.stringify(updates[0]));
  }
})();
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("afterRequest exact-owner fixture failed: %v\n%s", err, out)
	}
}

func TestLorebookSessionSwitchDefersWithoutPostingOrDroppingRetry(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for lorebook session switch fixture")
		}
	}
	src := readArchiveCenterJS(t)
	syncLorebook := extractArchiveCenterJSAsyncFunction(t, src, "syncCurrentLorebookReference")
	script := syncLorebook + `
const SESSION_FALLBACK="default";
const hostContext={sessionId:"session-a",charIdx:1,chatIdx:2,hostChatId:"chat-a"};
const _lorebookReferenceSync={attemptedScopeKey:"",syncedScopeKey:"",inFlight:null,lastScope:null};
let activityChecks=0;
let lorebookReads=0;
let snapshotPosts=0;
const R={async getCurrentLorebookEntries(){ lorebookReads++; return [{id:"a-lore",content:"A lore"}]; }};
async function getCurrentChatSessionId(){ throw new Error("current B must not choose the owner"); }
function captureSessionHostContextFromCache(){ return hostContext; }
async function observeLorebookReferenceScope(sessionId,observed){
  if(sessionId!=="session-a" || observed!==hostContext) throw new Error("lost captured A scope");
  return {chat_session_id:"session-a",character_index:1,chat_index:2,enabled_module_ids:[],enabled_modules_observed:true};
}
function lorebookReferenceScopeKey(){ return "scope-a"; }
async function capturedSessionIsCurrentlyActive(){ activityChecks++; return activityChecks===1; }
async function postLorebookReferenceSnapshot(){ snapshotPosts++; return {status:"ok"}; }
function updateRuntimeState() {}
function lorebookReferenceSnapshotFailureState(){ return {}; }
function lorebookReferenceSnapshotPath(){ return "/unused"; }
(async()=>{
  const result=await syncCurrentLorebookReference({sessionId:"session-a",hostContext,force:true});
  if(!result || result.status!=="deferred" || result.reason!=="session_changed_during_lorebook_read") {
    throw new Error("session switch was not deferred: "+JSON.stringify(result));
  }
  if(lorebookReads!==1 || snapshotPosts!==0) {
    throw new Error("B lorebook was posted under A: "+JSON.stringify({lorebookReads,snapshotPosts}));
  }
  if(_lorebookReferenceSync.attemptedScopeKey!=="") {
    throw new Error("deferred A scope was permanently suppressed instead of remaining retryable");
  }
})().catch(err=>{ console.error(err && err.stack || err); process.exitCode=1; });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("lorebook session switch fixture failed: %v\n%s", err, out)
	}
}

func TestLongRunningHostOperationsCarryCapturedSessionContext(t *testing.T) {
	src := readArchiveCenterJS(t)
	checks := map[string][]string{
		"beforeRequest": {
			"orchHostContext = captureSessionHostContextFromCache(orchSessionId)",
			"hostContext: orchHostContext",
			"captureFinalConfirmationRequestContext(orchSessionId, type, orchRequestId, orchHostContext)",
			"resolveActiveChatCompletedTurnsForRoutingBaseline(orchSessionId, orchHostContext)",
			"sessionId: orchSessionId",
			"chatSessionId: orchSessionId",
		},
		"committedPersistence": {
			"const persistenceHostContext = selectedRequestContext",
			"findLatestActiveChatUnsavedCompletedTurnPair(chatSessionId, persistenceHostContext)",
			"findActiveChatCompletedTurnPairForUserContent(chatSessionId, safeSavedUserInput, persistenceHostContext)",
			"hostContext: persistenceHostContext",
		},
		"coldStart": {
			"async function computeActiveChatRescanDryRunPlan(sessionId, hostContext = null)",
			"resolveCurrentActiveChatObject(sid, fixedHostContext)",
			"? captureSessionHostContextFromCache(sid)",
			"computeActiveChatRescanDryRunPlan(sid, normalizeHostContext)",
		},
		"migration": {
			"finalizeTimelineSessionMigrationRoute(migrationID, sourceSid, targetSid, migrationRouteContext)",
			"persistAcknowledgedCurrentSessionRoute(targetSid, \"migration_commit\", observedContext)",
			"establishSessionRoutingTurnBaseline(targetSid, \"timeline_copy\", targetHostContext)",
			"establishSessionRoutingTurnBaseline(routedTargetSid, \"timeline_migrate\", {",
		},
	}
	for area, markers := range checks {
		for _, marker := range markers {
			if !strings.Contains(src, marker) {
				t.Fatalf("%s lost captured-session wiring %q", area, marker)
			}
		}
	}
}

// This regression loads the production plugin as one program and invokes the
// callbacks that it actually registers with the RisuAI API.  Only the host,
// DOM/storage, and HTTP boundaries are substituted.  The substitutes record
// every call and reject routes or coordinates outside the fixture so copied
// test-only lifecycle logic cannot make the assertion pass.
func TestFullArchiveCenterRuntimeKeepsCommittedOutputWithCapturedSessionAfterChatSwitch(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for the full Archive Center runtime lifecycle regression")
		}
	}
	archivePath := filepath.Join(archiveCenterRoot(t), "Archive Center.js")
	script := `
const fs = require("fs");
const archivePath = process.argv[2];
const source = fs.readFileSync(archivePath, "utf8");
const callbacks = { input: null, beforeRequest: null, afterRequest: null, output: null, unload: null };
const registrations = [];
const backendCalls = [];
const rollbackDecisionCalls = [];
const rollbackDeleteCalls = [];
const unexpected = [];
const completed = [];
const completeTurnAttempts = [];
const persistedLogs = new Map();
const storageValues = new Map();
const chars = [{ chaId: "character-fixture", name: "Fixture Character" }];
const chats = [
  { id: "fixture-chat-a", name: "A", isStreaming: false, message: [] },
  { id: "fixture-chat-b", name: "B", isStreaming: false, message: [] },
  {
    id: "fixture-chat-child", name: "Child", isStreaming: false,
    message: [
      { role: "user", data: "Parent user", chatId: "parent-user", time: 900 },
      { role: "char", data: "Parent output", chatId: "parent-output", time: 901,
        generationInfo: { generationId: "parent-generation" } },
      { role: "char", data: "{{specialcomment::branchedfrom::fixture-chat-a::1}}", disabled: true,
        chatId: "branch-marker", time: 902 },
    ],
  },
];
let currentCharacterIndex = 0;
let currentChatIndex = 0;
let sequence = 0;
let holdNextOutputBranchRouting = false;
let releaseOutputBranchRouting = null;
let rejectNextCompleteTurn = false;
let holdNextRollbackPrepare = false;
let releaseRollbackPrepare = null;

function assert(condition, message) {
  if (!condition) throw new Error(message);
}
function clone(value) {
  return value == null ? value : JSON.parse(JSON.stringify(value));
}
function response(payload, status = 200) {
  const text = JSON.stringify(payload);
  return {
    status,
    ok: status >= 200 && status < 300,
    async json() { return clone(payload); },
    async text() { return text; },
  };
}
function sessionIDFor(chatIndex) {
  return "char_" + currentCharacterIndex + "_cid_" + chats[chatIndex].id;
}
function fixtureChatIndexForSession(sessionID) {
  return chats.findIndex((chat, index) => sessionIDFor(index) === sessionID);
}
function parseBody(init) {
  if (!init || init.body == null || init.body === "") return null;
  if (typeof init.body !== "string") return clone(init.body);
  return JSON.parse(init.body);
}
function logsKey(sessionID, turnIndex) {
  return String(sessionID) + "|" + String(turnIndex);
}
async function waitFor(predicate, label) {
  const started = Date.now();
  while (!predicate()) {
    if (Date.now() - started > 4000) throw new Error("timed out waiting for " + label);
    await new Promise(resolve => setTimeout(resolve, 10));
  }
}

const settings = {
  enabled: true,
  dbEnabled: true,
  supervisorEnabled: false,
  injectionEnabled: false,
  lorebookReferenceMode: "off",
  rollbackAutoEnabled: false,
  turnWorkflowHUDEnabled: false,
  pluginMainApplyMode: "off",
  pluginMainRewriteOptIn: false,
  narrativeGuideMode: "off",
  requestTimeoutMs: 1000,
  llmRetryCount: 0,
};
storageValues.set("risu_memory_orchestrator_settings", JSON.stringify(settings));

global.localStorage = {
  getItem(key) { return storageValues.has(String(key)) ? storageValues.get(String(key)) : null; },
  setItem(key, value) { storageValues.set(String(key), String(value)); },
  removeItem(key) { storageValues.delete(String(key)); },
};
global.window = {
  location: { protocol: "http:", hostname: "localhost", host: "localhost" },
  addEventListener() {},
  removeEventListener() {},
};
global.location = global.window.location;
global.navigator = { userAgent: "archive-center-full-runtime-test" };
global.document = {
  body: { appendChild() {} },
  head: { appendChild() {} },
  documentElement: { appendChild() {} },
  querySelector() { return null; },
  querySelectorAll() { return []; },
  getElementById() { return null; },
  createElement() { return { style: {}, dataset: {}, appendChild() {}, remove() {}, addEventListener() {} }; },
};
global.fetch = async function unexpectedGlobalFetch(url) {
  unexpected.push("global fetch " + String(url));
  throw new Error("unexpected global fetch");
};
global.confirm = function() { throw new Error("unexpected confirm"); };
global.alert = function() { throw new Error("unexpected alert"); };
global.prompt = function() { throw new Error("unexpected prompt"); };

const persistentStorage = {
  async getItem(key) { return storageValues.has(String(key)) ? storageValues.get(String(key)) : null; },
  async setItem(key, value) { storageValues.set(String(key), String(value)); },
  async removeItem(key) { storageValues.delete(String(key)); },
};

const Risuai = {
  pluginStorage: persistentStorage,
  async getLocalPluginStorage() { return persistentStorage; },
  async addRisuScriptHandler(name, callback) {
    assert(name === "input", "unexpected script handler " + name);
    assert(callbacks.input === null, "input handler registered twice");
    callbacks.input = callback;
    registrations.push("input");
  },
  async addRisuReplacer(name, callback) {
    assert(name === "beforeRequest" || name === "afterRequest", "unexpected replacer " + name);
    assert(callbacks[name] === null, name + " registered twice");
    callbacks[name] = callback;
    registrations.push(name);
  },
  async addRisuChatListener(name, callback) {
    assert(name === "output", "unexpected chat listener " + name);
    assert(callbacks.output === null, "output listener registered twice");
    callbacks.output = callback;
    registrations.push("output");
  },
  async onUnload(callback) { callbacks.unload = callback; },
  async registerSetting() { registrations.push("setting"); },
  async registerButton() { registrations.push("button"); },
  async getCurrentCharacterIndex() { return currentCharacterIndex; },
  async getCurrentChatIndex() { return currentChatIndex; },
  async getCharacter() { return clone(chars[currentCharacterIndex]); },
  async getChatFromIndex(characterIndex, chatIndex) {
    if (characterIndex !== currentCharacterIndex && characterIndex !== 0) {
      unexpected.push("character coordinate " + characterIndex);
      throw new Error("unexpected character coordinate");
    }
    if (!Number.isInteger(chatIndex) || !chats[chatIndex]) {
      unexpected.push("chat coordinate " + chatIndex);
      throw new Error("unexpected chat coordinate");
    }
    return chats[chatIndex];
  },
  async nativeFetch(urlValue, init = {}) {
    const url = new URL(String(urlValue));
    const method = String(init.method || "GET").toUpperCase();
    const body = parseBody(init);
    backendCalls.push({ path: url.pathname, search: url.search, method, body });

    if (url.pathname === "/config/update" && method === "POST") {
      return response({ status: "ok", backend_instance_id: "fixture-backend", runtime_config_trace: {} });
    }
    if (url.pathname === "/session-routing/turn-resolution" && method === "POST") {
      assert(body && typeof body.chat_session_id === "string" && body.chat_session_id, "turn resolution missing session");
      const observedOrdinal = Math.max(0, Number(body.observed_pair_ordinal || 0));
      const knownTurns = completed.filter(item => item.chat_session_id === body.chat_session_id).length;
      const resolvedTurn = body.mode === "pair" ? Math.max(observedOrdinal, knownTurns + 1) : knownTurns;
      const routingPayload = {
        status: "ok",
        contract_version: "session-routing.turn-resolution.v1",
        resolution: "normal",
        chat_session_id: body.chat_session_id,
        identity_resolution: "fixture_exact_host_identity",
        binding_acknowledged: true,
        binding_required: false,
        turn_index: resolvedTurn,
        completed_turns: knownTurns,
        local_turn_index: observedOrdinal,
        resolved_observations: [],
      };
      if (body.worldline_observation) {
        routingPayload.worldline = {
          state: "confirmed",
          reason: "fixture_exact_branch_marker",
          parent_session_id: sessionIDFor(0),
          child_session_id: body.chat_session_id,
          fork_turn_index: 1,
        };
		if (
			body.chat_session_id === sessionIDFor(2)
			&& body.worldline_observation.host_signal_source === "active_chat_pre_backfill"
		) {
			routingPayload.resolution = "worldline_ownership_unresolved";
			routingPayload.worldline = {
				state: "unresolved",
				reason: "fixture_parent_active_fork_source_unresolved",
			};
		}
      }
      if (
        holdNextOutputBranchRouting
        && body.worldline_observation
        && body.worldline_observation.host_signal_source === "output"
      ) {
        holdNextOutputBranchRouting = false;
        return await new Promise(resolve => {
          releaseOutputBranchRouting = () => resolve(response(routingPayload));
        });
      }
      return response(routingPayload);
    }
	if (url.pathname === "/prepare-turn" && method === "POST") {
      assert(body && typeof body.chat_session_id === "string" && body.chat_session_id, "prepare-turn missing session");
	  if (holdNextRollbackPrepare) {
		holdNextRollbackPrepare = false;
		return await new Promise(resolve => {
		  releaseRollbackPrepare = () => resolve(response({
			status: "ok",
			source: "fixture",
			current_input_decision: {
			  status: "eligible",
			  reason_code: "fixture_eligible",
			  effective_user_input: "B route fence input",
			  selected_observation_ref: "fixture:b-route-fence",
			  context_injection_eligible: false,
			},
		  }));
		});
	  }
      return response({
        status: "ok",
        source: "fixture",
        current_input_decision: { status: "deferred", reason_code: "fixture_owner_only" },
		});
	}
	if (url.pathname === "/rollback/decision" && method === "POST") {
	  rollbackDecisionCalls.push(clone(body));
	  const routedChatIndex = chats.findIndex(chat => chat.id === body.host_chat_id);
	  assert(routedChatIndex >= 0, "rollback decision used unknown host chat " + body.host_chat_id);
	  assert(body.chat_session_id === sessionIDFor(routedChatIndex),
		"rollback request crossed captured route: sid=" + body.chat_session_id + " host=" + body.host_chat_id);
	  return response({
		status: "ok",
		contract_version: "rollback.decision.v2",
		allowed: false,
		decision: "blocked",
		reason: "assistant_output_not_removed",
		chat_session_id: body.chat_session_id,
		assistant_observation_digest: "fixture-assistant-digest",
		incomplete_assistant_observations: [],
	  });
	}
	if (url.pathname.startsWith("/rollback/") && method === "DELETE") {
	  rollbackDeleteCalls.push({ path: url.pathname, search: url.search });
	  throw new Error("blocked rollback decision reached mutation route");
	}
	if (url.pathname.endsWith("/lorebook-reference/snapshots") && method === "POST") {
		const snapshotSession = decodeURIComponent(url.pathname.split("/")[2] || "");
		assert(fixtureChatIndexForSession(snapshotSession) >= 0,
			"lorebook snapshot crossed fixture sessions: " + snapshotSession);
		assert(body && Array.isArray(body.entries), "lorebook snapshot entries missing");
		return response({ status: "ok", stored: true }, 201);
	}
    if (url.pathname.startsWith("/canonical/") && url.pathname.endsWith("/chat-logs") && method === "GET") {
      const parts = url.pathname.split("/");
      const sessionID = decodeURIComponent(parts[2]);
      const fromTurn = Number(url.searchParams.get("from_turn"));
      return response({ status: "ok", items: clone(persistedLogs.get(logsKey(sessionID, fromTurn)) || []) });
    }
	if (url.pathname.startsWith("/canonical/") && url.pathname.endsWith("/chat-logs") && method === "POST") {
      const parts = url.pathname.split("/");
      const sessionID = decodeURIComponent(parts[2]);
      assert(body && body.chat_session_id === sessionID, "chat-log repair crossed sessions");
      const key = logsKey(sessionID, body.turn_index);
      const rows = persistedLogs.get(key) || [];
      rows.push({ role: body.role, content: body.content, turn_index: body.turn_index });
      persistedLogs.set(key, rows);
		return response({ status: "ok", saved: true });
	}
	if (url.pathname === "/timeline" && method === "GET") {
		const timelineSession = String(url.searchParams.get("sessionId") || "");
		assert(fixtureChatIndexForSession(timelineSession) >= 0,
			"timeline refresh crossed fixture sessions: " + timelineSession);
		return response({ status: "ok", items: [], total: 0 });
	}
	if (url.pathname === "/sessions" && method === "GET") {
		return response({ status: "ok", sessions: [] });
	}
	if (url.pathname === "/step23/capture-verification" && method === "POST") {
		assert(body && fixtureChatIndexForSession(body.chat_session_id) >= 0,
			"capture verification crossed fixture sessions");
		return response({ status: "ok", record_id: backendCalls.length });
	}
	if (url.pathname === "/maintenance/enqueue" && method === "POST") {
		assert(body && fixtureChatIndexForSession(body.chat_session_id) >= 0,
			"maintenance enqueue crossed fixture sessions");
		return response({ status: "ok", queued: true });
	}
	if (url.pathname === "/complete-turn" && method === "POST") {
		assert(body && typeof body.chat_session_id === "string" && body.chat_session_id, "complete-turn missing session");
		assert(typeof body.user_input === "string" && body.user_input, "complete-turn missing user input");
		assert(typeof body.assistant_content === "string" && body.assistant_content, "complete-turn missing assistant output");
		const finality = body.client_meta && body.client_meta.source_acceptance_observation;
		if (!finality || finality.contract_version !== "source_acceptance_observation.v1"
			|| finality.finality_source !== "risu_output") {
			const alreadyPersisted = completed.some(item => item.chat_session_id === body.chat_session_id
				&& item.assistant_content === body.assistant_content);
			const inheritedBranchSource = body.chat_session_id === sessionIDFor(2)
				&& body.user_input === "Parent user"
				&& body.assistant_content === "Parent output";
			assert(alreadyPersisted || inheritedBranchSource,
				"non-output path tried to canonically persist a new fixture turn");
			return response({
				status: "rejected",
				code: inheritedBranchSource ? "fixture_inherited_parent_not_child_turn" : "fixture_source_already_persisted",
				queue_action: "discard",
				retryable: false,
				turn_index: body.turn_index,
				fail_reasons: [],
			});
		}
		const expectedSession = body.user_input.includes("A input") ? sessionIDFor(0)
			: body.user_input.includes("B input") ? sessionIDFor(1)
			: body.user_input.includes("Child input") ? sessionIDFor(2) : "";
		assert(expectedSession, "unexpected complete-turn user content " + body.user_input);
		assert(body.chat_session_id === expectedSession,
			"cross-session complete-turn: expected " + expectedSession + " got " + body.chat_session_id);
		const expectedChatIndex = fixtureChatIndexForSession(expectedSession);
		assert(finality && finality.contract_version === "source_acceptance_observation.v1" && finality.finality_source === "risu_output",
			"complete-turn was not owned by the official output callback");
		assert(finality.host_signal_source === "output", "complete-turn did not retain the official output signal");
		assert(finality.host_chat_id === chats[expectedChatIndex].id,
			"complete-turn finality used the wrong host chat");
		assert(finality.message_index === chats[expectedChatIndex].message.length - 1,
			"complete-turn finality used the wrong committed message index");
		const exactCommittedMessage = chats[expectedChatIndex].message[finality.message_index];
		assert(exactCommittedMessage && exactCommittedMessage.generationInfo
			&& finality.generation_id === exactCommittedMessage.generationInfo.generationId,
			"complete-turn finality used the wrong generation");
		assert([
			"matched_committed_content",
			"different_from_committed_content",
			"not_observed_streaming_or_unexposed",
		].includes(finality.after_request_candidate_state),
			chats[expectedChatIndex].name + " carried an unknown afterRequest diagnostic state");
		completeTurnAttempts.push(clone(body));
		if (rejectNextCompleteTurn) {
			rejectNextCompleteTurn = false;
			return response({
				status: "rejected",
				code: "fixture_new_observation_required",
				queue_action: "retry_after_new_observation",
				retryable: true,
				turn_index: body.turn_index,
				fail_reasons: ["fixture_new_observation_required"],
			});
		}
		completed.push(clone(body));
		persistedLogs.set(logsKey(body.chat_session_id, body.turn_index), [
			{ role: "user", content: body.user_input, turn_index: body.turn_index },
        { role: "assistant", content: body.assistant_content, turn_index: body.turn_index },
      ]);
      return response({
        status: "ok",
        save_ok: true,
        critic_triggered: true,
        turn_index: body.turn_index,
        chat_logs_saved: 2,
        fail_reasons: [],
      });
    }

    unexpected.push(method + " " + url.pathname + url.search);
    throw new Error("unexpected backend route " + method + " " + url.pathname + url.search);
  },
};
global.Risuai = Risuai;
global.risuai = Risuai;

(async function run() {
  await eval(source);
  assert(typeof callbacks.input === "function", "production input callback was not registered");
  assert(typeof callbacks.beforeRequest === "function", "production beforeRequest callback was not registered");
  assert(typeof callbacks.afterRequest === "function", "production afterRequest callback was not registered");
  assert(typeof callbacks.output === "function", "production output callback was not registered");

  const userA = { role: "user", data: "A input " + (++sequence), chatId: "user-a", time: 1000 + sequence };
  chats[0].message.push(userA);
  currentChatIndex = 0;
  await callbacks.input(userA.data);
  await callbacks.beforeRequest({ messages: [{ role: "user", content: userA.data }] }, "model");

	// A and B are both outstanding before A's coordinate-free afterRequest fires.
	// That native callback is intentionally ambiguous; the exact official output
	// callback must still bind and persist A without consuming B's context.
	const userB = { role: "user", data: "B input " + (++sequence), chatId: "user-b", time: 1000 + sequence };
	chats[1].message.push(userB);
	currentChatIndex = 1;
	await callbacks.input(userB.data);
	await callbacks.beforeRequest({ messages: [{ role: "user", content: userB.data }] }, "model");

	const outputA = "A output " + (++sequence);
	const returnedA = callbacks.afterRequest(outputA, "model");
	assert(returnedA === outputA, "ambiguous A afterRequest changed the fixture output unexpectedly");
	await new Promise(resolve => setTimeout(resolve, 20));
	assert(completed.length === 0, "ambiguous A afterRequest persisted before an exact output observation");

  const assistantA = {
    role: "char", data: outputA, chatId: "assistant-a", time: 1000 + sequence,
    generationInfo: { generationId: "generation-a" },
  };
  chats[0].message.push(assistantA);
  callbacks.output({
    char: chars[0], chat: chats[0], characterIndex: 0, chatIndex: 0,
    messageIndex: chats[0].message.length - 1,
  });
	await waitFor(() => completed.length === 1, "A complete-turn from the production output listener");
	assert(completed[0].chat_session_id === sessionIDFor(0), "A final was not persisted to A");
	assert(completed[0].assistant_content === outputA, "A final content was not the committed output");
	assert(completed[0].client_meta.source_acceptance_observation.after_request_candidate_state
		=== "not_observed_streaming_or_unexposed",
		"ambiguous A afterRequest was not retained as a candidate-absence diagnostic");
	assert(completed.filter(item => item.chat_session_id === sessionIDFor(1)).length === 0,
		"A output consumed or persisted B's pending request");
	callbacks.output({
		char: chars[0], chat: chats[0], characterIndex: 0, chatIndex: 0,
		messageIndex: chats[0].message.length - 1,
	});
	await new Promise(resolve => setTimeout(resolve, 20));
	assert(completed.length === 1, "duplicate A output scheduled another complete-turn");

	const outputB = "B output " + (++sequence);
	const mismatchedCandidateB = "B stale candidate " + (++sequence);
	const returnedB = callbacks.afterRequest(mismatchedCandidateB, "model");
	assert(returnedB === mismatchedCandidateB, "B afterRequest changed the fixture candidate unexpectedly");
	await new Promise(resolve => setTimeout(resolve, 20));
	assert(completed.length === 1, "B afterRequest persisted before its exact output observation");
	const assistantB = {
    role: "char", data: outputB, chatId: "assistant-b", time: 1000 + sequence,
    generationInfo: { generationId: "generation-b" },
  };
  chats[1].message.push(assistantB);
	callbacks.output({
    char: chars[0], chat: chats[1], characterIndex: 0, chatIndex: 1,
		messageIndex: chats[1].message.length - 1,
	});
	await waitFor(() => completed.length === 2, "B complete-turn from the production output listener");
	callbacks.output({
		char: chars[0], chat: chats[1], characterIndex: 0, chatIndex: 1,
		messageIndex: chats[1].message.length - 1,
	});
	await new Promise(resolve => setTimeout(resolve, 50));
	assert(completed.length === 2, "B output callback scheduled a duplicate complete-turn");
  assert(completed[1].chat_session_id === sessionIDFor(1), "B final was not persisted to B");
  assert(completed[1].assistant_content === outputB, "B final content mismatch");
	assert(completed[1].assistant_content !== mismatchedCandidateB,
		"B candidate content substituted the exact official output");
	assert(completed[1].client_meta.source_acceptance_observation.after_request_candidate_state
		=== "different_from_committed_content",
		"B candidate mismatch was not retained as a diagnostic");

	// The official output callback may arrive before afterRequest.  A callback
	// for the user anchor (or any stale/wrong slot) cannot confirm the turn, but
	// the exact committed slot is itself authoritative and runs the same
	// canonical persistence pipeline once.
	const userA2 = { role: "user", data: "A input " + (++sequence), chatId: "user-a-2", time: 1000 + sequence };
	chats[0].message.push(userA2);
	currentChatIndex = 0;
	await callbacks.input(userA2.data);
	await callbacks.beforeRequest({ messages: [{ role: "user", content: userA2.data }] }, "model");
	callbacks.output({
		char: chars[0], chat: chats[0], characterIndex: 0, chatIndex: 0,
		messageIndex: chats[0].message.length - 1,
	});
	await new Promise(resolve => setTimeout(resolve, 20));
	assert(completed.length === 2, "wrong output messageIndex persisted the A2 request");

	const outputA2 = "A output " + (++sequence);
	chats[0].message.push({
		role: "char", data: outputA2, chatId: "assistant-a-2", time: 1000 + sequence,
		generationInfo: { generationId: "generation-a-2" },
	});
	callbacks.output({
		char: chars[0], chat: chats[0], characterIndex: 0, chatIndex: 0,
		messageIndex: chats[0].message.length - 1,
	});
	await waitFor(() => completed.length === 3, "A2 output-first complete-turn");
	assert(completed[2].chat_session_id === sessionIDFor(0), "A2 output-first final escaped A");
	assert(completed[2].client_meta.source_acceptance_observation.after_request_candidate_state
		=== "not_observed_streaming_or_unexposed",
		"output-first A2 did not retain candidate absence as a diagnostic");
	const returnedA2 = callbacks.afterRequest(outputA2, "model");
	assert(returnedA2 === outputA2, "A2 afterRequest changed the fixture output unexpectedly");
	await new Promise(resolve => setTimeout(resolve, 20));
	assert(completed.length === 3, "late A2 afterRequest scheduled another complete-turn");
	callbacks.output({
		char: chars[0], chat: chats[0], characterIndex: 0, chatIndex: 0,
		messageIndex: chats[0].message.length - 1,
	});
	await new Promise(resolve => setTimeout(resolve, 20));
	assert(completed.length === 3, "duplicate A2 output scheduled another complete-turn");

	// An unresolved branch preflight may skip historical backfill, but it cannot
	// suppress the current prepare-turn or exact committed-output persistence.
	// Output-side worldline routing remains diagnostic and may finish later.
	const userChild = {
		role: "user", data: "Child input " + (++sequence), chatId: "user-child", time: 1000 + sequence,
	};
	chats[2].message.push(userChild);
	currentChatIndex = 2;
	await callbacks.input(userChild.data);
	await callbacks.beforeRequest({ messages: [{ role: "user", content: userChild.data }] }, "model");
	const outputChild = "Child output " + (++sequence);
	const returnedChild = callbacks.afterRequest(outputChild, "model");
	assert(returnedChild === outputChild, "child afterRequest changed the fixture output unexpectedly");
	chats[2].message.push({
		role: "char", data: outputChild, chatId: "assistant-child", time: 1000 + sequence,
		generationInfo: { generationId: "generation-child" },
	});
	holdNextOutputBranchRouting = true;
	callbacks.output({
		char: chars[0], chat: chats[2], characterIndex: 0, chatIndex: 2,
		messageIndex: chats[2].message.length - 1,
	});
	await waitFor(
		() => typeof releaseOutputBranchRouting === "function" && completed.length === 4,
		"child complete-turn independent of held worldline diagnostic",
	);
	assert(completed[3].chat_session_id === sessionIDFor(2),
		"unresolved branch final was not persisted to the frozen child session");
	releaseOutputBranchRouting();
	await new Promise(resolve => setTimeout(resolve, 20));
	assert(completed.length === 4, "late worldline diagnostic scheduled duplicate child persistence");

	// A backend source-acceptance rejection remains durably recorded for the
	// existing retry-after-new-observation path.  Replaying the same output is
	// not itself a new observation and must not submit a duplicate request.
	const userB2 = { role: "user", data: "B input " + (++sequence), chatId: "user-b-2", time: 1000 + sequence };
	chats[1].message.push(userB2);
	currentChatIndex = 1;
	await callbacks.input(userB2.data);
	await callbacks.beforeRequest({ messages: [{ role: "user", content: userB2.data }] }, "model");
	const outputB2 = "B output " + (++sequence);
	callbacks.afterRequest(outputB2, "model");
	chats[1].message.push({
		role: "char", data: outputB2, chatId: "assistant-b-2", time: 1000 + sequence,
		generationInfo: { generationId: "generation-b-2" },
	});
	rejectNextCompleteTurn = true;
	const attemptsBeforeRejectedOutput = completeTurnAttempts.length;
	callbacks.output({
		char: chars[0], chat: chats[1], characterIndex: 0, chatIndex: 1,
		messageIndex: chats[1].message.length - 1,
	});
	await waitFor(() => completeTurnAttempts.length === attemptsBeforeRejectedOutput + 1,
		"backend-rejected B2 complete-turn attempt");
	assert(completed.length === 4, "backend-rejected B2 was reported as durably saved");
	await waitFor(
		() => Array.from(storageValues.keys()).some(key => key.includes("pendingFinalConfirmation")),
		"durable pending-final recovery state",
	);
	const pendingRecoveryStorageKey = Array.from(storageValues.keys())
		.find(key => key.includes("pendingFinalConfirmation"));
	const pendingRecoveryDocument = JSON.parse(storageValues.get(pendingRecoveryStorageKey));
	assert(Array.isArray(pendingRecoveryDocument.items)
		&& pendingRecoveryDocument.items.some(item => item
			&& item.state === "pending"
			&& item.payload
			&& item.payload.chat_session_id === sessionIDFor(1)
			&& item.payload.assistant_content === outputB2
			&& item.reason === "fixture_new_observation_required"),
		"backend rejection did not retain the exact B2 retry payload");
	callbacks.output({
		char: chars[0], chat: chats[1], characterIndex: 0, chatIndex: 1,
		messageIndex: chats[1].message.length - 1,
	});
	await new Promise(resolve => setTimeout(resolve, 30));
	assert(completeTurnAttempts.length === attemptsBeforeRejectedOutput + 1,
		"duplicate rejected output bypassed retry-after-new-observation state");

	// Hold the source decision after beforeRequest has frozen B's host route,
	// then switch the visible UI to A. The production rollback adapter must send
	// B's captured stable character/chat identity and the full B assistant set;
	// it must not reread the now-current A chat.
	const userB3 = { role: "user", data: "B route fence input", chatId: "user-b-3", time: 1000 + (++sequence) };
	chats[1].message.push(userB3);
	currentChatIndex = 1;
	await callbacks.input(userB3.data);
	holdNextRollbackPrepare = true;
	const bRollbackBeforeRequest = callbacks.beforeRequest(
	  { messages: [{ role: "user", content: userB3.data }] },
	  "model",
	);
	await waitFor(() => typeof releaseRollbackPrepare === "function", "held B rollback source decision");
	currentChatIndex = 0;
	releaseRollbackPrepare();
	await bRollbackBeforeRequest;
	assert(rollbackDecisionCalls.length === 1,
	  "captured B beforeRequest did not issue exactly one rollback decision: " + JSON.stringify(rollbackDecisionCalls));
	const rollbackObservation = rollbackDecisionCalls[0];
	assert(rollbackObservation.chat_session_id === sessionIDFor(1), "rollback decision did not retain B session");
	assert(rollbackObservation.stable_character_id === chars[0].chaId,
	  "rollback decision did not retain captured stable character ID");
	assert(rollbackObservation.host_chat_id === chats[1].id,
	  "rollback decision reread current A host chat instead of captured B");
	assert(rollbackObservation.stable_character_id_state === "observed"
	  && rollbackObservation.host_chat_id_state === "observed",
	  "rollback decision did not mark captured route observations");
	assert(rollbackObservation.assistant_observation_scope === "full_active_chat",
	  "rollback decision lost full assistant observation scope");
	assert(Array.isArray(rollbackObservation.assistant_observations)
	  && rollbackObservation.assistant_observations.some(item => item.message_id === "assistant-b"),
	  "rollback decision did not carry B's complete assistant observation set");
	assert(!rollbackObservation.assistant_observations.some(item => item.message_id === "assistant-a"),
	  "rollback decision mixed A assistant observations into B");
	assert(rollbackDeleteCalls.length === 0, "blocked B route decision mutated A or B");

	assert(completed.every(item => fixtureChatIndexForSession(item.chat_session_id) >= 0),
		"complete-turn escaped the captured fixture sessions");
  assert(unexpected.length === 0, "unexpected boundary calls: " + JSON.stringify(unexpected));
})().catch(error => {
	console.error(error && error.stack || error);
	console.error("completed=" + JSON.stringify(completed));
	console.error("backendCalls=" + JSON.stringify(backendCalls));
	if (unexpected.length) console.error("unexpected=" + JSON.stringify(unexpected));
	process.exitCode = 1;
});
`
	cmd := exec.Command(nodePath, "-", archivePath)
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("full Archive Center A/B lifecycle regression failed: %v\n%s", err, out)
	}
}

func extractArchiveCenterJSSyncFunction(t *testing.T, src, name string) string {
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
	return strings.TrimSpace(src[start : start+len(marker)+next])
}
