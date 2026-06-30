package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
)

const (
	criticArchiveLedgerContractVersion      = "critic_archive_ledger.v1"
	criticArchiveLedgerDebugContractVersion = "critic_archive_ledger_debug.v1"
	criticArchiveLedgerPreviewRoute         = "/critic/archive-ledger/preview"
	criticArchiveLedgerDebugRoute           = "/critic/archive-ledger/debug"
	supersessionResolutionDebugRoute        = "/critic/supersession-resolution/debug"
)

var criticArchiveLedgerLaneOrder = []string{"direct_evidence", "recent_accepted_memory", "active_state_snapshot", "unresolved_pending_thread", "recent_resolution_event"}

type criticArchiveLedgerPreviewRequest struct {
	ChatSessionID          string                            `json:"chat_session_id"`
	TurnIndex              int                               `json:"turn_index"`
	AssistantFinalText     string                            `json:"assistant_final_text"`
	AssistantFinalLanguage string                            `json:"assistant_final_language"`
	StreamingMismatch      string                            `json:"streaming_mismatch"`
	LimitsOverride         criticArchiveLedgerLimitsOverride `json:"limits_override"`
}

type criticArchiveLedgerLimitsOverride struct {
	MaxItemsTotal   *int `json:"max_items_total"`
	MaxItemsPerLane *int `json:"max_items_per_lane"`
	MaxCharsTotal   *int `json:"max_chars_total"`
	MaxCharsPerItem *int `json:"max_chars_per_item"`
	FreshnessTurns  *int `json:"freshness_turns"`
	FreshnessDays   *int `json:"freshness_days"`
}

type criticArchiveLedgerLimits struct {
	MaxItemsTotal   int `json:"max_items_total"`
	MaxItemsPerLane int `json:"max_items_per_lane"`
	MaxCharsTotal   int `json:"max_chars_total"`
	MaxCharsPerItem int `json:"max_chars_per_item"`
	FreshnessTurns  int `json:"freshness_turns"`
	FreshnessDays   int `json:"freshness_days"`
}

type criticArchiveLedgerLanguage struct {
	AssistantFinalLanguage string `json:"assistant_final_language"`
	Source                 string `json:"source"`
	OverrideApplied        bool   `json:"override_applied"`
}

type criticArchiveLedgerSafety struct {
	ReasoningScrubApplied bool   `json:"reasoning_scrub_applied"`
	StreamingMismatch     string `json:"streaming_mismatch"`
	RawArchiveDumpBlocked bool   `json:"raw_archive_dump_blocked"`
	ScrubbedItems         int    `json:"scrubbed_items"`
}

type criticArchiveLedgerItem struct {
	Lane      string         `json:"lane"`
	ID        string         `json:"id"`
	Authority string         `json:"authority"`
	Status    string         `json:"status"`
	Summary   string         `json:"summary"`
	Entities  []string       `json:"entities,omitempty"`
	UpdatedAt string         `json:"updated_at,omitempty"`
	SourceRef map[string]any `json:"source_ref"`
}

type criticArchiveLedgerPreviewResponse struct {
	Status               string                      `json:"status"`
	ContractVersion      string                      `json:"contract_version"`
	SessionID            string                      `json:"session_id"`
	GeneratedAt          string                      `json:"generated_at"`
	RuntimeProfile       string                      `json:"runtime_profile"`
	StoreMode            string                      `json:"store_mode"`
	VectorStatus         string                      `json:"vector_status"`
	Language             criticArchiveLedgerLanguage `json:"language"`
	Limits               criticArchiveLedgerLimits   `json:"limits"`
	Items                []criticArchiveLedgerItem   `json:"items"`
	Counts               map[string]int              `json:"counts"`
	Safety               criticArchiveLedgerSafety   `json:"safety"`
	Degraded             bool                        `json:"degraded"`
	Warnings             []string                    `json:"warnings"`
	Trace                map[string]any              `json:"trace"`
	WriteAttempted       bool                        `json:"write_attempted"`
	VectorWriteAttempted bool                        `json:"vector_write_attempted"`
	LLMCallAttempted     bool                        `json:"llm_call_attempted"`
}

func (s *Server) registerCriticLedgerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST "+criticArchiveLedgerPreviewRoute, s.handleCriticArchiveLedgerPreview)
	mux.HandleFunc("GET "+criticArchiveLedgerDebugRoute, s.handleCriticArchiveLedgerDebug)
	mux.HandleFunc("GET "+supersessionResolutionDebugRoute, s.handleSupersessionResolutionDebug)
}

func (s *Server) handleCriticArchiveLedgerPreview(w http.ResponseWriter, r *http.Request) {
	if !s.Cfg.CriticLedgerPreviewEnabled {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status":             "disabled",
			"contract_version":   criticArchiveLedgerContractVersion,
			"write_attempted":    false,
			"llm_call_attempted": false,
			"error":              "critic ledger preview is disabled",
		})
		return
	}

	var req criticArchiveLedgerPreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBadRequest(w, "invalid JSON body")
		return
	}
	req.ChatSessionID = strings.TrimSpace(req.ChatSessionID)
	if req.ChatSessionID == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}

	resp := s.buildCriticArchiveLedgerPreview(r, req)
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleCriticArchiveLedgerDebug(w http.ResponseWriter, r *http.Request) {
	if !s.Cfg.CriticLedgerPreviewEnabled {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{
			"status":                   "disabled",
			"contract_version":         criticArchiveLedgerDebugContractVersion,
			"preview_contract_version": criticArchiveLedgerContractVersion,
			"route":                    criticArchiveLedgerDebugRoute,
			"preview_route":            criticArchiveLedgerPreviewRoute,
			"read_only":                true,
			"debug_only":               true,
			"write_attempted":          false,
			"vector_write_attempted":   false,
			"llm_call_attempted":       false,
			"error":                    "critic ledger preview is disabled",
		})
		return
	}

	req, ok := criticArchiveLedgerDebugRequestFromQuery(w, r)
	if !ok {
		return
	}
	resp := s.buildCriticArchiveLedgerPreview(r, req)
	writeJSON(w, http.StatusOK, buildCriticArchiveLedgerDebugResponse(resp, s.Cfg.CriticLedgerPreviewEnabled, s.Cfg.CriticLedgerEnabled))
}

func (s *Server) handleSupersessionResolutionDebug(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	sid := strings.TrimSpace(query.Get("chat_session_id"))
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	limit := 50
	if value, ok := criticArchiveLedgerOptionalIntQuery(w, query.Get("limit"), "limit"); !ok {
		return
	} else if value != nil {
		limit = *value
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 200 {
		limit = 200
	}

	records := []map[string]any{}
	warnings := []string{}
	source := "audit_logs"
	if resolver, ok := s.Store.(store.SupersessionResolutionStore); ok {
		items, err := resolver.ListSupersessionResolutions(r.Context(), sid, limit)
		if err != nil {
			warnings = append(warnings, "supersession_resolution_store_unavailable: "+err.Error())
		} else {
			source = "supersession_resolution_store"
			for _, item := range items {
				records = append(records, supersessionResolutionRecordMap(item))
			}
		}
	}
	if len(records) == 0 {
		audits, err := s.Store.ListAuditLogs(r.Context(), sid, "supersession_resolution", limit)
		if err != nil {
			warnings = append(warnings, "audit_logs_unavailable: "+err.Error())
		} else {
			for _, item := range audits {
				records = append(records, supersessionResolutionAuditMap(item))
			}
		}
	}

	status := "ok"
	if len(warnings) > 0 {
		status = "degraded"
	} else if len(records) == 0 {
		status = "empty"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":                 status,
		"contract_version":       store.SupersessionResolutionContractVersion,
		"route":                  supersessionResolutionDebugRoute,
		"session_id":             sid,
		"generated_at":           time.Now().UTC().Format(time.RFC3339),
		"read_only":              true,
		"debug_only":             true,
		"write_attempted":        false,
		"vector_write_attempted": false,
		"llm_call_attempted":     false,
		"afterglow_turns":        store.SupersessionResolutionAfterglowTurns,
		"record_count":           len(records),
		"records":                records,
		"warnings":               warnings,
		"trace": map[string]any{
			"contract_owner":         "22-2",
			"source":                 source,
			"resolution_classes":     []string{"soft_demote", "stale_demote", "close", "supersede", "refine", "reverse"},
			"hard_delete_default":    false,
			"write_attempted":        false,
			"vector_write_attempted": false,
			"llm_call_attempted":     false,
		},
	})
}

func (s *Server) buildCriticArchiveLedgerPreview(r *http.Request, req criticArchiveLedgerPreviewRequest) criticArchiveLedgerPreviewResponse {
	ctx := context.Background()
	if r != nil {
		ctx = r.Context()
	}
	return s.buildCriticArchiveLedgerPreviewWithContext(ctx, req)
}

func (s *Server) buildCriticArchiveLedgerPreviewWithContext(ctx context.Context, req criticArchiveLedgerPreviewRequest) criticArchiveLedgerPreviewResponse {
	limits := criticArchiveLedgerDefaultLimits(s.Cfg.RuntimeProfile)
	limits.applyOverride(req.LimitsOverride)
	builder := &criticArchiveLedgerBuilder{
		limits:       limits,
		remaining:    limits.MaxCharsTotal,
		counts:       map[string]int{},
		sourceCounts: map[string]int{},
		skipped:      []map[string]any{},
	}

	if s.StoreOpenError != nil {
		builder.warn("store_open_error: " + s.StoreOpenError.Error())
	}

	language := criticArchiveLedgerLanguageFromRequest(req)
	streamingMismatch := normalizeLedgerStreamingMismatch(req.StreamingMismatch)
	fromTurn, toTurn := criticArchiveLedgerTurnWindow(req.TurnIndex, limits.FreshnessTurns)
	if ctx == nil {
		ctx = context.Background()
	}

	if evidence, err := s.Store.ListEvidence(ctx, req.ChatSessionID); err != nil {
		builder.warn("direct_evidence_unavailable: " + err.Error())
	} else {
		builder.sourceCounts["direct_evidence_records"] = len(evidence)
		sort.SliceStable(evidence, func(i, j int) bool {
			if evidence[i].SourceTurnStart == evidence[j].SourceTurnStart {
				return evidence[i].ID > evidence[j].ID
			}
			return evidence[i].SourceTurnStart > evidence[j].SourceTurnStart
		})
		for _, item := range evidence {
			if builder.laneCount("direct_evidence") >= limits.MaxItemsPerLane || builder.full() {
				break
			}
			if item.Tombstoned {
				builder.skip("direct_evidence", item.ID, "tombstoned")
				continue
			}
			text := firstNonEmptyLedgerString(item.EvidenceText, item.LineageJSON)
			builder.add("direct_evidence", fmt.Sprintf("direct_evidence_%d", item.ID), "mariadb_canonical", firstNonEmptyLedgerString(item.CaptureVerification, item.ArchiveState, "accepted"), text, item.CreatedAt, map[string]any{
				"type": "direct_evidence",
				"id":   item.ID,
			})
		}
	}

	if memories, err := s.Store.ListMemories(ctx, req.ChatSessionID, fromTurn, toTurn); err != nil {
		builder.warn("memories_unavailable: " + err.Error())
	} else {
		builder.sourceCounts["memories"] = len(memories)
		sort.SliceStable(memories, func(i, j int) bool {
			if memories[i].TurnIndex == memories[j].TurnIndex {
				if memories[i].Importance == memories[j].Importance {
					return memories[i].ID > memories[j].ID
				}
				return memories[i].Importance > memories[j].Importance
			}
			return memories[i].TurnIndex > memories[j].TurnIndex
		})
		for _, item := range memories {
			if builder.laneCount("recent_accepted_memory") >= limits.MaxItemsPerLane || builder.full() {
				break
			}
			text := ledgerSummaryFromJSONOrText(item.SummaryJSON)
			builder.add("recent_accepted_memory", fmt.Sprintf("memory_%d", item.ID), "mariadb_canonical", "accepted", text, item.CreatedAt, map[string]any{
				"type":       "memory",
				"id":         item.ID,
				"turn_index": item.TurnIndex,
			})
		}
	}

	if states, err := s.Store.ListActiveStates(ctx, req.ChatSessionID, ""); err != nil {
		builder.warn("active_states_unavailable: " + err.Error())
	} else {
		builder.sourceCounts["active_states"] = len(states)
		for _, item := range states {
			if builder.laneCount("active_state_snapshot") >= limits.MaxItemsPerLane || builder.full() {
				break
			}
			builder.add("active_state_snapshot", fmt.Sprintf("active_state_%d", item.ID), "mariadb_canonical", "active", item.StateType+": "+item.Content, item.CreatedAt, map[string]any{
				"type":       "active_state",
				"id":         item.ID,
				"state_type": item.StateType,
				"turn_index": item.TurnIndex,
			})
		}
	}

	if layers, err := s.Store.ListCanonicalStateLayers(ctx, req.ChatSessionID, ""); err != nil {
		builder.warn("canonical_state_layers_unavailable: " + err.Error())
	} else {
		builder.sourceCounts["canonical_state_layers"] = len(layers)
		for _, item := range layers {
			if builder.laneCount("active_state_snapshot") >= limits.MaxItemsPerLane || builder.full() {
				break
			}
			builder.add("active_state_snapshot", fmt.Sprintf("canonical_state_layer_%d", item.ID), "mariadb_canonical", "active", item.LayerType+": "+item.Content, item.CreatedAt, map[string]any{
				"type":       "canonical_state_layer",
				"id":         item.ID,
				"layer_type": item.LayerType,
				"turn_index": item.TurnIndex,
			})
		}
	}

	if threads, err := s.Store.ListPendingThreads(ctx, req.ChatSessionID, ""); err != nil {
		builder.warn("pending_threads_unavailable: " + err.Error())
	} else {
		builder.sourceCounts["pending_threads"] = len(threads)
		for _, item := range threads {
			if builder.laneCount("unresolved_pending_thread") >= limits.MaxItemsPerLane || builder.full() {
				break
			}
			if item.Suppressed {
				builder.skip("unresolved_pending_thread", item.ID, "suppressed")
				continue
			}
			text := firstNonEmptyLedgerString(item.Title, item.Description, item.ThreadKey, item.HookMetadataJSON)
			builder.add("unresolved_pending_thread", fmt.Sprintf("pending_thread_%d", item.ID), "mariadb_canonical", firstNonEmptyLedgerString(item.Status, "open"), text, item.UpdatedAt, map[string]any{
				"type":        "pending_thread",
				"id":          item.ID,
				"source_turn": item.SourceTurn,
			})
		}
	}

	if audits, err := s.Store.ListAuditLogs(ctx, req.ChatSessionID, "", limits.MaxItemsPerLane*8); err != nil {
		builder.warn("audit_logs_unavailable: " + err.Error())
	} else {
		builder.sourceCounts["audit_logs"] = len(audits)
		for _, item := range audits {
			if builder.laneCount("recent_resolution_event") >= limits.MaxItemsPerLane || builder.full() {
				break
			}
			if !criticLedgerLooksLikeResolutionEvent(item.EventType, item.Summary) {
				continue
			}
			text := firstNonEmptyLedgerString(item.Summary, item.EventType, item.DetailsJSON)
			builder.add("recent_resolution_event", fmt.Sprintf("audit_%d", item.ID), "mariadb_canonical", "audit", text, item.CreatedAt, map[string]any{
				"type":       "audit_log",
				"id":         item.ID,
				"event_type": item.EventType,
			})
		}
	}

	if feedback, err := s.Store.ListCriticFeedback(ctx, req.ChatSessionID, "", 0); err != nil {
		builder.warn("critic_feedback_unavailable: " + err.Error())
	} else {
		builder.sourceCounts["critic_feedback"] = len(feedback)
		for _, item := range feedback {
			if builder.laneCount("recent_resolution_event") >= limits.MaxItemsPerLane || builder.full() {
				break
			}
			text := firstNonEmptyLedgerString(item.FeedbackNote, item.FeedbackValue)
			builder.add("recent_resolution_event", fmt.Sprintf("critic_feedback_%d", item.ID), "mariadb_canonical", item.FeedbackValue, text, item.CreatedAt, map[string]any{
				"type":        "critic_feedback",
				"id":          item.ID,
				"target_type": item.TargetType,
				"target_id":   item.TargetID,
			})
		}
	}

	status := "ok"
	degraded := len(builder.warnings) > 0
	if degraded {
		status = "degraded"
	} else if len(builder.items) == 0 {
		status = "empty"
	}

	return criticArchiveLedgerPreviewResponse{
		Status:               status,
		ContractVersion:      criticArchiveLedgerContractVersion,
		SessionID:            req.ChatSessionID,
		GeneratedAt:          time.Now().UTC().Format(time.RFC3339),
		RuntimeProfile:       string(s.Cfg.RuntimeProfile),
		StoreMode:            string(s.Cfg.StoreMode),
		VectorStatus:         "not_required",
		Language:             language,
		Limits:               limits,
		Items:                builder.items,
		Counts:               builder.counts,
		Safety:               criticArchiveLedgerSafety{ReasoningScrubApplied: true, StreamingMismatch: streamingMismatch, RawArchiveDumpBlocked: true, ScrubbedItems: builder.scrubbedItems},
		Degraded:             degraded,
		Warnings:             builder.warnings,
		WriteAttempted:       false,
		VectorWriteAttempted: false,
		LLMCallAttempted:     false,
		Trace: map[string]any{
			"contract_owner":         "2.1-2",
			"route":                  criticArchiveLedgerPreviewRoute,
			"debug_route":            criticArchiveLedgerDebugRoute,
			"read_only":              true,
			"write_attempted":        false,
			"vector_write_attempted": false,
			"llm_call_attempted":     false,
			"critic_wiring_enabled":  s.Cfg.CriticLedgerEnabled,
			"turn_window":            map[string]int{"from_turn": fromTurn, "to_turn": toTurn},
			"source_counts":          builder.sourceCounts,
			"skipped":                builder.skipped,
			"selection_policy":       criticArchiveLedgerLaneOrder,
		},
	}
}

func criticArchiveLedgerDebugRequestFromQuery(w http.ResponseWriter, r *http.Request) (criticArchiveLedgerPreviewRequest, bool) {
	query := r.URL.Query()
	req := criticArchiveLedgerPreviewRequest{
		ChatSessionID:          strings.TrimSpace(query.Get("chat_session_id")),
		AssistantFinalText:     strings.TrimSpace(query.Get("assistant_final_text")),
		AssistantFinalLanguage: strings.TrimSpace(query.Get("assistant_final_language")),
		StreamingMismatch:      strings.TrimSpace(query.Get("streaming_mismatch")),
	}
	if req.ChatSessionID == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return req, false
	}

	if value, ok := criticArchiveLedgerOptionalIntQuery(w, query.Get("turn_index"), "turn_index"); !ok {
		return req, false
	} else if value != nil {
		req.TurnIndex = *value
	}
	if value, ok := criticArchiveLedgerOptionalIntQuery(w, query.Get("max_items_total"), "max_items_total"); !ok {
		return req, false
	} else {
		req.LimitsOverride.MaxItemsTotal = value
	}
	if value, ok := criticArchiveLedgerOptionalIntQuery(w, query.Get("max_items_per_lane"), "max_items_per_lane"); !ok {
		return req, false
	} else {
		req.LimitsOverride.MaxItemsPerLane = value
	}
	if value, ok := criticArchiveLedgerOptionalIntQuery(w, query.Get("max_chars_total"), "max_chars_total"); !ok {
		return req, false
	} else {
		req.LimitsOverride.MaxCharsTotal = value
	}
	if value, ok := criticArchiveLedgerOptionalIntQuery(w, query.Get("max_chars_per_item"), "max_chars_per_item"); !ok {
		return req, false
	} else {
		req.LimitsOverride.MaxCharsPerItem = value
	}
	if value, ok := criticArchiveLedgerOptionalIntQuery(w, query.Get("freshness_turns"), "freshness_turns"); !ok {
		return req, false
	} else {
		req.LimitsOverride.FreshnessTurns = value
	}
	if value, ok := criticArchiveLedgerOptionalIntQuery(w, query.Get("freshness_days"), "freshness_days"); !ok {
		return req, false
	} else {
		req.LimitsOverride.FreshnessDays = value
	}
	return req, true
}

func criticArchiveLedgerOptionalIntQuery(w http.ResponseWriter, raw, name string) (*int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		writeBadRequest(w, name+" must be an integer")
		return nil, false
	}
	return &value, true
}

func buildCriticArchiveLedgerDebugResponse(resp criticArchiveLedgerPreviewResponse, previewEnabled, criticWiringEnabled bool) map[string]any {
	missingLanes := []string{}
	for _, lane := range criticArchiveLedgerLaneOrder {
		if resp.Counts[lane] == 0 {
			missingLanes = append(missingLanes, lane)
		}
	}
	trace := map[string]any{
		"contract_owner":         "2.1-3",
		"route":                  criticArchiveLedgerDebugRoute,
		"preview_route":          criticArchiveLedgerPreviewRoute,
		"read_only":              true,
		"debug_only":             true,
		"write_attempted":        false,
		"vector_write_attempted": false,
		"llm_call_attempted":     false,
		"preview_trace":          resp.Trace,
	}
	return map[string]any{
		"status":                   resp.Status,
		"contract_version":         criticArchiveLedgerDebugContractVersion,
		"preview_contract_version": resp.ContractVersion,
		"session_id":               resp.SessionID,
		"generated_at":             resp.GeneratedAt,
		"route":                    criticArchiveLedgerDebugRoute,
		"preview_route":            criticArchiveLedgerPreviewRoute,
		"runtime_profile":          resp.RuntimeProfile,
		"store_mode":               resp.StoreMode,
		"vector_status":            resp.VectorStatus,
		"preview_enabled":          previewEnabled,
		"critic_wiring_enabled":    criticWiringEnabled,
		"read_only":                true,
		"debug_only":               true,
		"write_attempted":          false,
		"vector_write_attempted":   false,
		"llm_call_attempted":       false,
		"dashboard": map[string]any{
			"badge_status":             criticArchiveLedgerDashboardBadge(resp),
			"ledger_status":            resp.Status,
			"item_count":               len(resp.Items),
			"lane_counts":              resp.Counts,
			"missing_lanes":            missingLanes,
			"degraded":                 resp.Degraded,
			"warnings":                 resp.Warnings,
			"safety":                   resp.Safety,
			"limits":                   resp.Limits,
			"language":                 resp.Language,
			"items_preview":            criticArchiveLedgerDashboardItems(resp.Items, 8),
			"raw_archive_dump_blocked": true,
		},
		"trace": trace,
	}
}

func criticArchiveLedgerDashboardBadge(resp criticArchiveLedgerPreviewResponse) string {
	switch resp.Status {
	case "ok":
		return "ready"
	case "degraded":
		return "degraded"
	case "empty":
		return "empty"
	default:
		return "unknown"
	}
}

func criticArchiveLedgerDashboardItems(items []criticArchiveLedgerItem, maxItems int) []map[string]any {
	if maxItems <= 0 || maxItems > len(items) {
		maxItems = len(items)
	}
	out := make([]map[string]any, 0, maxItems)
	for _, item := range items[:maxItems] {
		out = append(out, map[string]any{
			"lane":            item.Lane,
			"id":              item.ID,
			"authority":       item.Authority,
			"status":          item.Status,
			"summary_preview": truncateLedgerText(item.Summary, 160),
			"updated_at":      item.UpdatedAt,
			"source_type":     item.SourceRef["type"],
		})
	}
	return out
}

func supersessionResolutionRecordMap(item store.SupersessionResolutionRecord) map[string]any {
	out := map[string]any{
		"id":               item.ID,
		"created_at":       formatLedgerTime(item.CreatedAt),
		"chat_session_id":  item.ChatSessionID,
		"target_type":      item.TargetType,
		"target_id":        item.TargetID,
		"source_turn":      item.SourceTurn,
		"resolution_class": item.ResolutionClass,
		"new_target_type":  item.NewTargetType,
		"new_target_id":    nullablePositiveInt64(item.NewTargetID),
		"relationship_key": item.RelationshipKey,
		"reason":           item.Reason,
		"source":           item.Source,
	}
	if details := jsonMapFromLedgerString(item.DetailsJSON); len(details) > 0 {
		out["details"] = details
	}
	return out
}

func supersessionResolutionAuditMap(item store.AuditLog) map[string]any {
	out := map[string]any{
		"id":              item.ID,
		"created_at":      formatLedgerTime(item.CreatedAt),
		"chat_session_id": item.ChatSessionID,
		"target_type":     item.TargetType,
		"target_id":       item.TargetID,
		"summary":         item.Summary,
		"source":          item.Source,
	}
	if details := jsonMapFromLedgerString(item.DetailsJSON); len(details) > 0 {
		out["details"] = details
		if value := strings.TrimSpace(fmt.Sprint(details["resolution_class"])); value != "" {
			out["resolution_class"] = value
		}
		if value := strings.TrimSpace(fmt.Sprint(details["source_turn"])); value != "" && value != "<nil>" {
			out["source_turn"] = details["source_turn"]
		}
		if value := strings.TrimSpace(fmt.Sprint(details["relationship_key"])); value != "" && value != "<nil>" {
			out["relationship_key"] = value
		}
		if value := strings.TrimSpace(fmt.Sprint(details["reason"])); value != "" && value != "<nil>" {
			out["reason"] = value
		}
	}
	return out
}

func jsonMapFromLedgerString(raw string) map[string]any {
	out := map[string]any{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &out); err != nil {
		return map[string]any{}
	}
	return out
}

func formatLedgerTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func nullablePositiveInt64(value int64) any {
	if value <= 0 {
		return nil
	}
	return value
}

type criticArchiveLedgerBuilder struct {
	limits        criticArchiveLedgerLimits
	remaining     int
	items         []criticArchiveLedgerItem
	counts        map[string]int
	sourceCounts  map[string]int
	warnings      []string
	skipped       []map[string]any
	scrubbedItems int
}

func (b *criticArchiveLedgerBuilder) add(lane, id, authority, status, summary string, updatedAt time.Time, sourceRef map[string]any) {
	if b.full() {
		b.skip(lane, id, "max_items_total")
		return
	}
	clean, scrubbed := criticLedgerScrubText(summary)
	if scrubbed {
		b.scrubbedItems++
	}
	clean = strings.TrimSpace(clean)
	if clean == "" {
		b.skip(lane, id, "empty_after_scrub")
		return
	}
	clean = truncateLedgerText(clean, b.limits.MaxCharsPerItem)
	if runeLen(clean) > b.remaining {
		if b.remaining <= 0 {
			b.skip(lane, id, "max_chars_total")
			return
		}
		clean = truncateLedgerText(clean, b.remaining)
	}
	if strings.TrimSpace(status) == "" {
		status = "accepted"
	}
	item := criticArchiveLedgerItem{
		Lane:      lane,
		ID:        id,
		Authority: authority,
		Status:    status,
		Summary:   clean,
		SourceRef: sourceRef,
	}
	if !updatedAt.IsZero() {
		item.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	}
	b.items = append(b.items, item)
	b.counts[lane]++
	b.remaining -= runeLen(clean)
}

func (b *criticArchiveLedgerBuilder) laneCount(lane string) int {
	return b.counts[lane]
}

func (b *criticArchiveLedgerBuilder) full() bool {
	return len(b.items) >= b.limits.MaxItemsTotal || b.remaining <= 0
}

func (b *criticArchiveLedgerBuilder) warn(message string) {
	b.warnings = append(b.warnings, message)
}

func (b *criticArchiveLedgerBuilder) skip(lane string, id any, reason string) {
	b.skipped = append(b.skipped, map[string]any{"lane": lane, "id": id, "reason": reason})
}

func criticArchiveLedgerDefaultLimits(profile config.RuntimeProfile) criticArchiveLedgerLimits {
	switch profile {
	case config.RuntimeProfileCoreLite, config.RuntimeProfileClientOnly:
		return criticArchiveLedgerLimits{MaxItemsTotal: 8, MaxItemsPerLane: 3, MaxCharsTotal: 3500, MaxCharsPerItem: 400, FreshnessTurns: 25, FreshnessDays: 14}
	default:
		return criticArchiveLedgerLimits{MaxItemsTotal: 12, MaxItemsPerLane: 4, MaxCharsTotal: 6000, MaxCharsPerItem: 500, FreshnessTurns: 40, FreshnessDays: 21}
	}
}

func (l *criticArchiveLedgerLimits) applyOverride(o criticArchiveLedgerLimitsOverride) {
	l.MaxItemsTotal = clampLedgerOverride(l.MaxItemsTotal, o.MaxItemsTotal, 1, 16)
	l.MaxItemsPerLane = clampLedgerOverride(l.MaxItemsPerLane, o.MaxItemsPerLane, 1, 6)
	l.MaxCharsTotal = clampLedgerOverride(l.MaxCharsTotal, o.MaxCharsTotal, 500, 8000)
	l.MaxCharsPerItem = clampLedgerOverride(l.MaxCharsPerItem, o.MaxCharsPerItem, 120, 700)
	l.FreshnessTurns = clampLedgerOverride(l.FreshnessTurns, o.FreshnessTurns, 1, 80)
	l.FreshnessDays = clampLedgerOverride(l.FreshnessDays, o.FreshnessDays, 1, 45)
}

func clampLedgerOverride(current int, override *int, minVal, maxVal int) int {
	if override == nil {
		return current
	}
	if *override < minVal {
		return minVal
	}
	if *override > maxVal {
		return maxVal
	}
	return *override
}

func criticArchiveLedgerTurnWindow(turnIndex, freshnessTurns int) (int, int) {
	if turnIndex <= 0 {
		return 0, 0
	}
	from := turnIndex - freshnessTurns
	if from < 1 {
		from = 1
	}
	return from, turnIndex
}

func criticArchiveLedgerLanguageFromRequest(req criticArchiveLedgerPreviewRequest) criticArchiveLedgerLanguage {
	if lang := strings.TrimSpace(req.AssistantFinalLanguage); lang != "" {
		return criticArchiveLedgerLanguage{AssistantFinalLanguage: lang, Source: "request_assistant_final_language", OverrideApplied: false}
	}
	if lang := guessLedgerLanguage(req.AssistantFinalText); lang != "" {
		return criticArchiveLedgerLanguage{AssistantFinalLanguage: lang, Source: "assistant_final_text", OverrideApplied: false}
	}
	return criticArchiveLedgerLanguage{AssistantFinalLanguage: "unknown", Source: "unknown_fallback", OverrideApplied: false}
}

func guessLedgerLanguage(text string) string {
	hasLatin := false
	for _, r := range text {
		switch {
		case unicode.In(r, unicode.Hangul):
			return "ko"
		case unicode.In(r, unicode.Hiragana, unicode.Katakana):
			return "ja"
		case unicode.In(r, unicode.Han):
			return "zh"
		case (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z'):
			hasLatin = true
		}
	}
	if hasLatin {
		return "en"
	}
	return ""
}

func normalizeLedgerStreamingMismatch(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "suspected", "confirmed", "unknown":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return "none"
	}
}

func ledgerSummaryFromJSONOrText(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err == nil {
		for _, key := range []string{"turn_summary", "summary", "memory", "text", "content", "description"} {
			if value, ok := obj[key]; ok {
				if s := strings.TrimSpace(fmt.Sprint(value)); s != "" {
					return s
				}
			}
		}
		if entities, ok := obj["entities"]; ok {
			delete(obj, "entities")
			if len(obj) == 0 {
				return fmt.Sprint(entities)
			}
		}
		if compact, err := json.Marshal(obj); err == nil {
			return string(compact)
		}
	}
	return raw
}

func criticLedgerScrubText(raw string) (string, bool) {
	text := raw
	original := text
	for _, pair := range [][2]string{{"<thinking", "</thinking>"}, {"<analysis", "</analysis>"}, {"<reasoning", "</reasoning>"}, {"<scratchpad", "</scratchpad>"}} {
		text = removeLedgerTagBlocks(text, pair[0], pair[1])
	}
	lines := strings.Split(text, "\n")
	kept := lines[:0]
	for _, line := range lines {
		lower := strings.ToLower(strings.TrimSpace(line))
		if strings.HasPrefix(lower, "chain of thought:") ||
			strings.HasPrefix(lower, "hidden chain-of-thought:") ||
			strings.HasPrefix(lower, "scratchpad:") ||
			strings.HasPrefix(lower, "reasoning:") ||
			strings.HasPrefix(lower, "analysis:") {
			continue
		}
		kept = append(kept, line)
	}
	text = strings.TrimSpace(strings.Join(kept, "\n"))
	return text, text != strings.TrimSpace(original)
}

func removeLedgerTagBlocks(text, openPrefix, closeTag string) string {
	for {
		lower := strings.ToLower(text)
		start := strings.Index(lower, openPrefix)
		if start < 0 {
			return text
		}
		closeStart := strings.Index(lower[start:], closeTag)
		if closeStart < 0 {
			return strings.TrimSpace(text[:start])
		}
		end := start + closeStart + len(closeTag)
		text = text[:start] + text[end:]
	}
}

func criticLedgerLooksLikeResolutionEvent(eventType, summary string) bool {
	text := strings.ToLower(eventType + " " + summary)
	for _, marker := range []string{"prune", "merge", "merged", "supersede", "superseded", "closure", "closed", "resolve", "resolved", "dedup", "duplicate", "review"} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func firstNonEmptyLedgerString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func truncateLedgerText(text string, maxRunes int) string {
	if maxRunes <= 0 || runeLen(text) <= maxRunes {
		return text
	}
	runes := []rune(text)
	if maxRunes <= 3 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-3]) + "..."
}

func runeLen(text string) int {
	return len([]rune(text))
}
