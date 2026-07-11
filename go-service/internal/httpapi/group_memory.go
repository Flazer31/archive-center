package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
	"github.com/shirou/gopsutil/v3/disk"
)

// registerMemoryRoutes mounts search, retrieval, explorer, and chroma-shadow.
func (s *Server) registerMemoryRoutes(mux *http.ServeMux) {
	// R1 read-only
	mux.HandleFunc("POST /search", s.handleSearch)
	mux.HandleFunc("GET /retrieval-index/runtime-config", s.handleRetrievalIndexRuntimeConfigGet)
	mux.HandleFunc("GET /intent-routing/runtime-config", s.handleIntentRoutingRuntimeConfigGet)
	mux.HandleFunc("GET /retrieval-index/{chat_session_id}", s.handleRetrievalIndexSnapshot)
	mux.HandleFunc("GET /retrieval-index/{chat_session_id}/source-row", s.handleRetrievalIndexSourceRow)
	mux.HandleFunc("GET /kg/recall", s.handleKGRecallGet)
	mux.HandleFunc("POST /kg/recall", s.handleKGRecall)

	// Chroma shadow: probes
	mux.HandleFunc("GET /chroma-shadow/preflight", s.handleChromaPreflight)

	// Chroma shadow: R1 read/audit
	mux.HandleFunc("POST /chroma-shadow/backfill-dry-run", s.handleChromaBackfillDryRun)
	mux.HandleFunc("POST /chroma-shadow/reembed-audit", s.handleChromaReembedAudit)
	mux.HandleFunc("POST /chroma-shadow/fallback-runbook", s.handleChromaFallbackRunbook)
	mux.HandleFunc("POST /chroma-shadow/release-hygiene", s.handleChromaReleaseHygiene)
	mux.HandleFunc("POST /chroma-shadow/visibility-guard", s.handleChromaVisibilityGuard)
	mux.HandleFunc("POST /chroma-shadow/health-probe", s.handleChromaHealthProbe)

	// Chroma shadow: R2 write
	mux.HandleFunc("POST /chroma-shadow/bootstrap", s.handleChromaBootstrap)
	mux.HandleFunc("POST /chroma-shadow/backfill-batch", s.handleChromaBackfillBatch)
	mux.HandleFunc("POST /chroma-shadow/rebuild-drill", s.handleChromaRebuildDrill)
	mux.HandleFunc("POST /chroma-shadow/adoption-gate", s.handleChromaAdoptionGate)

	// EM-1d: session-level reembed schedule (shadow/dry-run contract)
	mux.HandleFunc("POST /chroma-shadow/reembed-schedule", s.handleChromaReembedSchedule)

	// R2 write
	mux.HandleFunc("POST /retrieval-index/runtime-config", s.handleRetrievalIndexRuntimeConfigPost)
	mux.HandleFunc("POST /intent-routing/runtime-config", s.handleIntentRoutingRuntimeConfigPost)

	// Explorer: R1 read
	mux.HandleFunc("GET /explorer/chat_logs", s.handleExplorerChatLogs)
	mux.HandleFunc("GET /explorer/memories", s.handleExplorerMemories)
	mux.HandleFunc("GET /explorer/direct-evidence", s.handleExplorerDirectEvidence)
	mux.HandleFunc("GET /explorer/kg_triples", s.handleExplorerKGTriples)
	mux.HandleFunc("GET /explorer/chapter_summaries", s.handleExplorerChapterSummaries)
	mux.HandleFunc("GET /explorer/arc_summaries", s.handleExplorerArcSummaries)
	mux.HandleFunc("GET /explorer/saga_digests", s.handleExplorerSagaDigests)

	// Explorer: fake-id 404 parity
	mux.HandleFunc("GET /explorer/{sid}", s.handleExplorerGet404)

	// Explorer: R2 write
	mux.HandleFunc("PATCH /explorer/memories/{memory_id}", s.handlePatchMemory)
	mux.HandleFunc("PATCH /explorer/kg_triples/{triple_id}", s.handlePatchKGTriple)
	mux.HandleFunc("PATCH /explorer/direct-evidence/{record_id}", s.handlePatchEvidenceEdit)
	mux.HandleFunc("PATCH /explorer/direct-evidence/{record_id}/review", s.handlePatchEvidenceReview)
	mux.HandleFunc("PATCH /explorer/direct-evidence/{record_id}/revalidate", s.handlePatchEvidenceRevalidate)
	mux.HandleFunc("PATCH /explorer/direct-evidence/{record_id}/tombstone", s.handlePatchEvidenceTombstone)
	mux.HandleFunc("PATCH /explorer/direct-evidence/{record_id}/supersede", s.handlePatchEvidenceSupersede)
	mux.HandleFunc("POST /explorer/memories/regenerate", s.handleRegenerateMemory)
	mux.HandleFunc("DELETE /explorer/memories/{memory_id}", s.handleDeleteMemory)
	mux.HandleFunc("POST /explorer/memories/{memory_id}/delete", s.handleDeleteMemoryPost)
	mux.HandleFunc("DELETE /explorer/direct-evidence/{record_id}", s.handleDeleteDirectEvidence)
	mux.HandleFunc("POST /explorer/direct-evidence/{record_id}/delete", s.handleDeleteDirectEvidencePost)
	mux.HandleFunc("DELETE /explorer/kg_triples/{triple_id}", s.handleDeleteKGTriple)
	mux.HandleFunc("POST /explorer/kg_triples/{triple_id}/delete", s.handleDeleteKGTriplePost)
}

// R1 read-only: Store/Vector-backed

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

func (s *Server) handleRetrievalIndexRuntimeConfigGet(w http.ResponseWriter, r *http.Request) {
	// Python 0.8 parity: session_count reflects retrieval-index registry state,
	// not total store sessions. R1 Go does not yet maintain a retrieval-index
	// registry, so session_count remains 0 to match empty-registry fixture-live.
	sessionCount := 0
	writeJSON(w, http.StatusOK, map[string]any{
		"mode":                 "shadow",
		"shadow_write_enabled": true,
		"updated_at":           time.Now().UTC().Format(time.RFC3339Nano),
		"reason":               "default",
		"session_count":        sessionCount,
		"index_version":        "q1e.v1",
	})
}
func (s *Server) handleIntentRoutingRuntimeConfigGet(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"mode":            "single_query_shared",
		"updated_at":      time.Now().UTC().Format("2006-01-02 15:04:05"),
		"reason":          "default",
		"version":         "v0c.v1",
		"supported_modes": []string{"single_query_shared", "per_intent_shadow"},
	})
}

func (s *Server) handleRetrievalIndexSnapshot(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}

	documentCount := 0
	status := "empty"
	documents := []map[string]any{}
	documentSchema := map[string]any{
		"version":         "q1a.v1",
		"index_version":   "q1e.v1",
		"document_id":     "tier:id",
		"required_fields": []string{"document_id", "tier", "source_table", "source_row_id", "source_type"},
		"tiers":           []string{"memory", "episode", "chapter", "arc", "saga"},
	}
	addDocument := func(tier string, id int64, sourceTable, sourceType string) {
		documents = append(documents, map[string]any{
			"document_id":   fmt.Sprintf("%s:%d", tier, id),
			"tier":          tier,
			"source_table":  sourceTable,
			"source_row_id": id,
			"source_type":   sourceType,
		})
	}
	sourceTypeCounts := map[string]any{}
	tierCounts := map[string]any{
		"memory":  0,
		"episode": 0,
		"chapter": 0,
		"arc":     0,
		"saga":    0,
	}

	if s.Store != nil {
		memories, err := s.Store.ListMemories(r.Context(), sid, 0, 0)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		if err == nil {
			memCount := len(memories)
			documentCount += memCount
			tierCounts["memory"] = memCount
			if memCount > 0 {
				sourceTypeCounts["memories"] = memCount
			}
			for _, m := range memories {
				addDocument("memory", m.ID, "memories", "memory")
			}
		}

		evidence, err := s.Store.ListEvidence(r.Context(), sid)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		if err == nil {
			evCount := len(evidence)
			documentCount += evCount
			if evCount > 0 {
				sourceTypeCounts["direct_evidence"] = evCount
			}
			for _, e := range evidence {
				addDocument("memory", e.ID, "direct_evidence_records", "direct_evidence")
			}
		}

		kgTriples, err := s.Store.ListKGTriples(r.Context(), sid)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		if err == nil {
			kgCount := len(kgTriples)
			documentCount += kgCount
			if kgCount > 0 {
				sourceTypeCounts["kg_triples"] = kgCount
			}
			for _, t := range kgTriples {
				addDocument("memory", t.ID, "kg_triples", "kg_triple")
			}
		}

		episodes, err := s.Store.ListEpisodeSummaries(r.Context(), sid, 0, 0, 0)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		if err == nil {
			epCount := len(episodes)
			documentCount += epCount
			tierCounts["episode"] = epCount
			if epCount > 0 {
				sourceTypeCounts["episode_summaries"] = epCount
			}
			for _, ep := range episodes {
				addDocument("episode", ep.ID, "episode_summaries", "episode_summary")
			}
		}

		if chapterStore, ok := s.Store.(store.ChapterSummaryStore); ok {
			chapters, err := chapterStore.SearchChapterSummaries(r.Context(), sid, "", 0, 0, 0)
			if err != nil && !errors.Is(err, store.ErrNotEnabled) {
				writeInternalError(w, err.Error())
				return
			}
			if err == nil {
				chapterCount := len(chapters)
				documentCount += chapterCount
				tierCounts["chapter"] = chapterCount
				if chapterCount > 0 {
					sourceTypeCounts["chapter_summaries"] = chapterCount
				}
				for _, ch := range chapters {
					addDocument("chapter", ch.ID, "chapter_summaries", "chapter_summary")
				}
			}
		}

		if arcStore, ok := s.Store.(store.ArcSummaryStore); ok {
			arcs, err := arcStore.ListArcSummaries(r.Context(), sid, "", 0)
			if err != nil && !errors.Is(err, store.ErrNotEnabled) {
				writeInternalError(w, err.Error())
				return
			}
			if err == nil {
				arcCount := len(arcs)
				documentCount += arcCount
				tierCounts["arc"] = arcCount
				if arcCount > 0 {
					sourceTypeCounts["arc_summaries"] = arcCount
				}
				for _, arc := range arcs {
					addDocument("arc", arc.ID, "arc_summaries", "arc_summary")
				}
			}
		}

		if sagaStore, ok := s.Store.(store.SagaDigestStore); ok {
			sagas, err := sagaStore.ListSagaDigests(r.Context(), sid, 0)
			if err != nil && !errors.Is(err, store.ErrNotEnabled) {
				writeInternalError(w, err.Error())
				return
			}
			if err == nil {
				sagaCount := len(sagas)
				documentCount += sagaCount
				tierCounts["saga"] = sagaCount
				if sagaCount > 0 {
					sourceTypeCounts["saga_digests"] = sagaCount
				}
				for _, saga := range sagas {
					addDocument("saga", saga.ID, "saga_digests", "saga_digest")
				}
			}
		}

		chatLogs, err := s.Store.ListChatLogs(r.Context(), sid, 0, 0)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		if err == nil {
			clCount := len(chatLogs)
			if clCount > 0 {
				sourceTypeCounts["chat_logs"] = clCount
			}
			if clCount > 0 && documentCount == 0 {
				status = "ok"
			}
		}

		effectiveInputs := 0
		if len(chatLogs) > 0 && len(chatLogs) <= 200 {
			for _, l := range chatLogs {
				ei, err := s.Store.GetEffectiveInput(r.Context(), sid, l.TurnIndex)
				if err != nil {
					continue
				}
				if ei != nil {
					effectiveInputs++
				}
			}
		}
		if effectiveInputs > 0 {
			sourceTypeCounts["effective_inputs"] = effectiveInputs
		}

		if documentCount > 0 || len(chatLogs) > 0 {
			status = "ok"
		}
	}

	if s.Vector != nil {
		vecCount, err := s.Vector.Count(r.Context(), sid)
		if err == nil && vecCount > 0 {
			sourceTypeCounts["vectors"] = vecCount
		} else if !errors.Is(err, vector.ErrNotEnabled) {
			// Non-NotEnabled errors are informational only; do not fail.
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"chat_session_id":           sid,
		"dirty":                     false,
		"dirty_reason":              nil,
		"dirty_turn":                nil,
		"discard_turn":              nil,
		"document_schema":           documentSchema,
		"documents":                 documents,
		"document_count":            documentCount,
		"index_version":             "q1e.v1",
		"last_dirty_at":             nil,
		"last_discarded_at":         nil,
		"last_event":                nil,
		"last_event_reason":         nil,
		"partition_count":           0,
		"runtime_mode":              "shadow",
		"runtime_reason":            "default",
		"runtime_updated_at":        generatedAt(),
		"session_partitioned":       true,
		"shadow_write_enabled":      true,
		"source_type_counts":        sourceTypeCounts,
		"status":                    status,
		"tier_counts":               tierCounts,
		"retrieval_document_schema": documentSchema,
		"updated_at":                nil,
	})
}

func (s *Server) handleRetrievalIndexSourceRow(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}

	docID := r.URL.Query().Get("document_id")
	if docID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"status": "error",
			"detail": "chat_session_id and document_id are required",
		})
		return
	}

	// Backward-compatible resolution: document_id may be formatted as "type:id"
	parts := strings.SplitN(docID, ":", 2)
	rowType := ""
	var rowID int64
	if len(parts) == 2 {
		rowType = parts[0]
		rowID, _ = strconv.ParseInt(parts[1], 10, 64)
	}

	document := map[string]any(nil)
	sourceRow := map[string]any(nil)
	lookupStatus := "document_not_found"

	if s.Store != nil && rowType != "" && rowID > 0 {
		switch rowType {
		case "memory":
			memories, err := s.Store.ListMemories(r.Context(), sid, 0, 0)
			if err == nil {
				for _, m := range memories {
					if m.ID == rowID {
						document = map[string]any{
							"document_id":   docID,
							"source_table":  "memories",
							"source_row_id": m.ID,
							"tier":          "memory",
							"source_type":   "memory",
						}
						sourceRow = map[string]any{
							"id":              m.ID,
							"chat_session_id": m.ChatSessionID,
							"turn_index":      m.TurnIndex,
							"summary_json":    m.SummaryJSON,
							"importance":      m.Importance,
							"type":            "memory",
						}
						lookupStatus = "ok"
						break
					}
				}
			}
		case "evidence":
			evidence, err := s.Store.ListEvidence(r.Context(), sid)
			if err == nil {
				for _, e := range evidence {
					if e.ID == rowID {
						document = map[string]any{
							"document_id":   docID,
							"source_table":  "direct_evidence_records",
							"source_row_id": e.ID,
							"tier":          "memory",
							"source_type":   "direct_evidence",
						}
						sourceRow = map[string]any{
							"id":              e.ID,
							"chat_session_id": e.ChatSessionID,
							"evidence_kind":   e.EvidenceKind,
							"evidence_text":   e.EvidenceText,
							"archive_state":   e.ArchiveState,
							"type":            "evidence",
						}
						lookupStatus = "ok"
						break
					}
				}
			}
		case "kg_triple":
			triples, err := s.Store.ListKGTriples(r.Context(), sid)
			if err == nil {
				for _, t := range triples {
					if t.ID == rowID {
						document = map[string]any{
							"document_id":   docID,
							"source_table":  "kg_triples",
							"source_row_id": t.ID,
							"tier":          "memory",
							"source_type":   "kg_triple",
						}
						sourceRow = map[string]any{
							"id":              t.ID,
							"chat_session_id": t.ChatSessionID,
							"subject":         t.Subject,
							"predicate":       t.Predicate,
							"object":          t.Object,
							"type":            "kg_triple",
						}
						lookupStatus = "ok"
						break
					}
				}
			}
		case "episode":
			episodes, err := s.Store.ListEpisodeSummaries(r.Context(), sid, 0, 0, 0)
			if err == nil {
				for _, ep := range episodes {
					if ep.ID == rowID {
						document = map[string]any{
							"document_id":   docID,
							"source_table":  "episode_summaries",
							"source_row_id": ep.ID,
							"tier":          "episode",
							"source_type":   "episode_summary",
						}
						sourceRow = map[string]any{
							"id":              ep.ID,
							"chat_session_id": ep.ChatSessionID,
							"from_turn":       ep.FromTurn,
							"to_turn":         ep.ToTurn,
							"summary_text":    ep.SummaryText,
							"key_entities":    ep.KeyEntities,
							"key_events":      ep.KeyEvents,
							"type":            "episode",
						}
						lookupStatus = "ok"
						break
					}
				}
			}
		case "chapter":
			if chapterStore, ok := s.Store.(store.ChapterSummaryStore); ok {
				chapters, err := chapterStore.SearchChapterSummaries(r.Context(), sid, "", 0, 0, 0)
				if err == nil {
					for _, ch := range chapters {
						if ch.ID == rowID {
							document = map[string]any{
								"document_id":   docID,
								"source_table":  "chapter_summaries",
								"source_row_id": ch.ID,
								"tier":          "chapter",
								"source_type":   "chapter_summary",
							}
							sourceRow = map[string]any{
								"id":              ch.ID,
								"chat_session_id": ch.ChatSessionID,
								"from_turn":       ch.FromTurn,
								"to_turn":         ch.ToTurn,
								"chapter_index":   ch.ChapterIndex,
								"chapter_title":   ch.ChapterTitle,
								"summary_text":    ch.SummaryText,
								"resume_text":     ch.ResumeText,
								"type":            "chapter",
							}
							lookupStatus = "ok"
							break
						}
					}
				}
			}
		case "arc":
			if arcStore, ok := s.Store.(store.ArcSummaryStore); ok {
				arcs, err := arcStore.ListArcSummaries(r.Context(), sid, "", 0)
				if err == nil {
					for _, arc := range arcs {
						if arc.ID == rowID {
							document = map[string]any{
								"document_id":   docID,
								"source_table":  "arc_summaries",
								"source_row_id": arc.ID,
								"tier":          "arc",
								"source_type":   "arc_summary",
							}
							sourceRow = map[string]any{
								"id":              arc.ID,
								"chat_session_id": arc.ChatSessionID,
								"from_turn":       arc.FromTurn,
								"to_turn":         arc.ToTurn,
								"arc_index":       arc.ArcIndex,
								"arc_name":        arc.ArcName,
								"arc_status":      arc.ArcStatus,
								"arc_resume_text": arc.ArcResumeText,
								"type":            "arc",
							}
							lookupStatus = "ok"
							break
						}
					}
				}
			}
		case "saga":
			if sagaStore, ok := s.Store.(store.SagaDigestStore); ok {
				sagas, err := sagaStore.ListSagaDigests(r.Context(), sid, 0)
				if err == nil {
					for _, saga := range sagas {
						if saga.ID == rowID {
							document = map[string]any{
								"document_id":   docID,
								"source_table":  "saga_digests",
								"source_row_id": saga.ID,
								"tier":          "saga",
								"source_type":   "saga_digest",
							}
							sourceRow = map[string]any{
								"id":               saga.ID,
								"chat_session_id":  saga.ChatSessionID,
								"from_turn":        saga.FromTurn,
								"to_turn":          saga.ToTurn,
								"era_label":        saga.EraLabel,
								"saga_summary":     saga.SagaSummary,
								"resume_pack_text": saga.ResumePackText,
								"type":             "saga",
							}
							lookupStatus = "ok"
							break
						}
					}
				}
			}
		}
	}

	var sourceTable, sourceRowID, tier, sourceType any
	if document != nil {
		sourceTable = document["source_table"]
		sourceRowID = document["source_row_id"]
		tier = document["tier"]
		sourceType = document["source_type"]
	}

	payload := map[string]any{
		"status":          "ok",
		"lookup_status":   lookupStatus,
		"chat_session_id": sid,
		"document_id":     docID,
		"document":        document,
		"source_ref": map[string]any{
			"source_table":  sourceTable,
			"source_row_id": sourceRowID,
			"tier":          tier,
			"source_type":   sourceType,
		},
		"source_row": sourceRow,
	}

	if lookupStatus != "ok" {
		payload["status"] = "error"
		writeJSON(w, http.StatusNotFound, payload)
		return
	}
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) handleKGRecallGet(w http.ResponseWriter, r *http.Request) {
	sid := r.URL.Query().Get("chat_session_id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}

	items := []any{}
	total := 0

	if strings.TrimSpace(sid) == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":        "ok",
			"items":         items,
			"total":         total,
			"limit":         limit,
			"offset":        offset,
			"count":         0,
			"has_more":      false,
			"legacy_compat": true,
		})
		return
	}

	if s.Store != nil {
		triples, err := s.Store.ListKGTriples(r.Context(), sid)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		if err == nil {
			sort.SliceStable(triples, func(i, j int) bool {
				if triples[i].CreatedAt.Equal(triples[j].CreatedAt) {
					return triples[i].ID > triples[j].ID
				}
				return triples[i].CreatedAt.After(triples[j].CreatedAt)
			})
			total = len(triples)
			start := offset
			if start > len(triples) {
				start = len(triples)
			}
			end := start + limit
			if end > len(triples) {
				end = len(triples)
			}
			for _, t := range triples[start:end] {
				items = append(items, kgTripleExplorerItem(t))
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "ok",
		"items":         items,
		"count":         len(items),
		"total":         total,
		"limit":         limit,
		"offset":        offset,
		"has_more":      offset+len(items) < total,
		"legacy_compat": true,
	})
}

func (s *Server) handleKGRecall(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ChatSessionID string   `json:"chat_session_id"`
		Entities      []string `json:"entities"`
		Limit         int      `json:"limit"`
		CurrentTurn   int      `json:"current_turn"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	sid := body.ChatSessionID
	if sid == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":            "ok",
			"items":             []any{},
			"count":             0,
			"entities_received": 0,
			"entities_sent":     len(body.Entities),
		})
		return
	}

	safeEntities := nonEmptyStrings(body.Entities)
	if len(safeEntities) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":            "ok",
			"items":             []any{},
			"count":             0,
			"entities_received": 0,
		})
		return
	}

	items := []any{}
	expiredFiltered := 0

	if s.Store != nil {
		triples, err := s.Store.ListKGTriples(r.Context(), sid)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		if err == nil {
			sortKGTriplesForPython(triples)
			for _, t := range triples {
				if kgTripleExpiredAtTurn(t, body.CurrentTurn) {
					expiredFiltered++
					continue
				}
				if kgTripleMatchesEntities(t, safeEntities) {
					items = append(items, kgTripleExplorerItem(t))
				}
				if body.Limit > 0 && len(items) >= body.Limit {
					break
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":            "ok",
		"items":             items,
		"count":             len(items),
		"entities_received": len(safeEntities),
		"current_turn":      nullablePositiveInt(body.CurrentTurn),
		"expired_filtered":  expiredFiltered,
	})
}

func kgTripleExpiredAtTurn(t store.KGTriple, currentTurn int) bool {
	return currentTurn > 0 && t.ValidTo > 0 && t.ValidTo < currentTurn
}

// Chroma shadow: R0 probes

func (s *Server) handleChromaPreflight(w http.ResponseWriter, r *http.Request) {
	persistDir := s.Cfg.ChromaShadowPersistDir
	if persistDir == "" {
		persistDir = ".chroma_shadow"
	}
	if abs, err := filepath.Abs(persistDir); err == nil {
		persistDir = abs
	}

	_, err := os.Stat(persistDir)
	exists := err == nil
	writable := false
	if exists {
		f, err := os.CreateTemp(persistDir, ".write_probe_*")
		if err == nil {
			writable = true
			f.Close()
			os.Remove(f.Name())
		}
	}

	diskFree := 0.0
	diskTotal := 0.0
	if exists {
		if usage, err := disk.Usage(persistDir); err == nil && usage != nil {
			diskFree = safeRound2Float(float64(usage.Free) / (1024 * 1024))
			diskTotal = safeRound2Float(float64(usage.Total) / (1024 * 1024))
		}
	}

	provider := s.Cfg.EmbedderProvider
	model := s.Cfg.EmbedderModel
	endpoint := s.Cfg.EmbedderEndpoint
	if provider == "" {
		provider = "voyageai"
	}
	if model == "" {
		model = "voyage-4-large"
	}
	if endpoint == "" {
		endpoint = "https://api.voyageai.com/v1/embeddings"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"step":            "17-C1",
		"ready":           false,
		"issues":          []any{"chromadb_dependency_unavailable"},
		"enabled":         true,
		"collection_name": "archive_center_shadow",
		"embedder_identity": map[string]any{
			"provider": provider,
			"model":    model,
			"endpoint": endpoint,
		},
		"retrieval_document_schema": map[string]any{
			"version":       "q1a.v1",
			"tiers":         []any{"memory", "episode", "chapter", "arc", "saga"},
			"index_version": "q1e.v1",
		},
		"session_partitioning": map[string]any{
			"mode":                 "session_partitioned",
			"session_partitioned":  true,
			"shadow_runtime_mode":  "shadow",
			"shadow_write_enabled": true,
			"active_session_count": 0,
		},
		"persist_directory": map[string]any{
			"path":     persistDir,
			"exists":   exists,
			"writable": writable,
		},
		"disk_budget": map[string]any{
			"budget_mb":      2048,
			"free_mb":        diskFree,
			"total_mb":       diskTotal,
			"target_size_mb": 0.16,
		},
		"dependency": map[string]any{
			"package":   "chromadb",
			"available": false,
			"version":   nil,
			"detail":    "ModuleNotFoundError",
		},
	})
}

// Chroma shadow: R1 read/audit evidence surfaces.

func (s *Server) handleChromaBackfillDryRun(w http.ResponseWriter, r *http.Request) {
	var req dto.ChromaShadowBackfillDryRunRequest
	if err := dto.DecodeWithDefaults(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	storeEnabled := false
	vectorCount := 0
	vectorErr := "unavailable"
	memoryCount := 0
	evidenceCount := 0
	kgTripleCount := 0
	episodeCount := 0

	sid := ""
	if req.ChatSessionID != nil {
		sid = *req.ChatSessionID
	}

	if s.Store != nil && sid != "" {
		storeEnabled = true
		if mems, err := s.Store.ListMemories(r.Context(), sid, 0, 0); err == nil {
			memoryCount = len(mems)
		}
		if evs, err := s.Store.ListEvidence(r.Context(), sid); err == nil {
			evidenceCount = len(evs)
		}
		if kgs, err := s.Store.ListKGTriples(r.Context(), sid); err == nil {
			kgTripleCount = len(kgs)
		}
		if eps, err := s.Store.ListEpisodeSummaries(r.Context(), sid, 0, 0, 0); err == nil {
			episodeCount = len(eps)
		}
	}

	if s.Vector != nil && sid != "" {
		if c, err := s.Vector.Count(r.Context(), sid); err == nil {
			vectorCount = c
			vectorErr = ""
		} else if errors.Is(err, vector.ErrNotEnabled) {
			vectorErr = "not_enabled"
		} else {
			vectorErr = err.Error()
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"note":   "chroma-shadow backfill-dry-run is Store/Vector-backed R1 evidence",
		"evidence": map[string]any{
			"store_enabled":         storeEnabled,
			"memory_count":          memoryCount,
			"evidence_count":        evidenceCount,
			"kg_triple_count":       kgTripleCount,
			"episode_count":         episodeCount,
			"vector_count":          vectorCount,
			"vector_error":          vectorErr,
			"eligible_for_backfill": memoryCount + evidenceCount + kgTripleCount + episodeCount - vectorCount,
			"sync_scope":            "selected_tiers",
			"allowed_tiers":         []string{"memory", "evidence", "kg_triple", "episode"},
			"primary_source":        "canonical_row",
			"vector_role":           "shadow_backfill",
		},
		"counts": map[string]any{
			"memory":    memoryCount,
			"evidence":  evidenceCount,
			"kg_triple": kgTripleCount,
			"episode":   episodeCount,
			"vector":    vectorCount,
		},
		"trace_summary": map[string]any{
			"step":            "17-C1-r1",
			"source":          "shadow",
			"chat_session_id": sid,
		},
	})
}

func (s *Server) handleChromaReembedAudit(w http.ResponseWriter, r *http.Request) {
	var req dto.ChromaShadowReembedAuditRequest
	if err := dto.DecodeWithDefaults(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	sid := ""
	if req.ChatSessionID != nil {
		sid = *req.ChatSessionID
	}

	storeEnabled := false
	memoryCount := 0
	memoriesWithEmbedding := 0
	memoryModels := map[string]int{}
	embeddingIdentity := s.currentEmbeddingModelIdentity()
	currentModel := embeddingIdentity.Model
	statusCounts := map[string]int{}
	needsReembedCount := 0
	evidenceCount := 0
	episodeCount := 0
	vectorCount := 0
	vectorErr := "unavailable"

	if s.Store != nil && sid != "" {
		storeEnabled = true
		if mems, err := s.Store.ListMemories(r.Context(), sid, 0, 0); err == nil {
			memoryCount = len(mems)
			for _, m := range mems {
				status := classifyMemoryEmbeddingStatus(m, currentModel)
				statusCounts[status]++
				if memoryEmbeddingNeedsReembed(status) == true {
					needsReembedCount++
				}
				if strings.TrimSpace(m.Embedding) != "" {
					memoriesWithEmbedding++
				}
				model := m.EmbeddingModel
				if model == "" {
					model = "none"
				}
				memoryModels[model]++
			}
		}
		if evs, err := s.Store.ListEvidence(r.Context(), sid); err == nil {
			evidenceCount = len(evs)
		}
		if eps, err := s.Store.ListEpisodeSummaries(r.Context(), sid, 0, 0, 0); err == nil {
			episodeCount = len(eps)
		}
	}

	if s.Vector != nil && sid != "" {
		if c, err := s.Vector.Count(r.Context(), sid); err == nil {
			vectorCount = c
			vectorErr = ""
		} else if errors.Is(err, vector.ErrNotEnabled) {
			vectorErr = "not_enabled"
		} else {
			vectorErr = err.Error()
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"note":   "chroma-shadow reembed-audit is Store/Vector-backed R1 evidence",
		"evidence": map[string]any{
			"store_enabled":                      storeEnabled,
			"memory_count":                       memoryCount,
			"memories_with_embedding":            memoriesWithEmbedding,
			"memory_embedding_models":            memoryModels,
			"current_project_embedding_model":    currentModel,
			"current_embedding_model_source":     embeddingIdentity.Source,
			"memory_status_counts":               statusCounts,
			"needs_reembed_count":                needsReembedCount,
			"evidence_count":                     evidenceCount,
			"episode_count":                      episodeCount,
			"vector_count":                       vectorCount,
			"vector_error":                       vectorErr,
			"reembed_rule":                       "summary_edit_triggers_upsert",
			"model_switch_replay_policy_version": "em1e.v1",
			"retrieval_fallback_before_reembed":  "hybrid_degrade_or_importance_only",
			"retrieval_state_after_reembed":      "embedding_current",
			"truth_authority":                    "store_canonical",
			"vector_role":                        "accelerator_only",
		},
		"counts": map[string]any{
			"memory":   memoryCount,
			"evidence": evidenceCount,
			"episode":  episodeCount,
			"vector":   vectorCount,
		},
		"trace_summary": map[string]any{
			"step":            "17-C1-r1",
			"source":          "shadow",
			"policy_version":  "em1e.v1",
			"chat_session_id": sid,
		},
	})
}

// handleChromaReembedSchedule implements EM-1d: session-level reembed schedule surface.
// This is a shadow/dry-run contract; it does not execute live reembed.
func (s *Server) handleChromaReembedSchedule(w http.ResponseWriter, r *http.Request) {
	var req dto.ChromaShadowReembedAuditRequest
	if err := dto.DecodeWithDefaults(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	sid := ""
	if req.ChatSessionID != nil {
		sid = *req.ChatSessionID
	}

	embeddingIdentity := s.currentEmbeddingModelIdentity()
	currentModel := embeddingIdentity.Model
	schedule := []map[string]any{}
	candidateCount := 0
	modelMismatchCount := 0
	missingCount := 0

	if s.Store != nil && sid != "" {
		if mems, err := s.Store.ListMemories(r.Context(), sid, 0, 0); err == nil {
			for _, m := range mems {
				status := classifyMemoryEmbeddingStatus(m, currentModel)
				if memoryEmbeddingNeedsReembed(status) == true {
					candidateCount++
					if strings.HasPrefix(status, "missing_embedding") {
						missingCount++
					} else {
						modelMismatchCount++
					}
					schedule = append(schedule, map[string]any{
						"memory_id":            m.ID,
						"turn_index":           m.TurnIndex,
						"status":               status,
						"stored_model":         m.EmbeddingModel,
						"current_model":        currentModel,
						"current_model_source": embeddingIdentity.Source,
						"needs_reembed":        memoryEmbeddingNeedsReembed(status),
						"retrieval_fallback":   memoryEmbeddingRetrievalFallback(status),
						"action":               "dry_run_reembed",
						"truth_authority":      "store_canonical",
					})
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"note":   "chroma-shadow reembed-schedule is Store-backed dry-run contract (EM-1d)",
		"evidence": map[string]any{
			"store_enabled":          s.Store != nil,
			"chat_session_id":        sid,
			"current_model":          currentModel,
			"current_model_source":   embeddingIdentity.Source,
			"candidate_count":        candidateCount,
			"missing_count":          missingCount,
			"model_mismatch_count":   modelMismatchCount,
			"schedule":               schedule,
			"live_execution_allowed": false,
			"truth_authority":        "store_canonical",
			"vector_role":            "accelerator_only",
		},
		"trace_summary": map[string]any{
			"step":            "EM-1d",
			"source":          "shadow",
			"policy_version":  "em1d.v1",
			"chat_session_id": sid,
		},
	})
}

func (s *Server) handleChromaFallbackRunbook(w http.ResponseWriter, r *http.Request) {
	var req dto.ChromaShadowFallbackRunbookRequest
	if err := dto.DecodeWithDefaults(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	sid := ""
	if req.ChatSessionID != nil {
		sid = *req.ChatSessionID
	}

	storeEnabled := false
	var storeStats *store.StatsResult
	vectorAvailable := false
	vectorCount := 0
	vectorErr := "unavailable"

	if s.Store != nil && sid != "" {
		storeEnabled = true
		if st, err := s.Store.Stats(r.Context()); err == nil {
			storeStats = &st
		}
	}

	if s.Vector != nil && sid != "" {
		if c, err := s.Vector.Count(r.Context(), sid); err == nil {
			vectorAvailable = true
			vectorCount = c
			vectorErr = ""
		} else if errors.Is(err, vector.ErrNotEnabled) {
			vectorErr = "not_enabled"
		} else {
			vectorErr = err.Error()
		}
	}

	statsMap := map[string]any{}
	if storeStats != nil {
		statsMap = map[string]any{
			"chat_logs":  storeStats.ChatLogs,
			"memories":   storeStats.Memories,
			"kg_triples": storeStats.KgTriples,
		}
	}

	degradedMode := "canonical_baseline"
	if vectorAvailable {
		degradedMode = "vector_ready"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"note":   "chroma-shadow fallback-runbook is Store/Vector-backed R1 evidence",
		"evidence": map[string]any{
			"store_enabled":             storeEnabled,
			"store_stats":               statsMap,
			"vector_available":          vectorAvailable,
			"vector_count":              vectorCount,
			"vector_error":              vectorErr,
			"fallback_policy":           "store_first_then_vector",
			"degraded_mode":             degradedMode,
			"fail_open_baseline":        true,
			"retrieval_baseline":        "sqlite_canonical",
			"canonical_baseline_source": "sqlite_store",
			"sqlite_canonical_baseline": true,
		},
		"counts": map[string]any{
			"vector": vectorCount,
		},
		"trace_summary": map[string]any{
			"step":            "17-C1-r1",
			"source":          "shadow",
			"chat_session_id": sid,
		},
	})
}

func (s *Server) handleChromaReleaseHygiene(w http.ResponseWriter, r *http.Request) {
	var req dto.ChromaShadowReleaseHygieneRequest
	if err := dto.DecodeWithDefaults(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	sid := ""
	if req.ChatSessionID != nil {
		sid = *req.ChatSessionID
	}

	storeEnabled := false
	memoryCount := 0
	evidenceCount := 0
	tombstonedCount := 0
	kgTripleCount := 0
	chatLogCount := 0

	if s.Store != nil && sid != "" {
		storeEnabled = true
		if mems, err := s.Store.ListMemories(r.Context(), sid, 0, 0); err == nil {
			memoryCount = len(mems)
		}
		if evs, err := s.Store.ListEvidence(r.Context(), sid); err == nil {
			evidenceCount = len(evs)
			for _, e := range evs {
				if e.Tombstoned {
					tombstonedCount++
				}
			}
		}
		if kgs, err := s.Store.ListKGTriples(r.Context(), sid); err == nil {
			kgTripleCount = len(kgs)
		}
		if logs, err := s.Store.ListChatLogs(r.Context(), sid, 0, 0); err == nil {
			chatLogCount = len(logs)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"note":   "chroma-shadow release-hygiene is Store-backed R1 evidence",
		"evidence": map[string]any{
			"store_enabled":       storeEnabled,
			"memory_count":        memoryCount,
			"evidence_count":      evidenceCount,
			"tombstoned_count":    tombstonedCount,
			"kg_triple_count":     kgTripleCount,
			"chat_log_count":      chatLogCount,
			"stale_vector_policy": "tombstone_before_delete",
			"delete_policy":       "canonical_row_first",
			"rollback_policy":     "vector_doc_rollback_with_id",
			"merge_policy":        "merge_stale_vectors_to_tombstone",
		},
		"counts": map[string]any{
			"memory":     memoryCount,
			"evidence":   evidenceCount,
			"tombstoned": tombstonedCount,
			"kg_triple":  kgTripleCount,
			"chat_log":   chatLogCount,
		},
		"trace_summary": map[string]any{
			"step":            "17-C1-r1",
			"source":          "shadow",
			"chat_session_id": sid,
		},
	})
}

func (s *Server) handleChromaVisibilityGuard(w http.ResponseWriter, r *http.Request) {
	var req dto.ChromaShadowVisibilityGuardRequest
	if err := dto.DecodeWithDefaults(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	sid := ""
	if req.ChatSessionID != nil {
		sid = *req.ChatSessionID
	}

	storeEnabled := false
	memoryCount := 0
	evidenceCount := 0
	kgTripleCount := 0
	vectorCount := 0
	vectorErr := "unavailable"
	visibilityGap := 0

	if s.Store != nil && sid != "" {
		storeEnabled = true
		if mems, err := s.Store.ListMemories(r.Context(), sid, 0, 0); err == nil {
			memoryCount = len(mems)
		}
		if evs, err := s.Store.ListEvidence(r.Context(), sid); err == nil {
			evidenceCount = len(evs)
		}
		if kgs, err := s.Store.ListKGTriples(r.Context(), sid); err == nil {
			kgTripleCount = len(kgs)
		}
	}

	if s.Vector != nil && sid != "" {
		if c, err := s.Vector.Count(r.Context(), sid); err == nil {
			vectorCount = c
			vectorErr = ""
		} else if errors.Is(err, vector.ErrNotEnabled) {
			vectorErr = "not_enabled"
		} else {
			vectorErr = err.Error()
		}
	}

	storeTotal := memoryCount + evidenceCount + kgTripleCount
	if vectorErr == "" && storeTotal >= vectorCount {
		visibilityGap = storeTotal - vectorCount
	}

	driftStatus := "aligned"
	if visibilityGap > 0 {
		driftStatus = "drift_detected"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"note":   "chroma-shadow visibility-guard is Store/Vector-backed R1 evidence",
		"evidence": map[string]any{
			"store_enabled":           storeEnabled,
			"memory_count":            memoryCount,
			"evidence_count":          evidenceCount,
			"kg_triple_count":         kgTripleCount,
			"vector_count":            vectorCount,
			"vector_error":            vectorErr,
			"visibility_gap":          visibilityGap,
			"drift_policy":            "shadow_degraded",
			"drift_status":            driftStatus,
			"canonical_count":         storeTotal,
			"canonical_to_vector_gap": visibilityGap,
			"drift_action":            "keep_canonical_baseline",
		},
		"counts": map[string]any{
			"memory":    memoryCount,
			"evidence":  evidenceCount,
			"kg_triple": kgTripleCount,
			"vector":    vectorCount,
		},
		"trace_summary": map[string]any{
			"step":            "17-C1-r1",
			"source":          "shadow",
			"chat_session_id": sid,
		},
	})
}

func (s *Server) handleChromaHealthProbe(w http.ResponseWriter, r *http.Request) {
	var req dto.ChromaShadowHealthProbeRequest
	if err := dto.DecodeWithDefaults(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	sid := ""
	if req.ChatSessionID != nil {
		sid = *req.ChatSessionID
	}

	storeEnabled := false
	storeErr := ""
	vectorHealthStatus := "unavailable"
	vectorCount := 0
	vectorHealth := map[string]any{}

	if s.Store != nil && sid != "" {
		storeEnabled = true
		if _, err := s.Store.ListMemories(r.Context(), sid, 0, 0); err != nil {
			if errors.Is(err, store.ErrNotEnabled) {
				storeErr = "not_enabled"
				storeEnabled = false
			} else {
				storeErr = err.Error()
			}
		}
	}

	if s.Vector != nil {
		if h, err := s.Vector.Health(r.Context()); err == nil {
			vectorHealthStatus = h.Status
			vectorHealth = map[string]any{
				"status":      h.Status,
				"collection":  h.Collection,
				"total_count": h.TotalCount,
				"model_ready": h.ModelReady,
			}
			if sid != "" {
				if c, cerr := s.Vector.Count(r.Context(), sid); cerr == nil {
					vectorCount = c
				}
			}
		} else if errors.Is(err, vector.ErrNotEnabled) {
			vectorHealthStatus = "not_enabled"
		} else {
			vectorHealthStatus = "error"
			vectorHealth["error"] = err.Error()
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"note":   "chroma-shadow health-probe is Store/Vector-backed R1 evidence",
		"evidence": map[string]any{
			"store_enabled":        storeEnabled,
			"store_error":          storeErr,
			"vector_health_status": vectorHealthStatus,
			"vector_count":         vectorCount,
			"vector_health":        vectorHealth,
		},
		"counts": map[string]any{
			"vector": vectorCount,
		},
		"trace_summary": map[string]any{
			"step":            "17-C1-r1",
			"source":          "shadow",
			"chat_session_id": sid,
		},
	})
}

func (s *Server) handleChromaBootstrap(w http.ResponseWriter, r *http.Request) {
	writeShadowGuard(w, "POST /chroma-shadow/bootstrap")
}

func (s *Server) handleChromaBackfillBatch(w http.ResponseWriter, r *http.Request) {
	writeShadowGuard(w, "POST /chroma-shadow/backfill-batch")
}

func (s *Server) handleChromaRebuildDrill(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusServiceUnavailable, map[string]any{
		"status": "error",
		"code":   CodeShadowGuard,
		"error":  "POST /chroma-shadow/rebuild-drill is not available in R0/R1 shadow mode",
		"trace_summary": map[string]any{
			"step":          "17-C1-r1",
			"source":        "shadow",
			"rebuild_owner": "chroma_shadow_orchestrator",
			"rebuild_modes": []string{"targeted", "partial", "full"},
		},
	})
}

func (s *Server) handleChromaAdoptionGate(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":               "ok",
		"code":                 "chroma_adoption_gate_closed",
		"note":                 "chroma-shadow adoption-gate is closed in R1 shadow mode",
		"live_cutover_allowed": false,
		"cutover_prerequisites": []string{
			"vector_health_green",
			"visibility_gap_zero",
			"fallback_rate_acceptable",
		},
		"required_green_gates": []string{
			"health_probe",
			"visibility_guard",
			"fallback_runbook",
		},
		"multi_tier_cutover_scope":  "memory_only",
		"adoption_gate_state":       "closed",
		"owner_decision_state":      "pending_pre_12_5",
		"scope_truth_authority":     "store_canonical_truth",
		"long_memory_input_quality": "requires_replay_green",
		"future_125_owner_decision": map[string]any{
			"owner_decision_state":      "pending_pre_12_5",
			"scope_truth_authority":     "store_canonical_truth",
			"long_memory_input_quality": "requires_replay_green",
			"required_green_gates": []string{
				"sync_replay_gate",
				"stale_vector_rollback_rebuild_gate",
				"fail_open_sqlite_baseline_gate",
			},
		},
		"trace_summary": map[string]any{
			"step":   "17-C1-r1",
			"source": "shadow",
		},
	})
}

// Retrieval config: R2 write guards

func (s *Server) handleRetrievalIndexRuntimeConfigPost(w http.ResponseWriter, r *http.Request) {
	writeShadowGuard(w, "POST /retrieval-index/runtime-config")
}

func (s *Server) handleIntentRoutingRuntimeConfigPost(w http.ResponseWriter, r *http.Request) {
	var req map[string]any
	if err := dto.DecodeWithDefaults(r.Body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "bad_request", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"source":          "go_r1_read_shadow",
		"routing_version": "p58a.v1",
		"routing_mode":    "per_intent_shadow",
		"default_route":   "single_query_shared",
		"intents": []map[string]any{
			{"intent": "scene", "tiers": []string{"episode", "memory", "chapter"}, "budget_share": 0.34},
			{"intent": "callback", "tiers": []string{"arc", "saga", "memory"}, "budget_share": 0.22},
			{"intent": "resume", "tiers": []string{"chapter", "arc", "saga"}, "budget_share": 0.28},
			{"intent": "canon", "tiers": []string{"memory", "episode", "arc"}, "budget_share": 0.16},
		},
		"budget_policy": map[string]any{
			"budget_mode":    "policy_only",
			"degrade_policy": "drop_low_score_then_shorten_text",
		},
		"trace": map[string]any{
			"intent_route": "single_query_shared",
			"shadow_ready": true,
		},
	})
}

// Explorer read: Store-backed

func (s *Server) handleExplorerChatLogs(w http.ResponseWriter, r *http.Request) {
	sid := r.URL.Query().Get("chat_session_id")
	fromTurn, _ := strconv.Atoi(r.URL.Query().Get("from_turn"))
	toTurn, _ := strconv.Atoi(r.URL.Query().Get("to_turn"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}

	var logs []store.ChatLog
	if s.Store != nil {
		result, err := s.Store.ListChatLogs(r.Context(), sid, fromTurn, toTurn)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		if err == nil {
			logs = result
		}
	}
	sort.SliceStable(logs, func(i, j int) bool {
		if logs[i].TurnIndex == logs[j].TurnIndex {
			return logs[i].ID > logs[j].ID
		}
		return logs[i].TurnIndex > logs[j].TurnIndex
	})

	items := []any{}
	for i, l := range logs {
		if i < offset {
			continue
		}
		if len(items) >= limit {
			break
		}
		preview := pythonTextPreview(l.Content, 120)
		items = append(items, map[string]any{
			"id":              l.ID,
			"chat_session_id": l.ChatSessionID,
			"turn_index":      l.TurnIndex,
			"role":            l.Role,
			"content":         l.Content,
			"preview":         preview,
			"created_at":      formatKSTTime(l.CreatedAt),
		})
	}

	total := len(logs)
	hasMore := offset+len(items) < total

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"items":    items,
		"total":    total,
		"has_more": hasMore,
		"limit":    limit,
		"offset":   offset,
	})
}

func pythonTextPreview(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "..."
}

func (s *Server) handleExplorerMemories(w http.ResponseWriter, r *http.Request) {
	sid := r.URL.Query().Get("chat_session_id")
	fromTurn, _ := strconv.Atoi(r.URL.Query().Get("from_turn"))
	toTurn, _ := strconv.Atoi(r.URL.Query().Get("to_turn"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}

	items := []any{}
	total := 0

	if s.Store != nil {
		memories, err := s.Store.ListMemories(r.Context(), sid, fromTurn, toTurn)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		if err == nil {
			sort.SliceStable(memories, func(i, j int) bool {
				if memories[i].TurnIndex != memories[j].TurnIndex {
					return memories[i].TurnIndex > memories[j].TurnIndex
				}
				if memories[i].CreatedAt.Equal(memories[j].CreatedAt) {
					return memories[i].ID > memories[j].ID
				}
				return memories[i].CreatedAt.After(memories[j].CreatedAt)
			})
			total = len(memories)
			start := offset
			if start > len(memories) {
				start = len(memories)
			}
			end := start + limit
			if end > len(memories) {
				end = len(memories)
			}
			for _, m := range memories[start:end] {
				items = append(items, map[string]any{
					"id":                     m.ID,
					"chat_session_id":        m.ChatSessionID,
					"source_turn":            m.TurnIndex,
					"summary_json":           m.SummaryJSON,
					"summary_preview":        memorySummaryPreview(m.SummaryJSON),
					"importance":             m.Importance,
					"emotional_intensity":    nullableFloatZero(m.EmotionalIntensity),
					"narrative_significance": nullableFloatZero(m.NarrativeSignificance),
					"emotional_boost":        nullableFloatZero(m.EmotionalBoost),
					"evidence":               m.Evidence,
					"archive_wing":           m.PlaceWing,
					"archive_room":           m.PlaceRoom,
					"embedding_model":        m.EmbeddingModel,
					"created_at":             formatKSTTime(m.CreatedAt),
				})
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"items":    items,
		"total":    total,
		"has_more": offset+len(items) < total,
		"limit":    limit,
		"offset":   offset,
	})
}

func (s *Server) handleExplorerDirectEvidence(w http.ResponseWriter, r *http.Request) {
	sid := r.URL.Query().Get("chat_session_id")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	limit := 30
	offset := 0
	if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
		limit = v
	}
	if v, err := strconv.Atoi(offsetStr); err == nil && v >= 0 {
		offset = v
	}

	items := []any{}
	total := 0
	latestTurnIndex := 0
	stateCounts := map[string]int{}
	auditRows := []store.AuditLog{}

	if s.Store != nil {
		evidence, err := s.Store.ListEvidence(r.Context(), sid)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		if err == nil {
			sort.SliceStable(evidence, func(i, j int) bool {
				if evidence[i].CreatedAt.Equal(evidence[j].CreatedAt) {
					return evidence[i].ID > evidence[j].ID
				}
				return evidence[i].CreatedAt.After(evidence[j].CreatedAt)
			})
			total = len(evidence)

			if sid != "" {
				logs, logErr := s.Store.ListChatLogs(r.Context(), sid, 0, 0)
				if logErr == nil {
					for _, l := range logs {
						if l.TurnIndex > latestTurnIndex {
							latestTurnIndex = l.TurnIndex
						}
					}
				}
			}
			start := offset
			if start > len(evidence) {
				start = len(evidence)
			}
			end := start + limit
			if end > len(evidence) {
				end = len(evidence)
			}
			page := evidence[start:end]
			for _, e := range page {
				bucket := directEvidenceArchiveBucket(
					normalizeDirectEvidenceArchiveState(e.ArchiveState),
					normalizeDirectEvidenceCaptureVerification(e.CaptureVerification),
					e.RepairNeeded,
				)
				stateCounts[bucket]++
				items = append(items, directEvidenceExplorerItem(e, latestTurnIndex))
			}
		}

		if sid != "" {
			audits, auditErr := s.Store.ListAuditLogs(r.Context(), sid, "", 1000)
			if auditErr != nil && !errors.Is(auditErr, store.ErrNotEnabled) {
				writeInternalError(w, auditErr.Error())
				return
			}
			if auditErr == nil {
				auditRows = audits
			}
		}
	}

	stateCountsAny := directEvidenceStateCounts(stateCounts)
	var latestTurn any
	if latestTurnIndex > 0 {
		latestTurn = latestTurnIndex
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":            "ok",
		"items":             items,
		"total":             total,
		"has_more":          total > offset+len(items),
		"limit":             limit,
		"offset":            offset,
		"latest_turn_index": latestTurn,
		"state_contract":    directEvidenceStateContract(),
		"state_counts":      stateCountsAny,
		"cost_measurement":  directEvidenceCostMeasurement(stateCounts, auditRows),
	})
}

func directEvidenceStateContract() map[string]any {
	return map[string]any{
		"archive_states":                     []string{"pending_capture", "verified_direct", "previous_archive", "repair_queue"},
		"capture_verifications":              []string{"pending", "verified", "rejected", "needs_review"},
		"committed_gates":                    []string{"finalize", "recovery", "manual"},
		"conflict_resolution_policy_version": "ea1h.v1",
		"conflict_confidence_policy_version": "ea1i.v1",
		"conflict_classes":                   []string{"state_transition", "hard_contradiction", "parallel_context", "low_confidence_noise"},
		"conflict_routes":                    []string{"superseded", "tombstone", "hold", "manual_review"},
		"conflict_confidence_thresholds": map[string]any{
			"auto_promote_min":                  0.82,
			"hold_below":                        0.55,
			"high_impact_manual_review_below":   0.9,
			"user_confirmation_candidate_below": 0.9,
		},
		"conflict_high_impact_field_classes":            []string{"identity", "relationship", "trust", "world_rule", "canonical_fact"},
		"cost_measurement_policy_version":               "lc1a.v1",
		"deleted_turn_tombstone_retention_window_turns": 240,
		"retention_importance_tiers":                    []string{"critical", "high", "medium", "low"},
		"retention_policy_version":                      "ea1l.v1",
		"retention_windows_turns": map[string]any{
			"direct_evidence":  map[string]any{"critical": 720, "high": 480, "medium": 320, "low": 180},
			"previous_archive": map[string]any{"critical": 540, "high": 360, "medium": 240, "low": 160},
			"tombstone":        map[string]any{"critical": 480, "high": 320, "medium": 240, "low": 240},
		},
	}
}

func directEvidenceStateCounts(counts map[string]int) map[string]any {
	out := map[string]any{
		"pending_capture":  0,
		"verified_direct":  0,
		"previous_archive": 0,
		"repair_queue":     0,
	}
	for key, value := range counts {
		if _, ok := out[key]; ok {
			out[key] = value
		}
	}
	return out
}

func directEvidenceCostMeasurement(stateCounts map[string]int, auditRows []store.AuditLog) map[string]any {
	measurement := map[string]any{
		"policy_version":    "lc1a.v1",
		"audit_window_size": 200,
		"direct_evidence_write": map[string]any{
			"sample_count":    0,
			"avg_latency_ms":  0.0,
			"p95_latency_ms":  0.0,
			"last_latency_ms": 0.0,
			"avg_inserted":    0.0,
			"avg_skipped":     0.0,
			"avg_write_chars": 0.0,
		},
		"repair_queue": map[string]any{
			"queue_count":                stateCounts["repair_queue"],
			"review_sample_count":        0,
			"revalidate_sample_count":    0,
			"avg_review_latency_ms":      0.0,
			"avg_revalidate_latency_ms":  0.0,
			"last_revalidate_latency_ms": 0.0,
		},
	}

	if len(auditRows) == 0 {
		return measurement
	}

	sort.SliceStable(auditRows, func(i, j int) bool {
		return auditRows[i].ID > auditRows[j].ID
	})
	if len(auditRows) > 200 {
		auditRows = auditRows[:200]
	}

	writeLatencies := []float64{}
	writeInserted := []float64{}
	writeSkipped := []float64{}
	writeChars := []float64{}
	reviewLatencies := []float64{}
	revalidateLatencies := []float64{}

	for _, row := range auditRows {
		switch row.EventType {
		case "critic_ingest_trace":
			details := parseJSONMap(row.DetailsJSON)
			if strings.TrimSpace(stringFromAny(details["surface"])) != "direct_evidence" {
				continue
			}
			trace, _ := details["trace"].(map[string]any)
			latency := floatFromAny(trace["elapsed_ms"])
			writeLatencies = append(writeLatencies, latency)
			writeInserted = append(writeInserted, floatFromAny(trace["inserted"]))
			writeSkipped = append(writeSkipped, floatFromAny(trace["skipped"]))
			writeChars = append(writeChars, floatFromAny(trace["write_chars"]))
		case "direct_evidence_review":
			details := parseJSONMap(row.DetailsJSON)
			cost, _ := details["cost_measurement"].(map[string]any)
			reviewLatencies = append(reviewLatencies, floatFromAny(cost["latency_ms"]))
		case "direct_evidence_revalidate":
			details := parseJSONMap(row.DetailsJSON)
			cost, _ := details["cost_measurement"].(map[string]any)
			revalidateLatencies = append(revalidateLatencies, floatFromAny(cost["latency_ms"]))
		}
	}

	if len(writeLatencies) > 0 {
		measurement["direct_evidence_write"] = map[string]any{
			"sample_count":    len(writeLatencies),
			"avg_latency_ms":  safeMeanFloat(writeLatencies),
			"p95_latency_ms":  safeP95Float(writeLatencies),
			"last_latency_ms": safeRoundFloat(writeLatencies[0]),
			"avg_inserted":    safeMeanFloat(writeInserted),
			"avg_skipped":     safeMeanFloat(writeSkipped),
			"avg_write_chars": safeMeanFloat(writeChars),
		}
	}

	repairQueue, _ := measurement["repair_queue"].(map[string]any)
	if len(reviewLatencies) > 0 {
		repairQueue["review_sample_count"] = len(reviewLatencies)
		repairQueue["avg_review_latency_ms"] = safeMeanFloat(reviewLatencies)
	}
	if len(revalidateLatencies) > 0 {
		repairQueue["revalidate_sample_count"] = len(revalidateLatencies)
		repairQueue["avg_revalidate_latency_ms"] = safeMeanFloat(revalidateLatencies)
		repairQueue["last_revalidate_latency_ms"] = safeRoundFloat(revalidateLatencies[0])
	}
	measurement["repair_queue"] = repairQueue
	return measurement
}

func stringFromAny(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func floatFromAny(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case int32:
		return float64(n)
	case json.Number:
		out, err := n.Float64()
		if err == nil {
			return out
		}
	case string:
		out, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		if err == nil {
			return out
		}
	}
	return 0
}

func safeRoundFloat(v float64) float64 {
	return math.Round(v*1000) / 1000
}

func safeMeanFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, v := range values {
		total += v
	}
	return safeRoundFloat(total / float64(len(values)))
}

func safeP95Float(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sortedValues := append([]float64(nil), values...)
	sort.Float64s(sortedValues)
	idx := int(float64(len(sortedValues)-1) * 0.95)
	return safeRoundFloat(sortedValues[idx])
}

func directEvidenceExplorerItem(e store.DirectEvidence, latestTurnIndex int) map[string]any {
	sourceIDs := parseJSONList(e.SourceMessageIDsJSON)
	lineage := parseJSONMap(e.LineageJSON)
	normalizedArchiveState := normalizeDirectEvidenceArchiveState(e.ArchiveState)
	normalizedCaptureVerification := normalizeDirectEvidenceCaptureVerification(e.CaptureVerification)
	normalizedCommittedGate := resolveDirectEvidenceCommittedGate(normalizedCaptureVerification, e.RepairNeeded, e.CommittedGate)
	retentionTier := directEvidenceRetentionTier(normalizedArchiveState, normalizedCaptureVerification, e.RepairNeeded, e.CommittedGate, e.Tombstoned, lineage)
	retentionTTL := directEvidenceRetentionTTL(normalizedArchiveState, retentionTier, e.Tombstoned)
	retentionExpired := directEvidenceRetentionExpired(e.SourceTurnEnd, latestTurnIndex, retentionTTL)
	tombstoneRetained := directEvidenceTombstoneRetained(e.Tombstoned, e.SourceTurnEnd, latestTurnIndex)
	conflictResolution := directEvidenceConflictResolution(e, lineage, normalizedArchiveState, normalizedCaptureVerification, normalizedCommittedGate, retentionTier)
	return map[string]any{
		"id":                                 e.ID,
		"chat_session_id":                    e.ChatSessionID,
		"evidence_kind":                      e.EvidenceKind,
		"evidence_text":                      e.EvidenceText,
		"evidence_preview":                   truncateForPreview(e.EvidenceText, 120),
		"source_turn_start":                  e.SourceTurnStart,
		"source_turn_end":                    e.SourceTurnEnd,
		"turn_anchor":                        nullablePositiveInt(e.TurnAnchor),
		"source_message_ids_json":            nullableString(e.SourceMessageIDsJSON),
		"source_message_ids":                 sourceIDs,
		"source_hash":                        nullableString(e.SourceHash),
		"archive_state":                      e.ArchiveState,
		"normalized_archive_state":           normalizedArchiveState,
		"archive_bucket":                     directEvidenceArchiveBucket(normalizedArchiveState, normalizedCaptureVerification, e.RepairNeeded),
		"capture_stage":                      e.CaptureStage,
		"capture_verification":               e.CaptureVerification,
		"normalized_capture_verification":    normalizedCaptureVerification,
		"committed_gate":                     nullableString(e.CommittedGate),
		"normalized_committed_gate":          normalizedCommittedGate,
		"lineage_json":                       nullableString(e.LineageJSON),
		"lineage":                            lineage,
		"repair_needed":                      e.RepairNeeded,
		"tombstoned":                         e.Tombstoned,
		"superseded_by_id":                   nullableInt64(e.SupersededByID),
		"excluded_from_current_truth":        e.Tombstoned || e.SupersededByID > 0,
		"tombstone_retained_in_window":       tombstoneRetained,
		"tombstone_retention_expired":        e.Tombstoned && !tombstoneRetained,
		"retention_policy_version":           "ea1l.v1",
		"retention_importance_tier":          retentionTier,
		"retention_ttl_turns":                retentionTTL,
		"retention_expired":                  retentionExpired,
		"retention_blocked_from_consumption": retentionExpired,
		"conflict_resolution_policy_version": "ea1h.v1",
		"conflict_confidence_policy_version": "ea1i.v1",
		"conflict_resolution":                conflictResolution,
		"created_at":                         formatKSTTime(e.CreatedAt),
	}
}

func parseJSONList(raw string) []any {
	text := strings.TrimSpace(raw)
	if text == "" {
		return []any{}
	}
	var out []any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return []any{}
	}
	if out == nil {
		return []any{}
	}
	return out
}

func parseJSONMap(raw string) map[string]any {
	text := strings.TrimSpace(raw)
	if text == "" {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
}

func normalizeDirectEvidenceArchiveState(value string) string {
	v := strings.TrimSpace(strings.ToLower(value))
	switch v {
	case "verified_direct", "previous_archive", "repair_queue", "pending_capture":
		return v
	case "committed", "direct_evidence", "verified":
		return "verified_direct"
	case "":
		return "pending_capture"
	default:
		return v
	}
}

func normalizeDirectEvidenceCaptureVerification(value string) string {
	v := strings.TrimSpace(strings.ToLower(value))
	switch v {
	case "verified", "rejected", "needs_review", "pending":
		return v
	case "":
		return "pending"
	default:
		return v
	}
}

func resolveDirectEvidenceCommittedGate(captureVerification string, repairNeeded bool, committedGate string) any {
	if strings.TrimSpace(committedGate) != "" {
		return strings.TrimSpace(committedGate)
	}
	if repairNeeded {
		return "recovery"
	}
	if captureVerification == "verified" {
		return "finalize"
	}
	return nil
}

func directEvidenceArchiveBucket(archiveState, captureVerification string, repairNeeded bool) string {
	if repairNeeded || captureVerification == "rejected" || captureVerification == "needs_review" {
		return "repair_queue"
	}
	return archiveState
}

func directEvidenceRetentionTier(archiveState, captureVerification string, repairNeeded bool, committedGate string, tombstoned bool, lineage map[string]any) string {
	for _, key := range []string{"importance_tier", "retention_tier", "importance"} {
		if tier, ok := lineage[key].(string); ok {
			normalized := strings.TrimSpace(strings.ToLower(tier))
			if normalized == "critical" || normalized == "high" || normalized == "medium" || normalized == "low" {
				return normalized
			}
		}
	}
	for _, marker := range []string{"force_retain", "high_impact", "user_confirmation_candidate", "manual_review_required"} {
		if val, ok := lineage[marker].(bool); ok && val {
			return "critical"
		}
	}
	normalizedGate := resolveDirectEvidenceCommittedGate(captureVerification, repairNeeded, committedGate)
	if gateStr, ok := normalizedGate.(string); ok && gateStr == "manual" {
		return "high"
	}
	if tombstoned {
		return "low"
	}
	if archiveState == "verified_direct" && captureVerification == "verified" && !repairNeeded {
		if gateStr, ok := normalizedGate.(string); ok && gateStr == "manual" {
			return "high"
		}
		return "medium"
	}
	if archiveState == "previous_archive" {
		return "medium"
	}
	return "low"
}

func directEvidenceRetentionTTL(archiveState, tier string, tombstoned bool) int {
	if tombstoned {
		return 240
	}
	switch tier {
	case "critical":
		if archiveState == "previous_archive" {
			return 540
		}
		return 720
	case "high":
		if archiveState == "previous_archive" {
			return 360
		}
		return 480
	case "low":
		if archiveState == "previous_archive" {
			return 160
		}
		return 180
	default:
		if archiveState == "previous_archive" {
			return 240
		}
		return 320
	}
}

func directEvidenceConflictResolution(e store.DirectEvidence, lineage map[string]any, archiveState, captureVerification string, committedGate any, retentionTier string) map[string]any {
	confidence := directEvidenceConflictConfidence(lineage, captureVerification, e.RepairNeeded)
	fieldClass := directEvidenceConflictFieldClass(lineage)
	highImpact := directEvidenceConflictHighImpact(lineage, fieldClass)
	classification := directEvidenceConflictClassification(e, lineage, archiveState, captureVerification, confidence)
	route := directEvidenceConflictRoute(e, captureVerification, classification, confidence, highImpact)
	requiresManualReview := route == "manual_review"
	userConfirmationCandidate := highImpact && confidence < 0.9 && (classification == "hard_contradiction" || classification == "state_transition")
	return map[string]any{
		"policy_version":               "ea1h.v1",
		"confidence_policy_version":    "ea1i.v1",
		"classification":               classification,
		"route":                        route,
		"confidence":                   safeRoundFloat(confidence),
		"field_class":                  fieldClass,
		"high_impact":                  highImpact,
		"requires_manual_review":       requiresManualReview,
		"user_confirmation_candidate":  userConfirmationCandidate,
		"archive_state":                archiveState,
		"capture_verification":         captureVerification,
		"committed_gate":               committedGate,
		"retention_importance_tier":    retentionTier,
		"threshold_auto_promote_min":   0.82,
		"threshold_hold_below":         0.55,
		"threshold_high_impact_review": 0.9,
	}
}

func directEvidenceConflictConfidence(lineage map[string]any, captureVerification string, repairNeeded bool) float64 {
	for _, key := range []string{"conflict_confidence", "confidence", "score"} {
		if value, ok := lineage[key]; ok {
			n := floatFromAny(value)
			if n > 0 {
				if n > 1 {
					n = n / 100
				}
				if n > 1 {
					n = 1
				}
				return n
			}
		}
	}
	if repairNeeded || captureVerification == "needs_review" || captureVerification == "rejected" {
		return 0.35
	}
	if captureVerification == "verified" {
		return 0.86
	}
	return 0.45
}

func directEvidenceConflictFieldClass(lineage map[string]any) string {
	for _, key := range []string{"field_class", "conflict_field_class", "target_field"} {
		if value, ok := lineage[key].(string); ok {
			normalized := strings.TrimSpace(strings.ToLower(value))
			if normalized != "" {
				return normalized
			}
		}
	}
	return "canonical_fact"
}

func directEvidenceConflictHighImpact(lineage map[string]any, fieldClass string) bool {
	for _, key := range []string{"high_impact", "manual_review_required", "user_confirmation_candidate"} {
		if value, ok := lineage[key].(bool); ok && value {
			return true
		}
	}
	switch fieldClass {
	case "identity", "relationship", "trust", "world_rule", "canonical_fact":
		return true
	default:
		return false
	}
}

func directEvidenceConflictClassification(e store.DirectEvidence, lineage map[string]any, archiveState, captureVerification string, confidence float64) string {
	if raw, ok := lineage["conflict_class"].(string); ok {
		normalized := strings.TrimSpace(strings.ToLower(raw))
		switch normalized {
		case "state_transition", "hard_contradiction", "parallel_context", "low_confidence_noise":
			return normalized
		}
	}
	if e.Tombstoned || e.SupersededByID > 0 || captureVerification == "rejected" {
		return "hard_contradiction"
	}
	if archiveState == "previous_archive" {
		return "parallel_context"
	}
	if e.RepairNeeded || captureVerification == "needs_review" || confidence < 0.55 {
		return "low_confidence_noise"
	}
	return "state_transition"
}

func directEvidenceConflictRoute(e store.DirectEvidence, captureVerification, classification string, confidence float64, highImpact bool) string {
	if e.Tombstoned {
		return "tombstone"
	}
	if e.SupersededByID > 0 {
		return "superseded"
	}
	if e.RepairNeeded || captureVerification == "needs_review" || captureVerification == "rejected" {
		return "manual_review"
	}
	switch classification {
	case "hard_contradiction":
		if highImpact && confidence < 0.9 {
			return "manual_review"
		}
		if confidence >= 0.82 {
			return "superseded"
		}
		return "hold"
	case "low_confidence_noise", "parallel_context":
		return "hold"
	default:
		if confidence < 0.55 {
			return "hold"
		}
		return "superseded"
	}
}

func directEvidenceRetentionExpired(sourceTurnEnd, latestTurnIndex, ttl int) bool {
	if sourceTurnEnd <= 0 || latestTurnIndex <= 0 || ttl <= 0 {
		return false
	}
	return latestTurnIndex-sourceTurnEnd > ttl
}

func directEvidenceTombstoneRetained(tombstoned bool, sourceTurnEnd, latestTurnIndex int) bool {
	if !tombstoned {
		return false
	}
	if sourceTurnEnd <= 0 || latestTurnIndex <= 0 {
		return true
	}
	return latestTurnIndex-sourceTurnEnd <= 240
}

func sortKGTriplesForPython(triples []store.KGTriple) {
	sort.SliceStable(triples, func(i, j int) bool {
		if triples[i].CreatedAt.Equal(triples[j].CreatedAt) {
			return triples[i].ID > triples[j].ID
		}
		return triples[i].CreatedAt.After(triples[j].CreatedAt)
	})
}

func kgTripleExplorerItem(t store.KGTriple) map[string]any {
	return map[string]any{
		"id":              t.ID,
		"chat_session_id": t.ChatSessionID,
		"subject":         t.Subject,
		"predicate":       t.Predicate,
		"object":          t.Object,
		"valid_from":      nullablePositiveInt(t.ValidFrom),
		"valid_to":        nullablePositiveInt(t.ValidTo),
		"source_turn":     nullablePositiveInt(t.SourceTurn),
		"created_at":      formatKSTTime(t.CreatedAt),
	}
}

func nonEmptyStrings(items []string) []string {
	out := []string{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	if len(out) > 30 {
		return out[:30]
	}
	return out
}

func kgTripleMatchesEntities(t store.KGTriple, entities []string) bool {
	subject := strings.ToLower(t.Subject)
	object := strings.ToLower(t.Object)
	subjectKey := normalizeCharacterKey(t.Subject)
	objectKey := normalizeCharacterKey(t.Object)
	for _, entity := range entities {
		needle := strings.ToLower(entity)
		if needle != "" && (strings.Contains(subject, needle) || strings.Contains(object, needle)) {
			return true
		}
		needleKey := normalizeCharacterKey(entity)
		if kgNormalizedPartMatchesEntity(subjectKey, needleKey) || kgNormalizedPartMatchesEntity(objectKey, needleKey) {
			return true
		}
	}
	return false
}

func kgNormalizedPartMatchesEntity(partKey, entityKey string) bool {
	if len([]rune(partKey)) < 2 || len([]rune(entityKey)) < 2 {
		return false
	}
	return partKey == entityKey || strings.Contains(partKey, entityKey) || strings.Contains(entityKey, partKey)
}

func (s *Server) handleExplorerKGTriples(w http.ResponseWriter, r *http.Request) {
	sid := r.URL.Query().Get("chat_session_id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}

	items := []any{}
	total := 0

	if s.Store != nil {
		triples, err := s.Store.ListKGTriples(r.Context(), sid)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		if err == nil {
			sortKGTriplesForPython(triples)
			total = len(triples)
			start := offset
			if start > len(triples) {
				start = len(triples)
			}
			end := start + limit
			if end > len(triples) {
				end = len(triples)
			}
			for _, t := range triples[start:end] {
				items = append(items, kgTripleExplorerItem(t))
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"items":    items,
		"total":    total,
		"has_more": offset+len(items) < total,
		"limit":    limit,
		"offset":   offset,
	})
}

func explorerHierarchyPageParams(r *http.Request) (int, int) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func explorerHierarchyFetchLimit(limit, offset int) int {
	fetchLimit := limit + offset + 1
	if fetchLimit < limit {
		fetchLimit = limit
	}
	if fetchLimit > 100 {
		fetchLimit = 100
	}
	return fetchLimit
}

func writeExplorerHierarchyItems(w http.ResponseWriter, limit, offset int, items []any) {
	total := len(items)
	start := offset
	if start > len(items) {
		start = len(items)
	}
	end := start + limit
	if end > len(items) {
		end = len(items)
	}
	page := items[start:end]

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"items":    page,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
		"has_more": offset+len(page) < total,
	})
}

func chapterSummaryExplorerItem(ch store.ChapterSummary, source string) map[string]any {
	return map[string]any{
		"id":                        ch.ID,
		"chat_session_id":           ch.ChatSessionID,
		"from_turn":                 ch.FromTurn,
		"to_turn":                   ch.ToTurn,
		"chapter_index":             ch.ChapterIndex,
		"chapter_title":             ch.ChapterTitle,
		"summary_text":              ch.SummaryText,
		"open_loops_json":           ch.OpenLoopsJSON,
		"relationship_changes_json": ch.RelationshipChangesJSON,
		"world_changes_json":        ch.WorldChangesJSON,
		"callback_candidates_json":  ch.CallbackCandidatesJSON,
		"resume_text":               ch.ResumeText,
		"embedding_model":           ch.EmbeddingModel,
		"created_at":                ch.CreatedAt,
		"source":                    source,
	}
}

func arcSummaryExplorerItem(arc store.ArcSummary) map[string]any {
	return map[string]any{
		"id":                            arc.ID,
		"chat_session_id":               arc.ChatSessionID,
		"from_turn":                     arc.FromTurn,
		"to_turn":                       arc.ToTurn,
		"arc_index":                     arc.ArcIndex,
		"arc_name":                      arc.ArcName,
		"arc_status":                    arc.ArcStatus,
		"core_conflict":                 arc.CoreConflict,
		"key_turning_points_json":       arc.KeyTurningPointsJSON,
		"active_promises_json":          arc.ActivePromisesJSON,
		"unresolved_debts_json":         arc.UnresolvedDebtsJSON,
		"resolved_payoffs_json":         arc.ResolvedPayoffsJSON,
		"callback_candidates_json":      arc.CallbackCandidatesJSON,
		"future_payoff_candidates_json": arc.FuturePayoffCandidatesJSON,
		"irreversible_turns_json":       arc.IrreversibleTurnsJSON,
		"callback_debts_json":           arc.CallbackDebtsJSON,
		"relationship_pivots_json":      arc.RelationshipPivotsJSON,
		"arc_resume_text":               arc.ArcResumeText,
		"embedding_model":               arc.EmbeddingModel,
		"created_at":                    arc.CreatedAt,
		"source":                        "arc_summary",
	}
}

func sagaDigestExplorerItem(saga store.SagaDigest) map[string]any {
	return map[string]any{
		"id":                         saga.ID,
		"chat_session_id":            saga.ChatSessionID,
		"from_turn":                  saga.FromTurn,
		"to_turn":                    saga.ToTurn,
		"era_label":                  saga.EraLabel,
		"saga_summary":               saga.SagaSummary,
		"persistent_facts_json":      saga.PersistentFactsJSON,
		"never_drop_candidates_json": saga.NeverDropCandidatesJSON,
		"resume_pack_text":           saga.ResumePackText,
		"embedding_model":            saga.EmbeddingModel,
		"created_at":                 saga.CreatedAt,
		"source":                     "saga_digest",
	}
}

func (s *Server) handleExplorerChapterSummaries(w http.ResponseWriter, r *http.Request) {
	sid := r.URL.Query().Get("chat_session_id")
	limit, offset := explorerHierarchyPageParams(r)
	if sid == "" {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "items": []any{}, "total": 0, "limit": limit, "offset": offset, "has_more": false})
		return
	}

	items := []any{}

	if s.Store != nil {
		if chapterStore, ok := s.Store.(store.ChapterSummaryStore); ok {
			chapters, err := chapterStore.SearchChapterSummaries(r.Context(), sid, "", 0, 0, explorerHierarchyFetchLimit(limit, offset))
			if err != nil && !errors.Is(err, store.ErrNotEnabled) {
				writeInternalError(w, err.Error())
				return
			}
			if err == nil {
				for _, ch := range chapters {
					items = append(items, chapterSummaryExplorerItem(ch, "chapter_summary"))
				}
			}
		}

		if len(items) == 0 {
			pack, err := s.Store.GetResumePack(r.Context(), sid, "resume")
			if err != nil && !errors.Is(err, store.ErrNotEnabled) {
				writeInternalError(w, err.Error())
				return
			}
			if err == nil && pack != nil && pack.Chapter != nil {
				ch := pack.Chapter
				if ch.ChatSessionID == "" {
					ch.ChatSessionID = sid
				}
				items = append(items, chapterSummaryExplorerItem(*ch, "resume_pack_chapter"))
			}
		}
	}

	writeExplorerHierarchyItems(w, limit, offset, items)
}

func (s *Server) handleExplorerArcSummaries(w http.ResponseWriter, r *http.Request) {
	sid := r.URL.Query().Get("chat_session_id")
	limit, offset := explorerHierarchyPageParams(r)
	if sid == "" {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "items": []any{}, "total": 0, "limit": limit, "offset": offset, "has_more": false})
		return
	}

	items := []any{}
	if s.Store != nil {
		if arcStore, ok := s.Store.(store.ArcSummaryStore); ok {
			arcs, err := arcStore.ListArcSummaries(r.Context(), sid, r.URL.Query().Get("status"), explorerHierarchyFetchLimit(limit, offset))
			if err != nil && !errors.Is(err, store.ErrNotEnabled) {
				writeInternalError(w, err.Error())
				return
			}
			if err == nil {
				for _, arc := range arcs {
					items = append(items, arcSummaryExplorerItem(arc))
				}
			}
		}
	}

	writeExplorerHierarchyItems(w, limit, offset, items)
}

func (s *Server) handleExplorerSagaDigests(w http.ResponseWriter, r *http.Request) {
	sid := r.URL.Query().Get("chat_session_id")
	limit, offset := explorerHierarchyPageParams(r)
	if sid == "" {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "items": []any{}, "total": 0, "limit": limit, "offset": offset, "has_more": false})
		return
	}

	items := []any{}
	if s.Store != nil {
		if sagaStore, ok := s.Store.(store.SagaDigestStore); ok {
			sagas, err := sagaStore.ListSagaDigests(r.Context(), sid, explorerHierarchyFetchLimit(limit, offset))
			if err != nil && !errors.Is(err, store.ErrNotEnabled) {
				writeInternalError(w, err.Error())
				return
			}
			if err == nil {
				for _, saga := range sagas {
					items = append(items, sagaDigestExplorerItem(saga))
				}
			}
		}
	}

	writeExplorerHierarchyItems(w, limit, offset, items)
}

func (s *Server) handleExplorerGet404(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotFound, map[string]any{
		"detail": "Not Found",
	})
}

func memorySummaryPreview(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
		parts := []string{}
		for _, key := range []string{"turn_summary", "summary", "scene_summary", "core_meaning", "emotional_shift"} {
			value := strings.TrimSpace(jsonValueString(parsed[key]))
			if key == "turn_summary" {
				value = normalizeCriticTurnSummary(parsed[key])
			} else if looksLikeStructuredCriticPayloadText(value) {
				value = ""
			}
			if value != "" {
				parts = append(parts, truncatePlainForPreview(value, 80))
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, " | ")
		}
		return truncatePlainForPreview(pythonishJSONPreview(raw), 120)
	}
	return truncatePlainForPreview(raw, 120)
}

func nullableFloatZero(v float64) any {
	if v == 0 {
		return nil
	}
	return v
}

func safeRound2Float(v float64) float64 {
	return math.Round(v*100) / 100
}

func jsonValueString(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	b, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(b)
}

func pythonishJSONPreview(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	runes := []rune(raw)
	var b strings.Builder
	b.Grow(len(raw) + 16)
	skipOuterWhitespace := false
	for i := 0; i < len(runes); {
		r := runes[i]
		if r == '"' {
			content, next := readJSONStringPreviewToken(runes, i)
			lookahead := next
			for lookahead < len(runes) && isJSONPreviewWhitespace(runes[lookahead]) {
				lookahead++
			}
			quote := '\''
			if lookahead >= len(runes) || runes[lookahead] != ':' {
				if strings.ContainsRune(content, '\'') && !strings.ContainsRune(content, '"') {
					quote = '"'
				}
			}
			b.WriteRune(quote)
			b.WriteString(content)
			b.WriteRune(quote)
			i = next
			skipOuterWhitespace = false
			continue
		}
		if skipOuterWhitespace && isJSONPreviewWhitespace(r) {
			i++
			continue
		}
		skipOuterWhitespace = false
		switch r {
		case ':':
			b.WriteString(": ")
			skipOuterWhitespace = true
		case ',':
			b.WriteString(", ")
			skipOuterWhitespace = true
		default:
			b.WriteRune(r)
		}
		i++
	}
	return b.String()
}

func readJSONStringPreviewToken(runes []rune, start int) (string, int) {
	var b strings.Builder
	escaped := false
	for i := start + 1; i < len(runes); i++ {
		r := runes[i]
		if escaped {
			switch r {
			case '"', '\\', '/':
				b.WriteRune(r)
			case 'b':
				b.WriteRune('\b')
			case 'f':
				b.WriteRune('\f')
			case 'n':
				b.WriteRune('\n')
			case 'r':
				b.WriteRune('\r')
			case 't':
				b.WriteRune('\t')
			default:
				b.WriteRune(r)
			}
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == '"' {
			return b.String(), i + 1
		}
		b.WriteRune(r)
	}
	return b.String(), len(runes)
}

func isJSONPreviewWhitespace(r rune) bool {
	switch r {
	case ' ', '\t', '\r', '\n':
		return true
	default:
		return false
	}
}

func truncatePlainForPreview(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func truncateForPreview(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "..."
}

// Explorer write: R2 guards

func (s *Server) handlePatchMemory(w http.ResponseWriter, r *http.Request) {
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, "PATCH /explorer/memories/{memory_id}")
		return
	}
	mutationStore, ok := s.Store.(store.ExplorerMutationStore)
	if !ok {
		writeShadowGuard(w, "PATCH /explorer/memories/{memory_id}")
		return
	}
	memoryID, ok := parseExplorerPathID(w, r, "memory_id")
	if !ok {
		return
	}
	fields, sid, ok := decodeExplorerPatchRequest(w, r)
	if !ok {
		return
	}
	mem, found, err := s.findMemoryForExplorerPatch(r.Context(), sid, memoryID)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Not Found"})
		return
	}

	patch := store.MemoryExplorerPatch{}
	updatedFields := []string{}
	updatedValues := map[string]any{}
	if raw, exists := fields["summary_json"]; exists && !isJSONNull(raw) {
		value, ok := rawStringField(w, raw, "summary_json")
		if !ok {
			return
		}
		if strings.TrimSpace(value) != "" && !json.Valid([]byte(value)) {
			writeBadRequest(w, "summary_json must be valid JSON")
			return
		}
		patch.SummaryJSON = &value
		updatedFields = append(updatedFields, "summary_json")
		updatedValues["summary_json"] = value
	}
	if raw, exists := fields["importance"]; exists && !isJSONNull(raw) {
		value, ok := rawFloatField(w, raw, "importance")
		if !ok {
			return
		}
		patch.Importance = &value
		updatedFields = append(updatedFields, "importance")
		updatedValues["importance"] = value
	}
	if raw, exists := fields["archive_wing"]; exists && !isJSONNull(raw) {
		value, ok := rawStringField(w, raw, "archive_wing")
		if !ok {
			return
		}
		patch.PlaceWing = &value
		updatedFields = append(updatedFields, "archive_wing")
		updatedValues["archive_wing"] = value
	}
	if raw, exists := fields["archive_room"]; exists && !isJSONNull(raw) {
		value, ok := rawStringField(w, raw, "archive_room")
		if !ok {
			return
		}
		patch.PlaceRoom = &value
		updatedFields = append(updatedFields, "archive_room")
		updatedValues["archive_room"] = value
	}
	if len(updatedFields) == 0 {
		writeBadRequest(w, "no supported memory fields to update")
		return
	}

	changedAt := time.Now().UTC()
	if err := mutationStore.UpdateMemoryExplorerFields(r.Context(), sid, memoryID, patch); err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, "PATCH /explorer/memories/{memory_id}")
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	s.saveAuditLogBestEffort(r.Context(), &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "manual_edit",
		TargetType:    "memory",
		TargetID:      memoryID,
		Summary:       "Explorer manual memory edit",
		DetailsJSON: mustCompactJSON(map[string]any{
			"updated_fields": updatedFields,
			"updated_values": updatedValues,
			"previous": map[string]any{
				"summary_json": mem.SummaryJSON,
				"importance":   mem.Importance,
				"archive_wing": mem.PlaceWing,
				"archive_room": mem.PlaceRoom,
				"created_at":   mem.CreatedAt,
			},
			"changed_at": changedAt,
		}),
		Source:    "explorer_manual_edit",
		CreatedAt: changedAt,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"source":           s.storeWriteSource(),
		"mutation_enabled": true,
		"chat_session_id":  sid,
		"target_type":      "memory",
		"target_id":        memoryID,
		"updated_fields":   updatedFields,
		"changed_at":       changedAt,
		"audit_written":    true,
	})
}

func (s *Server) handlePatchKGTriple(w http.ResponseWriter, r *http.Request) {
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, "PATCH /explorer/kg_triples/{triple_id}")
		return
	}
	mutationStore, ok := s.Store.(store.ExplorerMutationStore)
	if !ok {
		writeShadowGuard(w, "PATCH /explorer/kg_triples/{triple_id}")
		return
	}
	tripleID, ok := parseExplorerPathID(w, r, "triple_id")
	if !ok {
		return
	}
	fields, sid, ok := decodeExplorerPatchRequest(w, r)
	if !ok {
		return
	}
	triple, found, err := s.findKGTripleForExplorerPatch(r.Context(), sid, tripleID)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Not Found"})
		return
	}

	patch := store.KGTripleExplorerPatch{}
	updatedFields := []string{}
	updatedValues := map[string]any{}
	for _, key := range []string{"subject", "predicate", "object"} {
		raw, exists := fields[key]
		if !exists || isJSONNull(raw) {
			continue
		}
		value, ok := rawStringField(w, raw, key)
		if !ok {
			return
		}
		switch key {
		case "subject":
			patch.Subject = &value
		case "predicate":
			patch.Predicate = &value
		case "object":
			patch.Object = &value
		}
		updatedFields = append(updatedFields, key)
		updatedValues[key] = value
	}
	if raw, exists := fields["valid_from"]; exists {
		value, ok := rawOptionalIntField(w, raw, "valid_from")
		if !ok {
			return
		}
		patch.ValidFrom = value
		updatedFields = append(updatedFields, "valid_from")
		updatedValues["valid_from"] = optionalIntValueForJSON(value)
	}
	if raw, exists := fields["valid_to"]; exists {
		value, ok := rawOptionalIntField(w, raw, "valid_to")
		if !ok {
			return
		}
		patch.ValidTo = value
		updatedFields = append(updatedFields, "valid_to")
		updatedValues["valid_to"] = optionalIntValueForJSON(value)
	}
	if len(updatedFields) == 0 {
		writeBadRequest(w, "no supported KG fields to update")
		return
	}

	changedAt := time.Now().UTC()
	if err := mutationStore.UpdateKGTripleExplorerFields(r.Context(), sid, tripleID, patch); err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, "PATCH /explorer/kg_triples/{triple_id}")
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	s.saveAuditLogBestEffort(r.Context(), &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "manual_edit",
		TargetType:    "kg_triple",
		TargetID:      tripleID,
		Summary:       "Explorer manual KG triple edit",
		DetailsJSON: mustCompactJSON(map[string]any{
			"updated_fields": updatedFields,
			"updated_values": updatedValues,
			"previous": map[string]any{
				"subject":    triple.Subject,
				"predicate":  triple.Predicate,
				"object":     triple.Object,
				"valid_from": triple.ValidFrom,
				"valid_to":   triple.ValidTo,
			},
			"changed_at": changedAt,
		}),
		Source:    "explorer_manual_edit",
		CreatedAt: changedAt,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"source":           s.storeWriteSource(),
		"mutation_enabled": true,
		"chat_session_id":  sid,
		"target_type":      "kg_triple",
		"target_id":        tripleID,
		"updated_fields":   updatedFields,
		"changed_at":       changedAt,
		"audit_written":    true,
	})
}

func (s *Server) handlePatchEvidenceEdit(w http.ResponseWriter, r *http.Request) {
	s.handlePatchEvidenceTransition(w, r, "edit")
}

func (s *Server) handlePatchEvidenceReview(w http.ResponseWriter, r *http.Request) {
	s.handlePatchEvidenceTransition(w, r, "review")
}

func (s *Server) handlePatchEvidenceRevalidate(w http.ResponseWriter, r *http.Request) {
	s.handlePatchEvidenceTransition(w, r, "revalidate")
}

func (s *Server) handlePatchEvidenceTombstone(w http.ResponseWriter, r *http.Request) {
	s.handlePatchEvidenceTransition(w, r, "tombstone")
}

func (s *Server) handlePatchEvidenceSupersede(w http.ResponseWriter, r *http.Request) {
	s.handlePatchEvidenceTransition(w, r, "supersede")
}

func (s *Server) handlePatchEvidenceTransition(w http.ResponseWriter, r *http.Request, action string) {
	endpoint := "PATCH /explorer/direct-evidence/{record_id}/" + action
	if action == "edit" {
		endpoint = "PATCH /explorer/direct-evidence/{record_id}"
	}
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, endpoint)
		return
	}
	mutationStore, ok := s.Store.(store.ExplorerMutationStore)
	if !ok {
		writeShadowGuard(w, endpoint)
		return
	}
	recordID, ok := parseExplorerPathID(w, r, "record_id")
	if !ok {
		return
	}
	fields, sid, ok := decodeExplorerPatchRequest(w, r)
	if !ok {
		return
	}
	evidence, found, err := s.findEvidenceForExplorerPatch(r.Context(), sid, recordID)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Not Found"})
		return
	}

	patch := store.DirectEvidenceExplorerPatch{}
	updatedFields := []string{}
	updatedValues := map[string]any{}
	switch action {
	case "edit":
		if raw, exists := fields["archive_state"]; exists && !isJSONNull(raw) {
			value, ok := rawStringField(w, raw, "archive_state")
			if !ok {
				return
			}
			patch.ArchiveState = &value
			updatedFields = append(updatedFields, "archive_state")
			updatedValues["archive_state"] = value
		}
		if raw, exists := fields["capture_verification"]; exists && !isJSONNull(raw) {
			value, ok := rawStringField(w, raw, "capture_verification")
			if !ok {
				return
			}
			patch.CaptureVerification = &value
			updatedFields = append(updatedFields, "capture_verification")
			updatedValues["capture_verification"] = value
		}
		if raw, exists := fields["committed_gate"]; exists && !isJSONNull(raw) {
			value, ok := rawStringField(w, raw, "committed_gate")
			if !ok {
				return
			}
			patch.CommittedGate = &value
			updatedFields = append(updatedFields, "committed_gate")
			updatedValues["committed_gate"] = value
		}
		if raw, exists := fields["repair_needed"]; exists && !isJSONNull(raw) {
			value, ok := rawBoolField(w, raw, "repair_needed")
			if !ok {
				return
			}
			patch.RepairNeeded = &value
			updatedFields = append(updatedFields, "repair_needed")
			updatedValues["repair_needed"] = value
		}
		if raw, exists := fields["tombstoned"]; exists && !isJSONNull(raw) {
			value, ok := rawBoolField(w, raw, "tombstoned")
			if !ok {
				return
			}
			patch.Tombstoned = &value
			updatedFields = append(updatedFields, "tombstoned")
			updatedValues["tombstoned"] = value
		}
		if raw, exists := fields["superseded_by_id"]; exists {
			value, ok := rawOptionalIntField(w, raw, "superseded_by_id")
			if !ok {
				return
			}
			patch.SupersededByID = value
			updatedFields = append(updatedFields, "superseded_by_id")
			updatedValues["superseded_by_id"] = optionalIntValueForJSON(value)
		}
		if len(updatedFields) == 0 {
			writeBadRequest(w, "at least one editable direct evidence field is required")
			return
		}
	case "review":
		raw, exists := fields["capture_verification"]
		if !exists || isJSONNull(raw) {
			writeBadRequest(w, "capture_verification is required")
			return
		}
		value, ok := rawStringField(w, raw, "capture_verification")
		if !ok {
			return
		}
		patch.CaptureVerification = &value
		updatedFields = append(updatedFields, "capture_verification")
		updatedValues["capture_verification"] = value
	case "revalidate":
		verification := "verified"
		state := "committed"
		gate := "manual_revalidate"
		repairNeeded := false
		patch.CaptureVerification = &verification
		patch.ArchiveState = &state
		patch.CommittedGate = &gate
		patch.RepairNeeded = &repairNeeded
		updatedFields = append(updatedFields, "capture_verification", "archive_state", "committed_gate", "repair_needed")
		updatedValues["capture_verification"] = verification
		updatedValues["archive_state"] = state
		updatedValues["committed_gate"] = gate
		updatedValues["repair_needed"] = repairNeeded
	case "tombstone":
		tombstoned := true
		state := "tombstoned"
		patch.Tombstoned = &tombstoned
		patch.ArchiveState = &state
		updatedFields = append(updatedFields, "tombstoned", "archive_state")
		updatedValues["tombstoned"] = tombstoned
		updatedValues["archive_state"] = state
	case "supersede":
		raw, exists := fields["superseded_by_id"]
		if !exists {
			writeBadRequest(w, "superseded_by_id is required")
			return
		}
		value, ok := rawOptionalIntField(w, raw, "superseded_by_id")
		if !ok {
			return
		}
		patch.SupersededByID = value
		updatedFields = append(updatedFields, "superseded_by_id")
		updatedValues["superseded_by_id"] = optionalIntValueForJSON(value)
	default:
		writeBadRequest(w, "unsupported evidence action")
		return
	}

	changedAt := time.Now().UTC()
	if err := mutationStore.UpdateDirectEvidenceExplorerFields(r.Context(), sid, recordID, patch); err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, endpoint)
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	s.saveAuditLogBestEffort(r.Context(), &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "manual_edit",
		TargetType:    "direct_evidence",
		TargetID:      recordID,
		Summary:       "Explorer manual direct evidence " + action,
		DetailsJSON: mustCompactJSON(map[string]any{
			"action":         action,
			"updated_fields": updatedFields,
			"updated_values": updatedValues,
			"previous": map[string]any{
				"archive_state":        evidence.ArchiveState,
				"capture_verification": evidence.CaptureVerification,
				"committed_gate":       evidence.CommittedGate,
				"repair_needed":        evidence.RepairNeeded,
				"tombstoned":           evidence.Tombstoned,
				"superseded_by_id":     evidence.SupersededByID,
			},
			"review_note": stringFromRawField(fields["review_note"]),
			"changed_at":  changedAt,
		}),
		Source:    "explorer_manual_edit",
		CreatedAt: changedAt,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"source":           s.storeWriteSource(),
		"mutation_enabled": true,
		"chat_session_id":  sid,
		"target_type":      "direct_evidence",
		"target_id":        recordID,
		"action":           action,
		"updated_fields":   updatedFields,
		"changed_at":       changedAt,
		"audit_written":    true,
	})
}

func parseExplorerPathID(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(strings.TrimSpace(r.PathValue(name)), 10, 64)
	if err != nil || id <= 0 {
		writeBadRequest(w, name+" must be a positive integer")
		return 0, false
	}
	return id, true
}

func decodeExplorerPatchRequest(w http.ResponseWriter, r *http.Request) (map[string]json.RawMessage, string, bool) {
	var fields map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&fields); err != nil {
		writeBadRequest(w, err.Error())
		return nil, "", false
	}
	sidRaw, ok := fields["chat_session_id"]
	if !ok || isJSONNull(sidRaw) {
		writeBadRequest(w, "chat_session_id is required")
		return nil, "", false
	}
	sid, ok := rawStringField(w, sidRaw, "chat_session_id")
	if !ok {
		return nil, "", false
	}
	sid = strings.TrimSpace(sid)
	if sid == "" {
		writeBadRequest(w, "chat_session_id is required")
		return nil, "", false
	}
	return fields, sid, true
}

func rawStringField(w http.ResponseWriter, raw json.RawMessage, field string) (string, bool) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		writeBadRequest(w, field+" must be a string")
		return "", false
	}
	return value, true
}

func rawBoolField(w http.ResponseWriter, raw json.RawMessage, field string) (bool, bool) {
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		writeBadRequest(w, field+" must be a boolean")
		return false, false
	}
	return value, true
}

func rawFloatField(w http.ResponseWriter, raw json.RawMessage, field string) (float64, bool) {
	var value float64
	if err := json.Unmarshal(raw, &value); err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		writeBadRequest(w, field+" must be a finite number")
		return 0, false
	}
	return value, true
}

func rawOptionalIntField(w http.ResponseWriter, raw json.RawMessage, field string) (store.OptionalIntPatch, bool) {
	out := store.OptionalIntPatch{Set: true}
	if isJSONNull(raw) {
		return out, true
	}
	var value int
	if err := json.Unmarshal(raw, &value); err != nil {
		writeBadRequest(w, field+" must be an integer or null")
		return out, false
	}
	out.Value = &value
	return out, true
}

func optionalIntValueForJSON(value store.OptionalIntPatch) any {
	if value.Value == nil {
		return nil
	}
	return *value.Value
}

func isJSONNull(raw json.RawMessage) bool {
	return strings.EqualFold(strings.TrimSpace(string(raw)), "null")
}

func stringFromRawField(raw json.RawMessage) string {
	if len(raw) == 0 || isJSONNull(raw) {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return value
}

func (s *Server) findMemoryForExplorerPatch(ctx context.Context, sid string, memoryID int64) (store.Memory, bool, error) {
	items, err := s.Store.ListMemories(ctx, sid, 0, 0)
	if err != nil {
		return store.Memory{}, false, err
	}
	for _, item := range items {
		if item.ID == memoryID && item.ChatSessionID == sid {
			return item, true, nil
		}
	}
	return store.Memory{}, false, nil
}

func (s *Server) findKGTripleForExplorerPatch(ctx context.Context, sid string, tripleID int64) (store.KGTriple, bool, error) {
	items, err := s.Store.ListKGTriples(ctx, sid)
	if err != nil {
		return store.KGTriple{}, false, err
	}
	for _, item := range items {
		if item.ID == tripleID && item.ChatSessionID == sid {
			return item, true, nil
		}
	}
	return store.KGTriple{}, false, nil
}

func (s *Server) findEvidenceForExplorerPatch(ctx context.Context, sid string, recordID int64) (store.DirectEvidence, bool, error) {
	items, err := s.Store.ListEvidence(ctx, sid)
	if err != nil {
		return store.DirectEvidence{}, false, err
	}
	for _, item := range items {
		if item.ID == recordID && item.ChatSessionID == sid {
			return item, true, nil
		}
	}
	return store.DirectEvidence{}, false, nil
}

type explorerRegenerateMemoryRequest struct {
	ChatSessionID string         `json:"chat_session_id"`
	TurnIndex     int            `json:"turn_index"`
	ClientMeta    map[string]any `json:"client_meta"`
	DryRun        bool           `json:"dry_run"`
}

func (s *Server) handleRegenerateMemory(w http.ResponseWriter, r *http.Request) {
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, "POST /explorer/memories/regenerate")
		return
	}
	if s.Store == nil {
		writeInternalError(w, "store is not configured")
		return
	}
	var req explorerRegenerateMemoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, "invalid JSON body")
		return
	}
	sid := strings.TrimSpace(req.ChatSessionID)
	if sid == "" {
		writeBadRequest(w, "chat_session_id is required")
		return
	}
	if req.TurnIndex <= 0 {
		writeBadRequest(w, "turn_index is required")
		return
	}

	logs, err := s.Store.ListChatLogs(r.Context(), sid, req.TurnIndex, req.TurnIndex)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		writeInternalError(w, err.Error())
		return
	}
	roleMap := map[string]string{}
	for _, log := range logs {
		if log.ChatSessionID != sid || log.TurnIndex != req.TurnIndex {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(log.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		roleMap[role] = appendUniqueTurnRoleText(roleMap[role], log.Content)
	}
	userText := sanitizeCriticStorageText(roleMap["user"])
	assistantText := sanitizeCriticStorageText(roleMap["assistant"])
	if strings.TrimSpace(assistantText) == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":          "failed",
			"detail":          "assistant_content_missing",
			"chat_session_id": sid,
			"turn_index":      req.TurnIndex,
			"source":          s.storeWriteSource(),
		})
		return
	}
	if shouldApplyCompleteTurnOOCGuard(userText, assistantText, nil) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":          "skipped",
			"reason":          "ooc_guard",
			"chat_session_id": sid,
			"turn_index":      req.TurnIndex,
			"source":          s.storeWriteSource(),
		})
		return
	}
	extractionCfg := s.completeTurnExtractionConfig(req.ClientMeta)
	llmTrace := completeTurnLLMConfigTrace(extractionCfg)
	if !extractionCfg.Critic.hasConfig() {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":           "failed",
			"detail":           "critic_config_missing",
			"chat_session_id":  sid,
			"turn_index":       req.TurnIndex,
			"source":           s.storeWriteSource(),
			"llm_config_trace": llmTrace,
		})
		return
	}
	if req.DryRun {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":           "ok",
			"dry_run":          true,
			"chat_session_id":  sid,
			"turn_index":       req.TurnIndex,
			"source":           s.storeWriteSource(),
			"llm_config_trace": llmTrace,
			"note":             "Explorer regenerate dry-run found a completed turn but did not call Critic or write artifacts",
		})
		return
	}

	extraction, criticTrace, err := s.runCompleteTurnCriticFromCanonicalLogs(r.Context(), sid, req.TurnIndex, userText, assistantText, extractionCfg.Critic)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":           "failed",
			"detail":           "critic_extract_failed: " + err.Error(),
			"chat_session_id":  sid,
			"turn_index":       req.TurnIndex,
			"source":           s.storeWriteSource(),
			"critic_trace":     criticTrace,
			"llm_config_trace": llmTrace,
		})
		return
	}
	now := time.Now().UTC()
	content := strings.TrimSpace(strings.Join([]string{userText, assistantText}, "\n"))
	saveResult := s.saveCriticExtractionArtifacts(r.Context(), sid, req.TurnIndex, extraction, content, extractionCfg.Embedder, now)
	status := "ok"
	if saveResult.Errors > 0 {
		status = "partial_error"
	}

	allLogs, _ := s.Store.ListChatLogs(r.Context(), sid, 0, 0)
	allMemories, _ := s.Store.ListMemories(r.Context(), sid, 0, 0)
	allEvidence, _ := s.Store.ListEvidence(r.Context(), sid)
	targetTurns := map[int]bool{req.TurnIndex: true}
	episodeInterval := normalizedEpisodeInterval(intFromAny(req.ClientMeta["episode_interval_turns"], 0))
	episodeBackfill := s.backfillEpisodeSummariesFromChatLogs(r.Context(), sid, allLogs, allMemories, allEvidence, episodeInterval, false, targetTurns, true)
	worldRuleBackfill := s.backfillWorldRulesFromMemories(r.Context(), sid, allMemories, targetTurns, false)

	s.saveAuditLogBestEffort(r.Context(), &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "explorer_regenerate_memory",
		TargetType:    "turn",
		TargetID:      int64(req.TurnIndex),
		Summary:       fmt.Sprintf("Explorer regenerated derived artifacts for turn %d", req.TurnIndex),
		DetailsJSON: mustCompactJSON(map[string]any{
			"artifact_result":     saveResult,
			"episode_backfill":    episodeBackfill,
			"world_rule_backfill": worldRuleBackfill,
			"critic_trace":        criticTrace,
		}),
		Source:    "explorer_regenerate",
		CreatedAt: now,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"status":                           status,
		"source":                           s.storeWriteSource(),
		"chat_session_id":                  sid,
		"turn_index":                       req.TurnIndex,
		"memories_saved":                   saveResult.Memories,
		"evidence_saved":                   saveResult.Evidence,
		"kg_triples_saved":                 saveResult.KGTriples,
		"subjective_entity_memories_saved": saveResult.SubjectiveEntityMemories,
		"character_states_saved":           saveResult.CharacterStates,
		"world_rules_saved":                saveResult.WorldRules,
		"entities_saved":                   saveResult.Entities,
		"trust_states_saved":               saveResult.TrustStates,
		"storylines_saved":                 saveResult.Storylines,
		"pending_threads_saved":            saveResult.PendingThreads,
		"active_states_saved":              saveResult.ActiveStates,
		"canonical_state_layers_saved":     saveResult.CanonicalStateLayers,
		"vectors_upserted":                 saveResult.VectorsUpserted,
		"episode_backfill":                 episodeBackfill,
		"world_rule_backfill":              worldRuleBackfill,
		"warnings":                         saveResult.Warnings,
		"skip_reasons":                     saveResult.SkipReasons,
		"store_write_errors":               saveResult.Errors,
		"store_write_error_details":        saveResult.ErrorDetails,
		"critic_result":                    extraction,
		"critic_trace":                     criticTrace,
		"llm_config_trace":                 llmTrace,
		"note":                             "Explorer regenerate rebuilt this turn through the same Critic artifact pipeline used by complete-turn and admin rescan",
	})
}

func (s *Server) handleDeleteMemory(w http.ResponseWriter, r *http.Request) {
	s.handleDeleteMemoryMutation(w, r, "DELETE /explorer/memories/{memory_id}")
}

func (s *Server) handleDeleteMemoryPost(w http.ResponseWriter, r *http.Request) {
	s.handleDeleteMemoryMutation(w, r, "POST /explorer/memories/{memory_id}/delete")
}

func (s *Server) handleDeleteDirectEvidence(w http.ResponseWriter, r *http.Request) {
	s.handleDeleteDirectEvidenceMutation(w, r, "DELETE /explorer/direct-evidence/{record_id}")
}

func (s *Server) handleDeleteDirectEvidencePost(w http.ResponseWriter, r *http.Request) {
	s.handleDeleteDirectEvidenceMutation(w, r, "POST /explorer/direct-evidence/{record_id}/delete")
}

func (s *Server) handleDeleteKGTriple(w http.ResponseWriter, r *http.Request) {
	s.handleDeleteKGTripleMutation(w, r, "DELETE /explorer/kg_triples/{triple_id}")
}

func (s *Server) handleDeleteKGTriplePost(w http.ResponseWriter, r *http.Request) {
	s.handleDeleteKGTripleMutation(w, r, "POST /explorer/kg_triples/{triple_id}/delete")
}

func (s *Server) handleDeleteMemoryMutation(w http.ResponseWriter, r *http.Request, endpoint string) {
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, endpoint)
		return
	}
	mutationStore, ok := s.Store.(store.ExplorerMutationStore)
	if !ok {
		writeShadowGuard(w, endpoint)
		return
	}
	memoryID, ok := parseExplorerPathID(w, r, "memory_id")
	if !ok {
		return
	}
	sid := strings.TrimSpace(r.URL.Query().Get("chat_session_id"))
	if sid == "" {
		writeBadRequest(w, "chat_session_id is required")
		return
	}
	mem, found, err := s.findMemoryForExplorerPatch(r.Context(), sid, memoryID)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Not Found"})
		return
	}

	changedAt := time.Now().UTC()
	if err := mutationStore.DeleteMemoryByID(r.Context(), sid, memoryID); err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, endpoint)
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	vectorCleanup := s.deleteMemoryVectorDocument(r.Context(), sid, mem)
	s.saveAuditLogBestEffort(r.Context(), &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "manual_delete",
		TargetType:    "memory",
		TargetID:      memoryID,
		Summary:       "Explorer manual memory delete",
		DetailsJSON: mustCompactJSON(map[string]any{
			"previous": map[string]any{
				"turn_index":    mem.TurnIndex,
				"summary_json":  mem.SummaryJSON,
				"importance":    mem.Importance,
				"archive_wing":  mem.PlaceWing,
				"archive_room":  mem.PlaceRoom,
				"created_at":    mem.CreatedAt,
				"evidence":      mem.Evidence,
				"embedding_set": strings.TrimSpace(mem.Embedding) != "",
			},
			"changed_at":     changedAt,
			"vector_cleanup": vectorCleanup,
		}),
		Source:    "explorer_manual_delete",
		CreatedAt: changedAt,
	})
	status := "ok"
	if ok, _ := vectorCleanup["ok"].(bool); !ok {
		status = "partial_error"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           status,
		"source":           s.storeWriteSource(),
		"mutation_enabled": true,
		"chat_session_id":  sid,
		"target_type":      "memory",
		"target_id":        memoryID,
		"deleted":          true,
		"changed_at":       changedAt,
		"audit_written":    true,
		"vector_cleanup":   vectorCleanup,
	})
}

func (s *Server) deleteMemoryVectorDocument(ctx context.Context, sid string, mem store.Memory) map[string]any {
	docID := memoryVectorDocumentID(sid, mem)
	cleanup := map[string]any{
		"attempted":   false,
		"ok":          true,
		"document_id": docID,
	}
	if docID == "" {
		cleanup["skipped_reason"] = "missing_vector_document_id"
		return cleanup
	}
	if s.Vector == nil {
		cleanup["skipped_reason"] = "vector_store_not_configured"
		return cleanup
	}
	if strings.TrimSpace(s.Cfg.ChromaEndpoint) == "" {
		cleanup["skipped_reason"] = "chromadb_endpoint_not_configured"
		return cleanup
	}
	deleter, ok := s.Vector.(vector.DocumentDeleter)
	if !ok {
		cleanup["ok"] = false
		cleanup["skipped_reason"] = "vector_store_does_not_support_document_delete"
		return cleanup
	}
	cleanup["attempted"] = true
	if err := deleter.DeleteDocuments(ctx, []string{docID}); err != nil {
		if errors.Is(err, vector.ErrNotEnabled) {
			cleanup["warning"] = "vector_store_not_enabled"
			cleanup["deleted_ids"] = 0
			return cleanup
		}
		cleanup["ok"] = false
		cleanup["error"] = err.Error()
		cleanup["deleted_ids"] = 0
		return cleanup
	}
	cleanup["deleted_ids"] = 1
	return cleanup
}

func (s *Server) handleDeleteDirectEvidenceMutation(w http.ResponseWriter, r *http.Request, endpoint string) {
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, endpoint)
		return
	}
	mutationStore, ok := s.Store.(store.ExplorerMutationStore)
	if !ok {
		writeShadowGuard(w, endpoint)
		return
	}
	recordID, ok := parseExplorerPathID(w, r, "record_id")
	if !ok {
		return
	}
	sid := strings.TrimSpace(r.URL.Query().Get("chat_session_id"))
	if sid == "" {
		writeBadRequest(w, "chat_session_id is required")
		return
	}
	evidence, found, err := s.findEvidenceForExplorerPatch(r.Context(), sid, recordID)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Not Found"})
		return
	}

	changedAt := time.Now().UTC()
	if err := mutationStore.DeleteDirectEvidenceByID(r.Context(), sid, recordID); err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, endpoint)
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	vectorCleanup := s.deleteDerivedArtifactVectorDocuments(r.Context(), sid, "evidence", recordID)
	s.saveAuditLogBestEffort(r.Context(), &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "manual_delete",
		TargetType:    "direct_evidence",
		TargetID:      recordID,
		Summary:       "Explorer manual direct evidence delete",
		DetailsJSON: mustCompactJSON(map[string]any{
			"previous": map[string]any{
				"evidence_kind":        evidence.EvidenceKind,
				"evidence_text":        evidence.EvidenceText,
				"archive_state":        evidence.ArchiveState,
				"capture_verification": evidence.CaptureVerification,
				"committed_gate":       evidence.CommittedGate,
				"tombstoned":           evidence.Tombstoned,
				"turn_anchor":          evidence.TurnAnchor,
				"source_turn_start":    evidence.SourceTurnStart,
				"source_turn_end":      evidence.SourceTurnEnd,
				"created_at":           evidence.CreatedAt,
			},
			"changed_at":     changedAt,
			"vector_cleanup": vectorCleanup,
		}),
		Source:    "explorer_manual_delete",
		CreatedAt: changedAt,
	})
	status := "ok"
	if ok, _ := vectorCleanup["ok"].(bool); !ok {
		status = "partial_error"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           status,
		"source":           s.storeWriteSource(),
		"mutation_enabled": true,
		"chat_session_id":  sid,
		"target_type":      "direct_evidence",
		"target_id":        recordID,
		"deleted":          true,
		"changed_at":       changedAt,
		"audit_written":    true,
		"vector_cleanup":   vectorCleanup,
	})
}

func (s *Server) handleDeleteKGTripleMutation(w http.ResponseWriter, r *http.Request, endpoint string) {
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, endpoint)
		return
	}
	mutationStore, ok := s.Store.(store.ExplorerMutationStore)
	if !ok {
		writeShadowGuard(w, endpoint)
		return
	}
	tripleID, ok := parseExplorerPathID(w, r, "triple_id")
	if !ok {
		return
	}
	sid := strings.TrimSpace(r.URL.Query().Get("chat_session_id"))
	if sid == "" {
		writeBadRequest(w, "chat_session_id is required")
		return
	}
	triple, found, err := s.findKGTripleForExplorerPatch(r.Context(), sid, tripleID)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Not Found"})
		return
	}

	changedAt := time.Now().UTC()
	if err := mutationStore.DeleteKGTripleByID(r.Context(), sid, tripleID); err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, endpoint)
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	s.saveAuditLogBestEffort(r.Context(), &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "manual_delete",
		TargetType:    "kg_triple",
		TargetID:      tripleID,
		Summary:       "Explorer manual KG triple delete",
		DetailsJSON: mustCompactJSON(map[string]any{
			"previous": map[string]any{
				"subject":     triple.Subject,
				"predicate":   triple.Predicate,
				"object":      triple.Object,
				"valid_from":  triple.ValidFrom,
				"valid_to":    triple.ValidTo,
				"source_turn": triple.SourceTurn,
				"created_at":  triple.CreatedAt,
			},
			"changed_at": changedAt,
		}),
		Source:    "explorer_manual_delete",
		CreatedAt: changedAt,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"source":           s.storeWriteSource(),
		"mutation_enabled": true,
		"chat_session_id":  sid,
		"target_type":      "kg_triple",
		"target_id":        tripleID,
		"deleted":          true,
		"changed_at":       changedAt,
		"audit_written":    true,
	})
}
