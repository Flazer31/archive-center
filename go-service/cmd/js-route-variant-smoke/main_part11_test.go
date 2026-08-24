package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/httpapi"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func TestSourceDiscoveryRendersRecordedBridgeFailure(t *testing.T) {
	src := readArchiveCenterJS(t)
	fn := extractArchiveCenterJSAsyncFunction(t, src, "referenceDiscoveryRunFromUI")
	for _, expected := range []string{
		`_lastBridgeFailureByPath.get("/source-discovery/preview/v1")`,
		`_lastBridgeFailureByPath.get("/source-discovery/jobs/v1")`,
		`String(failure.detail`,
	} {
		if !strings.Contains(fn, expected) {
			t.Fatalf("source discovery transport failure detail missing %q", expected)
		}
	}
}

func TestGuideNoneSkipsSupervisorCallRuntime(t *testing.T) {
	src := readArchiveCenterJS(t)
	if strings.Contains(src, "await runSupervisor(") || strings.Contains(src, `bridgeFetch("/supervisor"`) {
		t.Fatal("JavaScript must not perform a supervisor call outside /prepare-turn")
	}
	for _, marker := range []string{
		`const guideDisabled = normalizeNarrativeGuideStrength(settings.narrativeGuideStrength) === "none";`,
		`supervisor_enabled: !guideDisabled`,
		`guide_strength: settings.narrativeGuideStrength || "weak"`,
		`publisher_guidance_format: settings.publisherGuidanceFormat || DEFAULT_SETTINGS.publisherGuidanceFormat`,
	} {
		if !strings.Contains(src, marker) {
			t.Fatalf("guide-off /prepare-turn gate marker missing %q", marker)
		}
	}
}

func TestLanguageContextIgnoresRisuPromptScaffoldAssistantRuntime(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for language-source runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	script := extractArchiveCenterJSFunction(t, src, "normalizeLanguageCodeForTrace") + "\n" +
		extractArchiveCenterJSFunction(t, src, "detectTextLanguageForTrace") + "\n" +
		extractArchiveCenterJSFunction(t, src, "detectRecentAssistantOutputLanguage") + "\n" +
		extractArchiveCenterJSFunction(t, src, "isSupportedMemoryLanguageCode") + "\n" +
		extractArchiveCenterJSFunction(t, src, "buildLanguageFallbackChain") + "\n" +
		extractArchiveCenterJSAsyncFunction(t, src, "buildLanguageContextTrace") + `
const LANGUAGE_MEMORY_CONTRACT_VERSION = "language_memory.v1";
const LANGUAGE_MEMORY_SEARCH_TEXT_POLICY = "summary_plus_raw_plus_aliases";
const settings = {uiLanguage:"en"};
function getPayloadMessageRoleAndText(message) { return {role:String(message.role||""),text:String(message.content||"")}; }
function isMetaPromptLikeMessage(text) { return /^system\s*:/i.test(String(text||"").trim()); }
async function resolveRuntimeOutputLanguageOverride() { return ""; }
function normalizeLanguageContextTrace(value) { return value; }
(async function() {
  const scaffoldOnly = await buildLanguageContextTrace({
    userInput:"한얼은 숯불에 손을 다쳤다.",
    messages:[{role:"assistant",content:"system: POV instructions. Respond in Korean after reviewing these English instructions."}],
    stage:"beforeRequest"
  });
  if (scaffoldOnly.session_output_language !== "ko" || scaffoldOnly.output_language_source !== "current_user") {
    throw new Error("Risu scaffold contaminated language source: "+JSON.stringify(scaffoldOnly));
  }
  const realAssistant = await buildLanguageContextTrace({
    userInput:"한국어로 쓴 입력",
    messages:[{role:"assistant",content:"The previous actual assistant response remains in English."}],
    stage:"beforeRequest"
  });
  if (realAssistant.session_output_language !== "en" || realAssistant.output_language_source !== "recent_assistant") {
    throw new Error("real assistant continuity language was not preserved: "+JSON.stringify(realAssistant));
  }
  const taggedKorean = await buildLanguageContextTrace({
    userInput:"한얼이 아영에게 자신의 이름과 직책을 알려준다.",
    assistantContent:"<Narration><Suit.Natural><Emotion.Calm>아영은 한얼의 설명을 듣고 고개를 끄덕였다. 이제 그의 이름과 맡은 업무를 분명히 알게 되었다.</Emotion.Calm></Suit.Natural></Narration>",
    messages:[],
    stage:"completeTurn"
  });
  if (taggedKorean.session_output_language !== "ko" || taggedKorean.output_language_source !== "current_assistant" || taggedKorean.assistant_output_language !== "ko") {
    throw new Error("Risu rendering tags contaminated Korean output language: "+JSON.stringify(taggedKorean));
  }
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("language-source JS fixture failed: %v\n%s", err, out)
	}
}

func TestFinalPayloadParitySeparatesActualUserFromRisuPromptTailRuntime(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Fatalf("node is required for final-payload parity runtime fixture; set ARCHIVE_CENTER_NODE_BINARY: %v", err)
		}
	}
	src := readArchiveCenterJS(t)
	functions := strings.Join([]string{
		extractArchiveCenterJSFunction(t, src, "computeOrchestrationDirtyHashOr1c"),
		extractArchiveCenterJSFunction(t, src, "truncPreview"),
		extractArchiveCenterJSFunction(t, src, "isBoundaryOnlyUserInput"),
		extractArchiveCenterJSFunction(t, src, "isRisuPromptScaffoldMessage"),
		extractArchiveCenterJSFunction(t, src, "isMetaUserMessage"),
		extractArchiveCenterJSFunction(t, src, "normalizeRollbackMessageRole"),
		extractArchiveCenterJSFunction(t, src, "extractMessageContentCandidate"),
		extractArchiveCenterJSFunction(t, src, "extractComparableMessageRoleAndContent"),
		extractArchiveCenterJSFunction(t, src, "auxiliaryMessageContentText"),
		extractArchiveCenterJSFunction(t, src, "getPayloadMessageRoleAndText"),
		extractArchiveCenterJSFunction(t, src, "isChatMessageLike"),
		extractArchiveCenterJSFunction(t, src, "isChatMessageArray"),
		extractArchiveCenterJSFunction(t, src, "getPayloadPathValue"),
		extractArchiveCenterJSFunction(t, src, "buildPayloadPathRebuilder"),
		extractArchiveCenterJSFunction(t, src, "findPayloadMessagesPath"),
		extractArchiveCenterJSFunction(t, src, "extractMessages"),
		extractArchiveCenterJSFunction(t, src, "findLastPayloadMessage"),
		extractArchiveCenterJSFunction(t, src, "buildFinalPayloadParityTrace"),
	}, "\n")
	script := functions + `
const settings = {pluginMainApplyMode:"shadow",pluginMainRewriteOptIn:false};
const emptyPlan = {contract_version:"payload_application_plan.v1",owner:"go",apply_rule:"apply_exact_text_without_reassembly",status:"empty",auxiliary_text:"",input_context_text:""};
const emptyObservation = {contract_version:"payload_application_observation.v1",status:"ready",payload_application_status:"empty",blocks:[]};
const auxPlan = {contract_version:"payload_application_plan.v1",owner:"go",apply_rule:"apply_exact_text_without_reassembly",status:"ready",auxiliary_text:"required auxiliary",input_context_text:"",auxiliary_observation_hash:"or1c_aux"};
const missingObservation = {contract_version:"payload_application_observation.v1",status:"ambiguous",payload_application_status:"missing",blocks:[{key:"auxiliary_context",status:"missing",hash_match:false}]};
const appliedObservation = {contract_version:"payload_application_observation.v1",status:"ready",payload_application_status:"applied",blocks:[{key:"auxiliary_context",status:"applied",hash_match:true,planned_content_hash:"or1c_aux"}]};
const payload = [{role:"user",content:"system: POV instructions and host prompt"}];
const trace = buildFinalPayloadParityTrace(payload, payload, {
  chatSessionId:"session-copy", userInputSource:"active_chat:0", effectiveUserInput:"한얼은 숯불에 손을 다쳤다.",
  applyMode:{mode:"shadow",payloadReplaced:false},payloadMutated:false,
  injectionResult:{payloadApplicationPlan:emptyPlan,payloadApplicationObservation:emptyObservation}
});
if (trace.finalUserInputPreview !== "한얼은 숯불에 손을 다쳤다.") throw new Error("actual user preview was replaced by host prompt: "+JSON.stringify(trace));
if (trace.payloadUserRoleTailKind !== "different_user_role_message") throw new Error("different user-role tail was not observed separately: "+JSON.stringify(trace));
if (!trace.capturedBeforeRequestReturn || !trace.outboundPayloadHash) throw new Error("pre-request fingerprint missing: "+JSON.stringify(trace));
if (trace.status !== "mismatch" || trace.payloadContentMatch !== false) throw new Error("missing actual user was accepted: "+JSON.stringify(trace));
const mismatch = buildFinalPayloadParityTrace(payload, payload, {
  effectiveInputText:"required auxiliary", effectiveUserInput:"actual user",
  injectionResult:{payloadApplicationPlan:auxPlan,payloadApplicationObservation:missingObservation}
});
if (mismatch.status !== "mismatch" || mismatch.payloadContentMatch !== false) throw new Error("missing payload component was accepted: "+JSON.stringify(mismatch));
const matchedPayload = [{role:"system",content:"host scaffold\nrequired auxiliary"},{role:"user",content:"actual user"}];
const matched = buildFinalPayloadParityTrace(payload, matchedPayload, {
  effectiveInputText:"actual user\n\nrequired auxiliary", effectiveUserInput:"actual user",
  injectionResult:{payloadApplicationPlan:auxPlan,payloadApplicationObservation:appliedObservation}
});
if (matched.status !== "ready" || matched.payloadContentMatch !== true) throw new Error("present payload component was rejected: "+JSON.stringify(matched));
if (!matched.effectiveInputHash || !matched.outboundPayloadHash) throw new Error("non-empty verified input fingerprint missing: "+JSON.stringify(matched));
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("final-payload parity JS fixture failed: %v\n%s", err, out)
	}
}

func TestEffectiveInputUsesCompletePayloadPlanAndCurrentTurnRuntime(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Fatalf("node is required for effective-input production fixture; set ARCHIVE_CENTER_NODE_BINARY: %v", err)
		}
	}
	src := readArchiveCenterJS(t)
	for _, marker := range []string{
		`payloadApplicationPlan: injectionResult.payloadApplicationPlan || null`,
		`payloadApplicationObservation: injectionResult.payloadApplicationObservation || null`,
		`const finalUserText = backendPreview && typeof backendPreview.final_user_text === "string"`,
	} {
		if !strings.Contains(src, marker) {
			t.Fatalf("production transparency wiring missing %q", marker)
		}
	}
	functions := strings.Join([]string{
		extractArchiveCenterJSFunction(t, src, "computeOrchestrationDirtyHashOr1c"),
		extractArchiveCenterJSFunction(t, src, "truncPreview"),
		extractArchiveCenterJSFunction(t, src, "isBoundaryOnlyUserInput"),
		extractArchiveCenterJSFunction(t, src, "isRisuPromptScaffoldMessage"),
		extractArchiveCenterJSFunction(t, src, "isMetaUserMessage"),
		extractArchiveCenterJSFunction(t, src, "normalizeRollbackMessageRole"),
		extractArchiveCenterJSFunction(t, src, "extractMessageContentCandidate"),
		extractArchiveCenterJSFunction(t, src, "extractComparableMessageRoleAndContent"),
		extractArchiveCenterJSFunction(t, src, "auxiliaryMessageContentText"),
		extractArchiveCenterJSFunction(t, src, "getPayloadMessageRoleAndText"),
		extractArchiveCenterJSFunction(t, src, "isChatMessageLike"),
		extractArchiveCenterJSFunction(t, src, "isChatMessageArray"),
		extractArchiveCenterJSFunction(t, src, "getPayloadPathValue"),
		extractArchiveCenterJSFunction(t, src, "buildPayloadPathRebuilder"),
		extractArchiveCenterJSFunction(t, src, "findPayloadMessagesPath"),
		extractArchiveCenterJSFunction(t, src, "extractMessages"),
		extractArchiveCenterJSFunction(t, src, "sanitizeEnumValue"),
		extractArchiveCenterJSFunction(t, src, "normalizeAuxiliaryInjectionPlacement"),
		extractArchiveCenterJSFunction(t, src, "normalizeAuxiliaryInjectionAnchorMarker"),
		extractArchiveCenterJSFunction(t, src, "findFirstSystemInsertionIndex"),
		extractArchiveCenterJSFunction(t, src, "findLatestUserInsertionIndex"),
		extractArchiveCenterJSFunction(t, src, "findAnchorMarkerInsertionIndex"),
		extractArchiveCenterJSFunction(t, src, "findLastCachePointInsertionIndex"),
		extractArchiveCenterJSFunction(t, src, "resolveAuxiliaryInjectionPlacement"),
		extractArchiveCenterJSFunction(t, src, "injectAuxiliaryBlock"),
		extractArchiveCenterJSFunction(t, src, "observeGoPayloadApplication"),
		extractArchiveCenterJSFunction(t, src, "applyGoPayloadApplicationPlan"),
		extractArchiveCenterJSFunction(t, src, "isBackendEffectiveInputPreview"),
		extractArchiveCenterJSFunction(t, src, "composeEffectiveInputFromTransparency"),
		extractArchiveCenterJSFunction(t, src, "findLastPayloadMessage"),
		extractArchiveCenterJSFunction(t, src, "buildFinalPayloadParityTrace"),
		extractArchiveCenterJSFunction(t, src, "resolveLatestTransparencyTrace"),
		extractArchiveCenterJSFunction(t, src, "renderEffectiveInputSection"),
		extractArchiveCenterJSAsyncFunction(t, src, "buildCompleteTurnRequestBody"),
	}, "\n")
	script := functions + `
const AUXILIARY_INJECTION_PLACEMENT_OPTIONS = Object.freeze(["auto","before_latest_user","after_anchor_marker","after_last_cache_point","after_first_system","end"]);
const DEFAULT_SETTINGS = {auxiliaryInjectionPlacement:"before_latest_user"};
const settings = {auxiliaryInjectionPlacement:"before_latest_user",auxiliaryInjectionAnchorMarker:"",pluginMainApplyMode:"shadow",pluginMainRewriteOptIn:false,maxInputContextChars:12000};
const RECOMPOSER_BRIDGE_CONTRACT = "archive_center_recomposer_bridge.v1";
const AUTO_CONTINUE_USER_INPUT_MARKER = "[Continue]";
function updateRuntimeState() {}
function warnLog() {}
function debugLog() {}
function publishArchiveCenterRecomposerBridge() { return false; }
function t(key) { return key; }
function formatLanguageContextBlock() { return ""; }
function renderItBlock(title,text) { return "<BLOCK title=\""+title+"\">"+text+"</BLOCK>"; }
async function resolveRuntimeOutputLanguageOverride() { return ""; }
async function buildLanguageContextTrace() { return {}; }
async function buildCompleteTurnSourceAcceptanceObservation() { return {finality_source:"active_chat"}; }
async function observeRisuPersona() { return {}; }
function buildSourceToFinalLineageObservation() { return null; }
function buildRisuRequestObservation() { return {}; }
function normalizeLanguageContextTrace(value) { return value || {}; }
function buildRisuActiveChatContextMessageObservation(value) { return value; }
function assert(condition,message) { if(!condition) throw new Error(message); }
let lastTurnTrace=null;
let lastOrchResult=null;
let _effectiveInputAwaitingNewTurn=false;
function plan(auxiliary,input,status) {
  return {
    contract_version:"payload_application_plan.v1", owner:"go",
    apply_rule:"apply_exact_text_without_reassembly", status:status,
    auxiliary_text:auxiliary, input_context_text:input,
    auxiliary_observation_hash:auxiliary ? computeOrchestrationDirtyHashOr1c("[Archive Center \u2014 Auxiliary Context]\n\n"+auxiliary) : null,
    input_context_observation_hash:input ? computeOrchestrationDirtyHashOr1c("[Archive Center \u2014 Input Context]\n\n"+input) : null,
    auxiliary_chars:auxiliary.length, input_context_chars:input.length, lanes:[]
  };
}
function apply(payload,payloadPlan) {
  return applyGoPayloadApplicationPlan(payload,{
    _injectionPack:{payload_application_plan:payloadPlan},
    _sourceToPayloadLineage:{lineage_id:"lineage",payload_plan_id:"plan",source_refs:[],execution_items:[]},
    _trace:{}
  },{});
}
function transparency(user,applied) {
  return {
    backendEffectiveInputPreview:{contract_version:"effective_input_preview.v1",final_user_text:user},
    inputContext:applied.injectionResult.inputContext,
    injection:Object.assign({},applied.injectionResult)
  };
}
function parity(payload,applied,user,it) {
  const effective=composeEffectiveInputFromTransparency(it);
  return buildFinalPayloadParityTrace(payload,applied.payload,{
    effectiveInputText:effective,effectiveUserInput:user,payloadMutated:applied.payload!==payload,
    applyMode:{mode:"shadow",payloadReplaced:false},injectionResult:it.injection
  });
}
async function complete(user,it,finalParity) {
  return buildCompleteTurnRequestBody(0,user,"assistant",[],"session",null,{
    orchestrationResult:{_trace:{_inputTransparency:it,finalPayloadParity:finalParity}}
  });
}
(async function() {
  const user="  FIRST TURN USER \u2728 "+("full-user-\uD55C\uAE00\uD83E\uDDED".repeat(100))+"  \n";
  assert(user.length>1000,"fixture user must exceed the old transparency preview boundary");
  const firstPayload=[
    {role:"system",content:"host"},
	{role:"user",content:[{type:"text",text:user},{type:"image_url",image_url:{url:"data:image/png;base64,AAAA"}}]},
    {role:"user",content:"system: Risu scaffold contains raw input but is not the exact final user: "+user}
  ];
  const firstApplied=apply(firstPayload,plan("","","empty"));
  const firstInput=transparency(user,firstApplied);
  const firstEffective=composeEffectiveInputFromTransparency(firstInput);
  assert(firstEffective===user,"first-turn user-only effective input was empty or changed");
  const firstParity=parity(firstPayload,firstApplied,user,firstInput);
  assert(firstParity.status==="ready" && firstParity.payloadContentMatch===true,"first-turn user-only payload was withheld");
  const firstBody=await complete(user,firstInput,firstParity);
  assert(firstBody.client_meta.effective_input_observation.effective_input===user,"first-turn user-only observation was not saved");
  lastOrchResult={_trace:{_inputTransparency:firstInput,finalPayloadParity:firstParity}};
  const firstTurnHTML=renderEffectiveInputSection();
  assert(firstTurnHTML.includes(user) && !firstTurnHTML.includes("withheld"),"first-turn user-only input was hidden in the UI");

  const secondUser="TURN_TWO_CURRENT_MARKER";
  const secondPayload=[{role:"user",content:secondUser}];
  const secondApplied=apply(secondPayload,plan("","","empty"));
  const secondInput=transparency(secondUser,secondApplied);
  const secondParity=parity(secondPayload,secondApplied,secondUser,secondInput);
  lastTurnTrace={_inputTransparency:firstInput,finalPayloadParity:firstParity};
  lastOrchResult={_trace:{_inputTransparency:secondInput,finalPayloadParity:secondParity}};
  assert(resolveLatestTransparencyTrace()===lastOrchResult._trace,"stale completed trace won over current in-flight trace");
  const firstHTML=renderEffectiveInputSection();
  assert(firstHTML.includes(secondUser) && !firstHTML.includes("withheld"),"current in-flight actual user was hidden in the UI");
  assert(!firstHTML.includes("FIRST TURN USER"),"stale prior turn was rendered");

  const marker="_GUIDANCE_FINAL_\uB05D\uD83D\uDE80";
  const referenceLane="REFERENCE_FULL_MARKER";
  const memoryLane="MEMORY_START_"+("memory\uD55C\uAE00\uD83E\uDDED".repeat(180));
  const loreLane="LOREBOOK_FULL_MARKER";
  const guidanceLane="GUIDANCE_START_"+marker;
  const longAux=[referenceLane,memoryLane,loreLane,guidanceLane].join("\n\n");
  const inputContext="INPUT_CONTEXT_FULL_\uC7A5\uBA74";
  assert(longAux.indexOf(marker)>500,"fixture marker must be beyond the old preview boundary");
  const longPlan=plan(longAux,inputContext,"ready");
  longPlan.lanes=[
    {key:"original_work",title:"Original Work Context",text:referenceLane,applied:true,status:"applied"},
    {key:"long_term_memory",title:"Long-term Memory Context",text:memoryLane,applied:true,status:"applied"},
    {key:"lorebook_reference",title:"Lorebook Reference Context",text:loreLane,applied:true,status:"applied"},
    {key:"output_guidance",title:"Output Guidance Context",text:guidanceLane,applied:true,status:"applied"}
  ];
  const longApplied=apply(firstPayload,longPlan);
  assert(longApplied.injectionResult.payloadApplicationObservation.payload_application_status==="applied","full Go plan was not observed exactly");
  assert(!longApplied.injectionResult.auxiliaryPreview.includes(marker),"fixture did not isolate the 500-char preview boundary");
  const longInput=transparency(user,longApplied);
  const longEffective=composeEffectiveInputFromTransparency(longInput);
  assert(longEffective===user+"\n\n"+longAux,"host recent chat survived in effective input");
  assert(longEffective.includes(loreLane) && longEffective.includes(marker),"lore or guidance was omitted from full auxiliary text");
  const longParity=parity(firstPayload,longApplied,user,longInput);
  assert(longParity.status==="ready" && longParity.payloadContentMatch===true,"full payload exact observation was rejected");
  const longBody=await complete(user,longInput,longParity);
  const saved=longBody.client_meta.effective_input_observation;
  assert(saved && saved.effective_input===longEffective && saved.effective_input.includes(marker),">500 Unicode marker did not survive complete-turn observation");
  longInput.injection.memoryDeliveryPlan={used_chars:memoryLane.length,delivery_cap_chars:4000,global_cap_chars:4000,classes:[{key:"event_recent",title:"Event and Recent Memories",text:"[Event and Recent Memories]\n"+memoryLane}]};
  lastOrchResult={_trace:{_inputTransparency:longInput,finalPayloadParity:longParity}};
  const fullLaneHTML=renderEffectiveInputSection();
  [referenceLane,memoryLane,loreLane,guidanceLane,marker].forEach(function(value) {
    assert(fullLaneHTML.includes(value),"full canonical plan lane was not rendered: "+value.slice(0,40));
  });
  assert(!fullLaneHTML.includes(inputContext),"host recent chat was rendered as delivered effective input");

  assert(fullLaneHTML.includes("Event and Recent Memories") && !fullLaneHTML.includes("Long-term Memory Context"),"memory classes were not rendered as separate edit-check panes");

  assert((fullLaneHTML.match(/LOREBOOK_FULL_MARKER/g)||[]).length===1,"lorebook lane was duplicated");

  const loreOnlyPlan=plan(loreLane,"","ready");
  loreOnlyPlan.lanes=[{key:"lorebook_reference",title:"Lorebook Reference Context",text:loreLane,applied:true,status:"applied"}];
  const loreOnlyApplied=apply(firstPayload,loreOnlyPlan);
  const loreOnlyInput=transparency(user,loreOnlyApplied);
  const loreOnlyParity=parity(firstPayload,loreOnlyApplied,user,loreOnlyInput);
  lastOrchResult={_trace:{_inputTransparency:loreOnlyInput,finalPayloadParity:loreOnlyParity}};
  const loreOnlyHTML=renderEffectiveInputSection();
  assert(loreOnlyHTML.includes(loreLane) && !loreOnlyHTML.includes(referenceLane),"lorebook-only canonical lane was omitted or contaminated");

  const missingObservation=observeGoPayloadApplication(firstPayload,longPlan,{});
  const missingInput={
    backendEffectiveInputPreview:{contract_version:"effective_input_preview.v1",final_user_text:user},
    injection:{payloadApplicationPlan:longPlan,payloadApplicationObservation:missingObservation}
  };
  const missingParity=buildFinalPayloadParityTrace(firstPayload,firstPayload,{
    effectiveInputText:composeEffectiveInputFromTransparency(missingInput),effectiveUserInput:user,
    injectionResult:missingInput.injection,applyMode:{mode:"shadow",payloadReplaced:false}
  });
  assert(missingParity.status==="mismatch" && missingParity.payloadContentMatch===false,"missing full payload blocks were accepted");
  const missingBody=await complete(user,missingInput,missingParity);
  assert(!missingBody.client_meta.effective_input_observation,"missing payload blocks were persisted");
  lastOrchResult={_trace:{_inputTransparency:missingInput,finalPayloadParity:missingParity}};
  const missingHTML=renderEffectiveInputSection();
  assert(missingHTML.includes("will not be stored as verified effective input"),"payload mismatch warning was not rendered");
  assert(missingHTML.includes(user) && missingHTML.includes(loreLane) && missingHTML.includes(marker) && !missingHTML.includes(inputContext),"payload mismatch rendered non-delivered host recent chat");

  const mismatchPlan=Object.assign({},longPlan,{auxiliary_observation_hash:"or1c_wrong"});
  const mismatchApplied=apply(firstPayload,mismatchPlan);
  const mismatchInput=transparency(user,mismatchApplied);
  const mismatchParity=parity(firstPayload,mismatchApplied,user,mismatchInput);
  assert(mismatchApplied.injectionResult.payloadApplicationObservation.reason_code==="injected_block_hash_mismatch","hash mismatch fixture was not observed");
  assert(mismatchParity.status==="mismatch" && mismatchParity.payloadContentMatch===false,"hash mismatch was accepted");
  const mismatchBody=await complete(user,mismatchInput,mismatchParity);
  assert(!mismatchBody.client_meta.effective_input_observation,"hash-mismatched effective input was persisted");
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("effective-input production JS fixture failed: %v\n%s", err, out)
	}
}

func TestSourceDiscoveryUsesSelectedWorkWithoutDuplicateTitleInput(t *testing.T) {
	src := readArchiveCenterJS(t)
	remember := extractArchiveCenterJSFunction(t, src, "referenceDiscoveryRememberDraft")
	panel := extractArchiveCenterJSFunction(t, src, "renderReferenceCanonPackPanel")
	library := extractArchiveCenterJSFunction(t, src, "renderReferenceLibrarySection")
	run := extractArchiveCenterJSAsyncFunction(t, src, "referenceDiscoveryRunFromUI")
	if !strings.Contains(remember, `state.works.find`) || strings.Contains(remember, `mo-discovery-work-query`) {
		t.Fatalf("discovery draft must derive the title from the selected work: %s", remember)
	}
	if !strings.Contains(panel, `대상 작품`) || strings.Contains(panel, `id="mo-discovery-work-query"`) {
		t.Fatalf("discovery panel still exposes a duplicate title input: %s", panel)
	}
	if !strings.Contains(library, `tabs + selector + renderReferenceCanonPackPanel()`) {
		t.Fatalf("work selector is missing from the discovery panel")
	}
	if !strings.Contains(run, `const data = await bridgeFetch("/source-discovery/jobs/v1", {`) ||
		!strings.Contains(run, `timeoutMs: 0`) {
		t.Fatalf("source discovery transport must not impose the shared Plugin Timeout: %s", run)
	}
}

func TestSourceDiscoveryDisplaysRetainedDocumentsAndDoesNotHidePartialCandidates(t *testing.T) {
	src := readArchiveCenterJS(t)
	run := extractArchiveCenterJSAsyncFunction(t, src, "referenceDiscoveryRunFromUI")
	render := extractArchiveCenterJSFunction(t, src, "renderReferenceLibrarySection")
	if !strings.Contains(run, `else if (candidateCount === 0)`) || strings.Contains(run, `insufficient_source_coverage" || candidateCount === 0`) {
		t.Fatalf("partial discovery candidates are still hidden by coverage state: %s", run)
	}
	for _, expected := range []string{"원문 문서", "raw_retention", "raw_text_length", `libraryView === "documents"`} {
		if !strings.Contains(render, expected) {
			t.Fatalf("retained source document UI missing %q", expected)
		}
	}
	panel := extractArchiveCenterJSFunction(t, src, "renderReferenceCanonPackPanel")
	for _, expected := range []string{`documents_retained`, `documents_upgraded`, `원문 DB 저장 실패`} {
		if !strings.Contains(panel, expected) {
			t.Fatalf("source body staging diagnostics missing %q", expected)
		}
	}
}

func TestRetainedDocumentsExposeIndependentCriticActions(t *testing.T) {
	src := readArchiveCenterJS(t)
	render := extractArchiveCenterJSFunction(t, src, "renderReferenceLibrarySection")
	start := extractArchiveCenterJSAsyncFunction(t, src, "referenceLibraryStartExtraction")
	for _, expected := range []string{`data-reference-document-extract`, `mo-reference-document`, `mo-reference-document-action`, `평론가 정밀 분석`, `평론가 다시 분석`} {
		if !strings.Contains(render, expected) {
			t.Fatalf("per-document critic UI missing %q", expected)
		}
	}
	for _, expected := range []string{`selectedDocumentId`, `"/documents/" + referenceLibraryPath(documentId) + "/extract"`, `reference_document_extract`} {
		if !strings.Contains(start, expected) {
			t.Fatalf("per-document extraction request missing %q", expected)
		}
	}
	if !strings.Contains(src, `referenceLibraryStartExtraction(button.getAttribute("data-reference-document-extract"))`) {
		t.Fatalf("per-document critic button is not connected to its document ID")
	}
}

func TestSourceDiscoveryResumesStoredAnalysisWithoutStartingAnotherSearch(t *testing.T) {
	src := readArchiveCenterJS(t)
	loadLatest := extractArchiveCenterJSAsyncFunction(t, src, "referenceDiscoveryLoadLatestJob")
	resume := extractArchiveCenterJSAsyncFunction(t, src, "referenceDiscoveryResumeFromUI")
	render := extractArchiveCenterJSFunction(t, src, "renderReferenceCanonPackPanel")
	if !strings.Contains(loadLatest, `/source-discovery/latest-job/v1`) || !strings.Contains(loadLatest, `state.discoveryJob = data && data.job_id ? data : null`) {
		t.Fatalf("latest discovery job is not restored after plugin reload: %s", loadLatest)
	}
	for _, expected := range []string{`/complete/v1`, `source_discovery_corpus_analysis`, `referenceLibraryPollJob`} {
		if !strings.Contains(resume, expected) {
			t.Fatalf("source discovery resume adapter missing %q", expected)
		}
	}
	for _, expected := range []string{`mo-discovery-resume`, `remaining_document_count`, `duplicate_analysis_sections`} {
		if !strings.Contains(render, expected) {
			t.Fatalf("source discovery resume UI missing %q", expected)
		}
	}
	if !strings.Contains(render, `남은 원문 통합 분석`) || !strings.Contains(render, `원문 수집 완료 · 통합 분석 대기`) || !strings.Contains(render, `discoveryPlannedBatches`) {
		t.Fatalf("source discovery does not expose one-click corpus analysis: %s", render)
	}
	if strings.Contains(resume, `/source-discovery/jobs/v1`) {
		t.Fatalf("resume must not start another search job: %s", resume)
	}
}

func extractArchiveCenterJSFunction(t *testing.T, src, name string) string {
	t.Helper()
	marker := "  function " + name + "("
	start := strings.Index(src, marker)
	if start < 0 {
		t.Fatalf("Archive Center.js function %s not found", name)
	}
	nextFunction := strings.Index(src[start+len(marker):], "\n  function ")
	nextAsyncFunction := strings.Index(src[start+len(marker):], "\n  async function ")
	next := nextFunction
	if next < 0 || (nextAsyncFunction >= 0 && nextAsyncFunction < next) {
		next = nextAsyncFunction
	}
	if next < 0 {
		t.Fatalf("Archive Center.js function %s has no following function boundary", name)
	}
	end := start + len(marker) + next
	return strings.TrimSpace(src[start:end])
}

func TestDashboardNoticeTierRendersBelowWarningRuntime(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for dashboard notice runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	for _, marker := range []string{
		`.mo-dot-notice{background:#8fa7ff}`,
		`.mo-dash-card.has-notice`,
		`.mo-dash-chip-notice`,
		`.mo-hdr-health-badge-notice`,
	} {
		if !strings.Contains(src, marker) {
			t.Fatalf("dashboard notice style missing %q", marker)
		}
	}
	script := strings.Join([]string{
		extractArchiveCenterJSFunction(t, src, "statusDotClass"),
		extractArchiveCenterJSFunction(t, src, "runtimeStatusLabel"),
		extractArchiveCenterJSFunction(t, src, "renderDashboardViewModel"),
	}, "\n") + `
const labels = {
  "dash.status.state.ok": "정상",
  "dash.status.state.notice": "알림",
  "dash.status.state.warn": "경고",
  "dash.status.state.fail": "실패",
  "dash.status.state.skipped": "건너뜀",
  "dash.status.state.off": "꺼짐",
  "dash.status.state.running": "실행 중",
  "dash.status.state.queued": "대기열",
  "dash.status.state.idle": "유휴",
  "dash.status.state.unknown": "미확인",
  "header.health.allOk": "모두 정상",
};
function t(key) { return labels[key] || key; }
function escapeAttr(value) { return String(value == null ? "" : value); }
function dashboardSimpleText(key) { return key; }
function dashboardViewModelLabel(key) { return key; }
function dashboardViewModelText(value) { return String(value == null ? "" : value); }
function formatAuxiliaryPlacementTrace() { return ""; }
function formatDashboardTimestampLocal() { return ""; }

const vm = {
  status: "ok",
  summary: {ok: 1, notice: 2, warn: 0, fail: 0},
  cards: [
    {title: "Advisory", severity: "notice", summary: {notice: 1}, rows: [{label_key: "save", status: "notice", detail: "waiting"}]},
    {title: "Queued", severity: "notice", summary: {notice: 1}, chips: [{tone: "notice", label: "queued"}], rows: []},
  ],
};
const html = renderDashboardViewModel(vm, {});
if (!html.includes("has-notice") || !html.includes("mo-dash-chip-notice") || !html.includes("mo-dot-notice")) {
  throw new Error("notice card did not render with advisory styles: " + html);
}
if (statusDotClass("deferred") !== "mo-dot-notice" || statusDotClass("degraded") !== "mo-dot-warn") {
  throw new Error("notice/warning dot classification regressed");
}
if (runtimeStatusLabel("deferred") !== "알림" || runtimeStatusLabel("degraded") !== "경고") {
  throw new Error("notice/warning labels regressed");
}
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("dashboard notice JS fixture failed: %v\n%s", err, output)
	}
}

func TestPrepareTurnEmptyObservationProductionJSAndGoRoute(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for Archive Center prepare-turn observation runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	hashFunction := extractArchiveCenterJSFunction(t, src, "computeOrchestrationDirtyHashOr1c")
	observationFunction := extractArchiveCenterJSFunction(t, src, "buildPrepareTurnSourceObservations")
	script := hashFunction + "\n" + observationFunction + `
const result = {
  empty: buildPrepareTurnSourceObservations("session-a", "request-empty", 0, "user", "", '["messages"]', "chat-a"),
  non_empty: buildPrepareTurnSourceObservations("session-a", "request-non-empty", 0, "user", "hello", '["messages"]', "chat-a")
};
process.stdout.write(JSON.stringify(result));
`
	command := exec.Command(nodePath, "-e", script)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("prepare-turn observation runtime fixture failed: %v\n%s", err, output)
	}
	var observations map[string]map[string]any
	if err := json.Unmarshal(output, &observations); err != nil {
		t.Fatalf("decode production JS observations: %v\n%s", err, output)
	}
	emptySource := observations["empty"]["sourceObservation"].(map[string]any)
	emptyCapabilities := observations["empty"]["capabilityObservation"].(map[string]any)
	if _, exists := emptySource["raw_input_hash"]; exists {
		t.Fatalf("empty observation must omit raw_input_hash: %+v", emptySource)
	}
	if got := emptyCapabilities["capabilities"].(map[string]any)["raw_input_hash"]; got != "unavailable" {
		t.Fatalf("empty hash capability=%v, want unavailable", got)
	}
	nonEmptySource := observations["non_empty"]["sourceObservation"].(map[string]any)
	nonEmptyCapabilities := observations["non_empty"]["capabilityObservation"].(map[string]any)
	if hash, _ := nonEmptySource["raw_input_hash"].(string); hash == "" {
		t.Fatalf("non-empty observation lost hash: %+v", nonEmptySource)
	}
	if got := nonEmptyCapabilities["capabilities"].(map[string]any)["raw_input_hash"]; got != "observed" {
		t.Fatalf("non-empty hash capability=%v, want observed", got)
	}

	server := httpapi.NewServer(config.Default())
	server.Store = store.NewNoopStore()
	mux := http.NewServeMux()
	server.RegisterRoutes(mux)
	requestPrepare := func(t *testing.T, rawInput string, observation map[string]any) map[string]any {
		t.Helper()
		body, err := json.Marshal(map[string]any{
			"chat_session_id":        "session-a",
			"raw_user_input":         rawInput,
			"messages":               []map[string]any{{"role": "user", "content": rawInput}},
			"source_observation":     observation["sourceObservation"],
			"capability_observation": observation["capabilityObservation"],
		})
		if err != nil {
			t.Fatal(err)
		}
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/prepare-turn", bytes.NewReader(body)))
		if recorder.Code != http.StatusOK {
			t.Fatalf("prepare-turn status=%d body=%s", recorder.Code, recorder.Body.String())
		}
		var response map[string]any
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		return response["source_contract"].(map[string]any)["lane_status"].(map[string]any)
	}
	if lane := requestPrepare(t, "", observations["empty"]); lane["status"] != "empty" || lane["reason_code"] != "source_observation_empty" {
		t.Fatalf("empty production lane=%+v", lane)
	}
	if lane := requestPrepare(t, "hello", observations["non_empty"]); lane["status"] != "eligible" || lane["reason_code"] != "source_observation_eligible" {
		t.Fatalf("non-empty production lane=%+v", lane)
	}
	malformed := map[string]any{}
	encodedEmpty, err := json.Marshal(observations["empty"])
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encodedEmpty, &malformed); err != nil {
		t.Fatal(err)
	}
	malformedCapabilities := malformed["capabilityObservation"].(map[string]any)["capabilities"].(map[string]any)
	malformedCapabilities["raw_input_hash"] = "observed"
	if lane := requestPrepare(t, "", malformed); lane["status"] != "failed" || lane["reason_code"] != "source_observation_malformed" {
		t.Fatalf("malformed observed-hash lane=%+v", lane)
	}
}

func extractArchiveCenterJSAsyncFunction(t *testing.T, src, name string) string {
	t.Helper()
	marker := "  async function " + name + "("
	start := strings.Index(src, marker)
	if start < 0 {
		t.Fatalf("Archive Center.js async function %s not found", name)
	}
	nextFunction := strings.Index(src[start+len(marker):], "\n  function ")
	nextAsyncFunction := strings.Index(src[start+len(marker):], "\n  async function ")
	next := nextFunction
	if next < 0 || (nextAsyncFunction >= 0 && nextAsyncFunction < next) {
		next = nextAsyncFunction
	}
	if next < 0 {
		t.Fatalf("Archive Center.js async function %s has no following function boundary", name)
	}
	end := start + len(marker) + next
	return strings.TrimSpace(src[start:end])
}

func TestTurnWorkflowHUDEventStreamUsesOneConnectionAndNoPolling(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for turn workflow HUD watch runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	cancelStream := extractArchiveCenterJSFunction(t, src, "cancelTurnWorkflowHUDStream")
	streamFailure := extractArchiveCenterJSFunction(t, src, "turnWorkflowHUDStreamFailure")
	openStream := extractArchiveCenterJSAsyncFunction(t, src, "openTurnWorkflowHUDStream")
	consumeLine := extractArchiveCenterJSAsyncFunction(t, src, "consumeTurnWorkflowHUDStreamLine")
	consumeStream := extractArchiveCenterJSAsyncFunction(t, src, "consumeTurnWorkflowHUDStream")
	primeHUD := extractArchiveCenterJSFunction(t, src, "primeTurnWorkflowHUD")
	startWatch := extractArchiveCenterJSFunction(t, src, "startTurnWorkflowHUDWatch")
	streamSource := strings.Join([]string{cancelStream, openStream, consumeStream, primeHUD, startWatch}, "\n")
	for _, forbidden := range []string{"/turn-workflow/status", "wait_ms", "setInterval(", "setTimeout(", "llmRetryCount"} {
		if strings.Contains(streamSource, forbidden) {
			t.Fatalf("turn workflow HUD stream retained forbidden automatic transport %q", forbidden)
		}
	}
	if !strings.Contains(startWatch, "/turn-workflow/events") ||
		!strings.Contains(openStream, "R.nativeFetch") ||
		!strings.Contains(openStream, "response.body.getReader") {
		t.Fatal("turn workflow HUD stream is not using the nativeFetch readable-stream feature probe")
	}
	script := `
const settings = {turnWorkflowHUDEnabled:true,requestTimeoutMs:17000};
const TURN_WORKFLOW_HUD_CONTRACT = "turn_workflow_hud.v3";
const encoder = new TextEncoder();
let _turnWorkflowHUDWatchToken = 0;
let _turnWorkflowHUDWatchRunning = false;
let _turnWorkflowHUDActiveRequestId = "";
let _turnWorkflowHUDLastRevision = 0;
let _turnWorkflowHUDTerminalRequestId = "";
let _turnWorkflowHUDStreamAbortController = null;
let _turnWorkflowHUDStreamReader = null;
let _turnWorkflowHUDRenderChain = Promise.resolve();
const _turnWorkflowHUDHostWarningsByRequestId = new Map();
let streamResponses = [];
let streamPaths = [];
let consumedStatuses = [];
let transportErrors = [];
let dismissedRequests = [];
let fallbackFetchCalls = 0;
const R = {
  nativeFetch: async function(path) {
    streamPaths.push(String(path || ""));
    if (streamResponses.length === 0) throw new Error("unexpected extra stream request");
    return streamResponses.shift();
  },
};
function turnWorkflowHUDIsEnabled() { return settings.turnWorkflowHUDEnabled !== false; }
function getRequestTimeoutSettingMs() { return settings.requestTimeoutMs; }
function dismissTurnWorkflowHUD(requestId) {
  dismissedRequests.push(String(requestId || ""));
  _turnWorkflowHUDActiveRequestId = "";
  _turnWorkflowHUDLastRevision = 0;
  _turnWorkflowHUDTerminalRequestId = "";
}
function clearTurnWorkflowHUDTimer() {}
function turnWorkflowHUDHasHostWarning(requestId) {
  const warnings = _turnWorkflowHUDHostWarningsByRequestId.get(String(requestId || "").trim());
  return Array.isArray(warnings) && warnings.length > 0;
}
async function removeTurnWorkflowHUDDismissListeners() {}
function queueTurnWorkflowHUDOperation(_label, operation) {
  _turnWorkflowHUDRenderChain = _turnWorkflowHUDRenderChain.then(operation);
  return _turnWorkflowHUDRenderChain;
}
async function ensureTurnWorkflowHUDRoot() { return null; }
function resolveBridgeRuntimeRoute() { return {url:"http://127.0.0.1:28080"}; }
function consumeTurnWorkflowHUD(view) {
  consumedStatuses.push(String(view && view.status || ""));
  _turnWorkflowHUDLastRevision = Math.max(_turnWorkflowHUDLastRevision, Number(view && view.revision || 0));
  if (view && (view.status === "completed" || view.status === "failed" || view.status === "invalidated")) {
    _turnWorkflowHUDTerminalRequestId = String(view.request_id || "");
  }
  queueTurnWorkflowHUDOperation("render", async function() {});
  return true;
}
function renderTurnWorkflowHUDTransportError(requestId, reasonCode) {
  if (!requestId || requestId !== _turnWorkflowHUDActiveRequestId) return;
  transportErrors.push(String(requestId || "") + ":" + String(reasonCode || ""));
}
function debugLog() {}
function fetch() { fallbackFetchCalls++; throw new Error("unproven fallback fetch used"); }
function assert(condition, message) { if (!condition) throw new Error(message); }
async function settleWatch(label) {
  for (let index = 0; index < 20 && _turnWorkflowHUDWatchRunning; index++) {
    await new Promise(function(resolve) { setImmediate(resolve); });
  }
  assert(!_turnWorkflowHUDWatchRunning, label + " did not settle");
}
function responseFromLines(lines) {
  const chunks = [encoder.encode(lines.join("\n") + "\n")];
  return {
    status: 200,
    ok: true,
    body: {
      getReader: function() {
        return {
          read: async function() {
            if (chunks.length > 0) return {value:chunks.shift(),done:false};
            return {done:true};
          },
          cancel: async function() {},
        };
      },
    },
  };
}
` + "\n" + cancelStream + "\n" + streamFailure + "\n" + openStream + "\n" + consumeLine + "\n" + consumeStream + "\n" + primeHUD + "\n" + startWatch + `
(async function() {
  streamResponses = [responseFromLines([
    JSON.stringify({contract_version:"turn_workflow_hud.v3",request_id:"registered-request",status:"running",revision:1}),
    JSON.stringify({contract_version:"turn_workflow_hud.v3",request_id:"registered-request",status:"running",revision:2}),
    JSON.stringify({contract_version:"turn_workflow_hud.v3",request_id:"registered-request",status:"completed",revision:3}),
  ])];
  startTurnWorkflowHUDWatch("registered-request");
  await settleWatch("registered stream");
  assert(streamPaths.length === 1, "workflow used more than one HTTP connection");
  assert(streamPaths[0].includes("/turn-workflow/events?"), "workflow did not use the event stream endpoint");
  assert(streamPaths[0].includes("after_revision=0"), "new request did not begin after revision zero");
  assert(consumedStatuses.join(",") === "running,running,running,completed", "host pending state or stream revisions were not rendered sequentially");
  assert(transportErrors.length === 0, "valid stream produced a transport error");

  streamPaths = [];
  consumedStatuses = [];
  streamResponses = [{status:200,ok:true,body:null}];
  startTurnWorkflowHUDWatch("unsupported-request");
  await settleWatch("unsupported stream");
  assert(streamPaths.length === 1, "unsupported stream retried or polled");
  assert(fallbackFetchCalls === 0, "unsupported native stream fell back to unproven fetch");
  assert(transportErrors.length === 0, "HUD-only transport loss was rendered as a turn failure");
  assert(dismissedRequests.length === 0, "HUD-only transport loss discarded the active workflow identity");
  assert(_turnWorkflowHUDActiveRequestId === "unsupported-request", "HUD-only transport loss cleared the active workflow identity");
  renderTurnWorkflowHUDTransportError("unsupported-request", "complete_turn_transport_unavailable");
  assert(
    transportErrors.join(",") === "unsupported-request:complete_turn_transport_unavailable",
    "later complete-turn transport failure was hidden after HUD stream loss"
  );
  process.stdout.write("ok");
})().catch(function(err) {
  console.error(err && err.stack || err);
  process.exit(1);
});
`
	scriptPath := t.TempDir() + "/turn-workflow-hud-watch-runtime.js"
	if err := os.WriteFile(scriptPath, []byte(script), 0600); err != nil {
		t.Fatalf("write turn workflow HUD watch runtime fixture: %v", err)
	}
	command := exec.Command(nodePath, scriptPath)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("turn workflow HUD watch runtime fixture failed: %v\n%s", err, output)
	}
	if strings.TrimSpace(string(output)) != "ok" {
		t.Fatalf("turn workflow HUD watch runtime fixture output=%q, want ok", output)
	}
}

func TestTurnWorkflowHUDTransportFailureClassificationAndPersistenceRuntime(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for turn workflow HUD transport runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	bridge := extractArchiveCenterJSAsyncFunction(t, src, "bridgeFetch")
	classify := extractArchiveCenterJSFunction(t, src, "classifyTurnWorkflowHUDTransportFailure")
	remember := extractArchiveCenterJSFunction(t, src, "rememberTurnWorkflowHUDHostWarning")
	hasWarning := extractArchiveCenterJSFunction(t, src, "turnWorkflowHUDHasHostWarning")
	warningHTML := extractArchiveCenterJSFunction(t, src, "turnWorkflowHUDWarningListHTML")
	renderTransport := extractArchiveCenterJSFunction(t, src, "renderTurnWorkflowHUDTransportError")
	presentation := extractArchiveCenterJSFunction(t, src, "buildTurnWorkflowHUDPresentation")
	watch := extractArchiveCenterJSFunction(t, src, "startTurnWorkflowHUDWatch")

	if strings.Contains(renderTransport, `_turnWorkflowHUDTerminalRequestId = requestId`) {
		t.Fatal("host transport warning still terminates the workflow HUD before later save stages")
	}
	if !strings.Contains(watch, `turnWorkflowHUDHasHostWarning(normalizedRequestId)`) {
		t.Fatal("HUD stream cleanup can still erase a recorded host transport warning")
	}
	recoveringStart := strings.Index(presentation, `if (view.status === "recovering")`)
	failedStart := strings.Index(presentation, `if (view.status === "failed" || severity === "error")`)
	if recoveringStart < 0 || failedStart <= recoveringStart ||
		!strings.Contains(presentation[recoveringStart:failedStart], `+ turnWorkflowHUDWarningListHTML(view)`) {
		t.Fatal("recovering HUD mode does not preserve the host warning list")
	}
	if count := strings.Count(presentation, `+ turnWorkflowHUDWarningListHTML(view)`); count != 5 {
		t.Fatalf("HUD warning list must be rendered in all five presentation modes; count=%d", count)
	}
	for _, marker := range []string{
		`kind: String(kind || "unknown")`,
		`target_url: targetUrl`,
		`timeout_ms: timeout`,
		`elapsed_ms: Math.max(0, recordedAt - requestStartedAt)`,
		`error_code: String(diagnostics.error_code || "")`,
		`response_body: String(diagnostics.response_body || "").slice(0, 4000)`,
	} {
		if !strings.Contains(bridge, marker) {
			t.Fatalf("bridge transport diagnostic value is discarded: %s", marker)
		}
	}

	script := `
const _lastBridgeFailureByPath = new Map();
const _turnWorkflowHUDHostWarningsByRequestId = new Map();
const TURN_WORKFLOW_HUD_WARNING_ITEM_STYLE = "warning-item";
const TURN_WORKFLOW_HUD_WARNING_LIST_STYLE = "warning-list";
const TURN_WORKFLOW_HUD_WARNING_DETAIL_STYLE = "warning-detail";
const _turnWorkflowHUDUnloaded = false;
let _turnWorkflowHUDActiveRequestId = "request-timeout";
function t(key) { return key; }
function escapeTurnWorkflowHUDHTML(value) { return String(value == null ? "" : value); }
function turnWorkflowHUDIsEnabled() { return true; }
function dismissTurnWorkflowHUD() { throw new Error("timeout warning dismissed the active workflow"); }
function debugLog() {}
function assert(condition, message) { if (!condition) throw new Error(message); }
` + "\n" + classify + "\n" + remember + "\n" + hasWarning + "\n" + warningHTML + "\n" + renderTransport + `
_lastBridgeFailureByPath.set("/prepare-turn", {kind:"timeout",status:0,detail:"timeout"});
const timeoutRenderResult = renderTurnWorkflowHUDTransportError(
  "request-timeout",
  "/prepare-turn",
  "prepare_turn_transport_unavailable"
);
assert(timeoutRenderResult === undefined, "prepare timeout continued into terminal HUD rendering");
assert(!turnWorkflowHUDHasHostWarning("request-timeout"), "prepare timeout was persisted as a workflow warning");

const cases = [
  [{kind:"bridge_url_invalid",status:0,detail:"no valid bridgeUrl"}, "prepare_turn_bridge_url_invalid"],
  [{kind:"timeout",status:0,detail:"timeout"}, "prepare_turn_timeout"],
  [{kind:"response_decode_failed",status:200,detail:"json parse failed: Unexpected token"}, "prepare_turn_response_decode_failed"],
  [{kind:"http_error",status:503,detail:"service unavailable"}, "prepare_turn_http_error"],
  [{kind:"connection_failed",status:0,detail:"Failed to fetch"}, "prepare_turn_connection_failed"],
];
for (const entry of cases) {
  _lastBridgeFailureByPath.set("/prepare-turn", entry[0]);
  const warning = classifyTurnWorkflowHUDTransportFailure("/prepare-turn", "prepare_turn_transport_unavailable");
  assert(warning.code === entry[1], "wrong classification for " + JSON.stringify(entry[0]) + ": " + warning.code);
}
_lastBridgeFailureByPath.set("/prepare-turn", {
  kind:"http_error",path:"/prepare-turn",method:"POST",
  configured_url:"http://100.64.0.10:28080",target_url:"http://100.64.0.10:28080/prepare-turn",
  route_mode:"configured",page_host:"risu.example",loopback_on_hosted_page:false,mixed_content_risk:true,
  status:503,timeout_ms:45000,elapsed_ms:45123,error_name:"TypeError",error_code:"ECONNRESET",
  error_message:"request failed",error_cause:"socket closed",response_read_error:"body already read",
  response_body:'{"code":"UPSTREAM_UNAVAILABLE"}',detail:"service unavailable",at:1722513600000
});
const warning = classifyTurnWorkflowHUDTransportFailure("/prepare-turn", "prepare_turn_transport_unavailable");
rememberTurnWorkflowHUDHostWarning("request-a", warning);
assert(turnWorkflowHUDHasHostWarning("request-a"), "host warning was not retained for the workflow request");
const html = turnWorkflowHUDWarningListHTML({
  request_id:"request-a",
  warnings:[{code:"backend_warning",message:"backend warning"}],
});
assert(html.includes("backend warning · backend_warning"), "backend warning was lost");
assert(html.includes("prepare_turn_http_error"), "typed transport code was not shown");
for (const expected of [
  "failure_kind=http_error", "request_path=/prepare-turn", "method=POST",
  "configured_url=http://100.64.0.10:28080", "target_url=http://100.64.0.10:28080/prepare-turn",
  "route_mode=configured", "page_host=risu.example", "loopback_on_hosted_page=false",
  "mixed_content_risk=true", "timeout_ms=45000", "elapsed_ms=45123", "http_status=503",
  "error_name=TypeError", "error_code=ECONNRESET", "error_message=request failed",
  "error_cause=socket closed", "response_read_error=body already read",
  'backend_response={"code":"UPSTREAM_UNAVAILABLE"}', "detail=service unavailable",
  "recorded_at=2024-08-01T12:00:00.000Z",
]) assert(html.includes(expected), "missing transport diagnostic: " + expected);
process.stdout.write("ok");
`
	scriptPath := t.TempDir() + "/turn-workflow-hud-transport-runtime.js"
	if err := os.WriteFile(scriptPath, []byte(script), 0600); err != nil {
		t.Fatalf("write turn workflow HUD transport runtime fixture: %v", err)
	}
	command := exec.Command(nodePath, scriptPath)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("turn workflow HUD transport runtime fixture failed: %v\n%s", err, output)
	}
	if strings.TrimSpace(string(output)) != "ok" {
		t.Fatalf("turn workflow HUD transport runtime fixture output=%q, want ok", output)
	}
}

func TestBridgeFetchRecordsEffectiveTimeoutDiagnosticsRuntime(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for bridge timeout diagnostics runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	resolver := extractArchiveCenterJSFunction(t, src, "resolveRequestTimeoutMs")
	bridge := extractArchiveCenterJSAsyncFunction(t, src, "bridgeFetch")
	script := `
const settings = {bridgeUrl:"http://100.64.0.10:28080",requestTimeoutMs:25};
const _lastBridgeFailureByPath = new Map();
let requestMode = "delayed";
const R = {nativeFetch: async function(){
  if(requestMode === "delayed") {
    return await new Promise(function(resolve){
      setTimeout(function(){ resolve({ok:true,status:200,json:async function(){ return {status:"ok"}; }}); },40);
    });
  }
  return await new Promise(function(){});
}};
function getRequestTimeoutSettingMs(){ return 25; }
function resolveBridgeRuntimeRoute(rawUrl){
  return {url:rawUrl,configuredUrl:rawUrl,mode:"configured",remoteAuto:false,pageHost:"risu.example",loopbackOnHostedPage:false,mixedContentRisk:true};
}
function warnLog(){}
function debugLog(){}
function extractBridgeErrorDetail(_payload,fallback){ return fallback; }
function assert(condition,message){ if(!condition) throw new Error(message); }
` + "\n" + resolver + "\n" + bridge + `
(async function(){
  const longResult = await bridgeFetch("/import/hypamemory", {method:"POST",body:{memories:[]},timeoutMs:0});
  assert(longResult && longResult.status === "ok", "backend-owned wait was cut off by Plugin Timeout");
  assert(!_lastBridgeFailureByPath.has("/import/hypamemory"), "backend-owned wait recorded a timeout failure");
  requestMode = "hang";
  const result = await bridgeFetch("/short-operation", {method:"POST",body:{input:"test"},timeoutMs:25});
  assert(result === null, "timed-out request returned a result");
  const failure = _lastBridgeFailureByPath.get("/short-operation");
  assert(failure && failure.kind === "timeout", "timeout kind was not recorded");
  assert(failure.timeout_ms === 25, "effective UI timeout was replaced: " + JSON.stringify(failure));
  assert(failure.elapsed_ms >= 15 && failure.elapsed_ms < 5000, "elapsed timeout value is implausible: " + failure.elapsed_ms);
  assert(failure.method === "POST", "request method was not recorded");
  assert(failure.target_url === "http://100.64.0.10:28080/short-operation", "target URL was not recorded");
  assert(failure.mixed_content_risk === true, "route diagnostics were not recorded");
  process.stdout.write("ok");
})().catch(function(err){ console.error(err && err.stack || err); process.exit(1); });
`
	scriptPath := t.TempDir() + "/bridge-effective-timeout-runtime.js"
	if err := os.WriteFile(scriptPath, []byte(script), 0600); err != nil {
		t.Fatalf("write bridge timeout diagnostics runtime fixture: %v", err)
	}
	command := exec.Command(nodePath, scriptPath)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("bridge timeout diagnostics runtime fixture failed: %v\n%s", err, output)
	}
	if strings.TrimSpace(string(output)) != "ok" {
		t.Fatalf("bridge timeout diagnostics runtime fixture output=%q, want ok", output)
	}
}

func TestTurnWorkflowHUDUsesRisuMainRootDocumentRuntime(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for turn workflow HUD runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	start := strings.Index(src, `  const TURN_WORKFLOW_HUD_CONTRACT = "turn_workflow_hud.v3";`)
	if start < 0 {
		t.Fatal("turn workflow HUD contract marker not found")
	}
	endMarker := "\n  function turnWorkflowHUDRequestIdFromPrepareOptions("
	endOffset := strings.Index(src[start:], endMarker)
	if endOffset < 0 {
		t.Fatal("turn workflow HUD runtime boundary not found")
	}
	hudRuntime := strings.TrimSpace(src[start : start+endOffset])
	script := `
const nodesByClass = new Map();
let elapsedTimerSequence = 0;
const elapsedTimers = new Map();
const clearedElapsedTimers = [];
let risuEventListenerSequence = 0;
const risuEventListeners = new Map();
let mainDomPermissionRequests = 0;
async function dispatchRisuEvent(type, event) {
  for (const listener of Array.from(risuEventListeners.values())) {
    if (listener.type === type) {
      await listener.handler(Object.assign({type}, event || {}));
    }
  }
}
function setTimeout(fn, delay) {
  const id = ++elapsedTimerSequence;
  elapsedTimers.set(id, {fn, delay});
  return id;
}
function clearTimeout(id) {
  clearedElapsedTimers.push(id);
  elapsedTimers.delete(id);
}
class FakeRemoteNode {
  constructor(tag) {
    this.tag = tag;
    this.attributes = {};
    this.children = [];
    this.innerHTML = "";
    this.textContent = "";
    this.listeners = {};
    this.listenerIds = {};
    this.parent = null;
    this.card = null;
    this.elapsed = null;
    this.button = null;
    this.recoveryButton = null;
    this.surface = null;
  }
  async setAttribute(name, value) {
    if (!String(name).startsWith("x-")) {
      throw new Error("prohibited remote DOM attribute: " + name);
    }
    this.attributes[name] = String(value);
  }
  async addClass(name) {
    const className = String(name);
    this.attributes.class = [this.attributes.class || "", className].filter(Boolean).join(" ");
    nodesByClass.set(className, this);
  }
  async setInnerHTML(value) {
    const sourceHTML = String(value);
    this.innerHTML = sourceHTML
      .replace(/\s+x-[\w-]+="[^"]*"/g, "")
      .replace(/class="([^"]*)"/g, function(_, names) {
        return 'class="' + names.split(/\s+/).filter(Boolean).map(function(name) {
          return "x-risu-" + name;
        }).join(" ") + '"';
      });
    if (sourceHTML.includes("position:fixed;") && sourceHTML.includes('aria-live="polite"')) {
      this.surface = new FakeRemoteNode("surface");
      this.surface.attributes.style = sourceHTML.match(/style="([^"]*)"/)?.[1] || "";
    }
    this.card = this.tag === "surface" && sourceHTML.trimStart().startsWith("<div")
      ? new FakeRemoteNode("card")
      : null;
    if (this.card) {
      this.card.attributes.style = sourceHTML.match(/^\s*<div[^>]*style="([^"]*)"/)?.[1] || "";
    }
    this.elapsed = sourceHTML.includes("<time")
      ? new FakeRemoteNode("elapsed")
      : null;
    if (this.elapsed) {
      this.elapsed.attributes.style = sourceHTML.match(/<time[^>]*style="([^"]*)"/)?.[1] || "";
    }
    this.button = sourceHTML.includes("<button") ? new FakeRemoteNode("button") : null;
    this.recoveryButton = sourceHTML.includes("data-turn-workflow-recovery-action")
      ? new FakeRemoteNode("recovery-button")
      : null;
    if (this.card) {
      this.card.button = this.button;
    }
  }
  async setTextContent(value) {
    this.textContent = String(value);
  }
  async appendChild(child) {
    child.parent = this;
    this.children.push(child);
  }
  async remove() {
    if (this.parent) {
      this.parent.children = this.parent.children.filter(child => child !== this);
      this.parent = null;
    }
    for (const [className, node] of Array.from(nodesByClass.entries())) {
      if (node === this) nodesByClass.delete(className);
    }
  }
  async querySelector(selector) {
    if (selector === "div") return this.card;
    if (selector === "time") return this.elapsed;
    if (selector === "button") return this.button;
    if (selector === "[data-turn-workflow-recovery-action]") return this.recoveryButton;
    return null;
  }
  async addEventListener(name, handler) {
    const listenerId = "listener-" + (++risuEventListenerSequence);
    this.listeners[name] = handler;
    this.listenerIds[name] = listenerId;
    risuEventListeners.set(listenerId, {type:name, handler, node:this});
    return listenerId;
  }
  async getBoundingClientRect() {
    if (this.tag === "button") {
      return {left:110, top:10, right:128, bottom:28, width:18, height:18};
    }
    if (this.tag === "recovery-button") {
      return {left:10, top:160, right:130, bottom:190, width:120, height:30};
    }
    return {left:0, top:0, right:140, bottom:200, width:140, height:200};
  }
}
const head = new FakeRemoteNode("head");
const body = new FakeRemoteNode("body");
const rootDocument = {
  async querySelector(selector) {
    if (selector === "head") return head;
    if (selector === "body") return body;
    if (selector === ".mo-turn-workflow-hud-root > .mo-turn-workflow-hud-surface") {
      const root = nodesByClass.get("mo-turn-workflow-hud-root");
      return root && root.surface && root.surface.attributes.class === "mo-turn-workflow-hud-surface"
        ? root.surface
        : null;
    }
    if (selector === ".mo-turn-workflow-hud-root > div") {
      const root = nodesByClass.get("mo-turn-workflow-hud-root");
      return root && root.surface || null;
    }
    if (selector.startsWith(".")) return nodesByClass.get(selector.slice(1)) || null;
    return null;
  },
  async createElement(tag) {
    return new FakeRemoteNode(tag);
  }
};
const R = {
  requestPluginPermission: async function(permission) {
    if (permission !== "mainDom") throw new Error("unexpected permission " + permission);
    mainDomPermissionRequests++;
    return true;
  },
  getRootDocument: async () => rootDocument,
  async removeRisuEventListener(listenerId) {
    const listener = risuEventListeners.get(listenerId);
    if (listener && listener.node.listenerIds[listener.type] === listenerId) {
      delete listener.node.listenerIds[listener.type];
      delete listener.node.listeners[listener.type];
    }
    risuEventListeners.delete(listenerId);
  }
};
const settings = {turnWorkflowHUDEnabled:true};
const BUILD_ID = "20260802-4";
const _lastBridgeFailureByPath = new Map();
let recoveryConfirmCalls = 0;
let recoveryConfirmResult = true;
const recoveryBridgeCalls = [];
let recoveryResponseView = null;
let recoveryBridgeFailure = false;
function confirm() {
  recoveryConfirmCalls++;
  return recoveryConfirmResult;
}
async function bridgeFetch(path, options) {
  recoveryBridgeCalls.push({path, options});
  if (recoveryBridgeFailure) return null;
  return {turn_workflow_hud: recoveryResponseView};
}
const translations = {
  "turn_hud.recovery.retry_derived_turn": "이 턴 복구 재시도",
  "turn_hud.recovery.confirm_title": "턴 기억 복구",
  "turn_hud.recovery.confirm_retry_derived_turn": "{turn}턴 복구",
  "turn_hud.recovery.requested": "복구 요청을 보냈습니다.",
  "turn_hud.recovery.request_failed": "복구 요청에 실패했습니다.",
  "turn_hud.completed": "완료",
  "turn_hud.completed_with_warning": "경고와 함께 완료",
  "turn_hud.invalidated": "작업 중단",
  "turn_hud.failed": "실패",
  "turn_hud.tap_to_dismiss": "눌러서 닫기",
  "turn_hud.transport_unavailable": "전송 실패",
  "turn_hud.notice.ooc_recognized": "OOC 인식",
  "turn_hud.notice.ooc_recognized_detail": "OOC 판정으로 입력 처리를 취소했습니다.",
  "turn_hud.notice.delete_confirmed": "삭제 확인 테스트",
  "turn_hud.notice.delete_confirmed_detail": "삭제 출력 정리 테스트",
  "turn_hud.notice.reroll_confirmed": "리롤 확인 테스트",
  "turn_hud.notice.reroll_confirmed_detail": "기존 턴 교체 테스트",
  "turn_hud.not_retryable": "재시도 불가",
  "turn_hud.retryable": "재시도 가능",
  "turn_hud.stage_ledger": "전체 작동 확인",
  "turn_hud.stage_status.succeeded": "정상",
  "turn_hud.stage_status.skipped": "건너뜀",
  "turn_hud.stage_status.failed": "실패",
  "turn_hud.stage_status.invalidated": "중단",
  "turn_hud.stage_status.pending": "미실행",
  "turn_hud.stage_status.running": "진행 중",
  "turn_hud.stage_status.unknown": "미확인",
  "turn_hud.reason.deferred_no_guide_support": "지원 근거 없음",
  "turn_hud.stage.prepare_source": "원문 준비",
  "warn.publisher": "감독관 호출을 건너뜀",
  "count.raw": "원문 <저장>",
  "count.summary": "요약",
  "count.direct": "직접 근거",
  "count.relationship": "관계 지식",
  "count.item": "물건",
  "count.world": "세계",
  "count.total": "총 생성"
};
function t(key) { return translations[key] || String(key || ""); }
function tf(key, args) {
  if (key === "turn_hud.turn") return String(args.n) + "번째 턴";
  if (key === "turn_hud.elapsed_seconds") return String(args.n) + "초";
  return key;
}
function warnLog() {}
` + "\n" + hudRuntime + `
function assert(condition, message) {
  if (!condition) throw new Error(message);
}
(async function() {
  assert(primeTurnWorkflowHUD("pending-a") === "pending-a", "host pending HUD was not accepted");
  await _turnWorkflowHUDRenderChain;
  const primedRoot = nodesByClass.get("mo-turn-workflow-hud-root");
  const primedSurface = primedRoot && primedRoot.surface;
  assert(primedSurface && primedSurface.innerHTML.includes("ARCHIVE CENTER"), "host pending HUD did not render immediately");
  assert(primedSurface.innerHTML.includes(translations["turn_hud.stage.prepare_source"]), "host pending HUD omitted the visible waiting stage");
  assert(mainDomPermissionRequests === 1, "host pending HUD did not request mainDom permission exactly once");
  await dismissTurnWorkflowHUD("pending-a");
  await _turnWorkflowHUDRenderChain;
  const counts = [
    {key:"raw",label_key:"count.raw",value:1},
    {key:"summary",label_key:"count.summary",value:2},
    {key:"direct",label_key:"count.direct",value:3},
    {key:"relationship",label_key:"count.relationship",value:4},
    {key:"item",label_key:"count.item",value:5},
    {key:"world",label_key:"count.world",value:6},
    {key:"total_committed",label_key:"count.total",value:21}
  ];
  const stages = Array.from({length:12}, function(_, index) {
    return {
      key:"stage-" + (index + 1),
      label_key:"stage.label." + (index + 1),
      ordinal:index + 1,
      total:12,
      status:index === 3 ? "skipped" : "succeeded",
      duration_ms:index === 3 ? 0 : (index + 1) * 100,
      reason_code:index === 3 ? "deferred_no_guide_support" : "",
      llm_call:index === 3 || index === 8
    };
  });
  const facts = [
    {key:"host_observation",status:"accepted",disposition:"eligible",reason_code:"source_observation_eligible",severity:"normal"},
    {key:"context_selection",status:"selected",disposition:"selected",reason_code:"payload_plan_context_selected",severity:"normal",count:240},
    {key:"payload_delivery",status:"applied",disposition:"delivered",reason_code:"risu_host_payload_application_observed",severity:"normal"},
    {key:"raw_persistence",status:"ok",disposition:"delivered",reason_code:"ok",severity:"normal",count:2},
    {key:"derived_memory",status:"ok",disposition:"delivered",reason_code:"ok",severity:"normal",count:10},
    {key:"vector_index",status:"vector_not_configured",disposition:"dropped",reason_code:"vector_not_configured",severity:"warning",count:0}
  ];
  const memorySelection = {
    vector_candidate_limit:5,
    core_objective_memory:{requested_max_items:5,eligible_distinct_count:5,delivered_count:5,deferred_by_limit_count:0,deferred_by_budget_count:0,missing_to_limit:0},
    lanes:[{key:"direct_evidence",selected_count:12,eligible_count:13,deferred_count:0,deduplicated_count:1}],
    items:[{source_row_id:46,turn_index:22,selection_lane:"vector_relevant",disposition:"delivered",reason_code:"selected_within_final_delivery_plan",preview:"민감한 기억 상세 본문"}],
    exclusion_reasons:{protected_entity_not_in_current_input:2}
  };
  assert(consumeTurnWorkflowHUD({
    contract_version:TURN_WORKFLOW_HUD_CONTRACT,request_id:"completed-a",revision:1,
    logical_turn:55,host_turn:56,backend_turn:55,
    turn_alignment:{host_turn:56,backend_turn:55,state:"host_ahead",reason_code:"host_turn_ahead_of_backend"},
    status:"completed_with_warning",severity:"warning",dismissal_policy:"x_only",counts,stages,facts,
    memory_selection:memorySelection,
    warnings:[{code:"PUBLISHER_SKIPPED",message_key:"warn.publisher",stage_key:"stage-4"}]
  }), "completed HUD view was rejected");
  await _turnWorkflowHUDRenderChain;
  const root = nodesByClass.get("mo-turn-workflow-hud-root");
  const surface = root && root.surface;
  assert(root, "HUD root was not created in RisuAI main RootDocument");
  assert(body.children.includes(root), "HUD root was not appended to main body");
  assert(surface, "HUD surface was not created with the Yumi-compatible innerHTML path");
  assert(!nodesByClass.has("mo-turn-workflow-hud-style"), "HUD still injects a stylesheet that RisuAI does not activate");
  assert(surface.attributes.style.includes("top:50%") && surface.attributes.style.includes("translateY(-50%)"), "HUD is not positioned at right center");
  assert(surface.attributes.style.includes("right:max(5px"), "HUD right safe-area placement is missing");
  assert(surface.attributes.style.includes("width:min(140px"), "HUD is wider than the compact right rail");
  assert(surface.card.attributes.style.includes("background:#181C24"), "completed HUD has no opaque fintech panel");
  assert(surface.card.attributes.style.includes("font-size:10px"), "completed HUD text is not compact");
  assert(surface.innerHTML.includes("ARCHIVE CENTER"), "completed HUD has no product eyebrow");
  assert(surface.innerHTML.includes("font-size:22px"), "completed HUD does not promote the total as its primary metric");
  assert(surface.innerHTML.includes("grid-template-columns:repeat(2,minmax(0,1fr))"), "completed HUD details are not arranged as a compact ledger");
  assert(surface.innerHTML.includes("linear-gradient(135deg,rgba(93,115,230,.18),rgba(138,85,247,.10)"), "completed HUD total does not use the restrained blue-purple selection gradient");
  assert(surface.innerHTML.includes("color:#8B909A"), "completed HUD secondary text does not use the supplied hierarchy");
  assert(surface.innerHTML.includes("전체 작동 확인"), "completed HUD omitted the full stage ledger heading");
  assert(surface.innerHTML.includes("건너뜀 · 0초"), "completed HUD omitted skipped stage status or duration");
  assert(surface.innerHTML.includes("지원 근거 없음"), "completed HUD omitted the visible stage reason");
  assert(surface.innerHTML.includes("감독관 호출을 건너뜀 · PUBLISHER_SKIPPED"), "completed HUD omitted warning details");
  assert(surface.innerHTML.includes("Host 56 / Backend 55"), "completed HUD omitted host/backend turn mismatch");
  assert(!surface.innerHTML.includes("WORKFLOW FACTS"), "completed HUD still exposes internal workflow facts");
  assert(!surface.innerHTML.includes("eligible · accepted"), "completed HUD still exposes internal host disposition");
  assert(!surface.innerHTML.includes("dropped · vector_not_configured"), "completed HUD still exposes internal vector disposition");
  assert(!surface.innerHTML.includes("direct_evidence"), "completed HUD still exposes memory delivery lanes");
  assert(!surface.innerHTML.includes("민감한 기억 상세 본문"), "completed HUD still exposes final memory item previews");
  assert(!surface.innerHTML.includes("protected_entity_not_in_current_input"), "completed HUD still exposes exclusion reason codes");
  for (let index = 1; index <= 12; index++) {
    assert(surface.innerHTML.includes("stage.label." + index), "completed HUD omitted stage " + index);
  }
  assert(surface.button, "completed HUD has no visible close button");
  assert(!surface.innerHTML.includes("x-mo-turn-hud"), "rendered HUD still depends on x-* attributes stripped by RisuAI");
  assert(surface.innerHTML.includes("원문 &lt;저장&gt;"), "dynamic HUD label was not HTML escaped");
  for (const value of ["1","2","3","4","5","6","21"]) {
    assert(surface.innerHTML.includes(">" + value + "</span>"), "completed HUD omitted count " + value);
  }
  assert(surface.card && typeof surface.card.listeners.click !== "function", "warning HUD still has a card-wide dismiss listener");
  assert(surface.button && typeof surface.button.listeners.click === "function", "warning HUD close button listener missing");
  assert(risuEventListeners.size === 1, "warning HUD registered more than one global listener");
  assert(surface.innerHTML !== "", "warning HUD disappeared before the close button was used");
  await dispatchRisuEvent("click", {clientX:50, clientY:50});
  await _turnWorkflowHUDRenderChain;
  assert(surface.innerHTML !== "", "global click outside the close button dismissed the warning HUD");
  const warningListenerId = surface.button.listenerIds.click;
  await renderTurnWorkflowHUD({
    contract_version:TURN_WORKFLOW_HUD_CONTRACT,request_id:"completed-a",revision:2,
    logical_turn:55,status:"completed_with_warning",severity:"warning",
    dismissal_policy:"x_only",counts,stages,facts,
    warnings:[{code:"PUBLISHER_SKIPPED",message_key:"warn.publisher",stage_key:"stage-4"}]
  });
  await _turnWorkflowHUDRenderChain;
  assert(risuEventListeners.size === 1, "HUD rerender leaked a global click listener");
  assert(!risuEventListeners.has(warningListenerId), "HUD rerender retained its prior global click listener");
  await dispatchRisuEvent("click", {clientX:120, clientY:20});
  await _turnWorkflowHUDRenderChain;
  assert(surface.innerHTML === "", "warning HUD close button did not dismiss HUD");
  assert(risuEventListeners.size === 0, "warning HUD dismissal retained its global listener");

  assert(consumeTurnWorkflowHUD({
    contract_version:TURN_WORKFLOW_HUD_CONTRACT,request_id:"completed-info",revision:1,
    logical_turn:55,status:"completed",severity:"normal",dismissal_policy:"card_or_x",counts,stages
  }), "normal completed HUD view was rejected");
  await _turnWorkflowHUDRenderChain;
  assert(surface.card && typeof surface.card.listeners.click === "function", "normal completed HUD lost card-wide dismissal");
  await dispatchRisuEvent("click", {clientX:50, clientY:50});
  await _turnWorkflowHUDRenderChain;
  assert(surface.innerHTML === "", "normal completed HUD card click did not dismiss HUD");

  const startedAt = new Date(Date.now() - 2200).toISOString();
  assert(consumeTurnWorkflowHUD({
    contract_version:TURN_WORKFLOW_HUD_CONTRACT,request_id:"running-b",revision:1,
    logical_turn:56,status:"running",severity:"normal",dismissal_policy:"none",
    current_stage:{ordinal:3,total:7,label_key:"turn_hud.stage.prepare_source",llm_call:true,status:"running",started_at:startedAt}
  }), "running HUD view was rejected");
  await _turnWorkflowHUDRenderChain;
  assert(surface.elapsed && /초$/.test(surface.elapsed.textContent), "LLM elapsed seconds were not rendered");
  assert(elapsedTimers.size === 1, "LLM elapsed display did not schedule its fallback-safe timer");
  const firstElapsedTimer = Array.from(elapsedTimers.entries())[0];
  elapsedTimers.delete(firstElapsedTimer[0]);
  assert(firstElapsedTimer[1].delay === 250, "LLM elapsed display used an unexpected refresh cadence");
  firstElapsedTimer[1].fn();
  await Promise.resolve();
  await Promise.resolve();
  assert(elapsedTimers.size === 1, "LLM elapsed display did not continue without requestAnimationFrame");
  assert(surface.innerHTML.includes("height:3px") && surface.innerHTML.includes("width:42.9%"), "running HUD progress bar does not reflect the backend stage ordinal");
  await dismissTurnWorkflowHUD("running-b");
  await _turnWorkflowHUDRenderChain;
  assert(clearedElapsedTimers.length >= 1, "HUD dismissal did not cancel the elapsed timer");

  assert(consumeTurnWorkflowHUDNotice({
    contract_version:TURN_WORKFLOW_HUD_CONTRACT,request_id:"ooc-backend-notice",revision:1,
    logical_turn:56,host_turn:56,backend_turn:0,
    status:"completed",severity:"notice",dismissal_policy:"card_or_x",display_mode:"notice",
    title_key:"turn_hud.notice.ooc_recognized",message_key:"turn_hud.notice.ooc_recognized_detail",
    notice_code:"OOC_INPUT_CANCELLED",notice_kind:"ooc",presentation_tone:"attention",
    stages:[],counts:[],warnings:[],facts:[{key:"host_observation",status:"observed",disposition:"dropped",severity:"notice"}]
  }), "backend OOC notice was rejected");
  await _turnWorkflowHUDRenderChain;
  assert(surface.innerHTML.includes("OOC 인식"), "OOC decision did not replace the awaiting-response title");
  assert(surface.innerHTML.includes("OOC 판정으로 입력 처리를 취소했습니다."), "OOC cancellation detail was not rendered");
  assert(surface.card.attributes.style.includes("rgba(245,196,81,.58)"), "OOC notice did not use the yellow attention accent");
  assert(surface.innerHTML.includes("color:#F5C451"), "OOC notice title did not use the yellow attention color");
  assert(_turnWorkflowHUDWatchRunning === false, "OOC notice left the workflow status watcher running");
  assert(surface.card && typeof surface.card.listeners.click === "function", "OOC informational notice lost normal card dismissal");
  await dispatchRisuEvent("click", {clientX:50, clientY:50});
  await _turnWorkflowHUDRenderChain;
  assert(surface.innerHTML === "", "OOC informational notice did not dismiss");
  assert(consumeTurnWorkflowHUDNotice({
    contract_version:TURN_WORKFLOW_HUD_CONTRACT,request_id:"delete-confirmed",revision:1,
    logical_turn:56,status:"completed",severity:"notice",dismissal_policy:"card_or_x",display_mode:"notice",
    title_key:"turn_hud.notice.delete_confirmed",message_key:"turn_hud.notice.delete_confirmed_detail",
    notice_code:"ASSISTANT_OUTPUT_DELETE_CONFIRMED",counts:[],stages:[],warnings:[]
  }), "backend deletion notice was rejected");
  await _turnWorkflowHUDRenderChain;
  assert(surface.innerHTML.includes("삭제 확인 테스트"), "backend deletion notice title was not rendered");
  assert(surface.innerHTML.includes("삭제 출력 정리 테스트"), "backend deletion notice detail was not rendered");
  assert(surface.card && typeof surface.card.listeners.click === "function", "successful deletion notice lost card dismissal");
  await dispatchRisuEvent("click", {clientX:50, clientY:50});
  await _turnWorkflowHUDRenderChain;
  assert(surface.innerHTML === "", "successful deletion notice did not dismiss");

  assert(consumeTurnWorkflowHUD({
    contract_version:TURN_WORKFLOW_HUD_CONTRACT,request_id:"reroll-confirmed",revision:1,
    logical_turn:56,status:"completed",severity:"notice",dismissal_policy:"card_or_x",display_mode:"notice",
    title_key:"turn_hud.notice.reroll_confirmed",message_key:"turn_hud.notice.reroll_confirmed_detail",
    notice_code:"LOGICAL_TURN_REPLACED",counts:[],stages:[],warnings:[]
  }), "backend reroll notice was rejected");
  await _turnWorkflowHUDRenderChain;
  assert(surface.innerHTML.includes("리롤 확인 테스트"), "backend reroll notice title was not rendered");
  assert(surface.innerHTML.includes("기존 턴 교체 테스트"), "backend reroll notice detail was not rendered");
  assert(surface.card && typeof surface.card.listeners.click === "function", "successful reroll notice lost card dismissal");
  await dispatchRisuEvent("click", {clientX:50, clientY:50});
  await _turnWorkflowHUDRenderChain;
  assert(surface.innerHTML === "", "successful reroll notice did not dismiss");

  assert(consumeTurnWorkflowHUD({
    contract_version:TURN_WORKFLOW_HUD_CONTRACT,request_id:"failed-c",revision:1,
    logical_turn:57,status:"failed",severity:"error",dismissal_policy:"x_only",
    stages:stages.map(function(stage, index) {
      return index === 8
        ? {...stage,status:"failed",duration_ms:800,reason_code:"CRITIC_LLM_FAILED"}
        : stage;
    }),
    facts,
    memory_selection:memorySelection,
    error:{code:"BAD_<CODE>",message_key:"turn_hud.transport_unavailable",retryable:false,preserved_counts:counts}
  }), "failed HUD view was rejected");
  await _turnWorkflowHUDRenderChain;
  assert(surface.innerHTML.includes("BAD_&lt;CODE&gt;"), "error metadata was not HTML escaped");
  assert(surface.card.attributes.style.includes("background:#2A151D"), "failed HUD has no opaque fintech error window");
  assert(surface.innerHTML.includes("color:#E158A6"), "failed HUD does not use the supplied pink error accent");
  assert(surface.innerHTML.includes("실패 · 0.8초"), "failed HUD omitted failed stage status or duration");
  assert(surface.innerHTML.includes("CRITIC_LLM_FAILED"), "failed HUD omitted the failed stage reason code");
  assert(surface.innerHTML.includes(">21</span>"), "failed HUD omitted preserved generated counts");
  assert(!surface.innerHTML.includes("WORKFLOW FACTS"), "failed HUD still exposes internal workflow facts");
  assert(!surface.innerHTML.includes("민감한 기억 상세 본문"), "failed HUD still exposes memory item previews");
  for (let index = 1; index <= 12; index++) {
    assert(surface.innerHTML.includes("stage.label." + index), "failed HUD omitted stage " + index);
  }
  assert(surface.button, "failed HUD has no visible close button");
  const failedCard = surface.card;
  assert(typeof failedCard.listeners.click !== "function", "failed HUD still has a card-wide dismiss listener");
  assert(typeof failedCard.listeners.keydown !== "function", "failed HUD still has a card-wide keyboard dismiss listener");
  assert(typeof surface.button.listeners.keydown !== "function", "failed HUD registered an unidentifiable global keydown listener");
  assert(risuEventListeners.size === 1, "failed HUD registered more than one global listener");
  await dispatchRisuEvent("click", {clientX:50, clientY:50});
  await _turnWorkflowHUDRenderChain;
  assert(surface.innerHTML !== "", "global click outside the failed HUD close button dismissed it");
  await dispatchRisuEvent("click", {clientX:120, clientY:20});
  await _turnWorkflowHUDRenderChain;
  assert(surface.innerHTML === "", "failed HUD close button did not dismiss HUD");

  const recoverableError = {
    code:"DERIVED_PERSIST_FAILED",
    message_key:"turn_hud.transport_unavailable",
    retryable:true,
    preserved_counts:counts,
    recovery_actions:[{
      id:"retry_derived_turn",
      label_key:"turn_hud.recovery.retry_derived_turn",
      confirm_title_key:"turn_hud.recovery.confirm_title",
      confirm_message_key:"turn_hud.recovery.confirm_retry_derived_turn",
      status:"available"
    }]
  };
  const recoverableView = {
    contract_version:TURN_WORKFLOW_HUD_CONTRACT,request_id:"recoverable-turn",revision:1,
    logical_turn:57,status:"failed",severity:"error",dismissal_policy:"x_only",
    stages,counts,error:recoverableError
  };
  recoveryResponseView = {
    ...recoverableView,
    revision:2,
    error:{
      ...recoverableError,
      recovery_actions:[{
        ...recoverableError.recovery_actions[0],
        status:"requested",
        status_message_key:"turn_hud.recovery.requested"
      }]
    }
  };
  assert(consumeTurnWorkflowHUD(recoverableView), "recoverable failed HUD view was rejected");
  await _turnWorkflowHUDRenderChain;
  assert(surface.innerHTML.includes("이 턴 복구 재시도"), "recoverable HUD omitted its backend-supplied action");
  assert(surface.recoveryButton && typeof surface.recoveryButton.listeners.click === "function", "recoverable HUD action listener missing");
  assert(risuEventListeners.size === 2, "recoverable HUD registered duplicate global listeners");
  await dispatchRisuEvent("click", {clientX:50, clientY:50});
  await _turnWorkflowHUDRenderChain;
  assert(recoveryConfirmCalls === 0, "click outside the recovery button opened its confirmation");
  assert(recoveryBridgeCalls.length === 0, "click outside the recovery button called the backend");
  recoveryConfirmResult = false;
  await dispatchRisuEvent("click", {clientX:70, clientY:175});
  await _turnWorkflowHUDRenderChain;
  assert(recoveryConfirmCalls === 1, "recovery action did not confirm exactly once");
  assert(recoveryBridgeCalls.length === 0, "declined recovery action called the backend");
  assert(surface.innerHTML === "", "declined recovery action left its failed HUD active");
  await dispatchRisuEvent("click", {clientX:50, clientY:50});
  assert(recoveryConfirmCalls === 1, "declined recovery action reopened on a later click");

  recoveryConfirmResult = true;
  assert(consumeTurnWorkflowHUD(recoverableView), "recoverable failed HUD view was rejected after decline");
  await _turnWorkflowHUDRenderChain;
  await dispatchRisuEvent("click", {clientX:70, clientY:175});
  await _turnWorkflowHUDRenderChain;
  assert(recoveryConfirmCalls === 2, "accepted recovery action did not confirm exactly once");
  assert(recoveryBridgeCalls.length === 1, "recovery action did not call the backend exactly once");
  assert(recoveryBridgeCalls[0].path === "/turn-workflow/recovery", "recovery action called the wrong backend route");
  assert(recoveryBridgeCalls[0].options.body.request_id === "recoverable-turn", "recovery action lost its request identity");
  assert(surface.innerHTML.includes("복구 요청을 보냈습니다."), "recoverable HUD did not render the accepted recovery state");
  assert(!surface.recoveryButton, "accepted recovery action stayed clickable");
  await dispatchRisuEvent("click", {clientX:120, clientY:20});
  await _turnWorkflowHUDRenderChain;
  assert(surface.innerHTML === "", "recovery status HUD close button did not dismiss HUD");

  recoveryBridgeFailure = true;
  _lastBridgeFailureByPath.set("/turn-workflow/recovery", {
    kind:"http_error",path:"/turn-workflow/recovery",method:"POST",status:409,
    detail:"the failed workflow is not bound to a durable source revision",
    response_body:JSON.stringify({status:"error",code:"recovery_target_unavailable",error:"복구할 원본 기억을 확정하지 못했습니다."}),
    at:Date.now()
  });
  const failedRecoveryView = {
    ...recoverableView,
    request_id:"recoverable-http-error",
    revision:1
  };
  assert(consumeTurnWorkflowHUD(failedRecoveryView), "HTTP-error recovery HUD view was rejected");
  await _turnWorkflowHUDRenderChain;
  await dispatchRisuEvent("click", {clientX:70, clientY:175});
  await _turnWorkflowHUDRenderChain;
  assert(recoveryBridgeCalls.length === 2, "HTTP-error recovery did not call the backend exactly once");
  assert(surface.innerHTML.includes("recovery_target_unavailable"), "structured backend recovery code was hidden");
  assert(surface.innerHTML.includes("복구할 원본 기억을 확정하지 못했습니다."), "structured backend recovery message was hidden");
  assert(!surface.innerHTML.includes("missing its HUD ViewModel"), "structured 409 was replaced by a missing-ViewModel error");
  recoveryBridgeFailure = false;
  await dispatchRisuEvent("click", {clientX:120, clientY:20});
  await _turnWorkflowHUDRenderChain;
  assert(surface.innerHTML === "", "HTTP-error recovery HUD close button did not dismiss HUD");

  assert(consumeTurnWorkflowHUD({
    contract_version:TURN_WORKFLOW_HUD_CONTRACT,request_id:"invalidated-d",revision:1,
    logical_turn:58,status:"invalidated",severity:"warning",dismissal_policy:"x_only",counts,
    stages:stages.map(function(stage, index) {
      return index === 5
        ? {...stage,status:"invalidated",duration_ms:640,reason_code:"superseded_by_new_attempt"}
        : stage;
    })
  }), "invalidated HUD view was rejected");
  await _turnWorkflowHUDRenderChain;
  assert(surface.innerHTML.includes("작업 중단"), "invalidated HUD omitted terminal title: " + surface.innerHTML);
  assert(surface.innerHTML.includes("중단 · 0.64초"), "invalidated HUD omitted stage status or duration");
  assert(surface.innerHTML.includes("superseded_by_new_attempt"), "invalidated HUD omitted visible reason");
  assert(surface.card.attributes.style.includes("background:#1C1828"), "invalidated HUD does not use warning styling");
  assert(!surface.card.attributes.style.includes("background:#2A151D"), "invalidated HUD was incorrectly rendered as a red error");
  assert(surface.button, "invalidated HUD has no visible close button");
  assert(typeof surface.card.listeners.click !== "function", "invalidated HUD still has a card-wide dismiss listener");
  assert(typeof surface.button.listeners.click === "function", "invalidated HUD close button listener missing");
  await dispatchRisuEvent("click", {clientX:120, clientY:20});
  await _turnWorkflowHUDRenderChain;
  assert(surface.innerHTML === "", "invalidated HUD close button did not dismiss HUD");

  settings.turnWorkflowHUDEnabled = false;
  assert(!consumeTurnWorkflowHUD({
    contract_version:TURN_WORKFLOW_HUD_CONTRACT,request_id:"disabled-e",revision:1,
    logical_turn:59,status:"completed",severity:"normal",dismissal_policy:"card_or_x",counts
  }), "disabled HUD accepted a backend view");
  startTurnWorkflowHUDWatch("disabled-e");
  await _turnWorkflowHUDRenderChain;
  assert(_turnWorkflowHUDActiveRequestId === "", "disabled HUD started a request watch");
  assert(surface.innerHTML === "", "disabled HUD left visible content behind");

  settings.turnWorkflowHUDEnabled = true;
  await renderTurnWorkflowHUD({
    contract_version:TURN_WORKFLOW_HUD_CONTRACT,request_id:"unload-final",revision:1,
    logical_turn:60,status:"completed",severity:"normal",dismissal_policy:"card_or_x",counts,stages
  });
  await _turnWorkflowHUDRenderChain;
  assert(risuEventListeners.size === 1, "unload fixture did not register its global listener");
  const unloadElapsedTimer = setTimeout(function() {}, 250);
  _turnWorkflowHUDElapsedTimer = unloadElapsedTimer;
  await unloadTurnWorkflowHUD();
  assert(clearedElapsedTimers.includes(unloadElapsedTimer), "HUD unload did not cancel its elapsed timer");
  assert(risuEventListeners.size === 0, "HUD unload retained a global listener");
  assert(!body.children.includes(root), "HUD unload left its owned root attached");
  assert(!nodesByClass.has("mo-turn-workflow-hud-root"), "HUD unload left its owned root queryable");
  assert(_turnWorkflowHUDMainDocument === null, "HUD unload retained the main RootDocument handle");
  process.stdout.write("ok");
})().catch(function(err) {
  console.error(err && err.stack || err);
  process.exit(1);
});
`
	scriptPath := t.TempDir() + "/turn-workflow-hud-runtime.js"
	if err := os.WriteFile(scriptPath, []byte(script), 0600); err != nil {
		t.Fatalf("write turn workflow HUD runtime fixture: %v", err)
	}
	command := exec.Command(nodePath, scriptPath)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("turn workflow HUD main RootDocument runtime fixture failed: %v\n%s", err, output)
	}
	if strings.TrimSpace(string(output)) != "ok" {
		t.Fatalf("turn workflow HUD runtime fixture output=%q, want ok", output)
	}
}

func TestTryCompleteTurnRecordsOnlyBackendConfirmedReroll(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for confirmed reroll adapter fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functionBody := extractArchiveCenterJSAsyncFunction(t, src, "tryCompleteTurn")
	script := functionBody + `
const settings = {enabled:true,dbEnabled:true};
let nextResult = null;
const runtimeUpdates = [];
function buildCompleteTurnRequestBody() { return Promise.resolve({client_meta:{turn_workflow_request_id:"request-reroll"}}); }
function turnWorkflowHUDRequestIdFromCompleteBody(body) { return body.client_meta.turn_workflow_request_id; }
function startTurnWorkflowHUDWatch() {}
function consumeTurnWorkflowHUD() {}
function renderTurnWorkflowHUDTransportError() {}
function getCompleteTurnTimeoutMs() { return 1000; }
function bridgeFetchWithRetry() { return Promise.resolve(nextResult); }
async function safeCall(call) { return await call(); }
function debugLog() {}
function updateRuntimeState(key, status, extra) { runtimeUpdates.push({key,status,extra}); }
function assert(condition, message) { if (!condition) throw new Error(message); }
(async function() {
  nextResult = {
    status:"ok",turn_index:7,
    source_acceptance:{accepted:true,replace_existing:true,lifecycle:"active_final"},
    turn_workflow_hud:{contract_version:"turn_workflow_hud.v3",request_id:"request-reroll",status:"completed"}
  };
  await tryCompleteTurn(8, "user", "new answer", [], "session-1", null, null);
  assert(runtimeUpdates.length === 1, "confirmed reroll was not recorded exactly once");
  assert(runtimeUpdates[0].key === "lastRerollReplacement", "wrong runtime state key");
  assert(runtimeUpdates[0].status === "ok", "confirmed reroll status is not ok");
  assert(runtimeUpdates[0].extra.detail === "logical_turn_replaced", "stable reroll detail code missing");
  assert(runtimeUpdates[0].extra.turnIndex === 7, "backend-bound logical turn was not retained");

  nextResult = {
    status:"rejected",turn_index:8,
    source_acceptance:{accepted:false,replace_existing:true,lifecycle:"candidate_or_inactive"}
  };
  await tryCompleteTurn(8, "user", "candidate", [], "session-1", null, null);
  assert(runtimeUpdates.length === 1, "rejected candidate was misreported as a reroll");
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("confirmed reroll adapter fixture failed: %v\n%s", err, out)
	}
}

func TestPrepareTurnSourceCapabilityContractRuntime(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for Archive Center prepare-turn runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	fn := extractArchiveCenterJSFunction(t, src, "estimateAdaptiveInjectionBudgetParts") + "\n" +
		extractArchiveCenterJSFunction(t, src, "buildRisuRequestObservation") + "\n" +
		extractArchiveCenterJSAsyncFunction(t, src, "observeRisuPersona") + "\n" +
		extractArchiveCenterJSAsyncFunction(t, src, "tryPrepareTurn")
	script := fn + `
const R = {
  getDatabase: async () => ({selectedPersona: 0, personas: [{id: "persona-a", name: "Mira"}]}),
  getCurrentCharacterIndex: async () => 0,
  getCurrentChatIndex: async () => 0,
  getChatFromIndex: async () => ({bindedPersona: "persona-a"})
};
const settings = {
  narrativeGuideMode: "off", narrativeGuideStrength: "weak",
  pluginMainApplyMode: "shadow", inputContextEnabled: true, maxInjectionChars: 1000, injectionBudgetExtraChars: 0,
  topK: 3, injectionEnabled: true, primaryCanonBaseMaxChars: 1000,
  maxInputContextChars: 800, episodeIntervalTurns: 10,
  lorebookReferenceMode: "reference_assist",
  embeddingApiKey: "", embeddingEndpoint: "", embeddingModel: "", embeddingProvider: "off", embeddingTimeout: 1
};
const DEFAULT_SETTINGS = {maxInjectionChars: 1000, referenceInjectionMaxChars: 3000, lorebookReferenceMaxChars: 3000, topK: 3, episodeIntervalTurns: 10, embeddingProvider: "off", lorebookReferenceMode: "reference_assist"};
function normalizeNarrativeGuideStrength(value) { return value === "none" ? "none" : "weak"; }
function getPayloadMessageRoleAndText(message) { return {role: message.role || "", text: message.content || ""}; }
function sanitizeTopKSetting(value) { return Number(value || 0); }
function normalizeLanguageContextTrace(value) { return value || null; }
function normalizeEmbeddingProvider(value) { return value; }
function getEmbeddingTimeoutMs() { return 1000; }
function getRequestTimeoutSettingMs() { return 1000; }
function debugLog() {}
let lorebookSyncCalls = 0;
async function syncCurrentLorebookReference(options) {
  if (!options || options.sessionId !== "session-a") throw new Error("wrong lorebook session");
  lorebookSyncCalls++;
  return {status:"current"};
}
function currentLorebookReferencePrepareScope(sessionId) {
  if (sessionId !== "session-a") return null;
  return {contract_version:"lorebook_reference_scope.v1",observation_state:"observed",character_index:0,chat_index:0,enabled_module_ids:[],enabled_modules_observed:true};
}
const hudWatchIds = [];
function turnWorkflowHUDRequestIdFromPrepareOptions(options) {
  return String(options && options.sourceObservation && options.sourceObservation.request_id || "");
}
function startTurnWorkflowHUDWatch(requestId) { hudWatchIds.push(String(requestId || "")); }
function consumeTurnWorkflowHUD() { return true; }
function stopTurnWorkflowHUDWatch() {}
function renderTurnWorkflowHUDTransportError() {}
let capturedBody = null;
const capturedBodies = [];
let expectedLane = {
  status: "degraded", reason_code: "source_observation_optional_capability_missing",
  retryable: false, affected_lane: "source_observation", original_payload_preserved: true,
  contract_version: "source_lane_status.v1", capability_coverage: {}, request_correlation_id: "request-a"
};
async function bridgeFetch(path, options) {
  if (path !== "/prepare-turn") throw new Error("unexpected path " + path);
  if (options.timeoutMs !== 0) throw new Error("prepare-turn retained the Plugin Timeout: " + options.timeoutMs);
  capturedBody = options.body;
  capturedBodies.push(options.body);
  return {
    status: "ok", source: "shadow", fallback_reason: "",
    source_contract: {contract_version: "prepare_source_projection.v1", lane_status: expectedLane},
    current_input_decision: {status: "eligible", reason_code: "current_user_input_observed"},
    session_bootstrap: {status: "preserved", reason_code: "bootstrap_sources_preserved"}
  };
}

(async function() {
  const sourceObservation = {
    contract_version: "message_source_observation.v1", session_id: "session-a", request_id: "request-a",
    message_index: 0, observed_role: "user", observable: true, evidence_state: "observed"
  };
  const capabilityObservation = {
    contract_version: "host_source_capabilities.v1",
    capabilities: {session_identity: "observed", request_correlation: "observed", message_position: "observed", message_role: "observed"}
  };
  const hostObservations = {contract_version: "prepare_host_observations.v1", session_id: "session-a", request_id: "request-a"};
  const bootstrapObservation = {contract_version: "session_bootstrap_observation.v1", session_id: "session-a", request_id: "request-a", leading_messages: []};
  for (const status of ["degraded", "failed", "incompatible"]) {
    expectedLane = Object.assign({}, expectedLane, {status, reason_code: "source_observation_" + status});
    const result = await tryPrepareTurn("session-a", "hello", [{role: "user", content: "hello"}], null, "model", null, {
      sourceObservation, capabilityObservation, hostObservations, bootstrapObservation
    });
    if (!capturedBody) throw new Error("request body not captured");
    if (capturedBody.settings.lorebook_reference_mode !== "reference_assist" || !capturedBody.lorebook_reference_scope) {
      throw new Error("always-enabled lorebook scope was not forwarded");
    }
    if (capturedBody.source_observation !== sourceObservation || capturedBody.capability_observation !== capabilityObservation) {
      throw new Error("host observations were not forwarded unchanged");
    }
    if (capturedBody.host_observations !== hostObservations || capturedBody.bootstrap_observation !== bootstrapObservation) {
      throw new Error("3.3-D host/bootstrap observations were not forwarded unchanged");
    }
    if (!capturedBody.client_meta || capturedBody.client_meta.risu_persona_observation.persona_name !== "Mira") {
      throw new Error("official RisuAI persona observation was not forwarded");
    }
    for (const forbidden of ["authority", "stable_identity", "canonical_truth", "lifecycle_acceptance", "persistence"]) {
      if (Object.prototype.hasOwnProperty.call(capturedBody.source_observation, forbidden)) throw new Error("forbidden inference " + forbidden);
    }
    if (!result.sourceContract || result.sourceContract.lane_status.status !== status) {
      throw new Error("stable " + status + " status was not preserved");
    }
    if (!result.bundle.sourceContract || result.bundle.sourceContract.lane_status.reason_code !== expectedLane.reason_code) {
      throw new Error("stable reason code was not preserved in bundle");
    }
    if (!result.currentInputDecision || result.currentInputDecision.status !== "eligible" || !result.sessionBootstrap || result.sessionBootstrap.status !== "preserved") {
      throw new Error("Go-owned 3.3-D decisions were not returned to the adapter");
    }
  }
  R.getChatFromIndex = async () => ({bindedPersona: "persona-missing"});
  const unresolvedBinding = await observeRisuPersona();
  if (
    unresolvedBinding.observation_state !== "unobserved" ||
    unresolvedBinding.reason !== "chat_bound_persona_not_resolved"
  ) {
    throw new Error("unresolved explicit chat persona binding did not fail closed");
  }
  R.getChatFromIndex = async () => ({bindedPersona: "persona-a"});
  const hudWatchCountBeforeDecision = hudWatchIds.length;
  const decisionResult = await tryPrepareTurn("session-a", "", [{role: "user", content: "hello"}], null, "model", null, {
    sourceDecisionOnly: true, sourceObservation, capabilityObservation, hostObservations, bootstrapObservation
  });
  if (hudWatchIds.length !== hudWatchCountBeforeDecision) {
    throw new Error("source-decision-only request incorrectly started the workflow HUD");
  }
  const decisionBody = capturedBodies[capturedBodies.length - 1];
  if (decisionBody.source_decision_only !== true || decisionBody.host_observations !== hostObservations) {
    throw new Error("read-free source decision phase was not transported with the same host observations");
  }
  if (!decisionResult.currentInputDecision || decisionResult.currentInputDecision.status !== "eligible") {
    throw new Error("source decision response was not returned to the production adapter");
  }
  const languageContext = {session_output_language: "ko", output_language_override: "ko"};
  await tryPrepareTurn("session-a", "hello", [{role: "user", content: "hello"}], {
    triggerMode: "manual_resume", query: "continue the unresolved thread"
  }, "model", languageContext, {
    freshFirstTurnLightMode: true,
    freshFirstTurnLightModeMeta: {activeCompletedPairs: 0, latestBackendTurn: 0, routingBaselineBackendTurn: 0},
    sourceObservation, capabilityObservation, hostObservations, bootstrapObservation
  });
  const fullBody = capturedBodies[capturedBodies.length - 1];
  if (fullBody.source_decision_only === true) throw new Error("full prepare was mislabeled as decision-only");
  if (fullBody.continuity_trigger_mode !== "manual_resume" || fullBody.continuity_query !== "continue the unresolved thread") {
    throw new Error("continuityInfo was not delivered to full prepare");
  }
  if (!fullBody.client_meta || fullBody.client_meta.language_context !== languageContext || fullBody.output_language_override !== "ko") {
    throw new Error("languageContext was not delivered to full prepare");
  }
  if (fullBody.host_observations !== hostObservations || fullBody.bootstrap_observation !== bootstrapObservation) {
    throw new Error("full prepare did not repeat the correlated host observations");
  }
  if (fullBody.settings.top_k !== 0 || fullBody.settings.max_injection_chars !== 0 || fullBody.settings.reference_injection_budget_basis_chars !== 3000 || fullBody.settings.lorebook_reference_max_chars !== 3000 || fullBody.settings.max_input_context_chars !== 0 || fullBody.settings.injection_enabled !== true || Object.prototype.hasOwnProperty.call(fullBody.settings, "input_context_enabled")) {
    throw new Error("fresh-first-turn memory recall was not suppressed independently from guide and Go-default input context");
  }
  await tryPrepareTurn("session-a", "hello", [{role: "user", content: "hello"}], null, "model", null, {
    sourceObservation, capabilityObservation, hostObservations, bootstrapObservation
  });
  const existingSessionBody = capturedBodies[capturedBodies.length - 1];
  if (existingSessionBody.settings.top_k !== 3 || existingSessionBody.settings.max_injection_chars !== 1000 || existingSessionBody.settings.reference_injection_budget_basis_chars !== 3000 || existingSessionBody.settings.lorebook_reference_max_chars !== 3000 || Object.prototype.hasOwnProperty.call(existingSessionBody.settings, "input_context_enabled")) {
    throw new Error("existing-session prepare budget regressed");
  }
  settings.injectionBudgetExtraChars = 2500;
  await tryPrepareTurn("session-a", "hello", [{role: "user", content: "hello"}], null, "model", null, {
    runtimeTokenInfo: {currentChatTokens: 9135, source: "message_char_estimate"},
    sourceObservation, capabilityObservation, hostObservations, bootstrapObservation
  });
  const adaptiveBudgetBody = capturedBodies[capturedBodies.length - 1];
  if (adaptiveBudgetBody.settings.max_injection_chars !== 1000 ||
      adaptiveBudgetBody.client_meta.memory_budget_observation.extra_chars !== 2500) {
    throw new Error("configured budget and dynamic observation were not forwarded separately: " + JSON.stringify(adaptiveBudgetBody));
  }
  settings.narrativeGuideMode = "auto";
  settings.narrativeGuideStrength = "weak";
  settings.publisherGuidanceFormat = "explicit";
  settings.pluginMainApplyMode = "off";
  await tryPrepareTurn("session-a", "hello", [{role: "user", content: "hello"}], null, "model", null, {
    sourceObservation, capabilityObservation, hostObservations, bootstrapObservation
  });
  const guideAutoBody = capturedBodies[capturedBodies.length - 1];
  if (guideAutoBody.settings.apply_mode !== "off" || guideAutoBody.settings.guide_mode !== "auto" || guideAutoBody.settings.guide_strength !== "weak" || guideAutoBody.publisher_guidance_format !== "explicit" || guideAutoBody.settings.supervisor_enabled !== true || Object.prototype.hasOwnProperty.call(guideAutoBody.settings, "input_context_enabled")) {
    throw new Error("optional input improvement was not independent from Go-owned guide and input context: "+JSON.stringify(guideAutoBody.settings));
  }
  settings.narrativeGuideStrength = "none";
  await tryPrepareTurn("session-a", "hello", [{role: "user", content: "hello"}], null, "model", null, {
    sourceObservation, capabilityObservation, hostObservations, bootstrapObservation
  });
  const guideOffBody = capturedBodies[capturedBodies.length - 1];
  if (guideOffBody.settings.guide_mode !== "off" || guideOffBody.settings.guide_strength !== "none" || guideOffBody.settings.supervisor_enabled !== false) {
    throw new Error("guide none was not transported as an explicit OFF contract: "+JSON.stringify(guideOffBody.settings));
  }
  if (!hudWatchIds.includes("request-a")) throw new Error("full prepare did not start the correlated workflow HUD");
  if (lorebookSyncCalls !== capturedBodies.length) throw new Error("prepare-turn did not synchronize the active lorebook scope");
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-e", script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("prepare-turn source/capability runtime fixture failed: %v\n%s", err, output)
	}
}

func TestLorebookReferenceAdapterReadsOnlyOnScopeChangeOrManualRefresh(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for lorebook reference adapter runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	for _, marker := range []string{
		`lorebookReferenceMode: "reference_assist"`,
		`merged.lorebookReferenceMode === "off" ? DEFAULT_SETTINGS.lorebookReferenceMode`,
	} {
		if !strings.Contains(src, marker) {
			t.Fatalf("Archive Center.js missing always-enabled lorebook marker %q", marker)
		}
	}
	for _, forbidden := range []string{
		`lorebookReferenceMode: "off"`,
		`function revokeLastLorebookReferenceScope`,
		`consent_state: "revoked"`,
		`<option value="off"' + ((settings.lorebookReferenceMode`,
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("Archive Center.js retained disabled lorebook path %q", forbidden)
		}
	}
	functions := extractArchiveCenterJSFunction(t, src, "canonicalLorebookReferenceModuleIds") + "\n" +
		extractArchiveCenterJSAsyncFunction(t, src, "observeLorebookReferenceScope") + "\n" +
		extractArchiveCenterJSFunction(t, src, "lorebookReferenceScopeKey") + "\n" +
		extractArchiveCenterJSFunction(t, src, "currentLorebookReferencePrepareScope") + "\n" +
		extractArchiveCenterJSAsyncFunction(t, src, "postLorebookReferenceSnapshot") + "\n" +
		extractArchiveCenterJSAsyncFunction(t, src, "syncCurrentLorebookReference")
	script := functions + `
const SESSION_FALLBACK = "default";
const settings = {lorebookReferenceMode:"search_only"};
const _lorebookReferenceSync = {attemptedScopeKey:"", syncedScopeKey:"", inFlight:null, lastScope:null};
let characterIndex = 2;
let chatIndex = 3;
let enabledModules = ["module-b", "module-a"];
let lorebookReads = 0;
let shouldFailRead = false;
const snapshots = [];
const runtimeUpdates = [];
const R = {
  async getCurrentCharacterIndex(){ return characterIndex; },
  async getCurrentChatIndex(){ return chatIndex; },
  async getDatabase(paths){
    if (JSON.stringify(paths) !== JSON.stringify(["enabledModules"])) throw new Error("unexpected database path");
    return {enabledModules};
  },
  async getCurrentLorebookEntries(){
    lorebookReads++;
    if (shouldFailRead) throw new Error("host read failed");
    return [{id:"entry-1", key:"Han-eol", content:"Exam notice"}];
  }
};
function getRequestTimeoutSettingMs(){ return 1000; }
function updateRuntimeState(key, status, extra){ runtimeUpdates.push({key,status,extra}); }
async function getCurrentChatSessionId(){ return "session-a"; }
async function bridgeFetch(path, options){
  if (path !== "/sessions/session-a/lorebook-reference/snapshots") throw new Error("unexpected path " + path);
  snapshots.push(options.body);
  return {status:"ok", snapshot:{lifecycle_action:"current_projection_replaced"}};
}
(async function(){
  characterIndex = null;
  chatIndex = "";
  const unavailableScope = await observeLorebookReferenceScope("session-a");
  _lorebookReferenceSync.lastScope = unavailableScope;
  const partialPrepareScope = currentLorebookReferencePrepareScope("session-a");
  if (unavailableScope.character_index !== null || unavailableScope.chat_index !== null ||
      !partialPrepareScope || partialPrepareScope.observation_state !== "partial") {
    throw new Error("null or empty Host indexes were forged as index zero: " + JSON.stringify(partialPrepareScope));
  }
  characterIndex = 2;
  chatIndex = 3;
  _lorebookReferenceSync.lastScope = null;
  await syncCurrentLorebookReference({sessionId:"session-a"});
  await syncCurrentLorebookReference({sessionId:"session-a"});
  if (lorebookReads !== 1 || snapshots.length !== 1) {
    throw new Error("ordinary turn reread the full lorebook: reads=" + lorebookReads + " snapshots=" + snapshots.length);
  }
  if (snapshots[0].observation_state !== "observed" || snapshots[0].complete_snapshot !== true || snapshots[0].entries.length !== 1) {
    throw new Error("complete official snapshot was not forwarded: " + JSON.stringify(snapshots[0]));
  }
  if (JSON.stringify(snapshots[0].enabled_module_ids) !== JSON.stringify(["module-a","module-b"])) {
    throw new Error("module scope was not canonicalized: " + JSON.stringify(snapshots[0]));
  }
  const prepareScope = currentLorebookReferencePrepareScope("session-a");
  if (!prepareScope || prepareScope.contract_version !== "lorebook_reference_scope.v1" ||
      prepareScope.observation_state !== "observed" || prepareScope.character_index !== 2 ||
      prepareScope.chat_index !== 3 || JSON.stringify(prepareScope.enabled_module_ids) !== JSON.stringify(["module-a","module-b"])) {
    throw new Error("prepare-turn did not receive the exact observed scope: " + JSON.stringify(prepareScope));
  }
  enabledModules = ["module-c"];
  await syncCurrentLorebookReference({sessionId:"session-a"});
  if (lorebookReads !== 2 || snapshots.length !== 2) throw new Error("module scope change did not refresh once");
  await syncCurrentLorebookReference({sessionId:"session-a", force:true});
  if (lorebookReads !== 3 || snapshots.length !== 3) throw new Error("manual refresh did not read once");
  chatIndex = 4;
  shouldFailRead = true;
  await syncCurrentLorebookReference({sessionId:"session-a"});
  await syncCurrentLorebookReference({sessionId:"session-a"});
  if (lorebookReads !== 4 || snapshots.length !== 4) throw new Error("failed scope was retried on every ordinary turn");
  const unavailable = snapshots[3];
  if (unavailable.observation_state !== "unavailable" || unavailable.complete_snapshot !== false || unavailable.entries.length !== 0) {
    throw new Error("failed Host read pretended to be a complete replacement: " + JSON.stringify(unavailable));
  }
  settings.lorebookReferenceMode = "reference_assist";
  shouldFailRead = false;
  await syncCurrentLorebookReference({sessionId:"session-a", force:true});
  if (lorebookReads !== 5 || snapshots.length !== 5) throw new Error("reference assist did not refresh the Host lorebook");
})().catch(function(err){ console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("lorebook reference adapter runtime fixture failed: %v\n%s", err, output)
	}
}

func TestLegacyAutomaticInjectionBudgetMigratesOnceToCurrentBase(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for injection budget migration runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	fn := extractArchiveCenterJSFunction(t, src, "migrateLegacyInjectionBudgetSettings")
	script := "const DEFAULT_SETTINGS = {injectionBudgetProfileVersion: \"p34_9000_base_v1\"};\n" + fn + `
const legacy = migrateLegacyInjectionBudgetSettings({maxInjectionChars: 6000});
if (legacy.maxInjectionChars !== 9000) throw new Error("legacy default was not migrated: " + JSON.stringify(legacy));
const custom = migrateLegacyInjectionBudgetSettings({maxInjectionChars: 7500});
if (custom.maxInjectionChars !== 7500) throw new Error("non-default user value was overwritten: " + JSON.stringify(custom));
const versioned = migrateLegacyInjectionBudgetSettings({maxInjectionChars: 6000, injectionBudgetProfileVersion: "p34_9000_base_v1"});
if (versioned.maxInjectionChars !== 6000) throw new Error("versioned user value was migrated repeatedly: " + JSON.stringify(versioned));
`
	cmd := exec.Command(nodePath, "-e", script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("injection budget migration runtime fixture failed: %v\n%s", err, output)
	}
}

func TestBeforeRequestNonModelSkipsPrepareTurnRuntime(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required")
		}
	}
	src := readArchiveCenterJS(t)
	fn := extractArchiveCenterJSAsyncFunction(t, src, "onBeforeRequest")
	script := fn + `
const settings = {enabled: true};
function debugLog() {}
function warnLog() {}
function recordRisuHookLifecycle() {}
let recomposerClears = 0;
function clearArchiveCenterRecomposerBridge() { recomposerClears++; }
function isSaveType(type) { return type === "model"; }
let prepareCalls = 0;
async function tryPrepareTurn() { prepareCalls++; throw new Error("prepare-turn must not run"); }
(async function() {
  const payload = {messages: [{role: "user", content: "auxiliary"}]};
  const result = await onBeforeRequest(payload, "submodel");
  if (result !== payload) throw new Error("non-model payload identity changed");
  if (prepareCalls !== 0) throw new Error("non-model prepare calls=" + prepareCalls);
  if (recomposerClears !== 1) throw new Error("non-model request left stale Recomposer context");
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("non-model beforeRequest runtime fixture failed: %v\n%s", err, output)
	}
}

func TestBeforeRequestModelRunsDecisionThenFullWithContextRuntime(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required")
		}
	}
	src := readArchiveCenterJS(t)
	fn := extractArchiveCenterJSAsyncFunction(t, src, "onBeforeRequest")
	classifyFn := extractArchiveCenterJSFunction(t, src, "classifyLlmFailureReason")
	gateFn := extractArchiveCenterJSFunction(t, src, "buildLlmGateBlock")
	script := classifyFn + "\n" + gateFn + "\n" + fn + `
const settings = {enabled: true, debug: false};
let _sessionCache = null;
let _effectiveInputAwaitingNewTurn = false;
const _pendingPersistenceSkipBySession = new Map();
const _pendingOrchBySession = new Map();
let activePairs = 0;
let latestBackendTurn = 0;
let prepareCalls = [];
let runtimeConfigBindingCalls = 0;
let preFullSideEffects = 0;
let boundedHostLifecycleCalls = 0;
const hostObservationsFixture = {request_id: "request-runtime", payload: [{role: "user", raw_content: "actual input", message_index: 1}]};
const bootstrapObservationFixture = {request_id: "request-runtime"};
const languageContextFixture = {session_output_language: "ko", output_language_override: "ko"};
const continuityFixture = {triggerMode: "manual_resume", query: "unresolved thread"};
let currentContinuity = null;
function debugLog() {}
function warnLog() {}
function clearArchiveCenterRecomposerBridge() {}
function recordRisuHookLifecycle() {}
function isSaveType(type) { return type === "model"; }
function extractMessages(payload) { return {messages: payload.messages, path: ["messages"], hasMessageSlot: true}; }
function normalizeMessagesForOrchestration(messages) { return messages; }
function extractRuntimeCurrentChatTokenInfo() { return {}; }
async function getCurrentChatSessionId() { return "session-runtime"; }
async function resolveCanonicalWriteSessionId(value) { return value; }
async function getCurrentActiveChatSourceObservationMessages() { return [{role: "user", content: "actual input", risuMessageIndex: 1}]; }
function bindRawInputObservationToRequest(_sessionId, requestId) {
  return {text: "actual input", actualEmptyInput: false, observationId: 1, boundRequestId: requestId};
}
function makeOrchRequestId() { return "request-runtime"; }
function primeTurnWorkflowHUD() {}
function buildPostOutputSecondaryRequestContext() { return null; }
function buildPrepareTurnHostObservations() { return hostObservationsFixture; }
async function observePrepareTurnBootstrap() { return bootstrapObservationFixture; }
function buildPrepareTurnSourceObservations() { return {sourceObservation: {request_id: "request-runtime"}, capabilityObservation: {capabilities: {}}}; }
function updateRuntimeState() {}
async function ensureBackendRuntimeConfigBinding(instanceId) {
  runtimeConfigBindingCalls++;
  if (instanceId !== "backend-runtime") throw new Error("backend instance id was not forwarded");
  return {ok:true,bound:true,skipped:true,code:"config_sync_ok",backendInstanceId:instanceId,missingRoles:[]};
}
function ensureActiveChatCompletedTurnsBackfilled() { preFullSideEffects++; return Promise.resolve(); }
async function observePendingFinalConfirmationAtHostSignal() { boundedHostLifecycleCalls++; return {accepted:true}; }
async function captureAssistantPrefillSeedForSession() { boundedHostLifecycleCalls++; }
async function captureFinalConfirmationRequestContext() { boundedHostLifecycleCalls++; }
async function resolveRollbackComparableMessages() { preFullSideEffects++; return {messages: null, source: "fixture"}; }
function scrubOocDirectivesFromUserInput(text) { return {fullyOoc: false, changed: false, text}; }
function detectCurrentTurnOocInfo() { return {isOoc: false}; }
async function buildLanguageContextTrace() { return languageContextFixture; }
async function resolveContinuityTriggerInfo() { return currentContinuity; }
function getSessionRoutingTurnBaseline() { return {backendTurnAtRoute: 0}; }
async function safeCall(call, fallback) { try { return await call(); } catch { return fallback; } }
async function resolveActiveChatCompletedTurnsForRoutingBaseline() { return activePairs; }
async function fetchBackendLatestTurnIndexForSession() { return latestBackendTurn; }
async function tryPrepareTurn(sessionId, userInput, messages, continuityInfo, type, languageContext, options) {
  prepareCalls.push({sessionId, userInput, messages, continuityInfo, type, languageContext, options});
  if (options && options.sourceDecisionOnly === true) {
    return {source: "backend-source-decision", backendInstanceId: "backend-runtime", currentInputDecision: {
      status: "eligible", reason_code: "current_user_input_observed", effective_user_input: "actual input",
      selected_observation_ref: "input-hook:request-runtime", context_injection_eligible: true, memory_reads_allowed: true
    }, sessionBootstrap: {status: "preserved"}};
  }
  return null;
}
async function runFixture(expectedFresh, expectedContinuity) {
  prepareCalls = [];
  preFullSideEffects = 0;
  boundedHostLifecycleCalls = 0;
  runtimeConfigBindingCalls = 0;
  const payload = {messages: [{role: "system", content: "preset"}, {role: "user", content: "actual input"}, {role: "user", content: "later host prompt"}]};
  const result = await onBeforeRequest(payload, "model");
  if (result !== payload) throw new Error("fixture stop did not preserve original payload");
  if (prepareCalls.length !== 2) throw new Error("model prepare calls=" + prepareCalls.length + ", want 2");
  if (preFullSideEffects !== 1) throw new Error("rollback comparison did not run before full prepare failure=" + preFullSideEffects);
  if (boundedHostLifecycleCalls !== 3) throw new Error("official host lifecycle was not observed/captured before fail-open="+boundedHostLifecycleCalls);
  if (runtimeConfigBindingCalls !== 1) throw new Error("runtime config binding checks="+runtimeConfigBindingCalls+", want 1");
  const decision = prepareCalls[0];
  const full = prepareCalls[1];
  if (!decision.options.sourceDecisionOnly || decision.userInput !== "" || decision.continuityInfo !== null || decision.languageContext !== null) {
    throw new Error("first call was not a source-decision-only phase");
  }
  if (full.options.sourceDecisionOnly === true || full.userInput !== "actual input") throw new Error("full call did not use Go effective input");
  if (full.continuityInfo !== expectedContinuity) throw new Error("onBeforeRequest did not pass continuityInfo to full prepare");
  if (full.languageContext !== languageContextFixture) throw new Error("onBeforeRequest did not pass languageContext to full prepare");
  if (!!full.options.freshFirstTurnLightMode !== expectedFresh) throw new Error("fresh mode=" + full.options.freshFirstTurnLightMode + ", want " + expectedFresh);
  if (full.options.hostObservations !== decision.options.hostObservations || full.options.bootstrapObservation !== decision.options.bootstrapObservation) {
    throw new Error("decision/full calls did not repeat identical correlated observations");
  }
}
(async function() {
  activePairs = 0; latestBackendTurn = 0; currentContinuity = null;
  await runFixture(true, null);
  activePairs = 2; latestBackendTurn = 2; currentContinuity = continuityFixture;
  await runFixture(false, continuityFixture);
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("model beforeRequest two-phase runtime fixture failed: %v\n%s", err, output)
	}
}

func TestBeforeRequestBuildsObservationOnlySourceEnvelope(t *testing.T) {
	src := readArchiveCenterJS(t)
	fn := extractArchiveCenterJSAsyncFunction(t, src, "onBeforeRequest")
	observationFunction := extractArchiveCenterJSFunction(t, src, "buildPrepareTurnSourceObservations")
	dotClass := extractArchiveCenterJSFunction(t, src, "statusDotClass")
	for _, required := range []string{
		`if (!settings.enabled || !isSaveType(type)) return payload;`,
		`sourceDecisionOnly: true`,
		`const preparedTurnResult = await tryPrepareTurn(orchSessionId, userInput, messages, continuityInfo, type, turnLanguageContext, {`,
		`freshFirstTurnLightMode,`,
		`freshFirstTurnLightModeMeta,`,
		`const prepareSourceObservations = buildPrepareTurnSourceObservations(`,
		`const hostObservations = buildPrepareTurnHostObservations(`,
		`const bootstrapObservation = await observePrepareTurnBootstrap(`,
		`sourceObservation: prepareSourceObservations.sourceObservation`,
		`capabilityObservation: prepareSourceObservations.capabilityObservation`,
		`hostObservations,`,
		`bootstrapObservation,`,
	} {
		if !strings.Contains(fn, required) {
			t.Fatalf("beforeRequest source observation missing %q", required)
		}
	}
	if count := strings.Count(fn, "await tryPrepareTurn("); count != 2 {
		t.Fatalf("model beforeRequest must contain exactly decision-only + full prepare calls, got %d", count)
	}
	for _, forbidden := range []string{"scrubOocDirectivesFromUserInput(userInput)", "detectCurrentTurnOocInfo(payload, messages"} {
		if strings.Contains(fn, forbidden) {
			t.Fatalf("JavaScript changed or classified Go effective input before full prepare: %q", forbidden)
		}
	}
	for _, forbidden := range []string{"source_authority", "stable_identity", "canonical_truth", "lifecycle_acceptance", "persistence"} {
		if strings.Contains(observationFunction, forbidden) {
			t.Fatalf("JavaScript inferred forbidden source policy %q", forbidden)
		}
	}
	for _, required := range []string{
		`contract_version: "message_source_observation.v1"`,
		`contract_version: "host_source_capabilities.v1"`,
		`raw_input_hash: observedRawInputHash`,
		`raw_input_hash: observedRawInputHash ? "observed" : "unavailable"`,
	} {
		if !strings.Contains(observationFunction, required) {
			t.Fatalf("production observation helper missing %q", required)
		}
	}
	for _, required := range []string{
		`const observedSourcePath = !beforeRequestRecoveredForRead`,
		`const ptStatus = _prepareTurnEverContacted`,
		`? String(ptLaneStatus && ptLaneStatus.status || ptResult.status || "ok")`,
		`reason_code: ptLaneStatus && ptLaneStatus.reason_code || null`,
		`original_payload_preserved: ptLaneStatus ? !!ptLaneStatus.original_payload_preserved : null`,
		`detail: ptLaneStatus ? {`,
	} {
		if !strings.Contains(fn, required) {
			t.Fatalf("beforeRequest stable lane projection missing %q", required)
		}
	}
	for _, status := range []string{"eligible", "not_applicable", "empty", "deferred", "degraded", "failed", "incompatible"} {
		if !strings.Contains(dotClass, status) {
			t.Fatalf("stable source lane status %q is not represented by the UI adapter", status)
		}
	}
}

func TestActualInputAndBootstrapProductionObservationRuntime(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required")
		}
	}
	src := readArchiveCenterJS(t)
	hashFn := extractArchiveCenterJSFunction(t, src, "computeOrchestrationDirtyHashOr1c")
	contentFn := extractArchiveCenterJSFunction(t, src, "auxiliaryMessageContentText")
	payloadMessageFn := extractArchiveCenterJSFunction(t, src, "getPayloadMessageRoleAndText")
	messageFn := extractArchiveCenterJSFunction(t, src, "buildPrepareMessageObservation")
	hostFn := extractArchiveCenterJSFunction(t, src, "buildPrepareTurnHostObservations")
	bootstrapFn := extractArchiveCenterJSAsyncFunction(t, src, "observePrepareTurnBootstrap")
	script := `
"use strict";
const R = {
  getCharacter: async () => ({
    firstMessage: "first greeting",
    alternateGreetings: ["alternate zero", "alternate one"],
  }),
};
async function resolveCurrentActiveChatObject() { return { chat: { fmIndex: 1, data: {} } }; }
` + hashFn + "\n" + contentFn + "\n" + payloadMessageFn + "\n" + messageFn + "\n" + hostFn + "\n" + bootstrapFn + `
(async function() {
  const actual = '{"supervisor":"return JSON only","npc":"list"}';
  const active = [
    { role: "assistant", content: "start A", risuMessageIndex: 0, raw: { role: "char", data: "start A", chatId: "risu-start-0", time: 101, generationInfo: { generationId: "gen-start-0" } } },
    { role: "assistant", content: "start A", risuMessageIndex: 1, raw: { role: "char", data: "start A", chatId: "risu-start-1", time: 102 } },
    { role: "user", content: actual, risuMessageIndex: 2, raw: { role: "user", data: actual, chatId: "risu-user-2", time: 103, id: "unsupported-id", revision: "unsupported-revision" } },
  ];
  const payload = [
    { role: "system", content: "preset" },
	{ role: "user", content: [{type:"text",text:actual},{type:"image_url",image_url:{url:"data:image/png;base64,AAAA"}}] },
    { role: "system", content: "host suffix" },
  ];
  const host = buildPrepareTurnHostObservations(
    "session-d", "request-d", "model",
    { text: actual, capturedAt: 123456 }, active, payload,
    '["messages"]', true, "chat-d"
  );
  if (!host.input_hook || host.input_hook.raw_content !== actual || host.input_hook.role !== "user") {
    throw new Error("input-hook observation was classified or changed");
  }
  if (host.input_hook.source_kind !== "input_hook_intermediate" || host.input_hook.observation_stage !== "input_script_handler" || host.input_hook.lifecycle_kind !== "user_input_hook_intermediate") {
    throw new Error("input hook was presented as a raw/final host observation");
  }
  if (host.payload_observation_stage !== "before_request_replacer" || host.final_payload_observation !== "not_exposed") {
    throw new Error("beforeRequest payload was presented as the final sent payload");
  }
  if (host.active_chat[2].message_id !== "risu-user-2" || host.active_chat[2].message_time !== 103 || host.active_chat[2].observed_revision !== null) {
    throw new Error("official Risu message identity was not preserved exactly");
  }
  if (host.active_chat[0].generation_id !== "gen-start-0" || host.active_chat[0].message_id !== "risu-start-0") {
    throw new Error("official Risu generation identity was not observed");
  }
  if (host.input_hook.content_hash !== computeOrchestrationDirtyHashOr1c(actual)) {
    throw new Error("input-hook hash mismatch");
  }
  if (host.payload.length !== 3 || host.payload[2].role !== "system") {
    throw new Error("payload lifecycle observations were collapsed");
  }
	if (host.payload[1].raw_content !== actual || host.payload[1].content_hash !== computeOrchestrationDirtyHashOr1c(actual)) {
	  throw new Error("text-bearing prepare observation diverged from final payload parity text");
	}
  const bootstrap = await observePrepareTurnBootstrap("session-d", "request-d", active, "chat-d");
  if (bootstrap.leading_messages.length !== 2) throw new Error("multiple starts were not preserved");
  if (bootstrap.leading_messages[0].message_index !== 0 || bootstrap.leading_messages[1].message_index !== 1) {
    throw new Error("bootstrap order/boundaries changed");
  }
  if (bootstrap.leading_messages[0].raw_content !== "start A" || bootstrap.leading_messages[1].raw_content !== "start A") {
    throw new Error("duplicate bootstrap text was deduplicated");
  }
  if (!bootstrap.selection_exposed || bootstrap.selected_greeting_index !== 1) {
    throw new Error("selected greeting observation missing");
  }
  if (bootstrap.first_greeting !== "first greeting" || bootstrap.alternate_greetings[1] !== "alternate one") {
    throw new Error("host greeting fields were selected or normalized in JavaScript");
  }
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-e", script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("actual-input/bootstrap production observation runtime failed: %v\n%s", err, output)
	}
}

func TestActiveChatSourceObservationUsesOfficialRisuMessageShape(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required")
		}
	}
	src := readArchiveCenterJS(t)
	fn := extractArchiveCenterJSAsyncFunction(t, src, "getCurrentActiveChatSourceObservationMessages")
	script := fn + `
const active = {message: [
  {role: "user", data: "official user", chatId: "u-1", time: 10},
  {role: "char", data: "official character", chatId: "c-1", time: 11, generationInfo: {generationId: "g-1"}},
  {role: "assistant", content: "generic compatibility shape"},
  {role: "user", data: 42},
]};
async function resolveCurrentActiveChatObject() { return {chat: active}; }
function extractActiveChatMessageList(chat) { return chat.message; }
(async function() {
  const observed = await getCurrentActiveChatSourceObservationMessages("session-risu");
  if (observed.length !== 2) throw new Error("non-official message shape was inferred");
  if (observed[0].role !== "user" || observed[0].content !== "official user" || observed[0].raw.chatId !== "u-1") {
    throw new Error("official user message was not preserved");
  }
  if (observed[1].role !== "assistant" || observed[1].content !== "official character" || observed[1].raw.generationInfo.generationId !== "g-1") {
    throw new Error("official character message was not preserved");
  }
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("official Risu active-chat observation fixture failed: %v\n%s", err, output)
	}
}

func TestRollbackRequestPreservesZeroVisibleTurnsAfterCopiedSessionDelete(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for Archive Center rollback request runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	fn := extractArchiveCenterJSAsyncFunction(t, src, "requestBackendRollbackDecision")
	script := fn + `
let capturedBody = null;
function getRequestTimeoutSettingMs() { return 1000; }
async function fetchBackendLatestTurnIndexForSession() { return 9; }
async function safeCall(call) { return call(); }
function serializeSessionRoutingBaselineForBackend() {
  return {backend_turn_at_route: 8, local_pairs_at_route: 0, reason: "timeline_copy"};
}
async function bridgeFetch(path, options) {
  capturedBody = options.body;
  return {status: "ok", contract_version: "rollback.decision.v1", allowed: true, from_turn: 9, decision_token: "fixture"};
}
(async function() {
  await requestBackendRollbackDecision("char_1_cid_target", 9, "assistant_deleted_output_removed", {
    visibleCompletedTurnCount: 0,
    activeCompletedTurnCount: 8,
    backendLatestTurnIndex: 9
  }, "auto");
  if (!capturedBody) throw new Error("rollback request body was not captured");
  if (capturedBody.visible_completed_turns !== 0) {
    throw new Error("visible_completed_turns=" + capturedBody.visible_completed_turns + ", want 0");
  }
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-e", script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("rollback request runtime fixture failed: %v\n%s", err, output)
	}
}

func TestCopiedSessionFinalOutputRecoveryUsesCurrentChatIndex(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for Archive Center copied-session recovery runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	fn := extractArchiveCenterJSAsyncFunction(t, src, "recoverAssistantContentFromActiveChat")
	script := fn + `
async function resolveCurrentActiveChatObject() { return {chat: {message: []}, source: "R.getChatFromIndex"}; }
function extractActiveChatComparableMessages() {
  return [{role: "user", content: "new copied-session input"}, {role: "assistant", content: "final copied-session output"}];
}
function normalizeMainTurnCompareText(value) { return String(value || "").trim(); }
function mainTurnTextMatchesOriginal(left, right) { return String(left || "").trim() === String(right || "").trim(); }
function buildCompletedTurnPairsFromActiveChatMessages() {
  return [{userContent: "new copied-session input", assistantContent: "final copied-session output"}];
}
function normalizeAssistantPersistenceCandidate(value) { return String(value || "").trim(); }
function isAssistantPrefillSeedText() { return false; }
function debugLog() {}
(async function() {
  const recovered = await recoverAssistantContentFromActiveChat("char_1_cid_target", null, "new copied-session input");
  if (recovered !== "final copied-session output") {
    throw new Error("recovered=" + JSON.stringify(recovered));
  }
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-e", script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("copied-session final output recovery fixture failed: %v\n%s", err, output)
	}
}

func TestHistoryTrimGuardRequiresObservedSlashCommand(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for Archive Center history trim runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functions := []string{
		extractArchiveCenterJSFunction(t, src, "recordRisuHistoryTrimGuard"),
		extractArchiveCenterJSFunction(t, src, "getRecentRisuHistoryTrimGuard"),
		extractArchiveCenterJSFunction(t, src, "buildSnapshotHistoryTrimGuardOr1f"),
	}
	script := strings.Join(functions, "\n\n") + `
const _rollbackHistoryTrimGuardBySession = new Map();
const ROLLBACK_HISTORY_TRIM_GUARD_MS = 5 * 60 * 1000;
function compactSnapshotMessages(messages) { return Array.isArray(messages) ? messages : []; }
function computeTailHash() { return "fixture-tail"; }
function assertTrue(value, label) { if (!value) throw new Error(label); }

recordRisuHistoryTrimGuard("s", "active_chat_suffix_window_trim", {});
assertTrue(getRecentRisuHistoryTrimGuard("s") === null, "inferred trim must not become command intent");
const withoutCommand = buildSnapshotHistoryTrimGuardOr1f(
  "s",
  {messagesPreview:[{role:"user",content:"old"},{role:"assistant",content:"kept"}], turnIndex:1, tailHash:"old"},
  [{role:"assistant",content:"kept"}],
  {commonPrefixLen:0, commonSuffixLen:1, removedMsgCount:1, insertedMsgCount:0},
  {previousAssistantCount:1, currentAssistantCount:1}
);
assertTrue(withoutCommand === null, "ordinary deletion must not be protected as slash trim");

recordRisuHistoryTrimGuard("s", "risu_slash_command", {commandPreview:"/cut"});
assertTrue(getRecentRisuHistoryTrimGuard("s") !== null, "observed slash command must enable trim protection");
const withCommand = buildSnapshotHistoryTrimGuardOr1f(
  "s",
  {messagesPreview:[{role:"user",content:"old"},{role:"assistant",content:"kept"}], turnIndex:1, tailHash:"old"},
  [{role:"assistant",content:"kept"}],
  {commonPrefixLen:0, commonSuffixLen:1, removedMsgCount:1, insertedMsgCount:0},
  {previousAssistantCount:1, currentAssistantCount:1}
);
assertTrue(!!withCommand && withCommand.explicitCommandObserved === true, "slash trim protection missing");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Archive Center history trim runtime fixture failed: %v\n%s", err, out)
	}
}

func TestRollbackComparablePrefersActiveChatWhenCurrentInputIsConfirmed(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for Archive Center rollback source runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functionBody := extractArchiveCenterJSAsyncFunction(t, src, "resolveRollbackComparableMessages")
	script := functionBody + `
let fixtureActiveMessages = [];
let fixturePreviousSnapshot = null;
async function getCurrentActiveChatRollbackMessages() { return fixtureActiveMessages; }
function compactSnapshotMessages(messages) { return Array.isArray(messages) ? messages : []; }
function getSessionSnapshot() { return fixturePreviousSnapshot; }
function getLastPayloadUserText(messages) {
  for (let i = (messages || []).length - 1; i >= 0; i--) {
    if (messages[i] && messages[i].role === "user") return String(messages[i].content || "");
  }
  return "";
}

function mainTurnTextMatchesOriginal(left, right) {
  const a = String(left || "").trim();
  const b = String(right || "").trim();
  return a === b || (!!a && !!b && (a.startsWith(b) || b.startsWith(a)));
}
function assertEqual(actual, expected, label) {
  if (actual !== expected) throw new Error(label + ": got=" + JSON.stringify(actual) + " want=" + JSON.stringify(expected));
}

(async function() {
  fixturePreviousSnapshot = {msgCount: 8};
  fixtureActiveMessages = [
    {role:"user",content:"u1"}, {role:"assistant",content:"a1"},
    {role:"user",content:"u2"}, {role:"assistant",content:"a2"},
    {role:"user",content:"u3"}, {role:"assistant",content:"a3"},
    {role:"user",content:"new player input"}
  ];
  const longPayload = Array.from({length:20}, (_, i) => ({role:i % 2 ? "assistant" : "user", content:"prompt-" + i}));
  longPayload.push({role:"user",content:"rewritten payload input"});
  let result = await resolveRollbackComparableMessages("s", longPayload, "new player input");
  assertEqual(result.source, "active_chat_current_input_confirmed", "deleted active history with current input must remain authoritative");
  assertEqual(result.messages.length, 7, "active chat deletion shape lost");

  fixtureActiveMessages[fixtureActiveMessages.length - 1] = {role:"user",content:"stale previous input"};
  result = await resolveRollbackComparableMessages("s", longPayload, "new player input");
  assertEqual(result.source, "payload_newer_than_stale_active_chat", "unconfirmed stale active chat must keep payload fallback");
})().catch(function(err) {
  console.error(err && err.stack || err);
  process.exit(1);
});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Archive Center rollback source JS runtime fixture failed: %v\n%s", err, out)
	}
}

func TestRisuRequestObservationDoesNotInferOOCAndSurvivesQueueRuntime(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for Risu request observation runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	for _, forbidden := range []string{
		"function isOocUserMessage(",
		"function isOocDirectiveLine(",
		"function isOocHeaderOnlyLine(",
		"function scrubOocDirectivesFromUserInput(",
		"function isFullyOocUserInput(",
		"function shouldSkipTurnPersistenceForOoc(",
		"function showTurnWorkflowHUDOOCRecognition(",
		"_pendingPersistenceSkipBySession",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("Archive Center.js retains JavaScript OOC inference or skip authority %q", forbidden)
		}
	}
	prepareFn := extractArchiveCenterJSAsyncFunction(t, src, "tryPrepareTurn")
	completeFn := extractArchiveCenterJSAsyncFunction(t, src, "buildCompleteTurnRequestBody")
	for name, fn := range map[string]string{"prepare": prepareFn, "complete": completeFn} {
		if !strings.Contains(fn, "risu_request_observation") {
			t.Fatalf("%s request does not forward risu_request_observation.v1", name)
		}
	}

	observationFn := extractArchiveCenterJSFunction(t, src, "buildRisuRequestObservation")
	skipFn := extractArchiveCenterJSFunction(t, src, "shouldSkipUserInputPersistence")
	queueFn := extractArchiveCenterJSFunction(t, src, "buildCompleteTurnQueuePayload")
	script := observationFn + "\n" + skipFn + "\n" + queueFn + `
function isBoundaryOnlyUserInput() { return false; }
function isRisuHistoryTrimCommandText() { return false; }
function isMetaPromptLikeMessage() { return false; }
function normalizeLanguageContextTrace(value) { return value; }
function buildRisuActiveChatContextMessageObservation(msg) { return msg; }
function assertEqual(actual, expected, label) {
  if (actual !== expected) throw new Error(label + ": got=" + JSON.stringify(actual) + " want=" + JSON.stringify(expected));
}
const quoted = 'A character quoted the literal marker "[OOC]" without making a host request-class claim.';
if (shouldSkipUserInputPersistence(quoted)) throw new Error("quoted OOC marker was classified as an OOC request");
const before = buildRisuRequestObservation("model", "beforeRequest", "user");
assertEqual(before.contract_version, "risu_request_observation.v1", "contract");
assertEqual(before.observation_state, "observed", "lifecycle observation state");
assertEqual(before.request_type, "model", "raw replacer type");
assertEqual(before.request_type_state, "observed", "request type state");
assertEqual(before.lifecycle_stage, "beforeRequest", "before lifecycle");
assertEqual(before.message_role, "user", "observed role");
assertEqual(before.channel, null, "unexposed channel value");
assertEqual(before.channel_state, "not_exposed", "channel state");
assertEqual(before.ooc_class, null, "unexposed OOC value");
assertEqual(before.ooc_class_state, "not_exposed", "OOC state");
const after = buildRisuRequestObservation("otherAx", "afterRequest", "assistant");
const queued = buildCompleteTurnQueuePayload({
  chat_session_id:"session",
  turn_index:4,
  user_input:quoted,
  assistant_content:"assistant response",
  context_messages:[{role:"user",content:quoted}],
  request_type:"model",
  client_meta:{risu_request_observation:after}
});
assertEqual(queued.user_input, quoted, "quoted input preserved");
assertEqual(queued.context_messages[0].content, quoted, "quoted context preserved");
assertEqual(queued.client_meta.risu_request_observation.request_type, "otherAx", "queued raw type");
assertEqual(queued.client_meta.risu_request_observation.lifecycle_stage, "afterRequest", "queued lifecycle");
assertEqual(queued.client_meta.risu_request_observation.message_role, "assistant", "queued role");
assertEqual(queued.client_meta.risu_request_observation.ooc_class_state, "not_exposed", "queued OOC state");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Risu request observation runtime fixture failed: %v\n%s", err, out)
	}
}

func TestFailedQueueTerminalHistoryExpiresAndDoesNotBlockAdmission(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for failed queue history fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functions := strings.Join([]string{
		extractArchiveCenterJSFunction(t, src, "stableFailedQueuePayloadFingerprint"),
		extractArchiveCenterJSFunction(t, src, "makeFailedQueueDedupeKey"),
		extractArchiveCenterJSFunction(t, src, "deserializeFailedQueue"),
		extractArchiveCenterJSAsyncFunction(t, src, "loadFailedQueueFromStorage"),
		extractArchiveCenterJSFunction(t, src, "enqueue"),
	}, "\n")
	script := functions + `
const FAILED_QUEUE_STORAGE_KEY="failed";
const _failedQueue=[];
const settings={failedQueueMaxAgeDays:7,failedQueueMaxSize:2};
const runtimeState={queuePersistence:{}};
let scheduled=0;
function computeOrchestrationDirtyHashOr1c(value) { return "h:"+String(value || ""); }
function debugLog() {}
function warnLog() {}
function scheduleQueueSave() { scheduled++; }
function payload(key,turn) {
  return {chat_session_id:"session-1",turn_index:turn,user_input:"user",assistant_content:key,
    context_messages:[],client_meta:{idempotency_key:key}};
}
const oldAt=new Date(Date.now()-8*24*60*60*1000).toISOString();
const recentAt=new Date().toISOString();
const raw=JSON.stringify({v:1,items:[
  {id:"complete_turn|idempotency|expired",type:"complete_turn",payload:payload("expired",1),attempts:4,
    state:"terminal",terminalCode:"retry_limit_reached",terminalAt:oldAt,addedAt:oldAt},
  {id:"complete_turn|idempotency|active",type:"complete_turn",payload:payload("active",2),attempts:1,
    state:"retryable",addedAt:recentAt}
]});
async function persistentGet(key) {
  if(key!==FAILED_QUEUE_STORAGE_KEY) throw new Error("wrong storage key");
  return raw;
}
(async function() {
  await loadFailedQueueFromStorage();
  if(_failedQueue.length!==1 || _failedQueue[0].payload.client_meta.idempotency_key!=="active") {
    throw new Error("expired terminal incident was restored or active retry was lost");
  }
  if(scheduled!==1) throw new Error("expired durable history was not scheduled for storage cleanup");
  _failedQueue.push({type:"complete_turn",payload:payload("recent-terminal",3),attempts:4,state:"terminal",
    terminalCode:"retry_limit_reached",terminalAt:recentAt,addedAt:recentAt,
    _dedupeKey:"complete_turn|idempotency|recent-terminal"});
  const admitted=enqueue("complete_turn",payload("new-active",4));
  if(!admitted || admitted.status!=="accepted" || !admitted.queued || _failedQueue.length!==2) {
    throw new Error("terminal history still blocked a new retryable admission");
  }
  const keys=_failedQueue.map(item=>item.payload.client_meta.idempotency_key).sort().join(",");
  if(keys!=="active,new-active" || _failedQueue.some(item=>item.state==="terminal")) {
    throw new Error("capacity reclamation removed active work or retained terminal history");
  }
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed queue history fixture failed: %v\n%s", err, out)
	}
}

func TestPendingRecoveryHistoryCleanupPreservesActiveWorkAndIsReachable(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for pending recovery history fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functions := strings.Join([]string{
		extractArchiveCenterJSFunction(t, src, "stableFailedQueuePayloadFingerprint"),
		extractArchiveCenterJSFunction(t, src, "makeFailedQueueDedupeKey"),
		extractArchiveCenterJSFunction(t, src, "serializeCompleteTurnRecoveryPayload"),
		extractArchiveCenterJSFunction(t, src, "serializeChatLogRecoveryPayload"),
		extractArchiveCenterJSFunction(t, src, "serializeFailedQueueItem"),
		extractArchiveCenterJSFunction(t, src, "serializeFailedQueue"),
		extractArchiveCenterJSAsyncFunction(t, src, "saveFailedQueueToStorage"),
		extractArchiveCenterJSFunction(t, src, "pendingFinalConfirmationRecoveryKey"),
		extractArchiveCenterJSFunction(t, src, "serializePendingFinalConfirmationRecovery"),
		extractArchiveCenterJSAsyncFunction(t, src, "savePendingFinalConfirmationRecoveryToStorage"),
		extractArchiveCenterJSAsyncFunction(t, src, "persistPendingFinalConfirmationRecovery"),
		extractArchiveCenterJSAsyncFunction(t, src, "clearFailedQueue"),
		extractArchiveCenterJSFunction(t, src, "renderDashboardViewModel"),
	}, "\n")
	script := functions + `
const FAILED_QUEUE_STORAGE_KEY="failed";
const PENDING_FINAL_CONFIRMATION_STORAGE_KEY="pending";
const STARTUP_MESSAGE_TURN_INDEX=0;
const _failedQueue=[];
const _pendingFinalConfirmationRecoveryEntries=new Map();
const settings={failedQueueMaxSize:2,failedQueueMaxAgeDays:7};
const runtimeState={queuePersistence:{}};
const stored={};
function buildCompleteTurnQueuePayload(value) { return JSON.parse(JSON.stringify(value)); }
function serializeAcceptedFinalRecoveryPayload(value) { return value; }
function computeOrchestrationDirtyHashOr1c(value) { return "h:"+String(value || ""); }
function debugLog() {}
function warnLog() {}
function escapeAttr(value) { return String(value || ""); }
function t(key) { return key==="dash.queue.clearBtn" ? "Clear Past Errors" : key; }
function dashboardSimpleText(value) { return String(value || ""); }
function formatAuxiliaryPlacementTrace(value) { return String(value || ""); }
function formatDashboardTimestampLocal() { return ""; }
function statusDotClass() { return "mo-dot-notice"; }
function dashboardViewModelLabel(value) { return String(value || ""); }
function runtimeStatusLabel(value) { return String(value || ""); }
async function persistentSet(key,value) { stored[key]=value; }
function payload(key,turn) {
  return {chat_session_id:"session-1",turn_index:turn,user_input:"user",assistant_content:key,
    context_messages:[],client_meta:{idempotency_key:key}};
}
const now=new Date().toISOString();
_failedQueue.push(
  {type:"complete_turn",payload:payload("failed-terminal",1),attempts:4,state:"terminal",terminalAt:now,addedAt:now,
    _dedupeKey:"complete_turn|idempotency|failed-terminal"},
  {type:"complete_turn",payload:payload("failed-active",2),attempts:1,state:"retryable",addedAt:now,
    _dedupeKey:"complete_turn|idempotency|failed-active"}
);
_pendingFinalConfirmationRecoveryEntries.set("complete|pending-terminal",{
  key:"complete|pending-terminal",payload:payload("pending-terminal",3),state:"terminal",terminalAt:now,addedAt:now
});
_pendingFinalConfirmationRecoveryEntries.set("complete|pending-active",{
  key:"complete|pending-active",payload:payload("pending-active",4),state:"pending",addedAt:now
});
(async function() {
  if(!await persistPendingFinalConfirmationRecovery(payload("pending-new",5),"pending_confirmation","","")) {
    throw new Error("terminal pending history still blocked a new active recovery");
  }
  if(_pendingFinalConfirmationRecoveryEntries.has("complete|pending-terminal") ||
     !_pendingFinalConfirmationRecoveryEntries.has("complete|pending-active") ||
     !_pendingFinalConfirmationRecoveryEntries.has("complete|pending-new")) {
    throw new Error("pending recovery capacity reclamation removed active work");
  }
  _pendingFinalConfirmationRecoveryEntries.set("complete|pending-clear",{
    key:"complete|pending-clear",payload:payload("pending-clear",6),state:"terminal",terminalAt:now,addedAt:now
  });
  await clearFailedQueue();
  if(_failedQueue.length!==1 || _failedQueue[0].payload.client_meta.idempotency_key!=="failed-active") {
    throw new Error("manual history cleanup removed retryable failed-queue work");
  }
  if(_pendingFinalConfirmationRecoveryEntries.size!==2 ||
     !_pendingFinalConfirmationRecoveryEntries.has("complete|pending-active") ||
     !_pendingFinalConfirmationRecoveryEntries.has("complete|pending-new")) {
    throw new Error("manual history cleanup removed active pending recoveries");
  }
  const failedStored=JSON.parse(stored[FAILED_QUEUE_STORAGE_KEY]);
  const pendingStored=JSON.parse(stored[PENDING_FINAL_CONFIRMATION_STORAGE_KEY]);
  if(failedStored.items.length!==1 || pendingStored.items.length!==2) {
    throw new Error("cleaned history was not persisted to both pluginStorage records");
  }
  const html=renderDashboardViewModel({status:"ok",cards:[{id:"historical_queue",title:"Historical Queue",rows:[]}]},{});
  if(!html.includes('id="mo-queue-clear-btn"') || !html.includes("Clear Past Errors")) {
    throw new Error("historical queue cleanup control is not rendered on the normal dashboard card");
  }
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pending recovery history fixture failed: %v\n%s", err, out)
	}
}

func TestExpiredPendingRecoveryTerminalHistoryIsRemovedDurably(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for pending recovery expiry fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functions := strings.Join([]string{
		extractArchiveCenterJSFunction(t, src, "serializeCompleteTurnRecoveryPayload"),
		extractArchiveCenterJSFunction(t, src, "pendingFinalConfirmationRecoveryKey"),
		extractArchiveCenterJSFunction(t, src, "serializePendingFinalConfirmationRecovery"),
		extractArchiveCenterJSAsyncFunction(t, src, "savePendingFinalConfirmationRecoveryToStorage"),
		extractArchiveCenterJSFunction(t, src, "removeFailedCompleteTurnByIdempotencyKey"),
		extractArchiveCenterJSAsyncFunction(t, src, "loadPendingFinalConfirmationRecoveryFromStorage"),
	}, "\n")
	script := functions + `
const PENDING_FINAL_CONFIRMATION_STORAGE_KEY="pending";
const _pendingFinalConfirmationRecoveryEntries=new Map();
const _failedQueue=[];
const settings={failedQueueMaxAgeDays:7,failedQueueMaxSize:50};
let stored="";
let queued=0;
function buildCompleteTurnQueuePayload(value) { return JSON.parse(JSON.stringify(value)); }
function warnLog() {}
function updateRuntimeState() {}
async function flushQueueSave() {}
async function queuePendingCompleteTurnPayload() { queued++; return true; }
async function persistentGet(key) {
  if(key!==PENDING_FINAL_CONFIRMATION_STORAGE_KEY) throw new Error("wrong storage key");
  return stored;
}
async function persistentSet(key,value) {
  if(key!==PENDING_FINAL_CONFIRMATION_STORAGE_KEY) throw new Error("wrong storage key");
  stored=value;
}
const oldAt=new Date(Date.now()-8*24*60*60*1000).toISOString();
const payload={chat_session_id:"session-1",turn_index:1,user_input:"user",assistant_content:"assistant",
  context_messages:[],client_meta:{idempotency_key:"expired-pending"}};
stored=JSON.stringify({v:1,transition_intents:[],items:[{
  key:"complete|expired-pending",payload,state:"terminal",terminalCode:"retry_limit_reached",
  terminalAt:oldAt,addedAt:oldAt
}]});
(async function() {
  const restored=await loadPendingFinalConfirmationRecoveryFromStorage();
  if(restored!==0 || queued!==0 || _pendingFinalConfirmationRecoveryEntries.size!==0) {
    throw new Error("expired terminal pending recovery was restored or retried");
  }
  const durable=JSON.parse(stored);
  if(!Array.isArray(durable.items) || durable.items.length!==0) {
    throw new Error("expired terminal pending recovery remained in pluginStorage");
  }
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pending recovery expiry fixture failed: %v\n%s", err, out)
	}
}

func TestRisuMessageIndexesDriveLogicalTurnPairs(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for Risu message index runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functionBody := extractArchiveCenterJSFunction(t, src, "buildCompletedTurnPairsFromActiveChatMessages")
	script := functionBody + `
const AUTO_CONTINUE_USER_INPUT_MARKER = "[continue]";
function extractComparableMessageRoleAndContent(msg) { return msg; }
function selectBestAssistantCandidateRecord(items) { return items[items.length - 1] || null; }
function normalizeAssistantPersistenceCandidate(text) { return String(text || "").trim(); }
function shouldSkipUserInputPersistence(text) { return text === "skip indexed turn"; }
function shouldSkipTurnPersistenceForOoc() { return false; }
function computeOrchestrationDirtyHashOr1c(text) { return String(text).length; }
function getActiveChatMessageStreamingState() { return "done"; }
function debugLog() {}
function assertEqual(actual, expected, label) {
  if (actual !== expected) throw new Error(label + ": got=" + JSON.stringify(actual) + " want=" + JSON.stringify(expected));
}
const messages = [];
for (let turn = 1; turn <= 7; turn++) {
  const userIndex = (turn - 1) * 2;
  messages.push(
    {role:"user",content:turn === 2 ? "skip indexed turn" : "u"+turn,risuMessageIndex:userIndex},
    {role:"assistant",content:"a"+turn,risuMessageIndex:userIndex+1}
  );
}
let pairs = buildCompletedTurnPairsFromActiveChatMessages(messages);
assertEqual(JSON.stringify(pairs.map(p => p.risuUserMessageIndex)), JSON.stringify([0,4,6,8,10,12]), "adapter preserves raw Risu indexes after a filtered pair");
assertEqual(JSON.stringify(pairs.map(p => p.observedPairOrdinal)), JSON.stringify([1,2,3,4,5,6]), "adapter reports observation order without deriving logical turns");
assertEqual(pairs.some(p => Object.prototype.hasOwnProperty.call(p, "turnIndex")), false, "adapter must not calculate authoritative turn indexes");
pairs = buildCompletedTurnPairsFromActiveChatMessages(messages.slice(0, -2));
assertEqual(pairs[pairs.length - 1].risuUserMessageIndex, 10, "tail deletion exposes the preceding raw Risu index");
const fullContextMessages = [];
for (let turn = 1; turn <= 13; turn++) {
  const longPrefix = turn === 1 ? "x".repeat(2100) : "";
  fullContextMessages.push(
    {role:"user",content:"context-user-"+turn+longPrefix,risuMessageIndex:(turn-1)*2},
    {role:"assistant",content:"context-assistant-"+turn,risuMessageIndex:(turn-1)*2+1}
  );
}
pairs = buildCompletedTurnPairsFromActiveChatMessages(fullContextMessages);
assertEqual(pairs[12].contextMessages.length, 24, "critic context must include every preceding host message");
assertEqual(pairs[12].contextMessages[0].content, fullContextMessages[0].content, "critic context must preserve full exact content");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Risu message index JS runtime fixture failed: %v\n%s", err, out)
	}
}

func TestRisuIndexLatestTurnFeedsRoutingAndRollbackObservation(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for Risu index routing runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	script := extractArchiveCenterJSAsyncFunction(t, src, "requestBackendSessionRoutingTurnResolution") + `
let captured = null;
async function bridgeFetch(path, options) {
  if (path !== "/session-routing/turn-resolution") throw new Error("unexpected path: "+path);
  captured = options.body;
  return {status:"ok",contract_version:"session-routing.turn-resolution.v1",resolution:"normal",turn_index:7,completed_turns:7,local_turn_index:7,local_turn_source:"risu_user_message_index",baseline_applied:false,resolved_observations:[]};
}
function getRequestTimeoutSettingMs() { return 1000; }
function serializeSessionRoutingBaselineForBackend() { return null; }
function assertEqual(actual, expected, label) {
  if (actual !== expected) throw new Error(label+": got="+JSON.stringify(actual)+" want="+JSON.stringify(expected));
}
(async function() {
  const result = await requestBackendSessionRoutingTurnResolution("session", "pair", {risuUserMessageIndex:12,observedPairOrdinal:6});
  assertEqual(captured.risu_user_message_index, 12, "raw Risu user index is forwarded");
  assertEqual(captured.observed_pair_ordinal, 6, "observed pair ordinal is forwarded");
  assertEqual(Object.prototype.hasOwnProperty.call(captured, "local_turn_index"), false, "adapter must not send a calculated local turn");
  assertEqual(Object.prototype.hasOwnProperty.call(captured, "visible_completed_turns"), false, "adapter must not send a calculated visible turn count");
  assertEqual(result.localTurnIndex, 7, "backend local turn is applied");
})().catch(err => { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Risu index routing runtime fixture failed: %v\n%s", err, out)
	}
}

func TestSessionNormalizeUsesCanonicalRisuChatPairsWithoutLiveFilters(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for canonical Risu chat runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	script := extractArchiveCenterJSFunction(t, src, "computeOrchestrationDirtyHashOr1c") + "\n" +
		extractArchiveCenterJSFunction(t, src, "buildCompletedTurnPairsFromActiveChatMessages") + "\n" +
		extractArchiveCenterJSFunction(t, src, "buildSessionNormalizeCompletedTurnPairs") + `
const AUTO_CONTINUE_USER_INPUT_MARKER = "[continue]";
function extractActiveChatComparableMessages(chat) {
  return chat.message.map(function(item, index) {
    const role = item.role === "user" ? "user" : item.role === "char" ? "assistant" : item.role;
    return {role,content:String(item.data || ""),risuMessageIndex:index,raw:item};
  });
}
function extractComparableMessageRoleAndContent(msg) { return msg; }
function selectBestAssistantCandidateRecord(items) { return items[items.length - 1] || null; }
function normalizeAssistantPersistenceCandidate(text) { return String(text || "").trim(); }
function shouldSkipUserInputPersistence() { return false; }
function shouldSkipTurnPersistenceForOoc() { return false; }
function getActiveChatMessageStreamingState() { return "done"; }
function debugLog() {}
function assertEqual(actual, expected, label) {
  if (actual !== expected) throw new Error(label + ": got=" + JSON.stringify(actual) + " want=" + JSON.stringify(expected));
}
const chat = {message:[
  {role:"user",data:"first user"}, {role:"char",data:"first final output"},
  {role:"user",data:"# SYSTEM is visible user text here"}, {role:"char",data:"second final output"},
  {role:"user",data:"third user"}, {role:"char",data:"third draft"}, {role:"char",data:"third final output"},
  {role:"user",data:"fourth user"}, {role:"char",data:"fourth final output"},
]};
let result = buildSessionNormalizeCompletedTurnPairs(chat);
assertEqual(result.available, true, "canonical chat availability");
assertEqual(result.pairs.length, 4, "four visible turns must remain four turns");
assertEqual(result.pairs[1].userContent, "# SYSTEM is visible user text here", "content filters must not erase canonical raw turns");
assertEqual(result.pairs[2].assistantContent, "third final output", "latest visible assistant content wins within one turn");
assertEqual(JSON.stringify(result.pairs.map(p => p.risuUserMessageIndex)), JSON.stringify([0,2,4,7]), "canonical raw Risu indexes");
assertEqual(JSON.stringify(result.pairs.map(p => p.observedPairOrdinal)), JSON.stringify([1,2,3,4]), "canonical observation order");
assertEqual(result.pairs.some(p => Object.prototype.hasOwnProperty.call(p, "turnIndex")), false, "session normalize adapter does not calculate turns");
result = buildSessionNormalizeCompletedTurnPairs({message:chat.message.concat([{role:"user",data:"unfinished fifth user"}])});
assertEqual(result.pairs.length, 4, "unfinished trailing user must not become a completed turn");
result = buildSessionNormalizeCompletedTurnPairs({messages:chat.message});
assertEqual(result.available, false, "noncanonical fallback must stay explicit");
const longMessages = [];
for (let turn = 1; turn <= 120; turn++) {
  longMessages.push({role:"user",data:"user "+turn}, {role:"char",data:"assistant "+turn});
}
result = buildSessionNormalizeCompletedTurnPairs({message:longMessages});
assertEqual(result.pairs.length, 120, "long canonical chat must not collapse to its recent tail");
assertEqual(result.pairs[0].userContent, "user 1", "long chat first turn");
assertEqual(result.pairs[119].assistantContent, "assistant 120", "long chat last turn");
assertEqual(result.pairs[119].contextMessages.length, 238, "long chat critic context must not collapse to a recent fixed window");
assertEqual(result.pairs[119].contextMessages[0].content, "user 1", "long chat critic context must retain the earliest exact message");
const indexedGapChat = {message:[
  {role:"user",data:"gap user one"}, {role:"char",data:"gap assistant one"},
  {role:"system",data:"host metadata"}, {role:"system",data:"host metadata 2"},
  {role:"user",data:"gap user three"}, {role:"char",data:"gap assistant three"},
]};
result = buildSessionNormalizeCompletedTurnPairs(indexedGapChat);
assertEqual(JSON.stringify(result.pairs.map(p => p.risuUserMessageIndex)), JSON.stringify([0,4]), "session normalize preserves raw Risu index gaps");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("canonical Risu session-normalize JS fixture failed: %v\n%s", err, out)
	}
}

func TestSessionNormalizeRepairEntriesPreserveExistingTurnZeroAndRepairMissingPairs(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for session-normalize repair runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	script := extractArchiveCenterJSFunction(t, src, "sanitizeChatLogRepairEntry") + "\n" +
		extractArchiveCenterJSFunction(t, src, "buildSessionNormalizeRepairEntriesFromDryRunPlan") + `
function assertEqual(actual, expected, label) {
  if (actual !== expected) throw new Error(label + ": got=" + JSON.stringify(actual) + " want=" + JSON.stringify(expected));
}
const plan = {
  dbRows: [],
  rawMissingTurns: [1,2,3,4],
  pairs: [1,2,3,4].map(turn => ({turnIndex:turn,userContent:"user "+turn,assistantContent:"assistant "+turn})),
};
let entries = buildSessionNormalizeRepairEntriesFromDryRunPlan(plan);
assertEqual(JSON.stringify(entries.map(entry => entry.turn_index)), JSON.stringify([1,2,3,4]), "bootstrap must not be fabricated as canonical turn zero");
entries = buildSessionNormalizeRepairEntriesFromDryRunPlan({...plan, dbRows:[{turn_index:0,role:"assistant",content:"already stored"}]});
assertEqual(JSON.stringify(entries.map(entry => entry.turn_index)), JSON.stringify([1,2,3,4]), "existing turn zero must remain untouched");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("session-normalize repair JS fixture failed: %v\n%s", err, out)
	}
}

func TestRisuLogicalTurnReservationUsesImportedSessionBaseline(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for Risu imported baseline runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functionBody := extractArchiveCenterJSAsyncFunction(t, src, "reserveAfterRequestPersistenceTurnIndex")
	script := functionBody + `
let exactTurn = 0;
let fixture = null;
const lastOrchResult = null;
async function safeCall(fn, fallback) { try { return await fn(); } catch { return fallback; } }
async function fetchBackendLatestTurnIndexForSession() { return fixture.backendLatest; }
function setTurnCounterAtLeast() {}
function peekNextTurnIndex() { return fixture.backendLatest + 1; }
async function findActiveChatCompletedTurnPairForContent() {
  if (fixture.staleActivePair) return {observedPairOrdinal:8,pairCount:8,userContent:fixture.user,assistantContent:"old inherited assistant",risuUserMessageIndex:14,risuAssistantMessageIndex:15,source:"stale_same_user"};
  if (fixture.pairMissing) return null;
  return {observedPairOrdinal:fixture.ordinal || 1,pairCount:fixture.ordinal || 1,userContent:fixture.user,assistantContent:fixture.assistant,risuUserMessageIndex:fixture.userIndex || 0,risuAssistantMessageIndex:(fixture.userIndex || 0)+1,source:"active_chat_user_assistant_pair"};
}

function normalizeTurnPairCompareText(text) { return String(text || "").trim(); }
function normalizeAssistantPersistenceCandidate(text) { return String(text || "").trim(); }
async function findActiveChatCompletedTurnPairForUserContent() { return null; }
async function findLatestActiveChatCompletedTurnPair() { return null; }
async function requestBackendSessionRoutingTurnResolution(_sid, mode, observation) {
  if (mode !== "pair" || observation.risuUserMessageIndex !== (fixture.userIndex || 0) || observation.observedPairOrdinal !== (fixture.ordinal || 1)) {
    throw new Error("unexpected routing request: "+JSON.stringify(observation));
  }
  return fixture.routing || {status:"backend_unavailable",turnIndex:0,baseline:null};
}

function setTurnCounterExact(_sid, turn) { exactTurn = turn; }
function nextTurnIndex() { return 99; }
function debugLog() {}
(async function() {
  for (const backendLatest of [1, 5, 13, 34]) {
    fixture = {backendLatest,user:"u"+backendLatest,assistant:"a"+backendLatest};
    exactTurn = 0;
    const turn = await reserveAfterRequestPersistenceTurnIndex("s", fixture.user, fixture.assistant);
    const expected = backendLatest + 1;
    if (turn !== expected || exactTurn !== expected) {
      throw new Error("unavailable routing baseline must append after backend tail: backend="+backendLatest+" got="+turn);
    }
  }
  for (const status of ["skip_pre_route_visible_pair", "worldline_ownership_unresolved"]) {
    fixture = {
      backendLatest:9,
      user:"inherited user "+status,
      assistant:"inherited assistant "+status,
      routing:{status,turnIndex:status === "skip_pre_route_visible_pair" ? 1 : 0,localTurnIndex:1,baseline:null},
    };
    exactTurn = 0;
    const rejectedTurn = await reserveAfterRequestPersistenceTurnIndex("s", fixture.user, fixture.assistant);
    if (rejectedTurn !== 0 || exactTurn !== 0) {
      throw new Error("backend ownership rejection must not be replaced with the next turn: status="+status+" got="+rejectedTurn);
    }
  }
  fixture = {
    backendLatest:9,
    user:"official pre-commit user",
    assistant:"official pre-commit assistant",
    staleActivePair:true,
    userIndex:17,
    ordinal:9,
    routing:{status:"normal",turnIndex:9,localTurnIndex:9,baseline:null},
  };
  exactTurn = 0;
  const preCommitTurn = await reserveAfterRequestPersistenceTurnIndex("s", fixture.user, fixture.assistant, {
    accepted:true,
    user_message_index:17,
    user_observed_pair_ordinal:9,
  });
  if (preCommitTurn !== 9 || exactTurn !== 9) {
    throw new Error("official afterRequest coordinate must route before active-pair commit: got="+preCommitTurn);
  }
  fixture = {
    backendLatest:3,
    user:"rerolled user",
    assistant:"new assistant",
    userIndex:4,
    ordinal:3,
    routing:{status:"normal",turnIndex:3,localTurnIndex:3,baseline:null},
    existing:[{role:"user",content:"rerolled user"},{role:"assistant",content:"old assistant"}],
  };
  exactTurn = 0;
  const rerolledTurn = await reserveAfterRequestPersistenceTurnIndex("s", fixture.user, fixture.assistant);
  if (rerolledTurn !== 3 || exactTurn !== 3) throw new Error("reroll must reuse backend turn 3");
})().catch(function(err) {
  console.error(err && err.stack || err);
  process.exit(1);
});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Risu imported baseline JS runtime fixture failed: %v\n%s", err, out)
	}
}

func TestAfterRequestStopsBeforeCompleteTurnWhenBackendRejectsTurnOwnership(t *testing.T) {
	src := readArchiveCenterJS(t)
	reserve := strings.Index(src, `turnIdx = await reserveAfterRequestPersistenceTurnIndex`)
	stop := strings.Index(src, `const routingSkipReason = "session_routing_turn_ownership_not_admitted"`)
	complete := strings.Index(src, `() => tryCompleteTurn(turnIdx`)
	if reserve < 0 || stop < 0 || complete < 0 || !(reserve < stop && stop < complete) {
		t.Fatalf("backend ownership rejection must stop afterRequest before complete-turn: reserve=%d stop=%d complete=%d", reserve, stop, complete)
	}
}

func TestActiveChatRescanDropsBackendOwnedPrefixFromRebuildPlan(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for active-chat rebuild ownership fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functionBody := extractArchiveCenterJSAsyncFunction(t, src, "computeActiveChatRescanDryRunPlan")
	script := functionBody + `
let allInherited = false;
async function getCurrentChatSessionId() { return "child"; }
async function resolveCurrentActiveChatObject() { return {chat:{},source:"fixture"}; }
function extractActiveChatComparableMessages() { return allInherited ? [{}] : []; }
function summarizeActiveChatRawMessageShape() { return {unparsed_count:0,sample_keys:[],reference_keys:[],raw_sample_types:[],primitive_reference_count:0,active_chat_keys:[],risu_db_root_keys:[]}; }
async function explorerFetchAllChatLogsForSession() { return {items:[],limited:false}; }
async function explorerFetchTimelineItemsForSessionDryRun() { return {items:[]}; }
async function fetchWorldRules() { return {items:[{}],count:1}; }
function buildActiveChatRescanDbRawMap() { return allInherited ? new Map([[1,{}]]) : new Map(); }
function buildSessionNormalizeCompletedTurnPairs() {
  return {available:true,pairs:[1,8,9,10].map(turn => ({turnIndex:turn,risuUserMessageIndex:(turn-1)*2,observedPairOrdinal:turn,userContent:"u"+turn,assistantContent:"a"+turn}))};
}
function buildCompletedTurnPairsFromActiveChatMessages() { throw new Error("unexpected role parser fallback"); }
async function requestBackendSessionRoutingTurnResolution() {
  if (allInherited) {
    return {status:"batch",resolvedObservations:[1,8,9,10].map((turn,index) => ({
      observation_index:index,turn_index:turn,local_turn_index:turn,resolution:"skip_pre_route_visible_pair",source:"backend",
    }))};
  }
  return {status:"batch",resolvedObservations:[
    {observation_index:0,turn_index:1,local_turn_index:1,resolution:"skip_pre_route_visible_pair",source:"backend"},
    {observation_index:1,turn_index:8,local_turn_index:8,resolution:"skip_pre_route_visible_pair",source:"backend"},
    {observation_index:2,turn_index:9,local_turn_index:9,resolution:"normal",source:"backend"},
    {observation_index:3,turn_index:10,local_turn_index:10,resolution:"normal",source:"backend"},
  ]};
}
function buildActiveChatRescanPairsFromDbRawFallback() { throw new Error("backend-owned prefix entered raw fallback"); }
function buildActiveChatRescanDerivedMap() { return new Map(); }
function buildActiveChatRescanDryRunRows(pairs) { return pairs.map(pair => ({turn_index:pair.turnIndex,raw_status:"missing",derived_status:"missing_suspected"})); }
function debugLog() {}
(async function() {
  const plan = await computeActiveChatRescanDryRunPlan("child");
  const pairTurns = plan.pairs.map(pair => pair.turnIndex);
  if (JSON.stringify(pairTurns) !== JSON.stringify([9,10])) throw new Error("inherited pairs survived rebuild plan: "+JSON.stringify(pairTurns));
  if (JSON.stringify(plan.processableTurns) !== JSON.stringify([9,10])) throw new Error("inherited turns survived processable plan: "+JSON.stringify(plan.processableTurns));
  if (plan.pairs.some(pair => pair.turnResolution !== "normal")) throw new Error("backend resolution was not preserved");
  allInherited = true;
  const inheritedPlan = await computeActiveChatRescanDryRunPlan("child");
  if (inheritedPlan.pairs.length !== 0 || inheritedPlan.processableTurns.length !== 0) throw new Error("all-inherited plan was repopulated: "+JSON.stringify(inheritedPlan.processableTurns));
})().catch(function(err) {
  console.error(err && err.stack || err);
  process.exit(1);
});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("active-chat rebuild ownership fixture failed: %v\n%s", err, out)
	}
}

func TestCompleteTurnObservationUsesRealUserAnchorForAppendStyleReroll(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for source acceptance observation fixture")
		}
	}
	src := readArchiveCenterJS(t)
	activeWindowBody := extractArchiveCenterJSFunction(t, src, "getRisuActiveMessageWindowStart")
	functionBody := extractArchiveCenterJSAsyncFunction(t, src, "buildCompleteTurnSourceAcceptanceObservation")
	script := activeWindowBody + functionBody + `
const _streamingAfterRequestSyntheticCallDepth = 0;
const chat = {id:"chat-1",isStreaming:false,message:[
  {role:"user",data:"same user",chatId:"user-1",time:100},
  {role:"char",data:"old answer",chatId:"assistant-old",time:200,generationInfo:{generationId:"generation-old"}},
  {role:"char",data:"new answer",chatId:"assistant-new",time:300,generationInfo:{generationId:"generation-new"}},
  {role:"comment",data:"host metadata",disabled:true},
]};
function computeOrchestrationDirtyHashOr1c(value) { return "hash:"+String(value || "").trim(); }
async function resolveCurrentActiveChatObject() { return {chat}; }
function normalizeAssistantPersistenceCandidate(value) { return String(value || "").trim(); }
function isSameAssistantComparableText(a,b) { return a === b; }
function getSessionSnapshot() { return {msgCount:0}; }
function debugLog() {}
(async function() {
  const oldObservation = await buildCompleteTurnSourceAcceptanceObservation("session-1", "old answer", {allowExistingActiveMessage:true,userInput:"same user"});
  const newObservation = await buildCompleteTurnSourceAcceptanceObservation("session-1", "new answer", {allowExistingActiveMessage:true,userInput:"same user"});
  if (oldObservation.user_message_index !== 0 || newObservation.user_message_index !== 0) throw new Error("reroll candidates must share user anchor");
  if (oldObservation.user_message_chat_id !== "user-1" || newObservation.user_message_chat_id !== "user-1") throw new Error("host user id must be observed without synthesis");
  if (oldObservation.user_observed_content_hash !== newObservation.user_observed_content_hash) throw new Error("user anchor hashes differ");
  if (newObservation.position_observation !== "current_active_assistant_tail") throw new Error("new final is not active assistant tail");
  if (newObservation.later_active_turn_message_count !== 0 || newObservation.later_non_turn_message_count !== 1) throw new Error("later host metadata observation is wrong");
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("source acceptance observation JS fixture failed: %v\n%s", err, out)
	}
}

func TestCompleteTurnObservationDoesNotCrossRisuAllBeforeBoundary(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for allBefore source observation fixture")
		}
	}
	src := readArchiveCenterJS(t)
	activeWindowBody := extractArchiveCenterJSFunction(t, src, "getRisuActiveMessageWindowStart")
	observationBody := extractArchiveCenterJSAsyncFunction(t, src, "buildCompleteTurnSourceAcceptanceObservation")
	script := activeWindowBody + observationBody + `
const _streamingAfterRequestSyntheticCallDepth = 0;
const chat = {id:"chat-1",isStreaming:false,message:[
  {role:"user",data:"disabled user",chatId:"user-old",time:100},
  {role:"char",data:"disabled answer",chatId:"assistant-old",time:200,disabled:"allBefore"},
  {role:"user",data:"active user",chatId:"user-new",time:300},
  {role:"char",data:"active answer",chatId:"assistant-new",time:400,generationInfo:{generationId:"generation-new"}},
]};
function computeOrchestrationDirtyHashOr1c(value) { return "hash:"+String(value || "").trim(); }
async function resolveCurrentActiveChatObject() { return {chat}; }
function normalizeAssistantPersistenceCandidate(value) { return String(value || "").trim(); }
function isSameAssistantComparableText(a,b) { return a === b; }
function getSessionSnapshot() { return {msgCount:0}; }
function debugLog() {}
(async function() {
  const disabled = await buildCompleteTurnSourceAcceptanceObservation(
    "session-1", "disabled answer", {allowExistingActiveMessage:true,userInput:"disabled user"}
  );
  if (disabled.message_index !== -1 || disabled.user_message_index !== -1) {
    throw new Error("allBefore-disabled source was accepted: "+JSON.stringify(disabled));
  }
  const active = await buildCompleteTurnSourceAcceptanceObservation(
    "session-1", "active answer", {allowExistingActiveMessage:true,userInput:"active user"}
  );
  if (active.message_index !== 3 || active.user_message_index !== 2 ||
      active.message_chat_id !== "assistant-new" || active.user_message_chat_id !== "user-new") {
    throw new Error("active source after allBefore was not preserved: "+JSON.stringify(active));
  }
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("allBefore source observation JS fixture failed: %v\n%s", err, out)
	}
}

func TestRisuAfterRequestAcceptsCorrelatedFinalWithoutActiveChatFabrication(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for afterRequest final fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functionBody := extractArchiveCenterJSFunction(t, src, "acceptRisuAfterRequestFinal")
	script := functionBody + `
const lastOrchResult = {id:"orch-1"};
const pending = {requestId:"archive-request-1",orchResult:lastOrchResult};
const requestContext = {
  sessionId:"session-1",requestId:"archive-request-1",requestType:"model",
  hostChatId:"chat-1",requestMessageCount:2,userMessageIndex:1,
  userMessageChatId:"user-1",userMessageTimeMs:500,
  userObservedContentHash:"hash:user",userObservedContent:"user",
  state:"captured",
};
function normalizeAssistantPersistenceCandidate(value) { return String(value || "").trim(); }
function computeOrchestrationDirtyHashOr1c(value) { return "hash:"+String(value || ""); }
(async function() {
  const accepted = acceptRisuAfterRequestFinal("session-1","model",pending,requestContext,"final answer");
  if (!accepted.observed || !accepted.accepted || requestContext.state !== "accepted" ||
      requestContext.afterRequestCandidateHash !== "hash:final answer") {
    throw new Error("official afterRequest final was not accepted");
  }
  const observation = accepted.observation;
  if (!observation || observation.contract_version !== "source_acceptance_observation.v3" ||
      observation.finality_source !== "risu_afterRequest" ||
      observation.prompt_memory_availability !== "same_turn" ||
      observation.archive_center_request_correlation_id !== "archive-request-1" ||
      observation.message_index !== -1 || observation.message_chat_id ||
      observation.generation_id || observation.branch_id) {
    throw new Error("afterRequest observation lost correlation or fabricated active-chat facts");
  }
  const duplicate = acceptRisuAfterRequestFinal("session-1","model",pending,requestContext,"final answer");
  if (duplicate.observed || !duplicate.accepted || !duplicate.duplicate ||
      duplicate.reason !== "already_accepted_after_request") {
    throw new Error("duplicate afterRequest final was not ignored");
  }
  const superseded = {...requestContext,state:"superseded",requestId:"archive-request-old"};
  const oldPending = {requestId:"archive-request-old",orchResult:lastOrchResult};
  if (acceptRisuAfterRequestFinal("session-1","model",oldPending,superseded,"old").observed) {
    throw new Error("superseded request accepted a final");
  }
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("afterRequest final fixture failed: %v\n%s", err, out)
	}
}

func TestRisuAfterRequestReturnsPocketRisuRerollBeforePersistence(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for PocketRisu afterRequest return fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functionBody := extractArchiveCenterJSFunction(t, src, "onAfterRequest")
	script := `
const settings = {enabled:true};
const SESSION_FALLBACK = "session-default";
const orch = {_chatSessionId:"session-1"};
let lastOrchResult = orch;
const _sessionCache = {sessionId:"session-1"};
const pending = {requestId:"request-reroll",orchResult:orch};
const requestContext = {requestId:"request-reroll"};
const _pendingOrchBySession = new Map([["session-1",pending]]);
const _pendingPersistenceSkipBySession = new Map();
const _finalConfirmationRequestBySession = new Map([["session-1",requestContext]]);
const _failedQueue = [];
let scheduled = 0;
let accepted = false;
let resolverCalls = 0;
function recordRisuHookLifecycle(){}
function debugLog(){}
function warnLog(){}
function updateRuntimeState(){}
function isNarrativeType(type){ return !type || type === "model"; }
function isSaveType(type){ return !type || type === "model"; }
function normalizeSessionId(value){ return String(value || "").trim(); }
function takeNonMainRequestSkip(){ return null; }
function schedulePostOutputFinalReplacement(){ throw new Error("unexpected secondary replacement"); }
function markNonMainRequestHookSkipped(){ throw new Error("unexpected non-main skip"); }
function normalizeAssistantPersistenceCandidate(value){ return String(value || "").trim(); }
function takeAssistantPrefillSeedForSession(){ return ""; }
function sanitizeNarrativeOutputForDisplay(value){ return String(value || ""); }
function buildSanitizeTrace(){ return null; }
function stripAssistantPrefillFromResponse(value){ return String(value || ""); }
function acceptRisuAfterRequestFinal(){
  if (accepted) {
    return {accepted:true,duplicate:true,reason:"already_accepted_after_request"};
  }
  accepted = true;
  return {
    accepted:true,
    duplicate:false,
    observation:{accepted:true,finality_source:"risu_afterRequest"},
    observationKey:"reroll-observation",
  };
}
function resolveAfterRequestWriteSessionId(){
  resolverCalls++;
  return new Promise(function(){});
}
const originalResolve = Promise.resolve;
Promise.resolve = function(){
  return {
    then(callback){
      scheduled++;
      return {catch(){}};
    },
  };
};
` + functionBody + `
try {
  const first = onAfterRequest("new PocketRisu reroll", "model");
  if (first && typeof first.then === "function") {
    throw new Error("afterRequest returned a promise and can withhold the reroll output");
  }
  if (first !== "new PocketRisu reroll") {
    throw new Error("reroll output was not returned immediately: "+String(first));
  }
  if (resolverCalls !== 0) {
    throw new Error("afterRequest called the blocking session resolver");
  }
  if (scheduled !== 1) {
    throw new Error("accepted reroll persistence was not scheduled exactly once: "+scheduled);
  }
  const duplicate = onAfterRequest("new PocketRisu reroll", "model");
  if (duplicate !== "new PocketRisu reroll") {
    throw new Error("duplicate callback changed the visible reroll output");
  }
  if (scheduled !== 1) {
    throw new Error("duplicate afterRequest scheduled persistence twice: "+scheduled);
  }
} finally {
  Promise.resolve = originalResolve;
}
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("PocketRisu afterRequest return fixture failed: %v\n%s", err, out)
	}
}

func TestRisuAfterRequestObservationBypassesActiveChatReread(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for afterRequest observation fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functionBody := extractArchiveCenterJSAsyncFunction(t, src, "buildCompleteTurnSourceAcceptanceObservation")
	script := functionBody + `
function computeOrchestrationDirtyHashOr1c(value) { return "hash:"+String(value || "").trim(); }
async function resolveCurrentActiveChatObject() { throw new Error("v3 reread active chat"); }
function normalizeAssistantPersistenceCandidate(value) { return String(value || "").trim(); }
function isSameAssistantComparableText(a,b) { return a === b; }
function getSessionSnapshot() { return null; }
function debugLog() {}
(async function() {
  const finality = {
    accepted:true,contract_version:"source_acceptance_observation.v3",
    host_lifecycle_contract_version:"risu_host_lifecycle_observation.v1",
    observed_at_ms:1000,session_id:"session-1",finality_source:"risu_afterRequest",
    finality_state:"received_final_response",host_signal_source:"afterRequest",
    archive_center_request_correlation_id:"archive-request-1",
    request_id_provenance:"archive_center_correlation",
    request_correlation_state:"matched_before_request_context",request_type:"model",
    response_role:"assistant",after_request_content_hash:"hash:persisted answer",
    host_chat_id:"chat-1",host_chat_id_state:"observed_before_request",
    chat_streaming_state:"not_exposed_by_risu_afterRequest",
    active_message_count:0,request_message_count:2,message_index:-1,message_role:"",
    message_chat_id:"",message_chat_id_state:"not_exposed_by_risu_afterRequest",
    generation_id:"",generation_id_state:"not_exposed_by_risu_afterRequest",
    branch_id:"",branch_id_state:"not_exposed_by_risuai",
    message_swipe_id:-1,message_swipe_id_state:"unobserved",
    message_time_ms:0,message_time_state:"not_exposed_by_risu_afterRequest",
    user_message_index:1,user_message_chat_id:"user-1",
    user_message_chat_id_state:"observed_before_request",
    user_message_time_ms:500,user_message_time_state:"observed_before_request",
    user_observed_content_hash:"hash:user",user_persistence_content_hash:"hash:user",
    observed_content_hash:"hash:persisted answer",persistence_content_hash:"hash:persisted answer",
    hash_algorithm:"or1c_utf16_djb2.v1",
    position_observation:"not_exposed_by_risu_afterRequest",
    later_active_turn_message_count:0,later_disabled_turn_message_count:0,later_non_turn_message_count:0,
    message_disabled_state:"not_exposed_by_risu_afterRequest",revision_state:"not_exposed_by_risuai",
    prompt_memory_availability:"same_turn",
  };
  const observation = await buildCompleteTurnSourceAcceptanceObservation(
    "session-1","persisted answer",{sourceAcceptanceFinality:finality,userInput:"user"}
  );
  if (observation.contract_version !== "source_acceptance_observation.v3" ||
      observation.finality_source !== "risu_afterRequest" ||
      observation.user_persistence_content_hash !== "hash:user" ||
      observation.persistence_content_hash !== "hash:persisted answer") {
    throw new Error("official afterRequest observation was not preserved");
  }
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("afterRequest observation fixture failed: %v\n%s", err, out)
	}
}

func TestRisuNextHostSignalObservationPreservesCommittedChatFacts(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for next-host-signal observation fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functionBody := extractArchiveCenterJSAsyncFunction(t, src, "buildCompleteTurnSourceAcceptanceObservation")
	script := functionBody + `
function computeOrchestrationDirtyHashOr1c(value) { return "hash:"+String(value || "").trim(); }
async function resolveCurrentActiveChatObject() { throw new Error("next-host-signal v2 reread active chat"); }
function normalizeAssistantPersistenceCandidate(value) { return String(value || "").trim(); }
function isSameAssistantComparableText(a,b) { return a === b; }
function getSessionSnapshot() { return null; }
function debugLog() {}
(async function() {
  const finality = {
    accepted:true,contract_version:"source_acceptance_observation.v2",
    host_lifecycle_contract_version:"risu_host_lifecycle_observation.v1",
    observed_at_ms:1000,session_id:"session-1",finality_source:"risu_next_host_signal_active_chat",
    finality_state:"committed_assistant_observed",host_signal_source:"beforeRequest",
    archive_center_request_correlation_id:"archive-request-1",
    request_id_provenance:"archive_center_correlation",
    request_correlation_state:"matched_before_request_context",request_type:"model",
    after_request_content_hash:"hash:persisted answer",host_chat_id:"chat-1",
    host_chat_id_state:"observed",chat_streaming_state:"not_streaming",
    active_message_count:4,message_index:2,message_role:"char",
    message_chat_id:"assistant-1",message_chat_id_state:"observed",
    generation_id:"generation-1",generation_id_state:"observed",
    message_time_ms:900,message_time_state:"observed",request_message_count:2,
    user_message_index:1,user_message_chat_id:"user-1",
    user_message_chat_id_state:"observed_before_request",
    user_message_time_ms:500,user_message_time_state:"observed_before_request",
    user_observed_content_hash:"hash:user",
    observed_content_hash:"hash:persisted answer",
    position_observation:"committed_before_next_host_signal",
    later_active_turn_message_count:1,next_signal_active_role:"user",
    next_signal_user_index:3,next_signal_user_observed_content_hash:"hash:next user",
    message_disabled_state:"not_disabled",revision_state:"not_exposed_by_risuai",
  };
  const observation = await buildCompleteTurnSourceAcceptanceObservation(
    "session-1","persisted answer",{sourceAcceptanceFinality:finality,userInput:"user"}
  );
  if (observation.contract_version !== "source_acceptance_observation.v2" ||
      observation.finality_source !== "risu_next_host_signal_active_chat" ||
      observation.generation_id !== "generation-1" ||
      observation.message_index !== 2 ||
      observation.next_signal_user_index !== 3 ||
      observation.position_observation !== "committed_before_next_host_signal") {
    throw new Error("next-host-signal v2 lost active-chat commit facts");
  }
  if (observation.observed_content_hash !== "hash:persisted answer" ||
      observation.persistence_content_hash !== "hash:persisted answer") {
    throw new Error("committed and persisted hashes lost provenance");
  }
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("next-host-signal observation fixture failed: %v\n%s", err, out)
	}
}

func TestRisuHostFinalConfirmationAcceptsExactSlotAndOneTrailingCurrentUser(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for exact host-final fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functionBody := extractArchiveCenterJSFunction(t, src, "observePendingFinalConfirmationAtHostSignal")
	script := functionBody + `
let activeChat = {id:"chat-a",isStreaming:false,message:[
  {role:"char",data:"greeting"},
  {role:"user",data:"question",chatId:"user-1",time:100},
  {role:"char",data:"append final",chatId:"assistant-1",time:200,generationInfo:{generationId:"g-append"}},
  {role:"user",data:"next question",chatId:"user-2",time:300},
]};
const R = {
  async getCurrentCharacterIndex() { return 7; },
  async getCurrentChatIndex() { return 3; },
  async getChatFromIndex() { return activeChat; },
};
const _finalConfirmationRequestBySession = new Map();
let persisted = [];
function normalizeAssistantPersistenceCandidate(value) { return String(value || "").trim(); }
function computeOrchestrationDirtyHashOr1c(value) { return "h:" + String(value || "").trim(); }
function extractActiveChatComparableMessages(chat) {
  return chat.message.map((item,index)=>({
    role:item.role==="char"?"assistant":item.role,
    content:item.data,risuMessageIndex:index,
  }));
}
async function backfillOneActiveChatCompletedTurn(sid,pair,options) {
  persisted.push({sid,pair,observation:options.sourceAcceptanceFinality});
  return {status:"saved"};
}
function updateRuntimeState() {}
function warnLog() {}
function debugLog() {}

(async function() {
  const appendContext = {
    state:"candidate_observed",sessionId:"session-1",requestId:"request-1",requestType:"model",
    characterIndex:7,chatIndex:3,hostChatId:"chat-a",requestMessageCount:2,
    userMessageIndex:1,userMessageChatId:"user-1",userMessageTimeMs:100,
    userObservedContentHash:"h:question",userObservedContent:"question",
    baselineAssistantIndex:-1,afterRequestCandidateHash:"h:append final",
  };
  _finalConfirmationRequestBySession.set("session-1",appendContext);
  const append = await observePendingFinalConfirmationAtHostSignal("session-1","beforeRequest");
  await Promise.resolve();
  if (!append.accepted || appendContext.state !== "accepted" || persisted.length !== 1) {
    throw new Error("exact append slot was not confirmed");
  }
  const observed = persisted[0].observation;
  if (observed.later_active_turn_message_count !== 1 ||
      observed.next_signal_active_role !== "user" ||
      observed.next_signal_user_index !== 3 ||
      observed.next_signal_user_observed_content_hash !== "h:next question" ||
      observed.prompt_memory_availability !== "one_turn_late") {
    throw new Error("trailing current-user anchor was not preserved");
  }
  const duplicate = await observePendingFinalConfirmationAtHostSignal("session-1","input");
  if (!duplicate.accepted || !duplicate.duplicate || persisted.length !== 1) {
    throw new Error("accepted host final persisted more than once");
  }
  const replaceContext = {
    state:"captured",sessionId:"session-2",requestId:"request-2",requestType:"model",
    characterIndex:7,chatIndex:3,hostChatId:"chat-a",requestMessageCount:2,
    userMessageIndex:0,userObservedContentHash:"h:question",
    baselineAssistantIndex:1,baselineAssistantContentHash:"h:old final",
    baselineGenerationId:"g-old",baselineAssistantTimeMs:150,
  };
  activeChat = {id:"chat-a",isStreaming:false,message:[
    {role:"user",data:"question",chatId:"user-reroll",time:100},
    {role:"char",data:"rerolled final",chatId:"assistant-reroll",time:250,generationInfo:{generationId:"g-new"}},
  ]};
  _finalConfirmationRequestBySession.set("session-2",replaceContext);
  const reroll = await observePendingFinalConfirmationAtHostSignal("session-2","input");
  await Promise.resolve();
  if (!reroll.accepted || persisted.length !== 2 ||
      persisted[1].observation.message_index !== 1 ||
      persisted[1].observation.generation_id !== "g-new") {
    throw new Error("same-length reroll replacement was not confirmed");
  }
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("exact host-final fixture failed: %v\n%s", err, out)
	}
}

func TestRollbackDecisionForwardsObservedHostGenerationLifecycle(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for rollback observation fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functionBody := extractArchiveCenterJSAsyncFunction(t, src, "requestBackendRollbackDecision")
	script := functionBody + `
let capturedBody = null;
function getRequestTimeoutSettingMs() { return 90000; }
function serializeSessionRoutingBaselineForBackend() { return null; }
async function bridgeFetch(path, options) {
  if (path !== "/rollback/decision") throw new Error("unexpected path");
  capturedBody = options.body;
  return {status:"ok",contract_version:"rollback.decision.v1",allowed:false,reason:"pending_output_guard"};
}

(async function() {
  await requestBackendRollbackDecision("session-1", 4, "active_chat_tail_missing_from_runtime", {
    backendLatestTurnIndex:4,
    hostLifecycleObservation:"before_request_observed",
  }, "auto");
  if (!capturedBody || capturedBody.host_lifecycle_observation !== "before_request_observed") {
    throw new Error("observed host generation lifecycle was not forwarded");
  }
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rollback host lifecycle observation fixture failed: %v\n%s", err, out)
	}
}

func TestQueuedCompleteTurnRebuildsFromSameTurnActiveHostFinal(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for queued active-final fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functionBody := extractArchiveCenterJSAsyncFunction(t, src, "refreshQueuedCompleteTurnSourceObservation")
	script := functionBody + `
let buildAssistant = "";
let buildOptions = null;
let resolvedTurn = 15;
async function findActiveChatCompletedTurnPairForContent() { return null; }
async function findActiveChatCompletedTurnPairForUserContent() {
  return {observedPairOrdinal:15,userContent:"user",assistantContent:"host final",pairCount:15};
}
async function requestBackendSessionRoutingTurnResolution() { return {turnIndex:resolvedTurn,status:"normal"}; }
async function buildCompleteTurnRequestBody(turn,user,assistant,context,sid,trace,options) {
  buildAssistant = assistant;
  buildOptions = options;
  return {chat_session_id:sid,turn_index:turn,user_input:user,assistant_content:assistant,context_messages:context,
    improvement_trace:trace,request_type:"model",client_meta:{idempotency_key:"host-final-key",source_acceptance_observation:{
      observed_content_hash:"host-final-hash",hash_algorithm:"or1c_utf16_djb2.v1",generation_id:"generation-host-final",
      generation_id_state:"observed",position_observation:"current_active_chat_tail"
    }}};
}
function buildCompleteTurnQueuePayload(body) { return JSON.parse(JSON.stringify(body)); }
(async function() {
  const payload={chat_session_id:"session-1",turn_index:15,user_input:"user",assistant_content:"native before host apply",context_messages:[],client_meta:{
    idempotency_key:"old-key",source_acceptance_observation:{active_message_count:151},
    source_to_final_lineage_observation:{contract_version:"source_to_final_lineage_observation.v1",status:"ready",
      archive_center_request_correlation_id:"correlation-original",prepare_lineage_id:"stl_original",
      payload_plan_id:"stp_original",generation_id_state:"unobserved",payload_application_status:"applied",
      payload_observation_stage:"archive_center_before_request_return",final_provider_payload_state:"not_exposed",
      source_refs:["memory:session-1:41"],semantic_outcome:"unobserved"}
  }};
  const ok=await refreshQueuedCompleteTurnSourceObservation(payload);
  if(!ok) throw new Error("same-turn active host final did not refresh");
  if(buildAssistant!=="host final" || payload.assistant_content!=="host final") throw new Error("queued assistant was not replaced by host final");
  if(!buildOptions || buildOptions.allowExistingActiveMessage!==true) throw new Error("retry did not allow live existing message observation");
  if(payload.client_meta.idempotency_key!=="host-final-key") throw new Error("idempotency key was not rebuilt");
  const lineage=payload.client_meta.source_to_final_lineage_observation;
  if(!lineage || lineage.archive_center_request_correlation_id!=="correlation-original" ||
    lineage.prepare_lineage_id!=="stl_original" || lineage.payload_plan_id!=="stp_original") {
    throw new Error("queued source lineage correlation was replaced");
  }
  if(lineage.generation_id!=="generation-host-final" || lineage.final_observed_content_hash!=="host-final-hash") {
    throw new Error("queued source lineage did not refresh only the active-final observation");
  }
  resolvedTurn = 16;
  const wrongTurn={chat_session_id:"session-1",turn_index:15,user_input:"user",assistant_content:"native before host apply",context_messages:[],client_meta:{}};
  if(await refreshQueuedCompleteTurnSourceObservation(wrongTurn)) throw new Error("different logical turn was adopted");
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("queued active host final fixture failed: %v\n%s", err, out)
	}
}

func TestPersistedCompleteTurnQueueKeepsSourceFenceWithoutCredentials(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for complete-turn queue fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functions := extractArchiveCenterJSFunction(t, src, "buildCompleteTurnQueuePayload") +
		extractArchiveCenterJSFunction(t, src, "serializeCompleteTurnRecoveryPayload")
	script := functions + `
function normalizeLanguageContextTrace(value) { return value; }
function buildRisuActiveChatContextMessageObservation(msg) { return msg; }
const sourceObservation = {contract_version:"source_acceptance_observation.v1",observed_at_ms:123,message_index:4,active_message_count:5};
const sourceLineage = {contract_version:"source_to_final_lineage_observation.v1",status:"ready",
  archive_center_request_correlation_id:"correlation-1",prepare_lineage_id:"stl_1",payload_plan_id:"stp_1",
  generation_id:"generation-1",generation_id_state:"observed",source_revision:"source-revision-7",source_refs:["memory:session-1:41"],
  payload_application_status:"applied",payload_observation_stage:"archive_center_before_request_return",
  final_provider_payload_state:"not_exposed",semantic_outcome:"unobserved"};
const exactContext=Array.from({length:25},(_,index)=>({
  role:index%2===0?"user":"assistant",
  content:index===0?"x".repeat(2101):"context-"+index,
}));
const saved = serializeCompleteTurnRecoveryPayload({
  chat_session_id:"session-1",turn_index:3,user_input:"user",assistant_content:"assistant",context_messages:exactContext,
  client_meta:{source_acceptance_required:true,source_acceptance_observation:sourceObservation,
    source_to_final_lineage_observation:sourceLineage,idempotency_key:"key-1",source_revision:"source-revision-7",
    critic_input_budget_observation:{contract_version:"critic_input_budget_observation.v1",max_input_context_chars:975},
    critic:{api_key:"secret"},authorization:"Bearer secret"}
});
if (!saved || saved.client_meta.source_acceptance_required !== true) throw new Error("source fence requirement was lost");
if (!saved.client_meta.source_acceptance_observation || saved.client_meta.source_acceptance_observation.message_index !== 4) throw new Error("source observation was lost");
if (saved.client_meta.idempotency_key !== "key-1") throw new Error("idempotency key was lost");
if (saved.client_meta.source_revision !== "source-revision-7") throw new Error("source revision was lost");
if (!saved.client_meta.critic_input_budget_observation ||
    saved.client_meta.critic_input_budget_observation.contract_version!=="critic_input_budget_observation.v1" ||
    saved.client_meta.critic_input_budget_observation.max_input_context_chars!==975) throw new Error("critic input budget observation was lost");
const savedLineage=saved.client_meta.source_to_final_lineage_observation;
if (!savedLineage || savedLineage.archive_center_request_correlation_id!=="correlation-1" ||
  savedLineage.prepare_lineage_id!=="stl_1" || savedLineage.payload_plan_id!=="stp_1" ||
  savedLineage.source_revision!=="source-revision-7" ||
  savedLineage.status!=="ready" || savedLineage.payload_observation_stage!=="archive_center_before_request_return" ||
  savedLineage.final_provider_payload_state!=="not_exposed") throw new Error("source lineage fence was lost");
if (saved.context_messages.length!==exactContext.length ||
    saved.context_messages[0].content!==exactContext[0].content) throw new Error("retry queue truncated exact critic context");
if (saved.client_meta.critic || JSON.stringify(saved).includes("secret") || JSON.stringify(saved).includes("Bearer")) throw new Error("credential-bearing config was persisted");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("persisted complete-turn source fence fixture failed: %v\n%s", err, out)
	}
}

func TestPendingFinalConfirmationPersistsSeparatelyAndRestores(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for pending-final recovery fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functions := strings.Join([]string{
		extractArchiveCenterJSFunction(t, src, "buildCompleteTurnQueuePayload"),
		extractArchiveCenterJSFunction(t, src, "serializeCompleteTurnRecoveryPayload"),
		extractArchiveCenterJSFunction(t, src, "pendingFinalConfirmationRecoveryKey"),
		extractArchiveCenterJSFunction(t, src, "serializePendingFinalConfirmationRecovery"),
		extractArchiveCenterJSAsyncFunction(t, src, "savePendingFinalConfirmationRecoveryToStorage"),
		extractArchiveCenterJSAsyncFunction(t, src, "commitPendingFinalConfirmationTransitionIntent"),
		extractArchiveCenterJSAsyncFunction(t, src, "persistPendingFinalConfirmationRecovery"),
		extractArchiveCenterJSFunction(t, src, "removeFailedCompleteTurnByIdempotencyKey"),
		extractArchiveCenterJSAsyncFunction(t, src, "markPendingFinalConfirmationRecoveryTerminal"),
		extractArchiveCenterJSAsyncFunction(t, src, "loadPendingFinalConfirmationRecoveryFromStorage"),
	}, "\n")
	script := functions + `
const PENDING_FINAL_CONFIRMATION_STORAGE_KEY="pending-final";
const _pendingFinalConfirmationRecoveryEntries=new Map();
const _failedQueue=[];
const settings={failedQueueMaxSize:50};
	let stored="";
	let localStored="";
	let flushed=0;
let queued=null;
	function normalizeLanguageContextTrace(value) { return value; }
	function safeStorageGet(key) {
	  if (key!==PENDING_FINAL_CONFIRMATION_STORAGE_KEY) throw new Error("wrong local storage key");
	  return localStored;
	}
	function safeStorageSet(key,value) {
	  if (key!==PENDING_FINAL_CONFIRMATION_STORAGE_KEY) throw new Error("wrong local storage key");
	  localStored=value;
	}
	async function persistentSet(key,value) {
	  if (key!==PENDING_FINAL_CONFIRMATION_STORAGE_KEY) throw new Error("wrong storage key");
	  safeStorageSet(key,value);
	  stored=value;
}
async function persistentGet(key) {
  if (key!==PENDING_FINAL_CONFIRMATION_STORAGE_KEY) throw new Error("wrong storage key");
  return stored;
}
async function flushQueueSave() { flushed++; }
async function queuePendingCompleteTurnPayload(payload,reason,required,options) {
  queued={payload,reason,required,options};
  return true;
}
function warnLog() {}
function updateRuntimeState() {}
(async function() {
  const payload={chat_session_id:"session-1",turn_index:4,user_input:"user",assistant_content:"assistant",context_messages:[],
    client_meta:{idempotency_key:"key-4",source_acceptance_required:true,
      source_acceptance_observation:{host_chat_id:"chat-1",message_index:3,active_message_count:4},
      critic:{api_key:"critic-secret"},embedding:{api_key:"embedding-secret"}}};
  if(!await persistPendingFinalConfirmationRecovery(payload,"retry_after_new_observation","obs-old","")) {
    throw new Error("pending final was not persisted");
  }
  if(!stored || stored.includes("critic-secret") || stored.includes("embedding-secret")) {
    throw new Error("pending final storage leaked live credentials");
  }
  _pendingFinalConfirmationRecoveryEntries.clear();
  _failedQueue.push({type:"complete_turn",payload:{chat_session_id:"session-1",turn_index:4,
    client_meta:{idempotency_key:"key-4"}}});
  const restored=await loadPendingFinalConfirmationRecoveryFromStorage();
  if(restored!==1 || !queued || queued.required!=="obs-old" || queued.options.persist!==false ||
     queued.options.reconciliationRequired!==true) {
    throw new Error("pending final recovery was not reconstructed");
  }
  if(_failedQueue.length!==0 || flushed!==1) {
    throw new Error("failed-queue duplicate was not atomically transferred on reload");
  }
  if(queued.payload.client_meta.critic || queued.payload.client_meta.embedding) {
    throw new Error("recovered pending payload unexpectedly contains credentials");
  }
	  const terminalTransition=await markPendingFinalConfirmationRecoveryTerminal(payload,"","failed_queue_persistence_failed");
	  if(!terminalTransition || !terminalTransition.durable || !terminalTransition.plugin_persisted) {
	    throw new Error("pending terminal incident was not persisted");
	  }
  _pendingFinalConfirmationRecoveryEntries.clear();
  queued=null;
  const terminalRestored=await loadPendingFinalConfirmationRecoveryFromStorage();
  if(terminalRestored!==0 || queued!==null) {
    throw new Error("terminal pending incident was retried after reload");
  }
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pending-final recovery fixture failed: %v\n%s", err, out)
	}
}

func TestPendingCompleteTurnRequiresChangedHostObservation(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for pending observation fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functions := extractArchiveCenterJSFunction(t, src, "completeTurnPayloadObservationKey") +
		extractArchiveCenterJSAsyncFunction(t, src, "observePendingCompleteTurnFinalObservation")
	script := functions + `
function normalizeAssistantPersistenceCandidate(value) { return String(value || "").trim(); }
function computeOrchestrationDirtyHashOr1c(value) { return "h:" + String(value || ""); }
async function refreshQueuedCompleteTurnSourceObservation() { return true; }
(async function() {
  const payload={chat_session_id:"session-1",assistant_content:"same final",client_meta:{source_acceptance_observation:{
    host_chat_id_state:"observed",host_chat_id:"chat-1",chat_streaming_state:"not_streaming",
    message_role:"char",message_disabled_state:"not_disabled",position_observation:"current_active_chat_tail",
    active_message_count:2,message_index:1,generation_id:"g-1",
    observed_content_hash:"raw-hash",persistence_content_hash:"persist-hash"
  }}};
  const sameKey=completeTurnPayloadObservationKey(payload);
  const pending={payload,requestContext:null,requiredObservationChangeFrom:sameKey};
  const same=await observePendingCompleteTurnFinalObservation(pending);
  if(same.confirmed || same.reason!=="new_host_observation_required") {
    throw new Error("same host observation was reused");
  }
  payload.client_meta.source_acceptance_observation.generation_id="g-2";
  const changed=await observePendingCompleteTurnFinalObservation(pending);
  if(!changed.confirmed || changed.observationKey===sameKey) {
    throw new Error("changed host generation was not accepted");
  }
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pending observation fixture failed: %v\n%s", err, out)
	}
}

func TestConfirmedPendingFinalQueueFailureBecomesTerminalIncident(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for confirmed pending-final fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functions := extractArchiveCenterJSFunction(t, src, "completeTurnNeedsFreshReconciliationRetry") +
		extractArchiveCenterJSFunction(t, src, "pendingFinalConfirmationRecoveryKey") +
		extractArchiveCenterJSAsyncFunction(t, src, "queuePendingCompleteTurnPayload")
	script := functions + `
const _finalConfirmationRequestBySession=new Map();
const requestContext={state:"captured"};
_finalConfirmationRequestBySession.set("session-1",requestContext);
let queuedCount=0;
let pending=null;
let terminalMarked=0;
let lastComplete=null;
async function persistPendingFinalConfirmationRecovery() { return true; }
function queuePendingFinalConfirmation(value) { queuedCount++; pending=value; return true; }
async function refreshQueuedCompleteTurnSourceObservation() { return true; }
async function bridgeFetchWithRetry() { return null; }
function getCompleteTurnTimeoutMs() { return 10; }
function enqueue() { return {status:"accepted",queued:true,admitted:true,dedupe_key:"key-1"}; }
async function persistFailedQueueAdmission() {
  return {status:"rejected",code:"failed_queue_persistence_failed",queued:false,terminal:true};
}
async function markPendingFinalConfirmationRecoveryTerminal(payload,recoveryKey,code) {
  if(code!=="failed_queue_persistence_failed") throw new Error("wrong terminal code");
  terminalMarked++;
  return {status:"ok",code:"pending_recovery_persisted",durable:true,plugin_persisted:true};
}
function updateRuntimeState(name,status,value) {
  if(name==="lastCompleteTurnStatus") lastComplete={status,value};
}
(async function() {
  const payload={chat_session_id:"session-1",turn_index:4,assistant_content:"assistant",
    client_meta:{idempotency_key:"key-1"}};
  if(!await queuePendingCompleteTurnPayload(payload,"pending_confirmation","old-observation")) {
    throw new Error("pending payload was not admitted");
  }
  if(queuedCount!==1 || !pending) throw new Error("initial pending observation was not queued");
  await pending.resume({observationKey:"new-observation"});
  if(queuedCount!==1) throw new Error("confirmed-final transport failure re-entered pending observation");
  if(pending.state!=="terminal" || requestContext.state!=="terminal" || terminalMarked!==1) {
    throw new Error("confirmed-final failure was not terminalized");
  }
  if(!lastComplete || lastComplete.value.detail!=="failed_queue_persistence_failed") {
    throw new Error("typed terminal incident was not reported");
  }
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("confirmed pending-final terminal fixture failed: %v\n%s", err, out)
	}
}

func TestBackendPendingSupersessionRemovesRecoveryBeforeReload(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for pending supersession fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functions := strings.Join([]string{
		extractArchiveCenterJSFunction(t, src, "buildCompleteTurnQueuePayload"),
		extractArchiveCenterJSFunction(t, src, "serializeCompleteTurnRecoveryPayload"),
		extractArchiveCenterJSFunction(t, src, "pendingFinalConfirmationRecoveryKey"),
		extractArchiveCenterJSFunction(t, src, "serializePendingFinalConfirmationRecovery"),
		extractArchiveCenterJSAsyncFunction(t, src, "savePendingFinalConfirmationRecoveryToStorage"),
		extractArchiveCenterJSAsyncFunction(t, src, "commitPendingFinalConfirmationTransitionIntent"),
		extractArchiveCenterJSAsyncFunction(t, src, "persistPendingFinalConfirmationRecovery"),
		extractArchiveCenterJSAsyncFunction(t, src, "removePendingFinalConfirmationRecovery"),
		extractArchiveCenterJSAsyncFunction(t, src, "supersedePendingFinalConfirmation"),
		extractArchiveCenterJSAsyncFunction(t, src, "captureFinalConfirmationRequestContext"),
		extractArchiveCenterJSFunction(t, src, "removeFailedCompleteTurnByIdempotencyKey"),
		extractArchiveCenterJSAsyncFunction(t, src, "loadPendingFinalConfirmationRecoveryFromStorage"),
	}, "\n")
	script := functions + `
const PENDING_FINAL_CONFIRMATION_STORAGE_KEY="pending-final";
const _pendingFinalConfirmationRecoveryEntries=new Map();
const _pendingFinalConfirmations=new Map();
const _finalConfirmationRequestBySession=new Map();
const _failedQueue=[];
const settings={enabled:true,failedQueueMaxSize:50};
	let stored="";
	let localStored="";
	let restoredCalls=0;
function normalizeLanguageContextTrace(value) { return value; }
function normalizeAssistantPersistenceCandidate(value) { return String(value || "").trim(); }
function isSaveType() { return true; }
function updateRuntimeState() {}
function warnLog() {}
	function debugLog() {}
	function safeStorageGet(key) {
	  if(key!==PENDING_FINAL_CONFIRMATION_STORAGE_KEY) throw new Error("wrong local storage key");
	  return localStored;
	}
	function safeStorageSet(key,value) {
	  if(key!==PENDING_FINAL_CONFIRMATION_STORAGE_KEY) throw new Error("wrong local storage key");
	  localStored=value;
	}
	async function persistentSet(key,value) {
	  if(key!==PENDING_FINAL_CONFIRMATION_STORAGE_KEY) throw new Error("wrong storage key");
	  safeStorageSet(key,value);
	  stored=value;
}
async function persistentGet(key) {
  if(key!==PENDING_FINAL_CONFIRMATION_STORAGE_KEY) throw new Error("wrong storage key");
  return stored;
}
async function flushQueueSave() {}
async function queuePendingCompleteTurnPayload() { restoredCalls++; return true; }
const R={
  async getCurrentCharacterIndex(){return 1;},
  async getCurrentChatIndex(){return 2;},
  async getChatFromIndex(){return {id:"host-chat",message:[]};}
};
(async function() {
  const payload={chat_session_id:"session-1",turn_index:4,user_input:"user",assistant_content:"assistant",
    context_messages:[],client_meta:{idempotency_key:"key-4"}};
  if(!await persistPendingFinalConfirmationRecovery(payload,"pending_confirmation","old-observation","")) {
    throw new Error("pending recovery was not persisted");
  }
  const previousContext={state:"captured",requestId:"request-old"};
  _finalConfirmationRequestBySession.set("session-1",previousContext);
  const recoveryKey=pendingFinalConfirmationRecoveryKey(payload);
  const pending={kind:"backend_observation_retry",sessionId:"session-1",payload,
    requestContext:previousContext,recoveryKey,state:"pending"};
  _pendingFinalConfirmations.set(recoveryKey,pending);
  const next=await captureFinalConfirmationRequestContext("session-1","model","request-new");
  if(!next || previousContext.state!=="superseded" || pending.state!=="superseded") {
    throw new Error("backend pending was not superseded with request context");
  }
  if(_pendingFinalConfirmations.size!==0 || _pendingFinalConfirmationRecoveryEntries.size!==0) {
    throw new Error("superseded backend pending leaked in memory");
  }
  const persisted=JSON.parse(stored);
  if(!persisted.items || persisted.items.length!==0) throw new Error("superseded recovery remained durable");
  _pendingFinalConfirmationRecoveryEntries.clear();
  restoredCalls=0;
  const restored=await loadPendingFinalConfirmationRecoveryFromStorage();
  if(restored!==0 || restoredCalls!==0 || _pendingFinalConfirmations.size!==0) {
    throw new Error("superseded recovery returned after reload");
  }
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pending supersession reload fixture failed: %v\n%s", err, out)
	}
}

func TestPendingFinalTransitionStorageFailureRetainsDurableIntent(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for pending transition storage-failure fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functions := strings.Join([]string{
		extractArchiveCenterJSFunction(t, src, "buildCompleteTurnQueuePayload"),
		extractArchiveCenterJSFunction(t, src, "serializeCompleteTurnRecoveryPayload"),
		extractArchiveCenterJSFunction(t, src, "pendingFinalConfirmationRecoveryKey"),
		extractArchiveCenterJSFunction(t, src, "serializePendingFinalConfirmationRecovery"),
		extractArchiveCenterJSAsyncFunction(t, src, "savePendingFinalConfirmationRecoveryToStorage"),
		extractArchiveCenterJSAsyncFunction(t, src, "commitPendingFinalConfirmationTransitionIntent"),
		extractArchiveCenterJSAsyncFunction(t, src, "persistPendingFinalConfirmationRecovery"),
		extractArchiveCenterJSAsyncFunction(t, src, "removePendingFinalConfirmationRecovery"),
		extractArchiveCenterJSAsyncFunction(t, src, "markPendingFinalConfirmationRecoveryTerminal"),
		extractArchiveCenterJSFunction(t, src, "removeFailedCompleteTurnByIdempotencyKey"),
		extractArchiveCenterJSAsyncFunction(t, src, "loadPendingFinalConfirmationRecoveryFromStorage"),
	}, "\n")
	script := functions + `
const PENDING_FINAL_CONFIRMATION_STORAGE_KEY="pending-final";
const _pendingFinalConfirmationRecoveryEntries=new Map();
const _failedQueue=[];
const settings={failedQueueMaxSize:50};
let remoteStored="";
let writeCount=0;
let failOnWrite=0;
let restoredCalls=0;
let lastRestoreOptions=null;
let lastRuntimeDetail="";
function normalizeLanguageContextTrace(value) { return value; }
async function persistentSet(key,value) {
  if(key!==PENDING_FINAL_CONFIRMATION_STORAGE_KEY) throw new Error("wrong storage key");
  writeCount++;
  if(writeCount===failOnWrite) throw new Error("plugin storage transition failure");
  remoteStored=value;
}
async function persistentGet(key) {
  if(key!==PENDING_FINAL_CONFIRMATION_STORAGE_KEY) throw new Error("wrong storage key");
  return remoteStored;
}
async function queuePendingCompleteTurnPayload(payload,reason,required,options) {
  restoredCalls++;
  lastRestoreOptions=options;
  return true;
}
async function flushQueueSave() {}
function updateRuntimeState(name,status,value) {
  if(name==="lastCompleteTurnStatus") lastRuntimeDetail=String(value && value.detail || "");
}
function warnLog() {}
function payload(idempotency) {
  return {chat_session_id:"session-1",turn_index:4,user_input:"user",assistant_content:"assistant",
    context_messages:[],client_meta:{idempotency_key:idempotency}};
}
(async function() {
  const terminalPayload=payload("terminal-storage-failure");
  if(!await persistPendingFinalConfirmationRecovery(terminalPayload,"pending_confirmation","old-observation","")) {
    throw new Error("terminal fixture setup was not persisted");
  }
  failOnWrite=3;
  const terminalTransition=await markPendingFinalConfirmationRecoveryTerminal(
    terminalPayload,
    "",
    "failed_queue_persistence_failed"
  );
  if(!terminalTransition || !terminalTransition.durable || terminalTransition.plugin_persisted ||
     !terminalTransition.intent_persisted ||
     terminalTransition.code!=="pending_terminal_persistence_failed_intent_retained") {
    throw new Error("terminal transition failure did not retain durable intent");
  }
  const terminalIntentSnapshot=JSON.parse(remoteStored);
  if(terminalIntentSnapshot.items.length!==1 || terminalIntentSnapshot.items[0].state!=="pending" ||
     terminalIntentSnapshot.transition_intents.length!==1 ||
     terminalIntentSnapshot.transition_intents[0].target_state!=="terminal") {
    throw new Error("terminal intent was not committed before final state");
  }
  _pendingFinalConfirmationRecoveryEntries.clear();
  restoredCalls=0;
  const terminalRestored=await loadPendingFinalConfirmationRecoveryFromStorage();
  const terminalEntry=_pendingFinalConfirmationRecoveryEntries.get("complete|terminal-storage-failure");
  if(terminalRestored!==0 || restoredCalls!==0 || !terminalEntry || terminalEntry.state!=="terminal" ||
     terminalEntry.terminalCode!=="failed_queue_persistence_failed" ||
     lastRuntimeDetail!=="failed_queue_persistence_failed") {
    throw new Error("stale durable pending revived after terminal transition storage failure");
  }

  remoteStored="";
  writeCount=0;
  failOnWrite=0;
  _pendingFinalConfirmationRecoveryEntries.clear();
  const supersedePayload=payload("supersede-storage-failure");
  if(!await persistPendingFinalConfirmationRecovery(supersedePayload,"pending_confirmation","old-observation","")) {
    throw new Error("supersede fixture setup was not persisted");
  }
  failOnWrite=3;
  const supersedeTransition=await removePendingFinalConfirmationRecovery(
    supersedePayload,
    "",
    "new_request_superseded_pending_final"
  );
  if(!supersedeTransition || !supersedeTransition.durable || supersedeTransition.plugin_persisted ||
     !supersedeTransition.intent_persisted ||
     supersedeTransition.code!=="pending_supersede_persistence_failed_intent_retained") {
    throw new Error("supersede removal failure did not retain durable intent");
  }
  const supersedeIntentSnapshot=JSON.parse(remoteStored);
  if(supersedeIntentSnapshot.items.length!==1 || supersedeIntentSnapshot.items[0].state!=="pending" ||
     supersedeIntentSnapshot.transition_intents.length!==1 ||
     supersedeIntentSnapshot.transition_intents[0].target_state!=="superseded") {
    throw new Error("supersede intent was not committed before removal");
  }
  _pendingFinalConfirmationRecoveryEntries.clear();
  restoredCalls=0;
  const supersedeRestored=await loadPendingFinalConfirmationRecoveryFromStorage();
  if(supersedeRestored!==0 || restoredCalls!==0 || _pendingFinalConfirmationRecoveryEntries.size!==0) {
    throw new Error("stale durable pending revived after supersede removal storage failure");
  }

  remoteStored="";
  writeCount=0;
  failOnWrite=0;
  _pendingFinalConfirmationRecoveryEntries.clear();
  const intentFailurePayload=payload("intent-commit-failure");
  if(!await persistPendingFinalConfirmationRecovery(intentFailurePayload,"pending_confirmation","old-observation","")) {
    throw new Error("intent failure fixture setup was not persisted");
  }
  failOnWrite=2;
  const intentFailure=await markPendingFinalConfirmationRecoveryTerminal(
    intentFailurePayload,
    "",
    "failed_queue_persistence_failed"
  );
  const pendingEntry=_pendingFinalConfirmationRecoveryEntries.get("complete|intent-commit-failure");
  if(!intentFailure || intentFailure.durable || intentFailure.intent_persisted ||
     intentFailure.code!=="pending_terminal_intent_persistence_failed" ||
     !pendingEntry || pendingEntry.state!=="pending") {
    throw new Error("failed transition intent incorrectly claimed terminal durability");
  }
  _pendingFinalConfirmationRecoveryEntries.clear();
  restoredCalls=0;
  lastRestoreOptions=null;
  const intentFailureRestored=await loadPendingFinalConfirmationRecoveryFromStorage();
  if(intentFailureRestored!==1 || restoredCalls!==1 || !lastRestoreOptions ||
     lastRestoreOptions.reconciliationRequired!==true) {
    throw new Error("intent commit failure did not restore behind reconciliation gate");
  }
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pending transition storage-failure fixture failed: %v\n%s", err, out)
	}
}

func TestRecoveredPendingRequiresIdempotencyReconciliationBeforeRetry(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for pending reconciliation fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functions := extractArchiveCenterJSFunction(t, src, "pendingFinalConfirmationRecoveryKey") +
		extractArchiveCenterJSAsyncFunction(t, src, "queuePendingCompleteTurnPayload")
	script := functions + `
const _finalConfirmationRequestBySession=new Map();
let pending=null;
let queueCalls=0;
let statusCalls=0;
let postCalls=0;
let refreshCalls=0;
let terminalCode="";
let lastComplete=null;
async function persistPendingFinalConfirmationRecovery() { throw new Error("recovered pending must not persist again"); }
function queuePendingFinalConfirmation(value) { queueCalls++; pending=value; return true; }
async function bridgeFetch(path) {
  if(!path.startsWith("/complete-turn/request-status?idempotency_key=")) throw new Error("unexpected reconciliation path");
  statusCalls++;
  return null;
}
async function bridgeFetchWithRetry() { postCalls++; return null; }
async function refreshQueuedCompleteTurnSourceObservation() { refreshCalls++; return true; }
async function markPendingFinalConfirmationRecoveryTerminal(payload,recoveryKey,code) {
  terminalCode=code;
  return {status:"ok",code:"pending_recovery_persisted",durable:true,plugin_persisted:true,intent_persisted:true};
}
async function removePendingFinalConfirmationRecovery() { throw new Error("unreconciled recovery was removed"); }
function getRequestTimeoutSettingMs() { return 10; }
function getCompleteTurnTimeoutMs() { return 10; }
function completeTurnPayloadObservationKey() { return "observation"; }
function enqueue() { throw new Error("unreconciled recovery entered transport queue"); }
async function persistFailedQueueAdmission() { throw new Error("unreconciled recovery entered transport queue"); }
function updateRuntimeState(name,status,value) {
  if(name==="lastCompleteTurnStatus") lastComplete={status,value};
}
(async function() {
  const payload={chat_session_id:"session-1",turn_index:4,user_input:"user",assistant_content:"assistant",
    client_meta:{idempotency_key:"recovered-idempotency"}};
  if(!await queuePendingCompleteTurnPayload(
    payload,
    "pending_confirmation_recovered",
    "old-observation",
    {persist:false,reconciliationRequired:true}
  )) {
    throw new Error("recovered pending was not staged");
  }
  await pending.resume({observationKey:"new-observation"});
  if(statusCalls!==1 || postCalls!==0 || refreshCalls!==0 || queueCalls!==1) {
    throw new Error("recovered pending retried before idempotency reconciliation");
  }
  if(pending.state!=="terminal" ||
     terminalCode!=="pending_recovery_idempotency_reconciliation_unavailable" ||
     !lastComplete ||
     lastComplete.value.detail!=="pending_recovery_idempotency_reconciliation_unavailable") {
    throw new Error("unavailable reconciliation was not fail-closed and typed");
  }
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("pending reconciliation fixture failed: %v\n%s", err, out)
	}
}

func TestDashboardQueueObservationsExposeTypedStateWithoutSecrets(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for dashboard queue observation fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functionBody := extractArchiveCenterJSFunction(t, src, "buildDashboardQueueObservations")
	script := functionBody + `
const _failedQueue=[
  {type:"complete_turn",state:"retryable",attempts:1,payload:{chat_session_id:"session-1",turn_index:4,
    api_key:"transport-secret",client_meta:{idempotency_key:"retry-request",critic:{api_key:"critic-secret"}}}},
  {type:"complete_turn",state:"terminal",attempts:4,terminalCode:"idempotency_key_conflict",
    terminalAt:"2026-07-30T10:00:00.000Z",payload:{chat_session_id:"session-1",turn_index:5,
      token:"backend-token",client_meta:{idempotency_key:"terminal-request"}}}
];
const _pendingFinalConfirmations=new Map([["pending",{
  state:"pending",sessionId:"session-1",payload:{chat_session_id:"session-1",turn_index:6,
    secret:"pending-secret",client_meta:{idempotency_key:"pending-request"}}
}]]);
const _pendingFinalConfirmationRecoveryEntries=new Map([["complete|recovery-request",{
  state:"terminal",terminalCode:"failed_queue_persistence_failed",terminalAt:"2026-07-30T11:00:00.000Z",
  payload:{chat_session_id:"session-1",turn_index:7,password:"recovery-secret",
    client_meta:{idempotency_key:"recovery-request",embedding:{api_key:"embedding-secret"}}}
}]]);
function failedQueueMaxAttempts() { return 4; }
function turnWorkflowHUDRequestIdFromCompleteBody(payload) {
  return String(payload && payload.client_meta && payload.client_meta.idempotency_key || "");
}
const observations=buildDashboardQueueObservations({});
const serialized=JSON.stringify(observations);
for(const secret of ["transport-secret","critic-secret","backend-token","pending-secret","recovery-secret","embedding-secret"]) {
  if(serialized.includes(secret)) throw new Error("queue observation leaked secret: "+secret);
}
const retryable=observations.find(row=>row.queue_kind==="transport_retry" && row.request_id==="retry-request");
const terminal=observations.find(row=>row.queue_kind==="transport_retry" && row.request_id==="terminal-request");
const recovery=observations.find(row=>row.queue_kind==="pending_confirmation_recovery");
if(!retryable || retryable.state!=="retryable" || retryable.reason_code!=="" || retryable.terminal_at!=="") {
  throw new Error("retryable transport observation was not typed");
}
if(!terminal || terminal.state!=="terminal" || terminal.reason_code!=="idempotency_key_conflict" ||
   terminal.terminal_at!=="2026-07-30T10:00:00.000Z") {
  throw new Error("terminal transport observation lost reason or timestamp");
}
if(!recovery || recovery.state!=="terminal" || recovery.reason_code!=="failed_queue_persistence_failed" ||
   recovery.terminal_at!=="2026-07-30T11:00:00.000Z") {
  throw new Error("pending recovery terminal incident was not observed");
}
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("dashboard queue observation fixture failed: %v\n%s", err, out)
	}
}

func TestFailedQueuePersistenceFailureRollsBackOnlyNewAdmission(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for queue persistence fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functions := strings.Join([]string{
		extractArchiveCenterJSFunction(t, src, "stableFailedQueuePayloadFingerprint"),
		extractArchiveCenterJSFunction(t, src, "makeFailedQueueDedupeKey"),
		extractArchiveCenterJSFunction(t, src, "enqueue"),
		extractArchiveCenterJSFunction(t, src, "failedQueuePersistenceFailureResult"),
		extractArchiveCenterJSFunction(t, src, "removeQueuedItem"),
		extractArchiveCenterJSAsyncFunction(t, src, "persistFailedQueueAdmission"),
		extractArchiveCenterJSAsyncFunction(t, src, "saveFailedQueueToStorage"),
		extractArchiveCenterJSAsyncFunction(t, src, "flushQueueSave"),
	}, "\n")
	script := functions + `
const FAILED_QUEUE_STORAGE_KEY="failed";
const _failedQueue=[];
const settings={failedQueueMaxSize:4};
const runtimeState={queuePersistence:{}};
let _queueSaveScheduled=false;
let allowStore=false;
let scheduled=0;
function computeOrchestrationDirtyHashOr1c(value) { return "h:"+String(value || ""); }
function serializeFailedQueue() { return JSON.stringify({v:1,count:_failedQueue.length}); }
async function persistentSet() { if(!allowStore) throw new Error("storage unavailable"); }
function scheduleQueueSave() { scheduled++; }
function debugLog() {}
function warnLog() {}
(async function() {
  const payload={chat_session_id:"session-1",turn_index:4,user_input:"u",assistant_content:"a",
    context_messages:[],client_meta:{idempotency_key:"idem-persist"}};
  const admission=enqueue("complete_turn",payload);
  const failed=await persistFailedQueueAdmission("complete_turn",payload,admission);
  if(failed.code!=="failed_queue_persistence_failed" || !failed.terminal ||
     !failed.new_admission_rolled_back || _failedQueue.length!==0 || scheduled!==1) {
    throw new Error("new admission was not rolled back after durable write failure");
  }
  allowStore=true;
  const durableAdmission=enqueue("complete_turn",payload);
  const durable=await persistFailedQueueAdmission("complete_turn",payload,durableAdmission);
  if(!durable.queued || _failedQueue.length!==1) throw new Error("durable admission was not retained");
  allowStore=false;
  const reordered={client_meta:payload.client_meta,context_messages:[],assistant_content:"a",
    user_input:"u",turn_index:4,chat_session_id:"session-1"};
  const duplicate=enqueue("complete_turn",reordered);
  const duplicateFailure=await persistFailedQueueAdmission("complete_turn",reordered,duplicate);
  if(duplicateFailure.code!=="failed_queue_persistence_failed" || !duplicateFailure.duplicate_preserved ||
     duplicateFailure.new_admission_rolled_back || _failedQueue.length!==1) {
    throw new Error("pre-existing duplicate was removed after persistence failure");
  }
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("queue persistence rollback fixture failed: %v\n%s", err, out)
	}
}

func TestFailedQueueProductionAdmissionKeepsDistinctCompleteTurnRequests(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for failed queue admission fixture")
		}
	}
	src := readArchiveCenterJS(t)
	keyFunction := extractArchiveCenterJSFunction(t, src, "makeFailedQueueDedupeKey")
	if strings.Contains(keyFunction, "Date.now") || strings.Contains(keyFunction, "Math.random") {
		t.Fatal("failed queue identity still hides failures behind time/random fallback")
	}
	functions := extractArchiveCenterJSFunction(t, src, "stableFailedQueuePayloadFingerprint") +
		keyFunction +
		extractArchiveCenterJSFunction(t, src, "enqueue")
	script := functions + `
const _failedQueue=[];
const settings={failedQueueMaxSize:4};
let scheduled=0;
function computeOrchestrationDirtyHashOr1c(value) {
  const text=String(value || "");
  let hash=0;
  for(let i=0;i<text.length;i++) hash=((hash*33)^text.charCodeAt(i))>>>0;
  return hash.toString(16);
}
function debugLog() {}
function warnLog() {}
function scheduleQueueSave() { scheduled++; }
function payload(idempotency,hostRequest,generation,sourceRevision,assistant) {
  const clientMeta={
    source_acceptance_observation:{
      host_chat_id:"host-chat",message_index:3,message_chat_id:"message-3",
      message_time_ms:123,observed_content_hash:"content-hash",
      generation_id:generation,source_revision:sourceRevision
    },
    source_to_final_lineage_observation:{
      archive_center_request_correlation_id:hostRequest,generation_id:generation,source_revision:sourceRevision
    }
  };
  if(idempotency) clientMeta.idempotency_key=idempotency;
  return {chat_session_id:"session-1",turn_index:4,user_input:"user",assistant_content:assistant || "assistant",
    context_messages:[{role:"user",content:"user"}],request_type:"model",client_meta:clientMeta};
}
const canonicalA=payload("idem-a","host-a","generation-a",1,"assistant-a");
const canonicalChanged=payload("idem-a","host-b","generation-b",2,"assistant-b");
if(makeFailedQueueDedupeKey({type:"complete_turn",payload:canonicalA})!=="complete_turn|idempotency|idem-a") {
  throw new Error("canonical idempotency key was not preferred");
}
if(makeFailedQueueDedupeKey({type:"complete_turn",payload:canonicalA})!==
   makeFailedQueueDedupeKey({type:"complete_turn",payload:canonicalChanged})) {
  throw new Error("canonical idempotency did not dominate fallback coordinates");
}
const first=enqueue("complete_turn",canonicalA);
if(!first || first.status!=="accepted" || !first.admitted || !first.queued || first.terminal) {
  throw new Error("first request was not admitted with typed result");
}
const canonicalReordered={
  client_meta:canonicalA.client_meta,request_type:canonicalA.request_type,context_messages:canonicalA.context_messages,
  assistant_content:canonicalA.assistant_content,user_input:canonicalA.user_input,turn_index:canonicalA.turn_index,
  chat_session_id:canonicalA.chat_session_id
};
const duplicate=enqueue("complete_turn",canonicalReordered);
if(!duplicate || duplicate.status!=="duplicate" || duplicate.code!=="failed_queue_duplicate" ||
   duplicate.admitted || !duplicate.queued || !duplicate.duplicate || duplicate.terminal || _failedQueue.length!==1) {
  throw new Error("exact duplicate was not typed");
}
const conflict=enqueue("complete_turn",canonicalChanged);
if(!conflict || conflict.status!=="rejected" || conflict.code!=="failed_queue_idempotency_conflict" ||
   conflict.queued || !conflict.terminal || _failedQueue.length!==1) {
  throw new Error("same idempotency with different payload was not terminal conflict");
}
const second=enqueue("complete_turn",payload("idem-b","host-a","generation-a",1,"assistant-a"));
if(!second || second.status!=="accepted" || _failedQueue.length!==2) {
  throw new Error("distinct idempotency request was merged");
}
const fallback=payload("","host-fallback","generation-fallback",7,"assistant-fallback");
const fallbackKey=makeFailedQueueDedupeKey({type:"complete_turn",payload:fallback});
const fallbackReordered={
  request_type:fallback.request_type,
  context_messages:[{content:"user",role:"user"}],
  assistant_content:fallback.assistant_content,
  user_input:fallback.user_input,
  turn_index:fallback.turn_index,
  chat_session_id:fallback.chat_session_id,
  client_meta:{
    source_to_final_lineage_observation:{
      source_revision:7,
      generation_id:"generation-fallback",
      archive_center_request_correlation_id:"host-fallback"
    },
    source_acceptance_observation:{
      source_revision:7,
      generation_id:"generation-fallback",
      observed_content_hash:"content-hash",
      message_time_ms:123,
      message_chat_id:"message-3",
      message_index:3,
      host_chat_id:"host-chat"
    }
  }
};
if(makeFailedQueueDedupeKey({type:"complete_turn",payload:fallbackReordered})!==fallbackKey) {
  throw new Error("fallback identity changed with object property order");
}
for(const changed of [
  payload("","host-other","generation-fallback",7,"assistant-fallback"),
  payload("","host-fallback","generation-other",7,"assistant-fallback"),
  payload("","host-fallback","generation-fallback",8,"assistant-fallback")
]) {
  if(makeFailedQueueDedupeKey({type:"complete_turn",payload:changed})===fallbackKey) {
    throw new Error("fallback request/generation/source revision coordinate was merged");
  }
}
const fallbackFirst=enqueue("complete_turn",fallback);
const fallbackSecond=enqueue("complete_turn",payload("","host-other","generation-other",8,"assistant-fallback"));
if(!fallbackFirst.admitted || !fallbackSecond.admitted || _failedQueue.length!==4) {
  throw new Error("distinct fallback requests were not admitted");
}
const before=_failedQueue.map(item=>item._dedupeKey).join("\n");
const overflow=enqueue("complete_turn",payload("idem-overflow","host-overflow","generation-overflow",9,"overflow"));
const after=_failedQueue.map(item=>item._dedupeKey).join("\n");
if(!overflow || overflow.status!=="rejected" || overflow.code!=="failed_queue_capacity_reached" ||
   overflow.queued || overflow.retryable || !overflow.terminal || overflow.state!=="terminal") {
  throw new Error("capacity rejection was not typed terminal");
}
if(before!==after || _failedQueue.length!==4) throw new Error("capacity overflow evicted an existing item");
if(scheduled!==4) throw new Error("duplicate or rejected admission scheduled a queue write");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed queue production admission fixture failed: %v\n%s", err, out)
	}
}

func TestFailedQueueProductionReloadAndPruneDoNotEvictForCapacity(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for failed queue reload fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functions := strings.Join([]string{
		extractArchiveCenterJSFunction(t, src, "makeFailedQueueDedupeKey"),
		extractArchiveCenterJSFunction(t, src, "deserializeFailedQueue"),
		extractArchiveCenterJSAsyncFunction(t, src, "loadFailedQueueFromStorage"),
		extractArchiveCenterJSFunction(t, src, "prunePersistedFailedQueue"),
	}, "\n")
	script := functions + `
const FAILED_QUEUE_STORAGE_KEY="failed";
const _failedQueue=[];
const settings={failedQueueMaxAgeDays:7,failedQueueMaxSize:2};
const runtimeState={queuePersistence:{}};
let scheduled=0;
function computeOrchestrationDirtyHashOr1c(value) { return "h:"+String(value || ""); }
function debugLog() {}
function warnLog() {}
function scheduleQueueSave() { scheduled++; }
const addedAt=new Date().toISOString();
function stored(id,key) {
  return {id,type:"complete_turn",attempts:0,state:"retryable",addedAt,payload:{
    chat_session_id:"session-1",turn_index:4,user_input:"user",assistant_content:key,context_messages:[],
    client_meta:{idempotency_key:key}
  }};
}
const raw=JSON.stringify({v:1,items:[
  stored("legacy|session-1|4","idem-1"),
  stored("legacy|session-1|4","idem-2"),
  stored("legacy|session-1|4","idem-3"),
  stored("another-legacy-id","idem-3")
]});
async function persistentGet(key) {
  if(key!==FAILED_QUEUE_STORAGE_KEY) throw new Error("wrong storage key");
  return raw;
}
(async function() {
  await loadFailedQueueFromStorage();
  if(_failedQueue.length!==3) throw new Error("reload either merged distinct requests or retained an exact duplicate");
  if(_failedQueue.some(item=>!String(item._dedupeKey).startsWith("complete_turn|idempotency|idem-"))) {
    throw new Error("reload trusted a legacy coarse stored id");
  }
  prunePersistedFailedQueue();
  if(_failedQueue.length!==3) throw new Error("capacity pruning evicted restored durable items");
  if(scheduled!==0) throw new Error("capacity-only reload/prune scheduled destructive persistence");
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed queue production reload fixture failed: %v\n%s", err, out)
	}
}

func TestFailedQueueProductionDrainRetainsAndSkipsTerminalIncidents(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for failed queue drain fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functions := strings.Join([]string{
		extractArchiveCenterJSFunction(t, src, "failedQueueMaxAttempts"),
		extractArchiveCenterJSFunction(t, src, "markFailedQueueItemTerminal"),
		extractArchiveCenterJSAsyncFunction(t, src, "markFailedQueueItemTerminalDurably"),
		extractArchiveCenterJSAsyncFunction(t, src, "drainOneFailedQueueItem"),
		extractArchiveCenterJSAsyncFunction(t, src, "drainFailedQueue"),
	}, "\n")
	script := functions + `
const _failedQueue=[];
const settings={failedQueueMaxAttempts:1};
let mode="post_fail";
let postCalls=0;
let refreshCalls=0;
let scheduled=0;
function prunePersistedFailedQueue() {}
function debugLog() {}
function warnLog() {}
function updateRuntimeState() {}
function getRequestTimeoutSettingMs() { return 10; }
function getCompleteTurnTimeoutMs() { return 1; }
function completeTurnPayloadObservationKey() { return "observation"; }
function isBridgeShadowGuardFailure() { return false; }
function serializeChatLogRecoveryPayload(payload) { return payload; }
function scheduleQueueSave() { scheduled++; }
async function flushQueueSave() { scheduled++; return true; }
function makeFailedQueueDedupeKey(value) { return String(value && value._dedupeKey || ""); }
async function commitFailedQueueTransitionIntent(item,code) {
  return {status:"ok",code:"failed_queue_terminal_intent_persisted",durable:true,intent_persisted:true,
    intent:{queue_key:item._dedupeKey,target_state:"terminal",reason_code:code,transition_at:new Date().toISOString()}};
}
async function refreshQueuedCompleteTurnSourceObservation() {
  refreshCalls++;
  return mode!=="refresh_fail";
}
async function queuePendingCompleteTurnPayload() { return false; }
async function bridgeFetch() {
  if(mode==="processing") return {status:"processing"};
  if(mode==="completed_retryable") return {status:"completed",retryable:true,raw_saved:false,save_ok:false};
  return null;
}
async function bridgeFetchWithRetry() {
  postCalls++;
  if(mode==="terminal_response") {
    return {status:"rejected",queue_action:"discard",retryable:false,code:"idempotency_key_conflict"};
  }
  return null;
}
function item(key,state) {
  return {type:"complete_turn",payload:{chat_session_id:"session-1",turn_index:4,
    user_input:"u",assistant_content:"a",client_meta:{idempotency_key:key}},
    attempts:0,state:state || "retryable",addedAt:new Date(0).toISOString(),lastAttemptAt:null,
    _dedupeKey:"complete_turn|idempotency|"+key};
}
async function reset(nextMode,maxAttempts) {
  _failedQueue.length=0;
  mode=nextMode;
  settings.failedQueueMaxAttempts=maxAttempts;
  postCalls=0;
  refreshCalls=0;
  scheduled=0;
}
(async function() {
  await reset("post_fail",1);
  _failedQueue.push(item("first"));
  _failedQueue.push(item("second"));
  await drainFailedQueue();
  if(postCalls!==2 || _failedQueue.length!==2 || _failedQueue.some(row=>row.state!=="terminal")) {
    throw new Error("one host signal did not process every retryable item exactly once");
  }

  await reset("post_fail",1);
  _failedQueue.push(Object.assign(item("terminal-existing","terminal"),{terminalCode:"existing_terminal"}));
  _failedQueue.push(item("retryable"));
  await drainFailedQueue();
  if(postCalls!==1 || _failedQueue.length!==2 || _failedQueue.some(row=>row.state!=="terminal") ||
     !_failedQueue.some(row=>row.terminalCode==="retry_limit_reached")) {
    throw new Error("new terminal failure was not retained beside existing incident");
  }
  const callsAfterTerminal=postCalls;
  const schedulesAfterTerminal=scheduled;
  await drainFailedQueue();
  if(postCalls!==callsAfterTerminal || scheduled!==schedulesAfterTerminal) {
    throw new Error("terminal incidents were retried");
  }

  await reset("refresh_fail",2);
  _failedQueue.push(item("refresh"));
  await drainFailedQueue();
  if(_failedQueue[0].attempts!==1 || _failedQueue[0].state!=="retryable") {
    throw new Error("failed pending persistence did not consume an attempt");
  }
  await drainFailedQueue();
  if(_failedQueue[0].attempts!==2 || _failedQueue[0].state!=="terminal" ||
     _failedQueue[0].terminalCode!=="pending_confirmation_persistence_failed") {
    throw new Error("failed pending persistence bypassed retry limit");
  }

  await reset("processing",1);
  _failedQueue.push(item("processing"));
  await drainFailedQueue();
  if(_failedQueue.length!==1 || _failedQueue[0].state!=="terminal" ||
     _failedQueue[0].terminalCode!=="idempotent_processing_timeout") {
    throw new Error("processing timeout terminal incident was dropped");
  }

  await reset("completed_retryable",1);
  _failedQueue.push(item("completed"));
  await drainFailedQueue();
  if(_failedQueue.length!==1 || _failedQueue[0].state!=="terminal" ||
     _failedQueue[0].terminalCode!=="retry_limit_reached") {
    throw new Error("completed retry limit incident was dropped");
  }

  await reset("terminal_response",4);
  _failedQueue.push(item("terminal-response"));
  await drainFailedQueue();
  if(_failedQueue.length!==1 || _failedQueue[0].state!=="terminal" ||
     _failedQueue[0].terminalCode!=="idempotency_key_conflict") {
    throw new Error("backend terminal result was discarded");
  }
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed queue production drain fixture failed: %v\n%s", err, out)
	}
}

func TestFailedQueueTerminalWriteFailureReloadsFromDurableIntent(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for failed queue terminal durability fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functions := strings.Join([]string{
		extractArchiveCenterJSFunction(t, src, "buildCompleteTurnQueuePayload"),
		extractArchiveCenterJSFunction(t, src, "stableFailedQueuePayloadFingerprint"),
		extractArchiveCenterJSFunction(t, src, "makeFailedQueueDedupeKey"),
		extractArchiveCenterJSFunction(t, src, "serializeCompleteTurnRecoveryPayload"),
		extractArchiveCenterJSFunction(t, src, "serializeChatLogRecoveryPayload"),
		extractArchiveCenterJSFunction(t, src, "serializeFailedQueueItem"),
		extractArchiveCenterJSFunction(t, src, "serializeFailedQueue"),
		extractArchiveCenterJSFunction(t, src, "deserializeFailedQueue"),
		extractArchiveCenterJSAsyncFunction(t, src, "saveFailedQueueToStorage"),
		extractArchiveCenterJSAsyncFunction(t, src, "flushQueueSave"),
		extractArchiveCenterJSAsyncFunction(t, src, "loadFailedQueueFromStorage"),
		extractArchiveCenterJSFunction(t, src, "failedQueueMaxAttempts"),
		extractArchiveCenterJSFunction(t, src, "markFailedQueueItemTerminal"),
		extractArchiveCenterJSAsyncFunction(t, src, "commitFailedQueueTransitionIntent"),
		extractArchiveCenterJSAsyncFunction(t, src, "markFailedQueueItemTerminalDurably"),
		extractArchiveCenterJSAsyncFunction(t, src, "drainOneFailedQueueItem"),
		extractArchiveCenterJSAsyncFunction(t, src, "drainFailedQueue"),
		extractArchiveCenterJSFunction(t, src, "buildDashboardQueueObservations"),
	}, "\n")
	script := functions + `
const FAILED_QUEUE_STORAGE_KEY="failed";
const _failedQueue=[];
const _pendingFinalConfirmations=new Map();
const _pendingFinalConfirmationRecoveryEntries=new Map();
const settings={failedQueueMaxAttempts:1,failedQueueMaxAgeDays:7,failedQueueMaxSize:50};
const runtimeState={queuePersistence:{}};
let _queueSaveScheduled=false;
let remoteStored="";
let writeCount=0;
let failOnWrite=0;
let mode="retry_limit";
let postCalls=0;
function normalizeLanguageContextTrace(value) { return value; }
function computeOrchestrationDirtyHashOr1c(value) { return "h:"+String(value || ""); }
function debugLog() {}
function warnLog() {}
function updateRuntimeState() {}
function scheduleQueueSave() {}
function prunePersistedFailedQueue() {}
function getRequestTimeoutSettingMs() { return 10; }
function getCompleteTurnTimeoutMs() { return 1; }
function completeTurnPayloadObservationKey() { return "observation"; }
function turnWorkflowHUDRequestIdFromCompleteBody(payload) {
  return String(payload && payload.client_meta && payload.client_meta.idempotency_key || "");
}
function isBridgeShadowGuardFailure() { return false; }
async function persistentSet(key,value) {
  if(key!==FAILED_QUEUE_STORAGE_KEY) throw new Error("wrong storage key");
  writeCount++;
  if(writeCount===failOnWrite) throw new Error("terminal final write failed");
  remoteStored=value;
}
async function persistentGet(key) {
  if(key!==FAILED_QUEUE_STORAGE_KEY) throw new Error("wrong storage key");
  return remoteStored;
}
async function refreshQueuedCompleteTurnSourceObservation() { return true; }
async function queuePendingCompleteTurnPayload() { return false; }
async function bridgeFetch() { return null; }
async function bridgeFetchWithRetry() {
  postCalls++;
  if(mode==="terminal_response") {
    return {status:"rejected",queue_action:"discard",retryable:false,code:"idempotency_key_conflict"};
  }
  return null;
}
function item(key) {
  return {type:"complete_turn",payload:{chat_session_id:"session-1",turn_index:4,
    user_input:"user",assistant_content:"assistant",context_messages:[],
    client_meta:{idempotency_key:key}},attempts:0,state:"retryable",
    addedAt:new Date().toISOString(),lastAttemptAt:null,
    _dedupeKey:"complete_turn|idempotency|"+key};
}
async function runScenario(nextMode,key,expectedCode) {
  _failedQueue.length=0;
  remoteStored="";
  writeCount=0;
  failOnWrite=0;
  mode=nextMode;
  postCalls=0;
  _failedQueue.push(item(key));
  if(!await flushQueueSave()) throw new Error("initial retryable snapshot was not persisted");
  failOnWrite=3;
  await drainFailedQueue();
  if(_failedQueue.length!==1 || _failedQueue[0].state!=="terminal" ||
     _failedQueue[0].terminalCode!==expectedCode) {
    throw new Error("in-memory terminal transition failed");
  }
  const intentSnapshot=JSON.parse(remoteStored);
  if(intentSnapshot.items.length!==1 || intentSnapshot.items[0].state!=="retryable" ||
     intentSnapshot.transition_intents.length!==1 ||
     intentSnapshot.transition_intents[0].reason_code!==expectedCode) {
    throw new Error("remote terminal intent was not retained after final write failure");
  }
  _failedQueue.length=0;
  await loadFailedQueueFromStorage();
  if(_failedQueue.length!==1 || _failedQueue[0].state!=="terminal" ||
     _failedQueue[0].terminalCode!==expectedCode) {
    throw new Error("reload revived stale retryable queue state");
  }
  const callsBeforeSecondDrain=postCalls;
  await drainFailedQueue();
  if(postCalls!==callsBeforeSecondDrain) throw new Error("reloaded terminal incident was retried");
}
async function runIntentFailureScenario() {
  _failedQueue.length=0;
  remoteStored="";
  writeCount=0;
  failOnWrite=0;
  mode="retry_limit";
  postCalls=0;
  _failedQueue.push(item("intent-write-failure"));
  if(!await flushQueueSave()) throw new Error("intent failure setup was not persisted");
  failOnWrite=2;
  await drainFailedQueue();
  if(_failedQueue.length!==1 || _failedQueue[0].state!=="terminal" ||
     _failedQueue[0].terminalCode!=="failed_queue_terminal_intent_persistence_failed") {
    throw new Error("successful final flush did not establish terminal authority");
  }
  const blockedSnapshot=JSON.parse(remoteStored);
  if(blockedSnapshot.items.length!==1 || blockedSnapshot.items[0].state!=="terminal" ||
     blockedSnapshot.items[0].terminalCode!=="failed_queue_terminal_intent_persistence_failed") {
    throw new Error("normal final flush downgraded failed intent stop");
  }
  _failedQueue.length=0;
  await loadFailedQueueFromStorage();
  if(_failedQueue.length!==1 || _failedQueue[0].state!=="terminal" ||
     _failedQueue[0].terminalCode!=="failed_queue_terminal_intent_persistence_failed") {
    throw new Error("reload lost final-flush terminal authority");
  }
  const observation=buildDashboardQueueObservations({}).find(function(row) {
    return row.queue_kind==="transport_retry" && row.request_id==="intent-write-failure";
  });
  if(!observation || observation.state!=="terminal" ||
     observation.reason_code!=="failed_queue_terminal_intent_persistence_failed" ||
     !observation.terminal_at) {
    throw new Error("failed intent terminal authority was not typed for dashboard");
  }
  const callsBeforeSecondDrain=postCalls;
  await drainFailedQueue();
  if(postCalls!==callsBeforeSecondDrain) throw new Error("failed intent item posted after reload");
}
(async function() {
  await runScenario("retry_limit","retry-limit-intent","retry_limit_reached");
  await runScenario("terminal_response","terminal-response-intent","idempotency_key_conflict");
  await runIntentFailureScenario();
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed queue terminal durability fixture failed: %v\n%s", err, out)
	}
}

func TestPersistentSetPropagatesDurableStorageFailure(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for persistent storage fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functions := extractArchiveCenterJSFunction(t, src, "normalizePersistentValue") +
		extractArchiveCenterJSAsyncFunction(t, src, "persistentSet")
	script := functions + `
let pluginEnabled=true;
let _storageOk=true;
const _persistentKnownValues=new Map();
const _persistentPendingWrites=new Map();
const R={pluginStorage:{async setItem(){ throw new Error("plugin write failed"); }}};
function safeStorageSet() {}
function safeStorageGet() { return null; }
function _hasPluginStorage() { return pluginEnabled; }
function warnLog() {}
(async function() {
  let failed=false;
  try { await persistentSet("key","value"); } catch(err) { failed=String(err.message).includes("plugin write failed"); }
  if(!failed || _persistentKnownValues.has("key") || _persistentPendingWrites.has("key")) {
    throw new Error("plugin storage failure was swallowed");
  }
  pluginEnabled=false;
  _storageOk=false;
  failed=false;
  try { await persistentSet("memory-only","value"); } catch(err) { failed=String(err.message)==="persistent_storage_unavailable"; }
  if(!failed) throw new Error("memory-only storage was reported as durable");
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("persistent storage fixture failed: %v\n%s", err, out)
	}
}

func TestOutputFidelity35BProductionJSLineageBoundaries(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Fatalf("node is required for output-fidelity lineage fixture; set ARCHIVE_CENTER_NODE_BINARY: %v", err)
		}
	}
	src := readArchiveCenterJS(t)
	functions := strings.Join([]string{
		extractArchiveCenterJSFunction(t, src, "computeOrchestrationDirtyHashOr1c"),
		extractArchiveCenterJSFunction(t, src, "resolvePendingSourceLineageOwnership"),
		extractArchiveCenterJSFunction(t, src, "normalizeRollbackMessageRole"),
		extractArchiveCenterJSFunction(t, src, "extractMessageContentCandidate"),
		extractArchiveCenterJSFunction(t, src, "extractComparableMessageRoleAndContent"),
		extractArchiveCenterJSFunction(t, src, "auxiliaryMessageContentText"),
		extractArchiveCenterJSFunction(t, src, "getPayloadMessageRoleAndText"),
		extractArchiveCenterJSFunction(t, src, "isChatMessageLike"),
		extractArchiveCenterJSFunction(t, src, "isChatMessageArray"),
		extractArchiveCenterJSFunction(t, src, "getPayloadPathValue"),
		extractArchiveCenterJSFunction(t, src, "buildPayloadPathRebuilder"),
		extractArchiveCenterJSFunction(t, src, "findPayloadMessagesPath"),
		extractArchiveCenterJSFunction(t, src, "extractMessages"),
		extractArchiveCenterJSFunction(t, src, "observeGoPayloadApplication"),
		extractArchiveCenterJSFunction(t, src, "applyGoPayloadApplicationPlan"),
		extractArchiveCenterJSFunction(t, src, "buildSourceToFinalLineageObservation"),
	}, "\n")
	script := functions + `
const runtimeUpdates=[];
function updateRuntimeState(key,status,detail) { runtimeUpdates.push({key,status,detail}); }
function warnLog() { throw new Error("unexpected production warning"); }
const RECOMPOSER_BRIDGE_CONTRACT="archive_center_recomposer_bridge.v1";
function publishArchiveCenterRecomposerBridge() { return false; }
const exact="[Archive Center — Auxiliary Context]\n\nmemory guidance";
const plan={auxiliary_text:"memory guidance",input_context_text:"",
  auxiliary_observation_hash:computeOrchestrationDirtyHashOr1c(exact),payload_plan_id:"stp_1",
  guidance_application_trace:{final_hash:"sha256:guidance"}};
const lineage={archive_center_request_correlation_id:"correlation-1",lineage_id:"stl_1",payload_plan_id:"stp_1",
  source_refs:["memory:session-1:41"],execution_items:[{item_id:"ei_1"}]};
const activeChatAlias=getPayloadMessageRoleAndText({role:"char",content:"active chat assistant"});
if(activeChatAlias.role!=="assistant" || activeChatAlias.text!=="active chat assistant") {
  throw new Error("active-chat role normalization was bypassed by official payload parsing");
}
const good=observeGoPayloadApplication([{role:"system",content:exact}],plan,lineage);
if(good.status!=="ready" || good.payload_application_status!=="applied" || good.blocks[0].hash_match!==true) {
  throw new Error("exact returned message block was not observed");
}
if(good.final_provider_payload_state!=="not_exposed") throw new Error("provider payload boundary was overstated");
if(JSON.stringify(good).includes("memory guidance")) throw new Error("raw injected text leaked into lineage observation");
const mismatch=observeGoPayloadApplication([{role:"system",content:exact}],
  Object.assign({},plan,{auxiliary_observation_hash:"or1c_wrong"}),lineage);
if(mismatch.status!=="ambiguous" || mismatch.reason_code!=="injected_block_hash_mismatch") {
  throw new Error("payload hash mismatch was not left ambiguous");
}
const duplicate=observeGoPayloadApplication([{role:"system",content:exact},{role:"system",content:exact}],plan,lineage);
if(duplicate.status!=="ambiguous" || duplicate.reason_code!=="injected_block_position_ambiguous") {
  throw new Error("duplicate injected blocks were not left ambiguous");
}
const inputText="current scene continuity";
const exactInput="[Archive Center — Input Context]\n\n"+inputText;
const applyPlan={
  contract_version:"payload_application_plan.v1",owner:"go",
  apply_rule:"apply_exact_text_without_reassembly",status:"ready",
  auxiliary_text:"",input_context_text:inputText,
  input_context_observation_hash:computeOrchestrationDirtyHashOr1c(exactInput),
  input_context_chars:inputText.length,payload_plan_id:"stp_input",lanes:[]
};
const applyLineage={archive_center_request_correlation_id:"correlation-input",
  lineage_id:"stl_input",payload_plan_id:"stp_input",source_refs:[],execution_items:[]};
const originalPayload=[{role:"system",content:"host preset"},{role:"user",content:"continue"}];
const applied=applyGoPayloadApplicationPlan(originalPayload,{
  _injectionPack:{payload_application_plan:applyPlan},
  _sourceToPayloadLineage:applyLineage,_trace:{}
},{});
if(applied.injectionResult.applied || applied.injectionResult.inputContext.applied) {
  throw new Error("legacy input context was still reported as applied");
}
if(originalPayload.length!==2 || originalPayload.some(function(message){ return message.content===exactInput; })) {
  throw new Error("production payload application mutated the original RisuAI array");
}
const returnedMessages=extractMessages(applied.payload).messages;
const exactInputMatches=returnedMessages.filter(function(message) {
  const parsed=getPayloadMessageRoleAndText(message);
  return parsed.role==="system" && parsed.text===exactInput;
});
if(exactInputMatches.length!==0 || returnedMessages.length!==2 || returnedMessages[returnedMessages.length-1].content!=="continue") {
  throw new Error("production payload application reinjected host recent chat");
}
if(!applied.injectionResult.payloadApplicationObservation ||
  applied.injectionResult.payloadApplicationObservation.payload_application_status!=="empty") {
  throw new Error("production payload observation did not keep the input-only plan empty");
}
if(runtimeUpdates.length!==1 || runtimeUpdates[0].key!=="lastInjectionStatus" || runtimeUpdates[0].status!=="skipped") {
  throw new Error("production payload application did not publish one empty runtime state");
}
const pending={requestId:"request-1",sourceLineageAmbiguous:true};
const sticky=resolvePendingSourceLineageOwnership(pending,"request-1",false);
if(!sticky.ownsPending || !sticky.ambiguous) throw new Error("running-overlap ambiguity was lost");
const replaced=resolvePendingSourceLineageOwnership({requestId:"request-2"},"request-1",false);
if(replaced.ownsPending || replaced.ambiguous) throw new Error("replaced pending request retained ownership");
const orch={_sourceToPayloadLineage:lineage,_payloadApplicationObservation:good};
const finalReady=buildSourceToFinalLineageObservation({
  generation_id:"generation-1",generation_id_state:"observed",observed_content_hash:"or1c_final",
  hash_algorithm:"or1c_utf16_djb2.v1"
},orch);
if(!finalReady || finalReady.status!=="ready" ||
  finalReady.payload_observation_stage!=="archive_center_before_request_return" ||
  finalReady.final_provider_payload_state!=="not_exposed" || finalReady.semantic_outcome!=="unobserved") {
  throw new Error("ready final lineage boundary was not preserved");
}
orch._sourceLineageAmbiguous=true;
const finalOverlap=buildSourceToFinalLineageObservation({
  generation_id:"generation-1",generation_id_state:"observed"
},orch);
if(finalOverlap.status!=="ambiguous" || finalOverlap.reason_code!=="overlapping_main_request_lineage_ambiguous") {
  throw new Error("overlapping request attached to final output");
}
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("output-fidelity production JS lineage fixture failed: %v\n%s", err, out)
	}
}

func TestRestoredCompleteTurnQueueRebuildsLiveConfigAndSourceObservation(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for restored complete-turn queue fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functions := extractArchiveCenterJSFunction(t, src, "buildCompleteTurnQueuePayload") +
		extractArchiveCenterJSAsyncFunction(t, src, "refreshQueuedCompleteTurnSourceObservation")
	script := functions + `
function normalizeLanguageContextTrace(value) { return value; }
async function findActiveChatCompletedTurnPairForContent(session, user, assistant) {
  return {observedPairOrdinal:3,userContent:user,assistantContent:assistant,pairCount:3};
}
async function findActiveChatCompletedTurnPairForUserContent() { return null; }
async function requestBackendSessionRoutingTurnResolution() { return {turnIndex:3,status:"normal"}; }
async function buildCompleteTurnRequestBody(turn, user, assistant, context, session) {
  if (turn !== 3 || user !== "user" || assistant !== "assistant" || session !== "session-1") throw new Error("queue payload was not rebuilt");
  return {chat_session_id:session,turn_index:turn,user_input:user,assistant_content:assistant,context_messages:context,request_type:"model",client_meta:{
    source_acceptance_required:true,
    source_acceptance_observation:{observed_content_hash:"hash-final",host_chat_id:"chat-1",generation_id:"generation-final",message_chat_id:"message-final",message_time_state:"observed",message_time_ms:300},
    critic:{api_key:"must-not-be-copied"},
    embedding:{api_key:"must-not-be-copied"},
    idempotency_key:"rebuilt-key",
    request_id:"rebuilt-key"
  }};
}
function debugLog() {}
(async function() {
  const restored = {chat_session_id:"session-1",turn_index:3,user_input:"user",assistant_content:"assistant",context_messages:[],client_meta:{}};
  if (!await refreshQueuedCompleteTurnSourceObservation(restored)) throw new Error("restored queue was not refreshed");
  if (restored.client_meta.source_acceptance_required !== true || restored.client_meta.idempotency_key !== "rebuilt-key") throw new Error("source fence or request key was not rebuilt");
  if (restored.client_meta.critic || restored.client_meta.embedding) throw new Error("queued turn copied credential-bearing runtime config");

  const stale = {chat_session_id:"session-1",turn_index:3,user_input:"user",assistant_content:"assistant",context_messages:[],client_meta:{source_acceptance_observation:{observed_content_hash:"old",host_chat_id:"chat-1",generation_id:"generation-old"}}};
  if (await refreshQueuedCompleteTurnSourceObservation(stale)) throw new Error("stale generation was refreshed as current");
})().catch(function(err) { console.error(err && err.stack || err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("restored complete-turn queue refresh fixture failed: %v\n%s", err, out)
	}
}

func TestEmptyActiveChatCanVerifyFirstTurnDeletionFromLedger(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for empty active chat rollback fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functionBody := extractArchiveCenterJSFunction(t, src, "buildLedgerVerifiedTailRollback")
	script := functionBody + `
function loadRollbackTurnLedgerOr1f() {
  return {trackedTurnIndex:1,entries:[{role:"user",content:"u1"},{role:"assistant",content:"a1"}]};
}
function compactSnapshotMessages(messages) { return Array.isArray(messages) ? messages : []; }
function computeLedgerCurrentPrefixLengthOr1f(_ledger, current) { return current.length; }
function computeTailHash() { return "empty"; }
function debugLog() {}
const result = buildLedgerVerifiedTailRollback("s", [], 0, 1);
if (!result || result.status !== "verified_tail_delete" || result.rollbackFrom !== 1 || result.removedAssistantCount !== 1) {
  throw new Error("empty active chat deletion was not verified: " + JSON.stringify(result));
}
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("empty active chat rollback JS runtime fixture failed: %v\n%s", err, out)
	}
}

func TestEmptyActiveChatRecognizesIncompleteUserOnlyTailCandidate(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for incomplete tail rollback fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functionBody := extractArchiveCenterJSFunction(t, src, "buildLedgerVerifiedTailRollback")
	script := functionBody + `
let fixture = {trackedTurnIndex:1,entries:[{role:"user",turnIndex:1,fingerprint:"u1"}]};
function loadRollbackTurnLedgerOr1f() { return fixture; }
function compactSnapshotMessages(messages) { return Array.isArray(messages) ? messages : []; }
function computeLedgerCurrentPrefixLengthOr1f(_ledger, current) { return current.length; }
function computeTailHash() { return "empty"; }
function debugLog() {}
let result = buildLedgerVerifiedTailRollback("s", [], 0, 1);
if (!result || result.status !== "incomplete_user_only_tail_candidate" || result.rollbackFrom !== 1 || result.removedUserCount !== 1 || result.removedAssistantCount !== 0) {
  throw new Error("user-only tail was not recognized: " + JSON.stringify(result));
}
fixture = {trackedTurnIndex:2,entries:[{role:"user",turnIndex:1},{role:"user",turnIndex:2}]};
result = buildLedgerVerifiedTailRollback("s", [], 0, 2);
if (result !== null) throw new Error("multi-message deletion must not become an incomplete-tail candidate: " + JSON.stringify(result));
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("incomplete user-only tail JS runtime fixture failed: %v\n%s", err, out)
	}
}

func TestRollbackDecisionTransportIncludesNestedLedgerCounts(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for rollback decision transport fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functionBody := extractArchiveCenterJSAsyncFunction(t, src, "requestBackendRollbackDecision")
	script := functionBody + `
let sentBody = null;
function getRequestTimeoutSettingMs() { return 1000; }
async function bridgeFetch(path, options) {
  if (path !== "/rollback/decision") throw new Error("unexpected path " + path);
  sentBody = options.body;
  return {status:"ok",contract_version:"rollback.decision.v1"};
}
function serializeSessionRoutingBaselineForBackend() { return null; }
(async function() {
  await requestBackendRollbackDecision("s", 9, "delete", {
    tailReconcileVerification:{
      status:"incomplete_user_only_tail_candidate",
      removedAssistantCount:0,
      removedUserCount:1,
      removedMessageCount:1
    }
  }, "auto");
  if (!sentBody || sentBody.backend_latest_turn !== 0 || sentBody.removed_user_count !== 1 || sentBody.removed_message_count !== 1 || sentBody.removed_assistant_count !== 0 || sentBody.incomplete_tail_candidate !== true || sentBody.ledger_verified !== false) {
    throw new Error("nested rollback verification was not transported: " + JSON.stringify(sentBody));
  }
})().catch(function(err) {
  console.error(err && err.stack || err);
  process.exit(1);
});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rollback decision transport JS runtime fixture failed: %v\n%s", err, out)
	}
}

func TestExplorerManualDeleteRefreshesSessionAndChatLogsConcurrently(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for Explorer delete refresh fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functionBody := extractArchiveCenterJSAsyncFunction(t, src, "explorerDeleteChatLogTurn")
	script := functionBody + `
let sessionRefreshCompleted = false;
let chatRefreshObservedConcurrentStart = false;
let uiRefreshCount = 0;
function explorerSessionId() { return "session-1"; }
async function captureChatLogRestoreSnapshot() { return 2; }
async function executeAutoRollback() { return true; }
async function explorerFetchSessions() {
  await new Promise(function(resolve) { setTimeout(resolve, 25); });
  sessionRefreshCompleted = true;
}
async function explorerFetchChatLogs(reset) {
  if (reset !== true) throw new Error("chat logs were not reset");
  chatRefreshObservedConcurrentStart = !sessionRefreshCompleted;
}
async function refreshExplorerUI() { uiRefreshCount += 1; }
function debugLog() {}
function warnLog() {}
function t(key) { return key; }
function alert(message) { throw new Error("unexpected alert: " + message); }
(async function() {
  const ok = await explorerDeleteChatLogTurn(2);
  if (!ok || !chatRefreshObservedConcurrentStart || uiRefreshCount !== 1) {
    throw new Error("Explorer delete refresh was not concurrent: " + JSON.stringify({ok, chatRefreshObservedConcurrentStart, uiRefreshCount}));
  }
})().catch(function(err) {
  console.error(err && err.stack || err);
  process.exit(1);
});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Explorer delete refresh fixture failed: %v\n%s", err, out)
	}
}

func TestRollbackReadsCanonicalRisuChatAfterDeletion(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for canonical rollback chat fixture")
		}
	}
	src := readArchiveCenterJS(t)
	script := extractArchiveCenterJSAsyncFunction(t, src, "resolveCurrentActiveChatObject") + "\n" +
		extractArchiveCenterJSAsyncFunction(t, src, "getCurrentActiveChatRollbackMessages") + `
const R = {
  getCurrentCharacterIndex: async () => 4,
  getCurrentChatIndex: async () => 2,
  getChatFromIndex: async () => ({message:[]}),
};
function parseSessionDisplayIdentity() { return null; }
function extractActiveChatRollbackMessages(chat) {
  return (chat && Array.isArray(chat.message) ? chat.message : []).map(item => ({role:item.role,content:item.data}));
}
function debugLog() {}
(async function() {
  const messages = await getCurrentActiveChatRollbackMessages();
  if (!Array.isArray(messages) || messages.length !== 0) {
    throw new Error("rollback used stale character cache instead of canonical empty chat: " + JSON.stringify(messages));
  }
})().catch(function(err) {
  console.error(err && err.stack || err);
  process.exit(1);
});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("canonical rollback chat JS runtime fixture failed: %v\n%s", err, out)
	}
}

func TestCurrentChatIndexFailureFallsBackOnlyToIdentityMatchedCurrentCharacter(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for current chat index fallback fixture")
		}
	}
	src := readArchiveCenterJS(t)
	script := extractArchiveCenterJSFunction(t, src, "resolveIdentityVerifiedCurrentCharacterChat") + "\n" +
		extractArchiveCenterJSFunction(t, src, "parseSessionDisplayIdentity") + "\n" +
		extractArchiveCenterJSAsyncFunction(t, src, "resolveCurrentActiveChatObject") + `
let getCharacterCalls = 0;
let activeChatId = "target";
const R = {
  getCurrentCharacterIndex: async () => 4,
  getCurrentChatIndex: async () => 2,
  getChatFromIndex: async () => { throw new Error("index read unavailable"); },
  getCharacter: async () => {
    getCharacterCalls += 1;
    return {chatPage: 2, chats: [{}, {}, {id:activeChatId, message:[{role:"assistant", data:"current output"}]}]};
  },
};
function debugLog() {}
(async function() {
  const resolved = await resolveCurrentActiveChatObject("char_4_cid_target");
  if (!resolved.chat || resolved.source !== "R.getCharacter.identity_verified") {
    throw new Error("identity-matched current chat was not recovered: " + JSON.stringify(resolved));
  }
  activeChatId = "different-chat";
  const mismatched = await resolveCurrentActiveChatObject("char_4_cid_target");
  if (mismatched.chat !== null || mismatched.source !== "none") {
    throw new Error("CID-mismatched current chat was reused: " + JSON.stringify(mismatched));
  }
  const wrongCharacter = await resolveCurrentActiveChatObject("char_9_cid_target");
  if (wrongCharacter.chat !== null || wrongCharacter.source !== "none") {
    throw new Error("character-mismatched current chat was reused: " + JSON.stringify(wrongCharacter));
  }
  if (getCharacterCalls !== 2) {
    throw new Error("getCharacter fallback call count=" + getCharacterCalls);
  }
})().catch(function(err) {
  console.error(err && err.stack || err);
  process.exit(1);
});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("current chat index fallback JS runtime fixture failed: %v\n%s", err, out)
	}
}

func TestNormalSessionFinalOutputRecoverySurvivesCurrentChatIndexReadFailure(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for normal-session final-output recovery fixture")
		}
	}
	src := readArchiveCenterJS(t)
	script := extractArchiveCenterJSFunction(t, src, "resolveIdentityVerifiedCurrentCharacterChat") + "\n" +
		extractArchiveCenterJSFunction(t, src, "parseSessionDisplayIdentity") + "\n" +
		extractArchiveCenterJSAsyncFunction(t, src, "resolveCurrentActiveChatObject") + "\n" +
		extractArchiveCenterJSAsyncFunction(t, src, "recoverAssistantContentFromActiveChat") + `
const R = {
  getCurrentCharacterIndex: async () => 4,
  getCurrentChatIndex: async () => 2,
  getChatFromIndex: async () => { throw new Error("transient index read failure"); },
  getCharacter: async () => ({
    chatPage: 2,
    chats: [{}, {}, {
      id: "target",
      message: [
        {role: "user", data: "normal user input"},
        {role: "assistant", data: "normal final assistant output"},
      ],
    }],
  }),
};
function extractActiveChatComparableMessages(chat) {
  return (chat && Array.isArray(chat.message) ? chat.message : []).map(function(item, index) {
    return {role: item.role, content: item.data, risuMessageIndex: index};
  });
}
function buildCompletedTurnPairsFromActiveChatMessages(messages) {
  return [{
    userContent: messages[0].content,
    assistantContent: messages[1].content,
  }];
}
function normalizeMainTurnCompareText(value) { return String(value || "").trim(); }
function mainTurnTextMatchesOriginal(left, right) { return normalizeMainTurnCompareText(left) === normalizeMainTurnCompareText(right); }
function normalizeAssistantPersistenceCandidate(value) { return String(value || "").trim(); }
function isAssistantPrefillSeedText() { return false; }
function getSessionSnapshot() { return null; }
function getLastNonEmptyAssistantComparableContent() { return ""; }
function isSameAssistantComparableText(left, right) { return left === right; }
function debugLog() {}
(async function() {
  const recovered = await recoverAssistantContentFromActiveChat(
    "char_4_cid_target",
    null,
    "normal user input"
  );
  if (recovered !== "normal final assistant output") {
    throw new Error("normal-session final output was not recovered: " + JSON.stringify(recovered));
  }
})().catch(function(err) {
  console.error(err && err.stack || err);
  process.exit(1);
});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("normal-session final-output recovery fixture failed: %v\n%s", err, out)
	}
}

func TestCopiedEightPlusOneDeletionKeepsImportedEightTurns(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for copied-session rollback fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functionBody := extractArchiveCenterJSAsyncFunction(t, src, "reconcileActiveChatTailDeletionWithBackend")
	script := functionBody + `
const settings = {enabled:true,dbEnabled:true};
const SESSION_FALLBACK = "default";
const ROLLBACK_TAIL_RECONCILE_INTERVAL_MS = 0;
const ROLLBACK_TAIL_RECONCILE_MAX_BLIND_GAP_TURNS = 2;
let _rollbackTailReconcileInFlight = false;
let _rollbackTailReconcileLastAt = 0;
let _lastAutoRollbackSignature = null;
const _pendingFinalConfirmations = new Map();
let rollbackFrom = 0;
function hasPendingFinalConfirmationForSession() { return false; }
function extractActiveChatMessageList(chat) { return chat.message; }
function extractActiveChatComparableMessages(chat) { return chat.message; }
function buildCompletedTurnPairsFromActiveChatMessages() { return []; }
async function requestBackendSessionRoutingTurnResolution() {
  return {completedTurnCount:8,baseline:{backendTurnAtRoute:8,localPairCountAtRoute:0,reason:"timeline_copy"}};
}
async function fetchBackendLatestTurnIndexForSession() { return 9; }
function buildLedgerVerifiedTailRollback() { return null; }
function getRecentRisuHistoryTrimGuard() { return null; }
function updateRuntimeState() {}
async function executeAutoRollback(_sid, turn) { rollbackFrom = turn; return true; }
function updateSessionSnapshot() {}
function debugLog() {}
(async function() {
  const ok = await reconcileActiveChatTailDeletionWithBackend("copy-target", {message:[]}, {force:true});
  if (!ok || rollbackFrom !== 9) {
    throw new Error("8+1 -> 8+0 must delete only turn 9: ok=" + ok + " rollbackFrom=" + rollbackFrom);
  }
})().catch(function(err) {
  console.error(err && err.stack || err);
  process.exit(1);
});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("copied-session rollback JS runtime fixture failed: %v\n%s", err, out)
	}
}

func TestPostprocessorReplacementRebuildsDeletionSnapshot(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for postprocessor snapshot fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functionBody := extractArchiveCenterJSAsyncFunction(t, src, "replacePersistedTurnWithPostOutputFinal")
	script := functionBody + `
let snapshotMessages = null;
function normalizeMainTurnCompareText(text) { return String(text || "").trim(); }
function normalizeAssistantPersistenceCandidate(text) { return String(text || "").trim(); }
function isSameAssistantComparableText(left, right) { return left === right; }
async function findRecentPersistedCompleteTurnPairForContent() { return {turnIndex:9,latestBackendTurn:9}; }
async function buildCompleteTurnRequestBody() { return {client_meta:{}}; }
function computeAssistantSnapshotFingerprint() { return "old"; }
function buildCompleteTurnQueuePayload() { return null; }
function enqueue() {}
async function flushQueueSave() {}
async function executeAutoRollback() { return true; }
async function tryCompleteTurn() { return {status:"ok",save_ok:true}; }
function removeQueuedItem() { return false; }
function trackTurnIndex() {}
async function getCurrentActiveChatComparableMessages() {
  return [{role:"user",content:"player input"},{role:"assistant",content:"final output"}];
}
function updateSessionSnapshot(_sid, messages) { snapshotMessages = messages; }
function upsertTimelineCompleteTurnPendingArtifacts() {}
function scheduleTimelinePostCompleteTurnRefresh() {}
(async function() {
  const result = await replacePersistedTurnWithPostOutputFinal("s", {
    postOutputReplacement:{userContent:"player input",assistantContent:"draft output",contextMessages:[]}
  }, "final output");
  if (!result || !result.replaced) throw new Error("postprocessor replacement did not complete");
  if (!Array.isArray(snapshotMessages) || snapshotMessages.length !== 2 || snapshotMessages[1].content !== "final output") {
    throw new Error("postprocessor replacement did not rebuild final deletion snapshot: " + JSON.stringify(snapshotMessages));
  }
})().catch(function(err) {
  console.error(err && err.stack || err);
  process.exit(1);
});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("postprocessor snapshot JS runtime fixture failed: %v\n%s", err, out)
	}
}

func TestAdapterLifecycleStateRuntimeContracts(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for adapter lifecycle runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functions := strings.Join([]string{
		extractArchiveCenterJSFunction(t, src, "addRawInputSessionKey"),
		extractArchiveCenterJSFunction(t, src, "buildRawInputSessionKeys"),
		extractArchiveCenterJSFunction(t, src, "cacheRawInputForSession"),
		extractArchiveCenterJSFunction(t, src, "peekRawInputForSession"),
		extractArchiveCenterJSFunction(t, src, "bindRawInputObservationToRequest"),
		extractArchiveCenterJSFunction(t, src, "isRisuHistoryTrimCommandText"),
		extractArchiveCenterJSAsyncFunction(t, src, "saveSettings"),
		extractArchiveCenterJSAsyncFunction(t, src, "refreshArchiveCenterUpdateStatus"),
		extractArchiveCenterJSFunction(t, src, "stopTurnWorkflowHUDWatch"),
		extractArchiveCenterJSFunction(t, src, "schedulePostOutputFinalReplacement"),
	}, "\n")
	script := `
const SESSION_FALLBACK = "default";
const RAW_INPUT_CACHE_MAX = 100;
const _rawInputBySession = new Map();
let _rawInputObservationSeq = 0;
let settings = {enabled:true};
const SETTINGS_KEY = "settings";
let persisted = [];
let syncAck = {ok:false,code:"backend_down"};
let runtimeStates = [];
async function persistentSet(key, value) { persisted.push({key,value}); }
async function syncConfigToBackend() { return syncAck; }
function markBackendRuntimeConfigDirty() {}
function updateRuntimeState(key, status, state) { runtimeStates.push({key,status,state}); }
function debugLog() {}
function warnLog() {}
const updateStatusEl = {innerHTML:""};
const document = {getElementById:function(id) { return id === "mo-update-status" ? updateStatusEl : null; }};
const archiveUpdateState = {lastStatus:null};
let updateFetchCalls = 0;
async function fetchArchiveCenterUpdateStatus() { updateFetchCalls++; return {status:"ok"}; }
function formatArchiveCenterUpdateStatus(data) { return data ? "ready" : "missing"; }
let _turnWorkflowHUDActiveRequestId = "active-request";
let _turnWorkflowHUDWatchToken = 10;
let _turnWorkflowHUDWatchRunning = true;
let _turnWorkflowHUDLastRevision = 0;
let hudCleanupCalls = 0;
function clearTurnWorkflowHUDTimer() { hudCleanupCalls++; }
function dismissTurnWorkflowHUD() {}
let replacementCalls = 0;
let panelOpen = false;
async function replacePersistedTurnWithPostOutputFinal(_sid, _skip, response) {
  replacementCalls++;
  if (response !== "final output") throw new Error("post-output response was not forwarded");
  return {replaced:true,turnIndex:3};
}
async function renderSettingsPanel() {}
function assert(condition, message) { if (!condition) throw new Error(message); }
` + "\n" + functions + `
(async function() {
  cacheRawInputForSession("char_7_chat_1", "first turn");
  const first = bindRawInputObservationToRequest("char_7_chat_1", "request-1");
  assert(first && first.text === "first turn" && first.boundRequestId === "request-1", "first raw observation was not bound");
  assert(peekRawInputForSession("char_7_chat_1") === null, "bound observation remained reusable");
  assert(bindRawInputObservationToRequest("char_7_chat_1", "request-2") === null, "second turn reused the first raw observation");
  cacheRawInputForSession("char_7_chat_1", "second turn");
  const second = bindRawInputObservationToRequest("char_7_chat_1", "request-2");
  assert(second && second.text === "second turn" && second.observationId !== first.observationId, "second turn did not require a new observation");
  assert(bindRawInputObservationToRequest("missing", "request-3") === null, "missing correlation synthesized a raw observation");

  const failedSave = await saveSettings();
  assert(failedSave === false, "backend sync failure was reported as a successful save");
  assert(persisted.length === 1, "local settings were not retained on backend sync failure");
  assert(runtimeStates.some(function(item) {
    return item.key === "lastConfigSync" && item.status === "fail" &&
      item.state && item.state.detail === "settings_saved_locally_backend_unsynced";
  }), "backend sync failure was not made visible");
  syncAck = {ok:true,code:"config_sync_ok"};
  assert(await saveSettings(), "successful backend sync was not acknowledged");

  const updateStatus = await refreshArchiveCenterUpdateStatus();
  assert(updateStatus && updateStatus.status === "ok" && updateFetchCalls === 1, "global updater helper did not fetch status");
  assert(updateStatusEl.innerHTML === "ready", "global updater helper did not update its DOM target");

  stopTurnWorkflowHUDWatch("different-request", true);
  assert(hudCleanupCalls === 1, "mismatched HUD stop did not clear elapsed UI work");
  assert(_turnWorkflowHUDWatchToken === 10, "mismatched HUD stop terminated the active watcher");

  schedulePostOutputFinalReplacement("s", {}, "final output");
  await Promise.resolve();
  await Promise.resolve();
  await Promise.resolve();
  assert(replacementCalls === 1, "post-output replacement did not run as a single microtask");
  process.stdout.write("ok");
})().catch(function(err) {
  console.error(err && err.stack || err);
  process.exit(1);
});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("adapter lifecycle JS runtime fixture failed: %v\n%s", err, out)
	}
	if strings.TrimSpace(string(out)) != "ok" {
		t.Fatalf("adapter lifecycle JS runtime fixture output=%q, want ok", out)
	}
}

func TestAdapterRecurringPathsAreExplicitOneShotRuntime(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for one-shot adapter runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functions := strings.Join([]string{
		extractArchiveCenterJSAsyncFunction(t, src, "referenceLibraryPollJob"),
		extractArchiveCenterJSFunction(t, src, "cancelAdminBackgroundJobStream"),
		extractArchiveCenterJSFunction(t, src, "markAdminBackgroundJobStreamUnavailable"),
		extractArchiveCenterJSFunction(t, src, "applyAdminBackgroundJobSnapshot"),
		extractArchiveCenterJSAsyncFunction(t, src, "pollAdminBackgroundJob"),
		extractArchiveCenterJSAsyncFunction(t, src, "reconcileRollbackFromHostSignal"),
	}, "\n")
	script := `
const _referenceLibraryState = {job:null};
let referenceCalls = 0;
let adminCalls = 0;
let explorerRefreshes = 0;
let rollbackReconcileCalls = 0;
let rollbackCheckCalls = 0;
let _rollbackHostSignalReconcileInFlight = false;
const _rollbackHostSignalLastSignatureBySession = new Map();
const _adminBackgroundJobStreams = new Map();
const settings = {enabled:true,rollbackAutoEnabled:true};
const R = {getCharacter:function() {}};
function referenceLibraryPath(value) { return encodeURIComponent(String(value || "")); }
function resolveRequestTimeoutMs() { return 19000; }
function getRequestTimeoutSettingMs() { return 19000; }
async function bridgeFetch(path, options) {
  if (options && options.timeoutMs !== 19000) throw new Error("one-shot request lost UI timeout");
  if (String(path).startsWith("/reference-jobs/")) {
    referenceCalls++;
    return {status:"running",kind:"reference_import"};
  }
  if (String(path).startsWith("/admin/jobs/")) {
    adminCalls++;
    return adminCalls === 1
      ? {job_id:"admin-1",status:"running",terminal:false}
      : {job_id:"admin-1",status:"completed",terminal:true,result:{done:true}};
  }
  throw new Error("unexpected path " + path);
}
function referenceLibraryRefreshUI() {}
function referenceLibrarySetStatus(status) {
  if (status !== "running") throw new Error("running reference job was misclassified");
}
async function referenceDiscoveryLoadLatestJob() {}
async function referenceLibraryLoadData() {}
async function referenceLibraryLoadVectorStatus() {}
async function safeCall(fn) { return await fn(); }
function refreshExplorerUI() { explorerRefreshes++; }
async function getCurrentChatSessionId() { return "session"; }
async function resolveCurrentActiveChatObject() {
  return {chat:{message:[{role:"user",content:"u"},{role:"assistant",content:"a"}]}};
}
function extractActiveChatRollbackMessages(chat) { return chat.message; }
function computeTailHash(messages) { return JSON.stringify(messages); }
async function reconcileActiveChatTailDeletionWithBackend() { rollbackReconcileCalls++; return false; }
async function checkAndAutoRollback() { rollbackCheckCalls++; }
function debugLog() {}
function setTimeout() { throw new Error("one-shot path scheduled a timer"); }
function setInterval() { throw new Error("one-shot path scheduled an interval"); }
function assert(condition, message) { if (!condition) throw new Error(message); }
` + "\n" + functions + `
(async function() {
  const referenceRunning = await referenceLibraryPollJob("ref-1");
  assert(referenceRunning && referenceRunning.status === "running" && referenceCalls === 1, "reference job call was not one-shot");
  await Promise.resolve();
  assert(referenceCalls === 1, "reference job scheduled recursive polling");
  await referenceLibraryPollJob("ref-1");
  assert(referenceCalls === 2, "explicit reference refresh did not make exactly one request");

  const adminState = {loading:true,error:null,result:null,job:{job_id:"admin-1"}};
  const adminRunning = await pollAdminBackgroundJob("reindex", adminState, "admin-1");
  assert(adminRunning && adminRunning.status === "running" && adminCalls === 1 && adminState.loading, "admin job call was not one-shot");
  await Promise.resolve();
  assert(adminCalls === 1, "admin job scheduled recursive polling");
  await pollAdminBackgroundJob("reindex", adminState, "admin-1");
  assert(adminCalls === 2 && adminState.loading === false && adminState.result.done, "explicit admin refresh did not consume terminal status");

  assert(await reconcileRollbackFromHostSignal(), "first host lifecycle signal did not reconcile rollback state");
  assert(rollbackReconcileCalls === 2 && rollbackCheckCalls === 1, "rollback signal did not run the expected bounded reconciliation");
  assert(!(await reconcileRollbackFromHostSignal()), "unchanged host signature reconciled more than once");
  assert(rollbackReconcileCalls === 2 && rollbackCheckCalls === 1, "unchanged rollback signature repeated backend work");
  process.stdout.write("ok");
})().catch(function(err) {
  console.error(err && err.stack || err);
  process.exit(1);
});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("adapter one-shot JS runtime fixture failed: %v\n%s", err, out)
	}
	if strings.TrimSpace(string(out)) != "ok" {
		t.Fatalf("adapter one-shot JS runtime fixture output=%q, want ok", out)
	}
}

func TestAdminBackgroundJobUsesSingleNDJSONStreamAndCleansUp(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for admin background job stream runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	openStream := extractArchiveCenterJSAsyncFunction(t, src, "openTurnWorkflowHUDStream")
	streamFailure := extractArchiveCenterJSFunction(t, src, "turnWorkflowHUDStreamFailure")
	cancelStream := extractArchiveCenterJSFunction(t, src, "cancelAdminBackgroundJobStream")
	cancelAll := extractArchiveCenterJSFunction(t, src, "cancelAllAdminBackgroundJobStreams")
	markUnavailable := extractArchiveCenterJSFunction(t, src, "markAdminBackgroundJobStreamUnavailable")
	applySnapshot := extractArchiveCenterJSFunction(t, src, "applyAdminBackgroundJobSnapshot")
	consumeStream := extractArchiveCenterJSAsyncFunction(t, src, "consumeAdminBackgroundJobStream")
	startStream := extractArchiveCenterJSFunction(t, src, "startAdminBackgroundJobStream")
	acceptJob := extractArchiveCenterJSFunction(t, src, "acceptAdminBackgroundJob")
	renderProgress := extractArchiveCenterJSFunction(t, src, "renderAdminJobProgressHtml")
	unload := extractArchiveCenterJSAsyncFunction(t, src, "removeRegisteredRisuHooksOnUnload")
	streamSource := strings.Join([]string{
		openStream, cancelStream, cancelAll, markUnavailable, applySnapshot, consumeStream, startStream, acceptJob,
	}, "\n")
	for _, forbidden := range []string{"setTimeout(", "setInterval(", "pollAdminBackgroundJob(", `bridgeFetch("/admin/jobs/`} {
		if strings.Contains(streamSource, forbidden) {
			t.Fatalf("admin job stream retained forbidden recurring transport %q", forbidden)
		}
	}
	for _, required := range []string{"/events?after_revision=", "openTurnWorkflowHUDStream", "job.terminal === true"} {
		if !strings.Contains(streamSource, required) {
			t.Fatalf("admin job stream missing %q", required)
		}
	}
	if !strings.Contains(renderProgress, "data-admin-job-transport-notice") ||
		!strings.Contains(renderProgress, "Refresh job status") ||
		!strings.Contains(renderProgress, "job.terminal !== true") ||
		!strings.Contains(unload, "cancelAllAdminBackgroundJobStreams") {
		t.Fatal("admin job stream lost typed transport notice, manual Refresh, or unload cleanup")
	}

	script := `
const settings = {bridgeUrl:"http://127.0.0.1:28080"};
const encoder = new TextEncoder();
const _adminBackgroundJobStreams = new Map();
let streamResponses = [];
let streamPaths = [];
let readerCancels = 0;
let controllerAborts = 0;
let observedState = null;
let observedStatuses = [];
class AbortController {
  constructor() { this.signal = {}; this.aborted = false; }
  abort() { if (!this.aborted) { this.aborted = true; controllerAborts++; } }
}
const R = {
  nativeFetch: async function(path) {
    streamPaths.push(String(path || ""));
    if (streamResponses.length === 0) throw new Error("unexpected extra admin stream connection");
    return streamResponses.shift();
  },
};
function resolveBridgeRuntimeRoute() { return {url:"http://127.0.0.1:28080"}; }
function refreshExplorerUI() {
  if (observedState && observedState.job) observedStatuses.push(String(observedState.job.status || ""));
}
function responseFromLines(lines) {
  const chunks = [encoder.encode(lines.join("\n") + "\n")];
  return {
    status:200,
    ok:true,
    body:{
      getReader:function() {
        return {
          read:async function() {
            if (chunks.length > 0) return {value:chunks.shift(),done:false};
            return {done:true};
          },
          cancel:async function() { readerCancels++; },
        };
      },
    },
  };
}
function hangingResponse() {
  let finishRead = null;
  return {
    status:200,
    ok:true,
    body:{
      getReader:function() {
        return {
          read:function() {
            return new Promise(function(resolve) { finishRead = resolve; });
          },
          cancel:async function() {
            readerCancels++;
            if (finishRead) finishRead({done:true});
          },
        };
      },
    },
  };
}
function assert(condition, message) { if (!condition) throw new Error(message); }
async function settle(predicate, label) {
  for (let index = 0; index < 40 && !predicate(); index++) {
    await new Promise(function(resolve) { setImmediate(resolve); });
  }
  assert(predicate(), label + " did not settle");
}
` + "\n" + streamFailure + "\n" + openStream + "\n" + cancelStream + "\n" + cancelAll +
		"\n" + markUnavailable + "\n" + applySnapshot + "\n" + consumeStream + "\n" + startStream + "\n" + acceptJob + `
(async function() {
  const completedState = {loading:false,error:null,result:null,job:null};
  observedState = completedState;
  streamResponses = [responseFromLines([
    JSON.stringify({contract_version:"admin_background_job.v1",job_id:"job-1",status:"running",revision:2,terminal:false,progress:{progress_percent:8}}),
    JSON.stringify({contract_version:"admin_background_job.v1",job_id:"job-1",status:"completed",revision:3,terminal:true,result:{done:true}}),
  ])];
  assert(acceptAdminBackgroundJob("session_normalize", completedState, {
    contract_version:"admin_background_job.v1",job_id:"job-1",status:"accepted",revision:1,terminal:false
  }), "accepted admin job was rejected");
  await settle(function() { return _adminBackgroundJobStreams.size === 0; }, "terminal stream");
  assert(streamPaths.length === 1, "admin job used more than one stream connection");
  assert(streamPaths[0].includes("/admin/jobs/job-1/events?after_revision=1"), "admin job used the wrong event route");
  assert(observedStatuses.join(",") === "running,completed", "running to completed revisions were not rendered in order");
  assert(completedState.loading === false && completedState.error === null && completedState.result.done, "terminal completion was not applied");
  assert(readerCancels === 1 && controllerAborts === 1, "terminal stream did not clean reader and controller");

  const authorityState = {loading:true,error:null,result:null,job:{job_id:"job-authority"}};
  applyAdminBackgroundJobSnapshot("reindex", authorityState, "job-authority", {
    job_id:"job-authority",status:"completed",revision:2,terminal:false,result:{done:true}
  });
  assert(authorityState.loading === true && authorityState.result === null, "status text bypassed backend terminal authority");

  const unsupportedState = {loading:false,error:null,result:null,job:null};
  observedState = unsupportedState;
  streamResponses = [{status:200,ok:true,body:null}];
  assert(acceptAdminBackgroundJob("reindex", unsupportedState, {
    job_id:"job-unsupported",status:"accepted",revision:1,terminal:false
  }), "unsupported transport job was rejected");
  await settle(function() { return _adminBackgroundJobStreams.size === 0; }, "unsupported stream");
  assert(unsupportedState.loading === true && unsupportedState.error === null, "unsupported stream was misclassified as a job failure");
  assert(
    unsupportedState.job.transport_notice &&
    unsupportedState.job.transport_notice.reason_code === "stream_transport_unavailable",
    "unsupported stream lost its typed transport notice"
  );
  applyAdminBackgroundJobSnapshot("reindex", unsupportedState, "job-unsupported", {
    job_id:"job-unsupported",status:"running",revision:2,terminal:false,progress:{progress_percent:12}
  });
  assert(
    unsupportedState.job.transport_notice &&
    unsupportedState.job.transport_notice.reason_code === "stream_transport_unavailable",
    "manual running refresh removed the typed transport notice"
  );

  const oldState = {loading:false,error:null,result:null,job:null};
  observedState = oldState;
  streamResponses = [hangingResponse()];
  acceptAdminBackgroundJob("rescan", oldState, {job_id:"job-old",status:"accepted",revision:1,terminal:false});
  await settle(function() {
    const active = _adminBackgroundJobStreams.get("rescan");
    return !!(active && active.reader);
  }, "old stream reader");
  const cancelsBeforeReplacement = readerCancels;
  const abortsBeforeReplacement = controllerAborts;
  const newState = {loading:false,error:null,result:null,job:null};
  observedState = newState;
  streamResponses = [responseFromLines([
    JSON.stringify({job_id:"job-new",status:"completed",revision:2,terminal:true,result:{done:true}})
  ])];
  acceptAdminBackgroundJob("rescan", newState, {job_id:"job-new",status:"accepted",revision:1,terminal:false});
  await settle(function() { return _adminBackgroundJobStreams.size === 0; }, "replacement stream");
  assert(readerCancels >= cancelsBeforeReplacement + 2, "same-kind replacement or terminal did not cancel readers");
  assert(controllerAborts >= abortsBeforeReplacement + 2, "same-kind replacement or terminal did not abort controllers");
  cancelAllAdminBackgroundJobStreams();
  process.stdout.write("ok");
})().catch(function(err) {
  console.error(err && err.stack || err);
  process.exit(1);
});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("admin background job stream runtime fixture failed: %v\n%s", err, out)
	}
	if strings.TrimSpace(string(out)) != "ok" {
		t.Fatalf("admin background job stream runtime fixture output=%q, want ok", out)
	}
}

func TestAdapterTimerAndConfigAcknowledgementSourceContract(t *testing.T) {
	src := readArchiveCenterJS(t)
	if got := strings.Count(src, "setTimeout("); got != 2 {
		t.Fatalf("production adapter setTimeout count=%d, want the UI-configured fetch abort and HUD elapsed timers", got)
	}
	if strings.Contains(src, "setInterval(") || strings.Contains(src, "clearInterval(") {
		t.Fatal("production adapter still contains a fixed interval")
	}
	for _, forbidden := range []string{
		"resolvePostOutputFinalAssistant",
		"rollbackIdleWatcherMode",
		"FAILED_QUEUE_SAVE_DEBOUNCE_MS",
		"persistenceOrchResult._rawInputObservation",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("production adapter retained removed recurring policy %q", forbidden)
		}
	}
	for _, required := range []string{
		`const transportAccepted = !!(result && result.status === "ok");`,
		`const runtimeSynced = !!(trace && trace.synced === true);`,
		`? "config_sync_ok"`,
		`settings_saved_locally_backend_unsynced`,
		`String(pendingRawInputObservation.boundRequestId || "") === pendingRequestId`,
		`setTimeout(function turnWorkflowHUDElapsedFrame()`,
		`clearTimeout(_turnWorkflowHUDElapsedTimer)`,
	} {
		if !strings.Contains(src, required) {
			t.Fatalf("production adapter missing lifecycle/config contract marker %q", required)
		}
	}
}

func TestRuntimeTimeoutResolversPreserveExplicitUIValues(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for timeout resolver runtime fixture")
		}
	}
	src := readArchiveCenterJS(t)
	functions := strings.Join([]string{
		extractArchiveCenterJSFunction(t, src, "resolveRequestTimeoutMs"),
		extractArchiveCenterJSFunction(t, src, "getCompleteTurnTimeoutMs"),
		extractArchiveCenterJSFunction(t, src, "resolvePluginMainTimeoutMs"),
	}, "\n")
	script := `
function getRequestTimeoutSettingMs() { return 800000; }
function getCriticTimeoutMs() { return 900000; }
function getEmbeddingTimeoutMs() { return 200000; }
function getPluginMainTimeoutSettingMs() { return 450000; }
function assert(condition, message) { if (!condition) throw new Error(message); }
` + "\n" + functions + `
assert(resolveRequestTimeoutMs(700000) === 700000, "request override was silently clamped");
assert(resolveRequestTimeoutMs(0) === 0, "backend-owned wait was replaced by the UI request timeout");
assert(resolveRequestTimeoutMs(-1) === 800000, "invalid request override did not use the UI setting");
assert(getCompleteTurnTimeoutMs() === 1100000, "derived complete-turn timeout was silently clamped");
assert(resolvePluginMainTimeoutMs(500000) === 500000, "plugin-main override was silently clamped");
assert(resolvePluginMainTimeoutMs(NaN) === 450000, "invalid plugin-main override did not use the UI setting");
process.stdout.write("ok");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("timeout resolver JS runtime fixture failed: %v\n%s", err, out)
	}
	if strings.TrimSpace(string(out)) != "ok" {
		t.Fatalf("timeout resolver JS runtime fixture output=%q, want ok", out)
	}
	for _, forbidden := range []string{
		"sanitizeNumber(overrideMs, getRequestTimeoutSettingMs(), 1, 600000)",
		"sanitizeNumber(Math.max(base, critic + embedding), base, 1000, 600000)",
		"sanitizeNumber(overrideMs, getPluginMainTimeoutSettingMs(), 1, 300000)",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("runtime timeout helper retained hidden clamp %q", forbidden)
		}
	}
}
