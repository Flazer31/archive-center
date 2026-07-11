package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/risulongmemory/archive-center-go/internal/config"
	"github.com/risulongmemory/archive-center-go/internal/store"
	"github.com/risulongmemory/archive-center-go/internal/vector"
)

// turnRecordingStore implements store.Store and records all save/read calls.
type turnRecordingStore struct {
	memoryFakeStore
	savedChatLogs           []*store.ChatLog
	savedEffectiveInputs    []*store.EffectiveInput
	savedAuditLogs          []*store.AuditLog
	savedCriticFeedback     []*store.CriticFeedback
	returnMemories          []store.Memory
	savedMemories           []*store.Memory
	updatedImportance       map[int64]float64
	savedEvidence           []*store.DirectEvidence
	savedKGTriples          []*store.KGTriple
	savedStorylines         []*store.Storyline
	savedWorldRules         []*store.WorldRule
	savedEntities           []*store.Entity
	savedTrusts             []*store.Trust
	savedCharacterEvents    []*store.CharacterEvent
	savedCharacterStates    []*store.CharacterState
	returnStatusDefinitions []store.StatusSchemaDefinition
	savedStatusDefinitions  []store.StatusSchemaDefinition
	returnStatusCurrent     []store.StatusCurrentValue
	savedStatusCurrent      []store.StatusCurrentValue
	savedStatusEvents       []store.StatusChangeEvent
	savedStatusEffects      []store.StatusEffect
	savedPendingThreads     []*store.PendingThread
	savedActiveStates       []*store.ActiveState
	savedCanonicalLayers    []*store.CanonicalStateLayer
	returnKGTriples         []store.KGTriple
	returnEvidence          []store.DirectEvidence
	returnChatLogs          []store.ChatLog
	returnResumePack        *store.ResumePack
	returnStorylines        []store.Storyline
	returnWorldRules        []store.WorldRule
	returnCharStates        []store.CharacterState
	returnPendingThreads    []store.PendingThread
	returnActiveStates      []store.ActiveState
	returnCanonicalLayers   []store.CanonicalStateLayer
	returnEpisodeSums       []store.EpisodeSummary
	returnPersonaEntries    []store.PersonaMemoryEntry
	returnEntityMemories    []store.ProtagonistEntityMemory
	lastEpisodeLimit        int
	lastPersonaLimit        int
	lastEntityMemoryLimit   int
	savedEntityMemories     []*store.ProtagonistEntityMemory
	createdPersonaCapsules  []*store.PersonaMemoryCapsule
	createdPersonaEntries   []store.PersonaMemoryEntry
	deletedStorylineIDs     []int64
	deletedWorldRuleIDs     []int64
}

type turnRecordingVectorStore struct {
	docs               []vector.VectorDocument
	deletedDocumentIDs []string
	upsertErr          error
}

func (f *turnRecordingVectorStore) Search(ctx context.Context, sessionID string, query []float32, limit int, filter string) ([]vector.VectorDocument, error) {
	return nil, vector.ErrNotFound
}

func (f *turnRecordingVectorStore) Upsert(ctx context.Context, sessionID string, docs []vector.VectorDocument) error {
	if f.upsertErr != nil {
		return f.upsertErr
	}
	f.docs = append(f.docs, docs...)
	return nil
}

func (f *turnRecordingVectorStore) DeleteSession(ctx context.Context, sessionID string) error {
	return nil
}

func (f *turnRecordingVectorStore) DeleteDocuments(ctx context.Context, ids []string) error {
	f.deletedDocumentIDs = append(f.deletedDocumentIDs, ids...)
	remove := map[string]bool{}
	for _, id := range ids {
		remove[id] = true
	}
	out := f.docs[:0]
	for _, doc := range f.docs {
		if !remove[doc.ID] {
			out = append(out, doc)
		}
	}
	f.docs = out
	return nil
}

func (f *turnRecordingVectorStore) Rebuild(ctx context.Context, sessionID string) error { return nil }

func (f *turnRecordingVectorStore) Health(ctx context.Context) (vector.HealthSnapshot, error) {
	return vector.HealthSnapshot{Status: "ok", Collection: "test"}, nil
}

func (f *turnRecordingVectorStore) Count(ctx context.Context, sessionID string) (int, error) {
	if sessionID == "" {
		return len(f.docs), nil
	}
	count := 0
	for _, doc := range f.docs {
		if doc.ChatSessionID == sessionID {
			count++
		}
	}
	return count, nil
}

func (f *turnRecordingVectorStore) ListDocuments(ctx context.Context, sessionID string) ([]vector.VectorDocument, error) {
	out := []vector.VectorDocument{}
	for _, doc := range f.docs {
		if sessionID == "" || doc.ChatSessionID == sessionID {
			out = append(out, doc)
		}
	}
	return out, nil
}

func (f *turnRecordingVectorStore) Close(ctx context.Context) error { return nil }

func (f *turnRecordingStore) SaveChatLog(ctx context.Context, log *store.ChatLog) error {
	f.savedChatLogs = append(f.savedChatLogs, log)
	return nil
}

func (f *turnRecordingStore) SaveEffectiveInput(ctx context.Context, in *store.EffectiveInput) error {
	f.savedEffectiveInputs = append(f.savedEffectiveInputs, in)
	return nil
}

func (f *turnRecordingStore) SaveAuditLog(ctx context.Context, a *store.AuditLog) error {
	f.savedAuditLogs = append(f.savedAuditLogs, a)
	return nil
}

func (f *turnRecordingStore) SaveCriticFeedback(ctx context.Context, cf *store.CriticFeedback) error {
	f.savedCriticFeedback = append(f.savedCriticFeedback, cf)
	return nil
}

func (f *turnRecordingStore) ListMemories(ctx context.Context, sid string, fromTurn, toTurn int) ([]store.Memory, error) {
	return f.returnMemories, nil
}

func (f *turnRecordingStore) DeleteMemoryByID(ctx context.Context, sid string, memoryID int64) error {
	for i := range f.returnMemories {
		if f.returnMemories[i].ID == memoryID && f.returnMemories[i].ChatSessionID == sid {
			f.returnMemories = append(f.returnMemories[:i], f.returnMemories[i+1:]...)
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *turnRecordingStore) UpdateMemoryImportance(ctx context.Context, chatSessionID string, memoryID int64, importance float64) error {
	if f.updatedImportance == nil {
		f.updatedImportance = map[int64]float64{}
	}
	f.updatedImportance[memoryID] = importance
	return nil
}

func (f *turnRecordingStore) ListKGTriples(ctx context.Context, sid string) ([]store.KGTriple, error) {
	return f.returnKGTriples, nil
}

func (f *turnRecordingStore) ListEvidence(ctx context.Context, sid string) ([]store.DirectEvidence, error) {
	return f.returnEvidence, nil
}

func (f *turnRecordingStore) ListChatLogs(ctx context.Context, sid string, fromTurn, toTurn int) ([]store.ChatLog, error) {
	return f.returnChatLogs, nil
}

func (f *turnRecordingStore) GetResumePack(ctx context.Context, sid, trigger string) (*store.ResumePack, error) {
	return f.returnResumePack, nil
}

func (f *turnRecordingStore) ListStorylines(ctx context.Context, sid string) ([]store.Storyline, error) {
	return f.returnStorylines, nil
}

func (f *turnRecordingStore) DeleteStoryline(ctx context.Context, storylineID int64) error {
	for i := range f.returnStorylines {
		if f.returnStorylines[i].ID == storylineID {
			f.deletedStorylineIDs = append(f.deletedStorylineIDs, storylineID)
			f.returnStorylines = append(f.returnStorylines[:i], f.returnStorylines[i+1:]...)
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *turnRecordingStore) ListWorldRules(ctx context.Context, sid string) ([]store.WorldRule, error) {
	return f.returnWorldRules, nil
}

func (f *turnRecordingStore) DeleteWorldRule(ctx context.Context, ruleID int64) error {
	for i := range f.returnWorldRules {
		if f.returnWorldRules[i].ID == ruleID {
			f.deletedWorldRuleIDs = append(f.deletedWorldRuleIDs, ruleID)
			f.returnWorldRules = append(f.returnWorldRules[:i], f.returnWorldRules[i+1:]...)
			return nil
		}
	}
	return store.ErrNotFound
}

func (f *turnRecordingStore) ListCharacterStates(ctx context.Context, sid string) ([]store.CharacterState, error) {
	return f.returnCharStates, nil
}

func (f *turnRecordingStore) GetCharacterState(ctx context.Context, sid, characterName string) (*store.CharacterState, error) {
	for _, item := range f.returnCharStates {
		if sid != "" && item.ChatSessionID != "" && item.ChatSessionID != sid {
			continue
		}
		if strings.EqualFold(item.CharacterName, characterName) {
			cp := item
			return &cp, nil
		}
	}
	return nil, store.ErrNotFound
}

func (f *turnRecordingStore) ListPendingThreads(ctx context.Context, sid, status string) ([]store.PendingThread, error) {
	return f.returnPendingThreads, nil
}

func (f *turnRecordingStore) ListActiveStates(ctx context.Context, sid, stateType string) ([]store.ActiveState, error) {
	return f.returnActiveStates, nil
}

func (f *turnRecordingStore) ListCanonicalStateLayers(ctx context.Context, sid, layerType string) ([]store.CanonicalStateLayer, error) {
	return f.returnCanonicalLayers, nil
}

func (f *turnRecordingStore) ListEpisodeSummaries(ctx context.Context, sid string, limit, fromTurn, toTurn int) ([]store.EpisodeSummary, error) {
	f.lastEpisodeLimit = limit
	return f.returnEpisodeSums, nil
}

func (f *turnRecordingStore) CreatePersonaMemoryCapsule(ctx context.Context, capsule *store.PersonaMemoryCapsule, entries []store.PersonaMemoryEntry) (*store.PersonaMemoryCapsule, error) {
	f.createdPersonaCapsules = append(f.createdPersonaCapsules, capsule)
	f.createdPersonaEntries = append(f.createdPersonaEntries, entries...)
	return capsule, nil
}

func (f *turnRecordingStore) ListPersonaMemoryCapsules(ctx context.Context, filter store.PersonaCapsuleFilter) ([]store.PersonaMemoryCapsule, error) {
	return nil, nil
}

func (f *turnRecordingStore) GetPersonaMemoryCapsule(ctx context.Context, capsuleID int64) (*store.PersonaMemoryCapsule, []store.PersonaMemoryEntry, error) {
	return nil, nil, store.ErrNotFound
}

func (f *turnRecordingStore) DeletePersonaMemoryCapsule(ctx context.Context, capsuleID int64) error {
	return nil
}

func (f *turnRecordingStore) AttachPersonaMemoryCapsule(ctx context.Context, attachment *store.PersonaCapsuleAttachment) error {
	return nil
}

func (f *turnRecordingStore) DetachPersonaMemoryCapsule(ctx context.Context, capsuleID int64, targetChatSessionID string) error {
	return nil
}

func (f *turnRecordingStore) ListPersonaCapsuleAttachments(ctx context.Context, targetChatSessionID string) ([]store.PersonaCapsuleAttachment, error) {
	return nil, nil
}

func (f *turnRecordingStore) ListAttachedPersonaMemoryEntries(ctx context.Context, targetChatSessionID string, limit int) ([]store.PersonaMemoryEntry, error) {
	f.lastPersonaLimit = limit
	return f.returnPersonaEntries, nil
}

func (f *turnRecordingStore) CreateProtagonistEntityMemory(ctx context.Context, item *store.ProtagonistEntityMemory) (*store.ProtagonistEntityMemory, error) {
	cp := *item
	if cp.ID <= 0 {
		cp.ID = int64(len(f.savedEntityMemories) + 1)
	}
	f.savedEntityMemories = append(f.savedEntityMemories, &cp)
	return &cp, nil
}

func (f *turnRecordingStore) ListProtagonistEntityMemories(ctx context.Context, filter store.ProtagonistEntityMemoryFilter) ([]store.ProtagonistEntityMemory, error) {
	f.lastEntityMemoryLimit = filter.Limit
	out := []store.ProtagonistEntityMemory{}
	for _, item := range f.returnEntityMemories {
		if filter.SourceChatSessionID != "" && item.SourceChatSessionID != filter.SourceChatSessionID {
			continue
		}
		if filter.OwnerEntityRole != "" && item.OwnerEntityRole != filter.OwnerEntityRole {
			continue
		}
		if filter.OwnerVisibility != "" && item.OwnerVisibility != filter.OwnerVisibility {
			continue
		}
		if filter.OwnerEntityKey != "" && item.OwnerEntityKey != filter.OwnerEntityKey {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (f *turnRecordingStore) SaveMemory(ctx context.Context, m *store.Memory) error {
	f.savedMemories = append(f.savedMemories, m)
	return nil
}

func (f *turnRecordingStore) SaveEvidence(ctx context.Context, e *store.DirectEvidence) error {
	if e.ID <= 0 {
		e.ID = int64(len(f.savedEvidence) + 1)
	}
	f.savedEvidence = append(f.savedEvidence, e)
	return nil
}

func (f *turnRecordingStore) SaveKGTriple(ctx context.Context, t *store.KGTriple) error {
	f.savedKGTriples = append(f.savedKGTriples, t)
	return nil
}

func (f *turnRecordingStore) SaveCharacterEvent(ctx context.Context, e *store.CharacterEvent) error {
	f.savedCharacterEvents = append(f.savedCharacterEvents, e)
	return nil
}

func (f *turnRecordingStore) SaveStoryline(ctx context.Context, s *store.Storyline) error {
	f.savedStorylines = append(f.savedStorylines, s)
	return nil
}

func (f *turnRecordingStore) SaveWorldRule(ctx context.Context, w *store.WorldRule) error {
	if w.ID <= 0 {
		w.ID = int64(len(f.savedWorldRules) + 1)
	}
	f.savedWorldRules = append(f.savedWorldRules, w)
	return nil
}

func (f *turnRecordingStore) SaveEntity(ctx context.Context, e *store.Entity) error {
	f.savedEntities = append(f.savedEntities, e)
	return nil
}

func (f *turnRecordingStore) SaveTrust(ctx context.Context, t *store.Trust) error {
	f.savedTrusts = append(f.savedTrusts, t)
	return nil
}

func (f *turnRecordingStore) SaveCharacterState(ctx context.Context, c *store.CharacterState) error {
	f.savedCharacterStates = append(f.savedCharacterStates, c)
	return nil
}

func (f *turnRecordingStore) GetStatusSchemaDefinitionByKey(ctx context.Context, chatSessionID, statusKey, ownerScope string) (store.StatusSchemaDefinition, error) {
	for _, item := range f.returnStatusDefinitions {
		if item.ChatSessionID == chatSessionID && item.StatusKey == statusKey && item.OwnerScope == ownerScope && item.RegistryState == "active" {
			return item, nil
		}
	}
	return store.StatusSchemaDefinition{}, store.ErrNotFound
}

func (f *turnRecordingStore) ListStatusSchemaDefinitions(ctx context.Context, chatSessionID, registryState string, limit int) ([]store.StatusSchemaDefinition, error) {
	out := []store.StatusSchemaDefinition{}
	for _, item := range f.returnStatusDefinitions {
		if chatSessionID != "" && item.ChatSessionID != chatSessionID {
			continue
		}
		if registryState != "" && item.RegistryState != registryState {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (f *turnRecordingStore) SaveStatusSchemaDefinitions(ctx context.Context, definitions []store.StatusSchemaDefinition) ([]store.StatusSchemaDefinition, error) {
	out := make([]store.StatusSchemaDefinition, 0, len(definitions))
	for _, item := range definitions {
		if item.ID <= 0 {
			item.ID = int64(len(f.savedStatusDefinitions) + len(out) + 1)
		}
		if item.RegistryState == "" {
			item.RegistryState = "active"
		}
		f.savedStatusDefinitions = append(f.savedStatusDefinitions, item)
		f.returnStatusDefinitions = append(f.returnStatusDefinitions, item)
		out = append(out, item)
	}
	return out, nil
}

func (f *turnRecordingStore) ListStatusCurrentValues(ctx context.Context, chatSessionID, ownerScope, ownerID, statusKey string, limit int) ([]store.StatusCurrentValue, error) {
	out := []store.StatusCurrentValue{}
	for _, item := range f.returnStatusCurrent {
		if chatSessionID != "" && item.ChatSessionID != chatSessionID {
			continue
		}
		if ownerScope != "" && item.OwnerScope != ownerScope {
			continue
		}
		if ownerID != "" && item.OwnerID != ownerID {
			continue
		}
		if statusKey != "" && item.StatusKey != statusKey {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (f *turnRecordingStore) SaveStatusCurrentValue(ctx context.Context, value store.StatusCurrentValue) (store.StatusCurrentValue, error) {
	if value.ID <= 0 {
		value.ID = int64(len(f.savedStatusCurrent) + 1)
	}
	f.savedStatusCurrent = append(f.savedStatusCurrent, value)
	replaced := false
	for i := range f.returnStatusCurrent {
		item := f.returnStatusCurrent[i]
		if item.ChatSessionID == value.ChatSessionID && item.RegistryID == value.RegistryID && item.OwnerScope == value.OwnerScope && item.OwnerID == value.OwnerID {
			f.returnStatusCurrent[i] = value
			replaced = true
			break
		}
	}
	if !replaced {
		f.returnStatusCurrent = append(f.returnStatusCurrent, value)
	}
	return value, nil
}

func (f *turnRecordingStore) ListStatusChangeEvents(ctx context.Context, chatSessionID, ownerScope, ownerID, statusKey string, limit int) ([]store.StatusChangeEvent, error) {
	return append([]store.StatusChangeEvent(nil), f.savedStatusEvents...), nil
}

func (f *turnRecordingStore) SaveStatusChangeEvent(ctx context.Context, event store.StatusChangeEvent) (store.StatusChangeEvent, error) {
	if event.ID <= 0 {
		event.ID = int64(len(f.savedStatusEvents) + 1)
	}
	f.savedStatusEvents = append(f.savedStatusEvents, event)
	return event, nil
}

func (f *turnRecordingStore) ListStatusEffects(ctx context.Context, chatSessionID, ownerScope, ownerID, effectState string, limit int) ([]store.StatusEffect, error) {
	out := []store.StatusEffect{}
	for _, item := range f.savedStatusEffects {
		if chatSessionID != "" && item.ChatSessionID != chatSessionID {
			continue
		}
		if ownerScope != "" && item.OwnerScope != ownerScope {
			continue
		}
		if ownerID != "" && item.OwnerID != ownerID {
			continue
		}
		if effectState != "" && item.EffectState != effectState {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (f *turnRecordingStore) SaveStatusEffect(ctx context.Context, effect store.StatusEffect) (store.StatusEffect, error) {
	if effect.ID <= 0 {
		effect.ID = int64(len(f.savedStatusEffects) + 1)
	}
	f.savedStatusEffects = append(f.savedStatusEffects, effect)
	return effect, nil
}

func (f *turnRecordingStore) UpdateStatusEffectState(ctx context.Context, id int64, effectState, clearedEvidenceJSON string, clearedTurn int) error {
	for i := range f.savedStatusEffects {
		if f.savedStatusEffects[i].ID == id {
			f.savedStatusEffects[i].EffectState = effectState
			f.savedStatusEffects[i].ClearedEvidenceJSON = clearedEvidenceJSON
			f.savedStatusEffects[i].ClearedTurn = clearedTurn
			return nil
		}
	}
	return store.ErrNotFound
}

type relationshipAccumulatingTurnStore struct {
	turnRecordingStore
	characterStates map[string]store.CharacterState
}

func newRelationshipAccumulatingTurnStore(initial []store.CharacterState) *relationshipAccumulatingTurnStore {
	f := &relationshipAccumulatingTurnStore{characterStates: map[string]store.CharacterState{}}
	for _, item := range initial {
		f.characterStates[relationshipStateKey(item.ChatSessionID, item.CharacterName)] = item
		f.returnCharStates = append(f.returnCharStates, item)
	}
	return f
}

func relationshipStateKey(sid, characterName string) string {
	return sid + "\x00" + strings.ToLower(strings.TrimSpace(characterName))
}

func (f *relationshipAccumulatingTurnStore) GetCharacterState(ctx context.Context, sid, characterName string) (*store.CharacterState, error) {
	if item, ok := f.characterStates[relationshipStateKey(sid, characterName)]; ok {
		cp := item
		return &cp, nil
	}
	return f.turnRecordingStore.GetCharacterState(ctx, sid, characterName)
}

func (f *relationshipAccumulatingTurnStore) ListCharacterStates(ctx context.Context, sid string) ([]store.CharacterState, error) {
	out := []store.CharacterState{}
	for _, item := range f.characterStates {
		if sid == "" || item.ChatSessionID == sid {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *relationshipAccumulatingTurnStore) SaveCharacterState(ctx context.Context, c *store.CharacterState) error {
	cp := *c
	f.savedCharacterStates = append(f.savedCharacterStates, &cp)
	f.characterStates[relationshipStateKey(cp.ChatSessionID, cp.CharacterName)] = cp
	return nil
}

func (f *turnRecordingStore) SavePendingThread(ctx context.Context, p *store.PendingThread) error {
	f.savedPendingThreads = append(f.savedPendingThreads, p)
	return nil
}

func (f *turnRecordingStore) SaveActiveState(ctx context.Context, a *store.ActiveState) error {
	f.savedActiveStates = append(f.savedActiveStates, a)
	return nil
}

func (f *turnRecordingStore) SaveCanonicalStateLayer(ctx context.Context, item *store.CanonicalStateLayer) error {
	f.savedCanonicalLayers = append(f.savedCanonicalLayers, item)
	return nil
}

func TestCompleteTurnNoopModeDoesNotWrite(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-noop","turn_index":3,"user_input":"hello","assistant_content":"hi","improvement_trace":{"score":8,"feedback_type":"style"}}`
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader([]byte(body)))
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

	if resp["save_ok"] != false {
		t.Errorf("save_ok = %v, want false", resp["save_ok"])
	}
	if resp["chat_logs_saved"] != float64(0) {
		t.Errorf("chat_logs_saved = %v, want 0", resp["chat_logs_saved"])
	}
	if resp["effective_input_saved"] != float64(0) {
		t.Errorf("effective_input_saved = %v, want 0", resp["effective_input_saved"])
	}
	if resp["audit_saved"] != float64(0) {
		t.Errorf("audit_saved = %v, want 0", resp["audit_saved"])
	}
	if resp["critic_feedback_saved"] != float64(0) {
		t.Errorf("critic_feedback_saved = %v, want 0", resp["critic_feedback_saved"])
	}
	if resp["store_write_attempted"] != float64(0) {
		t.Errorf("store_write_attempted = %v, want 0", resp["store_write_attempted"])
	}

	note, _ := resp["note"].(string)
	if !strings.Contains(note, "no mutations") {
		t.Errorf("note = %q, expected 'no mutations'", note)
	}

	if len(fake.savedChatLogs) != 0 {
		t.Errorf("expected 0 saved chat logs, got %d", len(fake.savedChatLogs))
	}

	wp, ok := resp["writeback_plan"].(map[string]any)
	if !ok {
		t.Fatalf("writeback_plan is not an object")
	}
	if wp["status"] != "ready" {
		t.Errorf("writeback_plan.status = %v, want ready", wp["status"])
	}
	if wp["store_write_enabled"] != false {
		t.Errorf("writeback_plan.store_write_enabled = %v, want false", wp["store_write_enabled"])
	}
	if wp["would_write"] != false {
		t.Errorf("writeback_plan.would_write = %v, want false", wp["would_write"])
	}
	wpTargets, _ := wp["targets"].([]any)
	if len(wpTargets) == 0 {
		t.Errorf("writeback_plan.targets is empty")
	}
	wpNotes, _ := wp["notes"].(string)
	if !strings.Contains(wpNotes, "R1 read-shadow") {
		t.Errorf("writeback_plan.notes missing R1 marker: %q", wpNotes)
	}
}

func TestCompleteTurnDualShadowWritesAll(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeDualShadow
	srv := NewServer(cfg)
	srv.Store = fake

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-dual","turn_index":4,"user_input":"user text","assistant_content":"assistant text","improvement_trace":{"score":7,"suggestion":"be more concise"}}`
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader([]byte(body)))
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

	if resp["save_ok"] != true {
		t.Errorf("save_ok = %v, want true", resp["save_ok"])
	}
	if resp["source"] != "dual_shadow" {
		t.Errorf("source = %v, want dual_shadow", resp["source"])
	}
	if resp["chat_logs_saved"] != float64(2) {
		t.Errorf("chat_logs_saved = %v, want 2", resp["chat_logs_saved"])
	}
	if resp["effective_input_saved"] != float64(1) {
		t.Errorf("effective_input_saved = %v, want 1", resp["effective_input_saved"])
	}
	if resp["audit_saved"] != float64(2) {
		t.Errorf("audit_saved = %v, want 2", resp["audit_saved"])
	}
	if resp["store_write_attempted"] != float64(6) {
		t.Errorf("store_write_attempted = %v, want 6", resp["store_write_attempted"])
	}
	if resp["memories_saved"] != float64(0) {
		t.Errorf("memories_saved = %v, want 0 without critic config", resp["memories_saved"])
	}
	if resp["evidence_saved"] != float64(0) {
		t.Errorf("evidence_saved = %v, want 0 without critic config", resp["evidence_saved"])
	}
	if resp["kg_triples_saved"] != float64(0) {
		t.Errorf("kg_triples_saved = %v, want 0 without critic config", resp["kg_triples_saved"])
	}
	if len(fake.savedMemories) != 0 {
		t.Errorf("savedMemories count = %d, want 0 without critic config", len(fake.savedMemories))
	}
	if len(fake.savedEvidence) != 0 {
		t.Errorf("savedEvidence count = %d, want 0 without critic config", len(fake.savedEvidence))
	}
	if len(fake.savedKGTriples) != 0 {
		t.Errorf("savedKGTriples count = %d, want 0 without critic config", len(fake.savedKGTriples))
	}

	wp, ok := resp["writeback_plan"].(map[string]any)
	if !ok {
		t.Fatalf("writeback_plan is not an object")
	}
	if wp["status"] != "ready" {
		t.Errorf("writeback_plan.status = %v, want ready", wp["status"])
	}
	if wp["store_write_enabled"] != true {
		t.Errorf("writeback_plan.store_write_enabled = %v, want true", wp["store_write_enabled"])
	}
	if wp["would_write"] != true {
		t.Errorf("writeback_plan.would_write = %v, want true", wp["would_write"])
	}
	wpTargets, _ := wp["targets"].([]any)
	if len(wpTargets) == 0 {
		t.Errorf("writeback_plan.targets is empty")
	}
	wpPreview, _ := wp["content_preview"].(string)
	if !strings.Contains(wpPreview, "user text") {
		t.Errorf("writeback_plan.content_preview missing user input: %q", wpPreview)
	}
}

func TestCompleteTurnMariaDBAuthorityWritesAll(t *testing.T) {
	fake := &turnRecordingStore{}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-auth","turn_index":5,"user_input":"authority input","assistant_content":"authority reply","improvement_trace":{"score":9}}`
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader([]byte(body)))
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

	if resp["save_ok"] != true {
		t.Errorf("save_ok = %v, want true", resp["save_ok"])
	}
	if resp["source"] != "mariadb_authority" {
		t.Errorf("source = %v, want mariadb_authority", resp["source"])
	}
	if resp["chat_logs_saved"] != float64(2) {
		t.Errorf("chat_logs_saved = %v, want 2", resp["chat_logs_saved"])
	}
	if resp["effective_input_saved"] != float64(1) {
		t.Errorf("effective_input_saved = %v, want 1", resp["effective_input_saved"])
	}
	if resp["memories_saved"] != float64(0) {
		t.Errorf("memories_saved = %v, want 0 without critic config", resp["memories_saved"])
	}
	if resp["evidence_saved"] != float64(0) {
		t.Errorf("evidence_saved = %v, want 0 without critic config", resp["evidence_saved"])
	}
	if resp["kg_triples_saved"] != float64(0) {
		t.Errorf("kg_triples_saved = %v, want 0 without critic config", resp["kg_triples_saved"])
	}
	if resp["derived_artifacts_saved"] != float64(0) {
		t.Errorf("derived_artifacts_saved = %v, want 0 without critic config", resp["derived_artifacts_saved"])
	}
	llmTrace, ok := resp["llm_config_trace"].(map[string]any)
	if !ok {
		t.Fatalf("llm_config_trace missing from complete-turn response: %+v", resp)
	}
	criticTrace, ok := llmTrace["critic"].(map[string]any)
	if !ok {
		t.Fatalf("critic trace missing: %+v", llmTrace)
	}
	if criticTrace["configured"] != false {
		t.Fatalf("critic trace configured = %v, want false without critic config", criticTrace["configured"])
	}
	missingFields, _ := criticTrace["missing_fields"].([]any)
	if len(missingFields) == 0 {
		t.Fatalf("critic missing_fields should explain why derived extraction was skipped: %+v", criticTrace)
	}
	if resp["store_write_attempted"] != float64(6) {
		t.Errorf("store_write_attempted = %v, want 6", resp["store_write_attempted"])
	}
	if len(fake.savedMemories) != 0 {
		t.Errorf("savedMemories count = %d, want 0 without critic config", len(fake.savedMemories))
	}
	if len(fake.savedEvidence) != 0 {
		t.Errorf("savedEvidence count = %d, want 0 without critic config", len(fake.savedEvidence))
	}
	if len(fake.savedKGTriples) != 0 {
		t.Errorf("savedKGTriples count = %d, want 0 without critic config", len(fake.savedKGTriples))
	}
	note, _ := resp["note"].(string)
	if !strings.Contains(note, "mariadb_authority") {
		t.Errorf("note = %q, want mariadb_authority marker", note)
	}
}

func TestCompleteTurnIdempotentReplaySkipsDuplicateDerivedWrites(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-retry", TurnIndex: 5, Role: "user", Content: "retry user"},
			{ID: 2, ChatSessionID: "sess-retry", TurnIndex: 5, Role: "assistant", Content: "retry assistant"},
		},
		returnMemories: []store.Memory{{ID: 10, ChatSessionID: "sess-retry", TurnIndex: 5, SummaryJSON: `{"turn_summary":"retry already derived"}`}},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	body := `{"chat_session_id":"sess-retry","turn_index":5,"user_input":"retry user","assistant_content":"retry assistant","client_meta":{"critic":{"api_key":"k","endpoint":"https://example.test/v1/chat/completions","model":"m","provider":"openai"}}}`
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader([]byte(body)))
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
	if resp["save_ok"] != true {
		t.Fatalf("save_ok = %v, want true", resp["save_ok"])
	}
	if resp["chat_logs_saved"] != float64(0) || resp["derived_artifacts_saved"] != float64(0) {
		t.Fatalf("idempotent replay should not write duplicate artifacts: %+v", resp)
	}
	if resp["critic_triggered"] != false {
		t.Fatalf("critic_triggered = %v, want false for idempotent replay", resp["critic_triggered"])
	}
	trace, ok := resp["trace_handoff"].(map[string]any)
	if !ok || trace["idempotent_replay"] != true {
		t.Fatalf("trace_handoff missing idempotent replay marker: %+v", resp["trace_handoff"])
	}
	if len(fake.savedChatLogs) != 0 || len(fake.savedMemories) != 0 || len(fake.savedEvidence) != 0 || len(fake.savedKGTriples) != 0 {
		t.Fatalf("idempotent replay should not save duplicate rows, logs=%d memories=%d evidence=%d kg=%d", len(fake.savedChatLogs), len(fake.savedMemories), len(fake.savedEvidence), len(fake.savedKGTriples))
	}
}

func TestCompleteTurnExistingRawWithoutDerivedRetriesCriticWithoutDuplicateLogs(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-raw-only", TurnIndex: 3, Role: "user", Content: "Mina found a brass key."},
			{ID: 2, ChatSessionID: "sess-raw-only", TurnIndex: 3, Role: "assistant", Content: "Mina gave Rowan the brass key."},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	extraction := map[string]any{
		"turn_summary":      "Mina found a brass key and gave it to Rowan.",
		"importance_score":  7,
		"evidence_excerpts": []any{"Mina found a brass key."},
		"kg_triples":        []any{map[string]any{"subject": "Mina", "predicate": "gave", "object": "brass key", "valid_from": 3}},
		"entities":          map[string]any{"characters": []any{map[string]any{"name": "Mina"}}, "items": []any{map[string]any{"name": "brass key"}}},
	}
	extractionBytes, _ := json.Marshal(extraction)
	chatResp, _ := json.Marshal(map[string]any{
		"model":   "critic-model",
		"choices": []any{map[string]any{"message": map[string]any{"content": string(extractionBytes)}}},
	})
	oldClient := proxyHTTPClient
	proxyHTTPClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(chatResp)),
		}, nil
	})}
	defer func() { proxyHTTPClient = oldClient }()

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"sess-raw-only","turn_index":3,"user_input":"Mina found a brass key.","assistant_content":"Mina gave Rowan the brass key.","client_meta":{"critic":{"api_key":"k","endpoint":"https://api.example.com/v1/chat/completions","model":"critic-model","provider":"openai"}}}`
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader([]byte(body)))
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
	if resp["critic_triggered"] != true {
		t.Fatalf("critic_triggered = %v, want true for raw-only retry", resp["critic_triggered"])
	}
	if resp["chat_logs_saved"] != float64(0) || len(fake.savedChatLogs) != 0 {
		t.Fatalf("raw-only retry must not duplicate chat logs, resp=%+v saved=%#v", resp, fake.savedChatLogs)
	}
	if len(fake.savedMemories) != 1 || len(fake.savedEvidence) != 1 || len(fake.savedKGTriples) != 1 {
		t.Fatalf("raw-only retry should save derived artifacts, memories=%d evidence=%d kg=%d", len(fake.savedMemories), len(fake.savedEvidence), len(fake.savedKGTriples))
	}
	if fake.savedMemories[0].TurnIndex != 3 || fake.savedEvidence[0].TurnAnchor != 3 || fake.savedKGTriples[0].SourceTurn != 3 {
		t.Fatalf("raw-only retry must derive against original turn 3, memory=%d evidence=%d kg=%d", fake.savedMemories[0].TurnIndex, fake.savedEvidence[0].TurnAnchor, fake.savedKGTriples[0].SourceTurn)
	}
}

func TestCompleteTurnConflictingExistingRawPairDoesNotDuplicateArtifacts(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-conflict", TurnIndex: 2, Role: "user", Content: "old user text"},
			{ID: 2, ChatSessionID: "sess-conflict", TurnIndex: 2, Role: "assistant", Content: "old assistant text"},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"sess-conflict","turn_index":2,"user_input":"new user text","assistant_content":"new assistant text","client_meta":{"active_chat_backfill":{"source":"risu_active_chat_complete_turn_backfill","preserve_requested_turn_index":true},"critic":{"api_key":"","endpoint":"","model":""}}}`
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader([]byte(body)))
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
	if resp["chat_logs_saved"] != float64(0) || len(fake.savedChatLogs) != 0 {
		t.Fatalf("conflicting raw pair must not duplicate chat logs, resp=%+v saved=%#v", resp, fake.savedChatLogs)
	}
	if len(fake.savedMemories) != 0 || len(fake.savedEvidence) != 0 || len(fake.savedKGTriples) != 0 {
		t.Fatalf("conflicting raw pair must not create derived duplicates, memories=%d evidence=%d kg=%d", len(fake.savedMemories), len(fake.savedEvidence), len(fake.savedKGTriples))
	}
	failReasons, _ := resp["fail_reasons"].([]any)
	if len(failReasons) == 0 || failReasons[0] != "raw_turn_content_conflict" {
		t.Fatalf("fail_reasons = %#v, want raw_turn_content_conflict", resp["fail_reasons"])
	}
}

func TestCompleteTurnConflictingExistingRawPairWithoutPreserveDoesNotDuplicateArtifacts(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-native-conflict", TurnIndex: 14, Role: "user", Content: "old user text"},
			{ID: 2, ChatSessionID: "sess-native-conflict", TurnIndex: 14, Role: "assistant", Content: "old assistant text"},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"sess-native-conflict","turn_index":14,"user_input":"new user text","assistant_content":"new assistant text","client_meta":{"critic":{"api_key":"k","endpoint":"https://example.test/v1/chat/completions","model":"m","provider":"openai"}}}`
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader([]byte(body)))
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
	if resp["turn_index"] != float64(14) || resp["chat_logs_saved"] != float64(0) || resp["derived_artifacts_saved"] != float64(0) {
		t.Fatalf("conflicting native replay must not append duplicate turn rows: %+v", resp)
	}
	if len(fake.savedChatLogs) != 0 || len(fake.savedMemories) != 0 || len(fake.savedEvidence) != 0 || len(fake.savedKGTriples) != 0 {
		t.Fatalf("conflicting native replay must not save rows, logs=%d memories=%d evidence=%d kg=%d", len(fake.savedChatLogs), len(fake.savedMemories), len(fake.savedEvidence), len(fake.savedKGTriples))
	}
	failReasons, _ := resp["fail_reasons"].([]any)
	if len(failReasons) == 0 || failReasons[0] != "raw_turn_content_conflict" {
		t.Fatalf("fail_reasons = %#v, want raw_turn_content_conflict", resp["fail_reasons"])
	}
}

func TestCompleteTurnPartialExistingRawKeepsRequestedTurnIndex(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-partial-raw", TurnIndex: 7, Role: "user", Content: "same user text"},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"sess-partial-raw","turn_index":7,"user_input":"same user text","assistant_content":"assistant repaired later","client_meta":{"critic":{"api_key":"","endpoint":"","model":""}}}`
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader([]byte(body)))
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
	if resp["turn_index"] != float64(7) {
		t.Fatalf("turn_index = %v, want partial raw retry to keep requested turn 7", resp["turn_index"])
	}
	if len(fake.savedChatLogs) != 1 || fake.savedChatLogs[0].TurnIndex != 7 || fake.savedChatLogs[0].Role != "assistant" {
		t.Fatalf("partial raw retry should save only missing assistant at turn 7, saved=%#v", fake.savedChatLogs)
	}
}

func TestCompleteTurnActiveChatBackfillPreservesRequestedTurnIndex(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-active-rebuild", TurnIndex: 35, Role: "user", Content: "existing late user"},
			{ID: 2, ChatSessionID: "sess-active-rebuild", TurnIndex: 35, Role: "assistant", Content: "existing late assistant"},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"sess-active-rebuild","turn_index":6,"user_input":"early missing user","assistant_content":"early missing assistant","client_meta":{"active_chat_backfill":{"source":"active_chat_recent_rebuild","preserve_requested_turn_index":true},"critic":{"api_key":"","endpoint":"","model":""}}}`
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader([]byte(body)))
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
	if resp["turn_index"] != float64(6) {
		t.Fatalf("turn_index = %v, want requested turn 6 preserved", resp["turn_index"])
	}
	if len(fake.savedChatLogs) != 2 {
		t.Fatalf("savedChatLogs = %d, want 2", len(fake.savedChatLogs))
	}
	for _, log := range fake.savedChatLogs {
		if log.TurnIndex != 6 {
			t.Fatalf("saved chat log turn = %d, want 6; logs=%#v", log.TurnIndex, fake.savedChatLogs)
		}
	}
}

func TestCompleteTurnExactPairAlreadyPersistedOnAnotherTurnSkipsDuplicate(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-pair-replay", TurnIndex: 5, Role: "user", Content: "same user text"},
			{ID: 2, ChatSessionID: "sess-pair-replay", TurnIndex: 5, Role: "assistant", Content: "same assistant text"},
			{ID: 3, ChatSessionID: "sess-pair-replay", TurnIndex: 7, Role: "user", Content: "later user text"},
			{ID: 4, ChatSessionID: "sess-pair-replay", TurnIndex: 7, Role: "assistant", Content: "later assistant text"},
		},
		returnMemories: []store.Memory{
			{ID: 10, ChatSessionID: "sess-pair-replay", TurnIndex: 5, SummaryJSON: `{"summary":"same pair already derived"}`},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	body := `{"chat_session_id":"sess-pair-replay","turn_index":8,"user_input":"same user text","assistant_content":"same assistant text","client_meta":{"critic":{"api_key":"k","endpoint":"https://example.test/v1/chat/completions","model":"m","provider":"openai"}}}`
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader([]byte(body)))
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
	if resp["turn_index"] != float64(5) {
		t.Fatalf("turn_index = %v, want existing turn 5", resp["turn_index"])
	}
	if resp["chat_logs_saved"] != float64(0) || resp["derived_artifacts_saved"] != float64(0) || resp["critic_triggered"] != false {
		t.Fatalf("exact pair replay must not save duplicate raw/derived rows: %+v", resp)
	}
	if len(fake.savedChatLogs) != 0 || len(fake.savedMemories) != 0 || len(fake.savedEvidence) != 0 || len(fake.savedKGTriples) != 0 {
		t.Fatalf("exact pair replay saved duplicate artifacts, logs=%d memories=%d evidence=%d kg=%d", len(fake.savedChatLogs), len(fake.savedMemories), len(fake.savedEvidence), len(fake.savedKGTriples))
	}
	trace, _ := resp["trace_handoff"].(map[string]any)
	if trace["duplicate_guard"] != "same_session_exact_pair_exists_on_another_turn" {
		t.Fatalf("duplicate_guard = %v, want same_session_exact_pair_exists_on_another_turn; resp=%+v", trace["duplicate_guard"], resp)
	}
}

func TestCompleteTurnPostprocessorPairAlreadyPersistedOnAnotherTurnSkipsDuplicate(t *testing.T) {
	fake := &turnRecordingStore{
		returnChatLogs: []store.ChatLog{
			{ID: 1, ChatSessionID: "sess-post-replay", TurnIndex: 7, Role: "user", Content: "same selected option"},
			{ID: 2, ChatSessionID: "sess-post-replay", TurnIndex: 7, Role: "assistant", Content: `<ReKoCompare><ReKoBefore>draft text</ReKoBefore><ReKoAfter>final polished text</ReKoAfter><ReKoMeta>mode=KR</ReKoMeta></ReKoCompare>`},
		},
		returnMemories: []store.Memory{
			{ID: 10, ChatSessionID: "sess-post-replay", TurnIndex: 7, SummaryJSON: `{"summary":"already derived"}`},
		},
	}
	cfg := config.Default()
	cfg.StoreMode = config.StoreModeMariaDBAuthority
	srv := NewServer(cfg)
	srv.Store = fake
	srv.StoreOpenError = nil

	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)
	bodyBytes, err := json.Marshal(map[string]any{
		"chat_session_id":   "sess-post-replay",
		"turn_index":        8,
		"user_input":        "same selected option",
		"assistant_content": "final polished text",
		"client_meta": map[string]any{
			"critic": map[string]any{
				"api_key":  "k",
				"endpoint": "https://example.test/v1/chat/completions",
				"model":    "m",
				"provider": "openai",
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/complete-turn", bytes.NewReader(bodyBytes))
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
	if resp["turn_index"] != float64(7) {
		t.Fatalf("turn_index = %v, want existing turn 7", resp["turn_index"])
	}
	if resp["chat_logs_saved"] != float64(0) || resp["derived_artifacts_saved"] != float64(0) || resp["critic_triggered"] != false {
		t.Fatalf("postprocessor replay must not save duplicate rows: %+v", resp)
	}
	if len(fake.savedChatLogs) != 0 || len(fake.savedMemories) != 0 || len(fake.savedEvidence) != 0 || len(fake.savedKGTriples) != 0 {
		t.Fatalf("postprocessor replay saved duplicate artifacts, logs=%d memories=%d evidence=%d kg=%d", len(fake.savedChatLogs), len(fake.savedMemories), len(fake.savedEvidence), len(fake.savedKGTriples))
	}
	trace, _ := resp["trace_handoff"].(map[string]any)
	if trace["duplicate_guard"] != "same_session_exact_pair_exists_on_another_turn" {
		t.Fatalf("duplicate_guard = %v, want same_session_exact_pair_exists_on_another_turn; resp=%+v", trace["duplicate_guard"], resp)
	}
}
