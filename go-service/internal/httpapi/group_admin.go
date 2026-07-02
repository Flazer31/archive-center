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

func (s *Server) handleAdminReindex(w http.ResponseWriter, r *http.Request) {
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, "POST /admin/reindex")
		return
	}
	req, ok := decodeAdminAuditBody(w, r)
	if !ok {
		return
	}
	sid := strings.TrimSpace(extractionStringFromAny(req["chat_session_id"]))
	if sid == "" {
		writeBadRequest(w, "chat_session_id is required")
		return
	}
	if completeTurnBoolFromAny(req["background"]) {
		if s.AdminJobs == nil {
			s.AdminJobs = newAdminJobManager()
		}
		job := s.AdminJobs.start("reindex", sid, req, func(ctx context.Context, progress adminJobProgressFunc) (map[string]any, error) {
			return s.runAdminReindexJob(ctx, sid, req, progress)
		})
		job["status"] = "accepted"
		job["job_status"] = "queued"
		job["poll_route"] = "/admin/jobs/" + fmt.Sprint(job["job_id"])
		job["note"] = "reindex is running in the background; poll the job route for progress"
		writeJSON(w, http.StatusAccepted, job)
		return
	}
	maxItems := intFromAny(req["max_items"], 200)
	if maxItems <= 0 {
		maxItems = 200
	}
	if maxItems > 5000 {
		maxItems = 5000
	}
	batchSize := intFromAny(req["batch_size"], 20)
	if batchSize <= 0 {
		batchSize = 20
	}
	if batchSize > 100 {
		batchSize = 100
	}
	force := completeTurnBoolFromAny(req["force"])
	dryRun := completeTurnBoolFromAny(req["dry_run"])
	meta := mapFromAny(req["client_meta"])
	cfg := s.completeTurnExtractionConfig(meta)

	memories, err := s.Store.ListMemories(r.Context(), sid, 0, 0)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeShadowGuard(w, "POST /admin/reindex")
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	allMemories := append([]store.Memory(nil), memories...)
	preIntegrity := s.adminReindexIntegrityReport(r.Context(), sid, allMemories, strings.TrimSpace(cfg.Embedder.Model))
	if len(memories) > maxItems {
		memories = memories[:maxItems]
	}

	processed := 0
	upserted := 0
	skipped := 0
	errorsOut := []string{}
	failedIDs := []int64{}
	skippedIDs := []int64{}
	if !dryRun {
		for i := range memories {
			mem := memories[i]
			processed++
			summary := reindexMemoryDocumentText(mem)
			if summary == "" {
				skipped++
				skippedIDs = append(skippedIDs, mem.ID)
				continue
			}
			embeddingText := strings.TrimSpace(mem.Embedding)
			embeddingModel := strings.TrimSpace(mem.EmbeddingModel)
			if (force || embeddingText == "" || embeddingText == "[]") && cfg.Embedder.hasConfig() {
				emb, model, err := callEmbedding(r.Context(), cfg.Embedder, summary)
				if err != nil {
					errorsOut = append(errorsOut, fmt.Sprintf("memory:%d embedding: %s", mem.ID, err.Error()))
					failedIDs = append(failedIDs, mem.ID)
					skipped++
					continue
				}
				embeddingText = emb
				embeddingModel = model
			}
			embedding := parseFloat32JSONList(embeddingText)
			if len(embedding) == 0 {
				skipped++
				skippedIDs = append(skippedIDs, mem.ID)
				continue
			}
			mem.Embedding = embeddingText
			mem.EmbeddingModel = embeddingModel
			result := artifactSaveResult{VectorStatus: "not_requested"}
			s.upsertMemoryVector(r.Context(), sid, mem.TurnIndex, &mem, summary, embedding, &result)
			if result.VectorsUpserted > 0 {
				upserted += result.VectorsUpserted
			} else {
				skipped++
				if result.VectorStatus != "" && result.VectorStatus != "not_requested" && result.VectorStatus != "ok" {
					errorsOut = append(errorsOut, fmt.Sprintf("memory:%d vector: %s", mem.ID, result.VectorStatus))
					failedIDs = append(failedIDs, mem.ID)
				} else {
					skippedIDs = append(skippedIDs, mem.ID)
				}
			}
		}
	}
	processedBatches := 0
	if processed > 0 {
		processedBatches = (processed + batchSize - 1) / batchSize
	}
	qualityStatus := "not_run"
	if dryRun {
		qualityStatus = "dry_run"
	} else if upserted > 0 {
		qualityStatus = "requires_before_after_report"
	}
	integrityReport := preIntegrity
	var postIntegrity map[string]any
	if !dryRun {
		postIntegrity = s.adminReindexIntegrityReport(r.Context(), sid, allMemories, strings.TrimSpace(cfg.Embedder.Model))
		integrityReport = postIntegrity
	}
	now := time.Now().UTC()
	s.saveAuditLogBestEffort(r.Context(), &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "admin_reindex",
		TargetType:    adminAuditTargetType(sid),
		TargetID:      0,
		Summary:       "Admin reindex requested",
		DetailsJSON: mustCompactJSON(map[string]any{
			"request_keys":             adminAuditRequestKeys(req),
			"dry_run":                  dryRun,
			"force":                    force,
			"batch_size":               batchSize,
			"max_items":                maxItems,
			"candidates":               len(memories),
			"processed":                processed,
			"processed_batches":        processedBatches,
			"upserted":                 upserted,
			"skipped":                  skipped,
			"embedding_model":          strings.TrimSpace(cfg.Embedder.Model),
			"embedding_provider":       strings.TrimSpace(cfg.Embedder.Provider),
			"embedding_configured":     cfg.Embedder.hasConfig(),
			"embedding_missing_fields": cfg.Embedder.missingFields(),
			"failed_ids":               failedIDs,
			"skipped_ids":              skippedIDs,
			"errors":                   errorsOut,
			"integrity_report":         integrityReport,
			"pre_reindex_integrity":    preIntegrity,
			"post_reindex_integrity":   postIntegrity,
			"quality_verification": map[string]any{
				"status":               qualityStatus,
				"required_for_cutover": true,
			},
		}),
		Source:    s.storeWriteSource(),
		CreatedAt: now,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"status":                   "ok",
		"source":                   s.storeWriteSource(),
		"chat_session_id":          sid,
		"mutation_enabled":         true,
		"reindex_executed":         !dryRun && upserted > 0,
		"dry_run":                  dryRun,
		"force":                    force,
		"batch_size":               batchSize,
		"max_items":                maxItems,
		"candidates":               len(memories),
		"processed":                processed,
		"processed_batches":        processedBatches,
		"upserted":                 upserted,
		"skipped":                  skipped,
		"embedding_model":          strings.TrimSpace(cfg.Embedder.Model),
		"embedding_provider":       strings.TrimSpace(cfg.Embedder.Provider),
		"embedding_configured":     cfg.Embedder.hasConfig(),
		"embedding_missing_fields": cfg.Embedder.missingFields(),
		"failed_ids":               failedIDs,
		"skipped_ids":              skippedIDs,
		"errors":                   errorsOut,
		"integrity_report":         integrityReport,
		"pre_reindex_integrity":    preIntegrity,
		"post_reindex_integrity":   postIntegrity,
		"quality_verification": map[string]any{
			"status":                qualityStatus,
			"required_for_cutover":  true,
			"before_after_required": true,
			"report_scope":          "search quality before/after reindex",
		},
		"audit_written": true,
		"changed_at":    now,
		"note":          "reindex rebuilt vector documents for memories that had an embedding or could be embedded with configured settings",
	})
}

func (s *Server) runAdminReindexJob(ctx context.Context, sid string, req map[string]any, progress adminJobProgressFunc) (map[string]any, error) {
	maxItems := intFromAny(req["max_items"], 200)
	if maxItems <= 0 {
		maxItems = 200
	}
	if maxItems > 5000 {
		maxItems = 5000
	}
	batchSize := intFromAny(req["batch_size"], 20)
	if batchSize <= 0 {
		batchSize = 20
	}
	if batchSize > 100 {
		batchSize = 100
	}
	force := completeTurnBoolFromAny(req["force"])
	dryRun := completeTurnBoolFromAny(req["dry_run"])
	meta := mapFromAny(req["client_meta"])
	cfg := s.completeTurnExtractionConfig(meta)

	memories, err := s.Store.ListMemories(ctx, sid, 0, 0)
	if err != nil {
		return nil, err
	}
	allMemories := append([]store.Memory(nil), memories...)
	preIntegrity := s.adminReindexIntegrityReport(ctx, sid, allMemories, strings.TrimSpace(cfg.Embedder.Model))
	if len(memories) > maxItems {
		memories = memories[:maxItems]
	}
	if progress != nil {
		progress(map[string]any{
			"status":             "running",
			"candidate_count":    len(memories),
			"processed":          0,
			"upserted":           0,
			"skipped_count":      0,
			"failed_count":       0,
			"progress_percent":   0,
			"foreground_timeout": false,
			"timeout_policy":     "background_job_detached_from_http_request",
			"integrity_report":   preIntegrity,
		})
	}

	processed := 0
	upserted := 0
	skipped := 0
	errorsOut := []string{}
	failedIDs := []int64{}
	skippedIDs := []int64{}
	if !dryRun {
		for i := range memories {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}
			mem := memories[i]
			processed++
			summary := reindexMemoryDocumentText(mem)
			if summary == "" {
				skipped++
				skippedIDs = append(skippedIDs, mem.ID)
			} else {
				embeddingText := strings.TrimSpace(mem.Embedding)
				embeddingModel := strings.TrimSpace(mem.EmbeddingModel)
				if (force || embeddingText == "" || embeddingText == "[]") && cfg.Embedder.hasConfig() {
					emb, model, err := callEmbedding(ctx, cfg.Embedder, summary)
					if err != nil {
						errorsOut = append(errorsOut, fmt.Sprintf("memory:%d embedding: %s", mem.ID, err.Error()))
						failedIDs = append(failedIDs, mem.ID)
						skipped++
						if progress != nil {
							progress(adminReindexProgress(processed, len(memories), upserted, skipped, failedIDs, skippedIDs, mem.ID, errorsOut))
						}
						continue
					}
					embeddingText = emb
					embeddingModel = model
				}
				embedding := parseFloat32JSONList(embeddingText)
				if len(embedding) == 0 {
					skipped++
					skippedIDs = append(skippedIDs, mem.ID)
				} else {
					mem.Embedding = embeddingText
					mem.EmbeddingModel = embeddingModel
					result := artifactSaveResult{VectorStatus: "not_requested"}
					s.upsertMemoryVector(ctx, sid, mem.TurnIndex, &mem, summary, embedding, &result)
					if result.VectorsUpserted > 0 {
						upserted += result.VectorsUpserted
					} else {
						skipped++
						if result.VectorStatus != "" && result.VectorStatus != "not_requested" && result.VectorStatus != "ok" {
							errorsOut = append(errorsOut, fmt.Sprintf("memory:%d vector: %s", mem.ID, result.VectorStatus))
							failedIDs = append(failedIDs, mem.ID)
						} else {
							skippedIDs = append(skippedIDs, mem.ID)
						}
					}
				}
			}
			if progress != nil {
				progress(adminReindexProgress(processed, len(memories), upserted, skipped, failedIDs, skippedIDs, mem.ID, errorsOut))
			}
		}
	}
	processedBatches := 0
	if processed > 0 {
		processedBatches = (processed + batchSize - 1) / batchSize
	}
	qualityStatus := "not_run"
	if dryRun {
		qualityStatus = "dry_run"
	} else if upserted > 0 {
		qualityStatus = "requires_before_after_report"
	}
	integrityReport := preIntegrity
	var postIntegrity map[string]any
	if !dryRun {
		postIntegrity = s.adminReindexIntegrityReport(ctx, sid, allMemories, strings.TrimSpace(cfg.Embedder.Model))
		integrityReport = postIntegrity
	}
	now := time.Now().UTC()
	s.saveAuditLogBestEffort(ctx, &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "admin_reindex",
		TargetType:    adminAuditTargetType(sid),
		TargetID:      0,
		Summary:       "Admin reindex requested",
		DetailsJSON: mustCompactJSON(map[string]any{
			"background":               true,
			"request_keys":             adminAuditRequestKeys(req),
			"dry_run":                  dryRun,
			"force":                    force,
			"batch_size":               batchSize,
			"max_items":                maxItems,
			"candidates":               len(memories),
			"processed":                processed,
			"processed_batches":        processedBatches,
			"upserted":                 upserted,
			"skipped":                  skipped,
			"embedding_model":          strings.TrimSpace(cfg.Embedder.Model),
			"embedding_provider":       strings.TrimSpace(cfg.Embedder.Provider),
			"embedding_configured":     cfg.Embedder.hasConfig(),
			"embedding_missing_fields": cfg.Embedder.missingFields(),
			"failed_ids":               failedIDs,
			"skipped_ids":              skippedIDs,
			"errors":                   errorsOut,
			"integrity_report":         integrityReport,
			"pre_reindex_integrity":    preIntegrity,
			"post_reindex_integrity":   postIntegrity,
			"quality_verification": map[string]any{
				"status":               qualityStatus,
				"required_for_cutover": true,
			},
		}),
		Source:    s.storeWriteSource(),
		CreatedAt: now,
	})
	result := map[string]any{
		"status":                   "ok",
		"source":                   s.storeWriteSource(),
		"chat_session_id":          sid,
		"mutation_enabled":         true,
		"reindex_executed":         !dryRun && upserted > 0,
		"dry_run":                  dryRun,
		"force":                    force,
		"batch_size":               batchSize,
		"max_items":                maxItems,
		"candidates":               len(memories),
		"processed":                processed,
		"processed_batches":        processedBatches,
		"upserted":                 upserted,
		"skipped":                  skipped,
		"embedding_model":          strings.TrimSpace(cfg.Embedder.Model),
		"embedding_provider":       strings.TrimSpace(cfg.Embedder.Provider),
		"embedding_configured":     cfg.Embedder.hasConfig(),
		"embedding_missing_fields": cfg.Embedder.missingFields(),
		"failed_ids":               failedIDs,
		"skipped_ids":              skippedIDs,
		"errors":                   errorsOut,
		"integrity_report":         integrityReport,
		"pre_reindex_integrity":    preIntegrity,
		"post_reindex_integrity":   postIntegrity,
		"quality_verification": map[string]any{
			"status":                qualityStatus,
			"required_for_cutover":  true,
			"before_after_required": true,
			"report_scope":          "search quality before/after reindex",
		},
		"audit_written": true,
		"changed_at":    now,
		"background":    true,
		"note":          "reindex rebuilt vector documents for memories that had an embedding or could be embedded with configured settings",
	}
	if progress != nil {
		progress(map[string]any{
			"status":           "completed",
			"candidate_count":  len(memories),
			"processed":        processed,
			"upserted":         upserted,
			"skipped_count":    skipped,
			"failed_count":     len(failedIDs),
			"failed_ids":       failedIDs,
			"skipped_ids":      skippedIDs,
			"errors":           errorsOut,
			"integrity_report": integrityReport,
			"progress_percent": 100,
		})
	}
	return result, nil
}

const adminReindexIntegrityPolicyVersion = "29-3.v1"

func (s *Server) adminReindexIntegrityReport(ctx context.Context, sid string, memories []store.Memory, expectedEmbeddingModel string) map[string]any {
	expectedEmbeddingModel = strings.TrimSpace(expectedEmbeddingModel)
	missingEmbeddingIDs := []int64{}
	modelMismatchIDs := []int64{}
	observedModels := map[string]int{}
	for _, mem := range memories {
		model := strings.TrimSpace(mem.EmbeddingModel)
		if model != "" {
			observedModels[model]++
		}
		embedding := parseFloat32JSONList(strings.TrimSpace(mem.Embedding))
		if len(embedding) == 0 {
			if mem.ID > 0 {
				missingEmbeddingIDs = append(missingEmbeddingIDs, mem.ID)
			}
			continue
		}
		if expectedEmbeddingModel != "" && model != expectedEmbeddingModel {
			if mem.ID > 0 {
				modelMismatchIDs = append(modelMismatchIDs, mem.ID)
			}
		}
	}

	vectorConfigured := s != nil && s.Vector != nil && strings.TrimSpace(s.Cfg.ChromaEndpoint) != ""
	vectorStatus := "not_configured"
	vectorCount := 0
	vectorCountKnown := false
	vectorCountErr := ""
	vectorHealth := map[string]any{
		"status": "not_configured",
	}
	if vectorConfigured {
		vectorStatus = "configured"
		health, err := s.Vector.Health(ctx)
		if err != nil {
			vectorStatus = "health_error"
			vectorHealth = map[string]any{
				"status": "error",
				"error":  err.Error(),
			}
		} else {
			if strings.TrimSpace(health.Status) != "" {
				vectorStatus = strings.TrimSpace(health.Status)
			}
			vectorHealth = map[string]any{
				"status":           strings.TrimSpace(health.Status),
				"collection":       strings.TrimSpace(health.Collection),
				"persist_dir":      strings.TrimSpace(health.PersistDir),
				"total_count":      health.TotalCount,
				"project_model":    strings.TrimSpace(health.ProjectModel),
				"model_ready":      health.ModelReady,
				"preflight_issues": append([]string(nil), health.PreflightIssues...),
			}
		}
		count, err := s.Vector.Count(ctx, sid)
		if err != nil {
			vectorCountErr = err.Error()
		} else {
			vectorCount = count
			vectorCountKnown = true
		}
	}

	canonicalCount := len(memories)
	missingVectorEstimate := 0
	extraVectorEstimate := 0
	if vectorCountKnown {
		if canonicalCount > vectorCount {
			missingVectorEstimate = canonicalCount - vectorCount
		} else if vectorCount > canonicalCount {
			extraVectorEstimate = vectorCount - canonicalCount
		}
	}

	reasons := []string{}
	reembedReasons := []string{}
	if !vectorConfigured {
		reasons = append(reasons, "vector_not_configured")
	}
	if vectorCountErr != "" {
		reasons = append(reasons, "vector_count_error")
	}
	if vectorCountKnown && missingVectorEstimate > 0 {
		reasons = append(reasons, "vector_count_below_canonical_memory_count")
	}
	if vectorCountKnown && extraVectorEstimate > 0 {
		reasons = append(reasons, "vector_count_above_canonical_memory_count")
	}
	if len(missingEmbeddingIDs) > 0 {
		reasons = append(reasons, "memory_rows_missing_embedding")
		reembedReasons = append(reembedReasons, "memory_rows_missing_embedding")
	}
	if len(modelMismatchIDs) > 0 {
		reasons = append(reasons, "embedding_model_mismatch")
		reembedReasons = append(reembedReasons, "embedding_model_mismatch")
	}
	projectModel := strings.TrimSpace(stringFromAny(vectorHealth["project_model"]))
	if expectedEmbeddingModel != "" && projectModel != "" && projectModel != expectedEmbeddingModel {
		reasons = append(reasons, "vector_project_model_mismatch")
		reembedReasons = append(reembedReasons, "vector_project_model_mismatch")
	}

	status := "usable"
	if len(reasons) > 0 {
		status = "reindex_recommended"
	}
	if !vectorConfigured {
		status = "vector_not_configured"
	}
	return map[string]any{
		"policy_version":                     adminReindexIntegrityPolicyVersion,
		"status":                             status,
		"chat_session_id":                    sid,
		"canonical_memory_count":             canonicalCount,
		"vector_configured":                  vectorConfigured,
		"vector_status":                      vectorStatus,
		"vector_health":                      vectorHealth,
		"vector_count":                       vectorCount,
		"vector_count_known":                 vectorCountKnown,
		"vector_count_error":                 nilIfEmpty(vectorCountErr),
		"vector_count_matches_canonical":     vectorCountKnown && vectorCount == canonicalCount,
		"missing_vector_count_estimate":      missingVectorEstimate,
		"extra_vector_count_estimate":        extraVectorEstimate,
		"missing_embedding_count":            len(missingEmbeddingIDs),
		"missing_embedding_ids":              missingEmbeddingIDs,
		"expected_embedding_model":           expectedEmbeddingModel,
		"observed_embedding_models":          observedModels,
		"embedding_model_mismatch_count":     len(modelMismatchIDs),
		"embedding_model_mismatch_ids":       modelMismatchIDs,
		"reindex_recommended":                len(reasons) > 0,
		"reindex_reasons":                    reasons,
		"reembed_recommended":                len(reembedReasons) > 0,
		"reembed_reasons":                    reembedReasons,
		"index_usable_for_vector_first_read": vectorConfigured && vectorCountKnown && vectorCount > 0 && len(reasons) == 0,
	}
}

func adminReindexProgress(processed, total, upserted, skipped int, failedIDs, skippedIDs []int64, lastID int64, errorsOut []string) map[string]any {
	return map[string]any{
		"status":           "running",
		"candidate_count":  total,
		"processed":        processed,
		"upserted":         upserted,
		"skipped_count":    skipped,
		"failed_count":     len(failedIDs),
		"failed_ids":       append([]int64{}, failedIDs...),
		"skipped_ids":      append([]int64{}, skippedIDs...),
		"errors":           append([]string{}, errorsOut...),
		"last_processed":   lastID,
		"progress_percent": adminJobProgressPercent(processed, total),
	}
}

func reindexMemoryDocumentText(mem store.Memory) string {
	if searchText := strings.TrimSpace(memorySearchTextFromMemory(mem).Text); searchText != "" {
		return searchText
	}
	return strings.TrimSpace(memorySummaryPreview(mem.SummaryJSON))
}

func (s *Server) handleAdminRescan(w http.ResponseWriter, r *http.Request) {
	if !s.usesShadowWriteStore() {
		writeShadowGuard(w, "POST /admin/rescan")
		return
	}

	var req adminRescanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	sid := strings.TrimSpace(req.ChatSessionID)
	if sid == "" {
		writeBadRequest(w, "chat_session_id is required")
		return
	}
	if req.Background || boolFromAny(req.ClientMeta["background"]) {
		if s.AdminJobs == nil {
			s.AdminJobs = newAdminJobManager()
		}
		jobRequest := map[string]any{
			"chat_session_id": sid,
			"max_items":       req.MaxItems,
			"turn_indices":    req.TurnIndices,
			"client_meta":     req.ClientMeta,
			"dry_run":         req.DryRun,
			"background":      true,
		}
		job := s.AdminJobs.start("rescan", sid, jobRequest, func(ctx context.Context, progress adminJobProgressFunc) (map[string]any, error) {
			bgReq := req
			bgReq.Background = false
			return s.runAdminRescanWithProgress(ctx, sid, bgReq, progress)
		})
		job["status"] = "accepted"
		job["job_status"] = "queued"
		job["poll_route"] = "/admin/jobs/" + fmt.Sprint(job["job_id"])
		job["note"] = "rescan is running in the background; poll the job route for progress"
		writeJSON(w, http.StatusAccepted, job)
		return
	}
	result, err := s.runAdminRescan(r.Context(), sid, req)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	now := time.Now().UTC()
	s.saveAuditLogBestEffort(r.Context(), &store.AuditLog{
		ChatSessionID: sid,
		EventType:     "admin_rescan",
		TargetType:    "session",
		TargetID:      0,
		Summary:       "Admin rescan requested",
		DetailsJSON: mustCompactJSON(map[string]any{
			"dry_run":         req.DryRun,
			"candidate_count": result["candidate_count"],
			"succeeded":       result["succeeded"],
			"failed":          result["failed"],
			"skipped":         result["skipped"],
			"processed_turns": result["processed_turns"],
		}),
		Source:    s.storeWriteSource(),
		CreatedAt: now,
	})
	result["audit_written"] = true
	result["changed_at"] = now
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleAdminSessionMigrate(w http.ResponseWriter, r *http.Request) {
	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	sourceSID := strings.TrimSpace(stringFromMap(req, "source_session_id"))
	targetSID := strings.TrimSpace(stringFromMap(req, "target_session_id"))
	if sourceSID == "" || targetSID == "" {
		writeBadRequest(w, "source_session_id and target_session_id are required")
		return
	}
	if sourceSID == targetSID {
		writeBadRequest(w, "source_session_id and target_session_id must differ")
		return
	}
	dryRun := true
	if raw, ok := req["dry_run"]; ok {
		if b, ok := raw.(bool); ok {
			dryRun = b
		}
	}
	report, err := s.buildSessionMigrationReport(r.Context(), sourceSID, targetSID)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeJSON(w, http.StatusOK, map[string]any{
				"status":            "blocked",
				"code":              "store_not_enabled",
				"detail":            "store_not_enabled",
				"dry_run":           dryRun,
				"source":            s.storeWriteSource(),
				"source_session_id": sourceSID,
				"target_session_id": targetSID,
				"policy_versions":   []string{"sp1a.v1", "sp1b.v1", "sp1c.v1", "sp1d.v1", "sp1e.v1"},
			})
			return
		}
		writeInternalError(w, err.Error())
		return
	}
	report["dry_run"] = dryRun
	report["requested_gate_status"] = strings.TrimSpace(stringFromMap(req, "gate_status"))
	report["requested_gate_reason"] = strings.TrimSpace(stringFromMap(req, "gate_reason"))
	report["manual_first"] = true
	report["operation_policy_version"] = "sp1e.v1"
	report["auto_copy_detection"] = "deferred"

	if dryRun {
		report["status"] = "ok"
		report["code"] = "dry_run_only"
		report["apply_status"] = "dry_run_only"
		writeJSON(w, http.StatusOK, report)
		return
	}
	if report["gate_status"] != "ready" || !strings.EqualFold(strings.TrimSpace(stringFromMap(req, "gate_status")), "ready") {
		report["status"] = "blocked"
		report["code"] = "gate_not_ready"
		report["apply_status"] = "gate_not_ready"
		writeJSON(w, http.StatusOK, report)
		return
	}
	applySummary, err := s.applySessionMigrationReport(r.Context(), sourceSID, targetSID, report)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	for key, value := range applySummary {
		report[key] = value
	}
	report["status"] = "ok"
	report["code"] = "applied"
	report["apply_status"] = "applied"
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) buildSessionMigrationReport(ctx context.Context, sourceSID, targetSID string) (map[string]any, error) {
	sourceLogs, err := s.Store.ListChatLogs(ctx, sourceSID, 0, 0)
	if err != nil {
		return nil, err
	}
	targetLogs, err := s.Store.ListChatLogs(ctx, targetSID, 0, 0)
	if err != nil {
		return nil, err
	}
	sourceEvidence, err := s.Store.ListEvidence(ctx, sourceSID)
	if err != nil {
		return nil, err
	}
	targetEvidence, err := s.Store.ListEvidence(ctx, targetSID)
	if err != nil {
		return nil, err
	}
	sourceMemories, err := s.Store.ListMemories(ctx, sourceSID, 0, 0)
	if err != nil {
		return nil, err
	}
	targetMemories, err := s.Store.ListMemories(ctx, targetSID, 0, 0)
	if err != nil {
		return nil, err
	}
	sourceKG, err := s.Store.ListKGTriples(ctx, sourceSID)
	if err != nil {
		return nil, err
	}
	targetKG, err := s.Store.ListKGTriples(ctx, targetSID)
	if err != nil {
		return nil, err
	}
	sourceCanonical, _ := s.Store.ListCanonicalStateLayers(ctx, sourceSID, "")
	targetCanonical, _ := s.Store.ListCanonicalStateLayers(ctx, targetSID, "")

	targetEvidenceByHash := map[string]store.DirectEvidence{}
	for _, item := range targetEvidence {
		hash := strings.TrimSpace(item.SourceHash)
		if hash != "" {
			targetEvidenceByHash[hash] = item
		}
	}
	duplicateEvidence := 0
	tombstoneMerge := 0
	supersedeMerge := 0
	unresolvedSuperseded := 0
	evidenceApplyCandidates := []store.DirectEvidence{}
	mergeCandidates := []map[string]any{}
	for _, item := range sourceEvidence {
		hash := strings.TrimSpace(item.SourceHash)
		target, duplicate := targetEvidenceByHash[hash]
		if hash != "" && duplicate {
			duplicateEvidence++
			merge := map[string]any{
				"source_hash":             hash,
				"source_record_id":        item.ID,
				"target_record_id":        target.ID,
				"merge_policy":            "sp1c.v1",
				"source_tombstoned":       item.Tombstoned,
				"source_superseded_by_id": item.SupersededByID,
				"action":                  "drop_duplicate",
			}
			if item.Tombstoned && !target.Tombstoned {
				tombstoneMerge++
				merge["action"] = "propagate_tombstone_then_drop_duplicate"
			}
			if item.SupersededByID > 0 {
				if !sessionMigrationHasSourceEvidenceID(sourceEvidence, item.SupersededByID) && !sessionMigrationHasTargetEvidenceID(targetEvidence, item.SupersededByID) {
					unresolvedSuperseded++
					merge["action"] = "block_unresolved_superseded_duplicate"
					merge["unresolved_superseded_by_id"] = item.SupersededByID
				} else {
					supersedeMerge++
					merge["action"] = "propagate_supersede_then_drop_duplicate"
				}
			}
			mergeCandidates = append(mergeCandidates, merge)
			continue
		}
		evidenceApplyCandidates = append(evidenceApplyCandidates, item)
	}

	targetMemoryTurns := map[int]bool{}
	for _, item := range targetMemories {
		targetMemoryTurns[item.TurnIndex] = true
	}
	memoryDuplicateTurns := 0
	for _, item := range sourceMemories {
		if targetMemoryTurns[item.TurnIndex] {
			memoryDuplicateTurns++
		}
	}
	targetKGKeys := map[string]bool{}
	for _, item := range targetKG {
		targetKGKeys[sessionMigrationKGKey(item)] = true
	}
	kgDuplicateRows := 0
	for _, item := range sourceKG {
		if targetKGKeys[sessionMigrationKGKey(item)] {
			kgDuplicateRows++
		}
	}
	canonicalCollisions := 0
	targetCanonicalKeys := map[string]bool{}
	for _, item := range targetCanonical {
		targetCanonicalKeys[sessionMigrationCanonicalKey(item)] = true
	}
	for _, item := range sourceCanonical {
		if targetCanonicalKeys[sessionMigrationCanonicalKey(item)] {
			canonicalCollisions++
		}
	}

	gateStatus := "ready"
	gateReasons := []string{}
	if len(sourceLogs) == 0 && len(sourceEvidence) == 0 && len(sourceMemories) == 0 && len(sourceCanonical) == 0 {
		gateStatus = "blocked"
		gateReasons = append(gateReasons, "source_session_empty")
	}
	if unresolvedSuperseded > 0 {
		gateStatus = "blocked"
		gateReasons = append(gateReasons, "unresolved_superseded_duplicate")
	}
	if len(gateReasons) == 0 {
		gateReasons = append(gateReasons, "source_hash_source_turn_session_origin_gate_ready")
	}
	moveCandidates := len(sourceLogs) + len(sourceMemories) + len(evidenceApplyCandidates) + len(sourceKG)
	return map[string]any{
		"status":                     "ok",
		"source":                     s.storeWriteSource(),
		"source_session_id":          sourceSID,
		"target_session_id":          targetSID,
		"gate_status":                gateStatus,
		"gate_reasons":               gateReasons,
		"policy_versions":            []string{"sp1a.v1", "sp1b.v1", "sp1c.v1", "sp1d.v1", "sp1e.v1"},
		"ingest_gate_policy_version": "sp1b.v1",
		"merge_policy_version":       "sp1c.v1",
		"package_policy_version":     "sp1a.v1",
		"lineage_preserve_fields":    []string{"source_hash", "source_turn", "source_turn_start", "source_turn_end", "turn_anchor", "session_origin", "tombstoned", "superseded_by_id"},
		"dedupe_keys":                []string{"direct_evidence_records.source_hash", "effective_inputs.source_turn", "canonical_state_layers.layer_type+source_turn", "session_origin"},
		"session_origin":             sourceSID,
		"source_counts": map[string]int{
			"chat_logs":               len(sourceLogs),
			"memories":                len(sourceMemories),
			"direct_evidence_records": len(sourceEvidence),
			"kg_triples":              len(sourceKG),
			"canonical_state_layers":  len(sourceCanonical),
		},
		"target_counts": map[string]int{
			"chat_logs":               len(targetLogs),
			"memories":                len(targetMemories),
			"direct_evidence_records": len(targetEvidence),
			"kg_triples":              len(targetKG),
			"canonical_state_layers":  len(targetCanonical),
		},
		"dedupe_report": map[string]any{
			"direct_evidence_duplicate_source_hash": duplicateEvidence,
			"memory_duplicate_source_turn":          memoryDuplicateTurns,
			"kg_duplicate_rows":                     kgDuplicateRows,
			"canonical_layer_collisions":            canonicalCollisions,
			"dropped_direct_evidence_duplicates":    duplicateEvidence,
		},
		"merge_report": map[string]any{
			"tombstone_propagations":       tombstoneMerge,
			"supersede_propagations":       supersedeMerge,
			"unresolved_superseded_blocks": unresolvedSuperseded,
			"candidates":                   mergeCandidates,
		},
		"rebuild_handoff": map[string]any{
			"policy_version":   "sp1d.v1",
			"dirty_event_type": "backfill_import",
			"rebuild_mode":     "selective",
			"start_point":      "next_prepare_turn_fetch",
			"rebuild_targets":  []string{"direct_evidence", "canonical_state", "dense_summary", "sidecar"},
			"runtime_versions": map[string]string{"dirty_matrix": "or1h.v1", "rebuild": "or1i.v1", "stale_serving_guard": "or1j.v1"},
			"canonical_layers": "read_only_handoff",
		},
		"move_candidates":        moveCandidates,
		"moved_rows":             0,
		"source_rows_remaining":  moveCandidates,
		"apply_candidate_counts": map[string]int{"direct_evidence_records": len(evidenceApplyCandidates)},
	}, nil
}

func (s *Server) applySessionMigrationReport(ctx context.Context, sourceSID, targetSID string, report map[string]any) (map[string]any, error) {
	sourceEvidence, err := s.Store.ListEvidence(ctx, sourceSID)
	if err != nil {
		return nil, err
	}
	targetEvidence, err := s.Store.ListEvidence(ctx, targetSID)
	if err != nil {
		return nil, err
	}
	targetByHash := map[string]store.DirectEvidence{}
	for _, item := range targetEvidence {
		if strings.TrimSpace(item.SourceHash) != "" {
			targetByHash[strings.TrimSpace(item.SourceHash)] = item
		}
	}
	moved := 0
	merged := 0
	for _, item := range sourceEvidence {
		hash := strings.TrimSpace(item.SourceHash)
		if target, ok := targetByHash[hash]; hash != "" && ok {
			if mut, ok := s.Store.(store.ExplorerMutationStore); ok && (item.Tombstoned || item.SupersededByID > 0) {
				patch := store.DirectEvidenceExplorerPatch{}
				if item.Tombstoned {
					tombstoned := true
					archiveState := "tombstoned"
					patch.Tombstoned = &tombstoned
					patch.ArchiveState = &archiveState
				}
				if item.SupersededByID > 0 {
					value := int(item.SupersededByID)
					patch.SupersededByID = store.OptionalIntPatch{Set: true, Value: &value}
				}
				if err := mut.UpdateDirectEvidenceExplorerFields(ctx, targetSID, target.ID, patch); err != nil {
					return nil, err
				}
				merged++
			}
			continue
		}
		item.ID = 0
		item.ChatSessionID = targetSID
		item.LineageJSON = sessionMigrationMergeLineage(item.LineageJSON, sourceSID)
		if err := s.Store.SaveEvidence(ctx, &item); err != nil {
			return nil, err
		}
		moved++
	}
	s.saveAuditLogBestEffort(ctx, &store.AuditLog{
		ChatSessionID: targetSID,
		EventType:     "session_migrate",
		TargetType:    "session",
		TargetID:      0,
		Summary:       "Session migration apply completed",
		DetailsJSON:   mustCompactJSON(map[string]any{"source_session_id": sourceSID, "moved_rows": moved, "merged_rows": merged, "policies": report["policy_versions"]}),
		Source:        s.storeWriteSource(),
		CreatedAt:     time.Now().UTC(),
	})
	return map[string]any{
		"moved_rows":            moved,
		"merged_rows":           merged,
		"source_rows_remaining": 0,
		"write_scope":           []string{"direct_evidence_records"},
		"canonical_write_scope": "not_supported_in_current_store_contract",
		"audit_written":         true,
	}, nil
}

func sessionMigrationHasSourceEvidenceID(items []store.DirectEvidence, id int64) bool {
	return sessionMigrationHasEvidenceID(items, id)
}

func sessionMigrationHasTargetEvidenceID(items []store.DirectEvidence, id int64) bool {
	return sessionMigrationHasEvidenceID(items, id)
}

func sessionMigrationHasEvidenceID(items []store.DirectEvidence, id int64) bool {
	if id <= 0 {
		return false
	}
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

func sessionMigrationKGKey(item store.KGTriple) string {
	return strings.Join([]string{item.Subject, item.Predicate, item.Object, strconv.Itoa(item.SourceTurn)}, "\x1f")
}

func sessionMigrationCanonicalKey(item store.CanonicalStateLayer) string {
	return item.LayerType + "\x1f" + strconv.Itoa(item.SourceTurn)
}

func sessionMigrationMergeLineage(raw, sourceSID string) string {
	var lineage map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &lineage); err != nil || lineage == nil {
		lineage = map[string]any{}
	}
	lineage["session_origin"] = sourceSID
	lineage["import_policy_version"] = "sp1b.v1"
	return mustCompactJSON(lineage)
}

func decodeAdminAuditBody(w http.ResponseWriter, r *http.Request) (map[string]any, bool) {
	out := map[string]any{}
	if r.Body == nil {
		return out, true
	}
	err := json.NewDecoder(r.Body).Decode(&out)
	if err == nil {
		return out, true
	}
	if errors.Is(err, io.EOF) {
		return out, true
	}
	writeBadRequest(w, err.Error())
	return nil, false
}

func adminAuditTargetType(sid string) string {
	if strings.TrimSpace(sid) != "" {
		return "session"
	}
	return "global"
}

func adminAuditRequestKeys(req map[string]any) []string {
	keys := make([]string, 0, len(req))
	for key := range req {
		lower := strings.ToLower(key)
		if strings.Contains(lower, "key") || strings.Contains(lower, "secret") || strings.Contains(lower, "password") || strings.Contains(lower, "token") {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

type adminRescanRequest struct {
	ChatSessionID string         `json:"chat_session_id"`
	MaxItems      int            `json:"max_items"`
	TurnIndices   []int          `json:"turn_indices"`
	ClientMeta    map[string]any `json:"client_meta"`
	DryRun        bool           `json:"dry_run"`
	Background    bool           `json:"background"`
}

func (s *Server) runAdminRescan(ctx context.Context, sid string, req adminRescanRequest) (map[string]any, error) {
	return s.runAdminRescanWithProgress(ctx, sid, req, nil)
}

func (s *Server) runAdminRescanWithProgress(ctx context.Context, sid string, req adminRescanRequest, progress adminJobProgressFunc) (map[string]any, error) {
	maxItems := req.MaxItems
	if maxItems <= 0 {
		maxItems = 50
	}
	if maxItems > 1000 {
		maxItems = 1000
	}

	logs, err := s.Store.ListChatLogs(ctx, sid, 0, 0)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	memories, err := s.Store.ListMemories(ctx, sid, 0, 0)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}

	targetTurns := map[int]bool{}
	for _, turn := range req.TurnIndices {
		if turn > 0 {
			targetTurns[turn] = true
		}
	}
	forceWorldRuleBackfill := boolFromAny(req.ClientMeta["force_world_rule_backfill"]) ||
		boolFromAny(req.ClientMeta["force_focused_world_rule_audit"])
	fullSessionBackfill := boolFromAny(req.ClientMeta["full_session_backfill"]) ||
		boolFromAny(req.ClientMeta["session_normalize_full_session_backfill"])
	forceRawWorldRuleAudit := boolFromAny(req.ClientMeta["force_raw_world_rule_audit"]) ||
		boolFromAny(req.ClientMeta["force_focused_world_rule_audit"]) ||
		fullSessionBackfill
	forceDerivedRebuild := boolFromAny(req.ClientMeta["force_derived_rebuild"]) ||
		boolFromAny(req.ClientMeta["derived_backfill_only"]) ||
		forceWorldRuleBackfill ||
		boolFromAny(req.ClientMeta["force_episode_backfill"])
	memoryTurns := map[int]bool{}
	for _, mem := range memories {
		if mem.ChatSessionID == sid && mem.TurnIndex > 0 {
			memoryTurns[mem.TurnIndex] = true
		}
	}
	turnLogs := map[int]map[string]string{}
	for _, log := range logs {
		if log.ChatSessionID != sid || log.TurnIndex <= 0 {
			continue
		}
		if len(targetTurns) > 0 && !targetTurns[log.TurnIndex] {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(log.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		if turnLogs[log.TurnIndex] == nil {
			turnLogs[log.TurnIndex] = map[string]string{}
		}
		turnLogs[log.TurnIndex][role] = appendUniqueTurnRoleText(turnLogs[log.TurnIndex][role], log.Content)
	}

	turns := []int{}
	for turn, roleMap := range turnLogs {
		if memoryTurns[turn] && !forceDerivedRebuild {
			continue
		}
		if strings.TrimSpace(roleMap["user"]) == "" && strings.TrimSpace(roleMap["assistant"]) == "" {
			continue
		}
		turns = append(turns, turn)
	}
	turns = uniqueSortedInts(turns)
	if len(turns) > maxItems {
		turns = turns[:maxItems]
	}
	if progress != nil {
		progress(map[string]any{
			"status":             "running",
			"stage":              "candidate_scan",
			"candidate_count":    len(turns),
			"processed":          0,
			"succeeded":          0,
			"failed_count":       0,
			"skipped_count":      0,
			"processed_turns":    []int{},
			"failed_turns":       []map[string]any{},
			"skipped_turns":      []map[string]any{},
			"progress_percent":   0,
			"foreground_timeout": false,
			"timeout_policy":     "background_job_detached_from_http_request",
		})
	}

	extractionCfg := s.completeTurnExtractionConfig(req.ClientMeta)
	llmTrace := completeTurnLLMConfigTrace(extractionCfg)
	failedTurns := []map[string]any{}
	skippedTurns := []map[string]any{}
	processedTurns := []int{}
	succeeded := 0
	failed := 0
	skipped := 0
	artifactCounts := map[string]int{
		"memories":          0,
		"evidence":          0,
		"kg_triples":        0,
		"character_events":  0,
		"storylines":        0,
		"world_rules":       0,
		"character_states":  0,
		"pending_threads":   0,
		"active_states":     0,
		"entities":          0,
		"trust_states":      0,
		"episode_summaries": 0,
		"chapter_summaries": 0,
		"arc_summaries":     0,
		"saga_digests":      0,
		"vectors_upserted":  0,
	}
	warnings := []string{}
	episodeInterval := normalizedEpisodeInterval(intFromAny(req.ClientMeta["episode_interval_turns"], 0))
	forceEpisodeBackfill := boolFromAny(req.ClientMeta["force_episode_backfill"])
	episodeBackfill := skippedEpisodeBackfillResult(req.DryRun, episodeInterval, forceEpisodeBackfill, "not_run")
	worldRuleBackfill := skippedWorldRuleBackfillResult(req.DryRun, "not_run")
	hierarchyBackfill := skippedHierarchyBackfillResult(req.DryRun, "not_run")
	runBackfills := func(runLogs []store.ChatLog, runMemories []store.Memory, runEvidence []store.DirectEvidence, runTargets map[int]bool) {
		if progress != nil {
			progress(map[string]any{"stage": "episode_backfill", "candidate_count": len(turns)})
		}
		episodeBackfill = s.backfillEpisodeSummariesFromChatLogs(ctx, sid, runLogs, runMemories, runEvidence, episodeInterval, req.DryRun, runTargets, forceEpisodeBackfill)
		artifactCounts["episode_summaries"] += intFromAny(episodeBackfill["generated"], 0)
		if errText := strings.TrimSpace(stringFromMap(episodeBackfill, "error")); errText != "" {
			warnings = append(warnings, "episode_backfill_failed: "+errText)
		}
		if progress != nil {
			progress(map[string]any{"stage": "world_rule_backfill", "episode_backfill": episodeBackfill})
		}
		worldRuleBackfill = s.backfillWorldRulesFromMemories(ctx, sid, runMemories, runTargets, req.DryRun)
		artifactCounts["world_rules"] += intFromAny(worldRuleBackfill["generated"], 0)
		if errText := strings.TrimSpace(stringFromMap(worldRuleBackfill, "error")); errText != "" {
			warnings = append(warnings, "world_rule_backfill_failed: "+errText)
		}
		shouldRunRawWorldAudit := forceWorldRuleBackfill &&
			(forceRawWorldRuleAudit || (artifactCounts["world_rules"] == 0 && intFromAny(worldRuleBackfill["generated"], 0) == 0))
		if shouldRunRawWorldAudit {
			rawWorldRuleBackfill := s.backfillWorldRulesFromChatLogs(ctx, sid, runLogs, runTargets, req.DryRun, extractionCfg.Critic)
			worldRuleBackfill = mergeWorldRuleBackfillResults(worldRuleBackfill, rawWorldRuleBackfill)
			artifactCounts["world_rules"] += intFromAny(rawWorldRuleBackfill["generated"], 0)
			if errText := strings.TrimSpace(stringFromMap(rawWorldRuleBackfill, "error")); errText != "" {
				warnings = append(warnings, "raw_world_rule_backfill_failed: "+errText)
			}
		}
		if progress != nil {
			progress(map[string]any{"stage": "hierarchy_backfill", "episode_backfill": episodeBackfill, "world_rule_backfill": worldRuleBackfill})
		}
		hierarchyBackfill = s.backfillHierarchySummaries(ctx, sid, runLogs, runTargets, req.ClientMeta, req.DryRun)
		artifactCounts["chapter_summaries"] += intFromAny(mapFromAny(hierarchyBackfill["chapter"])["generated"], 0)
		artifactCounts["arc_summaries"] += intFromAny(mapFromAny(hierarchyBackfill["arc"])["generated"], 0)
		artifactCounts["saga_digests"] += intFromAny(mapFromAny(hierarchyBackfill["saga"])["generated"], 0)
		if errText := strings.TrimSpace(stringFromMap(hierarchyBackfill, "error")); errText != "" {
			warnings = append(warnings, "hierarchy_backfill_failed: "+errText)
		}
		if progress != nil {
			progress(map[string]any{"stage": "backfill_done", "episode_backfill": episodeBackfill, "world_rule_backfill": worldRuleBackfill, "hierarchy_backfill": hierarchyBackfill})
		}
	}
	episodeBackfillOnly := boolFromAny(req.ClientMeta["episode_backfill_only"])
	if episodeBackfillOnly {
		runBackfills(logs, memories, nil, targetTurns)
		return map[string]any{
			"status":                "ok",
			"source":                s.storeWriteSource(),
			"chat_session_id":       sid,
			"dry_run":               req.DryRun,
			"episode_backfill_only": true,
			"candidate_count":       0,
			"succeeded":             0,
			"failed":                0,
			"skipped":               0,
			"processed_turns":       []int{},
			"failed_turns":          []map[string]any{},
			"skipped_turns":         []map[string]any{},
			"artifact_counts":       artifactCounts,
			"episode_backfill":      episodeBackfill,
			"world_rule_backfill":   worldRuleBackfill,
			"hierarchy_backfill":    hierarchyBackfill,
			"warnings":              warnings,
			"llm_config_trace":      llmTrace,
			"note":                  "rescan ran episode/world-rule backfill only and did not reprocess Critic-derived artifacts",
		}, nil
	}

	if len(turns) == 0 {
		runBackfills(logs, memories, nil, targetTurns)
		return map[string]any{
			"status":              "ok",
			"source":              s.storeWriteSource(),
			"chat_session_id":     sid,
			"dry_run":             req.DryRun,
			"candidate_count":     0,
			"succeeded":           0,
			"failed":              0,
			"skipped":             0,
			"processed_turns":     []int{},
			"failed_turns":        []map[string]any{},
			"skipped_turns":       []map[string]any{},
			"artifact_counts":     artifactCounts,
			"episode_backfill":    episodeBackfill,
			"world_rule_backfill": worldRuleBackfill,
			"hierarchy_backfill":  hierarchyBackfill,
			"llm_config_trace":    llmTrace,
			"note":                "rescan found no raw chat_log turns missing memory for this session/target set",
		}, nil
	}

	if !extractionCfg.Critic.hasConfig() {
		for _, turn := range turns {
			failedTurns = append(failedTurns, map[string]any{"turn_index": turn, "reason": "critic_config_missing"})
		}
		runBackfills(logs, memories, nil, targetTurns)
		if progress != nil {
			progress(adminRescanProgress(len(turns), len(turns), 0, len(turns), 0, []int{}, failedTurns, []map[string]any{}, artifactCounts, 0, "critic_config_missing"))
		}
		return map[string]any{
			"status":              "ok",
			"source":              s.storeWriteSource(),
			"chat_session_id":     sid,
			"dry_run":             req.DryRun,
			"candidate_count":     len(turns),
			"succeeded":           0,
			"failed":              len(turns),
			"skipped":             0,
			"processed_turns":     []int{},
			"failed_turns":        failedTurns,
			"skipped_turns":       []map[string]any{},
			"artifact_counts":     artifactCounts,
			"episode_backfill":    episodeBackfill,
			"world_rule_backfill": worldRuleBackfill,
			"hierarchy_backfill":  hierarchyBackfill,
			"llm_config_trace":    llmTrace,
			"note":                "rescan needs configured Critic LLM settings before derived Memory/Direct Evidence/KG/state can be regenerated",
		}, nil
	}

	now := time.Now().UTC()
	for _, turn := range turns {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		roleMap := turnLogs[turn]
		userText := sanitizeCriticStorageText(roleMap["user"])
		assistantText := sanitizeCriticStorageText(roleMap["assistant"])
		if strings.TrimSpace(assistantText) == "" {
			failed++
			failedTurns = append(failedTurns, map[string]any{"turn_index": turn, "reason": "assistant_content_missing"})
			if progress != nil {
				progress(adminRescanProgress(len(processedTurns)+failed+skipped, len(turns), succeeded, failed, skipped, processedTurns, failedTurns, skippedTurns, artifactCounts, turn, "assistant_content_missing"))
			}
			continue
		}
		if shouldApplyCompleteTurnOOCGuard(userText, assistantText, nil) || shouldSkipDerivedIngestForSourceAwareGuard(userText, assistantText) {
			skipped++
			skippedTurns = append(skippedTurns, map[string]any{"turn_index": turn, "reason": "source_guard"})
			if progress != nil {
				progress(adminRescanProgress(len(processedTurns)+failed+skipped, len(turns), succeeded, failed, skipped, processedTurns, failedTurns, skippedTurns, artifactCounts, turn, "source_guard"))
			}
			continue
		}
		if req.DryRun {
			skipped++
			skippedTurns = append(skippedTurns, map[string]any{"turn_index": turn, "reason": "dry_run"})
			if progress != nil {
				progress(adminRescanProgress(len(processedTurns)+failed+skipped, len(turns), succeeded, failed, skipped, processedTurns, failedTurns, skippedTurns, artifactCounts, turn, "dry_run"))
			}
			continue
		}
		extraction, trace, err := s.runCompleteTurnCritic(ctx, sid, turn, userText, assistantText, nil, nil, extractionCfg.Critic)
		if err != nil {
			failed++
			failedTurns = append(failedTurns, map[string]any{"turn_index": turn, "reason": "critic_extract_failed: " + err.Error(), "trace": trace})
			if progress != nil {
				progress(adminRescanProgress(len(processedTurns)+failed+skipped, len(turns), succeeded, failed, skipped, processedTurns, failedTurns, skippedTurns, artifactCounts, turn, "critic_extract_failed"))
			}
			continue
		}
		content := strings.TrimSpace(strings.Join([]string{userText, assistantText}, "\n"))
		saveResult := s.saveCriticExtractionArtifacts(ctx, sid, turn, extraction, content, extractionCfg.Embedder, now)
		if saveResult.Errors > 0 {
			failed++
			failedTurns = append(failedTurns, map[string]any{"turn_index": turn, "reason": "artifact_save_failed", "errors": saveResult.ErrorDetails})
			warnings = append(warnings, saveResult.Warnings...)
			if progress != nil {
				progress(adminRescanProgress(len(processedTurns)+failed+skipped, len(turns), succeeded, failed, skipped, processedTurns, failedTurns, skippedTurns, artifactCounts, turn, "artifact_save_failed"))
			}
			continue
		}
		succeeded++
		processedTurns = append(processedTurns, turn)
		artifactCounts["memories"] += saveResult.Memories
		artifactCounts["evidence"] += saveResult.Evidence
		artifactCounts["kg_triples"] += saveResult.KGTriples
		artifactCounts["character_events"] += saveResult.CharacterEvents
		artifactCounts["storylines"] += saveResult.Storylines
		artifactCounts["world_rules"] += saveResult.WorldRules
		artifactCounts["character_states"] += saveResult.CharacterStates
		artifactCounts["pending_threads"] += saveResult.PendingThreads
		artifactCounts["active_states"] += saveResult.ActiveStates
		artifactCounts["entities"] += saveResult.Entities
		artifactCounts["trust_states"] += saveResult.TrustStates
		artifactCounts["vectors_upserted"] += saveResult.VectorsUpserted
		warnings = append(warnings, saveResult.Warnings...)
		if progress != nil {
			progress(adminRescanProgress(len(processedTurns)+failed+skipped, len(turns), succeeded, failed, skipped, processedTurns, failedTurns, skippedTurns, artifactCounts, turn, "saved"))
		}
	}

	backfillTargets := targetTurns
	if fullSessionBackfill {
		backfillTargets = map[int]bool{}
	} else if len(processedTurns) > 0 {
		backfillTargets = intsToSet(processedTurns)
	}
	postLogs := logs
	postMemories := memories
	postEvidence := []store.DirectEvidence(nil)
	if s.Store != nil {
		if listed, err := s.Store.ListChatLogs(ctx, sid, 0, 0); err == nil {
			postLogs = listed
		}
		if listed, err := s.Store.ListMemories(ctx, sid, 0, 0); err == nil {
			postMemories = listed
		}
		if listed, err := s.Store.ListEvidence(ctx, sid); err == nil {
			postEvidence = listed
		}
	}
	runBackfills(postLogs, postMemories, postEvidence, backfillTargets)

	if succeeded > 0 {
		_ = s.Store.SaveAuditLog(ctx, &store.AuditLog{
			ChatSessionID: sid,
			EventType:     "rescan_rebuild",
			TargetType:    "session",
			TargetID:      0,
			Summary:       fmt.Sprintf("Rescan rebuilt derived artifacts for %d turns", succeeded),
			DetailsJSON: mustCompactJSON(map[string]any{
				"processed_turns":     processedTurns,
				"artifact_counts":     artifactCounts,
				"episode_backfill":    episodeBackfill,
				"world_rule_backfill": worldRuleBackfill,
				"hierarchy_backfill":  hierarchyBackfill,
				"failed":              failed,
				"skipped":             skipped,
			}),
			Source:    s.storeWriteSource(),
			CreatedAt: now,
		})
	}

	result := map[string]any{
		"status":              "ok",
		"source":              s.storeWriteSource(),
		"chat_session_id":     sid,
		"dry_run":             req.DryRun,
		"candidate_count":     len(turns),
		"succeeded":           succeeded,
		"failed":              failed,
		"skipped":             skipped,
		"processed_turns":     uniqueSortedInts(processedTurns),
		"failed_turns":        failedTurns,
		"skipped_turns":       skippedTurns,
		"artifact_counts":     artifactCounts,
		"episode_backfill":    episodeBackfill,
		"world_rule_backfill": worldRuleBackfill,
		"hierarchy_backfill":  hierarchyBackfill,
		"warnings":            warnings,
		"llm_config_trace":    llmTrace,
		"note":                "rescan reprocessed raw chat_logs that were missing memory and rebuilt derived artifacts through the configured Critic pipeline",
	}
	if progress != nil {
		progress(map[string]any{
			"status":              "completed",
			"stage":               "completed",
			"candidate_count":     len(turns),
			"processed":           len(processedTurns) + failed + skipped,
			"succeeded":           succeeded,
			"failed_count":        failed,
			"skipped_count":       skipped,
			"processed_turns":     uniqueSortedInts(processedTurns),
			"failed_turns":        failedTurns,
			"skipped_turns":       skippedTurns,
			"artifact_counts":     cloneIntMapAny(artifactCounts),
			"episode_backfill":    episodeBackfill,
			"world_rule_backfill": worldRuleBackfill,
			"hierarchy_backfill":  hierarchyBackfill,
			"progress_percent":    100,
		})
	}
	return result, nil
}

func adminRescanProgress(processed, total, succeeded, failed, skipped int, processedTurns []int, failedTurns, skippedTurns []map[string]any, artifactCounts map[string]int, lastTurn int, lastReason string) map[string]any {
	return map[string]any{
		"status":           "running",
		"stage":            "critic_artifact_rebuild",
		"candidate_count":  total,
		"processed":        processed,
		"succeeded":        succeeded,
		"failed_count":     failed,
		"skipped_count":    skipped,
		"processed_turns":  uniqueSortedInts(processedTurns),
		"failed_turns":     append([]map[string]any{}, failedTurns...),
		"skipped_turns":    append([]map[string]any{}, skippedTurns...),
		"artifact_counts":  cloneIntMapAny(artifactCounts),
		"last_processed":   lastTurn,
		"last_reason":      nilIfEmpty(lastReason),
		"progress_percent": adminJobProgressPercent(processed, total),
	}
}

func cloneIntMapAny(in map[string]int) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func skippedEpisodeBackfillResult(dryRun bool, interval int, force bool, reason string) map[string]any {
	return map[string]any{
		"status":    "skipped",
		"dry_run":   dryRun,
		"interval":  interval,
		"candidate": 0,
		"generated": 0,
		"existing":  0,
		"skipped":   0,
		"force":     force,
		"reason":    reason,
	}
}

func skippedWorldRuleBackfillResult(dryRun bool, reason string) map[string]any {
	return map[string]any{
		"status":    "skipped",
		"dry_run":   dryRun,
		"candidate": 0,
		"generated": 0,
		"existing":  0,
		"skipped":   0,
		"reason":    reason,
	}
}

func skippedHierarchyBackfillResult(dryRun bool, reason string) map[string]any {
	return map[string]any{
		"status":  "skipped",
		"dry_run": dryRun,
		"reason":  reason,
		"chapter": hierarchyLayerBackfillResult(dryRun, reason),
		"arc":     hierarchyLayerBackfillResult(dryRun, reason),
		"saga":    hierarchyLayerBackfillResult(dryRun, reason),
	}
}

func hierarchyLayerBackfillResult(dryRun bool, reason string) map[string]any {
	return map[string]any{
		"status":    "skipped",
		"dry_run":   dryRun,
		"candidate": 0,
		"generated": 0,
		"existing":  0,
		"skipped":   0,
		"blocked":   []map[string]any{},
		"reason":    reason,
	}
}

func (s *Server) backfillHierarchySummaries(ctx context.Context, sid string, logs []store.ChatLog, targetTurns map[int]bool, meta map[string]any, dryRun bool) map[string]any {
	result := skippedHierarchyBackfillResult(dryRun, "not_run")
	result["status"] = "ok"
	result["reason"] = nil
	if s == nil || s.Store == nil {
		result["status"] = "skipped"
		result["reason"] = "store_unavailable"
		return result
	}
	minTurn, maxTurn := chatLogTurnBounds(sid, logs)
	if minTurn <= 0 || maxTurn <= 0 {
		result["status"] = "skipped"
		result["reason"] = "no_chat_logs"
		return result
	}
	chapterInterval := normalizedChapterInterval(intFromAny(meta["chapter_interval_turns"], 0))
	arcInterval := normalizedHierarchyInterval(intFromAny(meta["arc_interval_turns"], 0), 240, chapterInterval, 1200)
	sagaInterval := normalizedHierarchyInterval(intFromAny(meta["saga_interval_turns"], 0), 960, arcInterval, 4800)
	force := boolFromAny(meta["force_hierarchy_backfill"]) || boolFromAny(meta["force_chapter_backfill"]) || boolFromAny(meta["force_arc_backfill"]) || boolFromAny(meta["force_saga_backfill"])

	var chapterResult map[string]any
	if hierarchyBackfillLayerEnabled(meta, "chapter_auto_enabled", true) {
		var err error
		chapterResult, err = s.backfillChapterSummaries(ctx, sid, minTurn, maxTurn, chapterInterval, targetTurns, dryRun, force)
		if err != nil {
			result["status"] = "partial_error"
			result["error"] = err.Error()
		}
	} else {
		chapterResult = hierarchyLayerBackfillResult(dryRun, "chapter_auto_disabled")
		chapterResult["interval"] = chapterInterval
	}
	result["chapter"] = chapterResult

	var arcResult map[string]any
	if hierarchyBackfillLayerEnabled(meta, "arc_auto_enabled", true) {
		var err error
		arcResult, err = s.backfillArcSummaries(ctx, sid, minTurn, maxTurn, arcInterval, targetTurns, dryRun, force)
		if err != nil && result["status"] != "partial_error" {
			result["status"] = "partial_error"
			result["error"] = err.Error()
		}
	} else {
		arcResult = hierarchyLayerBackfillResult(dryRun, "arc_auto_disabled")
		arcResult["interval"] = arcInterval
	}
	result["arc"] = arcResult

	var sagaResult map[string]any
	if hierarchyBackfillLayerEnabled(meta, "saga_auto_enabled", true) {
		var err error
		sagaResult, err = s.backfillSagaDigests(ctx, sid, minTurn, maxTurn, sagaInterval, targetTurns, dryRun, force)
		if err != nil && result["status"] != "partial_error" {
			result["status"] = "partial_error"
			result["error"] = err.Error()
		}
	} else {
		sagaResult = hierarchyLayerBackfillResult(dryRun, "saga_auto_disabled")
		sagaResult["interval"] = sagaInterval
	}
	result["saga"] = sagaResult
	result["chapter_interval_turns"] = chapterInterval
	result["arc_interval_turns"] = arcInterval
	result["saga_interval_turns"] = sagaInterval
	result["range"] = map[string]any{"from_turn": minTurn, "to_turn": maxTurn}
	result["policy"] = "step23_closed_range_hierarchy_backfill"
	return result
}

func hierarchyBackfillLayerEnabled(meta map[string]any, key string, fallback bool) bool {
	if meta == nil {
		return fallback
	}
	if _, ok := meta[key]; !ok {
		return fallback
	}
	return boolFromAny(meta[key])
}

func (s *Server) backfillChapterSummaries(ctx context.Context, sid string, minTurn, maxTurn, interval int, targetTurns map[int]bool, dryRun, force bool) (map[string]any, error) {
	layer := hierarchyLayerBackfillResult(dryRun, "")
	layer["interval"] = interval
	chapterStore, ok := s.Store.(store.ChapterSummaryStore)
	if !ok {
		layer["status"] = "skipped"
		layer["reason"] = "chapter_store_not_available"
		return layer, nil
	}
	for fromTurn := alignHierarchyStart(minTurn, interval); fromTurn <= maxTurn; fromTurn += interval {
		toTurn := fromTurn + interval - 1
		if toTurn > maxTurn {
			addHierarchyBlocked(layer, fromTurn, toTurn, "open_tail_range")
			continue
		}
		if len(targetTurns) > 0 && !turnRangeContainsTargetTurn(fromTurn, toTurn, targetTurns) {
			layer["skipped"] = intFromAny(layer["skipped"], 0) + 1
			continue
		}
		episodes, err := s.Store.ListEpisodeSummaries(ctx, sid, 0, fromTurn, toTurn)
		if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
			return layer, err
		}
		episodes = filterEpisodes(episodes, "", fromTurn, toTurn, 0)
		if !episodeCoverageComplete(episodes, fromTurn, toTurn) {
			addHierarchyBlocked(layer, fromTurn, toTurn, "blocked_missing_episode")
			continue
		}
		layer["candidate"] = intFromAny(layer["candidate"], 0) + 1
		exists, err := chapterSummaryExists(ctx, chapterStore, sid, fromTurn, toTurn)
		if err != nil {
			return layer, err
		}
		if exists {
			layer["existing"] = intFromAny(layer["existing"], 0) + 1
			continue
		}
		if dryRun {
			continue
		}
		chapter, _ := s.buildChapterSummaryForRange(ctx, sid, fromTurn, toTurn, chapterIndexForRange(toTurn, interval), episodes)
		if err := chapterStore.SaveChapterSummary(ctx, &chapter); err != nil {
			return layer, err
		}
		layer["generated"] = intFromAny(layer["generated"], 0) + 1
	}
	layer["status"] = "ok"
	layer["reason"] = nil
	return layer, nil
}

func (s *Server) backfillArcSummaries(ctx context.Context, sid string, minTurn, maxTurn, interval int, targetTurns map[int]bool, dryRun, force bool) (map[string]any, error) {
	layer := hierarchyLayerBackfillResult(dryRun, "")
	layer["interval"] = interval
	arcStore, ok := s.Store.(store.ArcSummaryStore)
	if !ok {
		layer["status"] = "skipped"
		layer["reason"] = "arc_store_not_available"
		return layer, nil
	}
	chapterStore, ok := s.Store.(store.ChapterSummaryStore)
	if !ok {
		layer["status"] = "skipped"
		layer["reason"] = "chapter_store_not_available"
		return layer, nil
	}
	for fromTurn := alignHierarchyStart(minTurn, interval); fromTurn <= maxTurn; fromTurn += interval {
		toTurn := fromTurn + interval - 1
		if toTurn > maxTurn {
			addHierarchyBlocked(layer, fromTurn, toTurn, "open_tail_range")
			continue
		}
		if len(targetTurns) > 0 && !turnRangeContainsTargetTurn(fromTurn, toTurn, targetTurns) {
			layer["skipped"] = intFromAny(layer["skipped"], 0) + 1
			continue
		}
		chapters, err := chapterStore.SearchChapterSummaries(ctx, sid, "", fromTurn, toTurn, 0)
		if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
			return layer, err
		}
		if !chapterCoverageComplete(chapters, fromTurn, toTurn) {
			addHierarchyBlocked(layer, fromTurn, toTurn, "blocked_missing_chapter")
			continue
		}
		layer["candidate"] = intFromAny(layer["candidate"], 0) + 1
		exists, err := arcSummaryExists(ctx, arcStore, sid, fromTurn, toTurn)
		if err != nil {
			return layer, err
		}
		if exists {
			layer["existing"] = intFromAny(layer["existing"], 0) + 1
			continue
		}
		if dryRun {
			continue
		}
		arc, _ := s.buildArcSummaryForRange(ctx, sid, fromTurn, toTurn, hierarchyIndexForRange(toTurn, interval), chapters)
		if err := arcStore.SaveArcSummary(ctx, sid, &arc); err != nil {
			return layer, err
		}
		layer["generated"] = intFromAny(layer["generated"], 0) + 1
	}
	layer["status"] = "ok"
	layer["reason"] = nil
	return layer, nil
}

func (s *Server) backfillSagaDigests(ctx context.Context, sid string, minTurn, maxTurn, interval int, targetTurns map[int]bool, dryRun, force bool) (map[string]any, error) {
	layer := hierarchyLayerBackfillResult(dryRun, "")
	layer["interval"] = interval
	sagaStore, ok := s.Store.(store.SagaDigestStore)
	if !ok {
		layer["status"] = "skipped"
		layer["reason"] = "saga_store_not_available"
		return layer, nil
	}
	arcStore, ok := s.Store.(store.ArcSummaryStore)
	if !ok {
		layer["status"] = "skipped"
		layer["reason"] = "arc_store_not_available"
		return layer, nil
	}
	for fromTurn := alignHierarchyStart(minTurn, interval); fromTurn <= maxTurn; fromTurn += interval {
		toTurn := fromTurn + interval - 1
		if toTurn > maxTurn {
			addHierarchyBlocked(layer, fromTurn, toTurn, "open_tail_range")
			continue
		}
		if len(targetTurns) > 0 && !turnRangeContainsTargetTurn(fromTurn, toTurn, targetTurns) {
			layer["skipped"] = intFromAny(layer["skipped"], 0) + 1
			continue
		}
		arcs, err := arcStore.SearchArcSummaries(ctx, sid, "", fromTurn, toTurn, 0)
		if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
			return layer, err
		}
		if !arcCoverageComplete(arcs, fromTurn, toTurn) {
			addHierarchyBlocked(layer, fromTurn, toTurn, "blocked_missing_arc")
			continue
		}
		layer["candidate"] = intFromAny(layer["candidate"], 0) + 1
		exists, err := sagaDigestExists(ctx, sagaStore, sid, fromTurn, toTurn)
		if err != nil {
			return layer, err
		}
		if exists {
			layer["existing"] = intFromAny(layer["existing"], 0) + 1
			continue
		}
		if dryRun {
			continue
		}
		saga, _ := s.buildSagaDigestForRange(ctx, sid, fromTurn, toTurn, arcs)
		if err := sagaStore.SaveSagaDigest(ctx, sid, &saga); err != nil {
			return layer, err
		}
		layer["generated"] = intFromAny(layer["generated"], 0) + 1
	}
	layer["status"] = "ok"
	layer["reason"] = nil
	return layer, nil
}

func normalizedHierarchyInterval(value, fallback, minValue, maxValue int) int {
	if value <= 0 {
		value = fallback
	}
	if minValue > 0 && value < minValue {
		value = minValue
	}
	if maxValue > 0 && value > maxValue {
		value = maxValue
	}
	return value
}

func alignHierarchyStart(minTurn, interval int) int {
	if minTurn <= 1 {
		return 1
	}
	return ((minTurn-1)/interval)*interval + 1
}

func chatLogTurnBounds(sid string, logs []store.ChatLog) (int, int) {
	minTurn, maxTurn := 0, 0
	for _, log := range logs {
		if log.ChatSessionID != sid || log.TurnIndex <= 0 {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(log.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		if strings.TrimSpace(log.Content) == "" {
			continue
		}
		if minTurn == 0 || log.TurnIndex < minTurn {
			minTurn = log.TurnIndex
		}
		if log.TurnIndex > maxTurn {
			maxTurn = log.TurnIndex
		}
	}
	return minTurn, maxTurn
}

func addHierarchyBlocked(layer map[string]any, fromTurn, toTurn int, reason string) {
	layer["skipped"] = intFromAny(layer["skipped"], 0) + 1
	blocked := []map[string]any{}
	if raw, ok := layer["blocked"].([]map[string]any); ok {
		blocked = append(blocked, raw...)
	}
	blocked = append(blocked, map[string]any{"from_turn": fromTurn, "to_turn": toTurn, "reason": reason})
	layer["blocked"] = blocked
}

func chapterSummaryExists(ctx context.Context, chapterStore store.ChapterSummaryStore, sid string, fromTurn, toTurn int) (bool, error) {
	items, err := chapterStore.SearchChapterSummaries(ctx, sid, "", fromTurn, toTurn, 50)
	if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
		return false, err
	}
	for _, item := range items {
		if item.FromTurn == fromTurn && item.ToTurn == toTurn {
			return true, nil
		}
	}
	return false, nil
}

func arcSummaryExists(ctx context.Context, arcStore store.ArcSummaryStore, sid string, fromTurn, toTurn int) (bool, error) {
	items, err := arcStore.SearchArcSummaries(ctx, sid, "", fromTurn, toTurn, 50)
	if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
		return false, err
	}
	for _, item := range items {
		if item.FromTurn == fromTurn && item.ToTurn == toTurn {
			return true, nil
		}
	}
	return false, nil
}

func sagaDigestExists(ctx context.Context, sagaStore store.SagaDigestStore, sid string, fromTurn, toTurn int) (bool, error) {
	items, err := sagaStore.SearchSagaDigests(ctx, sid, "", fromTurn, toTurn, 50)
	if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
		return false, err
	}
	for _, item := range items {
		if item.FromTurn == fromTurn && item.ToTurn == toTurn {
			return true, nil
		}
	}
	return false, nil
}

func episodeCoverageComplete(items []store.EpisodeSummary, fromTurn, toTurn int) bool {
	ranges := make([]turnRange, 0, len(items))
	for _, item := range items {
		ranges = append(ranges, turnRange{fromTurn: item.FromTurn, toTurn: item.ToTurn})
	}
	return turnRangesCover(ranges, fromTurn, toTurn)
}

func chapterCoverageComplete(items []store.ChapterSummary, fromTurn, toTurn int) bool {
	ranges := make([]turnRange, 0, len(items))
	for _, item := range items {
		ranges = append(ranges, turnRange{fromTurn: item.FromTurn, toTurn: item.ToTurn})
	}
	return turnRangesCover(ranges, fromTurn, toTurn)
}

func arcCoverageComplete(items []store.ArcSummary, fromTurn, toTurn int) bool {
	ranges := make([]turnRange, 0, len(items))
	for _, item := range items {
		ranges = append(ranges, turnRange{fromTurn: item.FromTurn, toTurn: item.ToTurn})
	}
	return turnRangesCover(ranges, fromTurn, toTurn)
}

type turnRange struct {
	fromTurn int
	toTurn   int
}

func turnRangesCover(ranges []turnRange, fromTurn, toTurn int) bool {
	if fromTurn <= 0 || toTurn < fromTurn {
		return false
	}
	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].fromTurn == ranges[j].fromTurn {
			return ranges[i].toTurn < ranges[j].toTurn
		}
		return ranges[i].fromTurn < ranges[j].fromTurn
	})
	next := fromTurn
	for _, item := range ranges {
		if item.fromTurn <= 0 || item.toTurn < item.fromTurn {
			continue
		}
		if item.toTurn < next {
			continue
		}
		if item.fromTurn > next {
			return false
		}
		next = item.toTurn + 1
		if next > toTurn {
			return true
		}
	}
	return next > toTurn
}

func intsToSet(items []int) map[int]bool {
	out := map[int]bool{}
	for _, item := range items {
		if item > 0 {
			out[item] = true
		}
	}
	return out
}

func (s *Server) backfillEpisodeSummariesFromChatLogs(ctx context.Context, sid string, logs []store.ChatLog, memories []store.Memory, evidence []store.DirectEvidence, interval int, dryRun bool, targetTurns map[int]bool, force bool) map[string]any {
	result := map[string]any{
		"status":    "skipped",
		"dry_run":   dryRun,
		"interval":  interval,
		"candidate": 0,
		"generated": 0,
		"existing":  0,
		"skipped":   0,
		"force":     force,
	}
	if s == nil || s.Store == nil {
		result["reason"] = "store_unavailable"
		return result
	}
	episodeStore, ok := s.Store.(store.EpisodeSummaryStore)
	if !ok {
		result["reason"] = "episode_store_not_available"
		return result
	}
	if interval <= 0 {
		interval = normalizedEpisodeInterval(0)
	}
	if memories == nil {
		if listed, err := s.Store.ListMemories(ctx, sid, 0, 0); err == nil {
			memories = listed
		}
	}
	if evidence == nil {
		if listed, err := s.Store.ListEvidence(ctx, sid); err == nil {
			evidence = listed
		}
	}
	minTurn, maxTurn := 0, 0
	for _, log := range logs {
		if log.ChatSessionID != sid || log.TurnIndex <= 0 {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(log.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		if minTurn == 0 || log.TurnIndex < minTurn {
			minTurn = log.TurnIndex
		}
		if log.TurnIndex > maxTurn {
			maxTurn = log.TurnIndex
		}
	}
	if minTurn <= 0 || maxTurn <= 0 {
		result["reason"] = "no_chat_logs"
		return result
	}
	if minTurn > 1 {
		minTurn = ((minTurn-1)/interval)*interval + 1
	}
	candidates := 0
	generated := 0
	existingCount := 0
	skipped := 0
	partialSkipped := 0
	for fromTurn := minTurn; fromTurn <= maxTurn; fromTurn += interval {
		fullToTurn := fromTurn + interval - 1
		if fullToTurn > maxTurn {
			partialSkipped++
			skipped++
			continue
		}
		toTurn := fullToTurn
		if len(targetTurns) > 0 && !turnRangeContainsTargetTurn(fromTurn, toTurn, targetTurns) {
			skipped++
			continue
		}
		chatLogs := filterChatLogsForTurnRange(logs, fromTurn, toTurn, 24)
		rangeMemories := filterMemoriesForTurnRange(memories, sid, fromTurn, toTurn)
		rangeEvidence := filterEvidenceForTurnRange(evidence, sid, fromTurn, toTurn)
		if len(chatLogs) == 0 && len(rangeMemories) == 0 && len(rangeEvidence) == 0 {
			skipped++
			continue
		}
		candidates++
		existing, err := s.Store.ListEpisodeSummaries(ctx, sid, 0, fromTurn, toTurn)
		if err == nil {
			foundExact := false
			for _, item := range existing {
				if item.FromTurn == fromTurn && item.ToTurn == toTurn {
					foundExact = true
					break
				}
			}
			if foundExact {
				if !force {
					existingCount++
					continue
				}
				if !dryRun {
					if deleter, ok := s.Store.(episodeSummaryRangeDeleter); ok {
						if _, err := deleter.DeleteEpisodeSummariesInRange(ctx, sid, fromTurn, toTurn); err != nil {
							result["status"] = "partial_error"
							result["error"] = err.Error()
							return result
						}
					} else {
						existingCount++
						continue
					}
				}
			}
		} else if !errors.Is(err, store.ErrNotEnabled) && !errors.Is(err, store.ErrNotFound) {
			result["status"] = "partial_error"
			result["error"] = err.Error()
			return result
		}
		if dryRun {
			continue
		}
		episode, _ := buildEpisodeSummaryForRangeWithArtifacts(sid, fromTurn, toTurn, chatLogs, rangeMemories, rangeEvidence)
		if err := episodeStore.SaveEpisodeSummary(ctx, &episode); err != nil {
			result["status"] = "partial_error"
			result["error"] = err.Error()
			return result
		}
		generated++
	}
	result["status"] = "ok"
	result["candidate"] = candidates
	result["generated"] = generated
	result["existing"] = existingCount
	result["skipped"] = skipped
	result["partial_skipped"] = partialSkipped
	return result
}

func turnRangeContainsTargetTurn(fromTurn, toTurn int, targetTurns map[int]bool) bool {
	if len(targetTurns) == 0 {
		return true
	}
	for turn := range targetTurns {
		if turn >= fromTurn && turn <= toTurn {
			return true
		}
	}
	return false
}

func filterMemoriesForTurnRange(items []store.Memory, sid string, fromTurn, toTurn int) []store.Memory {
	out := []store.Memory{}
	for _, item := range items {
		if item.ChatSessionID != sid || item.TurnIndex <= 0 {
			continue
		}
		if fromTurn > 0 && item.TurnIndex < fromTurn {
			continue
		}
		if toTurn > 0 && item.TurnIndex > toTurn {
			continue
		}
		out = append(out, item)
	}
	return out
}

func filterEvidenceForTurnRange(items []store.DirectEvidence, sid string, fromTurn, toTurn int) []store.DirectEvidence {
	out := []store.DirectEvidence{}
	for _, item := range items {
		if item.ChatSessionID != sid {
			continue
		}
		start := item.SourceTurnStart
		end := item.SourceTurnEnd
		if start <= 0 {
			start = item.TurnAnchor
		}
		if end <= 0 {
			end = start
		}
		if start <= 0 {
			continue
		}
		if toTurn > 0 && start > toTurn {
			continue
		}
		if fromTurn > 0 && end < fromTurn {
			continue
		}
		out = append(out, item)
	}
	return out
}

func (s *Server) backfillWorldRulesFromMemories(ctx context.Context, sid string, memories []store.Memory, targetTurns map[int]bool, dryRun bool) map[string]any {
	result := map[string]any{
		"status":    "skipped",
		"dry_run":   dryRun,
		"candidate": 0,
		"generated": 0,
		"existing":  0,
		"skipped":   0,
	}
	if s == nil || s.Store == nil {
		result["reason"] = "store_unavailable"
		return result
	}
	saver, ok := s.Store.(worldRuleSaver)
	if !ok {
		result["reason"] = "world_rule_store_not_available"
		return result
	}
	if memories == nil {
		listed, err := s.Store.ListMemories(ctx, sid, 0, 0)
		if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
			result["status"] = "partial_error"
			result["error"] = err.Error()
			return result
		}
		memories = listed
	}
	existingRules, err := s.Store.ListWorldRules(ctx, sid)
	if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
		result["status"] = "partial_error"
		result["error"] = err.Error()
		return result
	}
	seen := map[string]bool{}
	for _, rule := range existingRules {
		seen[worldRuleDedupeSignature(rule.Scope, rule.ScopeName, rule.Key)] = true
	}
	now := time.Now().UTC()
	candidates := 0
	generated := 0
	existing := 0
	skipped := 0
	for _, mem := range memories {
		if mem.ChatSessionID != sid || mem.TurnIndex <= 0 {
			continue
		}
		if len(targetTurns) > 0 && !targetTurns[mem.TurnIndex] {
			continue
		}
		extraction := map[string]any{}
		if err := json.Unmarshal([]byte(mem.SummaryJSON), &extraction); err != nil {
			skipped++
			continue
		}
		for _, raw := range worldRuleItemsForSave(extraction) {
			ruleMap := mapFromAny(raw)
			key := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(ruleMap, "key"), stringFromMap(ruleMap, "name")))
			if key == "" {
				skipped++
				continue
			}
			scope := store.NormalizeWorldRuleScope(extractionFirstNonEmpty(stringFromMap(ruleMap, "scope"), "session"))
			scopeName := stringFromMap(ruleMap, "scope_name")
			sig := worldRuleDedupeSignature(scope, scopeName, key)
			candidates++
			if seen[sig] {
				existing++
				continue
			}
			seen[sig] = true
			if dryRun {
				continue
			}
			err := saver.SaveWorldRule(ctx, &store.WorldRule{
				ChatSessionID: sid,
				Scope:         scope,
				ScopeName:     scopeName,
				Category:      extractionFirstNonEmpty(stringFromMap(ruleMap, "category"), "critic_backfill"),
				Key:           key,
				ValueJSON:     normalizeWorldRuleValueJSON(extractionFirstNonEmpty(stringFromMap(ruleMap, "value"), stringFromMap(ruleMap, "value_json"), mustCompactJSON(ruleMap))),
				Genre:         stringFromMap(ruleMap, "genre"),
				SourceTurn:    mem.TurnIndex,
				CreatedAt:     now,
				UpdatedAt:     now,
			})
			if err != nil {
				result["status"] = "partial_error"
				result["error"] = err.Error()
				return result
			}
			generated++
		}
	}
	result["status"] = "ok"
	result["candidate"] = candidates
	result["generated"] = generated
	result["existing"] = existing
	result["skipped"] = skipped
	return result
}

func (s *Server) backfillWorldRulesFromChatLogs(ctx context.Context, sid string, logs []store.ChatLog, targetTurns map[int]bool, dryRun bool, cfg completeTurnLLMConfig) map[string]any {
	result := map[string]any{
		"status":     "skipped",
		"source":     "raw_chat_logs_world_rule_audit",
		"dry_run":    dryRun,
		"candidate":  0,
		"generated":  0,
		"existing":   0,
		"skipped":    0,
		"audit_runs": 0,
	}
	if s == nil || s.Store == nil {
		result["reason"] = "store_unavailable"
		return result
	}
	if dryRun {
		result["reason"] = "dry_run_no_llm_call"
		return result
	}
	if !cfg.hasConfig() {
		result["reason"] = "critic_config_missing"
		return result
	}
	saver, ok := s.Store.(worldRuleSaver)
	if !ok {
		result["reason"] = "world_rule_store_not_available"
		return result
	}
	if logs == nil {
		listed, err := s.Store.ListChatLogs(ctx, sid, 0, 0)
		if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
			result["status"] = "partial_error"
			result["error"] = err.Error()
			return result
		}
		logs = listed
	}
	chunks := buildWorldRuleAuditChatLogChunks(sid, logs, targetTurns)
	if len(chunks) == 0 {
		result["reason"] = "no_chat_log_chunks"
		return result
	}
	existingRules, err := s.Store.ListWorldRules(ctx, sid)
	if err != nil && !errors.Is(err, store.ErrNotFound) && !errors.Is(err, store.ErrNotEnabled) {
		result["status"] = "partial_error"
		result["error"] = err.Error()
		return result
	}
	seen := map[string]bool{}
	for _, rule := range existingRules {
		seen[worldRuleDedupeSignature(rule.Scope, rule.ScopeName, rule.Key)] = true
	}
	now := time.Now().UTC()
	generated := 0
	existing := 0
	skipped := 0
	candidates := 0
	auditRuns := 0
	traces := []map[string]any{}
	for _, chunk := range chunks {
		select {
		case <-ctx.Done():
			result["status"] = "partial_error"
			result["error"] = ctx.Err().Error()
			return result
		default:
		}
		auditRuns++
		audited, trace := s.runCompleteTurnWorldRuleAudit(
			ctx,
			sid,
			chunk.endTurn,
			"Session-level world rule audit from raw chat logs. Extract only durable rules grounded in the transcript.",
			chunk.text,
			nil,
			nil,
			map[string]any{},
			cfg,
		)
		traces = append(traces, map[string]any{
			"start_turn": chunk.startTurn,
			"end_turn":   chunk.endTurn,
			"trace":      trace,
		})
		if stringFromMap(trace, "status") == "error" {
			skipped++
			continue
		}
		for _, raw := range worldRuleItemsForSave(audited) {
			ruleMap := mapFromAny(raw)
			key := strings.TrimSpace(extractionFirstNonEmpty(stringFromMap(ruleMap, "key"), stringFromMap(ruleMap, "name")))
			if key == "" {
				skipped++
				continue
			}
			scope := store.NormalizeWorldRuleScope(extractionFirstNonEmpty(stringFromMap(ruleMap, "scope"), "session"))
			scopeName := stringFromMap(ruleMap, "scope_name")
			sig := worldRuleDedupeSignature(scope, scopeName, key)
			candidates++
			if seen[sig] {
				existing++
				continue
			}
			seen[sig] = true
			err := saver.SaveWorldRule(ctx, &store.WorldRule{
				ChatSessionID: sid,
				Scope:         scope,
				ScopeName:     scopeName,
				Category:      extractionFirstNonEmpty(stringFromMap(ruleMap, "category"), "raw_chat_audit"),
				Key:           key,
				ValueJSON:     normalizeWorldRuleValueJSON(extractionFirstNonEmpty(stringFromMap(ruleMap, "value"), stringFromMap(ruleMap, "value_json"), mustCompactJSON(ruleMap))),
				Genre:         stringFromMap(ruleMap, "genre"),
				SourceTurn:    chunk.endTurn,
				CreatedAt:     now,
				UpdatedAt:     now,
			})
			if err != nil {
				result["status"] = "partial_error"
				result["error"] = err.Error()
				result["candidate"] = candidates
				result["generated"] = generated
				result["existing"] = existing
				result["skipped"] = skipped
				result["audit_runs"] = auditRuns
				result["audit_trace"] = traces
				return result
			}
			generated++
		}
	}
	result["status"] = "ok"
	result["candidate"] = candidates
	result["generated"] = generated
	result["existing"] = existing
	result["skipped"] = skipped
	result["audit_runs"] = auditRuns
	result["audit_trace"] = traces
	if candidates == 0 {
		result["reason"] = "raw_audit_returned_no_world_rules"
	}
	return result
}

type worldRuleAuditChatLogChunk struct {
	startTurn int
	endTurn   int
	text      string
}

func buildWorldRuleAuditChatLogChunks(sid string, logs []store.ChatLog, targetTurns map[int]bool) []worldRuleAuditChatLogChunk {
	turnLogs := map[int]map[string]string{}
	for _, log := range logs {
		if log.ChatSessionID != sid || log.TurnIndex <= 0 {
			continue
		}
		if len(targetTurns) > 0 && !targetTurns[log.TurnIndex] {
			continue
		}
		role := strings.ToLower(strings.TrimSpace(log.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		if turnLogs[log.TurnIndex] == nil {
			turnLogs[log.TurnIndex] = map[string]string{}
		}
		turnLogs[log.TurnIndex][role] = appendUniqueTurnRoleText(turnLogs[log.TurnIndex][role], log.Content)
	}
	turns := make([]int, 0, len(turnLogs))
	for turn := range turnLogs {
		turns = append(turns, turn)
	}
	sort.Ints(turns)
	chunks := []worldRuleAuditChatLogChunk{}
	var b strings.Builder
	startTurn := 0
	endTurn := 0
	flush := func() {
		text := strings.TrimSpace(b.String())
		if text == "" || startTurn <= 0 || endTurn <= 0 {
			b.Reset()
			startTurn = 0
			endTurn = 0
			return
		}
		chunks = append(chunks, worldRuleAuditChatLogChunk{startTurn: startTurn, endTurn: endTurn, text: text})
		b.Reset()
		startTurn = 0
		endTurn = 0
	}
	for _, turn := range turns {
		roleMap := turnLogs[turn]
		userText := strings.TrimSpace(roleMap["user"])
		assistantText := strings.TrimSpace(roleMap["assistant"])
		if userText == "" && assistantText == "" {
			continue
		}
		var tb strings.Builder
		fmt.Fprintf(&tb, "\n[turn %d]\n", turn)
		if userText != "" {
			tb.WriteString("user: ")
			tb.WriteString(truncateRunes(userText, 1600))
			tb.WriteString("\n")
		}
		if assistantText != "" {
			tb.WriteString("assistant: ")
			tb.WriteString(truncateRunes(assistantText, 3200))
			tb.WriteString("\n")
		}
		turnBlock := tb.String()
		if b.Len() > 0 && (b.Len()+len(turnBlock) > 14000 || endTurn-startTurn >= 11) {
			flush()
		}
		if startTurn == 0 {
			startTurn = turn
		}
		endTurn = turn
		b.WriteString(turnBlock)
	}
	flush()
	return chunks
}

func mergeWorldRuleBackfillResults(primary, raw map[string]any) map[string]any {
	if primary == nil {
		primary = map[string]any{}
	}
	if raw == nil {
		return primary
	}
	out := map[string]any{}
	for k, v := range primary {
		out[k] = v
	}
	out["raw_chat_audit"] = raw
	out["candidate"] = intFromAny(primary["candidate"], 0) + intFromAny(raw["candidate"], 0)
	out["generated"] = intFromAny(primary["generated"], 0) + intFromAny(raw["generated"], 0)
	out["existing"] = intFromAny(primary["existing"], 0) + intFromAny(raw["existing"], 0)
	out["skipped"] = intFromAny(primary["skipped"], 0) + intFromAny(raw["skipped"], 0)
	if intFromAny(raw["generated"], 0) > 0 || intFromAny(raw["candidate"], 0) > 0 {
		out["status"] = raw["status"]
		delete(out, "reason")
	}
	return out
}

func worldRuleDedupeSignature(scope, scopeName, key string) string {
	return strings.ToLower(store.NormalizeWorldRuleScope(scope) + "\x00" + strings.TrimSpace(scopeName) + "\x00" + strings.TrimSpace(key))
}

const (
	maintenanceTM1dPolicyVersion      = "tm1d.v1"
	maintenanceTM1dDirtyMatrixVersion = "or1h.tm1d.v1"
)

func buildTM1dAuditReplayContract(eventType string, passState map[string]any, importanceReweighting map[string]any, turnIndex int) map[string]any {
	var rows []map[string]any
	switch eventType {
	case "drift_detected":
		rows = buildTM1dDriftDirtyMatrixRows(passState, turnIndex)
	case "importance_reevaluation":
		rows = buildTM1dImportanceDirtyMatrixRows(importanceReweighting, turnIndex)
	}
	replay := buildTM1dReplayMeasurements(eventType, passState, importanceReweighting, len(rows))
	return map[string]any{
		"or_phase_dirty_matrix": map[string]any{
			"policy_version": maintenanceTM1dPolicyVersion,
			"matrix_version": maintenanceTM1dDirtyMatrixVersion,
			"event_type":     eventType,
			"row_count":      len(rows),
			"rows":           rows,
		},
		"replay_measurements": replay,
	}
}

func buildTM1dDriftDirtyMatrixRows(passState map[string]any, turnIndex int) []map[string]any {
	var canonicalUpdates []map[string]any
	if rawList, ok := passState["canonical_updates"].([]map[string]any); ok {
		canonicalUpdates = rawList
	} else {
		for _, raw := range asAnySlice(passState["canonical_updates"]) {
			if m, ok := raw.(map[string]any); ok {
				canonicalUpdates = append(canonicalUpdates, m)
			}
		}
	}
	driftSignalTypes := []string{}
	for _, t := range asAnySlice(passState["drift_signal_types"]) {
		if s, ok := t.(string); ok {
			driftSignalTypes = append(driftSignalTypes, s)
		}
	}
	severity := extractionStringFromAny(passState["strongest_signal_severity"])
	sourcePolicyVersion := extractionStringFromAny(passState["version"])
	if sourcePolicyVersion == "" {
		sourcePolicyVersion = "tm1b.shadow.v1"
	}

	rows := []map[string]any{}
	for i, update := range canonicalUpdates {
		if !completeTurnBoolFromAny(update["would_degrade_confidence"]) {
			continue
		}
		layerType := extractionStringFromAny(update["layer_type"])
		if layerType == "" {
			layerType = "canonical_state"
		}
		signalType := "canonical_drift"
		if i < len(driftSignalTypes) {
			signalType = driftSignalTypes[i]
		} else if len(driftSignalTypes) > 0 {
			signalType = driftSignalTypes[0]
		}
		rows = append(rows, map[string]any{
			"event_type":            "drift_detected",
			"turn_index":            turnIndex,
			"source_policy_version": sourcePolicyVersion,
			"or_phase_trigger":      "truth_maintenance_drift",
			"dirty_signal":          signalType,
			"dirty_scope":           layerType,
			"dirty_targets":         tm1dDirtyTargetsForDriftSignal(signalType),
			"replay_metric_refs":    []string{"tm_drift_pass_count", "tm_drift_layer_count", "tm_drift_signal_type_count"},
			"severity":              severity,
		})
	}
	return rows
}

func buildTM1dImportanceDirtyMatrixRows(importanceReweighting map[string]any, turnIndex int) []map[string]any {
	var updates []map[string]any
	if rawList, ok := importanceReweighting["updates"].([]map[string]any); ok {
		updates = rawList
	} else {
		for _, raw := range asAnySlice(importanceReweighting["updates"]) {
			if m, ok := raw.(map[string]any); ok {
				updates = append(updates, m)
			}
		}
	}
	sourcePolicyVersion := extractionStringFromAny(importanceReweighting["policy_version"])
	if sourcePolicyVersion == "" {
		sourcePolicyVersion = "tm1c.v1"
	}

	grouped := map[string]map[string]any{}
	for _, item := range updates {
		memoryID := int64FromMap(item, "memory_id", 0)
		oldImp := maintenanceFloatFromAnyTM1b(item["old_importance"], 0)
		nextImp := maintenanceFloatFromAnyTM1b(item["next_importance"], 0)
		delta := maintenanceTM1cRound(nextImp - oldImp)
		reasons := []string{}
		for _, r := range asAnySlice(item["reasons"]) {
			if s, ok := r.(string); ok {
				reasons = append(reasons, s)
			}
		}
		if len(reasons) == 0 {
			reasons = append(reasons, "importance_changed")
		}
		for _, reason := range reasons {
			if _, ok := grouped[reason]; !ok {
				grouped[reason] = map[string]any{
					"event_type":            "importance_reevaluation",
					"turn_index":            turnIndex,
					"source_policy_version": sourcePolicyVersion,
					"or_phase_trigger":      "truth_maintenance_importance",
					"dirty_signal":          reason,
					"dirty_scope":           "memory_importance",
					"dirty_targets":         tm1dDirtyTargetsForImportanceReason(reason),
					"replay_metric_refs":    tm1dImportanceReplayMetricRefs(reason),
					"affected_memory_ids":   []int64{},
				}
			}
			row := grouped[reason]
			ids := row["affected_memory_ids"].([]int64)
			ids = append(ids, memoryID)
			row["affected_memory_ids"] = ids
			row["importance_delta"] = delta
		}
	}

	rows := []map[string]any{}
	for _, reason := range sortedStringKeys(grouped) {
		row := grouped[reason]
		ids := row["affected_memory_ids"].([]int64)
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		deduped := []int64{}
		seen := map[int64]bool{}
		for _, id := range ids {
			if !seen[id] {
				seen[id] = true
				deduped = append(deduped, id)
			}
		}
		row["affected_memory_ids"] = deduped
		row["affected_count"] = len(deduped)
		delta := maintenanceFloatFromAnyTM1b(row["importance_delta"], 0)
		direction := "stable"
		if delta > 0 {
			direction = "boost"
		} else if delta < 0 {
			direction = "decay"
		}
		row["delta_direction"] = direction
		delete(row, "importance_delta")
		rows = append(rows, row)
	}
	return rows
}

func sortedStringKeys(m map[string]map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func tm1dDirtyTargetsForDriftSignal(signalType string) []string {
	switch signalType {
	case "canonical_relationship_mismatch":
		return []string{"guidance_state", "entity_coprocessor", "narrative_quality", "sidecar_cache"}
	case "canonical_unresolved_thread_mismatch":
		return []string{"guidance_state", "entity_coprocessor", "world_coprocessor", "narrative_quality", "sidecar_cache"}
	case "canonical_scene_archive_mismatch":
		return []string{"guidance_state", "world_coprocessor", "narrative_quality", "sidecar_cache"}
	case "canonical_drift":
		return []string{"guidance_state", "canonical_state", "sidecar_cache"}
	case "memory_conflict":
		return []string{"memory_index", "guidance_state", "canonical_state"}
	case "entity_drift":
		return []string{"entity_index", "guidance_state"}
	case "narrative_drift":
		return []string{"storyline_index", "guidance_state", "director_state"}
	case "sidecar_drift":
		return []string{"sidecar_cache", "guidance_state"}
	default:
		return []string{"guidance_state", "sidecar_cache"}
	}
}

func tm1dDirtyTargetsForImportanceReason(reason string) []string {
	switch reason {
	case "recent_remention_boost":
		return []string{"guidance_state", "entity_coprocessor", "narrative_quality", "sidecar_cache"}
	case "freshness_decay":
		return []string{"guidance_state", "narrative_quality", "sidecar_cache"}
	case "resolved_reference_decay":
		return []string{"guidance_state", "entity_coprocessor", "world_coprocessor", "narrative_quality", "sidecar_cache"}
	case "emotional_decay":
		return []string{"guidance_state", "narrative_quality", "sidecar_cache"}
	case "protected_reference":
		return []string{"guidance_state", "sidecar_cache"}
	default:
		return []string{"guidance_state", "sidecar_cache"}
	}
}

func tm1dImportanceReplayMetricRefs(reason string) []string {
	refs := []string{"tm_importance_pass_count", "tm_importance_updated_count"}
	switch reason {
	case "recent_remention_boost":
		refs = append(refs, "tm_importance_boosted_count")
	case "freshness_decay", "resolved_reference_decay", "emotional_decay":
		refs = append(refs, "tm_importance_decayed_count")
	case "protected_reference":
		refs = append(refs, "tm_importance_protected_count")
	}
	return uniquePreserveOrderStrings(refs)
}

func buildTM1dReplayMeasurements(eventType string, passState map[string]any, importanceReweighting map[string]any, rowCount int) map[string]any {
	switch eventType {
	case "drift_detected":
		driftSignalTypes := []string{}
		for _, t := range asAnySlice(passState["drift_signal_types"]) {
			if s, ok := t.(string); ok {
				driftSignalTypes = append(driftSignalTypes, s)
			}
		}
		return map[string]any{
			"measurement_policy_version": maintenanceTM1dPolicyVersion,
			"tm_drift_pass_count":        1,
			"tm_drift_layer_count":       rowCount,
			"tm_drift_signal_type_count": len(driftSignalTypes),
		}
	case "importance_reevaluation":
		var updates []map[string]any
		if rawList, ok := importanceReweighting["updates"].([]map[string]any); ok {
			updates = rawList
		} else {
			for _, raw := range asAnySlice(importanceReweighting["updates"]) {
				if m, ok := raw.(map[string]any); ok {
					updates = append(updates, m)
				}
			}
		}
		return map[string]any{
			"measurement_policy_version":    maintenanceTM1dPolicyVersion,
			"tm_importance_pass_count":      1,
			"tm_importance_updated_count":   intFromAny(importanceReweighting["updated_count"], len(updates)),
			"tm_importance_boosted_count":   intFromAny(importanceReweighting["boosted_count"], 0),
			"tm_importance_decayed_count":   intFromAny(importanceReweighting["decayed_count"], 0),
			"tm_importance_protected_count": intFromAny(importanceReweighting["protected_count"], 0),
		}
	default:
		return map[string]any{
			"measurement_policy_version": maintenanceTM1dPolicyVersion,
		}
	}
}

func uniquePreserveOrderStrings(values []string) []string {
	ordered := []string{}
	seen := map[string]bool{}
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		ordered = append(ordered, v)
	}
	return ordered
}
