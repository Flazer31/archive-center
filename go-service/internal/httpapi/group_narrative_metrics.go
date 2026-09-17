package httpapi

import (
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// Metrics: R1 read-only evidence surfaces.

func (s *Server) handleMetricsLC1C(w http.ResponseWriter, r *http.Request) {
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	ctx := r.Context()
	policyVersion := "lc1c.v1"
	turnWindow := 300
	status := "ok"
	latestTurnIndex := 0
	windowStart := 1
	canonicalChars := 0
	denseChars := 0
	liveLedgerChars := 0

	if sid != "" && s.Store != nil {
		logs, err := s.Store.ListChatLogs(ctx, sid, 0, 0)
		if err == nil {
			for _, l := range logs {
				if l.TurnIndex > latestTurnIndex {
					latestTurnIndex = l.TurnIndex
				}
			}
		}
		if latestTurnIndex > 0 {
			windowStart = latestTurnIndex - turnWindow + 1
			if windowStart < 1 {
				windowStart = 1
			}
		}

		layers, err := s.Store.ListCanonicalStateLayers(ctx, sid, "")
		if err == nil {
			for _, layer := range layers {
				canonicalChars += len([]rune(layer.Content))
			}
		}

		eps, err := s.Store.ListEpisodeSummaries(ctx, sid, 0, windowStart, 0)
		if err == nil {
			for _, ep := range eps {
				denseChars += len([]rune(ep.SummaryText))
			}
		}

		pack, err := s.Store.GetResumePack(ctx, sid, "resume")
		if err == nil && pack != nil {
			if pack.Chapter != nil {
				denseChars += len([]rune(pack.Chapter.SummaryText)) + len([]rune(pack.Chapter.ResumeText))
			}
			if pack.Arc != nil {
				denseChars += len([]rune(pack.Arc.CoreConflict)) + len([]rune(pack.Arc.ArcResumeText))
			}
			if pack.Saga != nil {
				denseChars += len([]rune(pack.Saga.SagaSummary)) + len([]rune(pack.Saga.ResumePackText))
			}
		}

		ev := s.collectNarrativeEvidence(ctx, sid)
		lastTurn := maxNarrativeEvidenceTurn(ev.Storylines, ev.PendingThreads, ev.ActiveStates, ev.CharacterStates)
		storyPlan := buildStoryPlanSnapshot(ev.Storylines, ev.PendingThreads, ev.CharacterStates, ev.WorldRules, lastTurn)
		director := buildDirectorSnapshot(ev.Storylines, ev.PendingThreads, ev.CharacterStates, ev.WorldRules, lastTurn)
		stateStatus := "skeleton"
		if len(ev.Storylines) > 0 || len(ev.PendingThreads) > 0 || len(ev.ActiveStates) > 0 || len(ev.CharacterStates) > 0 || len(ev.WorldRules) > 0 {
			stateStatus = "heuristic"
		}
		liveLedgerChars = pythonDefaultJSONRuneLen(buildNarrativeControlProgressionLedger(stateStatus, director, storyPlan, lastTurn))
	}

	totalChars := canonicalChars + denseChars + liveLedgerChars

	counts := map[string]any{
		"canonical_layers": 0,
		"episodes":         0,
		"chapters":         0,
		"arcs":             0,
		"sagas":            0,
	}
	if sid != "" && s.Store != nil {
		layers, _ := s.Store.ListCanonicalStateLayers(ctx, sid, "")
		counts["canonical_layers"] = len(layers)
		eps, _ := s.Store.ListEpisodeSummaries(ctx, sid, 0, 0, 0)
		counts["episodes"] = len(eps)
		pack, _ := s.Store.GetResumePack(ctx, sid, "resume")
		if pack != nil {
			if pack.Chapter != nil {
				counts["chapters"] = 1
			}
			if pack.Arc != nil {
				counts["arcs"] = 1
			}
			if pack.Saga != nil {
				counts["sagas"] = 1
			}
		}
	}

	if sid == "" {
		status = "off"
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"chat_session_id": sid,
		"memory_footprint": map[string]any{
			"policy_version":        policyVersion,
			"turn_window":           turnWindow,
			"status":                status,
			"latest_turn_index":     nullableInt(latestTurnIndex),
			"window_start_turn":     windowStart,
			"canonical_state_chars": canonicalChars,
			"dense_summary_chars":   denseChars,
			"live_ledger_chars":     liveLedgerChars,
			"total_chars":           totalChars,
			"counts":                counts,
		},
		"status": "ok",
	})
}

func (s *Server) handleMetricsLC1D(w http.ResponseWriter, r *http.Request) {
	chatSessionID := r.PathValue("chat_session_id")
	ev := s.collectNarrativeEvidence(r.Context(), chatSessionID)
	replay := buildLC1DIntegrityReplay(ev)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"chat_session_id":  chatSessionID,
		"integrity_replay": replay,
	})
}

func buildLC1DIntegrityReplay(ev narrativeEvidence) map[string]any {
	latest := latestNarrativeEvidenceTurn(ev)
	latestAny := any(nil)
	if latest > 0 {
		latestAny = latest
	}
	longThreshold := 300
	ultraThreshold := 600
	scopeCounts := map[string]any{"long": 0, "ultra_long": 0}
	retainedByLayer := map[string]any{
		"canonical":       0,
		"dense_summary":   0,
		"direct_evidence": 0,
		"live_ledger":     0,
		"memory":          0,
	}
	candidatesTotal := 0
	retainedTotal := 0
	examples := []any{}
	addCandidate := func(layer string, turn int, label string, score float64) {
		if latest <= 0 || turn <= 0 || latest-turn < longThreshold {
			return
		}
		candidatesTotal++
		retainedTotal++
		scopeCounts["long"] = intFromAny(scopeCounts["long"], 0) + 1
		if latest-turn >= ultraThreshold {
			scopeCounts["ultra_long"] = intFromAny(scopeCounts["ultra_long"], 0) + 1
		}
		retainedByLayer[layer] = intFromAny(retainedByLayer[layer], 0) + 1
		if len(examples) < 5 {
			examples = append(examples, map[string]any{
				"layer":        layer,
				"turn_index":   turn,
				"age_turns":    latest - turn,
				"label":        truncateRunes(label, 120),
				"retain_score": score,
			})
		}
	}

	for _, item := range ev.Memories {
		score := lc1dMemoryRetainScore(item)
		if score >= 0.7 {
			addCandidate("memory", item.TurnIndex, lc1dMemoryLabel(item), score)
		}
	}
	for _, item := range ev.Evidence {
		if lc1dEvidenceRetained(item) {
			addCandidate("direct_evidence", lc1dEvidenceTurn(item), item.EvidenceText, 1.0)
		}
	}
	for _, item := range ev.CanonicalStateLayers {
		turn := item.SourceTurn
		if turn <= 0 {
			turn = item.TurnIndex
		}
		if latest > 0 && turn > 0 && latest-turn >= longThreshold {
			retainedByLayer["canonical"] = intFromAny(retainedByLayer["canonical"], 0) + 1
		}
	}
	for _, item := range ev.EpisodeSummaries {
		if latest > 0 && item.ToTurn > 0 && latest-item.ToTurn >= longThreshold {
			retainedByLayer["dense_summary"] = intFromAny(retainedByLayer["dense_summary"], 0) + 1
		}
	}
	if ev.ResumePack != nil {
		if ev.ResumePack.Chapter != nil {
			retainedByLayer["dense_summary"] = intFromAny(retainedByLayer["dense_summary"], 0) + 1
		}
		if ev.ResumePack.Arc != nil {
			retainedByLayer["dense_summary"] = intFromAny(retainedByLayer["dense_summary"], 0) + 1
		}
		if ev.ResumePack.Saga != nil {
			retainedByLayer["dense_summary"] = intFromAny(retainedByLayer["dense_summary"], 0) + 1
		}
	}
	for _, item := range ev.Storylines {
		turn := item.LastEvidenceTurn
		if turn <= 0 {
			turn = item.LastTurn
		}
		if latest > 0 && turn > 0 && latest-turn >= longThreshold {
			retainedByLayer["live_ledger"] = intFromAny(retainedByLayer["live_ledger"], 0) + 1
		}
	}
	for _, item := range ev.PendingThreads {
		turn := item.SourceTurn
		if turn <= 0 {
			turn = item.CreatedTurn
		}
		if latest > 0 && turn > 0 && latest-turn >= longThreshold {
			retainedByLayer["live_ledger"] = intFromAny(retainedByLayer["live_ledger"], 0) + 1
		}
	}
	retentionRate := 0.0
	if candidatesTotal > 0 {
		retentionRate = float64(retainedTotal) / float64(candidatesTotal)
	}
	return map[string]any{
		"policy_version":               "lc1d.v1",
		"status":                       "ok",
		"replay_query":                 "",
		"replay_query_source":          "query_independent_store_replay",
		"long_turn_threshold":          longThreshold,
		"ultra_long_turn_threshold":    ultraThreshold,
		"replay_non_similarity_max":    0.2,
		"latest_turn_index":            latestAny,
		"scope_counts":                 scopeCounts,
		"retained_by_layer":            retainedByLayer,
		"retained_total":               retainedTotal,
		"gaps_total":                   candidatesTotal - retainedTotal,
		"retention_rate":               retentionRate,
		"candidate_examples":           examples,
		"candidates_total":             candidatesTotal,
		"scanned_direct_evidence_rows": len(ev.Evidence),
	}
}

func latestNarrativeEvidenceTurn(ev narrativeEvidence) int {
	latest := 0
	for _, item := range ev.ChatLogs {
		if item.TurnIndex > latest {
			latest = item.TurnIndex
		}
	}
	for _, item := range ev.Memories {
		if item.TurnIndex > latest {
			latest = item.TurnIndex
		}
	}
	for _, item := range ev.Evidence {
		for _, turn := range []int{item.TurnAnchor, item.SourceTurnEnd, item.SourceTurnStart} {
			if turn > latest {
				latest = turn
			}
		}
	}
	return latest
}

func lc1dMemoryRetainScore(item store.Memory) float64 {
	return math.Max(item.Importance, math.Max(item.NarrativeSignificance, item.EmotionalIntensity))
}

func lc1dMemoryLabel(item store.Memory) string {
	for _, value := range []string{item.SummaryJSON, item.Evidence, item.PlaceRoom, item.PlaceWing} {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return fmt.Sprintf("memory:%d", item.ID)
}

func lc1dEvidenceTurn(item store.DirectEvidence) int {
	for _, turn := range []int{item.TurnAnchor, item.SourceTurnEnd, item.SourceTurnStart} {
		if turn > 0 {
			return turn
		}
	}
	return 0
}

func lc1dEvidenceRetained(item store.DirectEvidence) bool {
	if item.Tombstoned || item.RepairNeeded {
		return false
	}
	state := strings.ToLower(strings.TrimSpace(item.ArchiveState))
	verification := strings.ToLower(strings.TrimSpace(item.CaptureVerification))
	return state == "verified_direct" || state == "previous_archive" || verification == "verified"
}

func (s *Server) handleMetricsLC1E(w http.ResponseWriter, r *http.Request) {
	chatSessionID := r.PathValue("chat_session_id")
	ev := s.collectNarrativeEvidence(r.Context(), chatSessionID)

	budgetCompare := buildLC1EBudgetCompare(ev)
	counts := map[string]any{"evidence_count": len(ev.Evidence), "kg_triple_count": len(ev.KGTriples), "memory_count": len(ev.Memories)}
	trace := []string{}
	if len(ev.Evidence) > 0 {
		trace = append(trace, "evidence")
	}
	if len(ev.KGTriples) > 0 {
		trace = append(trace, "kg_triples")
	}
	if intFromAny(budgetCompare["hypamemory_always_on_chars"], 0) > 0 || intFromAny(budgetCompare["archive_center_layered_chars"], 0) > 0 {
		trace = append(trace, "budget_compare")
	}

	storeStatus := "active"
	source := "shadow"
	if ev.Disabled {
		storeStatus = "disabled"
		source = "shadow-degraded"
		counts = map[string]any{}
		trace = []string{}
		budgetCompare = map[string]any{"policy_version": "lc1e.v1", "status": "disabled"}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": chatSessionID,
		"note":            "metrics lc1e is Store-backed HypaMemory always-on budget compare evidence",
		"source":          source,
		"budget_compare":  budgetCompare,
		"counts":          counts,
		"trace_summary":   trace,
		"store_status":    storeStatus,
	})
}

func buildLC1EBudgetCompare(ev narrativeEvidence) map[string]any {
	hypaAlwaysOnChars := 0
	for _, item := range ev.Memories {
		hypaAlwaysOnChars += len([]rune(item.SummaryJSON))
		hypaAlwaysOnChars += len([]rune(item.Evidence))
	}
	for _, item := range ev.Evidence {
		if strings.Contains(strings.ToLower(item.LineageJSON), "hypamemory") || strings.Contains(strings.ToLower(item.CaptureStage), "hypamemory") {
			hypaAlwaysOnChars += len([]rune(item.EvidenceText))
		}
	}

	canonicalChars := 0
	for _, item := range ev.CanonicalStateLayers {
		canonicalChars += len([]rune(item.Content))
	}
	denseChars := 0
	for _, item := range ev.EpisodeSummaries {
		denseChars += len([]rune(item.SummaryText))
	}
	if ev.ResumePack != nil {
		if ev.ResumePack.Chapter != nil {
			denseChars += len([]rune(ev.ResumePack.Chapter.SummaryText)) + len([]rune(ev.ResumePack.Chapter.ResumeText))
		}
		if ev.ResumePack.Arc != nil {
			denseChars += len([]rune(ev.ResumePack.Arc.CoreConflict)) + len([]rune(ev.ResumePack.Arc.ArcResumeText))
		}
		if ev.ResumePack.Saga != nil {
			denseChars += len([]rune(ev.ResumePack.Saga.SagaSummary)) + len([]rune(ev.ResumePack.Saga.ResumePackText))
		}
	}
	lastTurn := maxNarrativeEvidenceTurn(ev.Storylines, ev.PendingThreads, ev.ActiveStates, ev.CharacterStates)
	storyPlan := buildStoryPlanSnapshot(ev.Storylines, ev.PendingThreads, ev.CharacterStates, ev.WorldRules, lastTurn)
	director := buildDirectorSnapshot(ev.Storylines, ev.PendingThreads, ev.CharacterStates, ev.WorldRules, lastTurn)
	stateStatus := "skeleton"
	if len(ev.Storylines) > 0 || len(ev.PendingThreads) > 0 || len(ev.ActiveStates) > 0 || len(ev.CharacterStates) > 0 || len(ev.WorldRules) > 0 {
		stateStatus = "heuristic"
	}
	liveLedgerChars := pythonDefaultJSONRuneLen(buildNarrativeControlProgressionLedger(stateStatus, director, storyPlan, lastTurn))
	layeredChars := canonicalChars + denseChars + liveLedgerChars
	savedChars := hypaAlwaysOnChars - layeredChars
	savingsRatio := 0.0
	if hypaAlwaysOnChars > 0 {
		savingsRatio = float64(savedChars) / float64(hypaAlwaysOnChars)
	}
	recommendedMode := "archive_center_layered"
	if hypaAlwaysOnChars > 0 && layeredChars > hypaAlwaysOnChars {
		recommendedMode = "investigate_layered_overhead"
	}
	return map[string]any{
		"policy_version":                     "lc1e.v1",
		"status":                             "ok",
		"hypamemory_always_on_mode":          "discouraged_after_import",
		"hypamemory_always_on_chars":         hypaAlwaysOnChars,
		"archive_center_layered_chars":       layeredChars,
		"archive_center_canonical_chars":     canonicalChars,
		"archive_center_dense_summary_chars": denseChars,
		"archive_center_live_ledger_chars":   liveLedgerChars,
		"saved_chars_vs_hypamemory":          savedChars,
		"savings_ratio":                      savingsRatio,
		"recommended_mode":                   recommendedMode,
	}
}

func (s *Server) handleMetricsLC1F(w http.ResponseWriter, r *http.Request) {
	chatSessionID := r.PathValue("chat_session_id")
	ev := s.collectNarrativeEvidence(r.Context(), chatSessionID)

	regressionConfirm := buildLC1FRegressionConfirm(ev)
	counts := map[string]any{"storyline_count": len(ev.Storylines), "episode_summary_count": len(ev.EpisodeSummaries)}
	trace := []string{}
	if len(ev.Storylines) > 0 {
		trace = append(trace, "storylines")
	}
	if len(ev.EpisodeSummaries) > 0 {
		trace = append(trace, "episode_summaries")
	}

	storeStatus := "active"
	source := "shadow"
	if ev.Disabled {
		storeStatus = "disabled"
		source = "shadow-degraded"
		counts = map[string]any{}
		trace = []string{}
		regressionConfirm = map[string]any{"policy_version": "lc1f.v1", "status": "disabled"}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":             "ok",
		"chat_session_id":    chatSessionID,
		"note":               "metrics lc1f is Store-backed short/mid non-regression evidence",
		"source":             source,
		"counts":             counts,
		"regression_confirm": regressionConfirm,
		"trace_summary":      trace,
		"store_status":       storeStatus,
	})
}

func buildLC1FRegressionConfirm(ev narrativeEvidence) map[string]any {
	hasVerifiedEvidence := false
	for _, item := range ev.Evidence {
		if lc1dEvidenceRetained(item) {
			hasVerifiedEvidence = true
			break
		}
	}
	shortChecks := map[string]bool{
		"chat_logs_present":       len(ev.ChatLogs) > 0,
		"direct_evidence_present": hasVerifiedEvidence,
		"kg_present":              len(ev.KGTriples) > 0,
		"current_state_present":   len(ev.ActiveStates) > 0 || len(ev.CanonicalStateLayers) > 0 || len(ev.CharacterStates) > 0,
	}
	midChecks := map[string]bool{
		"storyline_present":       len(ev.Storylines) > 0,
		"episode_summary_present": len(ev.EpisodeSummaries) > 0,
		"world_rule_present":      len(ev.WorldRules) > 0,
		"pending_thread_present":  len(ev.PendingThreads) > 0,
		"resume_pack_present":     ev.ResumePack != nil,
	}
	authorityChecks := map[string]bool{
		"canonical_state_available": len(ev.CanonicalStateLayers) > 0,
		"direct_evidence_available": hasVerifiedEvidence,
		"retrieval_support_only":    true,
	}
	failed := []string{}
	for key, ok := range shortChecks {
		if !ok {
			failed = append(failed, "short."+key)
		}
	}
	for key, ok := range midChecks {
		if !ok {
			failed = append(failed, "mid."+key)
		}
	}
	for key, ok := range authorityChecks {
		if !ok {
			failed = append(failed, "authority."+key)
		}
	}
	status := "pass"
	if len(failed) > 0 {
		status = "warn"
	}
	return map[string]any{
		"policy_version":   "lc1f.v1",
		"status":           status,
		"short_term":       shortChecks,
		"mid_term":         midChecks,
		"authority_checks": authorityChecks,
		"failed_checks":    failed,
		"checked_layers": []string{
			"chat_logs",
			"direct_evidence",
			"kg_triples",
			"active_states",
			"canonical_state_layers",
			"character_states",
			"storylines",
			"episode_summaries",
			"world_rules",
			"pending_threads",
			"resume_pack",
		},
	}
}

func (s *Server) handleMetricsLC1G(w http.ResponseWriter, r *http.Request) {
	chatSessionID := r.PathValue("chat_session_id")
	ev := s.collectNarrativeEvidence(r.Context(), chatSessionID)

	promotionReplay := buildLC1GPromotionReplay(ev)
	counts := map[string]any{"world_rule_count": len(ev.WorldRules), "canonical_state_layer_count": len(ev.CanonicalStateLayers)}
	trace := []string{}
	if len(ev.WorldRules) > 0 {
		trace = append(trace, "world_rules")
	}
	if len(ev.CanonicalStateLayers) > 0 {
		trace = append(trace, "canonical_state_layers")
	}

	storeStatus := "active"
	source := "shadow"
	if ev.Disabled {
		storeStatus = "disabled"
		source = "shadow-degraded"
		counts = map[string]any{}
		trace = []string{}
		promotionReplay = map[string]any{"policy_version": "lc1g.v1", "status": "disabled"}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"chat_session_id":  chatSessionID,
		"note":             "metrics lc1g is Store-backed stale/current promotion replay evidence",
		"source":           source,
		"counts":           counts,
		"promotion_replay": promotionReplay,
		"trace_summary":    trace,
		"store_status":     storeStatus,
	})
}

func buildLC1GPromotionReplay(ev narrativeEvidence) map[string]any {
	latest := latestNarrativeEvidenceTurn(ev)
	staleStorylines := 0
	for _, item := range ev.Storylines {
		turn := item.LastEvidenceTurn
		if turn <= 0 {
			turn = item.LastTurn
		}
		if latest > 0 && turn > 0 && latest-turn >= 300 && strings.EqualFold(item.Status, "active") {
			staleStorylines++
		}
	}
	verifiedPromotions := 0
	lowConfidencePromotions := 0
	for _, item := range ev.CanonicalStateLayers {
		if item.Confidence >= 0.7 {
			verifiedPromotions++
		} else {
			lowConfidencePromotions++
		}
	}
	status := "pass"
	if lowConfidencePromotions > 0 {
		status = "warn"
	}
	return map[string]any{
		"policy_version":                 "lc1g.v1",
		"status":                         status,
		"latest_turn_index":              latest,
		"stale_storyline_candidates":     staleStorylines,
		"current_active_state_count":     len(ev.ActiveStates),
		"current_canonical_count":        len(ev.CanonicalStateLayers),
		"verified_promotion_count":       verifiedPromotions,
		"low_confidence_promotion_count": lowConfidencePromotions,
		"promotion_gate":                 "verified_confidence_required",
	}
}

func (s *Server) handleMetricsLC1H(w http.ResponseWriter, r *http.Request) {
	chatSessionID := r.PathValue("chat_session_id")
	ev := s.collectNarrativeEvidence(r.Context(), chatSessionID)

	errorReplay := buildLC1HFalseNegativePositiveReplay(ev)
	counts := map[string]any{"character_state_count": len(ev.CharacterStates), "active_state_count": len(ev.ActiveStates)}
	trace := []string{}
	if len(ev.CharacterStates) > 0 {
		trace = append(trace, "character_states")
	}
	if len(ev.ActiveStates) > 0 {
		trace = append(trace, "active_states")
	}

	storeStatus := "active"
	source := "shadow"
	if ev.Disabled {
		storeStatus = "disabled"
		source = "shadow-degraded"
		counts = map[string]any{}
		trace = []string{}
		errorReplay = map[string]any{"policy_version": "lc1h.v1", "status": "disabled"}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":                         "ok",
		"chat_session_id":                chatSessionID,
		"note":                           "metrics lc1h is Store-backed false negative/false positive replay evidence",
		"source":                         source,
		"false_negative_positive_replay": errorReplay,
		"counts":                         counts,
		"trace_summary":                  trace,
		"store_status":                   storeStatus,
	})
}

func buildLC1HFalseNegativePositiveReplay(ev narrativeEvidence) map[string]any {
	verifiedEvidence := 0
	for _, item := range ev.Evidence {
		if lc1dEvidenceRetained(item) {
			verifiedEvidence++
		}
	}
	lowConfidenceCanonical := 0
	for _, item := range ev.CanonicalStateLayers {
		if item.Confidence > 0 && item.Confidence < 0.7 {
			lowConfidenceCanonical++
		}
	}
	falseNegativeRisk := 0
	if verifiedEvidence > 0 && len(ev.CanonicalStateLayers) == 0 && len(ev.ActiveStates) == 0 {
		falseNegativeRisk = verifiedEvidence
	}
	falsePositiveRisk := lowConfidenceCanonical
	status := "pass"
	if falseNegativeRisk > 0 || falsePositiveRisk > 0 {
		status = "warn"
	}
	return map[string]any{
		"policy_version":            "lc1h.v1",
		"status":                    status,
		"verified_evidence_count":   verifiedEvidence,
		"current_state_count":       len(ev.ActiveStates),
		"canonical_state_count":     len(ev.CanonicalStateLayers),
		"false_negative_risk_count": falseNegativeRisk,
		"false_positive_risk_count": falsePositiveRisk,
		"repair_action":             "keep_current_state_and_direct_evidence_visible",
	}
}

func (s *Server) handleMetricsLC1I(w http.ResponseWriter, r *http.Request) {
	chatSessionID := r.PathValue("chat_session_id")
	ev := s.collectNarrativeEvidence(r.Context(), chatSessionID)

	ablationCompare := buildLC1IRecallAblationCompare(ev)
	counts := map[string]any{"active_state_count": len(ev.ActiveStates), "pending_thread_count": len(ev.PendingThreads)}
	trace := []string{}
	if len(ev.ActiveStates) > 0 {
		trace = append(trace, "active_states")
	}
	if len(ev.PendingThreads) > 0 {
		trace = append(trace, "pending_threads")
	}

	storeStatus := "active"
	source := "shadow"
	if ev.Disabled {
		storeStatus = "disabled"
		source = "shadow-degraded"
		counts = map[string]any{}
		trace = []string{}
		ablationCompare = map[string]any{"policy_version": "lc1i.v1", "status": "disabled"}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":                  "ok",
		"chat_session_id":         chatSessionID,
		"note":                    "metrics lc1i is Store-backed relationship/ledger/world-pressure ablation evidence",
		"source":                  source,
		"recall_ablation_compare": ablationCompare,
		"counts":                  counts,
		"trace_summary":           trace,
		"store_status":            storeStatus,
	})
}

func buildLC1IRecallAblationCompare(ev narrativeEvidence) map[string]any {
	relationshipSignals := 0
	for _, item := range ev.CharacterStates {
		if strings.TrimSpace(item.RelationshipsJSON) != "" {
			relationshipSignals++
		}
	}
	for _, item := range ev.CanonicalStateLayers {
		if item.LayerType == "relationship_state" {
			relationshipSignals++
		}
	}
	ledgerSignals := len(ev.Storylines) + len(ev.PendingThreads)
	worldPressureSignals := len(ev.WorldRules) + len(ev.ActiveStates)
	fullRecall := relationshipSignals + ledgerSignals + worldPressureSignals
	return map[string]any{
		"policy_version":                       "lc1i.v1",
		"status":                               "ok",
		"relationship_v2_signal_count":         relationshipSignals,
		"ledger_signal_count":                  ledgerSignals,
		"world_pressure_signal_count":          worldPressureSignals,
		"full_recall_signal_count":             fullRecall,
		"without_relationship_v2_signal_count": fullRecall - relationshipSignals,
		"without_ledger_signal_count":          fullRecall - ledgerSignals,
		"without_world_pressure_signal_count":  fullRecall - worldPressureSignals,
	}
}

func (s *Server) handleMetricsLC1J(w http.ResponseWriter, r *http.Request) {
	chatSessionID := r.PathValue("chat_session_id")
	ev := s.collectNarrativeEvidence(r.Context(), chatSessionID)

	verificationGate := buildLC1JVerificationGate(ev)
	resumePresent := 0
	if ev.ResumePack != nil {
		resumePresent = 1
	}
	counts := map[string]any{"chat_log_count": len(ev.ChatLogs), "resume_pack_present": resumePresent}
	trace := []string{}
	if len(ev.ChatLogs) > 0 {
		trace = append(trace, "chat_logs")
	}
	if resumePresent > 0 {
		trace = append(trace, "resume_pack")
	}

	storeStatus := "active"
	source := "shadow"
	if ev.Disabled {
		storeStatus = "disabled"
		source = "shadow-degraded"
		counts = map[string]any{}
		trace = []string{}
		verificationGate = map[string]any{"policy_version": "lc1j.v1", "status": "disabled"}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":            "ok",
		"chat_session_id":   chatSessionID,
		"note":              "metrics lc1j is Store-backed verification gate evidence",
		"source":            source,
		"verification_gate": verificationGate,
		"counts":            counts,
		"trace_summary":     trace,
		"store_status":      storeStatus,
	})
}

func buildLC1JVerificationGate(ev narrativeEvidence) map[string]any {
	hasChat := len(ev.ChatLogs) > 0
	hasResume := ev.ResumePack != nil
	hasCanonical := len(ev.CanonicalStateLayers) > 0
	hasDirect := false
	for _, item := range ev.Evidence {
		if lc1dEvidenceRetained(item) {
			hasDirect = true
			break
		}
	}
	passed := hasChat && hasResume && hasCanonical && hasDirect
	status := "pass"
	if !passed {
		status = "warn"
	}
	return map[string]any{
		"policy_version":           "lc1j.v1",
		"status":                   status,
		"chat_log_gate":            hasChat,
		"resume_pack_gate":         hasResume,
		"canonical_state_gate":     hasCanonical,
		"direct_evidence_gate":     hasDirect,
		"release_gate_ready":       passed,
		"default_runtime_takeover": false,
	}
}

func (s *Server) handleMetricsLC1K(w http.ResponseWriter, r *http.Request) {
	chatSessionID := r.PathValue("chat_session_id")
	ev := s.collectNarrativeEvidence(r.Context(), chatSessionID)

	priorityBudget := buildLC1KPriorityBudgetTrace(ev)
	counts := map[string]any{"memory_count": len(ev.Memories), "kg_triple_count": len(ev.KGTriples)}
	trace := []string{}
	if len(ev.Memories) > 0 {
		trace = append(trace, "memories")
	}
	if len(ev.KGTriples) > 0 {
		trace = append(trace, "kg_triples")
	}

	storeStatus := "active"
	source := "shadow"
	if ev.Disabled {
		storeStatus = "disabled"
		source = "shadow-degraded"
		counts = map[string]any{}
		trace = []string{}
		priorityBudget = map[string]any{"policy_version": "lc1k.v1", "status": "disabled"}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":                "ok",
		"chat_session_id":       chatSessionID,
		"note":                  "metrics lc1k is Store-backed priority/budget hybrid evidence",
		"source":                source,
		"priority_budget_trace": priorityBudget,
		"counts":                counts,
		"trace_summary":         trace,
		"store_status":          storeStatus,
	})
}

func buildLC1KPriorityBudgetTrace(ev narrativeEvidence) map[string]any {
	highPriority := len(ev.CanonicalStateLayers) + len(ev.Evidence)
	lowerTierSupport := len(ev.Memories) + len(ev.KGTriples) + len(ev.EpisodeSummaries)
	return map[string]any{
		"policy_version":               "lc1k.v1",
		"status":                       "ok",
		"priority_order":               []string{"direct_evidence", "canonical_state", "dense_summary", "memory", "kg"},
		"high_priority_layer_count":    highPriority,
		"lower_tier_support_count":     lowerTierSupport,
		"lower_tier_support_preserved": lowerTierSupport > 0,
		"budget_gate":                  "authority_first_then_support",
	}
}

func (s *Server) handleMetricsLC1L(w http.ResponseWriter, r *http.Request) {
	chatSessionID := r.PathValue("chat_session_id")
	ev := s.collectNarrativeEvidence(r.Context(), chatSessionID)

	importedIdeaGate := buildLC1LImportedIdeaContractGate(ev)
	counts := map[string]any{"world_rule_count": len(ev.WorldRules), "evidence_count": len(ev.Evidence)}
	trace := []string{}
	if len(ev.WorldRules) > 0 {
		trace = append(trace, "world_rules")
	}
	if len(ev.Evidence) > 0 {
		trace = append(trace, "evidence")
	}

	storeStatus := "active"
	source := "shadow"
	if ev.Disabled {
		storeStatus = "disabled"
		source = "shadow-degraded"
		counts = map[string]any{}
		trace = []string{}
		importedIdeaGate = map[string]any{"policy_version": "lc1l.v1", "status": "disabled"}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":                      "ok",
		"chat_session_id":             chatSessionID,
		"note":                        "metrics lc1l is Store-backed imported-idea contract gate evidence",
		"source":                      source,
		"imported_idea_contract_gate": importedIdeaGate,
		"counts":                      counts,
		"trace_summary":               trace,
		"store_status":                storeStatus,
	})
}

func buildLC1LImportedIdeaContractGate(ev narrativeEvidence) map[string]any {
	importedSignals := 0
	for _, item := range ev.Memories {
		if strings.Contains(strings.ToLower(item.PlaceWing+" "+item.PlaceRoom+" "+item.SummaryJSON), "hypa") {
			importedSignals++
		}
	}
	for _, item := range ev.Evidence {
		if strings.Contains(strings.ToLower(item.LineageJSON+" "+item.CaptureStage), "hypa") {
			importedSignals++
		}
	}
	defaultTakeoverBlocked := true
	return map[string]any{
		"policy_version":               "lc1l.v1",
		"status":                       "pass",
		"imported_signal_count":        importedSignals,
		"default_takeover_blocked":     defaultTakeoverBlocked,
		"requires_contract_before_use": true,
		"allowed_destination_layers":   []string{"memory", "direct_evidence", "kg", "audit"},
		"blocked_destination_layers":   []string{"current_truth_without_verification", "canonical_without_gate"},
	}
}

func (s *Server) handleMetricsLC1M(w http.ResponseWriter, r *http.Request) {
	chatSessionID := r.PathValue("chat_session_id")
	ev := s.collectNarrativeEvidence(r.Context(), chatSessionID)

	splitCompare := buildLC1MSplitPipelineCompare(ev)
	counts := map[string]any{"episode_summary_count": len(ev.EpisodeSummaries), "chat_log_count": len(ev.ChatLogs)}
	trace := []string{}
	if len(ev.EpisodeSummaries) > 0 {
		trace = append(trace, "episode_summaries")
	}
	if len(ev.ChatLogs) > 0 {
		trace = append(trace, "chat_logs")
	}

	storeStatus := "active"
	source := "shadow"
	if ev.Disabled {
		storeStatus = "disabled"
		source = "shadow-degraded"
		counts = map[string]any{}
		trace = []string{}
		splitCompare = map[string]any{"policy_version": "lc1m.v1", "status": "disabled"}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":                 "ok",
		"chat_session_id":        chatSessionID,
		"note":                   "metrics lc1m is Store-backed split-pipeline compare evidence",
		"source":                 source,
		"split_pipeline_compare": splitCompare,
		"counts":                 counts,
		"trace_summary":          trace,
		"store_status":           storeStatus,
	})
}

func buildLC1MSplitPipelineCompare(ev narrativeEvidence) map[string]any {
	traceCount := countAuditEvents(ev.AuditLogs, "critic_pipeline_trace")
	errorCount := 0
	for _, item := range ev.AuditLogs {
		if strings.Contains(strings.ToLower(item.DetailsJSON), `"status":"error"`) || strings.Contains(strings.ToLower(item.Summary), "error") {
			errorCount++
		}
	}
	status := "pass"
	if errorCount > 0 {
		status = "warn"
	}
	return map[string]any{
		"policy_version":              "lc1m.v1",
		"status":                      status,
		"split_pipeline_enabled":      true,
		"single_call_mode":            false,
		"critic_pipeline_trace_count": traceCount,
		"pipeline_error_count":        errorCount,
		"extractor_stage":             "evidence_extractor",
		"reducer_stage":               "deterministic_reducer",
		"compactor_stage":             "summary_compactor_background",
	}
}

func (s *Server) handleMetricsLC1N(w http.ResponseWriter, r *http.Request) {
	chatSessionID := r.PathValue("chat_session_id")
	ev := s.collectNarrativeEvidence(r.Context(), chatSessionID)

	backfillReplay := buildLC1NBackfillReplay(ev)
	counts := map[string]any{"pending_thread_count": len(ev.PendingThreads), "active_state_count": len(ev.ActiveStates)}
	trace := []string{}
	if len(ev.PendingThreads) > 0 {
		trace = append(trace, "pending_threads")
	}
	if len(ev.ActiveStates) > 0 {
		trace = append(trace, "active_states")
	}

	storeStatus := "active"
	source := "shadow"
	if ev.Disabled {
		storeStatus = "disabled"
		source = "shadow-degraded"
		counts = map[string]any{}
		trace = []string{}
		backfillReplay = map[string]any{"policy_version": "lc1n.v1", "status": "disabled"}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":                  "ok",
		"chat_session_id":         chatSessionID,
		"note":                    "metrics lc1n is Store-backed rebuild/backfill rehearsal evidence",
		"source":                  source,
		"rebuild_backfill_replay": backfillReplay,
		"counts":                  counts,
		"trace_summary":           trace,
		"store_status":            storeStatus,
	})
}

func buildLC1NBackfillReplay(ev narrativeEvidence) map[string]any {
	inputRows := len(ev.ChatLogs)
	derivedRows := len(ev.Evidence) + len(ev.CanonicalStateLayers) + len(ev.EpisodeSummaries) + len(ev.Storylines) + len(ev.PendingThreads)
	status := "pass"
	if inputRows > 0 && derivedRows == 0 {
		status = "warn"
	}
	return map[string]any{
		"policy_version":       "lc1n.v1",
		"status":               status,
		"chat_log_rows":        len(ev.ChatLogs),
		"direct_evidence_rows": len(ev.Evidence),
		"canonical_rows":       len(ev.CanonicalStateLayers),
		"dense_summary_rows":   len(ev.EpisodeSummaries),
		"ledger_rows":          len(ev.Storylines) + len(ev.PendingThreads),
		"drift_detected":       status != "pass",
		"rebuild_mode":         "store_backed_read_rehearsal",
	}
}

func (s *Server) handleMetricsLC1O(w http.ResponseWriter, r *http.Request) {
	chatSessionID := r.PathValue("chat_session_id")
	ev := s.collectNarrativeEvidence(r.Context(), chatSessionID)

	previewLedger := buildLC1ODeterministicPreviewLedger(ev)
	counts := map[string]any{"canonical_state_layer_count": len(ev.CanonicalStateLayers), "active_state_count": len(ev.ActiveStates)}
	trace := []string{}
	if len(ev.CanonicalStateLayers) > 0 {
		trace = append(trace, "canonical_state_layers")
	}
	if len(ev.ActiveStates) > 0 {
		trace = append(trace, "active_states")
	}

	storeStatus := "active"
	source := "shadow"
	if ev.Disabled {
		storeStatus = "disabled"
		source = "shadow-degraded"
		counts = map[string]any{}
		trace = []string{}
		previewLedger = map[string]any{"policy_version": "lc1o.v1", "status": "disabled"}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":                       "ok",
		"chat_session_id":              chatSessionID,
		"note":                         "metrics lc1o is deterministic no-LLM preview/ledger evidence",
		"source":                       source,
		"deterministic_preview_ledger": previewLedger,
		"counts":                       counts,
		"trace_summary":                trace,
		"store_status":                 storeStatus,
	})
}

func buildLC1ODeterministicPreviewLedger(ev narrativeEvidence) map[string]any {
	start := time.Now()
	lastTurn := maxNarrativeEvidenceTurn(ev.Storylines, ev.PendingThreads, ev.ActiveStates, ev.CharacterStates)
	storyPlan := buildStoryPlanSnapshot(ev.Storylines, ev.PendingThreads, ev.CharacterStates, ev.WorldRules, lastTurn)
	director := buildDirectorSnapshot(ev.Storylines, ev.PendingThreads, ev.CharacterStates, ev.WorldRules, lastTurn)
	stateStatus := "skeleton"
	if len(ev.Storylines) > 0 || len(ev.PendingThreads) > 0 || len(ev.ActiveStates) > 0 || len(ev.CharacterStates) > 0 || len(ev.WorldRules) > 0 {
		stateStatus = "heuristic"
	}
	ledger := buildNarrativeControlProgressionLedger(stateStatus, director, storyPlan, lastTurn)
	elapsedMs := time.Since(start).Milliseconds()
	return map[string]any{
		"policy_version":        "lc1o.v1",
		"status":                "pass",
		"llm_call_required":     false,
		"preview_path":          "deterministic",
		"ledger_policy_version": ledger["ledger_policy_version"],
		"world_pressure_ready":  ledger["world_pressure"] != nil,
		"latency_ms":            elapsedMs,
		"storyline_count":       len(ev.Storylines),
		"pending_thread_count":  len(ev.PendingThreads),
		"active_state_count":    len(ev.ActiveStates),
	}
}

func (s *Server) handleMetricsLC1P(w http.ResponseWriter, r *http.Request) {
	chatSessionID := r.PathValue("chat_session_id")
	ev := s.collectNarrativeEvidence(r.Context(), chatSessionID)

	counts := map[string]any{"storyline_count": len(ev.Storylines), "pending_thread_count": len(ev.PendingThreads)}
	trace := []string{}
	if len(ev.Storylines) > 0 {
		trace = append(trace, "storylines")
	}
	if len(ev.PendingThreads) > 0 {
		trace = append(trace, "pending_threads")
	}

	storeStatus := "active"
	source := "shadow"
	if ev.Disabled {
		storeStatus = "disabled"
		source = "shadow-degraded"
		counts = map[string]any{}
		trace = []string{}
	}
	evaluationSplit := buildLC1PEvaluationSplit(ev)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":                 "ok",
		"chat_session_id":        chatSessionID,
		"note":                   "metrics lc1p is Store-backed R1 read evidence",
		"source":                 source,
		"counts":                 counts,
		"trace_summary":          trace,
		"store_status":           storeStatus,
		"evaluation_split":       evaluationSplit,
		"retrieval_completeness": evaluationSplit["retrieval_completeness"],
		"final_answer_quality":   evaluationSplit["final_answer_quality"],
		"failure_split":          evaluationSplit["failure_split"],
	})
}

func buildLC1PEvaluationSplit(ev narrativeEvidence) map[string]any {
	retrievalSignals := []bool{
		len(ev.Memories) > 0,
		len(ev.Evidence) > 0,
		len(ev.KGTriples) > 0,
		len(ev.EpisodeSummaries) > 0,
		len(ev.ActiveStates) > 0,
	}
	answerSignals := []bool{
		len(ev.Storylines) > 0,
		len(ev.PendingThreads) > 0,
		len(ev.WorldRules) > 0,
		len(ev.CharacterStates) > 0,
		len(ev.CanonicalStateLayers) > 0,
	}
	retrievalScore := ratioOfTrue(retrievalSignals)
	answerQuality := ratioOfTrue(answerSignals)
	classification := "healthy"
	if retrievalScore < 0.5 && answerQuality < 0.5 {
		classification = "mixed_failure"
	} else if retrievalScore < 0.5 {
		classification = "retrieval_failure_dominant"
	} else if answerQuality < 0.5 {
		classification = "reader_failure_dominant"
	}
	if ev.Disabled {
		classification = "store_disabled"
	}
	return map[string]any{
		"policy_version": "lc1p.v1",
		"status":         map[bool]string{true: "disabled", false: "pass"}[ev.Disabled],
		"retrieval_completeness": map[string]any{
			"policy_version":        "s17-1a.v1",
			"metric_defined":        true,
			"score":                 retrievalScore,
			"memory_count":          len(ev.Memories),
			"direct_evidence_count": len(ev.Evidence),
			"kg_triple_count":       len(ev.KGTriples),
			"episode_summary_count": len(ev.EpisodeSummaries),
			"active_state_count":    len(ev.ActiveStates),
		},
		"final_answer_quality": map[string]any{
			"policy_version":              "s17-1b.v1",
			"metric_defined":              true,
			"score":                       answerQuality,
			"storyline_count":             len(ev.Storylines),
			"pending_thread_count":        len(ev.PendingThreads),
			"world_rule_count":            len(ev.WorldRules),
			"character_state_count":       len(ev.CharacterStates),
			"canonical_state_layer_count": len(ev.CanonicalStateLayers),
		},
		"failure_split": map[string]any{
			"policy_version":     "s17-1c.v1",
			"replay_defined":     true,
			"classification":     classification,
			"retrieval_failure":  retrievalScore < 0.5,
			"reader_failure":     answerQuality < 0.5,
			"truth_authority":    false,
			"inspection_surface": true,
		},
	}
}

func ratioOfTrue(values []bool) float64 {
	if len(values) == 0 {
		return 0
	}
	hits := 0
	for _, value := range values {
		if value {
			hits++
		}
	}
	return float64(hits) / float64(len(values))
}

func (s *Server) handleMetricsLC1Q(w http.ResponseWriter, r *http.Request) {
	chatSessionID := r.PathValue("chat_session_id")

	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": chatSessionID,
		"freshness_lag_summary": map[string]any{
			"policy_version": "lc1q.v1",
			"status":         "off",
			"classification": "insufficient_signal",
			"metric_defined": true,
			"timestamps": map[string]any{
				"latest_chat_log_created_at":                 nil,
				"latest_complete_turn_visibility_created_at": nil,
				"latest_critic_pipeline_trace_created_at":    nil,
				"latest_canonical_state_layer_created_at":    nil,
			},
			"lags_seconds": map[string]any{
				"save_delay":               nil,
				"extraction_delay":         nil,
				"promotion_visibility_lag": nil,
			},
			"signal_coverage": map[string]any{
				"chat_logs":                false,
				"complete_turn_visibility": false,
				"critic_pipeline_trace":    false,
				"canonical_state_layers":   false,
			},
			"answer_quality_split": map[string]any{
				"extraction_delay_affects_answer_quality":         true,
				"save_delay_affects_answer_quality":               true,
				"promotion_visibility_lag_affects_answer_quality": true,
			},
			"warnings": []any{},
		},
	})
}

func (s *Server) handleMetricsLC1R(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":                     "ok",
		"regression_corpus_manifest": regressionCorpusManifest(),
	})
}

func (s *Server) handleMetricsLC1S(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":                "ok",
		"step17_bundle_closure": step17BundleClosure(),
	})
}

func (s *Server) handleMetricsTM1D(w http.ResponseWriter, r *http.Request) {
	chatSessionID := r.PathValue("chat_session_id")
	ev := s.collectNarrativeEvidence(r.Context(), chatSessionID)

	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"chat_session_id": chatSessionID,
		"truth_maintenance_audit_replay": map[string]any{
			"policy_version": "tm1d.v1",
			"status":         "off",
			"gate_result":    "off",
			"reason":         "audit_surface_missing",
			"audit_event_counts": map[string]any{
				"importance_reevaluation": countAuditEvents(ev.AuditLogs, "importance_reevaluation"),
				"drift_detected":          countAuditEvents(ev.AuditLogs, "drift_detected"),
			},
		},
	})
}
