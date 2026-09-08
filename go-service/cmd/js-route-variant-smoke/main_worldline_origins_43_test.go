package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestWorldline43HostOriginReadAndRouting(t *testing.T) {
	src := readArchiveCenterJS(t)
	functions := strings.Join([]string{
		extractArchiveCenterJSFunction(t, src, "buildRisuWorldlineObservationFromMessages"),
		extractArchiveCenterJSAsyncFunction(t, src, "observeRisuWorldlineMessageOrigins"),
		extractArchiveCenterJSAsyncFunction(t, src, "requestBackendSessionRoutingTurnResolution"),
	}, "\n")
	script := `
const assert=require('node:assert/strict');
const debugLog=()=>{};
const getRequestTimeoutSettingMs=()=>1000;
const serializeSessionRoutingBaselineForBackend=()=>null;
const getSessionRoutingTurnBaseline=()=>null;
let _sessionCache=null;
let reads=0,calls=[],failure='',ancestor=false;
const root={id:'root',message:[{role:'user',chatId:'u',data:'PRIVATE user'}, {role:'char',chatId:'a',data:'PRIVATE assistant'}]};
const branch={id:'branch',message:[{role:'user',chatId:'b-u',data:'PRIVATE user'},{role:'char',chatId:'b-a',data:'PRIVATE assistant'}, {role:'char',chatId:'marker',disabled:true,data:'{{specialcomment::branchedfrom::root::Parent::a::}}'}]};
let character={chaId:'stable',chats:[root,branch,{id:'unrelated',message:[{chatId:'a',data:'WRONG'}]}]};
const R={getCharacterFromIndex:async index=>{ assert.equal(index,2);reads++;return character; }};
const plan={parent_host_chat_id:'branch',source_message_id:'b-a',child_host_chat_id:'child',child_marker_index:2};
const ok={status:'ok',contract_version:'session-routing.turn-resolution.v1',chat_session_id:'C',resolution:'normal',binding_acknowledged:true};
async function bridgeFetch(path,options) {
 assert.equal(path,'/session-routing/turn-resolution'); calls.push(options.body);
 assert(!JSON.stringify(options.body).includes('PRIVATE')); assert(!JSON.stringify(options.body).includes('WRONG'));
 if(calls.length===1) return {...ok,worldline:{state:'confirmed',origin_read_request:plan}};
 if(failure==='throw') throw new Error('network');
 if(failure==='error') return {status:'error'};
 const origins=options.body.worldline_observation.message_origins;
 assert.equal(origins.parent_messages[0].message_chat_id,'b-u');
 assert.equal(origins.child_messages[0].message_chat_id,'c-u');
 if(ancestor && calls.length===2) return {...ok,worldline:{state:'unresolved',origin_read_request:{parent_host_chat_id:'root',source_message_id:'a',child_host_chat_id:'branch',child_marker_index:2,ancestor_depth:1}}};
 if(ancestor) {
   assert.equal(origins.parent_observation.message_origins.parent_messages[0].message_chat_id,'u');
   assert.equal(origins.parent_observation.message_origins.child_messages[0].message_chat_id,'b-u');
   assert.equal(origins.parent_observation.host_signal_source,'active_chat_pre_backfill');
   assert(Number.isFinite(origins.parent_observation.observed_at_ms));
 }
 return {...ok,worldline:{state:'confirmed',message_origins_recorded:true}};
}
` + functions + `
(async()=>{
 const child=[{role:'user',chatId:'c-u'},{role:'char',chatId:'c-a'},{role:'char',disabled:true,data:'{{specialcomment::branchedfrom::branch::Parent::b-a::}}'}];
 const observed={stableCharacterId:'stable',hostChatId:'child',worldlineObservation:buildRisuWorldlineObservationFromMessages(child,Date.now(),'output'),worldlineHostContext:{characterIndex:2,childMessages:child.slice(0,2).map((m,i)=>({message_index:i,role:m.role,message_chat_id:m.chatId,disabled:false}))}};
 const first=await requestBackendSessionRoutingTurnResolution('C','identity',observed);
 assert.equal(first.worldline.message_origins_recorded,true);assert.equal(calls.length,2);assert.equal(reads,1);
 assert.equal(calls[0].worldline_observation.branch_marker,observed.worldlineObservation.branch_marker);
 assert(!calls[0].worldline_observation.message_origins,'first bounded request grew into a full prefix');
 assert(!observed.worldlineObservation.message_origins,'frozen observation mutated');
 const originalBridge=bridgeFetch;
 calls=[];reads=0;
 _sessionCache={sessionId:'C',stableCharacterId:'stable',observedChatUniqueId:'child'};
 bridgeFetch=async(...args)=>{
   const response=await originalBridge(...args);
   _sessionCache={sessionId:'another-session',stableCharacterId:'another-character'};
   return response;
 };
 await requestBackendSessionRoutingTurnResolution('C','identity',{...observed,stableCharacterId:''});
 assert.equal(calls.length,2);
 assert.equal(calls[1].stable_character_id,'stable','origin follow-up changed the captured character');
 bridgeFetch=originalBridge;_sessionCache=null;
 calls=[];reads=0;ancestor=true;
 const nested=await requestBackendSessionRoutingTurnResolution('C','identity',observed);
 assert.equal(nested.worldline.state,'confirmed');assert.equal(calls.length,3);assert.equal(reads,2);
 ancestor=false;
 for(failure of ['throw','error']) {
   calls=[];const got=await requestBackendSessionRoutingTurnResolution('C','identity',observed);
   assert.equal(got.worldline.state,'confirmed','optional metadata failure lost original route');assert.equal(calls.length,2);
 }
 failure='';calls=[];character={chaId:'stable',chats:[]};
 const unavailable=await requestBackendSessionRoutingTurnResolution('C','identity',observed);
 assert.equal(unavailable.worldline.state,'confirmed');assert.equal(calls.length,1);
 assert.equal(buildRisuWorldlineObservationFromMessages(root.message,Date.now(),'output'),null,'ordinary copy invented a marker');
 reads=0;calls=[];
 bridgeFetch=async()=>({...ok,worldline:{state:'confirmed',message_origins_recorded:true}});
 await requestBackendSessionRoutingTurnResolution('C','identity',observed);
 assert.equal(reads,0,'durable origin reuse read the host again');
})().catch(err=>{console.error(err);process.exitCode=1;});
`
	node := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if node == "" {
		node = "node"
	}
	cmd := exec.Command(node, "-")
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("production worldline bridge: %v\n%s", err, out)
	}
}
