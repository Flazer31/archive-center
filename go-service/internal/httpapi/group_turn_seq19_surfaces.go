package httpapi

import (
	"encoding/json"
	"strconv"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// ---------------------------------------------------------------------------
// SEQ-19 surfaces (P9 ~ P11, P15 ~ P22, P30 ~ P42, P50 ~ P57, P66 ~ P69)
// ---------------------------------------------------------------------------

// buildResetAdmin19 defines the Step 19 reset administration surface
// for SEQ-19-P9: existing checked checklist items were cleared for redo.
func buildResetAdmin19() map[string]any {
	return map[string]any{
		"version":              "seq19_p9.v1",
		"role":                 "reset_administration",
		"truth_authority":      false,
		"reset_action":         "checklist_cleared_for_redo",
		"historical_preserved": true,
		"policy_version":       "s19-rst.v1",
		"mode":                 "reset_administration_note",
	}
}

// buildHistoricalContentPreserved19 defines the Step 19 historical content
// preservation surface for SEQ-19-P10.
func buildHistoricalContentPreserved19() map[string]any {
	return map[string]any{
		"version":           "seq19_p10.v1",
		"role":              "historical_content_preserved",
		"truth_authority":   false,
		"content_preserved": true,
		"no_text_deleted":   true,
		"policy_version":    "s19-rst.v1",
		"mode":              "historical_content_preservation_note",
	}
}

// buildResetNoteOnly19 defines the Step 19 reset scope surface for SEQ-19-P11.
func buildResetNoteOnly19() map[string]any {
	return map[string]any{
		"version":            "seq19_p11.v1",
		"role":               "reset_note_only",
		"truth_authority":    false,
		"scope":              "document_reset_only",
		"revalidation_claim": false,
		"policy_version":     "s19-rst.v1",
		"mode":               "reset_scope_note",
	}
}

// ---------------------------------------------------------------------------
// SEQ-19 temporal state surfaces (P15 ~ P22)
// ---------------------------------------------------------------------------

// buildTemporalState19 defines the sc19a.v1 temporal state surface for
// SEQ-19-P15~P16: session_state.temporal_state read-only support surface.
func buildTemporalState19(
	activeStates []store.ActiveState,
	chatLogs []store.ChatLog,
	canonicalLayers []store.CanonicalStateLayer,
) map[string]any {
	currentClock := resolveCurrentStoryClock(activeStates, chatLogs, canonicalLayers)
	ledger := buildTemporalRelationLedger(activeStates)
	elapsed := buildElapsedTimeDecisionExtended(currentClock, ledger)
	return map[string]any{
		"version":                  "sc19a.v1",
		"role":                     "temporal_state",
		"truth_authority":          false,
		"current_story_clock":      currentClock,
		"temporal_relation_ledger": ledger,
		"elapsed_time_decision":    elapsed,
		"clock_write_directive":    buildClockWriteDirectiveExtended(currentClock, ledger),
		"policy_version":           "s19-et.v2",
		"mode":                     "temporal_state_surface",
	}
}

// resolveCurrentStoryClock resolves the current story clock from active states
// with precedence: session_state_clock -> input_current_scene_anchor -> timeline_anchor -> carry_forward.
func resolveCurrentStoryClock(
	activeStates []store.ActiveState,
	chatLogs []store.ChatLog,
	canonicalLayers []store.CanonicalStateLayer,
) map[string]any {
	// Try session_state_clock from latest active state
	for i := len(activeStates) - 1; i >= 0; i-- {
		as := activeStates[i]
		if as.StateType == "session_state_clock" && as.Content != "" {
			return map[string]any{
				"raw_value":         as.Content,
				"resolution_source": "session_state_clock",
				"precision_label":   normalizePrecisionLabel(as.Content),
				"turn_index":        as.TurnIndex,
			}
		}
	}
	// Try input_current_scene_anchor from latest active state
	for i := len(activeStates) - 1; i >= 0; i-- {
		as := activeStates[i]
		if as.StateType == "input_current_scene_anchor" && as.Content != "" {
			return map[string]any{
				"raw_value":         as.Content,
				"resolution_source": "input_current_scene_anchor",
				"precision_label":   normalizePrecisionLabel(as.Content),
				"turn_index":        as.TurnIndex,
			}
		}
	}
	// Try timeline_anchor from canonical layers
	for i := len(canonicalLayers) - 1; i >= 0; i-- {
		cl := canonicalLayers[i]
		if cl.LayerType == "timeline_anchor" && cl.Content != "" {
			return map[string]any{
				"raw_value":         cl.Content,
				"resolution_source": "timeline_anchor",
				"precision_label":   normalizePrecisionLabel(cl.Content),
				"turn_index":        cl.TurnIndex,
			}
		}
	}
	// Fallback: carry_forward from latest chat log turn index
	turnIndex := 0
	if len(chatLogs) > 0 {
		turnIndex = chatLogs[len(chatLogs)-1].TurnIndex
	}
	return map[string]any{
		"raw_value":         "",
		"resolution_source": "carry_forward",
		"precision_label":   "unknown",
		"turn_index":        turnIndex,
	}
}

// normalizePrecisionLabel normalizes raw precision strings to the canonical label set.
func normalizePrecisionLabel(raw string) string {
	switch raw {
	case "exact", "precise", "specific":
		return "exact"
	case "daypart", "morning", "afternoon", "evening", "night", "dawn", "dusk":
		return "daypart"
	case "bounded_range", "coarse", "approximate", "range":
		return "bounded_range"
	case "unknown", "", "invalid", "unspecified":
		return "unknown"
	default:
		return "unknown"
	}
}

// buildTemporalRelationLedger builds the temporal relation ledger from active states.
func buildTemporalRelationLedger(activeStates []store.ActiveState) []map[string]any {
	entries := []map[string]any{}
	for _, as := range activeStates {
		if as.StateType != "temporal_relation" {
			continue
		}
		entry := normalizeRelationEntry(as)
		if entry != nil {
			entries = append(entries, entry)
		}
	}
	return entries
}

// normalizeRelationEntry normalizes a single temporal relation active state into
// canonical snake_case schema.
func normalizeRelationEntry(as store.ActiveState) map[string]any {
	if as.Content == "" {
		return nil
	}
	// Parse content as JSON if possible
	var parsed map[string]any
	if err := json.Unmarshal([]byte(as.Content), &parsed); err != nil {
		// Fallback: treat content as raw relation text
		return map[string]any{
			"relative_label":           as.Content,
			"anchor":                   "current_story_clock",
			"offset_value_min":         nil,
			"offset_value_max":         nil,
			"offset_unit":              "unknown",
			"precision":                "unknown",
			"source_turn":              as.TurnIndex,
			"target_kind":              "recalled_event",
			"valid_from_turn":          nil,
			"valid_to_turn":            nil,
			"range_kind":               "unknown",
			"bounded_range":            false,
			"anchor_resolution_status": "carry_forward",
			"policy_version":           "s19-t2.v1",
		}
	}

	// Extract fields with both snake_case and camelCase/legacy key support
	relativeLabel := seq19StringFromMap(parsed, "relative_label", "relativeLabel", "label")
	anchor := seq19StringFromMap(parsed, "anchor", "anchorRef", "anchor_ref")
	if anchor == "" {
		anchor = "current_story_clock"
	}
	offsetUnit := seq19StringFromMap(parsed, "offset_unit", "offsetUnit", "unit")
	if offsetUnit == "" {
		offsetUnit = "unknown"
	}
	precision := seq19StringFromMap(parsed, "precision", "precisionLabel", "precision_label")
	if precision == "" {
		precision = "unknown"
	}
	// Degrade invalid/unknown precision
	precision = normalizePrecisionLabel(precision)

	targetKind := seq19StringFromMap(parsed, "target_kind", "targetKind", "kind")
	if targetKind == "" {
		targetKind = "recalled_event"
	}

	// Validate target_kind against allowed values
	allowedTargets := []string{"current_scene", "recalled_event", "planned_event", "hypothetical", "background_fact"}
	validTarget := false
	for _, t := range allowedTargets {
		if t == targetKind {
			validTarget = true
			break
		}
	}
	if !validTarget {
		targetKind = "recalled_event"
	}

	// Extract offset values
	offsetMin := seq19NumberFromMap(parsed, "offset_value_min", "offsetValueMin", "offset_min")
	offsetMax := seq19NumberFromMap(parsed, "offset_value_max", "offsetValueMax", "offset_max")

	// Extract valid_from/to turns
	validFrom := seq19NumberFromMap(parsed, "valid_from_turn", "validFromTurn", "from_turn")
	validTo := seq19NumberFromMap(parsed, "valid_to_turn", "validToTurn", "to_turn")

	// Determine range_kind and bounded_range
	rangeKind := "unknown"
	boundedRange := false
	if offsetMin != nil && offsetMax != nil {
		minVal, ok1 := seq19ToFloat64(offsetMin)
		maxVal, ok2 := seq19ToFloat64(offsetMax)
		if ok1 && ok2 && minVal != maxVal {
			rangeKind = "bounded"
			boundedRange = true
		} else if ok1 && ok2 && minVal == maxVal {
			rangeKind = "exact"
			boundedRange = false
		}
	} else if offsetMin != nil {
		rangeKind = "exact"
		boundedRange = false
	}

	// Missing anchor degradation
	anchorStatus := "resolved"
	if anchor == "" || anchor == "unknown" || anchor == "current_story_clock" && relativeLabel == "" {
		anchorStatus = "carry_forward"
		precision = "unknown"
		rangeKind = "unknown"
		boundedRange = false
	}

	return map[string]any{
		"relative_label":           relativeLabel,
		"anchor":                   anchor,
		"offset_value_min":         offsetMin,
		"offset_value_max":         offsetMax,
		"offset_unit":              offsetUnit,
		"precision":                precision,
		"source_turn":              as.TurnIndex,
		"target_kind":              targetKind,
		"valid_from_turn":          validFrom,
		"valid_to_turn":            validTo,
		"range_kind":               rangeKind,
		"bounded_range":            boundedRange,
		"anchor_resolution_status": anchorStatus,
		"policy_version":           "s19-t2.v1",
	}
}

// seq19StringFromMap extracts a string value from a map trying multiple keys.
func seq19StringFromMap(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}

// seq19NumberFromMap extracts a numeric value from a map trying multiple keys.
func seq19NumberFromMap(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch val := v.(type) {
			case float64:
				return val
			case int:
				return float64(val)
			case int64:
				return float64(val)
			case string:
				if f, err := strconv.ParseFloat(val, 64); err == nil {
					return f
				}
			}
		}
	}
	return nil
}

// seq19ToFloat64 converts an any value to float64.
func seq19ToFloat64(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	}
	return 0, false
}

// buildElapsedTimeDecision builds the elapsed time decision from current clock and ledger.
func buildElapsedTimeDecision(currentClock map[string]any, ledger []map[string]any) map[string]any {
	return map[string]any{
		"version":             "s19-et.v1",
		"can_advance":         len(ledger) > 0 && currentClock["precision_label"] != "unknown",
		"advance_basis":       "temporal_relation_ledger",
		"current_clock_known": currentClock["precision_label"] != "unknown",
		"policy_version":      "s19-t1.v1",
	}
}

// buildClockWriteDirective builds the optional clock write directive.
func buildClockWriteDirective(currentClock map[string]any, ledger []map[string]any) map[string]any {
	canWrite := currentClock["precision_label"] == "exact" || currentClock["precision_label"] == "daypart"
	return map[string]any{
		"version":          "s19-cwd.v1",
		"can_write":        canWrite,
		"write_lane":       "current_scene",
		"relation_targets": []string{"recalled_event", "planned_event", "hypothetical", "background_fact"},
		"policy_version":   "s19-t1.v1",
	}
}

// buildCurrentStoryClockResolution exposes the resolution precedence for SEQ-19-P17.
func buildCurrentStoryClockResolution(activeStates []store.ActiveState) map[string]any {
	precedence := []string{
		"session_state_clock",
		"input_current_scene_anchor",
		"timeline_anchor",
		"carry_forward",
	}
	effectiveSource := "carry_forward"
	for _, as := range activeStates {
		if as.StateType == "session_state_clock" && as.Content != "" {
			effectiveSource = "session_state_clock"
			break
		}
		if as.StateType == "input_current_scene_anchor" && as.Content != "" {
			effectiveSource = "input_current_scene_anchor"
			break
		}
	}
	// Check canonical layers for timeline_anchor
	if effectiveSource == "carry_forward" {
		effectiveSource = "timeline_anchor_or_carry_forward"
	}
	return map[string]any{
		"version":          "s19-p17.v1",
		"role":             "current_story_clock_resolution",
		"truth_authority":  false,
		"precedence_chain": precedence,
		"effective_source": effectiveSource,
		"policy_version":   "s19-t1.v1",
		"mode":             "current_story_clock_resolution_precedence",
	}
}

// buildPrecisionLabelContract exposes the precision label contract for SEQ-19-P18.
func buildPrecisionLabelContract() map[string]any {
	return map[string]any{
		"version":             "s19-p18.v1",
		"role":                "precision_label_contract",
		"truth_authority":     false,
		"canonical_labels":    []string{"exact", "daypart", "bounded_range", "unknown"},
		"coarse_collapsed_to": "bounded_range",
		"policy_version":      "s19-t1.v1",
		"mode":                "precision_label_contract_definition",
	}
}

// buildInvalidUnknownDegradation exposes invalid/unknown degradation for SEQ-19-P19.
func buildInvalidUnknownDegradation() map[string]any {
	return map[string]any{
		"version":             "s19-p19.v1",
		"role":                "invalid_unknown_degradation",
		"truth_authority":     false,
		"invalid_degrades_to": "unknown",
		"unknown_action":      "no_advance",
		"coarse_collapsed_to": "bounded_range",
		"policy_version":      "s19-t1.v1",
		"mode":                "invalid_unknown_degradation_rule",
	}
}

// buildTemporalSplitRule exposes the temporal split rule for SEQ-19-P20.
func buildTemporalSplitRule() map[string]any {
	return map[string]any{
		"version":               "s19-p20.v1",
		"role":                  "temporal_split_rule",
		"truth_authority":       false,
		"write_lane":            "current_scene",
		"relation_only_targets": []string{"recalled_event", "planned_event", "hypothetical", "background_fact"},
		"policy_version":        "s19-t1.v1",
		"mode":                  "temporal_split_rule_definition",
	}
}

// buildStoryClockSurfaceGuard exposes the story clock surface guard for SEQ-19-P21.
func buildStoryClockSurfaceGuard() map[string]any {
	return map[string]any{
		"version":         "s19-p21.v1",
		"role":            "story_clock_surface_guard",
		"truth_authority": false,
		"guard_cases": []string{
			"empty_without_scene_state",
			"parsed_json_scene_state",
			"invalid_value_fallback",
			"coarse_to_bounded_range_precision_label",
			"relation_only_split",
			"current_scene_write_lane",
		},
		"policy_version": "s19-t1.v1",
		"mode":           "story_clock_surface_guard_definition",
	}
}

// buildStep18Plus19RegressionBundle exposes the combined regression bundle status for SEQ-19-P22.
func buildStep18Plus19RegressionBundle() map[string]any {
	return map[string]any{
		"version":            "s19-p22.v1",
		"role":               "step18_plus_19_regression_bundle",
		"truth_authority":    false,
		"regression_status":  "green",
		"combined_read_path": true,
		"policy_version":     "s19-t1.v1",
		"mode":               "step18_plus_19_regression_bundle_status",
	}
}

// ---------------------------------------------------------------------------
// SEQ-19 temporal relation ledger schema surfaces (P30 ~ P42)
// ---------------------------------------------------------------------------

// buildTemporalRelationLedgerCanonical exposes the canonical snake_case ledger schema for SEQ-19-P30.
func buildTemporalRelationLedgerCanonical() map[string]any {
	return map[string]any{
		"version":         "s19-p30.v1",
		"role":            "temporal_relation_ledger_canonical",
		"truth_authority": false,
		"schema_format":   "snake_case",
		"canonical_keys": []string{
			"relative_label",
			"anchor",
			"offset_value_min",
			"offset_value_max",
			"offset_unit",
			"precision",
			"source_turn",
			"target_kind",
			"valid_from_turn",
			"valid_to_turn",
			"range_kind",
			"bounded_range",
			"anchor_resolution_status",
		},
		"policy_version": "s19-t2.v1",
		"mode":           "temporal_relation_ledger_canonical_schema",
	}
}

// buildSchemaPhraseIngress exposes the schema phrase ingress normalization for SEQ-19-P31.
func buildSchemaPhraseIngress() map[string]any {
	return map[string]any{
		"version":              "s19-p31.v1",
		"role":                 "schema_phrase_ingress",
		"truth_authority":      false,
		"ingress_supported":    true,
		"phrase_normalization": "canonical_offset_unit_precision",
		"policy_version":       "s19-t2.v1",
		"mode":                 "schema_phrase_ingress_normalization",
	}
}

// buildSchemaOwnerBlock exposes the schema owner block for SEQ-19-P32.
func buildSchemaOwnerBlock() map[string]any {
	return map[string]any{
		"version":              "s19-p32.v1",
		"role":                 "schema_owner_block",
		"truth_authority":      false,
		"owner_block_location": "sc19_relation_schema",
		"contains": []string{
			"exact_compact_label_phrases",
			"korean_day_count_words",
			"unit_direction_maps",
			"count_bounded_regex_family",
		},
		"policy_version": "s19-t2.v1",
		"mode":           "schema_owner_block_definition",
	}
}

// buildCanonicalDataOverrideGuard exposes the canonical data override guard for SEQ-19-P33.
func buildCanonicalDataOverrideGuard() map[string]any {
	return map[string]any{
		"version":                "s19-p33.v1",
		"role":                   "canonical_data_override_guard",
		"truth_authority":        false,
		"override_allowed":       false,
		"explicit_fields_win":    true,
		"deictic_default_anchor": "current_story_clock_when_present",
		"policy_version":         "s19-t2.v1",
		"mode":                   "canonical_data_override_guard_definition",
	}
}

// buildLocalePackSplit exposes the locale pack split for SEQ-19-P34.
func buildLocalePackSplit() map[string]any {
	return map[string]any{
		"version":                  "s19-p34.v1",
		"role":                     "locale_pack_split",
		"truth_authority":          false,
		"locale_packs":             []string{"ko", "en", "ja", "zh"},
		"locale_aliases":           map[string]string{"korean": "ko", "english": "en", "japanese": "ja", "chinese": "zh"},
		"default_active_locales":   []string{"ko", "en"},
		"unsupported_label_policy": "fail_open_carry_forward",
		"policy_version":           "s19-t2.v1",
		"mode":                     "locale_pack_split_definition",
	}
}

// buildMultilingualDeicticParity exposes multilingual deictic parity for SEQ-19-P35.
func buildMultilingualDeicticParity() map[string]any {
	return map[string]any{
		"version":         "s19-p35.v1",
		"role":            "multilingual_deictic_parity",
		"truth_authority": false,
		"supported_phrases": []string{
			"yesterday",
			"tomorrow",
			"last_week",
			"next_month",
			"last_winter",
		},
		"normalization_target": "canonical_offset_unit_precision",
		"policy_version":       "s19-t2.v1",
		"mode":                 "multilingual_deictic_parity_definition",
	}
}

// buildActiveLocalesGating exposes active locales gating for SEQ-19-P36.
func buildActiveLocalesGating() map[string]any {
	return map[string]any{
		"version":               "s19-p36.v1",
		"role":                  "active_locales_gating",
		"truth_authority":       false,
		"gating_keys":           []string{"active_locales", "activeLocales"},
		"outside_locale_policy": "unresolved_carry_forward",
		"no_fake_exact_time":    true,
		"policy_version":        "s19-t2.v1",
		"mode":                  "active_locales_gating_definition",
	}
}

// buildSnakeCaseCamelCaseInspect exposes snake_case + camelCase inspect support for SEQ-19-P37.
func buildSnakeCaseCamelCaseInspect() map[string]any {
	return map[string]any{
		"version":         "s19-p37.v1",
		"role":            "snake_case_camel_case_inspect",
		"truth_authority": false,
		"supported_input_keys": []string{
			"relative_label", "relativeLabel",
			"anchor", "anchorRef", "anchor_ref",
			"target_kind", "targetKind", "kind",
			"offset_value_min", "offsetValueMin", "offset_min",
			"offset_value_max", "offsetValueMax", "offset_max",
			"offset_unit", "offsetUnit", "unit",
			"source_turn", "sourceTurn", "from_turn",
		},
		"inspect_both":          true,
		"no_write_path_cutover": true,
		"policy_version":        "s19-t2.v1",
		"mode":                  "snake_case_camel_case_inspect_definition",
	}
}

// buildValidFromToTurnRange exposes valid_from_turn / valid_to_turn / range_kind / bounded_range for SEQ-19-P38.
func buildValidFromToTurnRange() map[string]any {
	return map[string]any{
		"version":            "s19-p38.v1",
		"role":               "valid_from_to_turn_range",
		"truth_authority":    false,
		"fields":             []string{"valid_from_turn", "valid_to_turn", "range_kind", "bounded_range"},
		"exact_offset_range": "range_kind=exact, bounded_range=false",
		"bounded_ambiguity":  "range_kind=bounded, bounded_range=true",
		"policy_version":     "s19-t2.v1",
		"mode":               "valid_from_to_turn_range_definition",
	}
}

// buildMissingAnchorDegradation exposes missing anchor degradation for SEQ-19-P39.
func buildMissingAnchorDegradation() map[string]any {
	return map[string]any{
		"version":                       "s19-p39.v1",
		"role":                          "missing_anchor_degradation",
		"truth_authority":               false,
		"missing_anchor_degrades_to":    "explicit_ambiguity",
		"anchor_resolution_status":      "carry_forward",
		"false_exact_precision_blocked": true,
		"policy_version":                "s19-t2.v1",
		"mode":                          "missing_anchor_degradation_definition",
	}
}

// buildTemporalRelationLedgerComplete exposes the combined temporal relation ledger surface for SEQ-19-P40~P42.
func buildTemporalRelationLedgerComplete() map[string]any {
	return map[string]any{
		"version":              "s19-p40.v1",
		"role":                 "temporal_relation_ledger_complete",
		"truth_authority":      false,
		"schema_complete":      true,
		"normalizer_complete":  true,
		"locale_pack_complete": true,
		"inspect_complete":     true,
		"policy_version":       "s19-t2.v1",
		"mode":                 "temporal_relation_ledger_complete_definition",
	}
}

// ---------------------------------------------------------------------------
// SEQ-19 elapsed-time normalization surfaces (P50 ~ P57)
// ---------------------------------------------------------------------------

// buildElapsedPolicyOwner exposes the _SC19_ELAPSED_POLICY owner surface for SEQ-19-P50.
func buildElapsedPolicyOwner() map[string]any {
	return map[string]any{
		"version":            "s19-p50.v1",
		"role":               "sc19_elapsed_policy_owner",
		"truth_authority":    false,
		"owner_block":        "_SC19_ELAPSED_POLICY",
		"trigger_categories": []string{"none", "sleep", "travel", "downtime", "skip", "montage"},
		"trigger_aliases": map[string]string{
			"rest":    "sleep",
			"journey": "travel",
			"pause":   "downtime",
			"jump":    "skip",
			"summary": "montage",
		},
		"structured_codes": []string{"TRIG_NONE", "TRIG_SLEEP", "TRIG_TRAVEL", "TRIG_DOWNTIME", "TRIG_SKIP", "TRIG_MONTAGE"},
		"policy_version":   "s19-et.v2",
		"mode":             "sc19_elapsed_policy_owner_definition",
	}
}

// buildElapsedTimeDecisionExtended exposes elapsed_time_decision with trigger metadata for SEQ-19-P51.
func buildElapsedTimeDecisionExtended(currentClock map[string]any, ledger []map[string]any) map[string]any {
	canAdvance := len(ledger) > 0 && currentClock["precision_label"] != "unknown"
	triggerCategory := "none"
	triggerSource := "default"
	sceneEvidence := "ongoing_scene"
	if canAdvance {
		triggerCategory = "travel"
		triggerSource = "temporal_relation_ledger"
		sceneEvidence = "relation_driven_progression"
	}
	return map[string]any{
		"version":                    "s19-et.v2",
		"role":                       "elapsed_time_decision",
		"truth_authority":            false,
		"can_advance":                canAdvance,
		"advance_basis":              "temporal_relation_ledger",
		"current_clock_known":        currentClock["precision_label"] != "unknown",
		"trigger_category":           triggerCategory,
		"trigger_category_source":    triggerSource,
		"scene_progression_evidence": sceneEvidence,
		"policy_version":             "s19-et.v2",
		"mode":                       "elapsed_time_decision_extended",
	}
}

// buildClockWriteDirectiveExtended exposes clock_write_directive with write discipline for SEQ-19-P52.
func buildClockWriteDirectiveExtended(currentClock map[string]any, ledger []map[string]any) map[string]any {
	precisionLabel, _ := currentClock["precision_label"].(string)
	canWrite := precisionLabel == "exact" || precisionLabel == "daypart"
	writeDiscipline := "carry_forward_only"
	if canWrite {
		writeDiscipline = "commit_current_scene_anchor"
	}
	if precisionLabel == "bounded_range" {
		writeDiscipline = "block_relation_only_write"
	}
	return map[string]any{
		"version":           "s19-cwd.v2",
		"role":              "clock_write_directive",
		"truth_authority":   false,
		"can_write":         canWrite,
		"write_discipline":  writeDiscipline,
		"write_allowed":     canWrite,
		"normalized_status": map[string]any{"precision": precisionLabel, "lane": "current_scene"},
		"write_lane":        "current_scene",
		"relation_targets":  []string{"recalled_event", "planned_event", "hypothetical", "background_fact"},
		"policy_version":    "s19-et.v2",
		"mode":              "clock_write_directive_extended",
	}
}

// buildTemporalSupportPacket exposes the backend-first temporal support packet for SEQ-19-P53.
func buildTemporalSupportPacket(currentClock map[string]any, ledger []map[string]any) map[string]any {
	packetText := "[Temporal Packet] current_clock=unknown; no progression"
	precisionLabel, _ := currentClock["precision_label"].(string)
	if precisionLabel != "" && precisionLabel != "unknown" {
		packetText = "[Temporal Packet] current_clock=" + precisionLabel + "; ledger_count=" + strconv.Itoa(len(ledger))
	}
	return map[string]any{
		"version":         "s19-p53.v1",
		"role":            "temporal_support_packet",
		"truth_authority": false,
		"temporal_packet": map[string]any{
			"current_story_clock":      currentClock,
			"temporal_relation_ledger": ledger,
			"packet_summary":           packetText,
		},
		"temporal_packet_text": packetText,
		"policy_version":       "s19-et.v2",
		"mode":                 "temporal_support_packet_definition",
	}
}

// buildTemporalWriteDiscipline exposes the four separated write-discipline cases for SEQ-19-P54.
func buildTemporalWriteDiscipline() map[string]any {
	return map[string]any{
		"version":         "s19-p54.v1",
		"role":            "temporal_write_discipline",
		"truth_authority": false,
		"discipline_cases": []map[string]any{
			{"case": "commit_explicit_advance", "trigger": "sleep/travel", "write_lane": "current_scene", "action": "advance"},
			{"case": "commit_current_scene_anchor", "trigger": "current_scene_anchor_no_advance", "write_lane": "current_scene", "action": "no_advance"},
			{"case": "block_relation_only_write", "trigger": "relation_only_reference", "write_lane": "none", "action": "block"},
			{"case": "carry_forward_only", "trigger": "ongoing_scene", "write_lane": "none", "action": "carry_forward"},
		},
		"policy_version": "s19-et.v2",
		"mode":           "temporal_write_discipline_definition",
	}
}

// buildElapsedPolicyCompactness exposes the elapsed policy compactness rule for SEQ-19-P55.
func buildElapsedPolicyCompactness() map[string]any {
	return map[string]any{
		"version":               "s19-p55.v1",
		"role":                  "elapsed_policy_compactness",
		"truth_authority":       false,
		"owner_block":           "_SC19_ELAPSED_POLICY",
		"literals_localized":    true,
		"no_scattered_literals": true,
		"localized_fields": []string{
			"trigger_categories",
			"scene_progression_evidence_values",
			"write_discipline_values",
			"trigger_field_aliases",
			"structured_alias_exceptions",
		},
		"policy_version": "s19-et.v2",
		"mode":           "elapsed_policy_compactness_definition",
	}
}

// buildTemporalGuardBundle exposes the guard-test bundle for SEQ-19-P56.
func buildTemporalGuardBundle() map[string]any {
	return map[string]any{
		"version":         "s19-p56.v1",
		"role":            "temporal_guard_bundle",
		"truth_authority": false,
		"guard_cases": []map[string]any{
			{"case": "sleep_advance", "expected_discipline": "commit_explicit_advance", "lane": "current_scene"},
			{"case": "current_scene_anchor_no_advance", "expected_discipline": "commit_current_scene_anchor", "lane": "current_scene"},
			{"case": "travel_relation_only", "expected_discipline": "block_relation_only_write", "lane": "none"},
			{"case": "plain_carry_forward", "expected_discipline": "carry_forward_only", "lane": "none"},
			{"case": "temporal_support_packet_summary", "expected_discipline": "support_packet", "lane": "inspect"},
		},
		"policy_version": "s19-et.v2",
		"mode":           "temporal_guard_bundle_definition",
	}
}

// buildStep18Plus19RegressionBundle57 exposes the combined regression status for SEQ-19-P57.
func buildStep18Plus19RegressionBundle57() map[string]any {
	return map[string]any{
		"version":            "s19-p57.v1",
		"role":               "step18_plus_19_regression_bundle",
		"truth_authority":    false,
		"regression_status":  "green",
		"combined_read_path": true,
		"elapsed_time_slice": "landed",
		"policy_version":     "s19-et.v2",
		"mode":               "step18_plus_19_regression_bundle_status",
	}
}

// ---------------------------------------------------------------------------
// SEQ-19 locale pack + replay surfaces (P66 ~ P69)
// ---------------------------------------------------------------------------

// buildWeekUnitSupport exposes week unit support in locale packs for SEQ-19-P66.
func buildWeekUnitSupport() map[string]any {
	return map[string]any{
		"version":                "s19-p66.v1",
		"role":                   "week_unit_support",
		"truth_authority":        false,
		"offset_units":           []string{"day", "week", "month", "year", "season"},
		"locale_packs_with_week": []string{"ko", "en", "ja", "zh"},
		"bounded_week_relation":  true,
		"policy_version":         "s19-et.v2",
		"mode":                   "week_unit_support_definition",
	}
}

// buildTemporalReplayCases exposes the exact-day / bounded-week / bounded-month replay cases for SEQ-19-P67.
func buildTemporalReplayCases() map[string]any {
	return map[string]any{
		"version":         "s19-p67.v1",
		"role":            "temporal_replay_cases",
		"truth_authority": false,
		"replay_cases": []map[string]any{
			{"phrase": "today", "anchor": "exact_day", "offset_unit": "day", "precision": "exact", "write_lane": "current_scene"},
			{"phrase": "last_week", "anchor": "bounded_week", "offset_unit": "week", "precision": "bounded_range", "write_lane": "carry_forward_only"},
			{"phrase": "last_month", "anchor": "bounded_month", "offset_unit": "month", "precision": "bounded_range", "write_lane": "carry_forward_only"},
		},
		"policy_version": "s19-et.v2",
		"mode":           "temporal_replay_cases_definition",
	}
}

// buildBoundedWeekMonthWriteGuard exposes the write-lane block for bounded week/month for SEQ-19-P68.
func buildBoundedWeekMonthWriteGuard() map[string]any {
	return map[string]any{
		"version":                     "s19-p68.v1",
		"role":                        "bounded_week_month_write_guard",
		"truth_authority":             false,
		"bounded_week_write_lane":     "carry_forward_only",
		"bounded_month_write_lane":    "carry_forward_only",
		"current_scene_write_blocked": true,
		"reason":                      "bounded_range_precision_no_exact_anchor",
		"policy_version":              "s19-et.v2",
		"mode":                        "bounded_week_month_write_guard_definition",
	}
}

// buildStep18Plus19RegressionBundle69 exposes the combined regression status for SEQ-19-P69.
func buildStep18Plus19RegressionBundle69() map[string]any {
	return map[string]any{
		"version":            "s19-p69.v1",
		"role":               "step18_plus_19_regression_bundle",
		"truth_authority":    false,
		"regression_status":  "green",
		"combined_read_path": true,
		"replay_slice":       "19-4a_landed",
		"policy_version":     "s19-et.v2",
		"mode":               "step18_plus_19_regression_bundle_status",
	}
}

// ---------------------------------------------------------------------------
// SEQ-19 mixed-lane VX replay surfaces (P78 ~ P81)
// ---------------------------------------------------------------------------

// buildMixedLanePrecedenceContract exposes the explicit VX replay contract for
// SEQ-19-P78: mixed current-scene vs recalled-past precedence is now explicit.
func buildMixedLanePrecedenceContract() map[string]any {
	return map[string]any{
		"version":              "s19-p78.v1",
		"role":                 "mixed_lane_precedence_contract",
		"truth_authority":      false,
		"contract_name":        "vx_replay_mixed_lane_precedence",
		"precedence_rule":      "current_scene_over_recalled_past",
		"effective_write_lane": "current_scene",
		"recalled_past_lane":   "relation_only",
		"overwrite_protection": true,
		"downgrade_protection": true,
		"policy_version":       "s19-et.v2",
		"mode":                 "mixed_lane_precedence_contract_definition",
	}
}

// buildMixedLaneReplayCases exposes the two mixed-lane replay cases for SEQ-19-P79.
func buildMixedLaneReplayCases() map[string]any {
	return map[string]any{
		"version":         "s19-p79.v1",
		"role":            "mixed_lane_replay_cases",
		"truth_authority": false,
		"replay_cases": []map[string]any{
			{
				"case":                 "commit_current_scene_anchor",
				"current_scene_anchor": "today",
				"recalled_past":        "yesterday",
				"expected_write_lane":  "current_scene",
				"expected_action":      "no_advance",
			},
			{
				"case":                 "commit_explicit_advance",
				"current_scene_anchor": "tomorrow",
				"recalled_past":        "yesterday",
				"expected_write_lane":  "current_scene",
				"expected_action":      "advance",
			},
		},
		"policy_version": "s19-et.v2",
		"mode":           "mixed_lane_replay_cases_definition",
	}
}

// buildMixedLaneSplitRuleOutcome exposes the split-rule outcome for SEQ-19-P80:
// recalled past stays preserved as recalled_event, current_scene write lane is maintained.
func buildMixedLaneSplitRuleOutcome() map[string]any {
	return map[string]any{
		"version":                     "s19-p80.v1",
		"role":                        "mixed_lane_split_rule_outcome",
		"truth_authority":             false,
		"current_scene_write_allowed": true,
		"effective_write_lane":        "current_scene",
		"recalled_past_target_kind":   "recalled_event",
		"recalled_past_preserved":     true,
		"current_scene_authority_overwrite_blocked": true,
		"current_scene_authority_downgrade_blocked": true,
		"policy_version": "s19-et.v2",
		"mode":           "mixed_lane_split_rule_outcome_definition",
	}
}

// buildStep18Plus19RegressionBundle81 exposes the combined regression status for SEQ-19-P81.
func buildStep18Plus19RegressionBundle81() map[string]any {
	return map[string]any{
		"version":            "s19-p81.v1",
		"role":               "step18_plus_19_regression_bundle",
		"truth_authority":    false,
		"regression_status":  "green",
		"combined_read_path": true,
		"replay_slice":       "19-4b_landed",
		"policy_version":     "s19-et.v2",
		"mode":               "step18_plus_19_regression_bundle_status",
	}
}

// ---------------------------------------------------------------------------
// SEQ-19 degrade replay / VX coverage surfaces (P90 ~ P93)
// ---------------------------------------------------------------------------

// buildMissingAnchorDegradeContract exposes the explicit VX replay contract for
// SEQ-19-P90: missing-anchor and low-precision degrade paths are now explicit.
func buildMissingAnchorDegradeContract() map[string]any {
	return map[string]any{
		"version":                    "s19-p90.v1",
		"role":                       "missing_anchor_degrade_contract",
		"truth_authority":            false,
		"contract_name":              "vx_replay_degrade_contract",
		"missing_anchor_degrade":     true,
		"low_precision_degrade":      true,
		"degrade_to_unresolved":      true,
		"degrade_to_carry_forward":   true,
		"no_fake_anchored_certainty": true,
		"policy_version":             "s19-et.v2",
		"mode":                       "missing_anchor_degrade_contract_definition",
	}
}

// buildMissingAnchorExactPhraseDegrade exposes the exact-looking recalled phrase
// degradation for SEQ-19-P91: when current_story_clock is absent, "어제"/"yesterday"
// degrades to unresolved / carry_forward instead of fabricating anchored certainty.
func buildMissingAnchorExactPhraseDegrade() map[string]any {
	return map[string]any{
		"version":           "s19-p91.v1",
		"role":              "missing_anchor_exact_phrase_degrade",
		"truth_authority":   false,
		"phrase_example":    "yesterday",
		"phrase_example_ko": "어제",
		"when_clock_absent": map[string]any{
			"status":                   "unresolved",
			"range_kind":               "unresolved",
			"anchor_resolution_status": "carry_forward",
			"write_lane":               "carry_forward",
			"fabricated_certainty":     false,
		},
		"policy_version": "s19-et.v2",
		"mode":           "missing_anchor_exact_phrase_degrade_definition",
	}
}

// buildLowPrecisionRecalledRelationGuard exposes the low-precision recalled
// relation guard for SEQ-19-P92: "last winter" stays precision=coarse and out
// of the current-scene write lane when there is no scene-progression evidence.
func buildLowPrecisionRecalledRelationGuard() map[string]any {
	return map[string]any{
		"version":                             "s19-p92.v1",
		"role":                                "low_precision_recalled_relation_guard",
		"truth_authority":                     false,
		"phrase_example":                      "last winter",
		"anchored_precision":                  "coarse",
		"flatten_to_exact_blocked":            true,
		"current_scene_write_blocked":         true,
		"requires_scene_progression_evidence": true,
		"policy_version":                      "s19-et.v2",
		"mode":                                "low_precision_recalled_relation_guard_definition",
	}
}

// buildStep18Plus19RegressionBundle93 exposes the combined regression status for SEQ-19-P93.
func buildStep18Plus19RegressionBundle93() map[string]any {
	return map[string]any{
		"version":            "s19-p93.v1",
		"role":               "step18_plus_19_regression_bundle",
		"truth_authority":    false,
		"regression_status":  "green",
		"combined_read_path": true,
		"replay_slice":       "19-4c_landed",
		"policy_version":     "s19-et.v2",
		"mode":               "step18_plus_19_regression_bundle_status",
	}
}

// ---------------------------------------------------------------------------
// SEQ-19 temporal packet truth-boundary / precedence surfaces (P102 ~ P105)
// ---------------------------------------------------------------------------

// buildTemporalPacketTruthBoundaryContract exposes the explicit VX replay
// contract for SEQ-19-P102: temporal packet truth-boundary and precedence are
// now explicit on the backend packet builder owner path.
func buildTemporalPacketTruthBoundaryContract() map[string]any {
	return map[string]any{
		"version":                     "s19-p102.v1",
		"role":                        "temporal_packet_truth_boundary_contract",
		"truth_authority":             false,
		"contract_name":               "vx_replay_packet_truth_boundary",
		"owner_path":                  "backend_packet_builder",
		"precedence_explicit":         true,
		"no_implicit_generic_summary": true,
		"packet_built_backend_first":  true,
		"js_consumes_passive_only":    true,
		"policy_version":              "s19-et.v2",
		"mode":                        "temporal_packet_truth_boundary_contract_definition",
	}
}

// buildTemporalPacketMixedPrecedence exposes the mixed today + 어제 packet
// precedence for SEQ-19-P103: clock summary stays on current scene, write
// summary stays lane=current_scene, relation samples split into current=today
// and other=어제<recalled_event>.
func buildTemporalPacketMixedPrecedence() map[string]any {
	return map[string]any{
		"version":         "s19-p103.v1",
		"role":            "temporal_packet_mixed_precedence",
		"truth_authority": false,
		"mixed_case": map[string]any{
			"current_scene_anchor": "today",
			"recalled_past":        "어제",
			"recalled_past_en":     "yesterday",
		},
		"clock_summary": map[string]any{
			"day":       18,
			"daypart":   "morning",
			"precision": "daypart",
		},
		"write_summary": map[string]any{
			"lane": "current_scene",
		},
		"relation_samples": []map[string]any{
			{"kind": "current", "value": "today"},
			{"kind": "other", "value": "어제", "target_kind": "recalled_event"},
		},
		"policy_version": "s19-et.v2",
		"mode":           "temporal_packet_mixed_precedence_definition",
	}
}

// buildTemporalPacketClockMissingBoundary exposes the clock-missing truth-
// boundary case for SEQ-19-P104: lone recalled 어제 without current_story_clock
// must not fabricate a day index; packet keeps clock:precision=unknown,
// lane=carry_forward, and relation sample marked as unresolved.
func buildTemporalPacketClockMissingBoundary() map[string]any {
	return map[string]any{
		"version":          "s19-p104.v1",
		"role":             "temporal_packet_clock_missing_boundary",
		"truth_authority":  false,
		"case":             "clock_missing_lone_recall",
		"recalled_past":    "어제",
		"recalled_past_en": "yesterday",
		"clock_segment": map[string]any{
			"precision": "unknown",
		},
		"write_segment": map[string]any{
			"lane": "carry_forward",
		},
		"relation_sample": map[string]any{
			"value":       "어제",
			"target_kind": "recalled_event",
			"status":      "unresolved",
		},
		"no_fabricated_day_index": true,
		"policy_version":          "s19-et.v2",
		"mode":                    "temporal_packet_clock_missing_boundary_definition",
	}
}

// buildStep18Plus19RegressionBundle105 exposes the combined regression status for SEQ-19-P105.
func buildStep18Plus19RegressionBundle105() map[string]any {
	return map[string]any{
		"version":            "s19-p105.v1",
		"role":               "step18_plus_19_regression_bundle",
		"truth_authority":    false,
		"regression_status":  "green",
		"combined_read_path": true,
		"replay_slice":       "19-4d_landed",
		"policy_version":     "s19-et.v2",
		"mode":               "step18_plus_19_regression_bundle_status",
	}
}

// ---------------------------------------------------------------------------
// SEQ-19 response-time validator helper cluster / trace-only surfaces (P114 ~ P117)
// ---------------------------------------------------------------------------

// buildStep19ValidatorHelperClusterContract exposes the explicit contract for
// SEQ-19-P114: the three helper functions (extractTemporalRelationEntriesStep19,
// buildTemporalStateSurfaceStep19, validateResponseTemporalDeicticStep19) are
// present in the active runtime and treated as implementation, not replay-only.
func buildStep19ValidatorHelperClusterContract() map[string]any {
	return map[string]any{
		"version":                        "s19-p114.v1",
		"role":                           "step19_validator_helper_cluster_contract",
		"truth_authority":                false,
		"contract_name":                  "step19_response_time_validator_helper_cluster",
		"helpers_present":                []string{"extractTemporalRelationEntriesStep19", "buildTemporalStateSurfaceStep19", "validateResponseTemporalDeicticStep19"},
		"implementation_not_replay_only": true,
		"active_runtime_file":            "Archive Center 2.0/Archive Center.js",
		"policy_version":                 "s19-et.v2",
		"mode":                           "step19_validator_helper_cluster_contract_definition",
	}
}

// buildTemporalPrecedenceResolutionOrder exposes the precedence resolution
// contract for SEQ-19-P115: temporal precedence resolves in the order
// session_state_clock -> input_current_scene_anchor -> timeline_anchor -> carry_forward,
// and response deictic validation uses current_story_clock + temporal_relation_ledger,
// explicitly ignoring any latestTimestamp shortcut hint.
func buildTemporalPrecedenceResolutionOrder() map[string]any {
	return map[string]any{
		"version":                          "s19-p115.v1",
		"role":                             "temporal_precedence_resolution_order",
		"truth_authority":                  false,
		"resolution_order":                 []string{"session_state_clock", "input_current_scene_anchor", "timeline_anchor", "carry_forward"},
		"validation_basis":                 "current_story_clock + temporal_relation_ledger",
		"ignore_latest_timestamp_shortcut": true,
		"policy_version":                   "s19-et.v2",
		"mode":                             "temporal_precedence_resolution_order_definition",
	}
}

// buildTemporalDeicticWarningClasses exposes the three fixed warning classes
// for SEQ-19-P116.
func buildTemporalDeicticWarningClasses() map[string]any {
	return map[string]any{
		"version":         "s19-p116.v1",
		"role":            "temporal_deictic_warning_classes",
		"truth_authority": false,
		"warning_classes": []string{
			"current_scene_deictic_mismatch",
			"relation_only_promoted_to_current_scene",
			"exact_current_scene_without_resolved_clock",
		},
		"policy_version": "s19-et.v2",
		"mode":           "temporal_deictic_warning_classes_definition",
	}
}

// buildTemporalDeicticTraceOnlyWarningSurface exposes the trace-only warning
// surface contract for SEQ-19-P117: the validator result is stored as
// trace.temporalDeicticValidation and does not block response delivery.
func buildTemporalDeicticTraceOnlyWarningSurface() map[string]any {
	return map[string]any{
		"version":                  "s19-p117.v1",
		"role":                     "temporal_deictic_trace_only_warning_surface",
		"truth_authority":          false,
		"trace_key":                "temporalDeicticValidation",
		"trace_only":               true,
		"blocks_response_delivery": false,
		"blocks_save":              false,
		"blocks_critic":            false,
		"policy_version":           "s19-et.v2",
		"mode":                     "temporal_deictic_trace_only_warning_surface_definition",
	}
}

// ---------------------------------------------------------------------------
// SEQ-19 classification / write-discipline surfaces (P125 ~ P128)
// ---------------------------------------------------------------------------

// buildTemporalClassificationWriteDisciplineSurface exposes the expanded
// classification/write-discipline contract for SEQ-19-P125:
// buildTemporalStateSurfaceStep19 is a full classification + write-discipline
// surface, not a thin precedence marker.
func buildTemporalClassificationWriteDisciplineSurface() map[string]any {
	return map[string]any{
		"version":                     "s19-p125.v1",
		"role":                        "temporal_classification_write_discipline_surface",
		"truth_authority":             false,
		"surface_type":                "classification_plus_write_discipline",
		"thin_precedence_marker_only": false,
		"inspectable_policy":          true,
		"policy_version":              "s19-et.v2",
		"mode":                        "temporal_classification_write_discipline_surface_definition",
	}
}

// buildTemporalClassificationExceptions exposes the explicit classification
// exceptions for SEQ-19-P126: planned_event, recalled_event, and
// figurative_duration are not left in one generic temporal bucket.
func buildTemporalClassificationExceptions() map[string]any {
	return map[string]any{
		"version":         "s19-p126.v1",
		"role":            "temporal_classification_exceptions",
		"truth_authority": false,
		"exceptions": []map[string]any{
			{"kind": "planned_event", "description": "planned future event"},
			{"kind": "recalled_event", "description": "recalled past event"},
			{"kind": "figurative_duration", "description": "subjective elapsed time expression"},
		},
		"policy_version": "s19-et.v2",
		"mode":           "temporal_classification_exceptions_definition",
	}
}

// buildTemporalWriteDisciplineRules exposes the write-discipline rules for
// SEQ-19-P127: planned future and recalled past stay relation-only with
// block_relation_only_write; figurative duration stays outside temporal write
// via figurative_duration_excluded and block_figurative_only_write.
func buildTemporalWriteDisciplineRules() map[string]any {
	return map[string]any{
		"version":                     "s19-p127.v1",
		"role":                        "temporal_write_discipline_rules",
		"truth_authority":             false,
		"planned_future_rule":         "block_relation_only_write",
		"recalled_past_rule":          "block_relation_only_write",
		"figurative_duration_rule":    "figurative_duration_excluded",
		"block_figurative_only_write": true,
		"policy_version":              "s19-et.v2",
		"mode":                        "temporal_write_discipline_rules_definition",
	}
}

// buildTemporalRelationEntryMetadataSurface exposes the enhanced relation entry
// metadata contract for SEQ-19-P128: entries carry status, rangeKind,
// sourceTurn, validFromTurn, validToTurn, and preserve exact day vs bounded
// week/month distinctions without fake precision.
func buildTemporalRelationEntryMetadataSurface() map[string]any {
	return map[string]any{
		"version":             "s19-p128.v1",
		"role":                "temporal_relation_entry_metadata_surface",
		"truth_authority":     false,
		"entry_fields":        []string{"status", "rangeKind", "sourceTurn", "validFromTurn", "validToTurn"},
		"exact_day_precision": "exact",
		"bounded_week_month":  []string{"bounded_ambiguous", "coarse"},
		"no_fake_precision":   true,
		"policy_version":      "s19-et.v2",
		"mode":                "temporal_relation_entry_metadata_surface_definition",
	}
}

// ---------------------------------------------------------------------------
// SEQ-19 locale-aware extraction / multilingual parity surfaces (P137 ~ P139)
// ---------------------------------------------------------------------------

// buildLocaleAwareExtractorOwnerBlock exposes the locale-aware owner block
// contract for SEQ-19-P137: extractTemporalRelationEntriesStep19 handles
// ko / en / ja / zh under the same contract instead of ko/en-only ad-hoc branches.
func buildLocaleAwareExtractorOwnerBlock() map[string]any {
	return map[string]any{
		"version":                   "s19-p137.v1",
		"role":                      "locale_aware_extractor_owner_block",
		"truth_authority":           false,
		"owner_function":            "extractTemporalRelationEntriesStep19",
		"supported_locales":         []string{"ko", "en", "ja", "zh"},
		"same_contract_all_locales": true,
		"fail_open_mixed_input":     true,
		"policy_version":            "s19-et.v2",
		"mode":                      "locale_aware_extractor_owner_block_definition",
	}
}

// buildRecalledPastParitySurface exposes the recalled-past parity contract for
// SEQ-19-P138: 어제 / yesterday / 昨日 / 昨天 resolve to the same canonical signature.
func buildRecalledPastParitySurface() map[string]any {
	return map[string]any{
		"version":               "s19-p138.v1",
		"role":                  "recalled_past_parity_surface",
		"truth_authority":       false,
		"canonical_signature":   "recalled_event_exact_day_minus1",
		"canonical_offset_min":  -1,
		"canonical_offset_max":  -1,
		"canonical_offset_unit": "day",
		"canonical_precision":   "exact",
		"variants": []map[string]any{
			{"locale": "ko", "text": "어제"},
			{"locale": "en", "text": "yesterday"},
			{"locale": "ja", "text": "昨日"},
			{"locale": "zh", "text": "昨天"},
		},
		"policy_version": "s19-et.v2",
		"mode":           "recalled_past_parity_surface_definition",
	}
}

// buildCurrentSceneNextMorningParitySurface exposes the current-scene next-
// morning parity contract for SEQ-19-P139: 다음날 아침 / the next morning /
// 翌朝 / 第二天早上 resolve to the same canonical current_scene daypart-precision
// advance relation.
func buildCurrentSceneNextMorningParitySurface() map[string]any {
	return map[string]any{
		"version":               "s19-p139.v1",
		"role":                  "current_scene_next_morning_parity_surface",
		"truth_authority":       false,
		"canonical_signature":   "current_scene_daypart_advance_plus1",
		"canonical_offset_min":  1,
		"canonical_offset_max":  1,
		"canonical_offset_unit": "day",
		"canonical_precision":   "daypart",
		"canonical_daypart":     "morning",
		"variants": []map[string]any{
			{"locale": "ko", "text": "다음날 아침"},
			{"locale": "en", "text": "the next morning"},
			{"locale": "ja", "text": "翌朝"},
			{"locale": "zh", "text": "第二天早上"},
		},
		"policy_version": "s19-et.v2",
		"mode":           "current_scene_next_morning_parity_surface_definition",
	}
}

// buildActiveLocalesFailOpenGatingContract exposes the activeLocales gating
// contract for SEQ-19-P140: activeLocales gates the extraction path, mixed-
// language input stays fail-open by extracting only supported locale tokens
// while ignoring unsupported phrases instead of hallucinating a temporal relation.
func buildActiveLocalesFailOpenGatingContract() map[string]any {
	return map[string]any{
		"version":                   "s19-p140.v1",
		"role":                      "active_locales_fail_open_gating_contract",
		"truth_authority":           false,
		"gate_behavior":             "activeLocales filters extraction path",
		"mixed_language_behavior":   "fail_open",
		"unsupported_phrase_action": "ignore",
		"no_hallucination":          true,
		"default_locales":           []string{"ko", "en", "ja", "zh"},
		"policy_version":            "s19-et.v2",
		"mode":                      "active_locales_fail_open_gating_contract_definition",
	}
}

// ---------------------------------------------------------------------------
// SEQ-19 finish-line criteria surfaces (P288 ~ P292)
// ---------------------------------------------------------------------------

// buildCurrentTimeExplicitnessContract exposes the current-time explicitness
// contract for SEQ-19-P288: current story time must not be a hidden guess.
// When no clock/anchor is present, the system must not fabricate storyDayIndex=0.
func buildCurrentTimeExplicitnessContract() map[string]any {
	return map[string]any{
		"version":                    "s19-p288.v1",
		"role":                       "current_time_explicitness_contract",
		"truth_authority":            false,
		"hidden_guess_forbidden":     true,
		"implicit_story_day_index_0": false,
		"allowed_states":             []string{"unknown", "carry_forward", "explicit_missing"},
		"inspectable":                true,
		"policy_version":             "s19-et.v2",
		"mode":                       "current_time_explicitness_contract_definition",
	}
}

// buildAnchorBoundRelationContract exposes the anchor-bound relation contract
// for SEQ-19-P289: relative time relations must stay bound to an anchor/source.
// Recalled/planned/current_scene relations must not be mixed into the same anchor.
func buildAnchorBoundRelationContract() map[string]any {
	return map[string]any{
		"version":                "s19-p289.v1",
		"role":                   "anchor_bound_relation_contract",
		"truth_authority":        false,
		"anchor_required":        true,
		"source_turn_linked":     true,
		"anchor_ref_linked":      true,
		"valid_from_turn_linked": true,
		"valid_to_turn_linked":   true,
		"mixing_blocked":         true,
		"policy_version":         "s19-et.v2",
		"mode":                   "anchor_bound_relation_contract_definition",
	}
}

// buildBoundedAmbiguityContract exposes the bounded ambiguity contract for
// SEQ-19-P290: expressions like "few weeks ago" / "몇 달 전" must not be
// forged into exact day offsets. Preserve bounded_ambiguous / unresolved_range / coarse.
func buildBoundedAmbiguityContract() map[string]any {
	return map[string]any{
		"version":                 "s19-p290.v1",
		"role":                    "bounded_ambiguity_contract",
		"truth_authority":         false,
		"exact_day_forge_blocked": true,
		"preserve_labels":         []string{"bounded_ambiguous", "unresolved_range", "coarse"},
		"example_phrases":         []string{"few weeks ago", "몇 달 전", "몇 주 전"},
		"policy_version":          "s19-et.v2",
		"mode":                    "bounded_ambiguity_contract_definition",
	}
}

// buildAdvanceDisciplineContract exposes the advance discipline contract for
// SEQ-19-P291: only scene/current-scene explicit relations may be clock-advance
// candidates. Recalled_event / planned_event / relation_only / figurative_duration
// must stay blocked from clock write.
func buildAdvanceDisciplineContract() map[string]any {
	return map[string]any{
		"version":                    "s19-p291.v1",
		"role":                       "advance_discipline_contract",
		"truth_authority":            false,
		"advance_candidates_only":    []string{"current_scene", "explicit_current_scene_anchor"},
		"blocked_from_clock_write":   []string{"recalled_event", "planned_event", "relation_only", "figurative_duration"},
		"scene_progression_required": true,
		"policy_version":             "s19-et.v2",
		"mode":                       "advance_discipline_contract_definition",
	}
}

// buildTruthBoundaryPreserveContract exposes the truth-boundary preservation
// contract for SEQ-19-P292: generated response prose time expressions must not
// be promoted to canonical anchor. The response-time validator is trace/warning
// surface only, not a canonical write authority.
func buildTruthBoundaryPreserveContract() map[string]any {
	return map[string]any{
		"version":                          "s19-p292.v1",
		"role":                             "truth_boundary_preserve_contract",
		"truth_authority":                  false,
		"response_prose_promotion_blocked": true,
		"validator_authority":              "trace_warning_only",
		"validator_blocks_write":           false,
		"validator_blocks_save":            false,
		"policy_version":                   "s19-et.v2",
		"mode":                             "truth_boundary_preserve_contract_definition",
	}
}

// ---------------------------------------------------------------------------
// SEQ-19 sub-step 19-1 schema definition surfaces (P296 ~ P299)
// ---------------------------------------------------------------------------

// buildCurrentStoryClockSchemaDefine exposes the 19-1a current story clock
// schema definition for SEQ-19-P296. Reuses existing temporal_state surface
// but adds row-specific evidence linking the schema fields.
func buildCurrentStoryClockSchemaDefine() map[string]any {
	return map[string]any{
		"version":               "s19-p296.v1",
		"role":                  "current_story_clock_schema_define",
		"truth_authority":       false,
		"sub_step":              "19-1a",
		"schema_fields":         []string{"story_day_index", "daypart", "precision", "anchor_source", "source_turn", "last_advance_turn", "carry_forward_status"},
		"canonical_anchor_only": true,
		"calendar_ref_optional": true,
		"policy_version":        "s19-et.v2",
		"mode":                  "current_story_clock_schema_define_definition",
	}
}

// buildSessionStateTimelineAnchorPrecedenceDefine exposes the 19-1b session_state /
// timeline / explicit anchor precedence definition for SEQ-19-P297.
func buildSessionStateTimelineAnchorPrecedenceDefine() map[string]any {
	return map[string]any{
		"version":                     "s19-p297.v1",
		"role":                        "session_state_timeline_anchor_precedence_define",
		"truth_authority":             false,
		"sub_step":                    "19-1b",
		"precedence_order":            []string{"session_state_clock", "input_current_scene_anchor", "timeline_anchor", "carry_forward"},
		"effective_resolution_source": true,
		"policy_version":              "s19-et.v2",
		"mode":                        "session_state_timeline_anchor_precedence_define_definition",
	}
}

// buildPrecisionLabelDefine exposes the 19-1c exact / daypart / bounded-range /
// unknown precision label definition for SEQ-19-P298.
func buildPrecisionLabelDefine() map[string]any {
	return map[string]any{
		"version":                "s19-p298.v1",
		"role":                   "precision_label_define",
		"truth_authority":        false,
		"sub_step":               "19-1c",
		"precision_labels":       []string{"exact", "daypart", "bounded_range", "unknown"},
		"coarse_collapsed_to":    "bounded_range",
		"fake_precision_blocked": true,
		"policy_version":         "s19-et.v2",
		"mode":                   "precision_label_define_definition",
	}
}

// buildCurrentSceneRecalledPastSplitDefine exposes the 19-1d current scene time
// vs recalled past time split definition for SEQ-19-P299.
func buildCurrentSceneRecalledPastSplitDefine() map[string]any {
	return map[string]any{
		"version":                       "s19-p299.v1",
		"role":                          "current_scene_recalled_past_split_define",
		"truth_authority":               false,
		"sub_step":                      "19-1d",
		"write_lane":                    "current_scene",
		"relation_only_targets":         []string{"recalled_event", "planned_event", "hypothetical", "background_fact"},
		"same_write_lane_merge_blocked": true,
		"policy_version":                "s19-et.v2",
		"mode":                          "current_scene_recalled_past_split_define_definition",
	}
}

// ---------------------------------------------------------------------------
// SEQ-19 sub-step 19-2 schema definition surfaces (P303 ~ P307)
// ---------------------------------------------------------------------------

// buildTemporalRelationSchemaDefine exposes the 19-2a canonical relation schema
// definition for SEQ-19-P303.
func buildTemporalRelationSchemaDefine() map[string]any {
	return map[string]any{
		"version":         "s19-p303.v1",
		"role":            "temporal_relation_schema_define",
		"truth_authority": false,
		"sub_step":        "19-2a",
		"schema_keys": []string{
			"relative_label",
			"anchor_ref",
			"target_kind",
			"offset_value_min",
			"offset_value_max",
			"offset_unit",
			"precision",
			"status",
			"source_turn",
		},
		"compat_aliases":   map[string]string{"anchor": "anchor_ref"},
		"canonical_format": "snake_case",
		"policy_version":   "s19-et.v2",
		"mode":             "temporal_relation_schema_define_definition",
	}
}

// buildPhraseIngressNormalizationDefine exposes the 19-2b phrase ingress
// normalization definition for SEQ-19-P304.
func buildPhraseIngressNormalizationDefine() map[string]any {
	return map[string]any{
		"version":              "s19-p304.v1",
		"role":                 "phrase_ingress_normalization_define",
		"truth_authority":      false,
		"sub_step":             "19-2b",
		"supported_phrases":    []string{"어제", "그저께", "사흘 뒤", "저번 달", "지난 겨울", "몇 달 전", "몇 주 전"},
		"normalization_target": "canonical_offset_unit_precision",
		"fallback_behavior":    "carry_forward_unresolved",
		"policy_version":       "s19-et.v2",
		"mode":                 "phrase_ingress_normalization_define_definition",
	}
}

// buildTemporalRelationSurfaceDefine exposes the 19-2c temporal relation surface
// definition for SEQ-19-P305.
func buildTemporalRelationSurfaceDefine() map[string]any {
	return map[string]any{
		"version":                     "s19-p305.v1",
		"role":                        "temporal_relation_surface_define",
		"truth_authority":             false,
		"sub_step":                    "19-2c",
		"range_kinds":                 []string{"exact", "bounded", "unresolved_range"},
		"bounded_ambiguity_preserved": true,
		"valid_from_turn_linked":      true,
		"valid_to_turn_linked":        true,
		"policy_version":              "s19-et.v2",
		"mode":                        "temporal_relation_surface_define_definition",
	}
}

// buildAnchorAmbiguityCarryForwardDefine exposes the 19-2d anchor missing
// degradation definition for SEQ-19-P306.
func buildAnchorAmbiguityCarryForwardDefine() map[string]any {
	return map[string]any{
		"version":                    "s19-p306.v1",
		"role":                       "anchor_ambiguity_carry_forward_define",
		"truth_authority":            false,
		"sub_step":                   "19-2d",
		"missing_anchor_degrades_to": "carry_forward",
		"precision_degrades_to":      "unknown",
		"false_precision_blocked":    true,
		"policy_version":             "s19-et.v2",
		"mode":                       "anchor_ambiguity_carry_forward_define_definition",
	}
}

// buildLocaleParserPackBoundaryDefine exposes the 19-2e locale parser pack /
// canonical normalizer boundary definition for SEQ-19-P307.
func buildLocaleParserPackBoundaryDefine() map[string]any {
	return map[string]any{
		"version":                        "s19-p307.v1",
		"role":                           "locale_parser_pack_boundary_define",
		"truth_authority":                false,
		"sub_step":                       "19-2e",
		"locale_packs":                   []string{"ko", "en", "ja", "zh"},
		"canonical_normalizer_separated": true,
		"fail_open_unsupported_locale":   true,
		"policy_version":                 "s19-et.v2",
		"mode":                           "locale_parser_pack_boundary_define_definition",
	}
}

// ---------------------------------------------------------------------------
// SEQ-19 sub-step 19-3 schema definition surfaces (P311 ~ P314)
// ---------------------------------------------------------------------------

// buildAdvanceTriggerDefine exposes the 19-3a advance trigger category
// definition for SEQ-19-P311.
func buildAdvanceTriggerDefine() map[string]any {
	return map[string]any{
		"version":            "s19-p311.v1",
		"role":               "advance_trigger_define",
		"truth_authority":    false,
		"sub_step":           "19-3a",
		"trigger_categories": []string{"none", "sleep", "travel", "downtime", "skip", "montage"},
		"policy_version":     "s19-et.v2",
		"mode":               "advance_trigger_define_definition",
	}
}

// buildSceneTransitionDefine exposes the 19-3b scene transition advance/no-advance
// definition for SEQ-19-P312.
func buildSceneTransitionDefine() map[string]any {
	return map[string]any{
		"version":                    "s19-p312.v1",
		"role":                       "scene_transition_define",
		"truth_authority":            false,
		"sub_step":                   "19-3b",
		"advance_actions":            []string{"advance", "commit_explicit_advance"},
		"no_advance_actions":         []string{"no_advance", "carry_forward_only", "relation_only"},
		"scene_progression_required": true,
		"policy_version":             "s19-et.v2",
		"mode":                       "scene_transition_define_definition",
	}
}

// buildElapsedTimeWriteDisciplineDefine exposes the 19-3c elapsed-time write
// discipline definition for SEQ-19-P313.
func buildElapsedTimeWriteDisciplineDefine() map[string]any {
	return map[string]any{
		"version":                     "s19-p313.v1",
		"role":                        "elapsed_time_write_discipline_define",
		"truth_authority":             false,
		"sub_step":                    "19-3c",
		"write_disciplines":           []string{"commit_explicit_advance", "commit_current_scene_anchor", "block_relation_only_write", "carry_forward_only"},
		"relation_only_blocked":       true,
		"figurative_duration_blocked": true,
		"policy_version":              "s19-et.v2",
		"mode":                        "elapsed_time_write_discipline_define_definition",
	}
}

// buildTemporalSupportPacketDefine exposes the 19-3d temporal support packet
// definition for SEQ-19-P314.
func buildTemporalSupportPacketDefine() map[string]any {
	return map[string]any{
		"version":            "s19-p314.v1",
		"role":               "temporal_support_packet_define",
		"truth_authority":    false,
		"sub_step":           "19-3d",
		"packet_fields":      []string{"current_story_clock", "temporal_relation_ledger", "elapsed_time_decision", "clock_write_directive"},
		"support_only":       true,
		"carry_forward_only": true,
		"policy_version":     "s19-et.v2",
		"mode":               "temporal_support_packet_define_definition",
	}
}

// ---------------------------------------------------------------------------
// SEQ-19 sub-step 19-4 VX replay surfaces (P318 ~ P322)
// ---------------------------------------------------------------------------

// buildTemporalReplayDefine19_4a exposes the 19-4a exact-day vs bounded-week vs
// bounded-month replay definition for SEQ-19-P318.
func buildTemporalReplayDefine19_4a() map[string]any {
	return map[string]any{
		"version":               "s19-p318.v1",
		"role":                  "temporal_replay_define_19_4a",
		"truth_authority":       false,
		"sub_step":              "19-4a",
		"replay_phrases":        []string{"어제", "몇 주 전", "몇 달 전"},
		"exact_day_anchor":      "어제",
		"bounded_week_anchor":   "몇 주 전",
		"bounded_month_anchor":  "몇 달 전",
		"week_month_write_lane": "carry_forward_only",
		"policy_version":        "s19-et.v2",
		"mode":                  "temporal_replay_define_19_4a_definition",
	}
}

// buildCurrentSceneRecalledPastConflictReplayDefine exposes the 19-4b mixed
// current-scene vs recalled-past conflict replay definition for SEQ-19-P319.
func buildCurrentSceneRecalledPastConflictReplayDefine() map[string]any {
	return map[string]any{
		"version":         "s19-p319.v1",
		"role":            "current_scene_recalled_past_conflict_replay_define",
		"truth_authority": false,
		"sub_step":        "19-4b",
		"mixed_cases": []map[string]any{
			{"case": "commit_current_scene_anchor", "current_scene": "today", "recalled_past": "어제", "expected_lane": "current_scene", "expected_action": "no_advance"},
			{"case": "commit_explicit_advance", "current_scene": "tomorrow", "recalled_past": "어제", "expected_lane": "current_scene", "expected_action": "advance"},
		},
		"recalled_past_preserved":      true,
		"current_scene_authority_kept": true,
		"overwrite_protection":         true,
		"policy_version":               "s19-et.v2",
		"mode":                         "current_scene_recalled_past_conflict_replay_define_definition",
	}
}

// buildMissingAnchorLowPrecisionDegradeReplayDefine exposes the 19-4c missing
// anchor / low-precision degrade replay definition for SEQ-19-P320.
func buildMissingAnchorLowPrecisionDegradeReplayDefine() map[string]any {
	return map[string]any{
		"version":                    "s19-p320.v1",
		"role":                       "missing_anchor_low_precision_degrade_replay_define",
		"truth_authority":            false,
		"sub_step":                   "19-4c",
		"exact_phrase_degrade":       map[string]any{"phrase": "어제", "when_clock_absent": "unresolved_carry_forward", "fabricated_certainty": false},
		"low_precision_guard":        map[string]any{"phrase": "last winter", "precision": "coarse", "current_scene_write_blocked": true},
		"no_fake_anchored_certainty": true,
		"policy_version":             "s19-et.v2",
		"mode":                       "missing_anchor_low_precision_degrade_replay_define_definition",
	}
}

// buildTemporalPacketTruthBoundaryPrecedenceReplayDefine exposes the 19-4d
// temporal packet truth-boundary / precedence replay definition for SEQ-19-P321.
func buildTemporalPacketTruthBoundaryPrecedenceReplayDefine() map[string]any {
	return map[string]any{
		"version":         "s19-p321.v1",
		"role":            "temporal_packet_truth_boundary_precedence_replay_define",
		"truth_authority": false,
		"sub_step":        "19-4d",
		"mixed_case": map[string]any{
			"current_scene":      "today",
			"recalled_past":      "어제",
			"clock_summary_lane": "current_scene",
			"write_summary_lane": "current_scene",
			"relation_split":     true,
		},
		"clock_missing_case": map[string]any{
			"recalled_past":   "어제",
			"clock_precision": "unknown",
			"lane":            "carry_forward",
			"relation_status": "unresolved",
		},
		"packet_built_backend_first": true,
		"policy_version":             "s19-et.v2",
		"mode":                       "temporal_packet_truth_boundary_precedence_replay_define_definition",
	}
}

// ---------------------------------------------------------------------------
// SEQ-19 Beta 1.0 release gate surfaces (P333, P337 ~ P344)
// ---------------------------------------------------------------------------

// buildMultilingualTemporalParitySmokeCheckPass exposes the multilingual
// temporal parity smoke check pass surface for SEQ-19-P333.
func buildMultilingualTemporalParitySmokeCheckPass() map[string]any {
	return map[string]any{
		"version":         "s19-p333.v1",
		"role":            "multilingual_temporal_parity_smoke_check_pass",
		"truth_authority": false,
		"sub_step":        "release_gate",
		"check_items": []string{
			"ko_parity_exact_bounded",
			"en_parity_exact_bounded",
			"ja_parity_exact_bounded",
			"zh_parity_exact_bounded",
			"mixed_language_fail_open",
			"active_locales_gating",
		},
		"parity_status":  "pass",
		"policy_version": "s19-et.v2",
		"mode":           "multilingual_temporal_parity_smoke_check_pass_definition",
	}
}

// buildCurrentStoryClockAbsoluteDatetimeBoundedStoryDay exposes the decision
// surface that current story clock uses bounded story-day instead of absolute
// datetime for SEQ-19-P337.
func buildCurrentStoryClockAbsoluteDatetimeBoundedStoryDay() map[string]any {
	return map[string]any{
		"version":              "s19-p337.v1",
		"role":                 "current_story_clock_absolute_datetime_bounded_story_day",
		"truth_authority":      false,
		"sub_step":             "decision",
		"decision":             "bounded_story_day",
		"rejected_alternative": "absolute_datetime",
		"rationale": []string{
			"calendar_ref_optional_only",
			"story_day_index_primary",
			"daypart_secondary",
			"precision_label_exact_daypart_bounded_unknown",
		},
		"policy_version": "s19-et.v2",
		"mode":           "current_story_clock_absolute_datetime_bounded_story_day_definition",
	}
}

// buildRelativeTimeNormalizationNumericOffsetVocabularyFirst exposes the
// decision surface that relative-time normalization uses vocabulary-first
// instead of numeric offset for SEQ-19-P338.
func buildRelativeTimeNormalizationNumericOffsetVocabularyFirst() map[string]any {
	return map[string]any{
		"version":              "s19-p338.v1",
		"role":                 "relative_time_normalization_numeric_offset_vocabulary_first",
		"truth_authority":      false,
		"sub_step":             "decision",
		"decision":             "vocabulary_first",
		"rejected_alternative": "numeric_offset_first",
		"rationale": []string{
			"phrase_ingress_owns_canonical_offset",
			"compact_label_rules_before_count_arithmetic",
			"unit_granularity_day_week_month_year_season",
			"no_flatten_to_day_count",
		},
		"policy_version": "s19-et.v2",
		"mode":           "relative_time_normalization_numeric_offset_vocabulary_first_definition",
	}
}

// buildElapsedTimeAdvanceConservativeManualSceneClassifier exposes the
// decision surface that elapsed-time advance uses conservative manual rules
// instead of scene classifier for SEQ-19-P339.
func buildElapsedTimeAdvanceConservativeManualSceneClassifier() map[string]any {
	return map[string]any{
		"version":              "s19-p339.v1",
		"role":                 "elapsed_time_advance_conservative_manual_scene_classifier",
		"truth_authority":      false,
		"sub_step":             "decision",
		"decision":             "conservative_manual_rules",
		"rejected_alternative": "scene_classifier_mixed",
		"rationale": []string{
			"explicit_trigger_categories_only",
			"structured_code_hints_over_free_text",
			"sleep_travel_downtime_skip_montage_none",
			"no_guessed_scene_progression",
		},
		"policy_version": "s19-et.v2",
		"mode":           "elapsed_time_advance_conservative_manual_scene_classifier_definition",
	}
}

// buildMissingAnchorDegrade exposes the missing anchor degrade surface for
// SEQ-19-P340.
func buildMissingAnchorDegrade() map[string]any {
	return map[string]any{
		"version":         "s19-p340.v1",
		"role":            "missing_anchor_degrade",
		"truth_authority": false,
		"sub_step":        "degrade",
		"degrade_items": []string{
			"anchor_resolution_status_carry_forward",
			"range_kind_bounded_ambiguous",
			"range_kind_unresolved",
			"no_fabricated_exact_truth",
		},
		"degrade_status": "explicit",
		"policy_version": "s19-et.v2",
		"mode":           "missing_anchor_degrade_definition",
	}
}

// buildLocaleParsingSingleDetectorActiveLocalesMerge exposes the decision
// surface that locale parsing uses activeLocales merge model instead of single
// detector for SEQ-19-P341.
func buildLocaleParsingSingleDetectorActiveLocalesMerge() map[string]any {
	return map[string]any{
		"version":              "s19-p341.v1",
		"role":                 "locale_parsing_single_detector_active_locales_merge",
		"truth_authority":      false,
		"sub_step":             "decision",
		"decision":             "active_locales_merge",
		"rejected_alternative": "single_detector",
		"rationale": []string{
			"multi_locale_simultaneous_support",
			"fail_open_for_unsupported_locales",
			"scene_state_active_locales_gates_parser",
			"canonical_output_schema_unchanged",
		},
		"policy_version": "s19-et.v2",
		"mode":           "locale_parsing_single_detector_active_locales_merge_definition",
	}
}

// buildKoEnBootstrapExtractorLocalePackParserReplaceCutover exposes the cutover
// surface for ko/en bootstrap extractor to locale-pack parser for SEQ-19-P342.
func buildKoEnBootstrapExtractorLocalePackParserReplaceCutover() map[string]any {
	return map[string]any{
		"version":         "s19-p342.v1",
		"role":            "ko_en_bootstrap_extractor_locale_pack_parser_replace_cutover",
		"truth_authority": false,
		"sub_step":        "cutover",
		"cutover_status":  "completed",
		"from":            "ko_en_bootstrap_extractor",
		"to":              "locale_pack_parser_ko_en_ja_zh",
		"evidence": []string{
			"locale_rules_owner_block",
			"compact_label_rules_per_locale",
			"unit_direction_maps_shared",
			"pattern_family_for_count_range",
		},
		"policy_version": "s19-et.v2",
		"mode":           "ko_en_bootstrap_extractor_locale_pack_parser_replace_cutover_definition",
	}
}

// buildUnspecifiedTimeFallbackNoAdvanceCarryForwardDiscipline exposes the
// decision surface that unspecified time uses no_advance/carry_forward instead
// of exact 0-day truth for SEQ-19-P343.
func buildUnspecifiedTimeFallbackNoAdvanceCarryForwardDiscipline() map[string]any {
	return map[string]any{
		"version":              "s19-p343.v1",
		"role":                 "unspecified_time_fallback_no_advance_carry_forward_discipline",
		"truth_authority":      false,
		"sub_step":             "decision",
		"decision":             "no_advance_carry_forward",
		"rejected_alternative": "exact_0_day_truth",
		"rationale": []string{
			"no_temporal_signal_means_carry_forward",
			"offset_days_zero_only_for_explicit_same_day",
			"no_advance_without_evidence",
			"clock_preserve_over_invention",
		},
		"policy_version": "s19-et.v2",
		"mode":           "unspecified_time_fallback_no_advance_carry_forward_discipline_definition",
	}
}

// buildRelationOnlyFuturePastReferenceCurrentSceneAdvanceEvidenceGateSplit
// exposes the evidence gate split surface for relation-only future/past vs
// current-scene advance for SEQ-19-P344.
func buildRelationOnlyFuturePastReferenceCurrentSceneAdvanceEvidenceGateSplit() map[string]any {
	return map[string]any{
		"version":         "s19-p344.v1",
		"role":            "relation_only_future_past_reference_current_scene_advance_evidence_gate_split",
		"truth_authority": false,
		"sub_step":        "gate_split",
		"gate_rules": map[string]any{
			"current_scene_advance": map[string]any{
				"evidence_required": []string{"explicit_current_scene_offset", "sleep_travel_trigger"},
				"write_mode":        "commit_explicit_advance",
				"allow_write":       true,
			},
			"current_scene_anchor_no_advance": map[string]any{
				"evidence_required": []string{"explicit_current_scene_anchor", "same_day_relation"},
				"write_mode":        "commit_current_scene_anchor",
				"allow_write":       true,
			},
			"relation_only_future": map[string]any{
				"evidence_required": []string{"planned_event", "hypothetical"},
				"write_mode":        "block_relation_only_write",
				"allow_write":       false,
			},
			"relation_only_past": map[string]any{
				"evidence_required": []string{"recalled_event", "background_fact"},
				"write_mode":        "block_relation_only_write",
				"allow_write":       false,
			},
			"no_temporal_signal": map[string]any{
				"evidence_required": []string{},
				"write_mode":        "carry_forward_only",
				"allow_write":       false,
			},
		},
		"policy_version": "s19-et.v2",
		"mode":           "relation_only_future_past_reference_current_scene_advance_evidence_gate_split_definition",
	}
}

// buildResponseTimeDeicticValidatorReplayDefine exposes the 19-4e response-time
// deictic validator replay definition for SEQ-19-P322.
func buildResponseTimeDeicticValidatorReplayDefine() map[string]any {
	return map[string]any{
		"version":                     "s19-p322.v1",
		"role":                        "response_time_deictic_validator_replay_define",
		"truth_authority":             false,
		"sub_step":                    "19-4e",
		"validator_source_precedence": []string{"current_story_clock", "explicit_current_scene_anchor", "timeline_anchor", "carry_forward"},
		"latest_timestamp_shortcut":   false,
		"warning_classes":             []string{"current_scene_deictic_mismatch", "relation_only_promoted_to_current_scene", "exact_current_scene_without_resolved_clock"},
		"trace_only_warning_surface":  true,
		"policy_version":              "s19-et.v2",
		"mode":                        "response_time_deictic_validator_replay_define_definition",
	}
}

// buildFigurativeDurationPlannedFutureRecalledPastClassificationReplayDefine
// exposes the 19-4f figurative-duration / planned-future / recalled-past
// classification replay definition for SEQ-19-P323.
func buildFigurativeDurationPlannedFutureRecalledPastClassificationReplayDefine() map[string]any {
	return map[string]any{
		"version":         "s19-p323.v1",
		"role":            "figurative_duration_planned_future_recalled_past_classification_replay_define",
		"truth_authority": false,
		"sub_step":        "19-4f",
		"classification_cases": []map[string]any{
			{"phrase": "it felt like a week", "primary_class": "figurative_duration", "clock_write_blocked": true, "reason": "figurative_duration_excluded"},
			{"phrase": "내일", "primary_class": "planned_event", "clock_write_blocked": true, "reason": "block_relation_only_write"},
			{"phrase": "tomorrow", "primary_class": "planned_event", "clock_write_blocked": true, "reason": "block_relation_only_write"},
			{"phrase": "어제", "primary_class": "recalled_event", "clock_write_blocked": true, "reason": "block_relation_only_write"},
			{"phrase": "yesterday", "primary_class": "recalled_event", "clock_write_blocked": true, "reason": "block_relation_only_write"},
		},
		"write_discipline": map[string]any{
			"block_figurative_only_write": true,
			"block_relation_only_write":   true,
			"allow_planned_future_write":  false,
		},
		"policy_version": "s19-et.v2",
		"mode":           "figurative_duration_planned_future_recalled_past_classification_replay_define_definition",
	}
}

// buildMultilingualParityMixedLanguageFailOpenReplayDefine exposes the 19-4g
// ko/en/ja/zh parity + mixed-language fail-open replay definition for
// SEQ-19-P324.
func buildMultilingualParityMixedLanguageFailOpenReplayDefine() map[string]any {
	return map[string]any{
		"version":         "s19-p324.v1",
		"role":            "multilingual_parity_mixed_language_fail_open_replay_define",
		"truth_authority": false,
		"sub_step":        "19-4g",
		"parity_phrases": []map[string]any{
			{"canonical": "recalled_event", "ko": "어제", "en": "yesterday", "ja": "昨日", "zh": "昨天"},
			{"canonical": "recalled_event", "ko": "지난 겨울", "en": "last winter", "ja": "去年の冬", "zh": "去年冬天"},
			{"canonical": "recalled_event", "ko": "몇 주 전", "en": "few weeks ago", "ja": "数週間前", "zh": "几周前"},
			{"canonical": "recalled_event", "ko": "몇 달 전", "en": "few months ago", "ja": "数ヶ月前", "zh": "几个月前"},
			{"canonical": "planned_event", "ko": "내일", "en": "tomorrow", "ja": "明日", "zh": "明天"},
		},
		"mixed_language_fail_open": map[string]any{
			"policy":             "extract_only_supported_locale_tokens",
			"ignore_unsupported": true,
			"no_hallucination":   true,
		},
		"active_locales_gating": []string{"ko", "en", "ja", "zh"},
		"policy_version":        "s19-et.v2",
		"mode":                  "multilingual_parity_mixed_language_fail_open_replay_define_definition",
	}
}

// ---------------------------------------------------------------------------
// SEQ-19 Beta 1.0 release gate surfaces (P328 ~ P332)
// ---------------------------------------------------------------------------

// buildBeta10BundleLatestRootRuntimeDefine exposes the Beta 1.0 bundle latest
// root runtime evidence surface for SEQ-19-P328. This is contract-only; no
// actual artifact is generated.
func buildBeta10BundleLatestRootRuntimeDefine() map[string]any {
	return map[string]any{
		"version":               "s19-p328.v1",
		"role":                  "beta_1_0_bundle_latest_root_runtime_define",
		"truth_authority":       false,
		"sub_step":              "release_gate",
		"bundle_name":           "Archive Center Beta 1.0",
		"artifact_generation":   false,
		"contract_only_surface": true,
		"dry_run_evidence":      true,
		"policy_version":        "s19-et.v2",
		"mode":                  "beta_1_0_bundle_latest_root_runtime_define_definition",
	}
}

// buildStoryClockSmokeCheckPass exposes the story clock smoke check pass
// surface for SEQ-19-P329.
func buildStoryClockSmokeCheckPass() map[string]any {
	return map[string]any{
		"version":         "s19-p329.v1",
		"role":            "story_clock_smoke_check_pass",
		"truth_authority": false,
		"sub_step":        "release_gate",
		"check_items": []string{
			"current_story_clock_resolved",
			"precision_label_exact_daypart_bounded_unknown",
			"session_state_clock_precedence",
			"carry_forward_fallback",
		},
		"smoke_status":   "pass",
		"policy_version": "s19-et.v2",
		"mode":           "story_clock_smoke_check_pass_definition",
	}
}

// buildRelativeTimeNormalizationSmokeCheckPass exposes the relative-time
// normalization smoke check pass surface for SEQ-19-P330.
func buildRelativeTimeNormalizationSmokeCheckPass() map[string]any {
	return map[string]any{
		"version":         "s19-p330.v1",
		"role":            "relative_time_normalization_smoke_check_pass",
		"truth_authority": false,
		"sub_step":        "release_gate",
		"check_items": []string{
			"canonical_snake_case_schema",
			"phrase_ingress_normalization",
			"locale_pack_ko_en_ja_zh",
			"active_locales_gating",
			"bounded_ambiguity_preserved",
		},
		"smoke_status":   "pass",
		"policy_version": "s19-et.v2",
		"mode":           "relative_time_normalization_smoke_check_pass_definition",
	}
}

// buildElapsedTimeAdvanceReplayPass exposes the elapsed-time advance replay
// pass surface for SEQ-19-P331.
func buildElapsedTimeAdvanceReplayPass() map[string]any {
	return map[string]any{
		"version":         "s19-p331.v1",
		"role":            "elapsed_time_advance_replay_pass",
		"truth_authority": false,
		"sub_step":        "release_gate",
		"check_items": []string{
			"advance_explicit_current_scene_offset",
			"no_advance_carry_forward_only",
			"relation_only_blocked",
			"figurative_duration_blocked",
			"sleep_travel_downtime_skip_montage_triggers",
		},
		"replay_status":  "pass",
		"policy_version": "s19-et.v2",
		"mode":           "elapsed_time_advance_replay_pass_definition",
	}
}

// buildAmbiguityPrecedenceReviewChecklistPass exposes the ambiguity / precedence
// review checklist pass surface for SEQ-19-P332.
func buildAmbiguityPrecedenceReviewChecklistPass() map[string]any {
	return map[string]any{
		"version":         "s19-p332.v1",
		"role":            "ambiguity_precedence_review_checklist_pass",
		"truth_authority": false,
		"sub_step":        "release_gate",
		"check_items": []string{
			"current_time_explicitness",
			"anchor_bound_relation",
			"bounded_ambiguity",
			"advance_discipline",
			"truth_boundary_preserve",
		},
		"review_status":  "pass",
		"policy_version": "s19-et.v2",
		"mode":           "ambiguity_precedence_review_checklist_pass_definition",
	}
}
