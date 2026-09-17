package httpapi

import (
	"fmt"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// --- SEQ-16-P164 / P165 / P167 / P168 contract helpers -------------------

// buildRetrievalRoleBoundary exposes the session/permanent role split for
// the prepare-turn surface. Permanent = storylines, character states, world
// rules. Session = active states, pending threads, chat logs. The split is
// derived from already-read Store data only.
func buildRetrievalRoleBoundary(sid string, storylines []store.Storyline, worldRules []store.WorldRule, charStates []store.CharacterState, activeStates []store.ActiveState, pendingThreads []store.PendingThread, chatLogs []store.ChatLog) map[string]any {
	permanentItems := []map[string]any{}
	for _, sl := range storylines {
		permanentItems = append(permanentItems, map[string]any{
			"role":       "permanent",
			"subrole":    "storyline",
			"id":         sl.ID,
			"name":       sl.Name,
			"last_turn":  sl.LastTurn,
			"suppressed": sl.Suppressed,
		})
	}
	for _, cs := range charStates {
		permanentItems = append(permanentItems, map[string]any{
			"role":           "permanent",
			"subrole":        "character_state",
			"id":             cs.ID,
			"character_name": cs.CharacterName,
			"turn_index":     cs.TurnIndex,
		})
	}
	for _, wr := range worldRules {
		permanentItems = append(permanentItems, map[string]any{
			"role":        "permanent",
			"subrole":     "world_rule",
			"id":          wr.ID,
			"scope":       wr.Scope,
			"scope_name":  wr.ScopeName,
			"category":    wr.Category,
			"key":         wr.Key,
			"source_turn": wr.SourceTurn,
			"suppressed":  wr.Suppressed,
		})
	}
	sessionItems := []map[string]any{}
	for _, as := range activeStates {
		sessionItems = append(sessionItems, map[string]any{
			"role":       "session",
			"subrole":    "active_state",
			"id":         as.ID,
			"state_type": as.StateType,
			"turn_index": as.TurnIndex,
		})
	}
	for _, pt := range pendingThreads {
		sessionItems = append(sessionItems, map[string]any{
			"role":         "session",
			"subrole":      "pending_thread",
			"id":           pt.ID,
			"thread_key":   pt.ThreadKey,
			"created_turn": pt.CreatedTurn,
		})
	}
	for _, cl := range chatLogs {
		sessionItems = append(sessionItems, map[string]any{
			"role":       "session",
			"subrole":    "chat_log",
			"id":         cl.ID,
			"turn_index": cl.TurnIndex,
		})
	}
	return map[string]any{
		"version":              "p164a.v1",
		"chat_session_id":      sid,
		"permanent_role":       "permanent",
		"session_role":         "session",
		"split_policy":         "session_permanent_role_boundary",
		"permanent_item_count": len(permanentItems),
		"session_item_count":   len(sessionItems),
		"permanent_items":      permanentItems,
		"session_items":        sessionItems,
		"boundary_active":      len(permanentItems) > 0 || len(sessionItems) > 0,
		"reason":               "seq16_p164_session_permanent_role_split",
	}
}

// buildRetrievalIndexIRSupportOnly exposes the IR-normalized retrieval unit
// truth floor for the prepare-turn surface. The floor is `support_only_ir`,
// meaning the retrieval unit is a support/retrieval accelerator and never
// the truth authority. Counts come from already-read Store data.
func buildRetrievalIndexIRSupportOnly(recallResult map[string]any, memories []store.Memory, evidence []store.DirectEvidence, kgTriples []store.KGTriple, chatLogs []store.ChatLog, resumePack *store.ResumePack) map[string]any {
	indexed := 0
	if counts, ok := recallResult["counts"].(map[string]any); ok {
		if v, ok := counts["documents_total"].(int); ok {
			indexed = v
		}
	}
	truthStore := "maria_db"
	retrievalAccelerator := "chromadb_compatible"
	irVersion := "p165a.v1"
	unitKind := "support_only_ir_normalized_retrieval_unit"
	return map[string]any{
		"version":               irVersion,
		"unit_kind":             unitKind,
		"support_only":          true,
		"truth_floor":           "support_only_ir",
		"truth_store":           truthStore,
		"retrieval_accelerator": retrievalAccelerator,
		"indexed_unit_count":    indexed,
		"source_counts": map[string]any{
			"memories":   len(memories),
			"evidence":   len(evidence),
			"kg_triples": len(kgTriples),
			"chat_logs":  len(chatLogs),
			"resume_pack": func() int {
				if resumePack == nil {
					return 0
				}
				return 1
			}(),
		},
		"truth_authority_role": "mariadb_canonical_only",
		"retrieval_role":       "support_accelerator_only",
		"reason":               "seq16_p165_support_only_ir_normalized_retrieval_unit_truth_floor",
	}
}

// buildRetrievalExtendAuthority exposes the retrieval-extend authority
// reorder for the prepare-turn surface. Authority order is fixed:
// permanent > session > support (ChromaDB) > fallback (chat_log).
func buildRetrievalExtendAuthority(retrievalRoleBoundary map[string]any) map[string]any {
	permanentCount := 0
	sessionCount := 0
	if v, ok := retrievalRoleBoundary["permanent_item_count"].(int); ok {
		permanentCount = v
	}
	if v, ok := retrievalRoleBoundary["session_item_count"].(int); ok {
		sessionCount = v
	}
	return map[string]any{
		"version":                  "p168a.v1",
		"authority_order":          []string{"permanent", "session", "support", "fallback"},
		"reorder_applied":          true,
		"reorder_policy":           "permanent_first_then_session_then_support_then_fallback",
		"permanent_authority":      "permanent",
		"session_authority":        "session",
		"support_authority":        "support",
		"fallback_authority":       "fallback",
		"permanent_item_count":     permanentCount,
		"session_item_count":       sessionCount,
		"authority_boundary_ready": permanentCount > 0 || sessionCount > 0,
		"reason":                   "seq16_p168_retrieval_extend_authority_reorder",
	}
}

// buildTemporalReadValidityFirst exposes the validity-first temporal read
// signal for the prepare-turn surface. Validity is recency-based and
// recency_event is the latest observed chat_log turn when present.
func buildTemporalReadValidityFirst(chatLogs []store.ChatLog, episodeSums []store.EpisodeSummary, recentChatCount int) map[string]any {
	latestChatTurn := 0
	latestChatRole := ""
	for _, cl := range chatLogs {
		if cl.TurnIndex > latestChatTurn {
			latestChatTurn = cl.TurnIndex
			latestChatRole = cl.Role
		}
	}
	latestEpisodeTo := 0
	for _, ep := range episodeSums {
		if ep.ToTurn > latestEpisodeTo {
			latestEpisodeTo = ep.ToTurn
		}
	}
	validityOrder := []string{"validity_first", "recency", "tier", "session_bond"}
	recencyEvent := any(nil)
	if latestChatTurn > 0 {
		recencyEvent = map[string]any{
			"kind":       "chat_log",
			"turn_index": latestChatTurn,
			"role":       latestChatRole,
		}
	} else if latestEpisodeTo > 0 {
		recencyEvent = map[string]any{
			"kind":    "episode_summary",
			"to_turn": latestEpisodeTo,
		}
	}
	return map[string]any{
		"version":               "p167a.v1",
		"validity_first":        true,
		"validity_order":        validityOrder,
		"recency_event":         recencyEvent,
		"recency_signal_source": "chat_log_then_episode",
		"recent_chat_count":     recentChatCount,
		"latest_chat_turn":      latestChatTurn,
		"latest_episode_to":     latestEpisodeTo,
		"reason":                "seq16_p167_validity_first_temporal_read",
	}
}

// buildSessionMemoryBoundary exposes the session-side memory boundary for
// the prepare-turn surface (SEQ-16-P172). It counts session-scoped items
// and declares the boundary_active flag.
func buildSessionMemoryBoundary(sid string, activeStates []store.ActiveState, pendingThreads []store.PendingThread, chatLogs []store.ChatLog, storylines []store.Storyline, worldRules []store.WorldRule, charStates []store.CharacterState) map[string]any {
	sessionItems := []map[string]any{}
	for _, as := range activeStates {
		sessionItems = append(sessionItems, map[string]any{
			"role":       "session",
			"subrole":    "active_state",
			"id":         as.ID,
			"state_type": as.StateType,
			"turn_index": as.TurnIndex,
		})
	}
	for _, pt := range pendingThreads {
		sessionItems = append(sessionItems, map[string]any{
			"role":         "session",
			"subrole":      "pending_thread",
			"id":           pt.ID,
			"thread_key":   pt.ThreadKey,
			"created_turn": pt.CreatedTurn,
		})
	}
	for _, cl := range chatLogs {
		sessionItems = append(sessionItems, map[string]any{
			"role":       "session",
			"subrole":    "chat_log",
			"id":         cl.ID,
			"turn_index": cl.TurnIndex,
		})
	}
	permanentItems := []map[string]any{}
	for _, sl := range storylines {
		permanentItems = append(permanentItems, map[string]any{
			"role":    "permanent",
			"subrole": "storyline",
			"id":      sl.ID,
			"name":    sl.Name,
		})
	}
	for _, wr := range worldRules {
		permanentItems = append(permanentItems, map[string]any{
			"role":    "permanent",
			"subrole": "world_rule",
			"id":      wr.ID,
			"key":     wr.Key,
		})
	}
	for _, cs := range charStates {
		permanentItems = append(permanentItems, map[string]any{
			"role":           "permanent",
			"subrole":        "character_state",
			"id":             cs.ID,
			"character_name": cs.CharacterName,
		})
	}
	return map[string]any{
		"version":              "p172a.v1",
		"chat_session_id":      sid,
		"session_role":         "session",
		"permanent_role":       "permanent",
		"split_policy":         "session_permanent_role_boundary",
		"session_item_count":   len(sessionItems),
		"permanent_item_count": len(permanentItems),
		"session_items":        sessionItems,
		"permanent_items":      permanentItems,
		"boundary_active":      len(sessionItems) > 0 || len(permanentItems) > 0,
		"reason":               "seq16_p172_session_memory_boundary",
	}
}

// buildBridgePromotionEntry exposes the bridge / promotion entry surface
// for the prepare-turn response (SEQ-16-P173). It lists pending threads
// and canonical layers that are candidates for promotion.
func buildBridgePromotionEntry(sid string, pendingThreads []store.PendingThread, canonicalLayers []store.CanonicalStateLayer) map[string]any {
	candidates := []map[string]any{}
	for _, pt := range pendingThreads {
		candidates = append(candidates, map[string]any{
			"kind":         "pending_thread",
			"id":           pt.ID,
			"thread_key":   pt.ThreadKey,
			"created_turn": pt.CreatedTurn,
			"status":       "awaiting_promotion",
		})
	}
	for _, cl := range canonicalLayers {
		candidates = append(candidates, map[string]any{
			"kind":       "canonical_layer",
			"id":         cl.ID,
			"layer_type": cl.LayerType,
			"turn_index": cl.TurnIndex,
			"status":     "awaiting_promotion",
		})
	}
	return map[string]any{
		"version":         "p173a.v1",
		"chat_session_id": sid,
		"promotion_ready": len(candidates) > 0,
		"candidate_count": len(candidates),
		"candidates":      candidates,
		"bridge_policy":   "pending_and_canonical_await_promotion",
		"reason":          "seq16_p173_bridge_promotion_entry",
	}
}

// buildSessionFirstPermanentFallbackReadRule exposes the session-first /
// permanent-fallback read rule surface (SEQ-16-P174). It declares that
// session items are read first, with permanent as fallback, and echoes
// counts from the session memory boundary.
func buildSessionFirstPermanentFallbackReadRule(sid string, sessionMemoryBoundary, retrievalRoleBoundary map[string]any) map[string]any {
	sessionCount := 0
	permanentCount := 0
	if v, ok := sessionMemoryBoundary["session_item_count"].(int); ok {
		sessionCount = v
	}
	if v, ok := sessionMemoryBoundary["permanent_item_count"].(int); ok {
		permanentCount = v
	}
	// Also accept float64 from JSON unmarshalling in tests.
	if v, ok := sessionMemoryBoundary["session_item_count"].(float64); ok {
		sessionCount = int(v)
	}
	if v, ok := sessionMemoryBoundary["permanent_item_count"].(float64); ok {
		permanentCount = int(v)
	}
	readOrder := []string{"session", "permanent"}
	return map[string]any{
		"version":              "p174a.v1",
		"chat_session_id":      sid,
		"read_policy":          "session_first_permanent_fallback",
		"read_order":           readOrder,
		"session_item_count":   sessionCount,
		"permanent_item_count": permanentCount,
		"fallback_triggered":   sessionCount == 0 && permanentCount > 0,
		"reason":               "seq16_p174_session_first_permanent_fallback_read_rule",
	}
}

// buildPromotionWaitVisibility exposes the promotion-wait visibility
// surface (SEQ-16-P175). It ensures current-turn important facts remain
// visible on the read surface even before canonical promotion, via a
// pending/support lane.
func buildPromotionWaitVisibility(sid string, pendingThreads []store.PendingThread, canonicalLayers []store.CanonicalStateLayer, chatLogs []store.ChatLog) map[string]any {
	pendingCount := len(pendingThreads)
	canonicalCount := len(canonicalLayers)
	latestChatTurn := 0
	for _, cl := range chatLogs {
		if cl.TurnIndex > latestChatTurn {
			latestChatTurn = cl.TurnIndex
		}
	}
	visibilityLanes := []map[string]any{}
	if pendingCount > 0 {
		visibilityLanes = append(visibilityLanes, map[string]any{
			"lane":  "pending_thread",
			"count": pendingCount,
			"role":  "support",
		})
	}
	if canonicalCount > 0 {
		visibilityLanes = append(visibilityLanes, map[string]any{
			"lane":  "canonical_layer",
			"count": canonicalCount,
			"role":  "support",
		})
	}
	if latestChatTurn > 0 {
		visibilityLanes = append(visibilityLanes, map[string]any{
			"lane":        "chat_log",
			"latest_turn": latestChatTurn,
			"role":        "session",
		})
	}
	return map[string]any{
		"version":          "p175a.v1",
		"chat_session_id":  sid,
		"visibility_ready": len(visibilityLanes) > 0,
		"visibility_lanes": visibilityLanes,
		"pending_count":    pendingCount,
		"canonical_count":  canonicalCount,
		"latest_chat_turn": latestChatTurn,
		"wait_policy":      "pending_support_lane_visible_before_promotion",
		"reason":           "seq16_p175_promotion_wait_visibility",
	}
}

// buildRetrievalUnitsIR exposes the normalized retrieval unit schema
// surface for the prepare-turn response (SEQ-16-P179). Each unit carries
// a stable schema identifier, source type, record id, and a support-only
// marker so it is never mistaken for the truth authority.
func buildRetrievalUnitsIR(sid string, memories []store.Memory, evidence []store.DirectEvidence, kgTriples []store.KGTriple, chatLogs []store.ChatLog, resumePack *store.ResumePack) map[string]any {
	units := []map[string]any{}
	for _, m := range memories {
		excerpt := strings.Join(strings.Fields(memorySearchText(m)), " ")
		units = append(units, map[string]any{
			"unit_schema":             "normalized_retrieval_unit_v1",
			"unit_id":                 fmt.Sprintf("mem_%d", m.ID),
			"source_type":             "memory",
			"source_record_id":        m.ID,
			"source_turn_start":       m.TurnIndex,
			"source_turn_end":         m.TurnIndex,
			"excerpt":                 excerpt,
			"summary_only_dependency": true,
			"source_depth":            "derived_summary",
			"truth_authority":         false,
		})
	}
	for _, e := range evidence {
		excerpt := strings.Join(strings.Fields(e.EvidenceText), " ")
		units = append(units, map[string]any{
			"unit_schema":             "normalized_retrieval_unit_v1",
			"unit_id":                 fmt.Sprintf("ev_%d", e.ID),
			"source_type":             "direct_evidence",
			"source_record_id":        e.ID,
			"source_turn_start":       e.SourceTurnStart,
			"source_turn_end":         e.SourceTurnEnd,
			"excerpt":                 excerpt,
			"summary_only_dependency": false,
			"source_depth":            "canonical_evidence",
			"truth_authority":         false,
			"canonical_source_role":   "direct_evidence_original",
		})
	}
	for _, k := range kgTriples {
		excerpt := strings.Join(strings.Fields(fmt.Sprintf("%s %s %s", k.Subject, k.Predicate, k.Object)), " ")
		units = append(units, map[string]any{
			"unit_schema":             "normalized_retrieval_unit_v1",
			"unit_id":                 fmt.Sprintf("kg_%d", k.ID),
			"source_type":             "kg_triple",
			"source_record_id":        k.ID,
			"source_turn_start":       k.SourceTurn,
			"source_turn_end":         k.SourceTurn,
			"excerpt":                 excerpt,
			"summary_only_dependency": false,
			"source_depth":            "derived_graph",
			"truth_authority":         false,
		})
	}
	for _, c := range chatLogs {
		excerpt := strings.Join(strings.Fields(c.Content), " ")
		units = append(units, map[string]any{
			"unit_schema":             "normalized_retrieval_unit_v1",
			"unit_id":                 fmt.Sprintf("cl_%d", c.ID),
			"source_type":             "chat_log",
			"source_record_id":        c.ID,
			"source_turn_start":       c.TurnIndex,
			"source_turn_end":         c.TurnIndex,
			"excerpt":                 excerpt,
			"summary_only_dependency": false,
			"source_depth":            "raw_turn",
			"truth_authority":         false,
		})
	}
	resumeCount := 0
	if resumePack != nil {
		resumeCount = 1
		start, end := resumePackTurnSpan(resumePack)
		units = append(units, map[string]any{
			"unit_schema":             "normalized_retrieval_unit_v1",
			"unit_id":                 "resume_pack",
			"source_type":             "resume_pack",
			"source_record_id":        "resume_pack",
			"source_turn_start":       start,
			"source_turn_end":         end,
			"excerpt":                 strings.Join(strings.Fields(resumePackExcerpt(resumePack)), " "),
			"summary_only_dependency": true,
			"source_depth":            "assembled_resume_pack",
			"truth_authority":         false,
		})
	}
	return map[string]any{
		"version":           "p179a.v1",
		"chat_session_id":   sid,
		"unit_schema":       "normalized_retrieval_unit_v1",
		"unit_count":        len(units),
		"units":             units,
		"support_only":      true,
		"truth_store":       "maria_db",
		"retrieval_role":    "support_accelerator_only",
		"resume_pack_units": resumeCount,
		"reason":            "seq16_p179_normalized_retrieval_unit_schema",
	}
}

// buildDirectEvidenceDualRepresentation exposes the dual-representation
// surface that lets callers distinguish the canonical direct-evidence original
// from its normalized retrieval-unit counterpart (SEQ-16-P180).
func buildDirectEvidenceDualRepresentation(evidence []store.DirectEvidence) map[string]any {
	canonical := []map[string]any{}
	normalized := []map[string]any{}
	for _, e := range evidence {
		canonical = append(canonical, map[string]any{
			"id":            e.ID,
			"turn_index":    e.TurnAnchor,
			"evidence_text": e.EvidenceText,
			"role":          "canonical_evidence",
		})
		normalized = append(normalized, map[string]any{
			"id":               e.ID,
			"unit_id":          fmt.Sprintf("ev_%d", e.ID),
			"source_record_id": e.ID,
			"turn_index":       e.TurnAnchor,
			"excerpt":          strings.Join(strings.Fields(e.EvidenceText), " "),
			"role":             "normalized_retrieval_unit",
			"truth_authority":  false,
		})
	}
	return map[string]any{
		"version":           "p180a.v1",
		"dual_policy":       "canonical_original_plus_normalized_unit",
		"canonical_count":   len(canonical),
		"normalized_count":  len(normalized),
		"canonical_items":   canonical,
		"normalized_items":  normalized,
		"identifiable_both": len(canonical) == len(normalized),
		"reason":            "seq16_p180_direct_evidence_vs_normalized_unit_dual_representation",
	}
}

// buildSourceTaggedRetrievalUnitSurface exposes the source-tagged retrieval
// unit surface (SEQ-16-P181). It lists every normalized unit with its
// source tag so the consumer knows which lane it came from.
func buildSourceTaggedRetrievalUnitSurface(memories []store.Memory, evidence []store.DirectEvidence, kgTriples []store.KGTriple, chatLogs []store.ChatLog, resumePack *store.ResumePack) map[string]any {
	tagged := []map[string]any{}
	for _, m := range memories {
		tagged = append(tagged, map[string]any{
			"unit_id":     fmt.Sprintf("mem_%d", m.ID),
			"source_tag":  "primary_signal_memory",
			"source_type": "memory",
		})
	}
	for _, e := range evidence {
		tagged = append(tagged, map[string]any{
			"unit_id":     fmt.Sprintf("ev_%d", e.ID),
			"source_tag":  "support_signal_evidence",
			"source_type": "direct_evidence",
		})
	}
	for _, k := range kgTriples {
		tagged = append(tagged, map[string]any{
			"unit_id":     fmt.Sprintf("kg_%d", k.ID),
			"source_tag":  "support_signal_kg",
			"source_type": "kg_triple",
		})
	}
	for _, c := range chatLogs {
		tagged = append(tagged, map[string]any{
			"unit_id":     fmt.Sprintf("cl_%d", c.ID),
			"source_tag":  "fallback_signal_chat_log",
			"source_type": "chat_log",
		})
	}
	if resumePack != nil {
		tagged = append(tagged, map[string]any{
			"unit_id":     "resume_pack",
			"source_tag":  "support_signal_resume_pack",
			"source_type": "resume_pack",
		})
	}
	return map[string]any{
		"version":        "p181a.v1",
		"tagged_count":   len(tagged),
		"tagged_units":   tagged,
		"tagging_policy": "source_derived_from_store_type",
		"reason":         "seq16_p181_source_tagged_retrieval_unit_surface",
	}
}

// buildRawTurnSpanMetadata exposes the raw-turn span, excerpt pointer,
// and source-depth metadata surface (SEQ-16-P182). It marks whether each
// unit still depends only on a summary, or has a direct raw-turn / evidence
// pointer available.
func buildRawTurnSpanMetadata(chatLogs []store.ChatLog, episodeSums []store.EpisodeSummary, memories []store.Memory, evidence []store.DirectEvidence, resumePack *store.ResumePack) map[string]any {
	spans := []map[string]any{}
	latestChatTurn := 0
	for _, cl := range chatLogs {
		if cl.TurnIndex > latestChatTurn {
			latestChatTurn = cl.TurnIndex
		}
	}
	for _, cl := range chatLogs {
		spans = append(spans, map[string]any{
			"unit_id":            fmt.Sprintf("cl_%d", cl.ID),
			"source_type":        "chat_log",
			"turn_span":          map[string]any{"start": cl.TurnIndex, "end": cl.TurnIndex},
			"excerpt_pointer":    strings.Join(strings.Fields(cl.Content), " "),
			"source_depth":       "raw_turn",
			"summary_only":       false,
			"has_direct_pointer": true,
		})
	}
	for _, e := range evidence {
		spans = append(spans, map[string]any{
			"unit_id":            fmt.Sprintf("ev_%d", e.ID),
			"source_type":        "direct_evidence",
			"turn_span":          map[string]any{"start": e.SourceTurnStart, "end": e.SourceTurnEnd},
			"excerpt_pointer":    strings.Join(strings.Fields(e.EvidenceText), " "),
			"source_depth":       "canonical_evidence",
			"summary_only":       false,
			"has_direct_pointer": true,
		})
	}
	for _, m := range memories {
		text := memorySearchText(m)
		spans = append(spans, map[string]any{
			"unit_id":            fmt.Sprintf("mem_%d", m.ID),
			"source_type":        "memory",
			"turn_span":          map[string]any{"start": m.TurnIndex, "end": m.TurnIndex},
			"excerpt_pointer":    strings.Join(strings.Fields(text), " "),
			"source_depth":       "derived_summary",
			"summary_only":       true,
			"has_direct_pointer": false,
		})
	}
	if resumePack != nil {
		start, end := resumePackTurnSpan(resumePack)
		spans = append(spans, map[string]any{
			"unit_id":            "resume_pack",
			"source_type":        "resume_pack",
			"turn_span":          map[string]any{"start": start, "end": end},
			"excerpt_pointer":    strings.Join(strings.Fields(resumePackExcerpt(resumePack)), " "),
			"source_depth":       "assembled_resume_pack",
			"summary_only":       true,
			"has_direct_pointer": false,
		})
	}
	latestEpisodeTo := 0
	for _, ep := range episodeSums {
		if ep.ToTurn > latestEpisodeTo {
			latestEpisodeTo = ep.ToTurn
		}
	}
	return map[string]any{
		"version":            "p182a.v1",
		"span_count":         len(spans),
		"spans":              spans,
		"latest_chat_turn":   latestChatTurn,
		"latest_episode_to":  latestEpisodeTo,
		"pointer_policy":     "excerpt_plus_turn_span",
		"summary_only_guard": true,
		"reason":             "seq16_p182_raw_turn_span_excerpt_pointer_source_depth_metadata",
	}
}

func resumePackTurnSpan(pack *store.ResumePack) (int, int) {
	if pack == nil {
		return 0, 0
	}
	if pack.Chapter != nil {
		return pack.Chapter.FromTurn, pack.Chapter.ToTurn
	}
	if pack.Arc != nil {
		return pack.Arc.FromTurn, pack.Arc.ToTurn
	}
	if pack.Saga != nil {
		return pack.Saga.FromTurn, pack.Saga.ToTurn
	}
	return 0, 0
}

func resumePackExcerpt(pack *store.ResumePack) string {
	if pack == nil {
		return ""
	}
	if strings.TrimSpace(pack.AssembledText) != "" {
		return pack.AssembledText
	}
	if pack.Chapter != nil {
		if strings.TrimSpace(pack.Chapter.ResumeText) != "" {
			return pack.Chapter.ResumeText
		}
		return pack.Chapter.SummaryText
	}
	if pack.Arc != nil {
		return pack.Arc.ArcResumeText
	}
	if pack.Saga != nil {
		return pack.Saga.ResumePackText
	}
	return pack.AssemblyNote
}

// buildSignalMixContract exposes the semantic / keyword / entity / graph /
// time-range signal mix surface (SEQ-16-P186). It is inspectable and
// support-only: no signal lane claims truth authority.
func buildSignalMixContract(sid string, memories []store.Memory, evidence []store.DirectEvidence, kgTriples []store.KGTriple, chatLogs []store.ChatLog, episodeSums []store.EpisodeSummary) map[string]any {
	semanticCount := len(memories)
	keywordCount := len(chatLogs)
	entityCount := 0
	for _, k := range kgTriples {
		if k.Subject != "" || k.Object != "" {
			entityCount++
		}
	}
	graphCount := len(kgTriples)
	timeRangeCount := len(episodeSums)
	signals := []map[string]any{
		{"signal": "semantic", "source": "memory_embedding", "count": semanticCount, "role": "support_accelerator", "truth_authority": false},
		{"signal": "keyword", "source": "chat_log_verbatim", "count": keywordCount, "role": "fallback_support", "truth_authority": false},
		{"signal": "entity", "source": "kg_triple_subject_object", "count": entityCount, "role": "support_accelerator", "truth_authority": false},
		{"signal": "graph", "source": "kg_triple_predicate_link", "count": graphCount, "role": "support_accelerator", "truth_authority": false},
		{"signal": "time_range", "source": "episode_summary_span", "count": timeRangeCount, "role": "support_accelerator", "truth_authority": false},
	}
	return map[string]any{
		"version":         "p186a.v1",
		"chat_session_id": sid,
		"mix_policy":      "semantic_keyword_entity_graph_time_range",
		"signals":         signals,
		"signal_count":    len(signals),
		"truth_store":     "maria_db",
		"retrieval_role":  "support_accelerator_only",
		"reason":          "seq16_p186_semantic_keyword_entity_graph_time_range_signal_mix",
	}
}

// buildQueryClassRouting exposes the query-class retrieval depth / signal
// routing surface (SEQ-16-P187). Each class maps to a depth policy and a
// primary signal lane; all lanes remain support-only.
func buildQueryClassRouting(sid string, memories []store.Memory, evidence []store.DirectEvidence, kgTriples []store.KGTriple, chatLogs []store.ChatLog, episodeSums []store.EpisodeSummary) map[string]any {
	classes := []map[string]any{
		{
			"query_class":      "factual_lookup",
			"depth_policy":     "canonical_evidence_first",
			"primary_signal":   "direct_evidence",
			"fallback_signals": []string{"memory", "kg_triple"},
			"truth_authority":  false,
			"routing_reason":   "evidence_is_canonical_truth",
		},
		{
			"query_class":      "relationship_state",
			"depth_policy":     "graph_then_memory",
			"primary_signal":   "kg_triple",
			"fallback_signals": []string{"memory", "episode_summary"},
			"truth_authority":  false,
			"routing_reason":   "kg_links_are_support_only",
		},
		{
			"query_class":      "narrative_progression",
			"depth_policy":     "episode_then_chat_log",
			"primary_signal":   "episode_summary",
			"fallback_signals": []string{"memory", "chat_log"},
			"truth_authority":  false,
			"routing_reason":   "episodes_are_derived_support",
		},
		{
			"query_class":      "recent_context",
			"depth_policy":     "raw_turn_first",
			"primary_signal":   "chat_log",
			"fallback_signals": []string{"memory"},
			"truth_authority":  false,
			"routing_reason":   "chat_logs_are_fallback_support",
		},
		{
			"query_class":      "semantic_recall",
			"depth_policy":     "dense_summary_then_evidence",
			"primary_signal":   "memory",
			"fallback_signals": []string{"episode_summary", "chat_log"},
			"truth_authority":  false,
			"routing_reason":   "memories_are_support_only",
		},
	}
	return map[string]any{
		"version":         "p187a.v1",
		"chat_session_id": sid,
		"routing_policy":  "query_class_depth_signal_routing",
		"classes":         classes,
		"class_count":     len(classes),
		"truth_store":     "maria_db",
		"retrieval_role":  "support_accelerator_only",
		"reason":          "seq16_p187_query_class_retrieval_depth_signal_routing",
	}
}

// buildRetrievalResultInspection exposes the retrieval result inspection
// surface (SEQ-16-P188). It lists every retrieved lane with its count,
// bound, and authority status so the consumer can audit what was
// considered without trusting it blindly.
func buildRetrievalResultInspection(sid string, memories []store.Memory, evidence []store.DirectEvidence, kgTriples []store.KGTriple, chatLogs []store.ChatLog, episodeSums []store.EpisodeSummary, recallLimit int) map[string]any {
	recallLimit = prepareTurnRecallLimit(recallLimit)
	lanes := []map[string]any{
		{
			"lane":         "memory",
			"total":        len(memories),
			"bound":        minInt(len(memories), recallLimit),
			"authority":    false,
			"role":         "support_accelerator",
			"source_depth": "derived_summary",
		},
		{
			"lane":         "direct_evidence",
			"total":        len(evidence),
			"bound":        minInt(len(evidence), recallLimit),
			"authority":    true,
			"role":         "canonical_truth",
			"source_depth": "canonical_evidence",
		},
		{
			"lane":         "kg_triple",
			"total":        len(kgTriples),
			"bound":        minInt(len(kgTriples), recallLimit),
			"authority":    false,
			"role":         "support_accelerator",
			"source_depth": "derived_graph",
		},
		{
			"lane":         "chat_log",
			"total":        len(chatLogs),
			"bound":        minInt(len(chatLogs), recallLimit),
			"authority":    false,
			"role":         "fallback_support",
			"source_depth": "raw_turn",
		},
		{
			"lane":         "episode_summary",
			"total":        len(episodeSums),
			"bound":        minInt(len(episodeSums), recallLimit),
			"authority":    false,
			"role":         "support_accelerator",
			"source_depth": "derived_summary",
		},
	}
	return map[string]any{
		"version":           "p188a.v1",
		"chat_session_id":   sid,
		"inspection_policy": "lane_count_bound_authority",
		"lanes":             lanes,
		"lane_count":        len(lanes),
		"truth_store":       "maria_db",
		"retrieval_role":    "support_accelerator_only",
		"reason":            "seq16_p188_retrieval_result_inspection_surface",
	}
}

// buildSparseTailRecall exposes the sparse-tail recall route surface
// (SEQ-16-P189). It marks the dense-summary route and the raw/evidence
// support route so callers know when a sparse tail is being recalled
// through non-summary lanes.
func buildSparseTailRecall(sid string, memories []store.Memory, evidence []store.DirectEvidence, kgTriples []store.KGTriple, chatLogs []store.ChatLog, episodeSums []store.EpisodeSummary) map[string]any {
	denseSummaryCount := len(memories) + len(episodeSums)
	rawEvidenceCount := len(evidence) + len(chatLogs)
	graphCount := len(kgTriples)
	routes := []map[string]any{
		{
			"route_name":         "dense_summary",
			"sources":            []string{"memory", "episode_summary"},
			"count":              denseSummaryCount,
			"role":               "primary_support",
			"summary_only":       true,
			"has_direct_pointer": false,
		},
		{
			"route_name":         "raw_evidence_support",
			"sources":            []string{"direct_evidence", "chat_log"},
			"count":              rawEvidenceCount,
			"role":               "fallback_support",
			"summary_only":       false,
			"has_direct_pointer": true,
		},
		{
			"route_name":         "graph_link_support",
			"sources":            []string{"kg_triple"},
			"count":              graphCount,
			"role":               "support_accelerator",
			"summary_only":       false,
			"has_direct_pointer": true,
		},
	}
	return map[string]any{
		"version":         "p189a.v1",
		"chat_session_id": sid,
		"recall_policy":   "dense_summary_plus_raw_evidence_support",
		"routes":          routes,
		"route_count":     len(routes),
		"truth_store":     "maria_db",
		"retrieval_role":  "support_accelerator_only",
		"reason":          "seq16_p189_sparse_tail_recall_dense_summary_raw_evidence_support_route",
	}
}

// buildValidityWindowReading exposes the validity-window / invalidation
// reading surface (SEQ-16-P193). It marks the current validity window
// (latest chat turn to latest episode) and flags whether any evidence
// has been invalidated by a newer turn.
