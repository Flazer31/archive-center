package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

const (
	narrativeRecallPacketContractVersion = "narrative_recall_packet.v1"
	narrativeRecallPacketRoute           = "/narrative-recall/packet/preview"
)

type narrativeRecallSource struct {
	SourceType string
	ID         int64
	Turn       int
	Text       string
	Weight     float64
	SourceRef  map[string]any
}

func (s *Server) handleNarrativeRecallPacketPreview(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	sid := strings.TrimSpace(query.Get("chat_session_id"))
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	turnIndex := narrativeRecallIntQuery(query.Get("turn_index"), 0)
	limit := narrativeRecallIntQuery(query.Get("limit"), 12)
	if limit < 1 {
		limit = 1
	}
	if limit > 30 {
		limit = 30
	}
	rawUserInput := strings.TrimSpace(query.Get("raw_user_input"))
	profile := strings.TrimSpace(query.Get("progression_profile"))

	resp := s.buildNarrativeRecallPacketPreview(r, sid, turnIndex, rawUserInput, profile, limit)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) buildNarrativeRecallPacketPreview(r *http.Request, sid string, turnIndex int, rawUserInput, requestedProfile string, limit int) map[string]any {
	ctx := r.Context()
	warnings := []string{}
	sourceCounts := map[string]int{}

	memories, err := s.Store.ListMemories(ctx, sid, 0, 0)
	if err != nil && !errors.Is(err, store.ErrNotEnabled) {
		warnings = append(warnings, "memories_unavailable: "+err.Error())
	}
	sourceCounts["memories"] = len(memories)

	evidence, err := s.Store.ListEvidence(ctx, sid)
	if err != nil && !errors.Is(err, store.ErrNotEnabled) {
		warnings = append(warnings, "direct_evidence_unavailable: "+err.Error())
	}
	sourceCounts["direct_evidence"] = len(evidence)

	chatLogs, err := s.Store.ListChatLogs(ctx, sid, 0, 0)
	if err != nil && !errors.Is(err, store.ErrNotEnabled) {
		warnings = append(warnings, "chat_logs_unavailable: "+err.Error())
	}
	sourceCounts["chat_logs"] = len(chatLogs)

	storylines, err := s.Store.ListStorylines(ctx, sid)
	if err != nil && !errors.Is(err, store.ErrNotEnabled) {
		warnings = append(warnings, "storylines_unavailable: "+err.Error())
	}
	sourceCounts["storylines"] = len(storylines)

	pendingThreads, err := s.Store.ListPendingThreads(ctx, sid, "")
	if err != nil && !errors.Is(err, store.ErrNotEnabled) {
		warnings = append(warnings, "pending_threads_unavailable: "+err.Error())
	}
	sourceCounts["pending_threads"] = len(pendingThreads)

	activeStates, err := s.Store.ListActiveStates(ctx, sid, "")
	if err != nil && !errors.Is(err, store.ErrNotEnabled) {
		warnings = append(warnings, "active_states_unavailable: "+err.Error())
	}
	sourceCounts["active_states"] = len(activeStates)

	canonicalLayers, err := s.Store.ListCanonicalStateLayers(ctx, sid, "")
	if err != nil && !errors.Is(err, store.ErrNotEnabled) {
		warnings = append(warnings, "canonical_state_layers_unavailable: "+err.Error())
	}
	sourceCounts["canonical_state_layers"] = len(canonicalLayers)

	episodes, err := s.Store.ListEpisodeSummaries(ctx, sid, 6, 0, 0)
	if err != nil && !errors.Is(err, store.ErrNotEnabled) {
		warnings = append(warnings, "episode_summaries_unavailable: "+err.Error())
	}
	sourceCounts["episode_summaries"] = len(episodes)

	sources := narrativeRecallSources(memories, evidence, storylines, pendingThreads, activeStates, canonicalLayers, episodes)
	if limit > 0 && len(sources) > limit*4 {
		sources = sources[:limit*4]
	}
	relationshipPacket := narrativeRecallRelationshipPacket(sources, storylines, pendingThreads)
	carryover := narrativeRecallCarryover(sources, limit)
	sceneMicrostate := narrativeRecallSceneMicrostate(rawUserInput, chatLogs, activeStates, canonicalLayers, pendingThreads)
	progressProfile := narrativeRecallProgressionProfile(requestedProfile, sceneMicrostate)
	newSceneOpportunity := narrativeRecallNewSceneOpportunity(carryover, pendingThreads, storylines, sceneMicrostate)
	promptAuthority := narrativeRecallPromptAuthorityTrace(s.Cfg.PromptDir)

	status := "ok"
	if len(warnings) > 0 {
		status = "degraded"
	} else if len(sources) == 0 {
		status = "empty"
	}
	return map[string]any{
		"status":                       status,
		"contract_version":             narrativeRecallPacketContractVersion,
		"route":                        narrativeRecallPacketRoute,
		"session_id":                   sid,
		"turn_index":                   turnIndex,
		"generated_at":                 time.Now().UTC().Format(time.RFC3339),
		"read_only":                    true,
		"support_only":                 true,
		"truth_write":                  false,
		"write_attempted":              false,
		"vector_write_attempted":       false,
		"llm_call_attempted":           false,
		"relationship_packet":          relationshipPacket,
		"carryover":                    carryover,
		"new_scene_opportunity":        newSceneOpportunity,
		"scene_microstate":             sceneMicrostate,
		"progression_profile":          progressProfile,
		"prompt_authority_trace":       promptAuthority,
		"source_counts":                sourceCounts,
		"warnings":                     warnings,
		"same_incident_foreground_cap": 1,
		"trace": map[string]any{
			"contract_owner":               "22-4",
			"truth_boundary":               "support_guidance_only_not_canonical_truth",
			"heavy_carryover_max":          2,
			"same_incident_foreground_cap": 1,
			"light_resurfacing_enabled":    true,
			"auto_apply":                   false,
			"write_attempted":              false,
			"vector_write_attempted":       false,
			"llm_call_attempted":           false,
		},
	}
}

func narrativeRecallSources(memories []store.Memory, evidence []store.DirectEvidence, storylines []store.Storyline, pendingThreads []store.PendingThread, activeStates []store.ActiveState, canonicalLayers []store.CanonicalStateLayer, episodes []store.EpisodeSummary) []narrativeRecallSource {
	out := []narrativeRecallSource{}
	for _, item := range memories {
		text := prepareTurnMemorySummary(item)
		if text == "" {
			text = ledgerSummaryFromJSONOrText(item.SummaryJSON)
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		out = append(out, narrativeRecallSource{
			SourceType: "memory",
			ID:         item.ID,
			Turn:       item.TurnIndex,
			Text:       text,
			Weight:     item.Importance + item.EmotionalBoost + item.NarrativeSignificance,
			SourceRef:  map[string]any{"type": "memory", "id": item.ID, "turn_index": item.TurnIndex},
		})
	}
	for _, item := range evidence {
		if item.Tombstoned {
			continue
		}
		text := strings.TrimSpace(item.EvidenceText)
		if text == "" {
			continue
		}
		turn := item.TurnAnchor
		if turn <= 0 {
			turn = item.SourceTurnEnd
		}
		out = append(out, narrativeRecallSource{
			SourceType: "direct_evidence",
			ID:         item.ID,
			Turn:       turn,
			Text:       text,
			Weight:     2.0,
			SourceRef:  map[string]any{"type": "direct_evidence", "id": item.ID, "turn_anchor": turn, "capture_verification": item.CaptureVerification},
		})
	}
	for _, item := range storylines {
		text := firstNonEmptyLedgerString(item.CurrentContext, item.OngoingTensionsJSON, item.KeyPointsJSON, item.Name)
		if text == "" || item.Suppressed {
			continue
		}
		out = append(out, narrativeRecallSource{
			SourceType: "storyline",
			ID:         item.ID,
			Turn:       item.LastTurn,
			Text:       text,
			Weight:     item.Confidence + float64(item.EvidenceCount)/10,
			SourceRef:  map[string]any{"type": "storyline", "id": item.ID, "name": item.Name, "last_turn": item.LastTurn},
		})
	}
	for _, item := range pendingThreads {
		if item.Suppressed || !narrativeRecallThreadOpen(item) {
			continue
		}
		text := firstNonEmptyLedgerString(item.Title, item.Description, item.ThreadKey, item.HookMetadataJSON, item.DetailsJSON)
		if text == "" {
			continue
		}
		out = append(out, narrativeRecallSource{
			SourceType: "pending_thread",
			ID:         item.ID,
			Turn:       item.SourceTurn,
			Text:       text,
			Weight:     1.4 + float64(item.Priority)/100,
			SourceRef:  map[string]any{"type": "pending_thread", "id": item.ID, "status": item.Status, "source_turn": item.SourceTurn},
		})
	}
	for _, item := range activeStates {
		text := strings.TrimSpace(item.Content)
		if text == "" {
			continue
		}
		out = append(out, narrativeRecallSource{
			SourceType: "active_state",
			ID:         item.ID,
			Turn:       item.TurnIndex,
			Text:       item.StateType + ": " + text,
			Weight:     1.6,
			SourceRef:  map[string]any{"type": "active_state", "id": item.ID, "state_type": item.StateType, "turn_index": item.TurnIndex},
		})
	}
	for _, item := range canonicalLayers {
		text := strings.TrimSpace(item.Content)
		if text == "" {
			continue
		}
		out = append(out, narrativeRecallSource{
			SourceType: "canonical_state_layer",
			ID:         item.ID,
			Turn:       item.TurnIndex,
			Text:       item.LayerType + ": " + text,
			Weight:     1.8 + item.Confidence,
			SourceRef:  map[string]any{"type": "canonical_state_layer", "id": item.ID, "layer_type": item.LayerType, "turn_index": item.TurnIndex},
		})
	}
	for _, item := range episodes {
		text := firstNonEmptyLedgerString(item.SummaryText, item.KeyEvents, item.RelationshipChangesJSON, item.OpenLoopsJSON)
		if text == "" {
			continue
		}
		out = append(out, narrativeRecallSource{
			SourceType: "episode_summary",
			ID:         item.ID,
			Turn:       item.ToTurn,
			Text:       text,
			Weight:     1.2,
			SourceRef:  map[string]any{"type": "episode_summary", "id": item.ID, "from_turn": item.FromTurn, "to_turn": item.ToTurn},
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Turn == out[j].Turn {
			return out[i].Weight > out[j].Weight
		}
		return out[i].Turn > out[j].Turn
	})
	return out
}

func narrativeRecallRelationshipPacket(sources []narrativeRecallSource, storylines []store.Storyline, pendingThreads []store.PendingThread) map[string]any {
	shift := narrativeRecallFirstMatchingSource(sources, narrativeRecallRelationshipShiftText)
	turning := narrativeRecallFirstMatchingSource(sources, func(text string) bool { return true })
	tensions := []map[string]any{}
	for _, item := range pendingThreads {
		if len(tensions) >= 3 {
			break
		}
		if item.Suppressed || !narrativeRecallThreadOpen(item) {
			continue
		}
		text := firstNonEmptyLedgerString(item.Title, item.Description, item.ThreadKey)
		if text == "" {
			continue
		}
		tensions = append(tensions, map[string]any{
			"summary":    truncateLedgerText(text, 220),
			"source_ref": map[string]any{"type": "pending_thread", "id": item.ID, "status": item.Status, "source_turn": item.SourceTurn},
		})
	}
	for _, item := range storylines {
		if len(tensions) >= 3 {
			break
		}
		text := firstNonEmptyLedgerString(item.OngoingTensionsJSON, item.CurrentContext)
		if text == "" || item.Suppressed {
			continue
		}
		tensions = append(tensions, map[string]any{
			"summary":    truncateLedgerText(text, 220),
			"source_ref": map[string]any{"type": "storyline", "id": item.ID, "name": item.Name, "last_turn": item.LastTurn},
		})
	}
	return map[string]any{
		"relationship_shift": shift,
		"turning_point":      turning,
		"unresolved_tension": tensions,
		"truth_boundary":     "derived_support_packet_not_canonical_truth",
	}
}

func narrativeRecallCarryover(sources []narrativeRecallSource, limit int) map[string]any {
	type group struct {
		key    string
		tokens map[string]bool
		items  []narrativeRecallSource
		score  float64
	}
	groups := []*group{}
	for _, item := range sources {
		tokens := narrativeRecallTokenSet(item.Text)
		key := narrativeRecallIncidentKeyFromSet(tokens)
		if key == "" {
			key = fmt.Sprintf("%s_%d", item.SourceType, item.ID)
		}
		var g *group
		for _, candidate := range groups {
			if narrativeRecallIncidentOverlap(candidate.tokens, tokens) {
				g = candidate
				break
			}
		}
		if g == nil {
			g = &group{key: key, tokens: tokens}
			groups = append(groups, g)
		}
		g.items = append(g.items, item)
		recency := float64(item.Turn) / 100
		g.score += item.Weight + recency
	}
	ordered := make([]*group, 0, len(groups))
	for _, g := range groups {
		sort.SliceStable(g.items, func(i, j int) bool {
			if g.items[i].Turn == g.items[j].Turn {
				return g.items[i].Weight > g.items[j].Weight
			}
			return g.items[i].Turn > g.items[j].Turn
		})
		ordered = append(ordered, g)
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].score > ordered[j].score
	})

	heavy := []map[string]any{}
	light := []map[string]any{}
	heavyLimit := 2
	if limit < heavyLimit {
		heavyLimit = limit
	}
	for _, g := range ordered {
		if len(g.items) == 0 {
			continue
		}
		item := g.items[0]
		entry := map[string]any{
			"incident_key": g.key,
			"summary":      truncateLedgerText(item.Text, 260),
			"source_ref":   item.SourceRef,
			"source_count": len(g.items),
		}
		if len(heavy) < heavyLimit {
			entry["carryover_weight"] = "heavy"
			heavy = append(heavy, entry)
			for _, extra := range g.items[1:] {
				light = append(light, narrativeRecallLightTag(g.key, extra, "same_incident_demoted"))
			}
			continue
		}
		light = append(light, narrativeRecallLightTag(g.key, item, "foreground_diversity_cap"))
		for _, extra := range g.items[1:] {
			light = append(light, narrativeRecallLightTag(g.key, extra, "same_incident_demoted"))
		}
	}
	if len(light) > 12 {
		light = light[:12]
	}
	return map[string]any{
		"heavy_carryover":       heavy,
		"light_resurfacing_tag": light,
		"foreground_policy": map[string]any{
			"same_incident_foreground_cap": 1,
			"heavy_carryover_max":          heavyLimit,
			"repeated_incident_demoted":    true,
		},
	}
}

func narrativeRecallNewSceneOpportunity(carryover map[string]any, pendingThreads []store.PendingThread, storylines []store.Storyline, sceneMicrostate map[string]any) map[string]any {
	for _, item := range pendingThreads {
		if item.Suppressed || !narrativeRecallThreadOpen(item) {
			continue
		}
		text := firstNonEmptyLedgerString(item.Title, item.Description, item.ThreadKey)
		if text == "" {
			continue
		}
		return map[string]any{
			"slot":           "optional_light_opening",
			"summary":        truncateLedgerText(text, 220),
			"source_ref":     map[string]any{"type": "pending_thread", "id": item.ID, "source_turn": item.SourceTurn},
			"scene_type":     sceneMicrostate["scene_type"],
			"authority":      "opportunity_not_mandate",
			"truth_boundary": "support_only",
		}
	}
	for _, item := range storylines {
		text := firstNonEmptyLedgerString(item.OngoingTensionsJSON, item.CurrentContext)
		if text == "" || item.Suppressed {
			continue
		}
		return map[string]any{
			"slot":           "optional_storyline_callback",
			"summary":        truncateLedgerText(text, 220),
			"source_ref":     map[string]any{"type": "storyline", "id": item.ID, "name": item.Name},
			"scene_type":     sceneMicrostate["scene_type"],
			"authority":      "opportunity_not_mandate",
			"truth_boundary": "support_only",
		}
	}
	return map[string]any{
		"slot":           "none",
		"summary":        "",
		"source_ref":     nil,
		"scene_type":     sceneMicrostate["scene_type"],
		"authority":      "opportunity_not_mandate",
		"truth_boundary": "support_only",
	}
}

func narrativeRecallSceneMicrostate(rawUserInput string, chatLogs []store.ChatLog, activeStates []store.ActiveState, canonicalLayers []store.CanonicalStateLayer, pendingThreads []store.PendingThread) map[string]any {
	recent := rawUserInput
	if strings.TrimSpace(recent) == "" {
		for i := len(chatLogs) - 1; i >= 0; i-- {
			if strings.TrimSpace(chatLogs[i].Content) != "" {
				recent = chatLogs[i].Content
				break
			}
		}
	}
	stateText := []string{recent}
	for _, item := range activeStates {
		stateText = append(stateText, item.StateType, item.Content)
	}
	for _, item := range canonicalLayers {
		stateText = append(stateText, item.LayerType, item.Content)
	}
	joined := strings.ToLower(strings.Join(stateText, " "))
	sceneType := "dialogue"
	switch {
	case narrativeRecallContainsAny(joined, "fight", "battle", "검", "전투", "공격", "추격", "위협"):
		sceneType = "action"
	case narrativeRecallContainsAny(joined, "kiss", "date", "love", "romance", "손을", "고백", "연애", "키스", "낭만"):
		sceneType = "romance"
	case narrativeRecallContainsAny(joined, "living room", "kitchen", "bedroom", "거실", "주방", "침대", "집"):
		sceneType = "domestic"
	case narrativeRecallContainsAny(joined, "travel", "arrive", "street", "station", "학교", "거리", "도착"):
		sceneType = "transition"
	}
	pressure := "low"
	openThreads := 0
	for _, item := range pendingThreads {
		if narrativeRecallThreadOpen(item) && !item.Suppressed {
			openThreads++
		}
	}
	if sceneType == "action" || narrativeRecallContainsAny(joined, "danger", "urgent", "위험", "긴급", "위기") {
		pressure = "high"
	} else if openThreads > 0 || narrativeRecallContainsAny(joined, "hesitate", "worry", "tension", "고민", "망설", "긴장") {
		pressure = "medium"
	}
	physical := narrativeRecallPhysicalState(activeStates, canonicalLayers, recent)
	approach := "observe_then_respond"
	switch sceneType {
	case "romance":
		approach = "preserve_emotional_continuity"
	case "action":
		approach = "resolve_immediate_pressure"
	case "domestic":
		approach = "keep_scene_grounded"
	}
	return map[string]any{
		"scene_type":          sceneType,
		"immediate_pressure":  pressure,
		"physical_state":      physical,
		"approach_tendency":   approach,
		"open_thread_count":   openThreads,
		"derived_from_latest": truncateLedgerText(recent, 220),
		"truth_boundary":      "derived_support_only",
	}
}

func narrativeRecallProgressionProfile(requested string, scene map[string]any) map[string]any {
	normalized := narrativeRecallNormalizeProfile(requested)
	resolved := normalized
	source := "request"
	if resolved == "" || resolved == "ai_recommend" {
		resolved = narrativeRecallRecommendProfile(scene)
		source = "ai_recommend"
	}
	labels := map[string]string{
		"calm":      "정온",
		"romance":   "낭만",
		"balanced":  "균형",
		"push":      "추진",
		"turbulent": "격동",
	}
	return map[string]any{
		"requested":         firstNonEmptyLedgerString(requested, "AI추천"),
		"resolved":          resolved,
		"label":             labels[resolved],
		"source":            source,
		"allowed_profiles":  []string{"정온", "낭만", "균형", "추진", "격동", "AI추천"},
		"authority":         "pacing_preference_only",
		"must_not_override": []string{"explicit_user_intent", "current_scene_danger", "canonical_truth"},
	}
}

func narrativeRecallPromptAuthorityTrace(promptDir string) map[string]any {
	trace := buildPromptAssemblyTrace(promptDir)
	supervisor := readPromptFileEvidence(promptDir, "supervisor_system.txt")
	critic := readPromptFileEvidence(promptDir, "critic_system.txt")
	promptRole := func(ev promptFileEvidence, fallbackName string) map[string]any {
		source := "builtin_fallback"
		if ev.Exists && ev.ReadError == "" {
			source = "file"
		}
		return map[string]any{
			"file":                          ev.Name,
			"source":                        source,
			"exists":                        ev.Exists,
			"read_error":                    ev.ReadError,
			"sha256":                        ev.SHA256,
			"char_count":                    ev.CharCount,
			"builtin_fallback_name":         fallbackName,
			"code_fallback_can_override":    false,
			"fallback_only_when_unreadable": true,
		}
	}
	trace["supervisor_system_authority"] = promptRole(supervisor, "supervisor_builtin_json_directive")
	trace["critic_system_authority"] = promptRole(critic, "critic_builtin_json_extractor")
	trace["authority_policy"] = "configured_prompt_file_wins; code fallback is traceable fallback only"
	trace["truth_boundary"] = "prompt_authority_trace_only_no_prompt_write"
	return trace
}

func narrativeRecallFirstMatchingSource(sources []narrativeRecallSource, pred func(string) bool) map[string]any {
	for _, item := range sources {
		if !pred(item.Text) {
			continue
		}
		return map[string]any{
			"summary":    truncateLedgerText(item.Text, 260),
			"source_ref": item.SourceRef,
		}
	}
	return map[string]any{"summary": "", "source_ref": nil}
}

func narrativeRecallLightTag(key string, item narrativeRecallSource, reason string) map[string]any {
	return map[string]any{
		"incident_key":      key,
		"tag":               truncateLedgerText(item.Text, 120),
		"source_ref":        item.SourceRef,
		"carryover_weight":  "light",
		"demotion_reason":   reason,
		"foreground_policy": "do_not_replay_full_incident_unless_current_scene_asks",
	}
}

func narrativeRecallRelationshipShiftText(text string) bool {
	lower := strings.ToLower(text)
	return narrativeRecallContainsAny(lower,
		"trust", "no longer", "closer", "distant", "confess", "promise", "betray", "forgive",
		"믿", "불신", "가까", "멀어", "고백", "약속", "배신", "용서", "관계", "사이",
	)
}

func narrativeRecallThreadOpen(item store.PendingThread) bool {
	switch strings.ToLower(strings.TrimSpace(item.Status)) {
	case "", "open", "active", "pending", "unresolved":
		return true
	default:
		return false
	}
}

func narrativeRecallIncidentKey(text string) string {
	return narrativeRecallIncidentKeyFromSet(narrativeRecallTokenSet(text))
}

func narrativeRecallIncidentKeyFromSet(set map[string]bool) string {
	tokens := make([]string, 0, len(set))
	for token := range set {
		tokens = append(tokens, token)
	}
	sort.Strings(tokens)
	if len(tokens) == 0 {
		return ""
	}
	if len(tokens) > 6 {
		tokens = tokens[:6]
	}
	return strings.Join(tokens, "_")
}

func narrativeRecallTokenSet(text string) map[string]bool {
	set := map[string]bool{}
	for _, token := range narrativeRecallTokens(text) {
		set[token] = true
	}
	return set
}

func narrativeRecallIncidentOverlap(a, b map[string]bool) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	shared := 0
	for token := range a {
		if b[token] {
			shared++
		}
	}
	shorter := len(a)
	if len(b) < shorter {
		shorter = len(b)
	}
	return shared >= 3 || (shorter > 0 && float64(shared)/float64(shorter) >= 0.45)
}

func narrativeRecallTokens(text string) []string {
	set := map[string]bool{}
	var current []rune
	flush := func() {
		if len(current) == 0 {
			return
		}
		token := strings.ToLower(string(current))
		current = current[:0]
		if len([]rune(token)) < 2 || narrativeRecallStopword(token) {
			return
		}
		set[token] = true
	}
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			current = append(current, unicode.ToLower(r))
			continue
		}
		flush()
	}
	flush()
	out := make([]string, 0, len(set))
	for token := range set {
		out = append(out, token)
	}
	sort.Strings(out)
	return out
}

func narrativeRecallStopword(token string) bool {
	switch token {
	case "the", "and", "for", "with", "that", "this", "from", "into", "about", "after", "before", "they", "their", "them", "she", "her", "him", "his", "was", "were", "are", "is", "to", "of", "in", "on", "at", "a", "an", "에게", "그리고", "하지만", "있는", "없는":
		return true
	default:
		return false
	}
}

func narrativeRecallContainsAny(text string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(text, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func narrativeRecallPhysicalState(activeStates []store.ActiveState, canonicalLayers []store.CanonicalStateLayer, fallback string) string {
	for _, item := range activeStates {
		if narrativeRecallContainsAny(strings.ToLower(item.StateType+" "+item.Content), "physical", "location", "scene", "body", "place", "위치", "장면", "몸", "자세") {
			return truncateLedgerText(item.Content, 180)
		}
	}
	for _, item := range canonicalLayers {
		if narrativeRecallContainsAny(strings.ToLower(item.LayerType+" "+item.Content), "physical", "location", "scene", "body", "place", "위치", "장면", "몸", "자세") {
			return truncateLedgerText(item.Content, 180)
		}
	}
	return truncateLedgerText(fallback, 180)
}

func narrativeRecallNormalizeProfile(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "ai", "ai_recommend", "ai추천", "추천", "auto":
		return "ai_recommend"
	case "정온", "calm", "quiet":
		return "calm"
	case "낭만", "romance", "romantic":
		return "romance"
	case "균형", "balanced", "balance":
		return "balanced"
	case "추진", "push", "advance":
		return "push"
	case "격동", "turbulent", "intense":
		return "turbulent"
	default:
		return "ai_recommend"
	}
}

func narrativeRecallRecommendProfile(scene map[string]any) string {
	sceneType := strings.TrimSpace(fmt.Sprint(scene["scene_type"]))
	pressure := strings.TrimSpace(fmt.Sprint(scene["immediate_pressure"]))
	switch {
	case sceneType == "romance":
		return "romance"
	case sceneType == "action" || pressure == "high":
		return "turbulent"
	case pressure == "medium":
		return "push"
	case sceneType == "domestic":
		return "calm"
	default:
		return "balanced"
	}
}

func narrativeRecallIntQuery(raw string, fallback int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}
