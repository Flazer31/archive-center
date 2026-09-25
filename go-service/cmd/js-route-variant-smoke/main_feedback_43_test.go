package main

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

func TestFeedback43HUDListenersUseRegisteringSafeElement(t *testing.T) {
	src := readArchiveCenterJS(t)
	functions := strings.Join([]string{
		extractArchiveCenterJSFunction(t, src, "takeTurnWorkflowHUDDismissListenerIds"),
		extractArchiveCenterJSAsyncFunction(t, src, "removeTurnWorkflowHUDDismissListeners"),
		extractArchiveCenterJSAsyncFunction(t, src, "attachTurnWorkflowHUDDismiss"),
	}, "\n")
	script := `
const assert = require('node:assert/strict');
const R = {}; // PocketRisu v1.11.2 has no global removeRisuEventListener.
let _turnWorkflowHUDDismissListenerIds = [];
let _turnWorkflowHUDUnloaded = false;
const handlers = new Map();
let serial = 0, rectReads = 0, dismissals = 0;
const errors = [];
const debugLog = (...args) => errors.push(args);
async function dismissTurnWorkflowHUD() { dismissals++; }
function target() {
  const own = new Map();
  return {
    async addEventListener(type, callback) {
      const id = ++serial;
      own.set(id, {type, callback}); handlers.set(id, callback);
      return id;
    },
    async removeEventListener(type, id) {
      assert.equal(own.get(id)?.type, type, 'cleanup used another SafeElement');
      own.delete(id); handlers.delete(id);
    },
    async getBoundingClientRect() { rectReads++; return {left:0,top:0,right:10,bottom:10}; },
    async querySelectorAll(selector) { assert.equal(selector, 'details[open]'); return {length:async()=>0}; }
  };
}
` + functions + `
(async () => {
  for (let i=0; i<100; i++) {
    await removeTurnWorkflowHUDDismissListeners();
    await attachTurnWorkflowHUDDismiss(target(), 'request', false);
    assert.equal(handlers.size, 1, 'terminal HUD replacement leaked a listener');
  }
  await Promise.all([...handlers.values()].map(fn => fn({clientX:100,clientY:100})));
  assert.equal(rectReads, 1, 'outside click queried removed HUD targets');
  await Promise.all([...handlers.values()].map(fn => fn({clientX:5,clientY:5})));
  assert.equal(dismissals, 1);
  await removeTurnWorkflowHUDDismissListeners(takeTurnWorkflowHUDDismissListenerIds());
  assert.equal(handlers.size, 0);
  const late = target();
  const register = late.addEventListener;
  late.addEventListener = async (...args) => {
    const id = await register(...args); _turnWorkflowHUDUnloaded = true; return id;
  };
  await attachTurnWorkflowHUDDismiss(late, 'late', false);
  assert.equal(handlers.size, 0, 'unload during registration leaked its listener');
  assert.equal(errors.length, 0);
  _turnWorkflowHUDUnloaded = false;
  const retry = target();
  const remove = retry.removeEventListener;
  let failOnce = true;
  retry.removeEventListener = async (...args) => {
    if (failOnce) { failOnce = false; throw new Error('transient host error'); }
    return remove(...args);
  };
  await attachTurnWorkflowHUDDismiss(retry, 'retry', false);
  await removeTurnWorkflowHUDDismissListeners();
  assert.equal(handlers.size, 1);
  assert.equal(_turnWorkflowHUDDismissListenerIds.length, 1, 'failed cleanup lost the registering owner');
  assert.equal(errors.length, 1);
  await removeTurnWorkflowHUDDismissListeners();
  assert.equal(handlers.size, 0, 'next existing cleanup did not remove the retained listener');
  assert.equal(_turnWorkflowHUDDismissListenerIds.length, 0);
})().catch(err => { console.error(err); process.exitCode=1; });
`
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		nodePath = "node"
	}
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("production HUD fixture: %v\n%s", err, output)
	}
}

func TestHUDSuccessfulCardsDismissOnHostClick(t *testing.T) {
	src := readArchiveCenterJS(t)
	var production []string
	for _, name := range []string{"escapeTurnWorkflowHUDHTML", "turnWorkflowHUDDismissButtonHTML", "turnWorkflowHUDStageStatus", "turnWorkflowHUDStageStatusColor", "turnWorkflowHUDStageDuration", "turnWorkflowHUDStageReason", "turnWorkflowHUDStageLedgerHTML", "turnWorkflowHUDCountPresentation", "turnWorkflowHUDCountLedgerHTML", "turnWorkflowHUDTimingHTML", "projectTurnWorkflowHUDPhaseView", "buildTurnWorkflowHUDPresentation", "turnWorkflowHUDSlotHTML", "buildTurnWorkflowHUDStackPresentation", "turnWorkflowHUDRecoveryPresentation", "turnWorkflowHUDErrorSummaryHTML", "turnWorkflowHUDTurnLabel", "turnWorkflowHUDCloseButtonOnly", "turnWorkflowHUDSeverityStyle", "turnWorkflowHUDWarningListHTML", "takeTurnWorkflowHUDDismissListenerIds", "dismissTurnWorkflowHUD", "dismissTurnWorkflowHUDPrevious"} {
		production = append(production, extractArchiveCenterJSFunction(t, src, name))
	}
	for _, name := range []string{"removeTurnWorkflowHUDDismissListeners", "attachTurnWorkflowHUDDismiss", "applyTurnWorkflowHUDStack"} {
		production = append(production, extractArchiveCenterJSAsyncFunction(t, src, name))
	}
	production = append(production, regexp.MustCompile(`(?m)^  const TURN_WORKFLOW_HUD_[A-Z_]+_STYLE = [^\r\n]+`).FindAllString(src, -1)...)
	for _, name := range []string{"startTurnWorkflowHUDPreviousWatch", "renderTurnWorkflowHUDPrevious", "consumeTurnWorkflowHUDPrevious", "turnWorkflowHUDStreamFailure"} {
		production = append(production, extractArchiveCenterJSFunction(t, src, name))
	}
	for _, name := range []string{"consumeTurnWorkflowHUDPreviousStreamLine", "consumeTurnWorkflowHUDPreviousStream"} {
		production = append(production, extractArchiveCenterJSAsyncFunction(t, src, name))
	}
	production = append(production, extractTurnWorkflowHUDStreamIO(t, src))
	script := strings.Join(production, "\n") + `
const assert=require('node:assert/strict');
const t=key=>key,tf=(key,args)=>key+JSON.stringify(args),BUILD_ID='fixture';
const TURN_WORKFLOW_HUD_SURFACE_SELECTOR='.surface';
const TURN_WORKFLOW_HUD_CONTRACT='turn_workflow_hud.v3';
let settings={turnWorkflowHUDMode:'normal'};
let _turnWorkflowHUDPreviousStreamAbortController=null,_turnWorkflowHUDPreviousStreamReader=null;
let _turnWorkflowHUDRenderChain=Promise.resolve(),streamController,streamOpens=0;
function turnWorkflowHUDIsEnabled(){return true;}
function resolveBridgeRuntimeRoute(){return {url:'http://fixture.invalid'};}
async function openTurnWorkflowHUDStream(){
 streamOpens++;
 return new ReadableStream({start(controller){streamController=controller;}}).getReader();
}
async function sendPrevious(view){
 streamController.enqueue(new TextEncoder().encode(JSON.stringify(view)+'\n'));
 for(let i=0;i<10;i++){await new Promise(setImmediate);await flush();}
}
let _turnWorkflowHUDUnloaded=false,_turnWorkflowHUDDismissListenerIds=[];
let _turnWorkflowHUDActiveRequestId='',_turnWorkflowHUDPreviousRequestId='';
let _turnWorkflowHUDLastView=null,_turnWorkflowHUDPreviousLastView=null;
let _turnWorkflowHUDWatchToken=0,_turnWorkflowHUDWatchRunning=false,_turnWorkflowHUDLastRevision=0,_turnWorkflowHUDTerminalRequestId='';
let _turnWorkflowHUDPreviousWatchToken=0,_turnWorkflowHUDPreviousWatchRunning=false,_turnWorkflowHUDPreviousLastRevision=0;
let _turnWorkflowHUDCurrentFinalizationMode='next_user_input',_turnWorkflowHUDElapsedElement=null;
const _turnWorkflowHUDHostWarningsByRequestId=new Map();
const handlers=new Map(),operations=[];let serial=0,cancelCurrent=0,cancelPrevious=0;
function cancelTurnWorkflowHUDStream(){cancelCurrent++;}
function cancelTurnWorkflowHUDPreviousStream(){cancelPrevious++;}
function clearTurnWorkflowHUDTimer(){_turnWorkflowHUDElapsedElement=null;}
async function updateTurnWorkflowHUDElapsed(){throw Error('fixture has no active timers');}
function scheduleTurnWorkflowHUDElapsedFrame(){throw Error('fixture has no active timers');}
function debugLog(...args){throw Error(JSON.stringify(args));}
function queueTurnWorkflowHUDOperation(label,fn){operations.push(fn);return Promise.resolve();}
async function attachTurnWorkflowHUDRecovery(root,view,action){assert.ok(!action,'fixture requests no recovery action');}
function element(name){
 const own=new Map();
 return {name,html:'',
  async setInnerHTML(html){this.html=html;},
  async querySelector(selector){return selector==='button'?button:card;},
  async querySelectorAll(selector){assert.equal(selector,'details[open]');return {length:async()=>0};},
  async getBoundingClientRect(){return {left:100,top:100,right:200,bottom:200};},
  async addEventListener(type,fn,options){assert.equal(type,'click');const id=++serial;const entry={fn,options};own.set(id,entry);handlers.set(id,entry);return id;},
  async removeEventListener(type,id,options){assert.equal(type,'click');assert.ok(own.has(id),'wrong SafeElement owner');assert.equal(own.get(id).options,options);own.delete(id);handlers.delete(id);}
 };
}
const root=element('root'),card=element('card'),button=element('button');
async function getTurnWorkflowHUDMainDocument(){return {querySelector:async selector=>{assert.equal(selector,TURN_WORKFLOW_HUD_SURFACE_SELECTOR);return root;}};}
async function ensureTurnWorkflowHUDRoot(){return root;}
async function flush(){while(operations.length)await operations.shift()();}
function view(id,status){return {request_id:id,status,severity:['failed','recovering'].includes(status)?'error':'normal',dismissal_policy:['failed','recovering'].includes(status)?'x_only':'none',logical_turn:2,stages:[],counts:[],current_stage:{key:'complete',ordinal:12,total:12,status:'pending'},error:status==='failed'?{code:'CRITIC_PROVIDER_TIMEOUT',message_key:'turn_hud.error.critic_llm_failed'}:undefined};}
async function mount(current,previous){
 await removeTurnWorkflowHUDDismissListeners();
 _turnWorkflowHUDActiveRequestId=current?.request_id||'';_turnWorkflowHUDLastView=current;
 _turnWorkflowHUDPreviousRequestId=previous?.request_id||'';_turnWorkflowHUDPreviousLastView=previous;
 _turnWorkflowHUDCurrentFinalizationMode='next_user_input';
 cancelCurrent=cancelPrevious=0;
 await applyTurnWorkflowHUDStack(root);
}
async function outsideClick(){
 for(const entry of [...handlers.values()])await entry.fn({clientX:900,clientY:900,preventDefault(){throw Error('click was consumed');},stopPropagation(){throw Error('Host action was blocked');}});
 await flush();
}
(async()=>{
 for(const mode of ['normal','compact']){
  settings.turnWorkflowHUDMode=mode;
  for(const otherStatus of ['running','failed','recovering']){
   const previous=view('previous',otherStatus);
   await mount(view('current','completed'),previous);
   assert.equal([...handlers.values()].filter(x=>x.options===true).length,1,'one capture listener per stack');
   await outsideClick();
   assert.equal(_turnWorkflowHUDActiveRequestId,'');assert.equal(_turnWorkflowHUDPreviousLastView,previous);
   assert.equal(cancelPrevious,0,'successful current card cancelled previous processing');
   assert.ok(root.html.includes('previous'),'previous card disappeared');
   assert.equal([...handlers.values()].filter(x=>x.options===true).length,0,'completion listener leaked');
   const current=view('current',otherStatus);
   await mount(current,view('previous','completed'));
   await outsideClick();
   assert.equal(_turnWorkflowHUDPreviousRequestId,'');assert.equal(_turnWorkflowHUDLastView,current);
   assert.equal(cancelCurrent,0,'successful previous card cancelled current processing');
  }
  await mount(view('current','completed'),view('previous','completed'));
  assert.equal(handlers.size,1,'completed cards should share one Host listener');
  await outsideClick();assert.equal(root.html,'');assert.equal(handlers.size,0);
  await mount(view('current','completed'),null);
  const stale=[...handlers.values()][0].fn;
  await mount(view('new-current','running'),view('current','running'));
  await stale({});await flush();
  assert.equal(_turnWorkflowHUDActiveRequestId,'new-current','stale completed callback cleared the next request');
  assert.equal(_turnWorkflowHUDPreviousRequestId,'current','stale callback cleared the shifted previous request');
  await mount({...view('generation','running'),host_generation_finished:true},null);
  assert.equal(handlers.size,1);await outsideClick();assert.equal(root.html,'');
  await mount({...view('recovered','completed'),severity:'notice',display_mode:'notice'},null);
  assert.equal([...handlers.values()].filter(x=>x.options===true).length,1);await outsideClick();assert.equal(root.html,'');
  await mount(view('failed','failed'),null);await outsideClick();assert.equal(_turnWorkflowHUDActiveRequestId,'failed');
  for(const entry of [...handlers.values()])await entry.fn({clientX:150,clientY:150});
  await flush();assert.equal(_turnWorkflowHUDActiveRequestId,'','explicit X stopped working');
  const runningPrevious=view('previous-running','running');
  await mount(view('failed-current','failed'),runningPrevious);
  for(const entry of [...handlers.values()])await entry.fn({clientX:150,clientY:150});
  await flush();
  assert.equal(_turnWorkflowHUDActiveRequestId,'');
  assert.equal(_turnWorkflowHUDPreviousLastView,runningPrevious,'closing current failure also closed previous Critic');
  assert.equal(cancelPrevious,0,'current X cancelled previous Critic HUD stream');
  assert.ok(root.html.includes('previous'),'current X hid the independent previous card');
  for(let i=0;i<20;i++){await mount(view('completed-'+i,'completed'),null);assert.equal(handlers.size,1);}
  await removeTurnWorkflowHUDDismissListeners();assert.equal(handlers.size,0);

  // Start the real previous-stream owner with no response bytes yet. This is
  // the gap after /complete-turn starts, not a pre-mounted previous card.
  await mount({...view('next-current','running'),current_stage:{key:'awaiting_final_output',
    label_key:'turn_hud.stage.awaiting_final_output',status:'running',ordinal:6,total:12,llm_call:false}},null);
  startTurnWorkflowHUDPreviousWatch('pending-previous');
  await flush();
  assert.ok(root.html.includes('data-turn-workflow-card="previous"'),mode+': previous request invisible until first stream byte');
  assert.ok(root.html.includes('data-turn-workflow-card="current"'),mode+': current request disappeared');
  assert.equal(_turnWorkflowHUDPreviousLastRevision,0,'waiting card must not fabricate a backend revision');
  assert.equal(_turnWorkflowHUDPreviousLastView.current_stage.llm_call,false,'waiting card must not claim Critic was called');
  if(process.env.ARCHIVE_CENTER_PREVIOUS_HUD_PREVIEW){
   require('node:fs').writeFileSync(process.env.ARCHIVE_CENTER_PREVIOUS_HUD_PREVIEW+'.'+mode+'.html',
    '<!doctype html><meta charset="utf-8"><body style="background:#151821;color:#eee;font:13px sans-serif"><h3>'+mode+' · awaiting first previous-turn event</h3><main style="width:224px;display:flex;flex-direction:column;gap:7px">'+root.html+'</main>');
  }
  await outsideClick();
  assert.equal(_turnWorkflowHUDPreviousRequestId,'pending-previous','running previous card was dismissed');
  const opens=streamOpens;
  startTurnWorkflowHUDPreviousWatch('pending-previous');
  assert.equal(streamOpens,opens,'same request opened a duplicate stream');
  const previous={...view('pending-previous','running'),contract_version:TURN_WORKFLOW_HUD_CONTRACT,revision:9,
    current_stage:{key:'critic_llm',label_key:'turn_hud.stage.critic_llm',status:'running',ordinal:9,total:12,llm_call:true}};
  await sendPrevious(previous);
  assert.equal(_turnWorkflowHUDPreviousLastRevision,9);
  assert.ok(root.html.includes('turn_hud.stage.critic_llm'));
  await sendPrevious({...previous,status:'completed',revision:12,stages:[{...previous.current_stage,status:'succeeded',duration_ms:1200}]});
  assert.equal(_turnWorkflowHUDPreviousWatchRunning,false);
  await outsideClick();
  assert.equal(_turnWorkflowHUDPreviousRequestId,'');
  assert.equal(_turnWorkflowHUDActiveRequestId,'next-current','completed previous card dismissed current request');
  streamController.close();
 }
})().catch(err=>{console.error(err);process.exitCode=1;});
`
	node := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if node == "" {
		node = "node"
	}
	cmd := exec.Command(node, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("production HUD dismissal: %v\n%s", err, output)
	}
}

func TestFeedback43HUDRecoveryRefreshesStaleView(t *testing.T) {
	src := readArchiveCenterJS(t)
	script := `
const assert = require('node:assert/strict');
const TURN_WORKFLOW_HUD_CONTRACT = 'turn_workflow_hud.v3';
const TURN_WORKFLOW_HUD_RECOVERY_REQUEST_CONTRACT = 'turn_workflow_hud_recovery_request.v1';
let _turnWorkflowHUDActiveRequestId = 'request';
let _turnWorkflowHUDTerminalRequestId = 'request';
const _lastBridgeFailureByPath = new Map();
const t = x => x, tf = x => x;
const confirm = () => true;
const calls = [], rendered = [], dismissed = [];
const classifyTurnWorkflowHUDTransportFailure = () => ({});
const rememberTurnWorkflowHUDHostWarning = () => {};
const renderTurnWorkflowHUD = async view => rendered.push(view);
const dismissTurnWorkflowHUD = async id => dismissed.push(id);
const stopTurnWorkflowHUDWatch = () => {}, startTurnWorkflowHUDWatch = () => {};
const stale = {contract_version:TURN_WORKFLOW_HUD_CONTRACT, request_id:'request', logical_turn:3, status:'failed'};
let latest = {...stale, status:'invalidated'};
let body = {code:'recovery_action_unavailable', error:'unavailable'};
let changeRequestOnStatus = false;
async function bridgeFetch(path, options) {
  calls.push({path,options});
  if (path === '/turn-workflow/recovery') {
    _lastBridgeFailureByPath.set(path,{response_body:JSON.stringify(body)});
    throw new Error('HTTP error');
  }
  assert.equal(path, '/turn-workflow/status?request_id=request');
  if (changeRequestOnStatus) _turnWorkflowHUDActiveRequestId = 'new-request';
  return latest;
}
` + extractArchiveCenterJSAsyncFunction(t, src, "requestTurnWorkflowHUDRecovery") + `
(async () => {
  const action = {id:'retry_derived_turn'};
  await requestTurnWorkflowHUDRecovery(stale, action);
  assert.equal(calls.length, 2, 'stale recovery must refresh status once');
  assert.deepEqual(rendered, [latest], 'must not re-render the captured failed snapshot');
  calls.length=0; rendered.length=0;
  body = {...body, turn_workflow_hud:latest};
  await requestTurnWorkflowHUDRecovery(stale, action);
  assert.equal(calls.length, 1, 'backend snapshot should avoid another status request');
  assert.deepEqual(rendered, [latest]);
  calls.length=0; rendered.length=0;
  body = {code:'unknown_workflow', error:'not found'};
  latest = {contract_version:TURN_WORKFLOW_HUD_CONTRACT, request_id:'request', status:'unknown'};
  await requestTurnWorkflowHUDRecovery(stale, action);
  assert.deepEqual(dismissed, ['request'], 'expired backend workflow must release its obsolete card');
  assert.equal(rendered.length, 0);
  dismissed.length=0;
  changeRequestOnStatus = true;
  await requestTurnWorkflowHUDRecovery(stale, action);
  assert.equal(dismissed.length, 0, 'late recovery response dismissed another request');
  assert.equal(rendered.length, 0, 'late recovery response replaced another request');
})().catch(err => { console.error(err); process.exitCode=1; });
`
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		nodePath = "node"
	}
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("production recovery fixture: %v\n%s", err, output)
	}
}
