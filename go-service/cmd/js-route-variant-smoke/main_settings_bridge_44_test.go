package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestSettings44OverlappingBridgeActionsKeepRequestSettings(t *testing.T) {
	src := readArchiveCenterJS(t)
	var functions []string
	for _, name := range []string{"sanitizeNumber", "sanitizeBridgeUrl", "isValidBridgeUrlInput", "parseBridgeUrl", "getBrowserPageHostInfo", "normalizeHostForCompare", "isLoopbackHost", "resolveBridgeRuntimeRoute", "getRequestTimeoutSettingMs", "resolveRequestTimeoutMs", "summarizeBridgeReadiness"} {
		functions = append(functions, extractArchiveCenterJSFunction(t, src, name))
	}
	for _, name := range []string{"bridgeFetch", "testBridgeHealth", "testSupervisorWakeup", "checkArchiveCenterUpdate", "applyArchiveCenterUpdate"} {
		functions = append(functions, extractArchiveCenterJSAsyncFunction(t, src, name))
	}
	// These nested production helpers capture the settings form's DOM lookup.
	start := strings.Index(src, "      function getCurrentUiRequestTimeoutMs()")
	if start < 0 {
		t.Fatal("settings form helpers missing")
	}
	end := strings.Index(src[start:], "      const reasoningSyncRunners")
	if start < 0 || end < 0 {
		t.Fatal("settings form helper boundary missing")
	}
	functions = append(functions, src[start:start+end])
	for _, block := range []struct{ name, start, end string }{
		{"bindHealthTest", `$("mo-test-health")?.addEventListener`, "// 출판사 LLM 호출 테스트"},
		{"bindUpdateTest", `const updateDownloadBtn = $("mo-update-download");`, "void refreshArchiveCenterUpdateStatus();"},
	} {
		a := strings.Index(src, block.start)
		if a < 0 {
			t.Fatalf("button binding missing: %s", block.name)
		}
		b := strings.Index(src[a:], block.end)
		if b < 0 {
			t.Fatalf("button binding end missing: %s", block.name)
		}
		functions = append(functions, "function "+block.name+"(){\n"+src[a:a+b]+"\n}")
	}
	script := `
const assert = require('node:assert/strict');
const DEFAULT_SETTINGS = {bridgeUrl:'http://saved.test:28080',requestTimeoutMs:30000};
let settings, form, pending, timers;
const archiveUpdateState = {}, _lastBridgeFailureByPath = new Map();
const $ = id => form[id] || null;
const window = {location:{hostname:'fixture.test',protocol:'https:'}};
const debugLog = () => {}, warnLog = () => {}, updateRuntimeState = () => {};
const t = x=>x, escapeAttr=x=>String(x), renderBridgeHealthSummary=()=>'', formatArchiveUpdateResult=()=>'';
const safeCall = async (fn, fallback) => { try { return await fn(); } catch { return fallback; } };
// Only host networking and timer scheduling are replaced; production routing,
// timeout resolution, health sequence and response decoding execute below.
const setTimeout = (fn, ms) => { const timer={fn,ms}; timers.push(timer); return timer; };
const clearTimeout = timer => { timer.cleared=true; };
const host = (mode,url,options) => new Promise(resolve => pending.push({mode,url,options,resolve}));
const R = {nativeFetch:(url,opts)=>host('native',url,opts),risuFetch:(url,opts)=>host('direct',url,opts)};
const flush = () => new Promise(resolve => setImmediate(resolve));
function answer(index, data = {status:'ok',ready:true,store_ready:true,vector_ready:true}) {
 const call=pending[index];
 call.resolve(call.mode==='direct' ? {status:200,ok:true,data} : {status:200,ok:true,json:async()=>data});
}
function edit(url,timeout,direct) {
 form={'mo-bridgeUrl':{value:url},'mo-requestTimeoutMs':{value:timeout},'mo-webDirectBridgeEnabled':{checked:direct}};
}
function clickHealth() {
 const button={addEventListener:(name,handler)=>{assert.equal(name,'click');button.click=handler;}};
 form['mo-test-health']=button;form['mo-test-result']={innerHTML:''};
 bindHealthTest();return button.click();
}
` + strings.Join(functions, "\n") + `
(async()=>{
 for (const reverse of [false,true]) {
  settings={...DEFAULT_SETTINGS,webDirectBridgeEnabled:false}; pending=[];timers=[];
  const original=JSON.stringify(settings);
  edit('http://a.test:28081',11000,false);
  const a=clickHealth();
  edit('http://b.test:28082',22000,true);
  const b=clickHealth();
  const normal=bridgeFetch('/ordinary');
  assert.equal(JSON.stringify(settings),original,'UI tests changed saved global settings');
  assert.deepEqual(pending.map(x=>[x.url,x.mode]),[
   ['http://a.test:28081/health','native'],['http://b.test:28082/health','direct'],['http://saved.test:28080/ordinary','native']]);
  assert.deepEqual(timers.map(x=>x.ms),[11000,22000,30000]);
  const first=reverse?1:0, second=reverse?0:1;
  answer(first); await flush();
  assert.equal(pending[3].url,(reverse?'http://b.test:28082':'http://a.test:28081')+'/ready');
  assert.equal(timers[3].ms,reverse?22000:11000);
  // A settings save during the outstanding requests must survive completion.
  settings.bridgeUrl='http://new-saved.test:28083';
  answer(3); await flush(); answer(second); await flush();
  assert.equal(pending[4].url,(reverse?'http://a.test:28081':'http://b.test:28082')+'/ready');
  answer(4); answer(2); await Promise.all([a,b,normal]);
  assert.equal(settings.bridgeUrl,'http://new-saved.test:28083');
  assert.ok(timers.every(x=>x.cleared),'request timer not cleared');
 }
 // Empty address retains saved address; UI timeout and explicit no-timeout
 // update requests keep their existing meanings without mutating saved values.
 pending=[];timers=[]; edit('',17000,true);
 const wake=withUiBridgeSettings(bridgeSettings=>testSupervisorWakeup(bridgeSettings));
 assert.equal(pending[0].url,'http://new-saved.test:28083/wakeup');
 assert.equal(pending[0].options.requestTimeoutMs,17000);answer(0);await wake;
 for(const action of [checkArchiveCenterUpdate,applyArchiveCenterUpdate]) {
  const button={addEventListener:(name,handler)=>{assert.equal(name,'click');button.click=handler;}};
  form['mo-update-download']=button;form['mo-test-result']={innerHTML:''};bindUpdateTest();
  const result=action===applyArchiveCenterUpdate?button.click():withUiBridgeSettings(bridgeSettings=>action(bridgeSettings));
  const call=pending.at(-1);
  assert.equal(call.options.requestTimeoutMs,undefined);
  assert.ok(call.url.startsWith('http://new-saved.test:28083/update/'));
  answer(pending.length-1);await result;
 }
 assert.equal(timers.length,1,'explicit timeout 0 acquired a UI timeout');
 // The existing timeout failure remains local to this request.
 edit('http://timeout.test:28084',18000,true);
 const failed=withUiBridgeSettings(bridgeSettings=>testSupervisorWakeup(bridgeSettings));
 timers.at(-1).fn();assert.equal(await failed,null);
 assert.equal(_lastBridgeFailureByPath.get('/wakeup').configured_url,'http://timeout.test:28084');
 assert.equal(_lastBridgeFailureByPath.get('/wakeup').timeout_ms,18000);
 assert.equal(settings.bridgeUrl,'http://new-saved.test:28083');
})().catch(err=>{console.error(err);process.exitCode=1;});
`
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		nodePath = "node"
	}
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("settings bridge regression: %v\n%s", err, out)
	}
}
