package store

import "context"

// noopStore is an R0/R1 no-op implementation of Store.
// It records calls but performs no persistence and returns no errors.
type noopStore struct{}

// NewNoopStore returns a Store implementation that does nothing.
func NewNoopStore() Store { return &noopStore{} }

func (n *noopStore) SaveChatLog(ctx context.Context, log *ChatLog) error { return nil }
func (n *noopStore) ListChatLogs(ctx context.Context, chatSessionID string, fromTurn, toTurn int) ([]ChatLog, error) {
	return nil, nil
}

func (n *noopStore) SaveEffectiveInput(ctx context.Context, in *EffectiveInput) error { return nil }
func (n *noopStore) GetEffectiveInput(ctx context.Context, chatSessionID string, turnIndex int) (*EffectiveInput, error) {
	return nil, ErrNotFound
}

func (n *noopStore) SaveMemory(ctx context.Context, m *Memory) error { return nil }
func (n *noopStore) ListMemories(ctx context.Context, chatSessionID string, fromTurn, toTurn int) ([]Memory, error) {
	return nil, nil
}

func (n *noopStore) SaveEvidence(ctx context.Context, e *DirectEvidence) error { return nil }
func (n *noopStore) ListEvidence(ctx context.Context, chatSessionID string) ([]DirectEvidence, error) {
	return nil, nil
}

func (n *noopStore) SaveKGTriple(ctx context.Context, t *KGTriple) error { return nil }
func (n *noopStore) ListKGTriples(ctx context.Context, chatSessionID string) ([]KGTriple, error) {
	return nil, nil
}

func (n *noopStore) SaveAuditLog(ctx context.Context, a *AuditLog) error { return nil }
func (n *noopStore) ListAuditLogs(ctx context.Context, chatSessionID string, eventType string, limit int) ([]AuditLog, error) {
	return nil, nil
}

func (n *noopStore) SaveCriticFeedback(ctx context.Context, f *CriticFeedback) error { return nil }
func (n *noopStore) ListCriticFeedback(ctx context.Context, chatSessionID string, targetType string, targetID int64) ([]CriticFeedback, error) {
	return nil, nil
}

func (n *noopStore) SaveCharacterEvent(ctx context.Context, e *CharacterEvent) error { return nil }
func (n *noopStore) ListCharacterEvents(ctx context.Context, chatSessionID string, characterName string) ([]CharacterEvent, error) {
	return nil, nil
}

func (n *noopStore) SaveEntity(ctx context.Context, e *Entity) error { return nil }
func (n *noopStore) SaveTrust(ctx context.Context, t *Trust) error   { return nil }

func (n *noopStore) Stats(ctx context.Context) (StatsResult, error) {
	return StatsResult{}, nil
}

func (n *noopStore) ListSessions(ctx context.Context) ([]SessionSummary, error) {
	return nil, nil
}

func (n *noopStore) GetResumePack(ctx context.Context, chatSessionID string, trigger string) (*ResumePack, error) {
	return nil, nil
}

func (n *noopStore) ListStorylines(ctx context.Context, chatSessionID string) ([]Storyline, error) {
	return nil, nil
}

func (n *noopStore) SaveStoryline(ctx context.Context, s *Storyline) error { return nil }

func (n *noopStore) ListWorldRules(ctx context.Context, chatSessionID string) ([]WorldRule, error) {
	return nil, nil
}

func (n *noopStore) SaveWorldRule(ctx context.Context, w *WorldRule) error           { return nil }
func (n *noopStore) SaveCharacterState(ctx context.Context, c *CharacterState) error { return nil }
func (n *noopStore) SavePendingThread(ctx context.Context, p *PendingThread) error   { return nil }
func (n *noopStore) SaveActiveState(ctx context.Context, a *ActiveState) error       { return nil }
func (n *noopStore) SaveCanonicalStateLayer(ctx context.Context, item *CanonicalStateLayer) error {
	return nil
}

func (n *noopStore) ListInheritedWorldRules(ctx context.Context, chatSessionID string, activeScope, scopeName string) ([]WorldRule, error) {
	return nil, nil
}

func (n *noopStore) ListCharacterStates(ctx context.Context, chatSessionID string) ([]CharacterState, error) {
	return nil, nil
}

func (n *noopStore) GetCharacterState(ctx context.Context, chatSessionID, characterName string) (*CharacterState, error) {
	return nil, ErrNotFound
}

func (n *noopStore) ListPendingThreads(ctx context.Context, chatSessionID, status string) ([]PendingThread, error) {
	return nil, nil
}

func (n *noopStore) ListActiveStates(ctx context.Context, chatSessionID, stateType string) ([]ActiveState, error) {
	return nil, nil
}

func (n *noopStore) ListCanonicalStateLayers(ctx context.Context, chatSessionID, layerType string) ([]CanonicalStateLayer, error) {
	return nil, nil
}

func (n *noopStore) ListEpisodeSummaries(ctx context.Context, chatSessionID string, limit, fromTurn, toTurn int) ([]EpisodeSummary, error) {
	return nil, nil
}

func (n *noopStore) GetEpisodeSummary(ctx context.Context, episodeID int64) (*EpisodeSummary, error) {
	return nil, ErrNotFound
}

// RollbackStore no-op stubs.

func (n *noopStore) DeleteChatLogs(ctx context.Context, chatSessionID string, fromTurn int) error {
	return nil
}
func (n *noopStore) DeleteEffectiveInputs(ctx context.Context, chatSessionID string, fromTurn int) error {
	return nil
}
func (n *noopStore) DeleteMemories(ctx context.Context, chatSessionID string, fromTurn int) error {
	return nil
}
func (n *noopStore) DeleteEvidence(ctx context.Context, chatSessionID string, fromTurn int) error {
	return nil
}
func (n *noopStore) DeleteKGTriples(ctx context.Context, chatSessionID string, fromTurn int) error {
	return nil
}
func (n *noopStore) DeleteCriticFeedback(ctx context.Context, chatSessionID string, fromTurn int) error {
	return nil
}
func (n *noopStore) DeleteCharacterEvents(ctx context.Context, chatSessionID string, fromTurn int) error {
	return nil
}
func (n *noopStore) DeleteEntities(ctx context.Context, chatSessionID string, fromTurn int) error {
	return nil
}
func (n *noopStore) DeleteTrustStates(ctx context.Context, chatSessionID string, fromTurn int) error {
	return nil
}
func (n *noopStore) DeleteStorylines(ctx context.Context, chatSessionID string, fromTurn int) error {
	return nil
}
func (n *noopStore) DeleteWorldRules(ctx context.Context, chatSessionID string, fromTurn int) error {
	return nil
}
func (n *noopStore) DeleteCharacterStates(ctx context.Context, chatSessionID string, fromTurn int) error {
	return nil
}
func (n *noopStore) DeletePendingThreads(ctx context.Context, chatSessionID string, fromTurn int) error {
	return nil
}
func (n *noopStore) DeleteActiveStates(ctx context.Context, chatSessionID string, fromTurn int) error {
	return nil
}
func (n *noopStore) DeleteCanonicalStateLayers(ctx context.Context, chatSessionID string, fromTurn int) error {
	return nil
}
func (n *noopStore) DeleteEpisodeSummaries(ctx context.Context, chatSessionID string, fromTurn int) error {
	return nil
}
func (n *noopStore) DeleteGuidancePlanState(ctx context.Context, chatSessionID string, fromTurn int) error {
	return nil
}
func (n *noopStore) DeleteChapterSummaries(ctx context.Context, chatSessionID string, fromTurn int) error {
	return nil
}
func (n *noopStore) DeleteArcSummaries(ctx context.Context, chatSessionID string, fromTurn int) error {
	return nil
}
func (n *noopStore) DeleteSagaDigests(ctx context.Context, chatSessionID string, fromTurn int) error {
	return nil
}
func (n *noopStore) DeleteSessionActiveScopes(ctx context.Context, chatSessionID string, fromTurn int) error {
	return nil
}
func (n *noopStore) DeleteProtagonistEntityMemories(ctx context.Context, chatSessionID string, fromTurn int) error {
	return nil
}
func (n *noopStore) DeleteConsequenceRecords(ctx context.Context, chatSessionID string, fromTurn int) error {
	return nil
}
func (n *noopStore) DeletePsychologyBranches(ctx context.Context, chatSessionID string, fromTurn int) error {
	return nil
}
func (n *noopStore) DeleteThemeOffscreenCarries(ctx context.Context, chatSessionID string, fromTurn int) error {
	return nil
}
func (n *noopStore) DeleteCaptureVerificationRecords(ctx context.Context, chatSessionID string, fromTurn int) error {
	return nil
}
func (n *noopStore) DeleteStatusCurrentValues(ctx context.Context, chatSessionID string, fromTurn int) error {
	return nil
}
func (n *noopStore) DeleteStatusChangeEvents(ctx context.Context, chatSessionID string, fromTurn int) error {
	return nil
}
func (n *noopStore) DeleteStatusEffects(ctx context.Context, chatSessionID string, fromTurn int) error {
	return nil
}
func (n *noopStore) DeleteSession(ctx context.Context, chatSessionID string) error { return nil }

func (n *noopStore) ListConsequenceRecords(ctx context.Context, chatSessionID string, limit int) ([]ConsequenceRecord, error) {
	return nil, nil
}

func (n *noopStore) SaveConsequenceRecord(ctx context.Context, record ConsequenceRecord) (ConsequenceRecord, error) {
	return record, nil
}

func (n *noopStore) UpdateConsequenceRecordStatus(ctx context.Context, id int64, status string, paidTurn int) error {
	return nil
}

func (n *noopStore) ListPsychologyBranches(ctx context.Context, chatSessionID string, limit int) ([]PsychologyBranch, error) {
	return nil, nil
}

func (n *noopStore) SavePsychologyBranch(ctx context.Context, branch PsychologyBranch) (PsychologyBranch, error) {
	return branch, nil
}

func (n *noopStore) UpdatePsychologyBranchStatus(ctx context.Context, id int64, status string, quietTurns int) error {
	return nil
}

func (n *noopStore) ListForkLineageRecords(ctx context.Context, chatSessionID, scopeID string, limit int) ([]ForkLineageRecord, error) {
	return nil, nil
}

func (n *noopStore) SaveForkLineageRecord(ctx context.Context, record ForkLineageRecord) (ForkLineageRecord, error) {
	return record, nil
}

func (n *noopStore) ListThemeOffscreenCarries(ctx context.Context, chatSessionID, surfaceType string, limit int) ([]ThemeOffscreenCarryRecord, error) {
	return nil, nil
}

func (n *noopStore) SaveThemeOffscreenCarry(ctx context.Context, record ThemeOffscreenCarryRecord) (ThemeOffscreenCarryRecord, error) {
	return record, nil
}

func (n *noopStore) UpdateThemeOffscreenCarryStatus(ctx context.Context, id int64, status string, quietTurns int) error {
	return nil
}
func (n *noopStore) ListCaptureVerifications(ctx context.Context, chatSessionID string, limit int) ([]CaptureVerificationRecord, error) {
	return nil, nil
}

func (n *noopStore) SaveCaptureVerification(ctx context.Context, record CaptureVerificationRecord) (CaptureVerificationRecord, error) {
	return record, nil
}

func (n *noopStore) UpdateCaptureVerificationRepair(ctx context.Context, id int64, state, degradedReason, repairEvidenceJSON string, repairedByID int64, userInputPreserved bool) error {
	return nil
}

func (n *noopStore) ListStatusSchemaProposals(ctx context.Context, chatSessionID, proposalState string, limit int) ([]StatusSchemaProposal, error) {
	return nil, nil
}

func (n *noopStore) GetStatusSchemaProposal(ctx context.Context, id int64) (StatusSchemaProposal, error) {
	return StatusSchemaProposal{}, ErrNotFound
}

func (n *noopStore) SaveStatusSchemaProposal(ctx context.Context, proposal StatusSchemaProposal) (StatusSchemaProposal, error) {
	return proposal, nil
}

func (n *noopStore) UpdateStatusSchemaProposalReview(ctx context.Context, id int64, proposalState, reviewNote, reviewer string) error {
	return nil
}

func (n *noopStore) ListStatusSchemaDefinitions(ctx context.Context, chatSessionID, registryState string, limit int) ([]StatusSchemaDefinition, error) {
	return nil, nil
}

func (n *noopStore) SaveStatusSchemaDefinitions(ctx context.Context, definitions []StatusSchemaDefinition) ([]StatusSchemaDefinition, error) {
	return definitions, nil
}

func (n *noopStore) GetStatusSchemaDefinitionByKey(ctx context.Context, chatSessionID, statusKey, ownerScope string) (StatusSchemaDefinition, error) {
	return StatusSchemaDefinition{}, ErrNotFound
}

func (n *noopStore) ListStatusCurrentValues(ctx context.Context, chatSessionID, ownerScope, ownerID, statusKey string, limit int) ([]StatusCurrentValue, error) {
	return nil, nil
}

func (n *noopStore) SaveStatusCurrentValue(ctx context.Context, value StatusCurrentValue) (StatusCurrentValue, error) {
	return value, nil
}

func (n *noopStore) ListStatusChangeEvents(ctx context.Context, chatSessionID, ownerScope, ownerID, statusKey string, limit int) ([]StatusChangeEvent, error) {
	return nil, nil
}

func (n *noopStore) SaveStatusChangeEvent(ctx context.Context, event StatusChangeEvent) (StatusChangeEvent, error) {
	return event, nil
}

func (n *noopStore) ListStatusEffects(ctx context.Context, chatSessionID, ownerScope, ownerID, effectState string, limit int) ([]StatusEffect, error) {
	return nil, nil
}

func (n *noopStore) SaveStatusEffect(ctx context.Context, effect StatusEffect) (StatusEffect, error) {
	return effect, nil
}

func (n *noopStore) UpdateStatusEffectState(ctx context.Context, id int64, effectState, clearedEvidenceJSON string, clearedTurn int) error {
	return nil
}
