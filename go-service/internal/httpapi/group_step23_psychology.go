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
	step23PsychologyContractVersion = "step23_psychology_branch.v1"
	step23PsychologyListRoute       = "/step23/psychology-branches/{chat_session_id}"
	step23PsychologyCreateRoute     = "/step23/psychology-branches"
	step23PsychologyStatusRoute     = "/step23/psychology-branches/{id}/status"
)

type step23PsychologyBranchResponse struct {
	ID                     int64     `json:"id"`
	ChatSessionID          string    `json:"chat_session_id"`
	CharacterName          string    `json:"character_name"`
	BranchType             string    `json:"branch_type"`
	AxisName               string    `json:"axis_name"`
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
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

type step23PsychologyCounts struct {
	Total   int `json:"total"`
	Active  int `json:"active"`
	Dormant int `json:"dormant"`
	Review  int `json:"review"`
	Retired int `json:"retired"`
}

type step23PsychologyTruthBoundary struct {
	SupportOnly                        bool `json:"support_only"`
	CanonicalTruthWriter               bool `json:"canonical_truth_writer"`
	RequiresEvidence                   bool `json:"requires_evidence"`
	MayDecideUserPersonaAction         bool `json:"may_decide_user_persona_action"`
	MotiveShadowHintAutoPersistAllowed bool `json:"motive_shadow_hint_auto_persist_allowed"`
}

type step23PsychologyListResponse struct {
	Status          string                           `json:"status"`
	ContractVersion string                           `json:"contract_version"`
	ChatSessionID   string                           `json:"chat_session_id"`
	Branches        []step23PsychologyBranchResponse `json:"branches"`
	Counts          step23PsychologyCounts           `json:"counts"`
	TruthBoundary   step23PsychologyTruthBoundary    `json:"truth_boundary"`
}

type step23PsychologyCreateRequest struct {
	ChatSessionID          string  `json:"chat_session_id"`
	CharacterName          string  `json:"character_name"`
	BranchType             string  `json:"branch_type"`
	AxisName               string  `json:"axis_name"`
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
}

type step23PsychologyCreateResponse struct {
	Status          string                         `json:"status"`
	ContractVersion string                         `json:"contract_version"`
	Branch          step23PsychologyBranchResponse `json:"branch"`
	TruthBoundary   step23PsychologyTruthBoundary  `json:"truth_boundary"`
}

type step23PsychologyStatusRequest struct {
	Status     string `json:"status"`
	QuietTurns int    `json:"quiet_turns"`
}

func (s *Server) registerStep23PsychologyRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET "+step23PsychologyListRoute, s.handleStep23PsychologyList)
	mux.HandleFunc("POST "+step23PsychologyCreateRoute, s.handleStep23PsychologyCreate)
	mux.HandleFunc("PATCH "+step23PsychologyStatusRoute, s.handleStep23PsychologyStatus)
}

func (s *Server) handleStep23PsychologyList(w http.ResponseWriter, r *http.Request) {
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	limit := step23ClampInt(step23IntQuery(r.URL.Query().Get("limit"), 100), 1, 1000)

	ps, ok := s.Store.(store.PsychologyBranchStore)
	if !ok {
		writeJSON(w, http.StatusOK, step23PsychologyListResponse{
			Status:          "ok",
			ContractVersion: step23PsychologyContractVersion,
			ChatSessionID:   sid,
			Branches:        []step23PsychologyBranchResponse{},
			Counts:          step23PsychologyCounts{},
			TruthBoundary:   step23PsychologyTruthBoundaryValue(),
		})
		return
	}

	branches, err := ps.ListPsychologyBranches(r.Context(), sid, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	resp := step23PsychologyListResponse{
		Status:          "ok",
		ContractVersion: step23PsychologyContractVersion,
		ChatSessionID:   sid,
		Branches:        make([]step23PsychologyBranchResponse, 0, len(branches)),
		Counts:          step23PsychologyCounts{},
		TruthBoundary:   step23PsychologyTruthBoundaryValue(),
	}
	for _, branch := range branches {
		resp.Branches = append(resp.Branches, step23PsychologyBranchFromStore(branch))
		resp.Counts.Total++
		switch branch.Status {
		case "active":
			resp.Counts.Active++
		case "dormant":
			resp.Counts.Dormant++
		case "review":
			resp.Counts.Review++
		case "retired":
			resp.Counts.Retired++
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleStep23PsychologyCreate(w http.ResponseWriter, r *http.Request) {
	ps, ok := s.Store.(store.PsychologyBranchStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "store_unavailable", "psychology branch store is not available")
		return
	}
	var req step23PsychologyCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if err := step23ValidatePsychologyCreateRequest(req); err != nil {
		writeError(w, http.StatusBadRequest, CodeMissingParam, err.Error())
		return
	}

	branch := store.PsychologyBranch{
		ChatSessionID:          strings.TrimSpace(req.ChatSessionID),
		CharacterName:          strings.TrimSpace(req.CharacterName),
		BranchType:             step23NormalizePsychologyBranchType(req.BranchType),
		AxisName:               step23DefaultPsychologyAxisName(req.AxisName, req.BranchType),
		Summary:                strings.TrimSpace(req.Summary),
		Status:                 step23ApplyPsychologyDormancy(step23NormalizePsychologyStatus(req.Status), req.QuietTurns, req.DormantAfterQuietTurns),
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
	}
	branch.Status = step23ApplyPsychologyDormancy(branch.Status, branch.QuietTurns, branch.DormantAfterQuietTurns)

	saved, err := ps.SavePsychologyBranch(r.Context(), branch)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, step23PsychologyCreateResponse{
		Status:          "ok",
		ContractVersion: step23PsychologyContractVersion,
		Branch:          step23PsychologyBranchFromStore(saved),
		TruthBoundary:   step23PsychologyTruthBoundaryValue(),
	})
}

func (s *Server) handleStep23PsychologyStatus(w http.ResponseWriter, r *http.Request) {
	ps, ok := s.Store.(store.PsychologyBranchStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "store_unavailable", "psychology branch store is not available")
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("id")), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "valid id is required")
		return
	}
	var req step23PsychologyStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	status := step23NormalizePsychologyStatus(req.Status)
	if req.QuietTurns < 0 {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "quiet_turns must be non-negative")
		return
	}
	if err := ps.UpdatePsychologyBranchStatus(r.Context(), id, status, req.QuietTurns); err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"contract_version": step23PsychologyContractVersion,
		"id":               id,
		"branch_status":    status,
		"truth_boundary":   step23PsychologyTruthBoundaryValue(),
	})
}

func step23ValidatePsychologyCreateRequest(req step23PsychologyCreateRequest) error {
	if strings.TrimSpace(req.ChatSessionID) == "" {
		return errors.New("chat_session_id is required")
	}
	if strings.TrimSpace(req.CharacterName) == "" {
		return errors.New("character_name is required")
	}
	if step23NormalizePsychologyBranchType(req.BranchType) == "" {
		return errors.New("branch_type must be one of desire, fear, wound, mask, bond, fixation")
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
	if step23IsMotiveShadowOnlySource(req.SourceKind) {
		return errors.New("motive_shadow_hint alone must not be persisted as a psychology branch")
	}
	if label := strings.TrimSpace(req.ConfidenceLabel); label != "" {
		switch strings.ToLower(label) {
		case "low", "medium", "high":
		default:
			return errors.New("confidence_label must be low, medium, or high")
		}
	}
	if req.QuietTurns < 0 {
		return errors.New("quiet_turns must be non-negative")
	}
	return nil
}

func step23PsychologyTruthBoundaryValue() step23PsychologyTruthBoundary {
	return step23PsychologyTruthBoundary{
		SupportOnly:                        true,
		CanonicalTruthWriter:               false,
		RequiresEvidence:                   true,
		MayDecideUserPersonaAction:         false,
		MotiveShadowHintAutoPersistAllowed: false,
	}
}

func step23NormalizePsychologyBranchType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "desire", "fear", "wound", "mask", "bond", "fixation":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func step23NormalizePsychologyStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "active", "dormant", "review", "retired":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return "active"
	}
}

func step23NormalizePsychologyConfidenceLabel(raw string, confidence float64) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "low", "medium", "high":
		return strings.ToLower(strings.TrimSpace(raw))
	}
	confidence = step23ClampConfidence(confidence)
	if confidence < 0.34 {
		return "low"
	}
	if confidence < 0.67 {
		return "medium"
	}
	return "high"
}

func step23DefaultPsychologyAxisName(axisName, branchType string) string {
	axisName = strings.TrimSpace(axisName)
	if axisName != "" {
		return axisName
	}
	return step23NormalizePsychologyBranchType(branchType)
}

func step23DefaultDormantAfterQuietTurns(v int) int {
	if v <= 0 {
		return 15
	}
	return v
}

func step23ApplyPsychologyDormancy(status string, quietTurns, dormantAfter int) string {
	if status != "active" {
		return status
	}
	if quietTurns >= step23DefaultDormantAfterQuietTurns(dormantAfter) {
		return "dormant"
	}
	return status
}

func step23IsMotiveShadowOnlySource(sourceKind string) bool {
	switch strings.ToLower(strings.TrimSpace(sourceKind)) {
	case "motive_shadow_hint", "step20_motive_shadow_hint", "q20j_motive_shadow_hint":
		return true
	default:
		return false
	}
}

func step23PsychologyBranchFromStore(branch store.PsychologyBranch) step23PsychologyBranchResponse {
	return step23PsychologyBranchResponse{
		ID:                     branch.ID,
		ChatSessionID:          branch.ChatSessionID,
		CharacterName:          branch.CharacterName,
		BranchType:             branch.BranchType,
		AxisName:               branch.AxisName,
		Summary:                branch.Summary,
		Status:                 branch.Status,
		Confidence:             branch.Confidence,
		ConfidenceLabel:        branch.ConfidenceLabel,
		SourceKind:             branch.SourceKind,
		SourceTurnStart:        branch.SourceTurnStart,
		SourceTurnEnd:          branch.SourceTurnEnd,
		SourceHash:             branch.SourceHash,
		EvidenceJSON:           branch.EvidenceJSON,
		QuietTurns:             branch.QuietTurns,
		LastSeenTurn:           branch.LastSeenTurn,
		DormantAfterQuietTurns: branch.DormantAfterQuietTurns,
		CreatedAt:              branch.CreatedAt,
		UpdatedAt:              branch.UpdatedAt,
	}
}
