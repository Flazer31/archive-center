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
