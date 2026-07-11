package httpapi

import (
	"fmt"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// buildRecallResult assembles the JS-adapter-consumable recall bundle from
// already-read Store data. It does not perform any live vector retrieval or
// Store writes.
func buildRecallResult(
	sid string,
	queryPreview string,
	degraded bool,
	memories []store.Memory,
	evidence []store.DirectEvidence,
	kgTriples []store.KGTriple,
	episodeSums []store.EpisodeSummary,
	chatLogs []store.ChatLog,
	resumePack *store.ResumePack,
	vectorShadow map[string]any,
	storylines []store.Storyline,
	worldRules []store.WorldRule,
	pendingThreads []store.PendingThread,
	profile string,
	topK int,
) map[string]any {
	status := "ready"
	if degraded {
		status = "degraded"
	}
	topK = prepareTurnRecallLimit(topK)
	recallLimit := prepareTurnSupportRecallLimit(topK)

	var items []map[string]any
	memorySelection := selectPrepareTurnMemoryLanesWithVector(memories, queryPreview, topK, vectorShadow)
	appendMemoryItems := func(lane string, laneItems []store.Memory) {
		for _, m := range laneItems {
			summary := prepareTurnMemorySummary(m)
			summary = strings.Join(strings.Fields(summary), " ")
			if summary == "" {
				continue
			}
			item := map[string]any{
				"kind":       "memory",
				"source":     "memory",
				"lane":       lane,
				"id":         m.ID,
				"turn_index": m.TurnIndex,
				"summary":    summary,
				"importance": m.Importance,
			}
			if lane == "relevant" {
				if score := memorySelection.RelevantScores[prepareTurnMemoryLaneKey(m)]; score > 0 {
					item["keyword_overlap_score"] = score
				}
			}
			if lane == "vector_relevant" {
				if score := memorySelection.VectorScores[prepareTurnMemoryLaneKey(m)]; score > 0 {
					item["vector_rank_score"] = score
				}
			}
			items = append(items, item)
		}
	}
	appendMemoryItems("vector_relevant", memorySelection.VectorRelevant)
	appendMemoryItems("relevant", memorySelection.Relevant)
	appendMemoryItems("deep", memorySelection.Deep)
	appendMemoryItems("recent", memorySelection.Recent)
	for i, e := range evidence {
		if i >= recallLimit {
			break
		}
		text := strings.TrimSpace(e.EvidenceText)
		text = strings.Join(strings.Fields(text), " ")
		items = append(items, map[string]any{
			"kind":   "evidence",
			"source": "direct_evidence",
			"id":     e.ID,
			"text":   text,
		})
	}
	fallbackBound := 0
	rawFallbackLogs := []store.ChatLog{}
	vectorReadiness := buildPrepareTurnVectorReadiness(vectorShadow)
	vectorReadinessStatus := strings.TrimSpace(stringFromMap(vectorReadiness, "status"))
	vectorSearchAttempted := boolFromAny(vectorReadiness["search_attempted"])
	vectorFallbackApplies := !vectorSearchAttempted &&
		boolFromAny(vectorReadiness["fallback_recommended"]) &&
		vectorReadinessStatus != "disabled" &&
		vectorReadinessStatus != "vector_store_disabled" &&
		vectorReadinessStatus != "chromadb_unconfigured"
	rawFallbackActive := prepareTurnNeedsRawFallback(memorySelection, topK) || vectorFallbackApplies
	if rawFallbackActive && len(chatLogs) > 0 {
		rawFallbackLogs = selectRecentChatLogsByTurn(chatLogs, recallLimit)
		for _, cl := range rawFallbackLogs {
			content := strings.TrimSpace(cl.Content)
			content = strings.Join(strings.Fields(content), " ")
			if content == "" {
				continue
			}
			items = append(items, map[string]any{
				"kind":       "chat_log",
				"source":     "chat_log",
				"lane":       "raw_fallback",
				"id":         cl.ID,
				"turn_index": cl.TurnIndex,
				"role":       cl.Role,
				"content":    content,
			})
			fallbackBound++
		}
	}

	var kgItems []map[string]any
	for i, k := range kgTriples {
		if i >= recallLimit {
			break
		}
		kgItems = append(kgItems, map[string]any{
			"subject":   k.Subject,
			"predicate": k.Predicate,
			"object":    k.Object,
		})
	}

	var epItems []map[string]any
	for i, e := range episodeSums {
		if i >= recallLimit {
			break
		}
		summary := strings.TrimSpace(e.SummaryText)
		if summary == "" {
			summary = fmt.Sprintf("Episode %d-%d", e.FromTurn, e.ToTurn)
		}
		summary = strings.Join(strings.Fields(summary), " ")
		epItems = append(epItems, map[string]any{
			"from_turn": e.FromTurn,
			"to_turn":   e.ToTurn,
			"summary":   summary,
		})
	}

	counts := map[string]any{
		"memories_total":        len(memories),
		"memories_bound":        prepareTurnSelectedMemoryCount(memorySelection),
		"memory_count":          prepareTurnSelectedMemoryCount(memorySelection),
		"top_k_memory_target":   topK,
		"support_recall_limit":  recallLimit,
		"top_k_definition":      "semantic_memory_recall_limit",
		"vector_memory_bound":   len(memorySelection.VectorRelevant),
		"recent_memory_bound":   len(memorySelection.Recent),
		"relevant_memory_bound": len(memorySelection.Relevant),
		"deep_memory_bound":     len(memorySelection.Deep),
		"evidence_total":        len(evidence),
		"evidence_bound":        minInt(len(evidence), recallLimit),
		"kg_total":              len(kgTriples),
		"kg_bound":              minInt(len(kgTriples), recallLimit),
		"episodes_total":        len(episodeSums),
		"episodes_bound":        minInt(len(episodeSums), recallLimit),
		"chat_logs_total":       len(chatLogs),
		"fallback_total":        len(chatLogs),
		"fallback_bound":        fallbackBound,
		"fallback_count":        fallbackBound,
		"has_fallback":          fallbackBound > 0,
	}
	mergePrepareTurnMemoryLaneCounters(counts, memorySelection, false)
	recallLanes := buildPrepareTurnRecallLanes(memorySelection, rawFallbackLogs, vectorReadiness, topK)

	wouldCallVector := false
	if attempted, ok := vectorShadow["search_attempted"].(bool); ok {
		wouldCallVector = attempted
	}

	source := "go_r1_read_shadow"
	if vectorSource, _ := vectorShadow["source"].(string); vectorSource == "go_r2_chromadb_product_read" {
		source = vectorSource
	}

	chapter := chapterFromResumePack(resumePack)
	arc := arcFromResumePack(resumePack)
	saga := sagaFromResumePack(resumePack)
	chapterItems := []map[string]any{}
	if chapter != nil {
		chapterItems = append(chapterItems, map[string]any{
			"id":            chapter.ID,
			"from_turn":     chapter.FromTurn,
			"to_turn":       chapter.ToTurn,
			"chapter_index": chapter.ChapterIndex,
			"chapter_title": chapter.ChapterTitle,
			"summary":       strings.Join(strings.Fields(q1FirstNonEmptyString(chapter.SummaryText, chapter.ResumeText, chapter.ChapterTitle)), " "),
		})
	}
	arcItems := []map[string]any{}
	if arc != nil {
		arcItems = append(arcItems, map[string]any{
			"id":            arc.ID,
			"from_turn":     arc.FromTurn,
			"to_turn":       arc.ToTurn,
			"arc_index":     arc.ArcIndex,
			"arc_name":      arc.ArcName,
			"arc_status":    arc.ArcStatus,
			"core_conflict": arc.CoreConflict,
			"summary":       strings.Join(strings.Fields(q1FirstNonEmptyString(arc.ArcResumeText, arc.CoreConflict, arc.ArcName)), " "),
		})
	}
	sagaItems := []map[string]any{}
	if saga != nil {
		sagaItems = append(sagaItems, map[string]any{
			"id":         saga.ID,
			"from_turn":  saga.FromTurn,
			"to_turn":    saga.ToTurn,
			"era_label":  saga.EraLabel,
			"summary":    strings.Join(strings.Fields(q1FirstNonEmptyString(saga.ResumePackText, saga.SagaSummary, saga.EraLabel)), " "),
			"created_at": q1TimePtrAny(saga.CreatedAt),
		})
	}

	documents := buildUnifiedRetrievalDocuments(sid, memories, evidence, kgTriples, episodeSums, resumePack, chatLogs)
	documentSchema := retrievalDocumentSchemaQ1()
	indexSnapshot := retrievalIndexSnapshotFromDocuments(sid, documents)
	annSnapshot := buildANNCandidateSnapshotQ2(documents, vectorShadow)
	intentContract := buildIntentContractQ3()
	intentHitPreview := buildIntentHitPreviewQ3(queryPreview, documents)
	packetBudgetPolicy := q3PacketBudgetPolicy()
	tierCounts := map[string]int{}
	for _, doc := range documents {
		tier, _ := doc["tier"].(string)
		if tier != "" {
			tierCounts[tier]++
		}
	}
	counts["documents_total"] = len(documents)
	counts["tier_counts"] = tierCounts

	intentExecutionShadow := buildIntentExecutionShadow(documents, vectorShadow, profile, packetBudgetPolicy)

	// U-1e replay gate
	shadowStatus := "off"
	if s, ok := intentExecutionShadow["status"].(string); ok {
		shadowStatus = s
	}
	hasEvidence := len(chatLogs) > 0

	replayGate := map[string]any{
		"version":  "u1e.v1",
		"mode":     "captured_session_replay_gate_only",
		"status":   "pending",
		"decision": "hold",
		"reason":   "without_evidence",
	}
	routingMode, _ := intentExecutionShadow["routing_mode"].(string)
	if routingMode != "per_intent_shadow" {
		replayGate["status"] = "off"
		replayGate["decision"] = "fail_open"
		replayGate["reason"] = "runtime_mode_not_per_intent_shadow"
	} else if hasEvidence {
		replayGate["status"] = "ready"
		replayGate["decision"] = "promote_candidate"
		replayGate["reason"] = "passed_evidence"
	}
	intentExecutionShadow["replay_gate"] = replayGate

	// Routing shadow surfaces
	intentContract["routing_shadow_replay_gate"] = replayGate

	routingShadowBudget := map[string]any{
		"version":               "t1a.v1",
		"mode":                  "enforced_shadow",
		"selected_count_before": 0,
		"selected_count_after":  0,
		"dropped_count":         0,
		"event_count":           0,
		"reasons": map[string]int{
			"within_cap": 0,
			"over_cap":   0,
			"no_cap":     0,
		},
	}
	if be, ok := intentExecutionShadow["budget_enforcement"].(map[string]any); ok {
		if v, ok := be["selected_count_before"].(int); ok {
			routingShadowBudget["selected_count_before"] = v
		}
		if v, ok := be["selected_count_after"].(int); ok {
			routingShadowBudget["selected_count_after"] = v
		}
		if v, ok := be["dropped_count"].(int); ok {
			routingShadowBudget["dropped_count"] = v
		}
		if v, ok := be["event_count"].(int); ok {
			routingShadowBudget["event_count"] = v
		}
		if v, ok := be["budget_reasons"].(map[string]int); ok {
			routingShadowBudget["reasons"] = v
		}
	}
	intentContract["routing_shadow_budget"] = routingShadowBudget

	routingShadowTemporal := map[string]any{
		"version":                "s1g.v1",
		"mode":                   "shadow_temporal_scoring_only",
		"profile":                profile,
		"applied_intent_count":   0,
		"reordered_intent_count": 0,
		"reason":                 "profile_not_target",
	}
	if profile == "ultra" || profile == "extreme" {
		routingShadowTemporal["applied_intent_count"] = len(intentContract["intents"].([]map[string]any))
		routingShadowTemporal["reordered_intent_count"] = 0
		routingShadowTemporal["reason"] = "long_profile_temporal_scoring_applied"
	}
	intentContract["routing_shadow_temporal"] = routingShadowTemporal

	// Routing shadow takeover (s1e.v1)
	routingShadowTakeover := map[string]any{
		"version":  "s1e.v1",
		"mode":     "guarded_default_takeover_only",
		"status":   "off",
		"decision": "fail_open",
		"reason":   "runtime_mode_not_per_intent_shadow",
	}
	if shadowStatus != "off" {
		rgStatus := "pending"
		if rg, ok := replayGate["status"].(string); ok {
			rgStatus = rg
		}
		if rgStatus != "ready" {
			routingShadowTakeover["status"] = "pending"
			routingShadowTakeover["decision"] = "hold"
			routingShadowTakeover["reason"] = "replay_gate_not_ready"
		} else {
			selectedCountAfterVal := 0
			if be, ok := intentExecutionShadow["budget_enforcement"].(map[string]any); ok {
				if v, ok := be["selected_count_after"].(int); ok {
					selectedCountAfterVal = v
				}
			}
			if selectedCountAfterVal > 0 {
				routingShadowTakeover["status"] = "ready"
				routingShadowTakeover["decision"] = "promote_candidate"
				routingShadowTakeover["reason"] = "guarded_takeover_gate_passed"
			} else {
				routingShadowTakeover["status"] = "pending"
				routingShadowTakeover["decision"] = "hold"
				routingShadowTakeover["reason"] = "no_shadow_candidates"
			}
		}
	}
	intentContract["routing_shadow_takeover"] = routingShadowTakeover

	// Routing shadow enforced takeover (t1e.v1)
	takeoverStatus := "pending"
	takeoverReady := false
	promoteCandidate := ""
	selectedCountAfterVal := 0
	if be, ok := intentExecutionShadow["budget_enforcement"].(map[string]any); ok {
		if v, ok := be["selected_count_after"].(int); ok {
			selectedCountAfterVal = v
		}
	}
	if et, ok := intentExecutionShadow["enforced_takeover"].(map[string]any); ok {
		if cands, ok := et["selected_candidates"].([]string); ok && len(cands) > 0 {
			promoteCandidate = cands[0]
		}
	}
	if len(documents) > 0 && selectedCountAfterVal > 0 {
		takeoverReady = true
		takeoverStatus = "ready"
	} else if len(documents) == 0 {
		takeoverStatus = "off"
	}
	routingShadowEnforcedTakeover := map[string]any{
		"version":                 "t1e.v1",
		"mode":                    "enforced_default_takeover_only",
		"status":                  takeoverStatus,
		"ready":                   takeoverReady,
		"promote_candidate":       nilIfEmpty(promoteCandidate),
		"selected_candidates":     intentExecutionShadow["enforced_takeover"].(map[string]any)["selected_candidates"],
		"budget_enforcement_mode": "enforced_shadow",
		"selected_count_after":    selectedCountAfterVal,
		"reason":                  "routing_shadow_takeover_ready",
	}
	if !takeoverReady {
		if takeoverStatus == "off" {
			routingShadowEnforcedTakeover["reason"] = "no_candidates"
		} else {
			routingShadowEnforcedTakeover["reason"] = "guard_not_ready"
		}
	}
	intentContract["routing_shadow_enforced_takeover"] = routingShadowEnforcedTakeover
	if takeoverReady {
		packetBudgetPolicy["budget_mode"] = "enforced"
	}

	hierarchyConsistencyTrace := buildHierarchyConsistencyTrace(documents, resumePack, episodeSums)
	summaryFailureStability := buildSummaryFailureStability(degraded, chatLogs)
	annDefaultTakeoverGuard := buildANNTakeoverGuard(annSnapshot, vectorShadow)
	staleContextGuard := buildStaleContextGuard(storylines, worldRules, pendingThreads)
	searchBundle := map[string]any{
		"items":          items,
		"memory_count":   prepareTurnSelectedMemoryCount(memorySelection),
		"fallback_count": fallbackBound,
		"total_count":    len(items),
		"counts":         counts,
	}
	kgBundle := map[string]any{
		"items":         kgItems,
		"count":         len(kgItems),
		"entities_sent": 0,
	}
	episodeBundle := map[string]any{
		"items": epItems,
		"count": len(epItems),
	}

	return map[string]any{
		"status":                      status,
		"source":                      source,
		"chat_session_id":             sid,
		"query_preview":               queryPreview,
		"items":                       items,
		"kg_triples":                  kgItems,
		"episodes":                    epItems,
		"search":                      searchBundle,
		"kg":                          kgBundle,
		"episode":                     episodeBundle,
		"chapter":                     firstMapOrNil(chapterItems),
		"chapters":                    chapterItems,
		"arc":                         firstMapOrNil(arcItems),
		"arcs":                        arcItems,
		"saga":                        firstMapOrNil(sagaItems),
		"sagas":                       sagaItems,
		"documents":                   documents,
		"document_count":              len(documents),
		"document_schema":             documentSchema,
		"retrieval_document_schema":   documentSchema,
		"index_snapshot":              indexSnapshot,
		"ann_candidate_snapshot":      annSnapshot,
		"intent_contract":             intentContract,
		"packet_budget_policy":        packetBudgetPolicy,
		"intent_hit_preview":          intentHitPreview,
		"recall_lanes":                recallLanes,
		"counts":                      counts,
		"vector_shadow":               vectorShadow,
		"vector_readiness":            vectorReadiness,
		"would_call_vector":           wouldCallVector,
		"would_write":                 false,
		"intent_execution_shadow":     intentExecutionShadow,
		"hierarchy_consistency_trace": hierarchyConsistencyTrace,
		"summary_failure_stability":   summaryFailureStability,
		"ann_default_takeover_guard":  annDefaultTakeoverGuard,
		"stale_context_guard":         staleContextGuard,
		"temporal_proximity_boost": map[string]any{
			"version":      "p71a.v1",
			"status":       "shadow_only",
			"boost_active": profile == "long" || profile == "ultra" || profile == "extreme",
			"profile":      profile,
			"recent_turns": minInt(len(chatLogs), recallLimit),
			"reason":       "temporal_proximity_boost_shadow_only",
		},
		"trace": map[string]any{
			"indexed_candidate_path":  "unified_document_index_shadow",
			"legacy_candidate_source": "store_list_shadow_source_rows",
			"indexed_candidate_ready": len(documents) > 0,
			"intent_route":            "single_query_shared",
			"q1_retrieval_index": map[string]any{
				"status":         indexSnapshot["status"],
				"document_count": len(documents),
				"schema_version": documentSchema["version"],
			},
			"q2_hybrid_indexed_retrieval": map[string]any{
				"ann_candidate_preview": annSnapshot["candidate_count"],
				"rerank_policy":         annSnapshot["rerank_policy"],
				"merge_policy":          annSnapshot["merge_policy"],
				"benchmark_status":      annSnapshot["benchmark"].(map[string]any)["status"],
			},
			"q3_multi_intent_router": map[string]any{
				"routing_mode":    intentContract["routing_mode"],
				"intent_count":    len(intentContract["intents"].([]map[string]any)),
				"preview_status":  intentHitPreview["status"],
				"budget_policy":   packetBudgetPolicy["version"],
				"matched_intents": intentHitPreview["matched_intents"],
			},
			"r2_recall_lanes": map[string]any{
				"top_k_memory_target":     topK,
				"top_k_definition":        "semantic_memory_recall_limit",
				"vector_memory_count":     len(memorySelection.VectorRelevant),
				"recent_memory_count":     len(memorySelection.Recent),
				"relevant_memory_count":   len(memorySelection.Relevant),
				"deep_memory_count":       len(memorySelection.Deep),
				"raw_fallback_count":      fallbackBound,
				"vector_readiness_status": vectorReadiness["status"],
			},
		},
	}
}

func buildPrepareTurnRecallLanes(selection prepareTurnMemoryLaneSelection, rawFallbackLogs []store.ChatLog, vectorReadiness map[string]any, topK int) map[string]any {
	laneItems := func(lane string, memories []store.Memory) []map[string]any {
		out := make([]map[string]any, 0, len(memories))
		for _, item := range memories {
			summary := prepareTurnMemorySummary(item)
			if summary == "" {
				continue
			}
			row := map[string]any{
				"lane":       lane,
				"kind":       "memory",
				"id":         item.ID,
				"turn_index": item.TurnIndex,
				"summary":    summary,
				"importance": item.Importance,
				"reason":     laneSelectionReason(lane),
			}
			if lane == "relevant" {
				if score := selection.RelevantScores[prepareTurnMemoryLaneKey(item)]; score > 0 {
					row["keyword_overlap_score"] = score
				}
			}
			if lane == "vector_relevant" {
				if score := selection.VectorScores[prepareTurnMemoryLaneKey(item)]; score > 0 {
					row["vector_rank_score"] = score
				}
			}
			out = append(out, row)
		}
		return out
	}
	rawItems := make([]map[string]any, 0, len(rawFallbackLogs))
	for _, cl := range rawFallbackLogs {
		content := compactPrepareTurnLine(cl.Content, 0)
		if content == "" {
			continue
		}
		rawItems = append(rawItems, map[string]any{
			"lane":       "raw_fallback",
			"kind":       "chat_log",
			"id":         cl.ID,
			"turn_index": cl.TurnIndex,
			"role":       cl.Role,
			"content":    content,
			"reason":     "vector_or_memory_recall_degraded_raw_turn_support",
		})
	}
	return map[string]any{
		"version":             "r3.recall_lanes.v1",
		"top_k_definition":    "semantic_memory_recall_limit",
		"top_k_memory_target": topK,
		"vector_relevant": map[string]any{
			"count":      len(selection.VectorRelevant),
			"items":      laneItems("vector_relevant", selection.VectorRelevant),
			"policy":     "chromadb_semantic_hit_hydrated_to_mariadb_memory",
			"truth_role": "selector_only_mariadb_memory_is_canonical",
		},
		"recent": map[string]any{
			"count":  len(selection.Recent),
			"items":  laneItems("recent", selection.Recent),
			"policy": "latest_turn_index_anchor_after_relevant_memory",
		},
		"relevant": map[string]any{
			"count":  len(selection.Relevant),
			"items":  laneItems("relevant", selection.Relevant),
			"policy": "keyword_overlap_first_within_top_k_memory_target",
		},
		"deep": map[string]any{
			"count":  len(selection.Deep),
			"items":  laneItems("deep", selection.Deep),
			"policy": "high_importance_older_memory",
		},
		"raw_fallback": map[string]any{
			"count":      len(rawItems),
			"items":      rawItems,
			"active":     len(rawItems) > 0,
			"policy":     "recent_raw_turns_support_only",
			"truth_role": "fallback_support_not_canonical_truth",
		},
		"vector_readiness":      vectorReadiness,
		"selection_trace":       selection.Trace,
		"selected_total":        prepareTurnSelectedMemoryCount(selection) + len(rawItems),
		"no_user_input_rewrite": true,
	}
}

func laneSelectionReason(lane string) string {
	switch lane {
	case "vector_relevant":
		return "chromadb_semantic_hit_hydrated_to_mariadb_memory"
	case "recent":
		return "latest_memory_anchor_after_relevance"
	case "relevant":
		return "query_overlap_first_within_top_k_memory_target"
	case "deep":
		return "deep_past_importance_after_relevance_ranking"
	default:
		return "selected"
	}
}

// buildSessionState assembles the JS-adapter-consumable session_state bundle.
func buildSessionState(
	degraded bool,
	activeStates []store.ActiveState,
	storylines []store.Storyline,
	charStates []store.CharacterState,
	worldRules []store.WorldRule,
	pendingThreads []store.PendingThread,
	recallLimit int,
) map[string]any {
	recallLimit = prepareTurnRecallLimit(recallLimit)
	status := "ready"
	warnings := []any{}
	if degraded {
		status = "degraded"
		warnings = append(warnings, "Store unavailable; returning empty session state.")
	}

	boundedActive := make([]map[string]any, 0, minInt(len(activeStates), recallLimit))
	for i, as := range activeStates {
		if i >= recallLimit {
			break
		}
		boundedActive = append(boundedActive, map[string]any{
			"id":         as.ID,
			"state_type": as.StateType,
			"turn_index": as.TurnIndex,
		})
	}

	boundedStorylines := make([]map[string]any, 0, minInt(len(storylines), recallLimit))
	for i, sl := range storylines {
		if i >= recallLimit {
			break
		}
		boundedStorylines = append(boundedStorylines, map[string]any{
			"id":     sl.ID,
			"name":   sl.Name,
			"status": sl.Status,
		})
	}

	boundedChars := make([]map[string]any, 0, minInt(len(charStates), recallLimit))
	for i, cs := range charStates {
		if i >= recallLimit {
			break
		}
		boundedChars = append(boundedChars, map[string]any{
			"id":             cs.ID,
			"character_name": cs.CharacterName,
			"turn_index":     cs.TurnIndex,
		})
	}

	boundedRules := make([]map[string]any, 0, minInt(len(worldRules), recallLimit))
	for i, wr := range worldRules {
		if i >= recallLimit {
			break
		}
		boundedRules = append(boundedRules, map[string]any{
			"id":       wr.ID,
			"scope":    wr.Scope,
			"category": wr.Category,
			"key":      wr.Key,
		})
	}

	boundedThreads := make([]map[string]any, 0, minInt(len(pendingThreads), recallLimit))
	for i, pt := range pendingThreads {
		if i >= recallLimit {
			break
		}
		boundedThreads = append(boundedThreads, map[string]any{
			"id":           pt.ID,
			"thread_key":   pt.ThreadKey,
			"status":       pt.Status,
			"created_turn": pt.CreatedTurn,
		})
	}

	return map[string]any{
		"snapshot_status": status,
		"active_states":   boundedActive,
		"storylines":      boundedStorylines,
		"characters":      boundedChars,
		"world_rules":     boundedRules,
		"pending_threads": boundedThreads,
		"section_meta": map[string]any{
			"active_state_count":   len(activeStates),
			"storyline_count":      len(storylines),
			"character_count":      len(charStates),
			"world_rule_count":     len(worldRules),
			"pending_thread_count": len(pendingThreads),
		},
		"warnings":     warnings,
		"generated_at": time.Now().UTC().Format(time.RFC3339),
		"fetched":      true,
	}
}

// buildNarrativeControl assembles the JS-adapter-consumable narrative_control bundle.
func buildNarrativeControl(
	degraded bool,
	storylines []store.Storyline,
	worldRules []store.WorldRule,
	pendingThreads []store.PendingThread,
	charStates []store.CharacterState,
) map[string]any {
	stateStatus := "shadow_evidence"
	if degraded {
		stateStatus = "skeleton"
	}
	return map[string]any{
		"state_status":         stateStatus,
		"storyline_count":      len(storylines),
		"world_rule_count":     len(worldRules),
		"pending_thread_count": len(pendingThreads),
		"character_count":      len(charStates),
		"guide_mode":           "shadow_read",
		"narrative_stance":     "observational",
		"would_call_llm":       false,
		"would_write":          false,
	}
}

// buildContinuityPack assembles the JS-adapter-consumable continuity_pack bundle.
func buildContinuityPack(
	sid string,
	queryPreview string,
	degraded bool,
	resumePack *store.ResumePack,
	episodeSums []store.EpisodeSummary,
	chatLogs []store.ChatLog,
	activeStates []store.ActiveState,
	canonicalLayers []store.CanonicalStateLayer,
	recallLimit int,
) map[string]any {
	recallLimit = prepareTurnRecallLimit(recallLimit)
	status := "ready"
	if degraded {
		status = "degraded"
	}

	items := []map[string]any{}
	if resumePack != nil {
		text := strings.TrimSpace(resumePack.AssembledText)
		items = append(items, map[string]any{
			"kind":    "resume_pack",
			"present": true,
			"trigger": resumePack.Trigger,
			"text":    text,
		})
	}

	for i, es := range episodeSums {
		if i >= recallLimit {
			break
		}
		summary := strings.TrimSpace(es.SummaryText)
		if summary == "" {
			summary = fmt.Sprintf("Episode %d-%d", es.FromTurn, es.ToTurn)
		}
		summary = strings.Join(strings.Fields(summary), " ")
		items = append(items, map[string]any{
			"kind":      "episode_summary",
			"from_turn": es.FromTurn,
			"to_turn":   es.ToTurn,
			"summary":   summary,
		})
	}

	for _, cl := range selectRecentChatLogsByTurn(chatLogs, recallLimit) {
		content := strings.TrimSpace(cl.Content)
		content = strings.Join(strings.Fields(content), " ")
		items = append(items, map[string]any{
			"kind":       "chat_log",
			"turn_index": cl.TurnIndex,
			"role":       cl.Role,
			"content":    content,
		})
	}

	for i, as := range activeStates {
		if i >= recallLimit {
			break
		}
		content := strings.TrimSpace(as.Content)
		content = strings.Join(strings.Fields(content), " ")
		items = append(items, map[string]any{
			"kind":       "active_state",
			"state_type": as.StateType,
			"turn_index": as.TurnIndex,
			"content":    content,
		})
	}

	for i, cl := range canonicalLayers {
		if i >= recallLimit {
			break
		}
		content := strings.TrimSpace(cl.Content)
		content = strings.Join(strings.Fields(content), " ")
		items = append(items, map[string]any{
			"kind":       "canonical_layer",
			"layer_type": cl.LayerType,
			"turn_index": cl.TurnIndex,
			"content":    content,
		})
	}

	return map[string]any{
		"status":                status,
		"chat_session_id":       sid,
		"query_preview":         queryPreview,
		"resume_pack_present":   resumePack != nil,
		"episode_count":         len(episodeSums),
		"chat_log_count":        len(chatLogs),
		"active_state_count":    len(activeStates),
		"canonical_layer_count": len(canonicalLayers),
		"items":                 items,
		"would_call_llm":        false,
		"would_write":           false,
	}
}

// buildProgressionLedger assembles the JS-adapter-consumable progression_ledger bundle.
func buildProgressionLedger(sid string, degraded bool, storylines []store.Storyline, worldRules []store.WorldRule, pendingThreads []store.PendingThread, episodeSums []store.EpisodeSummary, recallLimit int) map[string]any {
	recallLimit = prepareTurnRecallLimit(recallLimit)
	status := "ready"
	if degraded {
		status = "degraded"
	}
	lastTurn := progressionLedgerLatestTurn(storylines, pendingThreads, episodeSums)
	lifecycleModel := map[string]any{
		"status":         "active",
		"states":         []string{"latent", "active", "escalating", "aftermath", "resolved", "dormant"},
		"pressure_scale": map[string]any{"min": 0, "max": 3},
		"decay_rules":    map[string]any{"latent": 5, "active": 4, "escalating": 3, "aftermath": 2, "resolved": 1, "dormant": 0},
		"mode":           "deterministic_no_llm",
	}
	doNotResolveGuard := map[string]any{
		"status":                "active",
		"mode":                  "deterministic_no_llm",
		"min_turn_gap":          2,
		"protected_entry_types": []string{"unresolved_tension", "payoff"},
		"protected_sources":     []string{"storyline.ongoing_tensions", "pending_thread.promise", "pending_thread.open_question"},
		"long_horizon_tokens":   []string{"promise", "payoff", "callback", "debt", "oath", "answer", "thread", "unresolved"},
	}
	unresolvedTensions := progressionLedgerUnresolvedTensions(storylines, pendingThreads, lifecycleModel, doNotResolveGuard, lastTurn, recallLimit)
	consequences := progressionLedgerConsequences(worldRules, episodeSums, lifecycleModel, lastTurn, recallLimit)
	payoffs := progressionLedgerPayoffs(storylines, pendingThreads, lifecycleModel, doNotResolveGuard, lastTurn, recallLimit)
	sceneDeltas := progressionLedgerSceneDeltas(episodeSums, lifecycleModel, lastTurn, recallLimit)
	worldPressure := progressionLedgerWorldPressure(worldRules, pendingThreads, storylines, lastTurn, recallLimit)
	lastAdvancedTurn := any(nil)
	lastValidatedTurn := any(nil)
	if lastTurn > 0 {
		lastAdvancedTurn = lastTurn
		if status == "ready" {
			lastValidatedTurn = lastTurn
		}
	}
	return map[string]any{
		"status":                               status,
		"chat_session_id":                      sid,
		"storyline_count":                      len(storylines),
		"world_rule_count":                     len(worldRules),
		"pending_thread_count":                 len(pendingThreads),
		"episode_count":                        len(episodeSums),
		"would_write":                          false,
		"ledger_policy_version":                "lw1h.v1",
		"ledger_mode":                          "deterministic_no_llm",
		"last_advanced_turn":                   lastAdvancedTurn,
		"last_validated_turn":                  lastValidatedTurn,
		"unresolved_tensions":                  unresolvedTensions,
		"consequences":                         consequences,
		"payoffs":                              payoffs,
		"scene_deltas":                         sceneDeltas,
		"world_pressure_policy_version":        "lw1d.v1",
		"world_pressure":                       worldPressure,
		"continuity_precedence_policy_version": "lw1e.v1",
		"supporting_precedence_guard": map[string]any{
			"status":                                   "supporting_only",
			"supporting_only":                          true,
			"cannot_override_current_user_input":       true,
			"cannot_override_verified_direct_evidence": true,
			"precedence_ceiling":                       "below_current_user_input_and_verified_direct_evidence",
			"allowed_usage":                            []string{"continuity_hint", "narrative_support"},
			"disallowed_usage":                         []string{"truth_overwrite", "canonical_override"},
		},
		"compatibility_policy_version": "lw1f.v1",
		"compatibility_contract": map[string]any{
			"status":           "compatible",
			"targets":          []string{"chapter_summary", "arc_summary", "continuity_pack"},
			"shape_mode":       "additive_non_breaking",
			"consumer_safe":    true,
			"adapter_required": false,
		},
		"lifecycle_policy_version":      "lw1g.v1",
		"lifecycle_model":               lifecycleModel,
		"do_not_resolve_policy_version": "lw1h.v1",
		"do_not_resolve_guard":          doNotResolveGuard,
	}
}

func progressionLedgerLatestTurn(storylines []store.Storyline, pendingThreads []store.PendingThread, episodeSums []store.EpisodeSummary) int {
	latest := 0
	for _, sl := range storylines {
		latest = maxInt(latest, sl.LastTurn)
		latest = maxInt(latest, sl.LastEvidenceTurn)
	}
	for _, pt := range pendingThreads {
		latest = maxInt(latest, pt.LastSeenTurn)
		latest = maxInt(latest, pt.SourceTurn)
		latest = maxInt(latest, pt.CreatedTurn)
	}
	for _, ep := range episodeSums {
		latest = maxInt(latest, ep.ToTurn)
	}
	return latest
}

func progressionLedgerUnresolvedTensions(storylines []store.Storyline, pendingThreads []store.PendingThread, lifecycleModel map[string]any, guard map[string]any, lastTurn, recallLimit int) []any {
	recallLimit = prepareTurnRecallLimit(recallLimit)
	items := []any{}
	for _, sl := range storylines {
		if sl.Suppressed || strings.EqualFold(sl.Status, "resolved") {
			continue
		}
		for _, tension := range denseJSONItems(sl.OngoingTensionsJSON, recallLimit) {
			label := normalizeStoryLedgerLabel(tension)
			if label == "" {
				continue
			}
			pressure, decay := lifecycleProfileForState("active", lifecycleModel)
			entry := map[string]any{
				"entry_type":         "unresolved_tension",
				"label":              label,
				"source":             "storyline.ongoing_tensions",
				"status":             "open",
				"lifecycle_state":    "active",
				"pressure_score":     pressure,
				"decay_turns":        decay,
				"deterministic":      true,
				"source_record_id":   sl.ID,
				"source_message_ids": []any{},
				"affected_relations": []any{sl.Name},
				"affected_world":     []any{},
			}
			attachDoNotResolveFields(entry, guard, lastTurn)
			items = append(items, entry)
			if len(items) >= recallLimit {
				return items
			}
		}
	}
	for _, pt := range pendingThreads {
		if pt.Suppressed || strings.EqualFold(pt.Status, "resolved") {
			continue
		}
		label := normalizeStoryLedgerLabel(q1FirstNonEmptyString(pt.Description, pt.Title, pt.ThreadKey))
		if label == "" {
			continue
		}
		pressure, decay := lifecycleProfileForState("latent", lifecycleModel)
		entry := map[string]any{
			"entry_type":         "unresolved_tension",
			"label":              label,
			"source":             "pending_thread." + q1FirstNonEmptyString(pt.HookType, "open_question"),
			"status":             "open",
			"lifecycle_state":    "latent",
			"pressure_score":     pressure,
			"decay_turns":        decay,
			"deterministic":      true,
			"source_record_id":   pt.ID,
			"source_message_ids": []any{},
			"affected_relations": []any{q1FirstNonEmptyString(pt.Target, pt.Owner)},
			"affected_world":     []any{},
		}
		attachDoNotResolveFields(entry, guard, lastTurn)
		items = append(items, entry)
		if len(items) >= recallLimit {
			return items
		}
	}
	return items
}

func progressionLedgerConsequences(worldRules []store.WorldRule, episodeSums []store.EpisodeSummary, lifecycleModel map[string]any, lastTurn, recallLimit int) []any {
	recallLimit = prepareTurnRecallLimit(recallLimit)
	items := []any{}
	for _, wr := range worldRules {
		if wr.Suppressed {
			continue
		}
		label := normalizeStoryLedgerLabel(q1FirstNonEmptyString(wr.Key, wr.Category, wr.ScopeName))
		if label == "" {
			continue
		}
		pressure, decay := lifecycleProfileForState("escalating", lifecycleModel)
		items = append(items, map[string]any{
			"entry_type":         "consequence",
			"label":              label,
			"source":             "world_rule",
			"status":             "pending",
			"turn_hint":          maxInt(wr.SourceTurn, lastTurn),
			"lifecycle_state":    "escalating",
			"pressure_score":     pressure,
			"decay_turns":        decay,
			"deterministic":      true,
			"source_record_id":   wr.ID,
			"source_message_ids": []any{},
			"affected_relations": []any{},
			"affected_world":     []any{label},
		})
		if len(items) >= recallLimit {
			return items
		}
	}
	for _, ep := range episodeSums {
		for _, event := range denseJSONItems(ep.KeyEvents, recallLimit) {
			label := normalizeStoryLedgerLabel(event)
			if label == "" {
				continue
			}
			pressure, decay := lifecycleProfileForState("aftermath", lifecycleModel)
			items = append(items, map[string]any{
				"entry_type":         "consequence",
				"label":              label,
				"source":             "episode.key_events",
				"status":             "pending",
				"turn_hint":          ep.ToTurn,
				"lifecycle_state":    "aftermath",
				"pressure_score":     pressure,
				"decay_turns":        decay,
				"deterministic":      true,
				"source_record_id":   ep.ID,
				"source_message_ids": []any{},
				"affected_relations": []any{},
				"affected_world":     []any{},
			})
			if len(items) >= recallLimit {
				return items
			}
		}
	}
	return items
}

func progressionLedgerPayoffs(storylines []store.Storyline, pendingThreads []store.PendingThread, lifecycleModel map[string]any, guard map[string]any, lastTurn, recallLimit int) []any {
	recallLimit = prepareTurnRecallLimit(recallLimit)
	items := []any{}
	for _, pt := range pendingThreads {
		if pt.Suppressed {
			continue
		}
		label := normalizeStoryLedgerLabel(q1FirstNonEmptyString(pt.Description, pt.Title, pt.ThreadKey))
		if label == "" {
			continue
		}
		state := "pending"
		lifecycleState := "active"
		if strings.EqualFold(pt.Status, "resolved") {
			state = "completed"
			lifecycleState = "resolved"
		} else if strings.EqualFold(pt.Status, "cancelled") || strings.EqualFold(pt.Status, "invalid") || strings.EqualFold(pt.Status, "suppressed") {
			state = "invalid"
			lifecycleState = "dormant"
		}
		pressure, decay := lifecycleProfileForState(lifecycleState, lifecycleModel)
		entry := map[string]any{
			"entry_type":         "payoff",
			"label":              label,
			"source":             "pending_thread." + q1FirstNonEmptyString(pt.HookType, "promise"),
			"status":             state,
			"payoff_state":       state,
			"lifecycle_state":    lifecycleState,
			"pressure_score":     pressure,
			"decay_turns":        decay,
			"deterministic":      true,
			"source_record_id":   pt.ID,
			"source_message_ids": []any{},
			"affected_relations": []any{q1FirstNonEmptyString(pt.Target, pt.Owner)},
			"affected_world":     []any{},
		}
		attachDoNotResolveFields(entry, guard, lastTurn)
		items = append(items, entry)
		if len(items) >= recallLimit {
			return items
		}
	}
	for _, sl := range storylines {
		if sl.Suppressed || strings.EqualFold(sl.Status, "resolved") {
			continue
		}
		for _, tension := range denseJSONItems(sl.OngoingTensionsJSON, recallLimit) {
			label := normalizeStoryLedgerLabel(tension)
			if label == "" {
				continue
			}
			pressure, decay := lifecycleProfileForState("latent", lifecycleModel)
			entry := map[string]any{
				"entry_type":         "payoff",
				"label":              label,
				"source":             "storyline.ongoing_tensions",
				"status":             "pending",
				"payoff_state":       "pending",
				"lifecycle_state":    "latent",
				"pressure_score":     pressure,
				"decay_turns":        decay,
				"deterministic":      true,
				"source_record_id":   sl.ID,
				"source_message_ids": []any{},
				"affected_relations": []any{sl.Name},
				"affected_world":     []any{},
			}
			attachDoNotResolveFields(entry, guard, lastTurn)
			items = append(items, entry)
			if len(items) >= recallLimit {
				return items
			}
		}
	}
	return items
}

func progressionLedgerSceneDeltas(episodeSums []store.EpisodeSummary, lifecycleModel map[string]any, lastTurn, recallLimit int) []any {
	recallLimit = prepareTurnRecallLimit(recallLimit)
	items := []any{}
	for _, ep := range episodeSums {
		label := normalizeStoryLedgerLabel(q1FirstNonEmptyString(ep.SummaryText, fmt.Sprintf("Episode %d-%d", ep.FromTurn, ep.ToTurn)))
		if label == "" {
			continue
		}
		pressure, decay := lifecycleProfileForState("active", lifecycleModel)
		items = append(items, map[string]any{
			"entry_type":         "scene_delta",
			"label":              truncateRunes(label, 180),
			"source":             "episode_summary",
			"status":             "observed",
			"turn_hint":          maxInt(ep.ToTurn, lastTurn),
			"lifecycle_state":    "active",
			"pressure_score":     pressure,
			"decay_turns":        decay,
			"deterministic":      true,
			"source_record_id":   ep.ID,
			"source_message_ids": []any{},
			"affected_relations": []any{},
			"affected_world":     []any{},
		})
		if len(items) >= recallLimit {
			return items
		}
	}
	return items
}

func progressionLedgerWorldPressure(worldRules []store.WorldRule, pendingThreads []store.PendingThread, storylines []store.Storyline, lastTurn, recallLimit int) map[string]any {
	recallLimit = prepareTurnRecallLimit(recallLimit)
	storyPlan := map[string]any{
		"next_beats":      []string{},
		"execution_notes": []string{},
		"guardrails":      []string{},
		"current_arc":     "",
	}
	director := map[string]any{
		"world_guardrails":  []string{},
		"resolved_outcomes": []string{},
	}
	for _, wr := range worldRules {
		if len(asStringSlice(director["world_guardrails"])) >= recallLimit {
			break
		}
		if wr.Suppressed {
			continue
		}
		label := q1FirstNonEmptyString(wr.Key, wr.Category, wr.ScopeName)
		if label != "" {
			director["world_guardrails"] = append(asStringSlice(director["world_guardrails"]), label)
		}
	}
	for _, pt := range pendingThreads {
		if len(asStringSlice(storyPlan["next_beats"])) >= recallLimit {
			break
		}
		if pt.Suppressed || strings.EqualFold(pt.Status, "resolved") {
			continue
		}
		label := q1FirstNonEmptyString(pt.Description, pt.Title, pt.ThreadKey)
		if label != "" {
			storyPlan["next_beats"] = append(asStringSlice(storyPlan["next_beats"]), label)
		}
	}
	for _, sl := range storylines {
		if len(asStringSlice(storyPlan["execution_notes"])) >= recallLimit && len(asStringSlice(director["resolved_outcomes"])) >= recallLimit {
			break
		}
		if sl.Suppressed {
			continue
		}
		if strings.EqualFold(sl.Status, "resolved") {
			director["resolved_outcomes"] = append(asStringSlice(director["resolved_outcomes"]), sl.Name)
			continue
		}
		if sl.Name != "" {
			storyPlan["execution_notes"] = append(asStringSlice(storyPlan["execution_notes"]), sl.Name)
		}
	}
	return buildWorldPressure(storyPlan, director, asStringSlice(storyPlan["next_beats"]), asStringSlice(director["resolved_outcomes"]), lastTurn)
}

func progressionLedgerTracePreviewFields(ledger map[string]any) map[string]any {
	unresolved := lenAnySlice(ledger["unresolved_tensions"])
	consequences := lenAnySlice(ledger["consequences"])
	payoffs := lenAnySlice(ledger["payoffs"])
	sceneDeltas := lenAnySlice(ledger["scene_deltas"])
	worldPressure, _ := ledger["world_pressure"].(map[string]any)
	lifecycleModel, _ := ledger["lifecycle_model"].(map[string]any)
	supportingGuard, _ := ledger["supporting_precedence_guard"].(map[string]any)
	compatibility, _ := ledger["compatibility_contract"].(map[string]any)
	return map[string]any{
		"story_ledger_mode":                                     ledger["ledger_mode"],
		"story_ledger_policy_version":                           ledger["ledger_policy_version"],
		"unresolved_tensions_count":                             unresolved,
		"consequences_count":                                    consequences,
		"payoffs_count":                                         payoffs,
		"scene_deltas_count":                                    sceneDeltas,
		"payoff_pending_count":                                  countLedgerPayoffState(ledger["payoffs"], "pending"),
		"payoff_completed_count":                                countLedgerPayoffState(ledger["payoffs"], "completed"),
		"payoff_invalid_count":                                  countLedgerPayoffState(ledger["payoffs"], "invalid"),
		"world_pressure_ready":                                  worldPressure != nil,
		"world_pressure_policy_version":                         ledger["world_pressure_policy_version"],
		"world_pressure_factions_count":                         lenAnySlice(worldPressure["factions"]),
		"world_pressure_regions_count":                          lenAnySlice(worldPressure["regions"]),
		"world_pressure_offscreen_threads_count":                lenAnySlice(worldPressure["offscreen_threads"]),
		"world_pressure_public_pressure_count":                  lenAnySlice(worldPressure["public_pressure"]),
		"world_pressure_timeline_count":                         lenAnySlice(worldPressure["timeline"]),
		"continuity_precedence_policy_version":                  ledger["continuity_precedence_policy_version"],
		"supporting_precedence_guard_ready":                     supportingGuard != nil,
		"supporting_precedence_supporting_only":                 supportingGuard["supporting_only"],
		"supporting_precedence_blocks_user_input_override":      supportingGuard["cannot_override_current_user_input"],
		"supporting_precedence_blocks_direct_evidence_override": supportingGuard["cannot_override_verified_direct_evidence"],
		"compatibility_policy_version":                          ledger["compatibility_policy_version"],
		"compatibility_ready":                                   compatibility != nil,
		"compatibility_targets_count":                           lenAnySlice(compatibility["targets"]),
		"compatibility_consumer_safe":                           compatibility["consumer_safe"],
		"lifecycle_policy_version":                              ledger["lifecycle_policy_version"],
		"lifecycle_ready":                                       lifecycleModel != nil,
		"lifecycle_states_count":                                lenAnySlice(lifecycleModel["states"]),
		"lifecycle_decay_rules_count":                           lenAnyMap(lifecycleModel["decay_rules"]),
		"lifecycle_entry_count":                                 unresolved + consequences + payoffs + sceneDeltas,
		"lifecycle_latent_count":                                countLedgerLifecycleState(ledger, "latent"),
		"lifecycle_active_count":                                countLedgerLifecycleState(ledger, "active"),
		"lifecycle_escalating_count":                            countLedgerLifecycleState(ledger, "escalating"),
		"lifecycle_aftermath_count":                             countLedgerLifecycleState(ledger, "aftermath"),
		"lifecycle_resolved_count":                              countLedgerLifecycleState(ledger, "resolved"),
		"lifecycle_dormant_count":                               countLedgerLifecycleState(ledger, "dormant"),
		"do_not_resolve_policy_version":                         ledger["do_not_resolve_policy_version"],
		"do_not_resolve_guard_ready":                            ledger["do_not_resolve_guard"] != nil,
		"do_not_resolve_protected_count":                        countLedgerDoNotResolve(ledger),
		"do_not_resolve_unresolved_count":                       countLedgerDoNotResolveIn(ledger["unresolved_tensions"]),
		"do_not_resolve_payoff_pending_count":                   countLedgerDoNotResolvePendingPayoffs(ledger["payoffs"]),
	}
}

func lenAnySlice(value any) int {
	switch v := value.(type) {
	case []any:
		return len(v)
	case []string:
		return len(v)
	case []map[string]any:
		return len(v)
	default:
		return 0
	}
}

func lenAnyMap(value any) int {
	switch v := value.(type) {
	case map[string]any:
		return len(v)
	case map[string]int:
		return len(v)
	default:
		return 0
	}
}

func mapSliceFromAny(value any) []map[string]any {
	switch items := value.(type) {
	case []map[string]any:
		return items
	case []any:
		out := []map[string]any{}
		for _, raw := range items {
			if item, ok := raw.(map[string]any); ok {
				out = append(out, item)
			}
		}
		return out
	default:
		return nil
	}
}

func ledgerEntryMaps(value any) []map[string]any {
	out := []map[string]any{}
	if items, ok := value.([]any); ok {
		for _, raw := range items {
			if item, ok := raw.(map[string]any); ok {
				out = append(out, item)
			}
		}
	}
	return out
}

func countLedgerPayoffState(value any, state string) int {
	count := 0
	for _, item := range ledgerEntryMaps(value) {
		if strings.EqualFold(asString(item["payoff_state"]), state) || strings.EqualFold(asString(item["status"]), state) {
			count++
		}
	}
	return count
}

func countLedgerLifecycleState(ledger map[string]any, state string) int {
	count := 0
	for _, key := range []string{"unresolved_tensions", "consequences", "payoffs", "scene_deltas"} {
		for _, item := range ledgerEntryMaps(ledger[key]) {
			if strings.EqualFold(asString(item["lifecycle_state"]), state) {
				count++
			}
		}
	}
	return count
}

func countLedgerDoNotResolve(ledger map[string]any) int {
	return countLedgerDoNotResolveIn(ledger["unresolved_tensions"]) + countLedgerDoNotResolveIn(ledger["payoffs"])
}

func countLedgerDoNotResolveIn(value any) int {
	count := 0
	for _, item := range ledgerEntryMaps(value) {
		if item["do_not_resolve_yet"] == true {
			count++
		}
	}
	return count
}

func countLedgerDoNotResolvePendingPayoffs(value any) int {
	count := 0
	for _, item := range ledgerEntryMaps(value) {
		if item["do_not_resolve_yet"] == true && strings.EqualFold(asString(item["payoff_state"]), "pending") {
			count++
		}
	}
	return count
}

// buildAutonomyPlan assembles the JS-adapter-consumable autonomy_plan bundle.
