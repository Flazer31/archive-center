package main

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestArchiveCenterJSCriticLedgerDebugRendererIsDefined(t *testing.T) {
	src := readArchiveCenterJS(t)
	for _, marker := range []string{
		"function renderCriticLedgerProbeDebugSection()",
		"${renderCriticLedgerProbeDebugSection()}",
		`data-critic-ledger-probe="1"`,
	} {
		if !strings.Contains(src, marker) {
			t.Fatalf("Archive Center.js missing critic ledger debug renderer marker %q", marker)
		}
	}
}

func TestArchiveCenterJSReferenceLibraryUIMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	for _, marker := range []string{
		`body: { auto_review: false, client_meta:`,
		`/library`,
		`data-reference-panel="library"`,
		`data-reference-panel="import"`,
		`data-reference-library-view="all"`,
		`data-reference-library-view="timeline"`,
		`data-reference-library-view="character"`,
		`data-reference-library-view="location"`,
		`data-reference-library-view="item"`,
		`data-reference-library-view="faction"`,
		`data-reference-library-view="claims"`,
		`data-reference-library-view="excluded"`,
		`data-reference-edit=`,
		`data-reference-exclude=`,
		`data-reference-restore=`,
		`id="mo-reference-timeline-normalize"`,
		`/library/timeline/normalize`,
		`생성된 자료`,
		`평론가 자동 생성`,
		`최신 소스 백엔드를 재시작하세요`,
	} {
		if !strings.Contains(src, marker) {
			t.Fatalf("Archive Center.js missing reference library marker %q", marker)
		}
	}
}

func TestArchiveCenterJSCanonPackAndDiscoveryUIMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	for _, marker := range []string{
		`data-reference-panel="canon"`,
		`data-tab="reference">📚 원작 자료</button>`,
		`/canon-registry/v1?q=`,
		`/canon-packs/preview/v1`,
		`/canon-packs/install/v1`,
		`/diagnostics/v1`,
		`/lifecycle/v1`,
		`/source-discovery/preview/v1`,
		`/source-discovery/jobs/v1`,
		`rawBody: true`,
		`headers: { "Content-Type": "application/zip" }`,
		`찾은 내용은 검토 대기로 저장되며 승인 전까지 원작 검색에 사용되지 않습니다.`,
		`공개 자료 주소로 직접 찾기`,
		`if (!draft.sourceUrl)`,
		`if (preview.search_provider_required)`,
		`const searchDiagnostic = result.search_llm || {};`,
		`const discoverySearch = discoveryResult.search_llm`,
		`allowed_source_types: draft.sourceUrl`,
		`["community_wiki"]`,
		`body.work_id = state.selectedWorkId;`,
		`body.continuity_id = state.selectedContinuityId;`,
		`const data = await bridgeFetch("/source-discovery/jobs/v1", {`,
		`/admit/v1`,
		`confirm_evidence_validated_batch: true`,
		`max_completion_tokens: getSubLlmMaxCompletionTokensSetting(settings.subLlmMaxCompletionTokens)`,
		`id="mo-discovery-admit"`,
		`discoveryCandidates.length === 0`,
		`searchDiagnostic.status === "completed_no_results"`,
		`discoverySearch.status || "") === "completed_no_results"`,
		`sourceSearchPlannerTemperature: sanitizeNumber(`,
		`sourceSearchPlannerReasoningEffort: normalizeReasoningEffort(`,
		`id="mo-sourceSearchPlannerTemperature"`,
		`id="mo-sourceSearchPlannerReasoningEffort"`,
		`id="mo-sourceSearchPlannerMaxCompletionTokens" value="' + Number(s.sourceSearchPlannerMaxCompletionTokens ?? 512) + '" min="1" max="128000"`,
		`<div class="mo-section">원작 자료 검색</div>`,
		`선택한 Provider의 웹 검색 기능으로 공개 출처를 찾습니다.`,
		`>Ollama Search Agent</option>`,
		`ollama: { endpoint: "https://ollama.com"`,
		`generationOptions.style.display = ""`,
		`Ollama 검색 에이전트 · effort none은 think=false`,
		`const discoveryExceptions = Array.isArray(discoveryResult.exceptions)`,
		`' · 검색 URL ' + Number(discoverySearch.result_count || 0)`,
		`'개 · 수집 실패 ' + discoveryExceptions.length`,
		`data-reference-panel="search_settings">검색 설정</button>`,
		`sourceSearchPlannerProvider: readValue("mo-sourceSearchPlannerProvider", settings.sourceSearchPlannerProvider, true)`,
		`id="mo-reference-work-edit"`,
		`id="mo-reference-work-delete"`,
		`expected_revision: Number(work.revision)`,
		`장기 기억은 변경하지 않았습니다.`,
		`<strong>선택한 원작 DB:</strong>`,
		`현재 실행 중인 백엔드에 작품 삭제 API가 없습니다.`,
		`온라인 팩 카탈로그가 아니라 이 PC에 설치된 Canon Pack을 검색합니다.`,
	} {
		if !strings.Contains(src, marker) {
			t.Fatalf("Archive Center.js missing Canon Pack UI marker %q", marker)
		}
	}
	if count := strings.Count(src, `<div class="mo-section">원작 자료 검색</div>`); count != 1 {
		t.Fatalf("source-search LLM settings panel count = %d, want 1", count)
	}
	previewStart := strings.Index(src, `if (!draft.sourceUrl) {`)
	if previewStart < 0 {
		t.Fatal("title-only Source Discovery preview guard missing")
	}
	previewEnd := strings.Index(src[previewStart:], `body.client_meta = buildAdminRuntimeClientMeta`)
	if previewEnd < 0 || !strings.Contains(src[previewStart:previewStart+previewEnd], `if (preview.search_provider_required)`) {
		t.Fatal("title-only Source Discovery must stop only when the backend reports a missing search provider")
	}
	for _, removed := range []string{
		`mo-discovery-original-title`,
		`mo-discovery-language`,
		`mo-discovery-edition`,
		`mo-discovery-requested-domains`,
		`mo-discovery-approved-domains`,
		`mo-discovery-provider-endpoint`,
		`mo-discovery-provider-api-key`,
		`mo-discovery-policy-confirmed`,
		`mo-sourceSearchProvider`,
		`mo-sourceSearchApiKey`,
		`<option value="brave"`,
		`<option value="tavily"`,
		`<div class="mo-section">원작 자료 검색 API</div>`,
		`<div class="mo-section">검색 계획 LLM</div>`,
		`["reference", "원작 자료"]`,
		`id="mo-source-search-llm-save"`,
		`renderReferenceBindingPanel(selector)`,
		`data.result && data.result.search_provider`,
		`찾은 자료는 검토 대기 상태로 저장했습니다.`,
	} {
		if strings.Contains(src, removed) {
			t.Fatalf("Archive Center.js still exposes removed Source Discovery field %q", removed)
		}
	}
	referencePanelStart := strings.Index(src, `data-tab-panel="reference"`)
	if referencePanelStart < 0 {
		t.Fatal("top-level reference panel missing")
	}
	referencePanelEnd := strings.Index(src[referencePanelStart:], `data-tab-panel="archive"`)
	if referencePanelEnd < 0 || strings.Contains(src[referencePanelStart:referencePanelStart+referencePanelEnd], `settingsSubtabsHtml("reference")`) {
		t.Fatal("reference workspace must not render the settings subtab bar")
	}
}

func TestArchiveCenterJSConsumesReferenceLaneOutsideMainInjectionBudget(t *testing.T) {
	src := readArchiveCenterJS(t)
	for _, marker := range []string{
		`reference_injection_budget_basis_chars: settings.maxInjectionChars || DEFAULT_SETTINGS.maxInjectionChars,`,
		`reference_recall_limit: sanitizeTopKSetting(settings.topK, DEFAULT_SETTINGS.topK),`,
		`reference_injection_enabled: settings.injectionEnabled !== false,`,
		`payloadApplicationPlan: result.payload_application_plan`,
		`const auxiliaryText = String(plan.auxiliary_text || "")`,
		`injectAuxiliaryBlock(finalPayload, auxiliaryText)`,
		`referenceIncluded: !!(laneByKey.original_work && laneByKey.original_work.applied)`,
	} {
		if !strings.Contains(src, marker) {
			t.Fatalf("Archive Center.js missing primary Canon Base host-consumption marker %q", marker)
		}
	}
	if strings.Contains(src, "await runSupervisor(") {
		t.Fatal("host adapter must not create a second supervisor path")
	}
}

func TestArchiveCenterJSEffectiveInputRendersMainAndReferencePreviewsSeparately(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("node is required for Effective Input preview runtime smoke")
		}
	}
	src := readArchiveCenterJS(t)
	script := extractJSFunctionBlockForTest(t, src, "function renderEffectiveInputSection()") + `
const assert = (condition, message) => { if (!condition) throw new Error(message); };
let currentTrace = null;
let _effectiveInputAwaitingNewTurn = false;
let lastOrchResult = null;
function resolveLatestTransparencyTrace() { return currentTrace; }
function composeEffectiveInputFromTransparency() { return "REFERENCE\n\nMAIN"; }
function isBackendEffectiveInputPreview(value) { return !!(value && value.contract_version === "effective_input_preview.v1"); }
function formatLanguageContextBlock() { return ""; }
function t(key) { return key; }
function escapeAttr(value) { return String(value == null ? "" : value); }
function truncPreview(value) { return String(value == null ? "" : value); }
function renderItBlockRaw(title, html) { return '<RAW title="' + title + '">' + html + '</RAW>'; }
function renderItBlock(title, text) { return '<BLOCK title="' + title + '">' + text + '</BLOCK>'; }

currentTrace = {_inputTransparency: {injection: {
  mainInjectionPreview: "MAIN",
  referenceInjectionPreview: "REFERENCE",
  guidanceInjectionPreview: "GUIDANCE",
  auxiliaryPreview: "REFERENCE\n\nMAIN"
}}};
let html = renderEffectiveInputSection();
assert(html.includes('<BLOCK title="Assembled Auxiliary Context">MAIN</BLOCK>'), "main context pane is not isolated");
assert(html.includes('<BLOCK title="Original Work Reference Context">REFERENCE</BLOCK>'), "reference context pane is not isolated");
assert(html.includes('<BLOCK title="Output Guidance Context">GUIDANCE</BLOCK>'), "guidance context pane is not isolated");
assert(!html.includes('<BLOCK title="Assembled Auxiliary Context">REFERENCE\n\nMAIN</BLOCK>'), "combined context leaked into main pane");

currentTrace = {_inputTransparency: {
  backendEffectiveInputPreview: {contract_version: "effective_input_preview.v1", final_user_text: "ACTUAL USER"},
  inputContext: {text: "INPUT CONTEXT"},
  injection: {
    mainInjectionPreview: "PRIORITY\n\nDIRECT\n\nEVENT",
    referenceInjectionPreview: "REFERENCE",
    protection: {text: "PRIORITY"},
    memoryDeliveryPlan: {classes: [
      {key: "direct_evidence", title: "Latest Direct Evidence", used_chars: 6, reserved_chars: 100, text: "[Latest Direct Evidence]\nDIRECT"},
      {key: "event_recent", title: "Event and Recent Memories", used_chars: 5, reserved_chars: 100, text: "[Event and Recent Memories]\nEVENT"}
    ]}
  }
}};
html = renderEffectiveInputSection();
assert(html.includes('<BLOCK title="Actual User Input">ACTUAL USER</BLOCK>'), "actual user pane is missing");
assert(html.includes('<BLOCK title="Priority and Base Rules">PRIORITY</BLOCK>'), "priority pane is missing");
assert(html.includes('title="Latest Direct Evidence · 사용 6 chars · 기본 배정 100 chars"'), "direct evidence pane is missing");
assert(html.includes('title="Event and Recent Memories · 사용 5 chars · 기본 배정 100 chars"'), "event memory pane is missing");
assert(html.includes('<BLOCK title="Input Context">INPUT CONTEXT</BLOCK>'), "input context pane is missing");
assert(!html.includes('title="Assembled Auxiliary Context"'), "planned classes fell back to one combined pane");
assert(!html.includes('Backend Effective Input Preview'), "diagnostic preview metadata leaked into final input panes");
assert(!html.includes('Final Payload Parity'), "payload parity diagnostics leaked into final input panes");

currentTrace = {_inputTransparency: {injection: {
  mainInjectionPreview: "",
  referenceInjectionPreview: "REFERENCE_ONLY"
}}};
html = renderEffectiveInputSection();
assert(!html.includes('title="Assembled Auxiliary Context"'), "empty main pane was rendered");
assert(html.includes('<BLOCK title="Original Work Reference Context">REFERENCE_ONLY</BLOCK>'), "reference-only pane is missing");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Effective Input preview runtime smoke failed: %v\n%s", err, out)
	}
}

func TestArchiveCenterJSGoPayloadPlanPreservesLanePreviewsForTransparency(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Fatalf("node is required for Go payload plan runtime smoke; set ARCHIVE_CENTER_NODE_BINARY: %v", err)
		}
	}
	src := readArchiveCenterJS(t)
	functions := strings.Join([]string{
		extractJSFunctionBlockForTest(t, src, "function computeOrchestrationDirtyHashOr1c("),
		extractJSFunctionBlockForTest(t, src, "function sanitizeEnumValue("),
		extractJSFunctionBlockForTest(t, src, "function normalizeAuxiliaryInjectionPlacement("),
		extractJSFunctionBlockForTest(t, src, "function normalizeAuxiliaryInjectionAnchorMarker("),
		extractJSFunctionBlockForTest(t, src, "function normalizeRollbackMessageRole("),
		extractJSFunctionBlockForTest(t, src, "function extractMessageContentCandidate("),
		extractJSFunctionBlockForTest(t, src, "function extractComparableMessageRoleAndContent("),
		extractJSFunctionBlockForTest(t, src, "function auxiliaryMessageContentText("),
		extractJSFunctionBlockForTest(t, src, "function getPayloadMessageRoleAndText("),
		extractJSFunctionBlockForTest(t, src, "function isChatMessageLike("),
		extractJSFunctionBlockForTest(t, src, "function isChatMessageArray("),
		extractJSFunctionBlockForTest(t, src, "function getPayloadPathValue("),
		extractJSFunctionBlockForTest(t, src, "function buildPayloadPathRebuilder("),
		extractJSFunctionBlockForTest(t, src, "function findPayloadMessagesPath("),
		extractJSFunctionBlockForTest(t, src, "function extractMessages("),
		extractJSFunctionBlockForTest(t, src, "function findFirstSystemInsertionIndex("),
		extractJSFunctionBlockForTest(t, src, "function findLatestUserInsertionIndex("),
		extractJSFunctionBlockForTest(t, src, "function findAnchorMarkerInsertionIndex("),
		extractJSFunctionBlockForTest(t, src, "function findLastCachePointInsertionIndex("),
		extractJSFunctionBlockForTest(t, src, "function resolveAuxiliaryInjectionPlacement("),
		extractJSFunctionBlockForTest(t, src, "function injectAuxiliaryBlock("),
		extractJSFunctionBlockForTest(t, src, "function injectInputContextBeforeUser("),
		extractJSFunctionBlockForTest(t, src, "function observeGoPayloadApplication("),
		extractJSFunctionBlockForTest(t, src, "function applyGoPayloadApplicationPlan("),
	}, "\n")
	script := functions + `
const assert = (condition, message) => { if (!condition) throw new Error(message); };
const AUXILIARY_INJECTION_PLACEMENT_OPTIONS = Object.freeze(["auto", "before_latest_user", "after_anchor_marker", "after_last_cache_point", "after_first_system", "end"]);
const DEFAULT_SETTINGS = {auxiliaryInjectionPlacement:"before_latest_user"};
const settings = {auxiliaryInjectionPlacement:"before_latest_user",auxiliaryInjectionAnchorMarker:""};
const runtimeUpdates = [];
function updateRuntimeState(key,status,detail) { runtimeUpdates.push({key,status,detail}); }
function warnLog() { throw new Error("unexpected production warning"); }
const RECOMPOSER_BRIDGE_CONTRACT = "archive_center_recomposer_bridge.v1";
function publishArchiveCenterRecomposerBridge() { return false; }
const memoryDeliveryPlan = {
  contract_version: "memory_delivery_plan.v1",
  classes: [{key: "event_recent", text: "MEMORY"}]
};
const auxiliaryText = "REFERENCE\n\nMEMORY\n\nGUIDANCE";
const inputContextText = "INPUT";
const exactAuxiliary = "[Archive Center — Auxiliary Context]\n\n" + auxiliaryText;
const exactInputContext = "[Archive Center — Input Context]\n\n" + inputContextText;
const plan = {
  contract_version: "payload_application_plan.v1",
  owner: "go",
  apply_rule: "apply_exact_text_without_reassembly",
  status: "ready",
  auxiliary_text: auxiliaryText,
  input_context_text: inputContextText,
  auxiliary_observation_hash: computeOrchestrationDirtyHashOr1c(exactAuxiliary),
  input_context_observation_hash: computeOrchestrationDirtyHashOr1c(exactInputContext),
  lanes: [
    {key: "original_work", text: "REFERENCE", applied: true, status: "applied"},
    {key: "long_term_memory", text: "MEMORY", applied: true, status: "applied"},
    {key: "output_guidance", text: "GUIDANCE", applied: true, status: "applied"}
  ]
};
const originalPayload = [
  {role:"system",content:"host preset"},
  {role:"assistant",content:"previous answer"},
  {role:"user",content:"continue"}
];
const applied = applyGoPayloadApplicationPlan(
  originalPayload,
  {
    _injectionPack: {payload_application_plan: plan, memory_delivery_plan: memoryDeliveryPlan},
    _sourceToPayloadLineage: {lineage_id:"stl_preview",payload_plan_id:"stp_preview",source_refs:[],execution_items:[]},
    _trace: {}
  },
  {}
);
assert(applied.injectionResult.applied === true, "production payload application was not confirmed");
assert(applied.injectionResult.mainInjectionPreview === "MEMORY", "long-term memory preview was not preserved");
assert(applied.injectionResult.referenceInjectionPreview === "REFERENCE", "original-work preview was not preserved");
assert(applied.injectionResult.guidanceInjectionPreview === "GUIDANCE", "output-guidance preview was not preserved");
assert(applied.injectionResult.memoryDeliveryPlan === memoryDeliveryPlan, "memory delivery plan was not preserved");
assert(originalPayload.length === 3, "production payload application mutated the original array");
const returnedMessages = extractMessages(applied.payload).messages;
const auxiliaryMatches = returnedMessages.filter((message) => getPayloadMessageRoleAndText(message).text === exactAuxiliary);
const inputMatches = returnedMessages.filter((message) => getPayloadMessageRoleAndText(message).text === exactInputContext);
assert(auxiliaryMatches.length === 1, "Go-owned auxiliary text was not applied exactly once");
assert(inputMatches.length === 1, "Go-owned input context was not applied exactly once");
assert(returnedMessages[returnedMessages.length - 1].content === "continue", "latest user message was not preserved");
assert(applied.injectionResult.payloadApplicationObservation.payload_application_status === "applied", "returned payload was not observed");
assert(runtimeUpdates.length === 1 && runtimeUpdates[0].status === "ok", "successful production runtime state was not recorded once");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Go payload plan transparency runtime smoke failed: %v\n%s", err, out)
	}
}

func archiveCenterRoot(t *testing.T) string {
	t.Helper()
	candidates := []string{
		filepath.Join("..", "..", "..", "Archive Center.js"),
		filepath.Join("..", "Archive Center.js"),
	}
	if root := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_ROOT")); root != "" {
		candidates = append([]string{filepath.Join(root, "Archive Center.js")}, candidates...)
	}
	if _, file, _, ok := runtime.Caller(0); ok {
		candidates = append(candidates, filepath.Join(filepath.Dir(file), "..", "..", "..", "Archive Center.js"))
	}
	var lastErr error
	for _, candidate := range candidates {
		_, err := os.Stat(candidate)
		if err == nil {
			return filepath.Dir(candidate)
		}
		lastErr = err
	}
	t.Fatalf("read Archive Center.js from candidates %v: %v", candidates, lastErr)
	return ""
}

func readArchiveCenterJS(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(archiveCenterRoot(t), "Archive Center.js"))
	if err != nil {
		t.Fatalf("read Archive Center.js: %v", err)
	}
	return string(data)
}

func TestBuildRouteCasesCoversRouteFamilies(t *testing.T) {
	cases := buildRouteCases("sess-test")
	if len(cases) < 81 {
		t.Fatalf("expected at least 81 route variants, got %d", len(cases))
	}
	seen := map[int]bool{}
	for _, tc := range cases {
		if tc.ID <= 0 {
			t.Fatalf("route %q has non-positive id %d", tc.Name, tc.ID)
		}
		if seen[tc.ID] {
			t.Fatalf("duplicate route id %d", tc.ID)
		}
		seen[tc.ID] = true
		if tc.Name == "" || tc.Method == "" || tc.Path == "" || tc.Tag == "" {
			t.Fatalf("route case has empty required field: %+v", tc)
		}
	}
}

func TestSafeRouteStatusRejectsNoRouteAndUnhandledServerErrors(t *testing.T) {
	rejected := []int{http.StatusNotFound, http.StatusMethodNotAllowed, http.StatusInternalServerError, http.StatusBadGateway}
	for _, status := range rejected {
		if safeRouteStatus(status) {
			t.Fatalf("status %d should be rejected", status)
		}
	}
	accepted := []int{http.StatusOK, http.StatusNoContent, http.StatusBadRequest, http.StatusForbidden, http.StatusServiceUnavailable}
	for _, status := range accepted {
		if !safeRouteStatus(status) {
			t.Fatalf("status %d should be accepted as route-surface liveness", status)
		}
	}
}

func TestProbeRouteDetectsMissingExpectedFields(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
	})
	result := probeRoute(handler, routeCase{
		ID:             1,
		Name:           "missing-field",
		Method:         http.MethodGet,
		Path:           "/x",
		Tag:            "R1-read",
		ExpectedFields: []string{"status", "items"},
	})
	if result.Passed {
		t.Fatal("expected missing field to fail")
	}
	if result.Detail != "missing_expected_fields:items" {
		t.Fatalf("detail = %q, want missing_expected_fields:items", result.Detail)
	}
}

func TestRunSmokeWithRealServer(t *testing.T) {
	handler := newRealSmokeHandler()
	report := runSmoke(handler, "sess-real")
	if report.Status != "ok" {
		failed := []routeResult{}
		for _, route := range report.Routes {
			if !route.Passed {
				failed = append(failed, route)
			}
		}
		t.Fatalf("real server smoke status = %q, failed routes = %+v", report.Status, failed)
	}
	if report.Summary.Total < 81 {
		t.Fatalf("summary total = %d, want >= 81", report.Summary.Total)
	}
	if report.Summary.Failed != 0 {
		t.Fatalf("summary failed = %d, want 0", report.Summary.Failed)
	}
	if report.Summary.StatusClassCounts["404_not_found"] != 0 || report.Summary.StatusClassCounts["405_method_not_allowed"] != 0 {
		t.Fatalf("unexpected no-route failure counts: %+v", report.Summary.StatusClassCounts)
	}
}

func TestArchiveCenterJSRerollRollbackPath(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"async function resolveRollbackComparableMessages",
		"function detectRollbackNeed",
		"async function checkAndAutoRollback",
		"async function executeAutoRollback",
		"await checkAndAutoRollback(orchSessionId, rollbackComparable.messages, {",
		`rollbackParams.set("req_source", requestSource);`,
		"method: \"DELETE\"",
		"requestSource = options && options.requestSource ? String(options.requestSource) : \"auto\"",
		"assistant_deleted_before_next_user_turn",
		"single_assistant_msg_removed",
		"msg_decrease_and_tail_change",
		"duplicate_rollback_blocked",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing reroll rollback path marker %q", needle)
		}
	}
}

func TestArchiveCenterJSProjectConfigGUIRuntimeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`"ollama", "custom"`,
		`<option value="ollama"${s.pluginMainProvider === "ollama" ? " selected" : ""}>Ollama</option>`,
		`<option value="ollama"${s.subLlmProvider === "ollama" ? " selected" : ""}>Ollama</option>`,
		`<option value="ollama"${s.embeddingProvider === "ollama" ? " selected" : ""}>Ollama</option>`,
		`normalized === "other" || normalized === "otherax" || normalized === "other_ax"`,
		`return "custom";`,
		`<input type="password" id="mo-pluginMainApiKey"`,
		`<input type="password" id="mo-subLlmApiKey"`,
		`<input type="password" id="mo-embeddingApiKey"`,
		`await persistentSet(SETTINGS_KEY, json)`,
		`settings = sanitizeSettings(parsed)`,
		`mainTemperature: getPluginMainTemperatureSetting(s.pluginMainTemperature)`,
		`criticTemperature: getSubLlmTemperatureSetting(s.subLlmTemperature)`,
		`supervisorTemperature: getPluginMainTemperatureSetting(s.pluginMainTemperature)`,
		`topK: s.topK`,
		`safeSettingsForLog(getSettings())`,
		`formatBridgeFailureForDisplay("/proxy/plugin-main"`,
		`narrativeGuideStrength: "weak"`,
		`const NARRATIVE_GUIDE_STRENGTH_OPTIONS = Object.freeze(["none", "weak", "medium", "strong"])`,
		`<select id="mo-narrativeGuideStrength"`,
		`<option value="none"`,
		`guide_strength: settings.narrativeGuideStrength || "weak"`,
		`Weak proposes response focus`,
		`Strong adds an arc anchor and preferred frontier`,
		`coreObjectiveMemoryMaxItems: 5`,
		`core_objective_memory_max_items: sanitizeTopKSetting(`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing project config GUI/runtime marker %q", needle)
		}
	}
}

func TestArchiveCenterJSOpenAICompatibleGatewayAndServiceTierMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`"openrouter", "llmgateway", "vercel", "vertex"`,
		`<option value="llmgateway"${s.pluginMainProvider === "llmgateway" ? " selected" : ""}>LLM Gateway</option>`,
		`<option value="llmgateway"${s.subLlmProvider === "llmgateway" ? " selected" : ""}>LLM Gateway</option>`,
		`<option value="vercel"${s.pluginMainProvider === "vercel" ? " selected" : ""}>Vercel AI Gateway</option>`,
		`<option value="vercel"${s.subLlmProvider === "vercel" ? " selected" : ""}>Vercel AI Gateway</option>`,
		`pluginMainLlmGatewayServiceTier: "standard"`,
		`subLlmLlmGatewayServiceTier: "standard"`,
		`function normalizeLlmGatewayServiceTierSetting(value)`,
		`if (serviceTier !== "standard")`,
		`payload.llm_gateway_service_tier = serviceTier`,
		`mainLlmGatewayServiceTier: mainOverrides.llmGatewayServiceTier`,
		`criticLlmGatewayServiceTier: criticOverrides.llmGatewayServiceTier`,
		`supervisorLlmGatewayServiceTier: mainOverrides.llmGatewayServiceTier`,
		`llm_gateway_service_tier: criticOverrides.llmGatewayServiceTier`,
		`id="mo-pluginMainLlmGatewayServiceTier"`,
		`id="mo-subLlmLlmGatewayServiceTier"`,
		`https://api.llmgateway.io/v1`,
		`https://ai-gateway.vercel.sh/v1`,
		`OpenAI-Compatible Service Tier`,
		`testBody.llm_gateway_service_tier = testLlmGatewayServiceTier`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing LLM Gateway marker %q", needle)
		}
	}
}

func TestArchiveCenterJSClaudePromptCacheMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const CLAUDE_PROMPT_CACHE_MODE_OPTIONS = Object.freeze(["off", "ephemeral_5m", "ephemeral_1h"])`,
		`pluginMainClaudePromptCacheMode: "off"`,
		`subLlmClaudePromptCacheMode: "off"`,
		`function normalizeClaudePromptCacheModeSetting(value)`,
		`payload.claude_prompt_cache_mode = normalizeClaudePromptCacheModeSetting(`,
		`mainClaudePromptCacheMode: mainOverrides.claudePromptCacheMode`,
		`criticClaudePromptCacheMode: criticOverrides.claudePromptCacheMode`,
		`supervisorClaudePromptCacheMode: mainOverrides.claudePromptCacheMode`,
		`claude_prompt_cache_mode: criticOverrides.claudePromptCacheMode`,
		`id="mo-pluginMainClaudePromptCacheMode"`,
		`id="mo-subLlmClaudePromptCacheMode"`,
		`>Automatic 5 min</option>`,
		`>Automatic 1 hour</option>`,
		`syncProviderSpecificRow("mo-pluginMainProvider", "mo-pluginMainClaudePromptCacheModeRow", "claude")`,
		`syncProviderSpecificRow("mo-subLlmProvider", "mo-subLlmClaudePromptCacheModeRow", "claude")`,
		`testBody.claude_prompt_cache_mode = testClaudePromptCacheMode`,
		`extraBodyJson: sanitizeProviderOverrideJsonSetting(`,
		`if (extraBody) payload.extra_body_json = extraBody;`,
		`const BUILD_NOTES = "Archive Center 3.9.11"`,
		`비용: 5분 캐시 쓰기 1.25배, 1시간 쓰기 2배, 캐시 읽기 0.1배`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing Claude prompt cache marker %q", needle)
		}
	}
}

func TestArchiveCenterJSAuxiliaryInjectionPlacementI18nAndNoStaleBudgetPreview(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const _i18n = {`,
		`"settings.label.auxiliaryInjectionPlacement":`,
		`"settings.hint.auxiliaryInjectionPlacement":`,
		`"settings.label.auxiliaryInjectionAnchorMarker":`,
		`"settings.hint.auxiliaryInjectionAnchorMarker":`,
		`"settings.option.auxiliaryInjectionPlacement.auto":`,
		`"settings.option.auxiliaryInjectionPlacement.before_latest_user":`,
		`"settings.option.auxiliaryInjectionPlacement.after_anchor_marker":`,
		`"settings.option.auxiliaryInjectionPlacement.after_last_cache_point":`,
		`"settings.option.auxiliaryInjectionPlacement.after_first_system":`,
		`"settings.option.auxiliaryInjectionPlacement.end":`,
		`<label>${t('settings.label.auxiliaryInjectionPlacement')}</label>`,
		`${t('settings.option.auxiliaryInjectionPlacement.auto')}`,
		`${t('settings.hint.auxiliaryInjectionAnchorMarker')}`,
		`const prepareInjectionBudget = estimateAdaptiveInjectionBudgetParts(settings, prepareOptions.runtimeTokenInfo || null);`,
		`max_injection_chars: freshFirstTurnLightMode ? 0 : prepareInjectionBudget.configuredBudgetChars,`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing auxiliary injection i18n/budget marker %q", needle)
		}
	}
	forbidden := []string{
		`<label>Memory Injection Placement</label>`,
		`<label>Memory Anchor Marker</label>`,
		`<small>Controls where the large Archive Center memory block is inserted.`,
		`const estimatedBudget = info.budgetLimit || estimatedParts.budgetLimit;`,
		`function renderSettingsInjectionBudgetPreview(`,
		`mo-injection-budget-preview`,
		`settings.label.injectionBudgetPreview`,
	}
	for _, needle := range forbidden {
		if strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js still has stale auxiliary injection UI/budget marker %q", needle)
		}
	}
}

func TestSeq01ContextInjectionToggleRemovedAndSyncedToInputImprovement(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`"settings.section.common.desc": "The previous completed turn is included as continuity context by default. The optional input-improvement LLM is independent from narrative guidance."`,
		`"settings.label.pluginMainApplyMode": "Input Improvement LLM (Optional)"`,
		`merged.dbEnabled = true;`,
		`merged.supervisorEnabled = true;`,
		`settings.pluginMainApplyMode`,
		`inputImprovementApplied`,
		`rewriteAllowed: applyModeName === 'reviewed_apply' && !!settings.pluginMainRewriteLegacyOptIn && payloadRewritten`,
		`<select id="mo-pluginMainApplyMode">`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-01 context-injection/input-improvement marker %q", needle)
		}
	}
	forbidden := []string{
		`id="mo-enabled"`,
		`id="mo-dbEnabled"`,
		`id="mo-supervisorEnabled"`,
		`mo-injection-budget-preview`,
		`<select id="mo-narrativeGuideStrength"${s.pluginMainApplyMode === "off"`,
		`const syncInputImprovementDependentControls =`,
		`if (_narrativeGuideOff) return Promise.resolve(null);`,
		`reasoningSummary: "guide_off"`,
	}
	for _, needle := range forbidden {
		if strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js still exposes legacy SEQ-01 manual toggle %q", needle)
		}
	}
}

func TestSeq01DeadNarrativeStanceAndResumeTriggerCustomUIRemoved(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`<select id="mo-narrativeGuideStrength">`,
		`pluginMainApplyMode: $("mo-pluginMainApplyMode").value`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-01 narrative guide marker %q", needle)
		}
	}
	forbidden := []string{
		`storyNarrativeStance`,
		`mo-storyNarrativeStance`,
		`NARRATIVE_STANCE_MODES`,
		`buildInitiativeModeSuffix`,
		`buildInitiativeModeBounds`,
		`resumeTrigger`,
		`customResumeTrigger`,
		`mo-resumeTrigger`,
	}
	for _, needle := range forbidden {
		if strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js still exposes removed SEQ-01 custom resume trigger marker %q", needle)
		}
	}
}

func TestSeq01NarrativeGuideAutoTraceDashboardAndLegacyCleanupMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`narrativeGuideMode: "auto"`,
		`"settings.label.narrativeGuideMode.help": "Auto does not infer genre from story keywords; it uses Standard. Genre-specific modes apply only when selected explicitly."`,
		`<select id="mo-narrativeGuideMode">`,
		`guide_mode: requestedGuideMode`,
		`const supervisorResult = (preparedBundle && preparedBundle.supervisorResult)`,
		`guideModeBasis: (supervisorResult && supervisorResult._guideModeBasis) || "manual"`,
		`const guideModeDashboardState = lastGuideSupervisor && lastGuideSupervisor.guideMode`,
		`guide_mode_state: guideModeDashboardState`,
		`renderDashboardViewModel(dashboardViewModel, dashLabel)`,
		`delete merged.projectMainProvider; delete merged.projectMainModel;`,
		`delete merged.projectSupervisorProvider; delete merged.projectSupervisorModel;`,
		`delete merged.projectCriticProvider; delete merged.projectCriticModel;`,
		`delete merged.projectEmbeddingProvider; delete merged.projectEmbeddingModel;`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-01 narrative guide/legacy cleanup marker %q", needle)
		}
	}
	forbidden := []string{
		`let _guideModeRuntimeCache =`,
		`function resolveNarrativeGuideMode(`,
		`function inferNarrativeGuideModeFromText(`,
	}
	for _, needle := range forbidden {
		if strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js still contains removed JavaScript narrative guide policy %q", needle)
		}
	}
}

func TestSeq01SettingsSaveResetAndBridgeConfigMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`async function saveSettings()`,
		`await persistentSet(SETTINGS_KEY, json)`,
		`const syncAck = await syncConfigToBackend(settings);`,
		`warnLog("Settings save failed:", err.message);`,
		`return false;`,
		`function attachSettingsEvents()`,
		`$("mo-save-btn").addEventListener("click", async () => {`,
		`$("mo-reset-btn").addEventListener("click", async () => {`,
		`!confirm(t("settings.confirm.resetDefaults"))`,
		`settings = { ...DEFAULT_SETTINGS };`,
		`await saveSettings();`,
		`<input type="text" id="mo-bridgeUrl"`,
		`<input type="number" id="mo-requestTimeoutMs"`,
		`<input type="number" id="mo-topK"`,
		`settings.bridgeUrl = sanitizeBridgeUrl(`,
		`settings.requestTimeoutMs = getCurrentUiRequestTimeoutMs();`,
		`topK: $("mo-topK").value`,
		`failedQueueMaxAttempts: failedQueueMaxAttempts(),`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-01 settings save/reset/config marker %q", needle)
		}
	}
}

func TestSeq01RuntimeStateNarrativeTypeAndSearchCallMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`function updateRuntimeState(key, status, extra = {})`,
		`function isNarrativeType(type)`,
		`if (!settings.enabled || !isSaveType(type)) return payload;`,
		`async function runMemorySearch(userInput, options = {})`,
		`() => bridgeFetch("/search", { method: "POST", body, timeoutMs: getRequestTimeoutSettingMs() })`,
		`updateRuntimeState("lastSearchStatus", "fail"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-01 runtime/search marker %q", needle)
		}
	}
}

func TestSeq01BridgeTimeoutAppliesToNativeAndFallbackFetch(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`fetchPromise = R.nativeFetch(url, fetchInit);`,
		`fetchPromise = fetch(url, fetchInit);`,
		`response = timeoutPromise ? await Promise.race([fetchPromise, timeoutPromise]) : await fetchPromise;`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing optional bridge timeout path %q", needle)
		}
	}
}

func TestSeq02SessionAwareExplorerSyncMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`activeChatSessionId: null`,
		`_explorer.activeChatSessionId = resolvedSid`,
		`explorer.sync.currentChat`,
		`explorer.sync.differentSession`,
		`explorer.sync.gotoLiveBtn`,
		`explorer.sync.matchTooltip`,
		`explorer.sync.mismatchTooltip`,
		`const liveSid = _explorer.activeChatSessionId`,
		`await explorerChangeSession(_explorer.activeChatSessionId)`,
		`const el = $("mo-session-id-display")`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-02 session-aware explorer sync marker %q", needle)
		}
	}
}

func TestExplorerChatLogsRenderLogicalTurnsWithTwoRawPanes(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const userRow = item.user && typeof item.user === "object" ? item.user : null;`,
		`const assistantRow = item.assistant && typeof item.assistant === "object" ? item.assistant : null;`,
		`t('explorer.chatLogs.userInput')`,
		`t('explorer.chatLogs.assistantOutput')`,
		`mo-chat-turn-panes`,
		`if (assistantRow && item.turn_index != null && sessionMatch)`,
		`if (row && (row.user || row.assistant))`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing logical chat turn marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq03ExplorerDeleteMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`// [DB DELETE — Sprint 3-A-1: LOGIC]`,
		`async function explorerDeleteWithPostFallback(postPath, deletePath, debugTag)`,
		`async function explorerDeleteMemory(memoryId)`,
		`async function explorerDeleteKgTriple(tripleId)`,
		`"/explorer/memories/" + memoryId + "/delete?chat_session_id=" + encodeURIComponent(sid)`,
		`"/explorer/kg_triples/" + tripleId + "/delete?chat_session_id=" + encodeURIComponent(sid)`,
		`await explorerLoadTab("memories", true);`,
		`await explorerLoadTab("kg_triples", true);`,
		`await refreshExplorerUI();`,
		`t('explorer.delete.failed')`,
		`t('explorer.delete.error')`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-03 explorer delete marker %q", needle)
		}
	}
}

func TestArchiveCenterJSPluginMainRuntimeWiringMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"async function syncConfigToBackend(s)",
		"mainApiKey: typeof s.pluginMainApiKey === \"string\" ? s.pluginMainApiKey : \"\"",
		"mainEndpoint: typeof s.pluginMainEndpoint === \"string\" ? s.pluginMainEndpoint : \"\"",
		"mainModel: typeof s.pluginMainModel === \"string\" ? s.pluginMainModel : \"\"",
		"mainProvider,",
		"supervisorApiKey: typeof s.pluginMainApiKey === \"string\" ? s.pluginMainApiKey : \"\"",
		"supervisorEndpoint: typeof s.pluginMainEndpoint === \"string\" ? s.pluginMainEndpoint : \"\"",
		"supervisorModel: typeof s.pluginMainModel === \"string\" ? s.pluginMainModel : \"\"",
		"mainTimeout: Math.ceil(getPluginMainTimeoutSettingMs(s.pluginMainTimeoutMs) / 1000)",
		"const runtimeSynced = !!(trace && trace.synced === true);",
		"async function ensureBackendRuntimeConfigBinding(backendInstanceId)",
		"settings_runtime_bound_to_backend_instance",
		`client_meta: buildAdminRuntimeClientMeta({ source: "hypamemory_import" })`,
		"function pluginMainHasConfig()",
		"settings.pluginMainApiKey.trim()",
		"settings.pluginMainEndpoint.trim()",
		"settings.pluginMainModel.trim()",
		"async function callPluginMainLlm(systemPrompt, userContent, options)",
		"const endpoint = settings.pluginMainEndpoint.trim().replace(/\\/$/, \"\")",
		"const model    = settings.pluginMainModel.trim();",
		"const apiKey   = settings.pluginMainApiKey.trim();",
		"const proxyBody = {",
		"bridgeFetch(\"/proxy/plugin-main\"",
		"pluginMainApiKey: $(\"mo-pluginMainApiKey\").value",
		"pluginMainEndpoint: $(\"mo-pluginMainEndpoint\").value.trim()",
		"pluginMainModel: $(\"mo-pluginMainModel\").value.trim()",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing Plugin Main runtime wiring marker %q", needle)
		}
	}
}

func TestRuntimeConfigBindsOncePerBackendInstanceBeforeFullPrepare(t *testing.T) {
	src := readArchiveCenterJS(t)
	beforeRequest := extractJSFunctionBlockForTest(t, src, "async function onBeforeRequest(payload, type)")
	sourceDecision := strings.Index(beforeRequest, "const sourceDecisionResult = await tryPrepareTurn(")
	runtimeBinding := strings.Index(beforeRequest, "const runtimeConfigBinding = await ensureBackendRuntimeConfigBinding(")
	fullPrepare := strings.Index(beforeRequest, "const preparedTurnResult = await tryPrepareTurn(")
	if sourceDecision < 0 || runtimeBinding < 0 || fullPrepare < 0 {
		t.Fatalf("runtime config binding markers missing: source=%d binding=%d full=%d", sourceDecision, runtimeBinding, fullPrepare)
	}
	if !(sourceDecision < runtimeBinding && runtimeBinding < fullPrepare) {
		t.Fatalf("runtime config must bind after backend reachability and before full prepare: source=%d binding=%d full=%d", sourceDecision, runtimeBinding, fullPrepare)
	}
	if strings.Contains(beforeRequest, "await syncConfigToBackend(settings)") {
		t.Fatal("normal turn path still performs an unconditional runtime config update")
	}
}

func TestResetDefaultsRequiresConfirmationBeforeMutation(t *testing.T) {
	src := readArchiveCenterJS(t)
	start := strings.Index(src, `$("mo-reset-btn").addEventListener("click", async () => {`)
	if start < 0 {
		t.Fatal("reset handler missing")
	}
	handler := src[start:]
	confirmIndex := strings.Index(handler, `!confirm(t("settings.confirm.resetDefaults"))`)
	mutationIndex := strings.Index(handler, `settings = { ...DEFAULT_SETTINGS };`)
	if confirmIndex < 0 || mutationIndex < 0 || confirmIndex > mutationIndex {
		t.Fatalf("reset confirmation must precede settings mutation: confirm=%d mutation=%d", confirmIndex, mutationIndex)
	}
}

func TestNormalTurnPayloadDoesNotRepeatRuntimeCredentials(t *testing.T) {
	src := readArchiveCenterJS(t)
	prepare := extractJSFunctionBlockForTest(t, src, "async function tryPrepareTurn(")
	complete := extractJSFunctionBlockForTest(t, src, "async function buildCompleteTurnRequestBody(")
	for name, block := range map[string]string{"prepare-turn": prepare, "complete-turn": complete} {
		for _, forbidden := range []string{"client_meta.embedding", "client_meta.critic", "api_key: effectiveCritic", "api_key: embeddingApiKey"} {
			if strings.Contains(block, forbidden) {
				t.Fatalf("%s repeats runtime credentials through %q", name, forbidden)
			}
		}
	}
}

func TestArchiveCenterJSSessionIsolationMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"async function getCurrentChatSessionId()",
		"function isCidSessionId(sessionId)",
		"function isIndexSessionId(sessionId)",
		"`char_${charIdx}_cid_${chatUniqueId}`",
		"savePinnedSessionId(charIdx, chatIdx, sessionId, chatUniqueId, stableCharacterId)",
		"cacheRawInputForSession(sessionId, rawInput)",
		"peekRawInputForSession(sessionId)",
		"async function resolveCanonicalWriteSessionId(rawSessionId",
		"activeChatIdentity.isFreshChat",
		"fresh_chat_kept",
		"params.set(\"sessionId\", requestedSessionId)",
		"data-timeline-session-id",
		"resolveRuntimeSessionLifecycle(sessionId)",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing session isolation marker %q", needle)
		}
	}
}

func TestArchiveCenterJSCrossChatCompatReadGuardMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"const activeCidSessionId = (charIdx != null && chatUniqueId)",
		"const primaryIsActiveCid = !!(activeCidSessionId && primarySessionId === activeCidSessionId)",
		"allowStructuralAlias: !treatAsFreshCidSession && !primaryIsActiveCid",
		"allowLooseFallback: false",
		"const sameCharacterAutoAttachDisabled = true",
		"manual_attach_or_migrate_required",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing cross-chat compat read guard marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq03RMG03SessionKeyHotfixMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"const cidSessionId = (charIdx != null && chatUniqueId)",
		"? `char_${charIdx}_cid_${chatUniqueId}`",
		"const fallbackIndexSessionId = (chatIdx != null && charIdx != null)",
		"? `char_${charIdx}_chat_${chatIdx}`",
		"sessionId = cidSessionId;",
		"(cidSessionId && isIndexSessionId(pinnedSessionId)) ? cidSessionId : pinnedSessionId",
		"else if (isIndexSessionId(pinnedSessionId))",
		"sessionId = cidSessionId || pinnedSessionId",
		"savePinnedSessionId(charIdx, chatIdx, sessionId, chatUniqueId, stableCharacterId)",
		"observedChatUniqueId: String(observedChatUniqueId || \"\").trim()",
		"function buildRawInputSessionKeys(sessionId)",
		"primary.match(/^(char_\\d+)_(?:cid_.+|chat_\\d+)$/)",
		"addRawInputSessionKey(keys, seen, charAliasMatch[1])",
		"addRawInputSessionKey(keys, seen, SESSION_FALLBACK)",
		"function bindRawInputObservationToRequest(sessionId, requestId)",
		"observation.boundRequestId = String(requestId)",
		"if (candidate === observation) _rawInputBySession.delete(key)",
		"const _sessionTurnIndices = new Map()",
		"const SESSION_TURN_MAP_MAX = 50",
		"function getSessionTurnIndex(sessionId)",
		"function setSessionTurnIndex(sessionId, idx)",
		"_sessionTurnIndices.keys().next().value",
		"_sessionTurnIndices.delete(oldest)",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-03/RMG-03 session-key marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq04SanitizeTraceMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function buildSanitizeTrace(stage, before, after)",
		"function attachSanitizeTrace(trace, entry)",
		"trace._inputTransparency.sanitization = trace.sanitization",
		`debugLog("sanitize trace:", entry.stage, "removed", entry.removedChars, "chars")`,
		`buildSanitizeTrace("display_output", content, normalizedContent)`,
		`buildSanitizeTrace("critic_persist_assistant", displayContent, persistedAssistantContent)`,
		`buildSanitizeTrace("critic_user_input", criticUserInput, safeUser)`,
		`renderItBlockRaw("1-0. Sanitization Trace", sanHtml, false)`,
		`rows.push(r("Sanitize", changedCount > 0 ? "ok" : "skipped"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-04 sanitize trace marker %q", needle)
		}
	}
}

func TestArchiveCenterJSActivitySnapshotMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"let _lastActivitySnapshot = null;",
		"runId: _actRunId",
		"startedAt: _actStarted",
		"duration_ms: _actTotalMs",
		"stages: _actStages",
		"const _actTotalMs = Date.now() - _actStarted;",
		"_actStages.prepare = Date.now() - _stageStart;",
		"_actStages.prepare",
		"_actStages.recall = Date.now() - _stageStart;",
		"_actStages.recall",
		"_actStages.supervisor = Date.now() - _stageStart;",
		"_actStages.supervisor",
		"_lastActivitySnapshot.stages.inject = _injectMs",
		"const _updatedActivityTotalMs = (_lastActivitySnapshot.duration_ms || _lastActivitySnapshot.totalMs || 0) + _injectMs;",
		"_lastActivitySnapshot.tokenUsage =",
		"injectedChars: injectionResult.totalChars || 0",
		"budgetUsed: injectionResult.totalChars || 0",
		"budgetLimit: injectionResult.budgetLimit || 0",
		"sectionsIncluded: (injectionResult.blocks || []).map(function(b) { return b.label; })",
		"sectionsSkipped: (injectionResult.trimmed || []).filter(function(t) { return t.reason === \"budget_exhausted\"; }).map(function(t) { return t.label; })",
		"llmCalls: {",
		"function renderActivitySection()",
		"renderActivitySection()",
		"trace.activity =",
		"duration_ms: _lastActivitySnapshot.duration_ms",
		"injectedChars: (_lastActivitySnapshot.tokenUsage || {}).injectedChars || 0",
		"const actDuration = act.duration_ms || act.totalMs || 0",
		"tu.sectionsIncluded.join(\", \")",
		"tu.sectionsSkipped.join(\", \")",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing E-6 activity snapshot marker %q", needle)
		}
	}
}

func TestArchiveCenterJSActivitySnapshotRuntimeDisplay(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for JS runtime behavior smoke")
	}
	src := readArchiveCenterJS(t)
	script := extractJSFunctionBlockForTest(t, src, "function renderActivitySection()") + `
const assert = (cond, msg) => { if (!cond) throw new Error(msg); };
function t(key) {
  const dict = {
    "dash.activity.noData": "no activity",
    "dash.activity.callSuffix": " call",
    "dash.activity.notCalled": "not called",
    "dash.activity.noCount": "no count"
  };
  return dict[key] || key;
}
function statusDotClass(status) { return "dot-" + status; }
function escapeAttr(value) {
  return String(value == null ? "" : value)
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}
let _lastActivitySnapshot = {
  runId: "seq05-run",
  startedAt: "2026-06-08T00:00:00Z",
  duration_ms: 48,
  stages: { prepare: 3, recall: 7, supervisor: 11, inject: 13 },
  llmCalls: { supervisor: 1, supervisorLatencyMs: 22 },
  counts: { memories: 2, kgTriples: 1, episodes: 1, activeStates: 1, storylines: 1, characters: 1, worldRules: 1 },
  tokenUsage: { injectedChars: 120, budgetLimit: 800, sectionsIncluded: ["memories", "world"], sectionsSkipped: ["episodes"] },
  flags: { continuityUsed: true, pathBUsed: true, guideModeActive: true, storylineOverlayUsed: true, worldRuleOverlayUsed: true }
};
const html = renderActivitySection();
for (const needle of [
  "Total", "48ms", "run:seq05-run",
  "prepare", "3ms", "recall", "7ms", "supervisor", "11ms", "inject", "13ms",
	"LLM: supervisor", "1 call", "22ms",
	"mem:2", "kg:1", "ep:1", "as:1", "sl:1", "ch:1", "wr:1",
	"Budget", "120/800ch", "included", "memories, world", "skipped", "episodes",
	"continuity", "pathB", "guideMode", "slOverlay", "wrOverlay"
]) {
  assert(html.includes(needle), "activity render missing " + needle + " in " + html);
}
for (const stage of ["prepare", "recall", "supervisor", "inject"]) {
  assert(Number.isFinite(_lastActivitySnapshot.stages[stage]) && _lastActivitySnapshot.stages[stage] >= 0, "invalid stage duration " + stage);
}
_lastActivitySnapshot = null;
assert(renderActivitySection().includes("no activity"), "empty activity snapshot fallback missing");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node activity snapshot runtime display smoke failed: %v\n%s", err, out)
	}
}

func TestArchiveCenterJSI18nFrameworkMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`uiLanguage: "ko"`,
		`const _i18n = {`,
		`ko: {`,
		`en: {`,
		`ja: {`,
		`function t(key, overrideLang)`,
		`var lang = overrideLang || (settings && settings.uiLanguage) || "ko";`,
		`if (lang !== "en" && _i18n.en && _i18n.en[key] != null) return _i18n.en[key];`,
		`if (lang !== "ko" && _i18n.ko && _i18n.ko[key] != null) return _i18n.ko[key];`,
		`return key;`,
		`"settings.label.uiLanguage"`,
		`<select id="mo-uiLanguage">`,
		`uiLanguage: $("mo-uiLanguage").value`,
		`await persistentSet(SETTINGS_KEY, json)`,
		`settings = sanitizeSettings(parsed)`,
		`${t('settings.title')}`,
		`${t('dash.section.turnTrace')}`,
		"const settingsSubtabsHtml = (activeTab) => {",
		`t('explorer.chatLogs.loading')`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing F-1 i18n marker %q", needle)
		}
	}

	thresholds := map[string]int{
		`"settings.`: 300,
		`"dash.`:     250,
		`"explorer.`: 300,
		`"common.`:   40,
	}
	for prefix, minCount := range thresholds {
		if got := strings.Count(src, prefix); got < minCount {
			t.Fatalf("Archive Center.js i18n key prefix %q count = %d, want >= %d", prefix, got, minCount)
		}
	}
	if strings.Contains(src, "function tp(") || strings.Contains(src, "tp(") {
		t.Fatal("Archive Center.js should not use tp(); non-UI prompt language separation belongs to F-3")
	}
}

func TestArchiveCenterJSI18nRuntimeSwitchAndPersistenceMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const UI_LANGUAGE_OPTIONS = Object.freeze(["ko", "en", "ja"]);`,
		`merged.uiLanguage = sanitizeEnumValue(`,
		`async function applyUiLanguageChange(nextLang)`,
		`const normalized = sanitizeEnumValue(nextLang, DEFAULT_SETTINGS.uiLanguage, UI_LANGUAGE_OPTIONS);`,
		`const prevActiveTab = _settingsActiveTab || "timeline";`,
		`await updateSettings({ uiLanguage: normalized });`,
		`await closeSettingsPanel();`,
		`await renderSettingsPanel();`,
		`const uiLanguageSelect = $("mo-uiLanguage");`,
		`uiLanguageSelect.addEventListener("change", () => {`,
		`applyUiLanguageChange(uiLanguageSelect.value);`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing F-1 runtime i18n switch marker %q", needle)
		}
	}
}

func TestArchiveCenterJSI18nRuntimeSwitchAndPersistenceBehavior(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for JS runtime behavior smoke")
	}
	src := readArchiveCenterJS(t)
	i18nStart := strings.Index(src, "const _i18n = {")
	i18nComment := strings.Index(src, "* F-1: UI")
	i18nEnd := -1
	if i18nComment > i18nStart {
		i18nEnd = strings.LastIndex(src[:i18nComment], "/**")
	}
	if i18nStart < 0 || i18nEnd < 0 || i18nEnd <= i18nStart {
		t.Fatalf("Archive Center.js missing i18n dictionary block")
	}
	script := `const VERSION = "3.7.0-dev";` + "\n" +
		src[i18nStart:i18nEnd] + "\n" +
		extractJSFunctionBlockForTest(t, src, "function t(key, overrideLang)") + "\n" +
		extractJSFunctionBlockForTest(t, src, "async function applyUiLanguageChange(nextLang)") + `
const assert = (cond, msg) => { if (!cond) throw new Error(msg); };
const UI_LANGUAGE_OPTIONS = ["ko", "en", "ja"];
const DEFAULT_SETTINGS = { uiLanguage: "ko" };
let settings = { uiLanguage: "ko" };
let selector = { disabled: false, value: "ko" };
let statusEl = { textContent: "", style: {} };
let _settingsActiveTab = "prompt";
let savedPayloads = [];
let closeCount = 0;
let renderCount = 0;
let saveResult = true;
function sanitizeEnumValue(value, fallback, options) {
  return options.includes(value) ? value : fallback;
}
function $(id) {
  if (id === "mo-uiLanguage") return selector;
  if (id === "mo-save-status") return statusEl;
  return null;
}
async function updateSettings(patch) {
  savedPayloads.push(patch);
  if (!saveResult) return false;
  settings = Object.assign({}, settings, patch);
  return true;
}
async function closeSettingsPanel() { closeCount++; }
async function renderSettingsPanel() { renderCount++; }

(async () => {
  assert(t("settings.title", "ko").includes("설정"), "ko settings title missing");
  assert(t("settings.title", "en").includes("Settings"), "en settings title missing");
  assert(t("settings.title", "en").includes(VERSION), "settings title is not synchronized with VERSION");
  assert(t("settings.title", "ja").includes("設定"), "ja settings title missing");
  assert(t("missing.seq05.key", "ja") === "missing.seq05.key", "missing key fallback regressed");
  await applyUiLanguageChange("ja");
  assert(settings.uiLanguage === "ja", "language not persisted to settings");
  assert(savedPayloads.length === 1 && savedPayloads[0].uiLanguage === "ja", "updateSettings payload mismatch");
  assert(closeCount === 1 && renderCount === 1, "settings panel did not close and render once");
  assert(_settingsActiveTab === "prompt", "active settings tab was not preserved");
  assert(selector.disabled === true, "successful change keeps selector disabled until rerender replaces it");
  selector.disabled = false;
  selector.value = "ja";
  saveResult = false;
  await applyUiLanguageChange("en");
  assert(settings.uiLanguage === "ja", "failed save changed settings");
  assert(selector.value === "ja" && selector.disabled === false, "failed save did not restore selector");
  assert(statusEl.textContent, "failed save did not surface status text");
  saveResult = true;
  await applyUiLanguageChange("invalid-language");
  assert(savedPayloads[savedPayloads.length - 1].uiLanguage === "ko", "invalid language did not sanitize to default");
})().catch((err) => {
  console.error(err && err.stack ? err.stack : err);
  process.exit(1);
});
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node i18n runtime switch smoke failed: %v\n%s", err, out)
	}
}

func TestArchiveCenterJSMultilingualEntityMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function extractEntitiesFromText(text)",
		`trimmed.match(/\b[A-Z][a-zA-Z]{1,20}\b/g)`,
		"trimmed.match(/[\uAC00-\uD7A3]{2,8}/g)",
		`trimmed.match(/[\u30A1-\u30F6\u30FC]{3,}/g)`,
		`trimmed.match(/[\u4E00-\u9FFF]{2,4}/g)`,
		"function romanizeKorean(text)",
		"function romanizeKatakana(text)",
		"function normalizeEntityName(name)",
		"const _entityAliasMap = new Map()",
		"function registerEntityAlias(name)",
		"function expandEntitiesWithAliases(entities)",
		"const expandedEntities = expandEntitiesWithAliases(extractedEntities)",
		"kgRecallResult = await runKGRecall(expandedEntities, chatSessionId)",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing F-2 multilingual entity marker %q", needle)
		}
	}
}

func TestArchiveCenterJSCharacterSpeechEditMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"async function explorerPatchCharacterSpeech(characterName)",
		`"/characters/" + encodeURIComponent(sid) + "/" + encodeURIComponent(name) + "/speech"`,
		`{ method: "PATCH", body: { speech_style: speechStyle } }`,
		`data-ent-speech-edit="`,
		`data-edit-type="char_speech"`,
		`data-save-type="char_speech"`,
		`data-field="default_tone"`,
		`data-field="honorific_style"`,
		`data-field="speech_notes"`,
		`document.querySelectorAll("[data-ent-speech-edit]")`,
		`explorerStartEdit("char_speech", name,`,
		`await explorerPatchCharacterSpeech(btn.dataset.saveKey || "")`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing character speech edit marker %q", needle)
		}
	}
}

func TestArchiveCenterJSMultilingualEntityRuntimeBehavior(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for JS runtime behavior smoke")
	}
	src := readArchiveCenterJS(t)
	start := strings.Index(src, "const _entityAliasMap = new Map();")
	runKGRecall := strings.Index(src, "async function runKGRecall")
	if start < 0 || runKGRecall < 0 || runKGRecall <= start {
		t.Fatalf("Archive Center.js missing multilingual entity runtime extraction block")
	}
	end := strings.LastIndex(src[:runKGRecall], "/**")
	if end <= start {
		t.Fatalf("Archive Center.js multilingual block end marker not found")
	}

	script := src[start:end] + `
const assert = (cond, msg) => { if (!cond) throw new Error(msg); };
const koMina = "\uBBFC\uC544";
const koRowan = "\uB85C\uC644";
const koAkira = "\uC544\uD0A4\uB77C";
const jaMina = "\u30DF\u30CA";
const jaAkira = "\u30A2\u30AD\u30E9";
const korean = extractEntitiesFromText(koMina + "\uAC00 " + koRowan + "\uC5D0\uAC8C \uB3CC\uC544\uC624\uACA0\uB2E4\uACE0 \uB9D0\uD588\uB2E4.");
assert(korean.includes(koMina), "Korean name extraction lost Mina");
assert(korean.includes(koRowan), "Korean name extraction lost Rowan");
assert(normalizeEntityName(koMina) === "mina", "Korean Mina normalization mismatch: " + normalizeEntityName(koMina));
const katakana = extractEntitiesFromText(jaAkira + "\u306F" + jaMina + "\u3068\u5E02\u5834\u3067\u4F1A\u3063\u305F\u3002");
assert(katakana.includes(jaAkira), "Katakana name extraction lost Akira");
assert(normalizeEntityName(jaAkira) === "akira", "Katakana Akira normalization mismatch: " + normalizeEntityName(jaAkira));
const english = extractEntitiesFromText("Mina met Rowan after The storm near Gate.");
assert(english.includes("Mina"), "English extraction lost Mina");
assert(english.includes("Rowan"), "English extraction lost Rowan");
assert(!english.includes("The"), "English stop-word filtering regressed");
assert(normalizeEntityName("Mina") === "mina", "English Mina normalization mismatch");
assert(normalizeEntityName(koMina) === normalizeEntityName("Mina"), "ko/en Mina alias key mismatch");
assert(normalizeEntityName(jaMina) === normalizeEntityName("Mina"), "ja/en Mina alias key mismatch");
assert(normalizeEntityName(koAkira) === normalizeEntityName("Akira"), "ko/en Akira alias key mismatch");
assert(normalizeEntityName(jaAkira) === normalizeEntityName("Akira"), "ja/en Akira alias key mismatch");
expandEntitiesWithAliases([koAkira]);
expandEntitiesWithAliases(["Akira"]);
const akiraAliases = expandEntitiesWithAliases([jaAkira]);
assert(akiraAliases.includes(koAkira), "alias map lost Korean Akira variant");
assert(akiraAliases.includes("Akira"), "alias map lost English Akira variant");
assert(akiraAliases.includes(jaAkira), "alias map lost Japanese Akira variant");
console.log(JSON.stringify({korean, katakana, english, akiraAliases}));
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node multilingual entity runtime smoke failed: %v\n%s", err, out)
	}
}

func TestArchiveCenterJSSameTurnOverlayFreshnessRuntimeBehavior(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for JS runtime behavior smoke")
	}
	src := readArchiveCenterJS(t)
	start := strings.Index(src, "function normalizeStorylineStatus")
	end := strings.Index(src, "function makeEmptyContinuityPackResult")
	if start < 0 || end < 0 || end <= start {
		t.Fatalf("Archive Center.js missing same-turn overlay helper block")
	}

	script := src[start:end] + `
const assert = (cond, msg) => { if (!cond) throw new Error(msg); };
const supervisor = {
  directive: {
    storylines: [{
      name: "Rooftop Promise",
      status: "active",
      current_context: "The same turn confession must guide the next answer.",
      key_points: ["confession"],
      ongoing_tensions: ["answer pending"]
    }],
    section_world: {
      applies: true,
      genre_hint: "mystery",
      rules: ["Rule primary"],
      world_rules: ["Legacy world rule fallback"],
      confidence_notes: ["Confidence note fallback"]
    }
  }
};
const storylineOverlay = buildStorylineOverlay(supervisor);
assert(storylineOverlay.length === 1, "storyline overlay count mismatch");
assert(storylineOverlay[0]._overlayCurrentTurn === true, "storyline overlay is not marked current-turn");
const mergedStorylines = mergeStorylineOverlay({
  items: [
    { name: "Rooftop Promise", status: "paused", last_turn: 4, updated_at: "2026-05-01T00:00:00Z" },
    { name: "Older Arc", status: "active", last_turn: 8, updated_at: "2026-05-02T00:00:00Z" }
  ]
}, storylineOverlay);
assert(mergedStorylines.count === 2, "storyline merge should update matching row, not duplicate it");
assert(mergedStorylines.usedOverlay === true, "storyline usedOverlay missing");
assert(mergedStorylines.overlayCount === 1, "storyline overlayCount mismatch");
assert(mergedStorylines.freshness.mode === "overlay", "storyline freshness mode mismatch");
assert(mergedStorylines.freshness.overlayItems === 1, "storyline freshness overlayItems mismatch");
assert(mergedStorylines.items[0].name === "Rooftop Promise", "storyline overlay should sort before DB-only rows");
assert(mergedStorylines.items[0].current_context.includes("same turn"), "storyline overlay context not applied");

const worldOverlay = buildWorldRuleOverlay(supervisor);
assert(worldOverlay.length === 3, "world-rule overlay should consume rules/world_rules/confidence_notes");
assert(worldOverlay.some((item) => item.key === "Rule primary"), "rules field lost");
assert(worldOverlay.some((item) => item.key === "Legacy world rule fallback"), "world_rules fallback lost");
assert(worldOverlay.some((item) => item.key === "Confidence note fallback"), "confidence_notes fallback lost");
const mergedWorldRules = mergeWorldRuleOverlay({
  items: [{ scope: "root", key: "Rule primary", source_turn: 3, updated_at: "2026-05-03T00:00:00Z" }]
}, worldOverlay);
assert(mergedWorldRules.count === 3, "world-rule merge count mismatch");
assert(mergedWorldRules.usedOverlay === true, "world-rule usedOverlay missing");
assert(mergedWorldRules.overlayCount === 3, "world-rule overlayCount mismatch");
assert(mergedWorldRules.freshness.mode === "overlay", "world-rule freshness mode mismatch");
assert(mergedWorldRules.freshness.latestTurn === 3, "world-rule source_turn freshness mismatch");
console.log(JSON.stringify({ storylines: mergedStorylines.freshness, worldRules: mergedWorldRules.freshness }));
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node same-turn overlay freshness smoke failed: %v\n%s", err, out)
	}
}

func TestArchiveCenterJSSameTurnOverlayWiringMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function buildStorylineOverlay(supervisorResult)",
		"function buildWorldRuleOverlay(supervisorResult)",
		"[sw.rules, sw.world_rules, sw.confidence_notes]",
		"function mergeStorylineOverlay(storylineResult, overlayItems)",
		"function mergeWorldRuleOverlay(worldRulesResult, overlayItems)",
		"storylineResult = mergeStorylineOverlay(storylineBaseResult, storylineOverlay);",
		"worldRulesResult = mergeWorldRuleOverlay(worldRulesBaseResult, worldRuleOverlay);",
		`usedOverlay: !!storylineResult.usedOverlay,`,
		`freshness: storylineResult.freshness || summarizeOverlayFreshness([], "last_turn"),`,
		`usedOverlay: !!worldRulesResult.usedOverlay,`,
		`freshness: worldRulesResult.freshness || summarizeOverlayFreshness([], "source_turn"),`,
		"var directive = supervisorResult && (supervisorResult.directive || supervisorResult);",
		"var sw = directive.section_world;",
		`if (!sw || typeof sw !== "object" || sw.applies === false) return [];`,
		"storylineResult = await fetchStorylines(chatSessionId);",
		"worldRulesResult = await fetchWorldRules(chatSessionId);",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing H-1 same-turn overlay wiring marker %q", needle)
		}
	}
}

func extractJSFunctionBlockForTest(t *testing.T, src, signature string) string {
	t.Helper()
	start := strings.Index(src, signature)
	if start < 0 {
		t.Fatalf("Archive Center.js missing JS function signature %q", signature)
	}
	brace := strings.Index(src[start:], "{")
	if brace < 0 {
		t.Fatalf("Archive Center.js function %q has no opening brace", signature)
	}
	brace += start

	depth := 0
	var quote byte
	escaped := false
	lineComment := false
	blockComment := false
	for i := brace; i < len(src); i++ {
		c := src[i]
		var next byte
		if i+1 < len(src) {
			next = src[i+1]
		}
		if lineComment {
			if c == '\n' || c == '\r' {
				lineComment = false
			}
			continue
		}
		if blockComment {
			if c == '*' && next == '/' {
				blockComment = false
				i++
			}
			continue
		}
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '/' && next == '/' {
			lineComment = true
			i++
			continue
		}
		if c == '/' && next == '*' {
			blockComment = true
			i++
			continue
		}
		if c == '\'' || c == '"' || c == '`' {
			quote = c
			continue
		}
		if c == '{' {
			depth++
			continue
		}
		if c == '}' {
			depth--
			if depth == 0 {
				return src[start : i+1]
			}
		}
	}
	t.Fatalf("Archive Center.js function %q did not close", signature)
	return ""
}
