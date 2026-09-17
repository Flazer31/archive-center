package dto

// PrepareTurnSourceObservationV1 contains only source facts observed by the
// host adapter. It intentionally carries no authority, acceptance, or
// persistence decision.
type PrepareTurnSourceObservationV1 struct {
	ContractVersion       string                   `json:"contract_version"`
	SessionID             string                   `json:"session_id,omitempty"`
	ChatID                *string                  `json:"chat_id,omitempty"`
	BranchID              *string                  `json:"branch_id,omitempty"`
	RequestID             *string                  `json:"request_id,omitempty"`
	GenerationID          *string                  `json:"generation_id,omitempty"`
	MessageIndex          *int                     `json:"message_index,omitempty"`
	ObservedRole          *string                  `json:"observed_role,omitempty"`
	ObservedSourcePath    *string                  `json:"observed_source_path,omitempty"`
	ObservedRevision      *string                  `json:"observed_revision,omitempty"`
	RawInputHash          *string                  `json:"raw_input_hash,omitempty"`
	RawInputHashAlgorithm *string                  `json:"raw_input_hash_algorithm,omitempty"`
	DisplayedOutputHash   *string                  `json:"displayed_output_hash,omitempty"`
	HostSource            *PrepareTurnHostSourceV1 `json:"host_source,omitempty"`
	Observable            *bool                    `json:"observable,omitempty"`
	EvidenceState         string                   `json:"evidence_state"`
}

// PrepareTurnHostSourceV1 is populated only when RisuAI directly exposes the
// corresponding source metadata.
type PrepareTurnHostSourceV1 struct {
	Kind       *string `json:"kind,omitempty"`
	ID         *string `json:"id,omitempty"`
	Scope      *string `json:"scope,omitempty"`
	Revision   *string `json:"revision,omitempty"`
	Activation *string `json:"activation,omitempty"`
	Visibility *string `json:"visibility,omitempty"`
}

// PrepareTurnCapabilityObservationV1 reports host availability. Whether a
// capability is required or optional remains a Go policy decision.
type PrepareTurnCapabilityObservationV1 struct {
	ContractVersion string            `json:"contract_version"`
	Capabilities    map[string]string `json:"capabilities,omitempty"`
}

// PrepareTurnLorebookReferenceScopeV1 contains only the exact Host scope
// observations needed to select the corresponding read-only MariaDB snapshot.
// Nil indexes and an unobserved module list remain unavailable facts.
type PrepareTurnLorebookReferenceScopeV1 struct {
	ContractVersion        string   `json:"contract_version"`
	ObservationState       string   `json:"observation_state"`
	CharacterIndex         *int64   `json:"character_index,omitempty"`
	ChatIndex              *int64   `json:"chat_index,omitempty"`
	EnabledModuleIDs       []string `json:"enabled_module_ids"`
	EnabledModulesObserved bool     `json:"enabled_modules_observed"`
}

// PrepareTurnContractRequest extends the generated legacy request without
// editing generated code or changing its defaulting behavior.
type PrepareTurnContractRequest struct {
	PrepareTurnRequest
	// RecentConversationMessages is the non-mutating Archive-only active-chat
	// read copy used solely to form recent conversation recall queries.
	RecentConversationMessages []map[string]any                     `json:"recent_conversation_messages,omitempty"`
	SourceDecisionOnly         bool                                 `json:"source_decision_only,omitempty"`
	ResponseProjection         string                               `json:"response_projection,omitempty"`
	NarrativeSupportMaxChars   *int                                 `json:"narrative_support_max_chars,omitempty"`
	PublisherGuidanceFormat    *string                              `json:"publisher_guidance_format,omitempty"`
	SourceObservation          *PrepareTurnSourceObservationV1      `json:"source_observation,omitempty"`
	CapabilityObservation      *PrepareTurnCapabilityObservationV1  `json:"capability_observation,omitempty"`
	HostObservations           *PrepareTurnHostObservationsV1       `json:"host_observations,omitempty"`
	BootstrapObservation       *PrepareTurnBootstrapObservationV1   `json:"bootstrap_observation,omitempty"`
	LorebookReferenceScope     *PrepareTurnLorebookReferenceScopeV1 `json:"lorebook_reference_scope,omitempty"`
}

func (request *PrepareTurnContractRequest) ApplyDefaults() {
	request.PrepareTurnRequest.ApplyDefaults()
	if request.PublisherGuidanceFormat == nil {
		value := "standard"
		request.PublisherGuidanceFormat = &value
	}
}

// SupervisorContractRequest extends the generated supervisor request with the
// backend-owned execution evidence needed to bound supervisor proposals.
type SupervisorContractRequest struct {
	SupervisorRequest
	GuideStrength             *string        `json:"guide_strength,omitempty"`
	ResponseExecutionContract map[string]any `json:"response_execution_contract,omitempty"`
}

func (request *SupervisorContractRequest) ApplyDefaults() {
	request.SupervisorRequest.ApplyDefaults()
	if request.GuideStrength == nil {
		value := "weak"
		request.GuideStrength = &value
	}
}

type PrepareTurnCapabilityCoverageV1 struct {
	Required        []string          `json:"required"`
	Optional        []string          `json:"optional"`
	States          map[string]string `json:"states"`
	RequiredMissing []string          `json:"required_missing"`
	OptionalMissing []string          `json:"optional_missing"`
}

type PrepareTurnLaneStatusV1 struct {
	Status                   string                          `json:"status"`
	ReasonCode               string                          `json:"reason_code"`
	Retryable                bool                            `json:"retryable"`
	AffectedLane             string                          `json:"affected_lane"`
	OriginalPayloadPreserved bool                            `json:"original_payload_preserved"`
	ContractVersion          string                          `json:"contract_version"`
	CapabilityCoverage       PrepareTurnCapabilityCoverageV1 `json:"capability_coverage"`
	RequestCorrelationID     *string                         `json:"request_correlation_id,omitempty"`
}

type PrepareTurnSourceContractProjectionV1 struct {
	ContractVersion           string                          `json:"contract_version"`
	SourceContractVersion     string                          `json:"source_contract_version,omitempty"`
	CapabilityContractVersion string                          `json:"capability_contract_version,omitempty"`
	NormalizedSource          *PrepareTurnSourceObservationV1 `json:"normalized_source,omitempty"`
	LaneStatus                PrepareTurnLaneStatusV1         `json:"lane_status"`
}

// PrepareTurnMessageObservationV1 is an exact host observation. RawContent is
// intentionally not normalized: Go verifies the hash and owns all selection.
type PrepareTurnMessageObservationV1 struct {
	ObservationRef   string                   `json:"observation_ref"`
	SourceKind       string                   `json:"source_kind"`
	ObservationStage string                   `json:"observation_stage,omitempty"`
	MessageID        *string                  `json:"message_id,omitempty"`
	GenerationID     *string                  `json:"generation_id,omitempty"`
	MessageTime      *int64                   `json:"message_time,omitempty"`
	MessageIndex     *int                     `json:"message_index,omitempty"`
	Role             *string                  `json:"role,omitempty"`
	RawContent       *string                  `json:"raw_content,omitempty"`
	ContentHash      *string                  `json:"content_hash,omitempty"`
	HashAlgorithm    *string                  `json:"hash_algorithm,omitempty"`
	ObservedAt       *string                  `json:"observed_at,omitempty"`
	ObservedRevision *string                  `json:"observed_revision,omitempty"`
	LifecycleKind    string                   `json:"lifecycle_kind,omitempty"`
	HostSource       *PrepareTurnHostSourceV1 `json:"host_source,omitempty"`
	EvidenceState    string                   `json:"evidence_state"`
}

// PrepareTurnHostObservationsV1 carries lifecycle facts only. It contains no
// client-authored authority, ownership, or injection decision.
type PrepareTurnHostObservationsV1 struct {
	ContractVersion         string                            `json:"contract_version"`
	SessionID               string                            `json:"session_id"`
	ChatID                  *string                           `json:"chat_id,omitempty"`
	BranchID                *string                           `json:"branch_id,omitempty"`
	RequestID               string                            `json:"request_id"`
	RequestType             string                            `json:"request_type"`
	PayloadPath             *string                           `json:"payload_path,omitempty"`
	PayloadWritable         bool                              `json:"payload_writable"`
	PayloadObservationStage string                            `json:"payload_observation_stage,omitempty"`
	FinalPayloadObservation string                            `json:"final_payload_observation,omitempty"`
	InputHook               *PrepareTurnMessageObservationV1  `json:"input_hook,omitempty"`
	ActiveChat              []PrepareTurnMessageObservationV1 `json:"active_chat,omitempty"`
	Payload                 []PrepareTurnMessageObservationV1 `json:"payload,omitempty"`
}

// PrepareTurnHostContextSpanV1 is an exact, request-scoped span observed in
// the official RisuAI beforeRequest message array. Host source metadata stays
// absent unless the host exposes it directly; role or prose never substitutes
// for source kind, ID, scope, revision, activation, or visibility.
type PrepareTurnHostContextSpanV1 struct {
	ObservationRef        string                   `json:"observation_ref"`
	PayloadPath           *string                  `json:"payload_path,omitempty"`
	MessageOrder          int                      `json:"message_order"`
	Role                  string                   `json:"role"`
	SpanStart             int                      `json:"span_start"`
	SpanEnd               int                      `json:"span_end"`
	ExactText             string                   `json:"-"`
	ContentHash           string                   `json:"content_hash"`
	HashAlgorithm         string                   `json:"hash_algorithm"`
	HostSource            *PrepareTurnHostSourceV1 `json:"host_source,omitempty"`
	SourceKindState       string                   `json:"source_kind_state"`
	SourceIDState         string                   `json:"source_id_state"`
	SourceScopeState      string                   `json:"source_scope_state"`
	SourceRevisionState   string                   `json:"source_revision_state"`
	ActivationState       string                   `json:"activation_state"`
	VisibilityState       string                   `json:"visibility_state"`
	NativePresent         bool                     `json:"native_present"`
	ReferenceEligibility  string                   `json:"reference_eligibility"`
	ReferenceReasonCode   string                   `json:"reference_reason_code"`
	CanonicalWriteAllowed bool                     `json:"canonical_write_allowed"`
}

// PrepareTurnRisuHostContextSnapshotV1 is a read-only view of the payload at
// the Archive Center beforeRequest hook. It is deliberately not described as
// the final provider request when official RisuAI can still run later
// replacers or request triggers.
type PrepareTurnRisuHostContextSnapshotV1 struct {
	ContractVersion         string                         `json:"contract_version"`
	Status                  string                         `json:"status"`
	ReasonCode              string                         `json:"reason_code"`
	SessionID               string                         `json:"session_id"`
	ChatID                  *string                        `json:"chat_id,omitempty"`
	BranchID                *string                        `json:"branch_id,omitempty"`
	RequestID               string                         `json:"request_id"`
	PayloadPath             *string                        `json:"payload_path,omitempty"`
	ObservationStage        string                         `json:"observation_stage"`
	FinalPayloadObservation string                         `json:"final_payload_observation"`
	SourceMetadataState     string                         `json:"source_metadata_state"`
	Spans                   []PrepareTurnHostContextSpanV1 `json:"spans"`
	ObservedSpanCount       int                            `json:"observed_span_count"`
	ReferenceEligibleCount  int                            `json:"reference_eligible_count"`
	CanonicalWriteAllowed   bool                           `json:"canonical_write_allowed"`
}

type PrepareTurnHostContextDuplicateV1 struct {
	KeptObservationRef       string   `json:"kept_observation_ref"`
	DuplicateObservationRefs []string `json:"duplicate_observation_refs"`
	ContentHash              string   `json:"content_hash"`
	ReasonCode               string   `json:"reason_code"`
}

// PrepareTurnHostContextReferenceEvidenceItemV1 keeps only exact native
// system-role spans. "unknown" is intentional: official API v3 does not
// expose whether such a span came from a character card, scenario, lorebook,
// author note, module, or another request source.
type PrepareTurnHostContextReferenceEvidenceItemV1 struct {
	EvidenceRef            string                   `json:"evidence_ref"`
	ObservationRef         string                   `json:"observation_ref"`
	PayloadPath            *string                  `json:"payload_path,omitempty"`
	MessageOrder           int                      `json:"message_order"`
	Role                   string                   `json:"role"`
	SpanStart              int                      `json:"span_start"`
	SpanEnd                int                      `json:"span_end"`
	ExactText              string                   `json:"-"`
	ContentHash            string                   `json:"content_hash"`
	HashAlgorithm          string                   `json:"hash_algorithm"`
	HostSource             *PrepareTurnHostSourceV1 `json:"host_source,omitempty"`
	SourceKind             string                   `json:"source_kind"`
	SourceMetadataState    string                   `json:"source_metadata_state"`
	Authority              string                   `json:"authority"`
	ConflictState          string                   `json:"conflict_state"`
	ConflictReasonCode     string                   `json:"conflict_reason_code"`
	SessionDivergenceState string                   `json:"session_divergence_state"`
	CanonicalWriteAllowed  bool                     `json:"canonical_write_allowed"`
}

type PrepareTurnHostContextReferenceEvidenceV1 struct {
	ContractVersion         string                                          `json:"contract_version"`
	Status                  string                                          `json:"status"`
	ReasonCode              string                                          `json:"reason_code"`
	SessionID               string                                          `json:"session_id"`
	ChatID                  *string                                         `json:"chat_id,omitempty"`
	BranchID                *string                                         `json:"branch_id,omitempty"`
	RequestID               string                                          `json:"request_id"`
	SnapshotContractVersion string                                          `json:"snapshot_contract_version"`
	Items                   []PrepareTurnHostContextReferenceEvidenceItemV1 `json:"items"`
	Duplicates              []PrepareTurnHostContextDuplicateV1             `json:"duplicates"`
	SelectedCount           int                                             `json:"selected_count"`
	DuplicateCount          int                                             `json:"duplicate_count"`
	DeferredCount           int                                             `json:"deferred_count"`
	CanonicalWriteAllowed   bool                                            `json:"canonical_write_allowed"`
}

// PrepareTurnBootstrapObservationV1 preserves ordered start observations.
// Greeting fields are included only when RisuAI exposes them directly.
type PrepareTurnBootstrapObservationV1 struct {
	ContractVersion       string                            `json:"contract_version"`
	SessionID             string                            `json:"session_id"`
	ChatID                *string                           `json:"chat_id,omitempty"`
	BranchID              *string                           `json:"branch_id,omitempty"`
	RequestID             string                            `json:"request_id"`
	ObservationState      string                            `json:"observation_state"`
	LeadingMessages       []PrepareTurnMessageObservationV1 `json:"leading_messages"`
	SelectedGreetingIndex *int                              `json:"selected_greeting_index,omitempty"`
	SelectionExposed      bool                              `json:"selection_exposed"`
	FirstGreeting         *string                           `json:"first_greeting,omitempty"`
	AlternateGreetings    []string                          `json:"alternate_greetings,omitempty"`
}

type PrepareTurnMessageSourceIdentityV1 struct {
	SessionID    string  `json:"session_id"`
	ChatID       *string `json:"chat_id,omitempty"`
	BranchID     *string `json:"branch_id,omitempty"`
	RequestID    *string `json:"request_id,omitempty"`
	MessageID    *string `json:"message_id,omitempty"`
	GenerationID *string `json:"generation_id,omitempty"`
	MessageTime  *int64  `json:"message_time,omitempty"`
	MessageIndex *int    `json:"message_index,omitempty"`
}

type PrepareTurnMessageSourceEnvelopeV1 struct {
	ContractVersion      string                             `json:"contract_version"`
	BackendSourceID      string                             `json:"backend_source_id"`
	Identity             PrepareTurnMessageSourceIdentityV1 `json:"identity"`
	RawRole              string                             `json:"raw_role"`
	Channel              string                             `json:"channel"`
	RawContent           string                             `json:"raw_content"`
	ContentHash          string                             `json:"content_hash"`
	HashAlgorithm        string                             `json:"hash_algorithm"`
	ObservedAt           *string                            `json:"observed_at,omitempty"`
	ObservedRevision     *string                            `json:"observed_revision,omitempty"`
	SourceOrigin         string                             `json:"source_origin"`
	OriginProvenance     string                             `json:"origin_provenance"`
	ObservationRef       string                             `json:"observation_ref"`
	LifecycleObservation PrepareTurnSourceLifecycleV1       `json:"lifecycle_observation"`
}

type PrepareTurnSourceLifecycleV1 struct {
	State        string `json:"state"`
	Streaming    string `json:"streaming"`
	FinalDisplay string `json:"final_display"`
	Deleted      bool   `json:"deleted"`
	Rerolled     bool   `json:"rerolled"`
	Edited       bool   `json:"edited"`
}

type PrepareTurnCurrentInputDecisionV1 struct {
	ContractVersion          string                              `json:"contract_version"`
	Status                   string                              `json:"status"`
	ReasonCode               string                              `json:"reason_code"`
	Retryable                bool                                `json:"retryable"`
	RequestOwnership         string                              `json:"request_ownership"`
	ContextInjectionEligible bool                                `json:"context_injection_eligible"`
	MemoryReadsAllowed       bool                                `json:"memory_reads_allowed"`
	OriginalPayloadPreserved bool                                `json:"original_payload_preserved"`
	EffectiveUserInput       string                              `json:"effective_user_input"`
	SelectedObservationRef   *string                             `json:"selected_observation_ref,omitempty"`
	Envelope                 *PrepareTurnMessageSourceEnvelopeV1 `json:"message_source_envelope,omitempty"`
	CapabilityCoverage       PrepareTurnCapabilityCoverageV1     `json:"capability_coverage"`
}

type PrepareTurnPriorResponseSourceV1 struct {
	Status     string `json:"status"`
	ReasonCode string `json:"reason_code"`
}

type PrepareTurnSessionBootstrapV1 struct {
	ContractVersion          string                               `json:"contract_version"`
	Status                   string                               `json:"status"`
	ReasonCode               string                               `json:"reason_code"`
	Sources                  []PrepareTurnMessageSourceEnvelopeV1 `json:"sources"`
	PriorResponseSource      PrepareTurnPriorResponseSourceV1     `json:"prior_response_source"`
	RetrievalCandidateStatus string                               `json:"retrieval_candidate_status"`
	MemoryReadsAllowed       bool                                 `json:"memory_reads_allowed"`
	InjectionAllowed         bool                                 `json:"injection_allowed"`
	CanonicalWriteAllowed    bool                                 `json:"canonical_write_allowed"`
	OriginalPayloadPreserved bool                                 `json:"original_payload_preserved"`
}
