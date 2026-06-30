package store

import "context"

// readOnlyStore wraps a Store and delegates all read operations while
// returning ErrNotEnabled for every write operation. It is used by the
// mariadb_read_shadow mode to allow HTTP read endpoints to query MariaDB
// without risking accidental writes.
type readOnlyStore struct {
	delegate Store
}

// NewReadOnlyStore returns a Store that delegates reads to the underlying
// store and blocks all writes with ErrNotEnabled.
func NewReadOnlyStore(delegate Store) Store {
	return &readOnlyStore{delegate: delegate}
}

// SaveChatLog blocks writes.
func (r *readOnlyStore) SaveChatLog(ctx context.Context, log *ChatLog) error {
	return ErrNotEnabled
}

// ListChatLogs delegates to the underlying store.
func (r *readOnlyStore) ListChatLogs(ctx context.Context, chatSessionID string, fromTurn, toTurn int) ([]ChatLog, error) {
	return r.delegate.ListChatLogs(ctx, chatSessionID, fromTurn, toTurn)
}

// ReadSessionStateSnapshot delegates to the underlying read store when
// available. This keeps read-only MariaDB shadow mode eligible for I-1
// aggregate session-state reads.
func (r *readOnlyStore) ReadSessionStateSnapshot(ctx context.Context, chatSessionID string) (*SessionStateSnapshot, error) {
	reader, ok := r.delegate.(SessionStateSnapshotReader)
	if !ok {
		return nil, ErrNotEnabled
	}
	return reader.ReadSessionStateSnapshot(ctx, chatSessionID)
}

// SaveEffectiveInput blocks writes.
func (r *readOnlyStore) SaveEffectiveInput(ctx context.Context, in *EffectiveInput) error {
	return ErrNotEnabled
}

// GetEffectiveInput delegates to the underlying store.
func (r *readOnlyStore) GetEffectiveInput(ctx context.Context, chatSessionID string, turnIndex int) (*EffectiveInput, error) {
	return r.delegate.GetEffectiveInput(ctx, chatSessionID, turnIndex)
}

// SaveMemory blocks writes.
func (r *readOnlyStore) SaveMemory(ctx context.Context, m *Memory) error {
	return ErrNotEnabled
}

// ListMemories delegates to the underlying store.
func (r *readOnlyStore) ListMemories(ctx context.Context, chatSessionID string, fromTurn, toTurn int) ([]Memory, error) {
	return r.delegate.ListMemories(ctx, chatSessionID, fromTurn, toTurn)
}

// SaveEvidence blocks writes.
func (r *readOnlyStore) SaveEvidence(ctx context.Context, e *DirectEvidence) error {
	return ErrNotEnabled
}

// ListEvidence delegates to the underlying store.
func (r *readOnlyStore) ListEvidence(ctx context.Context, chatSessionID string) ([]DirectEvidence, error) {
	return r.delegate.ListEvidence(ctx, chatSessionID)
}

// SaveKGTriple blocks writes.
func (r *readOnlyStore) SaveKGTriple(ctx context.Context, t *KGTriple) error {
	return ErrNotEnabled
}

// ListKGTriples delegates to the underlying store.
func (r *readOnlyStore) ListKGTriples(ctx context.Context, chatSessionID string) ([]KGTriple, error) {
	return r.delegate.ListKGTriples(ctx, chatSessionID)
}

// SaveAuditLog blocks writes.
func (r *readOnlyStore) SaveAuditLog(ctx context.Context, a *AuditLog) error {
	return ErrNotEnabled
}

// ListAuditLogs delegates to the underlying store.
func (r *readOnlyStore) ListAuditLogs(ctx context.Context, chatSessionID string, eventType string, limit int) ([]AuditLog, error) {
	return r.delegate.ListAuditLogs(ctx, chatSessionID, eventType, limit)
}

func (r *readOnlyStore) CountAuditLogs(ctx context.Context, chatSessionID string, eventType string) (int, error) {
	if counter, ok := r.delegate.(AuditLogCounter); ok {
		return counter.CountAuditLogs(ctx, chatSessionID, eventType)
	}
	items, err := r.delegate.ListAuditLogs(ctx, chatSessionID, eventType, 0)
	if err != nil {
		return 0, err
	}
	return len(items), nil
}

// SaveCriticFeedback blocks writes.
func (r *readOnlyStore) SaveCriticFeedback(ctx context.Context, f *CriticFeedback) error {
	return ErrNotEnabled
}

// ListCriticFeedback delegates to the underlying store.
func (r *readOnlyStore) ListCriticFeedback(ctx context.Context, chatSessionID string, targetType string, targetID int64) ([]CriticFeedback, error) {
	return r.delegate.ListCriticFeedback(ctx, chatSessionID, targetType, targetID)
}

// SaveCharacterEvent blocks writes.
func (r *readOnlyStore) SaveCharacterEvent(ctx context.Context, e *CharacterEvent) error {
	return ErrNotEnabled
}

func (r *readOnlyStore) SaveEntity(ctx context.Context, e *Entity) error { return ErrNotEnabled }
func (r *readOnlyStore) SaveTrust(ctx context.Context, t *Trust) error   { return ErrNotEnabled }

// ListCharacterEvents delegates to the underlying store.
func (r *readOnlyStore) ListCharacterEvents(ctx context.Context, chatSessionID string, characterName string) ([]CharacterEvent, error) {
	return r.delegate.ListCharacterEvents(ctx, chatSessionID, characterName)
}

// Stats delegates to the underlying store.
func (r *readOnlyStore) Stats(ctx context.Context) (StatsResult, error) {
	return r.delegate.Stats(ctx)
}

// ListSessions delegates to the underlying store.
func (r *readOnlyStore) ListSessions(ctx context.Context) ([]SessionSummary, error) {
	return r.delegate.ListSessions(ctx)
}

// GetResumePack delegates to the underlying store.
func (r *readOnlyStore) GetResumePack(ctx context.Context, chatSessionID string, trigger string) (*ResumePack, error) {
	return r.delegate.GetResumePack(ctx, chatSessionID, trigger)
}

// ListStorylines delegates to the underlying store.
func (r *readOnlyStore) ListStorylines(ctx context.Context, chatSessionID string) ([]Storyline, error) {
	return r.delegate.ListStorylines(ctx, chatSessionID)
}

func (r *readOnlyStore) SaveStoryline(ctx context.Context, s *Storyline) error { return ErrNotEnabled }

// ListWorldRules delegates to the underlying store.
func (r *readOnlyStore) ListWorldRules(ctx context.Context, chatSessionID string) ([]WorldRule, error) {
	return r.delegate.ListWorldRules(ctx, chatSessionID)
}

func (r *readOnlyStore) SaveWorldRule(ctx context.Context, w *WorldRule) error { return ErrNotEnabled }

func (r *readOnlyStore) SaveCharacterState(ctx context.Context, c *CharacterState) error {
	return ErrNotEnabled
}
func (r *readOnlyStore) SavePendingThread(ctx context.Context, p *PendingThread) error {
	return ErrNotEnabled
}
func (r *readOnlyStore) SaveActiveState(ctx context.Context, a *ActiveState) error {
	return ErrNotEnabled
}
func (r *readOnlyStore) SaveCanonicalStateLayer(ctx context.Context, item *CanonicalStateLayer) error {
	return ErrNotEnabled
}

// ListInheritedWorldRules delegates to the underlying store.
func (r *readOnlyStore) ListInheritedWorldRules(ctx context.Context, chatSessionID string, activeScope, scopeName string) ([]WorldRule, error) {
	return r.delegate.ListInheritedWorldRules(ctx, chatSessionID, activeScope, scopeName)
}

// ListCharacterStates delegates to the underlying store.
func (r *readOnlyStore) ListCharacterStates(ctx context.Context, chatSessionID string) ([]CharacterState, error) {
	return r.delegate.ListCharacterStates(ctx, chatSessionID)
}

// GetCharacterState delegates to the underlying store.
func (r *readOnlyStore) GetCharacterState(ctx context.Context, chatSessionID, characterName string) (*CharacterState, error) {
	return r.delegate.GetCharacterState(ctx, chatSessionID, characterName)
}

// ListPendingThreads delegates to the underlying store.
func (r *readOnlyStore) ListPendingThreads(ctx context.Context, chatSessionID, status string) ([]PendingThread, error) {
	return r.delegate.ListPendingThreads(ctx, chatSessionID, status)
}

// ListActiveStates delegates to the underlying store.
func (r *readOnlyStore) ListActiveStates(ctx context.Context, chatSessionID, stateType string) ([]ActiveState, error) {
	return r.delegate.ListActiveStates(ctx, chatSessionID, stateType)
}

// ListCanonicalStateLayers delegates to the underlying store.
func (r *readOnlyStore) ListCanonicalStateLayers(ctx context.Context, chatSessionID, layerType string) ([]CanonicalStateLayer, error) {
	return r.delegate.ListCanonicalStateLayers(ctx, chatSessionID, layerType)
}

// ListEpisodeSummaries delegates to the underlying store.
func (r *readOnlyStore) ListEpisodeSummaries(ctx context.Context, chatSessionID string, limit, fromTurn, toTurn int) ([]EpisodeSummary, error) {
	return r.delegate.ListEpisodeSummaries(ctx, chatSessionID, limit, fromTurn, toTurn)
}

// GetEpisodeSummary delegates to the underlying store.
func (r *readOnlyStore) GetEpisodeSummary(ctx context.Context, episodeID int64) (*EpisodeSummary, error) {
	return r.delegate.GetEpisodeSummary(ctx, episodeID)
}

// RollbackStore stubs for readOnlyStore (read-only, returns ErrNotEnabled).
func (r *readOnlyStore) DeleteChatLogs(ctx context.Context, chatSessionID string, fromTurn int) error {
	return ErrNotEnabled
}
func (r *readOnlyStore) DeleteEffectiveInputs(ctx context.Context, chatSessionID string, fromTurn int) error {
	return ErrNotEnabled
}
func (r *readOnlyStore) DeleteMemories(ctx context.Context, chatSessionID string, fromTurn int) error {
	return ErrNotEnabled
}
func (r *readOnlyStore) DeleteEvidence(ctx context.Context, chatSessionID string, fromTurn int) error {
	return ErrNotEnabled
}
func (r *readOnlyStore) DeleteKGTriples(ctx context.Context, chatSessionID string, fromTurn int) error {
	return ErrNotEnabled
}
func (r *readOnlyStore) DeleteCriticFeedback(ctx context.Context, chatSessionID string, fromTurn int) error {
	return ErrNotEnabled
}
func (r *readOnlyStore) DeleteCharacterEvents(ctx context.Context, chatSessionID string, fromTurn int) error {
	return ErrNotEnabled
}
func (r *readOnlyStore) DeleteEntities(ctx context.Context, chatSessionID string, fromTurn int) error {
	return ErrNotEnabled
}
func (r *readOnlyStore) DeleteTrustStates(ctx context.Context, chatSessionID string, fromTurn int) error {
	return ErrNotEnabled
}
func (r *readOnlyStore) DeleteStorylines(ctx context.Context, chatSessionID string, fromTurn int) error {
	return ErrNotEnabled
}
func (r *readOnlyStore) DeleteWorldRules(ctx context.Context, chatSessionID string, fromTurn int) error {
	return ErrNotEnabled
}
func (r *readOnlyStore) DeleteCharacterStates(ctx context.Context, chatSessionID string, fromTurn int) error {
	return ErrNotEnabled
}
func (r *readOnlyStore) DeletePendingThreads(ctx context.Context, chatSessionID string, fromTurn int) error {
	return ErrNotEnabled
}
func (r *readOnlyStore) DeleteActiveStates(ctx context.Context, chatSessionID string, fromTurn int) error {
	return ErrNotEnabled
}
func (r *readOnlyStore) DeleteCanonicalStateLayers(ctx context.Context, chatSessionID string, fromTurn int) error {
	return ErrNotEnabled
}
func (r *readOnlyStore) DeleteEpisodeSummaries(ctx context.Context, chatSessionID string, fromTurn int) error {
	return ErrNotEnabled
}
func (r *readOnlyStore) DeleteGuidancePlanState(ctx context.Context, chatSessionID string, fromTurn int) error {
	return ErrNotEnabled
}
func (r *readOnlyStore) DeleteChapterSummaries(ctx context.Context, chatSessionID string, fromTurn int) error {
	return ErrNotEnabled
}
func (r *readOnlyStore) DeleteArcSummaries(ctx context.Context, chatSessionID string, fromTurn int) error {
	return ErrNotEnabled
}
func (r *readOnlyStore) DeleteSagaDigests(ctx context.Context, chatSessionID string, fromTurn int) error {
	return ErrNotEnabled
}
func (r *readOnlyStore) DeleteSessionActiveScopes(ctx context.Context, chatSessionID string, fromTurn int) error {
	return ErrNotEnabled
}
func (r *readOnlyStore) DeleteProtagonistEntityMemories(ctx context.Context, chatSessionID string, fromTurn int) error {
	return ErrNotEnabled
}
func (r *readOnlyStore) DeleteConsequenceRecords(ctx context.Context, chatSessionID string, fromTurn int) error {
	return ErrNotEnabled
}
func (r *readOnlyStore) DeletePsychologyBranches(ctx context.Context, chatSessionID string, fromTurn int) error {
	return ErrNotEnabled
}
func (r *readOnlyStore) DeleteThemeOffscreenCarries(ctx context.Context, chatSessionID string, fromTurn int) error {
	return ErrNotEnabled
}
func (r *readOnlyStore) DeleteCaptureVerificationRecords(ctx context.Context, chatSessionID string, fromTurn int) error {
	return ErrNotEnabled
}
func (r *readOnlyStore) DeleteSession(ctx context.Context, chatSessionID string) error {
	return ErrNotEnabled
}

// GetGuidancePlanState delegates to the underlying store if it implements GuidancePlanStateStore.
func (r *readOnlyStore) GetGuidancePlanState(ctx context.Context, chatSessionID string) (*GuidancePlanState, error) {
	store, ok := r.delegate.(GuidancePlanStateStore)
	if !ok {
		return nil, ErrNotEnabled
	}
	return store.GetGuidancePlanState(ctx, chatSessionID)
}

// UpsertGuidancePlanState blocks writes.
func (r *readOnlyStore) UpsertGuidancePlanState(ctx context.Context, item *GuidancePlanState) error {
	return ErrNotEnabled
}

func (r *readOnlyStore) ListConsequenceRecords(ctx context.Context, chatSessionID string, limit int) ([]ConsequenceRecord, error) {
	store, ok := r.delegate.(ConsequenceRecordStore)
	if !ok {
		return nil, ErrNotEnabled
	}
	return store.ListConsequenceRecords(ctx, chatSessionID, limit)
}

func (r *readOnlyStore) SaveConsequenceRecord(ctx context.Context, record ConsequenceRecord) (ConsequenceRecord, error) {
	return record, ErrNotEnabled
}

func (r *readOnlyStore) UpdateConsequenceRecordStatus(ctx context.Context, id int64, status string, paidTurn int) error {
	return ErrNotEnabled
}

func (r *readOnlyStore) ListPsychologyBranches(ctx context.Context, chatSessionID string, limit int) ([]PsychologyBranch, error) {
	store, ok := r.delegate.(PsychologyBranchStore)
	if !ok {
		return nil, ErrNotEnabled
	}
	return store.ListPsychologyBranches(ctx, chatSessionID, limit)
}

func (r *readOnlyStore) SavePsychologyBranch(ctx context.Context, branch PsychologyBranch) (PsychologyBranch, error) {
	return branch, ErrNotEnabled
}

func (r *readOnlyStore) UpdatePsychologyBranchStatus(ctx context.Context, id int64, status string, quietTurns int) error {
	return ErrNotEnabled
}

func (r *readOnlyStore) ListForkLineageRecords(ctx context.Context, chatSessionID, scopeID string, limit int) ([]ForkLineageRecord, error) {
	store, ok := r.delegate.(ForkLineageStore)
	if !ok {
		return nil, ErrNotEnabled
	}
	return store.ListForkLineageRecords(ctx, chatSessionID, scopeID, limit)
}

func (r *readOnlyStore) SaveForkLineageRecord(ctx context.Context, record ForkLineageRecord) (ForkLineageRecord, error) {
	return record, ErrNotEnabled
}

func (r *readOnlyStore) ListThemeOffscreenCarries(ctx context.Context, chatSessionID, surfaceType string, limit int) ([]ThemeOffscreenCarryRecord, error) {
	store, ok := r.delegate.(ThemeOffscreenCarryStore)
	if !ok {
		return nil, ErrNotEnabled
	}
	return store.ListThemeOffscreenCarries(ctx, chatSessionID, surfaceType, limit)
}

func (r *readOnlyStore) SaveThemeOffscreenCarry(ctx context.Context, record ThemeOffscreenCarryRecord) (ThemeOffscreenCarryRecord, error) {
	return record, ErrNotEnabled
}

func (r *readOnlyStore) UpdateThemeOffscreenCarryStatus(ctx context.Context, id int64, status string, quietTurns int) error {
	return ErrNotEnabled
}
func (r *readOnlyStore) ListCaptureVerifications(ctx context.Context, chatSessionID string, limit int) ([]CaptureVerificationRecord, error) {
	store, ok := r.delegate.(CaptureVerificationStore)
	if !ok {
		return nil, ErrNotEnabled
	}
	return store.ListCaptureVerifications(ctx, chatSessionID, limit)
}

func (r *readOnlyStore) SaveCaptureVerification(ctx context.Context, record CaptureVerificationRecord) (CaptureVerificationRecord, error) {
	return record, ErrNotEnabled
}

func (r *readOnlyStore) UpdateCaptureVerificationRepair(ctx context.Context, id int64, state, degradedReason, repairEvidenceJSON string, repairedByID int64, userInputPreserved bool) error {
	return ErrNotEnabled
}

func (r *readOnlyStore) ListStatusSchemaProposals(ctx context.Context, chatSessionID, proposalState string, limit int) ([]StatusSchemaProposal, error) {
	store, ok := r.delegate.(StatusSchemaProposalStore)
	if !ok {
		return nil, ErrNotEnabled
	}
	return store.ListStatusSchemaProposals(ctx, chatSessionID, proposalState, limit)
}

func (r *readOnlyStore) GetStatusSchemaProposal(ctx context.Context, id int64) (StatusSchemaProposal, error) {
	store, ok := r.delegate.(StatusSchemaProposalStore)
	if !ok {
		return StatusSchemaProposal{}, ErrNotEnabled
	}
	return store.GetStatusSchemaProposal(ctx, id)
}

func (r *readOnlyStore) SaveStatusSchemaProposal(ctx context.Context, proposal StatusSchemaProposal) (StatusSchemaProposal, error) {
	return proposal, ErrNotEnabled
}

func (r *readOnlyStore) UpdateStatusSchemaProposalReview(ctx context.Context, id int64, proposalState, reviewNote, reviewer string) error {
	return ErrNotEnabled
}

func (r *readOnlyStore) ListStatusSchemaDefinitions(ctx context.Context, chatSessionID, registryState string, limit int) ([]StatusSchemaDefinition, error) {
	store, ok := r.delegate.(StatusSchemaRegistryStore)
	if !ok {
		return nil, ErrNotEnabled
	}
	return store.ListStatusSchemaDefinitions(ctx, chatSessionID, registryState, limit)
}

func (r *readOnlyStore) SaveStatusSchemaDefinitions(ctx context.Context, definitions []StatusSchemaDefinition) ([]StatusSchemaDefinition, error) {
	return definitions, ErrNotEnabled
}

func (r *readOnlyStore) GetStatusSchemaDefinitionByKey(ctx context.Context, chatSessionID, statusKey, ownerScope string) (StatusSchemaDefinition, error) {
	store, ok := r.delegate.(StatusSchemaRegistryStore)
	if !ok {
		return StatusSchemaDefinition{}, ErrNotEnabled
	}
	return store.GetStatusSchemaDefinitionByKey(ctx, chatSessionID, statusKey, ownerScope)
}

func (r *readOnlyStore) ListStatusCurrentValues(ctx context.Context, chatSessionID, ownerScope, ownerID, statusKey string, limit int) ([]StatusCurrentValue, error) {
	store, ok := r.delegate.(StatusCurrentValueStore)
	if !ok {
		return nil, ErrNotEnabled
	}
	return store.ListStatusCurrentValues(ctx, chatSessionID, ownerScope, ownerID, statusKey, limit)
}

func (r *readOnlyStore) SaveStatusCurrentValue(ctx context.Context, value StatusCurrentValue) (StatusCurrentValue, error) {
	return value, ErrNotEnabled
}

func (r *readOnlyStore) ListStatusChangeEvents(ctx context.Context, chatSessionID, ownerScope, ownerID, statusKey string, limit int) ([]StatusChangeEvent, error) {
	store, ok := r.delegate.(StatusLifecycleStore)
	if !ok {
		return nil, ErrNotEnabled
	}
	return store.ListStatusChangeEvents(ctx, chatSessionID, ownerScope, ownerID, statusKey, limit)
}

func (r *readOnlyStore) SaveStatusChangeEvent(ctx context.Context, event StatusChangeEvent) (StatusChangeEvent, error) {
	return event, ErrNotEnabled
}

func (r *readOnlyStore) ListStatusEffects(ctx context.Context, chatSessionID, ownerScope, ownerID, effectState string, limit int) ([]StatusEffect, error) {
	store, ok := r.delegate.(StatusLifecycleStore)
	if !ok {
		return nil, ErrNotEnabled
	}
	return store.ListStatusEffects(ctx, chatSessionID, ownerScope, ownerID, effectState, limit)
}

func (r *readOnlyStore) SaveStatusEffect(ctx context.Context, effect StatusEffect) (StatusEffect, error) {
	return effect, ErrNotEnabled
}

func (r *readOnlyStore) UpdateStatusEffectState(ctx context.Context, id int64, effectState, clearedEvidenceJSON string, clearedTurn int) error {
	return ErrNotEnabled
}

// Ping delegates to the underlying store if it implements Pinger.
func (r *readOnlyStore) Ping(ctx context.Context) error {
	if p, ok := r.delegate.(Pinger); ok {
		return p.Ping(ctx)
	}
	return ErrNotEnabled
}

// Close delegates to the underlying store if it implements interface{ Close() error }.
func (r *readOnlyStore) Close() error {
	if c, ok := r.delegate.(interface{ Close() error }); ok {
		return c.Close()
	}
	return nil
}
