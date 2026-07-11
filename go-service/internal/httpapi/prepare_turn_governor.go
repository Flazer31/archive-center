package httpapi

import (
	"fmt"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func buildInputAnchorGovernor(rawUserInput, inputContextText string, inputContextTruncated bool, maxChars int, chatLogs []store.ChatLog, resumePack *store.ResumePack, activeStates []store.ActiveState, canonicalLayers []store.CanonicalStateLayer, episodeSums []store.EpisodeSummary, pendingThreads []store.PendingThread, storylines []store.Storyline) map[string]any {
	type slotDef struct {
		name           string
		marker         string
		section        string
		source         string
		selected       bool
		selectedReason string
		droppedReason  string
		mandatory      bool
	}

	hasResume := resumePack != nil && strings.TrimSpace(resumePack.AssembledText) != ""
	hasScene := false
	hasEntity := false
	for _, as := range activeStates {
		switch strings.ToLower(strings.TrimSpace(as.StateType)) {
		case "scene":
			hasScene = true
		case "entity", "character", "npc":
			hasEntity = true
		}
	}
	for _, cl := range canonicalLayers {
		layerType := strings.ToLower(strings.TrimSpace(cl.LayerType))
		if strings.Contains(layerType, "scene") || strings.Contains(layerType, "world") {
			hasScene = true
		}
		if strings.Contains(layerType, "entity") || strings.Contains(layerType, "character") {
			hasEntity = true
		}
	}
	hasActiveThread := len(pendingThreads) > 0
	hasChapter := len(episodeSums) > 0
	hasSaga := len(storylines) > 0

	slots := []slotDef{
		{name: "Temporal Anchor", marker: "[Temporal Anchor]", section: "[Recent Chat]", source: "chat_logs", selected: len(chatLogs) > 0, selectedReason: "recent_chat_available", droppedReason: "no_recent_chat", mandatory: true},
		{name: "Previous", marker: "[Previous]", section: "[Resume Pack]", source: "resume_pack", selected: hasResume, selectedReason: "resume_pack_available", droppedReason: "no_resume_pack", mandatory: true},
		{name: "Scene", marker: "[Scene]", section: "[Active States]", source: "active_states_or_canonical_layers", selected: hasScene, selectedReason: "scene_anchor_available", droppedReason: "no_scene_anchor", mandatory: false},
		{name: "Entity", marker: "[Entity]", section: "[Active States]", source: "active_states_or_canonical_layers", selected: hasEntity, selectedReason: "entity_anchor_available", droppedReason: "no_entity_anchor", mandatory: false},
		{name: "Active Thread", marker: "[Active Thread]", section: "[Progression Ledger]", source: "pending_threads", selected: hasActiveThread, selectedReason: "active_thread_available", droppedReason: "no_active_thread", mandatory: false},
		{name: "Chapter", marker: "[Chapter]", section: "[Episode Summaries]", source: "episode_summaries", selected: hasChapter, selectedReason: "chapter_anchor_available", droppedReason: "no_chapter_anchor", mandatory: false},
		{name: "Saga", marker: "[Saga]", section: "[Progression Ledger]", source: "storylines", selected: hasSaga, selectedReason: "saga_anchor_available", droppedReason: "no_saga_anchor", mandatory: false},
	}

	mandatorySlots := make([]map[string]any, 0, 2)
	optionalSlots := make([]map[string]any, 0, 5)
	selectedTrace := []map[string]any{}
	droppedTrace := []map[string]any{}
	selectedNames := []string{}
	droppedNames := []string{}
	for _, slot := range slots {
		entry := map[string]any{
			"name":           slot.name,
			"marker":         slot.marker,
			"mapped_section": slot.section,
			"source":         slot.source,
			"selected":       slot.selected,
			"mandatory":      slot.mandatory,
		}
		if slot.selected {
			entry["reason"] = slot.selectedReason
			selectedNames = append(selectedNames, slot.name)
			selectedTrace = append(selectedTrace, map[string]any{
				"slot":           slot.name,
				"marker":         slot.marker,
				"mapped_section": slot.section,
				"source":         slot.source,
				"reason":         slot.selectedReason,
			})
		} else {
			entry["reason"] = slot.droppedReason
			droppedNames = append(droppedNames, slot.name)
			droppedTrace = append(droppedTrace, map[string]any{
				"slot":           slot.name,
				"marker":         slot.marker,
				"mapped_section": slot.section,
				"source":         slot.source,
				"reason":         slot.droppedReason,
			})
		}
		if slot.mandatory {
			mandatorySlots = append(mandatorySlots, entry)
		} else {
			optionalSlots = append(optionalSlots, entry)
		}
	}

	oldArcTrace := []map[string]any{}
	for _, storyline := range storylines {
		name := strings.TrimSpace(storyline.Name)
		if name == "" {
			name = fmt.Sprintf("storyline:%d", storyline.ID)
		}
		status := strings.ToLower(strings.TrimSpace(storyline.Status))
		if status == "" {
			status = "active"
		}
		decision := "keep"
		reason := "active_arc_anchor"
		if status == "resolved" || status == "dormant" || status == "inactive" {
			decision = "drop"
			reason = "stale_or_resolved_arc_demoted"
		}
		oldArcTrace = append(oldArcTrace, map[string]any{
			"name":        name,
			"status":      status,
			"last_turn":   storyline.LastTurn,
			"decision":    decision,
			"reason":      reason,
			"anchor_slot": "Active Thread",
		})
	}

	status := "empty"
	if strings.TrimSpace(inputContextText) != "" {
		status = "ready"
	}
	lowerInput := strings.ToLower(strings.TrimSpace(rawUserInput))
	explicitRedirection := strings.Contains(lowerInput, "move on") || strings.Contains(lowerInput, "go left") || strings.Contains(lowerInput, "go right") || strings.Contains(lowerInput, "instead")

	return map[string]any{
		"version":                 "seq16_5_input_anchor_governor.v1",
		"status":                  status,
		"role":                    "support_anchor_lane_only",
		"truth_authority":         false,
		"mandatory_slots":         mandatorySlots,
		"optional_slots":          optionalSlots,
		"selected_anchor_trace":   selectedTrace,
		"dropped_anchor_trace":    droppedTrace,
		"selected_slot_names":     selectedNames,
		"dropped_slot_names":      droppedNames,
		"old_arc_keep_drop_trace": oldArcTrace,
		"slot_policy": map[string]any{
			"max_slots":                            len(slots),
			"max_chars":                            maxChars,
			"input_context_truncated":              inputContextTruncated,
			"short_and_sharp_anchor_lane_preserve": true,
		},
		"promotion_demotion_rules": map[string]any{
			"weak_input":          "prefer_recent_temporal_and_previous_without_truth_promotion",
			"temporal_query":      "promote_temporal_anchor_then_previous",
			"resume":              "promote_previous_and_chapter_when_available",
			"explicit_user_input": "demote_stale_arc_and_preserve_current_user_direction",
		},
		"helper_injection_anchor_suppression": map[string]any{
			"enabled": true,
			"reason":  "helper_injection_must_not_duplicate_input_anchor_slots",
			"suppressed_markers": []string{
				"[Temporal Anchor]", "[Previous]", "[Scene]", "[Entity]", "[Active Thread]", "[Chapter]", "[Saga]",
				"[Resume Pack]", "[Direct Evidence]", "[Recent Chat]", "[Active States]", "[Canonical State Layers]", "[Episode Summaries]",
			},
		},
		"explicit_user_redirection": map[string]any{
			"detected":                  explicitRedirection,
			"stale_arc_demotes":         true,
			"current_user_input_wins":   true,
			"support_lane_may_suggest":  true,
			"support_lane_may_redirect": false,
		},
		"support_lane_wording_guard": map[string]any{
			"display_label":                   "support/anchor lane only",
			"truth_lane_label_forbidden":      true,
			"canonical_truth_wording_allowed": false,
			"disallowed_usage":                []string{"truth_overwrite", "canonical_override", "authority_reorder", "direct_execution"},
		},
	}
}

func buildHelperBudgetGovernorTrace(assembly prepareTurnInjectionAssembly, maxInjectionChars int) map[string]any {
	reasonCounts := map[string]int{}
	if assembly.BudgetDecisions != nil {
		if raw, ok := assembly.BudgetDecisions["reason_counts"].(map[string]int); ok {
			for k, v := range raw {
				reasonCounts[k] = v
			}
		}
	}
	if len(reasonCounts) == 0 {
		reasonCounts["tier_cap"] = 0
	}

	laneBreakdown := make([]map[string]any, 0, len(assembly.Blocks))
	for _, block := range assembly.Blocks {
		laneBreakdown = append(laneBreakdown, map[string]any{
			"label":           block.Label,
			"source":          block.Source,
			"count":           block.Count,
			"budget":          block.Budget,
			"chars":           len([]rune(block.Text)),
			"selected":        strings.TrimSpace(block.Text) != "",
			"support_lane":    true,
			"truth_authority": false,
		})
	}

	return map[string]any{
		"version":              "seq16_5_helper_budget_trace.v1",
		"role":                 "support_lane_only",
		"truth_authority":      false,
		"max_injection_chars":  maxInjectionChars,
		"reason_counts":        reasonCounts,
		"lane_breakdown":       laneBreakdown,
		"need_breakdown":       map[string]any{"memory": assembly.Counts["memories"], "kg": assembly.Counts["kg"], "evidence": assembly.Counts["evidence"]},
		"risk_breakdown":       map[string]any{"truth_overwrite": "blocked", "duplicate_anchor": "suppressed", "over_budget": reasonCounts["tier_cap"]},
		"budget_decision_mode": "turn_local_shadow_trace",
		"support_lane_wording_guard": map[string]any{
			"display_label":              "support lane only",
			"truth_lane_label_forbidden": true,
			"disallowed_usage":           []string{"truth_overwrite", "canonical_override", "direct_execution"},
		},
	}
}
