package httpapi

// ---------------------------------------------------------------------------
// SEQ-20 surfaces — Beta 1.1 Post-Chroma Temporal + Entity/Graph Retrieval
// ---------------------------------------------------------------------------
// This file contains builders for SEQ-20 temporal retrieval/read policy
// surfaces (q20a~q20e) and preparatory reset/admin rows (P9~P11).
//
// Step 20 is retrieval behavior on top of Step 19 substrate:
//   - q20a: temporal query expansion
//   - q20b: temporal validity read policy
//   - q20c: event retrieval / invalidation support
//   - q20d: promotion-lag pending-current support
//   - q20e: recent multi-turn hot recall buffer
//
// All surfaces are contract-only / support-only and do not overwrite Step 19
// canonical truth. MariaDB remains canonical truth authority; ChromaDB is shadow
// accelerator only.
// ---------------------------------------------------------------------------

// ===========================================================================
// SEQ-20 Preparatory reset/admin surfaces (P9 ~ P11)
// ===========================================================================

// buildSeq20ResetAdminNote exposes the reset administration note surface for
// SEQ-20-P9. This is preparatory/document-only; no production code is changed.
func buildSeq20ResetAdminNote() map[string]any {
	return map[string]any{
		"version":         "s20-p9.v1",
		"role":            "seq20_reset_admin_note",
		"truth_authority": false,
		"sub_step":        "preparatory",
		"note":            "Existing checked checklist items in this file were cleared for redo.",
		"action_taken":    "reset_cleared",
		"policy_version":  "s20-tv.v1",
		"mode":            "seq20_reset_admin_note_definition",
	}
}

// buildSeq20HistoricalContentPreserved exposes the historical content
// preservation note surface for SEQ-20-P10. Preparatory/document-only.
func buildSeq20HistoricalContentPreserved() map[string]any {
	return map[string]any{
		"version":         "s20-p10.v1",
		"role":            "seq20_historical_content_preserved",
		"truth_authority": false,
		"sub_step":        "preparatory",
		"note":            "Historical content in this file was preserved; no step text was deleted.",
		"action_taken":    "preserved",
		"policy_version":  "s20-tv.v1",
		"mode":            "seq20_historical_content_preserved_definition",
	}
}

// buildSeq20ResetNoteOnly exposes the reset work-only note surface for
// SEQ-20-P11. Preparatory/document-only.
func buildSeq20ResetNoteOnly() map[string]any {
	return map[string]any{
		"version":         "s20-p11.v1",
		"role":            "seq20_reset_note_only",
		"truth_authority": false,
		"sub_step":        "preparatory",
		"note":            "This note records document reset work only, not revalidation of the step itself.",
		"action_taken":    "document_only",
		"policy_version":  "s20-tv.v1",
		"mode":            "seq20_reset_note_only_definition",
	}
}

// ===========================================================================
// SEQ-20 q20a temporal query expansion surfaces (P21 ~ P28)
// ===========================================================================

// buildQ20aTemporalQueryExpansionPreparatory exposes the 20-1a preparatory
// query-builder owner work surface for SEQ-20-P21.
func buildQ20aTemporalQueryExpansionPreparatory() map[string]any {
	return map[string]any{
		"version":         "q20a-p21.v1",
		"role":            "q20a_temporal_query_expansion_preparatory",
		"truth_authority": false,
		"sub_step":        "q20a",
		"contract_owner":  "query_class_contract_qr1a",
		"status":          "preparatory_closed",
		"policy_version":  "s20-tv.v1",
		"mode":            "q20a_temporal_query_expansion_preparatory_definition",
	}
}

// buildQ20aV1TemporalQueryExpansion exposes the q20a.v1 temporal query expansion
// contract surface for SEQ-20-P22.
func buildQ20aV1TemporalQueryExpansion() map[string]any {
	return map[string]any{
		"version":         "q20a.v1",
		"role":            "temporal_query_expansion",
		"truth_authority": false,
		"sub_step":        "q20a",
		"contract_owner":  "query_class_contract_qr1a",
		"replaces":        "boolean_only_temporal_class_signal",
		"policy_version":  "s20-tv.v1",
		"mode":            "q20a_v1_temporal_query_expansion_definition",
	}
}

// buildQ20aRuleSurfaceFocusRange exposes the q20a rule surface with
// focus_mode / range_kind / granularity / cue_terms / prefer flags for
// SEQ-20-P23.
func buildQ20aRuleSurfaceFocusRange() map[string]any {
	return map[string]any{
		"version":         "q20a-p23.v1",
		"role":            "q20a_rule_surface_focus_range",
		"truth_authority": false,
		"sub_step":        "q20a",
		"focus_modes": []string{
			"current_clock",
			"past_event",
			"past_window",
			"timeline_order",
		},
		"range_kinds": []string{
			"exact_offset",
			"bounded_ambiguous",
			"unresolved_range",
		},
		"granularities": []string{
			"day",
			"week",
			"month",
			"year",
			"season",
		},
		"prefer_flags": []string{
			"prefer_current_truth",
			"prefer_event_retrieval",
			"prefer_validity_window",
		},
		"policy_version": "s20-tv.v1",
		"mode":           "q20a_rule_surface_focus_range_definition",
	}
}

// buildQ20aDerivesFromSc19RelationSchema exposes the surface that q20a derives
// locale cues from Step 19 _SC19_RELATION_SCHEMA instead of a duplicated token
// table for SEQ-20-P24.
func buildQ20aDerivesFromSc19RelationSchema() map[string]any {
	return map[string]any{
		"version":              "q20a-p24.v1",
		"role":                 "q20a_derives_from_sc19_relation_schema",
		"truth_authority":      false,
		"sub_step":             "q20a",
		"source_schema":        "_SC19_RELATION_SCHEMA",
		"source_owner":         "Step 19 compact relative label rules",
		"duplicated_table":     false,
		"week_labels_extended": true,
		"policy_version":       "s20-tv.v1",
		"mode":                 "q20a_derives_from_sc19_relation_schema_definition",
	}
}

// buildQ20aMirroredAtRecallIntent exposes the surface that q20a is mirrored at
// the top-level recall intent contract for SEQ-20-P25.
func buildQ20aMirroredAtRecallIntent() map[string]any {
	return map[string]any{
		"version":         "q20a-p25.v1",
		"role":            "q20a_mirrored_at_recall_intent",
		"truth_authority": false,
		"sub_step":        "q20a",
		"mirror_target":   "_build_recall_intent_contract_q3a",
		"mirror_payload":  "temporal_query_expansion",
		"purpose":         "reuse_one_owner_surface_for_later_slices",
		"policy_version":  "s20-tv.v1",
		"mode":            "q20a_mirrored_at_recall_intent_definition",
	}
}

// buildQ20aCurrentClockOverlayCuePack exposes the current_clock query-intent
// overlay cue pack surface for SEQ-20-P26.
func buildQ20aCurrentClockOverlayCuePack() map[string]any {
	return map[string]any{
		"version":         "q20a-p26.v1",
		"role":            "q20a_current_clock_overlay_cue_pack",
		"truth_authority": false,
		"sub_step":        "q20a",
		"cue_pack_scope":  "question_shape_metadata",
		"not_owned_by":    "Step 19 relative_label_canon",
		"example_cues": []string{
			"지금 며칠째야",
			"what day is it",
			"今何日目",
			"现在是第几天",
		},
		"policy_version": "s20-tv.v1",
		"mode":           "q20a_current_clock_overlay_cue_pack_definition",
	}
}

// buildQ20aQr1aLexicalRoutingNormalized exposes the qr1a lexical routing
// metadata normalization surface for SEQ-20-P27.
func buildQ20aQr1aLexicalRoutingNormalized() map[string]any {
	return map[string]any{
		"version":         "q20a-p27.v1",
		"role":            "q20a_qr1a_lexical_routing_normalized",
		"truth_authority": false,
		"sub_step":        "q20a",
		"owner_block":     "_QR1A_QUERY_CLASS_SIGNAL_RULES",
		"normalized_from": "separate_tuple_constants",
		"cue_categories": []string{
			"callback",
			"canon",
			"temporal",
		},
		"policy_version": "s20-tv.v1",
		"mode":           "q20a_qr1a_lexical_routing_normalized_definition",
	}
}

// buildQ20aContractOnlyGroundwork exposes the contract-only groundwork note
// surface for SEQ-20-P28.
func buildQ20aContractOnlyGroundwork() map[string]any {
	return map[string]any{
		"version":         "q20a-p28.v1",
		"role":            "q20a_contract_only_groundwork",
		"truth_authority": false,
		"sub_step":        "q20a",
		"scope":           "rule_definition_groundwork",
		"not_scope":       "full_temporal_validity_retrieval_execution",
		"live_execution":  false,
		"policy_version":  "s20-tv.v1",
		"mode":            "q20a_contract_only_groundwork_definition",
	}
}

// ===========================================================================
// SEQ-20 q20b temporal validity read policy surfaces (P36 ~ P40)
// ===========================================================================

// buildQ20bTemporalValidityReadPolicyPreparatory exposes the 20-1b
// contract-only temporal read-policy work surface for SEQ-20-P36.
func buildQ20bTemporalValidityReadPolicyPreparatory() map[string]any {
	return map[string]any{
		"version":         "q20b-p36.v1",
		"role":            "q20b_temporal_validity_read_policy_preparatory",
		"truth_authority": false,
		"sub_step":        "q20b",
		"contract_owner":  "query_class_contract_qr1a",
		"status":          "contract_only_closed",
		"policy_version":  "s20-tv.v1",
		"mode":            "q20b_temporal_validity_read_policy_preparatory_definition",
	}
}

// buildQ20bV1TemporalValidityReadPolicy exposes the q20b.v1 temporal validity
// read policy contract surface for SEQ-20-P37.
func buildQ20bV1TemporalValidityReadPolicy() map[string]any {
	return map[string]any{
		"version":         "q20b.v1",
		"role":            "temporal_validity_read_policy",
		"truth_authority": false,
		"sub_step":        "q20b",
		"contract_owner":  "query_class_contract_qr1a",
		"derives_from":    "q20a.v1",
		"replaces":        "implicit_current_vs_old_truth",
		"policy_version":  "s20-tv.v1",
		"mode":            "q20b_v1_temporal_validity_read_policy_definition",
	}
}

// buildQ20bReadPriorityModes exposes the read priority / current_truth_read_mode
// / old_truth_read_mode / separation_rule surface for SEQ-20-P38.
func buildQ20bReadPriorityModes() map[string]any {
	return map[string]any{
		"version":         "q20b-p38.v1",
		"role":            "q20b_read_priority_modes",
		"truth_authority": false,
		"sub_step":        "q20b",
		"read_modes": []map[string]any{
			{
				"mode":               "current_truth_first",
				"example_query":      "지금 며칠째야?",
				"read_priority":      1,
				"current_truth_mode": "prefer",
				"old_truth_mode":     "block",
				"separation_rule":    "current_only",
			},
			{
				"mode":               "exact_past_event_first",
				"example_query":      "어제 무슨 일이 있었지?",
				"read_priority":      2,
				"current_truth_mode": "compare",
				"old_truth_mode":     "prefer",
				"separation_rule":    "event_lane_separate",
			},
			{
				"mode":               "bounded_validity_window_first",
				"example_query":      "지난주에 무슨 일이 있었지?",
				"read_priority":      3,
				"current_truth_mode": "compare",
				"old_truth_mode":     "windowed",
				"separation_rule":    "window_lane_separate",
			},
			{
				"mode":               "chronology_reconstruction_first",
				"example_query":      "What happened before the harbor festival?",
				"read_priority":      4,
				"current_truth_mode": "context",
				"old_truth_mode":     "reconstruct",
				"separation_rule":    "chronology_lane_separate",
			},
		},
		"policy_version": "s20-tv.v1",
		"mode":           "q20b_read_priority_modes_definition",
	}
}

// buildQ20bMirroredAtRecallIntentAndQueryClass exposes the surface that q20b
// is mirrored at both top-level recall intent and query_class_contract for
// SEQ-20-P39.
func buildQ20bMirroredAtRecallIntentAndQueryClass() map[string]any {
	return map[string]any{
		"version":         "q20b-p39.v1",
		"role":            "q20b_mirrored_at_recall_intent_and_query_class",
		"truth_authority": false,
		"sub_step":        "q20b",
		"mirror_targets": []string{
			"top_level_recall_intent",
			"query_class_contract",
		},
		"purpose":        "one_owner_decision_surface",
		"policy_version": "s20-tv.v1",
		"mode":           "q20b_mirrored_at_recall_intent_and_query_class_definition",
	}
}

// buildQ20bStopsBeforeLaterTVWork exposes the boundary note that q20b
// deliberately stops before 20-1c/1d/1e for SEQ-20-P40.
func buildQ20bStopsBeforeLaterTVWork() map[string]any {
	return map[string]any{
		"version":         "q20b-p40.v1",
		"role":            "q20b_stops_before_later_tv_work",
		"truth_authority": false,
		"sub_step":        "q20b",
		"stops_before": []string{
			"20-1c_invalidation_support",
			"20-1d_promotion_lag_pending_current",
			"20-1e_hot_buffer_widening",
		},
		"no_backfill":    true,
		"policy_version": "s20-tv.v1",
		"mode":           "q20b_stops_before_later_tv_work_definition",
	}
}

// ===========================================================================
// SEQ-20 q20c temporal event invalidation support surfaces (P47 ~ P51)
// ===========================================================================

// buildQ20cTemporalEventInvalidationPreparatory exposes the 20-1c contract-only
// event retrieval / invalidation support work surface for SEQ-20-P47.
func buildQ20cTemporalEventInvalidationPreparatory() map[string]any {
	return map[string]any{
		"version":         "q20c-p47.v1",
		"role":            "q20c_temporal_event_invalidation_preparatory",
		"truth_authority": false,
		"sub_step":        "q20c",
		"contract_owner":  "query_class_contract_qr1a",
		"status":          "contract_only_closed",
		"policy_version":  "s20-tv.v1",
		"mode":            "q20c_temporal_event_invalidation_preparatory_definition",
	}
}

// buildQ20cV1TemporalEventInvalidationSupport exposes the q20c.v1 temporal event
// invalidation support contract surface for SEQ-20-P48.
func buildQ20cV1TemporalEventInvalidationSupport() map[string]any {
	return map[string]any{
		"version":         "q20c.v1",
		"role":            "temporal_event_invalidation_support",
		"truth_authority": false,
		"sub_step":        "q20c",
		"contract_owner":  "query_class_contract_qr1a",
		"replaces":        "implicit_event_compare_vs_blocked_current_truth",
		"policy_version":  "s20-tv.v1",
		"mode":            "q20c_v1_temporal_event_invalidation_support_definition",
	}
}

// buildQ20cInvalidationModes exposes the invalidation mode surface for
// SEQ-20-P49.
func buildQ20cInvalidationModes() map[string]any {
	return map[string]any{
		"version":         "q20c-p49.v1",
		"role":            "q20c_invalidation_modes",
		"truth_authority": false,
		"sub_step":        "q20c",
		"modes": []map[string]any{
			{
				"mode":          "off",
				"active_when":   "current_clock_query",
				"example_query": "지금 며칠째야?",
			},
			{
				"mode":          "exact_event_compare_note",
				"active_when":   "exact_past_event_query",
				"example_query": "어제 무슨 일이 있었지?",
			},
			{
				"mode":          "bounded_window_compare_note",
				"active_when":   "bounded_window_query",
				"example_query": "지난주에 무슨 일이 있었지?",
			},
			{
				"mode":          "chronology_compare_note",
				"active_when":   "chronology_query",
				"example_query": "What happened before the harbor festival?",
			},
		},
		"reuses_vocabulary": "direct_evidence_owner",
		"policy_version":    "s20-tv.v1",
		"mode":              "q20c_invalidation_modes_definition",
	}
}

// buildQ20cMirroredAtRecallIntent exposes the surface that q20c is mirrored at
// the top-level recall intent for SEQ-20-P50.
func buildQ20cMirroredAtRecallIntent() map[string]any {
	return map[string]any{
		"version":         "q20c-p50.v1",
		"role":            "q20c_mirrored_at_recall_intent",
		"truth_authority": false,
		"sub_step":        "q20c",
		"mirror_target":   "_build_recall_intent_contract_q3a",
		"mirror_payload":  "temporal_event_invalidation_support",
		"purpose":         "reuse_one_owner_surface",
		"policy_version":  "s20-tv.v1",
		"mode":            "q20c_mirrored_at_recall_intent_definition",
	}
}

// buildQ20cSeparateFromPromotionLag exposes the separation note that q20c keeps
// event invalidation separate from promotion-lag for SEQ-20-P51.
func buildQ20cSeparateFromPromotionLag() map[string]any {
	return map[string]any{
		"version":         "q20c-p51.v1",
		"role":            "q20c_separate_from_promotion_lag",
		"truth_authority": false,
		"sub_step":        "q20c",
		"q20c_owns":       "event_invalidation_support",
		"q20d_owns":       "pending_current_note_criteria",
		"no_overlap":      true,
		"policy_version":  "s20-tv.v1",
		"mode":            "q20c_separate_from_promotion_lag_definition",
	}
}

// ===========================================================================
// SEQ-20 q20d temporal promotion-lag support surfaces (P57 ~ P60)
// ===========================================================================

// buildQ20dTemporalPromotionLagPreparatory exposes the 20-1d contract-only
// promotion-lag pending-current work surface for SEQ-20-P57.
func buildQ20dTemporalPromotionLagPreparatory() map[string]any {
	return map[string]any{
		"version":         "q20d-p57.v1",
		"role":            "q20d_temporal_promotion_lag_preparatory",
		"truth_authority": false,
		"sub_step":        "q20d",
		"contract_owner":  "query_class_contract_qr1a",
		"status":          "contract_only_closed",
		"policy_version":  "s20-tv.v1",
		"mode":            "q20d_temporal_promotion_lag_preparatory_definition",
	}
}

// buildQ20dV1TemporalPromotionLagSupport exposes the q20d.v1 temporal promotion
// lag support contract surface for SEQ-20-P58.
func buildQ20dV1TemporalPromotionLagSupport() map[string]any {
	return map[string]any{
		"version":         "q20d.v1",
		"role":            "temporal_promotion_lag_support",
		"truth_authority": false,
		"sub_step":        "q20d",
		"contract_owner":  "query_class_contract_qr1a",
		"scope":           "pending_current_note_emission",
		"active_for": []string{
			"exact_past_event_reads",
			"bounded_window_reads",
		},
		"inactive_for": []string{
			"current_clock_reads",
		},
		"deferred_for": []string{
			"chronology_reads",
		},
		"policy_version": "s20-tv.v1",
		"mode":           "q20d_v1_temporal_promotion_lag_support_definition",
	}
}

// buildQ20dAnchorPrecedence exposes the anchor precedence and deferred widening
// note surface for SEQ-20-P59.
func buildQ20dAnchorPrecedence() map[string]any {
	return map[string]any{
		"version":         "q20d-p59.v1",
		"role":            "q20d_anchor_precedence",
		"truth_authority": false,
		"sub_step":        "q20d",
		"anchor_precedence": []string{
			"latest_direct_evidence",
			"recent_raw_turn",
		},
		"multi_turn_widening": "latest_turn_only_deferred_to_q20e",
		"current_clock":       "off",
		"chronology":          "deferred",
		"policy_version":      "s20-tv.v1",
		"mode":                "q20d_anchor_precedence_definition",
	}
}

// buildQ20dMirroredAtRecallIntent exposes the surface that q20d is mirrored at
// the top-level recall intent for SEQ-20-P60.
func buildQ20dMirroredAtRecallIntent() map[string]any {
	return map[string]any{
		"version":         "q20d-p60.v1",
		"role":            "q20d_mirrored_at_recall_intent",
		"truth_authority": false,
		"sub_step":        "q20d",
		"mirror_target":   "_build_recall_intent_contract_q3a",
		"mirror_payload":  "temporal_promotion_lag_support",
		"purpose":         "one_owner_pending_current",
		"policy_version":  "s20-tv.v1",
		"mode":            "q20d_mirrored_at_recall_intent_definition",
	}
}

// ===========================================================================
// SEQ-20 q20e temporal hot recall buffer surfaces (P66 ~ P69)
// ===========================================================================

// buildQ20eTemporalHotRecallBufferPreparatory exposes the 20-1e contract-only
// recent multi-turn hot recall buffer work surface for SEQ-20-P66.
func buildQ20eTemporalHotRecallBufferPreparatory() map[string]any {
	return map[string]any{
		"version":         "q20e-p66.v1",
		"role":            "q20e_temporal_hot_recall_buffer_preparatory",
		"truth_authority": false,
		"sub_step":        "q20e",
		"contract_owner":  "query_class_contract_qr1a",
		"status":          "contract_only_closed",
		"policy_version":  "s20-tv.v1",
		"mode":            "q20e_temporal_hot_recall_buffer_preparatory_definition",
	}
}

// buildQ20eV1TemporalHotRecallBuffer exposes the q20e.v1 temporal hot recall
// buffer contract surface for SEQ-20-P67.
func buildQ20eV1TemporalHotRecallBuffer() map[string]any {
	return map[string]any{
		"version":         "q20e.v1",
		"role":            "temporal_hot_recall_buffer",
		"truth_authority": false,
		"sub_step":        "q20e",
		"contract_owner":  "query_class_contract_qr1a",
		"widens_from":     "q20d_single_turn_anchor",
		"widens_to":       "recent_multi_turn_bridge",
		"policy_version":  "s20-tv.v1",
		"mode":            "q20e_v1_temporal_hot_recall_buffer_definition",
	}
}

// buildQ20eBridgeSourceSet exposes the bridge source set and bounded window
// surface for SEQ-20-P68.
func buildQ20eBridgeSourceSet() map[string]any {
	return map[string]any{
		"version":         "q20e-p68.v1",
		"role":            "q20e_bridge_source_set",
		"truth_authority": false,
		"sub_step":        "q20e",
		"bridge_sources": []string{
			"latest_direct_evidence",
			"scoped_verbatim_support",
			"recent_raw_turn",
		},
		"hot_window_turns": map[string]int{
			"min":     2,
			"default": 3,
			"max":     4,
		},
		"support_only":       true,
		"truth_override":     false,
		"thin_tag_downgrade": true,
		"policy_version":     "s20-tv.v1",
		"mode":               "q20e_bridge_source_set_definition",
	}
}

// ===========================================================================
// SEQ-20 q20f lightweight entity index surfaces (P76 ~ P81)
// ===========================================================================

// buildQ20fLightweightEntityIndexPreparatory exposes the 20-2a preparatory
// contract-only entity index work surface for SEQ-20-P76.
func buildQ20fLightweightEntityIndexPreparatory() map[string]any {
	return map[string]any{
		"version":         "q20f-p76.v1",
		"role":            "q20f_lightweight_entity_index_preparatory",
		"truth_authority": false,
		"sub_step":        "q20f",
		"contract_owner":  "query_class_contract_qr1a",
		"status":          "contract_only_closed",
		"policy_version":  "s20-eg.v1",
		"mode":            "q20f_lightweight_entity_index_preparatory_definition",
	}
}

// buildQ20fV1LightweightEntityIndex exposes the q20f.v1 lightweight entity
// index contract surface for SEQ-20-P77.
func buildQ20fV1LightweightEntityIndex() map[string]any {
	return map[string]any{
		"version":         "q20f.v1",
		"role":            "lightweight_entity_index",
		"truth_authority": false,
		"sub_step":        "q20f",
		"contract_owner":  "query_class_contract_qr1a",
		"replaces":        "prompt_side_entity_digest_formatting",
		"policy_version":  "s20-eg.v1",
		"mode":            "q20f_v1_lightweight_entity_index_definition",
	}
}

// buildQ20fStructuredStateSurfaces exposes the structured state surface labels
// that the entity index anchors to for SEQ-20-P78.
func buildQ20fStructuredStateSurfaces() map[string]any {
	return map[string]any{
		"version":         "q20f-p78.v1",
		"role":            "q20f_structured_state_surfaces",
		"truth_authority": false,
		"sub_step":        "q20f",
		"indexed_labels": []string{
			"character",
			"location",
			"pending_thread_owner",
			"pending_thread_target",
			"relationship_target",
		},
		"generic_entity_tokens_trusted": false,
		"retrieval_boost_only":          true,
		"policy_version":                "s20-eg.v1",
		"mode":                          "q20f_structured_state_surfaces_definition",
	}
}

// buildQ20fMirroredAtQueryClass exposes the surface that q20f is mirrored at
// query_class_contract for SEQ-20-P79.
func buildQ20fMirroredAtQueryClass() map[string]any {
	return map[string]any{
		"version":         "q20f-p79.v1",
		"role":            "q20f_mirrored_at_query_class",
		"truth_authority": false,
		"sub_step":        "q20f",
		"mirror_target":   "query_class_contract",
		"mirror_payload":  "lightweight_entity_index",
		"purpose":         "one_owner_entity_index",
		"policy_version":  "s20-eg.v1",
		"mode":            "q20f_mirrored_at_query_class_definition",
	}
}

// buildQ20fStopsBeforeGraphLikeSupport exposes the boundary surface that q20f
// stops before 20-2b/20-2c for SEQ-20-P80.
func buildQ20fStopsBeforeGraphLikeSupport() map[string]any {
	return map[string]any{
		"version":         "q20f-p80.v1",
		"role":            "q20f_stops_before_graph_like_support",
		"truth_authority": false,
		"sub_step":        "q20f",
		"stops_before": []string{
			"graph_like_support_signal",
			"entity_graph_boost_inspection_surface",
		},
		"scope":          "entity_side_index_only",
		"no_backfill":    true,
		"policy_version": "s20-eg.v1",
		"mode":           "q20f_stops_before_graph_like_support_definition",
	}
}

// buildQ20fTokenBoundaryStructuredLabels exposes the token-boundary matching
// rule surface that replaced the temporary Korean particle suffix table for
// SEQ-20-P81.
func buildQ20fTokenBoundaryStructuredLabels() map[string]any {
	return map[string]any{
		"version":                     "q20f-p81.v1",
		"role":                        "q20f_token_boundary_structured_labels",
		"truth_authority":             false,
		"sub_step":                    "q20f",
		"matching_rule":               "token_boundary_only",
		"preserves_attached_forms":    true,
		"blocks_mid_token_substrings": true,
		"example_attached_ok": []string{
			"미나는",
			"항구에서",
			"준과의",
		},
		"example_mid_token_blocked": []string{
			"기준 -> 준",
		},
		"policy_version": "s20-eg.v1",
		"mode":           "q20f_token_boundary_structured_labels_definition",
	}
}

// ===========================================================================
// SEQ-20 q20g graph-like support signal surfaces (P89 ~ P93)
// ===========================================================================

// buildQ20gGraphLikeSupportSignalPreparatory exposes the 20-2b preparatory
// contract-only graph-like support signal work surface for SEQ-20-P89.
func buildQ20gGraphLikeSupportSignalPreparatory() map[string]any {
	return map[string]any{
		"version":         "q20g-p89.v1",
		"role":            "q20g_graph_like_support_signal_preparatory",
		"truth_authority": false,
		"sub_step":        "q20g",
		"contract_owner":  "query_class_contract_qr1a",
		"status":          "contract_only_closed",
		"policy_version":  "s20-eg.v1",
		"mode":            "q20g_graph_like_support_signal_preparatory_definition",
	}
}

// buildQ20gV1GraphLikeSupportSignal exposes the q20g.v1 graph-like support
// signal contract surface for SEQ-20-P90.
func buildQ20gV1GraphLikeSupportSignal() map[string]any {
	return map[string]any{
		"version":         "q20g.v1",
		"role":            "graph_like_support_signal",
		"truth_authority": false,
		"sub_step":        "q20g",
		"contract_owner":  "query_class_contract_qr1a",
		"activation_gate": "q20f.v1_structured_entity_focus_terms_and_structured_pair_link",
		"policy_version":  "s20-eg.v1",
		"mode":            "q20g_v1_graph_like_support_signal_definition",
	}
}

// buildQ20gPairSourcesAndFailOpen exposes the pair source lanes and fail-open
// behavior surface for SEQ-20-P91.
func buildQ20gPairSourcesAndFailOpen() map[string]any {
	return map[string]any{
		"version":         "q20g-p91.v1",
		"role":            "q20g_pair_sources_and_fail_open",
		"truth_authority": false,
		"sub_step":        "q20g",
		"pair_sources": []string{
			"session_state.relationships_json",
			"pending_threads_owner_target_pairs",
		},
		"graph_support_mode": "optional_graph_accelerator",
		"fail_open_no_pair":  true,
		"required_read_lane": false,
		"policy_version":     "s20-eg.v1",
		"mode":               "q20g_pair_sources_and_fail_open_definition",
	}
}

// buildQ20gMirroredAtQueryClass exposes the surface that q20g is mirrored at
// query_class_contract for SEQ-20-P92.
func buildQ20gMirroredAtQueryClass() map[string]any {
	return map[string]any{
		"version":         "q20g-p92.v1",
		"role":            "q20g_mirrored_at_query_class",
		"truth_authority": false,
		"sub_step":        "q20g",
		"mirror_target":   "query_class_contract",
		"mirror_payload":  "graph_like_support_signal",
		"purpose":         "one_owner_graph_like_signal",
		"policy_version":  "s20-eg.v1",
		"mode":            "q20g_mirrored_at_query_class_definition",
	}
}

// buildQ20gStopsBeforeInspectionFormatting exposes the boundary surface that
// q20g stops before 20-2c/20-2g for SEQ-20-P93.
func buildQ20gStopsBeforeInspectionFormatting() map[string]any {
	return map[string]any{
		"version":         "q20g-p93.v1",
		"role":            "q20g_stops_before_inspection_formatting",
		"truth_authority": false,
		"sub_step":        "q20g",
		"stops_before": []string{
			"entity_graph_boost_inspection_surface",
			"relation_edge_support_ledger",
		},
		"scope":          "optional_pair_link_accelerator_only",
		"no_backfill":    true,
		"policy_version": "s20-eg.v1",
		"mode":           "q20g_stops_before_inspection_formatting_definition",
	}
}

// ===========================================================================
// SEQ-20 q20h entity/graph boost inspection surface surfaces (P99 ~ P102)
// ===========================================================================

// buildQ20hEntityGraphBoostInspectionSurfacePreparatory exposes the 20-2c
// preparatory contract-only inspection surface work for SEQ-20-P99.
func buildQ20hEntityGraphBoostInspectionSurfacePreparatory() map[string]any {
	return map[string]any{
		"version":         "q20h-p99.v1",
		"role":            "q20h_entity_graph_boost_inspection_surface_preparatory",
		"truth_authority": false,
		"sub_step":        "q20h",
		"contract_owner":  "query_class_contract_qr1a",
		"status":          "contract_only_closed",
		"policy_version":  "s20-eg.v1",
		"mode":            "q20h_entity_graph_boost_inspection_surface_preparatory_definition",
	}
}

// buildQ20hV1EntityGraphBoostInspectionSurface exposes the q20h.v1
// entity/graph boost inspection surface contract for SEQ-20-P100.
func buildQ20hV1EntityGraphBoostInspectionSurface() map[string]any {
	return map[string]any{
		"version":         "q20h.v1",
		"role":            "entity_graph_boost_inspection_surface",
		"truth_authority": false,
		"sub_step":        "q20h",
		"contract_owner":  "query_class_contract_qr1a",
		"mirrors": []string{
			"entity_focus_terms",
			"source_catalogs",
			"graph_source_lanes",
			"compact_graph_candidate_previews",
		},
		"policy_version": "s20-eg.v1",
		"mode":           "q20h_v1_entity_graph_boost_inspection_surface_definition",
	}
}

// buildQ20hInspectionRoleAndAuthorityNotice exposes the inspection surface
// role and authority notice for SEQ-20-P101.
func buildQ20hInspectionRoleAndAuthorityNotice() map[string]any {
	return map[string]any{
		"version":                 "q20h-p101.v1",
		"role":                    "q20h_inspection_role_and_authority_notice",
		"truth_authority":         false,
		"sub_step":                "q20h",
		"inspection_surface_mode": "entity_graph_boost_trace",
		"inspection_role":         "read_only_support_trace",
		"authority_notice":        "support_only_accelerator_not_truth",
		"policy_version":          "s20-eg.v1",
		"mode":                    "q20h_inspection_role_and_authority_notice_definition",
	}
}

// buildQ20hMirroredAtQueryClass exposes the surface that q20h is mirrored at
// query_class_contract for SEQ-20-P102.
func buildQ20hMirroredAtQueryClass() map[string]any {
	return map[string]any{
		"version":         "q20h-p102.v1",
		"role":            "q20h_mirrored_at_query_class",
		"truth_authority": false,
		"sub_step":        "q20h",
		"mirror_target":   "query_class_contract",
		"mirror_payload":  "entity_graph_boost_inspection_surface",
		"purpose":         "one_owner_inspection_payload",
		"policy_version":  "s20-eg.v1",
		"mode":            "q20h_mirrored_at_query_class_definition",
	}
}

// ===========================================================================
// SEQ-20 q20i lagging current state boost surfaces (P109 ~ P112)
// ===========================================================================

// buildQ20iLaggingCurrentStateBoostPreparatory exposes the 20-2d preparatory
// contract-only lagging-current-state boost work surface for SEQ-20-P109.
func buildQ20iLaggingCurrentStateBoostPreparatory() map[string]any {
	return map[string]any{
		"version":         "q20i-p109.v1",
		"role":            "q20i_lagging_current_state_boost_preparatory",
		"truth_authority": false,
		"sub_step":        "q20i",
		"contract_owner":  "query_class_contract_qr1a",
		"status":          "contract_only_closed",
		"policy_version":  "s20-eg.v1",
		"mode":            "q20i_lagging_current_state_boost_preparatory_definition",
	}
}

// buildQ20iV1LaggingCurrentStateBoost exposes the q20i.v1 lagging current state
// boost contract surface for SEQ-20-P110.
func buildQ20iV1LaggingCurrentStateBoost() map[string]any {
	return map[string]any{
		"version":         "q20i.v1",
		"role":            "lagging_current_state_boost",
		"truth_authority": false,
		"sub_step":        "q20i",
		"contract_owner":  "query_class_contract_qr1a",
		"composes": []string{
			"temporal_promotion_lag_support",
			"temporal_hot_recall_buffer",
			"lightweight_entity_index",
			"graph_like_support_signal",
		},
		"rescue_rule":    "support_only",
		"policy_version": "s20-eg.v1",
		"mode":           "q20i_v1_lagging_current_state_boost_definition",
	}
}

// buildQ20iActivationAndPrecedence exposes the activation gate and boost
// precedence surface for SEQ-20-P111.
func buildQ20iActivationAndPrecedence() map[string]any {
	return map[string]any{
		"version":         "q20i-p111.v1",
		"role":            "q20i_activation_and_precedence",
		"truth_authority": false,
		"sub_step":        "q20i",
		"activation_requires": []string{
			"pending_current_support_active",
			"structured_entity_focus_exists",
		},
		"chronology": "deferred",
		"boost_precedence": []string{
			"recent_multi_turn_bridge",
			"optional_graph_pair",
			"entity_side_index",
		},
		"support_only_accelerator_not_truth": true,
		"policy_version":                     "s20-eg.v1",
		"mode":                               "q20i_activation_and_precedence_definition",
	}
}

// buildQ20iMirroredAtQueryClass exposes the surface that q20i is mirrored at
// query_class_contract for SEQ-20-P112.
func buildQ20iMirroredAtQueryClass() map[string]any {
	return map[string]any{
		"version":         "q20i-p112.v1",
		"role":            "q20i_mirrored_at_query_class",
		"truth_authority": false,
		"sub_step":        "q20i",
		"mirror_target":   "query_class_contract",
		"mirror_payload":  "lagging_current_state_boost",
		"purpose":         "one_owner_current_state_gap_rescue",
		"policy_version":  "s20-eg.v1",
		"mode":            "q20i_mirrored_at_query_class_definition",
	}
}

// ===========================================================================
// SEQ-20 q20j motive-shadow hint surfaces (P118 ~ P121)
// ===========================================================================

// buildQ20jMotiveShadowHintPreparatory exposes the 20-2e preparatory
// contract-only motive-shadow hint work surface for SEQ-20-P118.
func buildQ20jMotiveShadowHintPreparatory() map[string]any {
	return map[string]any{
		"version":         "q20j-p118.v1",
		"role":            "q20j_motive_shadow_hint_preparatory",
		"truth_authority": false,
		"sub_step":        "q20j",
		"contract_owner":  "query_class_contract_qr1a",
		"status":          "contract_only_closed",
		"policy_version":  "s20-eg.v1",
		"mode":            "q20j_motive_shadow_hint_preparatory_definition",
	}
}

// buildQ20jV1MotiveShadowHint exposes the q20j.v1 motive-shadow hint contract
// surface for SEQ-20-P119.
func buildQ20jV1MotiveShadowHint() map[string]any {
	return map[string]any{
		"version":         "q20j.v1",
		"role":            "motive_shadow_hint",
		"truth_authority": false,
		"sub_step":        "q20j",
		"contract_owner":  "query_class_contract_qr1a",
		"signal_whitelist": []string{
			"drive",
			"vulnerability",
			"surface_persona",
			"attachment",
			"fixation",
		},
		"source":         "structured_character_personality_state",
		"query_anchor":   "character_anchored_only",
		"policy_version": "s20-eg.v1",
		"mode":           "q20j_v1_motive_shadow_hint_definition",
	}
}

// buildQ20jTruthWriteForbidden exposes the truth-write-forbidden guard surface
// for SEQ-20-P120.
func buildQ20jTruthWriteForbidden() map[string]any {
	return map[string]any{
		"version":                        "q20j-p120.v1",
		"role":                           "q20j_truth_write_forbidden",
		"truth_authority":                false,
		"sub_step":                       "q20j",
		"truth_write_mode":               "forbidden",
		"hint_role":                      "support_only",
		"stops_before_branch_escalation": true,
		"stops_before_foreground":        true,
		"policy_version":                 "s20-eg.v1",
		"mode":                           "q20j_truth_write_forbidden_definition",
	}
}

// buildQ20jMirroredAtQueryClass exposes the surface that q20j is mirrored at
// query_class_contract for SEQ-20-P121.
func buildQ20jMirroredAtQueryClass() map[string]any {
	return map[string]any{
		"version":         "q20j-p121.v1",
		"role":            "q20j_mirrored_at_query_class",
		"truth_authority": false,
		"sub_step":        "q20j",
		"mirror_target":   "query_class_contract",
		"mirror_payload":  "motive_shadow_hint",
		"purpose":         "one_owner_bounded_motive_hint",
		"policy_version":  "s20-eg.v1",
		"mode":            "q20j_mirrored_at_query_class_definition",
	}
}

// buildQ20eMirroredAtRecallIntent exposes the surface that q20e is mirrored at
// the top-level recall intent for SEQ-20-P69.
func buildQ20eMirroredAtRecallIntent() map[string]any {
	return map[string]any{
		"version":         "q20e-p69.v1",
		"role":            "q20e_mirrored_at_recall_intent",
		"truth_authority": false,
		"sub_step":        "q20e",
		"mirror_target":   "_build_recall_intent_contract_q3a",
		"mirror_payload":  "temporal_hot_recall_buffer",
		"purpose":         "one_owner_hot_bridge_policy",
		"policy_version":  "s20-tv.v1",
		"mode":            "q20e_mirrored_at_recall_intent_definition",
	}
}

// ===========================================================================
// SEQ-20 q20k motive-shadow non-escalation guard surfaces (P127 ~ P129)
// ===========================================================================

// buildQ20kMotiveShadowNonEscalationGuardPreparatory exposes the 20-2f
// preparatory contract-only motive-shadow non-escalation guard work surface
// for SEQ-20-P127.
func buildQ20kMotiveShadowNonEscalationGuardPreparatory() map[string]any {
	return map[string]any{
		"version":         "q20k-p127.v1",
		"role":            "q20k_motive_shadow_non_escalation_guard_preparatory",
		"truth_authority": false,
		"sub_step":        "q20k",
		"contract_owner":  "query_class_contract_qr1a",
		"status":          "contract_only_closed",
		"policy_version":  "s20-eg.v1",
		"mode":            "q20k_motive_shadow_non_escalation_guard_preparatory_definition",
	}
}

// buildQ20kV1MotiveShadowNonEscalationGuard exposes the q20k.v1
// motive_shadow_non_escalation_guard contract surface for SEQ-20-P128.
func buildQ20kV1MotiveShadowNonEscalationGuard() map[string]any {
	return map[string]any{
		"version":                            "q20k.v1",
		"role":                               "motive_shadow_non_escalation_guard",
		"truth_authority":                    false,
		"sub_step":                           "q20k",
		"contract_owner":                     "query_class_contract_qr1a",
		"lane":                               "support_only_disambiguation_hint",
		"blocked_write_targets":              []string{"current_fact", "canonical_relationship_state"},
		"prevents_stale_arc_auto_foreground": true,
		"policy_version":                     "s20-eg.v1",
		"mode":                               "q20k_v1_motive_shadow_non_escalation_guard_definition",
	}
}

// buildQ20kMirroredAtQueryClass exposes the surface that q20k is mirrored at
// query_class_contract for SEQ-20-P129.
func buildQ20kMirroredAtQueryClass() map[string]any {
	return map[string]any{
		"version":         "q20k-p129.v1",
		"role":            "q20k_mirrored_at_query_class",
		"truth_authority": false,
		"sub_step":        "q20k",
		"mirror_target":   "query_class_contract",
		"mirror_payload":  "motive_shadow_non_escalation_guard",
		"purpose":         "one_owner_non_escalation_boundary",
		"policy_version":  "s20-eg.v1",
		"mode":            "q20k_mirrored_at_query_class_definition",
	}
}

// ===========================================================================
// SEQ-20 q20l relation edge support ledger surfaces (P135 ~ P138)
// ===========================================================================

// buildQ20lRelationEdgeSupportLedgerPreparatory exposes the 20-2g
// preparatory contract-only relation edge support ledger work surface
// for SEQ-20-P135.
func buildQ20lRelationEdgeSupportLedgerPreparatory() map[string]any {
	return map[string]any{
		"version":         "q20l-p135.v1",
		"role":            "q20l_relation_edge_support_ledger_preparatory",
		"truth_authority": false,
		"sub_step":        "q20l",
		"contract_owner":  "query_class_contract_qr1a",
		"status":          "contract_only_closed",
		"policy_version":  "s20-eg.v1",
		"mode":            "q20l_relation_edge_support_ledger_preparatory_definition",
	}
}

// buildQ20lV1RelationEdgeSupportLedger exposes the q20l.v1
// relation_edge_support_ledger contract surface for SEQ-20-P136.
func buildQ20lV1RelationEdgeSupportLedger() map[string]any {
	return map[string]any{
		"version":         "q20l.v1",
		"role":            "relation_edge_support_ledger",
		"truth_authority": false,
		"sub_step":        "q20l",
		"contract_owner":  "query_class_contract_qr1a",
		"source":          "structured_relationship_state_only",
		"summary_fields": []string{
			"pair",
			"current_dynamic",
			"trust_level",
			"imbalance",
			"recent_shift",
		},
		"pending_thread_pair_alone_blocked": true,
		"policy_version":                    "s20-eg.v1",
		"mode":                              "q20l_v1_relation_edge_support_ledger_definition",
	}
}

// buildQ20lGraphTruthWriteForbidden exposes the graph-truth-write-forbidden
// guard surface for SEQ-20-P137.
func buildQ20lGraphTruthWriteForbidden() map[string]any {
	return map[string]any{
		"version":                       "q20l-p137.v1",
		"role":                          "q20l_graph_truth_write_forbidden",
		"truth_authority":               false,
		"sub_step":                      "q20l",
		"graph_truth_write_mode":        "forbidden",
		"support_only":                  true,
		"graph_truth_promotion_blocked": true,
		"policy_version":                "s20-eg.v1",
		"mode":                          "q20l_graph_truth_write_forbidden_definition",
	}
}

// buildQ20lMirroredAtQueryClass exposes the surface that q20l is mirrored at
// query_class_contract for SEQ-20-P138.
func buildQ20lMirroredAtQueryClass() map[string]any {
	return map[string]any{
		"version":         "q20l-p138.v1",
		"role":            "q20l_mirrored_at_query_class",
		"truth_authority": false,
		"sub_step":        "q20l",
		"mirror_target":   "query_class_contract",
		"mirror_payload":  "relation_edge_support_ledger",
		"purpose":         "one_owner_bounded_relation_edge",
		"policy_version":  "s20-eg.v1",
		"mode":            "q20l_mirrored_at_query_class_definition",
	}
}

// ===========================================================================
// SEQ-20 aggregate summary surfaces (P231 ~ P236)
// ===========================================================================

// buildSeq20P231ValidityPriority exposes the validity-priority summary surface
// for SEQ-20-P231: recency보다 validity/event가 먼저여야 함.
func buildSeq20P231ValidityPriority() map[string]any {
	return map[string]any{
		"version":         "s20-p231.v1",
		"role":            "seq20_validity_priority",
		"truth_authority": false,
		"sub_step":        "aggregate_summary",
		"priority_rule":   "validity_event_before_recency",
		"policy_version":  "s20-tv.v1",
		"mode":            "seq20_validity_priority_definition",
	}
}

// buildSeq20P232SupportOnlyAccelerator exposes the support-only accelerator
// summary surface for SEQ-20-P232: entity/graph는 boost lane이어야 함.
func buildSeq20P232SupportOnlyAccelerator() map[string]any {
	return map[string]any{
		"version":          "s20-p232.v1",
		"role":             "seq20_support_only_accelerator",
		"truth_authority":  false,
		"sub_step":         "aggregate_summary",
		"accelerator_lane": "support_only_boost",
		"disallowed":       []string{"truth_write", "canonical_overwrite", "direct_override"},
		"policy_version":   "s20-eg.v1",
		"mode":             "seq20_support_only_accelerator_definition",
	}
}

// buildSeq20P233AmbiguityReduction exposes the ambiguity-reduction summary
// surface for SEQ-20-P233: 더 덜 헷갈리는 후보를 만들어야 함.
func buildSeq20P233AmbiguityReduction() map[string]any {
	return map[string]any{
		"version":         "s20-p233.v1",
		"role":            "seq20_ambiguity_reduction",
		"truth_authority": false,
		"sub_step":        "aggregate_summary",
		"goal":            "narrow_candidates_without_truth_manipulation",
		"method":          "disambiguation_support_only",
		"policy_version":  "s20-eg.v1",
		"mode":            "seq20_ambiguity_reduction_definition",
	}
}

// buildSeq20P234InspectionVisibility exposes the inspection-visibility summary
// surface for SEQ-20-P234: temporal/entity boost 근거가 보여야 함.
func buildSeq20P234InspectionVisibility() map[string]any {
	return map[string]any{
		"version":         "s20-p234.v1",
		"role":            "seq20_inspection_visibility",
		"truth_authority": false,
		"sub_step":        "aggregate_summary",
		"inspection_surfaces": []string{
			"temporal_boost_rationale",
			"entity_boost_rationale",
			"graph_boost_rationale",
		},
		"policy_version": "s20-eg.v1",
		"mode":           "seq20_inspection_visibility_definition",
	}
}

// buildSeq20P235TruthPrecedencePreserve exposes the truth-precedence-preserve
// summary surface for SEQ-20-P235: support lane이 canonical/direct evidence 아래.
func buildSeq20P235TruthPrecedencePreserve() map[string]any {
	return map[string]any{
		"version":         "s20-p235.v1",
		"role":            "seq20_truth_precedence_preserve",
		"truth_authority": false,
		"sub_step":        "aggregate_summary",
		"precedence_order": []string{
			"canonical_state",
			"direct_evidence",
			"support_lane",
		},
		"support_lane_ceiling": "read_only_boost",
		"policy_version":       "s20-eg.v1",
		"mode":                 "seq20_truth_precedence_preserve_definition",
	}
}

// buildSeq20P236HotBridge exposes the hot-bridge summary surface for
// SEQ-20-P236: recent-turn hot recall은 bridge lane이지 별도 truth store가 아님.
func buildSeq20P236HotBridge() map[string]any {
	return map[string]any{
		"version":         "s20-p236.v1",
		"role":            "seq20_hot_bridge",
		"truth_authority": false,
		"sub_step":        "aggregate_summary",
		"bridge_lane":     "recent_turn_recall_bridge",
		"is_truth_store":  false,
		"source_set": []string{
			"latest_direct_evidence",
			"scoped_verbatim_support",
			"recent_raw_turn",
		},
		"policy_version": "s20-tv.v1",
		"mode":           "seq20_hot_bridge_definition",
	}
}

// ===========================================================================
// SEQ-20 q20m temporal ambiguity support note surfaces (P258 ~ P259)
// ===========================================================================

// buildQ20mTemporalAmbiguitySupportNotePreparatory exposes the 20-3a
// preparatory contract-only temporal ambiguity support note work surface
// for SEQ-20-P258.
func buildQ20mTemporalAmbiguitySupportNotePreparatory() map[string]any {
	return map[string]any{
		"version":         "q20m-p258.v1",
		"role":            "q20m_temporal_ambiguity_support_note_preparatory",
		"truth_authority": false,
		"sub_step":        "q20m",
		"contract_owner":  "query_class_contract_qr1a",
		"status":          "contract_only_closed",
		"policy_version":  "s20-tv.v1",
		"mode":            "q20m_temporal_ambiguity_support_note_preparatory_definition",
	}
}

// buildQ20mV1TemporalAmbiguitySupportNote exposes the q20m.v1
// temporal_ambiguity_support_note contract surface for SEQ-20-P259.
func buildQ20mV1TemporalAmbiguitySupportNote() map[string]any {
	return map[string]any{
		"version":         "q20m.v1",
		"role":            "temporal_ambiguity_support_note",
		"truth_authority": false,
		"sub_step":        "q20m",
		"contract_owner":  "query_class_contract_qr1a",
		"composes": []string{
			"q20c_exact_event_compare_note",
			"q20d_bounded_window_compare_note",
			"q20e_chronology_support_note",
		},
		"disabled_for_current_clock_query": true,
		"deferred_chronology_gap_visible":  true,
		"fake_fill_blocked":                true,
		"policy_version":                   "s20-tv.v1",
		"mode":                             "q20m_v1_temporal_ambiguity_support_note_definition",
	}
}

// ===========================================================================
// SEQ-20 q20n alias/entity conflict disambiguation surfaces (P260 ~ P261)
// ===========================================================================

// buildQ20nAliasEntityConflictDisambiguationPreparatory exposes the 20-3b
// preparatory contract-only alias/entity conflict disambiguation work surface
// for SEQ-20-P260.
func buildQ20nAliasEntityConflictDisambiguationPreparatory() map[string]any {
	return map[string]any{
		"version":         "q20n-p260.v1",
		"role":            "q20n_alias_entity_conflict_disambiguation_preparatory",
		"truth_authority": false,
		"sub_step":        "q20n",
		"contract_owner":  "query_class_contract_qr1a",
		"status":          "contract_only_closed",
		"policy_version":  "s20-eg.v1",
		"mode":            "q20n_alias_entity_conflict_disambiguation_preparatory_definition",
	}
}

// buildQ20nV1AliasEntityConflictDisambiguation exposes the q20n.v1
// alias_entity_conflict_disambiguation contract surface for SEQ-20-P261.
func buildQ20nV1AliasEntityConflictDisambiguation() map[string]any {
	return map[string]any{
		"version":                         "q20n.v1",
		"role":                            "alias_entity_conflict_disambiguation",
		"truth_authority":                 false,
		"sub_step":                        "q20n",
		"contract_owner":                  "query_class_contract_qr1a",
		"explicit_alias_table":            false,
		"structured_label_collision_only": true,
		"auto_resolution":                 false,
		"output":                          "candidate_entries_only",
		"policy_version":                  "s20-eg.v1",
		"mode":                            "q20n_v1_alias_entity_conflict_disambiguation_definition",
	}
}

// ===========================================================================
// SEQ-20 q20o temporal/entity support block source-tag rule surfaces (P262 ~ P263)
// ===========================================================================

// buildQ20oTemporalEntitySourceTagRulePreparatory exposes the 20-3c
// preparatory contract-only temporal/entity support block source-tag rule work
// surface for SEQ-20-P262.
func buildQ20oTemporalEntitySourceTagRulePreparatory() map[string]any {
	return map[string]any{
		"version":         "q20o-p262.v1",
		"role":            "q20o_temporal_entity_source_tag_rule_preparatory",
		"truth_authority": false,
		"sub_step":        "q20o",
		"contract_owner":  "query_class_contract_qr1a",
		"status":          "contract_only_closed",
		"policy_version":  "s20-eg.v1",
		"mode":            "q20o_temporal_entity_source_tag_rule_preparatory_definition",
	}
}

// buildQ20oV1TemporalEntitySourceTagRule exposes the q20o.v1
// temporal_entity_source_tag_rule contract surface for SEQ-20-P263.
func buildQ20oV1TemporalEntitySourceTagRule() map[string]any {
	return map[string]any{
		"version":         "q20o.v1",
		"role":            "temporal_entity_source_tag_rule",
		"truth_authority": false,
		"sub_step":        "q20o",
		"contract_owner":  "query_class_contract_qr1a",
		"source_catalogs": []string{
			"source_surfaces",
			"source_catalogs",
			"source_lanes",
		},
		"relabeling_blocked": true,
		"fake_alias_blocked": true,
		"policy_version":     "s20-eg.v1",
		"mode":               "q20o_v1_temporal_entity_source_tag_rule_definition",
	}
}

// ===========================================================================
// SEQ-20 q20p canonical-pending/stale-current conflict note surfaces (P264 ~ P265)
// ===========================================================================

// buildQ20pCanonicalPendingStaleCurrentConflictNotePreparatory exposes the
// 20-3d preparatory contract-only canonical-pending/stale-current conflict note
// work surface for SEQ-20-P264.
func buildQ20pCanonicalPendingStaleCurrentConflictNotePreparatory() map[string]any {
	return map[string]any{
		"version":         "q20p-p264.v1",
		"role":            "q20p_canonical_pending_stale_current_conflict_note_preparatory",
		"truth_authority": false,
		"sub_step":        "q20p",
		"contract_owner":  "query_class_contract_qr1a",
		"status":          "contract_only_closed",
		"policy_version":  "s20-tv.v1",
		"mode":            "q20p_canonical_pending_stale_current_conflict_note_preparatory_definition",
	}
}

// buildQ20pV1CanonicalPendingStaleCurrentConflictNote exposes the q20p.v1
// canonical_pending_stale_current_conflict_note contract surface for SEQ-20-P265.
func buildQ20pV1CanonicalPendingStaleCurrentConflictNote() map[string]any {
	return map[string]any{
		"version":         "q20p.v1",
		"role":            "canonical_pending_stale_current_conflict_note",
		"truth_authority": false,
		"sub_step":        "q20p",
		"contract_owner":  "query_class_contract_qr1a",
		"assembles": []string{
			"pending_current_gap",
			"hot_buffer",
			"optional_lagging_boost",
		},
		"read_only_conflict_note": true,
		"policy_version":          "s20-tv.v1",
		"mode":                    "q20p_v1_canonical_pending_stale_current_conflict_note_definition",
	}
}

// ===========================================================================
// SEQ-20 q20q recall cue rescue rule surfaces (P266 ~ P267)
// ===========================================================================

// buildQ20qRecallCueRescueRulePreparatory exposes the 20-3e preparatory
// contract-only recall cue rescue rule work surface for SEQ-20-P266.
func buildQ20qRecallCueRescueRulePreparatory() map[string]any {
	return map[string]any{
		"version":         "q20q-p266.v1",
		"role":            "q20q_recall_cue_rescue_rule_preparatory",
		"truth_authority": false,
		"sub_step":        "q20q",
		"contract_owner":  "query_class_contract_qr1a",
		"status":          "contract_only_closed",
		"policy_version":  "s20-tv.v1",
		"mode":            "q20q_recall_cue_rescue_rule_preparatory_definition",
	}
}

// buildQ20qV1RecallCueRescueRule exposes the q20q.v1 recall_cue_rescue_rule
// contract surface for SEQ-20-P267.
func buildQ20qV1RecallCueRescueRule() map[string]any {
	return map[string]any{
		"version":         "q20q.v1",
		"role":            "recall_cue_rescue_rule",
		"truth_authority": false,
		"sub_step":        "q20q",
		"contract_owner":  "query_class_contract_qr1a",
		"reuses": []string{
			"q20a_temporal_expansion",
			"qr1a_callback_lexical_signal",
			"qr1d_old_detail_signal",
		},
		"new_cue_table":                 false,
		"recall_widening_only":          true,
		"plain_detail_request_excluded": true,
		"policy_version":                "s20-tv.v1",
		"mode":                          "q20q_v1_recall_cue_rescue_rule_definition",
	}
}

// ===========================================================================
// SEQ-20 q20r wide gather -> validity join rule surfaces (P268 ~ P269)
// ===========================================================================

// buildQ20rWideGatherValidityJoinRulePreparatory exposes the 20-3f
// preparatory contract-only wide gather -> validity join rule work surface
// for SEQ-20-P268.
func buildQ20rWideGatherValidityJoinRulePreparatory() map[string]any {
	return map[string]any{
		"version":         "q20r-p268.v1",
		"role":            "q20r_wide_gather_validity_join_rule_preparatory",
		"truth_authority": false,
		"sub_step":        "q20r",
		"contract_owner":  "query_class_contract_qr1a",
		"status":          "contract_only_closed",
		"policy_version":  "s20-tv.v1",
		"mode":            "q20r_wide_gather_validity_join_rule_preparatory_definition",
	}
}

// buildQ20rV1WideGatherValidityJoinRule exposes the q20r.v1
// wide_gather_validity_join_rule contract surface for SEQ-20-P269.
func buildQ20rV1WideGatherValidityJoinRule() map[string]any {
	return map[string]any{
		"version":         "q20r.v1",
		"role":            "wide_gather_validity_join_rule",
		"truth_authority": false,
		"sub_step":        "q20r",
		"contract_owner":  "query_class_contract_qr1a",
		"reuses": []string{
			"q20b_read_priority",
			"q20m_compare_note_surface",
			"q20q_rescue_surface",
		},
		"validity_join_authorities": []string{
			"mariadb_canonical_truth",
			"storyline_status",
			"temporal_validity",
		},
		"bounded_wide_gather":                     true,
		"validity_join_for_temporal_queries_only": true,
		"callback_only_recall_fail_open":          true,
		"policy_version":                          "s20-tv.v1",
		"mode":                                    "q20r_v1_wide_gather_validity_join_rule_definition",
	}
}

// ===========================================================================
// SEQ-20 q20s thin support tag fallback surfaces (P270 ~ P271)
// ===========================================================================

// buildQ20sThinSupportTagFallbackPreparatory exposes the 20-3g preparatory
// contract-only thin support tag fallback work surface for SEQ-20-P270.
func buildQ20sThinSupportTagFallbackPreparatory() map[string]any {
	return map[string]any{
		"version":         "q20s-p270.v1",
		"role":            "q20s_thin_support_tag_fallback_preparatory",
		"truth_authority": false,
		"sub_step":        "q20s",
		"contract_owner":  "query_class_contract_qr1a",
		"status":          "contract_only_closed",
		"policy_version":  "s20-tv.v1",
		"mode":            "q20s_thin_support_tag_fallback_preparatory_definition",
	}
}

// buildQ20sV1ThinSupportTagFallback exposes the q20s.v1
// thin_support_tag_fallback contract surface for SEQ-20-P271.
func buildQ20sV1ThinSupportTagFallback() map[string]any {
	return map[string]any{
		"version":         "q20s.v1",
		"role":            "thin_support_tag_fallback",
		"truth_authority": false,
		"sub_step":        "q20s",
		"contract_owner":  "query_class_contract_qr1a",
		"reuses": []string{
			"q20e_thin_tag_mode",
			"q20o_source_tags",
		},
		"low_density_support_visibility": true,
		"requires_prior_validity_join":   true,
		"drop_replacement":               "thin_support_tag",
		"policy_version":                 "s20-tv.v1",
		"mode":                           "q20s_v1_thin_support_tag_fallback_definition",
	}
}

// ===========================================================================
// SEQ-20 vx20a~vx20g validation replay gates (P286 ~ P299)
// ===========================================================================

// buildVx20aTemporalValidityReplayGate exposes the vx20a.v1
// temporal_validity_replay_gate surface for SEQ-20-P286/P287.
func buildVx20aTemporalValidityReplayGate() map[string]any {
	return map[string]any{
		"version":         "vx20a.v1",
		"role":            "temporal_validity_replay_gate",
		"truth_authority": false,
		"sub_step":        "vx20a",
		"contract_owner":  "query_class_contract_qr1a",
		"references": []string{
			"q20b_read_priority",
			"q20m_compare_note_surface",
			"q20r_join_mode",
		},
		"check_target":   "temporal_validity_contract_drift_only",
		"policy_version": "s20-tv.v1",
		"mode":           "vx20a_v1_temporal_validity_replay_gate_definition",
	}
}

// buildVx20bEntityBoostFalsePositiveGate exposes the vx20b.v1
// entity_boost_false_positive_gate surface for SEQ-20-P288/P289.
func buildVx20bEntityBoostFalsePositiveGate() map[string]any {
	return map[string]any{
		"version":         "vx20b.v1",
		"role":            "entity_boost_false_positive_gate",
		"truth_authority": false,
		"sub_step":        "vx20b",
		"contract_owner":  "query_class_contract_qr1a",
		"references": []string{
			"q20h_inspection_surface",
			"q20i_lagging_boost_surface",
		},
		"false_positive_ceiling": "lc1j_reuse",
		"policy_version":         "s20-eg.v1",
		"mode":                   "vx20b_v1_entity_boost_false_positive_gate_definition",
	}
}

// buildVx20cGraphAcceleratorDegradeGate exposes the vx20c.v1
// graph_accelerator_degrade_gate surface for SEQ-20-P290/P291.
func buildVx20cGraphAcceleratorDegradeGate() map[string]any {
	return map[string]any{
		"version":         "vx20c.v1",
		"role":            "graph_accelerator_degrade_gate",
		"truth_authority": false,
		"sub_step":        "vx20c",
		"contract_owner":  "query_class_contract_qr1a",
		"references": []string{
			"q20g_graph_like_support",
			"q20h_inspection_surface",
			"q20i_lagging_boost_surface",
		},
		"degrade_scenario":      "graph_accelerator_off",
		"fail_open_required":    true,
		"entity_boost_survives": true,
		"policy_version":        "s20-eg.v1",
		"mode":                  "vx20c_v1_graph_accelerator_degrade_gate_definition",
	}
}

// buildVx20dCanonicalPrecedenceReplayGate exposes the vx20d.v1
// canonical_precedence_replay_gate surface for SEQ-20-P292/P293.
func buildVx20dCanonicalPrecedenceReplayGate() map[string]any {
	return map[string]any{
		"version":         "vx20d.v1",
		"role":            "canonical_precedence_replay_gate",
		"truth_authority": false,
		"sub_step":        "vx20d",
		"contract_owner":  "query_class_contract_qr1a",
		"references": []string{
			"q20p_source_precedence",
			"q20r_join_authority_ordering",
		},
		"check_target":   "canon_support_precedence_drift_only",
		"policy_version": "s20-eg.v1",
		"mode":           "vx20d_v1_canonical_precedence_replay_gate_definition",
	}
}

// buildVx20ePromotionBlockedFreshnessReplayGate exposes the vx20e.v1
// promotion_blocked_freshness_replay_gate surface for SEQ-20-P294/P295.
func buildVx20ePromotionBlockedFreshnessReplayGate() map[string]any {
	return map[string]any{
		"version":         "vx20e.v1",
		"role":            "promotion_blocked_freshness_replay_gate",
		"truth_authority": false,
		"sub_step":        "vx20e",
		"contract_owner":  "query_class_contract_qr1a",
		"references": []string{
			"q20d_promotion_lag_support",
			"q20e_hot_recall_buffer",
			"q20p_canonical_pending_conflict",
			"q20i_lagging_boost",
		},
		"check_target":   "pending_current_visibility_preservation",
		"policy_version": "s20-tv.v1",
		"mode":           "vx20e_v1_promotion_blocked_freshness_replay_gate_definition",
	}
}

// buildVx20fRecallCueRescueReplayGate exposes the vx20f.v1
// recall_cue_rescue_replay_gate surface for SEQ-20-P296/P297.
func buildVx20fRecallCueRescueReplayGate() map[string]any {
	return map[string]any{
		"version":         "vx20f.v1",
		"role":            "recall_cue_rescue_replay_gate",
		"truth_authority": false,
		"sub_step":        "vx20f",
		"contract_owner":  "query_class_contract_qr1a",
		"references": []string{
			"q20q_rescue_surface",
		},
		"check_target":              "over_filter_miss_reduction",
		"stale_arc_auto_foreground": false,
		"policy_version":            "s20-tv.v1",
		"mode":                      "vx20f_v1_recall_cue_rescue_replay_gate_definition",
	}
}

// buildVx20gHotBufferWideGatherNonRegressionGate exposes the vx20g.v1
// hot_buffer_wide_gather_non_regression_gate surface for SEQ-20-P298/P299.
func buildVx20gHotBufferWideGatherNonRegressionGate() map[string]any {
	return map[string]any{
		"version":         "vx20g.v1",
		"role":            "hot_buffer_wide_gather_non_regression_gate",
		"truth_authority": false,
		"sub_step":        "vx20g",
		"contract_owner":  "query_class_contract_qr1a",
		"references": []string{
			"q20e_hot_recall_buffer",
			"q20r_wide_gather_validity_join",
			"q20s_thin_support_tag_fallback",
			"vx18c_upstream_gate",
			"vx18d_upstream_gate",
		},
		"check_target":    "truth_boundary_latency_ceiling",
		"step20_closeout": true,
		"policy_version":  "s20-tv.v1",
		"mode":            "vx20g_v1_hot_buffer_wide_gather_non_regression_gate_definition",
	}
}

// ===========================================================================
// SEQ-20 Beta 1.1 release smoke gate surfaces (P312 ~ P316)
// ===========================================================================

// buildSeq20P312Beta11BundleDryRun exposes the Beta 1.1 bundle dry-run
// evidence-only surface for SEQ-20-P312. Actual bundle creation is forbidden.
func buildSeq20P312Beta11BundleDryRun() map[string]any {
	return map[string]any{
		"version":         "s20-p312.v1",
		"role":            "seq20_beta11_bundle_dry_run",
		"truth_authority": false,
		"sub_step":        "release_gate",
		"note":            "Dry-run evidence only; actual bundle artifact creation is forbidden by policy.",
		"dry_run":         true,
		"policy_version":  "s20-tv.v1",
		"mode":            "seq20_beta11_bundle_dry_run_definition",
	}
}

// buildSeq20P313TemporalValidityRecallSmoke exposes the temporal validity
// recall smoke check surface for SEQ-20-P313.
func buildSeq20P313TemporalValidityRecallSmoke() map[string]any {
	return map[string]any{
		"version":         "s20-p313.v1",
		"role":            "seq20_temporal_validity_recall_smoke",
		"truth_authority": false,
		"sub_step":        "release_gate",
		"smoke_target":    "temporal_validity_recall",
		"references": []string{
			"q20a_temporal_query_expansion",
			"q20b_temporal_validity_read_policy",
			"q20m_temporal_ambiguity_support_note",
		},
		"policy_version": "s20-tv.v1",
		"mode":           "seq20_temporal_validity_recall_smoke_definition",
	}
}

// buildSeq20P314EntityGraphAcceleratorSmoke exposes the entity/graph
// accelerator smoke check surface for SEQ-20-P314.
func buildSeq20P314EntityGraphAcceleratorSmoke() map[string]any {
	return map[string]any{
		"version":         "s20-p314.v1",
		"role":            "seq20_entity_graph_accelerator_smoke",
		"truth_authority": false,
		"sub_step":        "release_gate",
		"smoke_target":    "entity_graph_accelerator",
		"references": []string{
			"q20f_lightweight_entity_index",
			"q20g_graph_like_support_signal",
			"q20h_entity_graph_boost_inspection",
			"q20i_lagging_current_state_boost",
		},
		"policy_version": "s20-eg.v1",
		"mode":           "seq20_entity_graph_accelerator_smoke_definition",
	}
}

// buildSeq20P315TemporalEntityDisambiguationSmoke exposes the temporal/entity
// disambiguation smoke check surface for SEQ-20-P315.
func buildSeq20P315TemporalEntityDisambiguationSmoke() map[string]any {
	return map[string]any{
		"version":         "s20-p315.v1",
		"role":            "seq20_temporal_entity_disambiguation_smoke",
		"truth_authority": false,
		"sub_step":        "release_gate",
		"smoke_target":    "temporal_entity_disambiguation",
		"references": []string{
			"q20n_alias_entity_conflict_disambiguation",
			"q20o_temporal_entity_source_tag_rule",
			"q20q_recall_cue_rescue_rule",
			"q20r_wide_gather_validity_join_rule",
		},
		"policy_version": "s20-tv.v1",
		"mode":           "seq20_temporal_entity_disambiguation_smoke_definition",
	}
}

// buildSeq20P316PrecedenceAmbiguityReviewChecklist exposes the precedence/
// ambiguity review checklist pass surface for SEQ-20-P316.
func buildSeq20P316PrecedenceAmbiguityReviewChecklist() map[string]any {
	return map[string]any{
		"version":         "s20-p316.v1",
		"role":            "seq20_precedence_ambiguity_review_checklist",
		"truth_authority": false,
		"sub_step":        "release_gate",
		"checklist_items": []string{
			"canonical_precedence_preserved",
			"support_lane_read_only",
			"ambiguity_reduction_active",
			"truth_boundary_intact",
		},
		"policy_version": "s20-eg.v1",
		"mode":           "seq20_precedence_ambiguity_review_checklist_definition",
	}
}

// ===========================================================================
// SEQ-20 final preserve summary surfaces (P330 ~ P333)
// ===========================================================================

// buildSeq20P330TemporalQueryExpansionPreserve exposes the temporal query
// expansion rule-first + metadata support confirmation preserve surface
// for SEQ-20-P330.
func buildSeq20P330TemporalQueryExpansionPreserve() map[string]any {
	return map[string]any{
		"version":         "s20-p330.v1",
		"role":            "seq20_temporal_query_expansion_preserve",
		"truth_authority": false,
		"sub_step":        "preserve_summary",
		"preserved": []string{
			"rule_first_temporal_expansion",
			"metadata_support_confirmation",
		},
		"policy_version": "s20-tv.v1",
		"mode":           "seq20_temporal_query_expansion_preserve_definition",
	}
}

// buildSeq20P331EntityIndexPreserve exposes the entity index lightweight
// entity/event axis + bounded relation edge summary granularity preserve
// surface for SEQ-20-P331.
func buildSeq20P331EntityIndexPreserve() map[string]any {
	return map[string]any{
		"version":         "s20-p331.v1",
		"role":            "seq20_entity_index_preserve",
		"truth_authority": false,
		"sub_step":        "preserve_summary",
		"preserved": []string{
			"lightweight_entity_event_axis",
			"bounded_relation_edge_summary_granularity",
		},
		"policy_version": "s20-eg.v1",
		"mode":           "seq20_entity_index_preserve_definition",
	}
}

// buildSeq20P332GraphAcceleratorPreserve exposes the graph accelerator
// structured edge signal degraded/off optional off + entity-side fail-open
// path preserve surface for SEQ-20-P332.
func buildSeq20P332GraphAcceleratorPreserve() map[string]any {
	return map[string]any{
		"version":         "s20-p332.v1",
		"role":            "seq20_graph_accelerator_preserve",
		"truth_authority": false,
		"sub_step":        "preserve_summary",
		"preserved": []string{
			"structured_edge_signal_degraded_optional_off",
			"entity_side_fail_open_path",
		},
		"policy_version": "s20-eg.v1",
		"mode":           "seq20_graph_accelerator_preserve_definition",
	}
}

// buildSeq20P333AmbiguitySupportNotePreserve exposes the ambiguity support
// note primary/compare/source-tag/join context bounded semi-structured note
// preserve surface for SEQ-20-P333.
func buildSeq20P333AmbiguitySupportNotePreserve() map[string]any {
	return map[string]any{
		"version":         "s20-p333.v1",
		"role":            "seq20_ambiguity_support_note_preserve",
		"truth_authority": false,
		"sub_step":        "preserve_summary",
		"preserved": []string{
			"primary_compare_source_tag_join_context",
			"bounded_semi_structured_note",
		},
		"policy_version": "s20-eg.v1",
		"mode":           "seq20_ambiguity_support_note_preserve_definition",
	}
}
