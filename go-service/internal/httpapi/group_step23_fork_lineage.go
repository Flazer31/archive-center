package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

const (
	step23ForkLineageContractVersion = "step23_fork_lineage.v1"
	step23ForkLineageListRoute       = "/step23/fork-lineage/{chat_session_id}"
	step23ForkLineageDeclareRoute    = "/step23/fork-lineage"
)

type step23ForkLineageRecordResponse struct {
	ID                  int64     `json:"id"`
	ChatSessionID       string    `json:"chat_session_id"`
	ScopeID             string    `json:"scope_id,omitempty"`
	ParentScopeID       string    `json:"parent_scope_id,omitempty"`
	CopiedFromScopeID   string    `json:"copied_from_scope_id,omitempty"`
	CopiedFromSessionID string    `json:"copied_from_session_id,omitempty"`
	ImportedAt          time.Time `json:"imported_at"`
	DivergenceMarker    string    `json:"divergence_marker,omitempty"`
	ProvenanceSource    string    `json:"provenance_source"`
	InheritanceMode     string    `json:"inheritance_mode"`
	InheritedItemsJSON  string    `json:"inherited_items_json,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type step23ForkLineageTruthBoundary struct {
	SupportOnly            bool   `json:"support_only"`
	CanonicalTruthWriter   bool   `json:"canonical_truth_writer"`
	SilentMergeBackAllowed bool   `json:"silent_merge_back_allowed"`
	HiddenOverwriteAllowed bool   `json:"hidden_overwrite_allowed"`
	DefaultInheritanceMode string `json:"default_inheritance_mode"`
	AutomaticHookAvailable bool   `json:"automatic_hook_available"`
	CloseoutMode           string `json:"closeout_mode"`
}

type step23ForkLineageListResponse struct {
	Status          string                            `json:"status"`
	ContractVersion string                            `json:"contract_version"`
	ChatSessionID   string                            `json:"chat_session_id"`
	ScopeID         string                            `json:"scope_id,omitempty"`
	Records         []step23ForkLineageRecordResponse `json:"records"`
	TruthBoundary   step23ForkLineageTruthBoundary    `json:"truth_boundary"`
}

type step23ForkLineageDeclareRequest struct {
	ChatSessionID       string `json:"chat_session_id"`
	ScopeID             string `json:"scope_id"`
	ParentScopeID       string `json:"parent_scope_id"`
	CopiedFromScopeID   string `json:"copied_from_scope_id"`
	CopiedFromSessionID string `json:"copied_from_session_id"`
	ImportedAt          string `json:"imported_at"`
	DivergenceMarker    string `json:"divergence_marker"`
	ProvenanceSource    string `json:"provenance_source"`
	InheritanceMode     string `json:"inheritance_mode"`
	InheritedItemsJSON  string `json:"inherited_items_json"`
}

type step23ForkLineageDeclareResponse struct {
	Status          string                          `json:"status"`
	ContractVersion string                          `json:"contract_version"`
	Record          step23ForkLineageRecordResponse `json:"record"`
	TruthBoundary   step23ForkLineageTruthBoundary  `json:"truth_boundary"`
}

func (s *Server) registerStep23ForkLineageRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET "+step23ForkLineageListRoute, s.handleStep23ForkLineageList)
	mux.HandleFunc("POST "+step23ForkLineageDeclareRoute, s.handleStep23ForkLineageDeclare)
}

func (s *Server) handleStep23ForkLineageList(w http.ResponseWriter, r *http.Request) {
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	scopeID := strings.TrimSpace(r.URL.Query().Get("scope_id"))
	limit := step23ClampInt(step23IntQuery(r.URL.Query().Get("limit"), 100), 1, 1000)

	fs, ok := s.Store.(store.ForkLineageStore)
	if !ok {
		writeJSON(w, http.StatusOK, step23ForkLineageListResponse{
			Status:          "ok",
			ContractVersion: step23ForkLineageContractVersion,
			ChatSessionID:   sid,
			ScopeID:         scopeID,
			Records:         []step23ForkLineageRecordResponse{},
			TruthBoundary:   step23ForkLineageTruthBoundaryValue(),
		})
		return
	}
	records, err := fs.ListForkLineageRecords(r.Context(), sid, scopeID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	resp := step23ForkLineageListResponse{
		Status:          "ok",
		ContractVersion: step23ForkLineageContractVersion,
		ChatSessionID:   sid,
		ScopeID:         scopeID,
		Records:         make([]step23ForkLineageRecordResponse, 0, len(records)),
		TruthBoundary:   step23ForkLineageTruthBoundaryValue(),
	}
	for _, record := range records {
		resp.Records = append(resp.Records, step23ForkLineageFromStore(record))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleStep23ForkLineageDeclare(w http.ResponseWriter, r *http.Request) {
	fs, ok := s.Store.(store.ForkLineageStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "store_unavailable", "fork lineage store is not available")
		return
	}
	var req step23ForkLineageDeclareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if err := step23ValidateForkLineageDeclare(req); err != nil {
		writeError(w, http.StatusBadRequest, CodeMissingParam, err.Error())
		return
	}
	importedAt, err := step23ParseOptionalRFC3339(req.ImportedAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeBadRequest, "imported_at must be RFC3339")
		return
	}
	record := store.ForkLineageRecord{
		ChatSessionID:       strings.TrimSpace(req.ChatSessionID),
		ScopeID:             strings.TrimSpace(req.ScopeID),
		ParentScopeID:       strings.TrimSpace(req.ParentScopeID),
		CopiedFromScopeID:   strings.TrimSpace(req.CopiedFromScopeID),
		CopiedFromSessionID: strings.TrimSpace(req.CopiedFromSessionID),
		ImportedAt:          importedAt,
		DivergenceMarker:    strings.TrimSpace(req.DivergenceMarker),
		ProvenanceSource:    step23NormalizeForkLineageProvenance(req.ProvenanceSource),
		InheritanceMode:     step23NormalizeForkLineageInheritance(req.InheritanceMode),
		InheritedItemsJSON:  strings.TrimSpace(req.InheritedItemsJSON),
	}
	saved, err := fs.SaveForkLineageRecord(r.Context(), record)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, step23ForkLineageDeclareResponse{
		Status:          "ok",
		ContractVersion: step23ForkLineageContractVersion,
		Record:          step23ForkLineageFromStore(saved),
		TruthBoundary:   step23ForkLineageTruthBoundaryValue(),
	})
}

func step23ValidateForkLineageDeclare(req step23ForkLineageDeclareRequest) error {
	if strings.TrimSpace(req.ChatSessionID) == "" {
		return errors.New("chat_session_id is required")
	}
	if strings.TrimSpace(req.ScopeID) == "" {
		return errors.New("scope_id is required")
	}
	if strings.TrimSpace(req.ParentScopeID) == "" && strings.TrimSpace(req.CopiedFromScopeID) == "" && strings.TrimSpace(req.CopiedFromSessionID) == "" {
		return errors.New("parent_scope_id, copied_from_scope_id, or copied_from_session_id is required")
	}
	if strings.TrimSpace(req.ScopeID) == strings.TrimSpace(req.ParentScopeID) || strings.TrimSpace(req.ScopeID) == strings.TrimSpace(req.CopiedFromScopeID) {
		return errors.New("scope_id must differ from parent/copied scope")
	}
	if strings.TrimSpace(req.InheritedItemsJSON) == "" {
		return errors.New("inherited_items_json is required, use [] when nothing was imported")
	}
	if step23NormalizeForkLineageProvenance(req.ProvenanceSource) == "automatic_hook" {
		return errors.New("automatic_hook provenance is not available in this build; use manual")
	}
	return nil
}

func step23ForkLineageTruthBoundaryValue() step23ForkLineageTruthBoundary {
	return step23ForkLineageTruthBoundary{
		SupportOnly:            true,
		CanonicalTruthWriter:   false,
		SilentMergeBackAllowed: false,
		HiddenOverwriteAllowed: false,
		DefaultInheritanceMode: "conservative_import",
		AutomaticHookAvailable: false,
		CloseoutMode:           "manual_provenance_declaration",
	}
}

func step23NormalizeForkLineageProvenance(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "manual", "operator_manual", "user_manual":
		return "manual"
	case "automatic_hook":
		return "automatic_hook"
	default:
		return "manual"
	}
}

func step23NormalizeForkLineageInheritance(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "conservative_import", "review_safe_support_only":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return "conservative_import"
	}
}

func step23ParseOptionalRFC3339(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Now().UTC(), nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}

func step23ForkLineageFromStore(record store.ForkLineageRecord) step23ForkLineageRecordResponse {
	return step23ForkLineageRecordResponse{
		ID:                  record.ID,
		ChatSessionID:       record.ChatSessionID,
		ScopeID:             record.ScopeID,
		ParentScopeID:       record.ParentScopeID,
		CopiedFromScopeID:   record.CopiedFromScopeID,
		CopiedFromSessionID: record.CopiedFromSessionID,
		ImportedAt:          record.ImportedAt,
		DivergenceMarker:    record.DivergenceMarker,
		ProvenanceSource:    record.ProvenanceSource,
		InheritanceMode:     record.InheritanceMode,
		InheritedItemsJSON:  record.InheritedItemsJSON,
		CreatedAt:           record.CreatedAt,
		UpdatedAt:           record.UpdatedAt,
	}
}
