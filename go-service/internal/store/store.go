// Package store defines the canonical truth storage interface for Archive Center 2.0.
// All implementations in R0/R1 are either no-op or explicitly disabled.
package store

import (
	"context"
	"errors"
	"time"
)

// Common errors.
var (
	ErrNotFound   = errors.New("record not found")
	ErrNotEnabled = errors.New("mariadb store is not enabled in R0/R1")
)

// ShadowStatusReporter is implemented by stores that expose shadow-side health.
// In R1 this is used by the dual-write wrapper to report shadow failures
// without leaking the unexported type.
type ShadowStatusReporter interface {
	ShadowStatus() (failures int64, lastErr error)
}

// Pinger is implemented by stores that can verify a live backend connection.
type Pinger interface {
	Ping(ctx context.Context) error
}

// SessionActiveScope stores the current world-rule scope selected for a session.
type SessionActiveScope struct {
	ID            int64     `json:"id"`
	ChatSessionID string    `json:"chat_session_id"`
	ActiveScope   string    `json:"active_scope"`
	ScopeName     string    `json:"scope_name"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ActiveScopeStore is optional. Stores that implement it can persist the
// I-4 World Graph Lite active scope used by inherited world-rule reads.
type ActiveScopeStore interface {
	GetActiveScope(ctx context.Context, chatSessionID string) (*SessionActiveScope, error)
	UpsertActiveScope(ctx context.Context, item *SessionActiveScope) error
}

// AuditLogCounter is an optional read-side helper for list endpoints that
// need a Python-compatible total while keeping ListAuditLogs bounded.
type AuditLogCounter interface {
	CountAuditLogs(ctx context.Context, chatSessionID string, eventType string) (int, error)
}

// SupersessionResolutionContractVersion identifies the audit-preserving
// resolution mini-ledger used by 22-2.
const SupersessionResolutionContractVersion = "supersession_resolution.v1"

// SupersessionResolutionAfterglowTurns is the default window during which a
// resolved item may still be shown as background context before it fades.
const SupersessionResolutionAfterglowTurns = 5

// SupersessionResolutionDecision describes a bounded state transition between
// old and new archive facts without requiring hard deletion.
type SupersessionResolutionDecision struct {
	ChatSessionID   string
	TargetType      string
	TargetID        int64
	SourceTurn      int
	ResolutionClass string
	NewTargetType   string
	NewTargetID     int64
	RelationshipKey string
	Reason          string
	EvidenceJSON    string
	Operator        string
}

// SupersessionResolutionRecord is a read model backed by audit_logs.
type SupersessionResolutionRecord struct {
	ID              int64
	CreatedAt       time.Time
	ChatSessionID   string
	TargetType      string
	TargetID        int64
	SourceTurn      int
	ResolutionClass string
	NewTargetType   string
	NewTargetID     int64
	RelationshipKey string
	Reason          string
	DetailsJSON     string
	Source          string
}

// SupersessionResolutionStore is optional. Stores that implement it can persist
// explicit close/supersede/refine/reverse/stale-demote decisions while
// preserving audit history.
type SupersessionResolutionStore interface {
	SaveSupersessionResolution(ctx context.Context, d *SupersessionResolutionDecision) (*SupersessionResolutionRecord, error)
	ListSupersessionResolutions(ctx context.Context, chatSessionID string, limit int) ([]SupersessionResolutionRecord, error)
}

// SessionStateSnapshot is the aggregate read contract for the I-1 session-state
// endpoint. Implementations may fill the Trace fields to prove that the read
// happened through a bounded aggregate path instead of independent HTTP calls.
type SessionStateSnapshot struct {
	ActiveStates         []ActiveState
	CanonicalStateLayers []CanonicalStateLayer
	Storylines           []Storyline
	CharacterStates      []CharacterState
	WorldRules           []WorldRule
	PendingThreads       []PendingThread
	CharacterEvents      []CharacterEvent
	RecentChatLogs       []ChatLog
	SingleConnection     bool
	TraceMethods         []string
}

// SessionStateSnapshotReader is optional. Stores that implement it can serve
// /session-state through one aggregate store read. MariaDB uses this to keep
// the component queries inside one read-only transaction/connection.
type SessionStateSnapshotReader interface {
	ReadSessionStateSnapshot(ctx context.Context, chatSessionID string) (*SessionStateSnapshot, error)
}

// OptionalIntPatch represents an integer field that may be omitted, set to a
// concrete value, or explicitly set to NULL.
type OptionalIntPatch struct {
	Set   bool
	Value *int
}

// MemoryExplorerPatch is the bounded manual edit surface exposed by Explorer.
type MemoryExplorerPatch struct {
	SummaryJSON *string
	Importance  *float64
	PlaceWing   *string
	PlaceRoom   *string
}

// KGTripleExplorerPatch is the bounded manual edit surface for KG triples.
type KGTripleExplorerPatch struct {
	Subject   *string
	Predicate *string
	Object    *string
	ValidFrom OptionalIntPatch
	ValidTo   OptionalIntPatch
}

// DirectEvidenceExplorerPatch is the bounded manual edit surface for evidence
// review state transitions.
type DirectEvidenceExplorerPatch struct {
	ArchiveState        *string
	CaptureVerification *string
	CommittedGate       *string
	RepairNeeded        *bool
	Tombstoned          *bool
	SupersededByID      OptionalIntPatch
}

// ExplorerMutationStore is optional. Stores that implement it can accept
// session-scoped Explorer manual edits while every mutation remains paired
// with an audit log at the HTTP layer.
type ExplorerMutationStore interface {
	UpdateMemoryExplorerFields(ctx context.Context, chatSessionID string, memoryID int64, patch MemoryExplorerPatch) error
	UpdateKGTripleExplorerFields(ctx context.Context, chatSessionID string, tripleID int64, patch KGTripleExplorerPatch) error
	UpdateDirectEvidenceExplorerFields(ctx context.Context, chatSessionID string, recordID int64, patch DirectEvidenceExplorerPatch) error
	DeleteMemoryByID(ctx context.Context, chatSessionID string, memoryID int64) error
	DeleteDirectEvidenceByID(ctx context.Context, chatSessionID string, recordID int64) error
	DeleteKGTripleByID(ctx context.Context, chatSessionID string, tripleID int64) error
	DeleteCharacterByName(ctx context.Context, chatSessionID string, characterName string) error
}

// PersonaCapsuleStore is optional. Stores that implement it can persist
// protagonist/player recollection capsules that may be attached to another
// session as support-only context.
type PersonaCapsuleStore interface {
	CreatePersonaMemoryCapsule(ctx context.Context, capsule *PersonaMemoryCapsule, entries []PersonaMemoryEntry) (*PersonaMemoryCapsule, error)
	ListPersonaMemoryCapsules(ctx context.Context, filter PersonaCapsuleFilter) ([]PersonaMemoryCapsule, error)
	GetPersonaMemoryCapsule(ctx context.Context, capsuleID int64) (*PersonaMemoryCapsule, []PersonaMemoryEntry, error)
	DeletePersonaMemoryCapsule(ctx context.Context, capsuleID int64) error
	AttachPersonaMemoryCapsule(ctx context.Context, attachment *PersonaCapsuleAttachment) error
	DetachPersonaMemoryCapsule(ctx context.Context, capsuleID int64, targetChatSessionID string) error
	ListPersonaCapsuleAttachments(ctx context.Context, targetChatSessionID string) ([]PersonaCapsuleAttachment, error)
	ListAttachedPersonaMemoryEntries(ctx context.Context, targetChatSessionID string, limit int) ([]PersonaMemoryEntry, error)
}

// PersonaCapsuleFilter constrains capsule list reads.
type PersonaCapsuleFilter struct {
	PersonaKey          string
	SourceChatSessionID string
}

// ProtagonistEntityMemory is a source-session-scoped subjective memory owned
// by a protagonist/player entity such as "??곷뻻??. It is the bank from which
// support-only persona capsules can later be built.
type ProtagonistEntityMemory struct {
	ID                  int64     `json:"id"`
	PersonaEntityKey    string    `json:"persona_entity_key"`
	PersonaEntityName   string    `json:"persona_entity_name"`
	OwnerEntityKey      string    `json:"owner_entity_key"`
	OwnerEntityName     string    `json:"owner_entity_name"`
	OwnerEntityRole     string    `json:"owner_entity_role"`
	OwnerVisibility     string    `json:"owner_visibility"`
	SourceChatSessionID string    `json:"source_chat_session_id"`
	SourceCharacterName string    `json:"source_character_name"`
	SourceTurn          int       `json:"source_turn_index"`
	MemoryText          string    `json:"memory_text"`
	EvidenceExcerpt     string    `json:"evidence_excerpt"`
	SecretGuard         bool      `json:"secret_guard"`
	Portability         string    `json:"portability"`
	TargetRevealPolicy  string    `json:"target_reveal_policy"`
	TagsJSON            string    `json:"tags_json"`
	Importance10        float64   `json:"importance_10"`
	EmotionalWeight     float64   `json:"emotional_weight"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// ProtagonistEntityMemoryFilter constrains protagonist memory-bank reads.
type ProtagonistEntityMemoryFilter struct {
	PersonaEntityKey    string
	OwnerEntityKey      string
	OwnerEntityRole     string
	OwnerVisibility     string
	SourceChatSessionID string
	Limit               int
}

// ProtagonistEntityMemoryOwnerUpdate rewrites only the owner/persona identity
// fields of an existing subjective memory. It is used for alias repair after a
// dry-run plan has shown that split spellings point to the same entity.
type ProtagonistEntityMemoryOwnerUpdate struct {
	ID                int64
	PersonaEntityKey  string
	PersonaEntityName string
	OwnerEntityKey    string
	OwnerEntityName   string
	OwnerEntityRole   string
	OwnerVisibility   string
	TagsJSON          string
}

// ProtagonistEntityMemoryUpdate rewrites user-editable subjective memory
// fields. It is intentionally scoped to one row and does not cascade into
// derived memories, chat logs, or evidence rows.
type ProtagonistEntityMemoryUpdate struct {
	ID                  int64
	PersonaEntityKey    string
	PersonaEntityName   string
	OwnerEntityKey      string
	OwnerEntityName     string
	OwnerEntityRole     string
	OwnerVisibility     string
	SourceCharacterName string
	MemoryText          string
	EvidenceExcerpt     string
	SecretGuard         bool
	Portability         string
	TargetRevealPolicy  string
	TagsJSON            string
	Importance10        float64
	EmotionalWeight     float64
}

// ProtagonistEntityMemoryStore is optional. It stores protagonist-entity
// subjective memories separately from portable capsule bundles.
type ProtagonistEntityMemoryStore interface {
	CreateProtagonistEntityMemory(ctx context.Context, item *ProtagonistEntityMemory) (*ProtagonistEntityMemory, error)
	ListProtagonistEntityMemories(ctx context.Context, filter ProtagonistEntityMemoryFilter) ([]ProtagonistEntityMemory, error)
}

// ProtagonistEntityMemoryRepairStore is optional and intentionally narrower
// than general update/delete access. It supports safe alias canonicalization
// without allowing memory text or evidence mutation.
type ProtagonistEntityMemoryRepairStore interface {
	UpdateProtagonistEntityMemoryOwner(ctx context.Context, update ProtagonistEntityMemoryOwnerUpdate) error
}

// ProtagonistEntityMemoryManagementStore supports explicit user management
// from the Entity Memory Browser.
type ProtagonistEntityMemoryManagementStore interface {
	UpdateProtagonistEntityMemory(ctx context.Context, update ProtagonistEntityMemoryUpdate) error
	DeleteProtagonistEntityMemory(ctx context.Context, id int64) error
}

// EffectiveInputListStore is an optional read helper used by session migration.
// The main Store contract only needs point reads for normal runtime flow.
type EffectiveInputListStore interface {
	ListEffectiveInputs(ctx context.Context, chatSessionID string, fromTurn, toTurn int) ([]EffectiveInput, error)
}

// SessionMigrationArtifactCounts mirrors the tables copied by complete session
// migration. ChromaDB vectors are intentionally handled in a later step.
type SessionMigrationArtifactCounts struct {
	ChatLogs                    int `json:"chat_logs"`
	EffectiveInputs             int `json:"effective_inputs"`
	Memories                    int `json:"memories"`
	DirectEvidence              int `json:"direct_evidence"`
	KGTriples                   int `json:"kg_triples"`
	Episodes                    int `json:"episodes"`
	SubjectiveEntityMemories    int `json:"subjective_entity_memories"`
	CanonicalTotal              int `json:"canonical_total"`
	CanonicalAndSubjectiveTotal int `json:"canonical_and_subjective_total"`
}

// SessionMigrationCompleteRequest is the MariaDB copy phase for complete
// session migration. It is intentionally scoped to an empty target session.
type SessionMigrationCompleteRequest struct {
	SourceSessionID string
	TargetSessionID string
	Mode            string
	OperatorNote    string
}

// SessionMigrationCompleteResult reports the durable copy ledger written by
// SessionMigrationStore. ChromaDB reindexing is a separate later phase.
type SessionMigrationCompleteResult struct {
	MigrationID           int64
	Status                string
	SourceSessionID       string
	TargetSessionID       string
	Mode                  string
	Counts                SessionMigrationArtifactCounts
	RowMapCount           int
	ChromaReindexedCount  int
	SourceLocked          bool
	ChromaReindexRequired bool
	ReadyForLive          bool
}

// SessionMigrationVectorDocument is a copied target row that can be reindexed
// into ChromaDB using an embedding already stored in MariaDB.
type SessionMigrationVectorDocument struct {
	ID                    string
	MigrationID           int64
	Tier                  string
	ChatSessionID         string
	SourceTable           string
	SourceRowID           string
	SchemaVersion         string
	DocumentText          string
	EmbeddingJSON         string
	MigratedFromSessionID string
}

// SessionMigrationStore performs the write phase of session migration.
// Implementations must use one transaction and write row provenance.
type SessionMigrationStore interface {
	CompleteSessionMigration(ctx context.Context, req SessionMigrationCompleteRequest) (*SessionMigrationCompleteResult, error)
}

// SessionMigrationVectorStore exposes copied rows that need ChromaDB reindexing
// and persists the vector verification result back into the migration ledger.
type SessionMigrationVectorStore interface {
	ListSessionMigrationVectorDocuments(ctx context.Context, migrationID int64) ([]SessionMigrationVectorDocument, error)
	UpdateSessionMigrationVectorStatus(ctx context.Context, migrationID int64, status string, reindexedCount int, errorsJSON string) error
}

// SessionMigrationLock represents a source session that has been migrated away
// and must no longer act as a live memory owner.
type SessionMigrationLock struct {
	MigrationID     int64
	SourceSessionID string
	TargetSessionID string
	Locked          bool
	LockStatus      string
	Reason          string
	LockedAt        time.Time
}

// SessionMigrationSourceLockResult reports the backend safety lock written
// after ChromaDB reindex verification.
type SessionMigrationSourceLockResult struct {
	MigrationID     int64
	SourceSessionID string
	TargetSessionID string
	Status          string
	Lock            SessionMigrationLock
	ReadyForLive    bool
}

// SessionMigrationSourceLockStore writes and reads migrated-away source locks.
type SessionMigrationSourceLockStore interface {
	LockSessionMigrationSource(ctx context.Context, migrationID int64, reason string) (*SessionMigrationSourceLockResult, error)
	GetSessionMigrationSourceLock(ctx context.Context, sourceSessionID string) (*SessionMigrationLock, error)
}

// SessionMigrationRollbackResult reports a ledger-scoped rollback. It deletes
// only copied target rows recorded in session_migration_row_map.
type SessionMigrationRollbackResult struct {
	MigrationID     int64
	SourceSessionID string
	TargetSessionID string
	Status          string
	Counts          SessionMigrationArtifactCounts
	RowMapCount     int
	SourceUnlocked  bool
	ReadyForLive    bool
}

// SessionMigrationCleanupPreview reports whether an abandoned source session
// can be safely cleaned after a successful copy, vector reindex, and source lock.
type SessionMigrationCleanupPreview struct {
	MigrationID     int64
	SourceSessionID string
	TargetSessionID string
	Status          string
	SourceLocked    bool
	Counts          SessionMigrationArtifactCounts
	BlockedReasons  []string
	ReadyForCleanup bool
}

// SessionMigrationCleanupResult reports an operator-confirmed source cleanup.
type SessionMigrationCleanupResult struct {
	MigrationID     int64
	SourceSessionID string
	TargetSessionID string
	Status          string
	Counts          SessionMigrationArtifactCounts
	SourceCleaned   bool
	ReadyForLive    bool
}

// SessionMigrationRecoveryStore performs safe post-copy recovery operations:
// target rollback by ledger row map and source cleanup after lock.
type SessionMigrationRecoveryStore interface {
	RollbackSessionMigration(ctx context.Context, migrationID int64, reason string) (*SessionMigrationRollbackResult, error)
	PreviewSessionMigrationSourceCleanup(ctx context.Context, migrationID int64) (*SessionMigrationCleanupPreview, error)
	CleanupSessionMigrationSource(ctx context.Context, migrationID int64, reason string) (*SessionMigrationCleanupResult, error)
}

// AdminResetResult reports the scope of a destructive full database reset.
type AdminResetResult struct {
	TablesCleared int   `json:"tables_cleared"`
	RowsDeleted   int64 `json:"rows_deleted"`
}

// AdminResetStore is an explicit operator/debug-only extension. It clears all
// application rows while preserving the schema so the service can restart.
type AdminResetStore interface {
	ResetAll(ctx context.Context) (AdminResetResult, error)
}

// Store is the canonical truth storage contract.
// It covers the 8 immutable tables identified in the MariaDB schema plan.
type Store interface {
	// Chat logs - append-only per turn.
	SaveChatLog(ctx context.Context, log *ChatLog) error
	ListChatLogs(ctx context.Context, chatSessionID string, fromTurn, toTurn int) ([]ChatLog, error)

	// Effective inputs - append-only per turn.
	SaveEffectiveInput(ctx context.Context, in *EffectiveInput) error
	GetEffectiveInput(ctx context.Context, chatSessionID string, turnIndex int) (*EffectiveInput, error)

	// Memories - append-only per turn (core retrieval index).
	SaveMemory(ctx context.Context, m *Memory) error
	ListMemories(ctx context.Context, chatSessionID string, fromTurn, toTurn int) ([]Memory, error)

	// Direct evidence - append-only with state transitions tracked via new rows.
	SaveEvidence(ctx context.Context, e *DirectEvidence) error
	ListEvidence(ctx context.Context, chatSessionID string) ([]DirectEvidence, error)

	// Knowledge graph triples - append-only with soft-delete (valid_to).
	SaveKGTriple(ctx context.Context, t *KGTriple) error
	ListKGTriples(ctx context.Context, chatSessionID string) ([]KGTriple, error)

	// Audit logs - append-only by definition.
	SaveAuditLog(ctx context.Context, a *AuditLog) error
	ListAuditLogs(ctx context.Context, chatSessionID string, eventType string, limit int) ([]AuditLog, error)

	// Critic feedback - append-only.
	SaveCriticFeedback(ctx context.Context, f *CriticFeedback) error
	ListCriticFeedback(ctx context.Context, chatSessionID string, targetType string, targetID int64) ([]CriticFeedback, error)

	// Character events - append-only.
	SaveCharacterEvent(ctx context.Context, e *CharacterEvent) error
	ListCharacterEvents(ctx context.Context, chatSessionID string, characterName string) ([]CharacterEvent, error)
	// Read-only aggregations (R1)
	Stats(ctx context.Context) (StatsResult, error)
	ListSessions(ctx context.Context) ([]SessionSummary, error)

	// Resume pack - read-only narrative hierarchy summary (R1).
	GetResumePack(ctx context.Context, chatSessionID string, trigger string) (*ResumePack, error)

	// Storylines - R1 read.
	ListStorylines(ctx context.Context, chatSessionID string) ([]Storyline, error)

	// World rules - R1 read.
	ListWorldRules(ctx context.Context, chatSessionID string) ([]WorldRule, error)
	ListInheritedWorldRules(ctx context.Context, chatSessionID string, activeScope, scopeName string) ([]WorldRule, error)

	// Character states - R1 read.
	ListCharacterStates(ctx context.Context, chatSessionID string) ([]CharacterState, error)
	GetCharacterState(ctx context.Context, chatSessionID, characterName string) (*CharacterState, error)

	// Pending threads - R1 read.
	ListPendingThreads(ctx context.Context, chatSessionID, status string) ([]PendingThread, error)

	// Active states - R1 read.
	ListActiveStates(ctx context.Context, chatSessionID, stateType string) ([]ActiveState, error)

	// Canonical state layers - R1 read.
	ListCanonicalStateLayers(ctx context.Context, chatSessionID, layerType string) ([]CanonicalStateLayer, error)

	// Episode summaries - R1 read.
	ListEpisodeSummaries(ctx context.Context, chatSessionID string, limit, fromTurn, toTurn int) ([]EpisodeSummary, error)
	GetEpisodeSummary(ctx context.Context, episodeID int64) (*EpisodeSummary, error)
}

// CharacterStateHistoryStore is an optional R1 extension for append-only
// character state snapshot history. Normal current-state readers should keep
// using ListCharacterStates/GetCharacterState so old snapshots do not leak into
// prompt assembly or the default Explorer list.
type CharacterStateHistoryStore interface {
	ListCharacterStateHistory(ctx context.Context, chatSessionID, characterName string, limit, offset int) ([]CharacterState, error)
}

// RollbackStore is an optional extension for stores that support
// rollback, reroll, and session delete mutations.
type RollbackStore interface {
	// DeleteChatLogs removes chat logs for the session from turn_index onward.
	DeleteChatLogs(ctx context.Context, chatSessionID string, fromTurn int) error

	// DeleteEffectiveInputs removes effective inputs for the session from turn_index onward.
	DeleteEffectiveInputs(ctx context.Context, chatSessionID string, fromTurn int) error

	// DeleteMemories removes memories for the session from turn_index onward.
	DeleteMemories(ctx context.Context, chatSessionID string, fromTurn int) error

	// DeleteEvidence removes direct evidence for the session from turn_index onward.
	DeleteEvidence(ctx context.Context, chatSessionID string, fromTurn int) error

	// DeleteKGTriples removes KG triples for the session from source_turn onward.
	DeleteKGTriples(ctx context.Context, chatSessionID string, fromTurn int) error

	// DeleteCriticFeedback removes critic feedback tied to turn targets from turn_index onward.
	DeleteCriticFeedback(ctx context.Context, chatSessionID string, fromTurn int) error

	// DeleteCharacterEvents removes character events for the session from turn_index onward.
	DeleteCharacterEvents(ctx context.Context, chatSessionID string, fromTurn int) error

	// DeleteEntities removes entities first seen from the rollback turn onward and
	// truncates earlier entity seen ranges instead of deleting established rows.
	DeleteEntities(ctx context.Context, chatSessionID string, fromTurn int) error

	// DeleteTrustStates removes trust snapshots for the session from source_turn onward.
	DeleteTrustStates(ctx context.Context, chatSessionID string, fromTurn int) error

	// DeleteStorylines removes storylines for the session whose turn range overlaps.
	DeleteStorylines(ctx context.Context, chatSessionID string, fromTurn int) error

	// DeleteWorldRules removes world rules for the session from source_turn onward.
	DeleteWorldRules(ctx context.Context, chatSessionID string, fromTurn int) error

	// DeleteCharacterStates removes append-only character state snapshots for the
	// session from turn_index onward; older snapshots remain as rollback fallback.
	DeleteCharacterStates(ctx context.Context, chatSessionID string, fromTurn int) error

	// DeletePendingThreads removes pending threads for the session from source_turn onward.
	DeletePendingThreads(ctx context.Context, chatSessionID string, fromTurn int) error

	// DeleteActiveStates removes active states for the session from turn_index onward.
	DeleteActiveStates(ctx context.Context, chatSessionID string, fromTurn int) error

	// DeleteCanonicalStateLayers removes canonical state layers for the session from turn_index onward.
	DeleteCanonicalStateLayers(ctx context.Context, chatSessionID string, fromTurn int) error

	// DeleteEpisodeSummaries removes episode summaries for the session whose turn range overlaps.
	DeleteEpisodeSummaries(ctx context.Context, chatSessionID string, fromTurn int) error

	// DeleteGuidancePlanState invalidates the cached guidance plan for the session
	// by resetting state_status to empty and last_turn to -1 when last_turn >= fromTurn.
	DeleteGuidancePlanState(ctx context.Context, chatSessionID string, fromTurn int) error

	// DeleteChapterSummaries removes chapter summaries whose turn range overlaps the rollback turn.
	DeleteChapterSummaries(ctx context.Context, chatSessionID string, fromTurn int) error

	// DeleteArcSummaries removes arc summaries whose turn range overlaps the rollback turn.
	DeleteArcSummaries(ctx context.Context, chatSessionID string, fromTurn int) error

	// DeleteSagaDigests removes saga digests whose turn range overlaps the rollback turn.
	DeleteSagaDigests(ctx context.Context, chatSessionID string, fromTurn int) error

	// DeleteSessionActiveScopes clears the session active-scope cache after rollback.
	DeleteSessionActiveScopes(ctx context.Context, chatSessionID string, fromTurn int) error

	// DeleteProtagonistEntityMemories removes subjective entity memories for the session from source_turn onward.
	DeleteProtagonistEntityMemories(ctx context.Context, chatSessionID string, fromTurn int) error

	// DeleteConsequenceRecords removes Step 23 consequence records whose source range overlaps.
	DeleteConsequenceRecords(ctx context.Context, chatSessionID string, fromTurn int) error

	// DeletePsychologyBranches removes Step 23 psychology branches whose source range overlaps.
	DeletePsychologyBranches(ctx context.Context, chatSessionID string, fromTurn int) error

	// DeleteThemeOffscreenCarries removes Step 23 theme/offscreen records whose source range overlaps.
	DeleteThemeOffscreenCarries(ctx context.Context, chatSessionID string, fromTurn int) error

	// DeleteCaptureVerificationRecords removes per-turn capture verification rows from turn_index onward.
	DeleteCaptureVerificationRecords(ctx context.Context, chatSessionID string, fromTurn int) error

	// DeleteSession removes all session-scoped rows across all tables.
	DeleteSession(ctx context.Context, chatSessionID string) error
}

// ChatLog is the canonical turn record.
type ChatLog struct {
	ID            int64
	ChatSessionID string
	TurnIndex     int
	Role          string
	Content       string
	CreatedAt     time.Time
}

// EffectiveInput is the processed user intent per turn.
type EffectiveInput struct {
	ID             int64
	ChatSessionID  string
	TurnIndex      int
	EffectiveInput string
	CreatedAt      time.Time
}

// Memory is a core retrieval document.
type Memory struct {
	ID                    int64
	ChatSessionID         string
	TurnIndex             int
	SummaryJSON           string
	Embedding             string // JSON float array
	EmbeddingModel        string
	Similarity            float64
	Importance            float64
	EmotionalBoost        float64
	Evidence              string
	EmotionalIntensity    float64
	NarrativeSignificance float64
	PlaceWing             string
	PlaceRoom             string
	CreatedAt             time.Time
}

// DirectEvidence is a verified fact record.
type DirectEvidence struct {
	ID                   int64
	ChatSessionID        string
	EvidenceKind         string
	EvidenceText         string
	SourceTurnStart      int
	SourceTurnEnd        int
	TurnAnchor           int
	SourceMessageIDsJSON string
	SourceHash           string
	ArchiveState         string
	CaptureStage         string
	CaptureVerification  string
	CommittedGate        string
	LineageJSON          string
	RepairNeeded         bool
	Tombstoned           bool
	SupersededByID       int64
	CreatedAt            time.Time
}

// KGTriple is a knowledge graph edge.
type KGTriple struct {
	ID            int64
	ChatSessionID string
	Subject       string
	Predicate     string
	Object        string
	ValidFrom     int
	ValidTo       int
	SourceTurn    int
	CreatedAt     time.Time
}

// AuditLog is an audit trail entry.
type AuditLog struct {
	ID            int64     `json:"id"`
	CreatedAt     time.Time `json:"created_at"`
	EventType     string    `json:"event_type"`
	ChatSessionID string    `json:"chat_session_id"`
	TargetType    string    `json:"target_type"`
	TargetID      int64     `json:"target_id"`
	Summary       string    `json:"summary"`
	DetailsJSON   string    `json:"details_json"`
	Source        string    `json:"source"`
}

// CriticFeedback is a feedback record.
type CriticFeedback struct {
	ID            int64
	CreatedAt     time.Time
	ChatSessionID string
	TargetType    string
	TargetID      int64
	FeedbackValue string
	FeedbackNote  string
	Source        string
}

// CharacterEvent is a character change event.
type CharacterEvent struct {
	ID            int64
	ChatSessionID string
	CharacterName string
	TurnIndex     int
	EventType     string
	DetailsJSON   string
	CreatedAt     time.Time
}

// ConsequenceRecord is a support-only decision->result->delayed-effect chain.
// It is not a canonical truth writer; it carries evidence metadata and a lifecycle
// (pending/active/paid/expired) for later foreground-eligibility passes.
type ConsequenceRecord struct {
	ID                     int64     `json:"id"`
	ChatSessionID          string    `json:"chat_session_id"`
	SourceTurnStart        int       `json:"source_turn_start"`
	SourceTurnEnd          int       `json:"source_turn_end"`
	Decision               string    `json:"decision"`
	ImmediateResult        string    `json:"immediate_result"`
	DelayedEffect          string    `json:"delayed_effect"`
	AffectedRelationsJSON  string    `json:"affected_relations_json,omitempty"`
	AffectedWorldJSON      string    `json:"affected_world_json,omitempty"`
	Status                 string    `json:"status"`
	Importance             float64   `json:"importance"`
	Confidence             float64   `json:"confidence"`
	ForegroundEligible     bool      `json:"foreground_eligible"`
	QuietTurns             int       `json:"quiet_turns"`
	LastSeenTurn           int       `json:"last_seen_turn"`
	PaidTurn               int       `json:"paid_turn"`
	ExpiresAfterQuietTurns int       `json:"expires_after_quiet_turns"`
	SourceHash             string    `json:"source_hash,omitempty"`
	EvidenceJSON           string    `json:"evidence_json,omitempty"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

// ConsequenceRecordStore is an optional Step 23 extension for persisting
// consequence ledger records. Stores that do not implement it degrade safely.
type ConsequenceRecordStore interface {
	ListConsequenceRecords(ctx context.Context, chatSessionID string, limit int) ([]ConsequenceRecord, error)
	SaveConsequenceRecord(ctx context.Context, record ConsequenceRecord) (ConsequenceRecord, error)
	UpdateConsequenceRecordStatus(ctx context.Context, id int64, status string, paidTurn int) error
}

// PsychologyBranch is a support-only long-horizon motivation axis.
// It is guidance/context material, not canonical truth about action, feeling,
// consent, private intent, or final choice.
type PsychologyBranch struct {
	ID                     int64     `json:"id"`
	ChatSessionID          string    `json:"chat_session_id"`
	CharacterName          string    `json:"character_name"`
	BranchType             string    `json:"branch_type"`
	AxisName               string    `json:"axis_name"`
	Summary                string    `json:"summary"`
	Status                 string    `json:"status"`
	Confidence             float64   `json:"confidence"`
	ConfidenceLabel        string    `json:"confidence_label,omitempty"`
	SourceKind             string    `json:"source_kind,omitempty"`
	SourceTurnStart        int       `json:"source_turn_start"`
	SourceTurnEnd          int       `json:"source_turn_end"`
	SourceHash             string    `json:"source_hash,omitempty"`
	EvidenceJSON           string    `json:"evidence_json,omitempty"`
	QuietTurns             int       `json:"quiet_turns"`
	LastSeenTurn           int       `json:"last_seen_turn"`
	DormantAfterQuietTurns int       `json:"dormant_after_quiet_turns"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

// PsychologyBranchStore is an optional Step 23 extension for support-only
// psychology branch storage. Stores that do not implement it degrade safely.
type PsychologyBranchStore interface {
	ListPsychologyBranches(ctx context.Context, chatSessionID string, limit int) ([]PsychologyBranch, error)
	SavePsychologyBranch(ctx context.Context, branch PsychologyBranch) (PsychologyBranch, error)
	UpdatePsychologyBranchStatus(ctx context.Context, id int64, status string, quietTurns int) error
}

// ForkLineageRecord is a support-only provenance entry for copied/forked sessions.
// It records where a session came from, what was inherited, and how it may diverge,
// without becoming a canonical truth writer in either session.
type ForkLineageRecord struct {
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

// ForkLineageStore is an optional Step 23 extension for persisting copied/forked
// session lineage. Stores that do not implement it degrade safely.
type ForkLineageStore interface {
	ListForkLineageRecords(ctx context.Context, chatSessionID, scopeID string, limit int) ([]ForkLineageRecord, error)
	SaveForkLineageRecord(ctx context.Context, record ForkLineageRecord) (ForkLineageRecord, error)
}

// PersonaMemoryCapsule groups portable protagonist/player recollections.
// ThemeOffscreenCarryRecord is a support-only recurring theme/motif trace or
// offscreen world progression/carryover surface. It is not a canonical world
// fact writer and may become a foreground candidate only through bounded
// eligibility rules.
type ThemeOffscreenCarryRecord struct {
	ID                     int64     `json:"id"`
	ChatSessionID          string    `json:"chat_session_id"`
	SurfaceType            string    `json:"surface_type"`
	Label                  string    `json:"label"`
	Summary                string    `json:"summary"`
	Status                 string    `json:"status"`
	Confidence             float64   `json:"confidence"`
	ConfidenceLabel        string    `json:"confidence_label,omitempty"`
	SourceKind             string    `json:"source_kind,omitempty"`
	SourceTurnStart        int       `json:"source_turn_start"`
	SourceTurnEnd          int       `json:"source_turn_end"`
	SourceHash             string    `json:"source_hash,omitempty"`
	EvidenceJSON           string    `json:"evidence_json,omitempty"`
	QuietTurns             int       `json:"quiet_turns"`
	LastSeenTurn           int       `json:"last_seen_turn"`
	DormantAfterQuietTurns int       `json:"dormant_after_quiet_turns"`
	ForegroundEligible     bool      `json:"foreground_eligible"`
	ForegroundReasonJSON   string    `json:"foreground_reason_json,omitempty"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

// ThemeOffscreenCarryStore is an optional Step 23-4 extension for support-only
// theme traces and offscreen world carryover. Stores that do not implement it
// degrade safely.

// CaptureVerificationRecord is a support-only per-turn capture integrity gate.
// It stores compact metadata and hashes first; raw payloads are not retained by default.
type CaptureVerificationRecord struct {
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

// CaptureVerificationStore is an optional Step 23-5 extension for persisting
// per-turn capture verification and repair lineage. Stores that do not implement
// it return support-only empty responses from the HTTP layer.
type CaptureVerificationStore interface {
	ListCaptureVerifications(ctx context.Context, chatSessionID string, limit int) ([]CaptureVerificationRecord, error)
	SaveCaptureVerification(ctx context.Context, record CaptureVerificationRecord) (CaptureVerificationRecord, error)
	UpdateCaptureVerificationRepair(ctx context.Context, id int64, state, degradedReason, repairEvidenceJSON string, repairedByID int64, userInputPreserved bool) error
}

// StatusSchemaProposal is a reviewable schema input-channel record.
// It stores proposed status/stat schema material only; it does not register
// canonical schema keys, current values, effects, or projections by itself.
type StatusSchemaProposal struct {
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

// StatusSchemaDefinition is a session-scoped registry entry for one status key.
// It defines structure only; current values and effects are separate lanes.
type StatusSchemaDefinition struct {
	ID               int64     `json:"id"`
	ChatSessionID    string    `json:"chat_session_id"`
	SourceProposalID int64     `json:"source_proposal_id,omitempty"`
	SchemaName       string    `json:"schema_name"`
	RulesetLabel     string    `json:"ruleset_label,omitempty"`
	StatusKey        string    `json:"status_key"`
	Label            string    `json:"label"`
	OwnerScope       string    `json:"owner_scope"`
	ValueKind        string    `json:"value_kind"`
	BoundsJSON       string    `json:"bounds_json,omitempty"`
	OptionsJSON      string    `json:"options_json,omitempty"`
	DefaultValueJSON string    `json:"default_value_json,omitempty"`
	RegistryState    string    `json:"registry_state"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// StatusCurrentValue is the canonical current value for one registered status
// key and one owner. It is evidence-bound and deliberately separate from
// history/event/effect lifecycle rows.
type StatusCurrentValue struct {
	ID            int64     `json:"id"`
	ChatSessionID string    `json:"chat_session_id"`
	RegistryID    int64     `json:"registry_id"`
	StatusKey     string    `json:"status_key"`
	OwnerScope    string    `json:"owner_scope"`
	OwnerID       string    `json:"owner_id"`
	OwnerLabel    string    `json:"owner_label,omitempty"`
	ValueKind     string    `json:"value_kind"`
	ValueJSON     string    `json:"value_json"`
	EvidenceJSON  string    `json:"evidence_json"`
	SourceTurn    int       `json:"source_turn,omitempty"`
	WriteState    string    `json:"write_state"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// StatusChangeEvent is the append-only ledger for status value changes.
// It records evidence and story-clock context without mutating current values.
type StatusChangeEvent struct {
	ID                int64     `json:"id"`
	ChatSessionID     string    `json:"chat_session_id"`
	RegistryID        int64     `json:"registry_id"`
	StatusValueID     int64     `json:"status_value_id,omitempty"`
	StatusKey         string    `json:"status_key"`
	OwnerScope        string    `json:"owner_scope"`
	OwnerID           string    `json:"owner_id"`
	EventKind         string    `json:"event_kind"`
	PreviousValueJSON string    `json:"previous_value_json,omitempty"`
	NewValueJSON      string    `json:"new_value_json,omitempty"`
	EvidenceJSON      string    `json:"evidence_json"`
	SourceTurn        int       `json:"source_turn,omitempty"`
	StoryClockJSON    string    `json:"story_clock_json,omitempty"`
	EventState        string    `json:"event_state"`
	CreatedAt         time.Time `json:"created_at"`
}

// StatusEffect is the lifecycle row for temporary status effects such as buffs,
// debuffs, injuries, and cooldowns. Timing remains anchored to story-clock JSON.
type StatusEffect struct {
	ID                  int64     `json:"id"`
	ChatSessionID       string    `json:"chat_session_id"`
	RegistryID          int64     `json:"registry_id"`
	StatusKey           string    `json:"status_key"`
	OwnerScope          string    `json:"owner_scope"`
	OwnerID             string    `json:"owner_id"`
	EffectKind          string    `json:"effect_kind"`
	EffectLabel         string    `json:"effect_label,omitempty"`
	EffectPayloadJSON   string    `json:"effect_payload_json,omitempty"`
	EvidenceJSON        string    `json:"evidence_json"`
	SourceTurn          int       `json:"source_turn,omitempty"`
	StartClockJSON      string    `json:"start_clock_json"`
	DurationJSON        string    `json:"duration_json,omitempty"`
	ExpiresAtClockJSON  string    `json:"expires_at_clock_json,omitempty"`
	EffectState         string    `json:"effect_state"`
	ClearedEvidenceJSON string    `json:"cleared_evidence_json,omitempty"`
	ClearedTurn         int       `json:"cleared_turn,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// StatusSchemaProposalStore is an optional extension for persisting
// schema proposal/review/import input records. Stores that do not implement it
// degrade through the HTTP layer without becoming canonical status writers.
type StatusSchemaProposalStore interface {
	GetStatusSchemaProposal(ctx context.Context, id int64) (StatusSchemaProposal, error)
	ListStatusSchemaProposals(ctx context.Context, chatSessionID, proposalState string, limit int) ([]StatusSchemaProposal, error)
	SaveStatusSchemaProposal(ctx context.Context, proposal StatusSchemaProposal) (StatusSchemaProposal, error)
	UpdateStatusSchemaProposalReview(ctx context.Context, id int64, proposalState, reviewNote, reviewer string) error
}

// StatusSchemaRegistryStore is an optional extension for canonical schema
// definitions. It never stores current status values or effect lifecycle rows.
type StatusSchemaRegistryStore interface {
	GetStatusSchemaDefinitionByKey(ctx context.Context, chatSessionID, statusKey, ownerScope string) (StatusSchemaDefinition, error)
	ListStatusSchemaDefinitions(ctx context.Context, chatSessionID, registryState string, limit int) ([]StatusSchemaDefinition, error)
	SaveStatusSchemaDefinitions(ctx context.Context, definitions []StatusSchemaDefinition) ([]StatusSchemaDefinition, error)
}

// StatusCurrentValueStore is an optional extension for canonical current status
// values. It writes only the latest value and does not create history or effects.
type StatusCurrentValueStore interface {
	ListStatusCurrentValues(ctx context.Context, chatSessionID, ownerScope, ownerID, statusKey string, limit int) ([]StatusCurrentValue, error)
	SaveStatusCurrentValue(ctx context.Context, value StatusCurrentValue) (StatusCurrentValue, error)
}

// StatusLifecycleStore is an optional extension for append-only status change
// events and effect lifecycle rows. It does not update current status values.
type StatusLifecycleStore interface {
	ListStatusChangeEvents(ctx context.Context, chatSessionID, ownerScope, ownerID, statusKey string, limit int) ([]StatusChangeEvent, error)
	SaveStatusChangeEvent(ctx context.Context, event StatusChangeEvent) (StatusChangeEvent, error)
	ListStatusEffects(ctx context.Context, chatSessionID, ownerScope, ownerID, effectState string, limit int) ([]StatusEffect, error)
	SaveStatusEffect(ctx context.Context, effect StatusEffect) (StatusEffect, error)
	UpdateStatusEffectState(ctx context.Context, id int64, effectState, clearedEvidenceJSON string, clearedTurn int) error
}

type ThemeOffscreenCarryStore interface {
	ListThemeOffscreenCarries(ctx context.Context, chatSessionID, surfaceType string, limit int) ([]ThemeOffscreenCarryRecord, error)
	SaveThemeOffscreenCarry(ctx context.Context, record ThemeOffscreenCarryRecord) (ThemeOffscreenCarryRecord, error)
	UpdateThemeOffscreenCarryStatus(ctx context.Context, id int64, status string, quietTurns int) error
}

type PersonaMemoryCapsule struct {
	ID                  int64     `json:"id"`
	PersonaKey          string    `json:"persona_key"`
	SourceChatSessionID string    `json:"source_chat_session_id"`
	SourceCharacterName string    `json:"source_character_name"`
	Title               string    `json:"title"`
	Mode                string    `json:"mode"`
	Summary             string    `json:"summary"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// PersonaMemoryEntry is one portable recollection inside a capsule.
type PersonaMemoryEntry struct {
	ID               int64     `json:"id"`
	CapsuleID        int64     `json:"capsule_id"`
	SourceMemoryType string    `json:"source_memory_type,omitempty"`
	SourceMemoryID   int64     `json:"source_memory_id,omitempty"`
	SourceTurn       int       `json:"source_turn_index"`
	MemoryText       string    `json:"memory_text"`
	EmotionalWeight  float64   `json:"emotional_weight"`
	Importance10     float64   `json:"importance_10"`
	Portability      string    `json:"portability"`
	TagsJSON         string    `json:"tags_json"`
	EvidenceExcerpt  string    `json:"evidence_excerpt"`
	InjectionPolicy  string    `json:"injection_policy"`
	CreatedAt        time.Time `json:"created_at"`
}

// PersonaCapsuleAttachment enables a capsule for a target session.
type PersonaCapsuleAttachment struct {
	ID                  int64     `json:"id"`
	CapsuleID           int64     `json:"capsule_id"`
	TargetChatSessionID string    `json:"target_chat_session_id"`
	InjectionMode       string    `json:"injection_mode"`
	Enabled             bool      `json:"enabled"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// Storyline is a narrative thread registry entry.
type Storyline struct {
	ID                  int64     `json:"id"`
	ChatSessionID       string    `json:"chat_session_id"`
	Name                string    `json:"name"`
	Status              string    `json:"status"`
	EntitiesJSON        string    `json:"entities_json"`
	CurrentContext      string    `json:"current_context"`
	KeyPointsJSON       string    `json:"key_points_json"`
	OngoingTensionsJSON string    `json:"ongoing_tensions_json"`
	Confidence          float64   `json:"confidence"`
	EvidenceCount       int       `json:"evidence_count"`
	LastEvidenceTurn    int       `json:"last_evidence_turn"`
	FirstTurn           int       `json:"first_turn"`
	LastTurn            int       `json:"last_turn"`
	Pinned              bool      `json:"pinned"`
	Suppressed          bool      `json:"suppressed"`
	UserCorrected       bool      `json:"user_corrected"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// WorldRule is a world constraint or rule.
type WorldRule struct {
	ID            int64     `json:"id"`
	ChatSessionID string    `json:"chat_session_id"`
	Scope         string    `json:"scope"`
	ScopeName     string    `json:"scope_name"`
	Category      string    `json:"category"`
	Key           string    `json:"key"`
	ValueJSON     string    `json:"value_json"`
	Genre         string    `json:"genre"`
	SourceTurn    int       `json:"source_turn"`
	Pinned        bool      `json:"pinned"`
	Suppressed    bool      `json:"suppressed"`
	UserCorrected bool      `json:"user_corrected"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Entity is an extracted named entity.
type Entity struct {
	ID            int64     `json:"id"`
	ChatSessionID string    `json:"chat_session_id"`
	Name          string    `json:"name"`
	EntityType    string    `json:"entity_type"`
	Description   string    `json:"description"`
	AliasesJSON   string    `json:"aliases_json"`
	FirstSeenTurn int       `json:"first_seen_turn"`
	LastSeenTurn  int       `json:"last_seen_turn"`
	Confidence    float64   `json:"confidence"`
	Pinned        bool      `json:"pinned"`
	Suppressed    bool      `json:"suppressed"`
	UserCorrected bool      `json:"user_corrected"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// Trust is a trust snapshot for a relationship or entity.
type Trust struct {
	ID            int64     `json:"id"`
	ChatSessionID string    `json:"chat_session_id"`
	TargetName    string    `json:"target_name"`
	TargetType    string    `json:"target_type"`
	Score         float64   `json:"score"`
	ReasonJSON    string    `json:"reason_json"`
	SourceTurn    int       `json:"source_turn"`
	Pinned        bool      `json:"pinned"`
	Suppressed    bool      `json:"suppressed"`
	UserCorrected bool      `json:"user_corrected"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CharacterState is a character snapshot.
type CharacterState struct {
	ID                int64     `json:"id"`
	ChatSessionID     string    `json:"chat_session_id"`
	CharacterName     string    `json:"character_name"`
	AppearanceJSON    string    `json:"appearance_json"`
	PersonalityJSON   string    `json:"personality_json"`
	StatusJSON        string    `json:"status_json"`
	RelationshipsJSON string    `json:"relationships_json"`
	SpeechStyleJSON   string    `json:"speech_style_json"`
	TurnIndex         int       `json:"turn_index"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// PendingThread is a continuity hook.
type PendingThread struct {
	ID               int64     `json:"id"`
	ChatSessionID    string    `json:"chat_session_id"`
	ThreadKey        string    `json:"thread_key"`
	Description      string    `json:"description"`
	Status           string    `json:"status"`
	CreatedTurn      int       `json:"created_turn"`
	ResolvedTurn     int       `json:"resolved_turn"`
	SourceTurn       int       `json:"source_turn"`
	Priority         int       `json:"priority"`
	HookType         string    `json:"hook_type"`
	HookMetadataJSON string    `json:"hook_metadata_json"`
	ThreadType       string    `json:"thread_type,omitempty"`
	Title            string    `json:"title,omitempty"`
	Owner            string    `json:"owner,omitempty"`
	Target           string    `json:"target,omitempty"`
	LastSeenTurn     int       `json:"last_seen_turn,omitempty"`
	Confidence       float64   `json:"confidence,omitempty"`
	DetailsJSON      string    `json:"details_json,omitempty"`
	ResolutionNote   string    `json:"resolution_note,omitempty"`
	Pinned           bool      `json:"pinned"`
	Suppressed       bool      `json:"suppressed"`
	UserCorrected    bool      `json:"user_corrected"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// ActiveState is a mutable state snapshot.
type ActiveState struct {
	ID            int64     `json:"id"`
	ChatSessionID string    `json:"chat_session_id"`
	StateType     string    `json:"state_type"`
	Content       string    `json:"content"`
	TurnIndex     int       `json:"turn_index"`
	CreatedAt     time.Time `json:"created_at"`
}

// CanonicalStateLayer is a verified state layer.
type CanonicalStateLayer struct {
	ID               int64     `json:"id"`
	ChatSessionID    string    `json:"chat_session_id"`
	LayerType        string    `json:"layer_type"`
	Content          string    `json:"content"`
	SourceStateType  string    `json:"source_state_type"`
	TurnIndex        int       `json:"turn_index"`
	SourceTurn       int       `json:"source_turn"`
	SourceRecord     int64     `json:"source_record"`
	LastVerifiedTurn int       `json:"last_verified_turn"`
	Confidence       float64   `json:"confidence"`
	CreatedAt        time.Time `json:"created_at"`
}

// EpisodeSummary is a narrative episode summary.
type EpisodeSummary struct {
	ID                      int64     `json:"id"`
	ChatSessionID           string    `json:"chat_session_id"`
	FromTurn                int       `json:"from_turn"`
	ToTurn                  int       `json:"to_turn"`
	SummaryText             string    `json:"summary_text"`
	KeyEntities             string    `json:"key_entities"`
	KeyEvents               string    `json:"key_events"`
	OpenLoopsJSON           string    `json:"open_loops_json"`
	RelationshipChangesJSON string    `json:"relationship_changes_json"`
	EmbeddingVector         string    `json:"embedding_vector"`
	EmbeddingModel          string    `json:"embedding_model"`
	CreatedAt               time.Time `json:"created_at"`
}

// StatsResult holds aggregate counts for the /stats endpoint.
type StatsResult struct {
	ChatLogs  int64 `json:"chat_logs"`
	Memories  int64 `json:"memories"`
	KgTriples int64 `json:"kg_triples"`
}

// SessionSummary is a lightweight session listing item.
type SessionSummary struct {
	ChatSessionID  string    `json:"chat_session_id"`
	ChatLogsCount  int       `json:"chat_logs_count,omitempty"`
	MemoriesCount  int       `json:"memories_count,omitempty"`
	KGTriplesCount int       `json:"kg_triples_count,omitempty"`
	LastActivity   time.Time `json:"last_activity,omitempty"`
}

// SagaDigest is the latest saga digest row for a session.
type SagaDigest struct {
	ID                      int64      `json:"id"`
	ChatSessionID           string     `json:"chat_session_id"`
	FromTurn                int        `json:"from_turn"`
	ToTurn                  int        `json:"to_turn"`
	EraLabel                string     `json:"era_label"`
	SagaSummary             string     `json:"saga_summary"`
	PersistentFactsJSON     string     `json:"persistent_facts_json"`
	NeverDropCandidatesJSON string     `json:"never_drop_candidates_json"`
	ResumePackText          string     `json:"resume_pack_text"`
	EmbeddingVector         string     `json:"embedding_vector"`
	EmbeddingModel          string     `json:"embedding_model"`
	CreatedAt               *time.Time `json:"created_at"`
}

// GuidancePlanState is the cached K-2 narrative plan snapshot for a session.
type GuidancePlanState struct {
	ID            int64     `json:"id"`
	ChatSessionID string    `json:"chat_session_id"`
	StoryPlanJSON string    `json:"story_plan_json"`
	DirectorJSON  string    `json:"director_json"`
	StateStatus   string    `json:"state_status"`
	LastTurn      int       `json:"last_turn"`
	WarningsJSON  string    `json:"warnings_json"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// GuidancePlanStateStore is an optional interface for stores that can cache
// K-2 narrative plan snapshots per session.
type GuidancePlanStateStore interface {
	GetGuidancePlanState(ctx context.Context, chatSessionID string) (*GuidancePlanState, error)
	UpsertGuidancePlanState(ctx context.Context, item *GuidancePlanState) error
}

// ChapterSummaryStore is an optional Step 9 store extension for chapter
// generation/search paths. It stays outside the base Store contract so older
// read-only/noop stores can degrade safely.
type ChapterSummaryStore interface {
	SaveChapterSummary(ctx context.Context, item *ChapterSummary) error
	SearchChapterSummaries(ctx context.Context, chatSessionID, query string, fromTurn, toTurn, limit int) ([]ChapterSummary, error)
}

// EpisodeSummaryStore is an optional Step 11 Dense Summary extension for
// episode generation paths. Read-only stores can omit it and keep shadow guards.
type EpisodeSummaryStore interface {
	SaveEpisodeSummary(ctx context.Context, item *EpisodeSummary) error
}

// ArcSummaryStore is an optional Step 9 store extension for arc
// generation/search paths.
type ArcSummaryStore interface {
	SaveArcSummary(ctx context.Context, chatSessionID string, item *ArcSummary) error
	GetLatestArcSummary(ctx context.Context, chatSessionID string) (*ArcSummary, error)
	ListArcSummaries(ctx context.Context, chatSessionID string, status string, limit int) ([]ArcSummary, error)
	SearchArcSummaries(ctx context.Context, chatSessionID, query string, fromTurn, toTurn, limit int) ([]ArcSummary, error)
}

// SagaDigestStore is an optional Step 9 store extension for saga
// generation/list paths.
type SagaDigestStore interface {
	SaveSagaDigest(ctx context.Context, chatSessionID string, item *SagaDigest) error
	GetLatestSagaDigest(ctx context.Context, chatSessionID string) (*SagaDigest, error)
	ListSagaDigests(ctx context.Context, chatSessionID string, limit int) ([]SagaDigest, error)
	SearchSagaDigests(ctx context.Context, chatSessionID, query string, fromTurn, toTurn, limit int) ([]SagaDigest, error)
}

// ArcSummary is the latest arc summary row for a session.
type ArcSummary struct {
	ID                         int64      `json:"id"`
	ChatSessionID              string     `json:"chat_session_id"`
	FromTurn                   int        `json:"from_turn"`
	ToTurn                     int        `json:"to_turn"`
	ArcIndex                   int        `json:"arc_index"`
	ArcName                    string     `json:"arc_name"`
	ArcStatus                  string     `json:"arc_status"`
	CoreConflict               string     `json:"core_conflict"`
	KeyTurningPointsJSON       string     `json:"key_turning_points_json"`
	ActivePromisesJSON         string     `json:"active_promises_json"`
	UnresolvedDebtsJSON        string     `json:"unresolved_debts_json"`
	ResolvedPayoffsJSON        string     `json:"resolved_payoffs_json"`
	CallbackCandidatesJSON     string     `json:"callback_candidates_json"`
	FuturePayoffCandidatesJSON string     `json:"future_payoff_candidates_json"`
	IrreversibleTurnsJSON      string     `json:"irreversible_turns_json"`
	CallbackDebtsJSON          string     `json:"callback_debts_json"`
	RelationshipPivotsJSON     string     `json:"relationship_pivots_json"`
	ArcResumeText              string     `json:"arc_resume_text"`
	EmbeddingVector            string     `json:"embedding_vector"`
	EmbeddingModel             string     `json:"embedding_model"`
	CreatedAt                  *time.Time `json:"created_at"`
}

// ChapterSummary is the latest chapter summary row for a session.
type ChapterSummary struct {
	ID                      int64      `json:"id"`
	ChatSessionID           string     `json:"chat_session_id"`
	FromTurn                int        `json:"from_turn"`
	ToTurn                  int        `json:"to_turn"`
	ChapterIndex            int        `json:"chapter_index"`
	ChapterTitle            string     `json:"chapter_title"`
	SummaryText             string     `json:"summary_text"`
	OpenLoopsJSON           string     `json:"open_loops_json"`
	RelationshipChangesJSON string     `json:"relationship_changes_json"`
	WorldChangesJSON        string     `json:"world_changes_json"`
	CallbackCandidatesJSON  string     `json:"callback_candidates_json"`
	ResumeText              string     `json:"resume_text"`
	EmbeddingVector         string     `json:"embedding_vector"`
	EmbeddingModel          string     `json:"embedding_model"`
	CreatedAt               *time.Time `json:"created_at"`
}

// ResumePack is the assembled resume-pack payload returned by the store.
type ResumePack struct {
	PackStatus    string          `json:"pack_status"`
	Trigger       string          `json:"trigger"`
	SourcesUsed   []string        `json:"sources_used"`
	LayerCount    int             `json:"layer_count"`
	AssembledText string          `json:"assembled_text"`
	Saga          *SagaDigest     `json:"saga"`
	Arc           *ArcSummary     `json:"arc"`
	Chapter       *ChapterSummary `json:"chapter"`
	AssemblyNote  string          `json:"assembly_note"`
}
