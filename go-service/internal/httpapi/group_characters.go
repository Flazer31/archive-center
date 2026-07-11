package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// Character: R1 read, R2 write

func (s *Server) handleCharactersGet(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}
	items, err := s.Store.ListCharacterStates(r.Context(), sid)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			items = []store.CharacterState{}
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	items = nonNilSlice(items)
	events, err := s.Store.ListCharacterEvents(r.Context(), sid, "")
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			events = []store.CharacterEvent{}
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	events = nonNilSlice(events)
	referenceTurn := s.characterReferenceTurn(r.Context(), sid, items)
	recentMentionText, recentMentionKeywords := s.characterRecentMentionSignal(r.Context(), sid, referenceTurn)
	characters := characterResponseItems(items, events, referenceTurn, recentMentionText, recentMentionKeywords)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"characters":      characters,
		"count":           len(characters),
		"omitted_count":   characterOmittedCount(items, events, referenceTurn, recentMentionText, recentMentionKeywords),
	})
}

func (s *Server) characterReferenceTurn(ctx context.Context, sid string, characters []store.CharacterState) int {
	ref := 0
	if s.Store != nil {
		if logs, err := s.Store.ListChatLogs(ctx, sid, 0, 0); err == nil {
			for _, log := range logs {
				if log.TurnIndex > ref {
					ref = log.TurnIndex
				}
			}
		}
	}
	for _, ch := range characters {
		if ch.TurnIndex > ref {
			ref = ch.TurnIndex
		}
	}
	return ref
}

func (s *Server) characterRecentMentionSignal(ctx context.Context, sid string, referenceTurn int) (string, map[string]struct{}) {
	if s.Store == nil || sid == "" || referenceTurn <= 0 {
		return "", map[string]struct{}{}
	}
	fromTurn := referenceTurn - 2
	if fromTurn < 0 {
		fromTurn = 0
	}
	logs, err := s.Store.ListChatLogs(ctx, sid, fromTurn, 0)
	if err != nil {
		return "", map[string]struct{}{}
	}
	sort.SliceStable(logs, func(i, j int) bool {
		if logs[i].TurnIndex != logs[j].TurnIndex {
			return logs[i].TurnIndex > logs[j].TurnIndex
		}
		return logs[i].ID > logs[j].ID
	})
	if len(logs) > 8 {
		logs = logs[:8]
	}
	parts := []string{}
	for _, log := range logs {
		if text := strings.TrimSpace(log.Content); text != "" {
			parts = append(parts, text)
		}
	}
	recentText := strings.Join(parts, " ")
	return recentText, extractCharacterRecentKeywords(recentText)
}

func characterRecentMentionSignalFromLogs(logs []store.ChatLog, referenceTurn int) (string, map[string]struct{}) {
	if referenceTurn <= 0 {
		return "", map[string]struct{}{}
	}
	fromTurn := referenceTurn - 2
	if fromTurn < 0 {
		fromTurn = 0
	}
	filtered := make([]store.ChatLog, 0, len(logs))
	for _, log := range logs {
		if log.TurnIndex < fromTurn {
			continue
		}
		filtered = append(filtered, log)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].TurnIndex != filtered[j].TurnIndex {
			return filtered[i].TurnIndex > filtered[j].TurnIndex
		}
		return filtered[i].ID > filtered[j].ID
	})
	if len(filtered) > 8 {
		filtered = filtered[:8]
	}
	parts := []string{}
	for _, log := range filtered {
		if text := strings.TrimSpace(log.Content); text != "" {
			parts = append(parts, text)
		}
	}
	recentText := strings.Join(parts, " ")
	return recentText, extractCharacterRecentKeywords(recentText)
}

func characterResponseItems(items []store.CharacterState, events []store.CharacterEvent, referenceTurn int, recentMentionText string, recentMentionKeywords map[string]struct{}) []map[string]any {
	latest := latestCharacterStatesByName(items)
	out := []map[string]any{}
	for _, item := range latest {
		recentEvents := recentCharacterEvents(events, item.CharacterName, 8)
		snapshot := characterStaleSnapshot(item, recentEvents, referenceTurn, recentMentionText, recentMentionKeywords)
		if stale, _ := snapshot["is_stale"].(bool); stale {
			continue
		}
		var latestEvent *store.CharacterEvent
		if len(recentEvents) > 0 {
			latestEvent = &recentEvents[0]
		}
		out = append(out, characterResponseItem(item, snapshot, latestEvent, recentEvents))
	}
	return out
}

func characterOmittedCount(items []store.CharacterState, events []store.CharacterEvent, referenceTurn int, recentMentionText string, recentMentionKeywords map[string]struct{}) int {
	omitted := 0
	for _, item := range latestCharacterStatesByName(items) {
		snapshot := characterStaleSnapshot(item, recentCharacterEvents(events, item.CharacterName, 8), referenceTurn, recentMentionText, recentMentionKeywords)
		if stale, _ := snapshot["is_stale"].(bool); stale {
			omitted++
		}
	}
	return omitted
}

func latestCharacterStatesByName(items []store.CharacterState) []store.CharacterState {
	byName := map[string]store.CharacterState{}
	for _, item := range items {
		name := strings.TrimSpace(item.CharacterName)
		if name == "" {
			continue
		}
		current, ok := byName[name]
		if !ok || item.TurnIndex > current.TurnIndex || (item.TurnIndex == current.TurnIndex && item.ID > current.ID) {
			byName[name] = item
		}
	}
	out := make([]store.CharacterState, 0, len(byName))
	for _, item := range byName {
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CharacterName < out[j].CharacterName })
	return out
}

func characterResponseItem(item store.CharacterState, snapshot map[string]any, latestEvent *store.CharacterEvent, recentEvents []store.CharacterEvent) map[string]any {
	relationshipLane := buildCharacterRelationshipLane(item)
	latestAnchor := buildCharacterLatestInteractionAnchor(latestEvent)
	stableSheet := buildStableCharacterSheet(item, snapshot)
	dynamicDigest := buildDynamicCharacterDigest(item, snapshot, relationshipLane, latestAnchor, recentEvents)
	return map[string]any{
		"id":                        item.ID,
		"chat_session_id":           item.ChatSessionID,
		"character_name":            item.CharacterName,
		"appearance_json":           nullableJSONString(item.AppearanceJSON),
		"personality_json":          nullableJSONString(item.PersonalityJSON),
		"status_json":               nullableJSONString(item.StatusJSON),
		"relationships_json":        nullableJSONString(item.RelationshipsJSON),
		"speech_style_json":         nullableJSONString(item.SpeechStyleJSON),
		"turn_index":                item.TurnIndex,
		"last_observed_turn":        snapshot["last_observed_turn"],
		"freshness_turn_gap":        snapshot["freshness_turn_gap"],
		"stale_after_turns":         snapshot["stale_after_turns"],
		"is_stale":                  snapshot["is_stale"],
		"stale_reason":              snapshot["stale_reason"],
		"admission_class":           snapshot["admission_class"],
		"admission_basis":           snapshot["admission_basis"],
		"continuity_anchor_types":   snapshot["continuity_anchor_types"],
		"recent_event_count":        snapshot["recent_event_count"],
		"stale_guard":               snapshot["stale_guard"],
		"stable_character_sheet":    stableSheet,
		"dynamic_continuity_digest": dynamicDigest,
		"relationship_lane":         relationshipLane,
		"latest_interaction_anchor": latestAnchor,
		"created_at":                formatNaiveUTCTime(item.CreatedAt),
		"updated_at":                formatNaiveUTCTime(item.UpdatedAt),
	}
}

func characterStaleSnapshot(item store.CharacterState, recentEvents []store.CharacterEvent, referenceTurn int, recentMentionText string, recentMentionKeywords map[string]struct{}) map[string]any {
	lastObserved := item.TurnIndex
	gapInt := 0
	var gap any
	if referenceTurn > 0 && lastObserved > 0 {
		gapInt = referenceTurn - lastObserved
		if gapInt < 0 {
			gapInt = 0
		}
		gap = gapInt
	}
	anchors := []string{}
	if strings.TrimSpace(item.AppearanceJSON) != "" {
		anchors = append(anchors, "appearance")
	}
	if strings.TrimSpace(item.PersonalityJSON) != "" {
		anchors = append(anchors, "personality")
	}
	if strings.TrimSpace(item.RelationshipsJSON) != "" {
		anchors = append(anchors, "relationships")
	}
	if strings.TrimSpace(item.SpeechStyleJSON) != "" {
		anchors = append(anchors, "speech_style")
	}
	for _, ev := range recentEvents {
		eventType := strings.TrimSpace(ev.EventType)
		if eventType == "relationship_shift" || eventType == "personality_change" {
			anchors = appendUniqueString(anchors, "event_anchor")
			break
		}
	}
	hasAnchor := len(anchors) > 0
	staleAfter := 3
	descriptorLike := looksLikeTransientCharacterName(item.CharacterName)
	recentlyRementioned := descriptorRecentlyRementioned(item.CharacterName, recentMentionText, recentMentionKeywords)
	isStale := descriptorLike && referenceTurn > 0 && gapInt >= staleAfter && !hasAnchor && !recentlyRementioned
	staleReason := any(nil)
	if isStale {
		staleReason = "transient_descriptor_not_rementioned"
	}
	admissionClass := "lightweight_named"
	if hasAnchor || recentlyRementioned || len(recentEvents) >= 2 {
		admissionClass = "major_recurring"
	} else if descriptorLike {
		admissionClass = "transient_descriptor"
	}
	admissionBasis := make([]string, len(anchors))
	copy(admissionBasis, anchors)
	recentEventCount := len(recentEvents)
	if recentEventCount > 3 {
		recentEventCount = 3
	}
	if recentlyRementioned {
		admissionBasis = append(admissionBasis, "recent_remention")
	}
	if len(recentEvents) >= 2 {
		admissionBasis = append(admissionBasis, "recent_event_history")
	}
	return map[string]any{
		"last_observed_turn":      nullablePositiveInt(lastObserved),
		"freshness_turn_gap":      gap,
		"stale_after_turns":       staleAfter,
		"is_stale":                isStale,
		"stale_reason":            staleReason,
		"admission_class":         admissionClass,
		"admission_basis":         admissionBasis,
		"continuity_anchor_types": anchors,
		"recent_event_count":      recentEventCount,
		"stale_guard": map[string]any{
			"active":                         isStale || (referenceTurn > 0 && gapInt >= staleAfter && !hasAnchor),
			"reason":                         staleReasonIfNeeded(staleReason, referenceTurn, gapInt, staleAfter, hasAnchor),
			"allow_weak_input_carry_forward": !isStale && (hasAnchor || recentlyRementioned),
			"admission_class":                admissionClass,
			"admission_basis":                admissionBasis,
		},
	}
}

func nullableJSONString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func staleReasonIfNeeded(staleReason any, referenceTurn, gapInt, staleAfter int, hasAnchor bool) any {
	if staleReason != nil {
		return staleReason
	}
	if referenceTurn > 0 && gapInt >= staleAfter && !hasAnchor {
		return "low_anchor_freshness_gap"
	}
	return nil
}

func descriptorRecentlyRementioned(name string, recentText string, recentKeywords map[string]struct{}) bool {
	rawName := normalizeCharacterDescriptorText(name)
	rawRecentText := normalizeCharacterDescriptorText(recentText)
	if rawName != "" && strings.Contains(rawRecentText, rawName) {
		return true
	}
	nameKeywords := extractCharacterDescriptorKeywords(name)
	genericHit := false
	for token := range characterGenericDescriptorTokens() {
		if _, ok := recentKeywords[token]; ok {
			genericHit = true
			break
		}
		if rawRecentText != "" && strings.Contains(rawRecentText, token) {
			genericHit = true
			break
		}
	}
	if !genericHit {
		return false
	}
	nonGeneric := []string{}
	generic := characterGenericDescriptorTokens()
	for _, token := range nameKeywords {
		if _, ok := generic[token]; !ok {
			nonGeneric = append(nonGeneric, token)
		}
	}
	if len(nonGeneric) == 0 {
		return true
	}
	overlap := 0
	for _, token := range nonGeneric {
		if _, ok := recentKeywords[token]; ok {
			overlap++
		}
	}
	required := 1
	if len(nonGeneric) > 1 {
		required = 2
		if len(nonGeneric) < required {
			required = len(nonGeneric)
		}
	}
	return overlap >= required
}

func extractCharacterRecentKeywords(text string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, token := range splitCharacterDescriptorTokens(text) {
		if len([]rune(token)) >= characterKeywordMinLength(token) {
			out[token] = struct{}{}
		}
	}
	return out
}

func extractCharacterDescriptorKeywords(name string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, token := range splitCharacterDescriptorTokens(name) {
		if token == "" || characterDescriptorStopwords()[token] {
			continue
		}
		if len([]rune(token)) < characterKeywordMinLength(token) {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		out = append(out, token)
	}
	return out
}

func splitCharacterDescriptorTokens(text string) []string {
	return strings.FieldsFunc(strings.ToLower(strings.TrimSpace(text)), func(r rune) bool {
		switch r {
		case ' ', '\t', '\r', '\n', '.', ',', '!', '?', ':', ';', '(', ')', '[', ']', '{', '}', '/', '|', '\\', '"', '\'', '`', '~', '@', '#', '$', '%', '^', '&', '*', '+', '=', '<', '>', '-', '_':
			return true
		default:
			return false
		}
	})
}

func normalizeCharacterDescriptorText(text string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(text))), " ")
}

func characterKeywordMinLength(token string) int {
	for _, r := range token {
		if (r >= '\uAC00' && r <= '\uD7A3') || (r >= '\u1100' && r <= '\u11FF') {
			return 2
		}
	}
	return 3
}

func characterGenericDescriptorTokens() map[string]struct{} {
	return map[string]struct{}{
		"woman": {}, "man": {}, "girl": {}, "boy": {}, "lady": {}, "gentleman": {}, "stranger": {}, "figure": {}, "person": {}, "voice": {},
	}
}

func characterDescriptorStopwords() map[string]bool {
	return map[string]bool{"a": true, "an": true, "the": true, "this": true, "that": true, "these": true, "those": true, "in": true, "on": true, "at": true, "of": true}
}

func looksLikeTransientCharacterName(name string) bool {
	text := strings.TrimSpace(name)
	if text == "" {
		return true
	}
	lower := strings.ToLower(text)
	transientTokens := []string{"unknown", "unnamed", "npc", "woman", "man", "girl", "boy", "person", "voice", "figure", "descriptor"}
	for _, token := range transientTokens {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return strings.Count(text, " ") >= 2
}

func recentCharacterEvents(events []store.CharacterEvent, characterName string, limit int) []store.CharacterEvent {
	filtered := []store.CharacterEvent{}
	for _, ev := range events {
		if ev.CharacterName == characterName {
			filtered = append(filtered, ev)
		}
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].TurnIndex != filtered[j].TurnIndex {
			return filtered[i].TurnIndex > filtered[j].TurnIndex
		}
		if !filtered[i].CreatedAt.Equal(filtered[j].CreatedAt) {
			return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
		}
		return filtered[i].ID > filtered[j].ID
	})
	if limit > 0 && len(filtered) > limit {
		return filtered[:limit]
	}
	return filtered
}

func parseSurfacePayload(raw string) any {
	text := strings.TrimSpace(raw)
	if text == "" {
		return nil
	}
	var parsed any
	if err := json.Unmarshal([]byte(text), &parsed); err == nil {
		return parsed
	}
	return text
}

func hasSurfaceValue(value any) bool {
	if value == nil {
		return false
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) != ""
	case []any:
		return len(v) > 0
	case map[string]any:
		return len(v) > 0
	default:
		rv := reflect.ValueOf(value)
		switch rv.Kind() {
		case reflect.Slice, reflect.Array, reflect.Map:
			return rv.Len() > 0
		}
		return true
	}
}

func surfaceStatus(payloads map[string]any) string {
	filled := 0
	for _, value := range payloads {
		if hasSurfaceValue(value) {
			filled++
		}
	}
	if filled == 0 {
		return "empty"
	}
	if filled == len(payloads) {
		return "ready"
	}
	return "partial"
}

func filledAxes(payloads map[string]any) []string {
	out := []string{}
	for _, key := range sortedMapKeys(payloads) {
		if hasSurfaceValue(payloads[key]) {
			out = append(out, key)
		}
	}
	return out
}

func filledAxesInOrder(payloads map[string]any, order ...string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, key := range order {
		seen[key] = true
		if hasSurfaceValue(payloads[key]) {
			out = append(out, key)
		}
	}
	for _, key := range sortedMapKeys(payloads) {
		if seen[key] {
			continue
		}
		if hasSurfaceValue(payloads[key]) {
			out = append(out, key)
		}
	}
	return out
}

func sortedMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func formatNaiveUTCTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC().Format("2006-01-02 15:04:05")
}

func cleanShadowText(value any, maxLen int) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return truncateRunes(strings.Join(strings.Fields(v), " "), maxLen)
	case float64, bool, int, int64:
		return truncateRunes(strings.TrimSpace(compactJSONForShadow(v, maxLen)), maxLen)
	case map[string]any, []any:
		return truncateRunes(strings.TrimSpace(compactJSONForShadow(v, maxLen)), maxLen)
	default:
		return truncateRunes(strings.Join(strings.Fields(strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(compactJSONForShadow(v, maxLen)), "\n", " "), "\t", " "))), " "), maxLen)
	}
}

func preferredSummaryText(payload any, maxLen int, keys ...string) string {
	if m, ok := payload.(map[string]any); ok {
		parts := []string{}
		rawTextKeys := map[string]bool{"summary": true, "summary_text": true, "detail": true, "note": true, "message": true, "interaction": true}
		for _, key := range keys {
			text := cleanShadowText(m[key], 90)
			if text == "" {
				continue
			}
			if rawTextKeys[key] {
				parts = append(parts, text)
			} else {
				parts = append(parts, key+": "+text)
			}
			if len(parts) >= 3 {
				break
			}
		}
		if len(parts) > 0 {
			return truncateRunes(strings.Join(parts, "; "), maxLen)
		}
	}
	return cleanShadowText(payload, maxLen)
}

func buildCharacterRelationshipLane(item store.CharacterState) map[string]any {
	payload := parseSurfacePayload(item.RelationshipsJSON)
	protagonistItems := []map[string]any{}
	otherItems := []map[string]any{}
	seen := map[string]bool{}
	var protagonist map[string]any
	appendItem := func(targetRaw any, relation any) {
		target := relationDisplayTarget(targetRaw)
		if target == "" {
			return
		}
		summary := preferredSummaryText(relation, 180, "summary", "summary_text", "status", "state", "detail", "note", "trust", "closeness", "tension")
		if summary == "" {
			return
		}
		key := strings.ToLower(target + "\x00" + summary)
		if seen[key] {
			return
		}
		seen[key] = true
		isPlayer := isPlayerReference(targetRaw)
		relationMap := map[string]any{"value": relation}
		if m, ok := relation.(map[string]any); ok {
			relationMap = m
		}
		entry := map[string]any{
			"target":           target,
			"summary_text":     summary,
			"state_snapshot":   projectRelationPayload(relationMap),
			"descriptor_bands": relationDescriptorBands(relationMap),
			"display_priority": 1,
		}
		if isPlayer {
			entry["display_priority"] = 0
			protagonistItems = append(protagonistItems, entry)
			if protagonist == nil {
				protagonist = entry
			}
			return
		}
		otherItems = append(otherItems, entry)
	}
	switch v := payload.(type) {
	case []any:
		for _, raw := range v {
			if m, ok := raw.(map[string]any); ok {
				appendItem(firstPresentValue(m, "target", "name", "character_name", "title", "scope_name"), m)
			}
		}
	case map[string]any:
		for _, key := range sortedMapKeys(v) {
			value := v[key]
			target := any(key)
			if m, ok := value.(map[string]any); ok {
				if tv := firstPresentValue(m, "target", "name", "character_name"); tv != nil {
					target = tv
				}
			}
			appendItem(target, value)
		}
	}
	ordered := append([]map[string]any{}, protagonistItems...)
	ordered = append(ordered, otherItems...)
	if len(ordered) > 6 {
		ordered = ordered[:6]
	}
	preferred := protagonist
	if preferred == nil && len(ordered) > 0 {
		preferred = ordered[0]
	}
	secondary := []map[string]any{}
	for _, entry := range ordered {
		if preferred != nil && entry["target"] == preferred["target"] && entry["summary_text"] == preferred["summary_text"] {
			continue
		}
		secondary = append(secondary, entry)
	}
	summary := ""
	if preferred != nil {
		summary, _ = preferred["summary_text"].(string)
	}
	if summary == "" {
		summary = preferredSummaryText(payload, 180, "summary", "summary_text", "status", "state", "detail", "note", "trust", "closeness", "tension")
	}
	status := "empty"
	if len(ordered) > 0 {
		status = "ready"
	} else if summary != "" {
		status = "summary_only"
	}
	descriptorSummary := ""
	if preferred != nil {
		if bands, ok := preferred["descriptor_bands"].([]string); ok {
			descriptorSummary = truncateRunes(strings.Join(bands, "; "), 180)
		}
	}
	return map[string]any{
		"surface_version":          "rl14a.v1",
		"surface_type":             "relationship_lane",
		"status":                   status,
		"display_mode":             "protagonist_first_then_observed_order",
		"count":                    len(protagonistItems) + len(otherItems),
		"summary_text":             nullableString(summary),
		"descriptor_summary":       descriptorSummary,
		"primary_target":           mapStringOrNil(preferred, "target"),
		"primary_descriptor_bands": mapAnyOrEmptyStringSlice(preferred, "descriptor_bands"),
		"protagonist_relation":     protagonist,
		"other_relations":          limitMapSlice(secondary, 5),
		"items":                    mapSliceToAny(ordered),
	}
}

func buildCharacterLatestInteractionAnchor(event *store.CharacterEvent) any {
	if event == nil {
		return nil
	}
	details := parseSurfacePayload(event.DetailsJSON)
	summary := preferredSummaryText(details, 180, "interaction", "detail", "summary", "summary_text", "note", "message", "status", "change")
	if summary == "" {
		summary = cleanShadowText(event.EventType, 80)
	}
	return map[string]any{
		"surface_version": "rl14b.v1",
		"surface_type":    "latest_interaction_anchor",
		"status":          "ready",
		"event_type":      event.EventType,
		"turn_index":      event.TurnIndex,
		"summary_text":    summary,
		"details":         details,
		"created_at":      formatNaiveUTCTime(event.CreatedAt),
	}
}

func buildStableCharacterSheet(item store.CharacterState, snapshot map[string]any) map[string]any {
	appearance := parseSurfacePayload(item.AppearanceJSON)
	personality := parseSurfacePayload(item.PersonalityJSON)
	speechStyle := parseSurfacePayload(item.SpeechStyleJSON)
	appearanceCore, appearanceSnapshot := splitAppearancePayload(appearance)
	appearanceObservable, appearanceNonObservable := splitObservableAppearancePayload(appearanceCore)
	axes := map[string]any{"appearance": appearanceCore, "personality": personality, "speech_style": speechStyle}
	return map[string]any{
		"surface_version":           "cc14a.v1",
		"surface_type":              "stable_character_sheet",
		"status":                    surfaceStatus(axes),
		"filled_axes":               filledAxes(axes),
		"appearance":                appearance,
		"appearance_core":           appearanceCore,
		"appearance_observable":     appearanceObservable,
		"appearance_non_observable": appearanceNonObservable,
		"appearance_snapshot_keys":  sortedMapKeys(appearanceSnapshot),
		"personality":               personality,
		"speech_style":              speechStyle,
		"durable_profile": map[string]any{
			"appearance":   appearanceObservable,
			"personality":  personality,
			"speech_style": speechStyle,
		},
		"sparse_policy": map[string]any{
			"mode":              "omit_unknown_fields",
			"filled_axes":       filledAxes(axes),
			"empty_axes":        emptyAxes(axes),
			"dynamic_redirects": []string{"current_status", "relationship_lane", "latest_interaction_anchor", "appearance_snapshot"},
		},
		"source_turn": snapshot["last_observed_turn"],
	}
}

func buildDynamicCharacterDigest(item store.CharacterState, snapshot map[string]any, relationshipLane map[string]any, latestAnchor any, recentEvents []store.CharacterEvent) map[string]any {
	currentStatus := parseSurfacePayload(item.StatusJSON)
	appearance := parseSurfacePayload(item.AppearanceJSON)
	_, appearanceSnapshot := splitAppearancePayload(appearance)
	currentStatusSummary := ""
	if m, ok := currentStatus.(map[string]any); ok {
		currentStatusSummary = preferredSummaryText(m, 180, "location", "emotion", "goal", "status", "state", "condition", "mood")
	}
	relationshipItems := mapAnySlice(relationshipLane["items"])
	relationshipDescriptorLane := []map[string]any{}
	for _, entry := range relationshipItems[:minInt(len(relationshipItems), 4)] {
		relationshipDescriptorLane = append(relationshipDescriptorLane, map[string]any{
			"target":           entry["target"],
			"summary_text":     entry["summary_text"],
			"descriptor_bands": mapAnyOrEmptyStringSlice(entry, "descriptor_bands"),
		})
	}
	var preferredRelation map[string]any
	if pr, ok := relationshipLane["protagonist_relation"].(map[string]any); ok && pr != nil {
		preferredRelation = pr
	} else if len(relationshipItems) > 0 {
		preferredRelation = relationshipItems[0]
	}
	var relationshipFocus map[string]any
	if preferredRelation != nil {
		relationshipFocus = compactMap(map[string]any{
			"target":           mapStringOrNil(preferredRelation, "target"),
			"summary_text":     mapStringOrNil(preferredRelation, "summary_text"),
			"descriptor_bands": mapAnyOrEmptyStringSlice(preferredRelation, "descriptor_bands"),
		})
	}
	var relationshipSurface any
	if preferredRelation != nil {
		relationshipSurface = preferredRelation
	} else if len(relationshipItems) > 0 {
		relationshipSurface = relationshipItems
	} else if st, ok := relationshipLane["summary_text"].(string); ok && strings.TrimSpace(st) != "" {
		relationshipSurface = st
	}
	axes := map[string]any{"current_status": currentStatus, "relationship_surface": relationshipSurface, "latest_interaction_anchor": latestAnchor}
	return map[string]any{
		"surface_version":              "cc14b.v1",
		"surface_type":                 "dynamic_continuity_digest",
		"status":                       surfaceStatus(axes),
		"filled_axes":                  filledAxesInOrder(axes, "current_status", "relationship_surface", "latest_interaction_anchor"),
		"admission_class":              snapshot["admission_class"],
		"admission_basis":              snapshot["admission_basis"],
		"stale_guard":                  snapshot["stale_guard"],
		"current_status":               currentStatus,
		"current_status_summary":       nullableString(currentStatusSummary),
		"current_snapshot":             compactMap(map[string]any{"status": currentStatus, "appearance": appearanceSnapshot, "relationship_focus": relationshipFocus}),
		"appearance_snapshot":          appearanceSnapshot,
		"relationship_summary_text":    relationshipLane["summary_text"],
		"relationship_primary_target":  relationshipLane["primary_target"],
		"relationship_display_mode":    relationshipLane["display_mode"],
		"protagonist_relation":         relationshipLane["protagonist_relation"],
		"relationship_lane":            relationshipLane["items"],
		"other_relations":              relationshipLane["other_relations"],
		"relationship_descriptor_lane": mapSliceToAny(relationshipDescriptorLane),
		"latest_interaction_anchor":    latestAnchor,
		"milestone_ledger":             characterMilestoneLedger(recentEvents),
		"digest_budget": map[string]any{
			"policy":                     "priority_capped",
			"relationship_lane_cap":      4,
			"milestone_cap":              3,
			"milestone_read_window":      8,
			"milestone_selection_policy": "latest_plus_priority_events",
			"relationship_lane_used":     len(relationshipDescriptorLane),
			"milestones_used":            len(characterMilestoneLedger(recentEvents)),
		},
		"recent_change_summary": recentChangeSummary(latestAnchor),
		"source_turn":           snapshot["last_observed_turn"],
	}
}

func firstPresentValue(m map[string]any, keys ...string) any {
	for _, key := range keys {
		if v, ok := m[key]; ok && hasSurfaceValue(v) {
			return v
		}
	}
	return nil
}

func isPlayerReference(value any) bool {
	text := strings.ToLower(cleanShadowText(value, 60))
	switch text {
	case "__player__", "{{user}}", "user", "player", "participant":
		return true
	default:
		return false
	}
}

func relationDisplayTarget(value any) string {
	if isPlayerReference(value) {
		return "{{user}}"
	}
	return cleanShadowText(value, 60)
}

func projectRelationPayload(payload any) any {
	switch v := payload.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, value := range v {
			if key == "target" || key == "name" || key == "character_name" || key == "owner" || key == "subject" || key == "object" || key == "from" || key == "to" {
				if display := relationDisplayTarget(value); display != "" {
					out[key] = display
					continue
				}
			}
			out[key] = projectRelationPayload(value)
		}
		return out
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, projectRelationPayload(item))
		}
		return out
	case string:
		if isPlayerReference(v) {
			return "{{user}}"
		}
		return v
	default:
		return v
	}
}

func relationDescriptorBands(payload map[string]any) []string {
	keys := []string{"trust", "closeness", "tension", "bond", "distance", "stance"}
	out := []string{}
	for _, key := range keys {
		if text := cleanShadowText(payload[key], 60); text != "" {
			out = append(out, key+": "+text)
		}
		if len(out) >= 3 {
			break
		}
	}
	return out
}

func splitAppearancePayload(payload any) (any, map[string]any) {
	m, ok := payload.(map[string]any)
	if !ok {
		return payload, map[string]any{}
	}
	durable := map[string]any{}
	snapshot := map[string]any{}
	snapshotTokens := []string{
		"outfit", "clothes", "clothing", "uniform", "coat", "jacket", "dress", "armor", "accessory",
		"expression", "posture", "condition", "injury", "blood", "mud", "wet",
	}
	for key, value := range m {
		normalized := strings.ToLower(strings.ReplaceAll(key, " ", ""))
		isSnapshot := false
		for _, token := range snapshotTokens {
			if strings.Contains(normalized, token) {
				isSnapshot = true
				break
			}
		}
		if isSnapshot {
			snapshot[key] = value
			continue
		}
		durable[key] = value
	}
	if len(durable) == 0 {
		return payload, snapshot
	}
	return durable, snapshot
}

func splitObservableAppearancePayload(payload any) (any, map[string]any) {
	m, ok := payload.(map[string]any)
	if !ok {
		return payload, map[string]any{}
	}
	observable := map[string]any{}
	nonObservable := map[string]any{}
	for key, value := range m {
		lower := strings.ToLower(strings.ReplaceAll(key, " ", ""))
		if strings.Contains(lower, "thought") || strings.Contains(lower, "emotion") || strings.Contains(lower, "feeling") || strings.Contains(lower, "internal") {
			nonObservable[key] = value
			continue
		}
		observable[key] = value
	}
	return observable, nonObservable
}

func emptyAxes(payloads map[string]any) []string {
	out := []string{}
	for _, key := range sortedMapKeys(payloads) {
		if !hasSurfaceValue(payloads[key]) {
			out = append(out, key)
		}
	}
	return out
}

func compactMap(payload map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range payload {
		if hasSurfaceValue(value) {
			out[key] = value
		}
	}
	return out
}

func characterMilestoneLedger(events []store.CharacterEvent) []any {
	candidates := []map[string]any{}
	for recencyIndex, ev := range events {
		details := parseSurfacePayload(ev.DetailsJSON)
		summary := preferredSummaryText(details, 180, "interaction", "detail", "summary", "summary_text", "note", "message", "status", "change")
		if summary == "" {
			summary = cleanShadowText(ev.EventType, 80)
		}
		if summary == "" {
			continue
		}
		priority := characterEventPriority(ev.EventType)
		candidates = append(candidates, map[string]any{
			"event_type":      ev.EventType,
			"turn_index":      ev.TurnIndex,
			"summary_text":    summary,
			"details":         details,
			"created_at":      formatNaiveUTCTime(ev.CreatedAt),
			"_event_priority": priority,
			"_recency_index":  recencyIndex,
		})
	}
	if len(candidates) == 0 {
		return []any{}
	}

	selected := []map[string]any{}
	seen := map[string]bool{}
	appendCandidate := func(candidate map[string]any) {
		key := fmt.Sprintf("%v|%v|%v", candidate["event_type"], candidate["turn_index"], candidate["summary_text"])
		if seen[key] {
			return
		}
		seen[key] = true
		selected = append(selected, candidate)
	}

	appendCandidate(candidates[0])
	if len(candidates) > 1 {
		remaining := make([]map[string]any, len(candidates[1:]))
		copy(remaining, candidates[1:])
		sort.SliceStable(remaining, func(i, j int) bool {
			pi, _ := remaining[i]["_event_priority"].(int)
			pj, _ := remaining[j]["_event_priority"].(int)
			ri, _ := remaining[i]["_recency_index"].(int)
			rj, _ := remaining[j]["_recency_index"].(int)
			if pi != pj {
				return pi < pj
			}
			return ri < rj
		})
		for _, candidate := range remaining {
			if len(selected) >= 3 {
				break
			}
			appendCandidate(candidate)
		}
	}
	if len(selected) < 3 {
		for _, candidate := range candidates[1:] {
			if len(selected) >= 3 {
				break
			}
			appendCandidate(candidate)
		}
	}

	sort.SliceStable(selected, func(i, j int) bool {
		ri, _ := selected[i]["_recency_index"].(int)
		rj, _ := selected[j]["_recency_index"].(int)
		return ri < rj
	})

	out := []any{}
	for _, candidate := range selected[:minInt(len(selected), 3)] {
		cleaned := map[string]any{}
		for k, v := range candidate {
			if !strings.HasPrefix(k, "_") {
				cleaned[k] = v
			}
		}
		out = append(out, cleaned)
	}
	return out
}

func recentChangeSummary(anchor any) any {
	if m, ok := anchor.(map[string]any); ok {
		return m["summary_text"]
	}
	return nil
}

func mapStringOrNil(m map[string]any, key string) any {
	if m == nil {
		return nil
	}
	if s, ok := m[key].(string); ok && strings.TrimSpace(s) != "" {
		return s
	}
	return nil
}

func mapAnyOrEmptyStringSlice(m map[string]any, key string) []string {
	if m == nil {
		return []string{}
	}
	if v, ok := m[key].([]string); ok {
		return v
	}
	return []string{}
}

func limitMapSlice(items []map[string]any, limit int) []any {
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return mapSliceToAny(items)
}

func mapSliceToAny(items []map[string]any) []any {
	out := make([]any, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	return out
}

func mapAnySlice(value any) []map[string]any {
	raw, ok := value.([]any)
	if !ok {
		return []map[string]any{}
	}
	out := []map[string]any{}
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func (s *Server) handleCharacterDetail(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	cname := r.PathValue("character_name")
	if sid == "" || cname == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id and character_name are required")
		return
	}
	item, err := s.Store.GetCharacterState(r.Context(), sid, cname)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			item = nil
		} else if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]any{
				"status": "error",
				"detail": fmt.Sprintf("character not found: %s", cname),
			})
			return
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"character_name":  cname,
		"found":           item != nil,
		"character":       item,
	})
}

func (s *Server) handleCharacterEvents(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	cname := r.PathValue("character_name")
	if sid == "" || cname == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id and character_name are required")
		return
	}
	limit := 30
	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 0 {
			offset = v
		}
	}
	items, err := s.Store.ListCharacterEvents(r.Context(), sid, cname)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			items = nil
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
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
		"status":          "ok",
		"chat_session_id": sid,
		"character_name":  cname,
		"events":          page,
		"total":           total,
		"limit":           limit,
		"offset":          offset,
	})
}

func (s *Server) handleCharacterStateHistory(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	cname := r.PathValue("character_name")
	if sid == "" || cname == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id and character_name are required")
		return
	}
	limit := 50
	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 0 {
			offset = v
		}
	}
	historyStore, ok := s.Store.(store.CharacterStateHistoryStore)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":          "ok",
			"chat_session_id": sid,
			"character_name":  cname,
			"state_history":   []store.CharacterState{},
			"count":           0,
			"limit":           limit,
			"offset":          offset,
			"mode":            "history_store_not_available",
		})
		return
	}
	items, err := historyStore.ListCharacterStateHistory(r.Context(), sid, cname, limit, offset)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			items = []store.CharacterState{}
		} else {
			writeInternalError(w, err.Error())
			return
		}
	}
	items = nonNilSlice(items)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"character_name":  cname,
		"state_history":   items,
		"count":           len(items),
		"limit":           limit,
		"offset":          offset,
		"mode":            "append_only_snapshots_latest_first",
	})
}

func (s *Server) handleCharacterPatch(w http.ResponseWriter, r *http.Request) {
	s.handleCharacterStatePatch(w, r, false)
}

func (s *Server) handleCharacterSpeech(w http.ResponseWriter, r *http.Request) {
	s.handleCharacterStatePatch(w, r, true)
}

func (s *Server) handleCharacterDelete(w http.ResponseWriter, r *http.Request) {
	endpoint := "DELETE /characters/{chat_session_id}/{character_name}"
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, endpoint)
		return
	}
	mutationStore, ok := s.Store.(store.ExplorerMutationStore)
	if !ok {
		writeShadowGuard(w, endpoint)
		return
	}
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	cname := strings.TrimSpace(r.PathValue("character_name"))
	if sid == "" || cname == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id and character_name are required")
		return
	}
	current, err := s.Store.GetCharacterState(r.Context(), sid, cname)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("character not found: %s", cname))
			return
		}
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, endpoint)
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	events, _ := s.Store.ListCharacterEvents(r.Context(), sid, cname)
	changedAt := time.Now().UTC()
	if err := mutationStore.DeleteCharacterByName(r.Context(), sid, cname); err != nil {
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
		TargetType:    "character",
		TargetID:      current.ID,
		Summary:       "Explorer manual character delete",
		DetailsJSON: mustCompactJSON(map[string]any{
			"character_name": cname,
			"previous": map[string]any{
				"turn_index":         current.TurnIndex,
				"appearance_json":    current.AppearanceJSON,
				"personality_json":   current.PersonalityJSON,
				"status_json":        current.StatusJSON,
				"relationships_json": current.RelationshipsJSON,
				"speech_style_json":  current.SpeechStyleJSON,
				"created_at":         current.CreatedAt,
				"updated_at":         current.UpdatedAt,
			},
			"character_events_deleted": len(events),
			"changed_at":               changedAt,
		}),
		Source:    "explorer_manual_delete",
		CreatedAt: changedAt,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"status":                   "ok",
		"source":                   s.storeWriteSource(),
		"mutation_enabled":         true,
		"chat_session_id":          sid,
		"target_type":              "character",
		"target_id":                current.ID,
		"character_name":           cname,
		"deleted":                  true,
		"character_events_deleted": len(events),
		"changed_at":               changedAt,
		"audit_written":            true,
	})
}

func (s *Server) handleCharacterStatePatch(w http.ResponseWriter, r *http.Request, speechOnly bool) {
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	cname := strings.TrimSpace(r.PathValue("character_name"))
	if sid == "" || cname == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id and character_name are required")
		return
	}
	saver, ok := s.Store.(characterStateSaver)
	if !ok {
		writeShadowGuard(w, r.Method+" "+r.URL.Path)
		return
	}
	payload, err := decodeNarrativeJSONMap(r)
	if err != nil {
		writeBadRequest(w, "invalid JSON body")
		return
	}
	updates, err := normalizeCharacterPatchPayload(payload, speechOnly)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if len(updates) == 0 {
		writeBadRequest(w, "no supported character fields to update")
		return
	}
	current, err := s.Store.GetCharacterState(r.Context(), sid, cname)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeNotFound(w, fmt.Sprintf("character not found: %s", cname))
			return
		}
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, r.Method+" "+r.URL.Path)
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	now := time.Now().UTC()
	next := *current
	next.ChatSessionID = sid
	next.CharacterName = cname
	next.UpdatedAt = now
	if next.CreatedAt.IsZero() {
		next.CreatedAt = now
	}
	changed := make([]string, 0, len(updates))
	for _, key := range []string{"appearance_json", "personality_json", "status_json", "relationships_json", "speech_style_json", "turn_index"} {
		val, exists := updates[key]
		if !exists {
			continue
		}
		changed = append(changed, key)
		switch key {
		case "appearance_json":
			next.AppearanceJSON = stringFromAnyNullable(val)
		case "personality_json":
			next.PersonalityJSON = stringFromAnyNullable(val)
		case "status_json":
			next.StatusJSON = stringFromAnyNullable(val)
		case "relationships_json":
			next.RelationshipsJSON = stringFromAnyNullable(val)
		case "speech_style_json":
			next.SpeechStyleJSON = stringFromAnyNullable(val)
		case "turn_index":
			if i, ok := val.(int); ok {
				next.TurnIndex = i
			}
		}
	}
	if err := saver.SaveCharacterState(r.Context(), &next); err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, r.Method+" "+r.URL.Path)
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	eventType := "manual_patch"
	if speechOnly {
		eventType = "speech_style_patch"
	}
	_ = s.Store.SaveCharacterEvent(r.Context(), &store.CharacterEvent{
		ChatSessionID: sid,
		CharacterName: cname,
		TurnIndex:     next.TurnIndex,
		EventType:     eventType,
		DetailsJSON:   mustCompactJSON(map[string]any{"updated_fields": changed, "source": "manual_patch"}),
		CreatedAt:     now,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"character_name":  cname,
		"updated_fields":  changed,
		"character":       characterResponseItem(next, characterStaleSnapshot(next, nil, next.TurnIndex, "", nil), nil, nil),
	})
}

func normalizeCharacterPatchPayload(payload map[string]any, speechOnly bool) (map[string]any, error) {
	updates := map[string]any{}
	fieldMap := map[string]string{
		"appearance_json":    "appearance_json",
		"appearance":         "appearance_json",
		"personality_json":   "personality_json",
		"personality":        "personality_json",
		"status_json":        "status_json",
		"status":             "status_json",
		"relationships_json": "relationships_json",
		"relationships":      "relationships_json",
		"speech_style_json":  "speech_style_json",
		"speech_style":       "speech_style_json",
	}
	if speechOnly {
		fieldMap = map[string]string{"speech_style_json": "speech_style_json", "speech_style": "speech_style_json"}
	}
	for rawKey, targetKey := range fieldMap {
		val, exists := payload[rawKey]
		if !exists {
			continue
		}
		normalized, err := normalizeStorylineJSONPatchValue(targetKey, val)
		if err != nil {
			return nil, err
		}
		updates[targetKey] = normalized
	}
	if !speechOnly {
		if val, exists := payload["turn_index"]; exists {
			i, ok := storylineIntPatchValue(val)
			if !ok || i < 0 {
				return nil, fmt.Errorf("turn_index must be a non-negative integer")
			}
			updates["turn_index"] = i
		}
	}
	return updates, nil
}

func stringFromAnyNullable(v any) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}
