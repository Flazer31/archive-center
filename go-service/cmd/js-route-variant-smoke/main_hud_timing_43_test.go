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
	production = append(production, extractArchiveCenterJSFunction(t, src, "turnWorkflowHUDTurnLabel"))
	production = append(production, regexp.MustCompile(`(?m)^  const BUILD_ID = [^\r\n]+`).FindString(src))
	for _, match := range regexp.MustCompile(`(?m)^  const TURN_WORKFLOW_HUD_[A-Z_]+_STYLE = [^\r\n]+`).FindAllString(src, -1) {
		production = append(production, match)
	}
	script := strings.Join(production, "\n") + `
const assert=require('node:assert/strict');
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
 const zeroSaved=buildTurnWorkflowHUDStackPresentation({...saved,counts},null,'immediate_after_response');
 assert.ok(zeroSaved.html.includes('총 생성·저장'));
 if(process.env.ARCHIVE_CENTER_HUD_FIXTURE_OUTPUT){
   const sample={...completedGeneration,logical_turn:117,
     host_timing:{started_ms:1000,prepare_completed_ms:188200,main_started_ms:188300,response_received_ms:369300,backend_timing:{total_ms:185300,stages_ms:{vector_recall:3350,injection_assembly:167900,supervisor_llm:12300,response_assembly:12400}}},
     preprocessing_search:{...searchView.preprocessing_search,status:'succeeded',duration_ms:55500,query_count:5,completed_count:5},
     preprocessing:[['event_recent',20600,13000],['character_objective',9350,7930],['subjective_relationship',36900,38100],['world_state',53000,25800],['unresolved_goal',11900,8460]].map(([role,first,second])=>({role,label_key:'turn_hud.preprocessing.'+role,selection_source:'ai',duration_ms:first+second,calls:[{round:1,status:'succeeded',duration_ms:first},{round:2,status:'succeeded',duration_ms:second}]}))};
   const render=(current,previous)=>buildTurnWorkflowHUDStackPresentation(current,previous,'next_user_input').html;
   const variants={current:render(sample,null),dual:render(sample,{...saved,logical_turn:116}),outcomes:render({...sample,preprocessing:sample.preprocessing.map((role,index)=>({...role,selection_source:index===3?'go_default':'ai',calls:role.calls.map(call=>({...call,status:['repaired','partial','no_recommendation','failed','succeeded'][index]}))}))},null)};
   const fs=require('fs'),target=process.env.ARCHIVE_CENTER_HUD_FIXTURE_OUTPUT;
   for(const [name,body] of Object.entries(variants))fs.writeFileSync(name==='current'?target:target.replace(/\.html$/,'.'+name+'.html'),'<!doctype html><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><body style="margin:0;background:#0d0f13;font-family:Arial,sans-serif"><main style="'+TURN_WORKFLOW_HUD_ROOT_STYLE+'">'+body+'</main>');
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
