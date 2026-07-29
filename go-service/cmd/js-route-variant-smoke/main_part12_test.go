package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestSessionRouteAdapterUsesOfficialStableHostIdentityAndReadback(t *testing.T) {
	src := readArchiveCenterJS(t)
	for _, needle := range []string{
		`typeof char.chaId === "string"`,
		`identity.stableCharacterId = char.chaId.trim()`,
		`identity.chatUniqueId = String(activeChat.id || "").trim()`,
		`stable_character_id: String(observed.stableCharacterId`,
		`host_chat_id: String(observed.hostChatId`,
		`binding_acknowledged`,
		`session_pin_readback_mismatch`,
		`legacy_index_fallback`,
	} {
		if !strings.Contains(src, needle) {
			t.Errorf("session route adapter missing %q", needle)
		}
	}
}

func TestSessionRouteBindingOrPinFailureCannotCacheCanonicalRoute(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for session route acknowledgement fixture")
		}
	}
	src := readArchiveCenterJS(t)
	ackFailureHelper := extractArchiveCenterJSSyncFunction(t, src, "isSessionRouteAcknowledgementFailure")
	resolver := extractArchiveCenterJSAsyncFunction(t, src, "resolveCanonicalWriteSessionId")
	script := `
const SESSION_FALLBACK = "default";
const SESSION_WRITE_CANONICAL_CACHE_MS = 10000;
const settings = {enabled:true,dbEnabled:true};
const R = {
  async getCurrentChatIndex(){ return 8; },
  async getCurrentCharacterIndex(){ return 3; },
};
function normalizeSessionId(value){ return String(value||"").trim(); }
async function getActiveChatSessionIdentity(){
  return {
    stableCharacterId:"stable-character",
    stableCharacterIdState:"observed",
    chatUniqueId:"opaque-chat",
    latestUserHash:"user-hash",
    latestAssistantHash:"assistant-hash",
    completedTurnCount:4,
    messageCount:8,
    isFreshChat:false,
  };
}
let bindingAcknowledged = false;
let pinResult = true;
let pinCalls = 0;
async function requestBackendSessionRoutingTurnResolution(){
  return {
    canonicalSessionId:"canonical-session",
    bindingAcknowledged,
    identityResolution:"durable_binding_existing",
  };
}
async function savePinnedSessionId(){ pinCalls++; return pinResult; }
let _sessionCache = {charIdx:null,chatIdx:null,sessionId:null,stableCharacterId:"",observedChatUniqueId:""};
let _sessionWriteCanonicalCache = {cacheKey:"",sessionId:"",rawSessionId:"",reason:"",cachedAt:0};
function updateRuntimeState(){}
function warnLog(){}
function isCidSessionId(){ return false; }
function buildCompatSessionReadPlan(){ throw new Error("legacy path must not run"); }
` + ackFailureHelper + "\n" + resolver + `
(async()=>{
  const pristineSessionCache = JSON.stringify(_sessionCache);
  const pristineWriteCache = JSON.stringify(_sessionWriteCanonicalCache);
  let failed = false;
  try { await resolveCanonicalWriteSessionId("requested-session"); } catch (err) {
    failed = err && err.message === "session_route_binding_readback_unverified";
  }
  if (!failed) throw new Error("binding failure did not propagate");
  if (pinCalls !== 0) throw new Error("pin write ran after binding failure");
  if (JSON.stringify(_sessionCache) !== pristineSessionCache || JSON.stringify(_sessionWriteCanonicalCache) !== pristineWriteCache) {
    throw new Error("binding failure mutated route cache");
  }

  bindingAcknowledged = true;
  pinResult = false;
  failed = false;
  try { await resolveCanonicalWriteSessionId("requested-session"); } catch (err) {
    failed = err && err.message === "session_pin_readback_unverified";
  }
  if (!failed) throw new Error("pin readback failure did not propagate");
  if (pinCalls !== 1) throw new Error("unexpected pin call count: "+pinCalls);
  if (JSON.stringify(_sessionCache) !== pristineSessionCache || JSON.stringify(_sessionWriteCanonicalCache) !== pristineWriteCache) {
    throw new Error("pin failure mutated route cache");
  }
})().catch(err=>{ console.error(err); process.exitCode=1; });
`
	cmd := exec.Command(nodePath, "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("session route acknowledgement fixture failed: %v\n%s", err, out)
	}
}

func TestDurableSessionPinRequiresExactReadbackAndUsesV3Record(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for durable pin readback fixture")
		}
	}
	src := readArchiveCenterJS(t)
	makeKey := extractArchiveCenterJSSyncFunction(t, src, "makeSessionPinKey")
	parseRecord := extractArchiveCenterJSSyncFunction(t, src, "parseSessionPinRecord")
	savePin := extractArchiveCenterJSAsyncFunction(t, src, "savePinnedSessionId")
	script := `
const PLUGIN_ID = "archive";
const SESSION_ID_PIN_PREFIX = PLUGIN_ID+"_session_id_pin_v1";
const SESSION_DURABLE_PIN_PREFIX = PLUGIN_ID+"_session_id_pin_v3";
const SESSION_PIN_RECORD_VERSION = "v3";
const SESSION_FALLBACK = "default";
let savedKey = "";
let savedPayload = "";
async function persistentSet(key,payload){ savedKey=key; savedPayload=payload; }
async function persistentGet(){ return JSON.stringify({version:"v3",sessionId:"wrong-session",observedChatUniqueId:"opaque-chat",stableCharacterId:"stable-character"}); }
function warnLog(){}
` + makeKey + "\n" + parseRecord + "\n" + savePin + `
(async()=>{
  const ok = await savePinnedSessionId(9,4,"canonical-session","opaque-chat","stable-character");
  if (ok !== false) throw new Error("mismatched pin readback was accepted");
  if (!savedKey.includes("session_id_pin_v3_character_stable-character_chat_opaque-chat")) {
    throw new Error("durable host identity key not used: "+savedKey);
  }
  const payload = JSON.parse(savedPayload);
  if (payload.version !== "v3" || payload.stableCharacterId !== "stable-character") {
    throw new Error("pin record version/identity mismatch");
  }
})().catch(err=>{ console.error(err); process.exitCode=1; });
`
	cmd := exec.Command(nodePath, "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("durable pin readback fixture failed: %v\n%s", err, out)
	}
}

func TestSessionRouteAcknowledgementFailureCannotReachTurnSavePayload(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for session route save-block fixture")
		}
	}
	src := readArchiveCenterJS(t)
	saveTurn := extractArchiveCenterJSAsyncFunction(t, src, "saveTurnToBackend")
	script := `
const settings = {enabled:true,dbEnabled:true};
let bridgeCalls = 0;
let queued = 0;
async function resolveCanonicalWriteSessionId(){ throw new Error("session_route_binding_readback_unverified"); }
async function getCurrentChatSessionId(){ throw new Error("must not run"); }
function sanitizeForCritic(v){ return v; }
function shouldSkipUserInputPersistence(){ return false; }
async function bridgeFetchWithRetry(){ bridgeCalls++; return {status:"ok"}; }
async function safeCall(fn){ return await fn(); }
function enqueue(){ queued++; }
function updateRuntimeState(){}
function debugLog(){}
function warnLog(){}
function trackTurnIndex(){}
` + saveTurn + `
(async()=>{
  let failed = false;
  try { await saveTurnToBackend(4,"user","assistant","requested-session"); } catch (err) {
    failed = err && err.message === "session_route_binding_readback_unverified";
  }
  if (!failed) throw new Error("unacknowledged route did not block save");
  if (bridgeCalls !== 0 || queued !== 0) throw new Error("save payload or queue reached after route failure");
})().catch(err=>{ console.error(err); process.exitCode=1; });
`
	cmd := exec.Command(nodePath, "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("session route save-block fixture failed: %v\n%s", err, out)
	}
}

func extractArchiveCenterJSSyncFunction(t *testing.T, src, name string) string {
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
	return strings.TrimSpace(src[start : start+len(marker)+next])
}
