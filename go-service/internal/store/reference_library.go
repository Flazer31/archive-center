package store

import (
	"context"
	"errors"
	"time"
)

var (
	ErrReferenceConflict  = errors.New("reference library record conflicts with an existing record")
	ErrReferenceWorkInUse = errors.New("reference work is linked to one or more sessions")
	ErrInvalidReference   = errors.New("invalid reference library record")
)

type ReferenceWork struct {
	WorkID          string    `json:"work_id"`
	Title           string    `json:"title"`
	WorkType        string    `json:"work_type"`
	DefaultLanguage string    `json:"default_language"`
	Status          string    `json:"status"`
	MetadataJSON    string    `json:"metadata_json,omitempty"`
	Revision        int64     `json:"revision"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type ReferenceContinuity struct {
	ContinuityID       string    `json:"continuity_id"`
	WorkID             string    `json:"work_id"`
	ContinuityKey      string    `json:"continuity_key"`
	Label              string    `json:"label"`
	ParentContinuityID string    `json:"parent_continuity_id,omitempty"`
	Status             string    `json:"status"`
	MetadataJSON       string    `json:"metadata_json,omitempty"`
	Revision           int64     `json:"revision"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type ReferenceDocument struct {
	DocumentID     string    `json:"document_id"`
	WorkID         string    `json:"work_id"`
	ContinuityID   string    `json:"continuity_id"`
	SourceType     string    `json:"source_type"`
	SourceURI      string    `json:"source_uri,omitempty"`
	ContentHash    string    `json:"content_hash"`
	RawRetention   string    `json:"raw_retention"`
	RawText        string    `json:"raw_text,omitempty"`
	ImportStatus   string    `json:"import_status"`
	ProvenanceJSON string    `json:"provenance_json,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type ReferenceTimelineNode struct {
	NodeID       string     `json:"node_id"`
	WorkID       string     `json:"work_id"`
	ContinuityID string     `json:"continuity_id"`
	NodeKey      string     `json:"node_key"`
	Label        string     `json:"label"`
	Ordinal      int64      `json:"ordinal"`
	ParentNodeID string     `json:"parent_node_id,omitempty"`
	BranchKey    string     `json:"branch_key"`
	NodeKind     string     `json:"node_kind"`
	MetadataJSON string     `json:"metadata_json,omitempty"`
	ReviewStatus string     `json:"review_status"`
	ReviewSource string     `json:"review_source,omitempty"`
	ReviewReason string     `json:"review_reason,omitempty"`
	ReviewedAt   *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type ReferenceEntity struct {
	EntityID        string     `json:"entity_id"`
	WorkID          string     `json:"work_id"`
	ContinuityID    string     `json:"continuity_id"`
	EntityType      string     `json:"entity_type"`
	CanonicalName   string     `json:"canonical_name"`
	DescriptionText string     `json:"description_text,omitempty"`
	MetadataJSON    string     `json:"metadata_json,omitempty"`
	ReviewStatus    string     `json:"review_status"`
	ReviewSource    string     `json:"review_source,omitempty"`
	ReviewReason    string     `json:"review_reason,omitempty"`
	ReviewedAt      *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type ReferenceEntityAlias struct {
	AliasID         int64     `json:"alias_id"`
	WorkID          string    `json:"work_id"`
	ContinuityID    string    `json:"continuity_id"`
	EntityID        string    `json:"entity_id"`
	AliasText       string    `json:"alias_text"`
	NormalizedAlias string    `json:"normalized_alias"`
	LanguageCode    string    `json:"language_code"`
	CreatedAt       time.Time `json:"created_at"`
}

type ReferenceClaim struct {
	ClaimID          string     `json:"claim_id"`
	WorkID           string     `json:"work_id"`
	ContinuityID     string     `json:"continuity_id"`
	DocumentID       string     `json:"document_id"`
	ClaimType        string     `json:"claim_type"`
	SubjectEntityID  string     `json:"subject_entity_id,omitempty"`
	ClaimText        string     `json:"claim_text"`
	EvidenceExcerpt  string     `json:"evidence_excerpt,omitempty"`
	TemporalScope    string     `json:"temporal_scope"`
	ValidFromNodeID  string     `json:"valid_from_node_id,omitempty"`
	ValidToNodeID    string     `json:"valid_to_node_id,omitempty"`
	RevealFromNodeID string     `json:"reveal_from_node_id,omitempty"`
	BranchKey        string     `json:"branch_key"`
	KnowledgeScope   string     `json:"knowledge_scope"`
	Confidence       float64    `json:"confidence"`
	ReviewStatus     string     `json:"review_status"`
	ReviewSource     string     `json:"review_source,omitempty"`
	ReviewReason     string     `json:"review_reason,omitempty"`
	ReviewedAt       *time.Time `json:"reviewed_at,omitempty"`
	MetadataJSON     string     `json:"metadata_json,omitempty"`
	KnowerEntityIDs  []string   `json:"knower_entity_ids,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type SessionReferenceBinding struct {
	BindingID           string    `json:"binding_id"`
	ChatSessionID       string    `json:"chat_session_id"`
	WorkID              string    `json:"work_id"`
	ContinuityID        string    `json:"continuity_id"`
	BindingRole         string    `json:"binding_role"`
	Enabled             bool      `json:"enabled"`
	AnchorMode          string    `json:"anchor_mode"`
	CurrentNodeID       string    `json:"current_node_id,omitempty"`
	RevealCeilingNodeID string    `json:"reveal_ceiling_node_id,omitempty"`
	DivergenceNodeID    string    `json:"divergence_node_id,omitempty"`
	FuturePolicy        string    `json:"future_policy"`
	Priority            int       `json:"priority"`
	Revision            int64     `json:"revision"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type SessionReferenceRuntime struct {
	BindingID             string    `json:"binding_id"`
	CandidateNodeID       string    `json:"candidate_node_id,omitempty"`
	CandidateSourceTurn   int       `json:"candidate_source_turn,omitempty"`
	CandidateEvidenceJSON string    `json:"candidate_evidence_json,omitempty"`
	CandidateConfirmed    bool      `json:"candidate_confirmed"`
	LastClaimIDsJSON      string    `json:"last_claim_ids_json,omitempty"`
	DiagnosticsJSON       string    `json:"diagnostics_json,omitempty"`
	Revision              int64     `json:"revision"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// ReferenceLibraryStore is an optional extension. Reference material remains
// independent from the session-scoped canonical Store and is never addressed
// through rollback methods.
type ReferenceLibraryStore interface {
	CreateReferenceWork(context.Context, *ReferenceWork) error
	GetReferenceWork(context.Context, string) (*ReferenceWork, error)
	ListReferenceWorks(context.Context, string, int) ([]ReferenceWork, error)
	UpdateReferenceWork(context.Context, *ReferenceWork, int64) error
	DeleteReferenceWork(context.Context, string) error

	UpsertReferenceContinuity(context.Context, *ReferenceContinuity) error
	ListReferenceContinuities(context.Context, string) ([]ReferenceContinuity, error)
	DeleteReferenceContinuity(context.Context, string) error

	SaveReferenceDocument(context.Context, *ReferenceDocument) error
	GetReferenceDocument(context.Context, string) (*ReferenceDocument, error)
	ListReferenceDocuments(context.Context, string, string, string) ([]ReferenceDocument, error)
	UpdateReferenceDocumentStatus(context.Context, string, string) error
	DeleteReferenceDocument(context.Context, string) error

	UpsertReferenceTimelineNode(context.Context, *ReferenceTimelineNode) error
	ListReferenceTimelineNodes(context.Context, string, string, string) ([]ReferenceTimelineNode, error)
	DeleteReferenceTimelineNode(context.Context, string) error

	UpsertReferenceEntity(context.Context, *ReferenceEntity) error
	ListReferenceEntities(context.Context, string, string, string) ([]ReferenceEntity, error)
	UpsertReferenceEntityAlias(context.Context, *ReferenceEntityAlias) error
	ListReferenceEntityAliases(context.Context, string) ([]ReferenceEntityAlias, error)
	DeleteReferenceEntity(context.Context, string) error

	UpsertReferenceClaim(context.Context, *ReferenceClaim) error
	ListReferenceClaims(context.Context, string, string, string, string) ([]ReferenceClaim, error)
	ReplaceReferenceClaimKnowers(context.Context, string, []string) error
	DeleteReferenceClaim(context.Context, string) error
	UpdateReferenceCandidateReview(context.Context, string, string, string, string, string, string) error

	UpsertSessionReferenceBinding(context.Context, *SessionReferenceBinding, int64) error
	ListSessionReferenceBindings(context.Context, string, bool) ([]SessionReferenceBinding, error)
	DeleteSessionReferenceBinding(context.Context, string, string) error
	UpsertSessionReferenceRuntime(context.Context, *SessionReferenceRuntime, int64) error
	GetSessionReferenceRuntime(context.Context, string) (*SessionReferenceRuntime, error)
}
