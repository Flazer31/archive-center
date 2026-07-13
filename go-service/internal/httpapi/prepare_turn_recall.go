package httpapi

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

func (s *Server) prepareTurnVectorShadow(ctx context.Context, req dto.PrepareTurnRequest, limit int) map[string]any {
	shadow := map[string]any{
		"status":                       "unconfigured",
		"engine":                       "chromadb",
		"source":                       "go_r1_read_shadow",
		"note":                         "ChromaDB is the 2.0 vector accelerator; MariaDB remains canonical truth",
		"configured":                   s.Cfg.Readiness.ChromaConfigured,
		"chromadb_endpoint_configured": strings.TrimSpace(s.Cfg.ChromaEndpoint) != "",
		"recall_read_drill_enabled":    true,
		"product_read_enabled":         strings.TrimSpace(s.Cfg.ChromaEndpoint) != "" && s.VectorOpenError == nil,
		"live_retrieval_enabled":       false,
		"chromadb_live_enabled":        false,
		"health_checked":               false,
		"search_attempted":             false,
		"backfill_attempted":           false,
	}
	defer finalizePrepareTurnVectorShadow(shadow)
	if s.Vector == nil {
		shadow["status"] = "disabled"
		shadow["health_error"] = "vector store is not configured"
		return shadow
	}
	health, err := s.Vector.Health(ctx)
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
	queryKey := "chroma_query_vector"
	if len(queryVector) == 0 {
		shadow["query_embedding_attempted"] = true
		embeddingCfg := s.completeTurnExtractionConfig(req.ClientMeta).Embedder
		shadow["query_embedding_configured"] = embeddingCfg.hasConfig()
		shadow["query_embedding_model"] = strings.TrimSpace(embeddingCfg.Model)
		if !embeddingCfg.hasConfig() {
			shadow["search_skipped_reason"] = "missing_chroma_query_vector_and_embedding_config"
			shadow["query_embedding_missing_fields"] = embeddingCfg.missingFields()
			return shadow
		}
		queryText := strings.TrimSpace(stringPtrValue(req.RawUserInput, ""))
		if queryText == "" {
			queryText = strings.TrimSpace(stringPtrValue(req.ContinuityQuery, ""))
		}
		if queryText == "" {
			for i := len(req.Messages) - 1; i >= 0; i-- {
				msg := req.Messages[i]
				if strings.TrimSpace(fmt.Sprint(msg["role"])) == "assistant" {
					continue
				}
				queryText = strings.TrimSpace(fmt.Sprint(msg["content"]))
				if queryText != "" {
					break
				}
			}
		}
		if queryText == "" {
			shadow["search_skipped_reason"] = "missing_query_text_for_embedding"
			return shadow
		}
		embeddingJSON, model, err := callEmbedding(ctx, embeddingCfg, queryText)
		if err != nil {
			shadow["status"] = "degraded"
			shadow["query_embedding_status"] = "error"
			shadow["query_embedding_error"] = err.Error()
			shadow["search_skipped_reason"] = "query_embedding_failed"
			return shadow
		}
		queryVector = parseFloat32JSONList(embeddingJSON)
		if len(queryVector) == 0 {
			shadow["status"] = "degraded"
			shadow["query_embedding_status"] = "empty"
			shadow["search_skipped_reason"] = "query_embedding_empty"
			return shadow
		}
		queryKey = "server_query_embedding"
		shadow["query_embedding_status"] = "ok"
		shadow["query_embedding_model"] = model
	}

	if strings.TrimSpace(s.Cfg.ChromaEndpoint) != "" && s.VectorOpenError == nil {
		shadow["source"] = "go_r2_chromadb_product_read"
		shadow["note"] = "R2 product read proof: ChromaDB search is enabled as the support-only vector accelerator"
		shadow["live_retrieval_enabled"] = true
		shadow["chromadb_live_enabled"] = true
	} else {
		shadow["note"] = "R2 bounded recall read drill: ChromaDB vector search remains support-only until endpoint readiness is configured"
	}
	limit = prepareTurnRecallLimit(limit)
	filter := strings.TrimSpace(clientMetaString(req.ClientMeta, "chroma_filter"))
	if filter == "" {
		filter = fmt.Sprintf("chat_session_id == %q", req.ChatSessionID)
	}
	shadow["search_attempted"] = true
	shadow["query_vector_key"] = queryKey
	shadow["query_vector_dim"] = len(queryVector)
	shadow["limit"] = limit
	shadow["filter"] = filter
	results, err := s.Vector.Search(ctx, req.ChatSessionID, queryVector, limit, filter)
	switch {
	case err == nil:
		shadow["search_result"] = "ok"
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
	return shadow
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
			"degrade_mode":                  "raw_recent_fallback",
			"fallback_lane":                 "raw_fallback",
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
		"fallback_lane":                 "raw_fallback",
		"degrade_mode":                  "recent_relevant_deep_raw_fallback",
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
	return normalizePrepareTurnPerspectiveContext(meta)
}

func prepareTurnPerspectiveContextFromRequest(req dto.PrepareTurnRequest) map[string]any {
	if ctx := prepareTurnPerspectiveContextFromClientMeta(req.ClientMeta); len(ctx) > 0 {
		return ctx
	}
	sources := []struct {
		source string
		text   string
	}{
		{source: "raw_user_input", text: stringPtrValue(req.RawUserInput, "")},
	}
	for i := len(req.Messages) - 1; i >= 0 && len(sources) < 8; i-- {
		msg := req.Messages[i]
		text := strings.TrimSpace(extractionStringFromAny(msg["content"]))
		if text == "" {
			continue
		}
		role := strings.TrimSpace(extractionStringFromAny(msg["role"]))
		if role == "" {
			role = "message"
		}
		sources = append(sources, struct {
			source string
			text   string
		}{source: "message." + role, text: text})
	}
	for _, source := range sources {
		if pov := inferPrepareTurnPerspectiveName(source.text); pov != "" {
			return normalizePrepareTurnPerspectiveContext(map[string]any{
				"current_pov": pov,
				"source":      "inferred_" + source.source,
			})
		}
	}
	return nil
}

func inferPrepareTurnPerspectiveName(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if pov := inferPrepareTurnPerspectiveNameFromLine(line); pov != "" {
			return pov
		}
	}
	return ""
}

func inferPrepareTurnPerspectiveNameFromLine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	lower := strings.ToLower(line)
	for _, marker := range []string{"pov", "point of view", "viewpoint"} {
		if idx := strings.Index(lower, marker); idx >= 0 {
			after := strings.TrimSpace(line[idx+len(marker):])
			if after != "" {
				if candidate := cleanPrepareTurnPerspectiveCandidate(after); candidate != "" {
					return candidate
				}
			}
			before := strings.TrimSpace(line[:idx])
			if candidate := cleanPrepareTurnPerspectiveCandidate(before); candidate != "" {
				return candidate
			}
		}
	}
	for _, marker := range []string{"시점", "관점", "입장", "視点", "の視点"} {
		if idx := strings.Index(line, marker); idx >= 0 {
			before := strings.TrimSpace(line[:idx])
			if candidate := cleanPrepareTurnPerspectiveCandidate(before); candidate != "" {
				return candidate
			}
		}
	}
	return ""
}

func cleanPrepareTurnPerspectiveCandidate(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, " \t\r\n:：-–—[](){}<>「」『』\"'`")
	replacers := []string{
		"hidden spoiler", "", "spoiler", "", "pov", "", "point of view", "", "viewpoint", "",
		"current", "", "현재", "", "히든 스포일러", "", "스포일러", "", "의", "", "の", "",
	}
	lower := strings.ToLower(value)
	for i := 0; i+1 < len(replacers); i += 2 {
		prefix := replacers[i]
		replacement := replacers[i+1]
		if strings.HasPrefix(lower, prefix) {
			value = strings.TrimSpace(replacement + strings.TrimSpace(value[len(prefix):]))
			lower = strings.ToLower(value)
		}
	}
	cutset := []string{"\n", "\r", ".", "。", ",", "，", ";", "；", "|", "/", "\\", " - ", " -- ", " — ", " – "}
	for _, sep := range cutset {
		if idx := strings.Index(value, sep); idx >= 0 {
			value = strings.TrimSpace(value[:idx])
		}
	}
	value = strings.Trim(value, " \t\r\n:：-–—[](){}<>「」『』\"'`")
	for _, suffix := range []string{"의", "の"} {
		if strings.HasSuffix(value, suffix) {
			value = strings.TrimSpace(strings.TrimSuffix(value, suffix))
		}
	}
	if !validPrepareTurnPerspectiveCandidate(value) {
		return ""
	}
	return value
}

func validPrepareTurnPerspectiveCandidate(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	runeCount := len([]rune(value))
	if runeCount < 2 || runeCount > 60 {
		return false
	}
	lower := strings.ToLower(value)
	for _, blocked := range []string{
		"freely", "take a moment", "instruction", "instructions", "system", "developer", "assistant", "user",
		"prompt", "rules", "response", "format", "review", "reasoning", "draft",
	} {
		if strings.Contains(lower, blocked) {
			return false
		}
	}
	if len(strings.Fields(value)) > 5 {
		return false
	}
	return normalizeCharacterKey(value) != ""
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
	if pov == "" {
		return nil
	}
	out := map[string]any{
		"contract_version": "perspective_context.v1",
		"current_pov":      truncateRunes(pov, 120),
		"current_pov_key":  normalizeCharacterKey(pov),
		"source":           extractionFirstNonEmpty(extractionStringFromAny(raw["source"]), "client_meta"),
	}
	if mode := strings.TrimSpace(extractionStringFromAny(raw["mode"])); mode != "" {
		out["mode"] = truncateRunes(mode, 80)
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

func prepareTurnSupportRecallLimit(topK int) int {
	return prepareTurnRecallLimit(topK)
}

func prepareTurnTextBudget(maxChars int) int {
	if maxChars > 0 {
		return maxChars
	}
	return 1
}

type prepareTurnMemoryLaneSelection struct {
	VectorRelevant []store.Memory
	Recent         []store.Memory
	Relevant       []store.Memory
	Deep           []store.Memory
	VectorScores   map[string]float64
	RelevantScores map[string]float64
	Trace          map[string]any
}

func prepareTurnMemorySelectionQuery(rawUserInput string, chatLogs []store.ChatLog, perspectiveContext map[string]any, topK int) string {
	parts := []string{}
	if text := strings.TrimSpace(rawUserInput); text != "" {
		parts = append(parts, text)
	}
	if pov := strings.TrimSpace(extractionStringFromAny(perspectiveContext["current_pov"])); pov != "" {
		parts = append(parts, "current_pov: "+pov)
	}
	for _, cl := range selectRecentChatLogsByTurn(chatLogs, prepareTurnRecallLimit(topK)) {
		if text := strings.TrimSpace(cl.Content); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func selectPrepareTurnMemoryLanes(memories []store.Memory, query string, topK int) prepareTurnMemoryLaneSelection {
	return selectPrepareTurnMemoryLanesWithVector(memories, query, topK, nil)
}

func selectPrepareTurnMemoryLanesWithVector(memories []store.Memory, query string, topK int, vectorShadow map[string]any) prepareTurnMemoryLaneSelection {
	topK = prepareTurnRecallLimit(topK)
	totalLimit := topK
	clean := make([]store.Memory, 0, len(memories))
	for _, item := range memories {
		if strings.TrimSpace(prepareTurnMemorySummary(item)) == "" {
			continue
		}
		clean = append(clean, item)
	}
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
		VectorScores:   map[string]float64{},
		RelevantScores: map[string]float64{},
		Trace: map[string]any{
			"version":                     "r3.recall_lanes.v1",
			"top_k_definition":            "semantic_memory_recall_limit",
			"top_k_memory_target":         totalLimit,
			"vector_memory_policy":        "chromadb_hits_hydrated_to_mariadb_memory_before_injection",
			"relevant_memory_limit":       totalLimit,
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
	vectorHydration := prepareTurnHydrateVectorMemoryHits(clean, vectorShadow, totalLimit)
	vectorRecallReady := prepareTurnVectorRecallReady(vectorHydration.Trace)
	vectorRecallAttempted := prepareTurnVectorSearchAttempted(vectorShadow)
	for _, item := range vectorHydration.Items {
		if prepareTurnSelectedMemoryCount(out) >= totalLimit {
			break
		}
		out.VectorRelevant = append(out.VectorRelevant, item)
		key := prepareTurnMemoryLaneKey(item)
		if score := vectorHydration.Scores[key]; score > 0 {
			out.VectorScores[key] = score
		}
	}
	out.Trace["vector_recall"] = vectorHydration.Trace
	out.Trace["vector_recall_ready"] = vectorRecallReady
	out.Trace["vector_recall_attempted"] = vectorRecallAttempted
	out.Trace["lexical_fill_enabled"] = prepareTurnSelectedMemoryCount(out) < totalLimit
	if vectorRecallReady && prepareTurnSelectedMemoryCount(out) >= totalLimit {
		out.Trace["vector_selected"] = len(out.VectorRelevant)
		out.Trace["recent_selected"] = 0
		out.Trace["relevant_selected"] = 0
		out.Trace["deep_selected"] = 0
		out.Trace["selected_total"] = prepareTurnSelectedMemoryCount(out)
		out.Trace["relevant_candidates"] = 0
		out.Trace["memory_budget_remaining"] = maxInt(totalLimit-prepareTurnSelectedMemoryCount(out), 0)
		out.Trace["average_importance"] = avgImportance
		out.Trace["relevant_degraded_reason"] = nilIfEmpty(relevantDegradedReason(query, len(out.Relevant), 0))
		return out
	}

	type scoredMemory struct {
		item       store.Memory
		key        string
		relevance  float64
		importance float64
		recency    float64
	}
	scored := []scoredMemory{}
	relevantCandidates := 0
	for _, item := range clean {
		key := prepareTurnMemoryLaneKey(item)
		relevance := 0.0
		if queryPresent {
			relevance = simpleTokenSimilarity(query, prepareTurnMemoryRelevanceText(item))
			if relevance > 0 {
				relevantCandidates++
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
			importance: item.Importance,
			recency:    recency,
		})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		ia := scored[i]
		ja := scored[j]
		if queryPresent {
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

	for _, candidate := range scored {
		if prepareTurnSelectedMemoryCount(out) >= totalLimit {
			break
		}
		if prepareTurnMemoryAlreadySelected(out, candidate.item) {
			continue
		}
		if candidate.relevance > 0 {
			out.Relevant = append(out.Relevant, candidate.item)
			out.RelevantScores[candidate.key] = candidate.relevance
			continue
		}
		if !queryPresent && avgImportance > 0 && candidate.importance >= avgImportance {
			out.Deep = append(out.Deep, candidate.item)
			continue
		}
		out.Recent = append(out.Recent, candidate.item)
	}
	out.Trace["vector_selected"] = len(out.VectorRelevant)
	out.Trace["recent_selected"] = len(out.Recent)
	out.Trace["relevant_selected"] = len(out.Relevant)
	out.Trace["deep_selected"] = len(out.Deep)
	out.Trace["selected_total"] = prepareTurnSelectedMemoryCount(out)
	out.Trace["relevant_candidates"] = relevantCandidates
	out.Trace["memory_budget_remaining"] = maxInt(totalLimit-prepareTurnSelectedMemoryCount(out), 0)
	out.Trace["average_importance"] = avgImportance
	out.Trace["relevant_degraded_reason"] = nilIfEmpty(relevantDegradedReason(query, len(out.Relevant), relevantCandidates))
	return out
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
	return boolFromAny(vectorShadow["search_attempted"])
}

func prepareTurnSelectedMemoryCount(selection prepareTurnMemoryLaneSelection) int {
	return len(selection.VectorRelevant) + len(selection.Recent) + len(selection.Relevant) + len(selection.Deep)
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
		"protected_memory_dropped_count":                intFromAny(selection.Trace["protected_memory_dropped_count"], 0),
		"protected_memory_gate":                         stringFromMap(selection.Trace, "protected_memory_gate"),
		"selected_memory_total_count":                   prepareTurnSelectedMemoryCount(selection),
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

func collapsePrepareTurnMemoryLaneSelection(selection prepareTurnMemoryLaneSelection) prepareTurnMemoryLaneSelection {
	seen := map[string]bool{}
	collapsed := 0
	collapseLane := func(items []store.Memory) []store.Memory {
		out := make([]store.Memory, 0, len(items))
		for _, item := range items {
			key := collapseTextKey(prepareTurnMemorySummary(item))
			if key == "" {
				key = prepareTurnMemoryLaneKey(item)
			}
			if key != "" && seen[key] {
				collapsed++
				continue
			}
			if key != "" {
				seen[key] = true
			}
			out = append(out, item)
		}
		return out
	}
	selection.VectorRelevant = collapseLane(selection.VectorRelevant)
	selection.Relevant = collapseLane(selection.Relevant)
	selection.Deep = collapseLane(selection.Deep)
	selection.Recent = collapseLane(selection.Recent)
	if selection.Trace == nil {
		selection.Trace = map[string]any{}
	}
	selection.Trace["memory_collapsed_count"] = collapsed
	selection.Trace["selected_total_after_collapse"] = prepareTurnSelectedMemoryCount(selection)
	return selection
}

func filterPrepareTurnProtectedMemoryLaneSelection(selection prepareTurnMemoryLaneSelection, rawUserInput string, chatLogs []store.ChatLog, perspectiveContext map[string]any) prepareTurnMemoryLaneSelection {
	ctx := buildPrepareTurnRecollectionContext(rawUserInput, chatLogs, nil, nil)
	before := prepareTurnSelectedMemoryCount(selection)
	dropped := []map[string]any{}
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
	if selection.Trace == nil {
		selection.Trace = map[string]any{}
	}
	selection.Trace["protected_memory_before_filter"] = before
	selection.Trace["protected_memory_after_filter"] = prepareTurnSelectedMemoryCount(selection)
	selection.Trace["protected_memory_dropped_count"] = len(dropped)
	selection.Trace["protected_memory_gate"] = "protected_owner_subject_knowledge_scope_or_current_pov_must_match_current_user_input_immediate_chat_or_pov"
	selection.Trace["protected_memory_dropped"] = dropped
	return selection
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
	if prepareTurnAnyOwnerTokenMatches(tokens, ctx.immediateChatText) {
		return true, "immediate_chat_mention"
	}
	if pov := strings.TrimSpace(extractionStringFromAny(perspectiveContext["current_pov"])); pov != "" && prepareTurnAnyOwnerTokenMatches(tokens, pov) {
		return true, "current_pov_match"
	}
	return false, "protected_entity_not_in_current_input_or_immediate_chat"
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
	seen := map[string]bool{}
	for _, item := range items {
		if item.Suppressed {
			continue
		}
		key := strings.Join([]string{
			collapseTextKey(item.Scope),
			collapseTextKey(item.ScopeName),
			collapseTextKey(item.Category),
			collapseTextKey(item.Key),
			collapseTextKey(item.ValueJSON),
		}, "|")
		if strings.Trim(key, "|") == "" {
			continue
		}
		if seen[key] {
			continue
		}
		seen[key] = true
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
	if strings.TrimSpace(stringFromMap(vectorShadow, "search_result")) != "ok" {
		out.Trace["status"] = "skipped"
		out.Trace["reason"] = strings.TrimSpace(stringFromMap(vectorShadow, "search_result"))
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
	hits := prepareTurnVectorSearchResultMaps(vectorShadow["search_results"])
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

func prepareTurnHydrateVectorArtifactHits(evidence []store.DirectEvidence, worldRules []store.WorldRule, vectorShadow map[string]any, limit int) prepareTurnVectorArtifactHydration {
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

func prepareTurnNeedsRawFallback(selection prepareTurnMemoryLaneSelection, topK int) bool {
	if boolFromAny(selection.Trace["vector_recall_ready"]) {
		return false
	}
	if boolFromAny(selection.Trace["vector_recall_attempted"]) {
		return false
	}
	return prepareTurnSelectedMemoryCount(selection) < prepareTurnRecallLimit(topK)
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
