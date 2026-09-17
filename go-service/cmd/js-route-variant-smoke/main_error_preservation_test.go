package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestErrorPreservationBridgeAndAudit(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Fatal("node required for error preservation regression")
	}
	src := readArchiveCenterJS(t)
	var functions []string
	for _, name := range []string{"extractBridgeErrorDetail", "renderAuditSection", "escapeAttr"} {
		functions = append(functions, extractArchiveCenterJSFunction(t, src, name))
	}
	for _, name := range []string{"bridgeFetch", "auditFetch", "safeCall"} {
		functions = append(functions, extractArchiveCenterJSAsyncFunction(t, src, name))
	}
	script := `
const assert=require('node:assert/strict');
const settings={bridgeUrl:'http://fixture.invalid',requestTimeoutMs:0,webDirectBridgeEnabled:false};
const _lastBridgeFailureByPath=new Map();
const _audit={items:[],total:0,loading:false,expanded:true,error:''};
const debugLog=()=>{},warnLog=()=>{},t=x=>x,explorerSessionId=()=> 'fixture-session';
const resolveRequestTimeoutMs=()=>0;
const resolveBridgeRuntimeRoute=url=>({url,configuredUrl:url,mode:'configured'});
let makeResponse, calls=0;
const R={nativeFetch:async(url,options)=>{
 assert.ok(url.startsWith('http://fixture.invalid/'));assert.equal(options.method,'GET');calls++;
 return makeResponse();
},risuFetch:async(url,options)=>{assert.equal(options.rawResponse,true);calls++;return makeResponse();}};
` + strings.Join(functions, "\n") + `
(async()=>{
 for(const test of [
  {name:'json HTTP error',body:'{"error":{"message":"fixture quota exhausted"}}',status:429,detail:'fixture quota exhausted'},
  {name:'plain HTTP error',body:'fixture database unavailable',status:500,detail:'fixture database unavailable'},
  {name:'JSON null HTTP error',body:'null',status:500,detail:'HTTP 500'},
  {name:'malformed success',body:'{"broken": fixture',status:200,kind:'response_decode_failed'},
 ]) {
  for(const direct of [false,true]) {
   settings.webDirectBridgeEnabled=direct;
   makeResponse=()=>direct?{status:test.status,ok:test.status===200,data:test.body}:new Response(test.body,{status:test.status});
   const before=calls;
   assert.equal(await bridgeFetch('/fixture'),null,test.name);
   assert.equal(calls,before+1,'unexpected retry');
   const error=_lastBridgeFailureByPath.get('/fixture');
   assert.equal(error.response_body,test.body,'original response lost');
   if(test.detail)assert.equal(error.detail,test.detail);
   if(test.kind)assert.equal(error.kind,test.kind);
   assert.ok(!error.response_read_error.includes('already been read'),'body consumed twice');
  }
 }
 settings.webDirectBridgeEnabled=false;
 makeResponse=()=>new Response(new ReadableStream({start(c){c.error(new Error('fixture body interrupted'));}}),{status:200});
 assert.equal(await bridgeFetch('/fixture'),null);
 assert.ok(_lastBridgeFailureByPath.get('/fixture').response_read_error.includes('fixture body interrupted'));
 // Existing JSON-only host shape still works; body handling does not require a new host method.
 makeResponse=()=>({status:500,json:async()=>({error:'fixture json-only error'})});
 assert.equal(await bridgeFetch('/fixture'),null);
 assert.equal(_lastBridgeFailureByPath.get('/fixture').detail,'fixture json-only error');
 for(const factory of [()=>new Response('{"status":"ok"}'),()=>({status:200,json:async()=>({status:'ok'})})]){
  makeResponse=factory;assert.equal((await bridgeFetch('/fixture')).status,'ok');
  assert.equal(_lastBridgeFailureByPath.has('/fixture'),false);
 }
 settings.webDirectBridgeEnabled=true;makeResponse=()=>({status:204,ok:true,data:''});
 assert.equal(await bridgeFetch('/fixture'),null);
 assert.equal(_lastBridgeFailureByPath.has('/fixture'),false,'empty direct response changed');
 settings.webDirectBridgeEnabled=false;
 makeResponse=()=>new Response('fixture audit <read> failed',{status:500});
 await auditFetch();
 assert.ok(_audit.error.includes('fixture audit <read> failed'),'audit cause lost');
 let html=renderAuditSection();
 assert.ok(html.includes('audit.loadFailed')&&!html.includes('audit.empty'),'query failure shown as no records');
 assert.ok(html.includes('&lt;read&gt;')&&!html.includes('<read>'),'raw error HTML injected');
 makeResponse=()=>new Response('{"status":"ok","items":[],"total":0}');
 await auditFetch();assert.equal(_audit.error,'');assert.ok(renderAuditSection().includes('audit.empty'));
 makeResponse=()=>new Response('{"status":"ok","items":[{"id":1}],"total":1}');
 await auditFetch();assert.equal(_audit.items.length,1);assert.equal(_audit.total,1);assert.equal(_audit.error,'');
 console.log('bridge and audit error preservation passed');
})().catch(e=>{console.error(e);process.exitCode=1;});
`
	cmd := exec.Command(node, "-e", script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("production JS error path regression: %v\n%s", err, output)
	}
}
