package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func (s *Server) handleSessionsCompare(w http.ResponseWriter, r *http.Request) {
	if idsRaw := strings.TrimSpace(r.URL.Query().Get("session_ids")); idsRaw != "" {
		ids := []string{}
		for _, part := range strings.Split(idsRaw, ",") {
			if sid := strings.TrimSpace(part); sid != "" {
				ids = append(ids, sid)
			}
		}
		if len(ids) < 2 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "detail": "At least 2 session_ids are required."})
			return
		}
		if len(ids) > 2 {
			ids = ids[:2]
		}
		previewLimit, _ := strconv.Atoi(r.URL.Query().Get("preview_limit"))
		if previewLimit < 1 {
			previewLimit = 10
		}
		if previewLimit > 30 {
			previewLimit = 30
		}
		result := map[string]any{}
		for _, sid := range ids {
			ev := s.collectNarrativeEvidence(r.Context(), sid)
			result[sid] = sessionComparePayload(ev, previewLimit)
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "sessions": result})
		return
	}
	writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "detail": "At least 2 session_ids are required."})
}

func sessionComparePayload(ev narrativeEvidence, previewLimit int) map[string]any {
	logs := append([]store.ChatLog(nil), ev.ChatLogs...)
	sort.SliceStable(logs, func(i, j int) bool {
		if logs[i].TurnIndex == logs[j].TurnIndex {
			return logs[i].ID > logs[j].ID
		}
		return logs[i].TurnIndex > logs[j].TurnIndex
	})
	if len(logs) > previewLimit*2 {
		logs = logs[:previewLimit*2]
	}
	logsPreview := []map[string]any{}
	for i := len(logs) - 1; i >= 0; i-- {
		log := logs[i]
		logsPreview = append(logsPreview, map[string]any{
			"id":         log.ID,
			"turn_index": log.TurnIndex,
			"role":       log.Role,
			"content":    truncateForPreview(log.Content, 200),
			"created_at": formatKSTTime(log.CreatedAt),
		})
	}

	memories := append([]store.Memory(nil), ev.Memories...)
	sort.SliceStable(memories, func(i, j int) bool { return memories[i].ID > memories[j].ID })
	memPreview := []map[string]any{}
	for _, mem := range memories {
		if len(memPreview) >= previewLimit {
			break
		}
		memPreview = append(memPreview, map[string]any{
			"id":           mem.ID,
			"summary_json": truncateForPreview(mem.SummaryJSON, 200),
			"importance":   mem.Importance,
			"created_at":   formatKSTTime(mem.CreatedAt),
		})
	}

	triples := append([]store.KGTriple(nil), ev.KGTriples...)
	sort.SliceStable(triples, func(i, j int) bool { return triples[i].ID > triples[j].ID })
	kgPreview := []map[string]any{}
	for _, triple := range triples {
		if len(kgPreview) >= previewLimit {
			break
		}
		kgPreview = append(kgPreview, map[string]any{
			"id":         triple.ID,
			"subject":    triple.Subject,
			"predicate":  triple.Predicate,
			"object":     triple.Object,
			"created_at": formatKSTTime(triple.CreatedAt),
		})
	}

	feedbackUp := 0
	feedbackDown := 0
	for _, feedback := range ev.CriticFeedback {
		switch strings.ToLower(feedback.FeedbackValue) {
		case "up":
			feedbackUp++
		case "down":
			feedbackDown++
		}
	}

	var lastActivity any
	for _, log := range ev.ChatLogs {
		if t := log.CreatedAt; !t.IsZero() {
			if lastActivity == nil || t.After(lastActivity.(time.Time)) {
				lastActivity = t
			}
		}
	}
	if t, ok := lastActivity.(time.Time); ok {
		lastActivity = formatKSTTime(t)
	}

	return map[string]any{
		"counts": map[string]any{
			"chat_logs":     len(ev.ChatLogs),
			"memories":      len(ev.Memories),
			"kg_triples":    len(ev.KGTriples),
			"audit_logs":    len(ev.AuditLogs),
			"feedback_up":   feedbackUp,
			"feedback_down": feedbackDown,
		},
		"last_activity":    lastActivity,
		"logs_preview":     logsPreview,
		"memories_preview": memPreview,
		"kg_triples":       kgPreview,
	}
}

func (s *Server) handleActiveStates(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}
	stateType := r.URL.Query().Get("state_type")
	items, err := s.Store.ListActiveStates(r.Context(), sid, stateType)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			items = nil
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	items = nonNilSlice(items)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"states":          items,
		"count":           len(items),
	})
}

func (s *Server) handleCanonicalStateLayer(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}
	layerType := r.URL.Query().Get("layer_type")
	items, err := s.Store.ListCanonicalStateLayers(r.Context(), sid, layerType)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			items = nil
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	items = nonNilSlice(items)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"layers":          items,
		"count":           len(items),
	})
}

func (s *Server) handleSessionState(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}
	ctx := r.Context()

	snapshot, err := s.readSessionStateSnapshot(ctx, sid)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}

	activeStates := nonNilSlice(snapshot.ActiveStates)
	canonicalLayers := nonNilSlice(snapshot.CanonicalStateLayers)
	storylines := nonNilSlice(snapshot.Storylines)
	characters := nonNilSlice(snapshot.CharacterStates)
	worldRules := nonNilSlice(snapshot.WorldRules)
	pendingThreads := nonNilSlice(snapshot.PendingThreads)
	characterEvents := nonNilSlice(snapshot.CharacterEvents)

	storylines = visibleSessionStateStorylines(storylines)
	worldRules = visibleSessionStateWorldRules(worldRules)
	pendingThreads = continuityPendingThreads(pendingThreads, 0)
	referenceTurn := resolveCharacterReferenceTurn(activeStates, storylines, characters)
	recentText, recentKeywords := characterRecentMentionSignalFromLogs(snapshot.RecentChatLogs, referenceTurn)
	if !snapshot.SingleConnection && len(snapshot.RecentChatLogs) == 0 {
		recentText, recentKeywords = s.characterRecentMentionSignal(ctx, sid, referenceTurn)
	}
	characterItems := characterResponseItems(characters, characterEvents, referenceTurn, recentText, recentKeywords)
	omittedCharacters := characterOmittedCount(characters, characterEvents, referenceTurn, recentText, recentKeywords)
	storylineItems := storylineResponseItems(storylines, resolveStorylineReferenceTurn(storylines, ""))
	worldRuleItems := worldRuleResponseItems(worldRules, "")
	pendingThreadItems := pendingThreadResponseItems(pendingThreads)

	sectionMeta := map[string]any{
		"active_states":         sessionStateMetaForActiveStates(activeStates),
		"storylines":            sessionStateMetaForStorylines(storylines),
		"characters":            sessionStateMetaForCharacterItems(characterItems),
		"world_rules":           sessionStateMetaForWorldRules(worldRules),
		"pending_threads":       sessionStateMetaForPendingThreads(pendingThreads),
		"continuity_hooks":      sessionStateMetaForPendingThreads(pendingThreads),
		"chapter_summaries":     map[string]any{"count": 0, "last_turn": nil, "updated_at": nil, "ready": false},
		"canonical_state_layer": sessionStateMetaForCanonicalLayers(canonicalLayers),
	}
	guidanceSnapshot, _ := s.buildL3GuidanceSnapshot(ctx, sid)
	sectionMeta["guidance_snapshot"] = map[string]any{
		"state_status": guidanceSnapshot["state_status"],
		"last_turn":    guidanceSnapshot["last_turn"],
		"ready":        guidanceSnapshot["state_status"] == "active",
	}
	snapshotStatus := sessionStateSnapshotStatus(len(activeStates), len(storylineItems), len(characterItems), len(worldRuleItems), len(pendingThreadItems))
	warnings := []any{}
	if omittedCharacters > 0 {
		warnings = append(warnings, fmt.Sprintf("%d transient descriptor character(s) omitted from characters section.", omittedCharacters))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"active_states":         activeStates,
		"canonical_state_layer": canonicalLayers,
		"chapter_summaries":     []any{},
		"characters":            characterItems,
		"chat_session_id":       sid,
		"generated_at":          generatedAt(),
		"guidance_snapshot":     guidanceSnapshot,
		"pending_threads":       pendingThreadItems,
		"continuity_hooks":      pendingThreadItems,
		"section_meta":          sectionMeta,
		"snapshot_status":       snapshotStatus,
		"status":                "ok",
		"storylines":            storylineItems,
		"warnings":              warnings,
		"world_rules":           worldRuleItems,
	})
}

func (s *Server) readSessionStateSnapshot(ctx context.Context, sid string) (store.SessionStateSnapshot, error) {
	if s.Store == nil {
		return store.SessionStateSnapshot{}, nil
	}
	if reader, ok := s.Store.(store.SessionStateSnapshotReader); ok {
		snapshot, err := reader.ReadSessionStateSnapshot(ctx, sid)
		if err != nil {
			if errors.Is(err, store.ErrNotEnabled) {
				return store.SessionStateSnapshot{}, nil
			}
			return store.SessionStateSnapshot{}, err
		}
		if snapshot != nil {
			return *snapshot, nil
		}
	}
	activeStates, _ := s.Store.ListActiveStates(ctx, sid, "")
	canonicalLayers, _ := s.Store.ListCanonicalStateLayers(ctx, sid, "")
	storylines, _ := s.Store.ListStorylines(ctx, sid)
	characters, _ := s.Store.ListCharacterStates(ctx, sid)
	worldRules, _ := s.Store.ListWorldRules(ctx, sid)
	pendingThreads, _ := s.Store.ListPendingThreads(ctx, sid, "")
	characterEvents, _ := s.Store.ListCharacterEvents(ctx, sid, "")
	return store.SessionStateSnapshot{
		ActiveStates:         activeStates,
		CanonicalStateLayers: canonicalLayers,
		Storylines:           storylines,
		CharacterStates:      characters,
		WorldRules:           worldRules,
		PendingThreads:       pendingThreads,
		CharacterEvents:      characterEvents,
		SingleConnection:     false,
	}, nil
}

func visibleSessionStateStorylines(items []store.Storyline) []store.Storyline {
	out := make([]store.Storyline, 0, len(items))
	for _, item := range items {
		if item.Suppressed {
			continue
		}
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Pinned != out[j].Pinned {
			return out[i].Pinned
		}
		if out[i].LastTurn != out[j].LastTurn {
			return out[i].LastTurn > out[j].LastTurn
		}
		return out[i].ID > out[j].ID
	})
	return out
}

func visibleSessionStateWorldRules(items []store.WorldRule) []store.WorldRule {
	out := make([]store.WorldRule, 0, len(items))
	for _, item := range items {
		if item.Suppressed {
			continue
		}
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Pinned != out[j].Pinned {
			return out[i].Pinned
		}
		for _, cmp := range []int{strings.Compare(out[i].Scope, out[j].Scope), strings.Compare(out[i].Category, out[j].Category), strings.Compare(out[i].Key, out[j].Key)} {
			if cmp != 0 {
				return cmp < 0
			}
		}
		return out[i].ID > out[j].ID
	})
	return out
}

func resolveCharacterReferenceTurn(activeStates []store.ActiveState, storylines []store.Storyline, characters []store.CharacterState) int {
	ref := 0
	for _, item := range activeStates {
		if item.TurnIndex > ref {
			ref = item.TurnIndex
		}
	}
	for _, item := range storylines {
		if item.LastTurn > ref {
			ref = item.LastTurn
		}
		if item.LastEvidenceTurn > ref {
			ref = item.LastEvidenceTurn
		}
	}
	for _, item := range characters {
		if item.TurnIndex > ref {
			ref = item.TurnIndex
		}
	}
	return ref
}

func sessionStateSnapshotStatus(counts ...int) string {
	readyCount := 0
	for _, count := range counts {
		if count > 0 {
			readyCount++
		}
	}
	if readyCount == len(counts) && len(counts) > 0 {
		return "ready"
	}
	if readyCount > 0 {
		return "partial"
	}
	return "empty"
}

func sessionStateMeta(count int, lastTurn int, updatedAt time.Time) map[string]any {
	var last any
	if lastTurn > 0 {
		last = lastTurn
	}
	return map[string]any{
		"count":      count,
		"last_turn":  last,
		"updated_at": nullableTime(updatedAt),
		"ready":      count > 0,
	}
}

func sessionStateMetaForActiveStates(items []store.ActiveState) map[string]any {
	maxTurn := 0
	var updated time.Time
	for _, item := range items {
		if item.TurnIndex > maxTurn {
			maxTurn = item.TurnIndex
		}
		if item.CreatedAt.After(updated) {
			updated = item.CreatedAt
		}
	}
	return sessionStateMeta(len(items), maxTurn, updated)
}

func sessionStateMetaForCanonicalLayers(items []store.CanonicalStateLayer) map[string]any {
	maxTurn := 0
	var updated time.Time
	for _, item := range items {
		if item.TurnIndex > maxTurn {
			maxTurn = item.TurnIndex
		}
		if item.CreatedAt.After(updated) {
			updated = item.CreatedAt
		}
	}
	return sessionStateMeta(len(items), maxTurn, updated)
}

func sessionStateMetaForStorylines(items []store.Storyline) map[string]any {
	maxTurn := 0
	var updated time.Time
	for _, item := range items {
		if item.LastTurn > maxTurn {
			maxTurn = item.LastTurn
		}
		if item.UpdatedAt.After(updated) {
			updated = item.UpdatedAt
		}
	}
	return sessionStateMeta(len(items), maxTurn, updated)
}

func sessionStateMetaForCharacterItems(items []map[string]any) map[string]any {
	maxTurn := 0
	var updated time.Time
	for _, item := range items {
		if turn, ok := mapIntValue(item, "turn_index"); ok && turn > maxTurn {
			maxTurn = turn
		}
		if t, ok := mapTimeValue(item, "updated_at"); ok && t.After(updated) {
			updated = t
		}
	}
	return sessionStateMeta(len(items), maxTurn, updated)
}

func sessionStateMetaForWorldRules(items []store.WorldRule) map[string]any {
	maxTurn := 0
	var updated time.Time
	for _, item := range items {
		if item.SourceTurn > maxTurn {
			maxTurn = item.SourceTurn
		}
		if item.UpdatedAt.After(updated) {
			updated = item.UpdatedAt
		}
	}
	return sessionStateMeta(len(items), maxTurn, updated)
}

func sessionStateMetaForPendingThreads(items []store.PendingThread) map[string]any {
	maxTurn := 0
	var updated time.Time
	for _, item := range items {
		turn := item.LastSeenTurn
		if turn == 0 {
			turn = item.SourceTurn
		}
		if turn == 0 {
			turn = item.ResolvedTurn
		}
		if turn > maxTurn {
			maxTurn = turn
		}
		if item.UpdatedAt.After(updated) {
			updated = item.UpdatedAt
		}
	}
	return sessionStateMeta(len(items), maxTurn, updated)
}

func mapIntValue(item map[string]any, key string) (int, bool) {
	switch typed := item[key].(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		if typed == float64(int(typed)) {
			return int(typed), true
		}
	case json.Number:
		i, err := typed.Int64()
		return int(i), err == nil
	}
	return 0, false
}

func mapTimeValue(item map[string]any, key string) (time.Time, bool) {
	raw, ok := item[key]
	if !ok || raw == nil {
		return time.Time{}, false
	}
	text := strings.TrimSpace(fmt.Sprint(raw))
	if text == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999999", "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, text); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func (s *Server) handleContinuityPack(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}
	ctx := r.Context()

	storylines, _ := s.Store.ListStorylines(ctx, sid)
	pendingThreads, _ := s.Store.ListPendingThreads(ctx, sid, "")
	episodes, _ := s.Store.ListEpisodeSummaries(ctx, sid, 1, 0, 0)
	worldRules, _ := s.Store.ListWorldRules(ctx, sid)
	characterEvents, _ := s.Store.ListCharacterEvents(ctx, sid, "")

	storylines = nonNilSlice(storylines)
	episodes = nonNilSlice(episodes)
	worldRules = nonNilSlice(worldRules)
	storylineSelection := selectStorylinesForSupervisor(storylines, nil, 3)
	activeStorylines := storylineResponseItems(selectedStorylineItems(storylineSelection), storylineSelection.ReferenceTurn)
	relationshipShifts := continuityRelationshipShifts(characterEvents, 5)
	pendingThreads = continuityPendingThreads(pendingThreads, 5)
	pendingThreadItems := pendingThreadResponseItems(pendingThreads)
	if len(worldRules) > 8 {
		worldRules = worldRules[:8]
	}

	var latestEpisode any
	if len(episodes) > 0 {
		latestEpisode = episodes[0]
	}

	packStatus := "empty"
	if len(activeStorylines) > 0 || len(relationshipShifts) > 0 || len(pendingThreadItems) > 0 || len(episodes) > 0 || len(worldRules) > 0 {
		packStatus = "ready"
	}

	warnings := []any{}
	if len(activeStorylines) == 0 && len(relationshipShifts) == 0 && len(episodes) == 0 && len(worldRules) == 0 {
		warnings = append(warnings, "No continuity source data available for this session yet.")
	}
	if dropped := len(storylineSelection.Dropped); dropped > 0 {
		warnings = append(warnings, fmt.Sprintf("%d storyline(s) omitted by continuity freshness gate.", dropped))
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"active_storylines":   activeStorylines,
		"chat_session_id":     sid,
		"generated_at":        generatedAt(),
		"latest_episode":      latestEpisode,
		"pack_status":         packStatus,
		"pending_threads":     pendingThreadItems,
		"relationship_shifts": relationshipShifts,
		"section_status": map[string]any{
			"active_storylines":   map[string]any{"ready": true, "count": len(activeStorylines), "note": "Selected with storyline freshness gate"},
			"relationship_shifts": map[string]any{"ready": true, "count": len(relationshipShifts), "note": "Recent relationship_shift events"},
			"pending_threads":     map[string]any{"ready": true, "count": len(pendingThreadItems), "note": "Open/paused hooks (pinned first, suppressed excluded)"},
			"continuity_hooks":    map[string]any{"ready": true, "count": len(pendingThreadItems), "note": "Alias for pending_threads; open/paused hooks (pinned first, suppressed excluded)"},
			"latest_episode":      map[string]any{"ready": true, "count": len(episodes), "note": "Latest generated episode summary"},
			"world_constraints":   map[string]any{"ready": true, "count": len(worldRules), "note": "Current world-rule snapshot"},
		},
		"skeleton_only":     false,
		"status":            "ok",
		"warnings":          warnings,
		"world_constraints": worldRules,
	})
}

func continuityRelationshipShifts(events []store.CharacterEvent, limit int) []map[string]any {
	items := []store.CharacterEvent{}
	for _, item := range nonNilSlice(events) {
		if strings.TrimSpace(item.EventType) != "relationship_shift" {
			continue
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].TurnIndex != items[j].TurnIndex {
			return items[i].TurnIndex > items[j].TurnIndex
		}
		if !items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].CreatedAt.After(items[j].CreatedAt)
		}
		return items[i].ID > items[j].ID
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{
			"id":              item.ID,
			"chat_session_id": item.ChatSessionID,
			"character_name":  item.CharacterName,
			"turn_index":      nullablePositiveInt(item.TurnIndex),
			"event_type":      item.EventType,
			"details_json":    nullableString(item.DetailsJSON),
			"created_at":      formatKSTTime(item.CreatedAt),
		})
	}
	return out
}

func continuityPendingThreads(items []store.PendingThread, limit int) []store.PendingThread {
	out := []store.PendingThread{}
	for _, item := range nonNilSlice(items) {
		status := strings.TrimSpace(item.Status)
		if status != "" && status != "open" && status != "paused" {
			continue
		}
		if item.Suppressed {
			continue
		}
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Pinned != out[j].Pinned {
			return out[i].Pinned
		}
		left := out[i].LastSeenTurn
		if left == 0 {
			left = out[i].SourceTurn
		}
		right := out[j].LastSeenTurn
		if right == 0 {
			right = out[j].SourceTurn
		}
		if left != right {
			return left > right
		}
		return out[i].ID > out[j].ID
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (s *Server) handlePendingThreads(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	items, err := s.Store.ListPendingThreads(r.Context(), sid, status)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			items = []store.PendingThread{}
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	items = nonNilSlice(items)
	statusFilter := status
	if statusFilter == "" {
		statusFilter = "open+paused"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"hooks":           pendingThreadResponseItems(items),
		"count":           len(items),
		"status_filter":   statusFilter,
	})
}

func (s *Server) handleContinuityHooks(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	items, err := s.Store.ListPendingThreads(r.Context(), sid, status)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			items = []store.PendingThread{}
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	items = nonNilSlice(items)
	statusFilter := status
	if statusFilter == "" {
		statusFilter = "open+paused"
	}
	hooks := pendingThreadResponseItems(items)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"items":           hooks,
		"hooks":           hooks,
		"count":           len(hooks),
		"fetched":         true,
		"status_filter":   statusFilter,
	})
}

func pendingThreadResponseItems(items []store.PendingThread) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		meta := pendingThreadMetadata(item)
		threadType := firstNonEmpty(item.ThreadType, metadataString(meta, "thread_type"), item.HookType)
		title := firstNonEmpty(item.Title, metadataString(meta, "title"), item.Description, item.ThreadKey)
		lastSeen := item.LastSeenTurn
		if lastSeen == 0 {
			if v, ok := metadataInt(meta, "last_seen_turn"); ok {
				lastSeen = v
			}
		}
		if lastSeen == 0 {
			lastSeen = item.ResolvedTurn
		}
		confidence := item.Confidence
		if confidence == 0 {
			if v, ok := metadataFloat(meta, "confidence"); ok {
				confidence = v
			}
		}
		if confidence == 0 && item.Priority > 0 {
			confidence = float64(item.Priority) / 100.0
		}
		detailsJSON := firstNonEmpty(item.DetailsJSON, metadataJSONText(meta, "details_json"), metadataJSONText(meta, "details"), item.HookMetadataJSON)
		resolutionNote := firstNonEmpty(item.ResolutionNote, metadataString(meta, "resolution_note"))
		out = append(out, map[string]any{
			"id":              item.ID,
			"chat_session_id": item.ChatSessionID,
			"thread_type":     threadType,
			"hook_type":       firstNonEmpty(item.HookType, threadType),
			"thread_key":      item.ThreadKey,
			"title":           title,
			"description":     item.Description,
			"status":          item.Status,
			"owner":           firstNonEmpty(item.Owner, metadataString(meta, "owner")),
			"target":          firstNonEmpty(item.Target, metadataString(meta, "target")),
			"source_turn":     nullablePositiveInt(item.SourceTurn),
			"last_seen_turn":  nullablePositiveInt(lastSeen),
			"confidence":      confidence,
			"details_json":    detailsJSON,
			"resolution_note": nullableString(resolutionNote),
			"pinned":          item.Pinned,
			"suppressed":      item.Suppressed,
			"user_corrected":  item.UserCorrected,
			"created_at":      formatKSTTime(item.CreatedAt),
			"updated_at":      formatKSTTime(item.UpdatedAt),
		})
	}
	return out
}

func pendingThreadMetadata(item store.PendingThread) map[string]any {
	raw := strings.TrimSpace(firstNonEmpty(item.HookMetadataJSON, item.DetailsJSON))
	if raw == "" {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func metadataString(meta map[string]any, key string) string {
	if meta == nil {
		return ""
	}
	switch typed := meta[key].(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

func metadataFloat(meta map[string]any, key string) (float64, bool) {
	if meta == nil {
		return 0, false
	}
	return storylineFloatPatchValue(meta[key])
}

func metadataInt(meta map[string]any, key string) (int, bool) {
	if meta == nil {
		return 0, false
	}
	return storylineIntPatchValue(meta[key])
}

func metadataJSONText(meta map[string]any, key string) string {
	if meta == nil {
		return ""
	}
	val, ok := meta[key]
	if !ok || val == nil {
		return ""
	}
	if text, ok := val.(string); ok {
		return strings.TrimSpace(text)
	}
	return mustCompactJSON(val)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func nullablePositiveInt(v int) any {
	if v == 0 {
		return nil
	}
	return v
}
