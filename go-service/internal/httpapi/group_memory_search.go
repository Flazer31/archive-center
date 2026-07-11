package httpapi

import (
	"context"
	"errors"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	var req dto.SearchRequest
	if err := dto.DecodeWithDefaults(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	items := []any{}
	memoryCount := 0
	totalCount := 0

	chatSessionID := ""
	if req.ChatSessionID != nil {
		chatSessionID = *req.ChatSessionID
	}
	if lock, err := s.sessionMigrationSourceLock(r.Context(), chatSessionID); err != nil {
		writeInternalError(w, err.Error())
		return
	} else if lock != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"items":                 []any{},
			"story_cards":           []any{},
			"story_cards_count":     0,
			"injection_text":        "",
			"memory_count":          0,
			"fallback_count":        0,
			"has_fallback":          false,
			"total_count":           0,
			"fallback_rate_metric":  0,
			"stale_hit_metric":      0,
			"no_candidate_metric":   1,
			"hydration_miss_metric": 0,
			"observability_status":  "source_session_migrated_away",
			"read_excluded":         true,
			"read_exclusion_reason": "source_session_migrated_away",
			"target_session_id":     lock.TargetSessionID,
			"migration_source_lock": sessionMigrationLockPayload(lock),
			"signal_mix_summary": map[string]any{
				"memory_signal_count":   0,
				"chat_log_signal_count": 0,
				"total_signal_count":    0,
				"tagging_version":       "p166a.v1",
				"tagging_policy":        "source_session_migrated_away_read_exclusion",
				"reason":                "session_migration_source_lock",
			},
		})
		return
	}

	retrievalMode := "store_only"
	retrievalFallbackReason := ""
	vectorSearchTrace := map[string]any{
		"status": "disabled",
		"reason": "vector_store_disabled",
	}
	hydrationMissMetric := 0
	topK := searchTopK(req.TopK)
	chatLogFallbackEnabled := true
	chatLogFallbackBlockedReason := ""

	if chatSessionID != "" && s.Store != nil {
		memories, err := s.Store.ListMemories(r.Context(), chatSessionID, 0, 0)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		if err == nil {
			totalCount = len(memories)
			if s.Vector != nil {
				items, vectorSearchTrace, retrievalMode, retrievalFallbackReason = s.vectorFirstSearchMemoryItems(r.Context(), req, chatSessionID, memories, topK)
				hydrationMissMetric = intFromAny(vectorSearchTrace["hydration_miss_count"], 0)
				if searchVectorFirstBlocksChatLogFallback(vectorSearchTrace) {
					chatLogFallbackEnabled = false
					chatLogFallbackBlockedReason = "vector_first_no_chat_log_top_k_fill"
				}
			} else {
				items = scoredSearchMemoryItems(memories, req.UserInput, topK, s.currentEmbeddingModelIdentity())
			}
			memoryCount = countSearchItemsBySource(items, "memory")
		}
		if len(items) < topK && chatLogFallbackEnabled {
			logs, err := s.Store.ListChatLogs(r.Context(), chatSessionID, 0, 0)
			if err != nil && !errors.Is(err, store.ErrNotEnabled) {
				writeInternalError(w, err.Error())
				return
			}
			if err == nil {
				fallbackItems := scoredFallbackChatLogItems(logs, req.UserInput, topK-len(items))
				items = append(items, fallbackItems...)
			}
		}
	}
	vectorSearchTrace["chat_log_fallback_enabled"] = chatLogFallbackEnabled
	vectorSearchTrace["chat_log_fallback_blocked_reason"] = nilIfEmpty(chatLogFallbackBlockedReason)

	injectionText := searchInjectionText(items)
	fallbackCount := countSearchItemsBySource(items, "chat_log")
	fallbackRate := 0.0
	if totalCount > 0 {
		fallbackRate = float64(fallbackCount) / float64(totalCount)
	}
	noCandidate := 0
	if memoryCount == 0 && fallbackCount == 0 {
		noCandidate = 1
	}
	// SEQ-16-P166: tag each item with a signal_mix_source_tag derived from its
	// existing `source` field. The tag is non-mutating and stays in sync with
	// the source string.
	taggedItems := make([]any, 0, len(items))
	for _, raw := range items {
		m, ok := raw.(map[string]any)
		if !ok {
			taggedItems = append(taggedItems, raw)
			continue
		}
		src := strings.TrimSpace(stringFromAny(m["source"]))
		tag := "support"
		switch src {
		case "memory":
			tag = "primary_signal_memory"
		case "chat_log":
			tag = "fallback_signal_chat_log"
		case "direct_evidence":
			tag = "support_signal_evidence"
		case "kg_triple":
			tag = "support_signal_kg"
		}
		copy := map[string]any{}
		for k, v := range m {
			copy[k] = v
		}
		copy["signal_mix_source_tag"] = tag
		copy["signal_mix_source_confirmed"] = src
		taggedItems = append(taggedItems, copy)
	}
	storyCards := searchStoryCards(taggedItems)
	signalMixSummary := map[string]any{
		"memory_signal_count":   countSearchItemsBySource(items, "memory"),
		"chat_log_signal_count": countSearchItemsBySource(items, "chat_log"),
		"total_signal_count":    len(items),
		"tagging_version":       "p166a.v1",
		"tagging_policy":        "tag_derived_from_source_field",
		"reason":                "seq16_p166_signal_mix_source_tag_confirm",
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":                 taggedItems,
		"story_cards":           storyCards,
		"story_cards_count":     len(storyCards),
		"injection_text":        injectionText,
		"memory_count":          memoryCount,
		"fallback_count":        fallbackCount,
		"has_fallback":          fallbackCount > 0,
		"total_count":           totalCount,
		"fallback_rate_metric":  fallbackRate,
		"stale_hit_metric":      0,
		"no_candidate_metric":   noCandidate,
		"hydration_miss_metric": hydrationMissMetric,
		"observability_status":  "shadow_r1",
		"retrieval_mode":        retrievalMode,
		"retrieval_fallback_reason": nilIfEmpty(
			retrievalFallbackReason,
		),
		"vector_search":                    vectorSearchTrace,
		"chat_log_fallback_enabled":        chatLogFallbackEnabled,
		"chat_log_fallback_blocked_reason": nilIfEmpty(chatLogFallbackBlockedReason),
		"signal_mix_summary":               signalMixSummary,
	})
}

func searchVectorFirstBlocksChatLogFallback(trace map[string]any) bool {
	if trace == nil {
		return false
	}
	if boolFromAny(trace["search_attempted"]) {
		return true
	}
	result := strings.TrimSpace(stringFromMap(trace, "search_result"))
	return result == "ok" || result == "not_found" || result == "error" || result == "err_not_enabled"
}

func searchStoryCards(items []any) []any {
	cards := []any{}
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok || strings.TrimSpace(stringFromAny(item["source"])) != "memory" {
			continue
		}
		parsed := parseJSONMap(stringFromAny(item["summary_json"]))
		cardType := normalizeStoryCardType(parsed)
		if cardType == "" {
			continue
		}
		title := storyCardTitle(cardType, parsed, item)
		summary := strings.TrimSpace(firstNonEmptyString([]string{
			stringFromAny(parsed["summary"]),
			stringFromAny(parsed["text"]),
			stringFromAny(item["summary_preview"]),
		}))
		if title == "" && summary == "" {
			continue
		}
		cards = append(cards, map[string]any{
			"card_type":    cardType,
			"title":        title,
			"summary":      summary,
			"source_turn":  item["turn_index"],
			"display_mode": "story_card",
			"memory_id":    item["id"],
			"importance":   item["importance"],
		})
	}
	return cards
}

func normalizeStoryCardType(parsed map[string]any) string {
	raw := strings.ToLower(strings.TrimSpace(firstNonEmptyString([]string{
		stringFromAny(parsed["entity_type"]),
		stringFromAny(parsed["type"]),
		stringFromAny(parsed["kind"]),
	})))
	switch raw {
	case "person", "character":
		return "person"
	case "place", "location":
		return "place"
	case "item", "object", "artifact":
		return "item"
	default:
		return ""
	}
}

func storyCardTitle(cardType string, parsed map[string]any, item map[string]any) string {
	switch cardType {
	case "person":
		return strings.TrimSpace(firstNonEmptyString([]string{
			stringFromAny(parsed["name"]),
			stringFromAny(parsed["character"]),
			stringFromAny(parsed["person"]),
		}))
	case "place":
		return strings.TrimSpace(firstNonEmptyString([]string{
			stringFromAny(parsed["location"]),
			stringFromAny(parsed["place"]),
			strings.TrimSpace(strings.Join(nonEmptyStrings([]string{
				stringFromAny(item["place_wing"]),
				stringFromAny(item["place_room"]),
			}), " / ")),
		}))
	case "item":
		return strings.TrimSpace(firstNonEmptyString([]string{
			stringFromAny(parsed["item"]),
			stringFromAny(parsed["object"]),
			stringFromAny(parsed["artifact"]),
			stringFromAny(parsed["name"]),
		}))
	default:
		return ""
	}
}

type scoredSearchItem struct {
	item       map[string]any
	finalScore float64
}

const (
	hybridKeywordPolicyVersion    = "hy1a.v1"
	hybridSoftBiasPolicyVersion   = "hy1b.v1"
	hybridTailBudgetPolicyVersion = "hy1d.v1"
	hybridKeywordScoreCap         = 0.18
	hybridSpeakerBiasWeight       = 0.04
	hybridLocationBiasWeight      = 0.05
	hybridStorylineBiasWeight     = 0.06
	hybridSoftBiasCap             = 0.12
)

var hybridKeywordStopwords = map[string]bool{
	"a": true, "an": true, "and": true, "are": true, "as": true, "at": true, "be": true,
	"but": true, "by": true, "for": true, "from": true, "has": true, "have": true,
	"he": true, "her": true, "his": true, "i": true, "in": true, "is": true,
	"it": true, "its": true, "me": true, "my": true, "of": true, "on": true,
	"or": true, "our": true, "she": true, "so": true, "that": true, "the": true,
	"their": true, "them": true, "then": true, "there": true, "they": true,
	"this": true, "to": true, "was": true, "we": true, "were": true, "with": true,
	"you": true, "your": true,
}

func searchTopK(value *int) int {
	if value == nil || *value <= 0 {
		return 5
	}
	return *value
}

func (s *Server) vectorFirstSearchMemoryItems(ctx context.Context, req dto.SearchRequest, sid string, memories []store.Memory, topK int) ([]any, map[string]any, string, string) {
	if topK <= 0 {
		topK = searchTopK(nil)
	}
	trace := map[string]any{
		"status":         "store_only",
		"retrieval_mode": "store_only",
		"reason":         "vector_store_disabled",
	}
	if s.Vector == nil {
		return scoredSearchMemoryItems(memories, req.UserInput, topK, s.currentEmbeddingModelIdentity()), trace, "store_only", ""
	}

	rawUserInput := strings.TrimSpace(req.UserInput)
	prepareReq := dto.PrepareTurnRequest{
		ChatSessionID: sid,
		RawUserInput:  &rawUserInput,
	}
	vectorShadow := s.prepareTurnVectorShadow(ctx, prepareReq, topK)
	hydration := prepareTurnHydrateVectorMemoryHits(memories, vectorShadow, topK)
	hydrationTrace := mapFromAny(hydration.Trace)
	vectorRecallReady := prepareTurnVectorRecallReady(hydration.Trace)
	hydratedCount := len(hydration.Items)
	hydrationMissCount := intFromAny(hydrationTrace["missing_count"], 0) + intFromAny(hydrationTrace["non_memory_count"], 0)

	out := make([]any, 0, topK)
	seenMemoryIDs := map[int64]bool{}
	identity := s.currentEmbeddingModelIdentity()
	vectorItemByID := searchMemoryItemMapByID(scoredSearchMemoryItems(hydration.Items, req.UserInput, len(hydration.Items), identity))
	for _, mem := range hydration.Items {
		item := copySearchItemMap(vectorItemByID[mem.ID])
		if len(item) == 0 {
			continue
		}
		item["lane"] = "vector_relevant"
		item["retrieval_lane"] = "vector_relevant"
		item["retrieval_mode"] = "vector_first"
		item["vector_hydrated"] = true
		item["vector_truth_boundary"] = "vector_hit_is_selector_only_mariadb_memory_is_canonical"
		if score := hydration.Scores[prepareTurnMemoryLaneKey(mem)]; score > 0 {
			item["vector_rank_score"] = score
		}
		out = append(out, item)
		seenMemoryIDs[mem.ID] = true
		if len(out) >= topK {
			break
		}
	}

	retrievalMode := "vector_degraded_lexical"
	fallbackReason := searchVectorFallbackReason(vectorShadow, hydrationTrace)
	vectorSearchAttempted := boolFromAny(vectorShadow["search_attempted"])
	if vectorRecallReady {
		retrievalMode = "vector_first"
		fallbackReason = ""
	} else if vectorSearchAttempted {
		retrievalMode = "vector_degraded_no_fill"
	}
	vectorSelectedCount := len(out)
	lexicalFillReason := ""
	lexicalFillEnabled := !(vectorRecallReady || vectorSearchAttempted)
	if lexicalFillEnabled && vectorSelectedCount > 0 && vectorSelectedCount < topK {
		lexicalFillReason = "vector_result_count_below_top_k"
	} else if lexicalFillEnabled && vectorSelectedCount == 0 {
		lexicalFillReason = fallbackReason
	}

	if lexicalFillEnabled && len(out) < topK {
		lexicalLimit := minInt(len(memories), topK+len(seenMemoryIDs))
		for _, raw := range scoredSearchMemoryItems(memories, req.UserInput, lexicalLimit, identity) {
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			id := int64(intFromSearchItem(item["id"]))
			if id > 0 && seenMemoryIDs[id] {
				continue
			}
			copyItem := copySearchItemMap(item)
			copyItem["lane"] = "lexical_relevant"
			copyItem["retrieval_lane"] = "lexical_relevant"
			copyItem["retrieval_mode"] = retrievalMode
			copyItem["vector_fallback_reason"] = lexicalFillReason
			out = append(out, copyItem)
			if id > 0 {
				seenMemoryIDs[id] = true
			}
			if len(out) >= topK {
				break
			}
		}
	}

	trace["status"] = "ready"
	trace["retrieval_mode"] = retrievalMode
	trace["fallback_reason"] = nilIfEmpty(fallbackReason)
	trace["lexical_fill_reason"] = nilIfEmpty(lexicalFillReason)
	trace["lexical_fill_enabled"] = lexicalFillEnabled
	trace["vector_recall_ready"] = vectorRecallReady
	trace["vector_recall_attempted"] = vectorSearchAttempted
	trace["vector_shadow"] = vectorShadow
	trace["hydration"] = hydrationTrace
	trace["vector_memory_count"] = hydratedCount
	trace["vector_memory_selected_count"] = len(hydration.Items)
	trace["lexical_memory_count"] = countSearchItemsByLane(out, "lexical_relevant")
	trace["memory_count"] = countSearchItemsBySource(out, "memory")
	trace["hydration_miss_count"] = hydrationMissCount
	trace["search_result"] = stringFromMap(vectorShadow, "search_result")
	trace["search_attempted"] = vectorSearchAttempted
	return out, trace, retrievalMode, fallbackReason
}

func searchMemoryItemMapByID(items []any) map[int64]map[string]any {
	out := map[int64]map[string]any{}
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id := int64(intFromSearchItem(item["id"]))
		if id > 0 {
			out[id] = item
		}
	}
	return out
}

func copySearchItemMap(item map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range item {
		out[key] = value
	}
	return out
}

func searchVectorFallbackReason(vectorShadow map[string]any, hydrationTrace map[string]any) string {
	if vectorShadow == nil {
		return "vector_shadow_missing"
	}
	if intFromAny(hydrationTrace["input_hit_count"], 0) > 0 && intFromAny(hydrationTrace["hydrated_count"], 0) == 0 {
		return "vector_hits_not_hydrated"
	}
	readiness := mapFromAny(vectorShadow["index_readiness"])
	if status := strings.TrimSpace(stringFromMap(readiness, "status")); status != "" && status != "ready" {
		return status
	}
	if reason := strings.TrimSpace(stringFromMap(vectorShadow, "search_skipped_reason")); reason != "" {
		return reason
	}
	if result := strings.TrimSpace(stringFromMap(vectorShadow, "search_result")); result != "" && result != "ok" {
		return "vector_" + result
	}
	if !boolFromAny(vectorShadow["search_attempted"]) {
		return "vector_search_not_attempted"
	}
	return "vector_no_hydrated_memory"
}

func scoredSearchMemoryItems(memories []store.Memory, query string, topK int, embeddingIdentity embeddingModelIdentity) []any {
	terms := searchTerms(query)
	hyTerms := hybridSignalTerms(terms)
	currentModel := embeddingIdentity.Model
	latestTurn := 0
	for _, m := range memories {
		if m.TurnIndex > latestTurn {
			latestTurn = m.TurnIndex
		}
	}
	scored := make([]scoredSearchItem, 0, len(memories))
	for _, m := range memories {
		embeddingStatus := classifyMemoryEmbeddingStatus(m, currentModel)
		text := memorySearchText(m)
		parsed := parseJSONMap(m.SummaryJSON)
		similarity := lexicalSimilarityScore(terms, text)
		importance := normalizedSearchImportance(m.Importance)
		recency := searchRecencyScore(m.TurnIndex, latestTurn)
		keywordTerms := hybridOverlapTerms(hyTerms, text)
		keywordScore := hybridKeywordOverlapScore(len(keywordTerms), len(hyTerms))
		softBias := hybridSoftBiasScore(hyTerms, m, parsed)
		semanticRank := roundSearchScore(similarity)
		hybridBaselineScore := roundSearchScore(semanticRank + keywordScore + softBias.total)
		finalScore := roundSearchScore(similarity*0.55 + importance*0.30 + recency*0.15 + keywordScore + softBias.total)
		item := map[string]any{
			"id":                             m.ID,
			"chat_session_id":                m.ChatSessionID,
			"turn_index":                     m.TurnIndex,
			"summary_json":                   m.SummaryJSON,
			"summary_preview":                memorySummaryPreview(m.SummaryJSON),
			"importance":                     m.Importance,
			"source":                         "memory",
			"similarity_score":               roundSearchScore(similarity),
			"importance_score":               roundSearchScore(importance),
			"recency_score":                  roundSearchScore(recency),
			"final_score":                    finalScore,
			"semantic_rank_score":            semanticRank,
			"keyword_overlap_score":          keywordScore,
			"hybrid_baseline_score":          hybridBaselineScore,
			"keyword_overlap_terms":          keywordTerms,
			"hybrid_baseline_policy_version": hybridKeywordPolicyVersion,
			"speaker_bias_score":             softBias.speakerScore,
			"location_bias_score":            softBias.locationScore,
			"storyline_bias_score":           softBias.storylineScore,
			"soft_bias_score":                softBias.total,
			"soft_bias_policy_version":       hybridSoftBiasPolicyVersion,
			"speaker_bias_terms":             softBias.speakerTerms,
			"location_bias_terms":            softBias.locationTerms,
			"storyline_bias_terms":           softBias.storylineTerms,
			"score_breakdown": map[string]any{
				"similarity": roundSearchScore(similarity),
				"importance": roundSearchScore(importance),
				"recency":    roundSearchScore(recency),
				"hybrid": map[string]any{
					"semantic_rank_score":            semanticRank,
					"keyword_overlap_score":          keywordScore,
					"soft_bias_score":                softBias.total,
					"hybrid_baseline_score":          hybridBaselineScore,
					"hybrid_baseline_policy_version": hybridKeywordPolicyVersion,
					"soft_bias_policy_version":       hybridSoftBiasPolicyVersion,
				},
				"weights": map[string]float64{"similarity": 0.55, "importance": 0.30, "recency": 0.15},
			},
			"embedding_provenance": map[string]any{
				"policy_version":       "em1c.v1",
				"embedding_model":      m.EmbeddingModel,
				"current_model":        currentModel,
				"current_model_source": embeddingIdentity.Source,
				"has_embedding":        strings.TrimSpace(m.Embedding) != "",
				"status":               embeddingStatus,
				"needs_reembed":        memoryEmbeddingNeedsReembed(embeddingStatus),
				"retrieval_fallback":   memoryEmbeddingRetrievalFallback(embeddingStatus),
				"truth_authority":      "store_canonical",
				"vector_role":          "accelerator_only",
			},
		}
		if !m.CreatedAt.IsZero() {
			item["created_at"] = m.CreatedAt.Format(time.RFC3339Nano)
		}
		scored = append(scored, scoredSearchItem{item: item, finalScore: finalScore})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].finalScore == scored[j].finalScore {
			return intFromSearchItem(scored[i].item["turn_index"]) > intFromSearchItem(scored[j].item["turn_index"])
		}
		return scored[i].finalScore > scored[j].finalScore
	})

	// SEQ-18-P65~P69: hy1d.v1 tail-budget rescue pass.
	// Keeps the same n_results budget; promotes at most one near-cutoff
	// candidate when its keyword + soft-bias signal is stronger than the
	// current cutline item.
	if topK > 0 && topK < len(scored) {
		cutlineIdx := topK - 1
		nearCutoffIdx := topK
		cutlineItem := scored[cutlineIdx].item
		nearCutoffItem := scored[nearCutoffIdx].item
		cutlineHybrid := floatFromAny(cutlineItem["keyword_overlap_score"]) + floatFromAny(cutlineItem["soft_bias_score"])
		nearCutoffHybrid := floatFromAny(nearCutoffItem["keyword_overlap_score"]) + floatFromAny(nearCutoffItem["soft_bias_score"])
		if nearCutoffHybrid > cutlineHybrid {
			nearCutoffItem["tail_budget_policy_version"] = "hy1d.v1"
			nearCutoffItem["tail_budget_original_rank"] = nearCutoffIdx + 1 // 1-based
			nearCutoffItem["tail_budget_promoted"] = true
			nearCutoffItem["tail_budget_reason"] = "keyword_soft_bias_stronger_than_cutline"
			nearCutoffItem["tail_budget_score_gap"] = roundSearchScore(nearCutoffHybrid - cutlineHybrid)
			scored[cutlineIdx] = scored[nearCutoffIdx]
		}
	}

	if topK > len(scored) {
		topK = len(scored)
	}
	items := make([]any, 0, topK)
	for _, item := range scored[:topK] {
		items = append(items, item.item)
	}
	return items
}

func scoredFallbackChatLogItems(logs []store.ChatLog, query string, limit int) []any {
	if limit <= 0 {
		return []any{}
	}
	terms := searchTerms(query)
	if len(terms) == 0 {
		return []any{}
	}
	latestTurn := 0
	for _, log := range logs {
		if log.TurnIndex > latestTurn {
			latestTurn = log.TurnIndex
		}
	}
	scored := make([]scoredSearchItem, 0, len(logs))
	for _, log := range logs {
		similarity := lexicalSimilarityScore(terms, log.Content)
		if similarity < 0.65 {
			continue
		}
		recency := searchRecencyScore(log.TurnIndex, latestTurn)
		finalScore := roundSearchScore(similarity*0.65 + recency*0.35)
		item := map[string]any{
			"id":               log.ID,
			"chat_session_id":  log.ChatSessionID,
			"turn_index":       log.TurnIndex,
			"role":             log.Role,
			"content":          log.Content,
			"content_preview":  truncatePlainForPreview(log.Content, 180),
			"source":           "chat_log",
			"similarity_score": roundSearchScore(similarity),
			"importance_score": 0.0,
			"recency_score":    roundSearchScore(recency),
			"final_score":      finalScore,
			"score_breakdown": map[string]any{
				"similarity": roundSearchScore(similarity),
				"importance": 0.0,
				"recency":    roundSearchScore(recency),
				"threshold":  0.65,
				"weights":    map[string]float64{"similarity": 0.65, "recency": 0.35},
			},
		}
		if !log.CreatedAt.IsZero() {
			item["created_at"] = log.CreatedAt.Format(time.RFC3339Nano)
		}
		scored = append(scored, scoredSearchItem{item: item, finalScore: finalScore})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].finalScore == scored[j].finalScore {
			return intFromSearchItem(scored[i].item["turn_index"]) > intFromSearchItem(scored[j].item["turn_index"])
		}
		return scored[i].finalScore > scored[j].finalScore
	})
	if limit > len(scored) {
		limit = len(scored)
	}
	items := make([]any, 0, limit)
	for _, item := range scored[:limit] {
		items = append(items, item.item)
	}
	return items
}

func memorySearchText(m store.Memory) string {
	parts := []string{m.SummaryJSON, m.Evidence, m.PlaceWing, m.PlaceRoom}
	parsed := parseJSONMap(m.SummaryJSON)
	for _, key := range []string{"summary", "turn_summary", "text", "content"} {
		if value := strings.TrimSpace(stringFromAny(parsed[key])); value != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, " ")
}

func searchTerms(query string) []string {
	raw := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool {
		return !(r == '_' || r == '-' || unicode.IsLetter(r) || unicode.IsNumber(r))
	})
	seen := map[string]bool{}
	terms := []string{}
	for _, term := range raw {
		term = strings.TrimSpace(term)
		if len([]rune(term)) < 2 || seen[term] {
			continue
		}
		seen[term] = true
		terms = append(terms, term)
	}
	return terms
}

func lexicalSimilarityScore(terms []string, text string) float64 {
	if len(terms) == 0 {
		return 0.5
	}
	lower := strings.ToLower(text)
	matches := 0
	for _, term := range terms {
		if strings.Contains(lower, term) {
			matches++
		}
	}
	return float64(matches) / float64(len(terms))
}

func hybridSignalTerms(terms []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, term := range terms {
		term = strings.TrimSpace(strings.ToLower(term))
		if term == "" || hybridKeywordStopwords[term] || seen[term] {
			continue
		}
		seen[term] = true
		out = append(out, term)
	}
	return out
}

func hybridOverlapTerms(terms []string, text string) []string {
	if len(terms) == 0 {
		return []string{}
	}
	lower := strings.ToLower(text)
	out := []string{}
	for _, term := range terms {
		if strings.Contains(lower, term) {
			out = append(out, term)
		}
	}
	return out
}

func hybridKeywordOverlapScore(matchCount, termCount int) float64 {
	if matchCount <= 0 || termCount <= 0 {
		return 0
	}
	score := (float64(matchCount) / float64(termCount)) * hybridKeywordScoreCap
	if score > hybridKeywordScoreCap {
		score = hybridKeywordScoreCap
	}
	return roundSearchScore(score)
}

type hybridSoftBiasResult struct {
	speakerScore   float64
	locationScore  float64
	storylineScore float64
	total          float64
	speakerTerms   []string
	locationTerms  []string
	storylineTerms []string
}

func hybridSoftBiasScore(terms []string, m store.Memory, parsed map[string]any) hybridSoftBiasResult {
	speakerText := strings.Join([]string{
		stringFromAny(parsed["speaker"]),
		stringFromAny(parsed["speakers"]),
		stringFromAny(parsed["character"]),
		stringFromAny(parsed["characters"]),
		stringFromAny(parsed["actor"]),
		stringFromAny(parsed["role"]),
	}, " ")
	locationText := strings.Join([]string{
		m.PlaceWing,
		m.PlaceRoom,
		stringFromAny(parsed["location"]),
		stringFromAny(parsed["place"]),
		stringFromAny(parsed["room"]),
		stringFromAny(parsed["wing"]),
		stringFromAny(parsed["setting"]),
	}, " ")
	storylineText := strings.Join([]string{
		stringFromAny(parsed["storyline"]),
		stringFromAny(parsed["storyline_id"]),
		stringFromAny(parsed["arc"]),
		stringFromAny(parsed["chapter"]),
		stringFromAny(parsed["plot"]),
		stringFromAny(parsed["goal"]),
		stringFromAny(parsed["thread"]),
	}, " ")

	speakerTerms := hybridOverlapTerms(terms, speakerText)
	locationTerms := hybridOverlapTerms(terms, locationText)
	storylineTerms := hybridOverlapTerms(terms, storylineText)

	result := hybridSoftBiasResult{
		speakerTerms:   speakerTerms,
		locationTerms:  locationTerms,
		storylineTerms: storylineTerms,
	}
	if len(speakerTerms) > 0 {
		result.speakerScore = hybridSpeakerBiasWeight
	}
	if len(locationTerms) > 0 {
		result.locationScore = hybridLocationBiasWeight
	}
	if len(storylineTerms) > 0 {
		result.storylineScore = hybridStorylineBiasWeight
	}
	result.total = result.speakerScore + result.locationScore + result.storylineScore
	if result.total > hybridSoftBiasCap {
		result.total = hybridSoftBiasCap
	}
	result.speakerScore = roundSearchScore(result.speakerScore)
	result.locationScore = roundSearchScore(result.locationScore)
	result.storylineScore = roundSearchScore(result.storylineScore)
	result.total = roundSearchScore(result.total)
	return result
}

func normalizedSearchImportance(value float64) float64 {
	if value <= 0 {
		return 0
	}
	if value > 1 {
		value = value / 10
	}
	if value > 1 {
		return 1
	}
	return value
}

func searchRecencyScore(turnIndex int, latestTurn int) float64 {
	if turnIndex <= 0 || latestTurn <= 0 {
		return 0.5
	}
	age := latestTurn - turnIndex
	if age <= 0 {
		return 1
	}
	return math.Exp(-float64(age) / 50.0)
}

func roundSearchScore(value float64) float64 {
	if value < 0 {
		value = 0
	}
	if value > 1 {
		value = 1
	}
	return math.Round(value*10000) / 10000
}

func intFromSearchItem(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

func searchInjectionText(items []any) string {
	lines := []string{}
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		summary := strings.TrimSpace(stringFromAny(item["summary_preview"]))
		if summary == "" {
			summary = strings.TrimSpace(stringFromAny(item["summary_json"]))
		}
		if summary == "" {
			summary = strings.TrimSpace(stringFromAny(item["content_preview"]))
		}
		if summary == "" {
			continue
		}
		lines = append(lines, "- "+truncatePlainForPreview(summary, 220))
	}
	return strings.Join(lines, "\n")
}

func countSearchItemsBySource(items []any, source string) int {
	count := 0
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(stringFromAny(item["source"])) == source {
			count++
		}
	}
	return count
}

func countSearchItemsByLane(items []any, lane string) int {
	count := 0
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(stringFromAny(item["lane"])) == lane || strings.TrimSpace(stringFromAny(item["retrieval_lane"])) == lane {
			count++
		}
	}
	return count
}
