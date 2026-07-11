package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

// registerAdminRoutes mounts maintenance and admin endpoints.
func (s *Server) registerAdminRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /maintenance/queue-status", s.handleMaintenanceQueueStatus)
	mux.HandleFunc("GET /maintenance/contradiction-duplicates/preview", s.handleMaintenanceContradictionDuplicatePreview)
	mux.HandleFunc("POST /maintenance-pass/{chat_session_id}", s.handleMaintenancePass)
	mux.HandleFunc("POST /maintenance/enqueue", s.handleMaintenanceEnqueue)
	mux.HandleFunc("POST /admin/database-reset", s.handleAdminDatabaseReset)
	mux.HandleFunc("POST /admin/reindex", s.handleAdminReindex)
	mux.HandleFunc("POST /admin/vector-orphan-audit", s.handleAdminVectorOrphanAudit)
	mux.HandleFunc("POST /admin/dedupe-cleanup", s.handleAdminDedupeCleanup)
	mux.HandleFunc("POST /admin/rescan", s.handleAdminRescan)
	mux.HandleFunc("POST /admin/session-normalize", s.handleAdminSessionNormalize)
	mux.HandleFunc("GET /admin/jobs", s.handleAdminJobs)
	mux.HandleFunc("GET /admin/jobs/{job_id}", s.handleAdminJob)
	mux.HandleFunc("POST /admin/session-migrate", s.handleAdminSessionMigrate)
}

const adminDatabaseResetConfirm = "RESET_ARCHIVE_CENTER_DB"

type adminDatabaseResetRequest struct {
	Debug       bool   `json:"debug"`
	Confirm     string `json:"confirm"`
	ResetVector *bool  `json:"reset_vector"`
}

type adminVectorResetter interface {
	ResetAll(ctx context.Context) error
}

func (s *Server) handleAdminDatabaseReset(w http.ResponseWriter, r *http.Request) {
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, "POST /admin/database-reset")
		return
	}
	var req adminDatabaseResetRequest
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if !req.Debug || strings.TrimSpace(req.Confirm) != adminDatabaseResetConfirm {
		writeForbidden(w, "database reset requires debug=true and the exact confirmation token")
		return
	}
	resetStore, ok := s.Store.(store.AdminResetStore)
	if !ok {
		writeError(w, http.StatusNotImplemented, "not_implemented", "store does not support full database reset")
		return
	}

	resetVector := true
	if req.ResetVector != nil {
		resetVector = *req.ResetVector
	}
	vectorStatus := "skipped_by_request"
	if resetVector {
		switch {
		case strings.TrimSpace(s.Cfg.ChromaEndpoint) == "":
			vectorStatus = "skipped_no_chroma_endpoint"
		default:
			vectorResetter, ok := s.Vector.(adminVectorResetter)
			if !ok {
				writeError(w, http.StatusNotImplemented, "not_implemented", "configured vector store does not support full reset")
				return
			}
			if err := vectorResetter.ResetAll(r.Context()); err != nil {
				writeInternalError(w, "vector reset failed: "+err.Error())
				return
			}
			vectorStatus = "ok"
		}
	}

	result, err := resetStore.ResetAll(r.Context())
	if err != nil {
		writeInternalError(w, "database reset failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":              "ok",
		"source":              s.storeWriteSource(),
		"debug_required":      true,
		"confirm_token":       adminDatabaseResetConfirm,
		"schema_preserved":    true,
		"mariadb_reset":       true,
		"tables_cleared":      result.TablesCleared,
		"rows_deleted":        result.RowsDeleted,
		"vector_reset":        resetVector,
		"vector_reset_status": vectorStatus,
		"note":                "all Archive Center application rows were deleted; schema remains intact",
	})
}

func (s *Server) handleMaintenanceQueueStatus(w http.ResponseWriter, r *http.Request) {
	sid := strings.TrimSpace(r.URL.Query().Get("chat_session_id"))
	eventType := strings.TrimSpace(r.URL.Query().Get("event_type"))
	limit := parseMaintenanceQueueLimit(r.URL.Query().Get("limit"))

	items := []store.AuditLog{}
	if s.Store != nil {
		logs, err := s.Store.ListAuditLogs(r.Context(), sid, eventType, limit)
		if err != nil {
			if errors.Is(err, store.ErrNotEnabled) {
				writeJSON(w, http.StatusOK, map[string]any{
					"status":          "ok",
					"source":          "shadow",
					"chat_session_id": sid,
					"queue_depth":     0,
					"audit_count":     0,
					"items":           []any{},
					"trace_summary": map[string]any{
						"store_backed": false,
						"reason":       "store_not_enabled",
					},
					"note": "maintenance queue-status store is not enabled in this R1 mode",
				})
				return
			}
			writeInternalError(w, err.Error())
			return
		}
		items = logs
	}

	statusCounts := map[string]int{}
	for _, item := range items {
		key := strings.TrimSpace(item.EventType)
		if key == "" {
			key = "unknown"
		}
		statusCounts[key]++
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":          "ok",
		"source":          "store_audit_shadow",
		"chat_session_id": sid,
		"event_type":      eventType,
		"queue_depth":     len(items),
		"audit_count":     len(items),
		"status_counts":   statusCounts,
		"items":           items,
		"trace_summary": map[string]any{
			"store_backed": true,
			"limit":        limit,
			"r1_shadow":    true,
		},
		"note": "maintenance queue-status is Store-backed R1 shadow evidence; no worker authority is enabled",
	})
}

func parseMaintenanceQueueLimit(raw string) int {
	limit, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || limit <= 0 {
		return 50
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func (s *Server) handleMaintenancePass(w http.ResponseWriter, r *http.Request) {
	s.handleMaintenanceShadowHandoff(w, r, r.PathValue("chat_session_id"), "maintenance_pass")
}

func (s *Server) handleMaintenanceEnqueue(w http.ResponseWriter, r *http.Request) {
	s.handleMaintenanceShadowHandoff(w, r, "", "maintenance_enqueue")
}

func (s *Server) handleMaintenanceShadowHandoff(w http.ResponseWriter, r *http.Request, pathSID, action string) {
	payload := map[string]any{}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			if errors.Is(err, io.EOF) {
				payload = map[string]any{}
			} else {
				writeJSON(w, http.StatusOK, map[string]any{
					"status":                   "skipped_malformed_payload",
					"source":                   "maintenance_shadow",
					"action":                   action,
					"queue_depth":              0,
					"shadow_only":              true,
					"maintenance_pass_enabled": false,
					"worker_enabled":           false,
					"error":                    err.Error(),
					"trace_summary": map[string]any{
						"non_blocking": true,
						"fallback":     "malformed_payload",
					},
					"note": "maintenance shadow payload was malformed; chat flow must continue",
				})
				return
			}
		}
	}

	sid := strings.TrimSpace(pathSID)
	if sid == "" {
		sid = strings.TrimSpace(extractionStringFromAny(payload["chat_session_id"]))
	}
	turnIndex := intFromAny(payload["turn_index"], -1)
	shadowOnly := true
	if _, ok := payload["shadow_only"]; ok {
		shadowOnly = completeTurnBoolFromAny(payload["shadow_only"])
	}
	assistantPreview := truncateRunes(strings.TrimSpace(extractionStringFromAny(payload["assistant_response"])), 400)
	recentResponses := asAnySlice(payload["recent_responses"])
	driftSignals, correctionHints := buildMaintenanceDriftSignals(payload, assistantPreview, recentResponses)
	maintenancePassState := buildMaintenancePassStateTM1b(payload, turnIndex, driftSignals)
	importanceReweighting := s.buildMaintenanceImportanceReweightingTM1c(r.Context(), sid, payload, turnIndex)
	now := time.Now().UTC()
	eventType := "maintenance_enqueued"
	if len(driftSignals) > 0 {
		eventType = "drift_detected"
	} else if intFromAny(importanceReweighting["updated_count"], 0) > 0 {
		eventType = "importance_reevaluation"
	}
	tm1dContract := map[string]any{}
	if eventType == "drift_detected" || eventType == "importance_reevaluation" {
		tm1dContract = buildTM1dAuditReplayContract(eventType, maintenancePassState, importanceReweighting, turnIndex)
	}
	refreshOutput := map[string]any{
		"story_plan_refresh":            "shadow_candidate",
		"director_refresh":              "shadow_candidate",
		"writeback_enabled":             false,
		"guidance_state_mutation":       false,
		"assistant_preview_chars":       len([]rune(assistantPreview)),
		"recent_response_samples":       minInt(len(recentResponses), 5),
		"drift_signal_count":            len(driftSignals),
		"correction_hint_count":         len(correctionHints),
		"partial_failure_fallback":      "continue_chat",
		"maintenance_pass_state":        maintenancePassState,
		"memory_importance_reweighting": importanceReweighting,
	}
	for k, v := range tm1dContract {
		refreshOutput[k] = v
	}
	trace := map[string]any{
		"owner":                         "maintenance_shadow",
		"action":                        action,
		"status":                        "audit_shadow_enqueued",
		"non_blocking":                  true,
		"queue_mode":                    "audit_shadow",
		"shadow_only":                   shadowOnly,
		"worker_enabled":                false,
		"maintenance_pass_enabled":      false,
		"refresh_output":                refreshOutput,
		"drift_signals":                 driftSignals,
		"correction_hints":              correctionHints,
		"maintenance_pass_state":        maintenancePassState,
		"memory_importance_reweighting": importanceReweighting,
	}

	auditSaved := false
	auditErr := ""
	if s.Store != nil {
		auditDetails := map[string]any{
			"action":                        action,
			"turn_index":                    turnIndex,
			"shadow_only":                   shadowOnly,
			"refresh_output":                refreshOutput,
			"assistant_chars":               len([]rune(assistantPreview)),
			"drift_signal_types":            maintenanceSignalTypesTM1b(driftSignals),
			"drift_signals_json":            maintenancePassState["drift_signals_json"],
			"maintenance_pass_state":        maintenancePassState,
			"confidence_floor":              maintenanceConfidenceFloorTM1b,
			"memory_importance_reweighting": importanceReweighting,
		}
		for k, v := range tm1dContract {
			auditDetails[k] = v
		}
		err := s.Store.SaveAuditLog(r.Context(), &store.AuditLog{
			ChatSessionID: sid,
			EventType:     eventType,
			TargetType:    "turn",
			TargetID:      int64(turnIndex),
			Summary:       fmt.Sprintf("%s shadow handoff queued turn %d", action, turnIndex),
			DetailsJSON:   mustCompactJSON(auditDetails),
			Source:        s.storeWriteSource(),
			CreatedAt:     now,
		})
		if err != nil {
			auditErr = err.Error()
			trace["status"] = "audit_shadow_enqueue_failed"
			trace["error"] = auditErr
		} else {
			auditSaved = true
		}
	}
	queueDepth := 0
	if auditSaved {
		queueDepth = 1
	}
	status := "ok"
	if auditErr != "" {
		status = "audit_shadow_enqueue_failed"
	}
	lastVerifiedTurn := intFromAny(payload["last_verified_turn"], turnIndex)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":                        status,
		"source":                        "store_audit_shadow",
		"action":                        action,
		"chat_session_id":               sid,
		"turn_index":                    turnIndex,
		"last_verified_turn":            lastVerifiedTurn,
		"queue_depth":                   queueDepth,
		"shadow_only":                   shadowOnly,
		"maintenance_pass_enabled":      false,
		"worker_enabled":                false,
		"audit_written":                 auditSaved,
		"audit_error":                   auditErr,
		"refresh_output":                refreshOutput,
		"drift_signals":                 driftSignals,
		"correction_hints":              correctionHints,
		"maintenance_pass_state":        maintenancePassState,
		"memory_importance_reweighting": importanceReweighting,
		"trace_summary":                 trace,
		"changed_at":                    now,
		"note":                          "maintenance is audit-shadow only; no guidance state mutation or worker authority is enabled",
	})
}

const maintenanceConfidenceFloorTM1b = 0.3

const (
	maintenanceTM1cPolicyVersion          = "tm1c.v1"
	maintenanceTM1cShadowVersion          = "tm1c.shadow.v1"
	maintenanceTM1cMemoryScanLimit        = 24
	maintenanceTM1cRecentTurnWindow       = 2
	maintenanceTM1cImportanceFloor        = 0.1
	maintenanceTM1cImportanceCeil         = 1.0
	maintenanceTM1cFreshnessDecayStart    = 6
	maintenanceTM1cFreshnessDecayStep     = 0.04
	maintenanceTM1cResolvedReferenceDecay = 0.08
	maintenanceTM1cRecentRementionBoost   = 0.06
	maintenanceTM1cEmotionalDecayStart    = 8
	maintenanceTM1cEmotionalDecay         = 0.05
	maintenanceTM1cUpdateThreshold        = 0.0049
)

func buildMaintenancePassStateTM1b(payload map[string]any, turnIndex int, driftSignals []map[string]any) map[string]any {
	updates := []map[string]any{}
	driftDetected := len(driftSignals) > 0
	severity := strongestMaintenanceSignalSeverityTM1b(driftSignals)
	delta := maintenanceConfidenceDeltaTM1b(severity)
	for _, raw := range asAnySlice(payload["canonical_state_layers"]) {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		layerType := strings.TrimSpace(extractionStringFromAny(item["layer_type"]))
		if layerType == "" {
			layerType = strings.TrimSpace(extractionStringFromAny(item["state_type"]))
		}
		lastVerified := intFromAny(item["last_verified_turn"], intFromAny(item["source_turn"], -1))
		confidence := maintenanceFloatFromAnyTM1b(item["confidence"], 0)
		nextConfidence := confidence
		if driftDetected {
			nextConfidence = confidence - delta
			if nextConfidence < maintenanceConfidenceFloorTM1b {
				nextConfidence = maintenanceConfidenceFloorTM1b
			}
		} else if turnIndex >= 0 {
			lastVerified = turnIndex
		}
		updates = append(updates, map[string]any{
			"layer_type":               layerType,
			"last_verified_turn":       lastVerified,
			"confidence":               safeRoundFloat(confidence),
			"next_confidence":          safeRoundFloat(nextConfidence),
			"confidence_delta":         safeRoundFloat(nextConfidence - confidence),
			"confidence_floor":         maintenanceConfidenceFloorTM1b,
			"would_update_provenance":  !driftDetected && turnIndex >= 0,
			"would_degrade_confidence": driftDetected,
		})
	}
	return map[string]any{
		"surface":                   "MaintenancePassState",
		"version":                   "tm1b.shadow.v1",
		"status":                    "shadow_only",
		"would_write":               false,
		"drift_detected":            driftDetected,
		"drift_signal_count":        len(driftSignals),
		"drift_signal_types":        maintenanceSignalTypesTM1b(driftSignals),
		"drift_signals_json":        mustCompactJSON(driftSignals),
		"confidence_floor":          maintenanceConfidenceFloorTM1b,
		"confidence_degradation":    map[string]any{"low": -0.05, "medium": -0.10, "high": -0.15, "floor": maintenanceConfidenceFloorTM1b},
		"strongest_signal_severity": severity,
		"canonical_updates":         updates,
	}
}

func (s *Server) buildMaintenanceImportanceReweightingTM1c(ctx context.Context, sid string, payload map[string]any, turnIndex int) map[string]any {
	state := map[string]any{
		"surface":                      "MemoryImportanceReweighting",
		"version":                      maintenanceTM1cShadowVersion,
		"policy_version":               maintenanceTM1cPolicyVersion,
		"status":                       "shadow_only",
		"would_write":                  false,
		"importance_scale":             "0..1",
		"scan_limit":                   maintenanceTM1cMemoryScanLimit,
		"recent_remention_turn_window": maintenanceTM1cRecentTurnWindow,
		"freshness_decay_start_turns":  maintenanceTM1cFreshnessDecayStart,
		"protected_exemptions":         []string{"pinned", "user_corrected"},
		"updates":                      []map[string]any{},
		"updated_count":                0,
		"boosted_count":                0,
		"decayed_count":                0,
		"protected_count":              0,
	}
	if turnIndex < 0 {
		state["status"] = "skipped_no_turn_index"
		return state
	}

	memories, memorySource, memoryErr := s.maintenanceTM1cMemories(ctx, sid, payload)
	storylines, storylineErr := s.maintenanceTM1cStorylines(ctx, sid, payload)
	pendingThreads, pendingErr := s.maintenanceTM1cPendingThreads(ctx, sid, payload)
	recentText, recentSource, recentErr := s.maintenanceTM1cRecentText(ctx, sid, payload, turnIndex)
	errs := compactNonEmptyStrings([]string{memoryErr, storylineErr, pendingErr, recentErr})
	state["source"] = memorySource
	state["recent_source"] = recentSource
	if len(errs) > 0 {
		state["source_errors"] = errs
	}

	sort.SliceStable(memories, func(i, j int) bool {
		if memories[i].TurnIndex == memories[j].TurnIndex {
			return memories[i].ID > memories[j].ID
		}
		return memories[i].TurnIndex > memories[j].TurnIndex
	})
	if len(memories) > maintenanceTM1cMemoryScanLimit {
		memories = memories[:maintenanceTM1cMemoryScanLimit]
	}
	recentKeywords := maintenanceTM1cExtractKeywords(recentText)
	resolvedSignals, protectedSignals := maintenanceTM1cReferenceSignals(storylines, pendingThreads)

	updates := []map[string]any{}
	protectedCount := 0
	boostedCount := 0
	decayedCount := 0
	for _, mem := range memories {
		summaryText := maintenanceTM1cMemorySummaryText(mem)
		memoryKeywords := maintenanceTM1cExtractKeywords(summaryText)
		if strings.TrimSpace(summaryText) == "" || len(memoryKeywords) == 0 {
			continue
		}
		ageGap := turnIndex - mem.TurnIndex
		if ageGap < 0 {
			continue
		}

		oldImportance := maintenanceTM1cSummaryImportance(mem.SummaryJSON)
		if mem.Importance > 0 {
			oldImportance = maintenanceTM1cClampImportance(mem.Importance)
		}
		newImportance := oldImportance
		reasons := []string{}

		protectedTitles := []string{}
		for _, signal := range protectedSignals {
			if maintenanceTM1cMatches(memoryKeywords, summaryText, signal.keywords, signal.text) {
				protectedTitles = append(protectedTitles, signal.title)
			}
		}
		isProtected := len(protectedTitles) > 0
		if isProtected {
			protectedCount++
			reasons = append(reasons, "protected_reference")
		}

		recentRementioned := len(recentKeywords) > 0 && maintenanceTM1cMatches(memoryKeywords, summaryText, recentKeywords, recentText)
		if recentRementioned {
			newImportance += maintenanceTM1cRecentRementionBoost
			reasons = append(reasons, "recent_remention_boost")
		}

		if !isProtected && ageGap >= maintenanceTM1cFreshnessDecayStart {
			steps := 1 + maxInt(0, (ageGap-maintenanceTM1cFreshnessDecayStart)/maintenanceTM1cFreshnessDecayStart)
			newImportance -= float64(steps) * maintenanceTM1cFreshnessDecayStep
			reasons = append(reasons, "freshness_decay")
		}

		resolvedTitles := []string{}
		if !isProtected {
			for _, signal := range resolvedSignals {
				if maintenanceTM1cMatches(memoryKeywords, summaryText, signal.keywords, signal.text) {
					resolvedTitles = append(resolvedTitles, signal.title)
				}
			}
			if len(resolvedTitles) > 0 {
				newImportance -= maintenanceTM1cResolvedReferenceDecay
				reasons = append(reasons, "resolved_reference_decay")
			}
		}

		hasEmotionalSignal := mem.EmotionalBoost > 0 || mem.EmotionalIntensity >= 0.7
		if !isProtected && !recentRementioned && hasEmotionalSignal && ageGap >= maintenanceTM1cEmotionalDecayStart {
			newImportance -= maintenanceTM1cEmotionalDecay
			reasons = append(reasons, "emotional_decay")
		}

		newImportance = maintenanceTM1cRound(maintenanceTM1cClampImportance(newImportance))
		oldImportance = maintenanceTM1cRound(oldImportance)
		if math.Abs(newImportance-oldImportance) < maintenanceTM1cUpdateThreshold {
			continue
		}
		if newImportance > oldImportance {
			boostedCount++
		}
		if newImportance < oldImportance {
			decayedCount++
		}
		updates = append(updates, map[string]any{
			"memory_id":                      mem.ID,
			"turn_index":                     mem.TurnIndex,
			"old_importance":                 oldImportance,
			"next_importance":                newImportance,
			"importance_delta":               maintenanceTM1cRound(newImportance - oldImportance),
			"age_gap":                        ageGap,
			"recent_rementioned":             recentRementioned,
			"protected_titles":               protectedTitles,
			"resolved_titles":                resolvedTitles,
			"reasons":                        reasons,
			"would_update_memory_importance": true,
		})
	}

	state["scanned_count"] = len(memories)
	state["resolved_reference_count"] = len(resolvedSignals)
	state["protected_reference_count"] = len(protectedSignals)
	state["recent_keyword_count"] = len(recentKeywords)
	state["updates"] = updates
	state["updated_count"] = len(updates)
	state["boosted_count"] = boostedCount
	state["decayed_count"] = decayedCount
	state["protected_count"] = protectedCount
	return state
}

func (s *Server) maintenanceTM1cMemories(ctx context.Context, sid string, payload map[string]any) ([]store.Memory, string, string) {
	if s.Store != nil && strings.TrimSpace(sid) != "" {
		items, err := s.Store.ListMemories(ctx, sid, 0, 0)
		if err == nil {
			return items, "store", ""
		}
		if !errors.Is(err, store.ErrNotEnabled) {
			return maintenanceTM1cPayloadMemories(payload), "payload", "ListMemories: " + err.Error()
		}
	}
	return maintenanceTM1cPayloadMemories(payload), "payload", ""
}

func (s *Server) maintenanceTM1cStorylines(ctx context.Context, sid string, payload map[string]any) ([]store.Storyline, string) {
	if s.Store != nil && strings.TrimSpace(sid) != "" {
		items, err := s.Store.ListStorylines(ctx, sid)
		if err == nil {
			return items, ""
		}
		if !errors.Is(err, store.ErrNotEnabled) {
			return maintenanceTM1cPayloadStorylines(payload), "ListStorylines: " + err.Error()
		}
	}
	return maintenanceTM1cPayloadStorylines(payload), ""
}

func (s *Server) maintenanceTM1cPendingThreads(ctx context.Context, sid string, payload map[string]any) ([]store.PendingThread, string) {
	if s.Store != nil && strings.TrimSpace(sid) != "" {
		items, err := s.Store.ListPendingThreads(ctx, sid, "")
		if err == nil {
			return items, ""
		}
		if !errors.Is(err, store.ErrNotEnabled) {
			return maintenanceTM1cPayloadPendingThreads(payload), "ListPendingThreads: " + err.Error()
		}
	}
	return maintenanceTM1cPayloadPendingThreads(payload), ""
}

func (s *Server) maintenanceTM1cRecentText(ctx context.Context, sid string, payload map[string]any, turnIndex int) (string, string, string) {
	if s.Store != nil && strings.TrimSpace(sid) != "" {
		fromTurn := maxInt(0, turnIndex-maintenanceTM1cRecentTurnWindow)
		items, err := s.Store.ListChatLogs(ctx, sid, fromTurn, turnIndex)
		if err == nil && len(items) > 0 {
			parts := make([]string, 0, len(items))
			for _, item := range items {
				parts = append(parts, item.Content)
			}
			return strings.Join(parts, " "), "store", ""
		}
		if err != nil && !errors.Is(err, store.ErrNotEnabled) {
			return maintenanceTM1cPayloadRecentText(payload), "payload", "ListChatLogs: " + err.Error()
		}
	}
	return maintenanceTM1cPayloadRecentText(payload), "payload", ""
}

type maintenanceTM1cSignal struct {
	kind     string
	title    string
	text     string
	keywords map[string]bool
}

func maintenanceTM1cReferenceSignals(storylines []store.Storyline, pendingThreads []store.PendingThread) ([]maintenanceTM1cSignal, []maintenanceTM1cSignal) {
	resolved := []maintenanceTM1cSignal{}
	protected := []maintenanceTM1cSignal{}
	add := func(entry maintenanceTM1cSignal, status string, pinned, userCorrected bool) {
		if strings.TrimSpace(entry.text) == "" || len(entry.keywords) == 0 {
			return
		}
		if strings.EqualFold(strings.TrimSpace(status), "resolved") {
			resolved = append(resolved, entry)
		}
		if pinned || userCorrected {
			protected = append(protected, entry)
		}
	}
	for _, item := range storylines {
		text := strings.Join(compactNonEmptyStrings([]string{item.Name, item.CurrentContext, item.KeyPointsJSON, item.OngoingTensionsJSON}), " ")
		add(maintenanceTM1cSignal{
			kind:     "storyline",
			title:    item.Name,
			text:     text,
			keywords: maintenanceTM1cExtractKeywords(text),
		}, item.Status, item.Pinned, item.UserCorrected)
	}
	for _, item := range pendingThreads {
		title := extractionFirstNonEmpty(item.ThreadKey, item.Title, item.Description)
		text := strings.Join(compactNonEmptyStrings([]string{item.ThreadKey, item.Title, item.Description, item.DetailsJSON, item.HookMetadataJSON, item.ResolutionNote}), " ")
		add(maintenanceTM1cSignal{
			kind:     "pending_thread",
			title:    title,
			text:     text,
			keywords: maintenanceTM1cExtractKeywords(text),
		}, item.Status, item.Pinned, item.UserCorrected)
	}
	return resolved, protected
}

func maintenanceTM1cPayloadMemories(payload map[string]any) []store.Memory {
	items := asAnySlice(payload["memories"])
	if len(items) == 0 {
		items = asAnySlice(payload["memory_candidates"])
	}
	out := []store.Memory{}
	for _, raw := range items {
		item := mapFromAny(raw)
		if len(item) == 0 {
			continue
		}
		summary := extractionFirstNonEmpty(extractionStringFromAny(item["summary_json"]), extractionStringFromAny(item["summary"]), extractionStringFromAny(item["turn_summary"]))
		out = append(out, store.Memory{
			ID:                 int64FromMap(item, "id", int64FromMap(item, "memory_id", 0)),
			TurnIndex:          intFromAny(item["turn_index"], intFromAny(item["source_turn"], 0)),
			SummaryJSON:        summary,
			Importance:         maintenanceTM1cNormalizeImportance(maintenanceFloatFromAnyTM1b(item["importance"], maintenanceFloatFromAnyTM1b(item["importance_score"], 0))),
			EmotionalBoost:     maintenanceFloatFromAnyTM1b(item["emotional_boost"], 0),
			EmotionalIntensity: maintenanceFloatFromAnyTM1b(item["emotional_intensity"], 0),
			Evidence:           extractionStringFromAny(item["evidence"]),
		})
	}
	return out
}

func maintenanceTM1cPayloadStorylines(payload map[string]any) []store.Storyline {
	out := []store.Storyline{}
	for _, raw := range asAnySlice(payload["storylines"]) {
		item := mapFromAny(raw)
		if len(item) == 0 {
			continue
		}
		out = append(out, store.Storyline{
			Name:                extractionStringFromAny(item["name"]),
			Status:              extractionStringFromAny(item["status"]),
			CurrentContext:      extractionStringFromAny(item["current_context"]),
			KeyPointsJSON:       extractionStringFromAny(item["key_points_json"]),
			OngoingTensionsJSON: extractionStringFromAny(item["ongoing_tensions_json"]),
			Pinned:              completeTurnBoolFromAny(item["pinned"]),
			UserCorrected:       completeTurnBoolFromAny(item["user_corrected"]),
		})
	}
	return out
}

func maintenanceTM1cPayloadPendingThreads(payload map[string]any) []store.PendingThread {
	out := []store.PendingThread{}
	for _, raw := range asAnySlice(payload["pending_threads"]) {
		item := mapFromAny(raw)
		if len(item) == 0 {
			continue
		}
		out = append(out, store.PendingThread{
			ThreadKey:        extractionStringFromAny(item["thread_key"]),
			Description:      extractionStringFromAny(item["description"]),
			Status:           extractionStringFromAny(item["status"]),
			HookMetadataJSON: extractionStringFromAny(item["hook_metadata_json"]),
			Title:            extractionStringFromAny(item["title"]),
			DetailsJSON:      extractionStringFromAny(item["details_json"]),
			ResolutionNote:   extractionStringFromAny(item["resolution_note"]),
			Pinned:           completeTurnBoolFromAny(item["pinned"]),
			UserCorrected:    completeTurnBoolFromAny(item["user_corrected"]),
		})
	}
	return out
}

func maintenanceTM1cPayloadRecentText(payload map[string]any) string {
	parts := []string{}
	for _, item := range asAnySlice(payload["recent_chat_logs"]) {
		if m := mapFromAny(item); len(m) > 0 {
			parts = append(parts, extractionStringFromAny(m["content"]))
		} else {
			parts = append(parts, extractionStringFromAny(item))
		}
	}
	for _, item := range asAnySlice(payload["recent_responses"]) {
		parts = append(parts, extractionStringFromAny(item))
	}
	if assistant := strings.TrimSpace(extractionStringFromAny(payload["assistant_response"])); assistant != "" {
		parts = append(parts, assistant)
	}
	return strings.Join(compactNonEmptyStrings(parts), " ")
}

func maintenanceTM1cMemorySummaryText(mem store.Memory) string {
	if parsed := mapFromJSONString(mem.SummaryJSON); len(parsed) > 0 {
		if text := extractionFirstNonEmpty(extractionStringFromAny(parsed["turn_summary"]), extractionStringFromAny(parsed["summary"])); text != "" {
			return text
		}
	}
	return strings.Join(compactNonEmptyStrings([]string{mem.SummaryJSON, mem.Evidence}), " ")
}

func maintenanceTM1cSummaryImportance(summaryJSON string) float64 {
	if parsed := mapFromJSONString(summaryJSON); len(parsed) > 0 {
		return maintenanceTM1cNormalizeImportance(maintenanceFloatFromAnyTM1b(parsed["importance_score"], 0.5))
	}
	return 0.5
}

func maintenanceTM1cMatches(memoryKeywords map[string]bool, memoryText string, signalKeywords map[string]bool, signalText string) bool {
	if strings.TrimSpace(memoryText) == "" || strings.TrimSpace(signalText) == "" {
		return false
	}
	normalizedMemory := normalizeMaintenanceComparableText(memoryText)
	normalizedSignal := normalizeMaintenanceComparableText(signalText)
	if len([]rune(normalizedSignal)) >= 5 && strings.Contains(normalizedMemory, normalizedSignal) {
		return true
	}
	overlap := 0
	for keyword := range signalKeywords {
		if memoryKeywords[keyword] {
			overlap++
		}
	}
	if overlap == 0 {
		return false
	}
	required := 2
	if len(memoryKeywords) <= 1 || len(signalKeywords) <= 1 {
		required = 1
	}
	return overlap >= required
}

func maintenanceTM1cExtractKeywords(text string) map[string]bool {
	keywords := map[string]bool{}
	var b strings.Builder
	flush := func() {
		token := strings.TrimSpace(strings.ToLower(b.String()))
		b.Reset()
		if token == "" {
			return
		}
		hasKorean := false
		for _, r := range token {
			if (r >= '\uAC00' && r <= '\uD7A3') || (r >= '\u1100' && r <= '\u11FF') {
				hasKorean = true
				break
			}
		}
		if (hasKorean && len([]rune(token)) >= 2) || (!hasKorean && len([]rune(token)) >= 3) {
			keywords[token] = true
		}
	}
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			b.WriteRune(unicode.ToLower(r))
		} else {
			flush()
		}
	}
	flush()
	return keywords
}

func maintenanceTM1cClampImportance(value float64) float64 {
	return clampFloat(maintenanceTM1cNormalizeImportance(value), maintenanceTM1cImportanceFloor, maintenanceTM1cImportanceCeil)
}

func maintenanceTM1cNormalizeImportance(value float64) float64 {
	if value <= 0 {
		return 0
	}
	if value > 1 {
		return value / 10.0
	}
	return value
}

func maintenanceTM1cRound(value float64) float64 {
	return math.Round(value*10000) / 10000
}

func mapFromJSONString(raw string) map[string]any {
	out := map[string]any{}
	if strings.TrimSpace(raw) == "" {
		return out
	}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

func compactNonEmptyStrings(values []string) []string {
	out := []string{}
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func strongestMaintenanceSignalSeverityTM1b(signals []map[string]any) string {
	out := "none"
	for _, signal := range signals {
		switch strings.ToLower(strings.TrimSpace(extractionStringFromAny(signal["severity"]))) {
		case "high":
			return "high"
		case "medium":
			if out != "high" {
				out = "medium"
			}
		case "low":
			if out == "none" {
				out = "low"
			}
		}
	}
	return out
}

func maintenanceConfidenceDeltaTM1b(severity string) float64 {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "high":
		return 0.15
	case "medium":
		return 0.10
	case "low":
		return 0.05
	default:
		return 0
	}
}

func maintenanceSignalTypesTM1b(signals []map[string]any) []string {
	out := []string{}
	for _, signal := range signals {
		if t := strings.TrimSpace(extractionStringFromAny(signal["signal_type"])); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func maintenanceFloatFromAnyTM1b(v any, fallback float64) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case json.Number:
		if f, err := val.Float64(); err == nil {
			return f
		}
	case string:
		if f, err := strconv.ParseFloat(strings.TrimSpace(val), 64); err == nil {
			return f
		}
	}
	return fallback
}

func buildMaintenanceDriftSignals(payload map[string]any, assistantPreview string, recentResponses []any) ([]map[string]any, []map[string]any) {
	signals := []map[string]any{}
	hints := []map[string]any{}

	text := strings.TrimSpace(assistantPreview)
	normalizedText := normalizeMaintenanceComparableText(text)
	if normalizedText == "" {
		return signals, hints
	}
	forbiddenMoves, currentArc := maintenanceDirectiveFields(payload)

	addSignal := func(signalType, severity, evidence string) {
		scene := currentArc
		if scene == "" {
			scene = "unspecified"
		}
		signals = append(signals, map[string]any{
			"signal_type":    signalType,
			"drift_type":     signalType,
			"canonical_name": signalType,
			"scene":          scene,
			"severity":       severity,
			"evidence":       truncateRunes(evidence, 180),
			"authority":      "shadow_diagnostic",
		})
	}
	addHint := func(hintType, suggestion string) {
		hints = append(hints, map[string]any{
			"hint_type":                         hintType,
			"suggestion":                        truncateRunes(suggestion, 220),
			"may_override_current_user_input":   false,
			"requires_future_turn_confirmation": true,
		})
	}
	for _, move := range forbiddenMoves {
		target := normalizeForbiddenMoveTarget(move)
		if len([]rune(target)) < 6 {
			continue
		}
		if strings.Contains(normalizedText, target) {
			addSignal("forbidden_move_conflict", "high", move)
			addHint("suppression_hint", "Do not reinforce this forbidden move; keep current user input authoritative.")
			break
		}
	}
	if currentArc != "" && len([]rune(normalizedText)) >= 40 {
		if !maintenanceTextOverlapsArc(normalizedText, currentArc) {
			addSignal("arc_mismatch", "medium", currentArc)
			addHint("arc_reentry_hint", "Before advancing, re-anchor the next response to the current arc if the user has not changed direction.")
		}
	}
	if repeated := detectMaintenancePatternRepeat(normalizedText, recentResponses); repeated != "" {
		addSignal("pattern_repeat", "medium", repeated)
		addHint("style_variation_hint", "Vary sentence rhythm and avoid repeating the highlighted phrase pattern.")
	}
	return signals, hints
}

func maintenanceDirectiveFields(payload map[string]any) ([]string, string) {
	supervisor := mapFromAny(payload["supervisor_result"])
	if directive := mapFromAny(supervisor["directive"]); len(directive) > 0 {
		supervisor = directive
	}
	director := mapFromAny(supervisor["director"])
	author := mapFromAny(supervisor["story_author"])
	forbidden := []string{}
	for _, item := range asAnySlice(director["forbidden_moves"]) {
		text := strings.TrimSpace(extractionStringFromAny(item))
		if text != "" {
			forbidden = append(forbidden, text)
		}
	}
	currentArc := strings.TrimSpace(extractionStringFromAny(author["current_arc"]))
	if currentArc == "" {
		currentArc = strings.TrimSpace(extractionStringFromAny(supervisor["current_arc"]))
	}
	return forbidden, currentArc
}

func normalizeMaintenanceComparableText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	text = strings.NewReplacer("\n", " ", "\r", " ", "\t", " ").Replace(text)
	return strings.Join(strings.Fields(text), " ")
}

func normalizeForbiddenMoveTarget(move string) string {
	target := normalizeMaintenanceComparableText(move)
	for _, prefix := range []string{"do not ", "don't ", "avoid ", "forbid ", "forbidden "} {
		target = strings.TrimPrefix(target, prefix)
	}
	return strings.TrimSpace(target)
}

func maintenanceTextOverlapsArc(normalizedText, currentArc string) bool {
	arc := normalizeMaintenanceComparableText(currentArc)
	if arc == "" {
		return true
	}
	parts := strings.Fields(arc)
	meaningful := 0
	hits := 0
	for _, part := range parts {
		if len([]rune(part)) < 4 {
			continue
		}
		meaningful++
		if strings.Contains(normalizedText, part) {
			hits++
		}
	}
	return meaningful == 0 || hits > 0
}

func detectMaintenancePatternRepeat(normalizedText string, recentResponses []any) string {
	phrases := maintenanceRepeatedPhrases(normalizedText)
	for phrase, count := range phrases {
		if count >= 2 && len([]rune(phrase)) >= 8 {
			return phrase
		}
	}
	for _, item := range recentResponses {
		other := normalizeMaintenanceComparableText(extractionStringFromAny(item))
		if other == "" || other == normalizedText {
			continue
		}
		for phrase := range phrases {
			if len([]rune(phrase)) >= 8 && strings.Contains(other, phrase) {
				return phrase
			}
		}
	}
	return ""
}

func maintenanceRepeatedPhrases(text string) map[string]int {
	words := strings.Fields(text)
	out := map[string]int{}
	for n := 2; n <= 4; n++ {
		if len(words) < n {
			continue
		}
		for i := 0; i <= len(words)-n; i++ {
			phrase := strings.Join(words[i:i+n], " ")
			out[phrase]++
		}
	}
	return out
}
