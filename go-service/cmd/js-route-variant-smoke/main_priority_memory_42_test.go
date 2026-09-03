package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestArchiveCenter42PriorityMemoryAndFinalizationWiringMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	for _, marker := range []string{
		`turnFinalizationMode: "immediate_after_response"`,
		`turn_finalization_mode: settings.turnFinalizationMode || DEFAULT_SETTINGS.turnFinalizationMode`,
		`id="mo-turnFinalizationMode"`,
		`function queueNextInputFinalization(requestContext, sourceAcceptanceFinality)`,
		`function beginNextInputFinalizationPipeline(sessionId, currentRequestContext, hostContext = null)`,
		`contract_version: "source_acceptance_observation.v2"`,
		`finality_source: "risu_next_host_signal_active_chat"`,
		`position_observation: "committed_before_next_host_signal"`,
		`return backfillOneActiveChatCompletedTurn(sid, pair, {`,
		`goFinalizationPolicy.owner === "go"`,
		`goFinalizationMode === "next_user_input"`,
		`return continueAcceptedFinalPersistence(persistenceOrchResult, sourceAcceptanceFinality);`,
		`await loadNextInputFinalizationsFromStorage()`,
	} {
		if !strings.Contains(src, marker) {
			t.Fatalf("Archive Center.js missing 4.2 marker %q", marker)
		}
	}

	serializer := extractJSFunctionBlockForTest(t, src, "function serializeNextInputFinalizationMarker(marker)")
	for _, forbidden := range []string{"user_content:", "assistant_content:", "context_messages:"} {
		if strings.Contains(serializer, forbidden) {
			t.Fatalf("persistent next-input marker stores full turn content via %q", forbidden)
		}
	}

	afterRequest := extractJSFunctionBlockForTest(t, src, "function onAfterRequest(content, type)")
	delayedIndex := strings.Index(afterRequest, `goFinalizationMode === "next_user_input"`)
	immediateIndex := strings.Index(afterRequest, `return continueAcceptedFinalPersistence(persistenceOrchResult, sourceAcceptanceFinality);`)
	if delayedIndex < 0 || immediateIndex < 0 || delayedIndex >= immediateIndex {
		t.Fatal("delayed finalization must branch before the unchanged immediate 4.1 persistence call")
	}

	beforeRequest := extractJSFunctionBlockForTest(t, src, "async function onBeforeRequest(payload, type)")
	if !strings.Contains(beforeRequest, "if (!nextInputFinalizationOwnership.owned)") {
		t.Fatal("next-input exact previous-turn owner does not suppress the automatic broad backfill for that request")
	}
	if strings.Index(beforeRequest, "beginNextInputFinalizationPipeline(") > strings.Index(beforeRequest, "const yumiArchiveReadContext") {
		t.Fatal("previous-turn Critic was not launched before current-request preparation")
	}
}

func TestArchiveCenter42NextInputFinalizationUsesPreviousStableRowWithoutBlocking(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Fatalf("node is required for the 4.2 next-input finalization runtime fixture: %v", err)
		}
	}
	src := readArchiveCenterJS(t)
	functions := strings.Join([]string{
		extractArchiveCenterJSFunction(t, src, "serializeNextInputFinalizationMarker"),
		extractArchiveCenterJSAsyncFunction(t, src, "saveNextInputFinalizationsToStorage"),
		extractArchiveCenterJSAsyncFunction(t, src, "loadNextInputFinalizationsFromStorage"),
		extractArchiveCenterJSFunction(t, src, "queueNextInputFinalization"),
		extractArchiveCenterJSAsyncFunction(t, src, "buildNextInputSourceAcceptanceFinality"),
		extractArchiveCenterJSFunction(t, src, "buildCompletedTurnPairsFromActiveChatMessages"),
		extractArchiveCenterJSFunction(t, src, "beginNextInputFinalizationPipeline"),
	}, "\n")
	script := functions + `
const NEXT_INPUT_FINALIZATION_STORAGE_KEY="next-input-test";
const _nextInputFinalizations=new Map();
let _nextInputFinalizationLoaded=false;
let stored="";
let activeChat=null;
let backfillCalls=0;
let releaseBackfill=null;
let lastFinality=null;
const runtimeUpdates=[];
async function persistentSet(_key,value){stored=String(value||"");}
async function persistentGet(){return stored;}
function warnLog(...args){throw new Error("unexpected warning: "+args.join(" "));}
function debugLog(){}
function updateRuntimeState(name,status,value){runtimeUpdates.push({name,status,value});}
function normalizeAssistantPersistenceCandidate(value){return String(value||"").trim();}
function shouldSkipUserInputPersistence(){return false;}
function getActiveChatMessageStreamingState(){return "not_streaming";}
const AUTO_CONTINUE_USER_INPUT_MARKER="[Continue]";
function computeOrchestrationDirtyHashOr1c(value){
  let hash=5381; const text=String(value||"");
  for(let i=0;i<text.length;i++) hash=((hash*33)^text.charCodeAt(i))>>>0;
  return "or1c:"+hash.toString(16);
}
function extractComparableMessageRoleAndContent(message){
  if(!message) return null;
  const role=message.role==="char" ? "assistant" : message.role;
  return {role,content:String(message.data!=null?message.data:message.content||"")};
}
function extractActiveChatComparableMessages(chat){
  return (chat && Array.isArray(chat.message)?chat.message:[]).map((message,index)=>{
    const comparable=extractComparableMessageRoleAndContent(message);
    return comparable ? {...comparable,risuMessageIndex:index} : null;
  }).filter(Boolean);
}
async function resolveCurrentActiveChatObject(){return {chat:activeChat};}
async function buildCompleteTurnSourceAcceptanceObservation(){
  const assistant=activeChat.message[1];
  const hash=computeOrchestrationDirtyHashOr1c(assistant.data);
  return {
    message_chat_id:String(assistant.chatId||""), message_chat_id_state:assistant.chatId?"observed":"unobserved",
    generation_id:"generation-b", generation_id_state:"observed",
    message_swipe_id:-1, message_swipe_id_state:"not_present",
    message_time_ms:2000, message_time_state:"observed",
    branch_id:"", branch_id_state:"not_exposed_by_risuai",
    observed_content_hash:hash, persistence_content_hash:hash,
  };
}
async function backfillOneActiveChatCompletedTurn(_sid,pair,options){
  backfillCalls++;
  lastFinality=options.sourceAcceptanceFinality;
  if(pair.userContent!=="same input" || pair.assistantContent!=="final B") throw new Error("wrong previous pair: "+JSON.stringify(pair));
  return await new Promise(resolve=>{releaseBackfill=resolve;});
}
async function flush(){for(let i=0;i<8;i++) await Promise.resolve();}
(async function(){
  const firstContext={sessionId:"session-1",requestId:"request-a",requestType:"model",hostChatId:"chat-1",requestMessageCount:1,userMessageIndex:0,userObservedPairOrdinal:1,userMessageChatId:"user-row-1",userMessageTimeMs:1000,userObservedContentHash:computeOrchestrationDirtyHashOr1c("same input")};
  if(!queueNextInputFinalization(firstContext,{persistence_content_hash:computeOrchestrationDirtyHashOr1c("first A")})) throw new Error("first marker was not queued");
  const sameRow=beginNextInputFinalizationPipeline("session-1",{userMessageIndex:0},{});
  if(!sameRow.owned || sameRow.started || sameRow.reason!=="same_user_row_reroll_or_edit") throw new Error("same-row reroll was finalized: "+JSON.stringify(sameRow));
  if(backfillCalls!==0) throw new Error("same-row reroll reached persistence");

  const rerolledContext={...firstContext,requestId:"request-b"};
  if(!queueNextInputFinalization(rerolledContext,{persistence_content_hash:computeOrchestrationDirtyHashOr1c("final B")})) throw new Error("rerolled marker was not queued");
  if(_nextInputFinalizations.get("session-1").requestId!=="request-b") throw new Error("reroll did not replace pending marker");
  await flush();
  if(stored.includes("same input") || stored.includes("final B")) throw new Error("persistent marker leaked full turn text: "+stored);

  activeChat={id:"chat-1",message:[
    {role:"user",data:"same input",chatId:"user-row-1",time:1000},
    {role:"char",data:"final B",chatId:"assistant-row-1",time:2000},
    {role:"user",data:"same input",chatId:"user-row-2",time:3000},
  ]};
  const nextRow=beginNextInputFinalizationPipeline("session-1",{userMessageIndex:2},{});
  if(!nextRow.owned || !nextRow.started) throw new Error("identical-text new row did not start previous finalization: "+JSON.stringify(nextRow));
  if(backfillCalls!==0) throw new Error("previous Critic blocked the synchronous current path");
  await flush();
  if(backfillCalls!==1 || typeof releaseBackfill!=="function") throw new Error("previous pair was not started asynchronously");
  if(!lastFinality || lastFinality.finality_source!=="risu_next_host_signal_active_chat" || lastFinality.next_signal_user_index!==2 || lastFinality.user_message_chat_id!=="user-row-1") {
    throw new Error("next-host finality lost the stable previous row: "+JSON.stringify(lastFinality));
  }
  releaseBackfill({status:"saved",turnIndex:1});
  await flush();
  if(_nextInputFinalizations.has("session-1")) throw new Error("saved previous marker was not consumed");

  queueNextInputFinalization(rerolledContext,{persistence_content_hash:computeOrchestrationDirtyHashOr1c("final B")});
  await flush();
  _nextInputFinalizations.clear(); _nextInputFinalizationLoaded=false;
  if(await loadNextInputFinalizationsFromStorage()!==1 || !_nextInputFinalizations.has("session-1")) throw new Error("restart did not restore pending marker");
  activeChat={...activeChat,id:"different-branch"};
  const branchAttempt=beginNextInputFinalizationPipeline("session-1",{userMessageIndex:2},{});
  if(!branchAttempt.started) throw new Error("branch check did not remain non-blocking");
  await flush();
  if(backfillCalls!==1 || !_nextInputFinalizations.has("session-1")) throw new Error("different branch consumed or saved the pending marker");
})().catch(function(err){console.error(err && err.stack || err);process.exit(1);});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("4.2 next-input finalization runtime fixture failed: %v\n%s", err, output)
	}
}
