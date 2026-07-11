package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

type narrativeSearchRequest struct {
	ChatSessionID string `json:"chat_session_id"`
	Query         string `json:"query"`
	Limit         int    `json:"limit"`
	TopK          int    `json:"top_k"`
	FromTurn      int    `json:"from_turn"`
	ToTurn        int    `json:"to_turn"`
	TurnIndex     int    `json:"turn_index"`
	Interval      int    `json:"interval"`
	Force         bool   `json:"force"`
}

func (req narrativeSearchRequest) normalizedLimit(defaultLimit ...int) int {
	fallback := 20
	if len(defaultLimit) > 0 && defaultLimit[0] > 0 {
		fallback = defaultLimit[0]
	}
	limit := req.Limit
	if limit <= 0 {
		limit = req.TopK
	}
	if limit <= 0 {
		limit = fallback
	}
	if limit > 100 {
		return 100
	}
	return limit
}

func (req narrativeSearchRequest) normalizedTurnRange() (int, int) {
	fromTurn := req.FromTurn
	toTurn := req.ToTurn
	if fromTurn < 0 {
		fromTurn = 0
	}
	if toTurn < 0 {
		toTurn = 0
	}
	if fromTurn > 0 && toTurn > 0 && fromTurn > toTurn {
		fromTurn, toTurn = toTurn, fromTurn
	}
	return fromTurn, toTurn
}

func decodeNarrativeSearchRequest(w http.ResponseWriter, r *http.Request) (narrativeSearchRequest, bool) {
	req := narrativeSearchRequest{
		ChatSessionID: strings.TrimSpace(r.URL.Query().Get("chat_session_id")),
		Query:         strings.TrimSpace(r.URL.Query().Get("query")),
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			req.Limit = parsed
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("top_k")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			req.TopK = parsed
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("from_turn")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			req.FromTurn = parsed
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("to_turn")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			req.ToTurn = parsed
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("turn_index")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			req.TurnIndex = parsed
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("interval")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			req.Interval = parsed
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("force")); raw != "" {
		if parsed, err := strconv.ParseBool(raw); err == nil {
			req.Force = parsed
		}
	}
	if r.Body != nil && r.ContentLength != 0 {
		var body narrativeSearchRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", err.Error())
			return narrativeSearchRequest{}, false
		}
		if strings.TrimSpace(body.ChatSessionID) != "" {
			req.ChatSessionID = strings.TrimSpace(body.ChatSessionID)
		}
		if strings.TrimSpace(body.Query) != "" {
			req.Query = strings.TrimSpace(body.Query)
		}
		if body.Limit != 0 {
			req.Limit = body.Limit
		}
		if body.TopK != 0 {
			req.TopK = body.TopK
		}
		if body.FromTurn != 0 {
			req.FromTurn = body.FromTurn
		}
		if body.ToTurn != 0 {
			req.ToTurn = body.ToTurn
		}
		if body.TurnIndex != 0 {
			req.TurnIndex = body.TurnIndex
		}
		if body.Interval != 0 {
			req.Interval = body.Interval
		}
		if body.Force {
			req.Force = true
		}
	}
	return req, true
}

func normalizedEpisodeInterval(interval int) int {
	if interval <= 0 {
		interval = 5
	}
	if interval < 5 {
		return 5
	}
	if interval > 60 {
		return 60
	}
	return interval
}

func episodeRangeFromTurn(turnIndex, interval int) (int, int) {
	if turnIndex <= 0 {
		return 0, 0
	}
	if interval <= 0 {
		interval = normalizedEpisodeInterval(interval)
	}
	toTurn := turnIndex
	fromTurn := toTurn - interval + 1
	if fromTurn < 1 {
		fromTurn = 1
	}
	return fromTurn, toTurn
}

func filterChatLogsForTurnRange(logs []store.ChatLog, fromTurn, toTurn, limit int) []store.ChatLog {
	out := []store.ChatLog{}
	for _, item := range logs {
		if fromTurn > 0 && item.TurnIndex < fromTurn {
			continue
		}
		if toTurn > 0 && item.TurnIndex > toTurn {
			continue
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TurnIndex == out[j].TurnIndex {
			return out[i].ID < out[j].ID
		}
		return out[i].TurnIndex < out[j].TurnIndex
	})
	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}

func buildEpisodeSummaryForRange(sid string, fromTurn, toTurn int, logs []store.ChatLog) (store.EpisodeSummary, map[string]any) {
	return buildEpisodeSummaryForRangeWithArtifacts(sid, fromTurn, toTurn, logs, nil, nil)
}

func buildEpisodeSummaryForRangeWithArtifacts(sid string, fromTurn, toTurn int, logs []store.ChatLog, memories []store.Memory, evidence []store.DirectEvidence) (store.EpisodeSummary, map[string]any) {
	lines := []string{}
	for _, mem := range memories {
		content := cleanEpisodeSourceText(memorySummaryText(mem))
		if content == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("memory: %s", truncateRunes(content, 260)))
	}
	for _, ev := range evidence {
		content := cleanEpisodeSourceText(ev.EvidenceText)
		if content == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("evidence: %s", truncateRunes(content, 220)))
	}
	for _, log := range logs {
		content := cleanEpisodeSourceText(log.Content)
		if content == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %s", firstNonEmpty(strings.TrimSpace(log.Role), "unknown"), truncateRunes(content, 220)))
	}
	keyEvents := episodeKeyEvents(lines)
	relationshipChanges := episodeRelationshipAnchors(lines)
	openLoops := episodeOpenLoopAnchors(lines)
	keyEntities := episodeKeyEntities(lines)
	summary := episodeDenseSummary(keyEvents, lines)
	if summary == "" {
		summary = fmt.Sprintf("Episode %d-%d", fromTurn, toTurn)
	}
	item := store.EpisodeSummary{
		ChatSessionID:           sid,
		FromTurn:                fromTurn,
		ToTurn:                  toTurn,
		SummaryText:             truncateRunes(summary, 700),
		KeyEntities:             mustCompactJSON(keyEntities),
		KeyEvents:               mustCompactJSON(keyEvents),
		OpenLoopsJSON:           mustCompactJSON(openLoops),
		RelationshipChangesJSON: mustCompactJSON(relationshipChanges),
		EmbeddingVector:         "[]",
		EmbeddingModel:          "none",
		CreatedAt:               time.Now().UTC(),
	}
	trace := map[string]any{
		"generation_source":          "deterministic_ds1a_fallback",
		"dense_summary_contract":     "ds1a.v1",
		"input_chat_log_count":       len(logs),
		"input_memory_count":         len(memories),
		"input_evidence_count":       len(evidence),
		"key_event_count":            len(keyEvents),
		"relationship_anchor_count":  len(relationshipChanges),
		"open_loop_anchor_count":     len(openLoops),
		"summary_text_anchor_policy": "memory_evidence_first_then_raw_line",
	}
	return item, trace
}

func episodeDenseSummary(keyEvents []string, lines []string) string {
	source := keyEvents
	if len(source) == 0 {
		source = lines
	}
	parts := []string{}
	for _, line := range source {
		text := strings.TrimSpace(line)
		if text == "" {
			continue
		}
		parts = append(parts, truncateRunes(text, 220))
		if len(parts) >= 4 {
			break
		}
	}
	return strings.TrimSpace(strings.Join(parts, " / "))
}

func cleanEpisodeSourceText(text string) string {
	s := strings.TrimSpace(text)
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	lines := []string{}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#### Chatindex:") || strings.HasPrefix(line, "Chatindex:") {
			continue
		}
		line = htmlImgTagPattern.ReplaceAllString(line, "")
		line = strings.Join(strings.Fields(line), " ")
		if line != "" {
			lines = append(lines, line)
		}
	}
	return strings.TrimSpace(strings.Join(lines, " "))
}

func episodeKeyEvents(lines []string) []string {
	out := []string{}
	for _, line := range lines {
		text := strings.TrimSpace(line)
		if text == "" {
			continue
		}
		out = append(out, truncateRunes(text, 180))
		if len(out) >= 3 {
			break
		}
	}
	if len(out) == 0 {
		return []string{}
	}
	return out
}

func episodeRelationshipAnchors(lines []string) []string {
	keywords := []string{"trust", "trusted", "trusts", "relationship", "bond", "promise", "confess", "confession", "kiss", "love", "betray", "betrayal", "ally", "friend"}
	out := []string{}
	for _, line := range lines {
		lower := strings.ToLower(line)
		for _, keyword := range keywords {
			if strings.Contains(lower, keyword) {
				out = append(out, truncateRunes(strings.TrimSpace(line), 180))
				break
			}
		}
		if len(out) >= 3 {
			break
		}
	}
	if len(out) == 0 {
		return []string{}
	}
	return out
}

func episodeOpenLoopAnchors(lines []string) []string {
	keywords := []string{"unresolved", "remains", "must", "will", "next", "promise", "debt", "mystery", "clue", "sealed", "gate", "return"}
	out := []string{}
	for _, line := range lines {
		lower := strings.ToLower(line)
		for _, keyword := range keywords {
			if strings.Contains(lower, keyword) {
				out = append(out, truncateRunes(strings.TrimSpace(line), 180))
				break
			}
		}
		if len(out) >= 3 {
			break
		}
	}
	if len(out) == 0 {
		return []string{}
	}
	return out
}

func episodeKeyEntities(lines []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, line := range lines {
		for _, token := range strings.Fields(line) {
			token = strings.Trim(token, "[](){}.,:;!?\"'")
			if len(token) < 3 {
				continue
			}
			if isEpisodeMetaEntityToken(token) {
				continue
			}
			r := []rune(token)[0]
			if r < 'A' || r > 'Z' {
				continue
			}
			key := strings.ToLower(token)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, token)
			if len(out) >= 8 {
				return out
			}
		}
	}
	return out
}

func isEpisodeMetaEntityToken(token string) bool {
	normalized := strings.Trim(strings.TrimSpace(token), "[](){}.,:;!?\"'")
	if normalized == "" {
		return true
	}
	lower := strings.ToLower(normalized)
	switch lower {
	case "chatindex", "step", "user", "assistant", "system", "narration", "narrative", "mon", "tue", "wed", "thu", "fri", "sat", "sun", "am", "pm":
		return true
	}
	if len(normalized) <= 5 && strings.ToUpper(normalized) == normalized {
		return true
	}
	if strings.Contains(lower, "chatindex") {
		return true
	}
	return false
}

func normalizedChapterInterval(interval int) int {
	if interval <= 0 {
		interval = 60
	}
	if interval < 10 {
		return 10
	}
	if interval > 200 {
		return 200
	}
	return interval
}

func chapterIntervalCheck(turnIndex, interval int) map[string]any {
	info := map[string]any{
		"checked":   true,
		"triggered": false,
		"range":     nil,
		"reason":    "",
	}
	if turnIndex < interval || turnIndex%interval != 0 {
		info["reason"] = "not_interval_boundary"
		return info
	}
	fromTurn := turnIndex - interval + 1
	info["triggered"] = true
	info["range"] = []int{fromTurn, turnIndex}
	info["reason"] = "interval_boundary"
	return info
}

func turnSpanRecommended(turnSpan any) bool {
	span, ok := turnSpan.(int)
	return ok && span >= 40 && span <= 80
}

const (
	chapterDenseSummaryPolicyVersion  = "ds1b.v1"
	arcDenseSummaryPolicyVersion      = "ds1c.v1"
	denseSummaryPriorityPolicyVersion = "ds1d.v1"
	denseSourceAnchorPolicyVersion    = "ds1f.v1"
	denseRetentionPolicyVersion       = "ds1g.v1"
	denseRoleSplitPolicyVersion       = "ds1h.v1"
	denseEvidencePromotionPolicy      = "ds1i.v1"
)

func denseJSONItems(raw string, limit int) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" || raw == "{}" || raw == "null" {
		return nil
	}
	var decoded any
	items := []string{}
	if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
		collectDenseJSONStrings(decoded, &items, limit)
	} else {
		items = appendDenseUnique(items, raw, limit)
	}
	return items
}

func collectDenseJSONStrings(value any, out *[]string, limit int) {
	if limit > 0 && len(*out) >= limit {
		return
	}
	switch v := value.(type) {
	case string:
		*out = appendDenseUnique(*out, v, limit)
	case []any:
		for _, item := range v {
			collectDenseJSONStrings(item, out, limit)
			if limit > 0 && len(*out) >= limit {
				return
			}
		}
	case map[string]any:
		preferredKeys := []string{"text", "summary", "event", "fact", "change", "debt", "callback", "turn", "relationship", "world", "value"}
		for _, key := range preferredKeys {
			if item, ok := v[key]; ok {
				collectDenseJSONStrings(item, out, limit)
				if limit > 0 && len(*out) >= limit {
					return
				}
			}
		}
		for _, item := range v {
			collectDenseJSONStrings(item, out, limit)
			if limit > 0 && len(*out) >= limit {
				return
			}
		}
	case float64, bool:
		*out = appendDenseUnique(*out, fmt.Sprint(v), limit)
	}
}

func appendDenseUnique(items []string, value string, limit int) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return items
	}
	value = truncateRunes(value, 240)
	for _, existing := range items {
		if strings.EqualFold(existing, value) {
			return items
		}
	}
	if limit > 0 && len(items) >= limit {
		return items
	}
	return append(items, value)
}

func denseJSONFromItems(items []string, limit int) string {
	out := []string{}
	for _, item := range items {
		out = appendDenseUnique(out, item, limit)
	}
	data, err := json.Marshal(out)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func denseLabeledLines(label string, items []string, limit int) []string {
	out := []string{}
	for _, item := range items {
		out = appendDenseUnique(out, fmt.Sprintf("%s: %s", label, item), limit)
	}
	return out
}

func containsWorldSignal(text string) bool {
	lowered := strings.ToLower(text)
	for _, token := range []string{"world", "rule", "law", "region", "faction", "public", "pressure", "city", "kingdom", "archive", "gate", "tower"} {
		if strings.Contains(lowered, token) {
			return true
		}
	}
	return false
}

func episodeInputPreviews(episodes []store.EpisodeSummary, limit int) []map[string]any {
	if limit <= 0 || limit > len(episodes) {
		limit = len(episodes)
	}
	out := make([]map[string]any, 0, limit)
	for _, ep := range episodes[:limit] {
		preview := ep.SummaryText
		if len(preview) > 160 {
			preview = preview[:160] + "..."
		}
		openLoops := denseJSONItems(ep.OpenLoopsJSON, 4)
		relationshipChanges := denseJSONItems(ep.RelationshipChangesJSON, 4)
		out = append(out, map[string]any{
			"id":                        ep.ID,
			"from_turn":                 ep.FromTurn,
			"to_turn":                   ep.ToTurn,
			"summary_preview":           preview,
			"key_events":                ep.KeyEvents,
			"open_loops_json":           ep.OpenLoopsJSON,
			"relationship_changes_json": ep.RelationshipChangesJSON,
			"dense_anchor_counts": map[string]any{
				"open_loops":           len(openLoops),
				"relationship_changes": len(relationshipChanges),
				"key_events":           len(denseJSONItems(ep.KeyEvents, 4)),
			},
		})
	}
	return out
}

func truncateString(value string, maxLen int) string {
	if maxLen <= 0 || len(value) <= maxLen {
		return value
	}
	return value[:maxLen]
}

func filterEpisodes(episodes []store.EpisodeSummary, query string, fromTurn, toTurn, limit int) []store.EpisodeSummary {
	if limit <= 0 {
		limit = 20
	}
	query = strings.ToLower(strings.TrimSpace(query))
	results := make([]store.EpisodeSummary, 0, len(episodes))
	for _, ep := range episodes {
		if fromTurn > 0 && ep.ToTurn > 0 && ep.ToTurn < fromTurn {
			continue
		}
		if toTurn > 0 && ep.FromTurn > toTurn {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(ep.SummaryText+" "+ep.KeyEntities+" "+ep.KeyEvents+" "+ep.OpenLoopsJSON+" "+ep.RelationshipChangesJSON), query) {
			continue
		}
		results = append(results, ep)
		if len(results) >= limit {
			break
		}
	}
	return results
}

func episodeResults(episodes []store.EpisodeSummary) []any {
	return episodeResultsWithEvidence(episodes, nil)
}

func episodeResultsWithEvidence(episodes []store.EpisodeSummary, evidence []store.DirectEvidence) []any {
	results := make([]any, 0, len(episodes))
	for _, ep := range episodes {
		denseScores := episodeDensePriorityScores(ep)
		item := map[string]any{
			"id":                           ep.ID,
			"source":                       "episode_summary",
			"chat_session_id":              ep.ChatSessionID,
			"from_turn":                    ep.FromTurn,
			"to_turn":                      ep.ToTurn,
			"summary_text":                 ep.SummaryText,
			"key_entities":                 ep.KeyEntities,
			"key_events":                   ep.KeyEvents,
			"open_loops_json":              ep.OpenLoopsJSON,
			"relationship_changes_json":    ep.RelationshipChangesJSON,
			"embedding_model":              ep.EmbeddingModel,
			"dense_summary_policy_version": denseSummaryPriorityPolicyVersion,
			"dense_priority_score":         denseScores["dense_priority_score"],
			"dense_importance_score":       denseScores["dense_importance_score"],
			"dense_relationship_score":     denseScores["dense_relationship_score"],
			"dense_world_score":            denseScores["dense_world_score"],
		}
		for k, v := range denseSummarySurfaceFields("episode", ep.ID, ep.FromTurn, ep.ToTurn, ep.SummaryText, episodeDenseStructuredPayload(ep), denseScores, evidence) {
			item[k] = v
		}
		results = append(results, item)
	}
	return results
}

func chapterResults(chapters []store.ChapterSummary) []any {
	return chapterResultsWithEvidence(chapters, nil)
}

func chapterResultsWithEvidence(chapters []store.ChapterSummary, evidence []store.DirectEvidence) []any {
	results := make([]any, 0, len(chapters))
	for _, ch := range chapters {
		denseScores := chapterDensePriorityScores(ch)
		item := map[string]any{
			"id":                           ch.ID,
			"source":                       "chapter_summary",
			"source_type":                  "chapter",
			"chat_session_id":              ch.ChatSessionID,
			"from_turn":                    ch.FromTurn,
			"to_turn":                      ch.ToTurn,
			"chapter_index":                ch.ChapterIndex,
			"title":                        ch.ChapterTitle,
			"chapter_title":                ch.ChapterTitle,
			"summary_text":                 ch.SummaryText,
			"resume_text":                  ch.ResumeText,
			"open_loops_json":              ch.OpenLoopsJSON,
			"relationship_changes_json":    ch.RelationshipChangesJSON,
			"world_changes_json":           ch.WorldChangesJSON,
			"callback_candidates_json":     ch.CallbackCandidatesJSON,
			"embedding_model":              ch.EmbeddingModel,
			"dense_summary_policy_version": denseSummaryPriorityPolicyVersion,
			"dense_priority_score":         denseScores["dense_priority_score"],
			"dense_importance_score":       denseScores["dense_importance_score"],
			"dense_relationship_score":     denseScores["dense_relationship_score"],
			"dense_world_score":            denseScores["dense_world_score"],
		}
		for k, v := range denseSummarySurfaceFields("chapter", ch.ID, ch.FromTurn, ch.ToTurn, q1FirstNonEmptyString(ch.ResumeText, ch.SummaryText, ch.ChapterTitle), chapterDenseStructuredPayload(ch), denseScores, evidence) {
			item[k] = v
		}
		results = append(results, item)
	}
	return results
}

func denseSummarySurfaceFields(recordType string, id int64, fromTurn, toTurn int, narrativeText string, structuredPayload map[string]any, denseScores map[string]int, evidence []store.DirectEvidence) map[string]any {
	if structuredPayload == nil {
		structuredPayload = map[string]any{}
	}
	fields := denseSourceAnchorFields(recordType, id, fromTurn, toTurn)
	fields["dense_role_split_policy_version"] = denseRoleSplitPolicyVersion
	fields["dense_narrative_text"] = strings.TrimSpace(narrativeText)
	fields["dense_narrative_usage"] = "read_only"
	fields["dense_structured_payload"] = structuredPayload
	fields["dense_structured_usage"] = "adjudication_retrieval"

	relationshipScore := denseScoreValue(denseScores, "dense_relationship_score")
	worldScore := denseScoreValue(denseScores, "dense_world_score")
	importanceScore := denseScoreValue(denseScores, "dense_importance_score")
	structuredCount := denseStructuredPayloadCount(structuredPayload)
	retentionApplied := relationshipScore > 0 || worldScore > 0 || importanceScore >= 2 || structuredCount >= 2
	retentionReason := "standard_dense_priority"
	if retentionApplied {
		retentionReason = "important_fact_retention"
	}
	fields["dense_retention_policy_version"] = denseRetentionPolicyVersion
	fields["dense_retention_applied"] = retentionApplied
	fields["dense_retention_reason"] = retentionReason
	fields["dense_retention_signal_count"] = structuredCount

	for k, v := range denseEvidencePromotionFields(evidence, fromTurn, toTurn, structuredPayload) {
		fields[k] = v
	}
	return fields
}

func denseSourceAnchorFields(recordType string, id int64, fromTurn, toTurn int) map[string]any {
	return map[string]any{
		"dense_source_anchor_policy_version": denseSourceAnchorPolicyVersion,
		"source_record_id":                   id,
		"source_record_type":                 recordType,
		"source_turn_range": map[string]any{
			"from_turn": fromTurn,
			"to_turn":   toTurn,
		},
	}
}

func episodeDenseStructuredPayload(ep store.EpisodeSummary) map[string]any {
	return map[string]any{
		"key_events":           denseJSONItems(ep.KeyEvents, 8),
		"open_loops":           denseJSONItems(ep.OpenLoopsJSON, 8),
		"relationship_changes": denseJSONItems(ep.RelationshipChangesJSON, 8),
	}
}

func chapterDenseStructuredPayload(ch store.ChapterSummary) map[string]any {
	return map[string]any{
		"open_loops":           denseJSONItems(ch.OpenLoopsJSON, 8),
		"relationship_changes": denseJSONItems(ch.RelationshipChangesJSON, 8),
		"world_changes":        denseJSONItems(ch.WorldChangesJSON, 8),
		"callback_candidates":  denseJSONItems(ch.CallbackCandidatesJSON, 8),
	}
}

func arcDenseStructuredPayload(arc store.ArcSummary) map[string]any {
	return map[string]any{
		"key_turning_points":       denseJSONItems(arc.KeyTurningPointsJSON, 8),
		"active_promises":          denseJSONItems(arc.ActivePromisesJSON, 8),
		"unresolved_debts":         denseJSONItems(arc.UnresolvedDebtsJSON, 8),
		"resolved_payoffs":         denseJSONItems(arc.ResolvedPayoffsJSON, 8),
		"callback_candidates":      denseJSONItems(arc.CallbackCandidatesJSON, 8),
		"future_payoff_candidates": denseJSONItems(arc.FuturePayoffCandidatesJSON, 8),
		"irreversible_turns":       denseJSONItems(arc.IrreversibleTurnsJSON, 8),
		"callback_debts":           denseJSONItems(arc.CallbackDebtsJSON, 8),
		"relationship_pivots":      denseJSONItems(arc.RelationshipPivotsJSON, 8),
	}
}

func sagaDenseStructuredPayload(saga store.SagaDigest) map[string]any {
	return map[string]any{
		"persistent_facts":      denseJSONItems(saga.PersistentFactsJSON, 8),
		"never_drop_candidates": denseJSONItems(saga.NeverDropCandidatesJSON, 8),
	}
}

func denseScoreValue(scores map[string]int, key string) int {
	if scores == nil {
		return 0
	}
	return scores[key]
}

func denseStructuredPayloadCount(payload map[string]any) int {
	count := 0
	for _, value := range payload {
		switch v := value.(type) {
		case []string:
			count += len(v)
		case []any:
			count += len(v)
		case string:
			if strings.TrimSpace(v) != "" {
				count++
			}
		}
	}
	return count
}

func denseEvidencePromotionFields(evidence []store.DirectEvidence, fromTurn, toTurn int, structuredPayload map[string]any) map[string]any {
	relationshipCount := 0
	worldCount := 0
	promiseCount := 0
	for _, ev := range evidence {
		if !denseEvidenceOverlaps(ev, fromTurn, toTurn) {
			continue
		}
		text := strings.ToLower(strings.TrimSpace(ev.EvidenceKind + " " + ev.EvidenceText + " " + ev.LineageJSON))
		if denseTextHasAny(text, []string{"relationship", "trust", "ally", "friend", "bond", "betray", "love", "rival"}) {
			relationshipCount++
		}
		if denseTextHasAny(text, []string{"world", "rule", "law", "faction", "city", "kingdom", "gate", "region", "pressure"}) {
			worldCount++
		}
		if denseTextHasAny(text, []string{"promise", "vow", "oath", "callback", "debt", "repay", "payoff"}) {
			promiseCount++
		}
	}
	structuredText := strings.ToLower(fmt.Sprint(structuredPayload))
	if relationshipCount == 0 && denseTextHasAny(structuredText, []string{"relationship", "trust", "ally", "bond", "pivot"}) {
		relationshipCount = 1
	}
	if worldCount == 0 && denseTextHasAny(structuredText, []string{"world", "rule", "law", "faction", "city", "gate"}) {
		worldCount = 1
	}
	if promiseCount == 0 && denseTextHasAny(structuredText, []string{"promise", "callback", "debt", "payoff"}) {
		promiseCount = 1
	}
	score := relationshipCount + worldCount + promiseCount
	return map[string]any{
		"dense_direct_evidence_promotion_policy_version":    denseEvidencePromotionPolicy,
		"dense_direct_evidence_promotion_score":             score,
		"dense_direct_evidence_promoted_relationship_count": relationshipCount,
		"dense_direct_evidence_promoted_world_count":        worldCount,
		"dense_direct_evidence_promoted_promise_count":      promiseCount,
		"dense_structured_precedence_applied":               score > 0,
	}
}

func denseEvidenceOverlaps(ev store.DirectEvidence, fromTurn, toTurn int) bool {
	start := ev.SourceTurnStart
	end := ev.SourceTurnEnd
	if start <= 0 {
		start = ev.TurnAnchor
	}
	if end <= 0 {
		end = start
	}
	if fromTurn <= 0 && toTurn <= 0 {
		return true
	}
	if toTurn <= 0 {
		toTurn = fromTurn
	}
	if fromTurn <= 0 {
		fromTurn = toTurn
	}
	return start <= toTurn && end >= fromTurn
}

func denseTextHasAny(text string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func chapterIndexForRange(toTurn, interval int) int {
	if interval <= 0 {
		interval = 60
	}
	if toTurn <= 0 {
		return 1
	}
	idx := toTurn / interval
	if toTurn%interval != 0 {
		idx++
	}
	if idx <= 0 {
		return 1
	}
	return idx
}

func hierarchyIndexForRange(toTurn, interval int) int {
	return chapterIndexForRange(toTurn, interval)
}
