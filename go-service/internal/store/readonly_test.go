package store

import (
	"context"
	"errors"
	"testing"
)

// fakeStoreForReadOnly is a minimal fake that records delegation.
type fakeStoreForReadOnly struct {
	listChatCalled      bool
	getEffCalled        bool
	listMemCalled       bool
	listEvidCalled      bool
	listKGCalled        bool
	listAuditCalled     bool
	listCriticCalled    bool
	listCharCalled      bool
	statsCalled         bool
	listSessionsCalled  bool
	getResumeCalled     bool
	listStoryCalled     bool
	listWorldCalled     bool
	listInheritedCalled bool
	listCharStateCalled bool
	getCharStateCalled  bool
	listPendingCalled   bool
	listActiveCalled    bool
	listLayerCalled     bool
	listEpisodeCalled   bool
	getEpisodeCalled    bool
}

func (f *fakeStoreForReadOnly) SaveChatLog(ctx context.Context, log *ChatLog) error { return nil }
func (f *fakeStoreForReadOnly) ListChatLogs(ctx context.Context, chatSessionID string, fromTurn, toTurn int) ([]ChatLog, error) {
	f.listChatCalled = true
	return nil, nil
}
func (f *fakeStoreForReadOnly) SaveEffectiveInput(ctx context.Context, in *EffectiveInput) error {
	return nil
}
func (f *fakeStoreForReadOnly) GetEffectiveInput(ctx context.Context, chatSessionID string, turnIndex int) (*EffectiveInput, error) {
	f.getEffCalled = true
	return nil, nil
}
func (f *fakeStoreForReadOnly) SaveMemory(ctx context.Context, m *Memory) error { return nil }
func (f *fakeStoreForReadOnly) ListMemories(ctx context.Context, chatSessionID string, fromTurn, toTurn int) ([]Memory, error) {
	f.listMemCalled = true
	return nil, nil
}
func (f *fakeStoreForReadOnly) SaveEvidence(ctx context.Context, e *DirectEvidence) error { return nil }
func (f *fakeStoreForReadOnly) ListEvidence(ctx context.Context, chatSessionID string) ([]DirectEvidence, error) {
	f.listEvidCalled = true
	return nil, nil
}
func (f *fakeStoreForReadOnly) SaveKGTriple(ctx context.Context, t *KGTriple) error { return nil }
func (f *fakeStoreForReadOnly) ListKGTriples(ctx context.Context, chatSessionID string) ([]KGTriple, error) {
	f.listKGCalled = true
	return nil, nil
}
func (f *fakeStoreForReadOnly) SaveAuditLog(ctx context.Context, a *AuditLog) error { return nil }
func (f *fakeStoreForReadOnly) ListAuditLogs(ctx context.Context, chatSessionID string, eventType string, limit int) ([]AuditLog, error) {
	f.listAuditCalled = true
	return nil, nil
}
func (f *fakeStoreForReadOnly) SaveCriticFeedback(ctx context.Context, cf *CriticFeedback) error {
	return nil
}
func (f *fakeStoreForReadOnly) ListCriticFeedback(ctx context.Context, chatSessionID string, targetType string, targetID int64) ([]CriticFeedback, error) {
	f.listCriticCalled = true
	return nil, nil
}
func (f *fakeStoreForReadOnly) SaveCharacterEvent(ctx context.Context, e *CharacterEvent) error {
	return nil
}
func (f *fakeStoreForReadOnly) ListCharacterEvents(ctx context.Context, chatSessionID string, characterName string) ([]CharacterEvent, error) {
	f.listCharCalled = true
	return nil, nil
}
func (f *fakeStoreForReadOnly) Stats(ctx context.Context) (StatsResult, error) {
	f.statsCalled = true
	return StatsResult{}, nil
}
func (f *fakeStoreForReadOnly) ListSessions(ctx context.Context) ([]SessionSummary, error) {
	f.listSessionsCalled = true
	return nil, nil
}
func (f *fakeStoreForReadOnly) GetResumePack(ctx context.Context, chatSessionID string, trigger string) (*ResumePack, error) {
	f.getResumeCalled = true
	return nil, nil
}
func (f *fakeStoreForReadOnly) ListStorylines(ctx context.Context, chatSessionID string) ([]Storyline, error) {
	f.listStoryCalled = true
	return nil, nil
}
func (f *fakeStoreForReadOnly) ListWorldRules(ctx context.Context, chatSessionID string) ([]WorldRule, error) {
	f.listWorldCalled = true
	return nil, nil
}
func (f *fakeStoreForReadOnly) ListInheritedWorldRules(ctx context.Context, chatSessionID string, activeScope, scopeName string) ([]WorldRule, error) {
	f.listInheritedCalled = true
	return nil, nil
}
func (f *fakeStoreForReadOnly) ListCharacterStates(ctx context.Context, chatSessionID string) ([]CharacterState, error) {
	f.listCharStateCalled = true
	return nil, nil
}
func (f *fakeStoreForReadOnly) GetCharacterState(ctx context.Context, chatSessionID, characterName string) (*CharacterState, error) {
	f.getCharStateCalled = true
	return nil, nil
}
func (f *fakeStoreForReadOnly) ListPendingThreads(ctx context.Context, chatSessionID, status string) ([]PendingThread, error) {
	f.listPendingCalled = true
	return nil, nil
}
func (f *fakeStoreForReadOnly) ListActiveStates(ctx context.Context, chatSessionID, stateType string) ([]ActiveState, error) {
	f.listActiveCalled = true
	return nil, nil
}
func (f *fakeStoreForReadOnly) ListCanonicalStateLayers(ctx context.Context, chatSessionID, layerType string) ([]CanonicalStateLayer, error) {
	f.listLayerCalled = true
	return nil, nil
}
func (f *fakeStoreForReadOnly) ListEpisodeSummaries(ctx context.Context, chatSessionID string, limit, fromTurn, toTurn int) ([]EpisodeSummary, error) {
	f.listEpisodeCalled = true
	return nil, nil
}
func (f *fakeStoreForReadOnly) GetEpisodeSummary(ctx context.Context, episodeID int64) (*EpisodeSummary, error) {
	f.getEpisodeCalled = true
	return nil, nil
}

func TestReadOnlyStoreBlocksAllWrites(t *testing.T) {
	fake := &fakeStoreForReadOnly{}
	ros := NewReadOnlyStore(fake)
	ctx := context.Background()

	if err := ros.SaveChatLog(ctx, &ChatLog{}); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("SaveChatLog: expected ErrNotEnabled, got %v", err)
	}
	if err := ros.SaveEffectiveInput(ctx, &EffectiveInput{}); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("SaveEffectiveInput: expected ErrNotEnabled, got %v", err)
	}
	if err := ros.SaveMemory(ctx, &Memory{}); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("SaveMemory: expected ErrNotEnabled, got %v", err)
	}
	if err := ros.SaveEvidence(ctx, &DirectEvidence{}); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("SaveEvidence: expected ErrNotEnabled, got %v", err)
	}
	if err := ros.SaveKGTriple(ctx, &KGTriple{}); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("SaveKGTriple: expected ErrNotEnabled, got %v", err)
	}
	if err := ros.SaveAuditLog(ctx, &AuditLog{}); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("SaveAuditLog: expected ErrNotEnabled, got %v", err)
	}
	if err := ros.SaveCriticFeedback(ctx, &CriticFeedback{}); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("SaveCriticFeedback: expected ErrNotEnabled, got %v", err)
	}
	if err := ros.SaveCharacterEvent(ctx, &CharacterEvent{}); !errors.Is(err, ErrNotEnabled) {
		t.Errorf("SaveCharacterEvent: expected ErrNotEnabled, got %v", err)
	}
}

func TestReadOnlyStoreDelegatesAllReads(t *testing.T) {
	fake := &fakeStoreForReadOnly{}
	ros := NewReadOnlyStore(fake)
	ctx := context.Background()

	_, _ = ros.ListChatLogs(ctx, "s1", 0, 0)
	if !fake.listChatCalled {
		t.Error("ListChatLogs not delegated")
	}
	_, _ = ros.GetEffectiveInput(ctx, "s1", 1)
	if !fake.getEffCalled {
		t.Error("GetEffectiveInput not delegated")
	}
	_, _ = ros.ListMemories(ctx, "s1", 0, 0)
	if !fake.listMemCalled {
		t.Error("ListMemories not delegated")
	}
	_, _ = ros.ListEvidence(ctx, "s1")
	if !fake.listEvidCalled {
		t.Error("ListEvidence not delegated")
	}
	_, _ = ros.ListKGTriples(ctx, "s1")
	if !fake.listKGCalled {
		t.Error("ListKGTriples not delegated")
	}
	_, _ = ros.ListAuditLogs(ctx, "s1", "", 0)
	if !fake.listAuditCalled {
		t.Error("ListAuditLogs not delegated")
	}
	_, _ = ros.ListCriticFeedback(ctx, "s1", "", 0)
	if !fake.listCriticCalled {
		t.Error("ListCriticFeedback not delegated")
	}
	_, _ = ros.ListCharacterEvents(ctx, "s1", "")
	if !fake.listCharCalled {
		t.Error("ListCharacterEvents not delegated")
	}
	_, _ = ros.Stats(ctx)
	if !fake.statsCalled {
		t.Error("Stats not delegated")
	}
	_, _ = ros.ListSessions(ctx)
	if !fake.listSessionsCalled {
		t.Error("ListSessions not delegated")
	}
	_, _ = ros.GetResumePack(ctx, "s1", "")
	if !fake.getResumeCalled {
		t.Error("GetResumePack not delegated")
	}
	_, _ = ros.ListStorylines(ctx, "s1")
	if !fake.listStoryCalled {
		t.Error("ListStorylines not delegated")
	}
	_, _ = ros.ListWorldRules(ctx, "s1")
	if !fake.listWorldCalled {
		t.Error("ListWorldRules not delegated")
	}
	_, _ = ros.ListInheritedWorldRules(ctx, "s1", "", "")
	if !fake.listInheritedCalled {
		t.Error("ListInheritedWorldRules not delegated")
	}
	_, _ = ros.ListCharacterStates(ctx, "s1")
	if !fake.listCharStateCalled {
		t.Error("ListCharacterStates not delegated")
	}
	_, _ = ros.GetCharacterState(ctx, "s1", "")
	if !fake.getCharStateCalled {
		t.Error("GetCharacterState not delegated")
	}
	_, _ = ros.ListPendingThreads(ctx, "s1", "")
	if !fake.listPendingCalled {
		t.Error("ListPendingThreads not delegated")
	}
	_, _ = ros.ListActiveStates(ctx, "s1", "")
	if !fake.listActiveCalled {
		t.Error("ListActiveStates not delegated")
	}
	_, _ = ros.ListCanonicalStateLayers(ctx, "s1", "")
	if !fake.listLayerCalled {
		t.Error("ListCanonicalStateLayers not delegated")
	}
	_, _ = ros.ListEpisodeSummaries(ctx, "s1", 0, 0, 0)
	if !fake.listEpisodeCalled {
		t.Error("ListEpisodeSummaries not delegated")
	}
	_, _ = ros.GetEpisodeSummary(ctx, 1)
	if !fake.getEpisodeCalled {
		t.Error("GetEpisodeSummary not delegated")
	}
}

// fakePingerCloser implements Store, Pinger, and Close.
type fakePingerCloser struct {
	fakeStoreForReadOnly
	pingCalled  bool
	pingErr     error
	closeCalled bool
	closeErr    error
}

func (f *fakePingerCloser) Ping(ctx context.Context) error {
	f.pingCalled = true
	return f.pingErr
}

func (f *fakePingerCloser) Close() error {
	f.closeCalled = true
	return f.closeErr
}

func TestReadOnlyStorePingDelegates(t *testing.T) {
	fake := &fakePingerCloser{}
	ros := NewReadOnlyStore(fake)
	ctx := context.Background()

	pinger, ok := ros.(Pinger)
	if !ok {
		t.Fatal("readOnlyStore should implement Pinger when delegate does")
	}
	if err := pinger.Ping(ctx); err != nil {
		t.Fatalf("Ping: expected nil error, got %v", err)
	}
	if !fake.pingCalled {
		t.Error("Ping not delegated")
	}
}

func TestReadOnlyStorePingReturnsErrNotEnabledWhenUnsupported(t *testing.T) {
	fake := &fakeStoreForReadOnly{}
	ros := NewReadOnlyStore(fake)
	ctx := context.Background()

	pinger, ok := ros.(Pinger)
	if !ok {
		t.Fatal("readOnlyStore should implement Pinger even when delegate does not")
	}
	if err := pinger.Ping(ctx); !errors.Is(err, ErrNotEnabled) {
		t.Fatalf("Ping: expected ErrNotEnabled, got %v", err)
	}
}

func TestReadOnlyStoreCloseDelegates(t *testing.T) {
	fake := &fakePingerCloser{}
	ros := NewReadOnlyStore(fake)

	closer, ok := ros.(interface{ Close() error })
	if !ok {
		t.Fatal("readOnlyStore should implement Close when delegate does")
	}
	if err := closer.Close(); err != nil {
		t.Fatalf("Close: expected nil error, got %v", err)
	}
	if !fake.closeCalled {
		t.Error("Close not delegated")
	}
}

func TestReadOnlyStoreCloseReturnsNilWhenUnsupported(t *testing.T) {
	fake := &fakeStoreForReadOnly{}
	ros := NewReadOnlyStore(fake)

	closer, ok := ros.(interface{ Close() error })
	if !ok {
		t.Fatal("readOnlyStore should implement Close even when delegate does not")
	}
	if err := closer.Close(); err != nil {
		t.Fatalf("Close: expected nil, got %v", err)
	}
}
