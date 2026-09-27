// Synthetic Host replay of production original-source readers and request owners.
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const zlib = require('node:zlib');
const assert = require('node:assert/strict');
const sourcePath = process.argv[2] || path.join(__dirname, '../Archive Center.js');
const source = fs.readFileSync(sourcePath, 'utf8').replace(/\r\n/g, '\n');
const names = [
  'extractActiveChatMessageList','getRisuActiveMessageWindowStart','extractActiveChatComparableMessages',
  'extractComparableMessageRoleAndContent','normalizeRollbackMessageRole','extractMessageContentCandidate',
  'buildCompletedTurnPairsFromActiveChatMessages','buildSessionNormalizeCompletedTurnPairs',
  'beginNextInputFinalizationPipeline','buildNextInputSourceAcceptanceFinality','acceptRisuAfterRequestFinal',
  'backfillOneActiveChatCompletedTurn','buildCompleteTurnRequestBody','buildCompleteTurnSourceAcceptanceObservation',
  'buildRisuActiveChatContextMessageObservation','buildYumiV1ArchiveReadContext','extractActiveChatOriginalMessages',
  'readTranslationOriginalsForArchive','normalizeAssistantPersistenceCandidate','canonicalizeAssistantOutputForPersistence',
  'canonicalizeAssistantTranslationDisplayForPersistence','extractPostprocessorCanonicalAssistantText',
  'extractGigaTransCanonicalAssistantText','extractAssistantTaggedBlocks','removeAssistantTaggedBlocks',
  'attachTranslationDisplayCanonicalizationTrace','attachPostprocessorCanonicalizationTrace',
  'sanitizeNarrativeOutputForDisplay','stripHiddenReasoningEnvelopes','normalizeReasoningEnvelopeName',
  'isReasoningEnvelopeName','getActiveChatMessageStreamingState','readMessageBooleanLike','readMessageStringLike',
  'computeOrchestrationDirtyHashOr1c','getHostContextMessages','getPayloadMessageRoleAndText','auxiliaryMessageContentText',
  'sanitizeForCritic','normalizeAssistantPrefillComparableText','isSameAssistantComparableText',
  'buildLanguageContextTrace','normalizeLanguageContextTrace','detectTextLanguageForTrace','normalizeLanguageCodeForTrace',
  'detectRecentAssistantOutputLanguage','isSupportedMemoryLanguageCode','buildLanguageFallbackChain',
  'buildRollbackAssistantObservations','buildPrepareMessageObservation','buildPrepareTurnHostObservations',
  'computeActiveChatRescanDryRunPlan','buildActiveChatRescanDbRawMap','buildActiveChatRescanDerivedMap',
  'buildActiveChatRescanDryRunRows','buildActiveChatRescanPairsFromDbRawFallback',
  'normalizeActiveChatRescanCompareText','summarizeActiveChatRawMessageShape',
  'buildSessionNormalizeRepairEntriesFromDryRunPlan','sanitizeChatLogRepairEntry','runActiveChatRescanDryRun','escapeAttr','extractActiveChatMessageCount',
];
for(const name of names) {
  const found = new RegExp('^  (?:async )?function '+name+'\\(', 'm').exec(source);
  assert(found, 'production function missing: '+name);
  vm.runInThisContext(source.slice(found.index,source.indexOf('\n  }',found.index)+4),{filename:sourcePath});
}
const requests=[];
const notices=[];
Object.assign(globalThis, {
  AUTO_CONTINUE_USER_INPUT_MARKER:'[Continue]',_nextInputFinalizations:new Map(),_latestOrchResultForUI:null,
  LANGUAGE_MEMORY_CONTRACT_VERSION:'language_memory.v1',LANGUAGE_MEMORY_SEARCH_TEXT_POLICY:'preserve_source',
  DEFAULT_SETTINGS:{},settings:{maxInputContextChars:8000,uiLanguage:'ko'},
  shouldSkipUserInputPersistence:()=>false,isMetaPromptLikeMessage:()=>false,
  debugLog:(...args)=>{throw new Error('unexpected diagnostic: '+args.join(' '));},
  warnLog:(...args)=>{throw new Error('unexpected warning: '+args.join(' '));},
  resolveCurrentActiveChatObject:async(sid)=>{assert.equal(sid,'translation-fixture');return {chat:globalThis.activeChat};},
  saveNextInputFinalizationsToStorage:async()=>{},updateRuntimeState:(...args)=>notices.push(args),
  requestBackendSessionRoutingTurnResolution:async(_sid,mode,observations)=>mode==='batch'
    ? {status:'normal',resolvedObservations:observations.map((item,index)=>({
      observation_index:index,turn_index:1,local_turn_index:1,resolution:'normal',turn_identity_state:'resolved',
      stored_assistant_content:translated,
    }))} : {status:'normal',turnIndex:1,localTurnIndex:1},
  explorerFetchAllChatLogsForSession:async()=>({items:[
    {turn_index:1,role:'user',content:activeChat.message[0].data},
    {turn_index:1,role:'assistant',content:translated},
  ]}),
  explorerFetchTimelineItemsForSessionDryRun:async()=>({items:[]}),
  fetchWorldRules:async()=>({items:[],count:0}),getRisuDbSnapshot:()=>null,
  fetchCanonicalChatLogsForTurn:async()=>[],chatLogItemsContainRole:()=>false,getSessionSnapshot:()=>null,
  resolveRuntimeOutputLanguageOverride:async()=>null,observeRisuPersona:async()=>({}),
  buildSourceToFinalLineageObservation:()=>null,buildRisuRequestObservation:()=>({}),
  tryCompleteTurn:async(_turn,_user,_assistant,_context,_sid,_trace,body)=>{
    requests.push(body);return {status:'ok',save_ok:true,turn_index:body.turn_index};
  },
  completeTurnNeedsFreshReconciliationRetry:()=>false,
  verifyAndRepairCompleteTurnChatLogs:async()=>({status:'verified'}),
  setTurnCounterAtLeast:()=>{},markActiveChatBackfillSaved:async()=>{},
});
const translated='그녀는 약속을 지키고 조용히 마을로 돌아왔다.';
const marker=(id,text)=>'<!-- yumi-tr:v1:'+id+':start -->'+text+'<!-- yumi-tr:v1:'+id+':end -->';
function context(id){return {sessionId:'translation-fixture',requestId:id,requestType:'model',hostChatId:'chat-1',
  state:'captured',userMessageIndex:0,userMessageChatId:'user-1',userMessageRefs:[],userObservedPairOrdinal:1,
  userObservedContentHash:computeOrchestrationDirtyHashOr1c('이야기를 이어서 진행한다.'),requestMessageCount:1};}
async function flush(){await new Promise(resolve=>setImmediate(resolve));}
(async()=>{
  const cases=[];
  for(const [language,original] of [['ja','彼女は約束を守り、静かに村に戻った。'],['en','She kept her promise and quietly returned to the village.']]) {
    const json=JSON.stringify({v:1,model:original,status:'done',translatedAt:1});
    for(const kind of ['yumi_plain','yumi_u','yumi_gzip','gigatrans']) {
      const metadata=kind==='yumi_gzip'?'z:'+zlib.gzipSync(Buffer.from(json)).toString('base64'):kind==='yumi_u'?'u:'+json:json;
      const display=kind==='gigatrans'?'<GigaTrans>'+original+'</GigaTrans>\n'+translated:marker('case1',translated);
      globalThis.activeChat={id:'chat-1',scriptstate:{'$__yumi_tr.case1':metadata},message:[
        {role:'user',data:'이야기를 이어서 진행한다.',chatId:'user-1',time:1000},
        {role:'char',data:display,chatId:'assistant-1',time:2000,generationInfo:{generationId:'generation-1'}},
        {role:'user',data:'그 다음날 이야기를 계속한다.',chatId:'user-2',time:3000},
      ]};
      const before=JSON.stringify(activeChat);
      const raw=extractActiveChatComparableMessages(activeChat);
      const read=await buildYumiV1ArchiveReadContext(raw,raw,activeChat);
      assert.equal(read.activeMessages[1].content,original);
      assert.equal(getHostContextMessages(read.activeMessages)[1].content,original);
      const host=buildPrepareTurnHostObservations('translation-fixture','request','model',null,read.activeMessages,read.payloadMessages,null,true,'chat-1');
      assert(!JSON.stringify(host).includes(translated),'prepare observations leaked translation');
      const pair=(await buildSessionNormalizeCompletedTurnPairs(activeChat)).pairs[0];
      assert.equal(pair.assistantContent,original,'cold start');
      const plan=await computeActiveChatRescanDryRunPlan('translation-fixture',{hostChatId:'chat-1'});
      assert.equal(plan.pairs.at(-1).assistantContent,original,'cold start DB merge restored translation');
      const observations=buildRollbackAssistantObservations(activeChat.message,read.activeMessages);
      assert.equal(observations[0].assistant_content,original);
      assert.equal(observations[0].content_hash,computeOrchestrationDirtyHashOr1c(display),'raw observation hash changed');

      _nextInputFinalizations.clear();
      _nextInputFinalizations.set('translation-fixture',{...context(kind+language),markerId:kind+language,inFlight:false});
      const count=requests.length;
      assert(beginNextInputFinalizationPipeline('translation-fixture',{userMessageIndex:2,userMessageChatId:'user-2'},{}).started);
      for(let i=0;i<100 && requests.length===count;i++) await flush();
      assert.equal(requests.length,count+1,'previous turn did not reach actual request body');
      const previous=requests.at(-1);
      assert.equal(previous.assistant_content,original);
      assert.equal(previous.client_meta.language_context.summary_language,language);
      assert.equal(previous.client_meta.source_acceptance_observation.observed_content_hash,computeOrchestrationDirtyHashOr1c(display));
      assert.equal(previous.client_meta.source_acceptance_observation.persistence_content_hash,computeOrchestrationDirtyHashOr1c(original));

      const currentContext=context('current-'+kind+language);
      const current=acceptRisuAfterRequestFinal(currentContext,read.activeMessages[1].content);
      assert(current.accepted);
      const body=await buildCompleteTurnRequestBody(1,pair.userContent,original,read.activeMessages,'translation-fixture',null,
        {sourceAcceptanceFinality:current.observation});
      assert.equal(body.assistant_content,original);
      assert.equal(body.client_meta.language_context.summary_language,language);
      requests.push(body);
      const repair=await buildCompleteTurnRequestBody(1,pair.userContent,display,raw,'translation-fixture',null,{allowExistingActiveMessage:true});
      assert.equal(repair.assistant_content,original,'raw repair/recovery bypassed source read');
      assert(!JSON.stringify(repair.context_messages).includes(translated));
      assert.equal(JSON.stringify(activeChat),before,'display or Host state mutated');
      cases.push({kind,language,original,previous,current:body,repair});
    }
  }
  // Missing/corrupt metadata must not turn translated display into memory, nor
  // promote an earlier assistant candidate when the latest source is missing.
  for(const broken of [undefined,'broken','z:broken']) {
    activeChat={id:'chat-1',scriptstate:{'$__yumi_tr.case1':broken},message:[
      {role:'user',data:'continue'},{role:'char',data:'an older draft'},
      {role:'char',data:marker('case1',translated)},
    ]};
    const before=JSON.stringify(activeChat);
    assert.equal((await buildSessionNormalizeCompletedTurnPairs(activeChat)).pairs.length,0);
    // Also cover the supplemental DB path with a prior translated stored row.
    activeChat.message.splice(1,1);
    assert.equal((await computeActiveChatRescanDryRunPlan('translation-fixture',{hostChatId:'chat-1'})).pairs.length,0);
    activeChat.message.splice(1,0,{role:'char',data:'an older draft'});
    assert.equal(await buildCompleteTurnRequestBody(1,'continue',marker('case1',translated),[],'translation-fixture'),null);
    assert.equal(JSON.stringify(activeChat),before);
  }
  assert(notices.some(entry=>entry[2]?.reason_code==='translation_original_unavailable'));
  for(const broken of ['<GigaTrans>'+translated,marker('unknown',translated)]) {
    assert.equal(normalizeAssistantPersistenceCandidate(broken),'');
  }
  // A first assistant-only greeting must survive content normalization and reach
  // the same repair entries used by cold start. External routing only assigns
  // the ordinal observed in the Host fixture; it never filters the source.
  const greetingCases=[];
  const greetingFixtures=[
    ...['inner_monologue','character_thoughts','reasons_for_visit','cotton','mascot','filtercomplete_scene'].map(tag=>({
      name:tag, text:'<'+tag+'>The traveler remembered her promise.</'+tag+'>',
    })),
    {name:'plain',text:'The traveler remembered her promise.'},
    {name:'html',text:'<div><p>The traveler remembered her promise.</p></div>'},
    {name:'macro',text:'{{char}} arrived. {{img::welcome}}'},
    {name:'reasoning_then_story',text:'<think>private plan</think>The traveler arrived.',expected:'The traveler arrived.'},
    {name:'reasoning_only',text:'<analysis>private plan</analysis>',reason:'assistant_content_empty_after_normalization'},
    {name:'yumi_missing',text:marker('intro',translated),reason:'translation_original_unavailable'},
    {name:'giga_missing',text:'<GigaTrans></GigaTrans>'+translated,reason:'translation_original_unavailable'},
    {name:'yumi_original',text:marker('intro',translated),original:'The traveler arrived.',expected:'The traveler arrived.'},
    {name:'giga_original',text:'<GigaTrans>The traveler arrived.</GigaTrans>'+translated,expected:'The traveler arrived.'},
  ];
  Object.assign(globalThis,{
    captureSessionHostContextFromCache:sid=>{assert.equal(sid,'translation-fixture');return {hostChatId:'chat-1'};},
    refreshExplorerUI:()=>{},
    explorerFetchAllChatLogsForSession:async()=>({items:[{turn_index:2,role:'assistant',content:'The traveler met the village elder.'}]}),
    requestBackendSessionRoutingTurnResolution:async(sid,mode,observations)=>{
      assert.equal(sid,'translation-fixture'); assert.equal(mode,'batch');
      return {status:'normal',resolvedObservations:observations.map((item,index)=>{
        const messageIndex=item.message_index ?? item.risuAssistantMessageIndex;
        assert(Number.isInteger(messageIndex));
        const turn=activeChat.message.slice(0,messageIndex+1).filter(message=>message.role==='char').length;
        return {observation_index:index,turn_index:turn,local_turn_index:turn,resolution:'normal',turn_identity_state:'resolved'};
      })};
    },
    formatTurnIndexPreview:turns=>turns.join(', '),
  });
  const renderStart=source.indexOf("    let activeChatRescanDryRunPanel = '';");
  const renderEnd=source.indexOf("      let recentRebuildResultHtml = '';",renderStart);
  assert(renderStart>0 && renderEnd>renderStart,'production dry-run panel missing');
  const render='(function(){ const selectedSid="translation-fixture", debugToolsVisible=true;\n'+
    source.slice(renderStart,renderEnd)+'\nreturn dryRunResultHtml; } return ""; })()';
  const labels={};
  for(const match of source.matchAll(/"(explorer\.activeRescan\.[^"]+)": "([^"]*)"/g)) (labels[match[1]]??=[]).push(match[2]);
  for(const fixture of greetingFixtures) for(const paired of [false,true]) {
    activeChat={id:'chat-1',scriptstate:{},message:[
      ...(paired?[{role:'user',data:'Begin the story.',chatId:'user-start'}]:[]),
      {role:'char',data:fixture.text,chatId:'greeting'},
      {role:'user',data:'Continue to the village square.',chatId:'user-next'},
      {role:'char',data:'The traveler met the village elder.',chatId:'assistant-next'},
      {role:'user',data:'What happens next?',chatId:'user-tail'},
    ]};
    if(fixture.original) activeChat.scriptstate['$__yumi_tr.intro']=JSON.stringify({v:1,model:fixture.original,status:'done'});
    const before=JSON.stringify(activeChat);
    const plan=await computeActiveChatRescanDryRunPlan('translation-fixture',{hostChatId:'chat-1'});
    const expectedTurns=activeChat.message.filter(message=>message.role==='char').map((_message,index)=>index+1).filter(turn=>!fixture.reason || turn!==1);
    assert.equal(plan.assistantObservationCount,2);
    assert.deepEqual(plan.pairs.map(pair=>pair.turnIndex),expectedTurns,fixture.name+' plan');
    assert.deepEqual(buildSessionNormalizeRepairEntriesFromDryRunPlan(plan).map(entry=>entry.turn_index),expectedTurns,fixture.name+' repair');
    if(!fixture.reason) assert.equal(plan.pairs[0].assistantContent,fixture.expected||fixture.text,'greeting body changed');
    assert.deepEqual(plan.assistantContentIssues,fixture.reason?[{
      turn_index:1,message_index:paired?1:0,reason:fixture.reason,
    }]:[],'missing or misleading content diagnostic');
    globalThis._activeChatRescanDryRunState={loading:false,error:null,result:null};
    assert(await runActiveChatRescanDryRun('translation-fixture'),_activeChatRescanDryRunState.error);
    const report=_activeChatRescanDryRunState.result;
    assert.equal(report.assistant_content_issue_count,fixture.reason?1:0);
    assert.deepEqual(report.assistant_content_issues,plan.assistantContentIssues);
    assert.equal(report.write_attempted,false); assert.equal(report.llm_call_attempted,false);
    for(const languageIndex of [0,1,2]) {
      globalThis.t=key=>{assert(labels[key]?.[languageIndex], 'missing localized scan label '+key);return labels[key][languageIndex];};
      const html=vm.runInThisContext(render,{filename:sourcePath});
      if(fixture.reason) {
        assert(html.includes('#1 · '+labels['explorer.activeRescan.'+fixture.reason][languageIndex]),'omitted first turn is invisible');
        assert(html.includes(labels['explorer.activeRescan.contentIssues'][languageIndex]));
      } else assert(!html.includes(labels['explorer.activeRescan.contentIssues'][languageIndex]),'false content warning');
    }
    assert.equal(JSON.stringify(activeChat),before,'scan or repair changed Host data');
    greetingCases.push({name:fixture.name,paired,turns:expectedTurns,issues:plan.assistantContentIssues});
  }
  console.log(JSON.stringify({status:'passed',cases,missingOriginalCases:3,greetingCases,hostUnchanged:true}));
})().catch(err=>{console.error(err);process.exitCode=1;});
