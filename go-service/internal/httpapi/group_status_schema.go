package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/risulongmemory/archive-center-go/internal/store"
	archvector "github.com/risulongmemory/archive-center-go/internal/vector"
)

const (
	statusSchemaContractVersion          = "status_schema_input.v1"
	statusSchemaRegistryContractVersion  = "status_schema_registry.v1"
	statusSchemaValueContractVersion     = "status_current_value.v1"
	statusSchemaLifecycleContractVersion = "status_lifecycle.v1"
	statusSchemaQueryContractVersion     = "status_query_projection.v1"
	statusSchemaListRoute                = "/status-schema/proposals/{chat_session_id}"
	statusSchemaCreateRoute              = "/status-schema/proposals"
	statusSchemaReviewRoute              = "/status-schema/proposals/{id}/review"
	statusSchemaRegistryListRoute        = "/status-schema/registry/{chat_session_id}"
	statusSchemaRegistryImportRoute      = "/status-schema/registry/from-proposal/{id}"
	statusSchemaValueListRoute           = "/status-schema/values/{chat_session_id}"
	statusSchemaValueWriteRoute          = "/status-schema/values"
	statusSchemaEventListRoute           = "/status-schema/events/{chat_session_id}"
	statusSchemaEventWriteRoute          = "/status-schema/events"
	statusSchemaEffectListRoute          = "/status-schema/effects/{chat_session_id}"
	statusSchemaEffectWriteRoute         = "/status-schema/effects"
	statusSchemaEffectStateRoute         = "/status-schema/effects/{id}/state"
	statusSchemaQueryRoute               = "/status-schema/query"
	statusSchemaProjectionRoute          = "/status-schema/projection/{chat_session_id}"
	statusSchemaMinListLimit             = 1
	statusSchemaDefaultListLimit         = 100
	statusSchemaMaxListLimit             = 1000
)

type statusSchemaProposalResponse struct {
	ID             int64     `json:"id"`
	ChatSessionID  string    `json:"chat_session_id"`
	InputChannel   string    `json:"input_channel"`
	ProposalState  string    `json:"proposal_state"`
	SchemaName     string    `json:"schema_name"`
	RulesetLabel   string    `json:"ruleset_label,omitempty"`
	SchemaJSON     string    `json:"schema_json"`
	ProvenanceJSON string    `json:"provenance_json,omitempty"`
	ReviewNote     string    `json:"review_note,omitempty"`
	Reviewer       string    `json:"reviewer,omitempty"`
	ReviewedAt     time.Time `json:"reviewed_at,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type statusSchemaCounts struct {
	Total         int `json:"total"`
	PendingReview int `json:"pending_review"`
	Approved      int `json:"approved"`
	Rejected      int `json:"rejected"`
	NeedsRevision int `json:"needs_revision"`
}

type statusSchemaTruthBoundary struct {
	ProposalOnly                 bool     `json:"proposal_only"`
	CanonicalStatusWriter        bool     `json:"canonical_status_writer"`
	ReviewRequiredBeforeCanon    bool     `json:"review_required_before_canon"`
	ApprovalRegistersSchema      bool     `json:"approval_registers_schema"`
	CurrentValueWritesAllowed    bool     `json:"current_value_writes_allowed"`
	EffectLifecycleWritesAllowed bool     `json:"effect_lifecycle_writes_allowed"`
	ArbitraryCodeFormulaAllowed  bool     `json:"arbitrary_code_formula_allowed"`
	AcceptedInputChannels        []string `json:"accepted_input_channels"`
	AcceptedReviewStates         []string `json:"accepted_review_states"`
}

type statusSchemaVectorPolicy struct {
	ChromaLinked          bool   `json:"chroma_linked"`
	VectorLane            string `json:"vector_lane"`
	Tier                  string `json:"tier"`
	SourceTable           string `json:"source_table"`
	HydrateRequired       bool   `json:"hydrate_required"`
	CanonicalTruthSource  string `json:"canonical_truth_source"`
	TruthWriter           bool   `json:"truth_writer"`
	IndexPendingProposals bool   `json:"index_pending_proposals"`
	IndexReviewedStates   bool   `json:"index_reviewed_states"`
}

type statusSchemaRegistryPolicy struct {
	CanonicalSchemaRegistry     bool     `json:"canonical_schema_registry"`
	RequiresApprovedProposal    bool     `json:"requires_approved_proposal"`
	CurrentValueWritesAllowed   bool     `json:"current_value_writes_allowed"`
	EffectLifecycleAllowed      bool     `json:"effect_lifecycle_allowed"`
	HardcodedStatusNamesAllowed bool     `json:"hardcoded_status_names_allowed"`
	AcceptedOwnerScopes         []string `json:"accepted_owner_scopes"`
	AcceptedValueKinds          []string `json:"accepted_value_kinds"`
	VectorLane                  string   `json:"vector_lane"`
	VectorTier                  string   `json:"vector_tier"`
	VectorSourceTable           string   `json:"vector_source_table"`
	HydrateRequired             bool     `json:"hydrate_required"`
}

type statusCurrentValuePolicy struct {
	CanonicalCurrentValueWriter bool     `json:"canonical_current_value_writer"`
	RequiresActiveRegistry      bool     `json:"requires_active_registry"`
	EvidenceRequired            bool     `json:"evidence_required"`
	HistoryWritesAllowed        bool     `json:"history_writes_allowed"`
	EffectLifecycleAllowed      bool     `json:"effect_lifecycle_allowed"`
	VectorTruthWriter           bool     `json:"vector_truth_writer"`
	DirectDerivedWritesAllowed  bool     `json:"direct_derived_writes_allowed"`
	AcceptedOwnerScopes         []string `json:"accepted_owner_scopes"`
	AcceptedValueKinds          []string `json:"accepted_value_kinds"`
	CanonicalTruthSource        string   `json:"canonical_truth_source"`
}

type statusLifecyclePolicy struct {
	ChangeEventLedgerWriter      bool     `json:"change_event_ledger_writer"`
	EffectLifecycleWriter        bool     `json:"effect_lifecycle_writer"`
	RequiresActiveRegistry       bool     `json:"requires_active_registry"`
	EvidenceRequired             bool     `json:"evidence_required"`
	RequiresStoryClockForEffects bool     `json:"requires_story_clock_for_effects"`
	CurrentValueMutationAllowed  bool     `json:"current_value_mutation_allowed"`
	VectorTruthWriter            bool     `json:"vector_truth_writer"`
	AcceptedEventKinds           []string `json:"accepted_event_kinds"`
	AcceptedEffectKinds          []string `json:"accepted_effect_kinds"`
	AcceptedEffectStates         []string `json:"accepted_effect_states"`
	CanonicalEventSource         string   `json:"canonical_event_source"`
	CanonicalEffectSource        string   `json:"canonical_effect_source"`
}

type statusQueryProjectionPolicy struct {
	CanonFirstQuery                   bool     `json:"canon_first_query"`
	SemanticMemoryFallbackAsTruth     bool     `json:"semantic_memory_fallback_as_truth"`
	ExternalRuntimeAuthoritySupported bool     `json:"external_runtime_authority_supported"`
	ExternalRuntimeOverridesArchive   bool     `json:"external_runtime_overrides_archive"`
	UnknownStatusCreatesCanon         bool     `json:"unknown_status_creates_canon"`
	UnknownStatusProposalOnly         bool     `json:"unknown_status_proposal_only"`
	VectorTruthWriter                 bool     `json:"vector_truth_writer"`
	AcceptedAuthorityModes            []string `json:"accepted_authority_modes"`
	AcceptedProjectionDensities       []string `json:"accepted_projection_densities"`
	CanonicalValueSource              string   `json:"canonical_value_source"`
	CanonicalEffectSource             string   `json:"canonical_effect_source"`
}

type statusSchemaVectorIndex struct {
	Status         string `json:"status"`
	Attempted      bool   `json:"attempted"`
	Reason         string `json:"reason,omitempty"`
	DocumentID     string `json:"document_id,omitempty"`
	Tier           string `json:"tier,omitempty"`
	SourceTable    string `json:"source_table,omitempty"`
	SourceRowID    string `json:"source_row_id,omitempty"`
	EmbeddingMode  string `json:"embedding_mode,omitempty"`
	EmbeddingModel string `json:"embedding_model,omitempty"`
	DocumentChars  int    `json:"document_chars,omitempty"`
}

type statusSchemaListResponse struct {
	Status          string                         `json:"status"`
	ContractVersion string                         `json:"contract_version"`
	ChatSessionID   string                         `json:"chat_session_id"`
	ProposalState   string                         `json:"proposal_state,omitempty"`
	Proposals       []statusSchemaProposalResponse `json:"proposals"`
	Counts          statusSchemaCounts             `json:"counts"`
	TruthBoundary   statusSchemaTruthBoundary      `json:"truth_boundary"`
	VectorPolicy    statusSchemaVectorPolicy       `json:"vector_policy"`
}

type statusSchemaCreateRequest struct {
	ChatSessionID   string          `json:"chat_session_id"`
	InputChannel    string          `json:"input_channel"`
	SchemaName      string          `json:"schema_name"`
	RulesetLabel    string          `json:"ruleset_label"`
	SchemaJSON      json.RawMessage `json:"schema_json"`
	ProvenanceJSON  json.RawMessage `json:"provenance_json"`
	ClientMeta      map[string]any  `json:"client_meta"`
	VectorEmbedding []float32       `json:"vector_embedding"`
}

type statusSchemaCreateResponse struct {
	Status          string                       `json:"status"`
	ContractVersion string                       `json:"contract_version"`
	Proposal        statusSchemaProposalResponse `json:"proposal"`
	TruthBoundary   statusSchemaTruthBoundary    `json:"truth_boundary"`
	VectorPolicy    statusSchemaVectorPolicy     `json:"vector_policy"`
	VectorIndex     statusSchemaVectorIndex      `json:"vector_index"`
}

type statusSchemaReviewRequest struct {
	ProposalState   string         `json:"proposal_state"`
	State           string         `json:"state"`
	ReviewNote      string         `json:"review_note"`
	Reviewer        string         `json:"reviewer"`
	ClientMeta      map[string]any `json:"client_meta"`
	VectorEmbedding []float32      `json:"vector_embedding"`
}

type statusSchemaRegistryImportRequest struct {
	ClientMeta      map[string]any `json:"client_meta"`
	VectorEmbedding []float32      `json:"vector_embedding"`
}

type statusSchemaRegistryListResponse struct {
	Status          string                         `json:"status"`
	ContractVersion string                         `json:"contract_version"`
	ChatSessionID   string                         `json:"chat_session_id"`
	RegistryState   string                         `json:"registry_state,omitempty"`
	Definitions     []store.StatusSchemaDefinition `json:"definitions"`
	Counts          map[string]int                 `json:"counts"`
	RegistryPolicy  statusSchemaRegistryPolicy     `json:"registry_policy"`
}

type statusSchemaRegistryImportResponse struct {
	Status          string                         `json:"status"`
	ContractVersion string                         `json:"contract_version"`
	Proposal        statusSchemaProposalResponse   `json:"proposal"`
	Definitions     []store.StatusSchemaDefinition `json:"definitions"`
	RegistryPolicy  statusSchemaRegistryPolicy     `json:"registry_policy"`
	VectorIndexes   []statusSchemaVectorIndex      `json:"vector_indexes"`
}

type statusCurrentValueWriteRequest struct {
	ChatSessionID string          `json:"chat_session_id"`
	StatusKey     string          `json:"status_key"`
	OwnerScope    string          `json:"owner_scope"`
	OwnerID       string          `json:"owner_id"`
	OwnerLabel    string          `json:"owner_label"`
	ValueJSON     json.RawMessage `json:"value_json"`
	EvidenceJSON  json.RawMessage `json:"evidence_json"`
	SourceTurn    int             `json:"source_turn"`
}

type statusCurrentValueListResponse struct {
	Status          string                     `json:"status"`
	ContractVersion string                     `json:"contract_version"`
	ChatSessionID   string                     `json:"chat_session_id"`
	Values          []store.StatusCurrentValue `json:"values"`
	Counts          map[string]int             `json:"counts"`
	Policy          statusCurrentValuePolicy   `json:"policy"`
}

type statusCurrentValueWriteResponse struct {
	Status          string                       `json:"status"`
	ContractVersion string                       `json:"contract_version"`
	Definition      store.StatusSchemaDefinition `json:"definition"`
	Value           store.StatusCurrentValue     `json:"value"`
	Policy          statusCurrentValuePolicy     `json:"policy"`
}

type statusChangeEventWriteRequest struct {
	ChatSessionID     string          `json:"chat_session_id"`
	StatusValueID     int64           `json:"status_value_id"`
	StatusKey         string          `json:"status_key"`
	OwnerScope        string          `json:"owner_scope"`
	OwnerID           string          `json:"owner_id"`
	EventKind         string          `json:"event_kind"`
	PreviousValueJSON json.RawMessage `json:"previous_value_json"`
	NewValueJSON      json.RawMessage `json:"new_value_json"`
	EvidenceJSON      json.RawMessage `json:"evidence_json"`
	SourceTurn        int             `json:"source_turn"`
	StoryClockJSON    json.RawMessage `json:"story_clock_json"`
}

type statusChangeEventListResponse struct {
	Status          string                    `json:"status"`
	ContractVersion string                    `json:"contract_version"`
	ChatSessionID   string                    `json:"chat_session_id"`
	Events          []store.StatusChangeEvent `json:"events"`
	Counts          map[string]int            `json:"counts"`
	Policy          statusLifecyclePolicy     `json:"policy"`
}

type statusChangeEventWriteResponse struct {
	Status          string                       `json:"status"`
	ContractVersion string                       `json:"contract_version"`
	Definition      store.StatusSchemaDefinition `json:"definition"`
	Event           store.StatusChangeEvent      `json:"event"`
	Policy          statusLifecyclePolicy        `json:"policy"`
}

type statusEffectWriteRequest struct {
	ChatSessionID      string          `json:"chat_session_id"`
	StatusKey          string          `json:"status_key"`
	OwnerScope         string          `json:"owner_scope"`
	OwnerID            string          `json:"owner_id"`
	EffectKind         string          `json:"effect_kind"`
	EffectLabel        string          `json:"effect_label"`
	EffectPayloadJSON  json.RawMessage `json:"effect_payload_json"`
	EvidenceJSON       json.RawMessage `json:"evidence_json"`
	SourceTurn         int             `json:"source_turn"`
	StartClockJSON     json.RawMessage `json:"start_clock_json"`
	DurationJSON       json.RawMessage `json:"duration_json"`
	ExpiresAtClockJSON json.RawMessage `json:"expires_at_clock_json"`
	EffectState        string          `json:"effect_state"`
}

type statusEffectStateRequest struct {
	EffectState         string          `json:"effect_state"`
	ClearedEvidenceJSON json.RawMessage `json:"cleared_evidence_json"`
	ClearedTurn         int             `json:"cleared_turn"`
}

type statusEffectListResponse struct {
	Status          string                `json:"status"`
	ContractVersion string                `json:"contract_version"`
	ChatSessionID   string                `json:"chat_session_id"`
	Effects         []store.StatusEffect  `json:"effects"`
	Counts          map[string]int        `json:"counts"`
	Policy          statusLifecyclePolicy `json:"policy"`
}

type statusEffectWriteResponse struct {
	Status          string                       `json:"status"`
	ContractVersion string                       `json:"contract_version"`
	Definition      store.StatusSchemaDefinition `json:"definition"`
	Effect          store.StatusEffect           `json:"effect"`
	Policy          statusLifecyclePolicy        `json:"policy"`
}

type statusExternalRuntimeValue struct {
	StatusKey    string          `json:"status_key"`
	OwnerScope   string          `json:"owner_scope"`
	OwnerID      string          `json:"owner_id"`
	ValueJSON    json.RawMessage `json:"value_json"`
	EvidenceJSON json.RawMessage `json:"evidence_json"`
	RuntimeName  string          `json:"runtime_name"`
	UpdatedAt    string          `json:"updated_at"`
}

type statusExternalRuntimeProjection struct {
	StatusKey    string `json:"status_key"`
	OwnerScope   string `json:"owner_scope"`
	OwnerID      string `json:"owner_id"`
	ValueJSON    string `json:"value_json,omitempty"`
	EvidenceJSON string `json:"evidence_json,omitempty"`
	RuntimeName  string `json:"runtime_name,omitempty"`
	UpdatedAt    string `json:"updated_at,omitempty"`
}

type statusQueryRequest struct {
	ChatSessionID     string                       `json:"chat_session_id"`
	QueryText         string                       `json:"query_text"`
	StatusKey         string                       `json:"status_key"`
	CandidateKeys     []string                     `json:"candidate_status_keys"`
	OwnerScope        string                       `json:"owner_scope"`
	OwnerID           string                       `json:"owner_id"`
	AuthorityMode     string                       `json:"authority_mode"`
	ProjectionDensity string                       `json:"projection_density"`
	ExternalValues    []statusExternalRuntimeValue `json:"external_values"`
}

type statusProposalGate struct {
	Required              bool           `json:"required"`
	Reason                string         `json:"reason,omitempty"`
	SuggestedStatusKey    string         `json:"suggested_status_key,omitempty"`
	ProposalOnly          bool           `json:"proposal_only"`
	AutoCanonWriteAllowed bool           `json:"auto_canon_write_allowed"`
	ProposalTemplate      map[string]any `json:"proposal_template,omitempty"`
}

type statusProjectionItem struct {
	Definition      store.StatusSchemaDefinition     `json:"definition"`
	Value           *store.StatusCurrentValue        `json:"value,omitempty"`
	ExternalRuntime *statusExternalRuntimeProjection `json:"external_runtime,omitempty"`
	Effects         []store.StatusEffect             `json:"effects,omitempty"`
	AuthorityMode   string                           `json:"authority_mode"`
	ValueSource     string                           `json:"value_source"`
	Density         string                           `json:"density"`
	ProjectionText  string                           `json:"projection_text"`
}

type statusQueryResponse struct {
	Status          string                         `json:"status"`
	ContractVersion string                         `json:"contract_version"`
	ChatSessionID   string                         `json:"chat_session_id"`
	ResultState     string                         `json:"result_state"`
	Definitions     []store.StatusSchemaDefinition `json:"definitions"`
	Projection      []statusProjectionItem         `json:"projection"`
	ProposalGate    statusProposalGate             `json:"proposal_gate"`
	Policy          statusQueryProjectionPolicy    `json:"policy"`
}

type statusProjectionResponse struct {
	Status          string                      `json:"status"`
	ContractVersion string                      `json:"contract_version"`
	ChatSessionID   string                      `json:"chat_session_id"`
	Density         string                      `json:"density"`
	Projection      []statusProjectionItem      `json:"projection"`
	Counts          map[string]int              `json:"counts"`
	Policy          statusQueryProjectionPolicy `json:"policy"`
}

func (s *Server) registerStatusSchemaRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET "+statusSchemaListRoute, s.handleStatusSchemaList)
	mux.HandleFunc("POST "+statusSchemaCreateRoute, s.handleStatusSchemaCreate)
	mux.HandleFunc("PATCH "+statusSchemaReviewRoute, s.handleStatusSchemaReview)
	mux.HandleFunc("GET "+statusSchemaRegistryListRoute, s.handleStatusSchemaRegistryList)
	mux.HandleFunc("POST "+statusSchemaRegistryImportRoute, s.handleStatusSchemaRegistryImport)
	mux.HandleFunc("GET "+statusSchemaValueListRoute, s.handleStatusCurrentValueList)
	mux.HandleFunc("POST "+statusSchemaValueWriteRoute, s.handleStatusCurrentValueWrite)
	mux.HandleFunc("GET "+statusSchemaEventListRoute, s.handleStatusChangeEventList)
	mux.HandleFunc("POST "+statusSchemaEventWriteRoute, s.handleStatusChangeEventWrite)
	mux.HandleFunc("GET "+statusSchemaEffectListRoute, s.handleStatusEffectList)
	mux.HandleFunc("POST "+statusSchemaEffectWriteRoute, s.handleStatusEffectWrite)
	mux.HandleFunc("PATCH "+statusSchemaEffectStateRoute, s.handleStatusEffectState)
	mux.HandleFunc("POST "+statusSchemaQueryRoute, s.handleStatusQuery)
	mux.HandleFunc("GET "+statusSchemaProjectionRoute, s.handleStatusProjection)
}

func (s *Server) handleStatusSchemaList(w http.ResponseWriter, r *http.Request) {
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	state, err := statusSchemaNormalizeOptionalProposalState(firstNonEmptyQuery(r, "proposal_state", "state"))
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeMissingParam, err.Error())
		return
	}
	limit := statusSchemaBoundedLimit(r.URL.Query().Get("limit"), statusSchemaDefaultListLimit, statusSchemaMinListLimit, statusSchemaMaxListLimit)

	ps, ok := s.Store.(store.StatusSchemaProposalStore)
	if !ok {
		writeJSON(w, http.StatusOK, emptyStatusSchemaListResponse(sid, state))
		return
	}
	proposals, err := ps.ListStatusSchemaProposals(r.Context(), sid, state, limit)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeJSON(w, http.StatusOK, emptyStatusSchemaListResponse(sid, state))
			return
		}
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	resp := statusSchemaListResponse{
		Status:          "ok",
		ContractVersion: statusSchemaContractVersion,
		ChatSessionID:   sid,
		ProposalState:   state,
		Proposals:       make([]statusSchemaProposalResponse, 0, len(proposals)),
		TruthBoundary:   statusSchemaTruthBoundaryValue(),
		VectorPolicy:    statusSchemaVectorPolicyValue(),
	}
	for _, proposal := range proposals {
		resp.Proposals = append(resp.Proposals, statusSchemaProposalFromStore(proposal))
		resp.Counts.Total++
		switch proposal.ProposalState {
		case "pending_review":
			resp.Counts.PendingReview++
		case "approved":
			resp.Counts.Approved++
		case "rejected":
			resp.Counts.Rejected++
		case "needs_revision":
			resp.Counts.NeedsRevision++
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleStatusSchemaCreate(w http.ResponseWriter, r *http.Request) {
	ps, ok := s.Store.(store.StatusSchemaProposalStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "store_unavailable", "status schema proposal store is not available")
		return
	}
	var req statusSchemaCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	proposal, err := statusSchemaProposalFromCreateRequest(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeMissingParam, err.Error())
		return
	}
	saved, err := ps.SaveStatusSchemaProposal(r.Context(), proposal)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "status schema proposal store is not enabled")
			return
		}
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, statusSchemaCreateResponse{
		Status:          "ok",
		ContractVersion: statusSchemaContractVersion,
		Proposal:        statusSchemaProposalFromStore(saved),
		TruthBoundary:   statusSchemaTruthBoundaryValue(),
		VectorPolicy:    statusSchemaVectorPolicyValue(),
		VectorIndex:     s.indexStatusSchemaProposal(r.Context(), saved, req.ClientMeta, req.VectorEmbedding),
	})
}

func (s *Server) handleStatusSchemaReview(w http.ResponseWriter, r *http.Request) {
	ps, ok := s.Store.(store.StatusSchemaProposalStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "store_unavailable", "status schema proposal store is not available")
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("id")), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "valid id is required")
		return
	}
	var req statusSchemaReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	state, err := statusSchemaNormalizeReviewState(firstNonEmptyStringLocal(req.ProposalState, req.State))
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeMissingParam, err.Error())
		return
	}
	if err := ps.UpdateStatusSchemaProposalReview(r.Context(), id, state, req.ReviewNote, req.Reviewer); err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "status schema proposal store is not enabled")
			return
		}
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	vectorIndex := statusSchemaVectorIndex{
		Status:    "skipped",
		Attempted: false,
		Reason:    "proposal_reload_unavailable",
	}
	if proposal, err := ps.GetStatusSchemaProposal(r.Context(), id); err == nil {
		vectorIndex = s.indexStatusSchemaProposal(r.Context(), proposal, req.ClientMeta, req.VectorEmbedding)
	} else if errors.Is(err, store.ErrNotFound) {
		vectorIndex.Reason = "proposal_not_found_after_review"
	} else if err != nil {
		vectorIndex.Reason = "proposal_reload_error: " + err.Error()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"contract_version": statusSchemaContractVersion,
		"id":               id,
		"proposal_state":   state,
		"truth_boundary":   statusSchemaTruthBoundaryValue(),
		"vector_policy":    statusSchemaVectorPolicyValue(),
		"vector_index":     vectorIndex,
	})
}

func (s *Server) handleStatusSchemaRegistryList(w http.ResponseWriter, r *http.Request) {
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	state, err := statusSchemaNormalizeOptionalRegistryState(firstNonEmptyQuery(r, "registry_state", "state"))
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeMissingParam, err.Error())
		return
	}
	limit := statusSchemaBoundedLimit(r.URL.Query().Get("limit"), statusSchemaDefaultListLimit, statusSchemaMinListLimit, statusSchemaMaxListLimit)
	registry, ok := s.Store.(store.StatusSchemaRegistryStore)
	if !ok {
		writeJSON(w, http.StatusOK, statusSchemaEmptyRegistryListResponse(sid, state))
		return
	}
	definitions, err := registry.ListStatusSchemaDefinitions(r.Context(), sid, state, limit)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeJSON(w, http.StatusOK, statusSchemaEmptyRegistryListResponse(sid, state))
			return
		}
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	resp := statusSchemaRegistryListResponse{
		Status:          "ok",
		ContractVersion: statusSchemaRegistryContractVersion,
		ChatSessionID:   sid,
		RegistryState:   state,
		Definitions:     definitions,
		Counts:          statusSchemaRegistryCounts(definitions),
		RegistryPolicy:  statusSchemaRegistryPolicyValue(),
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleStatusSchemaRegistryImport(w http.ResponseWriter, r *http.Request) {
	proposalStore, ok := s.Store.(store.StatusSchemaProposalStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "store_unavailable", "status schema proposal store is not available")
		return
	}
	registry, ok := s.Store.(store.StatusSchemaRegistryStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "store_unavailable", "status schema registry store is not available")
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("id")), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "valid id is required")
		return
	}
	var req statusSchemaRegistryImportRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
			writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
			return
		}
	}
	proposal, err := proposalStore.GetStatusSchemaProposal(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "status schema proposal not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	definitions, err := statusSchemaDefinitionsFromApprovedProposal(proposal)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_schema_proposal", err.Error())
		return
	}
	saved, err := registry.SaveStatusSchemaDefinitions(r.Context(), definitions)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "status schema registry store is not enabled")
			return
		}
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	indexes := make([]statusSchemaVectorIndex, 0, len(saved))
	for _, definition := range saved {
		indexes = append(indexes, s.indexStatusSchemaDefinition(r.Context(), definition, req.ClientMeta, req.VectorEmbedding))
	}
	writeJSON(w, http.StatusCreated, statusSchemaRegistryImportResponse{
		Status:          "ok",
		ContractVersion: statusSchemaRegistryContractVersion,
		Proposal:        statusSchemaProposalFromStore(proposal),
		Definitions:     saved,
		RegistryPolicy:  statusSchemaRegistryPolicyValue(),
		VectorIndexes:   indexes,
	})
}

func (s *Server) handleStatusCurrentValueList(w http.ResponseWriter, r *http.Request) {
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	ownerScope := strings.TrimSpace(r.URL.Query().Get("owner_scope"))
	if ownerScope != "" {
		ownerScope = statusSchemaNormalizeOwnerScope(ownerScope)
		if ownerScope == "" {
			writeError(w, http.StatusBadRequest, CodeMissingParam, "owner_scope is invalid")
			return
		}
	}
	ownerID := strings.TrimSpace(r.URL.Query().Get("owner_id"))
	statusKey := strings.TrimSpace(r.URL.Query().Get("status_key"))
	if statusKey != "" && !statusSchemaValidKey(statusKey) {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "status_key is invalid")
		return
	}
	limit := statusSchemaBoundedLimit(r.URL.Query().Get("limit"), statusSchemaDefaultListLimit, statusSchemaMinListLimit, statusSchemaMaxListLimit)
	valueStore, ok := s.Store.(store.StatusCurrentValueStore)
	if !ok {
		writeJSON(w, http.StatusOK, statusCurrentValueEmptyListResponse(sid))
		return
	}
	values, err := valueStore.ListStatusCurrentValues(r.Context(), sid, ownerScope, ownerID, statusKey, limit)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeJSON(w, http.StatusOK, statusCurrentValueEmptyListResponse(sid))
			return
		}
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, statusCurrentValueListResponse{
		Status:          "ok",
		ContractVersion: statusSchemaValueContractVersion,
		ChatSessionID:   sid,
		Values:          values,
		Counts:          statusCurrentValueCounts(values),
		Policy:          statusCurrentValuePolicyValue(),
	})
}

func (s *Server) handleStatusCurrentValueWrite(w http.ResponseWriter, r *http.Request) {
	registry, ok := s.Store.(store.StatusSchemaRegistryStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "store_unavailable", "status schema registry store is not available")
		return
	}
	valueStore, ok := s.Store.(store.StatusCurrentValueStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "store_unavailable", "status current value store is not available")
		return
	}
	var req statusCurrentValueWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	definition, value, err := statusCurrentValueFromWriteRequest(r.Context(), registry, req)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusBadRequest, "status_schema_not_found", "active status schema definition not found")
			return
		}
		writeError(w, http.StatusBadRequest, CodeMissingParam, err.Error())
		return
	}
	saved, err := valueStore.SaveStatusCurrentValue(r.Context(), value)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "status current value store is not enabled")
			return
		}
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, statusCurrentValueWriteResponse{
		Status:          "ok",
		ContractVersion: statusSchemaValueContractVersion,
		Definition:      definition,
		Value:           saved,
		Policy:          statusCurrentValuePolicyValue(),
	})
}

func (s *Server) handleStatusChangeEventList(w http.ResponseWriter, r *http.Request) {
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	ownerScope, err := statusSchemaOptionalOwnerScope(r.URL.Query().Get("owner_scope"))
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeMissingParam, err.Error())
		return
	}
	statusKey := strings.TrimSpace(r.URL.Query().Get("status_key"))
	if statusKey != "" && !statusSchemaValidKey(statusKey) {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "status_key is invalid")
		return
	}
	limit := statusSchemaBoundedLimit(r.URL.Query().Get("limit"), statusSchemaDefaultListLimit, statusSchemaMinListLimit, statusSchemaMaxListLimit)
	lifecycle, ok := s.Store.(store.StatusLifecycleStore)
	if !ok {
		writeJSON(w, http.StatusOK, statusChangeEventEmptyListResponse(sid))
		return
	}
	events, err := lifecycle.ListStatusChangeEvents(r.Context(), sid, ownerScope, strings.TrimSpace(r.URL.Query().Get("owner_id")), statusKey, limit)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeJSON(w, http.StatusOK, statusChangeEventEmptyListResponse(sid))
			return
		}
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, statusChangeEventListResponse{
		Status:          "ok",
		ContractVersion: statusSchemaLifecycleContractVersion,
		ChatSessionID:   sid,
		Events:          events,
		Counts:          statusChangeEventCounts(events),
		Policy:          statusLifecyclePolicyValue(),
	})
}

func (s *Server) handleStatusChangeEventWrite(w http.ResponseWriter, r *http.Request) {
	registry, ok := s.Store.(store.StatusSchemaRegistryStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "store_unavailable", "status schema registry store is not available")
		return
	}
	lifecycle, ok := s.Store.(store.StatusLifecycleStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "store_unavailable", "status lifecycle store is not available")
		return
	}
	var req statusChangeEventWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	definition, event, err := statusChangeEventFromWriteRequest(r.Context(), registry, req)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusBadRequest, "status_schema_not_found", "active status schema definition not found")
			return
		}
		writeError(w, http.StatusBadRequest, CodeMissingParam, err.Error())
		return
	}
	saved, err := lifecycle.SaveStatusChangeEvent(r.Context(), event)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "status lifecycle store is not enabled")
			return
		}
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, statusChangeEventWriteResponse{
		Status:          "ok",
		ContractVersion: statusSchemaLifecycleContractVersion,
		Definition:      definition,
		Event:           saved,
		Policy:          statusLifecyclePolicyValue(),
	})
}

func (s *Server) handleStatusEffectList(w http.ResponseWriter, r *http.Request) {
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	ownerScope, err := statusSchemaOptionalOwnerScope(r.URL.Query().Get("owner_scope"))
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeMissingParam, err.Error())
		return
	}
	effectState, err := statusNormalizeOptionalEffectState(r.URL.Query().Get("effect_state"))
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeMissingParam, err.Error())
		return
	}
	limit := statusSchemaBoundedLimit(r.URL.Query().Get("limit"), statusSchemaDefaultListLimit, statusSchemaMinListLimit, statusSchemaMaxListLimit)
	lifecycle, ok := s.Store.(store.StatusLifecycleStore)
	if !ok {
		writeJSON(w, http.StatusOK, statusEffectEmptyListResponse(sid))
		return
	}
	effects, err := lifecycle.ListStatusEffects(r.Context(), sid, ownerScope, strings.TrimSpace(r.URL.Query().Get("owner_id")), effectState, limit)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeJSON(w, http.StatusOK, statusEffectEmptyListResponse(sid))
			return
		}
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, statusEffectListResponse{
		Status:          "ok",
		ContractVersion: statusSchemaLifecycleContractVersion,
		ChatSessionID:   sid,
		Effects:         effects,
		Counts:          statusEffectCounts(effects),
		Policy:          statusLifecyclePolicyValue(),
	})
}

func (s *Server) handleStatusEffectWrite(w http.ResponseWriter, r *http.Request) {
	registry, ok := s.Store.(store.StatusSchemaRegistryStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "store_unavailable", "status schema registry store is not available")
		return
	}
	lifecycle, ok := s.Store.(store.StatusLifecycleStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "store_unavailable", "status lifecycle store is not available")
		return
	}
	var req statusEffectWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	definition, effect, err := statusEffectFromWriteRequest(r.Context(), registry, req)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusBadRequest, "status_schema_not_found", "active status schema definition not found")
			return
		}
		writeError(w, http.StatusBadRequest, CodeMissingParam, err.Error())
		return
	}
	saved, err := lifecycle.SaveStatusEffect(r.Context(), effect)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "status lifecycle store is not enabled")
			return
		}
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, statusEffectWriteResponse{
		Status:          "ok",
		ContractVersion: statusSchemaLifecycleContractVersion,
		Definition:      definition,
		Effect:          saved,
		Policy:          statusLifecyclePolicyValue(),
	})
}

func (s *Server) handleStatusEffectState(w http.ResponseWriter, r *http.Request) {
	lifecycle, ok := s.Store.(store.StatusLifecycleStore)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, "store_unavailable", "status lifecycle store is not available")
		return
	}
	id, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("id")), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "valid id is required")
		return
	}
	var req statusEffectStateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	state, evidenceJSON, err := statusEffectStateUpdateFromRequest(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeMissingParam, err.Error())
		return
	}
	if err := lifecycle.UpdateStatusEffectState(r.Context(), id, state, evidenceJSON, req.ClearedTurn); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "status effect not found")
			return
		}
		if errors.Is(err, store.ErrNotEnabled) {
			writeError(w, http.StatusServiceUnavailable, "store_unavailable", "status lifecycle store is not enabled")
			return
		}
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"contract_version": statusSchemaLifecycleContractVersion,
		"id":               id,
		"effect_state":     state,
		"policy":           statusLifecyclePolicyValue(),
	})
}

func (s *Server) handleStatusQuery(w http.ResponseWriter, r *http.Request) {
	var req statusQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	sid := strings.TrimSpace(req.ChatSessionID)
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	ownerScope, err := statusSchemaOptionalOwnerScope(req.OwnerScope)
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeMissingParam, err.Error())
		return
	}
	req.OwnerScope = ownerScope
	req.OwnerID = strings.TrimSpace(req.OwnerID)
	req.StatusKey = strings.TrimSpace(req.StatusKey)
	if req.StatusKey != "" && !statusSchemaValidKey(req.StatusKey) {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "status_key is invalid")
		return
	}
	authorityMode, err := statusNormalizeAuthorityMode(req.AuthorityMode)
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeMissingParam, err.Error())
		return
	}
	req.AuthorityMode = authorityMode
	density, err := statusNormalizeProjectionDensity(firstNonEmptyStringLocal(req.ProjectionDensity, "full"))
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeMissingParam, err.Error())
		return
	}
	registry, ok := s.Store.(store.StatusSchemaRegistryStore)
	if !ok {
		writeJSON(w, http.StatusOK, statusQueryProposalGateResponse(sid, req, "registry_unavailable"))
		return
	}
	definitions, err := registry.ListStatusSchemaDefinitions(r.Context(), sid, "active", statusSchemaMaxListLimit)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeJSON(w, http.StatusOK, statusQueryProposalGateResponse(sid, req, "registry_unavailable"))
			return
		}
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	matches := statusMatchDefinitions(definitions, req.StatusKey, req.CandidateKeys, req.QueryText, req.OwnerScope)
	if len(matches) == 0 {
		writeJSON(w, http.StatusOK, statusQueryProposalGateResponse(sid, req, "status_schema_not_registered"))
		return
	}
	projection := s.buildStatusProjection(r.Context(), sid, matches, req.OwnerScope, req.OwnerID, req.StatusKey, authorityMode, density, req.ExternalValues, true)
	resultState := statusQueryResultState(projection)
	writeJSON(w, http.StatusOK, statusQueryResponse{
		Status:          "ok",
		ContractVersion: statusSchemaQueryContractVersion,
		ChatSessionID:   sid,
		ResultState:     resultState,
		Definitions:     matches,
		Projection:      projection,
		ProposalGate:    statusProposalGate{Required: false, ProposalOnly: true, AutoCanonWriteAllowed: false},
		Policy:          statusQueryProjectionPolicyValue(),
	})
}

func (s *Server) handleStatusProjection(w http.ResponseWriter, r *http.Request) {
	sid := strings.TrimSpace(r.PathValue("chat_session_id"))
	if sid == "" {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "chat_session_id is required")
		return
	}
	ownerScope, err := statusSchemaOptionalOwnerScope(r.URL.Query().Get("owner_scope"))
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeMissingParam, err.Error())
		return
	}
	statusKey := strings.TrimSpace(r.URL.Query().Get("status_key"))
	if statusKey != "" && !statusSchemaValidKey(statusKey) {
		writeError(w, http.StatusBadRequest, CodeMissingParam, "status_key is invalid")
		return
	}
	density, err := statusNormalizeProjectionDensity(firstNonEmptyStringLocal(r.URL.Query().Get("density"), "auto"))
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeMissingParam, err.Error())
		return
	}
	authorityMode, err := statusNormalizeAuthorityMode(r.URL.Query().Get("authority_mode"))
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeMissingParam, err.Error())
		return
	}
	registry, ok := s.Store.(store.StatusSchemaRegistryStore)
	if !ok {
		writeJSON(w, http.StatusOK, statusProjectionResponse{
			Status:          "ok",
			ContractVersion: statusSchemaQueryContractVersion,
			ChatSessionID:   sid,
			Density:         density,
			Projection:      []statusProjectionItem{},
			Counts:          map[string]int{"total": 0},
			Policy:          statusQueryProjectionPolicyValue(),
		})
		return
	}
	definitions, err := registry.ListStatusSchemaDefinitions(r.Context(), sid, "active", statusSchemaMaxListLimit)
	if err != nil {
		if errors.Is(err, store.ErrNotEnabled) {
			writeJSON(w, http.StatusOK, statusProjectionResponse{
				Status:          "ok",
				ContractVersion: statusSchemaQueryContractVersion,
				ChatSessionID:   sid,
				Density:         density,
				Projection:      []statusProjectionItem{},
				Counts:          map[string]int{"total": 0},
				Policy:          statusQueryProjectionPolicyValue(),
			})
			return
		}
		writeError(w, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	matches := statusFilterDefinitions(definitions, statusKey, ownerScope)
	projection := s.buildStatusProjection(r.Context(), sid, matches, ownerScope, strings.TrimSpace(r.URL.Query().Get("owner_id")), statusKey, authorityMode, density, nil, false)
	writeJSON(w, http.StatusOK, statusProjectionResponse{
		Status:          "ok",
		ContractVersion: statusSchemaQueryContractVersion,
		ChatSessionID:   sid,
		Density:         density,
		Projection:      projection,
		Counts:          statusProjectionCounts(projection),
		Policy:          statusQueryProjectionPolicyValue(),
	})
}

func statusSchemaProposalFromCreateRequest(req statusSchemaCreateRequest) (store.StatusSchemaProposal, error) {
	sid := strings.TrimSpace(req.ChatSessionID)
	if sid == "" {
		return store.StatusSchemaProposal{}, errors.New("chat_session_id is required")
	}
	channel := statusSchemaNormalizeInputChannel(req.InputChannel)
	if channel == "" {
		return store.StatusSchemaProposal{}, errors.New("input_channel must be one of bootstrap, direct_json, portable_import")
	}
	schemaName := strings.TrimSpace(req.SchemaName)
	if schemaName == "" {
		return store.StatusSchemaProposal{}, errors.New("schema_name is required")
	}
	schemaJSON, err := statusSchemaCompactJSONObject(req.SchemaJSON, "schema_json")
	if err != nil {
		return store.StatusSchemaProposal{}, err
	}
	provenanceJSON, err := statusSchemaCompactOptionalJSONObject(req.ProvenanceJSON, "provenance_json")
	if err != nil {
		return store.StatusSchemaProposal{}, err
	}
	if channel == "portable_import" && provenanceJSON == "" {
		return store.StatusSchemaProposal{}, errors.New("provenance_json is required for portable_import")
	}
	return store.StatusSchemaProposal{
		ChatSessionID:  sid,
		InputChannel:   channel,
		ProposalState:  "pending_review",
		SchemaName:     schemaName,
		RulesetLabel:   strings.TrimSpace(req.RulesetLabel),
		SchemaJSON:     schemaJSON,
		ProvenanceJSON: provenanceJSON,
	}, nil
}

func statusSchemaProposalFromStore(item store.StatusSchemaProposal) statusSchemaProposalResponse {
	return statusSchemaProposalResponse{
		ID:             item.ID,
		ChatSessionID:  item.ChatSessionID,
		InputChannel:   item.InputChannel,
		ProposalState:  item.ProposalState,
		SchemaName:     item.SchemaName,
		RulesetLabel:   item.RulesetLabel,
		SchemaJSON:     item.SchemaJSON,
		ProvenanceJSON: item.ProvenanceJSON,
		ReviewNote:     item.ReviewNote,
		Reviewer:       item.Reviewer,
		ReviewedAt:     item.ReviewedAt,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
	}
}

func statusSchemaTruthBoundaryValue() statusSchemaTruthBoundary {
	return statusSchemaTruthBoundary{
		ProposalOnly:                 true,
		CanonicalStatusWriter:        false,
		ReviewRequiredBeforeCanon:    true,
		ApprovalRegistersSchema:      false,
		CurrentValueWritesAllowed:    false,
		EffectLifecycleWritesAllowed: false,
		ArbitraryCodeFormulaAllowed:  false,
		AcceptedInputChannels:        []string{"bootstrap", "direct_json", "portable_import"},
		AcceptedReviewStates:         []string{"approved", "rejected", "needs_revision"},
	}
}

func statusSchemaVectorPolicyValue() statusSchemaVectorPolicy {
	return statusSchemaVectorPolicy{
		ChromaLinked:          true,
		VectorLane:            "chroma_support_only",
		Tier:                  "status_schema_proposal",
		SourceTable:           "status_schema_proposals",
		HydrateRequired:       true,
		CanonicalTruthSource:  "mariadb.status_schema_proposals",
		TruthWriter:           false,
		IndexPendingProposals: true,
		IndexReviewedStates:   true,
	}
}

func statusSchemaRegistryPolicyValue() statusSchemaRegistryPolicy {
	return statusSchemaRegistryPolicy{
		CanonicalSchemaRegistry:     true,
		RequiresApprovedProposal:    true,
		CurrentValueWritesAllowed:   false,
		EffectLifecycleAllowed:      false,
		HardcodedStatusNamesAllowed: false,
		AcceptedOwnerScopes:         []string{"character", "party", "faction", "world", "entity", "session"},
		AcceptedValueKinds:          []string{"scalar", "resource", "enum", "boolean", "clock", "tags", "note", "derived"},
		VectorLane:                  "chroma_support_only",
		VectorTier:                  "status_schema_definition",
		VectorSourceTable:           "status_schema_registry",
		HydrateRequired:             true,
	}
}

func statusCurrentValuePolicyValue() statusCurrentValuePolicy {
	return statusCurrentValuePolicy{
		CanonicalCurrentValueWriter: true,
		RequiresActiveRegistry:      true,
		EvidenceRequired:            true,
		HistoryWritesAllowed:        false,
		EffectLifecycleAllowed:      false,
		VectorTruthWriter:           false,
		DirectDerivedWritesAllowed:  false,
		AcceptedOwnerScopes:         []string{"character", "party", "faction", "world", "entity", "session"},
		AcceptedValueKinds:          []string{"scalar", "resource", "enum", "boolean", "clock", "tags", "note"},
		CanonicalTruthSource:        "mariadb.status_current_values",
	}
}

func statusLifecyclePolicyValue() statusLifecyclePolicy {
	return statusLifecyclePolicy{
		ChangeEventLedgerWriter:      true,
		EffectLifecycleWriter:        true,
		RequiresActiveRegistry:       true,
		EvidenceRequired:             true,
		RequiresStoryClockForEffects: true,
		CurrentValueMutationAllowed:  false,
		VectorTruthWriter:            false,
		AcceptedEventKinds:           []string{"set", "increase", "decrease", "clear", "effect_applied", "effect_expired", "effect_cleared"},
		AcceptedEffectKinds:          []string{"temporary_effect", "buff", "debuff", "injury", "cooldown"},
		AcceptedEffectStates:         []string{"pending", "active", "expired", "cleared"},
		CanonicalEventSource:         "mariadb.status_change_events",
		CanonicalEffectSource:        "mariadb.status_effects",
	}
}

func statusQueryProjectionPolicyValue() statusQueryProjectionPolicy {
	return statusQueryProjectionPolicy{
		CanonFirstQuery:                   true,
		SemanticMemoryFallbackAsTruth:     false,
		ExternalRuntimeAuthoritySupported: true,
		ExternalRuntimeOverridesArchive:   true,
		UnknownStatusCreatesCanon:         false,
		UnknownStatusProposalOnly:         true,
		VectorTruthWriter:                 false,
		AcceptedAuthorityModes:            []string{"auto", "archive_canonical", "external_runtime"},
		AcceptedProjectionDensities:       []string{"auto", "full", "light"},
		CanonicalValueSource:              "mariadb.status_current_values",
		CanonicalEffectSource:             "mariadb.status_effects",
	}
}

func emptyStatusSchemaListResponse(sid, state string) statusSchemaListResponse {
	return statusSchemaListResponse{
		Status:          "ok",
		ContractVersion: statusSchemaContractVersion,
		ChatSessionID:   sid,
		ProposalState:   state,
		Proposals:       []statusSchemaProposalResponse{},
		Counts:          statusSchemaCounts{},
		TruthBoundary:   statusSchemaTruthBoundaryValue(),
		VectorPolicy:    statusSchemaVectorPolicyValue(),
	}
}

func statusSchemaEmptyRegistryListResponse(sid, state string) statusSchemaRegistryListResponse {
	return statusSchemaRegistryListResponse{
		Status:          "ok",
		ContractVersion: statusSchemaRegistryContractVersion,
		ChatSessionID:   sid,
		RegistryState:   state,
		Definitions:     []store.StatusSchemaDefinition{},
		Counts:          map[string]int{"total": 0},
		RegistryPolicy:  statusSchemaRegistryPolicyValue(),
	}
}

func statusCurrentValueEmptyListResponse(sid string) statusCurrentValueListResponse {
	return statusCurrentValueListResponse{
		Status:          "ok",
		ContractVersion: statusSchemaValueContractVersion,
		ChatSessionID:   sid,
		Values:          []store.StatusCurrentValue{},
		Counts:          map[string]int{"total": 0},
		Policy:          statusCurrentValuePolicyValue(),
	}
}

func statusChangeEventEmptyListResponse(sid string) statusChangeEventListResponse {
	return statusChangeEventListResponse{
		Status:          "ok",
		ContractVersion: statusSchemaLifecycleContractVersion,
		ChatSessionID:   sid,
		Events:          []store.StatusChangeEvent{},
		Counts:          map[string]int{"total": 0},
		Policy:          statusLifecyclePolicyValue(),
	}
}

func statusEffectEmptyListResponse(sid string) statusEffectListResponse {
	return statusEffectListResponse{
		Status:          "ok",
		ContractVersion: statusSchemaLifecycleContractVersion,
		ChatSessionID:   sid,
		Effects:         []store.StatusEffect{},
		Counts:          map[string]int{"total": 0},
		Policy:          statusLifecyclePolicyValue(),
	}
}

func statusSchemaRegistryCounts(definitions []store.StatusSchemaDefinition) map[string]int {
	counts := map[string]int{"total": len(definitions)}
	for _, definition := range definitions {
		state := strings.TrimSpace(definition.RegistryState)
		if state == "" {
			state = "active"
		}
		counts[state]++
	}
	return counts
}

func statusCurrentValueCounts(values []store.StatusCurrentValue) map[string]int {
	counts := map[string]int{"total": len(values)}
	for _, value := range values {
		scope := strings.TrimSpace(value.OwnerScope)
		if scope == "" {
			scope = "unknown"
		}
		counts["owner_scope:"+scope]++
	}
	return counts
}

func statusChangeEventCounts(events []store.StatusChangeEvent) map[string]int {
	counts := map[string]int{"total": len(events)}
	for _, event := range events {
		kind := strings.TrimSpace(event.EventKind)
		if kind == "" {
			kind = "unknown"
		}
		counts["event_kind:"+kind]++
	}
	return counts
}

func statusEffectCounts(effects []store.StatusEffect) map[string]int {
	counts := map[string]int{"total": len(effects)}
	for _, effect := range effects {
		state := strings.TrimSpace(effect.EffectState)
		if state == "" {
			state = "active"
		}
		counts[state]++
	}
	return counts
}

func statusProjectionCounts(items []statusProjectionItem) map[string]int {
	counts := map[string]int{"total": len(items)}
	for _, item := range items {
		counts["density:"+item.Density]++
		counts["authority:"+item.AuthorityMode]++
		counts["source:"+item.ValueSource]++
	}
	return counts
}

func statusQueryResultState(items []statusProjectionItem) string {
	if len(items) == 0 {
		return "not_found"
	}
	hasAnswered := false
	hasExternalRequired := false
	hasMissingArchive := false
	for _, item := range items {
		switch item.ValueSource {
		case "archive_current", "external_runtime":
			hasAnswered = true
		case "external_value_required":
			hasExternalRequired = true
		case "archive_value_missing":
			hasMissingArchive = true
		}
	}
	if hasAnswered {
		return "answered"
	}
	if hasExternalRequired {
		return "external_value_required"
	}
	if hasMissingArchive {
		return "archive_value_missing"
	}
	return "not_found"
}

func statusQueryProposalGateResponse(sid string, req statusQueryRequest, reason string) statusQueryResponse {
	suggestedKey := statusSuggestedProposalKey(req)
	return statusQueryResponse{
		Status:          "ok",
		ContractVersion: statusSchemaQueryContractVersion,
		ChatSessionID:   sid,
		ResultState:     "proposal_required",
		Definitions:     []store.StatusSchemaDefinition{},
		Projection:      []statusProjectionItem{},
		ProposalGate: statusProposalGate{
			Required:              true,
			Reason:                reason,
			SuggestedStatusKey:    suggestedKey,
			ProposalOnly:          true,
			AutoCanonWriteAllowed: false,
			ProposalTemplate:      statusProposalTemplate(req, suggestedKey),
		},
		Policy: statusQueryProjectionPolicyValue(),
	}
}

func statusSuggestedProposalKey(req statusQueryRequest) string {
	if statusSchemaValidKey(strings.TrimSpace(req.StatusKey)) {
		return strings.TrimSpace(req.StatusKey)
	}
	for _, key := range req.CandidateKeys {
		if statusSchemaValidKey(strings.TrimSpace(key)) {
			return strings.TrimSpace(key)
		}
	}
	return ""
}

func statusProposalTemplate(req statusQueryRequest, key string) map[string]any {
	if key == "" {
		return nil
	}
	ownerScope := strings.TrimSpace(req.OwnerScope)
	if ownerScope == "" {
		ownerScope = "<review_required>"
	}
	return map[string]any{
		"input_channel":   "direct_json",
		"schema_name":     "status_schema",
		"review_required": true,
		"schema_json": map[string]any{
			"stats": []map[string]any{
				{
					"status_key":  key,
					"label":       key,
					"owner_scope": ownerScope,
					"value_kind":  "<review_required>",
					"note":        "Query saw an unregistered status key; review schema before import.",
				},
			},
		},
	}
}

func (s *Server) buildStatusProjection(ctx context.Context, sid string, definitions []store.StatusSchemaDefinition, ownerScope, ownerID, statusKey, requestedAuthority, requestedDensity string, externalValues []statusExternalRuntimeValue, queryFocused bool) []statusProjectionItem {
	values := statusLoadCurrentValues(ctx, s.Store, sid, ownerScope, ownerID, statusKey)
	effects := statusLoadActiveEffects(ctx, s.Store, sid, ownerScope, ownerID)
	external := statusNormalizeExternalValues(externalValues)
	out := make([]statusProjectionItem, 0, len(definitions))
	for _, definition := range definitions {
		authority := statusDefinitionAuthorityMode(definition, requestedAuthority)
		density := statusDefinitionProjectionDensity(definition, requestedDensity, queryFocused)
		if authority == "external_runtime" {
			matches := statusExternalValuesForDefinition(external, definition, ownerID)
			if len(matches) == 0 {
				if queryFocused {
					out = append(out, statusProjectionItem{
						Definition:     definition,
						AuthorityMode:  authority,
						ValueSource:    "external_value_required",
						Density:        density,
						ProjectionText: statusProjectionText(definition, nil, nil, nil, "external_value_required", density),
					})
				}
				continue
			}
			for _, ext := range matches {
				extCopy := ext
				item := statusProjectionItem{
					Definition:     definition,
					AuthorityMode:  authority,
					ValueSource:    "external_runtime",
					Density:        density,
					ProjectionText: statusProjectionText(definition, nil, &extCopy, nil, "external_runtime", density),
				}
				if density == "full" {
					item.ExternalRuntime = &extCopy
				}
				out = append(out, item)
			}
			continue
		}
		matchedValues := statusValuesForDefinition(values, definition, ownerID)
		if len(matchedValues) == 0 {
			if queryFocused {
				out = append(out, statusProjectionItem{
					Definition:     definition,
					AuthorityMode:  "archive_canonical",
					ValueSource:    "archive_value_missing",
					Density:        density,
					ProjectionText: statusProjectionText(definition, nil, nil, nil, "archive_value_missing", density),
				})
			}
			continue
		}
		for _, value := range matchedValues {
			valueCopy := value
			effectMatches := statusEffectsForValue(effects, valueCopy)
			item := statusProjectionItem{
				Definition:     definition,
				AuthorityMode:  "archive_canonical",
				ValueSource:    "archive_current",
				Density:        density,
				ProjectionText: statusProjectionText(definition, &valueCopy, nil, effectMatches, "archive_current", density),
			}
			if density == "full" {
				item.Value = &valueCopy
				item.Effects = effectMatches
			}
			out = append(out, item)
		}
	}
	return out
}

func statusLoadCurrentValues(ctx context.Context, st store.Store, sid, ownerScope, ownerID, statusKey string) []store.StatusCurrentValue {
	valueStore, ok := st.(store.StatusCurrentValueStore)
	if !ok {
		return nil
	}
	values, err := valueStore.ListStatusCurrentValues(ctx, sid, ownerScope, ownerID, statusKey, statusSchemaMaxListLimit)
	if err != nil {
		return nil
	}
	return values
}

func statusLoadActiveEffects(ctx context.Context, st store.Store, sid, ownerScope, ownerID string) []store.StatusEffect {
	lifecycle, ok := st.(store.StatusLifecycleStore)
	if !ok {
		return nil
	}
	effects, err := lifecycle.ListStatusEffects(ctx, sid, ownerScope, ownerID, "active", statusSchemaMaxListLimit)
	if err != nil {
		return nil
	}
	return effects
}

func statusMatchDefinitions(definitions []store.StatusSchemaDefinition, statusKey string, candidateKeys []string, queryText, ownerScope string) []store.StatusSchemaDefinition {
	if strings.TrimSpace(statusKey) != "" {
		return statusFilterDefinitions(definitions, statusKey, ownerScope)
	}
	candidates := map[string]bool{}
	for _, key := range candidateKeys {
		key = strings.TrimSpace(key)
		if statusSchemaValidKey(key) {
			candidates[strings.ToLower(key)] = true
		}
	}
	query := strings.ToLower(strings.TrimSpace(queryText))
	out := make([]store.StatusSchemaDefinition, 0, len(definitions))
	for _, definition := range definitions {
		if ownerScope != "" && definition.OwnerScope != ownerScope {
			continue
		}
		if candidates[strings.ToLower(definition.StatusKey)] {
			out = append(out, definition)
			continue
		}
		if query != "" {
			key := strings.ToLower(strings.TrimSpace(definition.StatusKey))
			label := strings.ToLower(strings.TrimSpace(definition.Label))
			if (key != "" && strings.Contains(query, key)) || (label != "" && strings.Contains(query, label)) {
				out = append(out, definition)
			}
		}
	}
	return out
}

func statusFilterDefinitions(definitions []store.StatusSchemaDefinition, statusKey, ownerScope string) []store.StatusSchemaDefinition {
	out := make([]store.StatusSchemaDefinition, 0, len(definitions))
	for _, definition := range definitions {
		if statusKey != "" && definition.StatusKey != statusKey {
			continue
		}
		if ownerScope != "" && definition.OwnerScope != ownerScope {
			continue
		}
		out = append(out, definition)
	}
	return out
}

func statusNormalizeExternalValues(values []statusExternalRuntimeValue) []statusExternalRuntimeProjection {
	out := make([]statusExternalRuntimeProjection, 0, len(values))
	for _, value := range values {
		statusKey := strings.TrimSpace(value.StatusKey)
		ownerScope := statusSchemaNormalizeOwnerScope(value.OwnerScope)
		ownerID := strings.TrimSpace(value.OwnerID)
		if !statusSchemaValidKey(statusKey) || ownerScope == "" || ownerID == "" {
			continue
		}
		valueJSON, err := statusSchemaCompactRawJSON(value.ValueJSON, "value_json")
		if err != nil {
			continue
		}
		evidenceJSON, _ := statusSchemaCompactOptionalRawJSON(value.EvidenceJSON, "evidence_json")
		out = append(out, statusExternalRuntimeProjection{
			StatusKey:    statusKey,
			OwnerScope:   ownerScope,
			OwnerID:      ownerID,
			ValueJSON:    valueJSON,
			EvidenceJSON: evidenceJSON,
			RuntimeName:  strings.TrimSpace(value.RuntimeName),
			UpdatedAt:    strings.TrimSpace(value.UpdatedAt),
		})
	}
	return out
}

func statusValuesForDefinition(values []store.StatusCurrentValue, definition store.StatusSchemaDefinition, ownerID string) []store.StatusCurrentValue {
	out := make([]store.StatusCurrentValue, 0, len(values))
	for _, value := range values {
		if value.StatusKey != definition.StatusKey || value.OwnerScope != definition.OwnerScope {
			continue
		}
		if ownerID != "" && value.OwnerID != ownerID {
			continue
		}
		out = append(out, value)
	}
	return out
}

func statusEffectsForValue(effects []store.StatusEffect, value store.StatusCurrentValue) []store.StatusEffect {
	out := make([]store.StatusEffect, 0, len(effects))
	for _, effect := range effects {
		if effect.StatusKey == value.StatusKey && effect.OwnerScope == value.OwnerScope && effect.OwnerID == value.OwnerID {
			out = append(out, effect)
		}
	}
	return out
}

func statusExternalValuesForDefinition(values []statusExternalRuntimeProjection, definition store.StatusSchemaDefinition, ownerID string) []statusExternalRuntimeProjection {
	out := make([]statusExternalRuntimeProjection, 0, len(values))
	for _, value := range values {
		if value.StatusKey != definition.StatusKey || value.OwnerScope != definition.OwnerScope {
			continue
		}
		if ownerID != "" && value.OwnerID != ownerID {
			continue
		}
		out = append(out, value)
	}
	return out
}

func statusDefinitionAuthorityMode(definition store.StatusSchemaDefinition, requested string) string {
	requested = strings.TrimSpace(requested)
	if requested == "archive_canonical" || requested == "external_runtime" {
		return requested
	}
	options := statusDefinitionOptions(definition)
	if statusOptionBool(options, "external_runtime_authority") {
		return "external_runtime"
	}
	for _, key := range []string{"authority_mode", "value_authority", "runtime_authority"} {
		value := strings.ToLower(strings.TrimSpace(statusOptionString(options, key)))
		if value == "external_runtime" || value == "lua" || value == "lua_runtime" {
			return "external_runtime"
		}
	}
	return "archive_canonical"
}

func statusDefinitionProjectionDensity(definition store.StatusSchemaDefinition, requested string, queryFocused bool) string {
	requested = strings.TrimSpace(requested)
	if requested == "full" || requested == "light" {
		return requested
	}
	if queryFocused {
		return "full"
	}
	options := statusDefinitionOptions(definition)
	optionDensity := strings.ToLower(strings.TrimSpace(statusOptionString(options, "projection_density")))
	if optionDensity == "full" || optionDensity == "light" {
		return optionDensity
	}
	if statusOptionBool(options, "scene_blocking") || statusOptionBool(options, "critical") {
		return "full"
	}
	return "light"
}

func statusDefinitionOptions(definition store.StatusSchemaDefinition) map[string]any {
	raw := strings.TrimSpace(definition.OptionsJSON)
	if raw == "" {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return out
}

func statusOptionString(options map[string]any, key string) string {
	if len(options) == 0 {
		return ""
	}
	switch value := options[key].(type) {
	case string:
		return value
	default:
		return ""
	}
}

func statusOptionBool(options map[string]any, key string) bool {
	if len(options) == 0 {
		return false
	}
	switch value := options[key].(type) {
	case bool:
		return value
	case string:
		return strings.EqualFold(value, "true") || strings.EqualFold(value, "yes")
	default:
		return false
	}
}

func statusProjectionText(definition store.StatusSchemaDefinition, value *store.StatusCurrentValue, external *statusExternalRuntimeProjection, effects []store.StatusEffect, source, density string) string {
	key := strings.TrimSpace(definition.StatusKey)
	label := strings.TrimSpace(definition.Label)
	if label == "" {
		label = key
	}
	switch source {
	case "archive_current":
		if value == nil {
			return label + ": archive current value unavailable"
		}
		if density == "light" {
			return label + ": current value available; active_effects=" + strconv.Itoa(len(effects))
		}
		return label + ": " + value.ValueJSON + "; active_effects=" + strconv.Itoa(len(effects))
	case "external_runtime":
		if external == nil {
			return label + ": external runtime value unavailable"
		}
		if density == "light" {
			return label + ": external runtime value available"
		}
		return label + ": " + external.ValueJSON + " (external_runtime)"
	case "external_value_required":
		return label + ": delegated to external runtime; value not supplied"
	case "archive_value_missing":
		return label + ": registered but no archive current value"
	default:
		return label + ": status projection unavailable"
	}
}

func statusCurrentValueFromWriteRequest(ctx context.Context, registry store.StatusSchemaRegistryStore, req statusCurrentValueWriteRequest) (store.StatusSchemaDefinition, store.StatusCurrentValue, error) {
	sid := strings.TrimSpace(req.ChatSessionID)
	if sid == "" {
		return store.StatusSchemaDefinition{}, store.StatusCurrentValue{}, errors.New("chat_session_id is required")
	}
	statusKey := strings.TrimSpace(req.StatusKey)
	if !statusSchemaValidKey(statusKey) {
		return store.StatusSchemaDefinition{}, store.StatusCurrentValue{}, errors.New("status_key is invalid")
	}
	ownerScope := statusSchemaNormalizeOwnerScope(req.OwnerScope)
	if ownerScope == "" {
		return store.StatusSchemaDefinition{}, store.StatusCurrentValue{}, errors.New("owner_scope is invalid")
	}
	ownerID := strings.TrimSpace(req.OwnerID)
	if ownerID == "" {
		return store.StatusSchemaDefinition{}, store.StatusCurrentValue{}, errors.New("owner_id is required")
	}
	valueJSON, err := statusSchemaCompactRawJSON(req.ValueJSON, "value_json")
	if err != nil {
		return store.StatusSchemaDefinition{}, store.StatusCurrentValue{}, err
	}
	evidenceJSON, err := statusSchemaCompactRawJSON(req.EvidenceJSON, "evidence_json")
	if err != nil {
		return store.StatusSchemaDefinition{}, store.StatusCurrentValue{}, err
	}
	definition, err := registry.GetStatusSchemaDefinitionByKey(ctx, sid, statusKey, ownerScope)
	if err != nil {
		return store.StatusSchemaDefinition{}, store.StatusCurrentValue{}, err
	}
	valueKind := statusSchemaNormalizeValueKind(definition.ValueKind)
	if valueKind == "" {
		return store.StatusSchemaDefinition{}, store.StatusCurrentValue{}, errors.New("registered value_kind is invalid")
	}
	if err := statusCurrentValueMatchesKind(valueJSON, valueKind); err != nil {
		return store.StatusSchemaDefinition{}, store.StatusCurrentValue{}, err
	}
	return definition, store.StatusCurrentValue{
		ChatSessionID: sid,
		RegistryID:    definition.ID,
		StatusKey:     definition.StatusKey,
		OwnerScope:    definition.OwnerScope,
		OwnerID:       ownerID,
		OwnerLabel:    strings.TrimSpace(req.OwnerLabel),
		ValueKind:     valueKind,
		ValueJSON:     valueJSON,
		EvidenceJSON:  evidenceJSON,
		SourceTurn:    req.SourceTurn,
		WriteState:    "current",
	}, nil
}

func statusChangeEventFromWriteRequest(ctx context.Context, registry store.StatusSchemaRegistryStore, req statusChangeEventWriteRequest) (store.StatusSchemaDefinition, store.StatusChangeEvent, error) {
	sid := strings.TrimSpace(req.ChatSessionID)
	if sid == "" {
		return store.StatusSchemaDefinition{}, store.StatusChangeEvent{}, errors.New("chat_session_id is required")
	}
	statusKey := strings.TrimSpace(req.StatusKey)
	if !statusSchemaValidKey(statusKey) {
		return store.StatusSchemaDefinition{}, store.StatusChangeEvent{}, errors.New("status_key is invalid")
	}
	ownerScope := statusSchemaNormalizeOwnerScope(req.OwnerScope)
	if ownerScope == "" {
		return store.StatusSchemaDefinition{}, store.StatusChangeEvent{}, errors.New("owner_scope is invalid")
	}
	ownerID := strings.TrimSpace(req.OwnerID)
	if ownerID == "" {
		return store.StatusSchemaDefinition{}, store.StatusChangeEvent{}, errors.New("owner_id is required")
	}
	eventKind := statusNormalizeEventKind(req.EventKind)
	if eventKind == "" {
		return store.StatusSchemaDefinition{}, store.StatusChangeEvent{}, errors.New("event_kind must be one of set, increase, decrease, clear, effect_applied, effect_expired, effect_cleared")
	}
	evidenceJSON, err := statusSchemaCompactRawJSON(req.EvidenceJSON, "evidence_json")
	if err != nil {
		return store.StatusSchemaDefinition{}, store.StatusChangeEvent{}, err
	}
	previousValueJSON, err := statusSchemaCompactOptionalRawJSON(req.PreviousValueJSON, "previous_value_json")
	if err != nil {
		return store.StatusSchemaDefinition{}, store.StatusChangeEvent{}, err
	}
	newValueJSON, err := statusSchemaCompactOptionalRawJSON(req.NewValueJSON, "new_value_json")
	if err != nil {
		return store.StatusSchemaDefinition{}, store.StatusChangeEvent{}, err
	}
	if eventKind != "clear" && eventKind != "effect_expired" && eventKind != "effect_cleared" && newValueJSON == "" {
		return store.StatusSchemaDefinition{}, store.StatusChangeEvent{}, errors.New("new_value_json is required for this event_kind")
	}
	storyClockJSON, err := statusSchemaCompactOptionalRawJSON(req.StoryClockJSON, "story_clock_json")
	if err != nil {
		return store.StatusSchemaDefinition{}, store.StatusChangeEvent{}, err
	}
	definition, err := registry.GetStatusSchemaDefinitionByKey(ctx, sid, statusKey, ownerScope)
	if err != nil {
		return store.StatusSchemaDefinition{}, store.StatusChangeEvent{}, err
	}
	if statusSchemaNormalizeValueKind(definition.ValueKind) == "derived" {
		return store.StatusSchemaDefinition{}, store.StatusChangeEvent{}, errors.New("derived status events are projection-only")
	}
	return definition, store.StatusChangeEvent{
		ChatSessionID:     sid,
		RegistryID:        definition.ID,
		StatusValueID:     req.StatusValueID,
		StatusKey:         definition.StatusKey,
		OwnerScope:        definition.OwnerScope,
		OwnerID:           ownerID,
		EventKind:         eventKind,
		PreviousValueJSON: previousValueJSON,
		NewValueJSON:      newValueJSON,
		EvidenceJSON:      evidenceJSON,
		SourceTurn:        req.SourceTurn,
		StoryClockJSON:    storyClockJSON,
		EventState:        "recorded",
	}, nil
}

func statusEffectFromWriteRequest(ctx context.Context, registry store.StatusSchemaRegistryStore, req statusEffectWriteRequest) (store.StatusSchemaDefinition, store.StatusEffect, error) {
	sid := strings.TrimSpace(req.ChatSessionID)
	if sid == "" {
		return store.StatusSchemaDefinition{}, store.StatusEffect{}, errors.New("chat_session_id is required")
	}
	statusKey := strings.TrimSpace(req.StatusKey)
	if !statusSchemaValidKey(statusKey) {
		return store.StatusSchemaDefinition{}, store.StatusEffect{}, errors.New("status_key is invalid")
	}
	ownerScope := statusSchemaNormalizeOwnerScope(req.OwnerScope)
	if ownerScope == "" {
		return store.StatusSchemaDefinition{}, store.StatusEffect{}, errors.New("owner_scope is invalid")
	}
	ownerID := strings.TrimSpace(req.OwnerID)
	if ownerID == "" {
		return store.StatusSchemaDefinition{}, store.StatusEffect{}, errors.New("owner_id is required")
	}
	effectKind := statusNormalizeEffectKind(req.EffectKind)
	if effectKind == "" {
		return store.StatusSchemaDefinition{}, store.StatusEffect{}, errors.New("effect_kind must be one of temporary_effect, buff, debuff, injury, cooldown")
	}
	state := statusNormalizeEffectState(firstNonEmptyStringLocal(req.EffectState, "active"))
	if state == "" {
		return store.StatusSchemaDefinition{}, store.StatusEffect{}, errors.New("effect_state must be one of pending, active, expired, cleared")
	}
	evidenceJSON, err := statusSchemaCompactRawJSON(req.EvidenceJSON, "evidence_json")
	if err != nil {
		return store.StatusSchemaDefinition{}, store.StatusEffect{}, err
	}
	startClockJSON, err := statusSchemaCompactJSONObject(req.StartClockJSON, "start_clock_json")
	if err != nil {
		return store.StatusSchemaDefinition{}, store.StatusEffect{}, err
	}
	durationJSON, err := statusSchemaCompactOptionalJSONObject(req.DurationJSON, "duration_json")
	if err != nil {
		return store.StatusSchemaDefinition{}, store.StatusEffect{}, err
	}
	expiresJSON, err := statusSchemaCompactOptionalJSONObject(req.ExpiresAtClockJSON, "expires_at_clock_json")
	if err != nil {
		return store.StatusSchemaDefinition{}, store.StatusEffect{}, err
	}
	if durationJSON == "" && expiresJSON == "" {
		return store.StatusSchemaDefinition{}, store.StatusEffect{}, errors.New("duration_json or expires_at_clock_json is required")
	}
	payloadJSON, err := statusSchemaCompactOptionalRawJSON(req.EffectPayloadJSON, "effect_payload_json")
	if err != nil {
		return store.StatusSchemaDefinition{}, store.StatusEffect{}, err
	}
	definition, err := registry.GetStatusSchemaDefinitionByKey(ctx, sid, statusKey, ownerScope)
	if err != nil {
		return store.StatusSchemaDefinition{}, store.StatusEffect{}, err
	}
	if statusSchemaNormalizeValueKind(definition.ValueKind) == "derived" {
		return store.StatusSchemaDefinition{}, store.StatusEffect{}, errors.New("derived status effects are projection-only")
	}
	return definition, store.StatusEffect{
		ChatSessionID:      sid,
		RegistryID:         definition.ID,
		StatusKey:          definition.StatusKey,
		OwnerScope:         definition.OwnerScope,
		OwnerID:            ownerID,
		EffectKind:         effectKind,
		EffectLabel:        strings.TrimSpace(req.EffectLabel),
		EffectPayloadJSON:  payloadJSON,
		EvidenceJSON:       evidenceJSON,
		SourceTurn:         req.SourceTurn,
		StartClockJSON:     startClockJSON,
		DurationJSON:       durationJSON,
		ExpiresAtClockJSON: expiresJSON,
		EffectState:        state,
	}, nil
}

func statusEffectStateUpdateFromRequest(req statusEffectStateRequest) (string, string, error) {
	state := statusNormalizeEffectState(req.EffectState)
	if state == "" {
		return "", "", errors.New("effect_state must be one of pending, active, expired, cleared")
	}
	evidenceJSON, err := statusSchemaCompactOptionalRawJSON(req.ClearedEvidenceJSON, "cleared_evidence_json")
	if err != nil {
		return "", "", err
	}
	if (state == "expired" || state == "cleared") && evidenceJSON == "" {
		return "", "", errors.New("cleared_evidence_json is required for expired or cleared effect_state")
	}
	return state, evidenceJSON, nil
}

func statusCurrentValueMatchesKind(valueJSON, valueKind string) error {
	var value any
	if err := json.Unmarshal([]byte(valueJSON), &value); err != nil {
		return errors.New("value_json must be valid JSON")
	}
	switch valueKind {
	case "boolean":
		if _, ok := value.(bool); !ok {
			return errors.New("boolean status value must be true or false")
		}
	case "tags":
		if _, ok := value.([]any); !ok {
			return errors.New("tags status value must be a JSON array")
		}
	case "note":
		if _, ok := value.(string); !ok {
			return errors.New("note status value must be a JSON string")
		}
	case "enum":
		if _, ok := value.(string); !ok {
			return errors.New("enum status value must be a JSON string")
		}
	case "derived":
		return errors.New("derived status values are projection-only and cannot be written directly")
	}
	return nil
}

func (s *Server) indexStatusSchemaProposal(ctx context.Context, proposal store.StatusSchemaProposal, clientMeta map[string]any, explicitVector []float32) statusSchemaVectorIndex {
	docID := statusSchemaVectorDocumentID(proposal)
	sourceRowID := strconv.FormatInt(proposal.ID, 10)
	out := statusSchemaVectorIndex{
		Status:      "skipped",
		Attempted:   false,
		DocumentID:  docID,
		Tier:        "status_schema_proposal",
		SourceTable: "status_schema_proposals",
		SourceRowID: sourceRowID,
	}
	if proposal.ID <= 0 || strings.TrimSpace(proposal.ChatSessionID) == "" {
		out.Reason = "proposal_row_not_persisted"
		return out
	}
	if s.Vector == nil {
		out.Reason = "vector_not_configured"
		return out
	}
	if strings.TrimSpace(s.Cfg.ChromaEndpoint) == "" {
		out.Reason = "chroma_endpoint_not_configured"
		return out
	}
	documentText := statusSchemaVectorDocumentText(proposal)
	out.DocumentChars = len([]rune(documentText))
	embedding := append([]float32(nil), explicitVector...)
	if len(embedding) > 0 {
		out.EmbeddingMode = "request_vector"
	}
	if len(embedding) == 0 {
		for _, key := range []string{"status_schema_vector", "chroma_document_vector", "schema_embedding"} {
			if candidate := clientMetaFloat32Vector(clientMeta, key); len(candidate) > 0 {
				embedding = candidate
				out.EmbeddingMode = "client_meta:" + key
				break
			}
		}
	}
	if len(embedding) == 0 {
		cfg := s.completeTurnExtractionConfig(clientMeta).Embedder
		if !cfg.hasConfig() {
			out.Reason = "missing_embedding_config"
			return out
		}
		embeddingJSON, model, err := callEmbedding(ctx, cfg, documentText)
		if err != nil {
			out.Status = "failed"
			out.Attempted = true
			out.Reason = "embedding_error: " + err.Error()
			return out
		}
		embedding = parseFloat32JSONList(embeddingJSON)
		out.EmbeddingMode = "backend_embedding"
		out.EmbeddingModel = model
	}
	if len(embedding) == 0 {
		out.Reason = "empty_embedding"
		return out
	}
	out.Attempted = true
	doc := archvector.VectorDocument{
		ID:            docID,
		Embedding:     embedding,
		Tier:          "status_schema_proposal",
		ChatSessionID: proposal.ChatSessionID,
		SourceTable:   "status_schema_proposals",
		SourceRowID:   sourceRowID,
		SchemaVersion: statusSchemaContractVersion,
		DocumentText:  documentText,
	}
	if err := s.Vector.Upsert(ctx, proposal.ChatSessionID, []archvector.VectorDocument{doc}); err != nil {
		out.Status = "failed"
		out.Reason = "vector_upsert_error: " + err.Error()
		return out
	}
	out.Status = "ok"
	return out
}

func (s *Server) indexStatusSchemaDefinition(ctx context.Context, definition store.StatusSchemaDefinition, clientMeta map[string]any, explicitVector []float32) statusSchemaVectorIndex {
	docID := statusSchemaDefinitionVectorDocumentID(definition)
	sourceRowID := strconv.FormatInt(definition.ID, 10)
	out := statusSchemaVectorIndex{
		Status:      "skipped",
		Attempted:   false,
		DocumentID:  docID,
		Tier:        "status_schema_definition",
		SourceTable: "status_schema_registry",
		SourceRowID: sourceRowID,
	}
	if definition.ID <= 0 || strings.TrimSpace(definition.ChatSessionID) == "" {
		out.Reason = "registry_row_not_persisted"
		return out
	}
	if s.Vector == nil {
		out.Reason = "vector_not_configured"
		return out
	}
	if strings.TrimSpace(s.Cfg.ChromaEndpoint) == "" {
		out.Reason = "chroma_endpoint_not_configured"
		return out
	}
	documentText := statusSchemaDefinitionVectorDocumentText(definition)
	out.DocumentChars = len([]rune(documentText))
	embedding := append([]float32(nil), explicitVector...)
	if len(embedding) > 0 {
		out.EmbeddingMode = "request_vector"
	}
	if len(embedding) == 0 {
		for _, key := range []string{"status_registry_vector", "status_schema_vector", "schema_embedding"} {
			if candidate := clientMetaFloat32Vector(clientMeta, key); len(candidate) > 0 {
				embedding = candidate
				out.EmbeddingMode = "client_meta:" + key
				break
			}
		}
	}
	if len(embedding) == 0 {
		cfg := s.completeTurnExtractionConfig(clientMeta).Embedder
		if !cfg.hasConfig() {
			out.Reason = "missing_embedding_config"
			return out
		}
		embeddingJSON, model, err := callEmbedding(ctx, cfg, documentText)
		if err != nil {
			out.Status = "failed"
			out.Attempted = true
			out.Reason = "embedding_error: " + err.Error()
			return out
		}
		embedding = parseFloat32JSONList(embeddingJSON)
		out.EmbeddingMode = "backend_embedding"
		out.EmbeddingModel = model
	}
	if len(embedding) == 0 {
		out.Reason = "empty_embedding"
		return out
	}
	out.Attempted = true
	doc := archvector.VectorDocument{
		ID:            docID,
		Embedding:     embedding,
		Tier:          "status_schema_definition",
		ChatSessionID: definition.ChatSessionID,
		SourceTable:   "status_schema_registry",
		SourceRowID:   sourceRowID,
		SchemaVersion: statusSchemaRegistryContractVersion,
		DocumentText:  documentText,
	}
	if err := s.Vector.Upsert(ctx, definition.ChatSessionID, []archvector.VectorDocument{doc}); err != nil {
		out.Status = "failed"
		out.Reason = "vector_upsert_error: " + err.Error()
		return out
	}
	out.Status = "ok"
	return out
}

func statusSchemaVectorDocumentID(proposal store.StatusSchemaProposal) string {
	if proposal.ID <= 0 {
		return ""
	}
	return "status_schema_proposal:" + strings.TrimSpace(proposal.ChatSessionID) + ":" + strconv.FormatInt(proposal.ID, 10)
}

func statusSchemaVectorDocumentText(proposal store.StatusSchemaProposal) string {
	parts := []string{
		"Archive Center status schema proposal",
		"schema_name: " + strings.TrimSpace(proposal.SchemaName),
		"ruleset_label: " + strings.TrimSpace(proposal.RulesetLabel),
		"input_channel: " + strings.TrimSpace(proposal.InputChannel),
		"proposal_state: " + strings.TrimSpace(proposal.ProposalState),
		"schema_json:",
		strings.TrimSpace(proposal.SchemaJSON),
	}
	if provenance := strings.TrimSpace(proposal.ProvenanceJSON); provenance != "" {
		parts = append(parts, "provenance_json:", provenance)
	}
	if note := strings.TrimSpace(proposal.ReviewNote); note != "" {
		parts = append(parts, "review_note:", note)
	}
	if reviewer := strings.TrimSpace(proposal.Reviewer); reviewer != "" {
		parts = append(parts, "reviewer: "+reviewer)
	}
	return strings.Join(parts, "\n")
}

func statusSchemaDefinitionVectorDocumentID(definition store.StatusSchemaDefinition) string {
	if definition.ID <= 0 {
		return ""
	}
	return "status_schema_definition:" + strings.TrimSpace(definition.ChatSessionID) + ":" + strconv.FormatInt(definition.ID, 10)
}

func statusSchemaDefinitionVectorDocumentText(definition store.StatusSchemaDefinition) string {
	parts := []string{
		"Archive Center status schema definition",
		"schema_name: " + strings.TrimSpace(definition.SchemaName),
		"ruleset_label: " + strings.TrimSpace(definition.RulesetLabel),
		"status_key: " + strings.TrimSpace(definition.StatusKey),
		"label: " + strings.TrimSpace(definition.Label),
		"owner_scope: " + strings.TrimSpace(definition.OwnerScope),
		"value_kind: " + strings.TrimSpace(definition.ValueKind),
		"registry_state: " + strings.TrimSpace(definition.RegistryState),
	}
	if definition.BoundsJSON != "" {
		parts = append(parts, "bounds_json:", definition.BoundsJSON)
	}
	if definition.OptionsJSON != "" {
		parts = append(parts, "options_json:", definition.OptionsJSON)
	}
	if definition.DefaultValueJSON != "" {
		parts = append(parts, "default_value_json:", definition.DefaultValueJSON)
	}
	return strings.Join(parts, "\n")
}

func statusSchemaDefinitionsFromApprovedProposal(proposal store.StatusSchemaProposal) ([]store.StatusSchemaDefinition, error) {
	if strings.TrimSpace(proposal.ProposalState) != "approved" {
		return nil, errors.New("proposal must be approved before registry import")
	}
	var root map[string]any
	if err := json.Unmarshal([]byte(proposal.SchemaJSON), &root); err != nil || root == nil {
		return nil, errors.New("schema_json must be a JSON object")
	}
	stats := sliceFromAny(root["stats"])
	if len(stats) == 0 {
		stats = sliceFromAny(root["status_definitions"])
	}
	if len(stats) == 0 {
		return nil, errors.New("schema_json must include non-empty stats or status_definitions array")
	}
	definitions := make([]store.StatusSchemaDefinition, 0, len(stats))
	seen := map[string]bool{}
	for idx, raw := range stats {
		item := mapFromAny(raw)
		if len(item) == 0 {
			return nil, errors.New("status definition at index " + strconv.Itoa(idx) + " must be an object")
		}
		if statusSchemaHasExecutableFormulaField(item) {
			return nil, errors.New("status definition " + strconv.Itoa(idx) + " uses formula/script/code fields that are not enabled")
		}
		key := strings.TrimSpace(firstNonEmptyStringLocal(stringFromMap(item, "status_key"), stringFromMap(item, "key")))
		if !statusSchemaValidKey(key) {
			return nil, errors.New("status definition " + strconv.Itoa(idx) + " has invalid status_key")
		}
		if seen[key] {
			return nil, errors.New("duplicate status_key " + key)
		}
		seen[key] = true
		ownerScope := statusSchemaNormalizeOwnerScope(firstNonEmptyStringLocal(stringFromMap(item, "owner_scope"), stringFromMap(item, "scope")))
		if ownerScope == "" {
			return nil, errors.New("status definition " + key + " requires owner_scope")
		}
		valueKind := statusSchemaNormalizeValueKind(firstNonEmptyStringLocal(stringFromMap(item, "value_kind"), stringFromMap(item, "kind"), stringFromMap(item, "type")))
		if valueKind == "" {
			return nil, errors.New("status definition " + key + " requires value_kind")
		}
		boundsJSON, err := statusSchemaCompactOptionalValue(item["bounds"], "bounds")
		if err != nil {
			return nil, errors.New("status definition " + key + ": " + err.Error())
		}
		optionsJSON, err := statusSchemaCompactOptionalValue(item["options"], "options")
		if err != nil {
			return nil, errors.New("status definition " + key + ": " + err.Error())
		}
		defaultValue := item["default_value"]
		if defaultValue == nil {
			defaultValue = item["default"]
		}
		defaultValueJSON, err := statusSchemaCompactOptionalValue(defaultValue, "default_value")
		if err != nil {
			return nil, errors.New("status definition " + key + ": " + err.Error())
		}
		definitions = append(definitions, store.StatusSchemaDefinition{
			ChatSessionID:    proposal.ChatSessionID,
			SourceProposalID: proposal.ID,
			SchemaName:       proposal.SchemaName,
			RulesetLabel:     proposal.RulesetLabel,
			StatusKey:        key,
			Label:            firstNonEmptyStringLocal(stringFromMap(item, "label"), key),
			OwnerScope:       ownerScope,
			ValueKind:        valueKind,
			BoundsJSON:       boundsJSON,
			OptionsJSON:      optionsJSON,
			DefaultValueJSON: defaultValueJSON,
			RegistryState:    "active",
		})
	}
	return definitions, nil
}

func statusSchemaHasExecutableFormulaField(item map[string]any) bool {
	for _, key := range []string{"formula", "script", "code", "expression"} {
		if hasMeaningfulPayload(item[key]) {
			return true
		}
	}
	return false
}

func statusSchemaValidKey(key string) bool {
	if key == "" {
		return false
	}
	for _, r := range key {
		if r == '_' || r == '-' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		return false
	}
	return true
}

func statusSchemaNormalizeOwnerScope(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "character", "party", "faction", "world", "entity", "session":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func statusSchemaNormalizeValueKind(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "scalar", "number", "numeric":
		return "scalar"
	case "resource":
		return "resource"
	case "enum", "choice":
		return "enum"
	case "boolean", "bool":
		return "boolean"
	case "clock", "time":
		return "clock"
	case "tags", "tag_list":
		return "tags"
	case "note", "text":
		return "note"
	case "derived":
		return "derived"
	default:
		return ""
	}
}

func statusNormalizeEventKind(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "set", "increase", "decrease", "clear", "effect_applied", "effect_expired", "effect_cleared":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func statusNormalizeEffectKind(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "temporary", "temporary_effect":
		return "temporary_effect"
	case "buff", "debuff", "injury", "cooldown":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func statusNormalizeEffectState(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "pending", "active", "expired", "cleared":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func statusNormalizeOptionalEffectState(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	state := statusNormalizeEffectState(raw)
	if state == "" {
		return "", errors.New("effect_state must be one of pending, active, expired, cleared")
	}
	return state, nil
}

func statusNormalizeAuthorityMode(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "auto":
		return "auto", nil
	case "archive", "canonical", "archive_canonical", "archive-current", "archive_current":
		return "archive_canonical", nil
	case "external", "external_runtime", "runtime", "lua", "lua_runtime":
		return "external_runtime", nil
	default:
		return "", errors.New("authority_mode must be one of auto, archive_canonical, external_runtime")
	}
}

func statusNormalizeProjectionDensity(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "auto":
		return "auto", nil
	case "full":
		return "full", nil
	case "light", "tag":
		return "light", nil
	default:
		return "", errors.New("projection_density must be one of auto, full, light")
	}
}

func statusSchemaOptionalOwnerScope(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", nil
	}
	scope := statusSchemaNormalizeOwnerScope(raw)
	if scope == "" {
		return "", errors.New("owner_scope is invalid")
	}
	return scope, nil
}

func statusSchemaNormalizeOptionalRegistryState(raw string) (string, error) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return "", nil
	}
	switch raw {
	case "active", "deprecated", "disabled":
		return raw, nil
	default:
		return "", errors.New("registry_state must be one of active, deprecated, disabled")
	}
}

func statusSchemaNormalizeInputChannel(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "bootstrap", "schema_bootstrap":
		return "bootstrap"
	case "direct", "direct_json", "settings", "settings_json":
		return "direct_json"
	case "import", "portable_import", "schema_import":
		return "portable_import"
	default:
		return ""
	}
}

func statusSchemaNormalizeOptionalProposalState(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if raw == "pending" {
		raw = "pending_review"
	}
	switch raw {
	case "pending_review", "approved", "rejected", "needs_revision":
		return raw, nil
	default:
		return "", errors.New("proposal_state must be one of pending_review, approved, rejected, needs_revision")
	}
}

func statusSchemaNormalizeReviewState(raw string) (string, error) {
	state, err := statusSchemaNormalizeOptionalProposalState(raw)
	if err != nil {
		return "", err
	}
	if state == "" || state == "pending_review" {
		return "", errors.New("proposal_state must be one of approved, rejected, needs_revision")
	}
	return state, nil
}

func statusSchemaCompactJSONObject(raw json.RawMessage, field string) (string, error) {
	compact, err := statusSchemaCompactRawJSON(raw, field)
	if err != nil {
		return "", err
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(compact), &obj); err != nil || obj == nil {
		return "", errors.New(field + " must be a JSON object")
	}
	return compact, nil
}

func statusSchemaCompactOptionalJSONObject(raw json.RawMessage, field string) (string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "", nil
	}
	return statusSchemaCompactJSONObject(raw, field)
}

func statusSchemaCompactOptionalRawJSON(raw json.RawMessage, field string) (string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "", nil
	}
	if !json.Valid(trimmed) {
		return "", errors.New(field + " must be valid JSON")
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, trimmed); err != nil {
		return "", errors.New(field + " must be valid JSON")
	}
	return buf.String(), nil
}

func statusSchemaCompactOptionalValue(raw any, field string) (string, error) {
	if !hasMeaningfulPayload(raw) {
		return "", nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return "", errors.New(field + " must be JSON serializable")
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, b); err != nil {
		return "", errors.New(field + " must be valid JSON")
	}
	return buf.String(), nil
}

func statusSchemaCompactRawJSON(raw json.RawMessage, field string) (string, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return "", errors.New(field + " is required")
	}
	if !json.Valid(trimmed) {
		return "", errors.New(field + " must be valid JSON")
	}
	var buf bytes.Buffer
	if err := json.Compact(&buf, trimmed); err != nil {
		return "", errors.New(field + " must be valid JSON")
	}
	return buf.String(), nil
}

func firstNonEmptyQuery(r *http.Request, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(r.URL.Query().Get(key)); value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptyStringLocal(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func statusSchemaBoundedLimit(raw string, fallback, minValue, maxValue int) int {
	value := fallback
	if strings.TrimSpace(raw) != "" {
		if parsed, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
			value = parsed
		}
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
