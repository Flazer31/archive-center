package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func legacySourceShapeSessionRouteAdapterUsesOfficialStableHostIdentityAndReadback(t *testing.T) {
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
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for complete-turn request-context fixture")
		}
	}
	src := readArchiveCenterJS(t)
	bodyFunction := extractArchiveCenterJSAsyncFunction(t, src, "buildCompleteTurnRequestBody")
	script := bodyFunction + `
const AUTO_CONTINUE_USER_INPUT_MARKER="[auto-continue]";
const DEFAULT_SETTINGS={episodeIntervalTurns:20,chapterIntervalEpisodes:5,arcIntervalChapters:5,sagaIntervalArcs:3};
const settings={...DEFAULT_SETTINGS,maxInputContextChars:4321};
let _latestOrchResultForUI={_chatSessionId:"wrong-session"};
async function resolveRuntimeOutputLanguageOverride(){return "ko";}
async function buildLanguageContextTrace(){return {output_language_override:"ko"};}
async function buildCompleteTurnSourceAcceptanceObservation(sid,assistant,options){
  if(sid!=="session-a" || assistant!=="assistant-a") throw new Error("complete-turn input owner changed");
  return options.sourceAcceptanceFinality;
}
async function observeRisuPersona(sid,hostContext){
  if(sid!=="session-a" || hostContext.hostChatId!=="cid-a") throw new Error("persona observation lost captured host context");
  return {persona_id:"persona-a"};
}
function buildSourceToFinalLineageObservation(){return null;}
function buildRisuRequestObservation(){return {contract_version:"risu_request_observation.v1"};}
function computeOrchestrationDirtyHashOr1c(value){return "hash:"+String(value||"");}
function buildRisuActiveChatContextMessageObservation(value){return value;}
function normalizeLanguageContextTrace(value){return value;}
function composeEffectiveInputFromTransparency(){return "";}
function debugLog(...args){throw new Error("unexpected build failure: "+args.join(" "));}
(async()=>{
  const sourceAcceptanceFinality={
    accepted:true,
    finality_source:"risu_afterRequest",
    archive_center_request_correlation_id:"request-a",
    generation_id_state:"not_exposed_by_risu_afterRequest",
  };
  const body=await buildCompleteTurnRequestBody(75,"user-a","assistant-a",[],"session-a",null,{
    orchestrationResult:{_chatSessionId:"session-a",_trace:{}},
    sourceAcceptanceFinality,
    hostContext:{sessionId:"session-a",charIdx:1,chatIdx:2,hostChatId:"cid-a"},
  });
  if(!body || body.chat_session_id!=="session-a" || body.turn_index!==75) throw new Error("complete-turn body lost fixed session");
  if(body.client_meta.turn_workflow_request_id!=="request-a" || body.client_meta.archive_center_request_correlation_id!=="request-a") {
    throw new Error("workflow request id was not forwarded from the accepted beforeRequest context");
  }
  const budget=body.client_meta.critic_input_budget_observation;
  if(!budget || budget.contract_version!=="critic_input_budget_observation.v1" || budget.max_input_context_chars!==4321) {
    throw new Error("critic budget observation mismatch: "+JSON.stringify(budget));
  }
})().catch(err=>{console.error(err && err.stack || err);process.exit(1);});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("complete-turn request-context runtime fixture failed: %v\n%s", err, output)
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
	beforeRequest := extractArchiveCenterJSAsyncFunction(t, src, "onBeforeRequest") + "\nfunction observeTurnWorkflowHUDTiming() {} // Timing UI is exercised in its dedicated runtime fixture."
	script := beforeRequest + `
const SESSION_FALLBACK = "default";
const settings = {enabled:true};
let _latestOrchResultForUI = null;
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
let _activeFinalConfirmationRequestContext = null;
let _pendingFinalConfirmationDrainRequested = false;
let _memoryTransportBodyInterceptorRegistration = null;
const lifecycleStates = {};
function recordRisuHookLifecycle(name,state){ lifecycleStates[name]=state; }
function warnLog(){}
function debugLog(){}
async function registerMemoryTransportBodyInterceptor(){ calls.push("add:body"); }
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
  _turnWorkflowHUDActiveRequestId = "req-1";
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
		if os.IsNotExist(err) {
			t.Skip("optional AC Recomposer Agent.js is not part of the public repository")
		}
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

func TestRegisteredRequestCallbacksDetachExactBeforeRequestContext(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for registered request-context runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functions := strings.Join([]string{
		extractArchiveCenterJSAsyncFunction(t, src, "captureFinalConfirmationRequestContext"),
		extractArchiveCenterJSFunction(t, src, "finalConfirmationRequestContextOwnsPendingResponse"),
		extractArchiveCenterJSFunction(t, src, "finalConfirmationRequestContextRetryIdentityMatches"),
		extractArchiveCenterJSFunction(t, src, "finalConfirmationRequestContextHasReusablePayloadPlan"),
		extractArchiveCenterJSFunction(t, src, "installFinalConfirmationRequestContext"),
		extractArchiveCenterJSFunction(t, src, "acceptRisuAfterRequestFinal"),
		extractArchiveCenterJSAsyncFunction(t, src, "onBeforeRequest") + "\nfunction observeTurnWorkflowHUDTiming() {} // Timing UI is exercised in its dedicated runtime fixture.",
		extractArchiveCenterJSFunction(t, src, "onAfterRequest") + "\nfunction observeTurnWorkflowHUDTiming() {} // Timing UI is exercised in its dedicated runtime fixture.",
		extractArchiveCenterJSAsyncFunction(t, src, "registerRisuLifecycleHooks"),
		extractArchiveCenterJSFunction(t, src, "startTurnWorkflowHUDWatch"),
	}, "\n")
	script := functions + `
const LOG_PREFIX="[test]";
const settings={enabled:true,debug:false,webDirectBridgeEnabled:false};
const runtimeUpdates=[];
const registered={};
let current={sessionId:"session-a",charIdx:1,chatIdx:2,hostChatId:"cid-a",user:"user-a"};
let requestSeq=0;
let _activeFinalConfirmationRequestContext=null;
let _sessionCache={};
let _latestOrchResultForUI={_chatSessionId:"wrong-global-session"};
let lastTurnTrace=null;
let _effectiveInputAwaitingNewTurn=false;
let _turnWorkflowHUDWatchRunning=false;
let _turnWorkflowHUDActiveRequestId="";
const console={log(){},warn(){},error(...args){globalThis.__errors=(globalThis.__errors||[]).concat([args.join(" ")]);}};
const R={
  async addRisuScriptHandler(name,fn){registered[name]=fn;},
  async addRisuReplacer(name,fn){registered[name]=fn;},
  async addRisuChatListener(name,fn){registered[name]=fn;},
  async onUnload(fn){registered.unload=fn;},
  async getChatFromIndex(charIdx,chatIdx){
    if(charIdx!==current.charIdx || chatIdx!==current.chatIdx) throw new Error("uncaptured coordinates");
    return {id:current.hostChatId,message:[{role:"user",data:current.user,chatId:"user-"+current.hostChatId,time:1000+requestSeq}]};
  }
};
function recordRisuHookLifecycle(){}
async function registerMemoryTransportBodyInterceptor(){}
function warnLog(...args){throw new Error("unexpected warning: "+args.join(" "));}
function debugLog(){}
function clearArchiveCenterRecomposerBridge(){}
function isSaveType(type){return !type || type==="model";}
function isNarrativeType(type){return !type || type==="model" || type==="submodel" || type==="otherAx";}
function extractMessages(payload){return {messages:payload.messages,path:["messages"],hasMessageSlot:true};}
function normalizeMessagesForOrchestration(messages){return messages;}
function extractRuntimeCurrentChatTokenInfo(){return {currentChatTokens:null,source:"none"};}
async function getCurrentChatSessionId(){return current.sessionId;}
async function resolveCanonicalWriteSessionId(sid){return sid;}
function captureSessionHostContextFromCache(sid){
  if(sid!==current.sessionId) throw new Error("session recaptured from wrong owner");
  return {sessionId:sid,charIdx:current.charIdx,chatIdx:current.chatIdx,hostChatId:current.hostChatId,stableCharacterId:"char-"+current.charIdx};
}
async function getCurrentActiveChatSourceObservationMessages(_sid,_hostContext,includeChat){
  const messages=[{role:"user",raw_content:current.user,message_index:0}];
  return includeChat ? {messages,chat:{scriptstate:{}}} : messages;
}
async function buildYumiV1ArchiveReadContext(payloadMessages,activeMessages){
  return {payloadMessages,activeMessages,stats:{markerBlocks:0,modelSourceBlocks:0,displayFallbackBlocks:0}};
}
function makeOrchRequestId(sid){requestSeq++; return sid+":request:"+requestSeq;}
function primeTurnWorkflowHUD(requestId){_turnWorkflowHUDActiveRequestId=requestId; return requestId;}
async function captureAssistantPrefillSeedForSession(){}
async function resolveCurrentActiveChatObject(sid,hostContext){
  if(sid!==hostContext.sessionId || hostContext.hostChatId!==current.hostChatId) throw new Error("capture lost fixed host context");
  return {charIdx:hostContext.charIdx,chatIdx:hostContext.chatIdx,chat:await R.getChatFromIndex(hostContext.charIdx,hostContext.chatIdx)};
}
function computeOrchestrationDirtyHashOr1c(value){return "hash:"+String(value||"");}
function updateRuntimeState(name,status,value){runtimeUpdates.push({name,status,value});}
function bindRawInputObservationToRequest(sid,requestId){return {sessionId:sid,boundRequestId:requestId,text:current.user,actualEmptyInput:false};}
function buildPostOutputSecondaryRequestContext(){return null;}
function buildPrepareTurnHostObservations(){return {active_chat:[{role:"user",raw_content:current.user,message_index:0}]};}
async function observePrepareTurnBootstrap(){return null;}
function buildPrepareTurnSourceObservations(){return {sourceObservation:{},capabilityObservation:{}};}
function beginNextInputFinalizationPipeline(){return {owned:false,started:false,reason:"no_pending_previous_turn"};}
async function tryPrepareTurn(){return {source:"backend",currentInputDecision:{status:"deferred",reason_code:"fixture_stop_after_capture"}};}
async function onInputHook(value){return value;}
function onRisuOutput(){}
async function removeRegisteredRisuHooksOnUnload(){}
function normalizeAssistantPersistenceCandidate(value){return String(value||"").trim();}
function takeAssistantPrefillSeedForSession(){return "";}
function sanitizeNarrativeOutputForDisplay(value){return value;}
function buildSanitizeTrace(){return null;}
function stripAssistantPrefillFromResponse(value){return value;}
function schedulePostOutputFinalReplacement(){throw new Error("unexpected secondary replacement");}
function markNonMainRequestHookSkipped(){throw new Error("unexpected non-main skip");}
function turnWorkflowHUDIsEnabled(){return true;}
function dismissTurnWorkflowHUD(){throw new Error("unexpected HUD dismiss");}

async function runRequest(expectedSession,expectedCID,expectedChar,expectedChat,userText){
  current={sessionId:expectedSession,charIdx:expectedChar,chatIdx:expectedChat,hostChatId:expectedCID,user:userText};
  _sessionCache={sessionId:expectedSession,charIdx:expectedChar,chatIdx:expectedChat,observedChatUniqueId:expectedCID};
  const payload={messages:[{role:"user",content:userText}]};
  const returned=await registered.beforeRequest(payload,"model");
  if(returned!==payload) throw new Error("beforeRequest did not preserve fixture payload");
  const context=_activeFinalConfirmationRequestContext;
  if(!context) throw new Error("beforeRequest did not install request context");
  if(context.sessionId!==expectedSession || context.hostChatId!==expectedCID || context.characterIndex!==expectedChar || context.chatIndex!==expectedChat) {
    throw new Error("captured identity mismatch: "+JSON.stringify(context));
  }
  if(!context.requestId.startsWith(expectedSession+":request:") || !context.rawInputObservation || context.rawInputObservation.boundRequestId!==context.requestId) {
    throw new Error("workflow request binding mismatch: "+JSON.stringify(context));
  }
  const accepted=acceptRisuAfterRequestFinal(context,"assistant-"+expectedCID);
  if(!accepted.accepted || accepted.observation.session_id!==expectedSession || accepted.observation.host_chat_id!==expectedCID || accepted.observation.archive_center_request_correlation_id!==context.requestId) {
    throw new Error("accepted observation lost fixed context: "+JSON.stringify(accepted));
  }
  return context;
}

(async function(){
  await registerRisuLifecycleHooks();
  if(registered.beforeRequest!==onBeforeRequest || registered.afterRequest!==onAfterRequest) {
    throw new Error("production callbacks were not the registered callbacks");
  }

  const first=await runRequest("session-a","cid-a",1,2,"user-a");
  _sessionCache={sessionId:"session-b",charIdx:9,chatIdx:9,observedChatUniqueId:"cid-b"};
  _latestOrchResultForUI={_chatSessionId:"session-b",_userInput:"wrong-user"};
  const firstOutput=registered.afterRequest("assistant-cid-a","model");
  if(firstOutput!=="assistant-cid-a" || _activeFinalConfirmationRequestContext!==null) throw new Error("afterRequest did not detach first context");
  const firstUpdate=runtimeUpdates.filter(item=>item.name==="lastStreamingAfterRequest").at(-1);
  if(!firstUpdate || firstUpdate.value.sessionId!=="session-a" || firstUpdate.value.requestType!=="model") {
    throw new Error("afterRequest re-decided owner from globals: "+JSON.stringify(firstUpdate));
  }

  const consecutive=await runRequest("session-a","cid-a",1,2,"user-a-next");
  if(consecutive===first || consecutive.requestId===first.requestId) throw new Error("same-session requests shared request state");
  registered.afterRequest("assistant-cid-a","model");

  const second=await runRequest("session-b","cid-b",4,5,"user-b");
  if(second.sessionId===first.sessionId || second.requestId===first.requestId) throw new Error("cross-session requests shared request state");
  registered.afterRequest("assistant-cid-b","model");

  _turnWorkflowHUDActiveRequestId=second.requestId;
  _turnWorkflowHUDWatchRunning=false;
  startTurnWorkflowHUDWatch(first.requestId);
  if(_turnWorkflowHUDActiveRequestId!==second.requestId || _turnWorkflowHUDWatchRunning) {
    throw new Error("older request stole newer request HUD ownership");
  }
})().catch(function(err){console.error(err && err.stack || err);process.exit(1);});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("registered request-context runtime fixture failed: %v\n%s", err, output)
	}
}

func TestAfterRequestTurnReservationMutatesOnlyCapturedRequestTrace(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for request-scoped turn reservation fixture")
		}
	}
	src := readArchiveCenterJS(t)
	reserve := extractArchiveCenterJSAsyncFunction(t, src, "reserveAfterRequestPersistenceTurnIndex")
	script := reserve + `
const capturedHostContext={sessionId:"session-a",charIdx:1,chatIdx:2,hostChatId:"cid-a"};
const capturedOrchestration={_chatSessionId:"session-a",_trace:{owner:"request-a"}};
const newerUIRequest={_chatSessionId:"session-b",_trace:{owner:"request-b"}};
let _latestOrchResultForUI=newerUIRequest;
let observedHostContext=null;
function peekNextTurnIndex(){return 75;}
async function safeCall(fn){return await fn();}
async function fetchBackendLatestTurnIndexForSession(sid){if(sid!=="session-a") throw new Error("latest-turn session changed");return 74;}
function setTurnCounterAtLeast(){}
async function findActiveChatCompletedTurnPairForContent(sid,user,assistant,hostContext){
  if(sid!=="session-a" || user!=="user-a" || assistant!=="assistant-a") throw new Error("pair lookup owner changed");
  observedHostContext=hostContext;
  return {source:"captured_active_chat",risuUserMessageIndex:10,risuAssistantMessageIndex:11,pairCount:75};
}
function normalizeTurnPairCompareText(value){return String(value||"").trim();}
function normalizeAssistantPersistenceCandidate(value){return String(value||"").trim();}
async function findActiveChatCompletedTurnPairForUserContent(){throw new Error("unexpected secondary pair lookup");}
async function findLatestActiveChatCompletedTurnPair(){throw new Error("unexpected latest active-chat lookup");}
async function requestBackendSessionRoutingTurnResolution(sid,mode,pair){
  if(sid!=="session-a" || mode!=="pair" || pair.source!=="captured_active_chat") throw new Error("routing owner changed");
  return {status:"normal",turnIndex:75,localTurnIndex:75,baseline:{reason:"captured"}};
}
function setTurnCounterExact(){}
function nextTurnIndex(){return 999;}
function debugLog(...args){throw new Error("unexpected reservation failure: "+args.join(" "));}
(async()=>{
  const turn=await reserveAfterRequestPersistenceTurnIndex(
    "session-a","user-a","assistant-a",null,capturedHostContext,capturedOrchestration
  );
  if(turn!==75) throw new Error("reserved wrong turn: "+turn);
  if(observedHostContext!==capturedHostContext) throw new Error("captured host context was replaced");
  if(!capturedOrchestration._trace.turnIndexResolution || capturedOrchestration._trace.turnIndexResolution.turnIndex!==75) {
    throw new Error("captured request trace did not receive turn resolution");
  }
  if(newerUIRequest._trace.turnIndexResolution) throw new Error("older persistence mutated newer request trace");
})().catch(err=>{console.error(err && err.stack || err);process.exit(1);});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("request-scoped turn reservation fixture failed: %v\n%s", err, output)
	}
}

func TestOverlappingBeforeRequestContextsFailClosedWithoutOwnershipMixing(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for request overlap fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functions := strings.Join([]string{
		extractArchiveCenterJSFunction(t, src, "finalConfirmationRequestContextOwnsPendingResponse"),
		extractArchiveCenterJSFunction(t, src, "finalConfirmationRequestContextRetryIdentityMatches"),
		extractArchiveCenterJSFunction(t, src, "finalConfirmationRequestContextHasReusablePayloadPlan"),
		extractArchiveCenterJSFunction(t, src, "installFinalConfirmationRequestContext"),
		extractArchiveCenterJSFunction(t, src, "onAfterRequest") + "\nfunction observeTurnWorkflowHUDTiming() {} // Timing UI is exercised in its dedicated runtime fixture.",
	}, "\n")
	script := functions + `
let _activeFinalConfirmationRequestContext=null;
const updates=[];
const settings={enabled:true};
function recordRisuHookLifecycle(){}
function debugLog(){}
function isNarrativeType(){return true;}
function isSaveType(){return true;}
function updateRuntimeState(name,status,value){updates.push({name,status,value});}
const a={sessionId:"session-a",requestId:"request-a",requestType:"model",state:"captured"};
const b={sessionId:"session-b",requestId:"request-b",requestType:"model",state:"captured"};
const firstInstall=installFinalConfirmationRequestContext(a);
if(!firstInstall || firstInstall.status!=="installed" || _activeFinalConfirmationRequestContext!==a) throw new Error("A context was not installed");
const overlapInstall=installFinalConfirmationRequestContext(b);
if(!overlapInstall || overlapInstall.status!=="rejected") throw new Error("overlap was incorrectly installed");
if(_activeFinalConfirmationRequestContext!==null) throw new Error("overlap retained an ambiguous owner");
if(a.state!=="terminal" || b.state!=="terminal") throw new Error("overlap was not terminalized: "+JSON.stringify({a,b}));
if(a.terminalReason!=="overlapping_before_request_without_host_correlation" || b.terminalReason!==a.terminalReason) throw new Error("overlap reason mismatch");
if(onAfterRequest("A response","model")!=="A response") throw new Error("A completion changed display content");
if(onAfterRequest("B response","model")!=="B response") throw new Error("B completion changed display content");
if(updates.length!==3 || updates[0].value.reason_code!=="overlapping_before_request_without_host_correlation") throw new Error("overlap warning missing");
if(updates[1].value.reason_code!=="before_request_context_missing" || updates[2].value.reason_code!=="before_request_context_missing") throw new Error("completion was assigned to an ambiguous context");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("overlapping request context fixture failed: %v\n%s", err, output)
	}
}

func TestSameLogicalRequestRetryReusesPreparedContextAndAppliesAuxiliaryOnce(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for same-request retry fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functions := strings.Join([]string{
		extractArchiveCenterJSFunction(t, src, "finalConfirmationRequestContextOwnsPendingResponse"),
		extractArchiveCenterJSFunction(t, src, "finalConfirmationRequestContextRetryIdentityMatches"),
		extractArchiveCenterJSFunction(t, src, "finalConfirmationRequestContextHasReusablePayloadPlan"),
		extractArchiveCenterJSFunction(t, src, "markFinalConfirmationRetryPayloadReady"),
		extractArchiveCenterJSFunction(t, src, "installFinalConfirmationRequestContext"),
		extractArchiveCenterJSFunction(t, src, "injectAuxiliaryBlock"),
	}, "\n")
	script := functions + `
let _activeFinalConfirmationRequestContext=null;
const updates=[];
const settings={auxiliaryInjectionPlacement:"after_first_system",auxiliaryInjectionAnchorMarker:""};
function updateRuntimeState(name,status,value){updates.push({name,status,value});}
function warnLog(...args){throw new Error("unexpected warning: "+args.join(" "));}
function getPayloadMessageRoleAndText(message){return {role:String(message&&message.role||""),text:String(message&&message.content||"")};}
function extractMessages(payload){return {messages:payload,rebuild(messages){return messages;}};}
function resolveAuxiliaryInjectionPlacement(messages){return {insertIndex:1,resolvedMode:"after_first_system"};}

const identity={
  sessionId:"session-a",requestType:"model",characterIndex:1,chatIndex:2,hostChatId:"cid-a",
  requestMessageCount:3,userMessageIndex:2,userObservedPairOrdinal:2,userMessageChatId:"user-cid-a",
  userMessageTimeMs:2000,userObservedContentHash:"hash:user-a",userObservedContent:"user-a",baselineAssistantIndex:1,
  baselineAssistantContentHash:"hash:assistant-prev",baselineGenerationId:"generation-prev",
  baselineAssistantTimeMs:1500,
};
const prepared={...identity,requestId:"request-a",state:"captured",beforeRequestAttemptCount:1,
  pendingContext:{status:"ready"},orchestrationResult:{
    _chatSessionId:"session-a",
    _payloadApplicationObservation:{payload_application_status:"applied"},
    _injectionPack:{payload_application_plan:{
      contract_version:"payload_application_plan.v1",owner:"go",
      apply_rule:"apply_exact_text_without_reassembly",status:"ready",auxiliary_text:"memory-a"
    }}
  }};
const retry={...identity,requestId:"request-b",state:"captured",beforeRequestAttemptCount:1,
  pendingContext:null,orchestrationResult:null};

if(!markFinalConfirmationRetryPayloadReady(prepared,prepared.orchestrationResult,{injectionRequested:true})) {
  throw new Error("prepared payload was not marked reusable: "+JSON.stringify(prepared));
}

const firstInstall=installFinalConfirmationRequestContext(prepared);
if(!firstInstall || firstInstall.status!=="installed" || _activeFinalConfirmationRequestContext!==prepared) {
  throw new Error("first beforeRequest context was not installed: "+JSON.stringify(firstInstall));
}

const firstPayload=injectAuxiliaryBlock([
  {role:"system",content:"base"},
  {role:"user",content:"user-a"},
],"memory-a").payload;

// The provider's first attempt fails retryably. RisuAI then invokes the same
// production beforeRequest replacer again for the unchanged Host turn.
const retryInstall=installFinalConfirmationRequestContext(retry);
if(!retryInstall || retryInstall.status!=="retry_reused") {
  throw new Error("same logical request was not reused: "+JSON.stringify(retryInstall));
}
if(_activeFinalConfirmationRequestContext!==prepared || prepared.state!=="captured") {
  throw new Error("retry replaced or terminalized the prepared owner: "+JSON.stringify({prepared,retry}));
}
if(prepared.beforeRequestAttemptCount!==2 || retry.state!=="superseded") {
  throw new Error("retry attempt lifecycle mismatch: "+JSON.stringify({prepared,retry}));
}

const secondPayload=injectAuxiliaryBlock(firstPayload,"memory-a").payload;
const auxiliary=secondPayload.filter(message=>message.role==="system" && message.content==="[Archive Center — Auxiliary Context]\n\nmemory-a");
if(auxiliary.length!==1) throw new Error("retry duplicated auxiliary context: "+JSON.stringify(secondPayload));
const duplicatePayload=injectAuxiliaryBlock(firstPayload.concat([{role:"system",content:"[Archive Center — Auxiliary Context]\n\nmemory-a"}]),"memory-a");
if(duplicatePayload.payload.filter(message=>message.content==="[Archive Center — Auxiliary Context]\n\nmemory-a").length!==1 || duplicatePayload.duplicateCollapsedCount!==1) {
  throw new Error("exact duplicate Archive blocks were not collapsed");
}
const conflictingInput=[{role:"system",content:"base"},{role:"system",content:"[Archive Center — Auxiliary Context]\n\nother-memory"}];
const conflicting=injectAuxiliaryBlock(conflictingInput,"memory-a");
if(conflicting.payload!==conflictingInput || conflicting.ambiguous!==true || conflicting.reason!=="archive_auxiliary_context_conflict") {
  throw new Error("conflicting Archive block was overwritten: "+JSON.stringify(conflicting));
}
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("same-request retry fixture failed: %v\n%s", err, output)
	}
}

func TestFailOnceProviderRetryUsesProductionBeforeRequestFastPath(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for fail-once beforeRequest retry fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functions := strings.Join([]string{
		extractArchiveCenterJSAsyncFunction(t, src, "captureFinalConfirmationRequestContext"),
		extractArchiveCenterJSFunction(t, src, "finalConfirmationRequestContextOwnsPendingResponse"),
		extractArchiveCenterJSFunction(t, src, "finalConfirmationRequestContextRetryIdentityMatches"),
		extractArchiveCenterJSFunction(t, src, "finalConfirmationRequestContextHasReusablePayloadPlan"),
		extractArchiveCenterJSFunction(t, src, "reapplyFinalConfirmationRetryPayload"),
		extractArchiveCenterJSFunction(t, src, "installFinalConfirmationRequestContext"),
		extractArchiveCenterJSAsyncFunction(t, src, "onBeforeRequest") + "\nfunction observeTurnWorkflowHUDTiming() {} // Timing UI is exercised in its dedicated runtime fixture.",
	}, "\n")
	script := functions + `
const settings={enabled:true,debug:false};
const updates=[];
let heavyCalls=0;
let replayCalls=0;
let retryHUDCalls=0;
const chat={id:"cid-a",message:[{role:"user",data:"same user",chatId:"user-cid-a",time:2000}]};
const R={getChatFromIndex(){return chat;}};
const identity={
  sessionId:"session-a",requestType:"model",characterIndex:1,chatIndex:2,hostChatId:"cid-a",
  requestMessageCount:1,userMessageIndex:0,userObservedPairOrdinal:1,userMessageChatId:"user-cid-a",
  userMessageTimeMs:2000,userObservedContentHash:"hash:same user",userObservedContent:"same user",
  baselineAssistantIndex:-1,baselineAssistantContentHash:"",baselineGenerationId:"",baselineAssistantTimeMs:0,
};
const prepared={...identity,requestId:"session-a:request:prepared",state:"captured",beforeRequestAttemptCount:1,
  retryPayloadReady:true,payloadInjectionReplayAllowed:true,payloadRewriteApplied:false,
  pendingContext:{status:"ready"},orchestrationResult:{
    _chatSessionId:"session-a",
    _injectionPack:{payload_application_plan:{contract_version:"payload_application_plan.v1",owner:"go",apply_rule:"apply_exact_text_without_reassembly",status:"ready",auxiliary_text:"memory-a"}}
  }};
let _activeFinalConfirmationRequestContext=prepared;
function recordRisuHookLifecycle(){}
function debugLog(){}
function clearArchiveCenterRecomposerBridge(){}
function isSaveType(){return true;}
function extractMessages(payload){return {messages:payload,hasMessageSlot:true};}
function normalizeMessagesForOrchestration(messages){return messages;}
function extractRuntimeCurrentChatTokenInfo(){return {};}
async function getCurrentChatSessionId(){return "session-a";}
async function resolveCanonicalWriteSessionId(value){return value;}
function captureSessionHostContextFromCache(){return {sessionId:"session-a",charIdx:1,chatIdx:2,hostChatId:"cid-a"};}
async function resolveCurrentActiveChatObject(){return {charIdx:1,chatIdx:2,chat};}
function computeOrchestrationDirtyHashOr1c(value){return "hash:"+String(value);}
function updateRuntimeState(name,status,value){updates.push({name,status,value});}
function renderTurnWorkflowHUDSameRequestRetry(requestId,attemptCount){
  retryHUDCalls++;
  if(requestId!==prepared.requestId || attemptCount!==2) throw new Error("retry HUD lost request ownership");
}
function applyContextInjection(payload,orchResult){
  replayCalls++;
  if(orchResult!==prepared.orchestrationResult) throw new Error("prepared Go plan was replaced");
  return {payload:payload.concat([{role:"system",content:"[Archive Center — Auxiliary Context]\\n\\nmemory-a"}]),injectionResult:{applied:true}};
}
function rewriteLastUserMessage(){throw new Error("unexpected input rewrite");}
function makeOrchRequestId(){throw new Error("retry created a new request ID");}
function primeTurnWorkflowHUD(){heavyCalls++;throw new Error("retry primed a new HUD");}
async function captureAssistantPrefillSeedForSession(){heavyCalls++;throw new Error("retry recaptured prefill");}
async function getCurrentActiveChatSourceObservationMessages(){heavyCalls++;throw new Error("retry re-read active chat preparation inputs");}
function bindRawInputObservationToRequest(){heavyCalls++;throw new Error("retry rebound raw input");}
async function tryPrepareTurn(){heavyCalls++;throw new Error("retry called prepare-turn");}

(async function(){
  let providerAttempts=1;
  let outgoing=[{role:"user",content:"same user"}];
  const firstFailure={retryable:true,code:"fixture_fail_once"};
  if(!firstFailure.retryable) throw new Error("fixture did not fail retryably");
  outgoing=await onBeforeRequest(outgoing,"model");
  providerAttempts++;
  if(providerAttempts!==2) throw new Error("provider retry count mismatch");
  if(prepared.beforeRequestAttemptCount!==2 || _activeFinalConfirmationRequestContext!==prepared) throw new Error("prepared request was not reused");
  if(replayCalls!==1 || retryHUDCalls!==1 || heavyCalls!==0) throw new Error("retry fast path side effects mismatch: "+JSON.stringify({replayCalls,retryHUDCalls,heavyCalls}));
  if(outgoing.filter(message=>String(message.content||"").startsWith("[Archive Center — Auxiliary Context]")).length!==1) throw new Error("replayed payload missing exact Archive block");
})().catch(err=>{console.error(err&&err.stack||err);process.exit(1);});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fail-once beforeRequest retry fixture failed: %v\n%s", err, output)
	}
}

func TestNewInputEndsStaleRetryContextAndSameTextStartsNewTurn(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for new-input request lifecycle fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functions := strings.Join([]string{
		extractArchiveCenterJSFunction(t, src, "finalConfirmationRequestContextOwnsPendingResponse"),
		extractArchiveCenterJSFunction(t, src, "finalConfirmationRequestContextRetryIdentityMatches"),
		extractArchiveCenterJSFunction(t, src, "finalConfirmationRequestContextHasReusablePayloadPlan"),
		extractArchiveCenterJSFunction(t, src, "installFinalConfirmationRequestContext"),
		extractArchiveCenterJSFunction(t, src, "terminalizeActiveFinalConfirmationRequestContext"),
		extractArchiveCenterJSAsyncFunction(t, src, "onInputHook"),
	}, "\n")
	script := functions + `
let _activeFinalConfirmationRequestContext={sessionId:"session-a",requestId:"request-old",requestType:"model",state:"captured"};
const stale=_activeFinalConfirmationRequestContext;
const updates=[];
const cached=[];
const _rollbackHistoryTrimGuardBySession=new Map();
function recordRisuHookLifecycle(){}
function updateRuntimeState(name,status,value){updates.push({name,status,value});}
async function getCurrentChatSessionId(){return "session-a";}
function cacheRawInputForSession(sessionId,text){cached.push({sessionId,text});}
function isRisuHistoryTrimCommandText(){return false;}
function warnLog(...args){throw new Error("unexpected warning: "+args.join(" "));}
const next={sessionId:"session-a",requestId:"request-next",requestType:"model",state:"captured"};
(async function(){
  const text=await onInputHook("same sentence");
  if(text!=="same sentence" || cached.length!==1) throw new Error("new input observation was lost");
  if(_activeFinalConfirmationRequestContext!==null || stale.state!=="terminal" || stale.terminalReason!=="new_host_input_observed") throw new Error("stale request survived new input");
  const installed=installFinalConfirmationRequestContext(next);
  if(!installed || installed.status!=="installed" || _activeFinalConfirmationRequestContext!==next) throw new Error("identical text on the next turn was mistaken for a retry");
})().catch(err=>{console.error(err&&err.stack||err);process.exit(1);});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("new-input request lifecycle fixture failed: %v\n%s", err, output)
	}
}

func TestSameRequestRetryHUDKeepsStageSixUntilStageSevenArrives(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for retry HUD lifecycle fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functions := strings.Join([]string{
		extractArchiveCenterJSFunction(t, src, "rememberTurnWorkflowHUDHostWarning"),
		extractArchiveCenterJSFunction(t, src, "consumeTurnWorkflowHUD"),
		extractArchiveCenterJSFunction(t, src, "retainTurnWorkflowHUDHostTiming"),
		extractArchiveCenterJSFunction(t, src, "renderTurnWorkflowHUDSameRequestRetry"),
	}, "\n")
	script := functions + `
const TURN_WORKFLOW_HUD_CONTRACT="turn_workflow_hud.v3";
const settings={turnWorkflowHUDEnabled:true};
let _turnWorkflowHUDUnloaded=false;
let _turnWorkflowHUDActiveRequestId="request-a";
let _turnWorkflowHUDLastRevision=6;
let _turnWorkflowHUDLastView={
  contract_version:TURN_WORKFLOW_HUD_CONTRACT,request_id:"request-a",revision:6,status:"awaiting_final_output",
  current_stage:{key:"awaiting_final_output",ordinal:6,total:12}
};
const _turnWorkflowHUDHostWarningsByRequestId=new Map();
const rendered=[];
function turnWorkflowHUDIsEnabled(){return true;}
function tf(_key,values){return "Risu retry · "+String(values.n);}
function renderTurnWorkflowHUD(view){rendered.push({ordinal:Number(view.current_stage.ordinal),status:String(view.status)});}

if(!renderTurnWorkflowHUDSameRequestRetry("request-a",2)) throw new Error("retry HUD observation was rejected");
const warning=(_turnWorkflowHUDHostWarningsByRequestId.get("request-a")||[])[0];
if(!warning || warning.message!=="Risu retry · 2") throw new Error("retry attempt notice missing");
if(rendered.length!==1 || rendered[0].ordinal!==6 || rendered[0].status!=="awaiting_final_output") throw new Error("retry reset or advanced the 6/12 HUD stage");
if(!consumeTurnWorkflowHUD({
  contract_version:TURN_WORKFLOW_HUD_CONTRACT,request_id:"request-a",revision:7,status:"running",
  current_stage:{key:"final_output_accepted",ordinal:7,total:12}
})) throw new Error("stage-seven backend view was rejected");
if(rendered.length!==2 || rendered[1].ordinal!==7) throw new Error("accepted output did not advance HUD to 7/12");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("retry HUD lifecycle fixture failed: %v\n%s", err, output)
	}
}

func TestRegisteredAfterRequestCarriesEachCapturedContextIntoCompleteTurn(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for registered afterRequest persistence fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functions := strings.Join([]string{
		extractArchiveCenterJSFunction(t, src, "acceptRisuAfterRequestFinal"),
		extractArchiveCenterJSFunction(t, src, "onAfterRequest") + "\nfunction observeTurnWorkflowHUDTiming() {} // Timing UI is exercised in its dedicated runtime fixture.",
		extractArchiveCenterJSAsyncFunction(t, src, "registerRisuLifecycleHooks"),
	}, "\n")
	script := functions + `
const settings={enabled:true,debug:false};
const registered={};
const completeCalls=[];
const warnings=[];
const AUTO_CONTINUE_USER_INPUT_MARKER="[auto-continue]";
let _activeFinalConfirmationRequestContext=null;
let lastTurnTrace=null;
let panelOpen=false;
const runtimeState={lastCompleteTurnStatus:{}};
const R={
  async addRisuScriptHandler(name,fn){registered[name]=fn;},
  async addRisuReplacer(name,fn){registered[name]=fn;},
  async addRisuChatListener(name,fn){registered[name]=fn;},
  async onUnload(fn){registered.unload=fn;},
};
async function onInputHook(value){return value;}
async function onBeforeRequest(value){return value;}
function onRisuOutput(){}
async function removeRegisteredRisuHooksOnUnload(){}
async function registerMemoryTransportBodyInterceptor(){}
function recordRisuHookLifecycle(){}
function debugLog(){}
function warnLog(...args){warnings.push(args.join(" "));}
function isNarrativeType(type){return !type||type==="model";}
function isSaveType(type){return !type||type==="model";}
function updateRuntimeState(){}
function normalizeAssistantPersistenceCandidate(value){return String(value||"").trim();}
function takeAssistantPrefillSeedForSession(){return "";}
function sanitizeNarrativeOutputForDisplay(value){return value;}
function buildSanitizeTrace(){return null;}
function stripAssistantPrefillFromResponse(value){return value;}
function schedulePostOutputFinalReplacement(){throw new Error("unexpected secondary replacement");}
function markNonMainRequestHookSkipped(){throw new Error("unexpected non-main skip");}
function computeOrchestrationDirtyHashOr1c(value){return "hash:"+String(value||"");}
function attachSanitizeTrace(){}
function recordStep13GovernorTurnOutcomeGv1c(){return {};}
function applyStep13GovernorFailureBudgetTraceGv1c(){}
function loadTurnCounter(){return 1;}
function shouldSkipUserInputPersistence(value){return !String(value||"").trim();}
function isCanonicalHostUserInputText(value){return !!String(value||"").trim();}
function peekNextTurnIndex(sid){return sid==="session-a"?75:12;}
function canonicalizeAssistantOutputForPersistence(value){return value;}
function normalizeTurnPairCompareText(value){return String(value||"").trim();}
async function findRecentPersistedCompleteTurnPairForContent(){return null;}
async function reserveAfterRequestPersistenceTurnIndex(sid,user,assistant,observation,hostContext,orchestrationResult){
  if(hostContext.sessionId!==sid || orchestrationResult._chatSessionId!==sid) throw new Error("reservation lost captured owner");
  if(observation.archive_center_request_correlation_id!==orchestrationResult.requestId) throw new Error("reservation lost workflow request id");
  return ({"request-a":75,"request-b":12,"request-c":76,"request-d":77})[orchestrationResult.requestId] || 1;
}
function buildMainNarrativePersistenceGateDecision(){return {allowed:true};}
function buildTemporalStateSurfaceStep19(){return {};}
function readSceneTemporalStateFromOrchResultStep19(){return {};}
function validateResponseTemporalDeicticStep19(){return {status:"ok"};}
function isMetaUserMessage(){return false;}
function sanitizeForCritic(value){return value;}
async function safeCall(fn,fallback){try{return await fn();}catch{return fallback;}}
function buildRisuRequestObservation(){return {contract_version:"risu_request_observation.v1"};}
async function buildCompleteTurnRequestBody(turn,user,assistant,context,sid,improvement,options){
  return {
    chat_session_id:sid,turn_index:turn,user_input:user,assistant_content:assistant,
    client_meta:{
      turn_workflow_request_id:options.sourceAcceptanceFinality.archive_center_request_correlation_id,
      captured_host_context:options.hostContext,
      captured_orchestration:options.orchestrationResult,
    },
  };
}
function buildCompleteTurnQueuePayload(body){return body;}
async function tryCompleteTurn(turn,user,assistant,context,sid,improvement,body){
  completeCalls.push({turn,user,assistant,sid,body});
  return {status:"ok",save_ok:true,raw_committed:true,critic_triggered:true,turn_index:turn};
}
function context(sid,cid,requestId,user,turn){
  return {
    sessionId:sid,requestId,requestType:"model",characterIndex:turn,chatIndex:turn+1,hostChatId:cid,
    hostContext:{sessionId:sid,charIdx:turn,chatIdx:turn+1,hostChatId:cid},
    rawInputObservation:{text:user,actualEmptyInput:false,boundRequestId:requestId},
    orchestrationResult:{_chatSessionId:sid,_userInput:user,_recentContext:[],requestId,_trace:{owner:requestId}},
    state:"captured",
  };
}
(async()=>{
  await registerRisuLifecycleHooks();
  if(registered.afterRequest!==onAfterRequest) throw new Error("production afterRequest was not registered");
  const a=context("session-a","cid-a","request-a","user-a",1);
  _activeFinalConfirmationRequestContext=a;
  if(registered.afterRequest("assistant-a","model")!=="assistant-a") throw new Error("A response changed");
  const b=context("session-b","cid-b","request-b","user-b",4);
  _activeFinalConfirmationRequestContext=b;
  if(registered.afterRequest("assistant-b","model")!=="assistant-b") throw new Error("B response changed");
  const c=context("session-a","cid-a","request-c","user-c",1);
  _activeFinalConfirmationRequestContext=c;
  if(registered.afterRequest("assistant-c","model")!=="assistant-c") throw new Error("C response changed");
  const d=context("session-a","cid-a","request-d","user-d",1);
  _activeFinalConfirmationRequestContext=d;
  if(registered.afterRequest("assistant-d","model")!=="assistant-d") throw new Error("D response changed");
  if(registered.afterRequest("assistant-d","model")!=="assistant-d") throw new Error("duplicate D response changed");
  const sayNothing=context("session-a","cid-a","request-e","user-e",1);
  _activeFinalConfirmationRequestContext=sayNothing;
  if(registered.afterRequest("","model")!=="") throw new Error("Say Nothing response changed");
  if(_activeFinalConfirmationRequestContext!==null || sayNothing.state!=="terminal" || sayNothing.terminalReason!=="after_request_final_unavailable") {
    throw new Error("Say Nothing did not close its request context: "+JSON.stringify(sayNothing));
  }
  for(let i=0;i<100 && completeCalls.length<4;i++) await new Promise(resolve=>setTimeout(resolve,1));
  if(completeCalls.length!==4) throw new Error("complete-turn calls="+completeCalls.length+" warnings="+warnings.join(" | "));
  const byRequest=Object.fromEntries(completeCalls.map(call=>[call.body.client_meta.turn_workflow_request_id,call]));
  for(const [sid,requestId,cid,user,assistant,turn] of [
    ["session-a","request-a","cid-a","user-a","assistant-a",75],
    ["session-b","request-b","cid-b","user-b","assistant-b",12],
    ["session-a","request-c","cid-a","user-c","assistant-c",76],
    ["session-a","request-d","cid-a","user-d","assistant-d",77],
  ]){
    const call=byRequest[requestId];
    if(!call || call.turn!==turn || call.user!==user || call.assistant!==assistant) throw new Error("complete-turn owner mismatch: "+JSON.stringify(call));
    if(call.body.client_meta.turn_workflow_request_id!==requestId) throw new Error("workflow id mismatch: "+JSON.stringify(call.body));
    if(call.body.client_meta.captured_host_context.hostChatId!==cid || call.body.client_meta.captured_orchestration._chatSessionId!==sid) {
      throw new Error("captured Char/CID/session mismatch: "+JSON.stringify(call.body));
    }
  }
})().catch(err=>{console.error(err && err.stack || err);process.exit(1);});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("registered afterRequest persistence fixture failed: %v\n%s", err, output)
	}
}

// This regression loads the production plugin as one program and invokes the
// callbacks that it actually registers with the RisuAI API.  Only the host,
// DOM/storage, and HTTP boundaries are substituted.  The substitutes record
// every call and reject routes or coordinates outside the fixture so copied
// test-only lifecycle logic cannot make the assertion pass.

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
