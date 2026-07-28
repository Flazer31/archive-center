package store

import (
	"context"
	"time"
)

const PreciseMemoryUnitContract = "precise_memory_unit.v1"

// PreciseMemoryUnit is an additive, exact-source memory projection. It does
// not replace the legacy aggregate Memory row. Each unit preserves one
// semantic claim or occurrence and the accepted source span that admitted it.
type PreciseMemoryUnit struct {
	ID                      int64     `json:"id"`
	UnitID                  string    `json:"unit_id"`
	ContractVersion         string    `json:"contract_version"`
	ChatSessionID           string    `json:"chat_session_id"`
	SourceTurnStart         int       `json:"source_turn_start"`
	SourceTurnEnd           int       `json:"source_turn_end"`
	SourceContract          string    `json:"source_contract"`
	SourceRevision          string    `json:"source_revision"`
	SourceLogicalTurnID     string    `json:"source_logical_turn_id,omitempty"`
	SourceMessageID         string    `json:"source_message_id,omitempty"`
	SourceGenerationID      string    `json:"source_generation_id,omitempty"`
	SourceContentHash       string    `json:"source_content_hash"`
	SourceRole              string    `json:"source_role"`
	SourceSpanStart         int       `json:"source_span_start"`
	SourceSpanEnd           int       `json:"source_span_end"`
	EvidenceExcerpt         string    `json:"evidence_excerpt"`
	EvidenceHash            string    `json:"evidence_hash"`
	RootEvidenceID          int64     `json:"root_evidence_id"`
	DirectEvidenceIDsJSON   string    `json:"direct_evidence_ids_json"`
	Kind                    string    `json:"kind"`
	Subtype                 string    `json:"subtype,omitempty"`
	PayloadJSON             string    `json:"payload_json"`
	ActorEntityID           string    `json:"actor_entity_id,omitempty"`
	SubjectEntityID         string    `json:"subject_entity_id,omitempty"`
	AffectedEntityID        string    `json:"affected_entity_id,omitempty"`
	LocationEntityID        string    `json:"location_entity_id,omitempty"`
	ObjectEntityID          string    `json:"object_entity_id,omitempty"`
	RelationshipKey         string    `json:"relationship_key,omitempty"`
	TruthScope              string    `json:"truth_scope"`
	EpistemicMode           string    `json:"epistemic_mode"`
	AuthorityClass          string    `json:"authority_class"`
	AdmissionState          string    `json:"admission_state"`
	ReviewState             string    `json:"review_state"`
	Visibility              string    `json:"visibility"`
	KnowledgeHolderEntityID string    `json:"knowledge_holder_entity_id,omitempty"`
	RevealCondition         string    `json:"reveal_condition,omitempty"`
	Confidence              float64   `json:"confidence"`
	IdempotencyKey          string    `json:"idempotency_key"`
	LifecycleState          string    `json:"lifecycle_state"`
	CreatedAt               time.Time `json:"created_at"`
	UpdatedAt               time.Time `json:"updated_at"`
}

// PreciseMemoryWriter is optional so legacy, fixture, noop, and read-only
// stores remain compatible without claiming that atomic writes succeeded.
type PreciseMemoryWriter interface {
	SavePreciseMemoryUnit(context.Context, *PreciseMemoryUnit) (inserted bool, err error)
}

// PreciseMemoryWriteAvailability lets composite stores report whether at least
// one real persistence lane can accept the optional projection.
type PreciseMemoryWriteAvailability interface {
	PreciseMemoryWritesEnabled() bool
}
