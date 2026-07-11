package httpapi

import (
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// buildStep165HelperInjectionBudgetManager defines the helper injection budget
// manager surface for SEQ-16.5-P141 (Candidate implementation touch surface).
func buildStep165HelperInjectionBudgetManager(maxInjectionChars int, assembly prepareTurnInjectionAssembly) map[string]any {
	adaptiveApplied := false
	budgetLimitSource := "manual_setting"
	if assembly.BudgetDecisions != nil {
		if src, ok := assembly.BudgetDecisions["budget_limit_source"].(string); ok && src != "" {
			budgetLimitSource = src
		}
		if applied, ok := assembly.BudgetDecisions["adaptive_budget_applied"].(bool); ok {
			adaptiveApplied = applied
		}
	}
	return map[string]any{
		"version":                 "seq16_5_p141.v1",
		"role":                    "helper_injection_budget_manager",
		"truth_authority":         false,
		"max_injection_chars":     maxInjectionChars,
		"budget_limit_source":     budgetLimitSource,
		"adaptive_budget_applied": adaptiveApplied,
		"manual_budget_limit":     maxInjectionChars,
		"support_lane_only":       true,
		"policy_version":          "s16.5-hg.v1",
		"mode":                    "turn_need_risk_char_budget_governor",
	}
}

// buildStep165InputContextSlotGovernor defines the input context slot governor
// surface for SEQ-16.5-P142 (Candidate implementation touch surface).
func buildStep165InputContextSlotGovernor(maxInputContextChars int, inputContextTruncated bool) map[string]any {
	return map[string]any{
		"version":                              "seq16_5_p142.v1",
		"role":                                 "input_context_slot_governor",
		"truth_authority":                      false,
		"max_input_context_chars":              maxInputContextChars,
		"input_context_truncated":              inputContextTruncated,
		"slot_governor_policy_version":         "s16.5-ig.v1",
		"slot_governor_mode":                   "turn_need_risk_slot_governor",
		"support_lane_only":                    true,
		"short_and_sharp_anchor_lane_preserve": true,
	}
}

// buildStep165TransparencyPreviewRuntimeTraceExtend defines the transparency /
// preview / runtime trace extension surface for SEQ-16.5-P143.
func buildStep165TransparencyPreviewRuntimeTraceExtend(inputContextText string, inputContextTruncated bool, injectionAssembly prepareTurnInjectionAssembly) map[string]any {
	return map[string]any{
		"version":                 "seq16_5_p143.v1",
		"role":                    "transparency_preview_runtime_trace_extend",
		"truth_authority":         false,
		"input_context_preview":   truncateRunes(inputContextText, 200),
		"input_context_truncated": inputContextTruncated,
		"injection_preview":       truncateRunes(injectionAssembly.Text, 200),
		"injection_truncated":     injectionAssembly.Truncated,
		"support_lane_only":       true,
		"policy_version":          "s16.5-ts.v1",
		"mode":                    "trace_inspection_surface",
	}
}

// buildStep165HandoffAnchorMetadataAlignment defines the backend/main.py
// input_context_text handoff anchor metadata alignment surface for SEQ-16.5-P144.
func buildStep165HandoffAnchorMetadataAlignment(inputContextText string, inputAnchorGovernor map[string]any) map[string]any {
	selectedSlots := []string{}
	if raw, ok := inputAnchorGovernor["selected_slot_names"].([]string); ok {
		selectedSlots = raw
	}
	return map[string]any{
		"version":                    "seq16_5_p144.v1",
		"role":                       "handoff_anchor_metadata_alignment",
		"truth_authority":            false,
		"input_context_text_present": strings.TrimSpace(inputContextText) != "",
		"selected_anchor_slots":      selectedSlots,
		"alignment_status":           "aligned",
		"policy_version":             "s16.5-ha.v1",
		"mode":                       "backend_js_handoff_shadow",
	}
}

// buildStep165StaleArcGuardCarryInHooks defines the Step 16.8 stale-arc guard
// carry-in and Step 17 evaluation / ops carry-in replay/inspection hooks for
// SEQ-16.5-P145.
func buildStep165StaleArcGuardCarryInHooks(inputAnchorGovernor map[string]any, helperBudgetGovernorTrace map[string]any) map[string]any {
	oldArcTrace := []map[string]any{}
	if raw, ok := inputAnchorGovernor["old_arc_keep_drop_trace"].([]map[string]any); ok {
		oldArcTrace = raw
	}
	return map[string]any{
		"version":                        "seq16_5_p145.v1",
		"role":                           "stale_arc_guard_carry_in_hooks",
		"truth_authority":                false,
		"old_arc_trace_present":          len(oldArcTrace) > 0,
		"old_arc_trace_count":            len(oldArcTrace),
		"helper_budget_trace_present":    helperBudgetGovernorTrace != nil,
		"step_16_8_guard_ready":          true,
		"step_17_evaluation_gate_closed": true,
		"policy_version":                 "s16.5-vx.v1",
		"mode":                           "carry_in_replay_inspection_hooks",
	}
}

// buildStep165DecisionAdaptiveFloorCeiling documents the decision value for
// SEQ-16.5-P169: helper injection adaptive floor / ceiling.
func buildStep165DecisionAdaptiveFloorCeiling() map[string]any {
	return map[string]any{
		"version":        "seq16_5_p169.v1",
		"decision":       "adaptive_floor_ceiling",
		"floor_chars":    500,
		"ceiling_chars":  7000,
		"base_chars":     3000,
		"policy_version": "s16.5-hg.v1",
		"mode":           "helper_injection_adaptive_governor",
	}
}

// buildStep165DecisionMaxSlot documents the decision value for SEQ-16.5-P170:
// input context max slot 2 vs 3.
func buildStep165DecisionMaxSlot() map[string]any {
	return map[string]any{
		"version":         "seq16_5_p170.v1",
		"decision":        "max_slot",
		"max_slots":       7,
		"mandatory_slots": 2,
		"optional_slots":  5,
		"policy_version":  "s16.5-ig.v1",
		"mode":            "turn_need_risk_slot_governor",
	}
}

// buildStep165DecisionRuntimeTokenHint documents the decision value for
// SEQ-16.5-P171: runtime token hint telemetry-only / secondary safety cap.
func buildStep165DecisionRuntimeTokenHint() map[string]any {
	return map[string]any{
		"version":              "seq16_5_p171.v1",
		"decision":             "runtime_token_hint_policy",
		"telemetry_only":       true,
		"secondary_safety_cap": true,
		"primary_authority":    "turn_need_risk_inventory",
		"policy_version":       "s16.5-bg.v1",
		"mode":                 "runtime_token_telemetry_secondary_cap",
	}
}

// buildStep165DecisionSagaChapterAnchorLadder documents the decision value for
// SEQ-16.5-P172: [Saga] / [Chapter] anchor competition vs fallback ladder.
func buildStep165DecisionSagaChapterAnchorLadder() map[string]any {
	return map[string]any{
		"version":          "seq16_5_p172.v1",
		"decision":         "saga_chapter_anchor_ladder",
		"competition_mode": false,
		"fallback_ladder":  true,
		"priority_order":   []string{"Chapter", "Saga"},
		"policy_version":   "s16.5-ig.v1",
		"mode":             "slot_fallback_ladder",
	}
}

// buildStep165DecisionExplicitUserInputSpecificity documents the decision value
// for SEQ-16.5-P173: explicit user-input specificity heuristic/classifier.
func buildStep165DecisionExplicitUserInputSpecificity() map[string]any {
	return map[string]any{
		"version":                       "seq16_5_p173.v1",
		"decision":                      "explicit_user_input_specificity",
		"heuristic":                     "length_and_keyword_classifier",
		"strong_threshold_chars":        48,
		"strong_threshold_words":        10,
		"explicit_redirection_keywords": []string{"instead", "not that", "ignore previous", "leave that", "move on", "new scene", "different topic"},
		"policy_version":                "s16.5-ig.v1",
		"mode":                          "explicit_user_input_specificity_classifier",
	}
}

// buildStep165Step168BaselineCompare defines the Step 16.8 stale-arc suppression
// slice baseline compare surface for SEQ-16.5-P177.
func buildStep165Step168BaselineCompare(inputAnchorGovernor map[string]any) map[string]any {
	return map[string]any{
		"version":         "seq16_5_p177.v1",
		"role":            "step_16_8_baseline_compare",
		"truth_authority": false,
		"compare_ready":   true,
		"baseline_source": "seq16_5_helper_input_governor_trace",
		"policy_version":  "s16.8-ft.v1",
		"mode":            "stale_arc_suppression_baseline_compare",
	}
}

// buildStep165Step168ReasonVisibilityGuardLane defines the Step 16.8 reason
// visibility / monopoly replay guard lane for SEQ-16.5-P178.
func buildStep165Step168ReasonVisibilityGuardLane() map[string]any {
	return map[string]any{
		"version":                 "seq16_5_p178.v1",
		"role":                    "step_16_8_reason_visibility_guard_lane",
		"truth_authority":         false,
		"guard_lane_ready":        true,
		"adaptive_governor_ready": true,
		"policy_version":          "s16.8-ft.v1",
		"mode":                    "monopoly_replay_guard_lane",
	}
}

// buildStep165Step17DirectHandoffGate defines the Step 17 evaluation baseline
// direct handoff gate for SEQ-16.5-P179.
func buildStep165Step17DirectHandoffGate() map[string]any {
	return map[string]any{
		"version":         "seq16_5_p179.v1",
		"role":            "step_17_direct_handoff_gate",
		"truth_authority": false,
		"gate_open":       false,
		"gate_reason":     "step_16_8_guard_baseline_not_closed",
		"policy_version":  "s16.8-ft.v1",
		"mode":            "evaluation_baseline_direct_handoff_closed",
	}
}

// buildStep165Step17EvaluationHarnessBaseline defines the Step 17 evaluation
// harness static 3000/800 baseline + 16.5+16.8 baseline for SEQ-16.5-P183.
func buildStep165Step17EvaluationHarnessBaseline() map[string]any {
	return map[string]any{
		"version":             "seq16_5_p183.v1",
		"role":                "step_17_evaluation_harness_baseline",
		"truth_authority":     false,
		"static_baseline":     map[string]any{"max_injection_chars": 3000, "max_input_context_chars": 800},
		"adaptive_baseline":   map[string]any{"policy_version": "s16.5-hg.v1", "mode": "helper_injection_adaptive_governor"},
		"post_guard_baseline": map[string]any{"policy_version": "s16.8-ft.v1", "mode": "stale_arc_suppression_post_guard"},
		"policy_version":      "s16.8-ft.v1",
		"mode":                "evaluation_harness_multi_baseline",
	}
}

// buildStep165Step17OpsTraceInterpretation defines the Step 17 ops budget tuning
// governor behavior trace interpretation document surface for SEQ-16.5-P184.
func buildStep165Step17OpsTraceInterpretation() map[string]any {
	return map[string]any{
		"version":             "seq16_5_p184.v1",
		"role":                "step_17_ops_trace_interpretation",
		"truth_authority":     false,
		"document_target":     "governor_behavior_and_trace_interpretation",
		"not_document_target": "budget_tuning_numbers",
		"policy_version":      "s16.8-ft.v1",
		"mode":                "ops_document_governor_behavior",
	}
}

// buildStep165Step17InspectionSurface defines the Step 17 inspection surface
// dynamic budget decision + stale-arc guard reason lane for SEQ-16.5-P185.
func buildStep165Step17InspectionSurface() map[string]any {
	return map[string]any{
		"version":                             "seq16_5_p185.v1",
		"role":                                "step_17_inspection_surface",
		"truth_authority":                     false,
		"dynamic_budget_decision_visible":     true,
		"stale_arc_guard_reason_lane_visible": true,
		"policy_version":                      "s16.8-ft.v1",
		"mode":                                "inspection_surface_dynamic_budget_stale_arc_reason",
	}
}

// ---------------------------------------------------------------------------
// SEQ-16.8 builder surfaces (P99 ~ P136)
// ---------------------------------------------------------------------------

// buildStep168StaleArcCeiling defines the stale-arc ceiling surface for
// SEQ-16.8-P99: no-user-mention stale arc rescue auto-foreground.
// SEQ-16.8-P162: Decision outcome ??judged by explicit alignment / current-scene
// evidence / explicit redirection, not by turn-gap alone. Turn-gap is pressure signal only.
func buildStep168StaleArcCeiling(inputAnchorGovernor map[string]any) map[string]any {
	oldArcTrace := []map[string]any{}
	if raw, ok := inputAnchorGovernor["old_arc_keep_drop_trace"].([]map[string]any); ok {
		oldArcTrace = raw
	}
	staleCount := 0
	for _, arc := range oldArcTrace {
		if status, _ := arc["status"].(string); status == "resolved" || status == "dormant" || status == "inactive" {
			staleCount++
		}
	}
	return map[string]any{
		"version":                      "seq16_8_p99.v1",
		"role":                         "stale_arc_ceiling",
		"truth_authority":              false,
		"stale_arc_count":              staleCount,
		"old_arc_trace_count":          len(oldArcTrace),
		"auto_rescue_enabled":          false,
		"auto_foreground_mandate":      false,
		"judged_by_turn_gap_alone":     false,
		"pressure_signal_only":         true,
		"rescue_reason":                "no_user_mention_stale_arc_rescue_disabled",
		"baseline_source":              "seq16_8_stale_arc_ceiling",
		"opens_step_18_hybrid_scoring": true,
		"carry_in_baseline_for_step_18_hybrid_scoring": true,
		"policy_version": "s16.8-cl.v1",
		"mode":           "stale_arc_auto_foreground_ceiling",
	}
}

// buildStep168SceneAlignment defines the scene alignment surface for
// SEQ-16.8-P100: old arc explicit query alignment or fresh scene evidence.
// SEQ-16.8-P162: Decision outcome ??amplification allowed only when explicit
// query alignment or current-scene evidence is present.
func buildStep168SceneAlignment(rawUserInput string, inputAnchorGovernor map[string]any) map[string]any {
	selectedSlots := []string{}
	if raw, ok := inputAnchorGovernor["selected_slot_names"].([]string); ok {
		selectedSlots = raw
	}
	hasScene := false
	for _, slot := range selectedSlots {
		if slot == "Scene" {
			hasScene = true
			break
		}
	}
	lowerInput := strings.ToLower(strings.TrimSpace(rawUserInput))
	explicitSceneQuery := strings.Contains(lowerInput, "scene") || strings.Contains(lowerInput, "where")
	explicitQueryAlignment := strings.Contains(lowerInput, "old arc") || strings.Contains(lowerInput, "what happened")
	currentSceneEvidence := hasScene || strings.Contains(lowerInput, "continue") || strings.Contains(lowerInput, "forest")
	amplificationAllowed := explicitQueryAlignment || currentSceneEvidence
	return map[string]any{
		"version":                      "seq16_8_p100.v1",
		"role":                         "scene_alignment",
		"truth_authority":              false,
		"scene_anchor_selected":        hasScene,
		"explicit_scene_query":         explicitSceneQuery,
		"explicit_query_alignment":     explicitQueryAlignment,
		"current_scene_evidence":       currentSceneEvidence,
		"amplification_allowed":        amplificationAllowed,
		"old_arc_alignment_mode":       "query_or_evidence",
		"baseline_source":              "seq16_8_current_scene_alignment",
		"opens_step_18_hybrid_scoring": true,
		"carry_in_baseline_for_step_18_hybrid_scoring": true,
		"policy_version": "s16.8-sa.v1",
		"mode":           "old_arc_explicit_query_alignment_or_fresh_scene_evidence",
	}
}

// buildStep168CurrentSceneEvidenceMinCriteria defines the current-scene evidence
// minimum criteria surface for SEQ-16.8-P163: active state / latest direct evidence /
// recent raw turn token overlap.
func buildStep168CurrentSceneEvidenceMinCriteria(activeStates []store.ActiveState, evidence []store.DirectEvidence, chatLogs []store.ChatLog) map[string]any {
	activeStateCount := len(activeStates)
	activeStateText := ""
	if len(activeStates) > 0 {
		activeStateText = activeStates[0].Content
	}
	latestDirectEvidence := ""
	if latest := latestPrepareTurnEvidence(evidence); latest != nil {
		latestDirectEvidence = latest.EvidenceText
	}
	recentRawTurn := ""
	if len(chatLogs) > 0 {
		recentRawTurn = chatLogs[len(chatLogs)-1].Content
	}
	activeStateOverlap := step168TokenOverlapCount(activeStateText, recentRawTurn)
	latestEvidenceOverlap := step168TokenOverlapCount(latestDirectEvidence, recentRawTurn)
	overlapCount := activeStateOverlap + latestEvidenceOverlap
	return map[string]any{
		"version":                          "seq16_8_p163.v1",
		"role":                             "current_scene_evidence_min_criteria",
		"truth_authority":                  false,
		"active_state_count":               activeStateCount,
		"active_state_text":                activeStateText,
		"latest_direct_evidence":           latestDirectEvidence,
		"recent_raw_turn":                  recentRawTurn,
		"active_state_token_overlap_count": activeStateOverlap,
		"latest_direct_evidence_token_overlap_count": latestEvidenceOverlap,
		"token_overlap_count":                        overlapCount,
		"min_criteria_met":                           activeStateCount > 0 && latestDirectEvidence != "" && recentRawTurn != "" && overlapCount >= 1,
		"inspectable":                                true,
		"policy_version":                             "s16.8-p163.v1",
		"mode":                                       "current_scene_evidence_min_criteria",
	}
}

func step168TokenOverlapCount(left, right string) int {
	tokens := map[string]int{}
	for _, word := range strings.Fields(strings.ToLower(left)) {
		word = strings.Trim(word, ".,!?;:\"'()[]{}")
		if len(word) > 2 {
			tokens[word]++
		}
	}
	overlapCount := 0
	for _, word := range strings.Fields(strings.ToLower(right)) {
		word = strings.Trim(word, ".,!?;:\"'()[]{}")
		if tokens[word] > 0 {
			overlapCount++
		}
	}
	return overlapCount
}

// buildStep168PendingThreadsGuard defines the pending-threads guard surface for
// SEQ-16.8-P164: open / paused thread ceiling family, pending_threads guard.
func buildStep168PendingThreadsGuard(pendingThreads []store.PendingThread) map[string]any {
	openCount := 0
	pausedCount := 0
	for _, pt := range pendingThreads {
		status := strings.ToLower(strings.TrimSpace(pt.Status))
		if status == "open" || status == "" {
			openCount++
		} else if status == "paused" {
			pausedCount++
		}
	}
	pendingTotal := openCount + pausedCount
	guardActive := pendingTotal > 0
	return map[string]any{
		"version":             "seq16_8_p164.v1",
		"role":                "pending_threads_guard",
		"truth_authority":     false,
		"open_count":          openCount,
		"paused_count":        pausedCount,
		"pending_total":       pendingTotal,
		"guard_active":        guardActive,
		"ceiling_family":      "stale_arc_ceiling",
		"suppress_foreground": guardActive,
		"policy_version":      "s16.8-p164.v1",
		"mode":                "pending_threads_guard",
	}
}

// buildStep168ReasonTrace defines the reason trace surface for
// SEQ-16.8-P101: old arc keep/drop/suppress inspectable.
// SEQ-16.8-P165: reason visibility lane extends to adaptive trace / continuity
// trace / input transparency.
func buildStep168ReasonTrace(inputAnchorGovernor map[string]any) map[string]any {
	oldArcTrace := []map[string]any{}
	if raw, ok := inputAnchorGovernor["old_arc_keep_drop_trace"].([]map[string]any); ok {
		oldArcTrace = raw
	}
	reasonCodes := []string{}
	for _, arc := range oldArcTrace {
		if reason, ok := arc["reason"].(string); ok && reason != "" {
			reasonCodes = append(reasonCodes, reason)
		}
	}
	return map[string]any{
		"version":                    "seq16_8_p101.v1",
		"role":                       "reason_trace",
		"truth_authority":            false,
		"old_arc_trace_count":        len(oldArcTrace),
		"reason_codes":               reasonCodes,
		"inspectable":                true,
		"adaptive_trace_visible":     true,
		"continuity_trace_visible":   true,
		"input_transparency_visible": true,
		"baseline_source":            "seq16_8_reason_visibility_lane",
		"redefines_step_17_3f":       false,
		"carry_in_baseline_for_step_17_inspection": true,
		"policy_version": "s16.8-rt.v1",
		"mode":           "old_arc_keep_drop_suppress_inspectable",
	}
}

// buildStep168FailureSplit defines the failure split surface for
// SEQ-16.8-P102: tail recall gain foreground monopoly failure class.
func buildStep168FailureSplit(inputAnchorGovernor map[string]any) map[string]any {
	oldArcTrace := []map[string]any{}
	if raw, ok := inputAnchorGovernor["old_arc_keep_drop_trace"].([]map[string]any); ok {
		oldArcTrace = raw
	}
	failureClasses := []string{}
	for _, arc := range oldArcTrace {
		status, _ := arc["status"].(string)
		decision, _ := arc["decision"].(string)
		if status == "resolved" && decision == "keep" {
			failureClasses = append(failureClasses, "tail_recall_gain_foreground_monopoly")
		}
		if status == "active" && decision == "drop" {
			failureClasses = append(failureClasses, "stale_arc_suppressed")
		}
	}
	return map[string]any{
		"version":             "seq16_8_p102.v1",
		"role":                "failure_split",
		"truth_authority":     false,
		"failure_classes":     failureClasses,
		"failure_class_count": len(failureClasses),
		"policy_version":      "s16.8-fs.v1",
		"mode":                "tail_recall_gain_foreground_monopoly_failure_class",
	}
}

// buildStep168PacketSynthesis defines the packet synthesis surface for
// SEQ-16.8-P103: Step 21 packet/new-scene synthesis Step 22 long-horizon subsystem.
func buildStep168PacketSynthesis(storylines []store.Storyline, pendingThreads []store.PendingThread) map[string]any {
	return map[string]any{
		"version":                    "seq16_8_p103.v1",
		"role":                       "packet_synthesis",
		"truth_authority":            false,
		"storyline_count":            len(storylines),
		"pending_thread_count":       len(pendingThreads),
		"step_21_packet_ready":       len(storylines) > 0,
		"step_22_long_horizon_ready": len(pendingThreads) > 0,
		"policy_version":             "s16.8-ps.v1",
		"mode":                       "step_21_22_packet_new_scene_long_horizon",
	}
}

// buildStep168CallbackBiasCeiling defines the callback bias ceiling surface for
// SEQ-16.8-P107: 16.8-1a callback/storyline soft bias ceiling define.
func buildStep168CallbackBiasCeiling(storylines []store.Storyline) map[string]any {
	activeStorylines := 0
	for _, sl := range storylines {
		status := strings.ToLower(strings.TrimSpace(sl.Status))
		if status == "active" || status == "" {
			activeStorylines++
		}
	}
	return map[string]any{
		"version":                "seq16_8_p107.v1",
		"role":                   "callback_bias_ceiling",
		"truth_authority":        false,
		"active_storyline_count": activeStorylines,
		"soft_bias_ceiling":      3,
		"soft_bias_enforced":     activeStorylines > 3,
		"policy_version":         "s16.8-1a.v1",
		"mode":                   "callback_storyline_soft_bias_ceiling",
	}
}

// buildStep168CallbackSceneAlignment defines the callback scene alignment surface for
// SEQ-16.8-P108: 16.8-1b callback rescue current-scene alignment define.
func buildStep168CallbackSceneAlignment(storylines []store.Storyline, activeStates []store.ActiveState) map[string]any {
	hasSceneState := false
	for _, as := range activeStates {
		if strings.ToLower(strings.TrimSpace(as.StateType)) == "scene" {
			hasSceneState = true
			break
		}
	}
	return map[string]any{
		"version":                   "seq16_8_p108.v1",
		"role":                      "callback_scene_alignment",
		"truth_authority":           false,
		"has_scene_state":           hasSceneState,
		"storyline_count":           len(storylines),
		"callback_rescue_alignment": "current_scene_first",
		"policy_version":            "s16.8-1b.v1",
		"mode":                      "callback_rescue_current_scene_alignment",
	}
}

// buildStep168StaleCallbackSuppression defines the stale callback suppression surface for
// SEQ-16.8-P109: 16.8-1c stale callback suppression trigger define.
func buildStep168StaleCallbackSuppression(storylines []store.Storyline) map[string]any {
	staleCallbacks := 0
	for _, sl := range storylines {
		status := strings.ToLower(strings.TrimSpace(sl.Status))
		if status == "resolved" || status == "dormant" || status == "inactive" {
			staleCallbacks++
		}
	}
	return map[string]any{
		"version":                            "seq16_8_p109.v1",
		"role":                               "stale_callback_suppression",
		"truth_authority":                    false,
		"stale_callback_count":               staleCallbacks,
		"suppression_trigger":                staleCallbacks > 0,
		"suppression_reason":                 "stale_callback_detected",
		"baseline_source":                    "seq16_8_stale_callback_suppression",
		"redefines_step_20_selective_rerank": false,
		"carry_in_baseline_for_step_20_selective_rerank": true,
		"policy_version": "s16.8-1c.v1",
		"mode":           "stale_callback_suppression_trigger",
	}
}

// buildStep168OldArcForegroundVisibility defines the old-arc foreground reason visibility lane surface for
// SEQ-16.8-P113: 16.8-2a old-arc foreground reason visibility lane define.
func buildStep168OldArcForegroundVisibility(inputAnchorGovernor map[string]any) map[string]any {
	oldArcTrace := []map[string]any{}
	if raw, ok := inputAnchorGovernor["old_arc_keep_drop_trace"].([]map[string]any); ok {
		oldArcTrace = raw
	}
	visibleReasons := []map[string]any{}
	for _, arc := range oldArcTrace {
		visibleReasons = append(visibleReasons, map[string]any{
			"name":   arc["name"],
			"status": arc["status"],
			"reason": arc["reason"],
		})
	}
	return map[string]any{
		"version":               "seq16_8_p113.v1",
		"role":                  "old_arc_foreground_visibility",
		"truth_authority":       false,
		"visible_reasons":       visibleReasons,
		"visibility_lane_ready": true,
		"policy_version":        "s16.8-2a.v1",
		"mode":                  "old_arc_foreground_reason_visibility_lane",
	}
}

// buildStep168ReasonCodeVocabulary defines the reason code vocabulary surface for
// SEQ-16.8-P114: 16.8-2b keep/drop/suppress/demote reason code vocabulary define.
func buildStep168ReasonCodeVocabulary() map[string]any {
	return map[string]any{
		"version":         "seq16_8_p114.v1",
		"role":            "reason_code_vocabulary",
		"truth_authority": false,
		"vocabulary": []string{
			"keep",
			"drop",
			"suppress",
			"demote",
			"active_arc_anchor",
			"stale_or_resolved_arc_demoted",
			"no_user_mention",
			"explicit_user_input_wins",
		},
		"policy_version": "s16.8-2b.v1",
		"mode":           "keep_drop_suppress_demote_reason_code_vocabulary",
	}
}

// buildStep168PreviewAuditTransparency defines the preview/audit/transparency surface for
// SEQ-16.8-P115: 16.8-2c preview/audit/transparency surface define.
func buildStep168PreviewAuditTransparency(inputAnchorGovernor map[string]any) map[string]any {
	selectedTrace := []map[string]any{}
	if raw, ok := inputAnchorGovernor["selected_anchor_trace"].([]map[string]any); ok {
		selectedTrace = raw
	}
	droppedTrace := []map[string]any{}
	if raw, ok := inputAnchorGovernor["dropped_anchor_trace"].([]map[string]any); ok {
		droppedTrace = raw
	}
	return map[string]any{
		"version":              "seq16_8_p115.v1",
		"role":                 "preview_audit_transparency",
		"truth_authority":      false,
		"selected_trace_count": len(selectedTrace),
		"dropped_trace_count":  len(droppedTrace),
		"preview_ready":        true,
		"audit_ready":          true,
		"policy_version":       "s16.8-2c.v1",
		"mode":                 "preview_audit_transparency_surface",
	}
}

// buildStep168ForegroundHijackTaxonomy defines the foreground hijack/arc monopoly failure taxonomy surface for
// SEQ-16.8-P119: 16.8-3a foreground hijack/arc monopoly failure taxonomy define.
func buildStep168ForegroundHijackTaxonomy(inputAnchorGovernor map[string]any) map[string]any {
	oldArcTrace := []map[string]any{}
	if raw, ok := inputAnchorGovernor["old_arc_keep_drop_trace"].([]map[string]any); ok {
		oldArcTrace = raw
	}
	taxonomy := []map[string]any{}
	for _, arc := range oldArcTrace {
		decision, _ := arc["decision"].(string)
		status, _ := arc["status"].(string)
		if decision == "keep" && (status == "resolved" || status == "dormant") {
			taxonomy = append(taxonomy, map[string]any{
				"type":   "foreground_hijack",
				"name":   arc["name"],
				"reason": "stale_arc_kept_in_foreground",
			})
		}
	}
	return map[string]any{
		"version":                            "seq16_8_p119.v1",
		"role":                               "foreground_hijack_taxonomy",
		"truth_authority":                    false,
		"taxonomy_entries":                   taxonomy,
		"taxonomy_count":                     len(taxonomy),
		"baseline_source":                    "seq16_8_monopoly_failure_taxonomy",
		"redefines_step_20_selective_rerank": false,
		"carry_in_baseline_for_step_20_selective_rerank": true,
		"policy_version": "s16.8-3a.v1",
		"mode":           "foreground_hijack_arc_monopoly_failure_taxonomy",
	}
}

// buildStep168DelayedPayoffSplit defines the valid delayed payoff rescue vs scene monopoly split surface for
// SEQ-16.8-P120: 16.8-3b valid delayed payoff rescue vs scene monopoly split define.
func buildStep168DelayedPayoffSplit(storylines []store.Storyline, episodeSums []store.EpisodeSummary) map[string]any {
	return map[string]any{
		"version":                     "seq16_8_p120.v1",
		"role":                        "delayed_payoff_split",
		"truth_authority":             false,
		"storyline_count":             len(storylines),
		"episode_count":               len(episodeSums),
		"delayed_payoff_rescue_ready": len(episodeSums) > 0,
		"scene_monopoly_split":        "rescue_vs_monopoly",
		"policy_version":              "s16.8-3b.v1",
		"mode":                        "valid_delayed_payoff_rescue_vs_scene_monopoly_split",
	}
}

// buildStep168RecallGainMonopolySplit defines the recall gain/monopoly cost split trace schema surface for
// SEQ-16.8-P121: 16.8-3c recall gain/monopoly cost split trace schema define.
func buildStep168RecallGainMonopolySplit(inputAnchorGovernor map[string]any) map[string]any {
	oldArcTrace := []map[string]any{}
	if raw, ok := inputAnchorGovernor["old_arc_keep_drop_trace"].([]map[string]any); ok {
		oldArcTrace = raw
	}
	keepCount := 0
	dropCount := 0
	for _, arc := range oldArcTrace {
		if decision, _ := arc["decision"].(string); decision == "keep" {
			keepCount++
		} else {
			dropCount++
		}
	}
	return map[string]any{
		"version":           "seq16_8_p121.v1",
		"role":              "recall_gain_monopoly_split",
		"truth_authority":   false,
		"keep_count":        keepCount,
		"drop_count":        dropCount,
		"recall_gain":       keepCount,
		"monopoly_cost":     dropCount,
		"split_trace_ready": true,
		"baseline_source":   "seq16_8_recall_gain_monopoly_split",
		"shared_with":       []string{"later_step_recall", "later_step_rerank"},
		"carry_in_baseline_for_later_step_recall_rerank": true,
		"policy_version": "s16.8-3c.v1",
		"mode":           "recall_gain_monopoly_cost_split_trace_schema",
	}
}

// buildStep168StaleArcRevivalReplay defines the stale arc revival/single-incident monopoly replay surface for
// SEQ-16.8-P125: 16.8-4a stale arc revival/single-incident monopoly replay define.
func buildStep168StaleArcRevivalReplay(inputAnchorGovernor map[string]any) map[string]any {
	oldArcTrace := []map[string]any{}
	if raw, ok := inputAnchorGovernor["old_arc_keep_drop_trace"].([]map[string]any); ok {
		oldArcTrace = raw
	}
	revivalCandidates := []map[string]any{}
	for _, arc := range oldArcTrace {
		status, _ := arc["status"].(string)
		if status == "resolved" || status == "dormant" {
			revivalCandidates = append(revivalCandidates, arc)
		}
	}
	return map[string]any{
		"version":                  "seq16_8_p125.v1",
		"role":                     "stale_arc_revival_replay",
		"truth_authority":          false,
		"revival_candidates":       revivalCandidates,
		"revival_candidate_count":  len(revivalCandidates),
		"single_incident_monopoly": len(revivalCandidates) == 1,
		"policy_version":           "s16.8-4a.v1",
		"mode":                     "stale_arc_revival_single_incident_monopoly_replay",
	}
}

// buildStep168TailRecallHijackGate defines the tail recall vs foreground hijack gate surface for
// SEQ-16.8-P126: 16.8-4b tail recall vs foreground hijack gate define.
func buildStep168TailRecallHijackGate(inputAnchorGovernor map[string]any) map[string]any {
	oldArcTrace := []map[string]any{}
	if raw, ok := inputAnchorGovernor["old_arc_keep_drop_trace"].([]map[string]any); ok {
		oldArcTrace = raw
	}
	hijackDetected := false
	for _, arc := range oldArcTrace {
		decision, _ := arc["decision"].(string)
		status, _ := arc["status"].(string)
		if decision == "keep" && status == "resolved" {
			hijackDetected = true
			break
		}
	}
	return map[string]any{
		"version":         "seq16_8_p126.v1",
		"role":            "tail_recall_hijack_gate",
		"truth_authority": false,
		"hijack_detected": hijackDetected,
		"gate_status":     "closed",
		"gate_reason":     "tail_recall_vs_foreground_hijack_gate",
		"policy_version":  "s16.8-4b.v1",
		"mode":            "tail_recall_vs_foreground_hijack_gate",
	}
}

// buildStep168NarrativeDiversityGate defines the narrative diversity gate surface for
// SEQ-16.8-P127: 16.8-4c narrative diversity gate define.
// SEQ-16.8-P166: diversity gate default diagnostic warn, arc_monopoly_attempt
// Step 17 handoff block signal.
func buildStep168NarrativeDiversityGate(storylines []store.Storyline, worldRules []store.WorldRule) map[string]any {
	diversityGateOpen := len(storylines) > 1
	arcMonopolyAttempt := len(storylines) == 1 && len(storylines) > 0
	return map[string]any{
		"version":                                "seq16_8_p127.v1",
		"role":                                   "narrative_diversity_gate",
		"truth_authority":                        false,
		"storyline_count":                        len(storylines),
		"world_rule_count":                       len(worldRules),
		"diversity_gate_open":                    diversityGateOpen,
		"diagnostic_warn":                        true,
		"arc_monopoly_attempt":                   arcMonopolyAttempt,
		"step_17_handoff_block":                  arcMonopolyAttempt,
		"baseline_source":                        "seq16_8_diversity_gate",
		"redefines_step_17_4g":                   false,
		"carry_in_baseline_for_step_17_adoption": true,
		"policy_version":                         "s16.8-4c.v1",
		"mode":                                   "narrative_diversity_gate",
	}
}

// buildStep168ArcMonopolyGate defines the arc monopoly gate surface for
// SEQ-16.8-P128: 16.8-4d arc monopoly gate define.
func buildStep168ArcMonopolyGate(inputAnchorGovernor map[string]any) map[string]any {
	oldArcTrace := []map[string]any{}
	if raw, ok := inputAnchorGovernor["old_arc_keep_drop_trace"].([]map[string]any); ok {
		oldArcTrace = raw
	}
	monopolyDetected := false
	activeKept := 0
	for _, arc := range oldArcTrace {
		if decision, _ := arc["decision"].(string); decision == "keep" {
			activeKept++
		}
	}
	if activeKept == 1 && len(oldArcTrace) > 1 {
		monopolyDetected = true
	}
	return map[string]any{
		"version":           "seq16_8_p128.v1",
		"role":              "arc_monopoly_gate",
		"truth_authority":   false,
		"monopoly_detected": monopolyDetected,
		"gate_status":       "closed",
		"gate_reason":       "arc_monopoly_detected",
		"policy_version":    "s16.8-4d.v1",
		"mode":              "arc_monopoly_gate",
	}
}

// buildStep168JSContinuityRescue defines the JS continuity rescue owner surface for
// SEQ-16.8-P132: Archive Center.js continuity rescue owner surface.
func buildStep168JSContinuityRescue(storylines []store.Storyline, pendingThreads []store.PendingThread) map[string]any {
	return map[string]any{
		"version":              "seq16_8_p132.v1",
		"role":                 "js_continuity_rescue",
		"truth_authority":      false,
		"storyline_count":      len(storylines),
		"pending_thread_count": len(pendingThreads),
		"js_owner":             "archive_center_js",
		"js_functions":         []string{"fetchStorylines", "fetchPendingThreads", "buildContinuityPackQuery", "buildContinuityPackWakeUpBlock"},
		"policy_version":       "s16.8-js.v1",
		"mode":                 "archive_center_js_continuity_rescue_owner_surface",
	}
}

// buildStep168JSPromptAssemblyGuard defines the JS prompt assembly guard surface for
// SEQ-16.8-P133: Archive Center.js prompt assembly guard.
func buildStep168JSPromptAssemblyGuard(injectionAssembly prepareTurnInjectionAssembly) map[string]any {
	return map[string]any{
		"version":                "seq16_8_p133.v1",
		"role":                   "js_prompt_assembly_guard",
		"truth_authority":        false,
		"js_owner":               "archive_center_js",
		"js_functions":           []string{"assembleInjectionWithBudget", "applyContextInjection"},
		"injection_text_present": strings.TrimSpace(injectionAssembly.Text) != "",
		"policy_version":         "s16.8-js.v1",
		"mode":                   "archive_center_js_prompt_assembly_guard",
	}
}

// buildStep168JSTracePreviewTransparency defines the JS trace/preview/transparency surface extend for
// SEQ-16.8-P134: Archive Center.js trace/preview/transparency surface extend.
func buildStep168JSTracePreviewTransparency(inputContextText string, injectionAssembly prepareTurnInjectionAssembly) map[string]any {
	return map[string]any{
		"version":               "seq16_8_p134.v1",
		"role":                  "js_trace_preview_transparency",
		"truth_authority":       false,
		"js_owner":              "archive_center_js",
		"trace_preview_ready":   true,
		"input_context_preview": truncateRunes(inputContextText, 200),
		"injection_preview":     truncateRunes(injectionAssembly.Text, 200),
		"policy_version":        "s16.8-js.v1",
		"mode":                  "archive_center_js_trace_preview_transparency_surface_extend",
	}
}

// buildStep168ReplayCorpusBaseline defines the replay corpus/inspection baseline add surface for
// SEQ-16.8-P135: replay corpus/inspection baseline add.
func buildStep168ReplayCorpusBaseline(inputAnchorGovernor map[string]any) map[string]any {
	oldArcTrace := []map[string]any{}
	if raw, ok := inputAnchorGovernor["old_arc_keep_drop_trace"].([]map[string]any); ok {
		oldArcTrace = raw
	}
	corpus := []map[string]any{}
	for _, arc := range oldArcTrace {
		corpus = append(corpus, map[string]any{
			"name":     arc["name"],
			"status":   arc["status"],
			"decision": arc["decision"],
			"reason":   arc["reason"],
		})
	}
	return map[string]any{
		"version":              "seq16_8_p135.v1",
		"role":                 "replay_corpus_baseline",
		"truth_authority":      false,
		"corpus_entries":       corpus,
		"corpus_count":         len(corpus),
		"cases":                []string{"stale_arc_revival", "foreground_hijack", "scene_monopoly"},
		"baseline_source":      "seq16_8_replay_corpus",
		"redefines_step_17_1f": false,
		"carry_in_baseline_for_step_17_evaluation": true,
		"policy_version": "s16.8-rc.v1",
		"mode":           "replay_corpus_inspection_baseline_add",
	}
}

// buildStep168BackendMetadataAlignment defines the backend/main.py storyline/pending-thread read metadata alignment surface for
// SEQ-16.8-P136: backend/main.py storyline/pending-thread read metadata alignment - suppression trace confirm.
func buildStep168BackendMetadataAlignment(storylines []store.Storyline, pendingThreads []store.PendingThread) map[string]any {
	return map[string]any{
		"version":                     "seq16_8_p136.v1",
		"role":                        "backend_metadata_alignment",
		"truth_authority":             false,
		"storyline_count":             len(storylines),
		"pending_thread_count":        len(pendingThreads),
		"metadata_aligned":            true,
		"suppression_trace_confirmed": true,
		"policy_version":              "s16.8-be.v1",
		"mode":                        "backend_storyline_pending_thread_read_metadata_alignment",
	}
}
