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

func resolveStorylineReferenceTurn(items []store.Storyline, rawCurrentTurn string) *int {
	if raw := strings.TrimSpace(rawCurrentTurn); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			return &parsed
		}
	}
	var latest *int
	for _, item := range items {
		for _, candidate := range []int{item.LastEvidenceTurn, item.LastTurn} {
			if candidate < 0 {
				continue
			}
			if latest == nil || candidate > *latest {
				value := candidate
				latest = &value
			}
		}
	}
	return latest
}

func storylineResponseItems(items []store.Storyline, referenceTurn *int) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		snapshot := storylineStaleSnapshot(item, referenceTurn)
		keyPointsJSON := normalizeStorylineListJSONString(item.KeyPointsJSON)
		tensionsJSON := normalizeStorylineListJSONString(item.OngoingTensionsJSON)
		out = append(out, map[string]any{
			"id":                    item.ID,
			"chat_session_id":       item.ChatSessionID,
			"name":                  item.Name,
			"status":                item.Status,
			"entities_json":         item.EntitiesJSON,
			"current_context":       item.CurrentContext,
			"key_points_json":       keyPointsJSON,
			"ongoing_tensions_json": tensionsJSON,
			"confidence":            item.Confidence,
			"evidence_count":        item.EvidenceCount,
			"last_evidence_turn":    item.LastEvidenceTurn,
			"last_observed_turn":    snapshot["last_observed_turn"],
			"freshness_turn_gap":    snapshot["freshness_turn_gap"],
			"stale_after_turns":     snapshot["stale_after_turns"],
			"is_stale":              snapshot["is_stale"],
			"stale_reason":          snapshot["stale_reason"],
			"first_turn":            item.FirstTurn,
			"last_turn":             item.LastTurn,
			"pinned":                item.Pinned,
			"suppressed":            item.Suppressed,
			"user_corrected":        item.UserCorrected,
			"created_at":            nullableTime(item.CreatedAt),
			"updated_at":            nullableTime(item.UpdatedAt),
		})
	}
	return out
}

func storylineStaleSnapshot(item store.Storyline, referenceTurn *int) map[string]any {
	evidenceCount := item.EvidenceCount
	if evidenceCount < 0 {
		evidenceCount = 0
	}
	lastObserved := item.LastEvidenceTurn
	if lastObserved <= 0 {
		lastObserved = item.LastTurn
	}
	var freshness any
	if referenceTurn != nil && lastObserved >= 0 {
		gap := *referenceTurn - lastObserved
		if gap < 0 {
			gap = 0
		}
		freshness = gap
	}
	staleAfter := evidenceCount + 2
	if staleAfter < 3 {
		staleAfter = 3
	}
	if staleAfter > 8 {
		staleAfter = 8
	}
	isStale := false
	var staleReason any
	if item.Status == "active" {
		if gap, ok := freshness.(int); ok && gap >= staleAfter {
			isStale = true
			if evidenceCount <= 1 {
				staleReason = "low_evidence_gap"
			} else {
				staleReason = "freshness_gap"
			}
		}
	}
	return map[string]any{
		"last_observed_turn": lastObserved,
		"freshness_turn_gap": freshness,
		"stale_after_turns":  staleAfter,
		"is_stale":           isStale,
		"stale_reason":       staleReason,
	}
}

type storylineSelectionEntry struct {
	Item             store.Storyline
	Snapshot         map[string]any
	LastObservedTurn int
	FreshnessGap     *int
	StaleAfterTurns  int
	IsStale          bool
	StaleReason      any
	Confidence       float64
}

type storylineSupervisorSelection struct {
	ReferenceTurn *int
	Selected      []storylineSelectionEntry
	Dropped       []storylineSelectionEntry
	Resolved      []storylineSelectionEntry
	Suppressed    []storylineSelectionEntry
}

func selectStorylinesForSupervisor(items []store.Storyline, referenceTurn *int, limit int) storylineSupervisorSelection {
	if limit <= 0 {
		limit = 5
	}
	if referenceTurn == nil {
		referenceTurn = resolveStorylineReferenceTurn(items, "")
	}
	selection := storylineSupervisorSelection{ReferenceTurn: referenceTurn}
	active := make([]storylineSelectionEntry, 0, len(items))
	for _, item := range items {
		entry := buildStorylineSelectionEntry(item, referenceTurn)
		status := normalizedStorylineStatus(item.Status)
		if item.Suppressed {
			selection.Suppressed = append(selection.Suppressed, entry)
			continue
		}
		if status != "active" {
			if isResolvedStorylineStatus(status) {
				selection.Resolved = append(selection.Resolved, entry)
			} else {
				selection.Dropped = append(selection.Dropped, entry)
			}
			continue
		}
		active = append(active, entry)
	}
	sort.SliceStable(active, func(i, j int) bool {
		return storylineSelectionLess(active[i], active[j])
	})
	pinned := make([]storylineSelectionEntry, 0, len(active))
	fresh := make([]storylineSelectionEntry, 0, len(active))
	stale := make([]storylineSelectionEntry, 0, len(active))
	for _, entry := range active {
		if entry.Item.Pinned {
			pinned = append(pinned, entry)
		} else if entry.IsStale {
			stale = append(stale, entry)
		} else {
			fresh = append(fresh, entry)
		}
	}
	if len(pinned) > 0 || len(fresh) > 0 {
		for _, group := range [][]storylineSelectionEntry{pinned, fresh} {
			for _, entry := range group {
				if len(selection.Selected) < limit {
					selection.Selected = append(selection.Selected, entry)
				} else {
					selection.Dropped = append(selection.Dropped, entry)
				}
			}
		}
		selection.Dropped = append(selection.Dropped, stale...)
		return selection
	}
	if len(stale) > 0 {
		selection.Selected = append(selection.Selected, stale[0])
		selection.Dropped = append(selection.Dropped, stale[1:]...)
	}
	return selection
}

func selectedStorylineItems(selection storylineSupervisorSelection) []store.Storyline {
	out := make([]store.Storyline, 0, len(selection.Selected))
	for _, entry := range selection.Selected {
		out = append(out, entry.Item)
	}
	return out
}

func storylineDetailCompareText(value string) string {
	clean := strings.TrimLeftFunc(strings.TrimSpace(value), func(r rune) bool {
		switch r {
		case ' ', '\t', '\r', '\n', '-', 0x2022, 0x26A1:
			return true
		default:
			return false
		}
	})
	return strings.ToLower(strings.Join(strings.Fields(clean), " "))
}

func isStorylineSelfEchoDetail(item store.Storyline, detail string) bool {
	key := storylineDetailCompareText(detail)
	if key == "" {
		return false
	}
	name := strings.TrimSpace(item.Name)
	context := strings.TrimSpace(item.CurrentContext)
	refs := []string{name, context}
	if name != "" && context != "" {
		refs = append(refs, name+" "+string(rune(0x2014))+" "+context, name+" - "+context)
	}
	for _, ref := range refs {
		if storylineDetailCompareText(ref) == key {
			return true
		}
	}
	return false
}

func filterStorylineContextDetails(item store.Storyline, details []string) []string {
	out := make([]string, 0, len(details))
	seen := make(map[string]bool, len(details))
	for _, detail := range details {
		clean := strings.TrimSpace(detail)
		key := storylineDetailCompareText(clean)
		if key == "" || seen[key] || isStorylineSelfEchoDetail(item, clean) {
			continue
		}
		seen[key] = true
		out = append(out, clean)
	}
	return out
}

func formatStorylinesForSupervisor(selection storylineSupervisorSelection) string {
	lines := make([]string, 0, len(selection.Selected)+len(selection.Resolved)+2)
	if len(selection.Selected) > 0 {
		lines = append(lines, "[Storylines]")
		for _, entry := range selection.Selected {
			desc := strings.TrimSpace(entry.Item.CurrentContext)
			if desc == "" {
				desc = strings.TrimSpace(entry.Item.Name)
			}
			if desc == "" {
				continue
			}
			keyPoints := filterStorylineContextDetails(entry.Item, parseStorylineListJSON(entry.Item.KeyPointsJSON))
			tensions := filterStorylineContextDetails(entry.Item, parseStorylineListJSON(entry.Item.OngoingTensionsJSON))
			lines = append(lines, fmt.Sprintf(
				"- %s (confidence=%.2f, evidence=%d, freshness_gap=%s)",
				truncateRunes(desc, 180),
				entry.Confidence,
				entry.Item.EvidenceCount,
				storylineGapLabel(entry.FreshnessGap),
			))
			if len(keyPoints) > 0 {
				lines = append(lines, fmt.Sprintf("  key_points: %s", strings.Join(keyPoints[:minInt(len(keyPoints), 3)], " / ")))
			}
			if len(tensions) > 0 {
				lines = append(lines, fmt.Sprintf("  tensions: %s", strings.Join(tensions[:minInt(len(tensions), 3)], " / ")))
			}
		}
	}
	if len(selection.Resolved) > 0 {
		lines = append(lines, "[Resolved Storylines Summary]")
		for i, entry := range selection.Resolved {
			if i >= 2 {
				break
			}
			name := strings.TrimSpace(entry.Item.Name)
			if name == "" {
				name = fmt.Sprintf("storyline_%d", entry.Item.ID)
			}
			lines = append(lines, fmt.Sprintf("- %s resolved at turn %s", truncateRunes(name, 120), storylineObservedLabel(entry.LastObservedTurn)))
		}
	}
	return strings.Join(lines, "\n")
}

func storylineSelectionSummary(selection storylineSupervisorSelection) map[string]any {
	selected := make([]map[string]any, 0, len(selection.Selected))
	for _, entry := range selection.Selected {
		selected = append(selected, storylineSelectionEntryMap(entry))
	}
	dropped := make([]map[string]any, 0, len(selection.Dropped))
	for _, entry := range selection.Dropped {
		dropped = append(dropped, storylineSelectionEntryMap(entry))
	}
	resolved := make([]map[string]any, 0, minInt(len(selection.Resolved), 3))
	for i, entry := range selection.Resolved {
		if i >= 3 {
			break
		}
		resolved = append(resolved, storylineSelectionEntryMap(entry))
	}
	suppressed := make([]map[string]any, 0, minInt(len(selection.Suppressed), 3))
	for i, entry := range selection.Suppressed {
		if i >= 3 {
			break
		}
		suppressed = append(suppressed, storylineSelectionEntryMap(entry))
	}
	staleDropped := 0
	for _, entry := range selection.Dropped {
		if entry.IsStale {
			staleDropped++
		}
	}
	staleSelected := 0
	for _, entry := range selection.Selected {
		if entry.IsStale {
			staleSelected++
		}
	}
	return map[string]any{
		"policy_version":           "storyline_selection.h2d.go.v1",
		"reference_turn":           nullableIntPtr(selection.ReferenceTurn),
		"selected_count":           len(selection.Selected),
		"dropped_count":            len(selection.Dropped),
		"resolved_summary_count":   len(selection.Resolved),
		"suppressed_count":         len(selection.Suppressed),
		"stale_selected_count":     staleSelected,
		"stale_dropped_count":      staleDropped,
		"selected":                 selected,
		"dropped":                  dropped,
		"resolved_summary":         resolved,
		"suppressed_summary":       suppressed,
		"fresh_rows_take_priority": true,
	}
}

func buildStorylineSelectionEntry(item store.Storyline, referenceTurn *int) storylineSelectionEntry {
	snapshot := storylineStaleSnapshot(item, referenceTurn)
	entry := storylineSelectionEntry{
		Item:       item,
		Snapshot:   snapshot,
		Confidence: normalizeStorylineConfidence(item.Confidence),
	}
	if v, ok := snapshot["last_observed_turn"].(int); ok {
		entry.LastObservedTurn = v
	}
	if v, ok := snapshot["freshness_turn_gap"].(int); ok {
		value := v
		entry.FreshnessGap = &value
	}
	if v, ok := snapshot["stale_after_turns"].(int); ok {
		entry.StaleAfterTurns = v
	}
	if v, ok := snapshot["is_stale"].(bool); ok {
		entry.IsStale = v
	}
	entry.StaleReason = snapshot["stale_reason"]
	return entry
}

func storylineSelectionEntryMap(entry storylineSelectionEntry) map[string]any {
	return map[string]any{
		"id":                 entry.Item.ID,
		"name":               entry.Item.Name,
		"status":             normalizedStorylineStatus(entry.Item.Status),
		"confidence":         entry.Confidence,
		"evidence_count":     entry.Item.EvidenceCount,
		"last_evidence_turn": nullablePositiveInt(entry.Item.LastEvidenceTurn),
		"last_observed_turn": nullablePositiveInt(entry.LastObservedTurn),
		"freshness_turn_gap": entry.Snapshot["freshness_turn_gap"],
		"stale_after_turns":  entry.StaleAfterTurns,
		"is_stale":           entry.IsStale,
		"stale_reason":       entry.StaleReason,
		"pinned":             entry.Item.Pinned,
		"suppressed":         entry.Item.Suppressed,
		"user_corrected":     entry.Item.UserCorrected,
	}
}

func storylineSelectionLess(a, b storylineSelectionEntry) bool {
	if a.IsStale != b.IsStale {
		return !a.IsStale
	}
	if a.Item.Pinned != b.Item.Pinned {
		return a.Item.Pinned
	}
	aGap, bGap := storylineSortGap(a), storylineSortGap(b)
	if aGap != bGap {
		return aGap < bGap
	}
	if a.Confidence != b.Confidence {
		return a.Confidence > b.Confidence
	}
	if a.Item.EvidenceCount != b.Item.EvidenceCount {
		return a.Item.EvidenceCount > b.Item.EvidenceCount
	}
	if a.LastObservedTurn != b.LastObservedTurn {
		return a.LastObservedTurn > b.LastObservedTurn
	}
	if a.Item.LastTurn != b.Item.LastTurn {
		return a.Item.LastTurn > b.Item.LastTurn
	}
	return a.Item.ID < b.Item.ID
}

func storylineSortGap(entry storylineSelectionEntry) int {
	if entry.FreshnessGap == nil {
		return 1_000_000
	}
	return *entry.FreshnessGap
}

func normalizeStorylineConfidence(value float64) float64 {
	if value <= 0 {
		return 0.5
	}
	if value > 1 {
		return 1
	}
	return value
}

func normalizedStorylineStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	if status == "" {
		return "active"
	}
	return status
}

func isResolvedStorylineStatus(status string) bool {
	switch status {
	case "resolved", "completed", "closed", "done":
		return true
	default:
		return false
	}
}

func storylineGapLabel(gap *int) string {
	if gap == nil {
		return "unknown"
	}
	return strconv.Itoa(*gap)
}

func storylineObservedLabel(turn int) string {
	if turn <= 0 {
		return "unknown"
	}
	return strconv.Itoa(turn)
}

func parseStorylineListJSON(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return []string{}
	}
	items, ok := compactStorylineTextList(decoded)
	if !ok {
		return []string{}
	}
	return items
}

func normalizeStorylineListJSONString(raw string) string {
	items := parseStorylineListJSON(raw)
	if len(items) == 0 {
		return ""
	}
	return mustCompactJSON(items)
}

func isValidWorldRuleScope(scope string) bool {
	switch store.NormalizeWorldRuleScope(scope) {
	case "root", "region", "location", "faction", "system", "session":
		return true
	default:
		return false
	}
}

func worldRuleScopeChain(scope string) []string {
	return store.WorldRuleScopeChain(scope)
}

func (s *Server) resolveActiveScope(ctx context.Context, sid string) (*store.SessionActiveScope, string, error) {
	activeScopeStore, ok := s.Store.(store.ActiveScopeStore)
	if ok {
		item, err := activeScopeStore.GetActiveScope(ctx, sid)
		if err == nil && item != nil {
			if strings.TrimSpace(item.ActiveScope) == "" {
				item.ActiveScope = "root"
			}
			return item, "store", nil
		}
		if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
			return nil, "", err
		}
	}
	return &store.SessionActiveScope{
		ChatSessionID: sid,
		ActiveScope:   "root",
	}, "default", nil
}

func activeScopeResponse(sid string, item *store.SessionActiveScope, source string) map[string]any {
	activeScope := "root"
	scopeName := ""
	var updatedAt time.Time
	if item != nil {
		if strings.TrimSpace(item.ActiveScope) != "" {
			activeScope = strings.TrimSpace(item.ActiveScope)
		}
		scopeName = strings.TrimSpace(item.ScopeName)
		updatedAt = item.UpdatedAt
	}
	return map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"active_scope":    activeScope,
		"scope_name":      nullableString(scopeName),
		"scope_chain":     worldRuleScopeChain(activeScope),
		"source":          source,
		"updated_at":      nullableTime(updatedAt),
	}
}

func worldRuleResponseItems(items []store.WorldRule, activeScope string) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		originalScope := item.Scope
		scope := store.NormalizeWorldRuleScope(item.Scope)
		m := map[string]any{
			"id":              item.ID,
			"chat_session_id": item.ChatSessionID,
			"scope":           scope,
			"scope_name":      item.ScopeName,
			"category":        item.Category,
			"key":             item.Key,
			"value_json":      item.ValueJSON,
			"genre":           item.Genre,
			"source_turn":     item.SourceTurn,
			"pinned":          item.Pinned,
			"suppressed":      item.Suppressed,
			"user_corrected":  item.UserCorrected,
			"created_at":      nullableTime(item.CreatedAt),
			"updated_at":      nullableTime(item.UpdatedAt),
		}
		if originalScope != "" && originalScope != scope {
			m["original_scope"] = originalScope
		}
		if activeScope != "" {
			m["inherited"] = scope != store.NormalizeWorldRuleScope(activeScope)
		}
		out = append(out, m)
	}
	return out
}

func nullableIntPtr(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}

func nullableTime(v time.Time) any {
	if v.IsZero() {
		return nil
	}
	return v.Format(time.RFC3339Nano)
}

func boundedQueryLimit(r *http.Request, defaultLimit, maxLimit int) int {
	if defaultLimit <= 0 {
		defaultLimit = 20
	}
	if maxLimit <= 0 {
		maxLimit = defaultLimit
	}
	limit := defaultLimit
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}
