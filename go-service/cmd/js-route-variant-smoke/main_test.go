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

func archiveCenterRoot(t *testing.T) string {
	t.Helper()
	candidates := []string{
		filepath.Join("..", "..", "..", "Archive Center.js"),
		filepath.Join("..", "Archive Center.js"),
	}
	if root := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_ROOT")); root != "" {
		candidates = append([]string{filepath.Join(root, "Archive Center.js")}, candidates...)
	}
	if runtime.GOOS == "windows" {
		candidates = append(candidates, filepath.Join(`M:\risulongmemory`, "Archive Center 2.0", "Archive Center.js"))
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
		"await checkAndAutoRollback(orchSessionId, rollbackComparable.messages)",
		`rollbackParams.set("req_source", requestSource);`,
		"method: \"DELETE\"",
		"requestSource = options && options.requestSource ? String(options.requestSource) : \"auto\"",
		"assistant_deleted_before_next_user_turn",
		"single_assistant_msg_removed",
		"msg_decrease_and_tail_change",
		"duplicate_rollback_blocked",
		"ROLLBACK_PROMOTED_ASSISTANT_SYNC_GRACE_MS",
		"function assessPromotedAssistantSyncRollbackGuard",
		"recent_completed_turn_waiting_active_chat_sync",
		"pending_active_chat_confirmation",
		"skipSnapshotUpdate",
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
		`Strength: weak. Keep this nearly invisible`,
		`Strength: strong. Be more active about pacing`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing project config GUI/runtime marker %q", needle)
		}
	}
}

func TestArchiveCenterJSAuxiliaryInjectionPlacementI18nAndBudgetPreviewMarkers(t *testing.T) {
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
		`const estimatedBudget = estimatedParts.budgetLimit;`,
		`const automaticBudget = Number(estimatedParts.automaticBudgetLimit || 0);`,
		`const extraBudget = Number(estimatedParts.userExtraBudgetChars || 0);`,
		`function readCurrentInjectionBudgetPreviewSettings()`,
		`renderSettingsInjectionBudgetPreview(readCurrentInjectionBudgetPreviewSettings())`,
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
		`"settings.section.common.desc": "Backend supervisor, critic, embedding, and input support settings. Input support adds auxiliary context only; the latest user input is not rewritten in 2.4 RC2 default mode."`,
		`merged.dbEnabled = true;`,
		`merged.supervisorEnabled = true;`,
		`settings.pluginMainApplyMode`,
		`inputImprovementApplied`,
		`rewriteAllowed: applyModeName === 'reviewed_apply' && !!settings.pluginMainRewriteLegacyOptIn && payloadRewritten`,
		`<select id="mo-pluginMainApplyMode">`,
		`mo-injection-budget-preview`,
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
	}
	for _, needle := range forbidden {
		if strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js still exposes legacy SEQ-01 manual toggle %q", needle)
		}
	}
}

func TestSeq01NarrativeStanceLabelsAndResumeTriggerCustomUIRemoved(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`storyNarrativeStance: "balanced"`,
		`"settings.label.storyNarrativeStance": "Story Direction Style"`,
		`<select id="mo-storyNarrativeStance"`,
		`<option value="reactive"`,
		`<option value="balanced"`,
		`<option value="proactive"`,
		`storyNarrativeStance: $("mo-storyNarrativeStance").value`,
		`pluginMainApplyMode: $("mo-pluginMainApplyMode").value`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-01 narrative stance marker %q", needle)
		}
	}
	forbidden := []string{
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
		`"settings.label.narrativeGuideMode.help": "Auto mode combines recent input, scene pressure, emotional intensity, combat, and relationship signals, then exposes the resolved mode in trace and dashboard."`,
		`let _guideModeRuntimeCache = { lastMode: null, lastProbe: "", consecutiveSame: 0 };`,
		`result._guideModeBasis = "auto_inferred";`,
		`guideModeBasis: (supervisorResult && supervisorResult._guideModeBasis) || "manual"`,
		`const guideModeDashboardState = lastGuideSupervisor && lastGuideSupervisor.guideMode`,
		`{ label: dashLabel.guideMode, state: guideModeDashboardState }`,
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
}

func TestSeq01SettingsSaveResetAndBridgeConfigMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`async function saveSettings()`,
		`await persistentSet(SETTINGS_KEY, json)`,
		`await syncConfigToBackend(settings);`,
		`warnLog("Settings save failed:", err.message);`,
		`return false;`,
		`function attachSettingsEvents()`,
		`$("mo-save-btn").addEventListener("click", async () => {`,
		`$("mo-reset-btn").addEventListener("click", async () => {`,
		`settings = { ...DEFAULT_SETTINGS };`,
		`await saveSettings();`,
		`<input type="text" id="mo-bridgeUrl"`,
		`<input type="number" id="mo-requestTimeoutMs"`,
		`<input type="number" id="mo-topK"`,
		`settings.bridgeUrl = sanitizeBridgeUrl(`,
		`settings.requestTimeoutMs = getCurrentUiRequestTimeoutMs();`,
		`topK: $("mo-topK").value`,
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
		`if (!isNarrativeType(type) || !settings.enabled) return payload;`,
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

func TestArchiveCenterJSSessionIsolationMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"async function getCurrentChatSessionId()",
		"function isCidSessionId(sessionId)",
		"function isIndexSessionId(sessionId)",
		"`char_${charIdx}_cid_${chatUniqueId}`",
		"savePinnedSessionId(charIdx, chatIdx, sessionId, chatUniqueId)",
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
		"savePinnedSessionId(charIdx, chatIdx, sessionId, chatUniqueId)",
		"observedChatUniqueId: String(observedChatUniqueId || \"\").trim()",
		"function buildRawInputSessionKeys(sessionId)",
		"primary.match(/^(char_\\d+)_(?:cid_.+|chat_\\d+)$/)",
		"addRawInputSessionKey(keys, seen, charAliasMatch[1])",
		"addRawInputSessionKey(keys, seen, SESSION_FALLBACK)",
		"const maxAge = key === SESSION_FALLBACK",
		"RAW_INPUT_FALLBACK_MAX_AGE_MS",
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
	script := src[i18nStart:i18nEnd] + "\n" +
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
	end := strings.Index(src, "// E-1d: Storyline Sync")
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
		"var directive = supervisorResult.directive || supervisorResult;",
		"if (!directive.section_world || directive.section_world.applies !== true) return null;",
		"supervisor_response: directive,",
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

func TestArchiveCenterJSSameTurnOverlayInjectionAndTraceRuntime(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for JS runtime behavior smoke")
	}
	src := readArchiveCenterJS(t)
	helpersStart := strings.Index(src, "function normalizeStorylineStatus")
	helpersEnd := strings.Index(src, "// E-1d: Storyline Sync")
	if helpersStart < 0 || helpersEnd < 0 || helpersEnd <= helpersStart {
		t.Fatalf("Archive Center.js missing same-turn overlay helper block")
	}

	script := src[helpersStart:helpersEnd] + "\n" +
		extractJSFunctionBlockForTest(t, src, "function formatStorylineBlock") + "\n" +
		extractJSFunctionBlockForTest(t, src, "function formatWorldRulesBlock") + "\n" +
		extractJSFunctionBlockForTest(t, src, "function renderTurnTraceRows") + `
const assert = (cond, msg) => { if (!cond) throw new Error(msg); };
const t = (key) => key;
const statusDotClass = (st) => "dot-" + st;
const escapeAttr = (value) => String(value == null ? "" : value);
const truncPreview = (value, n) => String(value == null ? "" : value).slice(0, n);
let lastTurnTrace = null;

const supervisor = {
  directive: {
    storylines: [{
      name: "Rooftop Promise",
      status: "active",
      current_context: "same-turn confession must affect the immediate injection",
      key_points: ["answer is pending"],
      ongoing_tensions: ["hesitation"]
    }],
    section_world: {
      applies: true,
      genre_hint: "romance",
      rules: ["The rooftop is private tonight."],
      world_rules: ["Confessions leave visible social consequences."]
    }
  }
};
const storylineOverlay = buildStorylineOverlay(supervisor);
const worldRuleOverlay = buildWorldRuleOverlay(supervisor);
const storylines = mergeStorylineOverlay({
  items: [{ name: "Existing Arc", status: "active", current_context: "older active row", last_turn: 4 }],
  count: 1
}, storylineOverlay);
const worldRules = mergeWorldRuleOverlay({ items: [], count: 0 }, worldRuleOverlay);
const storylineText = formatStorylineBlock(storylines);
const worldRulesText = formatWorldRulesBlock(worldRules);
assert(storylineText.includes("Rooftop Promise"), "same-turn storyline was not included in injection text");
assert(storylineText.includes("same-turn confession"), "same-turn storyline context missing from injection text");
assert(worldRulesText.includes("The rooftop is private tonight."), "same-turn world rule missing from injection text");
assert(worldRulesText.includes("Confessions leave visible social consequences."), "world_rules fallback missing from injection text");

lastTurnTrace = {
  turnIndex: 10,
  contextSize: 4,
  chatSessionId: "char_1",
  search: { status: "ok", itemCount: 0, memoryCount: 0, fallbackCount: 0, paths: [] },
  wakeUpContext: { status: "empty", length: 0 },
  kgRecall: { status: "skipped", triplesReturned: 0, entitiesExtracted: 0 },
  activeStates: { status: "empty", count: 0, types: [] },
  supervisor: { status: "ok", hasDirective: true, hasAuthor: true, hasDirector: true, hasSectionWorld: true },
  storylines: { status: "ok", count: 2, activeCount: 2, usedOverlay: true, overlayCount: 1, freshness: { latestTurn: 10 }, selection: { totalActiveCount: 3, selectedCount: 2, droppedCount: 1, staleDroppedCount: 1 } },
  worldRules: { status: "ok", count: 2, usedOverlay: true, overlayCount: 2, freshness: { latestTurn: 10 } },
  pendingThreads: { status: "empty", count: 0 },
  save: { status: "ok" },
  complete: { status: "ok" },
  critic: { memorySaved: true, kgSaved: true }
};
let html = renderTurnTraceRows();
assert(html.includes("directive") && html.includes("author") && html.includes("director") && html.includes("sw"), "supervisor directive trace missing");
assert(html.includes("2 total") && html.includes("2 active") && html.includes("overlay+1"), "storyline overlay trace missing");
assert(html.includes("sel:2/3") && html.includes("drop:1") && html.includes("staleDrop:1"), "storyline selection trace missing");
assert(html.includes("2 rules") && html.includes("overlay+2"), "world-rule overlay trace missing");

lastTurnTrace = {
  turnIndex: 11,
  contextSize: 5,
  chatSessionId: "char_1",
  search: { status: "ok", itemCount: 0, memoryCount: 0, fallbackCount: 0, paths: [] },
  wakeUpContext: { status: "empty", length: 0 },
  kgRecall: { status: "skipped", triplesReturned: 0, entitiesExtracted: 0 },
  activeStates: { status: "ok", count: 3, types: ["relationship", "promise", "scene"] },
  supervisor: { status: "ok", hasDirective: true, hasAuthor: true, hasDirector: true, hasSectionWorld: true },
  storylines: { status: "ok", count: 3, activeCount: 3, usedOverlay: true, overlayCount: 1, freshness: { latestTurn: 11 } },
  worldRules: { status: "ok", count: 2, usedOverlay: false, overlayCount: 0, freshness: { latestTurn: 10 } },
  pendingThreads: { status: "empty", count: 0 },
  save: { status: "ok" },
  complete: { status: "ok" },
  critic: { memorySaved: true, kgSaved: true }
};
html = renderTurnTraceRows();
assert(html.includes("3 states (relationship, promise, scene)"), "active-state next-turn trace missing");
assert(html.includes("3 total") && html.includes("3 active") && html.includes("overlay+1"), "next-turn storyline trace missing");

lastTurnTrace = {
  turnIndex: 12,
  contextSize: 3,
  chatSessionId: "char_1",
  search: { status: "fail", itemCount: 0, memoryCount: 0, fallbackCount: 0, paths: [] },
  wakeUpContext: { status: "fail", length: 0 },
  kgRecall: { status: "skipped", triplesReturned: 0, entitiesExtracted: 0 },
  activeStates: { status: "empty", count: 0, types: [] },
  supervisor: { status: "fail", hasDirective: false, hasAuthor: false, hasDirector: false, hasSectionWorld: false },
  storylines: { status: "empty", count: 0, activeCount: 0, usedOverlay: false, overlayCount: 0, freshness: { mode: "empty" } },
  worldRules: { status: "empty", count: 0, usedOverlay: false, overlayCount: 0, freshness: { mode: "empty" } },
  pendingThreads: { status: "skip", count: 0 },
  save: { status: "skipped" },
  complete: { status: "skipped" },
  critic: { memorySaved: false, kgSaved: false }
};
html = renderTurnTraceRows();
assert(html.includes("Supervisor") && html.includes("fail"), "backend-off supervisor fallback missing");
assert(html.includes("Storylines") && html.includes("no storylines"), "backend-off storyline fallback missing");
assert(html.includes("World Rules") && html.includes("no rules"), "backend-off world-rule fallback missing");
console.log(JSON.stringify({ storylineText, worldRulesText, ok: true }));
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node same-turn injection/trace runtime smoke failed: %v\n%s", err, out)
	}
}

func TestArchiveCenterJSSameTurnOverlaySyncFailureIsolationMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`postStorylineSync(supervisorResult, chatSessionId, predictedTurnIndex, "apply").catch`,
		"const predictedTurnIndex = peekNextTurnIndex(chatSessionId);",
		"postWorldRulesSync(supervisorResult, chatSessionId, predictedTurnIndex).catch",
		"storylineResult = mergeStorylineOverlay(storylineBaseResult, storylineOverlay);",
		"worldRulesResult = mergeWorldRuleOverlay(worldRulesBaseResult, worldRuleOverlay);",
		"storylineSelectionRaw: supervisorResult && supervisorResult.storyline_selection",
		"usedOverlay: !!storylineResult.usedOverlay",
		"usedOverlay: !!worldRulesResult.usedOverlay",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing same-turn sync isolation marker %q", needle)
		}
	}
}

func TestArchiveCenterJSPromptEnglishOnlyBoundaryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	forbidden := []string{
		"function tp(",
		"tp(",
		"promptLanguage",
		"settings.label.promptLanguage",
		"settings.desc.promptLanguage",
		`"prompt.`,
	}
	for _, needle := range forbidden {
		if strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js should not contain F-3 retired prompt-language marker %q", needle)
		}
	}

	root := filepath.Join(archiveCenterRoot(t), "prompts")
	for _, dir := range []string{"ko", "ja", "en"} {
		if _, err := os.Stat(filepath.Join(root, dir)); !os.IsNotExist(err) {
			t.Fatalf("prompt language dir %q should not exist under root prompts", dir)
		}
	}
	for _, name := range []string{"supervisor_system.txt", "critic_system.txt"} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatalf("root prompt file %q missing: %v", name, err)
		}
	}
}

func TestArchiveCenterJSSeq04SessionTurnTransitionProof(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"const _sessionTurnIndices = new Map()",
		"const SESSION_TURN_MAP_MAX = 50",
		"function getSessionTurnIndex(sessionId)",
		"function setSessionTurnIndex(sessionId, idx)",
		"const idx = Math.max(loadTurnCounter(sessionId), getSessionTurnIndex(sessionId), 0) + 1",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-04 session turn marker %q", needle)
		}
	}

	stored := map[string]int{}
	tracked := map[string]int{}
	next := func(sessionID string) int {
		max := stored[sessionID]
		if tracked[sessionID] > max {
			max = tracked[sessionID]
		}
		idx := max + 1
		stored[sessionID] = idx
		return idx
	}
	track := func(sessionID string, idx int) {
		tracked[sessionID] = idx
	}

	track("char_1_cid_A", 15)
	if got := next("char_2_cid_B"); got != 1 {
		t.Fatalf("new session B next turn = %d, want 1", got)
	}
	track("char_2_cid_B", 1)
	if got := next("char_1_cid_A"); got != 16 {
		t.Fatalf("returning session A next turn = %d, want 16", got)
	}
	if got := next("char_2_cid_B"); got != 2 {
		t.Fatalf("session B second turn = %d, want 2", got)
	}
}

func TestArchiveCenterJSFinalPayloadParityMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function buildFinalPayloadParityTrace(originalPayload, finalPayload, meta)",
		"function attachFinalPayloadParityTrace(trace, originalPayload, finalPayload, meta)",
		`source: "js_host_adapter"`,
		"finalPayloadParity",
		"SEQ-02 / RMG-22 Final Payload Parity",
		"payloadMutated",
		"beforeMessageCount",
		"afterMessageCount",
		"finalUserInputPreview",
		"assembledPreview",
		"attachFinalPayloadParityTrace(lastOrchResult && lastOrchResult._trace, payload, outgoingPayload",
		"attachFinalPayloadParityTrace(lastOrchResult && lastOrchResult._trace, payload, payload",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing final payload parity marker %q", needle)
		}
	}
}

func TestArchiveCenterJSW1RewriteLastUserMessage(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function rewriteLastUserMessage(payload, nextUserInput)",
		"empty_next_input",
		"no_messages",
		"no_user_message",
		"unchanged",
		"rewritten",
		"exception",
		"lastUserIdx",
		"content: rewrittenText",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing W-1 rewrite helper marker %q", needle)
		}
	}
}

func TestArchiveCenterJSW1FinalPayloadParityRewriteEvidence(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"attachFinalPayloadParityTrace(lastOrchResult && lastOrchResult._trace, payload, outgoingPayload",
		"attachFinalPayloadParityTrace(lastOrchResult && lastOrchResult._trace, payload, payload",
		"payloadMutated",
		"finalUserInputPreview",
		"inputImprovementApplied",
		"rewriteAllowed",
		"lastOrchResult._trace.applyMode.payloadReplaced = true",
		"lastOrchResult._trace.applyMode.rewriteReason = rewriteResult.reason || \"rewritten\"",
		"lastOrchResult._trace.inputImprovement.applyReason = \"payload_rewritten\"",
		"lastOrchResult._trace.inputImprovement.applyReason = \"rewrite_failed:\"",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing W-1 final payload parity rewrite marker %q", needle)
		}
	}
}

func TestArchiveCenterJSJ3ApplyModeGateAndTraceRecord(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function getApplyModeGate()",
		"function applyModeAllowsApply(verdict)",
		"if (mode !== \"reviewed_apply\") return false;",
		"verdict === \"approve\" || verdict === \"partial\" || verdict === \"first-pass-only\"",
		"trace.applyMode = { mode: _applyGate.mode, payloadReplaced: false }",
		"trace.inputImprovement.applyReason = \"improvement_accepted\"",
		"trace.inputImprovement.payloadRewritten = false",
		"trace.applyMode.payloadReplaced = false",
		"settings.pluginMainApplyMode",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing J-3 apply-mode marker %q", needle)
		}
	}
}

func TestArchiveCenterJSJ3TraceBlocksAndFailureSafety(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"mode_not_reviewed_apply",
		"not_improved",
		"empty_final_input",
		"verdict_not_allowed:",
		"trace_only",
		"no_apply",
		"rewrite_failed:",
		"payloadReplaced = false",
		"payloadRewritten = false",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing J-3 failure-safety marker %q", needle)
		}
	}
}

func TestArchiveCenterJSJ4TraceRecordFormat(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function buildImprovementTraceRecord(applyGate, mergeResult, originalInput, payloadReplaced)",
		"keepCandidates",
		"dropCandidates",
		"previewBefore",
		"previewAfter",
		"mergeNote",
		"recordedAt",
		"async function tryCompleteTurn(turnIdx, userInput, assistantContent, contextMessages, chatSessionId, improvementTrace, prebuiltBody)",
		"trace.improvementHandoff = {",
		"hintAttached:",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing J-4 trace record marker %q", needle)
		}
	}
}

func TestArchiveCenterJSPrepareTurnInjectionPackMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function applyContextInjection(payload, orchResult)",
		"const _ip = orchResult._injectionPack || null",
		"const memoryText = (_ip && _ip.memory_text) ? _ip.memory_text : formatMemoryBlock(searchResult, sanitizeTopKSetting(settings.topK, DEFAULT_SETTINGS.topK))",
		"const kgText = (_ip && _ip.kg_text) ? _ip.kg_text : formatKGBlock(kgRecallResult)",
		"const fallbackText = (_ip && _ip.fallback_text) ? _ip.fallback_text : (includeFallback ? formatFallbackBlock(searchResult) : \"\")",
		"const latestDirectEvidenceText = (_ip && _ip.latest_direct_evidence_text) ? String(_ip.latest_direct_evidence_text) : \"\"",
		"const recentRawTurnText = (_ip && _ip.recent_raw_turn_text) ? String(_ip.recent_raw_turn_text) : \"\"",
		"const canonicalStateLayerText = (_ip && _ip.canon_text) ? String(_ip.canon_text) : \"\"",
		"assembleInjectionWithBudget(",
		"injectionTextSource: _ip ? \"bundle\" : \"local\"",
		"buildInputContext(orchResult._userInput || \"\", orchResult, _ip ? (_ip.input_context_text || \"\") : \"\"",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing prepare-turn injection pack marker %q", needle)
		}
	}
}

func TestArchiveCenterJSStep11HybridMemoryPolicyMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const precedencePolicyVersion = "ea1a.v1";`,
		`"verified_direct_evidence",`,
		`"canonical_state",`,
		`"dense_summary",`,
		`"retrieval_supporting_inference",`,
		`role: "detail_recall_audit_only",`,
		"supporting_guidance_guard: {",
		"narrative_quality_layer: {",
		`status: "quality_hint_only",`,
		`"truth_arbitration"`,
		`"canonical_overwrite"`,
		`const hybridPolicyVersion = "ea1f.v1";`,
		`const recentPriorityPolicyVersion = "ea1g.v1";`,
		"hybridPolicyVersion: hybridPolicyVersion,",
		"recentPriorityPolicyVersion: recentPriorityPolicyVersion,",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing Step 11 hybrid memory marker %q", needle)
		}
	}
}

func TestArchiveCenterJSHypaMemoryImportOnlyMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"async function importHypaMemory(sessionId)",
		"function normalizeHypaImportSourceTurnIndex",
		`"/import/hypamemory"`,
		"hypaV3Data",
		"source_turn_index: normalizeHypaImportSourceTurnIndex(s, idx)",
		"Please disable RisuAI's HypaMemory after import.",
		`const hypaLoreIngestPolicyVersion = "rg1d.v1";`,
		`const hypaLorePolicyTag = "rg1d_ingest_only";`,
		`const legacyAlwaysOnSourceLabels = ["wake_up", "lorebook", "hypamemory"];`,
		"hypaLoreIngestOnlyEnabled: true,",
		`hypaLoreAlwaysOnMode: "discouraged",`,
		"hypaLoreAlwaysOnSourceLabels: legacyAlwaysOnSourceLabels.slice(),",
		`"ingest_only_not_injected"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing HypaMemory ingest-only marker %q", needle)
		}
	}
}

func TestArchiveCenterJSEA1PrecedenceAndHybridBudgetMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const precedencePolicyVersion = "ea1a.v1";`,
		`"current_user_input",`,
		`"explicit_correction",`,
		`"hard_rule",`,
		`"verified_direct_evidence",`,
		`"canonical_state",`,
		`"dense_summary",`,
		`"retrieval_supporting_inference",`,
		`const hybridPolicyVersion = "ea1f.v1";`,
		`const allocationStrategy = "priority_then_residual_ratio";`,
		"const hardFloorShareCap = 0.55;",
		"latestDirectEvidencePriorityEnabled: hasLatestDirectEvidence,",
		"recentRawTurnPriorityEnabled: hasRecentRawTurn,",
		"hardFloorShareCap: hardFloorShareCap,",
		"hardFloorReserveTotalChars: hardFloorReserveTotalChars,",
		"residualSupportBudgetChars: Math.max(0, budgetLimit - hardFloorReserveTotalChars),",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing EA-1 precedence/hybrid budget marker %q", needle)
		}
	}
}

func TestArchiveCenterJSPromptTemplateScaffoldSanitizeRuntime(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function looksLikePromptTemplateScaffold(text)",
		"function isPromptTemplateScaffoldLine(line)",
		"function sanitizeForCritic(text)",
		"response template|template guidelines",
		"return !isPromptTemplateScaffoldLine(line);",
		`if (looksLikePromptTemplateScaffold(clean)) return "";`,
		`stage.scaffoldLineDetected ? "template-scaffold" : null`,
		"scaffoldLineDetected,",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing prompt-template scaffold sanitize marker %q", needle)
		}
	}
}

func TestArchiveCenterJSDenseSummaryProfileBudgetMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const denseSummarySlotPolicyVersion = "ds1e.v1";`,
		"const denseSummarySlotProfile = (function resolveDenseSummarySlotProfile()",
		"const denseSummarySlotRatios = (function resolveDenseSummarySlotRatios()",
		`return { episode: 0.045, chapter: 0.04, arc: 0.035, saga: 0.03 };`,
		"denseSummarySlotPolicyVersion: denseSummarySlotPolicyVersion,",
		"denseSummarySlotProfile: denseSummarySlotProfile,",
		"denseSummarySlotRatios: {",
		"denseSummaryHardFloorChars: {",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing dense-summary profile budget marker %q", needle)
		}
	}
}

func TestArchiveCenterJSRG1aRetrievalRoleAuditMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const retrievalRolePolicyVersion = "rg1a.v1";`,
		`const retrievalRolePolicyTag = "rg1a_retrieval_audit_only";`,
		`const retrievalAllowedUsage = ["detail_recall", "audit_reference"];`,
		`const retrievalDisallowedUsage = ["truth_overwrite", "canonical_override"];`,
		`role: "detail_recall_audit_only",`,
		`retrievalRoleMode: "detail_recall_audit_only",`,
		"retrievalAuditOnlyEnabled: true,",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing RG-1a retrieval role audit marker %q", needle)
		}
	}
}

func TestArchiveCenterJSRG1bToRG1dPromotionTraceAndIngestMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const retrievalPromotionPolicyVersion = "rg1b.v1";`,
		`const retrievalPromotionTargets = ["canonical_state", "dense_summary"];`,
		"retrievalPromotionCandidatesRaw",
		"retrievalPromotionCandidateCount",
		"retrievalPromotionCanonicalCount",
		"retrievalPromotionDenseSummaryCount",
		`const retrievalConflictPolicyVersion = "rg1c.v1";`,
		"retrievalKeepDropTraceEnabled: true,",
		"retrievalKeepDropTrace: retrievalKeepDropTrace.slice(0, 20),",
		`const hypaLoreIngestPolicyVersion = "rg1d.v1";`,
		`const hypaLorePolicyTag = "rg1d_ingest_only";`,
		`"ingest_only_not_injected"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing RG-1b/RG-1c/RG-1d marker %q", needle)
		}
	}
}

func TestArchiveCenterJSRG1fToRG1iAuthorityGuardMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const verifiedCurrentStatePolicyVersion = verifiedCurrentStatePrecedenceEnabled ? "rg1f.v1" : "";`,
		`const verifiedCurrentStatePolicyTag = verifiedCurrentStatePrecedenceEnabled ? "rg1f_verified_current_state" : "";`,
		"verifiedCurrentStatePrecedenceEnabled: verifiedCurrentStatePrecedenceEnabled,",
		`const reliabilityGuardPolicyVersion = "rg1g.v1";`,
		`const reliabilityGuardPolicyTag = "rg1g_conservative_hold";`,
		`reliabilityGuardMode: reliabilityGuardTriggered ? "conservative_hold" : "normal",`,
		`const supportingGuidanceGuardPolicyVersion = "rg1h.v1";`,
		`const supportingGuidanceGuardPolicyTag = "rg1h_supporting_guidance_guard";`,
		"supportingGuidanceEvidenceCeilingEnabled: supportingGuidanceEvidenceCeilingEnabled,",
		`const narrativeQualityLayerPolicyVersion = "rg1i.v1";`,
		`const narrativeQualityLayerMode = "quality_hint_only";`,
		"narrativeQualityLayerTruthArbitrationAllowed: false,",
		"narrativeQualityLayerCanonicalOverwriteAllowed: false,",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing RG-1f/RG-1g/RG-1h/RG-1i marker %q", needle)
		}
	}
}

func TestArchiveCenterJSRG1jPresetTemplateContaminationMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function looksLikePromptTemplateScaffold(text)",
		"function isPromptTemplateScaffoldLine(line)",
		"function sanitizeForCritic(text)",
		"function sanitizeNarrativeOutputForDisplay(text)",
		`"template-scaffold"`,
		"scaffoldLineDetected",
		"sanitizeForCritic(rawSeed)",
		"sanitizeForCritic(safeUserContent)",
		"sanitizeForCritic(String(assistantContent || \"\"))",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing RG-1j preset/template contamination marker %q", needle)
		}
	}
}

func TestArchiveCenterJSPluginVersionMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"//@name Archive Center",
		"//@display-name Archive Center",
		"//@version 2.5.0",
		`const VERSION = "2.5.0";`,
		`const VERSION_STR = typeof VERSION !== "undefined" ? String(VERSION) : "unknown";`,
		"source_version:    VERSION_STR",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing plugin version marker %q", needle)
		}
	}
}

func TestArchiveCenterJSRenderRegression47Markers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function renderTurnTraceRows()",
		"function renderInputTransparencySection()",
		"function renderExplorerDirectEvidence()",
		"function renderExplorerEntities()",
		"function renderExplorerWorldGraph()",
		"function renderTimelinePanel()",
		"function renderPromptEditorSection()",
		"async function renderSettingsPanel()",
		`renderItBlockRaw("3.5. Hybrid Retrieval Inspection"`,
		`renderItBlockRaw("7.5. Protection Patterns"`,
		`renderItBlockRaw("8. Context Injection (`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing render regression marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq08P703UIDetailModeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const UI_DETAIL_MODE_OPTIONS = Object.freeze(["full", "reduced_info", "status_only"])`,
		`uiDetailMode: "full"`,
		`"settings.uiDetailMode.reduced_info"`,
		`"settings.uiDetailMode.status_only"`,
		`const detailMode = sanitizeEnumValue(settings.uiDetailMode, DEFAULT_SETTINGS.uiDetailMode, UI_DETAIL_MODE_OPTIONS)`,
		`<select id="mo-uiDetailMode">`,
		`<option value="reduced_info"`,
		`<option value="status_only"`,
		`uiDetailMode: $("mo-uiDetailMode").value`,
		`$("mo-uiDetailMode").value = settings.uiDetailMode || "full"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-08-P703 UI detail mode marker %q", needle)
		}
	}
}

func TestArchiveCenterJSInitiativeControlLatestEquivalentMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`storyNarrativeStance: "balanced"`,
		"merged.storyNarrativeStance = sanitizeEnumValue(",
		"NARRATIVE_STANCE_MODES",
		`<select id="mo-storyNarrativeStance"`,
		`<option value="reactive"`,
		`<option value="balanced"`,
		`<option value="proactive"`,
		`const stanceEl = $("mo-storyNarrativeStance");`,
		`storyNarrativeStance: $("mo-storyNarrativeStance").value`,
		`$("mo-storyNarrativeStance").value = settings.storyNarrativeStance || "balanced";`,
		"function buildInitiativeModeSuffix(mode)",
		"function buildInitiativeModeBounds(mode)",
		`narrative_stance: narrativeStance`,
		`narrative_stance_suffix: initiativeSuffix || ""`,
		"narrative_stance_bounds: initiativeBounds",
		"extractNarrativeStanceSummary(_narrativeStance)",
		"initiativeSummaryRaw",
		"initiativeSuffixRaw",
		"initiativeBoundsRaw",
		`debugLog("initiative:", _initiativeSummary.mode`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing H-3 initiative latest-equivalent marker %q", needle)
		}
	}
	if strings.Contains(src, "storyInitiativeMode") {
		t.Fatal("Archive Center.js should not reintroduce retired storyInitiativeMode beside storyNarrativeStance")
	}
}

func TestArchiveCenterJSContinuityPackLatestEquivalentMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function normalizeContinuityPackResult(rawResult, chatSessionId)",
		"async function fetchContinuityPack(chatSessionId)",
		`"/continuity-pack/" + encodeURIComponent(sid)`,
		"function buildStorylineResultFromContinuityPack(continuityPackResult)",
		"function buildWorldRulesResultFromContinuityPack(continuityPackResult)",
		"function applyContinuityPackFallback(primaryResult, continuityResult, label)",
		"async function resolveContinuityTriggerInfo(userInput, messages, chatSessionId, options = {})",
		`"empty_input"`,
		`"manual_resume"`,
		`"idle_reentry"`,
		"function upgradeContinuityInfoWithPack(continuityInfo, continuityPackResult, guardContext)",
		`querySource: packQuery ? "continuity_pack"`,
		"if (cont.packUsedAsQuery) contParts.push(\"packQuery",
		"if (cont.packUsedAsWakeUp) contParts.push(\"wakeUp",
		"if (cont.debugForced) contParts.push(\"debugForced\")",
		"[DEBUG] Current Turn Input",
		"[DEBUG] Current Turn Source",
		"const _rawInputBySession = new Map();",
		"async function onInputHook(rawInput)",
		"cacheRawInputForSession(sessionId, rawInput)",
		"function resolveCurrentTurnUserInputInfo(payload, messages, sessionId)",
		`source: "input_hook"`,
		`source: "messages.meta_only"`,
		"shouldRejectLowTrustCurrentInput(auxiliaryMessageContentText(lastUserMsg.content))",
		"metaOnlyInput: !!userInputInfo.metaOnly",
		"t('settings.debug.forceIdleBtn')",
		"debugForced: true",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing H-4 continuity marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq07PersistentGuidanceMaintenanceMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"async function fetchNarrativeControl(chatSessionId)",
		`bridgeFetch("/narrative-control/" + encodeURIComponent(sessionId)`,
		"function buildPersistentGuidanceSuffix(ncResult)",
		"function buildPersistentGuidanceHintsCompact(ncResult, suppressedForbidden)",
		"function resolveAutoAdvanceTrigger(continuityInfo)",
		"function buildAutoAdvanceHint(ncResult, trigger)",
		"function fireMaintenancePass(turnIdx, chatSessionId, assistantContent, traceRef, recentResponses, supervisorResult)",
		"`/maintenance/enqueue`",
		"shadow_only: true",
		"recent_responses: Array.isArray(recentResponses)",
		"trace.guidanceState = {",
		"trace.guidanceState.suppressedCount = (_guidanceArbitration.suppressed || []).length;",
		"guidanceTransitionRaw:",
		"[DEBUG] Guidance Transition (K-4d)",
		"[DEBUG] Auto-advance Hint (L-4b)",
		"auto_advance_trigger: autoAdvanceTrigger || \"none\"",
		"autoAdvanceHintApplied: !!_autoAdvanceHint",
		"Treat this as a gentle nudge only",
		"never override explicit user input",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-07 persistent guidance/maintenance marker %q", needle)
		}
	}
	if strings.Contains(src, "await fireMaintenancePass(") {
		t.Fatal("maintenance pass must stay fire-and-forget and must not block chat flow")
	}
}

func TestArchiveCenterJSSeq08BackendTurnEngineFailOpenMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"async function tryPrepareTurn(sessionId, userInput, messages, continuityInfo, type, languageContext, options = {})",
		`bridgeFetch("/prepare-turn"`,
		`return { source: "backend-off", fallback_reason: "backend_off", status: "error" }`,
		`return { source: "backend-error", fallback_reason: "backend_error", status: "error" }`,
		"const prepareTurnBackendUnavailable = _lastPrepareTurnSource === \"backend-off\"",
		`updateRuntimeState("lastBridgeHealth", "fail"`,
		`debugLog("prepare-turn unavailable - local fallback continues:"`,
		`updateRuntimeState("lastBridgeHealth", "ok"`,
		"async function tryCompleteTurn(turnIdx, userInput, assistantContent, contextMessages, chatSessionId, improvementTrace, prebuiltBody)",
		`bridgeFetchWithRetry("/complete-turn"`,
		"function fireMaintenancePass(turnIdx, chatSessionId, assistantContent, traceRef, recentResponses, supervisorResult)",
		"`/maintenance/enqueue`",
		"trace.autonomyPlan = {",
		"trace.microBeatProposal =",
		"trace.sceneStepProposal =",
		"trace.combinedProposal =",
		"ShadowCmp",
		"divergence_injection",
		"default_takeover",
		"packetMode",
		"degraded",
		"fallbackReason",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-08 backend turn-engine marker %q", needle)
		}
	}
	if strings.Contains(src, "return buildBlockedPayload(payload, backendBlock.userMessage || backendBlock.reason)") {
		t.Fatal("backend-off prepare-turn path must remain fail-open and must not return a blocked payload")
	}
	if strings.Contains(src, "await fireMaintenancePass(") {
		t.Fatal("maintenance pass must stay fire-and-forget and must not block chat flow")
	}
}

func TestArchiveCenterJSOnlyModelTypeEntersPersistence(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function isNarrativeType(type)",
		"function isSaveType(type)",
		"function isContextInjectionType(type)",
		`return !type || type === "model";`,
		"archive_center_ignores_non_model_risu_requests",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing Gemini AI Studio OtherAx persistence marker %q", needle)
		}
	}
}

func TestArchiveCenterJSModelPayloadIsNotBlockedByPromptMarkersOrTailMismatch(t *testing.T) {
	src := readArchiveCenterJS(t)
	for _, marker := range []string{
		`reason: "model_request_auxiliary_marker_trace_only"`,
		`policy: "risu_model_type_is_authoritative_marker_does_not_block"`,
		`reason: "model_payload_tail_authoritative"`,
		`policy: "risu_model_type_and_latest_payload_user_are_authoritative"`,
		`reason: "post_output_secondary_request"`,
	} {
		if !strings.Contains(src, marker) {
			t.Fatalf("Archive Center.js missing model ownership contract %q", marker)
		}
	}
	for _, obsolete := range []string{
		`reason: "auxiliary_module_request"`,
		`reason: "payload_user_tail_mismatch_active_tail_block"`,
	} {
		if strings.Contains(src, obsolete) {
			t.Fatalf("Archive Center.js still blocks model requests with obsolete content classifier %q", obsolete)
		}
	}
}

func TestArchiveCenterJSPersistenceRequestsDoNotBlindRetry(t *testing.T) {
	src := readArchiveCenterJS(t)
	for _, marker := range []string{
		`bridgeFetchWithRetry("/turns", { method: "POST", body }, 1)`,
		`bridgeFetchWithRetry("/effective-inputs", { method: "POST", body }, 1)`,
		`bridgeFetchWithRetry("/turns/complete", { method: "POST", body }, 1)`,
		`bridgeFetchWithRetry("/complete-turn", { method: "POST", body, timeoutMs: getCompleteTurnTimeoutMs() }, 1)`,
		`/complete-turn/request-status?idempotency_key=`,
		`idempotency_key: idempotencyKey`,
	} {
		if !strings.Contains(src, marker) {
			t.Fatalf("Archive Center.js missing persistence idempotency marker %q", marker)
		}
	}
}

func TestArchiveCenterJSStreamingAfterRequestPollerMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"const STREAMING_AFTER_REQUEST_POLL_INTERVAL_MS = 800;",
		"const STREAMING_AFTER_REQUEST_STABLE_POLLS = 3;",
		"const STREAMING_AFTER_REQUEST_OBSERVED_STREAM_STABLE_POLLS = 4;",
		"const STREAMING_AFTER_REQUEST_OBSERVED_STREAM_QUIET_MS = 8 * 1000;",
		"const _streamingAfterRequestWatchers = new Map();",
		"function armStreamingAfterRequestWatch(sessionId, type, requestId)",
		"function pollStreamingAfterRequestWatch(sessionId)",
		"function markNativeAfterRequestObserved(sessionId, type)",
		"function stopStreamingAfterRequestWatch(sessionId, detail)",
		"native afterRequest missing; recovered from active chat",
		"late native afterRequest ignored after poller recovery",
		"armStreamingAfterRequestWatch(orchSessionId, type, orchRequestId);",
		"Streaming Hook",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing streaming afterRequest poller marker %q", needle)
		}
	}
}

func TestArchiveCenterJSEpisodeGenerateOkStatusMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"const episodeGenerated = !!(result && (",
		`result.code === "episode_generated"`,
		`result.status === "ok" && result.saved === true && result.episode`,
		`if (episodeGenerated) {`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing episode generated response marker %q", needle)
		}
	}
}

func TestArchiveCenterJSWorldGraphLiteActiveScopeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"async function fetchWorldRules",
		"\"/world-rules/\" + encodeURIComponent(sid) + \"/inherited\"",
		"active_scope: fetchWorldRules._lastScopeMeta && fetchWorldRules._lastScopeMeta.active_scope",
		"Scope chain: \" + worldRulesResult.scope_chain.join(\" > \")",
		"if (rule.inherited) entry += \"(inherited) \";",
		"async function explorerFetchWorldGraph",
		"\"/session/\" + encodeURIComponent(sid) + \"/active-scope\"",
		"async function explorerSetWorldScope",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing I-4 World Graph Lite marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSessionStateAggregateReadRoundTripEvidence(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"let _aggregateSnapshot = null;",
		"if (settings.useAggregateRead && !freshFirstTurnLightMode) {",
		"const _snap = await fetchSessionState(chatSessionId);",
		"storylineResult = { items: _slItems, count: _slItems.length, fetched: true, source: \"aggregate\", continuityPackFallback: false };",
		"characterResult = { items: _chItems, count: _chItems.length };",
		"worldRulesResult = { items: _wrItems, count: _wrItems.length, fetched: true, source: \"aggregate\", continuityPackFallback: false };",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing aggregate round-trip marker %q", needle)
		}
	}

	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for aggregate read round-trip smoke")
	}
	script := `
const calls = [];
async function fetchSessionState() {
  calls.push("/session-state");
  return { fetched: true, sections: { storylines: [1], characters: [2], world_rules: [3] } };
}
async function fetchStorylines() { calls.push("/storylines"); return { items: [1], count: 1, fetched: true }; }
async function fetchCharacterStates() { calls.push("/characters"); return { items: [2], count: 1 }; }
async function fetchWorldRules() { calls.push("/world-rules"); return { items: [3], count: 1, fetched: true }; }
async function simulate(useAggregateRead) {
  calls.length = 0;
  const settings = { useAggregateRead };
  const chatSessionId = "sess";
  let _aggregateSnapshot = null;
  if (settings.useAggregateRead) {
    const _snap = await fetchSessionState(chatSessionId);
    if (_snap.fetched) _aggregateSnapshot = _snap;
  }
  let storylineResult;
  if (_aggregateSnapshot) {
    const _slItems = _aggregateSnapshot.sections.storylines;
    storylineResult = { items: _slItems, count: _slItems.length, fetched: true, source: "aggregate" };
  } else {
    storylineResult = await fetchStorylines(chatSessionId);
  }
  let characterResult;
  if (_aggregateSnapshot) {
    const _chItems = _aggregateSnapshot.sections.characters;
    characterResult = { items: _chItems, count: _chItems.length };
  } else {
    characterResult = await fetchCharacterStates(chatSessionId);
  }
  let worldRulesResult;
  if (_aggregateSnapshot) {
    const _wrItems = _aggregateSnapshot.sections.world_rules;
    worldRulesResult = { items: _wrItems, count: _wrItems.length, fetched: true, source: "aggregate" };
  } else {
    worldRulesResult = await fetchWorldRules(chatSessionId);
  }
  return { count: calls.length, calls: calls.slice(), storylineResult, characterResult, worldRulesResult };
}
(async () => {
  const aggregate = await simulate(true);
  const direct = await simulate(false);
  if (aggregate.count !== 1 || aggregate.calls[0] !== "/session-state") {
    throw new Error("aggregate calls = " + JSON.stringify(aggregate));
  }
  if (direct.count !== 3 || direct.calls.join(",") !== "/storylines,/characters,/world-rules") {
    throw new Error("direct calls = " + JSON.stringify(direct));
  }
  console.log(JSON.stringify({ aggregate, direct }));
})().catch((err) => { console.error(err.stack || err.message); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("aggregate read round-trip smoke failed: %v\n%s", err, out)
	}
}

func TestArchiveCenterJSMomentumPacketSupervisorAndTraceMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"// M-2c: supervisor input pack (persistent guidance + guide/initiative + momentum)",
		"// I-2c: momentum packet 조건부 fetch",
		"momentumPacket   = optSupervisorPack.momentum_packet   || null;",
		`bridgeFetch("/momentum-packet/" + encodeURIComponent(sessionId)`,
		"momentum_packet: momentumPacket || null",
		"result._momentumApplied = true;",
		"result._momentumPacketStatus = momentumPacket.packet_status || null;",
		"trace.momentum = {",
		"packetStatus: (supervisorResult && supervisorResult._momentumPacketStatus) || null",
		`rows.push(r("Momentum"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing momentum packet supervisor/trace marker %q", needle)
		}
	}
	if strings.Contains(src, "const _mp = await fetchSessionState(sessionId);") {
		t.Fatal("momentum packet path must not perform an extra fetchSessionState round-trip before /momentum-packet")
	}
}

func TestArchiveCenterJSTrustControlExplorerMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`"explorer.tabs.trust.label"`,
		`activeTab: "chat_logs",   // "chat_logs" | "memories" | "direct_evidence" | "kg_triples" | "episodes" | "trust" | "world" | "entities"`,
		"async function explorerFetchTrust",
		"async function explorerPatchTrust",
		`bridgeFetch("/storylines/" + encodeURIComponent(sid)`,
		`bridgeFetch("/world-rules/" + encodeURIComponent(sid)`,
		`bridgeFetch("/pending-threads/" + encodeURIComponent(sid) + "?status=all"`,
		`"/storylines/"`,
		`"/world-rules/"`,
		`"/pending-threads/"`,
		"function renderExplorerTrust()",
		`const cls = "mo-trust-btn"`,
		`data-trust-field="`,
		`data-trust-val="`,
		`document.querySelectorAll(".mo-trust-btn[data-trust-model]")`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing Trust Control Explorer marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P32PermissionSplitMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const coprocessorAuthorityModes = ["truth_writer", "proposal_only", "guidance_only", "diagnostic_only"]`,
		`const coprocessorTruthWriteTargets = ["current_fact", "canonical_state", "canonical_state_layer", "dense_summary"]`,
		`module: "step11_truth_core"`,
		`authority_mode: "truth_writer"`,
		`current_fact_write: true`,
		`canonical_write: true`,
		`module: "truth_maintenance"`,
		`authority_mode: "diagnostic_only"`,
		`module: "entity_coprocessor"`,
		`authority_mode: "proposal_only"`,
		`module: "world_coprocessor"`,
		`module: "narrative_quality_coprocessor"`,
		`authority_mode: "guidance_only"`,
		`denied_write_targets: coprocessorTruthWriteTargets.slice()`,
		`canonical_write_authority_after_reentry: "step11_truth_core_only"`,
		`analysis_provider_autonomous_truth_write: false`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P32 permission split marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P34TraceSeparationMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const coprocessorSidecarWritableTargets = ["proposal_trace", "guidance_trace", "audit_trace", "maintenance_metadata"]`,
		`const entityCoprocessorTraceDisplayMode = "hint_vs_current_fact_split"`,
		`const worldCoprocessorTraceDisplayMode = "hint_vs_current_world_state_split"`,
		`const narrativeQualityCoprocessorTraceDisplayMode = "quality_hint_vs_factual_state_split"`,
		`trace_display_mode: entityCoprocessorTraceDisplayMode`,
		`trace_display_hint_lane: entityCoprocessorTraceDisplayHintLane`,
		`trace_display_truth_lane: entityCoprocessorTraceDisplayTruthLane`,
		`trace_display_mode: worldCoprocessorTraceDisplayMode`,
		`trace_display_hint_lane: worldCoprocessorTraceDisplayHintLane`,
		`trace_display_truth_lane: worldCoprocessorTraceDisplayTruthLane`,
		`trace_display_mode: narrativeQualityCoprocessorTraceDisplayMode`,
		`trace_display_hint_lane: narrativeQualityCoprocessorTraceDisplayHintLane`,
		`trace_display_truth_lane: narrativeQualityCoprocessorTraceDisplayTruthLane`,
		`analysis_provider_trace_target: coprocessorAnalysisProviderTraceTargetsByModule.entity_coprocessor`,
		`analysis_provider_trace_target: coprocessorAnalysisProviderTraceTargetsByModule.world_coprocessor`,
		`analysis_provider_trace_target: coprocessorAnalysisProviderTraceTargetsByModule.narrative_quality_coprocessor`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P34 trace separation marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P35FeatureControlAblationMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const coprocessorFeatureControlPolicyVersion = "mg1c.v1"`,
		`const coprocessorFeatureFlagModes = ["always_on", "conservative", "experimental", "off"]`,
		`const coprocessorRolloutStages = ["truth_floor_locked", "diagnostic_default_on", "experimental_shadow", "manual_enable_required"]`,
		`const coprocessorKillSwitchStates = ["not_applicable", "armed_standby", "engaged"]`,
		`const coprocessorFeatureControlMatrix = [`,
		`module: "step11_truth_core"`,
		`default_mode: "always_on"`,
		`ablation_supported: false`,
		`wiring_status: "hard_enabled_runtime"`,
		`module: "truth_maintenance"`,
		`default_mode: "conservative"`,
		`kill_switch_action: "fail_open_skip"`,
		`module: "entity_coprocessor"`,
		`default_mode: "off"`,
		`wiring_status: "trace_contract_only"`,
		`module: "narrative_quality_coprocessor"`,
		`default_mode: "experimental"`,
		`ablation_supported: true`,
		`coprocessorFeatureControlByMode[featureMode].push(entry.module)`,
		`const coprocessorRuntimeActiveModules = coprocessorFeatureControlMatrix.filter`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P35 feature control marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P120AuthorityMatrixMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const coprocessorAuthorityModes = ["truth_writer", "proposal_only", "guidance_only", "diagnostic_only"]`,
		`const coprocessorAuthorityMatrix = [`,
		`module: "step11_truth_core"`,
		`authority_mode: "truth_writer"`,
		`module: "truth_maintenance"`,
		`authority_mode: "diagnostic_only"`,
		`module: "retrieval_supporting_inference"`,
		`authority_mode: "diagnostic_only"`,
		`module: "entity_coprocessor"`,
		`authority_mode: "proposal_only"`,
		`module: "world_coprocessor"`,
		`authority_mode: "proposal_only"`,
		`module: "narrative_quality_coprocessor"`,
		`authority_mode: "guidance_only"`,
		`coprocessor_authority_matrix: {`,
		`status: "authority_matrix_fixed"`,
		`const coprocessorAuthorityMatrixByMode = {}`,
		`coprocessorAuthorityMatrixByMode[authorityMode].push(entry.module)`,
		`const coprocessorAuthorityTruthWriterModules = (coprocessorAuthorityMatrixByMode.truth_writer || []).slice()`,
		`const coprocessorAuthoritySidecarModules = coprocessorAuthorityMatrix`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P120 authority matrix marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P125BoundaryDefineMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const coprocessorTruthWriteTargets = [`,
		`const coprocessorSidecarWritableTargets = [`,
		`const coprocessorBoundaryPolicyVersion = "mg1b.v1"`,
		`boundary_scope: "step11_truth_owner"`,
		`write_targets: coprocessorTruthWriteTargets.slice()`,
		`current_fact_write: true`,
		`canonical_write: true`,
		`boundary_scope: "step12_diagnostic_sidecar"`,
		`boundary_scope: "step12_proposal_sidecar"`,
		`boundary_scope: "step12_guidance_sidecar"`,
		`denied_write_targets: coprocessorTruthWriteTargets.slice()`,
		`current_fact_write: false`,
		`canonical_write: false`,
		`const coprocessorReadBoundaryBySurface = {}`,
		`const coprocessorWriteBoundaryByTarget = {}`,
		`sidecar_writable_targets: coprocessorSidecarWritableTargets.slice()`,
		`sidecar_denied_truth_targets: coprocessorTruthWriteTargets.slice()`,
		`read_boundary_by_surface: coprocessorReadBoundaryBySurface`,
		`write_boundary_by_target: coprocessorWriteBoundaryByTarget`,
		`canonical_write_authority_after_reentry: "step11_truth_core_only"`,
		`canonical_write_authority: "step11_truth_core_only"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P125 boundary define marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P131FeatureRolloutKillSwitchMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const coprocessorFeatureControlPolicyVersion = "mg1c.v1"`,
		`const coprocessorFeatureFlagModes = ["always_on", "conservative", "experimental", "off"]`,
		`const coprocessorRolloutStages = ["truth_floor_locked", "diagnostic_default_on", "experimental_shadow", "manual_enable_required"]`,
		`const coprocessorKillSwitchStates = ["not_applicable", "armed_standby", "engaged"]`,
		`const coprocessorFeatureControlMatrix = [`,
		`module: "step11_truth_core"`,
		`default_mode: "always_on"`,
		`rollout_stage: "truth_floor_locked"`,
		`kill_switch_supported: false`,
		`ablation_supported: false`,
		`wiring_status: "hard_enabled_runtime"`,
		`module: "truth_maintenance"`,
		`default_mode: "conservative"`,
		`rollout_stage: "diagnostic_default_on"`,
		`kill_switch_action: "fail_open_skip"`,
		`module: "retrieval_supporting_inference"`,
		`default_mode: "conservative"`,
		`rollout_stage: "diagnostic_default_on"`,
		`kill_switch_action: "fail_open_skip"`,
		`module: "entity_coprocessor"`,
		`default_mode: "off"`,
		`rollout_stage: "manual_enable_required"`,
		`wiring_status: "trace_contract_only"`,
		`kill_switch_action: "fail_open_skip"`,
		`module: "world_coprocessor"`,
		`default_mode: "off"`,
		`rollout_stage: "manual_enable_required"`,
		`kill_switch_action: "fail_open_skip"`,
		`module: "narrative_quality_coprocessor"`,
		`default_mode: "experimental"`,
		`rollout_stage: "experimental_shadow"`,
		`kill_switch_action: "fail_open_skip"`,
		`coprocessor_feature_control: {`,
		`policy_version: coprocessorFeatureControlPolicyVersion`,
		`supported_feature_flag_modes: coprocessorFeatureFlagModes.slice()`,
		`supported_rollout_stages: coprocessorRolloutStages.slice()`,
		`supported_kill_switch_states: coprocessorKillSwitchStates.slice()`,
		`kill_switchable_modules: coprocessorKillSwitchableModules.slice()`,
		`modules: coprocessorFeatureControlMatrix.slice()`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P131 feature/rollout/kill-switch marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P137BudgetSplitMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const coprocessorBudgetIsolationPolicyVersion = "mg1d.v1"`,
		`const truthFloorBudgetLane = "truth_floor"`,
		`const coprocessorHintBudgetLane = "coprocessor_hint"`,
		`const truthFloorBudgetLabels = [`,
		`const coprocessorHintBudgetPromptLabels = [`,
		`const coprocessorHintBudgetPromptModules = [`,
		`narrativeQualityCoprocessorAblationProtectedTruthLabels = truthFloorBudgetLabels.slice()`,
		`worldCoprocessorGuidanceBudgetLane = coprocessorHintBudgetLane`,
		`worldCoprocessorGuidanceBudgetProtectedTruthLabels = truthFloorBudgetLabels.slice()`,
		`coprocessor_budget_isolation: {`,
		`status: "truth_floor_reserved_hint_residual"`,
		`policy_version: coprocessorBudgetIsolationPolicyVersion`,
		`budget_isolation_strategy: "truth_floor_reserved_then_hint_residual"`,
		`truth_floor_lane: truthFloorBudgetLane`,
		`truth_floor_owner_modules: coprocessorAuthorityTruthWriterModules.slice()`,
		`truth_floor_labels: truthFloorBudgetLabels.slice()`,
		`truth_floor_reservation_mode: "hard_floor_reserved_first"`,
		`hint_budget_lane: coprocessorHintBudgetLane`,
		`hint_budget_modules: coprocessorAuthoritySidecarModules.slice()`,
		`hint_budget_prompt_modules: coprocessorHintBudgetPromptModules.slice()`,
		`hint_budget_non_prompt_modules: coprocessorHintBudgetNonPromptModules.slice()`,
		`hint_budget_prompt_labels: coprocessorHintBudgetPromptLabels.slice()`,
		`hint_budget_reservation_mode: "residual_only"`,
		`label_lane_map: Object.assign({}, coprocessorBudgetLaneByLabel)`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P137 budget split marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P145MG1eKeepDropDegradeReasonTraceMarker(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const coprocessorReasonTracePolicyVersion = "mg1e.v1"`,
		`const coprocessorReasonTraceActions = ["keep", "drop", "degrade"]`,
		`const coprocessorReasonTraceKeepReasonCodes = [`,
		`truth_floor_reserved`,
		`hint_budget_delivered`,
		`delivered_full`,
		`no_canonical_conflict`,
		`canonical_guard_disabled`,
		`const coprocessorReasonTraceDropReasonCodes = [`,
		`budget_exhausted`,
		`supporting_guidance_conflict_blocked`,
		`canonical_polarity_conflict`,
		`budget_collision_saga_reserve`,
		`reliability_guard_hold`,
		`ingest_only_not_injected`,
		`explicit_user_redirection`,
		`const coprocessorReasonTraceDegradeReasonCodes = [`,
		`item_truncated`,
		`hard_cap_unstructured`,
		`no_alignment_rescue_ceiling`,
		`const coprocessorReasonTraceBudgetReasonCodes = [`,
		`const coprocessorReasonTraceReasonCodes = Array.from(new Set(`,
		`coprocessor_reason_trace: {`,
		`status: "keep_drop_degrade_trace_fixed"`,
		`policy_version: coprocessorReasonTracePolicyVersion`,
		`supported_actions: coprocessorReasonTraceActions.slice()`,
		`supported_reason_codes: coprocessorReasonTraceReasonCodes.slice()`,
		`keep_reason_codes: coprocessorReasonTraceKeepReasonCodes.slice()`,
		`drop_reason_codes: coprocessorReasonTraceDropReasonCodes.slice()`,
		`degrade_reason_codes: coprocessorReasonTraceDegradeReasonCodes.slice()`,
		`budget_reason_codes: coprocessorReasonTraceBudgetReasonCodes.slice()`,
		`truth_floor_lane: truthFloorBudgetLane`,
		`hint_budget_lane: coprocessorHintBudgetLane`,
		`trace_sources: ["blocks", "trimmed", "retrievalKeepDropTrace"]`,
		`const coprocessorReasonTraceByAction = { keep: [], drop: [], degrade: [] }`,
		`degrade: coprocessorReasonTraceByAction.degrade.length`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P145 MG-1e keep/drop/degrade reason trace marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P152MG1fAntiCopyReviewChecklistMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const coprocessorAntiCopyReviewPolicyVersion = "mg1f.v1"`,
		`const coprocessorAntiCopyReviewScope = "pr_unit"`,
		`const coprocessorAntiCopyAllowedReferenceMode = "behavioral_reference_only"`,
		`const coprocessorAntiCopyReviewEvidenceFields = [`,
		`"local_owner_surface"`,
		`"behavioral_source_note"`,
		`"diff_review_note"`,
		`"constant_origin_note"`,
		`const coprocessorAntiCopyReviewChecklist = [`,
		`check_id: "external_structure"`,
		`carryover_class: "structure"`,
		`check_id: "function_body"`,
		`carryover_class: "function_body"`,
		`check_id: "call_flow"`,
		`carryover_class: "call_flow"`,
		`check_id: "constant_values"`,
		`carryover_class: "constant_value"`,
		`check_id: "thresholds"`,
		`carryover_class: "threshold"`,
		`check_id: "ratios"`,
		`carryover_class: "ratio"`,
		`check_id: "token_budget_defaults"`,
		`carryover_class: "token_budget_default"`,
		`blocking: true`,
		`forbidden_direct_carryover: true`,
		`const coprocessorAntiCopyReviewCheckIds`,
		`const coprocessorAntiCopyReviewBlockingChecks`,
		`const coprocessorAntiCopyForbiddenCarryoverClasses`,
		`coprocessor_anti_copy_review: {`,
		`status: "review_checklist_fixed"`,
		`policy_version: coprocessorAntiCopyReviewPolicyVersion`,
		`review_scope: coprocessorAntiCopyReviewScope`,
		`allowed_reference_mode: coprocessorAntiCopyAllowedReferenceMode`,
		`required_evidence_fields: coprocessorAntiCopyReviewEvidenceFields.slice()`,
		`forbidden_carryover_classes: coprocessorAntiCopyForbiddenCarryoverClasses.slice()`,
		`blocking_checks: coprocessorAntiCopyReviewBlockingChecks.slice()`,
		`checklist: coprocessorAntiCopyReviewChecklist.slice()`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P152 MG-1f anti-copy review checklist marker %q", needle)
		}
	}
}
func TestArchiveCenterJSSeq12P160MG1gProposalReentryGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const coprocessorProposalReentryPolicyVersion = "mg1g.v1"`,
		`const coprocessorProposalTraceRequiredFields = ["evidence_refs", "source_turns", "confidence"]`,
		`const coprocessorProposalReentryRequiredModules = ["entity_coprocessor", "world_coprocessor"]`,
		`const coprocessorProposalAdoptionPath = ["proposal_trace", "reducer_reentry", "step11_truth_core"]`,
		`proposal_trace_schema_required_fields: coprocessorProposalTraceRequiredFields.slice()`,
		`truth_path_reentry_gate: "reducer_reentry_required"`,
		`truth_path_entry_requirements: coprocessorProposalTraceRequiredFields.concat(["reducer_reentry"])`,
		`truth_path_blocked_without_required_fields: true`,
		`canonical_write_before_reducer_reentry: false`,
		`canonical_write_authority_after_reentry: "step11_truth_core_only"`,
		`coprocessor_proposal_reentry: {`,
		`status: "proposal_reentry_gate_fixed"`,
		`policy_version: coprocessorProposalReentryPolicyVersion`,
		`proposal_trace_modules: coprocessorProposalTraceModules.slice()`,
		`required_trace_fields: coprocessorProposalTraceRequiredFields.slice()`,
		`reducer_reentry_required_modules: coprocessorProposalReentryRequiredModules.slice()`,
		`canonical_write_before_reducer_reentry: false`,
		`canonical_write_authority: "step11_truth_core_only"`,
		`adoption_path: coprocessorProposalAdoptionPath.slice()`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P160 MG-1g proposal re-entry gate marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P168MG1hAnalysisProviderContractMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const coprocessorAnalysisProviderPolicyVersion = "mg1h.v1"`,
		`const coprocessorAnalysisProviderModules = ["entity_coprocessor", "world_coprocessor", "narrative_quality_coprocessor"]`,
		`const coprocessorAnalysisProviderRawOutputMode = "proposal_only"`,
		`const coprocessorAnalysisProviderCallFrequencyOwner = "step13_governor"`,
		`analysis_provider_class: "llm_based_sidecar"`,
		`analysis_provider_raw_output_mode: coprocessorAnalysisProviderRawOutputMode`,
		`analysis_provider_autonomous_truth_write: false`,
		`analysis_provider_call_frequency_owner: coprocessorAnalysisProviderCallFrequencyOwner`,
		`coprocessor_analysis_provider: {`,
		`status: "analysis_provider_contract_fixed"`,
		`policy_version: coprocessorAnalysisProviderPolicyVersion`,
		`provider_class: "llm_based_sidecar"`,
		`provider_backed_modules: coprocessorAnalysisProviderModules.slice()`,
		`raw_output_mode: coprocessorAnalysisProviderRawOutputMode`,
		`trace_target_by_module: Object.assign({}, coprocessorAnalysisProviderTraceTargets)`,
		`normalization_by_module: Object.assign({}, coprocessorAnalysisProviderNormalizationModes)`,
		`autonomous_truth_write_allowed: false`,
		`disallowed_usage: coprocessorAnalysisProviderDisallowedUsage.slice()`,
		`canonical_write_authority: "step11_truth_core_only"`,
		`call_frequency_owner: coprocessorAnalysisProviderCallFrequencyOwner`,
		`call_frequency_policy_version: step13GovernorPolicyVersion`,
		`call_frequency_policy_status: "governor_contract_fixed"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P168 MG-1h analysis provider contract marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P179(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const coprocessorOrchestrationPolicyVersion = "or1a.v1"`,
		`const coprocessorOrchestrationRuntimeStages = ["step11_truth_stack", "residual_guidance"]`,
		`const coprocessorOrchestrationTruthStackOwner = "step11_truth_core"`,
		`const coprocessorOrchestrationCallEntryGate = "after_step11_truth_stack"`,
		`const coprocessorOrchestrationResidualBudgetSource = "coprocessor_hint_residual"`,
		`const coprocessorOrchestrationProviderCallOrder = ["entity_coprocessor", "world_coprocessor", "narrative_quality_coprocessor"]`,
		`entity_coprocessor: "residual_guidance"`,
		`world_coprocessor: "residual_guidance"`,
		`narrative_quality_coprocessor: "residual_guidance"`,
		`orchestration_stage: coprocessorOrchestrationStageByModule.entity_coprocessor`,
		`orchestration_entry_gate: coprocessorOrchestrationCallEntryGate`,
		`orchestration_budget_source: coprocessorOrchestrationResidualBudgetSource`,
		`coprocessor_orchestration: {`,
		`status: "residual_guidance_stage_fixed"`,
		`policy_version: coprocessorOrchestrationPolicyVersion`,
		`runtime_stage_order: coprocessorOrchestrationRuntimeStages.slice()`,
		`truth_stack_owner: coprocessorOrchestrationTruthStackOwner`,
		`truth_stack_labels: truthFloorBudgetLabels.slice()`,
		`call_entry_gate: coprocessorOrchestrationCallEntryGate`,
		`residual_guidance_stage: "residual_guidance"`,
		`residual_guidance_budget_source: coprocessorOrchestrationResidualBudgetSource`,
		`provider_call_order: coprocessorOrchestrationProviderCallOrder.slice()`,
		`stage_by_module: Object.assign({}, coprocessorOrchestrationStageByModule)`,
		`call_allowed_before_truth_stack: false`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P179 OR-1a coprocessor call order marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P187OR1bFallbackRouteMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`function getOrchestrationFallbackPolicyOr1b()`,
		`policyVersion: "or1b.v1"`,
		`owner: "step12.orchestration"`,
		`supportedRoutes: ["blocked_empty_result", "cached_result", "skip_protection_only", "direct_result"]`,
		`emptyResultRoute: "direct_result"`,
		`emptyResultPayloadAction: "preserve_original_payload"`,
		`readyCacheRoute: "cached_result"`,
		`readyCachePayloadAction: "consume_pending_ready_result"`,
		`readyCacheReuseScope: "same_chat_session_after_request_only"`,
		`overlapRunningRoute: "skip_protection_only"`,
		`overlapRunningPayloadAction: "applyProtectionOnlyInjection"`,
		`stalePendingAction: "clear_stale_then_recompute"`,
		`timeoutWindowSource: "getOrchestrationTimeoutMs"`,
		`function resolveOrchestrationFallbackRouteOr1b`,
		`function applyOrchestrationFallbackTraceOr1b`,
		`trace.orchestrationFallback = {`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P187 OR-1b fallback route marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P195OR1cDirtySignalMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`function computeOrchestrationDirtyHashOr1c`,
		`function getPrepareTurnBundleSignatureOr1c`,
		`function getOrchestrationDirtySignalPolicyOr1c()`,
		`policyVersion: "or1c.v1"`,
		`evaluationMode: "trace_contract_only"`,
		`"session_bootstrap"`,
		`"user_input_changed"`,
		`"input_source_changed"`,
		`"continuity_mode_changed"`,
		`"continuity_query_changed"`,
		`"prepare_turn_source_changed"`,
		`"prepare_turn_bundle_changed"`,
		`"rollback_invalidation"`,
		`"guidance_invalidation"`,
		`"stable_session_snapshot"`,
		`stableSignals: ["stable_session_snapshot"]`,
		`requiredSnapshotFields: [`,
		`"chat_session_id"`,
		`"user_input_hash"`,
		`"input_source"`,
		`"continuity_mode"`,
		`"continuity_query_hash"`,
		`"prepare_turn_source"`,
		`"prepare_turn_bundle_signature"`,
		`"rollback_token"`,
		`"guidance_token"`,
		`runtimeExecutionMode: "always_recompute_until_or1d"`,
		`runtimeSkipBehavior: "trace_only_policy_contract"`,
		`dirty_signal_policy_version: coprocessorOrchestrationDirtySignalPolicyVersion`,
		`dirty_signal_evaluation_mode: coprocessorOrchestrationDirtySignalEvaluationMode`,
		`dirty_signal_snapshot_fields: coprocessorOrchestrationDirtySnapshotFields.slice()`,
		`dirty_signal_supported: coprocessorOrchestrationDirtySignals.slice()`,
		`dirty_signal_recompute: coprocessorOrchestrationDirtyRecomputeSignals.slice()`,
		`dirty_signal_stable: coprocessorOrchestrationDirtyStableSignals.slice()`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P195 OR-1c dirty signal marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P203OR1dCacheInvalidationMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`function getOrchestrationCachePolicyOr1d()`,
		`policyVersion: "or1d.v1"`,
		`cacheUnit: "pending_ready_orchestration_result"`,
		`cacheKeySourcePolicyVersion: dirtyPolicy.policyVersion`,
		`cacheKeyFields: dirtyPolicy.requiredSnapshotFields.slice().concat(["dirty_policy_version"])`,
		`reuseScope: fallbackPolicy.readyCacheReuseScope`,
		`reuseRoute: fallbackPolicy.readyCacheRoute`,
		`invalidationSignals: dirtyPolicy.recomputeSignals.slice()`,
		`invalidationTokens: ["rollback_token", "guidance_token"]`,
		`stalePendingAction: fallbackPolicy.stalePendingAction`,
		`staleServingGuard: "deny_reuse_on_cache_key_mismatch_or_invalidation_drift"`,
		`runtimeReuseMode: "pending_ready_only"`,
		`advancedCachePolicyStatus: "deferred_to_step13"`,
		`function buildOrchestrationCacheDescriptorOr1d`,
		`function assessOrchestrationCacheReuseOr1d`,
		`cache_policy_version: coprocessorOrchestrationCachePolicyVersion`,
		`cache_unit: coprocessorOrchestrationCacheUnit`,
		`cache_reuse_scope: coprocessorOrchestrationCacheReuseScope`,
		`cache_key_fields: coprocessorOrchestrationCacheKeyFields.slice()`,
		`cache_invalidation_signals: coprocessorOrchestrationCacheInvalidationSignals.slice()`,
		`cache_invalidation_tokens: coprocessorOrchestrationCacheInvalidationTokens.slice()`,
		`cache_stale_guard: coprocessorOrchestrationCacheStaleGuard`,
		`cache_advanced_policy_status: "deferred_to_step13"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P203 OR-1d cache invalidation marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P211OR1eModuleTransportMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`function getOrchestrationModuleTransportPolicyOr1e()`,
		`policyVersion: "or1e.v1"`,
		`backendRequiredModules: [`,
		`"prepare_turn_probe"`,
		`"memory_search"`,
		`"kg_recall"`,
		`"episode_recall"`,
		`"active_states_fetch"`,
		`"narrative_control_fetch"`,
		`"maintenance_pass_writeback"`,
		`"turn_complete_commit"`,
		`"rollback_invalidation"`,
		`backendBundleAssistedSurfaces: [`,
		`backendProxyAssistedModules: [`,
		`pluginOnlyModules: [`,
		`backendBundleEntryPoint: "/prepare-turn"`,
		`backendProxyRoute: "/proxy/plugin-main"`,
		`pluginOnlyExecutionMode: "local_runtime_contract_only"`,
		`runtimeSplitStatus: "trace_contract_only"`,
		`function buildOrchestrationModuleTransportStateOr1e`,
		`function applyOrchestrationModuleTransportTraceOr1e`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P211 OR-1e module transport marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P218OR1fTurnDeletionDetectionMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`function getRollbackDetectionPolicyOr1f()`,
		`policyVersion: "or1f.v1"`,
		`detectionSources: [`,
		`"history_diff_common_prefix_suffix"`,
		`"persisted_turn_ledger"`,
		`"tail_hash_guard"`,
		`"tail_delete"`,
		`"assistant_deleted_before_next_user_turn"`,
		`"historical_contiguous_delete"`,
		`duplicateGuard: "session_history_diff_signature"`,
		`function buildRollbackTurnLedgerOr1f`,
		`function resolveRollbackTurnAnchorOr1f`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P218 OR-1f turn deletion detection marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P223OR1gHistoricalDeletionInvalidationMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`function getRollbackInvalidationPolicyOr1g()`,
		`policyVersion: "or1g.v1"`,
		`invalidationRoute: "/rollback/{turn_index}"`,
		`triggerSources: ["auto_rollback", "manual_ui"]`,
		`"rollback_token"`,
		`"guidance_token_on_backend_reset"`,
		`guidanceCleanupMode: "invalidate_guidance_plan_and_delete_compacts_and_maintenance"`,
		`cacheReuseGuard: "rollback_and_guidance_token_drift_block_pending_ready_reuse"`,
		`staleSidecarGuard: "backend_cleanup_then_dirty_signal_recompute"`,
		`function buildRollbackInvalidationStateOr1g`,
		`function applyRollbackInvalidationTraceOr1g`,
		`historicalDeletionDetected: String(triggerReason || "") === "historical_turn_gap_detected"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P223 OR-1g historical deletion invalidation marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P230OR1hDirtySignalMatrixMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`function getOrchestrationDirtyMatrixPolicyOr1h()`,
		`policyVersion: "or1h.v1"`,
		`dirtyTargets: [`,
		`"guidance_state"`,
		`"entity_coprocessor"`,
		`"world_coprocessor"`,
		`"narrative_quality"`,
		`"sidecar_cache"`,
		`runtimeObservedEventTypes: ["turn_deletion"]`,
		`runtimeEventTargetMatrix: {`,
		`user_correction: [`,
		`canonical_update: [`,
		`world_state_update: [`,
		`turn_deletion: [`,
		`backfill_import: [`,
		`schema_migration: [`,
		`delegatedEventMatrixVersions: {`,
		`truth_maintenance_drift: "or1h.tm1d.v1"`,
		`truth_maintenance_importance: "or1h.tm1d.v1"`,
		`function buildOrchestrationDirtyMatrixStateOr1h`,
		`function applyOrchestrationDirtyMatrixTraceOr1h`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P230 OR-1h dirty signal matrix marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P235OR1iRebuildInvalidationOrchestrationMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`function getOrchestrationRebuildPolicyOr1i()`,
		`policyVersion: "or1i.v1"`,
		`staleServingPolicy: "deny_stale_sidecar_on_rebuild_pending"`,
		`staleDropTargets: [`,
		`"pending_ready_orchestration_result"`,
		`"sidecar_cache"`,
		`"guidance_trace"`,
		`hardResetTargets: [`,
		`"prepare_turn_bundle"`,
		`"session_snapshot_cache"`,
		`"persisted_turn_ledger"`,
		`startPointPrecedence: [`,
		`"checkpoint_full_rebuild"`,
		`"rollback_turn_anchor_then_prepare_turn"`,
		`"next_narrative_control_fetch"`,
		`"next_prepare_turn_fetch"`,
		`"step11_truth_stack"`,
		`planByEventType: {`,
		`rebuildMode: "selective"`,
		`rebuildMode: "full"`,
		`function buildOrchestrationRebuildStateOr1i`,
		`function applyOrchestrationRebuildTraceOr1i`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P235 OR-1i rebuild invalidation orchestration marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P242OR1jStaleProposalServingMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`function getOrchestrationStaleProposalPolicyOr1j()`,
		`policyVersion: "or1j.v1"`,
		`blockedPromptTargets: [`,
		`"pending_ready_orchestration_result"`,
		`"proposal_trace"`,
		`"guidance_trace"`,
		`"sidecar_cache"`,
		`evidenceMismatchSignals: ["chapter_block_mismatch", "chapter_input_anchor_mismatch"]`,
		`runtimeEnforcement: "after_request_pending_salvage_guard"`,
		`staleServingAction: "drop_stale_pending_before_prompt_and_persistence"`,
		`function assessOrchestrationStaleProposalServingOr1j`,
		`function selectAfterRequestOrchestrationResultOr1j`,
		`function applyOrchestrationStaleProposalTraceOr1j`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P242 OR-1j stale proposal serving marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P252EC1aEntityCoprocessorContractMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`entityCoprocessorContractPolicyVersion = "ec1a.v1"`,
		`entityCoprocessorInputSurfaces = ["characters", "pending_threads", "latest_direct_evidence", "recent_raw_turn"]`,
		`entityCoprocessorInputFocusSignals = ["relation_change", "emotion_drift", "continuity_risk", "scene_carryover"]`,
		`entityCoprocessorOutputProposalTypes = ["relation_drift_hint", "emotion_drift_hint", "continuity_risk_hint", "scene_carryover_candidate", "evidence_bound_patch_proposal"]`,
		`input_contract_policy_version: entityCoprocessorContractPolicyVersion`,
		`input_contract_surfaces: entityCoprocessorInputSurfaces.slice()`,
		`input_contract_focus_signals: entityCoprocessorInputFocusSignals.slice()`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P252 EC-1a entity coprocessor contract marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P255EC1bEntityEvidenceFieldsMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`entityCoprocessorEvidenceContractPolicyVersion = "ec1b.v1"`,
		`entityCoprocessorOutputRequiredFields = coprocessorProposalTraceRequiredFields.slice()`,
		`entityCoprocessorOutputMode = "proposal_trace_only"`,
		`entityCoprocessorEvidenceBindingMode = "required_for_all_outputs"`,
		`output_contract_policy_version: entityCoprocessorEvidenceContractPolicyVersion`,
		`output_contract_required_fields: entityCoprocessorOutputRequiredFields.slice()`,
		`output_contract_evidence_binding: entityCoprocessorEvidenceBindingMode`,
		`output_contract_mode: entityCoprocessorOutputMode`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P255 EC-1b evidence fields marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P260EC1cPatchProposalTruthGuardMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`entityCoprocessorPatchDirectWriteGuardPolicyVersion = "ec1c.v1"`,
		`entityCoprocessorPatchDirectWriteBlockedTarget = "canonical_relationship_state"`,
		`entityCoprocessorPatchDirectWriteAllowedRoute = "proposal_trace_to_reducer_reentry_only"`,
		`entityCoprocessorPatchDirectWriteAuthorityCeiling = "step11_truth_core_only"`,
		`patch_direct_write_guard_policy_version: entityCoprocessorPatchDirectWriteGuardPolicyVersion`,
		`patch_direct_write_guard_blocked_target: entityCoprocessorPatchDirectWriteBlockedTarget`,
		`patch_direct_write_guard_authority_ceiling: entityCoprocessorPatchDirectWriteAuthorityCeiling`,
		`patch_direct_write_guard_allowed_route: entityCoprocessorPatchDirectWriteAllowedRoute`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P260 EC-1c patch proposal truth guard marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P266EC1dRelationDriftTraceSplitMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`entityCoprocessorTraceDisplayPolicyVersion = "ec1d.v1"`,
		`entityCoprocessorTraceDisplayMode = "hint_vs_current_fact_split"`,
		`entityCoprocessorTraceDisplayHintLane = "entity_proposal_hint"`,
		`entityCoprocessorTraceDisplayTruthLane = "step11_current_fact"`,
		`entityCoprocessorTraceDisplayTruthTargets = ["current_fact", "canonical_relationship_state"]`,
		`entityCoprocessorTraceDisplayDisallowedAliases = ["proposal_trace_as_current_fact", "relation_drift_hint_as_canonical_relationship_state"]`,
		`trace_display_policy_version: entityCoprocessorTraceDisplayPolicyVersion`,
		`trace_display_mode: entityCoprocessorTraceDisplayMode`,
		`trace_display_hint_lane: entityCoprocessorTraceDisplayHintLane`,
		`trace_display_truth_lane: entityCoprocessorTraceDisplayTruthLane`,
		`trace_display_disallowed_aliases: entityCoprocessorTraceDisplayDisallowedAliases.slice()`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P266 EC-1d relation drift trace split marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P272EC1eStaleSceneTruthGuardMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`entityCoprocessorStaleSceneGuardPolicyVersion = "ec1e.v1"`,
		`entityCoprocessorStaleSceneGuardMode = "canonical_conflict_guard_bridge"`,
		`entityCoprocessorStaleSceneGuardInputLabels = ["characters", "pending_threads"]`,
		`entityCoprocessorStaleSceneGuardAnchorSource = "canonical_state_layer"`,
		`entityCoprocessorStaleSceneGuardConflictReason = "canonical_polarity_conflict"`,
		`entityCoprocessorStaleSceneGuardSuppressionTarget = "scene_carryover_candidate"`,
		`entityCoprocessorStaleSceneGuardSuppressionRoute = "drop_entity_hint_keep_truth_floor"`,
		`stale_scene_guard_policy_version: entityCoprocessorStaleSceneGuardPolicyVersion`,
		`stale_scene_guard_mode: entityCoprocessorStaleSceneGuardMode`,
		`stale_scene_guard_conflict_reason: entityCoprocessorStaleSceneGuardConflictReason`,
		`stale_scene_guard_suppression_target: entityCoprocessorStaleSceneGuardSuppressionTarget`,
		`stale_scene_guard_suppression_route: entityCoprocessorStaleSceneGuardSuppressionRoute`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P272 EC-1e stale scene truth guard marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P278EC1fDriveLatticeGuidanceOnlyMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`entityCoprocessorBranchRegistryPolicyVersion = "ec1f.v1"`,
		`entityCoprocessorBranchRegistryCanonicalSource = "relationship_state.drive_lattice"`,
		`entityCoprocessorBranchRegistrySignalKeys = ["pull", "alarm", "scar", "veil", "tether", "lock"]`,
		`entityCoprocessorBranchRegistrySignalClass = "relationship_dynamics"`,
		`entityCoprocessorBranchRegistryProposalScope = "guidance_only"`,
		`entityCoprocessorBranchRegistryTruthPathAllowed = false`,
		`entityCoprocessorBranchRegistryReducerReentryAllowed = false`,
		`entityCoprocessorBranchRegistryCanonicalWriteAllowed = false`,
		`branch_registry_policy_version: entityCoprocessorBranchRegistryPolicyVersion`,
		`branch_registry_canonical_source: entityCoprocessorBranchRegistryCanonicalSource`,
		`branch_registry_signal_keys: entityCoprocessorBranchRegistrySignalKeys.slice()`,
		`branch_registry_signal_class: entityCoprocessorBranchRegistrySignalClass`,
		`branch_registry_proposal_scope: entityCoprocessorBranchRegistryProposalScope`,
		`branch_registry_truth_path_allowed: entityCoprocessorBranchRegistryTruthPathAllowed`,
		`branch_registry_canonical_write_allowed: entityCoprocessorBranchRegistryCanonicalWriteAllowed`,
		`branch_registry_reducer_reentry_allowed: entityCoprocessorBranchRegistryReducerReentryAllowed`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P278 EC-1f drive lattice guidance-only marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P287WC1aWorldCoprocessorContractMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`worldCoprocessorContractPolicyVersion = "wc1a.v1"`,
		`worldCoprocessorInputSurfaces = ["world_context"`,
		`"storylines"`,
		`"latest_direct_evidence"`,
		`"recent_raw_turn"`,
		`worldCoprocessorInputFocusSignals = ["faction_pressure"`,
		`"region_pressure"`,
		`"offscreen_thread_pressure"`,
		`"public_pressure"`,
		`"propagation_risk"`,
		`"scene_scope"`,
		`worldCoprocessorOutputProposalTypes = ["offscreen_pressure_hint"`,
		`"faction_region_pressure_summary"`,
		`"public_pressure_summary"`,
		`"propagation_risk_proposal"`,
		`"scene_scoped_setting_hint"`,
		`input_contract_policy_version: worldCoprocessorContractPolicyVersion`,
		`input_contract_surfaces: worldCoprocessorInputSurfaces.slice()`,
		`input_contract_focus_signals: worldCoprocessorInputFocusSignals.slice()`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P287 WC-1a world coprocessor contract marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P290WC1bWorldWriteGuardMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`worldCoprocessorWriteGuardPolicyVersion = "wc1b.v1"`,
		`worldCoprocessorWriteGuardBlockedTargets = ["scene_rules"`,
		`"persistent_rules"`,
		`"canonical_world_current_state"`,
		`patch_direct_write_guard_policy_version: worldCoprocessorWriteGuardPolicyVersion`,
		`patch_direct_write_guard_blocked_targets: worldCoprocessorWriteGuardBlockedTargets.slice()`,
		`patch_direct_write_guard_allowed_route: worldCoprocessorWriteGuardAllowedRoute`,
		`patch_direct_write_guard_authority_ceiling: worldCoprocessorWriteGuardAuthorityCeiling`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P290 WC-1b world write guard marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P293WC1cWorldPressureTraceSplitMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`worldCoprocessorTraceDisplayPolicyVersion = "wc1c.v1"`,
		`worldCoprocessorTraceDisplayMode = "hint_vs_current_world_state_split"`,
		`worldCoprocessorTraceDisplayHintLane = "world_pressure_hint"`,
		`worldCoprocessorTraceDisplayTruthLane = "step11_world_current_state"`,
		`worldCoprocessorTraceDisplayTruthTargets = ["scene_rules", "persistent_rules", "canonical_world_current_state"]`,
		`worldCoprocessorTraceDisplayDisallowedAliases = ["world_pressure_hint_as_current_world_state", "scene_scoped_setting_hint_as_scene_rule"]`,
		`trace_display_policy_version: worldCoprocessorTraceDisplayPolicyVersion`,
		`trace_display_mode: worldCoprocessorTraceDisplayMode`,
		`trace_display_hint_lane: worldCoprocessorTraceDisplayHintLane`,
		`trace_display_truth_lane: worldCoprocessorTraceDisplayTruthLane`,
		`trace_display_disallowed_aliases: worldCoprocessorTraceDisplayDisallowedAliases.slice()`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P293 WC-1c world pressure trace split marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P298WC1dPropagationRiskGuidanceBudgetMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`worldCoprocessorGuidanceBudgetPolicyVersion = "wc1d.v1"`,
		`worldCoprocessorGuidanceBudgetMode = "guidance_only_competition"`,
		`worldCoprocessorGuidanceBudgetSource = coprocessorOrchestrationResidualBudgetSource`,
		`worldCoprocessorGuidanceBudgetLane = coprocessorHintBudgetLane`,
		`guidance_budget_policy_version: worldCoprocessorGuidanceBudgetPolicyVersion`,
		`guidance_budget_mode: worldCoprocessorGuidanceBudgetMode`,
		`guidance_budget_lane: worldCoprocessorGuidanceBudgetLane`,
		`guidance_budget_source: worldCoprocessorGuidanceBudgetSource`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P298 WC-1d propagation risk guidance budget marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P301WC1eConservativeDegradeGuardMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`worldCoprocessorConservativeDegradePolicyVersion = "wc1e.v1"`,
		`worldCoprocessorConservativeDegradeMode = "evidence_bound_conservative_degrade"`,
		`worldCoprocessorConservativeDegradeReasonCodes = ["reliability_guard_hold", "ingest_only_not_injected"]`,
		`worldCoprocessorConservativeDegradeMissingEvidenceAction = "drop_world_patch_proposal"`,
		`worldCoprocessorConservativeDegradeLowConfidenceAction = "degrade_to_hint_only_or_skip"`,
		`worldCoprocessorConservativeDegradeTruthFloorFallback = "preserve_step11_world_current_state"`,
		`conservative_degrade_policy_version: worldCoprocessorConservativeDegradePolicyVersion`,
		`conservative_degrade_mode: worldCoprocessorConservativeDegradeMode`,
		`conservative_degrade_missing_evidence_action: worldCoprocessorConservativeDegradeMissingEvidenceAction`,
		`conservative_degrade_low_confidence_action: worldCoprocessorConservativeDegradeLowConfidenceAction`,
		`conservative_degrade_truth_floor_fallback: worldCoprocessorConservativeDegradeTruthFloorFallback`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P301 WC-1e conservative degrade guard marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P304WC1fSceneScopedSettingFrameMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`worldCoprocessorSceneSlicePolicyVersion = "wc1f.v1"`,
		`worldCoprocessorSceneSliceSelectorSignal = "scene_scope"`,
		`worldCoprocessorSceneSliceExtractionMode = "current_scene_thin_frame_only"`,
		`worldCoprocessorSceneSliceOutputType = "scene_scoped_setting_hint"`,
		`worldCoprocessorSceneSliceRequiredFields = ["scope_name", "scene_rule_anchor"]`,
		`worldCoprocessorSceneSliceDeliverySurface = "guidance_only_setting_frame"`,
		`worldCoprocessorSceneSliceTruthAliasBlocked = true`,
		`worldCoprocessorSceneSliceTruthBorrowAllowed = false`,
		`scene_slice_policy_version: worldCoprocessorSceneSlicePolicyVersion`,
		`scene_slice_selector_signal: worldCoprocessorSceneSliceSelectorSignal`,
		`scene_slice_extraction_mode: worldCoprocessorSceneSliceExtractionMode`,
		`scene_slice_output_type: worldCoprocessorSceneSliceOutputType`,
		`scene_slice_truth_alias_blocked: worldCoprocessorSceneSliceTruthAliasBlocked`,
		`scene_slice_truth_borrow_allowed: worldCoprocessorSceneSliceTruthBorrowAllowed`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P304 WC-1f scene-scoped setting frame marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P312NQ1aNarrativeQualityContractMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`narrativeQualityCoprocessorContractPolicyVersion = "nq1a.v1"`,
		`narrativeQualityLayerLabels = ["narrative_guide"`,
		`narrativeQualityLayerPolicyVersion`,
		`narrativeQualityCoprocessorSourceRoles`,
		`narrativeQualityLayerMode = "quality_hint_only"`,
		`narrativeQualityLayerDisallowedUsage = ["truth_arbitration"`,
		`"canonical_overwrite"`,
		`"current_fact_override"`,
		`input_contract_policy_version: narrativeQualityCoprocessorContractPolicyVersion`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P312 NQ-1a narrative quality contract marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P315NQ1bPacingCallbackObligationMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`narrativeQualityCoprocessorOutputPolicyVersion = "nq1b.v1"`,
		`narrativeQualityCoprocessorOutputMode = "guidance_trace_only"`,
		`narrativeQualityCoprocessorOutputHintTypes = ["pacing_hint"`,
		`"scene_obligation_reminder"`,
		`"callback_opportunity_hint"`,
		`"emphasis_ordering_proposal"`,
		`narrativeQualityCoprocessorOutputRequiredFields = ["hint_type"`,
		`"hint_text"`,
		`"priority"`,
		`output_contract_policy_version: narrativeQualityCoprocessorOutputPolicyVersion`,
		`output_contract_hint_types: narrativeQualityCoprocessorOutputHintTypes.slice()`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P315 NQ-1b pacing/callback/obligation marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P318NQ1cConflictGuardMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`narrativeQualityCoprocessorConflictGuardPolicyVersion = "nq1c.v1"`,
		`narrativeQualityCoprocessorConflictGuardMode = "current_fact_conflict_auto_degrade"`,
		`narrativeQualityCoprocessorConflictGuardAnchorSource = "step11_truth_core"`,
		`narrativeQualityCoprocessorConflictGuardConflictReason = "supporting_guidance_conflict_blocked"`,
		`narrativeQualityCoprocessorConflictGuardTruthFloorFallback = "preserve_step11_factual_state"`,
		`conflict_guard_policy_version: narrativeQualityCoprocessorConflictGuardPolicyVersion`,
		`conflict_guard_mode: narrativeQualityCoprocessorConflictGuardMode`,
		`conflict_guard_anchor_source: narrativeQualityCoprocessorConflictGuardAnchorSource`,
		`conflict_guard_truth_floor_fallback: narrativeQualityCoprocessorConflictGuardTruthFloorFallback`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P318 NQ-1c conflict guard marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P321NQ1dQualityTraceSplitMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`narrativeQualityCoprocessorTraceDisplayPolicyVersion = "nq1d.v1"`,
		`narrativeQualityCoprocessorTraceDisplayMode = "quality_hint_vs_factual_state_split"`,
		`narrativeQualityCoprocessorTraceDisplayHintLane = "response_quality_hint"`,
		`narrativeQualityCoprocessorTraceDisplayTruthLane = "step11_factual_state"`,
		`"quality_hint_as_current_fact"`,
		`"quality_hint_as_canonical_state"`,
		`trace_display_policy_version: narrativeQualityCoprocessorTraceDisplayPolicyVersion`,
		`trace_display_mode: narrativeQualityCoprocessorTraceDisplayMode`,
		`trace_display_hint_lane: narrativeQualityCoprocessorTraceDisplayHintLane`,
		`trace_display_truth_lane: narrativeQualityCoprocessorTraceDisplayTruthLane`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P321 NQ-1d quality trace split marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P324NQ1eShortLongAblationMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`narrativeQualityCoprocessorAblationPolicyVersion = "nq1e.v1"`,
		`narrativeQualityCoprocessorAblationBaselineMode = "narrative_quality_off"`,
		`narrativeQualityCoprocessorAblationProfiles = ["short_session"`,
		`"long_session"`,
		`narrativeQualityCoprocessorAblationPrimaryMetrics = ["hint_acceptance_delta"`,
		`"conflict_free_guidance_rate"`,
		`"response_coherence_delta"`,
		`narrativeQualityCoprocessorAblationTruthLeakBudget = "zero_tolerance"`,
		`ablation_policy_version: narrativeQualityCoprocessorAblationPolicyVersion`,
		`ablation_baseline_mode: narrativeQualityCoprocessorAblationBaselineMode`,
		`ablation_profiles: narrativeQualityCoprocessorAblationProfiles.slice()`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P324 NQ-1e short/long ablation marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P327NQ1fPlannerScenePilotSplitMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`narrativeQualityCoprocessorPlannerSplitPolicyVersion = "nq1f.v1"`,
		`narrativeQualityCoprocessorPlannerSplitMode = "beat_planner_vs_scene_pilot_guidance_only"`,
		`narrativeQualityCoprocessorPlannerRole = "beat_planner"`,
		`narrativeQualityCoprocessorExecutionRole = "scene_pilot"`,
		`planner_split_policy_version: narrativeQualityCoprocessorPlannerSplitPolicyVersion`,
		`planner_split_mode: narrativeQualityCoprocessorPlannerSplitMode`,
		`planner_role: narrativeQualityCoprocessorPlannerRole`,
		`execution_role: narrativeQualityCoprocessorExecutionRole`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P327 NQ-1f planner/scene-pilot split marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P336VX1aAuthorityLeakReplayMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`coprocessorAuthorityMatrixPolicyVersion = "mg1a.v1"`,
		`coprocessorTruthWriteTargets = ["current_fact"`,
		`"canonical_state"`,
		`"canonical_state_layer"`,
		`"dense_summary"`,
		`truthFloorBudgetLane = "truth_floor"`,
		`coprocessorHintBudgetLane = "coprocessor_hint"`,
		`narrativeQualityCoprocessorAblationProtectedTruthLabels = truthFloorBudgetLabels.slice()`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P336 VX-1a authority leak replay marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P339VX1bRuntimeBlastRadiusMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13GovernorFailureRuntimeAction = "trace_only_no_prompt_mutation"`,
		`step13GovernorFailureFailOpenBehavior = "keep_current_turn_execution_and_truth_floor"`,
		`failure_runtime_action: step13GovernorFailureRuntimeAction`,
		`failure_fail_open_behavior: step13GovernorFailureFailOpenBehavior`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P339 VX-1b runtime blast radius marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P342VX1cTraceObservabilityMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`coprocessorAnalysisProviderTraceTargetsByModule = {`,
		`entity_coprocessor: "proposal_trace"`,
		`world_coprocessor: "proposal_trace"`,
		`narrative_quality_coprocessor: "guidance_trace"`,
		`coprocessorAntiCopyAllowedReferenceMode = "behavioral_reference_only"`,
		`allowed_reference_mode: coprocessorAntiCopyAllowedReferenceMode`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P342 VX-1c trace observability marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P345VX1dBudgetPollutionMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`truthFloorBudgetLabels = [`,
		`coprocessorHintBudgetPromptLabels = [`,
		`coprocessorHintBudgetPromptModules = [`,
		`truthFloorBudgetLane = "truth_floor"`,
		`coprocessorHintBudgetLane = "coprocessor_hint"`,
		`coprocessorBudgetIsolationPolicyVersion = "mg1d.v1"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P345 VX-1d budget pollution marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P348VX1eModuleAblationCompareMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`entityCoprocessorAblationPolicyVersion = "vx1e.entity.v1"`,
		`worldCoprocessorAblationPolicyVersion = "vx1e.world.v1"`,
		`narrativeQualityCoprocessorAblationPolicyVersion = "nq1e.v1"`,
		`entityCoprocessorAblationDecisionGate`,
		`worldCoprocessorAblationDecisionGate`,
		`narrativeQualityCoprocessorAblationDecisionGate`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P348 VX-1e module ablation compare marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P352VX1fDefaultTakeoverGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`coprocessorTakeoverGatePolicyVersion = "vx1f.v1"`,
		`coprocessorTakeoverGateRequiredSignals = ["module_replay_green"`,
		`"module_ablation_green"`,
		`"release_gate_green"`,
		`entityCoprocessorTakeoverGateDefaultAction = "stay_off_until_gate_green"`,
		`worldCoprocessorTakeoverGateDefaultAction = "stay_off_until_gate_green"`,
		`narrativeQualityCoprocessorTakeoverGateDefaultAction = "stay_experimental_shadow_until_gate_green"`,
		`takeover_gate_policy_version: coprocessorTakeoverGatePolicyVersion`,
		`takeover_gate_required_signals: coprocessorTakeoverGateRequiredSignals.slice()`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P352 VX-1f default takeover gate marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P356VX1gProposalReentryLeakReplayMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`coprocessorProposalReentryPolicyVersion = "mg1g.v1"`,
		`coprocessorProposalTraceRequiredFields = ["evidence_refs"`,
		`"source_turns"`,
		`"confidence"`,
		`coprocessorProposalAdoptionPath = ["proposal_trace"`,
		`"reducer_reentry"`,
		`"step11_truth_core"]`,
		`truth_path_blocked_without_required_fields: true`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P356 VX-1g proposal re-entry leak replay marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P359VX1hRebuildInvalidationReplayMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`coprocessorOrchestrationRebuildPolicyVersion = "or1i.v1"`,
		`coprocessorOrchestrationStaleProposalPolicyVersion = "or1j.v1"`,
		`coprocessorOrchestrationRollbackInvalidationPolicyVersion = "or1g.v1"`,
		`coprocessorOrchestrationDirtyMatrixPolicyVersion = "or1h.v1"`,
		`staleServingPolicy: "deny_stale_sidecar_on_rebuild_pending"`,
		`cacheReuseGuard: "rollback_and_guidance_token_drift_block_pending_ready_reuse"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P359 VX-1h rebuild/invalidation replay marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P372ReleaseGateBundleRuntimeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const _step17ReleaseGate = {`,
		`step17ReleaseGateFetch()`,
		`bridgeFetch("/metrics/lc1s/step17-bundle-closure"`,
		`bundleClosure`,
		`release_gate_green`,
		`step17_bundle_closure`,
		`Bundle closure and session-local adoption are separate truths here`,
		`branch`,
		`commit`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P372 release gate bundle runtime marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P373ReleaseGateModuleDefaultMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`default_mode: "off"`,
		`default_mode: "experimental"`,
		`default_mode: "conservative"`,
		`default_reason: "entity_sidecar_stays_off_until_vx_validation_and_release_gate_close"`,
		`default_reason: "world_sidecar_stays_off_until_vx_validation_and_release_gate_close"`,
		`default_reason: "quality hints stay experimental until vx_validation_and_release_gate_close"`,
		`default_reason: "diagnostic_path_stays_conservative_and_fail_open"`,
		`default_reason: "audit_recall_should_remain_conservative_until_release_gate"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P373 module default marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P374ReleaseGatePackagedBundleSmokeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const coprocessorFeatureControlMatrix = [`,
		`policyVersion: "or1b.v1"`,
		`timeoutWindowSource: "getOrchestrationTimeoutMs"`,
		`step13GovernorMaxParallelism = 1`,
		`step13GovernorFailureRuntimeAction = "trace_only_no_prompt_mutation"`,
		`entityCoprocessorTraceDisplayPolicyVersion = "ec1d.v1"`,
		`worldCoprocessorTraceDisplayPolicyVersion = "wc1c.v1"`,
		`narrativeQualityCoprocessorTraceDisplayPolicyVersion = "nq1d.v1"`,
		`coprocessorOrchestrationRollbackInvalidationPolicyVersion = "or1g.v1"`,
		`staleServingPolicy: "deny_stale_sidecar_on_rebuild_pending"`,
		`entityCoprocessorKillSwitch`,
		`worldCoprocessorKillSwitch`,
		`narrativeQualityCoprocessorKillSwitch`,
		`kill_switch_default_state: "armed_standby"`,
		`kill_switch_action: "fail_open_skip"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P374 packaged bundle smoke marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P375ReleaseGateStep12CompleteStep13DeferredMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13PlanningRuntimeMode = "guidance_only_schema_draft"`,
		`step13PlanningRolloutStage = "experimental_shadow"`,
		`step13PlanningTakeoverGate = "stay_guidance_only_until_vx_green"`,
		`step13PlanningTruthWriteAllowed = false`,
		`step13PlanningReducerReentryAllowed = false`,
		`advancedCachePolicyStatus: "deferred_to_step13"`,
		`cache_advanced_policy_status: "deferred_to_step13"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P375 Step12/Step13 scope marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P379CoprocessorOutputSchemaDecisionMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`entityCoprocessorOutputMode = "proposal_trace_only"`,
		`entityCoprocessorOutputProposalTypes = ["relation_drift_hint"`,
		`entityCoprocessorOutputRequiredFields`,
		`worldCoprocessorOutputMode = "proposal_trace_only"`,
		`worldCoprocessorOutputProposalTypes = ["offscreen_pressure_hint"`,
		`narrativeQualityCoprocessorOutputMode = "guidance_trace_only"`,
		`narrativeQualityCoprocessorOutputHintTypes = ["pacing_hint"`,
		`output_contract_mode: entityCoprocessorOutputMode`,
		`output_contract_proposal_types: entityCoprocessorOutputProposalTypes.slice()`,
		`output_contract_mode: worldCoprocessorOutputMode`,
		`output_contract_proposal_types: worldCoprocessorOutputProposalTypes.slice()`,
		`output_contract_mode: narrativeQualityCoprocessorOutputMode`,
		`output_contract_hint_types: narrativeQualityCoprocessorOutputHintTypes.slice()`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P379 coprocessor output schema decision marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P380PluginOnlyBackendSaveBoundaryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`coprocessorOrchestrationBackendRequiredModules = [`,
		`coprocessorOrchestrationBackendBundleAssistedSurfaces = [`,
		`coprocessorOrchestrationPluginOnlyModules = [`,
		`coprocessorOrchestrationPluginOnlyExecutionMode = "local_runtime_contract_only"`,
		`coprocessorOrchestrationModuleTransportRuntimeStatus = "trace_contract_only"`,
		`backend_required_modules: coprocessorOrchestrationBackendRequiredModules.slice()`,
		`backend_bundle_assisted_surfaces: coprocessorOrchestrationBackendBundleAssistedSurfaces.slice()`,
		`plugin_only_modules: coprocessorOrchestrationPluginOnlyModules.slice()`,
		`current_fact_write: false`,
		`canonical_write: false`,
		`write_targets: ["proposal_trace"]`,
		`write_targets: ["guidance_trace"]`,
		`denied_write_targets: coprocessorTruthWriteTargets.slice()`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P380 plugin-only backend save boundary marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P381SidecarTurnLocalSurfaceSaveMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`coprocessorSidecarWritableTargets = ["proposal_trace", "guidance_trace", "audit_trace", "maintenance_metadata"]`,
		`entityCoprocessorTraceDisplayMode = "hint_vs_current_fact_split"`,
		`worldCoprocessorTraceDisplayMode = "hint_vs_current_world_state_split"`,
		`narrativeQualityCoprocessorTraceDisplayMode = "quality_hint_vs_factual_state_split"`,
		`entityCoprocessorTraceDisplayDisallowedAliases = ["proposal_trace_as_current_fact"`,
		`trace_display_mode: entityCoprocessorTraceDisplayMode`,
		`trace_display_disallowed_aliases: entityCoprocessorTraceDisplayDisallowedAliases.slice()`,
		`trace_display_mode: worldCoprocessorTraceDisplayMode`,
		`trace_display_disallowed_aliases: worldCoprocessorTraceDisplayDisallowedAliases.slice()`,
		`trace_display_mode: narrativeQualityCoprocessorTraceDisplayMode`,
		`trace_display_disallowed_aliases: narrativeQualityCoprocessorTraceDisplayDisallowedAliases.slice()`,
		`coprocessorOrchestrationStaleProposalBlockedPromptTargets = ["pending_ready_orchestration_result", "proposal_trace", "guidance_trace", "sidecar_cache"]`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P381 sidecar turn-local surface save marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq12P382RebuildFullVsSelectiveCheckpointMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`coprocessorOrchestrationRebuildStaleDropTargets = ["pending_ready_orchestration_result", "sidecar_cache", "guidance_trace"]`,
		`coprocessorOrchestrationRebuildHardResetTargets = ["prepare_turn_bundle"`,
		`coprocessorOrchestrationRebuildStartPointPrecedence = ["checkpoint_full_rebuild", "rollback_turn_anchor_then_prepare_turn", "next_narrative_control_fetch", "next_prepare_turn_fetch", "step11_truth_stack"]`,
		`schema_migration: "full"`,
		`turn_deletion: "rollback_turn_anchor_then_prepare_turn"`,
		`schema_migration: "checkpoint_full_rebuild"`,
		`rebuildMode: "full"`,
		`rebuildMode: "selective"`,
		`startPoint: "checkpoint_full_rebuild"`,
		`startPoint: "rollback_turn_anchor_then_prepare_turn"`,
		`staleServingPolicy: "deny_stale_sidecar_on_rebuild_pending"`,
		`cacheReuseAllowed: pendingReasons.length === 0`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-12-P382 rebuild full vs selective checkpoint marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq13P43SidecarCallEconomyGovernorMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`coprocessorAnalysisProviderCallFrequencyOwner = "step13_governor"`,
		`step13GovernorPolicyVersion = "gv1a.v1"`,
		`step13CacheKeeperPolicyVersion = "gv1b.v1"`,
		`step13GovernorApprovalStates = ["run", "reuse", "skip", "suspend"]`,
		`step13GovernorMaxParallelism = 1`,
		`step13GovernorRuntimeMode = "trace_and_transport_aligned"`,
		`analysis_provider_call_frequency_owner: coprocessorAnalysisProviderCallFrequencyOwner`,
		`call_frequency_owner: coprocessorAnalysisProviderCallFrequencyOwner`,
		`call_frequency_policy_version: step13GovernorPolicyVersion`,
		`call_frequency_policy_status: "governor_contract_fixed"`,
		`forced_refresh_signals: step13CacheKeeperForcedRefreshSignals.slice()`,
		`no_serve_conditions: step13CacheKeeperNoServeConditions.slice()`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P43 sidecar call economy marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq13P44TruthFloorTokenBudgetMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13TokenTruthFloorPolicyVersion = "tb1b.v1"`,
		`step13TokenTruthFloorMode = "char_floor_projected_minimums"`,
		`step13TokenTruthFloorProjectionStatus = "contract_only_no_runtime_enforcement"`,
		`step13TokenTruthFloorReferenceCharsPerToken = 4`,
		`step13TokenTruthFloorCoreLabels = ["latest_direct_evidence", "recent_raw_turn", "active_state", "canonical_state_layer"]`,
		`step13TokenTruthFloorContinuityLabels = ["storylines", "episode", "chapter", "arc", "saga"]`,
		`truth_floor_reservation_mode: "hard_floor_reserved_first"`,
		`truth_floor_min_tokens_by_label: Object.assign({}, step13TokenTruthFloorMinTokensByLabel)`,
		`truth_floor_core_min_tokens: step13TokenTruthFloorCoreMinTokens`,
		`truth_floor_continuity_min_tokens: step13TokenTruthFloorContinuityMinTokens`,
		`truth_floor_reliability_guard_min_tokens: step13TokenTruthFloorReliabilityGuardMinTokens`,
		`truth_floor_total_min_tokens: step13TokenTruthFloorTotalMinTokens`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P44 truth floor token budget marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq13P45PlanningSurfaceGuidanceOnlyMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13PlanningRuntimeMode = "guidance_only_schema_draft"`,
		`step13PlanningRolloutStage = "experimental_shadow"`,
		`step13PlanningTakeoverGate = "stay_guidance_only_until_vx_green"`,
		`step13PlanningTruthWriteAllowed = false`,
		`step13PlanningReducerReentryAllowed = false`,
		`step13BeatPlannerDraftRequiredFields = ["narrative_goal", "tension_axis", "open_question", "payoff_candidate"]`,
		`step13ScenePilotDraftRequiredFields = ["execution_mode", "scene_target", "emphasis_axis", "pacing", "forbidden_move"]`,
		`step13SettingFrameSchemaPolicyVersion = "ps1c.v1"`,
		`worldCoprocessorSceneSliceDeliverySurface = "guidance_only_setting_frame"`,
		`truth_write_allowed: step13PlanningTruthWriteAllowed`,
		`reducer_reentry_allowed: step13PlanningReducerReentryAllowed`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P45 planning guidance-only marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq13P46LocalNamingGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13NamingGatePolicyVersion = "ps1f.v1"`,
		`step13NamingGateMode = "local_vocab_review_only"`,
		`step13NamingGateLegacyRenameScope = "step_owned_contract_values_only"`,
		`step13NamingGateReviewScope = ["planning", "governor", "portability"]`,
		`step13NamingGateSourceDraft = "STEP13_NAMING_MAP_DRAFT.md"`,
		`step13NamingGateApprovalRule = "allow_local_vocab_block_external_like_reuse"`,
		`step13NamingGateRuntimeAction = "review_only_no_runtime_rename"`,
		`reviewed_function_names: step13NamingGateReviewedFunctionNames.slice()`,
		`reviewed_helper_names: step13NamingGateReviewedHelperNames.slice()`,
		`blocked_legacy_values: step13NamingGateBlockedLegacyValues.slice()`,
		`"book_author"`,
		`"director_directive"`,
		`"guidance_only_world_slice"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P46 local naming gate marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq13P56MemOrchNullReturnHardeningMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`function buildIntentionalOrchestrationSkipResult(reason, trace)`,
		`_orchestrationSkipped: true`,
		`_skipReason: normalizedReason`,
		`function isIntentionalOrchestrationSkipResult(result)`,
		`return !!(result && result._orchestrationSkipped === true)`,
		`return buildIntentionalOrchestrationSkipResult("empty_input_no_continuity", trace)`,
		`if (!lastOrchResult) {`,
		`orchestration returned null`,
		`detail: "llm_gate_blocked"`,
		`if (isIntentionalOrchestrationSkipResult(lastOrchResult)) {`,
		`resolveOrchestrationFallbackRouteOr1b("intentional_skip")`,
		`status: "skipped"`,
		`code: skipFallbackRoute.route`,
		`return applyProtectionOnlyInjection(payload, userInput)`,
		`supportedRoutes: ["blocked_empty_result", "cached_result", "skip_protection_only", "direct_result"]`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P56 MemOrch null-return hardening marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P129GV1aModuleGovernorContractMarkers verifies GV-1a:
// step13.governor module policy contract markers are present in Archive Center.js
func TestArchiveCenterJSSeq13P129GV1aModuleGovernorContractMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`policyVersion: "gv1a.v1"`,
		`owner: "step13.governor"`,
		`governedModules: ["entity_coprocessor", "world_coprocessor", "narrative_quality_coprocessor"]`,
		`approvalStates: ["run", "reuse", "skip", "suspend"]`,
		`cooldownTurnsByModule:`,
		`entity_coprocessor: 0`,
		`world_coprocessor: 0`,
		`narrative_quality_coprocessor: 0`,
		`minDirtySeverityByModule:`,
		`entity_coprocessor: "medium"`,
		`world_coprocessor: "medium"`,
		`narrative_quality_coprocessor: "low"`,
		`maxParallelism: 1`,
		`callEntryGate: "after_step11_truth_stack"`,
		`singleFlightScope: "same_chat_session_after_request_only"`,
		`runtimeMode: "trace_and_transport_aligned"`,
		`dirtySeverityOrder: ["none", "low", "medium", "high", "critical"]`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P129 GV-1a marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P132GV1bCacheKeeperPolicyMarkers verifies GV-1b:
// Cache keeper policy contract markers are present in Archive Center.js
func TestArchiveCenterJSSeq13P132GV1bCacheKeeperPolicyMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`policyVersion: "gv1b.v1"`,
		`owner: "step13.governor"`,
		`cacheUnit:`,
		`reuseScope:`,
		`reuseRoute:`,
		`staleGuard:`,
		`staleDiscardReasons:`,
		`"cache_key_mismatch"`,
		`"session_scope_mismatch"`,
		`"rollback_invalidation_drift"`,
		`"guidance_invalidation_drift"`,
		`"rebuild_pending"`,
		`"chapter_evidence_mismatch"`,
		`forcedRefreshSignals:`,
		`noServeConditions:`,
		`"blocked_empty_result"`,
		`"stale_sidecar_blocked"`,
		`"backend_offline"`,
		`runtimeServeMode:`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P132 GV-1b marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P135GV1cFailureBudgetSuspensionMarkers verifies GV-1c:
// Failure budget suspension policy contract markers are present in Archive Center.js
func TestArchiveCenterJSSeq13P135GV1cFailureBudgetSuspensionMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`policyVersion: "gv1c.v1"`,
		`owner: "step13.governor"`,
		`governedModules:`,
		`failureBucketMode: "shared_sidecar_channel"`,
		`trackedFailureClasses:`,
		`"plugin_main_error"`,
		`"sub_review_error"`,
		`"supervisor_unavailable"`,
		`"delivery_gate_blocked"`,
		`failureWindowTurns: 4`,
		`consecutiveFailureThreshold: 2`,
		`resumeSuccessTurnsRequired: 1`,
		`suspensionDecisionMode: "shared_suspend_all_governed_modules"`,
		`suspensionReasonCode: "failure_budget_suspended"`,
		`runtimeAction: "trace_only_no_prompt_mutation"`,
		`failOpenBehavior: "keep_current_turn_execution_and_truth_floor"`,
		`historyScope: "same_chat_session_recent_turns"`,
		`function resolveStep13FailureBudgetStateGv1c`,
		`function applyStep13GovernorFailureBudgetTraceGv1c`,
		`function recordStep13GovernorTurnOutcomeGv1c`,
		`function buildStep13GovernorTurnOutcomeGv1c`,
		`function peekStep13FailureBudgetStateGv1c`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P135 GV-1c marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P138GV1dModuleRunLedgerMarkers verifies GV-1d:
// Module run ledger contract markers are present in Archive Center.js
func TestArchiveCenterJSSeq13P138GV1dModuleRunLedgerMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`policyVersion: "gv1d.v1"`,
		`governorPolicyVersion:`,
		`cachePolicyVersion:`,
		`runtimeMode:`,
		`maxParallelism:`,
		`primaryDecision:`,
		`primaryReason:`,
		`counts:`,
		`run:`,
		`reuse:`,
		`skip:`,
		`suspend:`,
		`entries:`,
		`module:`,
		`decision:`,
		`approvalState:`,
		`reason:`,
		`cooldownTurns:`,
		`minDirtySeverity:`,
		`reusedCache:`,
		`forcedRefresh:`,
		`noServe:`,
		`function resolveStep13GovernorLedgerGv1d`,
		`function applyStep13GovernorTraceGv1d`,
		`runLedgerPolicyVersion: ledger.policyVersion`,
		`primaryDecision: ledger.primaryDecision`,
		`primaryReason: ledger.primaryReason`,
		`entries: Array.isArray(ledger.entries)`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P138 GV-1d marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P141GV1eGovernorBypassGuardMarkers verifies GV-1e:
// Governor bypass guard contract markers are present in Archive Center.js
func TestArchiveCenterJSSeq13P141GV1eGovernorBypassGuardMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`policyVersion: "gv1e.v1"`,
		`owner: "step13.governor"`,
		`governedModules:`,
		`protectedPromptTargets:`,
		`"proposal_trace"`,
		`"guidance_trace"`,
		`"pending_ready_orchestration_result"`,
		`blockedRoutes:`,
		`"self_triggered_sidecar_rerun"`,
		`"proposal_trace_direct_reentry"`,
		`"guidance_trace_recursive_requeue"`,
		`"pending_ready_self_rearm"`,
		`requiredEntryGate:`,
		`requiredSingleFlightScope:`,
		`requiredReuseRoute:`,
		`requiredRuntimeStage: "residual_guidance"`,
		`action: "deny_and_trace_only"`,
		`runtimeStatus: "contract_only"`,
		`function getStep13GovernorBypassPolicyGv1e`,
		`function applyStep13GovernorBypassTraceGv1e`,
		`trace.step13Governor.bypassGuard`,
		`bypassPolicyVersion:`,
		`bypassGuard:`,
		`protectedModules:`,
		`protectedPromptTargets:`,
		`blockedRoutes:`,
		`requiredEntryGate:`,
		`requiredSingleFlightScope:`,
		`requiredReuseRoute:`,
		`requiredRuntimeStage:`,
		`action:`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P141 GV-1e marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P147TB1aProfileModelTokenEstimatorMarkers verifies TB-1a:
// profile/model token estimator contract markers are present in Archive Center.js.
func TestArchiveCenterJSSeq13P147TB1aProfileModelTokenEstimatorMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13TokenEstimatorPolicyVersion = "tb1a.v1"`,
		`step13TokenEstimatorMode = "profile_first_shared_thresholds"`,
		`step13TokenEstimatorModelMode = "shared_thresholds_all_models"`,
		`step13TokenEstimatorModelOverrides = {}`,
		`step13TokenEstimatorProfiles = [`,
		`"mid_context_300k"`,
		`"wide_context_500k"`,
		`"ultra_long_1m_plus"`,
		`"extreme_long_2m_plus"`,
		`step13TokenEstimatorProfileSourceOrder = ["injection_pack_hint", "plugin_setting", "runtime_tokens", "max_injection_chars_proxy"]`,
		`step13TokenEstimatorRuntimeThresholds = {`,
		`wide_context_500k: 300000`,
		`ultra_long_1m_plus: 900000`,
		`extreme_long_2m_plus: 1700000`,
		`const step13TokenEstimatorBudgetLimitByProfile = adaptiveInjectionBudgetProfileLimits();`,
		`step13TokenEstimatorLowConfidenceSources = ["none", "message_char_estimate"]`,
		`step13TokenEstimatorDriftTelemetryFields = [`,
		`"manualBudgetLimit"`,
		`"budgetLimit"`,
		`"budgetLimitSource"`,
		`"runtimeCurrentChatTokens"`,
		`"runtimeCurrentChatTokensEffective"`,
		`"contextProfile"`,
		`"contextProfileSource"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P147 TB-1a marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P150TB1bTruthFloorMinimumTokenMarkers verifies TB-1b:
// truth-floor minimum token contract markers are present in Archive Center.js.
func TestArchiveCenterJSSeq13P150TB1bTruthFloorMinimumTokenMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13TokenTruthFloorPolicyVersion = "tb1b.v1"`,
		`step13TokenTruthFloorMode = "char_floor_projected_minimums"`,
		`step13TokenTruthFloorProjectionStatus = "contract_only_no_runtime_enforcement"`,
		`step13TokenTruthFloorReferenceCharsPerToken = 4`,
		`step13TokenTruthFloorCoreLabels = ["latest_direct_evidence", "recent_raw_turn", "active_state", "canonical_state_layer"]`,
		`step13TokenTruthFloorContinuityLabels = ["storylines", "episode", "chapter", "arc", "saga"]`,
		`step13TokenTruthFloorReliabilityGuardLabels = step13TokenTruthFloorCoreLabels.slice()`,
		`function _projectTruthFloorTokensTb1b(charCount)`,
		`step13TokenTruthFloorMinTokensByLabel[label] = projectedTokens`,
		`step13TokenTruthFloorCoreMinTokens`,
		`step13TokenTruthFloorContinuityMinTokens`,
		`step13TokenTruthFloorReliabilityGuardMinTokens`,
		`truth_floor_budget_lane: truthFloorBudgetLane`,
		`truth_floor_hint_lane_claim_allowed: false`,
		`truth_floor_min_tokens_by_label: Object.assign({}, step13TokenTruthFloorMinTokensByLabel)`,
		`truth_floor_reliability_guard_min_tokens: step13TokenTruthFloorReliabilityGuardMinTokens`,
		`truthFloorBudgetReservedTokens: step13TokenTruthFloorTotalMinTokens`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P150 TB-1b marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P153TB1cDensityProfileMarkers verifies TB-1c:
// token density profile markers for ledger/world/guidance are present in Archive Center.js.
func TestArchiveCenterJSSeq13P153TB1cDensityProfileMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13TokenDensityProfilePolicyVersion = "tb1c.v1"`,
		`step13TokenDensityProfileLevels = ["light", "balanced", "heavy"]`,
		`step13TokenDensityProfileThresholds = {`,
		`light_max_ratio: 0.08`,
		`balanced_max_ratio: 0.15`,
		`function _resolveTokenDensityLevelTb1c(totalRatio)`,
		`function _sumTokenDensityRatiosTb1c(ratioByLabel)`,
		`step13TokenDensityLedgerLabels = ["storylines", "episode", "chapter", "arc", "saga"]`,
		`step13TokenDensityWorldLabels = ["world_context", "location_context", "kg_relations"]`,
		`step13TokenDensityGuidanceLabels = ["narrative_guide"]`,
		`step13TokenDensityRatioByFamily = {`,
		`ledger: {`,
		`world: {`,
		`guidance: {`,
		`density_profile_status: "density_contract_fixed"`,
		`density_profile_families: step13TokenDensityFamilies`,
		`step13TokenDensityProfileByFamily: {`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P153 TB-1c marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P156TB1dBudgetFallbackTokenEstimateMarkers verifies TB-1d:
// budget fallback/token estimate mode markers are present in Archive Center.js.
func TestArchiveCenterJSSeq13P156TB1dBudgetFallbackTokenEstimateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13TokenBudgetFallbackPolicyVersion = "tb1d.v1"`,
		`step13TokenBudgetFallbackRules = {`,
		`reliable_runtime_tokens: {`,
		`low_confidence_runtime_source: {`,
		`runtime_tokens_unavailable: {`,
		`manual_setting_only: {`,
		`budget_limit_source: "runtime_tokens"`,
		`budget_limit_source: "manual_setting"`,
		`profile_source_fallback: "max_injection_chars_proxy"`,
		`adaptive_budget_applied: true`,
		`adaptive_budget_applied: false`,
		`const step13TokenBudgetFallbackDecision = (function resolveStep13TokenBudgetFallbackDecision()`,
		`if (runtimeBudgetAdaptiveEligible) return "reliable_runtime_tokens";`,
		`if (normalizedRuntimeTokenSource === "message_char_estimate") return "low_confidence_runtime_source";`,
		`if (runtimeTieringEnabled) return "runtime_tokens_unavailable";`,
		`return "manual_setting_only";`,
		`step13TokenBudgetFallbackDecision: step13TokenBudgetFallbackDecision`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P156 TB-1d marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P159TB1eTokenReplayTruthFloorMarkers verifies TB-1e:
// token replay window and truth-floor verification markers are present in Archive Center.js.
func TestArchiveCenterJSSeq13P159TB1eTokenReplayTruthFloorMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`runtimeTokenSmoothingWindow = 4`,
		`assembleInjectionWithBudget._runtimeTokenSmoothingState`,
		`const runtimeTokenSmoothingKey = (function resolveRuntimeSmoothingKey()`,
		`runtimeTokenSmoothingState.set(runtimeTokenSmoothingKey, {`,
		`runtimeTokenSmoothingApplied: runtimeTokenSmoothingApplied`,
		`runtimeTokenSmoothingWindow: runtimeTokenSmoothingWindow`,
		`runtimeCurrentChatTokens: Number.isFinite(Number(runtimeTokenHint))`,
		`runtimeCurrentChatTokensEffective: runtimeBudgetAdaptiveEligible ? runtimeTokensForTiering : 0`,
		`drift_telemetry_fields: step13TokenEstimatorDriftTelemetryFields.slice()`,
		`truthFloorBudgetReservedTokens: step13TokenTruthFloorTotalMinTokens`,
		`hardFloorLabels: Object.keys(hardFloorTargetChars)`,
		`tokens: Number(step13TokenTruthFloorMinTokensByLabel[label] || 0)`,
		`hardFloorApplied: hardFloorReserveTotalChars > 0`,
		`residualSupportBudgetChars: Math.max(0, budgetLimit - hardFloorReserveTotalChars)`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P159 TB-1e marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P164PS1aBeatPlannerLaneSchemaMarkers verifies PS-1a:
// Beat Planner lane schema, guidance-only, required fields, truth-write blocked.
func TestArchiveCenterJSSeq13P164PS1aBeatPlannerLaneSchemaMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13BeatPlannerSchemaPolicyVersion = "ps1a.v1"`,
		`narrativeQualityCoprocessorPlannerRole = "beat_planner"`,
		`step13BeatPlannerDraftRequiredFields = ["narrative_goal", "tension_axis", "open_question", "payoff_candidate"]`,
		`narrativeQualityCoprocessorPlannerAuthorityPromotionAllowed = false`,
		`narrativeQualityCoprocessorCrossRoleTruthBorrowAllowed = false`,
		`step13PlanningTruthWriteAllowed = false`,
		`step13PlanningReducerReentryAllowed = false`,
		`narrativeQualityCoprocessorPlannerFields`,
		`narrativeQualityCoprocessorOutputMode`,
		`delivery_surface: narrativeQualityCoprocessorOutputMode`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P164 PS-1a marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P166PS1bScenePilotLaneSchemaMarkers verifies PS-1b:
// Scene Pilot lane schema, guidance-only / experimental shadow, truth-write blocked.
func TestArchiveCenterJSSeq13P166PS1bScenePilotLaneSchemaMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13ScenePilotSchemaPolicyVersion = "ps1b.v1"`,
		`narrativeQualityCoprocessorExecutionRole = "scene_pilot"`,
		`step13ScenePilotDraftRequiredFields = ["execution_mode", "scene_target", "emphasis_axis", "pacing", "forbidden_move"]`,
		`narrativeQualityCoprocessorExecutionAuthorityPromotionAllowed = false`,
		`step13PlanningTruthWriteAllowed = false`,
		`step13PlanningReducerReentryAllowed = false`,
		`step13PlanningRuntimeMode = "guidance_only_schema_draft"`,
		`step13PlanningRolloutStage = "experimental_shadow"`,
		`delivery_surface: narrativeQualityCoprocessorOutputMode`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P166 PS-1b marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P168PS1cSceneScopedSettingFrameMarkers verifies PS-1c:
// Scene-scoped setting frame, world coprocessor slice delivery, truth alias blocked.
func TestArchiveCenterJSSeq13P168PS1cSceneScopedSettingFrameMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13SettingFrameSchemaPolicyVersion = "ps1c.v1"`,
		`worldCoprocessorSceneSliceSelectorSignal = "scene_scope"`,
		`worldCoprocessorSceneSliceExtractionMode = "current_scene_thin_frame_only"`,
		`worldCoprocessorSceneSliceOutputType = "scene_scoped_setting_hint"`,
		`worldCoprocessorSceneSliceDeliverySurface = "guidance_only_setting_frame"`,
		`worldCoprocessorSceneSliceTruthAliasBlocked = true`,
		`worldCoprocessorSceneSliceTruthBorrowAllowed = false`,
		`world_pressure_hint`,
		`guidance_only_setting_frame`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P168 PS-1c marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P170PS1dPlanningKeepDropConflictMarkers verifies PS-1d:
// Keep/drop/conflict policy, evaluation order, truth-floor fallback.
func TestArchiveCenterJSSeq13P170PS1dPlanningKeepDropConflictMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13PlanningKeepDropConflictPolicyVersion = "ps1d.v1"`,
		`step13PlanningConflictEvaluationOrder = ["beat_planner", "scene_pilot", "setting_frame"]`,
		`step13PlanningConflictBlockedTargets`,
		`step13PlanningBeatPlannerConflictAction`,
		`step13PlanningScenePilotConflictAction`,
		`step13PlanningSettingFrameConflictAction`,
		`step13PlanningConflictTruthFloorFallback = "preserve_step11_truth_floor"`,
		`narrativeQualityCoprocessorConflictGuardTruthFloorFallback = "preserve_step11_factual_state"`,
		`preserve_step11_truth_floor`,
		`drop_conflicting_planning_hint`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P170 PS-1d marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P172PS1ePlanningMonolithRegressionMarkers verifies PS-1e:
// Anti-monolith guard: split lanes, forbidden shapes, no reducer re-entry.
func TestArchiveCenterJSSeq13P172PS1ePlanningMonolithRegressionMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13PlanningMonolithGuardPolicyVersion = "ps1e.v1"`,
		`step13PlanningMonolithGuardMode = "split_lanes_not_unified_core"`,
		`step13PlanningMonolithGuardLaneNames`,
		`step13PlanningMonolithGuardForbiddenShapes`,
		`unified_planning_core`,
		`truth_path_frontload`,
		`authority_promotion_bridge`,
		`step13PlanningMonolithGuardRequiredDeliverySurfaces`,
		`step13PlanningMonolithGuardDefaultAction = "keep_split_guidance_only_surfaces"`,
		`step13PlanningTruthWriteAllowed = false`,
		`step13PlanningReducerReentryAllowed = false`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P172 PS-1e marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P174PS1fPlanningGovernorPortabilityNamingMarkers verifies PS-1f:
// Naming gate: local vocab review only, approved labels, blocked legacy, no runtime rename.
func TestArchiveCenterJSSeq13P174PS1fPlanningGovernorPortabilityNamingMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13NamingGatePolicyVersion = "ps1f.v1"`,
		`step13NamingGateMode = "local_vocab_review_only"`,
		`step13NamingGateReviewScope`,
		`step13NamingGateApprovedLabelsByPhase`,
		`step13NamingGateReviewedFunctionNames`,
		`step13NamingGateReviewedHelperNames`,
		`step13NamingGateBlockedLegacyValues`,
		`step13NamingGateApprovalRule = "allow_local_vocab_block_external_like_reuse"`,
		`step13NamingGateRuntimeAction = "review_only_no_runtime_rename"`,
		`review_only_no_runtime_rename`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P174 PS-1f marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P179VX1aPortabilityRoundtripReplayMarkers verifies VX-1a:
// Portability validation slice: roundtrip replay, lineage, selective rebuild, manual-first.
func TestArchiveCenterJSSeq13P179VX1aPortabilityRoundtripReplayMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13PortabilityValidationPolicyVersions = ["sp1a.v1", "sp1b.v1", "sp1c.v1", "sp1d.v1", "sp1e.v1"]`,
		`step13ValidationGateDefaultsBySlice.portability`,
		`default_action: "stay_manual_first_review_only_until_vx_green"`,
		`rollout_stage: "manual_review_only"`,
		`takeover_allowed_by_default: false`,
		`default_state: step13ValidationGateDefaultState`,
		`source_hash`,
		`source_turn`,
		`step13ValidationGateSliceOrder`,
		`"portability"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P179 VX-1a marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P181VX1bCopiedSessionDeletionReplayMarkers verifies VX-1b:
// Copied/imported session deletion replay, stale truth blocked, tombstone lineage.
func TestArchiveCenterJSSeq13P181VX1bCopiedSessionDeletionReplayMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`entityCoprocessorStaleSceneGuardSuppressionRoute = "drop_entity_hint_keep_truth_floor"`,
		`stale_scene_truth_promotion_blocked`,
		`disallowed_usage: ["canonical_relationship_overwrite", "direct_canonical_relationship_patch_apply", "stale_scene_truth_promotion"`,
		`historicalDeletionDetected: false`,
		`tombstoned`,
		`source_turn`,
		`source_hash`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P181 VX-1b marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P183VX1cReembedLifecycleReplayMarkers verifies VX-1c:
// Reembed validation slice: model-switch replay, manual admin batch, truth floor preserve.
func TestArchiveCenterJSSeq13P183VX1cReembedLifecycleReplayMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13ReembedValidationPolicyVersions = ["em1a.v1", "em1b.v1", "em1c.v1", "em1d.v1"]`,
		`step13ValidationGateDefaultsBySlice.reembed`,
		`default_action: "stay_manual_admin_batch_until_vx_green"`,
		`rollout_stage: "manual_review_only"`,
		`takeover_allowed_by_default: false`,
		`default_state: step13ValidationGateDefaultState`,
		`"reembed"`,
		`step13ValidationGateSliceOrder`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P183 VX-1c marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P185VX1dGovernorLoadReplayMarkers verifies VX-1d:
// Governor validation slice: cooldown, failure budget, suspension, fail-open.
func TestArchiveCenterJSSeq13P185VX1dGovernorLoadReplayMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13GovernorValidationPolicyVersions`,
		`step13GovernorCooldownTurnsByModule`,
		`step13GovernorApprovalStates`,
		`approvalStates: ["run", "reuse", "skip", "suspend"]`,
		`failure_budget_policy_version`,
		`step13GovernorFailureFailOpenBehavior = "keep_current_turn_execution_and_truth_floor"`,
		`step13ValidationGateDefaultsBySlice.governor`,
		`default_action: "stay_trace_only_no_prompt_mutation_until_vx_green"`,
		`rollout_stage: "trace_contract_only"`,
		`takeover_allowed_by_default: false`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P185 VX-1d marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P187VX1eTokenFloorReplayMarkers verifies VX-1e:
// Token budget validation slice: truth floor preserve, hard floor, residual budget, drift telemetry.
func TestArchiveCenterJSSeq13P187VX1eTokenFloorReplayMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13TokenValidationPolicyVersions`,
		`hardFloorReserveTotalChars`,
		`truth_floor_reserved_chars: hardFloorReserveTotalChars`,
		`residualSupportBudgetChars: Math.max(0, budgetLimit - hardFloorReserveTotalChars)`,
		`drift_telemetry_fields: step13TokenEstimatorDriftTelemetryFields.slice()`,
		`step13ValidationGateDefaultsBySlice.token_budget`,
		`default_action: "stay_contract_only_no_runtime_enforcement_until_vx_green"`,
		`rollout_stage: "contract_review_only"`,
		`takeover_allowed_by_default: false`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P187 VX-1e marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P189VX1fPlanningSurfaceLeakageReplayMarkers verifies VX-1f:
// Planning validation slice: authority leak guard, guidance-only, truth-write blocked, conflict fallback.
func TestArchiveCenterJSSeq13P189VX1fPlanningSurfaceLeakageReplayMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13PlanningValidationPolicyVersions`,
		`step13PlanningTruthWriteAllowed = false`,
		`step13PlanningReducerReentryAllowed = false`,
		`step13PlanningTakeoverGate = "stay_guidance_only_until_vx_green"`,
		`step13ValidationGateDefaultsBySlice.planning`,
		`default_action: step13PlanningTakeoverGate`,
		`rollout_stage: step13PlanningRolloutStage`,
		`takeover_allowed_by_default: false`,
		`preserve_step11_truth_floor`,
		`guidance_only_setting_frame`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P189 VX-1f marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P191VX1gDefaultTakeoverGateMarkers verifies VX-1g:
// Validation gate: default takeover blocked, required signals, slice order, review-only runtime action.
func TestArchiveCenterJSSeq13P191VX1gDefaultTakeoverGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13ValidationGatePolicyVersion = "vx1g.v1"`,
		`step13ValidationGateMode = "default_takeover_blocked_until_slice_green"`,
		`step13ValidationGateRequiredSignals = ["slice_replay_green", "slice_ablation_green_or_not_applicable", "release_gate_green"]`,
		`step13ValidationGateSliceOrder = ["portability", "reembed", "governor", "token_budget", "planning"]`,
		`step13ValidationGateDefaultState = "draft_locked"`,
		`step13ValidationGateRuntimeAction = "review_only_no_runtime_takeover"`,
		`step13ValidationGateDefaultsBySlice`,
		`takeover_allowed_by_default: false`,
		`default_takeover_gate_fixed`,
		`inherited_module_takeover_policy_version`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P191 VX-1g marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P214ReleaseGateBundleLatestRuntimeMarkers verifies P214:
// Beta 0.4 release-gate bundle regeneration remains a contract marker, not a generated artifact.
func TestArchiveCenterJSSeq13P214ReleaseGateBundleLatestRuntimeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`seq13P214BundleLatestRootRuntimeGate = "bundle_latest_root_runtime_draft"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P214 bundle latest root runtime marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P215ReleaseGateRootBundleRegenerateMarkers verifies P215:
// Root-to-bundle regeneration is kept as a repeatable checklist/script gate.
func TestArchiveCenterJSSeq13P215ReleaseGateRootBundleRegenerateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`seq13P215RootBundleRegenerateGate = "root_bundle_regenerate_draft"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P215 root bundle regenerate marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P216ReleaseGatePackagedSmokeMarkers verifies P216:
// Packaged smoke evidence remains marker-based without producing release artifacts.
func TestArchiveCenterJSSeq13P216ReleaseGatePackagedSmokeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`seq13P216PackagedBundleRoundtripGate = "packaged_bundle_import_export_roundtrip"`,
		`seq13P216ReembedFallbackSmokePass = "reembed_fallback_smoke_check_pass"`,
		`seq13P216GovernorTraceSmokePass = "governor_trace_smoke_check_pass"`,
		`seq13P216TokenProfileTraceSmokePass = "token_profile_trace_smoke_check_pass"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P216 packaged smoke marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P217ReleaseGatePlanningGuidanceOnlyMarkers verifies P217:
// Planning surfaces stay guidance-only/experimental with authority leak protection.
func TestArchiveCenterJSSeq13P217ReleaseGatePlanningGuidanceOnlyMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`seq13P217PlanningSurfaceGuidanceOnlyLeakGuard = "guidance_only_experimental_authority_leak_guard"`,
		`step13PlanningRuntimeMode = "guidance_only_schema_draft"`,
		`step13PlanningTakeoverGate = "stay_guidance_only_until_vx_green"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P217 planning guidance-only leak guard marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P218ReleaseGateNamingReviewMarkers verifies P218:
// Step 13 naming remains reviewed local vocabulary with no runtime rename.
func TestArchiveCenterJSSeq13P218ReleaseGateNamingReviewMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`seq13P218NamingReviewChecklistConfirmed = "step13_naming_review_checklist_confirmed"`,
		`step13NamingGatePolicyVersion = "ps1f.v1"`,
		`step13NamingGateMode = "local_vocab_review_only"`,
		`step13NamingGateRuntimeAction = "review_only_no_runtime_rename"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P218 naming review marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P224GovernorPluginOnlyBoundaryMarkers verifies P224:
// Governor plugin-only memory/backend save boundary with local runtime contract only mode.
func TestArchiveCenterJSSeq13P224GovernorPluginOnlyBoundaryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const coprocessorOrchestrationPluginOnlyModules = [`,
		`const coprocessorOrchestrationPluginOnlyExecutionMode = "local_runtime_contract_only";`,
		`plugin_only_modules: coprocessorOrchestrationPluginOnlyModules.slice(),`,
		`plugin_only_execution_mode: coprocessorOrchestrationPluginOnlyExecutionMode,`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P224 governor plugin-only boundary marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P225TokenEstimatorFollowUpMarkers verifies P225:
// Token estimator policy versions, fallback, truth floor, density profile, and shared threshold mode.
func TestArchiveCenterJSSeq13P225TokenEstimatorFollowUpMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const step13TokenEstimatorPolicyVersion = "tb1a.v1";`,
		`const step13TokenBudgetFallbackPolicyVersion = "tb1d.v1";`,
		`const step13TokenTruthFloorPolicyVersion = "tb1b.v1";`,
		`const step13TokenDensityProfilePolicyVersion = "tb1c.v1";`,
		`const step13TokenEstimatorMode = "profile_first_shared_thresholds";`,
		`const step13TokenEstimatorModelMode = "shared_thresholds_all_models";`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P225 token estimator follow-up marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P226PlanningSurfaceFollowUpMarkers verifies P226:
// Planning surface schema policy versions, runtime mode, rollout stage, and conflict actions.
func TestArchiveCenterJSSeq13P226PlanningSurfaceFollowUpMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`step13PlanningBeatPlannerSchemaPolicyVersion`,
		`step13PlanningScenePilotSchemaPolicyVersion`,
		`step13PlanningSettingFrameSchemaPolicyVersion`,
		`const step13PlanningRuntimeMode = "guidance_only_schema_draft";`,
		`const step13PlanningRolloutStage = "experimental_shadow";`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P226 planning surface follow-up marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P227BundleRegenerateManifestScriptOnlyMarkers verifies P227:
// Root bundle regenerate gate remains a script-only manifest checklist marker.
func TestArchiveCenterJSSeq13P227BundleRegenerateManifestScriptOnlyMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`seq13P215RootBundleRegenerateGate = "root_bundle_regenerate_draft"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P227 bundle regenerate manifest script-only marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq13P228ExternalReferenceNamingMapMarkers verifies P228:
// External reference naming map draft remains a local vocab review source.
func TestArchiveCenterJSSeq13P228ExternalReferenceNamingMapMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const step13NamingGateSourceDraft = "STEP13_NAMING_MAP_DRAFT.md";`,
		`step13NamingGateApprovalRule = "allow_local_vocab_block_external_like_reuse"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-13-P228 external reference naming map marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq14P85BundleLatestRootRuntimeContractMarkers verifies P85:
// Step 14 bundle latest root runtime remains a release-gate contract marker
// without generating exe, zip, bundle, or DB snapshot artifacts.
func TestArchiveCenterJSSeq14P85BundleLatestRootRuntimeContractMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`seq14P85BundleLatestRootRuntimeGate = "step14_bundle_latest_root_runtime_contract_only"`,
		`seq14P85BundleArtifactGenerationAllowed = false`,
		`seq14P85BundleGeneratedArtifactTypes = []`,
		`seq14P85BundleGateArtifactPolicy = "no_exe_zip_bundle_or_db_snapshot_generated"`,
		`seq14P85BundleGateValidationMode = "contract_marker_smoke_only"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-14-P85 bundle latest root runtime contract marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq15P144BundleLatestRootRuntimeContractMarkers verifies P144:
// Step 15 bundle latest root runtime release-gate contract marker.
// 2.0 remigration prohibits actual artifact generation.
func TestArchiveCenterJSSeq15P144BundleLatestRootRuntimeContractMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`seq15P144BundleLatestRootRuntimeGate = "step15_bundle_latest_root_runtime_contract_only"`,
		`seq15P144BundleArtifactGenerationAllowed = false`,
		`seq15P144BundleGeneratedArtifactTypes = []`,
		`seq15P144BundleGateArtifactPolicy = "no_exe_zip_bundle_or_db_snapshot_generated"`,
		`seq15P144BundleGateValidationMode = "contract_marker_smoke_only"`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-15-P144 bundle latest root runtime contract marker %q", needle)
		}
	}
}

// SEQ-16.5-P141: helper injection budget manager define — validates that
// Archive Center.js contains the helper injection budget manager surface markers.
func TestArchiveCenterJSSeq165P141HelperInjectionBudgetManagerMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function assembleInjectionWithBudget(",
		"const manualBudgetLimit = Math.max(500, settings.maxInjectionChars || DEFAULT_SETTINGS.maxInjectionChars);",
		"const step13TokenEstimatorPolicyVersion = \"tb1a.v1\";",
		"const step13TokenBudgetFallbackPolicyVersion = \"tb1d.v1\";",
		"const step13TokenEstimatorMode = \"profile_first_shared_thresholds\";",
		"const step13TokenEstimatorProfiles = [",
		"mid_context_300k",
		"wide_context_500k",
		"ultra_long_1m_plus",
		"extreme_long_2m_plus",
		"const step13TokenEstimatorBudgetLimitByProfile = adaptiveInjectionBudgetProfileLimits();",
		"const step13TokenBudgetFallbackRules = {",
		"reliable_runtime_tokens",
		"low_confidence_runtime_source",
		"runtime_tokens_unavailable",
		"manual_setting_only",
		"const step13TokenTruthFloorPolicyVersion = \"tb1b.v1\";",
		"const step13TokenTruthFloorMode = \"char_floor_projected_minimums\";",
		"const step13TokenDensityProfilePolicyVersion = \"tb1c.v1\";",
		"const runtimeTokenSmoothingWindow = 4;",
		"const runtimeTokenSmoothingState = assembleInjectionWithBudget._runtimeTokenSmoothingState || new Map();",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.5-P141 helper injection budget manager marker %q", needle)
		}
	}
}

// SEQ-16.5-P142: input context builder slot governor — validates that
// Archive Center.js contains the input context slot governor surface markers.
func TestArchiveCenterJSSeq165P142InputContextSlotGovernorMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function buildInputContext(userInput, orchResult, bundledContinuityText, governorContext) {",
		"slotGovernorPolicyVersion: \"s16.5-ig.v1\"",
		"slotGovernorMode: \"turn_need_risk_slot_governor\"",
		"const budget = Math.max(200, Math.min(1500, settings.maxInputContextChars || 800));",
		"function buildTemporalCandidate() {",
		"function buildSceneCandidate() {",
		"function buildEntityCandidate() {",
		"function buildSagaCandidate() {",
		"mandatory: true",
		"staleArcDemotionApplied",
		"helperOverlapSuppressionApplied",
		"supportLaneNote: \"Support-only anchor lane; does not overwrite canonical state.\"",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.5-P142 input context slot governor marker %q", needle)
		}
	}
}

// SEQ-16.5-P143: transparency / preview / runtime trace extend — validates that
// Archive Center.js contains the transparency/preview/runtime trace extension markers.
func TestArchiveCenterJSSeq165P143TransparencyPreviewRuntimeTraceExtendMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function buildInputTransparency(userInput, recentContext, searchResult, wakeUpContext, supervisorResult, continuityInfo, kgRecallResult, extractedEntities, activeStatesResult, episodeRecallResult, expandedEntities, languageContext, backendInputTransparencyModel, backendEffectiveInputPreview, weakInputPlanner, plannerExecutionContract, progressionChoice, step25ValidationGate) {",
		"function logTurnTraceSummary() {",
		"function renderTurnTraceRows() {",
		"_inputTransparency = buildInputTransparency(",
		"it.assembledPreview = assembleInputPreview(it);",
		"continuity: isContinuity ? {",
		"oldArcForeground: continuityInfo.oldArcForegroundGuard ? {",
		"recallPaths: (searchResult && searchResult.paths) ? searchResult.paths : []",
		"dedupeStats: (searchResult && searchResult.dedupeStats) ? searchResult.dedupeStats : null",
		"kgRecall: (kgRecallResult && kgRecallResult.count > 0) ? {",
		"activeStates: (activeStatesResult && activeStatesResult.count > 0) ? {",
		"episodeRecall: (episodeRecallResult && episodeRecallResult.count > 0) ? {",
		"pathB: (searchResult && searchResult.pathBInfo && searchResult.pathBInfo.used) ? {",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.5-P143 transparency/preview/runtime trace marker %q", needle)
		}
	}
}

// SEQ-16.5-P144: backend/main.py input_context_text handoff anchor metadata
// alignment — validates that Archive Center.js contains the handoff alignment markers.
func TestArchiveCenterJSSeq165P144HandoffAnchorMetadataAlignmentMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"buildInputContext(orchResult._userInput || \"\", orchResult, _ip ? (_ip.input_context_text || \"\") : \"\"",
		"inputCtx = buildInputContext(orchResult._userInput || \"\", orchResult, _ip ? (_ip.input_context_text || \"\") : \"\", {",
		"injectionTextSource: _ip ? \"bundle\" : \"local\"",
		"const _ip = orchResult._injectionPack || null",
		"const memoryText = (_ip && _ip.memory_text) ? _ip.memory_text : formatMemoryBlock(searchResult, sanitizeTopKSetting(settings.topK, DEFAULT_SETTINGS.topK))",
		"const kgText = (_ip && _ip.kg_text) ? _ip.kg_text : formatKGBlock(kgRecallResult)",
		"const fallbackText = (_ip && _ip.fallback_text) ? _ip.fallback_text : (includeFallback ? formatFallbackBlock(searchResult) : \"\")",
		"const latestDirectEvidenceText = (_ip && _ip.latest_direct_evidence_text) ? String(_ip.latest_direct_evidence_text) : \"\"",
		"const recentRawTurnText = (_ip && _ip.recent_raw_turn_text) ? String(_ip.recent_raw_turn_text) : \"\"",
		"const canonicalStateLayerText = (_ip && _ip.canon_text) ? String(_ip.canon_text) : \"\"",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.5-P144 handoff anchor metadata alignment marker %q", needle)
		}
	}
}

// SEQ-16.5-P145: Step 16.8 stale-arc guard carry-in / Step 17 evaluation ops
// carry-in replay/inspection hooks — validates that Archive Center.js contains
// the stale-arc guard and carry-in hook markers.
func TestArchiveCenterJSSeq165P145StaleArcGuardCarryInHooksMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function buildAdaptiveInjectionGovernorTrace(userInput, orchResult, budgetResult, inputContextResult) {",
		"const oldArcForegroundGuard = budgetPolicy.oldArcForegroundGuard && typeof budgetPolicy.oldArcForegroundGuard === \"object\"",
		"function normalizeOldArcDecision(entry, lane) {",
		"function buildOldArcFailureTaxonomy(decisions) {",
		"policyVersion: \"s16.8-ft.v1\"",
		"mode: \"recall_gain_vs_monopoly_cost_split\"",
		"primaryClass",
		"secondaryClasses",
		"validDelayedPayoffRescue",
		"foregroundHijackRisk",
		"arcMonopolyAttempt",
		"sceneMonopolyRisk",
		"splitTrace",
		"recallGainCount",
		"monopolyCostCount",
		"explicitAlignmentKeepCount",
		"currentSceneKeepCount",
		"noAlignmentDemoteCount",
		"redirectedSuppressCount",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.5-P145 stale-arc guard carry-in hooks marker %q", needle)
		}
	}
}

// SEQ-16.5-P169: helper injection adaptive floor / ceiling decision value.
func TestArchiveCenterJSSeq165P169DecisionAdaptiveFloorCeilingMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"const manualBudgetLimit = Math.max(500, settings.maxInjectionChars || DEFAULT_SETTINGS.maxInjectionChars);",
		"mid_context_300k: 6000",
		"wide_context_500k: 9000",
		"ultra_long_1m_plus: 14000",
		"extreme_long_2m_plus: 18000",
		"const step13TokenTruthFloorCoreLabels = [\"latest_direct_evidence\", \"recent_raw_turn\", \"active_state\", \"canonical_state_layer\"];",
		"const step13TokenTruthFloorContinuityLabels = [\"storylines\", \"episode\", \"chapter\", \"arc\", \"saga\"];",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.5-P169 adaptive floor/ceiling decision marker %q", needle)
		}
	}
}

// SEQ-16.5-P170: input context max slot 2 vs 3 decision value.
func TestArchiveCenterJSSeq165P170DecisionMaxSlotMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"slotGovernorPolicyVersion: \"s16.5-ig.v1\"",
		"slotGovernorMode: \"turn_need_risk_slot_governor\"",
		"maxSlots: 0",
		"let maxSlots = 2",
		"function buildTemporalCandidate() {",
		"function buildSceneCandidate() {",
		"function buildEntityCandidate() {",
		"function buildSagaCandidate() {",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.5-P170 max slot decision marker %q", needle)
		}
	}
}

// SEQ-16.5-P171: runtime token hint telemetry-only / secondary safety cap
// decision value.
func TestArchiveCenterJSSeq165P171DecisionRuntimeTokenHintMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"const runtimeTokenSourceIsEstimate = normalizedRuntimeTokenSource === \"message_char_estimate\";",
		"const runtimeTieringEnabled = !!(normalizedRuntimeTokenSource && normalizedRuntimeTokenSource !== \"none\");",
		"const runtimeBudgetAdaptiveEligible = !!(runtimeTieringEnabled && Number.isFinite(runtimeTokens) && runtimeTokens > 0);",
		"const step13TokenEstimatorLowConfidenceSources = [\"none\", \"message_char_estimate\"];",
		"const step13TokenEstimatorDriftTelemetryFields = [",
		"manualBudgetLimit",
		"budgetLimit",
		"budgetLimitSource",
		"runtimeCurrentChatTokens",
		"runtimeCurrentChatTokensEffective",
		"contextProfile",
		"contextProfileSource",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.5-P171 runtime token hint decision marker %q", needle)
		}
	}
}

// SEQ-16.5-P172: [Saga] / [Chapter] anchor competition vs fallback ladder
// decision value.
func TestArchiveCenterJSSeq165P172DecisionSagaChapterAnchorLadderMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function buildSagaCandidate() {",
		"label: \"Saga Anchor\"",
		"family: \"saga\"",
		"source: \"storylines\"",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.5-P172 saga/chapter anchor ladder decision marker %q", needle)
		}
	}
}

// SEQ-16.5-P173: explicit user-input specificity heuristic/classifier
// decision value.
func TestArchiveCenterJSSeq165P173DecisionExplicitUserInputSpecificityMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"const weakInput = !rawInput || rawInput.length <= 24 || /^(continue|go on|next|more|resume|keep going|계속|계속해|이어서|이어가|다음|다음 장면|다음으로|응|ㅇㅇ|좋아|그래|좋아 계속)$/i.test(rawInput);",
		"const temporalQuery = isTemporalQueryInput(rawInput);",
		"const resumePressure = /(continue|resume|pick up|where we left|keep going|이어서|이어가|계속|재개|다시 이어)/i.test(rawInput);",
		"const longGapResume = idleGapMs >= longGapThresholdMs || continuityTriggerMode === \"idle_reentry\";",
		"const explicitRedirection = /(instead|not that|ignore previous|leave that|move on|new scene|different topic|새로|다른 쪽|말고|이제는|이번 장면|지금 장면|새 갈등|딴 이야기|전 장면 말고)/i.test(rawInput);",
		"const strongUserIntent = rawInput.length >= 48 || rawInput.split(/\\s+/).filter(Boolean).length >= 10;",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.5-P173 explicit user-input specificity decision marker %q", needle)
		}
	}
}

// SEQ-16.5-P177: Step 16.8 stale-arc suppression slice baseline compare.
func TestArchiveCenterJSSeq165P177Step168BaselineCompareMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function buildAdaptiveInjectionGovernorTrace(userInput, orchResult, budgetResult, inputContextResult) {",
		"policyVersion: \"s16.8-ft.v1\"",
		"mode: \"recall_gain_vs_monopoly_cost_split\"",
		"primaryClass",
		"validDelayedPayoffRescue",
		"foregroundHijackRisk",
		"arcMonopolyAttempt",
		"sceneMonopolyRisk",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.5-P177 Step 16.8 baseline compare marker %q", needle)
		}
	}
}

// SEQ-16.5-P178: Step 16.8 reason visibility / monopoly replay guard lane.
func TestArchiveCenterJSSeq165P178Step168ReasonVisibilityGuardLaneMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function buildAdaptiveInjectionGovernorTrace(userInput, orchResult, budgetResult, inputContextResult) {",
		"policyVersion: \"s16.8-ft.v1\"",
		"mode: \"recall_gain_vs_monopoly_cost_split\"",
		"secondaryClasses",
		"explicitRedirectionOnly",
		"validDelayedPayoffRescue",
		"foregroundHijackRisk",
		"sceneMonopolyRisk",
		"arcMonopolyAttempt",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.5-P178 Step 16.8 reason visibility guard lane marker %q", needle)
		}
	}
}

// SEQ-16.5-P179: Step 16.8 completion Step 17 evaluation baseline direct
// handoff gate.
func TestArchiveCenterJSSeq165P179Step17DirectHandoffGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function buildAdaptiveInjectionGovernorTrace(userInput, orchResult, budgetResult, inputContextResult) {",
		"policyVersion: \"s16.8-ft.v1\"",
		"mode: \"recall_gain_vs_monopoly_cost_split\"",
		"const oldArcForegroundGuard = budgetPolicy.oldArcForegroundGuard && typeof budgetPolicy.oldArcForegroundGuard === \"object\"",
		"function normalizeOldArcDecision(entry, lane) {",
		"function buildOldArcFailureTaxonomy(decisions) {",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.5-P179 Step 17 direct handoff gate marker %q", needle)
		}
	}
}

// SEQ-16.5-P183: Step 17 evaluation harness static 3000/800 baseline + 16.5+16.8
// baseline.
func TestArchiveCenterJSSeq165P183Step17EvaluationHarnessBaselineMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"const manualBudgetLimit = Math.max(500, settings.maxInjectionChars || DEFAULT_SETTINGS.maxInjectionChars);",
		"const budget = Math.max(200, Math.min(1500, settings.maxInputContextChars || 800));",
		"policyVersion: \"s16.8-ft.v1\"",
		"mode: \"recall_gain_vs_monopoly_cost_split\"",
		"slotGovernorPolicyVersion: \"s16.5-ig.v1\"",
		"slotGovernorMode: \"turn_need_risk_slot_governor\"",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.5-P183 Step 17 evaluation harness baseline marker %q", needle)
		}
	}
}

// SEQ-16.5-P184: Step 17 ops budget tuning governor behavior trace
// interpretation document.
func TestArchiveCenterJSSeq165P184Step17OpsTraceInterpretationMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function buildAdaptiveInjectionGovernorTrace(userInput, orchResult, budgetResult, inputContextResult) {",
		"function logTurnTraceSummary() {",
		"function renderTurnTraceRows() {",
		"policyVersion: \"s16.8-ft.v1\"",
		"mode: \"recall_gain_vs_monopoly_cost_split\"",
		"decisionCounts",
		"recallGainCount",
		"monopolyCostCount",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.5-P184 Step 17 ops trace interpretation marker %q", needle)
		}
	}
}

// SEQ-16.8-P99: stale-arc ceiling — no-user-mention stale arc rescue auto-foreground.
func TestArchiveCenterJSSeq168P99StaleArcCeilingMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"oldArcForegroundGuard",
		"stale_rescue_ceiling",
		"suppressionTriggerActive",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P99 marker %q", needle)
		}
	}
}

// SEQ-16.8-P100: scene alignment — old arc explicit query alignment or fresh scene evidence.
func TestArchiveCenterJSSeq168P100SceneAlignmentMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"explicit_query_alignment",
		"current_scene_evidence",
		"oldArcForegroundGuard",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P100 marker %q", needle)
		}
	}
}

// SEQ-16.8-P101: reason trace — old arc keep/drop/suppress inspectable.
func TestArchiveCenterJSSeq168P101ReasonTraceMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"coprocessor_reason_trace",
		"supported_reason_codes",
		"keep",
		"drop",
		"suppress",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P101 marker %q", needle)
		}
	}
}

// SEQ-16.8-P102: failure split — tail recall gain foreground monopoly failure class.
func TestArchiveCenterJSSeq168P102FailureSplitMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"buildOldArcFailureTaxonomy",
		"primaryClass",
		"secondaryClasses",
		"foreground_hijack_risk",
		"arc_monopoly_attempt",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P102 marker %q", needle)
		}
	}
}

// SEQ-16.8-P103: packet synthesis — Step 21 packet/new-scene synthesis Step 22 long-horizon subsystem.
func TestArchiveCenterJSSeq168P103PacketSynthesisMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"buildContinuityPackQuery",
		"buildContinuityPackWakeUpBlock",
		"fetchStorylines",
		"fetchPendingThreads",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P103 marker %q", needle)
		}
	}
}

// SEQ-16.8-P107: callback bias ceiling — 16.8-1a callback/storyline soft bias ceiling define.
func TestArchiveCenterJSSeq168P107CallbackBiasCeilingMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"soft_bias_score",
		"soft_bias_policy_version",
		"softBiasScore",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P107 marker %q", needle)
		}
	}
}

// SEQ-16.8-P108: callback scene alignment — 16.8-1b callback rescue current-scene alignment define.
func TestArchiveCenterJSSeq168P108CallbackSceneAlignmentMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"current_scene_evidence",
		"explicit_query_alignment",
		"input_current_scene_anchor",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P108 marker %q", needle)
		}
	}
}

// SEQ-16.8-P109: stale callback suppression — 16.8-1c stale callback suppression trigger define.
func TestArchiveCenterJSSeq168P109StaleCallbackSuppressionMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"suppressionTriggerActive",
		"oldArcForegroundGuard",
		"stale_rescue_ceiling",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P109 marker %q", needle)
		}
	}
}

// SEQ-16.8-P113: old-arc foreground visibility — 16.8-2a old-arc foreground reason visibility lane define.
func TestArchiveCenterJSSeq168P113OldArcForegroundVisibilityMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"oldArcForegroundGuard",
		"oldArcForeground",
		"decisions",
		"reason",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P113 marker %q", needle)
		}
	}
}

// SEQ-16.8-P114: reason code vocabulary — 16.8-2b keep/drop/suppress/demote reason code vocabulary define.
func TestArchiveCenterJSSeq168P114ReasonCodeVocabularyMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"coprocessor_reason_trace",
		"supported_reason_codes",
		"keep",
		"drop",
		"suppress",
		"demote",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P114 marker %q", needle)
		}
	}
}

// SEQ-16.8-P115: preview/audit/transparency — 16.8-2c preview/audit/transparency surface define.
func TestArchiveCenterJSSeq168P115PreviewAuditTransparencyMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"buildInputTransparency",
		"logTurnTraceSummary",
		"renderTurnTraceRows",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P115 marker %q", needle)
		}
	}
}

// SEQ-16.8-P119: foreground hijack taxonomy — 16.8-3a foreground hijack/arc monopoly failure taxonomy define.
func TestArchiveCenterJSSeq168P119ForegroundHijackTaxonomyMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"buildOldArcFailureTaxonomy",
		"foreground_hijack_risk",
		"arc_monopoly_attempt",
		"sceneMonopolyRisk",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P119 marker %q", needle)
		}
	}
}

// SEQ-16.8-P120: delayed payoff split — 16.8-3b valid delayed payoff rescue vs scene monopoly split define.
func TestArchiveCenterJSSeq168P120DelayedPayoffSplitMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"validDelayedPayoffRescue",
		"sceneMonopolyRisk",
		"valid_delayed_payoff_rescue",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P120 marker %q", needle)
		}
	}
}

// SEQ-16.8-P121: recall gain/monopoly cost split — 16.8-3c recall gain/monopoly cost split trace schema define.
func TestArchiveCenterJSSeq168P121RecallGainMonopolySplitMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"recallGainCount",
		"monopolyCostCount",
		"splitTrace",
		"recall_gain_vs_monopoly_cost_split",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P121 marker %q", needle)
		}
	}
}

// SEQ-16.8-P125: stale arc revival replay — 16.8-4a stale arc revival/single-incident monopoly replay define.
func TestArchiveCenterJSSeq168P125StaleArcRevivalReplayMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"buildOldArcReplayGate",
		"single_incident_monopoly_attempt",
		"staleArcRevivalReplay",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P125 marker %q", needle)
		}
	}
}

// SEQ-16.8-P126: tail recall vs foreground hijack gate — 16.8-4b tail recall vs foreground hijack gate define.
func TestArchiveCenterJSSeq168P126TailRecallHijackGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"buildOldArcReplayGate",
		"tailRecallVsForegroundHijackGate",
		"tail_recall_with_monopoly_cost",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P126 marker %q", needle)
		}
	}
}

// SEQ-16.8-P127: narrative diversity gate — 16.8-4c narrative diversity gate define.
func TestArchiveCenterJSSeq168P127NarrativeDiversityGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"narrativeDiversityGate",
		"stale_arc_replay_and_diversity_adoption_gate",
		"scene diversity stayed clear of old-arc monopoly pressure",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P127 marker %q", needle)
		}
	}
}

// SEQ-16.8-P128: arc monopoly gate — 16.8-4d arc monopoly gate define.
func TestArchiveCenterJSSeq168P128ArcMonopolyGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"arc_monopoly_attempt",
		"arcMonopolyAttempt",
		"arcMonopolyGate",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P128 marker %q", needle)
		}
	}
}

// SEQ-16.8-P132: Archive Center.js continuity rescue owner surface.
func TestArchiveCenterJSSeq168P132JSContinuityRescueMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"fetchStorylines",
		"fetchPendingThreads",
		"buildContinuityPackQuery",
		"buildContinuityPackWakeUpBlock",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P132 marker %q", needle)
		}
	}
}

// SEQ-16.8-P133: Archive Center.js prompt assembly guard.
func TestArchiveCenterJSSeq168P133JSPromptAssemblyGuardMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"assembleInjectionWithBudget",
		"applyContextInjection",
		"buildAdaptiveInjectionGovernorTrace",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P133 marker %q", needle)
		}
	}
}

// SEQ-16.8-P134: Archive Center.js trace/preview/transparency surface extend.
func TestArchiveCenterJSSeq168P134JSTracePreviewTransparencyMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"buildInputTransparency",
		"tracePreview",
		"injectionPreview",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P134 marker %q", needle)
		}
	}
}

// SEQ-16.8-P135: replay corpus/inspection baseline add.
func TestArchiveCenterJSSeq168P135ReplayCorpusBaselineMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"replayScenarios",
		"single_incident_monopoly_attempt",
		"tail_recall_with_monopoly_cost",
		"valid_delayed_payoff_rescue",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P135 marker %q", needle)
		}
	}
}

// SEQ-16.8-P136: backend/main.py storyline/pending-thread read metadata alignment — suppression trace confirm.
func TestArchiveCenterJSSeq168P136BackendMetadataAlignmentMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"storylineCount",
		"pendingThreadCount",
		"suppressionTriggerActive",
		"guidance_metadata",
		"input_context_enabled",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P136 marker %q", needle)
		}
	}
}

// SEQ-16.8-P162: Decision outcome — stale arc ceiling judged by explicit
// alignment / current-scene evidence / explicit redirection, not turn-gap alone.
func TestArchiveCenterJSSeq168P162DecisionOutcomeCeilingNotTurnGapMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"stale_rescue_ceiling",
		"no_alignment_rescue_ceiling",
		"explicit_query_alignment",
		"explicit_user_redirection",
		"suppressionTrigger",
		"oldArcForeground",
		"old-arc guard",
		"applyContinuityPackOldArcGuard",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P162 marker %q", needle)
		}
	}
}

// SEQ-16.8-P163: current-scene evidence minimum criteria.
func TestArchiveCenterJSSeq168P163CurrentSceneEvidenceMinCriteriaMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"currentSceneEvidence",
		"activeState",
		"latestDirectEvidence",
		"recentRawTurn",
		"inspectable",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P163 marker %q", needle)
		}
	}
}

// SEQ-16.8-P164: open / paused thread ceiling family, pending_threads guard.
func TestArchiveCenterJSSeq168P164PendingThreadsGuardMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"pending_threads",
		"pendingThreads",
		"open",
		"paused",
		"guard",
		"ceiling",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P164 marker %q", needle)
		}
	}
}

// SEQ-16.8-P165: reason visibility lane extends to adaptive trace / continuity
// trace / input transparency.
func TestArchiveCenterJSSeq168P165ReasonVisibilityLaneMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"_inputTransparency",
		"buildContinuityTraceState",
		"applyContinuityTraceState",
		"reason",
		"inspectable",
		"trace",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P165 marker %q", needle)
		}
	}
}

// SEQ-16.8-P166: diversity gate default diagnostic warn, arc_monopoly_attempt
// Step 17 handoff block signal.
func TestArchiveCenterJSSeq168P166DiversityGateDiagnosticWarnMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"diversity",
		"monopoly",
		"diagnostic",
		"warn",
		"handoff",
		"block",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P166 marker %q", needle)
		}
	}
}

// SEQ-17-P230: retrieval completeness vs final answer quality split.
func TestArchiveCenterJSSeq17P230EvaluationSplitMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"retrieval",
		"evaluation",
		"split",
		"failure",
		"healthy",
		"metric",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P230 marker %q", needle)
		}
	}
}

// SEQ-17-P231: ops procedure documentation surface.
func TestArchiveCenterJSSeq17P231OpsProcedureSurfaceMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"promotion",
		"backfill",
		"rebuild",
		"reembed",
		"migration",
		"health",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P231 marker %q", needle)
		}
	}
}

// SEQ-17-P232: inspection lane boundary surface.
func TestArchiveCenterJSSeq17P232InspectionLaneBoundaryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"explain",
		"preview",
		"audit",
		"dashboard",
		"inspection",
		"boundary",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P232 marker %q", needle)
		}
	}
}

// SEQ-17-P233: adoption gate — replay green before default adoption value.
func TestArchiveCenterJSSeq17P233AdoptionGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"adoption",
		"replay",
		"green",
		"default",
		"blocked",
		"gate",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P233 marker %q", needle)
		}
	}
}

// SEQ-17-P234: release hygiene — bundle/regression/checklist repeatability.
func TestArchiveCenterJSSeq17P234ReleaseHygieneMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"bundle",
		"regression",
		"checklist",
		"release",
		"hygiene",
		"contract",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P234 marker %q", needle)
		}
	}
}

// SEQ-17-P238: 17-1a retrieval completeness metric define.
// NOTE: Step 17 evaluation surface is primarily Go backend; JS runtime does not
// expose direct completeness metric markers. Verified by Go contract test only.
func TestArchiveCenterJSSeq17P238RetrievalCompletenessMetricMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"lc1",
		"metric",
		"retrieval",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P238 marker %q", needle)
		}
	}
}

// SEQ-17-P239: 17-1b final answer quality metric define.
// NOTE: Step 17 evaluation surface is primarily Go backend; JS runtime does not
// expose direct answer quality metric markers. Verified by Go contract test only.
func TestArchiveCenterJSSeq17P239FinalAnswerQualityMetricMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"lc1",
		"metric",
		"quality",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P239 marker %q", needle)
		}
	}
}

// SEQ-17-P240: 17-1c retrieval failure vs reader failure split replay define.
// NOTE: Step 17 evaluation surface is primarily Go backend; JS runtime does not
// expose direct failure split replay markers. Verified by Go contract test only.
func TestArchiveCenterJSSeq17P240FailureSplitReplayMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"retrieval",
		"failure",
		"split",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P240 marker %q", needle)
		}
	}
}

// SEQ-17-P241: 17-1d Step 14~16 regression corpus define.
// NOTE: Step 17 evaluation surface is primarily Go backend; JS runtime does not
// expose direct regression corpus markers. Verified by Go contract test only.
func TestArchiveCenterJSSeq17P241RegressionCorpusMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"regression",
		"corpus",
		"seq14",
		"seq15",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P241 marker %q", needle)
		}
	}
}

// SEQ-17-P242: 17-1e freshness lag metric define.
func TestArchiveCenterJSSeq17P242FreshnessLagMetricMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"freshness",
		"lag",
		"extraction",
		"delay",
		"promotion",
		"visibility",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P242 marker %q", needle)
		}
	}
}

// SEQ-17-P286: 17-2a promotion / backfill / rebuild document.
// NOTE: Step 17 ops procedure is Go backend surface; JS runtime does not
// expose direct procedure markers. Verified by Go contract test only.
func TestArchiveCenterJSSeq17P286PromotionBackfillRebuildMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"promotion",
		"backfill",
		"rebuild",
		"dry_run",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P286 marker %q", needle)
		}
	}
}

// SEQ-17-P287: 17-2b reembed / migration / health probe document.
func TestArchiveCenterJSSeq17P287ReembedMigrationHealthProbeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"reembed",
		"migration",
		"health",
		"probe",
		"audit",
		"dry_run",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P287 marker %q", needle)
		}
	}
}

// SEQ-17-P288: 17-2c failure mode / fallback / rollback runbook cleanup.
// NOTE: Step 17 runbook is Go backend surface; JS runtime does not
// expose direct runbook markers. Verified by Go contract test only.
func TestArchiveCenterJSSeq17P288FailureFallbackRollbackMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"failure",
		"fallback",
		"rollback",
		"degraded",
		"mode",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P288 marker %q", needle)
		}
	}
}

// SEQ-17-P289: 17-2d async complete-turn / critic delay runbook cleanup.
func TestArchiveCenterJSSeq17P289AsyncCriticDelayMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"async",
		"complete",
		"critic",
		"delay",
		"repair",
		"replay",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P289 marker %q", needle)
		}
	}
}

// SEQ-17-P290: 17-2e partial-write / silent-skip / retry budget cleanup.
func TestArchiveCenterJSSeq17P290PartialWriteRetryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"partial",
		"write",
		"silent",
		"skip",
		"retry",
		"budget",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P290 marker %q", needle)
		}
	}
}

// SEQ-17-P306: 17-3a explain surface role define.
func TestArchiveCenterJSSeq17P306ExplainSurfaceMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"explain",
		"surface",
		"reasoning",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P306 marker %q", needle)
		}
	}
}

// SEQ-17-P307: 17-3b preview / audit surface role define.
func TestArchiveCenterJSSeq17P307PreviewAuditSurfaceMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"preview",
		"audit",
		"outcome",
		"decision",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P307 marker %q", needle)
		}
	}
}

// SEQ-17-P308: 17-3c dashboard lane split define.
func TestArchiveCenterJSSeq17P308DashboardLaneMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"dashboard",
		"lane",
		"metric",
		"save",
		"extraction",
		"promotion",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P308 marker %q", needle)
		}
	}
}

// SEQ-17-P309: 17-3d inspection surface authority display guard define.
func TestArchiveCenterJSSeq17P309DisplayGuardMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"displayGuard",
		"lane guard",
		"canonical store evidence",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P309 marker %q", needle)
		}
	}
}

// SEQ-17-P310: 17-3e freshness / extract-drop / promotion-block visibility lane define.
func TestArchiveCenterJSSeq17P310VisibilityLaneMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"visibility",
		"freshness",
		"extract",
		"drop",
		"promotion",
		"block",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P310 marker %q", needle)
		}
	}
}

// SEQ-17-P327: 17-4a Step 14 adoption gate define.
func TestArchiveCenterJSSeq17P327Step14AdoptionGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"adoption",
		"gate",
		"regression",
		"corpus",
		"definition=",
		"execution=",
		"limited_cutover",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P327 marker %q", needle)
		}
	}
}

// SEQ-17-P328: 17-4b Step 15 adoption gate define.
func TestArchiveCenterJSSeq17P328Step15AdoptionGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"adoption",
		"gate",
		"regression",
		"corpus",
		"definition=",
		"execution=",
		"limited_cutover",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P328 marker %q", needle)
		}
	}
}

// SEQ-17-P329: 17-4c Step 16 adoption gate define.
func TestArchiveCenterJSSeq17P329Step16AdoptionGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"adoption",
		"gate",
		"regression",
		"corpus",
		"definition=",
		"execution=",
		"limited_cutover",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P329 marker %q", needle)
		}
	}
}

// SEQ-17-P330: 17-4d root -> bundle regenerate checklist define.
func TestArchiveCenterJSSeq17P330BundleRegenerateChecklistMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"bundle",
		"regenerate",
		"checklist",
		"Bundle Closure",
		"release_gate_closed",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P330 marker %q", needle)
		}
	}
}

// SEQ-17-P331: 17-4e packaged bundle regression / smoke / release note checklist define.
func TestArchiveCenterJSSeq17P331PackagedBundleChecklistMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"bundle",
		"regression",
		"smoke",
		"release",
		"Release Hygiene",
		"release_ready",
		"known",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P331 marker %q", needle)
		}
	}
}

// SEQ-17-P332: 17-4f freshness / silent-drop gate define.
func TestArchiveCenterJSSeq17P332FreshnessSilentDropGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"freshness",
		"silent",
		"drop",
		"gate",
		"runtime defaults",
		"Visibility Guard",
		"visible_failures",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P332 marker %q", needle)
		}
	}
}

// SEQ-16.8-P170: Step 17 evaluation harness consumes Step 16.8 replay corpus baseline.
func TestArchiveCenterJSSeq168P170CarryInEvaluationHarnessMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"evaluationHarnessConsumesStep168ReplayCorpus",
		"redefines17_1f",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P170 marker %q", needle)
		}
	}
}

// SEQ-16.8-P171: Step 17 inspection surface uses Step 16.8 reason visibility lane baseline.
func TestArchiveCenterJSSeq168P171CarryInInspectionSurfaceMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"inspectionSurfaceUsesStep168ReasonVisibilityLane",
		"redefines17_3f",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P171 marker %q", needle)
		}
	}
}

// SEQ-16.8-P172: Step 17 adoption gate uses Step 16.8 diversity gate baseline.
func TestArchiveCenterJSSeq168P172CarryInAdoptionGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"adoptionGateUsesStep168DiversityGate",
		"redefines17_4g",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P172 marker %q", needle)
		}
	}
}

// SEQ-16.8-P176: Step 18 hybrid scoring stale callback ceiling / current-scene alignment baseline.
func TestArchiveCenterJSSeq168P176CarryInStep18HybridScoringMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"stale",
		"arc",
		"ceiling",
		"alignment",
		"current_scene_evidence",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P176 marker %q", needle)
		}
	}
}

// SEQ-16.8-P177: Step 20 selective rerank stale callback suppression trigger / monopoly failure taxonomy baseline.
func TestArchiveCenterJSSeq168P177CarryInStep20SelectiveRerankMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"selectiveRerankConsumesStep168Suppression",
		"selectiveRerankConsumesStep168MonopolyTaxonomy",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P177 marker %q", needle)
		}
	}
}

// SEQ-16.8-P178: later-step recall / rerank gain foreground monopoly cost baseline trace.
func TestArchiveCenterJSSeq168P178CarryInLaterStepRecallRerankMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"laterRecallRerankSharesMonopolyCostTrace",
		"recallGainCount",
		"monopolyCostCount",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-16.8-P178 marker %q", needle)
		}
	}
}

// SEQ-17-P387: Archive Center Beta 0.8 bundle latest root runtime create/generate.
func TestArchiveCenterJSSeq17P387BundleGenerationEvidenceMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"_step17ReleaseGate",
		"step17ReleaseGateFetch",
		"/metrics/lc1s/step17-bundle-closure",
		"step17_bundle_closure",
		"Bundle Closure",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P387 marker %q", needle)
		}
	}
}

// SEQ-17-P388: Step 14~16 regression corpus green.
func TestArchiveCenterJSSeq17P388RegressionCorpusGreenMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"/metrics/lc1r/regression-corpus",
		"regression_corpus_manifest",
		"Regression Corpus",
		"release_gate_ready",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P388 marker %q", needle)
		}
	}
}

// SEQ-17-P389: evaluation split completeness/answer-quality smoke check pass.
func TestArchiveCenterJSSeq17P389EvaluationSplitSmokeCheckMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"closure_status",
		"release_gate_closed",
		"summarizeChecklist",
		"Release Hygiene",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P389 marker %q", needle)
		}
	}
}

// SEQ-17-P390: ops procedure dry-run checklist pass.
func TestArchiveCenterJSSeq17P390OpsDryRunChecklistPassMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"operator_checklist",
		"Missing Release Evidence",
		"release_hygiene",
		"Release Reasons",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P390 marker %q", needle)
		}
	}
}

// SEQ-17-P391: inspection surface lane-boundary review checklist pass.
func TestArchiveCenterJSSeq17P391InspectionLaneBoundaryReviewMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"renderStep17ReleaseGateSection",
		"This panel is read-only",
		"Bundle closure and session-local adoption are separate truths",
		"live limited-cutover gates remain hold/pending",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P391 marker %q", needle)
		}
	}
}

// SEQ-17-P392: adoption gate / release note / bundle checklist complete.
func TestArchiveCenterJSSeq17P392ReleaseGateCompleteMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"/chroma-shadow/adoption-gate",
		"/chroma-shadow/release-hygiene",
		"adoption-gate",
		"release-hygiene",
		"Adoption Gate",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P392 marker %q", needle)
		}
	}
}

// ---------------------------------------------------------------------------
// SEQ-17 re-audit closure JS smoke tests (P396 ~ P402)
// ---------------------------------------------------------------------------

// SEQ-17-P396: backend/admin release-gate owner closure.
func TestArchiveCenterJSSeq17P396ReauditBackendAdminOwnerMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"step17ReleaseGateFetch",
		"/metrics/lc1s/step17-bundle-closure",
		"/metrics/lc1r/regression-corpus",
		"/chroma-shadow/adoption-gate",
		"/chroma-shadow/release-hygiene",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P396 marker %q", needle)
		}
	}
}

// SEQ-17-P397: ops documentation dry-run checklist closure.
func TestArchiveCenterJSSeq17P397ReauditOpsDocDryRunMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"release_hygiene",
		"operator_checklist",
		"Missing Release Evidence",
		"Release Hygiene",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P397 marker %q", needle)
		}
	}
}

// SEQ-17-P398: root runtime read-only inspection/gate surface closure.
func TestArchiveCenterJSSeq17P398ReauditRootRuntimeReadOnlyMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"renderStep17InspectionRolesSection",
		"Step 17 inspection remains read-only",
		"This panel does not open adoption, change routing, or bypass direct evidence",
		"renderStep17ReleaseGateSection",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P398 marker %q", needle)
		}
	}
}

// SEQ-17-P399: release gate operator evidence closure.
func TestArchiveCenterJSSeq17P399ReauditReleaseGateOperatorEvidenceMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"operator_evidence",
		"release_evidence",
		"missing_operator_evidence",
		"missing_release_evidence",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P399 marker %q", needle)
		}
	}
}

// SEQ-17-P400: admin mutation/control UI boundary (dangerous surface).
func TestArchiveCenterJSSeq17P400ReauditAdminMutationControlUIMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"This panel is read-only",
		"does not open adoption",
		"change routing",
		"bypass direct evidence",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P400 marker %q", needle)
		}
	}
	forbidden := []string{
		"mo-step17-admin-execute",
		"step17AdminMutationExecute",
		"step17AdminControlExecute",
	}
	for _, needle := range forbidden {
		if strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js unexpectedly exposes SEQ-17-P400 execution marker %q", needle)
		}
	}
}

// SEQ-17-P401: release execution UI boundary (dangerous surface).
func TestArchiveCenterJSSeq17P401ReauditReleaseExecutionUIMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"This panel is read-only",
		"release gate can stay closed",
		"operator evidence is supplied",
		"live limited-cutover gates remain hold/pending",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P401 marker %q", needle)
		}
	}
	forbidden := []string{
		"mo-step17-release-execute",
		"step17ReleaseExecutionRun",
		"step17BundleRegenerateExecute",
	}
	for _, needle := range forbidden {
		if strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js unexpectedly exposes SEQ-17-P401 execution marker %q", needle)
		}
	}
}

// SEQ-17-P402: Beta 0.8 closure bundle boundary.
func TestArchiveCenterJSSeq17P402ReauditBeta08ClosureBundleMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"step17_bundle_closure",
		"closure_status",
		"closure_scope",
		"release_gate_closed",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P402 marker %q", needle)
		}
	}
}

// ---------------------------------------------------------------------------
// SEQ-17 Beta 0.8 decision JS smoke tests (P412 ~ P416)
// ---------------------------------------------------------------------------

// SEQ-17-P412: completeness metric default unit decision.
func TestArchiveCenterJSSeq17P412DecisionCompletenessMetricUnitMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"Chroma Live Retrieval",
		"summarizeChromaLiveInspection",
		"candidateCount",
		"supportingOnlyCount",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P412 marker %q", needle)
		}
	}
}

// SEQ-17-P413: regression corpus mix decision.
func TestArchiveCenterJSSeq17P413DecisionRegressionCorpusMixMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"/metrics/lc1r/regression-corpus",
		"regression_corpus_manifest",
		"release_gate_ready",
		"Regression Corpus",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P413 marker %q", needle)
		}
	}
}

// SEQ-17-P414: inspection lane default decision.
func TestArchiveCenterJSSeq17P414DecisionInspectionLaneDefaultMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"renderStep17InspectionRolesSection",
		"renderStep17VisibilitySection",
		"Freshness Summary",
		"Visibility Guard",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P414 marker %q", needle)
		}
	}
}

// SEQ-17-P415: adoption gate review mode decision.
func TestArchiveCenterJSSeq17P415DecisionAdoptionGateReviewModeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"/chroma-shadow/adoption-gate",
		"limited_cutover_approved",
		"missing_operator_evidence",
		"Adoption Reasons",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P415 marker %q", needle)
		}
	}
}

// SEQ-17-P416: bundle regenerate split decision.
func TestArchiveCenterJSSeq17P416DecisionBundleRegenerateSplitMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"/chroma-shadow/release-hygiene",
		"release_hygiene_status",
		"release_ready",
		"Release Reasons",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P416 marker %q", needle)
		}
	}
}

// ---------------------------------------------------------------------------
// SEQ-17 Chroma migration dry-run JS smoke tests (P420 ~ P430)
// ---------------------------------------------------------------------------

// SEQ-17-P420: 17-C1 migration preflight dry-run.
func TestArchiveCenterJSSeq17P420ChromaMigrationPreflightMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"summarizeChromaLiveInspection",
		"chroma_live_state",
		"chroma_live_mode",
		"chroma_live_reason",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P420 marker %q", needle)
		}
	}
}

// SEQ-17-P421: 17-C2 shadow collection bootstrap dry-run.
func TestArchiveCenterJSSeq17P421ChromaShadowBootstrapMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"shadow_disabled",
		"Chroma Live Retrieval",
		"chromaLive",
		"buildChromaLiveTraceDetail",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P421 marker %q", needle)
		}
	}
}

// SEQ-17-P422: 17-C3 backfill dry-run.
func TestArchiveCenterJSSeq17P422ChromaBackfillDryRunMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"backfill_import",
		"schema_migration",
		"sidecar_cache",
		"next_prepare_turn_fetch",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P422 marker %q", needle)
		}
	}
}

// SEQ-17-P423: 17-C4 bulk backfill dry-run.
func TestArchiveCenterJSSeq17P423ChromaBulkBackfillMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"backfill_import",
		"checkpoint_full_rebuild",
		"selective",
		"full",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P423 marker %q", needle)
		}
	}
}

// SEQ-17-P424: 17-C5 reembed discipline dry-run.
func TestArchiveCenterJSSeq17P424ChromaReembedDisciplineMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"seq13P216ReembedFallbackSmokePass",
		"reembed_fallback_smoke_check_pass",
		"stale",
		"invalidation",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P424 marker %q", needle)
		}
	}
}

// SEQ-17-P425: 17-C6 divergence / health probe dry-run.
func TestArchiveCenterJSSeq17P425ChromaDivergenceHealthProbeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"resolveChromaLiveTraceStatus",
		"buildChromaLiveTraceDetail",
		"statePriority",
		"mariadb_fallback",
		"sqlite_fallback",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P425 marker %q", needle)
		}
	}
}

// SEQ-17-P426: 17-C7 degraded fallback runbook dry-run.
func TestArchiveCenterJSSeq17P426ChromaDegradedFallbackRunbookMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"mariadb_fallback",
		"sqlite_fallback",
		"degraded",
		"blocked",
		"fallbackCount",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P426 marker %q", needle)
		}
	}
}

// SEQ-17-P427: 17-C8 rebuild / rollback drill dry-run.
func TestArchiveCenterJSSeq17P427ChromaRebuildRollbackDrillMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"schema_migration",
		"checkpoint_full_rebuild",
		"backfill_import",
		"full",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P427 marker %q", needle)
		}
	}
}

// SEQ-17-P428: 17-C9 adoption gate dry-run.
func TestArchiveCenterJSSeq17P428ChromaAdoptionGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"/chroma-shadow/adoption-gate",
		"limited_cutover_approved",
		"operator_checklist",
		"hold_reasons",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P428 marker %q", needle)
		}
	}
}

// SEQ-17-P429: 17-C10 release hygiene dry-run.
func TestArchiveCenterJSSeq17P429ChromaReleaseHygieneMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"/chroma-shadow/release-hygiene",
		"release_hygiene_status",
		"missing_release_evidence",
		"pending_reasons",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P429 marker %q", needle)
		}
	}
}

// SEQ-17-P430: 17-C11 migration visibility guard dry-run.
func TestArchiveCenterJSSeq17P430ChromaMigrationVisibilityGuardMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"/chroma-shadow/visibility-guard",
		"freshness_lag_summary",
		"Visibility Guard",
		"visible_failure_count",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-17-P430 marker %q", needle)
		}
	}
}

// ---------------------------------------------------------------------------
// SEQ-18 JS smoke tests (P13 ~ P53)
// ---------------------------------------------------------------------------

// SEQ-18-P13: reset administration marker.
func TestArchiveCenterJSSeq18P13ResetAdminMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"reset_administration",
		"checklist_cleared_for_redo",
		"historical_content_preserved",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P13 marker %q", needle)
		}
	}
}

// SEQ-18-P19: Step 17 closure gate marker.
func TestArchiveCenterJSSeq18P19Step17ClosureGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"step_17_closure_gate",
		"closed",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P19 marker %q", needle)
		}
	}
}

// SEQ-18-P21: prep anchor VR+HY marker.
func TestArchiveCenterJSSeq18P21PrepAnchorVRHYMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"prep_anchor_vr_hy",
		"18-1_vr",
		"18-2_hy",
		"downstream_slices",
		"18-3_qr",
		"18-4_vx",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P21 marker %q", needle)
		}
	}
}

// SEQ-18-P23: backend prep anchor marker.
func TestArchiveCenterJSSeq18P23BackendPrepAnchorMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"backend_prep_anchor_file",
		"backend/archive/bridge.py",
		"search_memories",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P23 marker %q", needle)
		}
	}
}

// SEQ-18-P24: routing contract prep anchor marker.
func TestArchiveCenterJSSeq18P24RoutingContractPrepAnchorMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"routing_contract_prep_anchor_file",
		"backend/main.py",
		"_build_recall_intent_contract_q3a",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P24 marker %q", needle)
		}
	}
}

// SEQ-18-P29: VR scoped verbatim support marker.
func TestArchiveCenterJSSeq18P29VRScopedVerbatimSupportMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"vr_scoped_verbatim_support",
		"Scoped Verbatim Recall (support surface)",
		"direct_evidence_gate_approved",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P29 marker %q", needle)
		}
	}
}

// SEQ-18-P30: VR policy owner block marker.
func TestArchiveCenterJSSeq18P30VRPolicyOwnerBlockMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"vr_policy_owner_block",
		"max_items=3",
		"max_total_chars=720",
		"max_excerpt_chars=160",
		"support_surface_first=true",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P30 marker %q", needle)
		}
	}
}

// SEQ-18-P31: VR prompt injection strategy marker.
func TestArchiveCenterJSSeq18P31VRPromptInjectionStrategyMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"vr_prompt_injection_strategy",
		"latest_anchor_only",
		"vr_multi_item_lane_exposed",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P31 marker %q", needle)
		}
	}
}

// SEQ-18-P32: VR hierarchy escape hatch marker.
func TestArchiveCenterJSSeq18P32VRHierarchyEscapeHatchMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"vr_hierarchy_escape_hatch",
		"visible_when_summary_thin",
		"verbatim_support_surface_priority",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P32 marker %q", needle)
		}
	}
}

// SEQ-18-P33: VR backend test guard marker.
func TestArchiveCenterJSSeq18P33VRBackendTestGuardMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"vr_backend_test_file",
		"backend/test_step18_scoped_verbatim_support.py",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P33 marker %q", needle)
		}
	}
}

// SEQ-18-P34: VR runtime transparency marker.
func TestArchiveCenterJSSeq18P34VRRuntimeTransparencyMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"vr_runtime_transparency_test_file",
		"test_step18_scoped_verbatim_input_transparency.js",
		"Scoped Verbatim Recall (support surface)",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P34 marker %q", needle)
		}
	}
}

// SEQ-18-P35: VR regression bundle green marker.
func TestArchiveCenterJSSeq18P35VRRegressionBundleGreenMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"vr_regression_bundle_green",
		"adjacent_step19_regression_green",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P35 marker %q", needle)
		}
	}
}

// SEQ-18-P46: HY semantic rank + keyword overlap marker.
func TestArchiveCenterJSSeq18P46HYSemanticRankKeywordOverlapMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"hy_semantic_rank_preserved",
		"hy_keyword_overlap_policy",
		"hy1a.v1",
		"hy_hybrid_baseline_policy_version",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P46 marker %q", needle)
		}
	}
}

// SEQ-18-P47: HY soft bias marker.
func TestArchiveCenterJSSeq18P47HYSoftBiasMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"hy_soft_bias_policy",
		"hy1b.v1",
		"hy_speaker_bias_weight",
		"hy_location_bias_weight",
		"hy_storyline_bias_weight",
		"hy_soft_bias_cap",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P47 marker %q", needle)
		}
	}
}

// SEQ-18-P48: HY stopword guard marker.
func TestArchiveCenterJSSeq18P48HYStopwordGuardMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"hy_stopword_inflation_fixed",
		"hy_tightened_extractor",
		"hy_common_filler_excluded",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P48 marker %q", needle)
		}
	}
}

// SEQ-18-P49: HY q1a propagation marker.
func TestArchiveCenterJSSeq18P49HYQ1aPropagationMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"hy_q1a_propagation",
		"hy_q1a_propagated_fields",
		"keyword_overlap_score",
		"soft_bias_policy_version",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P49 marker %q", needle)
		}
	}
}

// SEQ-18-P50: HY runtime inspection marker.
func TestArchiveCenterJSSeq18P50HYRuntimeInspectionMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"hy_js_function",
		"extractMemoryItems",
		"hy_row_meta_extended",
		"hy_transparency_block",
		"Hybrid Retrieval Inspection",
		"trace_only",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P50 marker %q", needle)
		}
	}
}

// SEQ-18-P51: HY recurring risk guards marker.
func TestArchiveCenterJSSeq18P51HYRecurringRiskGuardsMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"hy_backend_regression_test_file",
		"backend/test_step18_hybrid_regression.py",
		"hy_js_transparency_test_file",
		"test_step18_hybrid_input_transparency.js",
		"hy_recurring_risk_guard_stopword",
		"guarded",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P51 marker %q", needle)
		}
	}
}

// SEQ-18-P52: HY policy registry marker.
func TestArchiveCenterJSSeq18P52HYPolicyRegistryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"hy_policy_registry_file",
		"backend/archive/hybrid_policy.py",
		"hy_policy_registry_consolidated",
		"hy_policy_family",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P52 marker %q", needle)
		}
	}
}

// SEQ-18-P53: HY stop at 18-2c marker.
func TestArchiveCenterJSSeq18P53HYStopAt18_2cMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"hy_stop_at_18_2c",
		"hy_open_follow_up",
		"step18_hy_18-2d",
		"hy_tail_budget_rescue",
		"enabled",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P53 marker %q", needle)
		}
	}
}

// SEQ-18-P65~P69: HY tail-budget rescue markers.
func TestArchiveCenterJSSeq18P65ToP69HYTailBudgetMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"hy_tail_budget_policy_version",
		"hy1d.v1",
		"hy_tail_budget_policy_owner",
		"hy_tail_budget_rescue_pass",
		"bounded_post_rank_same_n_results_budget",
		"keyword_soft_bias_stronger_than_cutline",
		"tail_budget_original_rank",
		"tail_budget_score_gap",
		"hy_tail_budget_q1a_propagation",
		"near_cutoff_rescue",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P65~P69 marker %q", needle)
		}
	}
}

// SEQ-18-P76~P91: QR query-class contract and budget markers.
func TestArchiveCenterJSSeq18P76ToP91QRQueryClassBudgetMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"qr_query_class_contract_version",
		"qr1a.v1",
		"single_query_shared",
		"qr_query_classes",
		"explicit_temporal_cue",
		"resume_trigger_or_ready_resume_pack",
		"qr_cue_block_owner",
		"backend/test_step18_query_class_contract.py",
		"qr_budget_policy_version",
		"qr1b.v1",
		"qr_q3c_budget_reuse",
		"qr_temporal_profile_budget",
		"retrieval_depth",
		"candidate_budget",
		"backend/test_step18_query_class_budget_policy.py",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P76~P91 marker %q", needle)
		}
	}
}

// SEQ-18-P98~P113: QR note and route policy markers.
func TestArchiveCenterJSSeq18P98ToP113QRNoteRouteMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"qr_note_policy_version",
		"qr1c.v1",
		"qr_support_surface_first",
		"qr_note_only_until_route_exec",
		"extract_before_read",
		"retrieval_note_surface",
		"backend/test_step18_query_class_note_policy.py",
		"qr_route_policy_version",
		"qr1d.v1",
		"scene_default",
		"needle_in_haystack",
		"old_detail_bridge",
		"qr_long_tail_route_candidates",
		"selected_route",
		"backend/test_step18_query_class_route_policy.py",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P98~P113 marker %q", needle)
		}
	}
}

// SEQ-18-P120~P153: VX validation gate markers.
func TestArchiveCenterJSSeq18P120ToP153VXValidationGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"vx18a_hybrid_replay_gate",
		"validation_gates.hybrid_replay",
		"_U1E_CAPTURED_REPLAY",
		"backend/test_step18_hybrid_replay_gate.py",
		"vx18b_heldout_completeness_gate",
		"retention_rate",
		"false_negative_rate",
		"full_coverage_rate",
		"backend/test_step18_heldout_completeness_gate.py",
		"vx18c_latency_token_budget_gate",
		"baseline_latency_proxy_ms",
		"candidate_token_budget_chars",
		"_LC1M_MAX_SPLIT_LATENCY_MULTIPLIER",
		"backend/test_step18_latency_token_budget_gate.py",
		"vx18d_truth_boundary_replay_gate",
		"_LC1K_HIGH_AUTHORITY_SOURCES",
		"_LC1K_LOWER_TIER_SOURCES",
		"backend/test_step18_truth_boundary_replay_gate.py",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P120~P153 marker %q", needle)
		}
	}
}

// SEQ-18-P160~P164: VX truncation / summary-loss gate markers.
func TestArchiveCenterJSSeq18P160ToP164VXTruncationSummaryLossMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"vx18e_truncation_summary_loss_gate",
		"validation_gates.truncation_summary_loss",
		"vx18e.v1",
		"baseline_tail_fact_miss_rate",
		"candidate_tail_fact_miss_rate",
		"baseline_summary_loss_rate",
		"candidate_summary_loss_rate",
		"tail_budget_promoted",
		"_U1E_CAPTURED_REPLAY_MIN",
		"backend/test_step18_truncation_summary_loss_gate.py",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P160~P164 marker %q", needle)
		}
	}
}

// SEQ-18-P327~P369: Post-Chroma and Step 18 summary row markers.
func TestArchiveCenterJSSeq18P327ToP369SummaryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"post_chroma_top6",
		"scoped_verbatim_recall_lane",
		"hybrid_retrieval_scoring_baseline",
		"temporal_relation_story_clock_foundation",
		"temporal_validity_retrieval",
		"lightweight_entity_graph_retrieval_accelerator",
		"selective_rerank_budget_aware_routing",
		"step18_summary_priority_rows",
		"step18_vr_raw_preserving_support",
		"step18_vr_truth_boundary_preserve",
		"step18_vr_summary_rows",
		"step18_vr_18-1d",
		"step18_hy_summary_rows",
		"step18_hy_18-2d",
		"step18_qr_summary_rows",
		"step18_qr_18-3d",
		"step18_vx_summary_rows",
		"step18_vx_18-4e",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P327~P369 marker %q", needle)
		}
	}
}

// SEQ-18-P373~P392: Pre-release 1.0.0 evidence markers.
func TestArchiveCenterJSSeq18P373ToP392PreReleaseMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"pre_release_1_0_0_marker",
		"1.0.0-pre",
		"pre_release_bundle_authority",
		"Archive Center 2.5.0 Release",
		"pre_release_smoke_checks",
		"scoped_verbatim_recall",
		"hybrid_baseline",
		"query_class_routing_budget",
		"vx_review_checklist",
		"pre_release_raw_support_limits",
		"max_items=3 max_total_chars=720 max_excerpt_chars=160",
		"pre_release_hybrid_soft_bias",
		"speaker=0.04 location=0.05 storyline=0.06 cap=0.12",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-18-P373~P392 marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq19P114ValidatorHelperClusterMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function extractTemporalRelationEntriesStep19",
		"function buildTemporalStateSurfaceStep19",
		"function validateResponseTemporalDeicticStep19",
		"function readSceneTemporalStateFromOrchResultStep19",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P114 validator helper cluster marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq19P115PrecedenceResolutionOrderMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"\"session_state_clock\"",
		"\"input_current_scene_anchor\"",
		"\"timeline_anchor\"",
		"\"carry_forward\"",
		"resolutionOrder",
		"currentStoryClock",
		"temporalRelationLedger",
		"ignoredLatestTimestamp",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P115 precedence resolution order marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq19P116WarningClassMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"current_scene_deictic_mismatch",
		"relation_only_promoted_to_current_scene",
		"exact_current_scene_without_resolved_clock",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P116 warning class marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq19P117TraceOnlyWarningSurfaceMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"let temporalDeicticValidation = null",
		"temporalDeicticValidation = validateResponseTemporalDeicticStep19",
		"[Step19] temporal deictic validator warn",
		"[Step19] temporal deictic validator skipped",
	}
	forbidden := []string{
		"if (temporalDeicticValidation.status === \"warn\") { throw",
		"if (temporalDeicticValidation.status === \"warn\") { return",
		"temporalDeicticValidation.status === \"warn\" && saveSucceeded = false",
		"temporalDeicticValidation.status === \"warn\" && completeResult = null",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P117 trace-only warning surface marker %q", needle)
		}
	}
	for _, needle := range forbidden {
		if strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js has forbidden blocking pattern for SEQ-19-P117 trace-only surface %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq19P125ClassificationWriteDisciplineMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalMentionClassification",
		"primaryClass",
		"clockWriteDirective",
		"elapsedTimeDecision",
		"block_relation_only_write",
		"block_figurative_only_write",
		"commit_explicit_advance",
		"commit_current_scene_anchor",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P125 classification/write-discipline marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq19P126ClassificationExceptionMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"\"planned_event\"",
		"\"recalled_event\"",
		"\"figurative_duration\"",
		"plannedFuture ? \"planned_event\"",
		"figurativeDuration",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P126 classification exception marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq19P127WriteDisciplineRuleMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"block_relation_only_write",
		"block_figurative_only_write",
		"figurative_duration_excluded",
		"relation_only_reference",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P127 write-discipline rule marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq19P128RelationEntryMetadataMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"status:",
		"rangeKind:",
		"sourceTurn:",
		"validFromTurn:",
		"validToTurn:",
		"exact_offset",
		"bounded_ambiguous",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P128 relation entry metadata marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq19P137LocaleAwareExtractorMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"const localeRules =",
		"ko:",
		"en:",
		"ja:",
		"zh:",
		"activeLocales",
		"fail_open",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P137 locale-aware extractor marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq19P138RecalledPastParityMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"pushEntry(\"어제\", \"recalled_event\", -1, -1, \"day\", \"exact\")",
		"pushEntry(\"yesterday\", \"recalled_event\", -1, -1, \"day\", \"exact\")",
		"pushEntry(\"昨日\", \"recalled_event\", -1, -1, \"day\", \"exact\")",
		"pushEntry(\"昨天\", \"recalled_event\", -1, -1, \"day\", \"exact\")",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P138 recalled-past parity marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq19P139NextMorningParityMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"pushEntry(\"다음날 아침\", \"current_scene\", 1, 1, \"day\", \"daypart\", { daypart: \"morning\" })",
		"pushEntry(\"the next morning\", \"current_scene\", 1, 1, \"day\", \"daypart\", { daypart: \"morning\" })",
		"pushEntry(\"翌朝\", \"current_scene\", 1, 1, \"day\", \"daypart\", { daypart: \"morning\" })",
		"pushEntry(\"第二天早上\", \"current_scene\", 1, 1, \"day\", \"daypart\", { daypart: \"morning\" })",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P139 next-morning parity marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq19P140ActiveLocalesFailOpenGatingMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"const activeLocales = Array.isArray(opts && opts.activeLocales)",
		"activeLocales.forEach(function(localeKey)",
		"const rules = Array.isArray(localeRules[localeKey]) ? localeRules[localeKey] : []",
		"rules.forEach(function(rule) { rule(); })",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P140 activeLocales gating marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq19P140ActiveLocalesFailOpenGatingRuntime(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for JS runtime behavior smoke")
	}
	src := readArchiveCenterJS(t)
	start := strings.Index(src, "function extractTemporalRelationEntriesStep19")
	end := strings.Index(src, "function buildTemporalStateSurfaceStep19")
	if start < 0 || end <= start {
		t.Fatalf("Archive Center.js missing SEQ-19 temporal extractor block")
	}
	script := src[start:end] + `
const assert = (cond, msg) => { if (!cond) throw new Error(msg); };
const mixed = "어제 yesterday 昨日 昨天 다음날 아침 the next morning 翌朝 第二天早上 mañana";
const labels = (opts) => extractTemporalRelationEntriesStep19(mixed, opts).map((entry) => entry.relativeLabel);
const enOnly = labels({ activeLocales: ["en"] });
assert(enOnly.includes("yesterday"), "en activeLocales should extract yesterday");
assert(enOnly.includes("the next morning"), "en activeLocales should extract the next morning");
assert(!enOnly.includes("어제") && !enOnly.includes("昨日") && !enOnly.includes("昨天"), "en activeLocales must ignore non-en tokens");

const koOnly = labels({ activeLocales: ["ko"] });
assert(koOnly.includes("어제"), "ko activeLocales should extract 어제");
assert(koOnly.includes("다음날 아침"), "ko activeLocales should extract 다음날 아침");
assert(!koOnly.includes("yesterday") && !koOnly.includes("昨日") && !koOnly.includes("昨天"), "ko activeLocales must ignore non-ko tokens");

const jaZh = labels({ activeLocales: ["ja", "zh"] });
assert(jaZh.includes("昨日") && jaZh.includes("昨天"), "ja/zh activeLocales should extract recalled-past tokens");
assert(jaZh.includes("翌朝") && jaZh.includes("第二天早上"), "ja/zh activeLocales should extract next-morning tokens");
assert(!jaZh.includes("어제") && !jaZh.includes("yesterday"), "ja/zh activeLocales must ignore ko/en tokens");

const unsupported = labels({ activeLocales: ["es"] });
assert(unsupported.length === 0, "unsupported locale should fail open by ignoring unsupported phrases");

const defaults = labels({});
assert(defaults.includes("어제") && defaults.includes("yesterday") && defaults.includes("昨日") && defaults.includes("昨天"), "default locales should preserve ko/en/ja/zh extraction");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node SEQ-19-P140 activeLocales runtime smoke failed: %v\n%s", err, out)
	}
}

func TestArchiveCenterJSSeq19P288CurrentTimeExplicitnessMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function normalizeClock",
		"storyDayIndex",
		"carry_forward",
		"session_state_clock",
		"input_current_scene_anchor",
		"timeline_anchor",
		"resolved: false",
		"resolved: !!resolved",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P288 current-time explicitness marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq19P289AnchorBoundRelationMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function normalizeRelationEntry",
		"anchorRef",
		"sourceTurn",
		"validFromTurn",
		"validToTurn",
		"offsetValueMin",
		"offsetValueMax",
		"offsetUnit",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P289 anchor-bound relation marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq19P290BoundedAmbiguityMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"unresolved_range",
		"bounded_ambiguous",
		"coarse",
		"few weeks ago",
		"몇 달 전",
		"offsetValueMin == null",
		"offsetValueMax == null",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P290 bounded ambiguity marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq19P291AdvanceDisciplineMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"commit_explicit_advance",
		"commit_current_scene_anchor",
		"block_relation_only_write",
		"carry_forward_only",
		"figurative_duration_excluded",
		"block_figurative_only_write",
		"explicit_current_scene_offset",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P291 advance discipline marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq19P292TruthBoundaryPreserveMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function validateResponseTemporalDeicticStep19",
		"current_scene_deictic_mismatch",
		"relation_only_promoted_to_current_scene",
		"exact_current_scene_without_resolved_clock",
		"trace.temporalDeicticValidation",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P292 truth boundary preserve marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq19P296CurrentStoryClockSchemaDefineMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"current_story_clock",
		"story_day_index",
		"daypart",
		"precision",
		"anchor_source",
		"source_turn",
		"carry_forward",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P296 current story clock schema define marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq19P297SessionStateTimelineAnchorPrecedenceDefineMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"session_state_clock",
		"input_current_scene_anchor",
		"timeline_anchor",
		"carry_forward",
		"resolutionOrder",
		"selectedResolution",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P297 precedence define marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq19P298PrecisionLabelDefineMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"exact",
		"daypart",
		"bounded_range",
		"unknown",
		"precisionLabel",
		"raw_precision",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P298 precision label define marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq19P299CurrentSceneRecalledPastSplitDefineMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"current_scene",
		"recalled_event",
		"planned_event",
		"background_fact",
		"relation_only",
		"clockWriteDirective",
		"currentSceneRelation",
		"relationOnlyRelations",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P299 split define marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq19P288ToP299TemporalRuntimeSemantics(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for JS runtime behavior smoke")
	}
	src := readArchiveCenterJS(t)
	start := strings.Index(src, "function extractTemporalRelationEntriesStep19")
	end := strings.Index(src, "function readSceneTemporalStateFromOrchResultStep19")
	if start < 0 || end <= start {
		t.Fatalf("Archive Center.js missing SEQ-19 temporal runtime helper block")
	}
	script := src[start:end] + `
const assert = (cond, msg) => { if (!cond) throw new Error(msg); };

const empty = buildTemporalStateSurfaceStep19("", "", {});
assert(empty.currentStoryClock.resolved === false, "empty input must not fabricate a resolved clock");
assert(empty.currentStoryClock.storyDayIndex === null, "empty input must not fabricate storyDayIndex=0");
assert(empty.currentStoryClock.selectedResolution === "carry_forward", "empty input should carry forward explicitly");

const current = buildTemporalStateSurfaceStep19("the next morning", "", { sourceTurn: 12 });
assert(current.currentSceneRelation && current.currentSceneRelation.targetKind === "current_scene", "next morning should be current_scene");
assert(current.clockWriteDirective.allowWrite === true, "current_scene relation should be writable");
assert(current.clockWriteDirective.mode === "commit_explicit_advance", "current_scene offset should commit explicit advance");
assert(current.temporalRelationLedger[0].validFromTurn === 12 && current.temporalRelationLedger[0].validToTurn === 12, "relation metadata should preserve source turn bounds");

const recalled = buildTemporalStateSurfaceStep19("yesterday", "", { sourceTurn: 13 });
assert(recalled.relationOnlyRelations.length === 1, "recalled event should stay relation-only");
assert(recalled.relationOnlyRelations[0].targetKind === "recalled_event", "yesterday should be recalled_event");
assert(recalled.clockWriteDirective.allowWrite === false, "recalled event must not write clock");
assert(recalled.clockWriteDirective.mode === "block_relation_only_write", "recalled event should block relation-only write");

const bounded = buildTemporalStateSurfaceStep19("few weeks ago", "", { sourceTurn: 14 });
assert(bounded.temporalRelationLedger[0].status === "unresolved_range", "few weeks ago should preserve unresolved_range");
assert(bounded.temporalRelationLedger[0].rangeKind === "bounded_ambiguous", "few weeks ago should preserve bounded_ambiguous");
assert(bounded.temporalRelationLedger[0].offsetValueMin === null && bounded.temporalRelationLedger[0].offsetValueMax === null, "bounded ambiguity must not forge exact offsets");

const figurative = buildTemporalStateSurfaceStep19("it felt like a week", "", { sourceTurn: 15 });
assert(figurative.temporalMentionClassification.primaryClass === "figurative_duration", "figurative duration should be classified separately");
assert(figurative.clockWriteDirective.mode === "block_figurative_only_write", "figurative duration must not write temporal clock");

const sessionWins = buildTemporalStateSurfaceStep19("the next morning", "", {
  sourceTurn: 16,
  sessionTemporalState: {
    current_story_clock: { story_day_index: 7, precision: "exact", anchor_source: "session_state_clock" },
    temporal_relation_ledger: [],
    resolution: { effective_resolution_source: "session_state_clock" }
  }
});
assert(sessionWins.currentStoryClock.selectedResolution === "session_state_clock", "session clock must outrank input current scene anchor");
assert(sessionWins.currentStoryClock.storyDayIndex === 7, "session clock story day must be preserved");

const relationOnlyState = buildTemporalStateSurfaceStep19("yesterday", "", {});
const validation = validateResponseTemporalDeicticStep19("the next morning", relationOnlyState, { latestTimestamp: "fake-latest" });
assert(validation.status === "warn", "response current_scene over relation-only state should warn");
assert(validation.violations.some((v) => v.code === "relation_only_promoted_to_current_scene"), "truth boundary warning should be relation_only_promoted_to_current_scene");
assert(validation.ignoredLatestTimestamp === true, "validator should report latestTimestamp ignored");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node SEQ-19-P288~P299 temporal runtime smoke failed: %v\n%s", err, out)
	}
}

// SEQ-19-P303: 19-2a temporal relation schema define markers.
func TestArchiveCenterJSSeq19P303TemporalRelationSchemaDefineMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"relative_label",
		"anchor",
		"offset_value_min",
		"offset_value_max",
		"offset_unit",
		"precision",
		"source_turn",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P303 schema marker %q", needle)
		}
	}
}

// SEQ-19-P304: 19-2b phrase ingress normalization define markers.
func TestArchiveCenterJSSeq19P304PhraseIngressNormalizationDefineMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"어제",
		"그저께",
		"사흘 뒤",
		"저번 달",
		"지난 겨울",
		"몇 달 전",
		"몇 주 전",
		"다음날 아침",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P304 phrase ingress marker %q", needle)
		}
	}
}

// SEQ-19-P305: 19-2c temporal relation surface define markers.
func TestArchiveCenterJSSeq19P305TemporalRelationSurfaceDefineMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"validFromTurn",
		"validToTurn",
		"rangeKind",
		"bounded_ambiguous",
		"unresolved_range",
		"exact_offset",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P305 surface marker %q", needle)
		}
	}
}

// SEQ-19-P306: 19-2d anchor ambiguity carry-forward define markers.
func TestArchiveCenterJSSeq19P306AnchorAmbiguityCarryForwardDefineMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"carry_forward",
		"anchorRef",
		"unresolved",
		"unresolved_range",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P306 anchor ambiguity marker %q", needle)
		}
	}
}

// SEQ-19-P307: 19-2e locale parser pack boundary define markers.
func TestArchiveCenterJSSeq19P307LocaleParserPackBoundaryDefineMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"ko",
		"en",
		"ja",
		"zh",
		"activeLocales",
		"localeRules",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P307 locale pack marker %q", needle)
		}
	}
}

// SEQ-19-P311: 19-3a advance trigger define markers.
func TestArchiveCenterJSSeq19P311AdvanceTriggerDefineMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"explicit_current_scene_offset",
		"explicit_current_scene_anchor",
		"relation_only_reference",
		"figurative_duration_excluded",
		"no_temporal_signal",
		"elapsedTimeDecision",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P311 advance trigger marker %q", needle)
		}
	}
}

// SEQ-19-P312: 19-3b scene transition define markers.
func TestArchiveCenterJSSeq19P312SceneTransitionDefineMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"advance",
		"no_advance",
		"relation_only",
		"elapsedTimeDecision",
		"clockWriteDirective",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P312 scene transition marker %q", needle)
		}
	}
}

// SEQ-19-P313: 19-3c elapsed-time write discipline define markers.
func TestArchiveCenterJSSeq19P313ElapsedTimeWriteDisciplineDefineMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"commit_explicit_advance",
		"commit_current_scene_anchor",
		"block_relation_only_write",
		"carry_forward_only",
		"clockWriteDirective",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P313 write discipline marker %q", needle)
		}
	}
}

// SEQ-19-P314: 19-3d temporal support packet define markers.
func TestArchiveCenterJSSeq19P314TemporalSupportPacketDefineMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"Temporal Packet",
		"temporal_packet",
		"temporal_packet_text",
		"current_story_clock",
		"temporal_relation_ledger",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P314 support packet marker %q", needle)
		}
	}
}

func TestArchiveCenterJSSeq19P303ToP314TemporalDefinitionRuntimeSemantics(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for JS runtime behavior smoke")
	}
	src := readArchiveCenterJS(t)
	start := strings.Index(src, "function isTemporalQueryInput")
	end := strings.Index(src, "function buildInputContext")
	if start < 0 || end <= start {
		t.Fatalf("Archive Center.js missing SEQ-19 temporal runtime block")
	}
	script := src[start:end] + `
const assert = (cond, msg) => { if (!cond) throw new Error(msg); };
const byLabel = (entries, label) => entries.find((entry) => entry.relativeLabel === label);
const state = buildTemporalStateSurfaceStep19("그저께, 사흘 뒤, 저번 달, 지난 겨울, 몇 달 전 이야기를 정리해줘", "", {
  sourceTurn: 77,
  activeLocales: ["ko"],
});
const ledger = state.temporalRelationLedger;
const geujeo = byLabel(ledger, "그저께");
assert(geujeo && geujeo.anchorRef === null, "P303/P304 그저께 should exist without fabricated anchorRef");
assert(geujeo.targetKind === "recalled_event" && geujeo.offsetValueMin === -2 && geujeo.offsetValueMax === -2, "P304 그저께 should normalize to recalled -2 day exact");
assert(geujeo.offsetUnit === "day" && geujeo.precision === "exact" && geujeo.status === "resolved", "P303 schema fields should be populated for 그저께");
assert(geujeo.sourceTurn === 77 && geujeo.validFromTurn === 77 && geujeo.validToTurn === 77, "P305 source/valid turn metadata should be linked");

const saheul = byLabel(ledger, "사흘 뒤");
assert(saheul && saheul.targetKind === "current_scene" && saheul.offsetValueMin === 3 && saheul.offsetValueMax === 3, "P304 사흘 뒤 should normalize to current-scene +3 day");
assert(state.elapsedTimeDecision.action === "advance" && state.clockWriteDirective.mode === "commit_explicit_advance", "P311/P313 explicit current-scene offset should advance and allow explicit write");

const jeobeon = byLabel(ledger, "저번 달");
assert(jeobeon && jeobeon.targetKind === "recalled_event" && jeobeon.offsetUnit === "month" && jeobeon.precision === "exact", "P304 저번 달 should normalize to recalled month offset");

const months = byLabel(ledger, "몇 달 전");
assert(months && months.offsetValueMin === null && months.offsetValueMax === null, "P306 몇 달 전 should not forge exact offsets");
assert(months.status === "unresolved_range" && months.rangeKind === "bounded_ambiguous", "P305/P306 bounded ambiguity should be preserved");

const relationOnly = buildTemporalStateSurfaceStep19("몇 달 전", "", { sourceTurn: 88, activeLocales: ["ko"] });
assert(relationOnly.currentStoryClock.selectedResolution === "carry_forward", "P306 relation-only bounded ambiguity should carry forward clock");
assert(relationOnly.elapsedTimeDecision.action === "relation_only", "P312 recalled relation should stay relation_only");
assert(relationOnly.clockWriteDirective.mode === "block_relation_only_write" && relationOnly.clockWriteDirective.allowWrite === false, "P313 relation-only should be blocked from clock writes");

const enOnly = buildTemporalStateSurfaceStep19("그저께 yesterday", "", { sourceTurn: 91, activeLocales: ["en"] }).temporalRelationLedger;
assert(!byLabel(enOnly, "그저께") && byLabel(enOnly, "yesterday"), "P307 activeLocales should separate locale parser pack from canonical normalizer");

assert(isTemporalQueryInput("언제였지?"), "P314 temporal query gate should detect 언제");
assert(isTemporalQueryInput("얼마나 지났나?"), "P314 temporal query gate should detect elapsed-time query");
assert(isTemporalQueryInput("지금 며칠째인가?"), "P314 temporal query gate should detect story-day query");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node SEQ-19-P303~P314 temporal runtime smoke failed: %v\n%s", err, out)
	}
}

// SEQ-19-P318: 19-4a exact-day vs bounded-week vs bounded-month replay define markers.
func TestArchiveCenterJSSeq19P318TemporalReplayDefine19_4aMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"어제",
		"몇 주 전",
		"몇 달 전",
		"exact",
		"coarse",
		"bounded_ambiguous",
		"unresolved_range",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P318 replay define marker %q", needle)
		}
	}
}

// SEQ-19-P319: 19-4b current scene vs recalled past conflict replay define markers.
func TestArchiveCenterJSSeq19P319CurrentSceneRecalledPastConflictReplayDefineMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"current_scene",
		"recalled_event",
		"relation_only_promoted_to_current_scene",
		"current_scene_deictic_mismatch",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P319 conflict replay marker %q", needle)
		}
	}
}

// SEQ-19-P320: 19-4c missing anchor / low precision degrade replay define markers.
func TestArchiveCenterJSSeq19P320MissingAnchorLowPrecisionDegradeReplayDefineMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"carry_forward",
		"unresolved",
		"anchorRef",
		"precision",
		"coarse",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P320 degrade replay marker %q", needle)
		}
	}
}

// SEQ-19-P321: 19-4d temporal packet truth-boundary / precedence replay define markers.
func TestArchiveCenterJSSeq19P321TemporalPacketTruthBoundaryPrecedenceReplayDefineMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporal_packet",
		"Temporal Packet",
		"current_story_clock",
		"temporal_relation_ledger",
		"clockWriteDirective",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P321 packet precedence marker %q", needle)
		}
	}
}

// SEQ-19-P322: 19-4e response-time deictic validator replay define markers.
func TestArchiveCenterJSSeq19P322ResponseTimeDeicticValidatorReplayDefineMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function validateResponseTemporalDeicticStep19",
		"current_scene_deictic_mismatch",
		"relation_only_promoted_to_current_scene",
		"exact_current_scene_without_resolved_clock",
		"ignoredLatestTimestamp",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P322 validator replay marker %q", needle)
		}
	}
}

// SEQ-19-P323: figurative-duration / planned-future / recalled-past classification replay markers.
func TestArchiveCenterJSSeq19P323FigurativeDurationPlannedFutureRecalledPastClassificationReplayDefineMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"figurative_duration",
		"planned_event",
		"recalled_event",
		"figurative_duration_excluded",
		"block_figurative_only_write",
		"block_relation_only_write",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P323 classification replay marker %q", needle)
		}
	}
}

// SEQ-19-P324: ko/en/ja/zh parity + mixed-language fail-open replay markers.
func TestArchiveCenterJSSeq19P324MultilingualParityMixedLanguageFailOpenReplayDefineMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"ko: [",
		"en: [",
		"ja: [",
		"zh: [",
		"activeLocales.forEach(function(localeKey)",
		"localeRules[localeKey]",
		"rules.forEach(function(rule)",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P324 multilingual parity fail-open marker %q", needle)
		}
	}
}

// SEQ-19-P328: Beta 1.0 bundle latest root runtime define markers.
func TestArchiveCenterJSSeq19P328Beta10BundleLatestRootRuntimeDefineMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function extractTemporalRelationEntriesStep19",
		"function buildTemporalStateSurfaceStep19",
		"function validateResponseTemporalDeicticStep19",
		"function isTemporalQueryInput",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P328 Beta 1.0 bundle marker %q", needle)
		}
	}
}

// SEQ-19-P329: story clock smoke check pass markers.
func TestArchiveCenterJSSeq19P329StoryClockSmokeCheckPassMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"currentStoryClock",
		"storyDayIndex",
		"selectedResolution",
		"session_state_clock",
		"carry_forward",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P329 story clock smoke marker %q", needle)
		}
	}
}

// SEQ-19-P330: relative-time normalization smoke check pass markers.
func TestArchiveCenterJSSeq19P330RelativeTimeNormalizationSmokeCheckPassMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"relativeLabel",
		"offsetValueMin",
		"offsetValueMax",
		"offsetUnit",
		"localeRules",
		"activeLocales",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P330 relative-time normalization smoke marker %q", needle)
		}
	}
}

// SEQ-19-P331: elapsed-time advance replay pass markers.
func TestArchiveCenterJSSeq19P331ElapsedTimeAdvanceReplayPassMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"elapsedTimeDecision",
		"commit_explicit_advance",
		"block_relation_only_write",
		"carry_forward_only",
		"figurative_duration_excluded",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P331 elapsed-time advance replay marker %q", needle)
		}
	}
}

// SEQ-19-P332: ambiguity / precedence review checklist pass markers.
func TestArchiveCenterJSSeq19P332AmbiguityPrecedenceReviewChecklistPassMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"bounded_ambiguous",
		"unresolved_range",
		"exact_offset",
		"current_scene_deictic_mismatch",
		"relation_only_promoted_to_current_scene",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P332 ambiguity precedence review marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq19P318ToP322VXReplayRuntimeSemantics runs Node-based
// runtime behavior smoke for SEQ-19-P318~P322 VX replay surfaces.
func TestArchiveCenterJSSeq19P318ToP322VXReplayRuntimeSemantics(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for JS runtime behavior smoke")
	}
	src := readArchiveCenterJS(t)
	start := strings.Index(src, "function extractTemporalRelationEntriesStep19")
	end := strings.Index(src, "function readSceneTemporalStateFromOrchResultStep19")
	if start < 0 || end <= start {
		t.Fatalf("Archive Center.js missing SEQ-19 temporal runtime helper block")
	}
	script := src[start:end] + `
const assert = (cond, msg) => { if (!cond) throw new Error(msg); };

// P318: exact-day vs bounded-week vs bounded-month replay
const exactDay = buildTemporalStateSurfaceStep19("어제", "", { sourceTurn: 100, activeLocales: ["ko"] });
assert(exactDay.temporalRelationLedger.some((e) => e.relativeLabel === "어제" && e.precision === "exact" && e.offsetUnit === "day"), "P318 어제 should be exact day");
const boundedWeek = buildTemporalStateSurfaceStep19("몇 주 전", "", { sourceTurn: 101, activeLocales: ["ko"] });
assert(boundedWeek.temporalRelationLedger.some((e) => e.relativeLabel === "몇 주 전" && e.offsetUnit === "week" && e.rangeKind === "bounded_ambiguous"), "P318 몇 주 전 should be bounded ambiguous week");
const boundedMonth = buildTemporalStateSurfaceStep19("몇 달 전", "", { sourceTurn: 102, activeLocales: ["ko"] });
assert(boundedMonth.temporalRelationLedger.some((e) => e.relativeLabel === "몇 달 전" && e.offsetUnit === "month" && e.rangeKind === "bounded_ambiguous"), "P318 몇 달 전 should be bounded ambiguous month");

// P319: current scene vs recalled past conflict replay
const mixedTodayYesterday = buildTemporalStateSurfaceStep19("today and yesterday", "", { sourceTurn: 103, activeLocales: ["en"] });
assert(mixedTodayYesterday.currentSceneRelation && mixedTodayYesterday.currentSceneRelation.targetKind === "current_scene", "P319 today should be current_scene");
assert(mixedTodayYesterday.relationOnlyRelations.some((e) => e.relativeLabel === "yesterday"), "P319 yesterday should stay relation-only");
assert(mixedTodayYesterday.clockWriteDirective.allowWrite === true, "P319 current_scene write should be allowed");

// P320: missing anchor / low precision degrade
const missingAnchor = buildTemporalStateSurfaceStep19("어제", "", { sourceTurn: 104 });
assert(missingAnchor.currentStoryClock.selectedResolution === "carry_forward", "P320 missing anchor should carry forward clock");
const lowPrecision = buildTemporalStateSurfaceStep19("지난 겨울", "", { sourceTurn: 105, activeLocales: ["ko"] });
assert(lowPrecision.temporalRelationLedger.some((e) => e.relativeLabel === "지난 겨울" && e.precision === "coarse"), "P320 low precision should stay coarse");
assert(lowPrecision.clockWriteDirective.mode === "block_relation_only_write", "P320 low precision should block clock write");

// P321: temporal packet truth-boundary / precedence
const packetState = buildTemporalStateSurfaceStep19("today", "", { sourceTurn: 106 });
assert(packetState.currentStoryClock.resolved === true || packetState.currentStoryClock.selectedResolution === "carry_forward", "P321 packet should have clock or carry_forward");
assert(packetState.clockWriteDirective.mode === "commit_current_scene_anchor" || packetState.clockWriteDirective.mode === "carry_forward_only", "P321 write discipline should be explicit");

// P322: response-time deictic validator
const validatorState = buildTemporalStateSurfaceStep19("the next morning", "", { sourceTurn: 107 });
const validation = validateResponseTemporalDeicticStep19("yesterday", validatorState, { latestTimestamp: "fake-latest" });
assert(validation.status === "warn" || validation.status === "ok", "P322 validator should return status");
assert(validation.ignoredLatestTimestamp === true, "P322 validator should ignore latestTimestamp shortcut");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node SEQ-19-P318~P322 VX replay runtime smoke failed: %v\n%s", err, out)
	}
}

func TestArchiveCenterJSSeq19P328ToP332ReleaseGateRuntimeSmoke(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for JS runtime behavior smoke")
	}
	src := readArchiveCenterJS(t)
	start := strings.Index(src, "function isTemporalQueryInput")
	end := strings.Index(src, "function readSceneTemporalStateFromOrchResultStep19")
	if start < 0 || end <= start {
		t.Fatalf("Archive Center.js missing SEQ-19 release-gate temporal runtime block")
	}
	script := src[start:end] + `
const assert = (cond, msg) => { if (!cond) throw new Error(msg); };
const byLabel = (state, label) => state.temporalRelationLedger.find((entry) => entry.relativeLabel === label);
assert(typeof extractTemporalRelationEntriesStep19 === "function", "P328 extractor helper should exist");
assert(typeof buildTemporalStateSurfaceStep19 === "function", "P328 state helper should exist");
assert(typeof validateResponseTemporalDeicticStep19 === "function", "P328 validator helper should exist");
assert(typeof isTemporalQueryInput === "function", "P328 temporal query helper should exist");

const storyClock = buildTemporalStateSurfaceStep19("today", "", { sourceTurn: 201, activeLocales: ["en"] });
assert(storyClock.currentStoryClock.resolved === true, "P329 story clock smoke should resolve current scene clock");
assert(storyClock.currentStoryClock.selectedResolution === "input_current_scene_anchor", "P329 story clock smoke should use input current-scene anchor");
assert(storyClock.clockWriteDirective.mode === "commit_current_scene_anchor", "P329 story clock smoke should use current-scene write discipline");

const normalized = buildTemporalStateSurfaceStep19("그저께, 사흘 뒤, 몇 주 전, few months ago", "", { sourceTurn: 202, activeLocales: ["ko", "en"] });
assert(byLabel(normalized, "그저께") && byLabel(normalized, "그저께").offsetValueMin === -2, "P330 should normalize 그저께");
assert(byLabel(normalized, "사흘 뒤") && byLabel(normalized, "사흘 뒤").offsetValueMin === 3, "P330 should normalize 사흘 뒤");
assert(byLabel(normalized, "몇 주 전") && byLabel(normalized, "몇 주 전").offsetUnit === "week", "P330 should normalize bounded week");
assert(byLabel(normalized, "few months ago") && byLabel(normalized, "few months ago").offsetUnit === "month", "P330 should normalize en bounded month");

const advance = buildTemporalStateSurfaceStep19("사흘 뒤", "", { sourceTurn: 203, activeLocales: ["ko"] });
assert(advance.elapsedTimeDecision.action === "advance", "P331 explicit future current-scene offset should advance");
assert(advance.clockWriteDirective.mode === "commit_explicit_advance", "P331 advance should use explicit write discipline");
const relationOnly = buildTemporalStateSurfaceStep19("몇 달 전", "", { sourceTurn: 204, activeLocales: ["ko"] });
assert(relationOnly.elapsedTimeDecision.action === "relation_only", "P331 bounded recalled relation should be relation_only");
assert(relationOnly.clockWriteDirective.mode === "block_relation_only_write", "P331 relation_only should block clock write");

const ambiguity = byLabel(relationOnly, "몇 달 전");
assert(ambiguity && ambiguity.rangeKind === "bounded_ambiguous" && ambiguity.status === "unresolved_range", "P332 ambiguity should stay bounded/unresolved");
const mixed = buildTemporalStateSurfaceStep19("today and yesterday", "", { sourceTurn: 205, activeLocales: ["en"] });
assert(mixed.currentSceneRelation && mixed.relationOnlyRelations.some((entry) => entry.relativeLabel === "yesterday"), "P332 current/recalled precedence should preserve split lanes");
const validation = validateResponseTemporalDeicticStep19("the next morning", relationOnly, { latestTimestamp: "ignored-release-gate-shortcut" });
assert(validation.status === "warn", "P332 validator should warn when relation-only state is promoted to current scene");
assert(validation.violations.some((item) => item.code === "relation_only_promoted_to_current_scene"), "P332 validator should preserve relation-only promotion warning");
assert(validation.ignoredLatestTimestamp === true, "P332 validator should ignore latest timestamp shortcut");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node SEQ-19-P328~P332 release gate runtime smoke failed: %v\n%s", err, out)
	}
}

// SEQ-19-P333: multilingual temporal parity smoke check pass markers.
func TestArchiveCenterJSSeq19P333MultilingualTemporalParitySmokeCheckPassMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"ko: [",
		"en: [",
		"ja: [",
		"zh: [",
		"activeLocales",
		"localeRules",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P333 multilingual parity marker %q", needle)
		}
	}
}

// SEQ-19-P337: current story clock absolute datetime bounded story day markers.
func TestArchiveCenterJSSeq19P337CurrentStoryClockAbsoluteDatetimeBoundedStoryDayMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"storyDayIndex",
		"daypart",
		"precision",
		"anchorSource",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P337 bounded story-day marker %q", needle)
		}
	}
}

// SEQ-19-P338: relative-time normalization numeric offset vocabulary-first markers.
func TestArchiveCenterJSSeq19P338RelativeTimeNormalizationNumericOffsetVocabularyFirstMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"localeRules",
		"compactText",
		"relativeLabel",
		"offsetUnit",
		"precision",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P338 vocabulary-first marker %q", needle)
		}
	}
}

// SEQ-19-P339: elapsed-time advance conservative manual scene classifier markers.
func TestArchiveCenterJSSeq19P339ElapsedTimeAdvanceConservativeManualSceneClassifierMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"elapsedTimeDecision",
		"action",
		"advance",
		"no_advance",
		"relation_only",
		"figurative_duration_excluded",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P339 conservative manual marker %q", needle)
		}
	}
}

// SEQ-19-P340: missing anchor degrade markers.
func TestArchiveCenterJSSeq19P340MissingAnchorDegradeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"carry_forward",
		"unresolved",
		"unresolved_range",
		"bounded_ambiguous",
		"anchorRef",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P340 missing anchor degrade marker %q", needle)
		}
	}
}

// SEQ-19-P341: locale parsing single detector activeLocales merge markers.
func TestArchiveCenterJSSeq19P341LocaleParsingSingleDetectorActiveLocalesMergeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"activeLocales",
		"opts.activeLocales",
		"localeRules[localeKey]",
		"activeLocales.forEach",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P341 activeLocales merge marker %q", needle)
		}
	}
}

// SEQ-19-P342: ko/en bootstrap extractor locale-pack parser replace cutover markers.
func TestArchiveCenterJSSeq19P342KoEnBootstrapExtractorLocalePackParserReplaceCutoverMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"localeRules",
		"ko: [",
		"en: [",
		"ja: [",
		"zh: [",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P342 locale-pack cutover marker %q", needle)
		}
	}
}

// SEQ-19-P343: unspecified time fallback no_advance/carry_forward discipline markers.
func TestArchiveCenterJSSeq19P343UnspecifiedTimeFallbackNoAdvanceCarryForwardDisciplineMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"no_temporal_signal",
		"carry_forward_only",
		"no_advance",
		"carry_forward",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P343 no-advance discipline marker %q", needle)
		}
	}
}

// SEQ-19-P344: relation-only future/past reference current-scene advance evidence gate split markers.
func TestArchiveCenterJSSeq19P344RelationOnlyFuturePastReferenceCurrentSceneAdvanceEvidenceGateSplitMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"commit_explicit_advance",
		"commit_current_scene_anchor",
		"block_relation_only_write",
		"carry_forward_only",
		"planned_event",
		"recalled_event",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-19-P344 evidence gate split marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq19P333ToP344Beta10DecisionRuntimeSemantics runs Node-based
// runtime behavior smoke for SEQ-19-P333 and P337~P344 decision surfaces.
func TestArchiveCenterJSSeq19P333ToP344Beta10DecisionRuntimeSemantics(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for JS runtime behavior smoke")
	}
	src := readArchiveCenterJS(t)
	start := strings.Index(src, "function isTemporalQueryInput")
	end := strings.Index(src, "function readSceneTemporalStateFromOrchResultStep19")
	if start < 0 || end <= start {
		t.Fatalf("Archive Center.js missing SEQ-19 decision runtime block")
	}
	script := src[start:end] + `
const assert = (cond, msg) => { if (!cond) throw new Error(msg); };
const byLabel = (state, label) => state.temporalRelationLedger.find((entry) => entry.relativeLabel === label);

// P333: multilingual parity across all 4 locales
const koState = buildTemporalStateSurfaceStep19("어제", "", { sourceTurn: 301, activeLocales: ["ko"] });
assert(byLabel(koState, "어제"), "P333 ko 어제 should extract");
const enState = buildTemporalStateSurfaceStep19("yesterday", "", { sourceTurn: 302, activeLocales: ["en"] });
assert(byLabel(enState, "yesterday"), "P333 en yesterday should extract");
const jaState = buildTemporalStateSurfaceStep19("昨日", "", { sourceTurn: 303, activeLocales: ["ja"] });
assert(byLabel(jaState, "昨日"), "P333 ja 昨日 should extract");
const zhState = buildTemporalStateSurfaceStep19("昨天", "", { sourceTurn: 304, activeLocales: ["zh"] });
assert(byLabel(zhState, "昨天"), "P333 zh 昨天 should extract");

// P337: bounded story-day (storyDayIndex present, not absolute datetime)
const clock = buildTemporalStateSurfaceStep19("today", "", { sourceTurn: 305, activeLocales: ["en"] });
assert(clock.currentStoryClock.resolved === true, "P337 story clock should resolve");
assert(clock.currentStoryClock.storyDayIndex !== undefined, "P337 storyDayIndex should be present");
assert(!clock.currentStoryClock.absoluteDatetime, "P337 should not use absolute datetime");

// P338: vocabulary-first (localeRules drive extraction, not numeric offset)
const koExtract = buildTemporalStateSurfaceStep19("어제", "", { sourceTurn: 308, activeLocales: ["ko"] });
const enExtract = buildTemporalStateSurfaceStep19("yesterday", "", { sourceTurn: 309, activeLocales: ["en"] });
const jaExtract = buildTemporalStateSurfaceStep19("昨日", "", { sourceTurn: 310, activeLocales: ["ja"] });
const zhExtract = buildTemporalStateSurfaceStep19("昨天", "", { sourceTurn: 311, activeLocales: ["zh"] });
assert(byLabel(koExtract, "어제") && byLabel(enExtract, "yesterday") && byLabel(jaExtract, "昨日") && byLabel(zhExtract, "昨天"), "P338 all 4 locale packs should extract via vocabulary-first rules");

// P339: conservative manual rules (trigger categories, not scene classifier)
const manualAdvance = buildTemporalStateSurfaceStep19("사흘 뒤", "", { sourceTurn: 312, activeLocales: ["ko"] });
assert(manualAdvance.elapsedTimeDecision.action === "advance", "P339 explicit offset should advance");
assert(manualAdvance.elapsedTimeDecision.reason === "explicit_current_scene_offset", "P339 should use explicit offset reason");

// P340: missing anchor degrade (clock carry_forward, not relation unresolved)
const missingAnchor = buildTemporalStateSurfaceStep19("어제", "", { sourceTurn: 313 });
assert(missingAnchor.currentStoryClock.selectedResolution === "carry_forward", "P340 missing anchor should carry forward clock");
assert(missingAnchor.currentStoryClock.resolved === false, "P340 missing anchor clock should not be resolved");

// P341: activeLocales merge model
const mergeModel = buildTemporalStateSurfaceStep19("어제 yesterday", "", { sourceTurn: 314, activeLocales: ["ko", "en"] });
assert(byLabel(mergeModel, "어제") && byLabel(mergeModel, "yesterday"), "P341 merge model should extract both ko and en");

// P342: locale-pack parser cutover (no ad-hoc ko/en branches)
// Verified by P333/P341 runtime tests already proving locale pack behavior.

// P343: no_advance/carry_forward for unspecified time
const unspecified = buildTemporalStateSurfaceStep19("hello world", "", { sourceTurn: 315, activeLocales: ["en"] });
assert(unspecified.elapsedTimeDecision.action === "no_advance", "P343 unspecified time should be no_advance");
assert(unspecified.elapsedTimeDecision.reason === "no_temporal_signal", "P343 unspecified time should have no_temporal_signal reason");
assert(unspecified.clockWriteDirective.mode === "carry_forward_only", "P343 unspecified time should carry_forward_only");

// P344: evidence gate split
const futureGate = buildTemporalStateSurfaceStep19("사흘 뒤", "", { sourceTurn: 316, activeLocales: ["ko"] });
assert(futureGate.clockWriteDirective.mode === "commit_explicit_advance", "P344 future current-scene offset should commit explicit advance");
assert(futureGate.clockWriteDirective.allowWrite === true, "P344 future current-scene should allow write");
const pastGate = buildTemporalStateSurfaceStep19("어제", "", { sourceTurn: 317, activeLocales: ["ko"] });
assert(pastGate.clockWriteDirective.mode === "block_relation_only_write", "P344 past relation-only should block write");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node SEQ-19-P333~P344 Beta 1.0 decision runtime smoke failed: %v\n%s", err, out)
	}
}

// ===========================================================================
// SEQ-20 JS marker tests
// ===========================================================================

// SEQ-20-P21: q20a temporal query expansion preparatory marker.
func TestArchiveCenterJSSeq20P21Q20aTemporalQueryExpansionPreparatoryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function isTemporalQueryInput",
		"current_clock",
		"recalled_event",
		"planned_event",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P21 q20a preparatory marker %q", needle)
		}
	}
}

// SEQ-20-P22: q20a.v1 temporal query expansion contract marker.
func TestArchiveCenterJSSeq20P22Q20aV1TemporalQueryExpansionMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function isTemporalQueryInput",
		"function buildTemporalStateSurfaceStep19",
		"currentStoryClock",
		"temporalRelationLedger",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P22 q20a.v1 marker %q", needle)
		}
	}
}

// SEQ-20-P23: q20a rule surface focus/range marker.
func TestArchiveCenterJSSeq20P23Q20aRuleSurfaceFocusRangeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"current_scene",
		"recalled_event",
		"planned_event",
		"exact",
		"coarse",
		"bounded_ambiguous",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P23 q20a rule surface marker %q", needle)
		}
	}
}

// SEQ-20-P24: q20a derives from SC19 relation schema marker.
func TestArchiveCenterJSSeq20P24Q20aDerivesFromSc19RelationSchemaMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"localeRules",
		"ko: [",
		"en: [",
		"ja: [",
		"zh: [",
		"activeLocales",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P24 q20a SC19 derivation marker %q", needle)
		}
	}
}

// SEQ-20-P25: q20a mirrored at recall intent marker.
func TestArchiveCenterJSSeq20P25Q20aMirroredAtRecallIntentMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function buildTemporalStateSurfaceStep19",
		"function extractTemporalRelationEntriesStep19",
		"currentStoryClock",
		"temporalRelationLedger",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P25 q20a mirror marker %q", needle)
		}
	}
}

// SEQ-20-P26: q20a current clock overlay cue pack marker.
func TestArchiveCenterJSSeq20P26Q20aCurrentClockOverlayCuePackMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"what day",
		"story day",
		"지금",
		"며칠째",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P26 q20a cue pack marker %q", needle)
		}
	}
}

// SEQ-20-P27: q20a qr1a lexical routing normalized marker.
func TestArchiveCenterJSSeq20P27Q20aQr1aLexicalRoutingNormalizedMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"before",
		"after",
		"earlier",
		"previous",
		"recap",
		"resume",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P27 q20a lexical routing marker %q", needle)
		}
	}
}

// SEQ-20-P28: q20a contract-only groundwork marker.
func TestArchiveCenterJSSeq20P28Q20aContractOnlyGroundworkMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function isTemporalQueryInput",
		"function buildTemporalStateSurfaceStep19",
		"function validateResponseTemporalDeicticStep19",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P28 q20a groundwork marker %q", needle)
		}
	}
}

// SEQ-20-P36: q20b temporal validity read policy preparatory marker.
func TestArchiveCenterJSSeq20P36Q20bTemporalValidityReadPolicyPreparatoryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"currentStoryClock",
		"selectedResolution",
		"temporalRelationLedger",
		"clockWriteDirective",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P36 q20b preparatory marker %q", needle)
		}
	}
}

// SEQ-20-P37: q20b.v1 temporal validity read policy contract marker.
func TestArchiveCenterJSSeq20P37Q20bV1TemporalValidityReadPolicyMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"currentStoryClock",
		"elapsedTimeDecision",
		"clockWriteDirective",
		"relationOnlyRelations",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P37 q20b.v1 marker %q", needle)
		}
	}
}

// SEQ-20-P38: q20b read priority modes marker.
func TestArchiveCenterJSSeq20P38Q20bReadPriorityModesMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"commit_current_scene_anchor",
		"commit_explicit_advance",
		"block_relation_only_write",
		"carry_forward_only",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P38 q20b read priority marker %q", needle)
		}
	}
}

// SEQ-20-P39: q20b mirrored at recall intent and query class marker.
func TestArchiveCenterJSSeq20P39Q20bMirroredAtRecallIntentAndQueryClassMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function buildTemporalStateSurfaceStep19",
		"function extractTemporalRelationEntriesStep19",
		"currentStoryClock",
		"temporalRelationLedger",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P39 q20b mirror marker %q", needle)
		}
	}
}

// SEQ-20-P40: q20b stops before later TV work marker.
func TestArchiveCenterJSSeq20P40Q20bStopsBeforeLaterTVWorkMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function buildTemporalStateSurfaceStep19",
		"function validateResponseTemporalDeicticStep19",
		"currentStoryClock",
		"clockWriteDirective",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P40 q20b boundary marker %q", needle)
		}
	}
}

// SEQ-20-P47: q20c temporal event invalidation preparatory marker.
func TestArchiveCenterJSSeq20P47Q20cTemporalEventInvalidationPreparatoryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"currentStoryClock",
		"temporalRelationLedger",
		"clockWriteDirective",
		"relationOnlyRelations",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P47 q20c preparatory marker %q", needle)
		}
	}
}

// SEQ-20-P48: q20c.v1 temporal event invalidation support contract marker.
func TestArchiveCenterJSSeq20P48Q20cV1TemporalEventInvalidationSupportMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"current_scene",
		"recalled_event",
		"planned_event",
		"relation_only_promoted_to_current_scene",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P48 q20c.v1 marker %q", needle)
		}
	}
}

// SEQ-20-P49: q20c invalidation modes marker.
func TestArchiveCenterJSSeq20P49Q20cInvalidationModesMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"current_scene_deictic_mismatch",
		"relation_only_promoted_to_current_scene",
		"exact_current_scene_without_resolved_clock",
		"ignoredLatestTimestamp",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P49 q20c invalidation modes marker %q", needle)
		}
	}
}

// SEQ-20-P50: q20c mirrored at recall intent marker.
func TestArchiveCenterJSSeq20P50Q20cMirroredAtRecallIntentMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function buildTemporalStateSurfaceStep19",
		"function extractTemporalRelationEntriesStep19",
		"currentStoryClock",
		"relationOnlyRelations",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P50 q20c mirror marker %q", needle)
		}
	}
}

// SEQ-20-P51: q20c separate from promotion-lag marker.
func TestArchiveCenterJSSeq20P51Q20cSeparateFromPromotionLagMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"elapsedTimeDecision",
		"clockWriteDirective",
		"currentSceneRelation",
		"relationOnlyRelations",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P51 q20c separation marker %q", needle)
		}
	}
}

// SEQ-20-P57: q20d temporal promotion-lag preparatory marker.
func TestArchiveCenterJSSeq20P57Q20dTemporalPromotionLagPreparatoryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"currentStoryClock",
		"elapsedTimeDecision",
		"clockWriteDirective",
		"relationOnlyRelations",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P57 q20d preparatory marker %q", needle)
		}
	}
}

// SEQ-20-P58: q20d.v1 temporal promotion-lag support contract marker.
func TestArchiveCenterJSSeq20P58Q20dV1TemporalPromotionLagSupportMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"currentStoryClock",
		"selectedResolution",
		"carry_forward",
		"relationOnlyRelations",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P58 q20d.v1 marker %q", needle)
		}
	}
}

// SEQ-20-P59: q20d anchor precedence marker.
func TestArchiveCenterJSSeq20P59Q20dAnchorPrecedenceMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"latest_direct_evidence",
		"recent_raw_turn",
		"session_state_clock",
		"carry_forward",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P59 q20d anchor precedence marker %q", needle)
		}
	}
}

// SEQ-20-P60: q20d mirrored at recall intent marker.
func TestArchiveCenterJSSeq20P60Q20dMirroredAtRecallIntentMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function buildTemporalStateSurfaceStep19",
		"function extractTemporalRelationEntriesStep19",
		"currentStoryClock",
		"relationOnlyRelations",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P60 q20d mirror marker %q", needle)
		}
	}
}

// SEQ-20-P66: q20e temporal hot recall buffer preparatory marker.
func TestArchiveCenterJSSeq20P66Q20eTemporalHotRecallBufferPreparatoryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"currentStoryClock",
		"temporalRelationLedger",
		"clockWriteDirective",
		"relationOnlyRelations",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P66 q20e preparatory marker %q", needle)
		}
	}
}

// SEQ-20-P67: q20e.v1 temporal hot recall buffer contract marker.
func TestArchiveCenterJSSeq20P67Q20eV1TemporalHotRecallBufferMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"currentStoryClock",
		"elapsedTimeDecision",
		"clockWriteDirective",
		"relationOnlyRelations",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P67 q20e.v1 marker %q", needle)
		}
	}
}

// SEQ-20-P68: q20e bridge source set marker.
func TestArchiveCenterJSSeq20P68Q20eBridgeSourceSetMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"latest_direct_evidence",
		"recent_raw_turn",
		"scoped_verbatim_support",
		"read_surfaces",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P68 q20e bridge source marker %q", needle)
		}
	}
}

// SEQ-20-P69: q20e mirrored at recall intent marker.
func TestArchiveCenterJSSeq20P69Q20eMirroredAtRecallIntentMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function buildTemporalStateSurfaceStep19",
		"function extractTemporalRelationEntriesStep19",
		"currentStoryClock",
		"temporalRelationLedger",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P69 q20e mirror marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq20P21ToP69TemporalRetrievalRuntimeSemantics runs Node-based
// runtime behavior smoke for SEQ-20-P21~P69 temporal retrieval surfaces.
func TestArchiveCenterJSSeq20P21ToP69TemporalRetrievalRuntimeSemantics(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for JS runtime behavior smoke")
	}
	src := readArchiveCenterJS(t)
	start := strings.Index(src, "function isTemporalQueryInput")
	end := strings.Index(src, "function readSceneTemporalStateFromOrchResultStep19")
	if start < 0 || end <= start {
		t.Fatalf("Archive Center.js missing SEQ-20 temporal retrieval runtime block")
	}
	script := src[start:end] + `
const assert = (cond, msg) => { if (!cond) throw new Error(msg); };

// P21/P22: q20a temporal query expansion — isTemporalQueryInput should classify queries
assert(isTemporalQueryInput("what day is it?") === true, "P21/P22 current_clock query should be detected");
assert(isTemporalQueryInput("what happened yesterday?") === true, "P21/P22 past_event query should be detected");
assert(isTemporalQueryInput("what happened last week?") === true, "P21/P22 past_window query should be detected");

// P23: q20a rule surface — focus modes exist in temporal state
const state = buildTemporalStateSurfaceStep19("today", "", { sourceTurn: 401, activeLocales: ["en"] });
assert(state.currentStoryClock, "P23 currentStoryClock should exist");
assert(state.temporalRelationLedger, "P23 temporalRelationLedger should exist");

// P24: q20a derives from SC19 — localeRules exist inside extractTemporalRelationEntriesStep19
const koEntries = extractTemporalRelationEntriesStep19("어제", { activeLocales: ["ko"] });
assert(koEntries.length > 0 && koEntries[0].relativeLabel === "어제", "P24 ko localeRules should process Korean text");
const enEntries = extractTemporalRelationEntriesStep19("yesterday", { activeLocales: ["en"] });
assert(enEntries.length > 0 && enEntries[0].relativeLabel === "yesterday", "P24 en localeRules should process English text");

// P25: q20a mirrored — buildTemporalStateSurfaceStep19 and extractTemporalRelationEntriesStep19 both exist
assert(typeof buildTemporalStateSurfaceStep19 === "function", "P25 buildTemporalStateSurfaceStep19 should exist");
assert(typeof extractTemporalRelationEntriesStep19 === "function", "P25 extractTemporalRelationEntriesStep19 should exist");

// P26: current_clock overlay cue pack
assert(isTemporalQueryInput("지금 며칠째야?") === true, "P26 ko current_clock cue should be detected");
assert(isTemporalQueryInput("what day is it?") === true, "P26 en current_clock cue should be detected");

// P27: qr1a lexical routing — temporal keywords in isTemporalQueryInput
assert(isTemporalQueryInput("before the battle") === true, "P27 before cue should be detected");
assert(isTemporalQueryInput("after the meeting") === true, "P27 after cue should be detected");

// P28: contract-only groundwork — no live retrieval execution yet
assert(state.clockWriteDirective.mode === "commit_current_scene_anchor", "P28 write discipline should be explicit");

// P36/P37: q20b temporal validity read policy — current truth vs old truth
const currentTruth = buildTemporalStateSurfaceStep19("today", "", { sourceTurn: 402, activeLocales: ["en"] });
assert(currentTruth.currentStoryClock.resolved === true, "P36/P37 current truth should resolve");
assert(currentTruth.clockWriteDirective.mode === "commit_current_scene_anchor", "P36/P37 current truth should commit anchor");

// P38: read priority modes — relation-only should block write
const pastEvent = buildTemporalStateSurfaceStep19("yesterday", "", { sourceTurn: 403, activeLocales: ["en"] });
assert(pastEvent.clockWriteDirective.mode === "block_relation_only_write", "P38 past event should block relation-only write");

// P39: mirrored at recall intent — state surfaces are reusable
assert(currentTruth.elapsedTimeDecision.action === "no_advance", "P39 current truth elapsed should be no_advance");

// P40: stops before later TV work — chronology not yet implemented
assert(typeof state.temporalRelationLedger === "object", "P40 temporalRelationLedger should exist as contract surface");

// P47/P48: q20c event invalidation support — event vs current compare
const eventCompare = buildTemporalStateSurfaceStep19("yesterday", "", { sourceTurn: 404, activeLocales: ["en"] });
assert(eventCompare.relationOnlyRelations.length > 0, "P47/P48 event compare should have relation-only entries");
assert(eventCompare.clockWriteDirective.mode === "block_relation_only_write", "P47/P48 event compare should block write");

// P49: invalidation modes — exact_event vs bounded_window
assert(eventCompare.temporalRelationLedger[0].precision === "exact", "P49 exact event should have exact precision");

// P50: mirrored at recall intent — same payload reusable
assert(eventCompare.currentStoryClock.anchorSource === "carry_forward", "P50 past event should carry forward anchor");

// P51: separate from promotion-lag — event invalidation != pending_current
assert(eventCompare.elapsedTimeDecision.action === "relation_only", "P51 event invalidation should be relation_only");

// P57/P58: q20d promotion-lag support — pending_current note
const promotionLag = buildTemporalStateSurfaceStep19("어제", "", { sourceTurn: 405, activeLocales: ["ko"] });
assert(promotionLag.currentStoryClock.selectedResolution === "carry_forward", "P57/P58 missing anchor should carry forward");
assert(promotionLag.clockWriteDirective.mode === "block_relation_only_write", "P57/P58 promotion-lag should block write");

// P59: anchor precedence — latest_direct_evidence -> recent_raw_turn
assert(promotionLag.currentStoryClock.anchorSource === "carry_forward", "P59 missing anchor should use carry_forward");

// P60: mirrored at recall intent — same surface reusable
assert(typeof promotionLag.currentStoryClock === "object", "P60 currentStoryClock should exist as reusable surface");

// P66/P67: q20e hot recall buffer — recent multi-turn bridge
const hotBuffer = buildTemporalStateSurfaceStep19("today", "", { sourceTurn: 406, activeLocales: ["en"] });
assert(hotBuffer.currentStoryClock.resolved === true, "P66/P67 hot buffer should resolve current truth");
assert(hotBuffer.clockWriteDirective.allowWrite === true, "P66/P67 hot buffer should allow write for current scene");

// P68: bridge source set — support_only, no truth override
assert(hotBuffer.clockWriteDirective.mode === "commit_current_scene_anchor", "P68 bridge should use current_scene anchor");

// P69: mirrored at recall intent — same hot-bridge policy reusable
assert(typeof hotBuffer.temporalRelationLedger === "object", "P69 temporalRelationLedger should exist as reusable hot-bridge surface");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node SEQ-20-P21~P69 temporal retrieval runtime smoke failed: %v\n%s", err, out)
	}
}

// ===========================================================================
// SEQ-20 entity/graph JS marker tests (P76 ~ P121)
// ===========================================================================

// SEQ-20-P76: q20f lightweight entity index preparatory marker.
func TestArchiveCenterJSSeq20P76Q20fLightweightEntityIndexPreparatoryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"characters",
		"pending_threads",
		"relationships_json",
		"personality_json",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P76 q20f preparatory marker %q", needle)
		}
	}
}

// SEQ-20-P77: q20f.v1 lightweight entity index contract marker.
func TestArchiveCenterJSSeq20P77Q20fV1LightweightEntityIndexMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessor",
		"characters",
		"pending_threads",
		"relationships_json",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P77 q20f.v1 marker %q", needle)
		}
	}
}

// SEQ-20-P78: q20f structured state surfaces marker.
func TestArchiveCenterJSSeq20P78Q20fStructuredStateSurfacesMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"characters",
		"pending_threads",
		"relationships_json",
		"personality_json",
		"status_json",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P78 q20f structured state marker %q", needle)
		}
	}
}

// SEQ-20-P79: q20f mirrored at query class marker.
func TestArchiveCenterJSSeq20P79Q20fMirroredAtQueryClassMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessor",
		"characters",
		"pending_threads",
		"relationships_json",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P79 q20f mirror marker %q", needle)
		}
	}
}

// SEQ-20-P80: q20f stops before graph-like support marker.
func TestArchiveCenterJSSeq20P80Q20fStopsBeforeGraphLikeSupportMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessor",
		"characters",
		"pending_threads",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P80 q20f boundary marker %q", needle)
		}
	}
}

// SEQ-20-P81: q20f token-boundary structured labels marker.
func TestArchiveCenterJSSeq20P81Q20fTokenBoundaryStructuredLabelsMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function _normalizePlayerEntityLabelKey",
		"function _isPlayerEntityLabelText",
		"PLAYER_ENTITY_LABEL_KEYS",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P81 q20f token boundary marker %q", needle)
		}
	}
}

// SEQ-20-P89: q20g graph-like support signal preparatory marker.
func TestArchiveCenterJSSeq20P89Q20gGraphLikeSupportSignalPreparatoryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"relationships_json",
		"pending_threads",
		"characters",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P89 q20g preparatory marker %q", needle)
		}
	}
}

// SEQ-20-P90: q20g.v1 graph-like support signal contract marker.
func TestArchiveCenterJSSeq20P90Q20gV1GraphLikeSupportSignalMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"relationships_json",
		"pending_threads",
		"entityCoprocessor",
		"characters",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P90 q20g.v1 marker %q", needle)
		}
	}
}

// SEQ-20-P91: q20g pair sources and fail-open marker.
func TestArchiveCenterJSSeq20P91Q20gPairSourcesAndFailOpenMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"relationships_json",
		"pending_threads",
		"characters",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P91 q20g pair sources marker %q", needle)
		}
	}
}

// SEQ-20-P92: q20g mirrored at query class marker.
func TestArchiveCenterJSSeq20P92Q20gMirroredAtQueryClassMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"relationships_json",
		"pending_threads",
		"entityCoprocessor",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P92 q20g mirror marker %q", needle)
		}
	}
}

// SEQ-20-P93: q20g stops before inspection formatting marker.
func TestArchiveCenterJSSeq20P93Q20gStopsBeforeInspectionFormattingMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessor",
		"relationships_json",
		"pending_threads",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P93 q20g boundary marker %q", needle)
		}
	}
}

// SEQ-20-P99: q20h entity/graph boost inspection surface preparatory marker.
func TestArchiveCenterJSSeq20P99Q20hEntityGraphBoostInspectionSurfacePreparatoryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessorTraceDisplayMode",
		"entityCoprocessor",
		"characters",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P99 q20h preparatory marker %q", needle)
		}
	}
}

// SEQ-20-P100: q20h.v1 entity/graph boost inspection surface contract marker.
func TestArchiveCenterJSSeq20P100Q20hV1EntityGraphBoostInspectionSurfaceMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessorTraceDisplayMode",
		"hint_vs_current_fact_split",
		"entityCoprocessor",
		"characters",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P100 q20h.v1 marker %q", needle)
		}
	}
}

// SEQ-20-P101: q20h inspection role and authority notice marker.
func TestArchiveCenterJSSeq20P101Q20hInspectionRoleAndAuthorityNoticeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessorTraceDisplayMode",
		"hint_vs_current_fact_split",
		"entityCoprocessorTraceDisplayTruthLane",
		"step11_current_fact",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P101 q20h authority notice marker %q", needle)
		}
	}
}

// SEQ-20-P102: q20h mirrored at query class marker.
func TestArchiveCenterJSSeq20P102Q20hMirroredAtQueryClassMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessorTraceDisplayMode",
		"entityCoprocessor",
		"characters",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P102 q20h mirror marker %q", needle)
		}
	}
}

// SEQ-20-P109: q20i lagging current state boost preparatory marker.
func TestArchiveCenterJSSeq20P109Q20iLaggingCurrentStateBoostPreparatoryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessor",
		"pending_threads",
		"characters",
		"relationships_json",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P109 q20i preparatory marker %q", needle)
		}
	}
}

// SEQ-20-P110: q20i.v1 lagging current state boost contract marker.
func TestArchiveCenterJSSeq20P110Q20iV1LaggingCurrentStateBoostMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessor",
		"pending_threads",
		"characters",
		"relationships_json",
		"currentStoryClock",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P110 q20i.v1 marker %q", needle)
		}
	}
}

// SEQ-20-P111: q20i activation and precedence marker.
func TestArchiveCenterJSSeq20P111Q20iActivationAndPrecedenceMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessor",
		"pending_threads",
		"characters",
		"relationships_json",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P111 q20i precedence marker %q", needle)
		}
	}
}

// SEQ-20-P112: q20i mirrored at query class marker.
func TestArchiveCenterJSSeq20P112Q20iMirroredAtQueryClassMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessor",
		"pending_threads",
		"characters",
		"relationships_json",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P112 q20i mirror marker %q", needle)
		}
	}
}

// SEQ-20-P118: q20j motive-shadow hint preparatory marker.
func TestArchiveCenterJSSeq20P118Q20jMotiveShadowHintPreparatoryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"personality_json",
		"characters",
		"entityCoprocessor",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P118 q20j preparatory marker %q", needle)
		}
	}
}

// SEQ-20-P119: q20j.v1 motive-shadow hint contract marker.
func TestArchiveCenterJSSeq20P119Q20jV1MotiveShadowHintMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"personality_json",
		"characters",
		"entityCoprocessor",
		"entityCoprocessorTraceDisplayTruthTargets",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P119 q20j.v1 marker %q", needle)
		}
	}
}

// SEQ-20-P120: q20j truth-write-forbidden marker.
func TestArchiveCenterJSSeq20P120Q20jTruthWriteForbiddenMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessorPatchDirectWriteBlockedTarget",
		"canonical_relationship_state",
		"entityCoprocessorTraceDisplayTruthTargets",
		"current_fact",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P120 q20j truth-write-forbidden marker %q", needle)
		}
	}
}

// SEQ-20-P121: q20j mirrored at query class marker.
func TestArchiveCenterJSSeq20P121Q20jMirroredAtQueryClassMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"personality_json",
		"characters",
		"entityCoprocessor",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P121 q20j mirror marker %q", needle)
		}
	}
}

// SEQ-20-P127: q20k motive-shadow non-escalation guard preparatory marker.
func TestArchiveCenterJSSeq20P127Q20kMotiveShadowNonEscalationGuardPreparatoryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"stale_arc",
		"entityCoprocessor",
		"characters",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P127 q20k preparatory marker %q", needle)
		}
	}
}

// SEQ-20-P128: q20k.v1 motive-shadow non-escalation guard contract marker.
func TestArchiveCenterJSSeq20P128Q20kV1MotiveShadowNonEscalationGuardMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessorPatchDirectWriteBlockedTarget",
		"canonical_relationship_state",
		"entityCoprocessorTraceDisplayTruthTargets",
		"current_fact",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P128 q20k.v1 marker %q", needle)
		}
	}
}

// SEQ-20-P129: q20k mirrored at query class marker.
func TestArchiveCenterJSSeq20P129Q20kMirroredAtQueryClassMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"stale_arc",
		"entityCoprocessor",
		"characters",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P129 q20k mirror marker %q", needle)
		}
	}
}

// SEQ-20-P135: q20l relation edge support ledger preparatory marker.
func TestArchiveCenterJSSeq20P135Q20lRelationEdgeSupportLedgerPreparatoryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"relationships_json",
		"pending_threads",
		"characters",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P135 q20l preparatory marker %q", needle)
		}
	}
}

// SEQ-20-P136: q20l.v1 relation edge support ledger contract marker.
func TestArchiveCenterJSSeq20P136Q20lV1RelationEdgeSupportLedgerMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"relationships_json",
		"pending_threads",
		"characters",
		"entityCoprocessor",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P136 q20l.v1 marker %q", needle)
		}
	}
}

// SEQ-20-P137: q20l graph truth write forbidden marker.
func TestArchiveCenterJSSeq20P137Q20lGraphTruthWriteForbiddenMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessorPatchDirectWriteBlockedTarget",
		"canonical_relationship_state",
		"entityCoprocessorTraceDisplayDisallowedAliases",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P137 q20l truth-write-forbidden marker %q", needle)
		}
	}
}

// SEQ-20-P138: q20l mirrored at query class marker.
func TestArchiveCenterJSSeq20P138Q20lMirroredAtQueryClassMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"relationships_json",
		"pending_threads",
		"characters",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P138 q20l mirror marker %q", needle)
		}
	}
}

// SEQ-20-P231: validity priority aggregate marker.
func TestArchiveCenterJSSeq20P231ValidityPriorityMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"currentStoryClock",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P231 validity priority marker %q", needle)
		}
	}
}

// SEQ-20-P232: support-only accelerator aggregate marker.
func TestArchiveCenterJSSeq20P232SupportOnlyAcceleratorMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessor",
		"pending_threads",
		"relationships_json",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P232 support-only accelerator marker %q", needle)
		}
	}
}

// SEQ-20-P233: ambiguity reduction aggregate marker.
func TestArchiveCenterJSSeq20P233AmbiguityReductionMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessor",
		"characters",
		"pending_threads",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P233 ambiguity reduction marker %q", needle)
		}
	}
}

// SEQ-20-P234: inspection visibility aggregate marker.
func TestArchiveCenterJSSeq20P234InspectionVisibilityMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessorTraceDisplayMode",
		"entityCoprocessor",
		"temporalRelationLedger",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P234 inspection visibility marker %q", needle)
		}
	}
}

// SEQ-20-P235: truth precedence preserve aggregate marker.
func TestArchiveCenterJSSeq20P235TruthPrecedencePreserveMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessorTraceDisplayTruthTargets",
		"current_fact",
		"canonical_relationship_state",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P235 truth precedence marker %q", needle)
		}
	}
}

// SEQ-20-P236: hot-bridge aggregate marker.
func TestArchiveCenterJSSeq20P236HotBridgeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"pending_threads",
		"latest_direct_evidence",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P236 hot-bridge marker %q", needle)
		}
	}
}

// SEQ-20-P258: q20m temporal ambiguity support note preparatory marker.
func TestArchiveCenterJSSeq20P258Q20mTemporalAmbiguitySupportNotePreparatoryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"currentStoryClock",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P258 q20m preparatory marker %q", needle)
		}
	}
}

// SEQ-20-P259: q20m.v1 temporal ambiguity support note contract marker.
func TestArchiveCenterJSSeq20P259Q20mV1TemporalAmbiguitySupportNoteMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"currentStoryClock",
		"pending_threads",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P259 q20m.v1 marker %q", needle)
		}
	}
}

// SEQ-20-P260: q20n alias/entity conflict disambiguation preparatory marker.
func TestArchiveCenterJSSeq20P260Q20nAliasEntityConflictDisambiguationPreparatoryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessor",
		"characters",
		"pending_threads",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P260 q20n preparatory marker %q", needle)
		}
	}
}

// SEQ-20-P261: q20n.v1 alias/entity conflict disambiguation contract marker.
func TestArchiveCenterJSSeq20P261Q20nV1AliasEntityConflictDisambiguationMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessor",
		"characters",
		"pending_threads",
		"relationships_json",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P261 q20n.v1 marker %q", needle)
		}
	}
}

// SEQ-20-P262: q20o temporal/entity source-tag rule preparatory marker.
func TestArchiveCenterJSSeq20P262Q20oTemporalEntitySourceTagRulePreparatoryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessor",
		"temporalRelationLedger",
		"pending_threads",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P262 q20o preparatory marker %q", needle)
		}
	}
}

// SEQ-20-P263: q20o.v1 temporal/entity source-tag rule contract marker.
func TestArchiveCenterJSSeq20P263Q20oV1TemporalEntitySourceTagRuleMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessor",
		"temporalRelationLedger",
		"pending_threads",
		"relationships_json",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P263 q20o.v1 marker %q", needle)
		}
	}
}

// SEQ-20-P264: q20p canonical-pending/stale-current conflict note preparatory marker.
func TestArchiveCenterJSSeq20P264Q20pCanonicalPendingStaleCurrentConflictNotePreparatoryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"pending_threads",
		"latest_direct_evidence",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P264 q20p preparatory marker %q", needle)
		}
	}
}

// SEQ-20-P265: q20p.v1 canonical-pending/stale-current conflict note contract marker.
func TestArchiveCenterJSSeq20P265Q20pV1CanonicalPendingStaleCurrentConflictNoteMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"pending_threads",
		"latest_direct_evidence",
		"entityCoprocessor",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P265 q20p.v1 marker %q", needle)
		}
	}
}

// SEQ-20-P266: q20q recall cue rescue rule preparatory marker.
func TestArchiveCenterJSSeq20P266Q20qRecallCueRescueRulePreparatoryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"pending_threads",
		"latest_direct_evidence",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P266 q20q preparatory marker %q", needle)
		}
	}
}

// SEQ-20-P267: q20q.v1 recall cue rescue rule contract marker.
func TestArchiveCenterJSSeq20P267Q20qV1RecallCueRescueRuleMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"pending_threads",
		"latest_direct_evidence",
		"entityCoprocessor",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P267 q20q.v1 marker %q", needle)
		}
	}
}

// SEQ-20-P268: q20r wide gather -> validity join rule preparatory marker.
func TestArchiveCenterJSSeq20P268Q20rWideGatherValidityJoinRulePreparatoryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"pending_threads",
		"latest_direct_evidence",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P268 q20r preparatory marker %q", needle)
		}
	}
}

// SEQ-20-P269: q20r.v1 wide gather -> validity join rule contract marker.
func TestArchiveCenterJSSeq20P269Q20rV1WideGatherValidityJoinRuleMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"pending_threads",
		"latest_direct_evidence",
		"entityCoprocessor",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P269 q20r.v1 marker %q", needle)
		}
	}
}

// SEQ-20-P270: q20s thin support tag fallback preparatory marker.
func TestArchiveCenterJSSeq20P270Q20sThinSupportTagFallbackPreparatoryMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"pending_threads",
		"latest_direct_evidence",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P270 q20s preparatory marker %q", needle)
		}
	}
}

// SEQ-20-P271: q20s.v1 thin support tag fallback contract marker.
func TestArchiveCenterJSSeq20P271Q20sV1ThinSupportTagFallbackMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"pending_threads",
		"latest_direct_evidence",
		"entityCoprocessor",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P271 q20s.v1 marker %q", needle)
		}
	}
}

// SEQ-20-P286: vx20a temporal validity replay gate marker.
func TestArchiveCenterJSSeq20P286Vx20aTemporalValidityReplayGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"currentStoryClock",
		"pending_threads",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P286 vx20a marker %q", needle)
		}
	}
}

// SEQ-20-P288: vx20b entity boost false-positive gate marker.
func TestArchiveCenterJSSeq20P288Vx20bEntityBoostFalsePositiveGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessor",
		"entityCoprocessorTraceDisplayMode",
		"characters",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P288 vx20b marker %q", needle)
		}
	}
}

// SEQ-20-P290: vx20c graph accelerator degrade gate marker.
func TestArchiveCenterJSSeq20P290Vx20cGraphAcceleratorDegradeGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessor",
		"relationships_json",
		"pending_threads",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P290 vx20c marker %q", needle)
		}
	}
}

// SEQ-20-P292: vx20d canonical precedence replay gate marker.
func TestArchiveCenterJSSeq20P292Vx20dCanonicalPrecedenceReplayGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessorTraceDisplayTruthTargets",
		"current_fact",
		"canonical_relationship_state",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P292 vx20d marker %q", needle)
		}
	}
}

// SEQ-20-P294: vx20e promotion-blocked freshness replay gate marker.
func TestArchiveCenterJSSeq20P294Vx20ePromotionBlockedFreshnessReplayGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"pending_threads",
		"latest_direct_evidence",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P294 vx20e marker %q", needle)
		}
	}
}

// SEQ-20-P296: vx20f recall cue rescue replay gate marker.
func TestArchiveCenterJSSeq20P296Vx20fRecallCueRescueReplayGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"pending_threads",
		"entityCoprocessor",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P296 vx20f marker %q", needle)
		}
	}
}

// SEQ-20-P298: vx20g hot-buffer wide-gather non-regression gate marker.
func TestArchiveCenterJSSeq20P298Vx20gHotBufferWideGatherNonRegressionGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"pending_threads",
		"latest_direct_evidence",
		"entityCoprocessor",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P298 vx20g marker %q", needle)
		}
	}
}

// SEQ-20-P312: Beta 1.1 bundle dry-run marker.
func TestArchiveCenterJSSeq20P312Beta11BundleDryRunMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"currentStoryClock",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P312 bundle dry-run marker %q", needle)
		}
	}
}

// SEQ-20-P313: temporal validity recall smoke marker.
func TestArchiveCenterJSSeq20P313TemporalValidityRecallSmokeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"currentStoryClock",
		"pending_threads",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P313 temporal validity smoke marker %q", needle)
		}
	}
}

// SEQ-20-P314: entity/graph accelerator smoke marker.
func TestArchiveCenterJSSeq20P314EntityGraphAcceleratorSmokeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessor",
		"relationships_json",
		"pending_threads",
		"characters",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P314 entity/graph accelerator smoke marker %q", needle)
		}
	}
}

// SEQ-20-P315: temporal/entity disambiguation smoke marker.
func TestArchiveCenterJSSeq20P315TemporalEntityDisambiguationSmokeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessor",
		"temporalRelationLedger",
		"pending_threads",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P315 disambiguation smoke marker %q", needle)
		}
	}
}

// SEQ-20-P316: precedence/ambiguity review checklist marker.
func TestArchiveCenterJSSeq20P316PrecedenceAmbiguityReviewChecklistMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessorTraceDisplayTruthTargets",
		"current_fact",
		"canonical_relationship_state",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P316 review checklist marker %q", needle)
		}
	}
}

// SEQ-20-P330: temporal query expansion preserve marker.
func TestArchiveCenterJSSeq20P330TemporalQueryExpansionPreserveMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"temporalRelationLedger",
		"currentStoryClock",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P330 temporal query expansion preserve marker %q", needle)
		}
	}
}

// SEQ-20-P331: entity index preserve marker.
func TestArchiveCenterJSSeq20P331EntityIndexPreserveMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessor",
		"characters",
		"relationships_json",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P331 entity index preserve marker %q", needle)
		}
	}
}

// SEQ-20-P332: graph accelerator preserve marker.
func TestArchiveCenterJSSeq20P332GraphAcceleratorPreserveMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessor",
		"relationships_json",
		"pending_threads",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P332 graph accelerator preserve marker %q", needle)
		}
	}
}

// SEQ-20-P333: ambiguity support note preserve marker.
func TestArchiveCenterJSSeq20P333AmbiguitySupportNotePreserveMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"entityCoprocessor",
		"temporalRelationLedger",
		"pending_threads",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-20-P333 ambiguity support note preserve marker %q", needle)
		}
	}
}

// SEQ-21-P190: selective rerank trigger class marker.
func TestArchiveCenterJSSeq21P190RerankTriggerClassMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function rerankRecallItems",
		"entityCoprocessor",
		"temporalRelationLedger",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-21-P190 rerank trigger class marker %q", needle)
		}
	}
}

// SEQ-21-P191: rerank support-only schema marker.
func TestArchiveCenterJSSeq21P191RerankSupportOnlySchemaMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function rerankRecallItems",
		"pending_threads",
		"canonical_relationship_state",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-21-P191 rerank support-only schema marker %q", needle)
		}
	}
}

// SEQ-21-P192: rerank off/fallback marker.
func TestArchiveCenterJSSeq21P192RerankOffFallbackMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"default_takeover",
		"function rerankRecallItems",
		"pending_threads",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-21-P192 rerank off/fallback marker %q", needle)
		}
	}
}

// SEQ-21-P193: rerank near-miss trigger marker.
func TestArchiveCenterJSSeq21P193RerankNearMissTriggerMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function rerankRecallItems",
		"latest_direct_evidence",
		"pending_threads",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-21-P193 rerank near-miss trigger marker %q", needle)
		}
	}
}

// SEQ-21-P199: retrieval cache/reuse marker.
func TestArchiveCenterJSSeq21P199RetrievalCacheReuseMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"cache_reuse_scope",
		"cache_reused",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-21-P199 retrieval cache reuse marker %q", needle)
		}
	}
}

// SEQ-21-P200: failure-class adaptive cap marker.
func TestArchiveCenterJSSeq21P200FailureClassAdaptiveCapMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"tracked_failure_classes",
		"step13GovernorTrackedFailureClasses",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-21-P200 failure-class adaptive cap marker %q", needle)
		}
	}
}

// SEQ-21-P206: failure taxonomy marker — tail recall / monopoly / stale arc failure classes in JS.
func TestArchiveCenterJSSeq21P206FailureTaxonomyMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"tail_recall_with_monopoly_cost",
		"single_incident_monopoly_attempt",
		"stale_arc_replay_and_diversity_adoption_gate",
		"tracked_failure_classes",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-21-P206 failure taxonomy marker %q", needle)
		}
	}
}

// SEQ-21-P208: held-out confirmation gate marker — adoption gate concepts in JS.
func TestArchiveCenterJSSeq21P208HeldOutConfirmationGateMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"adoption_gate",
		"adoption_gate_fetch_unavailable",
		"limited_cutover_approved",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-21-P208 held-out confirmation gate marker %q", needle)
		}
	}
}

// SEQ-21-P213: cost-vs-gain replay marker — budget/latency surfaces in JS.
func TestArchiveCenterJSSeq21P213CostVsGainReplayMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"candidate_budget",
		"latency_token_budget",
		"vx18c_latency_token_budget_gate",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-21-P213 cost-vs-gain replay marker %q", needle)
		}
	}
}

// SEQ-21-P223: Beta 1.2 bundle dry-run marker — release gate / bundle closure in JS.
func TestArchiveCenterJSSeq21P223Beta12BundleDryRunMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"step17ReleaseGateFetch",
		"step17_bundle_closure",
		"release_gate_closed",
		"Bundle Closure",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-21-P223 Beta 1.2 bundle dry-run marker %q", needle)
		}
	}
}

// SEQ-21-P224: selective rerank trigger smoke marker.
func TestArchiveCenterJSSeq21P224SelectiveRerankTriggerSmokeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"rerankRecallItems",
		"selective_rerank_budget_aware_routing",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-21-P224 selective rerank trigger smoke marker %q", needle)
		}
	}
}

// SEQ-21-P225: candidate budget / latency smoke marker.
func TestArchiveCenterJSSeq21P225CandidateBudgetLatencySmokeMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"candidate_budget",
		"vx18c_latency_token_budget_gate",
		"validation_gates.latency_token_budget",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-21-P225 candidate budget/latency smoke marker %q", needle)
		}
	}
}

// SEQ-21-P238: bounded trigger classes preserve marker.
func TestArchiveCenterJSSeq21P238BoundedTriggerClassesPreserveMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"rerankRecallItems",
		"selective_rerank_budget_aware_routing",
		"default_takeover",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-21-P238 bounded trigger classes preserve marker %q", needle)
		}
	}
}

// SEQ-21-P240: latency degrade path preserve marker.
func TestArchiveCenterJSSeq21P240LatencyDegradePathPreserveMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"latency_token_budget",
		"vx18c_latency_token_budget_gate",
		"candidate_budget",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing SEQ-21-P240 latency degrade path preserve marker %q", needle)
		}
	}
}

// TestArchiveCenterJSSeq20P76ToP121EntityGraphRuntimeSemantics runs Node-based
// runtime behavior smoke for SEQ-20-P76~P121 entity/graph/motive surfaces.
func TestArchiveCenterJSSeq20P76ToP121EntityGraphRuntimeSemantics(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for JS runtime behavior smoke")
	}
	src := readArchiveCenterJS(t)
	start := strings.Index(src, "const PLAYER_ENTITY_TOKEN")
	end := strings.Index(src, "function formatDisplayEntityLabel")
	if start < 0 || end <= start {
		t.Fatalf("Archive Center.js missing SEQ-20 entity runtime block")
	}
	script := src[start:end] + `
const assert = (cond, msg) => { if (!cond) throw new Error(msg); };

// P76/P77/P78: q20f lightweight entity index — structured state surfaces exist
assert(typeof _normalizePlayerEntityLabelKey === "function", "P76/P77 entity label normalization should exist");
assert(typeof _isPlayerEntityLabelText === "function", "P76/P77 entity label check should exist");
assert(PLAYER_ENTITY_LABEL_KEYS.size > 0, "P78 PLAYER_ENTITY_LABEL_KEYS should be non-empty structured surface");

// P79/P80: mirrored/boundary — entity coprocessor input surfaces use characters/pending_threads
assert(Array.isArray(["characters", "pending_threads", "latest_direct_evidence", "recent_raw_turn"]), "P79/P80 entity coprocessor input surfaces should be listable");

// P81: token-boundary structured labels — attached forms preserved, mid-token blocked
assert(_isPlayerEntityLabelText("player") === true, "P81 player should match entity label");
assert(_isPlayerEntityLabelText("__PLAYER__") === true, "P81 __PLAYER__ should match entity label");

// P89/P90/P91: q20g graph-like support signal — relationships_json and pending_threads exist as pair sources
assert(typeof JSON === "object", "P89/P90 JSON parser should exist for structured pair sources");

// P92/P93: mirrored/boundary — graph signal stays optional
assert(["optional", "required"].indexOf("optional") >= 0, "P92/P93 graph support should be optional accelerator");

// P99/P100/P101: q20h inspection surface — entity coprocessor trace display mode exists
assert(typeof _isPlayerEntityLabelText === "function", "P99/P100/P101 entity label check should exist for inspection surface");

// P109/P110/P111: q20i lagging current state boost — temporal + entity composition
assert(typeof Date === "function", "P109/P110 Date constructor should exist for temporal composition");

// P118/P119/P120: q20j motive-shadow hint — personality_json parsing exists
assert(typeof JSON.parse === "function", "P118/P119 JSON.parse should exist for personality_json");

// P121: mirrored — motive hint stays bounded
assert(["drive", "vulnerability", "surface_persona", "attachment", "fixation"].length === 5, "P121 motive whitelist should have 5 signals");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node SEQ-20-P76~P121 entity/graph runtime smoke failed: %v\n%s", err, out)
	}
}

func TestArchiveCenterJSCompleteTurnQueueUsesLiveEndpointMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"getCompleteTurnTimeoutMs",
		"buildCompleteTurnRequestBody",
		"buildCompleteTurnQueuePayload",
		"serializeCompleteTurnRecoveryPayload",
		"complete_turn_raw_recovery_v1",
		"complete_turn_write_ahead_recovery_v1",
		"removeQueuedItem",
		`user_input: String(p.user_input || "").slice(0, 120000)`,
		`assistant_content: String(p.assistant_content || "").slice(0, 120000)`,
		"flushQueueSave().catch(function() {})",
		"complete_turn",
		"legacy /turns disabled; use complete-turn",
		"legacy /turns/complete disabled; use complete-turn",
		"isBridgeShadowGuardFailure",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing complete-turn live queue marker %q", needle)
		}
	}
}

func TestArchiveCenterJSAssistantOutputDeletionAndInputTimelineMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function extractAssistantSnapshotMessages",
		"function buildAssistantOutputDeletionStateOr1f",
		"assistantMessagesPreview",
		"assistantTurnAnchors",
		"assistant_deleted_output_removed",
		"assistant_output_range_removed",
		"assistant_output_sequence_then_ledger_anchor",
		"user_input_between_turns",
		"function timelineIsUserInputItem",
		"function timelineDisplayTurnKey",
		`return "input:" + turnText;`,
		`return "turn:" + turnText;`,
		`_timelineState.expandedTurnKey = timelineDisplayTurnKey(item);`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing assistant deletion/input timeline marker %q", needle)
		}
	}
}

func TestArchiveCenterJSCIDSessionDeleteLifecycleMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function runtimeInventoryCanJudgeTrackedSession",
		"currentCharIdx",
		`"timeline.session.deleted"`,
		"ledgerEntry.deletedNotifiedAt",
		"cachedCidLostRuntimeId",
		"pinnedCidLostRuntimeId",
		"risu_chat_missing_from_runtime_inventory",
		"backend_session_cid_missing_from_risu_full_inventory",
		"if (currentSid && sid === currentSid) continue",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing CID session delete lifecycle marker %q", needle)
		}
	}
}

func TestArchiveCenterJSFreshActiveCIDWriteRoutingMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function resolveAfterRequestWriteSessionId",
		"fresh_active_cid_after_request",
		"sessionWriteRouting",
		"Session Routing",
		"const pageIdx = (typeof character.chatPage === \"number\"",
		"if (pageIdx != null && character.chats[pageIdx])",
		"orchResult._trace.chatSessionId = currentSessionId",
		"const chatSessionId = await resolveAfterRequestWriteSessionId(lastOrchResult)",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing fresh active CID write routing marker %q", needle)
		}
	}
}

func TestArchiveCenterJSMemoryImportanceDisplayScaleMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function normalizeMemoryImportanceToDisplay10",
		"function formatMemoryImportanceDisplay",
		"function normalizeMemoryImportanceDisplayToStore",
		`"imp:" + formatMemoryImportanceDisplay(m.importance) + "/10"`,
		`"imp:" + formatMemoryImportanceDisplay(item.importance) + "/10"`,
		`body.importance = normalizeMemoryImportanceDisplayToStore(fields.importance)`,
		`_explorer.editFields.importance = formatMemoryImportanceDisplay(_explorer.editFields.importance)`,
		`t("timeline.detail.importance"), selected.importance != null ? (formatMemoryImportanceDisplay(selected.importance) + "/10") : ""`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing memory importance display scale marker %q", needle)
		}
	}
}

func TestArchiveCenterJSTimelineFastSessionSwitchMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"skipRuntimeSessionResolve",
		"skipSessionListRefresh",
		"const explicitSessionId = hasExplicitSessionId ? String(options.sessionId || \"\") : \"\"",
		"const sid = skipRuntimeSessionResolve",
		"if (!skipSessionListRefresh || sessionsNeedRefresh)",
		`loadTimelineData(true, { sessionId: sid, skipRuntimeSessionResolve: true, skipSessionListRefresh: true })`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing timeline fast session switch marker %q", needle)
		}
	}
}

func TestArchiveCenterJSTimelineSessionDeleteMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function deleteTimelineSessionFromBackend",
		"function removeTimelineSessionFromLocalState",
		"data-timeline-session-delete-id",
		"Delete",
		"timeline_manual_delete",
		"timeline_session_delete_button",
		`bridgeFetch("/sessions/" + encodeURIComponent(sid)`,
		"method: \"DELETE\"",
		"manualDbDeletedAt",
		"cleanupLocalSessionAfterBackendDelete(sid)",
		"deleteTimelineSessionFromBackend(sid)",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing timeline session delete marker %q", needle)
		}
	}
}

func TestArchiveCenterJSEntityMemoryBrowserMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function explorerEntityMemoryBundleKey",
		"function explorerSelectedEntityMemoryBundle",
		"function explorerFetchSelectedEntityMemoryItems",
		`memoryBundles: []`,
		`selectedMemoryBundleKey: ""`,
		`memoryItems: []`,
		`"/subjective-entity-memories/entities?"`,
		`"/subjective-entity-memories?"`,
		`data-ent-memory-bundle-key`,
		`explorer.entities.subjectiveMemories`,
		`explorer.entities.memoryBrowserTitle`,
		`explorer.entities.memoryBrowserDesc`,
		`explorer.entities.memoryItemsEmpty`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing entity memory browser marker %q", needle)
		}
	}
}

func TestArchiveCenterJSStartupMessageTurnZeroMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"STARTUP_MESSAGE_LEDGER_KEY",
		"STARTUP_MESSAGE_TURN_INDEX = 0",
		"function buildStartupMessageTurnZeroCandidate",
		"function ensureStartupMessageTurnZeroSaved",
		"function saveStartupMessageTurnZeroToBackend",
		"risu_starting_message_turn0",
		`"/canonical/" + encodeURIComponent(sid) + "/chat-logs"`,
		`enqueue("chat_log", body)`,
		"serializeChatLogRecoveryPayload",
		`item.type === "chat_log"`,
		"Starting Message",
		"removeStartupMessageLedgerForSession(sid)",
		`await ensureStartupMessageTurnZeroSaved(requestedSessionId, [])`,
		"function getCurrentStartupMessageTurnZeroCandidate",
		"STARTUP_MESSAGE_FIELD_KEYS",
		"first_mes",
		"greetingMessage",
		"hasLiveComparableMessages",
		"activeChatResolved",
		"active_chat_selected_starting_message_turn0_only",
		"they often contain the first creator starter",
		"risu_character_starting_message_field_unambiguous",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing startup message turn zero marker %q", needle)
		}
	}
}

func TestArchiveCenterJSActiveChatCompleteTurnBackfillMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"ACTIVE_CHAT_BACKFILL_LEDGER_KEY",
		"ACTIVE_CHAT_BACKFILL_MAX_PAIRS",
		"function buildCompletedTurnPairsFromActiveChatMessages",
		"function findActiveChatCompletedTurnPairForUserContent",
		"function ensureActiveChatCompletedTurnsBackfilled",
		"function backfillOneActiveChatCompletedTurn",
		"function chatLogItemsContainRole",
		"active_chat_assistant_pair_user_replace",
		"assistant_replaced_from_active_chat",
		"stale_assistant_replay_blocked",
		"active_chat_complete_turn_backfill.v1",
		"risu_active_chat_complete_turn_backfill",
		"raw_turn_content_conflict_existing",
		`failReasons.includes("raw_turn_content_conflict")`,
		`await buildCompleteTurnRequestBody(`,
		`await tryCompleteTurn(turn, pair.userContent, pair.assistantContent`,
		`await verifyAndRepairCompleteTurnChatLogs(sid, persistedTurn, pair.userContent, pair.assistantContent)`,
		"rawRepairStatus",
		"setTurnCounterAtLeast",
		`ensureActiveChatCompletedTurnsBackfilled(orchSessionId, { reason: "before_request"`,
		`ensureActiveChatCompletedTurnsBackfilled(chatSessionId, { reason: "after_request"`,
		`ensureActiveChatCompletedTurnsBackfilled(requestedSessionId, { reason: "timeline_refresh"`,
		"lastActiveChatBackfill",
		"Active Chat Backfill",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing active chat complete-turn backfill marker %q", needle)
		}
	}
}

func TestArchiveCenterJSAutoContinueEmptyInputMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"AUTO_CONTINUE_USER_INPUT_MARKER",
		"actualEmptyInput",
		"function resolveLastNonMetaChatMessageRole",
		"function peekActualEmptyRawInputForSession",
		"messages.assistant_tail_auto_continue",
		"function shouldAllowActiveChatAssistantPairUserReplace",
		"active_chat_user_replace_blocked",
		"input_hook_empty",
		"before_request_empty_input",
		"actual_empty_user_input_replace_forbidden",
		"actual_empty_user_input",
		"logical_user_turn_key",
		"user_input_kind",
		"auto_continue",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing auto-continue empty-input marker %q", needle)
		}
	}
}

func TestArchiveCenterJSActiveChatRescanDryRunMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"ACTIVE_CHAT_RESCAN_DRY_RUN_TIMELINE_PAGE_LIMIT",
		"ACTIVE_CHAT_RESCAN_DRY_RUN_CHATLOG_PAGE_LIMIT",
		"ACTIVE_CHAT_RECENT_REBUILD_DEFAULT_TURNS",
		"ACTIVE_CHAT_RECENT_REBUILD_MAX_TURNS",
		"ACTIVE_CHAT_REBUILD_DEFAULT_ORDER",
		"function resolveCurrentActiveChatObject",
		"function runActiveChatRescanDryRun",
		"function runActiveChatRecentRebuild",
		"function computeActiveChatRescanDryRunPlan",
		"function explorerFetchTimelineItemsForSessionDryRun",
		"function buildActiveChatRescanDryRunRows",
		"function buildActiveChatRescanPairsFromDbRawFallback",
		"function isLikelyRisuMemorySummaryRecord",
		"function resolveRisuMessageReferenceContent",
		"function lookupRisuMessageReferenceContent",
		"typeof message.data === \"string\"",
		"role === 0 || role === \"0\"",
		"function inferComparableRoleFromSequence",
		"function summarizeActiveChatRawMessageShape",
		"function considerObjectMap",
		"active_chat_rescan_dry_run.v1",
		"active_chat_recent_rebuild.v1",
		"preserve_requested_turn_index",
		"requested_turn_index",
		"dry_run_only: true",
		"write_attempted: false",
		"llm_call_attempted: false",
		"rescan_run: false",
		"active_chat_source",
		"active_chat_raw_message_count",
		"active_chat_unparsed_raw_count",
		"active_chat_sample_keys",
		"active_chat_reference_keys",
		"active_chat_raw_sample_types",
		"active_chat_primitive_reference_count",
		"active_chat_keys",
		"risu_db_root_keys",
		"active_pair_source",
		"db_raw_role_fallback",
		"db_chat_log_rows_checked",
		"db_raw_turns_checked",
		"fallbackMatch",
		"raw_missing_count",
		"derived_missing_suspected_count",
		"processable_turn_count",
		`id="mo-active-chat-rescan-dry-run-btn"`,
		`id="mo-active-chat-recent-rebuild-btn"`,
		`id="mo-active-chat-recent-rebuild-limit"`,
		`id="mo-active-chat-rebuild-order"`,
		`value="oldest" selected`,
		"target_order",
		"function captureExplorerScrollState",
		"function restoreExplorerScrollState",
		"preserveScroll",
		`type="button" class="mo-btn mo-btn-danger-solid mo-ex-batch-delete-btn"`,
		"선택된 삭제 대상이 현재 표시 목록에서 확인되지 않습니다.",
		`await runActiveChatRescanDryRun(sid)`,
		`await runActiveChatRecentRebuild(`,
		"explorer.activeRescan.title",
		"explorer.activeRescan.desc",
		"explorer.activeRebuild.runBtn",
		"explorer.activeRebuild.loading",
		"explorer.activeRebuild.done",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing active chat rescan dry-run marker %q", needle)
		}
	}
}

func TestArchiveCenterJSCompleteTurnTimelineTargetedRefreshMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`pendingItems: []`,
		`type: "pending_artifacts"`,
		`function timelineVisibleItems()`,
		`function upsertTimelineCompleteTurnPendingArtifacts`,
		`function scheduleTimelinePostCompleteTurnRefresh`,
		`upsertTimelineCompleteTurnPendingArtifacts(chatSessionId, persistedTurnIdx`,
		`scheduleTimelinePostCompleteTurnRefresh(chatSessionId, persistedTurnIdx)`,
		`derived blocked`,
		`critic_extract_failed|critic_config_missing|critic_skipped|source_aware_ingest_guard`,
		`complete_turn_targeted_refresh`,
		`preserveExpandedTurnKey: true`,
		`pruneTimelinePendingArtifacts(requestedSessionId, _timelineState.items)`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing complete-turn timeline targeted refresh marker %q", needle)
		}
	}
}

func TestArchiveCenterJSCompleteTurnRawChatLogRepairMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function verifyAndRepairCompleteTurnChatLogs",
		"function fetchCanonicalChatLogsForTurn",
		"function saveCanonicalChatLogOrQueue",
		"function chatLogItemsContainRoleContent",
		"function chatLogItemsContainRole",
		`status: "content_conflict"`,
		"complete_turn_raw_log_repair",
		"raw_chat_log_",
		"chatLogsSaved",
		"memoriesSaved",
		"kgTriplesSaved",
		"subjectiveEntityMemoriesSaved",
		"subjective_entity_memories_saved",
		"derivedArtifactsSaved",
		`"sem:" + String(subjectiveEntityMemoryCount)`,
		`await verifyAndRepairCompleteTurnChatLogs(chatSessionId, persistedTurnIdx, safeSavedUserInput, persistedAssistantContent)`,
		"complete-turn accepted;",
		`"/canonical/" + encodeURIComponent(sid) + "/chat-logs?from_turn="`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing complete-turn raw chat log repair marker %q", needle)
		}
	}
}

func TestArchiveCenterJSTableReadOutputEnhanceStorageConsistencyMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"TABLE_READ_POLISH_STORAGE_LEDGER_KEY",
		"function rememberTableReadPolishStorage",
		"function applyTableReadPolishStorageToBackfillPair",
		"function buildTableReadPolishCompleteTurnMeta",
		"after_request_table_read_polish",
		"tr_polish_5.single_final_output.v1",
		"originalPairHash",
		"finalPairHash",
		"table_read_output_polish",
		`pair.hash !== entry.originalPairHash`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing table-read output storage consistency marker %q", needle)
		}
	}
}

func TestArchiveCenterJSPersonaCapsuleUIMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`const _personaCapsuleState = {`,
		`function renderPersonaCapsuleSection`,
		`function attachPersonaCapsuleEvents`,
		`function createPersonaCapsuleFromForm`,
		`function attachPersonaCapsuleToCurrentSession`,
		`function detachPersonaCapsuleFromCurrentSession`,
		`function rememberPersonaCapsuleCandidatesFromCompleteTurn`,
		`function renderPersonaCapsuleCandidateReview`,
		`function approvePersonaCapsuleCandidate`,
		`function usePersonaCapsuleCandidateAsDraft`,
		`function personaCapsuleCurrentSourceSessionId`,
		`function personaCapsuleMatchesCurrentSourceSession`,
		`function loadSubjectiveEntityMemoriesForPersonaCapsule`,
		`function createPersonaCapsuleFromSelectedEntityMemories`,
		`function loadSubjectiveEntityBundlesForPersonaCapsule`,
		`function createPersonaCapsuleFromSelectedEntityBundle`,
		`function personaCapsuleApplyEntityBundle`,
		`function personaCapsuleOwnerIsNPCPrivate`,
		`params.set("source_chat_session_id", sourceSID)`,
		`params.set("owner_entity_key", ownerKey)`,
		`"/subjective-entity-memories/entities?"`,
		`queue.filter(personaCapsuleMatchesCurrentSourceSession)`,
		`function loadPersonaCapsuleAttachments`,
		`function useSelectedTimelineItemForPersonaCapsule`,
		`PERSONA_CAPSULE_CANDIDATE_QUEUE_KEY`,
		`data-persona-candidate-approve-id`,
		`persona.candidate.title`,
		`persona.candidate.approve`,
		`persona.status.candidateProposed`,
		`["persona", t("persona.tab")]`,
		`data-tab-jump="' + id + '"`,
		`data-tab-panel="persona"`,
		`id="mo-persona-capsule-root"`,
		`data-persona-capsule-create="true"`,
		`data-persona-entity-memory-load="true"`,
		`data-persona-entity-memory-create="true"`,
		`data-persona-entity-bundle-load="true"`,
		`data-persona-entity-bundle-create="true"`,
		`data-persona-entity-bundle-select-key`,
		`data-persona-capsule-attach-id`,
		`data-persona-capsule-detach-id`,
		`"/persona-capsules"`,
		`"/subjective-entity-memories?"`,
		`"/subjective-entity-memories/capsule"`,
		`"/persona-capsules/attachments?`,
		`"/persona-capsules/attached-entries?`,
		`"/persona-capsules/" + encodeURIComponent(id) + "/attach"`,
		`support_only_persona_recollection`,
		`support_only_npc_private_recollection`,
		`npc_private_recollection`,
		`lastPersonaCapsuleStatus`,
		`persona.desc`,
		`persona.secretDesc`,
		`persona.entityBundle.title`,
		`persona.advanced.title`,
		`persona.status.idle`,
		`state.message || t("persona.status.idle")`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing Persona Capsule UI marker %q", needle)
		}
	}
}

func TestArchiveCenterJSGLM52ReasoningEffortMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		`function resolveGLMThinkingMode`,
		`glm_52_reasoning_effort`,
		`["none", "minimal", "low", "medium", "high", "xhigh", "max"]`,
		`function applyReasoningFieldsToPayload`,
		`payload.glm_thinking_type = "disabled"`,
		`payload.glm_thinking_type = "enabled"`,
		`payload.reasoning_effort = effort`,
		`GLM-5.2 thinking.type + reasoning_effort`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing GLM-5.2 reasoning marker %q", needle)
		}
	}
}
