package httpapi

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

const prepareTurnMemoryDeliveryPlanVersion = "memory_delivery_plan.v1"

var prepareTurnMemoryDeliveryOrder = []string{
	"direct_evidence",
	"protected_secret",
	"event_recent",
	"character_objective",
	"subjective_relationship",
	"world_state",
	"unresolved_goal",
}

var prepareTurnMemoryDeliveryTitles = map[string]string{
	"direct_evidence":         "Latest Direct Evidence",
	"protected_secret":        "Protected Memory Guidance",
	"event_recent":            "Event and Recent Memories",
	"character_objective":     "Character Objective States",
	"subjective_relationship": "Subjective Memories and Relationships",
	"world_state":             "Item, Location, and World States",
	"unresolved_goal":         "Unresolved Goals",
}

func prepareTurnResolveMemoryBudgets(perspective map[string]any) (string, map[string]int) {
	mode := strings.ToLower(strings.TrimSpace(extractionStringFromAny(perspective["_memory_delivery_budget_mode"])))
	if mode != "custom" {
		return "auto", nil
	}
	custom, ok := perspective["_memory_delivery_budgets"].(map[string]int)
	if !ok {
		return "auto", nil
	}
	budgets := map[string]int{}
	for _, key := range prepareTurnMemoryDeliveryOrder {
		value := custom[key]
		if value < 0 {
			value = 0
		}
		budgets[key] = value
	}
	return "custom", budgets
}

func prepareTurnDeliveryItems(texts ...string) []string {
	items := []string{}
	seen := map[string]bool{}
	for _, text := range texts {
		lines := strings.Split(strings.TrimSpace(text), "\n")
		for index, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || (index == 0 && strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]")) {
				continue
			}
			key := collapseTextKey(line)
			if key != "" && seen[key] {
				continue
			}
			if key != "" {
				seen[key] = true
			}
			items = append(items, line)
		}
	}
	return items
}

func prepareTurnDeliveryFactKey(line string) string {
	line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
	for strings.HasPrefix(line, "[") {
		end := strings.Index(line, "]")
		if end < 0 || end+1 >= len(line) {
			break
		}
		line = strings.TrimSpace(line[end+1:])
	}
	return collapseTextKey(line)
}

func buildPrepareTurnMemoryDeliveryPlan(out *prepareTurnInjectionAssembly, maxChars int, perspective map[string]any) map[string]any {
	mode, budgets := prepareTurnResolveMemoryBudgets(perspective)
	coreObjectiveLimitPresent := boolFromAny(perspective["_core_objective_memory_max_items_present"])
	coreObjectiveLimit := intFromAny(perspective["_core_objective_memory_max_items"], 0)
	if coreObjectiveLimitPresent && coreObjectiveLimit < 1 {
		coreObjectiveLimit = 1
	}
	// Section headers are measured by appendWithin together with their content,
	// so no fixed reserve or context-size tier is needed outside the observed
	// final injection budget.
	deliveryCap := maxInt(maxChars, 0)
	configuredBudgets := map[string]int{}
	if mode == "custom" {
		budgetTotal := 0
		for _, key := range prepareTurnMemoryDeliveryOrder {
			configuredBudgets[key] = budgets[key]
			budgetTotal += budgets[key]
		}
		if budgetTotal > deliveryCap && budgetTotal > 0 {
			for _, key := range prepareTurnMemoryDeliveryOrder {
				budgets[key] = budgets[key] * deliveryCap / budgetTotal
			}
		}
	}
	coreObjectiveItems := prepareTurnDeliveryItems(out.ActualMemoryText)
	coreObjectiveFactKeys := map[string]bool{}
	coreObjectiveDeduplicatedFactKeys := map[string]bool{}
	for _, item := range coreObjectiveItems {
		if key := prepareTurnDeliveryFactKey(item); key != "" {
			coreObjectiveFactKeys[key] = true
		}
	}
	coreObjectiveSelectedCount := 0
	coreObjectiveDeduplicatedCount := 0
	coreObjectiveDeferredByLimit := []string{}
	eventSupportItems := prepareTurnDeliveryItems(out.EpisodeText, out.ChapterText, out.ArcText, out.SagaText, out.CanonEventText)
	eventRecentItems := prepareTurnDeliveryItems(strings.Join(append(append([]string{}, coreObjectiveItems...), eventSupportItems...), "\n"))
	requiredItems := map[string][]string{
		"direct_evidence":         prepareTurnDeliveryItems(out.LatestDirectEvidenceText, out.ContinuityCorrectionText),
		"protected_secret":        prepareTurnDeliveryItems(out.ProtectedMemoryText),
		"event_recent":            coreObjectiveItems,
		"character_objective":     prepareTurnDeliveryItems(out.CharacterObjectiveText, out.CanonCharacterText),
		"subjective_relationship": prepareTurnDeliveryItems(out.CharacterPrivateText, out.CharacterRelationshipText, out.CanonRelationshipText),
		"world_state":             prepareTurnDeliveryItems(out.CanonWorldText, out.WorldRulesText),
		"unresolved_goal":         prepareTurnDeliveryItems(out.PendingThreadText),
	}
	auxiliaryItems := map[string][]string{
		"direct_evidence":         prepareTurnDeliveryItems(out.DirectEvidenceText, out.ScopedVerbatimText),
		"protected_secret":        nil,
		"event_recent":            eventSupportItems,
		"character_objective":     nil,
		"subjective_relationship": prepareTurnDeliveryItems(out.PersonaText, out.KGText),
		"world_state":             nil,
		"unresolved_goal":         prepareTurnDeliveryItems(out.StorylineText),
	}
	items := map[string][]string{
		// Raw chat fallback remains available as a diagnostic/search result, but it
		// is not an authoritative memory source. The previous logical turn is
		// already delivered once through Input Context.
		"event_recent":            eventRecentItems,
		"character_objective":     prepareTurnDeliveryItems(out.CharacterObjectiveText, out.CanonCharacterText),
		"subjective_relationship": prepareTurnDeliveryItems(out.CharacterPrivateText, out.CharacterRelationshipText, out.PersonaText, out.CanonRelationshipText, out.KGText),
		"world_state":             prepareTurnDeliveryItems(out.CanonWorldText, out.WorldRulesText),
		"protected_secret":        prepareTurnDeliveryItems(out.ProtectedMemoryText),
		"unresolved_goal":         prepareTurnDeliveryItems(out.StorylineText, out.PendingThreadText),
		// The immediately previous logical turn is already owned by Input Context.
		// Do not copy chat-log text into the authoritative evidence lane, where an
		// old user instruction could regain current-request authority.
		"direct_evidence": prepareTurnDeliveryItems(out.LatestDirectEvidenceText, out.DirectEvidenceText, out.ScopedVerbatimText, out.ContinuityCorrectionText),
	}
	selected := map[string][]string{}
	remaining := map[string][]string{}
	deduplicated := map[string]int{}
	borrowedChars := map[string]int{}
	requiredSelected := map[string]int{}
	auxiliarySelected := map[string]int{}
	seenFacts := map[string]bool{}
	seenClassItems := map[string]map[string]bool{}
	usedGlobal := 0
	appendWithin := func(key string, candidates []string, cap int, tier string) []string {
		deferred := []string{}
		for _, item := range candidates {
			itemKey := collapseTextKey(item)
			if seenClassItems[key] != nil && itemKey != "" && seenClassItems[key][itemKey] {
				deduplicated[key]++
				continue
			}
			factKey := ""
			isCoreObjective := false
			if key != "protected_secret" && key != "subjective_relationship" {
				factKey = prepareTurnDeliveryFactKey(item)
				isCoreObjective = key == "event_recent" && coreObjectiveFactKeys[factKey]
				if factKey != "" && seenFacts[factKey] {
					deduplicated[key]++
					if isCoreObjective && !coreObjectiveDeduplicatedFactKeys[factKey] {
						coreObjectiveDeduplicatedFactKeys[factKey] = true
						coreObjectiveDeduplicatedCount++
					}
					continue
				}
				if isCoreObjective && coreObjectiveLimitPresent && coreObjectiveSelectedCount >= coreObjectiveLimit {
					coreObjectiveDeferredByLimit = append(coreObjectiveDeferredByLimit, item)
					deferred = append(deferred, item)
					continue
				}
			}
			candidate := append(append([]string{}, selected[key]...), item)
			text := makePrepareTurnSection("["+prepareTurnMemoryDeliveryTitles[key]+"]", candidate)
			oldText := makePrepareTurnSection("["+prepareTurnMemoryDeliveryTitles[key]+"]", selected[key])
			delta := len([]rune(text)) - len([]rune(oldText))
			if len([]rune(text)) <= cap && usedGlobal+delta <= deliveryCap {
				selected[key] = candidate
				usedGlobal += delta
				if seenClassItems[key] == nil {
					seenClassItems[key] = map[string]bool{}
				}
				if itemKey != "" {
					seenClassItems[key][itemKey] = true
				}
				if factKey != "" {
					seenFacts[factKey] = true
				}
				if isCoreObjective {
					coreObjectiveSelectedCount++
				}
				switch tier {
				case "required":
					requiredSelected[key]++
				case "auxiliary":
					auxiliarySelected[key]++
				}
			} else {
				deferred = append(deferred, item)
			}
		}
		return deferred
	}
	if mode == "custom" {
		for _, key := range prepareTurnMemoryDeliveryOrder {
			classCap := budgets[key]
			if classCap <= 0 {
				classCap = deliveryCap
			}
			remaining[key] = appendWithin(key, items[key], classCap, "custom")
		}
		// Custom reservations are explicit user policy. Current-character lanes may
		// still use otherwise idle global space after every configured class had a pass.
		for _, key := range []string{"character_objective", "subjective_relationship"} {
			before := usedGlobal
			remaining[key] = appendWithin(key, remaining[key], deliveryCap, "custom")
			borrowedChars[key] = usedGlobal - before
		}
	} else {
		// Automatic delivery has no per-class numeric quotas. Existing relevance,
		// perspective and privacy gates define eligibility; the only numeric bound
		// is the final host envelope. Deliver current-error-prevention material
		// across all classes before auxiliary continuity and expression support.
		for _, key := range prepareTurnMemoryDeliveryOrder {
			remaining[key] = appendWithin(key, requiredItems[key], deliveryCap, "required")
		}
		for _, key := range prepareTurnMemoryDeliveryOrder {
			remaining[key] = append(remaining[key], appendWithin(key, auxiliaryItems[key], deliveryCap, "auxiliary")...)
		}
	}
	classes := []map[string]any{}
	parts := []string{}
	for _, key := range prepareTurnMemoryDeliveryOrder {
		text := makePrepareTurnSection("["+prepareTurnMemoryDeliveryTitles[key]+"]", selected[key])
		usedChars := len([]rune(text))
		if text != "" {
			parts = append(parts, text)
		}
		classTrace := map[string]any{
			"key": key, "title": prepareTurnMemoryDeliveryTitles[key],
			"used_chars": usedChars, "eligible_count": len(items[key]), "selected_count": len(selected[key]),
			"deduplicated_count": deduplicated[key], "deferred_count": len(remaining[key]), "text": nilIfEmpty(text),
		}
		if mode == "custom" {
			classTrace["selection_policy"] = "explicit_custom_class_reservation"
			classTrace["required_eligible_count"] = nil
			classTrace["required_selected_count"] = nil
			classTrace["auxiliary_eligible_count"] = nil
			classTrace["auxiliary_selected_count"] = nil
			classTrace["reserved_chars"] = budgets[key]
			classTrace["configured_reserved_chars"] = configuredBudgets[key]
			classTrace["borrowed_chars"] = borrowedChars[key]
			classTrace["unused_chars"] = maxInt(budgets[key]-usedChars+borrowedChars[key], 0)
		} else {
			classTrace["selection_policy"] = "required_then_auxiliary_global_envelope"
			classTrace["required_eligible_count"] = len(requiredItems[key])
			classTrace["required_selected_count"] = requiredSelected[key]
			classTrace["auxiliary_eligible_count"] = len(auxiliaryItems[key])
			classTrace["auxiliary_selected_count"] = auxiliarySelected[key]
			classTrace["reserved_chars"] = nil
			classTrace["configured_reserved_chars"] = nil
			classTrace["borrowed_chars"] = 0
			classTrace["unused_chars"] = nil
		}
		classes = append(classes, classTrace)
	}
	finalText := strings.Join(parts, "\n\n")
	finalHash := fmt.Sprintf("%x", sha256.Sum256([]byte(finalText)))
	directEntities := stringsFromAny(out.Counts["directly_referenced_entities"])
	directMemoryFactKeys := map[string]bool{}
	for _, item := range prepareTurnDeliveryItems(out.ActualMemoryText, out.CharacterPrivateText) {
		if key := prepareTurnDeliveryFactKey(item); key != "" {
			directMemoryFactKeys[key] = true
		}
	}
	deliveredDirectMemoryLines := []string{}
	for _, key := range []string{"event_recent", "subjective_relationship"} {
		for _, item := range selected[key] {
			if directMemoryFactKeys[prepareTurnDeliveryFactKey(item)] {
				deliveredDirectMemoryLines = append(deliveredDirectMemoryLines, item)
			}
		}
	}
	directMemoryText := strings.Join(deliveredDirectMemoryLines, "\n")
	deliveredDirectEntities := 0
	for _, entity := range directEntities {
		if prepareTurnRecallContainsAnchor(directMemoryText, entity) {
			deliveredDirectEntities++
		}
	}
	coreObjectiveDeliveredCount := 0
	for _, item := range selected["event_recent"] {
		if coreObjectiveFactKeys[prepareTurnDeliveryFactKey(item)] {
			coreObjectiveDeliveredCount++
		}
	}
	coreObjectiveEligibleCount := maxInt(len(coreObjectiveItems)-coreObjectiveDeduplicatedCount, 0)
	coreObjectiveCandidateCount := maxInt(coreObjectiveEligibleCount-len(coreObjectiveDeferredByLimit), 0)
	coreObjectiveDeferredByBudget := maxInt(coreObjectiveCandidateCount-coreObjectiveDeliveredCount, 0)
	coreObjectiveGapReason := "legacy_item_limit_absent"
	switch {
	case !coreObjectiveLimitPresent:
		coreObjectiveGapReason = "legacy_item_limit_absent"
	case coreObjectiveEligibleCount == 0:
		coreObjectiveGapReason = "no_relevant_objective_memory"
	case coreObjectiveDeferredByBudget > 0:
		coreObjectiveGapReason = "delivery_char_budget"
	case coreObjectiveEligibleCount < coreObjectiveLimit:
		coreObjectiveGapReason = "fewer_relevant_items_than_limit"
	default:
		coreObjectiveGapReason = "limit_satisfied"
	}
	coreObjectiveDeferredFactKeys := []string{}
	for _, item := range coreObjectiveDeferredByLimit {
		if key := prepareTurnDeliveryFactKey(item); key != "" {
			coreObjectiveDeferredFactKeys = append(coreObjectiveDeferredFactKeys, key)
		}
	}
	coreObjectiveContract := map[string]any{
		"contract_version":         "core_objective_memory_delivery.v1",
		"status":                   map[bool]string{true: "active", false: "legacy_absent"}[coreObjectiveLimitPresent],
		"requested_max_items":      nil,
		"eligible_distinct_count":  coreObjectiveEligibleCount,
		"candidate_count":          coreObjectiveCandidateCount,
		"delivered_count":          coreObjectiveDeliveredCount,
		"deferred_by_limit_count":  len(coreObjectiveDeferredByLimit),
		"deferred_by_budget_count": coreObjectiveDeferredByBudget,
		"missing_to_limit":         0,
		"gap_reason":               coreObjectiveGapReason,
		"garbage_fill":             false,
		"top_k_reinterpreted":      false,
		"counted_lane":             "objective_event_summary_only",
		"item_count_exempt_lanes": []string{
			"direct_evidence",
			"protected_secret",
			"subjective_relationship",
			"character_objective",
			"world_state",
			"unresolved_goal",
			"hierarchy_support",
		},
		"all_lanes_remain_char_budgeted": true,
		"automatic_class_quotas":         false,
		"deferred_fact_keys":             coreObjectiveDeferredFactKeys,
	}
	if coreObjectiveLimitPresent {
		coreObjectiveContract["requested_max_items"] = coreObjectiveLimit
		coreObjectiveContract["missing_to_limit"] = maxInt(coreObjectiveLimit-coreObjectiveDeliveredCount, 0)
	}
	borrowingPolicy := "required_then_auxiliary_global_envelope"
	if mode == "custom" {
		borrowingPolicy = "explicit_custom_class_reservations_then_current_character_idle_space"
	}
	return map[string]any{
		"contract_version": prepareTurnMemoryDeliveryPlanVersion, "status": "ready", "mode": mode,
		"final_budget_owner": "go_memory_delivery_plan", "global_cap_chars": maxChars,
		"delivery_cap_chars": deliveryCap, "host_envelope_reserved_chars": 0,
		"used_chars": len([]rune(finalText)), "order": prepareTurnMemoryDeliveryOrder,
		"automatic_class_quotas":               false,
		"automatic_selection_order":            []string{"required", "auxiliary"},
		"required_selection_basis":             "current_error_prevention_after_relevance_perspective_privacy_and_active_source_gates",
		"auxiliary_selection_basis":            "continuity_and_expression_support_after_required_delivery",
		"final_text_sha256":                    finalHash,
		"direct_entity_memory_requested_count": len(directEntities),
		"direct_entity_memory_delivered_count": deliveredDirectEntities,
		"direct_entity_memory_gap":             maxInt(len(directEntities)-deliveredDirectEntities, 0),
		"borrowing_policy":                     borrowingPolicy, "classes": classes, "final_text": nilIfEmpty(finalText),
		"core_objective_memory":            coreObjectiveContract,
		"historical_chat_authority_policy": "previous_logical_turn_owned_by_input_context_not_direct_evidence",
		"recent_raw_turn_delivery":         "excluded_from_final_memory_delivery",
		"raw_chat_fallback_delivery":       "diagnostic_only_excluded_from_final_memory_delivery",
	}
}

func finalizePrepareTurnMemoryDeliveryLineage(lineage, plan map[string]any) map[string]any {
	if len(lineage) == 0 || len(plan) == 0 {
		return lineage
	}
	deliveredByClass := map[string]map[string]bool{}
	for _, rawClass := range prepareTurnMemoryLineageSlice(plan["classes"]) {
		class := mapFromAny(rawClass)
		key := extractionStringFromAny(class["key"])
		if key == "" {
			continue
		}
		deliveredByClass[key] = map[string]bool{}
		for _, item := range prepareTurnDeliveryItems(extractionStringFromAny(class["text"])) {
			if factKey := prepareTurnDeliveryFactKey(item); factKey != "" {
				deliveredByClass[key][factKey] = true
			}
		}
	}
	coreContract := mapFromAny(plan["core_objective_memory"])
	deferredByCoreLimit := map[string]bool{}
	for _, key := range stringSliceFromAny(coreContract["deferred_fact_keys"]) {
		if key = strings.TrimSpace(key); key != "" {
			deferredByCoreLimit[key] = true
		}
	}

	items := prepareTurnMemoryLineageSlice(lineage["items"])
	deliveredActual := 0
	deliveredProtected := 0
	for _, rawItem := range items {
		item := mapFromAny(rawItem)
		if !boolFromAny(item["delivered"]) {
			continue
		}
		classKey := "event_recent"
		if boolFromAny(item["protected_guard"]) {
			classKey = "protected_secret"
		}
		factKey := prepareTurnDeliveryFactKey(extractionStringFromAny(item["final_text"]))
		if factKey != "" && deliveredByClass[classKey][factKey] {
			if classKey == "protected_secret" {
				deliveredProtected++
			} else {
				deliveredActual++
				if coreContract["requested_max_items"] != nil {
					item["core_objective_k_consumption"] = "counted"
				} else {
					item["core_objective_k_consumption"] = "legacy_limit_absent"
				}
			}
			item["delivery_status"] = "delivered_final"
			item["reason_code"] = "selected_within_final_delivery_plan"
			continue
		}
		item["delivered"] = false
		if deferredByCoreLimit[factKey] {
			item["delivery_status"] = "deferred_core_objective_limit"
			item["reason_code"] = "core_objective_memory_max_items"
			item["core_objective_k_consumption"] = "deferred_by_core_objective_limit"
		} else {
			item["delivery_status"] = "deferred_final_char_budget"
			item["reason_code"] = "memory_delivery_char_budget"
			if !boolFromAny(item["protected_guard"]) {
				item["core_objective_k_consumption"] = "deferred_by_char_budget"
			}
		}
	}
	lineage["items"] = items
	lineage["final_delivered_count"] = deliveredActual + deliveredProtected
	lineage["final_actual_memory_count"] = deliveredActual
	lineage["final_protected_guard_count"] = deliveredProtected
	lineage["core_objective_memory"] = coreContract
	issueCodes := []string{}
	switch {
	case deliveredActual == 0 && deliveredProtected == 0:
		lineage["status"] = "empty"
	case deliveredActual == 0:
		lineage["status"] = "protected_guard_only"
		issueCodes = append(issueCodes, "actual_memory_absent")
	case deliveredProtected == 0:
		lineage["status"] = "actual_memory_only"
	default:
		lineage["status"] = "mixed"
	}
	lineage["known_issue_codes"] = issueCodes
	return lineage
}
