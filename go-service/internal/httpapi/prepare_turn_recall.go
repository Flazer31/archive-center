package httpapi

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

const prepareTurnPrivatePreciseMemorySearchResultsKey = "_priority_precise_memory_search_results"

type prepareTurnRetrievalQuery struct {
	Text   string
	Source string
}

func prepareTurnVectorDocumentRanksBefore(left, right vector.VectorDocument) bool {
	if left.SimilarityAvailable != right.SimilarityAvailable {
		return left.SimilarityAvailable
	}
	if left.Similarity != right.Similarity {
		return left.Similarity > right.Similarity
	}
	return left.Distance < right.Distance
}

// prepareTurnRecentConversationQueries turns each completed Host conversation
// into one retrieval-only query. The user input and final assistant output stay
// together so the semantic search sees both the intent and its resulting scene.
// A trailing current user input is intentionally left to RawUserInput below.
func prepareTurnRecentConversationQueries(messages []map[string]any, limit int) []prepareTurnRetrievalQuery {
	if limit < 1 {
		return nil
	}
	completed := make([]string, 0, limit)
	pendingUserInputs := []string{}
	for _, msg := range messages {
		role := strings.ToLower(strings.TrimSpace(fmt.Sprint(msg["role"])))
		content := strings.TrimSpace(fmt.Sprint(msg["content"]))
		if content == "" {
			continue
		}
		switch role {
		case "user":
			pendingUserInputs = append(pendingUserInputs, content)
		case "assistant", "char":
			parts := make([]string, 0, len(pendingUserInputs)+1)
			for _, userInput := range pendingUserInputs {
				parts = append(parts, "user:\n"+userInput)
			}
			parts = append(parts, "assistant:\n"+content)
			completed = append(completed, strings.Join(parts, "\n"))
			pendingUserInputs = pendingUserInputs[:0]
		}
	}
	if len(completed) > limit {
		completed = completed[len(completed)-limit:]
	}
	queries := make([]prepareTurnRetrievalQuery, 0, len(completed))
	for i := len(completed) - 1; i >= 0; i-- {
		queries = append(queries, prepareTurnRetrievalQuery{Text: completed[i], Source: "recent_conversation_turn"})
	}
	return queries
}

func prepareTurnRecentConversationReferenceLimit(settings dto.PrepareTurnSettings) int {
	defaults := dto.PrepareTurnSettings{}
	defaults.ApplyDefaults()
	return prepareTurnIntSetting(settings.RecentConversationReferenceCount, defaults.RecentConversationReferenceCount)
}

// prepareTurnRetrievalQueries is the request-owned query set for vector recall.
// The current query and configured number of recent completed conversations are
// separate from the Chroma result limit. Each completed conversation remains
// its own semantic query. It is retrieval context only and never becomes another
// final-payload block; no prompt phrase is classified.
func prepareTurnRetrievalQueries(req dto.PrepareTurnRequest, recentConversationLimit int) []prepareTurnRetrievalQuery {
	queries := make([]prepareTurnRetrievalQuery, 0, maxInt(recentConversationLimit, 0)+1)
	if query := strings.TrimSpace(stringPtrValue(req.ContinuityQuery, "")); query != "" {
		queries = append(queries, prepareTurnRetrievalQuery{Text: query, Source: "continuity_query"})
	} else if rawUserInput := strings.TrimSpace(stringPtrValue(req.RawUserInput, "")); rawUserInput != "" {
		queries = append(queries, prepareTurnRetrievalQuery{Text: rawUserInput, Source: "raw_user_input"})
	} else {
		for i := len(req.Messages) - 1; i >= 0; i-- {
			msg := req.Messages[i]
			if strings.EqualFold(strings.TrimSpace(fmt.Sprint(msg["role"])), "assistant") {
				continue
			}
			if query := strings.TrimSpace(fmt.Sprint(msg["content"])); query != "" {
				queries = append(queries, prepareTurnRetrievalQuery{Text: query, Source: "latest_non_assistant_message"})
				break
			}
		}
	}

	queries = append(queries, prepareTurnRecentConversationQueries(req.Messages, recentConversationLimit)...)
	return queries
}

// prepareTurnEffectiveContinuityQuery is the fact-scoring representation of
// the same request query set used by vector recall. Individual raw messages
// remain separate embedding inputs in prepareTurnVectorShadow.
func prepareTurnEffectiveContinuityQuery(req dto.PrepareTurnRequest, recentConversationLimit int) (string, string) {
	queries := prepareTurnRetrievalQueries(req, recentConversationLimit)
	if len(queries) == 0 {
		return "", "unobserved"
	}
	texts := make([]string, 0, len(queries))
	recentConversationCount := 0
	for _, query := range queries {
		texts = append(texts, query.Text)
		if query.Source == "recent_conversation_turn" {
			recentConversationCount++
		}
	}
	source := queries[0].Source
	if recentConversationCount > 0 {
		if len(queries) == recentConversationCount {
			source = "recent_conversation_turns"
		} else {
			source += "+recent_conversation_turns"
		}
	}
	return strings.Join(texts, "\n"), source
}

func (s *Server) prepareTurnVectorShadow(ctx context.Context, req dto.PrepareTurnRequest, limit int, historyScopes ...prepareTurnHistoryScope) map[string]any {
	return s.prepareTurnVectorShadowWithPreciseCandidateLimits(ctx, req, limit, nil, historyScopes...)
}

func (s *Server) prepareTurnVectorShadowWithPreciseCandidateLimits(ctx context.Context, req dto.PrepareTurnRequest, limit int, preciseCandidateLimits map[string]int, historyScopes ...prepareTurnHistoryScope) map[string]any {
	limit = prepareTurnRecallLimit(limit)
	recentConversationLimit := prepareTurnRecentConversationReferenceLimit(req.Settings)
	retrievalQueries := prepareTurnRetrievalQueries(req, recentConversationLimit)
	effectiveQuery, effectiveQuerySource := prepareTurnEffectiveContinuityQuery(req, recentConversationLimit)
	recentConversationQueryCount := 0
	for _, query := range retrievalQueries {
		if query.Source == "recent_conversation_turn" {
			recentConversationQueryCount++
		}
	}
	shadow := map[string]any{
		"status":                          "unconfigured",
		"engine":                          "chromadb",
		"source":                          "go_r1_read_shadow",
		"note":                            "ChromaDB is the 2.0 vector accelerator; MariaDB remains canonical truth",
		"configured":                      s.Cfg.Readiness.ChromaConfigured,
		"chromadb_endpoint_configured":    strings.TrimSpace(s.Cfg.ChromaEndpoint) != "",
		"recall_read_drill_enabled":       true,
		"product_read_enabled":            strings.TrimSpace(s.Cfg.ChromaEndpoint) != "" && s.VectorOpenError == nil,
		"live_retrieval_enabled":          false,
		"chromadb_live_enabled":           false,
		"health_checked":                  false,
		"search_attempted":                false,
		"backfill_attempted":              false,
		"query_text_source":               effectiveQuerySource,
		"query_text_chars":                len([]rune(effectiveQuery)),
		"query_text_count":                len(retrievalQueries),
		"recent_conversation_query_limit": recentConversationLimit,
		"recent_conversation_query_count": recentConversationQueryCount,
	}
	searchTiming := newBackendTimingTrace("")
	shadow["breakdown_ms"] = searchTiming.stagesMS
	defer finalizePrepareTurnVectorShadow(shadow)
	if s.Vector == nil {
		shadow["status"] = "disabled"
		shadow["health_error"] = "vector store is not configured"
		return shadow
	}
	healthStarted := time.Now()
	health, err := s.Vector.Health(ctx)
	searchTiming.addElapsed("health", healthStarted)
	shadow["health_checked"] = true
	if err != nil {
		shadow["status"] = "degraded"
		shadow["health_error"] = err.Error()
		return shadow
	}
	shadow["status"] = health.Status
	shadow["collection"] = health.Collection
	shadow["persist_dir"] = health.PersistDir
	shadow["total_count"] = health.TotalCount
	shadow["project_model"] = health.ProjectModel
	shadow["model_ready"] = health.ModelReady
	shadow["preflight_issues"] = health.PreflightIssues
	queryVector := clientMetaFloat32Vector(req.ClientMeta, "chroma_query_vector")
	queryVectors := [][]float32{}
	queryKey := "chroma_query_vector"
	if len(queryVector) > 0 {
		queryVectors = append(queryVectors, queryVector)
		shadow["query_vector_supplied_count"] = 1
		shadow["query_history_embedding_skipped_count"] = maxInt(len(retrievalQueries)-1, 0)
	} else {
		shadow["query_embedding_attempted"] = true
		embeddingCfg := s.completeTurnExtractionConfig(req.ClientMeta).Embedder
		shadow["query_embedding_configured"] = embeddingCfg.hasConfig()
		shadow["query_embedding_model"] = strings.TrimSpace(embeddingCfg.Model)
		if !embeddingCfg.hasConfig() {
			shadow["search_skipped_reason"] = "missing_chroma_query_vector_and_embedding_config"
			shadow["query_embedding_missing_fields"] = embeddingCfg.missingFields()
			return shadow
		}
		if len(retrievalQueries) == 0 {
			shadow["search_skipped_reason"] = "missing_query_text_for_embedding"
			return shadow
		}
		historyEmbeddingErrors := []string{}
		model := strings.TrimSpace(embeddingCfg.Model)
		for index, query := range retrievalQueries {
			embeddingStarted := time.Now()
			embeddingJSON, resolvedModel, err := callQueryEmbedding(ctx, embeddingCfg, query.Text)
			searchTiming.addElapsed("embedding", embeddingStarted)
			if err != nil {
				if index == 0 {
					shadow["status"] = "degraded"
					shadow["query_embedding_status"] = "error"
					shadow["query_embedding_error"] = err.Error()
					shadow["search_skipped_reason"] = "query_embedding_failed"
					return shadow
				}
				historyEmbeddingErrors = append(historyEmbeddingErrors, err.Error())
				continue
			}
			vectorValue := parseFloat32JSONList(embeddingJSON)
			if len(vectorValue) == 0 {
				if index == 0 {
					shadow["status"] = "degraded"
					shadow["query_embedding_status"] = "empty"
					shadow["search_skipped_reason"] = "query_embedding_empty"
					return shadow
				}
				historyEmbeddingErrors = append(historyEmbeddingErrors, "query_embedding_empty")
				continue
			}
			queryVectors = append(queryVectors, vectorValue)
			if strings.TrimSpace(resolvedModel) != "" {
				model = strings.TrimSpace(resolvedModel)
			}
		}
		queryVector = queryVectors[0]
		queryKey = "server_query_embedding"
		if len(queryVectors) > 1 {
			queryKey = "server_query_embeddings"
		}
		shadow["query_embedding_status"] = "ok"
		shadow["query_embedding_model"] = model
		shadow["query_embedding_count"] = len(queryVectors)
		shadow["query_history_embedding_error_count"] = len(historyEmbeddingErrors)
		if len(historyEmbeddingErrors) > 0 {
			shadow["query_history_embedding_errors"] = historyEmbeddingErrors
		}
	}

	if strings.TrimSpace(s.Cfg.ChromaEndpoint) != "" && s.VectorOpenError == nil {
		shadow["source"] = "go_r2_chromadb_product_read"
		shadow["note"] = "R2 product read proof: ChromaDB search is enabled as the support-only vector accelerator"
		shadow["live_retrieval_enabled"] = true
		shadow["chromadb_live_enabled"] = true
	} else {
		shadow["note"] = "R2 bounded recall read drill: ChromaDB vector search remains support-only until endpoint readiness is configured"
	}
	candidateLimit := limit
	filter := strings.TrimSpace(clientMetaString(req.ClientMeta, "chroma_filter"))
	searchSessionIDs := prepareTurnVectorHistorySessionIDs(req.ChatSessionID, historyScopes)
	searchAcrossSessions := func(searchFilter func(string) string, perSessionLimits map[string]int) ([]vector.VectorDocument, error) {
		resultsByID := map[string]vector.VectorDocument{}
		resultOrder := []string{}
		var firstErr error
		for _, searchSessionID := range searchSessionIDs {
			sessionLimit := candidateLimit
			if perSessionLimits != nil {
				sessionLimit = perSessionLimits[searchSessionID]
				if sessionLimit <= 0 {
					continue
				}
			}
			for _, searchVector := range queryVectors {
				vectorStarted := time.Now()
				sessionResults, searchErr := s.Vector.Search(ctx, searchSessionID, searchVector, sessionLimit, searchFilter(searchSessionID))
				searchTiming.addElapsed("vector_search", vectorStarted)
				switch {
				case searchErr == nil:
					for index := range sessionResults {
						if strings.TrimSpace(sessionResults[index].ChatSessionID) == "" {
							sessionResults[index].ChatSessionID = searchSessionID
						}
						doc := sessionResults[index]
						key := strings.TrimSpace(doc.ChatSessionID) + "\x1f" + strings.TrimSpace(doc.ID)
						if strings.TrimSpace(doc.ID) == "" {
							key += "\x1f" + strings.TrimSpace(doc.SourceTable) + "\x1f" + strings.TrimSpace(doc.SourceRowID) + "\x1f" + strings.TrimSpace(doc.DocumentText)
						}
						previous, exists := resultsByID[key]
						if !exists {
							resultOrder = append(resultOrder, key)
						}
						if !exists || prepareTurnVectorDocumentRanksBefore(doc, previous) {
							resultsByID[key] = doc
						}
					}
				case errors.Is(searchErr, vector.ErrNotFound):
				default:
					if firstErr == nil {
						firstErr = searchErr
					}
				}
			}
		}
		results := make([]vector.VectorDocument, 0, len(resultOrder))
		for _, key := range resultOrder {
			results = append(results, resultsByID[key])
		}
		if len(results) > 0 {
			sort.SliceStable(results, func(i, j int) bool {
				return prepareTurnVectorDocumentRanksBefore(results[i], results[j])
			})
			return results, nil
		}
		if firstErr != nil {
			return nil, firstErr
		}
		return nil, vector.ErrNotFound
	}
	shadow["search_attempted"] = true
	shadow["query_vector_key"] = queryKey
	shadow["query_vector_dim"] = len(queryVector)
	shadow["query_vector_count"] = len(queryVectors)
	shadow["limit"] = limit
	shadow["candidate_limit"] = candidateLimit
	shadow["candidate_policy"] = "ui_configured_vector_recall_limit_per_worldline_history_session"
	shadow["filter"] = filter
	shadow["history_session_ids"] = searchSessionIDs
	results, err := searchAcrossSessions(func(searchSessionID string) string {
		if filter != "" {
			return filter
		}
		return fmt.Sprintf("chat_session_id == %q", searchSessionID)
	}, nil)
	switch {
	case err == nil:
		revisionStarted := time.Now()
		results, revisionFilter := s.filterPrepareTurnActiveSourceRevisionVectors(ctx, req.ChatSessionID, results)
		searchTiming.addElapsed("revision_checks", revisionStarted)
		shadow["source_revision_filter"] = revisionFilter
		if len(results) == 0 {
			shadow["search_result"] = "not_found"
		} else {
			shadow["search_result"] = "ok"
		}
		shadow["search_result_count"] = len(results)
		shadow["search_results"] = vectorDocumentSearchPreview(results)
	case errors.Is(err, vector.ErrNotFound):
		shadow["search_result"] = "not_found"
		shadow["search_result_count"] = 0
		shadow["search_results"] = []map[string]any{}
	case errors.Is(err, vector.ErrNotEnabled):
		shadow["status"] = "degraded"
		shadow["search_result"] = "err_not_enabled"
		shadow["search_result_count"] = 0
		shadow["search_results"] = []map[string]any{}
	default:
		shadow["status"] = "degraded"
		shadow["search_result"] = "error"
		shadow["search_error"] = err.Error()
	}
	memoryFilter := `tier == "memory"`
	shadow["memory_search_attempted"] = true
	shadow["memory_search_filter"] = memoryFilter
	memoryResults, memoryErr := searchAcrossSessions(func(string) string { return memoryFilter }, nil)
	switch {
	case memoryErr == nil:
		revisionStarted := time.Now()
		memoryResults, revisionFilter := s.filterPrepareTurnActiveSourceRevisionVectors(ctx, req.ChatSessionID, memoryResults)
		searchTiming.addElapsed("revision_checks", revisionStarted)
		shadow["memory_source_revision_filter"] = revisionFilter
		if len(memoryResults) == 0 {
			shadow["memory_search_result"] = "not_found"
		} else {
			shadow["memory_search_result"] = "ok"
		}
		shadow["memory_search_result_count"] = len(memoryResults)
		shadow["memory_search_results"] = vectorDocumentSearchPreview(memoryResults)
	case errors.Is(memoryErr, vector.ErrNotFound):
		shadow["memory_search_result"] = "not_found"
		shadow["memory_search_result_count"] = 0
		shadow["memory_search_results"] = []map[string]any{}
	case errors.Is(memoryErr, vector.ErrNotEnabled):
		shadow["memory_search_result"] = "err_not_enabled"
		shadow["memory_search_result_count"] = 0
		shadow["memory_search_results"] = []map[string]any{}
	default:
		shadow["memory_search_result"] = "error"
		shadow["memory_search_result_count"] = 0
		shadow["memory_search_results"] = []map[string]any{}
		shadow["memory_search_error"] = memoryErr.Error()
	}
	preciseCandidateCount := 0
	for _, count := range preciseCandidateLimits {
		preciseCandidateCount += maxInt(count, 0)
	}
	shadow["precise_memory_search_candidate_count"] = preciseCandidateCount
	preciseMemoryFilter := `source_table == "precise_memory_units"`
	shadow["precise_memory_search_filter"] = preciseMemoryFilter
	if preciseCandidateCount <= 0 {
		shadow["precise_memory_search_attempted"] = false
		shadow["precise_memory_search_result"] = "no_canonical_candidates"
		shadow["precise_memory_search_result_count"] = 0
		return shadow
	}
	shadow["precise_memory_search_attempted"] = true
	preciseResults, preciseErr := searchAcrossSessions(func(string) string { return preciseMemoryFilter }, preciseCandidateLimits)
	switch {
	case preciseErr == nil:
		shadow["precise_memory_search_result"] = "ok"
		shadow["precise_memory_search_result_count"] = len(preciseResults)
		// This private handoff is consumed and removed by prepare-turn before the
		// public response is assembled. It reuses the same query vector and never
		// becomes a second payload or persistence surface.
		shadow[prepareTurnPrivatePreciseMemorySearchResultsKey] = vectorDocumentSearchPreview(preciseResults)
	case errors.Is(preciseErr, vector.ErrNotFound):
		shadow["precise_memory_search_result"] = "not_found"
		shadow["precise_memory_search_result_count"] = 0
	case errors.Is(preciseErr, vector.ErrNotEnabled):
		shadow["precise_memory_search_result"] = "err_not_enabled"
		shadow["precise_memory_search_result_count"] = 0
	default:
		shadow["precise_memory_search_result"] = "error"
		shadow["precise_memory_search_result_count"] = 0
		shadow["precise_memory_search_error"] = preciseErr.Error()
	}
	return shadow
}

func prepareTurnLoadGeneralPreciseMemoryUnits(
	ctx context.Context,
	reader store.GeneralVectorPreciseMemoryReader,
	fallbackSessionID string,
	historyScope prepareTurnHistoryScope,
) (map[string][]store.PreciseMemoryUnit, map[string]int, map[string]any) {
	unitsBySession := map[string][]store.PreciseMemoryUnit{}
	candidateLimits := map[string]int{}
	trace := map[string]any{
		"contract_version": "prepare_turn.precise_fact_candidate_snapshot.v1",
		"status":           "unavailable",
		"candidate_count":  0,
		"session_count":    0,
		"scope_dropped":    0,
	}
	if reader == nil {
		trace["reason"] = "general_precise_memory_reader_unavailable"
		return unitsBySession, candidateLimits, trace
	}
	segmentsBySession := map[string][]prepareTurnHistorySegment{}
	for _, segment := range historyScope.Segments {
		segmentsBySession[segment.SessionID] = append(segmentsBySession[segment.SessionID], segment)
	}
	errorsBySession := []string{}
	for _, sid := range prepareTurnVectorHistorySessionIDs(fallbackSessionID, []prepareTurnHistoryScope{historyScope}) {
		units, err := reader.ListGeneralVectorPreciseMemoryUnits(ctx, sid)
		if err != nil {
			if !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
				errorsBySession = append(errorsBySession, sid+": "+err.Error())
			}
			continue
		}
		filtered := make([]store.PreciseMemoryUnit, 0, len(units))
		for _, unit := range units {
			if strings.TrimSpace(unit.ChatSessionID) != sid || !store.PreciseMemoryGeneralVectorEligible(&unit) {
				continue
			}
			turn := maxInt(unit.SourceTurnStart, unit.SourceTurnEnd)
			inScope := len(segmentsBySession) == 0
			for _, segment := range segmentsBySession[sid] {
				if prepareTurnHistorySegmentContains(segment, turn) {
					inScope = true
					break
				}
			}
			if !inScope {
				trace["scope_dropped"] = intFromAny(trace["scope_dropped"], 0) + 1
				continue
			}
			filtered = append(filtered, unit)
		}
		if len(filtered) > 0 {
			unitsBySession[sid] = filtered
			candidateLimits[sid] = len(filtered)
			trace["candidate_count"] = intFromAny(trace["candidate_count"], 0) + len(filtered)
		}
	}
	trace["session_count"] = len(unitsBySession)
	trace["read_error_count"] = len(errorsBySession)
	if len(errorsBySession) > 0 {
		trace["status"] = "partial"
		trace["read_errors"] = errorsBySession
	} else {
		trace["status"] = "ready"
	}
	return unitsBySession, candidateLimits, trace
}

func prepareTurnVectorHitLooksLikePreciseMemory(hit map[string]any) bool {
	sourceTable := strings.ToLower(strings.TrimSpace(stringFromMap(hit, "source_table")))
	if sourceTable != "" {
		return sourceTable == "precise_memory_units"
	}
	tier := strings.ToLower(strings.TrimSpace(stringFromMap(hit, "tier")))
	if tier == "precise_memory" {
		return true
	}
	id := strings.ToLower(strings.TrimSpace(stringFromMap(hit, "id")))
	return strings.HasPrefix(id, "precise_memory:")
}

// prepareTurnHydratePreciseMemoryVectorFacts converts precise-memory vector
// hits into MariaDB-backed fact score observations. Production prepare-turn
// supplies a canonical unit snapshot and a dedicated tier search made with the
// already-created query vector. The variadic snapshot keeps older direct unit
// tests and diagnostic callers compatible with the broad-search observation.
// Missing vector observations never become an eligibility condition for the
// existing memory path.
func prepareTurnHydratePreciseMemoryVectorFacts(
	ctx context.Context,
	reader store.GeneralVectorPreciseMemoryReader,
	vectorShadow map[string]any,
	historyScope prepareTurnHistoryScope,
	preloadedUnits ...map[string][]store.PreciseMemoryUnit,
) ([]prepareTurnPrioritySemanticFact, map[string]any) {
	trace := map[string]any{
		"contract_version": "prepare_turn.precise_fact_vector_hydration.v1",
		"status":           "not_attempted",
		"score_owner":      "precise_memory_unit_vector_similarity",
		"search_owner":     "existing_prepare_turn_broad_vector_search",
		"input_hit_count":  0,
		"hydrated_count":   0,
		"missing_count":    0,
		"scope_dropped":    0,
	}
	if reader == nil {
		trace["status"] = "unavailable"
		trace["reason"] = "general_precise_memory_reader_unavailable"
		return nil, trace
	}
	searchResults := vectorShadow["search_results"]
	searchResult := strings.TrimSpace(stringFromMap(vectorShadow, "search_result"))
	if dedicatedResults, ok := vectorShadow[prepareTurnPrivatePreciseMemorySearchResultsKey]; ok {
		searchResults = dedicatedResults
		searchResult = strings.TrimSpace(stringFromMap(vectorShadow, "precise_memory_search_result"))
		trace["search_owner"] = "dedicated_precise_memory_tier_same_query_vector"
	}
	if searchResult != "ok" {
		trace["status"] = "skipped"
		trace["reason"] = searchResult
		return nil, trace
	}
	type scoreObservation struct {
		score  float64
		source string
	}
	hits := map[string]scoreObservation{}
	hitOrder := []string{}
	sessions := map[string]bool{}
	for _, hit := range prepareTurnVectorSearchResultMaps(searchResults) {
		if !prepareTurnVectorHitLooksLikePreciseMemory(hit) {
			continue
		}
		score, ok := prepareTurnVectorHitSimilarity(hit)
		if !ok || !prepareTurnVectorSimilarityEligible(score, stringFromMap(hit, "similarity_source")) {
			continue
		}
		unitID := strings.TrimSpace(stringFromMap(hit, "source_row_id"))
		if unitID == "" {
			id := strings.TrimSpace(stringFromMap(hit, "id"))
			parts := strings.SplitN(id, ":", 3)
			if len(parts) == 3 {
				unitID = strings.TrimSpace(parts[2])
			}
		}
		sid := strings.TrimSpace(stringFromMap(hit, "chat_session_id"))
		if unitID == "" || sid == "" {
			continue
		}
		key := sid + "\x1f" + unitID
		if previous, exists := hits[key]; exists && previous.score >= score {
			continue
		}
		if _, exists := hits[key]; !exists {
			hitOrder = append(hitOrder, key)
		}
		hits[key] = scoreObservation{score: score, source: strings.TrimSpace(stringFromMap(hit, "similarity_source"))}
		sessions[sid] = true
	}
	trace["input_hit_count"] = len(hits)
	if len(hits) == 0 {
		trace["status"] = "ready"
		trace["reason"] = "no_precise_memory_vector_hits"
		return nil, trace
	}

	segmentsBySession := map[string][]prepareTurnHistorySegment{}
	for _, segment := range historyScope.Segments {
		segmentsBySession[segment.SessionID] = append(segmentsBySession[segment.SessionID], segment)
	}
	unitsByKey := map[string]store.PreciseMemoryUnit{}
	readErrors := []string{}
	for sid := range sessions {
		var units []store.PreciseMemoryUnit
		if len(preloadedUnits) > 0 && preloadedUnits[0] != nil {
			units = preloadedUnits[0][sid]
		} else {
			loaded, err := reader.ListGeneralVectorPreciseMemoryUnits(ctx, sid)
			if err != nil {
				if !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
					readErrors = append(readErrors, sid+": "+err.Error())
				}
				continue
			}
			units = loaded
		}
		for _, unit := range units {
			if strings.TrimSpace(unit.ChatSessionID) != sid || !store.PreciseMemoryGeneralVectorEligible(&unit) {
				continue
			}
			turn := maxInt(unit.SourceTurnStart, unit.SourceTurnEnd)
			inScope := len(segmentsBySession) == 0
			for _, segment := range segmentsBySession[sid] {
				if prepareTurnHistorySegmentContains(segment, turn) {
					inScope = true
					break
				}
			}
			if !inScope {
				trace["scope_dropped"] = intFromAny(trace["scope_dropped"], 0) + 1
				continue
			}
			key := sid + "\x1f" + strings.TrimSpace(unit.UnitID)
			if _, wanted := hits[key]; wanted {
				unitsByKey[key] = unit
			}
		}
	}
	facts := []prepareTurnPrioritySemanticFact{}
	for _, key := range hitOrder {
		unit, ok := unitsByKey[key]
		if !ok {
			trace["missing_count"] = intFromAny(trace["missing_count"], 0) + 1
			continue
		}
		fact, ok := prepareTurnPrioritySemanticFactFromPreciseUnit(unit, hits[key].score, hits[key].source)
		if !ok {
			trace["missing_count"] = intFromAny(trace["missing_count"], 0) + 1
			continue
		}
		facts = append(facts, fact)
	}
	trace["hydrated_count"] = len(facts)
	trace["read_error_count"] = len(readErrors)
	if len(readErrors) > 0 {
		trace["status"] = "partial"
		trace["read_errors"] = readErrors
	} else {
		trace["status"] = "ready"
	}
	return facts, trace
}

func prepareTurnVectorHistorySessionIDs(fallbackSessionID string, historyScopes []prepareTurnHistoryScope) []string {
	sessionIDs := []string{}
	seen := map[string]bool{}
	if len(historyScopes) > 0 {
		for _, segment := range historyScopes[0].Segments {
			sessionID := strings.TrimSpace(segment.SessionID)
			if sessionID != "" && !seen[sessionID] {
				seen[sessionID] = true
				sessionIDs = append(sessionIDs, sessionID)
			}
		}
		return sessionIDs
	}
	fallbackSessionID = strings.TrimSpace(fallbackSessionID)
	if fallbackSessionID != "" {
		sessionIDs = append(sessionIDs, fallbackSessionID)
	}
	return sessionIDs
}

func (s *Server) filterPrepareTurnActiveSourceRevisionVectors(
	ctx context.Context,
	fallbackSessionID string,
	documents []vector.VectorDocument,
) ([]vector.VectorDocument, map[string]any) {
	trace := map[string]any{
		"status":         "unavailable",
		"input_count":    len(documents),
		"retained_count": len(documents),
		"dropped_count":  0,
		"checked_count":  0,
	}
	sourceRevisions, ok := s.Store.(store.SourceRevisionStore)
	if !ok {
		return documents, trace
	}
	if availability, exists := s.Store.(store.MemoryDerivationLifecycleAvailability); exists &&
		!availability.MemoryDerivationLifecycleEnabled() {
		trace["status"] = "disabled"
		return documents, trace
	}

	type sourceRevisionCheck struct {
		active bool
		err    error
	}
	checks := map[string]sourceRevisionCheck{}
	filtered := make([]vector.VectorDocument, 0, len(documents))
	droppedMissingRevision := 0
	droppedInactive := 0
	droppedCheckError := 0
	for _, document := range documents {
		metadata := document.Metadata
		revision := strings.TrimSpace(extractionStringFromAny(metadata["source_revision"]))
		revisionBacked := revision != "" ||
			strings.TrimSpace(extractionStringFromAny(metadata["source_contract"])) != "" ||
			strings.TrimSpace(extractionStringFromAny(metadata["index_identity"])) != "" ||
			strings.TrimSpace(extractionStringFromAny(metadata["content_fingerprint"])) != ""
		if !revisionBacked {
			// Once the canonical lifecycle is enabled, an unversioned vector
			// cannot be proven to belong to an active source. Keep it out of
			// previews and ranking until the normal reindex path replaces it
			// with revision-backed metadata.
			droppedMissingRevision++
			continue
		}
		if revision == "" {
			droppedMissingRevision++
			continue
		}
		sessionID := strings.TrimSpace(document.ChatSessionID)
		if sessionID == "" {
			sessionID = strings.TrimSpace(fallbackSessionID)
		}
		if sessionID == "" {
			droppedMissingRevision++
			continue
		}
		key := sessionID + "\x00" + revision
		check, exists := checks[key]
		if !exists {
			check.active, check.err = sourceRevisions.IsSourceRevisionActive(ctx, sessionID, revision)
			checks[key] = check
		}
		if check.err != nil {
			droppedCheckError++
			continue
		}
		if !check.active {
			droppedInactive++
			continue
		}
		filtered = append(filtered, document)
	}

	dropped := len(documents) - len(filtered)
	trace["status"] = "applied"
	trace["retained_count"] = len(filtered)
	trace["dropped_count"] = dropped
	trace["checked_count"] = len(checks)
	trace["dropped_missing_revision"] = droppedMissingRevision
	trace["dropped_inactive"] = droppedInactive
	trace["dropped_check_error"] = droppedCheckError
	return filtered, trace
}

func finalizePrepareTurnVectorShadow(shadow map[string]any) {
	readiness := buildPrepareTurnVectorReadiness(shadow)
	shadow["index_readiness"] = readiness
	shadow["fallback_recommended"] = boolFromAny(readiness["fallback_recommended"])
	shadow["reindex_recommended"] = boolFromAny(readiness["reindex_recommended"])
	shadow["degrade_mode"] = readiness["degrade_mode"]
}

func buildPrepareTurnVectorReadiness(shadow map[string]any) map[string]any {
	if shadow == nil {
		return map[string]any{
			"status":                        "disabled",
			"ready":                         false,
			"reason":                        "vector_shadow_missing",
			"fallback_recommended":          true,
			"reindex_recommended":           false,
			"degrade_mode":                  "public_lexical_relevant_deep",
			"fallback_lane":                 "mariadb_public_projection",
			"embedding_ready_before_search": false,
		}
	}
	status := strings.TrimSpace(stringFromMap(shadow, "status"))
	if status == "" {
		status = "unknown"
	}
	searchAttempted := boolFromAny(shadow["search_attempted"])
	searchResult := strings.TrimSpace(stringFromMap(shadow, "search_result"))
	modelReady := boolFromAny(shadow["model_ready"])
	totalCount := intFromAny(shadow["total_count"], 0)
	configured := boolFromAny(shadow["configured"]) || boolFromAny(shadow["chromadb_endpoint_configured"])
	reason := ""
	ready := false
	reindexRecommended := false
	switch {
	case status == "disabled":
		reason = "vector_store_disabled"
	case !configured:
		reason = "chromadb_unconfigured"
	case strings.TrimSpace(stringFromMap(shadow, "health_error")) != "":
		reason = "vector_health_error"
		reindexRecommended = true
	case strings.TrimSpace(stringFromMap(shadow, "query_embedding_error")) != "":
		reason = "query_embedding_failed"
	case !modelReady:
		reason = "embedding_model_not_ready"
		reindexRecommended = true
	case totalCount <= 0:
		reason = "vector_index_empty_or_not_reindexed"
		reindexRecommended = true
	case searchAttempted && searchResult == "error":
		reason = "vector_search_error"
		reindexRecommended = true
	case searchAttempted && searchResult == "err_not_enabled":
		reason = "vector_search_not_enabled"
	default:
		reason = "ready"
		ready = true
	}
	if searchAttempted && searchResult == "not_found" && reason == "ready" {
		reason = "searchable_no_hits"
	}
	fallbackRecommended := !ready || (searchAttempted && searchResult != "" && searchResult != "ok")
	return map[string]any{
		"status":                        reason,
		"ready":                         ready,
		"configured":                    configured,
		"engine_status":                 status,
		"model_ready":                   modelReady,
		"total_count":                   totalCount,
		"search_attempted":              searchAttempted,
		"search_result":                 nilIfEmpty(searchResult),
		"fallback_recommended":          fallbackRecommended,
		"fallback_lane":                 "mariadb_public_projection",
		"degrade_mode":                  "public_lexical_relevant_deep",
		"reindex_recommended":           reindexRecommended,
		"embedding_ready_before_search": modelReady && totalCount > 0,
	}
}

func clientMetaFloat32Vector(meta map[string]any, key string) []float32 {
	if meta == nil {
		return nil
	}
	value, ok := meta[key]
	if !ok {
		return nil
	}
	switch typed := value.(type) {
	case []float32:
		return typed
	case []float64:
		out := make([]float32, 0, len(typed))
		for _, item := range typed {
			out = append(out, float32(item))
		}
		return out
	case []any:
		out := make([]float32, 0, len(typed))
		for _, item := range typed {
			switch n := item.(type) {
			case float64:
				out = append(out, float32(n))
			case float32:
				out = append(out, n)
			case int:
				out = append(out, float32(n))
			default:
				return nil
			}
		}
		return out
	default:
		return nil
	}
}

func clientMetaString(meta map[string]any, key string) string {
	if meta == nil {
		return ""
	}
	value, ok := meta[key]
	if !ok {
		return ""
	}
	s, _ := value.(string)
	return s
}

func prepareTurnPerspectiveContextFromClientMeta(meta map[string]any) map[string]any {
	if len(meta) == 0 {
		return nil
	}
	for _, nestedKey := range []string{"perspective_context", "viewpoint_context", "pov_context"} {
		if nested := normalizePrepareTurnPerspectiveContext(mapFromAny(meta[nestedKey])); len(nested) > 0 {
			if _, ok := nested["source"]; !ok {
				nested["source"] = nestedKey
			}
			return nested
		}
	}
	persona := mapFromAny(meta["risu_persona_observation"])
	if extractionStringFromAny(persona["contract_version"]) == "risu_persona_observation.v1" &&
		extractionStringFromAny(persona["observation_state"]) == "observed" {
		if personaName := strings.TrimSpace(extractionStringFromAny(persona["persona_name"])); personaName != "" {
			return normalizePrepareTurnPerspectiveContext(map[string]any{
				"current_pov": personaName,
				"source":      "risu_persona_observation",
				"mode":        "active_user_persona_knowledge_holder",
			})
		}
	}
	return normalizePrepareTurnPerspectiveContext(meta)
}

func prepareTurnPerspectiveContextFromRequest(req dto.PrepareTurnRequest) map[string]any {
	if ctx := prepareTurnPerspectiveContextFromClientMeta(req.ClientMeta); len(ctx) > 0 {
		return ctx
	}
	return nil
}

func normalizePrepareTurnPerspectiveContext(raw map[string]any) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	pov := strings.TrimSpace(extractionFirstNonEmpty(
		extractionStringFromAny(raw["current_pov"]),
		extractionStringFromAny(raw["pov_character"]),
		extractionStringFromAny(raw["viewpoint_character"]),
		extractionStringFromAny(raw["narrator_character"]),
		extractionStringFromAny(raw["speaker_character"]),
		extractionStringFromAny(raw["current_speaker"]),
		extractionStringFromAny(raw["speaker"]),
		extractionStringFromAny(raw["current_character"]),
		extractionStringFromAny(raw["active_character"]),
	))
	entityID := strings.TrimSpace(extractionStringFromAny(raw["current_pov_entity_id"]))
	if pov == "" && entityID == "" {
		return nil
	}
	out := map[string]any{
		"contract_version": "perspective_context.v1",
		"source":           extractionFirstNonEmpty(extractionStringFromAny(raw["source"]), "client_meta"),
	}
	if pov != "" {
		out["current_pov"] = pov
		out["current_pov_key"] = normalizeCharacterKey(pov)
	}
	if entityID != "" {
		out["current_pov_entity_id"] = entityID
	}
	if identityState := strings.TrimSpace(extractionStringFromAny(raw["identity_state"])); identityState != "" {
		out["identity_state"] = identityState
	}
	if mode := strings.TrimSpace(extractionStringFromAny(raw["mode"])); mode != "" {
		out["mode"] = mode
	}
	return out
}

func buildInjectionText(memories []store.Memory, kgTriples []store.KGTriple, storylines []store.Storyline, worldRules []store.WorldRule, charStates []store.CharacterState, pendingThreads []store.PendingThread, topK, maxChars int) (string, bool) {
	assembly := buildPrepareTurnInjectionAssembly(memories, kgTriples, nil, nil, storylines, worldRules, charStates, pendingThreads, nil, nil, nil, nil, nil, topK, maxChars, "", "default", nil, nil, nil)
	return assembly.Text, assembly.Truncated
}

func prepareTurnIntSetting(value, fallback *int) int {
	if value != nil && *value > 0 {
		return *value
	}
	if fallback != nil && *fallback > 0 {
		return *fallback
	}
	return 1
}

func prepareTurnRecallLimit(topK int) int {
	if topK > 0 {
		return topK
	}
	return 1
}

func prepareTurnTextBudget(maxChars int) int {
	if maxChars > 0 {
		return maxChars
	}
	return 1
}

type prepareTurnMemoryLaneSelection struct {
	VectorRelevant          []store.Memory
	Recent                  []store.Memory
	Relevant                []store.Memory
	Deep                    []store.Memory
	ProtectedSelected       []store.Memory
	DirectlyReferenced      []string
	ProtectedCandidates     []store.Memory
	ProtectedAliasCanonical map[string]string
	ProtectedAmbiguousAlias map[string]bool
	VectorScores            map[string]float64
	RelevantScores          map[string]float64
	Trace                   map[string]any
}

func prepareTurnEventMemoryQuery(ctx prepareTurnRecollectionContext, directlyReferencedEntities []string) string {
	return strings.TrimSpace(strings.Join(nonEmptyStrings([]string{
		ctx.rawUserInput,
		ctx.previousEventSummary,
		strings.Join(directlyReferencedEntities, "\n"),
	}), "\n"))
}

type prepareTurnRecallEvidence struct {
	Eligible          bool
	ExactPhrase       bool
	LexicalOverlap    bool
	StructuredAnchors []string
	OverlapTerms      []string
}

func prepareTurnMemoryRecallEvidence(query string, item store.Memory) prepareTurnRecallEvidence {
	query = strings.TrimSpace(query)
	if query == "" {
		return prepareTurnRecallEvidence{}
	}
	evidence := prepareTurnRecallEvidence{}
	for _, anchor := range prepareTurnMemoryStructuredAnchors(item) {
		if prepareTurnRecallContainsAnchor(query, anchor) {
			evidence.StructuredAnchors = append(evidence.StructuredAnchors, anchor)
		}
	}
	queryTerms := prepareTurnDistinctiveRecallTerms(query, evidence.StructuredAnchors...)
	memoryTerms := map[string]bool{}
	for _, term := range prepareTurnRecallTerms(prepareTurnMemoryRelevanceText(item)) {
		memoryTerms[term] = true
	}
	for _, term := range queryTerms {
		if memoryTerms[term] {
			evidence.OverlapTerms = append(evidence.OverlapTerms, term)
		}
	}
	memoryText := prepareTurnMemoryRelevanceText(item)
	evidence.ExactPhrase = prepareTurnSharedRecallPhrase(query, memoryText)
	evidence.LexicalOverlap = len(evidence.OverlapTerms) >= prepareTurnRecallRequiredOverlap(queryTerms, memoryText)
	parsed := parseJSONMap(item.SummaryJSON)
	protected := len(sliceFromAny(parsed["protected_secrets"])) > 0 ||
		len(sliceFromAny(parsed["character_identity_accuracy"])) > 0
	evidence.Eligible = evidence.ExactPhrase || evidence.LexicalOverlap ||
		(protected && len(evidence.StructuredAnchors) > 0)
	return evidence
}

func prepareTurnSupportRecallEligible(query, text string, anchors ...string) bool {
	query = strings.TrimSpace(query)
	text = strings.TrimSpace(text)
	if query == "" || text == "" {
		return query == "" && text != ""
	}
	if prepareTurnSharedRecallPhrase(query, text) {
		return true
	}
	queryTerms := prepareTurnDistinctiveRecallTerms(query, anchors...)
	overlap := prepareTurnDistinctiveRecallOverlapCount(queryTerms, text)
	return overlap >= prepareTurnRecallRequiredOverlap(queryTerms, text)
}

// An exact adjacent term pair is request-local evidence without requiring a
// language-specific stop-word list. It preserves concise phrases such as a
// location or task name that can be lost by single-token frequency filtering.
func prepareTurnSharedRecallPhrase(query, text string) bool {
	sequenceTerms := func(value string) []string {
		out := []string{}
		for _, term := range strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
			return !(r == '_' || r == '-' || unicode.IsLetter(r) || unicode.IsNumber(r))
		}) {
			if term = strings.TrimSpace(term); term != "" {
				out = append(out, term)
			}
		}
		return out
	}
	queryTerms := sequenceTerms(query)
	textTerms := sequenceTerms(text)
	if len(queryTerms) < 2 || len(textTerms) < 2 {
		return false
	}
	queryDistinctive := map[string]bool{}
	for _, term := range prepareTurnDistinctiveRecallTerms(query) {
		queryDistinctive[term] = true
	}
	textDistinctive := map[string]bool{}
	for _, term := range prepareTurnDistinctiveRecallTerms(text) {
		textDistinctive[term] = true
	}
	pairs := make(map[string]bool, len(queryTerms)-1)
	for i := 0; i+1 < len(queryTerms); i++ {
		if queryTerms[i] != queryTerms[i+1] &&
			(queryDistinctive[queryTerms[i]] || queryDistinctive[queryTerms[i+1]]) {
			pairs[queryTerms[i]+"\x00"+queryTerms[i+1]] = true
		}
	}
	for i := 0; i+1 < len(textTerms); i++ {
		if pairs[textTerms[i]+"\x00"+textTerms[i+1]] &&
			(textDistinctive[textTerms[i]] || textDistinctive[textTerms[i+1]]) {
			return true
		}
	}
	return false
}

func prepareTurnDistinctiveRecallOverlapCount(queryTerms []string, text string) int {
	textTerms := map[string]bool{}
	for _, term := range prepareTurnRecallTerms(text) {
		textTerms[term] = true
	}
	overlap := 0
	for _, term := range queryTerms {
		if textTerms[term] {
			overlap++
		}
	}
	return overlap
}

// prepareTurnDistinctiveRecallTerms derives request-local lexical evidence
// without a language stop-word table. Terms repeated inside the request are
// treated as low-information glue, and typed entity/scope anchors are removed
// from the lexical proof so a character name alone cannot reactivate every old
// memory attached to that character.
func prepareTurnDistinctiveRecallTerms(text string, anchors ...string) []string {
	all := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !(r == '_' || r == '-' || unicode.IsLetter(r) || unicode.IsNumber(r))
	})
	counts := map[string]int{}
	lengths := []int{}
	for _, term := range all {
		term = strings.TrimSpace(term)
		if term != "" {
			counts[term]++
			lengths = append(lengths, len([]rune(term)))
		}
	}
	sort.Ints(lengths)
	medianLength := 0
	minimumLength := 0
	if len(lengths) > 0 {
		medianLength = lengths[len(lengths)/2]
		minimumLength = lengths[0]
	}
	excluded := map[string]bool{}
	for _, anchor := range anchors {
		for _, term := range prepareTurnRecallTerms(anchor) {
			excluded[term] = true
		}
	}
	out := []string{}
	seen := map[string]bool{}
	for _, term := range all {
		term = strings.TrimSpace(term)
		termLength := len([]rune(term))
		lowInformationRepeat := counts[term] > 1 && termLength < medianLength
		shortestTier := len(lengths) > 1 && termLength == minimumLength
		if term == "" || seen[term] || excluded[term] || lowInformationRepeat || shortestTier {
			continue
		}
		seen[term] = true
		out = append(out, term)
	}
	if len(out) > 0 {
		return out
	}
	for _, term := range prepareTurnRecallTerms(text) {
		if !excluded[term] {
			out = append(out, term)
		}
	}
	return out
}

// The lexical proof grows with request complexity instead of using short/long
// context bands. It is a relevance criterion, not an item-count or turn cap.
func prepareTurnDynamicOverlapRequirement(distinctiveTermCount int) int {
	if distinctiveTermCount <= 0 {
		return 1
	}
	return 1 + int(math.Ceil(math.Log10(float64(distinctiveTermCount))))
}

func prepareTurnRecallRequiredOverlap(queryTerms []string, candidateText string) int {
	queryRequirement := prepareTurnDynamicOverlapRequirement(len(queryTerms))
	candidateTerms := prepareTurnDistinctiveRecallTerms(candidateText)
	if len(candidateTerms) == 0 {
		return queryRequirement
	}
	candidateMajority := (len(candidateTerms) + 1) / 2
	if candidateMajority < queryRequirement {
		return candidateMajority
	}
	return queryRequirement
}

func prepareTurnRequestFirstRelevant(rawQuery, fallbackQuery, text string, anchors ...string) bool {
	rawQuery = strings.TrimSpace(rawQuery)
	if rawQuery != "" && prepareTurnSupportRecallEligible(rawQuery, text, anchors...) {
		return true
	}
	if strings.TrimSpace(fallbackQuery) == "" {
		return false
	}
	return prepareTurnSupportRecallEligible(fallbackQuery, text, anchors...)
}

func prepareTurnRecallOverlapCount(query, text string) int {
	textTerms := map[string]bool{}
	for _, term := range prepareTurnRecallTerms(text) {
		textTerms[term] = true
	}
	overlap := 0
	for _, term := range prepareTurnRecallTerms(query) {
		if textTerms[term] {
			overlap++
		}
	}
	return overlap
}

func prepareTurnDirectEntityMentionRank(rawUserInput, characterName string, aliases map[string][]string) int {
	if prepareTurnRecallContainsAnchor(rawUserInput, characterName) {
		return 3
	}
	canonical := normalizePrepareTurnEntityNeedle(characterName)
	for _, alias := range aliases[canonical] {
		if prepareTurnRecallContainsAnchor(rawUserInput, alias) {
			return 2
		}
	}
	return 0
}

// A knowledge-graph edge is not current merely because one endpoint appears in
// the scene. It needs both endpoints, or an endpoint plus corroborating relation
// or event terms. This prevents every historical edge of a current character
// from consuming the relationship budget.
func prepareTurnKGRecallEligible(query string, triple store.KGTriple) (bool, string) {
	subjectMatched := prepareTurnRecallContainsAnchor(query, triple.Subject)
	objectMatched := prepareTurnRecallContainsAnchor(query, triple.Object)
	if subjectMatched && objectMatched {
		return true, "both_endpoints_matched"
	}
	if subjectMatched || objectMatched {
		complementaryEvidence := strings.TrimSpace(triple.Predicate)
		if subjectMatched {
			complementaryEvidence = strings.TrimSpace(complementaryEvidence + " " + triple.Object)
		} else {
			complementaryEvidence = strings.TrimSpace(triple.Subject + " " + complementaryEvidence)
		}
		if prepareTurnSupportRecallEligible(query, complementaryEvidence) {
			return true, "endpoint_plus_relation_evidence"
		}
		return false, "single_endpoint_only"
	}
	line := strings.TrimSpace(triple.Subject + " " + triple.Predicate + " " + triple.Object)
	if prepareTurnSupportRecallEligible(query, line) {
		return true, "relation_event_evidence"
	}
	return false, "unrelated"
}

func prepareTurnRecallTerms(text string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, term := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !(r == '_' || r == '-' || unicode.IsLetter(r) || unicode.IsNumber(r))
	}) {
		term = strings.TrimSpace(term)
		if term == "" || seen[term] {
			continue
		}
		seen[term] = true
		out = append(out, term)
	}
	return out
}

func prepareTurnRecallContainsAnchor(text, anchor string) bool {
	anchor = strings.TrimSpace(anchor)
	if anchor == "" {
		return false
	}
	return strings.Contains(normalizePrepareTurnEntityNeedle(text), normalizePrepareTurnEntityNeedle(anchor))
}

func prepareTurnMemoryStructuredAnchors(item store.Memory) []string {
	parsed := parseJSONMap(item.SummaryJSON)
	out := []string{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || stringSliceContains(out, value) {
			return
		}
		out = append(out, value)
	}
	for _, key := range []string{"characters", "character_names", "people", "places", "locations", "items", "factions"} {
		for _, value := range memorySearchStringValues(parsed[key]) {
			add(value)
		}
	}
	for _, entry := range memorySearchMapItems(parsed["entities"]) {
		for _, key := range []string{"name", "canonical_name", "display_name", "location"} {
			add(stringFromMap(entry, key))
		}
	}
	entityBuckets := mapFromAny(parsed["entities"])
	for _, bucket := range []string{"characters", "people", "locations", "places", "items", "objects", "groups", "factions"} {
		for _, entry := range memorySearchMapItems(entityBuckets[bucket]) {
			for _, key := range []string{"name", "canonical_name", "display_name", "label", "title", "location"} {
				add(stringFromMap(entry, key))
			}
		}
	}
	for _, entry := range memorySearchMapItems(parsed["character_states"]) {
		add(stringFromMap(entry, "name"))
		add(stringFromMap(entry, "location"))
	}
	for _, entry := range memorySearchMapItems(parsed["kg_triples"]) {
		add(stringFromMap(entry, "subject"))
		add(stringFromMap(entry, "object"))
	}
	for _, entry := range memorySearchMapItems(parsed["protected_secrets"]) {
		add(stringFromMap(entry, "owner"))
		for _, value := range memorySearchStringValues(entry["subject"]) {
			add(value)
		}
	}
	for _, entry := range memorySearchMapItems(parsed["character_identity_accuracy"]) {
		for _, key := range []string{"canonical_entity_name", "surface_identity_name", "true_identity_name", "public_identity_name", "alias_name", "real_identity_name"} {
			add(stringFromMap(entry, key))
		}
	}
	return out
}

func prepareTurnMemoryCharacterAnchors(item store.Memory) []string {
	parsed := parseJSONMap(item.SummaryJSON)
	out := []string{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || len([]rune(value)) < 2 || prepareTurnRelationshipNameInList(value, out) {
			return
		}
		out = append(out, value)
	}
	for _, key := range []string{"characters", "character_names", "people"} {
		for _, value := range memorySearchStringValues(parsed[key]) {
			add(value)
		}
	}
	for _, entry := range memorySearchMapItems(parsed["entities"]) {
		add(extractionFirstNonEmpty(
			stringFromMap(entry, "name"),
			stringFromMap(entry, "canonical_name"),
			stringFromMap(entry, "display_name"),
		))
	}
	for _, entry := range memorySearchMapItems(parsed["character_states"]) {
		add(extractionFirstNonEmpty(
			stringFromMap(entry, "name"),
			stringFromMap(entry, "character_name"),
		))
	}
	return out
}

func prepareTurnMemoryDirectEntityMatches(item store.Memory, directlyReferencedEntities []string) []string {
	anchors := prepareTurnMemoryStructuredAnchors(item)
	summary := prepareTurnMemorySummary(item)
	out := []string{}
	for _, entity := range directlyReferencedEntities {
		matched := prepareTurnRecallContainsAnchor(summary, entity)
		for _, anchor := range anchors {
			if normalizePrepareTurnEntityNeedle(entity) != normalizePrepareTurnEntityNeedle(anchor) {
				continue
			}
			matched = true
			break
		}
		if matched {
			if !prepareTurnRelationshipNameInList(entity, out) {
				out = append(out, entity)
			}
		}
	}
	return out
}

func selectPrepareTurnMemoryLanes(memories []store.Memory, query string, topK int) prepareTurnMemoryLaneSelection {
	return selectPrepareTurnMemoryLanesWithVector(memories, query, topK, nil)
}

func selectPrepareTurnMemoryLanesWithVector(memories []store.Memory, query string, topK int, vectorShadow map[string]any, directlyReferencedEntityGroups ...[]string) prepareTurnMemoryLaneSelection {
	return selectPrepareTurnMemoryLanesWithVectorHydrationSource(memories, memories, query, topK, vectorShadow, directlyReferencedEntityGroups...)
}

func selectPrepareTurnMemoryLanesWithVectorHydrationSource(memories, vectorHydrationMemories []store.Memory, query string, topK int, vectorShadow map[string]any, directlyReferencedEntityGroups ...[]string) prepareTurnMemoryLaneSelection {
	vectorLimit := prepareTurnRecallLimit(topK)
	directlyReferencedEntities := []string{}
	if len(directlyReferencedEntityGroups) > 0 {
		directlyReferencedEntities = directlyReferencedEntityGroups[0]
	}
	storedSceneEntities := []string{}
	if len(directlyReferencedEntityGroups) > 1 {
		storedSceneEntities = directlyReferencedEntityGroups[1]
	}
	directEntitiesOutsideStoredScene := []string{}
	for _, entity := range directlyReferencedEntities {
		if !prepareTurnRelationshipNameInList(entity, storedSceneEntities) {
			directEntitiesOutsideStoredScene = append(directEntitiesOutsideStoredScene, entity)
		}
	}
	clean := make([]store.Memory, 0, len(memories))
	for _, item := range memories {
		if strings.TrimSpace(prepareTurnMemorySummary(item)) == "" {
			continue
		}
		clean = append(clean, item)
	}
	// Read every already-materialized MariaDB candidate. Relevance and the final
	// Go-owned character budget decide delivery; an arbitrary row ceiling must
	// not make a long session forget older eligible memories.
	candidateLimit := len(clean)
	query = strings.TrimSpace(query)
	queryPresent := query != ""
	maxTurn := 0
	minTurn := 0
	importanceTotal := 0.0
	importanceSeen := 0
	for _, item := range clean {
		if item.TurnIndex > 0 {
			if minTurn == 0 || item.TurnIndex < minTurn {
				minTurn = item.TurnIndex
			}
			if item.TurnIndex > maxTurn {
				maxTurn = item.TurnIndex
			}
		}
		if item.Importance > 0 {
			importanceTotal += item.Importance
			importanceSeen++
		}
	}
	avgImportance := 0.0
	if importanceSeen > 0 {
		avgImportance = importanceTotal / float64(importanceSeen)
	}

	out := prepareTurnMemoryLaneSelection{
		DirectlyReferenced: append([]string(nil), directlyReferencedEntities...),
		VectorScores:       map[string]float64{},
		RelevantScores:     map[string]float64{},
		Trace: map[string]any{
			"version":                     "r3.recall_lanes.v1",
			"top_k_definition":            "vector_memory_search_limit_only",
			"top_k_memory_target":         vectorLimit,
			"candidate_safety_limit":      candidateLimit,
			"vector_memory_policy":        "chromadb_hits_hydrated_to_mariadb_memory_before_injection",
			"relevant_memory_limit":       "final_category_char_budget",
			"deep_memory_policy":          "importance_only_when_no_current_query",
			"input_memory_count":          len(memories),
			"eligible_memory_count":       len(clean),
			"recent_order":                "ranked_recency_tiebreak_for_non_relevant_memory",
			"relevant_order":              "query_overlap_first_then_importance_then_recency",
			"deep_order":                  "ranked_non_relevant_high_importance_support",
			"selection_policy":            "query_relevance_then_recent_fallback; old_importance_alone_cannot_outrank_current_scene",
			"query_present":               queryPresent,
			"input_rewrite_applied":       false,
			"raw_user_input_preserved":    true,
			"long_gap_policy":             "widen_by_lanes_not_by_replacing_user_input",
			"selection_reason_visibility": true,
		},
	}
	protectedAliasCanonical, protectedAmbiguousAliases := prepareTurnProtectedAliasResolution(clean)
	out.ProtectedAliasCanonical = protectedAliasCanonical
	out.ProtectedAmbiguousAlias = protectedAmbiguousAliases
	protectedCandidateMemories := map[string]bool{}
	for _, item := range clean {
		if !prepareTurnProtectedMemoryGuard(item).Active {
			continue
		}
		key := prepareTurnMemoryLaneKey(item)
		protectedCandidateMemories[key] = true
		out.ProtectedCandidates = append(out.ProtectedCandidates, item)
	}
	protectedSelectedCount := 0
	candidateLimitRejected := 0
	actualMemorySelectedCount := 0
	actualMemoryVectorSelectedCount := 0
	actualMemoryRelevantRefillSelectedCount := 0
	actualMemoryDeepRefillSelectedCount := 0
	actualMemoryRecentRefillSelectedCount := 0
	exactPhraseSelectedCount := 0
	lexicalSelectedCount := 0
	acceptProtectedCoverage := func(item store.Memory) (bool, bool) {
		if !prepareTurnProtectedMemoryGuard(item).Active {
			return true, false
		}
		memoryKey := prepareTurnMemoryLaneKey(item)
		if !protectedCandidateMemories[memoryKey] {
			protectedCandidateMemories[memoryKey] = true
			out.ProtectedCandidates = append(out.ProtectedCandidates, item)
		}
		if protectedSelectedCount >= candidateLimit {
			candidateLimitRejected++
			return false, true
		}
		return true, true
	}
	vectorCandidateLimit := len(prepareTurnVectorMemorySearchResultMaps(vectorShadow))
	if vectorCandidateLimit <= 0 {
		vectorCandidateLimit = vectorLimit
	}
	vectorHydration := prepareTurnHydrateVectorMemoryHits(vectorHydrationMemories, vectorShadow, vectorCandidateLimit)
	vectorRecallReady := prepareTurnVectorRecallReady(vectorHydration.Trace)
	vectorRecallAttempted := prepareTurnVectorSearchAttempted(vectorShadow)
	vectorScopeRejected := 0
	for _, item := range vectorHydration.Items {
		protected := prepareTurnProtectedMemoryGuard(item).Active
		if !protected && actualMemoryVectorSelectedCount >= vectorLimit {
			continue
		}
		if queryPresent && !protected && len(directEntitiesOutsideStoredScene) > 0 &&
			len(prepareTurnMemoryCharacterAnchors(item)) > 0 &&
			len(prepareTurnMemoryDirectEntityMatches(item, directEntitiesOutsideStoredScene)) == 0 {
			vectorScopeRejected++
			continue
		}
		accepted, protected := acceptProtectedCoverage(item)
		if !accepted {
			continue
		}
		if !protected && actualMemorySelectedCount >= candidateLimit {
			candidateLimitRejected++
			continue
		}
		out.VectorRelevant = append(out.VectorRelevant, item)
		if protected {
			protectedSelectedCount++
		} else {
			actualMemorySelectedCount++
			actualMemoryVectorSelectedCount++
		}
		key := prepareTurnMemoryLaneKey(item)
		if score := vectorHydration.Scores[key]; score > 0 {
			out.VectorScores[key] = score
		}
	}
	out.Trace["vector_recall"] = vectorHydration.Trace
	out.Trace["vector_recall_ready"] = vectorRecallReady
	out.Trace["vector_recall_attempted"] = vectorRecallAttempted
	out.Trace["vector_scope_rejected_count"] = vectorScopeRejected
	vectorActualReady := vectorRecallReady && actualMemoryVectorSelectedCount > 0
	out.Trace["lexical_fill_enabled"] = actualMemorySelectedCount < candidateLimit
	coveredDirectEntities := map[string]bool{}
	finalizeActualMemoryRefillTrace := func() {
		refillSelected := actualMemoryRelevantRefillSelectedCount + actualMemoryDeepRefillSelectedCount + actualMemoryRecentRefillSelectedCount
		directCoverageGap := maxInt(len(directlyReferencedEntities)-len(coveredDirectEntities), 0)
		out.Trace["actual_memory_candidate_safety_limit"] = candidateLimit
		out.Trace["actual_memory_target_kind"] = "no_fill_target_final_category_budget_owns_delivery"
		out.Trace["actual_memory_vector_selected"] = actualMemoryVectorSelectedCount
		out.Trace["actual_memory_relevant_refill_selected"] = actualMemoryRelevantRefillSelectedCount
		out.Trace["actual_memory_deep_refill_selected"] = actualMemoryDeepRefillSelectedCount
		out.Trace["actual_memory_recent_refill_selected"] = actualMemoryRecentRefillSelectedCount
		out.Trace["actual_memory_refill_selected"] = refillSelected
		out.Trace["actual_memory_refill_gap"] = directCoverageGap
		out.Trace["protected_candidates_consume_actual_memory_target"] = false
		out.Trace["actual_memory_refill_policy"] = "vector_actual_then_evidence_linked_mariadb; no_target_fill; unused_budget_remains_empty"
		out.Trace["vector_candidate_limit"] = vectorCandidateLimit
		out.Trace["vector_candidate_policy"] = "materialize_returned_vector_hits_then_mariadb_canonical_refill"
		out.Trace["actual_memory_refill_gap_stage"] = "selector_candidate_coverage_before_render_and_final_budget"
		out.Trace["candidate_safety_limit_reached"] = candidateLimitRejected > 0
		out.Trace["candidate_safety_truncated"] = candidateLimitRejected > 0
		out.Trace["candidate_safety_rejected_count"] = candidateLimitRejected
	}
	type scoredMemory struct {
		item       store.Memory
		key        string
		relevance  float64
		evidence   prepareTurnRecallEvidence
		importance float64
		recency    float64
	}
	scored := []scoredMemory{}
	relevantCandidates := 0
	exactPhraseCandidates := 0
	lexicalCandidates := 0
	lexicalRejectedCandidates := 0
	for _, item := range clean {
		key := prepareTurnMemoryLaneKey(item)
		relevance := 0.0
		evidence := prepareTurnRecallEvidence{}
		if queryPresent {
			evidence = prepareTurnMemoryRecallEvidence(query, item)
			if evidence.Eligible {
				relevance = simpleTokenSimilarity(query, prepareTurnMemoryRelevanceText(item))
				relevantCandidates++
			} else {
				lexicalRejectedCandidates++
			}
			if evidence.ExactPhrase {
				exactPhraseCandidates++
			} else if evidence.LexicalOverlap {
				lexicalCandidates++
			}
		}
		recency := 0.0
		if item.TurnIndex > 0 {
			if maxTurn > minTurn {
				recency = float64(item.TurnIndex-minTurn) / float64(maxTurn-minTurn)
			} else {
				recency = 1
			}
		}
		scored = append(scored, scoredMemory{
			item:       item,
			key:        key,
			relevance:  relevance,
			evidence:   evidence,
			importance: item.Importance,
			recency:    recency,
		})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		ia := scored[i]
		ja := scored[j]
		if queryPresent {
			if ia.evidence.ExactPhrase != ja.evidence.ExactPhrase {
				return ia.evidence.ExactPhrase
			}
			ir := ia.relevance > 0
			jr := ja.relevance > 0
			if ir != jr {
				return ir
			}
			if ia.relevance != ja.relevance {
				return ia.relevance > ja.relevance
			}
			if !ir && !jr {
				if ia.recency != ja.recency {
					return ia.recency > ja.recency
				}
				if ia.item.TurnIndex != ja.item.TurnIndex {
					return ia.item.TurnIndex > ja.item.TurnIndex
				}
			}
		}
		if ia.importance != ja.importance {
			return ia.importance > ja.importance
		}
		if ia.recency != ja.recency {
			return ia.recency > ja.recency
		}
		if ia.item.TurnIndex != ja.item.TurnIndex {
			return ia.item.TurnIndex > ja.item.TurnIndex
		}
		return ia.item.ID > ja.item.ID
	})

	selectCandidate := func(candidate scoredMemory) bool {
		if prepareTurnMemoryAlreadySelected(out, candidate.item) {
			return false
		}
		protectedCandidate := prepareTurnProtectedMemoryGuard(candidate.item).Active
		if queryPresent && !candidate.evidence.Eligible && !protectedCandidate {
			return false
		}
		accepted, protected := acceptProtectedCoverage(candidate.item)
		if !accepted {
			return false
		}
		if !protected && actualMemorySelectedCount >= candidateLimit {
			candidateLimitRejected++
			return false
		}
		if protected {
			protectedSelectedCount++
		} else {
			actualMemorySelectedCount++
		}
		if protected && queryPresent && !candidate.evidence.Eligible {
			out.Relevant = append(out.Relevant, candidate.item)
			return true
		}
		if candidate.evidence.Eligible {
			out.Relevant = append(out.Relevant, candidate.item)
			out.RelevantScores[candidate.key] = candidate.relevance
			if candidate.evidence.ExactPhrase {
				exactPhraseSelectedCount++
			} else if candidate.evidence.LexicalOverlap {
				lexicalSelectedCount++
			}
			if !protected {
				actualMemoryRelevantRefillSelectedCount++
			}
			return true
		}
		if !queryPresent && avgImportance > 0 && candidate.importance >= avgImportance {
			out.Deep = append(out.Deep, candidate.item)
			if !protected {
				actualMemoryDeepRefillSelectedCount++
			}
			return true
		}
		if !queryPresent {
			out.Recent = append(out.Recent, candidate.item)
			if !protected {
				actualMemoryRecentRefillSelectedCount++
			}
			return true
		}
		return false
	}
	markDirectCoverage := func(item store.Memory) {
		for _, entity := range prepareTurnMemoryDirectEntityMatches(item, directlyReferencedEntities) {
			coveredDirectEntities[normalizePrepareTurnEntityNeedle(entity)] = true
		}
	}
	currentPairEntities := append(append([]string{}, directlyReferencedEntities...), storedSceneEntities...)
	coveredDirectPairEntities := map[string]bool{}
	for _, item := range out.VectorRelevant {
		if !prepareTurnProtectedMemoryGuard(item).Active {
			markDirectCoverage(item)
			matches := prepareTurnMemoryDirectEntityMatches(item, currentPairEntities)
			if len(matches) >= 2 {
				for _, entity := range directlyReferencedEntities {
					if prepareTurnRelationshipNameInList(entity, matches) {
						coveredDirectPairEntities[normalizePrepareTurnEntityNeedle(entity)] = true
					}
				}
			}
		}
	}
	if queryPresent {
		for _, entity := range directlyReferencedEntities {
			entityKey := normalizePrepareTurnEntityNeedle(entity)
			if entityKey == "" || coveredDirectPairEntities[entityKey] {
				continue
			}
			pairSelected := false
			for _, candidate := range scored {
				matches := prepareTurnMemoryDirectEntityMatches(candidate.item, currentPairEntities)
				if len(matches) < 2 ||
					!prepareTurnRelationshipNameInList(entity, matches) ||
					prepareTurnProtectedMemoryGuard(candidate.item).Active {
					continue
				}
				candidate.evidence.Eligible = true
				if prepareTurnMemoryAlreadySelected(out, candidate.item) || selectCandidate(candidate) {
					markDirectCoverage(candidate.item)
					pairSelected = true
					break
				}
			}
			if pairSelected || coveredDirectEntities[entityKey] {
				continue
			}
			for _, candidate := range scored {
				if !candidate.evidence.Eligible || prepareTurnProtectedMemoryGuard(candidate.item).Active {
					continue
				}
				matches := prepareTurnMemoryDirectEntityMatches(candidate.item, directlyReferencedEntities)
				if !prepareTurnRelationshipNameInList(entity, matches) {
					continue
				}
				if selectCandidate(candidate) {
					markDirectCoverage(candidate.item)
					break
				}
			}
		}
	}
	for _, candidate := range scored {
		if selectCandidate(candidate) {
			markDirectCoverage(candidate.item)
		}
	}
	out.Trace["general_lexical_refill_skipped_after_vector_success"] = false
	out.Trace["general_lexical_evaluated_with_vector_success"] = vectorActualReady
	out.Trace["vector_selected"] = len(out.VectorRelevant)
	out.Trace["recent_selected"] = len(out.Recent)
	out.Trace["relevant_selected"] = len(out.Relevant)
	out.Trace["deep_selected"] = len(out.Deep)
	out.Trace["selected_total"] = prepareTurnSelectedMemoryCount(out)
	out.Trace["relevant_candidates"] = relevantCandidates
	out.Trace["exact_phrase_candidate_count"] = exactPhraseCandidates
	out.Trace["exact_phrase_selected_count"] = exactPhraseSelectedCount
	out.Trace["lexical_candidate_count"] = lexicalCandidates
	out.Trace["lexical_selected_count"] = lexicalSelectedCount
	out.Trace["lexical_rejected_candidate_count"] = lexicalRejectedCandidates
	out.Trace["query_present_recent_fill_disabled"] = queryPresent
	out.Trace["average_importance"] = avgImportance
	out.Trace["relevant_degraded_reason"] = nilIfEmpty(relevantDegradedReason(query, len(out.Relevant), relevantCandidates))
	out.Trace["protected_duplicate_candidate_count"] = 0
	out.Trace["protected_duplicate_candidates"] = []map[string]any{}
	out.Trace["protected_coverage_key_count"] = 0
	out.Trace["protected_selector_dedupe_policy"] = "defer_exact_source_scope_grouping_to_final_render"
	out.Trace["actual_memory_selected"] = actualMemorySelectedCount
	out.Trace["protected_guard_selected"] = protectedSelectedCount
	out.Trace["protected_guard_candidate_safety_limit"] = candidateLimit
	out.Trace["direct_entity_requested_count"] = len(directlyReferencedEntities)
	out.Trace["direct_entity_outside_stored_scene_count"] = len(directEntitiesOutsideStoredScene)
	out.Trace["direct_entity_memory_covered_count"] = len(coveredDirectEntities)
	finalizeActualMemoryRefillTrace()
	return out
}

func prepareTurnProtectedAliasResolution(memories []store.Memory) (map[string]string, map[string]bool) {
	candidates := map[string]map[string]bool{}
	add := func(alias, canonical string) {
		aliasKey := normalizeCharacterKey(alias)
		canonicalKey := normalizeCharacterKey(canonical)
		if aliasKey == "" || canonicalKey == "" {
			return
		}
		if candidates[aliasKey] == nil {
			candidates[aliasKey] = map[string]bool{}
		}
		candidates[aliasKey][canonicalKey] = true
	}
	for _, item := range memories {
		parsed := parseJSONMap(item.SummaryJSON)
		for _, raw := range sliceFromAny(parsed["character_identity_accuracy"]) {
			identity := mapFromAny(raw)
			canonical := extractionFirstNonEmpty(
				stringFromMap(identity, "canonical_entity_name"),
				stringFromMap(identity, "true_identity_name"),
				stringFromMap(identity, "real_identity_name"),
			)
			if canonical == "" {
				continue
			}
			add(canonical, canonical)
			if !boolFromAny(identity["same_entity"]) {
				continue
			}
			for _, key := range []string{"surface_identity_name", "public_identity_name", "alias_name"} {
				add(stringFromMap(identity, key), canonical)
			}
			for _, alias := range stringsFromAny(identity["aliases"]) {
				add(alias, canonical)
			}
		}
	}
	out := map[string]string{}
	ambiguous := map[string]bool{}
	for alias, canonicalSet := range candidates {
		if len(canonicalSet) != 1 {
			ambiguous[alias] = true
			continue
		}
		for canonical := range canonicalSet {
			out[alias] = canonical
		}
	}
	return out, ambiguous
}

func prepareTurnProtectedAliasCanonicalMap(memories []store.Memory) map[string]string {
	canonical, _ := prepareTurnProtectedAliasResolution(memories)
	return canonical
}

func prepareTurnProtectedMemoryCoverageKeys(item store.Memory, aliasCanonical map[string]string, ambiguousAliases ...map[string]bool) []string {
	parsed := parseJSONMap(item.SummaryJSON)
	keys := []string{}
	ambiguous := map[string]bool{}
	if len(ambiguousAliases) > 0 && ambiguousAliases[0] != nil {
		ambiguous = ambiguousAliases[0]
	}
	canonicalPerson := func(value string) string {
		key := normalizeCharacterKey(value)
		if ambiguous[key] {
			return fmt.Sprintf("ambiguous:%s:%v", key, prepareTurnMemorySourceRowID(item))
		}
		if canonical := aliasCanonical[key]; canonical != "" {
			return canonical
		}
		return key
	}
	add := func(kind, person, category string) {
		kind = normalizeProtectedSecretToken(kind)
		if kind == "" {
			kind = category
		}
		person = canonicalPerson(person)
		if person == "" {
			return
		}
		key := category + "|" + person + "|" + kind
		if !stringSliceContains(keys, key) {
			keys = append(keys, key)
		}
	}
	for _, raw := range sliceFromAny(parsed["protected_secrets"]) {
		secret := mapFromAny(raw)
		kind := stringFromMap(secret, "secret_kind")
		owner := stringFromMap(secret, "owner")
		if strings.TrimSpace(owner) != "" {
			add(kind, owner, "secret")
			continue
		}
		subjects := stringsFromAny(secret["subject"])
		if len(subjects) == 0 {
			unscopedKind := normalizeProtectedSecretToken(kind)
			if unscopedKind == "" {
				unscopedKind = "secret"
			}
			key := "secret|unscoped|" + unscopedKind
			if !stringSliceContains(keys, key) {
				keys = append(keys, key)
			}
			continue
		}
		subjectKeys := []string{}
		for _, subject := range subjects {
			if key := canonicalPerson(subject); key != "" && !stringSliceContains(subjectKeys, key) {
				subjectKeys = append(subjectKeys, key)
			}
		}
		sort.Strings(subjectKeys)
		if len(subjectKeys) > 0 {
			add(kind, "subjects:"+strings.Join(subjectKeys, "+"), "secret")
		}
	}
	for _, raw := range sliceFromAny(parsed["character_identity_accuracy"]) {
		identity := mapFromAny(raw)
		person := extractionFirstNonEmpty(
			stringFromMap(identity, "canonical_entity_name"),
			stringFromMap(identity, "true_identity_name"),
			stringFromMap(identity, "real_identity_name"),
			stringFromMap(identity, "surface_identity_name"),
			stringFromMap(identity, "public_identity_name"),
			stringFromMap(identity, "alias_name"),
		)
		add(stringFromMap(identity, "identity_kind"), person, "identity")
	}
	return keys
}

func prepareTurnProtectedPerspectiveContext(perspectiveContext map[string]any, memories []store.Memory, charStates []store.CharacterState) map[string]any {
	perspectiveContext = normalizePrepareTurnPerspectiveContext(perspectiveContext)
	povKey := normalizeCharacterKey(extractionStringFromAny(perspectiveContext["current_pov"]))
	if povKey == "" {
		return nil
	}
	known := map[string]bool{}
	add := func(value string) {
		if key := normalizeCharacterKey(value); key != "" {
			known[key] = true
		}
	}
	for _, state := range charStates {
		add(state.CharacterName)
	}
	for _, item := range memories {
		parsed := parseJSONMap(item.SummaryJSON)
		for _, key := range []string{"characters", "character_names", "people"} {
			for _, value := range memorySearchStringValues(parsed[key]) {
				add(value)
			}
		}
		for _, entity := range memorySearchMapItems(parsed["entities"]) {
			for _, key := range []string{"name", "canonical_name", "display_name"} {
				add(stringFromMap(entity, key))
			}
			for _, alias := range memorySearchStringValues(entity["aliases"]) {
				add(alias)
			}
		}
		for _, secret := range memorySearchMapItems(parsed["protected_secrets"]) {
			add(stringFromMap(secret, "owner"))
			for _, subject := range memorySearchStringValues(secret["subject"]) {
				add(subject)
			}
		}
		for _, identity := range memorySearchMapItems(parsed["character_identity_accuracy"]) {
			for _, key := range []string{"canonical_entity_name", "surface_identity_name", "true_identity_name", "public_identity_name", "alias_name", "real_identity_name"} {
				add(stringFromMap(identity, key))
			}
			for _, alias := range memorySearchStringValues(identity["aliases"]) {
				add(alias)
			}
		}
	}
	if !known[povKey] {
		return nil
	}
	return perspectiveContext
}

func prepareTurnVectorRecallReady(trace map[string]any) bool {
	if trace == nil {
		return false
	}
	return strings.TrimSpace(stringFromMap(trace, "status")) == "ready" && intFromAny(trace["selected_count"], 0) > 0
}

func prepareTurnVectorSearchAttempted(vectorShadow map[string]any) bool {
	if vectorShadow == nil {
		return false
	}
	return boolFromAny(vectorShadow["memory_search_attempted"])
}

func prepareTurnSelectedMemoryCount(selection prepareTurnMemoryLaneSelection) int {
	return len(selection.VectorRelevant) + len(selection.Recent) + len(selection.Relevant) + len(selection.Deep) + len(selection.ProtectedSelected)
}

func prepareTurnRecallMemoryCount(selection prepareTurnMemoryLaneSelection) int {
	count := 0
	for _, lane := range [][]store.Memory{selection.VectorRelevant, selection.Relevant, selection.Deep, selection.Recent} {
		for _, item := range lane {
			if !prepareTurnProtectedMemoryGuard(item).Active {
				count++
			}
		}
	}
	return count
}

func prepareTurnMemoryLaneCounters(selection prepareTurnMemoryLaneSelection, injected bool) map[string]any {
	vectorTrace := mapFromAny(selection.Trace["vector_recall"])
	injectedCount := 0
	if injected {
		injectedCount = len(selection.VectorRelevant)
	}
	return map[string]any{
		"memory_lane_order":                             []string{"vector_relevant", "relevant", "deep", "recent"},
		"vector_memory_hit_count":                       intFromAny(vectorTrace["memory_hit_count"], maxInt(intFromAny(vectorTrace["input_hit_count"], 0)-intFromAny(vectorTrace["non_memory_count"], 0), 0)),
		"vector_memory_hydrated_count":                  intFromAny(vectorTrace["hydrated_count"], 0),
		"vector_memory_selected_count":                  len(selection.VectorRelevant),
		"vector_memory_injected_count":                  injectedCount,
		"vector_memory_duplicate_count":                 intFromAny(vectorTrace["duplicate_count"], 0),
		"vector_memory_missing_count":                   intFromAny(vectorTrace["missing_count"], 0),
		"vector_non_memory_hit_count":                   intFromAny(vectorTrace["non_memory_count"], 0),
		"vector_memory_hit_language_context_count":      intFromAny(vectorTrace["hit_language_context_count"], 0),
		"vector_memory_hit_alias_indexed_count":         intFromAny(vectorTrace["hit_alias_indexed_count"], 0),
		"vector_memory_hydrated_language_context_count": intFromAny(vectorTrace["hydrated_language_context_count"], 0),
		"vector_memory_hydrated_alias_ready_count":      intFromAny(vectorTrace["hydrated_alias_ready_count"], 0),
		"vector_memory_search_text_policy":              stringFromMap(vectorTrace, "search_text_policy"),
		"vector_memory_recall_status":                   stringFromMap(vectorTrace, "status"),
		"vector_memory_recall_reason":                   stringFromMap(vectorTrace, "reason"),
		"vector_relevant_memory_count":                  len(selection.VectorRelevant),
		"relevant_memory_count":                         len(selection.Relevant),
		"deep_memory_count":                             len(selection.Deep),
		"recent_memory_count":                           len(selection.Recent),
		"protected_memory_selected_count":               len(selection.ProtectedSelected),
		"protected_memory_dropped_count":                intFromAny(selection.Trace["protected_memory_dropped_count"], 0),
		"protected_memory_gate":                         stringFromMap(selection.Trace, "protected_memory_gate"),
		"selected_memory_total_count":                   prepareTurnSelectedMemoryCount(selection),
		"actual_memory_selected_count":                  prepareTurnRecallMemoryCount(selection),
		"protected_guard_selected_count":                intFromAny(selection.Trace["protected_guard_selected"], 0),
		"protected_guard_budget":                        intFromAny(selection.Trace["protected_guard_budget"], 0),
		"selected_memory_total_target":                  intFromAny(selection.Trace["top_k_memory_target"], 0),
		"selected_memory_top_k_contract":                stringFromMap(selection.Trace, "top_k_definition"),
	}
}

func mergePrepareTurnMemoryLaneCounters(counts map[string]any, selection prepareTurnMemoryLaneSelection, injected bool) {
	if counts == nil {
		return
	}
	for key, value := range prepareTurnMemoryLaneCounters(selection, injected) {
		counts[key] = value
	}
}

func prepareTurnMemorySourceOccurrenceKey(item store.Memory) string {
	sessionID := strings.TrimSpace(item.ChatSessionID)
	if sessionID == "" || item.TurnIndex == 0 {
		return ""
	}
	coordinates := map[string]any{}
	coordinateConflict := false
	setCoordinate := func(key string, value any) {
		if key == "source_content_hash" {
			key = "content_hash"
		} else if key == "source_turn_index" {
			key = "source_turn_start"
		}
		if existing, ok := coordinates[key]; ok {
			if mustCompactJSON(existing) != mustCompactJSON(value) {
				coordinateConflict = true
			}
			return
		}
		coordinates[key] = value
	}
	for _, payload := range []map[string]any{parseJSONMap(item.SummaryJSON), parseJSONMap(item.Evidence)} {
		for _, key := range []string{
			"source_revision", "precise_memory_unit_id", "source_occurrence_id",
			"logical_turn_id", "generation_id", "source_turn_start", "source_turn_end",
			"source_turn_index", "source_message_id", "source_message_ids", "content_hash", "source_content_hash",
		} {
			if value, ok := payload[key]; ok && strings.TrimSpace(fmt.Sprint(value)) != "" {
				setCoordinate(key, value)
			}
		}
		if source := mapFromAny(payload["source"]); len(source) > 0 {
			for _, key := range []string{
				"source_revision", "precise_memory_unit_id", "source_occurrence_id",
				"logical_turn_id", "generation_id", "source_turn_start", "source_turn_end",
				"source_turn_index", "source_message_id", "source_message_ids", "content_hash", "source_content_hash",
			} {
				if value, ok := source[key]; ok && strings.TrimSpace(fmt.Sprint(value)) != "" {
					setCoordinate(key, value)
				}
			}
		}
	}
	revision := strings.TrimSpace(extractionStringFromAny(coordinates["source_revision"]))
	unitID := strings.TrimSpace(extractionStringFromAny(coordinates["precise_memory_unit_id"]))
	occurrenceID := strings.TrimSpace(extractionStringFromAny(coordinates["source_occurrence_id"]))
	contentHash := strings.TrimSpace(extractionFirstNonEmpty(
		extractionStringFromAny(coordinates["content_hash"]),
		extractionStringFromAny(coordinates["source_content_hash"]),
	))
	fromTurn := intFromAny(coordinates["source_turn_start"], 0)
	toTurn := intFromAny(coordinates["source_turn_end"], fromTurn)
	if !coordinateConflict && revision != "" && (unitID != "" || occurrenceID != "") && contentHash != "" && fromTurn > 0 && toTurn >= fromTurn {
		coordinates["source_turn_start"] = fromTurn
		coordinates["source_turn_end"] = toTurn
		return strings.Join([]string{sessionID, mustCompactJSON(coordinates), strconv.Itoa(fromTurn), strconv.Itoa(toTurn)}, "\x1f")
	}
	if item.ID > 0 {
		return fmt.Sprintf("memory-row:%s:%d:%d", sessionID, item.TurnIndex, item.ID)
	}
	return ""
}

func filterPrepareTurnProtectedMemoryLaneSelection(selection prepareTurnMemoryLaneSelection, ctx prepareTurnRecollectionContext, perspectiveContext map[string]any) prepareTurnMemoryLaneSelection {
	before := prepareTurnSelectedMemoryCount(selection)
	dropped := []map[string]any{}
	selectedKeys := map[string]bool{}
	for _, lane := range [][]store.Memory{selection.VectorRelevant, selection.Relevant, selection.Deep, selection.Recent} {
		for _, item := range lane {
			selectedKeys[prepareTurnMemoryLaneKey(item)] = true
		}
	}
	for _, item := range selection.ProtectedSelected {
		selectedKeys[prepareTurnMemoryLaneKey(item)] = true
	}
	filterLane := func(lane string, items []store.Memory) []store.Memory {
		out := make([]store.Memory, 0, len(items))
		for _, item := range items {
			ok, reason := prepareTurnProtectedMemoryRelevant(item, ctx, perspectiveContext)
			if ok {
				out = append(out, item)
				continue
			}
			dropped = append(dropped, map[string]any{
				"lane":       lane,
				"id":         item.ID,
				"turn_index": item.TurnIndex,
				"reason":     reason,
			})
		}
		return out
	}
	selection.VectorRelevant = filterLane("vector_relevant", selection.VectorRelevant)
	selection.Relevant = filterLane("relevant", selection.Relevant)
	selection.Deep = filterLane("deep", selection.Deep)
	selection.Recent = filterLane("recent", selection.Recent)
	selection.ProtectedSelected = filterLane("protected", selection.ProtectedSelected)
	for _, item := range selection.ProtectedCandidates {
		if selectedKeys[prepareTurnMemoryLaneKey(item)] {
			continue
		}
		if ok, reason := prepareTurnProtectedMemoryRelevant(item, ctx, perspectiveContext); !ok {
			dropped = append(dropped, map[string]any{
				"lane":       "preselection",
				"id":         item.ID,
				"turn_index": item.TurnIndex,
				"reason":     reason,
			})
		}
	}
	if selection.Trace == nil {
		selection.Trace = map[string]any{}
	}
	selection.Trace["protected_memory_before_filter"] = before
	selection.Trace["protected_memory_after_filter"] = prepareTurnSelectedMemoryCount(selection)
	selection.Trace["protected_memory_dropped_count"] = len(dropped)
	selection.Trace["protected_memory_gate"] = "protected_owner_subject_knowledge_scope_or_current_pov_must_match_current_input_stored_active_scene_relevant_previous_event_open_goal_or_pov"
	selection.Trace["protected_memory_dropped"] = dropped
	return selection
}

// projectPrepareTurnGeneralMemories produces the public-only copy consumed by
// ordinary lexical/vector selection and general memory delivery. Typed and
// protected guidance remains on its separate canonical reader path.
func projectPrepareTurnGeneralMemories(items []store.Memory) ([]store.Memory, map[string]any) {
	out := make([]store.Memory, 0, len(items))
	for _, item := range items {
		if projected, ok := publicMemoryFromCanonical(item); ok {
			out = append(out, projected)
		}
	}
	return out, map[string]any{
		"public_memory_projection_input_count":   len(items),
		"public_memory_projection_output_count":  len(out),
		"public_memory_projection_dropped_count": len(items) - len(out),
	}
}

func prepareTurnProtectedMemoryRelevant(item store.Memory, ctx prepareTurnRecollectionContext, perspectiveContext map[string]any) (bool, string) {
	tokens, protected := prepareTurnProtectedMemoryEntityTokens(item)
	if !protected {
		return true, "not_protected_memory"
	}
	if len(tokens) == 0 {
		return true, "protected_memory_without_entity_scope"
	}
	if guard := prepareTurnProtectedMemoryGuard(item, perspectiveContext); guard.Active && guard.POVScoped {
		return true, "current_pov_scoped_identity_guard"
	}
	if prepareTurnAnyOwnerTokenMatches(tokens, ctx.rawUserInput) {
		return true, "explicit_current_user_input"
	}
	if prepareTurnAnyOwnerTokenMatches(tokens, ctx.currentEntities) {
		return true, "stored_active_scene_entity"
	}
	if prepareTurnAnyOwnerTokenMatches(tokens, ctx.previousEventGuardSummary) {
		return true, "previous_final_event_guard"
	}
	if prepareTurnAnyOwnerTokenMatches(tokens, ctx.unresolvedGoals) {
		return true, "relevant_open_goal_guard"
	}
	if pov := strings.TrimSpace(extractionStringFromAny(perspectiveContext["current_pov"])); pov != "" && prepareTurnAnyOwnerTokenMatches(tokens, pov) {
		return true, "current_pov_match"
	}
	return false, "protected_entity_not_in_current_input_stored_active_scene_relevant_previous_event_open_goal_or_pov"
}

func prepareTurnProtectedMemoryEntityTokens(item store.Memory) ([]string, bool) {
	parsed := parseJSONMap(item.SummaryJSON)
	protectedSecrets := sliceFromAny(parsed["protected_secrets"])
	identityAccuracy := sliceFromAny(parsed["character_identity_accuracy"])
	if len(protectedSecrets) == 0 && len(identityAccuracy) == 0 {
		return nil, false
	}
	tokens := []string{}
	add := func(value string) {
		for _, token := range prepareTurnOwnerTokens(value, value) {
			if token != "" && !stringSliceContains(tokens, token) {
				tokens = append(tokens, token)
			}
		}
	}
	addValues := func(values []string) {
		for _, value := range values {
			add(value)
		}
	}
	for _, raw := range protectedSecrets {
		secret := mapFromAny(raw)
		add(stringFromMap(secret, "owner"))
		addValues(stringsFromAny(secret["subject"]))
		scope := mapFromAny(secret["knowledge_scope"])
		addValues(stringsFromAny(scope["known_by"]))
		addValues(stringsFromAny(scope["suspected_by"]))
		addValues(stringsFromAny(scope["unknown_to"]))
	}
	for _, raw := range identityAccuracy {
		identity := mapFromAny(raw)
		for _, key := range []string{
			"canonical_entity_name",
			"surface_identity_name",
			"true_identity_name",
			"public_identity_name",
			"alias_name",
			"real_identity_name",
		} {
			add(stringFromMap(identity, key))
		}
		scope := mapFromAny(identity["knowledge_scope"])
		addValues(stringsFromAny(scope["known_by"]))
		addValues(stringsFromAny(scope["suspected_by"]))
		addValues(stringsFromAny(scope["unknown_to"]))
	}
	return tokens, true
}

func collapsePrepareTurnStorylines(items []store.Storyline) []store.Storyline {
	out := make([]store.Storyline, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		key := collapseTextKey(extractionFirstNonEmpty(item.Name, item.CurrentContext))
		detailKey := collapseTextKey(item.CurrentContext)
		if key == "" {
			key = detailKey
		}
		if key != "" && seen[key] {
			continue
		}
		if detailKey != "" && seen[detailKey] {
			continue
		}
		if key != "" {
			seen[key] = true
		}
		if detailKey != "" {
			seen[detailKey] = true
		}
		out = append(out, item)
	}
	return out
}

func mergePrepareTurnWorldRulesForInjection(priority, rest []store.WorldRule) []store.WorldRule {
	out := make([]store.WorldRule, 0, len(priority)+len(rest))
	out = append(out, priority...)
	out = append(out, rest...)
	return out
}

func collapsePrepareTurnWorldRules(items []store.WorldRule) []store.WorldRule {
	out := make([]store.WorldRule, 0, len(items))
	groupIndex := map[string]int{}
	for _, item := range items {
		if item.Suppressed {
			continue
		}
		keyParts := []string{
			strings.TrimSpace(item.ChatSessionID),
			collapseTextKey(item.Scope),
			collapseTextKey(item.ScopeName),
			collapseTextKey(item.Category),
			collapseTextKey(item.Key),
			strings.TrimSpace(item.ValueJSON),
		}
		key := strings.Join(keyParts, "\x1f")
		if strings.Trim(strings.Join(keyParts[1:], ""), " ") == "" {
			if item.ID <= 0 {
				out = append(out, item)
				continue
			}
			key = fmt.Sprintf("row:%d", item.ID)
		}
		if index, ok := groupIndex[key]; ok {
			current := out[index]
			preferCandidate := item.UserCorrected != current.UserCorrected && item.UserCorrected
			if item.UserCorrected == current.UserCorrected && item.Pinned != current.Pinned {
				preferCandidate = item.Pinned
			}
			if item.UserCorrected == current.UserCorrected && item.Pinned == current.Pinned {
				switch {
				case item.UpdatedAt.After(current.UpdatedAt):
					preferCandidate = true
				case item.UpdatedAt.Equal(current.UpdatedAt) && item.SourceTurn > current.SourceTurn:
					preferCandidate = true
				case item.UpdatedAt.Equal(current.UpdatedAt) && item.SourceTurn == current.SourceTurn && len([]rune(item.ValueJSON)) > len([]rune(current.ValueJSON)):
					preferCandidate = true
				}
			}
			if preferCandidate {
				out[index] = item
			}
			continue
		}
		groupIndex[key] = len(out)
		out = append(out, item)
	}
	return out
}

func collapseTextKey(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return ""
	}
	return strings.Join(strings.Fields(text), " ")
}

func prepareTurnMemoryLaneProtectedCounts(selection prepareTurnMemoryLaneSelection, perspectiveContext map[string]any) map[string]any {
	counts := map[string]any{
		"protected_secret_count":          0,
		"identity_accuracy_count":         0,
		"protected_memory_guarded_count":  0,
		"pov_scoped_identity_guard_count": 0,
		"protected_memory_selected_count": 0,
	}
	seen := map[string]bool{}
	add := func(item store.Memory) {
		key := prepareTurnMemoryLaneKey(item)
		if key == "" {
			key = fmt.Sprintf("turn:%d:%s", item.TurnIndex, item.SummaryJSON)
		}
		if seen[key] {
			return
		}
		seen[key] = true
		parsed := parseJSONMap(item.SummaryJSON)
		protectedSecrets := sliceFromAny(parsed["protected_secrets"])
		identityAccuracy := sliceFromAny(parsed["character_identity_accuracy"])
		counts["protected_secret_count"] = intFromAny(counts["protected_secret_count"], 0) + len(protectedSecrets)
		counts["identity_accuracy_count"] = intFromAny(counts["identity_accuracy_count"], 0) + len(identityAccuracy)
		if len(protectedSecrets) > 0 || len(identityAccuracy) > 0 {
			counts["protected_memory_selected_count"] = intFromAny(counts["protected_memory_selected_count"], 0) + 1
		}
		if guard := prepareTurnProtectedMemoryGuard(item, perspectiveContext); guard.Active {
			counts["protected_memory_guarded_count"] = intFromAny(counts["protected_memory_guarded_count"], 0) + 1
			if guard.POVScoped {
				counts["pov_scoped_identity_guard_count"] = intFromAny(counts["pov_scoped_identity_guard_count"], 0) + 1
			}
		}
	}
	for _, item := range selection.VectorRelevant {
		add(item)
	}
	for _, item := range selection.Relevant {
		add(item)
	}
	for _, item := range selection.Deep {
		add(item)
	}
	for _, item := range selection.Recent {
		add(item)
	}
	for _, item := range selection.ProtectedSelected {
		add(item)
	}
	return counts
}

type prepareTurnVectorMemoryHydration struct {
	Items  []store.Memory
	Scores map[string]float64
	Trace  map[string]any
}

type prepareTurnVectorArtifactHydration struct {
	Evidence   []store.DirectEvidence
	WorldRules []store.WorldRule
	Trace      map[string]any
}

const (
	prepareTurnMinCosineSimilarity          = 0.30
	prepareTurnMinInverseDistanceSimilarity = 0.55
)

func prepareTurnHydrateVectorMemoryHits(memories []store.Memory, vectorShadow map[string]any, limit int) prepareTurnVectorMemoryHydration {
	out := prepareTurnVectorMemoryHydration{
		Items:  []store.Memory{},
		Scores: map[string]float64{},
		Trace: map[string]any{
			"version":                         "vdb1.hydrate_memory_hits.v1",
			"status":                          "not_attempted",
			"truth_boundary":                  "vector_hit_is_selector_only_mariadb_memory_is_canonical",
			"input_hit_count":                 0,
			"memory_hit_count":                0,
			"hydrated_count":                  0,
			"duplicate_count":                 0,
			"missing_count":                   0,
			"scope_filtered_count":            0,
			"non_memory_count":                0,
			"score_missing_count":             0,
			"below_similarity_count":          0,
			"search_text_policy":              languageMemorySearchPolicy,
			"hit_language_context_count":      0,
			"hit_alias_indexed_count":         0,
			"hydrated_language_context_count": 0,
			"hydrated_alias_ready_count":      0,
		},
	}
	limit = prepareTurnRecallLimit(limit)
	if vectorShadow == nil {
		out.Trace["reason"] = "vector_shadow_missing"
		return out
	}
	if strings.TrimSpace(stringFromMap(vectorShadow, "memory_search_result")) != "ok" {
		out.Trace["status"] = "skipped"
		out.Trace["reason"] = strings.TrimSpace(stringFromMap(vectorShadow, "memory_search_result"))
		if out.Trace["reason"] == "" {
			out.Trace["reason"] = strings.TrimSpace(stringFromMap(vectorShadow, "search_skipped_reason"))
		}
		return out
	}
	memoryByID := map[int64]store.Memory{}
	for _, item := range memories {
		if item.ID > 0 {
			memoryByID[item.ID] = item
		}
	}
	seen := map[int64]bool{}
	hits := prepareTurnVectorMemorySearchResultMaps(vectorShadow)
	out.Trace["status"] = "ready"
	out.Trace["input_hit_count"] = len(hits)
	hitRawLanguageCounts := map[string]int{}
	hitSummaryLanguageCounts := map[string]int{}
	hitSessionLanguageCounts := map[string]int{}
	for _, hit := range hits {
		if prepareTurnVectorHitHasLanguageMetadata(hit) {
			out.Trace["hit_language_context_count"] = intFromAny(out.Trace["hit_language_context_count"], 0) + 1
		}
		if intFromAny(hit["alias_count"], 0) > 0 {
			out.Trace["hit_alias_indexed_count"] = intFromAny(out.Trace["hit_alias_indexed_count"], 0) + 1
		}
		incrementLanguageCount(hitRawLanguageCounts, stringFromMap(hit, "raw_language"))
		incrementLanguageCount(hitSummaryLanguageCounts, stringFromMap(hit, "summary_language"))
		incrementLanguageCount(hitSessionLanguageCounts, stringFromMap(hit, "session_output_language"))
	}
	out.Trace["hit_raw_language_counts"] = hitRawLanguageCounts
	out.Trace["hit_summary_language_counts"] = hitSummaryLanguageCounts
	out.Trace["hit_session_output_language_counts"] = hitSessionLanguageCounts
	hydratedRawLanguageCounts := map[string]int{}
	hydratedSummaryLanguageCounts := map[string]int{}
	hydratedSessionLanguageCounts := map[string]int{}
	for _, hit := range hits {
		if len(out.Items) >= limit {
			break
		}
		if !prepareTurnVectorHitLooksLikeMemory(hit) {
			out.Trace["non_memory_count"] = intFromAny(out.Trace["non_memory_count"], 0) + 1
			continue
		}
		out.Trace["memory_hit_count"] = intFromAny(out.Trace["memory_hit_count"], 0) + 1
		id := prepareTurnVectorMemoryRowID(hit)
		if id <= 0 {
			out.Trace["missing_count"] = intFromAny(out.Trace["missing_count"], 0) + 1
			continue
		}
		item, ok := memoryByID[id]
		if !ok {
			out.Trace["missing_count"] = intFromAny(out.Trace["missing_count"], 0) + 1
			continue
		}
		if seen[id] {
			out.Trace["duplicate_count"] = intFromAny(out.Trace["duplicate_count"], 0) + 1
			continue
		}
		projected, projectionOK := publicMemoryFromCanonical(item)
		if !projectionOK {
			out.Trace["scope_filtered_count"] = intFromAny(out.Trace["scope_filtered_count"], 0) + 1
			continue
		}
		item = projected
		score, scoreOK := prepareTurnVectorHitSimilarity(hit)
		if !scoreOK {
			out.Trace["score_missing_count"] = intFromAny(out.Trace["score_missing_count"], 0) + 1
			continue
		}
		if !prepareTurnVectorSimilarityEligible(score, stringFromMap(hit, "similarity_source")) {
			out.Trace["below_similarity_count"] = intFromAny(out.Trace["below_similarity_count"], 0) + 1
			continue
		}
		seen[id] = true
		out.Items = append(out.Items, item)
		languageMeta := memoryVectorLanguageMetadata(item)
		if prepareTurnMemoryHasLanguageMetadata(languageMeta) {
			out.Trace["hydrated_language_context_count"] = intFromAny(out.Trace["hydrated_language_context_count"], 0) + 1
		}
		incrementLanguageCount(hydratedRawLanguageCounts, languageMeta["raw_language"])
		incrementLanguageCount(hydratedSummaryLanguageCounts, languageMeta["summary_language"])
		incrementLanguageCount(hydratedSessionLanguageCounts, languageMeta["session_output_language"])
		if memorySearchTextFromMemory(item).AliasCount > 0 {
			out.Trace["hydrated_alias_ready_count"] = intFromAny(out.Trace["hydrated_alias_ready_count"], 0) + 1
		}
		key := prepareTurnMemoryLaneKey(item)
		out.Scores[key] = score
	}
	out.Trace["hydrated_count"] = len(out.Items)
	out.Trace["selected_count"] = len(out.Items)
	out.Trace["hydrated_raw_language_counts"] = hydratedRawLanguageCounts
	out.Trace["hydrated_summary_language_counts"] = hydratedSummaryLanguageCounts
	out.Trace["hydrated_session_output_language_counts"] = hydratedSessionLanguageCounts
	if len(out.Items) == 0 {
		out.Trace["status"] = "empty"
	}
	return out
}

func prepareTurnHydrateVectorArtifactHits(
	evidence []store.DirectEvidence,
	worldRules []store.WorldRule,
	vectorShadow map[string]any,
	limit int,
	blockedEvidenceIDsArg ...map[int64]bool,
) prepareTurnVectorArtifactHydration {
	out := prepareTurnVectorArtifactHydration{
		Evidence:   []store.DirectEvidence{},
		WorldRules: []store.WorldRule{},
		Trace: map[string]any{
			"version":                   "vdb2.hydrate_artifact_hits.v1",
			"status":                    "not_attempted",
			"truth_boundary":            "vector_hit_is_selector_only_mariadb_row_is_canonical",
			"input_hit_count":           0,
			"evidence_hit_count":        0,
			"world_rule_hit_count":      0,
			"evidence_hydrated_count":   0,
			"world_rule_hydrated_count": 0,
			"scope_filtered_count":      0,
			"missing_count":             0,
			"duplicate_count":           0,
			"score_missing_count":       0,
			"below_similarity_count":    0,
		},
	}
	limit = prepareTurnRecallLimit(limit)
	if vectorShadow == nil {
		out.Trace["reason"] = "vector_shadow_missing"
		return out
	}
	if strings.TrimSpace(stringFromMap(vectorShadow, "search_result")) != "ok" {
		out.Trace["status"] = "skipped"
		out.Trace["reason"] = strings.TrimSpace(stringFromMap(vectorShadow, "search_result"))
		if out.Trace["reason"] == "" {
			out.Trace["reason"] = strings.TrimSpace(stringFromMap(vectorShadow, "search_skipped_reason"))
		}
		return out
	}
	evidenceByID := map[int64]store.DirectEvidence{}
	for _, item := range evidence {
		if item.ID > 0 {
			evidenceByID[item.ID] = item
		}
	}
	worldRuleByID := map[int64]store.WorldRule{}
	for _, item := range worldRules {
		if item.ID > 0 {
			worldRuleByID[item.ID] = item
		}
	}
	seenEvidence := map[int64]bool{}
	seenWorldRule := map[int64]bool{}
	blockedEvidenceIDs := map[int64]bool{}
	if len(blockedEvidenceIDsArg) > 0 && blockedEvidenceIDsArg[0] != nil {
		blockedEvidenceIDs = blockedEvidenceIDsArg[0]
	}
	hits := prepareTurnVectorSearchResultMaps(vectorShadow["search_results"])
	out.Trace["status"] = "ready"
	out.Trace["input_hit_count"] = len(hits)
	for _, hit := range hits {
		if len(out.Evidence)+len(out.WorldRules) >= limit {
			break
		}
		score, scoreOK := prepareTurnVectorHitSimilarity(hit)
		if !scoreOK {
			out.Trace["score_missing_count"] = intFromAny(out.Trace["score_missing_count"], 0) + 1
			continue
		}
		if !prepareTurnVectorSimilarityEligible(score, stringFromMap(hit, "similarity_source")) {
			out.Trace["below_similarity_count"] = intFromAny(out.Trace["below_similarity_count"], 0) + 1
			continue
		}
		sourceTable := strings.ToLower(strings.TrimSpace(stringFromMap(hit, "source_table")))
		tier := strings.ToLower(strings.TrimSpace(stringFromMap(hit, "tier")))
		id := prepareTurnVectorSourceRowID(hit)
		switch {
		case sourceTable == "direct_evidence_records" || tier == "evidence" || strings.HasPrefix(strings.ToLower(strings.TrimSpace(stringFromMap(hit, "id"))), "evidence:"):
			out.Trace["evidence_hit_count"] = intFromAny(out.Trace["evidence_hit_count"], 0) + 1
			if id <= 0 {
				out.Trace["missing_count"] = intFromAny(out.Trace["missing_count"], 0) + 1
				continue
			}
			item, ok := evidenceByID[id]
			if !ok {
				if blockedEvidenceIDs[id] {
					out.Trace["scope_filtered_count"] = intFromAny(out.Trace["scope_filtered_count"], 0) + 1
					continue
				}
				out.Trace["missing_count"] = intFromAny(out.Trace["missing_count"], 0) + 1
				continue
			}
			if item.Tombstoned || item.RepairNeeded || item.SupersededByID != 0 {
				out.Trace["scope_filtered_count"] = intFromAny(out.Trace["scope_filtered_count"], 0) + 1
				continue
			}
			if seenEvidence[id] {
				out.Trace["duplicate_count"] = intFromAny(out.Trace["duplicate_count"], 0) + 1
				continue
			}
			seenEvidence[id] = true
			out.Evidence = append(out.Evidence, item)
		case sourceTable == "world_rules" || tier == "world_rule" || strings.HasPrefix(strings.ToLower(strings.TrimSpace(stringFromMap(hit, "id"))), "world_rule:"):
			out.Trace["world_rule_hit_count"] = intFromAny(out.Trace["world_rule_hit_count"], 0) + 1
			if id <= 0 {
				out.Trace["missing_count"] = intFromAny(out.Trace["missing_count"], 0) + 1
				continue
			}
			item, ok := worldRuleByID[id]
			if !ok {
				out.Trace["missing_count"] = intFromAny(out.Trace["missing_count"], 0) + 1
				continue
			}
			if item.Suppressed {
				out.Trace["scope_filtered_count"] = intFromAny(out.Trace["scope_filtered_count"], 0) + 1
				continue
			}
			if seenWorldRule[id] {
				out.Trace["duplicate_count"] = intFromAny(out.Trace["duplicate_count"], 0) + 1
				continue
			}
			seenWorldRule[id] = true
			out.WorldRules = append(out.WorldRules, item)
		}
	}
	out.Trace["evidence_hydrated_count"] = len(out.Evidence)
	out.Trace["world_rule_hydrated_count"] = len(out.WorldRules)
	out.Trace["hydrated_count"] = len(out.Evidence) + len(out.WorldRules)
	if len(out.Evidence)+len(out.WorldRules) == 0 {
		out.Trace["status"] = "empty"
	}
	return out
}

func filterPrepareTurnPerspectiveScopedEvidence(
	evidence []store.DirectEvidence,
	memories []store.Memory,
) ([]store.DirectEvidence, map[int64]bool) {
	protectedEvidenceKeysByTurn := map[int]map[string]bool{}
	for _, memory := range memories {
		if memory.TurnIndex <= 0 {
			continue
		}
		extraction := parseJSONMap(memory.SummaryJSON)
		keys, _ := memoryAdmissionPerspectiveEvidenceScope(extraction)
		if len(keys) > 0 {
			protectedEvidenceKeysByTurn[memory.TurnIndex] = keys
		}
	}
	safe := make([]store.DirectEvidence, 0, len(evidence))
	blockedIDs := map[int64]bool{}
	for _, item := range evidence {
		if strings.EqualFold(strings.TrimSpace(item.EvidenceKind), "perspective_scoped_turn_excerpt") {
			if item.ID > 0 {
				blockedIDs[item.ID] = true
			}
			continue
		}
		start := item.SourceTurnStart
		end := item.SourceTurnEnd
		if start <= 0 {
			start = item.TurnAnchor
		}
		if end <= 0 {
			end = item.TurnAnchor
		}
		if end < start {
			start, end = end, start
		}
		blocked := false
		if start > 0 && end > 0 {
			evidenceKey := normalizeArtifactDedupeText(item.EvidenceText)
			for turn, protectedKeys := range protectedEvidenceKeysByTurn {
				if turn >= start && turn <= end && protectedKeys[evidenceKey] {
					blocked = true
					break
				}
			}
		}
		if blocked {
			if item.ID > 0 {
				blockedIDs[item.ID] = true
			}
			continue
		}
		safe = append(safe, item)
	}
	return safe, blockedIDs
}

func prepareTurnVectorSourceRowID(hit map[string]any) int64 {
	raw := strings.TrimSpace(stringFromMap(hit, "source_row_id"))
	if raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
			return id
		}
	}
	idText := strings.TrimSpace(stringFromMap(hit, "id"))
	if idText == "" {
		return 0
	}
	parts := strings.Split(idText, ":")
	for i := len(parts) - 1; i >= 0; i-- {
		part := strings.TrimSpace(parts[i])
		if id, err := strconv.ParseInt(part, 10, 64); err == nil && id > 0 {
			return id
		}
	}
	return 0
}

func prepareTurnVectorHistoryRowIDs(vectorShadow map[string]any) ([]int64, []int64) {
	memoryIDs := []int64{}
	evidenceIDs := []int64{}
	seenMemory := map[int64]bool{}
	seenEvidence := map[int64]bool{}
	for _, hit := range prepareTurnVectorMemorySearchResultMaps(vectorShadow) {
		score, ok := prepareTurnVectorHitSimilarity(hit)
		if !ok || !prepareTurnVectorSimilarityEligible(score, stringFromMap(hit, "similarity_source")) {
			continue
		}
		id := prepareTurnVectorSourceRowID(hit)
		if id <= 0 {
			continue
		}
		if prepareTurnVectorHitLooksLikeMemory(hit) {
			if !seenMemory[id] {
				seenMemory[id] = true
				memoryIDs = append(memoryIDs, id)
			}
		}
	}
	for _, hit := range prepareTurnVectorSearchResultMaps(vectorShadow["search_results"]) {
		score, ok := prepareTurnVectorHitSimilarity(hit)
		if !ok || !prepareTurnVectorSimilarityEligible(score, stringFromMap(hit, "similarity_source")) {
			continue
		}
		id := prepareTurnVectorSourceRowID(hit)
		if id <= 0 {
			continue
		}
		sourceTable := strings.ToLower(strings.TrimSpace(stringFromMap(hit, "source_table")))
		tier := strings.ToLower(strings.TrimSpace(stringFromMap(hit, "tier")))
		hitID := strings.ToLower(strings.TrimSpace(stringFromMap(hit, "id")))
		if sourceTable == "direct_evidence_records" || tier == "evidence" || strings.HasPrefix(hitID, "evidence:") {
			if !seenEvidence[id] {
				seenEvidence[id] = true
				evidenceIDs = append(evidenceIDs, id)
			}
		}
	}
	return memoryIDs, evidenceIDs
}

func mergePrepareTurnVectorArtifactCounters(counts map[string]any, hydration prepareTurnVectorArtifactHydration, directEvidenceInjected bool, directEvidenceLineCount, worldRuleLineCount int) {
	if counts == nil {
		return
	}
	trace := hydration.Trace
	if trace == nil {
		trace = map[string]any{}
	}
	evidenceInjected := 0
	if directEvidenceInjected {
		evidenceInjected = directEvidenceLineCount
	}
	worldRulesInjected := minInt(len(hydration.WorldRules), worldRuleLineCount)
	counts["vector_artifact_recall"] = trace
	counts["vector_evidence_hit_count"] = intFromAny(trace["evidence_hit_count"], 0)
	counts["vector_evidence_hydrated_count"] = intFromAny(trace["evidence_hydrated_count"], 0)
	counts["vector_evidence_selected_count"] = len(hydration.Evidence)
	counts["vector_evidence_injected_count"] = evidenceInjected
	counts["vector_world_rule_hit_count"] = intFromAny(trace["world_rule_hit_count"], 0)
	counts["vector_world_rule_hydrated_count"] = intFromAny(trace["world_rule_hydrated_count"], 0)
	counts["vector_world_rule_selected_count"] = len(hydration.WorldRules)
	counts["vector_world_rule_injected_count"] = worldRulesInjected
	counts["vector_scope_filtered_count"] = intFromAny(trace["scope_filtered_count"], 0)
	counts["vector_missing_count"] = intFromAny(counts["vector_memory_missing_count"], 0) + intFromAny(trace["missing_count"], 0)
	counts["vector_duplicate_count"] = intFromAny(counts["vector_memory_duplicate_count"], 0) + intFromAny(trace["duplicate_count"], 0)
	counts["vector_hit_count"] = intFromAny(counts["vector_memory_hit_count"], 0) + intFromAny(trace["evidence_hit_count"], 0) + intFromAny(trace["world_rule_hit_count"], 0)
	counts["vector_hydrated_count"] = intFromAny(counts["vector_memory_hydrated_count"], 0) + intFromAny(trace["evidence_hydrated_count"], 0) + intFromAny(trace["world_rule_hydrated_count"], 0)
	counts["vector_selected_count"] = intFromAny(counts["vector_memory_selected_count"], 0) + len(hydration.Evidence) + len(hydration.WorldRules)
	counts["vector_injected_count"] = intFromAny(counts["vector_memory_injected_count"], 0) + evidenceInjected + worldRulesInjected
}

func prepareTurnVectorHitHasLanguageMetadata(hit map[string]any) bool {
	return strings.TrimSpace(stringFromMap(hit, "raw_language")) != "" ||
		strings.TrimSpace(stringFromMap(hit, "summary_language")) != "" ||
		strings.TrimSpace(stringFromMap(hit, "session_output_language")) != ""
}

func prepareTurnMemoryHasLanguageMetadata(meta map[string]string) bool {
	return strings.TrimSpace(meta["raw_language"]) != "" ||
		strings.TrimSpace(meta["summary_language"]) != "" ||
		strings.TrimSpace(meta["session_output_language"]) != ""
}

func incrementLanguageCount(counts map[string]int, language string) {
	language = strings.TrimSpace(language)
	if language == "" {
		return
	}
	counts[language]++
}

func prepareTurnVectorHitLooksLikeMemory(hit map[string]any) bool {
	sourceTable := strings.ToLower(strings.TrimSpace(stringFromMap(hit, "source_table")))
	if sourceTable != "" {
		return sourceTable == "memories" || sourceTable == "memory"
	}
	tier := strings.ToLower(strings.TrimSpace(stringFromMap(hit, "tier")))
	if tier == "memory" || tier == "memories" {
		return true
	}
	id := strings.ToLower(strings.TrimSpace(stringFromMap(hit, "id")))
	return strings.HasPrefix(id, "memory:")
}

func prepareTurnVectorMemoryRowID(hit map[string]any) int64 {
	raw := strings.TrimSpace(stringFromMap(hit, "source_row_id"))
	if raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
			return id
		}
	}
	idText := strings.TrimSpace(stringFromMap(hit, "id"))
	if idText == "" {
		return 0
	}
	parts := strings.Split(idText, ":")
	for i := len(parts) - 1; i >= 0; i-- {
		part := strings.TrimSpace(parts[i])
		if part == "" {
			continue
		}
		if id, err := strconv.ParseInt(part, 10, 64); err == nil && id > 0 {
			return id
		}
	}
	return 0
}

func prepareTurnVectorSearchResultMaps(value any) []map[string]any {
	switch items := value.(type) {
	case []map[string]any:
		return items
	case []any:
		out := make([]map[string]any, 0, len(items))
		for _, item := range items {
			if m := mapFromAny(item); len(m) > 0 {
				out = append(out, m)
			}
		}
		return out
	default:
		return nil
	}
}

func prepareTurnVectorMemorySearchResultMaps(vectorShadow map[string]any) []map[string]any {
	if vectorShadow == nil {
		return nil
	}
	if value, exists := vectorShadow["memory_search_results"]; exists {
		return prepareTurnVectorSearchResultMaps(value)
	}
	return prepareTurnVectorSearchResultMaps(vectorShadow["search_results"])
}

func prepareTurnVectorHitSimilarity(hit map[string]any) (float64, bool) {
	if hit == nil {
		return 0, false
	}
	raw, ok := hit["similarity"]
	if !ok {
		return 0, false
	}
	score := extractionFloatFromAny(raw, -2)
	if score < -1 || score > 1 {
		return 0, false
	}
	return score, true
}

func prepareTurnVectorSimilarityEligible(score float64, source string) bool {
	if strings.HasSuffix(strings.TrimSpace(source), "distance_inverse") {
		return score >= prepareTurnMinInverseDistanceSimilarity
	}
	return score >= prepareTurnMinCosineSimilarity
}

func prepareTurnMemoryAlreadySelected(selection prepareTurnMemoryLaneSelection, item store.Memory) bool {
	key := prepareTurnMemoryLaneKey(item)
	for _, lane := range [][]store.Memory{selection.VectorRelevant, selection.Relevant, selection.Deep, selection.Recent} {
		for _, selected := range lane {
			if prepareTurnMemoryLaneKey(selected) == key {
				return true
			}
		}
	}
	return false
}

func relevantDegradedReason(query string, selected, candidates int) string {
	if strings.TrimSpace(query) == "" {
		return "missing_query"
	}
	if selected > 0 {
		return ""
	}
	if candidates == 0 {
		return "no_keyword_overlap_candidates"
	}
	return "candidate_limit_zero"
}
