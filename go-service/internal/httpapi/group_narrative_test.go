package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/store"
)

type narrativeFakeStore struct {
	chatLogs               []store.ChatLog
	memories               []store.Memory
	evidence               []store.DirectEvidence
	kgTriples              []store.KGTriple
	storylines             []store.Storyline
	worldRules             []store.WorldRule
	activeScope            *store.SessionActiveScope
	characterStates        []store.CharacterState
	characterState         *store.CharacterState
	characterEvents        []store.CharacterEvent
	pendingThreads         []store.PendingThread
	activeStates           []store.ActiveState
	canonicalStateLayers   []store.CanonicalStateLayer
	episodeSummaries       []store.EpisodeSummary
	episodeSummary         *store.EpisodeSummary
	savedEpisodeSummaries  []store.EpisodeSummary
	deletedEpisodeID       int64
	deletedEpisodeRanges   []string
	auditLogs              []store.AuditLog
	criticFeedback         []store.CriticFeedback
	resumePack             *store.ResumePack
	chapterSummaries       []store.ChapterSummary
	savedChapterSummaries  []store.ChapterSummary
	arcSummaries           []store.ArcSummary
	savedArcSummaries      []store.ArcSummary
	sagaDigests            []store.SagaDigest
	savedSagaDigests       []store.SagaDigest
	sessions               []store.SessionSummary
	errNotEnabled          bool
	characterNotFound      bool
	storylineNotFound      bool
	episodeNotFound        bool
	deleteSessionCalled    bool
	deleteSessionErr       error
	storylinePatches       []map[string]any
	storylineTrustPatch    map[string]any
	deletedStorylineID     int64
	worldRulePatches       []map[string]any
	worldRuleTrustPatch    map[string]any
	deletedWorldRuleID     int64
	deletedCharacterName   string
	pendingThreadPatches   []map[string]any
	pendingThreadTrust     map[string]any
	deletedPendingThread   int64
	savedStorylines        []store.Storyline
	savedWorldRules        []store.WorldRule
	savedActiveScope       *store.SessionActiveScope
	savedCharacterStates   []store.CharacterState
	savedCharacterEvents   []store.CharacterEvent
	aggregateReadCalls     int
	listChatLogCalls       int
	guidancePlanState      *store.GuidancePlanState
	savedGuidancePlanState *store.GuidancePlanState
	guidancePlanStateErr   error
	guidancePlanUpsertErr  error
}

func orderedUpdatedFields(updates map[string]any, order []string) []string {
	out := make([]string, 0, len(updates))
	for _, key := range order {
		if _, ok := updates[key]; ok {
			out = append(out, key)
		}
	}
	return out
}

func (f *narrativeFakeStore) SaveChatLog(ctx context.Context, log *store.ChatLog) error { return nil }

func (f *narrativeFakeStore) ListChatLogs(ctx context.Context, chatSessionID string, fromTurn, toTurn int) ([]store.ChatLog, error) {
	f.listChatLogCalls++
	if f.errNotEnabled {
		return nil, store.ErrNotEnabled
	}
	return f.chatLogs, nil
}

func (f *narrativeFakeStore) SaveEffectiveInput(ctx context.Context, in *store.EffectiveInput) error {
	return nil
}

func (f *narrativeFakeStore) GetEffectiveInput(ctx context.Context, chatSessionID string, turnIndex int) (*store.EffectiveInput, error) {
	return nil, store.ErrNotFound
}

func (f *narrativeFakeStore) SaveMemory(ctx context.Context, m *store.Memory) error { return nil }

func (f *narrativeFakeStore) ListMemories(ctx context.Context, chatSessionID string, fromTurn, toTurn int) ([]store.Memory, error) {
	if f.errNotEnabled {
		return nil, store.ErrNotEnabled
	}
	return f.memories, nil
}

func (f *narrativeFakeStore) SaveEvidence(ctx context.Context, e *store.DirectEvidence) error {
	return nil
}

func (f *narrativeFakeStore) ListEvidence(ctx context.Context, chatSessionID string) ([]store.DirectEvidence, error) {
	if f.errNotEnabled {
		return nil, store.ErrNotEnabled
	}
	return f.evidence, nil
}

func (f *narrativeFakeStore) SaveKGTriple(ctx context.Context, t *store.KGTriple) error { return nil }

func (f *narrativeFakeStore) ListKGTriples(ctx context.Context, chatSessionID string) ([]store.KGTriple, error) {
	if f.errNotEnabled {
		return nil, store.ErrNotEnabled
	}
	return f.kgTriples, nil
}

func (f *narrativeFakeStore) SaveAuditLog(ctx context.Context, a *store.AuditLog) error {
	if a.ID == 0 {
		a.ID = int64(len(f.auditLogs) + 1)
	}
	f.auditLogs = append([]store.AuditLog{*a}, f.auditLogs...)
	return nil
}

func (f *narrativeFakeStore) ListAuditLogs(ctx context.Context, chatSessionID string, eventType string, limit int) ([]store.AuditLog, error) {
	if f.errNotEnabled {
		return nil, store.ErrNotEnabled
	}
	out := []store.AuditLog{}
	for _, item := range f.auditLogs {
		if strings.TrimSpace(chatSessionID) != "" && item.ChatSessionID != chatSessionID {
			continue
		}
		if strings.TrimSpace(eventType) != "" && item.EventType != eventType {
			continue
		}
		out = append(out, item)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (f *narrativeFakeStore) SaveCriticFeedback(ctx context.Context, cf *store.CriticFeedback) error {
	if cf.ID == 0 {
		cf.ID = int64(len(f.criticFeedback) + 1)
	}
	f.criticFeedback = append([]store.CriticFeedback{*cf}, f.criticFeedback...)
	return nil
}

func (f *narrativeFakeStore) ListCriticFeedback(ctx context.Context, chatSessionID string, targetType string, targetID int64) ([]store.CriticFeedback, error) {
	if f.errNotEnabled {
		return nil, store.ErrNotEnabled
	}
	out := []store.CriticFeedback{}
	for _, item := range f.criticFeedback {
		if strings.TrimSpace(chatSessionID) != "" && item.ChatSessionID != chatSessionID {
			continue
		}
		if strings.TrimSpace(targetType) != "" && item.TargetType != targetType {
			continue
		}
		if targetID > 0 && item.TargetID != targetID {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (f *narrativeFakeStore) SaveCharacterEvent(ctx context.Context, e *store.CharacterEvent) error {
	f.savedCharacterEvents = append(f.savedCharacterEvents, *e)
	f.characterEvents = append(f.characterEvents, *e)
	return nil
}

func (f *narrativeFakeStore) ListCharacterEvents(ctx context.Context, chatSessionID string, characterName string) ([]store.CharacterEvent, error) {
	return f.characterEvents, nil
}

func (f *narrativeFakeStore) ReadSessionStateSnapshot(ctx context.Context, chatSessionID string) (*store.SessionStateSnapshot, error) {
	f.aggregateReadCalls++
	if f.errNotEnabled {
		return nil, store.ErrNotEnabled
	}
	return &store.SessionStateSnapshot{
		ActiveStates:         f.activeStates,
		CanonicalStateLayers: f.canonicalStateLayers,
		Storylines:           f.storylines,
		CharacterStates:      f.characterStates,
		WorldRules:           f.worldRules,
		PendingThreads:       f.pendingThreads,
		CharacterEvents:      f.characterEvents,
		RecentChatLogs:       f.chatLogs,
		SingleConnection:     true,
		TraceMethods:         []string{"fake-aggregate"},
	}, nil
}

func (f *narrativeFakeStore) Stats(ctx context.Context) (store.StatsResult, error) {
	return store.StatsResult{}, nil
}

func (f *narrativeFakeStore) ListSessions(ctx context.Context) ([]store.SessionSummary, error) {
	if f.errNotEnabled {
		return nil, store.ErrNotEnabled
	}
	return f.sessions, nil
}

func (f *narrativeFakeStore) GetResumePack(ctx context.Context, chatSessionID string, trigger string) (*store.ResumePack, error) {
	if f.errNotEnabled {
		return nil, store.ErrNotEnabled
	}
	return f.resumePack, nil
}

func (f *narrativeFakeStore) SaveChapterSummary(ctx context.Context, item *store.ChapterSummary) error {
	if f.errNotEnabled {
		return store.ErrNotEnabled
	}
	if item.ID == 0 {
		item.ID = int64(len(f.savedChapterSummaries) + 1)
	}
	f.savedChapterSummaries = append(f.savedChapterSummaries, *item)
	f.chapterSummaries = append(f.chapterSummaries, *item)
	return nil
}

func (f *narrativeFakeStore) SearchChapterSummaries(ctx context.Context, chatSessionID, query string, fromTurn, toTurn, limit int) ([]store.ChapterSummary, error) {
	if f.errNotEnabled {
		return nil, store.ErrNotEnabled
	}
	if limit <= 0 {
		limit = 20
	}
	query = strings.ToLower(strings.TrimSpace(query))
	out := []store.ChapterSummary{}
	for _, ch := range f.chapterSummaries {
		if strings.TrimSpace(chatSessionID) != "" && ch.ChatSessionID != chatSessionID {
			continue
		}
		if fromTurn > 0 && ch.ToTurn > 0 && ch.ToTurn < fromTurn {
			continue
		}
		if toTurn > 0 && ch.FromTurn > toTurn {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(ch.ChapterTitle+" "+ch.SummaryText+" "+ch.ResumeText+" "+ch.OpenLoopsJSON+" "+ch.RelationshipChangesJSON+" "+ch.WorldChangesJSON+" "+ch.CallbackCandidatesJSON), query) {
			continue
		}
		out = append(out, ch)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (f *narrativeFakeStore) SaveArcSummary(ctx context.Context, chatSessionID string, item *store.ArcSummary) error {
	if f.errNotEnabled {
		return store.ErrNotEnabled
	}
	if item.ChatSessionID == "" {
		item.ChatSessionID = chatSessionID
	}
	if item.ID == 0 {
		item.ID = int64(len(f.savedArcSummaries) + 1)
	}
	f.savedArcSummaries = append(f.savedArcSummaries, *item)
	f.arcSummaries = append(f.arcSummaries, *item)
	return nil
}

func (f *narrativeFakeStore) GetLatestArcSummary(ctx context.Context, chatSessionID string) (*store.ArcSummary, error) {
	items, err := f.SearchArcSummaries(ctx, chatSessionID, "", 0, 0, 1)
	if err != nil || len(items) == 0 {
		return nil, err
	}
	return &items[0], nil
}

func (f *narrativeFakeStore) ListArcSummaries(ctx context.Context, chatSessionID string, status string, limit int) ([]store.ArcSummary, error) {
	items, err := f.SearchArcSummaries(ctx, chatSessionID, "", 0, 0, limit)
	if err != nil || strings.TrimSpace(status) == "" {
		return items, err
	}
	out := []store.ArcSummary{}
	for _, item := range items {
		if item.ArcStatus == status {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *narrativeFakeStore) SearchArcSummaries(ctx context.Context, chatSessionID, query string, fromTurn, toTurn, limit int) ([]store.ArcSummary, error) {
	if f.errNotEnabled {
		return nil, store.ErrNotEnabled
	}
	if limit <= 0 {
		limit = 20
	}
	query = strings.ToLower(strings.TrimSpace(query))
	out := []store.ArcSummary{}
	for _, arc := range f.arcSummaries {
		if strings.TrimSpace(chatSessionID) != "" && arc.ChatSessionID != chatSessionID {
			continue
		}
		if fromTurn > 0 && arc.ToTurn > 0 && arc.ToTurn < fromTurn {
			continue
		}
		if toTurn > 0 && arc.FromTurn > toTurn {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(arc.ArcName+" "+arc.CoreConflict+" "+arc.ArcResumeText+" "+arc.KeyTurningPointsJSON+" "+arc.ActivePromisesJSON+" "+arc.UnresolvedDebtsJSON+" "+arc.CallbackCandidatesJSON+" "+arc.FuturePayoffCandidatesJSON+" "+arc.IrreversibleTurnsJSON+" "+arc.CallbackDebtsJSON+" "+arc.RelationshipPivotsJSON), query) {
			continue
		}
		out = append(out, arc)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (f *narrativeFakeStore) SaveSagaDigest(ctx context.Context, chatSessionID string, item *store.SagaDigest) error {
	if f.errNotEnabled {
		return store.ErrNotEnabled
	}
	if item.ChatSessionID == "" {
		item.ChatSessionID = chatSessionID
	}
	if item.ID == 0 {
		item.ID = int64(len(f.savedSagaDigests) + 1)
	}
	f.savedSagaDigests = append(f.savedSagaDigests, *item)
	f.sagaDigests = append(f.sagaDigests, *item)
	return nil
}

func (f *narrativeFakeStore) GetLatestSagaDigest(ctx context.Context, chatSessionID string) (*store.SagaDigest, error) {
	items, err := f.SearchSagaDigests(ctx, chatSessionID, "", 0, 0, 1)
	if err != nil || len(items) == 0 {
		return nil, err
	}
	return &items[0], nil
}

func (f *narrativeFakeStore) ListSagaDigests(ctx context.Context, chatSessionID string, limit int) ([]store.SagaDigest, error) {
	return f.SearchSagaDigests(ctx, chatSessionID, "", 0, 0, limit)
}

func (f *narrativeFakeStore) SearchSagaDigests(ctx context.Context, chatSessionID, query string, fromTurn, toTurn, limit int) ([]store.SagaDigest, error) {
	if f.errNotEnabled {
		return nil, store.ErrNotEnabled
	}
	if limit <= 0 {
		limit = 20
	}
	query = strings.ToLower(strings.TrimSpace(query))
	out := []store.SagaDigest{}
	for _, saga := range f.sagaDigests {
		if strings.TrimSpace(chatSessionID) != "" && saga.ChatSessionID != chatSessionID {
			continue
		}
		if fromTurn > 0 && saga.ToTurn > 0 && saga.ToTurn < fromTurn {
			continue
		}
		if toTurn > 0 && saga.FromTurn > toTurn {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(saga.EraLabel+" "+saga.SagaSummary+" "+saga.ResumePackText), query) {
			continue
		}
		out = append(out, saga)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (f *narrativeFakeStore) ListStorylines(ctx context.Context, chatSessionID string) ([]store.Storyline, error) {
	if f.errNotEnabled {
		return nil, store.ErrNotEnabled
	}
	return f.storylines, nil
}

func (f *narrativeFakeStore) SaveStoryline(ctx context.Context, s *store.Storyline) error {
	f.savedStorylines = append(f.savedStorylines, *s)
	if s.ID == 0 {
		s.ID = int64(len(f.storylines) + len(f.savedStorylines))
	}
	f.storylines = append(f.storylines, *s)
	return nil
}

func (f *narrativeFakeStore) PatchStoryline(ctx context.Context, storylineID int64, updates map[string]any) ([]string, error) {
	if f.storylineNotFound {
		return nil, store.ErrNotFound
	}
	f.storylinePatches = append(f.storylinePatches, updates)
	out := make([]string, 0, len(updates))
	for _, key := range []string{"name", "status", "entities_json", "current_context", "key_points_json", "ongoing_tensions_json", "confidence", "evidence_count", "last_evidence_turn", "first_turn", "last_turn"} {
		if _, ok := updates[key]; ok {
			out = append(out, key)
		}
	}
	return out, nil
}

func (f *narrativeFakeStore) PatchStorylineTrust(ctx context.Context, storylineID int64, updates map[string]any) ([]string, error) {
	if f.storylineNotFound {
		return nil, store.ErrNotFound
	}
	f.storylineTrustPatch = updates
	out := make([]string, 0, len(updates))
	for _, key := range []string{"pinned", "suppressed", "user_corrected"} {
		if _, ok := updates[key]; ok {
			out = append(out, key)
		}
	}
	return out, nil
}

func (f *narrativeFakeStore) DeleteStoryline(ctx context.Context, storylineID int64) error {
	if f.storylineNotFound {
		return store.ErrNotFound
	}
	f.deletedStorylineID = storylineID
	return nil
}

func (f *narrativeFakeStore) ListWorldRules(ctx context.Context, chatSessionID string) ([]store.WorldRule, error) {
	if f.errNotEnabled {
		return nil, store.ErrNotEnabled
	}
	return f.worldRules, nil
}

func (f *narrativeFakeStore) ListInheritedWorldRules(ctx context.Context, chatSessionID string, activeScope, scopeName string) ([]store.WorldRule, error) {
	if f.errNotEnabled {
		return nil, store.ErrNotEnabled
	}
	activeScope = strings.TrimSpace(activeScope)
	scopeName = strings.TrimSpace(scopeName)
	if activeScope == "" && f.activeScope != nil {
		activeScope = f.activeScope.ActiveScope
		if scopeName == "" {
			scopeName = f.activeScope.ScopeName
		}
	}
	if activeScope == "" {
		activeScope = "root"
	}
	chain := worldRuleScopeChain(activeScope)
	chainOrder := map[string]int{}
	for i, scope := range chain {
		chainOrder[scope] = i
	}
	out := []store.WorldRule{}
	for _, item := range f.worldRules {
		if item.ChatSessionID != chatSessionID || item.Suppressed {
			continue
		}
		if _, ok := chainOrder[item.Scope]; !ok {
			continue
		}
		if item.Scope == activeScope {
			if scopeName != "" && strings.TrimSpace(item.ScopeName) != scopeName {
				continue
			}
			if scopeName == "" && strings.TrimSpace(item.ScopeName) != "" {
				continue
			}
		}
		out = append(out, item)
	}
	sort.SliceStable(out, func(i, j int) bool {
		left, right := chainOrder[out[i].Scope], chainOrder[out[j].Scope]
		if left != right {
			return left < right
		}
		if out[i].Pinned != out[j].Pinned {
			return out[i].Pinned
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (f *narrativeFakeStore) GetActiveScope(ctx context.Context, chatSessionID string) (*store.SessionActiveScope, error) {
	if f.errNotEnabled {
		return nil, store.ErrNotEnabled
	}
	if f.activeScope == nil || f.activeScope.ChatSessionID != chatSessionID {
		return nil, store.ErrNotFound
	}
	cp := *f.activeScope
	return &cp, nil
}

func (f *narrativeFakeStore) GetGuidancePlanState(ctx context.Context, chatSessionID string) (*store.GuidancePlanState, error) {
	if f.errNotEnabled {
		return nil, store.ErrNotEnabled
	}
	if f.guidancePlanStateErr != nil {
		return nil, f.guidancePlanStateErr
	}
	if f.guidancePlanState == nil || f.guidancePlanState.ChatSessionID != chatSessionID {
		return nil, store.ErrNotFound
	}
	cp := *f.guidancePlanState
	return &cp, nil
}

func (f *narrativeFakeStore) UpsertGuidancePlanState(ctx context.Context, item *store.GuidancePlanState) error {
	if f.errNotEnabled {
		return store.ErrNotEnabled
	}
	if f.guidancePlanUpsertErr != nil {
		return f.guidancePlanUpsertErr
	}
	cp := *item
	f.savedGuidancePlanState = &cp
	f.guidancePlanState = &cp
	return nil
}

func (f *narrativeFakeStore) UpsertActiveScope(ctx context.Context, item *store.SessionActiveScope) error {
	if f.errNotEnabled {
		return store.ErrNotEnabled
	}
	cp := *item
	f.savedActiveScope = &cp
	f.activeScope = &cp
	return nil
}

func (f *narrativeFakeStore) SaveWorldRule(ctx context.Context, w *store.WorldRule) error {
	f.savedWorldRules = append(f.savedWorldRules, *w)
	if w.ID == 0 {
		w.ID = int64(len(f.worldRules) + len(f.savedWorldRules))
	}
	replaced := false
	for i, item := range f.worldRules {
		if item.ChatSessionID == w.ChatSessionID && item.Scope == w.Scope && item.Category == w.Category && item.Key == w.Key {
			f.worldRules[i] = *w
			replaced = true
			break
		}
	}
	if !replaced {
		f.worldRules = append(f.worldRules, *w)
	}
	return nil
}

func (f *narrativeFakeStore) PatchWorldRule(ctx context.Context, ruleID int64, updates map[string]any) ([]string, error) {
	f.worldRulePatches = append(f.worldRulePatches, updates)
	out := make([]string, 0, len(updates))
	for _, key := range []string{"scope", "scope_name", "category", "key", "value_json", "genre", "source_turn"} {
		if _, ok := updates[key]; ok {
			out = append(out, key)
		}
	}
	return out, nil
}

func (f *narrativeFakeStore) PatchWorldRuleTrust(ctx context.Context, ruleID int64, updates map[string]any) ([]string, error) {
	f.worldRuleTrustPatch = updates
	out := make([]string, 0, len(updates))
	for _, key := range []string{"pinned", "suppressed", "user_corrected"} {
		if _, ok := updates[key]; ok {
			out = append(out, key)
		}
	}
	return out, nil
}

func (f *narrativeFakeStore) DeleteWorldRule(ctx context.Context, ruleID int64) error {
	f.deletedWorldRuleID = ruleID
	return nil
}

func (f *narrativeFakeStore) UpdateMemoryExplorerFields(ctx context.Context, chatSessionID string, memoryID int64, patch store.MemoryExplorerPatch) error {
	return store.ErrNotFound
}

func (f *narrativeFakeStore) UpdateKGTripleExplorerFields(ctx context.Context, chatSessionID string, tripleID int64, patch store.KGTripleExplorerPatch) error {
	return store.ErrNotFound
}

func (f *narrativeFakeStore) UpdateDirectEvidenceExplorerFields(ctx context.Context, chatSessionID string, recordID int64, patch store.DirectEvidenceExplorerPatch) error {
	return store.ErrNotFound
}

func (f *narrativeFakeStore) DeleteMemoryByID(ctx context.Context, chatSessionID string, memoryID int64) error {
	return store.ErrNotFound
}

func (f *narrativeFakeStore) DeleteDirectEvidenceByID(ctx context.Context, chatSessionID string, recordID int64) error {
	return store.ErrNotFound
}

func (f *narrativeFakeStore) DeleteKGTripleByID(ctx context.Context, chatSessionID string, tripleID int64) error {
	return store.ErrNotFound
}

func (f *narrativeFakeStore) DeleteCharacterByName(ctx context.Context, chatSessionID string, characterName string) error {
	f.deletedCharacterName = characterName
	filteredStates := f.characterStates[:0]
	for _, item := range f.characterStates {
		if item.ChatSessionID == chatSessionID && item.CharacterName == characterName {
			continue
		}
		filteredStates = append(filteredStates, item)
	}
	f.characterStates = filteredStates
	filteredEvents := f.characterEvents[:0]
	for _, item := range f.characterEvents {
		if item.ChatSessionID == chatSessionID && item.CharacterName == characterName {
			continue
		}
		filteredEvents = append(filteredEvents, item)
	}
	f.characterEvents = filteredEvents
	if f.characterState != nil && f.characterState.ChatSessionID == chatSessionID && f.characterState.CharacterName == characterName {
		f.characterState = nil
	}
	return nil
}

func (f *narrativeFakeStore) ListCharacterStates(ctx context.Context, chatSessionID string) ([]store.CharacterState, error) {
	if f.errNotEnabled {
		return nil, store.ErrNotEnabled
	}
	return f.characterStates, nil
}

func (f *narrativeFakeStore) GetCharacterState(ctx context.Context, chatSessionID, characterName string) (*store.CharacterState, error) {
	if f.errNotEnabled {
		return nil, store.ErrNotEnabled
	}
	if f.characterNotFound {
		return nil, store.ErrNotFound
	}
	return f.characterState, nil
}

func (f *narrativeFakeStore) SaveCharacterState(ctx context.Context, c *store.CharacterState) error {
	f.savedCharacterStates = append(f.savedCharacterStates, *c)
	f.characterState = c
	replaced := false
	for i, item := range f.characterStates {
		if item.ChatSessionID == c.ChatSessionID && item.CharacterName == c.CharacterName {
			f.characterStates[i] = *c
			replaced = true
			break
		}
	}
	if !replaced {
		f.characterStates = append(f.characterStates, *c)
	}
	return nil
}

func (f *narrativeFakeStore) ListPendingThreads(ctx context.Context, chatSessionID, status string) ([]store.PendingThread, error) {
	if f.errNotEnabled {
		return nil, store.ErrNotEnabled
	}
	return f.pendingThreads, nil
}

func (f *narrativeFakeStore) PatchPendingThread(ctx context.Context, hookID int64, updates map[string]any) ([]string, error) {
	f.pendingThreadPatches = append(f.pendingThreadPatches, updates)
	return orderedUpdatedFields(updates, []string{"status", "thread_type", "title", "owner", "target", "confidence", "details_json", "resolution_note"}), nil
}

func (f *narrativeFakeStore) PatchPendingThreadTrust(ctx context.Context, hookID int64, updates map[string]any) ([]string, error) {
	f.pendingThreadTrust = updates
	return orderedUpdatedFields(updates, []string{"pinned", "suppressed", "user_corrected"}), nil
}

func (f *narrativeFakeStore) DeletePendingThread(ctx context.Context, hookID int64) error {
	f.deletedPendingThread = hookID
	return nil
}

func (f *narrativeFakeStore) ListActiveStates(ctx context.Context, chatSessionID, stateType string) ([]store.ActiveState, error) {
	if f.errNotEnabled {
		return nil, store.ErrNotEnabled
	}
	return f.activeStates, nil
}

func (f *narrativeFakeStore) ListCanonicalStateLayers(ctx context.Context, chatSessionID, layerType string) ([]store.CanonicalStateLayer, error) {
	if f.errNotEnabled {
		return nil, store.ErrNotEnabled
	}
	return f.canonicalStateLayers, nil
}

func (f *narrativeFakeStore) ListEpisodeSummaries(ctx context.Context, chatSessionID string, limit, fromTurn, toTurn int) ([]store.EpisodeSummary, error) {
	if f.errNotEnabled {
		return nil, store.ErrNotEnabled
	}
	out := make([]store.EpisodeSummary, 0, len(f.episodeSummaries))
	for _, item := range f.episodeSummaries {
		if chatSessionID != "" && item.ChatSessionID != "" && item.ChatSessionID != chatSessionID {
			continue
		}
		if fromTurn > 0 && item.FromTurn < fromTurn {
			continue
		}
		if toTurn > 0 && item.ToTurn > toTurn {
			continue
		}
		out = append(out, item)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (f *narrativeFakeStore) GetEpisodeSummary(ctx context.Context, episodeID int64) (*store.EpisodeSummary, error) {
	if f.errNotEnabled {
		return nil, store.ErrNotEnabled
	}
	if f.episodeNotFound {
		return nil, store.ErrNotFound
	}
	return f.episodeSummary, nil
}

func (f *narrativeFakeStore) SaveEpisodeSummary(ctx context.Context, item *store.EpisodeSummary) error {
	if f.errNotEnabled {
		return store.ErrNotEnabled
	}
	cp := *item
	if cp.ID == 0 {
		cp.ID = int64(len(f.savedEpisodeSummaries) + 1)
		item.ID = cp.ID
	}
	f.savedEpisodeSummaries = append(f.savedEpisodeSummaries, cp)
	f.episodeSummaries = append(f.episodeSummaries, cp)
	f.episodeSummary = &cp
	return nil
}

func (f *narrativeFakeStore) DeleteEpisodeSummary(ctx context.Context, episodeID int64) error {
	f.deletedEpisodeID = episodeID
	next := f.episodeSummaries[:0]
	deleted := false
	for _, item := range f.episodeSummaries {
		if item.ID == episodeID {
			deleted = true
			continue
		}
		next = append(next, item)
	}
	f.episodeSummaries = next
	if !deleted && f.episodeNotFound {
		return store.ErrNotFound
	}
	return nil
}

func (f *narrativeFakeStore) DeleteEpisodeSummariesInRange(ctx context.Context, chatSessionID string, fromTurn, toTurn int) (int64, error) {
	f.deletedEpisodeRanges = append(f.deletedEpisodeRanges, fmt.Sprintf("%s:%d:%d", chatSessionID, fromTurn, toTurn))
	next := f.episodeSummaries[:0]
	var deleted int64
	for _, item := range f.episodeSummaries {
		if chatSessionID != "" && item.ChatSessionID != chatSessionID {
			next = append(next, item)
			continue
		}
		if (fromTurn <= 0 || item.ToTurn >= fromTurn) && (toTurn <= 0 || item.FromTurn <= toTurn) {
			deleted++
			continue
		}
		next = append(next, item)
	}
	f.episodeSummaries = next
	return deleted, nil
}

func (f *narrativeFakeStore) DeleteChatLogs(ctx context.Context, sid string, fromTurn int) error {
	return nil
}

func (f *narrativeFakeStore) DeleteEffectiveInputs(ctx context.Context, sid string, fromTurn int) error {
	return nil
}

func (f *narrativeFakeStore) DeleteMemories(ctx context.Context, sid string, fromTurn int) error {
	return nil
}

func (f *narrativeFakeStore) DeleteEvidence(ctx context.Context, sid string, fromTurn int) error {
	return nil
}

func (f *narrativeFakeStore) DeleteKGTriples(ctx context.Context, sid string, fromTurn int) error {
	return nil
}

func (f *narrativeFakeStore) DeleteCriticFeedback(ctx context.Context, sid string, fromTurn int) error {
	return nil
}

func (f *narrativeFakeStore) DeleteCharacterEvents(ctx context.Context, sid string, fromTurn int) error {
	return nil
}

func (f *narrativeFakeStore) DeleteEntities(ctx context.Context, sid string, fromTurn int) error {
	return nil
}

func (f *narrativeFakeStore) DeleteTrustStates(ctx context.Context, sid string, fromTurn int) error {
	return nil
}

func (f *narrativeFakeStore) DeleteStorylines(ctx context.Context, sid string, fromTurn int) error {
	return nil
}

func (f *narrativeFakeStore) DeleteWorldRules(ctx context.Context, sid string, fromTurn int) error {
	return nil
}

func (f *narrativeFakeStore) DeleteCharacterStates(ctx context.Context, sid string, fromTurn int) error {
	return nil
}

func (f *narrativeFakeStore) DeletePendingThreads(ctx context.Context, sid string, fromTurn int) error {
	return nil
}

func (f *narrativeFakeStore) DeleteActiveStates(ctx context.Context, sid string, fromTurn int) error {
	return nil
}

func (f *narrativeFakeStore) DeleteCanonicalStateLayers(ctx context.Context, sid string, fromTurn int) error {
	return nil
}

func (f *narrativeFakeStore) DeleteEpisodeSummaries(ctx context.Context, sid string, fromTurn int) error {
	return nil
}

func (f *narrativeFakeStore) DeleteGuidancePlanState(ctx context.Context, sid string, fromTurn int) error {
	return nil
}

func (f *narrativeFakeStore) DeleteChapterSummaries(ctx context.Context, sid string, fromTurn int) error {
	return nil
}

func (f *narrativeFakeStore) DeleteArcSummaries(ctx context.Context, sid string, fromTurn int) error {
	return nil
}

func (f *narrativeFakeStore) DeleteSagaDigests(ctx context.Context, sid string, fromTurn int) error {
	return nil
}

func (f *narrativeFakeStore) DeleteSessionActiveScopes(ctx context.Context, sid string, fromTurn int) error {
	return nil
}

func (f *narrativeFakeStore) DeleteProtagonistEntityMemories(ctx context.Context, sid string, fromTurn int) error {
	return nil
}

func (f *narrativeFakeStore) DeleteConsequenceRecords(ctx context.Context, sid string, fromTurn int) error {
	return nil
}

func (f *narrativeFakeStore) DeletePsychologyBranches(ctx context.Context, sid string, fromTurn int) error {
	return nil
}

func (f *narrativeFakeStore) DeleteThemeOffscreenCarries(ctx context.Context, sid string, fromTurn int) error {
	return nil
}

func (f *narrativeFakeStore) DeleteCaptureVerificationRecords(ctx context.Context, sid string, fromTurn int) error {
	return nil
}

func (f *narrativeFakeStore) DeleteStatusCurrentValues(ctx context.Context, sid string, fromTurn int) error {
	return nil
}

func (f *narrativeFakeStore) DeleteStatusChangeEvents(ctx context.Context, sid string, fromTurn int) error {
	return nil
}

func (f *narrativeFakeStore) DeleteStatusEffects(ctx context.Context, sid string, fromTurn int) error {
	return nil
}

func (f *narrativeFakeStore) DeleteSession(ctx context.Context, sid string) error {
	f.deleteSessionCalled = true
	return f.deleteSessionErr
}

type narrativeAuditCounterStore struct {
	*narrativeFakeStore
	total int
}

func (f *narrativeAuditCounterStore) CountAuditLogs(ctx context.Context, chatSessionID string, eventType string) (int, error) {
	return f.total, nil
}

func TestNarrativeRoutesStoreBacked(t *testing.T) {
	fake := &narrativeFakeStore{
		storylines: []store.Storyline{
			{ID: 1, ChatSessionID: "sess-1", Name: "Arc 1", Status: "active"},
		},
		worldRules: []store.WorldRule{
			{ID: 1, ChatSessionID: "sess-1", Scope: "root", Category: "magic", Key: "mana"},
		},
		characterStates: []store.CharacterState{
			{ID: 1, ChatSessionID: "sess-1", CharacterName: "Alice", TurnIndex: 5},
		},
		characterState: &store.CharacterState{ID: 2, ChatSessionID: "sess-1", CharacterName: "Bob", TurnIndex: 6},
		characterEvents: []store.CharacterEvent{
			{ID: 1, ChatSessionID: "sess-1", CharacterName: "Alice", EventType: "mood_shift"},
		},
		pendingThreads: []store.PendingThread{
			{ID: 1, ChatSessionID: "sess-1", ThreadKey: "hook-1", Status: "open"},
		},
		activeStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "sess-1", StateType: "scene_state", Content: `{"loc":"temple"}`, TurnIndex: 7},
		},
		canonicalStateLayers: []store.CanonicalStateLayer{
			{ID: 1, ChatSessionID: "sess-1", LayerType: "scene_state", Content: `{"loc":"temple"}`, TurnIndex: 7},
		},
		episodeSummaries: []store.EpisodeSummary{
			{ID: 1, ChatSessionID: "sess-1", FromTurn: 1, ToTurn: 10, SummaryText: "Alice arrives."},
		},
		episodeSummary: &store.EpisodeSummary{ID: 2, ChatSessionID: "sess-1", FromTurn: 11, ToTurn: 20, SummaryText: "Bob joins."},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	tests := []struct {
		path       string
		wantCode   int
		wantCount  int
		countField string
	}{
		{path: "/storylines/sess-1", wantCode: http.StatusOK, wantCount: 1, countField: "count"},
		{path: "/world-rules/sess-1", wantCode: http.StatusOK, wantCount: 1, countField: "count"},
		{path: "/world-rules/sess-1/inherited", wantCode: http.StatusOK, wantCount: 1, countField: "count"},
		{path: "/characters/sess-1", wantCode: http.StatusOK, wantCount: 1, countField: "count"},
		{path: "/characters/sess-1/Alice", wantCode: http.StatusOK, wantCount: 0, countField: ""},
		{path: "/characters/sess-1/Alice/events", wantCode: http.StatusOK, wantCount: 1, countField: "total"},
		{path: "/pending-threads/sess-1", wantCode: http.StatusOK, wantCount: 1, countField: "count"},
		{path: "/active-states/sess-1", wantCode: http.StatusOK, wantCount: 1, countField: "count"},
		{path: "/canonical-state-layer/sess-1", wantCode: http.StatusOK, wantCount: 1, countField: "count"},
		{path: "/episodes/sess-1", wantCode: http.StatusOK, wantCount: 1, countField: "count"},
		{path: "/episodes/detail/2", wantCode: http.StatusOK, wantCount: 0, countField: ""},
		{path: "/session-state/sess-1", wantCode: http.StatusOK, wantCount: 0, countField: ""},
		{path: "/continuity-pack/sess-1", wantCode: http.StatusOK, wantCount: 0, countField: ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tt.wantCode, rec.Body.String())
			}
			var resp map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if resp["status"] != "ok" {
				t.Errorf("status = %v, want ok", resp["status"])
			}
			if tt.countField != "" {
				if resp[tt.countField] != float64(tt.wantCount) {
					t.Errorf("%s = %v, want %d", tt.countField, resp[tt.countField], tt.wantCount)
				}
			}
		})
	}
}

func TestSessionsListEmptyStoreShape(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = &narrativeFakeStore{}
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/sessions", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "ok" || resp["count"] != float64(0) {
		t.Fatalf("unexpected sessions response: %#v", resp)
	}
	items, ok := resp["sessions"].([]any)
	if !ok || len(items) != 0 {
		t.Fatalf("sessions = %#v, want empty list", resp["sessions"])
	}
}

func TestSessionExportWritesAuditLog(t *testing.T) {
	fake := &narrativeFakeStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-export", TurnIndex: 1, Role: "user", Content: "hello"},
		},
		memories: []store.Memory{
			{ID: 2, ChatSessionID: "sess-export", TurnIndex: 1, SummaryJSON: `{"summary":"hello"}`},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/sessions/sess-export/export", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.auditLogs) == 0 {
		t.Fatal("expected export audit log")
	}
	audit := fake.auditLogs[0]
	if audit.EventType != "export" {
		t.Fatalf("event_type = %q, want export", audit.EventType)
	}
	if audit.ChatSessionID != "sess-export" {
		t.Fatalf("chat_session_id = %q, want sess-export", audit.ChatSessionID)
	}
	if audit.TargetType != "session" {
		t.Fatalf("target_type = %q, want session", audit.TargetType)
	}
	if !strings.Contains(audit.DetailsJSON, "chat_logs_count") {
		t.Fatalf("details_json missing export summary counts: %s", audit.DetailsJSON)
	}
}

func TestAuditGetUsesCounterTotalBeyondLimit(t *testing.T) {
	fake := &narrativeAuditCounterStore{
		narrativeFakeStore: &narrativeFakeStore{
			auditLogs: []store.AuditLog{
				{ID: 3, ChatSessionID: "sess-1", EventType: "memory_write"},
				{ID: 2, ChatSessionID: "sess-1", EventType: "memory_write"},
			},
		},
		total: 113,
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/audit?chat_session_id=sess-1&event_type=memory_write&limit=1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["total"] != float64(113) {
		t.Fatalf("total = %v, want 113", resp["total"])
	}
	items, ok := resp["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("items len = %d, want 1: %#v", len(items), resp["items"])
	}
}

func TestAuditGetFallbackTotalUsesReturnedItems(t *testing.T) {
	fake := &narrativeFakeStore{
		auditLogs: []store.AuditLog{
			{ID: 2, ChatSessionID: "sess-1", EventType: "memory_write"},
			{ID: 1, ChatSessionID: "sess-1", EventType: "memory_write"},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/audit?chat_session_id=sess-1&event_type=memory_write&limit=1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["total"] != float64(1) {
		t.Fatalf("fallback total = %v, want 1", resp["total"])
	}
}

func TestFeedbackPostAndLatestUseCanonicalStoreShape(t *testing.T) {
	fake := &narrativeFakeStore{
		memories: []store.Memory{{ID: 42, ChatSessionID: "sess-1", SummaryJSON: `{"summary":"ok"}`}},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/feedback", strings.NewReader(`{"chat_session_id":"sess-1","target_type":"memory","target_id":42,"feedback_value":"up","feedback_note":"keep"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /feedback status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var postResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &postResp); err != nil {
		t.Fatalf("decode post: %v", err)
	}
	if postResp["status"] != "ok" || postResp["feedback_value"] != "up" {
		t.Fatalf("unexpected post response: %#v", postResp)
	}

	latestReq := httptest.NewRequest(http.MethodGet, "/feedback/latest?chat_session_id=sess-1&target_type=memory&target_ids=42", nil)
	latestRec := httptest.NewRecorder()
	mux.ServeHTTP(latestRec, latestReq)
	if latestRec.Code != http.StatusOK {
		t.Fatalf("GET /feedback/latest status = %d, want 200: %s", latestRec.Code, latestRec.Body.String())
	}
	var latestResp map[string]any
	if err := json.Unmarshal(latestRec.Body.Bytes(), &latestResp); err != nil {
		t.Fatalf("decode latest: %v", err)
	}
	feedbacks, ok := latestResp["feedbacks"].(map[string]any)
	if !ok {
		t.Fatalf("feedbacks missing: %#v", latestResp)
	}
	item, ok := feedbacks["42"].(map[string]any)
	if !ok || item["feedback_value"] != "up" || item["feedback_note"] != "keep" {
		t.Fatalf("unexpected latest feedbacks: %#v", feedbacks)
	}

	wrongSessionReq := httptest.NewRequest(http.MethodPost, "/feedback", strings.NewReader(`{"chat_session_id":"other","target_type":"memory","target_id":42,"feedback_value":"down"}`))
	wrongSessionReq.Header.Set("Content-Type", "application/json")
	wrongSessionRec := httptest.NewRecorder()
	mux.ServeHTTP(wrongSessionRec, wrongSessionReq)
	if wrongSessionRec.Code != http.StatusBadRequest {
		t.Fatalf("wrong-session feedback status = %d, want 400: %s", wrongSessionRec.Code, wrongSessionRec.Body.String())
	}
}
