// Production-owner replay. Host reads and backend persistence are recorded external boundaries.
const fs = require('fs');
const path = require('path');
const vm = require('vm');
const assert = require('assert');
const sourcePath = process.argv[2] || path.resolve(__dirname, '../Archive Center.js');
const src = fs.readFileSync(sourcePath, 'utf8');
function extract(name) {
  const match = new RegExp('^  (?:async )?function ' + name + '\\(', 'm').exec(src);
  assert(match, name);
  const end = src.indexOf('\n  }', match.index);
  assert(end > match.index, name);
  return src.slice(match.index, end + 4);
}
function host(messages) {
  const chat = {id:'audit-chat', message:structuredClone(messages)};
  const events=[];
  const context = {
    settings:{enabled:true}, isSaveType:type=>type==='model',
    R:{getChatFromIndex() { throw Error('Unexpected host call'); }},
    resolveCurrentActiveChatObject:async(sid, coordinates)=>{
      assert.equal(sid,'audit-session'); assert.equal(coordinates.charIdx,0);
      return {charIdx:0,chatIdx:0,chat};
    },
    updateRuntimeState(...args){events.push(args);}, debugLog(...args){throw Error(args.join(' '));},
    warnLog(...args){events.push(['warning',...args]);},
    extractActiveChatComparableMessages:chat=>comparable(chat.message),
    buildCompleteTurnSourceAcceptanceObservation:async(sid,answer,options)=>{
      assert.equal(sid,'audit-session'); assert.equal(answer,'answer');
      assert(options.allowExistingActiveMessage); assert(['A','B'].includes(options.userInput));
      return {observed_content_hash:context.computeOrchestrationDirtyHashOr1c(answer)};
    },
    backfillOneActiveChatCompletedTurn:async(sid,pair,options)=>{
      assert.equal(sid,'audit-session');assert.equal(pair.assistantContent,'answer');
      const finality=options.sourceAcceptanceFinality;
      assert.equal(finality.user_message_refs.length,1);
      assert.equal(finality.user_message_refs[0].message_chat_id,pair.userContent);
      assert.equal(finality.user_message_index,0);assert.equal(finality.message_index,1);
      assert.equal(finality.next_signal_user_index,2);
      assert.equal(finality.user_persistence_content_hash,context.computeOrchestrationDirtyHashOr1c(pair.userContent));
      events.push(['persist',pair.userContent,finality]);
      return {status:'saved',turnIndex:1};
    },
    _nextInputFinalizations:new Map(),
    saveNextInputFinalizationsToStorage:async()=>{},
    extractComparableMessageRoleAndContent:m=>m?{role:m.role==='char'?'assistant':m.role,content:m.data}:null,
    shouldSkipUserInputPersistence:()=>false,
    normalizeAssistantPersistenceCandidate:text=>String(text).trim(),
    getActiveChatMessageStreamingState:()=>false,
    AUTO_CONTINUE_USER_INPUT_MARKER:'[auto-continue]',
    _activeFinalConfirmationRequestContext:null,
  };
  vm.createContext(context);
  for(const name of ['computeOrchestrationDirtyHashOr1c','observeActiveChatInputGroup','captureFinalConfirmationRequestContext',
    'extractActiveChatMessageList','pickActiveChatArrayCandidate','scoreActiveChatMessageArrayCandidate','isLikelyRisuMemorySummaryRecord',
    'buildCompletedTurnPairsFromActiveChatMessages','extractActiveChatOriginalMessages','buildYumiV1ArchiveReadContext',
    'extractGigaTransCanonicalAssistantText','finalConfirmationRequestContextOwnsPendingResponse',
    'finalConfirmationRequestContextRetryIdentityMatches','finalConfirmationRequestContextHasReusablePayloadPlan',
    'installFinalConfirmationRequestContext','serializeAcceptedFinalRecoveryPayload','queueNextInputFinalization','buildNextInputSourceAcceptanceFinality','beginNextInputFinalizationPipeline']) vm.runInContext(extract(name),context);
  return {chat,context,events,capture:()=>context.captureFinalConfirmationRequestContext('audit-session','model','audit-request',{charIdx:0,chatIdx:0})};
}
function preceding(turns) {
  return Array.from({length:turns},(_,i)=>[
    {role:'user',data:'input-'+(i+1),chatId:'u-'+(i+1),time:i*2+1},
    {role:'char',data:'answer-'+(i+1),chatId:'a-'+(i+1),time:i*2+2},
  ]).flat();
}
function comparable(messages) {
  return messages.map((m,i)=>({role:m.role==='char'?'assistant':m.role,
    content:m.data,risuMessageIndex:i,raw:m}));
}
function ready(context) {
  context.retryPayloadReady=true;
  context.pendingContext={status:'ready'};
  context.orchestrationResult={_injectionPack:{payload_application_plan:{
    contract_version:'payload_application_plan.v1',owner:'go',apply_rule:'apply_exact_text_without_reassembly',status:'ready'}}};
  return context;
}
(async()=>{
  const rows=[];
  for(const n of [0,1,2,3]) {
    const messages=Array.from({length:n},(_,i)=>({role:'user',data:i===1?'':'input-'+i,chatId:'u-'+i}));
    const h=host(messages);
    h.chat.history=preceding(10);
    assert.strictEqual(h.context.extractActiveChatMessageList(h.chat),h.chat.message,
      'official current list, including empty, must not resurrect deleted history');
    const pairs=h.context.buildCompletedTurnPairsFromActiveChatMessages(comparable([...messages,{role:'char',data:'answer'}]),{preserveAllUserInputs:true});
    assert.equal(pairs.length,n>0?1:0);
    if(n>0) assert.equal(pairs[0].userMessageRefs.length,n);
  }
  const legacy=host([]);delete legacy.chat.message;legacy.chat.history=preceding(2);
  assert.strictEqual(legacy.context.extractActiveChatMessageList(legacy.chat),legacy.chat.history,
    'existing fallback remains available only when the official field is absent');
  for(const n of [1,2,3]) {
    const messages=preceding(27);
    for(let i=1;i<=n;i++)messages.push({role:'user',data:'current-input-'+i,chatId:'current-'+i,time:100+i});
    const h=host(messages), anchor=await h.capture();
    const completed=[...messages,{role:'char',data:'answer-28',chatId:'a-28'}];
    const pairs=h.context.buildCompletedTurnPairsFromActiveChatMessages(comparable(completed),{preserveAllUserInputs:true});
    assert.equal(anchor.userObservedPairOrdinal,preceding(27).filter(m=>m.role==='char').length+1);
    assert.equal(anchor.userMessageIndex,messages.length-1);
    assert.equal(anchor.userObservedContent,Array.from({length:n},(_,i)=>'current-input-'+(i+1)).join('\n\n'));
    assert.equal(pairs.at(-1).userContent,anchor.userObservedContent);
    assert.equal(pairs.at(-1).observedInputGroupOrdinal,anchor.userObservedPairOrdinal);
    assert.equal(pairs.at(-1).userMessageRefs.length,n);
    const payload=h.context.serializeAcceptedFinalRecoveryPayload({source_acceptance_finality:{user_message_refs:anchor.userMessageRefs}});
    assert.equal(payload.source_acceptance_finality.user_message_refs.length,n);
    rows.push({kind:'input_count',inputs:n,expectedTurn:28,anchor,lastPair:pairs.at(-1),pairCount:pairs.length});
  }
  for(const changedIndex of [0,1]) {
    const h=host([{role:'user',data:'A',chatId:'A',time:1},{role:'user',data:'B',chatId:'B',time:2}]);
    const first=ready(await h.capture());
    h.context._activeFinalConfirmationRequestContext=first;
    h.chat.message[changedIndex].data+=' edited';
    const second=await h.capture();
    const identityMatch=h.context.finalConfirmationRequestContextRetryIdentityMatches(first,second);
    const install=h.context.installFinalConfirmationRequestContext(second);
    assert.equal(identityMatch,false,'editing any group member must invalidate pending payload reuse');
    rows.push({kind:'pending_input_edit',editedInput:changedIndex===0?'first':'last',identityMatch,installStatus:install.status,reason:install.reason||''});
  }
  const h=host([{role:'user',data:'A',chatId:'A'},{role:'user',data:'B',chatId:'B'},
    {role:'char',data:'first answer',chatId:'C'},{role:'char',data:'second answer',chatId:'D'}]);
  const pairs=h.context.buildCompletedTurnPairsFromActiveChatMessages(comparable(h.chat.message),{preserveAllUserInputs:true});
  rows.push({kind:'consecutive_assistant_candidates',pair:pairs[0],pairCount:pairs.length,
    note:'No inference that separate char rows are continuation versus reroll; this records current behavior.'});
  const missing=host([{role:'char',data:'greeting',chatId:'G'}]);
  rows.push({kind:'missing_user',anchor:await missing.capture()});
  for(const removed of ['first','last','all']) {
    const h=host([{role:'user',data:'A',chatId:'A'},{role:'user',data:'B',chatId:'B'}]);
    const previous=await h.capture();
    h.context.queueNextInputFinalization(previous,{persistence_content_hash:h.context.computeOrchestrationDirtyHashOr1c('answer')});
    h.chat.message.push({role:'char',data:'answer',chatId:'C'});
    h.chat.message=h.chat.message.filter(m=>removed==='all'?m.role!=='user':m.chatId!==(removed==='first'?'A':'B'));
    h.chat.message.push({role:'user',data:'next input',chatId:'D'});
    const current=await h.capture();
    const start=h.context.beginNextInputFinalizationPipeline('audit-session',current,{charIdx:0,chatIdx:0});
    await new Promise(resolve=>setImmediate(resolve));
    assert.equal(start.started,true);
    const persistence=h.events.filter(e=>e[0]==='persist');
    assert.equal(persistence.length,removed==='all'?0:1);
    assert.equal(h.context._nextInputFinalizations.has('audit-session'),removed==='all');
    assert.equal(h.events.filter(e=>e[0]==='warning').length,0);
    rows.push({kind:'pending_previous_input_deleted',removed,previousIndex:previous.userMessageIndex,currentIndex:current.userMessageIndex,start,
      stillPending:h.context._nextInputFinalizations.has('audit-session'),events:h.events.filter(e=>e[0]==='lastCompleteTurnStatus'||e[0]==='warning')});
  }
  console.log(JSON.stringify({status:'passed',cases:rows.length,coverage:'host capture, pair grouping, retry edits, previous-input deletion, recovery metadata'}));
})().catch(err=>{console.error(err);process.exitCode=1;});
