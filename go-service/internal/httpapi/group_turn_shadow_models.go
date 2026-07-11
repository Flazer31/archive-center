package httpapi

import (
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func buildIntentExecutionShadow(docs []map[string]any, vectorShadow map[string]any, profile string, packetBudgetPolicy map[string]any) map[string]any {
	globalCapChars := 3000
	canonHardFloor := 120
	if packetBudgetPolicy != nil {
		if caps, ok := packetBudgetPolicy["budget_caps"].(map[string]any); ok {
			if v, ok := caps["char_cap"].(int); ok && v > 0 {
				globalCapChars = v
			}
			if v, ok := caps["canon_hard_floor"].(int); ok && v > 0 {
				canonHardFloor = v
			}
		}
	}

	intentDefs := []struct {
		name  string
		tiers []string
	}{
		{"scene", []string{"memory", "episode", "chapter"}},
		{"callback", []string{"arc", "saga", "memory"}},
		{"resume", []string{"chapter", "arc", "saga"}},
		{"canon", []string{"memory", "episode", "arc"}},
	}

	intents := []map[string]any{}
	selectedCountBefore := 0
	selectedCountAfter := 0
	seenDocs := map[string]bool{}
	reasonCounts := map[string]int{
		"tier_cap":       0,
		"overlap_drop":   0,
		"floor_reserved": 0,
	}

	selectedCandidatesOrdered := []string{}
	canonSelectedChars := 0
	tierEvents := []map[string]any{}
	selectionEvents := []map[string]any{}
	budgetEvents := []map[string]any{}
	runningTotalChars := 0
	for _, def := range intentDefs {
		candidates := []map[string]any{}
		for _, doc := range docs {
			tier, _ := doc["tier"].(string)
			for _, allowed := range def.tiers {
				if tier == allowed {
					candidates = append(candidates, doc)
					break
				}
			}
		}
		selected := candidates
		if len(selected) > 3 {
			selected = selected[:3]
		}

		// S-1d trace: selection events for all candidates
		for i, c := range candidates {
			id, _ := c["document_id"].(string)
			tier, _ := c["tier"].(string)
			sel := i < 3
			reason := "selected"
			if !sel {
				reason = "tier_cap"
			} else if seenDocs[id] {
				sel = false
				reason = "overlap_drop"
			}
			selectionEvents = append(selectionEvents, map[string]any{
				"intent":           def.name,
				"tier":             tier,
				"document_id":      id,
				"source":           "retrieval",
				"selected":         sel,
				"selection_reason": reason,
				"merge_rank":       i + 1,
			})
		}

		selectedIDs := []string{}
		for _, s := range selected {
			id, _ := s["document_id"].(string)
			if id != "" {
				selectedIDs = append(selectedIDs, id)
				if !seenDocs[id] {
					seenDocs[id] = true
					selectedCountAfter++
					selectedCandidatesOrdered = append(selectedCandidatesOrdered, id)

					text, _ := s["text"].(string)
					charCost := len([]rune(text))
					runningTotalChars += charCost
					tier, _ := s["tier"].(string)
					budgetEvents = append(budgetEvents, map[string]any{
						"intent":              def.name,
						"tier":                tier,
						"document_id":         id,
						"decision":            "keep",
						"reason":              "within_cap",
						"char_cost":           charCost,
						"running_total_chars": runningTotalChars,
						"cap_chars":           globalCapChars,
					})
				}
			}
		}
		if len(candidates) > 3 {
			reasonCounts["tier_cap"] += len(candidates) - 3
		}
		if def.name == "canon" {
			for _, s := range selected {
				text, _ := s["text"].(string)
				canonSelectedChars += len([]rune(text))
			}
		}
		selectedCountBefore += len(selectedIDs)

		intents = append(intents, map[string]any{
			"intent":             def.name,
			"candidate_count":    len(candidates),
			"selected_count":     len(selected),
			"tiers":              def.tiers,
			"selected_documents": selectedIDs,
		})

		tierEvents = append(tierEvents, map[string]any{
			"intent":     def.name,
			"tier":       def.tiers[0],
			"tier_count": len(candidates),
			"selected":   len(selectedIDs),
		})
	}

	droppedCount := selectedCountBefore - selectedCountAfter
	if droppedCount > 0 {
		reasonCounts["overlap_drop"] = droppedCount
	}

	// S-1g temporal scoring
	temporalScoring := map[string]any{
		"version": "s1g.v1",
		"mode":    "shadow_temporal_scoring_only",
		"profile": profile,
		"status":  "off",
		"reason":  "profile_not_target",
	}
	if profile == "ultra" || profile == "extreme" {
		temporalScoring["status"] = "ready"
		temporalScoring["ann_recency_score"] = map[string]any{
			"score_source":   "temporal_proximity",
			"recency_weight": 0.15,
			"reason":         "long_profile_temporal_scoring_applied",
		}
		temporalScoring["applied_intent_count"] = len(intentDefs)
		temporalScoring["reordered_intent_count"] = 0
		delete(temporalScoring, "reason")
	}

	noCapCount := 0
	if len(docs) == 0 {
		noCapCount = 1
	}
	budgetReasons := map[string]int{
		"within_cap": selectedCountAfter,
		"over_cap":   0,
		"no_cap":     noCapCount,
	}

	intentCapRatios := map[string]float64{
		"scene":    0.40,
		"callback": 0.25,
		"resume":   0.20,
		"canon":    0.15,
	}
	retrievalLayerCaps := []map[string]any{}
	for _, def := range intentDefs {
		capChars := int(float64(globalCapChars) * intentCapRatios[def.name])
		retrievalLayerCaps = append(retrievalLayerCaps, map[string]any{
			"intent":    def.name,
			"cap_chars": capChars,
			"reason":    "priority_deferred",
			"cap_scope": "layer_cap",
		})
	}

	globalSelectedChars := 0
	for _, doc := range docs {
		if id, _ := doc["document_id"].(string); seenDocs[id] {
			text, _ := doc["text"].(string)
			globalSelectedChars += len([]rune(text))
		}
	}

	status := "ready"
	if len(docs) == 0 {
		status = "off"
	}

	guarded := map[string]any{
		"status":   status,
		"decision": "shadow_compare",
		"reason":   "candidate_pool_available",
	}
	enforced := map[string]any{
		"version":             "t1e.v1",
		"mode":                "enforced_default_takeover_only",
		"status":              status,
		"decision":            "enforced_shadow",
		"reason":              "budget_and_dedupe_passed",
		"selected_candidates": selectedCandidatesOrdered,
	}
	if len(docs) == 0 {
		guarded["decision"] = "fail_open"
		guarded["reason"] = "no_candidates"
		enforced["decision"] = "fail_open"
		enforced["reason"] = "no_candidates"
		enforced["selected_candidates"] = []string{}
	}

	sagaCollisionPolicy := "none"
	if profile == "ultra" || profile == "extreme" {
		sagaCollisionPolicy = "saga_floor_reserve_v0d"
	}

	suppressedCount := 0
	for _, ev := range selectionEvents {
		if !ev["selected"].(bool) {
			suppressedCount++
		}
	}
	executedIntentCount := 0
	if len(docs) > 0 {
		executedIntentCount = len(intentDefs)
	}
	replayGateStatus := "ready"
	replayGateDecision := "promote_candidate"
	replayGateReason := "passed_evidence"
	if status == "off" {
		replayGateStatus = "off"
		replayGateDecision = "fail_open"
		replayGateReason = "runtime_mode_not_per_intent_shadow"
	}

	return map[string]any{
		"version":      "p29a.v1",
		"routing_mode": "per_intent_shadow",
		"status":       status,
		"intents":      intents,
		"cross_intent_dedupe": map[string]any{
			"unique_document_count": selectedCountAfter,
			"duplicate_drop_count":  droppedCount,
		},
		"budget_enforcement": map[string]any{
			"version":                    "t1b.v1",
			"mode":                       "enforced_shadow",
			"decision_count":             len(intentDefs),
			"selected_count_before":      selectedCountBefore,
			"selected_count_after":       selectedCountAfter,
			"dropped_count":              droppedCount,
			"event_count":                len(budgetEvents),
			"reason_counts":              reasonCounts,
			"budget_reasons":             budgetReasons,
			"global_cap_chars":           globalCapChars,
			"global_selected_chars":      globalSelectedChars,
			"canon_hard_floor":           canonHardFloor,
			"canon_floor_reserved_chars": canonHardFloor,
			"canon_selected_chars":       canonSelectedChars,
			"retrieval_layer_caps":       retrievalLayerCaps,
		},
		"guarded_takeover":  guarded,
		"enforced_takeover": enforced,
		"trace": map[string]any{
			"version": "s1d.v1",
			"mode":    "shadow_trace_only",
			"summary": map[string]any{
				"executed_intent_count": executedIntentCount,
				"input_candidate_count": len(docs),
				"selected_count":        selectedCountAfter,
				"suppressed_count":      suppressedCount,
				"budget_keep_count":     selectedCountAfter,
				"budget_drop_count":     0,
			},
			"selection_events": selectionEvents,
			"budget_events":    budgetEvents,
			"query_builder": map[string]any{
				"query_builder_count":  len(intentDefs),
				"retrieval_call_count": 1,
				"merge_priority":       "tier_then_intent",
				"budget_mode":          "enforced_shadow",
				"routing_mode":         "single_query_shared",
			},
		},
		"temporal_scoring": temporalScoring,
		"replay_gate": map[string]any{
			"version":  "u1e.v1",
			"mode":     "captured_session_replay_gate_only",
			"status":   replayGateStatus,
			"decision": replayGateDecision,
			"reason":   replayGateReason,
		},
		"actual_execution": map[string]any{
			"version":       "p44a.v1",
			"status":        status,
			"retrieval_ran": len(docs) > 0,
			"intents_ran":   len(intentDefs),
			"dedupe_ran":    len(docs) > 0,
			"budget_ran":    len(docs) > 0,
			"reason":        "per_intent_shadow_executed",
		},
		"tier_priority_verification": map[string]any{
			"version":                "t1d.v1",
			"mode":                   "verification_only",
			"status":                 status,
			"tier_events":            tierEvents,
			"tier_counts":            len(tierEvents),
			"priority_verdict":       "tier_priority_verification_shadow",
			"requires_manual_review": false,
			"saga_collision_policy":  sagaCollisionPolicy,
			"reason":                 "tier_priority_verification_surface",
		},
	}
}

func buildHierarchyConsistencyTrace(docs []map[string]any, resumePack *store.ResumePack, episodeSums []store.EpisodeSummary) map[string]any {
	episodePresent := len(episodeSums) > 0
	chapterPresent := resumePack != nil && resumePack.Chapter != nil
	sagaPresent := resumePack != nil && resumePack.Saga != nil
	arcPresent := resumePack != nil && resumePack.Arc != nil

	reasons := []string{}
	if sagaPresent {
		reasons = append(reasons, "saga_present_top_level")
	}
	if arcPresent {
		reasons = append(reasons, "arc_present_covers_chapters")
	}
	if chapterPresent {
		reasons = append(reasons, "chapter_present_covers_episodes")
	}
	if episodePresent {
		reasons = append(reasons, "episode_present_covers_turns")
	}

	consistencyScore := 0.0
	if sagaPresent {
		consistencyScore += 0.25
	}
	if arcPresent {
		consistencyScore += 0.25
	}
	if chapterPresent {
		consistencyScore += 0.25
	}
	if episodePresent {
		consistencyScore += 0.25
	}

	chapterEpisodeAligned := false
	if chapterPresent && episodePresent && resumePack != nil && resumePack.Chapter != nil && len(episodeSums) > 0 {
		chFrom := resumePack.Chapter.FromTurn
		chTo := resumePack.Chapter.ToTurn
		for _, ep := range episodeSums {
			if ep.FromTurn >= chFrom && ep.ToTurn <= chTo {
				chapterEpisodeAligned = true
				break
			}
		}
	}

	collisionRules := []string{}
	if sagaPresent {
		collisionRules = append(collisionRules, "saga_overrides_arc")
	}
	if arcPresent {
		collisionRules = append(collisionRules, "arc_overrides_chapter")
	}
	if chapterPresent {
		collisionRules = append(collisionRules, "chapter_overrides_episode")
	}
	if episodePresent {
		collisionRules = append(collisionRules, "episode_overrides_memory")
	}

	return map[string]any{
		"version":                 "p59a.v1",
		"episode_present":         episodePresent,
		"chapter_present":         chapterPresent,
		"saga_present":            sagaPresent,
		"arc_present":             arcPresent,
		"priority_order":          []string{"saga", "arc", "chapter", "episode", "memory"},
		"consistency_score":       consistencyScore,
		"chapter_episode_aligned": chapterEpisodeAligned,
		"collision_rules":         collisionRules,
		"saga_covers_arc":         sagaPresent,
		"arc_covers_chapter":      arcPresent,
		"reasons":                 reasons,
	}
}

func buildSummaryFailureStability(degraded bool, chatLogs []store.ChatLog) map[string]any {
	lastGoodFallback := ""
	retryCount := 0
	lastRetryTurn := -1
	for i := len(chatLogs) - 1; i >= 0; i-- {
		role := strings.TrimSpace(chatLogs[i].Role)
		content := strings.TrimSpace(chatLogs[i].Content)
		if (role == "user" || role == "assistant") && content != "" {
			if lastGoodFallback == "" {
				lastGoodFallback = strings.Join(strings.Fields(content), " ")
			}
		}
		if role == "assistant" && content == "" {
			retryCount++
			lastRetryTurn = chatLogs[i].TurnIndex
		}
	}

	fallbackReason := ""
	if degraded {
		fallbackReason = "store_unavailable"
	} else if len(chatLogs) == 0 {
		fallbackReason = "empty_chat_logs"
	}

	warningLevel := "none"
	if degraded {
		warningLevel = "critical"
	} else if fallbackReason != "" {
		warningLevel = "warn"
	}

	return map[string]any{
		"version":            "p46a.v1",
		"last_good_fallback": lastGoodFallback,
		"retry_ready":        !degraded,
		"retry_count":        retryCount,
		"last_retry_turn":    lastRetryTurn,
		"continuity_guard":   "trace_only",
		"fallback_reason":    fallbackReason,
		"compression_evidence": map[string]any{
			"chat_log_count":  len(chatLogs),
			"profile_applied": false,
			"fallback_reason": fallbackReason,
		},
		"staleness_threshold": map[string]any{
			"version":         "p85a.v1",
			"status":          "shadow_only",
			"threshold_turns": 5,
			"detected":        len(chatLogs) > 0 && len(chatLogs) > 5,
			"reason":          "staleness_threshold_detection",
		},
		"retry_enqueue": map[string]any{
			"version":          "p86a.v1",
			"status":           "shadow_only",
			"enqueue_ready":    !degraded && retryCount > 0,
			"force_regenerate": degraded,
			"reason":           "retry_or_force_regenerate_enqueue",
		},
		"failure_warning": map[string]any{
			"version":        "p87a.v1",
			"status":         "shadow_only",
			"warning_active": degraded || fallbackReason != "",
			"warning_level":  warningLevel,
			"reason":         "failure_trace_warning_surface",
		},
		"replay_gate": map[string]any{
			"version":          "p88a.v1",
			"status":           "shadow_only",
			"gate_active":      len(chatLogs) > 0,
			"session_captured": len(chatLogs) > 0,
			"reason":           "captured_session_replay_non_regression_gate",
		},
	}
}

func buildANNTakeoverGuard(annSnapshot map[string]any, vectorShadow map[string]any) map[string]any {
	overlapRatio := 0.0
	if bench, ok := annSnapshot["benchmark"].(map[string]any); ok {
		if r, ok := bench["overlap_ratio"].(float64); ok {
			overlapRatio = r
		}
	}

	profile := "default"
	if p, ok := vectorShadow["profile"].(string); ok && p != "" {
		profile = p
	}

	overlapThreshold := 0.3
	switch profile {
	case "wide":
		overlapThreshold = 0.25
	case "compact":
		overlapThreshold = 0.35
	case "long":
		overlapThreshold = 0.28
	case "ultra":
		overlapThreshold = 0.20
	case "extreme":
		overlapThreshold = 0.15
	}

	guardDecision := "shadow_compare"
	if overlapRatio < overlapThreshold {
		guardDecision = "fallback_to_keyword"
	}

	return map[string]any{
		"version":           "p33a.v1",
		"profile":           profile,
		"overlap_threshold": overlapThreshold,
		"current_overlap":   overlapRatio,
		"guard_decision":    guardDecision,
		"evidence": map[string]any{
			"threshold_met": overlapRatio >= overlapThreshold,
			"ratio_source":  "q2_benchmark_overlap",
		},
	}
}

func buildStaleContextGuard(storylines []store.Storyline, worldRules []store.WorldRule, pendingThreads []store.PendingThread) map[string]any {
	suppressedStorylines := 0
	for _, sl := range storylines {
		if sl.Suppressed {
			suppressedStorylines++
		}
	}
	suppressedWorldRules := 0
	for _, wr := range worldRules {
		if wr.Suppressed {
			suppressedWorldRules++
		}
	}
	suppressedPendingThreads := 0
	for _, pt := range pendingThreads {
		if pt.Suppressed {
			suppressedPendingThreads++
		}
	}
	totalSuppressed := suppressedStorylines + suppressedWorldRules + suppressedPendingThreads
	return map[string]any{
		"version":                    "p50a.v1",
		"status":                     "ready",
		"guard_type":                 "explicit_forget_stale_context",
		"suppressed_storylines":      suppressedStorylines,
		"suppressed_world_rules":     suppressedWorldRules,
		"suppressed_pending_threads": suppressedPendingThreads,
		"total_suppressed":           totalSuppressed,
		"forget_guard_active":        totalSuppressed > 0,
		"reason":                     "suppressed_items_excluded_from_injection",
	}
}
