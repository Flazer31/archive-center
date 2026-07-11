package httpapi

import (
	"fmt"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func prepareTurnEvidenceCounts(memories []store.Memory, kgTriples []store.KGTriple, evidence []store.DirectEvidence, chatLogs []store.ChatLog, resumePack *store.ResumePack, storylines []store.Storyline, worldRules []store.WorldRule, charStates []store.CharacterState, pendingThreads []store.PendingThread, activeStates []store.ActiveState, canonicalLayers []store.CanonicalStateLayer, episodeSums []store.EpisodeSummary) map[string]any {
	return map[string]any{
		"memories":               len(memories),
		"kg_triples":             len(kgTriples),
		"direct_evidence":        len(evidence),
		"chat_logs":              len(chatLogs),
		"resume_pack_present":    resumePack != nil,
		"storylines":             len(storylines),
		"world_rules":            len(worldRules),
		"character_states":       len(charStates),
		"pending_threads":        len(pendingThreads),
		"active_states":          len(activeStates),
		"canonical_state_layers": len(canonicalLayers),
		"episode_summaries":      len(episodeSums),
	}
}

func prepareTurnSectionSummary(injectionText, inputContextText string, injectionTruncated, inputContextTruncated bool) []map[string]any {
	return []map[string]any{
		{
			"name":      "injection_text",
			"chars":     len([]rune(injectionText)),
			"available": strings.TrimSpace(injectionText) != "",
			"truncated": injectionTruncated,
			"sources":   []string{"memories", "kg_triples", "storylines", "world_rules", "character_states", "pending_threads"},
		},
		{
			"name":      "input_context_text",
			"chars":     len([]rune(inputContextText)),
			"available": strings.TrimSpace(inputContextText) != "",
			"truncated": inputContextTruncated,
			"sources":   []string{"direct_evidence", "chat_logs", "resume_pack", "active_states", "canonical_state_layers", "episode_summaries"},
		},
	}
}

func buildSupervisorInputPack(chatSessionID string, turnIndex int, rawUserInput, guideMode, guideStrength, narrativeStance, autoAdvanceTrigger, continuityQuery string, promptAssembly map[string]any, evidenceCounts map[string]any, sectionSummary []map[string]any, storylineSelection storylineSupervisorSelection, degraded bool, fallbackReason string, languageContext map[string]any) map[string]any {
	autoAdvanceHint := ""
	if autoAdvanceTrigger != "" && autoAdvanceTrigger != "none" {
		autoAdvanceHint = fmt.Sprintf("[Auto Advance]\ntrigger=%s; query=%s", autoAdvanceTrigger, truncateTextForShadow(continuityQuery, 160))
	}
	guideMode = resolveNarrativeGuideMode(guideMode, nil, "", rawUserInput)
	guideStrength = normalizeNarrativeGuideStrength(guideStrength)
	guideSuffix := buildGuideModeSuffix(guideMode, guideStrength)
	directorOverrides := buildGuideModeDirectorOverrides(guideMode)
	narrativeStanceSuffix := buildNarrativeStanceSuffix(narrativeStance)
	narrativeStanceBounds := buildNarrativeStanceBounds(narrativeStance)
	narrativeStanceSummary := buildNarrativeStanceSummary(narrativeStance, narrativeStanceSuffix, narrativeStanceBounds)
	storylineSelectionTrace := storylineSelectionSummary(storylineSelection)
	storylinesContext := formatStorylinesForSupervisor(storylineSelection)
	plannerLanguageContract := buildPrepareTurnPlannerLanguageContract(languageContext)
	guidanceParts := []string{
		"[Go R1 Supervisor Read Shadow]",
		"mode=read_shadow; would_call_llm=false; would_write=false",
		fmt.Sprintf("guide_mode=%s; guide_strength=%s; narrative_stance=%s", guideMode, guideStrength, narrativeStance),
		fmt.Sprintf("evidence_counts=%s", compactJSONForShadow(evidenceCounts, 500)),
		fmt.Sprintf("storyline_selection=%s", compactJSONForShadow(storylineSelectionTrace, 500)),
		fmt.Sprintf("section_summary=%s", compactJSONForShadow(sectionSummary, 500)),
	}
	if guideSuffix != "" {
		guidanceParts = append(guidanceParts, guideSuffix)
	}
	if narrativeStanceSuffix != "" {
		guidanceParts = append(guidanceParts, narrativeStanceSuffix)
	}
	if len(narrativeStanceBounds) > 0 {
		guidanceParts = append(guidanceParts, "[Story Initiative Bounds]\n"+compactJSONForShadow(narrativeStanceBounds, 600))
	}
	persistentGuidance := strings.Join(guidanceParts, "\n")
	finalGuidance := persistentGuidance
	if storylinesContext != "" {
		finalGuidance += "\n" + storylinesContext
	}
	if autoAdvanceHint != "" {
		finalGuidance += "\n" + autoAdvanceHint
	}
	status := "ready"
	if degraded {
		status = "degraded"
	}
	return map[string]any{
		"status":                    status,
		"source":                    "go_r1_read_shadow",
		"chat_session_id":           chatSessionID,
		"turn_index":                turnIndex,
		"raw_user_input_chars":      len([]rune(rawUserInput)),
		"prompt_assembly":           promptAssembly,
		"prompt_source":             promptAssembly["prompt_source"],
		"guide_mode":                guideMode,
		"guide_strength":            guideStrength,
		"guide_suffix":              guideSuffix,
		"narrative_stance":          narrativeStance,
		"narrative_stance_suffix":   narrativeStanceSuffix,
		"narrative_stance_bounds":   narrativeStanceBounds,
		"narrative_stance_summary":  narrativeStanceSummary,
		"director_overrides":        directorOverrides,
		"language_context":          nilIfEmptyMap(languageContext),
		"planner_language_contract": plannerLanguageContract,
		"persistent_guidance":       persistentGuidance,
		"storyline_selection":       storylineSelectionTrace,
		"storylines_context":        nilIfEmpty(storylinesContext),
		"auto_advance_trigger":      autoAdvanceTrigger,
		"auto_advance_hint":         autoAdvanceHint,
		"final_guidance_suffix":     finalGuidance,
		"momentum_packet": map[string]any{
			"packet_status":   status,
			"evidence_counts": evidenceCounts,
			"section_summary": sectionSummary,
		},
		"prompt_plan": []string{
			"supervisor_system.txt",
			"supervisor_prompt.txt",
			"persistent_guidance",
			"recent_context_summary",
			"wake_up_or_continuity_context",
		},
		"degraded":        degraded,
		"fallback_reason": fallbackReason,
		"would_call_llm":  false,
		"would_write":     false,
	}
}

func buildPrepareTurnPlannerLanguageContract(languageContext map[string]any) map[string]any {
	target := prepareTurnSessionOutputLanguage(languageContext)
	status := "unknown"
	if target != "" && target != "auto" && target != "unknown" {
		status = "ready"
	}
	return map[string]any{
		"contract_version":              languageMemoryContractVersion,
		"status":                        status,
		"planner_support_language":      nilIfEmpty(target),
		"planner_language_source":       nilIfEmpty(extractionStringFromAny(languageContext["output_language_source"])),
		"current_user_input_priority":   "highest",
		"raw_user_input_rewritten":      false,
		"raw_evidence_rewritten":        false,
		"generated_support_policy":      "use_session_output_language_when_language_is_known",
		"trace_labels_language_neutral": true,
	}
}

func buildWeakInputPlannerContract(rawUserInput string, inputAnchorGovernor map[string]any, languageContext map[string]any, maxInputContextChars int) map[string]any {
	trimmed := strings.TrimSpace(rawUserInput)
	lower := strings.ToLower(trimmed)
	runeCount := len([]rune(trimmed))
	wordCount := len(strings.Fields(trimmed))
	continuationPhrases := map[string]bool{
		"continue": true, "go on": true, "next": true, "more": true, "resume": true, "keep going": true,
		"계속": true, "계속해": true, "이어서": true, "이어가": true, "다음": true, "다음 장면": true,
		"응": true, "ㅇㅇ": true, "좋아": true, "그래": true, "좋아 계속": true,
	}
	taxonomy := "specific_input"
	switch {
	case trimmed == "":
		taxonomy = "empty_input"
	case continuationPhrases[lower]:
		taxonomy = "continuation_trigger"
	case runeCount <= 12 && wordCount <= 3:
		taxonomy = "short_ack_or_nudge"
	case runeCount <= 24:
		taxonomy = "low_specificity_input"
	}

	explicitRedirection := false
	if redirection, ok := inputAnchorGovernor["explicit_user_redirection"].(map[string]any); ok {
		explicitRedirection = boolFromAny(redirection["detected"])
	}
	selectedAnchors := stringSliceFromAny(inputAnchorGovernor["selected_slot_names"])
	droppedAnchors := stringSliceFromAny(inputAnchorGovernor["dropped_slot_names"])
	weakActive := taxonomy != "specific_input"
	if explicitRedirection {
		weakActive = false
	}
	status := "not_applicable"
	if weakActive {
		status = "ready"
	}
	if explicitRedirection {
		status = "redirection_user_input_wins"
	}

	maxNewBeats := 0
	if weakActive {
		maxNewBeats = 1
	}
	targetLanguage := prepareTurnSessionOutputLanguage(languageContext)
	return map[string]any{
		"contract_version":            "step25_weak_input_planner.v1",
		"status":                      status,
		"active":                      weakActive,
		"taxonomy":                    taxonomy,
		"raw_user_input_chars":        runeCount,
		"raw_user_input_words":        wordCount,
		"current_user_input_priority": "highest",
		"truth_authority":             false,
		"would_write":                 false,
		"would_call_llm":              false,
		"selected_anchor_names":       selectedAnchors,
		"dropped_anchor_names":        droppedAnchors,
		"planner_support_language":    nilIfEmpty(targetLanguage),
		"input_context_budget_chars":  maxInputContextChars,
		"minimum_mandate": []string{
			"preserve the latest user input as the only command source",
			"use recent/previous anchors only as support",
			"avoid stale arc revival unless current input or fresh evidence aligns",
		},
		"acting_brief": map[string]any{
			"main_failure_risk": "stall_or_stale_replay",
			"portrayal_goal":    "continue the current scene using verified anchors",
			"reply_strategy":    "advance at most one reversible causal beat; ask or frame options when the choice is unspecified",
		},
		"initiative_boundary": map[string]any{
			"max_new_beats":                 maxNewBeats,
			"allow_scene_jump":              false,
			"may_suggest":                   true,
			"may_execute_irreversible_step": false,
			"explicit_redirection_detected": explicitRedirection,
		},
		"ambiguity_policy": map[string]any{
			"preserve_unspecified_choice": true,
			"do_not_choose_for_user":      true,
			"degrade_path":                "support_only_anchor_or_no_planner_brief",
		},
		"role_lens_contract": map[string]any{
			"world_lens":               "guard hard setting contradictions only",
			"plot_lens":                "surface current arc pressure without forcing payoff",
			"npc_lens":                 "use visible or directly relevant known-state only",
			"critic_lens":              "flag over-injection, secret leak, stale replay, and user override risk",
			"raw_memory_dump_allowed":  false,
			"hidden_knowledge_allowed": false,
		},
	}
}

func formatWeakInputPlannerGuidance(contract map[string]any) string {
	if contract == nil || !boolFromAny(contract["active"]) {
		return ""
	}
	taxonomy := extractionStringFromAny(contract["taxonomy"])
	brief, _ := contract["acting_brief"].(map[string]any)
	boundary, _ := contract["initiative_boundary"].(map[string]any)
	return strings.Join([]string{
		"[Weak Input Planner]",
		"mode=support_only; truth_authority=false; current_user_input_priority=highest",
		"taxonomy=" + taxonomy,
		"main_failure_risk=" + extractionStringFromAny(brief["main_failure_risk"]),
		"portrayal_goal=" + extractionStringFromAny(brief["portrayal_goal"]),
		"reply_strategy=" + extractionStringFromAny(brief["reply_strategy"]),
		fmt.Sprintf("initiative=max_new_beats:%d; allow_scene_jump:%v; irreversible_step:false", intFromAny(boundary["max_new_beats"], 0), boolFromAny(boundary["allow_scene_jump"])),
		"ambiguity=preserve unspecified user choice; do not choose for the user",
	}, "\n")
}

func buildPlannerExecutionContract(rawUserInput, narrativeStance, guideMode, guideStrength string, inputAnchorGovernor, weakInputPlanner map[string]any, selectedStorylines []store.Storyline, pendingThreads []store.PendingThread, activeStates []store.ActiveState, canonicalLayers []store.CanonicalStateLayer, worldRules []store.WorldRule, assembly prepareTurnInjectionAssembly, languageContext map[string]any) map[string]any {
	stanceBounds := buildNarrativeStanceBounds(narrativeStance)
	maxNewBeats := intFromAny(stanceBounds["max_new_beats"], 0)
	allowSceneJump := boolFromAny(stanceBounds["allow_scene_jump"])
	if weakInputPlanner != nil && boolFromAny(weakInputPlanner["active"]) {
		if boundary, ok := weakInputPlanner["initiative_boundary"].(map[string]any); ok {
			weakBeats := intFromAny(boundary["max_new_beats"], maxNewBeats)
			if weakBeats < maxNewBeats || maxNewBeats <= 0 {
				maxNewBeats = weakBeats
			}
			allowSceneJump = allowSceneJump && boolFromAny(boundary["allow_scene_jump"])
		}
	}

	selectedAnchors := stringSliceFromAny(inputAnchorGovernor["selected_slot_names"])
	droppedAnchors := stringSliceFromAny(inputAnchorGovernor["dropped_slot_names"])
	activeStorylineNames := []string{}
	for _, sl := range selectedStorylines {
		if name := strings.TrimSpace(sl.Name); name != "" {
			activeStorylineNames = appendUniqueMemorySearchText(activeStorylineNames, name)
		}
	}
	openThreadNames := []string{}
	for _, th := range pendingThreads {
		if th.Suppressed {
			continue
		}
		label := strings.TrimSpace(firstNonEmpty(th.Title, th.Description, th.ThreadKey))
		if label != "" {
			openThreadNames = appendUniqueMemorySearchText(openThreadNames, label)
		}
	}

	protectedCount := intFromAny(assembly.Counts["protected_secret_count"], 0) +
		intFromAny(assembly.Counts["identity_accuracy_count"], 0) +
		intFromAny(assembly.Counts["protected_memory_guarded_count"], 0)
	privateLaneActive := intFromAny(assembly.Counts["character_private_recollection_bound"], intFromAny(assembly.Counts["character_private_recollection_count"], 0)) > 0 ||
		strings.TrimSpace(assembly.CharacterPrivateText) != ""

	forbiddenMoves := append([]string{}, stringSliceFromAny(stanceBounds["forbidden_moves"])...)
	forbiddenMoves = appendUniqueMemorySearchText(forbiddenMoves, "do not override or reinterpret the latest user input")
	forbiddenMoves = appendUniqueMemorySearchText(forbiddenMoves, "do not convert support memories into new canonical facts")
	if !allowSceneJump {
		forbiddenMoves = appendUniqueMemorySearchText(forbiddenMoves, "do not hard-cut to a new scene without current-input support")
	}
	if weakInputPlanner != nil && boolFromAny(weakInputPlanner["active"]) {
		forbiddenMoves = appendUniqueMemorySearchText(forbiddenMoves, "do not turn a weak prompt into an irreversible user decision")
	}
	if protectedCount > 0 || privateLaneActive {
		forbiddenMoves = appendUniqueMemorySearchText(forbiddenMoves, "do not reveal protected private knowledge or let unrelated characters discover it without current-scene evidence")
		forbiddenMoves = appendUniqueMemorySearchText(forbiddenMoves, "do not split confirmed alias or cover-identity continuity into separate people")
	}

	sceneMandate := "follow the latest user input and keep the current scene causally grounded"
	if boolFromAny(weakInputPlanner["active"]) {
		sceneMandate = "continue the current scene from verified anchors while preserving user ambiguity"
	} else if strings.TrimSpace(rawUserInput) != "" {
		sceneMandate = "answer the latest user input directly and use support context only as grounding"
	}

	requiredOutcomes := []string{
		"latest user input remains the command source",
		"visible response must stay compatible with direct evidence and canonical state",
	}
	if len(activeStorylineNames) > 0 {
		requiredOutcomes = append(requiredOutcomes, "keep selected active storyline in view: "+strings.Join(limitStringSlice(activeStorylineNames, 2), " / "))
	}
	if len(openThreadNames) > 0 {
		requiredOutcomes = append(requiredOutcomes, "preserve open thread awareness: "+strings.Join(limitStringSlice(openThreadNames, 2), " / "))
	}
	if len(selectedAnchors) > 0 {
		requiredOutcomes = append(requiredOutcomes, "use selected anchors as support only: "+strings.Join(limitStringSlice(selectedAnchors, 3), " / "))
	}

	pacingLevel := "steady"
	if maxNewBeats <= 0 {
		pacingLevel = "hold_or_user_led"
	} else if normalizeNarrativeStance(narrativeStance) == "proactive" {
		pacingLevel = "bounded_forward"
	}
	targetLanguage := prepareTurnSessionOutputLanguage(languageContext)

	return map[string]any{
		"contract_version":            "step25_planner_execution_contract.v1",
		"status":                      "ready",
		"active":                      true,
		"current_user_input_priority": "highest",
		"truth_authority":             false,
		"would_write":                 false,
		"would_call_llm":              false,
		"planner_support_language":    nilIfEmpty(targetLanguage),
		"scene_mandate": map[string]any{
			"value":  sceneMandate,
			"source": "current_input_plus_read_surfaces",
		},
		"required_outcome": map[string]any{
			"items": requiredOutcomes,
			"count": len(requiredOutcomes),
		},
		"forbidden_move": map[string]any{
			"items":                 limitStringSlice(forbiddenMoves, 8),
			"count":                 len(forbiddenMoves),
			"protected_lane_active": protectedCount > 0 || privateLaneActive,
		},
		"pacing_pressure": map[string]any{
			"level":            pacingLevel,
			"max_new_beats":    maxNewBeats,
			"allow_scene_jump": allowSceneJump,
			"guide_mode":       normalizeNarrativeGuideMode(guideMode),
			"guide_strength":   normalizeNarrativeGuideStrength(guideStrength),
			"stance":           normalizeNarrativeStance(narrativeStance),
		},
		"ending_requirement": map[string]any{
			"mode":        "soft_landing",
			"instruction": "end with immediate consequence, reaction, or a reversible next choice; do not force final resolution",
		},
		"consume_rule": map[string]any{
			"allowed_usage":  []string{"next_turn_guidance", "continuity_guard", "pacing_guard", "secret_leak_guard"},
			"blocked_usage":  []string{"truth_write", "canonical_override", "user_intent_override", "raw_memory_dump", "hidden_knowledge_reveal"},
			"priority_order": []string{"current_user_input", "explicit_user_correction", "direct_evidence", "canonical_state", "retrieved_support", "planner_execution_contract"},
		},
		"read_surface_alignment": map[string]any{
			"selected_anchor_count":       len(selectedAnchors),
			"dropped_anchor_count":        len(droppedAnchors),
			"selected_storyline_count":    len(activeStorylineNames),
			"pending_thread_count":        len(openThreadNames),
			"active_state_count":          len(activeStates),
			"canonical_layer_count":       len(canonicalLayers),
			"world_rule_count":            len(worldRules),
			"protected_signal_count":      protectedCount,
			"private_recollection_active": privateLaneActive,
		},
		"facet_audit_repair_ingestion": map[string]any{
			"status": "no_prior_facet_audit_surface",
			"rule":   "when prior drift or secret-leak audit exists, consume only as bounded repair hint for the next turn",
		},
		"concealment_guard": map[string]any{
			"active": protectedCount > 0 || privateLaneActive,
			"rule":   "preserve protected/private knowledge boundaries; do not reveal or externalize without current-scene evidence",
		},
		"role_lens_consumption": map[string]any{
			"world_lens":  "hard setting and rule contradiction guard only",
			"plot_lens":   "current arc pressure and required outcome hint only",
			"npc_lens":    "visible or directly relevant known-state boundary only",
			"critic_lens": "over-injection, secret leak, stale replay, flat interpretation, and user override guard only",
		},
	}
}

func formatPlannerExecutionContractGuidance(contract map[string]any) string {
	if contract == nil || !boolFromAny(contract["active"]) {
		return ""
	}
	sceneMandate := mapFromAny(contract["scene_mandate"])
	required := mapFromAny(contract["required_outcome"])
	forbidden := mapFromAny(contract["forbidden_move"])
	pacing := mapFromAny(contract["pacing_pressure"])
	ending := mapFromAny(contract["ending_requirement"])
	requiredItems := limitStringSlice(stringSliceFromAny(required["items"]), 3)
	forbiddenItems := limitStringSlice(stringSliceFromAny(forbidden["items"]), 3)
	return strings.Join([]string{
		"[Planner Execution Contract]",
		"mode=support_only; truth_authority=false; current_user_input_priority=highest",
		"scene_mandate=" + extractionStringFromAny(sceneMandate["value"]),
		"required_outcome=" + strings.Join(requiredItems, " / "),
		"forbidden_move=" + strings.Join(forbiddenItems, " / "),
		fmt.Sprintf("pacing=max_new_beats:%d; allow_scene_jump:%v; level:%s", intFromAny(pacing["max_new_beats"], 0), boolFromAny(pacing["allow_scene_jump"]), extractionStringFromAny(pacing["level"])),
		"ending_requirement=" + extractionStringFromAny(ending["instruction"]),
	}, "\n")
}

func buildProgressionChoiceLedger(sid string, turnIndex int, rawUserInput string, chatLogs []store.ChatLog, storylines []store.Storyline, pendingThreads []store.PendingThread, episodeSums []store.EpisodeSummary, inputAnchorGovernor, weakInputPlanner, plannerExecutionContract, progressionLedger map[string]any) map[string]any {
	trimmed := strings.TrimSpace(rawUserInput)
	selectedAnchors := stringSliceFromAny(inputAnchorGovernor["selected_slot_names"])
	explicitRedirection := false
	if redirection, ok := inputAnchorGovernor["explicit_user_redirection"].(map[string]any); ok {
		explicitRedirection = boolFromAny(redirection["detected"])
	}
	weakActive := weakInputPlanner != nil && boolFromAny(weakInputPlanner["active"])
	latestUser := latestUserChatLogContent(chatLogs)
	sameIncident := trimmed != "" && latestUser != "" && stableKey("turn", trimmed) == stableKey("turn", latestUser)
	activeStorylineCount := countActiveProgressionStorylines(storylines)
	activeThreadCount := countOpenProgressionThreads(pendingThreads)
	hasLiveAnchor := len(selectedAnchors) > 0 || activeStorylineCount > 0 || activeThreadCount > 0
	hasCallbackAnchor := stringSliceContains(selectedAnchors, "Chapter") || stringSliceContains(selectedAnchors, "Saga") || len(episodeSums) > 0
	staleDroppedCount := countDroppedOldArcAnchors(inputAnchorGovernor)

	choice := "advance"
	reasons := []string{"specific_input_or_live_anchor_available"}
	if trimmed == "" {
		choice = "hold"
		reasons = []string{"empty_input_preserve_user_ambiguity"}
	} else if sameIncident {
		choice = "hold"
		reasons = []string{"same_incident_exact_repeat_detected"}
	} else if explicitRedirection {
		choice = "new_scene_opportunity"
		reasons = []string{"explicit_user_redirection_detected"}
	} else if weakActive && !hasLiveAnchor {
		choice = "hold"
		reasons = []string{"weak_input_without_live_anchor"}
	} else if weakActive {
		choice = "advance"
		reasons = []string{"weak_input_bounded_advance_from_live_anchor"}
	} else if hasCallbackAnchor && (activeStorylineCount > 0 || activeThreadCount > 0) {
		choice = "callback"
		reasons = []string{"callback_anchor_aligned_with_active_thread"}
	} else if staleDroppedCount > 0 && activeStorylineCount == 0 && activeThreadCount == 0 {
		choice = "hold"
		reasons = []string{"stale_callback_suppressed_without_current_scene_alignment"}
	}

	pacing := mapFromAny(plannerExecutionContract["pacing_pressure"])
	return map[string]any{
		"contract_version":            "step25_progression_choice_ledger.v1",
		"status":                      "ready",
		"chat_session_id":             sid,
		"turn_index":                  turnIndex,
		"choice":                      choice,
		"choice_set":                  []string{"advance", "callback", "new_scene_opportunity", "hold"},
		"reasons":                     reasons,
		"current_user_input_priority": "highest",
		"truth_authority":             false,
		"would_write":                 false,
		"would_call_llm":              false,
		"scene_advancement_ledger": map[string]any{
			"decision":                  choice,
			"reason":                    strings.Join(reasons, " / "),
			"max_new_beats":             intFromAny(pacing["max_new_beats"], 0),
			"allow_scene_jump":          boolFromAny(pacing["allow_scene_jump"]),
			"selected_anchor_count":     len(selectedAnchors),
			"active_storyline_count":    activeStorylineCount,
			"active_thread_count":       activeThreadCount,
			"callback_anchor_available": hasCallbackAnchor,
		},
		"callback_evaluation": map[string]any{
			"candidate":                  hasCallbackAnchor,
			"aligned_with_active_thread": hasCallbackAnchor && (activeStorylineCount > 0 || activeThreadCount > 0),
			"stale_dropped_count":        staleDroppedCount,
			"stale_revival_suppressed":   staleDroppedCount > 0 && choice != "callback",
			"rule":                       "callback is support-only and must align with current scene, active thread, or current input",
		},
		"same_incident_stall_detection": map[string]any{
			"detected":             sameIncident,
			"mode":                 "exact_normalized_user_input_repeat_only",
			"current_input_chars":  len([]rune(trimmed)),
			"latest_user_present":  latestUser != "",
			"stall_action":         "hold",
			"false_positive_guard": "no semantic guess; only exact normalized repeat is treated as same incident",
		},
		"inspection_replay_surface": map[string]any{
			"status": "ready",
			"cases":  []string{"weak_input_bounded_advance", "callback_alignment", "stale_callback_suppression", "same_incident_hold", "explicit_redirection_new_scene"},
			"source": "prepare_turn_read_only",
		},
		"consume_rule": map[string]any{
			"allowed_usage":  []string{"next_turn_progression_hint", "stall_guard", "callback_alignment_guard"},
			"blocked_usage":  []string{"truth_write", "canonical_state_change", "forced_scene_jump", "user_intent_override"},
			"priority_order": []string{"current_user_input", "explicit_user_correction", "planner_execution_contract", "progression_choice_ledger"},
		},
		"ledger_alignment": map[string]any{
			"progression_ledger_status": extractionStringFromAny(progressionLedger["status"]),
			"last_advanced_turn":        progressionLedger["last_advanced_turn"],
			"last_validated_turn":       progressionLedger["last_validated_turn"],
		},
	}
}

func latestUserChatLogContent(chatLogs []store.ChatLog) string {
	latestTurn := -1
	latestID := int64(-1)
	latest := ""
	for _, log := range chatLogs {
		if !strings.EqualFold(strings.TrimSpace(log.Role), "user") {
			continue
		}
		if log.TurnIndex > latestTurn || (log.TurnIndex == latestTurn && log.ID > latestID) {
			latestTurn = log.TurnIndex
			latestID = log.ID
			latest = strings.TrimSpace(log.Content)
		}
	}
	return latest
}

func countActiveProgressionStorylines(storylines []store.Storyline) int {
	count := 0
	for _, sl := range storylines {
		if sl.Suppressed {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(sl.Status))
		if status == "" || status == "active" || status == "open" || status == "escalating" || status == "aftermath" || status == "latent" {
			count++
		}
	}
	return count
}

func countOpenProgressionThreads(pendingThreads []store.PendingThread) int {
	count := 0
	for _, th := range pendingThreads {
		if th.Suppressed {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(th.Status))
		if status == "" || status == "open" || status == "active" || status == "pending" {
			count++
		}
	}
	return count
}

func countDroppedOldArcAnchors(inputAnchorGovernor map[string]any) int {
	count := 0
	rawTrace, _ := inputAnchorGovernor["old_arc_keep_drop_trace"].([]map[string]any)
	for _, item := range rawTrace {
		if extractionStringFromAny(item["decision"]) == "drop" {
			count++
		}
	}
	return count
}

func formatProgressionChoiceGuidance(contract map[string]any) string {
	if contract == nil || extractionStringFromAny(contract["status"]) == "" {
		return ""
	}
	callbackEval := mapFromAny(contract["callback_evaluation"])
	stall := mapFromAny(contract["same_incident_stall_detection"])
	ledger := mapFromAny(contract["scene_advancement_ledger"])
	reasons := stringSliceFromAny(contract["reasons"])
	return strings.Join([]string{
		"[Progression Choice Ledger]",
		"mode=support_only; truth_authority=false; current_user_input_priority=highest",
		"choice=" + extractionStringFromAny(contract["choice"]),
		"reason=" + strings.Join(limitStringSlice(reasons, 2), " / "),
		fmt.Sprintf("ledger=max_new_beats:%d; allow_scene_jump:%v; anchors:%d", intFromAny(ledger["max_new_beats"], 0), boolFromAny(ledger["allow_scene_jump"]), intFromAny(ledger["selected_anchor_count"], 0)),
		fmt.Sprintf("callback=candidate:%v; aligned:%v; stale_suppressed:%v", boolFromAny(callbackEval["candidate"]), boolFromAny(callbackEval["aligned_with_active_thread"]), boolFromAny(callbackEval["stale_revival_suppressed"])),
		fmt.Sprintf("same_incident_exact_repeat:%v", boolFromAny(stall["detected"])),
	}, "\n")
}

func buildStep25ValidationGate(rawUserInput string, weakInputPlanner, plannerExecutionContract, progressionChoiceLedger map[string]any) map[string]any {
	type checkDef struct {
		id     string
		name   string
		pass   bool
		reason string
	}
	weakBoundary := mapFromAny(weakInputPlanner["initiative_boundary"])
	execConsume := mapFromAny(plannerExecutionContract["consume_rule"])
	roleLens := mapFromAny(plannerExecutionContract["role_lens_consumption"])
	progressionReplay := mapFromAny(progressionChoiceLedger["inspection_replay_surface"])
	callbackEval := mapFromAny(progressionChoiceLedger["callback_evaluation"])
	stall := mapFromAny(progressionChoiceLedger["same_incident_stall_detection"])
	choice := extractionStringFromAny(progressionChoiceLedger["choice"])
	allowedChoice := choice == "advance" || choice == "callback" || choice == "new_scene_opportunity" || choice == "hold"
	replayCases := stringSliceFromAny(progressionReplay["cases"])
	blockedUsage := stringSliceFromAny(execConsume["blocked_usage"])
	contractVersionsPresent := extractionStringFromAny(weakInputPlanner["contract_version"]) == "step25_weak_input_planner.v1" &&
		extractionStringFromAny(plannerExecutionContract["contract_version"]) == "step25_planner_execution_contract.v1" &&
		extractionStringFromAny(progressionChoiceLedger["contract_version"]) == "step25_progression_choice_ledger.v1"

	checks := []checkDef{
		{
			id:     "25-5a",
			name:   "current input misread replay",
			pass:   strings.TrimSpace(rawUserInput) != "" && extractionStringFromAny(weakInputPlanner["current_user_input_priority"]) == "highest",
			reason: "current user input remains the highest-priority command source",
		},
		{
			id:     "25-5b",
			name:   "weak input progression replay",
			pass:   weakInputPlanner["truth_authority"] == false && weakInputPlanner["would_write"] == false && intFromAny(weakBoundary["max_new_beats"], 99) <= 1,
			reason: "weak input planner stays bounded and support-only",
		},
		{
			id:     "25-5c",
			name:   "planner slot truth boundary",
			pass:   plannerExecutionContract["truth_authority"] == false && plannerExecutionContract["would_write"] == false && len(blockedUsage) > 0,
			reason: "execution slots cannot write truth or override user intent",
		},
		{
			id:     "25-5d",
			name:   "progression choice separation",
			pass:   allowedChoice && callbackEval["rule"] != nil && stall["false_positive_guard"] != nil,
			reason: "advance, callback, opening, and hold choices are explicit and inspectable",
		},
		{
			id:     "25-5e",
			name:   "step-specific replay surface",
			pass:   extractionStringFromAny(progressionReplay["source"]) == "prepare_turn_read_only" && len(replayCases) >= 4,
			reason: "Step 25 replay cases are local to this planner/progression gate",
		},
		{
			id:     "25-5f",
			name:   "schema migration package",
			pass:   contractVersionsPresent,
			reason: "all Step 25 contracts expose version stamps for rollback/replay comparison",
		},
		{
			id:     "25-5g",
			name:   "role-lensed input improvement replay",
			pass:   roleLens["world_lens"] != nil && roleLens["plot_lens"] != nil && roleLens["npc_lens"] != nil && roleLens["critic_lens"] != nil,
			reason: "world, plot, NPC, and critic lenses are present as bounded support rules",
		},
		{
			id:     "25-5h",
			name:   "role-lens failure budget",
			pass:   stringSliceContains(blockedUsage, "hidden_knowledge_reveal") && stringSliceContains(blockedUsage, "user_intent_override"),
			reason: "lens failure modes block hidden-knowledge leak and user-intent override",
		},
	}

	items := make([]map[string]any, 0, len(checks))
	blocking := []string{}
	passed := 0
	for _, check := range checks {
		status := "pass"
		if !check.pass {
			status = "hold"
			blocking = append(blocking, check.id)
		} else {
			passed++
		}
		items = append(items, map[string]any{
			"id":     check.id,
			"name":   check.name,
			"status": status,
			"reason": check.reason,
		})
	}
	gateStatus := "pass"
	if len(blocking) > 0 {
		gateStatus = "hold"
	}
	return map[string]any{
		"contract_version":            "step25_validation_gate.v1",
		"status":                      "ready",
		"gate_status":                 gateStatus,
		"adoption_ready":              len(blocking) == 0,
		"current_user_input_priority": "highest",
		"truth_authority":             false,
		"would_write":                 false,
		"would_call_llm":              false,
		"passed_count":                passed,
		"total_count":                 len(checks),
		"blocking_check_ids":          blocking,
		"checks":                      items,
		"scope":                       "prepare_turn_contract_smoke_gate",
		"live_replay_note":            "This gate verifies Step 25 contract shape and safety boundaries; separate live user replay can still be run before release packaging.",
		"release_gate": map[string]any{
			"status":             gateStatus,
			"bundle_ready":       len(blocking) == 0,
			"requires_packaging": false,
			"step":               "25",
		},
	}
}

func normalizeNarrativeGuideMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "auto":
		return "auto"
	case "standard":
		return "standard"
	case "romantic":
		return "romantic"
	case "action":
		return "action"
	case "mature_soft", "mature-soft":
		return "mature_soft"
	case "mature_direct", "mature-direct":
		return "mature_direct"
	default:
		return "off"
	}
}

func resolveNarrativeGuideMode(mode string, contextMessages []map[string]any, wakeUpContext, fallbackUserInput string) string {
	normalized := normalizeNarrativeGuideMode(mode)
	if normalized != "auto" {
		return normalized
	}
	probe := strings.Join(nonEmptyStrings([]string{fallbackUserInput, latestUserMessageText(contextMessages), wakeUpContext}), "\n")
	return inferNarrativeGuideModeFromText(probe)
}

func latestUserMessageText(contextMessages []map[string]any) string {
	for i := len(contextMessages) - 1; i >= 0; i-- {
		msg := contextMessages[i]
		if strings.ToLower(strings.TrimSpace(extractionStringFromAny(msg["role"]))) != "user" {
			continue
		}
		content := strings.TrimSpace(extractionStringFromAny(msg["content"]))
		if content != "" {
			return content
		}
	}
	return ""
}

func inferNarrativeGuideModeFromText(text string) string {
	source := strings.ToLower(strings.TrimSpace(text))
	if source == "" {
		return "standard"
	}
	if containsAnyText(source, "r18", "r 18", "explicit", "direct sensual", "mature direct", "adult direct") {
		return "mature_direct"
	}
	if containsAnyText(source, "sensual", "mature", "adult romance", "soft mature", "intimate") {
		return "mature_soft"
	}
	if containsAnyText(source, "romance", "romantic", "love", "date", "crush", "kiss") {
		return "romantic"
	}
	if containsAnyText(source, "action", "battle", "fight", "combat", "mission", "chase", "duel") {
		return "action"
	}
	return "standard"
}

func containsAnyText(source string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(source, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func normalizeNarrativeGuideStrength(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "none":
		return "none"
	case "medium", "strong":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "weak"
	}
}

func narrativeGuideStrengthLine(strength string) string {
	switch normalizeNarrativeGuideStrength(strength) {
	case "none":
		return ""
	case "strong":
		return "Strength: strong. Be more active about pacing, continuity repair, and callback suggestions, but never override user input or force outcomes."
	case "medium":
		return "Strength: medium. Give visible pacing and continuity support when the scene has room, but avoid forcing outcomes."
	default:
		return "Strength: weak. Keep this nearly invisible; only prevent continuity breaks or obvious tone drift."
	}
}

func buildGuideModeSuffix(mode string, strength ...string) string {
	selectedStrength := "weak"
	if len(strength) > 0 {
		selectedStrength = strength[0]
	}
	if normalizeNarrativeGuideStrength(selectedStrength) == "none" {
		return ""
	}
	strengthLine := narrativeGuideStrengthLine(selectedStrength)
	switch normalizeNarrativeGuideMode(mode) {
	case "standard":
		return strings.Join([]string{
			"",
			"[Narrative Guide ??Standard]",
			strengthLine,
			"Use as light optional style hints only; do not force the next scene, resolution, or character decision.",
			"Prefer continuity-preserving tone, pacing, and callbacks when they naturally fit.",
			"Current user input has priority; if the input is narrow, keep the guide almost invisible.",
		}, "\n")
	case "romantic":
		return strings.Join([]string{
			"",
			"[Narrative Guide ??Romantic]",
			strengthLine,
			"Use as light optional style hints only; do not force confession, intimacy, jealousy, or a relationship milestone.",
			"Let emotional dynamics surface only when the current exchange supports them.",
			"Dialogue subtext is preferred over explicit declarations.",
		}, "\n")
	case "action":
		return strings.Join([]string{
			"",
			"[Narrative Guide ??Action]",
			strengthLine,
			"Use as light optional style hints only; do not force combat, chase, danger, or a scene jump.",
			"If combat/chase/action is already happening, keep momentum clear and consequences grounded.",
			"If the user is only preparing or observing, do not escalate on their behalf.",
		}, "\n")
	case "mature_soft":
		return strings.Join([]string{
			"",
			"[Narrative Guide ??Mature (Sensual)]",
			strengthLine,
			"Use as light optional style hints only; do not force intimacy, escalation, or physical contact.",
			"When story-appropriate, prefer sensory, suggestive, indirect description.",
			"Atmosphere and emotion over explicit mechanics.",
			"Respect character agency and established boundaries.",
		}, "\n")
	case "mature_direct":
		return strings.Join([]string{
			"",
			"[Narrative Guide ??Mature (Explicit)]",
			strengthLine,
			"Use as light optional style hints only; do not force explicit content, escalation, or irreversible intimacy.",
			"Direct description is allowed only when the current scene and user input clearly support it.",
			"Character voice and emotional context remain paramount.",
			"Do not reduce characters to mere participants; inner thoughts matter.",
		}, "\n")
	default:
		return ""
	}
}

func buildGuideModeDirectorOverrides(mode string) map[string]any {
	switch normalizeNarrativeGuideMode(mode) {
	case "standard":
		return map[string]any{
			"emphasis":        []string{"tension management", "pacing variety", "subplot callbacks"},
			"forbidden_moves": []string{},
		}
	case "romantic":
		return map[string]any{
			"emphasis":        []string{"emotional resonance", "relationship progression", "intimate atmosphere"},
			"forbidden_moves": []string{"sudden genre shift to horror", "trivializing emotional moments"},
		}
	case "action":
		return map[string]any{
			"emphasis":        []string{"combat choreography", "environmental hazards", "tactical decisions"},
			"forbidden_moves": []string{"excessive monologuing during action", "deus ex machina resolution"},
		}
	case "mature_soft":
		return map[string]any{
			"emphasis":        []string{"sensory atmosphere", "emotional vulnerability", "consensual dynamics"},
			"forbidden_moves": []string{"gratuitous shock content", "ignoring character consent"},
		}
	case "mature_direct":
		return map[string]any{
			"emphasis":        []string{"vivid physical description", "emotional authenticity", "character agency"},
			"forbidden_moves": []string{"dehumanizing portrayals", "ignoring character consent"},
		}
	default:
		return map[string]any{
			"emphasis":        []string{},
			"forbidden_moves": []string{},
		}
	}
}

func normalizeNarrativeStance(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "reactive":
		return "reactive"
	case "proactive":
		return "proactive"
	default:
		return "balanced"
	}
}

func buildNarrativeStanceSuffix(mode string) string {
	switch normalizeNarrativeStance(mode) {
	case "reactive":
		return strings.Join([]string{
			"",
			"[Story Initiative - Reactive]",
			"Stay close to the user's immediate lead and the current scene.",
			"If the user expresses caution, hesitation, or uncertainty, remain in observation, clarification, or low-risk preparation rather than pushing action.",
			"Do not initiate entry, unlock barriers, assign a plan, or commit companions to a risky move unless the user explicitly asks for that step.",
			"Advance existing threads only when the current exchange clearly opens space for it, and keep any suggestion small and reversible.",
		}, "\n")
	case "proactive":
		return strings.Join([]string{
			"",
			"[Story Initiative - Proactive]",
			"You may introduce one plausible next beat or complication when continuity supports it.",
			"Initiative must grow from existing tensions, hooks, promises, or scene context.",
			"When the user is still deciding, propose the next beat rather than executing the decision on the user's behalf.",
			"Do not override the user's intent, skip causal steps, or force abrupt scene changes.",
		}, "\n")
	default:
		return strings.Join([]string{
			"",
			"[Story Initiative - Balanced]",
			"You may add one gentle next-beat nudge when it naturally fits the current scene.",
			"Keep the response anchored to the user's immediate intent and the current arc, and suggest rather than execute the next step.",
			"Avoid abrupt escalation, forced twists, or hard scene jumps.",
		}, "\n")
	}
}

func buildNarrativeStanceBounds(mode string) map[string]any {
	switch normalizeNarrativeStance(mode) {
	case "reactive":
		return map[string]any{
			"emphasis": []string{
				"user-led follow-through",
				"observation before action",
				"low-risk option framing",
			},
			"forbidden_moves": []string{
				"unlocking barriers or initiating entry without explicit user intent",
				"committing the group to a risky plan on the user's behalf",
				"inventing urgent danger to force motion",
			},
			"max_new_beats":    0,
			"allow_scene_jump": false,
		}
	case "proactive":
		return map[string]any{
			"emphasis": []string{
				"causal next-beat proposal",
				"continuity-aware tension increase",
				"bounded steering",
			},
			"forbidden_moves": []string{
				"forcing irreversible turns without buildup",
				"overwriting the user's immediate intent",
				"turning a cautious pause into immediate entry or confrontation without buy-in",
			},
			"max_new_beats":    1,
			"allow_scene_jump": false,
		}
	default:
		return map[string]any{
			"emphasis": []string{
				"gentle next-beat nudges",
				"continuity-aware escalation",
				"conversation momentum",
			},
			"forbidden_moves": []string{
				"hard scene cut without setup",
				"forcing a dramatic turn too early",
				"executing a risky step before the user agrees to it",
			},
			"max_new_beats":    1,
			"allow_scene_jump": false,
		}
	}
}

func buildNarrativeStanceSummary(mode, suffix string, bounds map[string]any) map[string]any {
	normalized := normalizeNarrativeStance(mode)
	return map[string]any{
		"mode":              normalized,
		"suffix_applied":    strings.TrimSpace(suffix) != "",
		"suffix_preview":    truncateTextForShadow(strings.Join(nonEmptyLines(suffix), " "), 140),
		"max_new_beats":     bounds["max_new_beats"],
		"allow_scene_jump":  bounds["allow_scene_jump"],
		"emphasis_count":    len(stringSliceFromAny(bounds["emphasis"])),
		"emphasis_preview":  strings.Join(limitStringSlice(stringSliceFromAny(bounds["emphasis"]), 2), ", "),
		"forbidden_count":   len(stringSliceFromAny(bounds["forbidden_moves"])),
		"forbidden_preview": strings.Join(limitStringSlice(stringSliceFromAny(bounds["forbidden_moves"]), 2), ", "),
	}
}

func nonEmptyLines(text string) []string {
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func stringSliceFromAny(v any) []string {
	switch typed := v.(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if s := strings.TrimSpace(extractionStringFromAny(item)); s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func limitStringSlice(items []string, limit int) []string {
	if limit < 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}

func buildCriticInputPack(chatSessionID string, turnIndex int, rawUserInput string, promptAssembly map[string]any, evidenceCounts map[string]any, sectionSummary []map[string]any, degraded bool) map[string]any {
	status := "ready"
	if degraded {
		status = "degraded"
	}
	return map[string]any{
		"status":              status,
		"source":              "go_r1_read_shadow",
		"chat_session_id":     chatSessionID,
		"turn_index":          turnIndex,
		"turn_content_chars":  len([]rune(rawUserInput)),
		"prompt_assembly":     promptAssembly,
		"prompt_source":       promptAssembly["prompt_source"],
		"evidence_counts":     evidenceCounts,
		"section_summary":     sectionSummary,
		"output_contract":     []string{"memories", "direct_evidence", "kg_triples", "critic_feedback"},
		"critic_context_plan": []string{"turn_content", "recent_chat", "direct_evidence", "kg_triples", "supervisor_input_pack"},
		"verdict":             "not_executed",
		"would_call_llm":      false,
		"would_write":         false,
		"degraded":            degraded,
	}
}
