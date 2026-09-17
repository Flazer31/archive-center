package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

const (
	step23ThemeOffscreenContractVersion = "step23_theme_offscreen.v1"
	step23ThemeOffscreenListRoute       = "/step23/theme-offscreen/{chat_session_id}"
	step23ThemeOffscreenCreateRoute     = "/step23/theme-offscreen"
	step23ThemeOffscreenStatusRoute     = "/step23/theme-offscreen/{id}/status"
)

type step23ThemeOffscreenRecordResponse struct {
	ID                     int64     `json:"id"`
	ChatSessionID          string    `json:"chat_session_id"`
	SurfaceType            string    `json:"surface_type"`
	Label                  string    `json:"label"`
	Summary                string    `json:"summary"`
	Status                 string    `json:"status"`
	Confidence             float64   `json:"confidence"`
	ConfidenceLabel        string    `json:"confidence_label,omitempty"`
	SourceKind             string    `json:"source_kind,omitempty"`
	SourceTurnStart        int       `json:"source_turn_start"`
	SourceTurnEnd          int       `json:"source_turn_end"`
	SourceHash             string    `json:"source_hash,omitempty"`
	EvidenceJSON           string    `json:"evidence_json,omitempty"`
	QuietTurns             int       `json:"quiet_turns"`
	LastSeenTurn           int       `json:"last_seen_turn"`
	DormantAfterQuietTurns int       `json:"dormant_after_quiet_turns"`
	ForegroundEligible     bool      `json:"foreground_eligible"`
	ForegroundReasonJSON   string    `json:"foreground_reason_json,omitempty"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

type step23ThemeOffscreenCounts struct {
	Total              int `json:"total"`
	ThemeTrace         int `json:"theme_trace"`
	OffscreenThread    int `json:"offscreen_thread"`
	Active             int `json:"active"`
	Dormant            int `json:"dormant"`
	ForegroundEligible int `json:"foreground_eligible"`
}

type step23ThemeOffscreenTruthBoundary struct {
	SupportOnly                 bool `json:"support_only"`
	CanonicalWorldFactWriter    bool `json:"canonical_world_fact_writer"`
	AlwaysInjected              bool `json:"always_injected"`
	RequiresEvidence            bool `json:"requires_evidence"`
	MayOverrideCurrentUserInput bool `json:"may_override_current_user_input"`
	MayOverrideDirectEvidence   bool `json:"may_override_direct_evidence"`
}

type step23ThemeOffscreenListResponse struct {
	Status          string                               `json:"status"`
	ContractVersion string                               `json:"contract_version"`
	ChatSessionID   string                               `json:"chat_session_id"`
	SurfaceType     string                               `json:"surface_type,omitempty"`
	Records         []step23ThemeOffscreenRecordResponse `json:"records"`
	Counts          step23ThemeOffscreenCounts           `json:"counts"`
	TruthBoundary   step23ThemeOffscreenTruthBoundary    `json:"truth_boundary"`
}

type step23ThemeOffscreenCreateRequest struct {
	ChatSessionID          string  `json:"chat_session_id"`
	SurfaceType            string  `json:"surface_type"`
	Label                  string  `json:"label"`
	Summary                string  `json:"summary"`
	Status                 string  `json:"status"`
	Confidence             float64 `json:"confidence"`
	ConfidenceLabel        string  `json:"confidence_label"`
	SourceKind             string  `json:"source_kind"`
	SourceTurnStart        int     `json:"source_turn_start"`
	SourceTurnEnd          int     `json:"source_turn_end"`
	SourceHash             string  `json:"source_hash"`
	EvidenceJSON           string  `json:"evidence_json"`
	QuietTurns             int     `json:"quiet_turns"`
	LastSeenTurn           int     `json:"last_seen_turn"`
	DormantAfterQuietTurns int     `json:"dormant_after_quiet_turns"`
	ForegroundEligible     bool    `json:"foreground_eligible"`
	ForegroundReasonJSON   string  `json:"foreground_reason_json"`
}

type step23ThemeOffscreenCreateResponse struct {
	Status          string                             `json:"status"`
	ContractVersion string                             `json:"contract_version"`
	Record          step23ThemeOffscreenRecordResponse `json:"record"`
	TruthBoundary   step23ThemeOffscreenTruthBoundary  `json:"truth_boundary"`
}

type step23ThemeOffscreenStatusRequest struct {
	Status     string `json:"status"`
	QuietTurns int    `json:"quiet_turns"`
}

func (s *Server) registerStep23ThemeOffscreenRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET "+step23ThemeOffscreenListRoute, s.handleStep23ThemeOffscreenList)
	mux.HandleFunc("POST "+step23ThemeOffscreenCreateRoute, s.handleStep23ThemeOffscreenCreate)
	mux.HandleFunc("PATCH "+step23ThemeOffscreenStatusRoute, s.handleStep23ThemeOffscreenStatus)
}

func (s *Server) handleStep23ThemeOffscreenList(w http.ResponseWriter, r *http.Request) {
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	surfaceType := step23NormalizeThemeOffscreenSurfaceType(r.URL.Query().Get("surface_type"))
	limit := step23ClampInt(step23IntQuery(r.URL.Query().Get("limit"), 100), 1, 1000)

	ts, ok := s.Store.(store.ThemeOffscreenCarryStore)
	if !ok {
		writeJSON(w, http.StatusOK, step23ThemeOffscreenListResponse{
			Status:          "ok",
			ContractVersion: step23ThemeOffscreenContractVersion,
			ChatSessionID:   sid,
			SurfaceType:     surfaceType,
			Records:         []step23ThemeOffscreenRecordResponse{},
			TruthBoundary:   step23ThemeOffscreenTruthBoundaryValue(),
		})
		return
	}
	records, err := ts.ListThemeOffscreenCarries(r.Context(), sid, surfaceType, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	resp := step23ThemeOffscreenListResponse{
		Status:          "ok",
		ContractVersion: step23ThemeOffscreenContractVersion,
		ChatSessionID:   sid,
		SurfaceType:     surfaceType,
		Records:         make([]step23ThemeOffscreenRecordResponse, 0, len(records)),
		TruthBoundary:   step23ThemeOffscreenTruthBoundaryValue(),
	}
	for _, record := range records {
		resp.Records = append(resp.Records, step23ThemeOffscreenFromStore(record))
		resp.Counts.Total++
		switch record.SurfaceType {
		case "theme_trace":
			resp.Counts.ThemeTrace++
		case "offscreen_thread":
			resp.Counts.OffscreenThread++
		}
		switch record.Status {
		case "active":
			resp.Counts.Active++
		case "dormant":
			resp.Counts.Dormant++
		}
		if record.ForegroundEligible {
			resp.Counts.ForegroundEligible++
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleStep23ThemeOffscreenCreate(w http.ResponseWriter, r *http.Request) {
	ts, ok := s.Store.(store.ThemeOffscreenCarryStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "store_unavailable", "theme/offscreen store is not available")
		return
	}
	var req step23ThemeOffscreenCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if err := step23ValidateThemeOffscreenCreate(req); err != nil {
		writeError(w, http.StatusBadRequest, CodeMissingParam, err.Error())
		return
	}
	status := step23ApplyThemeOffscreenDormancy(step23NormalizeThemeOffscreenStatus(req.Status), req.QuietTurns, req.DormantAfterQuietTurns)
	record := store.ThemeOffscreenCarryRecord{
		ChatSessionID:          strings.TrimSpace(req.ChatSessionID),
		SurfaceType:            step23NormalizeThemeOffscreenSurfaceType(req.SurfaceType),
		Label:                  strings.TrimSpace(req.Label),
		Summary:                strings.TrimSpace(req.Summary),
		Status:                 status,
		Confidence:             step23ClampConfidence(req.Confidence),
		ConfidenceLabel:        step23NormalizePsychologyConfidenceLabel(req.ConfidenceLabel, req.Confidence),
		SourceKind:             strings.TrimSpace(req.SourceKind),
		SourceTurnStart:        req.SourceTurnStart,
		SourceTurnEnd:          req.SourceTurnEnd,
		SourceHash:             strings.TrimSpace(req.SourceHash),
		EvidenceJSON:           strings.TrimSpace(req.EvidenceJSON),
		QuietTurns:             req.QuietTurns,
		LastSeenTurn:           req.LastSeenTurn,
		DormantAfterQuietTurns: step23DefaultDormantAfterQuietTurns(req.DormantAfterQuietTurns),
		ForegroundEligible:     req.ForegroundEligible,
		ForegroundReasonJSON:   strings.TrimSpace(req.ForegroundReasonJSON),
	}
	saved, err := ts.SaveThemeOffscreenCarry(r.Context(), record)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, step23ThemeOffscreenCreateResponse{
		Status:          "ok",
		ContractVersion: step23ThemeOffscreenContractVersion,
		Record:          step23ThemeOffscreenFromStore(saved),
		TruthBoundary:   step23ThemeOffscreenTruthBoundaryValue(),
	})
}

func (s *Server) handleStep23ThemeOffscreenStatus(w http.ResponseWriter, r *http.Request) {
	ts, ok := s.Store.(store.ThemeOffscreenCarryStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "store_unavailable", "theme/offscreen store is not available")
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("id")), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "valid id is required")
		return
	}
	var req step23ThemeOffscreenStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if req.QuietTurns < 0 {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "quiet_turns must be non-negative")
		return
	}
	status := step23NormalizeThemeOffscreenStatus(req.Status)
	if err := ts.UpdateThemeOffscreenCarryStatus(r.Context(), id, status, req.QuietTurns); err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"contract_version": step23ThemeOffscreenContractVersion,
		"id":               id,
		"surface_status":   status,
		"truth_boundary":   step23ThemeOffscreenTruthBoundaryValue(),
	})
}

func step23ValidateThemeOffscreenCreate(req step23ThemeOffscreenCreateRequest) error {
	if strings.TrimSpace(req.ChatSessionID) == "" {
		return errors.New("chat_session_id is required")
	}
	if step23NormalizeThemeOffscreenSurfaceType(req.SurfaceType) == "" {
		return errors.New("surface_type must be theme_trace or offscreen_thread")
	}
	if strings.TrimSpace(req.Label) == "" {
		return errors.New("label is required")
	}
	if strings.TrimSpace(req.Summary) == "" {
		return errors.New("summary is required")
	}
	if req.SourceTurnStart < 0 || req.SourceTurnEnd < 0 {
		return errors.New("source_turn_start and source_turn_end must be non-negative")
	}
	if req.SourceTurnStart > req.SourceTurnEnd {
		return errors.New("source_turn_start must not exceed source_turn_end")
	}
	if strings.TrimSpace(req.SourceHash) == "" && strings.TrimSpace(req.EvidenceJSON) == "" {
		return errors.New("source_hash or evidence_json is required")
	}
	if req.ForegroundEligible && strings.TrimSpace(req.ForegroundReasonJSON) == "" {
		return errors.New("foreground_reason_json is required when foreground_eligible is true")
	}
	if req.QuietTurns < 0 {
		return errors.New("quiet_turns must be non-negative")
	}
	return nil
}

func step23ThemeOffscreenTruthBoundaryValue() step23ThemeOffscreenTruthBoundary {
	return step23ThemeOffscreenTruthBoundary{
		SupportOnly:                 true,
		CanonicalWorldFactWriter:    false,
		AlwaysInjected:              false,
		RequiresEvidence:            true,
		MayOverrideCurrentUserInput: false,
		MayOverrideDirectEvidence:   false,
	}
}

func step23NormalizeThemeOffscreenSurfaceType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "theme_trace", "offscreen_thread":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func step23NormalizeThemeOffscreenStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "active", "dormant", "review", "retired":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return "active"
	}
}

func step23ApplyThemeOffscreenDormancy(status string, quietTurns, dormantAfter int) string {
	if status != "active" {
		return status
	}
	if quietTurns >= step23DefaultDormantAfterQuietTurns(dormantAfter) {
		return "dormant"
	}
	return status
}

func step23ThemeOffscreenFromStore(record store.ThemeOffscreenCarryRecord) step23ThemeOffscreenRecordResponse {
	return step23ThemeOffscreenRecordResponse{
		ID:                     record.ID,
		ChatSessionID:          record.ChatSessionID,
		SurfaceType:            record.SurfaceType,
		Label:                  record.Label,
		Summary:                record.Summary,
		Status:                 record.Status,
		Confidence:             record.Confidence,
		ConfidenceLabel:        record.ConfidenceLabel,
		SourceKind:             record.SourceKind,
		SourceTurnStart:        record.SourceTurnStart,
		SourceTurnEnd:          record.SourceTurnEnd,
		SourceHash:             record.SourceHash,
		EvidenceJSON:           record.EvidenceJSON,
		QuietTurns:             record.QuietTurns,
		LastSeenTurn:           record.LastSeenTurn,
		DormantAfterQuietTurns: record.DormantAfterQuietTurns,
		ForegroundEligible:     record.ForegroundEligible,
		ForegroundReasonJSON:   record.ForegroundReasonJSON,
		CreatedAt:              record.CreatedAt,
		UpdatedAt:              record.UpdatedAt,
	}
}
