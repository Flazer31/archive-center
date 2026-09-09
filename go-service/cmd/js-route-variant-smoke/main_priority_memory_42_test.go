package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestArchiveCenter42PriorityMemoryAndFinalizationWiringMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	for _, marker := range []string{
		`turnFinalizationMode: "immediate_after_response"`,
		`turn_finalization_mode: settings.turnFinalizationMode || DEFAULT_SETTINGS.turnFinalizationMode`,
		`id="mo-turnFinalizationMode"`,
		`function queueNextInputFinalization(requestContext, sourceAcceptanceFinality)`,
		`function beginNextInputFinalizationPipeline(sessionId, currentRequestContext, hostContext = null)`,
		`contract_version: "source_acceptance_observation.v2"`,
		`finality_source: "risu_next_host_signal_active_chat"`,
		`position_observation: "committed_before_next_host_signal"`,
		`return backfillOneActiveChatCompletedTurn(sid, pair, {`,
		`goFinalizationPolicy.owner === "go"`,
		`goFinalizationMode === "next_user_input"`,
		`return continueAcceptedFinalPersistence(persistenceOrchResult, sourceAcceptanceFinality);`,
		`await loadNextInputFinalizationsFromStorage()`,
	} {
		if !strings.Contains(src, marker) {
			t.Fatalf("Archive Center.js missing 4.2 marker %q", marker)
		}
	}

	serializer := extractJSFunctionBlockForTest(t, src, "function serializeNextInputFinalizationMarker(marker)")
	for _, forbidden := range []string{"user_content:", "assistant_content:", "context_messages:"} {
		if strings.Contains(serializer, forbidden) {
			t.Fatalf("persistent next-input marker stores full turn content via %q", forbidden)
		}
	}

	afterRequest := extractJSFunctionBlockForTest(t, src, "function onAfterRequest(content, type)")
	delayedIndex := strings.Index(afterRequest, `goFinalizationMode === "next_user_input"`)
	immediateIndex := strings.Index(afterRequest, `return continueAcceptedFinalPersistence(persistenceOrchResult, sourceAcceptanceFinality);`)
	if delayedIndex < 0 || immediateIndex < 0 || delayedIndex >= immediateIndex {
		t.Fatal("delayed finalization must branch before the unchanged immediate 4.1 persistence call")
	}

	beforeRequest := extractJSFunctionBlockForTest(t, src, "async function onBeforeRequest(payload, type)")
	if !strings.Contains(beforeRequest, "if (!nextInputFinalizationOwnership.owned)") {
		t.Fatal("next-input exact previous-turn owner does not suppress the automatic broad backfill for that request")
	}
	if strings.Index(beforeRequest, "beginNextInputFinalizationPipeline(") > strings.Index(beforeRequest, "const yumiArchiveReadContext") {
		t.Fatal("previous-turn Critic was not launched before current-request preparation")
	}
}

func TestArchiveCenter42PrimarySettingsPlaceRecentConversationAndLifecycleControls(t *testing.T) {
	src := readArchiveCenterJS(t)
	embedding := strings.Index(src, `id="mo-embeddingTimeout"`)
	memoryTransport := strings.Index(src, `id="mo-memoryTransportMode"`)
	turnFinalization := strings.Index(src, `id="mo-turnFinalizationMode"`)
	advanced := strings.Index(src, `<!-- ▸ 고급 설정`)
	if embedding < 0 || memoryTransport < 0 || turnFinalization < 0 || advanced < 0 ||
		!(embedding < memoryTransport && memoryTransport < turnFinalization && turnFinalization < advanced) {
		t.Fatal("memory transport and save finalization controls must appear below embedding timeout and outside advanced settings")
	}

	topK := strings.Index(src, `id="mo-topK"`)
	coreMemory := strings.Index(src, `id="mo-coreObjectiveMemoryMaxItems"`)
	recentConversation := strings.Index(src, `id="mo-recentConversationReferenceCount"`)
	llmRetry := strings.Index(src, `id="mo-llmRetryCount"`)
	if topK < 0 || coreMemory < 0 || recentConversation < 0 || llmRetry < 0 ||
		!(topK < coreMemory && coreMemory < recentConversation && recentConversation < llmRetry) {
		t.Fatal("recent conversation reference count must follow the Chroma and core-memory controls in primary settings")
	}
}

func TestArchiveCenter42AfterRequestLetsGoResolveRawOnlyAndCompletedReplays(t *testing.T) {
	src := readArchiveCenterJS(t)
	afterRequest := extractJSFunctionBlockForTest(t, src, "function onAfterRequest(content, type)")
	if strings.Contains(afterRequest, `findRecentPersistedCompleteTurnPairForContent(`) ||
		strings.Contains(afterRequest, `skipped_duplicate_complete_turn`) {
		t.Fatal("afterRequest must not treat an existing raw pair as a completed turn before Go checks derived artifacts")
	}
	reserveIndex := strings.Index(afterRequest, `turnIdx = await reserveAfterRequestPersistenceTurnIndex(`)
	completeIndex := strings.Index(afterRequest, `() => tryCompleteTurn(turnIdx`)
	if reserveIndex < 0 || completeIndex < 0 || reserveIndex >= completeIndex {
		t.Fatal("afterRequest no longer routes the accepted pair through Go-owned turn resolution and /complete-turn")
	}
}

func TestArchiveCenter42OrchestrationCarriesGoFinalizationPolicyToAfterRequest(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Fatalf("node is required for the 4.2 finalization-policy handoff fixture: %v", err)
		}
	}
	src := readArchiveCenterJS(t)
	orchestrate := extractArchiveCenterJSAsyncFunction(t, src, "orchestrateTurnHelpers")
	script := `
const settings={enabled:true};
function debugLog(){}
function newTurnTrace(){return {providerCallBudgetLedgers:{}};}
function truncPreview(value){return String(value||"");}
function normalizeResponseExecutionContractTrace(value){return value||null;}
function applyOrchestrationModuleTransportTraceOr1e(){}
function buildOrchestrationModuleTransportStateOr1e(){return {};}
const _lastPrepareTurnSource="backend-shadow";
async function getCurrentChatSessionId(){return "fallback-session";}
function peekNextTurnIndex(){return 2;}
function buildInputTransparency(){return {};}
let _effectiveInputAwaitingNewTurn=true;
let _lastActivitySnapshot=null;
` + orchestrate + `
(async function(){
  const policy={contract_version:"turn_finalization_policy.v1",owner:"go",mode:"next_user_input"};
  const result=await orchestrateTurnHelpers("next input",[],null,{
    payloadApplicationPlan:{contract_version:"payload_application_plan.v1",owner:"go",apply_rule:"apply_exact_text_without_reassembly"},
    injectionPack:{},
    turnFinalizationPolicy:policy,
  },null,{chatSessionId:"session-1"});
  if(!result || result.turnFinalizationPolicy!==policy){
    throw new Error("Go finalization policy was dropped before afterRequest: "+JSON.stringify(result&&result.turnFinalizationPolicy));
  }
})().catch(function(err){console.error(err&&err.stack||err);process.exit(1);});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("4.2 finalization-policy handoff runtime fixture failed: %v\n%s", err, output)
	}
}

func TestArchiveCenter42NextInputCompleteTurnUsesPreviousHUDWithoutChangingImmediateHUD(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Fatalf("node is required for the 4.2 dual-HUD routing fixture: %v", err)
		}
	}
	src := readArchiveCenterJS(t)
	tryCompleteTurn := extractArchiveCenterJSAsyncFunction(t, src, "tryCompleteTurn")
	phaseProjection := extractArchiveCenterJSFunction(t, src, "projectTurnWorkflowHUDPhaseView")
	finishCurrentGeneration := extractArchiveCenterJSFunction(t, src, "finishTurnWorkflowHUDCurrentGeneration")
	dismissPrevious := extractArchiveCenterJSFunction(t, src, "dismissTurnWorkflowHUDPrevious")
	slotHTML := extractArchiveCenterJSFunction(t, src, "turnWorkflowHUDSlotHTML")
	stackPresentation := extractArchiveCenterJSFunction(t, src, "buildTurnWorkflowHUDStackPresentation")
	applyStack := extractArchiveCenterJSAsyncFunction(t, src, "applyTurnWorkflowHUDStack")
	currentIndex := strings.Index(stackPresentation, `turnWorkflowHUDSlotHTML(currentPresentation, "current", "turn_hud.slot.current", true)`)
	previousIndex := strings.Index(stackPresentation, `turnWorkflowHUDSlotHTML(previousPresentation, "previous", "turn_hud.slot.previous", true)`)
	if currentIndex < 0 || previousIndex < 0 || currentIndex >= previousIndex {
		t.Fatal("dual HUD must render current generation above previous-turn Critic/save")
	}
	script := `
const TURN_WORKFLOW_HUD_SLOT_STYLE="slot-style";
const TURN_WORKFLOW_HUD_SLOT_LABEL_STYLE="label-style";
function escapeTurnWorkflowHUDHTML(value){return String(value==null?"":value);}
function t(key){return key==="turn_hud.slot.current"?"CURRENT":(key==="turn_hud.slot.previous"?"PREVIOUS":key);}
function turnWorkflowHUDDismissButtonHTML(){return '<button data-close="1"></button>';}
function buildTurnWorkflowHUDPresentation(view){
  const stage=view&&view.current_stage||{};
  return {terminal:view&&view.terminal===true,dismissible:view&&view.dismissible===true,closeButtonOnly:view&&view.closeButtonOnly===true,recoveryAction:null,elapsedStartedAt:"",ordinal:Number(stage.ordinal||0),total:Number(stage.total||0),html:'<div style="max-height:calc(100vh - 20px)">'+turnWorkflowHUDDismissButtonHTML()+String(view.name||"")+':'+String(stage.ordinal||0)+'/'+String(stage.total||0)+'</div>'};
}
` + phaseProjection + "\n" + finishCurrentGeneration + "\n" + dismissPrevious + "\n" + slotHTML + "\n" + stackPresentation + "\n" + applyStack + `
const settings={enabled:true,dbEnabled:true};
const primaryStarts=[];
const previousStarts=[];
const primaryViews=[];
const previousViews=[];
const transportErrors=[];
const dismissBindings=[];
const queuedOperations=[];
const appliedStacks=[];
let _turnWorkflowHUDActiveRequestId="request-waiting";
let _turnWorkflowHUDLastView={request_id:"request-waiting",name:"CURRENT-CARD"};
let _turnWorkflowHUDLastRevision=6;
let _turnWorkflowHUDTerminalRequestId="";
let _turnWorkflowHUDCurrentFinalizationMode="next_user_input";
let _turnWorkflowHUDWatchToken=1;
let _turnWorkflowHUDWatchRunning=true;
let _turnWorkflowHUDPreviousRequestId="request-previous";
let _turnWorkflowHUDPreviousLastRevision=12;
let _turnWorkflowHUDPreviousLastView={request_id:"request-previous",name:"PREVIOUS-CARD",terminal:true};
let _turnWorkflowHUDPreviousWatchToken=1;
let _turnWorkflowHUDPreviousWatchRunning=false;
let _turnWorkflowHUDElapsedElement=null;
let _turnWorkflowHUDElapsedStartedAt="";
const _turnWorkflowHUDHostWarningsByRequestId=new Map();
const root={
  async setInnerHTML(html){appliedStacks.push(String(html||""));},
  async querySelector(selector){return {selector:String(selector||"")};},
};
function cancelTurnWorkflowHUDStream(){}
function cancelTurnWorkflowHUDPreviousStream(){}
function clearTurnWorkflowHUDTimer(){}
function queueTurnWorkflowHUDOperation(_label,operation){const pending=Promise.resolve().then(operation);queuedOperations.push(pending);return pending;}
async function removeTurnWorkflowHUDDismissListeners(){}
async function ensureTurnWorkflowHUDRoot(){return root;}
async function attachTurnWorkflowHUDDismiss(card,requestId,closeButtonOnly,onClick){dismissBindings.push({card,requestId,closeButtonOnly,onClick});}
async function attachTurnWorkflowHUDRecovery(){}
async function updateTurnWorkflowHUDElapsed(){}
function scheduleTurnWorkflowHUDElapsedFrame(){}
async function drainQueuedOperations(){for(let index=0;index<queuedOperations.length;index++)await queuedOperations[index];}
function turnWorkflowHUDRequestIdFromCompleteBody(body){return String(body&&body.client_meta&&body.client_meta.turn_workflow_request_id||"");}
function startTurnWorkflowHUDWatch(requestId){primaryStarts.push(String(requestId||""));}
function startTurnWorkflowHUDPreviousWatch(requestId){previousStarts.push(String(requestId||""));}
function consumeTurnWorkflowHUD(view){primaryViews.push(view);}
function consumeTurnWorkflowHUDPrevious(view){previousViews.push(view);}
function renderTurnWorkflowHUDTransportError(){transportErrors.push("current");}
function renderTurnWorkflowHUDPreviousTransportError(){transportErrors.push("previous");}
async function safeCall(call){return await call();}
async function bridgeFetchWithRetry(_path,options){
  const requestId=String(options&&options.body&&options.body.client_meta&&options.body.client_meta.turn_workflow_request_id||"");
  return {
    status:"ok",
    source_acceptance:{accepted:false,replace_existing:false,lifecycle:"candidate_or_inactive"},
    turn_workflow_hud:{contract_version:"turn_workflow_hud.v3",request_id:requestId,status:"completed",revision:12},
  };
}
function debugLog(){}
function updateRuntimeState(){}
` + tryCompleteTurn + `
(async function(){
  const stages=Array.from({length:12},function(_,index){return {key:"stage-"+(index+1),ordinal:index+1,total:12,status:"pending"};});
  const generation=projectTurnWorkflowHUDPhaseView({current_stage:stages[5],stages},"generation");
  if(generation.current_stage.ordinal!==6 || generation.current_stage.total!==6 || generation.stages.length!==6 || generation.stages[0].key!=="stage-1" || generation.stages[5].key!=="stage-6"){
    throw new Error("current generation was not projected as six stages: "+JSON.stringify(generation));
  }
  const finalization=projectTurnWorkflowHUDPhaseView({current_stage:stages[8],stages},"finalization");
  if(finalization.current_stage.ordinal!==3 || finalization.current_stage.total!==6 || finalization.stages.length!==6 || finalization.stages[0].key!=="stage-7" || finalization.stages[5].key!=="stage-12"){
    throw new Error("previous finalization was not projected as six stages: "+JSON.stringify(finalization));
  }
  const dual=buildTurnWorkflowHUDStackPresentation({name:"CURRENT-CARD",current_stage:stages[2],stages},{name:"PREVIOUS-CARD",current_stage:stages[8],stages},"next_user_input");
  if(dual.html.indexOf("CURRENT")<0 || dual.html.indexOf("PREVIOUS")<0 || dual.html.indexOf("CURRENT")>=dual.html.indexOf("PREVIOUS")){
    throw new Error("dual HUD labels were not rendered current-above-previous: "+dual.html);
  }
  if(dual.currentPresentation.ordinal!==3 || dual.currentPresentation.total!==6 || dual.previousPresentation.ordinal!==3 || dual.previousPresentation.total!==6 || dual.html.indexOf("CURRENT-CARD:3/6")<0 || dual.html.indexOf("PREVIOUS-CARD:3/6")<0){
    throw new Error("dual HUD did not render both tracks with independent 1-6 numbering: "+dual.html);
  }
  if(dual.html.indexOf('data-turn-workflow-card="current"')<0 || dual.html.indexOf('data-turn-workflow-card="previous"')<0){
    throw new Error("dual HUD did not render both request-scoped cards: "+dual.html);
  }
  if((dual.html.match(/data-close=/g)||[]).length!==1){
    throw new Error("previous card retained a dead second close button: "+dual.html);
  }
  const dualRecovering=buildTurnWorkflowHUDStackPresentation(
    {name:"CURRENT-CARD",current_stage:stages[2],stages},
    {name:"PREVIOUS-RECOVERY",current_stage:stages[8],stages,dismissible:true,closeButtonOnly:true},
    "next_user_input"
  );
  if((dualRecovering.html.match(/data-close=/g)||[]).length!==2){
    throw new Error("recovering previous card lost its requested close button: "+dualRecovering.html);
  }
  const delayedCurrentOnly=buildTurnWorkflowHUDStackPresentation({name:"CURRENT-CARD",current_stage:stages[2],stages},null,"next_user_input");
  if(delayedCurrentOnly.html.indexOf('data-turn-workflow-slot="current"')<0 || delayedCurrentOnly.currentPresentation.ordinal!==3 || delayedCurrentOnly.currentPresentation.total!==6){
    throw new Error("next-input current-only HUD did not identify the six-stage generation phase");
  }
  const immediateCurrentOnly=buildTurnWorkflowHUDStackPresentation({name:"CURRENT-CARD",current_stage:stages[2],stages},null,"immediate_after_response");
  if(immediateCurrentOnly.html.indexOf('data-turn-workflow-slot="current"')>=0 || immediateCurrentOnly.currentPresentation.ordinal!==3 || immediateCurrentOnly.currentPresentation.total!==12){
    throw new Error("immediate HUD no longer preserves the original single twelve-stage presentation");
  }
  _turnWorkflowHUDPreviousLastView={request_id:"request-previous",name:"PREVIOUS-RECOVERY",dismissible:true,closeButtonOnly:true};
  await applyTurnWorkflowHUDStack(root);
  if(!dismissBindings.find(function(binding){return binding.requestId==="request-previous" && binding.closeButtonOnly===true;})){
    throw new Error("recovering previous HUD did not bind its close button");
  }
  dismissBindings.length=0;
  _turnWorkflowHUDPreviousLastView={request_id:"request-previous",name:"PREVIOUS-CARD",terminal:true};
  await applyTurnWorkflowHUDStack(root);
  const previousDismiss=dismissBindings.find(function(binding){return binding.requestId==="request-previous";});
  if(!previousDismiss || typeof previousDismiss.onClick!=="function"){
    throw new Error("completed previous HUD was not dismissible while the current HUD was present");
  }
  await previousDismiss.onClick();
  await drainQueuedOperations();
  if(_turnWorkflowHUDPreviousRequestId!=="" || _turnWorkflowHUDPreviousLastView!==null || _turnWorkflowHUDActiveRequestId!=="request-waiting"){
    throw new Error("dismissing the previous HUD did not preserve only the current HUD");
  }
  _turnWorkflowHUDPreviousRequestId="request-previous";
  _turnWorkflowHUDPreviousLastRevision=9;
  _turnWorkflowHUDPreviousLastView={request_id:"request-previous",name:"PREVIOUS-CARD"};
  if(!finishTurnWorkflowHUDCurrentGeneration("request-waiting")){
    throw new Error("accepted delayed response did not finish the current generation HUD");
  }
  await drainQueuedOperations();
  if(_turnWorkflowHUDActiveRequestId!=="" || _turnWorkflowHUDLastView!==null || _turnWorkflowHUDPreviousRequestId!=="request-previous" || _turnWorkflowHUDPreviousLastView===null){
    throw new Error("finishing current generation did not leave only the previous Critic HUD");
  }
  await tryCompleteTurn(10,"user-a","assistant-a",[],"session-1",null,{
    client_meta:{turn_workflow_request_id:"request-a",turn_finalization_mode:"next_user_input"},
  });
  if(previousStarts.join(",")!=="request-a" || primaryStarts.length!==0){
    throw new Error("next-input complete-turn did not use only the previous-turn HUD: "+JSON.stringify({primaryStarts,previousStarts}));
  }
  if(previousViews.length!==1 || primaryViews.length!==0){
    throw new Error("next-input terminal ViewModel did not stay on the previous-turn HUD");
  }
  await tryCompleteTurn(11,"user-b","assistant-b",[],"session-1",null,{
    client_meta:{turn_workflow_request_id:"request-b",turn_finalization_mode:"immediate_after_response"},
  });
  if(primaryStarts.join(",")!=="request-b" || previousStarts.join(",")!=="request-a"){
    throw new Error("immediate mode no longer uses the unchanged primary HUD: "+JSON.stringify({primaryStarts,previousStarts}));
  }
  if(primaryViews.length!==1 || previousViews.length!==1 || transportErrors.length!==0){
    throw new Error("dual-HUD completion routing produced an unexpected result");
  }
})().catch(function(err){console.error(err&&err.stack||err);process.exit(1);});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("4.2 dual-HUD routing runtime fixture failed: %v\n%s", err, output)
	}
}

func TestArchiveCenter42PreviousHUDStreamsTheSameRequestScopedWorkflow(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Fatalf("node is required for the 4.2 previous-HUD stream fixture: %v", err)
		}
	}
	src := readArchiveCenterJS(t)
	functions := strings.Join([]string{
		extractArchiveCenterJSFunction(t, src, "cancelTurnWorkflowHUDPreviousStream"),
		extractArchiveCenterJSFunction(t, src, "turnWorkflowHUDStreamFailure"),
		extractArchiveCenterJSAsyncFunction(t, src, "openTurnWorkflowHUDStream"),
		extractArchiveCenterJSAsyncFunction(t, src, "consumeTurnWorkflowHUDPreviousStreamLine"),
		extractArchiveCenterJSAsyncFunction(t, src, "consumeTurnWorkflowHUDPreviousStream"),
		extractArchiveCenterJSFunction(t, src, "startTurnWorkflowHUDPreviousWatch"),
	}, "\n")
	for _, forbidden := range []string{"/turn-workflow/status", "setInterval(", "setTimeout("} {
		if strings.Contains(functions, forbidden) {
			t.Fatalf("previous-turn HUD added forbidden polling transport %q", forbidden)
		}
	}
	script := `
const settings={turnWorkflowHUDEnabled:true,webDirectBridgeEnabled:false,bridgeUrl:"http://127.0.0.1:28080"};
const encoder=new TextEncoder();
let _turnWorkflowHUDPreviousRequestId="";
let _turnWorkflowHUDPreviousWatchToken=0;
let _turnWorkflowHUDPreviousWatchRunning=false;
let _turnWorkflowHUDPreviousLastRevision=0;
let _turnWorkflowHUDPreviousLastView=null;
let _turnWorkflowHUDPreviousStreamAbortController=null;
let _turnWorkflowHUDPreviousStreamReader=null;
let _turnWorkflowHUDRenderChain=Promise.resolve();
const received=[];
const paths=[];
let response;
const R={nativeFetch:async function(url){paths.push(String(url||""));return response;}};
function turnWorkflowHUDIsEnabled(){return true;}
function dismissTurnWorkflowHUD(){throw new Error("previous HUD unexpectedly dismissed the current HUD");}
function debugLog(){}
function resolveBridgeRuntimeRoute(raw){return {url:String(raw||"")};}
function consumeTurnWorkflowHUDPrevious(view){
  received.push(String(view&&view.status||""));
  _turnWorkflowHUDPreviousLastRevision=Math.max(_turnWorkflowHUDPreviousLastRevision,Number(view&&view.revision||0));
  _turnWorkflowHUDPreviousLastView=view;
  _turnWorkflowHUDRenderChain=Promise.resolve();
  return true;
}
function responseFromLines(lines){
  const chunks=[encoder.encode(lines.join("\n")+"\n")];
  return {ok:true,status:200,body:{getReader:function(){return {
    read:async function(){return chunks.length?{value:chunks.shift(),done:false}:{done:true};},
    cancel:async function(){},
  };}}};
}
` + functions + `
(async function(){
  response=responseFromLines([
    JSON.stringify({contract_version:"turn_workflow_hud.v3",request_id:"request-a",status:"running",revision:7}),
    JSON.stringify({contract_version:"turn_workflow_hud.v3",request_id:"request-a",status:"completed",revision:12}),
  ]);
  startTurnWorkflowHUDPreviousWatch("request-a");
  for(let i=0;i<20&&_turnWorkflowHUDPreviousWatchRunning;i++) await new Promise(function(resolve){setImmediate(resolve);});
  if(_turnWorkflowHUDPreviousWatchRunning) throw new Error("previous-turn HUD stream did not settle");
  if(paths.length!==1 || !paths[0].includes("request_id=request-a") || !paths[0].includes("after_revision=0")){
    throw new Error("previous-turn HUD did not open exactly one request-scoped event stream: "+JSON.stringify(paths));
  }
  if(received.join(",")!=="running,completed"){
    throw new Error("previous-turn HUD did not render sequential backend revisions: "+received.join(","));
  }
})().catch(function(err){console.error(err&&err.stack||err);process.exit(1);});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("4.2 previous-HUD stream runtime fixture failed: %v\n%s", err, output)
	}
}

func TestArchiveCenter42NextInputFinalizationUsesPreviousStableRowWithoutBlocking(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Fatalf("node is required for the 4.2 next-input finalization runtime fixture: %v", err)
		}
	}
	src := readArchiveCenterJS(t)
	functions := strings.Join([]string{
		extractArchiveCenterJSFunction(t, src, "serializeNextInputFinalizationMarker"),
		extractArchiveCenterJSAsyncFunction(t, src, "saveNextInputFinalizationsToStorage"),
		extractArchiveCenterJSAsyncFunction(t, src, "loadNextInputFinalizationsFromStorage"),
		extractArchiveCenterJSFunction(t, src, "queueNextInputFinalization"),
		extractArchiveCenterJSAsyncFunction(t, src, "buildNextInputSourceAcceptanceFinality"),
		extractArchiveCenterJSFunction(t, src, "buildCompletedTurnPairsFromActiveChatMessages"),
		extractArchiveCenterJSFunction(t, src, "beginNextInputFinalizationPipeline"),
	}, "\n")
	script := functions + `
const NEXT_INPUT_FINALIZATION_STORAGE_KEY="next-input-test";
const _nextInputFinalizations=new Map();
let _nextInputFinalizationLoaded=false;
let stored="";
let activeChat=null;
let backfillCalls=0;
let releaseBackfill=null;
let lastFinality=null;
const runtimeUpdates=[];
async function persistentSet(_key,value){stored=String(value||"");}
async function persistentGet(){return stored;}
function warnLog(...args){throw new Error("unexpected warning: "+args.join(" "));}
function debugLog(){}
function updateRuntimeState(name,status,value){runtimeUpdates.push({name,status,value});}
function normalizeAssistantPersistenceCandidate(value){return String(value||"").trim();}
function shouldSkipUserInputPersistence(){return false;}
function getActiveChatMessageStreamingState(){return "not_streaming";}
const AUTO_CONTINUE_USER_INPUT_MARKER="[Continue]";
function computeOrchestrationDirtyHashOr1c(value){
  let hash=5381; const text=String(value||"");
  for(let i=0;i<text.length;i++) hash=((hash*33)^text.charCodeAt(i))>>>0;
  return "or1c:"+hash.toString(16);
}
function extractComparableMessageRoleAndContent(message){
  if(!message) return null;
  const role=message.role==="char" ? "assistant" : message.role;
  return {role,content:String(message.data!=null?message.data:message.content||"")};
}
function extractActiveChatComparableMessages(chat){
  return (chat && Array.isArray(chat.message)?chat.message:[]).map((message,index)=>{
    const comparable=extractComparableMessageRoleAndContent(message);
    return comparable ? {...comparable,risuMessageIndex:index} : null;
  }).filter(Boolean);
}
async function resolveCurrentActiveChatObject(){return {chat:activeChat};}
async function buildCompleteTurnSourceAcceptanceObservation(){
  const assistant=activeChat.message[1];
  const hash=computeOrchestrationDirtyHashOr1c(assistant.data);
  return {
    message_chat_id:String(assistant.chatId||""), message_chat_id_state:assistant.chatId?"observed":"unobserved",
    generation_id:"generation-b", generation_id_state:"observed",
    message_swipe_id:-1, message_swipe_id_state:"not_present",
    message_time_ms:2000, message_time_state:"observed",
    branch_id:"", branch_id_state:"not_exposed_by_risuai",
    observed_content_hash:hash, persistence_content_hash:hash,
  };
}
async function backfillOneActiveChatCompletedTurn(_sid,pair,options){
  backfillCalls++;
  lastFinality=options.sourceAcceptanceFinality;
  if(pair.userContent!=="same input" || pair.assistantContent!=="final B") throw new Error("wrong previous pair: "+JSON.stringify(pair));
  return await new Promise(resolve=>{releaseBackfill=resolve;});
}
async function flush(){for(let i=0;i<8;i++) await Promise.resolve();}
(async function(){
  const firstContext={sessionId:"session-1",requestId:"request-a",requestType:"model",hostChatId:"chat-1",requestMessageCount:1,userMessageIndex:0,userObservedPairOrdinal:1,userMessageChatId:"user-row-1",userMessageTimeMs:1000,userObservedContentHash:computeOrchestrationDirtyHashOr1c("same input")};
  if(!queueNextInputFinalization(firstContext,{persistence_content_hash:computeOrchestrationDirtyHashOr1c("first A")})) throw new Error("first marker was not queued");
  const sameRow=beginNextInputFinalizationPipeline("session-1",{userMessageIndex:0},{});
  if(!sameRow.owned || sameRow.started || sameRow.reason!=="same_user_row_reroll_or_edit") throw new Error("same-row reroll was finalized: "+JSON.stringify(sameRow));
  if(backfillCalls!==0) throw new Error("same-row reroll reached persistence");

  const childRow=beginNextInputFinalizationPipeline("child-session",{userMessageIndex:2},{hostChatId:"child-chat"});
  if(childRow.owned || childRow.started || childRow.reason!=="no_pending_previous_turn") throw new Error("child input consumed parent's marker: "+JSON.stringify(childRow));
  await flush();
  if(backfillCalls!==0 || !_nextInputFinalizations.has("session-1")) throw new Error("parent pending marker changed after child input");

  const rerolledContext={...firstContext,requestId:"request-b"};
  if(!queueNextInputFinalization(rerolledContext,{persistence_content_hash:computeOrchestrationDirtyHashOr1c("final B")})) throw new Error("rerolled marker was not queued");
  if(_nextInputFinalizations.get("session-1").requestId!=="request-b") throw new Error("reroll did not replace pending marker");
  await flush();
  if(stored.includes("same input") || stored.includes("final B")) throw new Error("persistent marker leaked full turn text: "+stored);

  activeChat={id:"chat-1",message:[
    {role:"user",data:"same input",chatId:"user-row-1",time:1000},
    {role:"char",data:"final B",chatId:"assistant-row-1",time:2000},
    {role:"user",data:"same input",chatId:"user-row-2",time:3000},
  ]};
  const nextRow=beginNextInputFinalizationPipeline("session-1",{userMessageIndex:2},{});
  if(!nextRow.owned || !nextRow.started) throw new Error("identical-text new row did not start previous finalization: "+JSON.stringify(nextRow));
  if(backfillCalls!==0) throw new Error("previous Critic blocked the synchronous current path");
  await flush();
  if(backfillCalls!==1 || typeof releaseBackfill!=="function") throw new Error("previous pair was not started asynchronously");
  if(!lastFinality || lastFinality.finality_source!=="risu_next_host_signal_active_chat" || lastFinality.next_signal_user_index!==2 || lastFinality.user_message_chat_id!=="user-row-1") {
    throw new Error("next-host finality lost the stable previous row: "+JSON.stringify(lastFinality));
  }
  releaseBackfill({status:"saved",turnIndex:1});
  await flush();
  if(_nextInputFinalizations.has("session-1")) throw new Error("saved previous marker was not consumed");

  queueNextInputFinalization(rerolledContext,{persistence_content_hash:computeOrchestrationDirtyHashOr1c("final B")});
  await flush();
  _nextInputFinalizations.clear(); _nextInputFinalizationLoaded=false;
  if(await loadNextInputFinalizationsFromStorage()!==1 || !_nextInputFinalizations.has("session-1")) throw new Error("restart did not restore pending marker");
  activeChat={...activeChat,id:"different-branch"};
  const branchAttempt=beginNextInputFinalizationPipeline("session-1",{userMessageIndex:2},{});
  if(!branchAttempt.started) throw new Error("branch check did not remain non-blocking");
  await flush();
  if(backfillCalls!==1 || !_nextInputFinalizations.has("session-1")) throw new Error("different branch consumed or saved the pending marker");
})().catch(function(err){console.error(err && err.stack || err);process.exit(1);});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("4.2 next-input finalization runtime fixture failed: %v\n%s", err, output)
	}
}
