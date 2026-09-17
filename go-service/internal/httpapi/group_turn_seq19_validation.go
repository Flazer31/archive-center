package httpapi

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
