package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestFeedback43HUDListenersUseRegisteringSafeElement(t *testing.T) {
	src := readArchiveCenterJS(t)
	functions := strings.Join([]string{
		extractArchiveCenterJSFunction(t, src, "takeTurnWorkflowHUDDismissListenerIds"),
		extractArchiveCenterJSAsyncFunction(t, src, "removeTurnWorkflowHUDDismissListeners"),
		extractArchiveCenterJSAsyncFunction(t, src, "attachTurnWorkflowHUDDismiss"),
	}, "\n")
	script := `
const assert = require('node:assert/strict');
const R = {}; // PocketRisu v1.11.2 has no global removeRisuEventListener.
let _turnWorkflowHUDDismissListenerIds = [];
let _turnWorkflowHUDUnloaded = false;
const handlers = new Map();
let serial = 0, rectReads = 0, dismissals = 0;
const errors = [];
const debugLog = (...args) => errors.push(args);
async function dismissTurnWorkflowHUD() { dismissals++; }
function target() {
  const own = new Map();
  return {
    async addEventListener(type, callback) {
      const id = ++serial;
      own.set(id, {type, callback}); handlers.set(id, callback);
      return id;
    },
    async removeEventListener(type, id) {
      assert.equal(own.get(id)?.type, type, 'cleanup used another SafeElement');
      own.delete(id); handlers.delete(id);
    },
    async getBoundingClientRect() { rectReads++; return {left:0,top:0,right:10,bottom:10}; },
    async querySelectorAll(selector) { assert.equal(selector, 'details[open]'); return {length:async()=>0}; }
  };
}
` + functions + `
(async () => {
  for (let i=0; i<100; i++) {
    await removeTurnWorkflowHUDDismissListeners();
    await attachTurnWorkflowHUDDismiss(target(), 'request', false);
    assert.equal(handlers.size, 1, 'terminal HUD replacement leaked a listener');
  }
  await Promise.all([...handlers.values()].map(fn => fn({clientX:100,clientY:100})));
  assert.equal(rectReads, 1, 'outside click queried removed HUD targets');
  await Promise.all([...handlers.values()].map(fn => fn({clientX:5,clientY:5})));
  assert.equal(dismissals, 1);
  await removeTurnWorkflowHUDDismissListeners(takeTurnWorkflowHUDDismissListenerIds());
  assert.equal(handlers.size, 0);
  const late = target();
  const register = late.addEventListener;
  late.addEventListener = async (...args) => {
    const id = await register(...args); _turnWorkflowHUDUnloaded = true; return id;
  };
  await attachTurnWorkflowHUDDismiss(late, 'late', false);
  assert.equal(handlers.size, 0, 'unload during registration leaked its listener');
  assert.equal(errors.length, 0);
  _turnWorkflowHUDUnloaded = false;
  const retry = target();
  const remove = retry.removeEventListener;
  let failOnce = true;
  retry.removeEventListener = async (...args) => {
    if (failOnce) { failOnce = false; throw new Error('transient host error'); }
    return remove(...args);
  };
  await attachTurnWorkflowHUDDismiss(retry, 'retry', false);
  await removeTurnWorkflowHUDDismissListeners();
  assert.equal(handlers.size, 1);
  assert.equal(_turnWorkflowHUDDismissListenerIds.length, 1, 'failed cleanup lost the registering owner');
  assert.equal(errors.length, 1);
  await removeTurnWorkflowHUDDismissListeners();
  assert.equal(handlers.size, 0, 'next existing cleanup did not remove the retained listener');
  assert.equal(_turnWorkflowHUDDismissListenerIds.length, 0);
})().catch(err => { console.error(err); process.exitCode=1; });
`
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		nodePath = "node"
	}
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("production HUD fixture: %v\n%s", err, output)
	}
}

func TestFeedback43HUDRecoveryRefreshesStaleView(t *testing.T) {
	src := readArchiveCenterJS(t)
	script := `
const assert = require('node:assert/strict');
const TURN_WORKFLOW_HUD_CONTRACT = 'turn_workflow_hud.v3';
const TURN_WORKFLOW_HUD_RECOVERY_REQUEST_CONTRACT = 'turn_workflow_hud_recovery_request.v1';
let _turnWorkflowHUDActiveRequestId = 'request';
let _turnWorkflowHUDTerminalRequestId = 'request';
const _lastBridgeFailureByPath = new Map();
const t = x => x, tf = x => x;
const confirm = () => true;
const calls = [], rendered = [], dismissed = [];
const classifyTurnWorkflowHUDTransportFailure = () => ({});
const rememberTurnWorkflowHUDHostWarning = () => {};
const renderTurnWorkflowHUD = async view => rendered.push(view);
const dismissTurnWorkflowHUD = async id => dismissed.push(id);
const stopTurnWorkflowHUDWatch = () => {}, startTurnWorkflowHUDWatch = () => {};
const stale = {contract_version:TURN_WORKFLOW_HUD_CONTRACT, request_id:'request', logical_turn:3, status:'failed'};
let latest = {...stale, status:'invalidated'};
let body = {code:'recovery_action_unavailable', error:'unavailable'};
let changeRequestOnStatus = false;
async function bridgeFetch(path, options) {
  calls.push({path,options});
  if (path === '/turn-workflow/recovery') {
    _lastBridgeFailureByPath.set(path,{response_body:JSON.stringify(body)});
    throw new Error('HTTP error');
  }
  assert.equal(path, '/turn-workflow/status?request_id=request');
  if (changeRequestOnStatus) _turnWorkflowHUDActiveRequestId = 'new-request';
  return latest;
}
` + extractArchiveCenterJSAsyncFunction(t, src, "requestTurnWorkflowHUDRecovery") + `
(async () => {
  const action = {id:'retry_derived_turn'};
  await requestTurnWorkflowHUDRecovery(stale, action);
  assert.equal(calls.length, 2, 'stale recovery must refresh status once');
  assert.deepEqual(rendered, [latest], 'must not re-render the captured failed snapshot');
  calls.length=0; rendered.length=0;
  body = {...body, turn_workflow_hud:latest};
  await requestTurnWorkflowHUDRecovery(stale, action);
  assert.equal(calls.length, 1, 'backend snapshot should avoid another status request');
  assert.deepEqual(rendered, [latest]);
  calls.length=0; rendered.length=0;
  body = {code:'unknown_workflow', error:'not found'};
  latest = {contract_version:TURN_WORKFLOW_HUD_CONTRACT, request_id:'request', status:'unknown'};
  await requestTurnWorkflowHUDRecovery(stale, action);
  assert.deepEqual(dismissed, ['request'], 'expired backend workflow must release its obsolete card');
  assert.equal(rendered.length, 0);
  dismissed.length=0;
  changeRequestOnStatus = true;
  await requestTurnWorkflowHUDRecovery(stale, action);
  assert.equal(dismissed.length, 0, 'late recovery response dismissed another request');
  assert.equal(rendered.length, 0, 'late recovery response replaced another request');
})().catch(err => { console.error(err); process.exitCode=1; });
`
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		nodePath = "node"
	}
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("production recovery fixture: %v\n%s", err, output)
	}
}
