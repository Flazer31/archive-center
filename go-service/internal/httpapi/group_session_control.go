package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

func (s *Server) handleActiveScopeGet(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if strings.TrimSpace(sid) == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}
	ctx := r.Context()
	item, source, err := s.resolveActiveScope(ctx, sid)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, activeScopeResponse(sid, item, source))
}

func (s *Server) handleMomentumPacket(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	ctx := r.Context()

	storylines, _ := s.Store.ListStorylines(ctx, sid)
	pendingThreads, _ := s.Store.ListPendingThreads(ctx, sid, "")
	characterStates, _ := s.Store.ListCharacterStates(ctx, sid)
	storylines = nonNilSlice(storylines)
	pendingThreads = nonNilSlice(pendingThreads)
	characterStates = nonNilSlice(characterStates)

	nextPressure := momentumNextPressure(storylines)
	payoffCandidates := momentumPayoffCandidates(storylines, pendingThreads, characterStates)
	tensionToReuse := momentumTensionToReuse(pendingThreads)
	beatsToAvoid := momentumBeatsToAvoid(storylines)
	totalItems := len(nextPressure) + len(payoffCandidates) + len(tensionToReuse) + len(beatsToAvoid)

	warnings := []any{}
	if totalItems == 0 && len(storylines) == 0 && len(pendingThreads) == 0 && len(characterStates) == 0 {
		warnings = append(warnings, "No active storylines, open hooks, or relationship states found for this session.")
	}

	packetStatus := "partial"
	if totalItems == 0 {
		packetStatus = "empty"
	} else if totalItems >= 4 {
		packetStatus = "ready"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"beats_to_avoid":    beatsToAvoid,
		"chat_session_id":   sid,
		"generated_at":      generatedAt(),
		"next_pressure":     nextPressure,
		"packet_status":     packetStatus,
		"payoff_candidates": payoffCandidates,
		"status":            "ok",
		"tension_to_reuse":  tensionToReuse,
		"warnings":          warnings,
	})
}

func momentumNextPressure(storylines []store.Storyline) []map[string]any {
	active := activeNarrativeStorylines(storylines)
	out := []map[string]any{}
	for _, item := range active {
		tensions := parseStorylineListJSON(item.OngoingTensionsJSON)
		if len(tensions) == 0 {
			continue
		}
		out = append(out, momentumItem(tensions[0], "storyline", item.ID, item.Name, 100+item.EvidenceCount, map[string]any{
			"last_turn": item.LastTurn,
			"reason":    "active_storyline_ongoing_tension",
		}))
		if len(out) >= 2 {
			return out
		}
	}
	for _, item := range active {
		if len(out) >= 2 {
			break
		}
		out = append(out, momentumItem(firstNonEmpty(item.CurrentContext, item.Name), "storyline", item.ID, item.Name, 50+item.EvidenceCount, map[string]any{
			"last_turn": item.LastTurn,
			"reason":    "latest_active_storyline_fallback",
		}))
	}
	return out
}

func momentumPayoffCandidates(storylines []store.Storyline, pendingThreads []store.PendingThread, characterStates []store.CharacterState) []map[string]any {
	active := activeNarrativeStorylines(storylines)
	out := []map[string]any{}
	for _, item := range active {
		if item.EvidenceCount < 3 || item.Confidence < 0.7 {
			continue
		}
		label := firstNonEmpty(lastStorylineListItem(item.KeyPointsJSON), item.CurrentContext, item.Name)
		out = append(out, momentumItem(label, "storyline", item.ID, item.Name, 90+item.EvidenceCount, map[string]any{
			"confidence":     item.Confidence,
			"evidence_count": item.EvidenceCount,
			"reason":         "high_confidence_storyline_payoff",
		}))
		if len(out) >= 2 {
			break
		}
	}
	for _, item := range momentumRelationshipPayoffCandidates(characterStates) {
		if len(out) >= 4 {
			return out
		}
		out = append(out, item)
	}
	hooks := openNarrativeThreadsOldestFirst(pendingThreads)
	for _, hook := range hooks {
		if len(out) >= 4 {
			break
		}
		out = append(out, momentumItem(pendingThreadNarrativeLabel(hook), "pending_thread", hook.ID, pendingThreadTitle(hook), 60+hook.Priority, map[string]any{
			"last_seen_turn": firstPositiveInt(hook.LastSeenTurn, hook.SourceTurn, hook.CreatedTurn),
			"reason":         "older_open_hook_payoff",
		}))
	}
	return out
}

func momentumRelationshipPayoffCandidates(characterStates []store.CharacterState) []map[string]any {
	candidates := []map[string]any{}
	seen := map[string]bool{}
	states := append([]store.CharacterState{}, nonNilSlice(characterStates)...)
	sort.SliceStable(states, func(i, j int) bool {
		if states[i].TurnIndex != states[j].TurnIndex {
			return states[i].TurnIndex > states[j].TurnIndex
		}
		return states[i].ID > states[j].ID
	})
	for _, character := range states {
		lane := buildCharacterRelationshipLane(character)
		rawItems, _ := lane["items"].([]any)
		for _, raw := range rawItems {
			relation, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			target := cleanShadowText(relation["target"], 80)
			summary := cleanShadowText(relation["summary_text"], 180)
			if target == "" || summary == "" {
				continue
			}
			key := strings.ToLower(strings.TrimSpace(character.CharacterName) + "\x00" + target + "\x00" + summary)
			if seen[key] {
				continue
			}
			seen[key] = true
			sourceName := strings.TrimSpace(character.CharacterName)
			if sourceName == "" {
				sourceName = "relationship"
			}
			sourceName = sourceName + " -> " + target
			priority := 75 + minInt(maxInt(character.TurnIndex, 0), 10)
			if displayPriority, ok := relation["display_priority"].(int); ok && displayPriority == 0 {
				priority += 8
			}
			candidates = append(candidates, momentumItem(summary, "relationship", character.ID, sourceName, priority, map[string]any{
				"character_name": character.CharacterName,
				"target":         target,
				"turn_index":     nullablePositiveInt(character.TurnIndex),
				"reason":         "relationship_state_payoff",
			}))
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		return momentumPriority(candidates[i]) > momentumPriority(candidates[j])
	})
	if len(candidates) > 3 {
		return candidates[:3]
	}
	return candidates
}

func momentumTensionToReuse(pendingThreads []store.PendingThread) []map[string]any {
	hooks := openNarrativeThreadsOldestFirst(pendingThreads)
	out := []map[string]any{}
	for _, hook := range hooks {
		out = append(out, momentumItem(pendingThreadNarrativeLabel(hook), "pending_thread", hook.ID, pendingThreadTitle(hook), 70+hook.Priority, map[string]any{
			"last_seen_turn": firstPositiveInt(hook.LastSeenTurn, hook.SourceTurn, hook.CreatedTurn),
			"reason":         "oldest_open_or_paused_hook",
		}))
		if len(out) >= 3 {
			break
		}
	}
	return out
}

func momentumBeatsToAvoid(storylines []store.Storyline) []map[string]any {
	type duplicateBeat struct {
		label string
		count int
		ids   []int64
	}
	seen := map[string]*duplicateBeat{}
	for _, item := range visibleSessionStateStorylines(storylines) {
		for _, beat := range parseStorylineListJSON(item.KeyPointsJSON) {
			key := normalizeMomentumBeatKey(beat)
			if key == "" {
				continue
			}
			entry := seen[key]
			if entry == nil {
				entry = &duplicateBeat{label: beat}
				seen[key] = entry
			}
			entry.count++
			entry.ids = append(entry.ids, item.ID)
		}
	}
	out := []map[string]any{}
	for _, entry := range seen {
		if entry.count < 2 {
			continue
		}
		out = append(out, momentumItem(entry.label, "storyline_key_point", 0, entry.label, 40+entry.count, map[string]any{
			"duplicate_count": entry.count,
			"storyline_ids":   entry.ids,
			"reason":          "duplicate_key_point",
		}))
	}
	sort.SliceStable(out, func(i, j int) bool {
		return momentumPriority(out[i]) > momentumPriority(out[j])
	})
	if len(out) > 4 {
		out = out[:4]
	}
	return out
}

func momentumItem(label, sourceType string, sourceID int64, sourceName string, priority int, extra map[string]any) map[string]any {
	item := map[string]any{
		"label":       truncateRunes(strings.TrimSpace(label), 180),
		"source_type": sourceType,
		"source_id":   sourceID,
		"source_name": truncateRunes(strings.TrimSpace(sourceName), 120),
		"priority":    priority,
	}
	for key, value := range extra {
		item[key] = value
	}
	if item["label"] == "" {
		item["label"] = item["source_name"]
	}
	return item
}

func momentumPriority(item map[string]any) int {
	if val, ok := item["priority"].(int); ok {
		return val
	}
	return 0
}

func openNarrativeThreadsOldestFirst(items []store.PendingThread) []store.PendingThread {
	out := openNarrativeThreads(items)
	sort.SliceStable(out, func(i, j int) bool {
		left := firstPositiveInt(out[i].LastSeenTurn, out[i].SourceTurn, out[i].CreatedTurn)
		right := firstPositiveInt(out[j].LastSeenTurn, out[j].SourceTurn, out[j].CreatedTurn)
		if left != right {
			return left < right
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func lastStorylineListItem(raw string) string {
	items := parseStorylineListJSON(raw)
	if len(items) == 0 {
		return ""
	}
	return items[len(items)-1]
}

func normalizeMomentumBeatKey(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return ""
	}
	runes := []rune(text)
	if len(runes) > 20 {
		runes = runes[:20]
	}
	return string(runes)
}

func firstPositiveInt(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func (s *Server) handleNarrativeControlGet(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	ctx := r.Context()

	storylines, _ := s.Store.ListStorylines(ctx, sid)
	pendingThreads, _ := s.Store.ListPendingThreads(ctx, sid, "")
	characters, _ := s.Store.ListCharacterStates(ctx, sid)
	activeStates, _ := s.Store.ListActiveStates(ctx, sid, "")
	worldRules, _ := s.Store.ListWorldRules(ctx, sid)
	storylines = nonNilSlice(storylines)
	pendingThreads = nonNilSlice(pendingThreads)
	characters = nonNilSlice(characters)
	activeStates = nonNilSlice(activeStates)
	worldRules = nonNilSlice(worldRules)

	dbLastTurn := maxNarrativeEvidenceTurn(storylines, pendingThreads, activeStates, characters)

	var storyPlan map[string]any
	var director map[string]any
	var stateStatus string
	var warnings []any
	var fromCache bool

	// K-2b: try cached GuidancePlanState
	if gps, ok := s.Store.(store.GuidancePlanStateStore); ok {
		cached, _ := gps.GetGuidancePlanState(ctx, sid)
		if cached != nil && cached.StateStatus != "empty" && cached.StoryPlanJSON != "" && cached.DirectorJSON != "" {
			isUserPatched := cached.StateStatus == "user_patched"
			forwardFresh := cached.LastTurn >= max(0, dbLastTurn-3)
			backwardFresh := dbLastTurn >= max(0, cached.LastTurn-1)
			cacheFresh := isUserPatched || (forwardFresh && backwardFresh)
			if cacheFresh {
				var cachedStoryPlan map[string]any
				var cachedDirector map[string]any
				if err := json.Unmarshal([]byte(cached.StoryPlanJSON), &cachedStoryPlan); err == nil {
					if err := json.Unmarshal([]byte(cached.DirectorJSON), &cachedDirector); err == nil {
						fromCache = true
						storyPlan = cachedStoryPlan
						director = cachedDirector
						stateStatus = strings.TrimSpace(cached.StateStatus)
						if stateStatus == "" {
							stateStatus = "partial"
						}
						if cached.WarningsJSON != "" {
							_ = json.Unmarshal([]byte(cached.WarningsJSON), &warnings)
						}
					}
				}
			}
		}
	}

	if !fromCache {
		warnings = []any{}
		if len(storylines) == 0 && len(pendingThreads) == 0 {
			warnings = append(warnings, "No active storylines or open hooks found. Returning skeleton state.")
		}

		storyPlan = buildStoryPlanSnapshot(storylines, pendingThreads, characters, worldRules, dbLastTurn)
		director = buildDirectorSnapshot(storylines, pendingThreads, characters, worldRules, dbLastTurn)
		stateStatus = "partial"
		if len(storylines) == 0 && len(pendingThreads) == 0 && len(characters) == 0 && len(activeStates) == 0 && len(worldRules) == 0 {
			stateStatus = "skeleton"
		} else if hasNarrativePlanSignal(storyPlan) && hasDirectorSignal(director) {
			stateStatus = "ready"
		}

		// K-2c: conservative merge with previous cache when same arc
		if gps, ok := s.Store.(store.GuidancePlanStateStore); ok {
			prev, _ := gps.GetGuidancePlanState(ctx, sid)
			if prev != nil && prev.LastTurn > 0 {
				var prevPlan map[string]any
				var prevDirector map[string]any
				_ = json.Unmarshal([]byte(prev.StoryPlanJSON), &prevPlan)
				_ = json.Unmarshal([]byte(prev.DirectorJSON), &prevDirector)
				cachedArc := strings.TrimSpace(asString(prevPlan["current_arc"]))
				currentArc := strings.TrimSpace(asString(storyPlan["current_arc"]))
				if cachedArc != "" && (currentArc == cachedArc || currentArc == "") {
					oldBeats := asAnySlice(prevPlan["next_beats"])
					newBeats := asAnySlice(storyPlan["next_beats"])
					if len(oldBeats) > 0 {
						storyPlan["next_beats"] = unionAnyStringSlices(newBeats, oldBeats)
					}
					oldAnchors := asAnySlice(prevPlan["continuity_anchors"])
					newAnchors := asAnySlice(storyPlan["continuity_anchors"])
					if len(oldAnchors) > 0 {
						storyPlan["continuity_anchors"] = unionAnyStringSlices(newAnchors, oldAnchors)
					}
				}
				director = mergeDirectorPrev(director, prevDirector)
			}
		}

		// K-2b: non-fatal upsert
		if gps, ok := s.Store.(store.GuidancePlanStateStore); ok {
			spJSON, _ := json.Marshal(storyPlan)
			dirJSON, _ := json.Marshal(director)
			warnJSON, _ := json.Marshal(warnings)
			item := &store.GuidancePlanState{
				ChatSessionID: sid,
				StoryPlanJSON: string(spJSON),
				DirectorJSON:  string(dirJSON),
				StateStatus:   stateStatus,
				LastTurn:      dbLastTurn,
				WarningsJSON:  string(warnJSON),
				UpdatedAt:     time.Now().UTC(),
			}
			_ = gps.UpsertGuidancePlanState(ctx, item)
		}
	}

	lastTurnValue := any(nil)
	if dbLastTurn > 0 {
		lastTurnValue = dbLastTurn
	}
	compactHistory, compactMeta := buildNarrativeCompactHistory(storyPlan, director, storylines, pendingThreads)

	writeJSON(w, http.StatusOK, map[string]any{
		"chat_session_id":      sid,
		"compact_history":      compactHistory,
		"compact_history_meta": compactMeta,
		"director":             director,
		"generated_at":         generatedAt(),
		"last_advanced_turn":   lastTurnValue,
		"last_validated_turn":  lastTurnValue,
		"progression_ledger":   buildNarrativeControlProgressionLedger(stateStatus, director, storyPlan, dbLastTurn),
		"skeleton_only":        stateStatus == "skeleton",
		"state_status":         stateStatus,
		"status":               "ok",
		"story_guidance":       buildStoryGuidanceSurface(storyPlan, director),
		"story_plan":           storyPlan,
		"warnings":             warnings,
	})
}

func (s *Server) handleSessionsGet404(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Not Found"})
}

func (s *Server) handleSessionDelete(w http.ResponseWriter, r *http.Request) {
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	if sid == "" {
		writeBadRequest(w, "chat_session_id is required")
		return
	}

	rollbackStore, hasRollback := s.Store.(store.RollbackStore)
	if !hasRollback || !s.usesShadowWriteStore() {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":           "ok",
			"source":           "shadow",
			"chat_session_id":  sid,
			"deleted":          false,
			"mutation_enabled": false,
			"note":             "session delete is a shadow plan; no mutations performed",
		})
		return
	}

	ctx := r.Context()
	if err := rollbackStore.DeleteSession(ctx, sid); err != nil {
		writeInternalError(w, err.Error())
		return
	}

	vectorCleanup := map[string]any{
		"attempted": false,
		"ok":        true,
		"error":     nil,
	}
	if s.Vector != nil {
		vectorCleanup["attempted"] = true
		if err := s.Vector.DeleteSession(ctx, sid); err != nil {
			if errors.Is(err, vector.ErrNotEnabled) {
				vectorCleanup["ok"] = true
				vectorCleanup["error"] = "vector_not_enabled"
				vectorCleanup["warning"] = "vector store is not enabled"
			} else {
				vectorCleanup["ok"] = false
				vectorCleanup["error"] = err.Error()
			}
		}
	}

	status := "ok"
	if vectorCleanup["ok"] == false {
		status = "partial_error"
	}
	s.saveAuditLogBestEffort(ctx, &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "session_delete",
		TargetType:    "session",
		TargetID:      0,
		Summary:       "Session deleted",
		DetailsJSON:   mustCompactJSON(map[string]any{"vector_cleanup": vectorCleanup, "status": status}),
		Source:        s.storeWriteSource(),
		CreatedAt:     time.Now().UTC(),
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"status":           status,
		"source":           s.storeWriteSource(),
		"chat_session_id":  sid,
		"deleted":          true,
		"mutation_enabled": true,
		"vector_cleanup":   vectorCleanup,
		"note":             "session deleted",
	})
}

func (s *Server) handleSessionGet404(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotFound, map[string]any{"detail": "Not Found"})
}

// Session write guards (R2)

func (s *Server) handleActiveScopePatch(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if strings.TrimSpace(sid) == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}
	activeStore, ok := s.Store.(store.ActiveScopeStore)
	if !ok {
		writeShadowGuard(w, "PATCH /session/{chat_session_id}/active-scope")
		return
	}
	payload, err := decodeNarrativeJSONMap(r)
	if err != nil {
		writeBadRequest(w, "invalid JSON body")
		return
	}
	activeScope := strings.TrimSpace(extractionStringFromAny(payload["active_scope"]))
	if activeScope == "" {
		activeScope = "root"
	}
	if !isValidWorldRuleScope(activeScope) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"status": "error",
			"detail": "active_scope must be one of [root region location faction system session]",
		})
		return
	}
	scopeName := strings.TrimSpace(extractionStringFromAny(payload["scope_name"]))
	item := &store.SessionActiveScope{
		ChatSessionID: sid,
		ActiveScope:   activeScope,
		ScopeName:     scopeName,
		UpdatedAt:     time.Now().UTC(),
	}
	if err := activeStore.UpsertActiveScope(r.Context(), item); err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, "PATCH /session/{chat_session_id}/active-scope")
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, activeScopeResponse(sid, item, "store"))
}

func (s *Server) handleDirectorPatch(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if strings.TrimSpace(sid) == "" {
		writeBadRequest(w, "chat_session_id is required")
		return
	}

	gps, ok := s.Store.(store.GuidancePlanStateStore)
	if !ok {
		writeShadowGuard(w, "PATCH /narrative-control/{chat_session_id}/director-patch")
		return
	}

	payload, err := decodeNarrativeJSONMap(r)
	if err != nil {
		writeBadRequest(w, "invalid JSON body")
		return
	}

	ctx := r.Context()
	cached, err := gps.GetGuidancePlanState(ctx, sid)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, "PATCH /narrative-control/{chat_session_id}/director-patch")
			return
		}
		writeInternalError(w, err.Error())
		return
	}

	var director map[string]any
	if cached != nil && cached.DirectorJSON != "" {
		_ = json.Unmarshal([]byte(cached.DirectorJSON), &director)
	}
	if director == nil {
		director = map[string]any{}
	}

	// K-3d: apply allowed patchable fields
	patchable := []string{
		"scene_mandate", "required_outcomes", "forbidden_moves",
		"pressure_level", "resolved_outcomes", "expired_forbidden",
		"execution_checklist", "persona_guardrails", "world_guardrails",
		"focus_characters",
	}
	for _, key := range patchable {
		if v, ok := payload[key]; ok {
			director[key] = v
		}
	}

	// Build updated state preserving story plan and warnings
	dirJSON, _ := json.Marshal(director)
	item := &store.GuidancePlanState{
		ChatSessionID: sid,
		DirectorJSON:  string(dirJSON),
		StateStatus:   "user_patched",
		LastTurn:      0,
	}
	if cached != nil {
		item.StoryPlanJSON = cached.StoryPlanJSON
		item.WarningsJSON = cached.WarningsJSON
		item.LastTurn = cached.LastTurn
	}

	if err := gps.UpsertGuidancePlanState(ctx, item); err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, "PATCH /narrative-control/{chat_session_id}/director-patch")
			return
		}
		writeInternalError(w, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"director":        director,
		"patched":         true,
		"state_status":    "user_patched",
	})
}
