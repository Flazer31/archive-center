package store

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

// ---------------------------------------------------------------------------
// Fake stores for dual-write tests
// ---------------------------------------------------------------------------

type fakeStore struct {
	saveErr error

	listChat   []ChatLog
	getEff     *EffectiveInput
	listMem    []Memory
	listEvid   []DirectEvidence
	listKG     []KGTriple
	listAudit  []AuditLog
	listCritic []CriticFeedback
	listChar   []CharacterEvent
	stats      StatsResult
	sessions   []SessionSummary
	resume     *ResumePack

	saveChatLogCalls        int64
	saveEffInputCalls       int64
	saveMemoryCalls         int64
	saveEvidenceCalls       int64
	saveKGCalls             int64
	saveAuditCalls          int64
	saveCriticCalls         int64
	saveCharacterCalls      int64
	saveEntityCalls         int64
	saveTrustCalls          int64
	saveStorylineCalls      int64
	saveWorldRuleCalls      int64
	saveCharacterStateCalls int64
	saveThreadCalls         int64
	saveActiveCalls         int64

	listChatLogsCalls  int64
	getEffInputCalls   int64
	listMemoriesCalls  int64
	listEvidenceCalls  int64
	listKGCalls        int64
	listAuditCalls     int64
	listCriticCalls    int64
	listCharacterCalls int64
	statsCalls         int64
	listSessionsCalls  int64
	getResumePackCalls int64
}

func (f *fakeStore) SaveChatLog(ctx context.Context, log *ChatLog) error {
	atomic.AddInt64(&f.saveChatLogCalls, 1)
	return f.saveErr
}
func (f *fakeStore) ListChatLogs(ctx context.Context, chatSessionID string, fromTurn, toTurn int) ([]ChatLog, error) {
	atomic.AddInt64(&f.listChatLogsCalls, 1)
	return f.listChat, nil
}
func (f *fakeStore) SaveEffectiveInput(ctx context.Context, in *EffectiveInput) error {
	atomic.AddInt64(&f.saveEffInputCalls, 1)
	return f.saveErr
}
func (f *fakeStore) GetEffectiveInput(ctx context.Context, chatSessionID string, turnIndex int) (*EffectiveInput, error) {
	atomic.AddInt64(&f.getEffInputCalls, 1)
	return f.getEff, nil
}
func (f *fakeStore) SaveMemory(ctx context.Context, m *Memory) error {
	atomic.AddInt64(&f.saveMemoryCalls, 1)
	return f.saveErr
}
func (f *fakeStore) ListMemories(ctx context.Context, chatSessionID string, fromTurn, toTurn int) ([]Memory, error) {
	atomic.AddInt64(&f.listMemoriesCalls, 1)
	return f.listMem, nil
}
func (f *fakeStore) SaveEvidence(ctx context.Context, e *DirectEvidence) error {
	atomic.AddInt64(&f.saveEvidenceCalls, 1)
	return f.saveErr
}
func (f *fakeStore) ListEvidence(ctx context.Context, chatSessionID string) ([]DirectEvidence, error) {
	atomic.AddInt64(&f.listEvidenceCalls, 1)
	return f.listEvid, nil
}
func (f *fakeStore) SaveKGTriple(ctx context.Context, t *KGTriple) error {
	atomic.AddInt64(&f.saveKGCalls, 1)
	return f.saveErr
}
func (f *fakeStore) ListKGTriples(ctx context.Context, chatSessionID string) ([]KGTriple, error) {
	atomic.AddInt64(&f.listKGCalls, 1)
	return f.listKG, nil
}
func (f *fakeStore) SaveAuditLog(ctx context.Context, a *AuditLog) error {
	atomic.AddInt64(&f.saveAuditCalls, 1)
	return f.saveErr
}
func (f *fakeStore) ListAuditLogs(ctx context.Context, chatSessionID string, eventType string, limit int) ([]AuditLog, error) {
	atomic.AddInt64(&f.listAuditCalls, 1)
	return f.listAudit, nil
}
func (f *fakeStore) SaveCriticFeedback(ctx context.Context, cf *CriticFeedback) error {
	atomic.AddInt64(&f.saveCriticCalls, 1)
	return f.saveErr
}
func (f *fakeStore) ListCriticFeedback(ctx context.Context, chatSessionID string, targetType string, targetID int64) ([]CriticFeedback, error) {
	atomic.AddInt64(&f.listCriticCalls, 1)
	return f.listCritic, nil
}
func (f *fakeStore) SaveCharacterEvent(ctx context.Context, e *CharacterEvent) error {
	atomic.AddInt64(&f.saveCharacterCalls, 1)
	return f.saveErr
}
func (f *fakeStore) ListCharacterEvents(ctx context.Context, chatSessionID string, characterName string) ([]CharacterEvent, error) {
	atomic.AddInt64(&f.listCharacterCalls, 1)
	return f.listChar, nil
}
func (f *fakeStore) Stats(ctx context.Context) (StatsResult, error) {
	atomic.AddInt64(&f.statsCalls, 1)
	return f.stats, nil
}
func (f *fakeStore) ListSessions(ctx context.Context) ([]SessionSummary, error) {
	atomic.AddInt64(&f.listSessionsCalls, 1)
	return f.sessions, nil
}
func (f *fakeStore) GetResumePack(ctx context.Context, chatSessionID string, trigger string) (*ResumePack, error) {
	atomic.AddInt64(&f.getResumePackCalls, 1)
	return f.resume, nil
}

func (f *fakeStore) ListStorylines(ctx context.Context, chatSessionID string) ([]Storyline, error) {
	return nil, nil
}
func (f *fakeStore) ListWorldRules(ctx context.Context, chatSessionID string) ([]WorldRule, error) {
	return nil, nil
}
func (f *fakeStore) ListInheritedWorldRules(ctx context.Context, chatSessionID string, activeScope, scopeName string) ([]WorldRule, error) {
	return nil, nil
}
func (f *fakeStore) ListCharacterStates(ctx context.Context, chatSessionID string) ([]CharacterState, error) {
	return nil, nil
}
func (f *fakeStore) GetCharacterState(ctx context.Context, chatSessionID, characterName string) (*CharacterState, error) {
	return nil, ErrNotFound
}
func (f *fakeStore) ListPendingThreads(ctx context.Context, chatSessionID, status string) ([]PendingThread, error) {
	return nil, nil
}
func (f *fakeStore) ListActiveStates(ctx context.Context, chatSessionID, stateType string) ([]ActiveState, error) {
	return nil, nil
}
func (f *fakeStore) ListCanonicalStateLayers(ctx context.Context, chatSessionID, layerType string) ([]CanonicalStateLayer, error) {
	return nil, nil
}
func (f *fakeStore) ListEpisodeSummaries(ctx context.Context, chatSessionID string, limit, fromTurn, toTurn int) ([]EpisodeSummary, error) {
	return nil, nil
}
func (f *fakeStore) GetEpisodeSummary(ctx context.Context, episodeID int64) (*EpisodeSummary, error) {
	return nil, ErrNotFound
}
func (f *fakeStore) SaveEntity(ctx context.Context, e *Entity) error {
	atomic.AddInt64(&f.saveEntityCalls, 1)
	return f.saveErr
}
func (f *fakeStore) SaveTrust(ctx context.Context, t *Trust) error {
	atomic.AddInt64(&f.saveTrustCalls, 1)
	return f.saveErr
}
func (f *fakeStore) SaveStoryline(ctx context.Context, s *Storyline) error {
	atomic.AddInt64(&f.saveStorylineCalls, 1)
	return f.saveErr
}
func (f *fakeStore) SaveWorldRule(ctx context.Context, w *WorldRule) error {
	atomic.AddInt64(&f.saveWorldRuleCalls, 1)
	return f.saveErr
}
func (f *fakeStore) SaveCharacterState(ctx context.Context, c *CharacterState) error {
	atomic.AddInt64(&f.saveCharacterStateCalls, 1)
	return f.saveErr
}
func (f *fakeStore) SavePendingThread(ctx context.Context, p *PendingThread) error {
	atomic.AddInt64(&f.saveThreadCalls, 1)
	return f.saveErr
}
func (f *fakeStore) SaveActiveState(ctx context.Context, a *ActiveState) error {
	atomic.AddInt64(&f.saveActiveCalls, 1)
	return f.saveErr
}

// ---------------------------------------------------------------------------
// Dual-write tests
// ---------------------------------------------------------------------------

func TestDualWriteStoreImplementsInterface(t *testing.T) {
	var _ Store = NewDualWriteStore(NewNoopStore(), NewNoopStore())
}

func TestDualWriteStoreNilPrimaryAndShadow(t *testing.T) {
	// Should not panic and should satisfy Store.
	var _ Store = NewDualWriteStore(nil, nil)
}

func TestDualWritePrimarySuccessShadowCalled(t *testing.T) {
	primary := &fakeStore{}
	shadow := &fakeStore{}
	s := NewDualWriteStore(primary, shadow).(*dualWriteStore)

	ctx := context.Background()
	log := &ChatLog{ChatSessionID: "s1", TurnIndex: 1}
	if err := s.SaveChatLog(ctx, log); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt64(&primary.saveChatLogCalls) != 1 {
		t.Errorf("expected primary SaveChatLog called once, got %d", primary.saveChatLogCalls)
	}
	if atomic.LoadInt64(&shadow.saveChatLogCalls) != 1 {
		t.Errorf("expected shadow SaveChatLog called once, got %d", shadow.saveChatLogCalls)
	}

	in := &EffectiveInput{ChatSessionID: "s1", TurnIndex: 1}
	if err := s.SaveEffectiveInput(ctx, in); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt64(&shadow.saveEffInputCalls) != 1 {
		t.Errorf("expected shadow SaveEffectiveInput called once, got %d", shadow.saveEffInputCalls)
	}

	mem := &Memory{ChatSessionID: "s1", TurnIndex: 1}
	if err := s.SaveMemory(ctx, mem); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt64(&shadow.saveMemoryCalls) != 1 {
		t.Errorf("expected shadow SaveMemory called once, got %d", shadow.saveMemoryCalls)
	}

	evid := &DirectEvidence{ChatSessionID: "s1"}
	if err := s.SaveEvidence(ctx, evid); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt64(&shadow.saveEvidenceCalls) != 1 {
		t.Errorf("expected shadow SaveEvidence called once, got %d", shadow.saveEvidenceCalls)
	}

	kg := &KGTriple{ChatSessionID: "s1"}
	if err := s.SaveKGTriple(ctx, kg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt64(&shadow.saveKGCalls) != 1 {
		t.Errorf("expected shadow SaveKGTriple called once, got %d", shadow.saveKGCalls)
	}

	audit := &AuditLog{EventType: "test"}
	if err := s.SaveAuditLog(ctx, audit); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt64(&shadow.saveAuditCalls) != 1 {
		t.Errorf("expected shadow SaveAuditLog called once, got %d", shadow.saveAuditCalls)
	}

	critic := &CriticFeedback{ChatSessionID: "s1"}
	if err := s.SaveCriticFeedback(ctx, critic); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt64(&shadow.saveCriticCalls) != 1 {
		t.Errorf("expected shadow SaveCriticFeedback called once, got %d", shadow.saveCriticCalls)
	}

	char := &CharacterEvent{ChatSessionID: "s1"}
	if err := s.SaveCharacterEvent(ctx, char); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt64(&shadow.saveCharacterCalls) != 1 {
		t.Errorf("expected shadow SaveCharacterEvent called once, got %d", shadow.saveCharacterCalls)
	}
}

func TestDualWriteShadowFailureRecorded(t *testing.T) {
	primary := &fakeStore{}
	shadow := &fakeStore{saveErr: errors.New("shadow down")}
	s := NewDualWriteStore(primary, shadow).(*dualWriteStore)

	ctx := context.Background()
	log := &ChatLog{ChatSessionID: "s1", TurnIndex: 1}
	if err := s.SaveChatLog(ctx, log); err != nil {
		t.Fatalf("expected nil error when shadow fails, got %v", err)
	}
	failures, lastErr := s.ShadowStatus()
	if failures != 1 {
		t.Errorf("expected 1 shadow failure, got %d", failures)
	}
	if lastErr == nil || lastErr.Error() != "shadow down" {
		t.Errorf("expected shadow lastErr 'shadow down', got %v", lastErr)
	}
}

func TestDualWritePrimaryFailureNoShadowCall(t *testing.T) {
	primary := &fakeStore{saveErr: errors.New("primary down")}
	shadow := &fakeStore{}
	s := NewDualWriteStore(primary, shadow).(*dualWriteStore)

	ctx := context.Background()
	log := &ChatLog{ChatSessionID: "s1", TurnIndex: 1}
	if err := s.SaveChatLog(ctx, log); err == nil {
		t.Fatal("expected primary error")
	}
	if atomic.LoadInt64(&shadow.saveChatLogCalls) != 0 {
		t.Errorf("expected shadow SaveChatLog not called, got %d", shadow.saveChatLogCalls)
	}
}

func TestDualWriteReadsOnlyPrimary(t *testing.T) {
	primary := &fakeStore{}
	shadow := &fakeStore{}
	s := NewDualWriteStore(primary, shadow).(*dualWriteStore)

	ctx := context.Background()
	_, err := s.ListChatLogs(ctx, "s1", 0, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt64(&shadow.listChatLogsCalls) != 0 {
		t.Errorf("expected shadow ListChatLogs not called, got %d", shadow.listChatLogsCalls)
	}

	_, err = s.GetEffectiveInput(ctx, "s1", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt64(&shadow.getEffInputCalls) != 0 {
		t.Errorf("expected shadow GetEffectiveInput not called, got %d", shadow.getEffInputCalls)
	}

	_, err = s.ListMemories(ctx, "s1", 0, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt64(&shadow.listMemoriesCalls) != 0 {
		t.Errorf("expected shadow ListMemories not called, got %d", shadow.listMemoriesCalls)
	}

	_, err = s.ListEvidence(ctx, "s1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt64(&shadow.listEvidenceCalls) != 0 {
		t.Errorf("expected shadow ListEvidence not called, got %d", shadow.listEvidenceCalls)
	}

	_, err = s.ListKGTriples(ctx, "s1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt64(&shadow.listKGCalls) != 0 {
		t.Errorf("expected shadow ListKGTriples not called, got %d", shadow.listKGCalls)
	}

	_, err = s.ListAuditLogs(ctx, "s1", "type", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt64(&shadow.listAuditCalls) != 0 {
		t.Errorf("expected shadow ListAuditLogs not called, got %d", shadow.listAuditCalls)
	}

	_, err = s.ListCriticFeedback(ctx, "s1", "memory", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt64(&shadow.listCriticCalls) != 0 {
		t.Errorf("expected shadow ListCriticFeedback not called, got %d", shadow.listCriticCalls)
	}

	_, err = s.ListCharacterEvents(ctx, "s1", "X")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt64(&shadow.listCharacterCalls) != 0 {
		t.Errorf("expected shadow ListCharacterEvents not called, got %d", shadow.listCharacterCalls)
	}

	_, err = s.Stats(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt64(&shadow.statsCalls) != 0 {
		t.Errorf("expected shadow Stats not called, got %d", shadow.statsCalls)
	}

	_, err = s.ListSessions(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt64(&shadow.listSessionsCalls) != 0 {
		t.Errorf("expected shadow ListSessions not called, got %d", shadow.listSessionsCalls)
	}

	_, err = s.GetResumePack(ctx, "s1", "resume")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if atomic.LoadInt64(&shadow.getResumePackCalls) != 0 {
		t.Errorf("expected shadow GetResumePack not called, got %d", shadow.getResumePackCalls)
	}
}

func TestDualWriteSaverInterfacesDelegate(t *testing.T) {
	primary := &fakeStore{}
	shadow := &fakeStore{}
	d := NewDualWriteStore(primary, shadow).(*dualWriteStore)
	ctx := context.Background()

	d.SaveEntity(ctx, &Entity{ChatSessionID: "s1"})
	d.SaveTrust(ctx, &Trust{ChatSessionID: "s1"})
	d.SaveStoryline(ctx, &Storyline{ChatSessionID: "s1"})
	d.SaveWorldRule(ctx, &WorldRule{ChatSessionID: "s1"})
	d.SaveCharacterState(ctx, &CharacterState{ChatSessionID: "s1"})
	d.SavePendingThread(ctx, &PendingThread{ChatSessionID: "s1"})
	d.SaveActiveState(ctx, &ActiveState{ChatSessionID: "s1"})

	if atomic.LoadInt64(&shadow.saveEntityCalls) != 1 {
		t.Errorf("shadow SaveEntity calls = %d, want 1", shadow.saveEntityCalls)
	}
	if atomic.LoadInt64(&shadow.saveTrustCalls) != 1 {
		t.Errorf("shadow SaveTrust calls = %d, want 1", shadow.saveTrustCalls)
	}
	if atomic.LoadInt64(&shadow.saveStorylineCalls) != 1 {
		t.Errorf("shadow SaveStoryline calls = %d, want 1", shadow.saveStorylineCalls)
	}
	if atomic.LoadInt64(&shadow.saveWorldRuleCalls) != 1 {
		t.Errorf("shadow SaveWorldRule calls = %d, want 1", shadow.saveWorldRuleCalls)
	}
	if atomic.LoadInt64(&shadow.saveCharacterStateCalls) != 1 {
		t.Errorf("shadow SaveCharacterState calls = %d, want 1", shadow.saveCharacterStateCalls)
	}
	if atomic.LoadInt64(&shadow.saveThreadCalls) != 1 {
		t.Errorf("shadow SavePendingThread calls = %d, want 1", shadow.saveThreadCalls)
	}
	if atomic.LoadInt64(&shadow.saveActiveCalls) != 1 {
		t.Errorf("shadow SaveActiveState calls = %d, want 1", shadow.saveActiveCalls)
	}
}
