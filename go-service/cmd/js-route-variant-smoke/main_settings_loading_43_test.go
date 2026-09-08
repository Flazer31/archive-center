package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestSettings43OpensBeforeBackendStatusAndPreservesEdits(t *testing.T) {
	src := readArchiveCenterJS(t)
	functions := strings.Join([]string{
		extractArchiveCenterJSAsyncFunction(t, src, "renderSettingsPanel"),
		extractArchiveCenterJSAsyncFunction(t, src, "loadDashboardViewModel"),
	}, "\n")
	script := `
const assert = require('node:assert/strict');
const nodes = new Map(), pending = [], failures = [];
let shown = 0, attached = 0, panelOpen = false, _settingsPanelRenderRequestId = 0;
let _settingsActiveTab = 'settings';
const settings = {bridgeUrl:'http://100.96.60.55:28080',memoryDeliveryBudgets:{}};
const getSettings = () => settings, DEFAULT_SETTINGS = settings;
const runtimeState = {queuePersistence:{}}, _settingsStorageStatus = {}, _turnHistory = [], _failedQueue = [];
const _timelineState = {}, lastTurnTrace = null, _prepareTurnEverContacted = false, _turnWorkflowHUDActiveRequestId = '';
const PANEL_CSS = '', LOG_PREFIX = 'test', VERSION = 'test', BUILD_LABEL = '', BUILD_CHANNEL = '', BUILD_TIME = '', BUILD_NOTES = '';
const R = {showContainer:async () => { shown++; }};
const t = x => x, escapeAttr = x => String(x ?? ''), debugLog = () => {};
const warnLog = (...args) => failures.push(args.join(' '));
const resolveEffectiveCriticConfig = () => ({}), endpointSummary = () => '';
const getRequestTimeoutSettingMs = () => 15000, buildDashboardQueueObservations = () => ({});
const renderDashboardViewModel = vm => vm ? vm.html : 'unavailable';
const renderEffectiveInputSection = () => '', renderPromptEditorSection = () => '', renderBridgeRuntimeNotice = () => '';
const formatStateRow = () => '', renderStep17InspectionRolesSection = () => '', renderStep17VisibilitySection = () => '';
const renderStep17ReleaseGateSection = () => '', renderCriticLedgerProbeDebugSection = () => '';
const renderLorebookSelectionDiagnostics = () => '', renderProviderCallBudgetLedgers = () => '', renderTurnTraceRows = () => '';
const renderActivitySection = () => '', renderTurnHistorySection = () => '', renderPreviewSection = () => '';
const renderInputTransparencySection = () => '', renderAuditSection = () => '', renderCompareSection = () => '';
const attachSettingsEvents = () => { attached++; };
async function bridgeFetch(path) {
  assert.equal(path, '/dashboard/view-model');
  return new Promise(resolve => pending.push({resolve,url:settings.bridgeUrl}));
}
// DOM and unrelated subsection renderers are boundaries; execute the full production panel owner.
function element() {
  return {
    children:new Map(), setAttribute(){}, textContent:'',
    set innerHTML(value) {
      this.html = value;
      if (this.id === 'mo-settings-overlay') {
        assert.match(value, /id="mo-bridgeUrl"/);
        this.children.set('#mo-dashboard', {innerHTML:'loading'});
        this.children.set('#mo-bridgeUrl', {value:settings.bridgeUrl});
      }
    },
    get innerHTML() { return this.html; },
    querySelector(selector) { return this.children.get(selector) || null; },
    remove() { nodes.delete(this.id); }
  };
}
const document = {
  getElementById:id => nodes.get(id) || null,
  createElement:element,
  head:{appendChild:node => nodes.set(node.id,node)},
  body:{appendChild:node => nodes.set(node.id,node)}
};
` + functions + `
(async () => {
  const opening = renderSettingsPanel();
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(failures.length, 0, failures.join('\n'));
  assert.equal(shown, 1, 'settings must open while the backend has not responded');
  assert.equal(attached, 1, 'connection controls must be bound before the response');
  await opening;
  const first = nodes.get('mo-settings-overlay');
  first.querySelector('#mo-bridgeUrl').value = 'https://my-backend.example';
  pending[0].resolve({html:'first status'});
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(first.querySelector('#mo-dashboard').innerHTML, 'first status');
  assert.equal(first.querySelector('#mo-bridgeUrl').value, 'https://my-backend.example');
  assert.equal(nodes.get('mo-settings-overlay'), first, 'status refresh replaced the editable form');

  await renderSettingsPanel({recompose:true});
  const second = nodes.get('mo-settings-overlay');
  settings.bridgeUrl = 'https://saved-backend.example';
  await renderSettingsPanel({recompose:true});
  const third = nodes.get('mo-settings-overlay');
  pending[1].resolve({html:'stale status'});
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(second.querySelector('#mo-dashboard').innerHTML, 'loading');
  assert.equal(third.querySelector('#mo-dashboard').innerHTML, 'loading');
  pending[2].resolve(null);
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(third.querySelector('#mo-dashboard').innerHTML, 'unavailable');
  assert.equal(third.querySelector('#mo-bridgeUrl').value, settings.bridgeUrl);
  assert.equal(pending[2].url, settings.bridgeUrl);
  assert.equal(failures.length, 0, failures.join('\n'));
})().catch(err => { console.error(err); process.exitCode = 1; });
`
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		nodePath = "node"
	}
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("settings loading regression: %v\n%s", err, out)
	}
}
