package main

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

func Test43HUDPreprocessingAndResponseTiming(t *testing.T) {
	node := os.Getenv("ARCHIVE_CENTER_NODE_BINARY")
	if node == "" {
		var err error
		node, err = exec.LookPath("node")
		if err != nil {
			t.Fatal(err)
		}
	}
	src := readArchiveCenterJS(t)
	var production []string
	for _, name := range []string{"escapeTurnWorkflowHUDHTML", "turnWorkflowHUDDismissButtonHTML", "turnWorkflowHUDStageStatus", "turnWorkflowHUDStageStatusColor", "turnWorkflowHUDStageDuration", "turnWorkflowHUDStageReason", "turnWorkflowHUDStageLedgerHTML", "turnWorkflowHUDCountPresentation", "turnWorkflowHUDCountLedgerHTML", "turnWorkflowHUDTimingHTML", "observeTurnWorkflowHUDTiming", "retainTurnWorkflowHUDHostTiming", "consumeTurnWorkflowHUD", "finishTurnWorkflowHUDCurrentGeneration", "projectTurnWorkflowHUDPhaseView", "buildTurnWorkflowHUDPresentation", "turnWorkflowHUDSlotHTML", "buildTurnWorkflowHUDStackPresentation"} {
		production = append(production, extractArchiveCenterJSFunction(t, src, name))
	}
	production = append(production, extractArchiveCenterJSAsyncFunction(t, src, "updateTurnWorkflowHUDElapsed"))
	production = append(production, extractArchiveCenterJSAsyncFunction(t, src, "observeTurnWorkflowHUDHostGeneration"))
	production = append(production, extractArchiveCenterJSFunction(t, src, "turnWorkflowHUDRecoveryPresentation"))
	production = append(production, extractArchiveCenterJSFunction(t, src, "turnWorkflowHUDErrorSummaryHTML"))
	production = append(production, extractArchiveCenterJSFunction(t, src, "turnWorkflowHUDTurnLabel"))
	// Both package builders stamp VERSION. The HUD must follow that value,
	// even when the source checkout still carries the stable release version.
	production = append(production, `const VERSION = "4.4.0-test.3";`)
	production = append(production, regexp.MustCompile(`(?m)^  const BUILD_ID = [^\r\n]+`).FindString(src))
	for _, match := range regexp.MustCompile(`(?m)^  const TURN_WORKFLOW_HUD_[A-Z_]+_STYLE = [^\r\n]+`).FindAllString(src, -1) {
		production = append(production, match)
	}
	script := strings.Join(production, "\n") + `
const assert=require('node:assert/strict');
assert.equal(BUILD_ID, VERSION, 'HUD retained the source version after package stamping');
const translations={};
` + "\n" + func() string {
		lines := []string{}
		for _, match := range regexp.MustCompile(`(?m)^\s*"(turn_hud\.[^"]+)":\s*("[^"\r\n]*"),?`).FindAllStringSubmatch(src, -1) {
			lines = append(lines, "if(!translations["+`"`+match[1]+`"`+"])translations["+`"`+match[1]+`"`+"]="+match[2]+";")
		}
		return strings.Join(lines, "\n")
	}() + `
const t=key=>translations[key]||key;
const tf=(key,args)=>Object.entries(args).reduce((text,[k,v])=>text.replace('{'+k+'}',v),t(key));
const TURN_WORKFLOW_HUD_CONTRACT='turn_workflow_hud.v3';
let _activeFinalConfirmationRequestContext=null;
let _turnWorkflowHUDLastView=null,_turnWorkflowHUDActiveRequestId='request-a',_turnWorkflowHUDLastRevision=0;
let _turnWorkflowHUDWatchToken=0,_turnWorkflowHUDWatchRunning=true,_turnWorkflowHUDTerminalRequestId='';
let _turnWorkflowHUDCurrentFinalizationMode='next_user_input';
const _turnWorkflowHUDHostWarningsByRequestId=new Map();
let enabled=true; const rendered=[];
function turnWorkflowHUDIsEnabled(){return enabled;}
function renderTurnWorkflowHUD(view){_turnWorkflowHUDLastView=retainTurnWorkflowHUDHostTiming(view);rendered.push(_turnWorkflowHUDLastView);}
function cancelTurnWorkflowHUDStream(){}
function clearTurnWorkflowHUDTimer(){}
function queueTurnWorkflowHUDOperation(){throw Error('timed response card was removed');}
function turnWorkflowHUDCloseButtonOnly(){return false;}
function turnWorkflowHUDSeverityStyle(){return '';}
function turnWorkflowHUDWarningListHTML(){return '';}
const view={contract_version:TURN_WORKFLOW_HUD_CONTRACT,request_id:'request-a',revision:1,status:'running',host_timing:{started_ms:1000},stages:[],
 current_stage:{key:'context_assembly',label_key:'turn_hud.stage.context_assembly',ordinal:3,total:12},
 preprocessing:[{role:'event_recent',label_key:'turn_hud.preprocessing.event_recent',duration_ms:1200,calls:[{round:1,status:'succeeded',duration_ms:1200},{round:2,status:'running',started_at:'2026-09-07T00:00:00.000Z',duration_ms:0}]},
 {role:'world_state',label_key:'turn_hud.preprocessing.world_state',duration_ms:500,calls:[{round:1,status:'failed',duration_ms:500}]}]};
assert.equal(turnWorkflowHUDTimingHTML({}), '');
assert.equal(turnWorkflowHUDTimingHTML({preprocessing:[]}), '');
const modeFact=status=>({key:'preprocessing_mode',owner:'go',scope:'current_request',status,severity:'normal'});
const compactMode=(status,patch={})=>buildTurnWorkflowHUDPresentation({...view,facts:[modeFact(status)],...patch},'generation','compact');
for(const [status,text] of [['disabled','전처리 OFF'],['enabled','전처리 ON'],['unavailable','전처리 ?']]) {
  const html=compactMode(status,{preprocessing:[]}).html;
  assert.ok(html.includes(text),'missing compact setting: '+status);
  assert.ok(html.includes('width:min(112px,100%)') && !html.includes('<table'),'mode widened compact HUD');
}
assert.ok(compactMode('enabled').html.includes('전처리 중'),'dispatched running role was not shown');
assert.ok(compactMode('disabled').html.includes('전처리 OFF'),'Jev-only timing must not imply multi-agent ON');
assert.ok(compactMode('enabled',{preprocessing:[],preprocessing_search:{status:'running'}}).html.includes('전처리 중'),'supplemental search was hidden');
assert.ok(compactMode('enabled',{preprocessing:view.preprocessing.map(role=>({...role,calls:role.calls.map(call=>({...call,status:'succeeded'}))}))}).html.includes('전처리 ON'),'finished calls still look running');
assert.ok(!compactMode('enabled',{current_stage:{key:'awaiting_final_output',label_key:'turn_hud.main_response'}}).html.includes('data-turn-workflow-preprocessing'),'preprocessing badge escaped assembly');
assert.ok(!buildTurnWorkflowHUDPresentation({...view,preprocessing:[]},'generation','compact').html.includes('전처리 OFF'),'missing observation is not OFF');
for(const [phase,patch] of [['finalization',{}],['generation',{status:'failed',severity:'error'}],['generation',{host_generation_finished:true}]]) {
  const html=buildTurnWorkflowHUDPresentation({...view,facts:[modeFact('enabled')],...patch},phase,'compact').html;
  assert.ok(!html.includes('data-turn-workflow-preprocessing'),'badge crowded Critic/terminal card');
}
const sharedRoles=['character_objective','subjective_relationship','world_state'];
const groupView={preprocessing_requests:[
 {id:'gateway',display_row:1,round:1,status:'running',provider:'llmgateway',model:'gpt-5.6-luna',timer_role:'event_recent',shared_roles:['event_recent','unresolved_goal']},
 {id:'ollama',display_row:2,round:1,status:'running',provider:'ollama',model:'deepseek-v4.1-flash:cloud',timer_role:'character_objective',shared_roles:sharedRoles}],
 preprocessing:['event_recent',...sharedRoles,'unresolved_goal'].map(role=>({role,label_key:'turn_hud.preprocessing.'+role,calls:[{round:1,status:'running'}]}))};
const groupHTML=turnWorkflowHUDTimingHTML(groupView);
assert.equal((groupHTML.match(/<tr>/g)||[]).length,6,'five role rows must remain visible');
assert.ok(!groupHTML.includes('<details'),'role table must be visible without expansion');
for(const text of ['gpt-5.6-luna','deepseek-v4.1-flash:cloud','llmgateway','ollama','data-turn-workflow-request-']) assert.ok(!groupHTML.includes(text),'request model details leaked: '+text);
for(const role of ['event_recent',...sharedRoles,'unresolved_goal']) assert.ok(groupHTML.includes('data-turn-workflow-agent-time="'+role+'-1"'),'missing live role timer '+role);
assert.equal(groupHTML,turnWorkflowHUDTimingHTML({...groupView,preprocessing_requests:[]}),'request metadata must not alter role HUD');
groupView.preprocessing[0].calls[0]={round:1,status:'repaired',duration_ms:12600};
groupView.preprocessing[0].calls.push({round:2,status:'running'});
const updated=turnWorkflowHUDTimingHTML(groupView);
assert.ok(updated.includes('12.6초')&&updated.includes('보정')&&updated.includes('data-turn-workflow-agent-time="event_recent-2"'));
if(process.env.ARCHIVE_CENTER_HUD_PREVIEW)require('node:fs').writeFileSync(process.env.ARCHIVE_CENTER_HUD_PREVIEW,'<!doctype html><meta charset="utf-8"><body style="background:#252934;color:#F4F5F7;font-family:Arial,sans-serif"><main style="width:260px;padding:12px;background:#181d28;border-radius:12px">'+updated+'</main></body>');
assert.deepEqual(projectTurnWorkflowHUDPhaseView({...view,counts:[{key:'total_committed',value:0}]},'generation').counts,[]);
const searchView={preprocessing_search:{status:'running',started_at:'2026-09-07T00:00:00.000Z',duration_ms:10000,query_count:2,completed_count:1,
 queries:[{role:'event_recent',status:'succeeded',duration_ms:8000,breakdown_ms:{embedding:2000,vector_search:3000,assembly_wait:1000,assembly:2000}},
 {role:'world_state',status:'running',duration_ms:0}]}};
let searchHTML=turnWorkflowHUDTimingHTML(searchView);
assert.ok(searchHTML.includes('추가 기억 검색') && searchHTML.includes('1/2개 처리'),searchHTML);
assert.ok(searchHTML.includes('data-turn-workflow-search-time') && searchHTML.includes('검색별 세부 시간'),searchHTML);
assert.ok(searchHTML.includes('질문 임베딩 2초') && searchHTML.includes('조립 순서 대기 1초'),searchHTML);
searchView.preprocessing_search.status='partial';
searchView.preprocessing_search.completed_count=2;
Object.assign(searchView.preprocessing_search.queries[1],{status:'partial',duration_ms:9000,breakdown_ms:{'<img src=x>':1000,invalid:'not a duration'}});
searchHTML=turnWorkflowHUDTimingHTML(searchView);
assert.ok(searchHTML.includes('일부 결과 확인 · 2/2개 처리 · 10초'),searchHTML);
assert.ok(searchHTML.includes('8초') && searchHTML.includes('9초') && !searchHTML.includes('17초'),searchHTML);
assert.ok(!searchHTML.includes('서로 겹치므로 합산하지 않습니다'),searchHTML);
assert.ok(searchHTML.includes('&lt;img src=x&gt;') && !searchHTML.includes('<img src=x>') && !searchHTML.includes('not a duration'),searchHTML);
assert.ok(!searchHTML.includes('data-turn-workflow-search-time'),searchHTML);
consumeTurnWorkflowHUD(view);
const backendTiming={contract_version:'prepare_turn.backend_timing.v1',total_ms:2500,stages_ms:{vector_recall:200,injection_assembly:1200,supervisor_llm:800,response_assembly:900}};
observeTurnWorkflowHUDTiming('other-request','prepare_completed',3500,backendTiming);
assert.equal(_turnWorkflowHUDLastView.host_timing.backend_timing,undefined);
observeTurnWorkflowHUDTiming('request-a','prepare_completed',3500,backendTiming);
backendTiming.stages_ms.supervisor_llm=999999;
observeTurnWorkflowHUDTiming('request-a','prepare_completed',3900,{total_ms:999999});
observeTurnWorkflowHUDTiming('request-a','main_started',4000);
observeTurnWorkflowHUDTiming('request-a','main_started',9000); // Same logical-request retry keeps its initial wait.
consumeTurnWorkflowHUD({...view,host_timing:undefined,revision:2});
observeTurnWorkflowHUDTiming('request-a','response_received',14000);
observeTurnWorkflowHUDTiming('request-a','response_received',19000);
observeTurnWorkflowHUDTiming('other-request','response_received',30000);
assert.equal(_turnWorkflowHUDLastView.host_timing.main_started_ms,4000);
assert.equal(_turnWorkflowHUDLastView.host_timing.response_received_ms,14000);
assert.equal(_turnWorkflowHUDLastView.host_timing.backend_timing.stages_ms.supervisor_llm,800);
let html=turnWorkflowHUDTimingHTML(_turnWorkflowHUDLastView);
const recordedTiming=_turnWorkflowHUDLastView.host_timing;
const assemblyHTML=turnWorkflowHUDTimingHTML({..._turnWorkflowHUDLastView,host_timing:{...recordedTiming,backend_timing:{...recordedTiming.backend_timing,memory_assembly:{stages_ms:{initial_candidates:250,preprocessing_wall:1000,'<img src=x>':1},preprocessing_stages_ms:{first_input_preparation:50,first_round_wall:900}}}}});
for(const label of ['기억 조립 세부 시간','최초 후보·기본 전달 구성','1차 담당별 입력 구성','1차 호출 전체 (병렬 경과)']) assert.ok(assemblyHTML.includes(label),assemblyHTML);
assert.ok(assemblyHTML.includes('0.25초') && assemblyHTML.includes('0.05초'),assemblyHTML);
assert.ok(assemblyHTML.includes('&lt;img src=x&gt;') && !assemblyHTML.includes('<img src=x>'),assemblyHTML);
assert.ok(html.includes('13초') && html.includes('10초'),html);
assert.ok(html.includes('3초'),html);
const timingCells=Array.from(html.matchAll(/>([^<>]+)<\/(?:div|span)><(?:div|span)[^>]*>([^<>]+)<\/(?:div|span)>/g),match=>[match[1],match[2]]);
for(const [label,value] of [['백엔드 처리 전체','2.5초'],['기억 검색','0.2초'],['출판사 호출','0.8초'],['최종 조립','0.9초']]) assert.ok(timingCells.some(cell=>cell[0]===label && cell[1]===value),html);
assert.ok(!html.includes('합산하지 않습니다'),html);
assert.ok(!html.includes('담당별 호출 시간입니다') && !html.includes('요청 준비에는'),html);
assert.ok(html.includes('1.2초') && html.includes('0.5초') && html.includes('실패'),html);
const recoveryHTML=turnWorkflowHUDTimingHTML({..._turnWorkflowHUDLastView,preprocessing:[
 {role:'event_recent',label_key:'turn_hud.preprocessing.event_recent',selection_source:'ai',calls:[{round:1,status:'repaired',duration_ms:300},{round:2,status:'partial',duration_ms:400}],duration_ms:700},
 {role:'unresolved_goal',label_key:'turn_hud.preprocessing.unresolved_goal',selection_source:'go_default',calls:[{round:1,status:'no_recommendation',duration_ms:200}],duration_ms:200}
]});
for(const expected of ['형식 보정 후 해석','일부 결과 해석','추천 없음','최종 기억 선택 · AI 추천','최종 기억 선택 · Go 기본 선택','0.3초','0.4초']) assert.ok(recoveryHTML.includes(expected),recoveryHTML);
assert.ok(html.includes('data-turn-workflow-agent-time="event_recent-2"'));
assert.ok(buildTurnWorkflowHUDPresentation(_turnWorkflowHUDLastView).html.includes('전처리 담당별'));
assert.ok(finishTurnWorkflowHUDCurrentGeneration('request-a'));
assert.equal(_turnWorkflowHUDLastView.host_generation_finished,true);
assert.equal(_turnWorkflowHUDLastView.status,'running'); // Observation never changes backend finalization state.
assert.ok(buildTurnWorkflowHUDPresentation(_turnWorkflowHUDLastView).terminal);
consumeTurnWorkflowHUD({...view,host_timing:undefined,revision:3,status:'completed'});
assert.equal(_turnWorkflowHUDLastView.host_timing.response_received_ms,14000);
assert.ok(buildTurnWorkflowHUDPresentation(_turnWorkflowHUDLastView).html.includes('13초'));
_turnWorkflowHUDActiveRequestId='request-b';
consumeTurnWorkflowHUD({...view,request_id:'request-b',host_timing:{started_ms:20000},revision:4,preprocessing:undefined});
assert.equal(turnWorkflowHUDTimingHTML(_turnWorkflowHUDLastView),'');
assert.equal(_turnWorkflowHUDLastView.host_timing.backend_timing,undefined);
assert.equal(_turnWorkflowHUDLastView.host_generation_finished,undefined);
const before=_turnWorkflowHUDLastView;enabled=false;
assert.equal(consumeTurnWorkflowHUD({...view,request_id:'request-b',revision:5}),false);
assert.equal(_turnWorkflowHUDLastView,before);
let _turnWorkflowHUDElapsedLastSecond=-1; const live=[];
let _turnWorkflowHUDElapsedElement=[{startedAt:'2026-09-07T00:00:00.000Z',element:{setTextContent:async v=>live.push(v)}},{startedAt:'2026-09-07T00:00:05.000Z',element:{setTextContent:async v=>live.push(v)}}];
const originalNow=Date.now;Date.now=()=>Date.parse('2026-09-07T00:00:09.000Z');
(async()=>{
 await updateTurnWorkflowHUDElapsed();Date.now=originalNow;
 assert.deepEqual(live,[' · 9초',' · 4초']);
 const finished={...view,host_timing:recordedTiming,status:'completed',preprocessing:view.preprocessing.map(role=>({...role,calls:role.calls.map(call=>({...call,status:call.status==='running'?'succeeded':call.status,duration_ms:call.round===2?2100:call.duration_ms})),duration_ms:role.role==='event_recent'?3300:500}))};
 html=buildTurnWorkflowHUDPresentation(finished).html;
 assert.ok(html.includes('3.3초') && html.includes('2.1초'),html);
 const waiting={key:'awaiting_final_output',label_key:'turn_hud.stage.awaiting_final_output',ordinal:6,total:12,status:'running'};
 const saveStage={key:'canonical_memory_saved',label_key:'turn_hud.stage.canonical_memory_saved',ordinal:9,total:12,status:'succeeded',duration_ms:200};
 const counts=[{key:'total_committed',label_key:'turn_hud.count.total_committed',value:0},{key:'raw_user_logs',label_key:'turn_hud.count.raw_user_logs',value:0}];
 const completedGeneration={...finished,status:'running',host_generation_finished:true,current_stage:waiting,stages:[waiting,saveStage],counts};
 const beforeProjection=JSON.stringify(completedGeneration);
 const projected=projectTurnWorkflowHUDPhaseView(completedGeneration,'generation');
 assert.deepEqual(projected.counts,[]);
 assert.equal(projected.current_stage.status,'succeeded');
 assert.equal(projected.current_stage.label_key,'turn_hud.timing.response');
 assert.equal(projected.current_stage.duration_ms,recordedTiming.response_received_ms-recordedTiming.main_started_ms);
 assert.equal(projected.status,'running');
 assert.equal(JSON.stringify(completedGeneration),beforeProjection);
 const split=buildTurnWorkflowHUDStackPresentation(completedGeneration,null,'next_user_input');
 assert.ok(!split.html.includes('총 생성·저장') && !split.html.includes('본문 응답 기다리는 중'),split.html);
 assert.ok(split.html.includes('<table') && split.html.includes('<details'),split.html);
 assert.ok(!split.html.includes('<details open') && !split.currentPresentation.closeButtonOnly,split.html);
 const saved={...completedGeneration,status:'completed',current_stage:saveStage,counts:counts.map(c=>({...c,value:2}))};
 const finalization=projectTurnWorkflowHUDPhaseView(saved,'finalization');
 assert.deepEqual(finalization.counts,saved.counts);
 assert.equal(turnWorkflowHUDTimingHTML(finalization),'');
 const dual=buildTurnWorkflowHUDStackPresentation(completedGeneration,saved,'next_user_input');
 assert.ok(!dual.currentPresentation.html.includes('총 생성·저장'));
 assert.ok(dual.previousPresentation.html.includes('총 생성·저장'));
 assert.ok(!dual.previousPresentation.html.includes('전처리 담당별'));
 assert.equal((dual.html.match(/aria-label="눌러서 닫기"/g)||[]).length,2);
 const immediate=buildTurnWorkflowHUDStackPresentation(saved,null,'immediate_after_response');
 assert.ok(immediate.html.includes('총 생성·저장'));
 // A retained previous-turn card must not change the current request's phase.
 const critic={key:'critic_llm',label_key:'turn_hud.stage.critic_llm',ordinal:9,total:12,status:'running'};
 const modeSwitchViews=[view,{...view,current_stage:critic,stages:[waiting,critic],counts},saved];
 for(const current of modeSwitchViews){
   for(const previousStatus of ['running','failed','invalidated','completed']){
     const previous={...saved,status:previousStatus,host_generation_finished:false};
     const original=JSON.stringify([current,previous]);
     for(const mode of ['immediate_after_response','next_user_input']){
       const single=buildTurnWorkflowHUDStackPresentation(current,null,mode);
       const stacked=buildTurnWorkflowHUDStackPresentation(current,previous,mode);
       assert.deepEqual(stacked.currentPresentation,single.currentPresentation,
         mode+' current stage '+current.current_stage.key+' changed by previous '+previousStatus);
       assert.deepEqual(stacked.previousPresentation,
         buildTurnWorkflowHUDStackPresentation(null,previous,mode).previousPresentation);
       assert.equal(stacked.dual,true);
     }
     assert.equal(JSON.stringify([current,previous]),original);
   }
 }
 const zeroSaved=buildTurnWorkflowHUDStackPresentation({...saved,counts},null,'immediate_after_response');
 assert.ok(zeroSaved.html.includes('총 생성·저장'));

 const compactCritic={key:'critic_llm',label_key:'turn_hud.stage.critic_llm',ordinal:9,total:12,status:'succeeded',duration_ms:8250};
 const finishedCritic={...saved,host_generation_finished:false,stages:[compactCritic,{key:'complete',ordinal:12,total:12,status:'succeeded',duration_ms:0}],current_stage:{key:'complete',ordinal:12,total:12,status:'succeeded',duration_ms:0}};
 const compact=buildTurnWorkflowHUDStackPresentation(completedGeneration,finishedCritic,'next_user_input','compact');
 assert.ok(compact.previousPresentation.html.includes('평론가 호출') && compact.previousPresentation.html.includes('8.25초'), compact.html);
 assert.ok(compact.currentPresentation.html.includes(turnWorkflowHUDStageDuration({status:'succeeded',duration_ms:recordedTiming.response_received_ms-recordedTiming.main_started_ms})), compact.html);
 assert.equal(compact.currentPresentation.terminal,true);
 for(const forbidden of ['<details','<table','ARCHIVE CENTER','총 생성·저장','전처리 담당별','4.4.0-test.3']) assert.ok(!compact.html.includes(forbidden),forbidden);
 assert.ok(compact.html.includes('width:min(112px,100%)'));
 assert.equal(buildTurnWorkflowHUDStackPresentation(completedGeneration,saved,'next_user_input','normal').html, dual.html, 'switching back must preserve normal HUD');
 const failedCritic={...finishedCritic,status:'failed',severity:'error',current_stage:{...compactCritic,status:'failed'},stages:[{...compactCritic,status:'failed'}],error:{stage_key:'critic_llm',code:'PRIVATE_ERROR_CODE',message_key:'private_provider_error',details:[{key:'reason',value:'PRIVATE_ERROR_DETAIL'}]},warnings:[{message_key:'PRIVATE_WARNING'}]};
 const failure=buildTurnWorkflowHUDPresentation(failedCritic,'','compact');
 assert.ok(failure.html.includes(t('turn_hud.stage_status.failed')) && failure.html.includes('8.25초'));
 for(const text of ['PRIVATE_ERROR_CODE','PRIVATE_ERROR_DETAIL','PRIVATE_WARNING','private_provider_error','<details','data-turn-workflow-recovery-action']) assert.ok(!failure.html.includes(text),text);
 assert.equal(failure.terminal,true);assert.equal(failure.closeButtonOnly,true);
 for(const mode of ['normal','compact']) {
   assert.equal(buildTurnWorkflowHUDPresentation(finishedCritic,'',mode).dismissAnywhere,true);
   assert.equal(buildTurnWorkflowHUDPresentation(completedGeneration,'',mode).dismissAnywhere,true);
   for(const status of ['failed','recovering','running','completed_with_warning','invalidated']) {
     const result=buildTurnWorkflowHUDPresentation({...failedCritic,status,host_generation_finished:false},'',mode);
     assert.ok(!result.dismissAnywhere,mode+' '+status+' must stay visible');
   }
 }
 const conciseError={...failedCritic,error:{...failedCritic.error,code:'CRITIC_PROVIDER_TIMEOUT',recovery_actions:[{id:'retry',label_key:'turn_hud.recovery.retry_derived_turn'}]}};
 for(const status of ['failed','recovering']) {
   const result=buildTurnWorkflowHUDPresentation({...conciseError,status,dismissal_policy:'x_only'},'finalization','normal');
   assert.ok(result.html.includes('평론가 응답 시간 초과'),result.html);
   assert.ok(result.html.includes('data-turn-workflow-recovery-action'),'available retry must remain usable');
   for(const text of ['PRIVATE_ERROR_DETAIL','PRIVATE_WARNING','CRITIC_PROVIDER_TIMEOUT','<details','<table','총 생성·저장','단계별']) assert.ok(!result.html.includes(text),text);
 }
 assert.ok(buildTurnWorkflowHUDPresentation(failedCritic,'','normal').html.includes('private_provider_error'),'unknown code must retain message-key summary');
 const runningCritic={...failedCritic,status:'running',severity:'normal',error:null,current_stage:{...compactCritic,status:'running',started_at:'2026-09-07T00:00:05.000Z'},stages:[]};
 const runningCompact=buildTurnWorkflowHUDStackPresentation(view,runningCritic,'next_user_input','compact');
 assert.equal(runningCompact.previousPresentation.elapsedStartedAt,runningCritic.current_stage.started_at);
 assert.ok(!runningCompact.previousPresentation.terminal);
 for(const mode of ['next_user_input','immediate_after_response']) {
   const current={...view,facts:[modeFact('enabled')]};
   const previous={...runningCritic,facts:[modeFact('enabled')],current_stage:{...runningCritic.current_stage,ordinal:9,total:12}};
   const stack=buildTurnWorkflowHUDStackPresentation(current,previous,mode,'compact');
   assert.ok(stack.currentPresentation.html.includes('전처리 중'),'current assembly lost its badge');
   assert.ok(!stack.previousPresentation.html.includes('data-turn-workflow-preprocessing'),'stack projection leaked ON onto previous Critic');
   const immediate=buildTurnWorkflowHUDStackPresentation(previous,null,'immediate_after_response','compact');
   assert.ok(!immediate.currentPresentation.html.includes('data-turn-workflow-preprocessing'),'immediate Critic inherited its earlier preprocessing badge');
 }
 if(process.env.ARCHIVE_CENTER_COMPACT_HUD_PREVIEW){
   const variants={normal:dual.html,compact:compact.html,failed:failure.html,running:runningCompact.html,normal_failed:buildTurnWorkflowHUDPresentation(conciseError,'finalization','normal').html,normal_recovering:buildTurnWorkflowHUDPresentation({...conciseError,status:'recovering',dismissal_policy:'x_only'},'finalization','normal').html};
   const sections=Object.entries(variants).map(([name,body])=>'<section><h2>'+name+'</h2><div class="sample" style="width:224px;display:flex;flex-direction:column;gap:7px">'+body+'</div></section>').join('');
   require('fs').writeFileSync(process.env.ARCHIVE_CENTER_COMPACT_HUD_PREVIEW,'<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><body style="margin:16px;background:#0d0f13;color:#ccd1db;font:12px system-ui;display:flex;flex-wrap:wrap;gap:24px">'+sections);
 }
 if(process.env.ARCHIVE_CENTER_HUD_FIXTURE_OUTPUT){
   const sample={...completedGeneration,logical_turn:117,
     host_timing:{started_ms:1000,prepare_completed_ms:188200,main_started_ms:188300,response_received_ms:369300,backend_timing:{total_ms:185300,stages_ms:{vector_recall:3350,injection_assembly:167900,supervisor_llm:12300,response_assembly:12400}}},
     preprocessing_search:{...searchView.preprocessing_search,status:'succeeded',duration_ms:55500,query_count:5,completed_count:5},
     preprocessing:[['event_recent',20600,13000],['character_objective',9350,7930],['subjective_relationship',36900,38100],['world_state',53000,25800],['unresolved_goal',11900,8460]].map(([role,first,second])=>({role,label_key:'turn_hud.preprocessing.'+role,selection_source:'ai',duration_ms:first+second,calls:[{round:1,status:'succeeded',duration_ms:first},{round:2,status:'succeeded',duration_ms:second}]}))};
   const render=(current,previous)=>buildTurnWorkflowHUDStackPresentation(current,previous,'next_user_input').html;
   const variants={current:render(sample,null),dual:render(sample,{...saved,logical_turn:116}),outcomes:render({...sample,preprocessing:sample.preprocessing.map((role,index)=>({...role,selection_source:index===3?'go_default':'ai',calls:role.calls.map(call=>({...call,status:['repaired','partial','no_recommendation','failed','succeeded'][index]}))}))},null)};
   const fs=require('fs'),target=process.env.ARCHIVE_CENTER_HUD_FIXTURE_OUTPUT;
   for(const [name,body] of Object.entries(variants))fs.writeFileSync(name==='current'?target:target.replace(/\.html$/,'.'+name+'.html'),'<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><body style="margin:0;background:#0d0f13;font-family:Arial,sans-serif"><main style="'+TURN_WORKFLOW_HUD_ROOT_STYLE+'">'+body+'</main>');
   fs.writeFileSync(target.replace(/\.html$/,'.assembly.html'),'<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><body style="margin:12px;background:#0d0f13;color:#ddd;font-family:Arial,sans-serif"><main style="width:280px">'+assemblyHTML+'</main>');
 }
})().catch(error=>{console.error(error);process.exitCode=1});
`
	cmd := exec.Command(node, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("HUD timing runtime: %v\n%s", err, output)
	}
}

func Test43HUDSupplementSearchUsesExistingElapsedTimer(t *testing.T) {
	node := os.Getenv("ARCHIVE_CENTER_NODE_BINARY")
	if node == "" {
		var err error
		node, err = exec.LookPath("node")
		if err != nil {
			t.Fatal(err)
		}
	}
	src := readArchiveCenterJS(t)
	script := extractArchiveCenterJSAsyncFunction(t, src, "applyTurnWorkflowHUDStack") + "\n" +
		extractArchiveCenterJSAsyncFunction(t, src, "updateTurnWorkflowHUDElapsed") + `
const assert=require('node:assert/strict');
const settings={turnWorkflowHUDMode:'normal'};
let _turnWorkflowHUDLastView={preprocessing_search:{status:'running',started_at:'2026-09-07T00:00:05.000Z'}};
let _turnWorkflowHUDPreviousLastView=null,_turnWorkflowHUDCurrentFinalizationMode='next_user_input';
let _turnWorkflowHUDElapsedElement=null,_turnWorkflowHUDElapsedLastSecond=-1,scheduled=0;
const searchElement={text:'',setTextContent:async function(value){this.text=value}};
// Presentation is covered by Test43HUDPreprocessingAndResponseTiming. This
// boundary exposes exactly the element mounted by the current HUD render.
function buildTurnWorkflowHUDStackPresentation(view){return {html:view.preprocessing_search?.status==='running'?'<span data-turn-workflow-search-time></span>':'',currentPresentation:{},previousPresentation:null};}
const root={html:'',setInnerHTML:async function(value){this.html=value},querySelector:async function(selector){assert.equal(selector,'[data-turn-workflow-search-time]');return this.html.includes('data-turn-workflow-search-time')?searchElement:null}};
function clearTurnWorkflowHUDTimer(){_turnWorkflowHUDElapsedElement=null;_turnWorkflowHUDElapsedLastSecond=-1;}
function scheduleTurnWorkflowHUDElapsedFrame(){scheduled++;}
function tf(key,args){assert.equal(key,'turn_hud.elapsed_seconds');return args.n+'초';}
Date.now=()=>Date.parse('2026-09-07T00:00:09.000Z');
(async()=>{
 await applyTurnWorkflowHUDStack(root);
 assert.equal(_turnWorkflowHUDElapsedElement.length,1);
 assert.equal(_turnWorkflowHUDElapsedElement[0].element,searchElement);
 assert.equal(searchElement.text,' · 4초');assert.equal(scheduled,1);
 _turnWorkflowHUDLastView={preprocessing_search:{status:'succeeded',duration_ms:4000}};
 await applyTurnWorkflowHUDStack(root);
 assert.equal(_turnWorkflowHUDElapsedElement,null);assert.equal(scheduled,1);
 _turnWorkflowHUDLastView={}; // A new request without preprocessing has no stale timer.
 await applyTurnWorkflowHUDStack(root);
 assert.equal(root.html,'');assert.equal(_turnWorkflowHUDElapsedElement,null);assert.equal(scheduled,1);
})().catch(error=>{console.error(error);process.exitCode=1});
`
	cmd := exec.Command(node, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("HUD supplemental timer runtime: %v\n%s", err, output)
	}
}

func TestHUDHostGenerationEndsAndRetriesInBothModes(t *testing.T) {
	src := readArchiveCenterJS(t)
	var production []string
	for _, name := range []string{"escapeTurnWorkflowHUDHTML", "turnWorkflowHUDDismissButtonHTML", "turnWorkflowHUDStageStatus", "turnWorkflowHUDStageStatusColor", "turnWorkflowHUDStageDuration", "turnWorkflowHUDStageReason", "turnWorkflowHUDStageLedgerHTML", "turnWorkflowHUDCountPresentation", "turnWorkflowHUDCountLedgerHTML", "turnWorkflowHUDTimingHTML", "projectTurnWorkflowHUDPhaseView", "buildTurnWorkflowHUDPresentation", "turnWorkflowHUDSlotHTML", "buildTurnWorkflowHUDStackPresentation", "turnWorkflowHUDRecoveryPresentation", "turnWorkflowHUDErrorSummaryHTML", "turnWorkflowHUDTurnLabel", "turnWorkflowHUDCloseButtonOnly", "turnWorkflowHUDSeverityStyle", "turnWorkflowHUDWarningListHTML", "retainTurnWorkflowHUDHostTiming", "renderTurnWorkflowHUD", "rememberTurnWorkflowHUDHostWarning", "observeTurnWorkflowHUDTiming", "renderTurnWorkflowHUDSameRequestRetry", "finishTurnWorkflowHUDCurrentGeneration", "clearTurnWorkflowHUDTimer", "scheduleTurnWorkflowHUDElapsedFrame", "redactHostDiagnostic"} {
		production = append(production, extractArchiveCenterJSFunction(t, src, name))
	}
	production = append(production, extractArchiveCenterJSFunction(t, src, "dismissTurnWorkflowHUD"), extractArchiveCenterJSFunction(t, src, "turnWorkflowHUDIsEnabled"))
	for _, name := range []string{"observeTurnWorkflowHUDHostGeneration", "updateTurnWorkflowHUDElapsed", "applyTurnWorkflowHUDStack"} {
		production = append(production, extractArchiveCenterJSAsyncFunction(t, src, name))
	}
	production = append(production, regexp.MustCompile(`(?m)^  const TURN_WORKFLOW_HUD_[A-Z_]+_STYLE = [^\r\n]+`).FindAllString(src, -1)...)
	script := strings.Join(production, "\n") + `
const assert=require('node:assert/strict');
const t=key=>key,tf=(key,args)=>key+JSON.stringify(args),BUILD_ID='fixture';
const TURN_WORKFLOW_HUD_CONTRACT='turn_workflow_hud.v3';
let settings={turnWorkflowHUDMode:'normal',turnFinalizationMode:'next_user_input',apiKey:'synthetic-private-value'};
let _turnWorkflowHUDUnloaded=false,_turnWorkflowHUDActiveRequestId='current',_turnWorkflowHUDLastRevision=0,_turnWorkflowHUDTerminalRequestId='';
let _turnWorkflowHUDLastView=null,_turnWorkflowHUDPreviousLastView=null,_turnWorkflowHUDCurrentFinalizationMode='next_user_input';
let _turnWorkflowHUDElapsedElement=null,_turnWorkflowHUDElapsedLastSecond=-1,_turnWorkflowHUDElapsedTimer=null;
let _turnWorkflowHUDWatchToken=0,_turnWorkflowHUDWatchRunning=true;
let _turnWorkflowHUDPreviousWatchToken=0,_turnWorkflowHUDPreviousWatchRunning=true,_turnWorkflowHUDPreviousRequestId='previous',_turnWorkflowHUDPreviousLastRevision=0;
const TURN_WORKFLOW_HUD_SURFACE_SELECTOR='#fixture-hud';
let _activeFinalConfirmationRequestContext={requestId:'current',characterIndex:3,chatIndex:2};
const _turnWorkflowHUDHostWarningsByRequestId=new Map(),timers=new Map(),operations=[];
let clock=10000,serial=0,control=true,error='',selectedChat=2,readError=null,unavailable=false;
Date.now=()=>clock;
const setTimeout=fn=>{const id=++serial;timers.set(id,fn);return id;};
const clearTimeout=id=>timers.delete(id);
const R={getCurrentCharacterIndex:async()=>3,getCurrentChatIndex:async()=>selectedChat};
const hostDocument={querySelector:async selector=>{
 if(unavailable)throw Error('Host DOM temporarily unavailable');
 if(selector===TURN_WORKFLOW_HUD_SURFACE_SELECTOR)return root;
 if(selector==='button[aria-labelledby="cancel"]')return control?{}:null;
 assert.equal(selector,'[role="dialog"] h2 .text-draculared');
 if(readError)await readError();
 return error?{getParent:async()=>({getParent:async()=>({getParent:async()=>({innerText:async()=>error})})})}:null;
}};
async function getTurnWorkflowHUDMainDocument(){return hostDocument;}
const timeElements=new Map();
const root={html:'',setInnerHTML:async function(html){this.html=html;},querySelector:async selector=>{
 if(selector.endsWith(' time')||selector==='time'){
  if(!timeElements.has(selector))timeElements.set(selector,{text:'',setTextContent:async function(text){this.text=text;}});
  return timeElements.get(selector);
 }
 return {};
}};
async function ensureTurnWorkflowHUDRoot(){return root;}
function queueTurnWorkflowHUDOperation(label,fn){operations.push(fn);return Promise.resolve();}
async function removeTurnWorkflowHUDDismissListeners(){}
async function attachTurnWorkflowHUDDismiss(){}
async function attachTurnWorkflowHUDRecovery(root,view,action){assert.ok(!action);}
function cancelTurnWorkflowHUDStream(){}
function cancelTurnWorkflowHUDPreviousStream(){}
function takeTurnWorkflowHUDDismissListenerIds(){return [];}
function debugLog(){} // Optional DOM transport failures do not change outcomes.
function bridgeFetch(){throw Error('HUD observation must never mutate backend state');}
async function flush(){while(operations.length)await operations.shift()();}
function view(){return {contract_version:TURN_WORKFLOW_HUD_CONTRACT,request_id:'current',revision:6,status:'awaiting_final_output',severity:'normal',
 host_timing:{started_ms:1000,main_started_ms:5000,backend_timing:{total_ms:4000}},
 current_stage:{key:'awaiting_final_output',label_key:'turn_hud.stage.awaiting_final_output',ordinal:6,total:12,status:'running',llm_call:false},stages:[]};}
const previous={contract_version:TURN_WORKFLOW_HUD_CONTRACT,request_id:'previous',status:'running',severity:'normal',
 current_stage:{key:'critic_llm',label_key:'turn_hud.stage.critic_llm',ordinal:9,total:12,status:'running',llm_call:true,started_at:new Date(3000).toISOString()},stages:[]};
async function mount(){
 clearTurnWorkflowHUDTimer();control=true;error='';selectedChat=2;unavailable=false;readError=null;
 _turnWorkflowHUDActiveRequestId='current';_turnWorkflowHUDCurrentFinalizationMode='next_user_input';
 _turnWorkflowHUDPreviousRequestId='previous';
 _turnWorkflowHUDLastView=view();_turnWorkflowHUDPreviousLastView=previous;
 _turnWorkflowHUDHostWarningsByRequestId.clear();_turnWorkflowHUDTerminalRequestId='';
 await applyTurnWorkflowHUDStack(root);
 assert.equal(_turnWorkflowHUDLastView.host_generation_control_seen,true);
 assert.equal(timers.size,1,'normal and compact must share exactly one running HUD timer');
}
async function tick(){
 clock+=2000;
 const [id,fn]=[...timers][0];timers.delete(id);fn();
 await new Promise(setImmediate);await flush();
 assert.ok(timers.size<=1,'Host observation started a second timer');
}
(async()=>{
 for(const mode of ['normal','compact']){
  settings.turnWorkflowHUDMode=mode;
  for(const failure of ['HTTP 400 bad request','HTTP 429 rate limit','HTTP 500 server error','API connection failed','Model not found','Stream disconnected','']){
   await mount();control=failure==='HTTP 429 rate limit';error=failure ? failure+' Bearer synthetic-private-value' : '';
   await tick();
   const ended=_turnWorkflowHUDLastView.host_generation_end;
   assert.equal(ended?.kind,failure?'failed':'stopped',mode+': failed or cancelled Host kept waiting');
   assert.equal(_turnWorkflowHUDLastView.status,'awaiting_final_output','UI observation overwrote backend state');
   const stack=buildTurnWorkflowHUDStackPresentation(_turnWorkflowHUDLastView,previous,'next_user_input',mode);
   assert.equal(stack.currentPresentation.terminal,true);
   assert.equal(stack.currentPresentation.elapsedStartedAt,'');
   assert.equal(stack.previousPresentation.terminal,false,'main failure stopped previous Critic');
   assert.ok(root.html.includes('turn_hud.stage.critic_llm'));
   assert.ok(!root.html.includes('synthetic-private-value'));
   const warning=_turnWorkflowHUDHostWarningsByRequestId.get('current')[0];
   assert.ok(!warning.detail.includes('synthetic-private-value'),'dashboard error leaked a credential');
   if(failure)assert.ok(warning.detail.includes(failure),'dashboard lost the full error');
   const frozen=JSON.stringify(ended);clock+=10000;
   await applyTurnWorkflowHUDStack(root);assert.equal(JSON.stringify(_turnWorkflowHUDLastView.host_generation_end),frozen);
   // A late backend preparation replay cannot restart the failed wait.
   await renderTurnWorkflowHUD({...view(),revision:7});await flush();
   assert.deepEqual(_turnWorkflowHUDLastView.host_generation_end,ended);
   control=true;error='';clock+=1000;
   renderTurnWorkflowHUDSameRequestRetry('current',2);await flush();
   assert.equal(_turnWorkflowHUDLastView.host_generation_end,null,'retry stayed failed');
   assert.equal(_turnWorkflowHUDLastView.host_timing.main_started_ms,clock,'retry retained previous wait duration');
   assert.equal(_turnWorkflowHUDLastView.host_timing.backend_timing.total_ms,4000,'retry repeated preparation');
   assert.equal(_turnWorkflowHUDLastView.host_timing.preparation_finished_ms,5000,'failed wait was counted as preparation');
   assert.equal(buildTurnWorkflowHUDStackPresentation(_turnWorkflowHUDLastView,previous,'next_user_input',mode).currentPresentation.terminal,false);
   clock+=1000;control=false;
   observeTurnWorkflowHUDTiming('current','response_received',clock);finishTurnWorkflowHUDCurrentGeneration('current');await flush();
   assert.equal(_turnWorkflowHUDLastView.host_generation_end,null,'successful retry retained failure');
   assert.equal(_turnWorkflowHUDLastView.host_generation_finished,true);
   assert.equal(_turnWorkflowHUDPreviousLastView,previous);
  }
  for(const currentOnly of [true,false])for(const queuedClose of [false,true])for(const failure of ['HTTP 400','HTTP 429','HTTP 500','Connection failed','Model unavailable','Stream closed','']){
   settings.turnWorkflowHUDMode=mode;
   await mount();error=failure;control=false;await tick();
   dismissTurnWorkflowHUD('current',currentOnly);
   if(!queuedClose)await flush();
   assert.equal(_turnWorkflowHUDActiveRequestId,'');assert.equal(_turnWorkflowHUDLastView,null);
   if(!queuedClose)assert.ok(!root.html.includes('turn_hud.main_response'),'closed card remained visible');
   control=true;error='';clock+=2000;
   assert.equal(renderTurnWorkflowHUDSameRequestRetry('current',2),true,mode+': retry after closing HUD was rejected');
   await flush();
   assert.equal(_turnWorkflowHUDActiveRequestId,'current');
   assert.equal(_turnWorkflowHUDCurrentFinalizationMode,'next_user_input','reopened HUD changed finalization mode');
   assert.equal(_turnWorkflowHUDLastView.current_stage.key,'awaiting_final_output','reused payload was presented as new preparation');
   assert.equal(_turnWorkflowHUDLastView.host_timing.main_started_ms,clock);
   assert.equal(_turnWorkflowHUDLastView.host_timing.started_ms,undefined,'dismissed preparation time was invented');
   assert.ok(root.html.includes('turn_hud.stage.awaiting_final_output'),'retry after closing HUD stayed invisible');
   if(currentOnly)assert.equal(_turnWorkflowHUDPreviousLastView,previous,'retry discarded previous Critic');
   for(let attempt=3;attempt<=4;attempt++){
    control=false;error=attempt===3?'HTTP 429':'';await tick();
    assert.equal(_turnWorkflowHUDLastView.host_generation_end.kind,error?'failed':'stopped');
    control=true;error='';clock+=2000;
    assert.equal(renderTurnWorkflowHUDSameRequestRetry('current',attempt),true);await flush();
    assert.equal(_turnWorkflowHUDLastView.host_generation_end,null);
    assert.equal(_turnWorkflowHUDLastView.host_timing.main_started_ms,clock);
    assert.equal(timers.size,1,'retry lost or duplicated the HUD timer');
   }
   clock+=1000;control=false;
   observeTurnWorkflowHUDTiming('current','response_received',clock);
   finishTurnWorkflowHUDCurrentGeneration('current');await flush();
   assert.equal(_turnWorkflowHUDLastView.host_generation_finished,true,'restored HUD did not complete after successful output');
  }
  // A mode change between dismissal and retry uses the newly selected presentation.
  await mount();dismissTurnWorkflowHUD('current',true);await flush();
  settings.turnWorkflowHUDMode=mode==='normal'?'compact':'normal';
  renderTurnWorkflowHUDSameRequestRetry('current',2);await flush();
  assert.equal(root.html.includes('width:min(112px,100%)'),settings.turnWorkflowHUDMode==='compact');
  settings.turnWorkflowHUDMode=mode;
  await mount();dismissTurnWorkflowHUD('current',false);await flush();
  settings.turnFinalizationMode='immediate_after_response';
  renderTurnWorkflowHUDSameRequestRetry('current',2);await flush();
  assert.equal(_turnWorkflowHUDCurrentFinalizationMode,'immediate_after_response');
  assert.ok(root.html.includes('turn_hud.stage.awaiting_final_output'),'current-turn mode did not restore waiting HUD');
  settings.turnFinalizationMode='next_user_input';
  // Disabling/unloading the UI still leaves the request and provider path alone.
  for(const disabled of ['preference','unloaded']){
   await mount();dismissTurnWorkflowHUD('current',false);await flush();
   if(disabled==='preference')settings.turnWorkflowHUDEnabled=false;else _turnWorkflowHUDUnloaded=true;
   const context=_activeFinalConfirmationRequestContext;
   assert.equal(renderTurnWorkflowHUDSameRequestRetry('current',2),false);
   await flush();assert.equal(root.html,'');assert.equal(_activeFinalConfirmationRequestContext,context);
   settings.turnWorkflowHUDEnabled=true;_turnWorkflowHUDUnloaded=false;
  }
  await mount();const newer=_turnWorkflowHUDLastView;
  assert.equal(renderTurnWorkflowHUDSameRequestRetry('older-request',2),false);
  assert.equal(_turnWorkflowHUDLastView,newer,'old retry took over another request');
  assert.equal(renderTurnWorkflowHUDSameRequestRetry('',2),false);
  await mount();selectedChat=4;control=false;error='HTTP 429 on another chat';await tick();
  assert.equal(_turnWorkflowHUDLastView.host_generation_end,undefined,'chat switch was classified as cancellation');
  await mount();unavailable=true;await tick();
  assert.equal(_turnWorkflowHUDLastView.host_generation_end,undefined,'unavailable Host DOM was treated as failure');
  await mount();control=false;_turnWorkflowHUDLastView.host_generation_control_seen=false;await tick();
  assert.equal(_turnWorkflowHUDLastView.host_generation_end,undefined,'unobserved control became a guessed cancellation');
  await mount();control=false;error='late HTTP 500';
  readError=async()=>{_turnWorkflowHUDLastView={...view(),request_id:'new'};};
  await tick();assert.equal(_turnWorkflowHUDLastView.host_generation_end,undefined,'old observation stopped a new request');
  await mount();control=false;error='late failure from the prior attempt';
  readError=async()=>{readError=null;control=true;error='';renderTurnWorkflowHUDSameRequestRetry('current',2);};
  await tick();assert.equal(_turnWorkflowHUDLastView.host_generation_end,null,'old observation stopped a same-ID retry');
 }
 clearTurnWorkflowHUDTimer();
})().catch(err=>{console.error(err);process.exitCode=1;});
`
	node := os.Getenv("ARCHIVE_CENTER_NODE_BINARY")
	if node == "" {
		node = "node"
	}
	cmd := exec.Command(node, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Host failure/stop/retry HUD: %v\n%s", err, output)
	}
}

func TestCompactHUDUsesExistingTimerForBothCards(t *testing.T) {
	node := os.Getenv("ARCHIVE_CENTER_NODE_BINARY")
	if node == "" {
		var err error
		node, err = exec.LookPath("node")
		if err != nil {
			t.Fatal(err)
		}
	}
	src := readArchiveCenterJS(t)
	script := extractArchiveCenterJSAsyncFunction(t, src, "applyTurnWorkflowHUDStack") + "\n" + extractArchiveCenterJSAsyncFunction(t, src, "updateTurnWorkflowHUDElapsed") + `
const assert=require('node:assert/strict');
const settings={turnWorkflowHUDMode:'compact'};
let _turnWorkflowHUDLastView={},_turnWorkflowHUDPreviousLastView={},_turnWorkflowHUDCurrentFinalizationMode='next_user_input';
let _turnWorkflowHUDElapsedElement=null,_turnWorkflowHUDElapsedLastSecond=-1,scheduled=0;
const elements={current:{text:'',setTextContent:async function(x){this.text=x}},previous:{text:'',setTextContent:async function(x){this.text=x}}};
// The production presentation is covered above. This DOM boundary exposes the
// current/previous time elements actually mounted by the stack owner.
function buildTurnWorkflowHUDStackPresentation(current,previous,finalization,mode){
 assert.equal(mode,'compact');
 return {html:'<time current></time><time previous></time>',currentPresentation:{compact:true,elapsedStartedAt:'2026-09-22T00:00:01Z'},previousPresentation:previous?{compact:true,elapsedStartedAt:'2026-09-22T00:00:04Z'}:null,elapsedSelector:'current time',previousCardSelector:'previous'};
}
const root={setInnerHTML:async()=>{},querySelector:async selector=>{if(selector==='previous')return {};assert.ok(['current time','previous time'].includes(selector));return elements[selector.split(' ')[0]];}};
function clearTurnWorkflowHUDTimer(){_turnWorkflowHUDElapsedElement=null;_turnWorkflowHUDElapsedLastSecond=-1;}
function scheduleTurnWorkflowHUDElapsedFrame(){scheduled++;}
async function attachTurnWorkflowHUDRecovery(root,view,action){assert.equal(action,undefined);}
const tf=(key,args)=>args.n+'초';Date.now=()=>Date.parse('2026-09-22T00:00:09Z');
(async()=>{
 await applyTurnWorkflowHUDStack(root);
 assert.equal(_turnWorkflowHUDElapsedElement.length,2);assert.equal(scheduled,1);
 assert.equal(elements.current.text,' · 8초');assert.equal(elements.previous.text,' · 5초');
 _turnWorkflowHUDPreviousLastView=null;
 await applyTurnWorkflowHUDStack(root);
 assert.equal(_turnWorkflowHUDElapsedElement.length,1);assert.equal(scheduled,2);
})().catch(error=>{console.error(error);process.exitCode=1});
`
	cmd := exec.Command(node, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compact timer: %v\n%s", err, output)
	}
}
