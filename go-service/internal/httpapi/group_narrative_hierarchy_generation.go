package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/risulongmemory/archive-center-go/internal/dto"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

func deterministicChapterSummaryForRange(sid string, fromTurn, toTurn, chapterIndex int, episodes []store.EpisodeSummary) store.ChapterSummary {
	return store.ChapterSummary{
		ChatSessionID:           sid,
		FromTurn:                fromTurn,
		ToTurn:                  toTurn,
		ChapterIndex:            chapterIndex,
		ChapterTitle:            fmt.Sprintf("Chapter %d", chapterIndex),
		SummaryText:             deterministicChapterSummaryText(episodes),
		OpenLoopsJSON:           deterministicChapterOpenLoopsJSON(episodes),
		RelationshipChangesJSON: deterministicChapterRelationshipChangesJSON(episodes),
		WorldChangesJSON:        deterministicChapterWorldChangesJSON(episodes),
		CallbackCandidatesJSON:  deterministicChapterCallbacksJSON(episodes),
		ResumeText:              deterministicChapterResumeText(episodes),
		EmbeddingVector:         "[]",
		EmbeddingModel:          "none",
	}
}

func (s *Server) buildChapterSummaryForRange(ctx context.Context, sid string, fromTurn, toTurn, chapterIndex int, episodes []store.EpisodeSummary) (store.ChapterSummary, map[string]any) {
	deterministic := deterministicChapterSummaryForRange(sid, fromTurn, toTurn, chapterIndex, episodes)
	trace := map[string]any{
		"generation_source": "deterministic_migration_stub",
		"llm_attempted":     false,
		"llm_error":         nil,
		"llm_trace":         nil,
		"chapter_dense_summary_injection_policy_version": chapterDenseSummaryPolicyVersion,
	}

	cfg := s.chapterLLMConfig()
	if !cfg.hasConfig() {
		trace["chapter_shadow_compare"] = chapterShadowCompare(deterministic, deterministic, false, "deterministic_migration_stub")
		return deterministic, trace
	}

	trace["llm_attempted"] = true
	llmChapter, llmTrace, err := s.callChapterSummaryLLM(ctx, sid, fromTurn, toTurn, chapterIndex, episodes, cfg)
	if err != nil {
		trace["generation_source"] = "deterministic_fallback_after_llm_error"
		trace["llm_error"] = err.Error()
		trace["llm_trace"] = llmTrace
		trace["chapter_shadow_compare"] = chapterShadowCompare(deterministic, deterministic, true, "deterministic_fallback_after_llm_error")
		return deterministic, trace
	}
	trace["generation_source"] = "configured_llm"
	trace["llm_trace"] = llmTrace
	trace["chapter_shadow_compare"] = chapterShadowCompare(llmChapter, deterministic, true, "configured_llm")
	return llmChapter, trace
}

func (s *Server) callChapterSummaryLLM(ctx context.Context, sid string, fromTurn, toTurn, chapterIndex int, episodes []store.EpisodeSummary, cfg completeTurnLLMConfig) (store.ChapterSummary, map[string]any, error) {
	systemPrompt := "You generate Archive Center chapter summaries. Return only a compact JSON object with chapter_title, summary_text, open_loops, relationship_changes, world_changes, callback_candidates, and resume_text. Prefer structured episode anchors before prose summary_text in this order: open_loops, relationship_changes, world_changes, callback_candidates, resume_text, summary_text. Keep facts grounded in the provided episode summaries."
	episodePayload := episodeInputPreviews(episodes, 12)
	payload := map[string]any{
		"chat_session_id": sid,
		"from_turn":       fromTurn,
		"to_turn":         toTurn,
		"chapter_index":   chapterIndex,
		"chapter_dense_summary_injection_policy_version": chapterDenseSummaryPolicyVersion,
		"episodes": episodePayload,
	}
	payloadBytes, _ := json.Marshal(payload)
	userPrompt := "Create one chapter summary JSON for this range:\n" + string(payloadBytes)
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1400
	}
	maxCompletionTokens := cfg.MaxCompletionTokens
	if maxCompletionTokens <= 0 {
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
	applyProxyOverridesFromLLMConfig(&req, cfg)
	upstream, _, err := performProxyPluginMain(ctx, req)
	if err != nil {
		return store.ChapterSummary{}, map[string]any{
			"configured":    true,
			"endpoint_host": endpointHost(cfg.Endpoint),
			"model":         cfg.Model,
		}, err
	}
	content := chatCompletionText(upstream)
	parsed, err := parseJSONFromLLMContent(content)
	if err != nil {
		return store.ChapterSummary{}, map[string]any{
			"configured":    true,
			"endpoint_host": endpointHost(cfg.Endpoint),
			"model":         cfg.Model,
			"raw_preview":   truncateRunes(content, 1000),
		}, err
	}
	chapter := chapterSummaryFromLLMJSON(sid, fromTurn, toTurn, chapterIndex, parsed, episodes)
	trace := map[string]any{
		"configured":    true,
		"endpoint_host": endpointHost(cfg.Endpoint),
		"model":         extractionFirstNonEmpty(extractionStringFromAny(upstream["model"]), cfg.Model),
		"usage":         upstream["usage"],
		"chapter_dense_summary_injection_policy_version": chapterDenseSummaryPolicyVersion,
	}
	return chapter, trace, nil
}

func chapterSummaryFromLLMJSON(sid string, fromTurn, toTurn, chapterIndex int, parsed map[string]any, episodes []store.EpisodeSummary) store.ChapterSummary {
	fallback := deterministicChapterSummaryForRange(sid, fromTurn, toTurn, chapterIndex, episodes)
	title := extractionFirstNonEmpty(
		extractionStringFromAny(parsed["chapter_title"]),
		extractionStringFromAny(parsed["title"]),
		fallback.ChapterTitle,
	)
	summary := extractionFirstNonEmpty(
		extractionStringFromAny(parsed["summary_text"]),
		extractionStringFromAny(parsed["summary"]),
		fallback.SummaryText,
	)
	resume := extractionFirstNonEmpty(
		extractionStringFromAny(parsed["resume_text"]),
		extractionStringFromAny(parsed["resume"]),
		fallback.ResumeText,
	)
	return store.ChapterSummary{
		ChatSessionID:           sid,
		FromTurn:                fromTurn,
		ToTurn:                  toTurn,
		ChapterIndex:            chapterIndex,
		ChapterTitle:            title,
		SummaryText:             summary,
		OpenLoopsJSON:           compactChapterJSONField(parsed, fallback.OpenLoopsJSON, "open_loops", "openLoops"),
		RelationshipChangesJSON: compactChapterJSONField(parsed, fallback.RelationshipChangesJSON, "relationship_changes", "relationshipChanges"),
		WorldChangesJSON:        compactChapterJSONField(parsed, fallback.WorldChangesJSON, "world_changes", "worldChanges"),
		CallbackCandidatesJSON:  compactChapterJSONField(parsed, fallback.CallbackCandidatesJSON, "callback_candidates", "callbackCandidates"),
		ResumeText:              resume,
		EmbeddingVector:         "[]",
		EmbeddingModel:          "none",
	}
}

func compactChapterJSONField(parsed map[string]any, fallback string, keys ...string) string {
	for _, key := range keys {
		value, ok := parsed[key]
		if !ok || value == nil {
			continue
		}
		if raw := strings.TrimSpace(extractionStringFromAny(value)); raw != "" {
			var decoded any
			if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
				if data, err := json.Marshal(decoded); err == nil {
					return string(data)
				}
			}
			if data, err := json.Marshal([]string{raw}); err == nil {
				return string(data)
			}
		}
		if data, err := json.Marshal(value); err == nil {
			return string(data)
		}
	}
	if strings.TrimSpace(fallback) == "" {
		return "[]"
	}
	return fallback
}

func chapterShadowCompare(selected store.ChapterSummary, deterministic store.ChapterSummary, llmAttempted bool, source string) map[string]any {
	return map[string]any{
		"enabled":                     true,
		"selected_source":             source,
		"llm_attempted":               llmAttempted,
		"deterministic_summary_chars": utf8.RuneCountInString(deterministic.SummaryText),
		"selected_summary_chars":      utf8.RuneCountInString(selected.SummaryText),
		"deterministic_resume_chars":  utf8.RuneCountInString(deterministic.ResumeText),
		"selected_resume_chars":       utf8.RuneCountInString(selected.ResumeText),
		"summary_diverged":            selected.SummaryText != deterministic.SummaryText,
		"resume_diverged":             selected.ResumeText != deterministic.ResumeText,
	}
}

func deterministicArcSummaryForRange(sid string, fromTurn, toTurn, arcIndex int, chapters []store.ChapterSummary) store.ArcSummary {
	summaryParts := []string{}
	turningPoints := []string{}
	activePromises := []string{}
	unresolvedDebts := []string{}
	callbacks := []string{}
	futurePayoffs := []string{}
	irreversibleTurns := []string{}
	callbackDebts := []string{}
	relationshipPivots := []string{}
	for _, ch := range chapters {
		openLoops := denseJSONItems(ch.OpenLoopsJSON, 8)
		relationshipChanges := denseJSONItems(ch.RelationshipChangesJSON, 8)
		worldChanges := denseJSONItems(ch.WorldChangesJSON, 8)
		chapterCallbacks := denseJSONItems(ch.CallbackCandidatesJSON, 8)
		summaryParts = append(summaryParts, chapterDensePriorityLines(ch, 8)...)
		turningPoints = append(turningPoints, worldChanges...)
		turningPoints = append(turningPoints, denseJSONItems(ch.ResumeText, 3)...)
		activePromises = append(activePromises, relationshipChanges...)
		unresolvedDebts = append(unresolvedDebts, openLoops...)
		unresolvedDebts = append(unresolvedDebts, chapterCallbacks...)
		callbacks = append(callbacks, chapterCallbacks...)
		callbacks = append(callbacks, openLoops...)
		futurePayoffs = append(futurePayoffs, openLoops...)
		futurePayoffs = append(futurePayoffs, chapterCallbacks...)
		irreversibleTurns = append(irreversibleTurns, worldChanges...)
		if strings.TrimSpace(ch.ResumeText) != "" {
			irreversibleTurns = append(irreversibleTurns, truncateRunes(ch.ResumeText, 180))
		}
		callbackDebts = append(callbackDebts, openLoops...)
		callbackDebts = append(callbackDebts, chapterCallbacks...)
		relationshipPivots = append(relationshipPivots, relationshipChanges...)
	}
	core := strings.Join(summaryParts, " ")
	if core == "" {
		core = "Arc summary pending richer chapter material."
	}
	return store.ArcSummary{
		ChatSessionID:              sid,
		FromTurn:                   fromTurn,
		ToTurn:                     toTurn,
		ArcIndex:                   arcIndex,
		ArcName:                    fmt.Sprintf("Arc %d", arcIndex),
		ArcStatus:                  "active",
		CoreConflict:               truncateRunes(core, 600),
		KeyTurningPointsJSON:       denseJSONFromItems(turningPoints, 12),
		ActivePromisesJSON:         denseJSONFromItems(activePromises, 12),
		UnresolvedDebtsJSON:        denseJSONFromItems(unresolvedDebts, 12),
		ResolvedPayoffsJSON:        "[]",
		CallbackCandidatesJSON:     denseJSONFromItems(callbacks, 12),
		FuturePayoffCandidatesJSON: denseJSONFromItems(futurePayoffs, 12),
		IrreversibleTurnsJSON:      denseJSONFromItems(irreversibleTurns, 12),
		CallbackDebtsJSON:          denseJSONFromItems(callbackDebts, 12),
		RelationshipPivotsJSON:     denseJSONFromItems(relationshipPivots, 12),
		ArcResumeText:              fmt.Sprintf("Turns %d-%d: %s", fromTurn, toTurn, truncateRunes(core, 420)),
		EmbeddingVector:            "[]",
		EmbeddingModel:             "none",
	}
}

func (s *Server) buildArcSummaryForRange(ctx context.Context, sid string, fromTurn, toTurn, arcIndex int, chapters []store.ChapterSummary) (store.ArcSummary, map[string]any) {
	deterministic := deterministicArcSummaryForRange(sid, fromTurn, toTurn, arcIndex, chapters)
	trace := map[string]any{
		"generation_source": "deterministic_migration_stub",
		"llm_attempted":     false,
		"llm_error":         nil,
		"llm_trace":         nil,
		"status_reason":     "deterministic_default_active",
		"chapter_dense_summary_injection_policy_version": chapterDenseSummaryPolicyVersion,
		"arc_dense_summary_policy_version":               arcDenseSummaryPolicyVersion,
	}
	cfg := s.chapterLLMConfig()
	if !cfg.hasConfig() {
		trace["shadow_compare"] = arcShadowCompare(deterministic, deterministic, false, "deterministic_migration_stub")
		return deterministic, trace
	}
	trace["llm_attempted"] = true
	parsed, llmTrace, err := s.callHierarchySummaryLLM(ctx, "arc", sid, fromTurn, toTurn, chapters, cfg)
	if err != nil {
		trace["generation_source"] = "deterministic_fallback_after_llm_error"
		trace["llm_error"] = err.Error()
		trace["llm_trace"] = llmTrace
		trace["shadow_compare"] = arcShadowCompare(deterministic, deterministic, true, "deterministic_fallback_after_llm_error")
		return deterministic, trace
	}
	arc := arcSummaryFromLLMJSON(sid, fromTurn, toTurn, arcIndex, parsed, chapters)
	trace["generation_source"] = "configured_llm"
	trace["llm_trace"] = llmTrace
	trace["status_reason"] = "configured_llm_normalized"
	trace["shadow_compare"] = arcShadowCompare(arc, deterministic, true, "configured_llm")
	return arc, trace
}

func arcSummaryFromLLMJSON(sid string, fromTurn, toTurn, arcIndex int, parsed map[string]any, chapters []store.ChapterSummary) store.ArcSummary {
	fallback := deterministicArcSummaryForRange(sid, fromTurn, toTurn, arcIndex, chapters)
	status := strings.ToLower(extractionFirstNonEmpty(extractionStringFromAny(parsed["arc_status"]), fallback.ArcStatus))
	if status != "active" && status != "paused" && status != "resolved" {
		status = "active"
	}
	if status == "resolved" {
		if compactChapterJSONField(parsed, "[]", "active_promises", "activePromises") != "[]" ||
			compactChapterJSONField(parsed, "[]", "unresolved_debts", "unresolvedDebts") != "[]" {
			status = "active"
		}
	}
	return store.ArcSummary{
		ChatSessionID:              sid,
		FromTurn:                   fromTurn,
		ToTurn:                     toTurn,
		ArcIndex:                   arcIndex,
		ArcName:                    extractionFirstNonEmpty(extractionStringFromAny(parsed["arc_name"]), extractionStringFromAny(parsed["name"]), fallback.ArcName),
		ArcStatus:                  status,
		CoreConflict:               extractionFirstNonEmpty(extractionStringFromAny(parsed["core_conflict"]), fallback.CoreConflict),
		KeyTurningPointsJSON:       compactChapterJSONField(parsed, fallback.KeyTurningPointsJSON, "key_turning_points", "keyTurningPoints"),
		ActivePromisesJSON:         compactChapterJSONField(parsed, fallback.ActivePromisesJSON, "active_promises", "activePromises"),
		UnresolvedDebtsJSON:        compactChapterJSONField(parsed, fallback.UnresolvedDebtsJSON, "unresolved_debts", "unresolvedDebts"),
		ResolvedPayoffsJSON:        compactChapterJSONField(parsed, fallback.ResolvedPayoffsJSON, "resolved_payoffs", "resolvedPayoffs"),
		CallbackCandidatesJSON:     compactChapterJSONField(parsed, fallback.CallbackCandidatesJSON, "callback_candidates", "callbackCandidates"),
		FuturePayoffCandidatesJSON: compactChapterJSONField(parsed, fallback.FuturePayoffCandidatesJSON, "future_payoff_candidates", "futurePayoffCandidates"),
		IrreversibleTurnsJSON:      compactChapterJSONField(parsed, fallback.IrreversibleTurnsJSON, "irreversible_turns", "irreversibleTurns"),
		CallbackDebtsJSON:          compactChapterJSONField(parsed, fallback.CallbackDebtsJSON, "callback_debts", "callbackDebts"),
		RelationshipPivotsJSON:     compactChapterJSONField(parsed, fallback.RelationshipPivotsJSON, "relationship_pivots", "relationshipPivots"),
		ArcResumeText:              extractionFirstNonEmpty(extractionStringFromAny(parsed["arc_resume_text"]), extractionStringFromAny(parsed["resume_text"]), fallback.ArcResumeText),
		EmbeddingVector:            "[]",
		EmbeddingModel:             "none",
	}
}

func deterministicSagaDigestForRange(sid string, fromTurn, toTurn int, arcs []store.ArcSummary) store.SagaDigest {
	parts := []string{}
	neverDrop := []string{}
	for _, arc := range arcs {
		parts = append(parts, arcDensePriorityLines(arc, 12)...)
		neverDrop = append(neverDrop, denseJSONItems(arc.CallbackDebtsJSON, 6)...)
		neverDrop = append(neverDrop, denseJSONItems(arc.CallbackCandidatesJSON, 6)...)
		neverDrop = append(neverDrop, denseJSONItems(arc.RelationshipPivotsJSON, 6)...)
	}
	summary := strings.Join(parts, " ")
	if summary == "" {
		summary = "Saga digest pending richer arc material."
	}
	return store.SagaDigest{
		ChatSessionID:           sid,
		FromTurn:                fromTurn,
		ToTurn:                  toTurn,
		EraLabel:                fmt.Sprintf("Era %d-%d", fromTurn, toTurn),
		SagaSummary:             truncateRunes(summary, 900),
		PersistentFactsJSON:     "[]",
		NeverDropCandidatesJSON: denseJSONFromItems(neverDrop, 18),
		ResumePackText:          fmt.Sprintf("Turns %d-%d: %s", fromTurn, toTurn, truncateRunes(summary, 520)),
		EmbeddingVector:         "[]",
		EmbeddingModel:          "none",
	}
}

func (s *Server) buildSagaDigestForRange(ctx context.Context, sid string, fromTurn, toTurn int, arcs []store.ArcSummary) (store.SagaDigest, map[string]any) {
	deterministic := deterministicSagaDigestForRange(sid, fromTurn, toTurn, arcs)
	trace := map[string]any{
		"generation_source": "deterministic_migration_stub",
		"llm_attempted":     false,
		"llm_error":         nil,
		"llm_trace":         nil,
	}
	cfg := s.chapterLLMConfig()
	if !cfg.hasConfig() {
		trace["shadow_compare"] = sagaShadowCompare(deterministic, deterministic, false, "deterministic_migration_stub")
		return deterministic, trace
	}
	trace["llm_attempted"] = true
	parsed, llmTrace, err := s.callHierarchySummaryLLM(ctx, "saga", sid, fromTurn, toTurn, arcs, cfg)
	if err != nil {
		trace["generation_source"] = "deterministic_fallback_after_llm_error"
		trace["llm_error"] = err.Error()
		trace["llm_trace"] = llmTrace
		trace["shadow_compare"] = sagaShadowCompare(deterministic, deterministic, true, "deterministic_fallback_after_llm_error")
		return deterministic, trace
	}
	saga := sagaDigestFromLLMJSON(sid, fromTurn, toTurn, parsed, arcs)
	trace["generation_source"] = "configured_llm"
	trace["llm_trace"] = llmTrace
	trace["shadow_compare"] = sagaShadowCompare(saga, deterministic, true, "configured_llm")
	return saga, trace
}

func sagaDigestFromLLMJSON(sid string, fromTurn, toTurn int, parsed map[string]any, arcs []store.ArcSummary) store.SagaDigest {
	fallback := deterministicSagaDigestForRange(sid, fromTurn, toTurn, arcs)
	return store.SagaDigest{
		ChatSessionID:           sid,
		FromTurn:                fromTurn,
		ToTurn:                  toTurn,
		EraLabel:                extractionFirstNonEmpty(extractionStringFromAny(parsed["era_label"]), extractionStringFromAny(parsed["label"]), fallback.EraLabel),
		SagaSummary:             extractionFirstNonEmpty(extractionStringFromAny(parsed["saga_summary"]), extractionStringFromAny(parsed["summary"]), fallback.SagaSummary),
		PersistentFactsJSON:     compactChapterJSONField(parsed, fallback.PersistentFactsJSON, "persistent_facts", "persistentFacts"),
		NeverDropCandidatesJSON: compactChapterJSONField(parsed, fallback.NeverDropCandidatesJSON, "never_drop_candidates", "neverDropCandidates"),
		ResumePackText:          extractionFirstNonEmpty(extractionStringFromAny(parsed["resume_pack_text"]), extractionStringFromAny(parsed["resume_text"]), fallback.ResumePackText),
		EmbeddingVector:         "[]",
		EmbeddingModel:          "none",
	}
}

func (s *Server) callHierarchySummaryLLM(ctx context.Context, kind string, sid string, fromTurn, toTurn int, inputs any, cfg completeTurnLLMConfig) (map[string]any, map[string]any, error) {
	systemPrompt := "You generate Archive Center " + kind + " summaries. Return only compact JSON. Ground the result in the provided hierarchy inputs and do not invent unrelated facts."
	switch kind {
	case "arc":
		systemPrompt = "You generate Archive Center arc summaries. Return only compact JSON. Preserve chapter dense anchors before prose in this order: open_loops, relationship_changes, world_changes, callback_candidates, resume_text, summary_text. Include irreversible_turns, callback_debts, and relationship_pivots when supported by the input."
	case "saga":
		systemPrompt = "You generate Archive Center saga digests. Return only compact JSON. Preserve arc dense anchors before prose in this order: irreversible_turns, callback_debts, relationship_pivots, promises, debts, callbacks, resume_text, core_conflict."
	}
	payload := map[string]any{
		"chat_session_id": sid,
		"from_turn":       fromTurn,
		"to_turn":         toTurn,
		"inputs":          inputs,
	}
	if kind == "arc" {
		payload["chapter_dense_summary_injection_policy_version"] = chapterDenseSummaryPolicyVersion
		payload["arc_dense_summary_policy_version"] = arcDenseSummaryPolicyVersion
	}
	if kind == "saga" {
		payload["arc_dense_summary_policy_version"] = arcDenseSummaryPolicyVersion
	}
	payloadBytes, _ := json.Marshal(payload)
	userPrompt := "Create one " + kind + " summary JSON for this range:\n" + string(payloadBytes)
	maxTokens := cfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1400
	}
	maxCompletionTokens := cfg.MaxCompletionTokens
	if maxCompletionTokens <= 0 {
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
	applyProxyOverridesFromLLMConfig(&req, cfg)
	upstream, _, err := performProxyPluginMain(ctx, req)
	if err != nil {
		return nil, map[string]any{"configured": true, "endpoint_host": endpointHost(cfg.Endpoint), "model": cfg.Model}, err
	}
	content := chatCompletionText(upstream)
	parsed, err := parseJSONFromLLMContent(content)
	if err != nil {
		return nil, map[string]any{"configured": true, "endpoint_host": endpointHost(cfg.Endpoint), "model": cfg.Model, "raw_preview": truncateRunes(content, 1000)}, err
	}
	return parsed, map[string]any{"configured": true, "endpoint_host": endpointHost(cfg.Endpoint), "model": extractionFirstNonEmpty(extractionStringFromAny(upstream["model"]), cfg.Model), "usage": upstream["usage"]}, nil
}

func arcShadowCompare(selected store.ArcSummary, deterministic store.ArcSummary, llmAttempted bool, source string) map[string]any {
	return map[string]any{
		"enabled":                    true,
		"selected_source":            source,
		"llm_attempted":              llmAttempted,
		"deterministic_resume_chars": utf8.RuneCountInString(deterministic.ArcResumeText),
		"selected_resume_chars":      utf8.RuneCountInString(selected.ArcResumeText),
		"resume_diverged":            selected.ArcResumeText != deterministic.ArcResumeText,
		"status_diverged":            selected.ArcStatus != deterministic.ArcStatus,
	}
}

func sagaShadowCompare(selected store.SagaDigest, deterministic store.SagaDigest, llmAttempted bool, source string) map[string]any {
	return map[string]any{
		"enabled":                    true,
		"selected_source":            source,
		"llm_attempted":              llmAttempted,
		"deterministic_resume_chars": utf8.RuneCountInString(deterministic.ResumePackText),
		"selected_resume_chars":      utf8.RuneCountInString(selected.ResumePackText),
		"resume_diverged":            selected.ResumePackText != deterministic.ResumePackText,
	}
}

func deterministicChapterSummaryText(episodes []store.EpisodeSummary) string {
	parts := make([]string, 0, len(episodes))
	for _, ep := range episodes {
		parts = append(parts, episodeDensePriorityLines(ep, 5)...)
	}
	if len(parts) == 0 {
		return "Chapter summary pending richer episode material."
	}
	return strings.Join(parts, " ")
}

func deterministicChapterResumeText(episodes []store.EpisodeSummary) string {
	if len(episodes) == 0 {
		return ""
	}
	first := episodes[0]
	last := episodes[len(episodes)-1]
	summary := deterministicChapterSummaryText(episodes)
	return fmt.Sprintf("Turns %d-%d: %s", first.FromTurn, last.ToTurn, truncateRunes(summary, 360))
}

func deterministicChapterCallbacksJSON(episodes []store.EpisodeSummary) string {
	callbacks := []string{}
	for _, ep := range episodes {
		callbacks = append(callbacks, denseJSONItems(ep.OpenLoopsJSON, 4)...)
		callbacks = append(callbacks, denseJSONItems(ep.KeyEvents, 2)...)
		callbacks = append(callbacks, denseJSONItems(ep.KeyEntities, 2)...)
		if len(callbacks) >= 8 {
			break
		}
	}
	return denseJSONFromItems(callbacks, 8)
}

func deterministicChapterOpenLoopsJSON(episodes []store.EpisodeSummary) string {
	items := []string{}
	for _, ep := range episodes {
		items = append(items, denseJSONItems(ep.OpenLoopsJSON, 5)...)
	}
	return denseJSONFromItems(items, 12)
}

func deterministicChapterRelationshipChangesJSON(episodes []store.EpisodeSummary) string {
	items := []string{}
	for _, ep := range episodes {
		items = append(items, denseJSONItems(ep.RelationshipChangesJSON, 5)...)
	}
	return denseJSONFromItems(items, 12)
}

func deterministicChapterWorldChangesJSON(episodes []store.EpisodeSummary) string {
	items := []string{}
	for _, ep := range episodes {
		for _, item := range denseJSONItems(ep.KeyEvents, 5) {
			if containsWorldSignal(item) {
				items = appendDenseUnique(items, item, 12)
			}
		}
		for _, item := range denseJSONItems(ep.OpenLoopsJSON, 5) {
			if containsWorldSignal(item) {
				items = appendDenseUnique(items, item, 12)
			}
		}
	}
	return denseJSONFromItems(items, 12)
}

func episodeDensePriorityLines(ep store.EpisodeSummary, limit int) []string {
	lines := []string{}
	lines = append(lines, denseLabeledLines("open_loop", denseJSONItems(ep.OpenLoopsJSON, 4), limit)...)
	lines = append(lines, denseLabeledLines("relationship", denseJSONItems(ep.RelationshipChangesJSON, 4), limit)...)
	for _, event := range denseJSONItems(ep.KeyEvents, 4) {
		label := "callback"
		if containsWorldSignal(event) {
			label = "world"
		}
		lines = appendDenseUnique(lines, fmt.Sprintf("%s: %s", label, event), limit)
	}
	if len(lines) < limit {
		for _, item := range denseJSONItems(ep.SummaryText, 2) {
			lines = appendDenseUnique(lines, "summary: "+item, limit)
		}
	}
	return lines
}

func chapterDensePriorityLines(ch store.ChapterSummary, limit int) []string {
	lines := []string{}
	lines = append(lines, denseLabeledLines("open_loop", denseJSONItems(ch.OpenLoopsJSON, 4), limit)...)
	lines = append(lines, denseLabeledLines("relationship", denseJSONItems(ch.RelationshipChangesJSON, 4), limit)...)
	lines = append(lines, denseLabeledLines("world", denseJSONItems(ch.WorldChangesJSON, 4), limit)...)
	lines = append(lines, denseLabeledLines("callback", denseJSONItems(ch.CallbackCandidatesJSON, 4), limit)...)
	if len(lines) < limit && strings.TrimSpace(ch.ResumeText) != "" {
		lines = appendDenseUnique(lines, "resume: "+truncateRunes(ch.ResumeText, 180), limit)
	}
	if len(lines) < limit && strings.TrimSpace(ch.SummaryText) != "" {
		lines = appendDenseUnique(lines, "summary: "+truncateRunes(ch.SummaryText, 180), limit)
	}
	return lines
}

func arcDensePriorityLines(arc store.ArcSummary, limit int) []string {
	lines := []string{}
	lines = append(lines, denseLabeledLines("irreversible", denseJSONItems(arc.IrreversibleTurnsJSON, 4), limit)...)
	lines = append(lines, denseLabeledLines("callback_debt", denseJSONItems(arc.CallbackDebtsJSON, 4), limit)...)
	lines = append(lines, denseLabeledLines("relationship_pivot", denseJSONItems(arc.RelationshipPivotsJSON, 4), limit)...)
	lines = append(lines, denseLabeledLines("promise", denseJSONItems(arc.ActivePromisesJSON, 3), limit)...)
	lines = append(lines, denseLabeledLines("debt", denseJSONItems(arc.UnresolvedDebtsJSON, 3), limit)...)
	lines = append(lines, denseLabeledLines("callback", denseJSONItems(arc.CallbackCandidatesJSON, 3), limit)...)
	if len(lines) < limit && strings.TrimSpace(arc.ArcResumeText) != "" {
		lines = appendDenseUnique(lines, "resume: "+truncateRunes(arc.ArcResumeText, 220), limit)
	}
	if len(lines) < limit && strings.TrimSpace(arc.CoreConflict) != "" {
		lines = appendDenseUnique(lines, "core: "+truncateRunes(arc.CoreConflict, 220), limit)
	}
	return lines
}

func chapterDenseInputStats(episodes []store.EpisodeSummary) map[string]any {
	openLoops, relationshipChanges, worldChanges, callbacks := 0, 0, 0, 0
	for _, ep := range episodes {
		openLoops += len(denseJSONItems(ep.OpenLoopsJSON, 100))
		relationshipChanges += len(denseJSONItems(ep.RelationshipChangesJSON, 100))
		for _, item := range denseJSONItems(ep.KeyEvents, 100) {
			if containsWorldSignal(item) {
				worldChanges++
			}
			callbacks++
		}
	}
	return map[string]any{
		"episode_count": len(episodes),
		"chapter_dense_summary_injection_policy_version": chapterDenseSummaryPolicyVersion,
		"anchor_priority":                   []string{"open_loops", "relationship_changes", "world_changes", "callback_candidates", "resume_text", "summary_text"},
		"episode_open_loop_anchor_count":    openLoops,
		"episode_relationship_anchor_count": relationshipChanges,
		"episode_world_anchor_count":        worldChanges,
		"episode_callback_anchor_count":     callbacks,
	}
}

func arcDenseInputStats(chapters []store.ChapterSummary, fromTurn, toTurn int) map[string]any {
	openLoops, relationshipChanges, worldChanges, callbacks := 0, 0, 0, 0
	for _, ch := range chapters {
		openLoops += len(denseJSONItems(ch.OpenLoopsJSON, 100))
		relationshipChanges += len(denseJSONItems(ch.RelationshipChangesJSON, 100))
		worldChanges += len(denseJSONItems(ch.WorldChangesJSON, 100))
		callbacks += len(denseJSONItems(ch.CallbackCandidatesJSON, 100))
	}
	return map[string]any{
		"chapter_count":             len(chapters),
		"chapter_count_recommended": len(chapters) >= 3 && len(chapters) <= 6,
		"turn_span":                 (toTurn - fromTurn) + 1,
		"chapter_dense_summary_injection_policy_version": chapterDenseSummaryPolicyVersion,
		"arc_dense_summary_policy_version":               arcDenseSummaryPolicyVersion,
		"semantic_field_mapping": map[string]any{
			"irreversible_turns_json":  "world_changes_json + resume_text anchors",
			"callback_debts_json":      "open_loops_json + callback_candidates_json",
			"relationship_pivots_json": "relationship_changes_json",
		},
		"chapter_open_loop_anchor_count":    openLoops,
		"chapter_relationship_anchor_count": relationshipChanges,
		"chapter_world_anchor_count":        worldChanges,
		"chapter_callback_anchor_count":     callbacks,
	}
}

func sagaDenseInputStats(arcs []store.ArcSummary, fromTurn, toTurn int) map[string]any {
	irreversible, debts, pivots := 0, 0, 0
	for _, arc := range arcs {
		irreversible += len(denseJSONItems(arc.IrreversibleTurnsJSON, 100))
		debts += len(denseJSONItems(arc.CallbackDebtsJSON, 100))
		pivots += len(denseJSONItems(arc.RelationshipPivotsJSON, 100))
	}
	return map[string]any{
		"arc_count":                        len(arcs),
		"arc_count_recommended":            len(arcs) >= 2 && len(arcs) <= 6,
		"turn_span":                        (toTurn - fromTurn) + 1,
		"arc_dense_summary_policy_version": arcDenseSummaryPolicyVersion,
		"arc_irreversible_anchor_count":    irreversible,
		"arc_callback_debt_count":          debts,
		"arc_relationship_pivot_count":     pivots,
		"saga_input_priority":              []string{"irreversible_turns", "callback_debts", "relationship_pivots", "promises", "debts", "callbacks", "resume_text", "core_conflict"},
	}
}

func chapterDensePriorityScores(ch store.ChapterSummary) map[string]int {
	relationshipScore := len(denseJSONItems(ch.RelationshipChangesJSON, 100))
	worldScore := len(denseJSONItems(ch.WorldChangesJSON, 100))
	importanceScore := len(denseJSONItems(ch.OpenLoopsJSON, 100)) + len(denseJSONItems(ch.CallbackCandidatesJSON, 100))
	if strings.TrimSpace(ch.ResumeText) != "" {
		importanceScore++
	}
	priorityScore := relationshipScore*4 + worldScore*4 + importanceScore*2
	return map[string]int{
		"dense_priority_score":     priorityScore,
		"dense_importance_score":   importanceScore,
		"dense_relationship_score": relationshipScore,
		"dense_world_score":        worldScore,
	}
}

func episodeDensePriorityScores(ep store.EpisodeSummary) map[string]int {
	relationshipScore := len(denseJSONItems(ep.RelationshipChangesJSON, 100))
	worldScore := 0
	for _, item := range denseJSONItems(ep.KeyEvents, 100) {
		if containsWorldSignal(item) {
			worldScore++
		}
	}
	importanceScore := len(denseJSONItems(ep.OpenLoopsJSON, 100)) + len(denseJSONItems(ep.KeyEvents, 100))
	if strings.TrimSpace(ep.SummaryText) != "" {
		importanceScore++
	}
	priorityScore := relationshipScore*4 + worldScore*4 + importanceScore*2
	return map[string]int{
		"dense_priority_score":     priorityScore,
		"dense_importance_score":   importanceScore,
		"dense_relationship_score": relationshipScore,
		"dense_world_score":        worldScore,
	}
}

func sortChapterSummariesByDensePriority(chapters []store.ChapterSummary) {
	sort.SliceStable(chapters, func(i, j int) bool {
		left := chapterDensePriorityScores(chapters[i])["dense_priority_score"]
		right := chapterDensePriorityScores(chapters[j])["dense_priority_score"]
		if left != right {
			return left > right
		}
		if chapters[i].ToTurn != chapters[j].ToTurn {
			return chapters[i].ToTurn > chapters[j].ToTurn
		}
		return chapters[i].ID > chapters[j].ID
	})
}

func sortEpisodeSummariesByDensePriority(episodes []store.EpisodeSummary) {
	sort.SliceStable(episodes, func(i, j int) bool {
		left := episodeDensePriorityScores(episodes[i])["dense_priority_score"]
		right := episodeDensePriorityScores(episodes[j])["dense_priority_score"]
		if left != right {
			return left > right
		}
		if episodes[i].ToTurn != episodes[j].ToTurn {
			return episodes[i].ToTurn > episodes[j].ToTurn
		}
		return episodes[i].ID > episodes[j].ID
	})
}

func denseSearchStoreLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	widened := limit * 4
	if widened < limit+8 {
		widened = limit + 8
	}
	if widened > 100 {
		return 100
	}
	return widened
}

func matchesChapter(ch *store.ChapterSummary, query string) bool {
	if ch == nil {
		return false
	}
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return true
	}
	text := strings.ToLower(ch.ChapterTitle + " " + ch.ResumeText + " " + ch.SummaryText)
	return strings.Contains(text, query)
}
