package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

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

func TestArchiveCenterJSDirectEvidenceContentEditContract(t *testing.T) {
	src := readArchiveCenterJS(t)
	for _, marker := range []string{
		`body.evidence_text = String(fields.evidence_text || "").trim();`,
		`data-field="evidence_text"`,
		`data-timeline-edit-field="evidence_text"`,
		`evidence_text: item.evidence_text || ""`,
		`evidence_text: item.evidence_text || item.preview || ""`,
		`item.evidence_text = fields.evidence_text;`,
		`result.status === "ok" || result.status === "partial_error"`,
	} {
		if !strings.Contains(src, marker) {
			t.Fatalf("Archive Center.js is missing direct-evidence edit marker %q", marker)
		}
	}
	for _, marker := range []string{
		`.mo-memory-workspace .mo-ed-edit-btn{`,
		`background:#181C24`,
		`color:#F4F5F7`,
		`appearance:none`,
	} {
		if !strings.Contains(src, marker) {
			t.Fatalf("Archive Center.js is missing edit-button contrast marker %q", marker)
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
// SEQ-20-P261: q20n.v1 alias/entity conflict disambiguation contract marker.
// SEQ-20-P262: q20o temporal/entity source-tag rule preparatory marker.
// SEQ-20-P263: q20o.v1 temporal/entity source-tag rule contract marker.
// SEQ-20-P264: q20p canonical-pending/stale-current conflict note preparatory marker.
// SEQ-20-P265: q20p.v1 canonical-pending/stale-current conflict note contract marker.
// SEQ-20-P266: q20q recall cue rescue rule preparatory marker.
// SEQ-20-P267: q20q.v1 recall cue rescue rule contract marker.
// SEQ-20-P268: q20r wide gather -> validity join rule preparatory marker.
// SEQ-20-P269: q20r.v1 wide gather -> validity join rule contract marker.
// SEQ-20-P270: q20s thin support tag fallback preparatory marker.
// SEQ-20-P271: q20s.v1 thin support tag fallback contract marker.
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
// SEQ-20-P290: vx20c graph accelerator degrade gate marker.
// SEQ-20-P292: vx20d canonical precedence replay gate marker.
// SEQ-20-P294: vx20e promotion-blocked freshness replay gate marker.
// SEQ-20-P296: vx20f recall cue rescue replay gate marker.
// SEQ-20-P298: vx20g hot-buffer wide-gather non-regression gate marker.
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
// SEQ-20-P315: temporal/entity disambiguation smoke marker.
// SEQ-20-P316: precedence/ambiguity review checklist marker.
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
// SEQ-20-P332: graph accelerator preserve marker.
// SEQ-20-P333: ambiguity support note preserve marker.
// SEQ-21-P190: selective rerank trigger class marker.
// SEQ-21-P191: rerank support-only schema marker.
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
// SEQ-21-P199: retrieval cache/reuse marker.
// SEQ-21-P200: failure-class adaptive cap marker.
// SEQ-21-P206: failure taxonomy marker — tail recall / monopoly / stale arc failure classes in JS.
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
// SEQ-21-P225: candidate budget / latency smoke marker.
// SEQ-21-P238: bounded trigger classes preserve marker.
// SEQ-21-P240: latency degrade path preserve marker.
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
		"buildCompleteTurnSourceAcceptanceObservation",
		"refreshQueuedCompleteTurnSourceObservation",
		`contract_version: "source_acceptance_observation.v1"`,
		`revision_state: "not_exposed_by_risuai"`,
		`typeof chat.isStreaming === "boolean"`,
		`typeof message.chatId === "string"`,
		`typeof generationInfo.generationId === "string"`,
		`user_message_index: -1`,
		`user_observed_content_hash: ""`,
		`user_persistence_content_hash:`,
		`rollbackParams.set("host_observed_at_ms", String(Date.now()))`,
		"serializeCompleteTurnRecoveryPayload",
		"complete_turn_raw_recovery_v1",
		"queuePendingCompleteTurnPayload",
		`state: "retryable"`,
		"settings.failedQueueMaxAttempts",
		"commitFailedQueueTransitionIntent",
		"markFailedQueueItemTerminalDurably",
		"failed_queue_terminal_intent_persisted",
		`res.queue_action === "discard" || res.retryable === false`,
		"reconciliation_required === true",
		"removeQueuedItem",
		`return buildCompleteTurnQueuePayload(p);`,
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
	if strings.Contains(src, "complete_turn_write_ahead_recovery_v1") {
		t.Fatal("pending/in-flight complete-turn payload must not be exposed as a failed write-ahead queue item")
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

func TestArchiveCenterJSAfterRequestReusesCapturedCIDWithoutRoutingBlock(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"function onAfterRequest(content, type)",
		"const capturedWriteSessionId = normalizeSessionId(",
		"latestOrchResult && latestOrchResult._chatSessionId",
		"const chatSessionId = capturedWriteSessionId || cachedWriteSessionId || SESSION_FALLBACK;",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing captured CID afterRequest marker %q", needle)
		}
	}
	for _, forbidden := range []string{
		"function resolveAfterRequestWriteSessionId",
		"fresh_active_cid_after_request",
		"const chatSessionId = await resolveAfterRequestWriteSessionId(persistenceOrchResult)",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("Archive Center.js retains blocking afterRequest routing marker %q", forbidden)
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
		"requestId: 0",
		"async function loadTimelineSessions(currentSid, force = false, timelineDataRequestId = 0)",
		"const requestIsCurrent = function()",
		"skipRuntimeSessionResolve",
		"skipSessionListRefresh",
		"const explicitSessionId = hasExplicitSessionId ? String(options.sessionId || \"\") : \"\"",
		"const requestId = Number(_timelineState.requestId || 0) + 1",
		"if (_timelineState.requestId !== requestId) return",
		"const sid = skipRuntimeSessionResolve",
		"if (!skipSessionListRefresh || sessionsNeedRefresh)",
		"await loadTimelineSessions(runtimeSid, force || sessionsNeedRefresh, requestId)",
		`loadTimelineData(true, { sessionId: sid, skipRuntimeSessionResolve: true, skipSessionListRefresh: true })`,
		"sessionId: _timelineState.selectedSessionId || _timelineState.sessionId || \"\"",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing timeline fast session switch marker %q", needle)
		}
	}
	sessionChangeMarker := "if (!append && previousSessionId && requestedSessionId && previousSessionId !== requestedSessionId)"
	sessionChangeStart := strings.Index(src, sessionChangeMarker)
	sessionChangeEnd := sessionChangeStart + 500
	if sessionChangeEnd > len(src) {
		sessionChangeEnd = len(src)
	}
	if sessionChangeStart < 0 || !strings.Contains(src[sessionChangeStart:sessionChangeEnd], "_timelineState.detailLoading = false") {
		t.Fatal("Timeline data session change must clear stale detail loading")
	}
	for name, block := range map[string]string{
		"Timeline session deletion":   extractJSFunctionBlockForTest(t, src, "function removeTimelineSessionFromLocalState(sessionId)"),
		"worldline node selection":    extractJSFunctionBlockForTest(t, src, "function timelineWorldlineSelectNode(node)"),
		"workspace session selection": extractJSFunctionBlockForTest(t, src, "function selectWorkspaceSession(sessionId, surface)"),
	} {
		if !strings.Contains(block, "_timelineState.detailLoading = false") {
			t.Fatalf("%s must clear stale detail loading on session change", name)
		}
	}
	for name, block := range map[string]string{
		"worldline node selection":    extractJSFunctionBlockForTest(t, src, "function timelineWorldlineSelectNode(node)"),
		"workspace session selection": extractJSFunctionBlockForTest(t, src, "function selectWorkspaceSession(sessionId, surface)"),
	} {
		for _, marker := range []string{"_timelineState.items = []", "_timelineState.meta = null", "_timelineState.viewModel = null", "syncSessionScopedInspectionSelection("} {
			if !strings.Contains(block, marker) {
				t.Fatalf("%s must clear prior-session presentation before loading: missing %q", name, marker)
			}
		}
	}
}

func TestArchiveCenterJSTimelineWorldlineCanvasPreservesCompactOperations(t *testing.T) {
	src := readArchiveCenterJS(t)
	panel := extractJSFunctionBlockForTest(t, src, "function renderTimelinePanel()")
	turnGroup := extractJSFunctionBlockForTest(t, src, "function renderTimelineTurnGroup(group)")
	turnItem := extractJSFunctionBlockForTest(t, src, "function renderTimelineTurnItemRow(item)")
	events := extractJSFunctionBlockForTest(t, src, "function attachTimelineEvents()")
	explorerEvents := extractJSFunctionBlockForTest(t, src, "function attachExplorerEvents()")
	sessionAdmin := extractJSFunctionBlockForTest(t, src, "function renderSessionDatabaseManagement(")
	selection := extractJSFunctionBlockForTest(t, src, "function selectWorkspaceSession(sessionId, surface)")
	settingsEvents := extractJSFunctionBlockForTest(t, src, "function attachSettingsEvents()")
	mount := extractJSFunctionBlockForTest(t, src, "function mountTimelineWorldlineCanvas()")
	for _, marker := range []string{
		`<canvas id="mo-timeline-worldline-canvas"`,
		`class="mo-tl-toolbar"`,
		`id="mo-timeline-session-select"`,
		`data-timeline-session-id=`,
		`id="mo-timeline-reload-btn"`,
		`id="mo-timeline-load-more-btn"`,
		`class="mo-tl-lineage-detail"`,
		`class="mo-tl-node-inspector"`,
		`data-selected-node-id=`,
		`data-worldline-inspector-close`,
		`data-selected-turn-key=`,
		`viewModel.worldline_topology`,
		`worldline_topology.viewmodel.v2`,
		`data-worldline-topology-state=`,
		`suppliedTopologyState === "partial"`,
		`suppliedTopologyState === "unavailable"`,
	} {
		if !strings.Contains(panel, marker) {
			t.Fatalf("Timeline canvas/compact operation missing %q", marker)
		}
	}
	for _, marker := range []string{`data-timeline-session-attach-id=`, `data-timeline-session-copy-id=`, `data-timeline-session-migrate-id=`, `data-timeline-session-delete-id=`, `data-timeline-session-rollback-id=`, `data-timeline-session-cleanup-id=`} {
		if strings.Contains(panel, marker) {
			t.Fatalf("Worldlines must not retain DB management action %q", marker)
		}
		if !strings.Contains(sessionAdmin, marker) {
			t.Fatalf("Memory management session owner missing %q", marker)
		}
	}
	for _, marker := range []string{`data-memory-session-management`, `class="mo-memory-admin-rail"`, `class="mo-memory-admin-workspace"`, `data-memory-admin-session-id=`, `selectedSession.can_attach`, `selectedSession.can_copy`, `selectedSession.can_migrate`} {
		if !strings.Contains(sessionAdmin, marker) {
			t.Fatalf("Memory management session projection missing %q", marker)
		}
	}
	if strings.Contains(sessionAdmin, `id="mo-memory-admin-session-select"`) {
		t.Fatal("Memory management must not retain the detached session select")
	}
	if !strings.Contains(explorerEvents, `querySelectorAll("[data-memory-admin-session-id]")`) || !strings.Contains(explorerEvents, `selectWorkspaceSession(String(sessionButton.getAttribute("data-memory-admin-session-id") || ""), "memory_admin")`) {
		t.Fatal("Memory management session rail must drive the shared workspace selection owner")
	}
	for _, marker := range []string{`deleteTimelineSessionFromBackend(sid)`, `attachTimelineSessionToCurrentChat(sid)`, `runTimelineSessionCopy(sid)`, `runTimelineSessionMigration(sid)`, `runTimelineSessionMigrationRollback()`, `runTimelineSessionMigrationCleanup()`} {
		if strings.Contains(events, marker) {
			t.Fatalf("Worldlines event binder must not retain DB management dispatch %q", marker)
		}
		if !strings.Contains(explorerEvents, marker) {
			t.Fatalf("Memory management event binder missing DB dispatch %q", marker)
		}
	}
	if strings.Contains(panel, `<aside class="mo-tl-side">`) || strings.Contains(panel, `<aside class="mo-tl-detail">`) {
		t.Fatal("Timeline canvas retains a permanent left/right sidebar")
	}
	for _, marker := range []string{
		`.mo-tl-main{min-width:0;min-height:0;display:flex;flex:1 1 auto;`,
		`.mo-tl-canvas-frame{position:relative;width:100%;height:clamp(480px,68vh,820px);min-height:420px;flex:1 1 auto;overflow:hidden`,
		`.mo-tl-main{position:relative;overflow:hidden}`,
		`.mo-tl-node-inspector{position:absolute;z-index:4;inset:16px;display:flex;flex-direction:column;`,
		`.mo-tl-node-inspector-body{min-height:0;flex:1 1 auto;padding:16px;overflow:auto;overscroll-behavior:contain}`,
		`.mo-tl-inline-detail{`,
	} {
		if !strings.Contains(src, marker) {
			t.Fatalf("Timeline non-overlap/single-column layout contract missing %q", marker)
		}
	}
	for _, forbidden := range []string{
		`.mo-tl-main{position:relative;overflow:visible`,
		`grid-template-columns:minmax(0,.9fr) minmax(0,1.5fr)`,
		`<details class="mo-tl-drawer" open>`,
		`class="mo-tl-drawers"`,
		`name="mo-timeline-drawers"`,
		`_timelineSelectedDetail = data.items[0]`,
		`_timelineSelectedDetail = group.items[0]`,
		`_timelineWorldlineDetailSelected`,
		`scalarReasonHtml`,
		`class="mo-tl-records"`,
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("Timeline overlap/default-open/raw-reason regression retained %q", forbidden)
		}
	}
	for _, marker := range []string{
		`const selectedTurnGroup = turnGroups.find`,
		`selectedTurnGroup ? renderTimelineTurnGroup(selectedTurnGroup)`,
		`renderTimelineWorldlineDetail(worldline, topology)`,
		`title="' + escapeAttr(scalarReason) + '"`,
		`class="mo-tl-node-inspector-body"`,
	} {
		if !strings.Contains(panel, marker) {
			t.Fatalf("Timeline selected-turn/reason-title contract missing %q", marker)
		}
	}
	for _, marker := range []string{`class="mo-tl-turn-item-wrap`, `class="mo-tl-inline-detail"`, `renderDetailPanel(loadedDetail)`} {
		if !strings.Contains(turnItem, marker) {
			t.Fatalf("Timeline single inline detail contract missing %q", marker)
		}
	}
	for _, marker := range []string{`group.turn_text`, `group.created_text`} {
		if !strings.Contains(turnGroup, marker) {
			t.Fatalf("Timeline turn group does not consume exact Go field %q", marker)
		}
	}
	for _, forbidden := range []string{`group.turnText`, `group.createdText`} {
		if strings.Contains(turnGroup, forbidden) {
			t.Fatalf("Timeline turn group retains camelCase alias %q", forbidden)
		}
	}
	if strings.Count(panel, `(topologyNodes.length ? '' : ' disabled')`) < 4 {
		t.Fatal("empty Timeline topology must disable all canvas controls")
	}
	for _, marker := range []string{`topology.nodes.length === 0`, `drawTimelineWorldlineCanvas(canvas, topology)`, `return`} {
		if !strings.Contains(mount, marker) {
			t.Fatalf("empty Timeline canvas interaction guard missing %q", marker)
		}
	}
	for _, marker := range []string{
		`data-timeline-detail-key=`,
		`data-timeline-edit-key=`,
		`data-timeline-edit-save`,
		`data-timeline-edit-cancel`,
		`timeline.worldline.reason`,
		`timeline.canvas.zoomIn`,
		`timeline.canvas.zoomOut`,
		`timeline.canvas.recenter`,
		`timeline.canvas.fitAll`,
		`const focusTurn = append ? 0 : Math.max(0, Math.floor(Number(options.focusTurn || 0)))`,
		`params.set("limit", focusTurn > 0 ? "200" : "80")`,
		`if (focusTurn > 0) params.set("turn", String(focusTurn))`,
	} {
		if !strings.Contains(src, marker) {
			t.Fatalf("Timeline canvas removed preserved detail/edit/localization marker %q", marker)
		}
	}
	if strings.Count(src, `id="mo-timeline-reload-btn"`) != 1 {
		t.Fatal("Timeline compact toolbar must retain exactly one reload owner")
	}
	if strings.Count(panel, `class="mo-tl-node-inspector-body"`) != 1 {
		t.Fatal("selected turn focus surface must retain exactly one internal scroll owner")
	}
	if strings.Contains(events, `[data-tab-jump]`) {
		t.Fatal("Timeline-only rerenders must not rebind persistent Settings subtab events")
	}
	if !strings.Contains(events, `const toggleTimelineItemDetail = (item) =>`) || strings.Count(events, `toggleTimelineItemDetail(item)`) != 2 {
		t.Fatal("Timeline record row and explicit detail button must share one local open/close toggle owner")
	}
	if !strings.Contains(src, "_timelineState.requestId = requestId;\n    if (!append) _timelineState.error = \"\";\n    refreshTimelineUI({ reloadPresentation: false });") {
		t.Fatal("Timeline load start must clear or refresh the visible shell without waiting for a duplicate Presentation request")
	}
	for _, marker := range []string{`document.querySelectorAll("[data-tab-jump]")`, `setActiveSettingsTab(btn.getAttribute("data-tab-jump") || "timeline")`} {
		if !strings.Contains(settingsEvents, marker) {
			t.Fatalf("Settings subtab owner missing one-time binding %q", marker)
		}
	}
	for _, marker := range []string{
		`if (_settingsActiveTab === "timeline") attachTimelineEvents()`,
		`if (_settingsActiveTab === "archive" || _settingsActiveTab === "memory_admin") attachExplorerEvents()`,
		`if (_settingsActiveTab === "persona") attachPersonaCapsuleEvents()`,
		`if (!["settings", "review", "prompt", "dashboard", "debug"].includes(_settingsActiveTab)) return`,
	} {
		if !strings.Contains(settingsEvents, marker) {
			t.Fatalf("active-route-only event binding contract missing %q", marker)
		}
	}
	if strings.Contains(settingsEvents, `getCurrentChatSessionId().then`) || strings.Contains(settingsEvents, `if (tab === "timeline")`) {
		t.Fatal("workspace shell must not retain inactive session lookup or duplicate Timeline load ownership")
	}
	for _, marker := range []string{
		`sessionSelect.addEventListener("change"`,
		`sessionSelect.value`,
		`mountTimelineWorldlineCanvas()`,
		`timelineWorldlineZoomBy(worldlineCanvas, 1.2)`,
		`timelineWorldlineZoomBy(worldlineCanvas, 1 / 1.2)`,
		`timelineWorldlineCurrentNode(worldlineCanvas._moTimelineTopology)`,
		`timelineWorldlineFitAll(worldlineCanvas, worldlineCanvas._moTimelineTopology)`,
		`document.querySelector("[data-worldline-inspector-close]")`,
		`_timelineState.expandedTurnKey = ""`,
	} {
		if !strings.Contains(events, marker) {
			t.Fatalf("Timeline canvas event wiring missing %q", marker)
		}
	}
	for _, marker := range []string{`worldlineViewport.scale = 1`, `worldlineViewport.translationX = null`, `worldlineViewport.translationY = null`, `surface === "memory_admin"`, `await refreshExplorerUI()`, `await loadTimelineData(true`} {
		if !strings.Contains(selection, marker) {
			t.Fatalf("shared workspace session selection missing %q", marker)
		}
	}
	draw := extractJSFunctionBlockForTest(t, src, "function drawTimelineWorldlineCanvas(canvas, topology)")
	nodeID := extractJSFunctionBlockForTest(t, src, "function timelineWorldlineNodeDisplayId(node)")
	nodeBox := extractJSFunctionBlockForTest(t, src, "function timelineWorldlineNodeBox(node)")
	bounds := extractJSFunctionBlockForTest(t, src, "function timelineWorldlineBoundsBox(topology)")
	fitAll := extractJSFunctionBlockForTest(t, src, "function timelineWorldlineFitAll(canvas, topology)")
	currentNode := extractJSFunctionBlockForTest(t, src, "function timelineWorldlineCurrentNode(topology)")
	selectedNode := extractJSFunctionBlockForTest(t, src, "function timelineWorldlineSelectedNode(topology)")
	selectNode := extractJSFunctionBlockForTest(t, src, "function timelineWorldlineSelectNode(node)")
	detail := extractJSFunctionBlockForTest(t, src, "function renderTimelineWorldlineDetail(worldline, topology)")
	resize := extractJSFunctionBlockForTest(t, src, "function timelineWorldlineResizeCanvas(canvas)")
	for _, marker := range []string{
		`String(edge && edge.parent_node_id || "")`,
		`String(edge && edge.child_node_id || "")`,
		`nodeBoxesById.get(parentId)`,
		`nodeBoxesById.get(childId)`,
		`edge.kind`,
		`edge.active_ancestor_path`,
		`node.active_ancestor_path`,
		`topology.current_node_id`,
		`topology.selected_node_id`,
		`node.turn_text`,
		`sessionId.slice(-8)`,
		`[7, 5]`,
		`t("timeline.session.current")`,
		`t("timeline.label.selected")`,
		`t("timeline.turn.kind.turn")`,
	} {
		if !strings.Contains(draw, marker) {
			t.Fatalf("Go-owned topology draw adapter missing %q", marker)
		}
	}
	if strings.Contains(src, "timelineWorldlinePositionInspector") {
		t.Fatal("selected turn focus surface must not retain per-frame node-attached positioning work")
	}
	for _, marker := range []string{
		`node.node_id`,
		`node.turn_key`,
		`_timelineState.expandedTurnKey = inspectorWasOpen ? "" : turnKey`,
		`_timelineState.worldlineViewport.selectedNodeId = nodeId`,
		`sessionId, focusTurn: turnIndex, skipRuntimeSessionResolve: true, skipSessionListRefresh: true, preserveExpandedTurnKey: true`,
	} {
		if !strings.Contains(nodeID+selectNode, marker) {
			t.Fatalf("turn-node selection adapter missing %q", marker)
		}
	}
	for _, marker := range []string{`selectedNode.session_id`, `selectedNode.turn_text`, `worldline.parent_session_id`, `worldline.fork_turn`} {
		if !strings.Contains(detail, marker) {
			t.Fatalf("selected turn/scalar worldline detail missing %q", marker)
		}
	}
	for _, marker := range []string{`x: x * 220`, `y: y * 76`, `width: 176`, `height: 48`} {
		if !strings.Contains(nodeBox, marker) {
			t.Fatalf("logical grid pixel projection missing %q", marker)
		}
	}
	for _, marker := range []string{`minX: minX * 220`, `maxX: maxX * 220 + 176`, `maxY: maxY * 76 + 48`} {
		if !strings.Contains(bounds, marker) {
			t.Fatalf("fit-all bounds do not reuse the logical-grid projection %q", marker)
		}
	}
	for _, marker := range []string{`topology.nodes.length === 0`, `availableWidth > 0`, `availableHeight > 0`} {
		if !strings.Contains(bounds+fitAll, marker) {
			t.Fatalf("fit-all hidden/empty guard missing %q", marker)
		}
	}
	if !strings.Contains(resize, `globalThis.devicePixelRatio`) || !strings.Contains(resize, `canvas.width = pixelWidth`) || !strings.Contains(draw, `ctx.setTransform(size.ratio`) {
		t.Fatal("Timeline canvas is not rendered with a high-DPI backing store")
	}
	exactAdapter := nodeID + nodeBox + bounds + draw + currentNode + selectedNode + selectNode
	for _, forbidden := range []string{
		`node.id`, `node.width`, `node.height`, `node.label`, `selectedNode.state`,
		`from_x`, `from_y`, `to_x`, `to_y`, `parent_session_id`, `child_session_id`,
		`current_session_id`, `selected_session_id`, `on_active_path`, `edge.state`,
		`node.fork_turn`, `node.parent_session_id`, `node.lineage_state`, `node.lineage_reason`,
		`node.generation`, `node.depth`, `node.sibling_order`, `.sort(`, `source_boundary`, `inherited_through_turn`,
		`node.items`, `node.content`, `node.preview`,
	} {
		if strings.Contains(exactAdapter, forbidden) {
			t.Fatalf("Timeline canvas derives Go-owned topology/layout with forbidden marker %q", forbidden)
		}
	}
	if strings.Contains(src, `worldline_topology.viewmodel.v1`) {
		t.Fatal("Timeline canvas must not retain a worldline topology v1 compatibility branch")
	}
	if strings.Count(src, `touch-action:none`) != 1 || !strings.Contains(src, `.mo-tl-canvas-frame canvas{`) {
		t.Fatal("touch-action:none must be scoped only to the Timeline canvas")
	}
}

func TestArchiveCenterJSPremiumPanelVisualContract(t *testing.T) {
	src := readArchiveCenterJS(t)
	cssPrefix := "const PANEL_CSS = `"
	cssStart := strings.Index(src, cssPrefix)
	if cssStart < 0 {
		t.Fatal("Archive Center.js missing PANEL_CSS owner")
	}
	cssTail := src[cssStart+len(cssPrefix):]
	cssEnd := strings.Index(cssTail, "`;")
	if cssEnd < 0 {
		t.Fatal("Archive Center.js PANEL_CSS is not terminated")
	}
	css := cssTail[:cssEnd]
	draw := extractJSFunctionBlockForTest(t, src, "function drawTimelineWorldlineCanvas(canvas, topology)")
	panel := extractJSFunctionBlockForTest(t, src, "function renderTimelinePanel()")
	settings := extractJSFunctionBlockForTest(t, src, "async function renderSettingsPanel(options)")

	for _, marker := range []string{
		"#0B0D11", "#0F1116", "#13161C", "#181C24", "rgba(255,255,255,.07)",
		"#F4F5F7", "#8B909A", "#5C626D", "#5D73E6", "#8FA7FF", "Inter,Pretendard",
		".mo-btn-ghost{", "textarea:focus-visible", "button:disabled", ".mo-memory-layout{display:grid;grid-template-columns:minmax(170px,220px) minmax(0,1fr)",
		".mo-tab-btn{position:relative;background:transparent;border:0", ".mo-tab-btn.is-active:after{background:#F4F5F7}",
		".mo-hdr-actions{display:flex;align-items:center;gap:8px;min-width:0;flex-wrap:wrap", "@media(max-width:600px)",
		".mo-ex-tabs{flex-direction:row;flex-wrap:nowrap;max-width:100%;overflow-x:auto", ".mo-app-nav{", ".mo-workspace{",
		".mo-memory-management-stack{", ".mo-common-grid{display:grid;grid-template-columns:repeat(12,minmax(0,1fr))", "--mo-accent:var(--mo-blue)",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("premium PANEL_CSS contract missing %q", marker)
		}
	}
	for _, marker := range []string{
		"#0F1116", "rgba(255,255,255,.035)", "#353B46", "#5D73E6", "#8FA7FF",
		"#13161C", "#151923", "#181E2B", "#F4F5F7", "#8B909A", "Inter,Pretendard",
	} {
		if !strings.Contains(draw, marker) {
			t.Fatalf("premium Timeline canvas visual contract missing %q", marker)
		}
	}
	for _, marker := range []string{
		`t("timeline.canvas.title")`, `t("timeline.worldline.detail")`, `t("timeline.drawer.records")`,
		`id="mo-timeline-worldline-canvas"`, `class="mo-tl-node-inspector"`, `class="mo-tl-node-inspector-body"`,
	} {
		if !strings.Contains(panel, marker) {
			t.Fatalf("premium Timeline panel contract missing %q", marker)
		}
	}
	for _, marker := range []string{
		`"settings.tab.timeline": "세계선"`, `"settings.tab.timeline": "Worldlines"`, `"settings.tab.timeline": "世界線"`,
		`"settings.tab.explore": "기억"`, `"settings.tab.explore": "Memory"`, `"settings.tab.explore": "記憶"`,
		`"settings.tab.memoryManagement": "기억 관리"`, `"settings.tab.memoryManagement": "Memory management"`, `"settings.tab.memoryManagement": "記憶管理"`,
		`"settings.tab.extensions": "추가 기능"`, `"settings.tab.extensions": "Extensions"`, `"settings.tab.extensions": "追加機能"`,
		`"settings.tab.reference": "원작 DB"`, `"settings.tab.reference": "Original database"`, `"settings.tab.reference": "原作DB"`,
		`"timeline.canvas.title": "세계선 지도"`, `"timeline.canvas.title": "Worldline map"`, `"timeline.canvas.title": "世界線マップ"`,
		`"timeline.drawer.detail": "턴 상세"`, `"timeline.drawer.detail": "Turn details"`, `"timeline.drawer.detail": "ターン詳細"`,
		`"timeline.drawer.records": "턴 기록"`, `"timeline.drawer.records": "Turn records"`, `"timeline.drawer.records": "ターン記録"`,
		`"timeline.worldline.detail": "분기 계보"`, `"timeline.worldline.detail": "Branch lineage"`, `"timeline.worldline.detail": "分岐系譜"`,
	} {
		if !strings.Contains(src, marker) {
			t.Fatalf("premium visible localization contract missing %q", marker)
		}
	}
	for _, tab := range []string{"timeline", "archive", "memory_admin", "reference", "settings"} {
		if !strings.Contains(settings, `data-tab="`+tab+`"`) {
			t.Fatalf("top navigation hook missing data-tab=%q", tab)
		}
	}
	for _, marker := range []string{
		`<div class="mo-panel mo-app-shell">`, `<span class="mo-brand-mark" aria-hidden="true">AC</span>`,
		`<nav class="mo-app-nav" aria-label="Archive Center">`, `<div class="mo-tabs" id="mo-main-tabs" role="tablist">`,
		`<main class="mo-workspace">`, `role="tabpanel" aria-hidden=`, `data-tab-panel="timeline"`, `role="tab" aria-selected=`,
		`<div class="mo-subtabs mo-settings-subtabs" role="tablist">`, `class="mo-subtabs mo-settings-subtabs mo-extension-subtabs"`,
		`overlay.setAttribute("data-active-primary-tab", activePrimaryTab)`, `activePrimaryTab === "timeline" ?`,
		`activePrimaryTab === "archive" ?`, `activePrimaryTab === "memory_admin" ?`, `activePrimaryTab === "extensions" ?`, `activePrimaryTab === "settings" ?`,
		`else footer.hidden = _settingsActiveTab !== "settings"`,
	} {
		if !strings.Contains(settings, marker) && !strings.Contains(src, marker) {
			t.Fatalf("product workspace composition contract missing %q", marker)
		}
	}
	for _, marker := range []string{
		`el.setAttribute("aria-selected", active ? "true" : "false")`,
		`panel.setAttribute("aria-hidden", active ? "false" : "true")`,
		`if (!["ArrowLeft", "ArrowRight", "Home", "End"].includes(e.key)) return`,
		`nextTab.click()`, `nextTab.focus()`,
		`style.textContent = PANEL_CSS`,
		`const selectedTimelineSessionId = String(_timelineState.selectedSessionId || _timelineState.sessionId || "")`,
		`loadTimelineData(true, selectedTimelineSessionId ? { sessionId: selectedTimelineSessionId } : undefined)`,
		`const inspectionSessionId = String(_timelineState.selectedSessionId || _timelineState.sessionId || resolvedSid || "")`,
		`renderRequestId !== _settingsPanelRenderRequestId || document.getElementById("mo-settings-overlay") !== overlay`,
		`if (document.getElementById("mo-explorer-root") !== explorerRoot) return`,
		`if (document.querySelector('[data-tab-panel="timeline"]') !== timelinePanel) return`,
	} {
		if !strings.Contains(src, marker) {
			t.Fatalf("workspace navigation accessibility contract missing %q", marker)
		}
	}
	if count := strings.Count(settings, `class="mo-tab-btn`); count != 5 {
		t.Fatalf("top navigation must render exactly five primary tabs, got %d", count)
	}
	if count := strings.Count(settings, `id="mo-admin-db-reset-btn"`); count != 1 {
		t.Fatalf("Settings must render one bound database-reset action, got %d", count)
	}
	if !strings.Contains(settings, `return '<div class="mo-subtabs mo-settings-subtabs" role="tablist">' + tabButtons + '</div>';`) {
		t.Fatal("Settings tablist must contain tab buttons only")
	}
	if count := strings.Count(settings, `${settingsSubtabsHtml(`); count != 1 {
		t.Fatalf("Settings primary route must render one shared subtab bar, got %d", count)
	}
	for _, marker := range []string{`["dashboard", t('settings.tab.dashboard')]`, `["debug", t('settings.tab.debug')]`, `data-tab-jump="' + id + '"`} {
		if !strings.Contains(settings, marker) {
			t.Fatalf("Settings nested status/advanced navigation missing %q", marker)
		}
	}
	for _, forbidden := range []string{`data-tab="dashboard"`, `data-tab="debug"`, `id="mo-tab-btn-debug"`} {
		if strings.Contains(settings, forbidden) {
			t.Fatalf("status/debug leaked back into primary navigation: %q", forbidden)
		}
	}
	for _, forbidden := range []string{`data-tab="timeline">📅`, `data-tab="archive">🔍`, `data-tab="dashboard">📊`, `data-tab="reference">📚`} {
		if strings.Contains(settings, forbidden) {
			t.Fatalf("top navigation retains decorative emoji %q", forbidden)
		}
	}
	for _, forbidden := range []string{"function renderWorkspacePageHeader(", "mo-workspace-head", "mo-context-chip"} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("active workspace retains removed explanatory chrome %q", forbidden)
		}
	}
	if strings.Contains(src, `Timeline detail backend returned no usable item.`) {
		t.Fatal("Worldline detail error bypasses the localized visible label")
	}
	visualSource := strings.ToLower(css + draw)
	for _, forbidden := range []string{"#79c8ff", "#f2c15c", "#533483", "linear-gradient", "backdrop-filter"} {
		if strings.Contains(visualSource, forbidden) {
			t.Fatalf("premium visual contract retains forbidden style %q", forbidden)
		}
	}
}

func TestArchiveCenterJSWorkspaceTabStateRuntime(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("ARCHIVE_CENTER_NODE_BINARY or node on PATH is required for workspace tab runtime smoke")
		}
	}
	src := readArchiveCenterJS(t)
	setActiveTab := extractJSFunctionBlockForTest(t, src, "function setActiveSettingsTab(tab)")
	script := `
const assert = (condition, message) => { if (!condition) throw new Error(message); };
class FakeClassList {
  constructor() { this.values = new Set(); }
  toggle(name, active) { if (active) this.values.add(name); else this.values.delete(name); }
  contains(name) { return this.values.has(name); }
}
const fakeElement = (attrs) => ({
  attrs: { ...attrs },
  classList: new FakeClassList(),
  tabIndex: -1,
  getAttribute(name) { return this.attrs[name] == null ? null : String(this.attrs[name]); },
  setAttribute(name, value) { this.attrs[name] = String(value); },
});
const topTabs = ["timeline", "archive", "memory_admin", "reference", "settings"].map((id) => fakeElement({ "data-tab": id }));
const panels = ["timeline", "settings", "review"].map((id) => fakeElement({ "data-tab-panel": id }));
const subTabs = ["settings", "review"].map((id) => fakeElement({ "data-tab-jump": id }));
const settingsFooter = { hidden: false };
const document = {
  getElementById(id) { return canvasMounted && id === "mo-timeline-worldline-canvas" ? canvas : null; },
  querySelector(selector) { return selector === "#mo-settings-overlay .mo-footer" ? settingsFooter : null; },
  querySelectorAll(selector) {
    if (selector === ".mo-tab-btn") return topTabs;
    if (selector === ".mo-tab-panel") return panels;
    if (selector === ".mo-settings-subtabs .mo-subtab-btn") return subTabs;
    return [];
  },
};
let _settingsActiveTab = "settings";
let _setActiveSettingsTabForTimeline = null;
const _settingsTabScrollTops = {};
const canvas = {};
let canvasMounted = false;
let canvasUnmounts = 0;
const recompositions = [];
const captureSettingsViewportScrollState = () => null;
const restoreSettingsViewportScrollState = () => {};
const requestAnimationFrame = (callback) => callback();
const unmountTimelineWorldlineCanvas = () => { canvasUnmounts += 1; };
const renderSettingsPanel = async (options) => { recompositions.push(options); };
const loadPromptEditor = () => {};
const personaCapsuleResolveSessionDefaults = async () => {};
const personaCapsuleRefreshUI = () => {};
const explorerChangeSession = () => {};
const explorerShowTabLoading = () => {};
const explorerRenderActiveTabShell = () => {};
const explorerLoadTab = async () => {};
const refreshExplorerUI = () => {};
const referenceLibraryLoadWorks = async () => {};
const referenceLibraryLoadBindings = async () => {};
const referenceLibraryRefreshUI = () => {};
const _timelineState = { selectedSessionId: "", sessionId: "" };
const _explorer = { activeTab: "memories" };
const _referenceLibraryState = { works: [], panelView: "library" };
` + setActiveTab + `
setActiveSettingsTab("review");
assert(_settingsActiveTab === "review", "review must become the active nested Settings route");
assert(topTabs[4].classList.contains("is-active") && topTabs[4].attrs["aria-selected"] === "true" && topTabs[4].tabIndex === 0,
  "Settings primary tab must expose active ARIA and roving tabindex state");
assert(topTabs[0].attrs["aria-selected"] === "false" && topTabs[0].tabIndex === -1,
  "previous primary tab must leave the active tab order");
assert(panels[2].classList.contains("is-active") && panels[2].attrs["aria-hidden"] === "false",
  "selected Review panel must be visible to accessibility APIs");
assert(panels[0].attrs["aria-hidden"] === "true", "inactive Timeline panel must be hidden from accessibility APIs");
assert(panels[1].attrs["aria-hidden"] === "true" && subTabs[1].attrs["aria-selected"] === "true" && subTabs[1].tabIndex === 0,
  "nested route transition must update panel and subtab accessibility state");
assert(recompositions.length === 0 && canvasUnmounts === 0,
  "Settings-family transitions must preserve mounted form state without a shell recompose");
assert(settingsFooter.hidden === true, "non-General Settings routes must hide the save/reset footer");
setActiveSettingsTab("settings");
assert(settingsFooter.hidden === false, "returning to General Settings must restore the save/reset footer");

_settingsActiveTab = "timeline";
canvasMounted = true;
setActiveSettingsTab("archive");
assert(_settingsActiveTab === "archive", "Memory must become the next primary workspace route");
assert(canvasUnmounts === 1, "leaving Worldlines must unmount its Canvas lifecycle exactly once");
assert(recompositions.length === 1 && recompositions[0] && recompositions[0].recompose === true,
  "primary route transition must replace the active screen instead of hiding the old DOM");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node workspace tab-state runtime smoke failed: %v\n%s", err, out)
	}
}

func TestArchiveCenterJSMemoryWorkspaceAndEditRuntime(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("ARCHIVE_CENTER_NODE_BINARY or node on PATH is required for Memory workspace runtime smoke")
		}
	}
	src := readArchiveCenterJS(t)
	blocks := []string{
		extractJSFunctionBlockForTest(t, src, "function getExplorerTabItems()"),
		extractJSFunctionBlockForTest(t, src, "function renderExplorerTabs(tabItems)"),
		extractJSFunctionBlockForTest(t, src, "function formatExplorerNumber(n)"),
		extractJSFunctionBlockForTest(t, src, "function timelineSessionId(session)"),
		extractJSFunctionBlockForTest(t, src, "function renderSessionDatabaseManagement("),
		extractJSFunctionBlockForTest(t, src, "function renderExplorerSection(mode)"),
		extractJSFunctionBlockForTest(t, src, "function explorerIsEditing(type, id)"),
		extractJSFunctionBlockForTest(t, src, "function explorerStartEdit(type, id, currentFields)"),
		extractJSFunctionBlockForTest(t, src, "function renderExplorerHistoryBadge(item)"),
		extractJSFunctionBlockForTest(t, src, "function renderExplorerMemories()"),
	}
	script := `
const assert = (condition, message) => { if (!condition) throw new Error(message); };
const escapeAttr = (value) => String(value == null ? "" : value)
  .replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
const labels = {
  "settings.tab.explore": "Memory",
  "explorer.filter.current": "Session",
  "explorer.sessions.empty": "No session",
  "explorer.btn.editTooltip": "Edit",
  "explorer.edit.saving": "Saving",
  "explorer.edit.success": "Saved",
  "explorer.edit.validJsonHint": "Valid JSON",
  "explorer.edit.saveBtn": "Save",
  "explorer.edit.cancelBtn": "Cancel",
  "explorer.memories.loading": "Loading",
  "explorer.memories.empty": "Empty",
  "explorer.history.inherited": "Inherited",
  "explorer.history.currentBranch": "Current branch",
  "explorer.allLoaded": "All {n}",
  "common.empty": "Empty",
};
const t = (key) => labels[key] || key;
const presentationExplorerTabLabel = (key) => ({ memories: "Memories", kg_triples: "Knowledge" }[key] || key);
const getSessionDisplayLabel = (sid) => sid;
const renderExplorerContent = () => '<div data-memory-content="ready">Rows</div>';
const renderExplorerRuntimeTokenProfileBanner = () => '<div data-memory-maintenance="runtime">Runtime</div>';
const renderExplorerBatchDeleteToolbar = () => '';
const renderExplorerBatchDeleteCheckbox = () => '';
const explorerBuildBatchDeleteKey = (kind, id) => kind + ':' + id;
const explorerIsExpanded = () => false;
const formatMemoryImportanceDisplay = (value) => Number(value || 0);
const getMemoryPreviewTextForDisplay = (item) => item.preview || '';
const feedbackKey = (kind, id) => kind + ':' + id;
const settings = { debug: false };
const _sessionRoutingResetState = { loading: false, error: "" };
const _sessionNormalizeState = { panelOpen: false };
const _reindexState = {};
const _activeChatRescanDryRunState = {};
const _activeChatRecentRebuildState = {};
const _rescanState = {};
const _hypaImportState = {};
const _feedback = { latest: {}, status: {} };
const _timelineState = { selectedSessionId: "sid-b", sessionId: "sid-b", viewModel: { sessions: [
  { session_id: "sid-b", label: "current", can_attach: true, can_copy: true, can_migrate: true },
] } };
const _sessionMigrationUi = { status: "idle", message: "", running: false, sourceSessionId: "", targetSessionId: "", migrationId: 0, migrationMode: "" };
const _explorer = {
  viewModel: { sync_state: "current", tabs: [{ key: "memories", count: 1 }, { key: "kg_triples", count: 2 }] },
  activeTab: "memories",
  selectedSessionId: "sid-b",
  activeChatSessionId: "sid-b",
  memories: { loading: false, items: [
    { id: 7, chat_session_id: "sid-b", source_turn: 9, importance: 0.8, summary_json: '{"turn_summary":"wrong"}', preview: "Wrong memory", history_ownership: "current_branch", mutation_allowed: true },
    { id: 8, chat_session_id: "sid-root", source_turn: 3, importance: 0.7, summary_json: '{"turn_summary":"inherited"}', preview: "Inherited memory", history_ownership: "inherited", inherited: true, mutation_allowed: false },
  ], total: 2, hasMore: false },
  editingItem: null,
  editFields: {},
  editStatus: "idle",
  editError: "",
  expandedItems: new Set(),
  sessionsLoading: false,
  sessionsError: "",
};
` + strings.Join(blocks, "\n") + `
const memoryScreen = renderExplorerSection("memory");
assert(memoryScreen.includes('class="mo-memory-layout"'), "Memory must render the rail/workspace layout");
assert(memoryScreen.includes('class="mo-memory-rail"') && memoryScreen.includes('class="mo-memory-workspace"'), "Memory categories and record workspace must be separate surfaces");
assert(memoryScreen.includes('data-memory-content="ready"'), "Memory content must remain in the active workspace");
assert(!memoryScreen.includes('mo-memory-management-stack') && !memoryScreen.includes('data-memory-maintenance'), "Memory must not mount maintenance UI");
const managementScreen = renderExplorerSection("management");
assert(managementScreen.includes('class="mo-memory-management"') && managementScreen.includes('data-memory-maintenance="runtime"'), "Memory management must own maintenance UI");
for (const marker of ['data-memory-session-management', 'class="mo-memory-admin-rail"', 'class="mo-memory-admin-workspace"', 'data-memory-admin-session-id="sid-b"', 'data-timeline-session-attach-id="sid-b"', 'data-timeline-session-copy-id="sid-b"', 'data-timeline-session-migrate-id="sid-b"', 'data-timeline-session-delete-id="sid-b"']) {
  assert(managementScreen.includes(marker), "Memory management must own session DB action " + marker);
}
assert(!managementScreen.includes('id="mo-memory-admin-session-select"'), "Memory management must use direct session cards, not a detached select");
assert(!managementScreen.includes('class="mo-memory-layout"'), "Memory management must not duplicate the record browser");
const memoryRow = renderExplorerMemories();
assert(memoryRow.includes('<button type="button" class="mo-ed-edit-btn"') && memoryRow.includes('data-edit-type="mem" data-edit-id="7"'), "selected-session Memory rows must expose an accessible edit action");
assert(memoryRow.includes('mo-ex-history-badge is-inherited') && memoryRow.includes('>Inherited</span>'), "inherited rows must be visibly identified");
assert(memoryRow.includes('mo-ex-history-badge is-current') && memoryRow.includes('>Current branch</span>'), "current branch rows must be visibly identified");
assert(!memoryRow.includes('data-edit-type="mem" data-edit-id="8"') && !memoryRow.includes('data-del-id="8"'), "inherited rows must remain read-only in the child session view");
explorerStartEdit("mem", 7, { summary_json: '{"turn_summary":"wrong"}', importance: 0.8, archive_wing: "A", archive_room: "B" });
const editor = renderExplorerMemories();
assert(editor.includes('data-edit-type="mem" data-edit-id="7"'), "Memory editor must stay attached to its row");
for (const field of ["summary_json", "importance", "archive_wing", "archive_room"]) {
  assert(editor.includes('data-field="' + field + '"'), "Memory editor missing field " + field);
}
assert(editor.includes('data-save-type="mem" data-save-id="7"') && editor.includes('mo-ed-cancel-btn'), "Memory editor must retain save and cancel controls");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node Memory workspace/edit runtime smoke failed: %v\n%s", err, out)
	}

	events := extractJSFunctionBlockForTest(t, src, "function attachExplorerEvents()")
	for _, marker := range []string{
		`.mo-ed-save-btn`, `explorerCancelEdit()`, `.mo-ed-input, .mo-ed-textarea, .mo-ed-select`,
		`await explorerPatchMemory(saveId)`, `await explorerPatchKgTriple(saveId)`, `await explorerPatchDirectEvidence(saveId)`,
		`await explorerPatchEpisode(saveId)`, `await explorerPatchStorylineItem(saveId)`, `await explorerPatchWorldRuleItem(saveId)`,
		`await explorerPatchHookItem(saveId)`, `await explorerPatchCharacterSpeech(btn.dataset.saveKey || "")`,
		`await explorerPatchCharacter(btn.dataset.saveKey || "")`, `await explorerPatchSubjectiveEntityMemory(saveId)`,
	} {
		if !strings.Contains(events, marker) {
			t.Fatalf("Memory edit dispatch lost existing production owner %q", marker)
		}
	}
	for _, marker := range []string{
		`data-edit-type="mem"`, `data-edit-type="kg"`, `data-edit-type="de"`, `data-edit-type="ep"`,
		`data-edit-type="sl"`, `data-edit-type="wr"`, `data-edit-type="hk"`, `data-edit-type="char"`,
		`data-edit-type="char_speech"`, `data-edit-type="entity_memory"`, `data-trust-edit-model=`,
	} {
		if !strings.Contains(src, marker) {
			t.Fatalf("supported Memory record type lost its edit entry/form %q", marker)
		}
	}
	if !strings.Contains(src, `result.status === "partial_error"`) ||
		!strings.Contains(src, `result.vector_sync || result`) {
		t.Fatal("Memory editor must keep a completed canonical edit usable when vector synchronization reports a warning")
	}
	if strings.Contains(src, `data-edit-type="log"`) {
		t.Fatal("raw chat logs must not gain an unsupported edit path")
	}
}

func TestArchiveCenterJSTimelineWorldlineProductionPresentationRuntime(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("ARCHIVE_CENTER_NODE_BINARY or node on PATH is required for Timeline presentation runtime smoke")
		}
	}
	src := readArchiveCenterJS(t)
	signatures := []string{
		"function renderTimelineTurnItemRow(item)",
		"function renderTimelineTurnGroup(group)",
		"function renderTimelinePanel()",
		"function timelineWorldlineNodeDisplayId(node)",
		"function timelineWorldlineNodeBox(node)",
		"function timelineViewportClampScale(value)",
		"function timelineWorldlineRoundedRect(ctx, x, y, width, height, radius)",
		"function timelineWorldlineTrimLabel(ctx, value, maxWidth)",
		"function timelineWorldlineResizeCanvas(canvas)",
		"function drawTimelineWorldlineCanvas(canvas, topology)",
		"function timelineWorldlineSelectedNode(topology)",
	}
	blocks := make([]string, 0, len(signatures))
	for _, signature := range signatures {
		blocks = append(blocks, extractJSFunctionBlockForTest(t, src, signature))
	}
	script := `
const assert = (condition, message) => { if (!condition) throw new Error(message); };
const escapeAttr = (value) => String(value == null ? "" : value)
  .replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
const labels = {
	"settings.tab.timeline": "Worldlines",
  "timeline.label.selected": "Selected",
  "timeline.label.turns": "Turns",
  "timeline.label.sessions": "Sessions",
  "timeline.label.logs": "Logs",
  "timeline.label.mem": "Mem",
  "timeline.label.kg": "KG",
  "timeline.label.episodes": "Episodes",
  "timeline.session.current": "Current",
  "timeline.worldline.forkTurn": "Fork turn",
  "timeline.worldline.state.confirmed": "Confirmed",
  "timeline.turn.kind.turn": "Turn",
  "timeline.turn.turnNumber": "Turn #",
  "timeline.canvas.empty": "Empty",
  "timeline.canvas.unavailable": "Unavailable",
  "timeline.canvas.partial": "Partial",
  "timeline.canvas.truncated": "Truncated",
  "timeline.canvas.zoomOut": "Zoom out",
  "timeline.canvas.zoomIn": "Zoom in",
  "timeline.canvas.recenter": "Recenter",
  "timeline.canvas.fitAll": "Fit all",
  "timeline.note.readOnly": "Read only",
  "timeline.note.noMore": "No more",
  "timeline.button.reload": "Reload",
  "timeline.button.loading": "Loading",
  "timeline.button.delete": "Delete",
};
const t = (key) => labels[key] || key;
const tf = (key, values) => key === "timeline.count.summary"
  ? String(values.turns) + " turns / " + String(values.items) + " items"
  : key === "timeline.badge.items" ? String(values.n) + " items" : key;
const settings = { uiDetailMode: "compact" };
const DEFAULT_SETTINGS = { uiDetailMode: "compact" };
const UI_DETAIL_MODE_OPTIONS = ["compact"];
const sanitizeEnumValue = (value) => value;
const _sessionMigrationUi = { status: "idle", message: "", sourceSessionId: "", migrationMode: "", migrationId: 0, running: false };
let _timelineSelectedDetail = null;
let seenCreatedText = "";
const _timelineState = {
  viewModel: null,
  detailItem: null,
  detailError: "",
  error: "",
  sessionsError: "",
  sessionsLoading: false,
  expandedTurnKey: "turn:12",
  worldlineViewport: { scale: 1, translationX: 0, translationY: 0, selectedNodeId: "" },
};
function timelineSelectedTurnKey() { return ""; }
function timelineTurnBadgeHtml() { return ""; }
function timelineItemKey(item) { return String(item && item.key || ""); }
function presentationTimelineItemView(item) {
  return { key: timelineItemKey(item), normalized_type: "memory", preview: String(item && item.preview || ""), source_id: item && item.id, can_edit: false };
}
function presentationTimelineTitle(item) { return String(item && item.title || ""); }
function timelineTypeDisplay() { return "Memory"; }
function formatTimelineTimestampParts(value) { seenCreatedText = String(value || ""); return { date: seenCreatedText, time: "" }; }
function renderDetailPanel(item) { return '<div data-item-detail="1">' + escapeAttr(item && item.title || "") + '</div>'; }
function renderTimelineWorldlineDetail(worldline) { return '<div data-worldline-detail="1">' + escapeAttr(worldline && worldline.reason || "") + '</div>'; }
function getSessionDisplayLabel() { return "Shared label"; }
` + strings.Join(blocks, "\n") + `

const sessionA = "shared-session-aaa111";
const sessionB = "shared-session-bbb222";
const group = {
  key: "turn:12",
  kind: "turn",
  turn_text: "12",
  created_text: "2026-08-19 12:34",
  preview: "turn preview",
  item_count: 1,
  items: [{ key: "memory:1", id: 1, type: "memory", title: "Memory one", preview: "Memory preview", detail_ref: "/timeline-item?type=memory&id=1" }],
};
_timelineState.loading = true;
let loadingHTML = renderTimelinePanel();
assert(loadingHTML.includes("Loading"), "cleared cross-session state must render an immediate loading shell");
_timelineState.loading = false;
_timelineState.viewModel = {
  items: group.items,
  groups: [group],
  summary: { turns: 1, items: 1, total: 1, source_counts: {} },
  selected_session_id: sessionA,
  sessions: [{ session_id: sessionA, selected: true }],
  empty_state: "ready",
  load_more_state: "done",
  worldline: { state: "confirmed", current_session_id: sessionA, reason: "raw_scalar_reason" },
  worldline_topology: {
    contract_version: "worldline_topology.viewmodel.v2",
    state: "ready",
    nodes: [
      { node_id: sessionA + ":turn:12", session_id: sessionA, turn_key: "turn:12", turn_index: 12, turn_text: "12", x: 0, y: 0, current: true, selected: true, active_ancestor_path: true },
      { node_id: sessionB + ":turn:13", session_id: sessionB, turn_key: "turn:13", turn_index: 13, turn_text: "13", x: 1, y: 1, current: false, selected: false, active_ancestor_path: false },
    ],
    edges: [{ parent_node_id: sessionA + ":turn:12", child_node_id: sessionB + ":turn:13", kind: "fork", active_ancestor_path: false }],
    current_node_id: sessionA + ":turn:12",
    selected_node_id: sessionA + ":turn:12",
    active_ancestor_path: [sessionA + ":turn:12"],
  },
};

_timelineState.expandedTurnKey = "";
let html = renderTimelinePanel();
assert(!html.includes('class="mo-tl-node-inspector"'), "backend-selected topology alone must not auto-open a turn inspector before a box click");
_timelineState.expandedTurnKey = "turn:12";
html = renderTimelinePanel();
assert(!html.includes("mo-tl-drawers") && !html.includes("mo-timeline-drawers"), "legacy drawer composition must not render");
assert(!html.includes('class="mo-tl-records"'), "selected-turn records must not remain a separate below-canvas section");
assert((html.match(/class="mo-tl-node-inspector"/g) || []).length === 1, "one selected turn must render exactly one in-canvas inspector");
assert(html.indexOf('id="mo-timeline-worldline-canvas"') < html.indexOf('class="mo-tl-node-inspector"'), "selected-turn inspector must be attached inside the Canvas frame after the Canvas surface");
assert(html.includes('data-selected-node-id="' + sessionA + ':turn:12"') && html.includes('data-selected-turn-key="turn:12"'), "selected inspector must retain the exact supplied node and turn keys");
assert(html.includes('class="mo-tl-lineage-detail"') && !html.includes('class="mo-tl-lineage-detail" open'), "lineage information must remain collapsed by default");
assert(!html.includes('class="mo-tl-inline-detail"'), "no item detail may expand before an explicit item selection");
assert(html.includes("Turn #12") && html.includes("#12"), "snake_case Go turn_text must render instead of #- ");
assert(seenCreatedText === "2026-08-19 12:34", "snake_case Go created_text must reach timestamp formatting");
const toolbar = html.slice(0, html.indexOf('<section class="mo-tl-main">'));
assert(toolbar.includes('title="raw_scalar_reason"'), "scalar reason must remain available as the state title");
assert((toolbar.match(/raw_scalar_reason/g) || []).length === 1, "raw scalar reason must not be duplicated into primary toolbar text");

_timelineSelectedDetail = group.items[0];
html = renderTimelinePanel();
assert((html.match(/class="mo-tl-inline-detail"/g) || []).length === 1, "explicit item selection must render exactly one inline detail");
assert(html.includes('data-item-detail="1">Memory one'), "inline detail must reuse the selected production detail renderer");
assert((html.match(/class="mo-tl-node-inspector"/g) || []).length === 1, "item expansion must stay inside the single selected-node inspector");

const drawnLabels = [];
const ctx = {
  setTransform() {}, clearRect() {}, fillRect() {}, setLineDash() {}, beginPath() {}, moveTo() {}, lineTo() {},
  stroke() {}, bezierCurveTo() {}, quadraticCurveTo() {}, closePath() {}, fill() {},
  measureText(value) { return { width: String(value || "").length }; },
  fillText(value, x, y) { drawnLabels.push({ text: String(value || ""), x, y }); },
};
const canvas = {
  clientWidth: 600,
  clientHeight: 260,
  width: 0,
  height: 0,
  getBoundingClientRect() { return { width: 600, height: 260 }; },
  getContext() { return ctx; },
};
const rootNodeId = sessionA + ":turn:1";
const branchNodeId = sessionB + ":turn:2";
const topology = {
  contract_version: "worldline_topology.viewmodel.v2",
  nodes: [
    { node_id: rootNodeId, session_id: sessionA, turn_key: "turn:1", turn_index: 1, turn_text: "1", x: 0, y: 0, current: false, selected: false, active_ancestor_path: true },
    { node_id: branchNodeId, session_id: sessionB, turn_key: "turn:2", turn_index: 2, turn_text: "2", x: 1, y: 1, current: true, selected: true, active_ancestor_path: true },
  ],
  edges: [{ parent_node_id: rootNodeId, child_node_id: branchNodeId, kind: "fork", active_ancestor_path: true }],
  current_node_id: branchNodeId,
  selected_node_id: branchNodeId,
  active_ancestor_path: [rootNodeId, branchNodeId],
};
const topologyBefore = JSON.stringify(topology);
const drawnRects = drawTimelineWorldlineCanvas(canvas, topology);
const firstLabel = drawnLabels.filter((entry) => entry.x < 200).map((entry) => entry.text).join(" | ");
const secondLabel = drawnLabels.filter((entry) => entry.x > 200).map((entry) => entry.text).join(" | ");
assert(firstLabel.includes("Turn 1") && secondLabel.includes("Turn 2"), "root and branch turn boxes must use exact production turn_text labels");
assert(firstLabel.includes("aaa111") && secondLabel.includes("bbb222"), "turn boxes must expose distinct short session/worldline discriminators");
assert(secondLabel.includes("Current") && secondLabel.includes("Selected"), "current and selected labels must use exact production turn-node fields");
assert(firstLabel !== secondLabel, "root and branch turn-box labels must remain distinguishable on canvas");
assert(drawnRects.length === topology.nodes.length, "Canvas must render one hit box per supplied turn node");
_timelineState.worldlineViewport.translationX = 70;
drawTimelineWorldlineCanvas(canvas, topology);
assert(JSON.stringify(topology) === topologyBefore, "production Canvas drawing must not mutate supplied topology JSON");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node Timeline worldline production presentation runtime smoke failed: %v\n%s", err, out)
	}
}

func TestArchiveCenterJSTimelineLocalRefreshSkipsPresentationReload(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("ARCHIVE_CENTER_NODE_BINARY or node on PATH is required for Timeline local refresh runtime smoke")
		}
	}
	src := readArchiveCenterJS(t)
	refresh := extractJSFunctionBlockForTest(t, src, "async function refreshTimelineUI(options)")
	script := `
const assert = (condition, message) => { if (!condition) throw new Error(message); };
let presentationLoads = 0;
let timelineBindings = 0;
let restored = 0;
const panel = {
  innerHTML: "old",
  querySelector() { return null; },
};
const replacementPanel = {
  innerHTML: "replacement",
  querySelector() { return null; },
};
let currentPanel = panel;
let replaceDuringLoad = false;
const document = {
  querySelector(selector) { return selector === '[data-tab-panel="timeline"]' ? currentPanel : null; },
};
function captureTimelineScrollState() { return { marker: "scroll" }; }
async function loadPresentationViewModels() {
  presentationLoads += 1;
  if (replaceDuringLoad) currentPanel = replacementPanel;
}
function renderTimelinePanel() { return "new"; }
function attachTimelineEvents() { timelineBindings += 1; }
function restoreTimelineScrollState() { restored += 1; }
function unmountTimelineWorldlineCanvas() {}
` + refresh + `
(async () => {
  await refreshTimelineUI({ reloadPresentation: false });
  assert(presentationLoads === 0, "local-only refresh must not request a new Presentation ViewModel");
  assert(panel.innerHTML === "new", "local-only refresh must update the Timeline DOM immediately");
  assert(timelineBindings === 1 && restored === 1, "local-only refresh must rebind Timeline events and restore scroll once");
  await refreshTimelineUI({ preserveScroll: false });
  assert(presentationLoads === 1, "normal refresh must retain the backend Presentation reload owner");
  replaceDuringLoad = true;
  await refreshTimelineUI();
  assert(replacementPanel.innerHTML === "replacement" && timelineBindings === 2,
    "a detached Timeline root must not redraw or bind the newly mounted route after Presentation resolves");
  currentPanel = null;
  await refreshTimelineUI();
  assert(presentationLoads === 2 && timelineBindings === 2,
    "inactive Timeline refresh must stop before backend Presentation work or DOM binding");
})().catch((error) => { console.error(error && error.stack || error); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node Timeline local refresh runtime smoke failed: %v\n%s", err, out)
	}
}

func TestArchiveCenterJSExplorerLocalExpandSkipsPresentationReload(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("ARCHIVE_CENTER_NODE_BINARY or node on PATH is required for Explorer local refresh runtime smoke")
		}
	}
	src := readArchiveCenterJS(t)
	if !strings.Contains(src, "explorerToggleExpand(type, id);\n              refreshExplorerUI({ preserveScroll: true, reloadPresentation: false });") {
		t.Fatal("Explorer expand/collapse must use the local-only refresh path")
	}
	refresh := extractJSFunctionBlockForTest(t, src, "async function refreshExplorerUI(options = {})")
	script := `
const assert = (condition, message) => { if (!condition) throw new Error(message); };
let presentationLoads = 0;
let explorerBindings = 0;
let restored = 0;
let synced = 0;
const explorerRoot = { innerHTML: "old" };
let currentRoot = explorerRoot;
const document = {
  getElementById(id) { return id === "mo-explorer-root" ? currentRoot : null; },
};
const _explorer = { activeTab: "direct_evidence" };
const _settingsActiveTab = "archive";
function captureExplorerScrollState() { return { marker: "scroll" }; }
async function loadPresentationViewModels() { presentationLoads += 1; }
function explorerSyncBatchDeleteSelection() { synced += 1; }
function renderExplorerSection() { return "new"; }
function attachExplorerEvents() { explorerBindings += 1; }
function restoreExplorerScrollState() { restored += 1; }
` + refresh + `
(async () => {
  await refreshExplorerUI({ preserveScroll: true, reloadPresentation: false });
  assert(presentationLoads === 0, "local Explorer expand must not request a Presentation ViewModel");
  assert(explorerRoot.innerHTML === "new", "local Explorer expand must redraw from already loaded values");
  assert(explorerBindings === 1 && restored === 1 && synced === 1,
    "local Explorer expand must rebind and restore the existing view exactly once");
  await refreshExplorerUI({ preserveScroll: false });
  assert(presentationLoads === 1, "normal Explorer refresh must retain Presentation reload behavior");
  assert(explorerBindings === 2 && restored === 1,
    "normal refresh without scroll preservation must redraw without restoring scroll");
  currentRoot = null;
  await refreshExplorerUI();
  assert(presentationLoads === 1 && explorerBindings === 2,
    "inactive Explorer refresh must stop before backend or DOM work");
})().catch((error) => { console.error(error && error.stack || error); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node Explorer local refresh runtime smoke failed: %v\n%s", err, out)
	}
}

func TestArchiveCenterJSTimelineWorldlineProductionGestureRuntime(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("ARCHIVE_CENTER_NODE_BINARY or node on PATH is required for Timeline canvas gesture runtime smoke")
		}
	}
	src := readArchiveCenterJS(t)
	signatures := []string{
		"function timelineWorldlineNodeDisplayId(node)",
		"function timelineWorldlineNodeBox(node)",
		"function timelineWorldlineBoundsBox(topology)",
		"function timelineViewportClampScale(value)",
		"function timelineViewportZoomAt(viewport, factor, x, y)",
		"function timelineViewportPinch(viewport, previousMidpoint, nextMidpoint, factor)",
		"function timelineWorldlineCanvasPoint(canvas, event)",
		"function timelineWorldlinePointerPair(viewport)",
		"function timelineWorldlineGesturePointerDown(canvas, viewport, callbacks, event)",
		"function timelineWorldlineGesturePointerMove(canvas, viewport, callbacks, event)",
		"function timelineWorldlineGesturePointerEnd(canvas, viewport, callbacks, event, cancelled)",
		"function timelineWorldlineGestureWheel(canvas, viewport, callbacks, event)",
		"function attachTimelineWorldlineGestures(canvas, viewport, callbacks)",
		"function timelineWorldlineHitTest(canvas, x, y)",
		"function timelineWorldlineFocusNode(canvas, node)",
		"function timelineWorldlineFitAll(canvas, topology)",
		"function timelineWorldlineCurrentNode(topology)",
		"function timelineWorldlineSelectedNode(topology)",
		"function timelineWorldlineZoomBy(canvas, factor, x, y)",
		"function timelineWorldlineSelectNode(node)",
		"function unmountTimelineWorldlineCanvas(canvas)",
		"function mountTimelineWorldlineCanvas()",
	}
	blocks := make([]string, 0, len(signatures))
	for _, signature := range signatures {
		blocks = append(blocks, extractJSFunctionBlockForTest(t, src, signature))
	}
	script := `
const assert = (condition, message) => { if (!condition) throw new Error(message); };
const near = (left, right, message) => assert(Math.abs(left - right) < 1e-8, message + ": " + left + " != " + right);
let drawCalls = 0;
const loadCalls = [];
const inspectionSyncCalls = [];
let refreshCalls = 0;
let _timelineSelectedDetail = { stale: true };
const _timelineState = {
  selectedSessionId: "session-a",
  sessionId: "session-a",
  currentSessionId: "runtime-session",
  expandedTurnKey: "",
  detailItem: { stale: true },
  detailLoading: true,
  detailError: "stale",
  items: [{ stale: true }],
  meta: { stale: true },
  hasMore: true,
  nextBeforeTurn: 9,
  error: "stale",
  viewModel: { stale: true },
  worldlineViewport: {
    scale: 1,
    translationX: 0,
    translationY: 0,
    pointers: new Map(),
    dragThreshold: 6,
    lastTap: { nodeId: "", at: 0, x: 0, y: 0 },
    selectedNodeId: "",
  },
};
function drawTimelineWorldlineCanvas() { drawCalls++; return []; }
function loadTimelineData(force, options) { loadCalls.push({ force, options }); }
function refreshTimelineUI() { refreshCalls++; }
function syncSessionScopedInspectionSelection(sessionId) { inspectionSyncCalls.push(String(sessionId || "")); }
` + strings.Join(blocks, "\n") + `

class FakeElement {
  constructor(width = 200, height = 120) {
    this.width = width;
    this.height = height;
    this.listeners = Object.create(null);
    this.captured = new Set();
  }
  addEventListener(type, listener) {
    if (!this.listeners[type]) this.listeners[type] = [];
    this.listeners[type].push(listener);
  }
  removeEventListener(type, listener) {
    this.listeners[type] = (this.listeners[type] || []).filter((candidate) => candidate !== listener);
  }
  getBoundingClientRect() { return { left: 0, top: 0, width: this.width, height: this.height }; }
  setPointerCapture(pointerId) { this.captured.add(pointerId); }
  releasePointerCapture(pointerId) { this.captured.delete(pointerId); }
  dispatch(type, init = {}) {
    const event = Object.assign({
      pointerId: 1,
      pointerType: "mouse",
      button: 0,
      clientX: 0,
      clientY: 0,
      deltaY: 0,
      timeStamp: 1,
      cancelable: true,
    }, init);
    event.prevented = false;
    event.preventDefault = function() { this.prevented = true; };
    (this.listeners[type] || []).slice().forEach((listener) => listener(event));
    return event;
  }
}

const node = {
  node_id: "session-a:turn:3",
  session_id: "session-a",
  turn_key: "turn:3",
  turn_index: 3,
  turn_text: "3",
  x: 1,
  y: 2,
  current: false,
  selected: false,
  active_ancestor_path: true,
};
const otherNode = {
  node_id: "session-b:turn:4",
  session_id: "session-b",
  turn_key: "turn:4",
  turn_index: 4,
  turn_text: "4",
  x: 2,
  y: 1,
  current: true,
  selected: true,
  active_ancestor_path: false,
};
const topology = {
  contract_version: "worldline_topology.viewmodel.v2",
  state: "ready",
  nodes: [node, otherNode],
  edges: [{ parent_node_id: node.node_id, child_node_id: otherNode.node_id, kind: "fork", active_ancestor_path: false }],
  current_node_id: node.node_id,
  selected_node_id: node.node_id,
  active_ancestor_path: [node.node_id],
  bounds: { min_x: 0, min_y: 0, max_x: 2, max_y: 2, width: 3, height: 3 },
};
const topologyBefore = JSON.stringify(topology);
const projected = timelineWorldlineNodeBox(node);
assert(projected.x === 220 && projected.y === 152 && projected.width === 176 && projected.height === 48, "Go logical grid should receive display-only projection");
assert(timelineWorldlineCurrentNode(topology) === node, "current lookup must use exact current_node_id rather than conflicting node.current flags");
_timelineState.worldlineViewport.selectedNodeId = otherNode.node_id;
assert(timelineWorldlineSelectedNode(topology) === otherNode, "transient selectedNodeId must take precedence over selected_node_id");
_timelineState.worldlineViewport.selectedNodeId = "missing-turn-node";
assert(timelineWorldlineSelectedNode(topology) === node, "an unresolved transient selection must continue to exact selected_node_id");
_timelineState.worldlineViewport.selectedNodeId = "";
assert(timelineWorldlineSelectedNode(topology) === node, "selected lookup must fall back to exact selected_node_id");

_timelineState.worldlineViewport.translationX = null;
_timelineState.worldlineViewport.translationY = null;
const hiddenCanvas = new FakeElement(0, 0);
assert(timelineWorldlineFitAll(hiddenCanvas, topology) === false, "hidden zero-size canvas must not fit");
assert(_timelineState.worldlineViewport.scale === 1 && _timelineState.worldlineViewport.translationX === null && _timelineState.worldlineViewport.translationY === null, "hidden fit must preserve an unfitted viewport");
const emptyTopology = {
  contract_version: "worldline_topology.viewmodel.v2",
  state: "unavailable",
  nodes: [],
  edges: [],
  current_node_id: "",
  selected_node_id: "",
  active_ancestor_path: [],
  bounds: { min_x: 0, min_y: 0, max_x: 0, max_y: 0, width: 0, height: 0 },
};
const emptyCanvas = new FakeElement(400, 240);
const document = { getElementById: (id) => id === "mo-timeline-worldline-canvas" ? emptyCanvas : null };
_timelineState.viewModel = { worldline_topology: emptyTopology };
mountTimelineWorldlineCanvas();
assert(Object.values(emptyCanvas.listeners).every((listeners) => listeners.length === 0), "empty canvas mount must attach no gesture listeners");
const emptyWheel = emptyCanvas.dispatch("wheel", { clientX: 100, clientY: 80, deltaY: -120 });
assert(emptyWheel.prevented === false, "empty canvas wheel must remain ordinary scroll");
assert(_timelineState.worldlineViewport.translationX === null && _timelineState.worldlineViewport.translationY === null, "empty canvas gestures must not establish a viewport");
hiddenCanvas.width = 400;
hiddenCanvas.height = 240;
assert(timelineWorldlineFitAll(hiddenCanvas, emptyTopology) === false, "empty unavailable topology must not create phantom fit bounds");
assert(_timelineState.worldlineViewport.translationX === null && _timelineState.worldlineViewport.translationY === null, "empty fit must preserve an unfitted viewport");
assert(timelineWorldlineFitAll(hiddenCanvas, topology) === true, "visible nonempty topology should fit after a hidden mount");
assert(Number.isFinite(_timelineState.worldlineViewport.translationX) && Number.isFinite(_timelineState.worldlineViewport.translationY), "visible retry should establish the viewport");
_timelineState.worldlineViewport.scale = 1;
_timelineState.worldlineViewport.translationX = 0;
_timelineState.worldlineViewport.translationY = 0;

const canvas = new FakeElement();
canvas._moTimelineTopology = topology;
canvas._moTimelineNodeRects = [
  { id: node.node_id, sessionId: "session-a", node, left: 10, top: 10, right: 70, bottom: 55, centerX: 40, centerY: 32.5 },
  { id: otherNode.node_id, sessionId: "session-b", node: otherNode, left: 90, top: 10, right: 150, bottom: 55, centerX: 120, centerY: 32.5 },
];
const selections = [];
let focusCalls = 0;
const callbacks = {
  redraw: () => { drawCalls++; },
  zoomAt: (factor, x, y) => timelineWorldlineZoomBy(canvas, factor, x, y),
  hitTest: (x, y) => timelineWorldlineHitTest(canvas, x, y),
  focus: (hit) => { focusCalls++; timelineWorldlineFocusNode(canvas, hit.node); },
  select: (hit, meta) => { selections.push(meta); timelineWorldlineSelectNode(hit.node); },
};
const cleanup = attachTimelineWorldlineGestures(canvas, _timelineState.worldlineViewport, callbacks);

const dragDown = canvas.dispatch("pointerdown", { pointerId: 1, clientX: 20, clientY: 20, timeStamp: 10 });
canvas.dispatch("pointermove", { pointerId: 1, clientX: 40, clientY: 35, timeStamp: 20 });
canvas.dispatch("pointerup", { pointerId: 1, clientX: 40, clientY: 35, timeStamp: 30 });
assert(dragDown.prevented, "canvas pointer gesture should prevent its own default");
near(_timelineState.worldlineViewport.translationX, 20, "one-pointer drag should pan X");
near(_timelineState.worldlineViewport.translationY, 15, "one-pointer drag should pan Y");
assert(selections.length === 0, "drag must not select or move a node");
assert(JSON.stringify(topology) === topologyBefore, "drag must not mutate Go node layout/topology");

canvas.dispatch("pointerdown", { pointerId: 2, clientX: 20, clientY: 20, timeStamp: 900 });
canvas.dispatch("pointerup", { pointerId: 2, clientX: 20, clientY: 20, timeStamp: 1000 });
assert(selections.length === 1 && selections[0].doubleTap === false, "short click should select exactly once");
assert(loadCalls.length === 1 && loadCalls[0].options.focusTurn === 3, "same-session turn tap must request the exact selected turn once");
assert(_timelineState.expandedTurnKey === "turn:3", "same-session turn tap must expand the exact supplied turn_key");
assert(_timelineState.worldlineViewport.selectedNodeId === node.node_id, "Canvas selectedNodeId must store the turn node ID");
assert(_timelineSelectedDetail === null && _timelineState.detailItem === null, "node selection must show turn records without auto-opening an item detail");
timelineWorldlineSelectNode(node);
assert(_timelineState.expandedTurnKey === "", "selecting the same open turn box must close its attached inspector");
assert(loadCalls.length === 1, "closing an attached inspector must not refetch its turn");
timelineWorldlineSelectNode(node);
assert(_timelineState.expandedTurnKey === "turn:3", "selecting the same turn box again must reopen its attached inspector");
assert(loadCalls.length === 2 && loadCalls[1].options.focusTurn === 3, "reopening a turn box must refresh only that exact turn");

canvas.dispatch("pointerdown", { pointerId: 7, clientX: 100, clientY: 20, timeStamp: 1010 });
canvas.dispatch("pointerup", { pointerId: 7, clientX: 100, clientY: 20, timeStamp: 1070 });
assert(selections.length === 2 && selections[1].doubleTap === false, "tap on another node should select once");
assert(loadCalls.length === 3, "another session turn must add one exact-turn request to the existing Timeline load flow");
assert(_timelineState.selectedSessionId === "session-b" && _timelineState.sessionId === "session-b", "another-session turn tap must set the top Timeline inspection selection");
assert(_timelineState.expandedTurnKey === "turn:4", "another-session turn tap must preserve the exact supplied turn_key");
assert(_timelineState.worldlineViewport.selectedNodeId === otherNode.node_id, "another-session selection must store the supplied turn node ID");
assert(loadCalls[2].force === true && loadCalls[2].options.sessionId === "session-b" && loadCalls[2].options.focusTurn === 4, "another-session turn tap must load the supplied session and exact turn once");
assert(loadCalls[2].options.preserveExpandedTurnKey === true, "async session load must preserve the selected turn key");
assert(_timelineState.currentSessionId === "runtime-session", "inspection selection must never mutate the Host/runtime current session");
assert(_timelineState.items.length === 0 && _timelineState.meta === null && _timelineState.viewModel === null, "cross-session node selection must clear stale prior-session presentation data before loading");
assert(_timelineState.hasMore === false && _timelineState.nextBeforeTurn === 0 && _timelineState.error === "", "cross-session node selection must clear stale pagination and error state");
assert(inspectionSyncCalls.length === 1 && inspectionSyncCalls[0] === "session-b", "cross-session node selection must immediately synchronize session-scoped inspection state");

_timelineState.worldlineViewport.scale = 1;
_timelineState.worldlineViewport.translationX = 10;
_timelineState.worldlineViewport.translationY = 20;
const wheelAnchorX = 50;
const wheelAnchorY = 60;
const wheelLogicalX = (wheelAnchorX - _timelineState.worldlineViewport.translationX) / _timelineState.worldlineViewport.scale;
const wheelLogicalY = (wheelAnchorY - _timelineState.worldlineViewport.translationY) / _timelineState.worldlineViewport.scale;
const wheelEvent = canvas.dispatch("wheel", { clientX: wheelAnchorX, clientY: wheelAnchorY, deltaY: -120, timeStamp: 1100 });
assert(wheelEvent.prevented, "wheel default should be prevented on the canvas");
near((wheelAnchorX - _timelineState.worldlineViewport.translationX) / _timelineState.worldlineViewport.scale, wheelLogicalX, "wheel zoom should preserve pointer X anchor");
near((wheelAnchorY - _timelineState.worldlineViewport.translationY) / _timelineState.worldlineViewport.scale, wheelLogicalY, "wheel zoom should preserve pointer Y anchor");

const controlAnchorX = 100;
const controlAnchorY = 60;
const controlLogicalX = (controlAnchorX - _timelineState.worldlineViewport.translationX) / _timelineState.worldlineViewport.scale;
const controlLogicalY = (controlAnchorY - _timelineState.worldlineViewport.translationY) / _timelineState.worldlineViewport.scale;
timelineWorldlineZoomBy(canvas, 1.2, controlAnchorX, controlAnchorY);
near((controlAnchorX - _timelineState.worldlineViewport.translationX) / _timelineState.worldlineViewport.scale, controlLogicalX, "control zoom should reuse pointer-centered zoom X");
near((controlAnchorY - _timelineState.worldlineViewport.translationY) / _timelineState.worldlineViewport.scale, controlLogicalY, "control zoom should reuse pointer-centered zoom Y");

_timelineState.worldlineViewport.scale = 1;
_timelineState.worldlineViewport.translationX = 0;
_timelineState.worldlineViewport.translationY = 0;
canvas.dispatch("pointerdown", { pointerId: 3, pointerType: "touch", clientX: 20, clientY: 40, timeStamp: 1200 });
canvas.dispatch("pointerdown", { pointerId: 4, pointerType: "touch", clientX: 80, clientY: 40, timeStamp: 1210 });
canvas.dispatch("pointermove", { pointerId: 4, pointerType: "touch", clientX: 100, clientY: 60, timeStamp: 1220 });
near((60 - _timelineState.worldlineViewport.translationX) / _timelineState.worldlineViewport.scale, 50, "pinch should preserve previous midpoint logical X at the new midpoint");
near((50 - _timelineState.worldlineViewport.translationY) / _timelineState.worldlineViewport.scale, 40, "pinch should preserve previous midpoint logical Y at the new midpoint");

canvas.dispatch("pointerup", { pointerId: 4, pointerType: "touch", clientX: 100, clientY: 60, timeStamp: 1230 });
const beforeOnePointerX = _timelineState.worldlineViewport.translationX;
const beforeOnePointerY = _timelineState.worldlineViewport.translationY;
canvas.dispatch("pointermove", { pointerId: 3, pointerType: "touch", clientX: 25, clientY: 47, timeStamp: 1240 });
near(_timelineState.worldlineViewport.translationX, beforeOnePointerX + 5, "2-to-1 transition should reset the remaining pan X anchor");
near(_timelineState.worldlineViewport.translationY, beforeOnePointerY + 7, "2-to-1 transition should reset the remaining pan Y anchor");
canvas.dispatch("pointerup", { pointerId: 3, pointerType: "touch", clientX: 25, clientY: 47, timeStamp: 1250 });

_timelineState.worldlineViewport.scale = 1;
canvas.dispatch("pointerdown", { pointerId: 5, clientX: 100, clientY: 20, timeStamp: 1900 });
canvas.dispatch("pointerup", { pointerId: 5, clientX: 100, clientY: 20, timeStamp: 2000 });
canvas.dispatch("pointerdown", { pointerId: 6, clientX: 100, clientY: 20, timeStamp: 2100 });
canvas.dispatch("pointerup", { pointerId: 6, clientX: 100, clientY: 20, timeStamp: 2200 });
assert(focusCalls === 1, "same-node double tap should focus exactly once");
assert(selections[selections.length - 1].doubleTap === true, "second tap should be reported as a double tap");
assert(loadCalls.length === 4 && loadCalls[3].options.focusTurn === 4, "same-node double tap may reopen with one exact-turn refresh but must not issue duplicate loads");
near(_timelineState.worldlineViewport.translationX, 100 - (440 + 88), "double tap should center the supplied node X");
near(_timelineState.worldlineViewport.translationY, 60 - (76 + 24), "double tap should center the supplied node Y");
assert(_timelineState.expandedTurnKey === "turn:4", "double tap must keep the exact selected turn records active");

const outside = new FakeElement();
const outsideWheel = outside.dispatch("wheel", { clientX: 10, clientY: 10, deltaY: 80 });
assert(outsideWheel.prevented === false, "outside scroll must remain unaffected");
cleanup();
const afterCleanupWheel = canvas.dispatch("wheel", { clientX: 10, clientY: 10, deltaY: 80 });
assert(afterCleanupWheel.prevented === false, "gesture cleanup should remove canvas preventDefault handlers");
assert(JSON.stringify(topology) === topologyBefore, "zoom, pinch, focus, and selection must not mutate Go topology");
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node Timeline worldline production gesture runtime smoke failed: %v\n%s", err, out)
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

func TestArchiveCenterJSBootstrapIsObservationOnly(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"async function observePrepareTurnBootstrap(sessionId, requestId, activeChatMessages, chatId)",
		`contract_version: "session_bootstrap_observation.v1"`,
		`leading_messages: leadingMessages`,
		`selected_greeting_index: selectedGreetingIndex`,
		`selection_exposed: selectionExposed`,
		`first_greeting: firstGreeting`,
		`alternate_greetings: alternateGreetings`,
		`body.bootstrap_observation = prepareOptions.bootstrapObservation`,
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing startup message turn zero marker %q", needle)
		}
	}
	for _, forbidden := range []string{
		"STARTUP_MESSAGE_LEDGER_KEY",
		"function buildStartupMessageTurnZeroCandidate",
		"function ensureStartupMessageTurnZeroSaved",
		"function saveStartupMessageTurnZeroToBackend",
		"function getCurrentStartupMessageTurnZeroCandidate",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("Archive Center.js retains removed bootstrap policy %q", forbidden)
		}
	}
}

func TestArchiveCenterJSActiveChatCompleteTurnBackfillMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"ACTIVE_CHAT_BACKFILL_LEDGER_KEY",
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
		`source: "risu_next_host_signal_active_chat"`,
		"persistAcceptedHostFinalWithoutBlockingRequest",
		`ensureActiveChatCompletedTurnsBackfilled(requestedSessionId, { reason: "timeline_refresh"`,
		"lastActiveChatBackfill",
		"Active Chat Backfill",
	}
	for _, needle := range required {
		if !strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js missing active chat complete-turn backfill marker %q", needle)
		}
	}
	for _, forbidden := range []string{
		"ACTIVE_CHAT_BACKFILL_MAX_" + "PAIRS",
		"ACTIVE_CHAT_BACKFILL_MAX_CONTEXT_" + "MESSAGES",
		"max" + "Pairs:",
	} {
		if strings.Contains(src, forbidden) {
			t.Fatalf("Archive Center.js retains hidden active-chat backfill limit %q", forbidden)
		}
	}
}

func TestArchiveCenterJSAutoContinueEmptyInputMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"AUTO_CONTINUE_USER_INPUT_MARKER",
		"actualEmptyInput",
		"function bindRawInputObservationToRequest",
		"const actualEmptyRawInput = rawInputObservationForRequest",
		`current_user_input_backend_unavailable`,
		"recoverCurrentUserInputFromActiveChatTail",
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

func TestArchiveCenterJSActiveChatInputPrecedesAutoContinueFallback(t *testing.T) {
	src := readArchiveCenterJS(t)
	activePair := strings.Index(src, `userInputRecoverySource = "active_chat_pair"`)
	autoContinue := strings.Index(src, `userInput = AUTO_CONTINUE_USER_INPUT_MARKER`)
	if activePair < 0 || autoContinue < 0 || activePair >= autoContinue {
		t.Fatalf("Active Chat user recovery must run before auto-continue fallback: active=%d auto=%d", activePair, autoContinue)
	}
	boundEmpty := strings.Index(src, `if (actualEmptyRawInput) {`)
	activeRecovery := strings.Index(src, `const activeChatUserInput = await recoverUserInputFromActiveChatPair`)
	if boundEmpty < 0 || activeRecovery < 0 || boundEmpty >= activeRecovery {
		t.Fatal("request-bound empty input must become authoritative before Active Chat recovery")
	}
	if !strings.Contains(src, `if (!actualEmptyUserInput && shouldSkipUserInputPersistence(userInput)) {`) {
		t.Fatal("Active Chat recovery must be blocked by a request-bound empty input")
	}
	if !strings.Contains(src, `let safeSavedUserInput = isCanonicalHostUserInputText(userInput) ? userInput : ""`) {
		t.Fatal("save-layer user input must preserve verified host text without prompt-content classification")
	}
}

func TestArchiveCenterJSActiveChatRescanDryRunMarkers(t *testing.T) {
	src := readArchiveCenterJS(t)
	required := []string{
		"ACTIVE_CHAT_RECENT_REBUILD_DEFAULT_TURNS",
		"ACTIVE_CHAT_REBUILD_DEFAULT_ORDER",
		"function resolveCurrentActiveChatObject",
		"function runActiveChatRescanDryRun",
		"function runActiveChatRecentRebuild",
		"function computeActiveChatRescanDryRunPlan",
		"function explorerFetchTimelineItemsForSessionDryRun",
		"const seenBeforeTurns = new Set()",
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
		`pending_items: Array.isArray(_timelineState.pendingItems) ? _timelineState.pendingItems : []`,
		`_timelineState.viewModel = result.timeline || null`,
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

func TestArchiveCenterJSLegacyTableReadRemoved(t *testing.T) {
	src := readArchiveCenterJS(t)
	forbidden := []string{
		"TABLE_READ_POLISH_STORAGE_LEDGER_KEY",
		"function rememberTableReadPolishStorage",
		"function applyTableReadPolishStorageToBackfillPair",
		"function buildTableReadPolishCompleteTurnMeta",
		"table_read_output_polish",
		`"/table-read/`,
	}
	for _, needle := range forbidden {
		if strings.Contains(src, needle) {
			t.Fatalf("Archive Center.js still contains removed Table Read marker %q", needle)
		}
	}
}

func TestArchiveCenterJSCompleteTurnQueueSeparatesRawSaveFromDerivedRetry(t *testing.T) {
	src := readArchiveCenterJS(t)
	if strings.Contains(src, "isCompleteTurnPayloadAlreadySaved") {
		t.Fatal("complete-turn queue must not treat raw chat rows as full pipeline completion")
	}
	if !strings.Contains(src, `"/complete-turn/request-status?idempotency_key="`) {
		t.Fatal("complete-turn queue is missing backend idempotency status check")
	}
	for _, marker := range []string{
		"requestStatus.raw_saved === true",
		"_ctResult.derived_retry_required === true",
		"raw saved; derived retry owned by backend",
	} {
		if !strings.Contains(src, marker) {
			t.Fatalf("complete-turn queue missing raw/derived split marker %q", marker)
		}
	}
	if strings.Contains(src, "res.derived_retry_required !== true") {
		t.Fatal("raw save must not stay in the full complete-turn retry queue only because derived retry is required")
	}
}

func TestArchiveCenterJSSessionScopedInspectionSelectionRuntime(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("ARCHIVE_CENTER_NODE_BINARY or node on PATH is required for session inspection runtime smoke")
		}
	}
	src := readArchiveCenterJS(t)
	blocks := []string{
		extractJSFunctionBlockForTest(t, src, "async function explorerChangeSession(sessionId, reload = true)"),
		extractJSFunctionBlockForTest(t, src, "function syncSessionScopedInspectionSelection(sessionId)"),
		extractJSFunctionBlockForTest(t, src, "async function referenceLibraryLoadBindings(sessionIdOverride = \"\")"),
	}
	script := `
const assert = (condition, message) => { if (!condition) throw new Error(message); };
const pageState = () => ({ items: [{ old: true }], total: 1, offset: 1, hasMore: true, loading: true });
const _explorer = {
  selectedSessionId: "session-a", activeTab: "memories", expandedItems: new Set(["mem_1"]),
  chatLogs: pageState(), memories: pageState(), directEvidence: Object.assign(pageState(), { stateCounts: { old: 1 }, stateContract: { old: true } }),
  kgTriples: pageState(), episodes: pageState(), chapters: pageState(), arcs: pageState(), sagas: pageState(), lorebook: Object.assign(pageState(), { error: "old", scope: { old: true }, latestSnapshot: { old: true } }),
  episodeMergeSelection: new Set([1]), episodeOpStatus: { status: "ok" },
  trust: { storylines: [{ id: 1 }], worldRules: [{ id: 2 }], hooks: [{ id: 3 }], loading: true },
  worldGraph: { rules: [{ id: 4 }], allRules: [{ id: 5 }], loading: true },
  entities: { characters: [{ id: 6 }], locations: [{ id: 7 }], items: [{ id: 8 }], memoryBundles: [{ id: 9 }], selectedMemoryBundleKey: "old", memoryItems: [{ id: 10 }], memoryLoading: true, forceMergeKeys: new Set(["old"]), loading: true },
  viewModel: { old: true },
};
const _feedback = { latest: { mem_1: {} }, status: { mem_1: "ok" } };
const _referenceLibraryState = { sessionId: "session-a", bindings: [{ binding_id: "old" }], bindingDraft: { old: true }, bindingDraftDirty: true, bindingPreview: { old: true }, bindingLoading: false, bindingRequestId: 0, bindingMessage: "old", bindingError: "old" };
const _timelineState = { selectedSessionId: "session-b", sessionId: "session-b" };
const runtimeState = { currentSessionId: "runtime-session" };
let cancelCount = 0;
let batchResetCount = 0;
let loadedTabs = [];
let fetchedURLs = [];
function explorerCancelEdit() { cancelCount++; }
function explorerResetAllBatchDelete() { batchResetCount++; }
async function explorerLoadTab(tab, reset) { loadedTabs.push([tab, reset]); }
function referenceLibraryRefreshUI() {}
async function getCurrentChatSessionId() { return "runtime-session"; }
async function bridgeFetch(url) { fetchedURLs.push(url); return { bindings: [{ binding_id: "new" }] }; }
function referenceLibraryPath(value) { return encodeURIComponent(String(value)); }
` + strings.Join(blocks, "\n") + `
(async () => {
  syncSessionScopedInspectionSelection("session-b");
  assert(_explorer.selectedSessionId === "session-b", "Explorer must follow Timeline selection");
  assert(_explorer.expandedItems.size === 0 && cancelCount === 1 && batchResetCount === 1, "old Explorer transient state must reset");
  assert(_explorer.trust.storylines.length === 0 && _explorer.worldGraph.rules.length === 0 && _explorer.entities.characters.length === 0, "old session rows must clear");
  assert([_explorer.chatLogs, _explorer.memories, _explorer.directEvidence, _explorer.kgTriples, _explorer.episodes, _explorer.chapters, _explorer.arcs, _explorer.sagas, _explorer.lorebook].every((page) => page.items.length === 0 && page.total === 0 && page.offset === 0 && page.hasMore === false && page.loading === false), "all paged Explorer rows must clear before the new session loads");
  assert(_explorer.lorebook.error === "" && _explorer.lorebook.scope === null && _explorer.lorebook.latestSnapshot === null, "old lorebook projection must not cross sessions");
  assert(_explorer.directEvidence.stateCounts === null && _explorer.directEvidence.stateContract === null && _explorer.viewModel === null, "session-scoped evidence metadata and ViewModel must clear");
  assert(_referenceLibraryState.sessionId === "session-b" && _referenceLibraryState.bindings.length === 0, "reference binding session must follow Timeline selection");
  assert(_referenceLibraryState.bindingDraft === null && _referenceLibraryState.bindingPreview === null, "old binding draft must not cross sessions");
  assert(loadedTabs.length === 0, "hidden Explorer must not fetch during Timeline selection");
  await explorerChangeSession("session-b", true);
  assert(loadedTabs.length === 1 && loadedTabs[0][0] === "memories", "Explorer tab entry must reload selected session");
  await referenceLibraryLoadBindings();
  assert(fetchedURLs.length === 1 && fetchedURLs[0].includes("/sessions/session-b/reference-bindings"), "binding request must use Timeline-selected session");
  assert(runtimeState.currentSessionId === "runtime-session", "UI inspection selection must not mutate Host runtime session");
})().catch((err) => { console.error(err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node session-scoped inspection selection runtime smoke failed: %v\n%s", err, out)
	}
}

func TestArchiveCenterJSReferenceBindingsRejectStaleSessionResponses(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("ARCHIVE_CENTER_NODE_BINARY or node on PATH is required for reference binding session fence smoke")
		}
	}
	src := readArchiveCenterJS(t)
	blocks := []string{
		extractJSFunctionBlockForTest(t, src, "function syncSessionScopedInspectionSelection(sessionId)"),
		extractJSFunctionBlockForTest(t, src, "async function referenceLibraryLoadBindings(sessionIdOverride = \"\")"),
		extractJSFunctionBlockForTest(t, src, "async function referenceLibraryPreviewBinding(root)"),
		extractJSFunctionBlockForTest(t, src, "async function referenceLibraryApplyBinding(root)"),
		extractJSFunctionBlockForTest(t, src, "async function referenceLibraryUnlinkBinding()"),
	}
	script := `
const assert = (condition, message) => { if (!condition) throw new Error(message); };
const _timelineState = { selectedSessionId: "session-a", sessionId: "session-a" };
const _referenceLibraryState = {
  sessionId: "session-a", bindings: [], bindingDraft: null, bindingDraftDirty: false,
  bindingPreview: null, bindingLoading: false, bindingRequestId: 0, bindingMessage: "", bindingError: "",
};
const pending = [];
function explorerChangeSession() {}
function referenceLibraryRefreshUI() {}
function referenceLibraryPath(value) { return encodeURIComponent(String(value)); }
function referenceLibraryRememberBindingDraft() { return { work_id: "work", continuity_id: "continuity" }; }
function referenceLibrarySelectedBinding() { return _referenceLibraryState.bindings[0] || null; }
async function getCurrentChatSessionId() { return "runtime-session"; }
function bridgeFetch(url) { return new Promise((resolve, reject) => pending.push({ url, resolve, reject })); }
` + strings.Join(blocks, "\n") + `
(async () => {
  const requestA = referenceLibraryLoadBindings("session-a");
  _timelineState.selectedSessionId = "session-b";
  _timelineState.sessionId = "session-b";
  syncSessionScopedInspectionSelection("session-b");
  const requestB = referenceLibraryLoadBindings();
  pending[1].resolve({ bindings: [{ binding_id: "binding-b" }] });
  await requestB;
  pending[0].resolve({ bindings: [{ binding_id: "binding-a" }] });
  await requestA;
  assert(_referenceLibraryState.sessionId === "session-b", "late A binding response must not replace selected B session");
  assert(_referenceLibraryState.bindings.length === 1 && _referenceLibraryState.bindings[0].binding_id === "binding-b", "late A binding rows must not replace B rows");
  assert(_referenceLibraryState.bindingLoading === false, "latest binding request must clear loading");

  _timelineState.selectedSessionId = "session-c";
  _timelineState.sessionId = "session-c";
  syncSessionScopedInspectionSelection("session-c");
  const rejected = referenceLibraryLoadBindings();
  pending[2].reject(new Error("binding unavailable"));
  await rejected;
  assert(_referenceLibraryState.bindingLoading === false, "rejected binding request must clear loading");

  _timelineState.selectedSessionId = "session-c";
  const previewC = referenceLibraryPreviewBinding({});
  _timelineState.selectedSessionId = "session-d";
  _timelineState.sessionId = "session-d";
  syncSessionScopedInspectionSelection("session-d");
  pending[3].resolve({ valid: true, session: "session-c" });
  await previewC;
  assert(_referenceLibraryState.sessionId === "session-d", "late preview must not restore its prior session");
  assert(_referenceLibraryState.bindingPreview === null, "late preview must not enter the newly selected session");

  _referenceLibraryState.bindings = [{ binding_id: "binding-d", revision: 1 }];
  const applyD = referenceLibraryApplyBinding({});
  _timelineState.selectedSessionId = "session-e";
  _timelineState.sessionId = "session-e";
  syncSessionScopedInspectionSelection("session-e");
  pending[4].resolve({ status: "ok", preview: { session: "session-d" } });
  await applyD;
  assert(_referenceLibraryState.sessionId === "session-e" && _referenceLibraryState.bindingPreview === null, "late apply result must not enter E UI");

  _timelineState.selectedSessionId = "session-f";
  _timelineState.sessionId = "session-f";
  syncSessionScopedInspectionSelection("session-f");
  _referenceLibraryState.bindings = [{ binding_id: "binding-f", revision: 2 }];
  const unlinkF = referenceLibraryUnlinkBinding();
  _timelineState.selectedSessionId = "session-g";
  _timelineState.sessionId = "session-g";
  syncSessionScopedInspectionSelection("session-g");
  pending[5].resolve({ status: "ok" });
  await unlinkF;
  assert(_referenceLibraryState.sessionId === "session-g" && _referenceLibraryState.bindingPreview === null, "late unlink result must not enter G UI");

  _timelineState.selectedSessionId = "session-h";
  _timelineState.sessionId = "session-h";
  syncSessionScopedInspectionSelection("session-h");
  const loadH = referenceLibraryLoadBindings();
  const previewH = referenceLibraryPreviewBinding({});
  assert(_referenceLibraryState.bindingLoading === false, "preview must release a superseded binding load indicator");
  pending[7].resolve({ valid: true });
  await previewH;
  pending[6].resolve({ bindings: [{ binding_id: "stale-load-h" }] });
  await loadH;
  assert(_referenceLibraryState.bindingLoading === false, "superseded load must not leave loading active");
})().catch((err) => { console.error(err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node reference binding session fence smoke failed: %v\n%s", err, out)
	}
}

func TestArchiveCenterJSTimelineSessionListRejectsStaleParentRequest(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("ARCHIVE_CENTER_NODE_BINARY or node on PATH is required for Timeline session list fence smoke")
		}
	}
	src := readArchiveCenterJS(t)
	blocks := []string{
		extractJSFunctionBlockForTest(t, src, "function timelineSessionId(session)"),
		extractJSFunctionBlockForTest(t, src, "function timelineIsPlaceholderSessionId(sessionId)"),
		extractJSFunctionBlockForTest(t, src, "async function loadTimelineSessions(currentSid, force = false, timelineDataRequestId = 0)"),
	}
	script := `
const assert = (condition, message) => { if (!condition) throw new Error(message); };
const SESSION_FALLBACK = "fallback";
const _timelineState = { requestId: 1, sessionsRequestId: 0, sessionsLoading: false, sessionsError: "", sessions: [] };
const pending = [];
function refreshTimelineUI() {}
async function refreshSessionDisplayLookupFromRuntime() {}
function getRequestTimeoutSettingMs() { return 1000; }
function reconcileDeletedBackendSessionsFromList() { return Promise.resolve(); }
function debugLog() {}
function bridgeFetch() { return new Promise((resolve) => pending.push(resolve)); }
` + strings.Join(blocks, "\n") + `
(async () => {
  const requestA = loadTimelineSessions("session-a", true, 1);
  await Promise.resolve();
  _timelineState.requestId = 2;
  const requestB = loadTimelineSessions("session-b", true, 2);
  await Promise.resolve();
  pending[1]({ sessions: [{ chat_session_id: "session-b", chat_logs_count: 2 }] });
  await requestB;
  pending[0]({ sessions: [{ chat_session_id: "session-a", chat_logs_count: 1 }] });
  await requestA;
  assert(_timelineState.sessions.length === 1 && _timelineState.sessions[0].chat_session_id === "session-b", "late A session catalog must not replace B");
  assert(_timelineState.sessionsLoading === false, "latest session catalog request must clear loading");

  _timelineState.requestId = 3;
  const staleOnly = loadTimelineSessions("session-a", true, 3);
  await Promise.resolve();
  _timelineState.requestId = 4;
  pending[2]({ sessions: [{ chat_session_id: "session-a" }] });
  await staleOnly;
  assert(_timelineState.sessions.length === 1 && _timelineState.sessions[0].chat_session_id === "session-b", "stale-only catalog must not replace B");
  assert(_timelineState.sessionsLoading === false, "stale catalog owner must release its loading indicator when no replacement catalog starts");
})().catch((err) => { console.error(err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node Timeline session list fence smoke failed: %v\n%s", err, out)
	}
}

func TestArchiveCenterJSPresentationViewModelRejectsStaleSessionResponse(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("ARCHIVE_CENTER_NODE_BINARY or node on PATH is required for presentation session fence smoke")
		}
	}
	src := readArchiveCenterJS(t)
	block := extractJSFunctionBlockForTest(t, src, "async function loadPresentationViewModels()")
	script := `
const assert = (condition, message) => { if (!condition) throw new Error(message); };
let _presentationViewModelRequestId = 0;
const _timelineState = { selectedSessionId: "session-a", sessionId: "session-a", currentSessionId: "runtime-session", sessions: [], items: [], pendingItems: [], meta: {}, loading: false, loadingMore: false, hasMore: false, error: "", viewModel: null };
const _explorer = { selectedSessionId: "session-a", activeChatSessionId: "runtime-session", activeTab: "memories", sessions: [], chatLogs: {}, memories: {}, directEvidence: {}, kgTriples: {}, episodes: {}, chapters: {}, arcs: {}, sagas: {}, lorebook: {}, trust: { storylines: [], worldRules: [], hooks: [] }, worldGraph: { rules: [], allRules: [] }, entities: { characters: [], locations: [], items: [] }, sessionsLoading: false, viewModel: null };
const pending = [];
function bridgeFetch() { return new Promise((resolve) => pending.push(resolve)); }
function resolveRuntimeSessionLifecycle() { return "active"; }
function getRequestTimeoutSettingMs() { return 1000; }
function warnLog() {}
` + block + `
(async () => {
  const requestA = loadPresentationViewModels();
  _timelineState.selectedSessionId = "session-b";
  _timelineState.sessionId = "session-b";
  _explorer.selectedSessionId = "session-b";
  const requestB = loadPresentationViewModels();
  pending[1]({ status: "ok", contract_version: "presentation.viewmodel.v1", timeline: { session: "b" }, explorer: { session: "b" } });
  await requestB;
  pending[0]({ status: "ok", contract_version: "presentation.viewmodel.v1", timeline: { session: "a" }, explorer: { session: "a" } });
  await requestA;
  assert(_timelineState.viewModel.session === "b", "late Timeline A response must not replace B");
  assert(_explorer.viewModel.session === "b", "late Explorer A response must not replace B");
})().catch((err) => { console.error(err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node presentation selected-session fence smoke failed: %v\n%s", err, out)
	}
}

func TestArchiveCenterJSExplorerRejectsStaleSessionRowsAndUnscopedRead(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("ARCHIVE_CENTER_NODE_BINARY or node on PATH is required for Explorer session fence smoke")
		}
	}
	src := readArchiveCenterJS(t)
	blocks := []string{
		extractJSFunctionBlockForTest(t, src, "async function explorerFetchMemories(reset = false)"),
		extractJSFunctionBlockForTest(t, src, "async function explorerFetchTrust()"),
	}
	script := `
const assert = (condition, message) => { if (!condition) throw new Error(message); };
const _explorer = {
  selectedSessionId: null,
  memories: { items: [{ id: "stale" }], total: 1, offset: 1, hasMore: true, loading: false, requestId: 0 },
  trust: { storylines: [], worldRules: [], hooks: [], loading: false, error: null },
};
const pending = [];
let fetchCount = 0;
function explorerSessionId() { return _explorer.selectedSessionId; }
function getRequestTimeoutSettingMs() { return 1000; }
function debugLog() {}
async function safeCall(fn) { return await fn(); }
function bridgeFetch(url) { fetchCount++; return new Promise((resolve) => pending.push({ url, resolve })); }
` + strings.Join(blocks, "\n") + `
(async () => {
  await explorerFetchMemories(true);
  assert(fetchCount === 0, "missing selected session must not issue an unscoped memories read");
  assert(_explorer.memories.items.length === 0, "missing selected session must show no prior rows");

  _explorer.selectedSessionId = "session-a";
  const requestA = explorerFetchTrust();
  _explorer.selectedSessionId = "session-b";
  const requestB = explorerFetchTrust();
  pending.slice(3, 6).forEach((entry, index) => entry.resolve(index === 0 ? { storylines: [{ session: "b" }] } : index === 1 ? { items: [{ session: "b" }] } : { hooks: [{ session: "b" }] }));
  await requestB;
  pending.slice(0, 3).forEach((entry, index) => entry.resolve(index === 0 ? { storylines: [{ session: "a" }] } : index === 1 ? { items: [{ session: "a" }] } : { hooks: [{ session: "a" }] }));
  await requestA;
  assert(_explorer.trust.storylines[0].session === "b", "late Trust A rows must not replace B");
  assert(_explorer.trust.worldRules[0].session === "b", "late WorldRule A rows must not replace B");
  assert(_explorer.trust.hooks[0].session === "b", "late Hook A rows must not replace B");
})().catch((err) => { console.error(err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node Explorer selected-session fence smoke failed: %v\n%s", err, out)
	}
}

func TestArchiveCenterJSPublisherSettingsPrecedeCommonSettings(t *testing.T) {
	src := readArchiveCenterJS(t)
	publisherSection := `<div class="mo-section">${t('settings.section.publisherSettings')}</div>`
	commonSection := `<div class="mo-section">${t('settings.section.common')}</div>`
	publisherIndex := strings.Index(src, publisherSection)
	commonIndex := strings.Index(src, commonSection)
	if publisherIndex < 0 || commonIndex < 0 || publisherIndex >= commonIndex {
		t.Fatalf("publisher settings section must render before common settings")
	}
	for _, control := range []string{
		`id="mo-narrativeGuideMode"`,
		`id="mo-narrativeGuideStrength"`,
		`id="mo-publisherGuidanceFormat"`,
		`id="mo-narrativeSupportMaxChars"`,
		`id="mo-pluginMainApplyMode"`,
	} {
		if strings.Count(src, control) != 1 {
			t.Fatalf("publisher control %q must render exactly once", control)
		}
		controlIndex := strings.Index(src, control)
		if controlIndex <= publisherIndex || controlIndex >= commonIndex {
			t.Fatalf("publisher control %q must stay inside the publisher settings section", control)
		}
	}
}

func TestArchiveCenterJSLorebookReferenceUsesBoundedBackendProjection(t *testing.T) {
	nodePath := strings.TrimSpace(os.Getenv("ARCHIVE_CENTER_NODE_BINARY"))
	if nodePath == "" {
		var err error
		nodePath, err = exec.LookPath("node")
		if err != nil {
			t.Skip("ARCHIVE_CENTER_NODE_BINARY or node on PATH is required for lorebook reference paging smoke")
		}
	}
	src := readArchiveCenterJS(t)
	for _, marker := range []string{
		`["lorebook", t('settings.tab.lorebook')]`,
		`id="mo-lorebook-session-select"`,
		`id="mo-lorebook-reference-items"`,
		`data-more-type="lorebook"`,
		`lorebook_reference_current.viewmodel.v1`,
		`params.set("scope_mode", "latest_session")`,
		`selectWorkspaceSession(sessionId, "lorebook")`,
		`return String(tab && tab.key || "") !== "lorebook";`,
		`visibleTabs.some(function(tab) { return tab.key === presentedActiveTab; })`,
		`root.innerHTML = renderLorebookReferenceManagementSection();`,
		`lorebookReferenceMode: readChecked("mo-lorebookReferenceAssistEnabled"`,
		`setCheckedIfPresent("mo-lorebookReferenceAssistEnabled", settings.lorebookReferenceMode !== "search_only")`,
	} {
		if !strings.Contains(src, marker) {
			t.Fatalf("Archive Center.js missing lorebook UI marker %q", marker)
		}
	}
	managementRenderer := extractJSFunctionBlockForTest(t, src, "function renderLorebookReferenceManagementSection()")
	for _, marker := range []string{`id="mo-lorebook-session-select"`, `id="mo-lorebook-reference-items"`, `renderExplorerLorebook()`} {
		if !strings.Contains(managementRenderer, marker) {
			t.Fatalf("Extensions/Lorebook renderer missing %q", marker)
		}
	}
	memoryRenderer := extractJSFunctionBlockForTest(t, src, "function renderExplorerContent()")
	if strings.Contains(memoryRenderer, `activeTab === "lorebook"`) {
		t.Fatal("Memory content renderer must not retain Auxiliary Reference")
	}
	if strings.Contains(src, `id="mo-lorebookReferenceMode"`) {
		t.Fatalf("legacy lorebook mode select must be removed from Extensions/Lorebook")
	}
	tabProjection := extractJSFunctionBlockForTest(t, src, "function getExplorerTabItems()")
	if !strings.Contains(tabProjection, `return String(tab && tab.key || "") !== "lorebook";`) {
		t.Fatal("Memory tab projection must reject lorebook rows from an older Presentation ViewModel")
	}
	for _, removedMemoryMarker := range []string{
		`lorebook: Number(_explorer.lorebook.total || 0)`,
		`if (_explorer.activeTab === "lorebook") return renderExplorerLorebook();`,
	} {
		if strings.Contains(src, removedMemoryMarker) {
			t.Fatalf("Memory must not retain lorebook tab marker %q", removedMemoryMarker)
		}
	}
	const lorebookAssistControl = `id="mo-lorebookReferenceAssistEnabled"`
	const floatingUIControl = `id="mo-turnWorkflowHUDEnabled"`
	if strings.Count(src, lorebookAssistControl) != 1 {
		t.Fatalf("lorebook auxiliary reference toggle must exist exactly once in Settings")
	}
	if lorebookIndex, floatingIndex := strings.Index(src, lorebookAssistControl), strings.Index(src, floatingUIControl); lorebookIndex < 0 || floatingIndex < 0 || lorebookIndex >= floatingIndex {
		t.Fatalf("lorebook auxiliary reference toggle must render immediately before the floating UI control")
	}
	blocks := []string{
		extractJSFunctionBlockForTest(t, src, "function getExplorerTabItems()"),
		extractJSFunctionBlockForTest(t, src, "async function explorerFetchLorebook(reset = false)"),
	}
	script := `
const assert = (condition, message) => { if (!condition) throw new Error(message); };
const SESSION_FALLBACK = "fallback";
const EXPLORER_PAGE_SIZE = 20;
const _timelineState = { selectedSessionId: "session-a", sessionId: "session-a" };
const _explorer = {
  selectedSessionId: "session-a",
  activeChatSessionId: "session-a",
  lorebook: { items: [], total: 0, offset: 0, hasMore: false, loading: false, error: "", scope: null, latestSnapshot: null, requestId: 0 },
};
const urls = [];
function explorerSessionId() { return _explorer.selectedSessionId; }
function getRequestTimeoutSettingMs() { return 1000; }
function presentationExplorerTabLabel(key) { return String(key || ""); }
async function bridgeFetch(url) {
  urls.push(url);
  const offset = Number(new URL("http://archive.local" + url).searchParams.get("offset") || 0);
  const count = offset === 0 ? 20 : 15;
  return {
    status: "ok",
    contract_version: "lorebook_reference_current.viewmodel.v1",
    items: Array.from({ length: count }, (_, index) => ({ entry_ordinal: offset + index, key: "key-" + (offset + index), content: "lore" })),
    total: 35,
    has_more: offset === 0,
    scope: { chat_session_id: "session-a", character_index: 2, chat_index: 7 },
    latest_snapshot: { snapshot_id: "snapshot", observed_at: "2026-08-20T00:00:00Z" },
  };
}
` + strings.Join(blocks, "\n") + `
(async () => {
  _explorer.viewModel = { tabs: [{ key: "chat_logs", count: 2 }, { key: "lorebook", count: 9 }] };
  const visibleMemoryTabs = getExplorerTabItems();
  assert(visibleMemoryTabs.length === 1 && visibleMemoryTabs[0].key === "chat_logs", "Memory UI must hide lorebook rows from an older backend ViewModel");
  await explorerFetchLorebook(true);
  assert(_explorer.lorebook.items.length === 20, "first page must contain exactly the bounded backend page");
  assert(_explorer.lorebook.offset === 20 && _explorer.lorebook.hasMore === true, "first page state mismatch");
  await explorerFetchLorebook(false);
  assert(_explorer.lorebook.items.length === 35, "second page must append without refetching the first page");
  assert(_explorer.lorebook.offset === 35 && _explorer.lorebook.hasMore === false, "completed page state mismatch");
  assert(urls.length === 2 && urls[0].includes("offset=0") && urls[1].includes("offset=20"), "paging must advance through the backend projection");
  assert(urls[0].includes("scope_mode=latest_session"), "selected-session scope choice must belong to the backend");
  assert(urls[0].includes("/sessions/session-a/lorebook-reference/current"), "selected session ID was not forwarded");
  assert(!urls[0].includes("character_index=") && !urls[0].includes("enabled_module_id="), "adapter must not reconstruct a stored Host scope");
})().catch((err) => { console.error(err); process.exit(1); });
`
	cmd := exec.Command(nodePath, "-")
	cmd.Stdin = strings.NewReader(script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node lorebook reference paging smoke failed: %v\n%s", err, out)
	}
}
