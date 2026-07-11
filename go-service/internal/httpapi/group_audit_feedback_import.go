package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

// Audit / feedback / import

func (s *Server) handleAuditGet(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if rawLimit := strings.TrimSpace(r.URL.Query().Get("limit")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed < 0 {
			writeBadRequest(w, "limit must be a non-negative integer")
			return
		}
		if parsed > 0 {
			limit = parsed
		}
	}
	chatSessionID := strings.TrimSpace(r.URL.Query().Get("chat_session_id"))
	eventType := strings.TrimSpace(r.URL.Query().Get("event_type"))

	items, err := s.Store.ListAuditLogs(r.Context(), chatSessionID, eventType, limit)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			items = []store.AuditLog{}
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	items = nonNilSlice(items)
	// Convert to snake_case maps to match Python 0.8 response shape
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{
			"id":              item.ID,
			"created_at":      formatKSTTime(item.CreatedAt),
			"event_type":      item.EventType,
			"chat_session_id": item.ChatSessionID,
			"target_type":     nullableString(item.TargetType),
			"target_id":       nullableInt64(item.TargetID),
			"summary":         item.Summary,
			"details_json":    item.DetailsJSON,
			"source":          item.Source,
		})
	}
	total := len(out)
	if counter, ok := s.Store.(store.AuditLogCounter); ok {
		if found, err := counter.CountAuditLogs(r.Context(), chatSessionID, eventType); err == nil {
			total = found
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"items":  out,
		"limit":  limit,
		"offset": 0,
		"total":  total,
	})
}

func (s *Server) saveAuditLogBestEffort(ctx context.Context, audit *store.AuditLog) {
	if s == nil || s.Store == nil || audit == nil {
		return
	}
	if audit.CreatedAt.IsZero() {
		audit.CreatedAt = time.Now().UTC()
	}
	_ = s.Store.SaveAuditLog(ctx, audit)
}

func (s *Server) handleFeedbackPost(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChatSessionID string `json:"chat_session_id"`
		TargetType    string `json:"target_type"`
		TargetID      int64  `json:"target_id"`
		FeedbackValue string `json:"feedback_value"`
		FeedbackNote  string `json:"feedback_note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	req.ChatSessionID = strings.TrimSpace(req.ChatSessionID)
	req.TargetType = strings.TrimSpace(req.TargetType)
	req.FeedbackValue = strings.TrimSpace(strings.ToLower(req.FeedbackValue))
	req.FeedbackNote = strings.TrimSpace(req.FeedbackNote)
	if req.ChatSessionID == "" {
		writeBadRequest(w, "chat_session_id is required")
		return
	}
	if req.TargetID <= 0 {
		writeBadRequest(w, "target_id must be a positive integer")
		return
	}
	if req.TargetType != "memory" && req.TargetType != "kg_triple" {
		writeBadRequest(w, "target_type must be memory or kg_triple")
		return
	}
	if req.FeedbackValue != "up" && req.FeedbackValue != "down" {
		writeBadRequest(w, "feedback_value must be up or down")
		return
	}
	if ok, err := s.feedbackTargetBelongsToSession(r.Context(), req.ChatSessionID, req.TargetType, req.TargetID); err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeError(w, http.StatusServiceUnavailable, CodeShadowGuard, "POST /feedback requires canonical store reads")
			return
		}
		writeInternalError(w, err.Error())
		return
	} else if !ok {
		writeBadRequest(w, fmt.Sprintf("%s #%d not found for chat_session_id %s", req.TargetType, req.TargetID, req.ChatSessionID))
		return
	}

	now := time.Now().UTC()
	feedback := &store.CriticFeedback{
		ChatSessionID: req.ChatSessionID,
		TargetType:    req.TargetType,
		TargetID:      req.TargetID,
		FeedbackValue: req.FeedbackValue,
		FeedbackNote:  req.FeedbackNote,
		Source:        "manual_ui",
		CreatedAt:     now,
	}
	if err := s.Store.SaveCriticFeedback(r.Context(), feedback); err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeError(w, http.StatusServiceUnavailable, CodeShadowGuard, "POST /feedback requires canonical store writes")
			return
		}
		writeInternalError(w, err.Error())
		return
	}

	if items, err := s.Store.ListCriticFeedback(r.Context(), req.ChatSessionID, req.TargetType, req.TargetID); err == nil && len(items) > 0 {
		feedback.ID = items[0].ID
		feedback.CreatedAt = items[0].CreatedAt
	}
	audit := &store.AuditLog{
		ChatSessionID: req.ChatSessionID,
		EventType:     "critic_feedback",
		TargetType:    req.TargetType,
		TargetID:      req.TargetID,
		Summary:       fmt.Sprintf("Feedback %s on %s #%d", req.FeedbackValue, req.TargetType, req.TargetID),
		DetailsJSON:   mustCompactJSON(map[string]any{"feedback_value": req.FeedbackValue, "feedback_note": req.FeedbackNote}),
		Source:        "manual_ui",
		CreatedAt:     now,
	}
	_ = s.Store.SaveAuditLog(r.Context(), audit)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":         "ok",
		"ok":             true,
		"detail":         fmt.Sprintf("%s #%d feedback saved", req.TargetType, req.TargetID),
		"feedback_id":    feedback.ID,
		"feedback_value": feedback.FeedbackValue,
	})
}

func (s *Server) handleFeedbackLatest(w http.ResponseWriter, r *http.Request) {
	chatSessionID := strings.TrimSpace(r.URL.Query().Get("chat_session_id"))
	targetType := strings.TrimSpace(r.URL.Query().Get("target_type"))
	targetID := int64(0)
	if rawTargetID := strings.TrimSpace(r.URL.Query().Get("target_id")); rawTargetID != "" {
		parsed, err := strconv.ParseInt(rawTargetID, 10, 64)
		if err != nil || parsed < 0 {
			writeBadRequest(w, "target_id must be a non-negative integer")
			return
		}
		targetID = parsed
	}
	targetIDs := []int64{}
	if targetID > 0 {
		targetIDs = append(targetIDs, targetID)
	}
	if rawTargetIDs := strings.TrimSpace(r.URL.Query().Get("target_ids")); rawTargetIDs != "" {
		for _, part := range strings.Split(rawTargetIDs, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			parsed, err := strconv.ParseInt(part, 10, 64)
			if err != nil || parsed <= 0 {
				writeBadRequest(w, "target_ids must be comma-separated positive integers")
				return
			}
			targetIDs = append(targetIDs, parsed)
		}
	}

	items := []store.CriticFeedback{}
	if chatSessionID != "" {
		queryTargetID := int64(0)
		if len(targetIDs) == 1 {
			queryTargetID = targetIDs[0]
		}
		found, err := s.Store.ListCriticFeedback(r.Context(), chatSessionID, targetType, queryTargetID)
		if err != nil {
			if errors.Is(err, store.ErrNotEnabled) {
				found = nil
			} else {
				writeInternalError(w, err.Error())
				return
			}
		}
		items = found
	}

	mappedItems := make([]map[string]any, 0, len(items))
	feedbacks := map[string]any{}
	targetFilter := map[int64]bool{}
	for _, id := range targetIDs {
		targetFilter[id] = true
	}
	for _, item := range items {
		mapped := map[string]any{
			"id":              item.ID,
			"created_at":      formatKSTTime(item.CreatedAt),
			"chat_session_id": item.ChatSessionID,
			"target_type":     item.TargetType,
			"target_id":       item.TargetID,
			"feedback_value":  item.FeedbackValue,
			"feedback_note":   nullableString(item.FeedbackNote),
			"source":          item.Source,
		}
		mappedItems = append(mappedItems, mapped)
		if len(targetFilter) > 0 && !targetFilter[item.TargetID] {
			continue
		}
		key := strconv.FormatInt(item.TargetID, 10)
		if _, exists := feedbacks[key]; exists {
			continue
		}
		feedbacks[key] = map[string]any{
			"feedback_value": item.FeedbackValue,
			"feedback_note":  nullableString(item.FeedbackNote),
			"created_at":     formatKSTTime(item.CreatedAt),
		}
	}
	var latest any
	if len(mappedItems) > 0 {
		latest = mappedItems[0]
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"source":          "shadow",
		"chat_session_id": chatSessionID,
		"target_type":     targetType,
		"target_id":       targetID,
		"target_ids":      targetIDs,
		"latest":          latest,
		"feedbacks":       feedbacks,
		"items":           mappedItems,
		"count":           len(mappedItems),
	})
}

func (s *Server) feedbackTargetBelongsToSession(ctx context.Context, sid string, targetType string, targetID int64) (bool, error) {
	switch targetType {
	case "memory":
		items, err := s.Store.ListMemories(ctx, sid, 0, 0)
		if err != nil {
			return false, err
		}
		for _, item := range items {
			if item.ID == targetID && item.ChatSessionID == sid {
				return true, nil
			}
		}
	case "kg_triple":
		items, err := s.Store.ListKGTriples(ctx, sid)
		if err != nil {
			return false, err
		}
		for _, item := range items {
			if item.ID == targetID && item.ChatSessionID == sid {
				return true, nil
			}
		}
	}
	return false, nil
}

func (s *Server) handleImportHypamemory(w http.ResponseWriter, r *http.Request) {
	var req dto.HypaImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "code": "invalid_json", "detail": err.Error()})
		return
	}
	sid := strings.TrimSpace(req.ChatSessionID)
	if sid == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "code": "missing_chat_session_id", "detail": "chat_session_id is required"})
		return
	}
	if len(req.Summaries) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "code": "empty_summaries", "detail": "summaries must not be empty"})
		return
	}
	if len(req.Summaries) > 500 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "code": "too_many_summaries", "detail": "summaries are limited to 500 items", "total": len(req.Summaries)})
		return
	}
	if s.Store == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "error", "code": "store_not_enabled", "detail": "store is not enabled"})
		return
	}

	extractionCfg := s.completeTurnExtractionConfig(nil)
	llmTrace := completeTurnLLMConfigTrace(extractionCfg)
	if !extractionCfg.Critic.hasConfig() {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":           "error",
			"code":             "critic_config_missing",
			"detail":           "Critic provider settings are required for HypaMemory import.",
			"chat_session_id":  sid,
			"total":            len(req.Summaries),
			"succeeded":        0,
			"failed":           len(req.Summaries),
			"llm_config_trace": llmTrace,
			"warnings":         []string{"critic_config_missing"},
		})
		return
	}

	now := time.Now().UTC()
	succeeded := 0
	failed := 0
	skipped := 0
	errorDetails := []string{}
	warnings := []string{}
	criticTraces := []map[string]any{}
	scoringTraces := []map[string]any{}
	artifactCounts := map[string]int{
		"memories":         0,
		"direct_evidence":  0,
		"kg_triples":       0,
		"entities":         0,
		"trust_states":     0,
		"character_states": 0,
		"character_events": 0,
		"storylines":       0,
		"world_rules":      0,
		"pending_threads":  0,
		"active_states":    0,
		"vectors":          0,
	}

	for idx, summary := range req.Summaries {
		summary.ApplyDefaults()
		text := strings.TrimSpace(summary.Text)
		if text == "" {
			skipped++
			continue
		}
		turnIndex := hypaImportTurnIndex(summary, idx)
		tags := []string{}
		for _, tag := range summary.Tags {
			if cleaned := strings.TrimSpace(tag); cleaned != "" {
				tags = append(tags, cleaned)
			}
		}
		hints := []string{"source=HypaMemory import"}
		if summary.IsImportant != nil && *summary.IsImportant {
			hints = append(hints, "importance_hint=important")
		}
		if summary.Category != nil && strings.TrimSpace(*summary.Category) != "" {
			hints = append(hints, "category="+strings.TrimSpace(*summary.Category))
		}
		if len(tags) > 0 {
			hints = append(hints, "tags="+strings.Join(tags, ", "))
		}
		score, scoringTrace, scoringErr := s.scoreHypaMemoryImport(r.Context(), sid, summary, idx, turnIndex, extractionCfg.Critic)
		if scoringErr != nil {
			warnings = append(warnings, fmt.Sprintf("hypamemory_import_score_failed[%d]: %v", idx, scoringErr))
			score = fallbackHypaMemoryImportScore(summary)
			scoringTrace = map[string]any{"status": "fallback", "error": scoringErr.Error(), "score": score.mapValue()}
		}
		scoringTraces = append(scoringTraces, map[string]any{"index": idx, "turn_index": turnIndex, "trace": scoringTrace})
		hints = append(hints, "hypamemory_import_score="+mustCompactJSON(score.mapValue()))
		content := strings.Join(hints, "; ") + "\n" + text

		extraction, trace, err := s.runCompleteTurnCritic(r.Context(), sid, turnIndex, "HypaMemory import summary", content, nil, nil, extractionCfg.Critic)
		if trace != nil {
			criticTraces = append(criticTraces, map[string]any{"index": idx, "turn_index": turnIndex, "trace": trace})
		}
		if err != nil {
			failed++
			errorDetails = append(errorDetails, fmt.Sprintf("summary[%d]: %v", idx, err))
			continue
		}
		applyHypaMemoryImportScore(extraction, score)
		result := s.saveCriticExtractionArtifacts(r.Context(), sid, turnIndex, extraction, content, extractionCfg.Embedder, now)
		artifactCounts["memories"] += result.Memories
		artifactCounts["direct_evidence"] += result.Evidence
		artifactCounts["kg_triples"] += result.KGTriples
		artifactCounts["entities"] += result.Entities
		artifactCounts["trust_states"] += result.TrustStates
		artifactCounts["character_states"] += result.CharacterStates
		artifactCounts["character_events"] += result.CharacterEvents
		artifactCounts["storylines"] += result.Storylines
		artifactCounts["world_rules"] += result.WorldRules
		artifactCounts["pending_threads"] += result.PendingThreads
		artifactCounts["active_states"] += result.ActiveStates
		artifactCounts["vectors"] += result.VectorsUpserted
		warnings = append(warnings, result.Warnings...)
		if result.Errors > 0 {
			failed++
			errorDetails = append(errorDetails, result.ErrorDetails...)
			continue
		}
		succeeded++
	}

	status := "ok"
	if failed > 0 {
		status = "partial_error"
	}
	if succeeded == 0 && failed > 0 {
		status = "error"
	}
	detail := fmt.Sprintf("HypaMemory import processed: %d/%d succeeded", succeeded, len(req.Summaries))
	auditDetails := map[string]any{"total": len(req.Summaries), "succeeded": succeeded, "failed": failed, "skipped": skipped, "artifact_counts": artifactCounts}
	_ = s.Store.SaveAuditLog(r.Context(), &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "hypamemory_import",
		TargetType:    "session",
		Summary:       detail,
		DetailsJSON:   mustCompactJSON(auditDetails),
		Source:        "import",
		CreatedAt:     now,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           status,
		"code":             "hypamemory_import",
		"detail":           detail,
		"chat_session_id":  sid,
		"total":            len(req.Summaries),
		"succeeded":        succeeded,
		"failed":           failed,
		"skipped":          skipped,
		"artifact_counts":  artifactCounts,
		"errors":           errorDetails,
		"warnings":         warnings,
		"llm_config_trace": llmTrace,
		"critic_traces":    criticTraces,
		"scoring_traces":   scoringTraces,
		"scoring_policy":   hypaMemoryImportScoringPolicyVersion,
	})
}

const hypaMemoryImportScoringPolicyVersion = "hypa-import-score.v1"

type hypaMemoryImportScore struct {
	Importance10             float64
	RetrievalPriority        float64
	ContinuityWeight         float64
	DialogueOrSensoryDensity float64
	MemoryKind               string
	TimeAnchorQuality        string
	KeepReason               string
	EntityRelevance          []string
	Source                   string
}

func (s hypaMemoryImportScore) mapValue() map[string]any {
	return map[string]any{
		"policy_version":                hypaMemoryImportScoringPolicyVersion,
		"importance_10":                 roundHypaImportScore(s.Importance10),
		"retrieval_priority":            roundHypaImportScore(s.RetrievalPriority),
		"continuity_weight":             roundHypaImportScore(s.ContinuityWeight),
		"dialogue_or_sensory_density":   roundHypaImportScore(s.DialogueOrSensoryDensity),
		"memory_kind":                   s.MemoryKind,
		"time_anchor_quality":           s.TimeAnchorQuality,
		"keep_reason":                   s.KeepReason,
		"entity_relevance":              s.EntityRelevance,
		"source":                        s.Source,
		"hypamemory_min_importance_10":  5.0,
		"used_as_importance_floor":      true,
		"truth_authority":               "support_import_scoring_only",
		"canonical_truth_write_allowed": false,
	}
}

func (s *Server) scoreHypaMemoryImport(ctx context.Context, sid string, summary dto.HypaImportSummary, idx int, turnIndex int, cfg completeTurnLLMConfig) (hypaMemoryImportScore, map[string]any, error) {
	fallback := fallbackHypaMemoryImportScore(summary)
	if !cfg.hasConfig() {
		return fallback, map[string]any{"status": "fallback", "reason": "critic_config_missing", "score": fallback.mapValue()}, nil
	}
	systemPrompt := "You score imported HypaMemory summaries for long-term story recall. Return only compact JSON."
	userPrompt := buildHypaMemoryImportScoringPrompt(sid, summary, idx, turnIndex)
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 || maxTokens > 700 {
		maxTokens = 700
	}
	maxCompletionTokens := cfg.MaxCompletionTokens
	if maxCompletionTokens <= 0 || maxCompletionTokens > maxTokens {
		maxCompletionTokens = maxTokens
	}
	temp := cfg.Temperature
	req := dto.ProxyPluginMainRequest{
		APIKey:              &cfg.APIKey,
		Endpoint:            &cfg.Endpoint,
		Model:               &cfg.Model,
		Provider:            &cfg.Provider,
		Messages:            []any{map[string]any{"role": "system", "content": systemPrompt}, map[string]any{"role": "user", "content": userPrompt}},
		MaxTokens:           &maxTokens,
		MaxCompletionTokens: &maxCompletionTokens,
		Temperature:         &temp,
		TimeoutMs:           &cfg.TimeoutMs,
	}
	if strings.TrimSpace(cfg.ReasoningEffort) != "" {
		req.ReasoningEffort = &cfg.ReasoningEffort
	}
	if strings.TrimSpace(cfg.ReasoningPreset) != "" {
		req.ReasoningPreset = &cfg.ReasoningPreset
	}
	if cfg.ReasoningBudgetTokens > 0 {
		req.ReasoningBudgetTokens = &cfg.ReasoningBudgetTokens
		req.BudgetTokens = &cfg.ReasoningBudgetTokens
	}
	if strings.TrimSpace(cfg.GlmThinkingType) != "" {
		req.GlmThinkingType = &cfg.GlmThinkingType
	}
	applyProxyOverridesFromLLMConfig(&req, cfg)

	upstream, _, err := performProxyPluginMain(ctx, req)
	if err != nil {
		return fallback, nil, err
	}
	content := chatCompletionText(upstream)
	parsed, err := parseJSONFromLLMContent(content)
	if err != nil {
		return fallback, map[string]any{"status": "parse_failed", "raw_preview": truncateRunes(content, 800)}, err
	}
	score := normalizeHypaMemoryImportScore(parsed, fallback)
	trace := map[string]any{
		"status":         "ok",
		"policy_version": hypaMemoryImportScoringPolicyVersion,
		"model":          extractionFirstNonEmpty(extractionStringFromAny(upstream["model"]), cfg.Model),
		"usage":          upstream["usage"],
		"score":          score.mapValue(),
	}
	return score, trace, nil
}

func buildHypaMemoryImportScoringPrompt(sid string, summary dto.HypaImportSummary, idx int, turnIndex int) string {
	payload := map[string]any{
		"chat_session_id":    sid,
		"summary_index":      idx,
		"import_turn_index":  turnIndex,
		"text":               boundCompleteTurnCriticInput(sanitizeTextForCriticInput(summary.Text), 6000),
		"source":             "HypaMemory import",
		"is_important":       boolPtrValue(summary.IsImportant, false),
		"category":           stringPtrValue(summary.Category, ""),
		"tags":               summary.Tags,
		"minimum_importance": "5/10 for any useful imported long-term memory",
		"truth_authority":    "support scoring only; do not invent new facts",
		"scoring_dimensions": []string{"importance_10", "retrieval_priority", "continuity_weight", "dialogue_or_sensory_density", "memory_kind", "entity_relevance", "time_anchor_quality", "keep_reason"},
	}
	body, _ := json.Marshal(payload)
	return strings.Join([]string{
		"Score this imported HypaMemory summary for Archive Center retrieval.",
		"Use the summary as an old long-term memory candidate, not as a current-turn fact.",
		"Return only JSON with:",
		`{"importance_10":number 1..10,"retrieval_priority":number 0..1,"continuity_weight":number 0..1,"dialogue_or_sensory_density":number 0..1,"memory_kind":string,"entity_relevance":[string],"time_anchor_quality":string,"keep_reason":string}`,
		"Rules:",
		"- importance_10 below 5 is only allowed for empty/noise/control text.",
		"- Prefer higher scores for relationship shifts, injuries, vows, secrets, locations, time anchors, recurring conflicts, irreversible decisions, and strong dialogue/sensory detail.",
		"- Do not add facts not present in the summary.",
		"Input JSON:",
		string(body),
	}, "\n")
}

func normalizeHypaMemoryImportScore(raw map[string]any, fallback hypaMemoryImportScore) hypaMemoryImportScore {
	score := hypaMemoryImportScore{
		Importance10:             clampFloat(extractionFloatFromAny(raw["importance_10"], extractionFloatFromAny(raw["importance_score"], fallback.Importance10)), 1, 10),
		RetrievalPriority:        clampFloat(extractionFloatFromAny(raw["retrieval_priority"], fallback.RetrievalPriority), 0, 1),
		ContinuityWeight:         clampFloat(extractionFloatFromAny(raw["continuity_weight"], fallback.ContinuityWeight), 0, 1),
		DialogueOrSensoryDensity: clampFloat(extractionFloatFromAny(raw["dialogue_or_sensory_density"], fallback.DialogueOrSensoryDensity), 0, 1),
		MemoryKind:               extractionFirstNonEmpty(extractionStringFromAny(raw["memory_kind"]), fallback.MemoryKind),
		TimeAnchorQuality:        extractionFirstNonEmpty(extractionStringFromAny(raw["time_anchor_quality"]), fallback.TimeAnchorQuality),
		KeepReason:               extractionFirstNonEmpty(extractionStringFromAny(raw["keep_reason"]), fallback.KeepReason),
		EntityRelevance:          stringsFromAny(raw["entity_relevance"]),
		Source:                   "llm_scoring",
	}
	if score.Importance10 < 5 {
		score.Importance10 = 5
	}
	if len(score.EntityRelevance) == 0 {
		score.EntityRelevance = fallback.EntityRelevance
	}
	return score
}

func fallbackHypaMemoryImportScore(summary dto.HypaImportSummary) hypaMemoryImportScore {
	important := boolPtrValue(summary.IsImportant, false)
	importance := 5.0
	if important {
		importance = 6.0
	}
	tags := make([]string, 0, len(summary.Tags))
	for _, tag := range summary.Tags {
		if cleaned := strings.TrimSpace(tag); cleaned != "" {
			tags = append(tags, cleaned)
		}
	}
	return hypaMemoryImportScore{
		Importance10:             importance,
		RetrievalPriority:        0.55,
		ContinuityWeight:         0.55,
		DialogueOrSensoryDensity: 0.35,
		MemoryKind:               extractionFirstNonEmpty(stringPtrValue(summary.Category, ""), "imported_hypamemory_summary"),
		TimeAnchorQuality:        "summary_level",
		KeepReason:               "Imported HypaMemory should remain retrievable as long-term continuity support.",
		EntityRelevance:          tags,
		Source:                   "fallback_floor",
	}
}

func applyHypaMemoryImportScore(extraction map[string]any, score hypaMemoryImportScore) {
	if extraction == nil {
		return
	}
	current := clampFloat(extractionFloatFromAny(extraction["importance_score"], 3), 1, 10)
	next := current
	if score.Importance10 > next {
		next = score.Importance10
	}
	if next < 5 {
		next = 5
	}
	extraction["importance_score"] = next
	extraction["hypamemory_import_score"] = score.mapValue()
}

func boolPtrValue(v *bool, fallback bool) bool {
	if v == nil {
		return fallback
	}
	return *v
}

func roundHypaImportScore(v float64) float64 {
	return math.Round(v*100) / 100
}

func hypaImportTurnIndex(summary dto.HypaImportSummary, idx int) int {
	if summary.SourceTurnIndex != nil && *summary.SourceTurnIndex != 0 {
		n := *summary.SourceTurnIndex
		if n < 0 {
			return n
		}
		return -n
	}
	return -(idx + 1)
}
