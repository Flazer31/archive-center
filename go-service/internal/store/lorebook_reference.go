package store

import (
	"context"
	"errors"
	"time"
)

const (
	LorebookReferenceSnapshotContractV1 = "lorebook_reference_snapshot.v1"
	LorebookReferenceCurrentViewV1      = "lorebook_reference_current.viewmodel.v1"

	LorebookConsentActive  = "active"
	LorebookConsentRevoked = "revoked"

	LorebookObservationObserved    = "observed"
	LorebookObservationPartial     = "partial"
	LorebookObservationUnavailable = "unavailable"

	LorebookLifecycleCurrent        = "catalog_current"
	LorebookLifecycleStale          = "catalog_stale"
	LorebookLifecyclePartial        = "observed_partial"
	LorebookLifecycleConsentRevoked = "consent_revoked"
)

var ErrInvalidLorebookReference = errors.New("invalid lorebook reference snapshot")

// LorebookReferenceScope is the exact Host scope observed by the thin adapter.
// Nil indexes stay unobserved; they are never replaced with zero or inferred.
type LorebookReferenceScope struct {
	ChatSessionID          string   `json:"chat_session_id"`
	CharacterIndex         *int64   `json:"character_index,omitempty"`
	ChatIndex              *int64   `json:"chat_index,omitempty"`
	EnabledModuleIDs       []string `json:"enabled_module_ids"`
	EnabledModulesObserved bool     `json:"enabled_modules_observed"`
}

// LorebookReferenceEntryObservation mirrors only fields exposed by the
// official RisuAI/PocketRisu lorebook snapshot API. Pointer fields preserve the
// difference between false/zero and values the Host did not expose.
type LorebookReferenceEntryObservation struct {
	HostEntryID      string   `json:"host_entry_id,omitempty"`
	EntryOrdinal     int      `json:"entry_ordinal"`
	SourceKind       string   `json:"source_kind"`
	SourceIdentity   string   `json:"source_identity,omitempty"`
	Key              string   `json:"key"`
	SecondKey        string   `json:"second_key"`
	Comment          string   `json:"comment"`
	Content          string   `json:"content"`
	NormalizedSearch string   `json:"normalized_search_text"`
	Mode             string   `json:"mode,omitempty"`
	AlwaysActive     *bool    `json:"always_active,omitempty"`
	Selective        *bool    `json:"selective,omitempty"`
	UseRegex         *bool    `json:"use_regex,omitempty"`
	InsertOrder      *int     `json:"insert_order,omitempty"`
	ActivationPct    *float64 `json:"activation_percent,omitempty"`
	BookVersion      *int64   `json:"book_version,omitempty"`
	Folder           string   `json:"folder,omitempty"`
	ExtensionsJSON   string   `json:"extensions_json"`
	ContentHash      string   `json:"content_hash,omitempty"`
}

type LorebookReferenceSnapshot struct {
	SnapshotID       string                              `json:"snapshot_id"`
	ContractVersion  string                              `json:"contract_version"`
	ConsentState     string                              `json:"consent_state"`
	ObservationState string                              `json:"observation_state"`
	CompleteSnapshot bool                                `json:"complete_snapshot"`
	Scope            LorebookReferenceScope              `json:"scope"`
	Entries          []LorebookReferenceEntryObservation `json:"entries"`
	ProvenanceJSON   string                              `json:"provenance_json"`
	ObservedAt       time.Time                           `json:"observed_at"`
}

type LorebookReferenceSnapshotResult struct {
	ScopeID              int64  `json:"scope_id"`
	SnapshotID           string `json:"snapshot_id"`
	ObservationState     string `json:"observation_state"`
	LifecycleAction      string `json:"lifecycle_action"`
	ObservedEntryCount   int    `json:"observed_entry_count"`
	CurrentEntryCount    int    `json:"current_entry_count"`
	PreviousCurrentCount int64  `json:"previous_current_count"`
}

type LorebookReferenceCurrent struct {
	ScopeID        int64                               `json:"scope_id"`
	Scope          LorebookReferenceScope              `json:"scope"`
	LatestSnapshot *LorebookReferenceSnapshot          `json:"latest_snapshot,omitempty"`
	Entries        []LorebookReferenceEntryObservation `json:"entries"`
}

// LorebookReferenceCurrentPage is a bounded UI projection of the separate
// lorebook-reference store. It never promotes Host lore into canonical memory.
type LorebookReferenceCurrentPage struct {
	ScopeID        int64                               `json:"scope_id"`
	Scope          LorebookReferenceScope              `json:"scope"`
	LatestSnapshot *LorebookReferenceSnapshot          `json:"latest_snapshot,omitempty"`
	Entries        []LorebookReferenceEntryObservation `json:"items"`
	Total          int                                 `json:"total"`
	Limit          int                                 `json:"limit"`
	Offset         int                                 `json:"offset"`
}

// LorebookReferenceStore is deliberately separate from canonical Store.
// Host lore cannot be admitted as memory/evidence/entity/world-rule data by
// satisfying this interface.
type LorebookReferenceStore interface {
	ApplyLorebookReferenceSnapshot(context.Context, *LorebookReferenceSnapshot) (*LorebookReferenceSnapshotResult, error)
	GetLorebookReferenceCurrent(context.Context, LorebookReferenceScope) (*LorebookReferenceCurrent, error)
}

// LorebookReferenceExplorerStore is optional so prepare-turn readers remain
// independent from the UI-only bounded projection.
type LorebookReferenceExplorerStore interface {
	GetLorebookReferenceCurrentPage(context.Context, LorebookReferenceScope, int, int) (*LorebookReferenceCurrentPage, error)
	GetLorebookReferenceLatestSessionPage(context.Context, string, int, int) (*LorebookReferenceCurrentPage, error)
}
