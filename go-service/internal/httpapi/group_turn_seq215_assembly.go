package httpapi

func buildSeq215P663Phase1ValidationPassed() map[string]any {
	return map[string]any{
		"version":           "s215-p663.v1",
		"role":              "seq215_phase1_validation_passed",
		"truth_authority":   false,
		"sub_step":          "21.5-m3a-helper-slice",
		"phase":             "1",
		"validation_status": "passed",
		"note":              "Phase 1 validation passed before starting M-3a helper slice.",
		"policy_version":    "s215-sc.v1",
		"mode":              "seq215_phase1_validation_passed_definition",
	}
}

// buildSeq215P664PrepareTurnAssemblyCreated exposes the
// backend/prepare_turn_assembly.py creation evidence for SEQ-21.5-P664.
func buildSeq215P664PrepareTurnAssemblyCreated() map[string]any {
	return map[string]any{
		"version":         "s215-p664.v1",
		"role":            "seq215_prepare_turn_assembly_created",
		"truth_authority": false,
		"sub_step":        "21.5-m3a-helper-slice",
		"file_created":    "backend/prepare_turn_assembly.py",
		"authority_path":  "Archive Center Beta 0.8(fix)",
		"note":            "backend/prepare_turn_assembly.py created in active Beta 0.8 backend to hold M-3a helper functions.",
		"policy_version":  "s215-sc.v1",
		"mode":            "seq215_prepare_turn_assembly_created_definition",
	}
}

// buildSeq215P665FormatMemoryTextMoved exposes the _format_memory_text_m3a
// move evidence surface for SEQ-21.5-P665.
func buildSeq215P665FormatMemoryTextMoved() map[string]any {
	return map[string]any{
		"version":         "s215-p665.v1",
		"role":            "seq215_format_memory_text_moved",
		"truth_authority": false,
		"sub_step":        "21.5-m3a-helper-slice",
		"function_name":   "_format_memory_text_m3a",
		"source_file":     "main.py",
		"target_file":     "backend/prepare_turn_assembly.py",
		"acyclic_check":   true,
		"note":            "_format_memory_text_m3a moved from main.py to prepare_turn_assembly.py with acyclic import check.",
		"policy_version":  "s215-sc.v1",
		"mode":            "seq215_format_memory_text_moved_definition",
	}
}

// buildSeq215P666FormatKGTextMoved exposes the _format_kg_text_m3a
// move evidence surface for SEQ-21.5-P666.
func buildSeq215P666FormatKGTextMoved() map[string]any {
	return map[string]any{
		"version":         "s215-p666.v1",
		"role":            "seq215_format_kg_text_moved",
		"truth_authority": false,
		"sub_step":        "21.5-m3a-helper-slice",
		"function_name":   "_format_kg_text_m3a",
		"source_file":     "main.py",
		"target_file":     "backend/prepare_turn_assembly.py",
		"acyclic_check":   true,
		"note":            "_format_kg_text_m3a moved from main.py to prepare_turn_assembly.py with acyclic import check.",
		"policy_version":  "s215-sc.v1",
		"mode":            "seq215_format_kg_text_moved_definition",
	}
}

// buildSeq215P667FormatEpisodeTextMoved exposes the _format_episode_text_m3a
// move evidence surface for SEQ-21.5-P667.
func buildSeq215P667FormatEpisodeTextMoved() map[string]any {
	return map[string]any{
		"version":         "s215-p667.v1",
		"role":            "seq215_format_episode_text_moved",
		"truth_authority": false,
		"sub_step":        "21.5-m3a-helper-slice",
		"function_name":   "_format_episode_text_m3a",
		"source_file":     "main.py",
		"target_file":     "backend/prepare_turn_assembly.py",
		"acyclic_check":   true,
		"note":            "_format_episode_text_m3a moved from main.py to prepare_turn_assembly.py with acyclic import check.",
		"policy_version":  "s215-sc.v1",
		"mode":            "seq215_format_episode_text_moved_definition",
	}
}

// buildSeq215P668FormatChapterTextMoved exposes the _format_chapter_text_m3a
// move evidence surface for SEQ-21.5-P668.
func buildSeq215P668FormatChapterTextMoved() map[string]any {
	return map[string]any{
		"version":         "s215-p668.v1",
		"role":            "seq215_format_chapter_text_moved",
		"truth_authority": false,
		"sub_step":        "21.5-m3a-helper-slice",
		"function_name":   "_format_chapter_text_m3a",
		"source_file":     "main.py",
		"target_file":     "backend/prepare_turn_assembly.py",
		"acyclic_check":   true,
		"note":            "_format_chapter_text_m3a moved from main.py to prepare_turn_assembly.py with acyclic import check.",
		"policy_version":  "s215-sc.v1",
		"mode":            "seq215_format_chapter_text_moved_definition",
	}
}

// buildSeq215P669FormatFallbackTextMoved exposes the _format_fallback_text_m3a
// move evidence surface for SEQ-21.5-P669.
func buildSeq215P669FormatFallbackTextMoved() map[string]any {
	return map[string]any{
		"version":         "s215-p669.v1",
		"role":            "seq215_format_fallback_text_moved",
		"truth_authority": false,
		"sub_step":        "21.5-m3a-helper-slice",
		"function_name":   "_format_fallback_text_m3a",
		"source_file":     "main.py",
		"target_file":     "backend/prepare_turn_assembly.py",
		"acyclic_check":   true,
		"note":            "_format_fallback_text_m3a moved from main.py to prepare_turn_assembly.py with acyclic import check.",
		"policy_version":  "s215-sc.v1",
		"mode":            "seq215_format_fallback_text_moved_definition",
	}
}

// buildSeq215P670CleanShortMoved exposes the _clean_short_m3a
// move evidence surface for SEQ-21.5-P670.
func buildSeq215P670CleanShortMoved() map[string]any {
	return map[string]any{
		"version":         "s215-p670.v1",
		"role":            "seq215_clean_short_moved",
		"truth_authority": false,
		"sub_step":        "21.5-m3a-helper-slice",
		"function_name":   "_clean_short_m3a",
		"source_file":     "main.py",
		"target_file":     "backend/prepare_turn_assembly.py",
		"acyclic_check":   true,
		"note":            "_clean_short_m3a moved from main.py to prepare_turn_assembly.py with acyclic import check.",
		"policy_version":  "s215-sc.v1",
		"mode":            "seq215_clean_short_moved_definition",
	}
}

// buildSeq215P671JsonLoadMaybeMoved exposes the _json_load_maybe_m3a
// move evidence surface for SEQ-21.5-P671.
func buildSeq215P671JsonLoadMaybeMoved() map[string]any {
	return map[string]any{
		"version":         "s215-p671.v1",
		"role":            "seq215_json_load_maybe_moved",
		"truth_authority": false,
		"sub_step":        "21.5-m3a-helper-slice",
		"function_name":   "_json_load_maybe_m3a",
		"source_file":     "main.py",
		"target_file":     "backend/prepare_turn_assembly.py",
		"acyclic_check":   true,
		"note":            "_json_load_maybe_m3a moved from main.py to prepare_turn_assembly.py with acyclic import check.",
		"policy_version":  "s215-sc.v1",
		"mode":            "seq215_json_load_maybe_moved_definition",
	}
}

// buildSeq215P672PredicateMatchesMoved exposes the _predicate_matches_m3a
// move evidence surface for SEQ-21.5-P672.
func buildSeq215P672PredicateMatchesMoved() map[string]any {
	return map[string]any{
		"version":         "s215-p672.v1",
		"role":            "seq215_predicate_matches_moved",
		"truth_authority": false,
		"sub_step":        "21.5-m3a-helper-slice",
		"function_name":   "_predicate_matches_m3a",
		"source_file":     "main.py",
		"target_file":     "backend/prepare_turn_assembly.py",
		"acyclic_check":   true,
		"note":            "_predicate_matches_m3a moved from main.py to prepare_turn_assembly.py with acyclic import check.",
		"policy_version":  "s215-sc.v1",
		"mode":            "seq215_predicate_matches_moved_definition",
	}
}

// buildSeq215P673WorldRuleNoteMoved exposes the _world_rule_note_m3a
// move evidence surface for SEQ-21.5-P673.
func buildSeq215P673WorldRuleNoteMoved() map[string]any {
	return map[string]any{
		"version":         "s215-p673.v1",
		"role":            "seq215_world_rule_note_moved",
		"truth_authority": false,
		"sub_step":        "21.5-m3a-helper-slice",
		"function_name":   "_world_rule_note_m3a",
		"source_file":     "main.py",
		"target_file":     "backend/prepare_turn_assembly.py",
		"acyclic_check":   true,
		"note":            "_world_rule_note_m3a moved from main.py to prepare_turn_assembly.py with acyclic import check.",
		"policy_version":  "s215-sc.v1",
		"mode":            "seq215_world_rule_note_moved_definition",
	}
}

// buildSeq215P674FormatEntityDigestTextMoved exposes the
// _format_entity_digest_text_m3a move evidence surface for SEQ-21.5-P674.
func buildSeq215P674FormatEntityDigestTextMoved() map[string]any {
	return map[string]any{
		"version":         "s215-p674.v1",
		"role":            "seq215_format_entity_digest_text_moved",
		"truth_authority": false,
		"sub_step":        "21.5-m3a-helper-slice",
		"function_name":   "_format_entity_digest_text_m3a",
		"source_file":     "main.py",
		"target_file":     "backend/prepare_turn_assembly.py",
		"acyclic_check":   true,
		"note":            "_format_entity_digest_text_m3a moved from main.py to prepare_turn_assembly.py with acyclic import check.",
		"policy_version":  "s215-sc.v1",
		"mode":            "seq215_format_entity_digest_text_moved_definition",
	}
}

// buildSeq215P675FormatEntityAnchorTextMoved exposes the
// _format_entity_anchor_text_m3a move evidence surface for SEQ-21.5-P675.
func buildSeq215P675FormatEntityAnchorTextMoved() map[string]any {
	return map[string]any{
		"version":         "s215-p675.v1",
		"role":            "seq215_format_entity_anchor_text_moved",
		"truth_authority": false,
		"sub_step":        "21.5-m3a-helper-slice",
		"function_name":   "_format_entity_anchor_text_m3a",
		"source_file":     "main.py",
		"target_file":     "backend/prepare_turn_assembly.py",
		"acyclic_check":   true,
		"note":            "_format_entity_anchor_text_m3a moved from main.py to prepare_turn_assembly.py with acyclic import check.",
		"policy_version":  "s215-sc.v1",
		"mode":            "seq215_format_entity_anchor_text_moved_definition",
	}
}

// ===========================================================================
// SEQ-21.5 M-3a validation and guard evidence (P677 ~ P682)
// ===========================================================================

// buildSeq215P677DBSessionHelpersStay exposes the DB/session access helpers
// stay in main.py evidence surface for SEQ-21.5-P677.
func buildSeq215P677DBSessionHelpersStay() map[string]any {
	return map[string]any{
		"version":         "s215-p677.v1",
		"role":            "seq215_db_session_helpers_stay",
		"truth_authority": false,
		"sub_step":        "21.5-m3a-validation",
		"stayed_in":       "main.py",
		"helpers":         "DB/session access helpers",
		"note":            "DB/session access helpers stay in main.py; not moved to prepare_turn_assembly.py.",
		"policy_version":  "s215-sc.v1",
		"mode":            "seq215_db_session_helpers_stay_definition",
	}
}

// buildSeq215P678CoreLogicStay exposes the direct-evidence, temporal-integrity,
// canon-ledger, budget, replay-gate, and route coordination logic stay in
// main.py evidence surface for SEQ-21.5-P678.
func buildSeq215P678CoreLogicStay() map[string]any {
	return map[string]any{
		"version":         "s215-p678.v1",
		"role":            "seq215_core_logic_stay",
		"truth_authority": false,
		"sub_step":        "21.5-m3a-validation",
		"stayed_in":       "main.py",
		"logic_types":     []string{"direct-evidence", "temporal-integrity", "canon-ledger", "budget", "replay-gate", "route coordination"},
		"note":            "Direct-evidence, temporal-integrity, canon-ledger, budget, replay-gate, and route coordination logic stay in main.py.",
		"policy_version":  "s215-sc.v1",
		"mode":            "seq215_core_logic_stay_definition",
	}
}

// buildSeq215P679InjectionPackFieldsUnchanged exposes the injection_pack
// field names unchanged evidence surface for SEQ-21.5-P679.
func buildSeq215P679InjectionPackFieldsUnchanged() map[string]any {
	return map[string]any{
		"version":          "s215-p679.v1",
		"role":             "seq215_injection_pack_fields_unchanged",
		"truth_authority":  false,
		"sub_step":         "21.5-m3a-validation",
		"fields_unchanged": true,
		"pack_name":        "injection_pack",
		"field_names": []string{
			"memory_text",
			"kg_text",
			"fallback_text",
			"episode_text",
		},
		"backend_pack_status":  "advisory_not_authoritative",
		"js_fallback_contract": "local_formatters_remain_for_offline_fail_open",
		"note":                 "injection_pack field names are unchanged after M-3a helper slice.",
		"policy_version":       "s215-sc.v1",
		"mode":                 "seq215_injection_pack_fields_unchanged_definition",
	}
}

// buildSeq215P680PyCompilePrepareTurnAssembly exposes the py_compile pass
// for main.py + prepare_turn_assembly.py evidence surface for SEQ-21.5-P680.
func buildSeq215P680PyCompilePrepareTurnAssembly() map[string]any {
	return map[string]any{
		"version":         "s215-p680.v1",
		"role":            "seq215_py_compile_prepare_turn_assembly",
		"truth_authority": false,
		"sub_step":        "21.5-m3a-validation",
		"compile_command": "python -m py_compile backend/main.py backend/prepare_turn_assembly.py",
		"compile_status":  "pass",
		"compile_date":    "2026-05-19",
		"note":            "py_compile passed for main.py and prepare_turn_assembly.py after M-3a helper slice.",
		"policy_version":  "s215-sc.v1",
		"mode":            "seq215_py_compile_prepare_turn_assembly_definition",
	}
}

// buildSeq215P681FocusedBackendTestsM3a exposes the focused backend tests
// adjacent to injection/truth behavior evidence surface for SEQ-21.5-P681.
func buildSeq215P681FocusedBackendTestsM3a() map[string]any {
	return map[string]any{
		"version":                        "s215-p681.v1",
		"role":                           "seq215_focused_backend_tests_m3a",
		"truth_authority":                false,
		"sub_step":                       "21.5-m3a-validation",
		"focused_test_file":              "backend/test_step21_5_prepare_turn_assembly.py",
		"tests_run":                      5,
		"tests_passed":                   5,
		"scoped_regression_tests_passed": 25,
		"backend_tests_passed":           212,
		"node_check":                     "pass",
		"test_coverage": []string{
			"recall_block_formatting",
			"exact_kg_delimiter_preservation",
			"entity_digest_anchor_formatting",
			"empty_and_malformed_inputs",
			"main_py_import_bridge",
			"bundle_injection_assembly_consumes_extracted_formatters",
		},
		"test_focus":     "injection/truth behavior for moved helpers",
		"note":           "Focused backend tests adjacent to injection/truth behavior passed for moved M-3a helpers.",
		"policy_version": "s215-sc.v1",
		"mode":           "seq215_focused_backend_tests_m3a_definition",
	}
}

// buildSeq215P682M3aValidationRecord exposes the validation output and
// changed files record evidence surface for SEQ-21.5-P682.
func buildSeq215P682M3aValidationRecord() map[string]any {
	return map[string]any{
		"version":         "s215-p682.v1",
		"role":            "seq215_m3a_validation_record",
		"truth_authority": false,
		"sub_step":        "21.5-m3a-validation",
		"record_type":     "validation_output_and_changed_files",
		"record_location": "progress_file",
		"note":            "Validation output and changed files recorded in progress file after M-3a helper slice.",
		"policy_version":  "s215-sc.v1",
		"mode":            "seq215_m3a_validation_record_definition",
	}
}

// ===========================================================================
// SEQ-21.5 Proxy/Config owner split evidence (P730 ~ P740)
// ===========================================================================

// buildSeq215P730ProxyPluginMainModelSeparated exposes the evidence that
// /proxy/plugin-main request model and provider/upstream auth helpers are
// separated from the main.py monolith for SEQ-21.5-P730.
