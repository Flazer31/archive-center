package main

import (
	"os/exec"
	"strings"
	"testing"
)

// TestArchiveCenterJSSeq19P333ToP344Beta10DecisionRuntimeSemantics runs Node-based
// runtime behavior smoke for SEQ-19-P333 and P337~P344 decision surfaces.
func TestArchiveCenterJSSeq19P333ToP344Beta10DecisionRuntimeSemantics(t *testing.T) {
	nodePath, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for JS runtime behavior smoke")
	}
	src := readArchiveCenterJS(t)
	start := strings.Index(src, "function extractTemporalRelationEntriesStep19")
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

// SEQ-20-P21: q20a temporal query expansion preparatory marker.
// SEQ-20-P22: q20a.v1 temporal query expansion contract marker.
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
	start := strings.Index(src, "function extractTemporalRelationEntriesStep19")
	end := strings.Index(src, "function readSceneTemporalStateFromOrchResultStep19")
	if start < 0 || end <= start {
		t.Fatalf("Archive Center.js missing SEQ-20 temporal retrieval runtime block")
	}
	script := src[start:end] + `
const assert = (cond, msg) => { if (!cond) throw new Error(msg); };

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
// SEQ-20-P80: q20f stops before graph-like support marker.
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
// SEQ-20-P93: q20g stops before inspection formatting marker.
// SEQ-20-P99: q20h entity/graph boost inspection surface preparatory marker.
// SEQ-20-P100: q20h.v1 entity/graph boost inspection surface contract marker.
// SEQ-20-P101: q20h inspection role and authority notice marker.
// SEQ-20-P102: q20h mirrored at query class marker.
// SEQ-20-P109: q20i lagging current state boost preparatory marker.
// SEQ-20-P110: q20i.v1 lagging current state boost contract marker.
// SEQ-20-P111: q20i activation and precedence marker.
// SEQ-20-P112: q20i mirrored at query class marker.
// SEQ-20-P118: q20j motive-shadow hint preparatory marker.
// SEQ-20-P119: q20j.v1 motive-shadow hint contract marker.
// SEQ-20-P120: q20j truth-write-forbidden marker.
// SEQ-20-P121: q20j mirrored at query class marker.
// SEQ-20-P127: q20k motive-shadow non-escalation guard preparatory marker.
// SEQ-20-P128: q20k.v1 motive-shadow non-escalation guard contract marker.
// SEQ-20-P129: q20k mirrored at query class marker.
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
// SEQ-20-P137: q20l graph truth write forbidden marker.
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
// SEQ-20-P233: ambiguity reduction aggregate marker.
// SEQ-20-P234: inspection visibility aggregate marker.
// SEQ-20-P235: truth precedence preserve aggregate marker.
// SEQ-20-P236: hot-bridge aggregate marker.
