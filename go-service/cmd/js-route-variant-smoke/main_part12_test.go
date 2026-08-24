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
	if !strings.Contains(afterRequestSource, `ensureActiveChatCompletedTurnsBackfilled(chatSessionId, { reason: "after_request_user_input_missing" })`) {
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
		!strings.Contains(src, `const BUILD_ID = "4.0.0"`) ||
		!strings.Contains(src, `const BUILD_CHANNEL = "release"`) {
		t.Fatal("4.0.0 release build identity is not visible in the HUD")
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
` + backfill + "\n" + ensure + `
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
` + builder + "\n" + ensure + `
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

func TestOfficialAfterRequestFinalDoesNotRequireOptionalOrchestrationPendingState(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for afterRequest pending-state independence fixture")
		}
	}
	src := readArchiveCenterJS(t)
	acceptFinal := extractJSFunctionBlockForTest(t, src, "function acceptRisuAfterRequestFinal(sessionId, type, pendingContext, requestContext, assistantContent)")
	script := `
let lastOrchResult = null;
function normalizeAssistantPersistenceCandidate(value){ return String(value || "").trim(); }
function computeOrchestrationDirtyHashOr1c(value){ return "hash:" + String(value || ""); }
` + acceptFinal + `
const context = {
  state:"captured",
  requestId:"request-11",
  sessionId:"session-1",
  requestType:"model",
  hostChatId:"host-chat-1",
  requestMessageCount:21,
  userMessageIndex:20,
  userObservedContent:"current user input",
  userObservedContentHash:"hash:current user input",
};
const accepted = acceptRisuAfterRequestFinal("session-1", "model", null, context, "visible final response");
if (!accepted.accepted || accepted.reason !== "after_request_final_accepted" || context.state !== "accepted") {
  throw new Error("official afterRequest final still depends on optional pending state: " + JSON.stringify(accepted));
}
const mismatchContext = Object.assign({}, context, {state:"captured", acceptedObservation:null, acceptedObservationKey:""});
const rejected = acceptRisuAfterRequestFinal(
  "session-1",
  "model",
  {requestId:"different-request", orchResult:null},
  mismatchContext,
  "another response"
);
if (rejected.reason !== "after_request_correlation_mismatch" || mismatchContext.state !== "terminal") {
  throw new Error("present mismatched pending state was not rejected: " + JSON.stringify(rejected));
}
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("afterRequest pending-state independence fixture failed: %v\n%s", err, out)
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
	observe := extractJSFunctionBlockForTest(t, src, "function observePendingFinalConfirmationAtHostSignal(sessionId, signalSource)")
	script := `
const _finalConfirmationRequestBySession = new Map();
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
	cmd := exec.Command(nodePath, "-e", script)
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
const result=buildPostOutputSecondaryRequestContext(messages);
if (!result || result.contextMessages.length!==messages.length ||
    result.contextMessages[0].content!==messages[0].content) {
  throw new Error("post-output persistence truncated exact host context");
}
`
	cmd := exec.Command(nodePath, "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("post-output persistence context fixture failed: %v\n%s", err, out)
	}
}

func TestFeedbackOneNormalAndPostOutputRoutesStayConnected(t *testing.T) {
	src := readArchiveCenterJS(t)
	onBefore := extractArchiveCenterJSAsyncFunction(t, src, "onBeforeRequest")
	onAfter := extractArchiveCenterJSFunction(t, src, "onAfterRequest")

	for _, required := range []string{
		"buildPostOutputSecondaryRequestContext(mainRequestActiveMessages)",
		"rememberNonMainRequestSkip(orchSessionId, postOutputDecision, \"beforeRequest\")",
		"post_output_secondary_request",
	} {
		if !strings.Contains(onBefore, required) {
			t.Fatalf("beforeRequest post-output route is disconnected: missing %q", required)
		}
	}
	for _, required := range []string{
		"const sourceAcceptanceFinality = finalObservation.accepted === true",
		"persistAfterRequestContent",
		"continueAcceptedFinalPersistence(persistenceOrchResult, sourceAcceptanceFinality)",
	} {
		if !strings.Contains(onAfter, required) {
			t.Fatalf("normal afterRequest persistence route is disconnected: missing %q", required)
		}
	}
	if strings.Contains(onAfter, "after_request_final_not_accepted") {
		t.Fatal("afterRequest correlation rejection still blocks normal persistence")
	}
	for _, forbidden := range []string{"onPostprocessedRisuOutput"} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("feedback-one route retained forbidden output hook %q", forbidden)
		}
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
})().catch(err=>{ console.error(err); process.exitCode=1; });
`
	cmd := exec.Command(nodePath, "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("admin job fixture failed: %v\n%s", err, out)
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
const settings={llmRetryCount:0,pluginMainProvider:"openai",subLlmProvider:"openai"};
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
function getPluginMainTimeoutSettingMs(){ return 1000; }
function failedQueueMaxAttempts(){ return 3; }
function getRequestTimeoutSettingMs(){ return 1000; }
function getCriticTimeoutMs(){ return 1000; }
function getEmbeddingTimeoutMs(){ return 1000; }
async function bridgeFetch(path,options){ bridgeCalls++; syncedBody=options.body; return bridgeResponse; }
async function safeCall(fn){ return await fn(); }
` + syncConfig + "\n" + ensureBinding + "\n" + markDirty + "\n" + adminMeta + `
(async()=>{
  const result=await syncConfigToBackend({llmRetryCount:0,pluginMainProvider:"openai",subLlmProvider:"openai"});
  if(!result.ok || !syncedBody || syncedBody.llmRetryCount !== 0) throw new Error("runtime config lost retry=0");
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
