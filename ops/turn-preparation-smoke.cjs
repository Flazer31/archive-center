const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const vm = require('node:vm');
const assert = require('node:assert/strict');
const cp = require('node:child_process');
const root = path.resolve(__dirname, '..');
const source = fs.readFileSync(process.argv[2] || path.join(root, 'Archive Center.js'), 'utf8').replaceAll('\r\n', '\n');
function extract(name, text=source) {
  const m = new RegExp('^  (?:async )?function '+name+'\\(', 'm').exec(text);
  assert(m, name);
  const end = text.indexOf('\n  }\n', m.index);
  assert(end > m.index, name+' end');
  return text.slice(m.index, end+5);
}
const results=[];
const sleep=ms=>new Promise(r=>setTimeout(r,ms));
const loaded=new Set();
function load(ctx,names) { for(const name of names) {vm.runInContext(extract(name),ctx);loaded.add(name);} }
function bridgeContext(base) {
  const failures=[];
  const ctx={console,fetch,AbortController,setTimeout,clearTimeout,URLSearchParams,
    settings:{bridgeUrl:base,requestTimeoutMs:1000},R:null,
    _lastBridgeFailureByPath:new Map(),
    resolveBridgeRuntimeRoute:url=>({url,mode:'local-fixture'}),
    // Timeout-setting normalization is outside the transport under test.
    getRequestTimeoutSettingMs:value=>Number(value||1000),
    recordHostDiagnostic:event=>failures.push(event),debugLog:()=>{},warnLog:()=>{},
    extractBridgeErrorDetail:(_,fallback)=>fallback,
  };
  vm.createContext(ctx);load(ctx,['resolveRequestTimeoutMs','bridgeFetch']);
  return {ctx,failures};
}
function beforeContext(transport, inputCount) {
  const rows=Array.from({length:inputCount},(_,i)=>({role:'user',chatId:'u'+i,data:'input '+i,time:i+1}));
  const chat={id:'host-chat',message:rows};
  const hud=[],events=[],captures=[];
  let nextRequest=0;
  const ctx={...transport.ctx,
    settings:{enabled:true,requestTimeoutMs:1000,bridgeUrl:transport.ctx.settings.bridgeUrl},
    DEFAULT_SETTINGS:{memoryDeliveryBudgets:{},topK:8},
    _activeFinalConfirmationRequestContext:null,
    _sessionCache:{sessionId:'test-session',observedChatUniqueId:'host-chat'},
    _turnWorkflowHUDActiveRequestId:'',_turnWorkflowHUDWatchToken:0,
    _turnWorkflowHUDHostWarningsByRequestId:new Map(),
    TURN_WORKFLOW_HUD_CONTRACT:'turn_workflow_hud.v1',
    R:{getChatFromIndex:async()=>chat},
    // Host, settings and rendering boundaries are deterministic. No real user data.
    resolveCurrentActiveChatObject:async()=>({chat,charIdx:0,chatIdx:0}),
    getCurrentChatSessionId:async()=> 'test-session',resolveCanonicalWriteSessionId:async sid=>sid,
    captureSessionHostContextFromCache:()=>({charIdx:0,chatIdx:0}),
    isSaveType:type=>type==='model',recordRisuHookLifecycle:()=>{},clearArchiveCenterRecomposerBridge:()=>{},
    extractMessages:p=>({messages:p.messages,hasMessageSlot:true,path:['messages']}),
    normalizeMessagesForOrchestration:messages=>messages,
    extractRuntimeCurrentChatTokenInfo:()=>({}),makeOrchRequestId:()=> 'request-'+(++nextRequest),
    turnWorkflowHUDIsEnabled:()=>true,cancelTurnWorkflowHUDStream:()=>{},clearTurnWorkflowHUDTimer:()=>{},
    consumeTurnWorkflowHUD:view=>hud.push(structuredClone(view)),
    updateRuntimeState:(...args)=>events.push(args),
    normalizeAssistantPersistenceCandidate:s=>s.trim(),
    renderTurnWorkflowHUDTransportError:(requestId,path,reason,ended)=>hud.push({requestId,path,reason,status:'failed',ended}),
    finishTurnWorkflowHUDCurrentGeneration:requestId=>hud.push({requestId,status:'finished'}),observeTurnWorkflowHUDTiming:()=>{},
    captureAssistantPrefillSeedForSession:async()=>null,
    getCurrentActiveChatSourceObservationMessages:async()=>({chat,messages:rows.map((r,i)=>({role:'user',content:r.data,risuMessageIndex:i,raw:r}))}),
    reconcileRollbackFromHostSignal:async(sid,_host,options)=>{
      assert.equal(sid,'test-session');assert.equal(options.activeChat,chat);events.push(['deletion_observation',sid]);return false;
    },
    beginNextInputFinalizationPipeline:()=>({owned:false,started:false}),
    buildYumiV1ArchiveReadContext:async(payloadMessages,activeMessages)=>({payloadMessages,activeMessages,stats:{markerBlocks:0}}),
    bindRawInputObservationToRequest:()=>null,buildPostOutputSecondaryRequestContext:()=>null,
    buildPrepareTurnHostObservations:(_sid,_rid,_type,_raw,active)=>({active_chat:active.map(m=>({role:m.role,raw_content:m.content,message_index:m.risuMessageIndex}))}),
    observePrepareTurnBootstrap:async()=>({}),buildPrepareTurnSourceObservations:()=>({sourceObservation:{},capabilityObservation:{}}),
    // Prepare inputs/settings and Host observation boundaries; real tryPrepareTurn/bridgeFetch below.
    estimateAdaptiveInjectionBudgetParts:()=>({}),normalizeNarrativeGuideStrength:()=> 'none',
    syncCurrentLorebookReference:async()=>({status:'current'}),getPayloadMessageRoleAndText:m=>({role:m.role,text:m.content}),
    sanitizeTopKSetting:(v,d)=>v||d,currentLorebookReferencePrepareScope:()=>null,
    normalizeLanguageContextTrace:()=>null,observeRisuPersona:async()=>({}),buildRisuRequestObservation:()=>({}),
    debugLog:()=>{},warnLog:(...a)=>events.push(['warning',...a]),
  };
  vm.createContext(ctx);
  load(ctx,['computeOrchestrationDirtyHashOr1c','observeActiveChatInputGroup','captureFinalConfirmationRequestContext',
    'finalConfirmationRequestContextOwnsPendingResponse','finalConfirmationRequestContextRetryIdentityMatches',
    'finalConfirmationRequestContextHasReusablePayloadPlan','installFinalConfirmationRequestContext',
    'primeTurnWorkflowHUD','projectTurnWorkflowHUDPhaseView','tryPrepareTurn','onBeforeRequest',
    'finishFailedBeforeRequestPreparation','acceptRisuAfterRequestFinal']);
  const actual=ctx.captureFinalConfirmationRequestContext;
  ctx.captureFinalConfirmationRequestContext=async(...args)=>{const c=await actual(...args);captures.push(c);return c;};
  return {ctx,hud,events,captures};
}
(async()=>{
  let sourceCalls=0, holdPrepare=false;
  const server=http.createServer((req,res)=>{
    req.resume();
    if(req.url==='/prepare-turn') {
      sourceCalls++;
      if(holdPrepare)return;
      res.writeHead(503,{'Content-Type':'application/json'});res.end('{"error":"fixture unavailable"}');return;
    }
    if(req.url==='/hold-headers')return;
    if(req.url==='/hold-body'){res.writeHead(200,{'Content-Type':'application/json'});res.flushHeaders();res.write('{');return;}
    if(req.url==='/slow') {setTimeout(()=>res.end('{"ok":true}'),150);return;}
    res.end('{"ok":true}');
  });
  await new Promise(r=>server.listen(0,'127.0.0.1',r));
  const transport=bridgeContext('http://127.0.0.1:'+server.address().port);
  try {
    assert.equal((await transport.ctx.bridgeFetch('/ok',{timeoutMs:1000})).ok,true);
    for(const route of ['/hold-headers','/hold-body']) {
      const outcome=await Promise.race([transport.ctx.bridgeFetch(route,{timeoutMs:100}),sleep(650).then(()=> 'still_pending')]);
      assert.equal(outcome,null,route+' must obey the complete-response deadline');
      assert.equal(transport.failures.at(-1).kind,'timeout');
    }
    transport.ctx.settings.requestTimeoutMs=50;
    assert.equal((await transport.ctx.bridgeFetch('/slow',{timeoutMs:0})).ok,true);
    for(const n of [1,2,3]) {
      const h=beforeContext(transport,n), payload={messages:[{role:'user',content:'test input'}]};
      const before=sourceCalls;
      assert.equal(await h.ctx.onBeforeRequest(payload,'model'),payload);
      const failed=h.captures[0];
      assert.equal(failed.state,'captured','fail-open response must remain saveable');
      assert.equal(failed.preparationFailed,true);
      assert.equal(h.hud.at(-1).status,'failed');assert.equal(h.hud.at(-1).ended,true);
      assert.equal(failed.userMessageRefs.length,n);
      assert.equal(await h.ctx.onBeforeRequest(payload,'model'),payload);
      assert.equal(sourceCalls,before+2,'fresh attempt must reach prepare-turn');
      assert.notEqual(h.captures[1].requestId,failed.requestId);
      assert.equal(h.captures[1].userMessageRefs.length,n);
      const accepted=h.ctx.acceptRisuAfterRequestFinal(h.captures[1],'normal fail-open output');
      assert.equal(accepted.accepted,true,'cleanup must preserve actual output acceptance');
      assert.equal(accepted.observation.user_message_refs.length,n);
      const owner={state:'captured',requestId:'new-owner'};
      h.ctx._activeFinalConfirmationRequestContext=owner;
      const count=h.hud.length;
      h.ctx.finishFailedBeforeRequestPreparation(failed,'old-failure',true);
      assert.equal(h.ctx._activeFinalConfirmationRequestContext,owner);
      assert.equal(owner.preparationFailed,undefined);assert.equal(h.hud.length,count);
    }
    holdPrepare=true;
    const timed=beforeContext(transport,2);
    const pending=timed.ctx.onBeforeRequest({messages:[{role:'user',content:'fixture'}]},'model');
    const timedResult=await Promise.race([pending,sleep(1600).then(()=> 'still_pending')]);
    assert.notEqual(timedResult,'still_pending','source-only preparation must use plugin timeout');
    assert.equal(timed.hud.at(-1).status,'failed');
    const ready=beforeContext(transport,2);
    const active=await ready.ctx.captureFinalConfirmationRequestContext('test-session','model','ready',{charIdx:0,chatIdx:0});
    active.retryPayloadReady=true; active.pendingContext={status:'ready'};
    active.orchestrationResult={_injectionPack:{payload_application_plan:{contract_version:'payload_application_plan.v1',owner:'go',apply_rule:'apply_exact_text_without_reassembly',status:'ready'}}};
    ready.ctx._activeFinalConfirmationRequestContext=active;
    const retry=await ready.ctx.captureFinalConfirmationRequestContext('test-session','model','retry',{charIdx:0,chatIdx:0});
    assert.equal(ready.ctx.installFinalConfirmationRequestContext(retry).status,'retry_reused');
    assert.equal(ready.ctx._activeFinalConfirmationRequestContext,active);
    const policies=[];
    ready.ctx.turnWorkflowHUDRequestIdFromPrepareOptions=()=>'';
    ready.ctx.bridgeFetch=async(path,options)=>{assert.equal(path,'/prepare-turn');policies.push(options.timeoutMs);return null;};
    await ready.ctx.tryPrepareTurn('test-session','',[],null,'model',null,{});
    await ready.ctx.tryPrepareTurn('test-session','',[],null,'model',null,{sourceDecisionOnly:true});
    assert.deepEqual(policies,[0,ready.ctx.settings.requestTimeoutMs], 'only the short input decision uses Plugin Timeout');
    console.log(JSON.stringify({status:'passed',scope:'production JS + local HTTP; failure/retry/afterRequest and 1/2/3 input refs; no live Host or model'}));
  } finally {
    server.closeAllConnections();await new Promise(r=>server.close(r));
  }
})().catch(e=>{console.error(e);process.exitCode=1;});
