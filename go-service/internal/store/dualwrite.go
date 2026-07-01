package store

import (
	"context"
	"fmt"
	"sync"
)

// dualWriteStore wraps a primary Store and a shadow Store.
// Reads are delegated to primary only. Writes go to primary first,
// then to shadow if primary succeeds. Shadow failures are recorded
// but do not surface as errors to callers.
type dualWriteStore struct {
	primary Store
	shadow  Store

	mu             sync.Mutex
	shadowFailures int64
	shadowLastErr  error
}

// NewDualWriteStore returns a Store that writes to both primary and shadow.
// If shadow is nil it is treated as a no-op shadow.
// If primary is nil it is treated as a no-op primary.
func NewDualWriteStore(primary Store, shadow Store) Store {
	if primary == nil {
		primary = NewNoopStore()
	}
	if shadow == nil {
		shadow = NewNoopStore()
	}
	return &dualWriteStore{
		primary: primary,
		shadow:  shadow,
	}
}

// ShadowStatus returns the number of shadow write failures and the last error.
func (d *dualWriteStore) ShadowStatus() (failures int64, lastErr error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.shadowFailures, d.shadowLastErr
}

func (d *dualWriteStore) recordShadowErr(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.shadowFailures++
	d.shadowLastErr = err
}

// SaveChatLog writes to primary then shadow.
func (d *dualWriteStore) SaveChatLog(ctx context.Context, log *ChatLog) error {
	if err := d.primary.SaveChatLog(ctx, log); err != nil {
		return err
	}
	if err := d.shadow.SaveChatLog(ctx, log); err != nil {
		d.recordShadowErr(err)
	}
	return nil
}

// ListChatLogs reads from primary only.
func (d *dualWriteStore) ListChatLogs(ctx context.Context, chatSessionID string, fromTurn, toTurn int) ([]ChatLog, error) {
	return d.primary.ListChatLogs(ctx, chatSessionID, fromTurn, toTurn)
}

// ReadSessionStateSnapshot reads from primary only, preserving the same
// read-authority semantics as every other dual-write read.
func (d *dualWriteStore) ReadSessionStateSnapshot(ctx context.Context, chatSessionID string) (*SessionStateSnapshot, error) {
	reader, ok := d.primary.(SessionStateSnapshotReader)
	if !ok {
		return nil, ErrNotEnabled
	}
	return reader.ReadSessionStateSnapshot(ctx, chatSessionID)
}

// SaveEffectiveInput writes to primary then shadow.
func (d *dualWriteStore) SaveEffectiveInput(ctx context.Context, in *EffectiveInput) error {
	if err := d.primary.SaveEffectiveInput(ctx, in); err != nil {
		return err
	}
	if err := d.shadow.SaveEffectiveInput(ctx, in); err != nil {
		d.recordShadowErr(err)
	}
	return nil
}

// GetEffectiveInput reads from primary only.
func (d *dualWriteStore) GetEffectiveInput(ctx context.Context, chatSessionID string, turnIndex int) (*EffectiveInput, error) {
	return d.primary.GetEffectiveInput(ctx, chatSessionID, turnIndex)
}

// SaveMemory writes to primary then shadow.
func (d *dualWriteStore) SaveMemory(ctx context.Context, m *Memory) error {
	if err := d.primary.SaveMemory(ctx, m); err != nil {
		return err
	}
	if err := d.shadow.SaveMemory(ctx, m); err != nil {
		d.recordShadowErr(err)
	}
	return nil
}

// ListMemories reads from primary only.
func (d *dualWriteStore) ListMemories(ctx context.Context, chatSessionID string, fromTurn, toTurn int) ([]Memory, error) {
	return d.primary.ListMemories(ctx, chatSessionID, fromTurn, toTurn)
}

func (d *dualWriteStore) UpdateMemoryImportance(ctx context.Context, chatSessionID string, memoryID int64, importance float64) error {
	primary, ok := d.primary.(interface {
		UpdateMemoryImportance(context.Context, string, int64, float64) error
	})
	if !ok {
		return ErrNotEnabled
	}
	if err := primary.UpdateMemoryImportance(ctx, chatSessionID, memoryID, importance); err != nil {
		return err
	}
	if shadow, ok := d.shadow.(interface {
		UpdateMemoryImportance(context.Context, string, int64, float64) error
	}); ok {
		if err := shadow.UpdateMemoryImportance(ctx, chatSessionID, memoryID, importance); err != nil {
			d.recordShadowErr(err)
		}
	}
	return nil
}

func (d *dualWriteStore) UpdateMemoryExplorerFields(ctx context.Context, chatSessionID string, memoryID int64, patch MemoryExplorerPatch) error {
	primary, ok := d.primary.(ExplorerMutationStore)
	if !ok {
		return ErrNotEnabled
	}
	if err := primary.UpdateMemoryExplorerFields(ctx, chatSessionID, memoryID, patch); err != nil {
		return err
	}
	if shadow, ok := d.shadow.(ExplorerMutationStore); ok {
		if err := shadow.UpdateMemoryExplorerFields(ctx, chatSessionID, memoryID, patch); err != nil {
			d.recordShadowErr(err)
		}
	}
	return nil
}

func (d *dualWriteStore) UpdateKGTripleExplorerFields(ctx context.Context, chatSessionID string, tripleID int64, patch KGTripleExplorerPatch) error {
	primary, ok := d.primary.(ExplorerMutationStore)
	if !ok {
		return ErrNotEnabled
	}
	if err := primary.UpdateKGTripleExplorerFields(ctx, chatSessionID, tripleID, patch); err != nil {
		return err
	}
	if shadow, ok := d.shadow.(ExplorerMutationStore); ok {
		if err := shadow.UpdateKGTripleExplorerFields(ctx, chatSessionID, tripleID, patch); err != nil {
			d.recordShadowErr(err)
		}
	}
	return nil
}

func (d *dualWriteStore) UpdateDirectEvidenceExplorerFields(ctx context.Context, chatSessionID string, recordID int64, patch DirectEvidenceExplorerPatch) error {
	primary, ok := d.primary.(ExplorerMutationStore)
	if !ok {
		return ErrNotEnabled
	}
	if err := primary.UpdateDirectEvidenceExplorerFields(ctx, chatSessionID, recordID, patch); err != nil {
		return err
	}
	if shadow, ok := d.shadow.(ExplorerMutationStore); ok {
		if err := shadow.UpdateDirectEvidenceExplorerFields(ctx, chatSessionID, recordID, patch); err != nil {
			d.recordShadowErr(err)
		}
	}
	return nil
}

func (d *dualWriteStore) DeleteMemoryByID(ctx context.Context, chatSessionID string, memoryID int64) error {
	primary, ok := d.primary.(ExplorerMutationStore)
	if !ok {
		return ErrNotEnabled
	}
	if err := primary.DeleteMemoryByID(ctx, chatSessionID, memoryID); err != nil {
		return err
	}
	if shadow, ok := d.shadow.(ExplorerMutationStore); ok {
		if err := shadow.DeleteMemoryByID(ctx, chatSessionID, memoryID); err != nil {
			d.recordShadowErr(err)
		}
	}
	return nil
}

func (d *dualWriteStore) DeleteDirectEvidenceByID(ctx context.Context, chatSessionID string, recordID int64) error {
	primary, ok := d.primary.(ExplorerMutationStore)
	if !ok {
		return ErrNotEnabled
	}
	if err := primary.DeleteDirectEvidenceByID(ctx, chatSessionID, recordID); err != nil {
		return err
	}
	if shadow, ok := d.shadow.(ExplorerMutationStore); ok {
		if err := shadow.DeleteDirectEvidenceByID(ctx, chatSessionID, recordID); err != nil {
			d.recordShadowErr(err)
		}
	}
	return nil
}

func (d *dualWriteStore) DeleteKGTripleByID(ctx context.Context, chatSessionID string, tripleID int64) error {
	primary, ok := d.primary.(ExplorerMutationStore)
	if !ok {
		return ErrNotEnabled
	}
	if err := primary.DeleteKGTripleByID(ctx, chatSessionID, tripleID); err != nil {
		return err
	}
	if shadow, ok := d.shadow.(ExplorerMutationStore); ok {
		if err := shadow.DeleteKGTripleByID(ctx, chatSessionID, tripleID); err != nil {
			d.recordShadowErr(err)
		}
	}
	return nil
}

func (d *dualWriteStore) DeleteCharacterByName(ctx context.Context, chatSessionID string, characterName string) error {
	primary, ok := d.primary.(ExplorerMutationStore)
	if !ok {
		return ErrNotEnabled
	}
	if err := primary.DeleteCharacterByName(ctx, chatSessionID, characterName); err != nil {
		return err
	}
	if shadow, ok := d.shadow.(ExplorerMutationStore); ok {
		if err := shadow.DeleteCharacterByName(ctx, chatSessionID, characterName); err != nil {
			d.recordShadowErr(err)
		}
	}
	return nil
}

// SaveEvidence writes to primary then shadow.
func (d *dualWriteStore) SaveEvidence(ctx context.Context, e *DirectEvidence) error {
	if err := d.primary.SaveEvidence(ctx, e); err != nil {
		return err
	}
	if err := d.shadow.SaveEvidence(ctx, e); err != nil {
		d.recordShadowErr(err)
	}
	return nil
}

// ListEvidence reads from primary only.
func (d *dualWriteStore) ListEvidence(ctx context.Context, chatSessionID string) ([]DirectEvidence, error) {
	return d.primary.ListEvidence(ctx, chatSessionID)
}

// SaveKGTriple writes to primary then shadow.
func (d *dualWriteStore) SaveKGTriple(ctx context.Context, t *KGTriple) error {
	if err := d.primary.SaveKGTriple(ctx, t); err != nil {
		return err
	}
	if err := d.shadow.SaveKGTriple(ctx, t); err != nil {
		d.recordShadowErr(err)
	}
	return nil
}

// ListKGTriples reads from primary only.
func (d *dualWriteStore) ListKGTriples(ctx context.Context, chatSessionID string) ([]KGTriple, error) {
	return d.primary.ListKGTriples(ctx, chatSessionID)
}

// SaveAuditLog writes to primary then shadow.
func (d *dualWriteStore) SaveAuditLog(ctx context.Context, a *AuditLog) error {
	if err := d.primary.SaveAuditLog(ctx, a); err != nil {
		return err
	}
	if err := d.shadow.SaveAuditLog(ctx, a); err != nil {
		d.recordShadowErr(err)
	}
	return nil
}

// ListAuditLogs reads from primary only.
func (d *dualWriteStore) ListAuditLogs(ctx context.Context, chatSessionID string, eventType string, limit int) ([]AuditLog, error) {
	return d.primary.ListAuditLogs(ctx, chatSessionID, eventType, limit)
}

func (d *dualWriteStore) CountAuditLogs(ctx context.Context, chatSessionID string, eventType string) (int, error) {
	if counter, ok := d.primary.(AuditLogCounter); ok {
		return counter.CountAuditLogs(ctx, chatSessionID, eventType)
	}
	items, err := d.primary.ListAuditLogs(ctx, chatSessionID, eventType, 0)
	if err != nil {
		return 0, err
	}
	return len(items), nil
}

// SaveCriticFeedback writes to primary then shadow.
func (d *dualWriteStore) SaveCriticFeedback(ctx context.Context, f *CriticFeedback) error {
	if err := d.primary.SaveCriticFeedback(ctx, f); err != nil {
		return err
	}
	if err := d.shadow.SaveCriticFeedback(ctx, f); err != nil {
		d.recordShadowErr(err)
	}
	return nil
}

// ListCriticFeedback reads from primary only.
func (d *dualWriteStore) ListCriticFeedback(ctx context.Context, chatSessionID string, targetType string, targetID int64) ([]CriticFeedback, error) {
	return d.primary.ListCriticFeedback(ctx, chatSessionID, targetType, targetID)
}

// SaveCharacterEvent writes to primary then shadow.
func (d *dualWriteStore) SaveCharacterEvent(ctx context.Context, e *CharacterEvent) error {
	if err := d.primary.SaveCharacterEvent(ctx, e); err != nil {
		return err
	}
	if err := d.shadow.SaveCharacterEvent(ctx, e); err != nil {
		d.recordShadowErr(err)
	}
	return nil
}

// SaveEntity writes to primary then shadow.
func (d *dualWriteStore) SaveEntity(ctx context.Context, e *Entity) error {
	if primary, ok := d.primary.(interface {
		SaveEntity(context.Context, *Entity) error
	}); ok {
		if err := primary.SaveEntity(ctx, e); err != nil {
			return err
		}
	}
	if shadow, ok := d.shadow.(interface {
		SaveEntity(context.Context, *Entity) error
	}); ok {
		if err := shadow.SaveEntity(ctx, e); err != nil {
			d.recordShadowErr(err)
		}
	}
	return nil
}

// SaveTrust writes to primary then shadow.
func (d *dualWriteStore) SaveTrust(ctx context.Context, t *Trust) error {
	if primary, ok := d.primary.(interface {
		SaveTrust(context.Context, *Trust) error
	}); ok {
		if err := primary.SaveTrust(ctx, t); err != nil {
			return err
		}
	}
	if shadow, ok := d.shadow.(interface {
		SaveTrust(context.Context, *Trust) error
	}); ok {
		if err := shadow.SaveTrust(ctx, t); err != nil {
			d.recordShadowErr(err)
		}
	}
	return nil
}

// ListCharacterEvents reads from primary only.
func (d *dualWriteStore) ListCharacterEvents(ctx context.Context, chatSessionID string, characterName string) ([]CharacterEvent, error) {
	return d.primary.ListCharacterEvents(ctx, chatSessionID, characterName)
}

// Stats reads from primary only.
func (d *dualWriteStore) Stats(ctx context.Context) (StatsResult, error) {
	return d.primary.Stats(ctx)
}

// ListSessions reads from primary only.
func (d *dualWriteStore) ListSessions(ctx context.Context) ([]SessionSummary, error) {
	return d.primary.ListSessions(ctx)
}

// GetResumePack reads from primary only.
func (d *dualWriteStore) GetResumePack(ctx context.Context, chatSessionID string, trigger string) (*ResumePack, error) {
	return d.primary.GetResumePack(ctx, chatSessionID, trigger)
}

// ListStorylines reads from primary only.
func (d *dualWriteStore) ListStorylines(ctx context.Context, chatSessionID string) ([]Storyline, error) {
	return d.primary.ListStorylines(ctx, chatSessionID)
}

// SaveStoryline writes to primary then shadow.
func (d *dualWriteStore) SaveStoryline(ctx context.Context, s *Storyline) error {
	if primary, ok := d.primary.(interface {
		SaveStoryline(context.Context, *Storyline) error
	}); ok {
		if err := primary.SaveStoryline(ctx, s); err != nil {
			return err
		}
	}
	if shadow, ok := d.shadow.(interface {
		SaveStoryline(context.Context, *Storyline) error
	}); ok {
		if err := shadow.SaveStoryline(ctx, s); err != nil {
			d.recordShadowErr(err)
		}
	}
	return nil
}

func (d *dualWriteStore) PatchStoryline(ctx context.Context, storylineID int64, updates map[string]any) ([]string, error) {
	var updated []string
	if primary, ok := d.primary.(interface {
		PatchStoryline(context.Context, int64, map[string]any) ([]string, error)
	}); ok {
		var err error
		updated, err = primary.PatchStoryline(ctx, storylineID, updates)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, ErrNotEnabled
	}
	if shadow, ok := d.shadow.(interface {
		PatchStoryline(context.Context, int64, map[string]any) ([]string, error)
	}); ok {
		if _, err := shadow.PatchStoryline(ctx, storylineID, updates); err != nil {
			d.recordShadowErr(err)
		}
	}
	return updated, nil
}

func (d *dualWriteStore) PatchStorylineTrust(ctx context.Context, storylineID int64, updates map[string]any) ([]string, error) {
	var updated []string
	if primary, ok := d.primary.(interface {
		PatchStorylineTrust(context.Context, int64, map[string]any) ([]string, error)
	}); ok {
		var err error
		updated, err = primary.PatchStorylineTrust(ctx, storylineID, updates)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, ErrNotEnabled
	}
	if shadow, ok := d.shadow.(interface {
		PatchStorylineTrust(context.Context, int64, map[string]any) ([]string, error)
	}); ok {
		if _, err := shadow.PatchStorylineTrust(ctx, storylineID, updates); err != nil {
			d.recordShadowErr(err)
		}
	}
	return updated, nil
}

func (d *dualWriteStore) DeleteStoryline(ctx context.Context, storylineID int64) error {
	if primary, ok := d.primary.(interface {
		DeleteStoryline(context.Context, int64) error
	}); ok {
		if err := primary.DeleteStoryline(ctx, storylineID); err != nil {
			return err
		}
	} else {
		return ErrNotEnabled
	}
	if shadow, ok := d.shadow.(interface {
		DeleteStoryline(context.Context, int64) error
	}); ok {
		if err := shadow.DeleteStoryline(ctx, storylineID); err != nil {
			d.recordShadowErr(err)
		}
	}
	return nil
}

// ListWorldRules reads from primary only.
func (d *dualWriteStore) ListWorldRules(ctx context.Context, chatSessionID string) ([]WorldRule, error) {
	return d.primary.ListWorldRules(ctx, chatSessionID)
}

// SaveWorldRule writes to primary then shadow.
func (d *dualWriteStore) SaveWorldRule(ctx context.Context, w *WorldRule) error {
	if primary, ok := d.primary.(interface {
		SaveWorldRule(context.Context, *WorldRule) error
	}); ok {
		if err := primary.SaveWorldRule(ctx, w); err != nil {
			return err
		}
	}
	if shadow, ok := d.shadow.(interface {
		SaveWorldRule(context.Context, *WorldRule) error
	}); ok {
		if err := shadow.SaveWorldRule(ctx, w); err != nil {
			d.recordShadowErr(err)
		}
	}
	return nil
}

// ListInheritedWorldRules reads from primary only.
func (d *dualWriteStore) ListInheritedWorldRules(ctx context.Context, chatSessionID string, activeScope, scopeName string) ([]WorldRule, error) {
	return d.primary.ListInheritedWorldRules(ctx, chatSessionID, activeScope, scopeName)
}

// ListCharacterStates reads from primary only.
func (d *dualWriteStore) ListCharacterStates(ctx context.Context, chatSessionID string) ([]CharacterState, error) {
	return d.primary.ListCharacterStates(ctx, chatSessionID)
}

// GetCharacterState reads from primary only.
func (d *dualWriteStore) GetCharacterState(ctx context.Context, chatSessionID, characterName string) (*CharacterState, error) {
	return d.primary.GetCharacterState(ctx, chatSessionID, characterName)
}

// SaveCharacterState writes to primary then shadow when both stores support it.
func (d *dualWriteStore) SaveCharacterState(ctx context.Context, c *CharacterState) error {
	if primary, ok := d.primary.(interface {
		SaveCharacterState(context.Context, *CharacterState) error
	}); ok {
		if err := primary.SaveCharacterState(ctx, c); err != nil {
			return err
		}
	}
	if shadow, ok := d.shadow.(interface {
		SaveCharacterState(context.Context, *CharacterState) error
	}); ok {
		if err := shadow.SaveCharacterState(ctx, c); err != nil {
			d.recordShadowErr(err)
		}
	}
	return nil
}

// ListPendingThreads reads from primary only.
func (d *dualWriteStore) ListPendingThreads(ctx context.Context, chatSessionID, status string) ([]PendingThread, error) {
	return d.primary.ListPendingThreads(ctx, chatSessionID, status)
}

// SavePendingThread writes to primary then shadow when both stores support it.
func (d *dualWriteStore) SavePendingThread(ctx context.Context, p *PendingThread) error {
	if primary, ok := d.primary.(interface {
		SavePendingThread(context.Context, *PendingThread) error
	}); ok {
		if err := primary.SavePendingThread(ctx, p); err != nil {
			return err
		}
	}
	if shadow, ok := d.shadow.(interface {
		SavePendingThread(context.Context, *PendingThread) error
	}); ok {
		if err := shadow.SavePendingThread(ctx, p); err != nil {
			d.recordShadowErr(err)
		}
	}
	return nil
}

func (d *dualWriteStore) PatchPendingThread(ctx context.Context, hookID int64, updates map[string]any) ([]string, error) {
	var updated []string
	if primary, ok := d.primary.(interface {
		PatchPendingThread(context.Context, int64, map[string]any) ([]string, error)
	}); ok {
		var err error
		updated, err = primary.PatchPendingThread(ctx, hookID, updates)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, ErrNotEnabled
	}
	if shadow, ok := d.shadow.(interface {
		PatchPendingThread(context.Context, int64, map[string]any) ([]string, error)
	}); ok {
		if _, err := shadow.PatchPendingThread(ctx, hookID, updates); err != nil {
			d.recordShadowErr(err)
		}
	}
	return updated, nil
}

func (d *dualWriteStore) PatchPendingThreadTrust(ctx context.Context, hookID int64, updates map[string]any) ([]string, error) {
	var updated []string
	if primary, ok := d.primary.(interface {
		PatchPendingThreadTrust(context.Context, int64, map[string]any) ([]string, error)
	}); ok {
		var err error
		updated, err = primary.PatchPendingThreadTrust(ctx, hookID, updates)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, ErrNotEnabled
	}
	if shadow, ok := d.shadow.(interface {
		PatchPendingThreadTrust(context.Context, int64, map[string]any) ([]string, error)
	}); ok {
		if _, err := shadow.PatchPendingThreadTrust(ctx, hookID, updates); err != nil {
			d.recordShadowErr(err)
		}
	}
	return updated, nil
}

func (d *dualWriteStore) DeletePendingThread(ctx context.Context, hookID int64) error {
	if primary, ok := d.primary.(interface {
		DeletePendingThread(context.Context, int64) error
	}); ok {
		if err := primary.DeletePendingThread(ctx, hookID); err != nil {
			return err
		}
	} else {
		return ErrNotEnabled
	}
	if shadow, ok := d.shadow.(interface {
		DeletePendingThread(context.Context, int64) error
	}); ok {
		if err := shadow.DeletePendingThread(ctx, hookID); err != nil {
			d.recordShadowErr(err)
		}
	}
	return nil
}

// ListActiveStates reads from primary only.
func (d *dualWriteStore) ListActiveStates(ctx context.Context, chatSessionID, stateType string) ([]ActiveState, error) {
	return d.primary.ListActiveStates(ctx, chatSessionID, stateType)
}

// SaveActiveState writes to primary then shadow when both stores support it.
func (d *dualWriteStore) SaveActiveState(ctx context.Context, a *ActiveState) error {
	if primary, ok := d.primary.(interface {
		SaveActiveState(context.Context, *ActiveState) error
	}); ok {
		if err := primary.SaveActiveState(ctx, a); err != nil {
			return err
		}
	}
	if shadow, ok := d.shadow.(interface {
		SaveActiveState(context.Context, *ActiveState) error
	}); ok {
		if err := shadow.SaveActiveState(ctx, a); err != nil {
			d.recordShadowErr(err)
		}
	}
	return nil
}

// ListCanonicalStateLayers reads from primary only.
func (d *dualWriteStore) ListCanonicalStateLayers(ctx context.Context, chatSessionID, layerType string) ([]CanonicalStateLayer, error) {
	return d.primary.ListCanonicalStateLayers(ctx, chatSessionID, layerType)
}

func (d *dualWriteStore) SaveCanonicalStateLayer(ctx context.Context, item *CanonicalStateLayer) error {
	if primary, ok := d.primary.(interface {
		SaveCanonicalStateLayer(context.Context, *CanonicalStateLayer) error
	}); ok {
		if err := primary.SaveCanonicalStateLayer(ctx, item); err != nil {
			return err
		}
	}
	if shadow, ok := d.shadow.(interface {
		SaveCanonicalStateLayer(context.Context, *CanonicalStateLayer) error
	}); ok {
		if err := shadow.SaveCanonicalStateLayer(ctx, item); err != nil {
			d.recordShadowErr(err)
		}
	}
	return nil
}

// ListEpisodeSummaries reads from primary only.
func (d *dualWriteStore) ListEpisodeSummaries(ctx context.Context, chatSessionID string, limit, fromTurn, toTurn int) ([]EpisodeSummary, error) {
	return d.primary.ListEpisodeSummaries(ctx, chatSessionID, limit, fromTurn, toTurn)
}

// GetGuidancePlanState reads from primary only.
func (d *dualWriteStore) GetGuidancePlanState(ctx context.Context, chatSessionID string) (*GuidancePlanState, error) {
	primary, ok := d.primary.(GuidancePlanStateStore)
	if !ok {
		return nil, ErrNotEnabled
	}
	return primary.GetGuidancePlanState(ctx, chatSessionID)
}

// UpsertGuidancePlanState writes to primary then shadow.
func (d *dualWriteStore) UpsertGuidancePlanState(ctx context.Context, item *GuidancePlanState) error {
	primary, ok := d.primary.(GuidancePlanStateStore)
	if !ok {
		return ErrNotEnabled
	}
	if err := primary.UpsertGuidancePlanState(ctx, item); err != nil {
		return err
	}
	if shadow, ok := d.shadow.(GuidancePlanStateStore); ok {
		if err := shadow.UpsertGuidancePlanState(ctx, item); err != nil {
			d.recordShadowErr(err)
		}
	}
	return nil
}

func (d *dualWriteStore) ListConsequenceRecords(ctx context.Context, chatSessionID string, limit int) ([]ConsequenceRecord, error) {
	primary, ok := d.primary.(ConsequenceRecordStore)
	if !ok {
		return nil, ErrNotEnabled
	}
	return primary.ListConsequenceRecords(ctx, chatSessionID, limit)
}

func (d *dualWriteStore) SaveConsequenceRecord(ctx context.Context, record ConsequenceRecord) (ConsequenceRecord, error) {
	primary, ok := d.primary.(ConsequenceRecordStore)
	if !ok {
		return record, ErrNotEnabled
	}
	saved, err := primary.SaveConsequenceRecord(ctx, record)
	if err != nil {
		return saved, err
	}
	if shadow, ok := d.shadow.(ConsequenceRecordStore); ok {
		if _, err := shadow.SaveConsequenceRecord(ctx, saved); err != nil {
			d.recordShadowErr(err)
		}
	}
	return saved, nil
}

func (d *dualWriteStore) UpdateConsequenceRecordStatus(ctx context.Context, id int64, status string, paidTurn int) error {
	primary, ok := d.primary.(ConsequenceRecordStore)
	if !ok {
		return ErrNotEnabled
	}
	if err := primary.UpdateConsequenceRecordStatus(ctx, id, status, paidTurn); err != nil {
		return err
	}
	if shadow, ok := d.shadow.(ConsequenceRecordStore); ok {
		if err := shadow.UpdateConsequenceRecordStatus(ctx, id, status, paidTurn); err != nil {
			d.recordShadowErr(err)
		}
	}
	return nil
}

func (d *dualWriteStore) ListPsychologyBranches(ctx context.Context, chatSessionID string, limit int) ([]PsychologyBranch, error) {
	primary, ok := d.primary.(PsychologyBranchStore)
	if !ok {
		return nil, ErrNotEnabled
	}
	return primary.ListPsychologyBranches(ctx, chatSessionID, limit)
}

func (d *dualWriteStore) SavePsychologyBranch(ctx context.Context, branch PsychologyBranch) (PsychologyBranch, error) {
	primary, ok := d.primary.(PsychologyBranchStore)
	if !ok {
		return branch, ErrNotEnabled
	}
	saved, err := primary.SavePsychologyBranch(ctx, branch)
	if err != nil {
		return saved, err
	}
	if shadow, ok := d.shadow.(PsychologyBranchStore); ok {
		if _, err := shadow.SavePsychologyBranch(ctx, saved); err != nil {
			d.recordShadowErr(err)
		}
	}
	return saved, nil
}

func (d *dualWriteStore) UpdatePsychologyBranchStatus(ctx context.Context, id int64, status string, quietTurns int) error {
	primary, ok := d.primary.(PsychologyBranchStore)
	if !ok {
		return ErrNotEnabled
	}
	if err := primary.UpdatePsychologyBranchStatus(ctx, id, status, quietTurns); err != nil {
		return err
	}
	if shadow, ok := d.shadow.(PsychologyBranchStore); ok {
		if err := shadow.UpdatePsychologyBranchStatus(ctx, id, status, quietTurns); err != nil {
			d.recordShadowErr(err)
		}
	}
	return nil
}

func (d *dualWriteStore) ListForkLineageRecords(ctx context.Context, chatSessionID, scopeID string, limit int) ([]ForkLineageRecord, error) {
	primary, ok := d.primary.(ForkLineageStore)
	if !ok {
		return nil, ErrNotEnabled
	}
	return primary.ListForkLineageRecords(ctx, chatSessionID, scopeID, limit)
}

func (d *dualWriteStore) SaveForkLineageRecord(ctx context.Context, record ForkLineageRecord) (ForkLineageRecord, error) {
	primary, ok := d.primary.(ForkLineageStore)
	if !ok {
		return record, ErrNotEnabled
	}
	saved, err := primary.SaveForkLineageRecord(ctx, record)
	if err != nil {
		return saved, err
	}
	if shadow, ok := d.shadow.(ForkLineageStore); ok {
		if _, err := shadow.SaveForkLineageRecord(ctx, saved); err != nil {
			d.recordShadowErr(err)
		}
	}
	return saved, nil
}

func (d *dualWriteStore) ListThemeOffscreenCarries(ctx context.Context, chatSessionID, surfaceType string, limit int) ([]ThemeOffscreenCarryRecord, error) {
	primary, ok := d.primary.(ThemeOffscreenCarryStore)
	if !ok {
		return nil, ErrNotEnabled
	}
	return primary.ListThemeOffscreenCarries(ctx, chatSessionID, surfaceType, limit)
}

func (d *dualWriteStore) SaveThemeOffscreenCarry(ctx context.Context, record ThemeOffscreenCarryRecord) (ThemeOffscreenCarryRecord, error) {
	primary, ok := d.primary.(ThemeOffscreenCarryStore)
	if !ok {
		return record, ErrNotEnabled
	}
	saved, err := primary.SaveThemeOffscreenCarry(ctx, record)
	if err != nil {
		return saved, err
	}
	if shadow, ok := d.shadow.(ThemeOffscreenCarryStore); ok {
		if _, err := shadow.SaveThemeOffscreenCarry(ctx, saved); err != nil {
			d.recordShadowErr(err)
		}
	}
	return saved, nil
}

func (d *dualWriteStore) UpdateThemeOffscreenCarryStatus(ctx context.Context, id int64, status string, quietTurns int) error {
	primary, ok := d.primary.(ThemeOffscreenCarryStore)
	if !ok {
		return ErrNotEnabled
	}
	if err := primary.UpdateThemeOffscreenCarryStatus(ctx, id, status, quietTurns); err != nil {
		return err
	}
	if shadow, ok := d.shadow.(ThemeOffscreenCarryStore); ok {
		if err := shadow.UpdateThemeOffscreenCarryStatus(ctx, id, status, quietTurns); err != nil {
			d.recordShadowErr(err)
		}
	}
	return nil
}

func (d *dualWriteStore) ListCaptureVerifications(ctx context.Context, chatSessionID string, limit int) ([]CaptureVerificationRecord, error) {
	primary, ok := d.primary.(CaptureVerificationStore)
	if !ok {
		return nil, ErrNotEnabled
	}
	return primary.ListCaptureVerifications(ctx, chatSessionID, limit)
}

func (d *dualWriteStore) SaveCaptureVerification(ctx context.Context, record CaptureVerificationRecord) (CaptureVerificationRecord, error) {
	primary, ok := d.primary.(CaptureVerificationStore)
	if !ok {
		return record, ErrNotEnabled
	}
	saved, err := primary.SaveCaptureVerification(ctx, record)
	if err != nil {
		return saved, err
	}
	if shadow, ok := d.shadow.(CaptureVerificationStore); ok {
		if _, err := shadow.SaveCaptureVerification(ctx, saved); err != nil {
			d.recordShadowErr(err)
		}
	}
	return saved, nil
}

func (d *dualWriteStore) UpdateCaptureVerificationRepair(ctx context.Context, id int64, state, degradedReason, repairEvidenceJSON string, repairedByID int64, userInputPreserved bool) error {
	primary, ok := d.primary.(CaptureVerificationStore)
	if !ok {
		return ErrNotEnabled
	}
	if err := primary.UpdateCaptureVerificationRepair(ctx, id, state, degradedReason, repairEvidenceJSON, repairedByID, userInputPreserved); err != nil {
		return err
	}
	if shadow, ok := d.shadow.(CaptureVerificationStore); ok {
		if err := shadow.UpdateCaptureVerificationRepair(ctx, id, state, degradedReason, repairEvidenceJSON, repairedByID, userInputPreserved); err != nil {
			d.recordShadowErr(err)
		}
	}
	return nil
}

func (d *dualWriteStore) ListStatusSchemaProposals(ctx context.Context, chatSessionID, proposalState string, limit int) ([]StatusSchemaProposal, error) {
	primary, ok := d.primary.(StatusSchemaProposalStore)
	if !ok {
		return nil, ErrNotEnabled
	}
	return primary.ListStatusSchemaProposals(ctx, chatSessionID, proposalState, limit)
}

func (d *dualWriteStore) GetStatusSchemaProposal(ctx context.Context, id int64) (StatusSchemaProposal, error) {
	primary, ok := d.primary.(StatusSchemaProposalStore)
	if !ok {
		return StatusSchemaProposal{}, ErrNotEnabled
	}
	return primary.GetStatusSchemaProposal(ctx, id)
}

func (d *dualWriteStore) SaveStatusSchemaProposal(ctx context.Context, proposal StatusSchemaProposal) (StatusSchemaProposal, error) {
	primary, ok := d.primary.(StatusSchemaProposalStore)
	if !ok {
		return proposal, ErrNotEnabled
	}
	saved, err := primary.SaveStatusSchemaProposal(ctx, proposal)
	if err != nil {
		return saved, err
	}
	if shadow, ok := d.shadow.(StatusSchemaProposalStore); ok {
		if _, err := shadow.SaveStatusSchemaProposal(ctx, saved); err != nil {
			d.recordShadowErr(err)
		}
	}
	return saved, nil
}

func (d *dualWriteStore) UpdateStatusSchemaProposalReview(ctx context.Context, id int64, proposalState, reviewNote, reviewer string) error {
	primary, ok := d.primary.(StatusSchemaProposalStore)
	if !ok {
		return ErrNotEnabled
	}
	if err := primary.UpdateStatusSchemaProposalReview(ctx, id, proposalState, reviewNote, reviewer); err != nil {
		return err
	}
	if shadow, ok := d.shadow.(StatusSchemaProposalStore); ok {
		if err := shadow.UpdateStatusSchemaProposalReview(ctx, id, proposalState, reviewNote, reviewer); err != nil {
			d.recordShadowErr(err)
		}
	}
	return nil
}

func (d *dualWriteStore) ListStatusSchemaDefinitions(ctx context.Context, chatSessionID, registryState string, limit int) ([]StatusSchemaDefinition, error) {
	primary, ok := d.primary.(StatusSchemaRegistryStore)
	if !ok {
		return nil, ErrNotEnabled
	}
	return primary.ListStatusSchemaDefinitions(ctx, chatSessionID, registryState, limit)
}

func (d *dualWriteStore) SaveStatusSchemaDefinitions(ctx context.Context, definitions []StatusSchemaDefinition) ([]StatusSchemaDefinition, error) {
	primary, ok := d.primary.(StatusSchemaRegistryStore)
	if !ok {
		return definitions, ErrNotEnabled
	}
	saved, err := primary.SaveStatusSchemaDefinitions(ctx, definitions)
	if err != nil {
		return saved, err
	}
	if shadow, ok := d.shadow.(StatusSchemaRegistryStore); ok {
		if _, err := shadow.SaveStatusSchemaDefinitions(ctx, saved); err != nil {
			d.recordShadowErr(err)
		}
	}
	return saved, nil
}

func (d *dualWriteStore) GetStatusSchemaDefinitionByKey(ctx context.Context, chatSessionID, statusKey, ownerScope string) (StatusSchemaDefinition, error) {
	primary, ok := d.primary.(StatusSchemaRegistryStore)
	if !ok {
		return StatusSchemaDefinition{}, ErrNotEnabled
	}
	return primary.GetStatusSchemaDefinitionByKey(ctx, chatSessionID, statusKey, ownerScope)
}

func (d *dualWriteStore) ListStatusCurrentValues(ctx context.Context, chatSessionID, ownerScope, ownerID, statusKey string, limit int) ([]StatusCurrentValue, error) {
	primary, ok := d.primary.(StatusCurrentValueStore)
	if !ok {
		return nil, ErrNotEnabled
	}
	return primary.ListStatusCurrentValues(ctx, chatSessionID, ownerScope, ownerID, statusKey, limit)
}

func (d *dualWriteStore) SaveStatusCurrentValue(ctx context.Context, value StatusCurrentValue) (StatusCurrentValue, error) {
	primary, ok := d.primary.(StatusCurrentValueStore)
	if !ok {
		return value, ErrNotEnabled
	}
	saved, err := primary.SaveStatusCurrentValue(ctx, value)
	if err != nil {
		return saved, err
	}
	if shadow, ok := d.shadow.(StatusCurrentValueStore); ok {
		if _, err := shadow.SaveStatusCurrentValue(ctx, saved); err != nil {
			d.recordShadowErr(err)
		}
	}
	return saved, nil
}

func (d *dualWriteStore) ListStatusChangeEvents(ctx context.Context, chatSessionID, ownerScope, ownerID, statusKey string, limit int) ([]StatusChangeEvent, error) {
	primary, ok := d.primary.(StatusLifecycleStore)
	if !ok {
		return nil, ErrNotEnabled
	}
	return primary.ListStatusChangeEvents(ctx, chatSessionID, ownerScope, ownerID, statusKey, limit)
}

func (d *dualWriteStore) SaveStatusChangeEvent(ctx context.Context, event StatusChangeEvent) (StatusChangeEvent, error) {
	primary, ok := d.primary.(StatusLifecycleStore)
	if !ok {
		return event, ErrNotEnabled
	}
	saved, err := primary.SaveStatusChangeEvent(ctx, event)
	if err != nil {
		return saved, err
	}
	if shadow, ok := d.shadow.(StatusLifecycleStore); ok {
		if _, err := shadow.SaveStatusChangeEvent(ctx, saved); err != nil {
			d.recordShadowErr(err)
		}
	}
	return saved, nil
}

func (d *dualWriteStore) ListStatusEffects(ctx context.Context, chatSessionID, ownerScope, ownerID, effectState string, limit int) ([]StatusEffect, error) {
	primary, ok := d.primary.(StatusLifecycleStore)
	if !ok {
		return nil, ErrNotEnabled
	}
	return primary.ListStatusEffects(ctx, chatSessionID, ownerScope, ownerID, effectState, limit)
}

func (d *dualWriteStore) SaveStatusEffect(ctx context.Context, effect StatusEffect) (StatusEffect, error) {
	primary, ok := d.primary.(StatusLifecycleStore)
	if !ok {
		return effect, ErrNotEnabled
	}
	saved, err := primary.SaveStatusEffect(ctx, effect)
	if err != nil {
		return saved, err
	}
	if shadow, ok := d.shadow.(StatusLifecycleStore); ok {
		if _, err := shadow.SaveStatusEffect(ctx, saved); err != nil {
			d.recordShadowErr(err)
		}
	}
	return saved, nil
}

func (d *dualWriteStore) UpdateStatusEffectState(ctx context.Context, id int64, effectState, clearedEvidenceJSON string, clearedTurn int) error {
	primary, ok := d.primary.(StatusLifecycleStore)
	if !ok {
		return ErrNotEnabled
	}
	if err := primary.UpdateStatusEffectState(ctx, id, effectState, clearedEvidenceJSON, clearedTurn); err != nil {
		return err
	}
	if shadow, ok := d.shadow.(StatusLifecycleStore); ok {
		if err := shadow.UpdateStatusEffectState(ctx, id, effectState, clearedEvidenceJSON, clearedTurn); err != nil {
			d.recordShadowErr(err)
		}
	}
	return nil
}

// GetEpisodeSummary reads from primary only.
func (d *dualWriteStore) GetEpisodeSummary(ctx context.Context, episodeID int64) (*EpisodeSummary, error) {
	return d.primary.GetEpisodeSummary(ctx, episodeID)
}

func (d *dualWriteStore) SaveEpisodeSummary(ctx context.Context, item *EpisodeSummary) error {
	primary, ok := d.primary.(EpisodeSummaryStore)
	if !ok {
		return ErrNotEnabled
	}
	if err := primary.SaveEpisodeSummary(ctx, item); err != nil {
		return err
	}
	if shadow, ok := d.shadow.(EpisodeSummaryStore); ok {
		cp := *item
		if err := shadow.SaveEpisodeSummary(ctx, &cp); err != nil {
			d.recordShadowErr(err)
		}
	}
	return nil
}

func (d *dualWriteStore) DeleteEpisodeSummary(ctx context.Context, episodeID int64) error {
	primary, ok := d.primary.(interface {
		DeleteEpisodeSummary(context.Context, int64) error
	})
	if !ok {
		return ErrNotEnabled
	}
	if err := primary.DeleteEpisodeSummary(ctx, episodeID); err != nil {
		return err
	}
	if shadow, ok := d.shadow.(interface {
		DeleteEpisodeSummary(context.Context, int64) error
	}); ok {
		if err := shadow.DeleteEpisodeSummary(ctx, episodeID); err != nil {
			d.recordShadowErr(err)
		}
	}
	return nil
}

func (d *dualWriteStore) DeleteEpisodeSummariesInRange(ctx context.Context, sid string, fromTurn, toTurn int) (int64, error) {
	primary, ok := d.primary.(interface {
		DeleteEpisodeSummariesInRange(context.Context, string, int, int) (int64, error)
	})
	if !ok {
		return 0, ErrNotEnabled
	}
	n, err := primary.DeleteEpisodeSummariesInRange(ctx, sid, fromTurn, toTurn)
	if err != nil {
		return 0, err
	}
	if shadow, ok := d.shadow.(interface {
		DeleteEpisodeSummariesInRange(context.Context, string, int, int) (int64, error)
	}); ok {
		if _, err := shadow.DeleteEpisodeSummariesInRange(ctx, sid, fromTurn, toTurn); err != nil {
			d.recordShadowErr(err)
		}
	}
	return n, nil
}

// dualWriteRollback delegates delete to both primary and shadow if they implement RollbackStore.
func (d *dualWriteStore) dualWriteRollback(label string, fn func(RollbackStore) error) error {
	if primary, ok := d.primary.(RollbackStore); ok {
		if err := fn(primary); err != nil {
			return fmt.Errorf("%s primary: %w", label, err)
		}
	}
	if shadow, ok := d.shadow.(RollbackStore); ok {
		if err := fn(shadow); err != nil {
			d.recordShadowErr(fmt.Errorf("%s shadow: %w", label, err))
		}
	}
	return nil
}

func (d *dualWriteStore) DeleteChatLogs(ctx context.Context, sid string, fromTurn int) error {
	return d.dualWriteRollback("DeleteChatLogs", func(s RollbackStore) error { return s.DeleteChatLogs(ctx, sid, fromTurn) })
}
func (d *dualWriteStore) DeleteEffectiveInputs(ctx context.Context, sid string, fromTurn int) error {
	return d.dualWriteRollback("DeleteEffectiveInputs", func(s RollbackStore) error { return s.DeleteEffectiveInputs(ctx, sid, fromTurn) })
}
func (d *dualWriteStore) DeleteMemories(ctx context.Context, sid string, fromTurn int) error {
	return d.dualWriteRollback("DeleteMemories", func(s RollbackStore) error { return s.DeleteMemories(ctx, sid, fromTurn) })
}
func (d *dualWriteStore) DeleteEvidence(ctx context.Context, sid string, fromTurn int) error {
	return d.dualWriteRollback("DeleteEvidence", func(s RollbackStore) error { return s.DeleteEvidence(ctx, sid, fromTurn) })
}
func (d *dualWriteStore) DeleteKGTriples(ctx context.Context, sid string, fromTurn int) error {
	return d.dualWriteRollback("DeleteKGTriples", func(s RollbackStore) error { return s.DeleteKGTriples(ctx, sid, fromTurn) })
}
func (d *dualWriteStore) DeleteCriticFeedback(ctx context.Context, sid string, fromTurn int) error {
	return d.dualWriteRollback("DeleteCriticFeedback", func(s RollbackStore) error { return s.DeleteCriticFeedback(ctx, sid, fromTurn) })
}
func (d *dualWriteStore) DeleteCharacterEvents(ctx context.Context, sid string, fromTurn int) error {
	return d.dualWriteRollback("DeleteCharacterEvents", func(s RollbackStore) error { return s.DeleteCharacterEvents(ctx, sid, fromTurn) })
}
func (d *dualWriteStore) DeleteEntities(ctx context.Context, sid string, fromTurn int) error {
	return d.dualWriteRollback("DeleteEntities", func(s RollbackStore) error { return s.DeleteEntities(ctx, sid, fromTurn) })
}
func (d *dualWriteStore) DeleteTrustStates(ctx context.Context, sid string, fromTurn int) error {
	return d.dualWriteRollback("DeleteTrustStates", func(s RollbackStore) error { return s.DeleteTrustStates(ctx, sid, fromTurn) })
}
func (d *dualWriteStore) DeleteStorylines(ctx context.Context, sid string, fromTurn int) error {
	return d.dualWriteRollback("DeleteStorylines", func(s RollbackStore) error { return s.DeleteStorylines(ctx, sid, fromTurn) })
}
func (d *dualWriteStore) DeleteWorldRules(ctx context.Context, sid string, fromTurn int) error {
	return d.dualWriteRollback("DeleteWorldRules", func(s RollbackStore) error { return s.DeleteWorldRules(ctx, sid, fromTurn) })
}
func (d *dualWriteStore) DeleteCharacterStates(ctx context.Context, sid string, fromTurn int) error {
	return d.dualWriteRollback("DeleteCharacterStates", func(s RollbackStore) error { return s.DeleteCharacterStates(ctx, sid, fromTurn) })
}
func (d *dualWriteStore) DeletePendingThreads(ctx context.Context, sid string, fromTurn int) error {
	return d.dualWriteRollback("DeletePendingThreads", func(s RollbackStore) error { return s.DeletePendingThreads(ctx, sid, fromTurn) })
}
func (d *dualWriteStore) DeleteActiveStates(ctx context.Context, sid string, fromTurn int) error {
	return d.dualWriteRollback("DeleteActiveStates", func(s RollbackStore) error { return s.DeleteActiveStates(ctx, sid, fromTurn) })
}
func (d *dualWriteStore) DeleteCanonicalStateLayers(ctx context.Context, sid string, fromTurn int) error {
	return d.dualWriteRollback("DeleteCanonicalStateLayers", func(s RollbackStore) error { return s.DeleteCanonicalStateLayers(ctx, sid, fromTurn) })
}
func (d *dualWriteStore) DeleteEpisodeSummaries(ctx context.Context, sid string, fromTurn int) error {
	return d.dualWriteRollback("DeleteEpisodeSummaries", func(s RollbackStore) error { return s.DeleteEpisodeSummaries(ctx, sid, fromTurn) })
}
func (d *dualWriteStore) DeleteGuidancePlanState(ctx context.Context, sid string, fromTurn int) error {
	return d.dualWriteRollback("DeleteGuidancePlanState", func(s RollbackStore) error { return s.DeleteGuidancePlanState(ctx, sid, fromTurn) })
}
func (d *dualWriteStore) DeleteChapterSummaries(ctx context.Context, sid string, fromTurn int) error {
	return d.dualWriteRollback("DeleteChapterSummaries", func(s RollbackStore) error { return s.DeleteChapterSummaries(ctx, sid, fromTurn) })
}
func (d *dualWriteStore) DeleteArcSummaries(ctx context.Context, sid string, fromTurn int) error {
	return d.dualWriteRollback("DeleteArcSummaries", func(s RollbackStore) error { return s.DeleteArcSummaries(ctx, sid, fromTurn) })
}
func (d *dualWriteStore) DeleteSagaDigests(ctx context.Context, sid string, fromTurn int) error {
	return d.dualWriteRollback("DeleteSagaDigests", func(s RollbackStore) error { return s.DeleteSagaDigests(ctx, sid, fromTurn) })
}
func (d *dualWriteStore) DeleteSessionActiveScopes(ctx context.Context, sid string, fromTurn int) error {
	return d.dualWriteRollback("DeleteSessionActiveScopes", func(s RollbackStore) error { return s.DeleteSessionActiveScopes(ctx, sid, fromTurn) })
}
func (d *dualWriteStore) DeleteProtagonistEntityMemories(ctx context.Context, sid string, fromTurn int) error {
	return d.dualWriteRollback("DeleteProtagonistEntityMemories", func(s RollbackStore) error {
		return s.DeleteProtagonistEntityMemories(ctx, sid, fromTurn)
	})
}
func (d *dualWriteStore) DeleteConsequenceRecords(ctx context.Context, sid string, fromTurn int) error {
	return d.dualWriteRollback("DeleteConsequenceRecords", func(s RollbackStore) error {
		return s.DeleteConsequenceRecords(ctx, sid, fromTurn)
	})
}
func (d *dualWriteStore) DeletePsychologyBranches(ctx context.Context, sid string, fromTurn int) error {
	return d.dualWriteRollback("DeletePsychologyBranches", func(s RollbackStore) error {
		return s.DeletePsychologyBranches(ctx, sid, fromTurn)
	})
}
func (d *dualWriteStore) DeleteThemeOffscreenCarries(ctx context.Context, sid string, fromTurn int) error {
	return d.dualWriteRollback("DeleteThemeOffscreenCarries", func(s RollbackStore) error {
		return s.DeleteThemeOffscreenCarries(ctx, sid, fromTurn)
	})
}
func (d *dualWriteStore) DeleteCaptureVerificationRecords(ctx context.Context, sid string, fromTurn int) error {
	return d.dualWriteRollback("DeleteCaptureVerificationRecords", func(s RollbackStore) error {
		return s.DeleteCaptureVerificationRecords(ctx, sid, fromTurn)
	})
}
func (d *dualWriteStore) DeleteStatusCurrentValues(ctx context.Context, sid string, fromTurn int) error {
	return d.dualWriteRollback("DeleteStatusCurrentValues", func(s RollbackStore) error {
		return s.DeleteStatusCurrentValues(ctx, sid, fromTurn)
	})
}
func (d *dualWriteStore) DeleteStatusChangeEvents(ctx context.Context, sid string, fromTurn int) error {
	return d.dualWriteRollback("DeleteStatusChangeEvents", func(s RollbackStore) error {
		return s.DeleteStatusChangeEvents(ctx, sid, fromTurn)
	})
}
func (d *dualWriteStore) DeleteStatusEffects(ctx context.Context, sid string, fromTurn int) error {
	return d.dualWriteRollback("DeleteStatusEffects", func(s RollbackStore) error {
		return s.DeleteStatusEffects(ctx, sid, fromTurn)
	})
}
func (d *dualWriteStore) DeleteSession(ctx context.Context, sid string) error {
	return d.dualWriteRollback("DeleteSession", func(s RollbackStore) error { return s.DeleteSession(ctx, sid) })
}
