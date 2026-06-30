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
	step23ConsequenceContractVersion = "step23_consequence_ledger.v1"
	step23ConsequenceListRoute       = "/step23/consequences/{chat_session_id}"
	step23ConsequenceCreateRoute     = "/step23/consequences"
)

type step23ConsequenceRecordResponse struct {
	ID                     int64     `json:"id"`
	ChatSessionID          string    `json:"chat_session_id"`
	SourceTurnStart        int       `json:"source_turn_start"`
	SourceTurnEnd          int       `json:"source_turn_end"`
	Decision               string    `json:"decision"`
	ImmediateResult        string    `json:"immediate_result"`
	DelayedEffect          string    `json:"delayed_effect"`
	AffectedRelationsJSON  string    `json:"affected_relations_json,omitempty"`
	AffectedWorldJSON      string    `json:"affected_world_json,omitempty"`
	Status                 string    `json:"status"`
	Importance             float64   `json:"importance"`
	Confidence             float64   `json:"confidence"`
	ForegroundEligible     bool      `json:"foreground_eligible"`
	QuietTurns             int       `json:"quiet_turns"`
	LastSeenTurn           int       `json:"last_seen_turn"`
	PaidTurn               int       `json:"paid_turn"`
	ExpiresAfterQuietTurns int       `json:"expires_after_quiet_turns"`
	SourceHash             string    `json:"source_hash,omitempty"`
	EvidenceJSON           string    `json:"evidence_json,omitempty"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

type step23ConsequenceCounts struct {
	Total   int `json:"total"`
	Pending int `json:"pending"`
	Active  int `json:"active"`
	Paid    int `json:"paid"`
	Expired int `json:"expired"`
}

type step23ConsequenceTruthBoundary struct {
	SupportOnly          bool `json:"support_only"`
	CanonicalTruthWriter bool `json:"canonical_truth_writer"`
	RequiresEvidence     bool `json:"requires_evidence"`
}

type step23ConsequenceListResponse struct {
	Status          string                            `json:"status"`
	ContractVersion string                            `json:"contract_version"`
	ChatSessionID   string                            `json:"chat_session_id"`
	Records         []step23ConsequenceRecordResponse `json:"records"`
	Counts          step23ConsequenceCounts           `json:"counts"`
	TruthBoundary   step23ConsequenceTruthBoundary    `json:"truth_boundary"`
}

type step23ConsequenceCreateRequest struct {
	ChatSessionID          string  `json:"chat_session_id"`
	SourceTurnStart        int     `json:"source_turn_start"`
	SourceTurnEnd          int     `json:"source_turn_end"`
	Decision               string  `json:"decision"`
	ImmediateResult        string  `json:"immediate_result"`
	DelayedEffect          string  `json:"delayed_effect"`
	AffectedRelationsJSON  string  `json:"affected_relations_json"`
	AffectedWorldJSON      string  `json:"affected_world_json"`
	Status                 string  `json:"status"`
	Importance             float64 `json:"importance"`
	Confidence             float64 `json:"confidence"`
	ForegroundEligible     bool    `json:"foreground_eligible"`
	QuietTurns             int     `json:"quiet_turns"`
	LastSeenTurn           int     `json:"last_seen_turn"`
	PaidTurn               int     `json:"paid_turn"`
	ExpiresAfterQuietTurns int     `json:"expires_after_quiet_turns"`
	SourceHash             string  `json:"source_hash"`
	EvidenceJSON           string  `json:"evidence_json"`
}

type step23ConsequenceCreateResponse struct {
	Status          string                          `json:"status"`
	ContractVersion string                          `json:"contract_version"`
	Record          step23ConsequenceRecordResponse `json:"record"`
	TruthBoundary   step23ConsequenceTruthBoundary  `json:"truth_boundary"`
}

func (s *Server) registerStep23ConsequenceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET "+step23ConsequenceListRoute, s.handleStep23ConsequenceList)
	mux.HandleFunc("POST "+step23ConsequenceCreateRoute, s.handleStep23ConsequenceCreate)
}

func (s *Server) handleStep23ConsequenceList(w http.ResponseWriter, r *http.Request) {
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	limit := step23ClampInt(step23IntQuery(r.URL.Query().Get("limit"), 100), 1, 1000)

	cs, ok := s.Store.(store.ConsequenceRecordStore)
	if !ok {
		writeJSON(w, http.StatusOK, step23ConsequenceListResponse{
			Status:          "ok",
			ContractVersion: step23ConsequenceContractVersion,
			ChatSessionID:   sid,
			Records:         []step23ConsequenceRecordResponse{},
			Counts:          step23ConsequenceCounts{},
			TruthBoundary: step23ConsequenceTruthBoundary{
				SupportOnly:          true,
				CanonicalTruthWriter: false,
				RequiresEvidence:     true,
			},
		})
		return
	}

	records, err := cs.ListConsequenceRecords(r.Context(), sid, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}

	resp := step23ConsequenceListResponse{
		Status:          "ok",
		ContractVersion: step23ConsequenceContractVersion,
		ChatSessionID:   sid,
		Records:         make([]step23ConsequenceRecordResponse, 0, len(records)),
		Counts:          step23ConsequenceCounts{},
		TruthBoundary: step23ConsequenceTruthBoundary{
			SupportOnly:          true,
			CanonicalTruthWriter: false,
			RequiresEvidence:     true,
		},
	}
	for _, rec := range records {
		resp.Records = append(resp.Records, step23ConsequenceRecordFromStore(rec))
		resp.Counts.Total++
		switch rec.Status {
		case "pending":
			resp.Counts.Pending++
		case "active":
			resp.Counts.Active++
		case "paid":
			resp.Counts.Paid++
		case "expired":
			resp.Counts.Expired++
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleStep23ConsequenceCreate(w http.ResponseWriter, r *http.Request) {
	cs, ok := s.Store.(store.ConsequenceRecordStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "store_unavailable", "consequence store is not available")
		return
	}

	var req step23ConsequenceCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if err := step23ValidateConsequenceCreateRequest(req); err != nil {
		writeError(w, http.StatusBadRequest, CodeMissingParam, err.Error())
		return
	}

	rec := store.ConsequenceRecord{
		ChatSessionID:          strings.TrimSpace(req.ChatSessionID),
		SourceTurnStart:        req.SourceTurnStart,
		SourceTurnEnd:          req.SourceTurnEnd,
		Decision:               strings.TrimSpace(req.Decision),
		ImmediateResult:        strings.TrimSpace(req.ImmediateResult),
		DelayedEffect:          strings.TrimSpace(req.DelayedEffect),
		AffectedRelationsJSON:  strings.TrimSpace(req.AffectedRelationsJSON),
		AffectedWorldJSON:      strings.TrimSpace(req.AffectedWorldJSON),
		Status:                 step23NormalizeConsequenceStatus(req.Status),
		Importance:             req.Importance,
		Confidence:             step23ClampConfidence(req.Confidence),
		ForegroundEligible:     req.ForegroundEligible,
		QuietTurns:             req.QuietTurns,
		LastSeenTurn:           req.LastSeenTurn,
		PaidTurn:               req.PaidTurn,
		ExpiresAfterQuietTurns: step23DefaultExpiresAfterQuietTurns(req.ExpiresAfterQuietTurns),
		SourceHash:             strings.TrimSpace(req.SourceHash),
		EvidenceJSON:           strings.TrimSpace(req.EvidenceJSON),
	}

	saved, err := cs.SaveConsequenceRecord(r.Context(), rec)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, step23ConsequenceCreateResponse{
		Status:          "ok",
		ContractVersion: step23ConsequenceContractVersion,
		Record:          step23ConsequenceRecordFromStore(saved),
		TruthBoundary: step23ConsequenceTruthBoundary{
			SupportOnly:          true,
			CanonicalTruthWriter: false,
			RequiresEvidence:     true,
		},
	})
}

func step23ValidateConsequenceCreateRequest(req step23ConsequenceCreateRequest) error {
	if strings.TrimSpace(req.ChatSessionID) == "" {
		return errors.New("chat_session_id is required")
	}
	if req.SourceTurnStart < 0 || req.SourceTurnEnd < 0 {
		return errors.New("source_turn_start and source_turn_end must be non-negative")
	}
	if req.SourceTurnStart > req.SourceTurnEnd {
		return errors.New("source_turn_start must not exceed source_turn_end")
	}
	if strings.TrimSpace(req.Decision) == "" && strings.TrimSpace(req.ImmediateResult) == "" {
		return errors.New("decision or immediate_result is required")
	}
	if strings.TrimSpace(req.SourceHash) == "" && strings.TrimSpace(req.EvidenceJSON) == "" {
		return errors.New("source_hash or evidence_json is required")
	}
	return nil
}

func step23NormalizeConsequenceStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "pending", "active", "paid", "expired":
		return strings.ToLower(strings.TrimSpace(status))
	default:
		return "active"
	}
}

func step23ClampConfidence(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func step23DefaultExpiresAfterQuietTurns(v int) int {
	if v <= 0 {
		return 20
	}
	return v
}

func step23ConsequenceRecordFromStore(rec store.ConsequenceRecord) step23ConsequenceRecordResponse {
	return step23ConsequenceRecordResponse{
		ID:                     rec.ID,
		ChatSessionID:          rec.ChatSessionID,
		SourceTurnStart:        rec.SourceTurnStart,
		SourceTurnEnd:          rec.SourceTurnEnd,
		Decision:               rec.Decision,
		ImmediateResult:        rec.ImmediateResult,
		DelayedEffect:          rec.DelayedEffect,
		AffectedRelationsJSON:  rec.AffectedRelationsJSON,
		AffectedWorldJSON:      rec.AffectedWorldJSON,
		Status:                 rec.Status,
		Importance:             rec.Importance,
		Confidence:             rec.Confidence,
		ForegroundEligible:     rec.ForegroundEligible,
		QuietTurns:             rec.QuietTurns,
		LastSeenTurn:           rec.LastSeenTurn,
		PaidTurn:               rec.PaidTurn,
		ExpiresAfterQuietTurns: rec.ExpiresAfterQuietTurns,
		SourceHash:             rec.SourceHash,
		EvidenceJSON:           rec.EvidenceJSON,
		CreatedAt:              rec.CreatedAt,
		UpdatedAt:              rec.UpdatedAt,
	}
}

func step23IntQuery(raw string, fallback int) int {
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

func step23ClampInt(value, minValue, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
