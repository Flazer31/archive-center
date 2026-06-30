package httpapi

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

const (
	maintenanceCDMContractVersion = "contradiction_duplicate_maintenance.v1"
	maintenanceCDMRoute           = "/maintenance/contradiction-duplicates/preview"
)

type maintenanceCDMCandidate struct {
	ID             string           `json:"id"`
	CandidateType  string           `json:"candidate_type"`
	ProposedAction string           `json:"proposed_action"`
	Confidence     float64          `json:"confidence"`
	Reason         string           `json:"reason"`
	EvidenceBound  bool             `json:"evidence_bound"`
	SourceRefs     []map[string]any `json:"source_refs"`
	Inspection     map[string]any   `json:"inspection"`
}

func (s *Server) handleMaintenanceContradictionDuplicatePreview(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	sid := strings.TrimSpace(query.Get("chat_session_id"))
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	limit := 50
	if raw := strings.TrimSpace(query.Get("limit")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			writeBadRequest(w, "limit must be an integer")
			return
		}
		limit = value
	}
	limit = maintenanceCDMClampInt(limit, 1, 200)

	candidates, warnings, trace := s.buildMaintenanceContradictionDuplicatePreview(r, sid, limit)
	status := "ok"
	if len(warnings) > 0 {
		status = "degraded"
	} else if len(candidates) == 0 {
		status = "empty"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":                 status,
		"contract_version":       maintenanceCDMContractVersion,
		"route":                  maintenanceCDMRoute,
		"session_id":             sid,
		"generated_at":           time.Now().UTC().Format(time.RFC3339),
		"read_only":              true,
		"inspect_only":           true,
		"auto_apply":             false,
		"write_attempted":        false,
		"vector_write_attempted": false,
		"llm_call_attempted":     false,
		"candidate_count":        len(candidates),
		"candidates":             candidates,
		"dashboard":              maintenanceCDMDashboard(candidates, warnings),
		"warnings":               warnings,
		"trace":                  trace,
	})
}

func (s *Server) buildMaintenanceContradictionDuplicatePreview(r *http.Request, sid string, limit int) ([]maintenanceCDMCandidate, []string, map[string]any) {
	ctx := r.Context()
	warnings := []string{}
	sourceCounts := map[string]int{}

	memories, err := s.Store.ListMemories(ctx, sid, 0, 0)
	if err != nil {
		warnings = append(warnings, "memories_unavailable: "+err.Error())
	}
	sourceCounts["memories"] = len(memories)

	evidence, err := s.Store.ListEvidence(ctx, sid)
	if err != nil {
		warnings = append(warnings, "direct_evidence_unavailable: "+err.Error())
	}
	sourceCounts["direct_evidence"] = len(evidence)

	threads, err := s.Store.ListPendingThreads(ctx, sid, "")
	if err != nil {
		warnings = append(warnings, "pending_threads_unavailable: "+err.Error())
	}
	sourceCounts["pending_threads"] = len(threads)

	resolutionAudits, err := s.Store.ListAuditLogs(ctx, sid, "supersession_resolution", 200)
	if err != nil {
		warnings = append(warnings, "supersession_resolution_audits_unavailable: "+err.Error())
	}
	sourceCounts["supersession_resolution_audits"] = len(resolutionAudits)

	candidates := []maintenanceCDMCandidate{}
	candidates = append(candidates, maintenanceCDMScanMemoryDuplicates(memories)...)
	candidates = append(candidates, maintenanceCDMScanDirectEvidence(evidence)...)
	candidates = append(candidates, maintenanceCDMScanPendingThreads(threads, resolutionAudits)...)
	candidates = maintenanceCDMFilterEvidenceBound(candidates)
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Confidence == candidates[j].Confidence {
			return candidates[i].ID < candidates[j].ID
		}
		return candidates[i].Confidence > candidates[j].Confidence
	})
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	return candidates, warnings, map[string]any{
		"contract_owner":         "22-3",
		"source_counts":          sourceCounts,
		"scan_types":             []string{"near_duplicate_memory", "direct_evidence_duplicate", "direct_evidence_conflict", "pending_thread_duplicate", "thread_open_closed_contradiction"},
		"action_surface":         []string{"merge", "supersede", "review"},
		"evidence_bound_only":    true,
		"auto_apply":             false,
		"write_attempted":        false,
		"vector_write_attempted": false,
		"llm_call_attempted":     false,
	}
}

func maintenanceCDMScanMemoryDuplicates(memories []store.Memory) []maintenanceCDMCandidate {
	out := []maintenanceCDMCandidate{}
	maxScan := minInt(len(memories), 160)
	for i := 0; i < maxScan; i++ {
		left := memories[i]
		leftText := maintenanceCDMMemoryText(left)
		if strings.TrimSpace(leftText) == "" || !maintenanceCDMMemoryEvidenceBound(left) {
			continue
		}
		for j := i + 1; j < maxScan; j++ {
			right := memories[j]
			rightText := maintenanceCDMMemoryText(right)
			if strings.TrimSpace(rightText) == "" || !maintenanceCDMMemoryEvidenceBound(right) {
				continue
			}
			score := maintenanceCDMTextSimilarity(leftText, rightText)
			if score < 0.72 && maintenanceCDMTextFingerprint(leftText) != maintenanceCDMTextFingerprint(rightText) {
				continue
			}
			confidence := maintenanceCDMRound(maintenanceCDMMaxFloat(score, 0.78))
			out = append(out, maintenanceCDMCandidate{
				ID:             fmt.Sprintf("memory_duplicate_%d_%d", left.ID, right.ID),
				CandidateType:  "near_duplicate_memory",
				ProposedAction: "merge",
				Confidence:     confidence,
				Reason:         "similar_memory_cluster_detected",
				EvidenceBound:  true,
				SourceRefs: []map[string]any{
					maintenanceCDMMemoryRef(left, leftText),
					maintenanceCDMMemoryRef(right, rightText),
				},
				Inspection: map[string]any{
					"similarity":     confidence,
					"auto_apply":     false,
					"merge_strategy": "review_then_merge_or_keep_separate",
				},
			})
		}
	}
	return out
}

func maintenanceCDMScanDirectEvidence(items []store.DirectEvidence) []maintenanceCDMCandidate {
	out := []maintenanceCDMCandidate{}
	maxScan := minInt(len(items), 160)
	for i := 0; i < maxScan; i++ {
		left := items[i]
		if !maintenanceCDMEvidenceEvidenceBound(left) || left.Tombstoned {
			continue
		}
		for j := i + 1; j < maxScan; j++ {
			right := items[j]
			if !maintenanceCDMEvidenceEvidenceBound(right) || right.Tombstoned {
				continue
			}
			leftText := strings.TrimSpace(left.EvidenceText)
			rightText := strings.TrimSpace(right.EvidenceText)
			if leftText == "" || rightText == "" {
				continue
			}
			sameHash := left.SourceHash != "" && right.SourceHash != "" && left.SourceHash == right.SourceHash
			score := maintenanceCDMTextSimilarity(leftText, rightText)
			if sameHash || score >= 0.78 || maintenanceCDMTextFingerprint(leftText) == maintenanceCDMTextFingerprint(rightText) {
				confidence := maintenanceCDMRound(maintenanceCDMMaxFloat(score, 0.9))
				out = append(out, maintenanceCDMCandidate{
					ID:             fmt.Sprintf("direct_evidence_duplicate_%d_%d", left.ID, right.ID),
					CandidateType:  "direct_evidence_duplicate",
					ProposedAction: "merge",
					Confidence:     confidence,
					Reason:         "duplicate_direct_evidence_detected",
					EvidenceBound:  true,
					SourceRefs: []map[string]any{
						maintenanceCDMEvidenceRef(left),
						maintenanceCDMEvidenceRef(right),
					},
					Inspection: map[string]any{
						"same_source_hash": sameHash,
						"similarity":       confidence,
						"auto_apply":       false,
					},
				})
				continue
			}
			if score < 0.25 {
				continue
			}
			classification := classifyConflict(left.EvidenceText, right)
			if classification == conflictClassHardContradiction {
				out = append(out, maintenanceCDMCandidate{
					ID:             fmt.Sprintf("direct_evidence_conflict_%d_%d", left.ID, right.ID),
					CandidateType:  "direct_evidence_conflict",
					ProposedAction: "review",
					Confidence:     maintenanceCDMRound(maintenanceCDMMaxFloat(score, 0.72)),
					Reason:         "hard_contradiction_requires_manual_review",
					EvidenceBound:  true,
					SourceRefs: []map[string]any{
						maintenanceCDMEvidenceRef(left),
						maintenanceCDMEvidenceRef(right),
					},
					Inspection: map[string]any{
						"classification":        classification,
						"possible_next_actions": []string{"review", "supersede"},
						"auto_apply":            false,
					},
				})
			}
		}
	}
	return out
}

func maintenanceCDMScanPendingThreads(threads []store.PendingThread, audits []store.AuditLog) []maintenanceCDMCandidate {
	out := []maintenanceCDMCandidate{}
	openThreads := []store.PendingThread{}
	for _, item := range threads {
		if item.ID <= 0 || item.Suppressed {
			continue
		}
		if maintenanceCDMThreadOpen(item) {
			openThreads = append(openThreads, item)
		}
		if maintenanceCDMThreadOpen(item) && item.ResolvedTurn > 0 {
			out = append(out, maintenanceCDMCandidate{
				ID:             fmt.Sprintf("thread_open_closed_%d", item.ID),
				CandidateType:  "thread_open_closed_contradiction",
				ProposedAction: "review",
				Confidence:     0.86,
				Reason:         "thread_status_open_but_resolved_turn_is_set",
				EvidenceBound:  true,
				SourceRefs: []map[string]any{
					maintenanceCDMThreadRef(item),
				},
				Inspection: map[string]any{
					"status":        item.Status,
					"resolved_turn": item.ResolvedTurn,
					"auto_apply":    false,
				},
			})
		}
	}
	for i := 0; i < len(openThreads); i++ {
		left := openThreads[i]
		leftText := maintenanceCDMThreadText(left)
		for j := i + 1; j < len(openThreads); j++ {
			right := openThreads[j]
			rightText := maintenanceCDMThreadText(right)
			score := maintenanceCDMTextSimilarity(leftText, rightText)
			if score < 0.75 && maintenanceCDMTextFingerprint(leftText) != maintenanceCDMTextFingerprint(rightText) {
				continue
			}
			confidence := maintenanceCDMRound(maintenanceCDMMaxFloat(score, 0.8))
			out = append(out, maintenanceCDMCandidate{
				ID:             fmt.Sprintf("pending_thread_duplicate_%d_%d", left.ID, right.ID),
				CandidateType:  "pending_thread_duplicate",
				ProposedAction: "merge",
				Confidence:     confidence,
				Reason:         "similar_open_pending_threads_detected",
				EvidenceBound:  true,
				SourceRefs: []map[string]any{
					maintenanceCDMThreadRef(left),
					maintenanceCDMThreadRef(right),
				},
				Inspection: map[string]any{
					"similarity": confidence,
					"auto_apply": false,
				},
			})
		}
	}
	threadByID := map[int64]store.PendingThread{}
	for _, item := range threads {
		threadByID[item.ID] = item
	}
	for _, audit := range audits {
		targetType, targetID, resolutionClass := maintenanceCDMResolutionAuditTarget(audit)
		if targetType != "pending_thread" || targetID <= 0 || !maintenanceCDMClosedResolutionClass(resolutionClass) {
			continue
		}
		thread, ok := threadByID[targetID]
		if !ok || !maintenanceCDMThreadOpen(thread) {
			continue
		}
		out = append(out, maintenanceCDMCandidate{
			ID:             fmt.Sprintf("thread_open_closed_audit_%d_%d", thread.ID, audit.ID),
			CandidateType:  "thread_open_closed_contradiction",
			ProposedAction: "review",
			Confidence:     0.9,
			Reason:         "thread_is_open_but_resolution_audit_exists",
			EvidenceBound:  true,
			SourceRefs: []map[string]any{
				maintenanceCDMThreadRef(thread),
				maintenanceCDMAuditRef(audit),
			},
			Inspection: map[string]any{
				"resolution_class": resolutionClass,
				"auto_apply":       false,
				"allowed_actions":  []string{"review", "supersede"},
			},
		})
	}
	return out
}

func maintenanceCDMFilterEvidenceBound(in []maintenanceCDMCandidate) []maintenanceCDMCandidate {
	out := []maintenanceCDMCandidate{}
	seen := map[string]bool{}
	for _, item := range in {
		if !item.EvidenceBound || len(item.SourceRefs) == 0 || seen[item.ID] {
			continue
		}
		seen[item.ID] = true
		out = append(out, item)
	}
	return out
}

func maintenanceCDMDashboard(candidates []maintenanceCDMCandidate, warnings []string) map[string]any {
	byType := map[string]int{}
	byAction := map[string]int{}
	for _, item := range candidates {
		byType[item.CandidateType]++
		byAction[item.ProposedAction]++
	}
	badge := "ready"
	if len(warnings) > 0 {
		badge = "degraded"
	} else if len(candidates) == 0 {
		badge = "empty"
	}
	return map[string]any{
		"badge_status":        badge,
		"candidate_count":     len(candidates),
		"by_type":             byType,
		"by_action":           byAction,
		"evidence_bound_only": true,
		"auto_apply":          false,
		"available_actions":   []string{"merge", "supersede", "review"},
	}
}

func maintenanceCDMMemoryText(item store.Memory) string {
	return firstNonEmptyLedgerString(ledgerSummaryFromJSONOrText(item.SummaryJSON), item.Evidence)
}

func maintenanceCDMThreadText(item store.PendingThread) string {
	return firstNonEmptyLedgerString(item.Title, item.Description, item.ThreadKey, item.HookMetadataJSON, item.DetailsJSON)
}

func maintenanceCDMMemoryEvidenceBound(item store.Memory) bool {
	return item.ID > 0 && item.TurnIndex > 0
}

func maintenanceCDMEvidenceEvidenceBound(item store.DirectEvidence) bool {
	return item.ID > 0 && (item.SourceTurnStart > 0 || item.TurnAnchor > 0 || strings.TrimSpace(item.SourceHash) != "")
}

func maintenanceCDMThreadOpen(item store.PendingThread) bool {
	switch strings.ToLower(strings.TrimSpace(item.Status)) {
	case "", "open", "active", "pending", "unresolved":
		return true
	default:
		return false
	}
}

func maintenanceCDMResolutionAuditTarget(item store.AuditLog) (string, int64, string) {
	targetType := normalizeMaintenanceCDMTargetType(item.TargetType)
	targetID := item.TargetID
	resolutionClass := ""
	var details map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(item.DetailsJSON)), &details); err == nil {
		if target := jsonMapFromLedgerAny(details["target"]); len(target) > 0 {
			targetType = firstNonEmptyLedgerString(normalizeMaintenanceCDMTargetType(fmt.Sprint(target["type"])), targetType)
			if id := maintenanceCDMInt64(target["id"]); id > 0 {
				targetID = id
			}
		}
		resolutionClass = strings.ToLower(strings.TrimSpace(fmt.Sprint(details["resolution_class"])))
	}
	return targetType, targetID, resolutionClass
}

func maintenanceCDMClosedResolutionClass(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "close", "supersede", "refine", "reverse":
		return true
	default:
		return false
	}
}

func normalizeMaintenanceCDMTargetType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, " ", "_")
	switch value {
	case "pending_threads":
		return "pending_thread"
	case "direct_evidence_records", "direct_evidence_record", "evidence":
		return "direct_evidence"
	case "memories":
		return "memory"
	default:
		return value
	}
}

func maintenanceCDMMemoryRef(item store.Memory, text string) map[string]any {
	return map[string]any{
		"type":       "memory",
		"id":         item.ID,
		"turn_index": item.TurnIndex,
		"preview":    truncateLedgerText(text, 180),
	}
}

func maintenanceCDMEvidenceRef(item store.DirectEvidence) map[string]any {
	return map[string]any{
		"type":                 "direct_evidence",
		"id":                   item.ID,
		"source_turn_start":    item.SourceTurnStart,
		"source_turn_end":      item.SourceTurnEnd,
		"turn_anchor":          item.TurnAnchor,
		"archive_state":        item.ArchiveState,
		"capture_verification": item.CaptureVerification,
		"superseded_by_id":     nullablePositiveInt64(item.SupersededByID),
		"preview":              truncateLedgerText(item.EvidenceText, 180),
	}
}

func maintenanceCDMThreadRef(item store.PendingThread) map[string]any {
	return map[string]any{
		"type":          "pending_thread",
		"id":            item.ID,
		"status":        item.Status,
		"source_turn":   item.SourceTurn,
		"resolved_turn": item.ResolvedTurn,
		"preview":       truncateLedgerText(maintenanceCDMThreadText(item), 180),
	}
}

func maintenanceCDMAuditRef(item store.AuditLog) map[string]any {
	return map[string]any{
		"type":        "audit_log",
		"id":          item.ID,
		"event_type":  item.EventType,
		"target_type": item.TargetType,
		"target_id":   item.TargetID,
		"preview":     truncateLedgerText(firstNonEmptyLedgerString(item.Summary, item.DetailsJSON), 180),
	}
}

func maintenanceCDMTextSimilarity(left, right string) float64 {
	leftTokens := maintenanceCDMTokens(left)
	rightTokens := maintenanceCDMTokens(right)
	if len(leftTokens) == 0 || len(rightTokens) == 0 {
		return 0
	}
	intersection := 0
	union := map[string]bool{}
	for token := range leftTokens {
		union[token] = true
		if rightTokens[token] {
			intersection++
		}
	}
	for token := range rightTokens {
		union[token] = true
	}
	return float64(intersection) / float64(len(union))
}

func maintenanceCDMTokens(text string) map[string]bool {
	tokens := map[string]bool{}
	var current []rune
	flush := func() {
		if len(current) == 0 {
			return
		}
		token := strings.ToLower(string(current))
		current = current[:0]
		if len([]rune(token)) < 2 || maintenanceCDMStopword(token) {
			return
		}
		tokens[token] = true
	}
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			current = append(current, unicode.ToLower(r))
			continue
		}
		flush()
	}
	flush()
	return tokens
}

func maintenanceCDMTextFingerprint(text string) string {
	tokens := maintenanceCDMTokens(text)
	ordered := make([]string, 0, len(tokens))
	for token := range tokens {
		ordered = append(ordered, token)
	}
	sort.Strings(ordered)
	if len(ordered) > 12 {
		ordered = ordered[:12]
	}
	return strings.Join(ordered, " ")
}

func maintenanceCDMStopword(token string) bool {
	switch token {
	case "the", "and", "for", "with", "that", "this", "from", "into", "about", "after", "before", "they", "their", "them", "she", "her", "him", "his", "was", "were", "are", "is", "to", "of", "in", "on", "at", "a", "an":
		return true
	default:
		return false
	}
}

func maintenanceCDMRound(value float64) float64 {
	return math.Round(value*100) / 100
}

func maintenanceCDMClampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func maintenanceCDMMaxFloat(left, right float64) float64 {
	if left > right {
		return left
	}
	return right
}

func maintenanceCDMInt64(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	case json.Number:
		out, _ := typed.Int64()
		return out
	case string:
		out, _ := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return out
	default:
		return 0
	}
}

func jsonMapFromLedgerAny(value any) map[string]any {
	out, _ := value.(map[string]any)
	return out
}
