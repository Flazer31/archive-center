package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// Episodes: R1 read/search, R2 generate/write

type episodeSummaryIDDeleter interface {
	DeleteEpisodeSummary(ctx context.Context, episodeID int64) error
}

type episodeSummaryRangeDeleter interface {
	DeleteEpisodeSummariesInRange(ctx context.Context, chatSessionID string, fromTurn, toTurn int) (int64, error)
}

func (s *Server) handleEpisodesGet(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	fromTurn, _ := strconv.Atoi(r.URL.Query().Get("from_turn"))
	toTurn, _ := strconv.Atoi(r.URL.Query().Get("to_turn"))
	items, err := s.Store.ListEpisodeSummaries(r.Context(), sid, limit, fromTurn, toTurn)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			items = []store.EpisodeSummary{}
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	if items == nil {
		items = []store.EpisodeSummary{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"episodes":        items,
		"count":           len(items),
	})
}

func (s *Server) handleEpisodeDetail(w http.ResponseWriter, r *http.Request) {
	rawID := r.PathValue("episode_id")
	episodeID, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || episodeID <= 0 {
		writeBadRequest(w, "episode_id must be a positive integer")
		return
	}
	item, err := s.Store.GetEpisodeSummary(r.Context(), episodeID)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			item = nil
		} else if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]any{
				"status": "error",
				"detail": "episode not found",
			})
			return
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"episode_id": episodeID,
		"found":      item != nil,
		"episode":    item,
	})
}

func (s *Server) handleEpisodeGenerate(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeNarrativeSearchRequest(w, r)
	if !ok {
		return
	}
	if req.ChatSessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "detail": "chat_session_id is required"})
		return
	}
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, "POST /episodes/generate")
		return
	}
	episodeStore, ok := s.Store.(store.EpisodeSummaryStore)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "error",
			"code":   "episode_store_not_available",
			"detail": "episode summary store is not available",
		})
		return
	}
	resp, statusCode := s.generateEpisodeSummaryResponse(r.Context(), req, episodeStore, "episode_generated")
	writeJSON(w, statusCode, resp)
}

func (s *Server) generateEpisodeSummaryResponse(ctx context.Context, req narrativeSearchRequest, episodeStore store.EpisodeSummaryStore, successCode string) (map[string]any, int) {
	interval := normalizedEpisodeInterval(req.Interval)
	fromTurn, toTurn := req.normalizedTurnRange()
	if fromTurn <= 0 || toTurn <= 0 {
		fromTurn, toTurn = episodeRangeFromTurn(req.TurnIndex, interval)
	}
	if fromTurn <= 0 || toTurn <= 0 || fromTurn > toTurn {
		return map[string]any{
			"status":           "skipped",
			"code":             "episode_range_not_ready",
			"chat_session_id":  req.ChatSessionID,
			"turn_index":       req.TurnIndex,
			"interval":         interval,
			"blocking_reasons": []string{"episode_range_not_ready"},
			"llm_attempted":    false,
			"saved":            false,
		}, http.StatusOK
	}

	if !req.Force && s.Store != nil {
		if existing, err := s.Store.ListEpisodeSummaries(ctx, req.ChatSessionID, 0, fromTurn, toTurn); err == nil {
			for _, item := range existing {
				if item.FromTurn == fromTurn && item.ToTurn == toTurn {
					return map[string]any{
						"status":          "skipped",
						"code":            "episode_already_exists",
						"chat_session_id": req.ChatSessionID,
						"from_turn":       fromTurn,
						"to_turn":         toTurn,
						"interval":        interval,
						"episode":         item,
						"llm_attempted":   false,
						"saved":           false,
					}, http.StatusOK
				}
			}
		}
	}

	ev := s.collectNarrativeEvidence(ctx, req.ChatSessionID)
	chatLogs := filterChatLogsForTurnRange(ev.ChatLogs, fromTurn, toTurn, req.normalizedLimit(24))
	if len(chatLogs) == 0 {
		return map[string]any{
			"status":           "skipped",
			"code":             "no_chat_logs",
			"chat_session_id":  req.ChatSessionID,
			"from_turn":        fromTurn,
			"to_turn":          toTurn,
			"interval":         interval,
			"blocking_reasons": []string{"no_chat_logs"},
			"llm_attempted":    false,
			"saved":            false,
		}, http.StatusOK
	}

	replaced := int64(0)
	if req.Force {
		deleter, ok := s.Store.(episodeSummaryRangeDeleter)
		if !ok {
			return map[string]any{
				"status":          "error",
				"code":            "episode_range_delete_not_available",
				"chat_session_id": req.ChatSessionID,
				"from_turn":       fromTurn,
				"to_turn":         toTurn,
				"saved":           false,
			}, http.StatusServiceUnavailable
		}
		n, err := deleter.DeleteEpisodeSummariesInRange(ctx, req.ChatSessionID, fromTurn, toTurn)
		if err != nil {
			return map[string]any{
				"status":          "error",
				"code":            "episode_range_delete_failed",
				"chat_session_id": req.ChatSessionID,
				"from_turn":       fromTurn,
				"to_turn":         toTurn,
				"detail":          err.Error(),
				"saved":           false,
			}, http.StatusInternalServerError
		}
		replaced = n
	}

	episode, generationTrace := buildEpisodeSummaryForRangeWithArtifacts(req.ChatSessionID, fromTurn, toTurn, chatLogs,
		filterMemoriesForTurnRange(ev.Memories, req.ChatSessionID, fromTurn, toTurn),
		filterEvidenceForTurnRange(ev.Evidence, req.ChatSessionID, fromTurn, toTurn))
	if err := episodeStore.SaveEpisodeSummary(ctx, &episode); err != nil {
		return map[string]any{
			"status":          "error",
			"code":            "episode_summary_save_failed",
			"chat_session_id": req.ChatSessionID,
			"from_turn":       fromTurn,
			"to_turn":         toTurn,
			"detail":          err.Error(),
			"saved":           false,
		}, http.StatusInternalServerError
	}
	if successCode == "" {
		successCode = "episode_generated"
	}
	return map[string]any{
		"status":           "ok",
		"code":             successCode,
		"chat_session_id":  req.ChatSessionID,
		"from_turn":        fromTurn,
		"to_turn":          toTurn,
		"interval":         interval,
		"episode":          episode,
		"generation_trace": generationTrace,
		"llm_attempted":    false,
		"replaced":         replaced,
		"saved":            true,
	}, http.StatusOK
}

func (s *Server) handleChapterGenerate(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeNarrativeSearchRequest(w, r)
	if !ok {
		return
	}
	if req.ChatSessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "detail": "chat_session_id is required"})
		return
	}
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, "POST /chapters/generate")
		return
	}
	chapterStore, ok := s.Store.(store.ChapterSummaryStore)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "error",
			"code":   "chapter_store_not_available",
			"detail": "chapter summary store is not available",
		})
		return
	}

	interval := normalizedChapterInterval(req.Interval)
	fromTurn, toTurn := req.normalizedTurnRange()
	intervalCheck := chapterIntervalCheck(req.TurnIndex, interval)
	if fromTurn == 0 || toTurn == 0 {
		if rawRange, ok := intervalCheck["range"].([]int); ok && len(rawRange) == 2 {
			fromTurn = rawRange[0]
			toTurn = rawRange[1]
		}
	}
	if fromTurn <= 0 || toTurn <= 0 || fromTurn > toTurn {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":           "skipped",
			"chat_session_id":  req.ChatSessionID,
			"turn_index":       req.TurnIndex,
			"interval":         interval,
			"interval_check":   intervalCheck,
			"blocking_reasons": []string{"chapter_range_not_ready"},
			"llm_attempted":    false,
			"saved":            false,
		})
		return
	}

	ev := s.collectNarrativeEvidence(r.Context(), req.ChatSessionID)
	episodes := filterEpisodes(ev.EpisodeSummaries, "", fromTurn, toTurn, req.normalizedLimit(8))
	if len(episodes) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":           "skipped",
			"chat_session_id":  req.ChatSessionID,
			"from_turn":        fromTurn,
			"to_turn":          toTurn,
			"interval":         interval,
			"blocking_reasons": []string{"no_episode_summaries"},
			"llm_attempted":    false,
			"saved":            false,
		})
		return
	}

	chapterIndex := chapterIndexForRange(toTurn, interval)
	if !req.Force {
		existing, err := chapterStore.SearchChapterSummaries(r.Context(), req.ChatSessionID, "", fromTurn, toTurn, 1)
		if err == nil && len(existing) > 0 {
			for _, ec := range existing {
				if ec.FromTurn == fromTurn && ec.ToTurn == toTurn {
					writeJSON(w, http.StatusOK, map[string]any{
						"status":          "skipped",
						"chat_session_id": req.ChatSessionID,
						"from_turn":       fromTurn,
						"to_turn":         toTurn,
						"already_exists":  true,
						"chapter":         ec,
						"llm_attempted":   false,
						"saved":           false,
					})
					return
				}
			}
		}
	}
	chapter, generationTrace := s.buildChapterSummaryForRange(r.Context(), req.ChatSessionID, fromTurn, toTurn, chapterIndex, episodes)
	if err := chapterStore.SaveChapterSummary(r.Context(), &chapter); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":          "error",
			"code":            "chapter_save_failed",
			"detail":          err.Error(),
			"chat_session_id": req.ChatSessionID,
			"llm_attempted":   generationTrace["llm_attempted"],
			"saved":           false,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":                 "ok",
		"chat_session_id":        req.ChatSessionID,
		"from_turn":              fromTurn,
		"to_turn":                toTurn,
		"chapter":                chapter,
		"chapter_result":         map[string]any{"checked": true, "triggered": true, "range": map[string]any{"from_turn": fromTurn, "to_turn": toTurn}},
		"input_stats":            chapterDenseInputStats(episodes),
		"generation_source":      generationTrace["generation_source"],
		"llm_attempted":          generationTrace["llm_attempted"],
		"llm_error":              generationTrace["llm_error"],
		"chapter_llm_trace":      generationTrace["llm_trace"],
		"chapter_shadow_compare": generationTrace["chapter_shadow_compare"],
		"saved":                  true,
	})
}

func (s *Server) handleArcGenerate(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeNarrativeSearchRequest(w, r)
	if !ok {
		return
	}
	if req.ChatSessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "detail": "chat_session_id is required"})
		return
	}
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, "POST /arcs/generate")
		return
	}
	arcStore, ok := s.Store.(store.ArcSummaryStore)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "error", "code": "arc_store_not_available", "detail": "arc summary store is not available"})
		return
	}
	chapterStore, ok := s.Store.(store.ChapterSummaryStore)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "error", "code": "chapter_store_not_available", "detail": "chapter summary store is required for arc generation"})
		return
	}
	fromTurn, toTurn := req.normalizedTurnRange()
	if fromTurn <= 0 || toTurn <= 0 || fromTurn > toTurn {
		writeJSON(w, http.StatusOK, map[string]any{"status": "skipped", "chat_session_id": req.ChatSessionID, "blocking_reasons": []string{"arc_range_not_ready"}, "saved": false})
		return
	}
	existing, err := arcStore.SearchArcSummaries(r.Context(), req.ChatSessionID, "", fromTurn, toTurn, 20)
	if err != nil && !errors.Is(err, store.ErrNotEnabled) && !errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "detail": err.Error()})
		return
	}
	for _, arc := range existing {
		if arc.FromTurn == fromTurn && arc.ToTurn == toTurn && !req.Force {
			writeJSON(w, http.StatusOK, map[string]any{"status": "skipped", "chat_session_id": req.ChatSessionID, "from_turn": fromTurn, "to_turn": toTurn, "already_exists": true, "arc": arc, "saved": false})
			return
		}
	}
	chapters, err := chapterStore.SearchChapterSummaries(r.Context(), req.ChatSessionID, "", fromTurn, toTurn, req.normalizedLimit(6))
	if err != nil && !errors.Is(err, store.ErrNotEnabled) && !errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "detail": err.Error()})
		return
	}
	if len(chapters) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"status": "skipped", "chat_session_id": req.ChatSessionID, "from_turn": fromTurn, "to_turn": toTurn, "blocking_reasons": []string{"no_chapter_summaries"}, "saved": false})
		return
	}
	arcIndex := hierarchyIndexForRange(toTurn, 240)
	arc, generationTrace := s.buildArcSummaryForRange(r.Context(), req.ChatSessionID, fromTurn, toTurn, arcIndex, chapters)
	if err := arcStore.SaveArcSummary(r.Context(), req.ChatSessionID, &arc); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "code": "arc_save_failed", "detail": err.Error(), "saved": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":             "ok",
		"chat_session_id":    req.ChatSessionID,
		"from_turn":          fromTurn,
		"to_turn":            toTurn,
		"arc":                arc,
		"input_stats":        arcDenseInputStats(chapters, fromTurn, toTurn),
		"generation_source":  generationTrace["generation_source"],
		"llm_attempted":      generationTrace["llm_attempted"],
		"llm_error":          generationTrace["llm_error"],
		"arc_llm_trace":      generationTrace["llm_trace"],
		"arc_shadow_compare": generationTrace["shadow_compare"],
		"lifecycle":          map[string]any{"final_status": arc.ArcStatus, "status_reason": generationTrace["status_reason"]},
		"saved":              true,
	})
}

func (s *Server) handleSagaGenerate(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeNarrativeSearchRequest(w, r)
	if !ok {
		return
	}
	if req.ChatSessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "detail": "chat_session_id is required"})
		return
	}
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, "POST /sagas/generate")
		return
	}
	sagaStore, ok := s.Store.(store.SagaDigestStore)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "error", "code": "saga_store_not_available", "detail": "saga digest store is not available"})
		return
	}
	arcStore, ok := s.Store.(store.ArcSummaryStore)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "error", "code": "arc_store_not_available", "detail": "arc summary store is required for saga generation"})
		return
	}
	fromTurn, toTurn := req.normalizedTurnRange()
	if fromTurn <= 0 || toTurn <= 0 || fromTurn > toTurn {
		writeJSON(w, http.StatusOK, map[string]any{"status": "skipped", "chat_session_id": req.ChatSessionID, "blocking_reasons": []string{"saga_range_not_ready"}, "saved": false})
		return
	}
	existing, err := sagaStore.SearchSagaDigests(r.Context(), req.ChatSessionID, "", fromTurn, toTurn, 20)
	if err != nil && !errors.Is(err, store.ErrNotEnabled) && !errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "detail": err.Error()})
		return
	}
	for _, saga := range existing {
		if saga.FromTurn == fromTurn && saga.ToTurn == toTurn && !req.Force {
			writeJSON(w, http.StatusOK, map[string]any{"status": "skipped", "chat_session_id": req.ChatSessionID, "from_turn": fromTurn, "to_turn": toTurn, "already_exists": true, "saga": saga, "saved": false})
			return
		}
	}
	arcs, err := arcStore.SearchArcSummaries(r.Context(), req.ChatSessionID, "", fromTurn, toTurn, req.normalizedLimit(6))
	if err != nil && !errors.Is(err, store.ErrNotEnabled) && !errors.Is(err, store.ErrNotFound) {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "detail": err.Error()})
		return
	}
	if len(arcs) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"status": "skipped", "chat_session_id": req.ChatSessionID, "from_turn": fromTurn, "to_turn": toTurn, "blocking_reasons": []string{"no_arc_summaries"}, "saved": false})
		return
	}
	saga, generationTrace := s.buildSagaDigestForRange(r.Context(), req.ChatSessionID, fromTurn, toTurn, arcs)
	if err := sagaStore.SaveSagaDigest(r.Context(), req.ChatSessionID, &saga); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "code": "saga_save_failed", "detail": err.Error(), "saved": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":              "ok",
		"chat_session_id":     req.ChatSessionID,
		"from_turn":           fromTurn,
		"to_turn":             toTurn,
		"saga":                saga,
		"input_stats":         sagaDenseInputStats(arcs, fromTurn, toTurn),
		"generation_source":   generationTrace["generation_source"],
		"llm_attempted":       generationTrace["llm_attempted"],
		"llm_error":           generationTrace["llm_error"],
		"saga_llm_trace":      generationTrace["llm_trace"],
		"saga_shadow_compare": generationTrace["shadow_compare"],
		"saved":               true,
	})
}

func (s *Server) handleChapterDryRun(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeNarrativeSearchRequest(w, r)
	if !ok {
		return
	}
	if req.ChatSessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "detail": "chat_session_id is required"})
		return
	}
	if req.TurnIndex < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "detail": "turn_index must be >= 0"})
		return
	}
	ev := s.collectNarrativeEvidence(r.Context(), req.ChatSessionID)
	interval := normalizedChapterInterval(req.Interval)
	intervalCheck := chapterIntervalCheck(req.TurnIndex, interval)
	fromTurn, toTurn := 0, 0
	candidateRange := any(nil)
	blockingReasons := []string{}
	warnings := []string{}
	turnSpan := any(nil)
	if rawRange, ok := intervalCheck["range"].([]int); ok && len(rawRange) == 2 {
		fromTurn = rawRange[0]
		toTurn = rawRange[1]
		span := (toTurn - fromTurn) + 1
		turnSpan = span
		candidateRange = map[string]any{
			"from_turn": fromTurn,
			"to_turn":   toTurn,
			"turn_span": span,
		}
	} else if reason, _ := intervalCheck["reason"].(string); reason != "" {
		blockingReasons = append(blockingReasons, reason)
	}
	episodes := []store.EpisodeSummary{}
	if fromTurn > 0 || toTurn > 0 {
		episodes = filterEpisodes(ev.EpisodeSummaries, "", fromTurn, toTurn, req.normalizedLimit(8))
		if len(episodes) == 0 {
			blockingReasons = append(blockingReasons, "no_episode_summaries")
		} else if len(episodes) < 4 || len(episodes) > 8 {
			warnings = append(warnings, "episode_count_outside_recommended_window")
		}
		if span, ok := turnSpan.(int); ok && (span < 40 || span > 80) {
			warnings = append(warnings, "turn_span_outside_recommended_window")
		}
	}
	episodeInputs := episodeInputPreviews(episodes, req.normalizedLimit(8))

	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"mode":             "dry_run",
		"chat_session_id":  req.ChatSessionID,
		"turn_index":       req.TurnIndex,
		"interval":         interval,
		"force":            req.Force,
		"triggered":        candidateRange != nil,
		"interval_check":   intervalCheck,
		"candidate_range":  candidateRange,
		"already_exists":   false,
		"ready":            len(blockingReasons) == 0 && candidateRange != nil,
		"blocking_reasons": blockingReasons,
		"warnings":         warnings,
		"input_stats": map[string]any{
			"episode_count":             len(episodes),
			"episode_count_recommended": len(episodes) >= 4 && len(episodes) <= 8,
			"turn_span":                 turnSpan,
			"turn_span_recommended":     turnSpanRecommended(turnSpan),
		},
		"episode_inputs": episodeInputs,
	})
}

func (s *Server) handleChapterSearch(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeNarrativeSearchRequest(w, r)
	if !ok {
		return
	}
	if req.ChatSessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "detail": "chat_session_id is required"})
		return
	}
	if req.Query == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "detail": "query is required"})
		return
	}
	ev := s.collectNarrativeEvidence(r.Context(), req.ChatSessionID)
	fromTurn, toTurn := req.normalizedTurnRange()
	limit := req.normalizedLimit(3)
	results := []any{}
	if chapterStore, ok := s.Store.(store.ChapterSummaryStore); ok {
		chapters, err := chapterStore.SearchChapterSummaries(r.Context(), req.ChatSessionID, req.Query, fromTurn, toTurn, denseSearchStoreLimit(limit))
		if err == nil {
			sortChapterSummariesByDensePriority(chapters)
			if len(chapters) > limit {
				chapters = chapters[:limit]
			}
			results = chapterResultsWithEvidence(chapters, ev.Evidence)
		} else if !errors.Is(err, store.ErrNotEnabled) && !errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "detail": err.Error()})
			return
		}
	}
	if len(results) < limit {
		episodes := filterEpisodes(ev.EpisodeSummaries, req.Query, fromTurn, toTurn, denseSearchStoreLimit(limit-len(results)))
		sortEpisodeSummariesByDensePriority(episodes)
		if remaining := limit - len(results); len(episodes) > remaining {
			episodes = episodes[:remaining]
		}
		results = append(results, episodeResultsWithEvidence(episodes, ev.Evidence)...)
	}
	if ev.ResumePack != nil && ev.ResumePack.Chapter != nil && matchesChapter(ev.ResumePack.Chapter, req.Query) && len(results) < limit {
		item := map[string]any{
			"id":            ev.ResumePack.Chapter.ID,
			"source":        "resume_pack_chapter",
			"from_turn":     ev.ResumePack.Chapter.FromTurn,
			"to_turn":       ev.ResumePack.Chapter.ToTurn,
			"title":         ev.ResumePack.Chapter.ChapterTitle,
			"summary_text":  ev.ResumePack.Chapter.SummaryText,
			"resume_text":   ev.ResumePack.Chapter.ResumeText,
			"chapter_index": ev.ResumePack.Chapter.ChapterIndex,
		}
		for k, v := range denseSummarySurfaceFields("chapter", ev.ResumePack.Chapter.ID, ev.ResumePack.Chapter.FromTurn, ev.ResumePack.Chapter.ToTurn, q1FirstNonEmptyString(ev.ResumePack.Chapter.ResumeText, ev.ResumePack.Chapter.SummaryText, ev.ResumePack.Chapter.ChapterTitle), chapterDenseStructuredPayload(*ev.ResumePack.Chapter), chapterDensePriorityScores(*ev.ResumePack.Chapter), ev.Evidence) {
			item[k] = v
		}
		results = append(results, item)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": req.ChatSessionID,
		"query":           truncateString(req.Query, 200),
		"chapters":        results,
		"count":           len(results),
	})
}

func (s *Server) handleEpisodeSearch(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeNarrativeSearchRequest(w, r)
	if !ok {
		return
	}
	if req.ChatSessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "detail": "chat_session_id is required"})
		return
	}
	if req.Query == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "detail": "query is required"})
		return
	}
	ev := s.collectNarrativeEvidence(r.Context(), req.ChatSessionID)
	fromTurn, toTurn := req.normalizedTurnRange()
	limit := req.normalizedLimit(3)
	episodes := filterEpisodes(ev.EpisodeSummaries, req.Query, fromTurn, toTurn, denseSearchStoreLimit(limit))
	sortEpisodeSummariesByDensePriority(episodes)
	if len(episodes) > limit {
		episodes = episodes[:limit]
	}
	results := episodeResultsWithEvidence(episodes, ev.Evidence)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": req.ChatSessionID,
		"query":           truncateString(req.Query, 200),
		"episodes":        results,
		"count":           len(episodes),
	})
}

func (s *Server) handleEpisodePatch(w http.ResponseWriter, r *http.Request) {
	writeShadowGuard(w, "PATCH /episodes/{episode_id}")
}

func (s *Server) handleEpisodeDelete(w http.ResponseWriter, r *http.Request) {
	episodeID, ok := parseNarrativeInt64Path(w, r, "episode_id")
	if !ok {
		return
	}
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, "DELETE /episodes/{episode_id}")
		return
	}
	deleter, ok := s.Store.(episodeSummaryIDDeleter)
	if !ok {
		writeShadowGuard(w, "DELETE /episodes/{episode_id}")
		return
	}
	if err := deleter.DeleteEpisodeSummary(r.Context(), episodeID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("episode %d not found", episodeID))
			return
		}
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, "DELETE /episodes/{episode_id}")
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"episode_id": episodeID,
		"deleted":    true,
	})
}

func (s *Server) handleEpisodeRegenerate(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeNarrativeSearchRequest(w, r)
	if !ok {
		return
	}
	if req.ChatSessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "detail": "chat_session_id is required"})
		return
	}
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, "POST /episodes/regenerate")
		return
	}
	episodeStore, ok := s.Store.(store.EpisodeSummaryStore)
	if !ok {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status": "error",
			"code":   "episode_store_not_available",
			"detail": "episode summary store is not available",
		})
		return
	}
	req.Force = true
	resp, statusCode := s.generateEpisodeSummaryResponse(r.Context(), req, episodeStore, "episode_regenerated")
	writeJSON(w, statusCode, resp)
}

func (s *Server) handleEpisodeMerge(w http.ResponseWriter, r *http.Request) {
	writeShadowGuard(w, "POST /episodes/merge")
}
