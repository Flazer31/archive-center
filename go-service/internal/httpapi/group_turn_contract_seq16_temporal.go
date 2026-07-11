package httpapi

import (
	"fmt"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

func buildValidityWindowReading(sid string, chatLogs []store.ChatLog, episodeSums []store.EpisodeSummary, evidence []store.DirectEvidence, memories []store.Memory) map[string]any {
	latestChatTurn := 0
	latestChatRole := ""
	for _, cl := range chatLogs {
		if cl.TurnIndex > latestChatTurn {
			latestChatTurn = cl.TurnIndex
			latestChatRole = cl.Role
		}
	}
	latestEpisodeTo := 0
	latestEpisodeFrom := 0
	for _, ep := range episodeSums {
		if ep.ToTurn > latestEpisodeTo {
			latestEpisodeTo = ep.ToTurn
			latestEpisodeFrom = ep.FromTurn
		}
	}
	windowStart := latestEpisodeFrom
	if windowStart == 0 {
		windowStart = 1
	}
	windowEnd := latestChatTurn
	if windowEnd == 0 {
		windowEnd = latestEpisodeTo
	}
	invalidated := false
	invalidationReason := ""
	for _, e := range evidence {
		if e.TurnAnchor < latestChatTurn {
			invalidated = true
			invalidationReason = "newer_chat_turn_exists"
			break
		}
	}
	if !invalidated {
		for _, m := range memories {
			if m.TurnIndex < latestChatTurn {
				invalidated = true
				invalidationReason = "newer_chat_turn_exists"
				break
			}
		}
	}
	return map[string]any{
		"version":             "p193a.v1",
		"chat_session_id":     sid,
		"window_policy":       "validity_first_invalidation_reading",
		"window_start":        windowStart,
		"window_end":          windowEnd,
		"latest_chat_turn":    latestChatTurn,
		"latest_chat_role":    latestChatRole,
		"latest_episode_from": latestEpisodeFrom,
		"latest_episode_to":   latestEpisodeTo,
		"invalidated":         invalidated,
		"invalidation_reason": invalidationReason,
		"truth_store":         "maria_db",
		"retrieval_role":      "support_accelerator_only",
		"reason":              "seq16_p193_validity_window_invalidation_reading",
	}
}

// buildTruthCoexistenceRules exposes the current-truth vs old-truth
// coexistence rules surface (SEQ-16-P194). It lists evidence items
// with their authority status and whether a newer item supersedes them.
func buildTruthCoexistenceRules(sid string, evidence []store.DirectEvidence, memories []store.Memory, chatLogs []store.ChatLog) map[string]any {
	latestTurn := 0
	for _, cl := range chatLogs {
		if cl.TurnIndex > latestTurn {
			latestTurn = cl.TurnIndex
		}
	}
	for _, m := range memories {
		if m.TurnIndex > latestTurn {
			latestTurn = m.TurnIndex
		}
	}
	items := []map[string]any{}
	for _, e := range evidence {
		status := "current_truth"
		if e.TurnAnchor < latestTurn {
			status = "old_truth"
		}
		items = append(items, map[string]any{
			"id":           e.ID,
			"source_type":  "direct_evidence",
			"turn_anchor":  e.TurnAnchor,
			"status":       status,
			"authority":    true,
			"superseded":   status == "old_truth",
			"coexist_rule": "canonical_evidence_kept_both_current_and_old",
		})
	}
	for _, m := range memories {
		status := "current_support"
		if m.TurnIndex < latestTurn {
			status = "old_support"
		}
		items = append(items, map[string]any{
			"id":           m.ID,
			"source_type":  "memory",
			"turn_index":   m.TurnIndex,
			"status":       status,
			"authority":    false,
			"superseded":   false,
			"coexist_rule": "support_only_never_supersedes_truth",
		})
	}
	return map[string]any{
		"version":         "p194a.v1",
		"chat_session_id": sid,
		"coexist_policy":  "current_truth_vs_old_truth_kept",
		"items":           items,
		"item_count":      len(items),
		"latest_turn":     latestTurn,
		"truth_store":     "maria_db",
		"retrieval_role":  "support_accelerator_only",
		"reason":          "seq16_p194_current_truth_vs_old_truth_coexistence_rules",
	}
}

// buildTemporalDisambiguationContract exposes the event retrieval /
// temporal disambiguation contract surface (SEQ-16-P195). It maps each
// event-like record to a temporal bucket and marks whether the bucket
// is ambiguous (overlapping turns).
func buildTemporalDisambiguationContract(sid string, chatLogs []store.ChatLog, episodeSums []store.EpisodeSummary, evidence []store.DirectEvidence, memories []store.Memory) map[string]any {
	buckets := []map[string]any{}
	for _, ep := range episodeSums {
		buckets = append(buckets, map[string]any{
			"bucket_id":     fmt.Sprintf("ep_%d_%d", ep.FromTurn, ep.ToTurn),
			"bucket_type":   "episode_summary",
			"from_turn":     ep.FromTurn,
			"to_turn":       ep.ToTurn,
			"ambiguous":     false,
			"disambiguated": true,
			"source_depth":  "derived_summary",
		})
	}
	for _, e := range evidence {
		buckets = append(buckets, map[string]any{
			"bucket_id":     fmt.Sprintf("ev_%d", e.ID),
			"bucket_type":   "direct_evidence",
			"from_turn":     e.SourceTurnStart,
			"to_turn":       e.SourceTurnEnd,
			"ambiguous":     e.SourceTurnStart != e.SourceTurnEnd,
			"disambiguated": e.SourceTurnStart == e.SourceTurnEnd,
			"source_depth":  "canonical_evidence",
		})
	}
	for _, m := range memories {
		buckets = append(buckets, map[string]any{
			"bucket_id":     fmt.Sprintf("mem_%d", m.ID),
			"bucket_type":   "memory",
			"from_turn":     m.TurnIndex,
			"to_turn":       m.TurnIndex,
			"ambiguous":     false,
			"disambiguated": true,
			"source_depth":  "derived_summary",
		})
	}
	for _, cl := range chatLogs {
		buckets = append(buckets, map[string]any{
			"bucket_id":     fmt.Sprintf("cl_%d", cl.ID),
			"bucket_type":   "chat_log",
			"from_turn":     cl.TurnIndex,
			"to_turn":       cl.TurnIndex,
			"ambiguous":     false,
			"disambiguated": true,
			"source_depth":  "raw_turn",
		})
	}
	return map[string]any{
		"version":         "p195a.v1",
		"chat_session_id": sid,
		"disambig_policy": "turn_span_exactness",
		"buckets":         buckets,
		"bucket_count":    len(buckets),
		"truth_store":     "maria_db",
		"retrieval_role":  "support_accelerator_only",
		"reason":          "seq16_p195_event_retrieval_temporal_disambiguation_contract",
	}
}

// buildPromotionLagInvisibilitySplit exposes the current vs pending-current
// vs old-truth read contract with promotion-lag invisibility split
// (SEQ-16-P196). It marks pending threads and canonical layers with
// their promotion status so callers know what is not yet visible.
func buildPromotionLagInvisibilitySplit(sid string, pendingThreads []store.PendingThread, canonicalLayers []store.CanonicalStateLayer, chatLogs []store.ChatLog, evidence []store.DirectEvidence) map[string]any {
	latestChatTurn := 0
	for _, cl := range chatLogs {
		if cl.TurnIndex > latestChatTurn {
			latestChatTurn = cl.TurnIndex
		}
	}
	latestEvidenceTurn := 0
	for _, e := range evidence {
		if e.TurnAnchor > latestEvidenceTurn {
			latestEvidenceTurn = e.TurnAnchor
		}
	}
	reads := []map[string]any{}
	for _, pt := range pendingThreads {
		status := "pending_current"
		if pt.CreatedTurn < latestChatTurn {
			status = "old_pending"
		}
		reads = append(reads, map[string]any{
			"id":            pt.ID,
			"source_type":   "pending_thread",
			"created_turn":  pt.CreatedTurn,
			"status":        status,
			"visibility":    "invisible_until_promoted",
			"promotion_lag": latestChatTurn - pt.CreatedTurn,
			"authority":     false,
		})
	}
	for _, cl := range canonicalLayers {
		status := "current_truth"
		visibility := "visible_canonical_truth"
		if latestChatTurn > 0 && cl.TurnIndex < latestChatTurn {
			status = "old_truth"
			visibility = "visible_historical_truth"
		}
		reads = append(reads, map[string]any{
			"id":            cl.ID,
			"source_type":   "canonical_state_layer",
			"turn_index":    cl.TurnIndex,
			"status":        status,
			"visibility":    visibility,
			"promotion_lag": maxInt(latestChatTurn-cl.TurnIndex, 0),
			"authority":     true,
		})
	}
	for _, e := range evidence {
		status := "current_truth"
		visibility := "visible_canonical_truth"
		if latestChatTurn > 0 && e.TurnAnchor < latestChatTurn {
			status = "old_truth"
			visibility = "visible_historical_truth"
		}
		reads = append(reads, map[string]any{
			"id":            e.ID,
			"source_type":   "direct_evidence",
			"turn_anchor":   e.TurnAnchor,
			"status":        status,
			"visibility":    visibility,
			"promotion_lag": maxInt(latestChatTurn-e.TurnAnchor, 0),
			"authority":     true,
		})
	}
	currentTruthCount := 0
	oldTruthCount := 0
	pendingCurrentCount := 0
	oldPendingCount := 0
	for _, r := range reads {
		s, _ := r["status"].(string)
		switch s {
		case "current_truth":
			currentTruthCount++
		case "old_truth":
			oldTruthCount++
		case "pending_current":
			pendingCurrentCount++
		case "old_pending":
			oldPendingCount++
		}
	}
	return map[string]any{
		"version":               "p196a.v1",
		"chat_session_id":       sid,
		"split_policy":          "current_vs_pending_current_vs_old_truth",
		"reads":                 reads,
		"read_count":            len(reads),
		"current_truth_count":   currentTruthCount,
		"pending_current_count": pendingCurrentCount,
		"old_truth_count":       oldTruthCount,
		"old_pending_count":     oldPendingCount,
		"latest_chat_turn":      latestChatTurn,
		"latest_evidence_turn":  latestEvidenceTurn,
		"truth_store":           "maria_db",
		"retrieval_role":        "support_accelerator_only",
		"reason":                "seq16_p196_promotion_lag_invisibility_split",
	}
}

// buildSessionPermanentAuthorityReplay replays the session/permanent
// authority split from the existing retrieval_role_boundary surface
// (SEQ-16-P200). It confirms the boundary is still stable and
// authority-aware.
func buildSessionPermanentAuthorityReplay(sid string, retrievalRoleBoundary map[string]any) map[string]any {
	permCount := 0
	sessCount := 0
	if pc, ok := retrievalRoleBoundary["permanent_item_count"].(int); ok {
		permCount = pc
	}
	if sc, ok := retrievalRoleBoundary["session_item_count"].(int); ok {
		sessCount = sc
	}
	boundaryStable := retrievalRoleBoundary["split_policy"] == "session_permanent_role_boundary" &&
		retrievalRoleBoundary["permanent_role"] == "permanent" &&
		retrievalRoleBoundary["session_role"] == "session"
	authorityAware := boundaryStable && (permCount > 0 || sessCount > 0)
	return map[string]any{
		"version":         "p200a.v1",
		"chat_session_id": sid,
		"replay_policy":   "session_permanent_authority_replay",
		"permanent_count": permCount,
		"session_count":   sessCount,
		"boundary_stable": boundaryStable,
		"authority_aware": authorityAware,
		"truth_store":     "maria_db",
		"retrieval_role":  "support_accelerator_only",
		"reason":          "seq16_p200_session_permanent_authority_replay",
	}
}

// buildNormalizedUnitSupportOnlyReplay replays the normalized retrieval
// unit schema and confirms every unit remains support-only with
// truth_authority=false (SEQ-16-P201).
func buildNormalizedUnitSupportOnlyReplay(sid string, retrievalUnitsIR map[string]any) map[string]any {
	unitCount := 0
	if uc, ok := retrievalUnitsIR["unit_count"].(int); ok {
		unitCount = uc
	}
	units := mapSliceFromAny(retrievalUnitsIR["units"])
	allSupportOnly := retrievalUnitsIR["support_only"] == true && len(units) == unitCount && unitCount > 0
	for _, u := range units {
		if u["unit_schema"] != "normalized_retrieval_unit_v1" || u["truth_authority"] != false {
			allSupportOnly = false
			break
		}
	}
	return map[string]any{
		"version":                "p201a.v1",
		"chat_session_id":        sid,
		"replay_policy":          "normalized_unit_support_only_replay",
		"unit_count":             unitCount,
		"inspected_unit_count":   len(units),
		"top_level_support_only": retrievalUnitsIR["support_only"] == true,
		"all_support_only":       allSupportOnly,
		"truth_store":            "maria_db",
		"retrieval_role":         "support_accelerator_only",
		"reason":                 "seq16_p201_normalized_unit_support_only_replay",
	}
}

// buildMultiSignalRetrievalInspectionReplay replays the multi-signal
// retrieval inspection and confirms signal mix + lane inspection are
// still consistent (SEQ-16-P202).
func buildMultiSignalRetrievalInspectionReplay(sid string, signalMixContract map[string]any, retrievalResultInspection map[string]any) map[string]any {
	signalCount := 0
	laneCount := 0
	if sc, ok := signalMixContract["signal_count"].(int); ok {
		signalCount = sc
	}
	if lc, ok := retrievalResultInspection["lane_count"].(int); ok {
		laneCount = lc
	}
	signals := mapSliceFromAny(signalMixContract["signals"])
	lanes := mapSliceFromAny(retrievalResultInspection["lanes"])
	signalsSupportOnly := signalMixContract["retrieval_role"] == "support_accelerator_only" && len(signals) == signalCount && signalCount > 0
	for _, s := range signals {
		if s["truth_authority"] != false {
			signalsSupportOnly = false
			break
		}
	}
	inspectionStable := signalsSupportOnly &&
		retrievalResultInspection["retrieval_role"] == "support_accelerator_only" &&
		len(lanes) == laneCount &&
		laneCount > 0
	return map[string]any{
		"version":                "p202a.v1",
		"chat_session_id":        sid,
		"replay_policy":          "multi_signal_retrieval_inspection_replay",
		"signal_count":           signalCount,
		"inspected_signal_count": len(signals),
		"lane_count":             laneCount,
		"inspected_lane_count":   len(lanes),
		"signals_support_only":   signalsSupportOnly,
		"inspection_stable":      inspectionStable,
		"truth_store":            "maria_db",
		"retrieval_role":         "support_accelerator_only",
		"reason":                 "seq16_p202_multi_signal_retrieval_inspection_replay",
	}
}

// buildValidityWindowTemporalReplay replays the validity-first temporal
// read and the validity-window reading to confirm temporal consistency
// (SEQ-16-P203).
func buildValidityWindowTemporalReplay(sid string, temporalReadValidityFirst map[string]any, validityWindowReading map[string]any) map[string]any {
	latestChatTurn := 0
	if v, ok := temporalReadValidityFirst["latest_chat_turn"].(int); ok {
		latestChatTurn = v
	}
	windowEnd := 0
	if v, ok := validityWindowReading["window_end"].(int); ok {
		windowEnd = v
	}
	consistent := latestChatTurn > 0 && windowEnd > 0 && latestChatTurn == windowEnd &&
		temporalReadValidityFirst["validity_first"] == true &&
		validityWindowReading["window_policy"] == "validity_first_invalidation_reading"
	return map[string]any{
		"version":             "p203a.v1",
		"chat_session_id":     sid,
		"replay_policy":       "validity_window_temporal_replay",
		"latest_chat_turn":    latestChatTurn,
		"window_end":          windowEnd,
		"temporal_consistent": consistent,
		"truth_store":         "maria_db",
		"retrieval_role":      "support_accelerator_only",
		"reason":              "seq16_p203_validity_window_temporal_replay",
	}
}

// buildSourceTaggedAuthorityAwareAssemblyReplay replays the source-tagged
// retrieval unit surface and the retrieval role boundary to confirm
// authority-aware assembly is still tagged correctly (SEQ-16-P204).
func buildSourceTaggedAuthorityAwareAssemblyReplay(sid string, sourceTaggedRetrievalUnitSurface map[string]any, retrievalRoleBoundary map[string]any) map[string]any {
	taggedCount := 0
	if tc, ok := sourceTaggedRetrievalUnitSurface["tagged_count"].(int); ok {
		taggedCount = tc
	}
	taggedUnits := mapSliceFromAny(sourceTaggedRetrievalUnitSurface["tagged_units"])
	allUnitsTagged := len(taggedUnits) == taggedCount && taggedCount > 0
	for _, unit := range taggedUnits {
		if unit["unit_id"] == "" || unit["source_tag"] == "" || unit["source_type"] == "" {
			allUnitsTagged = false
			break
		}
	}
	boundaryStable := false
	if bs, ok := retrievalRoleBoundary["split_policy"].(string); ok && bs != "" {
		boundaryStable = true
	}
	authorityAware := allUnitsTagged && boundaryStable &&
		retrievalRoleBoundary["permanent_role"] == "permanent" &&
		retrievalRoleBoundary["session_role"] == "session"
	return map[string]any{
		"version":             "p204a.v1",
		"chat_session_id":     sid,
		"replay_policy":       "source_tagged_authority_aware_assembly_replay",
		"tagged_count":        taggedCount,
		"inspected_tag_count": len(taggedUnits),
		"all_units_tagged":    allUnitsTagged,
		"boundary_stable":     boundaryStable,
		"authority_aware":     authorityAware,
		"truth_store":         "maria_db",
		"retrieval_role":      "support_accelerator_only",
		"reason":              "seq16_p204_source_tagged_authority_aware_assembly_replay",
	}
}

// buildCriticTruncationSpilloverReplay replays the raw-turn span metadata,
// sparse-tail recall, and normalized retrieval units to confirm the
// summary/raw/evidence route is visible and recall-verifiable
// (SEQ-16-P205).
func buildCriticTruncationSpilloverReplay(sid string, rawTurnSpanMetadata map[string]any, sparseTailRecall map[string]any, retrievalUnitsIR map[string]any) map[string]any {
	spanCount := 0
	if sc, ok := rawTurnSpanMetadata["span_count"].(int); ok {
		spanCount = sc
	}
	routeCount := 0
	if rc, ok := sparseTailRecall["route_count"].(int); ok {
		routeCount = rc
	}
	unitCount := 0
	if uc, ok := retrievalUnitsIR["unit_count"].(int); ok {
		unitCount = uc
	}
	routes := mapSliceFromAny(sparseTailRecall["routes"])
	hasSummaryRoute := false
	hasRawEvidenceRoute := false
	hasDirectPointerRoute := false
	for _, route := range routes {
		switch route["route_name"] {
		case "dense_summary":
			hasSummaryRoute = route["summary_only"] == true
		case "raw_evidence_support":
			hasRawEvidenceRoute = route["summary_only"] == false && route["has_direct_pointer"] == true
		}
		if route["has_direct_pointer"] == true {
			hasDirectPointerRoute = true
		}
	}
	return map[string]any{
		"version":                  "p205a.v1",
		"chat_session_id":          sid,
		"replay_policy":            "critic_truncation_spillover_replay",
		"span_count":               spanCount,
		"route_count":              routeCount,
		"inspected_route_count":    len(routes),
		"unit_count":               unitCount,
		"has_summary_route":        hasSummaryRoute,
		"has_raw_evidence_route":   hasRawEvidenceRoute,
		"has_direct_pointer_route": hasDirectPointerRoute,
		"recall_verifiable":        spanCount > 0 && routeCount == len(routes) && unitCount > 0 && hasSummaryRoute && hasRawEvidenceRoute && hasDirectPointerRoute,
		"truth_store":              "maria_db",
		"retrieval_role":           "support_accelerator_only",
		"reason":                   "seq16_p205_critic_truncation_spillover_replay",
	}
}

// buildSessionPartitionedIndex exposes the session-partitioned index
// surface that remigrates the legacy backend/tests/test_q1b_session_partitioned_index.py
// contract (SEQ-16-P209). It shows the retrieval index is scoped per session
// and lists document tiers.
func buildSessionPartitionedIndex(sid string, documents []map[string]any, indexSnapshot map[string]any) map[string]any {
	tiers := map[string]int{}
	authorityTiers := map[string]int{"canonical": 0, "support": 0, "fallback": 0}
	for _, doc := range documents {
		tier, _ := doc["tier"].(string)
		if tier == "" {
			tier = "unknown"
		}
		tiers[tier]++
		switch tier {
		case "evidence":
			authorityTiers["canonical"]++
		case "chat_log":
			authorityTiers["fallback"]++
		default:
			authorityTiers["support"]++
		}
	}
	return map[string]any{
		"version":               "p209a.v1",
		"chat_session_id":       sid,
		"index_policy":          "session_partitioned",
		"document_count":        len(documents),
		"tier_counts":           tiers,
		"authority_tier_counts": authorityTiers,
		"index_snapshot":        indexSnapshot,
		"truth_store":           "maria_db",
		"retrieval_role":        "support_accelerator_only",
		"reason":                "seq16_p209_session_partitioned_index",
	}
}

// buildIndexLifecycle exposes the index lifecycle surface that remigrates
// the legacy backend/tests/test_q1c_index_lifecycle.py contract
// (SEQ-16-P210). It reports vector shadow health and rebuild readiness.
func buildIndexLifecycle(sid string, vectorShadow map[string]any) map[string]any {
	shadowStatus := "off"
	if s, ok := vectorShadow["status"].(string); ok {
		shadowStatus = s
	}
	searchAttempted := false
	if a, ok := vectorShadow["search_attempted"].(bool); ok {
		searchAttempted = a
	}
	configured := false
	if c, ok := vectorShadow["configured"].(bool); ok {
		configured = c
	}
	healthChecked := false
	if h, ok := vectorShadow["health_checked"].(bool); ok {
		healthChecked = h
	}
	modelReady := false
	if m, ok := vectorShadow["model_ready"].(bool); ok {
		modelReady = m
	}
	rebuildReady := configured && healthChecked && modelReady && (shadowStatus == "ready" || shadowStatus == "ok" || shadowStatus == "healthy")
	return map[string]any{
		"version":          "p210a.v1",
		"chat_session_id":  sid,
		"lifecycle_policy": "index_lifecycle_shadow",
		"shadow_status":    shadowStatus,
		"configured":       configured,
		"health_checked":   healthChecked,
		"model_ready":      modelReady,
		"search_attempted": searchAttempted,
		"rebuild_ready":    rebuildReady,
		"truth_store":      "maria_db",
		"retrieval_role":   "support_accelerator_only",
		"reason":           "seq16_p210_index_lifecycle",
	}
}

// buildSourceLookupAudit exposes the source lookup audit surface that
// remigrates the legacy backend/tests/test_q1d_source_lookup_audit.py
// contract (SEQ-16-P211). It lists evidence and memory sources with
// audit trail metadata without claiming truth authority.
func buildSourceLookupAudit(sid string, evidence []store.DirectEvidence, memories []store.Memory, kgTriples []store.KGTriple, chatLogs []store.ChatLog) map[string]any {
	sources := []map[string]any{}
	for _, e := range evidence {
		sources = append(sources, map[string]any{
			"id":           e.ID,
			"source_type":  "direct_evidence",
			"turn_anchor":  e.TurnAnchor,
			"audit_status": "canonical_truth",
			"authority":    true,
		})
	}
	for _, m := range memories {
		sources = append(sources, map[string]any{
			"id":           m.ID,
			"source_type":  "memory",
			"turn_index":   m.TurnIndex,
			"audit_status": "support_only",
			"authority":    false,
		})
	}
	for _, k := range kgTriples {
		sources = append(sources, map[string]any{
			"id":           k.ID,
			"source_type":  "kg_triple",
			"source_turn":  k.SourceTurn,
			"audit_status": "support_only",
			"authority":    false,
		})
	}
	for _, c := range chatLogs {
		sources = append(sources, map[string]any{
			"id":           c.ID,
			"source_type":  "chat_log",
			"turn_index":   c.TurnIndex,
			"audit_status": "fallback_support",
			"authority":    false,
		})
	}
	return map[string]any{
		"version":         "p211a.v1",
		"chat_session_id": sid,
		"audit_policy":    "source_lookup_inspectable",
		"sources":         sources,
		"source_count":    len(sources),
		"truth_store":     "maria_db",
		"retrieval_role":  "support_accelerator_only",
		"reason":          "seq16_p211_source_lookup_audit",
	}
}

// buildRuntimeToggle exposes the runtime toggle surface that remigrates
// the legacy backend/tests/test_q1e_runtime_toggle.py contract
// (SEQ-16-P212). It reports guarded/shadow toggle states without broad
// takeover.
func buildRuntimeToggle(sid string, degraded bool, injectionEnabled bool, inputContextEnabled bool, maxInjectionChars int, maxInputContextChars int) map[string]any {
	mode := "shadow_guarded"
	if degraded {
		mode = "degraded_fallback"
	}
	return map[string]any{
		"version":                 "p212a.v1",
		"chat_session_id":         sid,
		"toggle_policy":           "guarded_shadow_support_only",
		"mode":                    mode,
		"injection_enabled":       injectionEnabled,
		"input_context_enabled":   inputContextEnabled,
		"max_injection_chars":     maxInjectionChars,
		"max_input_context_chars": maxInputContextChars,
		"broad_takeover":          false,
		"truth_store":             "maria_db",
		"retrieval_role":          "support_accelerator_only",
		"reason":                  "seq16_p212_runtime_toggle",
	}
}
