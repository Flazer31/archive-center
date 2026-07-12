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
		`body: { auto_review: true, client_meta:`,
		`/library`,
		`data-reference-panel="library"`,
		`data-reference-panel="import"`,
		`data-reference-library-view="all"`,
		`data-reference-library-view="timeline"`,
		`data-reference-library-view="entities"`,
		`data-reference-library-view="claims"`,
		`생성된 자료`,
		`평론가 자동 생성`,
	} {
		if !strings.Contains(src, marker) {
			t.Fatalf("Archive Center.js missing reference library marker %q", marker)
		}
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
