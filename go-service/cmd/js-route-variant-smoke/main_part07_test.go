package main

import (
	"os/exec"
	"strings"
	"testing"
)

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
