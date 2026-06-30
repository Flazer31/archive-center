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
	step23CaptureVerificationContractVersion = "step23_capture_verification.v1"
	step23CaptureVerificationListRoute       = "/step23/capture-verification/{chat_session_id}"
	step23CaptureVerificationCreateRoute     = "/step23/capture-verification"
	step23CaptureVerificationRepairRoute     = "/step23/capture-verification/{id}/repair"
)

// Capture verification states.
const (
	captureStateSingleStage   = "single-stage"
	captureStateMultiStage    = "multi-stage"
	captureStateVerified      = "verified"
	captureStateVerifiedFinal = "verified-final"
	captureStateDegraded      = "degraded"
)

// Capture stage names.
const (
	captureStageBeforeRequestResponse = "beforeRequestResponse"
	captureStageAfterRequest          = "afterRequest"
	captureStageFinalize              = "finalize"
	captureStageRecovery              = "recovery"
)

type step23CaptureVerificationRecordResponse struct {
	ID                  int64     `json:"id"`
	ChatSessionID       string    `json:"chat_session_id"`
	TurnIndex           int       `json:"turn_index"`
	StageName           string    `json:"stage_name"`
	VerificationState   string    `json:"verification_state"`
	DegradedReason      string    `json:"degraded_reason,omitempty"`
	CompactMetadataJSON string    `json:"compact_metadata_json,omitempty"`
	ContentHash         string    `json:"content_hash,omitempty"`
	EvidenceJSON        string    `json:"evidence_json,omitempty"`
	PreviousRecordID    int64     `json:"previous_record_id,omitempty"`
	RepairedByRecordID  int64     `json:"repaired_by_record_id,omitempty"`
	RepairAttemptCount  int       `json:"repair_attempt_count"`
	RepairEvidenceJSON  string    `json:"repair_evidence_json,omitempty"`
	RepairedAt          time.Time `json:"repaired_at,omitempty"`
	UserInputPreserved  bool      `json:"user_input_preserved"`
	PayloadRewrite      bool      `json:"payload_rewrite"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type step23CaptureVerificationCounts struct {
	Total         int `json:"total"`
	SingleStage   int `json:"single_stage"`
	MultiStage    int `json:"multi_stage"`
	Verified      int `json:"verified"`
	VerifiedFinal int `json:"verified_final"`
	Degraded      int `json:"degraded"`
}

type step23CaptureVerificationTruthBoundary struct {
	SupportOnly          bool `json:"support_only"`
	CanonicalTruthWriter bool `json:"canonical_truth_writer"`
	RequiresEvidence     bool `json:"requires_evidence"`
	MayRewriteUserInput  bool `json:"may_rewrite_user_input"`
	MayAutoRepair        bool `json:"may_auto_repair"`
}

type step23CaptureVerificationListResponse struct {
	Status          string                                    `json:"status"`
	ContractVersion string                                    `json:"contract_version"`
	ChatSessionID   string                                    `json:"chat_session_id"`
	Records         []step23CaptureVerificationRecordResponse `json:"records"`
	Counts          step23CaptureVerificationCounts           `json:"counts"`
	TruthBoundary   step23CaptureVerificationTruthBoundary    `json:"truth_boundary"`
}

type step23CaptureVerificationCreateRequest struct {
	ChatSessionID       string `json:"chat_session_id"`
	TurnIndex           int    `json:"turn_index"`
	StageName           string `json:"stage_name"`
	VerificationState   string `json:"verification_state"`
	DegradedReason      string `json:"degraded_reason"`
	CompactMetadataJSON string `json:"compact_metadata_json"`
	ContentHash         string `json:"content_hash"`
	EvidenceJSON        string `json:"evidence_json"`
	PreviousRecordID    int64  `json:"previous_record_id"`
	RepairAttemptCount  int    `json:"repair_attempt_count"`
	UserInputPreserved  *bool  `json:"user_input_preserved"`
	PayloadRewrite      bool   `json:"payload_rewrite"`
}

type step23CaptureVerificationCreateResponse struct {
	Status          string                                  `json:"status"`
	ContractVersion string                                  `json:"contract_version"`
	Record          step23CaptureVerificationRecordResponse `json:"record"`
	TruthBoundary   step23CaptureVerificationTruthBoundary  `json:"truth_boundary"`
}

type step23CaptureVerificationRepairRequest struct {
	VerificationState  string `json:"verification_state"`
	DegradedReason     string `json:"degraded_reason"`
	RepairEvidenceJSON string `json:"repair_evidence_json"`
	RepairedByRecordID int64  `json:"repaired_by_record_id"`
	UserInputPreserved *bool  `json:"user_input_preserved"`
}

func (s *Server) registerStep23CaptureVerificationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET "+step23CaptureVerificationListRoute, s.handleStep23CaptureVerificationList)
	mux.HandleFunc("POST "+step23CaptureVerificationCreateRoute, s.handleStep23CaptureVerificationCreate)
	mux.HandleFunc("PATCH "+step23CaptureVerificationRepairRoute, s.handleStep23CaptureVerificationRepair)
}

func (s *Server) handleStep23CaptureVerificationList(w http.ResponseWriter, r *http.Request) {
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	limit := step23ClampInt(step23IntQuery(r.URL.Query().Get("limit"), 100), 1, 1000)

	cs, ok := s.Store.(store.CaptureVerificationStore)
	if !ok {
		writeJSON(w, http.StatusOK, step23CaptureVerificationListResponse{
			Status:          "ok",
			ContractVersion: step23CaptureVerificationContractVersion,
			ChatSessionID:   sid,
			Records:         []step23CaptureVerificationRecordResponse{},
			Counts:          step23CaptureVerificationCounts{},
			TruthBoundary:   step23CaptureVerificationTruthBoundaryValue(),
		})
		return
	}

	records, err := cs.ListCaptureVerifications(r.Context(), sid, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}

	resp := step23CaptureVerificationListResponse{
		Status:          "ok",
		ContractVersion: step23CaptureVerificationContractVersion,
		ChatSessionID:   sid,
		Records:         make([]step23CaptureVerificationRecordResponse, 0, len(records)),
		Counts:          step23CaptureVerificationCounts{Total: len(records)},
		TruthBoundary:   step23CaptureVerificationTruthBoundaryValue(),
	}
	for _, rec := range records {
		resp.Records = append(resp.Records, step23CaptureVerificationRecordFromStore(rec))
		switch rec.VerificationState {
		case captureStateSingleStage:
			resp.Counts.SingleStage++
		case captureStateMultiStage:
			resp.Counts.MultiStage++
		case captureStateVerified:
			resp.Counts.Verified++
		case captureStateVerifiedFinal:
			resp.Counts.VerifiedFinal++
		case captureStateDegraded:
			resp.Counts.Degraded++
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleStep23CaptureVerificationCreate(w http.ResponseWriter, r *http.Request) {
	cs, ok := s.Store.(store.CaptureVerificationStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "store_unavailable", "capture verification store is not available")
		return
	}

	var req step23CaptureVerificationCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if err := step23ValidateCaptureVerificationCreateRequest(req); err != nil {
		writeError(w, http.StatusBadRequest, CodeMissingParam, err.Error())
		return
	}

	userInputPreserved := step23BoolDefault(req.UserInputPreserved, true)
	rec := store.CaptureVerificationRecord{
		ChatSessionID:       strings.TrimSpace(req.ChatSessionID),
		TurnIndex:           req.TurnIndex,
		StageName:           step23NormalizeCaptureStage(req.StageName),
		VerificationState:   step23NormalizeCaptureState(req.VerificationState),
		DegradedReason:      strings.TrimSpace(req.DegradedReason),
		CompactMetadataJSON: strings.TrimSpace(req.CompactMetadataJSON),
		ContentHash:         strings.TrimSpace(req.ContentHash),
		EvidenceJSON:        strings.TrimSpace(req.EvidenceJSON),
		PreviousRecordID:    req.PreviousRecordID,
		RepairAttemptCount:  step23ClampInt(req.RepairAttemptCount, 0, 9999),
		UserInputPreserved:  userInputPreserved,
		PayloadRewrite:      req.PayloadRewrite,
	}

	saved, err := cs.SaveCaptureVerification(r.Context(), rec)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, step23CaptureVerificationCreateResponse{
		Status:          "ok",
		ContractVersion: step23CaptureVerificationContractVersion,
		Record:          step23CaptureVerificationRecordFromStore(saved),
		TruthBoundary:   step23CaptureVerificationTruthBoundaryValue(),
	})
}

func (s *Server) handleStep23CaptureVerificationRepair(w http.ResponseWriter, r *http.Request) {
	cs, ok := s.Store.(store.CaptureVerificationStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "store_unavailable", "capture verification store is not available")
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("id")), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "valid id is required")
		return
	}
	var req step23CaptureVerificationRepairRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}

	state := step23NormalizeCaptureState(req.VerificationState)
	if state == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "verification_state must be one of single-stage, multi-stage, verified, verified-final, degraded")
		return
	}
	// Do not mark repair success without evidence.
	if (state == captureStateVerified || state == captureStateVerifiedFinal) && strings.TrimSpace(req.RepairEvidenceJSON) == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "repair success state requires repair_evidence_json")
		return
	}
	if err := step23ValidateOptionalJSONField("repair_evidence_json", req.RepairEvidenceJSON); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json_field", err.Error())
		return
	}
	if state == captureStateDegraded && strings.TrimSpace(req.DegradedReason) == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "degraded state requires degraded_reason")
		return
	}
	userInputPreserved := step23BoolDefault(req.UserInputPreserved, true)
	if !userInputPreserved && state != captureStateDegraded {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "user_input_preserved=false requires degraded verification_state")
		return
	}

	if err := cs.UpdateCaptureVerificationRepair(r.Context(), id, state,
		strings.TrimSpace(req.DegradedReason), strings.TrimSpace(req.RepairEvidenceJSON),
		req.RepairedByRecordID, userInputPreserved); err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":             "ok",
		"contract_version":   step23CaptureVerificationContractVersion,
		"id":                 id,
		"verification_state": state,
		"truth_boundary":     step23CaptureVerificationTruthBoundaryValue(),
	})
}

func step23ValidateCaptureVerificationCreateRequest(req step23CaptureVerificationCreateRequest) error {
	if strings.TrimSpace(req.ChatSessionID) == "" {
		return errors.New("chat_session_id is required")
	}
	if req.TurnIndex < 0 {
		return errors.New("turn_index must be non-negative")
	}
	if step23NormalizeCaptureStage(req.StageName) == "" {
		return errors.New("stage_name must be one of beforeRequestResponse, afterRequest, finalize, recovery")
	}
	state := step23NormalizeCaptureState(req.VerificationState)
	if state == "" {
		return errors.New("verification_state must be one of single-stage, multi-stage, verified, verified-final, degraded")
	}
	if strings.TrimSpace(req.ContentHash) == "" && strings.TrimSpace(req.EvidenceJSON) == "" {
		return errors.New("content_hash or evidence_json is required")
	}
	if err := step23ValidateOptionalJSONField("compact_metadata_json", req.CompactMetadataJSON); err != nil {
		return err
	}
	if err := step23ValidateOptionalJSONField("evidence_json", req.EvidenceJSON); err != nil {
		return err
	}
	if state == captureStateDegraded && strings.TrimSpace(req.DegradedReason) == "" {
		return errors.New("degraded_reason is required when verification_state is degraded")
	}
	if req.UserInputPreserved != nil && !*req.UserInputPreserved && state != captureStateDegraded {
		return errors.New("user_input_preserved=false requires degraded verification_state")
	}
	if req.PayloadRewrite {
		return errors.New("payload_rewrite must be false in Step 23; user input is immutable")
	}
	return nil
}

func step23ValidateOptionalJSONField(fieldName, raw string) error {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil
	}
	if !json.Valid([]byte(value)) {
		return errors.New(fieldName + " must be valid JSON")
	}
	return nil
}

func step23CaptureVerificationTruthBoundaryValue() step23CaptureVerificationTruthBoundary {
	return step23CaptureVerificationTruthBoundary{
		SupportOnly:          true,
		CanonicalTruthWriter: false,
		RequiresEvidence:     true,
		MayRewriteUserInput:  false,
		MayAutoRepair:        false,
	}
}

func step23NormalizeCaptureStage(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "beforerequestresponse", "before_request_response", "before-request-response":
		return captureStageBeforeRequestResponse
	case "afterrequest", "after_request", "after-request":
		return captureStageAfterRequest
	case captureStageFinalize:
		return captureStageFinalize
	case captureStageRecovery:
		return captureStageRecovery
	default:
		return ""
	}
}

func step23NormalizeCaptureState(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case captureStateSingleStage, captureStateMultiStage, captureStateVerified, captureStateVerifiedFinal, captureStateDegraded:
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func step23BoolDefault(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func step23CaptureVerificationRecordFromStore(rec store.CaptureVerificationRecord) step23CaptureVerificationRecordResponse {
	return step23CaptureVerificationRecordResponse{
		ID:                  rec.ID,
		ChatSessionID:       rec.ChatSessionID,
		TurnIndex:           rec.TurnIndex,
		StageName:           rec.StageName,
		VerificationState:   rec.VerificationState,
		DegradedReason:      rec.DegradedReason,
		CompactMetadataJSON: rec.CompactMetadataJSON,
		ContentHash:         rec.ContentHash,
		EvidenceJSON:        rec.EvidenceJSON,
		PreviousRecordID:    rec.PreviousRecordID,
		RepairedByRecordID:  rec.RepairedByRecordID,
		RepairAttemptCount:  rec.RepairAttemptCount,
		RepairEvidenceJSON:  rec.RepairEvidenceJSON,
		RepairedAt:          rec.RepairedAt,
		UserInputPreserved:  rec.UserInputPreserved,
		PayloadRewrite:      rec.PayloadRewrite,
		CreatedAt:           rec.CreatedAt,
		UpdatedAt:           rec.UpdatedAt,
	}
}
