package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func extractTurnWorkflowHUDStreamIO(t *testing.T, src string) string {
	t.Helper()
	return extractArchiveCenterJSFunction(t, src, "decodeTurnWorkflowHUDStreamLine") + "\n" +
		extractArchiveCenterJSAsyncFunction(t, src, "readTurnWorkflowHUDNDJSON") + "\n"
}

func Test44HUDStreamChunksAndIndependentRequestOwners(t *testing.T) {
	node := os.Getenv("ARCHIVE_CENTER_NODE_BINARY")
	if node == "" {
		var err error
		node, err = exec.LookPath("node")
		if err != nil {
			t.Fatal(err)
		}
	}
	src := readArchiveCenterJS(t)
	production := []string{extractTurnWorkflowHUDStreamIO(t, src), extractArchiveCenterJSFunction(t, src, "turnWorkflowHUDStreamFailure")}
	for _, name := range []string{"consumeTurnWorkflowHUDStreamLine", "consumeTurnWorkflowHUDStream", "consumeTurnWorkflowHUDPreviousStreamLine", "consumeTurnWorkflowHUDPreviousStream"} {
		production = append(production, extractArchiveCenterJSAsyncFunction(t, src, name))
	}
	script := strings.Join(production, "\n") + `
const assert = require('node:assert/strict');
let _turnWorkflowHUDWatchToken=1, _turnWorkflowHUDActiveRequestId='now';
let _turnWorkflowHUDPreviousWatchToken=2, _turnWorkflowHUDPreviousRequestId='before';
let _turnWorkflowHUDRenderChain=Promise.resolve();
const current=[], previous=[];
function consumeTurnWorkflowHUD(view){current.push(view); return true;}
function consumeTurnWorkflowHUDPrevious(view){previous.push(view); return true;}
function stream(text, width=1) {
  const bytes=new TextEncoder().encode(text); let offset=0;
  return {reads:0, async read() {this.reads++; if(offset>=bytes.length)return {done:true};
    const value=bytes.slice(offset,offset+width); offset+=width; return {value,done:false};}};
}
function line(id,status='running'){return JSON.stringify({request_id:id,status,label:'기억 저장'});}
(async()=>{
  // Split every byte, including Korean UTF-8; consume several complete lines
  // and a final event without newline. Both owners run at the same time.
  const [a,b]=await Promise.all([
    consumeTurnWorkflowHUDStream(stream(' \r\n'+line('now')+'\n'+line('now','completed')),1,'now'),
    consumeTurnWorkflowHUDPreviousStream(stream(line('before')+'\n'+line('before','completed_with_warning'),7),2,'before')
  ]);
  assert.equal(a,true); assert.equal(b,true);
  assert.deepEqual(current.map(v=>v.status),['running','completed']);
  assert.deepEqual(previous.map(v=>v.status),['running','completed_with_warning']);
  assert.equal(current[0].label,'기억 저장');
  assert.equal(await consumeTurnWorkflowHUDStream(stream(line('now')),1,'now'),false);
  for(const [read,id,token,message] of [
    [consumeTurnWorkflowHUDStream,'now',1,'invalid NDJSON event'],
    [consumeTurnWorkflowHUDPreviousStream,'before',2,'invalid previous-turn NDJSON event']]) {
    await assert.rejects(read(stream('{"request_id":'),token,id),e=>e.code==='stream_decode_failed'&&e.message===message);
    await assert.rejects(read(stream(line('wrong')),token,id),e=>e.code==='stream_request_mismatch');
  }
  // Cancellation while a read is pending: neither the stale view nor an old
  // request can land on the new current card; the previous owner stays valid.
  const n=current.length;
  const stale={reads:0,async read(){this.reads++;_turnWorkflowHUDWatchToken=3;return {value:new TextEncoder().encode(line('now')+'\n'),done:false};}};
  assert.equal(await consumeTurnWorkflowHUDStream(stale,1,'now'),true);
  assert.equal(stale.reads,1); assert.equal(current.length,n);
  _turnWorkflowHUDActiveRequestId='next';
  const old=stream(line('now')); assert.equal(await consumeTurnWorkflowHUDStream(old,3,'now'),true);
  assert.equal(old.reads,0);
  assert.equal(await consumeTurnWorkflowHUDPreviousStream(stream(line('before','completed')),2,'before'),true);
  assert.equal(previous.length,3);
  process.stdout.write('ok');
})().catch(e=>{console.error(e);process.exitCode=1;});
`
	cmd := exec.Command(node, "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("HUD stream fixture: %v\n%s", err, out)
	}
}
