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
` + observe + `
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
const ACTIVE_CHAT_BACKFILL_MAX_PAIRS = 20;
const settings = {enabled:true,dbEnabled:true};
const _activeChatBackfillInFlight = new Set();
let completeTurnCalls = 0;
let builtOptions = null;
let postedBody = null;
async function requestBackendSessionRoutingTurnResolution(){
  return {status:"resolved",turnIndex:2,localTurnIndex:2,baseline:null};
}
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
    {userContent:"older user",assistantContent:"older answer",risuUserMessageIndex:0,risuAssistantMessageIndex:1},
    {userContent:"edited user",assistantContent:"edited answer",risuUserMessageIndex:2,risuAssistantMessageIndex:3},
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
  const original = backfillOneActiveChatCompletedTurn;
  backfillOneActiveChatCompletedTurn = async function(sid,observedPair,options){
    flags.push(options.hostObservedActiveTailReplacement === true);
    return {status:"exists",turnIndex:flags.length};
  };
  await ensureActiveChatCompletedTurnsBackfilled("session-1",{reason:"before_request",maxPairs:2});
  backfillOneActiveChatCompletedTurn = original;
  if (flags.length !== 2 || flags[0] || !flags[1]) {
    throw new Error("only latest official completed pair must be replacement-eligible: "+JSON.stringify(flags));
  }
})().catch(err=>{ console.error(err); process.exitCode=1; });
`
	cmd := exec.Command(nodePath, "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("active-tail edit fixture failed: %v\n%s", err, out)
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
const STARTUP_MESSAGE_MAX_CHARS = 120000;
const ACTIVE_CHAT_BACKFILL_MAX_CONTEXT_MESSAGES = 20;
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
	start := extractJSFunctionBlockForTest(t, src, "function startTurnWorkflowHUDWatch(requestId)")
	script := `
const settings = {bridgeUrl:"http://bridge"};
let _turnWorkflowHUDWatchToken = 0;
let _turnWorkflowHUDActiveRequestId = "";
let _turnWorkflowHUDWatchRunning = false;
let _turnWorkflowHUDLastRevision = 0;
let _turnWorkflowHUDStreamAbortController = null;
let _turnWorkflowHUDStreamReader = null;
let _turnWorkflowHUDRenderChain = Promise.resolve();
const opened = [];
let transportError = "";
function turnWorkflowHUDIsEnabled(){ return true; }
function dismissTurnWorkflowHUD(){}
function cancelTurnWorkflowHUDStream(){
  if (_turnWorkflowHUDStreamAbortController) _turnWorkflowHUDStreamAbortController.abort();
  _turnWorkflowHUDStreamAbortController = null;
  _turnWorkflowHUDStreamReader = null;
}
function clearTurnWorkflowHUDTimer(){}
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
` + line + "\n" + consume + "\n" + start + `
(async()=>{
  startTurnWorkflowHUDWatch("req-1");
  for(let i=0;i<50 && _turnWorkflowHUDWatchRunning;i++) await new Promise(resolve=>setTimeout(resolve,1));
  if (transportError !== "hud_transport_nonterminal_eof") throw new Error("missing typed EOF transport error: "+transportError);
  if (opened.length !== 1) throw new Error("nonterminal EOF caused hidden reconnect count="+opened.length);
  if (_turnWorkflowHUDLastRevision !== 1) throw new Error("running revision was not consumed");
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
	render := extractJSFunctionBlockForTest(t, src, "function renderAdminJobProgressHtml(job, label, kind)")
	applySnapshot := extractJSFunctionBlockForTest(t, src, "function applyAdminBackgroundJobSnapshot(kind, state, jobId, snapshot)")
	cancel := extractJSFunctionBlockForTest(t, src, "async function cancelAdminBackgroundJob(kind, state, jobId)")
	script := `
let requestPath = "";
let requestMethod = "";
const _adminBackgroundJobStreams = new Map();
function escapeAttr(value){ return String(value==null?"":value); }
function refreshExplorerUI(){}
function cancelAdminBackgroundJobStream(){ return true; }
function markAdminBackgroundJobStreamUnavailable(){ throw new Error("unexpected cancel transport failure"); }
function getRequestTimeoutSettingMs(){ return 1000; }
async function bridgeFetch(path,options){
  requestPath=path; requestMethod=options.method;
  return {job_id:"job-1",status:"cancelled",terminal:true,progress:{stage:"cancelled"}};
}
async function safeCall(fn){ return await fn(); }
` + value + "\n" + render + "\n" + applySnapshot + "\n" + cancel + `
(async()=>{
  const html = renderAdminJobProgressHtml({
    job_id:"job-1",
    status:"running",
    terminal:false,
    request:{repair_entry_count:99},
    progress:{progress_percent:8,processed:0,display_total:3,candidate_count:3},
  },"Normalize","session_normalize");
  if (!html.includes("8% (0/3)")) throw new Error("cold-start total did not render backend progress ViewModel: "+html);
  const laterStage = renderAdminJobProgressHtml({
    job_id:"job-1",status:"running",terminal:false,request:{repair_entry_count:99},
    progress:{stage:"inspect_after",progress_percent:90,processed:0,display_total:0},
  },"Normalize","session_normalize");
  if (!laterStage.includes("90% (0/0)") || laterStage.includes("0/99")) {
    throw new Error("request raw-repair count leaked into another stage: "+laterStage);
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
	adminMeta := extractArchiveCenterJSSyncFunction(t, src, "buildAdminRuntimeClientMeta")
	script := `
const DEFAULT_SETTINGS={llmRetryCount:3,embeddingProvider:"openai",episodeIntervalTurns:8};
const settings={llmRetryCount:0,pluginMainProvider:"openai",subLlmProvider:"openai"};
let syncedBody=null;
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
async function bridgeFetch(path,options){ syncedBody=options.body; return {status:"ok"}; }
async function safeCall(fn){ return await fn(); }
` + syncConfig + "\n" + adminMeta + `
(async()=>{
  const result=await syncConfigToBackend({llmRetryCount:0,pluginMainProvider:"openai",subLlmProvider:"openai"});
  if(!result.ok || !syncedBody || syncedBody.llmRetryCount !== 0) throw new Error("runtime config lost retry=0");
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
	data, err := os.ReadFile(filepath.Join(archiveCenterRoot(t), "Risu Recomposer.js"))
	if err != nil {
		t.Fatalf("read Risu Recomposer.js: %v", err)
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
