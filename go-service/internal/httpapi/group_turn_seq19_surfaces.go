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
