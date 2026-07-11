package httpapi

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// Explorer read: Store-backed

func (s *Server) handleExplorerChatLogs(w http.ResponseWriter, r *http.Request) {
	sid := r.URL.Query().Get("chat_session_id")
	fromTurn, _ := strconv.Atoi(r.URL.Query().Get("from_turn"))
	toTurn, _ := strconv.Atoi(r.URL.Query().Get("to_turn"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}

	var logs []store.ChatLog
	if s.Store != nil {
		result, err := s.Store.ListChatLogs(r.Context(), sid, fromTurn, toTurn)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		if err == nil {
			logs = result
		}
	}
	sort.SliceStable(logs, func(i, j int) bool {
		if logs[i].TurnIndex == logs[j].TurnIndex {
			return logs[i].ID > logs[j].ID
		}
		return logs[i].TurnIndex > logs[j].TurnIndex
	})

	items := []any{}
	for i, l := range logs {
		if i < offset {
			continue
		}
		if len(items) >= limit {
			break
		}
		preview := pythonTextPreview(l.Content, 120)
		items = append(items, map[string]any{
			"id":              l.ID,
			"chat_session_id": l.ChatSessionID,
			"turn_index":      l.TurnIndex,
			"role":            l.Role,
			"content":         l.Content,
			"preview":         preview,
			"created_at":      formatKSTTime(l.CreatedAt),
		})
	}

	total := len(logs)
	hasMore := offset+len(items) < total

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"items":    items,
		"total":    total,
		"has_more": hasMore,
		"limit":    limit,
		"offset":   offset,
	})
}

func pythonTextPreview(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "..."
}

func (s *Server) handleExplorerMemories(w http.ResponseWriter, r *http.Request) {
	sid := r.URL.Query().Get("chat_session_id")
	fromTurn, _ := strconv.Atoi(r.URL.Query().Get("from_turn"))
	toTurn, _ := strconv.Atoi(r.URL.Query().Get("to_turn"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}

	items := []any{}
	total := 0

	if s.Store != nil {
		memories, err := s.Store.ListMemories(r.Context(), sid, fromTurn, toTurn)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		if err == nil {
			sort.SliceStable(memories, func(i, j int) bool {
				if memories[i].TurnIndex != memories[j].TurnIndex {
					return memories[i].TurnIndex > memories[j].TurnIndex
				}
				if memories[i].CreatedAt.Equal(memories[j].CreatedAt) {
					return memories[i].ID > memories[j].ID
				}
				return memories[i].CreatedAt.After(memories[j].CreatedAt)
			})
			total = len(memories)
			start := offset
			if start > len(memories) {
				start = len(memories)
			}
			end := start + limit
			if end > len(memories) {
				end = len(memories)
			}
			for _, m := range memories[start:end] {
				items = append(items, map[string]any{
					"id":                     m.ID,
					"chat_session_id":        m.ChatSessionID,
					"source_turn":            m.TurnIndex,
					"summary_json":           m.SummaryJSON,
					"summary_preview":        memorySummaryPreview(m.SummaryJSON),
					"importance":             m.Importance,
					"emotional_intensity":    nullableFloatZero(m.EmotionalIntensity),
					"narrative_significance": nullableFloatZero(m.NarrativeSignificance),
					"emotional_boost":        nullableFloatZero(m.EmotionalBoost),
					"evidence":               m.Evidence,
					"archive_wing":           m.PlaceWing,
					"archive_room":           m.PlaceRoom,
					"embedding_model":        m.EmbeddingModel,
					"created_at":             formatKSTTime(m.CreatedAt),
				})
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"items":    items,
		"total":    total,
		"has_more": offset+len(items) < total,
		"limit":    limit,
		"offset":   offset,
	})
}

func (s *Server) handleExplorerDirectEvidence(w http.ResponseWriter, r *http.Request) {
	sid := r.URL.Query().Get("chat_session_id")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	limit := 30
	offset := 0
	if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
		limit = v
	}
	if v, err := strconv.Atoi(offsetStr); err == nil && v >= 0 {
		offset = v
	}

	items := []any{}
	total := 0
	latestTurnIndex := 0
	stateCounts := map[string]int{}
	auditRows := []store.AuditLog{}

	if s.Store != nil {
		evidence, err := s.Store.ListEvidence(r.Context(), sid)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		if err == nil {
			sort.SliceStable(evidence, func(i, j int) bool {
				if evidence[i].CreatedAt.Equal(evidence[j].CreatedAt) {
					return evidence[i].ID > evidence[j].ID
				}
				return evidence[i].CreatedAt.After(evidence[j].CreatedAt)
			})
			total = len(evidence)

			if sid != "" {
				logs, logErr := s.Store.ListChatLogs(r.Context(), sid, 0, 0)
				if logErr == nil {
					for _, l := range logs {
						if l.TurnIndex > latestTurnIndex {
							latestTurnIndex = l.TurnIndex
						}
					}
				}
			}
			start := offset
			if start > len(evidence) {
				start = len(evidence)
			}
			end := start + limit
			if end > len(evidence) {
				end = len(evidence)
			}
			page := evidence[start:end]
			for _, e := range page {
				bucket := directEvidenceArchiveBucket(
					normalizeDirectEvidenceArchiveState(e.ArchiveState),
					normalizeDirectEvidenceCaptureVerification(e.CaptureVerification),
					e.RepairNeeded,
				)
				stateCounts[bucket]++
				items = append(items, directEvidenceExplorerItem(e, latestTurnIndex))
			}
		}

		if sid != "" {
			audits, auditErr := s.Store.ListAuditLogs(r.Context(), sid, "", 1000)
			if auditErr != nil && !errors.Is(auditErr, store.ErrNotEnabled) {
				writeInternalError(w, auditErr.Error())
				return
			}
			if auditErr == nil {
				auditRows = audits
			}
		}
	}

	stateCountsAny := directEvidenceStateCounts(stateCounts)
	var latestTurn any
	if latestTurnIndex > 0 {
		latestTurn = latestTurnIndex
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":            "ok",
		"items":             items,
		"total":             total,
		"has_more":          total > offset+len(items),
		"limit":             limit,
		"offset":            offset,
		"latest_turn_index": latestTurn,
		"state_contract":    directEvidenceStateContract(),
		"state_counts":      stateCountsAny,
		"cost_measurement":  directEvidenceCostMeasurement(stateCounts, auditRows),
	})
}

func directEvidenceStateContract() map[string]any {
	return map[string]any{
		"archive_states":                     []string{"pending_capture", "verified_direct", "previous_archive", "repair_queue"},
		"capture_verifications":              []string{"pending", "verified", "rejected", "needs_review"},
		"committed_gates":                    []string{"finalize", "recovery", "manual"},
		"conflict_resolution_policy_version": "ea1h.v1",
		"conflict_confidence_policy_version": "ea1i.v1",
		"conflict_classes":                   []string{"state_transition", "hard_contradiction", "parallel_context", "low_confidence_noise"},
		"conflict_routes":                    []string{"superseded", "tombstone", "hold", "manual_review"},
		"conflict_confidence_thresholds": map[string]any{
			"auto_promote_min":                  0.82,
			"hold_below":                        0.55,
			"high_impact_manual_review_below":   0.9,
			"user_confirmation_candidate_below": 0.9,
		},
		"conflict_high_impact_field_classes":            []string{"identity", "relationship", "trust", "world_rule", "canonical_fact"},
		"cost_measurement_policy_version":               "lc1a.v1",
		"deleted_turn_tombstone_retention_window_turns": 240,
		"retention_importance_tiers":                    []string{"critical", "high", "medium", "low"},
		"retention_policy_version":                      "ea1l.v1",
		"retention_windows_turns": map[string]any{
			"direct_evidence":  map[string]any{"critical": 720, "high": 480, "medium": 320, "low": 180},
			"previous_archive": map[string]any{"critical": 540, "high": 360, "medium": 240, "low": 160},
			"tombstone":        map[string]any{"critical": 480, "high": 320, "medium": 240, "low": 240},
		},
	}
}

func directEvidenceStateCounts(counts map[string]int) map[string]any {
	out := map[string]any{
		"pending_capture":  0,
		"verified_direct":  0,
		"previous_archive": 0,
		"repair_queue":     0,
	}
	for key, value := range counts {
		if _, ok := out[key]; ok {
			out[key] = value
		}
	}
	return out
}

func directEvidenceCostMeasurement(stateCounts map[string]int, auditRows []store.AuditLog) map[string]any {
	measurement := map[string]any{
		"policy_version":    "lc1a.v1",
		"audit_window_size": 200,
		"direct_evidence_write": map[string]any{
			"sample_count":    0,
			"avg_latency_ms":  0.0,
			"p95_latency_ms":  0.0,
			"last_latency_ms": 0.0,
			"avg_inserted":    0.0,
			"avg_skipped":     0.0,
			"avg_write_chars": 0.0,
		},
		"repair_queue": map[string]any{
			"queue_count":                stateCounts["repair_queue"],
			"review_sample_count":        0,
			"revalidate_sample_count":    0,
			"avg_review_latency_ms":      0.0,
			"avg_revalidate_latency_ms":  0.0,
			"last_revalidate_latency_ms": 0.0,
		},
	}

	if len(auditRows) == 0 {
		return measurement
	}

	sort.SliceStable(auditRows, func(i, j int) bool {
		return auditRows[i].ID > auditRows[j].ID
	})
	if len(auditRows) > 200 {
		auditRows = auditRows[:200]
	}

	writeLatencies := []float64{}
	writeInserted := []float64{}
	writeSkipped := []float64{}
	writeChars := []float64{}
	reviewLatencies := []float64{}
	revalidateLatencies := []float64{}

	for _, row := range auditRows {
		switch row.EventType {
		case "critic_ingest_trace":
			details := parseJSONMap(row.DetailsJSON)
			if strings.TrimSpace(stringFromAny(details["surface"])) != "direct_evidence" {
				continue
			}
			trace, _ := details["trace"].(map[string]any)
			latency := floatFromAny(trace["elapsed_ms"])
			writeLatencies = append(writeLatencies, latency)
			writeInserted = append(writeInserted, floatFromAny(trace["inserted"]))
			writeSkipped = append(writeSkipped, floatFromAny(trace["skipped"]))
			writeChars = append(writeChars, floatFromAny(trace["write_chars"]))
		case "direct_evidence_review":
			details := parseJSONMap(row.DetailsJSON)
			cost, _ := details["cost_measurement"].(map[string]any)
			reviewLatencies = append(reviewLatencies, floatFromAny(cost["latency_ms"]))
		case "direct_evidence_revalidate":
			details := parseJSONMap(row.DetailsJSON)
			cost, _ := details["cost_measurement"].(map[string]any)
			revalidateLatencies = append(revalidateLatencies, floatFromAny(cost["latency_ms"]))
		}
	}

	if len(writeLatencies) > 0 {
		measurement["direct_evidence_write"] = map[string]any{
			"sample_count":    len(writeLatencies),
			"avg_latency_ms":  safeMeanFloat(writeLatencies),
			"p95_latency_ms":  safeP95Float(writeLatencies),
			"last_latency_ms": safeRoundFloat(writeLatencies[0]),
			"avg_inserted":    safeMeanFloat(writeInserted),
			"avg_skipped":     safeMeanFloat(writeSkipped),
			"avg_write_chars": safeMeanFloat(writeChars),
		}
	}

	repairQueue, _ := measurement["repair_queue"].(map[string]any)
	if len(reviewLatencies) > 0 {
		repairQueue["review_sample_count"] = len(reviewLatencies)
		repairQueue["avg_review_latency_ms"] = safeMeanFloat(reviewLatencies)
	}
	if len(revalidateLatencies) > 0 {
		repairQueue["revalidate_sample_count"] = len(revalidateLatencies)
		repairQueue["avg_revalidate_latency_ms"] = safeMeanFloat(revalidateLatencies)
		repairQueue["last_revalidate_latency_ms"] = safeRoundFloat(revalidateLatencies[0])
	}
	measurement["repair_queue"] = repairQueue
	return measurement
}

func stringFromAny(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func floatFromAny(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case int32:
		return float64(n)
	case json.Number:
		out, err := n.Float64()
		if err == nil {
			return out
		}
	case string:
		out, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		if err == nil {
			return out
		}
	}
	return 0
}

func safeRoundFloat(v float64) float64 {
	return math.Round(v*1000) / 1000
}

func safeMeanFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, v := range values {
		total += v
	}
	return safeRoundFloat(total / float64(len(values)))
}

func safeP95Float(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sortedValues := append([]float64(nil), values...)
	sort.Float64s(sortedValues)
	idx := int(float64(len(sortedValues)-1) * 0.95)
	return safeRoundFloat(sortedValues[idx])
}

func directEvidenceExplorerItem(e store.DirectEvidence, latestTurnIndex int) map[string]any {
	sourceIDs := parseJSONList(e.SourceMessageIDsJSON)
	lineage := parseJSONMap(e.LineageJSON)
	normalizedArchiveState := normalizeDirectEvidenceArchiveState(e.ArchiveState)
	normalizedCaptureVerification := normalizeDirectEvidenceCaptureVerification(e.CaptureVerification)
	normalizedCommittedGate := resolveDirectEvidenceCommittedGate(normalizedCaptureVerification, e.RepairNeeded, e.CommittedGate)
	retentionTier := directEvidenceRetentionTier(normalizedArchiveState, normalizedCaptureVerification, e.RepairNeeded, e.CommittedGate, e.Tombstoned, lineage)
	retentionTTL := directEvidenceRetentionTTL(normalizedArchiveState, retentionTier, e.Tombstoned)
	retentionExpired := directEvidenceRetentionExpired(e.SourceTurnEnd, latestTurnIndex, retentionTTL)
	tombstoneRetained := directEvidenceTombstoneRetained(e.Tombstoned, e.SourceTurnEnd, latestTurnIndex)
	conflictResolution := directEvidenceConflictResolution(e, lineage, normalizedArchiveState, normalizedCaptureVerification, normalizedCommittedGate, retentionTier)
	return map[string]any{
		"id":                                 e.ID,
		"chat_session_id":                    e.ChatSessionID,
		"evidence_kind":                      e.EvidenceKind,
		"evidence_text":                      e.EvidenceText,
		"evidence_preview":                   truncateForPreview(e.EvidenceText, 120),
		"source_turn_start":                  e.SourceTurnStart,
		"source_turn_end":                    e.SourceTurnEnd,
		"turn_anchor":                        nullablePositiveInt(e.TurnAnchor),
		"source_message_ids_json":            nullableString(e.SourceMessageIDsJSON),
		"source_message_ids":                 sourceIDs,
		"source_hash":                        nullableString(e.SourceHash),
		"archive_state":                      e.ArchiveState,
		"normalized_archive_state":           normalizedArchiveState,
		"archive_bucket":                     directEvidenceArchiveBucket(normalizedArchiveState, normalizedCaptureVerification, e.RepairNeeded),
		"capture_stage":                      e.CaptureStage,
		"capture_verification":               e.CaptureVerification,
		"normalized_capture_verification":    normalizedCaptureVerification,
		"committed_gate":                     nullableString(e.CommittedGate),
		"normalized_committed_gate":          normalizedCommittedGate,
		"lineage_json":                       nullableString(e.LineageJSON),
		"lineage":                            lineage,
		"repair_needed":                      e.RepairNeeded,
		"tombstoned":                         e.Tombstoned,
		"superseded_by_id":                   nullableInt64(e.SupersededByID),
		"excluded_from_current_truth":        e.Tombstoned || e.SupersededByID > 0,
		"tombstone_retained_in_window":       tombstoneRetained,
		"tombstone_retention_expired":        e.Tombstoned && !tombstoneRetained,
		"retention_policy_version":           "ea1l.v1",
		"retention_importance_tier":          retentionTier,
		"retention_ttl_turns":                retentionTTL,
		"retention_expired":                  retentionExpired,
		"retention_blocked_from_consumption": retentionExpired,
		"conflict_resolution_policy_version": "ea1h.v1",
		"conflict_confidence_policy_version": "ea1i.v1",
		"conflict_resolution":                conflictResolution,
		"created_at":                         formatKSTTime(e.CreatedAt),
	}
}

func parseJSONList(raw string) []any {
	text := strings.TrimSpace(raw)
	if text == "" {
		return []any{}
	}
	var out []any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		return []any{}
	}
	if out == nil {
		return []any{}
	}
	return out
}

func parseJSONMap(raw string) map[string]any {
	text := strings.TrimSpace(raw)
	if text == "" {
		return map[string]any{}
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil || out == nil {
		return map[string]any{}
	}
	return out
}

func normalizeDirectEvidenceArchiveState(value string) string {
	v := strings.TrimSpace(strings.ToLower(value))
	switch v {
	case "verified_direct", "previous_archive", "repair_queue", "pending_capture":
		return v
	case "committed", "direct_evidence", "verified":
		return "verified_direct"
	case "":
		return "pending_capture"
	default:
		return v
	}
}

func normalizeDirectEvidenceCaptureVerification(value string) string {
	v := strings.TrimSpace(strings.ToLower(value))
	switch v {
	case "verified", "rejected", "needs_review", "pending":
		return v
	case "":
		return "pending"
	default:
		return v
	}
}

func resolveDirectEvidenceCommittedGate(captureVerification string, repairNeeded bool, committedGate string) any {
	if strings.TrimSpace(committedGate) != "" {
		return strings.TrimSpace(committedGate)
	}
	if repairNeeded {
		return "recovery"
	}
	if captureVerification == "verified" {
		return "finalize"
	}
	return nil
}

func directEvidenceArchiveBucket(archiveState, captureVerification string, repairNeeded bool) string {
	if repairNeeded || captureVerification == "rejected" || captureVerification == "needs_review" {
		return "repair_queue"
	}
	return archiveState
}

func directEvidenceRetentionTier(archiveState, captureVerification string, repairNeeded bool, committedGate string, tombstoned bool, lineage map[string]any) string {
	for _, key := range []string{"importance_tier", "retention_tier", "importance"} {
		if tier, ok := lineage[key].(string); ok {
			normalized := strings.TrimSpace(strings.ToLower(tier))
			if normalized == "critical" || normalized == "high" || normalized == "medium" || normalized == "low" {
				return normalized
			}
		}
	}
	for _, marker := range []string{"force_retain", "high_impact", "user_confirmation_candidate", "manual_review_required"} {
		if val, ok := lineage[marker].(bool); ok && val {
			return "critical"
		}
	}
	normalizedGate := resolveDirectEvidenceCommittedGate(captureVerification, repairNeeded, committedGate)
	if gateStr, ok := normalizedGate.(string); ok && gateStr == "manual" {
		return "high"
	}
	if tombstoned {
		return "low"
	}
	if archiveState == "verified_direct" && captureVerification == "verified" && !repairNeeded {
		if gateStr, ok := normalizedGate.(string); ok && gateStr == "manual" {
			return "high"
		}
		return "medium"
	}
	if archiveState == "previous_archive" {
		return "medium"
	}
	return "low"
}

func directEvidenceRetentionTTL(archiveState, tier string, tombstoned bool) int {
	if tombstoned {
		return 240
	}
	switch tier {
	case "critical":
		if archiveState == "previous_archive" {
			return 540
		}
		return 720
	case "high":
		if archiveState == "previous_archive" {
			return 360
		}
		return 480
	case "low":
		if archiveState == "previous_archive" {
			return 160
		}
		return 180
	default:
		if archiveState == "previous_archive" {
			return 240
		}
		return 320
	}
}

func directEvidenceConflictResolution(e store.DirectEvidence, lineage map[string]any, archiveState, captureVerification string, committedGate any, retentionTier string) map[string]any {
	confidence := directEvidenceConflictConfidence(lineage, captureVerification, e.RepairNeeded)
	fieldClass := directEvidenceConflictFieldClass(lineage)
	highImpact := directEvidenceConflictHighImpact(lineage, fieldClass)
	classification := directEvidenceConflictClassification(e, lineage, archiveState, captureVerification, confidence)
	route := directEvidenceConflictRoute(e, captureVerification, classification, confidence, highImpact)
	requiresManualReview := route == "manual_review"
	userConfirmationCandidate := highImpact && confidence < 0.9 && (classification == "hard_contradiction" || classification == "state_transition")
	return map[string]any{
		"policy_version":               "ea1h.v1",
		"confidence_policy_version":    "ea1i.v1",
		"classification":               classification,
		"route":                        route,
		"confidence":                   safeRoundFloat(confidence),
		"field_class":                  fieldClass,
		"high_impact":                  highImpact,
		"requires_manual_review":       requiresManualReview,
		"user_confirmation_candidate":  userConfirmationCandidate,
		"archive_state":                archiveState,
		"capture_verification":         captureVerification,
		"committed_gate":               committedGate,
		"retention_importance_tier":    retentionTier,
		"threshold_auto_promote_min":   0.82,
		"threshold_hold_below":         0.55,
		"threshold_high_impact_review": 0.9,
	}
}

func directEvidenceConflictConfidence(lineage map[string]any, captureVerification string, repairNeeded bool) float64 {
	for _, key := range []string{"conflict_confidence", "confidence", "score"} {
		if value, ok := lineage[key]; ok {
			n := floatFromAny(value)
			if n > 0 {
				if n > 1 {
					n = n / 100
				}
				if n > 1 {
					n = 1
				}
				return n
			}
		}
	}
	if repairNeeded || captureVerification == "needs_review" || captureVerification == "rejected" {
		return 0.35
	}
	if captureVerification == "verified" {
		return 0.86
	}
	return 0.45
}

func directEvidenceConflictFieldClass(lineage map[string]any) string {
	for _, key := range []string{"field_class", "conflict_field_class", "target_field"} {
		if value, ok := lineage[key].(string); ok {
			normalized := strings.TrimSpace(strings.ToLower(value))
			if normalized != "" {
				return normalized
			}
		}
	}
	return "canonical_fact"
}

func directEvidenceConflictHighImpact(lineage map[string]any, fieldClass string) bool {
	for _, key := range []string{"high_impact", "manual_review_required", "user_confirmation_candidate"} {
		if value, ok := lineage[key].(bool); ok && value {
			return true
		}
	}
	switch fieldClass {
	case "identity", "relationship", "trust", "world_rule", "canonical_fact":
		return true
	default:
		return false
	}
}

func directEvidenceConflictClassification(e store.DirectEvidence, lineage map[string]any, archiveState, captureVerification string, confidence float64) string {
	if raw, ok := lineage["conflict_class"].(string); ok {
		normalized := strings.TrimSpace(strings.ToLower(raw))
		switch normalized {
		case "state_transition", "hard_contradiction", "parallel_context", "low_confidence_noise":
			return normalized
		}
	}
	if e.Tombstoned || e.SupersededByID > 0 || captureVerification == "rejected" {
		return "hard_contradiction"
	}
	if archiveState == "previous_archive" {
		return "parallel_context"
	}
	if e.RepairNeeded || captureVerification == "needs_review" || confidence < 0.55 {
		return "low_confidence_noise"
	}
	return "state_transition"
}

func directEvidenceConflictRoute(e store.DirectEvidence, captureVerification, classification string, confidence float64, highImpact bool) string {
	if e.Tombstoned {
		return "tombstone"
	}
	if e.SupersededByID > 0 {
		return "superseded"
	}
	if e.RepairNeeded || captureVerification == "needs_review" || captureVerification == "rejected" {
		return "manual_review"
	}
	switch classification {
	case "hard_contradiction":
		if highImpact && confidence < 0.9 {
			return "manual_review"
		}
		if confidence >= 0.82 {
			return "superseded"
		}
		return "hold"
	case "low_confidence_noise", "parallel_context":
		return "hold"
	default:
		if confidence < 0.55 {
			return "hold"
		}
		return "superseded"
	}
}

func directEvidenceRetentionExpired(sourceTurnEnd, latestTurnIndex, ttl int) bool {
	if sourceTurnEnd <= 0 || latestTurnIndex <= 0 || ttl <= 0 {
		return false
	}
	return latestTurnIndex-sourceTurnEnd > ttl
}

func directEvidenceTombstoneRetained(tombstoned bool, sourceTurnEnd, latestTurnIndex int) bool {
	if !tombstoned {
		return false
	}
	if sourceTurnEnd <= 0 || latestTurnIndex <= 0 {
		return true
	}
	return latestTurnIndex-sourceTurnEnd <= 240
}

func sortKGTriplesForPython(triples []store.KGTriple) {
	sort.SliceStable(triples, func(i, j int) bool {
		if triples[i].CreatedAt.Equal(triples[j].CreatedAt) {
			return triples[i].ID > triples[j].ID
		}
		return triples[i].CreatedAt.After(triples[j].CreatedAt)
	})
}

func kgTripleExplorerItem(t store.KGTriple) map[string]any {
	return map[string]any{
		"id":              t.ID,
		"chat_session_id": t.ChatSessionID,
		"subject":         t.Subject,
		"predicate":       t.Predicate,
		"object":          t.Object,
		"valid_from":      nullablePositiveInt(t.ValidFrom),
		"valid_to":        nullablePositiveInt(t.ValidTo),
		"source_turn":     nullablePositiveInt(t.SourceTurn),
		"created_at":      formatKSTTime(t.CreatedAt),
	}
}

func nonEmptyStrings(items []string) []string {
	out := []string{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	if len(out) > 30 {
		return out[:30]
	}
	return out
}

func kgTripleMatchesEntities(t store.KGTriple, entities []string) bool {
	subject := strings.ToLower(t.Subject)
	object := strings.ToLower(t.Object)
	subjectKey := normalizeCharacterKey(t.Subject)
	objectKey := normalizeCharacterKey(t.Object)
	for _, entity := range entities {
		needle := strings.ToLower(entity)
		if needle != "" && (strings.Contains(subject, needle) || strings.Contains(object, needle)) {
			return true
		}
		needleKey := normalizeCharacterKey(entity)
		if kgNormalizedPartMatchesEntity(subjectKey, needleKey) || kgNormalizedPartMatchesEntity(objectKey, needleKey) {
			return true
		}
	}
	return false
}

func kgNormalizedPartMatchesEntity(partKey, entityKey string) bool {
	if len([]rune(partKey)) < 2 || len([]rune(entityKey)) < 2 {
		return false
	}
	return partKey == entityKey || strings.Contains(partKey, entityKey) || strings.Contains(entityKey, partKey)
}

func (s *Server) handleExplorerKGTriples(w http.ResponseWriter, r *http.Request) {
	sid := r.URL.Query().Get("chat_session_id")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 30
	}
	if offset < 0 {
		offset = 0
	}

	items := []any{}
	total := 0

	if s.Store != nil {
		triples, err := s.Store.ListKGTriples(r.Context(), sid)
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			writeInternalError(w, err.Error())
			return
		}
		if err == nil {
			sortKGTriplesForPython(triples)
			total = len(triples)
			start := offset
			if start > len(triples) {
				start = len(triples)
			}
			end := start + limit
			if end > len(triples) {
				end = len(triples)
			}
			for _, t := range triples[start:end] {
				items = append(items, kgTripleExplorerItem(t))
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"items":    items,
		"total":    total,
		"has_more": offset+len(items) < total,
		"limit":    limit,
		"offset":   offset,
	})
}

func explorerHierarchyPageParams(r *http.Request) (int, int) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func explorerHierarchyFetchLimit(limit, offset int) int {
	fetchLimit := limit + offset + 1
	if fetchLimit < limit {
		fetchLimit = limit
	}
	if fetchLimit > 100 {
		fetchLimit = 100
	}
	return fetchLimit
}

func writeExplorerHierarchyItems(w http.ResponseWriter, limit, offset int, items []any) {
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
		"status":   "ok",
		"items":    page,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
		"has_more": offset+len(page) < total,
	})
}

func chapterSummaryExplorerItem(ch store.ChapterSummary, source string) map[string]any {
	return map[string]any{
		"id":                        ch.ID,
		"chat_session_id":           ch.ChatSessionID,
		"from_turn":                 ch.FromTurn,
		"to_turn":                   ch.ToTurn,
		"chapter_index":             ch.ChapterIndex,
		"chapter_title":             ch.ChapterTitle,
		"summary_text":              ch.SummaryText,
		"open_loops_json":           ch.OpenLoopsJSON,
		"relationship_changes_json": ch.RelationshipChangesJSON,
		"world_changes_json":        ch.WorldChangesJSON,
		"callback_candidates_json":  ch.CallbackCandidatesJSON,
		"resume_text":               ch.ResumeText,
		"embedding_model":           ch.EmbeddingModel,
		"created_at":                ch.CreatedAt,
		"source":                    source,
	}
}

func arcSummaryExplorerItem(arc store.ArcSummary) map[string]any {
	return map[string]any{
		"id":                            arc.ID,
		"chat_session_id":               arc.ChatSessionID,
		"from_turn":                     arc.FromTurn,
		"to_turn":                       arc.ToTurn,
		"arc_index":                     arc.ArcIndex,
		"arc_name":                      arc.ArcName,
		"arc_status":                    arc.ArcStatus,
		"core_conflict":                 arc.CoreConflict,
		"key_turning_points_json":       arc.KeyTurningPointsJSON,
		"active_promises_json":          arc.ActivePromisesJSON,
		"unresolved_debts_json":         arc.UnresolvedDebtsJSON,
		"resolved_payoffs_json":         arc.ResolvedPayoffsJSON,
		"callback_candidates_json":      arc.CallbackCandidatesJSON,
		"future_payoff_candidates_json": arc.FuturePayoffCandidatesJSON,
		"irreversible_turns_json":       arc.IrreversibleTurnsJSON,
		"callback_debts_json":           arc.CallbackDebtsJSON,
		"relationship_pivots_json":      arc.RelationshipPivotsJSON,
		"arc_resume_text":               arc.ArcResumeText,
		"embedding_model":               arc.EmbeddingModel,
		"created_at":                    arc.CreatedAt,
		"source":                        "arc_summary",
	}
}

func sagaDigestExplorerItem(saga store.SagaDigest) map[string]any {
	return map[string]any{
		"id":                         saga.ID,
		"chat_session_id":            saga.ChatSessionID,
		"from_turn":                  saga.FromTurn,
		"to_turn":                    saga.ToTurn,
		"era_label":                  saga.EraLabel,
		"saga_summary":               saga.SagaSummary,
		"persistent_facts_json":      saga.PersistentFactsJSON,
		"never_drop_candidates_json": saga.NeverDropCandidatesJSON,
		"resume_pack_text":           saga.ResumePackText,
		"embedding_model":            saga.EmbeddingModel,
		"created_at":                 saga.CreatedAt,
		"source":                     "saga_digest",
	}
}

func (s *Server) handleExplorerChapterSummaries(w http.ResponseWriter, r *http.Request) {
	sid := r.URL.Query().Get("chat_session_id")
	limit, offset := explorerHierarchyPageParams(r)
	if sid == "" {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "items": []any{}, "total": 0, "limit": limit, "offset": offset, "has_more": false})
		return
	}

	items := []any{}

	if s.Store != nil {
		if chapterStore, ok := s.Store.(store.ChapterSummaryStore); ok {
			chapters, err := chapterStore.SearchChapterSummaries(r.Context(), sid, "", 0, 0, explorerHierarchyFetchLimit(limit, offset))
			if err != nil && !errors.Is(err, store.ErrNotEnabled) {
				writeInternalError(w, err.Error())
				return
			}
			if err == nil {
				for _, ch := range chapters {
					items = append(items, chapterSummaryExplorerItem(ch, "chapter_summary"))
				}
			}
		}

		if len(items) == 0 {
			pack, err := s.Store.GetResumePack(r.Context(), sid, "resume")
			if err != nil && !errors.Is(err, store.ErrNotEnabled) {
				writeInternalError(w, err.Error())
				return
			}
			if err == nil && pack != nil && pack.Chapter != nil {
				ch := pack.Chapter
				if ch.ChatSessionID == "" {
					ch.ChatSessionID = sid
				}
				items = append(items, chapterSummaryExplorerItem(*ch, "resume_pack_chapter"))
			}
		}
	}

	writeExplorerHierarchyItems(w, limit, offset, items)
}

func (s *Server) handleExplorerArcSummaries(w http.ResponseWriter, r *http.Request) {
	sid := r.URL.Query().Get("chat_session_id")
	limit, offset := explorerHierarchyPageParams(r)
	if sid == "" {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "items": []any{}, "total": 0, "limit": limit, "offset": offset, "has_more": false})
		return
	}

	items := []any{}
	if s.Store != nil {
		if arcStore, ok := s.Store.(store.ArcSummaryStore); ok {
			arcs, err := arcStore.ListArcSummaries(r.Context(), sid, r.URL.Query().Get("status"), explorerHierarchyFetchLimit(limit, offset))
			if err != nil && !errors.Is(err, store.ErrNotEnabled) {
				writeInternalError(w, err.Error())
				return
			}
			if err == nil {
				for _, arc := range arcs {
					items = append(items, arcSummaryExplorerItem(arc))
				}
			}
		}
	}

	writeExplorerHierarchyItems(w, limit, offset, items)
}

func (s *Server) handleExplorerSagaDigests(w http.ResponseWriter, r *http.Request) {
	sid := r.URL.Query().Get("chat_session_id")
	limit, offset := explorerHierarchyPageParams(r)
	if sid == "" {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "items": []any{}, "total": 0, "limit": limit, "offset": offset, "has_more": false})
		return
	}

	items := []any{}
	if s.Store != nil {
		if sagaStore, ok := s.Store.(store.SagaDigestStore); ok {
			sagas, err := sagaStore.ListSagaDigests(r.Context(), sid, explorerHierarchyFetchLimit(limit, offset))
			if err != nil && !errors.Is(err, store.ErrNotEnabled) {
				writeInternalError(w, err.Error())
				return
			}
			if err == nil {
				for _, saga := range sagas {
					items = append(items, sagaDigestExplorerItem(saga))
				}
			}
		}
	}

	writeExplorerHierarchyItems(w, limit, offset, items)
}

func (s *Server) handleExplorerGet404(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotFound, map[string]any{
		"detail": "Not Found",
	})
}

func memorySummaryPreview(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return ""
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
		parts := []string{}
		for _, key := range []string{"turn_summary", "summary", "scene_summary", "core_meaning", "emotional_shift"} {
			value := strings.TrimSpace(jsonValueString(parsed[key]))
			if key == "turn_summary" {
				value = normalizeCriticTurnSummary(parsed[key])
			} else if looksLikeStructuredCriticPayloadText(value) {
				value = ""
			}
			if value != "" {
				parts = append(parts, truncatePlainForPreview(value, 80))
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, " | ")
		}
		return truncatePlainForPreview(pythonishJSONPreview(raw), 120)
	}
	return truncatePlainForPreview(raw, 120)
}

func nullableFloatZero(v float64) any {
	if v == 0 {
		return nil
	}
	return v
}

func safeRound2Float(v float64) float64 {
	return math.Round(v*100) / 100
}

func jsonValueString(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	b, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(b)
}

func pythonishJSONPreview(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	runes := []rune(raw)
	var b strings.Builder
	b.Grow(len(raw) + 16)
	skipOuterWhitespace := false
	for i := 0; i < len(runes); {
		r := runes[i]
		if r == '"' {
			content, next := readJSONStringPreviewToken(runes, i)
			lookahead := next
			for lookahead < len(runes) && isJSONPreviewWhitespace(runes[lookahead]) {
				lookahead++
			}
			quote := '\''
			if lookahead >= len(runes) || runes[lookahead] != ':' {
				if strings.ContainsRune(content, '\'') && !strings.ContainsRune(content, '"') {
					quote = '"'
				}
			}
			b.WriteRune(quote)
			b.WriteString(content)
			b.WriteRune(quote)
			i = next
			skipOuterWhitespace = false
			continue
		}
		if skipOuterWhitespace && isJSONPreviewWhitespace(r) {
			i++
			continue
		}
		skipOuterWhitespace = false
		switch r {
		case ':':
			b.WriteString(": ")
			skipOuterWhitespace = true
		case ',':
			b.WriteString(", ")
			skipOuterWhitespace = true
		default:
			b.WriteRune(r)
		}
		i++
	}
	return b.String()
}

func readJSONStringPreviewToken(runes []rune, start int) (string, int) {
	var b strings.Builder
	escaped := false
	for i := start + 1; i < len(runes); i++ {
		r := runes[i]
		if escaped {
			switch r {
			case '"', '\\', '/':
				b.WriteRune(r)
			case 'b':
				b.WriteRune('\b')
			case 'f':
				b.WriteRune('\f')
			case 'n':
				b.WriteRune('\n')
			case 'r':
				b.WriteRune('\r')
			case 't':
				b.WriteRune('\t')
			default:
				b.WriteRune(r)
			}
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == '"' {
			return b.String(), i + 1
		}
		b.WriteRune(r)
	}
	return b.String(), len(runes)
}

func isJSONPreviewWhitespace(r rune) bool {
	switch r {
	case ' ', '\t', '\r', '\n':
		return true
	default:
		return false
	}
}

func truncatePlainForPreview(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func truncateForPreview(value string, limit int) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "..."
}
