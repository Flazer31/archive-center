package store

import (
	"context"
	"errors"
	"time"
)

const (
	EntityIdentityLinkKindCanonicalEquivalence = "canonical_equivalence"
	EntityIdentityLinkStateReviewed            = "reviewed"
	EntityIdentityReviewStateReviewed          = "reviewed"
)

var ErrReviewedEntityIdentityAmbiguous = errors.New("reviewed entity identity has multiple canonical targets")

// EntityIdentity is an immutable, namespace-scoped identity occurrence.
// Display labels and aliases are stored separately and never act as identity
// keys. Separate occurrences may later be connected by reviewed links without
// rewriting their source-bound IDs.
type EntityIdentity struct {
	StableEntityID      string    `json:"stable_entity_id"`
	ChatSessionID       string    `json:"chat_session_id"`
	IdentityNamespace   string    `json:"identity_namespace"`
	EntityKind          string    `json:"entity_kind"`
	CanonicalLabel      string    `json:"canonical_label"`
	LifecycleState      string    `json:"lifecycle_state"`
	ReviewState         string    `json:"review_state"`
	PresenceAuthority   string    `json:"presence_authority"`
	OccurrenceAuthority string    `json:"occurrence_authority"`
	SourceContract      string    `json:"source_contract"`
	SourceRevision      string    `json:"source_revision"`
	SourceLogicalTurnID string    `json:"source_logical_turn_id"`
	SourceMessageID     string    `json:"source_message_id"`
	SourceGenerationID  string    `json:"source_generation_id"`
	SourceContentHash   string    `json:"source_content_hash"`
	SourceTurn          int       `json:"source_turn"`
	SourceIndex         int       `json:"source_index"`
	IdempotencyKey      string    `json:"idempotency_key"`
	MappingRevision     int       `json:"mapping_revision"`
	FirstSeenTurn       int       `json:"first_seen_turn"`
	LastSeenTurn        int       `json:"last_seen_turn"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// EntityIdentitySurface records one source-linked display name or alias.
// normalized_surface is indexed for candidate discovery only; it is not unique
// and must never be used as an automatic merge key.
type EntityIdentitySurface struct {
	SurfaceID         string    `json:"surface_id"`
	StableEntityID    string    `json:"stable_entity_id"`
	ChatSessionID     string    `json:"chat_session_id"`
	IdentityNamespace string    `json:"identity_namespace"`
	SurfaceKind       string    `json:"surface_kind"`
	SurfaceText       string    `json:"surface_text"`
	NormalizedSurface string    `json:"normalized_surface"`
	Scope             string    `json:"scope"`
	ValidFromTurn     int       `json:"valid_from_turn"`
	ValidToTurn       int       `json:"valid_to_turn"`
	SourceContract    string    `json:"source_contract"`
	SourceRevision    string    `json:"source_revision"`
	SourceTurn        int       `json:"source_turn"`
	SourceSpanStart   int       `json:"source_span_start"`
	SourceSpanEnd     int       `json:"source_span_end"`
	EvidenceExcerpt   string    `json:"evidence_excerpt"`
	ReviewState       string    `json:"review_state"`
	IdempotencyKey    string    `json:"idempotency_key"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// EntityIdentityArtifactBinding connects a legacy projection occurrence to a
// stable identity without changing or renumbering the legacy row.
type EntityIdentityArtifactBinding struct {
	BindingID       string    `json:"binding_id"`
	StableEntityID  string    `json:"stable_entity_id"`
	ChatSessionID   string    `json:"chat_session_id"`
	ArtifactKind    string    `json:"artifact_kind"`
	ArtifactRole    string    `json:"artifact_role"`
	ArtifactOrdinal int       `json:"artifact_ordinal"`
	SurfaceText     string    `json:"surface_text"`
	ReviewState     string    `json:"review_state"`
	SourceContract  string    `json:"source_contract"`
	SourceRevision  string    `json:"source_revision"`
	SourceTurn      int       `json:"source_turn"`
	IdempotencyKey  string    `json:"idempotency_key"`
	CreatedAt       time.Time `json:"created_at"`
}

// SpeakerAttribution separates the host message role from an in-world
// speaker. Ambiguous attribution retains the grounded raw span and points to a
// source-bound unknown identity instead of guessing a character.
type SpeakerAttribution struct {
	AttributionID     string    `json:"attribution_id"`
	ChatSessionID     string    `json:"chat_session_id"`
	SpeakerEntityID   string    `json:"speaker_entity_id"`
	IdentityNamespace string    `json:"identity_namespace"`
	SourceRole        string    `json:"source_role"`
	AttributionKind   string    `json:"attribution_kind"`
	AttributionState  string    `json:"attribution_state"`
	ReviewState       string    `json:"review_state"`
	Confidence        float64   `json:"confidence"`
	SourceContract    string    `json:"source_contract"`
	SourceRevision    string    `json:"source_revision"`
	SourceLogicalTurn string    `json:"source_logical_turn_id"`
	SourceMessageID   string    `json:"source_message_id"`
	SourceGeneration  string    `json:"source_generation_id"`
	SourceContentHash string    `json:"source_content_hash"`
	SourceTurn        int       `json:"source_turn"`
	SourceSpanStart   int       `json:"source_span_start"`
	SourceSpanEnd     int       `json:"source_span_end"`
	EvidenceExcerpt   string    `json:"evidence_excerpt"`
	IdempotencyKey    string    `json:"idempotency_key"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// EntityIdentityWriter is an additive extension. Store remains the compatibility
// contract while MariaDB and dual-write modes can persist the v1 projection.
type EntityIdentityWriter interface {
	SaveEntityIdentity(context.Context, *EntityIdentity) error
	SaveEntityIdentitySurface(context.Context, *EntityIdentitySurface) error
	SaveEntityIdentityArtifactBinding(context.Context, *EntityIdentityArtifactBinding) error
	SaveSpeakerAttribution(context.Context, *SpeakerAttribution) error
}

// EntityIdentityWriteAvailability lets composite stores expose whether at
// least one owned persistence lane can actually accept the optional extension.
type EntityIdentityWriteAvailability interface {
	EntityIdentityWritesEnabled() bool
}

// ReviewedEntityIdentityResolver resolves only an explicit, reviewed,
// directional source occurrence -> canonical target link. Implementations must
// never discover or merge identities by display label or surface text.
type ReviewedEntityIdentityResolver interface {
	ResolveReviewedCanonicalEntityID(ctx context.Context, chatSessionID, sourceEntityID string) (string, error)
}
