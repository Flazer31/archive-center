package httpapi

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
