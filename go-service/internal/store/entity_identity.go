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
	EntityIdentityReviewStateSourceObserved    = "source_observed"
	EntityIdentitySurfaceScope39               = "source_turn"
	EntityIdentitySurfaceScopeCurrent          = "source_turn_current"
)

var ErrReviewedEntityIdentityAmbiguous = errors.New("reviewed entity identity has multiple canonical targets")

// EntityIdentity is a namespace-scoped canonical identity. An exact repeated
// canonical label with the same namespace and entity kind may reuse its stable
// ID; turn-specific provenance remains on surfaces and artifact bindings.
// Aliases and non-exact labels never act as automatic identity keys.
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
// and is insufficient without an exact canonical label, namespace, and kind.
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

// EntityIdentityLink records an evidence-backed, reversible mapping between
// two immutable identity occurrences. LinkState controls whether read paths
// may canonicalize through the link; surface similarity alone never creates a
// reviewed link.
type EntityIdentityLink struct {
	LinkID          string    `json:"link_id"`
	ChatSessionID   string    `json:"chat_session_id"`
	SourceEntityID  string    `json:"source_entity_id"`
	TargetEntityID  string    `json:"target_entity_id"`
	LinkKind        string    `json:"link_kind"`
	LinkState       string    `json:"link_state"`
	EvidenceJSON    string    `json:"evidence_json"`
	MappingRevision int       `json:"mapping_revision"`
	SourceContract  string    `json:"source_contract"`
	SourceRevision  string    `json:"source_revision"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
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

// EntityIdentityLinkWriter is optional so stores that only support occurrence
// projection do not gain a new required method.
type EntityIdentityLinkWriter interface {
	SaveEntityIdentityLink(context.Context, *EntityIdentityLink) error
}

// ReviewedEntityIdentityResolver resolves only an explicit, reviewed,
// directional source occurrence -> canonical target link. Implementations must
// never discover or merge identities by display label or surface text.
type ReviewedEntityIdentityResolver interface {
	ResolveReviewedCanonicalEntityID(ctx context.Context, chatSessionID, sourceEntityID string) (string, error)
}

// UniqueActiveEntitySurfaceResolver resolves a source-observed surface when all
// active occurrences converge on one canonical identity. Multiple exact
// display-name rows may resolve to the earliest ID only when canonical label,
// namespace, and entity kind are identical.
type UniqueActiveEntitySurfaceResolver interface {
	ResolveUniqueActiveEntityIDBySurface(ctx context.Context, chatSessionID, normalizedSurface string) (string, error)
}

// ResolvedEntityIdentity is the database-owned result of a unique active
// surface resolution. Callers that make namespace-sensitive admission
// decisions must use both fields instead of accepting a model-proposed
// namespace for a resolved ID.
type ResolvedEntityIdentity struct {
	StableEntityID    string `json:"stable_entity_id"`
	IdentityNamespace string `json:"identity_namespace"`
	EntityKind        string `json:"entity_kind"`
	CanonicalLabel    string `json:"canonical_label"`
}

// UniqueActiveEntitySurfaceIdentityResolver extends the legacy ID-only
// resolver without changing its existing callers.
type UniqueActiveEntitySurfaceIdentityResolver interface {
	ResolveUniqueActiveEntityIdentityBySurface(ctx context.Context, chatSessionID, normalizedSurface string) (ResolvedEntityIdentity, error)
}
