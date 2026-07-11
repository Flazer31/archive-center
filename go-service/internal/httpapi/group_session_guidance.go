package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func (s *Server) buildL3GuidanceSnapshot(ctx context.Context, sid string) (map[string]any, bool) {
	warnings := []any{}
	snapshot := map[string]any{
		"state_status":     "no_state",
		"last_turn":        -1,
		"story_plan":       map[string]any{},
		"director":         map[string]any{},
		"compact_records":  []any{},
		"maintenance_last": nil,
		"warnings":         warnings,
	}

	gps, ok := s.Store.(store.GuidancePlanStateStore)
	if !ok {
		warnings = append(warnings, "GuidancePlanStateStore not available; safe degrade to no_state.")
		snapshot["warnings"] = warnings
		return snapshot, false
	}

	cached, err := gps.GetGuidancePlanState(ctx, sid)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) || errors.Is(err, store.ErrNotEnabled) {
			warnings = append(warnings, "No cached guidance plan state found; safe degrade to no_state.")
		} else {
			warnings = append(warnings, fmt.Sprintf("GuidancePlanState read error: %v; safe degrade to no_state.", err))
		}
		snapshot["warnings"] = warnings
		return snapshot, false
	}
	if cached == nil {
		warnings = append(warnings, "Cached guidance plan state is nil; safe degrade to no_state.")
		snapshot["warnings"] = warnings
		return snapshot, false
	}

	var storyPlan map[string]any
	var director map[string]any
	if cached.StoryPlanJSON != "" {
		_ = json.Unmarshal([]byte(cached.StoryPlanJSON), &storyPlan)
	}
	if cached.DirectorJSON != "" {
		_ = json.Unmarshal([]byte(cached.DirectorJSON), &director)
	}
	if storyPlan == nil {
		storyPlan = map[string]any{}
	}
	if director == nil {
		director = map[string]any{}
	}

	var cachedWarnings []any
	if cached.WarningsJSON != "" {
		_ = json.Unmarshal([]byte(cached.WarningsJSON), &cachedWarnings)
	}
	if cachedWarnings == nil {
		cachedWarnings = []any{}
	}

	stateStatus := strings.TrimSpace(cached.StateStatus)
	if stateStatus == "empty" {
		cachedWarnings = append(cachedWarnings, "rebuild will be triggered by next GET /narrative-control call")
	} else {
		stateStatus = "active"
	}

	lastTurn := cached.LastTurn
	if lastTurn < 0 {
		lastTurn = -1
	}

	snapshot = map[string]any{
		"state_status":     stateStatus,
		"last_turn":        lastTurn,
		"story_plan":       storyPlan,
		"director":         director,
		"compact_records":  []any{},
		"maintenance_last": nil,
		"warnings":         cachedWarnings,
	}

	return snapshot, true
}

func (s *Server) handleSessionGuidanceSnapshot(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	ctx := r.Context()

	snapshot, _ := s.buildL3GuidanceSnapshot(ctx, sid)
	snapshot["status"] = "ok"
	snapshot["chat_session_id"] = sid
	snapshot["generated_at"] = generatedAt()

	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) handleSessionStep7Health(w http.ResponseWriter, r *http.Request) {
	sid := r.PathValue("chat_session_id")
	ctx := r.Context()

	// L-5: parse passes query param (default 10, clamp 1..50)
	passes := 10
	if raw := strings.TrimSpace(r.URL.Query().Get("passes")); raw != "" {
		if p, err := strconv.Atoi(raw); err == nil {
			if p < 1 {
				passes = 1
			} else if p > 50 {
				passes = 50
			} else {
				passes = p
			}
		}
	}

	chatLogs, chatLogErr := s.Store.ListChatLogs(ctx, sid, 0, 0)
	chatLogs = nonNilSlice(chatLogs)

	storylines, _ := s.Store.ListStorylines(ctx, sid)
	pendingThreads, _ := s.Store.ListPendingThreads(ctx, sid, "")
	storylines = nonNilSlice(storylines)
	pendingThreads = nonNilSlice(pendingThreads)

	warnings := []any{}
	if chatLogErr != nil && !errors.Is(chatLogErr, store.ErrNotEnabled) {
		warnings = append(warnings, fmt.Sprintf("chat_logs read error: %v", chatLogErr))
	}

	// L-3 guidance snapshot read-through
	guidanceSnapshot, hasCached := s.buildL3GuidanceSnapshot(ctx, sid)
	stateStatus, _ := guidanceSnapshot["state_status"].(string)
	lastTurn := -1
	if v, ok := guidanceSnapshot["last_turn"].(int); ok {
		lastTurn = v
	}
	storyPlan, _ := guidanceSnapshot["story_plan"].(map[string]any)
	director, _ := guidanceSnapshot["director"].(map[string]any)

	arcAgeTurns := 0
	if hasCached && stateStatus == "active" && lastTurn >= 0 {
		arcAgeTurns = len(chatLogs) - lastTurn
		if arcAgeTurns < 0 {
			arcAgeTurns = 0
		}
	}

	// L-5 compaction via existing compact history builder
	_, compactMeta := buildNarrativeCompactHistory(storyPlan, director, storylines, pendingThreads)

	// L-5 maintenance via audit logs (existing store method, no new table)
	maintenanceLogs, maintErr := s.Store.ListAuditLogs(ctx, sid, "maintenance_enqueued", passes)
	if maintErr != nil && !errors.Is(maintErr, store.ErrNotEnabled) {
		warnings = append(warnings, fmt.Sprintf("maintenance audit log read error: %v", maintErr))
	}
	maintenanceLogs = nonNilSlice(maintenanceLogs)
	totalPasses := len(maintenanceLogs)
	okCount := totalPasses
	errorCount := 0
	lastSuggestions := []any{}
	if maintErr != nil {
		// read error means we can't confirm ok counts
		okCount = 0
		errorCount = totalPasses
	}
	for _, log := range maintenanceLogs {
		if strings.Contains(log.DetailsJSON, "suggestion") {
			lastSuggestions = append(lastSuggestions, log.DetailsJSON)
		}
	}
	if len(lastSuggestions) > 3 {
		lastSuggestions = lastSuggestions[:3]
	}
	okRate := 0.0
	if totalPasses > 0 {
		okRate = float64(okCount) / float64(totalPasses)
	}

	// L-5 drift summary (conservative: no new table)
	driftSummary := map[string]any{
		"passes_analyzed": totalPasses,
		"total_signals":   0,
		"high_severity":   0,
		"by_type":         map[string]any{},
	}

	// L-5 regression checks from actual data
	regressionChecks := map[string]any{
		"guidance_persistence": "skip",
		"arc_stability":        "skip",
		"compaction_health":    "skip",
		"maintenance_effect":   "skip",
		"notes":                []any{},
	}
	switch stateStatus {
	case "active":
		regressionChecks["guidance_persistence"] = "pass"
	case "empty":
		regressionChecks["guidance_persistence"] = "warn"
		warnings = append(warnings, "guidance snapshot state is empty; rebuild pending")
	default:
		regressionChecks["guidance_persistence"] = "fail"
	}
	if arcAgeTurns >= 3 {
		regressionChecks["arc_stability"] = "pass"
	} else if stateStatus == "active" {
		regressionChecks["arc_stability"] = "warn"
		regressionChecks["notes"] = append(regressionChecks["notes"].([]any), fmt.Sprintf("arc_age_turns=%d (<3)", arcAgeTurns))
	}
	records, _ := compactMeta["total_records"].(int)
	if records >= 1 {
		regressionChecks["compaction_health"] = "pass"
	} else {
		regressionChecks["compaction_health"] = "warn"
		regressionChecks["notes"] = append(regressionChecks["notes"].([]any), "compact record count is 0 - session may be unresolved")
	}
	if totalPasses > 0 {
		if okRate >= 0.8 {
			regressionChecks["maintenance_effect"] = "pass"
		} else {
			regressionChecks["maintenance_effect"] = "fail"
			regressionChecks["notes"] = append(regressionChecks["notes"].([]any), fmt.Sprintf("maintenance ok_rate=%.2f (<0.8)", okRate))
		}
	} else {
		regressionChecks["maintenance_effect"] = "skip"
	}

	guidanceWarnings, _ := guidanceSnapshot["warnings"].([]any)
	warnings = append(warnings, guidanceWarnings...)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": sid,
		"total_turns":     len(chatLogs),
		"guidance_state": map[string]any{
			"status":          stateStatus,
			"last_built_turn": lastTurn,
			"arc_age_turns":   arcAgeTurns,
			"active_tensions": firstPositiveInt(len(asStringSlice(storyPlan["active_tensions"])), len(storylines)),
			"next_beats":      len(asAnySlice(storyPlan["next_beats"])),
			"open_required":   firstPositiveInt(len(asStringSlice(director["required_outcomes"])), countPinnedPendingThreads(pendingThreads)),
			"forbidden_count": firstPositiveInt(len(asStringSlice(director["forbidden_moves"])), countRiskPendingThreads(pendingThreads)),
		},
		"drift_summary":      driftSummary,
		"compaction_summary": compactMeta,
		"maintenance_summary": map[string]any{
			"total_passes":     totalPasses,
			"ok_count":         okCount,
			"error_count":      errorCount,
			"ok_rate":          okRate,
			"last_suggestions": lastSuggestions,
		},
		"regression_checks": regressionChecks,
		"generated_at":      generatedAt(),
		"warnings":          warnings,
	})
}
func (s *Server) handleSessionResumePack(w http.ResponseWriter, r *http.Request) {
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	if sid == "" {
		writeError(w, http.StatusBadRequest, "missing_param", "chat_session_id is required")
		return
	}
	trigger := strings.TrimSpace(r.URL.Query().Get("continuity_trigger_mode"))
	if trigger == "" {
		trigger = strings.TrimSpace(r.URL.Query().Get("trigger"))
	}
	if trigger == "" {
		trigger = "resume"
	}
	ctx := r.Context()
	pack := emptyResumePack(trigger)
	warnings := []any{}
	if storedPack, err := s.Store.GetResumePack(ctx, sid, trigger); err == nil && storedPack != nil {
		pack = resumePackToResponse(storedPack, trigger)
	} else if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
		warnings = append(warnings, "resume pack read failed; safe empty resume pack returned")
	}
	guidanceSnapshot, _ := s.buildL3GuidanceSnapshot(ctx, sid)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":            "ok",
		"detail":            "resume_pack_returned",
		"chat_session_id":   sid,
		"resume_pack":       pack,
		"guidance_snapshot": guidanceSnapshot,
		"generated_at":      generatedAt(),
		"warnings":          warnings,
	})
}

func emptyResumePack(trigger string) map[string]any {
	return map[string]any{
		"pack_status":    "empty",
		"trigger":        trigger,
		"sources_used":   []string{},
		"layer_count":    0,
		"assembled_text": "",
		"saga":           nil,
		"arc":            nil,
		"chapter":        nil,
		"assembly_note":  "P-4c: read-only long-gap resume pack; not wired into injection or input_context",
	}
}

func resumePackToResponse(pack *store.ResumePack, trigger string) map[string]any {
	if pack == nil {
		return emptyResumePack(trigger)
	}
	sources := pack.SourcesUsed
	if sources == nil {
		sources = []string{}
	}
	packStatus := strings.TrimSpace(pack.PackStatus)
	if packStatus == "" {
		packStatus = "ready"
	}
	packTrigger := strings.TrimSpace(pack.Trigger)
	if packTrigger == "" {
		packTrigger = trigger
	}
	return map[string]any{
		"pack_status":    packStatus,
		"trigger":        packTrigger,
		"sources_used":   sources,
		"layer_count":    pack.LayerCount,
		"assembled_text": pack.AssembledText,
		"saga":           pack.Saga,
		"arc":            pack.Arc,
		"chapter":        pack.Chapter,
		"assembly_note":  pack.AssemblyNote,
	}
}

func generatedAt() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func nonNilSlice[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

func maxNarrativeEvidenceTurn(storylines []store.Storyline, pendingThreads []store.PendingThread, activeStates []store.ActiveState, characters []store.CharacterState) int {
	maxTurn := 0
	for _, sl := range storylines {
		if sl.LastTurn > maxTurn {
			maxTurn = sl.LastTurn
		}
		if sl.LastEvidenceTurn > maxTurn {
			maxTurn = sl.LastEvidenceTurn
		}
	}
	for _, hook := range pendingThreads {
		if hook.SourceTurn > maxTurn {
			maxTurn = hook.SourceTurn
		}
		if hook.CreatedTurn > maxTurn {
			maxTurn = hook.CreatedTurn
		}
		if hook.ResolvedTurn > maxTurn {
			maxTurn = hook.ResolvedTurn
		}
	}
	for _, st := range activeStates {
		if st.TurnIndex > maxTurn {
			maxTurn = st.TurnIndex
		}
	}
	for _, ch := range characters {
		if ch.TurnIndex > maxTurn {
			maxTurn = ch.TurnIndex
		}
	}
	return maxTurn
}

func parseStringList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	var items []string
	if err := json.Unmarshal([]byte(raw), &items); err == nil {
		return nonNilSlice(items)
	}
	var anyItems []any
	if err := json.Unmarshal([]byte(raw), &anyItems); err != nil {
		return []string{}
	}
	out := []string{}
	for _, item := range anyItems {
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

func buildStoryPlanSnapshot(storylines []store.Storyline, pendingThreads []store.PendingThread, characters []store.CharacterState, worldRules []store.WorldRule, lastTurn int) map[string]any {
	currentArc := ""
	narrativeGoal := ""
	activeTensions := []string{}
	nextBeats := []string{}
	continuityAnchors := []string{}
	focusCharacters := []string{}
	guardrails := []string{}

	activeStorylines := activeNarrativeStorylines(storylines)
	openHooks := openNarrativeThreads(pendingThreads)
	keyWorldRules := narrativeKeyWorldRules(worldRules)

	if len(activeStorylines) > 0 {
		primary := activeStorylines[0]
		currentArc = primary.Name
		narrativeGoal = truncateRunes(strings.TrimSpace(primary.CurrentContext), 200)
		activeTensions = parseStorylineListJSON(primary.OngoingTensionsJSON)
		keyPoints := parseStorylineListJSON(primary.KeyPointsJSON)
		if len(keyPoints) > 2 {
			keyPoints = keyPoints[len(keyPoints)-2:]
		}
		continuityAnchors = append(continuityAnchors, keyPoints...)
		for _, entity := range parseStringList(primary.EntitiesJSON) {
			if len(focusCharacters) >= 4 {
				break
			}
			focusCharacters = append(focusCharacters, entity)
		}
		for _, sl := range activeStorylines[1:minInt(len(activeStorylines), 3)] {
			tensions := parseStorylineListJSON(sl.OngoingTensionsJSON)
			for _, tension := range tensions[:minInt(len(tensions), 1)] {
				activeTensions = appendUniqueString(activeTensions, tension)
			}
		}
	}
	for _, hook := range openHooks {
		if len(nextBeats) >= 4 {
			break
		}
		beat := pendingThreadNarrativeLabel(hook)
		nextBeats = append(nextBeats, beat)
		threadType := strings.TrimSpace(hook.ThreadType)
		if threadType == "" {
			threadType = strings.TrimSpace(hook.HookType)
		}
		if threadType == "promise" || threadType == "unresolved_goal" {
			continuityAnchors = appendUniqueString(continuityAnchors, pendingThreadTitle(hook))
		}
	}
	for _, wr := range keyWorldRules[:minInt(len(keyWorldRules), 3)] {
		guardrails = append(guardrails, worldRuleGuardrail(wr, false))
	}
	status := "empty"
	if currentArc != "" || len(nextBeats) > 0 || len(activeTensions) > 0 {
		status = "heuristic"
	}
	return map[string]any{
		"current_arc":        currentArc,
		"narrative_goal":     narrativeGoal,
		"active_tensions":    limitStrings(activeTensions, 4),
		"next_beats":         limitStrings(nextBeats, 6),
		"continuity_anchors": limitStrings(continuityAnchors, 4),
		"guardrails":         limitStrings(guardrails, 4),
		"persona_priorities": []any{},
		"execution_notes":    []string{},
		"focus_characters":   limitStrings(focusCharacters, 4),
		"last_plan_turn":     lastTurn,
		"state_status":       status,
	}
}

func buildDirectorSnapshot(storylines []store.Storyline, pendingThreads []store.PendingThread, characters []store.CharacterState, worldRules []store.WorldRule, lastTurn int) map[string]any {
	required := []string{}
	forbidden := []string{}
	executionChecklist := []string{}
	personaGuardrails := []string{}
	worldGuardrails := []string{}
	focusCharacters := []string{}
	activeStorylines := activeNarrativeStorylines(storylines)
	openHooks := openNarrativeThreads(pendingThreads)
	keyWorldRules := narrativeKeyWorldRules(worldRules)

	for _, hook := range openHooks {
		label := pendingThreadTitle(hook)
		if hook.Pinned {
			required = append(required, "Carry forward: "+label)
		}
		if pendingThreadType(hook) == "risk" {
			forbidden = append(forbidden, "Do not abruptly resolve: "+label)
		}
	}
	if len(activeStorylines) > 0 {
		executionChecklist = append(executionChecklist,
			"Continue from the current scene state; do not open a new scene without cause.",
			"Deliver at least one visible beat before the response ends.",
		)
	}
	if len(activeStorylines) > 0 {
		for _, entity := range parseStringList(activeStorylines[0].EntitiesJSON) {
			if len(focusCharacters) >= 4 {
				break
			}
			focusCharacters = appendUniqueString(focusCharacters, entity)
		}
	}
	for _, wr := range keyWorldRules {
		if len(worldGuardrails) >= 4 {
			break
		}
		if wr.Category == "physics" {
			worldGuardrails = append(worldGuardrails, worldRuleGuardrail(wr, true))
		}
	}
	for _, ch := range latestCharacterStatesByName(characters) {
		if len(personaGuardrails) >= 4 {
			break
		}
		if !containsString(focusCharacters, ch.CharacterName) {
			continue
		}
		if hint := characterPersonaGuardrail(ch); hint != "" {
			personaGuardrails = append(personaGuardrails, hint)
		}
	}
	stateStatus := "empty"
	if len(activeStorylines) > 0 || len(required) > 0 || len(forbidden) > 0 || len(worldGuardrails) > 0 || len(focusCharacters) > 0 {
		stateStatus = "heuristic"
	}
	currentArc := ""
	if len(activeStorylines) > 0 {
		currentArc = activeStorylines[0].Name
	}
	pressureLevel := "light"
	if countPinnedPendingThreads(openHooks) >= 2 || len(asStringSlice(buildStoryPlanSnapshot(activeStorylines, openHooks, characters, worldRules, lastTurn)["active_tensions"])) >= 3 {
		pressureLevel = "strong"
	} else if countPinnedPendingThreads(openHooks) >= 1 || len(activeStorylines) > 0 {
		pressureLevel = "steady"
	}
	return map[string]any{
		"scene_mandate":       sceneMandateForArc(currentArc),
		"required_outcomes":   limitStrings(required, 6),
		"forbidden_moves":     limitStrings(forbidden, 6),
		"pressure_level":      pressureLevel,
		"execution_checklist": limitStrings(dedupeStrings(executionChecklist), 4),
		"persona_guardrails":  limitStrings(personaGuardrails, 4),
		"world_guardrails":    limitStrings(worldGuardrails, 4),
		"focus_characters":    limitStrings(focusCharacters, 4),
		"last_turn":           lastTurn,
		"state_status":        stateStatus,
		"resolved_outcomes":   []string{},
		"expired_forbidden":   []string{},
	}
}

func hasNarrativePlanSignal(plan map[string]any) bool {
	return strings.TrimSpace(asString(plan["current_arc"])) != "" || len(asStringSlice(plan["next_beats"])) > 0
}

func hasDirectorSignal(director map[string]any) bool {
	return strings.TrimSpace(asString(director["scene_mandate"])) != "" ||
		len(asStringSlice(director["required_outcomes"])) > 0 ||
		len(asStringSlice(director["world_guardrails"])) > 0
}

func asString(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	return ""
}

func asStringSlice(value any) []string {
	if items, ok := value.([]string); ok {
		return items
	}
	if items, ok := value.([]any); ok {
		out := make([]string, 0, len(items))
		for _, item := range items {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	}
	return []string{}
}

// asAnySlice coerces a value to []any, returning an empty slice on failure.
func asAnySlice(value any) []any {
	if items, ok := value.([]any); ok {
		return items
	}
	if items, ok := value.([]string); ok {
		out := make([]any, 0, len(items))
		for _, item := range items {
			out = append(out, item)
		}
		return out
	}
	return []any{}
}

// unionAnyStringSlices returns a deduplicated union of two []any slices that
// contain strings. New items come first; old items not already present are
// appended. This preserves order and avoids duplicates.
func unionAnyStringSlices(newItems, oldItems []any) []any {
	seen := map[string]bool{}
	out := []any{}
	for _, v := range newItems {
		if s, ok := v.(string); ok && s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, v := range oldItems {
		if s, ok := v.(string); ok && s != "" && !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func pendingThreadLabel(hook store.PendingThread) string {
	if strings.TrimSpace(hook.Description) != "" {
		return strings.TrimSpace(hook.Description)
	}
	if strings.TrimSpace(hook.ThreadKey) != "" {
		return strings.TrimSpace(hook.ThreadKey)
	}
	return "thread"
}

func activeNarrativeStorylines(items []store.Storyline) []store.Storyline {
	out := []store.Storyline{}
	for _, item := range items {
		if item.Suppressed {
			continue
		}
		if strings.TrimSpace(item.Status) != "" && !strings.EqualFold(item.Status, "active") {
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

func openNarrativeThreads(items []store.PendingThread) []store.PendingThread {
	out := []store.PendingThread{}
	for _, item := range items {
		if item.Suppressed {
			continue
		}
		status := strings.TrimSpace(item.Status)
		if status != "" && status != "open" && status != "paused" {
			continue
		}
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Pinned != out[j].Pinned {
			return out[i].Pinned
		}
		if out[i].LastSeenTurn != out[j].LastSeenTurn {
			return out[i].LastSeenTurn > out[j].LastSeenTurn
		}
		return out[i].ID > out[j].ID
	})
	return out
}

func narrativeKeyWorldRules(items []store.WorldRule) []store.WorldRule {
	out := []store.WorldRule{}
	for _, item := range items {
		if item.Suppressed {
			continue
		}
		switch item.Category {
		case "exists", "physics", "systems":
			out = append(out, item)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Pinned != out[j].Pinned {
			return out[i].Pinned
		}
		return out[i].ID > out[j].ID
	})
	if len(out) > 8 {
		return out[:8]
	}
	return out
}

func pendingThreadType(hook store.PendingThread) string {
	if strings.TrimSpace(hook.ThreadType) != "" {
		return strings.TrimSpace(hook.ThreadType)
	}
	return strings.TrimSpace(hook.HookType)
}

func pendingThreadTitle(hook store.PendingThread) string {
	if strings.TrimSpace(hook.Title) != "" {
		return strings.TrimSpace(hook.Title)
	}
	return pendingThreadLabel(hook)
}

func pendingThreadNarrativeLabel(hook store.PendingThread) string {
	threadType := pendingThreadType(hook)
	title := pendingThreadTitle(hook)
	if threadType == "" {
		return title
	}
	return "[" + threadType + "] " + title
}

func worldRuleGuardrail(rule store.WorldRule, withPrefix bool) string {
	desc := worldRuleDescription(rule)
	if withPrefix {
		return "World rule [" + rule.Key + "]: " + truncateRunes(desc, 80)
	}
	if strings.TrimSpace(rule.Key) == "" {
		return truncateRunes(desc, 80)
	}
	return rule.Key + ": " + truncateRunes(desc, 80)
}

func worldRuleDescription(rule store.WorldRule) string {
	raw := strings.TrimSpace(rule.ValueJSON)
	if raw != "" {
		var parsed any
		if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
			if m, ok := parsed.(map[string]any); ok {
				if s := firstStringValue(m, "description", "value", "summary", "detail"); s != "" {
					return s
				}
				return truncateRunes(compactJSONForShadow(m, 120), 120)
			}
			if s, ok := parsed.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
		return raw
	}
	return strings.TrimSpace(rule.Key)
}

func firstStringValue(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			if s := cleanShadowText(v, 180); s != "" {
				return s
			}
		}
	}
	return ""
}

func appendUniqueString(items []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" || containsString(items, value) {
		return items
	}
	return append(items, value)
}

func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func dedupeStrings(items []string) []string {
	out := []string{}
	for _, item := range items {
		out = appendUniqueString(out, item)
	}
	return out
}

func sceneMandateForArc(arc string) string {
	arc = strings.TrimSpace(arc)
	if arc == "" {
		return ""
	}
	return "Continue arc: " + arc
}

func characterPersonaGuardrail(ch store.CharacterState) string {
	hints := []string{}
	if style, ok := parseSurfacePayload(ch.SpeechStyleJSON).(map[string]any); ok {
		if tone := firstStringValue(style, "tone", "style", "default_tone", "speech_notes", "honorific_style"); tone != "" {
			hints = append(hints, "speaks "+truncateRunes(tone, 60))
		}
	} else if s := cleanShadowText(parseSurfacePayload(ch.SpeechStyleJSON), 60); s != "" {
		hints = append(hints, "speaks "+s)
	}
	if pers, ok := parseSurfacePayload(ch.PersonalityJSON).(map[string]any); ok {
		if trait := firstStringValue(pers, "core_trait", "trait", "personality"); trait != "" {
			hints = append(hints, "core trait: "+truncateRunes(trait, 60))
		}
	} else if list, ok := parseSurfacePayload(ch.PersonalityJSON).([]any); ok && len(list) > 0 {
		if trait := cleanShadowText(list[0], 60); trait != "" {
			hints = append(hints, "core trait: "+trait)
		}
	}
	if len(hints) == 0 {
		return ""
	}
	return "[" + ch.CharacterName + "] " + strings.Join(hints, "; ")
}

func limitStrings(items []string, limit int) []string {
	if items == nil {
		return []string{}
	}
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

// mergeDirectorPrev carries forward resolved_outcomes and expired_forbidden from a
// previous director snapshot. Newly resolved hooks (previously required but no
// longer present) are appended to resolved_outcomes. Newly expired risks
// (previously forbidden but no longer present) are appended to expired_forbidden.
// Hooks that reappear are removed from the historical lists.
func mergeDirectorPrev(newDirector, prevDirector map[string]any) map[string]any {
	if prevDirector == nil {
		return newDirector
	}
	prevRequired := asStringSlice(prevDirector["required_outcomes"])
	prevForbidden := asStringSlice(prevDirector["forbidden_moves"])
	prevResolved := asStringSlice(prevDirector["resolved_outcomes"])
	prevExpired := asStringSlice(prevDirector["expired_forbidden"])

	newRequired := asStringSlice(newDirector["required_outcomes"])
	newForbidden := asStringSlice(newDirector["forbidden_moves"])

	// resolved = previous resolved + (previously required that are no longer required)
	resolved := []string{}
	seenResolved := map[string]bool{}
	for _, item := range prevResolved {
		if !containsString(newRequired, item) && !seenResolved[item] {
			seenResolved[item] = true
			resolved = append(resolved, item)
		}
	}
	for _, item := range prevRequired {
		if !containsString(newRequired, item) && !seenResolved[item] {
			seenResolved[item] = true
			resolved = append(resolved, item)
		}
	}

	// expired = previous expired + (previously forbidden that are no longer forbidden)
	expired := []string{}
	seenExpired := map[string]bool{}
	for _, item := range prevExpired {
		if !containsString(newForbidden, item) && !seenExpired[item] {
			seenExpired[item] = true
			expired = append(expired, item)
		}
	}
	for _, item := range prevForbidden {
		if !containsString(newForbidden, item) && !seenExpired[item] {
			seenExpired[item] = true
			expired = append(expired, item)
		}
	}

	newDirector["resolved_outcomes"] = limitStrings(dedupeStrings(resolved), 6)
	newDirector["expired_forbidden"] = limitStrings(dedupeStrings(expired), 6)
	return newDirector
}

type narrativeCompactEntry struct {
	Summary    string
	RecordType string
	Weight     float64
	Turn       int
	Order      int
}

func buildNarrativeCompactHistory(storyPlan, director map[string]any, storylines []store.Storyline, pendingThreads []store.PendingThread) ([]string, map[string]any) {
	entries := []narrativeCompactEntry{}
	order := 0
	add := func(recordType, summary string, weight float64, turn int) {
		summary = strings.TrimSpace(summary)
		if summary == "" {
			return
		}
		entries = append(entries, narrativeCompactEntry{
			Summary:    truncateRunes(summary, 220),
			RecordType: recordType,
			Weight:     weight,
			Turn:       turn,
			Order:      order,
		})
		order++
	}

	baseWeight := 1.0
	switch strings.ToLower(strings.TrimSpace(asString(director["pressure_level"]))) {
	case "strong", "high":
		baseWeight += 0.65
	case "steady", "medium":
		baseWeight += 0.3
	}
	baseWeight += math.Min(0.4, float64(len(asStringSlice(storyPlan["active_tensions"])))*0.1)

	directorTurn := 0
	if v, ok := director["last_turn"].(int); ok {
		directorTurn = v
	} else if f, ok := director["last_turn"].(float64); ok {
		directorTurn = int(f)
	}
	for _, item := range asStringSlice(director["resolved_outcomes"]) {
		add("resolved_outcome", "Resolved: "+item, baseWeight+0.15, directorTurn)
	}
	for _, item := range asStringSlice(director["expired_forbidden"]) {
		add("expired_forbidden", "Forbidden expired: "+item, baseWeight+0.1, directorTurn)
	}

	for _, sl := range storylines {
		if !strings.EqualFold(strings.TrimSpace(sl.Status), "resolved") {
			continue
		}
		name := strings.TrimSpace(sl.Name)
		if name == "" {
			name = fmt.Sprintf("storyline_%d", sl.ID)
		}
		turn := firstPositiveInt(sl.LastTurn, sl.LastEvidenceTurn, sl.FirstTurn)
		weight := 0.9 + math.Min(0.5, sl.Confidence*0.5) + math.Min(0.35, float64(sl.EvidenceCount)*0.05)
		add("resolved_storyline", fmt.Sprintf("Resolved arc: %s resolved at turn %s", name, storylineObservedLabel(turn)), weight, turn)
	}

	for _, hook := range pendingThreads {
		if !strings.EqualFold(strings.TrimSpace(hook.Status), "resolved") {
			continue
		}
		title := pendingThreadTitle(hook)
		turn := firstPositiveInt(hook.ResolvedTurn, hook.LastSeenTurn, hook.SourceTurn, hook.CreatedTurn)
		weight := 0.85
		if hook.Pinned {
			weight += 0.4
		}
		if pendingThreadType(hook) == "risk" || pendingThreadType(hook) == "emotional_debt" {
			weight += 0.25
		}
		if hook.Priority > 0 {
			weight += math.Min(0.3, float64(hook.Priority)*0.05)
		}
		add("resolved_hook", fmt.Sprintf("Resolved hook: %s resolved at turn %s", title, storylineObservedLabel(turn)), weight, turn)
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Weight != entries[j].Weight {
			return entries[i].Weight > entries[j].Weight
		}
		if entries[i].Turn != entries[j].Turn {
			return entries[i].Turn > entries[j].Turn
		}
		return entries[i].Order < entries[j].Order
	})
	if len(entries) > 8 {
		entries = entries[:8]
	}

	summaries := make([]string, 0, len(entries))
	byType := map[string]int{}
	totalWeight := 0.0
	latestTurn := -1
	for _, entry := range entries {
		summaries = append(summaries, entry.Summary)
		byType[entry.RecordType]++
		totalWeight += entry.Weight
		if entry.Turn > latestTurn {
			latestTurn = entry.Turn
		}
	}
	avg := 0.0
	if len(entries) > 0 {
		avg = totalWeight / float64(len(entries))
	}
	return summaries, map[string]any{
		"total_records":           len(entries),
		"by_type":                 byType,
		"avg_emotional_weight":    avg,
		"latest_compaction_turn":  latestTurn,
		"emotion_weight_strategy": "pressure_level + active_tensions + pinned/risk/priority + storyline confidence/evidence",
	}
}

func countPinnedPendingThreads(items []store.PendingThread) int {
	count := 0
	for _, item := range items {
		if item.Pinned {
			count++
		}
	}
	return count
}

func countRiskPendingThreads(items []store.PendingThread) int {
	count := 0
	for _, item := range items {
		if strings.EqualFold(item.HookType, "risk") {
			count++
		}
	}
	return count
}
