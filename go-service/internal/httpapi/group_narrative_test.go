package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/vector"

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

// RollbackStore stubs for narrativeFakeStore.

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

func TestCharacterAndEpisodeNotFoundPythonShape(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		store      *narrativeFakeStore
		wantDetail string
	}{
		{
			name:       "character",
			path:       "/characters/sess-1/Alice",
			store:      &narrativeFakeStore{characterNotFound: true},
			wantDetail: "character not found: Alice",
		},
		{
			name:       "episode",
			path:       "/episodes/detail/999999",
			store:      &narrativeFakeStore{episodeNotFound: true},
			wantDetail: "episode not found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			srv := setupTestServer()
			srv.Store = tt.store
			srv.RegisterRoutes(mux)

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404: %s", rec.Code, rec.Body.String())
			}
			var resp map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if resp["status"] != "error" || resp["detail"] != tt.wantDetail {
				t.Fatalf("unexpected not-found response: %#v", resp)
			}
			if _, ok := resp["found"]; ok {
				t.Fatalf("unexpected found key in not-found response: %#v", resp)
			}
		})
	}
}

func TestNarrativeReadBehaviorMatchesPythonReferenceShape(t *testing.T) {
	fake := &narrativeFakeStore{
		storylines: []store.Storyline{
			{
				ID:               1,
				ChatSessionID:    "sess-1",
				Name:             "Gate pressure",
				Status:           "active",
				EvidenceCount:    1,
				LastEvidenceTurn: 5,
				LastTurn:         5,
			},
		},
		worldRules: []store.WorldRule{
			{ID: 1, ChatSessionID: "sess-1", Scope: "location", ScopeName: "Archive", Category: "access", Key: "sealed"},
			{ID: 2, ChatSessionID: "sess-1", Scope: "root", Category: "physics", Key: "gravity"},
		},
		episodeSummaries: []store.EpisodeSummary{
			{ID: 1, ChatSessionID: "sess-1", FromTurn: 1, ToTurn: 20, SummaryText: "Alice studies the sealed archive gate.", KeyEntities: "Alice"},
			{ID: 2, ChatSessionID: "sess-1", FromTurn: 21, ToTurn: 60, SummaryText: "The gate pressure rises.", KeyEvents: "pressure"},
		},
		resumePack: &store.ResumePack{
			PackStatus: "ok",
			Chapter: &store.ChapterSummary{
				ID:           7,
				FromTurn:     1,
				ToTurn:       60,
				ChapterTitle: "Archive Gate",
				SummaryText:  "Alice studies the sealed archive gate.",
			},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	t.Run("storylines include reference and stale fields", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/storylines/sess-1?current_turn=8", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp["reference_turn"] != float64(8) {
			t.Fatalf("reference_turn = %v, want 8", resp["reference_turn"])
		}
		items := resp["storylines"].([]any)
		first := items[0].(map[string]any)
		if first["last_observed_turn"] != float64(5) || first["freshness_turn_gap"] != float64(3) {
			t.Fatalf("stale snapshot = %#v", first)
		}
		if first["is_stale"] != true || first["stale_reason"] != "low_evidence_gap" {
			t.Fatalf("stale fields = %#v", first)
		}
	})

	t.Run("world rules inherited expose Python-compatible rules and scope chain", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/world-rules/sess-1/inherited?active_scope=location&scope_name=Archive", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp["active_scope"] != "location" || resp["scope_name"] != "Archive" {
			t.Fatalf("scope fields = %#v", resp)
		}
		chain := resp["scope_chain"].([]any)
		if len(chain) != 4 || chain[0] != "location" || chain[1] != "region" || chain[2] != "root" || chain[3] != "session" {
			t.Fatalf("scope_chain = %#v", chain)
		}
		rules := resp["rules"].([]any)
		if len(rules) != 2 {
			t.Fatalf("rules len = %d, want 2", len(rules))
		}
	})

	t.Run("chapter dry run exposes interval preview fields", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/chapters/dry-run", strings.NewReader(`{"chat_session_id":"sess-1","turn_index":60,"interval":60,"top_k":8}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp["mode"] != "dry_run" || resp["triggered"] != true {
			t.Fatalf("dry-run fields = %#v", resp)
		}
		candidate := resp["candidate_range"].(map[string]any)
		if candidate["from_turn"] != float64(1) || candidate["to_turn"] != float64(60) {
			t.Fatalf("candidate_range = %#v", candidate)
		}
	})

	t.Run("search responses keep 0.8 aliases", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/episodes/search", strings.NewReader(`{"chat_session_id":"sess-1","query":"Alice","top_k":1}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp["query"] != "Alice" || resp["count"] != float64(1) {
			t.Fatalf("search envelope = %#v", resp)
		}
		if _, ok := resp["episodes"].([]any); !ok {
			t.Fatalf("missing episodes alias: %#v", resp)
		}
	})
}

func TestStorylineRegistryWriteRoutes(t *testing.T) {
	fake := &narrativeFakeStore{
		storylines: []store.Storyline{
			{ID: 7, ChatSessionID: "sess-1", Name: "Old Arc", Status: "active", EvidenceCount: 1, LastEvidenceTurn: 3, FirstTurn: 2, LastTurn: 3},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	t.Run("patch storyline updates allowed fields", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/storylines/7", strings.NewReader(`{"status":"paused","key_points_json":["beat","beat"],"ongoing_tensions_json":["answer","answer"],"confidence":0.75,"evidence_count":3,"last_evidence_turn":9}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		if len(fake.storylinePatches) != 1 {
			t.Fatalf("patch calls = %d, want 1", len(fake.storylinePatches))
		}
		if fake.storylinePatches[0]["key_points_json"] != `["beat"]` {
			t.Fatalf("deduped key_points_json = %#v", fake.storylinePatches[0]["key_points_json"])
		}
		if fake.storylinePatches[0]["ongoing_tensions_json"] != `["answer"]` {
			t.Fatalf("deduped ongoing_tensions_json = %#v", fake.storylinePatches[0]["ongoing_tensions_json"])
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode patch response: %v", err)
		}
		if resp["confidence"] != float64(0.75) || resp["evidence_count"] != float64(3) || resp["last_evidence_turn"] != float64(9) {
			t.Fatalf("quality fields missing from patch response: %#v", resp)
		}
		updatedValues, _ := resp["updated_values"].(map[string]any)
		if updatedValues["confidence"] != float64(0.75) || updatedValues["last_evidence_turn"] != float64(9) {
			t.Fatalf("updated_values missing quality fields: %#v", updatedValues)
		}
	})

	t.Run("patch storyline rejects invalid quality fields", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/storylines/7", strings.NewReader(`{"confidence":1.2}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("confidence status = %d, want 400: %s", rec.Code, rec.Body.String())
		}

		req = httptest.NewRequest(http.MethodPatch, "/storylines/7", strings.NewReader(`{"evidence_count":-1}`))
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("evidence_count status = %d, want 400: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("trust patch updates flags", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/storylines/7/trust", strings.NewReader(`{"pinned":true,"suppressed":false}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		if fake.storylineTrustPatch["pinned"] != true || fake.storylineTrustPatch["suppressed"] != false {
			t.Fatalf("trust patch = %#v", fake.storylineTrustPatch)
		}
	})

	t.Run("delete storyline uses live store", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/storylines/7", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		if fake.deletedStorylineID != 7 {
			t.Fatalf("deleted id = %d, want 7", fake.deletedStorylineID)
		}
	})
}

func TestPendingThreadContinuityHookRoutes(t *testing.T) {
	fake := &narrativeFakeStore{
		pendingThreads: []store.PendingThread{
			{
				ID:               11,
				ChatSessionID:    "sess-1",
				ThreadKey:        "thread_rooftop_promise",
				Description:      "Mira answers the rooftop promise",
				Status:           "open",
				SourceTurn:       4,
				HookType:         "promise",
				HookMetadataJSON: `{"title":"Mira answers the rooftop promise","owner":"Nia","target":"Mira","confidence":0.82,"last_seen_turn":7}`,
			},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	t.Run("continuity-hooks alias returns pending thread items", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/continuity-hooks/sess-1", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp["fetched"] != true || resp["count"] != float64(1) {
			t.Fatalf("continuity hook envelope = %#v", resp)
		}
		items := resp["items"].([]any)
		first := items[0].(map[string]any)
		if first["title"] != "Mira answers the rooftop promise" || first["owner"] != "Nia" || first["target"] != "Mira" {
			t.Fatalf("metadata-derived fields missing: %#v", first)
		}
		if first["confidence"] != float64(0.82) || first["last_seen_turn"] != float64(7) {
			t.Fatalf("metadata-derived quality fields missing: %#v", first)
		}
	})

	t.Run("patch validates and forwards allowed fields", func(t *testing.T) {
		body := `{"status":"paused","thread_type":"open_question","title":"Ask Mira why she hesitated","owner":"Nia","target":"Mira","confidence":0.74,"details_json":{"reason":"follow-up"},"resolution_note":"waiting"}`
		req := httptest.NewRequest(http.MethodPatch, "/continuity-hooks/11", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		if len(fake.pendingThreadPatches) != 1 {
			t.Fatalf("patch calls = %d, want 1", len(fake.pendingThreadPatches))
		}
		if fake.pendingThreadPatches[0]["thread_type"] != "open_question" || fake.pendingThreadPatches[0]["confidence"] != float64(0.74) {
			t.Fatalf("patch payload = %#v", fake.pendingThreadPatches[0])
		}
		if fake.pendingThreadPatches[0]["details_json"] != `{"reason":"follow-up"}` {
			t.Fatalf("details_json = %#v", fake.pendingThreadPatches[0]["details_json"])
		}
	})

	t.Run("patch rejects invalid thread type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/pending-threads/11", strings.NewReader(`{"thread_type":"misc"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("trust patch and delete use live store", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/pending-threads/11/trust", strings.NewReader(`{"pinned":true,"suppressed":false}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("trust status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		if fake.pendingThreadTrust["pinned"] != true || fake.pendingThreadTrust["suppressed"] != false {
			t.Fatalf("trust payload = %#v", fake.pendingThreadTrust)
		}

		req = httptest.NewRequest(http.MethodDelete, "/pending-threads/11", nil)
		rec = httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("delete status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		if fake.deletedPendingThread != 11 {
			t.Fatalf("deleted id = %d, want 11", fake.deletedPendingThread)
		}
	})
}

func TestSessionStateAggregateReadBuildsCoreFiveSections(t *testing.T) {
	now := time.Date(2026, 5, 31, 10, 0, 0, 0, time.UTC)
	fake := &narrativeFakeStore{
		activeStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "sess-1", StateType: "scene_state", Content: `{"mood":"tense"}`, TurnIndex: 9, CreatedAt: now},
		},
		canonicalStateLayers: []store.CanonicalStateLayer{
			{ID: 2, ChatSessionID: "sess-1", LayerType: "scene", Content: `{"mood":"tense"}`, TurnIndex: 9, CreatedAt: now},
		},
		storylines: []store.Storyline{
			{ID: 3, ChatSessionID: "sess-1", Name: "Rooftop Promise", Status: "active", CurrentContext: "Mira owes Nia an answer.", KeyPointsJSON: `["promise"]`, OngoingTensionsJSON: `["answer pending"]`, Confidence: 0.8, EvidenceCount: 2, LastEvidenceTurn: 8, FirstTurn: 4, LastTurn: 8, UpdatedAt: now},
			{ID: 4, ChatSessionID: "sess-1", Name: "Suppressed Arc", Status: "active", LastTurn: 9, Suppressed: true, UpdatedAt: now},
		},
		characterStates: []store.CharacterState{
			{ID: 5, ChatSessionID: "sess-1", CharacterName: "Mira", StatusJSON: `{"emotion":"conflicted"}`, TurnIndex: 8, CreatedAt: now, UpdatedAt: now},
		},
		worldRules: []store.WorldRule{
			{ID: 6, ChatSessionID: "sess-1", Scope: "session", Category: "promise", Key: "answers_need_followup", ValueJSON: `{"rule":"Do not drop promises."}`, SourceTurn: 7, UpdatedAt: now},
			{ID: 7, ChatSessionID: "sess-1", Scope: "session", Category: "hidden", Key: "suppressed", ValueJSON: `{}`, SourceTurn: 9, Suppressed: true, UpdatedAt: now},
		},
		pendingThreads: []store.PendingThread{
			{ID: 8, ChatSessionID: "sess-1", ThreadKey: "thread_rooftop", Description: "Mira answers Nia", Status: "open", SourceTurn: 6, LastSeenTurn: 9, HookType: "promise", HookMetadataJSON: `{"title":"Mira answers Nia","owner":"Nia","target":"Mira","confidence":0.9}`, UpdatedAt: now},
			{ID: 9, ChatSessionID: "sess-1", ThreadKey: "thread_suppressed", Description: "Hidden", Status: "open", SourceTurn: 8, Suppressed: true, UpdatedAt: now},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/session-state/sess-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if fake.aggregateReadCalls != 1 {
		t.Fatalf("aggregate snapshot calls = %d, want 1", fake.aggregateReadCalls)
	}
	if fake.listChatLogCalls != 0 {
		t.Fatalf("ListChatLogs fallback calls = %d, want 0 when aggregate snapshot provides recent logs", fake.listChatLogCalls)
	}
	if resp["snapshot_status"] != "ready" {
		t.Fatalf("snapshot_status = %v, want ready: %#v", resp["snapshot_status"], resp)
	}
	if len(resp["storylines"].([]any)) != 1 || len(resp["world_rules"].([]any)) != 1 || len(resp["pending_threads"].([]any)) != 1 {
		t.Fatalf("suppressed rows were not filtered: story=%#v world=%#v threads=%#v", resp["storylines"], resp["world_rules"], resp["pending_threads"])
	}
	if _, ok := resp["continuity_hooks"].([]any); !ok {
		t.Fatalf("continuity_hooks alias missing: %#v", resp)
	}
	meta := resp["section_meta"].(map[string]any)
	for _, key := range []string{"active_states", "storylines", "characters", "world_rules", "pending_threads", "continuity_hooks", "canonical_state_layer"} {
		m, ok := meta[key].(map[string]any)
		if !ok {
			t.Fatalf("meta %s missing: %#v", key, meta)
		}
		if m["ready"] != true || m["count"] != float64(1) {
			t.Fatalf("meta %s = %#v, want ready count=1", key, m)
		}
		if _, exists := m["last_turn"]; !exists {
			t.Fatalf("meta %s missing last_turn: %#v", key, m)
		}
		if _, exists := m["updated_at"]; !exists {
			t.Fatalf("meta %s missing updated_at: %#v", key, m)
		}
	}
}

func TestMomentumPacketBuildsStorylineHookRules(t *testing.T) {
	now := time.Date(2026, 5, 31, 11, 0, 0, 0, time.UTC)
	fake := &narrativeFakeStore{
		activeStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "sess-1", StateType: "scene_state", Content: `{"mood":"tense"}`, TurnIndex: 14, CreatedAt: now},
		},
		storylines: []store.Storyline{
			{ID: 10, ChatSessionID: "sess-1", Name: "Confession Aftermath", Status: "active", CurrentContext: "Mira still owes Nia an answer.", KeyPointsJSON: `["rooftop hesitation","rooftop hesitation","answer promised"]`, OngoingTensionsJSON: `["answer the confession"]`, Confidence: 0.85, EvidenceCount: 4, FirstTurn: 4, LastTurn: 14, UpdatedAt: now},
			{ID: 12, ChatSessionID: "sess-1", Name: "Stairwell Echo", Status: "active", CurrentContext: "The same hesitation keeps returning.", KeyPointsJSON: `["rooftop hesitation","hand on railing"]`, OngoingTensionsJSON: `["admit the fear"]`, Confidence: 0.72, EvidenceCount: 3, FirstTurn: 5, LastTurn: 13, UpdatedAt: now},
			{ID: 11, ChatSessionID: "sess-1", Name: "Suppressed", Status: "active", KeyPointsJSON: `["hidden"]`, OngoingTensionsJSON: `["hidden"]`, Suppressed: true, LastTurn: 15, UpdatedAt: now},
		},
		pendingThreads: []store.PendingThread{
			{ID: 21, ChatSessionID: "sess-1", ThreadKey: "thread_old", Description: "Ask why Mira paused", Status: "open", SourceTurn: 6, LastSeenTurn: 6, Priority: 2, HookType: "open_question", HookMetadataJSON: `{"title":"Ask why Mira paused"}`, UpdatedAt: now},
			{ID: 22, ChatSessionID: "sess-1", ThreadKey: "thread_new", Description: "Follow the rooftop answer", Status: "paused", SourceTurn: 13, LastSeenTurn: 13, Priority: 1, HookType: "promise", HookMetadataJSON: `{"title":"Follow the rooftop answer"}`, UpdatedAt: now},
		},
		characterStates: []store.CharacterState{
			{ID: 31, ChatSessionID: "sess-1", CharacterName: "Mira", RelationshipsJSON: `{"Nia":{"summary":"Mira owes Nia a direct answer after the confession.","trust":0.73}}`, TurnIndex: 14, UpdatedAt: now},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/momentum-packet/sess-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["packet_status"] != "ready" {
		t.Fatalf("packet_status = %v, want ready: %#v", resp["packet_status"], resp)
	}
	for _, key := range []string{"next_pressure", "payoff_candidates", "tension_to_reuse", "beats_to_avoid"} {
		items, ok := resp[key].([]any)
		if !ok || len(items) == 0 {
			t.Fatalf("%s missing generated items: %#v", key, resp[key])
		}
		first := items[0].(map[string]any)
		for _, field := range []string{"label", "source_type", "source_id", "source_name", "priority"} {
			if _, exists := first[field]; !exists {
				t.Fatalf("%s first item missing %s: %#v", key, field, first)
			}
		}
	}
	payoffItems, _ := resp["payoff_candidates"].([]any)
	sourceTypes := map[string]bool{}
	for _, raw := range payoffItems {
		item, _ := raw.(map[string]any)
		sourceTypes[fmt.Sprint(item["source_type"])] = true
	}
	for _, want := range []string{"storyline", "relationship", "pending_thread"} {
		if !sourceTypes[want] {
			t.Fatalf("payoff_candidates missing %s source type: %#v", want, payoffItems)
		}
	}
}

func TestStorylineRegistrySyncDryRunAndApply(t *testing.T) {
	fake := &narrativeFakeStore{
		storylines: []store.Storyline{
			{ID: 3, ChatSessionID: "sess-1", Name: "Rooftop Promise", Status: "active", EvidenceCount: 1, LastEvidenceTurn: 2, FirstTurn: 1, LastTurn: 2},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	dryBody := `{"chat_session_id":"sess-1","mode":"dry_run","supervisor_result":{"storylines":[{"name":"Rooftop Promise","status":"active","key_points":["confession","confession"],"ongoing_tensions":["answer pending","answer pending"],"confidence":0.8,"evidence_count":2,"last_evidence_turn":5}]}}`
	req := httptest.NewRequest(http.MethodPost, "/storylines/sync", strings.NewReader(dryBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("dry-run status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedStorylines) != 0 {
		t.Fatalf("dry-run saved storylines = %d, want 0", len(fake.savedStorylines))
	}
	var dryResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &dryResp); err != nil {
		t.Fatalf("decode dry-run: %v", err)
	}
	if dryResp["mode"] != "dry_run" || dryResp["valid_count"] != float64(1) {
		t.Fatalf("dry-run response = %#v", dryResp)
	}
	candidates := dryResp["candidates"].([]any)
	candidate := candidates[0].(map[string]any)
	if candidate["key_points_json"] != `["confession"]` || candidate["ongoing_tensions_json"] != `["answer pending"]` {
		t.Fatalf("dry-run candidate did not normalize lists: %#v", candidate)
	}
	if candidate["confidence"] != float64(0.8) || candidate["evidence_count"] != float64(2) || candidate["last_evidence_turn"] != float64(5) {
		t.Fatalf("dry-run candidate missing quality fields: %#v", candidate)
	}

	applyBody := `{"chat_session_id":"sess-1","mode":"apply","turn_index":4,"supervisor_result":{"storylines":[{"name":"Rooftop Promise","status":"active","current_context":"She waits for the answer.","key_points":["confession"],"ongoing_tensions":["answer pending"],"confidence":0.8}]}}`
	req = httptest.NewRequest(http.MethodPost, "/storylines/sync", strings.NewReader(applyBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("apply status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedStorylines) != 1 {
		t.Fatalf("apply saved storylines = %d, want 1", len(fake.savedStorylines))
	}
	saved := fake.savedStorylines[0]
	if saved.EvidenceCount != 2 || saved.LastEvidenceTurn != 4 || saved.FirstTurn != 1 || saved.LastTurn != 4 {
		t.Fatalf("saved storyline evidence/turns = %#v", saved)
	}
	var applyResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &applyResp); err != nil {
		t.Fatalf("decode apply response: %v", err)
	}
	results := applyResp["results"].([]any)
	result := results[0].(map[string]any)
	if result["confidence"] != float64(0.8) || result["evidence_count"] != float64(2) || result["last_evidence_turn"] != float64(4) {
		t.Fatalf("apply result missing quality fields: %#v", result)
	}

	duplicateBody := `{"chat_session_id":"sess-1","mode":"apply","turn_index":4,"supervisor_result":{"storylines":[{"name":"Rooftop Promise","status":"active","current_context":"She still waits.","last_evidence_turn":4}]}}`
	req = httptest.NewRequest(http.MethodPost, "/storylines/sync", strings.NewReader(duplicateBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("duplicate status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedStorylines) != 2 {
		t.Fatalf("duplicate saved storylines = %d, want 2", len(fake.savedStorylines))
	}
	duplicateSaved := fake.savedStorylines[1]
	if duplicateSaved.EvidenceCount != 2 || duplicateSaved.LastEvidenceTurn != 4 {
		t.Fatalf("duplicate sync double-counted evidence: %#v", duplicateSaved)
	}

	explicitBody := `{"chat_session_id":"sess-1","mode":"apply","turn_index":6,"supervisor_result":{"storylines":[{"name":"Rooftop Promise","status":"active","current_context":"The answer lands.","evidence_count":3,"last_evidence_turn":6}]}}`
	req = httptest.NewRequest(http.MethodPost, "/storylines/sync", strings.NewReader(explicitBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("explicit status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedStorylines) != 3 {
		t.Fatalf("explicit saved storylines = %d, want 3", len(fake.savedStorylines))
	}
	explicitSaved := fake.savedStorylines[2]
	if explicitSaved.EvidenceCount != 5 || explicitSaved.LastEvidenceTurn != 6 {
		t.Fatalf("explicit quality fields were not applied as increment/current turn: %#v", explicitSaved)
	}
}

func TestStorylineQualityGateFiveTurnReplayEvidenceAndSelection(t *testing.T) {
	const sid = "sess-h2-five-turn"
	fake := &narrativeFakeStore{}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	var current *store.Storyline
	postSync := func(turn int) store.Storyline {
		if current != nil {
			fake.storylines = []store.Storyline{*current}
		}
		body := fmt.Sprintf(`{"chat_session_id":%q,"mode":"apply","turn_index":%d,"supervisor_result":{"storylines":[{"name":"Rooftop Promise","status":"active","current_context":"Turn %d keeps the same promise active.","key_points":["shared vow","shared vow","turn %d evidence"],"ongoing_tensions":["answer pending","answer pending"],"confidence":0.8}]}}`, sid, turn, turn, turn)
		req := httptest.NewRequest(http.MethodPost, "/storylines/sync", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("sync turn %d status = %d, want 200: %s", turn, rec.Code, rec.Body.String())
		}
		if len(fake.savedStorylines) == 0 {
			t.Fatalf("sync turn %d saved no storyline", turn)
		}
		saved := fake.savedStorylines[len(fake.savedStorylines)-1]
		current = &saved
		return saved
	}

	for turn := 1; turn <= 5; turn++ {
		saved := postSync(turn)
		if saved.LastEvidenceTurn != turn || saved.LastTurn != turn {
			t.Fatalf("turn %d saved turns = %#v", turn, saved)
		}
	}
	if current == nil || current.EvidenceCount != 5 || current.LastEvidenceTurn != 5 {
		t.Fatalf("five-turn evidence accumulation = %#v, want evidence=5 last_evidence_turn=5", current)
	}

	duplicateBefore := *current
	duplicate := postSync(5)
	if duplicate.EvidenceCount != duplicateBefore.EvidenceCount || duplicate.LastEvidenceTurn != duplicateBefore.LastEvidenceTurn {
		t.Fatalf("duplicate same-turn sync double-counted evidence: before=%#v after=%#v", duplicateBefore, duplicate)
	}
	current = &duplicateBefore

	fake.storylines = []store.Storyline{
		*current,
		{
			ID:               99,
			ChatSessionID:    sid,
			Name:             "Stale High",
			Status:           "active",
			CurrentContext:   "stale high should not guide",
			Confidence:       0.99,
			EvidenceCount:    1,
			LastEvidenceTurn: 1,
			LastTurn:         5,
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/storylines/"+sid, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("storylines status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var storyResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &storyResp); err != nil {
		t.Fatalf("decode storylines: %v", err)
	}
	rows, _ := storyResp["storylines"].([]any)
	if len(rows) != 2 {
		t.Fatalf("storylines rows = %#v, want fresh and stale", storyResp)
	}
	fresh := rows[0].(map[string]any)
	if fresh["evidence_count"] != float64(5) || fresh["last_evidence_turn"] != float64(5) || fresh["last_observed_turn"] != float64(5) || fresh["freshness_turn_gap"] != float64(0) {
		t.Fatalf("fresh storyline quality/freshness fields = %#v", fresh)
	}
	if strings.Count(fmt.Sprint(fresh["key_points_json"]), "shared vow") != 1 || strings.Count(fmt.Sprint(fresh["ongoing_tensions_json"]), "answer pending") != 1 {
		t.Fatalf("fresh storyline read path did not dedupe key/tension lists: %#v", fresh)
	}

	req = httptest.NewRequest(http.MethodPost, "/supervisor", strings.NewReader(fmt.Sprintf(`{"chat_session_id":%q,"context_messages":[{"role":"user","content":"continue"}],"guide_mode":"strict"}`, sid)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("supervisor status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var supResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &supResp); err != nil {
		t.Fatalf("decode supervisor: %v", err)
	}
	pack := supResp["supervisor_input_pack"].(map[string]any)
	selection := pack["storyline_selection"].(map[string]any)
	if selection["reference_turn"] != float64(5) || selection["selected_count"] != float64(1) || selection["stale_selected_count"] != float64(0) || selection["stale_dropped_count"] != float64(1) {
		t.Fatalf("storyline selection counts = %#v", selection)
	}
	contextText := extractionStringFromAny(pack["storylines_context"])
	if !strings.Contains(contextText, "Turn 5 keeps the same promise active.") || strings.Contains(contextText, "stale high should not guide") {
		t.Fatalf("storyline context did not select fresh-only row: %q", contextText)
	}
}

func TestSupervisorStorylineSelectionExposesQualityTrace(t *testing.T) {
	fake := &narrativeFakeStore{
		storylines: []store.Storyline{
			{ID: 1, ChatSessionID: "sess-e1f-supervisor", Name: "Fresh arc", Status: "active", CurrentContext: "Fresh arc should guide the next beat", KeyPointsJSON: `["fresh beat","fresh beat"," fresh beat "]`, OngoingTensionsJSON: `["answer pending","answer pending"]`, Confidence: 0.85, EvidenceCount: 3, LastEvidenceTurn: 10, LastTurn: 10},
			{ID: 2, ChatSessionID: "sess-e1f-supervisor", Name: "Stale arc", Status: "active", CurrentContext: "Stale arc should not repeat", KeyPointsJSON: `["stale beat","stale beat"]`, OngoingTensionsJSON: `["old tension","old tension"]`, Confidence: 0.95, EvidenceCount: 1, LastEvidenceTurn: 1, LastTurn: 1},
			{ID: 3, ChatSessionID: "sess-e1f-supervisor", Name: "Resolved arc", Status: "resolved", CurrentContext: "Resolved arc stays summary-only", Confidence: 0.7, EvidenceCount: 2, LastEvidenceTurn: 6, LastTurn: 6},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/supervisor", strings.NewReader(`{"chat_session_id":"sess-e1f-supervisor","context_messages":[{"role":"user","content":"continue"}],"guide_mode":"strict"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	pack := resp["supervisor_input_pack"].(map[string]any)
	selection := pack["storyline_selection"].(map[string]any)
	if selection["selected_count"] != float64(1) || selection["stale_dropped_count"] != float64(1) {
		t.Fatalf("storyline_selection = %#v, want one selected and one stale dropped", selection)
	}
	if selection["resolved_summary_count"] != float64(1) {
		t.Fatalf("resolved_summary_count = %v, want 1", selection["resolved_summary_count"])
	}
	contextText, _ := pack["storylines_context"].(string)
	if !strings.Contains(contextText, "Fresh arc") {
		t.Fatalf("storylines_context missing selected storyline: %q", contextText)
	}
	if strings.Contains(contextText, "Stale arc should not repeat") || strings.Contains(contextText, "stale beat") || strings.Contains(contextText, "Resolved arc stays summary-only") {
		t.Fatalf("storylines_context leaked stale/resolved full context: %q", contextText)
	}
	if !strings.Contains(contextText, "[Resolved Storylines Summary]") || !strings.Contains(contextText, "Resolved arc resolved at turn 6") {
		t.Fatalf("storylines_context missing resolved compressed summary: %q", contextText)
	}
	if strings.Count(contextText, "fresh beat") != 1 || strings.Count(contextText, "answer pending") != 1 {
		t.Fatalf("storylines_context did not dedupe key/tension fields: %q", contextText)
	}
	trace := resp["trace_summary"].(map[string]any)
	if trace["storyline_read_status"] != "ok" {
		t.Fatalf("storyline_read_status = %v, want ok", trace["storyline_read_status"])
	}
}

func TestFormatStorylinesForSupervisorSkipsSelfEchoDetails(t *testing.T) {
	selection := storylineSupervisorSelection{
		Selected: []storylineSelectionEntry{
			{
				Item: store.Storyline{
					Name:                "루나의 고향 마을 방문 약속",
					Status:              "active",
					CurrentContext:      "루나의 고향 마을을 답사 경로에 포함시키기로 함",
					KeyPointsJSON:       `["루나의 고향 마을 방문 약속","서부 답사 전 루나에게 동선 확인"]`,
					OngoingTensionsJSON: `["루나의 고향 마을 방문 약속","방문 시점 조율 필요"]`,
					Confidence:          0.82,
					EvidenceCount:       3,
				},
				Confidence: 0.82,
			},
			{
				Item: store.Storyline{
					Name:                "점심 약속 (시우-루나)",
					Status:              "active",
					KeyPointsJSON:       `["점심 약속 (시우-루나)","약속 장소를 정해야 함"]`,
					OngoingTensionsJSON: `["약속 장소를 정해야 함"]`,
					Confidence:          0.7,
					EvidenceCount:       1,
				},
				Confidence: 0.7,
			},
		},
	}

	text := formatStorylinesForSupervisor(selection)
	if strings.Contains(text, "key_points: 루나의 고향 마을 방문 약속") || strings.Contains(text, "tensions: 루나의 고향 마을 방문 약속") {
		t.Fatalf("storylines_context repeated title-equivalent detail: %q", text)
	}
	if strings.Count(text, "점심 약속 (시우-루나)") != 1 {
		t.Fatalf("fallback name should appear only as the main storyline line: %q", text)
	}
	for _, want := range []string{"서부 답사 전 루나에게 동선 확인", "방문 시점 조율 필요", "약속 장소를 정해야 함"} {
		if !strings.Contains(text, want) {
			t.Fatalf("storylines_context dropped non-duplicate detail %q: %q", want, text)
		}
	}
}

func TestSupervisorStorylineManualBatchSyncShapeDropsStaleHigh(t *testing.T) {
	fake := &narrativeFakeStore{
		storylines: []store.Storyline{
			{ID: 1, ChatSessionID: "sess-h2d-manual", Name: "Fresh Current", Status: "active", CurrentContext: "fresh current should guide", Confidence: 0.75, EvidenceCount: 2, LastEvidenceTurn: 10, LastTurn: 10},
			{ID: 2, ChatSessionID: "sess-h2d-manual", Name: "Stale High", Status: "active", CurrentContext: "stale high should not guide", Confidence: 0.99, EvidenceCount: 1, LastEvidenceTurn: 1, LastTurn: 10},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/supervisor", strings.NewReader(`{"chat_session_id":"sess-h2d-manual","context_messages":[{"role":"user","content":"continue"}],"guide_mode":"strict"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	pack := resp["supervisor_input_pack"].(map[string]any)
	contextText := extractionStringFromAny(pack["storylines_context"])
	if !strings.Contains(contextText, "fresh current should guide") || strings.Contains(contextText, "stale high should not guide") {
		t.Fatalf("manual batch-sync shape leaked stale context: %q", contextText)
	}
	selection := pack["storyline_selection"].(map[string]any)
	if selection["selected_count"] != float64(1) || selection["stale_dropped_count"] != float64(1) || selection["stale_selected_count"] != float64(0) {
		t.Fatalf("storyline_selection counts = %#v", selection)
	}
	dropped, _ := selection["dropped"].([]any)
	if len(dropped) != 1 {
		t.Fatalf("dropped = %#v, want one stale row", dropped)
	}
	staleHigh, _ := dropped[0].(map[string]any)
	if staleHigh["name"] != "Stale High" || staleHigh["last_observed_turn"] != float64(1) || staleHigh["freshness_turn_gap"] != float64(9) || staleHigh["is_stale"] != true {
		t.Fatalf("stale high debug fields = %#v", staleHigh)
	}
	if staleHigh["stale_reason"] != "low_evidence_gap" {
		t.Fatalf("stale high reason = %#v", staleHigh["stale_reason"])
	}
}

func TestStorylineSelectionOrdersFreshRowsAndDropsStale(t *testing.T) {
	referenceTurn := 20
	items := []store.Storyline{
		{ID: 1, Name: "Gap two high confidence", Status: "active", CurrentContext: "gap two", Confidence: 0.99, EvidenceCount: 5, LastEvidenceTurn: 18, LastTurn: 20},
		{ID: 2, Name: "Gap one low confidence", Status: "active", CurrentContext: "gap one low", Confidence: 0.20, EvidenceCount: 1, LastEvidenceTurn: 19, LastTurn: 20},
		{ID: 3, Name: "Gap one high confidence", Status: "active", CurrentContext: "gap one high", Confidence: 0.80, EvidenceCount: 1, LastEvidenceTurn: 19, LastTurn: 20},
		{ID: 4, Name: "Gap one high evidence", Status: "active", CurrentContext: "gap one evidence", Confidence: 0.80, EvidenceCount: 6, LastEvidenceTurn: 19, LastTurn: 20},
		{ID: 5, Name: "Stale high confidence", Status: "active", CurrentContext: "stale should drop", Confidence: 0.99, EvidenceCount: 1, LastEvidenceTurn: 1, LastTurn: 20},
	}

	selection := selectStorylinesForSupervisor(items, &referenceTurn, 4)
	got := storylineSelectionNames(selection.Selected)
	want := []string{"Gap one high evidence", "Gap one high confidence", "Gap one low confidence", "Gap two high confidence"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("selected order = %#v, want %#v", got, want)
	}
	dropped := storylineSelectionNames(selection.Dropped)
	if strings.Join(dropped, "|") != "Stale high confidence" {
		t.Fatalf("dropped = %#v, want stale row only", dropped)
	}

	summary := storylineSelectionSummary(selection)
	if summary["stale_dropped_count"] != 1 || summary["stale_selected_count"] != 0 || summary["fresh_rows_take_priority"] != true {
		t.Fatalf("selection summary = %#v", summary)
	}
	contextText := formatStorylinesForSupervisor(selection)
	if strings.Contains(contextText, "stale should drop") {
		t.Fatalf("stale storyline leaked into prompt context: %q", contextText)
	}
}

func TestStorylineSelectionSixActiveRowsDropsStaleAndLowPriority(t *testing.T) {
	referenceTurn := 12
	items := []store.Storyline{
		{ID: 1, Name: "Fresh high evidence", Status: "active", CurrentContext: "fresh high evidence", Confidence: 0.90, EvidenceCount: 5, LastEvidenceTurn: 12, LastTurn: 12},
		{ID: 2, Name: "Fresh high confidence", Status: "active", CurrentContext: "fresh high confidence", Confidence: 0.95, EvidenceCount: 2, LastEvidenceTurn: 11, LastTurn: 12},
		{ID: 3, Name: "Fresh medium", Status: "active", CurrentContext: "fresh medium", Confidence: 0.70, EvidenceCount: 2, LastEvidenceTurn: 10, LastTurn: 12},
		{ID: 4, Name: "Fresh low priority", Status: "active", CurrentContext: "fresh low priority should drop by limit", Confidence: 0.10, EvidenceCount: 1, LastEvidenceTurn: 10, LastTurn: 12},
		{ID: 5, Name: "Stale high confidence", Status: "active", CurrentContext: "stale high should drop", Confidence: 0.99, EvidenceCount: 1, LastEvidenceTurn: 1, LastTurn: 12},
		{ID: 6, Name: "Stale medium", Status: "active", CurrentContext: "stale medium should drop", Confidence: 0.50, EvidenceCount: 1, LastEvidenceTurn: 2, LastTurn: 12},
	}

	selection := selectStorylinesForSupervisor(items, &referenceTurn, 3)
	if got := strings.Join(storylineSelectionNames(selection.Selected), "|"); got != "Fresh high evidence|Fresh high confidence|Fresh medium" {
		t.Fatalf("selected = %q, want top three fresh rows", got)
	}
	dropped := strings.Join(storylineSelectionNames(selection.Dropped), "|")
	for _, unwanted := range []string{"Stale high confidence", "Stale medium", "Fresh low priority"} {
		if !strings.Contains(dropped, unwanted) {
			t.Fatalf("dropped = %q, missing %q", dropped, unwanted)
		}
	}
	summary := storylineSelectionSummary(selection)
	if summary["selected_count"] != 3 || summary["dropped_count"] != 3 || summary["stale_dropped_count"] != 2 {
		t.Fatalf("six-active selection summary = %#v", summary)
	}
	if contextText := formatStorylinesForSupervisor(selection); strings.Contains(contextText, "stale high should drop") || strings.Contains(contextText, "fresh low priority should drop by limit") {
		t.Fatalf("dropped storyline leaked into supervisor context: %q", contextText)
	}
}

func TestStorylineSelectionFallsBackToMostRecentStaleWhenNoFreshRows(t *testing.T) {
	referenceTurn := 35
	items := []store.Storyline{
		{ID: 1, Name: "Older stale", Status: "active", CurrentContext: "older stale fallback", Confidence: 0.99, EvidenceCount: 1, LastEvidenceTurn: 1, LastTurn: 35},
		{ID: 2, Name: "Recent stale", Status: "active", CurrentContext: "recent stale fallback", Confidence: 0.50, EvidenceCount: 1, LastEvidenceTurn: 31, LastTurn: 35},
	}

	selection := selectStorylinesForSupervisor(items, &referenceTurn, 5)
	got := storylineSelectionNames(selection.Selected)
	if strings.Join(got, "|") != "Recent stale" {
		t.Fatalf("selected stale fallback = %#v, want most recent stale", got)
	}
	if len(selection.Dropped) != 1 || selection.Dropped[0].Item.Name != "Older stale" {
		t.Fatalf("dropped stale fallback = %#v", storylineSelectionNames(selection.Dropped))
	}
	summary := storylineSelectionSummary(selection)
	if summary["stale_selected_count"] != 1 || summary["stale_dropped_count"] != 1 {
		t.Fatalf("summary = %#v, want one stale selected and one stale dropped", summary)
	}
}

func TestStorylineSelectionDropsStaleRowsEvenWhenActiveCountFitsLimit(t *testing.T) {
	referenceTurn := 7
	items := []store.Storyline{
		{ID: 1, Name: "Fresh first", Status: "active", CurrentContext: "fresh one", Confidence: 0.60, EvidenceCount: 2, LastEvidenceTurn: 7, LastTurn: 7},
		{ID: 2, Name: "Fresh second", Status: "active", CurrentContext: "fresh two", Confidence: 0.55, EvidenceCount: 2, LastEvidenceTurn: 6, LastTurn: 7},
		{ID: 3, Name: "Stale high confidence", Status: "active", CurrentContext: "stale high should not guide", Confidence: 0.99, EvidenceCount: 1, LastEvidenceTurn: 1, LastTurn: 7},
		{ID: 4, Name: "Stale second", Status: "active", CurrentContext: "stale second should not guide", Confidence: 0.95, EvidenceCount: 1, LastEvidenceTurn: 2, LastTurn: 7},
	}

	selection := selectStorylinesForSupervisor(items, &referenceTurn, 5)
	selected := storylineSelectionNames(selection.Selected)
	if strings.Join(selected, "|") != "Fresh first|Fresh second" {
		t.Fatalf("selected = %#v, want only fresh rows even though active count fits limit", selected)
	}
	dropped := storylineSelectionNames(selection.Dropped)
	if strings.Join(dropped, "|") != "Stale second|Stale high confidence" {
		t.Fatalf("dropped = %#v, want stale rows dropped", dropped)
	}
	summary := storylineSelectionSummary(selection)
	if summary["selected_count"] != 2 || summary["stale_selected_count"] != 0 || summary["stale_dropped_count"] != 2 {
		t.Fatalf("selection summary = %#v, want selected=2 stale_selected=0 stale_dropped=2", summary)
	}
	contextText := formatStorylinesForSupervisor(selection)
	if strings.Contains(contextText, "stale high should not guide") || strings.Contains(contextText, "stale second should not guide") {
		t.Fatalf("stale storyline leaked into prompt context: %q", contextText)
	}
}

func TestTrustControlsFilterAndPrioritizeNarrativeInjection(t *testing.T) {
	referenceTurn := 30
	storylines := []store.Storyline{
		{ID: 1, Name: "Pinned stale", Status: "active", CurrentContext: "pinned should stay in budget", Pinned: true, Confidence: 0.4, EvidenceCount: 1, LastEvidenceTurn: 1, LastTurn: 30},
		{ID: 2, Name: "Fresh normal", Status: "active", CurrentContext: "fresh should follow pinned", Confidence: 0.8, EvidenceCount: 3, LastEvidenceTurn: 30, LastTurn: 30},
		{ID: 3, Name: "Suppressed", Status: "active", CurrentContext: "suppressed must not guide", Suppressed: true, Confidence: 1, EvidenceCount: 8, LastEvidenceTurn: 30, LastTurn: 30},
	}
	selection := selectStorylinesForSupervisor(storylines, &referenceTurn, 2)
	if got := strings.Join(storylineSelectionNames(selection.Selected), "|"); got != "Pinned stale|Fresh normal" {
		t.Fatalf("selected = %q, want pinned first then fresh normal", got)
	}
	if len(selection.Suppressed) != 1 || selection.Suppressed[0].Item.Name != "Suppressed" {
		t.Fatalf("suppressed storylines = %#v", storylineSelectionNames(selection.Suppressed))
	}
	first := storylineSelectionEntryMap(selection.Selected[0])
	if first["pinned"] != true || first["suppressed"] != false {
		t.Fatalf("selected map missing trust flags: %#v", first)
	}
	if contextText := formatStorylinesForSupervisor(selection); strings.Contains(contextText, "suppressed must not guide") {
		t.Fatalf("suppressed storyline leaked into prompt context: %q", contextText)
	}

	worldRules := visibleSessionStateWorldRules([]store.WorldRule{
		{ID: 1, Scope: "session", Category: "plain", Key: "normal"},
		{ID: 2, Scope: "session", Category: "pinned", Key: "must_keep", Pinned: true},
		{ID: 3, Scope: "session", Category: "suppressed", Key: "must_drop", Suppressed: true},
	})
	if len(worldRules) != 2 || worldRules[0].ID != 2 || worldRules[1].ID == 3 {
		t.Fatalf("world rule trust filtering/order = %#v", worldRules)
	}

	hooks := continuityPendingThreads([]store.PendingThread{
		{ID: 1, Description: "normal hook", Status: "open", LastSeenTurn: 30},
		{ID: 2, Description: "pinned hook", Status: "open", LastSeenTurn: 1, Pinned: true},
		{ID: 3, Description: "suppressed hook", Status: "open", LastSeenTurn: 99, Suppressed: true},
	}, 0)
	if len(hooks) != 2 || hooks[0].ID != 2 || hooks[1].ID == 3 {
		t.Fatalf("pending thread trust filtering/order = %#v", hooks)
	}
}

func TestWorldGraphLiteActiveScopeAndInheritedRules(t *testing.T) {
	fake := &narrativeFakeStore{
		worldRules: []store.WorldRule{
			{ID: 1, ChatSessionID: "sess-world", Scope: "root", Category: "worldview", Key: "gravity", ValueJSON: `"stable"`, Pinned: true},
			{ID: 2, ChatSessionID: "sess-world", Scope: "region", Category: "weather", Key: "north_wind", ValueJSON: `"cold"`},
			{ID: 3, ChatSessionID: "sess-world", Scope: "location", ScopeName: "Archive", Category: "place", Key: "quiet_rule", ValueJSON: `"whisper"`},
			{ID: 4, ChatSessionID: "sess-world", Scope: "location", ScopeName: "Cellar", Category: "place", Key: "wrong_place", ValueJSON: `"exclude"`},
			{ID: 5, ChatSessionID: "sess-world", Scope: "root", Category: "hidden", Key: "suppressed", Suppressed: true},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPatch, "/session/sess-world/active-scope", strings.NewReader(`{"active_scope":"location","scope_name":"Archive"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var scopeResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &scopeResp); err != nil {
		t.Fatalf("decode patch: %v", err)
	}
	if scopeResp["active_scope"] != "location" || scopeResp["scope_name"] != "Archive" {
		t.Fatalf("active scope response = %#v", scopeResp)
	}
	chain := scopeResp["scope_chain"].([]any)
	if strings.Join([]string{fmt.Sprint(chain[0]), fmt.Sprint(chain[1]), fmt.Sprint(chain[2]), fmt.Sprint(chain[3])}, ">") != "location>region>root>session" {
		t.Fatalf("scope_chain = %#v, want location>region>root>session", chain)
	}

	req = httptest.NewRequest(http.MethodGet, "/world-rules/sess-world/inherited", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("inherited status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode inherited: %v", err)
	}
	if resp["active_scope"] != "location" || resp["scope_name"] != "Archive" {
		t.Fatalf("inherited active scope = %#v", resp)
	}
	rules := resp["rules"].([]any)
	if len(rules) != 3 {
		t.Fatalf("rules len = %d, want 3: %#v", len(rules), rules)
	}
	keys := []string{}
	inheritedByKey := map[string]bool{}
	for _, raw := range rules {
		item := raw.(map[string]any)
		key := fmt.Sprint(item["key"])
		keys = append(keys, key)
		inheritedByKey[key], _ = item["inherited"].(bool)
	}
	if strings.Join(keys, "|") != "quiet_rule|north_wind|gravity" {
		t.Fatalf("rule keys = %#v, want active location then region/root", keys)
	}
	if inheritedByKey["quiet_rule"] || !inheritedByKey["north_wind"] || !inheritedByKey["gravity"] {
		t.Fatalf("inherited flags = %#v", inheritedByKey)
	}
	for _, forbidden := range []string{"wrong_place", "suppressed"} {
		if strings.Contains(strings.Join(keys, "|"), forbidden) {
			t.Fatalf("forbidden rule leaked into inherited result: %s in %#v", forbidden, keys)
		}
	}
}

func TestStorylineStaleWindowClampAndLastEvidencePriority(t *testing.T) {
	referenceTurn := 20
	lowEvidence := store.Storyline{Status: "active", EvidenceCount: 0, LastEvidenceTurn: 1, LastTurn: 19}
	lowSnapshot := storylineStaleSnapshot(lowEvidence, &referenceTurn)
	if lowSnapshot["last_observed_turn"] != 1 || lowSnapshot["stale_after_turns"] != 3 || lowSnapshot["freshness_turn_gap"] != 19 || lowSnapshot["is_stale"] != true {
		t.Fatalf("low evidence snapshot = %#v", lowSnapshot)
	}
	if lowSnapshot["stale_reason"] != "low_evidence_gap" {
		t.Fatalf("low evidence stale_reason = %#v", lowSnapshot["stale_reason"])
	}

	highEvidence := store.Storyline{Status: "active", EvidenceCount: 20, LastEvidenceTurn: 15, LastTurn: 19}
	highSnapshot := storylineStaleSnapshot(highEvidence, &referenceTurn)
	if highSnapshot["last_observed_turn"] != 15 || highSnapshot["stale_after_turns"] != 8 || highSnapshot["freshness_turn_gap"] != 5 || highSnapshot["is_stale"] != false {
		t.Fatalf("high evidence snapshot = %#v", highSnapshot)
	}
}

func storylineSelectionNames(items []storylineSelectionEntry) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Item.Name)
	}
	return out
}

func TestCharacterStatePatchAndSpeechRoutesUseLiveStore(t *testing.T) {
	fake := &narrativeFakeStore{
		characterState: &store.CharacterState{
			ID:            4,
			ChatSessionID: "sess-char",
			CharacterName: "Chloe",
			StatusJSON:    `{"emotion":"calm"}`,
			TurnIndex:     6,
		},
		characterStates: []store.CharacterState{
			{ID: 4, ChatSessionID: "sess-char", CharacterName: "Chloe", StatusJSON: `{"emotion":"calm"}`, TurnIndex: 6},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPatch, "/characters/sess-char/Chloe", strings.NewReader(`{"status":{"emotion":"angry"},"relationships":{"Hero":{"affection":70,"tension":45}},"turn_index":7}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedCharacterStates) != 1 {
		t.Fatalf("saved states = %d, want 1", len(fake.savedCharacterStates))
	}
	saved := fake.savedCharacterStates[0]
	if saved.TurnIndex != 7 || !strings.Contains(saved.StatusJSON, "angry") || !strings.Contains(saved.RelationshipsJSON, "affection") {
		t.Fatalf("saved character state = %#v", saved)
	}
	if len(fake.savedCharacterEvents) != 1 || fake.savedCharacterEvents[0].EventType != "manual_patch" {
		t.Fatalf("manual patch event = %#v", fake.savedCharacterEvents)
	}

	req = httptest.NewRequest(http.MethodPatch, "/characters/sess-char/Chloe/speech", strings.NewReader(`{"speech_style":{"default_tone":"dry","speech_notes":"short replies"}}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("speech status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedCharacterStates) != 2 {
		t.Fatalf("saved states after speech = %d, want 2", len(fake.savedCharacterStates))
	}
	if !strings.Contains(fake.savedCharacterStates[1].SpeechStyleJSON, "dry") {
		t.Fatalf("speech style was not saved: %#v", fake.savedCharacterStates[1])
	}
	if len(fake.savedCharacterEvents) != 2 || fake.savedCharacterEvents[1].EventType != "speech_style_patch" {
		t.Fatalf("speech patch event = %#v", fake.savedCharacterEvents)
	}
}

func TestCharacterDeleteUsesLiveStoreAndWritesAudit(t *testing.T) {
	fake := &narrativeFakeStore{
		characterState: &store.CharacterState{
			ID:            8,
			ChatSessionID: "sess-char",
			CharacterName: "Noise",
			StatusJSON:    `{"emotion":"unknown"}`,
			TurnIndex:     9,
			CreatedAt:     time.Date(2026, 6, 1, 1, 2, 3, 0, time.UTC),
			UpdatedAt:     time.Date(2026, 6, 1, 1, 2, 4, 0, time.UTC),
		},
		characterStates: []store.CharacterState{
			{ID: 8, ChatSessionID: "sess-char", CharacterName: "Noise", TurnIndex: 9},
			{ID: 9, ChatSessionID: "sess-char", CharacterName: "Chloe", TurnIndex: 9},
		},
		characterEvents: []store.CharacterEvent{
			{ID: 1, ChatSessionID: "sess-char", CharacterName: "Noise", EventType: "snapshot"},
			{ID: 2, ChatSessionID: "sess-char", CharacterName: "Chloe", EventType: "snapshot"},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/characters/sess-char/Noise", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if fake.deletedCharacterName != "Noise" {
		t.Fatalf("deletedCharacterName = %q, want Noise", fake.deletedCharacterName)
	}
	if len(fake.characterStates) != 1 || fake.characterStates[0].CharacterName != "Chloe" {
		t.Fatalf("character state delete was not scoped: %#v", fake.characterStates)
	}
	if len(fake.characterEvents) != 1 || fake.characterEvents[0].CharacterName != "Chloe" {
		t.Fatalf("character events were not scoped: %#v", fake.characterEvents)
	}
	if len(fake.auditLogs) == 0 {
		t.Fatal("expected manual_delete audit log")
	}
	audit := fake.auditLogs[0]
	if audit.EventType != "manual_delete" || audit.TargetType != "character" || audit.TargetID != 8 {
		t.Fatalf("unexpected audit: %#v", audit)
	}
	if !strings.Contains(audit.DetailsJSON, "character_events_deleted") || !strings.Contains(audit.DetailsJSON, "Noise") {
		t.Fatalf("audit details missing character delete history: %s", audit.DetailsJSON)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["deleted"] != true || resp["audit_written"] != true {
		t.Fatalf("response missing delete/audit proof: %#v", resp)
	}
}

func TestSpeechStylePatchFlowsIntoPrepareTurnPromptWithDistinctTone(t *testing.T) {
	fake := &narrativeFakeStore{
		characterState: &store.CharacterState{
			ID:            9,
			ChatSessionID: "sess-speech-flow",
			CharacterName: "Chloe",
			StatusJSON:    `{"emotion":"focused"}`,
			TurnIndex:     11,
		},
		characterStates: []store.CharacterState{
			{ID: 9, ChatSessionID: "sess-speech-flow", CharacterName: "Chloe", StatusJSON: `{"emotion":"focused"}`, TurnIndex: 11},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	patchSpeech := func(body string) {
		req := httptest.NewRequest(http.MethodPatch, "/characters/sess-speech-flow/Chloe/speech", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("speech patch status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
	}
	prepare := func() (string, string) {
		body := `{"chat_session_id":"sess-speech-flow","turn_index":12,"raw_user_input":"How does Chloe answer the same question?","settings":{"max_injection_chars":1200,"injection_enabled":true,"input_context_enabled":false}}`
		req := httptest.NewRequest(http.MethodPost, "/prepare-turn", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("prepare-turn status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode prepare-turn response: %v", err)
		}
		pack, ok := resp["injection_pack"].(map[string]any)
		if !ok {
			t.Fatalf("injection_pack missing: %+v", resp)
		}
		gp, ok := resp["generation_packet"].(map[string]any)
		if !ok {
			t.Fatalf("generation_packet missing: %+v", resp)
		}
		return extractionStringFromAny(pack["character_text"]), extractionStringFromAny(gp["injection_text"])
	}

	patchSpeech(`{"speech_style":{"default_tone":"dry","honorific_style":"plain","speech_notes":"short"}}`)
	dryCharacterText, dryInjection := prepare()
	for _, want := range []string{"speech_style", "dry", "plain", "short"} {
		if !strings.Contains(dryCharacterText, want) || !strings.Contains(dryInjection, want) {
			t.Fatalf("dry speech style %q missing from character/injection text:\ncharacter=%s\ninjection=%s", want, dryCharacterText, dryInjection)
		}
	}

	patchSpeech(`{"speech_style":{"default_tone":"warm","honorific_style":"formal","speech_notes":"careful"}}`)
	warmCharacterText, warmInjection := prepare()
	for _, want := range []string{"speech_style", "warm", "formal", "careful"} {
		if !strings.Contains(warmCharacterText, want) || !strings.Contains(warmInjection, want) {
			t.Fatalf("warm speech style %q missing from character/injection text:\ncharacter=%s\ninjection=%s", want, warmCharacterText, warmInjection)
		}
	}
	for _, old := range []string{"dry", "plain", "short"} {
		if strings.Contains(warmCharacterText, old) || strings.Contains(warmInjection, old) {
			t.Fatalf("old speech style %q leaked after second patch:\ncharacter=%s\ninjection=%s", old, warmCharacterText, warmInjection)
		}
	}
	if dryInjection == warmInjection {
		t.Fatalf("same situation should produce distinct prompt text after speech style patch")
	}

	oldClient := proxyHTTPClient
	defer func() { proxyHTTPClient = oldClient }()
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(r.Body)
		body := string(raw)
		reply := ""
		switch {
		case strings.Contains(body, "dry") && strings.Contains(body, "plain") && strings.Contains(body, "short"):
			reply = "Dry. Short answer."
		case strings.Contains(body, "warm") && strings.Contains(body, "formal") && strings.Contains(body, "careful"):
			reply = "I will answer carefully in a warm, formal tone."
		default:
			t.Fatalf("proxy prompt did not carry expected speech style: %s", body)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"model":"style-replay","choices":[{"message":{"content":"` + reply + `"}}]}`)),
		}, nil
	})}
	callMainProxy := func(injection string) string {
		req := httptest.NewRequest(http.MethodPost, "/proxy/plugin-main", strings.NewReader(`{
			"endpoint":"https://api.example.com/v1",
			"api_key":"sk-style",
			"model":"style-replay",
			"provider":"openai",
			"messages":[
				{"role":"system","content":"Answer according to the supplied character speech_style."},
				{"role":"user","content":`+strconv.Quote(injection)+`}
			]
		}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("plugin-main status = %d, want 200: %s", rec.Code, rec.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode plugin-main response: %v", err)
		}
		choices, _ := resp["choices"].([]any)
		if len(choices) == 0 {
			t.Fatalf("plugin-main response missing choices: %+v", resp)
		}
		choice, _ := choices[0].(map[string]any)
		message, _ := choice["message"].(map[string]any)
		return extractionStringFromAny(message["content"])
	}
	dryReply := callMainProxy(dryInjection)
	warmReply := callMainProxy(warmInjection)
	if !strings.Contains(dryReply, "Dry") || !strings.Contains(dryReply, "Short") {
		t.Fatalf("dry reply did not reflect dry/short speech style: %q", dryReply)
	}
	if !strings.Contains(warmReply, "warm") || !strings.Contains(warmReply, "formal") {
		t.Fatalf("warm reply did not reflect warm/formal speech style: %q", warmReply)
	}
	if dryReply == warmReply {
		t.Fatalf("controlled provider replies should differ by speech style")
	}
}

func TestContinuityPackAssemblesPythonReferenceSources(t *testing.T) {
	now := time.Date(2026, 5, 31, 10, 0, 0, 0, time.UTC)
	fake := &narrativeFakeStore{
		storylines: []store.Storyline{
			{ID: 1, ChatSessionID: "sess-1", Name: "Fresh gate arc", Status: "active", CurrentContext: "Gate pressure rises.", LastTurn: 12, LastEvidenceTurn: 12},
		},
		characterEvents: []store.CharacterEvent{
			{ID: 1, ChatSessionID: "sess-1", CharacterName: "Alice", TurnIndex: 8, EventType: "speech_style_patch", DetailsJSON: `{"detail":"ignore"}`, CreatedAt: now.Add(-3 * time.Minute)},
			{ID: 2, ChatSessionID: "sess-1", CharacterName: "Bob", TurnIndex: 10, EventType: "relationship_shift", DetailsJSON: `{"detail":"trust warms"}`, CreatedAt: now.Add(-2 * time.Minute)},
			{ID: 3, ChatSessionID: "sess-1", CharacterName: "Alice", TurnIndex: 11, EventType: "relationship_shift", DetailsJSON: `{"detail":"trust sharpens"}`, CreatedAt: now.Add(-1 * time.Minute)},
		},
		pendingThreads: []store.PendingThread{
			{ID: 1, ChatSessionID: "sess-1", ThreadKey: "suppressed", Status: "open", SourceTurn: 13, Suppressed: true},
			{ID: 2, ChatSessionID: "sess-1", ThreadKey: "resolved", Status: "resolved", SourceTurn: 14},
			{ID: 3, ChatSessionID: "sess-1", ThreadKey: "promise", Status: "open", SourceTurn: 9, LastSeenTurn: 15, Pinned: true},
			{ID: 4, ChatSessionID: "sess-1", ThreadKey: "risk", Status: "paused", SourceTurn: 8, LastSeenTurn: 12},
		},
		episodeSummaries: []store.EpisodeSummary{
			{ID: 1, ChatSessionID: "sess-1", FromTurn: 1, ToTurn: 20, SummaryText: "They reach the gate."},
		},
		worldRules: []store.WorldRule{
			{ID: 1, ChatSessionID: "sess-1", Scope: "location", ScopeName: "Archive", Category: "access", Key: "sealed", SourceTurn: 12},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/continuity-pack/sess-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode continuity pack: %v", err)
	}
	if resp["pack_status"] != "ready" || resp["skeleton_only"] != false {
		t.Fatalf("pack status = %#v skeleton=%#v, want ready/false", resp["pack_status"], resp["skeleton_only"])
	}
	relationshipShifts := resp["relationship_shifts"].([]any)
	if len(relationshipShifts) != 2 {
		t.Fatalf("relationship_shifts len = %d, want 2: %#v", len(relationshipShifts), relationshipShifts)
	}
	firstShift := relationshipShifts[0].(map[string]any)
	if firstShift["event_type"] != "relationship_shift" || firstShift["character_name"] != "Alice" || firstShift["turn_index"] != float64(11) {
		t.Fatalf("relationship shift ordering/shape mismatch: %#v", firstShift)
	}
	pendingThreads := resp["pending_threads"].([]any)
	if len(pendingThreads) != 2 {
		t.Fatalf("pending_threads len = %d, want 2: %#v", len(pendingThreads), pendingThreads)
	}
	firstThread := pendingThreads[0].(map[string]any)
	if firstThread["thread_key"] != "promise" && firstThread["title"] != "promise" {
		t.Fatalf("pinned/open pending thread should sort first and exclude suppressed/resolved: %#v", pendingThreads)
	}
	sectionStatus := resp["section_status"].(map[string]any)
	for _, key := range []string{"active_storylines", "relationship_shifts", "pending_threads", "continuity_hooks", "latest_episode", "world_constraints"} {
		if _, ok := sectionStatus[key]; !ok {
			t.Fatalf("section_status missing %s: %#v", key, sectionStatus)
		}
	}
	if resp["latest_episode"] == nil {
		t.Fatal("latest_episode missing")
	}
	worldConstraints := resp["world_constraints"].([]any)
	if len(worldConstraints) != 1 {
		t.Fatalf("world_constraints len = %d, want 1", len(worldConstraints))
	}
}

func TestContinuityPackEmptySessionKeepsHooksAlias(t *testing.T) {
	fake := &narrativeFakeStore{}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/continuity-pack/sess-empty", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode continuity pack: %v", err)
	}
	if resp["pack_status"] != "empty" || resp["skeleton_only"] != false {
		t.Fatalf("empty pack status = %#v skeleton=%#v, want empty/false", resp["pack_status"], resp["skeleton_only"])
	}
	sectionStatus := resp["section_status"].(map[string]any)
	hooks, ok := sectionStatus["continuity_hooks"].(map[string]any)
	if !ok {
		t.Fatalf("continuity_hooks status missing: %#v", sectionStatus)
	}
	if hooks["count"] != float64(0) {
		t.Fatalf("continuity_hooks count = %#v, want 0", hooks["count"])
	}
	warnings := resp["warnings"].([]any)
	if len(warnings) == 0 {
		t.Fatal("empty continuity pack should include a warning")
	}
}

func TestWorldRulesSyncPatchTrustDeleteUseLiveStore(t *testing.T) {
	fake := &narrativeFakeStore{}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	syncBody := `{
		"chat_session_id":"sess-world",
		"mode":"apply",
		"turn_index":9,
		"supervisor_response":{
			"section_world":{
				"genre_hint":"mystery",
				"constants":[{"category":"physics","key":"sealed_gate","value":{"opens":"brass_key"}}],
				"rules":["The archive cellar stays locked after midnight."],
				"world_rules":["Legacy fallback rule is still accepted."],
				"confidence_notes":["Confidence note fallback is still accepted."]
			}
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/world-rules/sync", strings.NewReader(syncBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("sync status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedWorldRules) != 4 {
		t.Fatalf("saved world rules = %d, want 4: %#v", len(fake.savedWorldRules), fake.savedWorldRules)
	}
	if got := fake.savedWorldRules[0]; got.ChatSessionID != "sess-world" || got.Scope != "root" || got.Category != "physics" || got.Key != "sealed_gate" || got.Genre != "mystery" || got.SourceTurn != 9 || !strings.Contains(got.ValueJSON, "brass_key") {
		t.Fatalf("constant world rule = %#v", got)
	}
	if got := fake.savedWorldRules[1]; got.Scope != "root" || got.Category != "custom" || got.Key != "The archive cellar stays locked after midnight." || got.ValueJSON != "" {
		t.Fatalf("string world rule = %#v", got)
	}
	if got := fake.savedWorldRules[2]; got.Scope != "root" || got.Category != "custom" || got.Key != "Legacy fallback rule is still accepted." {
		t.Fatalf("world_rules fallback rule = %#v", got)
	}
	if got := fake.savedWorldRules[3]; got.Scope != "root" || got.Category != "custom" || got.Key != "Confidence note fallback is still accepted." {
		t.Fatalf("confidence_notes fallback rule = %#v", got)
	}

	req = httptest.NewRequest(http.MethodPatch, "/world-rules/7", strings.NewReader(`{"scope":"location","scope_name":"Archive","value":"The cellar rule changed.","source_turn":10}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.worldRulePatches) != 1 || fake.worldRulePatches[0]["scope"] != "location" || fake.worldRulePatches[0]["scope_name"] != "Archive" || fake.worldRulePatches[0]["source_turn"] != 10 || !strings.Contains(fmt.Sprint(fake.worldRulePatches[0]["value_json"]), "cellar") {
		t.Fatalf("world rule patch = %#v", fake.worldRulePatches)
	}

	req = httptest.NewRequest(http.MethodPatch, "/world-rules/7/trust", strings.NewReader(`{"pinned":true,"suppressed":false,"user_corrected":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("trust status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if fake.worldRuleTrustPatch["pinned"] != true || fake.worldRuleTrustPatch["suppressed"] != false || fake.worldRuleTrustPatch["user_corrected"] != true {
		t.Fatalf("world rule trust patch = %#v", fake.worldRuleTrustPatch)
	}

	req = httptest.NewRequest(http.MethodDelete, "/world-rules/7", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if fake.deletedWorldRuleID != 7 {
		t.Fatalf("deletedWorldRuleID = %d, want 7", fake.deletedWorldRuleID)
	}
}

func TestWorldRulesInheritedIncludesSessionScopedRules(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)
	fake := &narrativeFakeStore{
		worldRules: []store.WorldRule{
			{ID: 1, ChatSessionID: "sess-world", Scope: "root", Category: "cosmology", Key: "cassia_created_world", ValueJSON: `{"rule":"Cassia ordered chaos into the world."}`, UpdatedAt: now},
			{ID: 2, ChatSessionID: "sess-world", Scope: "session", ScopeName: "Cassia Doctrine", Category: "cosmology", Key: "apostles_borrow_divine_power", ValueJSON: `{"rule":"Apostles borrow divine power against monsters."}`, UpdatedAt: now},
			{ID: 3, ChatSessionID: "sess-world", Scope: "session", Category: "hidden", Key: "suppressed_rule", Suppressed: true, UpdatedAt: now},
			{ID: 4, ChatSessionID: "other", Scope: "session", Category: "other", Key: "other_session_rule", UpdatedAt: now},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/world-rules/sess-world/inherited?active_scope=root", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("inherited status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Status     string           `json:"status"`
		ScopeChain []string         `json:"scope_chain"`
		Rules      []map[string]any `json:"rules"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "ok" {
		t.Fatalf("status = %q", body.Status)
	}
	if !reflect.DeepEqual(body.ScopeChain, []string{"root", "session"}) {
		t.Fatalf("scope chain = %#v, want root+session", body.ScopeChain)
	}
	if len(body.Rules) != 2 {
		t.Fatalf("rules = %d, want 2: %#v", len(body.Rules), body.Rules)
	}
	var sawSession bool
	for _, rule := range body.Rules {
		if rule["scope"] == "session" && rule["key"] == "apostles_borrow_divine_power" {
			sawSession = true
			if rule["inherited"] != true {
				t.Fatalf("session rule should be marked inherited under root active scope: %#v", rule)
			}
		}
	}
	if !sawSession {
		t.Fatalf("session scoped rule missing from inherited response: %#v", body.Rules)
	}
}

func TestStorylineAndWorldRuleReadExposeFreshnessFields(t *testing.T) {
	now := time.Date(2026, 5, 31, 12, 0, 0, 0, time.UTC)
	fake := &narrativeFakeStore{
		storylines: []store.Storyline{
			{
				ID:                  1,
				ChatSessionID:       "sess-fresh",
				Name:                "Rooftop Promise",
				Status:              "active",
				Confidence:          0.82,
				EvidenceCount:       4,
				LastEvidenceTurn:    11,
				KeyPointsJSON:       `[" clue ","clue","Clue"]`,
				OngoingTensionsJSON: `["answer pending","answer pending"," answer pending "]`,
				LastTurn:            12,
				UpdatedAt:           now,
			},
		},
		worldRules: []store.WorldRule{
			{
				ID:            2,
				ChatSessionID: "sess-fresh",
				Scope:         "root",
				Category:      "custom",
				Key:           "The archive cellar stays locked.",
				SourceTurn:    12,
				UpdatedAt:     now,
			},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/storylines/sess-fresh", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("storylines status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var storylineResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &storylineResp); err != nil {
		t.Fatalf("decode storylines: %v", err)
	}
	storylines, _ := storylineResp["storylines"].([]any)
	if len(storylines) != 1 {
		t.Fatalf("storyline count = %d, want 1: %#v", len(storylines), storylineResp)
	}
	storyline := storylines[0].(map[string]any)
	if storyline["last_turn"] != float64(12) || strings.TrimSpace(fmt.Sprint(storyline["updated_at"])) == "" {
		t.Fatalf("storyline freshness fields missing: %#v", storyline)
	}
	if storyline["confidence"] != float64(0.82) || storyline["evidence_count"] != float64(4) || storyline["last_evidence_turn"] != float64(11) {
		t.Fatalf("storyline quality fields missing: %#v", storyline)
	}
	if storyline["last_observed_turn"] != float64(11) || storyline["stale_after_turns"] != float64(6) {
		t.Fatalf("storyline stale snapshot fields missing: %#v", storyline)
	}
	if storyline["key_points_json"] != `["clue"]` || storyline["ongoing_tensions_json"] != `["answer pending"]` {
		t.Fatalf("storyline read path did not normalize key/tension lists: %#v", storyline)
	}

	req = httptest.NewRequest(http.MethodGet, "/world-rules/sess-fresh", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("world-rules status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var worldResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &worldResp); err != nil {
		t.Fatalf("decode world-rules: %v", err)
	}
	items, _ := worldResp["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("world-rule count = %d, want 1: %#v", len(items), worldResp)
	}
	rule := items[0].(map[string]any)
	if rule["source_turn"] != float64(12) || strings.TrimSpace(fmt.Sprint(rule["updated_at"])) == "" {
		t.Fatalf("world-rule freshness fields missing: %#v", rule)
	}
}

func TestNarrativeRoutesErrNotEnabledFallback(t *testing.T) {
	fake := &narrativeFakeStore{errNotEnabled: true}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	tests := []string{
		"/storylines/sess-1",
		"/world-rules/sess-1",
		"/world-rules/sess-1/inherited",
		"/characters/sess-1",
		"/characters/sess-1/Alice",
		"/characters/sess-1/Alice/events",
		"/pending-threads/sess-1",
		"/active-states/sess-1",
		"/canonical-state-layer/sess-1",
		"/episodes/sess-1",
		"/session-state/sess-1",
		"/continuity-pack/sess-1",
	}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
			}
			var resp map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if resp["status"] != "ok" {
				t.Errorf("status = %v, want ok", resp["status"])
			}
		})
	}
}

func TestFiveNarrativeHandlersStoreBacked(t *testing.T) {
	fake := &narrativeFakeStore{
		storylines: []store.Storyline{
			{ID: 1, ChatSessionID: "sess-1", Name: "Arc 1", Status: "active"},
		},
		worldRules: []store.WorldRule{
			{ID: 1, ChatSessionID: "sess-1", Scope: "global", Category: "magic", Key: "mana"},
		},
		characterStates: []store.CharacterState{
			{ID: 1, ChatSessionID: "sess-1", CharacterName: "Alice", TurnIndex: 5},
		},
		pendingThreads: []store.PendingThread{
			{ID: 1, ChatSessionID: "sess-1", ThreadKey: "hook-1", Status: "open"},
		},
		activeStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "sess-1", StateType: "scene_state", Content: `{\"loc\":\"temple\"}`, TurnIndex: 7},
		},
		episodeSummaries: []store.EpisodeSummary{
			{ID: 1, ChatSessionID: "sess-1", FromTurn: 1, ToTurn: 10, SummaryText: "Alice arrives."},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	tests := []struct {
		path        string
		wantCode    int
		wantSource  string
		wantPresent []string
		wantAbsent  []string
	}{
		{
			path:        "/sessions/sess-1/guidance-snapshot",
			wantCode:    http.StatusOK,
			wantPresent: []string{"story_plan", "director", "compact_records", "maintenance_last", "last_turn"},
			wantAbsent:  []string{"source", "storyline_count", "trace_summary"},
		},
		{
			path:        "/sessions/sess-1/step7-health",
			wantCode:    http.StatusOK,
			wantPresent: []string{"total_turns", "guidance_state", "drift_summary", "compaction_summary", "maintenance_summary", "regression_checks"},
			wantAbsent:  []string{"source", "storyline_count", "trace_summary"},
		},
		{
			path:        "/session/sess-1/active-scope",
			wantCode:    http.StatusOK,
			wantSource:  "default",
			wantPresent: []string{"active_scope", "scope_chain", "updated_at"},
		},
		{
			path:        "/momentum-packet/sess-1",
			wantCode:    http.StatusOK,
			wantPresent: []string{"next_pressure", "payoff_candidates", "tension_to_reuse", "beats_to_avoid", "generated_at"},
			wantAbsent:  []string{"source", "storyline_count", "trace_summary"},
		},
		{
			path:        "/narrative-control/sess-1",
			wantCode:    http.StatusOK,
			wantPresent: []string{"story_plan", "director", "progression_ledger", "story_guidance", "generated_at"},
			wantAbsent:  []string{"source", "storyline_count", "trace_summary"},
		},
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
			if tt.wantSource != "" && resp["source"] != tt.wantSource {
				t.Errorf("source = %v, want %v", resp["source"], tt.wantSource)
			}
			for _, field := range tt.wantPresent {
				if _, ok := resp[field]; !ok {
					t.Errorf("missing field %s", field)
				}
			}
			for _, field := range tt.wantAbsent {
				if _, ok := resp[field]; ok {
					t.Errorf("unexpected field %s", field)
				}
			}
			if tt.path == "/session/sess-1/active-scope" && resp["active_scope"] != "root" {
				t.Errorf("active_scope = %v, want root", resp["active_scope"])
			}
		})
	}
}

func TestFiveNarrativeHandlersErrNotEnabledFallback(t *testing.T) {
	fake := &narrativeFakeStore{errNotEnabled: true}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	tests := []struct {
		path             string
		wantPacketStatus string
		wantStateStatus  string
	}{
		{path: "/sessions/sess-1/guidance-snapshot"},
		{path: "/sessions/sess-1/step7-health"},
		{path: "/session/sess-1/active-scope"},
		{path: "/momentum-packet/sess-1", wantPacketStatus: "empty"},
		{path: "/narrative-control/sess-1", wantStateStatus: "skeleton"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
			}
			var resp map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if resp["status"] != "ok" {
				t.Errorf("status = %v, want ok", resp["status"])
			}
			if tt.wantStateStatus != "" {
				if resp["state_status"] != tt.wantStateStatus {
					t.Errorf("state_status = %v, want %v", resp["state_status"], tt.wantStateStatus)
				}
			}
			if tt.wantPacketStatus != "" {
				if resp["packet_status"] != tt.wantPacketStatus {
					t.Errorf("packet_status = %v, want %v", resp["packet_status"], tt.wantPacketStatus)
				}
			}
		})
	}
}

func TestMetricsRoutesStoreBackedEvidence(t *testing.T) {
	fake := &narrativeFakeStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-1", TurnIndex: 1, Role: "user"},
			{ID: 2, ChatSessionID: "sess-1", TurnIndex: 2, Role: "assistant"},
		},
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-1", TurnIndex: 1},
		},
		evidence: []store.DirectEvidence{
			{ID: 1, ChatSessionID: "sess-1", EvidenceKind: "fact"},
		},
		kgTriples: []store.KGTriple{
			{ID: 1, ChatSessionID: "sess-1", Subject: "Alice", Predicate: "trusts", Object: "Bob"},
		},
		storylines: []store.Storyline{
			{ID: 1, ChatSessionID: "sess-1", Name: "Arc 1", Status: "active"},
		},
		worldRules: []store.WorldRule{
			{ID: 1, ChatSessionID: "sess-1", Scope: "global", Key: "rule"},
		},
		characterStates: []store.CharacterState{
			{ID: 1, ChatSessionID: "sess-1", CharacterName: "Alice"},
		},
		pendingThreads: []store.PendingThread{
			{ID: 1, ChatSessionID: "sess-1", ThreadKey: "hook-1", Status: "open"},
		},
		activeStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "sess-1", StateType: "scene_state"},
		},
		canonicalStateLayers: []store.CanonicalStateLayer{
			{ID: 1, ChatSessionID: "sess-1", LayerType: "scene_state"},
		},
		episodeSummaries: []store.EpisodeSummary{
			{ID: 1, ChatSessionID: "sess-1", FromTurn: 1, ToTurn: 2},
		},
		resumePack: &store.ResumePack{PackStatus: "ok", Trigger: "resume"},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	tests := []struct {
		path        string
		wantCount   string
		wantValue   float64
		wantPayload string
	}{
		{path: "/metrics/lc1d/sess-1", wantPayload: "integrity_replay"},
		{path: "/metrics/lc1e/sess-1", wantCount: "kg_triple_count", wantValue: 1},
		{path: "/metrics/lc1f/sess-1", wantCount: "storyline_count", wantValue: 1},
		{path: "/metrics/lc1g/sess-1", wantCount: "world_rule_count", wantValue: 1},
		{path: "/metrics/lc1h/sess-1", wantCount: "character_state_count", wantValue: 1},
		{path: "/metrics/lc1i/sess-1", wantCount: "pending_thread_count", wantValue: 1},
		{path: "/metrics/lc1j/sess-1", wantCount: "resume_pack_present", wantValue: 1},
		{path: "/metrics/lc1k/sess-1", wantCount: "memory_count", wantValue: 1},
		{path: "/metrics/lc1l/sess-1", wantCount: "evidence_count", wantValue: 1},
		{path: "/metrics/lc1m/sess-1", wantCount: "episode_summary_count", wantValue: 1},
		{path: "/metrics/lc1n/sess-1", wantCount: "active_state_count", wantValue: 1},
		{path: "/metrics/lc1o/sess-1", wantCount: "canonical_state_layer_count", wantValue: 1},
		{path: "/metrics/lc1p/sess-1", wantCount: "storyline_count", wantValue: 1},
		{path: "/metrics/lc1q/sess-1", wantPayload: "freshness_lag_summary"},
		{path: "/metrics/tm1d/sess-1", wantPayload: "truth_maintenance_audit_replay"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
			}
			var resp map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if resp["status"] != "ok" {
				t.Errorf("status = %v, want ok", resp["status"])
			}
			if tt.wantPayload != "" {
				if _, ok := resp[tt.wantPayload].(map[string]any); !ok {
					t.Fatalf("%s missing or wrong type: %#v", tt.wantPayload, resp[tt.wantPayload])
				}
				if _, ok := resp["counts"]; ok {
					t.Fatalf("unexpected counts in Python-compatible metric shape: %#v", resp)
				}
				return
			}
			if resp["store_status"] != "active" {
				t.Errorf("store_status = %v, want active", resp["store_status"])
			}
			counts, ok := resp["counts"].(map[string]any)
			if !ok {
				t.Fatalf("counts missing or wrong type: %#v", resp["counts"])
			}
			if counts[tt.wantCount] != tt.wantValue {
				t.Errorf("counts[%s] = %v, want %v", tt.wantCount, counts[tt.wantCount], tt.wantValue)
			}
		})
	}
}

func TestSeq17MetricsLC1PEvaluationSplitSurface(t *testing.T) {
	fake := &narrativeFakeStore{
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-lc1p", SummaryJSON: `{"text":"memory"}`},
		},
		evidence: []store.DirectEvidence{
			{ID: 1, ChatSessionID: "sess-lc1p", EvidenceText: "direct evidence", TurnAnchor: 2},
		},
		kgTriples: []store.KGTriple{
			{ID: 1, ChatSessionID: "sess-lc1p", Subject: "Alice", Predicate: "knows", Object: "Bob"},
		},
		episodeSummaries: []store.EpisodeSummary{
			{ID: 1, ChatSessionID: "sess-lc1p", FromTurn: 1, ToTurn: 3, SummaryText: "episode"},
		},
		activeStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "sess-lc1p", StateType: "scene", Content: "active"},
		},
		storylines: []store.Storyline{
			{ID: 1, ChatSessionID: "sess-lc1p", Name: "arc", Status: "active"},
		},
		pendingThreads: []store.PendingThread{
			{ID: 1, ChatSessionID: "sess-lc1p", ThreadKey: "hook", Status: "open"},
		},
		worldRules: []store.WorldRule{
			{ID: 1, ChatSessionID: "sess-lc1p", Scope: "global", Key: "rule"},
		},
		characterStates: []store.CharacterState{
			{ID: 1, ChatSessionID: "sess-lc1p", CharacterName: "Alice"},
		},
		canonicalStateLayers: []store.CanonicalStateLayer{
			{ID: 1, ChatSessionID: "sess-lc1p", LayerType: "scene"},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/metrics/lc1p/sess-lc1p", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	split, ok := resp["evaluation_split"].(map[string]any)
	if !ok {
		t.Fatalf("evaluation_split missing: %#v", resp)
	}
	retrieval, ok := resp["retrieval_completeness"].(map[string]any)
	if !ok {
		t.Fatalf("retrieval_completeness missing: %#v", resp)
	}
	quality, ok := resp["final_answer_quality"].(map[string]any)
	if !ok {
		t.Fatalf("final_answer_quality missing: %#v", resp)
	}
	failure, ok := resp["failure_split"].(map[string]any)
	if !ok {
		t.Fatalf("failure_split missing: %#v", resp)
	}
	if split["policy_version"] != "lc1p.v1" {
		t.Fatalf("policy_version=%v, want lc1p.v1", split["policy_version"])
	}
	if retrieval["policy_version"] != "s17-1a.v1" || retrieval["metric_defined"] != true {
		t.Fatalf("retrieval metric mismatch: %#v", retrieval)
	}
	if quality["policy_version"] != "s17-1b.v1" || quality["metric_defined"] != true {
		t.Fatalf("quality metric mismatch: %#v", quality)
	}
	if failure["policy_version"] != "s17-1c.v1" || failure["replay_defined"] != true {
		t.Fatalf("failure split mismatch: %#v", failure)
	}
	if failure["classification"] != "healthy" {
		t.Fatalf("classification=%v, want healthy", failure["classification"])
	}
}

func TestSeq17MetricsLC1QFreshnessLagAnswerQualitySplit(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = &narrativeFakeStore{}
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/metrics/lc1q/sess-lc1q", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	freshness, ok := resp["freshness_lag_summary"].(map[string]any)
	if !ok {
		t.Fatalf("freshness_lag_summary missing: %#v", resp)
	}
	if freshness["policy_version"] != "lc1q.v1" || freshness["metric_defined"] != true {
		t.Fatalf("freshness metric mismatch: %#v", freshness)
	}
	lags, ok := freshness["lags_seconds"].(map[string]any)
	if !ok {
		t.Fatalf("lags_seconds missing: %#v", freshness)
	}
	for _, key := range []string{"save_delay", "extraction_delay", "promotion_visibility_lag"} {
		if _, ok := lags[key]; !ok {
			t.Fatalf("lags_seconds[%s] missing: %#v", key, lags)
		}
	}
	qualitySplit, ok := freshness["answer_quality_split"].(map[string]any)
	if !ok {
		t.Fatalf("answer_quality_split missing: %#v", freshness)
	}
	if qualitySplit["extraction_delay_affects_answer_quality"] != true ||
		qualitySplit["save_delay_affects_answer_quality"] != true ||
		qualitySplit["promotion_visibility_lag_affects_answer_quality"] != true {
		t.Fatalf("answer_quality_split mismatch: %#v", qualitySplit)
	}
}

func TestMetricsLC1CMeasuresCanonicalDenseLedgerFootprint(t *testing.T) {
	chatLogs := make([]store.ChatLog, 300)
	for i := range chatLogs {
		chatLogs[i] = store.ChatLog{
			ID:            int64(i + 1),
			ChatSessionID: "sess-lc1c",
			TurnIndex:     i + 1,
			Role:          "assistant",
			Content:       "turn content",
		}
	}
	chapter := &store.ChapterSummary{SummaryText: "chapter dense summary", ResumeText: "chapter resume"}
	arc := &store.ArcSummary{CoreConflict: "arc conflict", ArcResumeText: "arc resume"}
	saga := &store.SagaDigest{SagaSummary: "saga summary", ResumePackText: "saga resume"}
	expectedDenseChars := len([]rune("episode dense summary")) +
		len([]rune(chapter.SummaryText)) + len([]rune(chapter.ResumeText)) +
		len([]rune(arc.CoreConflict)) + len([]rune(arc.ArcResumeText)) +
		len([]rune(saga.SagaSummary)) + len([]rune(saga.ResumePackText))
	expectedCanonicalChars := len([]rune(`{"scene_state":{"mood":"tense"}}`)) + len([]rune(`{"relationship_state":{"trust":"rising"}}`))

	fake := &narrativeFakeStore{
		chatLogs: chatLogs,
		canonicalStateLayers: []store.CanonicalStateLayer{
			{ID: 1, ChatSessionID: "sess-lc1c", LayerType: "scene_state", Content: `{"scene_state":{"mood":"tense"}}`, TurnIndex: 299},
			{ID: 2, ChatSessionID: "sess-lc1c", LayerType: "relationship_state", Content: `{"relationship_state":{"trust":"rising"}}`, TurnIndex: 300},
		},
		episodeSummaries: []store.EpisodeSummary{
			{ID: 1, ChatSessionID: "sess-lc1c", FromTurn: 1, ToTurn: 300, SummaryText: "episode dense summary"},
		},
		resumePack: &store.ResumePack{PackStatus: "ok", Trigger: "resume", Chapter: chapter, Arc: arc, Saga: saga},
		storylines: []store.Storyline{
			{ID: 1, ChatSessionID: "sess-lc1c", Name: "Sealed ledger", Status: "active", CurrentContext: "The ledger remains dangerous.", LastTurn: 300, Confidence: 0.9},
		},
		worldRules: []store.WorldRule{
			{ID: 1, ChatSessionID: "sess-lc1c", Scope: "session", Category: "world", Key: "ledger_seal", ValueJSON: `{"rule":"Seal changes require evidence."}`, SourceTurn: 280},
		},
		pendingThreads: []store.PendingThread{
			{ID: 1, ChatSessionID: "sess-lc1c", ThreadKey: "ledger-payoff", Status: "open", Description: "Pay off the ledger promise", SourceTurn: 295},
		},
		activeStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "sess-lc1c", StateType: "scene_state", Content: `{"pressure":"high"}`, TurnIndex: 300},
		},
		characterStates: []store.CharacterState{
			{ID: 1, ChatSessionID: "sess-lc1c", CharacterName: "Mina", StatusJSON: `{"summary":"Mina protects the ledger"}`, TurnIndex: 300},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/metrics/lc1c/sess-lc1c", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	mfp, ok := resp["memory_footprint"].(map[string]any)
	if !ok {
		t.Fatalf("memory_footprint missing or wrong type: %#v", resp["memory_footprint"])
	}
	if mfp["policy_version"] != "lc1c.v1" {
		t.Fatalf("policy_version = %v, want lc1c.v1", mfp["policy_version"])
	}
	if mfp["turn_window"] != float64(300) || mfp["latest_turn_index"] != float64(300) || mfp["window_start_turn"] != float64(1) {
		t.Fatalf("window metrics mismatch: %#v", mfp)
	}
	if mfp["canonical_state_chars"] != float64(expectedCanonicalChars) {
		t.Fatalf("canonical_state_chars = %v, want %d", mfp["canonical_state_chars"], expectedCanonicalChars)
	}
	if mfp["dense_summary_chars"] != float64(expectedDenseChars) {
		t.Fatalf("dense_summary_chars = %v, want %d", mfp["dense_summary_chars"], expectedDenseChars)
	}
	if live := mfp["live_ledger_chars"].(float64); live <= 0 {
		t.Fatalf("live_ledger_chars = %v, want > 0", live)
	}
	total := mfp["total_chars"].(float64)
	if total <= float64(expectedCanonicalChars+expectedDenseChars) {
		t.Fatalf("total_chars = %v, want canonical+dense+ledger footprint", total)
	}
	counts, ok := mfp["counts"].(map[string]any)
	if !ok {
		t.Fatalf("counts missing or wrong type: %#v", mfp["counts"])
	}
	wantCounts := map[string]float64{
		"canonical_layers": 2,
		"episodes":         1,
		"chapters":         1,
		"arcs":             1,
		"sagas":            1,
	}
	for key, want := range wantCounts {
		if counts[key] != want {
			t.Fatalf("counts[%s] = %v, want %v", key, counts[key], want)
		}
	}
}

func TestMetricsLC1DIntegrityReplayRetainsImportantLongMemory(t *testing.T) {
	fake := &narrativeFakeStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-lc1d", TurnIndex: 700, Role: "assistant", Content: "latest turn"},
		},
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-lc1d", TurnIndex: 50, Importance: 0.95, SummaryJSON: `{"summary":"old promise stays important"}`},
			{ID: 2, ChatSessionID: "sess-lc1d", TurnIndex: 350, NarrativeSignificance: 0.82, SummaryJSON: `{"summary":"middle arc still matters"}`},
			{ID: 3, ChatSessionID: "sess-lc1d", TurnIndex: 650, Importance: 0.99, SummaryJSON: `{"summary":"recent memory is not long-range"}`},
			{ID: 4, ChatSessionID: "sess-lc1d", TurnIndex: 20, Importance: 0.2, SummaryJSON: `{"summary":"old low-priority detail"}`},
		},
		evidence: []store.DirectEvidence{
			{ID: 1, ChatSessionID: "sess-lc1d", EvidenceText: "The old oath is verified.", TurnAnchor: 80, ArchiveState: "verified_direct", CaptureVerification: "verified"},
			{ID: 2, ChatSessionID: "sess-lc1d", EvidenceText: "Repair item should not count.", TurnAnchor: 90, ArchiveState: "repair_queue", RepairNeeded: true},
		},
		canonicalStateLayers: []store.CanonicalStateLayer{
			{ID: 1, ChatSessionID: "sess-lc1d", LayerType: "scene_state", Content: `{"mood":"charged"}`, SourceTurn: 60},
		},
		episodeSummaries: []store.EpisodeSummary{
			{ID: 1, ChatSessionID: "sess-lc1d", FromTurn: 1, ToTurn: 100, SummaryText: "dense early episode"},
		},
		resumePack: &store.ResumePack{
			PackStatus: "ok",
			Trigger:    "resume",
			Chapter:    &store.ChapterSummary{SummaryText: "chapter memory"},
			Arc:        &store.ArcSummary{CoreConflict: "arc memory"},
			Saga:       &store.SagaDigest{SagaSummary: "saga memory"},
		},
		storylines: []store.Storyline{
			{ID: 1, ChatSessionID: "sess-lc1d", Name: "Old oath", Status: "active", CurrentContext: "The oath remains unpaid.", LastEvidenceTurn: 100, EvidenceCount: 2},
		},
		pendingThreads: []store.PendingThread{
			{ID: 1, ChatSessionID: "sess-lc1d", ThreadKey: "oath", Status: "open", Description: "Resolve the old oath", CreatedTurn: 40, SourceTurn: 40},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/metrics/lc1d/sess-lc1d", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	replay, ok := resp["integrity_replay"].(map[string]any)
	if !ok {
		t.Fatalf("integrity_replay missing or wrong type: %#v", resp["integrity_replay"])
	}
	if replay["policy_version"] != "lc1d.v1" || replay["replay_query_source"] != "query_independent_store_replay" {
		t.Fatalf("replay policy/source mismatch: %#v", replay)
	}
	if replay["latest_turn_index"] != float64(700) || replay["candidates_total"] != float64(3) || replay["retained_total"] != float64(3) || replay["gaps_total"] != float64(0) {
		t.Fatalf("candidate retention mismatch: %#v", replay)
	}
	if replay["retention_rate"] != float64(1) || replay["scanned_direct_evidence_rows"] != float64(2) {
		t.Fatalf("retention/scanned mismatch: %#v", replay)
	}
	scopeCounts, ok := replay["scope_counts"].(map[string]any)
	if !ok {
		t.Fatalf("scope_counts missing or wrong type: %#v", replay["scope_counts"])
	}
	if scopeCounts["long"] != float64(3) || scopeCounts["ultra_long"] != float64(2) {
		t.Fatalf("scope_counts mismatch: %#v", scopeCounts)
	}
	retained, ok := replay["retained_by_layer"].(map[string]any)
	if !ok {
		t.Fatalf("retained_by_layer missing or wrong type: %#v", replay["retained_by_layer"])
	}
	wantLayers := map[string]float64{
		"memory":          2,
		"direct_evidence": 1,
		"canonical":       1,
		"dense_summary":   4,
		"live_ledger":     2,
	}
	for key, want := range wantLayers {
		if retained[key] != want {
			t.Fatalf("retained_by_layer[%s] = %v, want %v in %#v", key, retained[key], want, retained)
		}
	}
	examples, ok := replay["candidate_examples"].([]any)
	if !ok || len(examples) != 3 {
		t.Fatalf("candidate_examples = %#v, want 3 retained examples", replay["candidate_examples"])
	}
}

func TestMetricsLC1EComparesHypaMemoryAlwaysOnBudget(t *testing.T) {
	largeHypaSummary := strings.Repeat("HypaMemory imported summary. ", 500)
	fake := &narrativeFakeStore{
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-lc1e", TurnIndex: 1, SummaryJSON: largeHypaSummary, Evidence: largeHypaSummary, PlaceWing: "hypamemory"},
		},
		evidence: []store.DirectEvidence{
			{ID: 1, ChatSessionID: "sess-lc1e", EvidenceText: "Hypa imported evidence", LineageJSON: `{"source":"HypaMemory import"}`, CaptureStage: "hypamemory_import"},
		},
		kgTriples: []store.KGTriple{
			{ID: 1, ChatSessionID: "sess-lc1e", Subject: "Mina", Predicate: "remembers", Object: "oath"},
		},
		canonicalStateLayers: []store.CanonicalStateLayer{
			{ID: 1, ChatSessionID: "sess-lc1e", LayerType: "scene_state", Content: `{"mood":"focused"}`, SourceTurn: 1},
		},
		episodeSummaries: []store.EpisodeSummary{
			{ID: 1, ChatSessionID: "sess-lc1e", FromTurn: 1, ToTurn: 10, SummaryText: "short dense summary"},
		},
		resumePack: &store.ResumePack{PackStatus: "ok", Trigger: "resume"},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/metrics/lc1e/sess-lc1e", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	compare, ok := resp["budget_compare"].(map[string]any)
	if !ok {
		t.Fatalf("budget_compare missing or wrong type: %#v", resp["budget_compare"])
	}
	if compare["policy_version"] != "lc1e.v1" || compare["hypamemory_always_on_mode"] != "discouraged_after_import" {
		t.Fatalf("budget policy mismatch: %#v", compare)
	}
	hypaChars := compare["hypamemory_always_on_chars"].(float64)
	layeredChars := compare["archive_center_layered_chars"].(float64)
	if hypaChars <= layeredChars {
		t.Fatalf("expected layered budget to be smaller than always-on HypaMemory, hypa=%v layered=%v", hypaChars, layeredChars)
	}
	if compare["recommended_mode"] != "archive_center_layered" {
		t.Fatalf("recommended_mode = %v, want archive_center_layered", compare["recommended_mode"])
	}
	if compare["saved_chars_vs_hypamemory"].(float64) <= 0 || compare["savings_ratio"].(float64) <= 0 {
		t.Fatalf("budget savings not positive: %#v", compare)
	}
	counts, ok := resp["counts"].(map[string]any)
	if !ok {
		t.Fatalf("counts missing or wrong type: %#v", resp["counts"])
	}
	if counts["memory_count"] != float64(1) || counts["kg_triple_count"] != float64(1) || counts["evidence_count"] != float64(1) {
		t.Fatalf("counts mismatch: %#v", counts)
	}
	trace, ok := resp["trace_summary"].([]any)
	if !ok || len(trace) == 0 {
		t.Fatalf("trace_summary missing: %#v", resp["trace_summary"])
	}
}

func TestMetricsLC1FConfirmsShortMidRegressionLayers(t *testing.T) {
	fake := &narrativeFakeStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-lc1f", TurnIndex: 12, Role: "assistant", Content: "latest"},
		},
		evidence: []store.DirectEvidence{
			{ID: 1, ChatSessionID: "sess-lc1f", EvidenceText: "Verified fact", TurnAnchor: 12, ArchiveState: "verified_direct", CaptureVerification: "verified"},
		},
		kgTriples: []store.KGTriple{
			{ID: 1, ChatSessionID: "sess-lc1f", Subject: "Mina", Predicate: "protects", Object: "ledger"},
		},
		activeStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "sess-lc1f", StateType: "scene_state", Content: `{"mood":"tense"}`, TurnIndex: 12},
		},
		canonicalStateLayers: []store.CanonicalStateLayer{
			{ID: 1, ChatSessionID: "sess-lc1f", LayerType: "scene_state", Content: `{"mood":"tense"}`, SourceTurn: 12},
		},
		characterStates: []store.CharacterState{
			{ID: 1, ChatSessionID: "sess-lc1f", CharacterName: "Mina", StatusJSON: `{"intent":"protect ledger"}`, TurnIndex: 12},
		},
		storylines: []store.Storyline{
			{ID: 1, ChatSessionID: "sess-lc1f", Name: "Ledger oath", Status: "active", CurrentContext: "Oath remains active.", LastTurn: 12},
		},
		episodeSummaries: []store.EpisodeSummary{
			{ID: 1, ChatSessionID: "sess-lc1f", FromTurn: 1, ToTurn: 12, SummaryText: "Episode keeps the oath."},
		},
		worldRules: []store.WorldRule{
			{ID: 1, ChatSessionID: "sess-lc1f", Scope: "session", Category: "world", Key: "ledger_oath", ValueJSON: `{"rule":"Oaths require payoff."}`},
		},
		pendingThreads: []store.PendingThread{
			{ID: 1, ChatSessionID: "sess-lc1f", ThreadKey: "oath", Status: "open", Description: "Pay off the oath", SourceTurn: 12},
		},
		resumePack: &store.ResumePack{PackStatus: "ok", Trigger: "resume"},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/metrics/lc1f/sess-lc1f", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	confirm, ok := resp["regression_confirm"].(map[string]any)
	if !ok {
		t.Fatalf("regression_confirm missing or wrong type: %#v", resp["regression_confirm"])
	}
	if confirm["policy_version"] != "lc1f.v1" || confirm["status"] != "pass" {
		t.Fatalf("regression confirm mismatch: %#v", confirm)
	}
	failed, ok := confirm["failed_checks"].([]any)
	if !ok || len(failed) != 0 {
		t.Fatalf("failed_checks = %#v, want empty", confirm["failed_checks"])
	}
	shortTerm, ok := confirm["short_term"].(map[string]any)
	if !ok {
		t.Fatalf("short_term missing: %#v", confirm["short_term"])
	}
	for _, key := range []string{"chat_logs_present", "direct_evidence_present", "kg_present", "current_state_present"} {
		if shortTerm[key] != true {
			t.Fatalf("short_term[%s] = %v, want true in %#v", key, shortTerm[key], shortTerm)
		}
	}
	midTerm, ok := confirm["mid_term"].(map[string]any)
	if !ok {
		t.Fatalf("mid_term missing: %#v", confirm["mid_term"])
	}
	for _, key := range []string{"storyline_present", "episode_summary_present", "world_rule_present", "pending_thread_present", "resume_pack_present"} {
		if midTerm[key] != true {
			t.Fatalf("mid_term[%s] = %v, want true in %#v", key, midTerm[key], midTerm)
		}
	}
}

func TestMetricsLC1GThroughLC1OExposeReplayAndGateEvidence(t *testing.T) {
	fake := &narrativeFakeStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-lc-tail", TurnIndex: 720, Role: "assistant", Content: "latest"},
		},
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-lc-tail", TurnIndex: 50, Importance: 0.9, SummaryJSON: `{"source":"hypamemory","summary":"imported idea"}`, PlaceWing: "hypamemory"},
		},
		evidence: []store.DirectEvidence{
			{ID: 1, ChatSessionID: "sess-lc-tail", EvidenceText: "Verified old promise.", TurnAnchor: 600, ArchiveState: "verified_direct", CaptureVerification: "verified", LineageJSON: `{"source":"HypaMemory import"}`, CaptureStage: "hypamemory_import"},
		},
		kgTriples: []store.KGTriple{
			{ID: 1, ChatSessionID: "sess-lc-tail", Subject: "Mina", Predicate: "guards", Object: "ledger"},
		},
		canonicalStateLayers: []store.CanonicalStateLayer{
			{ID: 1, ChatSessionID: "sess-lc-tail", LayerType: "scene_state", Content: `{"mood":"tense"}`, SourceTurn: 700, Confidence: 0.9},
			{ID: 2, ChatSessionID: "sess-lc-tail", LayerType: "relationship_state", Content: `{"trust":"rising"}`, SourceTurn: 700, Confidence: 0.88},
		},
		activeStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "sess-lc-tail", StateType: "scene_state", Content: `{"pressure":"high"}`, TurnIndex: 720},
		},
		characterStates: []store.CharacterState{
			{ID: 1, ChatSessionID: "sess-lc-tail", CharacterName: "Mina", RelationshipsJSON: `{"Rowan":"trusted"}`, TurnIndex: 720},
		},
		storylines: []store.Storyline{
			{ID: 1, ChatSessionID: "sess-lc-tail", Name: "Ledger oath", Status: "active", CurrentContext: "The oath is still active.", LastEvidenceTurn: 650, EvidenceCount: 2, Confidence: 0.9},
		},
		worldRules: []store.WorldRule{
			{ID: 1, ChatSessionID: "sess-lc-tail", Scope: "session", Category: "world", Key: "ledger_oath", ValueJSON: `{"rule":"Oaths need payoff."}`, SourceTurn: 650},
		},
		pendingThreads: []store.PendingThread{
			{ID: 1, ChatSessionID: "sess-lc-tail", ThreadKey: "oath", Status: "open", Description: "Pay off the oath", CreatedTurn: 650, SourceTurn: 650},
		},
		episodeSummaries: []store.EpisodeSummary{
			{ID: 1, ChatSessionID: "sess-lc-tail", FromTurn: 600, ToTurn: 720, SummaryText: "Episode holds the oath."},
		},
		resumePack: &store.ResumePack{PackStatus: "ok", Trigger: "resume"},
		auditLogs: []store.AuditLog{
			{ID: 1, ChatSessionID: "sess-lc-tail", EventType: "critic_pipeline_trace", Summary: "split pipeline ok", DetailsJSON: `{"status":"ok"}`},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	cases := []struct {
		path       string
		payloadKey string
		policy     string
	}{
		{"/metrics/lc1g/sess-lc-tail", "promotion_replay", "lc1g.v1"},
		{"/metrics/lc1h/sess-lc-tail", "false_negative_positive_replay", "lc1h.v1"},
		{"/metrics/lc1i/sess-lc-tail", "recall_ablation_compare", "lc1i.v1"},
		{"/metrics/lc1j/sess-lc-tail", "verification_gate", "lc1j.v1"},
		{"/metrics/lc1k/sess-lc-tail", "priority_budget_trace", "lc1k.v1"},
		{"/metrics/lc1l/sess-lc-tail", "imported_idea_contract_gate", "lc1l.v1"},
		{"/metrics/lc1m/sess-lc-tail", "split_pipeline_compare", "lc1m.v1"},
		{"/metrics/lc1n/sess-lc-tail", "rebuild_backfill_replay", "lc1n.v1"},
		{"/metrics/lc1o/sess-lc-tail", "deterministic_preview_ledger", "lc1o.v1"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
			}
			var resp map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			payload, ok := resp[tc.payloadKey].(map[string]any)
			if !ok {
				t.Fatalf("%s missing or wrong type: %#v", tc.payloadKey, resp[tc.payloadKey])
			}
			if payload["policy_version"] != tc.policy {
				t.Fatalf("%s policy_version = %v, want %s: %#v", tc.payloadKey, payload["policy_version"], tc.policy, payload)
			}
			switch tc.payloadKey {
			case "promotion_replay":
				if payload["status"] != "pass" || payload["verified_promotion_count"].(float64) < 2 {
					t.Fatalf("promotion replay mismatch: %#v", payload)
				}
			case "false_negative_positive_replay":
				if payload["status"] != "pass" || payload["false_negative_risk_count"] != float64(0) || payload["false_positive_risk_count"] != float64(0) {
					t.Fatalf("false negative/positive replay mismatch: %#v", payload)
				}
			case "recall_ablation_compare":
				if payload["relationship_v2_signal_count"].(float64) <= 0 || payload["ledger_signal_count"].(float64) <= 0 || payload["world_pressure_signal_count"].(float64) <= 0 {
					t.Fatalf("ablation signal counts missing: %#v", payload)
				}
			case "verification_gate":
				if payload["status"] != "pass" || payload["release_gate_ready"] != true || payload["default_runtime_takeover"] != false {
					t.Fatalf("verification gate mismatch: %#v", payload)
				}
			case "priority_budget_trace":
				if payload["lower_tier_support_preserved"] != true || payload["high_priority_layer_count"].(float64) <= 0 {
					t.Fatalf("priority budget trace mismatch: %#v", payload)
				}
			case "imported_idea_contract_gate":
				if payload["default_takeover_blocked"] != true || payload["imported_signal_count"].(float64) <= 0 {
					t.Fatalf("imported idea gate mismatch: %#v", payload)
				}
			case "split_pipeline_compare":
				if payload["split_pipeline_enabled"] != true || payload["single_call_mode"] != false || payload["critic_pipeline_trace_count"].(float64) != 1 {
					t.Fatalf("split pipeline compare mismatch: %#v", payload)
				}
			case "rebuild_backfill_replay":
				if payload["status"] != "pass" || payload["drift_detected"] != false {
					t.Fatalf("backfill replay mismatch: %#v", payload)
				}
			case "deterministic_preview_ledger":
				if payload["llm_call_required"] != false || payload["preview_path"] != "deterministic" || payload["world_pressure_ready"] != true {
					t.Fatalf("deterministic preview ledger mismatch: %#v", payload)
				}
			}
		})
	}
}

func TestMetricsRoutesErrNotEnabledFallback(t *testing.T) {
	fake := &narrativeFakeStore{errNotEnabled: true}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/metrics/lc1d/sess-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := resp["integrity_replay"].(map[string]any); !ok {
		t.Fatalf("integrity_replay missing or wrong type: %#v", resp["integrity_replay"])
	}
	if _, ok := resp["counts"]; ok {
		t.Fatalf("unexpected counts in Python-compatible metric shape: %#v", resp)
	}
}

func TestRemainingReadPlaceholdersStoreBackedEvidence(t *testing.T) {
	now := time.Now().UTC()
	fake := &narrativeFakeStore{
		sessions: []store.SessionSummary{
			{ChatSessionID: "sess-1"},
		},
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-1", TurnIndex: 1, Role: "user"},
		},
		memories: []store.Memory{
			{ID: 1, ChatSessionID: "sess-1", TurnIndex: 1},
		},
		evidence: []store.DirectEvidence{
			{ID: 1, ChatSessionID: "sess-1", EvidenceText: "Alice entered the archive."},
		},
		kgTriples: []store.KGTriple{
			{ID: 1, ChatSessionID: "sess-1", Subject: "Alice", Predicate: "entered", Object: "archive"},
		},
		storylines: []store.Storyline{
			{ID: 1, ChatSessionID: "sess-1", Name: "Archive arrival", Status: "active", CurrentContext: "Archive arrival", KeyPointsJSON: `["arc"]`, OngoingTensionsJSON: `["answer pending"]`, Confidence: 0.8, EvidenceCount: 2, LastEvidenceTurn: 8, FirstTurn: 4, LastTurn: 8, UpdatedAt: now},
		},
		worldRules: []store.WorldRule{
			{ID: 1, ChatSessionID: "sess-1", Key: "archive_rules"},
		},
		characterStates: []store.CharacterState{
			{ID: 1, ChatSessionID: "sess-1", CharacterName: "Alice"},
		},
		pendingThreads: []store.PendingThread{
			{ID: 1, ChatSessionID: "sess-1", ThreadKey: "door", Status: "open"},
		},
		activeStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "sess-1", StateType: "scene_state"},
		},
		canonicalStateLayers: []store.CanonicalStateLayer{
			{ID: 1, ChatSessionID: "sess-1", LayerType: "scene_state"},
		},
		episodeSummaries: []store.EpisodeSummary{
			{ID: 1, ChatSessionID: "sess-1", FromTurn: 1, ToTurn: 8, SummaryText: "Alice explores the archive.", KeyEntities: "Alice"},
			{ID: 2, ChatSessionID: "sess-1", FromTurn: 9, ToTurn: 12, SummaryText: "Bob waits outside.", KeyEntities: "Bob"},
		},
		resumePack: &store.ResumePack{
			PackStatus: "ok",
			Chapter: &store.ChapterSummary{
				ID:           10,
				FromTurn:     1,
				ToTurn:       12,
				ChapterTitle: "Archive Gate",
				SummaryText:  "Alice explores the archive.",
			},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	getTests := []string{
		"/sessions/compare?session_ids=sess-1,sess-2&preview_limit=5",
		"/metrics/lc1r/regression-corpus?limit=5",
	}
	for _, path := range getTests {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
			}
			var resp map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if path == "/metrics/lc1r/regression-corpus?limit=5" {
				if _, ok := resp["regression_corpus_manifest"].(map[string]any); !ok {
					t.Fatalf("regression_corpus_manifest missing: %#v", resp)
				}
				return
			}
			if resp["status"] != "ok" {
				t.Fatalf("status = %v, want ok", resp["status"])
			}
			if _, ok := resp["store_status"]; ok {
				t.Fatalf("store_status should be omitted on Python-compatible compare response: %#v", resp)
			}
		})
	}

	postTests := []struct {
		path      string
		body      string
		wantCount float64
	}{
		{path: "/chapters/dry-run", body: `{"chat_session_id":"sess-1","turn_index":60,"interval":60,"limit":5}`, wantCount: -1},
		{path: "/chapters/search", body: `{"chat_session_id":"sess-1","query":"archive","limit":5}`, wantCount: 2},
		{path: "/episodes/search", body: `{"chat_session_id":"sess-1","query":"Alice","limit":5}`, wantCount: 1},
	}
	for _, tt := range postTests {
		t.Run(tt.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
			}
			var resp map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if _, ok := resp["store_status"]; ok {
				t.Fatalf("store_status should be omitted on Python-compatible response: %#v", resp)
			}
			if tt.wantCount >= 0 && resp["count"] != tt.wantCount {
				t.Fatalf("count = %v, want %v", resp["count"], tt.wantCount)
			}
		})
	}
}

func TestRemainingReadPlaceholdersErrNotEnabledFallback(t *testing.T) {
	fake := &narrativeFakeStore{errNotEnabled: true}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/sessions/compare", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "error" || resp["detail"] != "At least 2 session_ids are required." {
		t.Fatalf("unexpected compare fallback response: %#v", resp)
	}
}

func TestSessionResumePackIncludesGuidanceSnapshot(t *testing.T) {
	spJSON, _ := json.Marshal(map[string]any{"current_arc": "Resume Arc"})
	dirJSON, _ := json.Marshal(map[string]any{"scene_mandate": "Resume carefully"})
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = &narrativeFakeStore{
		resumePack: &store.ResumePack{
			PackStatus:    "ready",
			Trigger:       "resume",
			SourcesUsed:   []string{"chapter", "arc"},
			LayerCount:    2,
			AssembledText: "Resume from the rooftop arc.",
			AssemblyNote:  "store-backed resume",
		},
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-1",
			StoryPlanJSON: string(spJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "ready",
			LastTurn:      8,
		},
	}
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/sessions/sess-1/resume-pack?continuity_trigger_mode=resume", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("status = %v, want ok", resp["status"])
	}
	pack, ok := resp["resume_pack"].(map[string]any)
	if !ok {
		t.Fatalf("resume_pack is not an object: %#v", resp)
	}
	if pack["pack_status"] != "ready" || pack["assembled_text"] != "Resume from the rooftop arc." {
		t.Fatalf("resume_pack mismatch: %#v", pack)
	}
	gs, ok := resp["guidance_snapshot"].(map[string]any)
	if !ok {
		t.Fatalf("guidance_snapshot missing: %#v", resp)
	}
	if gs["state_status"] != "active" || gs["last_turn"] != float64(8) {
		t.Fatalf("guidance_snapshot mismatch: %#v", gs)
	}
}

func TestChapterGenerateWritesDeterministicChapterSummary(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Cfg.StoreMode = config.StoreModeMariaDBAuthority
	fake := &narrativeFakeStore{
		episodeSummaries: []store.EpisodeSummary{
			{ChatSessionID: "sess-chg", FromTurn: 1, ToTurn: 15, SummaryText: "Alice enters the archive.", KeyEntities: "Alice"},
			{ChatSessionID: "sess-chg", FromTurn: 16, ToTurn: 30, SummaryText: "Bob finds a sealed ledger.", KeyEntities: "Bob"},
			{ChatSessionID: "sess-chg", FromTurn: 31, ToTurn: 45, SummaryText: "The tower rule changes.", KeyEntities: "rule"},
			{ChatSessionID: "sess-chg", FromTurn: 46, ToTurn: 60, SummaryText: "Alice chooses to stay.", KeyEntities: "choice"},
		},
	}
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chapters/generate", strings.NewReader(`{"chat_session_id":"sess-chg","turn_index":60,"interval":60}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedChapterSummaries) != 1 {
		t.Fatalf("saved chapters = %d, want 1", len(fake.savedChapterSummaries))
	}
	saved := fake.savedChapterSummaries[0]
	if saved.ChatSessionID != "sess-chg" || saved.FromTurn != 1 || saved.ToTurn != 60 {
		t.Fatalf("saved chapter range/session mismatch: %+v", saved)
	}
	if !strings.Contains(saved.SummaryText, "Alice enters the archive") || !strings.Contains(saved.ResumeText, "Turns 1-60") {
		t.Fatalf("saved chapter text incomplete: %+v", saved)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["generation_source"] != "deterministic_migration_stub" {
		t.Fatalf("generation_source = %v", resp["generation_source"])
	}
	if resp["llm_attempted"] != false || resp["saved"] != true {
		t.Fatalf("unexpected generation flags: %#v", resp)
	}
}

func TestChapterGenerateUsesConfiguredLLMWhenAvailable(t *testing.T) {
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("upstream path = %s, want /v1/chat/completions", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("upstream body decode: %v", err)
		}
		if body["model"] != "chapter-model" {
			t.Fatalf("upstream model = %v", body["model"])
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"model": "chapter-model",
			"choices": []any{
				map[string]any{
					"message": map[string]any{
						"content": `{"chapter_title":"LLM Gate","summary_text":"LLM summary keeps the archive gate callback.","open_loops":["gate"],"relationship_changes":["Alice trusts Bob"],"world_changes":["archive gate opens"],"callback_candidates":["sealed ledger"],"resume_text":"LLM resume for turns 1-60."}`,
					},
				},
			},
			"usage": map[string]any{"total_tokens": 77},
		})
	}))
	defer upstream.Close()

	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv.updateRuntimeConfig(map[string]any{
		"mainProvider": "openai",
		"mainApiKey":   "sk-chapter-test",
		"mainEndpoint": upstream.URL,
		"mainModel":    "chapter-model",
		"mainTimeout":  5,
	})
	fake := &narrativeFakeStore{
		episodeSummaries: []store.EpisodeSummary{
			{ChatSessionID: "sess-chg-llm", FromTurn: 1, ToTurn: 15, SummaryText: "Alice enters the archive.", KeyEntities: "Alice"},
			{ChatSessionID: "sess-chg-llm", FromTurn: 16, ToTurn: 30, SummaryText: "Bob finds a sealed ledger.", KeyEntities: "Bob"},
			{ChatSessionID: "sess-chg-llm", FromTurn: 31, ToTurn: 45, SummaryText: "The tower rule changes.", KeyEntities: "rule"},
			{ChatSessionID: "sess-chg-llm", FromTurn: 46, ToTurn: 60, SummaryText: "Alice chooses to stay.", KeyEntities: "choice"},
		},
	}
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chapters/generate", strings.NewReader(`{"chat_session_id":"sess-chg-llm","turn_index":60,"interval":60}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if calls != 1 {
		t.Fatalf("upstream calls = %d, want 1", calls)
	}
	if len(fake.savedChapterSummaries) != 1 {
		t.Fatalf("saved chapters = %d, want 1", len(fake.savedChapterSummaries))
	}
	saved := fake.savedChapterSummaries[0]
	if saved.ChapterTitle != "LLM Gate" || saved.SummaryText != "LLM summary keeps the archive gate callback." || saved.ResumeText != "LLM resume for turns 1-60." {
		t.Fatalf("saved chapter did not use LLM JSON: %+v", saved)
	}
	if !strings.Contains(saved.OpenLoopsJSON, "gate") || !strings.Contains(saved.WorldChangesJSON, "archive gate opens") {
		t.Fatalf("saved chapter JSON fields incomplete: %+v", saved)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["generation_source"] != "configured_llm" || resp["llm_attempted"] != true {
		t.Fatalf("unexpected LLM generation flags: %#v", resp)
	}
	shadow, ok := resp["chapter_shadow_compare"].(map[string]any)
	if !ok || shadow["enabled"] != true || shadow["summary_diverged"] != true {
		t.Fatalf("chapter_shadow_compare missing/divergence not recorded: %#v", resp)
	}
}

func TestChapterSearchUsesChapterSummaryStoreBeforeFallback(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = &narrativeFakeStore{
		chapterSummaries: []store.ChapterSummary{
			{
				ID:            7,
				ChatSessionID: "sess-search",
				FromTurn:      1,
				ToTurn:        60,
				ChapterIndex:  1,
				ChapterTitle:  "Archive Gate",
				SummaryText:   "Alice studies the sealed archive gate.",
				ResumeText:    "Archive gate callback is active.",
			},
		},
		episodeSummaries: []store.EpisodeSummary{
			{ID: 99, ChatSessionID: "sess-search", FromTurn: 1, ToTurn: 10, SummaryText: "Archive fallback episode."},
		},
	}
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chapters/search", strings.NewReader(`{"chat_session_id":"sess-search","query":"gate","limit":5}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := resp["chapters"].([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("chapters missing: %#v", resp)
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("first chapter item shape = %#v", items[0])
	}
	if first["source"] != "chapter_summary" {
		t.Fatalf("source = %v, want chapter_summary", first["source"])
	}
	if first["chapter_title"] != "Archive Gate" {
		t.Fatalf("chapter_title = %v, want Archive Gate", first["chapter_title"])
	}
}

func TestChapterSearchIncludesDS1fSourceAnchors(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = &narrativeFakeStore{
		chapterSummaries: []store.ChapterSummary{
			{ID: 7, ChatSessionID: "sess-ds1f", FromTurn: 1, ToTurn: 60, ChapterIndex: 1, ChapterTitle: "Gate", SummaryText: "Alice opens the gate.", ResumeText: "Gate opened.", OpenLoopsJSON: `["loop1"]`, RelationshipChangesJSON: `["rel1"]`, WorldChangesJSON: `["world1"]`, CallbackCandidatesJSON: `["cb1"]`},
		},
	}
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/chapters/search", strings.NewReader(`{"chat_session_id":"sess-ds1f","query":"gate"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items := resp["chapters"].([]any)
	first := items[0].(map[string]any)
	if first["source_record_id"] != float64(7) {
		t.Fatalf("source_record_id = %v, want 7", first["source_record_id"])
	}
	if first["source_record_type"] != "chapter" {
		t.Fatalf("source_record_type = %v, want chapter", first["source_record_type"])
	}
	if first["dense_source_anchor_policy_version"] != "ds1f.v1" {
		t.Fatalf("dense_source_anchor_policy_version = %v", first["dense_source_anchor_policy_version"])
	}
}

func TestEpisodeSearchIncludesDS1gRetentionFields(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = &narrativeFakeStore{
		episodeSummaries: []store.EpisodeSummary{
			{ID: 1, ChatSessionID: "sess-ds1g", FromTurn: 1, ToTurn: 10, SummaryText: "Alice trusts Bob.", RelationshipChangesJSON: `["Alice trusts Bob"]`},
		},
	}
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/episodes/search", strings.NewReader(`{"chat_session_id":"sess-ds1g","query":"alice"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items := resp["episodes"].([]any)
	first := items[0].(map[string]any)
	if first["dense_retention_policy_version"] != "ds1g.v1" {
		t.Fatalf("dense_retention_policy_version = %v", first["dense_retention_policy_version"])
	}
	if first["dense_retention_applied"] != true {
		t.Fatalf("dense_retention_applied = %v, want true", first["dense_retention_applied"])
	}
	if first["dense_retention_reason"] != "important_fact_retention" {
		t.Fatalf("dense_retention_reason = %v", first["dense_retention_reason"])
	}
}

func TestChapterSearchIncludesDS1hRoleSplitFields(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = &narrativeFakeStore{
		chapterSummaries: []store.ChapterSummary{
			{ID: 3, ChatSessionID: "sess-ds1h", FromTurn: 1, ToTurn: 60, ChapterTitle: "Gate", SummaryText: "Summary.", OpenLoopsJSON: `["loop"]`, RelationshipChangesJSON: `["rel"]`, WorldChangesJSON: `["world"]`, CallbackCandidatesJSON: `["cb"]`},
		},
	}
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/chapters/search", strings.NewReader(`{"chat_session_id":"sess-ds1h","query":"gate"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items := resp["chapters"].([]any)
	first := items[0].(map[string]any)
	if first["dense_role_split_policy_version"] != "ds1h.v1" {
		t.Fatalf("dense_role_split_policy_version = %v", first["dense_role_split_policy_version"])
	}
	if first["dense_narrative_usage"] != "read_only" {
		t.Fatalf("dense_narrative_usage = %v", first["dense_narrative_usage"])
	}
	if first["dense_structured_usage"] != "adjudication_retrieval" {
		t.Fatalf("dense_structured_usage = %v", first["dense_structured_usage"])
	}
	payload, ok := first["dense_structured_payload"].(map[string]any)
	if !ok {
		t.Fatalf("dense_structured_payload missing or wrong type")
	}
	wc, ok := payload["world_changes"].([]any)
	if !ok || len(wc) == 0 {
		t.Fatalf("dense_structured_payload world_changes empty or wrong type: %v", payload["world_changes"])
	}
	if wc[0] != "world" {
		t.Fatalf("dense_structured_payload world_changes[0] = %v, want world", wc[0])
	}
}

func TestEpisodeSearchIncludesDS1iDirectEvidencePromotionFields(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = &narrativeFakeStore{
		episodeSummaries: []store.EpisodeSummary{
			{ID: 1, ChatSessionID: "sess-ds1i", FromTurn: 1, ToTurn: 10, SummaryText: "Gate opened.", KeyEvents: `["world gate opened"]`, OpenLoopsJSON: `["loop1"]`, RelationshipChangesJSON: `["Alice trusts Bob"]`},
		},
	}
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/episodes/search", strings.NewReader(`{"chat_session_id":"sess-ds1i","query":"gate"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items := resp["episodes"].([]any)
	first := items[0].(map[string]any)
	if first["dense_direct_evidence_promotion_policy_version"] != "ds1i.v1" {
		t.Fatalf("dense_direct_evidence_promotion_policy_version = %v", first["dense_direct_evidence_promotion_policy_version"])
	}
	if first["dense_structured_precedence_applied"] != true {
		t.Fatalf("dense_structured_precedence_applied = %v, want true", first["dense_structured_precedence_applied"])
	}
	if first["dense_direct_evidence_promoted_relationship_count"] != float64(1) {
		t.Fatalf("dense_direct_evidence_promoted_relationship_count = %v", first["dense_direct_evidence_promoted_relationship_count"])
	}
	if first["dense_direct_evidence_promoted_world_count"] != float64(1) {
		t.Fatalf("dense_direct_evidence_promoted_world_count = %v", first["dense_direct_evidence_promoted_world_count"])
	}
	if first["dense_direct_evidence_promoted_promise_count"] != float64(0) {
		t.Fatalf("dense_direct_evidence_promoted_promise_count = %v, want 0", first["dense_direct_evidence_promoted_promise_count"])
	}
	score, ok := first["dense_direct_evidence_promotion_score"].(float64)
	if !ok || score != 2 {
		t.Fatalf("dense_direct_evidence_promotion_score = %v, want 2", first["dense_direct_evidence_promotion_score"])
	}
}

func TestChapterSearchResumePackIncludesDS1fThroughDS1iFields(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = &narrativeFakeStore{
		resumePack: &store.ResumePack{
			Chapter: &store.ChapterSummary{
				ID: 99, ChatSessionID: "sess-resume", FromTurn: 1, ToTurn: 60, ChapterIndex: 1, ChapterTitle: "Gate",
				SummaryText: "Summary.", ResumeText: "Resume.",
				OpenLoopsJSON: `["loop"]`, RelationshipChangesJSON: `["rel"]`, WorldChangesJSON: `["world"]`, CallbackCandidatesJSON: `["cb"]`},
		},
	}
	srv.RegisterRoutes(mux)
	req := httptest.NewRequest(http.MethodPost, "/chapters/search", strings.NewReader(`{"chat_session_id":"sess-resume","query":"gate"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items := resp["chapters"].([]any)
	first := items[0].(map[string]any)
	if first["source_record_type"] != "chapter" {
		t.Fatalf("source_record_type = %v, want chapter", first["source_record_type"])
	}
	if first["dense_source_anchor_policy_version"] != "ds1f.v1" {
		t.Fatalf("dense_source_anchor_policy_version = %v", first["dense_source_anchor_policy_version"])
	}
	if first["dense_retention_applied"] != true {
		t.Fatalf("dense_retention_applied = %v", first["dense_retention_applied"])
	}
	if first["dense_role_split_policy_version"] != "ds1h.v1" {
		t.Fatalf("dense_role_split_policy_version = %v", first["dense_role_split_policy_version"])
	}
	if first["dense_direct_evidence_promotion_policy_version"] != "ds1i.v1" {
		t.Fatalf("dense_direct_evidence_promotion_policy_version = %v", first["dense_direct_evidence_promotion_policy_version"])
	}
}

func TestArcGenerateWritesArcSummaryFromChapters(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Cfg.StoreMode = config.StoreModeMariaDBAuthority
	fake := &narrativeFakeStore{
		chapterSummaries: []store.ChapterSummary{
			{ChatSessionID: "sess-arc", FromTurn: 1, ToTurn: 60, ChapterIndex: 1, ChapterTitle: "Gate", SummaryText: "Alice opens the gate.", ResumeText: "Gate opened."},
			{ChatSessionID: "sess-arc", FromTurn: 61, ToTurn: 120, ChapterIndex: 2, ChapterTitle: "Ledger", SummaryText: "Bob keeps the ledger.", ResumeText: "Ledger protected."},
			{ChatSessionID: "sess-arc", FromTurn: 121, ToTurn: 180, ChapterIndex: 3, ChapterTitle: "Tower", SummaryText: "The tower rule changes.", ResumeText: "Rule changed."},
		},
	}
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/arcs/generate", strings.NewReader(`{"chat_session_id":"sess-arc","from_turn":1,"to_turn":180}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedArcSummaries) != 1 {
		t.Fatalf("saved arcs = %d, want 1", len(fake.savedArcSummaries))
	}
	saved := fake.savedArcSummaries[0]
	if saved.ChatSessionID != "sess-arc" || saved.FromTurn != 1 || saved.ToTurn != 180 || saved.ArcStatus != "active" {
		t.Fatalf("saved arc mismatch: %+v", saved)
	}
	if !strings.Contains(saved.ArcResumeText, "Gate opened") {
		t.Fatalf("saved arc resume missing chapter material: %+v", saved)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["generation_source"] != "deterministic_migration_stub" || resp["saved"] != true {
		t.Fatalf("unexpected arc generation response: %#v", resp)
	}
}

func TestChapterGeneratePrioritizesEpisodeDenseAnchorsOverSummaryText(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Cfg.StoreMode = config.StoreModeMariaDBAuthority
	fake := &narrativeFakeStore{
		episodeSummaries: []store.EpisodeSummary{
			{
				ChatSessionID:           "sess-ds1b-chapter",
				FromTurn:                1,
				ToTurn:                  20,
				SummaryText:             "Generic school day summary.",
				KeyEvents:               `["tower gate rule changes at midnight"]`,
				OpenLoopsJSON:           `["sealed ledger callback remains unresolved"]`,
				RelationshipChangesJSON: `["Alice starts trusting Bob"]`,
			},
			{
				ChatSessionID:           "sess-ds1b-chapter",
				FromTurn:                21,
				ToTurn:                  60,
				SummaryText:             "Another generic summary.",
				KeyEvents:               `["archive city pressure rises"]`,
				OpenLoopsJSON:           `["ask why the gate opened"]`,
				RelationshipChangesJSON: `["Bob promises not to hide the ledger"]`,
			},
		},
	}
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chapters/generate", strings.NewReader(`{"chat_session_id":"sess-ds1b-chapter","turn_index":60,"interval":60}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedChapterSummaries) != 1 {
		t.Fatalf("saved chapters = %d, want 1", len(fake.savedChapterSummaries))
	}
	saved := fake.savedChapterSummaries[0]
	if !strings.Contains(saved.OpenLoopsJSON, "sealed ledger callback") || !strings.Contains(saved.RelationshipChangesJSON, "Alice starts trusting Bob") {
		t.Fatalf("saved chapter anchors incomplete: %+v", saved)
	}
	if !strings.Contains(saved.WorldChangesJSON, "tower gate rule changes") || !strings.Contains(saved.CallbackCandidatesJSON, "ask why the gate opened") {
		t.Fatalf("saved chapter world/callback anchors incomplete: %+v", saved)
	}
	openIdx := strings.Index(saved.SummaryText, "open_loop:")
	summaryIdx := strings.Index(saved.SummaryText, "summary:")
	if openIdx < 0 || summaryIdx < 0 || openIdx > summaryIdx {
		t.Fatalf("summary did not prioritize anchors before summary text: %q", saved.SummaryText)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	stats, ok := resp["input_stats"].(map[string]any)
	if !ok || stats["chapter_dense_summary_injection_policy_version"] != chapterDenseSummaryPolicyVersion {
		t.Fatalf("chapter dense stats missing: %#v", resp)
	}
}

func TestArcGeneratePrioritizesChapterDenseAnchorsAndPersistsDS1cFields(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Cfg.StoreMode = config.StoreModeMariaDBAuthority
	fake := &narrativeFakeStore{
		chapterSummaries: []store.ChapterSummary{
			{
				ChatSessionID:           "sess-ds1c-arc",
				FromTurn:                1,
				ToTurn:                  60,
				ChapterIndex:            1,
				ChapterTitle:            "Gate",
				SummaryText:             "Generic gate summary.",
				OpenLoopsJSON:           `["sealed gate callback debt"]`,
				RelationshipChangesJSON: `["Alice pivots from suspicion to trust"]`,
				WorldChangesJSON:        `["tower gate law changes permanently"]`,
				CallbackCandidatesJSON:  `["return to the sealed gate"]`,
				ResumeText:              "Resume should be below dense anchors.",
			},
			{
				ChatSessionID:           "sess-ds1c-arc",
				FromTurn:                61,
				ToTurn:                  120,
				ChapterIndex:            2,
				ChapterTitle:            "Ledger",
				SummaryText:             "Generic ledger summary.",
				OpenLoopsJSON:           `["ledger promise remains unpaid"]`,
				RelationshipChangesJSON: `["Bob becomes a guarded ally"]`,
				WorldChangesJSON:        `["archive faction pressure rises"]`,
				CallbackCandidatesJSON:  `["ledger oath callback"]`,
				ResumeText:              "Ledger resume should be below dense anchors.",
			},
		},
	}
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/arcs/generate", strings.NewReader(`{"chat_session_id":"sess-ds1c-arc","from_turn":1,"to_turn":120}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedArcSummaries) != 1 {
		t.Fatalf("saved arcs = %d, want 1", len(fake.savedArcSummaries))
	}
	saved := fake.savedArcSummaries[0]
	if !strings.Contains(saved.IrreversibleTurnsJSON, "tower gate law") || !strings.Contains(saved.CallbackDebtsJSON, "sealed gate callback") || !strings.Contains(saved.RelationshipPivotsJSON, "suspicion to trust") {
		t.Fatalf("saved arc DS-1c fields incomplete: %+v", saved)
	}
	openIdx := strings.Index(saved.CoreConflict, "open_loop:")
	summaryIdx := strings.Index(saved.CoreConflict, "summary:")
	if openIdx < 0 || summaryIdx < 0 || openIdx > summaryIdx {
		t.Fatalf("arc core did not prioritize chapter anchors before summary text: %q", saved.CoreConflict)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	stats, ok := resp["input_stats"].(map[string]any)
	if !ok || stats["chapter_dense_summary_injection_policy_version"] != chapterDenseSummaryPolicyVersion || stats["arc_dense_summary_policy_version"] != arcDenseSummaryPolicyVersion {
		t.Fatalf("arc dense stats missing: %#v", resp)
	}
}

func TestSagaGenerateConsumesArcDS1cAnchors(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Cfg.StoreMode = config.StoreModeMariaDBAuthority
	fake := &narrativeFakeStore{
		arcSummaries: []store.ArcSummary{
			{
				ChatSessionID:          "sess-ds1c-saga",
				FromTurn:               1,
				ToTurn:                 180,
				ArcIndex:               1,
				ArcName:                "Gate Arc",
				ArcStatus:              "active",
				CoreConflict:           "Generic core conflict.",
				IrreversibleTurnsJSON:  `["tower gate law cannot be reversed"]`,
				CallbackDebtsJSON:      `["repay the sealed gate callback"]`,
				RelationshipPivotsJSON: `["Alice and Bob become allies"]`,
				ArcResumeText:          "Resume should be lower priority.",
			},
		},
	}
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/sagas/generate", strings.NewReader(`{"chat_session_id":"sess-ds1c-saga","from_turn":1,"to_turn":180}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedSagaDigests) != 1 {
		t.Fatalf("saved sagas = %d, want 1", len(fake.savedSagaDigests))
	}
	saved := fake.savedSagaDigests[0]
	if !strings.Contains(saved.SagaSummary, "irreversible: tower gate law") || !strings.Contains(saved.SagaSummary, "callback_debt: repay") || !strings.Contains(saved.SagaSummary, "relationship_pivot: Alice") {
		t.Fatalf("saved saga did not consume DS-1c anchors first: %+v", saved)
	}
	if !strings.Contains(saved.NeverDropCandidatesJSON, "sealed gate callback") || !strings.Contains(saved.NeverDropCandidatesJSON, "become allies") {
		t.Fatalf("never drop candidates missing DS-1c anchors: %+v", saved)
	}
}

func TestChapterSearchDensePriorityPromotesAnchorsOverRecency(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = &narrativeFakeStore{
		chapterSummaries: []store.ChapterSummary{
			{
				ID:            2,
				ChatSessionID: "sess-ds1d-search",
				FromTurn:      61,
				ToTurn:        120,
				ChapterIndex:  2,
				ChapterTitle:  "Recent Gate Mention",
				SummaryText:   "gate appears in a plain recent recap",
				ResumeText:    "recent but anchor thin",
			},
			{
				ID:                      1,
				ChatSessionID:           "sess-ds1d-search",
				FromTurn:                1,
				ToTurn:                  60,
				ChapterIndex:            1,
				ChapterTitle:            "Older Dense Gate",
				SummaryText:             "brief recap",
				ResumeText:              "gate anchor still matters",
				OpenLoopsJSON:           `["gate promise remains unpaid"]`,
				RelationshipChangesJSON: `["Alice trusts Bob because of the gate"]`,
				WorldChangesJSON:        `["gate law changes the archive"]`,
				CallbackCandidatesJSON:  `["return to the gate promise"]`,
			},
		},
	}
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chapters/search", strings.NewReader(`{"chat_session_id":"sess-ds1d-search","query":"gate","limit":1}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := resp["chapters"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("chapters = %#v", resp["chapters"])
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("first chapter shape = %#v", items[0])
	}
	if first["chapter_title"] != "Older Dense Gate" {
		t.Fatalf("dense chapter was not promoted: %#v", first)
	}
	if first["dense_summary_policy_version"] != denseSummaryPriorityPolicyVersion {
		t.Fatalf("dense policy version missing: %#v", first)
	}
	if score, ok := first["dense_priority_score"].(float64); !ok || score <= 0 {
		t.Fatalf("dense priority score missing: %#v", first)
	}
}

func TestDenseSummarySearchResultsExposeSourceRoleRetentionAndEvidencePromotion(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = &narrativeFakeStore{
		evidence: []store.DirectEvidence{
			{
				ID:              31,
				ChatSessionID:   "sess-ds1f-search",
				EvidenceKind:    "relationship_world_promise",
				EvidenceText:    "Alice and Bob promise to obey the gate law together.",
				SourceTurnStart: 40,
				SourceTurnEnd:   44,
				TurnAnchor:      42,
			},
		},
		chapterSummaries: []store.ChapterSummary{
			{
				ID:                      7,
				ChatSessionID:           "sess-ds1f-search",
				FromTurn:                40,
				ToTurn:                  60,
				ChapterIndex:            2,
				ChapterTitle:            "Gate Promise",
				SummaryText:             "A plain summary of the gate promise.",
				ResumeText:              "Gate promise remains important.",
				OpenLoopsJSON:           `["gate promise remains unpaid"]`,
				RelationshipChangesJSON: `["Alice and Bob form a durable alliance"]`,
				WorldChangesJSON:        `["gate law changes the archive city"]`,
				CallbackCandidatesJSON:  `["repay the gate promise later"]`,
			},
		},
	}
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chapters/search", strings.NewReader(`{"chat_session_id":"sess-ds1f-search","query":"gate","limit":1}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := resp["chapters"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("chapters = %#v", resp["chapters"])
	}
	item := items[0].(map[string]any)
	if item["dense_source_anchor_policy_version"] != denseSourceAnchorPolicyVersion {
		t.Fatalf("DS-1f source anchor policy missing: %#v", item)
	}
	if item["source_record_id"] != float64(7) || item["source_record_type"] != "chapter" {
		t.Fatalf("DS-1f source record mismatch: %#v", item)
	}
	if item["dense_role_split_policy_version"] != denseRoleSplitPolicyVersion || item["dense_narrative_usage"] != "read_only" || item["dense_structured_usage"] != "adjudication_retrieval" {
		t.Fatalf("DS-1h role split missing: %#v", item)
	}
	payload, ok := item["dense_structured_payload"].(map[string]any)
	if !ok || payload["relationship_changes"] == nil || payload["world_changes"] == nil || payload["callback_candidates"] == nil {
		t.Fatalf("DS-1h structured payload missing: %#v", item["dense_structured_payload"])
	}
	if item["dense_retention_policy_version"] != denseRetentionPolicyVersion || item["dense_retention_applied"] != true {
		t.Fatalf("DS-1g retention fields missing: %#v", item)
	}
	if item["dense_direct_evidence_promotion_policy_version"] != denseEvidencePromotionPolicy || item["dense_structured_precedence_applied"] != true {
		t.Fatalf("DS-1i evidence promotion missing: %#v", item)
	}
	if item["dense_direct_evidence_promoted_relationship_count"].(float64) < 1 || item["dense_direct_evidence_promoted_world_count"].(float64) < 1 || item["dense_direct_evidence_promoted_promise_count"].(float64) < 1 {
		t.Fatalf("DS-1i evidence promotion counts missing: %#v", item)
	}
}

func TestSagaGenerateWritesSagaDigestFromArcs(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Cfg.StoreMode = config.StoreModeMariaDBAuthority
	fake := &narrativeFakeStore{
		arcSummaries: []store.ArcSummary{
			{ChatSessionID: "sess-saga", FromTurn: 1, ToTurn: 180, ArcIndex: 1, ArcName: "Gate Arc", ArcStatus: "active", CoreConflict: "Gate opens.", ArcResumeText: "The gate arc is active."},
			{ChatSessionID: "sess-saga", FromTurn: 181, ToTurn: 360, ArcIndex: 2, ArcName: "Ledger Arc", ArcStatus: "active", CoreConflict: "Ledger returns.", ArcResumeText: "The ledger arc returns."},
		},
	}
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/sagas/generate", strings.NewReader(`{"chat_session_id":"sess-saga","from_turn":1,"to_turn":360}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if len(fake.savedSagaDigests) != 1 {
		t.Fatalf("saved sagas = %d, want 1", len(fake.savedSagaDigests))
	}
	saved := fake.savedSagaDigests[0]
	if saved.ChatSessionID != "sess-saga" || saved.FromTurn != 1 || saved.ToTurn != 360 {
		t.Fatalf("saved saga mismatch: %+v", saved)
	}
	if !strings.Contains(saved.ResumePackText, "gate arc") && !strings.Contains(saved.ResumePackText, "Gate") {
		t.Fatalf("saved saga resume missing arc material: %+v", saved)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["generation_source"] != "deterministic_migration_stub" || resp["saved"] != true {
		t.Fatalf("unexpected saga generation response: %#v", resp)
	}
}

// ---------------------------------------------------------------------------
// Session delete tests
// ---------------------------------------------------------------------------

type narrativeFakeVectorStore struct {
	deleteCalled    bool
	deleteSessionID string
	deleteErr       error
}

func (f *narrativeFakeVectorStore) Search(ctx context.Context, sessionID string, vec []float32, limit int, filter string) ([]vector.VectorDocument, error) {
	return nil, vector.ErrNotEnabled
}
func (f *narrativeFakeVectorStore) Upsert(ctx context.Context, sessionID string, docs []vector.VectorDocument) error {
	return vector.ErrNotEnabled
}
func (f *narrativeFakeVectorStore) DeleteSession(ctx context.Context, sessionID string) error {
	f.deleteCalled = true
	f.deleteSessionID = sessionID
	return f.deleteErr
}
func (f *narrativeFakeVectorStore) Rebuild(ctx context.Context, sessionID string) error {
	return vector.ErrNotEnabled
}
func (f *narrativeFakeVectorStore) Health(ctx context.Context) (vector.HealthSnapshot, error) {
	return vector.HealthSnapshot{Status: "ok"}, nil
}
func (f *narrativeFakeVectorStore) Count(ctx context.Context, sessionID string) (int, error) {
	return 0, vector.ErrNotEnabled
}
func (f *narrativeFakeVectorStore) Close(ctx context.Context) error { return nil }

func TestSessionDeleteShadowNoMutation(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = &narrativeFakeStore{}
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/sessions/sess-shadow", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["deleted"] != false {
		t.Errorf("deleted = %v, want false", resp["deleted"])
	}
	if resp["mutation_enabled"] != false {
		t.Errorf("mutation_enabled = %v, want false", resp["mutation_enabled"])
	}
}

func TestSessionDeleteLiveExecutes(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority

	fake := &narrativeFakeStore{}
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil
	vs := &narrativeFakeVectorStore{}
	srv.Vector = vs

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/sessions/sess-live", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if !fake.deleteSessionCalled {
		t.Error("DeleteSession was not called")
	}
	if !vs.deleteCalled {
		t.Error("Vector DeleteSession was not called")
	}
	if vs.deleteSessionID != "sess-live" {
		t.Errorf("vector deleteSessionID = %s, want sess-live", vs.deleteSessionID)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["deleted"] != true {
		t.Errorf("deleted = %v, want true", resp["deleted"])
	}
	if resp["mutation_enabled"] != true {
		t.Errorf("mutation_enabled = %v, want true", resp["mutation_enabled"])
	}
	vc, ok := resp["vector_cleanup"].(map[string]any)
	if !ok {
		t.Fatalf("vector_cleanup is not an object, got %T", resp["vector_cleanup"])
	}
	if vc["attempted"] != true {
		t.Errorf("vector_cleanup.attempted = %v, want true", vc["attempted"])
	}
	if vc["ok"] != true {
		t.Errorf("vector_cleanup.ok = %v, want true", vc["ok"])
	}
	if len(fake.auditLogs) == 0 {
		t.Fatal("expected session_delete audit log")
	}
	audit := fake.auditLogs[0]
	if audit.EventType != "session_delete" {
		t.Fatalf("event_type = %q, want session_delete", audit.EventType)
	}
	if audit.ChatSessionID != "sess-live" {
		t.Fatalf("chat_session_id = %q, want sess-live", audit.ChatSessionID)
	}
	if audit.TargetType != "session" {
		t.Fatalf("target_type = %q, want session", audit.TargetType)
	}
}

func TestSessionDeleteLiveVectorWarning(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority

	fake := &narrativeFakeStore{}
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil
	vs := &narrativeFakeVectorStore{deleteErr: vector.ErrNotEnabled}
	srv.Vector = vs

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/sessions/sess-vec-warn", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("status = %v, want ok", resp["status"])
	}
	vc, ok := resp["vector_cleanup"].(map[string]any)
	if !ok {
		t.Fatalf("vector_cleanup is not an object, got %T", resp["vector_cleanup"])
	}
	if vc["ok"] != true {
		t.Errorf("vector_cleanup.ok = %v, want true", vc["ok"])
	}
	if vc["warning"] != "vector store is not enabled" {
		t.Errorf("vector_cleanup.warning = %v, want 'vector store is not enabled'", vc["warning"])
	}
}

func TestSessionDeleteLiveStoreError(t *testing.T) {
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority

	fake := &narrativeFakeStore{deleteSessionErr: errors.New("db failure")}
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/sessions/sess-err", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestNarrativeControlGetCachedFresh(t *testing.T) {
	cachedPlan := map[string]any{
		"current_arc":        "Arc A",
		"narrative_goal":     "goal A",
		"active_tensions":    []any{"t1"},
		"next_beats":         []any{"b1"},
		"continuity_anchors": []any{"a1"},
		"guardrails":         []any{},
		"persona_priorities": []any{},
		"execution_notes":    []any{},
		"focus_characters":   []any{},
		"last_plan_turn":     float64(10),
		"state_status":       "ready",
	}
	cachedDir := map[string]any{
		"scene_mandate":       "Continue arc: Arc A",
		"required_outcomes":   []any{},
		"forbidden_moves":     []any{},
		"pressure_level":      "steady",
		"execution_checklist": []any{},
		"persona_guardrails":  []any{},
		"world_guardrails":    []any{},
		"focus_characters":    []any{},
		"last_turn":           float64(10),
		"state_status":        "ready",
		"resolved_outcomes":   []any{},
		"expired_forbidden":   []any{},
	}
	spJSON, _ := json.Marshal(cachedPlan)
	dirJSON, _ := json.Marshal(cachedDir)
	warnJSON, _ := json.Marshal([]any{})

	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-fresh",
			StoryPlanJSON: string(spJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "ready",
			LastTurn:      10,
			WarningsJSON:  string(warnJSON),
		},
		storylines: []store.Storyline{
			{ChatSessionID: "sess-fresh", Name: "Arc A", LastTurn: 9, FirstTurn: 1},
		},
	}

	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/narrative-control/sess-fresh", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["state_status"] != "ready" {
		t.Fatalf("state_status = %v, want ready", resp["state_status"])
	}
	if fake.savedGuidancePlanState != nil {
		t.Fatal("expected no upsert when cache is fresh")
	}
	plan, ok := resp["story_plan"].(map[string]any)
	if !ok {
		t.Fatal("story_plan is not an object")
	}
	if plan["current_arc"] != "Arc A" {
		t.Fatalf("current_arc = %v, want Arc A", plan["current_arc"])
	}
}

func TestNarrativeControlGetStaleRebuildsAndUpserts(t *testing.T) {
	cachedPlan := map[string]any{
		"current_arc":        "Old Arc",
		"narrative_goal":     "old goal",
		"active_tensions":    []any{},
		"next_beats":         []any{"old-beat"},
		"continuity_anchors": []any{"old-anchor"},
		"guardrails":         []any{},
		"persona_priorities": []any{},
		"execution_notes":    []any{},
		"focus_characters":   []any{},
		"last_plan_turn":     float64(5),
		"state_status":       "ready",
	}
	cachedDir := map[string]any{
		"scene_mandate":       "Continue arc: Old Arc",
		"required_outcomes":   []any{},
		"forbidden_moves":     []any{},
		"pressure_level":      "light",
		"execution_checklist": []any{},
		"persona_guardrails":  []any{},
		"world_guardrails":    []any{},
		"focus_characters":    []any{},
		"last_turn":           float64(5),
		"state_status":        "ready",
		"resolved_outcomes":   []any{},
		"expired_forbidden":   []any{},
	}
	spJSON, _ := json.Marshal(cachedPlan)
	dirJSON, _ := json.Marshal(cachedDir)
	warnJSON, _ := json.Marshal([]any{})

	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-stale",
			StoryPlanJSON: string(spJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "ready",
			LastTurn:      5,
			WarningsJSON:  string(warnJSON),
		},
		storylines: []store.Storyline{
			{ChatSessionID: "sess-stale", Name: "New Arc", LastTurn: 12, FirstTurn: 6, CurrentContext: "new ctx"},
		},
	}

	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/narrative-control/sess-stale", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if fake.savedGuidancePlanState == nil {
		t.Fatal("expected upsert after stale rebuild")
	}
	if fake.savedGuidancePlanState.LastTurn != 12 {
		t.Fatalf("upsert last_turn = %d, want 12", fake.savedGuidancePlanState.LastTurn)
	}
	plan, _ := resp["story_plan"].(map[string]any)
	if plan["current_arc"] != "New Arc" {
		t.Fatalf("current_arc = %v, want New Arc", plan["current_arc"])
	}
}

func TestNarrativeControlGetSameArcConservativeMerge(t *testing.T) {
	cachedPlan := map[string]any{
		"current_arc":        "Arc A",
		"narrative_goal":     "old goal",
		"active_tensions":    []any{},
		"next_beats":         []any{"cached-beat-1", "cached-beat-2"},
		"continuity_anchors": []any{"cached-anchor"},
		"guardrails":         []any{},
		"persona_priorities": []any{},
		"execution_notes":    []any{},
		"focus_characters":   []any{},
		"last_plan_turn":     float64(8),
		"state_status":       "ready",
	}
	cachedDir := map[string]any{
		"scene_mandate":       "Continue arc: Arc A",
		"required_outcomes":   []any{},
		"forbidden_moves":     []any{},
		"pressure_level":      "light",
		"execution_checklist": []any{},
		"persona_guardrails":  []any{},
		"world_guardrails":    []any{},
		"focus_characters":    []any{},
		"last_turn":           float64(8),
		"state_status":        "ready",
		"resolved_outcomes":   []any{},
		"expired_forbidden":   []any{},
	}
	spJSON, _ := json.Marshal(cachedPlan)
	dirJSON, _ := json.Marshal(cachedDir)
	warnJSON, _ := json.Marshal([]any{})

	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-merge",
			StoryPlanJSON: string(spJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "ready",
			LastTurn:      6,
			WarningsJSON:  string(warnJSON),
		},
		storylines: []store.Storyline{
			{ChatSessionID: "sess-merge", Name: "Arc A", LastTurn: 10, FirstTurn: 1, CurrentContext: "new goal"},
		},
	}

	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/narrative-control/sess-merge", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	plan, _ := resp["story_plan"].(map[string]any)
	if plan["current_arc"] != "Arc A" {
		t.Fatalf("current_arc = %v, want Arc A", plan["current_arc"])
	}
	if plan["narrative_goal"] != "new goal" {
		t.Fatalf("narrative_goal = %v, want new goal", plan["narrative_goal"])
	}
	beats := plan["next_beats"].([]any)
	foundCached := false
	for _, b := range beats {
		if b == "cached-beat-1" || b == "cached-beat-2" {
			foundCached = true
		}
	}
	if !foundCached {
		t.Fatalf("expected merged next_beats to include cached beats")
	}
	anchors := plan["continuity_anchors"].([]any)
	foundAnchor := false
	for _, a := range anchors {
		if a == "cached-anchor" {
			foundAnchor = true
		}
	}
	if !foundAnchor {
		t.Fatalf("expected merged continuity_anchors to include cached anchor")
	}
}

func TestNarrativeControlGetNoStoreSupportNonFatal(t *testing.T) {
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = store.NewNoopStore()
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/narrative-control/sess-nogps", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("status = %v, want ok", resp["status"])
	}
	if resp["state_status"] != "skeleton" {
		t.Fatalf("state_status = %v, want skeleton", resp["state_status"])
	}
}

func TestNarrativeControlGetUpsertFailureNonFatal(t *testing.T) {
	fake := &narrativeFakeStore{
		storylines: []store.Storyline{
			{ChatSessionID: "sess-upsert-err", Name: "Arc C", LastTurn: 4, FirstTurn: 1},
		},
		guidancePlanUpsertErr: errors.New("upsert failure"),
	}

	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/narrative-control/sess-upsert-err", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("status = %v, want ok", resp["status"])
	}
}

func TestNarrativeControlGetInvalidCacheRebuildsNonFatal(t *testing.T) {
	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-invalid-cache",
			StoryPlanJSON: "{not-json",
			DirectorJSON:  `{"scene_mandate":"old"}`,
			StateStatus:   "ready",
			LastTurn:      7,
			WarningsJSON:  "[]",
		},
		storylines: []store.Storyline{
			{ChatSessionID: "sess-invalid-cache", Name: "Arc D", LastTurn: 7, FirstTurn: 1, CurrentContext: "rebuilt"},
		},
	}

	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/narrative-control/sess-invalid-cache", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if fake.savedGuidancePlanState == nil {
		t.Fatal("expected invalid cache to rebuild and upsert")
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	plan, _ := resp["story_plan"].(map[string]any)
	if plan["current_arc"] != "Arc D" {
		t.Fatalf("current_arc = %v, want Arc D", plan["current_arc"])
	}
}

func TestNarrativeControlGetBackwardFreshnessRebuildsAfterRollback(t *testing.T) {
	cachedPlan := map[string]any{
		"current_arc":        "Future Arc",
		"narrative_goal":     "future goal",
		"active_tensions":    []any{},
		"next_beats":         []any{"future-beat"},
		"continuity_anchors": []any{"future-anchor"},
		"guardrails":         []any{},
		"persona_priorities": []any{},
		"execution_notes":    []any{},
		"focus_characters":   []any{},
		"last_plan_turn":     float64(10),
		"state_status":       "ready",
	}
	cachedDir := map[string]any{
		"scene_mandate":       "Continue arc: Future Arc",
		"required_outcomes":   []any{},
		"forbidden_moves":     []any{},
		"pressure_level":      "steady",
		"execution_checklist": []any{},
		"persona_guardrails":  []any{},
		"world_guardrails":    []any{},
		"focus_characters":    []any{},
		"last_turn":           float64(10),
		"state_status":        "ready",
		"resolved_outcomes":   []any{},
		"expired_forbidden":   []any{},
	}
	spJSON, _ := json.Marshal(cachedPlan)
	dirJSON, _ := json.Marshal(cachedDir)
	warnJSON, _ := json.Marshal([]any{})
	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-backward-freshness",
			StoryPlanJSON: string(spJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "ready",
			LastTurn:      10,
			WarningsJSON:  string(warnJSON),
		},
		storylines: []store.Storyline{
			{ChatSessionID: "sess-backward-freshness", Name: "Rolled Back Arc", LastTurn: 5, FirstTurn: 1, CurrentContext: "after rollback"},
		},
	}

	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/narrative-control/sess-backward-freshness", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if fake.savedGuidancePlanState == nil {
		t.Fatal("expected future cache to be rejected and rebuilt")
	}
	if fake.savedGuidancePlanState.LastTurn != 5 {
		t.Fatalf("upsert last_turn = %d, want 5", fake.savedGuidancePlanState.LastTurn)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	plan, _ := resp["story_plan"].(map[string]any)
	if plan["current_arc"] != "Rolled Back Arc" {
		t.Fatalf("current_arc = %v, want Rolled Back Arc", plan["current_arc"])
	}
}

func TestNarrativeControlGetStoryGuidanceYieldsToExplicitUserInput(t *testing.T) {
	cachedPlan := map[string]any{
		"current_arc":        "Old Arc",
		"narrative_goal":     "force the confession scene",
		"active_tensions":    []any{"confession pressure"},
		"next_beats":         []any{"force confession now"},
		"continuity_anchors": []any{"old promise"},
		"guardrails":         []any{},
		"persona_priorities": []any{},
		"execution_notes":    []any{},
		"focus_characters":   []any{"Chloe"},
		"last_plan_turn":     float64(8),
		"state_status":       "ready",
	}
	cachedDir := map[string]any{
		"scene_mandate":       "Continue arc: Old Arc",
		"required_outcomes":   []any{"force confession now"},
		"forbidden_moves":     []any{"ignore the old arc"},
		"pressure_level":      "strong",
		"execution_checklist": []any{"advance the confession"},
		"persona_guardrails":  []any{},
		"world_guardrails":    []any{},
		"focus_characters":    []any{"Chloe"},
		"last_turn":           float64(8),
		"state_status":        "ready",
		"resolved_outcomes":   []any{},
		"expired_forbidden":   []any{},
	}
	spJSON, _ := json.Marshal(cachedPlan)
	dirJSON, _ := json.Marshal(cachedDir)
	warnJSON, _ := json.Marshal([]any{})
	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-user-conflict",
			StoryPlanJSON: string(spJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "ready",
			LastTurn:      8,
			WarningsJSON:  string(warnJSON),
		},
		storylines: []store.Storyline{
			{ChatSessionID: "sess-user-conflict", Name: "Old Arc", LastTurn: 8, FirstTurn: 1, CurrentContext: "old arc pressure"},
		},
	}

	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/narrative-control/sess-user-conflict?current_user_input=leave+the+scene", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	guidance, ok := resp["story_guidance"].(map[string]any)
	if !ok {
		t.Fatalf("story_guidance missing: %+v", resp)
	}
	conflictPolicy, ok := guidance["conflict_policy"].(map[string]any)
	if !ok {
		t.Fatalf("conflict_policy missing: %+v", guidance)
	}
	if conflictPolicy["current_user_input_wins"] != true || conflictPolicy["guidance_may_override_user_input"] != false {
		t.Fatalf("conflict_policy does not yield to user input: %+v", conflictPolicy)
	}
	if conflictPolicy["on_conflict"] != "yield_to_current_user_input" {
		t.Fatalf("on_conflict = %v, want yield_to_current_user_input", conflictPolicy["on_conflict"])
	}
	precedence, ok := guidance["precedence"].(map[string]any)
	if !ok {
		t.Fatalf("precedence missing: %+v", guidance)
	}
	if precedence["guidance_authority"] != "subordinate" {
		t.Fatalf("guidance_authority = %v, want subordinate", precedence["guidance_authority"])
	}
	higherPriority, _ := precedence["higher_priority_sources"].([]any)
	if len(higherPriority) == 0 || higherPriority[0] != "current_user_input" {
		t.Fatalf("higher_priority_sources = %+v, want current_user_input first", higherPriority)
	}
	disallowed, _ := precedence["disallowed_usage"].([]any)
	if !containsAnyStringValue(disallowed, "current_user_input_override") {
		t.Fatalf("disallowed_usage = %+v, want current_user_input_override", disallowed)
	}
	turnDirectives, ok := guidance["turn_directives"].(map[string]any)
	if !ok {
		t.Fatalf("turn_directives missing: %+v", guidance)
	}
	failMode, ok := turnDirectives["fail_mode"].(map[string]any)
	if !ok {
		t.Fatalf("fail_mode missing: %+v", turnDirectives)
	}
	if failMode["respect_explicit_user_correction"] != true {
		t.Fatalf("fail_mode does not respect explicit user correction: %+v", failMode)
	}
	if fake.savedGuidancePlanState != nil {
		t.Fatal("fresh conflicting cache should not be rewritten by a read-only precedence proof")
	}
}

func containsAnyStringValue(items []any, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}

func TestNarrativeControlGetResolvedOutcomesAccumulate(t *testing.T) {
	prevDirector := map[string]any{
		"scene_mandate":       "Continue arc: Arc A",
		"required_outcomes":   []string{"Carry forward: Hook A"},
		"forbidden_moves":     []string{},
		"pressure_level":      "light",
		"execution_checklist": []any{},
		"persona_guardrails":  []any{},
		"world_guardrails":    []any{},
		"focus_characters":    []any{},
		"last_turn":           float64(5),
		"state_status":        "ready",
		"resolved_outcomes":   []string{},
		"expired_forbidden":   []string{},
	}
	prevPlan := map[string]any{
		"current_arc":        "Arc A",
		"narrative_goal":     "goal",
		"active_tensions":    []any{},
		"next_beats":         []any{},
		"continuity_anchors": []any{},
		"guardrails":         []any{},
		"persona_priorities": []any{},
		"execution_notes":    []any{},
		"focus_characters":   []any{},
		"last_plan_turn":     float64(5),
		"state_status":       "heuristic",
	}
	dirJSON, _ := json.Marshal(prevDirector)
	planJSON, _ := json.Marshal(prevPlan)
	warnJSON, _ := json.Marshal([]any{})

	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-resolved",
			StoryPlanJSON: string(planJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "ready",
			LastTurn:      5,
			WarningsJSON:  string(warnJSON),
		},
		storylines: []store.Storyline{
			{ChatSessionID: "sess-resolved", Name: "Arc A", LastTurn: 10, FirstTurn: 1},
		},
		pendingThreads: []store.PendingThread{},
	}

	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/narrative-control/sess-resolved", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	director, _ := resp["director"].(map[string]any)
	resolved, _ := director["resolved_outcomes"].([]any)
	found := false
	for _, r := range resolved {
		if r == "Carry forward: Hook A" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected resolved_outcomes to include previously required hook, got %v", resolved)
	}
	required, _ := director["required_outcomes"].([]any)
	for _, r := range required {
		if r == "Carry forward: Hook A" {
			t.Fatal("expected required_outcomes to NOT include resolved hook")
		}
	}
}

func TestNarrativeControlGetExpiredForbiddenAccumulate(t *testing.T) {
	prevDirector := map[string]any{
		"scene_mandate":       "Continue arc: Arc A",
		"required_outcomes":   []string{},
		"forbidden_moves":     []string{"Do not abruptly resolve: Risk A"},
		"pressure_level":      "light",
		"execution_checklist": []any{},
		"persona_guardrails":  []any{},
		"world_guardrails":    []any{},
		"focus_characters":    []any{},
		"last_turn":           float64(5),
		"state_status":        "ready",
		"resolved_outcomes":   []string{},
		"expired_forbidden":   []string{},
	}
	prevPlan := map[string]any{
		"current_arc":        "Arc A",
		"narrative_goal":     "goal",
		"active_tensions":    []any{},
		"next_beats":         []any{},
		"continuity_anchors": []any{},
		"guardrails":         []any{},
		"persona_priorities": []any{},
		"execution_notes":    []any{},
		"focus_characters":   []any{},
		"last_plan_turn":     float64(5),
		"state_status":       "heuristic",
	}
	dirJSON, _ := json.Marshal(prevDirector)
	planJSON, _ := json.Marshal(prevPlan)
	warnJSON, _ := json.Marshal([]any{})

	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-expired",
			StoryPlanJSON: string(planJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "ready",
			LastTurn:      5,
			WarningsJSON:  string(warnJSON),
		},
		storylines: []store.Storyline{
			{ChatSessionID: "sess-expired", Name: "Arc A", LastTurn: 10, FirstTurn: 1},
		},
		pendingThreads: []store.PendingThread{},
	}

	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/narrative-control/sess-expired", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	director, _ := resp["director"].(map[string]any)
	expired, _ := director["expired_forbidden"].([]any)
	found := false
	for _, e := range expired {
		if e == "Do not abruptly resolve: Risk A" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected expired_forbidden to include previously forbidden risk, got %v", expired)
	}
	forbidden, _ := director["forbidden_moves"].([]any)
	for _, f := range forbidden {
		if f == "Do not abruptly resolve: Risk A" {
			t.Fatal("expected forbidden_moves to NOT include expired risk")
		}
	}
}

func TestNarrativeControlGetCompactHistorySummarizesResolvedWithoutActiveLeak(t *testing.T) {
	prevDirector := map[string]any{
		"scene_mandate":       "Continue arc: Arc A",
		"required_outcomes":   []string{"Carry forward: Hook A"},
		"forbidden_moves":     []string{"Do not abruptly resolve: Risk A"},
		"pressure_level":      "strong",
		"execution_checklist": []any{"keep visible beat"},
		"persona_guardrails":  []any{},
		"world_guardrails":    []any{},
		"focus_characters":    []any{},
		"last_turn":           float64(5),
		"state_status":        "ready",
		"resolved_outcomes":   []string{},
		"expired_forbidden":   []string{},
	}
	prevPlan := map[string]any{
		"current_arc":        "Arc A",
		"narrative_goal":     "goal",
		"active_tensions":    []string{"confession pressure", "promise debt"},
		"next_beats":         []any{},
		"continuity_anchors": []any{},
		"guardrails":         []any{},
		"persona_priorities": []any{},
		"execution_notes":    []any{},
		"focus_characters":   []any{},
		"last_plan_turn":     float64(5),
		"state_status":       "heuristic",
	}
	dirJSON, _ := json.Marshal(prevDirector)
	planJSON, _ := json.Marshal(prevPlan)
	warnJSON, _ := json.Marshal([]any{})

	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-compact",
			StoryPlanJSON: string(planJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "ready",
			LastTurn:      5,
			WarningsJSON:  string(warnJSON),
		},
		storylines: []store.Storyline{
			{ChatSessionID: "sess-compact", Name: "Resolved Arc", Status: "resolved", CurrentContext: "Resolved arc full context should not stay active", Confidence: 0.9, EvidenceCount: 4, LastTurn: 10, LastEvidenceTurn: 10, FirstTurn: 1},
		},
		pendingThreads: []store.PendingThread{},
	}

	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/narrative-control/sess-compact", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	director, _ := resp["director"].(map[string]any)
	for _, item := range director["required_outcomes"].([]any) {
		if item == "Carry forward: Hook A" {
			t.Fatal("resolved hook leaked back into required_outcomes")
		}
	}
	for _, item := range director["forbidden_moves"].([]any) {
		if item == "Do not abruptly resolve: Risk A" {
			t.Fatal("expired risk leaked back into forbidden_moves")
		}
	}
	history, _ := resp["compact_history"].([]any)
	if !containsAnyStringSubstring(history, "Resolved: Carry forward: Hook A") ||
		!containsAnyStringSubstring(history, "Forbidden expired: Do not abruptly resolve: Risk A") ||
		!containsAnyStringSubstring(history, "Resolved arc: Resolved Arc resolved at turn 10") {
		t.Fatalf("compact_history missing resolved/expired continuity summaries: %+v", history)
	}
	meta, _ := resp["compact_history_meta"].(map[string]any)
	if meta["total_records"] == float64(0) {
		t.Fatalf("compact_history_meta total_records = 0: %+v", meta)
	}
	if avg, _ := meta["avg_emotional_weight"].(float64); avg <= 1.0 {
		t.Fatalf("avg_emotional_weight = %v, want > 1.0: %+v", avg, meta)
	}
	if strings.Contains(strings.Join(anySliceToStringSlice(history), "\n"), "Resolved arc full context should not stay active") {
		t.Fatalf("compact_history leaked full resolved context: %+v", history)
	}
}

func TestBuildNarrativeCompactHistoryEmotionWeightPriority(t *testing.T) {
	history, meta := buildNarrativeCompactHistory(
		map[string]any{"active_tensions": []string{"fear", "promise", "public pressure"}},
		map[string]any{"pressure_level": "strong", "resolved_outcomes": []string{}, "expired_forbidden": []string{}, "last_turn": 12},
		[]store.Storyline{
			{ChatSessionID: "sess-weight", Name: "Low Weight Arc", Status: "resolved", Confidence: 0.1, EvidenceCount: 1, LastTurn: 11},
			{ChatSessionID: "sess-weight", Name: "High Weight Arc", Status: "resolved", Confidence: 0.95, EvidenceCount: 8, LastTurn: 10},
		},
		nil,
	)
	if len(history) < 2 {
		t.Fatalf("history len = %d, want >= 2: %+v", len(history), history)
	}
	if !strings.Contains(history[0], "High Weight Arc") {
		t.Fatalf("emotion/importance weighting did not prioritize high-weight arc: %+v", history)
	}
	if meta["emotion_weight_strategy"] == "" {
		t.Fatalf("compact meta missing emotion weight strategy: %+v", meta)
	}
}

func anySliceToStringSlice(items []any) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, fmt.Sprint(item))
	}
	return out
}

func TestDirectorPatchUpdatesState(t *testing.T) {
	prevDirector := map[string]any{
		"scene_mandate":       "Continue arc: Arc A",
		"required_outcomes":   []string{},
		"forbidden_moves":     []string{},
		"pressure_level":      "light",
		"execution_checklist": []any{},
		"persona_guardrails":  []any{},
		"world_guardrails":    []any{},
		"focus_characters":    []any{},
		"last_turn":           float64(5),
		"state_status":        "ready",
		"resolved_outcomes":   []string{},
		"expired_forbidden":   []string{},
	}
	prevPlan := map[string]any{
		"current_arc":        "Arc A",
		"narrative_goal":     "goal",
		"active_tensions":    []any{},
		"next_beats":         []any{},
		"continuity_anchors": []any{},
		"guardrails":         []any{},
		"persona_priorities": []any{},
		"execution_notes":    []any{},
		"focus_characters":   []any{},
		"last_plan_turn":     float64(5),
		"state_status":       "heuristic",
	}
	dirJSON, _ := json.Marshal(prevDirector)
	planJSON, _ := json.Marshal(prevPlan)
	warnJSON, _ := json.Marshal([]any{})

	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-patch",
			StoryPlanJSON: string(planJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "ready",
			LastTurn:      5,
			WarningsJSON:  string(warnJSON),
		},
	}

	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	patchBody := map[string]any{
		"scene_mandate":     "Patched mandate",
		"required_outcomes": []string{"Patched outcome"},
		"pressure_level":    "strong",
	}
	bodyJSON, _ := json.Marshal(patchBody)
	req := httptest.NewRequest(http.MethodPatch, "/narrative-control/sess-patch/director-patch", strings.NewReader(string(bodyJSON)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("status = %v, want ok", resp["status"])
	}
	if resp["patched"] != true {
		t.Fatalf("patched = %v, want true", resp["patched"])
	}
	if resp["state_status"] != "user_patched" {
		t.Fatalf("state_status = %v, want user_patched", resp["state_status"])
	}

	director, _ := resp["director"].(map[string]any)
	if director["scene_mandate"] != "Patched mandate" {
		t.Fatalf("scene_mandate = %v, want Patched mandate", director["scene_mandate"])
	}

	if fake.savedGuidancePlanState == nil {
		t.Fatal("expected saved guidance plan state after patch")
	}
	if fake.savedGuidancePlanState.StateStatus != "user_patched" {
		t.Fatalf("saved state_status = %v, want user_patched", fake.savedGuidancePlanState.StateStatus)
	}
	if fake.savedGuidancePlanState.LastTurn != 5 {
		t.Fatalf("saved last_turn = %d, want 5", fake.savedGuidancePlanState.LastTurn)
	}
}

func TestDirectorPatchUserPatchedCacheProtection(t *testing.T) {
	cachedDirector := map[string]any{
		"scene_mandate":       "User patched mandate",
		"required_outcomes":   []string{"User outcome"},
		"forbidden_moves":     []string{},
		"pressure_level":      "strong",
		"execution_checklist": []any{},
		"persona_guardrails":  []any{},
		"world_guardrails":    []any{},
		"focus_characters":    []any{},
		"last_turn":           float64(5),
		"state_status":        "ready",
		"resolved_outcomes":   []string{},
		"expired_forbidden":   []string{},
	}
	cachedPlan := map[string]any{
		"current_arc":        "Arc A",
		"narrative_goal":     "goal",
		"active_tensions":    []any{},
		"next_beats":         []any{},
		"continuity_anchors": []any{},
		"guardrails":         []any{},
		"persona_priorities": []any{},
		"execution_notes":    []any{},
		"focus_characters":   []any{},
		"last_plan_turn":     float64(5),
		"state_status":       "heuristic",
	}
	dirJSON, _ := json.Marshal(cachedDirector)
	planJSON, _ := json.Marshal(cachedPlan)
	warnJSON, _ := json.Marshal([]any{})

	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-userpatched",
			StoryPlanJSON: string(planJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "user_patched",
			LastTurn:      5,
			WarningsJSON:  string(warnJSON),
		},
		storylines: []store.Storyline{
			{ChatSessionID: "sess-userpatched", Name: "Arc B", LastTurn: 20, FirstTurn: 6},
		},
	}

	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/narrative-control/sess-userpatched", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["state_status"] != "user_patched" {
		t.Fatalf("state_status = %v, want user_patched", resp["state_status"])
	}
	director, _ := resp["director"].(map[string]any)
	if director["scene_mandate"] != "User patched mandate" {
		t.Fatalf("scene_mandate = %v, want User patched mandate (cache protected)", director["scene_mandate"])
	}
	if fake.savedGuidancePlanState != nil {
		t.Fatal("expected no upsert when user_patched cache is protected")
	}
}

func TestNarrativeControlGetDirectorFreshCacheKeepsCompactHistory(t *testing.T) {
	cachedPlan := map[string]any{
		"current_arc":        "Arc A",
		"narrative_goal":     "hold continuity",
		"active_tensions":    []string{"tension"},
		"next_beats":         []string{"next beat"},
		"continuity_anchors": []string{"anchor"},
		"last_plan_turn":     9,
		"state_status":       "ready",
	}
	cachedDirector := map[string]any{
		"scene_mandate":       "Continue arc: Arc A",
		"required_outcomes":   []string{"Carry forward: Hook A"},
		"forbidden_moves":     []string{"Do not abruptly resolve: Risk A"},
		"pressure_level":      "steady",
		"execution_checklist": []string{"Deliver a visible beat."},
		"persona_guardrails":  []string{"[Chloe] speaks dry"},
		"world_guardrails":    []string{"World rule [gravity]: stable"},
		"focus_characters":    []string{"Chloe"},
		"last_turn":           9,
		"state_status":        "ready",
		"resolved_outcomes":   []string{"Carry forward: Old Hook"},
		"expired_forbidden":   []string{"Do not abruptly resolve: Old Risk"},
	}
	planJSON, _ := json.Marshal(cachedPlan)
	dirJSON, _ := json.Marshal(cachedDirector)
	warnJSON, _ := json.Marshal([]any{})
	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-k3-cache",
			StoryPlanJSON: string(planJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "ready",
			LastTurn:      9,
			WarningsJSON:  string(warnJSON),
		},
		storylines: []store.Storyline{
			{ChatSessionID: "sess-k3-cache", Name: "Arc A", Status: "active", LastTurn: 9, FirstTurn: 1},
		},
	}

	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/narrative-control/sess-k3-cache", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	director, _ := resp["director"].(map[string]any)
	if !containsAnyStringValue(director["required_outcomes"].([]any), "Carry forward: Hook A") {
		t.Fatalf("required_outcomes not preserved: %+v", director["required_outcomes"])
	}
	if !containsAnyStringValue(director["forbidden_moves"].([]any), "Do not abruptly resolve: Risk A") {
		t.Fatalf("forbidden_moves not preserved: %+v", director["forbidden_moves"])
	}
	if !containsAnyStringValue(director["resolved_outcomes"].([]any), "Carry forward: Old Hook") {
		t.Fatalf("resolved_outcomes not preserved: %+v", director["resolved_outcomes"])
	}
	if !containsAnyStringValue(director["expired_forbidden"].([]any), "Do not abruptly resolve: Old Risk") {
		t.Fatalf("expired_forbidden not preserved: %+v", director["expired_forbidden"])
	}
	if fake.savedGuidancePlanState != nil {
		t.Fatal("fresh director cache should not be rewritten")
	}
}

func TestDirectorPatchPreservesStoryPlanWarningsAndIgnoresUnknownFields(t *testing.T) {
	prevDirector := map[string]any{
		"scene_mandate":       "Old mandate",
		"required_outcomes":   []string{},
		"forbidden_moves":     []string{},
		"pressure_level":      "light",
		"execution_checklist": []any{},
		"persona_guardrails":  []any{},
		"world_guardrails":    []any{},
		"resolved_outcomes":   []string{},
		"expired_forbidden":   []string{},
	}
	prevPlan := map[string]any{
		"current_arc": "Arc A",
		"next_beats":  []string{"do not lose this"},
	}
	dirJSON, _ := json.Marshal(prevDirector)
	planJSON, _ := json.Marshal(prevPlan)
	warnJSON, _ := json.Marshal([]string{"keep warning"})
	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-k3-patch-preserve",
			StoryPlanJSON: string(planJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "ready",
			LastTurn:      4,
			WarningsJSON:  string(warnJSON),
		},
	}

	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"scene_mandate":"Patched","story_plan":{"current_arc":"malicious overwrite"},"unknown_field":"ignored"}`
	req := httptest.NewRequest(http.MethodPatch, "/narrative-control/sess-k3-patch-preserve/director-patch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	if fake.savedGuidancePlanState == nil {
		t.Fatal("expected saved guidance state")
	}
	if fake.savedGuidancePlanState.StoryPlanJSON != string(planJSON) {
		t.Fatalf("story plan was not preserved: %s", fake.savedGuidancePlanState.StoryPlanJSON)
	}
	if fake.savedGuidancePlanState.WarningsJSON != string(warnJSON) {
		t.Fatalf("warnings were not preserved: %s", fake.savedGuidancePlanState.WarningsJSON)
	}
	var savedDirector map[string]any
	if err := json.Unmarshal([]byte(fake.savedGuidancePlanState.DirectorJSON), &savedDirector); err != nil {
		t.Fatalf("saved director unmarshal: %v", err)
	}
	if savedDirector["story_plan"] != nil || savedDirector["unknown_field"] != nil {
		t.Fatalf("unknown fields leaked into director: %+v", savedDirector)
	}
	if savedDirector["scene_mandate"] != "Patched" {
		t.Fatalf("scene_mandate = %v, want Patched", savedDirector["scene_mandate"])
	}
}

func TestNarrativeControlGetDirectorPressureStrongFromPinnedHooks(t *testing.T) {
	fake := &narrativeFakeStore{
		storylines: []store.Storyline{
			{ChatSessionID: "sess-k3-pressure", Name: "Arc Pressure", Status: "active", LastTurn: 10, FirstTurn: 1},
		},
		pendingThreads: []store.PendingThread{
			{ChatSessionID: "sess-k3-pressure", Title: "Hook A", ThreadType: "promise", Status: "open", Pinned: true, LastSeenTurn: 10},
			{ChatSessionID: "sess-k3-pressure", Title: "Hook B", ThreadType: "promise", Status: "open", Pinned: true, LastSeenTurn: 10},
		},
	}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/narrative-control/sess-k3-pressure", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	director, _ := resp["director"].(map[string]any)
	if director["pressure_level"] != "strong" {
		t.Fatalf("pressure_level = %v, want strong", director["pressure_level"])
	}
	required, _ := director["required_outcomes"].([]any)
	if len(required) < 2 {
		t.Fatalf("required_outcomes = %+v, want two pinned hook carry targets", required)
	}
}

func TestNarrativeControlGetDirectorGuardrailsBoundedAndPersonaFromCharacterState(t *testing.T) {
	fake := &narrativeFakeStore{
		storylines: []store.Storyline{
			{
				ChatSessionID:  "sess-k3-guardrails",
				Name:           "Arc A",
				Status:         "active",
				CurrentContext: "Chloe weighs the next move.",
				EntitiesJSON:   `["Chloe"]`,
				LastTurn:       12,
				FirstTurn:      1,
			},
		},
		pendingThreads: []store.PendingThread{
			{ChatSessionID: "sess-k3-guardrails", Title: "Hook A", ThreadType: "promise", Status: "open", Pinned: true, LastSeenTurn: 12},
			{ChatSessionID: "sess-k3-guardrails", Title: "Risk A", HookType: "risk", Status: "open", LastSeenTurn: 11},
		},
		characterStates: []store.CharacterState{
			{
				ChatSessionID:   "sess-k3-guardrails",
				CharacterName:   "Chloe",
				SpeechStyleJSON: `{"default_tone":"dry","speech_notes":"short replies"}`,
				PersonalityJSON: `{"core_trait":"guarded"}`,
				TurnIndex:       12,
			},
		},
		worldRules: []store.WorldRule{
			{ID: 1, ChatSessionID: "sess-k3-guardrails", Category: "physics", Key: "gravity", ValueJSON: `{"description":"gravity is stable"}`},
			{ID: 2, ChatSessionID: "sess-k3-guardrails", Category: "systems", Key: "oaths", ValueJSON: `{"description":"oaths bind public action"}`},
			{ID: 3, ChatSessionID: "sess-k3-guardrails", Category: "exists", Key: "tower", ValueJSON: `{"description":"the tower exists"}`},
			{ID: 4, ChatSessionID: "sess-k3-guardrails", Category: "physics", Key: "rain", ValueJSON: `{"description":"rain muffles sound"}`},
			{ID: 5, ChatSessionID: "sess-k3-guardrails", Category: "hidden", Key: "secret", ValueJSON: `{"description":"must not carry"}`},
			{ID: 6, ChatSessionID: "sess-k3-guardrails", Category: "physics", Key: "suppressed", ValueJSON: `{"description":"suppressed"}`, Suppressed: true},
		},
	}

	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/narrative-control/sess-k3-guardrails?current_user_input=walk+away", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	director, ok := resp["director"].(map[string]any)
	if !ok {
		t.Fatalf("director missing: %+v", resp)
	}
	required, _ := director["required_outcomes"].([]any)
	if !containsAnyStringValue(required, "Carry forward: Hook A") {
		t.Fatalf("required_outcomes = %+v, want Hook A carry-forward", required)
	}
	forbidden, _ := director["forbidden_moves"].([]any)
	if !containsAnyStringValue(forbidden, "Do not abruptly resolve: Risk A") {
		t.Fatalf("forbidden_moves = %+v, want Risk A guard", forbidden)
	}
	executionChecklist, _ := director["execution_checklist"].([]any)
	if len(executionChecklist) == 0 || len(executionChecklist) > 4 {
		t.Fatalf("execution_checklist len = %d, want 1..4: %+v", len(executionChecklist), executionChecklist)
	}
	worldGuardrails, _ := director["world_guardrails"].([]any)
	if len(worldGuardrails) == 0 || len(worldGuardrails) > 4 {
		t.Fatalf("world_guardrails len = %d, want 1..4: %+v", len(worldGuardrails), worldGuardrails)
	}
	if containsAnyStringSubstring(worldGuardrails, "secret") || containsAnyStringSubstring(worldGuardrails, "suppressed") {
		t.Fatalf("world_guardrails carried invalid/suppressed rule: %+v", worldGuardrails)
	}
	personaGuardrails, _ := director["persona_guardrails"].([]any)
	if !containsAnyStringSubstring(personaGuardrails, "Chloe") ||
		!containsAnyStringSubstring(personaGuardrails, "dry") ||
		!containsAnyStringSubstring(personaGuardrails, "guarded") {
		t.Fatalf("persona_guardrails = %+v, want Chloe speech/personality hints", personaGuardrails)
	}
	guidance, ok := resp["story_guidance"].(map[string]any)
	if !ok {
		t.Fatalf("story_guidance missing: %+v", resp)
	}
	conflictPolicy, _ := guidance["conflict_policy"].(map[string]any)
	if conflictPolicy["guidance_may_override_user_input"] != false {
		t.Fatalf("persona/world guidance can override user input: %+v", conflictPolicy)
	}
}

func containsAnyStringSubstring(items []any, needle string) bool {
	for _, item := range items {
		if strings.Contains(fmt.Sprint(item), needle) {
			return true
		}
	}
	return false
}

func TestSessionGuidanceSnapshotCachedActive(t *testing.T) {
	spJSON, _ := json.Marshal(map[string]any{"current_arc": "Arc A", "narrative_goal": "goal A"})
	dirJSON, _ := json.Marshal(map[string]any{"scene_mandate": "Continue"})
	warnJSON, _ := json.Marshal([]any{})

	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-gs-active",
			StoryPlanJSON: string(spJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "ready",
			LastTurn:      10,
			WarningsJSON:  string(warnJSON),
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/sessions/sess-gs-active/guidance-snapshot", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("status = %v, want ok", resp["status"])
	}
	if resp["state_status"] != "active" {
		t.Fatalf("state_status = %v, want active", resp["state_status"])
	}
	if resp["last_turn"] != float64(10) {
		t.Fatalf("last_turn = %v, want 10", resp["last_turn"])
	}
	plan, ok := resp["story_plan"].(map[string]any)
	if !ok {
		t.Fatal("story_plan is not an object")
	}
	if plan["current_arc"] != "Arc A" {
		t.Fatalf("current_arc = %v, want Arc A", plan["current_arc"])
	}
	warnings, _ := resp["warnings"].([]any)
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want empty", warnings)
	}
}

func TestSessionGuidanceSnapshotCachedEmpty(t *testing.T) {
	spJSON, _ := json.Marshal(map[string]any{"current_arc": ""})
	dirJSON, _ := json.Marshal(map[string]any{"scene_mandate": ""})
	warnJSON, _ := json.Marshal([]any{})

	fake := &narrativeFakeStore{
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-gs-empty",
			StoryPlanJSON: string(spJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "empty",
			LastTurn:      -1,
			WarningsJSON:  string(warnJSON),
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/sessions/sess-gs-empty/guidance-snapshot", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["state_status"] != "empty" {
		t.Fatalf("state_status = %v, want empty", resp["state_status"])
	}
	if resp["last_turn"] != float64(-1) {
		t.Fatalf("last_turn = %v, want -1", resp["last_turn"])
	}
	warnings, _ := resp["warnings"].([]any)
	found := false
	for _, w := range warnings {
		if strings.Contains(fmt.Sprint(w), "rebuild will be triggered by next GET /narrative-control call") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected rebuild warning in warnings: %v", warnings)
	}
}

func TestSessionGuidanceSnapshotNoStateDegrade(t *testing.T) {
	fake := &narrativeFakeStore{}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/sessions/sess-gs-nostate/guidance-snapshot", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("status = %v, want ok", resp["status"])
	}
	if resp["state_status"] != "no_state" {
		t.Fatalf("state_status = %v, want no_state", resp["state_status"])
	}
	if resp["last_turn"] != float64(-1) {
		t.Fatalf("last_turn = %v, want -1", resp["last_turn"])
	}
	plan, ok := resp["story_plan"].(map[string]any)
	if !ok || len(plan) != 0 {
		t.Fatalf("story_plan = %v, want empty object", resp["story_plan"])
	}
	warnings, _ := resp["warnings"].([]any)
	found := false
	for _, w := range warnings {
		if strings.Contains(fmt.Sprint(w), "No cached guidance plan state found") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected degrade warning in warnings: %v", warnings)
	}
}

func TestSessionExportIncludesGuidanceSnapshot(t *testing.T) {
	spJSON, _ := json.Marshal(map[string]any{"current_arc": "Arc A"})
	dirJSON, _ := json.Marshal(map[string]any{"scene_mandate": "Continue"})
	warnJSON, _ := json.Marshal([]any{})

	fake := &narrativeFakeStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-export-gs", TurnIndex: 1, Role: "user", Content: "hello"},
		},
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-export-gs",
			StoryPlanJSON: string(spJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "ready",
			LastTurn:      7,
			WarningsJSON:  string(warnJSON),
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/sessions/sess-export-gs/export", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("status = %v, want ok", resp["status"])
	}
	gs, ok := resp["guidance_snapshot"].(map[string]any)
	if !ok {
		t.Fatalf("guidance_snapshot is not an object: %v", resp["guidance_snapshot"])
	}
	if gs["state_status"] != "active" {
		t.Fatalf("guidance_snapshot.state_status = %v, want active", gs["state_status"])
	}
	if gs["last_turn"] != float64(7) {
		t.Fatalf("guidance_snapshot.last_turn = %v, want 7", gs["last_turn"])
	}
	plan, ok := gs["story_plan"].(map[string]any)
	if !ok {
		t.Fatal("guidance_snapshot.story_plan is not an object")
	}
	if plan["current_arc"] != "Arc A" {
		t.Fatalf("current_arc = %v, want Arc A", plan["current_arc"])
	}
}

// TestSeq13P222ExportPackageLogicalEventDecisionMarkers verifies P222:
// Export endpoint returns a logical event package with portability contract,
// manual-first deferred copy detection, lineage surfaces, artifact exclusions,
// Chroma-compatible retrieval lane defaults, and selective rebuild handoff.
func TestSeq13P222ExportPackageLogicalEventDecisionMarkers(t *testing.T) {
	fake := &narrativeFakeStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-p222", TurnIndex: 1, Role: "user", Content: "hello"},
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/sessions/sess-p222/export", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["export_version"] != "1.1" {
		t.Fatalf("export_version = %v, want 1.1", resp["export_version"])
	}
	contract, ok := resp["portability_contract"].(map[string]any)
	if !ok {
		t.Fatalf("portability_contract missing")
	}
	if contract["package_mode"] != "logical_event_package" {
		t.Fatalf("package_mode = %v, want logical_event_package", contract["package_mode"])
	}
	if contract["db_snapshot_policy"] != "admin_full_profile_explicit_only" {
		t.Fatalf("db_snapshot_policy = %v, want admin_full_profile_explicit_only", contract["db_snapshot_policy"])
	}
	if contract["db_snapshot_default_included"] != false {
		t.Fatalf("db_snapshot_default_included = %v, want false", contract["db_snapshot_default_included"])
	}
	if contract["runtime_artifact_policy"] != "exclude_cache_temp_logs_downloads_git_runtime_proofs" {
		t.Fatalf("runtime_artifact_policy = %v", contract["runtime_artifact_policy"])
	}
	if contract["vector_artifact_policy"] != "exclude_from_default_package_rebuildable_retrieval_artifact" {
		t.Fatalf("vector_artifact_policy = %v", contract["vector_artifact_policy"])
	}
	if contract["canonical_truth_authority"] != "mariadb_store" {
		t.Fatalf("canonical_truth_authority = %v, want mariadb_store", contract["canonical_truth_authority"])
	}
	if contract["vector_retrieval_lane"] != "chromadb_only" {
		t.Fatalf("vector_retrieval_lane = %v, want chromadb_only", contract["vector_retrieval_lane"])
	}
	if contract["vector_engine_policy"] != "chromadb_only" {
		t.Fatalf("vector_engine_policy = %v, want chromadb_only", contract["vector_engine_policy"])
	}
	if _, ok := contract["milvus_lite_policy"]; ok {
		t.Fatalf("milvus_lite_policy should not be exposed in 2.0 runtime contract: %+v", contract)
	}
	if contract["manual_first"] != true {
		t.Fatalf("manual_first = %v, want true", contract["manual_first"])
	}
	if contract["auto_copy_detection"] != "deferred" {
		t.Fatalf("auto_copy_detection = %v, want deferred", contract["auto_copy_detection"])
	}
	if contract["session_origin"] != "sess-p222" {
		t.Fatalf("session_origin = %v, want sess-p222", contract["session_origin"])
	}
	portable, ok := contract["portable_units"].([]any)
	if !ok || len(portable) == 0 {
		t.Fatalf("portable_units missing or empty")
	}
	lineage, ok := contract["lineage_surfaces"].([]any)
	if !ok || len(lineage) == 0 {
		t.Fatalf("lineage_surfaces missing or empty")
	}
	handoff, ok := contract["rebuild_handoff"].(map[string]any)
	if !ok {
		t.Fatalf("rebuild_handoff missing")
	}
	if handoff["dirty_event_type"] != "backfill_import" {
		t.Fatalf("rebuild_handoff.dirty_event_type = %v, want backfill_import", handoff["dirty_event_type"])
	}
	if handoff["rebuild_mode"] != "selective" {
		t.Fatalf("rebuild_handoff.rebuild_mode = %v, want selective", handoff["rebuild_mode"])
	}
	if handoff["start_point"] != "next_prepare_turn_fetch" {
		t.Fatalf("rebuild_handoff.start_point = %v, want next_prepare_turn_fetch", handoff["start_point"])
	}
}

func TestSessionStateIncludesGuidanceSnapshot(t *testing.T) {
	spJSON, _ := json.Marshal(map[string]any{"current_arc": "State Arc"})
	dirJSON, _ := json.Marshal(map[string]any{"scene_mandate": "Keep state aligned"})
	fake := &narrativeFakeStore{
		activeStates: []store.ActiveState{
			{ID: 1, ChatSessionID: "sess-state-gs", StateType: "scene_state", Content: "{}", TurnIndex: 3},
		},
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-state-gs",
			StoryPlanJSON: string(spJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "ready",
			LastTurn:      3,
		},
	}
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/session-state/sess-state-gs", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	gs, ok := resp["guidance_snapshot"].(map[string]any)
	if !ok {
		t.Fatalf("guidance_snapshot missing: %#v", resp)
	}
	if gs["state_status"] != "active" || gs["last_turn"] != float64(3) {
		t.Fatalf("guidance_snapshot mismatch: %#v", gs)
	}
	meta := resp["section_meta"].(map[string]any)
	gsMeta := meta["guidance_snapshot"].(map[string]any)
	if gsMeta["ready"] != true {
		t.Fatalf("guidance_snapshot meta not ready: %#v", gsMeta)
	}
}

func TestSessionStep7HealthSeededLongSession(t *testing.T) {
	// Seed >=10 chat logs
	chatLogs := make([]store.ChatLog, 12)
	for i := 0; i < 12; i++ {
		chatLogs[i] = store.ChatLog{ID: int64(i + 1), ChatSessionID: "sess-l5", TurnIndex: i + 1, Role: "user", Content: "msg"}
	}

	// Seed 8 maintenance_enqueued audit logs
	auditLogs := make([]store.AuditLog, 8)
	for i := 0; i < 8; i++ {
		auditLogs[i] = store.AuditLog{
			ID:            int64(i + 1),
			ChatSessionID: "sess-l5",
			EventType:     "maintenance_enqueued",
			DetailsJSON:   `{"suggestion":"ok"}`,
		}
	}

	spJSON, _ := json.Marshal(map[string]any{
		"next_beats":      []any{"approach the rooftop", "check the alleyway"},
		"active_tensions": []any{"tensionA"},
	})
	dirJSON, _ := json.Marshal(map[string]any{
		"scene_mandate":     "Rooftop confrontation",
		"required_outcomes": []any{"keep rooftop promise", "preserve hesitation"},
		"forbidden_moves":   []any{"do not jump scene", "do not erase tension", "do not resolve offscreen"},
		"resolved_outcomes": []any{"old staircase beat resolved"},
	})
	warnJSON, _ := json.Marshal([]any{})

	fake := &narrativeFakeStore{
		chatLogs: chatLogs,
		storylines: []store.Storyline{
			{ChatSessionID: "sess-l5", Name: "Arc A", Status: "resolved", FirstTurn: 1, LastTurn: 5, Confidence: 0.9, EvidenceCount: 2},
		},
		pendingThreads: []store.PendingThread{
			{ChatSessionID: "sess-l5", ThreadKey: "hook-1", Status: "resolved", ResolvedTurn: 4, Pinned: true},
		},
		guidancePlanState: &store.GuidancePlanState{
			ChatSessionID: "sess-l5",
			StoryPlanJSON: string(spJSON),
			DirectorJSON:  string(dirJSON),
			StateStatus:   "active",
			LastTurn:      7,
			WarningsJSON:  string(warnJSON),
		},
		auditLogs: auditLogs,
	}

	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/sessions/sess-l5/step7-health", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("status = %v, want ok", resp["status"])
	}
	if resp["total_turns"] != float64(12) {
		t.Fatalf("total_turns = %v, want 12", resp["total_turns"])
	}

	gs, ok := resp["guidance_state"].(map[string]any)
	if !ok {
		t.Fatal("guidance_state missing")
	}
	if gs["status"] != "active" {
		t.Fatalf("guidance_state.status = %v, want active", gs["status"])
	}
	if gs["last_built_turn"] != float64(7) {
		t.Fatalf("last_built_turn = %v, want 7", gs["last_built_turn"])
	}
	if gs["arc_age_turns"] != float64(5) {
		t.Fatalf("arc_age_turns = %v, want 5", gs["arc_age_turns"])
	}
	if gs["active_tensions"] != float64(1) {
		t.Fatalf("active_tensions = %v, want 1", gs["active_tensions"])
	}
	if gs["next_beats"] != float64(2) {
		t.Fatalf("next_beats = %v, want 2", gs["next_beats"])
	}
	if gs["open_required"] != float64(2) {
		t.Fatalf("open_required = %v, want 2", gs["open_required"])
	}
	if gs["forbidden_count"] != float64(3) {
		t.Fatalf("forbidden_count = %v, want 3", gs["forbidden_count"])
	}

	cs, ok := resp["compaction_summary"].(map[string]any)
	if !ok {
		t.Fatal("compaction_summary missing")
	}
	if cs["total_records"] == float64(0) {
		t.Fatalf("compaction_summary.total_records = 0, want >0")
	}

	ms, ok := resp["maintenance_summary"].(map[string]any)
	if !ok {
		t.Fatal("maintenance_summary missing")
	}
	if ms["total_passes"] != float64(8) {
		t.Fatalf("maintenance_summary.total_passes = %v, want 8", ms["total_passes"])
	}
	if ms["ok_count"] != float64(8) {
		t.Fatalf("maintenance_summary.ok_count = %v, want 8", ms["ok_count"])
	}
	if ms["ok_rate"] != float64(1.0) {
		t.Fatalf("maintenance_summary.ok_rate = %v, want 1.0", ms["ok_rate"])
	}
	ds, ok := resp["drift_summary"].(map[string]any)
	if !ok {
		t.Fatal("drift_summary missing")
	}
	if ds["passes_analyzed"] != float64(8) || ds["high_severity"] != float64(0) {
		t.Fatalf("drift_summary mismatch: %#v", ds)
	}

	rc, ok := resp["regression_checks"].(map[string]any)
	if !ok {
		t.Fatal("regression_checks missing")
	}
	if rc["guidance_persistence"] != "pass" {
		t.Fatalf("guidance_persistence = %v, want pass", rc["guidance_persistence"])
	}
	if rc["arc_stability"] != "pass" {
		t.Fatalf("arc_stability = %v, want pass", rc["arc_stability"])
	}
	if rc["compaction_health"] != "pass" {
		t.Fatalf("compaction_health = %v, want pass", rc["compaction_health"])
	}
	if rc["maintenance_effect"] != "pass" {
		t.Fatalf("maintenance_effect = %v, want pass", rc["maintenance_effect"])
	}

	warnings, _ := resp["warnings"].([]any)
	if len(warnings) != 0 {
		t.Fatalf("expected empty warnings, got %v", warnings)
	}
}

func TestChapterSummaryStoreAndExportContract(t *testing.T) {
	cs := store.ChapterSummary{
		ID:                      1,
		ChatSessionID:           "sess-p174",
		FromTurn:                1,
		ToTurn:                  60,
		ChapterIndex:            1,
		ChapterTitle:            "Gate",
		SummaryText:             "Alice opens the gate.",
		OpenLoopsJSON:           `["gate"]`,
		RelationshipChangesJSON: `["Alice trusts Bob"]`,
		WorldChangesJSON:        `["gate opens"]`,
		CallbackCandidatesJSON:  `["sealed ledger"]`,
		ResumeText:              "Resume for turns 1-60.",
		EmbeddingVector:         "vec",
		EmbeddingModel:          "model",
	}
	if cs.ChatSessionID != "sess-p174" || cs.FromTurn != 1 || cs.ToTurn != 60 {
		t.Fatalf("ChapterSummary contract fields mismatch: %+v", cs)
	}
	rp := store.ResumePack{Chapter: &cs}
	if rp.Chapter == nil || rp.Chapter.ChapterTitle != "Gate" {
		t.Fatalf("ResumePack.Chapter link broken")
	}
}

func TestEpisodeGeneratePersistsDS1aStructuredAnchors(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Cfg.StoreMode = config.StoreModeMariaDBAuthority
	fake := &narrativeFakeStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-ds1a", TurnIndex: 1, Role: "user", Content: "Alice asks Bob to open the sealed gate.", CreatedAt: time.Unix(10, 0)},
			{ID: 2, ChatSessionID: "sess-ds1a", TurnIndex: 2, Role: "assistant", Content: "Bob keeps his promise, Alice trusts Bob, but the sealed gate remains unresolved.", CreatedAt: time.Unix(11, 0)},
		},
	}
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/episodes/generate", strings.NewReader(`{"chat_session_id":"sess-ds1a","from_turn":1,"to_turn":2}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "ok" || resp["saved"] != true {
		t.Fatalf("expected saved ok episode response, got %#v", resp)
	}
	if len(fake.savedEpisodeSummaries) != 1 {
		t.Fatalf("savedEpisodeSummaries = %d, want 1", len(fake.savedEpisodeSummaries))
	}
	saved := fake.savedEpisodeSummaries[0]
	if saved.SummaryText == "" || saved.KeyEvents == "" {
		t.Fatalf("episode summary/key events missing: %+v", saved)
	}
	if !strings.Contains(saved.KeyEvents, "Alice asks Bob") {
		t.Fatalf("key_events did not preserve event anchor: %s", saved.KeyEvents)
	}
	if !strings.Contains(saved.RelationshipChangesJSON, "Alice trusts Bob") {
		t.Fatalf("relationship_changes_json did not preserve relationship anchor: %s", saved.RelationshipChangesJSON)
	}
	if !strings.Contains(saved.OpenLoopsJSON, "sealed gate") {
		t.Fatalf("open_loops_json did not preserve open-loop anchor: %s", saved.OpenLoopsJSON)
	}
	trace, _ := resp["generation_trace"].(map[string]any)
	if trace["dense_summary_contract"] != "ds1a.v1" {
		t.Fatalf("generation_trace missing ds1a contract: %+v", trace)
	}
}

func TestEpisodeRegenerateReplacesExistingRange(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Cfg.StoreMode = config.StoreModeMariaDBAuthority
	fake := &narrativeFakeStore{
		chatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-regen", TurnIndex: 1, Role: "user", Content: "Alice reaches the broken bridge.", CreatedAt: time.Unix(10, 0)},
			{ID: 2, ChatSessionID: "sess-regen", TurnIndex: 2, Role: "assistant", Content: "Bob repairs a cable while the bridge remains unsafe.", CreatedAt: time.Unix(11, 0)},
		},
		episodeSummaries: []store.EpisodeSummary{
			{ID: 7, ChatSessionID: "sess-regen", FromTurn: 1, ToTurn: 2, SummaryText: "old truncated episode"},
		},
	}
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/episodes/regenerate", strings.NewReader(`{"chat_session_id":"sess-regen","from_turn":1,"to_turn":2}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "ok" || resp["code"] != "episode_regenerated" || resp["saved"] != true {
		t.Fatalf("expected regenerated ok response, got %#v", resp)
	}
	if got := fmt.Sprint(fake.deletedEpisodeRanges); !strings.Contains(got, "sess-regen:1:2") {
		t.Fatalf("range delete not recorded: %s", got)
	}
	if len(fake.savedEpisodeSummaries) != 1 {
		t.Fatalf("savedEpisodeSummaries = %d, want 1", len(fake.savedEpisodeSummaries))
	}
	if strings.Contains(fake.savedEpisodeSummaries[0].SummaryText, "old truncated") {
		t.Fatalf("regenerated summary kept old text: %+v", fake.savedEpisodeSummaries[0])
	}
}

func TestEpisodeDeleteRemovesByID(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Cfg.StoreMode = config.StoreModeMariaDBAuthority
	fake := &narrativeFakeStore{
		episodeSummaries: []store.EpisodeSummary{
			{ID: 42, ChatSessionID: "sess-del", FromTurn: 1, ToTurn: 2, SummaryText: "delete me"},
		},
	}
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodDelete, "/episodes/42", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "ok" || resp["deleted"] != true || fake.deletedEpisodeID != 42 {
		t.Fatalf("delete response/store mismatch: resp=%#v deleted=%d", resp, fake.deletedEpisodeID)
	}
	if len(fake.episodeSummaries) != 0 {
		t.Fatalf("episode not removed: %+v", fake.episodeSummaries)
	}
}

func TestChapterGenerateDuplicateCheckAndRollbackInvalidation(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	srv.Cfg.StoreMode = config.StoreModeMariaDBAuthority
	fake := &narrativeFakeStore{
		chapterSummaries: []store.ChapterSummary{
			{ChatSessionID: "sess-dup", FromTurn: 1, ToTurn: 60, ChapterIndex: 1, ChapterTitle: "Existing", SummaryText: "Already here.", ResumeText: "Resume."},
		},
		episodeSummaries: []store.EpisodeSummary{
			{ChatSessionID: "sess-dup", FromTurn: 1, ToTurn: 15, SummaryText: "E1"},
			{ChatSessionID: "sess-dup", FromTurn: 16, ToTurn: 30, SummaryText: "E2"},
			{ChatSessionID: "sess-dup", FromTurn: 31, ToTurn: 45, SummaryText: "E3"},
			{ChatSessionID: "sess-dup", FromTurn: 46, ToTurn: 60, SummaryText: "E4"},
		},
	}
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chapters/generate", strings.NewReader(`{"chat_session_id":"sess-dup","turn_index":60,"interval":60}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "skipped" {
		t.Fatalf("expected skipped due to duplicate, got %#v", resp)
	}
	if resp["already_exists"] != true {
		t.Fatalf("expected already_exists=true, got %#v", resp)
	}
	if len(fake.savedChapterSummaries) != 0 {
		t.Fatalf("expected no new chapter saved, got %d", len(fake.savedChapterSummaries))
	}

	req2 := httptest.NewRequest(http.MethodPost, "/chapters/generate", strings.NewReader(`{"chat_session_id":"sess-dup","turn_index":60,"interval":60,"force":true}`))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("force status = %d, want 200: %s", rec2.Code, rec2.Body.String())
	}
	var resp2 map[string]any
	if err := json.Unmarshal(rec2.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp2["status"] != "ok" {
		t.Fatalf("expected ok with force, got %#v", resp2)
	}
	if len(fake.savedChapterSummaries) != 1 {
		t.Fatalf("expected 1 new chapter saved with force, got %d", len(fake.savedChapterSummaries))
	}
}

func TestChapterExportAndSnapshotSurfacesIncludeChapters(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	fake := &narrativeFakeStore{
		chapterSummaries: []store.ChapterSummary{
			{ChatSessionID: "sess-exp", FromTurn: 1, ToTurn: 60, ChapterIndex: 1, ChapterTitle: "Gate", SummaryText: "Open.", ResumeText: "Resume."},
		},
	}
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/sessions/sess-exp/export", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("export status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var exp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &exp); err != nil {
		t.Fatalf("decode export: %v", err)
	}
	contract, ok := exp["portability_contract"].(map[string]any)
	if !ok {
		t.Fatalf("portability_contract missing")
	}
	portable, ok := contract["portable_units"].([]any)
	if !ok {
		t.Fatalf("portable_units missing")
	}
	found := false
	for _, u := range portable {
		if u == "chapter_summaries" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("chapter_summaries not in portable_units: %v", portable)
	}
	if _, ok := exp["chapter_summaries"]; !ok {
		t.Fatalf("chapter_summaries missing in export response")
	}

	req2 := httptest.NewRequest(http.MethodGet, "/session-state/sess-exp", nil)
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("state status = %d, want 200: %s", rec2.Code, rec2.Body.String())
	}
	var st map[string]any
	if err := json.Unmarshal(rec2.Body.Bytes(), &st); err != nil {
		t.Fatalf("decode state: %v", err)
	}
	if _, ok := st["chapter_summaries"]; !ok {
		t.Fatalf("chapter_summaries missing in session state")
	}
}

func TestChapterDryRunReturnsIntervalCheckAndInputStats(t *testing.T) {
	mux := http.NewServeMux()
	srv := setupTestServer()
	fake := &narrativeFakeStore{
		episodeSummaries: []store.EpisodeSummary{
			{ChatSessionID: "sess-dry", FromTurn: 1, ToTurn: 15, SummaryText: "E1"},
			{ChatSessionID: "sess-dry", FromTurn: 16, ToTurn: 30, SummaryText: "E2"},
			{ChatSessionID: "sess-dry", FromTurn: 31, ToTurn: 45, SummaryText: "E3"},
			{ChatSessionID: "sess-dry", FromTurn: 46, ToTurn: 60, SummaryText: "E4"},
		},
	}
	srv.Store = fake
	srv.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/chapters/dry-run", strings.NewReader(`{"chat_session_id":"sess-dry","turn_index":60,"interval":60}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "ok" {
		t.Fatalf("status = %v, want ok", resp["status"])
	}
	if resp["mode"] != "dry_run" {
		t.Fatalf("mode = %v, want dry_run", resp["mode"])
	}
	if resp["triggered"] != true {
		t.Fatalf("triggered = %v, want true", resp["triggered"])
	}
	ic, ok := resp["interval_check"].(map[string]any)
	if !ok {
		t.Fatalf("interval_check missing")
	}
	if _, ok := ic["range"]; !ok {
		t.Fatalf("interval_check.range missing")
	}
	stats, ok := resp["input_stats"].(map[string]any)
	if !ok {
		t.Fatalf("input_stats missing")
	}
	if stats["episode_count"] != float64(4) {
		t.Fatalf("episode_count = %v, want 4", stats["episode_count"])
	}
	if stats["episode_count_recommended"] != true {
		t.Fatalf("episode_count_recommended = %v, want true", stats["episode_count_recommended"])
	}
	ready, _ := resp["ready"].(bool)
	if !ready {
		t.Fatalf("ready = %v, want true", ready)
	}
	br, _ := resp["blocking_reasons"].([]any)
	if len(br) != 0 {
		t.Fatalf("expected empty blocking_reasons, got %v", br)
	}
}
