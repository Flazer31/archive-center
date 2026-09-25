package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestDeletionReconciliationMaintenanceEntrypoints47(t *testing.T) {
	node := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if node == "" {
		var err error
		node, err = exec.LookPath("node")
		if err != nil {
			t.Fatal(err)
		}
	}
	src := readArchiveCenterJS(t)
	production := extractArchiveCenterJSAsyncFunction(t, src, "normalizeSession") + "\n" + extractArchiveCenterJSAsyncFunction(t, src, "explorerLoadTab")
	script := `
const assert = require('node:assert/strict');
let calls=[];
const _sessionNormalizeState={};
const _explorer={activeTab:'',memories:{items:[]},kgTriples:{items:[]}};
function explorerSessionId(){return 'fixture-session';}
function captureSessionHostContextFromCache(sid){return {sessionId:sid,hostChatId:'fixture-chat'};}
function t(k){return k;}
function alert(message){throw Error(message);}
async function showConfirmModal(_title, message){assert.match(message,/삭제된 끝부분/);return true;}
function refreshExplorerUI(){}
async function reconcileRollbackFromHostSignal(sid, context, options){
 assert.equal(sid,'fixture-session');assert.match(options.reason,/full_active_chat_observed/);
 calls.push('reconcile');await Promise.resolve();calls.push('reconciled');return true;
}
async function computeActiveChatRescanDryRunPlan(sid){
 assert.equal(sid,'fixture-session');assert.equal(calls.at(-1),'reconciled');calls.push('rescan');return {ok:true,pairs:[]};
}
function buildSessionNormalizeRepairEntriesFromDryRunPlan(){return [];}
function buildSessionNormalizeTargetTurnsFromDryRunPlan(){return [];}
function buildAdminRuntimeClientMeta(x){return x;}
async function safeCall(fn){return fn();}
async function bridgeFetch(path,options){assert.equal(path,'/admin/session-normalize');assert.equal(calls.at(-1),'rescan');calls.push('normalize');return {status:'ok'};}
function acceptAdminBackgroundJob(){return false;}
async function read(){assert.equal(calls.at(-1),'reconciled');calls.push('read');}
const explorerFetchChatLogs=read,explorerFetchMemories=read,explorerFetchDirectEvidence=read,explorerFetchKgTriples=read,
 explorerFetchHierarchySummaries=read,explorerFetchTrust=read,explorerFetchWorldGraph=read,explorerFetchEntities=read;
` + production + `
(async()=>{
 assert.equal(await normalizeSession('fixture-session',5,{skipRescan:false}),true);
 assert.deepEqual(calls,['reconcile','reconciled','rescan','normalize']);
 for(const tab of ['chat_logs','memories','direct_evidence','kg_triples','episodes','trust','world','entities']){
  calls=[];await explorerLoadTab(tab,true);assert.deepEqual(calls,['reconcile','reconciled','read'],tab);
 }
})().catch(e=>{console.error(e);process.exitCode=1});
`
	cmd := exec.Command(node, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("maintenance production entrypoints: %v\n%s", err, output)
	}
}

func TestRollbackPendingInputObservationUsesCapturedOwner47(t *testing.T) {
	node := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if node == "" {
		var err error
		node, err = exec.LookPath("node")
		if err != nil {
			t.Fatal(err)
		}
	}
	src := readArchiveCenterJS(t)
	production := extractArchiveCenterJSFunction(t, src, "buildRollbackPendingInputObservation") + "\n" + extractArchiveCenterJSFunction(t, src, "finalConfirmationRequestContextOwnsPendingResponse")
	script := `const assert=require('node:assert/strict');let _activeFinalConfirmationRequestContext=null;` + production + `
const current={sessionId:'A',state:'pending',hostChatId:'host-A',userMessageChatId:'user-A',userMessageIndex:12,userObservedContentHash:'raw-hash'};
let got=buildRollbackPendingInputObservation('A',current);
assert.equal(got.user_message_chat_id,'user-A');assert.equal(got.user_observed_content_hash,'raw-hash');
_activeFinalConfirmationRequestContext=current;
assert.equal(buildRollbackPendingInputObservation('B',null),null);
assert.equal(buildRollbackPendingInputObservation('A',null).user_message_index,12);
current.state='accepted';assert.equal(buildRollbackPendingInputObservation('A',null),null);
`
	cmd := exec.Command(node, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("pending input transport: %v\n%s", err, output)
	}
}
